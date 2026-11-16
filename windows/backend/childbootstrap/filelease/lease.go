// Package filelease retains one original Windows file handle and its ACL
// descriptor owner. File paths, private identity receipts, trust and surrounding
// process/installer lifetimes remain the responsibility of the caller.
package filelease

import "errors"

var ErrFile = errors.New("child bootstrap original file unavailable")
var ErrCleanup = errors.New("child bootstrap original file cleanup unconfirmed")
