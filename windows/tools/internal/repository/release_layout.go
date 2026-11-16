package repository

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

func portablePathKey(name string) string { return strings.ToLower(name) }

func metadataRecord(name string, raw []byte) (*document, string, error) {
	if !delivery.ValidMetadataName(name) {
		return nil, "", delivery.ErrTrust
	}
	parts := strings.Split(name, ".")
	role := parts[len(parts)-2]
	d, err := decodeDocument(role, raw)
	if err != nil {
		return nil, "", err
	}
	if len(d.signatures()) == 0 {
		return nil, "", errors.New("repository: unsigned release metadata")
	}
	if len(parts) == 3 {
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version != d.version() || strconv.FormatInt(version, 10) != parts[0] {
			return nil, "", errors.New("repository: metadata filename/version mismatch")
		}
	}
	name = fmt.Sprintf("metadata/%d.%s.json", d.version(), role)
	if role == tuf.TIMESTAMP {
		name = "metadata/timestamp.json"
	}
	return d, name, nil
}

func (c *releaseCandidate) buildLayout(now time.Time) error {
	policyOnly := c.wire.Manifest.Purpose == delivery.PinPolicy
	if c.wire.Manifest.Purpose != "" && !policyOnly || policyOnly && (len(c.artifacts) != 1 || c.artifacts[delivery.PinPolicyPath].kind != delivery.PinPolicy || len(c.wire.Manifest.Artifacts) != 1 || c.wire.Manifest.Artifacts[0].Kind != delivery.PinPolicy || c.wire.Manifest.Artifacts[0].Target != delivery.PinPolicyPath) {
		return errors.New("repository: explicit publication purpose and artifact inventory differ")
	}
	if !approved(c.records["root.json"], c.wire.Manifest.ApprovedRootSHA256) {
		return delivery.ErrTrust
	}
	sources := map[string]*document{}
	roots := map[int64]*document{}
	rootBytes := map[int64][]byte{}
	names := make([]string, 0, len(c.records))
	for name := range c.records {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := c.records[name]
		d, filename, err := metadataRecord(name, raw)
		if err != nil {
			return err
		}
		if err = c.addFile(filename, raw); err != nil {
			return err
		}
		c.wire.Manifest.SourceMetadata[name] = filename
		sources[name] = d
		if d.root != nil {
			if err = d.verify(d); err != nil {
				return err
			}
			if old := rootBytes[d.version()]; old != nil && !bytes.Equal(old, raw) {
				return errors.New("repository: conflicting root at one version")
			}
			roots[d.version()], rootBytes[d.version()] = d, raw
		}
	}
	versions := make([]int64, 0, len(roots))
	for version := range roots {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	if len(versions) == 0 || versions[len(versions)-1] != c.wire.Manifest.RootVersion {
		return errors.New("repository: root inventory differs from verified current root")
	}
	for i := 1; i < len(versions); i++ {
		previous, next := versions[i-1], versions[i]
		if next != previous+1 {
			return errors.New("repository: incomplete root rotation inventory")
		}
		if err := roots[previous].verify(roots[next]); err != nil {
			return err
		}
	}
	currentRoot := roots[c.wire.Manifest.RootVersion]
	currentRootBytes := rootBytes[c.wire.Manifest.RootVersion]
	if currentRoot == nil {
		return delivery.ErrTrust
	}
	if err := c.addFile("metadata/root.json", currentRootBytes); err != nil {
		return err
	}
	c.wire.Manifest.PublishedRootSHA256 = digest(currentRootBytes)
	stamp := sources["timestamp.json"]
	if stamp == nil || stamp.timestamp == nil || len(stamp.timestamp.Signed.Meta) != 1 {
		return errors.New("repository: exact current timestamp required")
	}
	if err := currentRoot.verify(stamp); err != nil {
		return err
	}
	snapshot, err := c.linked(tuf.SNAPSHOT, stamp.timestamp.Signed.Meta["snapshot.json"])
	if err != nil {
		return err
	}
	if err = currentRoot.verify(snapshot); err != nil {
		return err
	}
	if len(snapshot.snapshot.Signed.Meta) != 3 {
		return errors.New("repository: snapshot must cover exact target-role inventory")
	}
	top, err := c.linked(tuf.TARGETS, snapshot.snapshot.Signed.Meta["targets.json"])
	if err != nil {
		return err
	}
	if err = currentRoot.verify(top); err != nil {
		return err
	}
	current := map[string]*document{tuf.ROOT: currentRoot, tuf.TIMESTAMP: stamp, tuf.SNAPSHOT: snapshot, tuf.TARGETS: top}
	for _, role := range []string{"profiles", "updates"} {
		d, err := c.linked(role, snapshot.snapshot.Signed.Meta[role+".json"])
		if err != nil {
			return err
		}
		if err = top.verify(d); err != nil {
			return err
		}
		current[role] = d
	}
	for _, d := range current {
		if err = d.fresh(now); err != nil {
			return err
		}
		c.wire.Manifest.ValidUntil = earlier(c.wire.Manifest.ValidUntil, d.expires())
	}
	for _, d := range sources {
		if d.root != nil {
			continue
		}
		live := current[d.role]
		if live == nil || live.version() != d.version() || live.sourceSHA256 != d.sourceSHA256 {
			return errors.New("repository: stale/mixed unused metadata in release source")
		}
	}
	seen := map[string]bool{}
	for _, role := range []string{tuf.TARGETS, "profiles", "updates"} {
		d := current[role]
		targets := make([]string, 0, len(d.targets.Signed.Targets))
		for name := range d.targets.Signed.Targets {
			targets = append(targets, name)
		}
		sort.Strings(targets)
		for _, name := range targets {
			// All metadata signatures/links/freshness were checked above. An
			// emergency release accepts only this offline target's payload; it
			// neither loads nor represents acceptance of other application bytes.
			if policyOnly && (role != tuf.TARGETS || name != delivery.PinPolicyPath) {
				continue
			}
			artifact, exists := c.artifacts[name]
			if !exists || seen[name] {
				return errors.New("repository: every declared target needs exactly one artifact")
			}
			kind := delivery.Profile
			if role == "updates" {
				kind = delivery.UpdateManifest
			} else if role == tuf.TARGETS {
				switch name {
				case delivery.EpochPolicyPath:
					kind = delivery.ProfileEpoch
				case delivery.PinPolicyPath:
					kind = delivery.PinPolicy
				default:
					return delivery.ErrTrust
				}
			}
			if artifact.kind != kind {
				return delivery.ErrTrust
			}
			if err = (delivery.Request{Kind: kind, Path: name, Bytes: artifact.raw}).Validate(); err != nil {
				return err
			}
			target := d.targets.Signed.Targets[name]
			if target == nil || len(target.Hashes) == 0 || len(target.Hashes) > 2 {
				return delivery.ErrTrust
			}
			if err = target.VerifyLengthHashes(artifact.raw); err != nil {
				return err
			}
			// The pinned SDK chooses a hash from the map for remote naming.
			// Include every declared supported alias, never guess map order.
			for algorithm, value := range target.Hashes {
				if algorithm != "sha256" && algorithm != "sha512" || algorithm == "sha256" && len(value) != 32 || algorithm == "sha512" && len(value) != 64 {
					return errors.New("repository: unsupported target naming hash")
				}
				filename := path.Join("targets", path.Dir(name), hex.EncodeToString(value)+"."+path.Base(name))
				if err = c.addFile(filename, artifact.raw); err != nil {
					return err
				}
			}
			seen[name] = true
		}
	}
	if len(seen) != len(c.artifacts) {
		return errors.New("repository: release contains an undeclared artifact")
	}
	return nil
}

func (c *releaseCandidate) linked(role string, link *tuf.MetaFiles) (*document, error) {
	if link == nil || link.Version < 1 || link.Length < 1 || len(link.Hashes) == 0 {
		return nil, errors.New("repository: exact version/length/hash metadata link required")
	}
	filename := fmt.Sprintf("metadata/%d.%s.json", link.Version, role)
	raw := c.wire.Files[filename]
	if len(raw) == 0 {
		return nil, errors.New("repository: linked finalized metadata missing")
	}
	if err := link.VerifyLengthHashes(raw); err != nil {
		return nil, err
	}
	d, err := decodeDocument(role, raw)
	if err != nil {
		return nil, err
	}
	if d.version() != link.Version {
		return nil, delivery.ErrTrust
	}
	return d, nil
}
