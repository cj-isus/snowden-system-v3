package core

import (
	"context"
	"errors"
	"time"
)

// LifetimeProbe checks immutable local trust constraints, without disk/network
// I/O or Manager callbacks. It does not grant permission to release protection.
type LifetimeProbe interface {
	Probe
	LifetimeRemaining() (time.Duration, error)
}

// TrustFaultError carries a finite local reason across the lifecycle API; it is
// separate from a network probe failure and contains no profile/private bytes.
type TrustFaultError struct{ Reason string }

func (e *TrustFaultError) Error() string { return "local session trust unavailable: " + e.Reason }

func (e *TrustFaultError) Is(target error) bool {
	other, ok := target.(*TrustFaultError)
	return ok && other != nil && e.Reason == other.Reason
}

func reportTrustError(report *ProbeReport) error {
	if report == nil || !report.TrustUnavailable {
		return nil
	}
	return &TrustFaultError{Reason: trustFaultReason(report.TrustReason)}
}

func trustFaultReason(reason string) string {
	switch reason {
	case "clock_uncertain", "profile_expired", "certificate_expired", "trust_not_yet_valid":
		return reason
	default:
		return "trust_unavailable"
	}
}

// Keep local custody uncertainty typed across the report-only lifecycle seam.
// Raw native errors/paths/handles are not serialized or copied into UI detail.
// This finite flag only restricts admission and automatic successor attempts.
func reportLifecycleError(report *ProbeReport) error {
	var custody error
	if report != nil && report.CleanupUnconfirmed {
		custody = ErrHelperCleanup
	}
	return errors.Join(custody, reportTrustError(report))
}

func lifetimeRemaining(probe Probe) (remaining time.Duration, err error) {
	remaining = time.Second // existing non-Lifetime legacy contract, not a measured trust lifetime
	if p, ok := probe.(LifetimeProbe); ok {
		returned := false
		err = cleanupCall("local trust check", func() error {
			var sourceErr error
			remaining, sourceErr = p.LifetimeRemaining()
			returned = true
			return sourceErr
		})
		if !returned {
			remaining = 0 // unknown with typed error; never retain the positive legacy default
			err = errors.Join(&TrustFaultError{Reason: "trust_unavailable"}, err)
		} else if err == nil && remaining <= 0 {
			err = &TrustFaultError{Reason: "trust_unavailable"}
		}
	}
	return remaining, err
}

func operationLifetimeCause(ctx context.Context) error {
	cause := context.Cause(ctx)
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return nil // ordinary caller cancellation is handled by the lifecycle
	}
	return cause
}

// Notification is only a coalesced wake. The receiver rechecks this Manager's
// current retained LifetimeError/report; no arbitrary App callback runs here.
func (m *Manager) SetLifetimeFaultSignal(signal chan<- struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateStopped && m.active == nil {
		m.lifetimeSignal = signal
	}
}

func (m *Manager) LifetimeError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lifetimeErr
}

// GuardRunningOwner pins only lifecycle identity/state for a short value-only
// publication. Caller must release before events, disk/network I/O or lifecycle
// calls. Lock order is runtime -> Manager -> App -> epoch -> clock/health.
func (m *Manager) GuardRunningOwner(owner, generation uint64) (release func(), ok bool) {
	m.mu.RLock()
	if owner == 0 || generation == 0 || m.identity != owner || m.generation != generation || m.active != nil || m.state != StateRunning || m.engine != nil && m.engine.Status() != "running" {
		m.mu.RUnlock()
		return nil, false
	}
	return m.mu.RUnlock, true
}

// Negative-only publication may retire the exact inactive original engine
// after its process failed. A dead worker is not grounds to discard a local
// cleanup fault. This guard cannot admit forwarding or publish healthy state.
func (m *Manager) GuardRetirementOwner(owner, generation uint64, engine *Engine) (release func(), ok bool) {
	m.mu.RLock()
	if owner == 0 || generation == 0 || engine == nil || m.identity != owner || m.generation != generation || m.engine != engine || m.active != nil ||
		m.state != StateRunning && m.state != StateError {
		m.mu.RUnlock()
		return nil, false
	}
	return m.mu.RUnlock, true
}

// GuardLifecycleOwner pins the active, unpublished Start/Reload candidate for
// short value-only platform checks. Unlike GuardRunningOwner, it cannot accept
// a completed operation or a stopped/replacement engine. No native work, IPC,
// callbacks or Engine methods may run while the returned read lock is held.
// This identity guard grants no forwarding authority.
func (m *Manager) GuardLifecycleOwner(owner, generation uint64, engine *Engine) (release func(), ok bool) {
	m.mu.RLock()
	if owner == 0 || generation == 0 || engine == nil || m.identity != owner || m.generation != generation || m.engine != engine || m.active == nil || m.state != StateStarting && m.state != StateReloading {
		m.mu.RUnlock()
		return nil, false
	}
	return m.mu.RUnlock, true
}
