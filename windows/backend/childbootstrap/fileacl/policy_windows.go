//go:build windows

package fileacl

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

const maxDescriptorBytes = 128 << 10 // accepted returned descriptor size, not allocation/RSS proof

func within(pointer unsafe.Pointer, bytes, start, size uintptr) bool {
	address := uintptr(pointer)
	return pointer != nil && address >= start && address-start <= size && bytes <= size-(address-start)
}

func sidWithin(sid *windows.SID, start, size uintptr) bool {
	if !within(unsafe.Pointer(sid), 8, start, size) {
		return false
	}
	header := unsafe.Slice((*byte)(unsafe.Pointer(sid)), 8)
	return header[0] == 1 && header[1] <= 15 &&
		within(unsafe.Pointer(sid), 8+4*uintptr(header[1]), start, size) && sid.IsValid()
}

func validateDescriptor(descriptor *windows.SECURITY_DESCRIPTOR, directory bool, trusted func(*windows.SID) bool) error {
	if descriptor == nil || !descriptor.IsValid() {
		return ErrACL
	}
	control, _, err := descriptor.Control()
	size := uintptr(descriptor.Length())
	if err != nil || control&windows.SE_SELF_RELATIVE == 0 || size < 20 || size > maxDescriptorBytes {
		return ErrACL
	}
	start := uintptr(unsafe.Pointer(descriptor))
	owner, _, err := descriptor.Owner()
	if err != nil || !sidWithin(owner, start, size) || !trusted(owner) {
		return ErrACL
	}
	acl, _, err := descriptor.DACL()
	if err != nil || !within(unsafe.Pointer(acl), 8, start, size) || acl.AceCount > 128 {
		return ErrACL
	}
	// Pinned xsys ACL keeps AclSize private. Its Windows header is revision,
	// reserved, uint16 size, uint16 count, reserved; inspect only these8 bytes.
	header := unsafe.Slice((*byte)(unsafe.Pointer(acl)), 8)
	aclSize := uintptr(binary.LittleEndian.Uint16(header[2:4]))
	aclStart := uintptr(unsafe.Pointer(acl))
	if aclSize < 8 || !within(unsafe.Pointer(acl), aclSize, start, size) {
		return ErrACL
	}
	const fileDeleteChild windows.ACCESS_MASK = 0x40
	mutate := windows.ACCESS_MASK(windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL |
		windows.FILE_WRITE_DATA | windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES)
	mutate |= fileDeleteChild
	if !directory {
		mutate |= windows.FILE_APPEND_DATA
	}
	for index := uint32(0); index < uint32(acl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, index, &ace) != nil || !within(unsafe.Pointer(ace), 4, aclStart, aclSize) {
			return ErrACL
		}
		if ace.Header.AceSize < 16 || !within(unsafe.Pointer(ace), uintptr(ace.Header.AceSize), aclStart, aclSize) {
			return ErrACL
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			continue
		case windows.ACCESS_ALLOWED_ACE_TYPE:
		default:
			return ErrACL
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sidWithin(sid, uintptr(unsafe.Pointer(ace)), uintptr(ace.Header.AceSize)) ||
			ace.Mask&mutate != 0 && !trusted(sid) {
			return ErrACL
		}
	}
	return nil
}
