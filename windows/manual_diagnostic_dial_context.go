package main

import (
	"net"

	"github.com/snowden-system/windows/backend/core"
)

// The original registered dial owns parent preparation before any actual
// getter/factory callback. No Context method executes under task.mu.
func (dial *manualDiagnosticDial) prepareContext() error {
	entered := false
	defer func() {
		if !entered {
			dial.contextState.skipConstruction() // known no constructor attempt, not an actual Dial return
		}
	}()
	parentFence, err := core.CaptureOperationFence(dial.parent)
	if err != nil || dial.task.currentError() != nil {
		return errSecretTestUnavailable
	}
	entered = true
	ctx, childFence, err := dial.contextState.construct(appTaskParent{app: dial.task.app, ctx: dial.parent, fence: parentFence})
	if err != nil {
		return errSecretTestUnavailable
	}
	dial.task.mu.Lock()
	defer dial.task.mu.Unlock()
	if dial.task.sealed || dial.task.currentError() != nil || !parentFence.Current() || !childFence.Current() || !dial.contextState.cancellation.operation.Current() {
		return errSecretTestUnavailable
	}
	dial.ctx = ctx
	dial.contextState.parentFence, dial.contextState.childFence, dial.contextState.ready = parentFence, childFence, true
	return nil
}

// Actual original setup/callback/cancel completion, distinct from a request or
// notification. The original task retains every partial/late socket separately.
func (dial *manualDiagnosticDial) finish(returned *bool, conn *net.Conn, result *error) {
	payload := recover()
	normal := *returned && payload == nil
	defer close(dial.done)
	sequence := new(appCleanupSequence)
	var cleanupResult error
	defer func() {
		dial.retirement = appProducerReceipt{bodyReturned: normal && dial.called && dial.returned, preparationOnly: !dial.called,
			preparationReturned: normal, cleanupReturned: sequence.drained && cleanupResult == nil, cleanupResult: cleanupResult}
		if !normal || dial.retirement.terminalResult(errSecretTestUnavailable) != nil {
			dial.task.cleanupFailed.Store(true)
			*conn, *result = nil, errSecretTestUnavailable
		}
	}()
	defer sequence.finish(&cleanupResult)
	defer appCleanupStep("retire original diagnostic dial context", sequence, dial.contextState.cancellation.join)
}
