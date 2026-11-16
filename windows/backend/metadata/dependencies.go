package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DeliveryDependencies contains public pseudonyms for profile distribution.
// These are signed owner assertions, not measurements or account credentials.
// An alternative publisher label is not proof of an available recovery path.
type DeliveryDependencies struct {
	Publisher         string `json:"publisher,omitempty"`
	Account           string `json:"account,omitempty"`
	RecoveryPublisher string `json:"recovery_publisher,omitempty"`
}

func ValidDependencyLabel(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 256 {
		return false
	}
	for _, c := range []byte(value) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == ':' || c == '-' || c == '_') {
			return false
		}
	}
	return true // lexical bound only; public pseudonyms must not contain credentials
}

func (d DeliveryDependencies) Validate() error {
	for name, value := range map[string]string{"publisher": d.Publisher, "account": d.Account, "recovery_publisher": d.RecoveryPublisher} {
		if !ValidDependencyLabel(value) {
			return fmt.Errorf("metadata: dependencies.%s must be a bounded public identifier", name)
		}
	}
	if !ValidDependencyAccount(d.Account) {
		return errors.New("metadata: dependencies.account needs a public provider:pseudonym identity")
	}
	return nil
}

func ValidDependencyAccount(value string) bool {
	if value == "" || strings.EqualFold(value, "unknown") {
		return true
	}
	provider, alias, found := strings.Cut(value, ":")
	return found && provider != "" && alias != "" && ValidDependencyLabel(value)
}

func (d *DeliveryDependencies) UnmarshalJSON(data []byte) error {
	if len(data) > 4096 {
		return errors.New("metadata: dependency assertions too large")
	}
	object, err := decodePinObject(data, "publisher", "account", "recovery_publisher")
	if err != nil {
		return fmt.Errorf("metadata: delivery dependencies: %w", err)
	}
	var out DeliveryDependencies
	for name, field := range map[string]*string{"publisher": &out.Publisher, "account": &out.Account, "recovery_publisher": &out.RecoveryPublisher} {
		if raw, ok := object[name]; ok {
			if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, field) != nil {
				return errors.New("metadata: dependency identifiers must be strings, not null")
			}
		}
	}
	if err := out.Validate(); err != nil {
		return err
	}
	*d = out
	return nil
}
