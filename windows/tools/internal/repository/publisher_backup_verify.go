package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
)

func decodePublisherBackupManifest(raw []byte) (*publisherBackupManifest, error) {
	if len(raw) == 0 || len(raw) > maxPublisherBackupManifest {
		return nil, delivery.ErrBudget
	}
	fields, err := exactObject(raw, "schema", "state_sha256", "application_policy_sha256", "source_store_sha256", "inventory_sha256", "created_at", "files")
	if err != nil {
		return nil, err
	}
	var entries []json.RawMessage
	if json.Unmarshal(fields["files"], &entries) != nil || len(entries) < 1 || len(entries) > 2*maxPublisherPublicFiles {
		return nil, delivery.ErrBudget
	}
	for _, entry := range entries {
		if _, err = exactObject(entry, "path", "length", "sha256"); err != nil {
			return nil, err
		}
	}
	var manifest publisherBackupManifest
	if json.Unmarshal(raw, &manifest) != nil || manifest.Schema != 1 || !canonicalHash(manifest.StateSHA256) || !canonicalHash(manifest.AuthoritySHA) || !canonicalHash(manifest.SourceStoreSHA) || !canonicalHash(manifest.InventorySHA) || manifest.CreatedAt.IsZero() {
		return nil, errors.New("repository: invalid backup manifest")
	}
	last := ""
	seen := map[string]bool{}
	var total int64
	for _, record := range manifest.Files {
		key := strings.ToLower(record.Path)
		if record.Path <= last || seen[key] || !backupPublisherName(record.Path) || !canonicalHash(record.SHA256) || record.Length < 1 || record.Length > delivery.MaxEncodedPackageBytes || record.Length > maxPublisherBackupData-total {
			return nil, errors.New("repository: invalid accepted backup inventory")
		}
		last, seen[key], total = record.Path, true, total+record.Length
	}
	return &manifest, nil
}

// Independent checkpoint/lineage/policy approvals are mandatory in addition to
// the ZIP hash. Reconstruct the exact accepted inventory before creating any
// target. Historical copy is not new SDK artifact acceptance or key enrollment.
func verifyPublisherBackup(ctx context.Context, b *publisherBackupReader, layout *publisherStore, approvedState, approvedAnchor, approvedPolicy string, clock *metadata.ObservedClock) (*publisherSnapshot, error) {
	if !canonicalHash(approvedState) || !canonicalHash(approvedAnchor) || !canonicalHash(approvedPolicy) || clock == nil {
		return nil, errors.New("repository: independent checkpoint/lineage/policy approvals required")
	}
	raw, err := b.read(ctx, "backup-manifest.json", maxPublisherBackupManifest)
	if err != nil {
		return nil, err
	}
	b.manifest, err = decodePublisherBackupManifest(raw)
	if err != nil {
		return nil, err
	}
	if b.manifest.StateSHA256 != approvedState || b.manifest.AuthoritySHA != approvedPolicy || len(b.entries) != len(b.manifest.Files)+1 {
		return nil, errors.New("repository: independently approved backup intent differs")
	}
	b.records = make(map[string]publisherRecord, len(b.manifest.Files))
	for _, record := range b.manifest.Files {
		entry := b.entries[record.Path]
		if entry == nil || entry.UncompressedSize64 != uint64(record.Length) {
			return nil, errors.New("repository: backup entry differs from declared inventory")
		}
		b.records[record.Path] = record
	}
	source := &publisherSnapshot{}
	for i, name := range []string{"private/state.json", "public/current.json", "public/metadata/root.json", "public/metadata/timestamp.json"} {
		if b.entries[name] == nil {
			continue
		}
		limits := []int64{maxPublisherState, 16 << 10, delivery.MaxMetadataBytes, 64 << 10}
		source.raw[i], err = b.read(ctx, name, limits[i])
		if err != nil {
			return nil, err
		}
		source.present[i] = true
	}
	if !source.present[0] || digest(source.raw[0]) != approvedState {
		return nil, errors.New("repository: backup checkpoint approval mismatch")
	}
	source.state, err = decodePublisherState(source.raw[0])
	if err != nil {
		return nil, err
	}
	if source.state.Authority != approvedPolicy || source.state.Delivery.AnchorSHA256 != approvedAnchor {
		return nil, errors.New("repository: backup policy or enrollment lineage differs")
	}
	if err = source.checkAliases(); err != nil {
		return nil, err
	}
	root, err := decodeDocument("root", source.state.Delivery.Root)
	if err != nil {
		return nil, err
	}
	if err = root.verify(root); err != nil {
		return nil, err
	}
	now, err := clock.Now(publisherClockFloor(source.state))
	if err != nil || b.manifest.CreatedAt.Before(publisherClockFloor(source.state)) || b.manifest.CreatedAt.After(now.Add(metadata.MaxClockSkew)) {
		return nil, errors.Join(metadata.ErrClockUncertain, err)
	}
	derived, err := layout.checkpointInventory(ctx, source, func(name string, limit int64) ([]byte, error) { return b.read(ctx, name, limit) })
	if err != nil {
		return nil, err
	}
	if derived.sha256 != b.manifest.InventorySHA || len(derived.records) != len(b.records) {
		return nil, errors.New("repository: backup inventory is not exactly checkpoint-derived")
	}
	for _, record := range derived.records {
		if b.records[record.Path] != record {
			return nil, errors.New("repository: unaccepted or mismatched backup object")
		}
	}
	if err = b.check(ctx); err != nil {
		return nil, err
	}
	return source, ctx.Err()
}

// Plan canonical aliases from the retained target head. The target's exact
// state/floors stay unchanged, even if a compatible backup checkpoint is older.
func restoreExpectedSnapshot(ctx context.Context, target *publisherSnapshot, read func(string, int64) ([]byte, error)) (*publisherSnapshot, error) {
	if ctx == nil || target == nil || target.state == nil || read == nil {
		return nil, delivery.ErrTrust
	}
	out := &publisherSnapshot{state: target.state}
	out.raw[0], out.present[0] = target.raw[0], true
	if target.state.Published == nil {
		return out, nil
	}
	head := *target.state.Published
	name := "public/" + head.Directory + "/candidates/" + head.CandidateSHA256 + ".json"
	raw, err := read(name, delivery.MaxEncodedPackageBytes)
	if err != nil {
		return nil, err
	}
	view, err := decodeAcceptedHead(raw, head)
	if err != nil {
		return nil, err
	}
	out.raw[1], out.raw[2], out.raw[3] = view.current, view.stage.files["metadata/root.json"], view.stage.files["metadata/timestamp.json"]
	out.present[1], out.present[2], out.present[3] = true, true, true
	if err = out.checkAliases(); err != nil {
		return nil, err
	}
	return out, ctx.Err()
}

func restorePublicationTimeValid(source *publisherSnapshot, now time.Time) bool {
	return source != nil && source.state != nil && source.state.Published != nil && checkPublicationTime(source.state.Published.ValidUntil, publisherClockFloor(source.state), now) == nil
}
