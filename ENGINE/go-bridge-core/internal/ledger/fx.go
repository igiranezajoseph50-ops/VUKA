// Cross-border FX plumbing: settlement accounts and rate resolution.
//
// Cross-currency transfers (Phase 2, RWF -> KES) move value through a
// SETTLEMENT account per currency so every leg balances to zero:
//
//	Leg A (RWF):  debit source BUSINESS -amount, credit SETTLEMENT +amount
//	Leg B (KES):  debit SETTLEMENT -amount_kes, credit dest BUSINESS +amount_kes
//
// The SETTLEMENT account rows are created idempotently by
// EnsureSettlementAccounts; the engine never invents accounts on the fly.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
)

// settlementUserName / settlementUserPhone identify the engine-owned system
// user that owns every SETTLEMENT account. Reserved, never a trader.
const (
	settlementUserName  = "VUKA Settlement"
	settlementUserPhone = "+0000000000"
)

// EnsureSettlementAccounts idempotently creates the settlement system user and
// a SETTLEMENT account for each currency. It is safe to call on every startup
// and from tests before a cross-border transfer.
func (s *Service) EnsureSettlementAccounts(ctx context.Context, currencies ...string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ledger: begin settlement tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	// Create the system user once (phone is UNIQUE).
	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (full_name, phone_number, business_reg_number, kyc_status)
		VALUES ($1, $2, NULL, 'APPROVED')
		ON CONFLICT (phone_number) DO UPDATE SET full_name = EXCLUDED.full_name
		RETURNING id`,
		settlementUserName, settlementUserPhone,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("ledger: ensure settlement user: %w", err)
	}

	for _, currency := range currencies {
		if !validCurrency(currency) {
			return fmt.Errorf("ledger: invalid settlement currency %q", currency)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO accounts (user_id, type, currency)
			VALUES ($1, 'SETTLEMENT', $2)
			ON CONFLICT (user_id, type, currency) DO NOTHING`,
			userID, currency,
		); err != nil {
			return fmt.Errorf("ledger: ensure settlement account %s: %w", currency, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ledger: commit settlement accounts: %w", err)
	}
	return nil
}

// lockSettlementAccount resolves the SETTLEMENT account for a currency,
// locking its row FOR UPDATE so the cross-border tx owns both legs.
func lockSettlementAccount(ctx context.Context, tx pgx.Tx, currency string) (*Account, error) {
	var a Account
	err := tx.QueryRow(ctx, `
		SELECT a.id, a.user_id, a.type, a.currency, a.created_at
		FROM accounts a
		WHERE a.type = 'SETTLEMENT' AND a.currency = $1
		ORDER BY a.created_at
		LIMIT 1
		FOR UPDATE`, currency,
	).Scan(&a.ID, &a.UserID, &a.Type, &a.Currency, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s (call EnsureSettlementAccounts)", ErrSettlementAccountNotFound, currency)
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: lock settlement account %s: %w", currency, err)
	}
	return &a, nil
}

// resolveFxRate returns the effective rate for a cross-border transfer: the
// explicit request rate if > 0, else the engine default. Zero in both cases
// is an error (ErrInvalidFxRate).
func (s *Service) resolveFxRate(requested float64) (float64, error) {
	rate := requested
	if rate <= 0 {
		rate = s.defaultFxRate
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, ErrInvalidFxRate
	}
	return rate, nil
}

// round4 rounds a money amount to 4 decimal places to match NUMERIC(18,4).
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
