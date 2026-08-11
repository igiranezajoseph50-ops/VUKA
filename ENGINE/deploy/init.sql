-- ============================================================================
-- VUKA Phase 1 — PostgreSQL Double-Entry Ledger Schema
-- Immutable double-entry bookkeeping: accounts NEVER carry a balance column.
-- Balance = SUM(ledger_entries.amount) WHERE account_id = X
-- Every transfer writes exactly two entries (debit + credit) summing to zero.
-- ============================================================================

-- Enum types ----------------------------------------------------------------
CREATE TYPE account_type AS ENUM ('PERSONAL', 'BUSINESS', 'SETTLEMENT', 'FEES');
CREATE TYPE transaction_status AS ENUM ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'REVERSED');

-- Users ---------------------------------------------------------------------
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(150) NOT NULL,
    phone_number VARCHAR(30) UNIQUE NOT NULL,
    business_reg_number VARCHAR(100),
    kyc_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Accounts ------------------------------------------------------------------
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    type account_type NOT NULL,
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, type, currency)
);

-- Transactions (header record) ----------------------------------------------
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    invoice_number VARCHAR(100),
    source_account_id UUID NOT NULL REFERENCES accounts(id),
    destination_account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18,4) NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    fx_rate NUMERIC(18,6),              -- set only for cross-border transfers
    status transaction_status NOT NULL DEFAULT 'PENDING',
    external_reference VARCHAR(100),
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ledger entries (immutable double-entry log) -------------------------------
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18,4) NOT NULL,          -- negative = debit, positive = credit
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotency keys ----------------------------------------------------------
CREATE TABLE idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    transaction_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Invoices (Phase 3.5 — persisted invoice documents) ------------------------
-- An invoice is issued by one trader and billed to another (counterparty).
-- Its paid status is DERIVED from the ledger: PAID when a SUCCESS transfer
-- references the invoice number. Line items live in invoice_items.
CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number VARCHAR(100) UNIQUE NOT NULL,
    issuer_user_id UUID NOT NULL REFERENCES users(id),
    counterparty_user_id UUID NOT NULL REFERENCES users(id),
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    issue_date DATE NOT NULL,
    due_date DATE NOT NULL,
    vat_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
    terms TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (due_date >= issue_date)
);

CREATE TABLE invoice_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description VARCHAR(255) NOT NULL,
    quantity NUMERIC(18,4) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18,4) NOT NULL CHECK (unit_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes -------------------------------------------------------------------
CREATE INDEX idx_ledger_account_id ON ledger_entries(account_id);
CREATE INDEX idx_ledger_tx_id ON ledger_entries(transaction_id);
CREATE INDEX idx_transactions_source ON transactions(source_account_id);
CREATE INDEX idx_transactions_dest ON transactions(destination_account_id);
CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX idx_idempotency_tx ON idempotency_keys(transaction_id);
CREATE INDEX idx_invoices_issuer ON invoices(issuer_user_id);
CREATE INDEX idx_invoices_counterparty ON invoices(counterparty_user_id);
CREATE INDEX idx_invoice_items_invoice ON invoice_items(invoice_id);

-- Balance view for convenient reads (still derived from ledger_entries) ------
CREATE VIEW account_balances AS
SELECT
    a.id AS account_id,
    a.user_id,
    a.type,
    a.currency,
    COALESCE(SUM(le.amount), 0) AS balance
FROM accounts a
LEFT JOIN ledger_entries le ON le.account_id = a.id
GROUP BY a.id, a.user_id, a.type, a.currency;
