// useLiveStatus — shared SSE connection to the Go engine (/api/events).
//
// A single module-level EventSource is shared across every consumer (the
// header indicator, dashboard, invoices), so we never open duplicate streams.
// Subscribers register callbacks for both transfer events AND connection
// state changes, so the header Live/Offline badge stays accurate.
import { useEffect, useRef, useState } from 'react'
import type { TransferEvent } from '../api/types'

type Listener = (ev: TransferEvent | null) => void

let es: EventSource | null = null
let isConnected = false
let lastEvent: TransferEvent | null = null
const listeners = new Set<Listener>()

function notify() {
  for (const l of listeners) {
    try {
      l(lastEvent)
    } catch {
      /* a listener must not break the loop */
    }
  }
}

function ensureConnected() {
  if (es) return
  es = new EventSource('/api/events')

  es.onopen = () => {
    isConnected = true
    notify()
  }
  es.onerror = () => {
    // EventSource auto-reconnects; reflect the current state to the UI.
    isConnected = es?.readyState === EventSource.OPEN
    notify()
  }
  es.addEventListener('transfer', (msg: MessageEvent<string>) => {
    try {
      lastEvent = JSON.parse(msg.data) as TransferEvent
      notify()
    } catch {
      /* ignore malformed frames */
    }
  })
}

export function useLiveStatus(onEvent?: (ev: TransferEvent) => void): {
  connected: boolean
  lastEvent: TransferEvent | null
} {
  const [, force] = useState(0)
  const cbRef = useRef(onEvent)
  cbRef.current = onEvent

  useEffect(() => {
    ensureConnected()
    const listener: Listener = (ev) => {
      // Re-render this component so `connected` is fresh on state changes.
      force((n) => n + 1)
      if (ev) cbRef.current?.(ev)
    }
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  }, [])

  return { connected: isConnected, lastEvent }
}