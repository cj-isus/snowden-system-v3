//go:build !windows

package update

import "os"

func openPairMetadataNative(path string, _ bool) error {
	file, err := os.Open(path)
	if file != nil {
		pairMetadataOwner.file = file
	}
	return err
}

func samePairFile(a, b os.FileInfo) bool { return os.SameFile(a, b) }

func pairMetadataCreatedID(*pairMetadataLease, CreatedFileID) bool { return false }
