package metadata

import (
	"errors"
	"slices"
	"time"
)

// CheckPinPolicyContinuity checks retained values only, never their authority.
// A new value still needs a fresh VerifiedPinPolicy from the same TUF lineage.
// Expiry intentionally cannot erase previously accepted cumulative bans.
func CheckPinPolicyContinuity(previous, next *RetainedPinPolicy) error {
	if next == nil {
		if previous != nil {
			return errors.New("metadata: retained pin-policy removal denied")
		}
		return nil
	}
	if err := next.validate(); err != nil {
		return err
	}
	if previous == nil {
		return nil
	}
	if err := previous.validate(); err != nil {
		return err
	}
	if next.Version < previous.Version || next.IssuedAt.Before(previous.IssuedAt) || !pinPolicySuperset(next.RevokedCertificates, previous.RevokedCertificates) || !pinPolicySuperset(next.RetiredChannels, previous.RetiredChannels) {
		return errors.New("metadata: cumulative pin-policy rollback/removal denied")
	}
	if next.Version == previous.Version && !samePinPolicy(next, previous) {
		return errors.New("metadata: conflicting pin-policy content under one version")
	}
	return nil
}

// Retain prepares owned cumulative public policy for a publisher/client. It
// neither authenticates previous local state nor persists/activates anything.
func (v *VerifiedPinPolicy) Retain(policy *TrustPolicy, previous *RetainedPinPolicy, now time.Time) (*RetainedPinPolicy, error) {
	if err := v.Check(policy, now); err != nil {
		return nil, err
	}
	return v.retain(previous)
}

func (v *VerifiedPinPolicy) retain(previous *RetainedPinPolicy) (*RetainedPinPolicy, error) {
	next := v.retained()
	if err := CheckPinPolicyContinuity(previous, next); err != nil {
		return nil, err
	}
	return next, nil
}

// CheckEnvelope applies public global bans only. Signature, profile validity,
// local device history, credentials and lifecycle remain the caller's gates.
func (p *RetainedPinPolicy) CheckEnvelope(envelope *Envelope) error {
	if p == nil {
		return nil
	}
	if err := p.validate(); err != nil {
		return err
	}
	if envelope == nil {
		return errors.New("metadata: missing profile for global pin-policy checks")
	}
	for _, channel := range envelope.Channels {
		revokedID := slices.Contains(envelope.Revocations, channel.ID)
		if _, retired := slices.BinarySearch(p.RetiredChannels, channel.ID); retired && !revokedID {
			return errors.New("metadata: globally retired channel cannot be reauthorized")
		}
		if revokedID {
			continue
		}
		if channel.Enabled && channel.Protocol == "hysteria2" && channel.HY2Pins == nil {
			return errors.New("metadata: global pin policy requires signed HY2 leaf pins for enabled channels")
		}
		leaf, _ := channel.SignedLeafPins()
		if leaf == nil {
			continue
		}
		pins := []CertificatePin{leaf.Current}
		if leaf.Next != nil {
			pins = append(pins, *leaf.Next)
		}
		for _, pin := range pins {
			if _, revoked := slices.BinarySearch(p.RevokedCertificates, pin.SHA256); revoked && !slices.Contains(leaf.Revoked, pin.SHA256) {
				return errors.New("metadata: globally revoked certificate cannot be reauthorized")
			}
		}
	}
	return nil
}
