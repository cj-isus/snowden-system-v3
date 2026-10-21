package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

type importCommitFn func(*filetxn.Store, []filetxn.File) error

func importTargets(store *metadata.Store, vaultPath string) []filetxn.File {
	return []filetxn.File{{Name: "keys", Path: store.KeysPath()}, {Name: "envelope", Path: store.EnvelopePath()}, {Name: "version", Path: filepath.Join(store.Dir, "version.txt")}, {Name: "vault", Path: vaultPath}}
}

// Recovery precedes Seed/migration and any startup profile consumers. An
// incomplete/corrupt journal stays present and blocks metadata and vault reads.
func (a *App) recoverOnboardingImport() error {
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	if a.vault == nil {
		return errors.New("onboarding: vault unavailable")
	}
	return a.vault.RecoverImport(store.ImportJournal(), func(path string) error {
		return (&filetxn.Store{Dir: store.ImportJournal()}).Recover(importTargets(store, path))
	})
}

func readImportFloor(dir string) (uint64, error) {
	data, err := os.ReadFile(filepath.Join(dir, "version.txt"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, errors.New("onboarding: corrupt version floor")
	}
	return v, nil
}

// Both entrypoints share exactly the same validation path. Decryption is
// outside operationMu; prepare/commit is serialized with engine operations and
// App vault writers. Preview performs no recovery or disk writes.
func (a *App) importOnboarding(transport, passphrase string, commit bool) (*OnboardingPreview, error) {
	payload, err := onboarding.Open(transport, passphrase)
	if err != nil {
		return nil, err
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.vault == nil {
		return nil, errors.New("onboarding: vault unavailable")
	}
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	var vaultPath string
	if err := a.vault.RecoverImport(store.ImportJournal(), func(path string) error { vaultPath = path; return filetxn.Check(store.ImportJournal()) }); err != nil {
		return nil, err
	}
	current, err := readImportFloor(dir)
	if err != nil {
		return nil, err
	}
	merged, _, err := store.LoadKeys()
	if err != nil {
		return nil, err
	}
	if merged == nil {
		merged = map[string]ed25519.PublicKey{}
	}
	seen := map[string]bool{}
	for _, k := range payload.TrustedKeys {
		raw, err := hex.DecodeString(k.Public)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, errors.New("onboarding: invalid public key")
		}
		id := metadata.KeyIDFor(ed25519.PublicKey(raw))
		if k.KeyID != id || seen[id] {
			return nil, errors.New("onboarding: duplicate or mismatched key ID")
		}
		seen[id] = true
		if old, ok := merged[id]; ok && !bytes.Equal(old, raw) {
			return nil, errors.New("onboarding: conflicting trusted key")
		}
		merged[id] = ed25519.PublicKey(raw)
	}
	env, err := metadata.DecodeStrict(payload.Envelope)
	if err != nil {
		return nil, err
	}
	if err := env.Verify(merged); err != nil {
		return nil, err
	}
	if env.MetadataVersion == 0 {
		return nil, errors.New("onboarding: metadata version must be positive")
	}
	if err := env.Validate(time.Now(), current); err != nil {
		return nil, err
	}
	channels, err := render.DescriptorsFromEnvelope(env)
	if err != nil {
		return nil, err
	}
	preview := &OnboardingPreview{CreatedAt: payload.CreatedAt.Format(time.RFC3339), DeviceName: payload.DeviceName, Channels: len(channels), Secrets: len(payload.Secrets), SplitDirect: len(env.SplitDirect), BundleVer: env.MetadataVersion, CurrentVer: current, Keys: []OnboardingKeyPreview{}}
	for _, k := range payload.TrustedKeys {
		preview.Keys = append(preview.Keys, OnboardingKeyPreview{KeyID: k.KeyID, Fingerprint: onboarding.KeyFingerprint(k.Public), Comment: k.Comment})
	}
	slots := make([]secretvault.Slot, len(payload.Secrets))
	for i, slot := range payload.Secrets {
		slots[i] = secretvault.Slot{ID: slot.ID, Kind: slot.Kind, Value: slot.Value}
	}
	err = a.vault.WithPreparedImport(slots, func(prepared *secretvault.PreparedImport) error {
		if err := render.ValidateCredentials(channels, prepared); err != nil {
			return err
		}
		if !commit {
			return nil
		}
		keys, err := metadata.EncodeKeys(merged)
		if err != nil {
			return err
		}
		vault, err := prepared.Bytes()
		if err != nil {
			return err
		}
		files := importTargets(store, vaultPath)
		files[0].Data = keys
		files[1].Data = bytes.Clone(payload.Envelope)
		files[2].Data = []byte(strconv.FormatUint(env.MetadataVersion, 10))
		files[3].Data = vault
		tx := &filetxn.Store{Dir: store.ImportJournal()}
		if a.onboardingCommit != nil {
			return a.onboardingCommit(tx, files)
		}
		return tx.Commit(files)
	})
	if err != nil {
		return nil, fmt.Errorf("onboarding: import rejected: %w", err)
	}
	if commit {
		preview.CurrentVer = env.MetadataVersion
		a.appendLogLocked("info", fmt.Sprintf("onboarding: импорт применён (каналов: %d, секретов: %d, версия: %d)", len(channels), len(slots), env.MetadataVersion))
	}
	return preview, nil
}

func (a *App) PreviewOnboardingString(transport, passphrase string) (*OnboardingPreview, error) {
	return a.importOnboarding(transport, passphrase, false)
}
func (a *App) ApplyOnboardingString(transport, passphrase string, confirm bool) (*OnboardingPreview, error) {
	return a.importOnboarding(transport, passphrase, confirm)
}
