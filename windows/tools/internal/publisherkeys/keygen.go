// Package publisherkeys shares offline role-aware key provisioning between
// publisher CLIs. It is never called by the GUI or by incoming profile data.
package publisherkeys

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
)

func Generate(policyPath, privatePath string, role metadata.KeyRole) (string, error) {
	policyPath, err := filepath.Abs(policyPath)
	if err != nil {
		return "", err
	}
	privatePath, err = filepath.Abs(privatePath)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(policyPath, privatePath) {
		return "", errors.New("public policy and private key require distinct paths")
	}
	journal := filepath.Join(filepath.Dir(policyPath), "import.pending")
	if err := filetxn.Check(journal); err != nil {
		return "", err
	}
	policy, present, err := metadata.ReadTrustPolicyFile(policyPath)
	if err != nil {
		return "", err
	}
	before, err := policy.Digest()
	if err != nil {
		return "", err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	defer clear(priv)
	updated, err := policy.WithKey(pub, role)
	if err != nil {
		return "", err
	}
	data, err := metadata.EncodeTrustPolicy(updated)
	if err != nil {
		return "", err
	}
	// Never overwrite an existing private key, including an accidental alias
	// of the public policy. Do not publish authority before private durability.
	f, err := os.OpenFile(privatePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	privateBytes := make([]byte, hex.EncodedLen(len(priv)))
	hex.Encode(privateBytes, priv)
	n, writeErr := f.Write(privateBytes)
	clear(privateBytes)
	if writeErr == nil && n != ed25519.PrivateKeySize*2 {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = f.Sync()
	}
	writeErr = errors.Join(writeErr, f.Close())
	if writeErr != nil {
		return "", fmt.Errorf("private key file was created but not fully persisted: %w", writeErr)
	}
	// Detect observed changes, without claiming cross-process atomic CAS or
	// Windows directory/ACL durability. Provision while the application is off.
	current, stillPresent, err := metadata.ReadTrustPolicyFile(policyPath)
	if err != nil {
		return "", fmt.Errorf("private key retained; policy read failed: %w", err)
	}
	now, err := current.Digest()
	if err != nil || present != stillPresent || before != now {
		return "", errors.New("private key retained; trust policy changed, refusing overwrite")
	}
	if err := filetxn.Check(journal); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Dir(policyPath), ".trust-*")
	if err != nil {
		return "", err
	}
	name := temp.Name()
	defer os.Remove(name)
	n, writeErr = temp.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = temp.Sync()
	}
	writeErr = errors.Join(writeErr, temp.Close())
	if writeErr != nil {
		return "", fmt.Errorf("private key retained; public policy stage failed: %w", writeErr)
	}
	if err := os.Rename(name, policyPath); err != nil {
		return "", fmt.Errorf("private key retained; public policy publication failed: %w", err)
	}
	return metadata.KeyIDFor(pub), nil
}
