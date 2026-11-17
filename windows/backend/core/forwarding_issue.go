package core

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// One original ordinary issuance, not a forwarding grant. Only its exact
// admission slot can publish it; shared negative clearing permanently retires
// the attempt. The synchronous caller retains it through entropy/unwind.
type forwardingIssue struct {
	epoch                             uint64
	subject, selector                 string
	originalSubject, originalSelector string
	originalReceipt                   *workerNativeTUNReceipt
	until                             time.Time
}

// Called only after the actual worker probe passed, with its outer Engine.mu
// still held. The admission mutex is not held across OS entropy. Public reports
// cannot create this receipt; final consumption remains single-use and private.
func (admission *forwardingAdmission) issue(subject, selector string, expectedEpoch uint64) (uint64, string, error) {
	if admission == nil {
		return 0, "", ErrForwardingNotAdmitted
	}
	var original *forwardingIssue
	epoch, err := func() (uint64, error) {
		admission.mu.Lock()
		defer admission.mu.Unlock()
		admission.clearPendingLocked()
		if admission.closed || admission.tunScope != nil || admission.epoch == 0 || admission.epoch != expectedEpoch || !workerSubject(subject) || !validWorkerTag(selector) {
			return 0, ErrForwardingNotAdmitted
		}
		if admission.open.Load() {
			if admission.subject != subject || admission.selector != selector {
				return 0, ErrForwardingNotAdmitted
			}
			return admission.epoch, nil // existing admission; no new entropy/token/deadline
		}
		original = &forwardingIssue{epoch: admission.epoch, subject: subject, selector: selector,
			originalSubject: admission.subject, originalSelector: admission.selector, originalReceipt: admission.nativeReceipt,
			until: time.Now().Add(workerControlTimeout)}
		admission.pendingIssue = original // original owner BEFORE OS entropy
		return admission.epoch, nil
	}()
	if err != nil || original == nil {
		return epoch, "", err
	}
	defer func() {
		admission.mu.Lock()
		defer admission.mu.Unlock()
		if admission.pendingIssue == original {
			admission.pendingIssue = nil // never clear a newer attempt or published token
		}
	}()
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return 0, "", err
	}
	token := hex.EncodeToString(nonce[:])
	admission.mu.Lock()
	defer admission.mu.Unlock()
	if admission.pendingIssue != original || admission.closed || admission.open.Load() || admission.tunScope != nil || admission.epoch != original.epoch ||
		admission.subject != original.originalSubject || admission.selector != original.originalSelector || admission.nativeReceipt != original.originalReceipt ||
		admission.pending != "" || !admission.pendingUntil.IsZero() || admission.nativePending != nil || admission.nativeConsuming != nil || !time.Now().Before(original.until) {
		return 0, "", ErrForwardingNotAdmitted
	}
	admission.pendingIssue = nil
	admission.subject, admission.selector = original.subject, original.selector
	admission.pending, admission.pendingUntil = token, original.until // waiting never renews expiry
	return original.epoch, token, nil
}
