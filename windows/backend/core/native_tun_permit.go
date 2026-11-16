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
	return permit != nil && gate != nil && gate.nativePending == permit && !gate.closed && !gate.open.Load() && gate.tunScope == nil && gate.pending == "" &&
		gate.epoch == epoch && gate.subject == subject && gate.selector == selector &&
		permit.subject == subject && permit.selector == selector && permit.epoch == epoch && workerSubject(permit.token) && time.Now().Before(permit.until) &&
		permit.native.current(subject, selector, epoch, nonce) && permit.native.confirmed && permit.native.prepared
}

func (gate *forwardingAdmission) clearPendingLocked() {
	gate.pending, gate.pendingUntil, gate.nativePending = "", time.Time{}, nil
}

func (gate *forwardingAdmission) nativePendingCurrentLocked() bool {
	permit := gate.nativePending
	return permit != nil && permit.native != nil && permit.currentLocked(gate, permit.subject, permit.selector, permit.epoch, permit.native.nonce)
}

// Called only after this binding's actual original-worker outbound checks pass.
// Burn the original native fact before cancellation, validation or entropy can
// fail. Copying a binding cannot issue a second token from the same native fact.
func (binding *workerTUNProbeBinding) prepareForwarding(ctx context.Context) error {
	if binding == nil || binding.admission == nil {
		return ErrForwardingNotAdmitted
	}
	gate := binding.admission
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.clearPendingLocked()
	native := binding.native
	spent := native != nil && native.prepared
	if native != nil {
		native.prepared = true
	}
	if spent || contextError(ctx) != nil || binding.existing || gate.closed || gate.open.Load() || gate.tunScope != nil || gate.epoch != binding.epoch ||
		binding.epoch == 0 || !workerSubject(binding.subject) || !validWorkerTag(binding.selector) || native == nil || !native.confirmed || !native.egress.Is4() ||
		!native.current(binding.subject, binding.selector, binding.epoch, native.nonce) {
		return ErrForwardingNotAdmitted
	}
	until := time.Now().Add(workerControlTimeout)
	if native.until.Before(until) {
		until = native.until
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	if contextError(ctx) != nil || !time.Now().Before(until) {
		return ErrForwardingNotAdmitted
	}
	gate.subject, gate.selector = binding.subject, binding.selector
	gate.nativePending = &workerTUNPreparedPermit{subject: binding.subject, selector: binding.selector, epoch: binding.epoch, token: hex.EncodeToString(nonce[:]), until: until, native: native}
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
	gate.mu.Lock()
	defer gate.mu.Unlock()
	permit := gate.nativePending
	emitted := permit != nil && permit.emitted
	if permit != nil {
		permit.emitted = true // emission attempt is burned before every outcome
	}
	if emitted || actual != subject || selected != selector || contextError(ctx) != nil || !permit.currentLocked(gate, subject, selector, epoch, nonce) {
		gate.clearPendingLocked()
		return "", ErrForwardingNotAdmitted
	}
	return permit.token, nil
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
	return permit != nil && engine != nil && engine.isolated && engine.cleanupErr == nil && permit.source != nil && permit.process != nil && permit.process == engine.process && permit.instance == permit.process.instance &&
		!permit.process.exited() && contextError(permit.source.operation) == nil && time.Now().Before(permit.busyUntil)
}

// Caller holds Engine.mu. Every original lifecycle retirement/mutation revokes
// both lanes; a native token can never fall through ordinary admitForwarding.
func (engine *Engine) clearForwardingPermitsLocked() {
	engine.forwardingPermit, engine.nativePermit = nil, nil
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
