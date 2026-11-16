package deliverypack

import (
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
)

// A policy-only publication proves only the reserved offline target. It never
// loads a profile/update payload or claims application-role acceptance.
func assemblePolicyOnly(plan *Plan, records map[string][]byte, raw []byte, pack *delivery.Package, state *delivery.State, proof *delivery.Prepared, policy *metadata.TrustPolicy, approvedRoot string, rootExpiry, now time.Time) (*assembled, error) {
	checking := policy.Scoped()
	checking.Delivery = state.Clone()
	pin, err := metadata.PreparePinPolicy(checking, proof, raw, now)
	if err != nil {
		return nil, err
	}
	retained, err := pin.Retain(checking, nil, now)
	if err != nil {
		return nil, err
	}
	encoded, err := onboarding.EncodePinPolicyCapsule(raw, pack)
	if err != nil {
		return nil, err
	}
	deadline := pin.ValidUntil()
	if rootExpiry.Before(deadline) {
		deadline = rootExpiry.UTC()
	}
	out := &assembled{records: records, data: encoded, artifact: raw, proof: proof, pinRaw: raw, pinPolicy: pin, pinAuthority: checking, report: Report{
		Schema: 1, Kind: delivery.PinPolicy, TargetPath: plan.Target, ArtifactSHA256: digest(raw), ArtifactBytes: len(raw),
		ApprovedIssuerRootSHA256: approvedRoot, IssuerRootExpiresAt: rootExpiry.UTC(), RootVersion: proof.RootVersion(),
		DeliveryValidUntil: deadline, IssuerObservedAt: now, OutputBytes: len(encoded), RequiresClientAcceptance: true,
		PinPolicySHA256: retained.SHA256, PinPolicyVersion: fmt.Sprint(retained.Version), Metadata: []RecordSummary{},
	}}
	for _, name := range sortedNames(records) {
		value := records[name]
		out.report.Metadata = append(out.report.Metadata, RecordSummary{Name: name, Bytes: len(value), SHA256: digest(value)})
	}
	return out, nil
}
