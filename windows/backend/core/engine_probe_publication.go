package core

import (
	"context"
	"time"
)

// The original worker's reply and private permit candidates stay unpublished
// through RPC cancellation and diagnostic release. This slot grants nothing.
type engineProbePublication struct {
	origin      engineProcessOrigin
	operation   *nativeTUNOperation // original lifecycle, not the diagnostic child released by Run
	rpc         *workerRPCContext
	ordinary    *engineForwardingPermit
	native      *engineNativeTUNPermit
	requestID   uint64
	requested   bool
	rpcAccepted bool
	released    bool // set only after the original diagnostic release returned normally
}

func (e *Engine) probePublicationCurrentLocked(publication *engineProbePublication) bool {
	return publication != nil && publication.origin.process != nil && publication.origin.currentLocked(e) &&
		publication.origin.process.probePending == publication && e.cleanupErr == nil &&
		e.startPending == nil && e.reloadPending == nil && e.tunScope == nil &&
		publication.origin.process.ready && !publication.origin.process.unusable && !publication.origin.process.exited()
}

// A private staged native token still owns its existing bounded no-steal
// interval. It is not exposed through nativePreparationCurrent or consumption.
func (e *Engine) nativeProbeBusyLocked() bool {
	if e.process == nil {
		return false
	}
	publication := e.process.probePending
	return e.probePublicationCurrentLocked(publication) && publication.native != nil && publication.native.busyLocked(e)
}

// Called after the explicit RPC cancel, before Run deliberately releases its
// diagnostic reservation. Check that exact diagnostic parent's original Done.
func (e *Engine) acceptProbeRPCCleanup(publication *engineProbePublication) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if publication == nil {
		return ErrForwardingNotAdmitted
	}
	if publication.origin.process == nil {
		return nil // only a finite unavailable/no-steal report, never a permit
	}
	if publication.rpc == nil || publication.rpc.parentScope == nil || publication.operation == nil {
		return ErrForwardingNotAdmitted
	}
	if err := contextError(publication.operation); err != nil {
		return err
	}
	if err := contextError(publication.rpc.parentScope); err != nil {
		return err
	}
	if !time.Now().Before(publication.rpc.deadline) {
		return context.DeadlineExceeded
	}
	if !e.probePublicationCurrentLocked(publication) {
		return ErrEngineWorkerUnavailable
	}
	publication.rpcAccepted = true
	return nil
}

// The diagnostic child has now been intentionally canceled by its own release.
// Recheck the lifecycle, fixed budget and frozen negative sources, then publish
// only this original process/graph's candidates. No later callback is allowed.
func (e *Engine) publishProbeAfterRelease(publication *engineProbePublication, report *ProbeReport) (*ProbeReport, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if publication == nil || report == nil || !publication.released {
		return nil, ErrForwardingNotAdmitted
	}
	if publication.origin.process == nil {
		if report.AllPassed || publication.ordinary != nil || publication.native != nil {
			return nil, ErrForwardingNotAdmitted
		}
		return report, nil // no observation or handoff was claimed
	}
	if !publication.rpcAccepted || publication.rpc == nil || publication.operation == nil {
		return nil, ErrForwardingNotAdmitted
	}
	if err := contextError(publication.operation); err != nil {
		return nil, err
	}
	if !time.Now().Before(publication.rpc.deadline) {
		return nil, context.DeadlineExceeded
	}
	if !OperationSourcesCurrent(publication.rpc) || !e.probePublicationCurrentLocked(publication) ||
		publication.ordinary != nil && !publication.ordinary.currentOperation(publication.operation) ||
		publication.native != nil && !publication.native.busyLocked(e) {
		return nil, ErrForwardingNotAdmitted
	}
	e.forwardingPermit, e.nativePermit = publication.ordinary, publication.native
	publication.origin.process.probePending = nil
	return report, nil
}

func (e *Engine) discardProbePublication(publication *engineProbePublication) {
	if publication == nil || publication.origin.process == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if publication.origin.process.probePending == publication {
		publication.origin.process.probePending = nil // never clear/reap a successor
	}
}
