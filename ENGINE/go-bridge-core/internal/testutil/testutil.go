// Package testutil provides shared helpers for the Go engine tests:
// a live-PostgreSQL test harness and a deterministic MTN adapter.
package testutil

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vuka/go-bridge-core/internal/adapters"
)

var (
	poolOnce sync.Once
	poolPtr  *pgxpool.Pool
	poolURL  string
)

// Logger returns a discard logger for tests (avoids log noise in output).
func Logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// DefaultTestURL is used when TEST_DATABASE_URL is unset. It targets the
// vuka_test database provisioned on localhost:5432 (see deploy/).
const DefaultTestURL = "postgres://vuka:changeme@localhost:5432/vuka_test"

// TestPool returns a SHARED, process-wide pool to the test database and
// truncates all ledger tables so each test starts from a clean ledger.
//
// The pool is created once per test binary. Creating a pool per test leaks
// connections; those lingering connections hold lock pedges that make the
// next test's TRUNCATE deadlock (SQLSTATE 40P01) and poison later tests.
//
// go test ./internal/... runs packages in PARALLEL processes; each package
// shares the same vuka_test database. To keep packages from truncating each
// other's rows mid-test, TestPool takes a session-level advisory lock for the
// duration of the test, which serializes all packages onto the shared DB.
const advisoryLockKey = 0x564B41 // "VKA"

func TestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = DefaultTestURL
	}
	poolOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		poolPtr, err = pgxpool.New(ctx, url)
		if err != nil {
			t.Fatalf("testutil: connect to %s: %v", url, err)
		}
		poolURL = url
	})

	// Hold the advisory lock for the whole test on a dedicated connection,
	// released in t.Cleanup so the next test (or another package) can run.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lockConn, err := poolPtr.Acquire(ctx)
	if err != nil {
		t.Fatalf("testutil: acquire lock connection: %v", err)
	}
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		lockConn.Release()
		t.Fatalf("testutil: advisory lock: %v", err)
	}
	t.Cleanup(func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unlockCancel()
		_, _ = lockConn.Exec(unlockCtx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey)
		lockConn.Release()
	})

	// Truncate all ledger tables (idempotency_keys first due to FK order).
	truncCtx, truncCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer truncCancel()
	if _, err := poolPtr.Exec(truncCtx, `
		TRUNCATE TABLE idempotency_keys, ledger_entries, transactions, accounts, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("testutil: truncate tables: %v", err)
	}
	return poolPtr
}

// MTNSuccessAsync returns a fast, always-succeeding MTN adapter for tests.
// Delay is zero so tests never block on simulated network latency.
func MTNSuccessAsync() *adapters.MTNAdapter {
	return adapters.NewMTNAdapter(0, 0, "success", 5*time.Second)
}

// MTNAdapterRegistry builds a registry containing a deterministic adapter
// and returns both, for tests that resolve adapters by name.
func MTNAdapterRegistry(mode string, failRate float64) (*adapters.Registry, *adapters.MTNAdapter) {
	reg := adapters.NewRegistry()
	a := adapters.NewMTNAdapter(0, failRate, mode, 5*time.Second)
	reg.Register(a)
	return reg, a
}

// CorridorRegistry builds a registry with BOTH rails registered (MTN Rwanda
// for RWF legs, M-Pesa Kenya for KES legs) — the Phase 2 corridor setup.
func CorridorRegistry() *adapters.Registry {
	reg := adapters.NewRegistry()
	reg.Register(adapters.NewMTNAdapter(0, 0, "success", 5*time.Second))
	reg.Register(adapters.NewMpesaAdapter(0, 0, "success", 5*time.Second))
	return reg
}

// UniqueID returns a distinct, time-based identifier for idempotency keys.
func UniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}