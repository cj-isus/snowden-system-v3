package main

import "github.com/snowden-system/windows/backend/core"

// The root lease stays private through the report's trust/source finalizer.
// Copies of a retained probe share its slot; this record identifies one Run.
type appNativeProbePublication struct {
	slot  *appNativeCommitSlot
	lease *tunnelLease // guarded by slot.mu; never itself forwarding authority
}

func (slot *appNativeCommitSlot) reserveProbe() *appNativeProbePublication {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.pending != nil || slot.lease != nil {
		return nil // neither steal unfinished work nor replace an accepted lease
	}
	publication := &appNativeProbePublication{slot: slot}
	slot.pending = publication
	return publication
}

func (publication *appNativeProbePublication) stage(lease tunnelLease) error {
	if publication == nil || publication.slot == nil || lease.native == nil {
		return core.ErrNativeScopeUnavailable
	}
	slot := publication.slot
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.pending != publication || slot.lease != nil || publication.lease != nil {
		return core.ErrNativeScopeUnavailable
	}
	publication.lease = &lease
	return nil
}

func (publication *appNativeProbePublication) staged() bool {
	slot := publication.slot
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return publication.lease != nil // revoked stage still reaches the rejecting identity check
}

// Caller holds the original subject's runtime -> Manager -> App -> epoch lease.
// No context getter, trust/native callback, Engine work or I/O runs under mu.
func (publication *appNativeProbePublication) publish() error {
	slot := publication.slot
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.pending != publication || slot.lease != nil || publication.lease == nil {
		return core.ErrNativeScopeUnavailable
	}
	slot.lease = publication.lease
	slot.pending = nil
	return nil
}

func (publication *appNativeProbePublication) discard() {
	if publication == nil || publication.slot == nil {
		return
	}
	slot := publication.slot
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.pending == publication {
		slot.pending = nil
	}
	if publication.lease != nil && slot.lease == publication.lease {
		slot.lease = nil // abnormal return after publish revokes only this exact lease
	}
}

// Only the original Run calls this after its report finalizer returned. The
// outer ordinary-return guard still owns rollback through the subject release.
func (probe appProtectedProbe) publishNativeProbe(attempt *protectedProbeAttempt, report *core.ProbeReport) error {
	publication := attempt.native
	if publication == nil {
		return nil // periodic/non-TUN run; missing required reservation failed in the body
	}
	if report == nil || !attempt.subjectValid || !attempt.workStarted || report.Stale || report.TrustUnavailable || unsafeObservation(report) || !publication.staged() {
		publication.discard()
		return nil
	}
	release, ok := attempt.subject.lockCurrent(probe.app, attempt.scope, probe.config)
	if !ok {
		return core.ErrNativeScopeUnavailable
	}
	released := false
	releaseSubject := func() {
		if !released {
			released = true // never retry an interrupted original release
			release()
		}
	}
	defer releaseSubject()
	err := publication.publish()
	releaseSubject()
	return err
}
