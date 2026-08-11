# VUKA Engine — Cross-Border SME Trade Payment Network

> **Phase 1 Infrastructure:** Rwanda–Kenya Corridor (Expanding to 3–5 East African Corridors)  
> **Target Audience:** Core Engineering Team, Backend Developers, and Hackathon Evaluators  
> **Architecture Style:** Polyglot Microservices (Go + Python + PostgreSQL + React/TypeScript)

---

## 1. Project Vision & Purpose

**VUKA** ("Rise & Cross Over") is a trade-focused payment and financial ledger application engineered for East African Small and Medium Enterprises (SMEs).

While pan-African networks (PAPSS) and telecom operators (MTN Mobile Money, M-Pesa) provide basic consumer remittance channels, they lack the business tools required for commercial trade. VUKA operates as the **trader-facing execution layer** sitting directly on top of existing mobile money and banking infrastructure.

### Key Market Gaps Solved
* **Trade-Sized Limits:** Replaces consumer USSD transaction caps (~$1,700) with business-tier compliance and verification.
* **Invoice-Linked Transfers:** Binds every payment cryptographically to an invoice or order number, eliminating payment disputes.
* **Personal vs. Business Wallet Isolation:** Uses strict sub-account structures to separate household spending from trade capital.
* **Verifiable Trust Score:** Builds an in-app payment track record between traders and cross-border suppliers to reduce steep upfront deposit requirements.
* **ISO 20022 Compliance:** Natively translates internal transactions into standard financial messaging formats for bank and central bank interoperability.

---

## 2. High-Level System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          1. TRADER DASHBOARD                                │
│                       React + TypeScript (Vite)                             │
│       • Business / Personal Balance View    • Invoice Builder & Tracker     │
└──────────────────────────────────────┬──────────────────────────────────────┘
                                       │ REST / gRPC
┌──────────────────────────────────────▼──────────────────────────────────────┐
│                    2. CORE BRIDGE & LEDGER ENGINE                           │
│                             Go (Golang)                                     │
│   ┌──────────────────────────┐               ┌──────────────────────────┐   │
│   │ Idempotency & Lock Engine│               │ Double-Entry Bookkeeper  │   │
│   └────────────┬─────────────┘               └────────────┬─────────────┘   │
└────────────────┼──────────────────────────────────────────┼─────────────────┘
                 │                                          │
┌────────────────▼─────────────────────┐       ┌────────────▼─────────────────┐
│ 3. MESSAGING GATEWAY                 │       │ 4. DATABASE LEDGER           │
│    Python (FastAPI)                  │       │    PostgreSQL                │
│  • ISO 20022 XML (pacs.008)          │       │  • Double-Entry Accounts     │
│  • Schema Parsing & Validation       │       │  • Ledger Entries (Debits/Cr)│
└────────────────┬─────────────────────┘       └──────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────────────────┐
│ 5. TELECOM & RAIL ADAPTER LAYER                                            │
│    • Rwanda: MTN Mobile Money / Airtel Money Adapter                        │
│    • Kenya: Safaricom M-Pesa / Equity Bank Adapter                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Technology Stack & Microservices Matrix

| Component / Layer | Technology | Primary Role | Core Responsibility |
| :--- | :--- | :--- | :--- |
| **Core Engine** | Go (Golang 1.22+) | Ledger Orchestration & API | Concurrency management, balance validation, idempotency locks, transaction state machine. |
| **Messaging Gateway** | Python 3.11+ (FastAPI) | ISO 20022 Schema Parser | Converts internal JSON transactions to standard `pacs.008.001.10` XML messages and vice versa. |
| **Database** | PostgreSQL 16+ | Double-Entry Accounting | Stores immutable debit/credit entries, account relations, and enforces atomic ACID guarantees. |
| **Frontend** | React 18, TypeScript, Tailwind | Trader Dashboard UI | Provides balance visibility, invoice creation, live status tracking, and webhooks integration. |
| **Inter-Service** | gRPC / Protocol Buffers | Internal Communication | Ultra-low-latency binary transport between Go Core Engine and Python Messaging Gateway. |

---

## 4. End-to-End Payment Sequence Workflow

The flow below illustrates how a **$500 cross-border invoice payment** moves from a Rwandan importer to a Kenyan supplier:

```
[ Trader UI (React) ]         [ Go Core Engine ]          [ Python ISO Service ]      [ PostgreSQL ]          [ Telecom Rail ]
         │                            │                            │                      │                        │
         │── 1. POST /api/transfer ──>│                            │                      │                        │
         │   (Invoice ID, $500, KES)  │                            │                      │                        │
         │                            │── 2. Check Idempotency ───>│                      │                        │
         │                            │   & Lock Sender Account    │                      │                        │
         │                            │────────────────────────────┼─────────────────────>│                        │
         │                            │                            │                      │ (BEGIN TRANSACTION)    │
         │                            │── 3. Query Balance ───────>│                      │                        │
         │                            │   SUM(ledger_entries)      │                      │                        │
         │                            │<───────────────────────────┼──────────────────────│                        │
         │                            │                            │                      │                        │
         │                            │── 4. Serialize JSON ──────>│                      │                        │
         │                            │   to pacs.008 XML (gRPC)   │                      │                        │
         │                            │<── XML Payload Returned ───│                      │                        │
         │                            │                            │                      │                        │
         │                            │── 5. Dispatch Payout Payload ─────────────────────────────────────────>│
         │                            │                            │                      │                        │
         │                            │<── 6. Callback HTTP 200 OK (Transfer Cleared) ──────────────────────────│
         │                            │                            │                      │                        │
         │                            │── 7. Write Double-Entry Entries ─────────────────>│                        │
         │                            │   • Debit (-$500) Sender Business Acc             │                        │
         │                            │   • Credit (+$500) Settlement Acc                 │ (COMMIT)               │
         │<── 8. WebSocket Update ────│                            │                      │                        │
         │    (Status: SUCCESS)       │                            │                      │                        │
```

---

## 5. Database Architecture & Double-Entry Rules

VUKA adheres strictly to **immutable double-entry bookkeeping rules**.

> **Accounting Rule:** Account records **NEVER** contain a static `balance` column. An account balance is calculated dynamically as:
> $$Balance = \sum ledger\_entries.amount$$
> Every transaction MUST write exactly **two entries** (a Debit and a Credit) whose values sum to zero.

### PostgreSQL Schema Blueprint

```sql
-- Create ENUM types
CREATE TYPE account_type AS ENUM ('PERSONAL', 'BUSINESS', 'SETTLEMENT', 'FEES');
CREATE TYPE transaction_status AS ENUM ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED', 'REVERSED');

-- 1. Accounts Table (Personal vs Business Isolation)
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    type account_type NOT NULL,
    currency VARCHAR(3) NOT NULL, -- e.g., 'RWF', 'KES', 'UGX'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Transactions Table (Header Record)
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    invoice_number VARCHAR(100),
    source_account_id UUID NOT NULL REFERENCES accounts(id),
    destination_account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18, 4) NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    status transaction_status NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 3. Ledger Entries Table (Double-Entry Log)
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(18, 4) NOT NULL, -- Negative for Debit, Positive for Credit
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for Speed & Performance
CREATE INDEX idx_ledger_account_id ON ledger_entries(account_id);
CREATE INDEX idx_transactions_idempotency ON transactions(idempotency_key);
```

---

## 6. Directory Structure

```text
vuka-engine/
├── apps/
│   ├── web-dashboard/            # React + TypeScript + Tailwind UI
│   │   ├── src/
│   │   │   ├── components/       # Invoice forms, Wallet views, History
│   │   │   ├── hooks/            # SSE and API query hooks
│   │   │   └── pages/
│   │   └── package.json
│   │
│   ├── go-bridge-core/           # Go Core Ledger Engine
│   │   ├── cmd/main.go           # Entrypoint
│   │   ├── internal/
│   │   │   ├── ledger/           # Double-entry logic & atomic transfers
│   │   │   ├── idempotency/      # Redis / DB locking mechanisms
│   │   │   ├── adapters/         # Telecom mock interfaces (MTN, M-Pesa)
│   │   │   └── grpc/             # gRPC client for Python gateway
│   │   └── go.mod
│   │
│   └── python-iso-gateway/       # Python Messaging Gateway
│       ├── app/
│       │   ├── main.py           # FastAPI & gRPC server
│       │   ├── serializers/      # JSON <-> ISO 20022 XML converters
│       │   └── schemas/          # pacs.008 & pacs.002 XSD validation
│       └── requirements.txt
│
├── deploy/
│   ├── docker-compose.yml        # Orchestration for local development
│   └── init.sql                  # PostgreSQL database migrations
└── README.md
```

---

## 7. Developer Onboarding & Local Setup

### Prerequisites
* **Docker & Docker Compose** (v24.0+)
* **Go** (v1.22+)
* **Python** (v3.11+)
* **Node.js** (v20+) & **pnpm** / **npm**

### Step 1: Clone Repository & Configure Environment
```bash
git clone https://github.com/vuka-app/vuka-engine.git
cd vuka-engine

# Copy example environment files
cp .env.example .env
```

### Step 2: Spin Up Infrastructure (PostgreSQL + Redis + Python Service)
```bash
docker-compose -f deploy/docker-compose.yml up -d
```

### Step 3: Run Go Core Engine
```bash
cd apps/go-bridge-core
go run cmd/main.go
```

### Step 4: Run React Frontend Dashboard
```bash
cd apps/web-dashboard
npm install
npm run dev
```
The UI will be accessible at `http://localhost:5173`.

---

## 8. Corridor Expansion Roadmap

```
  Phase 1 (Hackathon MVP)          Phase 2 (Pilot)                    Phase 3 (Scale)
┌─────────────────────────┐     ┌─────────────────────────┐     ┌─────────────────────────┐
│ • Rwanda 🇷🇼 ↔ Kenya 🇰🇪  │     │ • Expand to Uganda 🇺🇬   │     │ • Expand to Tanzania 🇹🇿 │
│ • Double-Entry Engine   │ ──> │ • Telecom Partnership   │ ──> │ • Full Central Bank     │
│ • Mock Telecom Adapters │     │ • Live Pilot Cohort     │     │   Sandbox Licensing     │
│ • Invoice-Linked Claims │     │ • Real Rail Settlement  │     │ • Expanded Corridors    │
└─────────────────────────┘     └─────────────────────────┘     └─────────────────────────┘
```

---

## 9. Contributor Guidelines

1. **Never update account balances directly** via simple `UPDATE accounts SET balance = ...`. All balance operations must write linked `ledger_entries`.
2. **Enforce Idempotency:** Every transfer request must accept an `Idempotency-Key` header.
3. **Commit Messages:** Follow standard conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`).
