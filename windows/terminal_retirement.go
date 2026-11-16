package main

import (
	"context"
	"fmt"
)

type terminalProducerSet struct {
	poller, dns, tailer, background, diagnostic, tray, watchdog <-chan struct{}
	autostart                                                   *autostartRegistryTask
	metrics                                                     *metricsAttempt
}

// All cancel stages precede every join. An abnormal cancel leaves a negative
// sequence result even if its original done fence could not be returned.
func (a *App) retireTerminalProducers(ctx context.Context) (result error) {
	sequence := &appCleanupSequence{}
	var producers terminalProducerSet
	defer func() { a.recordTerminalRetirement(sequence.result) }() // also on Goexit, before caller publication
	defer sequence.finish(&result)
	defer appCleanupStep("retire original disk writer", sequence, func() error {
		return a.stopLogWriterContext(ctx)
	})
	defer appCleanupStep("log terminal producer outcome", sequence, func() error {
		if sequence.result != nil {
			a.appendLog("warn", "выход: завершение исходных фоновых задач не подтверждено")
		}
		return nil
	})
	defer appCleanupStep("join original terminal producers", sequence, func() error {
		return producers.join(ctx, a)
	})
	defer appCleanupStep("cancel original watchdog", sequence, func() error {
		var err error
		producers.watchdog, err = a.cancelWatchdog()
		return err
	})
	defer appCleanupStep("seal and cancel original metrics request", sequence, func() error {
		producers.metrics = a.cancelMetrics(true)
		return nil
	})
	defer appCleanupStep("cancel original tray", sequence, func() error {
		var err error
		producers.tray, err = a.cancelTray()
		return err
	})
	defer appCleanupStep("seal original autostart registry operation", sequence, func() error {
		var err error
		producers.autostart, err = a.cancelAutostart()
		return err
	})
	defer appCleanupStep("cancel original manual diagnostic", sequence, func() error {
		producers.diagnostic = a.cancelManualDiagnostic()
		return nil
	})
	defer appCleanupStep("cancel original background probes", sequence, func() error {
		producers.background = a.cancelBackgroundProbes()
		return nil
	})
	defer appCleanupStep("cancel original engine tailer", sequence, func() error {
		producers.tailer = a.cancelEngineLogTailer()
		return nil
	})
	defer appCleanupStep("cancel original network producers", sequence, func() error {
		var err error
		producers.poller, producers.dns, err = a.cancelNetWatch()
		return err
	})
	return nil
}

func (producers terminalProducerSet) join(ctx context.Context, a *App) (result error) {
	sequence := &appCleanupSequence{}
	defer sequence.finish(&result)
	defer appCleanupStep("join original autostart registry operation", sequence, func() error {
		return a.joinAutostart(ctx, producers.autostart)
	})
	defer appCleanupStep("join original watchdog", sequence, func() error {
		return a.joinWatchdog(ctx, producers.watchdog)
	})
	defer appCleanupStep("join original metrics request", sequence, func() error {
		return joinMetrics(ctx, producers.metrics)
	})
	defer appCleanupStep("join original background probes", sequence, func() error {
		return a.joinBackgroundProbes(ctx, producers.background)
	})
	defer appCleanupStep("join original network poller", sequence, func() error {
		return a.joinNetworkPoller(ctx, producers.poller)
	})
	defer appCleanupStep("join original DNS observer", sequence, func() error {
		return a.joinServiceDNSObserver(ctx, producers.dns)
	})
	defer appCleanupStep("join original tray", sequence, func() error {
		return a.joinTray(ctx, producers.tray)
	})
	tasks := []struct {
		name string
		done <-chan struct{}
	}{
		{"secret diagnostic", producers.diagnostic},
	}
	for index := len(tasks) - 1; index >= 0; index-- {
		task := tasks[index]
		defer appCleanupStep("join original "+task.name, sequence, func() error {
			return joinAppProducerContext(ctx, task.done, fmt.Errorf("%s: %w", task.name, errAppProducerPending))
		})
	}
	defer appCleanupStep("join original engine tailer", sequence, func() error {
		return a.joinEngineLogTailerContext(ctx, producers.tailer)
	})
	return nil
}
