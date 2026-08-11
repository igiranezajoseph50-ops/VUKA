// Landing — public page for VUKA (#/). Enterprise-grade front door.
//
// Tone: restrained, regulatory-adjacent, B2B. No taglines, no pills, no
// decorative arrows. Structure follows the approved 6-section blueprint
// (hero, solutions, corridor, developers, platform, footer) and every
// in-page "link" is a scroll button — the app runs on HashRouter, where a
// plain <a href="#..."> resolves to a route and bounces the visitor to
// /select. Scroll navigation is programmatic so the page never leaves itself.
//
// Colors use Tailwind's static navy/emerald utilities only (no template
// literals — Tailwind cannot scan those).

import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import BrandLogo from '../components/BrandLogo'
import CorridorScene from '../components/CorridorScene'

interface FxState {
  rate: number
  source: string
  updated: string
}

const NAV_SECTIONS = [
  { label: 'Solutions', id: 'solutions' },
  { label: 'Corridor', id: 'corridor' },
  { label: 'Architecture', id: 'devs' },
  { label: 'Position', id: 'platform' },
]

/** HashRouter-safe in-page navigation: scroll, never change the route. */
function scrollToSection(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const SOLUTIONS = [
  {
    no: '01',
    title: 'Invoice-linked settlement',
    body: 'Transfers are initiated against an order or invoice. Both parties hold a timestamped record of the commercial obligation being settled.',
  },
  {
    no: '02',
    title: 'Segregated business ledger',
    body: 'Business and personal funds are kept in separate accounts, enforced at the engine layer — the basis for bookkeeping and credit assessment.',
  },
  {
    no: '03',
    title: 'Trade-scale operation',
    body: 'Limits, compliance and controls are configured for business payments rather than personal-remittance ceilings.',
  },
  {
    no: '04',
    title: 'Verifiable payment history',
    body: "A trader's settlement record is verifiable before terms are agreed, reducing reliance on large upfront deposits.",
  },
  {
    no: '05',
    title: 'Visible settlement interface',
    body: 'The exchange rate is presented before commitment. Confirmation, history and status are traceable from initiation to settlement.',
  },
]

const STACK = [
  {
    tier: 'L4',
    name: 'Double-entry ledger',
    tech: 'PostgreSQL',
    role: 'Strict double-entry accounting. Every transfer nets to zero; balances are derived from entries, never stored.',
  },
  {
    tier: 'L3',
    name: 'Transfer engine',
    tech: 'Go · REST :8080 · gRPC :50051',
    role: 'Money-movement logic with serialised concurrency — simultaneous transfers cannot corrupt a balance.',
  },
  {
    tier: 'L2',
    name: 'Trader dashboard',
    tech: 'React',
    role: 'Live operator interface covering confirmation, history and status for the corridor.',
  },
  {
    tier: 'L1',
    name: 'Messaging standard',
    tech: 'ISO 20022',
    role: 'Standard message format eliminating the mismatches that cause failed transfers.',
  },
]

const REGULATORY = [
  {
    label: 'Passporting',
    body: 'Active fintech passporting agreements (Ghana, Feb 2025; Kenya, Mar 2026) open the legal path for cross-border licensing.',
  },
  {
    label: 'ISO 20022',
    body: 'The global messaging-standard transition deadline has passed. Rwanda has the opening to lead African adoption.',
  },
  {
    label: 'Corridor scope',
    body: 'Rwanda ↔ Kenya (RWF ↔ KES) on MTN MoMo and M-Pesa, structured to extend to Uganda and Tanzania.',
  },
]

const ASKS = [
  'Guidance on the BNR fintech sandbox application process.',
  'Introduction to telecom or aggregator partners for a pilot-corridor integration.',
  'Regulatory feedback on invoice-linked, business-focused products on mobile-money rails.',
]

export default function Landing() {
  const [fx, setFx] = useState<FxState | null>(null)
  const [fxError, setFxError] = useState(false)

  useEffect(() => {
    let cancelled = false
    api
      .getFxRate()
      .then((res) => {
        if (!cancelled) setFx(res)
      })
      .catch(() => {
        if (!cancelled) setFxError(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const rate = fx && !fxError ? fx.rate : null
  const debitAmount = 101500 // RWF — sample invoice INV-2026-00891
  const creditAmount = rate ? Math.round((debitAmount / rate) * 100) / 100 : null

  return (
    <main className="min-h-screen bg-slate-50 text-slate-900">
      {/* ═══════════════════ HEADER ═══════════════════ */}
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-6 px-6 py-4">
          <BrandLogo size={34} wordmark="VUKA" sublabel="Cross-border trade payments" />
          <nav className="hidden items-center gap-8 text-sm font-medium text-slate-600 lg:flex">
            {NAV_SECTIONS.map((s) => (
              <button key={s.id} onClick={() => scrollToSection(s.id)} className="transition hover:text-navy-900">
                {s.label}
              </button>
            ))}
          </nav>
          <div className="flex items-center gap-3">
            <Link
              to="/select"
              className="rounded-md bg-navy-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-navy-800"
            >
              Access platform
            </Link>
          </div>
        </div>
      </header>

      {/* ═══════════════════ HERO ═══════════════════ */}
      <section className="bg-navy-950 text-white">
        <div className="mx-auto grid max-w-6xl items-center gap-14 px-6 py-20 lg:grid-cols-[1.05fr_0.95fr] lg:py-28">
          <div>
            <p className="text-xs font-medium uppercase tracking-[0.22em] text-white/50">
              National Bank of Rwanda · Move money that moves life
            </p>
            <h1 className="mt-6 max-w-xl text-4xl font-bold leading-[1.08] tracking-tight sm:text-5xl">
              Invoice-linked settlement infrastructure for cross-border trade.
            </h1>
            <p className="mt-7 max-w-xl text-lg leading-8 text-white/70">
              VUKA operates on the mobile-money rails that already connect Rwanda and Kenya, adding the layer
              trade requires: invoice-linked payments, segregated business ledgers, and a verifiable
              settlement history.
            </p>

            <div className="mt-10 flex flex-wrap items-center gap-4">
              <Link
                to="/select"
                className="rounded-md bg-emerald-600 px-6 py-3 text-sm font-semibold text-white transition hover:bg-emerald-500"
              >
                Access platform
              </Link>
              <button
                onClick={() => scrollToSection('corridor')}
                className="border border-white/25 px-6 py-3 text-sm font-semibold text-white transition hover:bg-white/5"
              >
                View the corridor
              </button>
            </div>
          </div>

          {/* Live ledger terminal */}
          <div className="border border-white/10 bg-navy-900">
            <div className="flex items-center justify-between border-b border-white/10 px-5 py-3">
              <p className="font-mono text-xs tracking-[0.14em] text-white/50">DEMONSTRATION LEDGER · INV-2026-00891</p>
              <span className={`font-mono text-[11px] ${rate ? 'text-emerald-400' : 'text-white/30'}`}>
                {rate ? '● LIVE' : '○ OFFLINE'}
              </span>
            </div>
            <div className="px-5 py-4">
              <table className="w-full text-sm">
                <tbody>
                  <tr className="border-b border-white/10">
                    <td className="py-3 text-white/50">Debit · RWF</td>
                    <td className="py-3 text-right font-mono text-white">{debitAmount.toLocaleString()}</td>
                  </tr>
                  <tr className="border-b border-white/10">
                    <td className="py-3 text-white/50">Credit · KES</td>
                    <td className="py-3 text-right font-mono text-white">
                      {creditAmount !== null ? creditAmount.toLocaleString() : '—'}
                    </td>
                  </tr>
                  <tr className="border-b border-white/10">
                    <td className="py-3 text-white/50">Rate</td>
                    <td className="py-3 text-right font-mono text-white">{rate ? rate.toFixed(2) : '—'}</td>
                  </tr>
                  <tr>
                    <td className="py-3 text-white/50">Settlement</td>
                    <td className="py-3 text-right font-mono text-emerald-400">0.00 · balanced</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div className="border-t border-white/10 px-5 py-3 font-mono text-[11px] text-white/40">
              Double-entry · balances derived from ledger entries
            </div>
          </div>
        </div>
      </section>

      {/* ═══════════════════ SOLUTIONS ═══════════════════ */}
      <section id="solutions" className="bg-white py-20">
        <div className="mx-auto max-w-6xl px-6">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-emerald-700">Solutions</p>
          <h2 className="mt-4 max-w-2xl text-3xl font-bold tracking-tight text-navy-900 sm:text-4xl">
            What the platform provides
          </h2>

          <div className="mt-12 divide-y divide-slate-200 border-y border-slate-200">
            {SOLUTIONS.map((f) => (
              <div key={f.no} className="grid gap-2 py-6 md:grid-cols-[64px_240px_1fr] md:gap-8">
                <span className="font-mono text-sm text-slate-400">{f.no}</span>
                <h3 className="text-base font-semibold text-navy-900">{f.title}</h3>
                <p className="max-w-2xl text-sm leading-6 text-slate-600">{f.body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ═══════════════════ CORRIDOR ═══════════════════ */}
      <section id="corridor" className="bg-slate-50 py-20">
        <div className="mx-auto grid max-w-6xl items-start gap-14 px-6 lg:grid-cols-[1fr_1fr]">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-emerald-700">Corridor</p>
            <h2 className="mt-4 max-w-xl text-3xl font-bold tracking-tight text-navy-900 sm:text-4xl">
              Rwanda ↔ Kenya. RWF in, KES out, one balanced ledger.
            </h2>
            <p className="mt-6 max-w-xl text-base leading-7 text-slate-600">
              VUKA operates on the rails that already connect the two markets — MTN MoMo on the Rwanda side
              and M-Pesa on the Kenya side — and provides the trade layer between them: a rate presented
              before commitment, an invoice anchoring the payment, and settlement that nets to zero.
            </p>
            <dl className="mt-8 divide-y divide-slate-200 border-y border-slate-200">
              <div className="grid grid-cols-[140px_1fr] gap-4 py-3">
                <dt className="text-sm font-semibold text-navy-900">Live rate</dt>
                <dd className="text-sm text-slate-600">{rate ? `${rate.toFixed(2)} RWF per KES` : 'Engine offline'}</dd>
              </div>
              <div className="grid grid-cols-[140px_1fr] gap-4 py-3">
                <dt className="text-sm font-semibold text-navy-900">Rails</dt>
                <dd className="text-sm text-slate-600">MTN MoMo → M-Pesa</dd>
              </div>
              <div className="grid grid-cols-[140px_1fr] gap-4 py-3">
                <dt className="text-sm font-semibold text-navy-900">Ledger</dt>
                <dd className="text-sm text-slate-600">Double-entry · balances derived from entries</dd>
              </div>
            </dl>
          </div>

          <div className="border border-slate-200 bg-white">
            <div className="border-b border-slate-200 px-5 py-3">
              <p className="font-mono text-xs tracking-[0.14em] text-slate-400">COVERAGE</p>
            </div>
            <div className="p-5">
              <CorridorScene />
            </div>
            <div className="grid grid-cols-2 divide-x divide-slate-200 border-t border-slate-200">
              <div className="px-5 py-3">
                <p className="text-sm font-semibold text-navy-900">Rwanda</p>
                <p className="font-mono text-xs text-slate-500">RWF · MTN MoMo</p>
              </div>
              <div className="px-5 py-3">
                <p className="text-sm font-semibold text-navy-900">Kenya</p>
                <p className="font-mono text-xs text-slate-500">KES · M-Pesa</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ═══════════════════ ARCHITECTURE ═══════════════════ */}
      <section id="devs" className="bg-navy-950 py-20 text-white">
        <div className="mx-auto max-w-6xl px-6">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-emerald-400">Architecture</p>
          <h2 className="mt-4 max-w-xl text-3xl font-bold tracking-tight sm:text-4xl">
            One rule: every transfer nets to zero.
          </h2>
          <p className="mt-6 max-w-2xl text-base leading-7 text-white/60">
            VUKA is built on existing telecom infrastructure rather than competing with it — a stack designed
            for the reliability required to move commercial value.
          </p>

          <div className="mt-12 divide-y divide-white/10 border-y border-white/10">
            {STACK.map((l) => (
              <div key={l.tier} className="grid items-start gap-3 py-5 md:grid-cols-[64px_260px_1fr] md:gap-8">
                <span className="font-mono text-sm text-emerald-400">{l.tier}</span>
                <div>
                  <p className="text-sm font-semibold text-white">{l.name}</p>
                  <p className="mt-0.5 font-mono text-xs text-white/40">{l.tech}</p>
                </div>
                <p className="text-sm leading-6 text-white/60">{l.role}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ═══════════════════ POSITION ═══════════════════ */}
      <section id="platform" className="bg-white py-20">
        <div className="mx-auto max-w-6xl px-6">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-emerald-700">Position</p>
          <h2 className="mt-4 max-w-2xl text-3xl font-bold tracking-tight text-navy-900 sm:text-4xl">
            Infrastructure on top of the rails that exist.
          </h2>
          <p className="mt-6 max-w-2xl text-base leading-7 text-slate-600">
            Onafriq and PAPSS operate continent-scale backend infrastructure. Direct telecom corridors move
            value without commercial context. VUKA is the trade-specific layer neither provides.
          </p>

          <div className="mt-10 grid gap-px overflow-hidden border border-slate-200 bg-slate-200 sm:grid-cols-3">
            {REGULATORY.map((c) => (
              <div key={c.label} className="bg-white p-6">
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">{c.label}</p>
                <p className="mt-3 text-sm leading-6 text-slate-600">{c.body}</p>
              </div>
            ))}
          </div>

          <div className="mt-12 border border-slate-200">
            <div className="border-b border-slate-200 bg-slate-50 px-6 py-3">
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
                Regulatory & partnership requirement
              </p>
            </div>
            <ol className="divide-y divide-slate-200">
              {ASKS.map((a, i) => (
                <li key={a} className="grid grid-cols-[48px_1fr] gap-4 px-6 py-4">
                  <span className="font-mono text-sm text-slate-400">{String(i + 1).padStart(2, '0')}</span>
                  <span className="text-sm leading-6 text-slate-700">{a}</span>
                </li>
              ))}
            </ol>
          </div>
        </div>
      </section>

      {/* ═══════════════════ FOOTER ═══════════════════ */}
      <footer className="border-t border-slate-200 bg-slate-50">
        <div className="mx-auto max-w-6xl px-6 py-12">
          <div className="flex flex-wrap items-start justify-between gap-10">
            <div className="max-w-sm">
              <BrandLogo size={32} wordmark="VUKA" sublabel="Cross-border trade payments" />
              <p className="mt-5 text-sm leading-6 text-slate-600">
                A settlement layer for East African SME trade on existing mobile-money rails. Submitted to the
                National Bank of Rwanda FinTech Innovation Hackathon 2026.
              </p>
            </div>
            <div className="flex gap-16">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">Product</p>
                <ul className="mt-4 space-y-2.5 text-sm text-slate-600">
                  <li>
                    <button onClick={() => scrollToSection('solutions')} className="transition hover:text-navy-900">
                      Solutions
                    </button>
                  </li>
                  <li>
                    <button onClick={() => scrollToSection('corridor')} className="transition hover:text-navy-900">
                      Corridor
                    </button>
                  </li>
                  <li>
                    <button onClick={() => scrollToSection('devs')} className="transition hover:text-navy-900">
                      Architecture
                    </button>
                  </li>
                  <li>
                    <Link to="/select" className="transition hover:text-navy-900">
                      Access platform
                    </Link>
                  </li>
                </ul>
              </div>
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">Corridor</p>
                <ul className="mt-4 space-y-2.5 text-sm text-slate-600">
                  <li>Kigali, Rwanda</li>
                  <li>Nairobi, Kenya</li>
                </ul>
              </div>
            </div>
          </div>
          <div className="mt-10 flex flex-wrap items-center gap-x-8 gap-y-3 border-t border-slate-200 pt-6 font-mono text-xs text-slate-400">
            <span>Prototype · demonstration environment · no live funds</span>
            <span>BNR FinTech Innovation Hackathon 2026</span>
          </div>
        </div>
      </footer>
    </main>
  )
}