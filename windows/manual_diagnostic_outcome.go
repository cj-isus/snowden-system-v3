package main

import "time"

// Original synchronous RPC plus its existing cancellation watcher. No new
// goroutine is created for finalization, logging or a failed native operation.
type manualDiagnosticOutcome struct {
	app           *App
	task          *manualDiagnosticTask
	credentials   bool
	returned      bool
	beforeRelease func() error // optional original stage after known network retirement; outside App.mu
}

func (outcome *manualDiagnosticOutcome) finishSecret(report *SecretTestReport, result *error) {
	payload := recover()
	outcome.finish(payload, result, func() { *report = SecretTestReport{} }, "тест секрета")
}

func (outcome *manualDiagnosticOutcome) finishTest(report *TestResultView, result *error) {
	payload := recover()
	outcome.finish(payload, result, func() { *report = TestResultView{} }, "диагностика")
}

func (outcome *manualDiagnosticOutcome) finish(payload any, result *error, reject func(), label string) {
	task := outcome.task
	task.bodyReturned = outcome.returned && payload == nil
	if !task.bodyReturned {
		task.cleanupFailed.Store(true) // unreturned SDK/body defers cannot grant a completed observation
		*result = errSecretTestUnavailable
		reject()
	}
	sequence := &appCleanupSequence{}
	defer outcome.complete(sequence, result, reject)
	defer appCleanupStep("log original manual diagnostic outcome", sequence, func() error {
		returned := false
		defer func() {
			if !returned {
				task.cleanupFailed.Store(true)
			}
		}()
		if sequence.result != nil || *result != nil || !task.retirementConfirmed || !task.publicationCurrent ||
			outcome.credentials && task.credentialsChanged.Load() {
			outcome.app.appendLog("warn", label+": результат не получен")
		} else {
			outcome.app.appendLog("info", label+": наблюдение получено")
		}
		returned = true
		return nil
	})
	defer appCleanupStep("retire original manual diagnostic", sequence, func() error {
		return outcome.app.finishManualDiagnostic(task)
	})
}

func (outcome *manualDiagnosticOutcome) complete(sequence *appCleanupSequence, result *error, reject func()) {
	a, task := outcome.app, outcome.task
	finished := false
	defer func() {
		if payload := recover(); payload != nil || !finished {
			task.cleanupFailed.Store(true)
			*result = errSecretTestUnavailable
			reject()
			a.mu.Lock()
			task.resultRejected = true
			a.mu.Unlock()
		}
	}()
	if sequence.result != nil || *result != nil || !task.bodyReturned || outcome.credentials && task.credentialsChanged.Load() {
		*result = errSecretTestUnavailable
		reject()
	}
	if !task.retirementConfirmed || task.cleanupFailed.Load() {
		*result = errSecretTestUnavailable
		reject()
		finished = true
		return // original pointer, admission and done stay pending; no cleanup retry
	}
	a.mu.Lock()
	ours := a.diagnosticTask == task && !task.completed
	a.mu.Unlock()
	if !ours {
		*result = errSecretTestUnavailable
		reject()
		finished = true
		return
	}
	if outcome.beforeRelease != nil && *result == nil && task.publicationCurrent {
		if err := outcome.beforeRelease(); err != nil {
			*result = errSecretTestUnavailable
			reject()
		}
	}
	if task.cleanupFailed.Load() {
		*result = errSecretTestUnavailable
		reject()
		finished = true
		return // original optional stage may retain its own uncertain native resource
	}
	if task.release != nil {
		if task.releaseStarted {
			task.cleanupFailed.Store(true)
			*result = errSecretTestUnavailable
			reject()
			finished = true
			return
		}
		task.releaseStarted = true // before original callback; an interrupted release is never replayed
		task.release()
	}
	task.releaseReturned = true
	a.mu.Lock()
	defer a.mu.Unlock()
	deadline, bounded := task.ctx.Deadline()
	current := task.publicationCurrent && task.parent != nil && task.parent.Err() == nil && task.registration.currentLocked(a) && !a.closing.Load() &&
		(task.path == nil || task.path.current.Load()) && (!bounded || time.Now().Before(deadline)) &&
		sameDiagnosticContext(task.contextView, a.diagnosticContextLocked()) && (!outcome.credentials || !task.credentialsChanged.Load())
	task.resultRejected = !current || sequence.result != nil || *result != nil
	if task.resultRejected {
		*result = errSecretTestUnavailable
		reject()
	}
	if a.diagnosticTask != task {
		task.resultRejected = true
		*result = errSecretTestUnavailable
		reject()
		finished = true
		return
	}
	a.diagnosticTask = nil
	task.completed = true
	close(task.done) // body/SDK defers, watcher, dial/Close frames, logging and original release returned
	finished = true
}
