package delivery

import (
	"encoding/hex"

	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
)

// PinPolicyTargetSHA256 observes the protected retained SDK cache. It is not
// fresh artifact proof and must never authorize accepting caller-supplied bytes.
// Empty means no retained target observation, including an initial empty cache.
func (s *State) PinPolicyTargetSHA256() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	raw := s.Cache["targets.json"]
	if len(raw) == 0 {
		return "", nil
	}
	top, err := tuf.Targets().FromBytes(raw)
	if err != nil {
		return "", ErrTrust
	}
	if err = targetsPolicy(tuf.TARGETS, top); err != nil {
		return "", err
	}
	info, declared := top.Signed.Targets[PinPolicyPath]
	if !declared {
		return "", nil
	}
	if info == nil || info.Length <= 0 || info.Length > MaxPinPolicyBytes || len(info.Hashes["sha256"]) != 32 {
		return "", ErrTrust
	}
	return hex.EncodeToString(info.Hashes["sha256"]), nil
}
