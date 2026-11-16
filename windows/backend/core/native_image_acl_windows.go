//go:build windows

package core

import (
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/filelease"
	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"github.com/snowden-system/windows/backend/childbootstrap/imagelease"
	"golang.org/x/sys/windows"
)

// TrustedInstaller is the documented stable Windows service SID, not a local
// account lookup. No SAM/domain/name resolution is needed while holding a lease.
const trustedInstallerSID = fileacl.TrustedInstallerSID

// FILE_DELETE_CHILD is not exported by the declared x/sys version.
const nativeFileDeleteChild windows.ACCESS_MASK = 0x40

type nativeImageTrust struct{ identity fileacl.Trust }

func currentNativeImageTrust() (nativeImageTrust, error) {
	identity, err := fileacl.CurrentTrust()
	if err != nil {
		return nativeImageTrust{}, ErrNativeImageUnavailable
	}
	return nativeImageTrust{identity: identity}, nil
}

// Conservative allow-ACE inspection, not an effective-access/privilege engine.
// Current user/Admins/SYSTEM/TrustedInstaller are trusted local principals.
func (trust nativeImageTrust) trusted(sid *windows.SID) bool {
	return trust.identity.Trusted(sid)
}

func (trust nativeImageTrust) matchesUser(sid *windows.SID) bool {
	return trust.identity.MatchesUser(sid)
}

// Preserve the existing package API and the original typed error boundary.
func (trust nativeImageTrust) Validate(slot *fileacl.Slot, handle windows.Handle, directory bool) error {
	if slot == nil {
		return ErrNativeImageUnavailable
	}
	return nativeImageInspectionError(slot.Validate(handle, directory, trust.trusted))
}

func nativeImageInspectionError(err error) error {
	if err == nil || err == ErrNativeImageUnavailable {
		return err
	}
	if err == imagecheck.ErrImage || err == filelease.ErrFile || err == imagelease.ErrPair {
		return ErrNativeImageUnavailable // retain the exact ordinary inspection sentinel
	}
	if errors.Is(err, imagelease.ErrCleanup) || errors.Is(err, imagelease.ErrBusy) || errors.Is(err, filelease.ErrCleanup) || errors.Is(err, fileacl.ErrCleanup) || errors.Is(err, fileacl.ErrBusy) {
		return errors.Join(ErrNativeImageUnavailable, ErrHelperCleanup, err)
	}
	return errors.Join(ErrNativeImageUnavailable, err)
}
