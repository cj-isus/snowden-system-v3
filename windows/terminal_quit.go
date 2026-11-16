package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) recordTerminalRetirement(result error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.terminalRetirementErr = errors.Join(a.terminalRetirementErr, result)
}

func (a *App) runQuitTask(delay time.Duration) {
	returned := false
	sequence := &appCleanupSequence{}
	defer a.finishQuitTask(sequence, &returned)
	if delay > 0 {
		time.Sleep(delay)
	}
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	sequence.result = a.retireTerminalProducers(ctx)
	returned = true
}

// Direct defer of the existing once-reserved worker. Quit is terminal intent,
// never cleanup success. The independently armed existing backstop is retained.
func (a *App) finishQuitTask(sequence *appCleanupSequence, returned *bool) {
	payload := recover()
	if !*returned || payload != nil {
		sequence.result = errors.Join(sequence.result, errAppProducerPending,
			fmt.Errorf("terminal worker did not return normally (%T)", payload))
	}
	defer func() { a.recordTerminalRetirement(sequence.result) }()
	a.mu.Lock()
	a.quitRequested = true
	appCtx := a.ctx
	a.mu.Unlock()
	defer appCleanupStep("request Wails Quit", sequence, func() error {
		if appCtx != nil {
			runtime.Quit(appCtx)
		}
		return nil
	})
	defer appCleanupStep("log terminal cleanup outcome", sequence, func() error {
		if sequence.result != nil {
			a.appendLog("warn", "выход: завершение фоновых задач или журнала не подтверждено")
		}
		return nil
	})
}
