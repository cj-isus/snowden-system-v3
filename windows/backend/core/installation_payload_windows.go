//go:build windows

package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/netprobe"
	"github.com/snowden-system/windows/backend/update"
	"golang.org/x/sys/windows"
)

type installationPayloadFile struct {
	owner   *windowsInstallationRoot
	file    nativeImageFile
	payload update.Payload
	ready   bool
	closed  bool
}

func installationPayloadPath(root, path string) bool {
	if len(path) > 2048 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	name := filepath.Base(path)
	if name != update.ApplicationFile && name != netprobe.NativeHelperName && name != update.RecoveryApplicationFile {
		return false
	}
	if strings.EqualFold(filepath.Dir(path), root) {
		return true
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(relative) {
		return false
	}
	parts := strings.Split(strings.ToLower(relative), `\`)
	return len(parts) == 4 && parts[0] == ".snowden-update" && parts[1] == "data" && installationDataDirectory(parts[2]) ||
		len(parts) == 5 && strings.HasPrefix(parts[0], ".snowden-update-accepted-") && parts[1] == "transaction" &&
			parts[2] == "data" && installationDataDirectory(parts[3])
}

func installationAllowedPayload(path string, allowed []update.Payload) bool {
	if len(allowed) < 1 || len(allowed) > 2 {
		return false
	}
	for _, payload := range allowed {
		limit := int64(update.MaxPayloadSize)
		if payload.File == netprobe.NativeHelperName {
			limit = netprobe.MaxNativeImageBytes
		}
		if payload.File != filepath.Base(path) || payload.Size < 1 || payload.Size > limit {
			return false
		}
		if _, err := nativeImagePin(payload.SHA256); err != nil {
			return false
		}
	}
	return true
}

func installationPayloadMove(root, from, to string) bool {
	if strings.EqualFold(filepath.Dir(from), root) {
		parent := filepath.Base(filepath.Dir(to))
		return parent == "displaced" || parent == "rejected"
	}
	if !strings.EqualFold(filepath.Dir(to), root) {
		return false
	}
	switch filepath.Base(filepath.Dir(from)) {
	case "new", "displaced", "restore":
		return true
	}
	return false // old backups and rejected images cannot move back as new payloads
}

func (owner *windowsInstallationRoot) PinPayload(ctx context.Context, path string, allowed []update.Payload) (result update.PayloadFile, finalErr error) {
	if !installationPayloadPath(owner.root, path) || !installationAllowedPayload(path, allowed) || owner.Validate(ctx, owner.root) != nil ||
		owner.directoryParent(filepath.Dir(path)) == 0 {
		return nil, ErrInstallationRootUnavailable
	}
	for _, file := range owner.payloads {
		if strings.EqualFold(file.file.path, path) {
			for _, payload := range allowed {
				if file.payload == payload && file.Validate(ctx) == nil {
					return file, nil
				}
			}
			return file, ErrInstallationRootUnavailable // original failed/retired object is never replaced
		}
	}
	if len(owner.payloads) >= update.MaxOwnedPairPayloads {
		return nil, ErrInstallationRootUnavailable
	}
	file := &installationPayloadFile{owner: owner, file: nativeImageFile{path: path}}
	owner.payloads = append(owner.payloads, file) // before the first acquisition
	result = file
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return result, ErrInstallationRootUnavailable
	}
	if err := file.file.open(name, windows.GENERIC_READ|windows.DELETE|windows.READ_CONTROL|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_SEQUENTIAL_SCAN); err != nil || file.file.handle() == 0 {
		return result, errors.Join(ErrInstallationRootUnavailable, err)
	}
	if err := owner.image.readFileIdentity(&file.file, &file.file.identity); err != nil {
		return result, err
	}
	size := int64(file.file.identity.FileSizeHigh)<<32 | int64(file.file.identity.FileSizeLow)
	sizeAllowed := false
	for _, payload := range allowed {
		sizeAllowed = sizeAllowed || payload.Size == size
	}
	if !sizeAllowed {
		return result, ErrInstallationRootUnavailable
	}
	digest, err := file.digest(ctx, size, nil)
	if err != nil {
		return result, err
	}
	for _, payload := range allowed {
		if payload.Size == size && payload.SHA256 == digest {
			file.payload, file.ready = payload, true
			return result, file.Validate(ctx)
		}
	}
	return result, ErrInstallationRootUnavailable // unknown bytes remain owned, never renamed or deleted
}

func (file *installationPayloadFile) Path() string            { return file.file.path }
func (file *installationPayloadFile) Payload() update.Payload { return file.payload }

func (file *installationPayloadFile) validate(ctx context.Context) error {
	if !file.ready || file.closed || contextError(ctx) != nil {
		return ErrInstallationRootUnavailable
	}
	var current windows.ByHandleFileInformation
	if file.owner.image.readFileIdentity(&file.file, &current) != nil || !sameInstallationID(current, file.file.identity) ||
		current.FileSizeHigh != file.file.identity.FileSizeHigh || current.FileSizeLow != file.file.identity.FileSizeLow ||
		current.LastWriteTime != file.file.identity.LastWriteTime {
		return ErrInstallationRootUnavailable
	}
	return nil
}

func (file *installationPayloadFile) Validate(ctx context.Context) error {
	if err := file.owner.Validate(ctx, file.owner.root); err != nil {
		return err
	}
	return file.validate(ctx)
}

func (file *installationPayloadFile) digest(ctx context.Context, size int64, dst io.Writer) (string, error) {
	reader := &nativeImageReader{ctx: ctx, handle: file.file.handle(), size: size, limit: update.MaxPayloadSize}
	hash := sha256.New()
	writer := io.Writer(hash)
	if dst != nil {
		writer = io.MultiWriter(dst, hash)
	}
	n, err := io.CopyBuffer(writer, io.NewSectionReader(reader, 0, size), make([]byte, 32<<10))
	if err != nil || n != size {
		return "", errors.Join(ErrInstallationRootUnavailable, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (file *installationPayloadFile) CopyTo(ctx context.Context, dst io.Writer) error {
	if dst == nil || file.Validate(ctx) != nil {
		return ErrInstallationRootUnavailable
	}
	digest, err := file.digest(ctx, file.payload.Size, dst)
	if err != nil || digest != file.payload.SHA256 {
		return errors.Join(ErrInstallationRootUnavailable, err)
	}
	return file.Validate(ctx)
}

func (file *installationPayloadFile) Move(ctx context.Context, to string) (moved bool, finalErr error) {
	owner := file.owner
	from := file.file.path
	parent := owner.directoryParent(filepath.Dir(to))
	if file.Validate(ctx) != nil || parent == 0 || strings.EqualFold(from, to) ||
		!installationPayloadPath(owner.root, to) || filepath.Base(to) != file.payload.File || !installationPayloadMove(owner.root, from, to) {
		return false, ErrInstallationRootUnavailable
	}
	for _, other := range owner.payloads {
		if other != file && !other.closed && strings.EqualFold(other.file.path, to) {
			return false, ErrInstallationRootUnavailable
		}
	}
	name, err := windows.UTF16FromString(filepath.Base(to))
	if err != nil || len(name) > 256 {
		return false, ErrInstallationRootUnavailable
	}
	info := installationRenameInformation{RootDirectory: parent, FileNameLength: uint32(2 * (len(name) - 1))}
	copy(info.FileName[:], name)
	owner.moving = true
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
		}
		actual, err := nativeFilePath(file.file.handle())
		if err != nil || !strings.EqualFold(actual, from) && !strings.EqualFold(actual, to) {
			finalErr = errors.Join(finalErr, ErrInstallationRootUnavailable)
			return // retain moving=true and original handle on an unknown native outcome
		}
		moved = strings.EqualFold(actual, to)
		if moved {
			file.file.path = to
		} else if finalErr == nil {
			finalErr = ErrInstallationRootUnavailable
		}
		// Reconcile only this original object, never reopen a replacement path.
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := file.validate(cleanup); err != nil {
			finalErr = errors.Join(finalErr, err)
			return
		}
		owner.moving = false
		if ctx.Err() != nil {
			finalErr = errors.Join(finalErr, ctx.Err())
		}
	}()
	if ctx.Err() != nil {
		completed = true
		return false, ctx.Err()
	}
	finalErr = windows.NtSetInformationFile(file.file.handle(), &windows.IO_STATUS_BLOCK{},
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), windows.FileRenameInformation)
	completed = true
	return false, finalErr // actual original-handle path decides moved, even on error
}

func (file *installationPayloadFile) Close() error {
	if file.closed {
		return nil
	}
	if err := file.file.close(); err != nil {
		return err
	}
	file.closed, file.ready = true, false
	return nil
}

func (owner *windowsInstallationRoot) ReleasePayloads() error {
	owner.payloadRetiring = true
	for index := len(owner.payloads) - 1; index >= 0; index-- {
		if err := owner.payloads[index].Close(); err != nil {
			return err // original root/lock and remaining file handles stay owned
		}
	}
	owner.payloads = nil
	owner.payloadRetiring = false
	return nil
}
