package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

var supportedSpec = regexp.MustCompile(`^1\.0\.(?:0|[1-9][0-9]{0,5})$`)

func keyIdentity(key *tuf.Key) (string, error) {
	if key == nil || key.Type != "ed25519" || key.Scheme != "ed25519" || len(key.UnrecognizedFields) != 0 || len(key.Value.UnrecognizedFields) != 0 {
		return "", fmt.Errorf("%w: public Ed25519 keys required", ErrTrust)
	}
	pub, err := hex.DecodeString(key.Value.PublicKey)
	if err != nil || len(pub) != 32 || hex.EncodeToString(pub) != key.Value.PublicKey {
		return "", ErrTrust
	}
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:]), nil
}

func roleKeys(keys map[string]*tuf.Key, ids []string, threshold, minimum int) (map[string]bool, error) {
	if len(keys) > 64 || len(ids) > 32 || threshold < minimum || threshold > len(ids) {
		return nil, ErrTrust
	}
	identities := map[string]bool{}
	seenIDs := map[string]bool{}
	for _, id := range ids {
		if !validHash(id) || seenIDs[id] {
			return nil, ErrTrust
		}
		seenIDs[id] = true
		identity, err := keyIdentity(keys[id])
		if err != nil {
			return nil, err
		}
		if identities[identity] {
			return nil, fmt.Errorf("%w: duplicate public key material", ErrTrust)
		}
		identities[identity] = true
	}
	if len(identities) < threshold {
		return nil, ErrTrust
	}
	return identities, nil
}

func overlaps(a, b map[string]bool) bool {
	for id := range a {
		if b[id] {
			return true
		}
	}
	return false
}

// Product policy is additional to TUF verification, never a replacement for
// signatures. Apply it to every downloaded root, including intermediates, so
// a rotation cannot transiently downgrade the required threshold.
func rootPolicy(root *tuf.Metadata[tuf.RootType]) error {
	if root == nil || root.Signed.Type != tuf.ROOT || root.Signed.Version <= 0 || root.Signed.Version > math.MaxInt64-MaxRootRotations-1 || !root.Signed.ConsistentSnapshot || root.Signed.Expires.IsZero() || len(root.Signed.Keys) > 64 || len(root.Signed.Roles) != 4 {
		return fmt.Errorf("%w: root policy/version/consistent-snapshot constraint", ErrTrust)
	}
	sets := map[string]map[string]bool{}
	for _, name := range []string{tuf.ROOT, tuf.TARGETS, tuf.TIMESTAMP, tuf.SNAPSHOT} {
		role := root.Signed.Roles[name]
		if role == nil {
			return ErrTrust
		}
		minimum := 1
		if name == tuf.ROOT || name == tuf.TARGETS {
			minimum = 2
		}
		keys, err := roleKeys(root.Signed.Keys, role.KeyIDs, role.Threshold, minimum)
		if err != nil {
			return err
		}
		sets[name] = keys
	}
	for _, offline := range []string{tuf.ROOT, tuf.TARGETS} {
		for _, online := range []string{tuf.TIMESTAMP, tuf.SNAPSHOT} {
			if overlaps(sets[offline], sets[online]) {
				return fmt.Errorf("%w: online key also holds offline authority", ErrTrust)
			}
		}
	}
	for _, key := range root.Signed.Keys {
		if _, err := keyIdentity(key); err != nil {
			return err
		}
	}
	return nil
}

func targetsPolicy(role string, meta *tuf.Metadata[tuf.TargetsType]) error {
	if meta == nil || meta.Signed.Type != tuf.TARGETS || meta.Signed.Version <= 0 || meta.Signed.Expires.IsZero() || len(meta.Signed.Targets) > 256 {
		return ErrTrust
	}
	if role != tuf.TARGETS {
		prefix := role + "/"
		if role != "profiles" && role != "updates" || meta.Signed.Delegations != nil {
			return ErrTrust
		}
		for target := range meta.Signed.Targets {
			if !strings.HasPrefix(target, prefix) {
				return ErrTrust
			}
		}
		return nil
	}
	delegations := meta.Signed.Delegations
	if len(meta.Signed.Targets) > 2 || delegations == nil || delegations.SuccinctRoles != nil || len(delegations.Roles) != 2 {
		return fmt.Errorf("%w: separate profile/update delegations required", ErrTrust)
	}
	for target := range meta.Signed.Targets {
		if target != EpochPolicyPath && target != PinPolicyPath {
			return fmt.Errorf("%w: only reserved offline epoch/pin-policy targets are allowed", ErrTrust)
		}
	}
	sets := map[string]map[string]bool{}
	for _, role := range delegations.Roles {
		if role.Name != "profiles" && role.Name != "updates" || sets[role.Name] != nil || !role.Terminating || len(role.Paths) != 1 || role.Paths[0] != role.Name+"/*" || len(role.PathHashPrefixes) != 0 {
			return ErrTrust
		}
		minimum := 1
		if role.Name == "updates" {
			minimum = 2
		}
		keys, err := roleKeys(delegations.Keys, role.KeyIDs, role.Threshold, minimum)
		if err != nil {
			return err
		}
		sets[role.Name] = keys
	}
	if sets["profiles"] == nil || sets["updates"] == nil || overlaps(sets["profiles"], sets["updates"]) {
		return fmt.Errorf("%w: profile key cannot hold code-delivery authority", ErrTrust)
	}
	return nil
}

func guardMetadata(name string, data []byte) error {
	return guardMetadataShape(name, data, true)
}

// CheckPublicationMetadata shares product shape/resource policy with offline
// authoring. It permits an unsigned draft; it does not verify signatures or
// create an artifact proof. Client acceptance still uses the full SDK workflow.
func CheckPublicationMetadata(role string, data []byte) error {
	switch role {
	case tuf.ROOT, tuf.TARGETS, tuf.SNAPSHOT, tuf.TIMESTAMP, "profiles", "updates":
	default:
		return ErrTrust
	}
	if len(data) == 0 || len(data) > MaxMetadataBytes {
		return ErrBudget
	}
	return guardMetadataShape(role+".json", data, false)
}

func guardMetadataShape(name string, data []byte, requireSignatures bool) error {
	if err := strictJSON(data); err != nil {
		return err
	}
	var head struct {
		Signed struct {
			Type        string `json:"_type"`
			SpecVersion string `json:"spec_version"`
		} `json:"signed"`
		Signatures []json.RawMessage `json:"signatures"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return ErrTrust
	}
	if !supportedSpec.MatchString(head.Signed.SpecVersion) {
		return fmt.Errorf("%w: unsupported metadata specification family", ErrTrust)
	}
	if requireSignatures && len(head.Signatures) == 0 || len(head.Signatures) > 64 {
		return ErrBudget
	}
	role := strings.TrimSuffix(name, ".json")
	if i := strings.IndexByte(role, '.'); i >= 0 {
		role = role[i+1:]
	}
	switch role {
	case tuf.ROOT:
		if head.Signed.Type != tuf.ROOT {
			return ErrTrust
		}
		root, err := tuf.Root().FromBytes(data)
		if err != nil {
			return fmt.Errorf("%w: malformed root", ErrTrust)
		}
		return rootPolicy(root)
	case tuf.TARGETS, "profiles", "updates":
		if head.Signed.Type != tuf.TARGETS {
			return ErrTrust
		}
		meta, err := tuf.Targets().FromBytes(data)
		if err != nil {
			return fmt.Errorf("%w: malformed targets", ErrTrust)
		}
		return targetsPolicy(role, meta)
	case tuf.TIMESTAMP:
		meta, err := tuf.Timestamp().FromBytes(data)
		if err != nil || meta.Signed.Version <= 0 || meta.Signed.Expires.IsZero() || len(meta.Signed.Meta) != 1 || meta.Signed.Meta["snapshot.json"] == nil {
			return ErrTrust
		}
		return metadataReferences(meta.Signed.Meta)
	case tuf.SNAPSHOT:
		meta, err := tuf.Snapshot().FromBytes(data)
		if err != nil || meta.Signed.Version <= 0 || meta.Signed.Expires.IsZero() || meta.Signed.Meta["targets.json"] == nil {
			return ErrTrust
		}
		return metadataReferences(meta.Signed.Meta)
	}
	return ErrTrust
}

// Shape only: the full SDK workflow owns signature/version/link verification.
// Bound references and reject nil entries before SDK rollback iteration. Keep
// optional hashes/length semantics; do not invent a second metadata verifier.
func metadataReferences(refs map[string]*tuf.MetaFiles) error {
	if len(refs) == 0 || len(refs) > MaxRecords {
		return ErrBudget
	}
	for name, ref := range refs {
		if !metadataName.MatchString(name) || ref == nil || ref.Version <= 0 || ref.Length < 0 || ref.Length > MaxMetadataBytes {
			return ErrTrust
		}
	}
	return nil
}

// EnrollmentSource owns an explicitly approved, structurally valid and fresh
// root as pending verifier input. It performs no signature verification and is
// NOT an accepted trust policy or proof. The product must obtain the full SDK
// batch proof (including the initial root signature) before committing enrollment.
func EnrollmentSource(rootData []byte, approvedSHA256 string, now time.Time) (*State, error) {
	if len(rootData) == 0 || len(rootData) > MaxMetadataBytes || now.IsZero() || !validHash(approvedSHA256) {
		return nil, ErrTrust
	}
	sum := sha256.Sum256(rootData)
	if hex.EncodeToString(sum[:]) != approvedSHA256 {
		return nil, fmt.Errorf("%w: enrollment fingerprint mismatch", ErrTrust)
	}
	if err := guardMetadata("root.json", rootData); err != nil {
		return nil, err
	}
	root, err := tuf.Root().FromBytes(rootData)
	if err != nil || !root.Signed.Expires.After(now) {
		return nil, fmt.Errorf("%w: initial root expiry rejected", ErrTrust)
	}
	return &State{Schema: 1, AnchorSHA256: approvedSHA256, Root: append([]byte(nil), rootData...), Cache: map[string][]byte{}, ClockFloor: now.UTC()}, nil
}

// Enroll retains the offline publisher's synchronous signature-checked API.
// Product enrollment uses EnrollmentSource followed by its bounded SDK worker.
func Enroll(rootData []byte, approvedSHA256 string, now time.Time) (*State, error) {
	state, err := EnrollmentSource(rootData, approvedSHA256, now)
	if err != nil {
		return nil, err
	}
	trusted, err := trustedmetadata.New(state.Root)
	if err != nil || trusted.Root == nil || !trusted.Root.Signed.Expires.After(now) {
		return nil, fmt.Errorf("%w: initial root signature/expiry rejected", ErrTrust)
	}
	return state, nil
}
