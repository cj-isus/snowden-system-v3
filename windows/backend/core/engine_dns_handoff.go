package core

import (
	"context"
	"encoding/hex"
	"errors"
)

// These commands gate service-DNS work only. They cannot open ingress, mint a
// diagnostic/native token, select a channel or reset established SDK transports.
func (e *Engine) PauseServiceDNS(ctx context.Context, expected RuntimeIdentity) error {
	return e.serviceDNSCommand(ctx, expected, "dns_pause")
}

func (e *Engine) ResumeServiceDNS(ctx context.Context, expected RuntimeIdentity) error {
	return e.serviceDNSCommand(ctx, expected, "dns_resume")
}

func (e *Engine) serviceDNSCommand(ctx context.Context, expected RuntimeIdentity, action string) error {
	if action != "dns_pause" && action != "dns_resume" {
		return ErrForwardingNotAdmitted
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	if !e.isolated || e.process == nil || e.cleanupErr != nil || e.process.instance != expected.Instance || expected.Instance == 0 ||
		!expected.ForwardingAdmitted || expected.AdmissionEpoch == 0 || !validWorkerTag(expected.DefaultOutbound) ||
		e.nativePermit.busyLocked(e) || e.tunScope != nil {
		return errors.Join(ErrEngineWorkerUnavailable, e.cleanupErr)
	}
	cfg, digest, err := runtimeConfiguration(e.rawJSON)
	if err != nil || digest != expected.ConfigDigest {
		return ErrForwardingNotAdmitted
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
				return ErrForwardingNotAdmitted
			}
			tunName = inbound.InterfaceName
		}
	}
	if selected != expected.DefaultOutbound || tunName != expected.TUNName {
		return ErrForwardingNotAdmitted
	}
	subject := hex.EncodeToString(digest[:])
	sequence := e.process.sequence
	command := workerRequest{Action: action, Selector: "proxy", Tag: selected, Subject: subject, AdmissionEpoch: expected.AdmissionEpoch}
	reply, err := e.process.request(ctx, command, workerControlTimeout)
	if e.process.sourceCanceledBeforeRequest(ctx, sequence, err) {
		return err // superseded before reservation; original DNS state was not touched
	}
	if err != nil || contextError(ctx) != nil || e.process.exited() || !validWorkerControlReply(&command, reply) {
		return e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, err, contextError(ctx))
	}
	return nil
}

// Original child-side box/selector/admission check, never a public report's
// assertions. Pending receipts are revoked; existing admitted state is retained.
func (e *Engine) localServiceDNS(ctx context.Context, request workerRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	subject, selected, box, _, err := e.forwardingSubjectLocked()
	if err != nil || box == nil || box.bootstrap == nil || subject != request.Subject || selected != request.Tag {
		return ErrForwardingNotAdmitted
	}
	gate := box.admission
	gate.mu.Lock()
	valid := !gate.closed && gate.open.Load() && gate.epoch == request.AdmissionEpoch && gate.tunScope == nil
	if valid {
		gate.clearPendingLocked()
		gate.nativeReceipt = nil
	}
	gate.mu.Unlock()
	if !valid {
		return ErrForwardingNotAdmitted
	}
	if request.Action == "dns_pause" {
		err = box.bootstrap.pauseNetwork(ctx)
	} else if request.Action == "dns_resume" {
		err = box.bootstrap.resumeNetwork(ctx)
	} else {
		return ErrForwardingNotAdmitted
	}
	if err != nil && request.Version == workerProtocolVersion && request.ID != 0 && reportLifecycleError(workerProbeFailure(err)) != nil {
		// Only this actual validated original bootstrap call creates the private
		// fault. Request-shaped public data alone cannot claim this work occurred.
		return &workerDNSFault{id: request.ID, action: request.Action, subject: subject, selector: selected, epoch: request.AdmissionEpoch, cause: err}
	}
	return err
}

type workerDNSFault struct {
	id                        uint64
	action, subject, selector string
	epoch                     uint64
	cause                     error
}

func (*workerDNSFault) Error() string { return "original worker DNS operation failed" }

func (fault *workerDNSFault) Unwrap() error {
	if fault == nil {
		return nil
	}
	return fault.cause
}

func workerDNSFaultReport(request workerRequest, cause error) *ProbeReport {
	var fault *workerDNSFault
	if !errors.As(cause, &fault) || fault == nil || request.Version != workerProtocolVersion || request.ID == 0 || request.ID != fault.id ||
		request.Action != fault.action || request.Subject != fault.subject || request.Tag != fault.selector || request.AdmissionEpoch != fault.epoch {
		return nil
	}
	report := workerProbeFailure(fault.cause)
	if reportLifecycleError(report) == nil {
		return nil
	}
	return report // fixed finite negative shape, no original error/path/result bytes
}
