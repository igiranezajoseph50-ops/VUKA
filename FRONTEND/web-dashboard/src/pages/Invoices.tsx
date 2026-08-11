import { useMemo, useState, useEffect } from 'react'
import { useTrader } from '../state/TraderContext'
import { useInvoices } from '../hooks/useInvoices'
import { useBalances } from '../hooks/useBalances'
import { api } from '../api/client'
import type { Invoice, User } from '../api/types'
import { InvoiceForm, InvoiceDocument } from '../components/InvoiceForm'
import { Card, SkeletonTable, ErrorBanner } from '../components/ui/primitives'

export default function Invoices() {
  const { trader } = useTrader()
  const { wallets, refresh: refreshBalances } = useBalances(trader?.id ?? null)
  const { invoices, loading, error, refresh } = useInvoices(trader?.id ?? null)

  const [tab, setTab] = useState<'received' | 'issued'>('received')
  const [selectedInvoice, setSelectedInvoice] = useState<Invoice | null>(null)
  
  // Resolution of issuer/counterparty names for the selected invoice document
  const [issuerUser, setIssuerUser] = useState<User | null>(null)
  const [counterpartyUser, setCounterpartyUser] = useState<User | null>(null)

  // Payment state
  const [payingInvoice, setPayingInvoice] = useState<Invoice | null>(null)
  const [sourceWalletId, setSourceWalletId] = useState('')
  const [destWalletId, setDestWalletId] = useState('')
  const [payError, setPayError] = useState<string | null>(null)
  const [paySuccess, setPaySuccess] = useState(false)
  const [paying, setPaying] = useState(false)

  // Filter invoices based on direction
  const filteredInvoices = invoices.filter((inv) => {
    if (tab === 'issued') {
      return inv.issuer_user_id === trader?.id
    } else {
      return inv.counterparty_user_id === trader?.id
    }
  })

  // Real metrics computed from the invoice records currently loaded.
  const metrics = useMemo(() => {
    const now = Date.now()
    const overdue = invoices.filter((inv) => inv.status === 'ISSUED' && new Date(inv.due_date).getTime() < now)
    const paid = invoices.filter((inv) => inv.status === 'PAID')
    // Paid in the current calendar quarter.
    const qStart = new Date()
    qStart.setMonth(Math.floor(qStart.getMonth() / 3) * 3, 1)
    qStart.setHours(0, 0, 0, 0)
    const paidThisQuarter = paid.filter((inv) => new Date(inv.created_at).getTime() >= qStart.getTime())
    // Average invoice total across all records (in the invoice's currency).
    const totals = invoices.map((inv) =>
      inv.items.reduce((acc, l) => acc + l.quantity * l.unit_price, 0) * (1 + inv.vat_rate / 100),
    )
    const avg = totals.length ? totals.reduce((a, b) => a + b, 0) / totals.length : 0
    const currencies = Array.from(new Set(invoices.map((inv) => inv.currency))).join(' / ')
    return {
      total: invoices.length,
      overdue: overdue.length,
      paidThisQuarter: paidThisQuarter.length,
      avg,
      avgCurrency: currencies || '—',
    }
  }, [invoices])

  // Load user records for the selected invoice
  useEffect(() => {
    if (!selectedInvoice) {
      setIssuerUser(null)
      setCounterpartyUser(null)
      return
    }
    async function fetchDetails() {
      try {
        const issuer = await api.getUser(selectedInvoice!.issuer_user_id)
        const cp = await api.getUser(selectedInvoice!.counterparty_user_id)
        setIssuerUser(issuer)
        setCounterpartyUser(cp)
      } catch {
        // Fallback names
      }
    }
    fetchDetails()
  }, [selectedInvoice])

  // Prepare payment details
  useEffect(() => {
    if (!payingInvoice) {
      setSourceWalletId('')
      setDestWalletId('')
      setPayError(null)
      setPaySuccess(false)
      return
    }

    async function resolveDestAccount() {
      try {
        // Fetch issuer's accounts to find the BUSINESS wallet
        const { accounts } = await api.getAccounts(payingInvoice!.issuer_user_id)
        const bizAcc = accounts.find((a) => a.type === 'BUSINESS' && a.currency === payingInvoice!.currency)
        if (bizAcc) {
          setDestWalletId(bizAcc.id)
        } else {
          setPayError('Supplier does not have a business wallet in the invoice currency.')
        }
      } catch (err) {
        setPayError('Failed to resolve supplier destination wallet.')
      }
    }

    // Default source wallet to the active trader's business wallet matching invoice currency
    const matchingWallet = wallets.find(
      (w) => w.account.type === 'BUSINESS' && w.account.currency === payingInvoice.currency
    )
    if (matchingWallet) {
      setSourceWalletId(matchingWallet.account.id)
      setPayError(null)
    } else {
      setSourceWalletId('')
      const walletCcy = wallets.find((w) => w.account.type === 'BUSINESS')?.account.currency ?? '—'
      const viaPayments =
        wallets.some((w) => w.account.type === 'BUSINESS' && w.account.currency === 'RWF') &&
        payingInvoice.currency === 'KES'
          ? ' This is a cross-currency invoice: settle it from the Payments page (cross-border RWF → KES), '
          : ''
      setPayError(
        `This invoice is billed in ${payingInvoice.currency}, but your business wallet is in ${walletCcy}. ` +
          `The invoice Pay card settles in the invoice's own currency.${viaPayments}` +
          `or switch to a profile that holds a ${payingInvoice.currency} business wallet to settle it here.`
      )
    }

    resolveDestAccount()
  }, [payingInvoice, wallets])

  async function executePayment(e: React.FormEvent) {
    e.preventDefault()
    if (!payingInvoice || !sourceWalletId || !destWalletId) return

    setPaying(true)
    setPayError(null)
    setPaySuccess(false)

    // Compute invoice total
    const subtotal = payingInvoice.items.reduce((acc, l) => acc + l.quantity * l.unit_price, 0)
    const total = subtotal * (1 + payingInvoice.vat_rate / 100)

    // Check balance
    const wallet = wallets.find((w) => w.account.id === sourceWalletId)
    if (wallet && wallet.balance < total) {
      setPayError(`Insufficient balance. Invoice total is ${total} ${payingInvoice.currency}, but you only have ${wallet.balance} ${payingInvoice.currency}.`)
      setPaying(false)
      return
    }

    try {
      await api.createTransfer({
        source_account_id: sourceWalletId,
        destination_account_id: destWalletId,
        amount: total,
        currency: payingInvoice.currency,
        invoice_number: payingInvoice.number,
      })
      setPaySuccess(true)
      refresh()
      refreshBalances()
      setTimeout(() => {
        setPayingInvoice(null)
      }, 1500)
    } catch (err) {
      setPayError((err as Error).message)
    } finally {
      setPaying(false)
    }
  }

  return (
    <div className="mx-auto max-w-[1560px] animate-rise space-y-6">
      <section className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-950">Invoice management</h1>
          <p className="mt-2 text-sm text-slate-500">
            Issue, receive, preview and settle commercial invoices across the VUKA network.
          </p>
        </div>
      </section>

      <section className="grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <InvoiceMetric title="Total invoices" value={metrics.total.toLocaleString()} note="All counterparties" />
        <InvoiceMetric title="Require attention" value={metrics.overdue.toLocaleString()} note="Overdue and unpaid" negative={metrics.overdue > 0} />
        <InvoiceMetric title="Paid this quarter" value={metrics.paidThisQuarter.toLocaleString()} note="From the register" />
        <InvoiceMetric title="Avg invoice value" value={metrics.avg ? `${metrics.avg.toFixed(0)} ${metrics.avgCurrency}` : '—'} note="Across loaded records" />
      </section>

      <div className="grid gap-6 xl:grid-cols-[2fr_1fr]">
        {/* Left Column: List and Toggle */}
        <div className="lg:col-span-2 space-y-4">
          {/* Tabs */}
          <div className="inline-flex rounded-2xl border border-slate-200 bg-white p-1 shadow-sm">
            {(['received', 'issued'] as const).map((t) => (
              <button
                key={t}
                onClick={() => {
                  setTab(t)
                  setSelectedInvoice(null)
                  setPayingInvoice(null)
                }}
                className={`rounded-xl px-4 py-2 text-sm font-semibold transition-colors ${
                  tab === t
                    ? 'bg-navy-950 text-white'
                    : 'text-slate-500 hover:bg-slate-50 hover:text-slate-700'
                }`}
              >
                {t === 'received' ? 'Received Invoices' : 'Issued Invoices'}
              </button>
            ))}
          </div>

          {/* Invoices List */}
          <Card title={tab === 'received' ? 'Received invoices' : 'Issued invoices'} subtitle="Operational queue with payment status, due dates and actions">
            {loading ? (
              <SkeletonTable rows={4} />
            ) : error ? (
              <ErrorBanner message={error} onRetry={refresh} />
            ) : filteredInvoices.length === 0 ? (
              <p className="py-6 text-center text-sm text-slate-500">
                No {tab} invoices found.
              </p>
            ) : (
              <div className="divide-y divide-slate-100">
                {filteredInvoices.map((inv) => {
                  const sub = inv.items.reduce((acc, l) => acc + l.quantity * l.unit_price, 0)
                  const total = sub * (1 + inv.vat_rate / 100)
                  return (
                    <div key={inv.id} className="grid gap-3 px-1 py-4 md:grid-cols-[1fr_auto_auto] md:items-center">
                      <div>
                        <div className="font-mono font-semibold text-slate-950">{inv.number}</div>
                        <div className="text-xs text-slate-500 mt-0.5">
                          Total: <span className="font-semibold">{total.toFixed(2)} {inv.currency}</span> · Due: {inv.due_date}
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        {inv.status === 'PAID' ? (
                          <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-semibold text-emerald-800">
                            Paid
                          </span>
                        ) : (
                          <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-800">
                            Unpaid
                          </span>
                        )}
                        <button
                          onClick={() => setSelectedInvoice(inv)}
                          className="text-xs font-semibold text-emerald-600 hover:text-emerald-700"
                        >
                          View
                        </button>
                        {tab === 'received' && inv.status === 'ISSUED' && (
                          <button
                            onClick={() => setPayingInvoice(inv)}
                            className="rounded bg-navy-900 px-2.5 py-1 text-xs font-semibold text-white hover:bg-navy-800"
                          >
                            Pay
                          </button>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </Card>
        </div>

        {/* Right Column: Creation Form / Document View / Payment Form */}
        <div className="space-y-4">
          {payingInvoice ? (
            <Card title="Pay Invoice" subtitle={`Settling ${payingInvoice.number}`}>
              <form onSubmit={executePayment} className="space-y-4">
                {payError && (
                  <div className="rounded-lg bg-red-50 p-3 text-xs text-red-700 border border-red-200">
                    {payError}
                  </div>
                )}
                {paySuccess && (
                  <div className="rounded-lg bg-emerald-50 p-3 text-xs text-emerald-800 border border-emerald-200">
                    Payment successful! Invoice is settled.
                  </div>
                )}

                <div>
                  <label className="text-xs text-slate-500 font-medium">Source Account (Business wallet)</label>
                  <select
                    value={sourceWalletId}
                    onChange={(e) => setSourceWalletId(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm bg-white"
                    required
                    disabled={
                      !wallets.some(
                        (w) => w.account.type === 'BUSINESS' && w.account.currency === payingInvoice.currency
                      )
                    }
                  >
                    {wallets
                      .filter((w) => w.account.type === 'BUSINESS' && w.account.currency === payingInvoice.currency)
                      .map((w) => (
                        <option key={w.account.id} value={w.account.id}>
                          {w.account.currency} · Business Balance: {w.balance.toFixed(2)}
                        </option>
                      ))}
                  </select>
                </div>

                <div className="border-t border-slate-100 pt-3 flex justify-between gap-2">
                  <button
                    type="button"
                    onClick={() => setPayingInvoice(null)}
                    className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={paying || !sourceWalletId || !destWalletId}
                    className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-50"
                  >
                    {paying ? 'Processing...' : 'Confirm Payment'}
                  </button>
                </div>
              </form>
            </Card>
          ) : selectedInvoice ? (
            <div className="space-y-3">
              <div className="flex justify-between items-center">
                <h3 className="text-sm font-bold uppercase text-slate-950">Document View</h3>
                <button
                  onClick={() => setSelectedInvoice(null)}
                  className="text-xs font-semibold text-slate-500 hover:text-slate-700"
                >
                  Close
                </button>
              </div>
              <InvoiceDocument
                invoice={selectedInvoice}
                issuerName={issuerUser?.full_name ?? 'Supplier'}
                counterpartyName={counterpartyUser?.full_name ?? 'Buyer'}
                issuerReg={issuerUser?.business_reg_number}
                issuerPhone={issuerUser?.phone_number}
                counterpartyPhone={counterpartyUser?.phone_number}
              />
            </div>
          ) : (
            <InvoiceForm currentTrader={trader!} onInvoiceCreated={refresh} />
          )}
        </div>
      </div>
    </div>
  )
}

function InvoiceMetric({ title, value, note, negative }: { title: string; value: string; note: string; negative?: boolean }) {
  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm text-slate-500">{title}</p>
      <p className="mt-4 font-mono text-3xl font-bold text-slate-950">{value}</p>
      <p className={`mt-3 w-fit rounded-full px-2.5 py-1 text-xs font-bold ${negative ? 'bg-red-50 text-red-600' : 'bg-emerald-50 text-emerald-700'}`}>{note}</p>
    </article>
  )
}
