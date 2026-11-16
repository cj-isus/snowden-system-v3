package update

import (
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/netprobe"
)

const ApplicationFile = childbootstrap.ApplicationFile
const RecoveryApplicationFile = "snowden-recovery.exe"  // byte-identical signed NEW application, cold repair only
const RecoveryHelperCopyFile = "native-helper.recovery" // non-dispatched exact signed helper bytes, retained with control
const PairTarget = "windows/amd64"
const PairActivationProtocol = 5 // fixed authenticated new-code recovery entrypoint plus current-code GUI acceptance

var ErrPairActivationProtocol = errors.New("update: unsupported pair activation protocol")

type Payload struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type NativeHelper struct {
	Payload
	Protocol int `json:"protocol"`
}

func canonicalPayloadSHA(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == value
}

func (m *Manifest) validatePair() error {
	if m.Schema == Schema {
		if m.Target != "" || m.NativeHelper != nil || m.ActivationProtocol != 0 {
			return errors.New("update: schema1 cannot carry pair fields")
		}
		return nil
	}
	if m.ActivationProtocol != PairActivationProtocol {
		return ErrPairActivationProtocol
	}
	h := m.NativeHelper
	if m.Target != PairTarget || m.File != ApplicationFile || h == nil || h.File != netprobe.NativeHelperName || h.Protocol != netprobe.NativeProtocol ||
		h.Size <= 0 || h.Size > netprobe.MaxNativeImageBytes || m.Size > MaxPayloadSize-h.Size || !canonicalPayloadSHA(m.SHA256) || !canonicalPayloadSHA(h.SHA256) || m.SHA256 == h.SHA256 {
		return errors.New("update: invalid fixed Windows application/helper pair")
	}
	return nil
}

// DescribePayload streams a bounded regular file. It produces public artifact
// facts, not code authority, PE compatibility or a build-pin attestation.
func DescribePayload(path string, limit int64) (Payload, error) {
	if limit <= 0 || limit > MaxPayloadSize {
		return Payload{}, errors.New("update: invalid artifact limit")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return Payload{}, errors.New("update: bounded regular artifact required")
	}
	sum, err := payloadDigest(path, info.Size())
	if err != nil {
		return Payload{}, err
	}
	return Payload{File: filepath.Base(path), SHA256: sum, Size: info.Size()}, nil
}

func (p Payload) Verify(path string) error {
	return (&Manifest{Size: p.Size, SHA256: p.SHA256}).VerifyPayload(path)
}

// The destination is caller-owned staging, not an installed executable. An
// error may leave partial output; publish it only after this method succeeds.
func (p Payload) CopyVerified(path string, dst io.Writer) error {
	sum, err := payloadDigestTo(path, p.Size, dst)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, p.SHA256) {
		return errors.New("update: copied payload sha256 mismatch")
	}
	return nil
}

// VerifyPayloadSet authenticates both files for schema2. It does not apply,
// recover or claim support for installing this pair on the current machine.
func (m *Manifest) VerifyPayloadSet(dir string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := m.VerifyPayload(filepath.Join(dir, m.File)); err != nil {
		return err
	}
	if m.NativeHelper != nil {
		return m.NativeHelper.Payload.Verify(filepath.Join(dir, m.NativeHelper.File))
	}
	return nil
}
