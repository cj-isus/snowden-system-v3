package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

// Publisher-only operation. The application independently checks that this
// signing key was pre-enrolled for recovery and that all source/target bindings
// still match. A newly signed permit alone cannot bootstrap that authority.
func runRecoverySign(args []string, now time.Time) error {
	if len(args) != 3 {
		return errors.New("usage: metadatatool recovery-sign <permit.json> <@offline-recovery.key> <new-output.json>")
	}
	data, err := readBindingFile(args[0], metadata.MaxRecoveryPermitSize)
	if err != nil {
		return err
	}
	permit, err := metadata.DecodeRecoveryPermit(data)
	if err != nil {
		return err
	}
	if err := permit.Validate(now); err != nil {
		return err
	}
	if permit.Signature != "" {
		return errors.New("unsigned recovery plan required; preserve prior signed output")
	}
	if len(args[1]) == 0 || args[1][0] != '@' {
		return errors.New("private key requires an offline @file")
	}
	keyHex, err := readBindingFile(args[1][1:], 4096)
	if err != nil {
		return err
	}
	defer clear(keyHex)
	trimmed := bytes.TrimSpace(keyHex)
	if len(trimmed) != ed25519.PrivateKeySize*2 {
		return errors.New("invalid offline recovery private key")
	}
	key := make([]byte, ed25519.PrivateKeySize)
	defer clear(key)
	if _, err := hex.Decode(key, trimmed); err != nil {
		return errors.New("invalid offline recovery private key")
	}
	if permit.KeyID != "" && permit.KeyID != metadata.KeyIDFor(ed25519.PrivateKey(key).Public().(ed25519.PublicKey)) {
		return errors.New("recovery plan signing key does not match the offline key")
	}
	if err := permit.Sign(ed25519.PrivateKey(key)); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(permit, "", "  ")
	if err != nil {
		return err
	}
	return publishBindingFile(args[2], encoded)
}
