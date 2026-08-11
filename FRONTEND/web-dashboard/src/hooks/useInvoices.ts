import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { CreateInvoiceRequest, Invoice } from '../api/types'

export interface InvoiceActionResult {
  ok: boolean
  invoice?: Invoice
  error?: string
}

export function useInvoices(traderId: string | null, direction?: 'issued' | 'received'): {
  invoices: Invoice[]
  loading: boolean
  error: string | null
  refresh: () => void
  createInvoice: (req: CreateInvoiceRequest) => Promise<InvoiceActionResult>
} {
  const [invoices, setInvoices] = useState<Invoice[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)

  const refresh = useCallback(() => setTick((n) => n + 1), [])

  useEffect(() => {
    if (!traderId) return
    let cancelled = false
    setLoading(true)
    setError(null)

    api
      .listInvoices(traderId, direction)
      .then(({ invoices }) => {
        if (!cancelled) setInvoices(invoices || [])
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [traderId, direction, tick])

  const createInvoice = useCallback(
    async (req: CreateInvoiceRequest): Promise<InvoiceActionResult> => {
      if (!traderId) return { ok: false, error: 'No active trader' }
      try {
        const inv = await api.createInvoice(traderId, req)
        setTick((n) => n + 1)
        return { ok: true, invoice: inv }
      } catch (err) {
        return { ok: false, error: (err as Error).message }
      }
    },
    [traderId],
  )

  return { invoices, loading, error, refresh, createInvoice }
}
