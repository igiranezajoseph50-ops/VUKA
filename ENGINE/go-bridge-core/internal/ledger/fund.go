// Account funding (external cash-in).
//
// A funding operation models money arriving from OUTSIDE the VUKA ledger
// (a trader topping up their Business wallet from MTN MoMo, a bank transfer,
// or a cash deposit). Because the offsetting debit lives at the external
// institution, a funding writes exactly ONE ledger entry: a credit to the
// target account. Transfers — the only in-ledger money movement — always
// write the canonical two-entry pair via writeDoubleEntry (net zero).
//
// The funding is still recorded under a transaction header so every credit
// has an auditable provenance row with its own idempotency key. Funding is
// NOT double-entry internally by design; the pair invariant applies to
// transfers between VUKA accounts (see transfer.go).
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FundRequest is the payload for crediting an account with external funds.
type FundRequest struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	// Reference is an optional external reference (e.g. the MTN transaction
	// id of the incoming mobile-money transfer).
	Reference string `json:"reference,omitempty"`
}

// FundAccount credits an account from an external source. It returns the
// created transaction header. The operation is atomic and idempotency-keyed
// like a transfer.
func (s *Service) FundAccount(ctx context.Context, accountID, key string, req FundRequest) (*Transfer, error) {
	if key == "" {
		return nil, fmt.Errorf("%w: funding requires an idempotency key", ErrIdempotencyKeyRequired)
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("%w: funding amount must be positive", ErrInvalidAmount)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ledger: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	// The source and destination are the same account row for a funding:
	// the header records provenance, the single credit entry records value.
	acc, err := lockAccount(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	if req.Currency != "" && req.Currency != acc.Currency {
		return nil, fmt.Errorf("%w: account currency is %s, got %s", ErrCurrencyMismatch, acc.Currency, req.Currency)
	}
	currency := acc.Currency

	// Header insert. UNIQUE on idempotency_key is the first guard; if a
	// concurrent funding with the same key won the race, replay its result.
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO transactions (idempotency_key, source_account_id, destination_account_id,
		                          amount, currency, status, external_reference)
		VALUES ($1, $2, $3, $4, $5, 'SUCCESS', $6)
		RETURNING id`,
		key, accountID, accountID, req.Amount, currency, nullIfEmpty(req.Reference),
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			t, ferr := s.GetTransferByIdempotencyKey(ctx, key)
			if ferr != nil {
				return nil, fmt.Errorf("ledger: funding idempotency replay: %w", ferr)
			}
			return t, nil
		}
		return nil, fmt.Errorf("ledger: insert funding header: %w", err)
	}

	// Single credit entry (external cash-in; see package doc).
	if _, err := tx.Exec(ctx, `
		INSERT INTO ledger_entries (transaction_id, account_id, amount)
		VALUES ($1, $2, $3)`,
		id, accountID, req.Amount,
	); err != nil {
		return nil, fmt.Errorf("ledger: insert funding entry: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ledger: commit funding: %w", err)
	}
	return s.GetTransfer(ctx, id)
}

// GetAccountByUserAndType resolves the account of a given type for a user.
// Used by tests and the API to find a trader's Business wallet.
func (s *Service) GetAccountByUserAndType(ctx context.Context, userID string, at AccountType) (*Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, type, currency, created_at
		FROM accounts WHERE user_id = $1 AND type = $2`, userID, at,
	).Scan(&a.ID, &a.UserID, &a.Type, &a.Currency, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get account by user/type: %w", err)
	}
	return &a, nil
}
