// Package config loads runtime configuration from environment variables
// with sensible development defaults. VUKA follows the twelve-factor style:
// every tunable knob is an env var, never a hardcoded constant.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings for the VUKA Go bridge engine.
type Config struct {
	// HTTPAddr is the listen address for the REST API, e.g. ":8080".
	HTTPAddr string
	// GRPCAddr is the listen address for the gRPC bridge consumed by the
	// Python ISO 20022 gateway, e.g. ":50051".
	GRPCAddr string
	// DatabaseURL is the PostgreSQL DSN (pgx format), e.g.
	// "postgres://vuka:***@localhost:5432/vuka".
	DatabaseURL string

	// FxRateRWFKES is the engine-wide RWF->KES rate used when a cross-border
	// request omits its own fx_rate. Zero disables the default (requests must
	// then provide a rate).
	FxRateRWFKES float64

	// CORSOrigins is the allow-list of browser origins for the trader
	// dashboard, e.g. "http://localhost:5173". Comma-separated in the env.
	// Empty allows any origin (dev only).
	CORSOrigins []string

	// WebDir is an optional directory containing the built single-page
	// dashboard (Vite `dist/`). When set, the HTTP server serves the SPA
	// at "/" with index.html fallback so the whole product runs on one
	// port. Empty disables static serving (API-only mode).
	WebDir string

	// Adapter settings ----------------------------------------------------
	// AdapterName selects the telecom adapter from the registry, e.g. "mtn-rw".
	AdapterName string
	// MTNSimDelay is the simulated network latency of the MTN adapter.
	MTNSimDelay time.Duration
	// MTNSimFailRate is the probability (0.0-1.0) that a simulated payout fails.
	MTNSimFailRate float64
	// MTNSimMode forces a deterministic adapter behaviour:
	// "success", "fail", "timeout", or "" (random per fail rate).
	MTNSimMode string
	// MTNSimTimeout is how long the adapter waits before timing out.
	MTNSimTimeout time.Duration

	// Server timeouts ----------------------------------------------------
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Load reads configuration from the environment. Missing optional values fall
// back to development defaults; missing required values return an error so the
// process fails fast instead of starting half-configured.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:      getEnv("VUKA_HTTP_ADDR", ":8080"),
		GRPCAddr:      getEnv("VUKA_GRPC_ADDR", ":50051"),
		DatabaseURL:   os.Getenv("VUKA_DATABASE_URL"),
		FxRateRWFKES:  getFloat("VUKA_FX_RWF_KES", 0),
		CORSOrigins:   splitList("VUKA_CORS_ORIGINS"),
		WebDir:        getEnv("VUKA_WEB_DIR", ""),
		AdapterName:   getEnv("VUKA_ADAPTER", "mtn-rw"),
		MTNSimDelay:   getDuration("MTN_SIM_DELAY_MS", 150*time.Millisecond),
		MTNSimFailRate: getFloat("MTN_SIM_FAIL_RATE", 0.0),
		MTNSimMode:    getEnv("MTN_SIM_MODE", ""),
		MTNSimTimeout: getDuration("MTN_SIM_TIMEOUT_MS", 5*time.Second),
		ReadTimeout:   getDuration("VUKA_READ_TIMEOUT_S", 10*time.Second),
		WriteTimeout:  getDuration("VUKA_WRITE_TIMEOUT_S", 30*time.Second),
		IdleTimeout:   getDuration("VUKA_IDLE_TIMEOUT_S", 60*time.Second),
	}

	if cfg.DatabaseURL == "" {
			// Dev default matching deploy/docker-compose.yml (host port 5432).
			// Placeholder password only — real dev credentials come from the
			// environment (VUKA_DATABASE_URL) or ~/.pgpass; never commit a
			// working credential to the repository.
			cfg.DatabaseURL = "postgres://vuka:changeme@localhost:5432/vuka?sslmode=disable"
		}

	if cfg.MTNSimFailRate < 0 || cfg.MTNSimFailRate > 1 {
		return nil, fmt.Errorf("config: MTN_SIM_FAIL_RATE must be within [0, 1], got %v", cfg.MTNSimFailRate)
	}
	if cfg.MTNSimMode != "" && cfg.MTNSimMode != "success" && cfg.MTNSimMode != "fail" && cfg.MTNSimMode != "timeout" {
		return nil, fmt.Errorf("config: MTN_SIM_MODE must be one of success|fail|timeout|empty, got %q", cfg.MTNSimMode)
	}

	return cfg, nil
}

// Validate returns an error when required configuration is missing or invalid.
// Called explicitly by the entrypoint before any connection is attempted.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: VUKA_DATABASE_URL is required")
	}
	if c.HTTPAddr == "" {
		return fmt.Errorf("config: VUKA_HTTP_ADDR is required")
	}
	if c.GRPCAddr == "" {
		return fmt.Errorf("config: VUKA_GRPC_ADDR is required")
	}
	if c.FxRateRWFKES < 0 {
		return fmt.Errorf("config: VUKA_FX_RWF_KES must be >= 0, got %v", c.FxRateRWFKES)
	}
	return nil
}

// String returns a redacted, log-safe representation of the config.
// The database URL is printed without its password component.
func (c *Config) String() string {
	dsn := c.DatabaseURL
	// Strip credentials for logs: postgres://user:pass@host -> postgres://user:***@host
	if scheme := strings.Index(dsn, "://"); scheme >= 0 {
		rest := dsn[scheme+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			userinfo := rest[:at]
			if colon := strings.Index(userinfo, ":"); colon >= 0 {
				dsn = dsn[:scheme+3+colon+1] + "***@" + rest[at+1:]
			}
		}
	}
	return fmt.Sprintf(
		"http_addr=%s grpc_addr=%s db=%s adapter=%s mtn_delay=%s mtn_fail_rate=%.2f mtn_mode=%q fx_rwf_kes=%.4f",
		c.HTTPAddr, c.GRPCAddr, dsn, c.AdapterName, c.MTNSimDelay, c.MTNSimFailRate, c.MTNSimMode, c.FxRateRWFKES,
	)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// splitList parses a comma-separated env list into a trimmed slice, dropping
// empty entries (e.g. VUKA_CORS_ORIGINS).
func splitList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if raw, ok := os.LookupEnv(key); ok && raw != "" {
		if ms, err := strconv.ParseInt(raw, 10, 64); err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	if raw, ok := os.LookupEnv(key); ok && raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f
		}
	}
	return fallback
}
