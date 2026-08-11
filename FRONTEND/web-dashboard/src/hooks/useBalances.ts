// useBalances(traderId): fetch the trader's accounts and derived balances.
import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Account } from '../api/types'

export interface Wallet {
  account: Account
  balance: number
  currency: string
}

export function useBalances(traderId: string | null): {
  wallets: Wallet[]
  loading: boolean
  error: string | null
  refresh: () => void
} {
  const [wallets, setWallets] = useState<Wallet[]>([])
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
      .getAccounts(traderId)
      .then(async ({ accounts }) => {
        const withBalances = await Promise.all(
          accounts.map(async (account) => {
            const bal = await api.getBalance(account.id)
            return { account, balance: bal.amount, currency: account.currency }
          }),
        )
        if (!cancelled) setWallets(withBalances)
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

  return { wallets, loading, error, refresh }
}

/** Convenience: find a wallet by account type. */
export function walletOf(wallets: Wallet[], type: string): Wallet | undefined {
  return wallets.find((w) => w.account.type === type)
}