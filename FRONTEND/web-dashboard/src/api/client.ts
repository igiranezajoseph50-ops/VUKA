// Typed API client for the VUKA Go engine REST surface.
//
// Uses the native fetch API. The Vite dev server proxies /api to :8080
// (see vite.config.ts); in production the dashboard is served from an origin
// that CORS-allows the engine. All money amounts are handled as JS numbers
// in the UI layer (the engine persists NUMERIC(18,4)).
import type {
  Account,
  Balance,
  CreateUserRequest,
  CrossBorderRequest,
  FundRequest,
  LedgerEntry,
  Transfer,
  TransferRequest,
  TransferResponse,
  User,
  Invoice,
  CreateInvoiceRequest,
} from './types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const BASE = '/api'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`${BASE}${path}`, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        ...(init?.headers ?? {}),
      },
    })
  } catch (err) {
    throw new ApiError(0, 'network_error', `Cannot reach the VUKA engine (${path})`)
  }

  if (!res.ok) {
    let code = 'error'
    let message = `Request failed (${res.status})`
    try {
      const body = await res.json()
      code = body?.error?.code ?? code
      message = body?.error?.message ?? message
    } catch {
      // non-JSON error body — keep defaults
    }
    throw new ApiError(res.status, code, message)
  }

  return (await res.json()) as T
}

/** Generate a fresh crypto-grade UUIDv4 idempotency key. */
export function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  // Fallback for non-secure contexts.
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

export const api = {
  // -- users / accounts -----------------------------------------------------
  createUser: (req: CreateUserRequest, currency = 'RWF') =>
    request<{ user: User; accounts: Account[] }>(
      `/users?currency=${encodeURIComponent(currency)}`,
      {
        method: 'POST',
        body: JSON.stringify(req),
      },
    ),

  getUser: (id: string) => request<User>(`/users/${id}`),
  getUserByPhone: (phone: string) => request<User>(`/lookup/user/${encodeURIComponent(phone)}`),
  getAccounts: (userId: string) => request<{ accounts: Account[] }>(`/users/${userId}/accounts`),
  getAccount: (id: string) => request<Account>(`/accounts/${id}`),
  getBalance: (accountId: string) => request<Balance>(`/accounts/${accountId}/balance`),

  // transfers ----------------------------------------------------------------
  listTransfers: (userId: string, params?: { account_id?: string; status?: string }) => {
    const qs = new URLSearchParams()
    if (params?.account_id && params.account_id.length > 0) qs.set('account_id', params.account_id)
    if (params?.status) qs.set('status', params.status)
    const query = qs.toString()
    return request<{ transfers: Transfer[] }>(`/users/${userId}/transfers${query ? `?${query}` : ''}`)
  },

  getTransfer: (id: string) => request<Transfer>(`/transfers/${id}`),
  getTransferEntries: (id: string) => request<{ entries: LedgerEntry[] }>(`/transfers/${id}/entries`),

  createTransfer: (req: TransferRequest, idempotencyKey = newIdempotencyKey()) =>
    request<TransferResponse>('/transfers', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(req),
    }),

  // Cross-border RWF<->KES through the engine's FX settlement flow. The REST
  // handler mirrors the Phase 2 gRPC corridor (see api/server.go). Requires
  // an Idempotency-Key like every transfer.
  crossBorderTransfer: (req: CrossBorderRequest, idempotencyKey = newIdempotencyKey()) =>
    request<TransferResponse>('/transfers/cross-border', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(req),
    }),

  getFxRate: () => request<{ pair: string; rate: number; updated: string; source: string }>('/fx'),

  fundAccount: (accountId: string, req: FundRequest, idempotencyKey = newIdempotencyKey()) =>
    request<TransferResponse>(`/accounts/${accountId}/fund`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(req),
    }),

  createInvoice: (userId: string, req: CreateInvoiceRequest) =>
    request<Invoice>(`/users/${userId}/invoices`, {
      method: 'POST',
      body: JSON.stringify(req),
    }),

  listInvoices: (userId: string, direction?: 'issued' | 'received') => {
    const qs = new URLSearchParams()
    if (direction) qs.set('direction', direction)
    const query = qs.toString()
    return request<{ invoices: Invoice[] }>(`/users/${userId}/invoices${query ? `?${query}` : ''}`)
  },

  getInvoice: (id: string) => request<Invoice>(`/invoices/${id}`),
}