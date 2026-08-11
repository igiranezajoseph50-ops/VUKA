// TraderSelect — dedicated workspace-access page (route: /select).
//
// Layout follows the Select-Profile blueprint (FRONTEND/profile.png): a light
// shell with sidebar, breadcrumb, searchable enterprise table and an
// "Add profile" panel. Content is VUKA's real seeded enterprises and colors
// are the VUKA navy/emerald system — no placeholder companies or fees.
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { useTrader } from '../state/TraderContext'
import { useToasts } from '../components/ui/ToastProvider'
import BrandLogo from '../components/BrandLogo'

interface DemoTrader {
  name: string
  phone: string
  currency: string
  businessReg: string
  address: string
  category: string
  role: string
}

const DEMO_TRADERS: DemoTrader[] = [
  {
    name: 'Amina Uwera',
    phone: '+250700000991',
    currency: 'RWF',
    businessReg: 'RWC-2026-0441',
    address: 'Kigali, Rwanda',
    category: 'Importer',
    role: 'Kigali Coffee Coop',
  },
  {
    name: 'Kethan Gasana',
    phone: '+254700000882',
    currency: 'KES',
    businessReg: 'RWC-2026-0772',
    address: 'Nairobi, Kenya',
    category: 'Supplier',
    role: 'Nairobi Roasters Ltd',
  },
  {
    name: 'Jean-Paul Niyonzima',
    phone: '+250700000773',
    currency: 'RWF',
    businessReg: 'RWC-2026-0108',
    address: 'Musanze, Rwanda',
    category: 'Exporter',
    role: 'Musanze Minerals',
  },
]

const SIDEBAR = [
  { label: 'Home', to: '/', active: false },
  { label: 'Select Profile', to: '/select', active: true },
]

function initials(name: string): string {
  return name
    .split(' ')
    .map((s) => s[0])
    .slice(0, 2)
    .join('')
}

export default function TraderSelect() {
  const { select } = useTrader()
  const toasts = useToasts()
  const [busy, setBusy] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [showAdd, setShowAdd] = useState(false)

  async function enter(demo: DemoTrader) {
    setBusy(demo.phone)
    try {
      // Idempotent sign-in: resolve the seeded trader by phone first; only
      // create when the demo trader genuinely does not exist yet.
      const existing = await api.getUserByPhone(demo.phone)
      select(existing)
      toasts.success('Signed in', demo.name)
    } catch {
      try {
        const created = await api.createUser(
          {
            full_name: demo.name,
            phone_number: demo.phone,
            business_reg_number: demo.businessReg,
          },
          demo.currency,
        )
        select(created.user)
        toasts.success('Signed in', demo.name)
      } catch {
        toasts.error('Backend unreachable', 'Start the engine, then reload to sign in.')
      }
    } finally {
      setBusy(null)
    }
  }

  const q = query.trim().toLowerCase()
  const filteredDemos = DEMO_TRADERS.filter((d) => {
    if (!q) return true
    return [d.name, d.role, d.address, d.category, d.businessReg, d.phone, d.currency]
      .join(' ')
      .toLowerCase()
      .includes(q)
  })

  return (
    <div className="min-h-screen bg-slate-50 text-slate-950">
      <div className="flex min-h-screen">
        {/* ── Sidebar (blueprint: left rail nav) ── */}
        <aside className="hidden w-64 shrink-0 flex-col border-r border-slate-200 bg-white px-4 py-6 lg:flex">
          <div className="px-2">
            <BrandLogo size={36} wordmark="VUKA" sublabel="Trade payments" />
          </div>
          <nav className="mt-8 space-y-1">
            {SIDEBAR.map((item) => (
              <Link
                key={item.label}
                to={item.to}
                className={`block rounded-xl px-3 py-2.5 text-sm font-semibold transition ${
                  item.active
                    ? 'bg-emerald-50 text-emerald-700'
                    : 'text-slate-500 hover:bg-slate-50 hover:text-slate-800'
                }`}
              >
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="mt-auto rounded-2xl border border-slate-200 bg-slate-50 p-4">
            <p className="text-xs font-bold uppercase tracking-wider text-slate-400">Corridor</p>
            <p className="mt-1 font-mono text-sm font-bold text-slate-800">RWF → KES</p>
            <p className="mt-1 text-[11px] text-slate-500">Rwanda ↔ Kenya only</p>
          </div>
        </aside>

        {/* ── Main column ── */}
        <main className="min-w-0 flex-1 px-5 py-6 sm:px-8 lg:px-10">
          {/* Breadcrumb */}
          <nav className="flex items-center gap-2 text-sm text-slate-500" aria-label="Breadcrumb">
            <Link to="/" className="transition hover:text-emerald-700">Home</Link>
            <span aria-hidden="true">/</span>
            <span className="font-semibold text-slate-800">Select Profile</span>
          </nav>

          {/* Heading + search */}
          <div className="mt-6 flex flex-wrap items-end justify-between gap-4">
            <div>
              <h1 className="text-2xl font-black tracking-tight text-slate-950">Enterprise Profiles</h1>
              <p className="mt-2 text-sm text-slate-500">
                Choose the enterprise you want to transact with.
              </p>
            </div>
            <label className="block w-full max-w-sm">
              <span className="text-[10px] font-bold uppercase tracking-[0.2em] text-slate-400">Search</span>
              <input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search by name, reg number, phone…"
                className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 outline-none transition focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200"
              />
            </label>
          </div>

          {/* Enterprise table */}
          <div className="mt-6 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-slate-200 bg-slate-50 text-[11px] uppercase tracking-wider text-slate-400">
                    <th className="px-5 py-3 font-bold">Enterprise</th>
                    <th className="px-5 py-3 font-bold">Account</th>
                    <th className="hidden px-5 py-3 font-bold md:table-cell">Address</th>
                    <th className="hidden px-5 py-3 font-bold md:table-cell">Category</th>
                    <th className="hidden px-5 py-3 font-bold lg:table-cell">Ledger</th>
                    <th className="hidden px-5 py-3 font-bold lg:table-cell">Phone</th>
                    <th className="px-5 py-3" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {filteredDemos.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-5 py-10 text-center text-sm text-slate-500">
                        No enterprise matches “{query}”. Add a new profile below, or check the phone number.
                      </td>
                    </tr>
                  ) : (
                    filteredDemos.map((demo) => (
                      <tr
                        key={demo.phone}
                        onClick={() => enter(demo)}
                        className={`cursor-pointer transition hover:bg-emerald-50/60 ${busy ? 'opacity-60' : ''}`}
                      >
                        <td className="px-5 py-4">
                          <div className="flex items-center gap-3">
                            <div className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-navy-950 text-xs font-black text-white">
                              {initials(demo.name)}
                            </div>
                            <div className="min-w-0">
                              <p className="truncate font-bold text-slate-950">{demo.role}</p>
                              <p className="text-xs text-slate-500">{demo.name}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-5 py-4 font-mono text-xs text-slate-600">{demo.businessReg}</td>
                        <td className="hidden px-5 py-4 text-slate-600 md:table-cell">{demo.address}</td>
                        <td className="hidden px-5 py-4 text-slate-600 md:table-cell">{demo.category}</td>
                        <td className="hidden px-5 py-4 lg:table-cell">
                          <span className="rounded-full bg-slate-100 px-2.5 py-1 text-[11px] font-bold text-slate-600">
                            {demo.currency}
                          </span>
                        </td>
                        <td className="hidden px-5 py-4 font-mono text-xs text-slate-500 lg:table-cell">{demo.phone}</td>
                        <td className="px-5 py-4 text-right">
                          <span className="inline-flex items-center gap-1 rounded-xl bg-emerald-500 px-3 py-1.5 text-xs font-bold text-white">
                            Enter
                          </span>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Add profile panel */}
          <div className="mt-6 grid gap-4 lg:grid-cols-[1.2fr_0.8fr]">
            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              <p className="text-sm font-bold text-slate-950">Need a new profile?</p>
              <p className="mt-1 text-sm text-slate-500">
                Add an enterprise to start transacting on the corridor. The engine creates it with
                separate business and personal ledgers in the chosen currency.
              </p>
              <button
                onClick={() => setShowAdd((v) => !v)}
                className="mt-4 rounded-xl bg-navy-950 px-5 py-2.5 text-sm font-bold text-white shadow-sm transition hover:bg-navy-900"
              >
                {showAdd ? 'Cancel' : '+ Add Profile'}
              </button>

              {showAdd && <AddProfileForm />}
            </div>

            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
              {busy ? (
                <p className="text-center text-sm font-semibold text-slate-600">Opening workspace…</p>
              ) : (
                <>
                  <p className="text-sm font-bold text-slate-950">Live corridor</p>
                  <div className="mt-3 space-y-2 text-xs text-slate-500">
                    <p className="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-2">
                      <span>Corridor</span><span className="font-mono font-bold text-slate-800">RWF → KES</span>
                    </p>
                    <p className="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-2">
                      <span>Rails</span><span className="font-mono font-bold text-slate-800">MTN → M-Pesa</span>
                    </p>
                    <p className="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-2">
                      <span>Ledger</span><span className="font-mono font-bold text-slate-800">Double-entry</span>
                    </p>
                  </div>
                </>
              )}
            </div>
          </div>
        </main>
      </div>
    </div>
  )
}

/* ── Add Profile: real engine registration (POST /api/users) ─────────── */

function AddProfileForm() {
  const { select } = useTrader()
  const toasts = useToasts()
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [reg, setReg] = useState('')
  const [currency, setCurrency] = useState('RWF')
  const [submitting, setSubmitting] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !phone.trim()) {
      toasts.error('Missing fields', 'Business name and phone are required.')
      return
    }
    setSubmitting(true)
    try {
      const created = await api.createUser(
        {
          full_name: name.trim(),
          phone_number: phone.trim(),
          business_reg_number: reg.trim() || undefined,
        },
        currency,
      )
      select(created.user)
      toasts.success('Profile created', name.trim())
    } catch (err) {
      toasts.error('Could not create', (err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={submit} className="mt-5 space-y-3 rounded-2xl border border-slate-200 bg-slate-50 p-4">
      <label className="block">
        <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">Business name</span>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          placeholder="e.g. Kigali Coffee Coop"
          className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200"
        />
      </label>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block">
          <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">Phone</span>
          <input
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            required
            placeholder="+2507…"
            className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200"
          />
        </label>
        <label className="block">
          <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">Ledger currency</span>
          <select
            value={currency}
            onChange={(e) => setCurrency(e.target.value)}
            className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200"
          >
            <option value="RWF">RWF</option>
            <option value="KES">KES</option>
          </select>
        </label>
      </div>
      <label className="block">
        <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">Business reg number (optional)</span>
        <input
          value={reg}
          onChange={(e) => setReg(e.target.value)}
          placeholder="RWC-2026-…"
          className="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2.5 text-sm outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-200"
        />
      </label>
      <button
        type="submit"
        disabled={submitting}
        className="w-full rounded-xl bg-emerald-500 px-5 py-2.5 text-sm font-bold text-white shadow-sm transition hover:bg-emerald-400 disabled:opacity-60"
      >
        {submitting ? 'Creating…' : 'Create profile & enter'}
      </button>
    </form>
  )
}