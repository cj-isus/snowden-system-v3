package repository

import (
	"encoding/json"
	"errors"

	"github.com/snowden-system/windows/backend/delivery"
)

// The optional field cannot silently select a weaker interpretation. Absent
// means the existing full release; explicit null/empty/unknown is rejected.
func releasePurpose(fields map[string]json.RawMessage, ordinaryFields int) (delivery.Kind, error) {
	raw, exists := fields["purpose"]
	if !exists && len(fields) == ordinaryFields {
		return "", nil
	}
	var purpose delivery.Kind
	if !exists || len(fields) != ordinaryFields+1 || json.Unmarshal(raw, &purpose) != nil || purpose != delivery.PinPolicy {
		return "", errors.New("repository: unsupported/aliased publication purpose")
	}
	return purpose, nil
}
