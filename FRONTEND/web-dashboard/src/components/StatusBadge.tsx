// StatusBadge — maps a transfer status to a colored pill (enterprise tones).
import type { TransferStatus } from '../api/types'

const STYLES: Record<TransferStatus, string> = {
  PENDING: 'border-amber-200 bg-amber-50 text-amber-700',
  PROCESSING: 'border-blue-200 bg-blue-50 text-blue-700',
  SUCCESS: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  FAILED: 'border-red-200 bg-red-50 text-red-700',
  REVERSED: 'border-slate-200 bg-slate-100 text-slate-600',
}

export function StatusBadge({ status }: { status: TransferStatus }) {
  return (
    <span className={`badge ${STYLES[status] ?? STYLES.PENDING}`}>
      {status}
    </span>
  )
}