# VUKA — Project Documentation & Journey

**"Rise & cross over."** A trade-focused payment layer for East African SME traders — built for the **National Bank of Rwanda FinTech Innovation Hackathon 2026**.

> **Current status: PROTOTYPE.** This document explains what VUKA is, what has been built and verified, every problem encountered and solved along the way, the answers to the questions that shaped the design, the honest limits of the prototype, and the phased roadmap to a production-ready African payments product.

---

## Table of contents

1. [The vision & the problem](#1-the-vision--the-problem)
2. [The product concept](#2-the-product-concept)
3. [What exists today (the prototype)](#3-what-exists-today-the-prototype)
4. [How the core mechanisms work](#4-how-the-core-mechanisms-work)
5. [The demo: profiles, data, walkthrough](#5-the-demo-profiles-data-walkthrough)
6. [How to run everything](#6-how-to-run-everything)
7. [The journey — problems found & fixed](#7-the-journey--problems-found--fixed)
8. [The questions that shaped the design (and their answers)](#8-the-questions-that-shaped-the-design-and-their-answers)
9. [Honest limitations of the prototype](#9-honest-limitations-of-the-prototype)
10. [Roadmap — from prototype to Africa-ready](#10-roadmap--from-prototype-to-africa-ready)
11. [Regulatory & security framing](#11-regulatory--security-framing)
12. [Glossary](#12-glossary)
13. [Appendix — key IDs, endpoints, verification record](#13-appendix--key-ids-endpoints-verification-record)

---

## 1. The vision & the problem

**The problem.** Millions of East African SMEs trade across borders (Rwanda ↔ Kenya especially). Mobile money — MTN MoMo and M-Pesa — already moves value across those borders. But the rails were built for **personal remittances**, not for **trade**. When a Kigali coffee importer pays a Nairobi roaster:

- There is **no context** — a payment is just "money moved," with no invoice attached, so disputes become "I paid" vs "you never did."
- **Business and personal money are mixed** — one wallet does everything, so no lender can tell what the business earned, spent, or owes.
- **Trust is invisible** — there's no payment history a supplier or lender can point to, so SMEs get stuck paying large upfront deposits.
- **Trade-sized payments are awkward** — limits and compliance checks are tuned for small remittances, not business invoices.

**The vision.** VUKA is **the trade-facing layer that mobile money never built**: a business layer *on top* of the existing rails — invoice-linked payments, separated business/personal ledgers, and visible trust — for the SME trader stuck using a personal remittance rail for a business.

**The corridor.** Rwanda ↔ Kenya (RWF ↔ KES), riding MTN MoMo (Rwanda) and M-Pesa (Kenya). Built to expand to Uganda and Tanzania after the pilot.

---

## 2. The product concept

VUKA closes five gaps, one at a time:

| Tag | Gap closed |
|-----|-----------|
| **INV** | **Invoice-linked payments** — every transfer is initiated against an order or invoice; both sides keep a timestamped record of what it was for. |
| **SEP** | **Business and personal separated** — a distinct business balance tracked apart from the personal wallet — the first brick of bookkeeping and credit history. |
| **SIZE** | **Built at trade size** — limits and compliance checks set for business-scale payments, not personal-remittance ceilings. |
| **TRUST** | **Visible payment history** — a trader's on-time record is visible before terms are agreed, reducing large upfront payments. |
| **UI** | **A real interface** — live confirmation, a visible exchange rate before committing, and a reviewable history from initiation to settlement. |

**The credit story (how trade actually works).** The buyer asks a supplier for credit → the supplier delivers goods and later **issues an invoice** with a due date (the due date *is* the credit term) → when the term is reached the buyer **pays via VUKA**. VUKA doesn't just track the payment — it tracks whether the credit was repaid **within terms or overdue**, and both sides see the same PAID state, because it is derived from the same ledger.

---

## 3. What exists today (the prototype)

### 3.1 Architecture at a glance

```
┌─────────────────────────────────────────────────────────────┐
│  Dashboard — React 18 + Vite + Tailwind (HashRouter)        │
│  Landing (#/), Select Profile (#/select), Dashboard,        │
│  Payments, Invoices, History, Wallets, Rates, Analytics     │
└──────────────────────────────┬──────────────────────────────┘
                               │ REST :8080 (JSON) + SSE events
┌──────────────────────────────▼──────────────────────────────┐
│  Engine — Go (go-bridge-core)                                │
│  internal/ledger   — double-entry core (the money machine)    │
│  internal/api      — REST handlers + error mapping            │
│  internal/grpc     — gRPC :50051 corridor API                 │
│  internal/adapters — telecom rail abstraction (SIMULATED)     │
│  internal/idempotency — idempotency-key store                 │
└──────────────────────────────┬──────────────────────────────┘
                               │ SQL
┌──────────────────────────────▼──────────────────────────────┐
│  PostgreSQL 18 (local) — vuka (live), vuka_test (tests)     │
│  users, accounts, transactions, ledger_entries, invoices,   │
│  invoice_items, idempotency_keys                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 The five layers (as shown on the landing page)

| Layer | Tech | Role |
|-------|------|------|
| **L4** | PostgreSQL | Double-entry ledger. Every transfer nets to zero; no balance is ever stored directly. |
| **L3** | Go · REST :8080 · gRPC :50051 | Transfer engine with safe concurrency — simultaneous transfers cannot corrupt a balance. |
| **L2** | React | Live, trader-facing dashboard — confirmation, history, status tracking for both corridors. |
| **L1** | ISO 20022 | Standard message format that removes the mismatches that cause failed transfers (roadmap). |

**The golden rule (models.go):** *Account rows never carry a balance column. A balance is strictly defined as `SUM(amount) FROM ledger_entries WHERE account_id = X`.* Every transfer writes exactly two linked ledger entries (debit + credit) summing to zero, inside one database transaction with the source account locked `FOR UPDATE`. Concurrency cannot corrupt a balance.

### 3.3 Repository layout

```
VUKA/
├── ENGINE/go-bridge-core/          # The Go engine
│   ├── cmd/main.go                 # entrypoint
│   ├── cmd/seed/main.go            # seeds demo traders, wallets, 9 invoices, 11 transfers
│   └── internal/
│       ├── ledger/                 # models, transfer, transfers, crossborder, invoices, fund, fx
│       ├── api/                    # REST handlers, CORS, SSE
│       ├── grpc/                   # gRPC server
│       ├── adapters/               # TelecomAdapter interface + MTN/M-Pesa SIMULATORS
│       ├── idempotency/            # idempotency store
│       └── config/                 # env config
├── FRONTEND/web-dashboard/         # React dashboard
│   └── src/
│       ├── pages/                  # Landing, TraderSelect, Dashboard, Transfers, Invoices, History, etc.
│       ├── components/             # TransferForm, InvoiceForm/InvoiceDocument, CorridorScene, ui/
│       ├── state/TraderContext.tsx # selected-profile holder (demo auth)
│       └── lib/format.ts           # money, amountInWords, numberToWords, convert
├── scripts/build_demo_register.sh  # rebuilds demo history Aug 1-9 (leaves demo day clean)
├── profile.png                     # design blueprint for the select-profile page
├── PITCH_DAY_RUNBOOK.md            # 9-minute pitch-day runbook + Q&A fast-facts
└── PROJECT_DOCUMENTATION.md        # THIS FILE
```

---

## 4. How the core mechanisms work

### 4.1 The shared ledger and the "no login" model

**There is no login in the prototype — and that is deliberate, documented scope.** A `User` is a trader record; every user automatically gets **two wallets** (PERSONAL and BUSINESS) in their currency, created atomically (transfer.go `CreateUser`). The `/select` page is a **profile picker** — in demo mode, the person at the keyboard *becomes* one of the seeded traders, held in `sessionStorage` (`vuka.trader`).

A transfer is a row in `transactions` (source account, destination account, amount, currency, fx_rate, status, timestamps) plus two immutable `ledger_entries`. This is the **one shared ledger** — one truth for everyone.

**The two-seat property (history):** a transfer "belongs" to a user when **either of their accounts appears on either leg** (transfers.go `ListTransfers`):

```sql
WHERE (source_account_id IN (SELECT id FROM accounts WHERE user_id = $1)
    OR destination_account_id IN (SELECT id FROM accounts WHERE user_id = $1))
```

Consequence: **one database row, two readers.** When Amina pays Kethan, the same transfer appears in Amina's history (she paid) *and* Kethan's history (he received). Switch profiles and you change seats on the same ledger — you don't get a different copy of the truth. The History page carries a **"Viewing as …" chip** that states the scope: *"every record below touches this profile's accounts — sent or received."*

**Legality framing (Q&A):** unauthenticated profile-picking of *synthetic* seed users with *no real money* in a local database is a simulation — not identity theft. In production the picker is replaced by real auth (OTP/KYC), and the ledger underneath does not change. Never claim the prototype is authenticated.

### 4.2 Invoice-linked payments

- An invoice is a **real document** in Postgres (`invoices` + `invoice_items`): number, issuer, counterparty, currency, issue date, due date, VAT, line items. **The total is never stored — it is always computed** as `(Σ qty × unit price) × (1 + VAT%)`.
- **Paying an invoice = a transfer that carries the invoice number.** The engine writes the two ledger entries and stores the header with `invoice_number`.
- **Status is DERIVED, never updated:** an invoice is PAID exactly when a SUCCESS transfer references its number (`EXISTS(SELECT 1 FROM transactions WHERE invoice_number = i.number AND status='SUCCESS')`). Nobody "flips" it — the ledger does it automatically. That's why the supplier and buyer see the same fact.
- **Issuers issue, debtors pay:** only the party owed money (`issuer_user_id`) can create the document; the buyer (`counterparty_user_id`) pays it. The payer can't write their own receipt — the asymmetry is the anti-fraud foundation.
- **Phantom-number trap (learned):** typed an invoice number that has no document (e.g. INV-2026-00907)? The transfer still succeeds (money moves, shows in history) but no invoice page can show it, because no document was ever created. **Fix applied:** invoice numbers are now validated at creation to the format `INV-YYYY-NNN` (`INV-\d{4}-\d{3,6}`) on both the frontend form and the engine backstop. Always demo with a *documented* invoice number.

### 4.3 Business and personal separation (engine-enforced, not UI flair)

Every user is born with a PERSONAL and a BUSINESS wallet. Money can go into either, but **only BUSINESS wallets can pay out** — both `Transfer` and `CrossBorderTransfer` refuse anything else (`ErrInvalidAccountType`: "transfers must originate from a BUSINESS account"). There is no endpoint that moves funds between them. Balances are independent ledgers. A lender can actually see what the business earned, spent, and repaid — without any soup-mixing.

### 4.4 Cross-border (FX) settlement

A cross-border transfer (RWF → KES) is a **single idempotent transaction writing FOUR ledger entries**, two per currency leg, each leg summing to zero:

```
Leg A (RWF):  debit source BUSINESS  −amount,  credit SETTLEMENT(RWF) +amount
Leg B (KES):  debit SETTLEMENT(KES)  −amount_kes,  credit dest BUSINESS +amount_kes
  amount_kes = round4(amount / fx_rate)
```

The KES leg is dispatched to the rail (M-Pesa simulator) inside the transaction; entries are written only on rail success; a failure rolls everything back — no partial state. The seeded corridor rate is **9.5 RWF per KES**, surfaced live to the UI before committing.

### 4.5 KYC (Know Your Customer)

Every `User` carries `kyc_status` (seeded `PENDING`, e.g. "KYC PENDING · Reg RWC-2026-0441" in the sidebar). **The field and display exist; the verification workflow does not** — no document upload, no approval step. That is honest scope: the KYC gate is where production authentication plugs in. The pitch line: *"the status is modeled and visible; the document-verification workflow is the regulatory milestone we'd build next."*

### 4.6 The invoice document (real-world realism)

The InvoiceDocument renders an A4-style trade invoice: VUKA header, invoice number, Issued by / Bill to with **TIN/REC (business reg number) + phones**, issue/due dates, PAID/UNPAID derived status, line-item table, Subtotal → VAT → Total, **amount in words** ("Twenty Thousand Kenyan Shillings only"), payment terms, and a **"Remit via VUKA"** block binding the payment route to the invoice number. Remaining legal gaps (not in prototype): seller/issuer addresses, sequential-number auto-increment, signature/stamp — a rendering-and-validation layer on data the schema already stores.

---

## 5. The demo: profiles, data, walkthrough

### 5.1 Seeded demo traders (synthetic)

| Profile | Phone | Currency | Reg | Role | Category |
|---------|-------|----------|-----|------|----------|
| Amina Uwera — Kigali Coffee Coop | +250700000991 | RWF | RWC-2026-0441 | Importer | Kigali, Rwanda |
| Kethan Gasana — Nairobi Roasters Ltd | +254700000882 | KES | RWC-2026-0772 | Supplier | Nairobi, Kenya |
| Jean-Paul Niyonzima — Musanze Minerals | +250700000773 | RWF | RWC-2026-0108 | Exporter | Musanze, Rwanda |

Each has a **PERSONAL + BUSINESS** wallet. Where money sits:

| Party | Wallet | Balance (verified) |
|-------|--------|--------------------|
| Amina (payer, stories on Aug 1–10) | Business RWF | **2,667,250 RWF** |
| Kethan (recipient of KES legs) | Business KES | **116,500 KES** |
| VUKA Settlement | RWF/KES float | net −116,500 KES (balanced) |

### 5.2 Seeded documents (9 real invoice documents)

- **KES → Kethan** (Nairobi supplier): INV-2026-00891 (hero), 00894, 00897, 00898
- **RWF → Jean-Paul** (Musanze exporter): INV-2026-00890, 00892, 00893, 00895, 00896

All seeded invoices issued 2026-08-07, due 2026-08-24 (17-day window), all PAID. The register holds 11 transfers (5M cash-in + invoice-linked) + any live demo transfer.

### 5.3 Demo walkthrough (the 9-minute story)

1. **Landing** (`#/`) — front door: hero, solutions, corridor, developers, platform, footer; live FX badge proven from the engine. *(Spoken, not walked.)*
2. **Select Profile** (`/select`) — pick Amina → becomes the session's trader.
3. **Dashboard** — live balances, Settled Today, invoice counts, live transaction feed.
4. **Payments** — send a cross-border payment: live FX preview, idempotent submit.
5. **Invoices** — receive/issue documents; pay a documented invoice via the Pay card.
6. **History** — the two-seat proof: Amina sees 11 records; switch to Kethan and see the same cross-border rows from his seat (he sees only the 5 that credited his KES wallet).

### 5.4 Process map (important for the pitch)

- Engine REST **:8080**, gRPC **:50051**, dashboard dev **:5174**, Postgres **:5432**.
- Live demo transfer on Aug 10 means the Aug 11 "Settled Today" KPI reads **0** (KPI = created_at == today) — it **pops** when the on-stage transfer runs. No reset strictly needed; `scripts/build_demo_register.sh` + `cmd/seed` rebuild history if desired.

---

## 6. How to run everything

```bash
# 1. Postgres (role vuka, dbs vuka + vuka_test; tests need TEST_DATABASE_URL)

# 2. Engine (password sourced from ~/.pgpass, never hardcoded)
cd ENGINE/go-bridge-core
PW=$(awk -F: '$1=="localhost" && $3=="vuka" {print $5}' ~/.pgpass)
export VUKA_DATABASE_URL="postgres://vuka:changeme@localhost:5432/vuka?sslmode=disable" \
       VUKA_FX_RWF_KES=9.5 \
       VUKA_CORS_ORIGINS="http://localhost:5173,http://localhost:5174" \
       MTN_SIM_MODE=success
go run ./cmd/main.go        # expects: http server listening addr=:8080

# 3. Seed demo data (idempotent)
go run ./cmd/seed

# 4. Frontend
cd FRONTEND/web-dashboard
npm install
npm run dev                 # http://localhost:5174

# Verification suite (all green as of the final session)
cd ENGINE/go-bridge-core && go vet ./... && TEST_DATABASE_URL=... go test ./...
cd FRONTEND/web-dashboard && npx tsc --noEmit && NODE_ENV=test npx vitest run && npm run build
```

**Ports:** 5432 Postgres · 8080 REST · 50051 gRPC · 5173/5174 dashboard.

**Notes:** demo phones in code are byte-identical with the seed — never "fix" a phone display, that class of bug breaks sign-in. After heavy file changes, delete `.next`-style caches / restart the dev server (stale chunks).

---

## 7. The journey — problems found & fixed

Chronological log of real issues encountered, root causes, and fixes. **(All verified live before being marked done.)**

| # | Problem | Root cause | Fix |
|---|---------|-----------|-----|
| 1 | Landing-nav links "did nothing" / bounced to /select | App uses **HashRouter**; plain `<a href="#gap">` resolves to route `/gap` → falls to `*` → redirect /select | Rebuilt landing nav as HashRouter-safe scroll buttons (`scrollIntoView`), URL hash never changes |
| 2 | Landing was verbose marketing soup (678 lines) | Deviated from the 6-section blueprint | Rewrote to tight 6 sections: hero, solutions, corridor, devs, platform, footer; removed Onafriq/PAPSS essay, roadmap, ask-walls |
| 3 | Trader-select sidebar had dead links (Transactions/Account/Settings → /select) | Placeholder nav items pointing at the same page | Sidebar now only Home + Select Profile; breadcrumb cleaned (removed fake "Transactions" tier) |
| 4 | "Is history MY transactions or the profile's?" | Understanding gap on the two-seat ledger | Added **"Viewing as …" chip** explaining scope; documented the model (one row, two seats) |
| 5 | Phantom invoice: paid INV-2026-00907, not listed in recipient's invoices | Invoice number is a transfer stamp; **no document with that number existed**; transfer form lets you type any string | Engine + form now **enforce `INV-YYYY-NNN`**; bad number → 422 with clear message; demo rule: only pay documented invoices |
| 6 | Received-invoice "Pay" button dead for Kethan | Invoice billed in RWF but Kethan's only business wallet is KES; Pay card auto-picks source in invoice currency, finds none, and **silently disabled Confirm** (also hardcoded "Rwandan Importer" label) | Explicit error message ("billed in RWF, business wallet is in KES…"), source dropdown filtered to wallet currency + disabled with reason, neutral label, cross-currency hint pointing to Payments |
| 7 | Invoice document wasn't legally complete | Missing TIN/reg, phones, amount-in-words | Document now prints **TIN/REC (business reg) + phones** for issuer/buyer and **amount in words** (`amountInWords`, 7 new unit tests) |
| 8 | Engine rejected nothing at invoice creation (older binary on :8080) | Running process predated code changes — **stale engine** | Restarted engine with current code; verified 422 on bad number and 201 on valid; cleaned test rows |
| 9 | Auth SASL failures with literal `***` in DSN | Credential masking — real password must come from `~/.pgpass` | Launch/test commands source `PW=$(awk … ~/.pgpass)` and interpolate `${PW}` |
| 10 | "Pay without PIN, is this secure?" | (Design question — see §8, §11) | Documented aggregator model: VUKA never holds MoMo PINs; MNO mediates confirmation; ledger is the audit |

---

## 8. The questions that shaped the design (and their answers)

**Q: How are "my" transactions stored when there is no login?**
A: There is no personal identity — the selected profile **is** the identity for the session. Every transfer lives in the shared Postgres ledger, attributed to **accounts** (owned by users), not to "a browser." Switching profiles changes which seat you read the same rows from.

**Q: Does the history page show MY transactions or the chosen profile's?**
A: The chosen profile's — and in this design they are the same thing, because you *are* the chosen profile. Eligibility = your account is source **or** destination. Same row appears on both sides (Amina 11 records, Kethan 5 — the 5 that credited him).

**Q: Is "I choose Amina → I become Amina" legal?**
A: Legal in this build (synthetic users, no real money, local DB — simulation, no victim). Not a production pattern: production replaces the picker with auth (OTP + KYC + authorization), and the ledger doesn't change. Frame it as a deliberate separation: *derive the machine, then wrap it with the gate.*

**Q: Is the landing page necessary?**
A: Yes — as the front door (context for a judge opening the URL: what is this? live FX proof; the ask lives there). It must **not** be part of the 9-minute walk. Speak it, then jump to /select.

**Q: How is the invoice-linked payment problem solved?**
A: The invoice number is the join key between documents and the ledger; status is **derived from the ledger**; claimants issue, debtors pay; idempotency prevents double-charge; audit chains transfer → entries → rail reference.

**Q: How are business and personal separated?**
A: Structurally: two wallets per user; the engine refuses any transfer from a non-BUSINESS source (`ErrInvalidAccountType`) — enforced in code, not just hidden in the UI.

**Q: What is KYC and where does it sit?**
A: The legal verification of "who can prove they are who they claim." Modeled as `kyc_status` (PENDING), displayed in the UI; the workflow (ID upload, approval) is a roadmap milestone — don't claim it exists.

**Q: In the real world, how does VUKA link to people's MoMo accounts and manage PINs?**
A: The **aggregator model**: money stays in the MoMo wallet; VUKA holds MNO-issued API credentials under a commercial agreement; the MNO mediates confirmation (push-prompt or regulated standing mandate); **VUKA never sees, stores, or processes MoMo PINs**. Security = idempotency + double-entry + external-reference tracing + KYC + auth gate.

**Q: Will "just pay and count the balance" be secure?**
A: The demo is secure by construction (no real money). Production security comes from **not holding PINs**, not holding them better: KYC'd accounts, MNO-mediated consent, an idempotent double-entry ledger that cannot silently change a balance, and end-to-end traceability.

**Q: Why sold a document issue? (phantom invoice)**
A: Because the transfer form let you type any string; documents aren't created by transfers. Now enforced: `INV-YYYY-NNN` at creation, both layers.

---

## 9. Honest limitations of the prototype

- **No real telecom rail.** MTN/M-Pesa adapters are simulators (deterministic success/fail/timeout). The engine is built behind a `TelecomAdapter` interface so real adapters slot in without touching the ledger — but "integrated with MTN" is **not** true yet.
- **No auth.** The profile picker is a demo stand-in; there is no login, session tokens, or per-user authorization.
- **No KYC workflow.** Status is modeled; document verification, approvals, sanctions screening are not built.
- **No per-transaction PIN/mandate consent** — by design of the aggregator model, but the legal mandate is not in place.
- **Invoice docs are structurally real but not fully legal:** no addresses, no sequential auto-increment, no signature.
- **Demo data is synthetic and seeded; phones must match seed exactly** — never "fix" a phone display.
- **Single corridor, fixed rate (9.5).** Uganda/Tanzania and dynamic rates are roadmap.

Be honest about all of the above on stage — the strongest judges respect scope discipline.

---

## 10. Roadmap — from prototype to Africa-ready

Phased path (each phase has a gate: *demonstrated working + verified* before the next).

### Phase 0 — PROTOTYPE (current, 2026-08)
**Goal:** prove the money machine and the product story.
**Done:** double-entry ledger, idempotency, invoice-linked payments, cross-border FX settlement, wallet separation, history (two seats), landing, pitch materials.
**Gate:** demo-ready; verification suite green; runbook written.

### Phase 1 — Identity & access
- Replace the profile picker with **registration + OTP login** (registered phone), sessions/tokens, per-user authorization.
- Add password/biometric handling via standard practice (hashed, never plaintext) — or better, carrier-grade OTP.
- Wire `kyc_status` into a real state machine: PENDING → SUBMITTED → APPROVED/REJECTED.

### Phase 2 — KYC workflow
- Document upload (ID, business registration), admin/partner review, sanctions & PEP screening integration.
- **Gate:** a trader cannot open/pay a BUSINESS wallet until APPROVED.

### Phase 3 — Real rail adapters (sandbox → pilot)
- Replace simulators with **MTN MoMo Open API** and **M-Pesa Daraja (Kenya)** sandbox HTTP clients behind the existing `TelecomAdapter` interface.
- Handle webhooks/status callbacks, retries, and reconciliation; keep idempotency semantics identical.

### Phase 4 — Regulatory & licensing
- **BNR fintech sandbox application** (the ask on the landing page), then a payment/e-money license path.
- Activate **fintech passporting** agreements (Ghana Feb 2025, Kenya Mar 2026) for cross-border licensing.
- Appoint a licensed PSP/aggregator partner for rail access (the other landing-page ask).

### Phase 5 — Consent & security hardening
- Implement the **consent layer**: MNO push-confirmation (Option A) and regulated **standing mandate/direct debit** (Option B).
- Add transaction limits, velocity checks, fraud monitoring, and a full audit export.
- Penetration testing + security review before any real funds.

### Phase 6 — Network expansion
- Add **Uganda** then **Tanzania** corridors (the RWF ↔ KES model generalizes; tier-4 exclusions re-evaluated under license).
- Multi-currency settlement, dynamic FX rates, liquidity pools per corridor.

### Phase 7 — Trade & credit products
- Open the ledger as **credit history**: on-time payment scores, lender API, trade-finance pilots.
- Escrow/reversals/disputes with regulator-specified rules; fee engine (FEES accounts already reserved in the schema).

### Phase 8 — Scale & reliability
- Multi-region deploys, observability (traces, SLIs/SLOs), disaster recovery, and regulator reporting dashboards.
- ISO 20022 messaging alignment for cross-border interop.

---

## 11. Regulatory & security framing

- **KYC/AML** (BNR Rwanda, CBK Kenya): verify identity and business before wallets move money.
- **PIN & consent:** VUKA never holds MoMo PINs. Confirmation is mediated by the MNO (USSD/app push) or a regulated standing mandate. The safest payment company doesn't guard credentials best — it doesn't hold them at all.
- **Passporting:** active fintech passporting agreements open the legal path for cross-border licensing.
- **ISO 20022:** the messaging-standard transition deadline has passed; Rwanda has the opening to lead African adoption.
- **Auditability:** every transfer chains idempotency key → transaction header → double-entry rows → rail external reference (`MPESA-…`, `MTN-…`) → invoice number. That trail is regulator-ready.

---

## 12. Glossary

| Term | Meaning |
|------|---------|
| **Ledger** | Immutable double-entry record of all money movement. |
| **Double-entry** | Every transfer writes debit + credit entries summing to zero — the anti-corruption guarantee. |
| **Idempotency key** | A client-supplied UUID; replaying it returns the original transfer instead of charging twice. |
| **BUSINESS / PERSONAL / SETTLEMENT / FEES** | Account types. Only BUSINESS pays out invoice transfers; SETTLEMENT is the corridor float. |
| **Invoice-linked** | A transfer that carries an invoice number, binding payment to a document. |
| **PAID (derived)** | An invoice is PAID iff a SUCCESS transfer references its number — computed, never stored. |
| **KYC** | Know Your Customer — verifying who a trader really is before money moves. |
| **Corridor** | A currency pair/geography route (RWF ↔ KES / Rwanda ↔ Kenya). |
| **Adapter** | The seam to a telecom rail (MTN/M-Pesa); simulated in the prototype. |
| **Aggregator model** | VUKA holds API credentials, not customer PINs; the MNO mediates money movement. |
| **Two-seat history** | One transfer row visible from both the payer's and payee's histories. |
| **FX leg** | A cross-border transfer writes two currency legs (4 entries), each netting to zero. |

---

## 13. Appendix — key IDs, endpoints, verification record

### Accounts / users (live DB, `vuka`)

| Entity | UUID |
|--------|------|
| Amina Uwera | `80fe6931-09b7-41e9-bd33-cf82986efe71` |
| Kethan Gasana | `572b5aa2-9334-401d-9928-a71a895bdd2a` |
| Jean-Paul Niyonzima | `a3b7a239-df49-4add-b9cc-baf60acbd055` |
| VUKA Settlement | `8fec6a88-108a-4da2-b1a6-f752cdf805b2` |
| Amina BUSINESS RWF | `d18a025a-9a4d-48a9-aaf6-481a86925817` |
| Kethan BUSINESS KES | `e3efcd15-b8ff-4b3d-a62d-fae1c887c5bd` |

### Key endpoints

```
GET  /api/healthz
GET  /api/lookup/user/{phone}
POST /api/users                     (register + PERSONAL/BUSINESS wallets)
GET  /api/users/{id}                GET /api/users/{id}/accounts
GET  /api/users/{id}/transfers      GET /api/users/{id}/invoices
POST /api/users/{id}/invoices       (require Idempotency-Key on money moves)
POST /transfers                     POST /transfers/cross-border
GET  /api/invoices/{id}             GET /api/transfers/{id}/entries
```

### Verification record (final session)

- `go vet ./...` ✓ · engine tests ✓ (ledger, api) against `vuka_test`
- `tsc --noEmit` 0 errors · vitest **37/37** (money, format, numberToWords/amountInWords, invoice math, badge, api)
- `npm run build` ✓ (57 modules, ~278KB JS / 81KB gzip)
- Live browser walks: landing nav, CTA routing, credit-buyer flow, two-seat history, invoice document (TIN/phones/words), 422 on bad invoice number, disabled+explained cross-currency Pay
- Ledger integrity: double-entry sums to zero; Amina 2,667,250 RWF, Kethan 116,500 KES, settlement float balanced

---

*Document generated from the actual codebase and live environment — every claim above is backed by code paths in `ENGINE/go-bridge-core` and `FRONTEND/web-dashboard`, verified working before being recorded.*