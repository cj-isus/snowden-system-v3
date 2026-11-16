//go:build windows

package spawnprep

import (
	"errors"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

var brokerGetHandleInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")

func brokerHandle(raw uint64) error {
	handle := windows.Handle(raw)
	if uint64(handle) != raw || !nativelease.ValidHandleValue(handle) {
		return ErrProcess
	}
	if err := brokerGetHandleInformation.Find(); err != nil {
		return errors.Join(ErrProcess, err)
	}
	var flags uint32
	ok, _, _ := brokerGetHandleInformation.Call(uintptr(handle), uintptr(unsafe.Pointer(&flags)))
	if ok == 0 || flags != 0 { // neither inherited nor protected from checked Close
		return ErrProcess
	}
	return nil
}

// Original native outputs are retained before OpenProcessToken can fail or stop
// returning. Tokens remain query-only; their primary-user SID uses the existing
// bounded fileacl codec. No token/process is acquired using a persisted PID.
type brokerIdentity struct {
	user                      *windows.SID
	ids                       [2]uint32 // comparisons from original handles, never acquisition sources
	tokens                    [2]windows.Token
	acquiring, closeAttempted [2]bool
}

func (identity *brokerIdentity) prepare(processes [2]windows.Handle, purpose childbootstrap.BrokerPurpose, singleOrigin bool) error {
	if identity.user != nil || identity.tokens != [2]windows.Token{} {
		return ErrProcess
	}
	var err error
	identity.user, err = fileacl.CurrentUser()
	if err != nil || identity.user == nil || !identity.user.IsValid() || !windows.GetCurrentProcessToken().IsElevated() {
		return errors.Join(ErrProcess, err)
	}
	for index, process := range processes {
		if (purpose == childbootstrap.BrokerControl || singleOrigin) && index == 1 {
			break
		}
		if !nativelease.ValidHandleValue(process) {
			return ErrProcess
		}
		id, err := windows.GetProcessId(process)
		if err != nil || id == 0 || id == windows.GetCurrentProcessId() || index == 1 && id == identity.ids[0] {
			return errors.Join(ErrProcess, err)
		}
		state, err := windows.WaitForSingleObject(process, 0)
		if err != nil || state != uint32(windows.WAIT_TIMEOUT) {
			return errors.Join(ErrProcess, err)
		}
		identity.ids[index] = id
		identity.acquiring[index] = true
		err = windows.OpenProcessToken(process, windows.TOKEN_QUERY, &identity.tokens[index])
		identity.acquiring[index] = false
		if err != nil || !nativelease.ValidHandleValue(windows.Handle(identity.tokens[index])) {
			return errors.Join(ErrProcess, err)
		}
	}
	return nil
}

func (identity *brokerIdentity) validateSelf() error {
	user, err := fileacl.CurrentUser()
	if err != nil || identity.user == nil || user == nil || !user.Equals(identity.user) || !windows.GetCurrentProcessToken().IsElevated() {
		return errors.Join(ErrProcess, err)
	}
	return nil
}

func (identity *brokerIdentity) validate(processes [2]windows.Handle, purpose childbootstrap.BrokerPurpose, requireAlive, singleOrigin bool) error {
	if err := identity.validateSelf(); err != nil {
		return err
	}
	var previous uint32
	for index, process := range processes {
		if (purpose == childbootstrap.BrokerControl || singleOrigin) && index == 1 {
			break
		}
		if identity.acquiring[index] || identity.closeAttempted[index] || !nativelease.ValidHandleValue(windows.Handle(identity.tokens[index])) {
			return ErrProcess
		}
		id, err := windows.GetProcessId(process)
		if err != nil || id == 0 || id != identity.ids[index] || id == windows.GetCurrentProcessId() || index == 1 && id == previous {
			return errors.Join(ErrProcess, err)
		}
		previous = id // object-identity comparison only
		state, err := windows.WaitForSingleObject(process, 0)
		if err != nil || state != uint32(windows.WAIT_TIMEOUT) && (requireAlive || state != windows.WAIT_OBJECT_0) {
			return errors.Join(ErrProcess, err)
		}
		user, err := fileacl.UserSID(identity.tokens[index])
		if err != nil || user == nil || !user.Equals(identity.user) ||
			purpose == childbootstrap.BrokerSuccessor && index == 0 && !identity.tokens[index].IsElevated() {
			return errors.Join(ErrProcess, err)
		}
	}
	return nil
}

func (identity *brokerIdentity) closed() bool {
	return identity.tokens == [2]windows.Token{} && !identity.acquiring[0] && !identity.acquiring[1]
}

func (identity *brokerIdentity) close() error {
	for index := range identity.tokens {
		if identity.acquiring[index] {
			return ErrProcess
		}
		if err := CloseOnce((*windows.Handle)(&identity.tokens[index]), &identity.closeAttempted[index]); err != nil {
			return err
		}
	}
	return nil
}
