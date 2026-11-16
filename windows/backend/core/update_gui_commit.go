package core

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
)

const (
	updateGUICommitOpen uint32 = iota + 1
	updateGUICommitAuthorized
	updateGUICommitClosed
	updateGUICommitTransferred
	updateGUICommitFinalized
)

var ErrUpdateGUIUncertain = errors.New("final GUI command may have been delivered; original retirement required")

type updateGUICommitState struct {
	phase     atomic.Uint32 // one CAS linearizes acceptance against callback completion
	operation context.Context
	process   *engineProcess // original prepared child; never a PID or a caller report

	exchange                                  *updateGUIExchange
	use, ack                                  [updateGUIFrameSize]byte
	callerReturned, useAttempted, ackObserved bool // original owner.mu; effect data, not acceptance
}

func (state *updateGUICommitState) observeFinal() {
	state.useAttempted = state.useAttempted || state.exchange.mayHaveSent()
	state.ackObserved = state.ackObserved || state.exchange.replyMatches(state.use[:], state.ack[:])
}

// UpdateGUICommit is minted only while CommitGUI holds the original owner.
// Accept is value-only, so App may call it under its final lifecycle locks.
// Copies/late callbacks cannot redeem a zero, closed or already accepted grant.
type UpdateGUICommit struct{ state *updateGUICommitState }

func (commit UpdateGUICommit) Accept() error {
	state := commit.state
	if state == nil || state.process == nil {
		return ErrUpdateGUIBootstrap
	}
	if err := contextError(state.operation); err != nil {
		return errors.Join(ErrUpdateGUIBootstrap, err)
	}
	if state.process.exited() || !state.phase.CompareAndSwap(updateGUICommitOpen, updateGUICommitAuthorized) {
		return ErrUpdateGUIBootstrap
	}
	return nil // App authorization only; child still requires the final private frame
}

// UpdateGUIReceipt exists only after the authorized final frame/ACK completed.
// Consume matches the original App grant and finalizes an ownership fact even
// if a later context loss forbids new positive work. It grants no readiness.
type UpdateGUIReceipt struct{ state *updateGUICommitState }

func (receipt UpdateGUIReceipt) Consume(commit UpdateGUICommit) error {
	if receipt.state == nil || receipt.state != commit.state ||
		!receipt.state.phase.CompareAndSwap(updateGUICommitTransferred, updateGUICommitFinalized) {
		return ErrUpdateGUIBootstrap
	}
	return nil
}

// Caller holds owner.mu whenever it reads the decision pointer. Its phase is
// atomic because the finite callback may accept while holding App value locks.
func (transfer *updateGUITransfer) accepted() bool {
	if transfer == nil || transfer.decision == nil {
		return false
	}
	phase := transfer.decision.phase.Load()
	return phase == updateGUICommitTransferred || phase == updateGUICommitFinalized
}

// StartGUI prepares private OWN custody. This step obtains App authorization
// before the purpose's USE frame and returns a receipt after its exact ACK.
// guard may perform authority/image reads; accept only validates values and
// consumes the authorization grant, without native I/O or setting closing.
func (owner *UpdateCodeAcceptance) CommitGUI(ctx context.Context, guard func(context.Context) error, accept func(UpdateGUICommit) error) (receipt UpdateGUIReceipt, finalErr error) {
	if owner == nil || guard == nil || accept == nil {
		return receipt, ErrUpdateGUIBootstrap
	}
	if !owner.mu.TryLock() {
		return receipt, errors.Join(ErrHelperCleanup, ErrHelperBusy) // prepared custody cannot be abandoned on contention
	}
	defer owner.mu.Unlock()
	if !owner.lane.TryLock() {
		return receipt, errors.Join(ErrHelperCleanup, ErrHelperBusy)
	}
	defer owner.lane.Unlock()
	transfer := owner.gui
	if transfer.accepted() {
		return receipt, ErrUpdateGUICommitted
	}
	if !owner.verified || transfer == nil || !transfer.committed || transfer.decision != nil ||
		transfer.process == nil || owner.lane.owner != transfer.process || owner.lane.image == nil {
		return receipt, ErrUpdateGUIBootstrap
	}
	process := transfer.process
	decision := &updateGUICommitState{operation: ctx, process: process}
	decision.phase.Store(updateGUICommitOpen)
	transfer.decision = decision
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrUpdateGUIBootstrap)
		}
		decision.phase.CompareAndSwap(updateGUICommitOpen, updateGUICommitClosed)
		decision.phase.CompareAndSwap(updateGUICommitAuthorized, updateGUICommitClosed)
		if transfer.accepted() {
			// Final wire custody is irreversible. Keep original handles until
			// parent exit; a later source/error cannot adopt it for termination.
			if receipt.state == nil {
				receipt = UpdateGUIReceipt{state: decision} // preserve the completed fact alongside a late error
			}
			if finalErr != nil {
				finalErr = errors.Join(finalErr, ErrUpdateGUICommitted)
			}
			return
		}
		// Recheck after actual retirement too: cancellation may have returned
		// before the original I/O goroutine published a physically read ACK.
		defer func() {
			decision.observeFinal()
			if decision.useAttempted {
				finalErr = errors.Join(finalErr, ErrUpdateGUIUncertain)
			}
		}()
		if owner.lane.owner != process {
			finalErr = errors.Join(finalErr, ErrHelperCleanup, ErrUpdateGUIBootstrap)
			return // an unexpected replacement is not this original abort authority
		}
		process.unusable = true
		if err := retireHelperProcess(&owner.lane); err != nil {
			finalErr = errors.Join(finalErr, ErrHelperCleanup, err) // retain exact original owner on uncertain reap
		} else {
			owner.gui = nil // only confirmed original Job/I/O/handle retirement resolves this preparation
		}
		finalErr = errors.Join(finalErr, process.captureFailure())
	}()
	if process.unusable || process.exited() || process.ioDone == nil {
		return receipt, ErrUpdateGUIBootstrap
	}
	if err := process.ioRetirementReady(); err != nil {
		return receipt, errors.Join(ErrUpdateGUIBootstrap, err)
	}
	if err := process.captureFailure(); err != nil {
		return receipt, errors.Join(ErrUpdateGUIBootstrap, err)
	}
	if err := owner.lane.image.Validate(ctx); err != nil {
		return receipt, err
	}
	if err := helperScope(ctx, guard); err != nil {
		return receipt, err
	}
	if err := process.ancestorScope(ctx, true); err != nil {
		return receipt, err // native query before the value-only App callback
	}
	callbackErr := accept(UpdateGUICommit{state: decision})
	decision.phase.CompareAndSwap(updateGUICommitOpen, updateGUICommitClosed)
	if callbackErr != nil {
		return receipt, callbackErr
	}
	if decision.phase.Load() != updateGUICommitAuthorized {
		return receipt, ErrUpdateGUIBootstrap // returning nil cannot authorize the private original child
	}
	if err := helperScope(ctx, guard); err != nil {
		return receipt, err
	}
	protocol, err := owner.applicationGUIProtocol()
	if err != nil {
		return receipt, err
	}
	use := updateGUIFrame(protocol.use, protocol.binding)
	decision.use, decision.ack = use, updateGUIFrame(protocol.ack, protocol.binding)
	var reply [updateGUIFrameSize]byte
	err = exchangeUpdateGUIObserved(ctx, process.output, process.input, use[:], reply[:], process, &decision.exchange)
	decision.callerReturned = true // the caller may return before physical I/O completes
	decision.observeFinal()
	if err != nil {
		return receipt, err
	}
	ack := decision.ack
	if !bytes.Equal(reply[:], ack[:]) || process.exited() {
		return receipt, ErrUpdateGUIBootstrap
	}
	if err := owner.lane.image.Validate(ctx); err != nil {
		return receipt, err
	}
	if err := helperScope(ctx, guard); err != nil {
		return receipt, err
	}
	if err := process.closePipes(); err != nil {
		return receipt, err
	}
	if err := helperScope(ctx, guard); err != nil {
		return receipt, err
	}
	if err := process.ancestorScope(ctx, true); err != nil {
		return receipt, err // original owner still has not transferred; grant/receipt paths remain value-only
	}
	if process.exited() || !decision.phase.CompareAndSwap(updateGUICommitAuthorized, updateGUICommitTransferred) {
		return receipt, ErrUpdateGUIBootstrap
	}
	receipt = UpdateGUIReceipt{state: decision}
	completed = true
	return receipt, nil
}
