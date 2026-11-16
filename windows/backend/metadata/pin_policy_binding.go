package metadata

import (
	"fmt"

	"github.com/snowden-system/windows/backend/delivery"
)

var ErrPinPolicyPending = fmt.Errorf("%w: authenticated pin policy has not been applied", ErrTrustRole)
var ErrPinPolicyRestricted = fmt.Errorf("%w: no permitted runtime channel", ErrTrustRole)

// CheckAppliedPinPolicy is a runtime/prefix restriction, not an artifact
// acceptance API. A verified new top-target hash is a durable obligation even
// when downloading/parsing/committing its body later fails. Old applied bans
// remain retained; only a fresh private proof can authorize the next snapshot.
func CheckAppliedPinPolicy(state *delivery.State, applied *RetainedPinPolicy) error {
	if state == nil {
		if applied != nil {
			return ErrPinPolicyPending
		}
		return nil
	}
	expected, err := state.PinPolicyTargetSHA256()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTrustRole, err)
	}
	if expected == "" && applied == nil {
		return nil // no global policy observation; no claim of unseen history
	}
	if applied == nil || applied.validate() != nil || expected != applied.SHA256 {
		return ErrPinPolicyPending
	}
	return nil // expiry does not remove already accepted bans
}
