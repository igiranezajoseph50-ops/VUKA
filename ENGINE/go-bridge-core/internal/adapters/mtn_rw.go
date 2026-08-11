// MTN Rwanda Mobile Money simulated adapter.
//
// Phase 1 uses a deterministic, configurable simulation of the MTN MoMo
// payout API: it models network latency, success, provider rejection, and
// timeout paths. In later phases this file is replaced by a real HTTP client
// against MTN's Open API while the ledger engine stays untouched.
package adapters

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// MTNAdapter simulates the MTN Rwanda Mobile Money payout rail.
type MTNAdapter struct {
	// NameKey is the registry key, default "mtn-rw".
	NameKey string
	// Delay is the simulated network latency before the rail answers.
	Delay time.Duration
	// FailRate is the probability (0.0-1.0) of a provider rejection.
	FailRate float64
	// Mode forces deterministic behaviour: "success", "fail", "timeout",
	// or "" to use FailRate randomness.
	Mode string
	// Timeout is the rail response window; exceeding it yields
	// ErrPayoutTimeout.
	Timeout time.Duration
	// Now is injectable for deterministic tests (defaults to time.Now).
	Now func() time.Time
}

// NewMTNAdapter builds an MTN adapter with the given knobs.
func NewMTNAdapter(delay time.Duration, failRate float64, mode string, timeout time.Duration) *MTNAdapter {
	return &MTNAdapter{
		NameKey: "mtn-rw",
		Delay:   delay,
		FailRate: failRate,
		Mode:    mode,
		Timeout: timeout,
	}
}

// Name implements TelecomAdapter.
func (m *MTNAdapter) Name() string {
	if m.NameKey == "" {
		return "mtn-rw"
	}
	return m.NameKey
}

// Payout implements TelecomAdapter. It simulates the full rail round-trip:
// latency window -> decision (success | reject | timeout) -> external
// reference. The generated reference is deterministic for a given reference
// so retries of the same payout trace to the same external id.
func (m *MTNAdapter) Payout(ctx context.Context, req PayoutRequest) (*PayoutResult, error) {
	// Rail latency window. Cancellable so engine shutdown propagates.
	if err := SleepCtx(ctx, m.Delay); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPayoutTimeout, err)
	}

	outcome := m.outcome()

	switch outcome {
	case "timeout":
		if err := SleepCtx(ctx, m.Timeout); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPayoutTimeout, err)
		}
		return nil, fmt.Errorf("%w: MTN MoMo did not respond within %s", ErrPayoutTimeout, m.Timeout)

	case "fail":
		return nil, fmt.Errorf("%w: MTN MoMo rejected payout for %s (insufficient wallet balance)", ErrPayoutFailed, req.Phone)

	default: // success
		ref := mtnExternalRef(req.Reference, req.Amount)
		return &PayoutResult{
			ExternalRef: ref,
			Raw:         fmt.Sprintf(`{"status":"SUCCESS","external_ref":%q}`, ref),
		}, nil
	}
}

// outcome picks the simulated rail decision.
func (m *MTNAdapter) outcome() string {
	switch m.Mode {
	case "success", "fail", "timeout":
		return m.Mode
	}
	if m.FailRate > 0 && rand.Float64() < m.FailRate { //nolint:gosec // simulation only
		return "fail"
	}
	return "success"
}

// mtnExternalRef builds a deterministic, traceable external reference:
// MTN-<compact timestamp>-<6 hex chars derived from the engine reference>.
func mtnExternalRef(reference string, amount float64) string {
	now := time.Now()
	if reference == "" {
		return fmt.Sprintf("MTN-%d-%06x", now.UnixNano(), rand.Uint32()&0xffffff) //nolint:gosec
	}
	seed := uint32(0)
	for i := 0; i < len(reference); i++ {
		seed = seed*31 + uint32(reference[i])
	}
	seed ^= uint32(amount * 100)
	return fmt.Sprintf("MTN-%d-%06x", now.Unix(), seed&0xffffff)
}
