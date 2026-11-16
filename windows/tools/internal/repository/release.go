package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/tools/internal/deliverypack"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

const maxReleaseFiles = 256
const maxReleaseInputs = 32
const maxReleaseWorkBytes = 16 << 20

type ReleasePlan struct {
	Schema   int            `json:"schema"`
	Packages []ReleaseInput `json:"packages"`
	Purpose  delivery.Kind  `json:"purpose,omitempty"` // absent preserves ordinary full-release identity
}

type ReleaseInput struct {
	Kind delivery.Kind `json:"kind"`
	Plan string        `json:"plan"`
}

type ReleaseFile struct {
	Path   string `json:"path"`
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
}

type ReleaseArtifact struct {
	Kind          delivery.Kind `json:"kind"`
	Target        string        `json:"target"`
	SHA256        string        `json:"sha256"`
	ClientPackage string        `json:"client_package"`
}

type ReleaseManifest struct {
	Schema              int               `json:"schema"`
	ApprovedRootSHA256  string            `json:"approved_issuer_root_sha256"`
	PublishedRootSHA256 string            `json:"published_root_sha256"`
	RootVersion         int64             `json:"root_version"`
	ValidUntil          time.Time         `json:"valid_until"`
	SourceMetadata      map[string]string `json:"source_metadata"`
	Files               []ReleaseFile     `json:"files"`
	Artifacts           []ReleaseArtifact `json:"artifacts"`
	Purpose             delivery.Kind     `json:"purpose,omitempty"`
}

type releaseWire struct {
	Schema    int               `json:"schema"`
	ReleaseID string            `json:"release_id"`
	Manifest  ReleaseManifest   `json:"manifest"`
	Files     map[string][]byte `json:"files"`
}

type releaseArtifact struct {
	kind delivery.Kind
	raw  []byte
}

type releaseCandidate struct {
	wire           releaseWire
	records        map[string][]byte
	artifacts      map[string]releaseArtifact
	transferGrants map[string][]byte // per-profile transfer intent, distinct from inventory grant
	publications   []*deliverypack.Publication
	fileBytes      int
	paths          map[string]string
}

func loadReleasePlan(filename string) (*ReleasePlan, string, error) {
	raw, err := publisherfiles.Read(filename, 32<<10)
	if err != nil {
		return nil, "", err
	}
	fields, err := publisherfiles.Object(raw, 3)
	if err != nil || fields["schema"] == nil || fields["packages"] == nil {
		return nil, "", errors.New("repository: schema/packages with optional explicit policy-only purpose required")
	}
	purpose, err := releasePurpose(fields, 2)
	if err != nil {
		return nil, "", err
	}
	var entries []json.RawMessage
	if json.Unmarshal(fields["packages"], &entries) != nil || len(entries) == 0 || len(entries) > maxReleaseInputs {
		return nil, "", errors.New("repository: release package count outside bound")
	}
	for _, entry := range entries {
		fields, err := publisherfiles.Object(entry, 2)
		if err != nil || len(fields) != 2 || fields["kind"] == nil || fields["plan"] == nil {
			return nil, "", errors.New("repository: exact kind/plan input required")
		}
	}
	var plan ReleasePlan
	if json.Unmarshal(raw, &plan) != nil || plan.Schema != 1 {
		return nil, "", errors.New("repository: invalid release plan")
	}
	if purpose == delivery.PinPolicy && (len(plan.Packages) != 1 || plan.Packages[0].Kind != delivery.PinPolicy) {
		return nil, "", errors.New("repository: policy-only release requires exactly one policy input")
	}
	for _, input := range plan.Packages {
		if purpose == "" && input.Kind != delivery.Profile && input.Kind != delivery.UpdateManifest {
			return nil, "", errors.New("repository: ordinary release requires application inputs")
		}
	}
	base, err := filepath.Abs(filepath.Dir(filename))
	return &plan, base, err
}

func prepareRelease(ctx context.Context, filename, rootApproval string) (*releaseCandidate, error) {
	plan, base, err := loadReleasePlan(filename)
	if err != nil {
		return nil, err
	}
	c := &releaseCandidate{artifacts: map[string]releaseArtifact{}, paths: map[string]string{}}
	c.wire = releaseWire{Schema: 1, Files: map[string][]byte{}, Manifest: ReleaseManifest{Schema: 1, ApprovedRootSHA256: rootApproval, SourceMetadata: map[string]string{}, Artifacts: []ReleaseArtifact{}}}
	c.wire.Manifest.Purpose = plan.Purpose
	workBytes := 0
	for _, input := range plan.Packages {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if _, err = publisherfiles.ReadRelativeJSON(base, input.Plan, 32<<10); err != nil {
			return nil, err
		}
		pub, err := deliverypack.PreparePublication(ctx, input.Kind, filepath.Join(base, filepath.FromSlash(input.Plan)), rootApproval)
		if err != nil {
			return nil, err
		}
		summary := pub.Summary()
		for _, record := range summary.Metadata {
			workBytes += record.Bytes
		}
		workBytes += summary.ArtifactBytes + summary.OutputBytes + summary.PinPolicyBytes
		if workBytes > maxReleaseWorkBytes {
			return nil, errors.New("repository: cumulative release input budget exhausted")
		}
		records := pub.Metadata()
		if c.records == nil {
			c.records = records
		} else if !sameMetadata(c.records, records) {
			return nil, errors.New("repository: packages must use the exact same metadata snapshot")
		}
		if c.wire.Manifest.RootVersion != 0 && c.wire.Manifest.RootVersion != summary.RootVersion {
			return nil, errors.New("repository: mixed root generations")
		}
		c.wire.Manifest.RootVersion = summary.RootVersion
		c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, summary.DeliveryValidUntil)
		if c.artifacts[summary.TargetPath].raw != nil {
			return nil, errors.New("repository: duplicate release target")
		}
		c.artifacts[summary.TargetPath] = releaseArtifact{kind: input.Kind, raw: pub.Artifact()}
		if grant := pub.EpochGrant(); len(grant) != 0 {
			if old := c.artifacts[delivery.EpochPolicyPath].raw; old != nil && !bytes.Equal(old, grant) {
				return nil, errors.New("repository: mixed epoch grants")
			}
			c.artifacts[delivery.EpochPolicyPath] = releaseArtifact{kind: delivery.ProfileEpoch, raw: grant}
		}
		if pin := pub.PinPolicy(); len(pin) != 0 {
			if old := c.artifacts[delivery.PinPolicyPath].raw; old != nil && !bytes.Equal(old, pin) {
				return nil, errors.New("repository: mixed global pin policies")
			}
			c.artifacts[delivery.PinPolicyPath] = releaseArtifact{kind: delivery.PinPolicy, raw: pin}
		}
		clientName := "clients/" + digest([]byte(string(input.Kind)+"/"+summary.TargetPath)) + ".json"
		if err = c.addFile(clientName, pub.ClientPackage()); err != nil {
			return nil, err
		}
		c.wire.Manifest.Artifacts = append(c.wire.Manifest.Artifacts, ReleaseArtifact{Kind: input.Kind, Target: summary.TargetPath, SHA256: summary.ArtifactSHA256, ClientPackage: clientName})
		c.publications = append(c.publications, pub)
	}
	if err = c.buildLayout(time.Now().UTC()); err != nil {
		return nil, err
	}
	return c, nil
}

func sameMetadata(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for name, raw := range a {
		if !bytes.Equal(raw, b[name]) {
			return false
		}
	}
	return true
}

func earlier(a, b time.Time) time.Time {
	a, b = a.UTC(), b.UTC()
	if a.IsZero() || b.Before(a) {
		return b
	}
	return a
}

func (c *releaseCandidate) addFile(name string, raw []byte) error {
	if !publisherfiles.RelativeJSON(name) {
		return errors.New("repository: invalid derived release path")
	}
	key := portablePathKey(name)
	if previous := c.paths[key]; previous != "" && previous != name {
		return errors.New("repository: case-aliased release paths")
	}
	if old, exists := c.wire.Files[name]; exists {
		if !bytes.Equal(old, raw) {
			return errors.New("repository: different content at one release path")
		}
		return nil
	}
	if len(raw) == 0 || len(c.wire.Files) >= maxReleaseFiles || len(raw) > delivery.MaxPackageBytes-c.fileBytes {
		return delivery.ErrBudget
	}
	c.paths[key] = name
	c.fileBytes += len(raw)
	c.wire.Files[name] = raw // inputs are already owned; no external mutable view escapes
	return nil
}

func (c *releaseCandidate) check(ctx context.Context, now time.Time) error {
	if c == nil || ctx == nil || len(c.publications) == 0 {
		return delivery.ErrTrust // a decoded candidate is not an issuer verification result
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !now.Before(c.wire.Manifest.ValidUntil) {
		return errors.New("repository: release validity expired")
	}
	for _, pub := range c.publications {
		if err := pub.Check(now); err != nil {
			return err
		}
	}
	return nil
}

func (c *releaseCandidate) encode() ([]byte, error) {
	names := make([]string, 0, len(c.wire.Files))
	for name := range c.wire.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	c.wire.Manifest.Files = []ReleaseFile{}
	for _, name := range names {
		raw := c.wire.Files[name]
		c.wire.Manifest.Files = append(c.wire.Manifest.Files, ReleaseFile{Path: name, Length: len(raw), SHA256: digest(raw)})
	}
	sort.Slice(c.wire.Manifest.Artifacts, func(i, j int) bool { return c.wire.Manifest.Artifacts[i].Target < c.wire.Manifest.Artifacts[j].Target })
	manifest, err := json.Marshal(c.wire.Manifest)
	if err != nil {
		return nil, err
	}
	c.wire.ReleaseID = digest(manifest)
	raw, err := json.Marshal(c.wire)
	if err != nil || len(raw) > delivery.MaxEncodedPackageBytes {
		return nil, delivery.ErrBudget
	}
	return raw, nil
}

func release(filename, approval, output string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, err := prepareRelease(ctx, filename, approval)
	if err != nil {
		return nil, err
	}
	raw, err := c.encode()
	if err != nil {
		return nil, err
	}
	check := func() error { return c.check(ctx, time.Now().UTC()) }
	if err = publisherfiles.PublishNewChecked(output, raw, check); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ReleaseID                  string    `json:"release_id"`
		SHA256                     string    `json:"candidate_sha256"`
		Files                      int       `json:"files"`
		Bytes                      int       `json:"bytes"`
		ValidUntil                 time.Time `json:"valid_until"`
		RequiresPromotion          bool      `json:"requires_promotion"`
		RequiresClientAcceptance   bool      `json:"requires_client_acceptance"`
		IncludesPayloadExecutables bool      `json:"includes_payload_executables"`
	}{c.wire.ReleaseID, digest(raw), len(c.wire.Files), len(raw), c.wire.Manifest.ValidUntil, true, true, false})
}
