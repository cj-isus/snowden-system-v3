package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/secretvault"
)

func deliveryClockFloor(policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState) time.Time {
	return metadata.TrustClockFloor(policy, state)
}

var publicDeliveryReader = filetxn.NewBoundedReadSlot(max(delivery.MaxEncodedPackageBytes+512, metadata.MaxEnvelopeSize))

// Public source bytes escape only after bounded original-handle reads and a
// confirmed Close. An uncertain resource stays in this App intake slot; neither
// another read nor a new SDK workspace replaces it. Signature/source gates still
// decide authority after intake; filesystem checks do not accept a profile.
func readDeliveryFile(path string, limit int64) ([]byte, error) {
	data, err := publicDeliveryReader.Read(path, limit)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("delivery: empty public file")
	}
	return data, nil
}

// Caller holds local import-journal serialization and operation ownership,
// without a native profile scope or App/runtime/Manager mutex. The initial
// source fence acquires a reader; Store holds the final writer. A verified
// prefix is retained even if a later
// target/application gate rejects the artifact. The existing four-target
// recovery manifest is preserved; no fifth file or second journal is introduced.
// This cannot enroll a root. Initial enrollment needs the full profile gate.
func retainDeliveryProgress(store *metadata.Store, policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState, progress *delivery.Progress, vaultManager *secretvault.Manager, vaultPath string, committer importCommitFn) (*metadata.TrustPolicy, error) {
	if progress == nil {
		return policy, nil
	}
	if err := metadata.CheckDeliveryBinding(policy, state); err != nil {
		return nil, err
	}
	if policy.Delivery == nil || vaultManager == nil {
		return nil, errors.New("delivery: retained source authority/vault unavailable")
	}
	source, err := policy.Delivery.Digest()
	if err != nil || source != progress.SourceDigest() {
		return nil, errors.New("delivery: verified prefix source changed")
	}
	next := policy.Scoped()
	next.Delivery = progress.NextState()
	if next.Delivery == nil || next.Delivery.AnchorSHA256 != policy.Delivery.AnchorSHA256 || next.Delivery.ClockFloor.Before(policy.Delivery.ClockFloor) {
		return nil, errors.New("delivery: verified prefix lost anchor/time continuity")
	}
	if err := metadata.CheckDeliveryBinding(next, state); err != nil {
		return nil, err
	}
	encoded, err := metadata.EncodeTrustPolicy(next)
	if err != nil {
		return nil, err
	}
	sourcePolicy, err := metadata.EncodeTrustPolicy(policy)
	if err != nil {
		return nil, err
	}
	before, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	fence := func() error {
		currentPolicy, _, err := store.LoadTrustPolicy()
		if err != nil {
			return err
		}
		currentBytes, err := metadata.EncodeTrustPolicy(currentPolicy)
		if err != nil || currentPolicy.Schema != policy.Schema || !bytes.Equal(currentBytes, sourcePolicy) {
			return errors.New("delivery: public source authority changed during retention")
		}
		currentState, err := metadata.LoadAcceptedProfile(store.Dir)
		if err != nil {
			return err
		}
		after, err := json.Marshal(currentState)
		if err != nil || !bytes.Equal(before, after) {
			return errors.New("delivery: accepted checkpoint changed during retention")
		}
		return nil
	}
	if err = filetxn.WithRead(store.ImportJournal(), fence); err != nil {
		return nil, err
	}
	if bytes.Equal(sourcePolicy, encoded) {
		return next, nil // coherent source, no vault read or transaction
	}
	prepared, err := vaultManager.PrepareImportSnapshot(nil) // encrypted snapshot; no credential export/DPAPI
	if err != nil {
		return nil, err
	}
	files := importTargets(store, vaultPath)
	files[0].Data = encoded
	for i := 1; i < len(files); i++ {
		files[i].Keep = true // including a missing/corrupt old envelope during recovery
	}
	tx := &filetxn.Store{Dir: store.ImportJournal(), BeforeCommit: fence}
	err = vaultManager.WithPreparedTransaction(prepared, tx, func() error {
		if committer != nil {
			return committer(tx, files)
		}
		return tx.Commit(files)
	})
	err = errors.Join(err, profileStorageQuarantineError(tx))
	if err != nil {
		return nil, err
	}
	return next, nil
}

// A successful prefix may be a no-op. Record whether the actual commit callback
// succeeded, independently of the stable authority digest or cache/root changes.
// The caller retains this outcome across its later reads/activation failures.
func retainDeliveryProgressWithOutcome(store *metadata.Store, policy *metadata.TrustPolicy, state *metadata.AcceptedProfileState, progress *delivery.Progress, vaultManager *secretvault.Manager, vaultPath string, committer importCommitFn) (*metadata.TrustPolicy, bool, error) {
	committed := false
	next, err := retainDeliveryProgress(store, policy, state, progress, vaultManager, vaultPath,
		func(tx *filetxn.Store, files []filetxn.File) error {
			var err error
			if committer != nil {
				err = committer(tx, files)
			} else {
				err = tx.Commit(files)
			}
			committed = err == nil
			return err
		})
	return next, committed, err
}
