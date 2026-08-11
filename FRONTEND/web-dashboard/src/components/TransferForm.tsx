// TransferForm — same-currency + cross-border transfer with live FX preview.
//
// Same-currency transfers execute via the engine REST /transfers endpoint;
// cross-border (RWF -> KES) calls /transfers/cross-border, which runs the
// engine's FX settlement flow (mirrors the gRPC corridor). The cross-border
// destination is resolved live from the seeded Kenya counterparty's KES
// business wallet via the API — no hardcoded account ids.
import { useEffect, useMemo, useState } from 'react'
import type { Wallet } from '../hooks/useBalances'
import type { TransferActionResult } from '../hooks/useTransfers'
import type { Account } from '../api/types'
import { api } from '../api/client'
import { money, convert, amount } from '../lib/format'
import { Button, Card } from '../components/ui/primitives'

interface Props {
  wallets: Wallet[]
  fxRate: number | null
  onSubmit: (
    req: { source_account_id: string; destination_account_id: string; amount: number; currency: string; invoice_number?: string },
  ) => Promise<TransferActionResult>
  onCrossBorderSubmit: (
    req: { source_account_id: string; destination_account_id: string; amount: number; currency_from: string; currency_to: string; fx_rate?: number; invoice_number?: string },
  ) => Promise<TransferActionResult>
}

interface FieldError {
  source?: string
  destination?: string
  amount?: string
}

/** Seed-backed counterparties. Mirrors cmd/seed/main.go demo traders. */
const COUNTERPARTIES: { label: string; phone: string; currency: string }[] = [
  { label: 'Kethan Gasana · Nairobi Roasters Ltd', phone: '+254700000882', currency: 'KES' },
]

function initials(name: string): string {
  return name
    .split(' ')
    .map((s) => s[0])
    .slice(0, 2)
    .join('')
}

export function TransferForm({ wallets, fxRate, onSubmit, onCrossBorderSubmit }: Props) {
  const businessWallets = wallets.filter((w) => w.account.type === 'BUSINESS')
  // Cross-border outbound is only possible from an RWF business wallet
  // (the engine's settlement corridor debits RWF and pays out KES).
  const canCross = businessWallets.some((w) => w.currency === 'RWF')
  const [mode, setMode] = useState<'local' | 'cross'>('local')
  const [source, setSource] = useState('')
  const [dest, setDest] = useState('')
  const [counterparty, setCounterparty] = useState('')
  const [counterpartyWallet, setCounterpartyWallet] = useState<string | null>(null)
  const [counterpartyLoading, setCounterpartyLoading] = useState(false)
  const [counterpartyError, setCounterpartyError] = useState<string | null>(null)
  const [rawAmount, setRawAmount] = useState('')
  const [invoice, setInvoice] = useState('')
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<TransferActionResult | null>(null)
  const [errors, setErrors] = useState<FieldError>({})

  // When the trader switches to cross-border mode, resolve the counterparty's
  // KES BUSINESS wallet from the live engine.
  useEffect(() => {
    if (mode !== 'cross' || !canCross || counterpartyWallet) return
    let cancelled = false
    const cp = COUNTERPARTIES[0]
    setCounterpartyLoading(true)
    setCounterpartyError(null)
    ;(async () => {
      try {
        const user = await api.getUserByPhone(cp.phone)
        const { accounts } = await api.getAccounts(user.id)
        const wallet = accounts.find((a: Account) => a.type === 'BUSINESS' && a.currency === 'KES')
        if (cancelled) return
        if (!wallet) {
          setCounterpartyError('No KES business wallet found for the seeded counterparty.')
          return
        }
        setCounterparty( `${user.full_name} · ${cp.label}`)
        setCounterpartyWallet(wallet.id)
      } catch {
        if (!cancelled) setCounterpartyError('Could not reach the engine to resolve the counterparty wallet.')
      } finally {
        if (!cancelled) setCounterpartyLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [mode, canCross, counterpartyWallet])

  const amountNum = Number.parseFloat(rawAmount)
  const validAmount = Number.isFinite(amountNum) && amountNum > 0
  const sourceWallet = businessWallets.find((w) => w.account.id === source)
  const insufficient = validAmount && sourceWallet ? amountNum > sourceWallet.balance : false
  const sourceCcy = mode === 'cross' ? 'RWF' : (sourceWallet?.currency ?? 'RWF')
  const kesPreview = mode === 'cross' && validAmount && fxRate ? convert(amountNum, fxRate) : 0

  const preview = useMemo(() => {
    if (!validAmount) return null
    if (mode === 'cross') {
      return { label: 'Supplier receives', value: money(kesPreview, 'KES') }
    }
    return { label: 'Total to send', value: money(amountNum, sourceCcy) }
  }, [validAmount, mode, kesPreview, amountNum, sourceCcy])

  function validate(): boolean {
    const e: FieldError = {}
    if (!source) e.source = 'Select a source wallet'
    if (mode === 'cross') {
      if (!counterpartyWallet) e.destination = 'Select a counterparty with a KES wallet first'
    } else if (!dest) {
      e.destination = 'Select a destination wallet'
    }
    if (!validAmount) e.amount = 'Enter a positive amount'
    else if (insufficient && sourceWallet) e.amount = `Insufficient balance (${money(sourceWallet.balance, sourceWallet.currency)} available)`
    setErrors(e)
    return Object.keys(e).length === 0
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!validate() || !source) return
    setBusy(true)
    setResult(null)
    let res: TransferActionResult
    if (mode === 'cross') {
      // Always route cross-border mode to the FX settlement endpoint.
      // fx_rate is passed when we have a live rate; otherwise the engine
      // falls back to its configured corridor default.
      res = await onCrossBorderSubmit({
        source_account_id: source,
        destination_account_id: counterpartyWallet!,
        amount: amountNum,
        currency_from: 'RWF',
        currency_to: 'KES',
        ...(fxRate ? { fx_rate: fxRate } : {}),
        invoice_number: invoice || undefined,
      })
    } else {
      res = await onSubmit({
        source_account_id: source,
        destination_account_id: dest,
        amount: amountNum,
        currency: sourceCcy,
        invoice_number: invoice || undefined,
      })
    }
    setBusy(false)
    setResult(res)
    if (res.ok) {
      setRawAmount('')
      setInvoice('')
      setErrors({})
    }
  }

  const ccyOfSource = sourceWallet?.currency ?? 'RWF'

  return (
    <Card
      title="Send a payment"
      subtitle="Invoice-linked, idempotent — replaying the same key never double-charges."
    >
      <div className="mb-5 flex gap-2">
        {(['local', 'cross'] as const).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => {
              setMode(m)
              setErrors({})
              setResult(null)
            }}
            aria-pressed={mode === m}
            className={`rounded-lg px-3 py-1.5 text-sm font-semibold transition-colors ${
              mode === m ? 'bg-navy-900 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            {m === 'local' ? 'Same-currency' : 'Cross-border (RWF → KES)'}
          </button>
        ))}
      </div>

      {mode === 'cross' && !canCross && (
        <div role="alert" className="mb-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          Cross-border outbound is demoed from a Rwanda (RWF) profile. Switch to a RWF trader to pay a Nairobi supplier.
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className="input-label" htmlFor="tf-source">Source (business wallet)</label>
            <select
              id="tf-source"
              value={source}
              onChange={(e) => { setSource(e.target.value); setErrors((p) => ({ ...p, source: undefined })) }}
              className={`input ${errors.source ? 'border-red-400' : ''}`}
            >
              <option value="" disabled>Select source…</option>
              {businessWallets.map((w) => (
                <option key={w.account.id} value={w.account.id}>
                  {w.account.currency} · {money(w.balance, w.account.currency)}
                </option>
              ))}
            </select>
            {errors.source && <p className="mt-1 text-xs text-red-600">{errors.source}</p>}
          </div>

          {mode === 'local' ? (
            <div>
              <label className="input-label" htmlFor="tf-dest">Destination (business wallet)</label>
              <select
                id="tf-dest"
                value={dest}
                onChange={(e) => { setDest(e.target.value); setErrors((p) => ({ ...p, destination: undefined })) }}
                className={`input ${errors.destination ? 'border-red-400' : ''}`}
              >
                <option value="" disabled>Select destination…</option>
                {businessWallets
                  .filter((w) => w.account.id !== source)
                  .map((w) => (
                    <option key={w.account.id} value={w.account.id}>
                      {w.account.currency} · {w.account.type}
                    </option>
                  ))}
              </select>
              {errors.destination && <p className="mt-1 text-xs text-red-600">{errors.destination}</p>}
            </div>
          ) : (
            <div>
              <label className="input-label" htmlFor="tf-cp">Counterparty (Kenya)</label>
              <div className="relative">
                {counterpartyLoading ? (
                  <div className="input bg-slate-50 text-slate-500">Resolving counterparty wallet…</div>
                ) : counterpartyError ? (
                  <div className="input border-red-200 bg-red-50 text-sm text-red-700">{counterpartyError}</div>
                ) : counterparty ? (
                  <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-3 py-2">
                    <div className="flex items-center gap-2">
                      <span className="grid h-8 w-8 place-items-center rounded-full bg-navy-950 text-[11px] font-black text-white">{initials(counterparty)}</span>
                      <span className="text-sm font-semibold text-slate-800">{counterparty}</span>
                    </div>
                    <span className="rounded-full bg-emerald-50 px-2 py-0.5 font-mono text-xs font-bold text-emerald-700">KES</span>
                  </div>
                ) : (
                  <div className="flex items-center justify-between rounded-lg border border-slate-200 px-3 py-2 text-sm text-slate-400">
                    <span>{COUNTERPARTIES[0].label}</span>
                    <span>KES</span>
                  </div>
                )}
              </div>
              {errors.destination && <p className="mt-1 text-xs text-red-600">{errors.destination}</p>}
            </div>
          )}
        </div>

        <div>
          <label className="input-label" htmlFor="tf-amount">
            Amount ({mode === 'cross' ? 'RWF' : ccyOfSource})
          </label>
          <div className="relative">
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 font-mono text-sm text-slate-400">
              {mode === 'cross' ? 'RWF' : ccyOfSource}
            </span>
            <input
              id="tf-amount"
              type="number"
              min="0"
              step="0.01"
              value={rawAmount}
              onChange={(e) => { setRawAmount(e.target.value); setErrors((p) => ({ ...p, amount: undefined })) }}
              className={`input pl-16 font-mono ${errors.amount ? 'border-red-400' : ''}`}
              placeholder="0.00"
            />
          </div>
          {errors.amount && <p className="mt-1 text-xs text-red-600">{errors.amount}</p>}
          {sourceWallet && validAmount && !insufficient && mode === 'local' && (
            <p className="mt-1 text-xs text-slate-500">
              Available: <span className="font-medium">{money(sourceWallet.balance, sourceWallet.currency)}</span>
            </p>
          )}
        </div>

        {mode === 'cross' && validAmount && (
          <div className="animate-rise rounded-xl border border-emerald-500/40 bg-emerald-50 p-4">
            {fxRate ? (
              <>
                <div className="flex items-center justify-between text-sm">
                  <span className="font-medium text-emerald-800">Visible FX rate</span>
                  <span className="rounded bg-emerald-600 px-2 py-0.5 font-mono text-xs font-bold text-white">
                    1 KES = {amount(fxRate)} RWF
                  </span>
                </div>
                <div className="mt-3 flex items-center justify-between border-t border-emerald-200 pt-3">
                  <span className="text-sm text-emerald-800">Supplier receives</span>
                  <span className="font-mono text-2xl font-bold text-navy-900">{money(kesPreview, 'KES')}</span>
                </div>
              </>
            ) : (
              <div className="text-sm text-slate-500">Fetching live corridor FX rate…</div>
            )}
          </div>
        )}

        <div>
          <label className="input-label" htmlFor="tf-invoice">Invoice number (optional)</label>
          <input
            id="tf-invoice"
            type="text"
            value={invoice}
            onChange={(e) => setInvoice(e.target.value)}
            className="input"
            placeholder="INV-2026-…"
          />
        </div>

        {result && !result.ok && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {result.error}
          </div>
        )}
        {result?.ok && (
          <div role="status" className="animate-rise rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800">
            {result.replayed ? 'Replayed (no duplicate charge)' : mode === 'cross' ? 'Cross-border payment submitted' : 'Transfer submitted'} —{' '}
            <span className="font-mono">{result.transfer?.id.slice(0, 13)}…</span>
          </div>
        )}

        <div className="flex items-center justify-between border-t border-slate-100 pt-4">
          {preview ? (
            <div className="text-sm">
              <div className="text-slate-500">{preview.label}</div>
              <div className="font-mono text-lg font-bold text-navy-900">{preview.value}</div>
            </div>
          ) : (
            <span className="text-sm text-slate-400">Enter an amount to preview</span>
          )}
          <Button
            type="submit"
            loading={busy}
            disabled={
              busy ||
              !validAmount ||
              !source ||
              (mode === 'local' ? !dest : !counterpartyWallet || !canCross)
            }
          >
            {mode === 'cross' ? 'Send cross-border payment' : 'Send payment'}
          </Button>
        </div>
      </form>
    </Card>
  )
}