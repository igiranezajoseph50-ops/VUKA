# VUKA — Stage 2 Submission Pack (BNR Fintech Innovation Hackathon 2026)

> **Deadline: 12 August 2026 at 12:00 PM (noon).**
> Submit via the **Stage 2 Submission Form** (link in the reminder email from sandbox@bnr.rw).
>
> - **Pitch Deck** — max **2 pages** (problem statement, proposed solution, solution architecture, adoption strategy).
> - **Demo Video** — max **3 minutes** (functional demo + complete user journey).
>
> Record the video with **OBS Studio**. This file is your script, shot list, and OBS setup.

---

## THE ONE-STORY FRAMING (read this first — it is what keeps Stage 1 and Stage 2 the same project)

VUKA is ONE project with ONE architecture and ONE roadmap. The Stage-1 letter described the **infrastructure vision**; the demo delivers the **first product built on it**. They are not two different projects — they are the foundation and the application:

> **VUKA — Rise & Cross Over.** A universal mobile-money bridge for East Africa, with a trade-facing layer built on top of it.
> - **The bridge (the letter's story):** East African mobile-money networks operate as largely closed ecosystems. VUKA's interoperability layer connects wallets across borders — MTN MoMo, M-Pesa, Airtel, Tigo — through liquidity pools and a standard message format, making cross-border movement instant and affordable.
> - **The trade layer (the demo's story):** Connectivity alone isn't trade. On top of the bridge, VUKA provides invoice-linked payments, separated business and personal ledgers, trade-sized limits, visible trust history, and a real operator interface — exactly what personal remittance rails were never designed to give a business.
> - **One engine, five layers, five reliability principles.** Everything shown in the demo runs on the same five-layer architecture and the same non-negotiable reliability doctrine the letter promised: double-entry bookkeeping, idempotency keys, automatic reversal on partial failure, full end-to-end tracing, and daily reconciliation.
> - **One corridor, one roadmap.** Both documents point at the same start: the Rwanda–Kenya pilot corridor (RWF ↔ KES), then Uganda and Tanzania, under the BNR sandbox and fintech passporting agreements.

**Both descriptions are the same project at different altitudes.** When presenting, use the bridge as the vision, the demo as the proof that the engine underneath works, and the roadmap as the path that connects them. Never present the trade layer as "a change of direction" from the letter — present it as **the letter's first product**.

---

## PART 1 — THE 2-PAGE PITCH DECK (what goes on each page)

### Page 1 — Problem & Solution

**Title block (top):**
- VUKA — *Rise & Cross Over: the mobile-money bridge for East African trade*
- National Bank of Rwanda FinTech Innovation Hackathon 2026 · BNR
- Corridor: Rwanda ↔ Kenya (RWF ↔ KES) · MTN MoMo → M-Pesa (designed for Airtel, Tigo, and more)

**Problem statement (left ~half):**
- East Africa's mobile-money networks are largely **closed ecosystems** that transact poorly across borders — a Kigali trader paying a supplier often faces cash, high fees, and days of delay.
- Where cross-border movement does exist, it was **built for personal remittances, not business trade**: no invoice context (disputes = "I paid" vs "you never did"), business and personal money mixed, no visible payment history, no trade-sized compliance.
- Result: suppliers demand large upfront deposits; SMEs can't build the credit history lenders need.

**Proposed solution (right ~half):**
- **VUKA is the bridge AND the trade layer.** An interoperability engine connects wallets across borders; a trade-facing application makes that connectivity usable for business.
- 5 trade gaps closed: invoice-linked payments, business/personal separation, trade-size limits, visible trust history, a real interface.
- One double-entry ledger, two seats: payer and payee see the same immutable record.

### Page 2 — Architecture & Adoption

**Solution architecture (top ~half):**
- **The unified five-layer stack (as described in the Stage-1 letter):**
  1. Interoperability Engine — liquidity pools across telecom networks, rebalancing funds across borders without banks or correspondent routing.
  2. Messaging Standard Layer (ISO 20022) — standard message format removing the data mismatches that cause failed transfers and manual intervention.
  3. Bridge & Ledger Engine (Go) — core money-movement logic with safe concurrency; REST :8080 / gRPC :50051.
  4. Ledger (PostgreSQL) — strict double-entry accounting; every transfer is a linked debit and credit netting to zero; balances are always derived, never stored.
  5. Dashboard (React) — live operator interface: confirmation, history, status.
- **Reliability doctrine (all five from the letter):** double-entry bookkeeping · idempotency keys (no duplicate charges) · automatic reversal on partial failure · full end-to-end tracing · daily reconciliation.
- **Demo scope honesty:** the video demonstrates the **ledger engine and trade layer** running live (layers 3–5 fully implemented, layer 1 settlement accounts implemented). ISO-20022 emission and full liquidity-pool rebalancing across live telecoms are the **pilot build** — the roadmap from the same letter.
- Identity: KYC status modeled; auth is the milestone wrapping the same engine. PINs stay with the MNO (aggregator model) — VUKA never holds them.

**Adoption strategy (bottom ~half):**
- **Corridor-first:** launch the Rwanda–Kenya pilot (RWF ↔ KES) on mobile-money rails that already exist; prove the engine, then widen the bridge.
- **Regulatory:** apply to BNR FinTech sandbox → licensed path; leverage active fintech passporting agreements (Ghana Feb 2025, Kenya Mar 2026).
- **Partners:** telecom/aggregator integration for the KES leg and beyond; onboard a small cohort of real SME traders for pilot testing.
- **Proof:** the ledger itself becomes the trader's credit history — the wedge for lenders and trade finance.

---

## PART 2 — THE 3-MINUTE DEMO VIDEO SCRIPT

### Rules of the video
- **Max 3:00.** Aim 2:45–2:55 so the upload has margin.
- **Complete user journey:** Amina (Kigali buyer) sends a cross-border payment to Kethan (Nairobi supplier); then we switch to Kethan's seat and prove he sees the same record.
- Every screen is the **real running system** (engine on :8080, dashboard on :5174).
- One take style: speak, click, show. No cuts needed if you rehearse; cuts are fine if you keep it under 3:00.
- Phone/reg data on screen is synthetic demo data — this is fine (sandbox numbers), do not "fix" them.

---

### Script (SAY = narration · SHOW = what's on screen in OBS)

#### 0:00 – 0:20 · Hook + problem (the bridge + the trade gap)
- **SHOW:** Landing page hero (`http://localhost:5174/#/`). The LIVE FX badge and the ledger card (101,500 RWF → 10,684 KES, "Net: 0.00 — balanced").
- **SAY:**
  > "East Africa's mobile money is a set of closed ecosystems — MTN MoMo, M-Pesa — that move money brilliantly within their own borders and awkwardly across them. VUKA's vision is the bridge between them. But connectivity alone isn't trade: no invoice context, business and personal money mixed, no visible history. So on top of that bridge, VUKA builds what trade actually needs — and this is the machine doing it."

#### 0:20 – 0:45 · The buyer's workspace (the trade layer)
- **SHOW:** Click **Get started** → Select Profile (`/#/select`) → click **Amina Uwera · Kigali Coffee Coop** → Dashboard (`/#/dashboard`).
- **SHOW:** Point (cursor) at the wallet cards: **Business Wallet · RWF 2.7M** and **Personal Wallet · RWF 0.00**, then the live transaction feed.
- **SAY:**
  > "Meet Amina, a Kigali coffee importer. Her business and personal money live in two separate ledgers — enforced by the engine, not by the interface. You can't pay a supplier from a personal wallet even if you try. That separation is the first brick of a credit history."

#### 0:45 – 1:35 · The payment (the bridge in action: cross-border settlement)
- **SHOW:** Open **Payments** via the sidebar link "⇄ Payments" (route `/#/transfers` — the sidebar label is Payments; the route is /transfers). Switch to **Cross-border (RWF → KES)**. As **Source** choose the RWF business wallet. Enter **Amount: 100,000**. Invoice number: **INV-2026-00901** (a documented ref already in the register).
- **SHOW:** The green FX panel: **1 KES = 9.50 RWF** and **Supplier receives ≈ 10,526.32 KES**.
- **SHOW:** Click **Send cross-border payment** → success banner with the transfer id → the live feed updates with the new row.
- **SAY:**
  > "Now the bridge does its job. Amina pays her Nairobi supplier against an invoice — RWF goes out in Rwanda, the KES leg settles in Kenya, routed through VUKA's settlement pool. Watch the rate appear *before* she commits: 9.50 RWF per KES. The transfer carries the invoice number, so the payment and the document are one record. And it's idempotent: if the network retries, the same key replays the same transfer — it can never charge twice."

#### 1:35 – 2:20 · The supplier's seat (the two-seat proof + tracing)
- **SHOW:** Click the profile switch (top-right, `⇄` or back via `/select`) → choose **Kethan Gasana · Nairobi Roasters Ltd** → Transaction History (`/#/history`).
- **SHOW:** The **"VIEWING AS · Kethan Gasana"** chip + the list. The new 100,000 RWF cross-border row is there (MPESA external reference, INV-2026-00901).
- **SHOW:** Open **Invoices** → **View** the matching invoice document (TIN/REC: RWC-2026-0772, phones, amount in words, **PAID**).
- **SAY:**
  > "Here's what the bridge makes possible beyond moving money. Switch seats to the supplier, Kethan. The same row we just created is in *his* history — because the ledger records one transfer with two perspectives: money out for Amina, money in for Kethan. The invoice flips to PAID automatically, because PAID is derived from the ledger — nobody marks it. One record, two seats, fully traceable from our side to the rail's reference."

#### 2:20 – 2:55 · Close + ask (the roadmap back to the full bridge)
- **SHOW:** Landing page footer / **Position** section + **Regulatory & partnership requirement** (BNR sandbox guidance, telecom introductions, regulatory feedback).
- **SAY:**
  > "This is the engine our Stage-1 letter committed to — and this is the first product running on it. The next milestones are on the same roadmap: wire the ISO 20022 message layer into settlement, extend the settlement pool across more telecoms, and take it live with real traders on the Rwanda–Kenya corridor. What we need next: a seat in the BNR sandbox, and an introduction to a telecom partner — and the machine you just watched is ready."

#### 2:55 – 3:00 · End card
- **SHOW:** Final card / cream-dark slide: **VUKA — Rise & Cross Over.** BNR FinTech Innovation Hackathon 2026. *Team name · contacts.*
- **SAY:** "VUKA. Rise and cross over."

---

## PART 3 — OBS STUDIO SETUP (record clean, one-take friendly)

### Scene setup
1. **Source 1 — Window Capture:** select the browser running the VUKA dashboard. **Maximize the window** (F11) before recording.
   - Settings → Filters → add **Sharpen** (amount 0.5) — crisper text in video.
2. **Source 2 — Video Capture Device (optional):** small face-cam inset (bottom-right) — *recommended but optional; judges like seeing the presenter.*
3. **Audio:** only *Microphone/Aux* (turn off Desktop Audio to avoid system pings). Test level around **−12 dB**.
4. **Output:** Settings → Output → Recording: **MP4, 1920×1080, 30 fps** (or higher if your machine handles it). Bitrate ~8–12 Mbps.
5. **Mouse:** keep the cursor visible (default). It guides the viewer through clicks.

### Pre-flight checklist (before Record)
- [ ] Start Postgres, engine (`go run ./cmd/main.go` — wait for "http server listening"), dashboard (`npm run dev`), seed (`go run ./cmd/seed` if needed).
- [ ] **Reset demo state** so "Settled Today" shows a number that pops: optional but nice — run `bash scripts/build_demo_register.sh` + seed, or just accept 0 and let the on-stage transfer pop it.
- [ ] Browser: disable extensions, close notifications, use a **clean profile**. Pre-open only the landing page.
- [ ] Phone silent; no calendar alerts; OS notifications off.
- [ ] Rehearse the script **twice with a stopwatch** before the real take.
- [ ] If a click fails mid-take: **stop, fix, and re-record that segment** — a cut is invisible in the final file; a narrated mistake is not.

### After recording
- Trim the head/tail (2–3 s dead air) with any editor (or OBS → reopen file). Verify final is **≤ 3:00** and under the form's size limit.
- Export as MP4 (H.264) so it plays everywhere.

---

## PART 4 — REMINDERS (things we proved, use them in Q&A/comments)

- **One project, two altitudes:** the Stage-1 letter = the bridge vision; the demo = the first product on the bridge (the trade layer). Say it that way; never imply the letter was a different project.
- The demo is a **prototype on a local ledger**; rail adapters are simulated behind a real `TelecomAdapter` seam — the seam is exactly where real MTN/M-Pesa adapters plug in.
- ISO 20022 emission, live liquidity-pool rebalancing across multiple telecoms, and daily reconciliation are **pilot-build milestones** — present them as the roadmap from the letter, not as already running. The engine today already has: double-entry, idempotency, atomic two-phase (auto-reversal on failure), full tracing, and settlement accounts per currency.
- Do not claim auth, KYC workflow, or MNO integration exist — say they are the *gates we wrap it with*.
- "Settled Today" reads **0** on Aug 11 (KPI = created_at == today) → it pops when the video's transfer runs — that's the intended moment.
- Invoice numbers must be `INV-YYYY-NNN`; use documented numbers (e.g. INV-2026-00901) in the demo, never invent one.
- **Q&A if they compare against the letter:** "The letter promised the bridge and the engine discipline — you just watched the engine discipline running, in the shape of the trade product it powers. The ISO layer, full multi-network rebalancing, and reconciliation are the pilot build on the same roadmap."