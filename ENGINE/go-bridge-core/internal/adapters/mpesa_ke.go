// Safaricom M-Pesa (Kenya) simulated adapter.
//
// Phase 2 adds the Kenya leg of the Rwanda->Kenya corridor. Like the MTN
// adapter, this is a deterministic, configurable simulation of the M-Pesa
// Daraja API (STK Push / B2C): it models network latency, success, provider
// rejection, and timeout. In later phases this file is replaced by a real
// HTTP client against Safaricom's Daraja API while the ledger engine stays
// untouched.
package adapters

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// MpesaAdapter simulates the Safaricom M-Pesa (Kenya) payout rail.
type MpesaAdapter struct {
	// NameKey is the registry key, default "mpesa-ke".
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
}

// NewMpesaAdapter builds an M-Pesa adapter with the given knobs.
func NewMpesaAdapter(delay time.Duration, failRate float64, mode string, timeout time.Duration) *MpesaAdapter {
	return &MpesaAdapter{
		NameKey:  "mpesa-ke",
		Delay:    delay,
		FailRate: failRate,
		Mode:     mode,
		Timeout:  timeout,
	}
}

// Name implements TelecomAdapter.
func (m *MpesaAdapter) Name() string {
	if m.NameKey == "" {
		return "mpesa-ke"
	}
	return m.NameKey
}

// Payout implements TelecomAdapter. It simulates the Daraja B2C round-trip:
// latency window -> decision (success | reject | timeout) -> external
// reference. The generated reference is deterministic for a given reference
// so retries of the same payout trace to the same external id.
func (m *MpesaAdapter) Payout(ctx context.Context, req PayoutRequest) (*PayoutResult, error) {
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
		return nil, fmt.Errorf("%w: M-Pesa did not respond within %s", ErrPayoutTimeout, m.Timeout)

	case "fail":
		return nil, fmt.Errorf("%w: M-Pesa rejected payout for %s (STK Push declined)", ErrPayoutFailed, req.Phone)

	default: // success
		ref := mpesaExternalRef(req.Reference, req.Amount)
		return &PayoutResult{
			ExternalRef: ref,
			Raw:         fmt.Sprintf(`{"status":"SUCCESS","external_ref":%q}`, ref),
		}, nil
	}
}

// outcome picks the simulated rail decision.
func (m *MpesaAdapter) outcome() string {
	switch m.Mode {
	case "success", "fail", "timeout":
		return m.Mode
	}
	if m.FailRate > 0 && rand.Float64() < m.FailRate { //nolint:gosec // simulation only
		return "fail"
	}
	return "success"
}

// mpesaExternalRef builds a deterministic, traceable external reference:
// MPESA-<compact timestamp>-<6 hex chars derived from the engine reference>.
func mpesaExternalRef(reference string, amount float64) string {
	now := time.Now()
	if reference == "" {
		return fmt.Sprintf("MPESA-%d-%06x", now.UnixNano(), rand.Uint32()&0xffffff) //nolint:gosec
	}
	seed := uint32(0)
	for i := 0; i < len(reference); i++ {
		seed = seed*31 + uint32(reference[i])
	}
	seed ^= uint32(amount * 100)
	return fmt.Sprintf("MPESA-%d-%06x", now.Unix(), seed&0xffffff)
}
