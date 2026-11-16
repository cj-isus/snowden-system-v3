package delivery

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	updater "github.com/snowden-system/windows/backend/delivery/internal/tufupdater"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

// Keep each eligible successful-return prefix even when a later role/write
// fails. The set MUST come from the updater's per-role success receipts, never
// GetTrustedMetadataSet: timestamp/snapshot pointers can precede final rejection.
// Old accepted expired input remains history; it need not earn a fresh receipt.
func retainedState(previous *State, fetcher *packageFetcher, trusted updater.SuccessfulMetadataSet, now time.Time) (next *State, finalErr error) {
	next = previous.Clone()
	next.ClockFloor = now.UTC()
	defer func() {
		if err := next.Validate(); err != nil {
			next = nil
			finalErr = errors.Join(finalErr, err)
		}
	}()
	root, reset, err := retainedRoot(previous, fetcher, trusted.Root)
	if err != nil {
		return next, err
	}
	next.Root = root
	if reset {
		// TUF 5.3.11: only an SDK-accepted online key rotation grants this
		// reset. Keep it atomic with the root, even if the tail is unavailable.
		delete(next.Cache, "timestamp.json")
		delete(next.Cache, "snapshot.json")
	}
	if trusted.Timestamp == nil {
		return next, nil
	}
	if !trusted.Root.Signed.Expires.After(now) || !trusted.Timestamp.Signed.Expires.After(now) {
		return next, fmt.Errorf("%w: retained timestamp predecessor/expiry rejected", ErrTrust)
	}
	stamp, err := boundMetadata("timestamp.json", previous, fetcher, trusted.Timestamp)
	if err != nil {
		return next, err
	}
	if reset && bytes.Equal(stamp, previous.Cache["timestamp.json"]) {
		return next, onlineRotationContinuation()
	}
	if !reset {
		if err := timestampContinuity(previous.Cache["timestamp.json"], trusted.Timestamp); err != nil {
			return next, err
		}
	}
	next.Cache["timestamp.json"] = stamp
	if trusted.Snapshot == nil {
		return next, nil
	}
	if !trusted.Snapshot.Signed.Expires.After(now) {
		return next, fmt.Errorf("%w: retained snapshot expiry rejected", ErrTrust)
	}
	snapshot, err := boundMetadata("snapshot.json", previous, fetcher, trusted.Snapshot)
	if err != nil {
		return next, err
	}
	if reset && bytes.Equal(snapshot, previous.Cache["snapshot.json"]) {
		return next, onlineRotationContinuation()
	}
	// A successful isTrusted cache load skips this byte-link check upstream.
	// Keep the existing complete-chain check too: neither receipt nor pointer
	// alone binds an original cached snapshot to the current timestamp.
	if err := retainedRoleLink("snapshot.json", snapshot, trusted.Snapshot.Signed.Version, trusted.Timestamp.Signed.Meta["snapshot.json"]); err != nil {
		return next, err
	}
	if !reset {
		if err := snapshotContinuity(previous.Cache["snapshot.json"], trusted.Snapshot); err != nil {
			return next, err
		}
	}
	next.Cache["snapshot.json"] = snapshot
	for _, role := range []string{tuf.TARGETS, "profiles", "updates"} {
		meta := trusted.Targets[role]
		if meta == nil {
			continue // an unvisited role is not permission to erase its history
		}
		if !meta.Signed.Expires.After(now) {
			return next, fmt.Errorf("%w: retained targets expiry rejected", ErrTrust)
		}
		data, err := boundMetadata(role+".json", previous, fetcher, meta)
		if err != nil {
			return next, err
		}
		if err := retainedRoleLink(role+".json", data, meta.Signed.Version, trusted.Snapshot.Signed.Meta[role+".json"]); err != nil {
			return next, err
		}
		next.Cache[role+".json"] = data
	}
	return next, nil
}

func retainedRoleLink(name string, raw []byte, version int64, link *tuf.MetaFiles) error {
	if link == nil || link.Version != version || link.VerifyLengthHashes(raw) != nil {
		return fmt.Errorf("%w: retained %s link mismatch", ErrTrust, name)
	}
	return nil
}

func onlineRotationContinuation() error {
	// The SDK may have loaded the pre-rotation seed while keys overlapped.
	// Persist only the new root/fresh prefix, then start a bounded fresh pass.
	return fmt.Errorf("%w: online key rotation continuation required", ErrTrust)
}

// Artifact proofs require a complete exact-byte chain, including cached
// snapshots for which upstream's isTrusted path skips length/hash checks.
// Reuse SDK link verification; expiry/signatures/delegation remain its workflow
// and finalPolicy's responsibility. Prefix-only progress never calls this proof.
func completeRetainedState(next *State, trusted trustedmetadata.TrustedMetadata) error {
	if next == nil || trusted.Timestamp == nil || trusted.Snapshot == nil || trusted.Targets[tuf.TARGETS] == nil || len(trusted.Targets) > 3 {
		return ErrTrust
	}
	if !matchesMetadata(next.Root, trusted.Root) || !matchesMetadata(next.Cache["timestamp.json"], trusted.Timestamp) || !matchesMetadata(next.Cache["snapshot.json"], trusted.Snapshot) {
		return ErrTrust
	}
	link := trusted.Timestamp.Signed.Meta["snapshot.json"]
	if link == nil || link.Version != trusted.Snapshot.Signed.Version || link.VerifyLengthHashes(next.Cache["snapshot.json"]) != nil {
		return fmt.Errorf("%w: retained snapshot link mismatch", ErrTrust)
	}
	for role, meta := range trusted.Targets {
		if role != tuf.TARGETS && role != "profiles" && role != "updates" {
			return ErrTrust
		}
		name := role + ".json"
		if !matchesMetadata(next.Cache[name], meta) {
			return ErrTrust
		}
		link := trusted.Snapshot.Signed.Meta[name]
		if link == nil || link.Version != meta.Signed.Version || link.VerifyLengthHashes(next.Cache[name]) != nil {
			return fmt.Errorf("%w: retained targets link mismatch", ErrTrust)
		}
	}
	return nil
}
