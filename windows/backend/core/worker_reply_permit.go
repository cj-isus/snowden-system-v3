package core

import "time"

// Read-only worker fence for a reply carrying a freshly issued token. It is
// called after JSON encoding, before any frame header/body byte. The report and
// token text cannot replace the actual original admission record below.
func (engine *Engine) workerReplyPermitCurrent(request *workerRequest, reply workerReply) bool {
	if engine == nil || reply.Code != "ok" || reply.Admitted || !validWorkerProbeReport(request, reply) ||
		!validNativePreparedReply(request, reply) || !probePassed(reply.Report) {
		return false
	}
	ordinary := !request.Probe.TUN && request.Probe.NativeNonce == "" && workerSubject(reply.AdmissionToken) && reply.NativeAdmissionToken == ""
	native := request.Probe.TUN && workerSubject(request.Probe.NativeNonce) && reply.AdmissionToken == "" && workerSubject(reply.NativeAdmissionToken)
	if !ordinary && !native {
		return false
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	guarded, ok := engine.box.(*admissionBox)
	if !ok || guarded == nil || guarded.admission == nil || engine.cleanupErr != nil {
		return false
	}
	gate := guarded.admission
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.closed || gate.open.Load() || gate.tunScope != nil || gate.epoch != reply.AdmissionEpoch ||
		gate.subject != reply.Subject || gate.selector != reply.Previous {
		return false
	}
	if ordinary {
		return gate.pending == reply.AdmissionToken && time.Now().Before(gate.pendingUntil) &&
			gate.pendingIssue == nil && gate.nativePending == nil && gate.nativeConsuming == nil
	}
	permit := gate.nativePending
	return permit != nil && permit.emitted && permit.token == reply.NativeAdmissionToken &&
		permit.currentLocked(gate, reply.Subject, reply.Previous, reply.AdmissionEpoch, request.Probe.NativeNonce)
}
