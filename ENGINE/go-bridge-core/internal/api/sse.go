// SSE hub (Phase 3 — live trader dashboard updates).
//
// GET /api/events streams Server-Sent Events for transfer status changes:
//
//	event: transfer
//	data: {"id":"...","status":"SUCCESS","account_id":"...","amount":1000,"currency":"RWF"}
//
// The hub is in-memory and per-process, which is correct for the single
// hackathon instance. A multi-instance deployment would swap this for
// PostgreSQL LISTEN/NOTIFY; the handler contract stays identical.
package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// TransferEvent is the payload pushed to SSE subscribers on status change.
type TransferEvent struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	AccountID string  `json:"account_id,omitempty"`
	Amount    float64 `json:"amount,omitempty"`
	Currency  string  `json:"currency,omitempty"`
	UpdatedAt string  `json:"updated_at"`
}

// Hub fans status events out to connected SSE clients.
type Hub struct {
	mu     sync.Mutex
	subs   map[chan []byte]struct{}
	closed bool
}

// NewHub builds an empty SSE hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[chan []byte]struct{})}
}

// Publish serializes and broadcasts an event to all subscribers. It never
// blocks: slow consumers are dropped rather than stalling the ledger path.
func (h *Hub) Publish(ev TransferEvent) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	frame := append([]byte("event: transfer\ndata: "), payload...)
	frame = append(frame, '\n', '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- frame:
		default: // slow consumer: drop this event, keep the client
		}
	}
}

// Close terminates all subscriptions (used by tests / shutdown).
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.subs {
		close(ch)
	}
	h.subs = make(map[chan []byte]struct{})
}

// ServeHTTP implements the GET /api/events streaming endpoint.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "no_flusher", "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering

	// Keepalive comment so intermediaries don't idle-close the stream.
	if _, err := w.Write([]byte(": connected\n\n")); err != nil {
		return
	}
	flusher.Flush()

	ch := make(chan []byte, 16)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case frame, ok := <-ch:
			if !ok {
				return
			}
			if _, err := w.Write(frame); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}