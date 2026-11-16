package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

const maxMaintenanceObjects = 512
const maxMaintenancePlan = 1 << 20
const maxMaintenanceBytes int64 = 64 << 20
const maintenanceLifetime = 10 * time.Minute

type publisherMaintenancePlan struct {
	Schema       int               `json:"schema"`
	StateSHA256  string            `json:"state_sha256"`
	StoreSHA256  string            `json:"store_sha256"`
	ProtectedSHA string            `json:"protected_sha256"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Files        []publisherRecord `json:"files"`
	Directories  []string          `json:"directories"`
}

func publisherStoreIdentity(s *publisherStore) string {
	return digest([]byte(strings.ToLower(filepath.Clean(s.root))))
}

func approvedPublisherSource(store *publisherStore, approved string) (*publisherSnapshot, error) {
	if !canonicalHash(approved) {
		return nil, errors.New("repository: exact retained state approval required")
	}
	source, err := store.read()
	if err != nil {
		return nil, err
	}
	if source.state == nil || digest(source.raw[0]) != approved {
		return nil, errors.New("repository: approved publisher state changed")
	}
	return source, nil
}

func checkMaintenancePlan(raw []byte) (*publisherMaintenancePlan, error) {
	if len(raw) == 0 || len(raw) > maxMaintenancePlan {
		return nil, delivery.ErrBudget
	}
	fields, err := exactObject(raw, "schema", "state_sha256", "store_sha256", "protected_sha256", "created_at", "expires_at", "files", "directories")
	if err != nil {
		return nil, err
	}
	var entries []json.RawMessage
	if json.Unmarshal(fields["files"], &entries) != nil || len(entries) > maxMaintenanceObjects {
		return nil, delivery.ErrBudget
	}
	for _, entry := range entries {
		if _, err = exactObject(entry, "path", "length", "sha256"); err != nil {
			return nil, err
		}
	}
	var plan publisherMaintenancePlan
	if json.Unmarshal(raw, &plan) != nil || plan.Schema != 1 || !canonicalHash(plan.StateSHA256) || !canonicalHash(plan.StoreSHA256) || !canonicalHash(plan.ProtectedSHA) || plan.CreatedAt.IsZero() || !plan.ExpiresAt.After(plan.CreatedAt) || plan.ExpiresAt.Sub(plan.CreatedAt) > maintenanceLifetime || len(plan.Directories) > maxMaintenanceObjects {
		return nil, errors.New("repository: invalid maintenance intent")
	}
	var total int64
	last := ""
	seen := map[string]bool{}
	for _, record := range plan.Files {
		key := strings.ToLower(record.Path)
		if len(record.Path) > 400 || record.Path <= last || seen[key] || !canonicalHash(record.SHA256) || record.Length < 0 || record.Length > delivery.MaxEncodedPackageBytes || record.Length > maxMaintenanceBytes-total {
			return nil, errors.New("repository: invalid maintenance file inventory")
		}
		last, seen[key], total = record.Path, true, total+record.Length
	}
	last = ""
	for _, name := range plan.Directories {
		key := strings.ToLower(name)
		if len(name) > 400 || name <= last || seen[key] {
			return nil, errors.New("repository: invalid maintenance directory inventory")
		}
		last, seen[key] = name, true
	}
	return &plan, nil
}

// This creates a bounded reviewable intent only; it never deletes an object.
func planPublisherMaintenance(approvedState, directory, output string) (report []byte, finalErr error) {
	store, release, err := openExistingPublisher(directory)
	if err != nil {
		return nil, err
	}
	written := false
	defer func() {
		if err := release(); err != nil {
			if written {
				err = errors.Join(errors.New("repository: maintenance plan written; writer release failed"), err)
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
		return nil, errors.Join(errors.New("repository: absent maintenance plan output required"), err)
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
	plan := publisherMaintenancePlan{Schema: 1, StateSHA256: approvedState, StoreSHA256: publisherStoreIdentity(store), ProtectedSHA: protected.sha256, CreatedAt: now, ExpiresAt: now.Add(maintenanceLifetime), Files: []publisherRecord{}, Directories: []string{}}
	var candidates []publisherRecord
	_, err = walkPublisherTree(ctx, filepath.Join(store.root, "public"), publisherUsage{2 * maxPublisherPublicBytes, 2 * maxPublisherPublicFiles, 2 * maxPublisherPublicDirs}, func(filename string, info os.FileInfo) error {
		rel, err := filepath.Rel(store.root, filename)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !retiredPublisherPath(rel, info.IsDir(), protected) {
			return nil
		}
		if info.IsDir() {
			plan.Directories = append(plan.Directories, rel)
		} else if info.Size() <= delivery.MaxEncodedPackageBytes {
			candidates = append(candidates, publisherRecord{Path: rel, Length: info.Size()})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	var total int64
	for _, record := range candidates {
		if len(plan.Files) >= maxMaintenanceObjects {
			break
		}
		if record.Length > maxMaintenanceBytes-total {
			continue
		}
		filename := filepath.Join(store.root, filepath.FromSlash(record.Path))
		if err = publisherPath(filename); err != nil {
			return nil, err
		}
		raw, _, err := publisherfiles.ReadSnapshot(filename, max(1, record.Length), true)
		if err != nil || int64(len(raw)) != record.Length {
			return nil, errors.Join(errors.New("repository: orphan changed while planning"), err)
		}
		record.SHA256 = digest(raw)
		plan.Files, total = append(plan.Files, record), total+record.Length
	}
	// Prefer deeper directories within the bounded batch, then serialize exact
	// paths in lexical order; apply visits them in reverse (children first).
	sort.Slice(plan.Directories, func(i, j int) bool {
		if len(plan.Directories[i]) != len(plan.Directories[j]) {
			return len(plan.Directories[i]) > len(plan.Directories[j])
		}
		return plan.Directories[i] < plan.Directories[j]
	})
	if len(plan.Directories) > maxMaintenanceObjects {
		plan.Directories = plan.Directories[:maxMaintenanceObjects]
	}
	sort.Strings(plan.Directories)
	raw, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	if _, err = checkMaintenancePlan(raw); err != nil {
		return nil, err
	}
	gate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := store.fence(source); err != nil {
			return err
		}
		now, err := clock.Now(publisherClockFloor(source.state))
		if err != nil || !now.Before(plan.ExpiresAt) {
			return errors.Join(errors.New("repository: maintenance intent expired or clock uncertain"), err)
		}
		return publisherPath(output)
	}
	if err = gate(); err != nil {
		return nil, err
	}
	if err = publisherfiles.PublishNewChecked(output, raw, gate); err != nil {
		return nil, err
	}
	written = true
	return json.Marshal(struct {
		Operation string `json:"operation"`
		SHA256    string `json:"plan_sha256"`
		Files     int    `json:"files"`
		Bytes     int64  `json:"logical_bytes"`
		Dirs      int    `json:"empty_directory_candidates"`
	}{"local_maintenance_plan_only", digest(raw), len(plan.Files), total, len(plan.Directories)})
}
