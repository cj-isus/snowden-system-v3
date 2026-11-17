package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/snowden-system/windows/backend/core"
)

// Original sequence outcome only. Drained is not native resource retirement.
type appCleanupSequence struct {
	drained bool
	result  error
}

func (sequence *appCleanupSequence) finish(result *error) {
	if sequence.result != nil {
		sequence.result = errors.Join(core.ErrEngineCleanupFailed, sequence.result)
	}
	sequence.drained, *result = true, sequence.result
}

// Registered directly as a defer before any cleanup callback. A Goexit still
// unwinds subsequent stages; a blocked original callback remains pending.
func appCleanupStep(name string, sequence *appCleanupSequence, call func() error) {
	returned := false
	defer func() {
		if !returned {
			payload := recover()
			sequence.result = errors.Join(sequence.result, fmt.Errorf("%s cleanup did not return normally (%T)", name, payload))
		}
	}()
	sequence.result = errors.Join(sequence.result, call())
	returned = true
}

// operationMu held. Revoke general forwarding, retire the original actors, then
// freshly confirm the retained barrier (or final revocation for Disconnect).
// All negative steps are registered before callbacks, including abnormal unwind.
// Release remains a separate positive phase, never an unwind action.
func (a *App) stopResourceCleanup(sequence *appCleanupSequence, reason StopReason, manager *core.Manager, orphanedEngine bool, policyRequired *bool) (result error) {
	policy := a.protectionPolicy()
	retirement, staged := policy.(protectionRetirementPolicy)
	defer sequence.finish(&result)
	defer appCleanupStep("confirm final direct protection", sequence, func() error {
		if !*policyRequired {
			return nil
		}
		if reason.releasesProtection() {
			if staged {
				return retirement.RevokeDirect() // damaged base may still need explicit Release
			}
			return nil // preserve the existing injected policy's manual recovery contract
		}
		return policy.BlockDirect()
	})
	defer appCleanupStep("retire original App helpers", sequence, a.retireCaptureHelpers)
	if manager != nil {
		defer appCleanupStep("stop original Manager", sequence, func() error {
			return manager.Stop(context.Background())
		})
	}
	defer appCleanupStep("retire original metrics request", sequence, a.retireMetrics)
	defer appCleanupStep("retire original facts HTTP request", sequence, a.retireFactsEgress)
	if orphanedEngine {
		sequence.result = errors.Join(sequence.result, errRuntimeOwnerUnavailable)
	}
	defer appCleanupStep("retain direct protection", sequence, func() error {
		*policyRequired = tunRequested() || strictProtectionRetained()
		if *policyRequired {
			if staged {
				return retirement.RevokeDirect()
			}
			if !reason.releasesProtection() {
				return policy.BlockDirect()
			}
		}
		return nil
	})
	defer appCleanupStep("publish stopping state", sequence, func() error {
		a.pushState()
		return nil
	})
	return nil
}
