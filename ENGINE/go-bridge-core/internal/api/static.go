package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// WithWebDir wraps the API handler so the built single-page dashboard
// (Vite `dist/`) is served from the same HTTP port. Requests for /api/*
// and /healthz are forwarded to the ledger API; everything else is served
// from webDir with index.html fallback so HashRouter deep links resolve.
//
// Optional extra pages (e.g. "/deck" -> "deck.html") can be registered in
// extraPages to expose static pitch material on the same origin.
func WithWebDir(api http.Handler, webDir string) http.Handler {
	fs := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API + liveness stay on the ledger handlers.
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			api.ServeHTTP(w, r)
			return
		}

		// Named extra pages (extension-less) resolve before the SPA fallback.
		switch r.URL.Path {
		case "/deck":
			http.ServeFile(w, r, filepath.Join(webDir, "deck.html"))
			return
		}

		// Serve real static assets as-is.
		clean := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if clean == "" {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		if info, err := os.Stat(filepath.Join(webDir, clean)); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}

		// SPA fallback: unknown paths load the app shell.
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
}