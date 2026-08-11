// ToastProvider — global, enterprise-grade notification system.
//
// Renders a stack of dismissible toasts (success/error/info) in the top-right.
// Any component can fire one via useToasts(). Auto-dismisses, supports a
// manual close, and animates in/out. Powered by React context + a small id
// counter; no external dependency.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'
import type { ReactNode } from 'react'
import { Badge } from './primitives'

type ToastTone = 'success' | 'error' | 'info'

interface Toast {
  id: number
  tone: ToastTone
  title: string
  detail?: string
}

interface ToastContextValue {
  push: (tone: ToastTone, title: string, detail?: string) => void
  success: (title: string, detail?: string) => void
  error: (title: string, detail?: string) => void
  info: (title: string, detail?: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

let nextId = 1
const DEFAULT_DURATION = 6000

const TONE_LABEL: Record<ToastTone, string> = {
  success: 'Success',
  error: 'Error',
  info: 'Note',
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const push = useCallback((tone: ToastTone, title: string, detail?: string) => {
    const id = nextId++
    setToasts((prev) => [...prev, { id, tone, title, detail }])
    // Auto-dismiss unless it's an unhandled error (keep those until closed).
    window.setTimeout(() => dismiss(id), DEFAULT_DURATION)
  }, [dismiss])

  const value = useMemo<ToastContextValue>(() => {
    return {
      push,
      success: (title, detail) => push('success', title, detail),
      error: (title, detail) => push('error', title, detail),
      info: (title, detail) => push('info', title, detail),
    }
  }, [push])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setToasts([])
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed right-4 top-4 z-50 flex w-80 flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            role={t.tone === 'error' ? 'alert' : 'status'}
            className="pointer-events-auto animate-rise rounded-xl border border-slate-200 bg-white p-4 shadow-lg"
          >
            <div className="flex items-start gap-3">
              <Badge tone={t.tone === 'info' ? 'info' : t.tone === 'error' ? 'danger' : 'success'}>
                {TONE_LABEL[t.tone]}
              </Badge>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-navy-900">{t.title}</div>
                {t.detail && <div className="mt-0.5 text-xs text-slate-500">{t.detail}</div>}
              </div>
              <button
                onClick={() => dismiss(t.id)}
                aria-label="Dismiss"
                className="shrink-0 text-slate-400 transition-colors hover:text-slate-600"
              >
                ✕
              </button>
            </div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToasts(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToasts must be used within a ToastProvider')
  return ctx
}