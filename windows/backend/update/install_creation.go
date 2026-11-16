package update

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/snowden-system/windows/backend/netprobe"
)

// Shared exact write namespace for the backend and concrete native creator.
// No arbitrary upload, directory walk, installed target or unknown phase name.
func StagingFileLimit(rel string) (int64, error) {
	if len(rel) == 0 || len(rel) > 256 || !filepath.IsLocal(rel) || filepath.Clean(rel) != rel {
		return 0, ErrPairRecoveryRequired
	}
	switch rel {
	case "update.json":
		return MaxManifestSize, nil
	case "delivery.json":
		return maxPairDelivery, nil
	case "journal.json":
		return maxPairRecord, nil
	case "prepared", "helper-placed", "app-placed", "applied", "rolled-back":
		return 65, nil
	case RecoveryApplicationFile:
		return MaxPayloadSize, nil
	case RecoveryHelperCopyFile:
		return netprobe.MaxNativeImageBytes, nil
	}
	dir, name := filepath.Dir(rel), filepath.Base(rel)
	if dir != "new" && dir != "old" && dir != "restore" {
		return 0, ErrPairRecoveryRequired
	}
	switch name {
	case ApplicationFile:
		return MaxPayloadSize, nil
	case RecoveryApplicationFile:
		if dir == "new" || dir == "restore" {
			return MaxPayloadSize, nil
		}
	case netprobe.NativeHelperName:
		return netprobe.MaxNativeImageBytes, nil
	}
	return 0, ErrPairRecoveryRequired
}

// Read the current name through the existing checked reader; compare to the
// ORIGINAL native creation ID, never to a path captured after an opaque open.
func pairCreatedSnapshot(path string, expected CreatedFileID) (info os.FileInfo, finalErr error) {
	if !expected.Valid() {
		return nil, ErrPairRecoveryRequired
	}
	lease, err := openPairMetadata(path, false)
	if err != nil {
		return nil, err
	}
	defer func() { finalErr = errors.Join(finalErr, lease.Close()) }()
	if !pairMetadataCreatedID(lease, expected) {
		return nil, ErrPairRecoveryRequired
	}
	info, err = lease.owner.file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(ErrPairRecoveryRequired, err)
	}
	return info, nil
}
