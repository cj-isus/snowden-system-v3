package main

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/core"
)

type protectionArmContextPolicy interface {
	ArmContext(context.Context) error
}

type protectionArmContextKey struct{}

// Only the serialized App lifecycle issues this base-barrier scope. Network
// cancellation does not cancel protection intent. The original intent source
// and App lifetime still revoke it; this scope grants no TUN/diagnostic access.
type protectionArmScope struct {
	registration intentRegistration
	operation    *core.OperationSource
	application  context.Context
	closing      *atomic.Bool
	policy       protectionPolicy
}

const protectionArmTimeout = 5 * time.Second // operation policy, not native preemption or a measured SLO

func protectionArmSourceError(ctx context.Context) error {
	if ctx == nil {
		return core.ErrNativeScopeUnavailable
	}
	value := ctx.Value(protectionArmContextKey{})
	if value == nil {
		return nil // legacy generic callers keep their existing contract
	}
	scope, ok := value.(*protectionArmScope)
	if !ok || scope == nil || scope.registration.intent == 0 || scope.registration.source == nil || !scope.registration.source.Current() ||
		scope.operation == nil || !scope.operation.Current() ||
		scope.application == nil || scope.application.Err() != nil || scope.closing == nil || scope.closing.Load() {
		return context.Canceled
	}
	return nil
}

// Called with operationMu held, including cancellation's fail-closed path.
// Extract the original registration, never a later lookup by numeric intent.
func (a *App) armProtection(original context.Context) error {
	ctx, cancel, scope, err := a.prepareProtectionArm(original)
	if err != nil {
		return err
	}
	defer cancel()
	if err := childbootstrap.CheckScope(ctx, protectionArmSourceError); err != nil {
		return err
	}
	if policy, ok := scope.policy.(protectionArmContextPolicy); ok {
		err = policy.ArmContext(ctx)
	} else {
		err = scope.policy.Arm() // injected legacy seam; the same original scope is checked after its return
	}
	return errors.Join(err, childbootstrap.CheckScope(ctx, protectionArmSourceError))
}

func (a *App) prepareProtectionArm(original context.Context) (context.Context, context.CancelFunc, *protectionArmScope, error) {
	if a == nil || original == nil {
		return nil, nil, nil, core.ErrNativeScopeUnavailable
	}
	operation, ok := original.Value(appOperationContextKey{}).(*appOperationScope)
	if !ok || operation == nil || operation.app != a || operation.intent == 0 || operation.operation == nil || !operation.operation.Current() {
		return nil, nil, nil, context.Canceled
	}
	registration := intentRegistration{intent: operation.intent, source: operation.intentSource}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.appLifetimeCurrentLocked() || !registration.currentLocked(a) || !a.protectionDesired || a.profileTransition ||
		a.closing.Load() || a.releaseCommit != nil || a.restartCommit != nil {
		return nil, nil, nil, context.Canceled
	}
	scope := &protectionArmScope{registration: registration, operation: operation.operation, application: a.ctx, closing: &a.closing, policy: a.protectionPolicy()}
	ctx, cancel := context.WithTimeout(a.ctx, protectionArmTimeout)
	ctx = context.WithValue(ctx, protectionArmContextKey{}, scope)
	return ctx, cancel, scope, nil
}
