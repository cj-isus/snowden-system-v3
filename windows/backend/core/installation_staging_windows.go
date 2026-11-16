//go:build windows

package core

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/update"
	"golang.org/x/sys/windows"
)

var (
	errInstallationStagingClose = errors.New("installation staging close unconfirmed")
	errInstallationStagingWrite = errors.New("installation staging write unconfirmed")
	errInstallationStagingFlush = errors.New("installation staging flush unconfirmed")
)

type installationStagingFile struct {
	owner     *windowsInstallationRoot
	ctx       context.Context
	file      nativeImageFile // original handle, never transferred to os.File
	limit     int64
	written   int64
	confirmed bool
	ready     bool
	closed    bool
	writeErr  error
	closeErr  error
}

func installationStagingRelative(root, path string) (string, error) {
	if len(path) > 2048 || !filepath.IsAbs(path) || filepath.Clean(path) != path || !installationDirectoryPath(root, filepath.Dir(path)) {
		return "", ErrInstallationRootUnavailable
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(rel) {
		return "", ErrInstallationRootUnavailable
	}
	parts := strings.Split(rel, `\`)
	var start int
	switch {
	case strings.HasPrefix(parts[0], ".snowden-update-stage-"):
		start = 1
	case parts[0] == ".snowden-update" && len(parts) >= 3 && parts[1] == "data":
		start = 2
	case strings.HasPrefix(parts[0], ".snowden-update-accepted-") && len(parts) >= 4 && parts[1] == "transaction" && parts[2] == "data":
		start = 3
	default:
		return "", ErrInstallationRootUnavailable
	}
	if len(parts)-start < 1 || len(parts)-start > 2 {
		return "", ErrInstallationRootUnavailable
	}
	return filepath.Join(parts[start:]...), nil
}

func (owner *windowsInstallationRoot) CreateStagingFile(ctx context.Context, path string) (result update.StagingFile, finalErr error) {
	rel, err := installationStagingRelative(owner.root, path)
	if err != nil || owner.Validate(ctx, owner.root) != nil || owner.directoryParent(filepath.Dir(path)) == 0 {
		return nil, ErrInstallationRootUnavailable
	}
	limit, err := update.StagingFileLimit(rel)
	if err != nil || len(owner.staging) >= update.MaxOwnedPairStagingFiles {
		return nil, ErrInstallationRootUnavailable
	}
	file := &installationStagingFile{owner: owner, ctx: ctx, file: nativeImageFile{path: path}, limit: limit}
	owner.staging = append(owner.staging, file) // original owner before first native acquisition
	result = file
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
		}
	}()
	created, _, err := createInstallationObject(owner.directoryParent(filepath.Dir(path)), &file.file,
		windows.GENERIC_WRITE|windows.FILE_READ_ATTRIBUTES|windows.READ_CONTROL|windows.SYNCHRONIZE, windows.FILE_SHARE_READ)
	file.confirmed = created && err == nil
	if err != nil {
		completed = true
		return file, err
	}
	if err := owner.image.readFileIdentity(&file.file, &file.file.identity); err != nil {
		completed = true
		return file, err
	}
	if file.file.identity.FileSizeHigh != 0 || file.file.identity.FileSizeLow != 0 {
		completed = true
		return file, ErrInstallationRootUnavailable
	}
	file.ready = true
	finalErr = owner.Validate(ctx, owner.root)
	completed = true
	return file, finalErr
}

func (file *installationStagingFile) Identity() (update.CreatedFileID, error) {
	id := update.CreatedFileID{VolumeSerialNumber: file.file.identity.VolumeSerialNumber,
		IndexHigh: file.file.identity.FileIndexHigh, IndexLow: file.file.identity.FileIndexLow}
	if !file.confirmed || !id.Valid() {
		return update.CreatedFileID{}, ErrInstallationRootUnavailable
	}
	return id, nil // cached original creation fact, including a later validation/cancellation failure
}

func (file *installationStagingFile) validate(ctx context.Context) error {
	if file.closeErr != nil {
		return file.closeErr
	}
	if file.closed {
		return nil
	}
	if !file.ready || file.file.handle() == 0 || contextError(ctx) != nil {
		return ErrInstallationRootUnavailable
	}
	var info windows.ByHandleFileInformation
	if file.owner.image.readFileIdentity(&file.file, &info) != nil || !sameInstallationID(info, file.file.identity) {
		return ErrInstallationRootUnavailable
	}
	size := int64(info.FileSizeHigh)<<32 | int64(info.FileSizeLow)
	if size != file.written || size > file.limit {
		return ErrInstallationRootUnavailable
	}
	return nil
}

func (file *installationStagingFile) Write(data []byte) (total int, finalErr error) {
	if file.closed || !file.ready || file.closeErr != nil || file.writeErr != nil {
		return 0, ErrInstallationRootUnavailable
	}
	if err := contextError(file.ctx); err != nil {
		file.writeErr = err
		return 0, err
	}
	if int64(len(data)) > file.limit-file.written {
		file.writeErr = ErrInstallationRootUnavailable // a caller cannot swallow this and obtain a successful Sync
		return 0, file.writeErr
	}
	for len(data) != 0 {
		if contextError(file.ctx) != nil {
			file.writeErr = contextError(file.ctx)
			return total, file.writeErr
		}
		part := data
		if len(part) > 32<<10 {
			part = part[:32<<10]
		}
		var written uint32
		file.writeErr = errInstallationStagingWrite // retain uncertainty if native output processing panics
		err := windows.WriteFile(file.file.handle(), part, &written, nil)
		if written > uint32(len(part)) {
			file.writeErr = ErrInstallationRootUnavailable
			return total, file.writeErr
		}
		file.written += int64(written)
		total += int(written)
		if err != nil || written != uint32(len(part)) {
			file.writeErr = errors.Join(io.ErrShortWrite, err)
			return total, file.writeErr
		}
		file.writeErr = nil
		data = data[len(part):]
	}
	return total, nil
}

func (file *installationStagingFile) Sync() error {
	if file.closed || !file.ready || file.writeErr != nil || file.closeErr != nil || contextError(file.ctx) != nil {
		return ErrInstallationRootUnavailable
	}
	if err := file.owner.Validate(file.ctx, file.owner.root); err != nil {
		file.writeErr = err
		return err
	}
	file.writeErr = errInstallationStagingFlush
	if err := windows.FlushFileBuffers(file.file.handle()); err != nil {
		file.writeErr = errors.Join(file.writeErr, err)
		return file.writeErr
	}
	file.writeErr = contextError(file.ctx) // known flush does not erase canceled logical completion
	return file.writeErr
}

func (file *installationStagingFile) Close() error {
	if file.closeErr != nil {
		return file.closeErr
	}
	if file.closed {
		return nil
	}
	file.closeErr = errInstallationStagingClose // sticky on error/panic, including unreturned acquisition
	if err := file.file.close(); err != nil {
		return errors.Join(file.closeErr, err)
	}
	file.closeErr = nil
	file.closed, file.ready = true, false
	return nil
}

func (owner *windowsInstallationRoot) hasLiveStaging() bool {
	for _, file := range owner.staging {
		if !file.closed || file.closeErr != nil || file.file.handle() != 0 {
			return true
		}
	}
	return false
}

func (owner *windowsInstallationRoot) ReleaseStagingFiles() error {
	owner.stagingRetiring = true
	for index := len(owner.staging) - 1; index >= 0; index-- {
		if err := owner.staging[index].Close(); err != nil {
			return err
		}
	}
	owner.staging = nil
	owner.stagingRetiring = false
	return nil
}
