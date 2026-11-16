package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

type supplementRoots struct {
	from, to int64
	records  map[string][]byte
}

func (s *publisherStore) captureSupplementRoots(ctx context.Context, source *publisherSnapshot, first int64) (*supplementRoots, error) {
	if source == nil || source.state == nil {
		return nil, delivery.ErrTrust
	}
	current, err := decodeDocument("root", source.state.Delivery.Root)
	if err != nil {
		return nil, err
	}
	if first < 1 || first > current.version() || current.version()-first >= delivery.MaxRootRotations {
		return nil, errors.New("repository: root path exceeds one SDK refresh; explicit staged recovery required")
	}
	out := &supplementRoots{from: first, to: current.version(), records: map[string][]byte{}}
	for version := first; version <= out.to; version++ {
		raw, err := s.readIndexedRoot(ctx, source.state, version)
		if err != nil {
			return nil, err // never fill missing authority from discovered/input roots
		}
		out.records[fmt.Sprintf("%d.root.json", version)] = raw
	}
	return out, nil
}

func (r *supplementRoots) check(ctx context.Context, store *publisherStore, source *publisherSnapshot) error {
	for version := r.from; version <= r.to; version++ {
		raw, err := store.readIndexedRoot(ctx, source.state, version)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, r.records[fmt.Sprintf("%d.root.json", version)]) {
			return errors.New("repository: retained supplementation root changed")
		}
	}
	return ctx.Err()
}

// Explicit issuer operation only. It writes a new candidate outside the store,
// not a current alias or installed profile. First approve the exact old private
// checkpoint; its own authenticated prefix advance is separately durable.
func supplementRelease(candidateFile, candidateSHA, policyFile, policySHA, stateSHA, minimumRoot, directory, output string) (report []byte, finalErr error) {
	first, err := strconv.ParseInt(minimumRoot, 10, 64)
	if err != nil || first < 1 || strconv.FormatInt(first, 10) != minimumRoot || !canonicalHash(stateSHA) {
		return nil, errors.New("repository: exact checkpoint approval and canonical first root version required")
	}
	store, release, err := openExistingPublisher(directory)
	if err != nil {
		return nil, err
	}
	written := false
	defer func() {
		if err := release(); err != nil {
			if written {
				err = errors.Join(errors.New("repository: new candidate written; writer release failed"), err)
			}
			finalErr = errors.Join(finalErr, err)
		}
	}()
	source, err := store.read()
	if err != nil {
		return nil, err
	}
	if source.state == nil || digest(source.raw[0]) != stateSHA {
		return nil, errors.New("repository: approved existing publisher checkpoint required")
	}
	output, err = outsidePublisherPath(output, store)
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(errors.New("repository: new absent candidate output required"), err)
	}
	candidate, err := readPromotionInput(candidateFile, candidateSHA, delivery.MaxEncodedPackageBytes, store)
	if err != nil {
		return nil, err
	}
	authority, err := readPromotionInput(policyFile, policySHA, metadata.MaxTrustPolicySize, store)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	clock := &metadata.ObservedClock{}
	prepared, err := store.preparePublisherRelease(ctx, source, candidate, authority, "retained", clock)
	prefixRetained := prepared != nil && prepared.prefixRetained
	defer func() {
		if finalErr != nil && prefixRetained {
			finalErr = errors.Join(errors.New("repository: candidate not published; verified trust prefix retained separately"), finalErr)
		}
	}()
	if err != nil {
		return nil, err
	}
	source = prepared.source
	verified := prepared.progress.accepted
	roots, err := store.captureSupplementRoots(ctx, source, first)
	if err != nil {
		return nil, err
	}
	c, err := supplementCandidate(&verified.candidate.wire, roots.records)
	if err != nil {
		return nil, err
	}
	now, err := clock.Now(publisherClockFloor(source.state))
	if err != nil {
		return nil, err
	}
	if err = c.buildLayout(now); err != nil {
		return nil, err
	}
	raw, err := c.encode()
	if err != nil {
		return nil, err
	}
	approval := digest(raw)
	current, err := verifyRelease(ctx, raw, approval, source.state.Delivery, "", verified.policy, source.state.PinPolicy, now)
	if err != nil || current == nil || current.accepted == nil {
		return nil, errors.Join(delivery.ErrTrust, err)
	}
	// This ephemeral retained baseline is exclusively for the production
	// candidate gate. It does not enroll/reset any client or clone its history.
	// Every byte of this old root came from the private authenticated index.
	oldRoot := roots.records[fmt.Sprintf("%d.root.json", first)]
	old := &delivery.State{Schema: 1, AnchorSHA256: digest(oldRoot), Root: bytes.Clone(oldRoot), Cache: map[string][]byte{}, ClockFloor: publisherClockFloor(source.state)}
	path, err := verifyRelease(ctx, raw, approval, old, "", verified.policy, source.state.PinPolicy, now)
	if err != nil || path == nil || path.accepted == nil {
		return nil, errors.Join(errors.New("repository: supplemented root path not accepted by SDK"), err)
	}
	gate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := publisherPath(output); err != nil {
			return err
		}
		if err := store.fence(source); err != nil {
			return err
		}
		if err := errors.Join(candidate.check(), authority.check()); err != nil {
			return err
		}
		if err := roots.check(ctx, store, source); err != nil {
			return err
		}
		now, err := clock.Now(publisherClockFloor(source.state))
		if err != nil {
			return err
		}
		for _, proof := range []*verifiedRelease{verified, current.accepted, path.accepted} {
			if err := proof.check(ctx, now); err != nil {
				return err
			}
		}
		return ctx.Err()
	}
	if err = gate(); err != nil {
		return nil, err
	}
	if err = publisherfiles.PublishNewChecked(output, raw, gate); err != nil {
		return nil, err
	}
	written = true
	return json.Marshal(struct {
		Schema                   int    `json:"schema"`
		Operation                string `json:"operation"`
		ReleaseID                string `json:"release_id"`
		CandidateSHA256          string `json:"candidate_sha256"`
		RootPathFrom             int64  `json:"root_path_from_version"`
		RootPathTo               int64  `json:"root_path_to_version"`
		PrefixRetained           bool   `json:"verified_prefix_retained"`
		RequiresPromotion        bool   `json:"requires_promotion"`
		RequiresClientAcceptance bool   `json:"requires_client_acceptance"`
		ServerActivated          bool   `json:"server_activated"`
	}{1, "local_candidate_root_supplementation", c.wire.ReleaseID, approval, roots.from, roots.to, prefixRetained, true, true, false})
}
