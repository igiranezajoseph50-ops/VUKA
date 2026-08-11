# VUKA Phase 1 — Go Ledger Engine (Implementation Plan)

> **Status:** DRAFT — awaiting approval
> **Scope:** Backend engine only. All work lives under `ENGINE/` (the webapp/frontend folder is separate).
> **Phase 1 MVP target:** PostgreSQL double-entry ledger + Go transfer engine with full test coverage + one working (simulated) telecom adapter + end-to-end simulated transfer.

---

## 1. Goal

Build the VUKA Core Bridge & Ledger Engine (Go) exactly per the architecture spec:
- Immutable **double-entry ledger** in PostgreSQL — no `balance` column; balance = `SUM(ledger_entries.amount)`.
- **Idempotent transfers** via `Idempotency-Key` (UUIDv4) — duplicate charges impossible.
- **Business/Personal wallet isolation** (account types: PERSONAL, BUSINESS, SETTLEMENT, FEES).
- **Simulated MTN Rwanda adapter** — one working end-to-end simulated transfer (dispatch → callback → ledger commit).
- **Full unit + integration test coverage** (`go test -race`), `go vet` clean, `go build` clean.

## 2. Architecture Decisions (approve these)

| Decision | Choice | Why |
|---|---|---|
| HTTP framework | stdlib `net/http` + `http.ServeMux` (Go 1.22+ patterns) | Zero deps, production-grade; no framework lock-in |
| DB driver | `jackc/pgx/v5` (`pgxpool`) | Modern, fast, native PostgreSQL; no ORM — raw SQL keeps double-entry integrity explicit |
| Concurrency control | `SELECT ... FOR UPDATE` on the source account row before balance check + entry insert | Serializes transfers per account; prevents balance corruption under concurrent transfers (matches spec §3.2) |
| Idempotency | DB-backed `idempotency_keys` table with UNIQUE constraint, claimed inside the same DB transaction | Atomic with the transfer; no Redis dependency for MVP |
| Two-phase commit | Adapter dispatch happens *inside* the DB tx; entries written only on adapter SUCCESS; FAILED → no entries | Mirrors README sequence: dispatch → callback 200 OK → write entries → COMMIT |
| Telecom adapter | Interface `Payout(ctx, req) (result, err)` + registry; MTN Rwanda simulated impl with configurable success/timeout/failure | Phase 1 needs one working simulated rail (README Phase 1) |
| gRPC / ISO gateway | **Out of scope this phase** | Python ISO 20022 gateway is a separate workstream; adding a dead gRPC client now violates no-stubs rule |

## 3. File Layout (all under `ENGINE/`)

```
ENGINE/
├── PHASE1-TODO.md              ← this file
├── deploy/
│   ├── docker-compose.yml      # PostgreSQL 16 + adminer (dev)
│   └── init.sql                # full schema: enums, tables, indexes
└── go-bridge-core/
    ├── go.mod                  # module vuka/go-bridge-core
    ├── cmd/main.go             # entrypoint, config load, wiring, graceful shutdown
    └── internal/
        ├── config/config.go    # env-based config (DATABASE_URL, PORT, ADAPTER_MODE)
        ├── ledger/
        │   ├── models.go       # Account, Transaction, LedgerEntry structs + statuses
        │   ├── service.go      # Service: pool, CreateAccount, GetBalance, Transfer, Reverse
        │   ├── transfer.go     # core idempotent atomic transfer logic
        │   └── account.go      # account creation, balance computation
        ├── idempotency/
        │   └── store.go        # idempotency key claim/check (DB-backed)
        ├── adapters/
        │   ├── adapter.go      # TelecomAdapter interface + registry + PayoutRequest/Result
        │   └── mtn_rw.go       # simulated MTN Rwanda Mobile Money adapter
        └── api/
            ├── router.go       # mux, routes, CORS/logging/recovery middleware
            ├── accounts.go     # POST /api/accounts, GET /api/accounts/{id}/balance
            ├── transfers.go    # POST /api/transfers, GET /api/transfers/{id}, POST .../reverse
            └── respond.go      # JSON encode/decode, error mapping (422/404/409/500)
```

## 4. Database Schema (`deploy/init.sql`)

```sql
CREATE TYPE account_type AS ENUM ('PERSONAL', 'BUSINESS', 'SETTLEMENT', 'FEES');
CREATE TYPE transaction_status AS ENUM ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'REVERSED');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(150) NOT NULL,
    phone_number VARCHAR(30) UNIQUE NOT NULL,
    business_reg_number VARCHAR(100),
    kyc_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    type account_type NOT NULL,
    currency VARCHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, type, currency)
);

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    invoice_number VARCHAR(100),
    source_account_id UUID NOT NULL REFERENCES accounts(id),
    destination_account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18,4) NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    status transaction_status NOT NULL DEFAULT 'PENDING',
    external_reference VARCHAR(100),
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18,4) NOT NULL,          -- negative = debit, positive = credit
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    transaction_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_account_id ON ledger_entries(account_id);
CREATE INDEX idx_ledger_tx_id ON ledger_entries(transaction_id);
CREATE INDEX idx_transactions_source ON transactions(source_account_id);
CREATE INDEX idx_transactions_dest ON transactions(destination_account_id);
```

**Integrity rule enforced in Go + tests:** every transfer writes exactly 2 entries (debit + credit) summing to zero; accounts never carry a balance column.

## 5. Core Transfer Algorithm (inside a single DB tx)

1. **Idempotency:** `INSERT INTO idempotency_keys(key) VALUES($1) ON CONFLICT DO NOTHING` — if conflict, return the existing transaction (HTTP 200 replay) or 409.
2. **Lock source account:** `SELECT ... FROM accounts WHERE id=$1 FOR UPDATE`.
3. **Validate:** accounts exist, same currency, amount > 0, source is BUSINESS (invoice payments only from Business wallet — spec §4.1).
4. **Balance check:** `SELECT COALESCE(SUM(amount),0) FROM ledger_entries WHERE account_id=$1`; if `< amount` → 422 InsufficientFunds, mark tx FAILED.
5. **Dispatch to adapter** (simulated MTN payout): returns external reference or error.
6. **On success:** insert `transactions` (SUCCESS) + 2 `ledger_entries` (debit source BUSINESS, credit destination SETTLEMENT) → COMMIT.
7. **On adapter failure:** mark tx FAILED → ROLLBACK (no entries) → return error.

**Reversal:** `POST /api/transfers/{id}/reverse` — only for SUCCESS txs; writes contra entries (credit source, debit settlement), marks original REVERSED.

## 6. REST API

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/users` | Create user (auto-creates PERSONAL + BUSINESS accounts) |
| GET | `/api/accounts/{id}` | Account details |
| GET | `/api/accounts/{id}/balance` | Computed balance = SUM(ledger_entries) |
| POST | `/api/transfers` | Transfer; requires `Idempotency-Key` header; body: `{source_account_id, destination_account_id, amount, currency, invoice_number}` |
| GET | `/api/transfers/{id}` | Transaction status (PENDING → SUCCESS/FAILED) |
| POST | `/api/transfers/{id}/reverse` | Reverse a SUCCESS transfer |
| GET | `/healthz` | Liveness probe |

## 7. MTN Rwanda Simulated Adapter

- `Adapter.Payout(ctx, PayoutRequest{Amount, Currency, Phone, Reference}) (PayoutResult{ExternalRef}, error)`
- Env knobs: `MTN_SIM_DELAY_MS` (latency), `MTN_SIM_FAIL_RATE` (0.0–1.0 random failure), `MTN_SIM_MODE` = `success|fail|timeout`
- Returns deterministic external reference for tracing; log callback simulation.
- Interface + registry so M-Pesa Kenya can be added later without touching the engine.

## 8. Test Plan (`go test -race ./...`)

- `ledger/`: transfer happy path, duplicate idempotency key, insufficient funds, wrong currency, non-BUSINESS source, concurrent transfers (parallel goroutines, balance never negative), reversal, reversal idempotency.
- `idempotency/`: claim, conflict, replay returns same tx.
- `adapters/`: success/failure/timeout modes, external ref format.
- `api/`: httptest against real PG — full HTTP flows incl. missing Idempotency-Key → 400.
- Integration harness: `make test-integration` spins PG via docker-compose, applies init.sql to a `vuka_test` DB, runs suite.

## 9. Verification Steps (before done)

1. `go vet ./...` — clean
2. `go build ./...` — clean
3. `go test -race ./...` — all pass (against dockerized PG)
4. Live demo script: create user → fund business account → `curl` a transfer with Idempotency-Key → confirm SUCCESS + double-entry rows in PG → replay same key → confirm no duplicate.

## 10. Out of Scope (later phases)

- Python ISO 20022 gateway + gRPC bridge
- M-Pesa / Airtel / Equity adapters
- Trust score, webhooks, SSE/WebSocket push
- Frontend/webapp (separate folder)

---

## Approval

Review the decisions in §2 and schema in §4. Reply **approved** (or with changes) and I execute task-by-task, verifying `go vet` + `go build` + `go test -race` at every milestone.
