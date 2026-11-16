package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

// A lease binds an observed interface to an acknowledged worker/config during
// a lifecycle probe. It is private, in-memory evidence, not kernel ownership
// attestation or permission to reuse a result after lifecycle/epoch changes.
type tunnelLease struct {
	engine  *core.Engine
	runtime core.RuntimeIdentity
	luid    uint64
	native  *nativeTUNLifecycleScope // prepared original observation, never a forwarding permit
}

type tunnelBoundary interface {
	BeforeChange(context.Context) error
	Establish(context.Context, *render.ClientConfig) (tunnelLease, error)
	Validate(context.Context, *render.ClientConfig, tunnelLease) error
	Observe(context.Context, *render.ClientConfig) error
}

type nativeTunnelBoundary struct {
	policy protectionPolicy
	app    *App
}

func (b nativeTunnelBoundary) BeforeChange(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil || b.policy == nil {
		return errors.New("TUN protection change canceled or unavailable")
	}
	if err := b.policy.BlockDirect(); err != nil {
		return err
	}
	return verifyWFPClosedContext(ctx) // exact base, no general/diagnostic permits
}

func currentTunnelRuntime(ctx context.Context, cc *render.ClientConfig) (*core.Engine, core.RuntimeIdentity, error) {
	var identity core.RuntimeIdentity
	if cc == nil || cc.TUNName != render.TUNInterfaceName {
		return nil, identity, errors.New("TUN configuration identity unavailable")
	}
	runtime_.mu.Lock()
	engine, manager := runtime_.engine, runtime_.manager
	runtime_.mu.Unlock()
	if engine == nil || manager == nil {
		return nil, identity, core.ErrEngineWorkerUnavailable
	}
	identity, err := engine.CurrentRuntime(ctx)
	if err != nil {
		return nil, identity, err
	}
	expected, err := core.RuntimeConfigurationDigest(cc.Raw)
	if err != nil || identity.ConfigDigest != expected || identity.TUNName != cc.TUNName || identity.DefaultOutbound != cc.SelectorDefault {
		return nil, core.RuntimeIdentity{}, errors.New("TUN worker/config/selector mismatch")
	}
	return engine, identity, nil
}

func (b nativeTunnelBoundary) Establish(ctx context.Context, cc *render.ClientConfig) (tunnelLease, error) {
	if _, strict := b.policy.(strictWFPPolicy); !core.NativeTUNForwardingAvailable() || !strict || b.app == nil {
		return tunnelLease{}, core.ErrNativeScopeUnavailable
	}
	scope, err := b.app.captureTUNLifecycleScope(ctx, cc)
	if err != nil {
		return tunnelLease{}, err
	}
	if err := verifyWFPClosedContext(ctx); err != nil {
		return tunnelLease{}, err
	}
	observation, err := scope.engine.ObserveNativeTUN(ctx, scope.pin, 8*time.Second, scope.guard, nativeTUNWFPPolicy{})
	if err != nil {
		return tunnelLease{}, err // original helper/filter/ingress custody remains in core
	}
	// A released frame or a caller's HTTP/IP report is not accepted here. The
	// opaque observation exists only after the original Job/I/O/image retirement.
	if observation.Runtime() != scope.runtime || !scope.matchesEgress(observation.Egress()) {
		return tunnelLease{}, core.ErrNativeScopeUnavailable
	}
	scope.observation = observation
	lease := tunnelLease{engine: scope.engine, runtime: scope.runtime, luid: scope.luid, native: scope}
	if err := b.Validate(ctx, cc, lease); err != nil {
		return tunnelLease{}, err
	}
	// This is a prepared narrow observation only. The private App/Manager commit
	// consumes its original evidence before granting general LUID permission and
	// admitted ingress. The compiled native acceptance gate remains closed.
	return lease, nil
}

func (b nativeTunnelBoundary) Validate(ctx context.Context, cc *render.ClientConfig, lease tunnelLease) error {
	if lease.native != nil {
		if lease.native.app != b.app || lease.native.engine != lease.engine || lease.native.runtime != lease.runtime || lease.native.luid != lease.luid {
			return core.ErrNativeScopeUnavailable
		}
		return lease.native.validatePrepared(ctx, cc)
	}
	engine, identity, err := currentTunnelRuntime(ctx, cc)
	if err != nil {
		return err
	}
	if engine != lease.engine || identity != lease.runtime || !identity.ForwardingAdmitted || lease.luid == 0 {
		return errors.New("TUN lifecycle worker changed")
	}
	return verifyWFPPolicyContext(ctx, true, lease.luid)
}

// Periodic verification adds no external HTTP flow. It checks current worker,
// owned config/selector and observed LUID policy; it does not refresh the last
// external lifecycle observation or assert unmeasured native protocol health.
func (b nativeTunnelBoundary) Observe(ctx context.Context, cc *render.ClientConfig) error {
	engine, identity, err := currentTunnelRuntime(ctx, cc)
	if err != nil {
		return err
	}
	luid, err := waitInterfaceLUIDContext(ctx, cc.TUNName, 250*time.Millisecond)
	if err != nil {
		return err
	}
	return b.Validate(ctx, cc, tunnelLease{engine: engine, runtime: identity, luid: luid})
}
