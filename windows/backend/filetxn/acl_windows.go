//go:build windows

package filetxn

import (
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"golang.org/x/sys/windows"
)

var ErrWindowsFileACL = errors.New("filetxn: Windows file ACL unavailable or untrusted")

type WindowsFileACLSlot = fileacl.Slot

// Compatibility only: one retained slot, no handle/path map or unbounded queue.
// Original file owners should use a per-file slot through the owned API below.
var standaloneWindowsFileACL WindowsFileACLSlot

func ValidateWindowsFileACL(handle windows.Handle, directory bool, trusted func(*windows.SID) bool) error {
	return ValidateWindowsFileACLOwned(&standaloneWindowsFileACL, handle, directory, trusted)
}

// The caller retains slot before the query and closes it before its original file
// handle. trusted is a pure transient predicate; unknown native cleanup remains
// with the exact slot and cannot be erased by substituting a new one.
func ValidateWindowsFileACLOwned(slot *WindowsFileACLSlot, handle windows.Handle, directory bool, trusted func(*windows.SID) bool) error {
	if slot == nil {
		return ErrWindowsFileACL
	}
	if err := slot.Validate(handle, directory, trusted); err != nil {
		return errors.Join(ErrWindowsFileACL, err)
	}
	return nil
}
