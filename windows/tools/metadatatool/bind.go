package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

func runBind(args []string, now time.Time) error {
	if len(args) != 4 {
		return errors.New("usage: metadatatool bind <source.json> <bindings.json> <@private.key> <new-output.json>")
	}
	raw, err := readBindingFile(args[0], metadata.MaxBindingMigrationSize)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	source, err := metadata.DecodeStrict(raw)
	if err != nil {
		return err
	}
	raw, err = readBindingFile(args[1], metadata.MaxBindingMigrationSize)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	plan, err := metadata.DecodeBindingMigration(raw)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(args[2], "@") || len(args[2]) == 1 {
		return errors.New("private key must be @file")
	}
	keyHex, err := readBindingFile(args[2][1:], 1024)
	if err != nil {
		return fmt.Errorf("private key file: %w", err)
	}
	defer clear(keyHex)
	key, err := hex.DecodeString(strings.TrimSpace(string(keyHex)))
	defer clear(key)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return errors.New("private key: invalid hex or size")
	}
	// Ed25519 private files encode seed + public key. Reject an inconsistent
	// trailing public half rather than emitting a signature nobody can verify.
	canonicalKey := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	defer clear(canonicalKey)
	if !ed25519.PrivateKey(key).Equal(canonicalKey) {
		return errors.New("private key: inconsistent public half")
	}
	pub := canonicalKey.Public().(ed25519.PublicKey)
	keys := map[string]ed25519.PublicKey{metadata.KeyIDFor(pub): pub}
	out, err := metadata.MigrateLegacyBindings(source, plan, keys, now)
	if err != nil {
		return err
	}
	if err := out.Sign(canonicalKey); err != nil {
		return err
	}
	if err := out.Verify(keys); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if len(encoded) > metadata.MaxBindingMigrationSize {
		return errors.New("migration: output too large")
	}
	return publishBindingFile(args[3], encoded)
}

func readBindingFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, errors.New("regular file required")
	}
	if stat.Size() > limit {
		return nil, errors.New("file too large")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file too large")
	}
	return data, nil
}

// Publish a fully written file only if destination is absent. A hard link is
// the commit point; it cannot replace any existing input/output path. Unsupported
// filesystems return an error. Parent-directory crash durability is a separate gate.
func publishBindingFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".binding-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	if err := f.Sync(); err != nil {
		return err
	}
	err = f.Close()
	closed = true
	if err != nil {
		return err
	}
	return os.Link(temp, path)
}
