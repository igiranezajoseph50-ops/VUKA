-- ============================================================================
-- VUKA Phase 1 — PostgreSQL Double-Entry Ledger Schema (idempotent bootstrap)
--
-- This file is EMBEDDED into the Go engine and executed at startup
-- (ledger.Service.EnsureSchema). Unlike deploy/init.sql (which only runs in
-- the docker-compose first boot), this version guards every statement so it
-- can run on an EMPTY database (Render managed Postgres) OR an existing one
-- without errors. Keep it in sync with deploy/init.sql.
--
-- Immutable double-entry bookkeeping: accounts NEVER carry a balance column.
-- Balance = SUM(ledger_entries.amount) WHERE account_id = X
-- Every transfer writes exactly two entries (debit + credit) summing to zero.
-- ============================================================================

-- Enum types ----------------------------------------------------------------
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_type') THEN
        CREATE TYPE account_type AS ENUM ('PERSONAL', 'BUSINESS', 'SETTLEMENT', 'FEES');
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_status') THEN
        CREATE TYPE transaction_status AS ENUM ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'REVERSED');
    END IF;
END $$;

-- Users ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(150) NOT NULL,
    phone_number VARCHAR(30) UNIQUE NOT NULL,
    business_reg_number VARCHAR(100),
    kyc_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Accounts ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    type account_type NOT NULL,
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, type, currency)
);

-- Transactions (header record) ----------------------------------------------
CREATE TABLE IF NOT EXISTS transactions (
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
CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18,4) NOT NULL,          -- negative = debit, positive = credit
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotency keys ----------------------------------------------------------
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    transaction_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Invoices (Phase 3.5 — persisted invoice documents) ------------------------
-- An invoice is issued by one trader and billed to another (counterparty).
-- Its paid status is DERIVED from the ledger: PAID when a SUCCESS transfer
-- references the invoice number. Line items live in invoice_items.
CREATE TABLE IF NOT EXISTS invoices (
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

CREATE TABLE IF NOT EXISTS invoice_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description VARCHAR(255) NOT NULL,
    quantity NUMERIC(18,4) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(18,4) NOT NULL CHECK (unit_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes -------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_ledger_account_id ON ledger_entries(account_id);
CREATE INDEX IF NOT EXISTS idx_ledger_tx_id ON ledger_entries(transaction_id);
CREATE INDEX IF NOT EXISTS idx_transactions_source ON transactions(source_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_dest ON transactions(destination_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_idempotency ON transactions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_idempotency_tx ON idempotency_keys(transaction_id);
CREATE INDEX IF NOT EXISTS idx_invoices_issuer ON invoices(issuer_user_id);
CREATE INDEX IF NOT EXISTS idx_invoices_counterparty ON invoices(counterparty_user_id);
CREATE INDEX IF NOT EXISTS idx_invoice_items_invoice ON invoice_items(invoice_id);

-- Balance view for convenient reads (still derived from ledger_entries) ------
CREATE OR REPLACE VIEW account_balances AS
SELECT
    a.id AS account_id,
    a.user_id,
    a.type,
    a.currency,
    COALESCE(SUM(le.amount), 0) AS balance
FROM accounts a
LEFT JOIN ledger_entries le ON le.account_id = a.id
GROUP BY a.id, a.user_id, a.type, a.currency;