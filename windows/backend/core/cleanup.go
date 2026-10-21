package core

import (
	"errors"
	"fmt"
)

// Lifecycle boundaries may panic even while handling another failure. Convert
// each cleanup panic separately so remaining cleanup and finalization still run.
func cleanupCall(name string, fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s panic: %v", name, recovered)
		}
	}()
	return fn()
}

func (m *Manager) cleanup(engine *Engine, proxy systemProxy) error {
	var engineErr error
	if engine != nil {
		engineErr = cleanupCall("engine stop", engine.Stop)
	}
	return errors.Join(engineErr, m.restoreProxy(proxy))
}
