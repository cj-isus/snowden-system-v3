//go:build windows

package core

import (
	"bytes"
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const proxyAnchorKey = `Software\SnowdenSystem\ProxyRecovery`
const proxyAnchorValue = "AnchorV1"

var proxyAnchorAdvapi = windows.NewLazySystemDLL("advapi32.dll")
var proxyAnchorCreate = proxyAnchorAdvapi.NewProc("RegCreateKeyExW")
var proxyAnchorSet = proxyAnchorAdvapi.NewProc("RegSetValueExW")
var proxyAnchorFlush = proxyAnchorAdvapi.NewProc("RegFlushKey")

// Original fixed-purpose HKEY owner, held through this ProxyManager lifetime.
// All native outputs/inputs remain reachable after unknown API or Close. Only
// the private anchor value is written, never Internet Settings through this key.
type proxyReceiptAnchor struct {
	handle                                       windows.Handle
	path, name                                   []uint16
	disposition                                  uint32
	opened, missingKey, created, loaded          bool
	emptyValue                                   bool              // twice observed absent through an existing original HKEY; not new-key authority
	enrollment                                   *proxyStageReplay // sole original metadata-only initial replayer
	pending, closeStarted, closeReturned, closed bool
	query                                        *proxyRegistryQuery // existing bounded native query record, not a new dynamic reader
	write                                        []byte
	current                                      []byte
	state                                        proxyAnchorState
	crypto                                       proxyReceiptCrypto
	failure, closeErr                            error
}

func newProxyReceiptAnchor() *proxyReceiptAnchor {
	return &proxyReceiptAnchor{crypto: proxyReceiptCrypto{domain: proxyCryptoAnchor}}
}

// Known synchronous operation return, not closure of the intentionally held HKEY.
func (anchor *proxyReceiptAnchor) idle() bool {
	return anchor != nil && !anchor.pending && anchor.crypto.retired() && (anchor.query == nil || anchor.query.returned) &&
		(!anchor.closeStarted || anchor.closed && anchor.closeReturned && anchor.closeErr == nil)
}

func (anchor *proxyReceiptAnchor) open() (result error) {
	if anchor == nil || anchor.opened || anchor.closeStarted {
		return ErrProxyAnchorUnavailable
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			anchor.pending = true
			result = ErrProxyStateUncertain
		}
		if result != nil {
			anchor.failure = result
		}
	}()
	var err error
	anchor.path, err = windows.UTF16FromString(proxyAnchorKey)
	if err != nil {
		returned = true
		return ErrProxyAnchorUnavailable
	}
	anchor.name, err = windows.UTF16FromString(proxyAnchorValue)
	if err != nil {
		returned = true
		return ErrProxyAnchorUnavailable
	}
	anchor.pending = true
	err = windows.RegOpenKeyEx(windows.HKEY_CURRENT_USER, &anchor.path[0], 0, registry.QUERY_VALUE|registry.SET_VALUE, &anchor.handle)
	anchor.opened = true
	anchor.pending = false
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && anchor.handle == 0 {
		anchor.missingKey, returned = true, true
		return nil
	}
	if err != nil || !validProxyRegistryHandle(anchor.handle) {
		if anchor.handle != 0 || err == nil {
			anchor.pending = true
		}
		returned = true
		return ErrProxyAnchorUnavailable
	}
	raw, err := anchor.readCipher(true)
	if err != nil {
		returned = true
		return err
	}
	if len(raw) == 0 {
		anchor.emptyValue, returned = true, true
		return nil // store must prove an initial no-write stage; absence grants no adoption or reset
	}
	plain, err := anchor.crypto.transform(raw, false)
	if err != nil {
		returned = true
		return err
	}
	anchor.state, err = decodeProxyAnchor(plain)
	clear(plain)
	if err != nil {
		returned = true
		return err
	}
	anchor.current, anchor.loaded, returned = raw, true, true
	return nil
}

// Two fixed <=4KiB queries through the same original HKEY. A missing value in an
// existing key is classified only; a private initial replayer must separately
// prove eligibility before any publication. Absence cannot erase anchor history.
func (anchor *proxyReceiptAnchor) readCipher(allowMissing bool) ([]byte, error) {
	if !anchor.idle() || !validProxyRegistryHandle(anchor.handle) || anchor.closeStarted {
		return nil, ErrProxyStateUncertain
	}
	var first []byte
	missing := false
	for pass := 0; pass < 2; pass++ {
		query := &proxyRegistryQuery{name: anchor.name, buffer: make([]byte, maxProxyAnchorCipher), size: maxProxyAnchorCipher}
		anchor.query = query // retained before native query
		anchor.pending, query.started = true, true
		query.err = windows.RegQueryValueEx(anchor.handle, &query.name[0], nil, &query.kind, &query.buffer[0], &query.size)
		runtime.KeepAlive(anchor)
		query.returned, anchor.pending = true, false
		isMissing := errors.Is(query.err, windows.ERROR_FILE_NOT_FOUND)
		if isMissing {
			if !allowMissing || pass == 1 && !missing {
				return nil, ErrProxyAnchorUnavailable
			}
		} else if query.err != nil || query.kind != registry.BINARY || query.size == 0 || query.size > maxProxyAnchorCipher || pass == 1 && missing {
			return nil, ErrProxyAnchorUnavailable
		}
		if pass == 0 {
			missing = isMissing
			if !missing {
				first = bytes.Clone(query.buffer[:query.size])
			}
		} else if !missing && !bytes.Equal(first, query.buffer[:query.size]) {
			return nil, ErrProxyAnchorUnavailable
		}
	}
	return first, nil
}

func (anchor *proxyReceiptAnchor) create() error {
	if !anchor.idle() || !anchor.opened || !anchor.missingKey || anchor.handle != 0 || anchor.loaded || anchor.created || anchor.failure != nil {
		return ErrProxyAnchorUnavailable
	}
	anchor.pending = true
	if err := proxyAnchorCreate.Find(); err != nil {
		anchor.pending = false
		return ErrProxyAnchorUnavailable
	}
	status, _, _ := proxyAnchorCreate.Call(uintptr(windows.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(&anchor.path[0])), 0, 0, 0,
		uintptr(registry.QUERY_VALUE|registry.SET_VALUE), 0, uintptr(unsafe.Pointer(&anchor.handle)), uintptr(unsafe.Pointer(&anchor.disposition)))
	anchor.pending = false
	runtime.KeepAlive(anchor)
	if status != 0 || !validProxyRegistryHandle(anchor.handle) || anchor.disposition != 1 {
		if status != 0 && anchor.handle != 0 || status == 0 && !validProxyRegistryHandle(anchor.handle) {
			anchor.pending = true
		}
		return ErrProxyAnchorUnavailable // opened-existing is never permission to overwrite it
	}
	anchor.created, anchor.missingKey = true, false
	return nil
}

// Prepare/commit callers retain this record and the store before entry. No
// operation can replace unknown original native/crypto completion.
func (anchor *proxyReceiptAnchor) publish(next proxyAnchorState) (result error) {
	if !anchor.idle() || anchor.failure != nil || anchor.closed || !validProxyAnchorStep(anchor.state, next) {
		return ErrProxyAnchorUnavailable
	}
	if anchor.emptyValue && (anchor.enrollment == nil || !anchor.enrollment.admitsEmptyAnchor(next)) {
		return ErrProxyAnchorMigrationRequired
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			anchor.pending = true
			result = ErrProxyStateUncertain
		}
		if result != nil {
			anchor.failure = result
		}
	}()
	plain, err := encodeProxyAnchor(next)
	if err != nil {
		returned = true
		return err
	}
	anchor.write, err = anchor.crypto.transform(plain, true)
	if anchor.crypto.retired() {
		clear(plain)
	}
	if err != nil {
		returned = true
		return err
	}
	// Complete fallible encoding/DPAPI before creating a key. A crash between
	// Create and Set still requires the authenticated initial no-write replayer.
	if anchor.missingKey {
		if err := anchor.create(); err != nil {
			returned = true
			return err
		}
	}
	if anchor.emptyValue && (anchor.enrollment == nil || !anchor.enrollment.admitsEmptyAnchor(next)) {
		returned = true
		return ErrProxyAnchorMigrationRequired
	}
	before, err := anchor.readCipher(anchor.created && !anchor.loaded || anchor.emptyValue)
	if err != nil || !bytes.Equal(before, anchor.current) {
		returned = true
		return ErrProxyAnchorUnavailable
	}
	anchor.pending = true
	if err := proxyAnchorSet.Find(); err != nil {
		anchor.pending = false
		returned = true
		return ErrProxyAnchorUnavailable
	}
	status, _, _ := proxyAnchorSet.Call(uintptr(anchor.handle), uintptr(unsafe.Pointer(&anchor.name[0])), 0, uintptr(registry.BINARY),
		uintptr(unsafe.Pointer(&anchor.write[0])), uintptr(len(anchor.write)))
	anchor.pending = false
	runtime.KeepAlive(anchor)
	if status != 0 {
		returned = true
		return ErrProxyAnchorUnavailable
	}
	observed, err := anchor.readCipher(false)
	if err != nil || !bytes.Equal(observed, anchor.write) {
		returned = true
		return ErrProxyAnchorUnavailable
	}
	anchor.pending = true
	if err := proxyAnchorFlush.Find(); err != nil {
		anchor.pending = false
		returned = true
		return ErrProxyAnchorUnavailable
	}
	status, _, _ = proxyAnchorFlush.Call(uintptr(anchor.handle))
	anchor.pending = false
	if status != 0 {
		returned = true
		return errors.Join(ErrProxyAnchorUnavailable, syscall.Errno(status))
	}
	observed, err = anchor.readCipher(false)
	if err != nil || !bytes.Equal(observed, anchor.write) {
		returned = true
		return ErrProxyAnchorUnavailable
	}
	anchor.state, anchor.current, anchor.loaded = next, anchor.write, true
	anchor.emptyValue = false // only after actual Set/query/flush/requery completion
	anchor.write = nil        // after known Set/query/flush return only
	returned = true
	return nil
}

func (anchor *proxyReceiptAnchor) check() (result error) {
	if anchor == nil || anchor.state.pending {
		return ErrProxyAnchorUnavailable
	}
	return anchor.checkCurrent()
}

// Exact-byte consistency only. The current process may check its own prepared
// anchor before publishing the file; this never adopts a pending restart record.
func (anchor *proxyReceiptAnchor) checkCurrent() (result error) {
	if anchor == nil || !anchor.idle() || anchor.failure != nil || !anchor.loaded || anchor.closed {
		return ErrProxyAnchorUnavailable
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			anchor.pending = true
			result = ErrProxyStateUncertain
		}
		if result != nil {
			anchor.failure = result
		}
	}()
	raw, err := anchor.readCipher(false)
	returned = true
	if err != nil || !bytes.Equal(raw, anchor.current) {
		return ErrProxyAnchorUnavailable
	}
	return nil
}

func (anchor *proxyReceiptAnchor) close() (result error) {
	if anchor == nil || anchor.closed {
		return nil
	}
	if !anchor.idle() || anchor.closeStarted {
		return ErrProxyStateUncertain
	}
	anchor.closeStarted, anchor.closeErr = true, ErrProxyStateUncertain
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
		anchor.closeReturned, anchor.closeErr = returned, result
	}()
	if anchor.handle != 0 {
		if !validProxyRegistryHandle(anchor.handle) {
			returned = true
			return ErrProxyStateUncertain
		}
		if err := windows.RegCloseKey(anchor.handle); err != nil {
			returned = true
			return ErrProxyStateUncertain
		}
		anchor.handle = 0
	}
	anchor.closed, returned = true, true
	return nil
}
