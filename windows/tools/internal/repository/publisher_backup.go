package repository

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

const maxPublisherBackupData int64 = 288 << 20
const maxPublisherBackupOutput int64 = 320 << 20
const maxPublisherBackupManifest = 4 << 20

type publisherBackupManifest struct {
	Schema         int               `json:"schema"`
	StateSHA256    string            `json:"state_sha256"`
	AuthoritySHA   string            `json:"application_policy_sha256"`
	SourceStoreSHA string            `json:"source_store_sha256"`
	InventorySHA   string            `json:"inventory_sha256"`
	CreatedAt      time.Time         `json:"created_at"`
	Files          []publisherRecord `json:"files"`
}

type boundedBackupWriter struct {
	writer io.Writer
	wrote  int64
	ctx    context.Context
}

func (w *boundedBackupWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > maxPublisherBackupOutput-w.wrote {
		return 0, delivery.ErrBudget
	}
	n, err := w.writer.Write(p)
	w.wrote += int64(n)
	return n, err
}

// Backup copies only exact private state/aliases and accepted public inventory;
// never signing keys, writer.lock, journal, arbitrary discovered files or vault.
// It does not restore/enroll authority or assert publication/server freshness.
func backupPublisher(approvedState, directory, output string) (report []byte, finalErr error) {
	store, release, err := openExistingPublisher(directory)
	if err != nil {
		return nil, err
	}
	written := false
	defer func() {
		if err := release(); err != nil {
			if written {
				err = errors.Join(errors.New("repository: backup written; writer release failed"), err)
			}
			finalErr = errors.Join(finalErr, err)
		}
	}()
	source, err := approvedPublisherSource(store, approvedState)
	if err != nil {
		return nil, err
	}
	output, err = outsidePublisherPath(output, store)
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(errors.New("repository: absent backup output required"), err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	clock := &metadata.ObservedClock{}
	now, err := clock.Now(publisherClockFloor(source.state))
	if err != nil {
		return nil, err
	}
	protected, err := store.protectedInventory(ctx, source)
	if err != nil {
		return nil, err
	}
	var logicalBytes int64
	for _, record := range protected.records {
		if record.Length > maxPublisherBackupData-logicalBytes {
			return nil, delivery.ErrBudget
		}
		logicalBytes += record.Length
	}
	manifest := publisherBackupManifest{1, approvedState, source.state.Authority, publisherStoreIdentity(store), protected.sha256, now, protected.records}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil || len(manifestBytes) > maxPublisherBackupManifest {
		return nil, delivery.ErrBudget
	}
	gate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := publisherPath(output); err != nil {
			return err
		}
		if err := store.fence(source); err != nil {
			return err
		}
		if _, err := clock.Now(publisherClockFloor(source.state)); err != nil {
			return err
		}
		return ctx.Err()
	}
	if err = gate(); err != nil {
		return nil, err
	}
	hash := sha256.New()
	var archiveBytes int64
	write := func(w io.Writer) error {
		bounded := &boundedBackupWriter{writer: io.MultiWriter(w, hash), ctx: ctx}
		archive := zip.NewWriter(bounded)
		put := func(name string, data []byte) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			header := &zip.FileHeader{Name: name, Method: zip.Store}
			header.SetMode(0600)
			header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
			entry, err := archive.CreateHeader(header)
			if err != nil {
				return err
			}
			n, err := entry.Write(data)
			if err == nil && n != len(data) {
				err = io.ErrShortWrite
			}
			return err
		}
		if err := put("backup-manifest.json", manifestBytes); err != nil {
			return err
		}
		for _, record := range protected.records {
			if err := ctx.Err(); err != nil {
				return err
			}
			filename := filepath.Join(store.root, filepath.FromSlash(record.Path))
			if err := publisherPath(filename); err != nil {
				return err
			}
			raw, _, err := publisherfiles.ReadSnapshot(filename, max(1, record.Length), false)
			if err != nil || int64(len(raw)) != record.Length || digest(raw) != record.SHA256 {
				return errors.Join(errors.New("repository: backup source inventory changed"), err)
			}
			if err = put(record.Path, raw); err != nil {
				return err
			}
		}
		if err := archive.Close(); err != nil {
			return err
		}
		archiveBytes = bounded.wrote
		return gate()
	}
	if err = publisherfiles.PublishNewStreamChecked(output, write, gate); err != nil {
		return nil, err
	}
	written = true
	return json.Marshal(struct {
		Operation                  string `json:"operation"`
		SHA256                     string `json:"backup_sha256"`
		Bytes                      int64  `json:"archive_bytes"`
		Files                      int    `json:"accepted_files"`
		LogicalBytes               int64  `json:"logical_bytes"`
		ExternalRolePolicyRequired bool   `json:"external_role_policy_required"`
		RequiresRestoreAcceptance  bool   `json:"requires_restore_acceptance"`
		SigningKeysIncluded        bool   `json:"signing_keys_included"`
	}{"local_publisher_backup", hex.EncodeToString(hash.Sum(nil)), archiveBytes, len(protected.records), logicalBytes, true, true, false})
}
