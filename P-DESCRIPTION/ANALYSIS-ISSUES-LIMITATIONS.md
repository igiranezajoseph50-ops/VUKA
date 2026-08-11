# VUKA — Progress Analysis: Issues, Limitations, Contrasts, Missing

Created: 2026-08-06 · BNR Hackathon 2026 · Verified against live source (ENGINE Go + Python gateway + FRONTEND dashboard)

## 0. Delivery Summary

| Area | Status | Verified by |
|---|---|---|
| Phase 1 ledger (double-entry, idempotency, REST+SSE+gRPC, MTN/M-Pesa adapters) | ✅ done | `go build`/`vet` exit 0; `go test ./...` all packages `ok` |
| Phase 2 FX settlement + ISO 20022 (pacs.008↔pacs.002) | 🟡 code present | Go gRPC + Python gateway real; **NOT wired into REST/UI**, FX rate is a static env value |
| Phase 3 dashboard (React 18 + Vite + Tailwind SPA) | 🟡 in progress | `tsc --noEmit` exit 0; **30/30 vitest pass** (with `NODE_ENV=test`) |
| Deployment (PG 16 + Adminer compose) | ✅ | `deploy/docker-compose.yml` |
| **No git repo / no CI** | ⚠️ | `git log` confirms none |

Everything I claim below was read from source and/or verified by running the test suites — not assumed.

---

## 1. Issues / Limitations (highest-impact first)

### 1.1 Cross-border is NOT reachable from the dashboard (functional gap)
- The Go REST API (`internal/api/server.go`) exposes **no cross-border route**. Endpoints are: users, accounts, balance, fund, `POST /api/transfers`, invoices, entries, reverse, SSE events, healthz.
- Cross-border execution exists **only** in the ledger service (`CrossBorderTransfer`) and over **gRPC** (`internal/grpc`).
- The frontend `api/client.ts` has a stub:
  ```ts
  crossBorderTransfer: (_req) => { throw new ApiError(0,'NOT_SUPPORTED','Cross-border is gRPC-only; use the ISO gateway.') }
  ```
- Yet the `TransferForm` UI **offers a "Cross-border (RWF → KES)" mode** — but `Transfers.tsx` passes `createTransfer` (the same-currency REST path, `POST /api/transfers`) as `onSubmit` regardless of range flag. `TransferForm` builds the request as `currency: mode === 'cross' ? 'RWF' : …`. So a user clicking "Send cross-border payment" **silently performs a same-currency RWF transfer via REST** — the gRPC `CrossBorderTransfer` and its 4-entry FX settlement never run.
- **Consequence:** the headline demo feature (RWF→KES settlement) cannot be triggered from the product UI, and the UI gives no error — it quietly moves RWF as if it were cross-border. Only a direct gRPC client can reach the real settled FX path.

### 1.2 Money is handled as Go `float64` (correctness risk)
- `ledger_entries.amount`, `transactions.amount`, `fx_rate`, invoice `quantity`/`unit_price`/`vat_rate` are all `float64` in Go, scanned back from Postgres `NUMERIC`.
- The DB schema (`init.sql`) is `NUMERIC(18,4)` (correct), and Go rounds FX to 4dp (`round4`), so small cases are fine — but **the application layer never uses a decimal/integer-cents type**. Balance is summed in float64. For an MVP demo this is tolerable; for a real payments ledger it must be integer minor units or a proper decimal type. This is the biggest correctness smell in the codebase.

### 1.3 FX rate is static, not live
- `VUKA_FX_RWF_KES` env default (9.5 in demo) is the only rate source unless a caller passes `fx_rate` explicitly. `resolveFxRate` just picks request-rate-or-env-default.
- **No FX provider, no spread/margin model, no fee computation, single rate for the whole corridor.** A hackathon demo is fine; the "market intelligence" claim in the UI outruns the backend's capability.

### 1.4 ISO-20022 gateway is downstream/demonstrative, not in the execution path
- The Python FastAPI gateway (`python-iso-gateway`) generates `pacs.008`, parses `pacs.002` responses, and acts as a **gRPC client** to the Go engine. It is a real, coherent ISO 20022 pipeline — good for a hackathon story.
- But the runtime **transfer never calls it**. The `Transfer` and `CrossBorderTransfer` paths dispatch directly to simulated rails (`mtn-rw`, `mpesa-ke`) inside the DB tx. ISO-20022 output is generated separately / on demand, not emitted for every settlement. So "ISO 20022 ready" is an architecture demo, not an integrated message flow.

### 1.4 SSE is single-instance only
- `internal/api/sse.go` is an **in-memory hub**. The UI's `useLiveStatus` opens one shared `EventSource('/api/events')`. Any second engine instance doesn't publish to the first's listeners — no Postgres `LISTEN/NOTIFY`. Fine for a single dev node, breaks under horizontal scaling.

### 1.5 No auth / no KYC enforcement in runtime
- Trader selection is a click-through picker (`TraderSelect.tsx`), no password (by design for the MVP). `kyc_status` is a field but is never enforced before a transfer. Fine for demo; must be flagged as intentionally absent, not present-but-hidden.

### 1.6 Borderless "4-corridor" maps/claims are surface-level
- The landing `TraderSelect` shows a live "Rwanda→Kenya / Kenya→Uganda / Tanzania→Rwanda" corridor scene and headline "RWF 402M / KES 88M / TZS 1.9B" figures — **these are static/hardcoded** mock values on a dashboard; only the RWF↔KES corridor has a functional route. The other corridors (TZS, UGX) don't exist in the ledger/adapters.

---

## 2. Architectural Contrasts (deliberate choices worth knowing)

| Choice | VUKA | Classic alternative | Note |
|---|---|---|---|
| Ledger | **Double-entry, balance = `SUM(ledger_entries.amount)`**, no balance column | Materialized balance column | ✅ safer, auditable; slower reads, good for MVP |
| Idempotency | DB-backed `idempotency_keys` UNIQUE, claimed in the same tx | Redis TTL / in-memory | ✅ correct & durable; rejected replay cost is a DB round-trip |
| Settlement | **Single SETTLEMENT account per currency**, legs lock both + both settlement rows, 4 entries/2 legs zero-sum | Nostro/vostro pairs per bank | Correct zero-sum math, but one settlement account per currency can't represent multiple real bank accounts |
| Concurrency | `SELECT … FOR UPDATE` lock ordering (source→dest→settlement) | MVCC optimistic | ✅ deterministic, locks both legs together |
| Rail integration | Simulated adapters inside tx; **entry written only on rail SUCCESS**; crash rolls back | Webhooks/async reconciliation | ✅ atomic & demo-friendly; does NOT model the real world (a real rail returns async / can fail after local commit) |
| Dashboard data plane | React → REST :8080 + SSE only; **never touches Python** | BFF/gateway in middle | ✅ clean; but that's exactly why cross-border isn't wired |
| Build/runtime mgmt | Go stdlib `net/http` (Go 1.22 ServeMux), `pgx/v5` pool, no ORM, no framework | — | ✅ lean |
| No git repo / no CI | file state is the source of truth | standard tooling | ⚠️ no rollback, no reviewer merging, no pipeline |

---

## 3. Missing (gaps a hackathon judge / production reviewer would catch)

1. **Cross-border entry point in the UI + REST route** — the flagship feature is non-demoable from the product.
2. **Decimal money type** (integer minor units) in Go.
3. **Auth + KYC enforcement** gate on transfers/invoices.
4. **Live FX source** (or a fixed clearly-labeled demo rate) and a **spread/fee model**.
5. **ISO-20022 wired into the settlement path** (or rename the claim to "ISO 20022 converter library").
6. **Multi-instance SSE** via Postgres `LISTEN/NOTIFY`.
7. **Real corridor coverage** for UGX/TZS or explicit reduce scope.
8. **End-to-end integration test** that drives the dashboard → REST → cross-border → ISO gateway in one flow (the gRPC cross-border path is only tested in isolation).
9. **Reverse / dispute flow UX** — the `reverse` REST endpoint exists but there's no UI and no reversal → ISO `camt`/pacs copy message.
10. **git init + CI** (Go test, vitest, `make provision` on PR) — the project is demo-ready but has no delivery rails.

---

## 4. What I verified as solid (give yourself credit)

- Go build, vet, and **full test suite all pass** (`ledger`, `api`, `grpc`, `idempotency`, `adapters`, invoice + phase3 cross-border tests).
- Frontend `tsc --noEmit` clean; **vitest 30/30 pass once `NODE_ENV=test` is set** (the shell exported `production`, which breaks React `act()` in jsdom — a harness gotcha, not a code bug).
- Double-entry correctness is enforced at the query level (balance derivation, SUM, FOR UPDATE, idempotency last-write-wins).
- Cross-border FX writes **exactly 4 ledger entries / 2 zero-sum legs** with rate recorded on the transaction for audit/reversal.
- Clean layering: `ledger` (pure domain) vs `api` vs `adapters` (swappable), model.Does not claim.

---

### Suggested next moves (pick what fits the demo deadline)
- **Highest ROI:** surface an actual cross-border REST route + wire the UI toggle → makes the trademark demo work from the app.
- Then: add a clear "MTN/M-Pesa simulation" and live-FX note in the UI so nobody reads the static corridor claims as real.
- Write one `e2e` script that: seed → fund → same-ccy transfer → cross-border gRPC → assert 4 balanced entries → get ISO pacs.008 feed → mark SUCCESS.