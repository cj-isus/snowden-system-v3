package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

const maxPublisherHistory = 16
const maxPublisherRoots = 128 // exhaustion refuses advancement; never evicts trust roots

type publisherRoot struct {
	Version int64  `json:"version"`
	SHA256  string `json:"sha256"`
}

func validatePublisherHistoryFields(fields map[string]json.RawMessage) error {
	for name, bound := range map[string]int{"history": maxPublisherHistory, "roots": maxPublisherRoots} {
		raw, present := fields[name]
		if !present {
			continue // old checkpoints do not acquire history from public disk
		}
		var entries []json.RawMessage
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &entries) != nil || len(entries) > bound {
			return errors.New("repository: invalid bounded publisher history")
		}
		for _, entry := range entries {
			if name == "history" {
				if _, err := delivery.DecodePublicationHead(entry); err != nil {
					return err
				}
			} else if _, err := exactObject(entry, "version", "sha256"); err != nil {
				return err
			}
		}
	}
	if raw, ok := fields["serving_floor"]; ok {
		var value time.Time
		if json.Unmarshal(raw, &value) != nil || value.IsZero() {
			return metadata.ErrClockUncertain
		}
	}
	return nil
}

func (s *publisherState) checkHistory() error {
	if len(s.History) > maxPublisherHistory || len(s.Roots) > maxPublisherRoots || s.ServingFloor != nil && s.ServingFloor.IsZero() {
		return delivery.ErrBudget
	}
	seen := map[string]bool{}
	if s.Published != nil {
		seen[s.Published.ReleaseID] = true
	}
	for _, head := range s.History {
		if s.Published == nil || seen[head.ReleaseID] {
			return errors.New("repository: duplicate or orphan archived publication")
		}
		seen[head.ReleaseID] = true
	}
	for i, root := range s.Roots {
		if root.Version < 1 || !canonicalHash(root.SHA256) || i > 0 && (s.Roots[i-1].Version >= root.Version || root.Version-s.Roots[i-1].Version != 1) {
			return errors.New("repository: noncontiguous publisher root index")
		}
	}
	if len(s.Roots) != 0 {
		root, err := decodeDocument("root", s.Delivery.Root)
		last := s.Roots[len(s.Roots)-1]
		if err != nil || root.version() != last.Version || digest(s.Delivery.Root) != last.SHA256 {
			return errors.New("repository: root index differs from retained SDK authority")
		}
	}
	return nil
}

func publisherClockFloor(state *publisherState) time.Time {
	if state == nil || state.Delivery == nil {
		return time.Time{}
	}
	floor := state.Delivery.ClockFloor
	if state.ServingFloor != nil && state.ServingFloor.After(floor) {
		floor = *state.ServingFloor
	}
	if state.PinPolicy != nil && state.PinPolicy.IssuedAt.After(floor) {
		floor = state.PinPolicy.IssuedAt
	}
	return floor
}

func archivedPublisherHeads(source *publisherSnapshot, next publisherHead) []publisherHead {
	if source == nil || source.state == nil {
		return nil
	}
	var history []publisherHead
	seen := map[string]bool{next.ReleaseID: true}
	add := func(head publisherHead) {
		if !seen[head.ReleaseID] && len(history) < maxPublisherHistory {
			history = append(history, head)
			seen[head.ReleaseID] = true
		}
	}
	if source.state.Published != nil {
		add(*source.state.Published)
	}
	for _, head := range source.state.History {
		add(head)
	}
	return history // removal from serving index does not remove any disk object
}

func checkPublisherHistoryTransition(source *publisherSnapshot, next *publisherState, promoted *publisherHead) error {
	if next == nil {
		return errors.New("repository: next publisher state required")
	}
	var previousPin *metadata.RetainedPinPolicy
	if source != nil && source.state != nil {
		previousPin = source.state.PinPolicy
	}
	if err := metadata.CheckPinPolicyContinuity(previousPin, next.PinPolicy); err != nil {
		return err
	}
	if promoted == nil {
		a, err := json.Marshal(previousPin)
		if err != nil {
			return err
		}
		b, err := json.Marshal(next.PinPolicy)
		if err != nil || !bytes.Equal(a, b) {
			return errors.New("repository: pin policy may change only with verified promotion")
		}
	}
	if source == nil || source.state == nil {
		return nil
	}
	previous := source.state
	if publisherClockFloor(next).Before(publisherClockFloor(previous)) || len(next.Roots) < len(previous.Roots) {
		return errors.New("repository: publisher time/root history cannot be lowered")
	}
	for i, root := range previous.Roots {
		if next.Roots[i] != root {
			return errors.New("repository: accepted root history cannot be replaced")
		}
	}
	expected := previous.History
	if promoted != nil {
		expected = archivedPublisherHeads(source, *promoted)
	}
	if len(next.History) != len(expected) {
		return errors.New("repository: publication archive transition differs")
	}
	for i, head := range expected {
		a, err := json.Marshal(head)
		if err != nil {
			return err
		}
		b, err := json.Marshal(next.History[i])
		if err != nil || !bytes.Equal(a, b) {
			return errors.New("repository: publication archive may change only with promotion")
		}
	}
	return nil
}

// Walk forward from the exact enrolled/retained root, not backward from the
// candidate's self-signed inventory. Each step requires both thresholds and the
// endpoint must equal the SDK's private NextState.Root byte for byte.
func (p *promotionProgress) trustedRootFiles() (map[string][]byte, []publisherRoot, error) {
	if p == nil || p.next == nil {
		return nil, nil, delivery.ErrTrust
	}
	first, err := decodeDocument("root", p.rootSource)
	if err != nil {
		return nil, nil, err
	}
	last, err := decodeDocument("root", p.next.Root)
	if err != nil || last.version() < first.version() || last.version()-first.version() > delivery.MaxRootRotations {
		return nil, nil, delivery.ErrTrust
	}
	files := map[string][]byte{}
	var roots []publisherRoot
	current, raw := first, p.rootSource
	for {
		if err = current.verify(current); err != nil {
			return nil, nil, err
		}
		name := fmt.Sprintf("metadata/%d.root.json", current.version())
		files[name] = bytes.Clone(raw)
		roots = append(roots, publisherRoot{Version: current.version(), SHA256: digest(raw)})
		if current.version() == last.version() {
			if !bytes.Equal(raw, p.next.Root) {
				return nil, nil, errors.New("repository: root chain endpoint differs from SDK result")
			}
			return files, roots, nil
		}
		raw = p.rootRecords[fmt.Sprintf("%d.root.json", current.version()+1)]
		next, err := decodeDocument("root", raw)
		if err != nil || next.version() != current.version()+1 {
			return nil, nil, errors.New("repository: authenticated forward root chain incomplete")
		}
		if err = current.verify(next); err != nil {
			return nil, nil, err
		}
		current = next
	}
}

func (s *publisherStore) stageRoots(ctx context.Context, source *publisherSnapshot, progress *promotionProgress) ([]publisherRoot, error) {
	files, chain, err := progress.trustedRootFiles()
	if err != nil {
		return nil, err
	}
	var roots []publisherRoot
	if source.state != nil {
		roots = append(roots, source.state.Roots...)
	}
	for _, root := range chain {
		if len(roots) != 0 && roots[len(roots)-1].Version == root.Version {
			if roots[len(roots)-1].SHA256 != root.SHA256 {
				return nil, errors.New("repository: root history equivocation")
			}
			continue
		}
		if len(roots) != 0 && root.Version-roots[len(roots)-1].Version != 1 || len(roots) >= maxPublisherRoots {
			return nil, errors.New("repository: root retention capacity or continuity requires explicit migration")
		}
		roots = append(roots, root)
	}
	pending := map[string][]byte{}
	for name, raw := range files {
		pending[filepath.Join(s.root, "public", filepath.FromSlash(name))] = raw
	}
	if err = s.admitPublicStorage(ctx, pending); err != nil {
		return nil, err
	}
	for filename, raw := range pending {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if err = writeImmutable(filename, raw); err != nil {
			return nil, err
		}
	}
	return roots, ctx.Err()
}

// Called while holding the existing writer lease. The journal preserves the
// SDK state, every accepted head/root and aliases. Failures deny the response;
// reads never auto-recover a journal. At most one ordinary write per minute.
func (s *publisherStore) observeServingTime(ctx context.Context, source *publisherSnapshot, observed time.Time, gate func() error) (*publisherSnapshot, error) {
	if source == nil || source.state == nil || observed.IsZero() || gate == nil || metadata.CheckClockFloor(observed, publisherClockFloor(source.state)) != nil {
		return nil, metadata.ErrClockUncertain
	}
	previous := source.state.ServingFloor
	if previous != nil && observed.Sub(*previous) < metadata.ClockCheckpointInterval {
		return source, nil
	}
	next := *source.state
	value := observed.UTC()
	if value.Before(publisherClockFloor(source.state)) {
		value = publisherClockFloor(source.state)
	}
	next.ServingFloor = &value
	if err := s.commit(source, &next, nil, nil, nil, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return gate()
	}); err != nil {
		return nil, err
	}
	return s.read()
}

func (s *publisherStore) readIndexedRoot(ctx context.Context, state *publisherState, version int64) ([]byte, error) {
	for _, root := range state.Roots {
		if root.Version != version {
			continue
		}
		filename := filepath.Join(s.root, "public", "metadata", fmt.Sprintf("%d.root.json", version))
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := publisherPath(filename); err != nil {
			return nil, err
		}
		raw, err := publisherfiles.Read(filename, delivery.MaxMetadataBytes)
		if err != nil {
			return nil, err
		}
		doc, err := decodeDocument("root", raw)
		if err != nil || doc.version() != version || digest(raw) != root.SHA256 {
			return nil, errors.New("repository: retained public root changed")
		}
		return raw, ctx.Err()
	}
	return nil, errPublisherRootNotRetained
}
