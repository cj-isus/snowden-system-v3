//go:build windows

package core

import (
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

type proxyPriorProcessOutcome uint8

const (
	proxyPriorProcessUnknown proxyPriorProcessOutcome = iota
	proxyPriorProcessRunning
	proxyPriorProcessExited
	proxyPriorProcessReusedPID
)

// One observation of a DPAPI-decoded unresolved receipt, never a process killer
// or restoration grant. The store retains this object before OpenProcess.
// Persisted PID/creation are comparison inputs, not a native object capability;
// the original opened handle is held through every query/wait/checked Close.
type proxyReceiptProcess struct {
	expectedPID                                uint32
	expectedCreation                           uint64
	currentUser, observedUser                  *windows.SID // bounded Go-owned primary SID snapshots
	process                                    windows.Handle
	token                                      windows.Token
	pid                                        uint32
	times                                      [2][4]windows.Filetime // original native creation/exit/kernel/user outputs
	wait                                       uint32
	outcome                                    proxyPriorProcessOutcome
	attempted, returned, pending               bool
	processOpenReturned, tokenOpenReturned     bool
	processCloseAttempted, tokenCloseAttempted bool
	closeStarted, closeReturned, closed        bool
	result, closeErr                           error
}

func (owner *proxyReceiptProcess) retired() bool {
	return owner != nil && owner.closed && owner.returned && owner.closeReturned &&
		!owner.pending && owner.process == 0 && owner.token == 0 && owner.closeErr == nil
}

func (owner *proxyReceiptProcess) observe() (result error) {
	if owner == nil || owner.attempted || owner.closeStarted {
		return ErrProxyPriorOwnerUnresolved
	}
	owner.attempted = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			owner.outcome, owner.result, result = proxyPriorProcessUnknown, ErrProxyStateUncertain, ErrProxyStateUncertain
			return // no speculative Close after an unreturned original native call
		}
		owner.returned = true
		if err := owner.close(); err != nil {
			owner.outcome = proxyPriorProcessUnknown
			result = errors.Join(ErrProxyStateUncertain, err)
		}
		owner.result = result
	}()
	if owner.expectedPID == 0 || owner.expectedCreation == 0 || owner.currentUser == nil {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	owner.pending = true
	var err error
	owner.process, err = windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, owner.expectedPID)
	owner.processOpenReturned = true
	owner.pending = false
	if err != nil {
		if owner.process != 0 {
			owner.pending = true
		} // partial/contradictory output is not an acquisition receipt
		returned = true
		return ErrProxyPriorOwnerUnresolved // missing/access denied/invalid PID never means dead
	}
	if !nativelease.ValidHandleValue(owner.process) {
		owner.pending, returned = true, true
		return ErrProxyStateUncertain
	}
	owner.pending = true
	owner.pid, err = windows.GetProcessId(owner.process)
	owner.pending = false
	if err != nil || owner.pid != owner.expectedPID {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	owner.pending = true
	err = windows.GetProcessTimes(owner.process, &owner.times[0][0], &owner.times[0][1], &owner.times[0][2], &owner.times[0][3])
	owner.pending = false
	if err != nil {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	created := proxyProcessFiletime(owner.times[0][0])
	if created == 0 {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	if created != owner.expectedCreation {
		owner.outcome, returned = proxyPriorProcessReusedPID, true
		return ErrProxyPriorOwnerChanged // never interpret a replacement's exit as the original's death
	}
	owner.pending = true
	err = windows.OpenProcessToken(owner.process, windows.TOKEN_QUERY, &owner.token)
	owner.tokenOpenReturned = true
	owner.pending = false
	if err != nil {
		if owner.token != 0 {
			owner.pending = true
		}
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	if !nativelease.ValidHandleValue(windows.Handle(owner.token)) {
		owner.pending, returned = true, true
		return ErrProxyStateUncertain
	}
	owner.pending = true
	owner.observedUser, err = fileacl.UserSID(owner.token)
	owner.pending = false
	if err != nil || owner.observedUser == nil {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	owner.pending = true
	sameUser := owner.observedUser.Equals(owner.currentUser)
	owner.pending = false
	if !sameUser {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	owner.pending = true
	owner.wait, err = windows.WaitForSingleObject(owner.process, 0)
	owner.pending = false
	if err != nil {
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
	switch owner.wait {
	case uint32(windows.WAIT_TIMEOUT):
		owner.outcome, returned = proxyPriorProcessRunning, true
		return ErrProxyPriorOwnerRunning // an observation at this call, not perpetual liveness
	case windows.WAIT_OBJECT_0:
		owner.pending = true
		err = windows.GetProcessTimes(owner.process, &owner.times[1][0], &owner.times[1][1], &owner.times[1][2], &owner.times[1][3])
		owner.pending = false
		if err != nil || proxyProcessFiletime(owner.times[1][0]) != owner.expectedCreation || proxyProcessFiletime(owner.times[1][1]) == 0 {
			returned = true
			return ErrProxyPriorOwnerUnresolved
		}
		owner.outcome, returned = proxyPriorProcessExited, true
		return ErrProxyRecoveryRequired // observed original exit alone cannot authorize registry repair
	default:
		returned = true
		return ErrProxyPriorOwnerUnresolved
	}
}

func proxyProcessFiletime(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}

// Reuse original-slot CloseOnce. No TerminateProcess, duplicate, reopen, polling
// loop, helper or finalizer. Unknown Close retains both this owner and the store.
func (owner *proxyReceiptProcess) close() (result error) {
	if owner == nil || owner.closeStarted || owner.pending ||
		owner.process != 0 && !owner.processOpenReturned || owner.token != 0 && !owner.tokenOpenReturned {
		return ErrProxyStateUncertain
	}
	owner.closeStarted, owner.closeErr = true, ErrProxyStateUncertain
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
		owner.closeReturned, owner.closeErr = returned, result
	}()
	if err := spawnprep.CloseOnce((*windows.Handle)(&owner.token), &owner.tokenCloseAttempted); err != nil {
		returned = true
		return ErrProxyStateUncertain
	}
	if err := spawnprep.CloseOnce(&owner.process, &owner.processCloseAttempted); err != nil {
		returned = true
		return ErrProxyStateUncertain
	}
	owner.closed, returned = true, true
	return nil
}
