package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/netprobe"
)

const pairPendingName = ".snowden-update"
const maxPairRecord = 4096
const maxPairDelivery = delivery.MaxEncodedPackageBytes + 512

var ErrPairRecoveryRequired = errors.New("update: application/helper transaction requires verified recovery")
var ErrPairBusy = errors.New("update: application/helper transaction already owned")

type savedPayload struct {
	Present bool   `json:"present"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type pairRecord struct {
	Schema         int          `json:"schema"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	DeliverySHA256 string       `json:"delivery_sha256"`
	OldApp         savedPayload `json:"old_app"`
	OldHelper      savedPayload `json:"old_helper"`
}

func pairName(index int) string {
	if index == 0 {
		return ApplicationFile
	}
	return netprobe.NativeHelperName
}

func pairDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s savedPayload) valid(limit int64) bool {
	if !s.Present {
		return s.SHA256 == "" && s.Size == 0
	}
	return s.Size > 0 && s.Size <= limit && canonicalPayloadSHA(s.SHA256)
}

func (s savedPayload) payload(index int) Payload {
	return Payload{File: pairName(index), Size: s.Size, SHA256: s.SHA256}
}

func exactPairObject(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	first, err := dec.Token()
	if err != nil || first != json.Delim('{') {
		return nil, ErrPairRecoveryRequired
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	fields := make(map[string]json.RawMessage, len(names))
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] {
			return nil, ErrPairRecoveryRequired
		}
		delete(allowed, name)
		var value json.RawMessage
		if dec.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrPairRecoveryRequired
		}
		fields[name] = value
	}
	last, err := dec.Token()
	if err != nil || last != json.Delim('}') || len(allowed) != 0 || dec.Decode(new(any)) != io.EOF {
		return nil, ErrPairRecoveryRequired
	}
	return fields, nil
}

func decodePairRecord(raw []byte) (pairRecord, error) {
	var record pairRecord
	if len(raw) == 0 || len(raw) > maxPairRecord {
		return record, ErrPairRecoveryRequired
	}
	fields, err := exactPairObject(raw, "schema", "manifest_sha256", "delivery_sha256", "old_app", "old_helper")
	if err != nil {
		return record, err
	}
	for _, name := range []string{"old_app", "old_helper"} {
		if _, err := exactPairObject(fields[name], "present", "sha256", "size"); err != nil {
			return record, err
		}
	}
	if json.Unmarshal(raw, &record) != nil || record.Schema != 1 || !canonicalPayloadSHA(record.ManifestSHA256) ||
		record.DeliverySHA256 != "" && !canonicalPayloadSHA(record.DeliverySHA256) || !record.OldApp.Present || !record.OldApp.valid(MaxPayloadSize) ||
		!record.OldHelper.valid(netprobe.MaxNativeImageBytes) || record.OldApp.Size > MaxPayloadSize-record.OldHelper.Size {
		return pairRecord{}, ErrPairRecoveryRequired
	}
	return record, nil
}

// CheckPairPending is a restriction-only startup barrier. No journal field or
// phase marker can make an unfinished installation accepted. Absence alone is
// not installed-code authentication or a Windows power-loss durability proof.
func CheckPairPending(root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return ErrPairRecoveryRequired
	}
	state, stateErr := readPairStartupState()
	if stateErr != nil || state.IntentID != "" {
		return errors.Join(ErrPairRecoveryRequired, stateErr)
	}
	_, err := os.Lstat(filepath.Join(root, pairPendingName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return errors.Join(ErrPairRecoveryRequired, err)
}

// Public recovery material, NOT a prepared TUF/role proof or an old-code restore
// capability. The consumer must freshly authenticate this exact manifest and
// delivery set. Directory is computed from the supplied root, never a journal.
type PairCandidate struct {
	Directory   string
	RawManifest []byte
	Delivery    []byte
	Manifest    *Manifest
	archive     string // fixed locator only; no authority to execute/restore code
	staged      bool   // durable intent exists; directory publication did not finish
}

func LoadPairCandidate(ctx context.Context, root string, custody DirectoryCustodian) (*PairCandidate, error) {
	if ctx == nil || ctx.Err() != nil || custody == nil || custody.Validate(ctx, root) != nil {
		return nil, ErrPairRecoveryRequired
	}
	if _, err := pairRoot(root); err != nil {
		return nil, err
	}
	state, _, err := readPairState()
	binding, bindingErr := pairRootBinding(root)
	if err != nil || bindingErr != nil || state.Root != binding {
		return nil, ErrPairRecoveryRequired
	}
	pending, archive, stage := filepath.Join(root, pairPendingName), "", filepath.Join(root, state.Stage)
	pendingExists, stageExists := !pairAbsent(pending), !pairAbsent(stage)
	dataExists := !pairAbsent(filepath.Join(pending, "data"))
	var parents []string
	dir, staged := "", false
	if dataExists {
		if stageExists {
			return nil, ErrPairRecoveryRequired // never select between two live data sets
		}
		dir, parents = filepath.Join(pending, "data"), []string{pending}
	} else if stageExists {
		dir, staged = stage, true
		if pendingExists {
			parents = append(parents, pending) // a retained empty reservation, not a new directory
		}
	} else if pendingExists {
		return nil, ErrPairRecoveryRequired // missing payload/control bytes cannot be invented
	}
	if state.Archive != "" {
		container := filepath.Join(root, state.Archive)
		archived := filepath.Join(container, "transaction")
		if !pairAbsent(archived) {
			if dir != "" {
				if state.IntentID != "" {
					return nil, ErrPairRecoveryRequired // ambiguous unfinished move
				}
				// An accepted receipt may outlive the old directory rename after
				// power loss. Prefer its exact reappeared pending/stage; a fresh
				// SDK-authorized intent must precede any correction. Keep archive.
			} else {
				dir, archive, parents = filepath.Join(archived, "data"), container, []string{container, archived}
			}
		}
	}
	if dir == "" {
		return nil, ErrPairRecoveryRequired
	}
	for _, parent := range parents {
		if err := custody.PinDirectory(ctx, parent); err != nil {
			return nil, err
		}
	}
	if err := custody.PinDirectory(ctx, dir); err != nil {
		return nil, err
	}
	for _, name := range pairChildDirectories() {
		if err := custody.PinDirectory(ctx, filepath.Join(dir, name)); err != nil {
			return nil, err
		}
	}
	raw, err := pairRead(filepath.Join(dir, "journal.json"), maxPairRecord)
	if err != nil {
		return nil, err
	}
	record, err := decodePairRecord(raw)
	if err != nil {
		return nil, err
	}
	manifestRaw, err := pairRead(filepath.Join(dir, "update.json"), MaxManifestSize)
	if err != nil || pairDigest(manifestRaw) != record.ManifestSHA256 {
		return nil, ErrPairRecoveryRequired
	}
	manifest, err := DecodeStrict(manifestRaw)
	if err != nil || manifest.Schema != PairSchema {
		return nil, ErrPairRecoveryRequired
	}
	var sidecar []byte
	if record.DeliverySHA256 != "" {
		sidecar, err = pairRead(filepath.Join(dir, "delivery.json"), maxPairDelivery)
		if err != nil || pairDigest(sidecar) != record.DeliverySHA256 {
			return nil, ErrPairRecoveryRequired
		}
	} else if !pairAbsent(filepath.Join(dir, "delivery.json")) {
		return nil, ErrPairRecoveryRequired
	}
	version, versionErr := ParseVersion(manifest.Version)
	if !state.matches(root, manifestRaw, sidecar) || versionErr != nil || version.String() != state.Version ||
		manifest.SHA256 != state.AppSHA256 || manifest.Size != state.AppSize || manifest.NativeHelper == nil ||
		manifest.NativeHelper.SHA256 != state.HelperSHA256 || manifest.NativeHelper.Size != state.HelperSize {
		return nil, ErrPairRecoveryRequired
	}
	return &PairCandidate{Directory: dir, RawManifest: manifestRaw, Delivery: sidecar, Manifest: manifest, archive: archive, staged: staged}, nil
}
