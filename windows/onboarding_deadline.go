package main

import (
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

var errImportPreviewExpired = errors.New("onboarding: срок действия профиля или подтверждения истёк; получите новое превью")

func earliestImportDeadline(current, candidate time.Time) time.Time {
	if candidate.IsZero() || !current.IsZero() && current.Before(candidate) {
		return current
	}
	return candidate.UTC()
}

// Zero means this direct API call has no backend-owned preview nonce. It
// still repeats every signed validity check. A source token's deadline is
// private App state and can never be supplied or extended by public JSON.
func checkImportApprovalDeadline(now, approvedUntil time.Time) error {
	if !approvedUntil.IsZero() && !now.Before(approvedUntil) {
		return errImportPreviewExpired
	}
	return nil
}

func profileImportDeadline(delivered *bundleDelivery, policy *metadata.TrustPolicy, env *metadata.Envelope, raw, recovery []byte, now time.Time) (time.Time, error) {
	if env == nil || env.ExpiresAt.IsZero() || !now.Before(env.ExpiresAt) {
		return time.Time{}, errImportPreviewExpired
	}
	deadline, err := delivered.validUntil(policy, raw, now)
	if err != nil {
		return time.Time{}, err
	}
	deadline = earliestImportDeadline(env.ExpiresAt, deadline)
	if len(recovery) != 0 {
		permit, err := metadata.DecodeRecoveryPermit(recovery)
		if err != nil {
			return time.Time{}, err
		}
		if err = permit.Validate(now); err != nil {
			return time.Time{}, err
		}
		deadline = earliestImportDeadline(deadline, permit.ExpiresAt)
	}
	return deadline.UTC(), nil
}
