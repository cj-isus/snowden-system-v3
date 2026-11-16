package fileacl

import "errors"

var ErrIdentity = errors.New("child bootstrap file trust identity unavailable")

// TrustedInstallerSID is the existing fixed Windows service SID. It is neither
// an account-name lookup nor authority supplied by a profile or application UI.
const TrustedInstallerSID = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
