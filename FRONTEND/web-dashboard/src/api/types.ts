// Domain types mirroring the Go engine's JSON contract (Phase 1/3).

export type AccountType = 'PERSONAL' | 'BUSINESS' | 'SETTLEMENT' | 'FEES'
export type TransferStatus = 'PENDING' | 'PROCESSING' | 'SUCCESS' | 'FAILED' | 'REVERSED'

export interface User {
  id: string
  full_name: string
  phone_number: string
  business_reg_number?: string
  kyc_status: string
  created_at: string
}

export interface Account {
  id: string
  user_id: string
  type: AccountType
  currency: string
  created_at: string
}

export interface Balance {
  account_id: string
  type: AccountType
  currency: string
  amount: number
}

export interface Transfer {
  id: string
  idempotency_key: string
  invoice_number?: string
  source_account_id: string
  destination_account_id: string
  amount: number
  currency: string
  fx_rate?: number
  status: TransferStatus
  external_reference?: string
  failure_reason?: string
  created_at: string
  updated_at: string
}

export interface LedgerEntry {
  id: string
  transaction_id: string
  account_id: string
  amount: number
  created_at: string
}

export interface CreateUserRequest {
  full_name: string
  phone_number: string
  business_reg_number?: string
}

export interface TransferRequest {
  source_account_id: string
  destination_account_id: string
  amount: number
  currency: string
  invoice_number?: string
}

export interface CrossBorderRequest {
  source_account_id: string
  destination_account_id: string
  amount: number
  currency_from: string
  currency_to: string
  fx_rate?: number
  invoice_number?: string
}

export interface FundRequest {
  amount: number
  currency: string
  reference?: string
}

export interface TransferResponse {
  transfer: Transfer
  replayed?: boolean
}

/** SSE push payload (see Go server sse.go). */
export interface TransferEvent {
  id: string
  status: TransferStatus
  account_id?: string
  amount?: number
  currency?: string
  updated_at: string
}

export interface InvoiceItem {
  id: string
  invoice_id: string
  description: string
  quantity: number
  unit_price: number
  created_at: string
}

export interface Invoice {
  id: string
  number: string
  issuer_user_id: string
  counterparty_user_id: string
  currency: string
  issue_date: string
  due_date: string
  vat_rate: number
  terms?: string
  notes?: string
  status: 'ISSUED' | 'PAID'
  items: InvoiceItem[]
  created_at: string
  updated_at: string
}

export interface InvoiceItemRequest {
  description: string
  quantity: number
  unit_price: number
}

export interface CreateInvoiceRequest {
  number: string
  counterparty_user_id: string
  currency: string
  issue_date: string
  due_date: string
  vat_rate: number
  terms?: string
  notes?: string
  items: InvoiceItemRequest[]
}