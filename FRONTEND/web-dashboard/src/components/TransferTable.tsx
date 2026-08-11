// TransferTable — enterprise data table with status badges, hover rows and
// empty state. Reused by Dashboard, Transfers, and History.
import type { Transfer } from '../api/types'
import { datetime, money } from '../lib/format'
import { StatusBadge } from './StatusBadge'
import { EmptyState } from './ui/primitives'

interface Props {
  transfers: Transfer[]
  onShowEntries?: (t: Transfer) => void
}

export function TransferTable({ transfers, onShowEntries }: Props) {
  if (transfers.length === 0) {
    return (
      <EmptyState
        icon="⇄"
        title="No transfers yet"
        description="Start a payment to see your invoice-linked history here."
      />
    )
  }

  return (
    <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-slate-200 bg-slate-50">
            <th className="table-th">Status</th>
            <th className="table-th">Amount</th>
            <th className="table-th">Invoice</th>
            <th className="table-th hidden md:table-cell">Reference</th>
            <th className="table-th hidden sm:table-cell">Date</th>
          </tr>
        </thead>
        <tbody>
          {transfers.map((t) => (
            <tr
              key={t.id}
              onClick={() => onShowEntries?.(t)}
              className={`border-b border-slate-100 transition-colors last:border-0 ${
                onShowEntries ? 'cursor-pointer hover:bg-navy-50/60' : ''
              }`}
            >
              <td className="table-td">
                <StatusBadge status={t.status} />
              </td>
              <td className="table-td font-mono font-semibold text-navy-900">
                {t.amount < 0 ? '−' : ''}
                {money(Math.abs(t.amount), t.currency)}
              </td>
              <td className="table-td">{t.invoice_number ?? '—'}</td>
              <td className="table-td hidden font-mono text-xs text-slate-500 md:table-cell">
                {t.external_reference ?? t.id.slice(0, 13)}
              </td>
              <td className="table-td hidden whitespace-nowrap text-slate-500 sm:table-cell">
                {datetime(t.created_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}