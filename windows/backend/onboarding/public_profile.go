package onboarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/snowden-system/windows/backend/delivery"
)

// Public profiles carry signed descriptors, delivery proof and optional public
// recovery permits only. Credential transfer keeps the existing encrypted path.
// Decoding is not acceptance: App must run its common authority/lifecycle gate.
func DecodePublicProfile(data []byte) (*BundlePayload, error) {
	if len(data) == 0 || len(data) > MaxPublicProfileSize {
		return nil, ErrTooLarge
	}
	if err := checkPublicJSON(data); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return nil, ErrBadFormat
	}
	for name, value := range fields {
		switch name {
		case "schema", "created_at", "device_name", "trusted_keys", "envelope", "recovery", "delivery", "epoch_grant", "pin_policy", "secrets":
		default:
			return nil, ErrBadFormat
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrBadFormat
		}
		if name == "pin_policy" {
			if wire := bytes.TrimSpace(value); len(wire) == 0 || wire[0] != '"' {
				return nil, ErrBadFormat
			}
		}
	}
	var payload BundlePayload
	if err := decodeStrict(data, &payload); err != nil {
		return nil, ErrBadFormat
	}
	if _, exists := fields["pin_policy"]; exists && len(payload.PinPolicy) == 0 || len(payload.PinPolicy) == 0 && len(data) > MaxBundleSize {
		return nil, ErrBadFormat
	}
	if err := validatePublicProfile(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func EncodePublicProfile(payload *BundlePayload) ([]byte, error) {
	if err := validatePublicProfile(payload); err != nil {
		return nil, err
	}
	copy := *payload
	copy.Secrets = []SecretSlot{}
	data, err := json.Marshal(&copy)
	if err != nil {
		return nil, ErrBadFormat
	}
	if len(data) > MaxPublicProfileSize {
		return nil, ErrTooLarge
	}
	return data, nil
}

func validatePublicProfile(payload *BundlePayload) error {
	if payload == nil || payload.Schema != 1 || payload.CreatedAt.IsZero() || payload.Delivery == nil || len(payload.Secrets) != 0 || len(payload.TrustedKeys) == 0 {
		return errors.New("onboarding: public profile requires delivery and forbids credential slots")
	}
	if len(payload.PinPolicy) > delivery.MaxPinPolicyBytes || !payloadSizeAllowed(payload) {
		return ErrTooLarge
	}
	core := *payload
	core.PinPolicy = nil
	raw, err := json.Marshal(&core)
	if err != nil || len(raw) > MaxBundleSize {
		return ErrTooLarge // ordinary profile/metadata/key bounds are not enlarged
	}
	_, err = payload.ExactProfileBytes()
	return err
}

// Reject duplicate/case-aliased fields before encoding/json's field folding.
// Raw profile validation and TUF verification still have their own stricter gates.
func checkPublicJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 24 {
			return ErrBadFormat
		}
		token, err := dec.Token()
		if err != nil {
			return ErrBadFormat
		}
		delim, container := token.(json.Delim)
		if !container {
			return nil
		}
		if delim != '{' && delim != '[' {
			return ErrBadFormat
		}
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				token, err := dec.Token()
				name, ok := token.(string)
				if err != nil || !ok || seen[strings.ToLower(name)] {
					return ErrBadFormat
				}
				seen[strings.ToLower(name)] = true
			}
			if err := walk(depth + 1); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return ErrBadFormat
		}
		return nil
	}
	if err := walk(0); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return ErrBadFormat
	}
	return nil
}
