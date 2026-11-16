package update

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Caller owns the original transaction mutex. Publication concerns only the
// fixed auxiliary NEW-code name; installed Main/helper have not changed yet.
func (t *PairTransaction) publishRecoveryEntrypoint(ctx context.Context, authorize func(func() error) error) error {
	expected := t.next[0]
	expected.File = RecoveryApplicationFile // fixed byte-identical application alias
	target := filepath.Join(t.root, expected.File)
	current, err := recoveryPayload(ctx, target, 0)
	if err != nil {
		return err
	}
	if current != nil && *current == expected {
		t.recoveryInfo, err = pairMetadataSnapshot(target, false)
		return err // actual read-only image and current-code SDK readiness still follow
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
			return ErrPairRecoveryRequired
		}
	}
	var source PayloadFile
	ownedOutput := ""
	for _, output := range []string{"new", "restore"} {
		rel := filepath.Join(output, expected.File)
		path := filepath.Join(t.dir, rel)
		if !pairAbsent(path) {
			if resource := t.writes[rel]; resource != nil && !resource.complete {
				return ErrPairRecoveryRequired // matching bytes never erase this owner's failed Sync
			}
			payload, err := recoveryPayload(ctx, path, 0)
			if err != nil {
				return err
			}
			if payload == nil || *payload != expected {
				continue // occupied unknown output is preserved
			}
		} else {
			inputs := []string{filepath.Join(t.dir, RecoveryApplicationFile), filepath.Join(t.dir, "new", ApplicationFile),
				filepath.Join(t.root, ApplicationFile), filepath.Join(t.dir, "rejected", ApplicationFile),
				filepath.Join(t.dir, "restore", ApplicationFile), filepath.Join(t.dir, "displaced", ApplicationFile), filepath.Join(t.dir, "old", ApplicationFile)}
			copied := false
			for _, input := range inputs {
				payload, err := recoveryPayload(ctx, input, 0)
				if err != nil {
					return err
				}
				if payload == nil || payload.Size != expected.Size || payload.SHA256 != expected.SHA256 {
					continue
				}
				if err := t.writeNew(ctx, rel, func(dst io.Writer) error { return expected.CopyVerified(input, dst) }); err != nil {
					return err
				}
				copied = true
				break
			}
			if !copied {
				return ErrPairRecoveryRequired
			}
		}
		source, err = t.pinPayload(ctx, path, expected)
		if err != nil {
			return err
		}
		if _, created := t.files[rel]; created {
			ownedOutput = rel
		}
		break
	}
	if source == nil {
		return ErrPairRecoveryRequired
	}
	if err := t.commit(ctx, authorize, func() error {
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
		moved, err := source.Move(ctx, target)
		if moved && ownedOutput != "" {
			delete(t.files, ownedOutput) // physical object now recorded at the exact auxiliary root name below
		}
		if err != nil || !moved {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		t.recoveryInfo, err = pairMetadataSnapshot(target, false)
		return err
	}); err != nil {
		return err
	}
	return source.Validate(ctx)
}

func (t *PairTransaction) prepareRecoveryEntrypoint(ctx context.Context, authorize func(func() error) error, readiness PairReadiness) error {
	if readiness == nil || t.identity(ctx) != nil || t.retireWrites() != nil {
		return ErrPairRecoveryRequired
	}
	for _, resource := range t.writes {
		if !resource.complete {
			return ErrPairRecoveryRequired
		}
	}
	if err := t.publishRecoveryEntrypoint(ctx, authorize); err != nil {
		return err
	}
	if err := t.retireWrites(); err != nil {
		return err
	}
	if err := t.payloadCustody.ReleasePayloads(); err != nil {
		return err // no overlapping DELETE owner under the read-only companion image
	}
	t.payloads = nil // original native custodian confirmed every payload close
	paths := []string{}
	for path := t.dir; path != t.root; path = filepath.Dir(path) {
		relative, err := filepath.Rel(t.root, path)
		if len(paths) >= 3 || err != nil || !filepath.IsLocal(relative) {
			return ErrPairRecoveryRequired
		}
		paths = append(paths, path)
	}
	for left, right := 0, len(paths)-1; left < right; left, right = left+1, right-1 {
		paths[left], paths[right] = paths[right], paths[left]
	}
	for _, child := range pairChildDirectories() {
		paths = append(paths, filepath.Join(t.dir, child))
	}
	if len(paths) > 8 || t.identity(ctx) != nil {
		return ErrPairRecoveryRequired
	}
	ids := make(map[string]os.FileInfo, len(paths))
	for _, path := range paths {
		info, err := pairDirectory(path)
		if err != nil {
			return err
		}
		ids[path] = info // original writer still held; private facts, never reconstructed from journal
	}
	fileIDs := make(map[string]os.FileInfo, len(t.files))
	for rel, created := range t.files {
		info, err := pairCreatedSnapshot(filepath.Join(t.dir, rel), created)
		if err != nil {
			return ErrPairRecoveryRequired
		}
		fileIDs[rel] = info // original creation ID is stable; capture completed size/mtime now
	}
	called, repeated := false, false
	var bindErr error
	var callbackMu sync.Mutex
	active := true
	defer func() {
		callbackMu.Lock()
		active = false
		callbackMu.Unlock() // no admitted bind can outlive the transaction mutex, even on panic
	}()
	err := readiness(ctx, func(custody DirectoryCustodian) error {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		if !active || called {
			repeated = true
			return ErrPairRecoveryRequired
		}
		called = true
		bindErr = t.rebindReadiness(ctx, custody, paths, ids, fileIDs)
		return bindErr
	})
	callbackMu.Lock()
	active = false
	bound, duplicate, bindingErr := called, repeated, bindErr
	callbackMu.Unlock()
	if !bound || duplicate || bindingErr != nil || err != nil {
		return errors.Join(ErrPairRecoveryRequired, bindingErr, err)
	}
	return t.gate(ctx, authorize) // actual fresh SDK/clock/intent after child and hashes
}

func (t *PairTransaction) rebindReadiness(ctx context.Context, custody DirectoryCustodian, paths []string, ids, fileIDs map[string]os.FileInfo) error {
	capable, ok := custody.(CreationCustodian)
	if !ok || custody == nil || ctx == nil || ctx.Err() != nil || custody.Validate(ctx, t.root) != nil {
		return ErrPairRecoveryRequired
	}
	root, err := pairRoot(t.root)
	if err != nil || !samePairFile(root, t.rootInfo) {
		return ErrPairRecoveryRequired
	}
	for _, path := range paths {
		if err := custody.PinDirectory(ctx, path); err != nil {
			return err
		}
		current, err := pairDirectory(path)
		if err != nil || !samePairFile(current, ids[path]) {
			return ErrPairRecoveryRequired
		}
	}
	for rel, original := range fileIDs {
		current, err := pairMetadataSnapshot(filepath.Join(t.dir, rel), false)
		if err != nil || !samePairFile(current, original) || current.Size() != original.Size() || !current.ModTime().Equal(original.ModTime()) {
			return ErrPairRecoveryRequired
		}
	}
	companion, err := pairMetadataSnapshot(filepath.Join(t.root, RecoveryApplicationFile), false)
	if err != nil || t.recoveryInfo == nil || !samePairFile(companion, t.recoveryInfo) {
		return ErrPairRecoveryRequired
	}
	t.custody, t.payloadCustody = custody, capable // original private record/creation IDs stay in THIS transaction
	t.creationCustody = capable
	if t.identity(ctx) != nil || t.control() != nil {
		return ErrPairRecoveryRequired
	}
	if !t.forwardOnly {
		return t.verifyOldBackups(ctx) // original private old fingerprints, never journal-minted ones
	}
	return nil
}
