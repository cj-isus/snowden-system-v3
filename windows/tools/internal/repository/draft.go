package repository

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"sort"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

// DraftPlan contains public inputs only. Exact previous/parent byte approvals
// are separate CLI arguments and cannot be supplied as authority by this plan.
type DraftPlan struct {
	Schema   int                 `json:"schema"`
	Role     string              `json:"role"`
	Version  int64               `json:"version"`
	Expires  time.Time           `json:"expires"`
	Parent   string              `json:"parent"`
	Previous string              `json:"previous"`
	Roles    map[string]RoleKeys `json:"roles"`
	Targets  map[string]string   `json:"targets"`
	Metadata map[string]string   `json:"metadata"`
}

type RoleKeys struct {
	Threshold int      `json:"threshold"`
	Public    []string `json:"public"`
}

func loadDraftPlan(filename string) (*DraftPlan, string, error) {
	raw, err := publisherfiles.Read(filename, 32<<10)
	if err != nil {
		return nil, "", err
	}
	fields, err := publisherfiles.Object(raw, 9)
	if err != nil || len(fields) != 9 {
		return nil, "", errors.New("repository: exact nine-field public draft plan required")
	}
	for _, name := range []string{"schema", "role", "version", "expires", "parent", "previous", "roles", "targets", "metadata"} {
		if fields[name] == nil {
			return nil, "", errors.New("repository: draft field missing/aliased")
		}
	}
	roles, err := publisherfiles.Object(fields["roles"], 4)
	if err != nil {
		return nil, "", err
	}
	for _, rawRole := range roles {
		object, err := publisherfiles.Object(rawRole, 2)
		if err != nil || len(object) != 2 || object["threshold"] == nil || object["public"] == nil {
			return nil, "", errors.New("repository: exact threshold/public role required")
		}
	}
	if _, err = publisherfiles.Object(fields["targets"], 256); err != nil {
		return nil, "", err
	}
	if _, err = publisherfiles.Object(fields["metadata"], 3); err != nil {
		return nil, "", err
	}
	var plan DraftPlan
	if json.Unmarshal(raw, &plan) != nil || plan.Schema != 1 || plan.Version < 1 || plan.Expires.IsZero() {
		return nil, "", errors.New("repository: invalid draft plan")
	}
	base, err := filepath.Abs(filepath.Dir(filename))
	return &plan, base, err
}

func draft(filename, parentApproval, previousApproval, output string, now time.Time) ([]byte, error) {
	plan, base, err := loadDraftPlan(filename)
	if err != nil {
		return nil, err
	}
	d := &document{role: plan.Role}
	switch plan.Role {
	case tuf.ROOT:
		if len(plan.Roles) != 4 || len(plan.Targets) != 0 || len(plan.Metadata) != 0 {
			return nil, errors.New("repository: root requires exactly four roles and no artifacts")
		}
		d.root = tuf.Root(plan.Expires)
		d.root.Signed.Version = plan.Version
		for _, name := range []string{tuf.ROOT, tuf.TARGETS, tuf.SNAPSHOT, tuf.TIMESTAMP} {
			keys, err := roleKeys(plan.Roles[name])
			if err != nil {
				return nil, err
			}
			d.root.Signed.Roles[name].Threshold = plan.Roles[name].Threshold
			for _, key := range keys {
				if err = d.root.Signed.AddKey(key, name); err != nil {
					return nil, err
				}
			}
		}
	case tuf.TARGETS, "profiles", "updates":
		if len(plan.Metadata) != 0 {
			return nil, errors.New("repository: targets role cannot include metadata links")
		}
		d.targets = tuf.Targets(plan.Expires)
		d.targets.Signed.Version = plan.Version
		if plan.Role == tuf.TARGETS {
			if len(plan.Roles) != 2 {
				return nil, errors.New("repository: profiles and updates delegations required")
			}
			d.targets.Signed.Delegations = &tuf.Delegations{Keys: map[string]*tuf.Key{}, Roles: []tuf.DelegatedRole{}}
			for _, name := range []string{"profiles", "updates"} {
				keys, err := roleKeys(plan.Roles[name])
				if err != nil {
					return nil, err
				}
				d.targets.Signed.Delegations.Roles = append(d.targets.Signed.Delegations.Roles, tuf.DelegatedRole{Name: name, Threshold: plan.Roles[name].Threshold, Terminating: true, Paths: []string{name + "/*"}, KeyIDs: []string{}})
				for _, key := range keys {
					if err = d.targets.Signed.AddKey(key, name); err != nil {
						return nil, err
					}
				}
			}
		} else if len(plan.Roles) != 0 {
			return nil, errors.New("repository: delegated role cannot grant authority")
		}
		if err = addTargets(d, plan, base, now); err != nil {
			return nil, err
		}
	case tuf.SNAPSHOT:
		if len(plan.Roles) != 0 || len(plan.Targets) != 0 || len(plan.Metadata) != 3 {
			return nil, errors.New("repository: snapshot requires exactly three target-role links")
		}
		d.snapshot = tuf.Snapshot(plan.Expires)
		d.snapshot.Signed.Version = plan.Version
	case tuf.TIMESTAMP:
		if len(plan.Roles) != 0 || len(plan.Targets) != 0 || len(plan.Metadata) != 1 || plan.Metadata["snapshot.json"] == "" {
			return nil, errors.New("repository: timestamp requires exactly snapshot.json")
		}
		d.timestamp = tuf.Timestamp(plan.Expires)
		d.timestamp.Signed.Version = plan.Version
	default:
		return nil, errors.New("repository: unsupported draft role")
	}
	if err = d.fresh(now); err != nil {
		return nil, err
	}
	parent, err := loadParent(base, plan.Parent, parentApproval, d, now)
	if err != nil {
		return nil, err
	}
	if err = checkPrevious(plan, base, previousApproval, parent, d); err != nil {
		return nil, err
	}
	if len(plan.Metadata) != 0 {
		if err = addMetadata(d, plan, base, parent, now); err != nil {
			return nil, err
		}
	}
	out, err := d.encode()
	if err != nil {
		return nil, err
	}
	return publishDocument(d, parent, out, false, output, now)
}

func roleKeys(role RoleKeys) ([]*tuf.Key, error) {
	if role.Threshold < 1 || len(role.Public) > 32 || role.Threshold > len(role.Public) {
		return nil, errors.New("repository: invalid role threshold/public count")
	}
	keys := []*tuf.Key{}
	seen := map[string]bool{}
	pubs := append([]string(nil), role.Public...)
	sort.Strings(pubs)
	for _, value := range pubs {
		pub, err := hex.DecodeString(value)
		if err != nil || len(pub) != ed25519.PublicKeySize || hex.EncodeToString(pub) != value || seen[value] {
			return nil, errors.New("repository: distinct canonical Ed25519 public keys required")
		}
		key, err := tuf.KeyFromPublicKey(ed25519.PublicKey(pub))
		if err != nil {
			return nil, err
		}
		seen[value] = true
		keys = append(keys, key)
	}
	return keys, nil
}

func checkPrevious(plan *DraftPlan, base, approval string, parent, next *document) error {
	if plan.Previous == "" {
		if next.version() != 1 || approval != "first" {
			return errors.New("repository: version1 requires explicit first; otherwise previous signed bytes required")
		}
		return nil
	}
	raw, err := publisherfiles.ReadRelativeJSON(base, plan.Previous, delivery.MaxMetadataBytes)
	if err != nil {
		return err
	}
	if !approved(raw, approval) {
		return errors.New("repository: independently approved previous SHA256 required")
	}
	previous, err := decodeDocument(next.role, raw)
	if err != nil {
		return err
	}
	if len(previous.signatures()) == 0 || previous.version() == math.MaxInt64 || next.version() != previous.version()+1 {
		return errors.New("repository: version must increment finalized previous by exactly one")
	}
	if parent == nil {
		return delivery.ErrTrust
	}
	if next.root != nil {
		if plan.Previous != plan.Parent || digest(raw) != parent.sourceSHA256 {
			return errors.New("repository: root predecessor must be its exact approved parent")
		}
		return nil // loadParent checked predecessor self threshold and +1
	}
	// Delegation rotations use overlap until the previous role is verifiable
	// under the new parent. Actual clients retain their independent floors.
	return parent.verify(previous)
}

func addTargets(d *document, plan *DraftPlan, base string, now time.Time) error {
	names := make([]string, 0, len(plan.Targets))
	for name := range plan.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	remaining := delivery.MaxPackageBytes
	for _, name := range names {
		kind, limit := delivery.Profile, delivery.MaxProfileBytes
		if d.role == "updates" {
			kind, limit = delivery.UpdateManifest, delivery.MaxManifestBytes
		}
		if d.role == tuf.TARGETS {
			switch name {
			case delivery.EpochPolicyPath:
				kind, limit = delivery.ProfileEpoch, delivery.MaxEpochGrantBytes
			case delivery.PinPolicyPath:
				kind, limit = delivery.PinPolicy, delivery.MaxPinPolicyBytes
			default:
				return delivery.ErrTrust
			}
		}
		raw, err := publisherfiles.ReadRelativeJSON(base, plan.Targets[name], int64(min(remaining, limit)))
		if err != nil {
			return err
		}
		remaining -= len(raw)
		if err = (delivery.Request{Kind: kind, Path: name, Bytes: raw}).Validate(); err != nil {
			return err
		}
		if kind == delivery.ProfileEpoch {
			grant, err := metadata.DecodeEpochGrant(raw)
			if err != nil {
				return err
			}
			if err = grant.Validate(now); err != nil {
				return err
			}
		}
		if kind == delivery.PinPolicy {
			pin, err := metadata.DecodePinPolicy(raw)
			if err != nil {
				return err
			}
			if err = pin.Validate(now); err != nil {
				return err
			}
		}
		target, err := tuf.TargetFile().FromBytes(name, raw, "sha256")
		if err != nil {
			return err
		}
		d.targets.Signed.Targets[name] = target
	}
	return nil
}

func addMetadata(d *document, plan *DraftPlan, base string, parent *document, now time.Time) error {
	if parent == nil || parent.root == nil {
		return delivery.ErrTrust
	}
	roles := []string{tuf.TARGETS, "profiles", "updates"}
	if d.timestamp != nil {
		roles = []string{tuf.SNAPSHOT}
	}
	docs := map[string]*document{}
	links := map[string]*tuf.MetaFiles{}
	for _, role := range roles {
		raw, err := publisherfiles.ReadRelativeJSON(base, plan.Metadata[role+".json"], delivery.MaxMetadataBytes)
		if err != nil {
			return err
		}
		child, err := decodeDocument(role, raw)
		if err != nil {
			return err
		}
		if err = child.fresh(now); err != nil {
			return err
		}
		authority := parent
		if role == "profiles" || role == "updates" {
			authority = docs[tuf.TARGETS]
		}
		if authority == nil {
			return delivery.ErrTrust
		}
		if err = authority.verify(child); err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		links[role+".json"] = &tuf.MetaFiles{Version: child.version(), Length: int64(len(raw)), Hashes: tuf.Hashes{"sha256": sum[:]}}
		docs[role] = child
	}
	if d.snapshot != nil {
		d.snapshot.Signed.Meta = links
	} else {
		d.timestamp.Signed.Meta = links
	}
	return nil
}
