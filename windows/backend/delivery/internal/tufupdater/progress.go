package updater

import (
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

// SuccessfulMetadataSet is distinct from the SDK's intermediate set so callers
// cannot accidentally substitute GetTrustedMetadataSet for success receipts.
// It is an observation type, never an artifact/application acceptance proof.
type SuccessfulMetadataSet trustedmetadata.TrustedMetadata

// These wrappers return the original SDK result/error unchanged. A successful
// per-role return is observed before subsequent Storage writes can fail. In
// particular, UpdateTimestamp/UpdateSnapshot may assign an intermediate object
// and then reject its expiry/version: that pointer is NOT a success receipt.
// There is no callback, extra verifier, reference-time override or retry here.
func (update *Updater) updateRoot(data []byte) (*metadata.Metadata[metadata.RootType], error) {
	value, err := update.trusted.UpdateRoot(data)
	if err == nil {
		update.successful.Root = value
	}
	return value, err
}

func (update *Updater) updateTimestamp(data []byte) (*metadata.Metadata[metadata.TimestampType], error) {
	value, err := update.trusted.UpdateTimestamp(data)
	if err == nil {
		update.successful.Timestamp = value
	}
	return value, err
}

func (update *Updater) updateSnapshot(data []byte, isTrusted bool) (*metadata.Metadata[metadata.SnapshotType], error) {
	value, err := update.trusted.UpdateSnapshot(data, isTrusted)
	if err == nil {
		update.successful.Snapshot = value
	}
	return value, err
}

func (update *Updater) updateDelegatedTargets(data []byte, role, parent string) (*metadata.Metadata[metadata.TargetsType], error) {
	value, err := update.trusted.UpdateDelegatedTargets(data, role, parent)
	if err == nil {
		update.successful.Targets[role] = value
	}
	return value, err
}

// GetSuccessfulMetadataSet is only a restriction-progress observation. It is not
// a complete/fresh chain or artifact proof, and is not disk persistence. The
// adapter must still bind original bytes, continuity, effective time and links,
// then use the original full SDK set for final application acceptance. Old
// accepted expired history lives in the protected input State, not this set.
// SDK parsed objects are immutable in the pinned workflow; do not mutate them.
func (update *Updater) GetSuccessfulMetadataSet() SuccessfulMetadataSet {
	result := update.successful
	result.RefTime = update.trusted.RefTime
	result.Targets = make(map[string]*metadata.Metadata[metadata.TargetsType], len(update.successful.Targets))
	for role, value := range update.successful.Targets {
		result.Targets[role] = value
	}
	return result
}
