// useTransfers(traderId): history + actions (create, fund, reverse).
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { CrossBorderRequest, FundRequest, Transfer, TransferRequest } from '../api/types'

export interface TransferActionResult {
  ok: boolean
  transfer?: Transfer
  replayed?: boolean
  error?: string
}

export function useTransfers(traderId: string | null): {
  transfers: Transfer[]
  loading: boolean
  error: string | null
  refresh: () => void
  createTransfer: (req: TransferRequest) => Promise<TransferActionResult>
  createCrossBorderTransfer: (req: CrossBorderRequest) => Promise<TransferActionResult>
  fund: (accountId: string, req: FundRequest) => Promise<TransferActionResult>
} {
  const [transfers, setTransfers] = useState<Transfer[]>([])
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
      .listTransfers(traderId)
      .then(({ transfers }) => {
        if (!cancelled) setTransfers(transfers)
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
  }, [traderId, tick])

  const createTransfer = useCallback(
    async (req: TransferRequest): Promise<TransferActionResult> => {
      try {
        const resp = await api.createTransfer(req)
        setTick((n) => n + 1)
        return { ok: true, transfer: resp.transfer, replayed: resp.replayed }
      } catch (err) {
        return { ok: false, error: (err as Error).message }
      }
    },
    [],
  )

  const createCrossBorderTransfer = useCallback(
    async (req: CrossBorderRequest): Promise<TransferActionResult> => {
      try {
        const resp = await api.crossBorderTransfer(req)
        setTick((n) => n + 1)
        return { ok: true, transfer: resp.transfer, replayed: resp.replayed }
      } catch (err) {
        return { ok: false, error: (err as Error).message }
      }
    },
    [],
  )

  const fund = useCallback(
    async (accountId: string, req: FundRequest): Promise<TransferActionResult> => {
      try {
        const resp = await api.fundAccount(accountId, req)
        setTick((n) => n + 1)
        return { ok: true, transfer: resp.transfer }
      } catch (err) {
        return { ok: false, error: (err as Error).message }
      }
    },
    [],
  )

  return { transfers, loading, error, refresh, createTransfer, createCrossBorderTransfer, fund }
}