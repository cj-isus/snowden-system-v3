package update

import (
	"context"
	"io"
)

const MaxOwnedPairStagingFiles = 24

// Portable NTFS identity captured from the original creating handle, never a
// replacement path. Unknown identity grants no cleanup; this is not code trust.
type CreatedFileID struct {
	VolumeSerialNumber uint32
	IndexHigh          uint32
	IndexLow           uint32
}

func (id CreatedFileID) Valid() bool { return id.IndexHigh != 0 || id.IndexLow != 0 }

type StagingFile interface {
	io.Writer
	Sync() error
	Identity() (CreatedFileID, error)
	Close() error
}

// The actual Windows App supplies its concrete original root/transaction
// directory owner. This is filesystem custody, never update-role/SDK authority.
// Calls are synchronous under the existing installer owner; no App callbacks.
type DirectoryCustodian interface {
	PinDirectory(context.Context, string) error
	Validate(context.Context, string) error
	MoveDirectory(context.Context, string, string) (bool, error)
	ReleaseDirectory(string) error
}

const MaxOwnedPairPayloads = 8 // old installed2, new2, backup2, restore2; moves reuse owners

// A bounded original regular-file receipt, not SDK/signature authority. Open
// authenticates bytes against the caller's at-most-two private snapshots. Moves
// and reads use that original object; an error may still report a changed Path.
// The platform serializer owns all methods, with no concurrent Close/Copy/Move.
type PayloadFile interface {
	Path() string
	Payload() Payload
	Validate(context.Context) error
	CopyTo(context.Context, io.Writer) error
	Move(context.Context, string) (bool, error)
	Close() error
}

type PayloadCustodian interface {
	DirectoryCustodian
	PinPayload(context.Context, string, []Payload) (PayloadFile, error)
	ReleasePayloads() error
}

// Native creators retain original directory/writer outputs even on failure.
// created reports observed creation, never authorization to delete an unknown
// path. A nonnil partial writer belongs to the caller AND the actual root.
type CreationCustodian interface {
	PayloadCustodian
	CreateDirectory(context.Context, string) (created bool, err error)
	CreateUniqueDirectory(context.Context, string) (path string, created bool, err error)
	CreateStagingFile(context.Context, string) (StagingFile, error)
	ReleaseStagingFiles() error
}

// Synchronous platform readiness, not authority supplied by a nonnil callback.
// The actual App runs its owned companion process/current SDK verification and
// invokes bind exactly once with confirmed current root custody. The backend
// keeps its private transaction and IDs while the root writer is handed off.
type PairReadiness func(context.Context, func(DirectoryCustodian) error) error

func pairChildDirectories() [5]string {
	return [5]string{"new", "old", "displaced", "rejected", "restore"}
}
