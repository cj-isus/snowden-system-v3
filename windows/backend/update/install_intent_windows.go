//go:build windows

package update

import (
	"bytes"
	"errors"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A single retained original key. Close uncertainty quarantines this process;
// no reopen/retry can turn an unknown native close into a success receipt.
var pairRegistryOwner struct {
	sync.Mutex
	key      windows.Handle
	closeErr error
}

var pairRegistryAPI = windows.NewLazySystemDLL("advapi32.dll")
var pairRegCreate = pairRegistryAPI.NewProc("RegCreateKeyExW")
var pairRegSet = pairRegistryAPI.NewProc("RegSetValueExW")
var pairRegFlush = pairRegistryAPI.NewProc("RegFlushKey")

const pairRegistryPath = `Software\SnowdenSystem\UpdateIntentV1`

func lockPairStartup() (func() error, error) {
	if !floorMu.TryLock() {
		return nil, ErrPairBusy
	}
	path, err := FloorPath()
	if err != nil {
		floorMu.Unlock()
		return nil, err
	}
	release, err := lockFloor(path + ".lock")
	var once sync.Once
	releaseErr := ErrFloorUnavailable
	return func() error {
		once.Do(func() {
			defer floorMu.Unlock()
			if release != nil {
				releaseErr = release()
			} else {
				releaseErr = nil
			}
		})
		return releaseErr
	}, err
}

func pairRegistryStatus(status uintptr) error {
	if status != 0 {
		return errors.Join(ErrPairStateUnavailable, syscall.Errno(status)) // LSTATUS, not GetLastError
	}
	return nil
}

func pairRegistryLock() error {
	if !pairRegistryOwner.TryLock() {
		return ErrPairBusy
	}
	if pairRegistryOwner.key != 0 || pairRegistryOwner.closeErr != nil {
		pairRegistryOwner.Unlock()
		return ErrPairStateUnavailable
	}
	return nil
}

func pairRegistryClose() error {
	if pairRegistryOwner.closeErr != nil {
		return pairRegistryOwner.closeErr
	}
	key := pairRegistryOwner.key
	if key == 0 {
		return nil
	}
	// Predefined HKEY pseudo-handles are never receipts created by this owner.
	low := uint32(key)
	if low >= 0x80000000 && low <= 0x800000ff {
		pairRegistryOwner.closeErr = ErrPairStateUnavailable
		return pairRegistryOwner.closeErr
	}
	pairRegistryOwner.closeErr = ErrPairStateUnavailable
	if err := windows.RegCloseKey(key); err != nil {
		return errors.Join(pairRegistryOwner.closeErr, err)
	}
	pairRegistryOwner.key, pairRegistryOwner.closeErr = 0, nil
	return nil
}

func queryPairRegistry() ([]byte, error) {
	name, err := windows.UTF16PtrFromString("State")
	if err != nil {
		return nil, err
	}
	var buffer [maxPairStateBytes]byte
	var kind uint32
	size := uint32(len(buffer))
	err = windows.RegQueryValueEx(pairRegistryOwner.key, name, nil, &kind, &buffer[0], &size)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return nil, nil
	}
	if err != nil || kind != windows.REG_BINARY || size == 0 || size > uint32(len(buffer)) {
		return nil, errors.Join(ErrPairStateUnavailable, err)
	}
	return bytes.Clone(buffer[:size]), nil
}

func readPairStateNative() (raw []byte, finalErr error) {
	if err := pairRegistryLock(); err != nil {
		return nil, err
	}
	defer pairRegistryOwner.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrPairStateUnavailable)
		}
		finalErr = errors.Join(finalErr, pairRegistryClose())
		if finalErr != nil {
			raw = nil // an uncertain read/close is never initial state
		}
	}()
	path, err := windows.UTF16PtrFromString(pairRegistryPath)
	if err != nil {
		return nil, err
	}
	err = windows.RegOpenKeyEx(windows.HKEY_CURRENT_USER, path, 0, windows.KEY_QUERY_VALUE|windows.KEY_WOW64_64KEY, &pairRegistryOwner.key)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		completed = true
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, finalErr = queryPairRegistry()
	completed = true
	return raw, finalErr
}

type nativePairStateWrite struct {
	expected, next []byte
	locked         bool
	setAttempted   bool
	finished       bool
	result         error
}

func newPairStateWrite(expected, next []byte) pairStateWrite {
	return &nativePairStateWrite{expected: expected, next: next}
}

func (writer *nativePairStateWrite) Set() error {
	if writer.locked || writer.finished || len(writer.next) == 0 || len(writer.next) > maxPairStateBytes {
		return ErrPairStateUnavailable
	}
	if err := pairRegistryLock(); err != nil {
		return err
	}
	writer.locked = true // receipt before the first native open/create
	path, err := windows.UTF16PtrFromString(pairRegistryPath)
	if err != nil {
		return err
	}
	if err := pairRegCreate.Find(); err != nil {
		return err
	}
	if err := pairRegSet.Find(); err != nil {
		return err
	}
	if err := pairRegFlush.Find(); err != nil {
		return err
	}
	status, _, _ := pairRegCreate.Call(uintptr(windows.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(path)), 0, 0, 0,
		uintptr(windows.KEY_QUERY_VALUE|windows.KEY_SET_VALUE|windows.KEY_WOW64_64KEY), 0, uintptr(unsafe.Pointer(&pairRegistryOwner.key)), 0)
	if err := pairRegistryStatus(status); err != nil || pairRegistryOwner.key == 0 {
		return errors.Join(ErrPairStateUnavailable, err)
	}
	current, err := queryPairRegistry()
	if err != nil || !bytes.Equal(current, writer.expected) {
		return errors.Join(ErrPairStateUnavailable, err)
	}
	name, err := windows.UTF16PtrFromString("State")
	if err != nil {
		return err
	}
	writer.setAttempted = true // Set failure does not prove the old value survived
	status, _, _ = pairRegSet.Call(uintptr(pairRegistryOwner.key), uintptr(unsafe.Pointer(name)), 0, windows.REG_BINARY,
		uintptr(unsafe.Pointer(&writer.next[0])), uintptr(len(writer.next)))
	return pairRegistryStatus(status)
}

func (writer *nativePairStateWrite) Finish() (finalErr error) {
	if writer.finished {
		return writer.result
	}
	writer.finished, writer.result = true, ErrPairStateUnavailable
	if !writer.locked {
		writer.result = nil
		return nil
	}
	defer pairRegistryOwner.Unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			finalErr = errors.Join(finalErr, ErrPairStateUnavailable)
		}
		finalErr = errors.Join(finalErr, pairRegistryClose())
		writer.result = finalErr
	}()
	if writer.setAttempted {
		// Flush is deliberately outside the source metadata journal. The value
		// is already visible: failure must retain the App's logical restriction.
		status, _, _ := pairRegFlush.Call(uintptr(pairRegistryOwner.key))
		finalErr = pairRegistryStatus(status)
		current, err := queryPairRegistry()
		if err != nil || !bytes.Equal(current, writer.next) {
			finalErr = errors.Join(finalErr, ErrPairStateUnavailable, err)
		}
	}
	return finalErr
}
