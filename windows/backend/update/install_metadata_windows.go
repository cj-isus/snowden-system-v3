//go:build windows

package update

import (
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/filetxn"
)

func openPairMetadataNative(path string, directory bool) error {
	limit := int64(MaxPayloadSize + 1)
	if directory {
		limit = 0
	}
	original := filetxn.NewWindowsRead(path, directory, limit)
	pairMetadataOwner.file = original // retain before NtCreateFile writes its output
	if err := original.Open(); err != nil {
		return errors.Join(ErrPairRecoveryRequired, err)
	}
	return nil
}

func pairMetadataCreatedID(lease *pairMetadataLease, expected CreatedFileID) bool {
	if lease == nil || lease.released || lease.owner.file == nil {
		return false
	}
	info, err := lease.owner.file.Stat()
	return err == nil && info.Mode().IsRegular() && filetxn.MatchesWindowsFileID(info, expected.VolumeSerialNumber, expected.IndexHigh, expected.IndexLow)
}

func samePairFile(a, b os.FileInfo) bool { return filetxn.SameWindowsFile(a, b) }
