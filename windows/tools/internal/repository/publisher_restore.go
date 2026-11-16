package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

type publisherRestoreIntent struct {
	Schema       int    `json:"schema"`
	BackupSHA    string `json:"backup_sha256"`
	StateSHA     string `json:"state_sha256"`
	AuthoritySHA string `json:"application_policy_sha256"`
	AnchorSHA    string `json:"anchor_sha256"`
	TargetSHA    string `json:"target_store_sha256"`
}

// New/resume require an empty authority target with an exact cold-restore
// intent. Existing repair keeps the independently approved target state byte
// for byte, including roots/cache/floors/history newer than the backup.
func restorePublisher(archiveFile, archiveSHA, policyFile, policySHA, stateSHA, anchorSHA, targetIntent, directory string) (report []byte, finalErr error) {
	committed := false
	defer func() {
		if committed && finalErr != nil {
			finalErr = errors.Join(errors.New("repository: restore committed; later observation or cleanup failed"), finalErr)
		}
	}()
	isNew, resume := targetIntent == "new", targetIntent == "resume"
	if !isNew && !resume && !canonicalHash(targetIntent) {
		return nil, errors.New("repository: target intent must be new, resume or exact existing-state SHA256")
	}
	layout, err := publisherLayout(directory)
	if err != nil {
		return nil, err
	}
	archiveFile, err = outsidePublisherPath(archiveFile, layout)
	if err != nil {
		return nil, err
	}
	authority, err := readPromotionInput(policyFile, policySHA, metadata.MaxTrustPolicySize, layout)
	if err != nil {
		return nil, err
	}
	if _, err = publisherApplicationPolicy(authority.raw); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	clock := &metadata.ObservedClock{}
	backup, err := openPublisherBackup(ctx, archiveFile, archiveSHA)
	if err != nil {
		return nil, err
	}
	defer func() { finalErr = errors.Join(finalErr, backup.file.Close()) }()
	archived, err := verifyPublisherBackup(ctx, backup, layout, stateSHA, anchorSHA, policySHA, clock)
	if err != nil {
		return nil, err
	}
	created := false
	if isNew {
		if err = publisherPath(layout.root); err != nil {
			return nil, err
		}
		if err = os.Mkdir(layout.root, 0700); err != nil {
			return nil, err // exclusive fresh root; never adopt an existing folder
		}
		created = true
	}
	defer func() {
		if finalErr != nil && created && !committed {
			finalErr = errors.Join(errors.New("repository: new restore folder remains unactivated; inspect explicit recovery/resume state"), finalErr)
		}
	}()
	var store *publisherStore
	var release func() error
	if isNew {
		store, release, err = openPublisher(layout.root)
	} else {
		store, release, err = openExistingPublisher(layout.root)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := release(); err != nil {
			if committed {
				err = errors.Join(errors.New("repository: restore journal committed; writer release failed"), err)
			}
			finalErr = errors.Join(finalErr, err)
		}
	}()
	source, err := store.capture() // aliases may be missing; journal recovery remains explicit
	if err != nil {
		return nil, err
	}
	expected := archived
	intentName := filepath.Join(store.root, "private", "restore-intent.json")
	intentBytes, err := json.Marshal(publisherRestoreIntent{1, archiveSHA, stateSHA, policySHA, anchorSHA, publisherStoreIdentity(store)})
	if err != nil {
		return nil, err
	}
	if isNew || resume {
		for _, present := range source.present {
			if present {
				return nil, errors.New("repository: cold restore requires absent state/aliases; existing authority cannot be reset")
			}
		}
		if isNew {
			if err = writeImmutable(intentName, intentBytes); err != nil {
				return nil, err
			}
		} else {
			raw, present, err := optionalPublisherFile(intentName, 4096)
			if err != nil || !present || !bytes.Equal(raw, intentBytes) {
				return nil, errors.Join(errors.New("repository: exact original new-restore intent required for resume"), err)
			}
		}
	} else if source.state == nil || digest(source.raw[0]) != targetIntent || source.state.Authority != policySHA || source.state.Delivery.AnchorSHA256 != anchorSHA {
		return nil, errors.New("repository: existing target checkpoint/policy/lineage differs; refusing replacement")
	}
	// Use the backup when it has the exact known path. An existing newer target
	// may supply intact objects absent from an older backup; no trust is adopted
	// from those objects beyond its independently approved target checkpoint.
	read := func(name string, limit int64) ([]byte, error) {
		if backup.entries[name] != nil {
			return backup.read(ctx, name, limit)
		}
		filename := filepath.Join(store.root, filepath.FromSlash(name))
		if err := publisherPath(filename); err != nil {
			return nil, err
		}
		return publisherfiles.Read(filename, limit)
	}
	if !isNew && !resume {
		expected, err = restoreExpectedSnapshot(ctx, source, read)
		if err != nil {
			return nil, err
		}
	}
	inventory, err := store.checkpointInventory(ctx, expected, read)
	if err != nil {
		return nil, err
	}
	pending := map[string]int64{}
	for _, record := range inventory.records {
		if restoreControlName(record.Path) || backup.entries[record.Path] == nil {
			continue
		}
		if backup.records[record.Path] != record {
			return nil, errors.New("repository: backup conflicts with retained target object")
		}
		pending[filepath.Join(store.root, filepath.FromSlash(record.Path))] = record.Length
	}
	if err = store.admitPublicRecords(ctx, pending); err != nil {
		return nil, err
	}
	controlFence := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := store.capture()
		if err != nil {
			return err
		}
		if err = samePublisherSource(source, current); err != nil {
			return err
		}
		if isNew || resume {
			raw, present, err := optionalPublisherFile(intentName, 4096)
			if err != nil || !present || !bytes.Equal(raw, intentBytes) {
				return errors.New("repository: cold-restore intent changed")
			}
		}
		if err = authority.check(); err != nil {
			return err
		}
		if _, err = clock.Now(publisherClockFloor(expected.state)); err != nil {
			return err
		}
		return backup.identity(ctx)
	}
	if err = controlFence(); err != nil {
		return nil, err
	}
	for _, record := range inventory.records {
		if restoreControlName(record.Path) || backup.entries[record.Path] == nil {
			continue
		}
		raw, err := backup.read(ctx, record.Path, record.Length)
		if err != nil {
			return nil, err
		}
		if err = writeImmutable(filepath.Join(store.root, filepath.FromSlash(record.Path)), raw); err != nil {
			return nil, err // conflicting existing bytes are never silently overwritten
		}
	}
	finalGate := func() error {
		if err := controlFence(); err != nil {
			return err
		}
		localRead := func(name string, limit int64) ([]byte, error) {
			filename := filepath.Join(store.root, filepath.FromSlash(name))
			if err := publisherPath(filename); err != nil {
				return nil, err
			}
			return publisherfiles.Read(filename, limit)
		}
		actual, err := store.checkpointInventory(ctx, expected, localRead)
		if err != nil || actual.sha256 != inventory.sha256 {
			return errors.Join(errors.New("repository: staged restore inventory changed"), err)
		}
		if err = backup.check(ctx); err != nil {
			return err
		}
		return controlFence()
	}
	files := store.targets() // still exactly state/current/root/timestamp
	if isNew || resume {
		files[0].Data, files[0].Keep = expected.raw[0], false
	}
	for i := 1; i < len(files); i++ {
		if expected.present[i] {
			files[i].Data, files[i].Keep = expected.raw[i], false
		}
	}
	if err = controlFence(); err != nil {
		return nil, err
	}
	journal := filetxn.Store{Dir: store.journal, BeforeCommit: finalGate}
	if err = journal.Commit(files); err != nil {
		return nil, err
	}
	committed = true
	current, err := store.read()
	if err != nil || !bytes.Equal(current.raw[0], expected.raw[0]) {
		return nil, errors.Join(errors.New("repository: restore committed; final checkpoint read failed"), err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	now, err := clock.Now(publisherClockFloor(current.state))
	if err != nil {
		return nil, errors.Join(errors.New("repository: restore committed; final clock uncertain"), err)
	}
	mode := "repair_preserving_target_checkpoint"
	if isNew || resume {
		mode = "approved_checkpoint_replica"
	}
	return json.Marshal(struct {
		Operation            string `json:"operation"`
		Mode                 string `json:"mode"`
		StateSHA256          string `json:"state_sha256"`
		PublicationTimeValid bool   `json:"publication_time_valid"`
		ClientAcceptance     string `json:"client_acceptance"`
		ExternalReachability string `json:"external_reachability"`
	}{"local_publisher_restore", mode, digest(current.raw[0]), restorePublicationTimeValid(current, now), "required", "not_observed"})
}

func restoreControlName(name string) bool {
	switch name {
	case "private/state.json", "public/current.json", "public/metadata/root.json", "public/metadata/timestamp.json":
		return true
	}
	return false
}
