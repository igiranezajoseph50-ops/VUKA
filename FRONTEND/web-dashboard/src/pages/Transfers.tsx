// Transfers — new transfer form + full history with double-entry detail.
import { useEffect, useState } from 'react'
import { useTrader } from '../state/TraderContext'
import { useBalances } from '../hooks/useBalances'
import { useTransfers } from '../hooks/useTransfers'
import { TransferForm } from '../components/TransferForm'
import { TransferTable } from '../components/TransferTable'
import { Card, SkeletonTable, ErrorBanner } from '../components/ui/primitives'
import { api } from '../api/client'
import { amount, datetime, moneyCompact } from '../lib/format'
import type { LedgerEntry, Transfer } from '../api/types'

interface FxState {
  rate: number
  updated: string
  error: string | null
}

export default function Transfers() {
  const { trader } = useTrader()
  const { wallets, loading: wLoading, error: wError, refresh } = useBalances(trader?.id ?? null)
  const { transfers, loading, error, refresh: refreshTransfers, createTransfer, createCrossBorderTransfer } =
    useTransfers(trader?.id ?? null)
  const [detail, setDetail] = useState<Transfer | null>(null)
  const [entries, setEntries] = useState<LedgerEntry[] | null>(null)
  const [fx, setFx] = useState<FxState>({ rate: 0, updated: '', error: null })

  useEffect(() => {
    let cancelled = false
    api
      .getFxRate()
      .then((res) => {
        if (!cancelled) setFx({ rate: res.rate, updated: res.updated, error: null })
      })
      .catch((err: Error) => {
        if (!cancelled) setFx((p) => ({ ...p, error: err.message }))
      })
    return () => {
      cancelled = true
    }
  }, [])

  // Real metrics derived from the ledger register.
  const settledToday = transfers
    .filter((t) => t.status === 'SUCCESS' && new Date(t.created_at).toDateString() === new Date().toDateString())
    .reduce((sum, t) => sum + t.amount, 0)
  const processing = transfers.filter((t) => t.status === 'PROCESSING').length
  const pending = transfers.filter((t) => t.status === 'PENDING').length
  const attempts = transfers.length
  const successes = transfers.filter((t) => t.status === 'SUCCESS').length
  const successRate = attempts > 0 ? Math.round((successes / attempts) * 1000) / 10 : 0
  const crossBorderCount = transfers.filter((t) => t.fx_rate && t.fx_rate > 0).length

  async function showEntries(t: Transfer) {
    if (detail?.id === t.id) {
      setDetail(null)
      setEntries(null)
      return
    }
    setDetail(t)
    setEntries(null)
    try {
      const res = await api.getTransferEntries(t.id)
      setEntries(res.entries)
    } catch {
      setEntries([])
    }
  }

  return (
    <div className="mx-auto max-w-[1560px] animate-rise space-y-6">
      <section className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-950">Payments</h1>
          <p className="mt-2 text-sm text-slate-500">
            Invoice-linked local and cross-border settlements with double-entry audit rows.
          </p>
        </div>
      </section>

      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <Metric title="Settled today" value={moneyCompact(settledToday, 'RWF')} note={`${successes} successful`} />
        <Metric title="Processing now" value={String(processing)} note="in-flight" />
        <Metric title="Pending queue" value={String(pending)} note={pending === 0 ? 'empty' : 'awaiting rail'} />
        <Metric title="Success rate" value={`${successRate}%`} note={`${attempts} attempts`} />
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
        <div>
          {wLoading ? (
            <SkeletonTable rows={3} />
          ) : wError ? (
            <ErrorBanner message={wError} onRetry={refresh} />
          ) : (
            <TransferForm
              wallets={wallets}
              fxRate={fx.rate > 0 ? fx.rate : null}
              onSubmit={createTransfer}
              onCrossBorderSubmit={createCrossBorderTransfer}
            />
          )}
        </div>
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="text-base font-bold text-slate-950">Corridor status</h2>
          <p className="mt-1 text-sm text-slate-500">Live rail pricing from the settlement engine.</p>
          <div className="mt-6 space-y-4">
            <CorridorRow
              route="Rwanda → Kenya"
              rail="MTN MoMo → M-Pesa"
              fxRate={fx.rate}
              updated={fx.updated}
              count={crossBorderCount}
              fxError={fx.error}
            />
            <CorridorRow route="Rwanda (local)" rail="MTN MoMo" fxRate={0} updated="" count={transfers.length - crossBorderCount} fxError={null} />
          </div>
        </div>
      </section>

      {detail && (
        <Card title={`Double-entry audit · ${detail.id.slice(0, 13)}…`} subtitle="Ledger entries must net to zero">
          {entries === null ? (
            <SkeletonTable rows={2} />
          ) : (
            <div className="grid gap-2 sm:grid-cols-2">
              {entries.map((e) => (
                <div
                  key={e.id}
                  className="flex items-center justify-between rounded-lg border border-slate-100 bg-slate-50 px-3 py-2 font-mono text-sm"
                >
                  <span className="truncate text-slate-500">{e.account_id.slice(0, 13)}…</span>
                  <span className={e.amount < 0 ? 'font-semibold text-red-600' : 'font-semibold text-emerald-700'}>
                    {e.amount < 0 ? '−' : '+'}
                    {amount(Math.abs(e.amount))}
                  </span>
                </div>
              ))}
            </div>
          )}
        </Card>
      )}

      <Card title="Payment operations register" subtitle="Tap a row for immutable double-entry rows">
        {loading ? (
          <SkeletonTable rows={5} />
        ) : error ? (
          <ErrorBanner message={error} onRetry={refreshTransfers} />
        ) : (
          <TransferTable transfers={transfers} onShowEntries={showEntries} />
        )}
      </Card>
    </div>
  )
}

function Metric({ title, value, note }: { title: string; value: string; note: string }) {
  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm text-slate-500">{title}</p>
      <p className="mt-4 font-mono text-3xl font-bold text-slate-950">{value}</p>
      <p className="mt-3 w-fit rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-bold text-emerald-700">{note}</p>
    </article>
  )
}

function CorridorRow({
  route,
  rail,
  fxRate,
  updated,
  count,
  fxError,
}: {
  route: string
  rail: string
  fxRate: number
  updated: string
  count: number
  fxError: string | null
}) {
  return (
    <div className="rounded-2xl bg-slate-50 p-4">
      <div className="flex justify-between gap-3">
        <div>
          <p className="font-bold text-slate-950">{route}</p>
          <p className="mt-1 text-sm text-slate-500">{rail}</p>
        </div>
        {route.includes('Kenya') ? (
          fxError ? (
            <span className="h-fit rounded-full bg-amber-50 px-3 py-1 text-xs font-bold text-amber-700">rate offline</span>
          ) : fxRate > 0 ? (
            <span className="h-fit rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700">
              1 KES = {amount(fxRate)} RWF
            </span>
          ) : (
            <span className="h-fit rounded-full bg-slate-100 px-3 py-1 text-xs font-bold text-slate-500">loading…</span>
          )
        ) : (
          <span className="h-fit rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700">
            {count} settled
          </span>
        )}
      </div>
      {updated && fxRate > 0 && (
        <p className="mt-2 font-mono text-xs text-slate-400">as of {datetime(updated)}</p>
      )}
    </div>
  )
}