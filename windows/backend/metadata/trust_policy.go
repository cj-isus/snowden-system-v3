package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
)

type KeyRole string

const (
	RoleProfile        KeyRole = "profile"
	RoleUpdate         KeyRole = "update"
	RoleRecovery       KeyRole = "recovery"
	MaxTrustPolicySize         = 6 << 20 // public TUF cache plus bounded local key table
	MaxTrustKeys               = 256
)

var ErrTrustRole = errors.New("key is not authorized for the requested trust role")

// TrustPolicy holds locally established keys and optional retained delivery
// authority, never a bundle-controlled grant. A local key has one role.
// Ordinary onboarding cannot grant update/recovery power or enroll a root.
// Legacy schema1 keys are profile-only; comments never grant a role.
type TrustPolicy struct {
	Schema   int             `json:"schema"`
	Keys     []TrustKey      `json:"keys"`
	Delivery *delivery.State `json:"delivery,omitempty"`
}

func (p *TrustPolicy) Clone() *TrustPolicy {
	if p == nil {
		return nil
	}
	out := *p
	out.Keys = slices.Clone(p.Keys)
	out.Delivery = p.Delivery.Clone()
	return &out
}

type TrustKey struct {
	KeyID   string  `json:"key_id"`
	Public  string  `json:"public_hex"`
	Role    KeyRole `json:"role,omitempty"`
	Retired bool    `json:"retired,omitempty"`
	Comment string  `json:"comment,omitempty"`
}

func validRole(role KeyRole) bool {
	return role == RoleProfile || role == RoleUpdate || role == RoleRecovery
}

func (p *TrustPolicy) validate() error {
	if p == nil || p.Schema != 1 && p.Schema != 2 || len(p.Keys) > MaxTrustKeys {
		return errors.New("metadata: invalid trust policy schema or key count")
	}
	if p.Delivery != nil {
		if p.Schema != 2 {
			return errors.New("metadata: delivery authority requires scoped trust policy")
		}
		if _, err := p.Delivery.Digest(); err != nil {
			return err
		}
	}
	seen := make(map[string]bool, len(p.Keys))
	for _, key := range p.Keys {
		pub, err := hex.DecodeString(key.Public)
		if err != nil || len(pub) != ed25519.PublicKeySize || key.Public != hex.EncodeToString(pub) || len(key.Comment) > 256 {
			return errors.New("metadata: invalid public trust key")
		}
		id := KeyIDFor(ed25519.PublicKey(pub))
		if seen[id] || key.KeyID != id || p.Schema == 1 && (key.Role != "" || key.Retired) || p.Schema == 2 && !validRole(key.Role) {
			return errors.New("metadata: duplicate, mismatched or unscoped trust key")
		}
		seen[id] = true
	}
	return nil
}

func DecodeTrustPolicy(data []byte) (*TrustPolicy, error) {
	if len(data) == 0 || len(data) > MaxTrustPolicySize {
		return nil, errors.New("metadata: trust policy size outside bound")
	}
	if err := checkCheckpointJSON(data); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil || !bytes.HasPrefix(bytes.TrimSpace(object["keys"]), []byte("[")) {
		return nil, errors.New("metadata: trust policy object and keys required")
	}
	for name := range object {
		if name != "schema" && name != "keys" && name != "delivery" {
			return nil, errors.New("metadata: unknown/aliased trust policy field")
		}
	}
	if raw, exists := object["delivery"]; exists && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, errors.New("metadata: delivery authority cannot be null")
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(object["keys"], &entries); err != nil || len(entries) > MaxTrustKeys {
		return nil, errors.New("metadata: invalid trust key entries")
	}
	for _, entry := range entries {
		if entry == nil {
			return nil, errors.New("metadata: trust key object required")
		}
		for name, value := range entry {
			if name != "key_id" && name != "public_hex" && name != "role" && name != "retired" && name != "comment" {
				return nil, errors.New("metadata: unknown/aliased trust key field")
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, errors.New("metadata: null trust key property forbidden")
			}
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p TrustPolicy
	if err := dec.Decode(&p); err != nil {
		return nil, errors.New("metadata: malformed trust policy")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("metadata: trailing trust policy data")
	}
	// Legacy omitted IDs were derived by LoadKeys; retain only that migration.
	if p.Schema == 1 {
		for i := range p.Keys {
			if pub, err := hex.DecodeString(p.Keys[i].Public); err == nil {
				p.Keys[i].Public = hex.EncodeToString(pub)
			}
			if p.Keys[i].KeyID == "" {
				if pub, err := hex.DecodeString(p.Keys[i].Public); err == nil && len(pub) == ed25519.PublicKeySize {
					p.Keys[i].KeyID = KeyIDFor(ed25519.PublicKey(pub))
				}
			}
		}
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *TrustPolicy) Scoped() *TrustPolicy {
	out := &TrustPolicy{Schema: 2}
	if p == nil {
		return out
	}
	out.Keys = slices.Clone(p.Keys)
	out.Delivery = p.Delivery.Clone()
	for i := range out.Keys {
		if p.Schema == 1 {
			out.Keys[i].Role = RoleProfile
		}
	}
	return out
}

// VerificationKeys returns an owned role-specific map. Retired profile keys
// are exposed separately only to authenticate the exact installed history.
func (p *TrustPolicy) VerificationKeys(role KeyRole) map[string]ed25519.PublicKey {
	return p.keysForRole(role, false)
}

func (p *TrustPolicy) historicalProfileKeys() map[string]ed25519.PublicKey {
	return p.keysForRole(RoleProfile, true)
}

func (p *TrustPolicy) keysForRole(role KeyRole, history bool) map[string]ed25519.PublicKey {
	keys := map[string]ed25519.PublicKey{}
	if p.validate() != nil || !validRole(role) {
		return keys
	}
	for _, key := range p.Keys {
		keyRole := key.Role
		if p.Schema == 1 {
			keyRole = RoleProfile
		}
		if keyRole != role || key.Retired && !history {
			continue
		}
		pub, _ := hex.DecodeString(key.Public) // validated above, owned allocation
		keys[key.KeyID] = ed25519.PublicKey(pub)
	}
	return keys
}

func (p *TrustPolicy) WithKey(pub ed25519.PublicKey, role KeyRole) (*TrustPolicy, error) {
	if p.validate() != nil || !validRole(role) || len(pub) != ed25519.PublicKeySize {
		return nil, ErrTrustRole
	}
	out := p.Scoped()
	id := KeyIDFor(pub)
	for _, key := range out.Keys {
		if key.KeyID == id {
			return nil, errors.New("metadata: trust key already exists; role promotion/revival forbidden")
		}
	}
	out.Keys = append(out.Keys, TrustKey{KeyID: id, Public: hex.EncodeToString(pub), Role: role})
	if err := out.validate(); err != nil {
		return nil, err
	}
	return out, nil
}

func EncodeTrustPolicy(p *TrustPolicy) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	out := p.Scoped()
	if out.Keys == nil {
		out.Keys = []TrustKey{}
	}
	sort.Slice(out.Keys, func(i, j int) bool { return out.Keys[i].KeyID < out.Keys[j].KeyID })
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil || len(data) > MaxTrustPolicySize {
		return nil, errors.New("metadata: trust policy encoding exceeds bound")
	}
	return data, nil
}

func (p *TrustPolicy) Digest() (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	out := p.Scoped()
	retainedDelivery := out.Delivery
	out.Delivery = nil // rotating freshness/cache is not local signing authority
	for i := range out.Keys {
		out.Keys[i].Comment = "" // comments are not authority
	}
	data, err := EncodeTrustPolicy(out)
	if err != nil {
		return "", err
	}
	if retainedDelivery != nil {
		rootHash := sha256.Sum256(retainedDelivery.Root)
		data, err = json.Marshal(struct {
			Policy json.RawMessage `json:"policy"`
			Anchor string          `json:"anchor_sha256"`
			Root   string          `json:"root_sha256"`
		}{data, retainedDelivery.AnchorSHA256, hex.EncodeToString(rootHash[:])})
		if err != nil {
			return "", err
		}
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ReadTrustPolicyFile never reads a private key. No caller may treat an I/O or
// parse error as an absent/empty authority. Native reader checks and original
// Close precede decoding; Windows/runtime compatibility still requires acceptance.
func ReadTrustPolicyFile(path string) (*TrustPolicy, bool, error) {
	data, present, err := storedPolicyFile.read(path)
	if err != nil {
		return nil, present, err
	}
	if !present {
		return &TrustPolicy{Schema: 2}, false, nil
	}
	p, err := DecodeTrustPolicy(data)
	return p, true, err
}

func (s *Store) LoadTrustPolicy() (*TrustPolicy, bool, error) {
	p, present, _, err := s.loadTrustSource()
	return p, present, err
}

// Caller supplies coherent source ownership when combining these values with
// another file. Individual loaders do not recursively acquire a native scope.
func (s *Store) loadTrustSource() (*TrustPolicy, bool, *AcceptedProfileState, error) {
	if err := filetxn.Check(s.ImportJournal()); err != nil {
		return nil, false, nil, fmt.Errorf("%w: %w", ErrTrustRole, err)
	}
	p, present, err := ReadTrustPolicyFile(s.KeysPath())
	if err != nil {
		return nil, present, nil, fmt.Errorf("%w: %w", ErrTrustRole, err)
	}
	state, err := LoadAcceptedProfile(s.Dir)
	if err == nil {
		err = CheckDeliveryBinding(p, state)
	}
	if err != nil {
		return nil, present, nil, fmt.Errorf("%w: %w", ErrTrustRole, err)
	}
	return p, present, state, nil
}

func (s *Store) LoadRoleKeys(role KeyRole) (map[string]ed25519.PublicKey, bool, error) {
	p, present, err := s.LoadTrustPolicy()
	if err != nil {
		return nil, present, err
	}
	if !validRole(role) {
		return nil, present, ErrTrustRole
	}
	return p.VerificationKeys(role), present, nil
}
