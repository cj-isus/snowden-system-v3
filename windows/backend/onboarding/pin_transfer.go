package onboarding

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/snowden-system/windows/backend/delivery"
)

// Public adjunct admission is separate from the unchanged secrets/KDF/gzip
// limits. Neither wrapper framing nor its schema confers delivery authority.
const MaxPublicProfileSize = MaxBundleSize + ((delivery.MaxPinPolicyBytes+2)/3)*4 + 128
const MaxProtectedTransferSize = 2 << 20

type pinTransfer struct {
	Schema    int    `json:"schema"`
	Protected string `json:"protected"`
	PinPolicy []byte `json:"pin_policy"`
}

func wrapPinTransfer(protected string, policy []byte) (string, error) {
	if len(protected) == 0 || len(protected) > MaxBundleSize || !strings.HasPrefix(protected, magic+"1.") || len(policy) == 0 || len(policy) > delivery.MaxPinPolicyBytes {
		return "", ErrBadFormat
	}
	raw, err := json.Marshal(pinTransfer{Schema: 2, Protected: protected, PinPolicy: policy})
	if err != nil {
		return "", ErrBadFormat
	}
	if len(magic+"2.")+base64.RawURLEncoding.EncodedLen(len(raw)) > MaxProtectedTransferSize {
		return "", ErrTooLarge
	}
	return magic + "2." + base64.RawURLEncoding.EncodeToString(raw), nil
}

// splitPinTransfer owns decoded adjunct bytes before any expensive KDF. It
// forbids nesting and unknown/null/duplicate/case-aliased wrapper properties.
func splitPinTransfer(transport string) (string, []byte, error) {
	originalSize := len(transport)
	if originalSize == 0 || originalSize > MaxProtectedTransferSize {
		return "", nil, ErrTooLarge
	}
	transport = strings.TrimSpace(transport)
	if strings.HasPrefix(transport, magic+"1.") {
		if originalSize > MaxBundleSize {
			return "", nil, ErrTooLarge
		}
		return transport, nil, nil
	}
	if !strings.HasPrefix(transport, magic+"2.") {
		return "", nil, ErrBadFormat
	}
	raw, err := base64.RawURLEncoding.DecodeString(transport[len(magic+"2."):])
	if err != nil || checkPublicJSON(raw) != nil {
		return "", nil, ErrBadFormat
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 3 {
		return "", nil, ErrBadFormat
	}
	for _, name := range []string{"schema", "protected", "pin_policy"} {
		value, exists := fields[name]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return "", nil, ErrBadFormat
		}
	}
	if value := bytes.TrimSpace(fields["pin_policy"]); len(value) == 0 || value[0] != '"' {
		return "", nil, ErrBadFormat // only the base64 wire string, not a numeric byte array
	}
	var wire pinTransfer
	if decodeStrict(raw, &wire) != nil || wire.Schema != 2 || len(wire.Protected) == 0 || len(wire.Protected) > MaxBundleSize || !strings.HasPrefix(wire.Protected, magic+"1.") || len(wire.PinPolicy) == 0 || len(wire.PinPolicy) > delivery.MaxPinPolicyBytes {
		return "", nil, ErrBadFormat
	}
	return wire.Protected, wire.PinPolicy, nil
}

// ValidateProtectedTransport checks only bounded public framing. It neither
// derives a key nor authenticates encrypted contents or the public pin policy.
func ValidateProtectedTransport(transport string) error {
	_, _, err := splitPinTransfer(transport)
	return err
}
