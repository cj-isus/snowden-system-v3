package core

import (
	"context"
	"errors"
)

// LifecycleProbe extends the route probe with platform protection boundaries.
// Manager calls it during Start/select/Reload/rollback. Legacy implementations
// may also supply cleanup through BeforeEngineChange; platform probes with a
// positive source gate must implement LifecycleCleanupProbe separately.
// Implementations never hold Manager locks across platform/Engine work or
// release strict protection. Short value-only lifecycle identity guards are OK.
type LifecycleProbe interface {
	Probe
	BeforeEngineChange(context.Context) error
	RunLifecycle(context.Context) *ProbeReport
}

// Restriction before actual Stop is independent of canceled positive work.
// It may revoke only the retained platform boundary; never admit forwarding,
// release strict protection, or require a current App intent/new Manager.
type LifecycleCleanupProbe interface {
	Probe
	BeforeEngineStop(context.Context) error
}

// Optional platform source checks run outside Manager/Engine locks and add no
// authority to ordinary reports. Generic core probes retain context checks.
type LifecycleSourceProbe interface {
	Probe
	ValidateLifecycle(context.Context) error
}

// Called only while the original Manager holds mu and still owns the exact
// active operation/engine. The implementation may acquire downstream value
// locks (App -> epoch), never runtime/Manager locks, callbacks or external I/O.
// A successful lease remains held through the terminal state decision. Failed
// acquisition must release every partial lock; release must be synchronous.
type LifecycleCompletionProbe interface {
	LifecycleSourceProbe
	GuardLifecycleCompletion(context.Context, *Manager) (func(), error)
}

func validateLifecycleSource(ctx context.Context, probe Probe) error {
	return cleanupCall("validate lifecycle source", func() error {
		if err := contextError(ctx); err != nil {
			return err
		}
		if probe == nil {
			return errors.New("lifecycle source probe unavailable")
		}
		if source, ok := probe.(LifecycleSourceProbe); ok {
			if err := source.ValidateLifecycle(ctx); err != nil {
				return err
			}
		}
		return contextError(ctx)
	})
}

// Caller holds Manager.mu, so this must not call ValidateLifecycle (which may
// acquire runtime/Manager locks). The separate downstream-only API avoids a
// lock inversion at the final state decision.
func guardLifecycleCompletion(ctx context.Context, probe Probe, manager *Manager) (release func(), err error) {
	err = errors.New("lifecycle completion guard did not return") // negative default during unwind
	defer func() {
		if err != nil && release != nil {
			err = errors.Join(err, releaseLifecycleCompletion(release))
			release = nil
		}
	}()
	err = cleanupCall("guard lifecycle completion", func() error {
		if err := contextError(ctx); err != nil {
			return err
		}
		if source, ok := probe.(LifecycleCompletionProbe); ok {
			var guardErr error
			release, guardErr = source.GuardLifecycleCompletion(ctx, manager)
			if guardErr == nil && release == nil {
				guardErr = errors.New("lifecycle completion lease unavailable")
			}
			return guardErr
		}
		if _, scoped := probe.(LifecycleSourceProbe); scoped {
			return errors.New("platform lifecycle completion guard unavailable")
		}
		release = func() {} // existing explicit legacy/noop contract, not a platform source lease
		return nil
	})
	return release, err
}

func releaseLifecycleCompletion(release func()) error {
	err := cleanupCall("lifecycle completion lease release", func() error {
		release()
		return nil
	})
	if err != nil {
		return errors.Join(ErrEngineCleanupFailed, err)
	}
	return nil
}

func prepareEngineChange(ctx context.Context, probe Probe) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if guarded, ok := probe.(LifecycleProbe); ok {
		return guarded.BeforeEngineChange(ctx)
	}
	return nil
}

func prepareEngineStop(ctx context.Context, probe Probe) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if cleanup, ok := probe.(LifecycleCleanupProbe); ok {
		return cleanup.BeforeEngineStop(ctx)
	}
	if _, scoped := probe.(LifecycleSourceProbe); scoped {
		return errors.Join(ErrEngineCleanupFailed, errors.New("platform lifecycle cleanup boundary unavailable"))
	}
	return prepareEngineChange(ctx, probe) // legacy non-platform implementation
}

func runLifecycleProbe(ctx context.Context, probe Probe) *ProbeReport {
	if probe == nil {
		return nil
	}
	if guarded, ok := probe.(LifecycleProbe); ok {
		return guarded.RunLifecycle(ctx)
	}
	return probe.Run(ctx)
}
