//go:build windows

package delivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"golang.org/x/sys/windows"
)

type nativeWorkspace struct {
	ancestors               []*workspaceFile
	work, metadata, targets *workspaceFile
	active, probe           *workspaceFile
	trust                   fileacl.Trust
	badErr                  error
	started                 bool
}

func newWorkspacePlatform() workspacePlatform { return &nativeWorkspace{} }

func (owner *nativeWorkspace) Paths() (string, string) {
	return owner.metadata.path, owner.targets.path
}

func (owner *nativeWorkspace) trusted(sid *windows.SID) bool {
	return owner.trust.Trusted(sid)
}

func (owner *nativeWorkspace) Prepare(ctx context.Context, cache map[string][]byte) error {
	if owner.started || len(cache) > 5 || contextError(ctx) != nil {
		return ErrTrust
	}
	owner.started = true
	var err error
	owner.trust, err = fileacl.CurrentTrust()
	if err != nil {
		return ErrTrust
	}
	parent, err := filepath.Abs(os.TempDir())
	volume := filepath.VolumeName(parent)
	if err != nil || len(parent) > 2048 || !filepath.IsAbs(parent) || len(volume) != 2 || volume[1] != ':' ||
		!(volume[0] >= 'a' && volume[0] <= 'z' || volume[0] >= 'A' && volume[0] <= 'Z') {
		return ErrTrust // local drive custody; UNC/device/redirected profiles need their own gate
	}
	root := volume + `\`
	parts := strings.Split(strings.TrimPrefix(parent, root), `\`)
	if parent == root {
		parts = nil
	}
	if len(parts) >= 64 {
		return ErrBudget
	}
	for _, part := range parts {
		if !workspaceComponent(part) {
			return ErrTrust
		}
	}
	file := &workspaceFile{path: root, directory: true}
	owner.ancestors = append(owner.ancestors, file) // receipt before the native output
	if err := owner.open(file, 0, `\??\`+root, false, false, false); err != nil {
		return err
	}
	for _, part := range parts {
		if err := contextError(ctx); err != nil {
			return err
		}
		file = &workspaceFile{path: filepath.Join(owner.parent().path, part), directory: true}
		parentHandle := owner.parent().handle
		owner.ancestors = append(owner.ancestors, file)
		if err := owner.open(file, parentHandle, part, false, false, false); err != nil {
			return err
		}
	}
	var filesystem [16]uint16
	if windows.GetVolumeInformationByHandle(owner.parent().handle, nil, 0, nil, nil, nil, &filesystem[0], uint32(len(filesystem))) != nil ||
		!strings.EqualFold(windows.UTF16ToString(filesystem[:]), "NTFS") {
		return ErrTrust
	}
	if err := owner.createWork(ctx); err != nil {
		return err
	}
	owner.metadata = &workspaceFile{path: filepath.Join(owner.work.path, "metadata"), directory: true}
	if err := owner.open(owner.metadata, owner.work.handle, "metadata", true, false, true); err != nil {
		return err
	}
	owner.targets = &workspaceFile{path: filepath.Join(owner.work.path, "targets"), directory: true}
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := owner.open(owner.targets, owner.work.handle, "targets", true, false, true); err != nil {
		return err
	}
	for name, raw := range cache {
		if !cacheName(name) || len(raw) == 0 || len(raw) > MaxMetadataBytes {
			return ErrTrust
		}
		if err := owner.seed(ctx, name, raw); err != nil {
			return err
		}
	}
	return nil
}

func workspaceComponent(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 &&
		!strings.ContainsAny(name, "\\/:*?\"<>|\x00") && !strings.HasSuffix(name, ".") && !strings.HasSuffix(name, " ")
}

func (owner *nativeWorkspace) parent() *workspaceFile { return owner.ancestors[len(owner.ancestors)-1] }

func (owner *nativeWorkspace) createWork(ctx context.Context) error {
	for attempt := 0; attempt < 16; attempt++ {
		if err := contextError(ctx); err != nil {
			return err
		}
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		name := "snowden-tuf-" + hex.EncodeToString(nonce[:])
		owner.work = &workspaceFile{path: filepath.Join(owner.parent().path, name), directory: true}
		err := owner.open(owner.work, owner.parent().handle, name, true, false, true)
		if errors.Is(err, os.ErrExist) && owner.work.handle == 0 && !owner.work.created {
			owner.work = nil // only a confirmed no-create collision
			continue
		}
		return err
	}
	return ErrBudget
}

func (owner *nativeWorkspace) seed(ctx context.Context, name string, raw []byte) error {
	if owner.active != nil || contextError(ctx) != nil || owner.validate(owner.metadata) != nil {
		return ErrTrust
	}
	owner.active = &workspaceFile{path: filepath.Join(owner.metadata.path, name)}
	file := owner.active
	if err := owner.open(file, owner.metadata.handle, name, true, true, true); err != nil {
		return err
	}
	for offset := 0; offset < len(raw); {
		if err := contextError(ctx); err != nil {
			return err
		}
		end := offset + 32*1024
		if end > len(raw) {
			end = len(raw)
		}
		var done uint32
		if err := windows.WriteFile(file.handle, raw[offset:end], &done, nil); err != nil || done != uint32(end-offset) {
			return errors.Join(ErrTrust, io.ErrShortWrite, err)
		}
		offset = end
	}
	if windows.FlushFileBuffers(file.handle) != nil || owner.validate(file) != nil {
		return ErrTrust
	}
	var info windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(file.handle, &info) != nil || info.FileSizeHigh != 0 || info.FileSizeLow != uint32(len(raw)) {
		return ErrTrust
	}
	if err := file.Close(); err != nil {
		return err
	}
	owner.active = nil // SDK may replace this name only after original WRITE retirement
	return nil
}
