package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

type legacyProfileExport struct {
	envelope *metadata.Envelope
	channels []render.ChannelDescriptor
	trusted  []onboarding.TrustedKey
}

// Caller owns the native scope used to capture this public source.
// PrepareAcceptance is read-only; accepting here must never save a legacy anchor.
func prepareLegacyExportInScope(source *profileExportSource, now time.Time) (*legacyProfileExport, error) {
	if source == nil || source.policy == nil || source.state == nil || !source.policyPresent {
		return nil, errors.New("onboarding: доверенных ключей нет — переносить нечего")
	}
	if source.policy.Delivery != nil {
		return nil, errors.New("onboarding: для TUF-enrolled профиля нужен свежий пакет publisher; legacy export потерял бы delivery proof")
	}
	envelope, err := metadata.DecodeStrict(source.raw)
	if err != nil {
		return nil, err
	}
	keys := source.policy.VerificationKeys(metadata.RoleProfile)
	accepted, err := source.store.PrepareAcceptance(envelope, keys, now)
	if err != nil {
		return nil, err
	}
	if err := accepted.CheckLegacyExport(envelope); err != nil {
		return nil, err
	}
	channels, err := render.DescriptorsFromEnvelopeAt(envelope, now)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	trusted := make([]onboarding.TrustedKey, 0, len(ids))
	for _, id := range ids {
		trusted = append(trusted, onboarding.TrustedKey{KeyID: id, Public: hex.EncodeToString(keys[id])})
	}
	return &legacyProfileExport{envelope: envelope, channels: channels, trusted: trusted}, nil
}

func (a *App) exportLegacyProfile(ctx context.Context, passphrase string) (string, error) {
	_, requestScope, err := a.captureProfileRequest(ctx)
	if err != nil {
		return "", err
	}
	clock := a.trustClock() // App ownership precedes journal/vault ownership
	a.mu.Lock()
	vault := a.vault
	a.mu.Unlock()
	if vault == nil {
		return "", errors.New("onboarding: vault unavailable")
	}
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return "", err
	}
	store := &metadata.Store{Dir: dir}
	var source *profileExportSource
	var snapshot *secretvault.PreparedImport
	var prepared *legacyProfileExport
	var now time.Time
	err = func() error {
		a.operationMu.Lock()
		defer a.operationMu.Unlock()
		a.mu.Lock()
		current := a.vault == vault && requestScope.currentLocked(a)
		a.mu.Unlock()
		if !current {
			return context.Canceled
		}
		unlock := filetxn.Serialize(store.ImportJournal())
		defer unlock()
		if !requestScope.current() {
			return context.Canceled
		}
		var err error
		source, err = captureProfileExportSource(store, func(captured *profileExportSource) error {
			now, err = clock.Now(deliveryClockFloor(captured.policy, captured.state))
			if err != nil {
				return err
			}
			prepared, err = prepareLegacyExportInScope(captured, now)
			return err
		})
		if err != nil {
			return err
		}
		snapshot, err = vault.PrepareImportSnapshot(nil) // encrypted owned data only
		return err
	}()
	if err != nil {
		return "", err
	}
	if !requestScope.current() {
		return "", context.Canceled
	}
	// Decryption and KDF take no App/operation/journal/vault/native scope locks.
	slots, err := exportProfileCredentials(prepared.channels, snapshot)
	if err != nil {
		return "", err
	}
	defer clear(slots)
	payload := &onboarding.BundlePayload{Schema: 1, CreatedAt: now, TrustedKeys: prepared.trusted,
		Envelope: json.RawMessage(source.raw), Secrets: slots}
	transport, err := onboarding.Seal(payload, passphrase)
	if err != nil {
		return "", err
	}
	if !requestScope.current() {
		return "", context.Canceled
	}
	err = func() error {
		a.operationMu.Lock()
		defer a.operationMu.Unlock()
		// Check App identity before entering journal/vault; no App locks inside.
		a.mu.Lock()
		sameVault, current := a.vault == vault, requestScope.currentLocked(a)
		a.mu.Unlock()
		if !current {
			return context.Canceled
		}
		if !sameVault {
			return errors.New("onboarding: хранилище изменилось во время экспорта")
		}
		unlock := filetxn.Serialize(store.ImportJournal())
		defer unlock()
		return vault.WithPreparedSnapshot(snapshot, func() error {
			if !requestScope.current() {
				return context.Canceled
			}
			if err := source.checkInScope(); err != nil {
				return err
			}
			freshNow, err := clock.Now(deliveryClockFloor(source.policy, source.state))
			if err != nil {
				return err
			}
			// Exact public/vault source equality preserves the earlier role,
			// signature and portability checks; resample time after final I/O.
			if err := prepared.envelope.ValidateVersion(freshNow, source.state.ProfileVersion()); err != nil {
				return err
			}
			if !requestScope.current() {
				return context.Canceled
			}
			return nil
		})
	}()
	if err != nil {
		return "", err // never release ciphertext from stale/uncertain source
	}
	return transport, nil
}
