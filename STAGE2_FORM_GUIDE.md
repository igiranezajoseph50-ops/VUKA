# VUKA — Stage 2 Submission Form: Field-by-Field Fill Guide

**Form:** Fintech Innovation Hackathon 2026 — Stage 2 Submission Form (Microsoft Forms)
**Deadline:** August 12, 2026, 12:00 PM
**What the form collects:** 8 fields — contact info, company, topic, and **three LINKS** (pitch deck, demo video, working platform).

---

## ⚡ LIVE LINKS (verified 2026-08-11 — engine single-port + cloudflared tunnel)

| Field | URL | Status |
|---|---|---|
| **6. Pitch deck** | `https://lap-sandy-rogers-thursday.trycloudflare.com/deck` | ✅ 200 · live page (6 rubric points covered) |
| **7. Demo video** | *YouTube upload pending* — local file: `~/Videos/vuka-demo-video-3min.mp4` (2:51, H.264 720p, 11.9 MB) | ✅ compliant with ≤3:00 cap |
| **8. Working platform** | `https://lap-sandy-rogers-thursday.trycloudflare.com/` | ✅ 200 · SPA + API live (FX 9.5, ledger, invoices) |

> ⚠️ The tunnel URL is temporary — it dies when this machine sleeps or the tunnel stops. Keep the machine + tunnel running through the evaluation window (Aug 12+), or switch to Option 2 (PaaS) below for a durable link.

---

## THE 8 FIELDS — WHAT EACH ASKS AND HOW TO FILL IT

### 1. Full Name (Contact Person) *
**How to fill:** Your real name — the person judges can contact. *(You fill this in — it's personal.)*

### 2. Email Address *
**How to fill:** The email you registered with for the hackathon (the one that received sandbox@bnr.rw emails).

### 3. Phone Number *
**How to fill:** A working phone number in international format (e.g. +250 7XX XXX XXX).

### 4. Company Name *
**How to fill:**
```
VUKA
```
*(Optionally add: "VUKA — Cross-Border Trade Payments" or "VUKA (Rise & Cross Over)".)*

### 5. Please select the topic you responded to during the application process *
**How to fill:** Select option **C**:
> **C. Advancing Cross-Border Payments and Financial Connectivity**

*This is the only correct choice — VUKA is a cross-border payments solution. The other topics (savings mobilisation, MSME credit, digital insurance) do not match.*

---

### 6. Please send us the pitch deck link for reference *
**What it asks:** A **link** to your pitch deck (hosted, public) — the form explicitly wants the deck to cover SIX items:

1. Problem statement
2. Innovation and differentiation
3. Readiness for testing and market deployment
4. Expected impact + implementation plan + scaling
5. User journey
6. Payment process flow

**How to fill (link):**
- Upload `PITCH_DECK_2PAGES.md` (or its PDF export) to Google Drive, OneDrive, or Dropbox.
- Set the file to **"Anyone with the link can view."**
- Paste the share link in this field.

**How to fill (paste-alongside text — recommended):** even with the link, paste a short summary of all six rubric points so the committee sees the content without leaving the form:

```
VUKA is the bridge and the trade layer for East African cross-border payments — one
project, Phase 1 executed.

PROBLEM STATEMENT: East African mobile-money networks (MTN MoMo, M-Pesa, Airtel,
Tigo) are largely closed ecosystems that transact poorly across borders. Where
cross-border movement exists, it was built for personal remittances, not trade: no
invoice context, business and personal money mixed, no visible trust history, no
trade-sized limits, and blind USSD interfaces. SMEs suffer disputes, 50–60% upfront
deposits, and no path to credit history.

INNOVATION & DIFFERENTIATION: VUKA is not another remittance rail — it is the
trader-facing layer on top of existing rails. Invoice-linked payments bind every
transfer to a commercial document; a strict double-entry ledger gives both sides one
immutable record (payer and payee see the same row); business/personal wallets are
separated and enforced at the engine level; and settlement runs through a liquidity
pool across the RWF→KES corridor. Existing alternatives move money without commercial
context; VUKA adds the context that makes money usable for trade.

READINESS FOR TESTING & MARKET DEPLOYMENT: Phase 1 is implemented and running: Go
bridge/ledger engine (REST :8080, gRPC :50051), PostgreSQL double-entry ledger,
invoice-linked cross-border settlement with idempotency keys, business/personal
separation, and a React operator dashboard with live FX and realtime events. MTN MoMo
and M-Pesa rail adapters are simulated behind a real adapter seam; the team's next
milestones are live rail integration, ISO 20022 emission, and the BNR sandbox path.

EXPECTED IMPACT & SCALING: Lower cross-border payment friction for SME traders;
verifiable settlement history that becomes credit history; business-level bookkeeping
enabled by wallet separation. Implementation plan: pilot with a small cohort of real
SME traders on the Rwanda–Kenya corridor, then expand to Uganda and Tanzania; scale by
adding adapters for Airtel/Tigo and ISO 20022 compliance.

USER JOURNEY: Trader enters the platform → selects their business profile → sees a
live dashboard with separated business/personal balances → opens an invoice → sends a
cross-border payment with the FX rate shown before committing → the supplier (in
Kenya) receives the KES leg and sees the same record on their own profile → invoice
status flips to PAID automatically from the ledger.

PAYMENT PROCESS FLOW: Payment is initiated against an invoice from a BUSINESS wallet
(only business wallets can pay). The engine claims an idempotency key (no double
charge), locks the source account, dispatches the RWF leg and the KES leg through the
settlement float, and writes exactly two linked ledger entries that net to zero — all
in one atomic transaction. The rail returns an external reference (e.g. MPESA-…), the
transfer is marked SUCCESS, and both parties see the same result.
```

---

### 7. Please send us the link to your demo video of the solution operation *
**What it asks:** A **link** to your ≤3-minute demo video (hosted, public).

**How to fill (link):**
- Record with OBS per `STAGE2_SUBMISSION_PACK.md` → export MP4 (H.264).
- Upload to **YouTube (unlisted or public)** — best option: no expiry, plays everywhere, easy link.
- Paste the YouTube URL in this field.

*Fallback if YouTube is unavailable: Google Drive / OneDrive with "Anyone with the link can view."*

---

### 8. Please submit the link to your working platform *
**What it asks:** A **link to the WORKING PLATFORM** — the actual running product judges can open and click through. This is the most important field and the one that requires real hosting (localhost is NOT enough).

**How to fill (deploy plan — see PART B):**
- Deploy the VUKA app (React frontend + Go engine + Postgres) to a public host.
- Paste the public URL (e.g. `https://vuka.example.com`) in this field.

---

## PART B — HOSTING PLAN (the three links)

### B1. Pitch deck link (field 6)
Easiest: upload the two-page deck to **Google Drive** → Share → "Anyone with the link" → copy link.
Also acceptable: GitHub raw link, Dropbox, OneDrive.

### B2. Demo video link (field 7)
**YouTube** (unlisted) is strongly recommended: no expiry, no login needed to watch, mobile-friendly.
Record the video per the script in `STAGE2_SUBMISSION_PACK.md`, export MP4, upload, paste link.

### B3. Working platform link (field 8) — the critical one
The product currently runs on `localhost` (engine :8080, dashboard :5174). **Judges cannot open localhost.** You must get it onto the internet. Three realistic options, cheapest first:

**Option 1 — Tunnel (quick, free, temporary):** run `ngrok` or `cloudflared` on the machine with the app running. Gives a public `https://…` URL that works while the machine stays on and the tunnel is up.
- Pros: 5 minutes, no cost, no deployment.
- Cons: link dies when the machine sleeps / tunnel stops. **Risk if judges click after you close the laptop.**
- Use only if you can keep the machine + tunnel running through the evaluation window.

**Option 2 — One-click PaaS (recommended balance):** deploy the frontend build (`dist/`) to a static host (Netlify, Vercel, GitHub Pages) and the engine + Postgres to a small VM or container host (Railway, Render, Fly.io, DigitalOcean droplet).
- Pros: permanent link, looks professional, survives the judging window.
- Cons: needs ~1–2 hours of setup, a bit of money (free tiers usually suffice for a demo), and secrets handling (DB password, FX rate).

**Option 3 — Full VPS:** one small Ubuntu VPS running Docker (frontend + engine + Postgres in containers). Same as Option 2 but with full control. Overkill for the demo; choose only if you already have a VPS.

**My recommendation:** if the machine with the running app will stay on through Aug 12+, use **Option 1 (cloudflared tunnel)** for speed; if you want a durable link, use **Option 2 (frontend on Netlify/Vercel + engine on Railway/Render)**. I can prepare the deploy configs (Dockerfile / static build / env template) for either option on request.

---

## PART C — SUBMISSION CHECKLIST (before you hit Submit)

- [x] 4. Company: VUKA
- [x] 5. Topic: **C. Advancing Cross-Border Payments and Financial Connectivity**
- [x] 6. Pitch deck link public + (recommended) the six-point summary pasted → **field 6 URL live** (`…/deck`)
- [x] 8. Working platform link public and clickable → **field 8 URL live** (`…/` — SPA + API verified in browser)
- [ ] 1. Full name filled (personal — you fill this in)
- [ ] 2. Email filled (registered email)
- [ ] 3. Phone filled (international format)
- [ ] 7. Demo video link public (YouTube unlisted) — **upload `~/Videos/vuka-demo-video-3min.mp4`** (2:51, ≤3:00 cap) → paste URL
- [ ] All three links open in a private/incognito window without login
- [ ] Submit before **Aug 12, 2026, 12:00 PM**

---

*Prepared from the actual form screenshots (submission form shows 8 fields: contact ×3, company, topic, pitch deck link, demo video link, working platform link).*