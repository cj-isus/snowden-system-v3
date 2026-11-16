//go:build windows

package filetxn

import "os"

func (slot *BoundedReadSlot) open(path string, limit int64, directory bool) error {
	original := NewWindowsRead(path, directory, limit)
	original.selectedInput = slot.selectedInput // fixed before acquisition, not a fallback
	slot.file = original                        // before native acquisition, including partial failure
	return original.Open()
}

func sameBoundedReadFile(a, b os.FileInfo) bool { return SameWindowsFile(a, b) }
