package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// These values describe only the live original actor. Editing a returned copy
// cannot change the actor or install a grant. No PID or transferable handle is
// exposed; the worker receives its original PROCESS handle privately instead.
type NativeTUNDescriptor struct {
	Executable string
	Runtime    RuntimeIdentity
	SourcePort uint16
}

type nativeTUNActorReadStage uint8

const (
	nativeTUNActorReadIdle nativeTUNActorReadStage = iota
	nativeTUNActorReadContext
	nativeTUNActorReadRuntime
	nativeTUNActorReadImage
	nativeTUNActorReadAncestor
	nativeTUNActorReadExecutable
	nativeTUNActorReadComplete
	nativeTUNActorReadFinalFence
)

// One original read, embedded in the actor before any external call. Only its
// admitted caller writes the payload; pending/fault and reuse are under mu.
// An interrupted read retains its original contexts/cancel/stage, never a
// detached callback, replacement waiter or fabricated retirement receipt.
type nativeTUNActorRead struct {
	caller         context.Context
	ctx            context.Context
	cancel         context.CancelFunc
	deadline       time.Time
	hasDeadline    bool
	stage          nativeTUNActorReadStage
	pending        bool
	fault          bool
	cancelStarted  bool
	cancelReturned bool
}

type NativeTUNActor struct {
	mu        sync.Mutex
	engine    *Engine
	scope     *engineTUNScope
	process   *engineProcess
	image     nativeImageLease
	runtime   RuntimeIdentity
	port      uint16
	until     time.Time
	operation context.Context // original bounded capture; these source fields are immutable after publication
	active    bool
	revoked   atomic.Bool // one-way negative fence; never proof of a valid actor or descriptor
	read      nativeTUNActorRead
}

// Revoked observes only this original actor's negative lifetime. A false result
// supplies no runtime/image/process proof and cannot authorize a grant by itself.
func (actor *NativeTUNActor) Revoked() bool {
	return actor == nil || actor.revoked.Load()
}

func (actor *NativeTUNActor) Descriptor(ctx context.Context) (descriptor NativeTUNDescriptor, finalErr error) {
	if actor.Revoked() {
		return descriptor, ErrNativeScopeUnavailable
	}
	actor.mu.Lock()
	if !actor.active || actor.Revoked() || actor.read.fault {
		actor.mu.Unlock()
		return descriptor, ErrNativeScopeUnavailable
	}
	if actor.read.pending {
		actor.mu.Unlock()
		return descriptor, errors.Join(ErrNativeScopeUnavailable, ErrHelperBusy)
	}
	actor.read = nativeTUNActorRead{caller: ctx, ctx: ctx, pending: true, stage: nativeTUNActorReadContext}
	read := &actor.read
	actor.mu.Unlock()
	returned := false
	defer actor.finishDescriptor(read, &returned, &descriptor, &finalErr)
	descriptor, finalErr = actor.queryDescriptor()
	returned = true
	return descriptor, finalErr
}

// The final source callback is still part of the original read. It must return
// outside actor.mu before value/passive-channel facts can commit under the lock.
// An abnormal body, cancel or final fence keeps the exact pending read reachable.
func (actor *NativeTUNActor) finishDescriptor(read *nativeTUNActorRead, returned *bool, descriptor *NativeTUNDescriptor, result *error) {
	payload := recover()
	checked := false
	var sourceErr error
	defer func() {
		fencePayload := recover()
		actor.mu.Lock()
		defer actor.mu.Unlock()
		if payload != nil || !*returned || !checked || fencePayload != nil || !read.pending ||
			read.cancel != nil && (!read.cancelStarted || !read.cancelReturned) {
			actor.revoked.Store(true)
			actor.active, read.fault = false, true
			*descriptor = NativeTUNDescriptor{}
			*result = errors.Join(ErrNativeScopeUnavailable, ErrHelperCleanup, *result, sourceErr)
			return // original contexts/cancel/stage remain pending and cannot be replaced
		}
		now := time.Now()
		if !actor.active || actor.Revoked() || !now.Before(actor.until) || read.hasDeadline && !now.Before(read.deadline) ||
			actor.process == nil || actor.process.exited() || sourceErr != nil {
			*descriptor = NativeTUNDescriptor{}
			*result = errors.Join(ErrNativeScopeUnavailable, *result, sourceErr)
		}
		actor.read = nativeTUNActorRead{} // only after query AND its original cancel returned
	}()
	if payload != nil || !*returned {
		return
	}
	read.stage = nativeTUNActorReadFinalFence
	sourceErr = actor.descriptorScope(read.caller)
	checked = true
}

// Source fields are immutable; callbacks here never run with actor.mu held.
// Revocation can seal concurrently even if a caller-supplied context is blocked.
func (actor *NativeTUNActor) descriptorScope(ctx context.Context) error {
	if actor.Revoked() {
		return ErrNativeScopeUnavailable
	}
	if err := errors.Join(contextError(ctx), contextError(actor.operation)); err != nil {
		return errors.Join(ErrNativeScopeUnavailable, err)
	}
	if actor.Revoked() || !time.Now().Before(actor.until) {
		return ErrNativeScopeUnavailable
	}
	return nil
}

// Source fields are immutable. The original actor retains this read while all
// IPC/image/ancestor/path callbacks run outside its metadata mutex. Deactivate
// can seal immediately; helper/image release additionally checks this read.
func (actor *NativeTUNActor) queryDescriptor() (NativeTUNDescriptor, error) {
	read := &actor.read
	ctx := read.caller
	if err := actor.descriptorScope(ctx); err != nil {
		return NativeTUNDescriptor{}, err
	}
	if !time.Now().Before(actor.until) || actor.engine == nil || actor.scope == nil || actor.image == nil || actor.process == nil || actor.process.exited() || actor.port == 0 {
		return NativeTUNDescriptor{}, ErrNativeScopeUnavailable
	}
	ctx = withOriginalOperationSources(ctx, actor.operation)
	ctx, cancel := context.WithDeadline(ctx, actor.until) // caller may shorten, never extend the original capture
	read.ctx, read.cancel = ctx, cancel
	defer func() {
		read.cancelStarted = true
		cancel()
		read.cancelReturned = true
	}()
	read.deadline, read.hasDeadline = ctx.Deadline()
	if err := actor.descriptorScope(ctx); err != nil {
		return NativeTUNDescriptor{}, err
	}
	read.stage = nativeTUNActorReadRuntime
	identity, err := actor.engine.CurrentRuntime(ctx)
	if err != nil || identity != actor.runtime || identity.ForwardingAdmitted {
		return NativeTUNDescriptor{}, errors.Join(ErrNativeScopeUnavailable, err)
	}
	if err := actor.descriptorScope(ctx); err != nil {
		return NativeTUNDescriptor{}, err
	}
	read.stage = nativeTUNActorReadImage
	if err := actor.image.ValidateProcess(ctx, actor.process.host); err != nil {
		return NativeTUNDescriptor{}, err
	}
	if err := actor.descriptorScope(ctx); err != nil {
		return NativeTUNDescriptor{}, err
	}
	read.stage = nativeTUNActorReadAncestor
	if err := actor.process.ancestorScope(ctx, true); err != nil {
		return NativeTUNDescriptor{}, errors.Join(ErrNativeScopeUnavailable, err)
	}
	if err := actor.descriptorScope(ctx); err != nil {
		return NativeTUNDescriptor{}, err
	}
	read.stage = nativeTUNActorReadExecutable
	executable := actor.image.Executable()
	if err := actor.descriptorScope(ctx); err != nil {
		return NativeTUNDescriptor{}, err
	}
	if actor.process.exited() || !time.Now().Before(actor.until) {
		return NativeTUNDescriptor{}, ErrNativeScopeUnavailable
	}
	read.stage = nativeTUNActorReadComplete
	return NativeTUNDescriptor{Executable: executable, Runtime: identity, SourcePort: actor.port}, nil
}

func (actor *NativeTUNActor) deactivate() {
	if actor != nil {
		actor.revoked.Store(true) // before mutex acquisition and both negative revocations
		actor.mu.Lock()
		actor.active = false
		actor.mu.Unlock()
	}
}

// Negative cleanup may consume the same original read's normal completion on
// a later retry. Unknown completion stays sticky and cannot release the helper
// socket/image even after ingress and filter revocations succeeded.
func (actor *NativeTUNActor) descriptorRetired() error {
	if actor == nil {
		return nil // no actor/read was published by this partial capture
	}
	actor.mu.Lock()
	defer actor.mu.Unlock()
	if !actor.Revoked() || actor.active || actor.read.pending || actor.read.fault {
		return ErrNativeScopeUnavailable
	}
	return nil
}
