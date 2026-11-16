package metadata

import (
	"errors"
	"fmt"
)

// CheckLegacyExport requires a freshly authenticated accepted value. A legacy
// transfer cannot carry private cumulative history, so its signed envelope must
// independently retain every channel/certificate prohibition. It grants no
// enrollment authority and never changes the local checkpoint.
func (s *AcceptedProfileState) CheckLegacyExport(envelope *Envelope) error {
	if s == nil || envelope == nil || s.PinPolicy != nil || s.DeliveryAnchorSHA256 != "" || s.ProfileEpoch != 0 {
		return fmt.Errorf("%w: legacy transfer cannot carry enrolled trust or global pin policy", ErrTrustRole)
	}
	if err := s.validate(); err != nil {
		return errors.Join(ErrTrustRole, err)
	}
	digest, err := profileDigest(envelope)
	if err != nil || s.ProfileVersion() != envelope.ProfileVersion() || s.EnvelopeSHA256 != digest {
		return fmt.Errorf("%w: legacy transfer requires the authenticated installed profile", ErrTrustRole)
	}
	// Reuse the same first-acceptance algorithm as import. Do not approximate
	// which signed revocations survive that algorithm on a fresh destination.
	fresh, err := advanceProfile(&AcceptedProfileState{}, envelope)
	if err != nil {
		return err
	}
	channels, certificates := retainedChannelBans(fresh), retainedCertificateBans(fresh)
	for id := range retainedChannelBans(s) {
		if !channels[id] {
			return fmt.Errorf("%w: signed profile omits retained channel prohibitions; obtain a cumulative publisher profile", ErrTrustRole)
		}
	}
	for digest := range retainedCertificateBans(s) {
		if !certificates[digest] {
			return fmt.Errorf("%w: signed profile omits retained certificate prohibitions; obtain a cumulative publisher profile", ErrTrustRole)
		}
	}
	return nil
}
