// Package delivery adapts go-tuf to bounded offline profile/update packages.
// It neither fetches from the network nor installs an EXE or changes VPN state.
package delivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	MaxMetadataBytes       = 512 << 10
	MaxPackageBytes        = 8 << 20
	MaxEncodedPackageBytes = 12 << 20
	MaxStateBytes          = 5 << 20
	MaxRecords             = 64
	MaxRootRotations       = 16
	MaxDelegations         = 8
	MaxProfileBytes        = 256 << 10
	MaxManifestBytes       = 64 << 10
	MaxEpochGrantBytes     = 16 << 10
	MaxPinPolicyBytes      = 768 << 10
	MaxArtifactRequests    = 3
	EpochPolicyPath        = "trust/profile-epoch.json"
	PinPolicyPath          = "trust/pin-history.json"
)

var (
	ErrTrust     = errors.New("trusted delivery metadata unavailable")
	ErrBudget    = errors.New("delivery package exceeds local resource bounds")
	metadataName = regexp.MustCompile(`^(?:[1-9][0-9]{0,18}\.)?(?:root|timestamp|snapshot|targets|profiles|updates)\.json$`)
)

func ValidMetadataName(name string) bool { return metadataName.MatchString(name) }

type Kind string

const (
	Profile        Kind = "profile"
	UpdateManifest Kind = "update_manifest"
	ProfileEpoch   Kind = "profile_epoch"
	PinPolicy      Kind = "pin_policy"
)

// State is a public retained TUF checkpoint, not a fresh trust bootstrap.
// Cache holds original input bytes bound to successful per-role SDK returns
// and local continuity/time/link gates, plus old accepted expired history for
// rollback checks. Intermediate SDK pointers or disk persistence are not the
// source of new acceptance. A missing/corrupt state is not empty.
type State struct {
	Schema       int               `json:"schema"`
	AnchorSHA256 string            `json:"anchor_sha256"`
	Root         []byte            `json:"root"`
	Cache        map[string][]byte `json:"cache"`
	ClockFloor   time.Time         `json:"clock_floor"`
}

func (s *State) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || len(data) > MaxStateBytes {
		return ErrBudget
	}
	if err := strictJSON(data); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || len(object) != 5 {
		return ErrTrust
	}
	for _, name := range []string{"schema", "anchor_sha256", "root", "cache", "clock_floor"} {
		value, exists := object[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return ErrTrust
		}
	}
	type wireState State
	var out wireState
	if err := json.Unmarshal(data, &out); err != nil {
		return ErrTrust
	}
	next := State(out)
	if err := next.Validate(); err != nil {
		return err
	}
	*s = next
	return nil
}

func (s *State) Clone() *State {
	if s == nil {
		return nil
	}
	out := *s
	out.Root = bytes.Clone(s.Root)
	out.Cache = make(map[string][]byte, len(s.Cache))
	for name, data := range s.Cache {
		out.Cache[name] = bytes.Clone(data)
	}
	return &out
}

func validHash(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == sha256.Size && value == strings.ToLower(value)
}

func cacheName(name string) bool {
	switch name {
	case "timestamp.json", "snapshot.json", "targets.json", "profiles.json", "updates.json":
		return true
	}
	return false
}

func (s *State) Validate() error {
	if s == nil || s.Schema != 1 || !validHash(s.AnchorSHA256) || s.ClockFloor.IsZero() || len(s.Cache) > 5 || len(s.Root) == 0 || len(s.Root) > MaxMetadataBytes {
		return fmt.Errorf("%w: invalid retained delivery state", ErrTrust)
	}
	total := len(s.Root)
	if err := guardMetadata("root.json", s.Root); err != nil {
		return err
	}
	for name, data := range s.Cache {
		if !cacheName(name) || len(data) == 0 || len(data) > MaxMetadataBytes {
			return ErrBudget
		}
		total += len(data)
		if total > MaxStateBytes {
			return ErrBudget
		}
		if err := guardMetadata(name, data); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) Digest() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(s)
	if err != nil || len(data) > MaxStateBytes {
		return "", ErrBudget
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Package contains public metadata only. Artifact bytes have a separate bound
// and remain opaque to this layer (a profile or an update manifest, not an EXE).
type Package struct {
	Metadata map[string][]byte `json:"metadata"`
}

// NewPackage validates and owns explicitly supplied public records. It neither
// discovers repository files nor proves signatures; Prepare supplies proof.
func NewPackage(records map[string][]byte) (*Package, error) {
	return (&Package{Metadata: records}).snapshot()
}

func (p *Package) Encode() ([]byte, error) {
	owned, err := p.snapshot()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(owned)
	if err != nil || len(data) > MaxEncodedPackageBytes {
		return nil, ErrBudget
	}
	return data, nil
}

func (p *TargetPackage) Encode(kind Kind, artifact []byte) ([]byte, error) {
	if p == nil {
		return nil, ErrTrust
	}
	if err := (Request{Kind: kind, Path: p.TargetPath, Bytes: artifact}).validate(); err != nil {
		return nil, err
	}
	pack, err := p.Package.snapshot()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(&TargetPackage{TargetPath: p.TargetPath, Package: pack})
	if err != nil || len(data) > MaxEncodedPackageBytes+512 {
		return nil, ErrBudget
	}
	return data, nil
}

// TargetPackage is the public sidecar carried beside an update manifest. It
// identifies an opaque target path; it grants no trust or enrollment itself.
type TargetPackage struct {
	TargetPath string   `json:"target_path"`
	Package    *Package `json:"package"`
}

func DecodeTargetPackage(data []byte) (*TargetPackage, error) {
	if len(data) == 0 || len(data) > MaxEncodedPackageBytes+512 {
		return nil, ErrBudget
	}
	if err := strictJSON(data); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || len(object) != 2 || object["target_path"] == nil || object["package"] == nil {
		return nil, ErrTrust
	}
	var targetPath string
	if err := json.Unmarshal(object["target_path"], &targetPath); err != nil || len(targetPath) == 0 || len(targetPath) > 160 {
		return nil, ErrTrust
	}
	pack, err := DecodePackage(object["package"])
	if err != nil {
		return nil, err
	}
	return &TargetPackage{TargetPath: targetPath, Package: pack}, nil
}

func DecodePackage(data []byte) (*Package, error) {
	if len(data) == 0 || len(data) > MaxEncodedPackageBytes {
		return nil, ErrBudget
	}
	if err := strictJSON(data); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || len(object) != 1 || object["metadata"] == nil {
		return nil, ErrTrust
	}
	var pack Package
	if err := json.Unmarshal(data, &pack); err != nil {
		return nil, ErrTrust
	}
	return pack.snapshot()
}

func (p *Package) snapshot() (*Package, error) {
	if p == nil || len(p.Metadata) == 0 || len(p.Metadata) > MaxRecords {
		return nil, ErrBudget
	}
	out := &Package{Metadata: make(map[string][]byte, len(p.Metadata))}
	total := 0
	for name, data := range p.Metadata {
		if !ValidMetadataName(name) || len(data) == 0 || len(data) > MaxMetadataBytes {
			return nil, ErrBudget
		}
		total += len(data)
		if total > MaxPackageBytes {
			return nil, ErrBudget
		}
		if err := guardMetadata(name, data); err != nil {
			return nil, err
		}
		out.Metadata[name] = bytes.Clone(data)
	}
	return out, nil
}

type Request struct {
	Kind  Kind
	Path  string
	Bytes []byte
}

// Validate shares artifact purpose/path/size bounds with offline publishers.
// It does not verify any metadata or authorize acceptance.
func (r Request) Validate() error { return r.validate() }

func (r Request) validate() error {
	limit, prefix := MaxProfileBytes, "profiles/"
	if r.Kind == UpdateManifest {
		limit, prefix = MaxManifestBytes, "updates/"
	} else if r.Kind == ProfileEpoch {
		if r.Path != EpochPolicyPath {
			return ErrTrust
		}
		limit, prefix = MaxEpochGrantBytes, "trust/"
	} else if r.Kind == PinPolicy {
		if r.Path != PinPolicyPath {
			return ErrTrust
		}
		limit, prefix = MaxPinPolicyBytes, "trust/"
	} else if r.Kind != Profile {
		return ErrTrust
	}
	if len(r.Bytes) == 0 || len(r.Bytes) > limit {
		return ErrBudget
	}
	if len(r.Path) > 160 || !strings.HasPrefix(r.Path, prefix) || !strings.HasSuffix(r.Path, ".json") || path.Clean(r.Path) != r.Path || strings.ContainsAny(r.Path, `\:%?#`) {
		return fmt.Errorf("%w: artifact path is outside its declared purpose", ErrTrust)
	}
	for _, part := range strings.Split(r.Path, "/") {
		if part == "" || part == "." || part == ".." {
			return ErrTrust
		}
	}
	for _, c := range r.Path {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/._-", c)) {
			return ErrTrust
		}
	}
	return nil
}

// Prepared cannot be constructed by interpreting an untrusted State as proof.
// The caller still needs source-fenced persistence and its application-specific
// signature/schema/version/lifecycle gates before changing a profile or EXE.
type Prepared struct {
	source            string
	next              *State
	kind              Kind
	targetPath        string
	artifact          []byte
	rootVersion       int64
	expires           time.Time
	pinPolicyRequired bool // captured from authenticated offline top-level targets
}

// RequiresPinPolicy is a private verifier fact, never an unsigned bundle hint.
func (p *Prepared) RequiresPinPolicy() bool { return p != nil && p.pinPolicyRequired }

func (p *Prepared) NextState() *State {
	if p == nil {
		return nil
	}
	return p.next.Clone()
}
func (p *Prepared) SourceDigest() string {
	if p == nil {
		return ""
	}
	return p.source
}
func (p *Prepared) Artifact() []byte {
	if p == nil {
		return nil
	}
	return bytes.Clone(p.artifact)
}
func (p *Prepared) RootVersion() int64 {
	if p == nil {
		return 0
	}
	return p.rootVersion
}

func (p *Prepared) Kind() Kind {
	if p == nil {
		return ""
	}
	return p.kind
}
func (p *Prepared) TargetPath() string {
	if p == nil {
		return ""
	}
	return p.targetPath
}
func (p *Prepared) ValidUntil() time.Time {
	if p == nil {
		return time.Time{}
	}
	return p.expires
}

func (p *Prepared) Check(kind Kind, targetPath string, data []byte, now time.Time) error {
	if p == nil || p.next == nil || p.kind != kind || p.targetPath != targetPath || !bytes.Equal(p.artifact, data) || now.Before(p.next.ClockFloor) || !now.Before(p.expires) {
		return ErrTrust
	}
	return nil
}

// Progress can retain a verified metadata prefix even when a later repository
// step fails. No artifact proof exists until the complete lookup/hash gate passes.
type Progress struct {
	source string
	next   *State
	proof  *Prepared
	proofs map[Kind]*Prepared
}

func (p *Progress) SourceDigest() string {
	if p == nil {
		return ""
	}
	return p.source
}
func (p *Progress) NextState() *State {
	if p == nil {
		return nil
	}
	return p.next.Clone()
}
func (p *Progress) Proof() *Prepared {
	if p == nil {
		return nil
	}
	return p.proof
}

func (p *Progress) ProofFor(kind Kind) *Prepared {
	if p == nil {
		return nil
	}
	return p.proofs[kind]
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("delivery context is nil")
	}
	return ctx.Err()
}

// Reject duplicate JSON properties before the upstream canonicalizer sees them.
// TUF target paths/key IDs are case sensitive; no case folding is introduced.
func strictJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 24 {
			return ErrBudget
		}
		token, err := dec.Token()
		if err != nil {
			return err
		}
		delim, composite := token.(json.Delim)
		if !composite {
			return nil
		}
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("%w: duplicate metadata property", ErrTrust)
				}
				seen[name] = true
			}
			if err := walk(depth + 1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	if err := walk(0); err != nil {
		return fmt.Errorf("%w: invalid bounded JSON", ErrTrust)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%w: trailing metadata data", ErrTrust)
	}
	return nil
}
