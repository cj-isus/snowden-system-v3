package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/snowden-system/windows/backend/netprobe"
)

// PairTransaction owns exact staging and (after publication) pending resources.
// A nonnil result on error must be retained or explicitly Discarded. It is not a
// code-authority proof. No API reconstructs this old-file owner from a journal.
type PairTransaction struct {
	mu                                                  sync.Mutex
	root, dir                                           string
	rootInfo, dirInfo, pendingInfo                      os.FileInfo
	recoveryInfo                                        os.FileInfo
	dirs                                                map[string]os.FileInfo
	files                                               map[string]CreatedFileID
	writes                                              map[string]*pairStagingWrite
	payloads                                            []PayloadFile
	payloadCustody                                      PayloadCustodian
	creationCustody                                     CreationCustodian
	record                                              pairRecord
	manifest                                            *Manifest
	custody                                             DirectoryCustodian
	next                                                [2]Payload
	prepared, published, attempted, applied, rolledBack bool
	namesTouched                                        bool
	intentPending                                       bool
	forwardOnly                                         bool // a recovered journal can never mint original rollback authority
}

// StagePair streams both new artifacts and exact old backups. manifestRaw and
// deliveryRaw must come from the caller's real authenticated update snapshot;
// structural decoding and public hashes here never supply SDK/role authority.
// All outputs stay in a fresh sibling directory until Apply reserves the fixed
// pending barrier. A failed stage never changes installed artifact names.
func StagePair(ctx context.Context, root, source string, manifestRaw, deliveryRaw []byte, custody DirectoryCustodian) (tx *PairTransaction, err error) {
	tx = &PairTransaction{dirs: make(map[string]os.FileInfo), files: make(map[string]CreatedFileID), writes: make(map[string]*pairStagingWrite), custody: custody}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrPairRecoveryRequired)
		}
	}()
	err = tx.stage(ctx, root, source, manifestRaw, deliveryRaw)
	completed = true
	return tx, err
}

func (t *PairTransaction) stage(ctx context.Context, root, source string, manifestRaw, deliveryRaw []byte) error {
	if ctx == nil || ctx.Err() != nil || len(manifestRaw) == 0 || len(manifestRaw) > MaxManifestSize || len(deliveryRaw) > maxPairDelivery {
		return ErrPairRecoveryRequired
	}
	if t.custody == nil || t.custody.Validate(ctx, root) != nil {
		return ErrPairRecoveryRequired
	}
	var capable bool
	t.creationCustody, capable = t.custody.(CreationCustodian)
	if !capable {
		return ErrPairRecoveryRequired // no path-only substitute for the actual native file owner
	}
	t.payloadCustody = t.creationCustody
	rootInfo, err := pairRoot(root)
	if err != nil {
		return err
	}
	if _, err := pairRoot(source); err != nil {
		return err
	}
	if err := CheckPairPending(root); err != nil {
		return err
	}
	manifestRaw, deliveryRaw = bytes.Clone(manifestRaw), bytes.Clone(deliveryRaw)
	m, err := DecodeStrict(manifestRaw)
	if err != nil || m.Schema != PairSchema {
		return ErrPairRecoveryRequired
	}
	t.root, t.rootInfo, t.manifest = root, rootInfo, m
	t.next = [2]Payload{{File: m.File, SHA256: m.SHA256, Size: m.Size}, m.NativeHelper.Payload}
	t.record = pairRecord{Schema: 1, ManifestSHA256: pairDigest(manifestRaw)}
	if len(deliveryRaw) != 0 {
		t.record.DeliverySHA256 = pairDigest(deliveryRaw)
	}
	old := [2]savedPayload{}
	for index := range old {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(root, pairName(index))
		if index == 1 && pairAbsent(path) {
			continue // explicit first-pair migration may start without a helper
		}
		limit := MaxPayloadSize
		if index == 1 {
			limit = netprobe.MaxNativeImageBytes
		}
		payload, err := DescribePayload(path, limit)
		if err != nil {
			return err
		}
		old[index] = savedPayload{Present: true, SHA256: payload.SHA256, Size: payload.Size}
	}
	if old[0].Size > MaxPayloadSize-old[1].Size {
		return ErrPairRecoveryRequired
	}
	t.record.OldApp, t.record.OldHelper = old[0], old[1]
	if err := ctx.Err(); err != nil {
		return err
	}
	var created bool
	t.dir, created, err = t.creationCustody.CreateUniqueDirectory(ctx, ".snowden-update-stage-")
	if err != nil || !created {
		return errors.Join(ErrPairRecoveryRequired, err)
	}
	t.dirInfo, err = pairDirectory(t.dir)
	if err != nil {
		return err
	}
	if err := t.custody.PinDirectory(ctx, t.dir); err != nil {
		return err
	}
	for _, name := range pairChildDirectories() {
		path := filepath.Join(t.dir, name)
		if created, err := t.creationCustody.CreateDirectory(ctx, path); err != nil || !created {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		t.dirs[name], err = pairDirectory(path)
		if err != nil {
			return err
		}
		if err := t.custody.PinDirectory(ctx, path); err != nil {
			return err
		}
	}
	for index, payload := range t.next {
		if err := t.writeNew(ctx, filepath.Join("new", pairName(index)), func(dst io.Writer) error {
			return payload.CopyVerified(filepath.Join(source, payload.File), dst)
		}); err != nil {
			return err
		}
		if old[index].Present {
			original := old[index].payload(index)
			if err := t.writeNew(ctx, filepath.Join("old", pairName(index)), func(dst io.Writer) error {
				return original.CopyVerified(filepath.Join(root, original.File), dst)
			}); err != nil {
				return err
			}
		}
	}
	// Protocol5 retains one exact NEW recovery pair alongside the immutable
	// control. These are byte copies, never hard links or old-code permission.
	// Additional staged bytes <=MaxPayloadSize (the same combined pair limit).
	for index, rel := range []string{RecoveryApplicationFile, RecoveryHelperCopyFile} {
		payload := t.next[index]
		if err := t.writeNew(ctx, rel, func(dst io.Writer) error {
			return payload.CopyVerified(filepath.Join(source, payload.File), dst)
		}); err != nil {
			return err
		}
	}
	if err := t.bytes(ctx, "update.json", manifestRaw); err != nil {
		return err
	}
	if len(deliveryRaw) != 0 {
		if err := t.bytes(ctx, "delivery.json", deliveryRaw); err != nil {
			return err
		}
	}
	header, err := json.Marshal(t.record)
	if err != nil || len(header) > maxPairRecord {
		return ErrPairRecoveryRequired
	}
	if err := t.bytes(ctx, "journal.json", header); err != nil {
		return err
	}
	if err := t.identity(ctx); err != nil {
		return err
	}
	if err := t.control(); err != nil {
		return err
	}
	if err := t.manifest.VerifyPayloadSet(filepath.Join(t.dir, "new")); err != nil {
		return err
	}
	if err := t.verifyOldBackups(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	t.prepared = true
	return nil
}

func (t *PairTransaction) old(index int) savedPayload {
	if index == 0 {
		return t.record.OldApp
	}
	return t.record.OldHelper
}

func (t *PairTransaction) verifyOldBackups(ctx context.Context) error {
	for index := range t.next {
		old := t.old(index)
		if old.Present {
			path := filepath.Join(t.dir, "old", pairName(index))
			var err error
			if t.published {
				_, err = t.pinPayload(ctx, path, old.payload(index))
			} else {
				err = old.payload(index).Verify(path) // no regular-file pins before stage directory move
			}
			if err != nil {
				return errors.Join(ErrPairRecoveryRequired, err)
			}
		} else if !pairAbsent(filepath.Join(t.dir, "old", pairName(index))) {
			return ErrPairRecoveryRequired
		}
	}
	return nil
}

func (t *PairTransaction) marker(ctx context.Context, name string) error {
	// Names are internal constants. Markers are ordering hints, never authority.
	raw := []byte(t.record.ManifestSHA256 + "\n")
	if original, owned := t.files[name]; owned {
		if resource := t.writes[name]; resource == nil || !resource.complete {
			return ErrPairRecoveryRequired // matching bytes cannot repair an unconfirmed Sync/Close
		}
		current, err := pairCreatedSnapshot(filepath.Join(t.dir, name), original)
		if err == nil && current.Mode().IsRegular() {
			previous, err := pairRead(filepath.Join(t.dir, name), int64(len(raw)))
			if err == nil && bytes.Equal(previous, raw) {
				return ctx.Err() // retry only an exact marker created by this owner
			}
		}
		return ErrPairRecoveryRequired
	}
	return t.bytes(ctx, name, raw)
}

func (t *PairTransaction) control() error {
	expected, err := json.Marshal(t.record)
	if err != nil {
		return err
	}
	header, err := pairRead(filepath.Join(t.dir, "journal.json"), maxPairRecord)
	if err != nil || !bytes.Equal(expected, header) {
		return ErrPairRecoveryRequired
	}
	manifest, err := pairRead(filepath.Join(t.dir, "update.json"), MaxManifestSize)
	if err != nil || pairDigest(manifest) != t.record.ManifestSHA256 {
		return ErrPairRecoveryRequired
	}
	if t.record.DeliverySHA256 == "" {
		if !pairAbsent(filepath.Join(t.dir, "delivery.json")) {
			return ErrPairRecoveryRequired
		}
	} else {
		raw, err := pairRead(filepath.Join(t.dir, "delivery.json"), maxPairDelivery)
		if err != nil || pairDigest(raw) != t.record.DeliverySHA256 {
			return ErrPairRecoveryRequired
		}
	}
	return nil
}

type PairTransactionState struct {
	Prepared, Pending, InstalledNamesTouched, Applied, RolledBack bool
}

// Value-only owner state; no path, raw metadata or authority is exposed.
// Call after the owning operation returns, not from its authorization callback.
func (t *PairTransaction) State() PairTransactionState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return PairTransactionState{Prepared: t.prepared, Pending: t.published || t.intentPending, InstalledNamesTouched: t.namesTouched, Applied: t.applied, RolledBack: t.rolledBack}
}
