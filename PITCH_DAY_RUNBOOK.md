# VUKA — Pitch-Day Runbook & Cheat Sheet

**Pitch: 2026-08-11 (BNR Hackathon)**
**Stack:** Go ledger (Engine :8080 REST / :50051 gRPC) + React18/Vite dashboard (:5174) + PG18 (:5432)
**Repo root:** `/home/luqmaan/Documents/PROJECTS/FINANCE/VUKA`

---

## 1. Startup (60 seconds) — do in this exact order

```bash
cd /home/luqmaan/Documents/PROJECTS/FINANCE/VUKA/ENGINE/go-bridge-core

# 1) PostgreSQL (check first, usually already up)
pg_isready -h 127.0.0.1 -p 5432

# 2) Engine (mandatory env — FX + CORS + MTN simulator)
export VUKA_DATABASE_URL="postgres://vuka:changeme@localhost:5432/vuka?sslmode=disable"
export VUKA_FX_RWF_KES=9.5
export VUKA_CORS_ORIGINS="http://localhost:5173,http://localhost:5174"
export MTN_SIM_MODE=success
go run ./cmd/main.go          # wait for: "http server listening" addr=:8080

# 3) Frontend (only ONE dev server — the demo runs on :5174)
cd ../../FRONTEND/web-dashboard
npm run dev                    # must bind 5174 (vite.config.ts fixed port)
```

**Health check before walking on stage:**
```bash
curl -s localhost:8080/api/fx          # → {"rate":9.5}  (engine alive, FX correct)
curl -s -o /dev/null -w '%{http_code}\n' localhost:5174  # → 200
```

**Port map:** 5432 PG · 8080 REST · 50051 gRPC · 5174 frontend (5173 is dead — don't use it).

---

## 2. Demo storyboard (9 minutes)

### 0:00 — Landing page (`http://localhost:5174/#/`)
6 sections, enterprise navy/emerald, paper grid. Hero card shows the **real** transfer:
**101,500 RWF → 10,684 KES @ 9.50** (INV-2026-00891 — backed by a real ledger doc + Kethan's KES invoice).
☞ *"Cross-border trade payments, settled in minutes on a real double-entry ledger."*

### 0:30 — Press **Sign in** → profile select (`#/select`)
Three demo traders, phones byte-identical to seed:
- **Amina Uwera** — RWF importer (Kigali) — the operator you demo as
- **Jean-Paul Niyonzima** — RWF supplier, Musanze exporter
- **Kethan Gasana** — **KES** supplier, Nairobi Roasters Ltd (the corridor destination)

☞ Click Amina. *"Amina is a Kigali importer paying Rwandan and Kenyan suppliers."*

### 1:00 — Dashboard (`#/dashboard`)
Live KPIs (NOT hardcoded — computed from ledger):
- **Business Balance: RWF 2,917,250.00** (5M cash-in − 2,082,750 settled history)
- **Invoices: 9 paid / 0 outstanding** — corridor rate badge **RWF 9.50 = 1 KES**
- Transaction feed: clean `INV-2026-008xx` rows, Aug 1–9 history only
☞ *"Every number here is computed from the ledger — nothing is mocked."*

### 1:30 — Exchange Rates page — corridor status
Rwanda → Kenya `1 KES = 9.50 RWF`, live rail pricing from the settlement engine.
☞ *"Engine-priced corridor, sourced from the FX provider, not a hardcoded number."*

### 2:00 — **THE MONEY MOMENT** — Payments → **Cross-border (RWF → KES)** tab
Flow (walk slowly, narrate each step):
1. Source: **RWF · RWF 2,917,250.00** (business wallet)
2. Destination: **Kethan Gasana · Nairobi Roasters Ltd · KES** (pre-selected)
3. Amount: **250,000** RWF
4. Watch the **live FX preview** appear: *Visible FX rate 9.50 → Supplier receives **KES 26,315.79***
5. Invoice: `INV-2026-00901` (optional field)
6. Press **Send cross-border payment**

**Result — the pop:** register top row turns `SUCCESS · RWF 250,000.00 · MPESA-<id> · 10 Aug HH:MM`, and the **"Settled today" KPI flips RWF 0 → RWF 250K**, success rate 100%, 11 attempts.
☞ *"Idempotent, invoice-linked, double-entry — source leg debited, FX applied, M-Pesa credit leg written. Replay the same key and it never double-charges."*

### 2:30 — Click the new row → immutable double-entry audit view
Show the journal rows (debit/credit legs, FX leg). ☞ *"Every settlement leaves an immutable audit trail a regulator can verify."*

### 3:00 — Invoices page (if time)
9 invoice docs — KES invoices issued by **Kethan**, RWF by Jean-Paul, totals incl. VAT.
☞ *"Invoices are real documents with line items; payments reference them."*

### Wrap — one-line positioning
> *"VUKA is the settlement rail for cross-border SME trade in East Africa: real-time FX, double-entry audit, and one click from invoice to MPesa/MoMo payout."*

---

## 3. Reset for a pristine stage (before pitch)

Live test transfers dated **Aug 10** leave "Settled today" showing RWF 250K. For the KPI to pop live on stage, rebuild the register fresh (Aug 1–9 history, Aug 10 empty):

```bash
cd /home/luqmaan/Documents/PROJECTS/FINANCE/VUKA

# Engine must be RUNNING (script talks to :8080). Then:
bash scripts/build_demo_register.sh

# Regenerate invoice docs with corridor-correct sellers (KES → Kethan, RWF → Jean-Paul):
cd ENGINE/go-bridge-core
VUKA_DATABASE_URL="postgres://vuka:changeme@localhost:5432/vuka?sslmode=disable" go run ./cmd/seed

# Verify: 4 users / 8 accounts intact, 10 transfers (1 fund + 9 invoice-linked), FX 9.5 only on cross-border
```

**If anything looks off post-reset, the safety net is the pre-pitch dump:**
```bash
pg_restore -h 127.0.0.1 -U vuka -d vuka --clean --if-exists backups/vuka_pre_pitch_20260810.dump
```

---

## 4. Fast facts for Q&A

| Fact | Value |
|---|---|
| Engine | Go, pgx, double-entry ledger, REST :8080 + gRPC :50051 |
| Idempotency | Every POST carries `Idempotency-Key` (UUIDv4), replay-safe |
| Cross-border discriminator | `fx_rate IS NOT NULL` (transactions store source-leg currency RWF even for corridor rows) |
| Invoice docs | 9 seeded (4× KES by Kethan, 5× RWF by Jean-Paul); hero INV-2026-00891 = 10,684.21 KES after 18% VAT |
| Demo register | 10 transfers: 5M RWF cash-in + 5 domestic MTN refs + 4 cross-border MPESA refs |
| Ports | 5432 / 8080 / 50051 / 5174 |
| Tests | Go suite all packages green; vitest 30/30; tsc clean; prod build 282KB (81KB gzip) |

## 5. Pitfalls (learned the hard way)

- **The engine env is mandatory** — wrong FX (e.g. 101.5) or missing CORS → demo breaks silently. Always pin `VUKA_FX_RWF_KES=9.5` + `MTN_SIM_MODE=success`.
- **Only :5174** — a second dev server on 5173 confuses clients/CORS.
- **No GET /api/users** — look up by phone: `GET /api/lookup/user/<phone>`.
- **Demo phones must match the seed exactly** — byte-for-byte hex-verified; never retype from memory.
- **Cross-border rows display in RWF (source leg)**, not KES — expected behavior, mention if asked.
- **Transactions are immutable** — no PUT/PATCH; row click shows read-only double-entry.
- **Invoices page "paid" state** = `paid_at IS NOT NULL`; API field is `invoice_ref`, not `invoice_number`.
- **Engine runs via `go run`** — after engine-source edits, restart the process; the frontend picks up API changes without restart.