// Package ledger implements the VUKA double-entry accounting core.
//
// Golden rule: account rows NEVER carry a balance column. A balance is
// strictly defined as SUM(amount) FROM ledger_entries WHERE account_id = X.
// Every transfer writes exactly two linked ledger entries (a debit and a
// credit) whose values sum to zero. All mutations happen inside a single
// database transaction with the source account row locked FOR UPDATE, so
// concurrent transfers can never corrupt a balance.
package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/idempotency"
)

// AccountType mirrors the account_type PostgreSQL enum.
type AccountType string

const (
	AccountTypePersonal   AccountType = "PERSONAL"
	AccountTypeBusiness   AccountType = "BUSINESS"
	AccountTypeSettlement AccountType = "SETTLEMENT"
	AccountTypeFees       AccountType = "FEES"
)

// TransactionStatus mirrors the transaction_status PostgreSQL enum.
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "PENDING"
	StatusProcessing TransactionStatus = "PROCESSING"
	StatusSuccess    TransactionStatus = "SUCCESS"
	StatusFailed     TransactionStatus = "FAILED"
	StatusReversed   TransactionStatus = "REVERSED"
)

// Sentinels for idempotency replay semantics. The API layer maps these onto
// HTTP status codes; callers should use errors.Is.
var (
	// ErrDuplicateKey is returned when the caller retries an idempotency key
	// that has already been consumed by a prior transfer attempt.
	ErrDuplicateKey = errors.New("ledger: idempotency key already used")
	// ErrInsufficientFunds is returned when the source business account
	// cannot cover the requested amount.
	ErrInsufficientFunds = errors.New("ledger: insufficient funds")
	// ErrAccountNotFound is returned for unknown account UUIDs.
	ErrAccountNotFound = errors.New("ledger: account not found")
	// ErrUserNotFound is returned for unknown user UUIDs.
	ErrUserNotFound = errors.New("ledger: user not found")
	// ErrCurrencyMismatch is returned when accounts involved in a transfer
	// have different currencies.
	ErrCurrencyMismatch = errors.New("ledger: currency mismatch")
	// ErrInvalidAmount is returned for non-positive or non-finite amounts.
	ErrInvalidAmount = errors.New("ledger: invalid amount")
	// ErrInvalidAccountType is returned when a transfer source is not a
	// BUSINESS account. Invoice payments can ONLY be funded from the
	// Business wallet (spec §4.1 account isolation).
	ErrInvalidAccountType = errors.New("ledger: transfers must originate from a BUSINESS account")
	// ErrSameAccount is returned when source and destination are identical.
	ErrSameAccount = errors.New("ledger: source and destination accounts must differ")
	// ErrTransferNotSuccess is returned when reversing a transfer that is not
	// in SUCCESS state.
	ErrTransferNotSuccess = errors.New("ledger: only SUCCESS transfers can be reversed")
	// ErrTransferNotFound is returned for unknown transaction UUIDs.
	ErrTransferNotFound = errors.New("ledger: transfer not found")
	// ErrAdapterUnavailable is returned when the configured telecom adapter
	// cannot be resolved from the registry.
	ErrAdapterUnavailable = errors.New("ledger: telecom adapter unavailable")
	// ErrInvalidFxRate is returned when a cross-border FX rate is missing,
	// non-positive, or not finite.
	ErrInvalidFxRate = errors.New("ledger: fx rate must be positive")
	// ErrSettlementAccountNotFound is returned when no SETTLEMENT account
	// exists for a required cross-border currency (call EnsureSettlementAccounts).
	ErrSettlementAccountNotFound = errors.New("ledger: settlement account not found")
	// ErrIdempotencyKeyRequired is returned when a transfer/fund is called
	// without an Idempotency-Key. Every money-moving operation requires one.
	ErrIdempotencyKeyRequired = errors.New("ledger: idempotency key is required")
)

// User is the trader record. KYC state is tracked here per spec.
type User struct {
	ID                string    `json:"id"`
	FullName          string    `json:"full_name"`
	PhoneNumber       string    `json:"phone_number"`
	BusinessRegNumber *string   `json:"business_reg_number,omitempty"`
	KYCStatus         string    `json:"kyc_status"`
	CreatedAt         time.Time `json:"created_at"`
}

// Account is a wallet sub-account. It intentionally has no Balance field:
// balances are always derived from ledger entries.
type Account struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Type      AccountType `json:"type"`
	Currency  string      `json:"currency"`
	CreatedAt time.Time   `json:"created_at"`
}

// Balance is a point-in-time derived balance for an account.
type Balance struct {
	AccountID string      `json:"account_id"`
	Type      AccountType `json:"type"`
	Currency  string      `json:"currency"`
	Amount    float64     `json:"amount"`
}

// Transfer is the header record for a money movement. The double-entry rows
// live in ledger_entries and are not denormalised here.
type Transfer struct {
	ID                   string            `json:"id"`
	IdempotencyKey       string            `json:"idempotency_key"`
	InvoiceNumber        *string           `json:"invoice_number,omitempty"`
	SourceAccountID      string            `json:"source_account_id"`
	DestinationAccountID string            `json:"destination_account_id"`
	Amount               float64           `json:"amount"`
	Currency             string            `json:"currency"`
	FxRate               *float64          `json:"fx_rate,omitempty"`
	Status               TransactionStatus `json:"status"`
	ExternalReference    *string           `json:"external_reference,omitempty"`
	FailureReason        *string           `json:"failure_reason,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// LedgerEntry is one immutable row of the double-entry log. Negative amounts
// are debits, positive amounts are credits. Entries are never updated.
type LedgerEntry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	Amount        float64   `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateUserRequest is the payload for registering a trader.
type CreateUserRequest struct {
	FullName          string `json:"full_name"`
	PhoneNumber       string `json:"phone_number"`
	BusinessRegNumber string `json:"business_reg_number,omitempty"`
}

// TransferRequest is the payload for initiating an invoice-linked transfer.
// The Idempotency-Key travels in the HTTP header (API layer), not here.
type TransferRequest struct {
	SourceAccountID      string  `json:"source_account_id"`
	DestinationAccountID string  `json:"destination_account_id"`
	Amount               float64 `json:"amount"`
	Currency             string  `json:"currency"`
	InvoiceNumber        string  `json:"invoice_number,omitempty"`
}

// ReverseRequest optionally carries a human-readable reason for reversal.
type ReverseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CrossBorderRequest is the payload for a cross-currency (RWF -> KES)
// transfer through the FX settlement account. amount is denominated in
// currency_from; the destination credit is amount / fx_rate in currency_to.
type CrossBorderRequest struct {
	SourceAccountID      string  `json:"source_account_id"`
	DestinationAccountID string  `json:"destination_account_id"`
	Amount               float64 `json:"amount"`
	CurrencyFrom         string  `json:"currency_from"`
	CurrencyTo           string  `json:"currency_to"`
	FxRate               float64 `json:"fx_rate,omitempty"` // 0 = use engine default
	InvoiceNumber        string  `json:"invoice_number,omitempty"`
}

// InvoiceItem is one line of an invoice document.
type InvoiceItem struct {
	ID          string  `json:"id"`
	InvoiceID   string  `json:"invoice_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	CreatedAt   time.Time `json:"created_at"`
}

// Invoice is a persisted invoice document. Status is DERIVED from the ledger:
// PAID when a SUCCESS transfer references number; DRAFT/ISSUED otherwise.
type Invoice struct {
	ID                 string    `json:"id"`
	Number             string    `json:"number"`
	IssuerUserID       string    `json:"issuer_user_id"`
	CounterpartyUserID string    `json:"counterparty_user_id"`
	Currency           string    `json:"currency"`
	IssueDate          string    `json:"issue_date"`
	DueDate            string    `json:"due_date"`
	VATRate            float64   `json:"vat_rate"`
	Terms              *string   `json:"terms,omitempty"`
	Notes              *string   `json:"notes,omitempty"`
	Status             string    `json:"status"` // PAID | ISSUED
	Items              []*InvoiceItem `json:"items"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// InvoiceItemRequest is a line item payload for invoice creation.
type InvoiceItemRequest struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

// CreateInvoiceRequest is the payload for issuing an invoice.
type CreateInvoiceRequest struct {
	Number             string               `json:"number"`
	CounterpartyUserID string               `json:"counterparty_user_id"`
	Currency           string               `json:"currency"`
	IssueDate          string               `json:"issue_date"` // YYYY-MM-DD
	DueDate            string               `json:"due_date"`   // YYYY-MM-DD
	VATRate            float64              `json:"vat_rate"`
	Terms              string               `json:"terms,omitempty"`
	Notes              string               `json:"notes,omitempty"`
	Items              []InvoiceItemRequest `json:"items"`
}

// Service orchestrates the ledger. It owns the connection pool, the
// idempotency store, and the telecom adapter registry.
type Service struct {
	pool     *pgxpool.Pool
	idem     *idempotency.Store
	registry *adapters.Registry
	// defaultFxRate is used for cross-border transfers when the request does
	// not carry an explicit rate. Zero means "require an explicit rate".
	defaultFxRate float64
}

// NewService wires a Service around the given pool, idempotency store and
// adapter registry. Callers are responsible for closing the pool.
func NewService(pool *pgxpool.Pool, idem *idempotency.Store, registry *adapters.Registry) *Service {
	return &Service{pool: pool, idem: idem, registry: registry}
}

// SetDefaultFxRate sets the engine-wide FX rate used when a cross-border
// request omits its own. Set to 0 to require an explicit rate per request.
func (s *Service) SetDefaultFxRate(rate float64) {
	s.defaultFxRate = rate
}

// DefaultFxRate returns the engine-wide FX rate (0 when unset). Clients use
// this to render the live corridor rate without hardcoding it.
func (s *Service) DefaultFxRate() float64 {
	return s.defaultFxRate
}

// CreateUser inserts a trader and automatically creates their PERSONAL and
// BUSINESS sub-accounts in the requested currency (spec §3.1 wallet
// separation). Account creation is atomic: a failure leaves no orphan rows.
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest, currency string) (*User, []*Account, error) {
	if req.FullName == "" || req.PhoneNumber == "" {
		return nil, nil, fmt.Errorf("%w: full_name and phone_number are required", ErrInvalidAmount)
	}
	if currency == "" {
		currency = "RWF"
	}
	if !validCurrency(currency) {
		return nil, nil, fmt.Errorf("ledger: invalid currency %q", currency)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("ledger: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	var user User
	var businessReg *string
	if req.BusinessRegNumber != "" {
		businessReg = &req.BusinessRegNumber
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO users (full_name, phone_number, business_reg_number, kyc_status)
		VALUES ($1, $2, $3, 'PENDING')
		RETURNING id, full_name, phone_number, business_reg_number, kyc_status, created_at`,
		req.FullName, req.PhoneNumber, businessReg,
	).Scan(&user.ID, &user.FullName, &user.PhoneNumber, &user.BusinessRegNumber, &user.KYCStatus, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, fmt.Errorf("ledger: phone number already registered")
		}
		return nil, nil, fmt.Errorf("ledger: insert user: %w", err)
	}

	accounts := make([]*Account, 0, 2)
	for _, at := range []AccountType{AccountTypePersonal, AccountTypeBusiness} {
		acc, err := insertAccount(ctx, tx, user.ID, at, currency)
		if err != nil {
			return nil, nil, err
		}
		accounts = append(accounts, acc)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("ledger: commit user creation: %w", err)
	}
	return &user, accounts, nil
}

// insertAccount inserts a single account row within the caller's transaction.
func insertAccount(ctx context.Context, tx pgx.Tx, userID string, at AccountType, currency string) (*Account, error) {
	var acc Account
	err := tx.QueryRow(ctx, `
		INSERT INTO accounts (user_id, type, currency)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, type, currency, created_at`,
		userID, at, currency,
	).Scan(&acc.ID, &acc.UserID, &acc.Type, &acc.Currency, &acc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ledger: insert account %s: %w", at, err)
	}
	return &acc, nil
}

// GetUser fetches a trader record by UUID.
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, phone_number, business_reg_number, kyc_status, created_at
		FROM users WHERE id = $1`, userID,
	).Scan(&u.ID, &u.FullName, &u.PhoneNumber, &u.BusinessRegNumber, &u.KYCStatus, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get user: %w", err)
	}
	return &u, nil
}

// GetUserByPhone resolves a trader by phone number. Used by the dashboard's
// demo-mode trader picker to re-enter an existing demo trader instead of
// failing on the unique phone constraint.
func (s *Service) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
	if phone == "" {
		return nil, fmt.Errorf("%w: phone_number is required", ErrUserNotFound)
	}
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, full_name, phone_number, business_reg_number, kyc_status, created_at
		FROM users WHERE phone_number = $1`, phone,
	).Scan(&u.ID, &u.FullName, &u.PhoneNumber, &u.BusinessRegNumber, &u.KYCStatus, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get user by phone: %w", err)
	}
	return &u, nil
}

// GetAccount fetches a single account by UUID.
func (s *Service) GetAccount(ctx context.Context, accountID string) (*Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, type, currency, created_at
		FROM accounts WHERE id = $1`, accountID,
	).Scan(&a.ID, &a.UserID, &a.Type, &a.Currency, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get account: %w", err)
	}
	return &a, nil
}

// ListAccounts returns all accounts for a user, ordered by creation time.
func (s *Service) ListAccounts(ctx context.Context, userID string) ([]*Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, type, currency, created_at
		FROM accounts WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("ledger: list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.UserID, &a.Type, &a.Currency, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("ledger: scan account: %w", err)
		}
		accounts = append(accounts, &a)
	}
	return accounts, rows.Err()
}

// GetBalance derives an account balance as SUM(ledger_entries.amount). The
// account row itself never stores a balance (golden rule).
func (s *Service) GetBalance(ctx context.Context, accountID string) (*Balance, error) {
	var b Balance
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.type, a.currency, COALESCE(SUM(le.amount), 0) AS balance
		FROM accounts a
		LEFT JOIN ledger_entries le ON le.account_id = a.id
		WHERE a.id = $1
		GROUP BY a.id, a.type, a.currency`, accountID,
	).Scan(&b.AccountID, &b.Type, &b.Currency, &b.Amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get balance: %w", err)
	}
	return &b, nil
}

// GetTransfer fetches a transaction header by UUID.
func (s *Service) GetTransfer(ctx context.Context, transferID string) (*Transfer, error) {
	var t Transfer
	err := s.pool.QueryRow(ctx, `
		SELECT id, idempotency_key, invoice_number, source_account_id,
		       destination_account_id, amount, currency, fx_rate, status,
		       external_reference, failure_reason, created_at, updated_at
		FROM transactions WHERE id = $1`, transferID,
	).Scan(&t.ID, &t.IdempotencyKey, &t.InvoiceNumber, &t.SourceAccountID,
		&t.DestinationAccountID, &t.Amount, &t.Currency, &t.FxRate, &t.Status,
		&t.ExternalReference, &t.FailureReason, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get transfer: %w", err)
	}
	return &t, nil
}

// GetTransferByIdempotencyKey resolves a transfer by its idempotency key.
// Used when a concurrent request loses the claim race: the loser must
// replay the winner's transfer rather than charge twice.
func (s *Service) GetTransferByIdempotencyKey(ctx context.Context, key string) (*Transfer, error) {
	var t Transfer
	err := s.pool.QueryRow(ctx, `
		SELECT id, idempotency_key, invoice_number, source_account_id,
		       destination_account_id, amount, currency, fx_rate, status,
		       external_reference, failure_reason, created_at, updated_at
		FROM transactions WHERE idempotency_key = $1`, key,
	).Scan(&t.ID, &t.IdempotencyKey, &t.InvoiceNumber, &t.SourceAccountID,
		&t.DestinationAccountID, &t.Amount, &t.Currency, &t.FxRate, &t.Status,
		&t.ExternalReference, &t.FailureReason, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: get transfer by idempotency key: %w", err)
	}
	return &t, nil
}

// ListLedgerEntries returns the immutable double-entry rows for a transfer.
// Useful for audit and the demo verification step.
func (s *Service) ListLedgerEntries(ctx context.Context, transferID string) ([]*LedgerEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, transaction_id, account_id, amount, created_at
		FROM ledger_entries WHERE transaction_id = $1 ORDER BY created_at`, transferID)
	if err != nil {
		return nil, fmt.Errorf("ledger: list entries: %w", err)
	}
	defer rows.Close()

	var entries []*LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("ledger: scan entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// Ping verifies database connectivity. Used by the healthz endpoint.
func (s *Service) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// validCurrency enforces the ISO-4217 shape used across the schema.
func validCurrency(c string) bool {
	if len(c) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if c[i] < 'A' || c[i] > 'Z' {
			return false
		}
	}
	return true
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation
// (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isForeignKeyViolation reports whether err is a PostgreSQL
// foreign_key_violation (SQLSTATE 23503).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
