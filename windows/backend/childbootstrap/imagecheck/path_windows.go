//go:build windows

package imagecheck

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

const maxPath = 2048

// Component keeps the existing fixed-local image/installer name policy. It
// grants no filesystem authority and does not resolve a path or a principal.
func Component(value string) bool {
	return value != "" && len(value) <= 255 && filepath.IsLocal(value) &&
		!strings.ContainsAny(value, "\\/:*?\"<>|\x00") && strings.TrimRight(value, " .") == value
}

// DirectoryPaths plans at most64 ancestor names on a fixed local drive. The
// caller still owns acquisition, identity/ACL inspection and reverse cleanup.
func DirectoryPaths(directory string) ([]string, error) {
	if len(directory) > maxPath || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrImage
	}
	volume := filepath.VolumeName(directory)
	if len(volume) != 2 || volume[1] != ':' {
		return nil, ErrImage
	}
	root := volume + `\`
	name, err := windows.UTF16PtrFromString(root)
	if err != nil || windows.GetDriveType(name) != windows.DRIVE_FIXED {
		return nil, ErrImage
	}
	paths := []string{root}
	relative := strings.TrimPrefix(directory, root)
	if relative != "" {
		for _, component := range strings.Split(relative, `\`) {
			if !Component(component) || len(paths) >= 64 {
				return nil, ErrImage
			}
			paths = append(paths, filepath.Join(paths[len(paths)-1], component))
		}
	}
	return paths, nil
}

// NTFS checks the original held root, including persistent ACL support. It
// neither opens a volume nor attests other file handles or mapped image bytes.
func NTFS(handle windows.Handle) error {
	if !nativelease.ValidHandleValue(handle) {
		return ErrImage
	}
	var name [32]uint16
	var flags uint32
	if windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, &flags, &name[0], uint32(len(name))) != nil ||
		flags&windows.FILE_PERSISTENT_ACLS == 0 || windows.UTF16ToString(name[:]) != "NTFS" {
		return ErrImage
	}
	return nil
}

// FilePath reads the original held file's final path for fixed-local image and
// installer callers. It does not reopen by name, transfer or close the handle.
func FilePath(handle windows.Handle) (string, error) {
	if !nativelease.ValidHandleValue(handle) {
		return "", ErrImage
	}
	var path [maxPath + 1]uint16
	n, err := windows.GetFinalPathNameByHandle(handle, &path[0], uint32(len(path)), 0)
	if err != nil || n == 0 || n >= uint32(len(path)) {
		return "", ErrImage
	}
	return strings.TrimPrefix(windows.UTF16ToString(path[:n]), `\\?\`), nil
}

// ProcessPathMatches borrows an original process handle (or pseudo-self). A
// matching reported path is not loaded-section, signature or Job authority.
// Original owner serialization and the caller's existing scope remain required.
func ProcessPathMatches(ctx context.Context, process windows.Handle, expected string, scope func(context.Context) error) error {
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return err
	}
	if process != windows.CurrentProcess() && !nativelease.ValidHandleValue(process) || expected == "" || len(expected) > maxPath {
		return ErrImage
	}
	var path [maxPath + 1]uint16
	size := uint32(len(path))
	if windows.QueryFullProcessImageName(process, 0, &path[0], &size) != nil || size == 0 || size >= uint32(len(path)) ||
		!strings.EqualFold(windows.UTF16ToString(path[:size]), expected) {
		return ErrImage
	}
	return childbootstrap.CheckScope(ctx, scope)
}
