package core

import (
	"context"
	"errors"
	"sync"
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
)

// One original read, embedded in the actor before any external call. Only its
// admitted caller writes the payload; pending/fault and reuse are under mu.
// An interrupted read retains its original contexts/cancel/stage, never a
// detached callback, replacement waiter or fabricated retirement receipt.
type nativeTUNActorRead struct {
	caller         context.Context
	ctx            context.Context
	cancel         context.CancelFunc
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
	read      nativeTUNActorRead
}

func (actor *NativeTUNActor) Descriptor(ctx context.Context) (descriptor NativeTUNDescriptor, finalErr error) {
	if actor == nil {
		return descriptor, ErrNativeScopeUnavailable
	}
	actor.mu.Lock()
	if !actor.active || actor.read.fault {
		actor.mu.Unlock()
		return descriptor, ErrNativeScopeUnavailable
	}
	if actor.read.pending {
		actor.mu.Unlock()
		return descriptor, errors.Join(ErrNativeScopeUnavailable, ErrHelperBusy)
	}
	actor.read = nativeTUNActorRead{caller: ctx, ctx: ctx, pending: true, stage: nativeTUNActorReadContext}
	actor.mu.Unlock()
	returned := false
	defer func() {
		actor.mu.Lock()
		defer actor.mu.Unlock()
		if !returned {
			actor.active, actor.read.fault = false, true
			descriptor = NativeTUNDescriptor{}
			finalErr = errors.Join(ErrNativeScopeUnavailable, finalErr)
			return // keep the original unknown read; preserve panic/Goexit unwind
		}
		currentErr := errors.Join(contextError(ctx), contextError(actor.operation))
		if !actor.active || !time.Now().Before(actor.until) || actor.process == nil || actor.process.exited() || currentErr != nil {
			descriptor = NativeTUNDescriptor{}
			finalErr = errors.Join(ErrNativeScopeUnavailable, finalErr, currentErr)
		}
		actor.read = nativeTUNActorRead{} // only after query AND its original cancel returned
	}()
	descriptor, finalErr = actor.queryDescriptor()
	returned = true
	return descriptor, finalErr
}

// Source fields are immutable. The original actor retains this read while all
// IPC/image/ancestor/path callbacks run outside its metadata mutex. Deactivate
// can seal immediately; helper/image release additionally checks this read.
func (actor *NativeTUNActor) queryDescriptor() (NativeTUNDescriptor, error) {
	read := &actor.read
	ctx := read.caller
	if err := errors.Join(contextError(ctx), contextError(actor.operation)); err != nil {
		return NativeTUNDescriptor{}, errors.Join(ErrNativeScopeUnavailable, err)
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
	read.stage = nativeTUNActorReadRuntime
	identity, err := actor.engine.CurrentRuntime(ctx)
	if err != nil || identity != actor.runtime || identity.ForwardingAdmitted {
		return NativeTUNDescriptor{}, errors.Join(ErrNativeScopeUnavailable, err)
	}
	read.stage = nativeTUNActorReadImage
	if err := actor.image.ValidateProcess(ctx, actor.process.host); err != nil {
		return NativeTUNDescriptor{}, err
	}
	read.stage = nativeTUNActorReadAncestor
	if err := actor.process.ancestorScope(ctx, true); err != nil {
		return NativeTUNDescriptor{}, errors.Join(ErrNativeScopeUnavailable, err)
	}
	read.stage = nativeTUNActorReadExecutable
	executable := actor.image.Executable()
	if err := errors.Join(contextError(ctx), contextError(actor.operation)); err != nil {
		return NativeTUNDescriptor{}, errors.Join(ErrNativeScopeUnavailable, err)
	}
	if actor.process.exited() || !time.Now().Before(actor.until) {
		return NativeTUNDescriptor{}, ErrNativeScopeUnavailable
	}
	read.stage = nativeTUNActorReadComplete
	return NativeTUNDescriptor{Executable: executable, Runtime: identity, SourcePort: actor.port}, nil
}

func (actor *NativeTUNActor) deactivate() {
	if actor != nil {
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
	if actor.active || actor.read.pending || actor.read.fault {
		return ErrNativeScopeUnavailable
	}
	return nil
}
