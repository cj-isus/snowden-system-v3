package core

import (
	"context"
	"errors"
	"fmt"
)

// Cleanup and validation callbacks may fail while handling another failure.
// Only ordinary return can supply their result; a nil panic payload must not
// become nil success. This does not stop Goexit or force native I/O to return.
func cleanupCall(name string, fn func() error) (err error) {
	returned := false
	defer func() {
		if !returned {
			recovered := recover()
			err = fmt.Errorf("%s did not return normally (%T)", name, recovered)
		}
	}()
	err = fn()
	returned = true
	return err // preserve an ordinary error exactly; never include the panic value
}

// Sequence receipt only, not proof that a failed native resource was closed.
// The original lifecycle caller reads it after this frame's defers unwind.
type lifecycleCleanup struct {
	drained bool
	result  error
}

func lifecycleCleanupStep(name string, receipt *lifecycleCleanup, fn func() error) {
	returned := false
	defer func() {
		if !returned {
			payload := recover()
			receipt.result = errors.Join(receipt.result, fmt.Errorf("%s cleanup did not return normally (%T)", name, payload))
		}
	}()
	receipt.result = errors.Join(receipt.result, fn())
	returned = true
}

func (m *Manager) cleanup(engine *Engine, proxy systemProxy, probes ...Probe) (result error) {
	m.mu.Lock()
	if len(probes) == 0 {
		probes = []Probe{m.probe}
	}
	receipt := &lifecycleCleanup{}
	if operation := m.active; operation != nil {
		operation.cleanupResult = receipt // original sequence before any callback can acquire resources
	}
	m.mu.Unlock()
	defer func() {
		if receipt.result != nil {
			receipt.result = errors.Join(ErrEngineCleanupFailed, receipt.result)
		}
		receipt.drained, result = true, receipt.result
	}()
	// Register in reverse execution order. Goexit in a protection/engine
	// callback still runs every later original cleanup stage and its witness.
	defer lifecycleCleanupStep("restore system proxy", receipt, func() error { return m.restoreProxy(proxy) })
	if engine != nil {
		defer lifecycleCleanupStep("engine stop", receipt, engine.Stop)
	}
	for index := len(probes) - 1; index >= 0; index-- {
		probe := probes[index]
		defer lifecycleCleanupStep("protection before engine stop", receipt, func() error {
			return prepareEngineStop(context.Background(), probe)
		})
	}
	return nil
}
