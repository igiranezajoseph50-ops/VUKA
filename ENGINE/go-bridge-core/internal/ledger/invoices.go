// Invoice persistence (Phase 3.5).
//
// Invoices are real documents stored in PostgreSQL with line items. The paid
// status is DERIVED from the ledger — an invoice is PAID when a SUCCESS
// transfer references its number — so the invoice always reflects actual
// settlement, never a denormalised flag that could drift.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrInvoiceNotFound is returned for unknown invoice UUIDs/numbers.
var ErrInvoiceNotFound = errors.New("ledger: invoice not found")

// ErrInvalidInvoiceNumber is returned when an invoice number does not match
// the VUKA format INV-<year>-<sequence> (e.g. INV-2026-00001).
var ErrInvalidInvoiceNumber = errors.New("ledger: invoice number must match INV-YYYY-NNN (e.g. INV-2026-00001)")

// invoiceNumberRe enforces the VUKA invoice-number format: INV-<year>-<seq>,
// e.g. INV-2026-00001. Sequential numbering is a regulatory requirement for
// commercial invoices; the engine rejects anything that cannot be one.
var invoiceNumberRe = regexp.MustCompile(`^INV-\d{4}-\d{3,6}$`)

// InvoiceReference is a distinct invoice number referenced by the transfer
// register, with the settled amount and currency derived from the ledger.
// CrossBorder reports whether the reference settled over the cross-border
// corridor (fx_rate set) — the invoice currencies differ per leg then.
type InvoiceReference struct {
	Number      string
	Amount      float64
	Currency    string
	CrossBorder bool
	FxRate      float64
}

// ListInvoiceReferences returns the distinct invoice numbers that SUCCESS
// transfers reference, so the dashboard can show the documents behind the
// paid register. Amount is the largest settled amount for that number.
func (s *Service) ListInvoiceReferences(ctx context.Context) ([]InvoiceReference, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT invoice_number, max(amount)::float8 AS amount, max(currency) AS currency,
		       (bool_or(fx_rate IS NOT NULL AND fx_rate > 0)) AS cross_border,
		       COALESCE(max(fx_rate), 0)::float8 AS fx_rate
		FROM transactions
		WHERE invoice_number IS NOT NULL
		GROUP BY invoice_number
		ORDER BY invoice_number`)
	if err != nil {
		return nil, fmt.Errorf("ledger: query invoice references: %w", err)
	}
	defer rows.Close()

	var refs []InvoiceReference
	for rows.Next() {
		var r InvoiceReference
		if err := rows.Scan(&r.Number, &r.Amount, &r.Currency, &r.CrossBorder, &r.FxRate); err != nil {
			return nil, fmt.Errorf("ledger: scan invoice reference: %w", err)
		}
		refs = append(refs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: iterate invoice references: %w", err)
	}
	return refs, nil
}

// Invoice statuses (derived).
const (
	InvoiceStatusIssued = "ISSUED"
	InvoiceStatusPaid   = "PAID"
)

// validDate enforces the YYYY-MM-DD shape used by invoice dates.
func validDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// CreateInvoice persists an invoice with its line items (single tx).
// issuerUserID is the trader issuing the document; the counterparty must
// exist. Number must be unique.
func (s *Service) CreateInvoice(ctx context.Context, issuerUserID string, req CreateInvoiceRequest) (*Invoice, error) {
	req.Number = strings.TrimSpace(req.Number)
	if req.Number == "" {
		return nil, fmt.Errorf("ledger: invoice number is required")
	}
	if !invoiceNumberRe.MatchString(req.Number) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidInvoiceNumber, req.Number)
	}
	if len(req.Number) > 100 {
		return nil, fmt.Errorf("ledger: invoice number is required (max 100 chars)")
	}
	if req.CounterpartyUserID == "" {
		return nil, fmt.Errorf("ledger: counterparty_user_id is required")
	}
	if !validCurrency(req.Currency) {
		return nil, fmt.Errorf("ledger: invalid currency %q", req.Currency)
	}
	if !validDate(req.IssueDate) || !validDate(req.DueDate) {
		return nil, fmt.Errorf("ledger: invoice dates must be YYYY-MM-DD")
	}
	if req.VATRate < 0 || req.VATRate > 100 {
		return nil, fmt.Errorf("ledger: vat_rate must be within [0, 100]")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("ledger: invoice requires at least one line item")
	}
	for _, it := range req.Items {
		if strings.TrimSpace(it.Description) == "" {
			return nil, fmt.Errorf("ledger: line item description is required")
		}
		if it.Quantity <= 0 || it.UnitPrice < 0 {
			return nil, fmt.Errorf("ledger: line item quantity must be > 0 and unit_price >= 0")
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ledger: begin invoice tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var invID string
	err = tx.QueryRow(ctx, `
		INSERT INTO invoices (number, issuer_user_id, counterparty_user_id, currency,
		                      issue_date, due_date, vat_rate, terms, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		req.Number, issuerUserID, req.CounterpartyUserID, req.Currency,
		req.IssueDate, req.DueDate, req.VATRate, nullIfEmpty(req.Terms), nullIfEmpty(req.Notes),
	).Scan(&invID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("ledger: invoice number %q already exists", req.Number)
		}
		if isForeignKeyViolation(err) {
			return nil, fmt.Errorf("%w: issuer or counterparty user not found", ErrUserNotFound)
		}
		return nil, fmt.Errorf("ledger: insert invoice: %w", err)
	}

	for _, it := range req.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO invoice_items (invoice_id, description, quantity, unit_price)
			VALUES ($1, $2, $3, $4)`,
			invID, strings.TrimSpace(it.Description), it.Quantity, it.UnitPrice,
		); err != nil {
			return nil, fmt.Errorf("ledger: insert invoice item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ledger: commit invoice: %w", err)
	}
	return s.GetInvoice(ctx, invID)
}

// GetInvoice fetches an invoice by id, including line items and derived status.
func (s *Service) GetInvoice(ctx context.Context, invoiceID string) (*Invoice, error) {
	inv, err := s.scanInvoice(ctx, `WHERE i.id = $1`, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInvoiceNotFound
	}
	return inv, nil
}

// GetInvoiceByNumber resolves an invoice by its unique number.
func (s *Service) GetInvoiceByNumber(ctx context.Context, number string) (*Invoice, error) {
	inv, err := s.scanInvoice(ctx, `WHERE i.number = $1`, number)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInvoiceNotFound
	}
	return inv, nil
}

// ListInvoices returns invoices where the user is issuer OR counterparty,
// newest first. Direction ('issued' | 'received' | ”) filters accordingly.
func (s *Service) ListInvoices(ctx context.Context, userID string, direction string) ([]*Invoice, error) {
	var where string
	switch direction {
	case "issued":
		where = "WHERE i.issuer_user_id = $1"
	case "received":
		where = "WHERE i.counterparty_user_id = $1"
	default:
		where = "WHERE i.issuer_user_id = $1 OR i.counterparty_user_id = $1"
	}
	query := fmt.Sprintf(`
		SELECT i.id, i.number, i.issuer_user_id, i.counterparty_user_id, i.currency,
		       to_char(i.issue_date, 'YYYY-MM-DD'), to_char(i.due_date, 'YYYY-MM-DD'),
		       i.vat_rate, i.terms, i.notes, i.created_at, i.updated_at,
		       EXISTS(SELECT 1 FROM transactions t
		              WHERE t.invoice_number = i.number AND t.status = 'SUCCESS') AS paid
		FROM invoices i
		%s
		ORDER BY i.created_at DESC, i.id DESC`, where)

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger: list invoices: %w", err)
	}
	defer rows.Close()

	var out []*Invoice
	for rows.Next() {
		inv, err := scanInvoiceRow(rows)
		if err != nil {
			return nil, err
		}
		if err := s.attachItems(ctx, inv); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger: list invoices rows: %w", err)
	}
	return out, nil
}

// scanInvoice runs a full invoice query with the given WHERE clause/args.
func (s *Service) scanInvoice(ctx context.Context, where string, args ...any) (*Invoice, error) {
	query := fmt.Sprintf(`
		SELECT i.id, i.number, i.issuer_user_id, i.counterparty_user_id, i.currency,
		       to_char(i.issue_date, 'YYYY-MM-DD'), to_char(i.due_date, 'YYYY-MM-DD'),
		       i.vat_rate, i.terms, i.notes, i.created_at, i.updated_at,
		       EXISTS(SELECT 1 FROM transactions t
		              WHERE t.invoice_number = i.number AND t.status = 'SUCCESS') AS paid
		FROM invoices i
		%s`, where)

	row := s.pool.QueryRow(ctx, query, args...)
	inv, err := scanInvoiceRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: scan invoice: %w", err)
	}
	if err := s.attachItems(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// attachItems loads the line items for an invoice.
func (s *Service) attachItems(ctx context.Context, inv *Invoice) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, invoice_id, description, quantity, unit_price, created_at
		FROM invoice_items WHERE invoice_id = $1 ORDER BY created_at, id`, inv.ID)
	if err != nil {
		return fmt.Errorf("ledger: list invoice items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it InvoiceItem
		if err := rows.Scan(&it.ID, &it.InvoiceID, &it.Description, &it.Quantity, &it.UnitPrice, &it.CreatedAt); err != nil {
			return fmt.Errorf("ledger: scan invoice item: %w", err)
		}
		inv.Items = append(inv.Items, &it)
	}
	return rows.Err()
}

// rowScanner is satisfied by both *pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanInvoiceRow maps one invoice result row (with the paid subquery) into an
// Invoice. Works for both single-row and multi-row queries.
func scanInvoiceRow(row rowScanner) (*Invoice, error) {
	var inv Invoice
	var terms, notes *string
	var paid bool
	if err := row.Scan(&inv.ID, &inv.Number, &inv.IssuerUserID, &inv.CounterpartyUserID,
		&inv.Currency, &inv.IssueDate, &inv.DueDate, &inv.VATRate,
		&terms, &notes, &inv.CreatedAt, &inv.UpdatedAt, &paid); err != nil {
		return nil, err
	}
	inv.Terms = terms
	inv.Notes = notes
	if paid {
		inv.Status = InvoiceStatusPaid
	} else {
		inv.Status = InvoiceStatusIssued
	}
	return &inv, nil
}
