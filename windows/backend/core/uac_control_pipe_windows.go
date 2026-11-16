//go:build windows

package core

import (
	"context"
	"errors"
	"sync"

	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// Transport and purpose-specific token custody stay in the same original owner.
// Token mutex precedes native parent borrow and transport calls; no raw getter.
type uacControlPipe struct {
	native                    controlpipe.Pipe
	tokenMu                   sync.Mutex
	parentToken               windows.Token
	parentTokenAcquiring      bool
	parentTokenCloseAttempted bool
}

func (pipe *uacControlPipe) validateParent(ctx context.Context, lease *updateGUINativeLease, image nativeImageLease) error {
	if pipe == nil || !pipe.tokenMu.TryLock() {
		return ErrElevatedGUIUnavailable
	}
	defer pipe.tokenMu.Unlock()
	if !windows.GetCurrentProcessToken().IsElevated() || pipe.parentToken != 0 || pipe.parentTokenAcquiring || pipe.parentTokenCloseAttempted {
		return ErrElevatedGUIUnavailable
	}
	return lease.withParent(func(parent windows.Handle) error {
		id, err := windows.GetProcessId(parent)
		if err != nil || id == 0 {
			return ErrElevatedGUIUnavailable
		}
		if err := uacControlPipeError(pipe.native.CheckServer(ctx, id, contextError)); err != nil {
			return err
		}
		if err := nativeProcessPathMatches(ctx, parent, image.Executable()); err != nil {
			return err
		}
		pipe.parentTokenAcquiring = true
		err = windows.OpenProcessToken(parent, windows.TOKEN_QUERY, &pipe.parentToken)
		pipe.parentTokenAcquiring = false
		if err != nil {
			return err
		}
		self, ok := image.(*windowsVerifierImage)
		user, err := fileacl.UserSID(pipe.parentToken)
		if !ok || self == nil || self.image == nil || err != nil || !self.image.trust.matchesUser(user) {
			return ErrElevatedGUIUnavailable
		}
		if err := pipe.closeParentToken(); err != nil {
			return err
		}
		return uacControlPipeError(pipe.native.CheckServer(ctx, id, contextError))
	})
}

// tokenMu held. Unknown Close/acquisition never grants a retry or absence proof.
func (pipe *uacControlPipe) closeParentToken() error {
	if pipe.parentTokenAcquiring {
		return ErrHelperCleanup
	}
	if pipe.parentToken == 0 {
		return nil
	}
	if pipe.parentTokenCloseAttempted || !nativelease.ValidHandleValue(windows.Handle(pipe.parentToken)) {
		return ErrHelperCleanup
	}
	pipe.parentTokenCloseAttempted = true
	if err := pipe.parentToken.Close(); err != nil {
		return errors.Join(ErrHelperCleanup, err)
	}
	pipe.parentToken = 0
	return nil
}

func (pipe *uacControlPipe) close() error {
	if pipe == nil {
		return nil
	}
	pipe.native.Retire() // token cleanup uncertainty cannot leave transport reusable
	if !pipe.tokenMu.TryLock() {
		return ErrHelperCleanup
	}
	defer pipe.tokenMu.Unlock()
	if err := pipe.closeParentToken(); err != nil {
		return err
	}
	return uacControlPipeError(pipe.native.Close())
}

func (pipe *uacControlPipe) closeAfterServer(ctx context.Context) error {
	if pipe == nil {
		return ErrHelperCleanup
	}
	pipe.native.Retire()
	if !pipe.tokenMu.TryLock() {
		return ErrHelperCleanup
	}
	defer pipe.tokenMu.Unlock()
	if pipe.parentToken != 0 || pipe.parentTokenAcquiring {
		return ErrHelperCleanup
	}
	return uacControlPipeError(pipe.native.CloseAfterServer(ctx, contextError))
}
