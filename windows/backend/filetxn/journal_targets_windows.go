//go:build windows

package filetxn

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const maxTargetDirectories = 8 * maxWriteDirectories

type nativeTargetBinding struct {
	path   string
	parent int
}

// Common ancestors occur once, even when metadata and vault use different
// directories/volumes. Every original native output is inside this retained
// tree before acquisition. Parents allow child I/O but deny delete sharing.
type nativeJournalTargets struct {
	parents          boundedNativeWrite
	parentIndices    []int
	bindings         []nativeTargetBinding
	deleting         *nativeFileDeletion
	attempted, ready bool
	closeAttempted   bool
	closeErr         error
}

func newJournalTargets() journalTargets {
	return &nativeJournalTargets{
		parents:       boundedNativeWrite{limit: maxFileSize, directories: make([]boundedNativeWriteFile, 0, maxTargetDirectories)},
		parentIndices: make([]int, 0, maxTargetDirectories),
	}
}

func (owner *nativeJournalTargets) Acquire(files []File) error {
	if owner.attempted || owner.closeAttempted || len(files) == 0 || len(files) > 8 {
		return ErrRecoveryRequired
	}
	owner.attempted = true
	owner.bindings = make([]nativeTargetBinding, len(files))
	// Validate the whole caller-fixed path set before the first native open.
	for index, file := range files {
		path, _, err := windowsReadPath(file.Path)
		volume := filepath.VolumeName(path)
		if err != nil || len(volume) != 2 || volume[1] != ':' || !windowsReadComponent(filepath.Base(path)) {
			return errors.Join(ErrRecoveryRequired, err)
		}
		relative := strings.TrimPrefix(filepath.Dir(path), volume+`\`)
		if relative != "" && len(strings.Split(relative, `\`))+1 > maxWriteDirectories {
			return ErrRecoveryRequired
		}
		owner.bindings[index] = nativeTargetBinding{path: path, parent: -1}
	}
	if err := owner.parents.initTrust(); err != nil {
		return err
	}
	indices := make(map[string]int, maxTargetDirectories)
	for index := range owner.bindings {
		path := owner.bindings[index].path
		root := filepath.VolumeName(path) + `\`
		parent, err := owner.pinDirectory(indices, root, -1)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(filepath.Dir(path), root)
		if relative != "" {
			for _, part := range strings.Split(relative, `\`) {
				parent, err = owner.pinDirectory(indices, filepath.Join(owner.parents.directories[parent].path, part), parent)
				if err != nil {
					return err
				}
			}
		}
		owner.bindings[index].parent = parent
	}
	owner.ready = true
	return owner.Validate()
}

func (owner *nativeJournalTargets) pinDirectory(indices map[string]int, path string, parent int) (int, error) {
	key := strings.ToLower(path)
	if index, ok := indices[key]; ok {
		if owner.parentIndices[index] != parent {
			return -1, ErrRecoveryRequired
		}
		return index, nil
	}
	if len(owner.parents.directories) >= maxTargetDirectories {
		return -1, ErrRecoveryRequired
	}
	index := len(owner.parents.directories)
	owner.parents.directories = append(owner.parents.directories, boundedNativeWriteFile{path: path, directory: true})
	owner.parentIndices = append(owner.parentIndices, parent)
	name, handle, disposition := `\??\`+path, windows.Handle(0), uint32(windows.FILE_OPEN)
	if parent >= 0 {
		name, handle, disposition = filepath.Base(path), owner.parents.directories[parent].handle, windows.FILE_OPEN_IF
	}
	if err := owner.parents.open(&owner.parents.directories[index], handle, name, disposition); err != nil {
		return -1, err
	}
	indices[key] = index
	return index, nil
}

func (owner *nativeJournalTargets) Matches(files []File) bool {
	if len(files) != len(owner.bindings) {
		return false
	}
	for index, file := range files {
		if !strings.EqualFold(file.Path, owner.bindings[index].path) {
			return false
		}
	}
	return true
}

func (owner *nativeJournalTargets) Validate() error {
	if !owner.ready || owner.closeAttempted || owner.deleting != nil {
		return ErrRecoveryRequired
	}
	return owner.parents.validateDirectories()
}

func (owner *nativeJournalTargets) target(path string) int {
	for index, binding := range owner.bindings {
		if strings.EqualFold(path, binding.path) {
			return index
		}
	}
	return -1
}

func (owner *nativeJournalTargets) Check(path string) error {
	index := owner.target(path)
	if !owner.ready || owner.closeAttempted || owner.deleting != nil || index < 0 {
		return ErrRecoveryRequired
	}
	for parent := owner.bindings[index].parent; parent >= 0; parent = owner.parentIndices[parent] {
		if _, err := owner.parents.inspect(&owner.parents.directories[parent]); err != nil {
			return err
		}
	}
	return nil
}

func (owner *nativeJournalTargets) Erase(path string) error {
	if err := owner.Check(path); err != nil {
		return err
	}
	binding := owner.bindings[owner.target(path)]
	parent := &owner.parents.directories[binding.parent]
	owner.deleting = &nativeFileDeletion{} // retain before opening the original target
	err := owner.deleting.erase(&owner.parents, parent, filepath.Base(binding.path), nil)
	if !owner.deleting.Uncertain() {
		owner.deleting = nil
	}
	return err
}

func (owner *nativeJournalTargets) Close() (finalErr error) {
	if owner.closeAttempted {
		return owner.closeErr
	}
	owner.closeAttempted, owner.closeErr, owner.ready = true, ErrWriteRetirement, false
	defer func() {
		if recover() != nil {
			finalErr = ErrWriteRetirement
		}
		owner.closeErr = finalErr
	}()
	if owner.deleting != nil {
		return ErrWriteRetirement
	}
	for index := len(owner.parents.directories) - 1; index >= 0; index-- {
		if err := owner.parents.directories[index].Close(); err != nil {
			return err
		}
	}
	return nil
}
