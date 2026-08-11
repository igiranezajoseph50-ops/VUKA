// Transfer execution — the heart of the VUKA engine.
//
// A transfer is a single database transaction that guarantees:
//   1. Idempotency  — the Idempotency-Key is claimed atomically; replays
//      return the original transfer instead of charging twice.
//   2. Isolation    — the source account row is locked FOR UPDATE so
//      concurrent transfers serialize per account and can never corrupt a
//      balance.
//   3. Validation   — BUSINESS-only source (spec §4.1), same currency,
//      positive amount, distinct accounts, sufficient derived balance.
//   4. Two-phase    — the telecom adapter is dispatched inside the tx;
//      ledger entries are written ONLY on adapter SUCCESS. A provider
//      failure or timeout rolls the whole transfer back (no partial state).
//   5. Double-entry — exactly two ledger entries per transfer, summing to
//      zero: debit the source BUSINESS account, credit the destination
//      SETTLEMENT account.
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/idempotency"
)

// Transfer executes an idempotent, atomic, invoice-linked transfer.
//
// key is the client-supplied Idempotency-Key (UUIDv4). On replay of a key
// that already produced a transfer, the original transfer is returned with
// replay=true so the API can answer 200 with the stored result.
func (s *Service) Transfer(ctx context.Context, key string, req TransferRequest) (*Transfer, bool, error) {
	if key == "" {
		return nil, false, ErrIdempotencyKeyRequired
	}
	if req.SourceAccountID == "" || req.DestinationAccountID == "" {
		return nil, false, fmt.Errorf("ledger: source and destination accounts are required")
	}
	if req.SourceAccountID == req.DestinationAccountID {
		return nil, false, ErrSameAccount
	}
	if req.Amount <= 0 {
		return nil, false, fmt.Errorf("%w: amount must be positive, got %v", ErrInvalidAmount, req.Amount)
	}
	if !validCurrency(req.Currency) {
		return nil, false, fmt.Errorf("ledger: invalid currency %q", req.Currency)
	}

	adapter := s.registry.Get(s.adapterName())
	if adapter == nil {
		return nil, false, ErrAdapterUnavailable
	}

	// Idempotency pre-check outside the tx: if the key already resolved to a
	// transfer, replay it directly. The authoritative claim still happens
	// inside the tx (see below); this short-circuit avoids spinning a tx for
	// the common replay case.
	if existingID, ok, err := s.idem.Lookup(ctx, key); err != nil {
		return nil, false, err
	} else if ok {
		t, err := s.GetTransfer(ctx, existingID)
		if err != nil {
			return nil, false, err
		}
		return t, true, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("ledger: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	// 1. Lock the source account row. This serialises transfers touching the
	// same account; the balance computed below is therefore stable for the
	// lifetime of this tx.
	src, err := lockAccount(ctx, tx, req.SourceAccountID)
	if err != nil {
		return nil, false, err
	}
	if src.Type != AccountTypeBusiness {
		return nil, false, ErrInvalidAccountType
	}

	dst, err := lockAccount(ctx, tx, req.DestinationAccountID)
	if err != nil {
		return nil, false, err
	}

	// 2. Currency integrity across the pair.
	if src.Currency != req.Currency || dst.Currency != req.Currency {
		return nil, false, fmt.Errorf("%w: source=%s dest=%s requested=%s",
			ErrCurrencyMismatch, src.Currency, dst.Currency, req.Currency)
	}

	// 3. Derived balance check (golden rule: SUM of entries, never a column).
	balance, err := lockedBalance(ctx, tx, src.ID)
	if err != nil {
		return nil, false, err
	}
	if balance < req.Amount {
		return nil, false, ErrInsufficientFunds
	}

	// 4. Insert the transaction header in PROCESSING state (pending rail
	// dispatch). The UNIQUE constraint on transactions.idempotency_key is the
	// first idempotency guard: if a concurrent request with the same key
	// already committed, this insert fails with a unique violation and we
	// replay the winner's transfer.
	transferID, err := insertTransaction(ctx, tx, key, req, StatusProcessing, "", "", nil)
	if err != nil {
		if isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			t, err := s.GetTransferByIdempotencyKey(ctx, key)
			if err != nil {
				return nil, false, err
			}
			return t, true, nil
		}
		return nil, false, err
	}

	// 5. Claim the idempotency key inside this tx, pointing at the real
	// transaction id. If a concurrent request already claimed the key, we
	// lose the race: roll back and replay its transfer.
	if err := s.idem.Claim(ctx, tx, key, transferID); err != nil {
		if errors.Is(err, idempotency.ErrKeyExists) {
			_ = tx.Rollback(ctx)
			t, err := s.GetTransferByIdempotencyKey(ctx, key)
			if err != nil {
				return nil, false, err
			}
			return t, true, nil
		}
		return nil, false, err
	}

	// 6. Dispatch to the telecom rail. On failure/timeout the tx rolls back:
	// no entries, no SUCCESS, and the idempotency key remains unclaimed so a
	// later retry with the same key can still succeed.
	result, err := adapter.Payout(ctx, adapters.PayoutRequest{
		Amount:        req.Amount,
		Currency:      req.Currency,
		Phone:         dst.ID, // simulated rail uses the destination account as the wallet address
		Reference:     transferID,
		InvoiceNumber: req.InvoiceNumber,
	})
	if err != nil {
		return nil, false, fmt.Errorf("ledger: rail dispatch: %w", err)
	}

	// 7. Write the double-entry rows: debit source BUSINESS, credit
	// destination SETTLEMENT. Sum must be zero.
	if err := writeDoubleEntry(ctx, tx, transferID, src.ID, dst.ID, req.Amount); err != nil {
		return nil, false, err
	}

	// 8. Mark the header SUCCESS and commit everything atomically.
	if err := markStatus(ctx, tx, transferID, StatusSuccess, result.ExternalRef, ""); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("ledger: commit transfer: %w", err)
	}

	t, err := s.GetTransfer(ctx, transferID)
	if err != nil {
		return nil, false, err
	}
	return t, false, nil
}

// ReverseTransfer reverses a SUCCESS transfer by writing contra entries
// (credit back the source BUSINESS account, debit the destination SETTLEMENT
// account) and marking the original REVERSED. A new transaction header
// records the reversal for auditability.
func (s *Service) ReverseTransfer(ctx context.Context, transferID string, reason string) (*Transfer, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ledger: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	original, err := lockTransfer(ctx, tx, transferID)
	if err != nil {
		return nil, err
	}
	if original.Status != StatusSuccess {
		return nil, fmt.Errorf("%w: transfer is %s", ErrTransferNotSuccess, original.Status)
	}

	if original.FxRate != nil && *original.FxRate > 0 {
		// Cross-border reversal: contra BOTH FX legs (4 entries), restoring
		// source and destination balances and zeroing the settlement accounts.
		settleFrom, err := lockSettlementAccount(ctx, tx, original.Currency)
		if err != nil {
			return nil, err
		}
		// Destination currency is derivable from the destination account row;
		// the header only stores the source currency.
		dstAcc, err := lockAccount(ctx, tx, original.DestinationAccountID)
		if err != nil {
			return nil, err
		}
		settleTo, err := lockSettlementAccount(ctx, tx, dstAcc.Currency)
		if err != nil {
			return nil, err
		}
		amountTo := round4(original.Amount / *original.FxRate)
		if err := writeCrossBorderEntries(ctx, tx, original.ID, original.SourceAccountID, settleFrom.ID, settleTo.ID, original.DestinationAccountID, -original.Amount, -amountTo); err != nil {
			return nil, err
		}
	} else {
		// Contra entries: reverse the original pair exactly.
		if err := writeDoubleEntry(ctx, tx, original.ID, original.SourceAccountID, original.DestinationAccountID, -original.Amount); err != nil {
			return nil, err
		}
	}

	if err := markStatus(ctx, tx, original.ID, StatusReversed, "", reason); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ledger: commit reversal: %w", err)
	}
	return s.GetTransfer(ctx, original.ID)
}

// writeDoubleEntry inserts the canonical debit/credit pair for a transfer.
// When amount is negative the roles swap: the destination is debited and the
// source is credited (reversal). The pair always sums to zero.
func writeDoubleEntry(ctx context.Context, tx pgx.Tx, transferID, sourceID, destID string, amount float64) error {
	entries := []struct {
		accountID string
		amount    float64
	}{
		{sourceID, -amount},
		{destID, amount},
	}
	for _, e := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ledger_entries (transaction_id, account_id, amount)
			VALUES ($1, $2, $3)`,
			transferID, e.accountID, e.amount,
		); err != nil {
			return fmt.Errorf("ledger: insert entry for account %s: %w", e.accountID, err)
		}
	}
	return nil
}

// insertTransaction inserts a transaction header and returns its UUID.
// fxRate is non-nil only for cross-border transfers; same-currency transfers
// leave the column NULL.
func insertTransaction(ctx context.Context, tx pgx.Tx, key string, req TransferRequest, status TransactionStatus, externalRef, failureReason string, fxRate *float64) (string, error) {
	var id string
	var invoice *string
	if req.InvoiceNumber != "" {
		invoice = &req.InvoiceNumber
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO transactions (idempotency_key, invoice_number, source_account_id,
		                          destination_account_id, amount, currency, fx_rate, status,
		                          external_reference, failure_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		key, invoice, req.SourceAccountID, req.DestinationAccountID,
		req.Amount, req.Currency, fxRate, status, nullIfEmpty(externalRef), nullIfEmpty(failureReason),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("ledger: insert transaction: %w", err)
	}
	return id, nil
}

// markStatus updates a transaction header's status and optional fields.
func markStatus(ctx context.Context, tx pgx.Tx, transferID string, status TransactionStatus, externalRef, failureReason string) error {
	_, err := tx.Exec(ctx, `
		UPDATE transactions
		SET status = $2,
		    external_reference = COALESCE($3, external_reference),
		    failure_reason = COALESCE($4, failure_reason),
		    updated_at = now()
		WHERE id = $1`,
		transferID, status, nullIfEmpty(externalRef), nullIfEmpty(failureReason),
	)
	if err != nil {
		return fmt.Errorf("ledger: update status: %w", err)
	}
	return nil
}

// lockAccount fetches an account row FOR UPDATE within the tx. Returns
// ErrAccountNotFound for unknown ids and a wrapped error on FK failure.
func lockAccount(ctx context.Context, tx pgx.Tx, accountID string) (*Account, error) {
	var a Account
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, type, currency, created_at
		FROM accounts WHERE id = $1 FOR UPDATE`, accountID,
	).Scan(&a.ID, &a.UserID, &a.Type, &a.Currency, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: lock account %s: %w", accountID, err)
	}
	return &a, nil
}

// lockTransfer fetches a transaction header FOR UPDATE within the tx.
func lockTransfer(ctx context.Context, tx pgx.Tx, transferID string) (*Transfer, error) {
	var t Transfer
	err := tx.QueryRow(ctx, `
		SELECT id, idempotency_key, invoice_number, source_account_id,
		       destination_account_id, amount, currency, fx_rate, status,
		       external_reference, failure_reason, created_at, updated_at
		FROM transactions WHERE id = $1 FOR UPDATE`, transferID,
	).Scan(&t.ID, &t.IdempotencyKey, &t.InvoiceNumber, &t.SourceAccountID,
		&t.DestinationAccountID, &t.Amount, &t.Currency, &t.FxRate, &t.Status,
		&t.ExternalReference, &t.FailureReason, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: lock transfer %s: %w", transferID, err)
	}
	return &t, nil
}

// lockedBalance computes SUM(ledger_entries.amount) for an account whose row
// is already locked FOR UPDATE by the caller. Because the account lock
// serialises writers, this derived balance is race-free.
func lockedBalance(ctx context.Context, tx pgx.Tx, accountID string) (float64, error) {
	var balance float64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = $1`,
		accountID,
	).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("ledger: balance for %s: %w", accountID, err)
	}
	return balance, nil
}

// nullIfEmpty converts "" to nil so nullable columns stay NULL.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// adapterName returns the configured adapter key. The registry lookup in
// Transfer uses it; a zero config resolves to the MTN simulator.
func (s *Service) adapterName() string {
	if s.registry == nil || len(s.registry.Names()) == 0 {
		return ""
	}
	// Prefer "mtn-rw", else the single registered adapter, else first.
	names := s.registry.Names()
	for _, n := range names {
		if n == "mtn-rw" {
			return n
		}
	}
	return names[0]
}
