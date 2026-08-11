// App — enterprise layout shell: fixed sidebar, header bar, and route mount.
// The VUKA brand system (navy + emerald, paper grid) is applied globally via
// index.css; this file owns structure only. Trader identity and compliance
// status come from the live engine user record, never hardcoded strings.
import { NavLink, Navigate, Route, Routes } from 'react-router-dom'
import { useTrader } from './state/TraderContext'
import Landing from './pages/Landing'
import TraderSelect from './pages/TraderSelect'
import Dashboard from './pages/Dashboard'
import Transfers from './pages/Transfers'
import Invoices from './pages/Invoices'
import History from './pages/History'
import OperationsModule from './pages/OperationsModule'
import { useLiveStatus } from './hooks/useLiveStatus'
import BrandLogo from './components/BrandLogo'

interface NavItem {
  to: string
  label: string
  icon: string
}

const NAV: NavItem[] = [
  { to: '/dashboard', label: 'Dashboard', icon: '▦' },
  { to: '/transfers', label: 'Payments', icon: '⇄' },
  { to: '/invoices', label: 'Invoices', icon: '▤' },
  { to: '/wallet', label: 'Business Wallet', icon: '▣' },
  { to: '/history', label: 'Transaction History', icon: '↺' },
  { to: '/exchange-rates', label: 'Exchange Rates', icon: '◎' },
  { to: '/analytics', label: 'Analytics', icon: '▥' },
  { to: '/notifications', label: 'Notifications', icon: '♧' },
]

/** Initials from the trader's real full name, e.g. "Kigali Coffee Coop" -> "KC". */
function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('') || '?'
}

/** Country + currency derived from the trader's real phone prefix. */
function countryOf(phone: string): { flag: string; code: string; currency: string } {
  if (phone.startsWith('+254')) return { flag: '🇰🇪', code: 'Kenya', currency: 'KES' }
  return { flag: '🇷🇼', code: 'Rwanda', currency: 'RWF' }
}

function Sidebar({ onNavigate }: { onNavigate: () => void }) {
  const { trader, clear } = useTrader()

  return (
    <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 shrink-0 flex-col bg-navy-950 text-white lg:flex">
      {/* Brand block */}
      <div className="px-5 py-5">
        <BrandLogo size={40} wordmark="VUKA" sublabel="Trade payments" />
      </div>

      {/* Primary navigation */}
      <nav className="flex flex-1 flex-col gap-1 px-3">
        <div className="px-2 pb-3 pt-7 text-[10px] font-bold uppercase tracking-[0.22em] text-slate-500">Operations</div>
        {NAV.map((item) => (
          <NavLink
            key={`${item.to}-${item.label}`}
            to={item.to}
            onClick={onNavigate}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-white/10 text-white shadow-inner'
                  : 'text-slate-400 hover:bg-white/5 hover:text-white'
              }`
            }
          >
            <span className="w-5 text-center text-slate-400">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
      </nav>

      {/* Compliance card driven by the live user record */}
      {trader && (
        <div className="m-3 rounded-2xl border border-white/10 bg-white/5 p-4">
          <div className="flex items-center gap-2 text-sm font-bold">
            <span className="grid h-6 w-6 place-items-center rounded-full bg-emerald-500/15 text-emerald-400">◇</span>
            {trader.kyc_status === 'VERIFIED' ? 'Compliance verified' : 'Compliance pending'}
          </div>
          <p className="mt-3 text-xs leading-5 text-slate-400">
            KYC {trader.kyc_status}
            {trader.business_reg_number ? ` · Reg ${trader.business_reg_number}` : ''}
          </p>
        </div>
      )}

      {trader && (
        <div className="border-t border-white/10 p-4">
          <div className="flex items-center gap-3">
            <div className="grid h-9 w-9 place-items-center rounded-full bg-white/10 text-sm font-bold">{initialsOf(trader.full_name)}</div>
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-semibold">{trader.full_name}</div>
              <div className="truncate text-xs text-slate-400">{countryOf(trader.phone_number).code}</div>
            </div>
            <button onClick={clear} title="Switch trader" className="rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-white/10 hover:text-white">
              ⇄
            </button>
          </div>
        </div>
      )}
    </aside>
  )
}

function Header({ connected }: { connected: boolean }) {
  const { trader } = useTrader()

  return (
    <header className="sticky top-0 z-20 flex min-h-[68px] items-center justify-between gap-4 border-b border-slate-200 bg-white/95 px-5 backdrop-blur lg:px-8">
      <div className="min-w-0 flex-1">
        <NavLink
          to="/transfers"
          className="inline-flex items-center gap-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm font-semibold text-slate-600 transition-colors hover:border-emerald-200 hover:bg-emerald-50 hover:text-emerald-700"
        >
          <span>+</span> New Payment
        </NavLink>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {trader && (
          <>
            <span className="hidden rounded-2xl border border-slate-200 px-4 py-2.5 text-sm font-semibold md:block">
              {countryOf(trader.phone_number).flag} {countryOf(trader.phone_number).code}
            </span>
            <span className="hidden rounded-2xl border border-slate-200 px-4 py-2.5 text-sm font-semibold md:block">
              {countryOf(trader.phone_number).currency}
            </span>
          </>
        )}
        <span className={`grid h-10 w-10 place-items-center rounded-full border border-slate-200 ${connected ? 'text-emerald-600' : 'text-slate-400'}`} title={connected ? 'Live updates active' : 'Reconnecting'}>♧</span>
        {trader && (
          <div className="hidden items-center gap-3 rounded-2xl border border-slate-200 px-3 py-2 lg:flex">
            <div className="grid h-8 w-8 place-items-center rounded-full bg-navy-950 text-xs font-bold text-white">{initialsOf(trader.full_name)}</div>
            <div>
              <p className="text-xs font-bold text-slate-950">{trader.full_name}</p>
              <p className="text-xs text-slate-500">{countryOf(trader.phone_number).code}</p>
            </div>
          </div>
        )}
      </div>
    </header>
  )
}

function Shell() {
  const { connected } = useLiveStatus()

  return (
    <div className="min-h-screen bg-slate-50">
      <Sidebar onNavigate={() => undefined} />
      <div className="flex min-w-0 flex-1 flex-col lg:pl-64">
        <Header connected={connected} />
        <main className="paper flex-1 px-5 py-8 lg:px-8">
          <Routes>
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/transfers" element={<Transfers />} />
            <Route path="/invoices" element={<Invoices />} />
            <Route path="/history" element={<History />} />
            <Route path="/wallet" element={<OperationsModule module="wallet" />} />
            <Route path="/exchange-rates" element={<OperationsModule module="rates" />} />
            <Route path="/analytics" element={<OperationsModule module="analytics" />} />
            <Route path="/notifications" element={<OperationsModule module="notifications" />} />
            <Route path="*" element={<Navigate to="/dashboard" replace />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

export default function App() {
  const { trader } = useTrader()

  // Public routes before sign-in: the Landing page is marketing-only; the
  // workspace-access (user selection) page lives at /select. Anything else
  // redirects to the selection page so deep links still resolve.
  if (!trader) {
    return (
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/select" element={<TraderSelect />} />
        <Route path="*" element={<Navigate to="/select" replace />} />
      </Routes>
    )
  }

  return <Shell />
}
