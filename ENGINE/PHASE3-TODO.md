# VUKA Phase 3 — Trader Dashboard (React/TypeScript) Implementation Plan

> **Status:** DRAFT — awaiting approval
> **Scope:** The trader-facing web dashboard — the 5th and final layer of the README/P-DESCRIPTION architecture.
> **Phase 3 MVP target:** A live, usable React dashboard: business/personal balances, invoice-linked transfers (same-currency + cross-border RWF→KES with visible FX), transaction history, and live status tracking — talking **directly to the Go engine over REST + SSE**.

---

## 1. Goal

Per P-DESCRIPTION §3.5 ("A Real Interface") and §4 layer 5, build the trader-facing application:

- **Live payment confirmation** — initiate a transfer, see it move PENDING → PROCESSING → SUCCESS in real time.
- **Visible exchange rate before committing** — the cross-border form shows the RWF→KES rate and the exact KES the supplier receives *before* the user hits send.
- **Reviewable transaction history** — full history with per-transfer double-entry detail.
- **Invoice-linked payments** — create an invoice, pay it, and trace the payment back to the invoice.
- **Business vs personal separation** — distinct balance cards (PERSONAL vs BUSINESS), per the core product promise.
- **Trust/payment history visible** — a simple payment-history view per trader (seed for the trust score story).

**Architecture decision (approved in conversation):** React + Vite SPA — NO Next.js, NO Node/NestJS service. The dashboard is a pure client; all logic stays in the Go engine. Node is used only as the frontend build toolchain.

## 2. Architecture Decisions (approve these)

| # | Decision | Choice | Why |
|---|---|---|---|
| D1 | Framework | **React 18 + TypeScript + Vite + Tailwind** | Client-only SPA; backend already exists in Go. Next.js would add an unneeded Node server layer (decided with user). |
| D2 | Backend contract | Dashboard calls **Go REST directly** (`:8080`), never Python, never a new BFF | Matches the 5-layer architecture; Go is the authority. |
| D3 | CORS | Add a **small CORS middleware to the Go engine** | Vite dev (`:5173`) → Go (`:8080`) is cross-origin; Phase 1 had no CORS (server-to-server only). |
| D4 | Live status | **SSE endpoint on the Go engine** (`GET /api/events`), consumed by the dashboard via `EventSource` | Transfer status is one-directional (PROCESSING → SUCCESS); SSE is simpler and correct. No WebSocket hub, no Node socket server. |
| D5 | Transaction history | Add **`GET /api/users/{id}/transfers`** to the Go engine | Not in Phase 1 API surface; the dashboard cannot show history without it. Also add optional `?account_id=` filter. |
| D6 | Auth for hackathon | **Demo-mode trader selection**: a trader picker on the landing screen; no password auth this phase | Engine has no auth; P-DESCRIPTION doesn't require it for the MVP. Keeps the demo fast. Full auth (KYC + sessions) is a listed out-of-scope item. |
| D7 | State management | Lightweight: React Context + `fetch` hooks; **no Redux** | Small SPA; Redux would be overhead. One ApiClient module + per-page hooks. |
| D8 | Routing | `react-router-dom` (HashRouter for static hosting friendliness) | Single static bundle; hash routing avoids server rewrite rules on deploy. |
| D9 | Design | **Enterprise navy `#0F2D5A` + emerald `#10B981`**, paper-grid background, clean B2B — no glassmorphism/gradients/purple-pink | Matches the user's established fintech design system (LinkRail) and the B2B audience. |
| D10 | Money display | All amounts rendered with `Intl.NumberFormat` + ISO currency code; 2 dp for RWF/KES display | Correct East-African number formatting, no floating-point strings in UI. |

**Deliberate non-decisions:**
- No auth/sessions/KYC UI (demo mode instead) — engine has no auth and Phase 3 is the UI layer.
- No trust-score algorithm — just the raw payment-history view; the score itself is a later phase.
- No mobile app — the dashboard is web; Flutter mobile remains a separate workstream.
- No dark mode, no marketing site.

## 3. File Layout

The dashboard is the **UI layer**, so it lives in the top-level `FRONTEND/` folder — separate from the backend `ENGINE/` folder (Go engine, Python gateway, deploy). This matches the repo's existing structure: `VUKA/FRONTEND/` already exists (empty, awaiting this build) and `VUKA/ENGINE/` holds the backend tiers.

```
VUKA/
├── ENGINE/                    # backend (existing)
│   ├── PHASE1-TODO.md / PHASE2-TODO.md / PHASE3-TODO.md
│   ├── deploy/                # docker-compose, init.sql
│   ├── go-bridge-core/        # Go ledger engine (REST :8080, gRPC :50051)
│   ├── python-iso-gateway/    # FastAPI ISO 20022 gateway
│   └── proto/vuka.proto
└── FRONTEND/                  # UI layer (NEW — this build)
    └── web-dashboard/
        ├── package.json            # react, react-dom, react-router-dom, vite, tailwind
        ├── vite.config.ts          # dev server proxy /api -> localhost:8080
        ├── tailwind.config.js      # navy/emerald palette + paper grid
        ├── index.html
        ├── src/
        │   ├── main.tsx            # entry: Router + ApiProvider
        │   ├── App.tsx             # layout shell (sidebar, topbar)
        │   ├── api/
        │   │   ├── client.ts       # typed fetch wrapper (base URL, JSON, errors)
        │   │   └── types.ts        # User, Account, Balance, Transfer, LedgerEntry
        │   ├── hooks/
        │   │   ├── useBalances.ts  # per-account derived balance
        │   │   ├── useTransfers.ts # history + create + reverse
        │   │   └── useLiveStatus.ts# SSE EventSource -> transfer status updates
        │   ├── components/
        │   │   ├── BalanceCard.tsx # PERSONAL/BUSINESS wallet card
        │   │   ├── TransferForm.tsx# same-currency + cross-border w/ live FX preview
        │   │   ├── TransferTable.tsx
        │   │   ├── StatusBadge.tsx # PENDING/PROCESSING/SUCCESS/REVERSED
        │   │   └── InvoiceForm.tsx
        │   └── pages/
        │       ├── TraderSelect.tsx # demo-mode trader picker (D6)
        │       ├── Dashboard.tsx    # balances + recent activity + live updates
        │       ├── Transfers.tsx    # new transfer form + history
        │       ├── Invoices.tsx     # create/list invoices, link payments
        │       └── History.tsx      # full audit view incl. double-entry rows
        └── tests/
            ├── format.test.ts       # money formatting
            └── api.test.ts          # client error mapping (vitest)
```

## 4. Go Engine Additions (minimal, tested)

Three additions to the Go engine — all covered by tests, all keeping the existing surface intact:

1. **CORS middleware** — allow `Origin: http://localhost:5173` (configurable via `VUKA_CORS_ORIGINS`), handle `OPTIONS` preflight.
2. **`GET /api/users/{id}/transfers`** — list a user's transfers across their accounts (joins `transactions` → `accounts` → `users`), newest first, with optional `?account_id=&status=` filters. Returns `{ transfers: [...] }`.
3. **`GET /api/events` (SSE)** — an in-memory hub; the transfer/fund/reverse handlers publish status changes; clients receive `event: transfer` with `{id, status, updated_at}`. Per-process (fine for the single-instance hackathon deployment; multi-instance would use PG LISTEN/NOTIFY later).

## 5. Dashboard Pages & Flows

### TraderSelect (`/`)
Demo-mode picker: lists seeded traders from the engine, click one to "log in" (stores trader id in memory/URL). Mirrors D6.

### Dashboard (`/dashboard`)
- Two `BalanceCard`s: PERSONAL and BUSINESS (per wallet separation promise).
- "New transfer" quick action.
- Recent transfers table with live `StatusBadge` (SSE updates in place).
- Cross-border summary: current RWF→KES rate + KES balance card if the trader has one.

### Transfers (`/transfers`)
- **Same-currency form:** source account (BUSINESS), destination account, amount, currency, invoice number, Idempotency-Key (auto-generated UUID).
- **Cross-border form:** amount in RWF → **shows the KES the supplier receives at the current rate before send** (P-DESCRIPTION §3.5 "visible exchange rate before committing"); sends `fx_rate` explicitly so the engine's recorded rate matches what the user saw.
- History table with status badges + expandable double-entry detail (fetch `/api/transfers/{id}/entries`).

### Invoices (`/invoices`)
- Create invoice (number, amount, currency, counterparty name) — stored in-memory/localStorage for the demo (engine has no invoice table; the `invoice_number` is carried on transfers).
- List invoices, mark "paid" when a matching transfer exists (matched by invoice_number).

### History (`/history`)
- Full paginated transfer list for the trader with status filters and double-entry audit rows.

## 6. Frontend Design System

- **Palette:** navy `#0F2D5A` (primary), emerald `#10B981` (success/positive), paper grid background at full opacity; neutral slate for text; red for errors/reversals.
- **No** glassmorphism, gradients, or purple/pink anywhere.
- Status colors: PENDING amber, PROCESSING blue, SUCCESS emerald, REVERSED slate, FAILED red.
- Amounts: emerald for credits, navy/red for debits, always currency-suffixed (RWF/KES).

## 7. Test Plan

**Go (existing suite stays green + new):**
- `api`: CORS preflight returns allowed headers; `/api/users/{id}/transfers` returns transfers newest-first and filters; SSE hub delivers a status event after a transfer (using an in-process `httptest` + goroutine reader).

**Frontend (vitest + React Testing Library):**
- `format.test.ts`: number/currency formatting edge cases.
- `api.test.ts`: client maps HTTP errors to typed exceptions; builds correct URLs.
- Component tests: BalanceCard renders amounts, TransferForm FX preview computes KES = RWF / rate, StatusBadge maps status→color.
- Manual E2E (documented, not automated): trader select → fund → transfer → live SUCCESS badge → history row present.

**Integration:** `make dashboard` (install deps), `make dashboard-test` (vitest run), `make demo` (engine + dashboard dev server up together) — extended in the root Makefile.

## 8. Verification Steps (before done)

1. `go vet ./...` + `go build ./...` + `go test -race ./...` — clean (all Phase 1/2 tests + new SSE/list tests).
2. `vitest run` in `web-dashboard/` — all pass.
3. `npm run build` — production build succeeds with `tsc --noEmit` clean.
4. **Live demo:** start PG + engine + dashboard → select trader → fund business wallet → send cross-border transfer (see KES preview) → watch PENDING→PROCESSING→SUCCESS live via SSE → verify history row + double-entry detail → replay same Idempotency-Key → no duplicate.

## 9. Out of Scope (later phases)

- Auth/sessions/KYC onboarding UI (demo-mode trader selection instead)
- Trust-score algorithm (raw payment history only)
- Webhook delivery to third parties
- Flutter mobile app
- Multi-instance SSE (PG LISTEN/NOTIFY) — single-instance in-memory hub for the hackathon
- Dark mode / marketing site

---

## Approval

Review decisions §2 (especially D1 no-Next/no-Nest, D4 SSE, D5 the new list endpoint, D6 demo-mode auth) and §3 layout. Reply **approved** (or with changes) and I execute task-by-task, verifying `go vet` + `go build` + `go test -race` + `vitest` at every milestone.
