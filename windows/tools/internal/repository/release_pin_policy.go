package repository

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/snowden-system/windows/backend/delivery"
)

// Resolve bounded public bytes only. Unverified metadata is a lookup hint;
// complete SDK verification below still supplies the offline target authority.
// This also supports a release with updates and no profile transfer to carry it.
func restorePinPolicy(c *releaseCandidate, wire *releaseWire) ([]byte, error) {
	var top *document
	for name, raw := range c.records {
		if name != "targets.json" && !strings.HasSuffix(name, ".targets.json") {
			continue
		}
		d, _, err := metadataRecord(name, raw)
		if err != nil || d.targets == nil {
			return nil, delivery.ErrTrust
		}
		if top != nil && (top.version() != d.version() || top.sourceSHA256 != d.sourceSHA256) {
			return nil, errors.New("repository: conflicting top-level policy metadata")
		}
		top = d
	}
	if top == nil {
		return nil, delivery.ErrTrust
	}
	info, present := top.targets.Signed.Targets[delivery.PinPolicyPath]
	if !present {
		return nil, nil
	}
	if info == nil || len(info.Hashes["sha256"]) != 32 {
		return nil, delivery.ErrTrust
	}
	raw, err := candidateArtifact(wire, ReleaseArtifact{Kind: delivery.PinPolicy, Target: delivery.PinPolicyPath, SHA256: hex.EncodeToString(info.Hashes["sha256"])})
	if err != nil {
		return nil, err
	}
	c.artifacts[delivery.PinPolicyPath] = releaseArtifact{kind: delivery.PinPolicy, raw: raw}
	return raw, nil
}
