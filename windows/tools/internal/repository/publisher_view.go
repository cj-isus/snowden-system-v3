package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

var errNoPublishedRelease = errors.New("repository: no locally accepted publication")
var errPublicationExpired = errors.New("repository: accepted publication expired")
var errPublicationPolicySuperseded = errors.New("repository: publication pin policy superseded")

// acceptedView is an owned read of an already accepted local publication.
// Its authority is the protected publisher checkpoint, not a self-hash found
// on public disk. It never enrolls a root or substitutes for client acceptance.
type acceptedView struct {
	head         publisherHead
	current      []byte
	stage        *stagedPublication
	pinPolicySHA string
}

func readAcceptedView(ctx context.Context, store *publisherStore, source *publisherSnapshot) (*acceptedView, error) {
	if ctx == nil || source == nil || source.state == nil || source.state.Published == nil {
		return nil, errNoPublishedRelease
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	head := *source.state.Published
	return readAcceptedHeadView(ctx, store, head)
}

// The caller selects this value from private retained state, never from an HTTP
// caller or a discovered candidate frame on public disk.
func readAcceptedHeadView(ctx context.Context, store *publisherStore, head publisherHead) (*acceptedView, error) {
	if ctx == nil || store == nil {
		return nil, errNoPublishedRelease
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	filename := filepath.Join(store.root, "public", filepath.FromSlash(head.Directory), "candidates", head.CandidateSHA256+".json")
	if err := publisherPath(filename); err != nil {
		return nil, err
	}
	raw, err := publisherfiles.Read(filename, delivery.MaxEncodedPackageBytes)
	if err != nil {
		return nil, err
	}
	view, err := decodeAcceptedHead(raw, head)
	if err != nil {
		return nil, err
	}
	if err = view.stage.check(store, ctx.Err); err != nil {
		return nil, err
	}
	return view, nil
}

// Pure checkpoint-bound decoder shared by disk serving and approved backup
// restoration. It cannot discover or assign a publication signing authority.
func decodeAcceptedHead(raw []byte, head publisherHead) (*acceptedView, error) {
	wire, err := decodeCandidate(raw, head.CandidateSHA256)
	if err != nil {
		return nil, err
	}
	if wire.ReleaseID != head.ReleaseID || wire.Manifest.Purpose != head.Purpose || wire.Manifest.PublishedRootSHA256 != head.RootSHA256 || !wire.Manifest.ValidUntil.Equal(head.ValidUntil) || digest(wire.Files["metadata/root.json"]) != head.RootSHA256 || digest(wire.Files["metadata/timestamp.json"]) != head.TimestampSHA256 {
		return nil, errors.New("repository: immutable candidate differs from retained publication")
	}
	// These bytes derive from the exact privately accepted candidate, not from
	// discovered public files. This is historical shape/identity, not fresh proof.
	pin, err := restorePinPolicy(restoreCandidate(wire), wire)
	if err != nil {
		return nil, err
	}
	pinSHA := ""
	if len(pin) != 0 {
		if _, err = metadata.DecodePinPolicy(pin); err != nil {
			return nil, err
		}
		pinSHA = digest(pin)
	}
	stage := &stagedPublication{files: wire.Files, raw: raw, dir: head.Directory}
	current, err := json.Marshal(head)
	if err != nil {
		return nil, err
	}
	return &acceptedView{head: head, current: current, stage: stage, pinPolicySHA: pinSHA}, nil
}

func checkPublicationPinPolicy(state *publisherState, candidateSHA string) error {
	if state == nil {
		return delivery.ErrTrust
	}
	// Separately retained authenticated metadata can announce a new policy
	// before final publication succeeds. Do not continue serving the old
	// application release during that pending obligation, even from cache.
	if err := metadata.CheckAppliedPinPolicy(state.Delivery, state.PinPolicy); err != nil {
		return err
	}
	if state.PinPolicy != nil && state.PinPolicy.SHA256 != candidateSHA {
		return errPublicationPolicySuperseded
	}
	return nil
}

func (v *acceptedView) checkTime(floor, now time.Time) error {
	if v == nil {
		return errNoPublishedRelease
	}
	return checkPublicationTime(v.head.ValidUntil, floor, now)
}

func checkPublicationTime(deadline, floor, now time.Time) error {
	if err := metadata.CheckClockFloor(now, floor); err != nil {
		return err
	}
	if now.Before(floor) {
		now = floor
	}
	if !now.Before(deadline) {
		return errPublicationExpired
	}
	return nil
}

type publisherStatusReport struct {
	Schema               int            `json:"schema"`
	Operation            string         `json:"operation"`
	State                string         `json:"state"`
	RetainedStateSHA256  string         `json:"retained_state_sha256,omitempty"`
	Publication          *publisherHead `json:"publication,omitempty"`
	LocalIntegrity       string         `json:"local_integrity"`
	CanServeCurrent      bool           `json:"can_serve_current"`
	ClientAcceptance     string         `json:"client_acceptance"`
	ExternalReachability string         `json:"external_reachability"`
	ArchivedReleases     int            `json:"retained_archive_entries"`
	RetainedRoots        int            `json:"retained_root_entries"`
	ServingFloor         *time.Time     `json:"serving_floor,omitempty"`
	StoragePolicy        string         `json:"storage_policy"`
	PinPolicyVersion     string         `json:"pin_policy_version,omitempty"`
	PinPolicySHA256      string         `json:"pin_policy_sha256,omitempty"`
}

// Status observes local public data only. A zero exit/report is not health:
// consumers must inspect State/CanServeCurrent; external health is not observed.
func statusPublisher(directory string) (report []byte, finalErr error) {
	status := publisherStatusReport{Schema: 1, Operation: "local_repository_status", State: "unavailable", LocalIntegrity: "not_checked", ClientAcceptance: "required", ExternalReachability: "not_observed"}
	status.StoragePolicy = "bounded_public_admission_no_automatic_deletion"
	store, release, err := openExistingPublisher(directory)
	if err != nil {
		return json.Marshal(status)
	}
	defer func() { finalErr = errors.Join(finalErr, release()) }()
	source, err := store.capture()
	if err != nil {
		status.State = "inspection_required"
		if errors.Is(err, filetxn.ErrRecoveryRequired) {
			status.State = "recovery_required"
		}
		return json.Marshal(status)
	}
	if source.state != nil {
		status.RetainedStateSHA256 = digest(source.raw[0])
		status.Publication = source.state.Published
		status.ArchivedReleases, status.RetainedRoots = len(source.state.History), len(source.state.Roots)
		status.ServingFloor = source.state.ServingFloor
		if source.state.PinPolicy != nil {
			status.PinPolicyVersion, status.PinPolicySHA256 = fmt.Sprint(source.state.PinPolicy.Version), source.state.PinPolicy.SHA256
		}
	}
	if err = source.checkAliases(); err != nil {
		status.State, status.LocalIntegrity = "aliases_inconsistent", "failed"
		return json.Marshal(status)
	}
	if source.state == nil {
		status.State, status.LocalIntegrity = "not_enrolled", "controls_consistent"
		return json.Marshal(status)
	}
	if source.state.Published == nil {
		status.State, status.LocalIntegrity = "not_published", "controls_consistent"
		return json.Marshal(status)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clock := &metadata.ObservedClock{}
	if _, err = clock.Now(publisherClockFloor(source.state)); err != nil {
		status.State, status.LocalIntegrity = "clock_uncertain", "controls_consistent"
		return json.Marshal(status)
	}
	view, err := readAcceptedView(ctx, store, source)
	if err != nil {
		status.State, status.LocalIntegrity = "inventory_unavailable", "not_confirmed"
		if ctx.Err() != nil {
			status.State = "inspection_incomplete"
		}
		return json.Marshal(status)
	}
	if err = store.fence(source); err != nil {
		status.State, status.LocalIntegrity = "source_not_confirmed", "not_confirmed"
		return json.Marshal(status)
	}
	if err = ctx.Err(); err != nil {
		status.State, status.LocalIntegrity = "inspection_incomplete", "not_confirmed"
		return json.Marshal(status)
	}
	status.LocalIntegrity = "checkpoint_and_inventory_consistent"
	if err = checkPublicationPinPolicy(source.state, view.pinPolicySHA); err != nil {
		status.State = "publication_policy_superseded"
		if errors.Is(err, metadata.ErrPinPolicyPending) {
			status.State = "pin_policy_pending"
		}
		return json.Marshal(status)
	}
	now, err := clock.Now(publisherClockFloor(source.state))
	if err == nil {
		err = view.checkTime(publisherClockFloor(source.state), now)
	}
	if err != nil {
		status.State = "clock_uncertain"
		if errors.Is(err, errPublicationExpired) {
			status.State = "publication_expired"
		}
		return json.Marshal(status)
	}
	status.State, status.CanServeCurrent = "locally_publishable", true
	return json.Marshal(status)
}
