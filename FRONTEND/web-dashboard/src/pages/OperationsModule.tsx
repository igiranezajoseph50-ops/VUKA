// OperationsModule — real modules behind the secondary nav routes.
//
// Every value is derived from the live engine:
//   wallet        -> accounts + balances (useBalances)
//   rates         -> live RWF->KES engine FX (GET /api/fx)
//   analytics     -> status/volume KPIs computed from the transfer register
//   notifications -> the trader's real transfer events, newest first
// The fabricated modules (partners, trust-score, reports) were deleted in the
// tier-1 cleanup; their routes no longer exist.
import { useEffect, useMemo, useState } from 'react'
import { api } from '../api/client'
import { useTrader } from '../state/TraderContext'
import { useTransfers } from '../hooks/useTransfers'
import { useBalances, walletOf } from '../hooks/useBalances'
import { useLiveStatus } from '../hooks/useLiveStatus'

type ModuleKey = 'wallet' | 'rates' | 'analytics' | 'notifications'

const MODULE_META: Record<ModuleKey, { title: string; subtitle: string }> = {
  wallet: { title: 'Business wallet', subtitle: 'Segregated accounts and live balances from the ledger' },
  rates: { title: 'Exchange rates', subtitle: 'Live engine FX for the active RWF ↔ KES corridor' },
  analytics: { title: 'Analytics', subtitle: 'KPIs computed from the trader’s real transfer register' },
  notifications: { title: 'Notifications', subtitle: 'Real transfer events, newest first' },
}

function fmt(n: number, currency: string): string {
  return new Intl.NumberFormat('en', { style: 'currency', currency, maximumFractionDigits: 0 }).format(n)
}

function fmtTime(iso: string): string {
  const d = new Date(iso)
  const now = Date.now()
  const diff = Math.max(0, now - d.getTime())
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return d.toLocaleDateString('en', { month: 'short', day: 'numeric' })
}

function KpiCard({ title, value, note }: { title: string; value: string; note: string }) {
  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm text-slate-500">{title}</p>
      <p className="mt-4 font-mono text-3xl font-bold text-slate-950">{value}</p>
      <p className="mt-3 w-fit rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-bold text-emerald-700">{note}</p>
    </article>
  )
}

function Panel({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <header className="border-b border-slate-200 px-6 py-5">
        <h2 className="text-base font-bold text-slate-950">{title}</h2>
        <p className="mt-1 text-sm text-slate-500">{subtitle}</p>
      </header>
      <div className="p-6">{children}</div>
    </section>
  )
}

function WalletModule() {
  const { trader } = useTrader()
  const { wallets, loading, error, refresh } = useBalances(trader?.id ?? null)
  const { transfers } = useTransfers(trader?.id ?? null)

  const business = walletOf(wallets, 'BUSINESS')
  const personal = walletOf(wallets, 'PERSONAL')
  const settlement = walletOf(wallets, 'SETTLEMENT')
  const total = wallets.reduce((sum, w) => sum + w.balance, 0)

  const outbound = useMemo(
    () => transfers.filter((t) => t.status === 'SUCCESS' && t.source_account_id.startsWith(trader?.id ?? '__')).length,
    [transfers, trader],
  )
  const inbound = useMemo(
    () => transfers.filter((t) => t.status === 'SUCCESS' && !t.source_account_id.startsWith(trader?.id ?? '__')).length,
    [transfers, trader],
  )

  if (loading) return <p className="text-sm text-slate-500">Loading ledger…</p>
  if (error) return <p className="text-sm text-red-600">{error}</p>

  return (
    <div className="space-y-6">
      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Business balance" value={business ? fmt(business.balance, business.currency) : '—'} note={business ? business.currency : 'no account'} />
        <KpiCard title="Personal ledger" value={personal ? fmt(personal.balance, personal.currency) : '—'} note={personal ? 'separated' : 'no account'} />
        <KpiCard title="Settlement reserve" value={settlement ? fmt(settlement.balance, settlement.currency) : '—'} note={settlement ? 'engine-managed' : 'no account'} />
        <KpiCard title="Total across accounts" value={wallets.length ? fmt(total, wallets[0].currency) : '—'} note={`${wallets.length} account${wallets.length === 1 ? '' : 's'}`} />
      </section>

      <Panel title="Account register" subtitle="Live balances, one row per ledger account">
        {wallets.length === 0 ? (
          <p className="text-sm text-slate-500">No accounts yet — fund one from Payments.</p>
        ) : (
          <table className="min-w-full text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr><th className="px-4 py-3">Type</th><th className="px-4 py-3">Currency</th><th className="px-4 py-3">Account</th><th className="px-4 py-3 text-right">Balance</th></tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {wallets.map((w) => (
                <tr key={w.account.id}>
                  <td className="px-4 py-3 font-bold text-slate-950">{w.account.type}</td>
                  <td className="px-4 py-3 text-slate-600">{w.currency}</td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-500">{w.account.id.slice(0, 18)}…</td>
                  <td className="px-4 py-3 text-right font-mono font-bold text-slate-950">{fmt(w.balance, w.currency)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <div className="mt-4 flex items-center justify-between">
          <p className="text-xs text-slate-500">{inbound} inbound · {outbound} outbound successful transfers</p>
          <button onClick={refresh} className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">Refresh</button>
        </div>
      </Panel>
    </div>
  )
}

function RatesModule() {
  const [fx, setFx] = useState<{ pair: string; rate: number; updated: string; source: string } | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    api.getFxRate().then(setFx).catch(() => setError(true))
  }, [])

  return (
    <div className="space-y-6">
      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Active pair" value={fx ? fx.pair : error ? 'unavailable' : '…'} note="engine corridor" />
        <KpiCard title="Engine rate" value={fx ? fx.rate.toFixed(4) : error ? '—' : '…'} note={fx ? fx.source : error ? 'offline' : 'loading'} />
        <KpiCard title="Convert 1 RWF" value={fx ? (1 / fx.rate).toFixed(4) : '—'} note="KES" />
        <KpiCard title="Rate updated" value={fx ? fmtTime(fx.updated) : '—'} note={fx ? fx.updated : ''} />
      </section>

      <Panel title="Corridor pricing" subtitle="The single live corridor: Rwanda → Kenya, settled via the FX engine">
        {fx ? (
          <div className="space-y-3">
            {[
              ['Rwanda → Kenya', `${fx.rate.toFixed(4)} KES per RWF`, 'Live'],
            ].map(([route, metric, status]) => (
              <div key={route} className="grid grid-cols-[1fr_auto_auto] items-center gap-4 rounded-2xl bg-slate-50 p-4 text-sm">
                <span className="font-bold text-slate-950">{route}</span>
                <span className="font-mono font-bold text-slate-950">{metric}</span>
                <span className="rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700 ring-1 ring-emerald-200">{status}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-slate-500">{error ? 'FX endpoint unreachable — start the engine.' : 'Loading rate…'}</p>
        )}
      </Panel>
    </div>
  )
}

function AnalyticsModule() {
  const { trader } = useTrader()
  const { transfers, loading, error } = useTransfers(trader?.id ?? null)

  const stats = useMemo(() => {
    const success = transfers.filter((t) => t.status === 'SUCCESS')
    const failed = transfers.filter((t) => t.status === 'FAILED')
    const pending = transfers.filter((t) => t.status === 'PENDING' || t.status === 'PROCESSING')
    const reversed = transfers.filter((t) => t.status === 'REVERSED')
    const volume = success.reduce((sum, t) => sum + t.amount, 0)
    const crossBorder = success.filter((t) => t.fx_rate != null).length
    const rate = transfers.length ? Math.round((success.length / transfers.length) * 1000) / 10 : 0
    return { success: success.length, failed: failed.length, pending: pending.length, reversed: reversed.length, volume, crossBorder, rate, total: transfers.length }
  }, [transfers])

  if (loading) return <p className="text-sm text-slate-500">Loading register…</p>
  if (error) return <p className="text-sm text-red-600">{error}</p>

  return (
    <div className="space-y-6">
      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Successful transfers" value={stats.success.toLocaleString()} note={`${stats.rate}% success rate`} />
        <KpiCard title="Settled volume" value={fmt(stats.volume, 'RWF')} note={`${stats.crossBorder} cross-border`} />
        <KpiCard title="Pending / processing" value={stats.pending.toLocaleString()} note="in flight" />
        <KpiCard title="Failed + reversed" value={(stats.failed + stats.reversed).toLocaleString()} note="need review" />
      </section>

      <div className="grid gap-6 xl:grid-cols-[1.5fr_1fr]">
        <Panel title="Status distribution" subtitle="Count of transfers per ledger status">
          <table className="min-w-full text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr><th className="px-4 py-3">Status</th><th className="px-4 py-3 text-right">Count</th><th className="px-4 py-3 text-right">Share</th></tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {[
                ['SUCCESS', stats.success, 'bg-emerald-50 text-emerald-700'],
                ['FAILED', stats.failed, 'bg-red-50 text-red-600'],
                ['PENDING', stats.pending, 'bg-amber-50 text-amber-700'],
                ['REVERSED', stats.reversed, 'bg-slate-100 text-slate-600'],
              ].map(([label, count, chip]) => (
                <tr key={label as string}>
                  <td className="px-4 py-3 font-bold text-slate-950">{label}</td>
                  <td className="px-4 py-3 text-right font-mono font-bold">{String(count)}</td>
                  <td className="px-4 py-3 text-right">
                    <span className={`rounded-full px-3 py-1 text-xs font-bold ${chip}`}>
                      {stats.total ? Math.round((Number(count) / stats.total) * 1000) / 10 : 0}%
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="mt-4 text-xs text-slate-500">All numbers computed from the live register ({stats.total} transfers).</p>
        </Panel>

        <Panel title="Register health" subtitle="Engine-derived summary">
          <div className="space-y-4">
            {[
              ['Register size', `${stats.total} transfers`],
              ['Cross-border share', `${stats.total ? Math.round((stats.crossBorder / stats.total) * 100) : 0}% of successful`],
              ['FX-marked', `${stats.crossBorder} with fx_rate`],
              ['Needs review', `${stats.failed + stats.reversed}`],
            ].map(([label, value]) => (
              <div key={label} className="flex items-center justify-between rounded-2xl bg-slate-50 p-4 text-sm">
                <span className="font-semibold text-slate-950">{label}</span>
                <span className="font-mono font-bold text-emerald-700">{value}</span>
              </div>
            ))}
          </div>
        </Panel>
      </div>
    </div>
  )
}

function NotificationsModule() {
  const { trader } = useTrader()
  const { transfers, loading, error, refresh } = useTransfers(trader?.id ?? null)
  const { connected } = useLiveStatus()

  const events = useMemo(
    () =>
      [...transfers]
        .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
        .slice(0, 20),
    [transfers],
  )

  const unread = transfers.filter((t) => t.status === 'FAILED' || t.status === 'REVERSED').length

  if (loading) return <p className="text-sm text-slate-500">Loading events…</p>
  if (error) return <p className="text-sm text-red-600">{error}</p>

  return (
    <div className="space-y-6">
      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard title="Live feed" value={connected ? 'Connected' : 'Reconnecting'} note={connected ? 'SSE active' : 'engine offline'} />
        <KpiCard title="Events in register" value={events.length.toLocaleString()} note="newest 20 shown" />
        <KpiCard title="Failed + reversed" value={unread.toLocaleString()} note="need review" />
        <KpiCard title="Last event" value={events[0] ? fmtTime(events[0].created_at) : '—'} note="engine time" />
      </section>

      <Panel title="Transfer events" subtitle="Real ledger events, newest first">
        {events.length === 0 ? (
          <p className="text-sm text-slate-500">No transfers yet — make one from Payments.</p>
        ) : (
          <div className="space-y-3">
            {events.map((t) => (
              <div key={t.id} className="flex items-center justify-between gap-4 rounded-2xl bg-slate-50 p-4">
                <div className="min-w-0">
                  <p className="truncate font-bold text-slate-950">
                    {t.invoice_number ?? t.id.slice(0, 12)} <span className="font-normal text-slate-500">· {t.status}</span>
                  </p>
                  <p className="mt-1 text-xs text-slate-500">{fmtTime(t.created_at)} · {t.fx_rate != null ? 'cross-border' : 'local'}</p>
                </div>
                <div className="text-right">
                  <p className="font-mono font-bold text-slate-950">{fmt(t.amount, t.currency)}</p>
                  <p className={`mt-1 text-xs font-bold ${t.status === 'SUCCESS' ? 'text-emerald-600' : t.status === 'FAILED' || t.status === 'REVERSED' ? 'text-red-500' : 'text-amber-600'}`}>{t.status}</p>
                </div>
              </div>
            ))}
          </div>
        )}
        <div className="mt-4 flex justify-end">
          <button onClick={refresh} className="rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold text-slate-600 hover:bg-slate-50">Refresh</button>
        </div>
      </Panel>
    </div>
  )
}

export default function OperationsModule({ module }: { module: ModuleKey }) {
  const meta = MODULE_META[module]
  return (
    <div className="mx-auto max-w-[1560px] space-y-6">
      <section>
        <h1 className="text-2xl font-bold tracking-tight text-slate-950">{meta.title}</h1>
        <p className="mt-2 text-sm text-slate-500">{meta.subtitle}</p>
      </section>
      {module === 'wallet' && <WalletModule />}
      {module === 'rates' && <RatesModule />}
      {module === 'analytics' && <AnalyticsModule />}
      {module === 'notifications' && <NotificationsModule />}
    </div>
  )
}
