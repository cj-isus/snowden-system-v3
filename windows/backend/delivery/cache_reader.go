package delivery

import (
	"errors"

	"github.com/snowden-system/windows/backend/filetxn"
)

// Keep SDK seed reads separate from App intake and the SDK's opaque internal I/O.
var metadataCacheReader = filetxn.NewBoundedReadSlot(MaxMetadataBytes)

func readCache(name string) ([]byte, error) {
	raw, err := metadataCacheReader.Read(name, MaxMetadataBytes)
	if err != nil {
		return nil, errors.Join(ErrTrust, err)
	}
	return raw, nil
}
