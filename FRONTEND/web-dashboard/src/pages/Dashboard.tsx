// Dashboard — operations overview computed entirely from the live engine.
//
// Every number here is derived from real API data: wallet balances, the
// transfer register, invoice records and the corridor FX rate. No fabricated
// fallbacks, no invented counterparties, no UG/TZ corridors.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useBalances, walletOf } from '../hooks/useBalances'
import { useLiveStatus } from '../hooks/useLiveStatus'
import { useTransfers } from '../hooks/useTransfers'
import { useInvoices } from '../hooks/useInvoices'
import { useToasts } from '../components/ui/ToastProvider'
import { useTrader } from '../state/TraderContext'
import { api } from '../api/client'
import { amount, datetime, moneyCompact } from '../lib/format'
import type { Transfer, TransferEvent, TransferStatus } from '../api/types'

export default function Dashboard() {
  const { trader } = useTrader()
  const { wallets, refresh } = useBalances(trader?.id ?? null)
  const { transfers, refresh: refreshTransfers } = useTransfers(trader?.id ?? null)
  const { invoices, refresh: refreshInvoices } = useInvoices(trader?.id ?? null)
  const toasts = useToasts()
  const [fx, setFx] = useState<{ rate: number; updated: string } | null>(null)
  const [activity, setActivity] = useState<TransferEvent | null>(null)

  const handleEvent = useCallback(
    (ev: TransferEvent) => {
      setActivity(ev)
      refresh()
      refreshTransfers()
      refreshInvoices()
      toasts.info('Ledger updated', `${ev.status} · ${ev.id.slice(0, 8)}`)
    },
    [refresh, refreshTransfers, refreshInvoices, toasts],
  )

  useLiveStatus(handleEvent)

  useEffect(() => {
    let cancelled = false
    api
      .getFxRate()
      .then((res) => {
        if (!cancelled) setFx({ rate: res.rate, updated: res.updated })
      })
      .catch(() => {
        /* corridor rate offline — cards simply omit it */
      })
    return () => {
      cancelled = true
    }
  }, [])

  const business = walletOf(wallets, 'BUSINESS')
  const personal = walletOf(wallets, 'PERSONAL')

  const stats = useMemo(() => {
    const today = new Date().toDateString()
    const successful = transfers.filter((t) => t.status === 'SUCCESS')
    const settledToday = successful
      .filter((t) => new Date(t.created_at).toDateString() === today)
      .reduce((sum, t) => sum + t.amount, 0)
    const processing = transfers.filter((t) => t.status === 'PROCESSING')
    const pending = transfers.filter((t) => t.status === 'PENDING')
    const failed = transfers.filter((t) => t.status === 'FAILED')
    const crossBorder = successful.filter((t) => t.fx_rate && t.fx_rate > 0)
    const volume = successful.reduce((sum, t) => sum + Math.abs(t.amount), 0)
    const successRate = transfers.length ? Math.round((successful.length / transfers.length) * 1000) / 10 : 0

    // Real per-day settled volume for the trailing-7-day sparkline.
    const daily: number[] = []
    for (let i = 6; i >= 0; i--) {
      const day = new Date(Date.now() - i * 86_400_000).toDateString()
      daily.push(successful.filter((t) => new Date(t.created_at).toDateString() === day).reduce((s, t) => s + t.amount, 0))
    }

    return {
      businessBalance: business?.balance ?? 0,
      personalBalance: personal?.balance ?? 0,
      settledToday,
      volume,
      paidCount: successful.length,
      processingCount: processing.length,
      pendingCount: pending.length,
      failedCount: failed.length,
      crossBorderCount: crossBorder.length,
      successRate,
      daily,
      lastSettledAt: successful[0]?.updated_at ?? null,
    }
  }, [business, personal, transfers])

  const invoiceStats = useMemo(() => {
    const paid = invoices.filter((i) => i.status === 'PAID').length
    const outstanding = invoices.length - paid
    const due = invoices
      .filter((i) => i.status === 'ISSUED' && new Date(i.due_date).getTime() < Date.now())
      .map((i) => ({ number: i.number, due_date: i.due_date, currency: i.currency }))
    return { total: invoices.length, paid, outstanding, due }
  }, [invoices])

  if (!trader) return null

  return (
    <div className="mx-auto max-w-[1560px] space-y-6">
      <section className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-950">Operations overview</h1>
          <p className="mt-2 text-sm text-slate-500">
            {trader.full_name} · Rwanda ↔ Kenya corridor · {new Date().toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}
            {activity ? ` · Last event ${activity.status}` : ''}
          </p>
        </div>
      </section>

      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Business Balance" value={moneyCompact(stats.businessBalance, business?.currency)} note="Segregated business ledger" />
        <KpiCard title="Settled Today" value={moneyCompact(stats.settledToday, 'RWF')} note={`${stats.paidCount} successful total`} />
        <KpiCard title="Invoices Paid" value={invoiceStats.paid.toLocaleString()} note={`${invoiceStats.outstanding} outstanding`} />
        <KpiCard title="Success Rate" value={`${stats.successRate}%`} note={`${stats.failedCount} failed`} negative={stats.failedCount > 0} />
      </section>

      <section className="grid gap-6 xl:grid-cols-[2fr_1fr]">
        <Panel title="Live transaction feed" subtitle="Real transfers from the engine register" action={<LivePill label="Realtime" />}>
          <div className="divide-y divide-slate-200">
            {transfers.length === 0 ? (
              <p className="px-6 py-8 text-center text-sm text-slate-500">No transfers yet — send your first payment from the Payments page.</p>
            ) : (
              transfers.slice(0, 8).map((t) => <TransactionRow key={t.id} transfer={t} />)
            )}
          </div>
        </Panel>
        <Panel title="Corridor status" subtitle="The only live corridor is Rwanda ↔ Kenya">
          <div className="space-y-4 p-6">
            <CorridorCard
              route="Rwanda → Kenya"
              rail="MTN MoMo → M-Pesa"
              count={`${stats.crossBorderCount} settled`}
              fxRate={fx?.rate ?? null}
              updated={fx?.updated ?? null}
            />
            <CorridorCard
              route="Rwanda (domestic)"
              rail="MTN MoMo"
              count={`${stats.paidCount - stats.crossBorderCount} settled`}
              fxRate={null}
              updated={null}
            />
            <p className="rounded-xl bg-slate-50 p-4 text-xs leading-relaxed text-slate-500">
              Uganda and Tanzania corridors are not part of the Rwanda–Kenya pilot and are excluded from this dashboard.
            </p>
          </div>
        </Panel>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1fr_1fr_1fr]">
        <Panel title="Wallets" subtitle="Segregated business & personal ledgers">
          <div className="space-y-4 p-6">
            <div className="rounded-2xl bg-navy-950 p-6 text-white shadow-lg">
              <p className="text-sm text-slate-300">Business Wallet · {business?.currency ?? 'RWF'}</p>
              <p className="mt-4 font-mono text-3xl font-bold">{amount(stats.businessBalance)}</p>
              <p className="mt-3 text-xs text-slate-400">{business?.account.id ? `account ${business.account.id.slice(0, 13)}…` : 'no business account'}</p>
            </div>
            <div className="rounded-2xl border border-slate-200 p-5">
              <p className="text-sm text-slate-500">Personal Wallet · {personal?.currency ?? 'RWF'}</p>
              <p className="mt-3 font-mono text-2xl font-bold text-slate-950">{amount(stats.personalBalance)}</p>
              <p className="mt-3 text-xs text-slate-400">{personal?.account.id ? `account ${personal.account.id.slice(0, 13)}…` : 'no personal account'}</p>
            </div>
          </div>
        </Panel>
        <Panel title="Settlement volume" subtitle="Real settled RWF per day · trailing 7 days">
          <div className="p-6">
            {stats.daily.some((v) => v > 0) ? (
              <VolumeBars values={stats.daily} />
            ) : (
              <p className="py-10 text-center text-sm text-slate-500">No settled volume in the last 7 days.</p>
            )}
          </div>
        </Panel>
        <Panel title="Payment status" subtitle="Real register totals">
          <StatusDonut
            completed={stats.paidCount}
            processing={stats.processingCount}
            pending={stats.pendingCount}
            failed={stats.failedCount}
          />
        </Panel>
      </section>

      <Panel title="Invoices" subtitle={`${invoiceStats.total} real invoice records`}>
        {invoices.length === 0 ? (
          <p className="px-6 py-8 text-center text-sm text-slate-500">No invoices issued or received yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-left">
              <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  {['Invoice', 'Status', 'Currency', 'Due date', 'Items', 'Issued'].map((head) => (
                    <th key={head} className="px-6 py-4 font-bold">{head}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200">
                {invoices.slice(0, 6).map((inv) => (
                  <tr key={inv.id} className="text-sm">
                    <td className="px-6 py-4 font-bold text-slate-950">{inv.number}</td>
                    <td className="px-6 py-4"><StatusPill status={inv.status === 'PAID' ? 'Paid' : 'Pending'} /></td>
                    <td className="px-6 py-4 font-bold text-slate-950">{inv.currency}</td>
                    <td className="px-6 py-4 text-slate-600">{datetime(inv.due_date)}</td>
                    <td className="px-6 py-4 text-slate-600">{inv.items.length}</td>
                    <td className="px-6 py-4 text-slate-600">{datetime(inv.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {invoiceStats.due.length > 0 && (
          <p className="border-t border-slate-200 bg-amber-50 px-6 py-4 text-sm text-amber-800">
            {invoiceStats.due.length} invoice{invoiceStats.due.length > 1 ? 's' : ''} past due: {invoiceStats.due.map((d) => `${d.number} (${d.currency})`).join(', ')}
          </p>
        )}
      </Panel>

      <footer className="flex flex-wrap justify-between gap-3 border-t border-slate-200 py-6 text-sm text-slate-500">
        <span>VUKA Enterprise · Rwanda ↔ Kenya corridor</span>
        <span>{stats.lastSettledAt ? `Ledger last settled ${datetime(stats.lastSettledAt)}` : 'Ledger: no settled transfers yet'}</span>
      </footer>
    </div>
  )
}

function Panel({ title, subtitle, action, children }: { title: string; subtitle: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-6 py-5">
        <div>
          <h2 className="text-base font-bold text-slate-950">{title}</h2>
          <p className="mt-1 text-sm text-slate-500">{subtitle}</p>
        </div>
        {action}
      </header>
      {children}
    </section>
  )
}

function KpiCard({ title, value, note, negative }: { title: string; value: string; note: string; negative?: boolean }) {
  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm text-slate-500">{title}</p>
      <p className="mt-4 font-mono text-3xl font-bold tracking-tight text-slate-950">{value}</p>
      <p className={`mt-3 w-fit rounded-full px-2.5 py-1 text-xs font-bold ${negative ? 'bg-red-50 text-red-600' : 'bg-emerald-50 text-emerald-600'}`}>
        {note}
      </p>
    </article>
  )
}

function LivePill({ label = 'Live' }: { label?: string }) {
  return <span className="inline-flex items-center gap-1.5 text-xs text-slate-500"><span className="h-2 w-2 rounded-full bg-emerald-500" />{label}</span>
}

function TransactionRow({ transfer }: { transfer: Transfer }) {
  const corridor = transfer.fx_rate && transfer.fx_rate > 0 ? 'RWF → KES' : 'RWF domestic'
  const rail = corridor === 'RWF → KES' ? 'MTN MoMo → M-Pesa' : 'MTN MoMo'
  return (
    <div className="grid gap-3 px-6 py-4 text-sm md:grid-cols-[1.5fr_1fr_1fr_auto] md:items-center">
      <div className="min-w-0">
        <p className="truncate font-bold text-slate-950">
          {transfer.source_account_id.slice(0, 13)}… <span className="px-2 text-slate-400">{'->'}</span> {transfer.destination_account_id.slice(0, 13)}…
        </p>
        <p className="mt-1 text-xs text-slate-500">{transfer.invoice_number ? `${transfer.invoice_number} · ` : ''}{corridor} · {rail}</p>
      </div>
      <div>
        <p className="font-bold text-slate-950">{amount(transfer.amount)}</p>
        <p className="mt-1 text-xs text-slate-500">{transfer.currency}{transfer.fx_rate ? ` · FX ${transfer.fx_rate}` : ''}</p>
      </div>
      <div><p className="text-xs text-slate-500">{datetime(transfer.created_at)}</p></div>
      <StatusPill status={readableStatus(transfer.status)} />
    </div>
  )
}

function readableStatus(status: TransferStatus) {
  if (status === 'SUCCESS') return 'Completed'
  if (status === 'PROCESSING') return 'Processing'
  if (status === 'FAILED') return 'Failed'
  if (status === 'REVERSED') return 'Reversed'
  return 'Pending'
}

function StatusPill({ status }: { status: string }) {
  const cls = status === 'Paid' || status === 'Completed' ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : status === 'Processing' || status === 'Partial' ? 'bg-blue-50 text-blue-700 ring-blue-200' : status === 'Failed' || status === 'Overdue' ? 'bg-red-50 text-red-700 ring-red-200' : 'bg-amber-50 text-amber-700 ring-amber-200'
  return <span className={`inline-flex w-fit rounded-full px-3 py-1 text-xs font-bold ring-1 ${cls}`}>{status}</span>
}

function CorridorCard({ route, rail, count, fxRate, updated }: { route: string; rail: string; count: string; fxRate: number | null; updated: string | null }) {
  return (
    <div className="rounded-2xl bg-slate-50 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-bold text-slate-950">{route}</p>
          <p className="mt-1 text-sm text-slate-500">{rail}</p>
        </div>
        {fxRate ? (
          <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700">1 KES = {amount(fxRate)} RWF</span>
        ) : (
          <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700">{count}</span>
        )}
      </div>
      {fxRate && updated && <p className="mt-2 font-mono text-xs text-slate-400">as of {datetime(updated)}</p>}
    </div>
  )
}

function VolumeBars({ values }: { values: number[] }) {
  const max = Math.max(...values, 1)
  const labels = ['−6', '−5', '−4', '−3', '−2', '−1', 'today']
  return (
    <div className="flex h-56 items-end gap-3">
      {values.map((v, i) => (
        <div key={labels[i]} className="flex flex-1 flex-col items-center gap-3">
          <div className="w-full rounded-t-md bg-emerald-600" style={{ height: `${(v / max) * 160}px`, minHeight: v > 0 ? 4 : 2 }} />
          <span className="text-xs text-slate-500">{labels[i]}</span>
        </div>
      ))}
    </div>
  )
}

function StatusDonut({ completed, processing, pending, failed }: { completed: number; processing: number; pending: number; failed: number }) {
  const total = completed + processing + pending + failed
  if (total === 0) {
    return <p className="px-6 py-10 text-center text-sm text-slate-500">No transfers recorded yet.</p>
  }
  const pct = (n: number) => (n / total) * 100
  const rows: Array<[string, number, string]> = [
    ['Completed', completed, 'bg-emerald-500'],
    ['Processing', processing, 'bg-blue-500'],
    ['Pending', pending, 'bg-amber-500'],
    ['Failed', failed, 'bg-red-500'],
  ]
  return (
    <div className="grid gap-6 p-6 sm:grid-cols-[180px_1fr] sm:items-center">
      <div
        className="h-44 w-44 rounded-full"
        style={{
          background: `conic-gradient(#16a34a 0 ${pct(completed)}%, #3b82f6 ${pct(completed)}% ${pct(completed) + pct(processing)}%, #f59e0b ${pct(completed) + pct(processing)}% ${pct(completed) + pct(processing) + pct(pending)}%, #ef4444 ${pct(completed) + pct(processing) + pct(pending)}% 100%)`,
        }}
      >
        <div className="m-auto h-24 w-24 translate-y-10 rounded-full bg-white" />
      </div>
      <div className="space-y-4 text-sm">
        {rows.map(([label, value, color]) => (
          <div key={label} className="flex justify-between">
            <span className="flex items-center gap-2 text-slate-600"><i className={`h-2 w-2 rounded-full ${color}`} />{label}</span>
            <b>{value.toLocaleString()}</b>
          </div>
        ))}
        <div className="rounded-xl bg-slate-50 p-4 text-xs text-slate-600">
          {total} transfers in the register · statuses are live engine values.
        </div>
      </div>
    </div>
  )
}