# VUKA Phase 2 — Python ISO 20022 Gateway + Cross-Border Bridge (Implementation Plan)

> **Status:** DRAFT — awaiting approval
> **Scope:** Adds the messaging gateway and the Rwanda→Kenya corridor to the Phase 1 Go engine.
> **Phase 2 MVP target:** Python (FastAPI) ISO 20022 gateway ↔ gRPC ↔ Go ledger engine, a working
> cross-currency (RWF→KES) transfer with FX settlement legs, a Kenya rail adapter (M-Pesa), and a
> live end-to-end demo: **pacs.008 XML in → ledger entries out → pacs.008/pacs.002 response**.

---

## 1. Goal

Build the second tier of the README architecture (`§2`, `§4`, `§6`):

- **Python messaging gateway** that parses inbound ISO 20022 XML (`pacs.008.001.10` credit transfers)
  into internal JSON and serializes internal results back to ISO 20022.
- **gRPC bridge** (protobuf contract) between Python and Go — Go exposes the Phase 1 ledger engine as
  a gRPC *server*; Python is the *client*. HTTP REST stays untouched for the trader dashboard.
- **Cross-border transfer** in the Go engine: a single idempotent operation that moves value
  RWF (Rwanda importer business wallet) → KES (Kenya supplier business wallet) through a
  settlement/FX account, writing **two balanced legs (4 entries)** — the Phase 1 rule
  "exactly 2 entries per transfer" is extended to "2 entries per currency leg, each leg sums to zero".
- **Kenya rail adapter**: simulated M-Pesa (Safaricom) adapter, registered alongside MTN Rwanda —
  corridor is now Rwanda→Kenya.
- **Verification:** `go vet`/`go build`/`go test -race` clean, `pytest` green, and a live demo:
  submit a real `pacs.008` XML payment, watch it execute through gRPC into the ledger, then return
  an ISO 20022 response.

## 2. Architecture Decisions (approve these)

| # | Decision | Choice | Why |
|---|---|---|---|
| D1 | gRPC direction | **Go = gRPC server** (port `:50051`), **Python = gRPC client** | Go owns the ledger and must be the authority; the README's `internal/grpc` "client" wording is loose — the gateway consumes the engine, never the reverse. |
| D2 | Protobuf contract | Single `proto/vuka.proto` checked into `ENGINE/`; Go + Python generate from it | One source of truth; both langs already need `protoc` toolchains (standard grpc-go + grpcio). |
| D3 | Cross-currency model | **FX settlement account**: debit RWF business, credit RWF settlement; debit KES settlement, credit KES business. 4 entries, 2 legs, each leg sums to zero | Mirrors real bank FX; keeps double-entry immutability; the SETTLEMENT account type already exists in `init.sql`. |
| D4 | FX rate source | Explicit `fx_rate` field on the cross-border request; default from env `VUKA_FX_RWF_KES` if absent | Hackathon demo uses a fixed configurable rate; a live FX API is a noted future hook, not a Phase 2 dependency. |
| D5 | ISO 20022 scope | **pacs.008.001.10** (customer credit transfer) inbound + outbound; **pacs.002** status response | The README names pacs.008 specifically (§3, §6); pacs.002 is the natural reply. No camt/pain this phase. |
| D6 | XML parsing/serialization | `lxml` for parse; template-driven builder for serialize; structural validation (fields, currency, amounts) — **no full XSD engine** | Full XSD validation adds heavy deps and schemas for a hackathon; structural validation covers the demo and rejects malformed input. |
| D7 | M-Pesa adapter | Simulated `mpesa-ke` adapter in Go, same interface/registry as MTN (`Payout(ctx, req)`) | Registry was designed for this: `reg.Register()` is adapter-agnostic. Kenya rail is simulated exactly like MTN Phase 1. |
| D8 | Cross-border endpoint | New Go service method `CrossBorderTransfer(ctx, key, req)` + gRPC method; **REST unchanged** | Keeps Phase 1 REST surface stable; cross-border is a gateway-facing capability. |

**Deliberate non-decisions (carry from Phase 1):**
- No DB-level double-entry constraint — still app-enforced via `writeDoubleEntry` (Phase 1 decision).
- No Redis — idempotency stays DB-backed in the same transaction.
- No webhooks/SSE/WebSocket push this phase (README lists them; scope is the corridor itself).

## 3. File Layout (all under `ENGINE/`)

```
ENGINE/
├── PHASE2-TODO.md              ← this file
├── proto/
│   └── vuka.proto              # gRPC contract (shared, single source of truth)
├── go-bridge-core/
│   ├── cmd/main.go             # MODIFIED: dual listener HTTP :8080 + gRPC :50051
│   └── internal/
│       ├── grpc/
│       │   ├── server.go       # gRPC server wrapping ledger.Service
│       │   └── server_test.go  # gRPC client → server integration tests
│       ├── ledger/
│       │   ├── fx.go           # FX rate lookup + settlement leg helpers (NEW)
│       │   └── crossborder.go  # CrossBorderTransfer: 2-leg idempotent op (NEW)
│       └── adapters/
│           └── mpesa_ke.go     # simulated Safaricom M-Pesa adapter (NEW)
└── python-iso-gateway/
    ├── pyproject.toml          # fastapi, grpcio, lxml, uvicorn, pytest, httpx
    ├── app/
    │   ├── main.py             # FastAPI app: health + ISO 20022 endpoints
    │   ├── config.py           # gateway config (engine gRPC addr, FX rate)
    │   ├── grpc_client.py      # thin client → Go engine (:50051)
    │   └── iso20022/
    │       ├── parser.py       # pacs.008 XML → internal JSON
    │       ├── serializer.py   # internal result → pacs.008 / pacs.002 XML
    │       └── validate.py     # structural validation of inbound messages
    └── tests/
        ├── test_parser.py      # XML → JSON unit tests
        ├── test_serializer.py  # JSON → XML round-trip tests
        └── test_gateway.py     # FastAPI httpx tests (+ live Go via gRPC)
```

## 4. gRPC Contract (`proto/vuka.proto`)

```proto
syntax = "proto3";
package vuka.v1;

service Bridge {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
  rpc GetBalance(GetBalanceRequest) returns (GetBalanceResponse);
  rpc FundAccount(FundAccountRequest) returns (FundResponse);
  rpc Transfer(TransferRequest) returns (TransferResponse);          // same-currency
  rpc CrossBorderTransfer(CrossBorderRequest) returns (TransferResponse); // RWF→KES
  rpc GetTransfer(GetTransferRequest) returns (TransferResponse);
  rpc ReverseTransfer(ReverseRequest) returns (TransferResponse);
}
```

Key message shapes (details in the proto file):

- `CrossBorderRequest { idempotency_key, source_account_id, destination_account_id,
  amount (RWF), currency_from="RWF", currency_to="KES", fx_rate (optional), invoice_number }`
- `TransferResponse { id, status, currency, amount, fx_rate, external_reference, entries[] }`
  — reused for both corridor legs' confirmation.
- Idempotency is always inside the DB transaction (Phase 1 rule) — replay returns the original.

## 5. Cross-Border Transfer Algorithm (inside ONE DB tx)

1. **Idempotency claim** — same `idempotency_keys` table; replay returns the original op.
2. **Lock source account** (`FOR UPDATE`) + validate: source BUSINESS, `currency_from` matches,
   amount > 0, destination exists with `currency_to`, source ≠ destination.
3. **Resolve FX rate**: request `fx_rate` else env default `VUKA_FX_RWF_KES`.
4. **Balance check** source RWF (derived `SUM(ledger_entries)`) ≥ amount.
5. **Dispatch Kenya rail** (simulated M-Pesa payout for the KES leg) inside the tx.
6. **On rail SUCCESS — write 2 legs (4 entries):**
   - Leg A (RWF): debit source business `-amount`, credit settlement `+amount`
   - Leg B (KES): debit settlement `-amount_kes`, credit destination business `+amount_kes`
   - `amount_kes = amount / fx_rate`
   - Each leg sums to zero → the whole transfer sums to zero.
7. **On rail failure** → rollback, no entries (same semantics as Phase 1).
8. Mark the single transaction header `SUCCESS` with `fx_rate` + `external_reference` stored.

> Note: this extends Phase 1's "exactly two entries" invariant *only* for the cross-currency case.
> Same-currency transfers keep exactly 2 entries. Both invariants are tested.

## 6. ISO 20022 Mapping (pacs.008 ↔ internal JSON)

Inbound `pacs.008.001.10` (Rwanda importer pays Kenyan supplier):

| XML (pacs.008) | Internal JSON field |
|---|---|
| `GrpHdr.MsgId` | `message_id` (→ idempotency_key) |
| `GrpHdr.CreDtTm` | `created_at` |
| `GrpHdr.TtlIntrBkSttlmAmt` / `Ccy` | `amount`, `currency` |
| `CdtTrfTxInf.Dbtr.Nm` + `DbtrAcct.Id.IBAN` | `source_name`, `source_account_ref` |
| `CdtTrfTxInf.Cdtr.Nm` + `CdtrAcct.Id.IBAN` | `destination_name`, `destination_account_ref` |
| `CdtTrfTxInf.InstrForCdtrAgt.InstrInf` | `invoice_number` / `reference` |
| `CdtTrfTxInf.IntrBkSttlmAmt` | `settlement_amount` |

Outbound response: `pacs.002.001.10` status report with `OrgnlMsgId`, `OrgnlEndToEndId`,
`TxSts = ACSC` (accepted) / `RJCT` (rejected), and a `StsRsnInf` reason on rejection.

Account resolution: the gateway maps `Dbtr/Cdtr` account references → VUKA account UUIDs via a
lookup table (env-configured or first-match on name). Full registry is out of scope.

## 7. Gateway API Surface (FastAPI)

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | liveness |
| POST | `/iso20022/pacs.008` | accept pacs.008 XML (text/xml), parse, validate, execute via gRPC, return pacs.002 XML |
| POST | `/iso20022/translate` | non-executing: XML → internal JSON (debug/verify) |
| POST | `/iso20022/from-json` | internal JSON → pacs.008 XML (non-executing) |
| GET | `/engine/balance/{account_id}` | proxy: gRPC → Go engine |

The gateway is **stateless** — every call is a pass-through to the Go engine over gRPC.

## 8. Test Plan

**Go (`go test -race ./...`):**
- `grpc/`: happy path, idempotent replay, insufficient funds, unknown adapter, reverse, invalid input → proper codes.
- `ledger/` (crossborder): RWF→KES legs balance to zero, fx math exact, insufficient funds, replay, business-source-only, reversal of cross-border, same-currency path unchanged (existing 11 tests stay green).
- `adapters/`: M-Pesa success/fail/timeout + external ref format (`MPESA-<ts>-<suffix>`).

**Python (`pytest`):**
- `parser`: valid pacs.008 → correct JSON; missing fields / wrong currency / bad XML → validation errors.
- `serializer`: JSON result → pacs.008 & pacs.002 round-trip (parse back, compare fields).
- `gateway`: httptest-style `httpx` against FastAPI with a **live Go engine** via gRPC — submit pacs.008, assert pacs.002 `ACSC`, assert ledger entries exist in PostgreSQL.

**Integration harness:** extend `Makefile` with `make gateway-test` (runs pytest with the Go
engine + PG up) and add a `gateway` service to `deploy/docker-compose.yml` (optional local run).

## 9. Verification Steps (before done)

1. `go vet ./...` + `go build ./...` — clean.
2. `go test -race ./...` — all pass (Phase 1 tests unchanged + new).
3. `pytest` in `python-iso-gateway/` — all pass.
4. **Live demo:** start PG + Go engine + FastAPI gateway →
   `curl -X POST /iso20022/pacs.008 --data @sample.xml` →
   observe pacs.002 `ACSC` response, verify RWF debit / KES credit rows in PG,
   replay the same pacs.008 → same response, no double charge.

## 10. Out of Scope (later phases)

- Live FX rate API (Phase 2 uses configured/env rate)
- camt.05x statements, pain.00x initiation messages
- Webhooks / SSE / WebSocket push to the trader dashboard
- Full XSD schema validation engine
- Uganda/Tanzania corridors, real rail integration
- React/TS trader dashboard (separate workstream)

---

## Approval

Review decisions §2 (especially D1 gRPC direction, D3 FX settlement legs, D5 pacs.008-only scope)
and the algorithm in §5. Reply **approved** (or with changes) and I execute task-by-task,
verifying `go vet` + `go build` + `go test -race` + `pytest` at every milestone.
