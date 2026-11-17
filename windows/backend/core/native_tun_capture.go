package core

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

// NativeTUNPolicy is a compiled platform boundary, never a Wails/caller-supplied
// permission. Acquire MUST return a partial owner for any claimed resource;
// an ordinary nil/error return asserts that none was acquired. This observation
// primitive does not mint/store a forwarding permit, including for a fake policy.
type NativeTUNPolicy interface {
	Acquire(context.Context, *NativeTUNActor) (NativeTUNLease, error)
}

// Match the existing nativeImagePreparation allocation contract: NewLease is
// pure allocation. The original capture stores the owner before Acquire can
// mutate platform state. A legacy returned partial owner is still supported;
// an interrupted legacy call without an owner cannot prove safe retirement.
type NativeTUNPolicyPreparation interface {
	NativeTUNPolicy
	NewLease(*NativeTUNActor) NativeTUNLeasePreparation
}

type NativeTUNLeasePreparation interface {
	NativeTUNLease
	Acquire(context.Context) error
}

type NativeTUNLease interface {
	Validate(context.Context) error
	Close() error
}

type nativeTUNCapture struct {
	engine         *Engine
	scope          *engineTUNScope
	actor          *NativeTUNActor
	filter         NativeTUNLease
	filterStarted  bool
	filterResolved bool // cleanup/no-acquisition contract only, never forwarding authority
}

func (capture *nativeTUNCapture) acquireFilter(ctx context.Context, policy NativeTUNPolicy) error {
	if capture == nil || capture.actor == nil || policy == nil || capture.filterStarted || capture.filter != nil || contextError(ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	capture.filterStarted = true // retain an unresolved acquisition BEFORE calling policy code
	if prepared, ok := policy.(NativeTUNPolicyPreparation); ok {
		lease := prepared.NewLease(capture.actor)
		capture.filter = lease // original owner before any in-place native preparation
		if lease == nil {
			capture.filterResolved = true // pure allocator returned normally without a resource
			return ErrNativeScopeUnavailable
		}
		return lease.Acquire(ctx)
	}
	var err error
	capture.filter, err = policy.Acquire(ctx, capture.actor)
	if capture.filter == nil && err != nil {
		// Trusted legacy contract: an ordinary nil/error return means no grant;
		// any acquired resource MUST be returned as its partial owner. A panic
		// or Goexit never reaches this assignment and cannot assert absence.
		capture.filterResolved = true
	}
	return err
}

// The protected lane retains this record BEFORE opening either grant. Cleanup
// uses its own context, attempts BOTH revocations, and releases neither the
// process/socket nor its image while either original cleanup is uncertain.
func (capture *nativeTUNCapture) revoke() (result error) {
	if capture == nil {
		return ErrNativeScopeUnavailable
	}
	capture.actor.deactivate()
	sequence := &lifecycleCleanup{}
	defer func() { sequence.drained, result = true, sequence.result }()
	// Register the original metadata borrower before either revocation. It is
	// checked last, so a pending read does not skip negative ingress/filter work,
	// but still prevents the lane from releasing the helper/socket/image.
	defer lifecycleCleanupStep("retire native TUN descriptor", sequence, capture.actor.descriptorRetired)
	// Register BOTH original revocations before either callback. Goexit during
	// ingress retirement still attempts filter cleanup, without releasing the
	// helper/socket/image or declaring the interrupted scope retired.
	defer lifecycleCleanupStep("revoke native TUN filter", sequence, func() error {
		if capture.filter == nil {
			if capture.filterStarted && !capture.filterResolved {
				return ErrNativeScopeUnavailable // no returned original owner/absence receipt
			}
			return nil
		}
		if err := capture.filter.Close(); err != nil {
			return err
		}
		capture.filter, capture.filterResolved = nil, true
		return nil
	})
	defer lifecycleCleanupStep("revoke native TUN ingress", sequence, func() error {
		if capture.scope == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), workerReapTimeout)
		defer cancel()
		if err := capture.engine.retireCaptureTUNScope(ctx, capture.scope); err != nil {
			return err
		}
		capture.scope = nil
		return nil
	})
	return nil
}

// NativeTUNObservation is an HTTP fact from a retired original helper. Its
// original scope-close ACK also requires the actual selected adapter's one-flow
// I/O/callback and both streams to be retired. It is
// deliberately NOT accepted by admitForwarding and is not a Ready/native proof.
// The production final native+outbound+intent commit remains a separate gate.
type NativeTUNObservation struct {
	runtime RuntimeIdentity
	egress  string
	source  *nativeTUNObservationSource
}

func (observation NativeTUNObservation) Runtime() RuntimeIdentity { return observation.runtime }
func (observation NativeTUNObservation) Egress() string           { return observation.egress }

// ObserveNativeTUN owns one fixed operation through the existing protected lane.
// It does not open general ingress/LUID permits and does not issue a worker token.
// Callers must use the compiled native WFP adapter and their actual intent guard.
func (e *Engine) ObserveNativeTUN(parent context.Context, pin string, limit time.Duration, guard func(context.Context) error, policy NativeTUNPolicy) (observation NativeTUNObservation, finalErr error) {
	if e == nil || guard == nil || policy == nil || limit < 500*time.Millisecond || limit > 8*time.Second || limit%time.Millisecond != 0 {
		return observation, ErrNativeScopeUnavailable
	}
	if _, err := nativeImagePin(pin); err != nil {
		return observation, err
	}
	// Reserve the entire fixed HTTP body AND finite IPC exchange before a child
	// or pipe goroutine exists. This is not a kernel/TLS/RSS/elapsed-time bound.
	cost := int64(netprobe.MaxTUNRequest + 6*(netprobe.MaxTUNMessage+4) + netprobe.MaxTUNBody + 1)
	ctx, release, err := AdmitDiagnostic(parent, DiagnosticProtected, 1, cost, limit)
	if err != nil {
		return observation, err
	}
	defer release()
	if !protectedHelper.TryLock() {
		return observation, ErrHelperBusy
	}
	defer protectedHelper.Unlock()
	if err := retireHelperLane(&protectedHelper); err != nil {
		return observation, errors.Join(ErrHelperCleanup, err)
	}
	ownsNew, complete := false, false
	var process *engineProcess // exact original helper retained for late completed I/O cause
	defer func() {
		if recovered := recover(); recovered != nil || !complete && finalErr == nil {
			finalErr = errors.Join(finalErr, ErrNativeScopeUnavailable)
		}
		if ownsNew {
			if retirement := retireHelperLane(&protectedHelper); retirement != nil {
				finalErr = errors.Join(finalErr, ErrHelperCleanup, retirement)
			}
		}
		finalErr = errors.Join(finalErr, process.captureFailure())
		if finalErr == nil {
			finalErr = process.ancestorScope(ctx, false) // source only, after original child/Job/I-O retirement
		}
		if finalErr == nil {
			finalErr = helperScope(ctx, guard)
		}
		if finalErr != nil {
			observation = NativeTUNObservation{}
		}
	}()
	if err := helperScope(ctx, guard); err != nil {
		return observation, err
	}
	identity, err := e.CurrentRuntime(ctx)
	if err != nil || identity.TUNName == "" || identity.AdmissionEpoch == 0 || identity.ForwardingAdmitted {
		return observation, errors.Join(ErrNativeScopeUnavailable, err)
	}
	request, err := netprobe.NewTUNRequest(hex.EncodeToString(identity.ConfigDigest[:]), identity.DefaultOutbound, identity.AdmissionEpoch, int(limit/time.Millisecond))
	if err != nil {
		return observation, err
	}
	image := newNativeImage()
	protectedHelper.image, ownsNew = image, image != nil
	if image == nil {
		return observation, ErrNativeImageUnavailable
	}
	if err := image.prepareCurrent(ctx, pin); err != nil {
		return observation, errors.Join(ErrNativeImageUnavailable, err)
	}
	limits := ownedProcessLimits{processes: 1, commitBytes: helperCaptureCommitBytes, cleanSystemEnvironment: true}
	limits.beforeResume = func(ctx context.Context, host processHost) error {
		if err := image.ValidateProcess(ctx, host); err != nil {
			return err
		}
		return helperScope(ctx, guard)
	}
	arguments, err := netprobe.NewTUNInvocation().Arguments()
	if err != nil {
		return observation, err
	}
	process = newOwnedProcess()
	protectedHelper.owner, ownsNew = process, ownsNew || process != nil
	if process == nil {
		return observation, ErrEngineWorkerUnavailable
	}
	err = process.spawn(ctx, image.Executable(), arguments, limits)
	if err != nil || process == nil {
		return observation, errors.Join(ErrNativeScopeUnavailable, err)
	}
	ready, err := exchangeTUNHelper(ctx, process, request, &request, nil, false)
	if err != nil || ready.Stage != "ready" {
		return observation, errors.Join(netprobe.ErrNativeProtocol, err)
	}
	capture := &nativeTUNCapture{engine: e}
	protectedHelper.tun = capture // retained even if begin/Acquire return a partial owner
	_, err = e.beginTUNScopeOwned(ctx, request, ready.SourcePort, capture)
	if err != nil || capture.scope == nil {
		return observation, errors.Join(ErrNativeScopeUnavailable, err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return observation, ErrNativeScopeUnavailable
	}
	capture.actor = &NativeTUNActor{engine: e, scope: capture.scope, process: process, image: image, runtime: identity, port: ready.SourcePort, until: deadline, operation: ctx, active: true}
	err = capture.acquireFilter(ctx, policy)
	if err != nil || capture.filter == nil {
		return observation, errors.Join(ErrNativeScopeUnavailable, err)
	}
	if err := helperScope(ctx, guard); err != nil {
		return observation, err
	}
	if err := capture.filter.Validate(ctx); err != nil {
		return observation, err
	}
	goMessage := netprobe.NewTUNMessage(request, "go", ready.SourcePort)
	result, err := exchangeTUNHelper(ctx, process, request, nil, &goMessage, false)
	if err != nil || result.Stage != "result" || result.SourcePort != ready.SourcePort {
		return observation, errors.Join(netprobe.ErrNativeProtocol, err)
	}
	if _, err := capture.actor.Descriptor(ctx); err != nil {
		return observation, err
	}
	if err := capture.filter.Validate(ctx); err != nil {
		return observation, err
	}
	if err := helperScope(ctx, guard); err != nil {
		return observation, err
	}
	// Revoke BOTH grants before asking the actor to release its exclusive port.
	originalScope := capture.scope
	if err := capture.revoke(); err != nil {
		return observation, errors.Join(ErrHelperCleanup, err)
	}
	protectedHelper.tun = nil
	releaseMessage := netprobe.NewTUNMessage(request, "release", ready.SourcePort)
	released, err := exchangeTUNHelper(ctx, process, request, nil, &releaseMessage, false)
	if err != nil || released.Stage != "released" || released.SourcePort != ready.SourcePort {
		return observation, errors.Join(netprobe.ErrNativeProtocol, err)
	}
	if _, err := exchangeTUNHelper(ctx, process, request, nil, nil, true); err != nil {
		return observation, err // even one trailing byte invalidates the finite transcript
	}
	intentDone, operationDone, dependencyDone := operationSourceDone(ctx)
	select {
	case <-process.done:
		if process.waitErr != nil {
			return observation, ErrHelperExit
		}
	case <-ctx.Done():
		return observation, ctx.Err()
	case <-intentDone:
		return observation, context.Canceled
	case <-operationDone:
		return observation, context.Canceled
	case <-dependencyDone:
		return observation, context.Canceled
	}
	status, ok := process.host.(interface{ ExitCode() (uint32, error) })
	if !ok {
		return observation, ErrHelperExit
	}
	code, err := status.ExitCode()
	if err != nil || code != 0 {
		return observation, ErrHelperExit
	}
	if err := image.Validate(ctx); err != nil {
		return observation, err
	}
	if err := retireHelperLane(&protectedHelper); err != nil {
		return observation, errors.Join(ErrHelperCleanup, err)
	}
	ownsNew = false
	// Reparse only AFTER original zero exit + complete Job/I/O/image retirement.
	// A helper message, early EOF or successful TLS write is not that receipt.
	address, err := result.ObservedEgress(request)
	if err != nil || address.To4() == nil {
		return observation, netprobe.ErrNativeProtocol
	}
	current, err := e.CurrentRuntime(ctx)
	if err != nil || current != identity {
		return observation, errors.Join(ErrNativeScopeUnavailable, err)
	}
	operation, err := snapshotNativeTUNOperation(parent)
	if err != nil {
		return observation, errors.Join(ErrNativeScopeUnavailable, err)
	}
	observation = NativeTUNObservation{runtime: identity, egress: address.String(),
		source: &nativeTUNObservationSource{engine: e, scope: originalScope, operation: operation, runtime: identity, egress: address.String(), completed: time.Now()}}
	complete = true
	return observation, nil
}

// One original finite pipe exchange. Fields are written only by its I/O
// goroutine and read only after done. No result can be replaced before return.
type nativeTUNExchange struct {
	ownedPipeIO
	message netprobe.TUNMessage
}

// Exactly one bounded exchange owns the original pipes. Wait for that I/O owner
// before replacing ioDone; timeout poisons the protocol until original retirement.
func exchangeTUNHelper(ctx context.Context, process *engineProcess, request netprobe.TUNRequest, initial *netprobe.TUNRequest, command *netprobe.TUNMessage, eof bool) (message netprobe.TUNMessage, finalErr error) {
	if err := contextError(ctx); err != nil {
		return message, errors.Join(ErrNativeScopeUnavailable, err)
	}
	if process == nil || process.unusable || !eof && process.exited() {
		return netprobe.TUNMessage{}, ErrNativeScopeUnavailable
	}
	if err := process.ioRetirementReady(); err != nil {
		process.unusable = true
		return message, err
	}
	// Only the exact release and EOF shapes are negative. A malformed/mixed
	// request cannot borrow their exemption to issue a positive helper command.
	releaseOnly := initial == nil && !eof && command != nil && command.Stage == "release"
	eofOnly := initial == nil && eof && command == nil
	positive := !releaseOnly && !eofOnly
	if positive {
		if err := process.ancestorScope(ctx, true); err != nil {
			process.unusable = true
			return message, errors.Join(ErrNativeScopeUnavailable, err)
		}
	}
	exchange := &nativeTUNExchange{ownedPipeIO: ownedPipeIO{done: make(chan struct{}), effects: new(ownedPipeEffects)}}
	process.captureIO, process.ioDone = &exchange.ownedPipeIO, exchange.done
	intentDone, operationDone, dependencyDone := operationSourceDone(ctx)
	defer func() {
		if finalErr != nil {
			process.unusable = true
			finalErr = errors.Join(finalErr, process.captureFailure())
		}
	}()
	go func() {
		complete := false
		defer func() {
			if recovered := recover(); recovered != nil || !complete {
				exchange.err = errors.Join(exchange.err, netprobe.ErrNativeProtocol)
			}
			close(exchange.done) // publishes message/error; raw-call return has its own original facts
		}()
		exchange.err = contextError(ctx) // goroutine may start after source/deadline revocation
		input := ownedPipeWriter{ctx: ctx, writer: process.input, failure: netprobe.ErrNativeProtocol, effects: exchange.effects}
		output := ownedPipeReader{ctx: ctx, reader: process.output, failure: netprobe.ErrNativeProtocol, effects: exchange.effects}
		if exchange.err == nil {
			if initial != nil {
				exchange.err = netprobe.WriteTUNRequest(input, *initial)
			} else if command != nil {
				exchange.err = netprobe.WriteTUNMessage(input, *command, request)
			}
		}
		if exchange.err == nil {
			exchange.err = contextError(ctx) // no new decoder/EOF read after a revoked write
		}
		if exchange.err == nil {
			if eof {
				var trailing [1]byte
				_, err := io.ReadFull(output, trailing[:])
				if err != io.EOF {
					exchange.err = netprobe.ErrNativeProtocol
				}
			} else {
				exchange.message, exchange.err = netprobe.ReadTUNMessage(output, request)
			}
		}
		exchange.err = errors.Join(exchange.err, contextError(ctx))
		complete = true
	}()
	select {
	case <-exchange.done:
		if err := errors.Join(exchange.completedFailure(), contextError(ctx)); err != nil {
			return message, err
		}
		if positive {
			if err := process.ancestorScope(ctx, true); err != nil {
				return message, errors.Join(ErrNativeScopeUnavailable, err)
			}
		}
		return exchange.message, nil
	case <-ctx.Done():
		return message, ctx.Err()
	case <-intentDone:
		return message, errors.Join(context.Canceled, contextError(ctx))
	case <-operationDone:
		return message, errors.Join(context.Canceled, contextError(ctx))
	case <-dependencyDone:
		return message, errors.Join(context.Canceled, contextError(ctx))
	}
}
