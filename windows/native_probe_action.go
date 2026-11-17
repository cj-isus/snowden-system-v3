package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/netprobe"
)

const nativeProbeLimit = 12 * time.Second

// RunNativeProbe is an explicit optional action, never a startup/periodic gate.
// No caller-supplied destination, packet, helper path or timeout is accepted.
// A completed batch may contain only unsuccessful DNS attempts and still is
// not a VPN failure. Mandatory probe, health, retry and failover are untouched.
func (a *App) RunNativeProbe() (resultErr error) {
	if a == nil {
		return errors.New("native DNS observation busy or unavailable")
	}
	a.mu.Lock()
	outcome := &nativeProbeActionOutcome{app: a, registration: a.intentRegistrationLocked()}
	a.mu.Unlock()
	if !a.operationMu.TryLock() {
		return errors.New("native DNS observation busy or unavailable")
	}
	defer a.operationMu.Unlock()
	defer outcome.finish(&resultErr)
	resultErr = a.runNativeProbeBody(outcome)
	outcome.returned = true
	return resultErr
}

func (a *App) runNativeProbeBody(outcome *nativeProbeActionOutcome) error {
	intent := outcome.registration.intent
	parent, end, preparationErr := a.beginOperationContext(intent, outcome.registration.source)
	if preparationErr != nil {
		if localRetirementFailure(preparationErr) {
			return preparationErr // actual finite local custody failure, not an optional scope result
		}
		return core.ErrNativeScopeUnavailable
	}
	defer a.endOperation(end)
	ctx, cancel := context.WithTimeout(parent, nativeProbeLimit)
	defer cancel()
	publication := capturePublicationContext(ctx)
	if publication == nil {
		return core.ErrNativeScopeUnavailable
	}
	scope, err := a.captureNativeProbeScope(ctx, intent)
	if err != nil {
		return nativeProbePublicError(err)
	}
	draft, err := func() (*nativeProbeObservation, error) {
		release, now, err := scope.lockCurrent(a, ctx)
		if err != nil {
			return nil, core.ErrNativeScopeUnavailable
		}
		defer release()
		draft := &nativeProbeObservation{scope: scope, view: emptyNativeProbeView("running", "dns_batch_running"), expires: nativeProbeExpiry(now, scope)}
		draft.view.StartedAt, draft.view.ExpiresAt = scope.started.UTC().Format(time.RFC3339Nano), draft.expires.UTC().Format(time.RFC3339Nano)
		if !publication.current() {
			return nil, core.ErrNativeScopeUnavailable
		}
		outcome.draft, a.nativeObservation = draft, draft
		return draft, nil
	}()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			a.mu.Lock()
			defer a.mu.Unlock()
			if a.nativeObservation == draft {
				a.nativeObservation = nil // a failed/partial batch never replaces newer facts
			}
		}
	}()
	reply, err := core.CaptureNativeDNS(ctx, scope.pin, nativeProbeNetworks(), nativeProbeLimit, func(ctx context.Context) error { return scope.guard(a, ctx) })
	if err != nil {
		return nativeProbePublicError(err) // never forward paths, IPC text or OS errors
	}
	// The core rechecks the full native guard after decode/cleanup. This final
	// short value gate prevents an intervening Stop/epoch/owner/clock change
	// from committing its result. No native calls are made while locks are held.
	release, completed, err := scope.lockCurrent(a, ctx)
	if err != nil {
		return core.ErrNativeScopeUnavailable
	}
	defer release()
	if a.nativeObservation != draft {
		return core.ErrNativeScopeUnavailable
	}
	expires := nativeProbeExpiry(completed, scope)
	view, err := nativeProbeView(reply, scope.started, completed, expires)
	if err != nil {
		return nativeProbePublicError(err)
	}
	if !publication.current() {
		return core.ErrNativeScopeUnavailable
	}
	draft.view, draft.expires = view, expires
	committed = true
	return nil
}

// Read-only, memory-only polling. Invalid subjects expose no previous attempt
// slots as current. This does not refresh timestamps or the external probe.
func (a *App) GetNativeProbeReport() (view NativeProbeReportView) {
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			view = emptyNativeProbeView("unavailable", "scope_unavailable")
		}
	}()
	view = a.readNativeProbeReport()
	returned = true
	return view
}

func (a *App) readNativeProbeReport() NativeProbeReportView {
	if a == nil {
		return emptyNativeProbeView("no_data", "no_observation")
	}
	a.mu.Lock()
	observation := a.nativeObservation
	a.mu.Unlock()
	if observation == nil {
		return emptyNativeProbeView("no_data", "no_observation")
	}
	release, now, err := observation.scope.lockCurrent(a, context.Background())
	if err != nil {
		return emptyNativeProbeView("stale", "scope_unavailable")
	}
	defer release()
	if a.nativeObservation != observation {
		return emptyNativeProbeView("no_data", "observation_replaced")
	}
	if !now.Before(observation.expires) {
		return emptyNativeProbeView("expired", "observation_expired")
	}
	return observation.view // arrays of values: caller cannot mutate cached facts
}

func nativeProbePublicError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return errors.New("native DNS observation canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("native DNS observation deadline exceeded")
	case errors.Is(err, core.ErrNativeScopeUnavailable):
		return core.ErrNativeScopeUnavailable
	case errors.Is(err, core.ErrNativeImageUnavailable):
		return core.ErrNativeImageUnavailable
	case errors.Is(err, core.ErrNativeProbeIncomplete), errors.Is(err, netprobe.ErrNativeProtocol):
		return errors.New("native DNS observation incomplete or invalid")
	default:
		return errors.New("native DNS observation unavailable")
	}
}
