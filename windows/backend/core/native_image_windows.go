//go:build windows

package core

import (
	"context"
	"io"
	"path/filepath"

	"github.com/snowden-system/windows/backend/childbootstrap/filelease"
	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"golang.org/x/sys/windows"
)

type nativeImageFile struct {
	lease     *filelease.Lease
	path      string
	directory bool
	identity  windows.ByHandleFileInformation
}

// Original per-file owner is published before either native acquisition API.
// Copies of the installer/image record retain that same pointer, never a handle
// ownership copy. Caller serialization and explicit private-ID rebind remain.
func (file *nativeImageFile) open(name *uint16, access, share, flags uint32) error {
	if file.lease == nil {
		file.lease = new(filelease.Lease)
	}
	return nativeImageInspectionError(file.lease.Open(name, access, share, flags))
}

func (file *nativeImageFile) openRelative(parent windows.Handle, name *windows.NTUnicodeString, access, attributes, share, disposition, options uint32) (uintptr, error) {
	if file.lease == nil {
		file.lease = new(filelease.Lease)
	}
	information, err := file.lease.OpenRelative(parent, name, access, attributes, share, disposition, options)
	return information, nativeImageInspectionError(err)
}

func (file *nativeImageFile) handle() windows.Handle {
	return file.lease.HandleValue() // borrowed raw fact, never independent use authority
}

func (file *nativeImageFile) close() error {
	if file == nil {
		return nil
	}
	if err := file.lease.Close(); err != nil {
		return nativeImageInspectionError(err)
	}
	return nil
}

type windowsNativeImage struct {
	path  string
	files []nativeImageFile // mutable installer/verifier/recovery records, close in reverse
	trust nativeImageTrust
}

// Sharing and ACL checks cover cooperative update/path races and untrusted
// principals. They do not isolate hostile code running as the trusted owner,
// an administrator/kernel actor, or a preexisting writable mapped view.

func nativeImageComponent(value string) bool {
	return imagecheck.Component(value)
}

// Shared native directory custody for image reads and installation writes.
// A deny-delete handle pins each ancestor's identity, not arbitrary children.
func (image *windowsNativeImage) openDirectories(ctx context.Context, directory string) error {
	if contextError(ctx) != nil || len(image.files) != 0 || len(directory) > 2048 || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return ErrNativeImageUnavailable
	}
	paths, err := imagecheck.DirectoryPaths(directory)
	if err != nil {
		return ErrNativeImageUnavailable
	}
	image.trust, err = currentNativeImageTrust()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := image.openFile(ctx, path, true); err != nil {
			return err
		}
	}
	if imagecheck.NTFS(image.files[0].handle()) != nil {
		return ErrNativeImageUnavailable
	}
	return image.validateFiles(ctx)
}

func (image *windowsNativeImage) openFile(ctx context.Context, path string, directory bool) error {
	if contextError(ctx) != nil {
		return contextError(ctx)
	}
	for index := range image.files {
		file := &image.files[index]
		if !file.lease.Ready() {
			return ErrNativeImageUnavailable
		}
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrNativeImageUnavailable
	}
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT)
	access, share := uint32(windows.GENERIC_READ|windows.READ_CONTROL), uint32(windows.FILE_SHARE_READ)
	if directory {
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
		// Metadata/traversal custody pins the directory itself. Permit child
		// entry writes; relative rename target opens need these sharing rights.
		access = windows.FILE_TRAVERSE | windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE
		share |= windows.FILE_SHARE_WRITE
	}
	image.files = append(image.files, nativeImageFile{path: path, directory: directory})
	file := &image.files[len(image.files)-1] // reserve original output before CreateFile
	if err := file.open(name, access, share, flags); err != nil {
		return err
	}
	return image.readFileIdentity(file, &file.identity)
}

// Fixed header projection using standard PE types. A full PE decoder can
// allocate file-declared symbol/string tables; this capability check needs no
// such tables. The OS loader still validates the image before process creation.
func nativeImagePE(reader io.ReaderAt, size int64) error {
	return nativeImagePEKind(reader, size, false)
}

func nativeImagePEKind(reader io.ReaderAt, size int64, application bool) error {
	if imagecheck.PE(reader, size, application) != nil {
		return ErrNativeImageUnavailable
	}
	return nil
}

func (image *windowsNativeImage) readFileIdentity(file *nativeImageFile, identity *windows.ByHandleFileInformation) error {
	if file == nil || identity == nil || !file.lease.Ready() {
		return ErrNativeImageUnavailable
	}
	return nativeImageInspectionError(file.lease.Inspect(file.path, file.directory, identity, image.trust.trusted))
}

func nativeFilePath(handle windows.Handle) (string, error) {
	path, err := imagecheck.FilePath(handle)
	if err != nil {
		return "", ErrNativeImageUnavailable
	}
	return path, nil
}

func (image *windowsNativeImage) validateFiles(ctx context.Context) error {
	if len(image.files) == 0 {
		return ErrNativeImageUnavailable
	}
	for index := range image.files {
		if contextError(ctx) != nil {
			return contextError(ctx)
		}
		file := &image.files[index]
		var current windows.ByHandleFileInformation
		if err := image.readFileIdentity(file, &current); err != nil {
			return err // retain typed ACL/resource-custody failures for the original owner
		}
		if !imagecheck.SameSnapshot(file.identity, current, file.directory) {
			return ErrNativeImageUnavailable
		}
	}
	return contextError(ctx)
}

func nativeProcessPathMatches(ctx context.Context, process windows.Handle, expected string) error {
	if err := imagecheck.ProcessPathMatches(ctx, process, expected, contextError); err != nil {
		if err != imagecheck.ErrImage {
			return err // preserve the original source/deadline/cancellation cause
		}
		return ErrNativeImageUnavailable
	}
	return nil
}

func (image *windowsNativeImage) Close() error {
	for index := len(image.files) - 1; index >= 0; index-- {
		file := &image.files[index]
		if err := file.close(); err != nil {
			return err // remaining original handles and acquisition witnesses retained
		}
	}
	return nil
}
