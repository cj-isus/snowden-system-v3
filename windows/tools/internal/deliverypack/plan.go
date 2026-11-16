// Package deliverypack assembles public delivery files for the two existing
// publisher CLIs. It does not sign metadata, enroll a client, or read secrets.
package deliverypack

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

const maxPlanBytes = 32 << 10

type Plan struct {
	Schema     int               `json:"schema"`
	Kind       delivery.Kind     `json:"kind"`
	Target     string            `json:"target_path"`
	Artifact   string            `json:"artifact"`
	Authority  string            `json:"authority"`
	Metadata   map[string]string `json:"metadata"`
	EpochGrant string            `json:"epoch_grant,omitempty"`
	PinPolicy  string            `json:"pin_policy,omitempty"`
}

func loadPlan(filename string, kind delivery.Kind) (*Plan, string, error) {
	data, err := publisherfiles.Read(filename, maxPlanBytes)
	if err != nil {
		return nil, "", err
	}
	object, err := strictObject(data, 8)
	if err != nil || len(object) < 6 {
		return nil, "", errors.New("delivery-pack: six base plan properties required")
	}
	for _, name := range []string{"schema", "kind", "target_path", "artifact", "authority", "metadata"} {
		if object[name] == nil {
			return nil, "", errors.New("delivery-pack: required plan property missing")
		}
	}
	for name := range object {
		switch name {
		case "schema", "kind", "target_path", "artifact", "authority", "metadata", "epoch_grant", "pin_policy":
		default:
			return nil, "", errors.New("delivery-pack: unknown/aliased plan property")
		}
	}
	var plan Plan
	if json.Unmarshal(data, &plan) != nil || plan.Schema != 1 || plan.Kind != kind || len(plan.Target) > 160 {
		return nil, "", errors.New("delivery-pack: invalid plan schema/purpose")
	}
	if kind == delivery.PinPolicy && (plan.Target != delivery.PinPolicyPath || object["epoch_grant"] != nil || object["pin_policy"] != nil) {
		return nil, "", errors.New("delivery-pack: policy-only plan requires the reserved artifact without adjuncts")
	}
	if raw, exists := object["epoch_grant"]; exists && (kind != delivery.Profile || json.Unmarshal(raw, &plan.EpochGrant) != nil || !relativeJSON(plan.EpochGrant)) {
		return nil, "", errors.New("delivery-pack: epoch grant requires an explicit public profile .json input")
	}
	if raw, exists := object["pin_policy"]; exists && (json.Unmarshal(raw, &plan.PinPolicy) != nil || !relativeJSON(plan.PinPolicy)) {
		return nil, "", errors.New("delivery-pack: pin policy requires an explicit bounded public .json input")
	}
	records, err := strictObject(object["metadata"], delivery.MaxRecords)
	if err != nil || len(records) == 0 || records["root.json"] == nil {
		return nil, "", errors.New("delivery-pack: explicit metadata map with root.json required")
	}
	for name := range records {
		if !delivery.ValidMetadataName(name) {
			return nil, "", errors.New("delivery-pack: unsupported metadata filename")
		}
	}
	for _, rel := range append([]string{plan.Artifact, plan.Authority}, sortedValues(plan.Metadata)...) {
		if !relativeJSON(rel) {
			return nil, "", errors.New("delivery-pack: bounded portable relative .json path required")
		}
	}
	root, err := filepath.Abs(filepath.Dir(filename))
	return &plan, root, err
}

func strictObject(data []byte, limit int) (map[string]json.RawMessage, error) {
	return publisherfiles.Object(data, limit)
}

func sortedNames[V any](values map[string]V) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, name := range sortedNames(values) {
		out = append(out, values[name])
	}
	return out
}

func relativeJSON(rel string) bool { return publisherfiles.RelativeJSON(rel) }

func readRelative(root, rel string, limit int64) ([]byte, error) {
	return publisherfiles.ReadRelativeJSON(root, rel, limit)
}
