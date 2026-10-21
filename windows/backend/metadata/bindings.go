package metadata

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

const MaxBindingMigrationSize = 256 << 10

// BindingMigration is public publisher data, never a list of secret values.
// Channel/role entries use arrays so duplicate assignments can be rejected.
type BindingMigration struct {
	Schema          int               `json:"schema"`
	MetadataVersion uint64            `json:"metadata_version"`
	IssuedAt        time.Time         `json:"issued_at"`
	ExpiresAt       time.Time         `json:"expires_at"`
	Channels        []ChannelBindings `json:"channels"`
}

type ChannelBindings struct {
	ChannelID string              `json:"channel_id"`
	Secrets   []CredentialBinding `json:"secrets"`
}

type CredentialBinding struct {
	Ref      string `json:"ref"`
	SecretID string `json:"secret_id"`
}

func DecodeBindingMigration(data []byte) (*BindingMigration, error) {
	if len(data) > MaxBindingMigrationSize {
		return nil, errors.New("migration: plan too large")
	}
	if err := checkMigrationJSON(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var plan BindingMigration
	if err := dec.Decode(&plan); err != nil {
		return nil, fmt.Errorf("migration: decode plan: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, errors.New("migration: trailing plan data")
	}
	return &plan, nil
}

// MigrateLegacyBindings returns an unsigned copy. It authenticates the source,
// requires every non-revoked legacy channel to be explicitly bound and leaves
// existing exact bindings intact. It neither opens a vault nor selects by kind.
// The caller must sign again; ID existence/type is checked by staged import.
func MigrateLegacyBindings(source *Envelope, plan *BindingMigration, keys map[string]ed25519.PublicKey, now time.Time) (*Envelope, error) {
	if source == nil || plan == nil {
		return nil, errors.New("migration: missing source or plan")
	}
	if err := source.Verify(keys); err != nil {
		return nil, fmt.Errorf("migration: source signature: %w", err)
	}
	if source.MetadataVersion == 0 {
		return nil, errors.New("migration: invalid source version")
	}
	// An expired but authentic source can be renewed explicitly. Validate its
	// structure at issuance; new validity must pass against the current clock.
	if err := source.Validate(source.IssuedAt, 0); err != nil {
		return nil, fmt.Errorf("migration: source shape: %w", err)
	}
	if plan.Schema != 1 {
		return nil, errors.New("migration: unsupported plan schema")
	}
	if plan.MetadataVersion <= source.MetadataVersion {
		return nil, errors.New("migration: version must increase")
	}
	if !plan.ExpiresAt.After(now) {
		return nil, errors.New("migration: new expiry must be future")
	}
	if len(plan.Channels) == 0 || len(plan.Channels) > len(source.Channels) {
		return nil, errors.New("migration: invalid channel count")
	}
	byID := make(map[string]Channel, len(source.Channels))
	revoked := make(map[string]bool, len(source.Revocations))
	for _, id := range source.Revocations {
		revoked[id] = true
	}
	for _, ch := range source.Channels {
		byID[ch.ID] = ch
	}
	assignments := make(map[string]map[string]string, len(plan.Channels))
	for _, entry := range plan.Channels {
		ch, ok := byID[entry.ChannelID]
		if !ok {
			return nil, errors.New("migration: unknown channel")
		}
		if revoked[ch.ID] {
			return nil, errors.New("migration: revoked channel")
		}
		if len(ch.SecretIDs) != 0 {
			return nil, errors.New("migration: existing bindings are immutable")
		}
		if _, exists := assignments[ch.ID]; exists {
			return nil, errors.New("migration: duplicate channel")
		}
		if len(entry.Secrets) == 0 || len(entry.Secrets) != len(ch.CredentialRefs) {
			return nil, errors.New("migration: incomplete role assignments")
		}
		ids := make(map[string]string, len(entry.Secrets))
		for _, binding := range entry.Secrets {
			if _, exists := ids[binding.Ref]; exists {
				return nil, errors.New("migration: duplicate role")
			}
			ids[binding.Ref] = binding.SecretID
		}
		if err := ValidateSecretIDs(ch.CredentialRefs, ids); err != nil {
			return nil, fmt.Errorf("migration: role assignments: %w", err)
		}
		assignments[ch.ID] = ids
	}
	out := *source
	out.Channels = make([]Channel, len(source.Channels))
	for i, ch := range source.Channels {
		out.Channels[i] = ch.Clone()
		if revoked[ch.ID] || len(ch.SecretIDs) != 0 {
			continue
		}
		ids, ok := assignments[ch.ID]
		if !ok {
			return nil, errors.New("migration: missing legacy channel")
		}
		out.Channels[i].SecretIDs = ids
	}
	out.Revocations = slices.Clone(source.Revocations)
	out.SplitDirect = slices.Clone(source.SplitDirect)
	out.MetadataVersion, out.IssuedAt, out.ExpiresAt = plan.MetadataVersion, plan.IssuedAt, plan.ExpiresAt
	out.Signature = ""
	if err := out.Validate(now, source.MetadataVersion); err != nil {
		return nil, fmt.Errorf("migration: new validity or profile: %w", err)
	}
	return &out, nil
}

// An explicit assignment must have one interpretation. Reject duplicate JSON
// properties as well as excessive nesting before decoding into typed structs.
func checkMigrationJSON(data []byte) error {
	type frame struct {
		keys       map[string]bool
		expectsKey bool
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	first, err := dec.Token()
	if err != nil {
		return fmt.Errorf("migration: JSON: %w", err)
	}
	if first != json.Delim('{') {
		return errors.New("migration: plan must be an object")
	}
	stack := []frame{{keys: map[string]bool{}, expectsKey: true}}
	for len(stack) > 0 {
		token, err := dec.Token()
		if err != nil {
			return fmt.Errorf("migration: JSON: %w", err)
		}
		if end, ok := token.(json.Delim); ok && (end == '}' || end == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		parent := &stack[len(stack)-1]
		if parent.keys != nil && parent.expectsKey {
			key, ok := token.(string)
			if !ok {
				return errors.New("migration: invalid JSON key")
			}
			// encoding/json folds struct-field names, including Unicode aliases.
			// Migration properties use their exact lowercase ASCII spelling.
			for i := 0; i < len(key); i++ {
				if !(key[i] >= 'a' && key[i] <= 'z' || key[i] == '_') {
					return errors.New("migration: non-canonical JSON property")
				}
			}
			if parent.keys[key] {
				return errors.New("migration: duplicate JSON property")
			}
			parent.keys[key] = true
			parent.expectsKey = false
			continue
		}
		if parent.keys != nil {
			parent.expectsKey = true
		}
		if start, ok := token.(json.Delim); ok {
			if len(stack) >= 64 {
				return errors.New("migration: JSON nesting too deep")
			}
			next := frame{}
			if start == '{' {
				next.keys = map[string]bool{}
				next.expectsKey = true
			}
			stack = append(stack, next)
		}
	}
	return nil
}
