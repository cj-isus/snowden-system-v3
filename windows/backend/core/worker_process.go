package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
)

// processHost owns original kernel handles, never a saved PID. A native host
// may also expose resources acquired before process creation via preparation.
// Empty must cover the whole job, not merely the root process's exit.
type processHost interface {
	Wait() error
	Terminate() error
	Empty() (bool, error)
	Close() error
}

// Native preparation may own resources before any process was created. Hosts
// without this interface keep their existing already-created process contract.
type processPreparationHost interface {
	ProcessCreated() bool
	EnsureWait(*engineProcess)
	ClosePreparation() error
}

// A bounded verifier must establish its actual Job budget and final accounting
// before any captured receipt can escape. Exit status alone is insufficient.
type processResourceHost interface {
	ValidateResourceUsage() error
}

const ownedProcessUserTimeTick = spawnprep.UserTimeTick

type ownedProcessLimits struct {
	processes              uint32
	commitBytes            uintptr       // zero preserves the engine's existing memory contract
	userTime               time.Duration // aggregate Job user-mode CPU time; zero leaves it unchanged
	cleanSystemEnvironment bool
	deliveryVerifier       bool                                     // exact public-input SDK pipe worker; no application store environment
	applicationEnvironment bool                                     // fixed updater check/GUI modes; explicit public store location
	successorGUI           bool                                     // fixed bootstrap only; owns a lifetime Job, not a one-process helper
	beforeResume           func(context.Context, processHost) error // original suspended owner only
}

// Lifecycle methods run under Engine.mu or the dedicated helper admission.
// One Wait and one pipe-I/O goroutine have explicit completion channels.
// An uncertain owner is retained and prevents replacement within that owner.
type engineProcess struct {
	instance       uint64
	ancestor       guiAncestor // exact process bootstrap source, captured once before native acquisition
	successorGUI   bool        // original spawn role; successor leaves the old GUI role Job
	host           processHost
	input          io.WriteCloser
	output         io.ReadCloser
	done           chan struct{}
	waitErr        error // published by close(done)
	ioDone         chan struct{}
	captureIO      *ownedPipeIO // original worker/helper/TUN I/O result published only by its done
	sequence       uint64
	ready          bool
	spawnAttempted bool // original one-attempt native preparation, never reset after cleanup
	unusable       bool
	pipesClosed    bool
	drained        bool // no process created, or exit + I/O completion + empty job established
}

var engineProcessSequence atomic.Uint64

func (p *engineProcess) awaitExit() {
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			p.waitErr = errors.New("engine worker wait did not return normally")
		}
		close(p.done) // waiter completion, never physical exit without a successful Wait
	}()
	p.waitErr = p.host.Wait()
	returned = true
}

// An ended waiter can also carry waitErr. Positive users must reject it, and
// retirement still requires successful Wait, original I/O and empty Job facts.
func (p *engineProcess) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// Bind the original result header to its actual completion channel. A closed
// unrelated channel, a skipped registration or goroutine-end defer cannot
// authorize another exchange or release this process's native/image custody.
func (p *engineProcess) ioRetirementReady() error {
	if p == nil {
		return ErrHelperCleanup
	}
	if p.captureIO == nil && p.ioDone == nil {
		return nil
	}
	if p.captureIO == nil || p.ioDone == nil || p.captureIO.done != p.ioDone {
		return ErrHelperCleanup
	}
	return p.captureIO.retirementReady()
}

// A single exchange owns both pipes. Timeout invalidates the protocol instead
// of allowing a retry against an unknown command result. Retirement follows.
func (p *engineProcess) exchange(ctx context.Context, request *workerRequest, limit time.Duration) (workerReply, error) {
	if p.unusable || p.exited() {
		return workerReply{}, ErrEngineWorkerUnavailable
	}
	if err := p.ioRetirementReady(); err != nil {
		p.unusable = true
		return workerReply{}, errors.Join(errWorkerProtocol, err)
	}
	if err := contextError(ctx); err != nil {
		p.unusable = true // a request may already have reserved its sequence
		return workerReply{}, err
	}
	// Revocation/Stop must remain possible after a root query fails. All other
	// commands, including hello and admission, require the original custody.
	positive := request == nil || request.Action != "stop" && request.Action != "dns_pause" && request.Action != "tun_scope_close"
	if positive {
		if err := p.ancestorScope(ctx, true); err != nil {
			p.unusable = true
			return workerReply{}, err
		}
	}
	if request != nil && workerStateReplyAction(request.Action) && !request.replyState.matches(p, request) {
		p.unusable = true
		return workerReply{}, errWorkerProtocol // no unbound/stale state command reaches either pipe
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	intentDone, operationDone, dependencyDone := operationSourceDone(ctx)
	type workerExchange struct {
		*ownedPipeIO // finite header/effects only; no pointer back to the potentially large reply
		reply        workerReply
	}
	exchange := &workerExchange{ownedPipeIO: &ownedPipeIO{done: make(chan struct{}), effects: new(ownedPipeEffects)}}
	p.captureIO, p.ioDone = exchange.ownedPipeIO, exchange.done
	go func() {
		finished := false
		defer func() {
			if recovered := recover(); recovered != nil || !finished {
				exchange.err = errors.Join(exchange.err, errWorkerProtocol)
			}
			close(exchange.done) // goroutine end; original raw pending/return facts remain separate
		}()
		exchange.err = contextError(ctx) // the goroutine may start after intent/path revocation
		if exchange.err == nil && request != nil {
			exchange.err = writeWorkerFrame(ownedPipeWriter{ctx: ctx, writer: p.input, failure: errWorkerProtocol, effects: exchange.effects}, request, maxWorkerRequestBytes)
		}
		if exchange.err == nil {
			exchange.err = readWorkerFrame(ownedPipeReader{ctx: ctx, reader: p.output, failure: errWorkerProtocol, effects: exchange.effects}, &exchange.reply, maxWorkerReplyBytes)
		}
		if exchange.err != nil {
			if !errors.Is(exchange.err, context.Canceled) && !errors.Is(exchange.err, context.DeadlineExceeded) {
				exchange.err = errWorkerProtocol // omit raw pipe/codec values and paths
			}
		} else {
			expectedID := uint64(0)
			if request != nil {
				expectedID = request.ID
			}
			if exchange.reply.Version != workerProtocolVersion || exchange.reply.ID != expectedID || !validNativePreparedReply(request, exchange.reply) {
				exchange.err = errWorkerProtocol // retain correlation failure even if the caller is canceled
			} else {
				exchange.err = workerCommandReplyFailure(request, exchange.reply)
			}
		}
		finished = true
	}()
	var waitErr error
	select {
	case <-exchange.done:
	case <-intentDone:
		waitErr = context.Canceled
	case <-operationDone:
		waitErr = context.Canceled
	case <-dependencyDone:
		waitErr = context.Canceled
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	if err := errors.Join(waitErr, contextError(ctx), exchange.completedFailure()); err != nil {
		p.unusable = true
		return workerReply{}, err // only original-owner retirement may follow; no stale reply escapes
	}
	if positive {
		if err := p.ancestorScope(ctx, true); err != nil {
			p.unusable = true
			return workerReply{}, err // no Start/admission observation after original root custody failed
		}
	}
	return exchange.reply, nil // done published it and the original sources still permit observation
}

func (p *engineProcess) request(ctx context.Context, request workerRequest, limit time.Duration) (workerReply, error) {
	if err := contextError(ctx); err != nil {
		return workerReply{}, err // do not reserve an ID for already-revoked work
	}
	if !p.ready || p.sequence == ^uint64(0) {
		return workerReply{}, errWorkerProtocol
	}
	p.sequence++
	request.Version, request.ID = workerProtocolVersion, p.sequence
	return p.exchange(ctx, &request, limit)
}

// Caller holds the owning Engine lock and captured sequence before request.
// A revoked original source before reservation has neither used a protocol ID
// nor started this exchange. Any reservation/unusable/exit remains uncertain
// and must follow the existing original-process retirement path.
func (p *engineProcess) sourceCanceledBeforeRequest(ctx context.Context, sequence uint64, err error) bool {
	return p != nil && errors.Is(err, context.Canceled) && !OperationSourcesCurrent(ctx) &&
		p.sequence == sequence && !p.unusable && !p.exited()
}

// This cleanup budget is independent of the canceled operation. A deadline is
// not evidence of exit: retain handles/owner on any uncertain reap or job query.
func (p *engineProcess) retire() (result error) {
	p.unusable = true
	preparation, preparing := p.host.(processPreparationHost)
	resources := &lifecycleCleanup{}
	resourcesAttempted := false
	closeResources := func() error {
		if !resourcesAttempted {
			resourcesAttempted = true // no second callback after an unknown first attempt
			lifecycleCleanupStep("worker original preparation and pipes", resources, func() error {
				return p.closePreparationAndPipes(preparation)
			})
		}
		return resources.result
	}
	// Register before any host callback: an early creation/wait/termination
	// failure still attempts the independent original endpoints. Keep their
	// result even if that attempt itself exits its goroutine during unwind.
	defer func() {
		if err := resources.result; err != nil && !errors.Is(result, err) {
			result = errors.Join(result, err)
		}
	}()
	defer func() { _ = closeResources() }()
	if p.drained {
		if err := closeResources(); err != nil {
			return err
		}
		if err := p.ioRetirementReady(); err != nil {
			return err
		}
		return p.host.Close() // confirmed-closed slots are skipped; unknown native Close is not retried
	}
	if preparing && !preparation.ProcessCreated() {
		// No synthetic exit, PID poll, Wait or Terminate against a nonexistent
		// process. The complete original preparation and I/O owner still closes.
		if err := closeResources(); err != nil {
			return err
		}
		if err := p.ioRetirementReady(); err != nil {
			return err
		}
		p.drained = true
		return p.host.Close()
	}
	if preparing {
		preparation.EnsureWait(p) // repairs a post-Create interruption before waiter setup
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerReapTimeout)
	defer cancel()
	// Terminate the owned job even after graceful root exit: descendants may
	// otherwise keep resources/pipes alive. No PID lookup or taskkill.
	if err := p.host.Terminate(); err != nil {
		empty, queryErr := p.host.Empty()
		if !p.exited() || queryErr != nil || !empty {
			return errors.New("engine worker termination not confirmed")
		}
	}
	resourceErr := closeResources() // normal order stays terminate -> pipes -> Wait/Job
	select {
	case <-p.done:
		if p.waitErr != nil {
			return errors.New("engine worker wait not confirmed")
		}
	case <-ctx.Done():
		return errors.New("engine worker reap deadline exceeded")
	}
	if p.ioDone != nil {
		select {
		case <-p.ioDone:
		case <-ctx.Done():
			return errors.New("engine worker I/O retirement not confirmed")
		}
	}
	if err := p.ioRetirementReady(); err != nil {
		return err
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		empty, err := p.host.Empty()
		if err != nil {
			return errors.New("engine worker job query failed")
		}
		if empty {
			p.drained = true
			if resourceErr != nil {
				return resourceErr // drained root does not prove file/auxiliary closure
			}
			return p.host.Close()
		}
		select {
		case <-ctx.Done():
			return errors.New("engine worker job still owns processes")
		case <-ticker.C:
		}
	}
}

// Keep the parent's original endpoints in the unwind even if child preparation
// cleanup exits its goroutine. Each native slot still enforces its own custody;
// running all deferred steps does not prove that a failed resource was closed.
func (p *engineProcess) closePreparationAndPipes(preparation processPreparationHost) (result error) {
	sequence := &lifecycleCleanup{}
	defer func() { result = sequence.result }()
	defer lifecycleCleanupStep("worker parent pipes", sequence, p.closePipes)
	if preparation != nil {
		defer lifecycleCleanupStep("worker child preparation", sequence, preparation.ClosePreparation)
	}
	return nil
}

func (p *engineProcess) closePipes() (result error) {
	if p.pipesClosed {
		return nil
	}
	sequence := &lifecycleCleanup{}
	defer func() {
		result = sequence.result
		if result == nil {
			p.pipesClosed = true
		}
	}()
	// Register both original endpoints before calling either Close. Goexit in
	// input.Close still attempts output.Close and keeps a negative receipt.
	if p.output != nil {
		defer lifecycleCleanupStep("worker output pipe", sequence, p.output.Close)
	}
	if p.input != nil {
		defer lifecycleCleanupStep("worker input pipe", sequence, p.input.Close)
	}
	return nil
}

func (e *Engine) startIsolatedLocked(ctx context.Context) error {
	e.clearForwardingPermitsLocked()
	if e.process != nil {
		return ErrEngineCleanupFailed // original process must be retired before replacement
	}
	subject, subjectErr := configurationSubject(e.rawJSON)
	if subjectErr != nil {
		return subjectErr
	}
	if len(e.rawJSON) == 0 || len(e.rawJSON) > maxWorkerConfigBytes {
		return errors.New("engine worker config exceeds allowed bounds")
	}
	process := newOwnedProcess()
	e.process = process // before any native preparation or process creation
	var err error
	if process == nil {
		err = ErrEngineWorkerUnavailable
	}
	if process != nil {
		process.instance = engineProcessSequence.Add(1)
		if process.instance == 0 {
			err = errors.New("engine instance identity exhausted")
		}
	}
	if err == nil {
		err = process.spawnEngine(ctx)
	}
	if err == nil {
		var reply workerReply
		reply, err = process.exchange(ctx, nil, workerHelloTimeout)
		if err == nil && !validWorkerHelloReply(reply) {
			err = errWorkerProtocol
		}
		if err == nil {
			process.ready = true
			command := workerRequest{Action: "start", Raw: e.rawJSON}
			command.replyState, err = prepareWorkerStateReply(process, e.rawJSON, command)
			if err == nil {
				reply, err = process.request(ctx, command, workerStartTimeout)
			}
			if err == nil && (reply.Code != "ok" || reply.Previous != "" || !validWorkerState(reply) || reply.Subject != subject || reply.Admitted) {
				err = errors.New("engine worker start rejected")
			}
		}
	}
	if err != nil || contextError(ctx) != nil {
		return e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, err, contextError(ctx))
	}
	return nil
}

func (e *Engine) retireIsolatedLocked() error {
	e.clearForwardingPermitsLocked()
	if e.process == nil {
		return e.cleanupErr
	}
	if err := e.process.retire(); err != nil {
		e.cleanupErr = errors.Join(ErrEngineCleanupFailed, err)
		return e.cleanupErr // keep process ownership and forbid new Start/Release
	}
	if e.tunScope != nil && e.tunScope.process == e.process {
		e.tunScope.retired = true // exact original Job + I/O + handle retirement receipt
	}
	e.process = nil
	e.tunScope = nil // confirmed original Job retirement reclaims any remote handle
	e.cleanupErr = nil
	return nil
}

// Preserve the operation's late original I/O failure after the independent reap
// has had its bounded chance to finish it. This is NOT a cleanup receipt: a
// successfully retired Job still clears cleanupErr even when the command failed.
func (e *Engine) retireIsolatedFailureLocked(causes ...error) error {
	original := e.process
	retirement := e.retireIsolatedLocked()
	return errors.Join(errors.Join(causes...), retirement, original.captureFailure())
}

func (e *Engine) stopIsolatedLocked() error {
	if e.process == nil {
		return e.cleanupErr
	}
	if e.process.ready && !e.process.unusable && !e.process.exited() {
		_, _ = e.process.request(context.Background(), workerRequest{Action: "stop"}, workerCloseTimeout)
	}
	// A close reply is not cleanup proof; kernel handles/job accounting decide.
	return e.retireIsolatedLocked()
}

func (e *Engine) isolatedCommandLocked(ctx context.Context, request workerRequest) (workerReply, error) {
	if request.Action != "current" && request.Action != "switch" || request.Selector != "proxy" {
		return workerReply{}, ErrSelectorSwitchUnavailable
	}
	if e.process == nil {
		return workerReply{}, ErrEngineNotRunning
	}
	if !e.process.canReserveStateCommand() {
		return workerReply{}, e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable)
	}
	if err := contextError(ctx); err != nil {
		return workerReply{}, err // no state command was prepared or reserved
	}
	if request.replyState == nil {
		var err error
		request.replyState, err = prepareWorkerStateReply(e.process, e.rawJSON, request)
		if err != nil {
			return workerReply{}, e.retireIsolatedFailureLocked(err)
		}
	}
	sequence := e.process.sequence
	reply, err := e.process.request(ctx, request, workerControlTimeout)
	if err != nil {
		if e.process.sourceCanceledBeforeRequest(ctx, sequence, err) {
			return workerReply{}, err // no command issued; do not poison a healthy predecessor
		}
		return workerReply{}, e.retireIsolatedFailureLocked(ErrEngineWorkerUnavailable, err)
	}
	if contextError(ctx) != nil || e.process.exited() || workerStateReplyFailure(&request, reply) != nil {
		return workerReply{}, e.retireIsolatedFailureLocked(errWorkerProtocol, contextError(ctx))
	}
	// Original before/after digests and old selector were captured before IPC.
	// Successful switch publication still belongs to the caller holding e.mu.
	switch reply.Code {
	case "ok":
		if !validWorkerTag(reply.Previous) {
			return workerReply{}, e.retireIsolatedFailureLocked(errWorkerProtocol)
		}
		return reply, nil
	case "selector_unavailable":
		if reply.Previous != "" {
			return workerReply{}, e.retireIsolatedFailureLocked(errWorkerProtocol)
		}
		return reply, ErrSelectorSwitchUnavailable
	default:
		return workerReply{}, e.retireIsolatedFailureLocked(fmt.Errorf("%w: command rejected", ErrEngineWorkerUnavailable))
	}
}

func validWorkerState(reply workerReply) bool {
	return workerSubject(reply.Subject) && reply.AdmissionEpoch != 0 && reply.AdmissionToken == "" && reply.NativeAdmissionToken == "" && reply.Report == nil && reply.ScopeNonce == ""
}
