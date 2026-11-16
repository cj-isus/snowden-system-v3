package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

func maintenanceTimeGate(ctx context.Context, plan *publisherMaintenancePlan, source *publisherSnapshot, clock *metadata.ObservedClock) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now, err := clock.Now(publisherClockFloor(source.state))
	if err != nil || !now.Before(plan.ExpiresAt) || plan.CreatedAt.After(now.Add(metadata.MaxClockSkew)) {
		return errors.Join(errors.New("repository: maintenance intent expired or clock uncertain"), err)
	}
	return nil
}

// No recursive removal. Nonempty directories are retained, including files not
// listed in this bounded batch. The final identity/path checks cannot defeat a
// hostile native rename writer; protected ACL/handle deletion remain OS gates.
func removeEmptyPublisherDir(ctx context.Context, filename string, gate func() error) (removed, absent bool, finalErr error) {
	if err := ctx.Err(); err != nil {
		return false, false, err
	}
	if err := publisherPath(filename); err != nil {
		return false, false, err
	}
	info, err := os.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false, true, nil
	}
	if err != nil || !info.IsDir() {
		return false, false, errors.Join(errors.New("repository: planned directory changed"), err)
	}
	f, err := os.Open(filename)
	if err != nil {
		return false, false, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
		return false, false, errors.New("repository: planned directory identity changed")
	}
	entries, readErr := f.ReadDir(1)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return false, false, readErr
	}
	if err = f.Close(); err != nil {
		return false, false, err
	}
	if len(entries) != 0 {
		return false, false, nil
	}
	if err = publisherPath(filename); err != nil {
		return false, false, err
	}
	after, err := os.Lstat(filename)
	if err != nil || !after.IsDir() || !os.SameFile(info, after) {
		return false, false, errors.New("repository: empty directory changed before removal")
	}
	if err = gate(); err != nil {
		return false, false, err
	}
	err = os.Remove(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false, true, nil
	}
	return err == nil, false, err
}

// Explicit approved plan only. Partial completion is honest and retryable while
// this exact checkpoint/protection set/expiry remains valid. The agent does not
// execute this operation merely by implementing the source.
func applyPublisherMaintenance(planFile, approvedPlan, directory string) (report []byte, finalErr error) {
	store, release, err := openExistingPublisher(directory)
	if err != nil {
		return nil, err
	}
	removed, removedDirs, absent := 0, 0, 0
	var removedBytes int64
	defer func() {
		finalErr = errors.Join(finalErr, release())
		if finalErr != nil && (removed != 0 || removedDirs != 0) {
			finalErr = errors.Join(fmt.Errorf("repository: maintenance partly applied: %d retired files and %d empty directories removed", removed, removedDirs), finalErr)
		}
	}()
	input, err := readPromotionInput(planFile, approvedPlan, maxMaintenancePlan, store)
	if err != nil {
		return nil, err
	}
	plan, err := checkMaintenancePlan(input.raw)
	if err != nil {
		return nil, err
	}
	source, err := approvedPublisherSource(store, plan.StateSHA256)
	if err != nil {
		return nil, err
	}
	if plan.StoreSHA256 != publisherStoreIdentity(store) {
		return nil, errors.New("repository: maintenance intent belongs to another store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	clock := &metadata.ObservedClock{}
	if err = maintenanceTimeGate(ctx, plan, source, clock); err != nil {
		return nil, err
	}
	protected, err := store.protectedInventory(ctx, source)
	if err != nil {
		return nil, err
	}
	if plan.ProtectedSHA != protected.sha256 {
		return nil, errors.New("repository: protected maintenance inventory changed")
	}
	for _, record := range plan.Files {
		if !retiredPublisherPath(record.Path, false, protected) {
			return nil, errors.New("repository: plan targets a protected or unsupported file")
		}
	}
	for _, name := range plan.Directories {
		if !retiredPublisherPath(name, true, protected) {
			return nil, errors.New("repository: plan targets a protected or unsupported directory")
		}
	}
	timeGate := func() error {
		if err := maintenanceTimeGate(ctx, plan, source, clock); err != nil {
			return err
		}
		return filetxn.Check(store.journal)
	}
	fence := func() error {
		if err := timeGate(); err != nil {
			return err
		}
		if err := input.check(); err != nil {
			return err
		}
		return store.fence(source)
	}
	if err = fence(); err != nil {
		return nil, err
	}
	for i, record := range plan.Files {
		if i%16 == 0 {
			if err = fence(); err != nil {
				return nil, err
			}
		}
		if err = filetxn.Check(store.journal); err != nil {
			return nil, err
		}
		filename := filepath.Join(store.root, filepath.FromSlash(record.Path))
		if err = publisherPath(filename); err != nil {
			return nil, err
		}
		_, err = os.Lstat(filename)
		if errors.Is(err, os.ErrNotExist) {
			absent++
			continue
		}
		if err != nil {
			return nil, err
		}
		raw, info, err := publisherfiles.ReadSnapshot(filename, max(1, record.Length), true)
		if err != nil || int64(len(raw)) != record.Length || digest(raw) != record.SHA256 {
			return nil, errors.Join(errors.New("repository: planned file content changed"), err)
		}
		if err = publisherPath(filename); err != nil {
			return nil, err
		}
		after, err := os.Lstat(filename)
		if err != nil || !after.Mode().IsRegular() || !os.SameFile(info, after) || after.Size() != record.Length {
			return nil, errors.New("repository: planned file identity changed before removal")
		}
		if err = timeGate(); err != nil {
			return nil, err
		}
		if err = os.Remove(filename); err != nil {
			return nil, err
		}
		removed, removedBytes = removed+1, removedBytes+record.Length
	}
	for i := len(plan.Directories) - 1; i >= 0; i-- {
		if i%16 == 0 {
			if err = fence(); err != nil {
				return nil, err
			}
		}
		filename := filepath.Join(store.root, filepath.FromSlash(plan.Directories[i]))
		done, missing, err := removeEmptyPublisherDir(ctx, filename, timeGate)
		if err != nil {
			return nil, err
		}
		if done {
			removedDirs++
		}
		if missing {
			absent++
		}
	}
	if err = fence(); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Operation     string `json:"operation"`
		FilesRemoved  int    `json:"retired_files_removed"`
		DirsRemoved   int    `json:"empty_directories_removed"`
		AlreadyAbsent int    `json:"already_absent"`
		LogicalBytes  int64  `json:"logical_bytes_removed"`
		TrustChanged  bool   `json:"trust_changed"`
	}{"local_approved_maintenance", removed, removedDirs, absent, removedBytes, false})
}
