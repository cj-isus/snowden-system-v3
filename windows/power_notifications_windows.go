//go:build windows

package main

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	powerDLL        = windows.NewLazySystemDLL("powrprof.dll")
	powerRegister   = powerDLL.NewProc("PowerRegisterSuspendResumeNotification")
	powerUnregister = powerDLL.NewProc("PowerUnregisterSuspendResumeNotification")
)

// Native structure consists of two pointer-sized fields. Context is an opaque
// never-reused integer cookie, never a retained pointer to a Go owner.
type nativePowerParameters struct{ callback, cookie uintptr }

type nativePowerSubscription struct {
	mu                            sync.Mutex
	owner                         *networkPollerOwner
	cookie                        uintptr
	params                        *nativePowerParameters // separately allocated, no Go pointer fields
	pin                           runtime.Pinner
	entered                       sync.WaitGroup
	handle                        windows.Handle
	active                        bool // powerCallbacks.mu
	unknown                       bool
	closed                        bool
	callbackFailed                atomic.Bool
	retireStarted, retireReturned bool
	retireErr                     error
}

var powerCallbacks struct {
	mu        sync.Mutex
	next      uintptr
	current   *nativePowerSubscription // capacity one, including failed retirement
	thunkOnce sync.Once
	thunk     uintptr // one process-lifetime Go stdcall thunk, never one per wake
}

func reserveNativePower(owner *networkPollerOwner) (*nativePowerSubscription, error) {
	if owner == nil {
		return nil, errPowerNotificationUnavailable
	}
	powerCallbacks.thunkOnce.Do(func() { powerCallbacks.thunk = windows.NewCallback(nativePowerCallback) })
	powerCallbacks.mu.Lock()
	defer powerCallbacks.mu.Unlock()
	if powerCallbacks.current != nil || powerCallbacks.next == ^uintptr(0) || powerCallbacks.thunk == 0 {
		return nil, errPowerNotificationUnavailable
	}
	owner.mu.Lock()
	if owner.power != nil || !owner.contextState.currentLocked() || owner.stopping {
		owner.mu.Unlock()
		return nil, errPowerNotificationUnavailable
	}
	powerCallbacks.next++
	entry := &nativePowerSubscription{owner: owner, cookie: powerCallbacks.next, active: true}
	entry.params = &nativePowerParameters{callback: powerCallbacks.thunk, cookie: entry.cookie}
	powerCallbacks.current = entry
	owner.power = entry // retain original partial acquisition before Pin or native registration
	owner.mu.Unlock()
	entry.pin.Pin(entry.params)
	return entry, nil
}

func startNativePowerNotifications(owner *networkPollerOwner) (subscription powerSubscription, resultErr error) {
	var entry *nativePowerSubscription
	var handle windows.Handle
	var nativeStatus uintptr
	nativeStarted, nativeReturned := false, false
	defer func() {
		if payload := recover(); payload != nil || nativeStarted && !nativeReturned {
			if entry != nil {
				entry.handle = handle
				entry.unknown = nativeStarted && handle == 0 && (!nativeReturned || uint32(nativeStatus) == 0)
				subscription = entry // retain even interrupted/partial registration
			}
			resultErr = errors.Join(errPowerNotificationUnavailable, errors.New("power registration interrupted"))
		}
	}()
	if err := powerRegister.Find(); err != nil {
		return nil, errPowerNotificationUnavailable
	}
	if err := powerUnregister.Find(); err != nil {
		return nil, errPowerNotificationUnavailable
	}
	var err error
	entry, err = reserveNativePower(owner)
	if err != nil {
		return nil, err
	}
	// DEVICE_NOTIFY_CALLBACK=2. Return DWORD is authoritative, not LastError.
	nativeStarted = true
	nativeStatus, _, _ = powerRegister.Call(2, uintptr(unsafe.Pointer(entry.params)), uintptr(unsafe.Pointer(&handle)))
	nativeReturned = true
	runtime.KeepAlive(entry.params)
	entry.handle = handle
	if code := uint32(nativeStatus); code != 0 {
		return entry, errors.Join(errPowerNotificationUnavailable, syscall.Errno(code))
	}
	if handle == 0 {
		entry.unknown = true
		return entry, errPowerNotificationUnavailable
	}
	return entry, nil
}

func nativePowerCallback(cookie, event, _ uintptr) (status uintptr) {
	powerCallbacks.mu.Lock()
	entry := powerCallbacks.current
	if entry == nil || !entry.active || entry.cookie != cookie {
		powerCallbacks.mu.Unlock()
		return 0
	}
	entry.entered.Add(1) // no Add after deactivation; retirement never holds this lock over I/O
	powerCallbacks.mu.Unlock()
	defer entry.entered.Done()
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			entry.callbackFailed.Store(true)
			entry.owner.failPowerNotification()
			status = uintptr(windows.ERROR_GEN_FAILURE)
		}
	}()
	entry.owner.notePowerNotification(uintptr(uint32(event))) // native ULONG, not pointer width
	returned = true
	return 0
}

func (entry *nativePowerSubscription) retire() (result error) {
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.retireStarted {
		return entry.retireErr
	}
	entry.retireStarted, entry.retireErr = true, errPowerNotificationUnavailable
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errPowerNotificationUnavailable
		}
		entry.retireReturned, entry.retireErr = returned, result
	}()
	result = entry.retireBody()
	returned = true
	return
}

// entry.mu held. Never replay the original unregister/pin/callback retirement.
func (entry *nativePowerSubscription) retireBody() error {
	if entry.closed {
		return nil
	}
	powerCallbacks.mu.Lock()
	entry.active = false // old callbacks cannot mutate an owner or join a later cookie
	powerCallbacks.mu.Unlock()
	if entry.unknown && entry.handle == 0 {
		return errPowerNotificationUnavailable // no invented handle/cleanup receipt
	}
	if entry.handle != 0 {
		status, _, _ := powerUnregister.Call(uintptr(entry.handle))
		if code := uint32(status); code != 0 {
			return errors.Join(errPowerNotificationUnavailable, syscall.Errno(code))
		}
		entry.handle = 0
	}
	// Unregister is not assumed to join an already-entered Go callback. The
	// closed registry admission and original count cover our callback bodies.
	entry.entered.Wait()
	if entry.callbackFailed.Load() {
		return errPowerNotificationUnavailable
	}
	entry.pin.Unpin()
	entry.params = nil
	entry.closed = true
	powerCallbacks.mu.Lock()
	if powerCallbacks.current == entry {
		powerCallbacks.current = nil
	}
	powerCallbacks.mu.Unlock()
	return nil
}
