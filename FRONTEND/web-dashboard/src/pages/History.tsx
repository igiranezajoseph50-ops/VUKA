// History — full audit trail with status filters, sorting, and double-entry
// drilldown. Enterprise data-view patterns: filter chips, sortable columns,
// and pagination.
import { useMemo, useState } from 'react'
import { useTrader } from '../state/TraderContext'
import { useTransfers } from '../hooks/useTransfers'
import { Card, SkeletonTable, ErrorBanner, EmptyState } from '../components/ui/primitives'
import { api } from '../api/client'
import { amount } from '../lib/format'
import { InvoiceDocument, invoiceFromTransfer } from '../components/InvoiceForm'
import type { LedgerEntry, Transfer } from '../api/types'

const FILTERS = ['ALL', 'SUCCESS', 'PENDING', 'PROCESSING', 'FAILED', 'REVERSED'] as const
const PAGE_SIZE = 10

export default function History() {
  const { trader } = useTrader()
  const { transfers, loading, error, refresh } = useTransfers(trader?.id ?? null)
  const [status, setStatus] = useState<(typeof FILTERS)[number]>('ALL')
  const [page, setPage] = useState(0)
  const [sort, setSort] = useState<'date' | 'amount'>('date')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [expanded, setExpanded] = useState<Record<string, LedgerEntry[]>>({})
  const [openId, setOpenId] = useState<string | null>(null)
  const [detailTab, setDetailTab] = useState<'invoice' | 'entries'>('invoice')

  const filtered = useMemo(() => {
    let rows = status === 'ALL' ? transfers : transfers.filter((t) => t.status === status)
    rows = [...rows].sort((a, b) => {
      if (sort === 'amount') return sortDir === 'asc' ? a.amount - b.amount : b.amount - a.amount
      return sortDir === 'asc'
        ? new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
        : new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    })
    return rows
  }, [transfers, status, sort, sortDir])

  const pageRows = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)
  const pages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))

  // Real audit metrics computed from the loaded register.
  const audit = useMemo(() => {
    const successful = transfers.filter((t) => t.status === 'SUCCESS')
    const successRate = transfers.length ? (successful.length / transfers.length) * 100 : 0
    const failed = transfers.filter((t) => t.status === 'FAILED' || t.status === 'REVERSED')
    const finalityMs = successful
      .map((t) => new Date(t.updated_at).getTime() - new Date(t.created_at).getTime())
      .filter((ms) => ms >= 0)
    const avgFinality = finalityMs.length
      ? Math.round(finalityMs.reduce((a, b) => a + b, 0) / finalityMs.length / 1000)
      : null
    return {
      successRate,
      failedCount: failed.length,
      avgFinality,
      sampled: finalityMs.length,
    }
  }, [transfers])

  function toggleSort(which: 'date' | 'amount') {
    if (sort === which) setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    else {
      setSort(which)
      setSortDir('desc')
    }
  }

  async function toggleEntries(t: Transfer) {
    if (openId === t.id) {
      setOpenId(null)
      return
    }
    setOpenId(t.id)
    setDetailTab(t.invoice_number ? 'invoice' : 'entries')
    if (!expanded[t.id]) {
      try {
        const res = await api.getTransferEntries(t.id)
        setExpanded((prev) => ({ ...prev, [t.id]: res.entries }))
      } catch {
        setExpanded((prev) => ({ ...prev, [t.id]: [] }))
      }
    }
  }

  function SortHeader({ label, which }: { label: string; which: 'date' | 'amount' }) {
    return (
      <button
        onClick={() => toggleSort(which)}
        className={`table-th hover:text-navy-900 ${sort === which ? 'text-navy-900' : ''}`}
        title={`Sort by ${label}`}
      >
        {label} {sort === which ? (sortDir === 'asc' ? '▲' : '▼') : ''}
      </button>
    )
  }

  return (
    <div className="mx-auto max-w-[1560px] animate-rise space-y-6">
      <section className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-950">Transaction history</h1>
          <p className="mt-2 text-sm text-slate-500">Full audit trail with immutable double-entry detail and invoice documents.</p>
          {trader && (
            <div className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 rounded-xl border border-navy-900/10 bg-navy-50/60 px-3 py-2 font-mono text-xs text-navy-900">
              <span className="rounded-full bg-navy-950 px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.14em] text-white">
                Viewing as
              </span>
              <span className="font-bold">{trader.full_name}</span>
              <span className="hidden text-slate-500 sm:inline">
                · every record below touches this profile's accounts — sent or received
              </span>
            </div>
          )}
        </div>
        <div className="flex flex-wrap gap-2 rounded-2xl border border-slate-200 bg-white p-1 shadow-sm">
          {FILTERS.map((f) => (
            <button
              key={f}
              onClick={() => { setStatus(f); setPage(0) }}
              aria-pressed={status === f}
              className={`rounded-xl px-3 py-2 text-xs font-semibold transition-colors ${
                status === f ? 'bg-navy-950 text-white' : 'text-slate-600 hover:bg-slate-50'
              }`}
            >
              {f}
            </button>
          ))}
        </div>
      </section>

      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <AuditMetric title="Visible records" value={filtered.length.toLocaleString()} note={status} />
        <AuditMetric title="Success rate" value={`${audit.successRate.toFixed(1)}%`} note={`Of ${transfers.length} records`} />
        <AuditMetric title="Failed / reversed" value={audit.failedCount.toLocaleString()} note="Ledger exceptions" negative={audit.failedCount > 0} />
        <AuditMetric
          title="Avg finality"
          value={audit.avgFinality !== null ? `${audit.avgFinality}s` : '—'}
          note={audit.sampled ? `Sampled ${audit.sampled} settled` : 'No settled records yet'}
        />
      </section>

      {loading ? (
        <Card><SkeletonTable rows={8} /></Card>
      ) : error ? (
        <ErrorBanner message={error} onRetry={refresh} />
      ) : filtered.length === 0 ? (
        <Card className="overflow-hidden">
          <EmptyState icon="≡" title="No records" description="Nothing matches the current filter." />
        </Card>
      ) : (
        <Card>
          <div className="overflow-x-auto rounded-t-xl border-b border-slate-100">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50">
                  <th className="table-th">Status</th>
                  <th className="table-th">Amount</th>
                  <th className="table-th">Currency</th>
                  <th className="table-th">Invoice</th>
                  <th className="table-th hidden md:table-cell">Reference</th>
                  <th className="table-th"><SortHeader label="Date" which="date" /></th>
                </tr>
              </thead>
              <tbody>
                {pageRows.map((t) => (
                  <tr
                    key={t.id}
                    onClick={() => toggleEntries(t)}
                    className="cursor-pointer border-b border-slate-100 transition-colors last:border-0 hover:bg-navy-50/60"
                  >
                    <td className="table-td"><TransferStatusPill t={t} /></td>
                    <td className="table-td font-mono font-semibold text-navy-900">{amount(t.amount)}</td>
                    <td className="table-td">{t.currency}</td>
                    <td className="table-td">{t.invoice_number ?? '—'}</td>
                    <td className="table-td hidden font-mono text-xs text-slate-500 md:table-cell">
                      {t.external_reference ?? t.id.slice(0, 13)}
                    </td>
                    <td className="table-td whitespace-nowrap text-slate-500">{t.created_at.slice(0, 10)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          <div className="flex items-center justify-between px-5 py-4 text-sm text-slate-500">
            <span>
              {filtered.length} record{filtered.length === 1 ? '' : 's'} · page {page + 1}/{pages}
            </span>
            <div className="flex gap-1">
              <button className="btn btn-secondary btn-sm" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                Prev
              </button>
              <button className="btn btn-secondary btn-sm" disabled={page >= pages - 1} onClick={() => setPage((p) => p + 1)}>
                Next
              </button>
            </div>
          </div>
        </Card>
      )}

      {openId && (() => {
        const t = transfers.find((x) => x.id === openId)
        if (!t) return null
        const hasInvoice = Boolean(t.invoice_number)
        return (
          <div className="animate-rise space-y-4">
            {hasInvoice && (
              <div className="flex gap-2">
                <button
                  onClick={() => setDetailTab('invoice')}
                  className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors ${
                    detailTab === 'invoice' ? 'bg-navy-900 text-white' : 'bg-white text-slate-600 hover:bg-slate-100'
                  }`}
                >
                  Invoice
                </button>
                <button
                  onClick={() => setDetailTab('entries')}
                  className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors ${
                    detailTab === 'entries' ? 'bg-navy-900 text-white' : 'bg-white text-slate-600 hover:bg-slate-100'
                  }`}
                >
                  Double-entry rows
                </button>
              </div>
            )}

            {detailTab === 'invoice' && hasInvoice ? (
              <InvoiceDocument invoice={invoiceFromTransfer(t)} paid={t.status === 'SUCCESS'} />
            ) : (
              <Card title={`Double-entry rows · ${openId.slice(0, 13)}…`} subtitle="Immutable ledger entries">
                <div className="grid gap-2 sm:grid-cols-2">
                  {(expanded[openId] ?? []).map((e) => (
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
              </Card>
            )}
          </div>
        )
      })()}
    </div>
  )
}

function AuditMetric({ title, value, note, negative }: { title: string; value: string; note: string; negative?: boolean }) {
  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm text-slate-500">{title}</p>
      <p className="mt-4 font-mono text-3xl font-bold text-slate-950">{value}</p>
      <p className={`mt-3 w-fit rounded-full px-2.5 py-1 text-xs font-bold ${negative ? 'bg-red-50 text-red-600' : 'bg-emerald-50 text-emerald-700'}`}>{note}</p>
    </article>
  )
}

// Local status pill so History stays self-contained while sharing the tone
// vocabulary with StatusBadge.
function TransferStatusPill({ t }: { t: Transfer }) {
  const tones: Record<string, string> = {
    SUCCESS: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    PENDING: 'border-amber-200 bg-amber-50 text-amber-700',
    PROCESSING: 'border-blue-200 bg-blue-50 text-blue-700',
    FAILED: 'border-red-200 bg-red-50 text-red-700',
    REVERSED: 'border-slate-200 bg-slate-100 text-slate-600',
  }
  return <span className={`badge ${tones[t.status] ?? tones.PENDING}`}>{t.status}</span>
}
