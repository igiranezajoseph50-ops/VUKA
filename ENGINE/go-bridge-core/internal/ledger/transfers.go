// Transfer history queries (Phase 3 — trader dashboard).
//
// Adds the list endpoint the dashboard needs: all transfers touching any of
// a trader's accounts, newest first, with optional account/status filters.
package ledger

import (
	"context"
	"fmt"
	"strings"
)

// TransferFilter narrows a transfer history query.
type TransferFilter struct {
	// UserID is required: only transfers touching this trader's accounts.
	UserID string
	// AccountID, when set, restricts to transfers involving that account
	// (as source OR destination).
	AccountID string
	// Status, when set, restricts to a single transaction status.
	Status TransactionStatus
}

// ListTransfers returns the trader's transfer history, newest first.
//
// A transfer "belongs" to the user when either leg touches one of their
// accounts (source or destination). Filters compose: account + status.
func (s *Service) ListTransfers(ctx context.Context, f TransferFilter) ([]*Transfer, error) {
	if f.UserID == "" {
		return nil, fmt.Errorf("ledger: user_id is required for transfer history")
	}

	// Build WHERE clauses conditionally. Passing '' into UUID/status
	// comparisons makes Postgres throw "invalid input syntax for type uuid",
	// so we only append a clause when the filter is actually set.
	where := []string{
		"(source_account_id IN (SELECT id FROM accounts WHERE user_id = $1) " +
			"OR destination_account_id IN (SELECT id FROM accounts WHERE user_id = $1))",
	}
	args := []any{f.UserID}

	if f.AccountID != "" {
		args = append(args, f.AccountID)
		where = append(where, fmt.Sprintf("(source_account_id = $%d OR destination_account_id = $%d)", len(args), len(args)))
	}
	if f.Status != "" {
		args = append(args, string(f.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}

	query := `
		SELECT id, idempotency_key, invoice_number, source_account_id,
		       destination_account_id, amount, currency, fx_rate, status,
		       external_reference, failure_reason, created_at, updated_at
		FROM transactions
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, id DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ledger: list transfers: %w", err)
	}
	defer rows.Close()

	var out []*Transfer
	for rows.Next() {
		var t Transfer
		if err := rows.Scan(&t.ID, &t.IdempotencyKey, &t.InvoiceNumber, &t.SourceAccountID,
			&t.DestinationAccountID, &t.Amount, &t.Currency, &t.FxRate, &t.Status,
			&t.ExternalReference, &t.FailureReason, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("ledger: scan transfer: %w", err)
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: list transfers rows: %w", err)
	}
	return out, nil
}