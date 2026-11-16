// Package imagelease retains the original App/helper files and their ancestors.
// Hash/path/ACL facts do not grant signing, loaded-section or process-tree trust.
package imagelease

import (
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
)

const MaxHelperBytes int64 = 32 << 20
const MaxApplicationBytes = imagecheck.MaxBytes

var ErrPair = errors.New("child bootstrap image pair unavailable")
var ErrCleanup = errors.New("child bootstrap image pair cleanup unconfirmed")
var ErrBusy = errors.New("child bootstrap image pair busy")
