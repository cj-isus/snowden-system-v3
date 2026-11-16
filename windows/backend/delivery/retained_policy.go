package delivery

import (
	"fmt"
	"reflect"

	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

// These checks supplement the SDK when it could not load the seeded cache.
// They do not verify signatures or authorize artifacts. Same-version signed
// content is immutable; signatures/JSON formatting are not version content.
func timestampContinuity(raw []byte, next *tuf.Metadata[tuf.TimestampType]) error {
	if len(raw) == 0 {
		return nil
	}
	old, err := tuf.Timestamp().FromBytes(raw)
	if err != nil || next == nil {
		return ErrTrust
	}
	before, after := old.Signed.Meta["snapshot.json"], next.Signed.Meta["snapshot.json"]
	if before == nil || after == nil || next.Signed.Version < old.Signed.Version || after.Version < before.Version {
		return fmt.Errorf("%w: retained timestamp rollback", ErrTrust)
	}
	if next.Signed.Version == old.Signed.Version && !reflect.DeepEqual(next.Signed, old.Signed) {
		return fmt.Errorf("%w: timestamp version content changed", ErrTrust)
	}
	return nil
}

func snapshotContinuity(raw []byte, next *tuf.Metadata[tuf.SnapshotType]) error {
	if len(raw) == 0 {
		return nil
	}
	old, err := tuf.Snapshot().FromBytes(raw)
	if err != nil || next == nil {
		return ErrTrust
	}
	// Mirror the SDK's retained target-reference floors. An intermediate
	// snapshot may have a version different from the current timestamp, so
	// its own version is not an additional unconditional rollback counter.
	for name, before := range old.Signed.Meta {
		after := next.Signed.Meta[name]
		if before == nil || after == nil || after.Version < before.Version {
			return fmt.Errorf("%w: retained snapshot reference rollback", ErrTrust)
		}
	}
	if next.Signed.Version == old.Signed.Version && !reflect.DeepEqual(next.Signed, old.Signed) {
		return fmt.Errorf("%w: snapshot version content changed", ErrTrust)
	}
	return nil
}
