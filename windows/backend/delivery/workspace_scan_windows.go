//go:build windows

package delivery

import (
	"encoding/binary"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type workspaceEntry struct {
	name      string
	identity  uint64
	directory bool
}

// FILE_ID_BOTH_DIR_INFO uses a 104-byte fixed prefix and 8-byte alignment.
// No new search handle or unbounded directory allocation is needed.
func (owner *nativeWorkspace) entries(dir *workspaceFile) ([]workspaceEntry, error) {
	if err := owner.validate(dir); err != nil {
		return nil, err
	}
	var aligned [4096]uint64
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&aligned[0])), len(aligned)*8)
	entries := make([]workspaceEntry, 0, 8)
	seen := make(map[string]bool)
	class := uint32(windows.FileIdBothDirectoryRestartInfo)
	records := 0
	for page := 0; page < maxWorkspaceEntries+2; page++ {
		clear(raw)
		err := windows.GetFileInformationByHandleEx(dir.handle, class, &raw[0], uint32(len(raw)))
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return entries, owner.validate(dir)
		}
		if err != nil {
			return nil, ErrTrust
		}
		class = windows.FileIdBothDirectoryInfo
		for offset := 0; ; {
			if offset < 0 || offset > len(raw)-104 || records >= maxWorkspaceEntries+2 {
				return nil, ErrBudget
			}
			records++
			header := raw[offset:]
			next := int(binary.LittleEndian.Uint32(header[0:4]))
			nameBytes := int(binary.LittleEndian.Uint32(header[60:64]))
			if nameBytes == 0 || nameBytes > 510 || nameBytes%2 != 0 || nameBytes > len(header)-104 ||
				next != 0 && (next%8 != 0 || next < 104+nameBytes || next > len(header)-104) {
				return nil, ErrTrust
			}
			nameUTF16 := make([]uint16, nameBytes/2)
			for index := range nameUTF16 {
				nameUTF16[index] = binary.LittleEndian.Uint16(header[104+2*index : 106+2*index])
				if nameUTF16[index] == 0 {
					return nil, ErrTrust
				}
			}
			name := windows.UTF16ToString(nameUTF16)
			if seen[name] {
				return nil, ErrTrust
			}
			seen[name] = true
			if name != "." && name != ".." {
				attributes := binary.LittleEndian.Uint32(header[56:60])
				id := binary.LittleEndian.Uint64(header[96:104])
				if !workspaceComponent(name) || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || id == 0 || len(entries) >= maxWorkspaceEntries {
					return nil, ErrTrust
				}
				entries = append(entries, workspaceEntry{name: name, identity: id, directory: attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0})
			}
			if next == 0 {
				break
			}
			offset += next
		}
	}
	return nil, ErrBudget
}
