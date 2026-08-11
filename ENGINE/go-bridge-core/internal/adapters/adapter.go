// Package adapters defines the telecom rail abstraction for VUKA.
//
// Phase 1 ships one production-shaped simulated adapter: MTN Rwanda Mobile
// Money (mtn-rw). The interface and registry mirror the real-world contract
// (dispatch a payout, receive an external reference or a failure) so that
// M-Pesa Kenya, Airtel Money, and Equity Bank adapters can be added in later
// phases without touching the ledger engine.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrPayoutFailed is returned when the rail definitively rejects a payout
// (e.g. insufficient wallet balance, invalid number, provider error).
var ErrPayoutFailed = errors.New("adapter: payout failed")

// ErrPayoutTimeout is returned when the rail did not respond within the
// configured window. The ledger treats this as a failed dispatch (no entries
// written) and the operator can retry with the same idempotency key.
var ErrPayoutTimeout = errors.New("adapter: payout timed out")

// PayoutRequest is the normalized payload sent to a telecom rail.
type PayoutRequest struct {
	// Amount is the payout amount in the rail's minor-unit-free decimal form.
	Amount float64
	// Currency is the ISO-4217 code, e.g. "RWF".
	Currency string
	// Phone is the destination mobile-money number in E.164 shape, e.g.
	// "+250781234567".
	Phone string
	// Reference is the engine-side reference echoed to the rail for
	// reconciliation (the VUKA transaction id).
	Reference string
	// InvoiceNumber, when present, is carried through for audit tracing.
	InvoiceNumber string
	// Metadata carries rail-specific extras (narration, channel, etc.).
	Metadata map[string]string
}

// PayoutResult is the normalized response from a telecom rail.
type PayoutResult struct {
	// ExternalRef is the rail's own reference for the payout. Required on
	// success; used for end-to-end transaction tracing (spec §4).
	ExternalRef string
	// Raw is the provider's raw response payload for audit logging.
	Raw string
}

// TelecomAdapter is the contract every rail adapter must satisfy.
type TelecomAdapter interface {
	// Name returns the adapter's registry key, e.g. "mtn-rw".
	Name() string
	// Payout dispatches a payout to the rail and waits for the outcome.
	// It must return (result, nil) on success, ErrPayoutFailed with a
	// descriptive error on provider rejection, or ErrPayoutTimeout when the
	// rail does not answer in time. It MUST NOT panic on context
	// cancellation; it should return ctx.Err() wrapped appropriately.
	Payout(ctx context.Context, req PayoutRequest) (*PayoutResult, error)
}

// Registry resolves adapters by name. The ledger engine asks the registry
// for the configured adapter and never imports concrete implementations.
type Registry struct {
	adapters map[string]TelecomAdapter
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]TelecomAdapter)}
}

// Register adds an adapter under its Name(). Registering a duplicate name
// panics at startup time, which is the right failure mode for wiring errors.
func (r *Registry) Register(a TelecomAdapter) {
	name := a.Name()
	if _, exists := r.adapters[name]; exists {
		panic(fmt.Sprintf("adapters: duplicate registration for %q", name))
	}
	r.adapters[name] = a
}

// Get resolves an adapter by name. Returns nil when unknown.
func (r *Registry) Get(name string) TelecomAdapter {
	return r.adapters[name]
}

// Names returns the sorted set of registered adapter names (for logging).
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// SleepCtx sleeps for d unless the context is cancelled first. It is used by
// simulated adapters to model rail latency while remaining cancellable.
func SleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
