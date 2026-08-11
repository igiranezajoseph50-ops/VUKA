import { useEffect, useState, useMemo } from 'react'
import { api } from '../api/client'
import type { User, CreateInvoiceRequest, InvoiceItemRequest } from '../api/types'
import BrandLogo from './BrandLogo'
import { amountInWords } from '../lib/format'

// VUKA invoice-number format (mirrors the engine regexp INV-\d{4}-\d{3,6}).
const INVOICE_NUMBER_RE = /^INV-\d{4}-\d{3,6}$/

// Compatibility types for tests and History.tsx
export interface InvoiceLine {
  description: string
  quantity: number
  unitPrice: number
}

export interface Invoice {
  number: string
  issue_date: string
  due_date: string
  currency: string
  counterparty: string
  counterparty_address: string
  line_items: InvoiceLine[]
  vat_rate: number
  terms: string
  notes: string
  created_at: string
}

export function invoiceSubtotal(inv: Invoice | any): number {
  const items = inv.line_items || inv.items || []
  return items.reduce((acc: number, l: any) => acc + (l.quantity || 0) * (l.unitPrice || l.unit_price || 0), 0)
}

export function invoiceVat(inv: Invoice | any): number {
  return invoiceSubtotal(inv) * ((inv.vat_rate || 0) / 100)
}

export function invoiceTotal(inv: Invoice | any): number {
  return invoiceSubtotal(inv) + invoiceVat(inv)
}

export function invoiceFromTransfer(t: {
  id: string
  invoice_number?: string
  amount: number
  currency: string
  created_at: string
}): Invoice {
  const issue = (t.created_at || '').slice(0, 10) || new Date().toISOString().slice(0, 10)
  const due = new Date(issue)
  due.setDate(due.getDate() + 30)
  return {
    number: t.invoice_number || `TRF-${t.id.slice(0, 8)}`,
    issue_date: issue,
    due_date: due.toISOString().slice(0, 10),
    currency: t.currency,
    counterparty: 'VUKA payment',
    counterparty_address: 'Cross-border trade',
    line_items: [
      { description: 'Invoice-linked payment', quantity: 1, unitPrice: Math.abs(t.amount) },
    ],
    vat_rate: 0,
    terms: 'Paid via VUKA — automatically settled on transfer.',
    notes: `Transfer reference: ${t.id.slice(0, 13)}`,
    created_at: t.created_at,
  }
}

interface Props {
  currentTrader: User
  onInvoiceCreated: () => void
}

const DEMO_PHONES = ['+250700000991', '+254700000882', '+250700000773']

const today = () => new Date().toISOString().slice(0, 10)
const plus30 = () => {
  const d = new Date()
  d.setDate(d.getDate() + 30)
  return d.toISOString().slice(0, 10)
}

export function InvoiceForm({ currentTrader, onInvoiceCreated }: Props) {
  const [number, setNumber] = useState('')
  const [issueDate, setIssueDate] = useState(today)
  const [dueDate, setDueDate] = useState(plus30)
  const [currency, setCurrency] = useState('RWF')
  const [counterpartyUserId, setCounterpartyUserId] = useState('')
  const [lines, setLines] = useState<InvoiceItemRequest[]>([
    { description: '', quantity: 1, unit_price: 0 },
  ])
  const [vatRate, setVatRate] = useState(18)
  const [terms, setTerms] = useState('Payment due within 30 days of invoice date.')
  const [notes, setNotes] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<boolean>(false)

  const [availableCounterparties, setAvailableCounterparties] = useState<User[]>([])

  // Resolve demo counterparties from database on mount
  useEffect(() => {
    async function loadCounterparties() {
      const users: User[] = []
      for (const phone of DEMO_PHONES) {
        if (phone === currentTrader.phone_number) continue
        try {
          const u = await api.getUserByPhone(phone)
          users.push(u)
        } catch {
          // might not be registered yet in this session
        }
      }
      setAvailableCounterparties(users)
      if (users.length > 0) {
        setCounterpartyUserId(users[0].id)
      }
    }
    loadCounterparties()
  }, [currentTrader])

  function addLine() {
    setLines((prev) => [...prev, { description: '', quantity: 1, unit_price: 0 }])
  }

  function updateLine(i: number, patch: Partial<InvoiceItemRequest>) {
    setLines((prev) => prev.map((l, idx) => (idx === i ? { ...l, ...patch } as InvoiceItemRequest : l)))
  }

  function removeLine(i: number) {
    if (lines.length > 1) {
      setLines((prev) => prev.filter((_, idx) => idx !== i))
    }
  }

  const subtotal = useMemo(() => {
    return lines.reduce((acc, l) => acc + (l.quantity || 0) * (l.unit_price || 0), 0)
  }, [lines])

  const vatAmount = useMemo(() => {
    return subtotal * (vatRate / 100)
  }, [subtotal, vatRate])

  const total = useMemo(() => {
    return subtotal + vatAmount
  }, [subtotal, vatAmount])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!number.trim()) return
    if (!INVOICE_NUMBER_RE.test(number.trim())) {
      setError('Invoice number must look like INV-2026-00001 (INV-<year>-<sequence>).')
      return
    }
    const filteredLines = lines.filter((l) => l.description.trim() !== '')
    if (filteredLines.length === 0) {
      setError('Invoice requires at least one line item with a description')
      return
    }
    if (!counterpartyUserId) {
      setError('Please select a counterparty')
      return
    }

    setBusy(true)
    setError(null)
    setSuccess(false)

    const payload: CreateInvoiceRequest = {
      number: number.trim(),
      counterparty_user_id: counterpartyUserId,
      currency,
      issue_date: issueDate,
      due_date: dueDate,
      vat_rate: vatRate,
      terms: terms || undefined,
      notes: notes || undefined,
      items: filteredLines,
    }

    try {
      await api.createInvoice(currentTrader.id, payload)
      setSuccess(true)
      // reset form
      setNumber('')
      setLines([{ description: '', quantity: 1, unit_price: 0 }])
      setNotes('')
      setIssueDate(today())
      setDueDate(plus30())
      onInvoiceCreated()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const moneyFmt = (v: number) =>
    new Intl.NumberFormat('en-UG', { style: 'currency', currency, minimumFractionDigits: 2 }).format(v)

  return (
    <form onSubmit={submit} className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h3 className="mb-4 text-sm font-bold uppercase tracking-wide text-navy-900">New Invoice</h3>

      {error && (
        <div className="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 border border-red-200">
          {error}
        </div>
      )}
      {success && (
        <div className="mb-4 rounded-lg bg-emerald-50 p-3 text-sm text-emerald-800 border border-emerald-200">
          Invoice created successfully!
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <label className="block text-xs text-slate-500">
          Invoice Number
          <input
            value={number}
            onChange={(e) => setNumber(e.target.value)}
            placeholder="e.g. INV-2026-001"
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            required
          />
        </label>
        <label className="block text-xs text-slate-500">
          Currency
          <select
            value={currency}
            onChange={(e) => setCurrency(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          >
            <option value="RWF">RWF</option>
            <option value="KES">KES</option>
          </select>
        </label>
        <label className="block text-xs text-slate-500">
          Issue date
          <input
            type="date"
            value={issueDate}
            onChange={(e) => setIssueDate(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            required
          />
        </label>
        <label className="block text-xs text-slate-500">
          Due date
          <input
            type="date"
            value={dueDate}
            onChange={(e) => setDueDate(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            required
          />
        </label>
      </div>

      <div className="mt-3">
        <label className="block text-xs text-slate-500">
          Bill To (Counterparty)
          <select
            value={counterpartyUserId}
            onChange={(e) => setCounterpartyUserId(e.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            required
          >
            {availableCounterparties.length === 0 ? (
              <option value="" disabled>No other traders registered yet...</option>
            ) : (
              availableCounterparties.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.full_name} ({u.phone_number})
                </option>
              ))
            )}
          </select>
        </label>
      </div>

      {/* Line items */}
      <div className="mt-4">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Line items</span>
          <button
            type="button"
            onClick={addLine}
            className="text-xs font-semibold text-emerald-600 hover:text-emerald-700"
          >
            + Add line
          </button>
        </div>
        <div className="space-y-2">
          {lines.map((line, i) => (
            <div key={i} className="grid grid-cols-[1fr_64px_96px_28px] gap-2 items-center">
              <input
                value={line.description}
                onChange={(e) => updateLine(i, { description: e.target.value })}
                placeholder="Description (e.g. Rice shipment)"
                className="rounded-lg border border-slate-300 px-3 py-2 text-sm"
                required
              />
              <input
                type="number"
                min="1"
                step="any"
                value={line.quantity || ''}
                placeholder="Qty"
                onChange={(e) => updateLine(i, { quantity: parseFloat(e.target.value) || 0 })}
                className="rounded-lg border border-slate-300 px-2 py-2 text-sm"
                required
              />
              <input
                type="number"
                min="0"
                step="0.01"
                value={line.unit_price || ''}
                placeholder="Price"
                onChange={(e) => updateLine(i, { unit_price: parseFloat(e.target.value) || 0 })}
                className="rounded-lg border border-slate-300 px-2 py-2 text-sm"
                required
              />
              <button
                type="button"
                onClick={() => removeLine(i)}
                className="text-slate-400 hover:text-red-500 text-lg font-bold"
                disabled={lines.length === 1}
              >
                ×
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-3 grid grid-cols-3 gap-3">
        <label className="text-xs text-slate-500">
          VAT %
          <input
            type="number"
            min="0"
            max="100"
            value={vatRate}
            onChange={(e) => setVatRate(Number(e.target.value) || 0)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          />
        </label>
        <div className="col-span-2">
          <label className="text-xs text-slate-500">
            Payment terms
            <input
              value={terms}
              onChange={(e) => setTerms(e.target.value)}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
            />
          </label>
        </div>
      </div>

      <div className="mt-3">
        <label className="text-xs text-slate-500">
          Notes (optional)
          <input
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Terms, bank info, details..."
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm"
          />
        </label>
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-slate-100 pt-3">
        <div className="text-sm">
          <div className="text-slate-500">Total (incl. VAT)</div>
          <div className="font-mono text-lg font-bold text-navy-900">{moneyFmt(total)}</div>
        </div>
        <button
          type="submit"
          disabled={busy || availableCounterparties.length === 0}
          className="rounded-lg bg-navy-900 px-4 py-2 text-sm font-semibold text-white hover:bg-navy-800 disabled:opacity-50"
        >
          {busy ? 'Creating...' : 'Create Invoice'}
        </button>
      </div>
    </form>
  )
}

// InvoiceDocument — a realistic, printable A4-style trade invoice.
export function InvoiceDocument({ invoice, paid, issuerName, counterpartyName, issuerReg, issuerPhone, counterpartyPhone }: {
  invoice: any
  paid?: boolean
  issuerName?: string
  counterpartyName?: string
  issuerReg?: string
  issuerPhone?: string
  counterpartyPhone?: string
}) {
  const items = invoice.line_items || invoice.items || []
  const sub = invoiceSubtotal(invoice)
  const vat = invoiceVat(invoice)
  const total = invoiceTotal(invoice)
  const isPaid = paid !== undefined ? paid : invoice.status === 'PAID'

  const moneyFmt = (v: number) =>
    new Intl.NumberFormat('en-UG', { style: 'currency', currency: invoice.currency, minimumFractionDigits: 2 }).format(v)

  return (
    <div className="rounded-xl border border-slate-300 bg-white p-0 shadow-sm">
      <div className="rounded-t-xl bg-navy-950 px-6 py-4 text-white">
        <div className="flex items-center justify-between">
          <div>
            <BrandLogo size={40} wordmark="VUKA" sublabel="Cross-border trade payments" />
          </div>
          <div className="text-right text-xs text-navy-200">
            <div className="text-lg font-bold text-white">INVOICE</div>
            <div>{invoice.number}</div>
          </div>
        </div>
      </div>

      <div className="px-6 py-5">
        <div className="flex justify-between gap-4 text-sm">
          <div>
            <div className="text-xs uppercase tracking-wide text-slate-400">Issued by</div>
            <div className="mt-1 font-semibold text-navy-900">{issuerName || 'Supplier'}</div>
            <div className="text-slate-500">Exporter / Seller</div>
            {issuerReg && <div className="mt-1 font-mono text-xs text-slate-600">TIN/REC: {issuerReg}</div>}
            {issuerPhone && <div className="font-mono text-xs text-slate-600">{issuerPhone}</div>}
          </div>
          <div className="text-right">
            <div className="text-xs uppercase tracking-wide text-slate-400">Bill to</div>
            <div className="mt-1 font-semibold text-navy-900">{counterpartyName || 'Buyer'}</div>
            <div className="text-slate-500">Importer / Buyer</div>
            {counterpartyPhone && <div className="mt-1 font-mono text-xs text-slate-600">{counterpartyPhone}</div>}
          </div>
        </div>

        <div className="mt-4 flex gap-8 border-y border-slate-100 py-2 text-xs text-slate-500">
          <span>Issue date: <span className="font-medium text-slate-700">{invoice.issue_date}</span></span>
          <span>Due date: <span className="font-medium text-slate-700">{invoice.due_date}</span></span>
          <span>Currency: <span className="font-medium text-slate-700">{invoice.currency}</span></span>
          <span className="ml-auto">
            Status:{' '}
            {isPaid ? (
              <span className="font-semibold text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded-full border border-emerald-200">PAID</span>
            ) : (
              <span className="font-semibold text-amber-600 bg-amber-50 px-2 py-0.5 rounded-full border border-amber-200">UNPAID</span>
            )}
          </span>
        </div>

        {/* Line items table */}
        <table className="mt-4 w-full text-sm">
          <thead>
            <tr className="border-b-2 border-navy-900 text-left text-xs uppercase tracking-wide text-slate-500">
              <th className="py-2 pr-2">Description</th>
              <th className="py-2 text-right">Qty</th>
              <th className="py-2 text-right">Unit price</th>
              <th className="py-2 text-right">Amount</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 ? (
              <tr><td colSpan={4} className="py-3 text-slate-400">No line items.</td></tr>
            ) : (
              items.map((l: any, i: number) => (
                <tr key={i} className="border-b border-slate-100">
                  <td className="py-2 pr-2 text-slate-700">{l.description}</td>
                  <td className="py-2 text-right text-slate-600">{l.quantity}</td>
                  <td className="py-2 text-right font-mono text-slate-600">{moneyFmt(l.unitPrice || l.unit_price || 0)}</td>
                  <td className="py-2 text-right font-mono font-semibold text-navy-900">{moneyFmt(l.quantity * (l.unitPrice || l.unit_price || 0))}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>

        {/* Totals */}
        <div className="mt-4 flex justify-end">
          <div className="w-72 space-y-1 text-sm">
            <div className="flex justify-between text-slate-600">
              <span>Subtotal</span><span className="font-mono">{moneyFmt(sub)}</span>
            </div>
            <div className="flex justify-between text-slate-600">
              <span>VAT ({invoice.vat_rate}%)</span><span className="font-mono">{moneyFmt(vat)}</span>
            </div>
            <div className="flex justify-between border-t border-navy-900 pt-2 text-base font-bold text-navy-900">
              <span>Total</span><span className="font-mono">{moneyFmt(total)}</span>
            </div>
            <div className="border-t border-slate-100 pt-2 text-right text-[11px] italic leading-5 text-slate-500">
              {amountInWords(total, invoice.currency)}
            </div>
          </div>
        </div>

        {/* Terms + payment reference */}
        <div className="mt-6 gap-6 rounded-lg bg-slate-50 p-4 text-xs text-slate-600 sm:flex sm:justify-between">
          <div>
            <div className="font-semibold uppercase tracking-wide text-slate-400">Payment terms</div>
            <div className="mt-1">{invoice.terms || 'Standard payment terms'}</div>
            {invoice.notes && <div className="mt-1">{invoice.notes}</div>}
          </div>
          <div className="mt-3 sm:mt-0 sm:text-right">
            <div className="font-semibold uppercase tracking-wide text-slate-400">Remit via VUKA</div>
            <div className="mt-1 font-mono text-navy-900">
              Invoice ref: <span className="font-bold">{invoice.number}</span>
            </div>
            <div className="text-emerald-700">{isPaid ? 'Payment received ✓' : 'Awaiting payment'}</div>
          </div>
        </div>
      </div>
    </div>
  )
}