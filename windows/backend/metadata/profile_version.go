package metadata

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const EpochEnvelopeSchema = 2
const ProfileEpochProtocol = 2
const MaxRetainedProfileEpochs = 64 // per-installation transition/history budget, not a global epoch ceiling

var ErrProfileEpoch = fmt.Errorf("%w: profile epoch transition requires offline authority", ErrTrustRole)

// ProfileVersion is unrelated to the current network epoch. Scalar legacy
// APIs reject nonzero profile epochs instead of comparing only the counter.
type ProfileVersion struct {
	Epoch   uint64 `json:"profile_epoch,string"`
	Counter uint64 `json:"counter,string"`
}

func (v ProfileVersion) String() string { return fmt.Sprintf("%d:%d", v.Epoch, v.Counter) }

func (v ProfileVersion) Compare(other ProfileVersion) int {
	if v.Epoch < other.Epoch || v.Epoch == other.Epoch && v.Counter < other.Counter {
		return -1
	}
	if v == other {
		return 0
	}
	return 1
}

func (e *Envelope) ProfileVersion() ProfileVersion {
	if e == nil {
		return ProfileVersion{}
	}
	return ProfileVersion{Epoch: e.ProfileEpoch, Counter: e.MetadataVersion}
}

func (s *AcceptedProfileState) ProfileVersion() ProfileVersion {
	if s == nil {
		return ProfileVersion{}
	}
	return ProfileVersion{Epoch: s.ProfileEpoch, Counter: s.Version}
}

func ProfileEpochNamespace(epoch uint64) string {
	// Pinned go-tuf2.4.2 matches delegation patterns per path component;
	// profiles/* does not match nested folders. Keep a delimiter-bound flat
	// prefix so epoch1 cannot also match epoch10 and no policy widening is needed.
	return "profiles/epoch-" + strconv.FormatUint(epoch, 10) + "-"
}

// ValidateCandidate validates an issuer's own tuple, not a device floor or an
// authorization to change epochs. Application acceptance requires both gates.
func (e *Envelope) ValidateCandidate(now time.Time) error {
	if e == nil || e.MetadataVersion == 0 {
		return errors.New("metadata: positive candidate profile counter required")
	}
	return e.ValidateVersion(now, e.ProfileVersion())
}

func (e *Envelope) validateVersionShape(minimum ProfileVersion) error {
	if e == nil || e.SchemaVersion != SchemaVersion && e.SchemaVersion != EpochEnvelopeSchema {
		return fmt.Errorf("%w: unsupported profile schema/epoch", ErrProfileEpoch)
	}
	if e.SchemaVersion == SchemaVersion && e.ProfileEpoch != 0 || e.SchemaVersion == EpochEnvelopeSchema && e.ProfileEpoch == 0 {
		return fmt.Errorf("%w: profile schema/epoch mismatch", ErrProfileEpoch)
	}
	if e.ProfileEpoch != minimum.Epoch {
		return ErrProfileEpoch
	}
	if e.ProfileVersion().Compare(minimum) < 0 {
		return fmt.Errorf("metadata: profile tuple rollback (%d:%d below %d:%d)", e.ProfileEpoch, e.MetadataVersion, minimum.Epoch, minimum.Counter)
	}
	return nil
}
