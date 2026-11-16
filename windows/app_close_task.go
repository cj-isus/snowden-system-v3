package main

import (
	"errors"
	"fmt"

	"github.com/snowden-system/windows/backend/core"
)

// Single original close worker. Read result only after done; done is frame
// retirement, not a successful native cleanup or terminal Quit receipt.
type appCloseTask struct {
	app      *App
	done     chan struct{}
	sequence appCleanupSequence
	result   error
}

func (task *appCloseTask) run() {
	returned := false
	defer task.finish(&returned)
	task.result = task.cleanup()
	returned = true
}

func (task *appCloseTask) cleanup() (result error) {
	a, sequence := task.app, &task.sequence
	defer sequence.finish(&result)
	defer appCleanupStep("log close cleanup outcome", sequence, func() error {
		if sequence.result != nil {
			a.appendLog("warn", "закрытие: очистка не подтверждена")
		}
		return nil
	})
	defer appCleanupStep("stop original lifecycle for Close", sequence, func() error {
		return a.stopCoreFor(stopQuit)
	})
	defer appCleanupStep("cancel original network producers", sequence, func() error {
		_, _, err := a.cancelNetWatch() // join later, after Manager retirement
		return err
	})
	defer appCleanupStep("cancel original background probes", sequence, func() error {
		a.cancelBackgroundProbes()
		return nil
	})
	defer appCleanupStep("cancel original engine tailer", sequence, func() error {
		a.cancelEngineLogTailer()
		return nil
	})
	return nil
}

func (task *appCloseTask) finish(returned *bool) {
	payload := recover()
	defer close(task.done)
	if !*returned || payload != nil {
		task.result = errors.Join(task.result, task.sequence.result, core.ErrEngineCleanupFailed,
			fmt.Errorf("original Close did not return normally (%T)", payload))
	}
	task.app.recordTerminalRetirement(task.result)
	// Reuse the existing once-reserved Quit worker/backstop even after Goexit.
	// A blocked pre-Quit native call remains outside that existing backstop.
	task.app.requestQuit(0)
}
