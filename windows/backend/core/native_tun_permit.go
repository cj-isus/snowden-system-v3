package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Separate from the ordinary forwarding token. Preparation/emission does not
// open ingress or authorize a WFP change. The final compiled commit is separate.
type workerTUNPreparedPermit struct {
	subject, selector, token string
	epoch                    uint64
	until                    time.Time
	native                   *workerNativeTUNReceipt
	emitted                  bool
}

// Applied to every exchange, including hello/control/error replies. The new
// token is legal only on its exact native claiming probe; no other ACK can
// smuggle it into a parent record. Source custody is checked again by Engine.
func validNativePreparedReply(request *workerRequest, reply workerReply) bool {
	if reply.NativeAdmissionToken == "" {
		return true
	}
	return request != nil && request.Action == "probe" && request.Probe != nil && request.Probe.TUN && workerSubject(request.Probe.NativeNonce) &&
		reply.Code == "ok" && !reply.Admitted && reply.AdmissionToken == "" && reply.ScopeNonce == "" && reply.Report != nil &&
		workerSubject(reply.NativeAdmissionToken) && reply.Subject == request.Subject && reply.Previous == request.Tag && reply.AdmissionEpoch != 0
}

// Caller holds admission.mu. Both local monotonic deadlines remain original.
func (permit *workerTUNPreparedPermit) currentLocked(gate *forwardingAdmission, subject, selector string, epoch uint64, nonce string) bool {
	return gate != nil && gate.nativePending == permit && gate.nativeConsuming == nil && permit.matchesLocked(gate, subject, selector, epoch, nonce)
}

// Fixed original values only. Slot ownership must separately be established by
// currentLocked or the exact retained workerTUNConsumption, never by this match.
func (permit *workerTUNPreparedPermit) matchesLocked(gate *forwardingAdmission, subject, selector string, epoch uint64, nonce string) bool {
	return permit != nil && gate != nil && !gate.closed && !gate.open.Load() && gate.tunScope == nil && gate.pending == "" && gate.pendingUntil.IsZero() && gate.pendingIssue == nil &&
		gate.epoch == epoch && gate.subject == subject && gate.selector == selector &&
		permit.subject == subject && permit.selector == selector && permit.epoch == epoch && workerSubject(permit.token) && time.Now().Before(permit.until) &&
		permit.native.current(subject, selector, epoch, nonce) && permit.native.confirmed && permit.native.prepared
}

func (gate *forwardingAdmission) clearPendingLocked() {
	gate.pending, gate.pendingUntil, gate.nativePending = "", time.Time{}, nil
	gate.pendingIssue, gate.nativeConsuming = nil, nil // revoke original issuance/consumption attempts too
}

func (gate *forwardingAdmission) nativePendingCurrentLocked() bool {
	permit := gate.nativePending
	return permit != nil && permit.native != nil && permit.currentLocked(gate, permit.subject, permit.selector, permit.epoch, permit.native.nonce)
}

// Caller retains the original worker Engine.mu after its outbound checks pass.
// Burn the original native fact before cancellation, validation or entropy can
// fail. Copying a binding cannot issue a second token from the same native fact.
func (binding *workerTUNProbeBinding) prepareForwarding(ctx context.Context) error {
	if binding == nil || binding.admission == nil {
		return ErrForwardingNotAdmitted
	}
	gate := binding.admission
	subject, selector, epoch := binding.subject, binding.selector, binding.epoch
	var native, originalReceipt *workerNativeTUNReceipt
	var until time.Time
	var originalSubject, originalSelector string
	if err := func() error {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		gate.clearPendingLocked()
		native = binding.native
		spent := native != nil && native.prepared
		if native != nil {
			native.prepared = true // original one-use burn before ANY context/entropy callback
		}
		if spent || binding.existing || gate.closed || gate.open.Load() || gate.tunScope != nil || gate.epoch != epoch ||
			epoch == 0 || !workerSubject(subject) || !validWorkerTag(selector) || native == nil || !native.confirmed || !native.egress.Is4() ||
			!native.current(subject, selector, epoch, native.nonce) {
			return ErrForwardingNotAdmitted
		}
		originalSubject, originalSelector, originalReceipt = gate.subject, gate.selector, gate.nativeReceipt
		until = time.Now().Add(workerControlTimeout)
		if native.until.Before(until) {
			until = native.until // fixed before callbacks; waiting cannot renew the original budget
		}
		return nil
	}(); err != nil {
		return err
	}
	// Negative admission close/fault can now seal while context or OS entropy
	// work is pending. The outer Engine lock and one-use receipt remain original.
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return ErrForwardingNotAdmitted
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	candidate := &workerTUNPreparedPermit{subject: subject, selector: selector, epoch: epoch, token: hex.EncodeToString(nonce[:]), until: until, native: native}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if binding.admission != gate || binding.native != native || binding.subject != subject || binding.selector != selector || binding.epoch != epoch || binding.existing ||
		gate.closed || gate.open.Load() || gate.tunScope != nil || gate.epoch != epoch || gate.subject != originalSubject || gate.selector != originalSelector || gate.nativeReceipt != originalReceipt ||
		gate.pending != "" || !gate.pendingUntil.IsZero() || gate.pendingIssue != nil || gate.nativePending != nil || gate.nativeConsuming != nil || !native.prepared || !native.confirmed || !native.egress.Is4() ||
		!native.current(subject, selector, epoch, native.nonce) || contextError(operation) != nil || !time.Now().Before(until) {
		return ErrForwardingNotAdmitted
	}
	gate.subject, gate.selector = subject, selector
	gate.nativePending = candidate // original normally returned preparation only; still no ingress grant
	return nil
}

// One private emission from the actual pending record, after the local probe
// returns. No public report or request boolean can supply this record/token.
func (engine *Engine) emitPreparedNative(ctx context.Context, subject, selector string, epoch uint64, nonce string) (string, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	actual, selected, guarded, tunMode, err := engine.forwardingSubjectLocked()
	if err != nil || !tunMode || guarded == nil {
		return "", ErrForwardingNotAdmitted
	}
	gate := guarded.admission
	var original *workerTUNPreparedPermit
	if err := func() error {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		original = gate.nativePending
		emitted := original != nil && original.emitted
		if original != nil {
			original.emitted = true // burn emission BEFORE any context callback
		}
		if emitted || actual != subject || selected != selector || !original.currentLocked(gate, subject, selector, epoch, nonce) {
			gate.clearPendingLocked()
			return ErrForwardingNotAdmitted
		}
		return nil
	}(); err != nil {
		return "", err
	}
	returned := false
	defer func() {
		if returned {
			return
		}
		gate.mu.Lock()
		defer gate.mu.Unlock()
		if gate.nativePending == original {
			gate.clearPendingLocked() // same candidate only; never clear a newer issuance/consume
		}
	}()
	operation, err := snapshotNativeTUNOperation(ctx) // admission mutex is free; Engine owner is unchanged
	if err != nil {
		return "", ErrForwardingNotAdmitted
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.nativePending != original || !original.emitted || !original.currentLocked(gate, subject, selector, epoch, nonce) || contextError(operation) != nil {
		return "", ErrForwardingNotAdmitted
	}
	returned = true
	return original.token, nil
}

// Opaque original parent/source custody. busyUntil is only a local no-steal
// bound; the worker's earlier native/5s deadline remains authoritative. Neither
// this record nor a public report currently authorizes TUN consumption.
type engineNativeTUNPermit struct {
	process                  *engineProcess
	instance, epoch          uint64
	subject, selector, token string
	source                   *nativeTUNObservationSource
	busyUntil                time.Time
	diagnostic               *ProbeReport // original worker facts only, before product pending-activation projection
}

func (permit *engineNativeTUNPermit) busyLocked(engine *Engine) bool {
	return permit != nil && engine != nil && engine.isolated && engine.cleanupErr == nil && permit.source != nil && permit.source.operation != nil && permit.process != nil && permit.process == engine.process && permit.instance == permit.process.instance &&
		!permit.process.exited() && contextError(permit.source.operation) == nil && time.Now().Before(permit.busyUntil)
}

// Caller holds Engine.mu. Every original lifecycle retirement/mutation revokes
// both lanes and their in-flight handoffs; a native token can never fall
// through ordinary admitForwarding. Revoke before fallible resource work.
func (engine *Engine) clearForwardingPermitsLocked() {
	engine.forwardingPermit, engine.forwardingConsume, engine.nativePermit = nil, nil, nil
	if engine.process != nil {
		engine.process.nativeConsume = nil
		engine.process.tunOpening = nil
		engine.process.tunClosing = nil
		engine.process.probePending = nil
	}
}

const nativeActivationPendingDetail = "Общий допуск TUN не подтверждён; пользовательский трафик остаётся заблокирован."

func pendingTUNActivation(report *ProbeReport) *ProbeReport {
	if report == nil {
		return workerProbeFailure()
	}
	out := report.Clone()
	out.AllPassed = false // independent ProtectionUnavailable is never cleared or synthesized from this pending phase
	out.Results = append(out.Results, ProbeResult{Name: "TUN activation", Status: "unavailable", Detail: nativeActivationPendingDetail})
	return out
}
