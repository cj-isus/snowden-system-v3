package update

import (
	"context"
	"errors"
	"io"
	"path/filepath"
)

// Apply requires the actual source/time/intent guard supplied by the platform.
// Nonnil alone proves nothing. The caller retains its real lifecycle owner and
// serializer; no process launch/floor promotion/barrier removal occurs here.
// A nil result means files placed, with the pending barrier still installed.
// Readiness is a required typed argument; nil fails before mutation.
func (t *PairTransaction) Apply(ctx context.Context, authorize func(commit func() error) error, readiness PairReadiness) (err error) {
	if readiness == nil {
		return ErrPairRecoveryRequired
	}
	if !t.mu.TryLock() {
		return ErrPairBusy
	}
	defer t.mu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrPairRecoveryRequired)
		}
	}()
	err = t.apply(ctx, authorize, readiness)
	completed = true
	return err
}

func (t *PairTransaction) gate(ctx context.Context, authorize func(commit func() error) error) error {
	return t.commit(ctx, authorize, func() error { return nil })
}

// Source authority encloses the short mutation, after potentially long hashes.
// The caller must invoke the supplied commit synchronously exactly once.
func (t *PairTransaction) commit(ctx context.Context, authorize func(commit func() error) error, mutate func() error) error {
	if ctx == nil || ctx.Err() != nil || authorize == nil || t.identity(ctx) != nil {
		return ErrPairRecoveryRequired
	}
	called := false
	var mutationErr error
	err := authorize(func() error {
		if called {
			mutationErr = errors.Join(mutationErr, ErrPairRecoveryRequired)
			return mutationErr
		}
		called = true
		if ctx.Err() != nil {
			mutationErr = ctx.Err()
		} else if mutationErr = t.identity(ctx); mutationErr == nil {
			if ctx.Err() != nil {
				mutationErr = ctx.Err() // repeat after potentially stalled directory reads
			} else {
				mutationErr = mutate()
			}
		}
		return mutationErr
	})
	if !called {
		return errors.Join(ErrPairRecoveryRequired, err)
	}
	return errors.Join(err, mutationErr)
}

func (t *PairTransaction) apply(ctx context.Context, authorize func(commit func() error) error, readiness PairReadiness) error {
	if !t.prepared || t.attempted || t.applied || t.rolledBack {
		return ErrPairRecoveryRequired
	}
	if err := t.retireWrites(); err != nil {
		return err
	}
	if err := t.gate(ctx, authorize); err != nil {
		return err
	}
	if err := t.control(); err != nil {
		return err
	}
	if err := t.verifyOldBackups(ctx); err != nil {
		return err
	}
	if err := t.manifest.VerifyPayloadSet(filepath.Join(t.dir, "new")); err != nil {
		return err
	}
	if err := t.beginIntent(ctx, func(commit func() error) error {
		return t.commit(ctx, authorize, commit)
	}); err != nil {
		return err
	}
	// The durable restriction precedes every installed-name/control mutation.
	// Flush can stall: the next commit must freshly recheck the actual guard.
	// Atomic reservation, not rename-over an existing recovery directory. A
	// crash after this point leaves a barrier even when publication is partial.
	pending := filepath.Join(t.root, pairPendingName)
	if err := t.commit(ctx, authorize, func() error {
		created, err := t.creationCustody.CreateDirectory(ctx, pending)
		if created {
			t.published, t.attempted = true, true // preserve the barrier even on a partial native result
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
		destination := filepath.Join(pending, "data")
		if !pairAbsent(destination) {
			return ErrPairRecoveryRequired
		}
		moved, err := t.custody.MoveDirectory(ctx, t.dir, destination)
		if moved {
			t.dir = destination // actual original-handle location, including uncertain native result
		}
		if err != nil || !moved {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		return t.identity(ctx)
	}); err != nil {
		return err
	}
	if err := t.identity(ctx); err != nil {
		return err
	}
	if err := t.marker(ctx, "prepared"); err != nil {
		return err
	}
	if err := t.control(); err != nil {
		return err
	}
	// Helper first; no worker is allowed until the whole journal is accepted.
	if err := t.prepareRecoveryEntrypoint(ctx, authorize, readiness); err != nil {
		return err // main/helper names must remain unchanged without actual companion readiness
	}
	for _, index := range []int{1, 0} {
		if err := t.gate(ctx, authorize); err != nil {
			return err
		}
		if err := t.place(ctx, authorize, index); err != nil {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		name := "helper-placed"
		if index == 0 {
			name = "app-placed"
		}
		if err := t.marker(ctx, name); err != nil {
			return err
		}
	}
	for index, payload := range t.next {
		if _, err := t.pinPayload(ctx, filepath.Join(t.root, pairName(index)), payload); err != nil {
			return err
		}
	}
	if err := t.control(); err != nil {
		return err
	}
	if err := t.commit(ctx, authorize, func() error { return t.marker(ctx, "applied") }); err != nil {
		return err
	}
	t.applied = true
	return nil
}

func (t *PairTransaction) place(ctx context.Context, authorize func(commit func() error) error, index int) error {
	name := pairName(index)
	target := filepath.Join(t.root, name)
	old := t.old(index)
	var previous PayloadFile
	if old.Present {
		var err error
		previous, err = t.pinPayload(ctx, target, old.payload(index))
		if err != nil {
			return err
		}
	} else if !pairAbsent(target) {
		return ErrPairRecoveryRequired
	}
	newPath := filepath.Join(t.dir, "new", name)
	next, err := t.pinPayload(ctx, newPath, t.next[index])
	if err != nil {
		return err
	}
	displaced := filepath.Join(t.dir, "displaced", name)
	if !pairAbsent(displaced) {
		return ErrPairRecoveryRequired
	}
	if err := t.commit(ctx, authorize, func() error {
		if !pairAbsent(displaced) {
			return ErrPairRecoveryRequired
		}
		t.namesTouched = true // also retain on an uncertain rename result
		if old.Present {
			if moved, err := previous.Move(ctx, displaced); err != nil || !moved {
				return errors.Join(ErrPairRecoveryRequired, err)
			}
		}
		if !pairAbsent(target) || ctx.Err() != nil || t.identity(ctx) != nil {
			return ErrPairRecoveryRequired
		}
		if moved, err := next.Move(ctx, target); err != nil || !moved {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return next.Validate(ctx)
}

// Rollback is available only on the original private transaction, using its
// owned preinstall fingerprints. Public LoadPairCandidate cannot mint it.
// Restored bytes do not authorize execution: the persistent barrier remains.
func (t *PairTransaction) Rollback(ctx context.Context) (err error) {
	if !t.mu.TryLock() {
		return ErrPairBusy
	}
	defer t.mu.Unlock()
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrPairRecoveryRequired)
		}
	}()
	err = t.rollback(ctx)
	completed = true
	return err
}

func (t *PairTransaction) rollback(ctx context.Context) error {
	if t.forwardOnly || ctx == nil || ctx.Err() != nil || !t.published || t.identity(ctx) != nil {
		return ErrPairRecoveryRequired
	}
	if err := t.retireWrites(); err != nil {
		return err
	}
	if err := t.verifyOldBackups(ctx); err != nil {
		return err
	}
	for _, index := range []int{0, 1} {
		if ctx.Err() != nil || t.identity(ctx) != nil {
			return ErrPairRecoveryRequired
		}
		name, old := pairName(index), t.old(index)
		target := filepath.Join(t.root, name)
		if !old.Present && pairAbsent(target) {
			continue
		}
		if !pairAbsent(target) {
			allowed := []Payload{t.next[index]}
			if old.Present {
				allowed = append(allowed, old.payload(index))
			}
			current, err := t.pinPayload(ctx, target, allowed...)
			if err != nil {
				return ErrPairRecoveryRequired // never delete or replace unknown installed bytes
			}
			if old.Present && current.Payload() == old.payload(index) {
				continue
			}
			rejected := filepath.Join(t.dir, "rejected", name)
			if !pairAbsent(rejected) {
				return ErrPairRecoveryRequired
			}
			if ctx.Err() != nil || t.identity(ctx) != nil {
				return ErrPairRecoveryRequired
			}
			if moved, err := current.Move(ctx, rejected); err != nil || !moved {
				return errors.Join(ErrPairRecoveryRequired, err)
			}
		}
		if old.Present {
			if err := t.restore(ctx, index); err != nil {
				return err
			}
		}
	}
	for index := range t.next {
		old, target := t.old(index), filepath.Join(t.root, pairName(index))
		if ctx.Err() != nil || !old.Present && !pairAbsent(target) {
			return ErrPairRecoveryRequired
		}
		if old.Present {
			if _, err := t.pinPayload(ctx, target, old.payload(index)); err != nil {
				return err
			}
		}
	}
	if !t.rolledBack {
		if err := t.marker(ctx, "rolled-back"); err != nil {
			return err
		}
	}
	t.rolledBack = true
	return nil
}

func (t *PairTransaction) restore(ctx context.Context, index int) error {
	name, old := pairName(index), t.old(index).payload(index)
	target := filepath.Join(t.root, name)
	if !pairAbsent(target) {
		return ErrPairRecoveryRequired
	}
	displaced := filepath.Join(t.dir, "displaced", name)
	if !pairAbsent(displaced) {
		original, err := t.pinPayload(ctx, displaced, old)
		if err != nil {
			return err // unknown displaced file is retained, never overwritten or bypassed
		}
		if ctx.Err() != nil || t.identity(ctx) != nil || !pairAbsent(target) {
			return ErrPairRecoveryRequired
		}
		if moved, err := original.Move(ctx, target); err != nil || !moved {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		return original.Validate(ctx)
	}
	restore := filepath.Join(t.dir, "restore", name)
	if pairAbsent(restore) {
		backup, err := t.pinPayload(ctx, filepath.Join(t.dir, "old", name), old)
		if err != nil {
			return err
		}
		if err := t.writeNew(ctx, filepath.Join("restore", name), func(dst io.Writer) error {
			return backup.CopyTo(ctx, dst)
		}); err != nil {
			return err
		}
	}
	resource := t.writes[filepath.Join("restore", name)]
	if resource == nil || !resource.complete {
		return ErrPairRecoveryRequired // only this owner's fully synced/closed restoration may be moved
	}
	restored, err := t.pinPayload(ctx, restore, old)
	if err != nil {
		return err
	}
	if ctx.Err() != nil || !pairAbsent(target) || t.identity(ctx) != nil {
		return ErrPairRecoveryRequired
	}
	if moved, err := restored.Move(ctx, target); err != nil || !moved {
		return errors.Join(ErrPairRecoveryRequired, err)
	}
	return restored.Validate(ctx)
}
