package delivery

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

// This comparison is tied to go-tuf v2.4.2's public parsed objects. Its verifier
// does not mutate them; TargetsType.UnmarshalJSON populates Path on both sides.
// Compare signatures and unknown fields too. No re-marshaled bytes are retained
// or substituted into a hash/signature check, and this is not verification.
func matchesMetadata[T tuf.Roles](raw []byte, accepted *tuf.Metadata[T]) bool {
	if len(raw) == 0 || accepted == nil {
		return false
	}
	parsed, err := new(tuf.Metadata[T]).FromBytes(raw)
	return err == nil && reflect.DeepEqual(parsed, accepted)
}

// Only the protected input cache and successful fetch deliveries are candidates.
// Merely including a record in a package never makes it a verified prefix.
func boundMetadata[T tuf.Roles](name string, previous *State, fetcher *packageFetcher, accepted *tuf.Metadata[T]) ([]byte, error) {
	var selected []byte
	consider := func(raw []byte) error {
		if !matchesMetadata(raw, accepted) {
			return nil
		}
		if selected != nil && !bytes.Equal(selected, raw) {
			// Different serializations can have identical parsed objects. Without
			// an upstream raw-byte receipt, choosing either would be a guess.
			return fmt.Errorf("%w: ambiguous original metadata bytes", ErrTrust)
		}
		selected = raw
		return nil
	}
	if err := consider(previous.Cache[name]); err != nil {
		return nil, err
	}
	for delivered := range fetcher.delivered {
		if delivered != name && !strings.HasSuffix(delivered, "."+name) {
			continue
		}
		if err := consider(fetcher.pack.Metadata[delivered]); err != nil {
			return nil, err
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("%w: accepted metadata has no original input", ErrTrust)
	}
	return bytes.Clone(selected), nil
}

// The SDK accepts roots sequentially before exposing its current Root. Inspect
// that accepted prefix, including intermediate roots, for online key changes.
// This never verifies a root or advances the SDK's workflow independently.
func retainedRoot(previous *State, fetcher *packageFetcher, accepted *tuf.Metadata[tuf.RootType]) ([]byte, bool, error) {
	if err := rootPolicy(accepted); err != nil {
		return nil, false, err
	}
	old, err := tuf.Root().FromBytes(previous.Root)
	if err != nil || accepted.Signed.Version < old.Signed.Version || accepted.Signed.Version-old.Signed.Version > MaxRootRotations {
		return nil, false, ErrTrust
	}
	current, raw, reset := old, previous.Root, false
	for version := old.Signed.Version + 1; version <= accepted.Signed.Version; version++ {
		name := strconv.FormatInt(version, 10) + ".root.json"
		if !fetcher.delivered[name] {
			return nil, false, ErrTrust
		}
		candidate := fetcher.pack.Metadata[name]
		next, err := tuf.Root().FromBytes(candidate)
		if err != nil || rootPolicy(next) != nil || next.Signed.Version != version {
			return nil, false, ErrTrust
		}
		for _, role := range []string{tuf.TIMESTAMP, tuf.SNAPSHOT} {
			before, after := current.Signed.Roles[role], next.Signed.Roles[role]
			oldKeys, err := roleKeys(current.Signed.Keys, before.KeyIDs, before.Threshold, 1)
			if err != nil {
				return nil, false, err
			}
			newKeys, err := roleKeys(next.Signed.Keys, after.KeyIDs, after.Threshold, 1)
			if err != nil {
				return nil, false, err
			}
			// A threshold or key-ID alias change alone cannot erase floors.
			reset = reset || !reflect.DeepEqual(oldKeys, newKeys)
		}
		current, raw = next, candidate
	}
	if !matchesMetadata(raw, accepted) {
		return nil, false, ErrTrust
	}
	return bytes.Clone(raw), reset, nil
}
