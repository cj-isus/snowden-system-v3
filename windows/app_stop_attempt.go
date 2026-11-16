package main

import "github.com/snowden-system/windows/backend/core"

// Exact original negative cohort, not a successful native retirement receipt.
// All accesses hold operationMu; runtime identity reads also hold runtime_.mu.
type appStopAttempt struct {
	sequence  *appCleanupSequence
	manager   *core.Manager
	engine    *core.Engine
	restart   *core.RestartGUIOwner
	elevation *core.ElevatedGUIRequest
}

func (attempt *appStopAttempt) coversCurrentOwners(a *App) bool {
	runtime_.mu.Lock()
	same := (runtime_.manager == nil || runtime_.manager == attempt.manager) &&
		(runtime_.engine == nil || runtime_.engine == attempt.engine)
	runtime_.mu.Unlock()
	// Successful removal of one slot cannot justify replaying other original
	// callbacks. Only a newly substituted owner requires a different cleanup.
	return same && (a.restartGUI == nil || a.restartGUI == attempt.restart) &&
		(a.elevatedGUI == nil || a.elevatedGUI == attempt.elevation)
}

func (a *App) retainStopAttempt(registration intentRegistration, sequence *appCleanupSequence, manager *core.Manager, engine *core.Engine) {
	outcome := a.lifecycleOutcome
	if outcome == nil || outcome.registration.intent != registration.intent || outcome.registration.source != registration.source {
		return
	}
	outcome.cleanup = &appStopAttempt{sequence: sequence, manager: manager, engine: engine,
		restart: a.restartGUI, elevation: a.elevatedGUI}
}
