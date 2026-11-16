package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

type EpochEnrollmentRequest struct {
	Next               *Envelope
	SourcePolicy       *TrustPolicy
	Enrollment         *delivery.State
	ApprovedRootSHA256 string
	ProfileProof       *delivery.Prepared
	TargetPath         string
	ProfileBytes       []byte
	GrantProof         *delivery.Prepared
	GrantBytes         []byte
	Now                time.Time
}

func (s *AcceptedProfileState) emptyEpochSource() bool {
	return s != nil && s.Schema == 0 && s.ProfileVersion() == (ProfileVersion{}) &&
		s.EnvelopeSHA256 == "" && len(s.Pins) == 0 && len(s.Retired) == 0 &&
		s.TimeFloor.IsZero() && s.IssuedAtFloor.IsZero() && s.LastRecoverySHA256 == "" &&
		s.DeliveryAnchorSHA256 == "" && s.EpochNamespace == "" && s.LastEpochGrantSHA256 == "" &&
		len(s.EpochHistory) == 0 && s.EpochOrigin == nil && s.PinPolicy == nil
}

// CheckFreshEpochSource is called under the existing import journal ownership,
// again after lifecycle retirement. Numeric0, corrupt/symlink files or an old
// root are not a clean installation. This is not cross-process Windows CAS.
func (s *Store) CheckFreshEpochSource(policy *TrustPolicy) error {
	if policy == nil || policy.validate() != nil || policy.Delivery != nil {
		return fmt.Errorf("%w: fresh enrollment requires an unenrolled source", ErrProfileEpoch)
	}
	for _, path := range []string{filepath.Join(s.Dir, "version.txt"), s.EnvelopePath()} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: initial epoch enrollment requires absent version/envelope files", ErrProfileEpoch)
		}
	}
	current, _, err := s.LoadTrustPolicy()
	if err != nil {
		return err
	}
	source, err := EncodeTrustPolicy(policy)
	if err != nil {
		return err
	}
	actual, err := EncodeTrustPolicy(current)
	if err != nil || current.Schema != policy.Schema || !bytes.Equal(source, actual) {
		return fmt.Errorf("%w: clean source authority changed", ErrProfileEpoch)
	}
	return nil
}

// PrepareEpochEnrollment records a first approved origin, not a sequence of
// transitions the device never performed. App supplies a separately confirmed
// root fingerprint and target; both private SDK proofs use that exact baseline.
func (s *Store) PrepareEpochEnrollment(request EpochEnrollmentRequest) (*TrustPolicy, *AcceptedProfileState, error) {
	if err := s.CheckFreshEpochSource(request.SourcePolicy); err != nil {
		return nil, nil, err
	}
	baseline := request.Enrollment
	if baseline == nil || baseline.Validate() != nil || len(baseline.Cache) != 0 || baseline.AnchorSHA256 != request.ApprovedRootSHA256 || request.ApprovedRootSHA256 == "" {
		return nil, nil, fmt.Errorf("%w: approved initial root baseline required", ErrProfileEpoch)
	}
	rootDigest := sha256.Sum256(baseline.Root)
	if hex.EncodeToString(rootDigest[:]) != request.ApprovedRootSHA256 {
		return nil, nil, fmt.Errorf("%w: initial root bytes differ from approval", ErrProfileEpoch)
	}
	if err := CheckClockFloor(request.Now, baseline.ClockFloor); err != nil {
		return nil, nil, err
	}
	now := laterTime(request.Now, baseline.ClockFloor)
	checking := request.SourcePolicy.Scoped()
	checking.Delivery = baseline.Clone()
	verified, err := verifyEpochTarget(checking, request.Next, request.ProfileProof, request.TargetPath, request.ProfileBytes, request.GrantProof, request.GrantBytes, now)
	if err != nil {
		return nil, nil, err
	}
	updated := request.SourcePolicy.Scoped()
	active := updated.VerificationKeys(RoleProfile)
	for i := range updated.Keys {
		if updated.Keys[i].Role == RoleProfile && updated.Keys[i].KeyID != verified.keyID {
			updated.Keys[i].Retired = true // keep seed public history
		}
	}
	if !bytes.Equal(active[verified.keyID], verified.public) {
		updated, err = updated.WithKey(ed25519.PublicKey(verified.public), RoleProfile)
		if err != nil {
			return nil, nil, err // no foreign/retired role promotion or revival
		}
	}
	sourceDigest, err := request.SourcePolicy.Digest()
	if err != nil {
		return nil, nil, err
	}
	origin := &EpochOrigin{Schema: 1, Epoch: request.Next.ProfileEpoch, Counter: request.Next.MetadataVersion,
		EnvelopeSHA256: verified.contentSHA256, GrantSHA256: verified.grantSHA256,
		AnchorSHA256: baseline.AnchorSHA256, SourceTrustSHA256: sourceDigest}
	accepted, err := advanceProfileValue(&AcceptedProfileState{}, request.Next, nil, origin)
	if err != nil {
		return nil, nil, err
	}
	updated.Delivery = request.ProfileProof.NextState()
	if err := CheckDeliveryBinding(updated, accepted); err != nil {
		return nil, nil, err
	}
	return updated, accepted, nil
}
