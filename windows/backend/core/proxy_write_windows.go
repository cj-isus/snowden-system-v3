//go:build windows

package core

import "golang.org/x/sys/windows/registry"

type proxyWriteOutcome uint8

const (
	proxyWritePending proxyWriteOutcome = iota
	proxyWriteConfirmed
	proxyWriteUnchanged // error + exact observed before tuple, not proof against external ABA
	proxyWriteUnknown
)

// Process-local only, private and never logged/serialized: tuples may contain
// PAC credentials. Raw slices come from the immutable bounded reader/value
// constructors. This retains adapter inputs/results, not hidden SDK/FFI buffers
// or a durable/authenticated crash-recovery receipt.
type proxyWriteAttempt struct {
	index                     int
	before, target, observed  proxyRegistryState
	callStarted, callReturned bool
	readStarted, readReturned bool
	writeErr, readErr         error
	outcome                   proxyWriteOutcome
}

func (attempt *proxyWriteAttempt) call(key registry.Key) (result error) {
	if attempt == nil || attempt.callStarted || attempt.index < 0 || attempt.index >= len(proxyValueNames) {
		return ErrProxyStateUncertain
	}
	attempt.callStarted = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			attempt.outcome = proxyWriteUnknown
			attempt.writeErr, result = ErrProxyStateUncertain, ErrProxyStateUncertain
		}
		attempt.callReturned = returned
	}()
	attempt.writeErr = writeProxyRegistryValue(key, attempt.index, attempt.target[attempt.index])
	returned = true
	return attempt.writeErr
}

// Both existing value locks held. Unknown phases forbid new mutations/notify
// and even alias reads/Close during Restore; they cannot be replaced by a
// later identical tuple or by an ordinary error from another callback.
func (pm *ProxyManager) phaseErrorLocked() error {
	if pm.write != nil && (pm.write.outcome == proxyWritePending || pm.write.outcome == proxyWriteUnknown ||
		!pm.write.callReturned || !pm.write.readReturned) {
		return ErrProxyStateUncertain
	}
	if pm.notification != nil && !pm.notification.returned {
		return ErrProxyStateUncertain
	}
	return nil
}
