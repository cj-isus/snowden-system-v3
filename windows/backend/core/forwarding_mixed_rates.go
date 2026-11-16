package core

import (
	"context"
	"net"
	"time"

	N "github.com/sagernet/sing/common/network"
	"golang.org/x/time/rate"
)

// Initial per-box intake policy, not a measured throughput optimum or a byte/
// process-wide/OS quota. Existing authenticated requests/payload are not charged.
const (
	mixedInputStartsPerSecond = 256
	mixedInputStartBurst      = 512
	mixedHTTPAuthRetryRate    = 64
	mixedHTTPAuthRetryBurst   = 128
)

type mixedIngressRates struct {
	starts, authRetries *rate.Limiter
}

func newMixedIngressRates() mixedIngressRates {
	return mixedIngressRates{
		starts:      rate.NewLimiter(mixedInputStartsPerSecond, mixedInputStartBurst),
		authRetries: rate.NewLimiter(mixedHTTPAuthRetryRate, mixedHTTPAuthRetryBurst),
	}
}

func (limits mixedIngressRates) configured() bool {
	return limits.starts != nil && limits.authRetries != nil
}

func (limits mixedIngressRates) allowStart() bool {
	return limits.configured() && limits.starts.AllowN(time.Now(), 1)
}

// Keep backpressure in the original sole Accept task, before a new socket/root/
// timer callback/handler goroutine is acquired. Reuse its existing cancellable
// channel-timer wait frame; no rate.Wait/Reserve queue or hidden worker is added.
func (inbound *ownedMixedInbound) waitMixedIntake() bool {
	if !inbound.gate.mixedRates.configured() {
		inbound.gate.noteRouteFault()
		return false
	}
	for !inbound.isClosing() && inbound.ctx.Err() == nil {
		if inbound.gate.mixedRates.allowStart() {
			return true
		}
		if !waitMixedAcceptRetry(inbound.ctx, time.Second/time.Duration(mixedInputStartsPerSecond)) {
			return false
		}
	}
	return false
}

// Injection has already acquired raw elsewhere. A denied turn is retained by
// the existing bounded synchronous rejection lane before native Close/callback;
// it never creates a per-rejection handler/AfterFunc or retries a native Close.
func (gate *forwardingAdmission) registerMixedInput(ctx context.Context, raw net.Conn, callback N.CloseHandlerFunc) (*mixedIngress, bool) {
	if ctx == nil || raw == nil || ctx.Err() != nil || !gate.mixedRates.allowStart() {
		_ = (admissionRouter{admission: gate}).rejectIngress(raw, callback, ErrForwardingNotAdmitted)
		return nil, false
	}
	return gate.registerAcceptedMixedInput(ctx, raw, callback)
}

// Only an already rejected HTTP authentication attempt can request another
// keep-alive challenge. Successful credentials and ordinary payload do not
// debit this bucket. Native-peer/username/header values never mint a bypass.
func (input *mixedIngress) allowMixedHTTPAuthRetry() bool {
	gate := input.gate
	gate.mu.Lock()
	_, owned := gate.mixedInputs[input]
	current := owned && !gate.closed && !gate.routeFault && !gate.rejectionsSealed
	gate.mu.Unlock()
	input.mu.Lock()
	current = current && !input.closing && input.ctx.Err() == nil
	input.mu.Unlock()
	// Limiter's own value mutex is separate from gate/input locks. No timer,
	// callback, OS operation, wait or credential-dependent key cache is created.
	return current && gate.mixedRates.configured() && gate.mixedRates.authRetries.AllowN(time.Now(), 1)
}
