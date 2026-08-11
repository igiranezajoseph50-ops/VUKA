// CORS middleware (Phase 3 — trader dashboard).
//
// The React dashboard runs on the Vite dev origin (http://localhost:5173) and
// talks to the Go engine on :8080. That is a cross-origin request, so the
// engine must answer preflight (OPTIONS) and attach the Access-Control-Allow-*
// headers. Allowed origins are configurable; the default is the Vite dev
// server. In production this list should be tightened to the real UI origin.
package api

import (
	"net/http"
	"strings"
)

// NewCORS wraps a handler with permissive-but-configurable CORS headers.
func NewCORS(origins []string, next http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Attach allow headers when the origin is allowed (or all origins
		// when the allow-list is empty, i.e. CORS disabled/fully open).
		permit := len(allowed) == 0 || allowed[origin]
		if permit && origin != "" {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, Accept")
			h.Set("Access-Control-Max-Age", "600")
		}

		// Preflight: respond 204 with no body and stop.
		if r.Method == http.MethodOptions {
			if permit {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}