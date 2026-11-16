package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
)

var ErrElevatedGUIUnavailable = errors.New("elevated process request unavailable")
var ErrElevatedGUIDeclined = errors.New("elevation declined")
var ErrElevatedGUITransferred = errors.New("elevated request accepted; original owner retained to parent exit")
var ErrElevatedGUIUncertain = errors.New("final elevated handoff may have taken effect; original retirement required")

// A main dispatcher runs after package initialization and the native loader.
// U1 acceptance cannot attest that earlier execution or erase ranBeforeJob.
// Keep creation/bootstrap containment separate from child wire authorization;
// a smaller helper alone is not proof of original descendant-tree custody.
const elevatedGUIBootstrapAccepted = false

// Parent-side handles alone cannot authorize an elevated child. The private
// child bridge remains unaccepted; W4c alone must not enable this unverified path.
const elevatedGUIChildBridgeAccepted = false

func ElevatedGUIAvailable() bool {
	return NativeTUNForwardingAvailable() && elevatedGUIBootstrapAccepted && elevatedGUIChildBridgeAccepted && brokerParentAvailable()
}

const (
	elevatedNew uint32 = iota
	elevatedPreparing
	elevatedExecuting
	elevatedPrepared
	elevatedAuthorized
	elevatedTransferred
	elevatedFinalized
	elevatedRetiring
	elevatedRetired
)

// Original UAC request plus fixed legacy/broker handoff, never GUI/VPN readiness or signing authority.
// App must publish this pure owner before Execute can acquire native resources.
type ElevatedGUIRequest struct {
	mu        sync.Mutex
	phase     atomic.Uint32
	operation context.Context
	image     nativeImageLease
	native    elevatedGUINativeRequest
	pipe      uacControlPipe
	nonce     [32]byte
	broker    *brokerGUIParent

	finalExchange              controlpipe.ExchangeReceipt
	finalUse, finalAck         [updateGUIFrameSize]byte
	finalReturned, ackObserved bool // effect facts under original owner.mu, never transfer authority
}

func NewElevatedGUIRequest() *ElevatedGUIRequest { return &ElevatedGUIRequest{} }

type ElevatedGUIReceipt struct{ owner *ElevatedGUIRequest }
type ElevatedGUICommit struct{ owner *ElevatedGUIRequest }

func (commit ElevatedGUICommit) Accept() error {
	if commit.owner == nil || contextError(commit.owner.operation) != nil || !commit.owner.phase.CompareAndSwap(elevatedPrepared, elevatedAuthorized) {
		return ErrElevatedGUIUnavailable
	}
	return nil
}

// Final wire custody is an irreversible fact, matched to the exact original
// App grant. A later revoked operation cannot reopen the old App or erase it.
func (receipt ElevatedGUIReceipt) Consume(original *ElevatedGUIRequest, commit ElevatedGUICommit) error {
	if original == nil || receipt.owner != original || commit.owner != original ||
		!original.phase.CompareAndSwap(elevatedTransferred, elevatedFinalized) {
		return ErrElevatedGUIUnavailable
	}
	return nil
}

func (owner *ElevatedGUIRequest) Transferred() bool {
	if owner == nil {
		return false
	}
	phase := owner.phase.Load()
	return phase == elevatedTransferred || phase == elevatedFinalized
}

func (owner *ElevatedGUIRequest) scope(ctx context.Context, guard func(context.Context) error) error {
	if err := contextError(owner.operation); err != nil {
		return err
	}
	phase := owner.phase.Load()
	if phase != elevatedPreparing && phase != elevatedExecuting && phase != elevatedPrepared && phase != elevatedAuthorized {
		return ErrElevatedGUIUnavailable
	}
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	if owner.phase.Load() != phase {
		return ErrElevatedGUIUnavailable
	}
	return contextError(owner.operation)
}

func (owner *ElevatedGUIRequest) Execute(ctx context.Context, resume bool, guard func(context.Context) error) (finalErr error) {
	if owner == nil || guard == nil || !owner.mu.TryLock() {
		return ErrElevatedGUIUnavailable
	}
	defer owner.mu.Unlock()
	if !owner.phase.CompareAndSwap(elevatedNew, elevatedPreparing) {
		return ErrElevatedGUIUnavailable
	}
	owner.operation = ctx
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrElevatedGUIUnavailable)
		}
		if finalErr != nil {
			owner.phase.CompareAndSwap(elevatedPreparing, elevatedRetiring)
			owner.phase.CompareAndSwap(elevatedExecuting, elevatedRetiring)
			owner.phase.CompareAndSwap(elevatedPrepared, elevatedRetiring)
		}
	}()
	if !ElevatedGUIAvailable() {
		return ErrElevatedGUIUnavailable
	}
	if err := owner.scope(ctx, guard); err != nil {
		return err
	}
	if err := owner.prepareImage(ctx); err != nil {
		return err
	}
	if err := owner.scope(ctx, guard); err != nil {
		return err
	}
	if _, err := rand.Read(owner.nonce[:]); err != nil {
		return err
	}
	binding := hex.EncodeToString(owner.nonce[:])
	check := func(ctx context.Context) error { return owner.scope(ctx, guard) }
	if err := owner.scope(ctx, guard); err != nil {
		return err
	}
	if err := owner.listen(ctx, binding, check); err != nil {
		return err
	}
	if !owner.phase.CompareAndSwap(elevatedPreparing, elevatedExecuting) {
		return ErrElevatedGUIUnavailable
	}
	if err := owner.executeNative(ctx, resume, binding, check); err != nil {
		return err
	}
	if err := owner.scope(ctx, guard); err != nil {
		return err
	}
	if err := owner.validateNative(ctx); err != nil {
		return err
	}
	if err := owner.scope(ctx, guard); err != nil {
		return err
	}
	live := func(ctx context.Context) error {
		if err := owner.scope(ctx, guard); err != nil {
			return err
		}
		return owner.alive(ctx)
	}
	if err := owner.acceptPipe(ctx, live); err != nil {
		return err
	}
	protocol, err := owner.protocol(binding)
	if err != nil {
		return err
	}
	seed, err := owner.seed(protocol)
	if err != nil {
		return err
	}
	var reply [updateGUIFrameSize]byte
	if err := owner.exchange(ctx, seed, reply[:], live); err != nil {
		return err
	}
	armed := updateGUIFrame(protocol.armed, protocol.binding)
	if !bytes.Equal(reply[:], armed[:]) {
		return ErrElevatedGUIUnavailable
	}
	goFrame := updateGUIFrame(protocol.goStage, protocol.binding)
	if err := owner.exchange(ctx, goFrame[:], reply[:], live); err != nil {
		return err
	}
	owned := updateGUIFrame(protocol.owned, protocol.binding)
	if !bytes.Equal(reply[:], owned[:]) {
		return ErrElevatedGUIUnavailable
	}
	if err := owner.validateNative(ctx); err != nil {
		return err
	}
	if err := live(ctx); err != nil {
		return err
	}
	if !owner.phase.CompareAndSwap(elevatedExecuting, elevatedPrepared) {
		return ErrElevatedGUIUnavailable
	}
	completed = true
	return nil
}

func (owner *ElevatedGUIRequest) Commit(ctx context.Context, guard func(context.Context) error, accept func(ElevatedGUICommit) error) (receipt ElevatedGUIReceipt, finalErr error) {
	if owner == nil || guard == nil || accept == nil || !owner.mu.TryLock() {
		return receipt, ErrElevatedGUIUnavailable
	}
	defer owner.mu.Unlock()
	if owner.phase.Load() != elevatedPrepared {
		return receipt, ErrElevatedGUIUnavailable
	}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrElevatedGUIUnavailable)
		}
		if owner.Transferred() {
			if receipt.owner == nil {
				receipt = ElevatedGUIReceipt{owner: owner}
			}
			if finalErr != nil {
				finalErr = errors.Join(finalErr, ErrElevatedGUITransferred)
			}
			return
		}
		owner.ackObserved = owner.ackObserved || owner.finalExchange.ReplyMatches(owner.finalUse[:], owner.finalAck[:])
		if owner.finalExchange.MayHaveSent() {
			finalErr = errors.Join(finalErr, ErrElevatedGUIUncertain, ErrHelperCleanup)
		}
		owner.phase.CompareAndSwap(elevatedPrepared, elevatedRetiring)
		owner.phase.CompareAndSwap(elevatedAuthorized, elevatedRetiring)
	}()
	if err := owner.scope(ctx, guard); err != nil {
		return receipt, err
	}
	if err := owner.validateNative(ctx); err != nil {
		return receipt, err
	}
	if err := accept(ElevatedGUICommit{owner: owner}); err != nil {
		return receipt, err
	}
	if owner.phase.Load() != elevatedAuthorized {
		return receipt, ErrElevatedGUIUnavailable
	}
	live := func(ctx context.Context) error {
		if err := owner.scope(ctx, guard); err != nil {
			return err
		}
		return owner.alive(ctx)
	}
	protocol, err := owner.protocol(hex.EncodeToString(owner.nonce[:]))
	if err != nil {
		return receipt, err
	}
	use := updateGUIFrame(protocol.use, protocol.binding)
	owner.finalUse, owner.finalAck = use, updateGUIFrame(protocol.ack, protocol.binding)
	var reply [updateGUIFrameSize]byte
	err = owner.exchangeFinal(ctx, use[:], reply[:], live)
	owner.finalReturned = true // actual exchange return, including an error after native I/O
	owner.ackObserved = owner.finalExchange.ReplyMatches(owner.finalUse[:], owner.finalAck[:])
	if err != nil {
		return receipt, err
	}
	ack := owner.finalAck
	if !bytes.Equal(reply[:], ack[:]) {
		return receipt, ErrElevatedGUIUnavailable
	}
	if err := owner.validateNative(ctx); err != nil {
		return receipt, err
	}
	if err := live(ctx); err != nil {
		return receipt, err
	}
	if err := owner.closePipe(); err != nil {
		return receipt, err
	}
	if err := owner.afterACK(ctx, guard); err != nil {
		return receipt, err
	}
	if !owner.phase.CompareAndSwap(elevatedAuthorized, elevatedTransferred) {
		return receipt, ErrElevatedGUIUnavailable
	}
	receipt, completed = ElevatedGUIReceipt{owner: owner}, true
	return receipt, nil
}

func (owner *ElevatedGUIRequest) Close() (finalErr error) {
	if owner == nil {
		return nil
	}
	for {
		phase := owner.phase.Load()
		if phase == elevatedTransferred || phase == elevatedFinalized {
			return ErrElevatedGUITransferred
		}
		if phase == elevatedRetiring || phase == elevatedRetired {
			break
		}
		if owner.phase.CompareAndSwap(phase, elevatedRetiring) {
			break
		}
	}
	if !owner.mu.TryLock() {
		return errors.Join(ErrHelperCleanup, ErrHelperBusy)
	}
	defer owner.mu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrHelperCleanup)
		}
	}()
	if err := errors.Join(owner.closePipe(), owner.retireNative()); err != nil {
		return errors.Join(ErrHelperCleanup, err)
	}
	if owner.image != nil {
		if err := owner.image.Close(); err != nil {
			return errors.Join(ErrHelperCleanup, err)
		}
		owner.image = nil
	}
	owner.phase.Store(elevatedRetired)
	completed = true
	return nil
}
