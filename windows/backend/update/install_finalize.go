package update

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
)

// FinalizePair accepts only the complete currently installed NEW set. The real
// caller guard must freshly authorize the exact candidate and current code,
// including held native image identity/version/pin and floor policy. This API
// never accepts old journal hashes as code authority or removes backup bytes.
// On success the pending directory is preserved under a fresh archive root.
func FinalizePair(ctx context.Context, root string, rawManifest, rawDelivery []byte, custody DirectoryCustodian, authorize func(commit func() error) error) (archive string, finalErr error) {
	if ctx == nil || authorize == nil || custody == nil || ctx.Err() != nil || len(rawManifest) == 0 || len(rawManifest) > MaxManifestSize || len(rawDelivery) > maxPairDelivery {
		return "", ErrPairRecoveryRequired
	}
	if err := custody.Validate(ctx, root); err != nil {
		return "", err
	}
	creator, capable := custody.(CreationCustodian)
	if !capable {
		return "", ErrPairRecoveryRequired
	}
	rawManifest, rawDelivery = bytes.Clone(rawManifest), bytes.Clone(rawDelivery)
	rootInfo, err := pairRoot(root)
	if err != nil {
		return "", err
	}
	initial, err := LoadPairCandidate(ctx, root, custody)
	if err != nil {
		return "", err
	}
	pending := filepath.Dir(initial.Directory)
	pendingInfo, err := pairDirectory(pending)
	if err != nil {
		return "", err
	}
	check := func() (*PairCandidate, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		currentRoot, err := pairRoot(root)
		if err != nil || !samePairFile(currentRoot, rootInfo) {
			return nil, ErrPairRecoveryRequired
		}
		currentPending, err := pairDirectory(pending)
		if err != nil || !samePairFile(currentPending, pendingInfo) {
			return nil, ErrPairRecoveryRequired
		}
		candidate, err := LoadPairCandidate(ctx, root, custody)
		if err != nil || !bytes.Equal(candidate.RawManifest, rawManifest) || !bytes.Equal(candidate.Delivery, rawDelivery) {
			return nil, ErrPairRecoveryRequired
		}
		if ctx.Err() != nil {
			return nil, ctx.Err() // bounded bytes do not imply bounded filesystem latency
		}
		return candidate, nil
	}
	candidate, err := check()
	if err != nil {
		return "", err
	}
	if err := candidate.Manifest.VerifyPayloadSet(root); err != nil {
		return "", err
	}
	if candidate.archive != "" {
		// A prior move is not acceptance. Reauthenticate current code via the
		// actual callback; completion still has to flush the accepted receipt.
		called, repeated := false, false
		var checkErr error
		err := authorize(func() error {
			if called {
				repeated = true
				return ErrPairRecoveryRequired
			}
			called = true
			_, checkErr = check()
			return checkErr
		})
		if !called || repeated || err != nil || checkErr != nil {
			return candidate.archive, errors.Join(ErrPairRecoveryRequired, err, checkErr)
		}
		return candidate.archive, nil
	}
	state, _, err := readPairState()
	if err != nil || state.IntentID == "" || !state.matches(root, rawManifest, rawDelivery) {
		return "", errors.Join(ErrPairRecoveryRequired, err)
	}
	container, created := "", false
	if state.Archive == "" {
		container, created, err = creator.CreateUniqueDirectory(ctx, ".snowden-update-accepted-")
		if err == nil && !created {
			err = ErrPairRecoveryRequired
		}
	} else {
		container = filepath.Join(root, state.Archive)
		if pairAbsent(container) {
			created, err = creator.CreateDirectory(ctx, container)
			if err == nil && !created {
				err = ErrPairRecoveryRequired
			}
		}
	}
	if err != nil {
		return container, err // keep the native creation/partial path receipt
	}
	var containerInfo os.FileInfo
	moved, locatorAttempted := false, false
	defer func() {
		archive = container // retain a receipt even after uncertain move/cleanup
		if moved || locatorAttempted || !created || state.Archive != "" {
			return
		}
		if err := custody.ReleaseDirectory(container); err != nil {
			finalErr = errors.Join(finalErr, err)
			return
		}
		if containerInfo == nil {
			return // missing original identity is never permission to delete
		}
		current, err := pairDirectory(container)
		if err != nil || !samePairFile(current, containerInfo) {
			finalErr = errors.Join(finalErr, ErrPairRecoveryRequired)
			return
		}
		if err := os.Remove(container); err != nil { // only the exact empty created directory
			finalErr = errors.Join(finalErr, err)
			return
		}
		archive = "" // confirmed empty cleanup; no retained archive resource
	}()
	if err := custody.PinDirectory(ctx, container); err != nil {
		return container, err
	}
	containerInfo, err = pairDirectory(container)
	if err != nil {
		return container, err
	}
	// Persist the fixed recovery locator BEFORE moving pending. This can stall;
	// the next check/authorize must use fresh SDK/clock/image/intent observations.
	locatorAttempted = true
	if err := rememberPairArchive(ctx, root, rawManifest, rawDelivery, filepath.Base(container), authorize); err != nil {
		return container, err
	}
	state.Archive = filepath.Base(container) // retain this recovery container on later failure
	if _, err := check(); err != nil {
		return "", err
	}
	// The App runs this filesystem-only commit inside its actual source/clock
	// guard. A successful callback that never commits cannot remove the barrier.
	called, committed, repeated := false, false, false
	err = authorize(func() error {
		if called {
			repeated = true
			return ErrPairRecoveryRequired
		}
		called = true
		if _, err := check(); err != nil {
			return err
		}
		currentContainer, err := pairDirectory(container)
		if err != nil || !samePairFile(currentContainer, containerInfo) {
			return ErrPairRecoveryRequired
		}
		destination := filepath.Join(container, "transaction")
		if !pairAbsent(destination) || ctx.Err() != nil {
			return ErrPairRecoveryRequired
		}
		var err error
		moved, err = custody.MoveDirectory(ctx, pending, destination)
		if err != nil || !moved {
			return errors.Join(ErrPairRecoveryRequired, err)
		}
		committed = true
		return nil
	})
	if moved {
		archive = container
	}
	if err != nil || !committed || repeated {
		return archive, errors.Join(ErrPairRecoveryRequired, err)
	}
	return archive, nil
}
