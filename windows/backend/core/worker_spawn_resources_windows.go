//go:build windows

package core

import "github.com/snowden-system/windows/backend/childbootstrap/spawnprep"

var errSpawnResources = spawnprep.ErrResources

type windowsSpawnResources = spawnprep.Pipes
