//go:build windows

package fileacl

import (
	"encoding/binary"
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const maxSIDBytes = 8 + 4*15
const tokenUserBytes = 128

type tokenUserBuffer struct {
	user windows.Tokenuser // keep the native-written SID pointer visible to Go
	sid  [tokenUserBytes - unsafe.Sizeof(windows.Tokenuser{})]byte
}

// Trust is a Go-owned immutable snapshot of the original process-primary user
// and fixed installer SID. Value copies own no native resource; zero Trust
// rejects every SID. This is a conservative principal set, not effective access.
type Trust struct{ user, installer *windows.SID }

// CurrentUser returns a private Go-owned SID from the process primary token,
// matching the existing selector. No token handle needs Close: the pseudo token
// is borrowed. The fixed TOKEN_USER buffer never grows or retries a query.
func CurrentUser() (*windows.SID, error) {
	return UserSID(windows.GetCurrentProcessToken())
}

// UserSID queries a caller-owned token using the same bounded Go storage. It
// borrows the token and neither opens, closes nor transfers its native handle.
func UserSID(token windows.Token) (*windows.SID, error) {
	if token == 0 {
		return nil, ErrIdentity
	}
	storage := new(tokenUserBuffer)
	defer runtime.KeepAlive(storage)
	var returned uint32
	base := unsafe.Pointer(storage)
	if err := windows.GetTokenInformation(token, windows.TokenUser,
		(*byte)(base), tokenUserBytes, &returned); err != nil {
		return nil, errors.Join(ErrIdentity, err)
	}
	header := unsafe.Sizeof(windows.Tokenuser{})
	if uintptr(returned) < header || returned > tokenUserBytes {
		return nil, ErrIdentity
	}
	user := &storage.user
	if !sidWithin(user.User.Sid, uintptr(base)+header, uintptr(returned)-header) {
		return nil, ErrIdentity
	}
	// sidWithin already bounded the revision/count and all subauthority bytes.
	prefix := unsafe.Slice((*byte)(unsafe.Pointer(user.User.Sid)), 8)
	size := 8 + 4*int(prefix[1])
	sid, target := newSIDStorage()
	copy(target, unsafe.Slice((*byte)(unsafe.Pointer(user.User.Sid)), size))
	if !sid.IsValid() {
		return nil, ErrIdentity
	}
	return sid, nil
}

func newSIDStorage() (*windows.SID, []byte) {
	// SID's header is8 bytes followed by at most15 little-endian DWORDs. DWORD
	// storage keeps the Go allocation aligned without a native allocator/free.
	storage := new([maxSIDBytes / 4]uint32)
	base := unsafe.Pointer(storage)
	return (*windows.SID)(base), unsafe.Slice((*byte)(base), maxSIDBytes)
}

func fixedInstallerSID() (*windows.SID, error) {
	// Exact existing S-1-5-80 service SID, as named by TrustedInstallerSID.
	// This private fixed encoding is not a general string/input SID parser.
	subAuthorities := [...]uint32{80, 956008885, 3418522649, 1831038044, 1853292631, 2271478464}
	sid, target := newSIDStorage()
	target[0], target[1], target[7] = 1, byte(len(subAuthorities)), 5
	for index, value := range subAuthorities {
		binary.LittleEndian.PutUint32(target[8+index*4:], value)
	}
	if !sid.IsValid() {
		return nil, ErrIdentity
	}
	return sid, nil
}

// CurrentTrust keeps the existing user/Admins/SYSTEM/TrustedInstaller policy
// without ConvertStringSidToSid, native SID allocation or hidden LocalFree.
func CurrentTrust() (Trust, error) {
	user, err := CurrentUser()
	if err != nil {
		return Trust{}, err
	}
	installer, err := fixedInstallerSID()
	if err != nil {
		return Trust{}, err
	}
	return Trust{user: user, installer: installer}, nil
}

func (trust Trust) Trusted(sid *windows.SID) bool {
	return trust.user != nil && trust.installer != nil && sid != nil && sid.IsValid() &&
		(sid.Equals(trust.user) || sid.Equals(trust.installer) ||
			sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid))
}

// MatchesUser is exact primary-user equality, not membership in Trusted's wider
// Admins/SYSTEM/installer set. UAC must not substitute another trusted principal.
func (trust Trust) MatchesUser(sid *windows.SID) bool {
	return trust.user != nil && trust.installer != nil && sid != nil && sid.IsValid() && sid.Equals(trust.user)
}
