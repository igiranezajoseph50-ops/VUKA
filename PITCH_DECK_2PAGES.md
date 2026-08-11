# VUKA — Pitch Deck (2 pages, text)

**VUKA — Rise & Cross Over**
National Bank of Rwanda FinTech Innovation Hackathon 2026
Corridor: Rwanda ↔ Kenya (RWF ↔ KES) · MTN MoMo → M-Pesa

---

## PAGE 1 — PROBLEM STATEMENT, PROPOSED SOLUTION

### Problem statement

East Africa's mobile-money networks — MTN MoMo, M-Pesa, Airtel Money, Tigo — are largely closed
ecosystems that transact poorly across borders. A trader in Kigali paying a supplier in Nairobi or Kampala
is often forced to withdraw cash, pay high fees, and wait days for funds to clear, or to physically carry
money across a border. Connectivity between providers is limited and immature.

Where cross-border movement does exist, it was built for personal remittances — a person sending money
home to family — not for business trade. This produces five concrete gaps that directly affect a small
trader running a cross-border business:

1. Transaction limits far below what a real bulk trade payment requires.
2. No way to link a payment to a specific order or invoice, so neither party has a clear record of what
   was paid for.
3. No separation between a trader's personal spending and business income, making bookkeeping and tax
   preparation nearly impossible.
4. No visible trust or payment history between a buyer and a new supplier, forcing traders into paying
   50–60% upfront before goods are shipped.
5. An interface limited to blind USSD menus — no live confirmation, no visible exchange rate before
   committing, no reviewable history.

The result: suppliers demand large upfront deposits, and SMEs cannot build the credit history lenders
need to grow.

### Proposed solution

VUKA is the bridge and the trade layer in one project. Its interoperability engine connects mobile
money wallets across borders, making cross-border movement instant and affordable without depending on
banks or correspondent routing. On top of that bridge, VUKA provides the trader-facing layer that
personal remittance rails were never designed to give a business:

- **Invoice-linked payments** — a transfer is tied to an order or invoice, so both parties keep a clear,
  timestamped record of what the payment was for.
- **Business and personal money, separated** — every trader holds a distinct business balance apart from
  their personal wallet, enforced at the engine level: the first step toward bookkeeping, tax readiness,
  and credit history.
- **Trade-sized transactions** — limits and compliance are designed for business-scale payments, not
  personal-remittance ceilings.
- **Visible trust and payment history** — a supplier can see a buyer's on-time record before agreeing
  terms, reducing upfront deposits.
- **A real interface** — live confirmation, a visible exchange rate before commitment, a reviewable
  history, and status from initiation to settlement.

Every transfer is recorded on one double-entry ledger with two perspectives: the payer and the payee see
the same immutable record, so there is no reconciliation dispute.

---

## PAGE 2 — SOLUTION ARCHITECTURE, ADOPTION STRATEGY

### Solution architecture

VUKA operates on existing mobile-money infrastructure rather than competing with it. The system is a
five-layer architecture built for the reliability that moving real money requires:

1. **Interoperability Engine** — holds liquidity pools across telecom networks and rebalances funds
   across borders, avoiding dependence on banks or international correspondent routing.
2. **Messaging Standard Layer (ISO 20022)** — translates every transaction into the global ISO 20022
   format, eliminating the data mismatches that cause failed transfers and manual intervention.
3. **Bridge & Ledger Engine (Go)** — core money-movement logic with safe concurrency, so simultaneous
   transfers can never corrupt a balance.
4. **Ledger (PostgreSQL)** — strict double-entry accounting: every transfer is a linked debit and credit
   that must net to zero, and no balance is ever updated directly — balances are always derived.
5. **Dashboard (React)** — the live, trader-facing interface: confirmation, transaction history, and
   status tracking.

Five reliability principles are non-negotiable across every layer: double-entry bookkeeping for every
transfer, idempotency keys to prevent duplicate charges, automatic reversal on partial failure, full
end-to-end transaction tracing, and daily reconciliation.

**Current phase (demonstrated live):** Phase 1 — the ledger engine, transfer engine, settlement
accounts, and the trade layer are implemented and running on the Rwanda–Kenya corridor (RWF ↔ KES at a
live corridor rate), with the MTN MoMo and M-Pesa rail adapters simulated behind a real adapter seam.
Invoice-linked cross-border settlement, business/personal separation, and the two-seat payment history
are working end to end. ISO 20022 emission, live telecom API integration, and multi-network rebalancing
are the Phase 2 pilot build on the same roadmap.

### Adoption strategy

- **Corridor-first.** Launch the Rwanda–Kenya pilot on the mobile-money rails that already exist: prove
  the engine, then widen the bridge. VUKA does not attempt to replace payment rails; it builds the layer
  on top of them.
- **Regulatory.** Apply to the BNR FinTech sandbox and pursue a licensed path, leveraging active fintech
  passporting agreements with Ghana (February 2025) and Kenya (March 2026).
- **Partners.** Secure telecom or aggregator partner access (e.g. MTN, M-Pesa, or an existing
  aggregator) for real rail integration on the KES leg, and onboard a small cohort of real SME traders
  for pilot testing.
- **Proof through the ledger.** Every settled payment builds the trader's verifiable credit history —
  the wedge for lenders and trade finance, and the economic foundation for scaling beyond the corridor
  to Uganda, Tanzania, and the region.

**Built for Rwanda. Designed for Africa's traders.**

---

*Submitted to the National Bank of Rwanda FinTech Innovation Hackathon 2026 · Stage 2.*