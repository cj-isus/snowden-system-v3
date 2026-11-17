package core

import (
	"context"
	"encoding/hex"
	"errors"
	"time"
)

// Parent-only negative reservation. The original process owns this slot, so
// a pending old context callback cannot block or adopt a replacement worker.
// It grants no forwarding, native, path-health or DNS-result authority.
type engineServiceDNSCommand struct {
	origin              engineProcessOrigin
	command             workerRequest
	requestID           uint64 // actual next protocol reservation; compare only against the original process
	requested           bool   // set before a fallible request; only a proven unreserved cancellation clears it
	retirementAttempted bool
}

func (e *Engine) serviceDNSCommand(ctx context.Context, expected RuntimeIdentity, action string) error {
	if action != "dns_pause" && action != "dns_resume" {
		return ErrForwardingNotAdmitted
	}
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return err
	}
	attempt, err := e.beginServiceDNSCommand(expected, action)
	if err != nil {
		return err
	}
	accepted := false
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		defer func() {
			if attempt.origin.process.serviceDNSPending == attempt {
				attempt.origin.process.serviceDNSPending = nil // discard only this original slot, even during failed reap
			}
		}()
		if !accepted {
			_ = e.rejectServiceDNSCommandLocked(attempt, ErrEngineWorkerUnavailable)
		}
	}()
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerControlTimeout)
	if err != nil {
		return err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true // an interrupted parent unlink is never retried
			cancel()
		}
	}
	defer cancelRPC()
	if err := e.serviceDNSCommandPrepared(rpc, attempt); err != nil {
		return err
	}
	cancelRPC() // ordinary return before acceptance, outside Engine.mu
	e.mu.Lock()
	defer e.mu.Unlock()
	cause := contextError(operation)
	if cause == nil {
		switch {
		case !time.Now().Before(rpc.deadline):
			cause = context.DeadlineExceeded
		case !e.serviceDNSCommandCurrentLocked(attempt) || !attempt.origin.process.ready || attempt.origin.process.unusable || attempt.origin.process.exited():
			cause = ErrEngineWorkerUnavailable
		}
	}
	if cause != nil {
		return e.rejectServiceDNSCommandLocked(attempt, cause)
	}
	accepted = true
	return nil
}

// Value-only identity preparation under Engine.mu; arbitrary context creation
// follows outside. A duplicate DNS request cannot overtake this original slot.
func (e *Engine) beginServiceDNSCommand(expected RuntimeIdentity, action string) (*engineServiceDNSCommand, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.isolated || e.process == nil || e.cleanupErr != nil || e.process.instance != expected.Instance || expected.Instance == 0 ||
		!expected.ForwardingAdmitted || expected.AdmissionEpoch == 0 || !validWorkerTag(expected.DefaultOutbound) ||
		e.startPending != nil || e.reloadPending != nil || e.process.serviceDNSPending != nil || !e.process.canReserveStateCommand() ||
		e.nativePermit.busyLocked(e) || e.tunScope != nil {
		return nil, errors.Join(ErrEngineWorkerUnavailable, e.cleanupErr)
	}
	cfg, digest, err := runtimeConfiguration(e.rawJSON)
	if err != nil || digest != expected.ConfigDigest {
		return nil, ErrForwardingNotAdmitted
	}
	selected, tunName := "", ""
	for _, outbound := range cfg.Outbounds {
		if outbound.Tag == "proxy" {
			selected = outbound.Default
		}
	}
	for _, inbound := range cfg.Inbounds {
		if inbound.Type == "tun" {
			if tunName != "" {
				return nil, ErrForwardingNotAdmitted
			}
			tunName = inbound.InterfaceName
		}
	}
	if selected != expected.DefaultOutbound || tunName != expected.TUNName {
		return nil, ErrForwardingNotAdmitted
	}
	attempt := &engineServiceDNSCommand{
		origin:  engineProcessOrigin{process: e.process, instance: expected.Instance, raw: e.rawJSON},
		command: workerRequest{Action: action, Selector: "proxy", Tag: selected, Subject: hex.EncodeToString(digest[:]), AdmissionEpoch: expected.AdmissionEpoch},
	}
	e.process.serviceDNSPending = attempt
	return attempt, nil
}

// Metadata ownership only: an unusable or exited original must still be
// eligible for its existing reaper. Read-only protocol progress is harmless;
// a later non-current command, graph, lifecycle reservation or retained native
// scope supersedes this command, even if the process/config did not change.
func (e *Engine) serviceDNSCommandCurrentLocked(attempt *engineServiceDNSCommand) bool {
	return attempt != nil && attempt.origin.process != nil && attempt.origin.currentLocked(e) && e.cleanupErr == nil &&
		attempt.origin.process.serviceDNSPending == attempt && e.startPending == nil && e.reloadPending == nil &&
		!e.nativePermit.busyLocked(e) && e.tunScope == nil
}

func (e *Engine) rejectServiceDNSCommandLocked(attempt *engineServiceDNSCommand, cause error) error {
	if attempt != nil && attempt.requested && !attempt.retirementAttempted && e.serviceDNSCommandCurrentLocked(attempt) {
		attempt.retirementAttempted = true // no implicit retry after unknown original cleanup
		return e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, cause)
	}
	return errors.Join(ErrEngineWorkerUnavailable, cause) // never reap another graph/process/scope
}

func (e *Engine) serviceDNSCommandPrepared(ctx *workerRPCContext, attempt *engineServiceDNSCommand) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil || !e.serviceDNSCommandCurrentLocked(attempt) {
		return ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return err // no request or protocol ID reserved
	}
	process := attempt.origin.process
	sequence := process.sequence
	attempt.requestID = sequence + 1
	attempt.requested = true
	reply, err := process.request(ctx, attempt.command, workerControlTimeout)
	if process.sourceCanceledBeforeRequest(ctx, sequence, err) {
		attempt.requested = false
		return err // superseded before reservation; original DNS state was not touched
	}
	if err != nil || contextError(ctx) != nil || process.exited() || !validWorkerControlReply(&attempt.command, reply) {
		return e.rejectServiceDNSCommandLocked(attempt, errors.Join(err, contextError(ctx)))
	}
	return nil
}
