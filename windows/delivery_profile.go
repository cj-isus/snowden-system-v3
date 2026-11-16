package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
)

// This read-only local snapshot is not installed engine state or path health.
// Legacy numeric fields remain for callers; nullable facts and State define
// what is known. Version/epoch identity must be consumed as the string tuple.
type DeliveryProfile struct {
	Present             bool   `json:"present"`
	Version             uint64 `json:"version"`
	ProfileEpoch        uint64 `json:"profileEpoch"`
	ProfileIdentity     string `json:"profileIdentity"`
	KeyID               string `json:"keyId"`
	ExpiresAt           string `json:"expiresAt"`
	TrustedKeys         int    `json:"trustedKeys"`
	ChannelsApplied     int    `json:"channelsApplied"`
	Error               string `json:"error"`
	FactsSchema         int    `json:"factsSchema"`
	State               string `json:"state"` // embedded/accepted/legacy_unanchored/restricted/policy_pending/unavailable
	CheckedAt           string `json:"checkedAt"`
	RetainedIdentity    string `json:"retainedIdentity"`
	ActiveProfileKeys   *int   `json:"activeProfileKeys"`
	ChannelsDeclared    *int   `json:"channelsDeclared"`
	ChannelsEligible    *int   `json:"channelsEligible"`
	ChannelsRestricted  *int   `json:"channelsRestricted"`
	CertificatesRevoked *int   `json:"certificatesRevoked"`
	ChannelsRetired     *int   `json:"channelsRetired"`
	PinPolicyState      string `json:"pinPolicyState"` // not_observed/applied/pending/unknown
	PinPolicyVersion    string `json:"pinPolicyVersion"`
	PinPolicySHA256     string `json:"pinPolicySHA256"`
	StorageFactsSchema  int    `json:"storageFactsSchema"`
	StorageState        string `json:"storageState"` // not_observed/cleanup_pending/recovery_required; process-only
	StorageWarning      string `json:"storageWarning"`
}

func unavailableDeliveryProfile(reason string) DeliveryProfile {
	return DeliveryProfile{FactsSchema: 1, State: "unavailable", PinPolicyState: "unknown", Error: reason}
}

func (a *App) GetDeliveryProfile() (DeliveryProfile, error) {
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return unavailableDeliveryProfile("Хранилище метаданных недоступно: " + err.Error()), nil
	}
	clock := a.trustClock() // acquire App ownership before the journal lock
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	return inspectDeliveryProfile(store, clock), nil
}

func inspectDeliveryProfile(store *metadata.Store, clock *metadata.ObservedClock) (out DeliveryProfile) {
	out = unavailableDeliveryProfile("")
	defer func() {
		state, warning := profileStorageFacts(store.ImportJournal())
		if state == "recovery_required" {
			out = unavailableDeliveryProfile(warning) // never retain eligibility on a quarantined read
		}
		out.StorageFactsSchema, out.StorageState, out.StorageWarning = 1, state, warning
	}()
	snapshot, err := store.InspectProfileSnapshot()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	policy, state := snapshot.TrustPolicy(), snapshot.AcceptedProfile()
	profileKeys := policy.VerificationKeys(metadata.RoleProfile)
	keys := len(profileKeys)
	out.TrustedKeys, out.ActiveProfileKeys = keys, &keys
	certificates, retired := state.RevocationCounts()
	out.CertificatesRevoked, out.ChannelsRetired = &certificates, &retired
	if state.HasCheckpoint() {
		out.RetainedIdentity = state.ProfileVersion().String()
	}
	out.PinPolicyState = "not_observed"
	if state.PinPolicy != nil {
		out.PinPolicyVersion, out.PinPolicySHA256 = fmt.Sprint(state.PinPolicy.Version), state.PinPolicy.SHA256
		out.PinPolicyState = "applied"
	}
	if err = metadata.CheckAppliedPinPolicy(policy.Delivery, state.PinPolicy); err != nil {
		out.Error = err.Error()
		if errors.Is(err, metadata.ErrPinPolicyPending) {
			out.State, out.PinPolicyState = "policy_pending", "pending"
			out.Error = "Получен новый список отзывов. Примените его отдельно; прежний профиль не считается разрешённым для запуска."
		}
		return out
	}
	now, err := clock.Now(metadata.TrustClockFloor(policy, state))
	if err != nil {
		out.Error = "Локальное время требует проверки: " + err.Error()
		return out
	}
	out.CheckedAt = now.UTC().Format(time.RFC3339)
	view, err := snapshot.InspectRuntimeProfile(now, metadata.ProfileVersion{})
	if err != nil {
		out.Error = err.Error()
		return out
	}
	if view == nil {
		out.State = "embedded" // no external profile, not proof of a running embedded path
		return out
	}
	if view.SignerFingerprint != publicSHA(profileKeys[view.KeyID]) || view.PinPolicySHA256 != out.PinPolicySHA256 || view.Checkpointed != state.HasCheckpoint() || view.Checkpointed && (view.Version != state.ProfileVersion() || view.EnvelopeSHA256 != state.EnvelopeSHA256) {
		return unavailableDeliveryProfile("Источник профиля изменился. Получите сведения заново.")
	}
	channels, err := render.DescriptorsFromRuntimeProfile(view)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Present, out.Version, out.ProfileEpoch = true, view.Version.Counter, view.Version.Epoch
	out.ProfileIdentity, out.KeyID, out.ExpiresAt = view.Version.String(), view.KeyID, view.ExpiresAt.UTC().Format(time.RFC3339)
	if !view.Checkpointed {
		out.State, out.Error = "legacy_unanchored", "Подпись проверена, но принятие содержимого профиля ещё не зафиксировано. Открытие настроек не принимает профиль."
		return out
	}
	declared, eligible, restricted := view.DeclaredChannels, len(render.Selectable(channels)), view.DeclaredChannels-len(channels)
	// Signed revocations remove channels before projection. Count those as
	// restrictions too, then add disabled projected channels without overlap.
	for _, restriction := range view.Restrictions {
		if restriction.Disabled {
			restricted++
		}
	}
	out.ChannelsApplied, out.ChannelsDeclared, out.ChannelsEligible, out.ChannelsRestricted = len(channels), &declared, &eligible, &restricted
	out.State = "accepted"
	if eligible == 0 {
		out.State, out.Error = "restricted", "В профиле сейчас нет каналов, разрешённых для выбора."
	}
	return out
}
