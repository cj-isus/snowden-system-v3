package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	errAutostartUnavailable = errors.New("состояние или изменение автозапуска не подтверждено")
	errAutostartBusy        = errors.New("исходная операция автозапуска ещё не завершена")
)

// One process-wide native operation, shared by the tray and Wails. Uncertain
// acquisition/Close retains the original owner; no replacement or retry Close.
var autostartRegistry struct {
	mu     sync.Mutex
	app    *App
	active *autostartRegistryTask
	sealed atomic.Bool
}

type autostartRegistryTask struct {
	app  *App
	done chan struct{}
	key  windows.Handle

	openStarted, openReturned, openUnknown bool
	closeStarted, closeReturned            bool
	bodyReturned, cleanupReturned          bool // immutable after original done
	fault                                  error
}

func autostartKeyValid(key windows.Handle) bool {
	low := uint32(key)
	return nativelease.ValidHandleValue(key) && !(low >= 0x80000000 && low <= 0x800000ff)
}

func reserveAutostart(app *App) (*autostartRegistryTask, error) {
	if !autostartRegistry.mu.TryLock() {
		return nil, errAutostartBusy
	}
	defer autostartRegistry.mu.Unlock()
	if autostartRegistry.sealed.Load() || app != nil && app.closing.Load() ||
		autostartRegistry.app != nil && autostartRegistry.app != app {
		return nil, errAutostartUnavailable
	}
	if autostartRegistry.active != nil {
		return nil, errAutostartBusy
	}
	if app != nil {
		autostartRegistry.app = app
	}
	task := &autostartRegistryTask{app: app, done: make(chan struct{})}
	autostartRegistry.active = task // original task before command/native work
	return task, nil
}

func accessAutostart(app *App, write, wanted bool) (bool, error) {
	task, err := reserveAutostart(app)
	if err != nil {
		return false, err
	}
	return task.run(write, wanted)
}

func (task *autostartRegistryTask) run(write, wanted bool) (value bool, result error) {
	returned := false
	sequence := &appCleanupSequence{}
	var cleanupResult error
	defer task.finish(&returned, &value, &result, sequence)
	defer sequence.finish(&cleanupResult)
	defer appCleanupStep("close original autostart registry key", sequence, task.closeKey)
	defer func() {
		if payload := recover(); payload != nil || !returned {
			task.fault = errAutostartUnavailable // never expose a panic payload/value
			result = errors.Join(result, task.fault)
		}
	}()
	value, result = task.perform(write, wanted)
	returned = true // includes original body defers, not the native Close below
	return
}

func (task *autostartRegistryTask) finish(returned, value *bool, result *error, sequence *appCleanupSequence) {
	task.bodyReturned, task.cleanupReturned = *returned, sequence.drained && sequence.result == nil
	if !task.bodyReturned || !task.cleanupReturned {
		task.fault = errors.Join(task.fault, errAutostartUnavailable, sequence.result)
	}
	autostartRegistry.mu.Lock()
	defer autostartRegistry.mu.Unlock()
	if autostartRegistry.active != task {
		task.fault = errors.Join(task.fault, errAutostartUnavailable)
	}
	if task.fault != nil {
		*value, *result = false, errors.Join(*result, task.fault)
	} else {
		autostartRegistry.active = nil // known original body/cleanup; no native handle remains
	}
	if autostartRegistry.sealed.Load() || task.app != nil && task.app.closing.Load() {
		*value, *result = false, errors.Join(*result, errAutostartUnavailable)
	}
	close(task.done) // original outcome is immutable before any join can observe it
}

func (task *autostartRegistryTask) closeKey() error {
	if task.openUnknown || task.openStarted && !task.openReturned {
		return errAutostartUnavailable
	}
	if task.key == 0 {
		return nil // no native attempt, or ordinary failed Open with no output handle
	}
	if task.closeStarted || !autostartKeyValid(task.key) {
		return errAutostartUnavailable
	}
	task.closeStarted = true
	err := windows.RegCloseKey(task.key)
	task.closeReturned = true
	if err != nil {
		return errors.Join(errAutostartUnavailable, err)
	}
	task.key = 0
	return nil
}

func (task *autostartRegistryTask) perform(write, wanted bool) (bool, error) {
	expected, err := autostartCommand()
	if err != nil {
		return false, err
	}
	path, err := windows.UTF16PtrFromString(autostartRunKey)
	if err != nil || autostartRegistry.sealed.Load() || task.app != nil && task.app.closing.Load() {
		return false, errAutostartUnavailable
	}
	access := uint32(registry.QUERY_VALUE)
	if write {
		access |= registry.SET_VALUE
	}
	task.openStarted = true
	// Unlike registry.OpenKey, preserve the native output on an error/unwind.
	err = windows.RegOpenKeyEx(windows.Handle(registry.CURRENT_USER), path, 0, access, &task.key)
	task.openReturned = true
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) && task.key == 0 && (!write || !wanted) {
			return false, nil
		}
		return false, errAutostartUnavailable
	}
	if !autostartKeyValid(task.key) {
		task.openUnknown = true // successful native status without a usable original handle
		return false, errAutostartUnavailable
	}
	key := registry.Key(task.key)
	current, err := readAutostartValue(key, expected)
	if err != nil || !write || current == wanted {
		return current, err
	}
	if autostartRegistry.sealed.Load() || task.app != nil && task.app.closing.Load() {
		return false, errAutostartUnavailable
	}
	// Only the explicit user operation writes; a foreign/unsupported value was
	// rejected above. External registry writers are not covered by this mutex.
	if wanted {
		err = key.SetStringValue(autostartValueName, expected)
	} else {
		err = key.DeleteValue(autostartValueName)
	}
	if err != nil {
		return false, errAutostartUnavailable
	}
	actual, err := readAutostartValue(key, expected)
	if err != nil {
		return false, err
	}
	if actual != wanted {
		return false, errors.New("контрольное чтение автозапуска не подтвердило запрос")
	}
	return actual, nil // success still requires original Close and final seal check
}

func (a *App) cancelAutostart() (*autostartRegistryTask, error) {
	autostartRegistry.mu.Lock()
	defer autostartRegistry.mu.Unlock()
	if autostartRegistry.app != nil && autostartRegistry.app != a {
		return nil, errAutostartUnavailable
	}
	autostartRegistry.app = a
	autostartRegistry.sealed.Store(true) // closes admission; does not pretend to cancel Reg*
	return autostartRegistry.active, nil
}

func (a *App) joinAutostart(ctx context.Context, task *autostartRegistryTask) error {
	if task == nil {
		return nil // captured only after sealing process-wide admission
	}
	if task.app != nil && task.app != a {
		return errAutostartUnavailable
	}
	if err := joinAppProducerContext(ctx, task.done, errAutostartUnavailable); err != nil {
		return err
	}
	if !task.bodyReturned || !task.cleanupReturned || task.fault != nil {
		return errors.Join(errAutostartUnavailable, task.fault)
	}
	return nil // original frames/RegCloseKey status, not durable login or kernel proof
}
