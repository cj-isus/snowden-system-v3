//go:build windows

package spawnprep

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

type EnvironmentScope uint8

const (
	EnvironmentSystem EnvironmentScope = iota + 1
	EnvironmentPublicStore
	EnvironmentGUI
)

// Values come from the platform's directory APIs. Scope is compiled role policy,
// not an arbitrary environment map. Building does not read the OS or any store.
type EnvironmentLocations struct {
	SystemDirectory, WindowsDirectory                   string
	ConfigDirectory, CacheDirectory, TemporaryDirectory string
}

func fixedEnvironmentDirectory(value string) (string, bool) {
	value = filepath.Clean(value)
	return value, len(value) <= 2048 && filepath.IsAbs(value) && len(filepath.VolumeName(value)) == 2
}

// BuildEnvironment returns caller-owned value buffers for one synchronous native
// CreateProcess. Keep BOTH buffers reachable until that call returns. These paths
// do not authenticate installation/files, or prove loaded-DLL/SDK containment.
func BuildEnvironment(scope EnvironmentScope, locations EnvironmentLocations) (environment, directory []uint16, err error) {
	system, systemOK := fixedEnvironmentDirectory(locations.SystemDirectory)
	windowsDirectory, windowsOK := fixedEnvironmentDirectory(locations.WindowsDirectory)
	if !systemOK || !windowsOK {
		return nil, nil, ErrResources
	}
	entries := []string{"SystemRoot=" + windowsDirectory, "WINDIR=" + windowsDirectory}
	switch scope {
	case EnvironmentSystem:
		if locations.ConfigDirectory != "" || locations.CacheDirectory != "" || locations.TemporaryDirectory != "" {
			return nil, nil, ErrResources
		}
	case EnvironmentPublicStore, EnvironmentGUI:
		store, storeOK := fixedEnvironmentDirectory(locations.ConfigDirectory)
		temporary, tempOK := fixedEnvironmentDirectory(locations.TemporaryDirectory)
		if !storeOK || !tempOK {
			return nil, nil, ErrResources
		}
		entries = []string{"APPDATA=" + store, "SystemRoot=" + windowsDirectory, "TEMP=" + temporary, "TMP=" + temporary, "WINDIR=" + windowsDirectory}
		if scope == EnvironmentGUI {
			cache, cacheOK := fixedEnvironmentDirectory(locations.CacheDirectory)
			if !cacheOK {
				return nil, nil, ErrResources
			}
			entries = []string{"APPDATA=" + store, "LOCALAPPDATA=" + cache, "SystemRoot=" + windowsDirectory, "TEMP=" + temporary, "TMP=" + temporary, "WINDIR=" + windowsDirectory}
		} else if locations.CacheDirectory != "" {
			return nil, nil, ErrResources
		}
	default:
		return nil, nil, ErrResources
	}
	for _, entry := range entries {
		encoded, encodeErr := windows.UTF16FromString(entry)
		if encodeErr != nil || len(encoded) > 4096 {
			return nil, nil, ErrResources
		}
		environment = append(environment, encoded...)
	}
	environment = append(environment, 0)
	directory, err = windows.UTF16FromString(system)
	if err != nil {
		return nil, nil, ErrResources
	}
	return environment, directory, nil
}
