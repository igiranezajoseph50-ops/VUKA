// TraderContext — demo-mode trader selection (D6).
//
// For the hackathon MVP there is no auth; a trader is picked on the landing
// screen and held in this context (persisted to sessionStorage). The selected
// trader id drives every page's data fetching.
import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import type { User } from '../api/types'

const STORAGE_KEY = 'vuka.trader'

interface TraderContextValue {
  trader: User | null
  select: (user: User) => void
  clear: () => void
}

const TraderContext = createContext<TraderContextValue | null>(null)

export function TraderProvider({ children }: { children: ReactNode }) {
  const [trader, setTrader] = useState<User | null>(() => {
    try {
      const raw = sessionStorage.getItem(STORAGE_KEY)
      return raw ? (JSON.parse(raw) as User) : null
    } catch {
      return null
    }
  })

  useEffect(() => {
    if (trader) {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(trader))
    } else {
      sessionStorage.removeItem(STORAGE_KEY)
    }
  }, [trader])

  const value = useMemo<TraderContextValue>(
    () => ({
      trader,
      select: setTrader,
      clear: () => setTrader(null),
    }),
    [trader],
  )

  return <TraderContext.Provider value={value}>{children}</TraderContext.Provider>
}

export function useTrader(): TraderContextValue {
  const ctx = useContext(TraderContext)
  if (!ctx) throw new Error('useTrader must be used within a TraderProvider')
  return ctx
}