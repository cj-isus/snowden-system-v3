package main

import "github.com/snowden-system/windows/backend/core"

// Optional action outcome only. It cannot mutate mandatory health, channel
// evidence or engine/protection intent; core retains its original helper owner.
type nativeProbeActionOutcome struct {
	app          *App
	registration intentRegistration
	draft        *nativeProbeObservation
	returned     bool
}

func (outcome *nativeProbeActionOutcome) finish(result *error) {
	payload := recover()
	if outcome.returned && payload == nil {
		return
	}
	*result = core.ErrNativeScopeUnavailable // never expose arbitrary panic/IPC/OS payload
	outcome.app.mu.Lock()
	defer outcome.app.mu.Unlock()
	if outcome.draft != nil && outcome.app.nativeObservation == outcome.draft {
		outcome.app.nativeObservation = nil // only this original action's provisional/committed value
	}
}
