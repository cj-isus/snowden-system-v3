package metadata

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

// CheckDeliveryBinding prevents an enrolled installation from silently falling
// back to legacy delivery when either public state file is lost or replaced.
// It detects omissions, not a local attacker able to rewrite both files.
func CheckDeliveryBinding(policy *TrustPolicy, state *AcceptedProfileState) error {
	if policy == nil || state == nil {
		return ErrTrustRole
	}
	if policy.Delivery == nil {
		if state.DeliveryAnchorSHA256 != "" {
			return fmt.Errorf("%w: retained delivery authority missing", ErrTrustRole)
		}
		return nil
	}
	if !state.HasCheckpoint() || state.DeliveryAnchorSHA256 != policy.Delivery.AnchorSHA256 {
		return fmt.Errorf("%w: delivery authority/checkpoint binding missing or changed", ErrTrustRole)
	}
	return nil
}

// CheckDeliveryProof checks a private verifier result against the exact retained
// source. No caller-supplied state or boolean can stand in for a target proof.
func CheckDeliveryProof(policy *TrustPolicy, proof *delivery.Prepared, kind delivery.Kind, path string, data []byte, now time.Time) error {
	if policy == nil || policy.Delivery == nil || proof == nil {
		return errors.New("metadata: exact delivery proof required")
	}
	source, err := policy.Delivery.Digest()
	if err != nil {
		return err
	}
	if source != proof.SourceDigest() {
		// A caller may durably retain this exact verified prefix before later
		// application checks. No newer/different state may reuse its proof.
		next, nextErr := proof.NextState().Digest()
		if nextErr != nil || next != source {
			return errors.New("metadata: delivery proof source superseded")
		}
	}
	return proof.Check(kind, path, data, now)
}

func checkDeliveredEnvelope(policy *TrustPolicy, next *Envelope, proof *delivery.Prepared, path string, raw []byte, now time.Time) error {
	if err := CheckDeliveryProof(policy, proof, delivery.Profile, path, raw, now); err != nil {
		return err
	}
	decoded, err := DecodeStrict(raw)
	if err != nil || next == nil || decoded.Signature != next.Signature || !bytes.Equal(decoded.Canonical(), next.Canonical()) {
		return errors.New("metadata: delivered bytes differ from incoming profile")
	}
	return nil
}

func checkProfileTarget(state *AcceptedProfileState, next *Envelope, path string) error {
	if state == nil || next == nil || next.ProfileEpoch != state.ProfileEpoch {
		return ErrProfileEpoch
	}
	if state.ProfileEpoch != 0 && !strings.HasPrefix(path, state.EpochNamespace) {
		return errors.New("metadata: profile target outside retained epoch namespace")
	}
	return nil
}
