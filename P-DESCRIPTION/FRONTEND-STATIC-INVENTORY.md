# VUKA Frontend — Static / Unreachable / Placeholder Inventory

**Date:** 2026-08-06 · **Scope:** `FRONTEND/web-dashboard/src` (all 17 files read)
**Purpose:** Every hardcoded value, dead button, unreachable route, fake data source, and placeholder in the dashboard UI — so nothing "demo" can be mistaken for a working feature.

---

## 1. Routes that are real pages backed by API calls (the only live ones)

| Route | File | Data source |
|---|---|---|
| `/dashboard` | `pages/Dashboard.tsx` | Partial — balances/transfers live via hooks, but most panels are static (see §3) |
| `/transfers` | `pages/Transfers.tsx` | Live — `useBalances` + `useTransfers`, real create via `POST /api/transfers` |
| `/invoices` | `pages/Invoices.tsx` | Live — `useInvoices`, real create/pay via API (pay = plain `createTransfer`) |
| `/history` | `pages/History.tsx` | Live — `useTransfers`, double-entry rows via `getTransferEntries` |

## 2. Routes that are 100% static (fake pages, zero API calls)

All of these mount `OperationsModule` (`pages/OperationsModule.tsx`), which renders a hardcoded `moduleData` constant. **No hooks, no fetch, no SSE — every number is invented.**

| Route | Module key | Invented content |
|---|---|---|
| `/wallet` | `wallet` | "Available RWF 166.5M", "Locked in escrow RWF 18.4M", escrow/settlement-reserve accounts that don't exist in the ledger |
| `/partners` | `partners` | "248 verified partners", rows for Nairobi Roasters Ltd, **Kampala Fabrics (Uganda)**, **Dar Logistics Co (Tanzania)** — markets the backend doesn't support |
| `/trust-score` | `trust` | "87/100", "98.2% reliability", "Jinja Millers" anomaly — Jinja Millers is not in the DB |
| `/exchange-rates` | `rates` | "RWF/KES 0.1072", "KES/UGX 28.41", "USD/RWF 1,342.20" — **no rate endpoint exists**; backend FX is one env var (`VUKA_FX_RWF_KES`) |
| `/analytics` | `analytics` | "RWF 1.24B monthly revenue", "42s median finality" |
| `/reports` | `reports` | "Monthly statement July 2026 PDF" etc. — **no report generation exists at all** |
| `/notifications` | `notifications` | "7 unread", rows referencing INV-2048 / INV-2045, "USD/RWF above 1,340" — no notifications subsystem |

Also inside `OperationsModule`: the "Control panel" (`92 - index * 7`% for Policy/Counterparty/Ledger/Route) is a formula, not data; the sparkline polyline is a hardcoded SVG; **Export / Create workflow buttons have no `onClick`** (`OperationsModule.tsx:58-59`).

## 3. Dashboard.tsx — hybrid: live skeleton, static organs

### 3a. Hardcoded data constants (top of file)
- `transactions` — 7 fabricated cross-border rows ("Kigali Coffee Coop → Nairobi Roasters", "Mombasa Textiles", "Kampala Fabrics", "Arusha Agro", "Jinja Millers", "Eldoret Grain", "Mwanza Fisheries") — **none of these counterparties exist in the seed/DB**
- `invoices` — 6 fabricated invoice rows (INV-2048…INV-2043)
- `rates` — fabricated FX table
- `reportCards`, `kpiSparks` — static arrays

### 3b. Fabricated fallback numbers (the dangerous part)
`dashboard` useMemo supplies **fake values when the backend returns nothing**:
- `business?.balance ?? 184_920_400`, `personalBalance ?? 6_140_900`
- `volume || 42_318_000`, `paid || 1284`, `pending || 163`, `processing || 96`, `failed || 21`

→ With an empty DB you see "RWF 184.9M available", "1,284 paid" etc. **The UI cannot show "0" — it invents money.**

### 3c. `mergeTransfers` (line 273) — real data wearing fake clothes
When real transfers exist, each is **mapped onto a static template row**: counterparty names, corridor, and rail come from the fabricated `transactions` array (`base[0]`, `base[1]`, `base[3]`, `base[4]`). Only amount/status/invoice-number are real. So the "live transaction feed" shows **fabricated counterparties** even for genuine transfers.

### 3d. Fully static panels (no props, no data)
- KPI cards: "Monthly Revenue RWF 1.24B", "Pending Settlements RWF 18,405,220", "Trust Score 87/100 Tier A", "Avg. Settlement Time 42s" (lines ~54-59, hardcoded values)
- `TrustGauge` — 87/Tier A, 98.2%, 0.4%, 4,182 txns, ratings — all hardcoded
- `CorridorMap` — static SVG with **4 countries** (Rwanda/Uganda/Kenya/Tanzania); backend supports only RWF↔KES
- `Notifications()` — "Settlement complete · INV-2048", "Nakuru Traders joined" — invented
- `Compliance()` — 92%, KYB/AML/director/tax hardcoded; "Unusual volume · Jinja Millers" — Jinja Millers doesn't exist
- `Donut` — success rate 98.4%, avg invoice RWF 3.1M hardcoded (slice widths are fixed percentages, not computed)
- `AreaChart`/`BarChart`/`CashFlow` — hardcoded month arrays
- `InvoiceActions` — search input + Filters + Export buttons **with no `onClick`** (line 405)
- Footer "Ledger synced 14:02:44 CAT" — hardcoded timestamp
- Subtitle "Live across 4 corridors · MTN MoMo, M-Pesa, Airtel Money" — Airtel Money has no adapter
- "1,447 invoices · 22 require attention", "Showing 1-6 of 1,447" — invented

## 4. Transfers.tsx
- `FX_RATE = 9.5` hardcoded (line 13) — duplicates backend env, no live fetch
- Metric cards "Today settled RWF 42.3M", "Processing 96 median 42s", "Pending queue 163", "Rail uptime 99.98%" — hardcoded (lines 54-57)
- "Settlement route preview" — **Kenya→Uganda M-Pesa, Uganda→Tanzania Airtel Money** rows (lines 74-78): corridors that don't exist
- "Schedule batch" and "New payment" buttons — **no `onClick`** (lines 48-49)
- Header copy advertises "cross-border settlements" — not reachable from UI (§6)

## 5. History.tsx
- AuditMetrics: "Success rate 98.4%", "Avg finality 42s" — hardcoded (lines 103-105). ("Ledger exceptions 0 / Balanced" is hardcoded too.)
- `invoiceFromTransfer` — builds a **fake invoice document** from a transfer ("VUKA payment / Cross-border trade") — presentation only, not a real invoice record

## 6. The cross-border dead end (client.ts + TransferForm)
- `client.ts:137-143` — `crossBorderTransfer` is a **stub that throws `ApiError(0, 'NOT_SUPPORTED', …)`**; comment admits REST has no cross-border route
- `TransferForm.tsx` — has a **"Cross-border (RWF → KES)" toggle** (with an FX preview) but `Transfers.tsx:67` passes `onSubmit={createTransfer}` regardless of mode → **selecting cross-border silently executes a same-currency RWF transfer via `POST /api/transfers`**. No error, no KES, no FX. The toggle is a lie; the stub is dead code from the UI.
- `api/types.ts` `CrossBorderRequest` exists but is only consumed by the throwing stub.

## 7. Invoices.tsx
- `InvoiceMetric` "Total invoices" falls back to **'1,447' when the list is empty** (line 152) — invents data on empty state
- "Require attention 22", "Paid this quarter 1,284", "Avg invoice value RWF 3.1M" — hardcoded (lines 153-155)
- **Filters / Export buttons — no `onClick`** (lines 146-147)
- Pay flow is real (calls `createTransfer` with `invoice_number`), but there is **no dedicated invoice-settle endpoint** — payment is just a transfer with a reference

## 8. TraderSelect.tsx
- `DEMO_TRADERS` hardcoded (3 traders — matches seed, this one is intentional demo auth)
- `proof` const — "RWF 1.24B volume / 42s / 98.4% / 87/100" — hardcoded marketing numbers
- `CorridorScene` — static 4-country SVG; corridor-flow stats include **TZS 1.9B / UGX 3.4B** corridors that don't exist
- Offline fallback: if the backend is down, silently creates `offlineUser` with **"simulated live data"** — the whole dashboard then runs on fabricated numbers without any obvious banner

## 9. App.tsx (Shell chrome)
- Sidebar trader block: **"KC" avatar, "Kigali Coffee Coop", "Operations Admin" hardcoded** (lines 95-98) — ignores the actually-selected trader (e.g. Kethan Gasana would still show "Kigali Coffee Coop")
- Header trader block identical hardcode (lines 128-134)
- Sidebar **"Settings / Support / Logout" buttons — no `onClick`** (lines 76-81); Logout does nothing, and the `clear` prop wired to the switch-trader button only
- "Compliance verified / KYB & AML current. Next review 30 Sep 2026." — hardcoded card (lines 84-90)
- Notifications badge "7" hardcoded (line 72)
- Header: search input **decorative** (no state, no ⌘K handler), "🇷🇼 Rwanda" / "RWF" buttons **no `onClick`** (lines 121-122), "Create Invoice / Add Supplier / Convert Currency / Export Report" — **no `onClick`** (lines 124-126)
- "+ New Payment" is the only functional header button (NavLink to /transfers)

## 10. InvoiceForm.tsx
- `DEMO_PHONES` hardcoded lookup list (line 71) — matches seed, acceptable demo shortcut, but any real counterparty not in the list is unreachable from the form (no free-text phone entry)

## 11. Non-issues (verified live)
- `StatusBadge`, `TransferTable`, `BalanceCard`, `format.ts` (`Intl.NumberFormat`), `useLiveStatus` (SSE), `useTransfers`, `useBalances`, `useInvoices`, `TraderContext` (auth switch) — all real, prop/data-driven.

---

## Summary counts
- **Fake pages (100% static):** 7 routes → 1 component (`OperationsModule`)
- **Dead buttons (no `onClick`):** ~14 across Shell header/sidebar, Transfers, Invoices, Dashboard, OperationsModule
- **Fabricated fallback data when backend is empty:** Dashboard balances/volume/paid/pending/processing/failed + Invoices count '1,447'
- **Counterfeit data masquerading as live:** mergeTransfers counterparties, TrustGauge, compliance, notifications, corridors, charts, KPIs
- **Dead API stub:** `crossBorderTransfer` (throws NOT_SUPPORTED) — unreachable from UI; the UI toggle instead silently runs a same-currency transfer
- **Hardcoded constants:** `FX_RATE = 9.5` (frontend copy of backend env)
