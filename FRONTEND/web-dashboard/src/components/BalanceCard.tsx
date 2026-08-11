// BalanceCard — one wallet (PERSONAL / BUSINESS) with derived balance.
import type { Account } from '../api/types'
import { money } from '../lib/format'

interface Props {
  account: Account
  balance: number
  accent?: 'navy' | 'emerald'
}

export function BalanceCard({ account, balance, accent = 'navy' }: Props) {
  const positive = balance >= 0
  const accentClass =
    accent === 'emerald'
      ? 'border-emerald-500/30 bg-emerald-50/50'
      : 'border-navy-900/20 bg-white'

  return (
    <div className={`rounded-xl border p-5 shadow-sm ${accentClass}`}>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium uppercase tracking-wide text-slate-500">
          {account.type.toLowerCase()} wallet
        </span>
        <span className="rounded bg-navy-900 px-2 py-0.5 font-mono text-xs font-semibold text-white">
          {account.currency}
        </span>
      </div>
      <div className={`mt-3 text-2xl font-bold ${positive ? 'text-navy-900' : 'text-red-600'}`}>
        {money(balance, account.currency)}
      </div>
      <div className="mt-1 font-mono text-xs text-slate-400">{account.id.slice(0, 13)}…</div>
    </div>
  )
}