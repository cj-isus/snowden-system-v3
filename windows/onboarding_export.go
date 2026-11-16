package main

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

type DeliveredExportResult struct {
	DestinationMode            string `json:"destination_mode"`
	Transport                  string `json:"transport"` // encrypted only; never include plaintext slots
	ProfileIdentity            string `json:"profile_identity"`
	DestinationRootSHA256      string `json:"destination_root_sha256"`
	DestinationPinPolicySHA256 string `json:"destination_pin_policy_sha256"`
	TargetSHA256               string `json:"target_sha256"`
	Secrets                    int    `json:"secrets"`
}

// Only authenticated progress is written, and only on the explicitly approved
// export call. Preview remains read-only. No profile/epoch/history is changed;
// discovering an unapplied policy retires the session and refuses export.
func (a *App) retainEnrolledExportPrefix(source *enrolledExportSource, progress *delivery.Progress, requestScope *profileRequestScope) (updated *enrolledExportSource, committed bool, finalErr error) {
	if source == nil || progress == nil {
		return source, false, nil
	}
	if requestScope == nil || requestScope.app != a {
		return nil, false, context.Canceled
	}
	registration := requestScope.registration.Load()
	if registration == nil {
		return nil, false, context.Canceled
	}
	operationID := registration.intent // before waiting; never adopt a later Stop
	a.mu.Lock()
	vault, committer := a.vault, a.onboardingCommit
	a.mu.Unlock()
	if vault == nil {
		return nil, false, errors.New("onboarding: vault unavailable")
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	requestCurrent := requestScope.currentLocked(a)
	a.mu.Unlock()
	unlock := filetxn.Serialize(source.store.ImportJournal())
	locked := true
	releaseJournal := func() {
		if locked {
			unlock()
			locked = false
		}
	}
	defer releaseJournal()
	var transitionID uint64
	defer func() {
		if recovered := recover(); recovered != nil {
			releaseJournal()
			updated = nil
			finalErr = errors.New("onboarding: prefix retention interrupted")
		}
		if errors.Is(finalErr, errProfileStorageQuarantine) {
			releaseJournal()
			transitionID, finalErr = a.retireProfileStorageFailure(operationID, transitionID, finalErr)
		}
		if transitionID != 0 {
			releaseJournal()
			a.finishProfileTransition(transitionID, false, finalErr) // export cannot activate a new pin policy
		}
	}()
	if err := source.check(); err != nil {
		return nil, false, err
	}
	var vaultPath string
	var retirementErr error
	if !requestCurrent {
		retirementErr = context.Canceled
	}
	pending := metadata.CheckAppliedPinPolicy(progress.NextState(), source.state.PinPolicy)
	if pending != nil {
		releaseJournal()
		transitionID, retirementErr = a.retireProfileRequest(requestScope)
		unlock = filetxn.Serialize(source.store.ImportJournal())
		locked = true
	}
	if err := vault.RecoverImport(source.store.ImportJournal(), func(path string) error { vaultPath = path; return filetxn.Check(source.store.ImportJournal()) }); err != nil {
		return nil, false, errors.Join(retirementErr, err)
	}
	_, committed, err := retainDeliveryProgressWithOutcome(source.store, source.policy, source.state, progress, vault, vaultPath, committer)
	if err != nil {
		return nil, committed, errors.Join(retirementErr, err)
	}
	updated, err = readEnrolledExportSource(source.store)
	if pending != nil {
		return updated, committed, errors.Join(errors.New("onboarding: verified prefix retained; apply the pending pin policy before export"), pending, retirementErr, err)
	}
	return updated, committed, errors.Join(retirementErr, err)
}

// A recording resolver delegates role/ID/kind selection to the renderer and
// the vault. Only credentials actually requested by selectable channels leave
// the snapshot, in stable ID order; unrelated vault entries are not exported.
type exportCredentialSource struct {
	snapshot *secretvault.PreparedImport
	slots    map[string]secretvault.Slot
}

func (s *exportCredentialSource) get(id, kind string) (string, error) {
	slot, err := s.snapshot.ExportReference(id, kind)
	if err != nil {
		return "", err
	}
	if previous, exists := s.slots[slot.ID]; exists && previous != slot {
		return "", errors.New("onboarding: неоднозначный источник секрета")
	}
	s.slots[slot.ID] = slot
	return slot.Value, nil
}
func (s *exportCredentialSource) Get(ref string) (string, error) { return s.get("", ref) }
func (s *exportCredentialSource) GetByID(id, kind string) (string, error) {
	return s.get(id, kind)
}
func (*exportCredentialSource) PinCertPath(string) string { return "" }

func exportProfileCredentials(channels []render.ChannelDescriptor, snapshot *secretvault.PreparedImport) ([]onboarding.SecretSlot, error) {
	if snapshot == nil {
		return nil, errors.New("onboarding: отсутствует снимок хранилища")
	}
	credentials := &exportCredentialSource{snapshot: snapshot, slots: map[string]secretvault.Slot{}}
	defer clear(credentials.slots) // reference release, not Go string zeroization
	if err := render.ValidateCredentials(channels, credentials); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(credentials.slots))
	for id := range credentials.slots {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	slots := make([]onboarding.SecretSlot, 0, len(ids))
	for _, id := range ids {
		slot := credentials.slots[id]
		slots = append(slots, onboarding.SecretSlot{ID: slot.ID, Kind: slot.Kind, Value: slot.Value})
	}
	return slots, nil
}

func (a *App) captureExportVault(source *enrolledExportSource, requestScope *profileRequestScope) (*secretvault.Manager, *secretvault.PreparedImport, error) {
	a.mu.Lock()
	vault := a.vault
	a.mu.Unlock()
	if vault == nil {
		return nil, nil, errors.New("onboarding: vault unavailable")
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	current := a.vault == vault && requestScope.currentLocked(a)
	a.mu.Unlock()
	if !current {
		return nil, nil, context.Canceled
	}
	unlock := filetxn.Serialize(source.store.ImportJournal())
	defer unlock()
	if err := source.check(); err != nil {
		return nil, nil, err
	}
	if err := vault.RecoverImport(source.store.ImportJournal(), func(string) error { return filetxn.Check(source.store.ImportJournal()) }); err != nil {
		return nil, nil, err
	}
	snapshot, err := vault.PrepareImportSnapshot(nil) // encrypted owned snapshot only
	return vault, snapshot, err
}

func (a *App) ExportPublicProfileSource(token, passphrase string) (result *DeliveredExportResult, finalErr error) {
	if len(passphrase) < 8 || len(passphrase) > onboarding.MaxPassphraseSize || a.closing.Load() {
		return nil, errors.New("onboarding: требуется парольная фраза допустимой длины")
	}
	pending, flight, err := a.beginPublicSourceApply(token, "export")
	if err != nil {
		return nil, err
	}
	finalErr = a.runPublicSourceFlight(flight, func() error {
		var inner error
		result, inner = a.exportPublicProfileSource(flight, pending, passphrase)
		return inner
	})
	if finalErr != nil {
		return nil, finalErr
	}
	return result, nil
}

func (a *App) exportPublicProfileSource(flight *publicSourceFlight, pending *pendingPublicProfile, passphrase string) (result *DeliveredExportResult, finalErr error) {
	ctx, requestScope, err := a.captureProfileRequest(flight.ctx)
	if err != nil {
		return nil, err
	}
	clock := a.trustClock()
	now, err := clock.Now(time.Time{})
	if err != nil || !now.Before(pending.expires) {
		return nil, errors.New("onboarding: подтверждение переноса истекло или часы требуют проверки")
	}
	payload, err := onboarding.DecodePublicProfile([]byte(pending.request.JSON))
	if err != nil {
		return nil, err
	}
	prepared, source, progress, verifyErr := a.prepareEnrolledExport(ctx, payload, pending.preview, pending.exportCheckpoint)
	prefixRetained := false
	defer func() {
		if finalErr != nil && prefixRetained {
			finalErr = errors.Join(errors.New("onboarding: подтверждённое состояние доставки сохранено; перенос не создан"), finalErr)
		}
	}()
	if progress != nil {
		updated, committed, retentionErr := a.retainEnrolledExportPrefix(source, progress, requestScope)
		prefixRetained = committed
		if updated != nil {
			source = updated
		}
		err = retentionErr
		if err != nil {
			return nil, errors.Join(verifyErr, err)
		}
	}
	if verifyErr != nil {
		return nil, verifyErr // retained prefix is not acceptance/export of the artifact
	}
	if prepared == nil || source == nil || prepared.portableRoot != pending.exportRoot {
		return nil, errors.New("onboarding: корневой документ переноса изменился после превью")
	}
	prepared.source = source // after own authenticated prefix transaction
	now, err = clock.Now(deliveryClockFloor(source.policy, source.state))
	if err != nil {
		return nil, err
	}
	if err = checkImportApprovalDeadline(now, pending.expires); err != nil {
		return nil, err
	}
	if err = prepared.checkFresh(now); err != nil {
		return nil, err
	}
	vault, snapshot, err := a.captureExportVault(source, requestScope)
	if err != nil {
		return nil, err
	}
	raw, err := prepared.payload.ExactProfileBytes()
	if err != nil {
		return nil, err
	}
	env, err := metadata.DecodeStrict(raw)
	if err != nil {
		return nil, err
	}
	channels, err := render.DescriptorsFromEnvelopeAt(env, now)
	if err != nil {
		return nil, err
	}
	if !requestScope.current() {
		return nil, context.Canceled
	}
	outgoing := *prepared.payload
	outgoing.Secrets, err = exportProfileCredentials(channels, snapshot)
	if err != nil {
		return nil, err
	}
	defer clear(outgoing.Secrets)
	// All App/journal/vault locks are released during DPAPI reads and scrypt.
	transport, err := onboarding.Seal(&outgoing, passphrase)
	if err != nil {
		return nil, err
	}
	if !requestScope.current() {
		return nil, context.Canceled
	}
	err = func() error {
		a.operationMu.Lock()
		defer a.operationMu.Unlock()
		a.mu.Lock()
		sameVault, requestCurrent := a.vault == vault, requestScope.currentLocked(a)
		a.mu.Unlock()
		if !requestCurrent {
			return context.Canceled
		}
		if !sameVault {
			return errors.New("onboarding: хранилище изменилось во время экспорта")
		}
		unlock := filetxn.Serialize(source.store.ImportJournal())
		defer unlock()
		return vault.WithPreparedSnapshot(snapshot, func() error {
			if !requestScope.current() {
				return errors.New("onboarding: приложение завершает работу")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := source.checkInScope(); err != nil {
				return err
			}
			freshNow, err := clock.Now(deliveryClockFloor(source.policy, source.state))
			if err != nil {
				return err
			}
			if err = checkImportApprovalDeadline(freshNow, pending.expires); err != nil {
				return err
			}
			if err := prepared.checkFresh(freshNow); err != nil {
				return err
			}
			if !requestScope.current() {
				return context.Canceled
			}
			return nil
		})
	}()
	if err != nil {
		return nil, err
	}
	destinationMode := "enroll"
	if env.ProfileEpoch != 0 {
		destinationMode = "epoch_enroll"
	}
	return &DeliveredExportResult{Transport: transport, ProfileIdentity: env.ProfileVersion().String(), DestinationRootSHA256: prepared.portableRoot, DestinationPinPolicySHA256: prepared.preview.PinPolicySHA256, TargetSHA256: prepared.preview.TargetSHA256, Secrets: len(outgoing.Secrets), DestinationMode: destinationMode}, nil
}
