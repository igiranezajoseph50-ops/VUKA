import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ApiError, api, newIdempotencyKey } from '../src/api/client'

describe('ApiError', () => {
  it('carries status and machine code', () => {
    const err = new ApiError(422, 'insufficient_funds', 'no funds')
    expect(err.status).toBe(422)
    expect(err.code).toBe('insufficient_funds')
    expect(err.message).toBe('no funds')
  })
})

describe('newIdempotencyKey', () => {
  it('returns a UUID-shaped key', () => {
    const key = newIdempotencyKey()
    expect(key).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    )
  })

  it('returns distinct keys on successive calls', () => {
    expect(newIdempotencyKey()).not.toBe(newIdempotencyKey())
  })
})

describe('api client', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn())
  })

  it('builds the correct URL for createUser with currency param', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ user: { id: 'u1' }, accounts: [] }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await api.createUser({ full_name: 'A', phone_number: '+250****1' }, 'KES')
    const [url, init] = vi.mocked(fetch).mock.calls[0]
    expect(url).toBe('/api/users?currency=KES')
    expect(init?.method).toBe('POST')
    expect(JSON.parse(init?.body as string)).toEqual({
      full_name: 'A',
      phone_number: '+250****1',
    })
  })

  it('maps a 422 error body to ApiError', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({ error: { code: 'insufficient_funds', message: 'no funds' } }),
        { status: 422, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    await expect(
      api.getBalance('00000000-0000-4000-8000-000000000000'),
    ).rejects.toMatchObject({ status: 422, code: 'insufficient_funds' })
  })

  it('maps network failure to a network_error ApiError', async () => {
    vi.mocked(fetch).mockRejectedValue(new TypeError('fetch failed'))
    await expect(api.getBalance('abc')).rejects.toMatchObject({ code: 'network_error' })
  })

  it('sends the Idempotency-Key header on transfers', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(
        JSON.stringify({ transfer: { id: 't1' }, replayed: false }),
        { status: 201, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    await api.createTransfer(
      {
        source_account_id: 's',
        destination_account_id: 'd',
        amount: 100,
        currency: 'RWF',
      },
      'fixed-key',
    )
    const [url, init] = vi.mocked(fetch).mock.calls[0]
    expect(url).toBe('/api/transfers')
    expect(init?.headers).toMatchObject({ 'Idempotency-Key': 'fixed-key' })
  })

  it('listTransfers appends query params when provided', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ transfers: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await api.listTransfers('u1', { account_id: 'a1', status: 'SUCCESS' })
    const [url] = vi.mocked(fetch).mock.calls[0]
    expect(url).toBe('/api/users/u1/transfers?account_id=a1&status=SUCCESS')
  })

  it('getUserByPhone builds the lookup URL', async () => {
    vi.mocked(fetch).mockResolvedValue(
      new Response(JSON.stringify({ id: 'u9', full_name: 'A' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await api.getUserByPhone('+250****0301')
    const [url] = vi.mocked(fetch).mock.calls[0]
    // encodeURIComponent leaves '*' unescaped (RFC 3986 unreserved).
    expect(url).toBe('/api/lookup/user/%2B250****0301')
  })
})