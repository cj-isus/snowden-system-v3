package onboarding

import (
	"bytes"
	"encoding/json"

	"github.com/snowden-system/windows/backend/delivery"
)

const pinPolicyCapsulePurpose = "snowden-system/pin-policy/v1"
const MaxPinPolicyCapsuleSize = delivery.MaxEncodedPackageBytes + ((delivery.MaxPinPolicyBytes+2)/3)*4 + 512

// PinPolicyCapsule is public transport data only. It carries no profile,
// credential, application key or root-enrollment approval. Parsing is not proof.
type PinPolicyCapsule struct {
	Policy  []byte
	Package *delivery.Package
}

func DecodePinPolicyCapsule(raw []byte) (*PinPolicyCapsule, error) {
	if len(raw) == 0 || len(raw) > MaxPinPolicyCapsuleSize {
		return nil, ErrTooLarge
	}
	if err := checkPublicJSON(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 4 {
		return nil, ErrBadFormat
	}
	for _, name := range []string{"schema", "purpose", "policy", "package"} {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrBadFormat
		}
	}
	var schema int
	var purpose string
	if json.Unmarshal(fields["schema"], &schema) != nil || schema != 1 || json.Unmarshal(fields["purpose"], &purpose) != nil || purpose != pinPolicyCapsulePurpose {
		return nil, ErrBadFormat
	}
	wire := bytes.TrimSpace(fields["policy"])
	if len(wire) < 3 || len(wire) > ((delivery.MaxPinPolicyBytes+2)/3)*4+2 || wire[0] != '"' {
		return nil, ErrTooLarge // bound before base64 decode; no numeric-array alternate
	}
	var policy []byte
	if json.Unmarshal(wire, &policy) != nil || len(policy) == 0 || len(policy) > delivery.MaxPinPolicyBytes {
		return nil, ErrBadFormat
	}
	pack, err := delivery.DecodePackage(fields["package"])
	if err != nil {
		return nil, err
	}
	return &PinPolicyCapsule{Policy: policy, Package: pack}, nil
}

func EncodePinPolicyCapsule(policy []byte, pack *delivery.Package) ([]byte, error) {
	if pack == nil {
		return nil, ErrBadFormat
	}
	if err := (delivery.Request{Kind: delivery.PinPolicy, Path: delivery.PinPolicyPath, Bytes: policy}).Validate(); err != nil {
		return nil, err
	}
	policy = bytes.Clone(policy)
	encoded, err := pack.Encode() // owns/bounds every public metadata record
	if err != nil {
		return nil, err
	}
	wire := struct {
		Schema  int             `json:"schema"`
		Purpose string          `json:"purpose"`
		Policy  []byte          `json:"policy"`
		Package json.RawMessage `json:"package"`
	}{1, pinPolicyCapsulePurpose, policy, encoded}
	raw, err := json.Marshal(wire)
	if err != nil || len(raw) > MaxPinPolicyCapsuleSize {
		return nil, ErrTooLarge
	}
	return raw, nil
}
