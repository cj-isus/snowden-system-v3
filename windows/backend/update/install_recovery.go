package update

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/snowden-system/windows/backend/netprobe"
)

// A separate forward-only facade over the existing transaction custody/writers.
// It never exposes the original-owner Rollback or Discard capability. Public
// journal old hashes are control facts only; every placed byte must match the
// caller's freshly SDK-authorized NEW manifest. Keep partial owners on errors.
type PairRecovery struct {
	tx      *PairTransaction
	raw     []byte
	sidecar []byte
	archive string
	staged  bool
}

func OpenPairRecovery(ctx context.Context, root string, raw, sidecar []byte, custody DirectoryCustodian) (owner *PairRecovery, finalErr error) {
	owner = &PairRecovery{tx: &PairTransaction{root: root, custody: custody, forwardOnly: true, intentPending: true,
		dirs: make(map[string]os.FileInfo), files: make(map[string]CreatedFileID), writes: make(map[string]*pairStagingWrite)}}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrPairRecoveryRequired)
		}
	}()
	finalErr = owner.prepare(ctx, raw, sidecar)
	completed = true
	return owner, finalErr
}

func (owner *PairRecovery) prepare(ctx context.Context, raw, sidecar []byte) error {
	t := owner.tx
	if len(raw) == 0 || len(raw) > MaxManifestSize || len(sidecar) > maxPairDelivery {
		return ErrPairRecoveryRequired
	}
	var capable bool
	t.creationCustody, capable = t.custody.(CreationCustodian)
	if !capable {
		return ErrPairRecoveryRequired
	}
	t.payloadCustody = t.creationCustody
	candidate, err := LoadPairCandidate(ctx, t.root, t.custody)
	if err != nil || !bytes.Equal(raw, candidate.RawManifest) || !bytes.Equal(sidecar, candidate.Delivery) {
		return errors.Join(ErrPairRecoveryRequired, err)
	}
	owner.raw, owner.sidecar = bytes.Clone(raw), bytes.Clone(sidecar)
	owner.staged, owner.archive = candidate.staged, candidate.archive
	t.dir, t.manifest = candidate.Directory, candidate.Manifest
	t.next = [2]Payload{{File: t.manifest.File, SHA256: t.manifest.SHA256, Size: t.manifest.Size}, t.manifest.NativeHelper.Payload}
	t.rootInfo, err = pairRoot(t.root)
	if err != nil {
		return err
	}
	t.dirInfo, err = pairDirectory(t.dir)
	if err != nil {
		return err
	}
	for _, child := range pairChildDirectories() {
		t.dirs[child], err = pairDirectory(filepath.Join(t.dir, child))
		if err != nil {
			return err
		}
	}
	if candidate.archive == "" && !pairAbsent(filepath.Join(t.root, pairPendingName)) {
		t.pendingInfo, err = pairDirectory(filepath.Join(t.root, pairPendingName))
		if err != nil {
			return err
		}
		t.published = true // includes the exact incomplete stage-publication reservation
	}
	header, err := pairRead(filepath.Join(t.dir, "journal.json"), maxPairRecord)
	if err != nil {
		return err
	}
	t.record, err = decodePairRecord(header)
	if err != nil {
		return err
	}
	if err := t.control(); err != nil {
		return err
	}
	t.prepared = true
	return t.identity(ctx)
}

// Readiness is mandatory even for recovery; public journal facts cannot waive it.
func (owner *PairRecovery) Apply(ctx context.Context, authorize func(func() error) error, readiness PairReadiness) (finalErr error) {
	if readiness == nil {
		return ErrPairRecoveryRequired
	}
	if owner == nil || owner.tx == nil || !owner.tx.mu.TryLock() {
		return ErrPairBusy
	}
	defer owner.tx.mu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrPairRecoveryRequired)
		}
	}()
	finalErr = owner.apply(ctx, authorize, readiness)
	completed = true
	return finalErr
}

func (owner *PairRecovery) apply(ctx context.Context, authorize func(func() error) error, readiness PairReadiness) error {
	t := owner.tx
	if !t.prepared || t.applied || t.retireWrites() != nil || t.gate(ctx, authorize) != nil || t.control() != nil {
		return ErrPairRecoveryRequired
	}
	if err := ensurePairRecoveryIntent(ctx, t.root, owner.raw, owner.sidecar, owner.archive, func(commit func() error) error {
		return t.commit(ctx, authorize, commit)
	}); err != nil {
		return err
	}
	if owner.staged {
		pending := filepath.Join(t.root, pairPendingName)
		if err := t.commit(ctx, authorize, func() error {
			if !t.published {
				created, err := t.creationCustody.CreateDirectory(ctx, pending)
				if created {
					t.published = true // retain a partial native reservation; never retry as absent
				}
				if err != nil || !created {
					return errors.Join(ErrPairRecoveryRequired, err)
				}
				t.pendingInfo, err = pairDirectory(pending)
				if err != nil {
					return err
				}
				if err := t.custody.PinDirectory(ctx, pending); err != nil {
					return err
				}
			}
			destination := filepath.Join(pending, "data")
			if !pairAbsent(destination) {
				return ErrPairRecoveryRequired
			}
			moved, err := t.custody.MoveDirectory(ctx, t.dir, destination)
			if moved {
				t.dir, owner.staged = destination, false
			}
			if err != nil || !moved {
				return errors.Join(ErrPairRecoveryRequired, err)
			}
			return t.identity(ctx)
		}); err != nil {
			return err
		}
	}
	if err := t.prepareRecoveryEntrypoint(ctx, authorize, readiness); err != nil {
		return err
	}
	for _, index := range []int{1, 0} {
		if err := t.gate(ctx, authorize); err != nil {
			return err
		}
		if err := owner.place(ctx, authorize, index); err != nil {
			return err
		}
	}
	if err := t.manifest.VerifyPayloadSet(t.root); err != nil {
		return err
	}
	if err := t.gate(ctx, authorize); err != nil {
		return err // fresh source/time after the last potentially long hash/copy
	}
	t.applied = true // placement only; intent remains, no floor/code acceptance
	return nil
}

func recoveryPayload(ctx context.Context, path string, index int) (*Payload, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, ErrPairRecoveryRequired
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.Join(ErrPairRecoveryRequired, err)
	}
	limit := int64(MaxPayloadSize)
	if index == 1 {
		limit = netprobe.MaxNativeImageBytes
	}
	if info.Size() <= 0 || info.Size() > limit {
		return nil, ErrPairRecoveryRequired
	}
	digest, err := payloadDigestTo(path, info.Size(), pairContextWriter{ctx: ctx, dst: io.Discard})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Payload{File: filepath.Base(path), Size: info.Size(), SHA256: digest}, nil // private current bytes, not old-code authority
}

func (owner *PairRecovery) source(ctx context.Context, index int) (PayloadFile, error) {
	t, expected := owner.tx, owner.tx.next[index]
	// Fixed finite inventory, no directory walk, arbitrary path or old hash.
	for _, child := range []string{"new", "rejected", "restore", "displaced", "old"} {
		if ctx == nil || ctx.Err() != nil {
			return nil, ErrPairRecoveryRequired
		}
		path := filepath.Join(t.dir, child, expected.File)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.Join(ErrPairRecoveryRequired, err)
		}
		if info.Size() != expected.Size {
			continue // unknown bytes stay untouched; reject size before hashing
		}
		payload, err := recoveryPayload(ctx, path, index)
		if err != nil {
			return nil, err
		}
		if payload == nil || *payload != expected {
			continue
		}
		file, err := t.pinPayload(ctx, path, expected)
		if err != nil {
			return file, err
		}
		if child == "new" || child == "restore" || child == "displaced" {
			return file, nil // native move already permits these fixed source names
		}
		// Old/rejected names never move back as code. Copy only authenticated
		// NEW bytes to a fresh existing-family output with checked Sync/Close.
		for _, output := range []string{"new", "restore"} {
			rel := filepath.Join(output, expected.File)
			if !pairAbsent(filepath.Join(t.dir, rel)) {
				continue
			}
			if err := t.writeNew(ctx, rel, func(dst io.Writer) error { return file.CopyTo(ctx, dst) }); err != nil {
				return nil, err
			}
			return t.pinPayload(ctx, filepath.Join(t.dir, rel), expected)
		}
		return nil, ErrPairRecoveryRequired // preserve occupied/unknown outputs
	}
	// Protocol5 byte-identical retained copies can reconstruct a missing main
	// or helper. Their names alone grant nothing; every byte must match NEW.
	aliases := []string{filepath.Join(t.dir, RecoveryHelperCopyFile)}
	if index == 0 {
		aliases = []string{filepath.Join(t.dir, RecoveryApplicationFile), filepath.Join(t.root, RecoveryApplicationFile)}
	}
	for _, path := range aliases {
		payload, err := recoveryPayload(ctx, path, index)
		if err != nil {
			return nil, err
		}
		if payload == nil || payload.Size != expected.Size || payload.SHA256 != expected.SHA256 {
			continue
		}
		for _, output := range []string{"new", "restore"} {
			rel := filepath.Join(output, expected.File)
			if !pairAbsent(filepath.Join(t.dir, rel)) {
				continue
			}
			if err := t.writeNew(ctx, rel, func(dst io.Writer) error { return expected.CopyVerified(path, dst) }); err != nil {
				return nil, err
			}
			return t.pinPayload(ctx, filepath.Join(t.dir, rel), expected)
		}
		return nil, ErrPairRecoveryRequired
	}
	return nil, ErrPairRecoveryRequired
}

func (owner *PairRecovery) place(ctx context.Context, authorize func(func() error) error, index int) error {
	t, expected := owner.tx, owner.tx.next[index]
	target := filepath.Join(t.root, expected.File)
	current, err := recoveryPayload(ctx, target, index)
	if err != nil {
		return err
	}
	if current != nil && *current == expected {
		return nil // actual current-code image pin follows; no unnecessary DELETE open under GUI image lease
	}
	next, err := owner.source(ctx, index)
	if err != nil {
		return err
	}
	var previous PayloadFile
	quarantine := ""
	if current != nil {
		previous, err = t.pinPayload(ctx, target, *current)
		if err != nil {
			return err
		}
		for _, child := range []string{"displaced", "rejected"} {
			path := filepath.Join(t.dir, child, expected.File)
			if pairAbsent(path) {
				quarantine = path
				break
			}
		}
		if quarantine == "" {
			return ErrPairRecoveryRequired // do not overwrite old, unknown or signed source bytes
		}
	}
	if err := t.commit(ctx, authorize, func() error {
		t.namesTouched = true // conservatively sticky even for an uncertain native result
		if previous != nil {
			if !pairAbsent(quarantine) {
				return ErrPairRecoveryRequired
			}
			if moved, err := previous.Move(ctx, quarantine); err != nil || !moved {
				return errors.Join(ErrPairRecoveryRequired, err)
			}
		}
		if !pairAbsent(target) || ctx.Err() != nil {
			return ErrPairRecoveryRequired
		}
		if moved, err := next.Move(ctx, target); err != nil || !moved {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		return next.Validate(ctx)
	}); err != nil {
		return err
	}
	return nil
}

// Checked retirement only. No unlink, rollback or new authority is exposed.
// Payload handles remain in the actual root custodian until its confirmed close.
func (owner *PairRecovery) Retire() error {
	if owner == nil || owner.tx == nil {
		return nil
	}
	if !owner.tx.mu.TryLock() {
		return ErrPairBusy
	}
	defer owner.tx.mu.Unlock()
	owner.tx.prepared = false
	return owner.tx.retireWrites()
}

func (owner *PairRecovery) InstalledNamesTouched() bool {
	owner.tx.mu.Lock()
	defer owner.tx.mu.Unlock()
	return owner.tx.namesTouched
}
