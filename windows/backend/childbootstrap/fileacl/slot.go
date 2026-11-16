// Package fileacl owns the native descriptor returned for a Windows file ACL
// query. Callers retain the original file handle and choose the trusted SID set.
// The policy is conservative allow-ACE inspection, not effective-access analysis.
package fileacl

import "errors"

var ErrACL = errors.New("child bootstrap file ACL unavailable or untrusted")
var ErrCleanup = errors.New("child bootstrap file ACL retirement unconfirmed")
var ErrBusy = errors.New("child bootstrap file ACL owner busy")
