package metadata

import "errors"

// ValidateSecretIDs is shared by signed and embedded descriptors. Partial
// bindings must not silently fall back to credentials selected by kind.
func ValidateSecretIDs(refs []string, ids map[string]string) error {
	if len(ids) == 0 {
		return nil
	}
	if len(ids) != len(refs) {
		return errors.New("secret_ids must bind every credential ref exactly once")
	}
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref] {
			return errors.New("secret_ids: duplicate credential ref")
		}
		seen[ref] = true
		id, ok := ids[ref]
		if !ok || !validSecretID(id) {
			return errors.New("secret_ids: missing or invalid SecretID")
		}
	}
	return nil
}

func validSecretID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
