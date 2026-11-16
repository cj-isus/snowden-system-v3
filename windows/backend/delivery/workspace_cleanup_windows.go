//go:build windows

package delivery

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func (owner *nativeWorkspace) Close() error {
	if owner.badErr != nil {
		return owner.badErr // retain all original outputs after unknown acquisition
	}
	if owner.active != nil {
		if err := owner.active.Close(); err != nil {
			return err
		}
		owner.active = nil
	}
	if owner.work != nil && owner.work.created {
		for _, ancestor := range owner.ancestors {
			if err := owner.validate(ancestor); err != nil {
				return err
			}
		}
		rootEntries, err := owner.entries(owner.work)
		if err != nil {
			return err
		}
		for _, entry := range rootEntries {
			if !entry.directory || entry.name != "metadata" && entry.name != "targets" {
				return ErrTrust
			}
		}
		if dir := owner.metadata; dir != nil && dir.created {
			entries, err := owner.entries(dir)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.directory || !workspaceMetadataName(entry.name) {
					return ErrTrust // validate the whole bounded inventory before deleting
				}
			}
			for _, entry := range entries {
				owner.active = &workspaceFile{path: filepath.Join(dir.path, entry.name)}
				if err := owner.open(owner.active, dir.handle, entry.name, false, false, true); err != nil {
					return err
				}
				identity := owner.active.identity
				if uint64(identity.FileIndexHigh)<<32|uint64(identity.FileIndexLow) != entry.identity ||
					identity.VolumeSerialNumber != dir.identity.VolumeSerialNumber {
					return ErrTrust
				}
				if err := owner.remove(owner.active, dir); err != nil {
					return err
				}
				owner.active = nil
			}
			if err := owner.remove(dir, owner.work); err != nil {
				return err
			}
		}
		if dir := owner.targets; dir != nil && dir.created {
			entries, err := owner.entries(dir)
			if err != nil || len(entries) != 0 {
				return ErrTrust // this adapter never downloads target payloads to disk
			}
			if err := owner.remove(dir, owner.work); err != nil {
				return err
			}
		}
		if err := owner.remove(owner.work, owner.parent()); err != nil {
			return err
		}
	}
	for index := len(owner.ancestors) - 1; index >= 0; index-- {
		if err := owner.ancestors[index].Close(); err != nil {
			return err
		}
	}
	return nil
}

func (owner *nativeWorkspace) remove(file, parent *workspaceFile) error {
	if owner.validate(file) != nil || owner.validate(parent) != nil {
		return ErrTrust
	}
	var iosb windows.IO_STATUS_BLOCK
	disposition := byte(1)
	if err := windows.NtSetInformationFile(file.handle, &iosb, &disposition, 1, windows.FileDispositionInformation); err != nil {
		return ErrTrust // retain original object; no path deletion or close retry
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Successful disposition/Close alone does not prove other SDK handles were
	// retired. Require the original parent to report the exact name absent.
	owner.probe = &workspaceFile{path: file.path, directory: file.directory}
	err := owner.open(owner.probe, parent.handle, filepath.Base(file.path), false, false, false)
	if closeErr := owner.probe.Close(); closeErr != nil {
		return closeErr
	}
	if !errors.Is(err, os.ErrNotExist) {
		return ErrTrust
	}
	owner.probe = nil
	return nil
}
