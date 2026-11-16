package repository

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

func exactObject(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	fields, err := publisherfiles.Object(raw, len(names))
	if err != nil || len(fields) != len(names) {
		return nil, errors.New("repository: exact object shape required")
	}
	for _, name := range names {
		if fields[name] == nil {
			return nil, errors.New("repository: missing/aliased object field")
		}
	}
	return fields, nil
}

func candidateRecords(raw []byte, limit int, names ...string) error {
	var records []json.RawMessage
	if json.Unmarshal(raw, &records) != nil || len(records) == 0 || len(records) > limit {
		return delivery.ErrBudget
	}
	for _, record := range records {
		if _, err := exactObject(record, names...); err != nil {
			return err
		}
	}
	return nil
}

func canonicalHash(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}

// decodeCandidate verifies structure/content identity, not signing authority.
// The separate approval binds these input bytes; full retained-trust SDK and
// application checks must still precede any publication.
func decodeCandidate(raw []byte, approval string) (*releaseWire, error) {
	if len(raw) == 0 || len(raw) > delivery.MaxEncodedPackageBytes {
		return nil, delivery.ErrBudget
	}
	raw = bytes.Clone(raw) // bind the approval and parser to one owned input snapshot
	if !approved(raw, approval) {
		return nil, errors.New("repository: approved candidate bytes changed")
	}
	fields, err := exactObject(raw, "schema", "release_id", "manifest", "files")
	if err != nil {
		return nil, err
	}
	manifest, err := publisherfiles.Object(fields["manifest"], 9)
	if err != nil {
		return nil, err
	}
	purpose, err := releasePurpose(manifest, 8)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"schema", "approved_issuer_root_sha256", "published_root_sha256", "root_version", "valid_until", "source_metadata", "files", "artifacts"} {
		if manifest[name] == nil {
			return nil, errors.New("repository: missing/aliased manifest property")
		}
	}
	files, err := publisherfiles.Object(fields["files"], maxReleaseFiles)
	if err != nil || len(files) == 0 {
		return nil, delivery.ErrBudget
	}
	for _, value := range files {
		var encoded string
		if json.Unmarshal(value, &encoded) != nil {
			return nil, errors.New("repository: release files require base64 strings")
		}
	}
	sources, err := publisherfiles.Object(manifest["source_metadata"], delivery.MaxRecords)
	if err != nil || len(sources) == 0 {
		return nil, delivery.ErrBudget
	}
	if err = candidateRecords(manifest["files"], maxReleaseFiles, "path", "length", "sha256"); err != nil {
		return nil, err
	}
	if err = candidateRecords(manifest["artifacts"], maxReleaseInputs, "kind", "target", "sha256", "client_package"); err != nil {
		return nil, err
	}
	var wire releaseWire
	if json.Unmarshal(raw, &wire) != nil || wire.Schema != 1 || wire.Manifest.Schema != 1 || wire.Manifest.RootVersion < 1 || wire.Manifest.ValidUntil.IsZero() {
		return nil, errors.New("repository: malformed release candidate")
	}
	if purpose == delivery.PinPolicy && (len(wire.Manifest.Artifacts) != 1 || wire.Manifest.Artifacts[0].Kind != delivery.PinPolicy || wire.Manifest.Artifacts[0].Target != delivery.PinPolicyPath) {
		return nil, errors.New("repository: policy-only manifest requires one reserved target")
	}
	if !canonicalHash(wire.ReleaseID) || !canonicalHash(wire.Manifest.ApprovedRootSHA256) || !canonicalHash(wire.Manifest.PublishedRootSHA256) {
		return nil, errors.New("repository: malformed release hashes")
	}
	wire.Manifest.ValidUntil = wire.Manifest.ValidUntil.UTC()
	manifestBytes, err := json.Marshal(wire.Manifest)
	if err != nil || digest(manifestBytes) != wire.ReleaseID {
		return nil, errors.New("repository: manifest identity mismatch")
	}
	if len(wire.Manifest.Files) != len(wire.Files) {
		return nil, errors.New("repository: incomplete file manifest")
	}
	last := ""
	total := 0
	paths := map[string]bool{}
	for _, file := range wire.Manifest.Files {
		data, exists := wire.Files[file.Path]
		key := portablePathKey(file.Path)
		if !exists || !publisherfiles.RelativeJSON(file.Path) || file.Path <= last || paths[key] || len(data) == 0 || file.Length != len(data) {
			return nil, errors.New("repository: conflicting/aliased/unbounded file inventory")
		}
		if len(data) > delivery.MaxPackageBytes-total {
			return nil, delivery.ErrBudget
		}
		if file.SHA256 != digest(data) {
			return nil, errors.New("repository: file content hash changed")
		}
		total += len(data)
		last = file.Path
		paths[key] = true
	}
	for name, filename := range wire.Manifest.SourceMetadata {
		_, derived, err := metadataRecord(name, wire.Files[filename])
		if err != nil || filename != derived {
			return nil, errors.New("repository: source metadata mapping changed")
		}
	}
	if wire.Manifest.SourceMetadata["root.json"] == "" || wire.Manifest.SourceMetadata["timestamp.json"] == "" {
		return nil, errors.New("repository: baseline root/timestamp missing")
	}
	last = ""
	for _, artifact := range wire.Manifest.Artifacts {
		allowedKind := purpose == delivery.PinPolicy && artifact.Kind == delivery.PinPolicy || purpose == "" && (artifact.Kind == delivery.Profile || artifact.Kind == delivery.UpdateManifest)
		if !allowedKind || len(artifact.Target) == 0 || len(artifact.Target) > 160 || artifact.Target <= last || !canonicalHash(artifact.SHA256) {
			return nil, errors.New("repository: duplicate/unknown artifact inventory")
		}
		expected := "clients/" + digest([]byte(string(artifact.Kind)+"/"+artifact.Target)) + ".json"
		if artifact.ClientPackage != expected || len(wire.Files[expected]) == 0 {
			return nil, errors.New("repository: client package mapping changed")
		}
		last = artifact.Target
	}
	return &wire, nil
}

func restoreCandidate(wire *releaseWire) *releaseCandidate {
	c := &releaseCandidate{wire: releaseWire{Schema: 1, Files: map[string][]byte{}, Manifest: ReleaseManifest{Schema: 1, ApprovedRootSHA256: wire.Manifest.ApprovedRootSHA256, RootVersion: wire.Manifest.RootVersion, ValidUntil: wire.Manifest.ValidUntil, SourceMetadata: map[string]string{}, Artifacts: append([]ReleaseArtifact(nil), wire.Manifest.Artifacts...)}}, records: map[string][]byte{}, artifacts: map[string]releaseArtifact{}, transferGrants: map[string][]byte{}, paths: map[string]string{}}
	for name, filename := range wire.Manifest.SourceMetadata {
		c.records[name] = wire.Files[filename]
	}
	c.wire.Manifest.Purpose = wire.Manifest.Purpose
	return c // decoded buffers are owned by this operation, never caller maps
}

func candidateArtifact(wire *releaseWire, artifact ReleaseArtifact) ([]byte, error) {
	dir := path.Join("targets", path.Dir(artifact.Target))
	suffix := "." + path.Base(artifact.Target)
	var found []byte
	for filename, raw := range wire.Files {
		if path.Dir(filename) != dir || !strings.HasSuffix(path.Base(filename), suffix) || digest(raw) != artifact.SHA256 {
			continue
		}
		if found != nil && !bytes.Equal(found, raw) {
			return nil, errors.New("repository: conflicting artifact aliases")
		}
		found = raw
	}
	if found == nil {
		return nil, errors.New("repository: target bytes missing from candidate")
	}
	if err := (delivery.Request{Kind: artifact.Kind, Path: artifact.Target, Bytes: found}).Validate(); err != nil {
		return nil, err
	}
	return found, nil
}

// Public transfer metadata must agree with the original common repository
// bytes. Keys inside it remain hints until the independent local-role check.
func restoreTransfer(c *releaseCandidate, wire *releaseWire, artifact ReleaseArtifact) ([]byte, error) {
	raw := wire.Files[artifact.ClientPackage]
	if err := c.addFile(artifact.ClientPackage, raw); err != nil {
		return nil, err
	}
	if artifact.Kind == delivery.PinPolicy {
		capsule, err := onboarding.DecodePinPolicyCapsule(raw)
		if err != nil {
			return nil, err
		}
		if c.wire.Manifest.Purpose != delivery.PinPolicy || artifact.Target != delivery.PinPolicyPath || digest(capsule.Policy) != artifact.SHA256 || !bytes.Equal(capsule.Policy, c.artifacts[delivery.PinPolicyPath].raw) || !sameMetadata(c.records, capsule.Package.Metadata) {
			return nil, errors.New("repository: policy capsule differs from the exact release target/metadata")
		}
		return capsule.Policy, nil
	}
	if artifact.Kind == delivery.Profile {
		payload, err := onboarding.DecodePublicProfile(raw)
		if err != nil {
			return nil, err
		}
		if payload.Delivery == nil || payload.Delivery.Package == nil || payload.Delivery.TargetPath != artifact.Target || len(payload.Delivery.ProfileBytes) == 0 || len(payload.TrustedKeys) != 1 || len(payload.Recovery) != 0 || payload.DeviceName != "" {
			return nil, errors.New("repository: unsupported profile transfer shape")
		}
		if !sameMetadata(c.records, payload.Delivery.Package.Metadata) {
			return nil, errors.New("repository: profile transfer repository changed")
		}
		profile, err := payload.ExactProfileBytes()
		if err != nil {
			return nil, err
		}
		if digest(profile) != artifact.SHA256 {
			return nil, errors.New("repository: profile transfer artifact changed")
		}
		if !bytes.Equal(payload.PinPolicy, c.artifacts[delivery.PinPolicyPath].raw) {
			return nil, errors.New("repository: profile transfer must carry the exact common global pin policy")
		}
		if grant := payload.EpochGrant; len(grant) != 0 {
			if old := c.artifacts[delivery.EpochPolicyPath].raw; old != nil && !bytes.Equal(old, grant) {
				return nil, errors.New("repository: transfer epoch grants differ")
			}
			c.artifacts[delivery.EpochPolicyPath] = releaseArtifact{kind: delivery.ProfileEpoch, raw: grant}
			c.transferGrants[artifact.Target] = grant
		}
		return profile, nil
	}
	transfer, err := delivery.DecodeTargetPackage(raw)
	if err != nil {
		return nil, err
	}
	if transfer.TargetPath != artifact.Target || transfer.Package == nil || !sameMetadata(c.records, transfer.Package.Metadata) {
		return nil, errors.New("repository: update transfer repository changed")
	}
	return candidateArtifact(wire, artifact)
}
