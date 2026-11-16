//go:build windows

package core

import (
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// Protected by proxyOwnerMu (and ProxyManager.mu for a writer). Reserve the
// containing owner before open. Native output points into this original object,
// including failed/abnormal Open, never a helper-local handle lost on unwind.
type proxyRegistryKeyOwner struct {
	handle        windows.Handle
	path          []uint16
	openStarted   bool
	openReturned  bool
	openUnknown   bool
	openErr       error
	closeStarted  bool
	closeReturned bool
	closeErr      error
	closed        bool
	read          *proxyRegistryRead // original query buffers/outcomes, including an unreturned frame
}

func validProxyRegistryHandle(handle windows.Handle) bool {
	low := uint32(handle)
	return nativelease.ValidHandleValue(handle) && !(low >= 0x80000000 && low <= 0x800000ff)
}

func (owner *proxyRegistryKeyOwner) ready() bool {
	return owner != nil && owner.openStarted && owner.openReturned && !owner.openUnknown && owner.openErr == nil &&
		validProxyRegistryHandle(owner.handle) && !owner.closeStarted && !owner.closed && (owner.read == nil || owner.read.returned)
}

func (owner *proxyRegistryKeyOwner) open(access uint32) (result error) {
	if owner == nil || owner.openStarted || owner.closeStarted || owner.closed {
		return ErrProxyStateUncertain
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			owner.openUnknown = true
			result = ErrProxyStateUncertain // no panic payload or registry data
		}
	}()
	var err error
	owner.path, err = windows.UTF16FromString(internetSettingsKey)
	if err != nil {
		returned = true
		return err
	}
	owner.openStarted = true
	owner.openErr = windows.RegOpenKeyEx(windows.HKEY_CURRENT_USER, &owner.path[0], 0, access, &owner.handle)
	owner.openReturned = true
	returned = true
	if owner.openErr != nil {
		return owner.openErr
	}
	if !validProxyRegistryHandle(owner.handle) {
		owner.openUnknown = true // successful status without a usable original output
		return ErrProxyStateUncertain
	}
	return nil
}

func (owner *proxyRegistryKeyOwner) close() (result error) {
	if owner == nil || owner.closed {
		return nil
	}
	if owner.closeStarted {
		return errors.Join(ErrProxyStateUncertain, owner.closeErr) // no replay after unknown native Close
	}
	if owner.openUnknown || owner.openStarted && !owner.openReturned {
		return ErrProxyStateUncertain
	}
	if owner.read != nil && !owner.read.returned {
		return ErrProxyStateUncertain // no Close while original query retirement is unknown
	}
	if owner.handle == 0 {
		if !owner.openStarted || owner.openReturned && owner.openErr != nil {
			owner.closed = true // no acquisition, or an ordinary failed Open with zero output
			return nil
		}
		return ErrProxyStateUncertain
	}
	if !validProxyRegistryHandle(owner.handle) {
		return ErrProxyStateUncertain // never pass a reserved/pseudo HKEY to Close
	}
	owner.closeStarted, owner.closeErr = true, ErrProxyStateUncertain
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			owner.closeErr, result = ErrProxyStateUncertain, ErrProxyStateUncertain
		}
	}()
	owner.closeErr = windows.RegCloseKey(owner.handle)
	owner.closeReturned = true
	returned = true
	if owner.closeErr != nil {
		return errors.Join(ErrProxyStateUncertain, owner.closeErr)
	}
	owner.handle, owner.closed = 0, true
	owner.read = nil // known native Close only; dropping references is not secure zeroization
	return nil
}
