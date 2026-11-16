// Package imagecheck reuses bounded image reads and fixed PE-header checks. It
// grants no signature, image/path/ACL, loaded-section or process-tree authority.
package imagecheck

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"io"
)

var ErrImage = errors.New("child bootstrap image unavailable")

// MaxBytes is the existing Windows application payload ceiling, not a memory cap.
const MaxBytes int64 = 1 << 30

// PE reads fixed DOS/PE headers only, never file-declared symbol/string tables.
// application permits the GUI subsystem in addition to the helper's console one.
// The native loader and the caller's independent authority checks still apply.
func PE(reader io.ReaderAt, size int64, application bool) error {
	if reader == nil || size < 64 || size > MaxBytes {
		return ErrImage
	}
	var dos [64]byte
	if _, err := reader.ReadAt(dos[:], 0); err != nil || dos[0] != 'M' || dos[1] != 'Z' {
		return ErrImage
	}
	offset := int64(binary.LittleEndian.Uint32(dos[60:]))
	var raw [264]byte // signature4 + FileHeader20 + OptionalHeader64 240
	if offset < int64(len(dos)) || offset > size-int64(len(raw)) {
		return ErrImage
	}
	if _, err := reader.ReadAt(raw[:], offset); err != nil || !bytes.Equal(raw[:4], []byte{'P', 'E', 0, 0}) {
		return ErrImage
	}
	var header pe.FileHeader
	var optional pe.OptionalHeader64
	if binary.Read(bytes.NewReader(raw[4:24]), binary.LittleEndian, &header) != nil || binary.Read(bytes.NewReader(raw[24:]), binary.LittleEndian, &optional) != nil {
		return ErrImage
	}
	end := offset + int64(len(raw)) + 40*int64(header.NumberOfSections)
	subsystemOK := optional.Subsystem == pe.IMAGE_SUBSYSTEM_WINDOWS_CUI || application && optional.Subsystem == pe.IMAGE_SUBSYSTEM_WINDOWS_GUI
	if header.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || header.SizeOfOptionalHeader != 240 || header.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 || header.Characteristics&pe.IMAGE_FILE_DLL != 0 || optional.Magic != 0x20b || !subsystemOK || header.NumberOfSections < 1 || header.NumberOfSections > 96 || end > size || int64(optional.SizeOfHeaders) < end || int64(optional.SizeOfHeaders) > size || optional.AddressOfEntryPoint == 0 || optional.AddressOfEntryPoint >= optional.SizeOfImage {
		return ErrImage
	}
	return nil
}
