//go:build windows

package spawnprep

import (
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// CloseOnce operates on original caller-owned slots, never an acquired PID or a
// copied handle. Caller serializes and never resets attempted or replaces handle.
// Unknown closure can have consumed the numeric value: a later retry must not
// close an unrelated reused object. Zero records only no acquisition or a
// previously confirmed close; both original slots remain reachable on failure.
func CloseOnce(handle *windows.Handle, attempted *bool) error {
	if handle == nil || attempted == nil {
		return ErrResources
	}
	if *handle == 0 {
		return nil
	}
	if *attempted || !nativelease.ValidHandleValue(*handle) {
		return ErrResources
	}
	*attempted = true
	if err := windows.CloseHandle(*handle); err != nil {
		return ErrResources
	}
	*handle = 0
	return nil
}
