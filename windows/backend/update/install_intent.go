package update

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/netprobe"
)

const maxPairStateBytes = 2048

var ErrPairStateUnavailable = errors.New("update: durable pair state unavailable")

// One bounded per-user restriction, not signature/SDK authority. Root is a
// correlation hash; actual root/image custody remains the platform's job.
// Accepted state retains a monotonic floor and payload facts instead of deleting
// the value. Losing a filesystem floor rename cannot then lower this floor.
type pairInstallState struct {
	Schema         int    `json:"schema"`
	Floor          string `json:"floor"`
	IntentID       string `json:"intent_id"`
	Root           string `json:"root"`
	ManifestSHA256 string `json:"manifest_sha256"`
	DeliverySHA256 string `json:"delivery_sha256"`
	Stage          string `json:"stage"`
	Archive        string `json:"archive"`
	Version        string `json:"version"`
	AppSHA256      string `json:"app_sha256"`
	AppSize        int64  `json:"app_size"`
	HelperSHA256   string `json:"helper_sha256"`
	HelperSize     int64  `json:"helper_size"`
}

func pairRootBinding(root string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", ErrPairRecoveryRequired
	}
	return pairDigest([]byte(strings.ToLower(root))), nil
}

func pairStateName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) || len(name) <= len(prefix) || len(name) > len(prefix)+64 {
		return false
	}
	for _, c := range name[len(prefix):] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

func decodePairState(raw []byte) (pairInstallState, error) {
	if len(raw) == 0 { // only the native reader's confirmed missing-value result
		return pairInstallState{Schema: 1, Floor: "0.0.0"}, nil
	}
	var state pairInstallState
	if len(raw) > maxPairStateBytes {
		return state, ErrPairStateUnavailable
	}
	if _, err := exactPairObject(raw, "schema", "floor", "intent_id", "root", "manifest_sha256", "delivery_sha256", "stage", "archive", "version", "app_sha256", "app_size", "helper_sha256", "helper_size"); err != nil {
		return state, ErrPairStateUnavailable
	}
	if json.Unmarshal(raw, &state) != nil || state.Schema != 1 {
		return pairInstallState{}, ErrPairStateUnavailable
	}
	floor, fe := ParseVersion(state.Floor)
	version, ve := ParseVersion(state.Version)
	if fe != nil || ve != nil || floor.String() != state.Floor || version.String() != state.Version || version.Compare(floor) < 0 ||
		!canonicalPayloadSHA(state.Root) || !canonicalPayloadSHA(state.ManifestSHA256) ||
		state.DeliverySHA256 != "" && !canonicalPayloadSHA(state.DeliverySHA256) ||
		!pairStateName(state.Stage, ".snowden-update-stage-") ||
		state.Archive != "" && !pairStateName(state.Archive, ".snowden-update-accepted-") ||
		state.IntentID != "" && !canonicalPayloadSHA(state.IntentID) || state.IntentID == "" && state.Archive == "" ||
		!(savedPayload{true, state.AppSHA256, state.AppSize}).valid(MaxPayloadSize) ||
		!(savedPayload{true, state.HelperSHA256, state.HelperSize}).valid(netprobe.MaxNativeImageBytes) || state.AppSize > MaxPayloadSize-state.HelperSize {
		return pairInstallState{}, ErrPairStateUnavailable
	}
	return state, nil
}

func readPairState() (pairInstallState, []byte, error) {
	raw, err := readPairStateNative()
	if err != nil {
		return pairInstallState{}, nil, errors.Join(ErrPairStateUnavailable, err)
	}
	state, err := decodePairState(raw)
	return state, raw, err
}

// Startup is not a writer's source/floor recheck. Serialize it with the actual
// interprocess writer so a visible Set cannot admit a worker while Flush still
// runs. ReadFloor remains restriction-only and must also work inside the guard.
func readPairStartupState() (state pairInstallState, finalErr error) {
	release, err := lockPairStartup()
	if release != nil {
		defer func() { finalErr = errors.Join(finalErr, release()) }()
	}
	if err != nil {
		return state, err
	}
	state, _, finalErr = readPairState()
	return state, finalErr
}

type pairStateWrite interface {
	Set() error
	Finish() error
}

// Keep the registry slot free while the actual caller rechecks SDK/clock/floor.
// Only the bounded compare+Set occurs in that guard. Flush/close run after the
// guard returns, with the interprocess writer lock and original key retained.
func changePairState(ctx context.Context, authorize func(func() error) error, change func(*pairInstallState) error) (finalErr error) {
	if ctx == nil || ctx.Err() != nil || authorize == nil || !floorMu.TryLock() {
		return ErrPairStateUnavailable
	}
	defer floorMu.Unlock()
	var writer pairStateWrite
	var release func() error
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrPairStateUnavailable)
		}
		if writer != nil {
			finalErr = errors.Join(finalErr, writer.Finish())
		}
		if release != nil {
			finalErr = errors.Join(finalErr, release())
		}
	}()
	path, err := FloorPath()
	if err != nil {
		return err
	}
	release, err = lockFloor(path + ".lock")
	if err != nil {
		return err
	}
	state, original, err := readPairState()
	if err != nil {
		return err
	}
	floor, err := readFloorFile()
	if err != nil {
		return err // corrupt legacy state must never be replaced by registry zero
	}
	stored, err := ParseVersion(state.Floor)
	if err != nil {
		return err
	}
	if floor.Compare(stored) > 0 {
		state.Floor = floor.String()
	}
	if err := change(&state); err != nil {
		return err
	}
	next, err := json.Marshal(state)
	if err != nil || len(next) == 0 || len(next) > maxPairStateBytes {
		return ErrPairStateUnavailable
	}
	if _, err := decodePairState(next); err != nil {
		return err
	}
	writer = newPairStateWrite(bytes.Clone(original), next)
	called, repeated := false, false
	var mutationErr error
	err = authorize(func() error {
		if called {
			repeated = true
			return ErrPairStateUnavailable
		}
		called = true
		if ctx.Err() != nil {
			mutationErr = ctx.Err()
		} else {
			mutationErr = writer.Set()
		}
		return mutationErr
	})
	completed = true
	if !called || repeated {
		return errors.Join(ErrPairStateUnavailable, err, mutationErr)
	}
	return errors.Join(err, mutationErr)
}

func (t *PairTransaction) beginIntent(ctx context.Context, authorize func(func() error) error) error {
	binding, err := pairRootBinding(t.root)
	if err != nil {
		return err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	version, err := ParseVersion(t.manifest.Version)
	if err != nil {
		return err
	}
	// Set may become visible even when Flush/Close subsequently fails. Retain
	// staging and the logical barrier on every publication attempt, never discard.
	t.intentPending = true
	return changePairState(ctx, authorize, func(state *pairInstallState) error {
		floor, err := ParseVersion(state.Floor)
		if err != nil || state.IntentID != "" || version.Compare(floor) <= 0 {
			return ErrPairRecoveryRequired
		}
		*state = pairInstallState{Schema: 1, Floor: state.Floor, IntentID: hex.EncodeToString(nonce[:]), Root: binding,
			ManifestSHA256: t.record.ManifestSHA256, DeliverySHA256: t.record.DeliverySHA256, Stage: filepath.Base(t.dir), Version: version.String(),
			AppSHA256: t.next[0].SHA256, AppSize: t.next[0].Size, HelperSHA256: t.next[1].SHA256, HelperSize: t.next[1].Size}
		return nil
	})
}

func (state pairInstallState) matches(root string, manifest, delivery []byte) bool {
	binding, err := pairRootBinding(root)
	deliverySHA := ""
	if len(delivery) != 0 {
		deliverySHA = pairDigest(delivery)
	}
	return err == nil && state.Root == binding && state.ManifestSHA256 == pairDigest(manifest) && state.DeliverySHA256 == deliverySHA
}

func rememberPairArchive(ctx context.Context, root string, manifest, delivery []byte, archive string, authorize func(func() error) error) error {
	if !pairStateName(archive, ".snowden-update-accepted-") {
		return ErrPairRecoveryRequired
	}
	return changePairState(ctx, authorize, func(state *pairInstallState) error {
		if state.IntentID == "" || !state.matches(root, manifest, delivery) || state.Archive != "" && state.Archive != archive {
			return ErrPairRecoveryRequired
		}
		state.Archive = archive
		return nil
	})
}

// Reappearance of a filesystem barrier after accepted Set/Flush is still a
// recovery operation. Only the App's fresh SDK guard can reopen that restriction;
// receipt fields never authorize code, floor promotion or old-file restoration.
func ensurePairRecoveryIntent(ctx context.Context, root string, manifest, delivery []byte, archive string, authorize func(func() error) error) error {
	state, _, err := readPairState()
	if err != nil || !state.matches(root, manifest, delivery) {
		return errors.Join(ErrPairRecoveryRequired, err)
	}
	if state.IntentID != "" {
		return nil // actual Apply gate still repeats before every filesystem mutation
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	return changePairState(ctx, authorize, func(current *pairInstallState) error {
		if current.IntentID != "" || !current.matches(root, manifest, delivery) {
			return ErrPairRecoveryRequired
		}
		current.IntentID = hex.EncodeToString(nonce[:])
		current.Archive = ""
		if archive != "" {
			if filepath.Dir(archive) != root || !pairStateName(filepath.Base(archive), ".snowden-update-accepted-") {
				return ErrPairRecoveryRequired
			}
			current.Archive = filepath.Base(archive)
		}
		return nil // retain accepted floor; never lower it or adopt a journal phase
	})
}

// Called with actual current-code SDK/image/renderer authority held by the App.
// No public registry fact authenticates code. The same bounded value commits
// the accepted receipt and floor; a failed flush leaves startup restrictive.
func CompletePairIntent(ctx context.Context, root string, manifest, delivery []byte, archive string, authorize func(func() error) error) error {
	return changePairState(ctx, authorize, func(state *pairInstallState) error {
		if state.IntentID == "" || !state.matches(root, manifest, delivery) || state.Archive != filepath.Base(archive) || filepath.Dir(archive) != root {
			return ErrPairRecoveryRequired
		}
		version, ve := ParseVersion(state.Version)
		floor, fe := ParseVersion(state.Floor)
		if ve != nil || fe != nil || version.Compare(floor) < 0 {
			return ErrFloorUnavailable
		}
		state.Floor, state.IntentID = version.String(), ""
		return nil
	})
}

// Value-only extra startup restrictions; this is not a fresh delivery proof.
type AcceptedPairReceipt struct {
	Version, ManifestSHA256 string
	Application, Helper     Payload
}

func ReadAcceptedPair(root string) (*AcceptedPairReceipt, error) {
	binding, err := pairRootBinding(root)
	if err != nil {
		return nil, err
	}
	state, err := readPairStartupState()
	if err != nil || state.IntentID != "" {
		return nil, errors.Join(ErrPairRecoveryRequired, err)
	}
	if state.Root != binding { // no accepted pair for this initial installation
		return nil, nil
	}
	return &AcceptedPairReceipt{Version: state.Version, ManifestSHA256: state.ManifestSHA256,
		Application: Payload{File: ApplicationFile, SHA256: state.AppSHA256, Size: state.AppSize},
		Helper:      Payload{File: netprobe.NativeHelperName, SHA256: state.HelperSHA256, Size: state.HelperSize}}, nil
}
