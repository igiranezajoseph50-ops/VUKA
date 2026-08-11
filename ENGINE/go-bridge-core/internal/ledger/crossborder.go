// Cross-border transfer execution (Phase 2).
//
// A CrossBorderTransfer moves value between a source account in one currency
// (e.g. RWF BUSINESS, Rwanda importer) and a destination account in another
// (e.g. KES BUSINESS, Kenya supplier), routed through the engine's SETTLEMENT
// accounts. It is a single idempotent database transaction that writes FOUR
// ledger entries (two per currency leg) so every leg sums to zero:
//
//	Leg A (RWF):  debit source BUSINESS  -amount
//	             credit SETTLEMENT(RWF) +amount
//	Leg B (KES):  debit SETTLEMENT(KES)  -amount_kes
//	credit dest BUSINESS                +amount_kes
//
// amount_kes = round4(amount / fx_rate). The rail (M-Pesa Kenya for the KES
// leg) is dispatched inside the tx exactly like a same-currency transfer:
// entries are written only on rail SUCCESS, and a failure rolls everything
// back leaving no trace.
package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/idempotency"
)

// CrossBorderTransfer executes an idempotent cross-currency transfer. Key
// semantics match Transfer: replays return the original transfer.
// currency_from must equal the source account's currency and currency_to the
// destination's; the two legs are balanced through SETTLEMENT accounts.
func (s *Service) CrossBorderTransfer(ctx context.Context, key string, req CrossBorderRequest) (*Transfer, bool, error) {
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
	if !validCurrency(req.CurrencyFrom) || !validCurrency(req.CurrencyTo) {
		return nil, false, fmt.Errorf("ledger: invalid currency pair %q -> %q", req.CurrencyFrom, req.CurrencyTo)
	}
	if req.CurrencyFrom == req.CurrencyTo {
		return nil, false, fmt.Errorf("%w: use Transfer for same-currency movements", ErrCurrencyMismatch)
	}
	fxRate, err := s.resolveFxRate(req.FxRate)
	if err != nil {
		return nil, false, err
	}

	// Idempotency pre-check outside the tx (see Transfer for rationale).
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

	// 1. Lock both business accounts.
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

	// 2. Currency integrity for the corridor.
	if src.Currency != req.CurrencyFrom {
		return nil, false, fmt.Errorf("%w: source=%s requested=%s", ErrCurrencyMismatch, src.Currency, req.CurrencyFrom)
	}
	if dst.Currency != req.CurrencyTo {
		return nil, false, fmt.Errorf("%w: destination=%s requested=%s", ErrCurrencyMismatch, dst.Currency, req.CurrencyTo)
	}

	// 3. Derived balance check on the source (golden rule).
	balance, err := lockedBalance(ctx, tx, src.ID)
	if err != nil {
		return nil, false, err
	}
	if balance < req.Amount {
		return nil, false, ErrInsufficientFunds
	}

	// 4. Lock the SETTLEMENT legs, one per currency.
	settleFrom, err := lockSettlementAccount(ctx, tx, req.CurrencyFrom)
	if err != nil {
		return nil, false, err
	}
	settleTo, err := lockSettlementAccount(ctx, tx, req.CurrencyTo)
	if err != nil {
		return nil, false, err
	}

	// 5. Insert the header with the fx_rate recorded for audit + reversal.
	transferReq := TransferRequest{
		SourceAccountID:      src.ID,
		DestinationAccountID: dst.ID,
		Amount:               req.Amount,
		Currency:             req.CurrencyFrom,
		InvoiceNumber:        req.InvoiceNumber,
	}
	transferID, err := insertTransaction(ctx, tx, key, transferReq, StatusProcessing, "", "", &fxRate)
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

	// 6. Claim the idempotency key inside this tx (same semantics as Transfer).
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

	// 7. Dispatch the KES leg to the Kenya rail. The rail is charged the
	// converted amount via the destination account id as the wallet address
	// (mirroring Transfer's simulated dispatch).
	adapter := s.registry.Get("mpesa-ke")
	if adapter == nil {
		adapter = s.registry.Get(s.adapterName())
	}
	if adapter == nil {
		return nil, false, ErrAdapterUnavailable
	}
	amountTo := round4(req.Amount / fxRate)
	result, err := adapter.Payout(ctx, adapters.PayoutRequest{
		Amount:        amountTo,
		Currency:      req.CurrencyTo,
		Phone:         dst.ID, // simulated rail wallet address
		Reference:     transferID,
		InvoiceNumber: req.InvoiceNumber,
	})
	if err != nil {
		return nil, false, fmt.Errorf("ledger: cross-border rail dispatch: %w", err)
	}

	// 8. Write the two FX legs (4 entries). Each leg sums to zero.
	if err := writeCrossBorderEntries(ctx, tx, transferID, src.ID, settleFrom.ID, settleTo.ID, dst.ID, req.Amount, amountTo); err != nil {
		return nil, false, err
	}

	// 9. Mark SUCCESS and commit atomically.
	if err := markStatus(ctx, tx, transferID, StatusSuccess, result.ExternalRef, ""); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("ledger: commit cross-border transfer: %w", err)
	}

	t, err := s.GetTransfer(ctx, transferID)
	if err != nil {
		return nil, false, err
	}
	return t, false, nil
}

// writeCrossBorderEntries writes the four ledger entries for a cross-border
// transfer (two per leg, each leg summing to zero).
func writeCrossBorderEntries(ctx context.Context, tx pgx.Tx, transferID, sourceID, settleFromID, settleToID, destID string, amount, amountTo float64) error {
	entries := []struct {
		accountID string
		amount    float64
	}{
		{sourceID, -amount},    // RWF: debit source BUSINESS
		{settleFromID, amount}, // RWF: credit SETTLEMENT
		{settleToID, -amountTo}, // KES: debit SETTLEMENT
		{destID, amountTo},     // KES: credit destination BUSINESS
	}
	for _, e := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ledger_entries (transaction_id, account_id, amount)
			VALUES ($1, $2, $3)`,
			transferID, e.accountID, e.amount,
		); err != nil {
			return fmt.Errorf("ledger: insert cross-border entry for account %s: %w", e.accountID, err)
		}
	}
	return nil
}