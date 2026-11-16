package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
)

var errPinWakeSource = errors.New("future pin activation source changed or is unavailable")

type pinWakeSource struct {
	identity   pinWakeIdentity
	view       *metadata.RuntimeProfile
	now        time.Time
	selectable int
}

type pinWakeIdentity struct {
	trustSHA, acceptedSHA string
	version               metadata.ProfileVersion
	envelopeSHA           string
	floor                 time.Time
	checkpointed          bool
}

type pinActivationWake struct {
	timer                  *time.Timer
	intent                 uint64
	trustSHA, acceptedSHA  string
	version                metadata.ProfileVersion
	envelopeSHA, signerSHA string
	policySHA              string
	activation             metadata.RuntimePinActivation
	delay                  time.Duration
	plannedAt              time.Time // monotonic scheduling only, not trust authority
}

// Caller holds operationMu, not App/runtime/Manager locks. Inspection is
// read-only, under the existing journal; it cannot enroll or repair trust.
func (a *App) inspectPinWakeSource() (*pinWakeSource, error) {
	clock := a.trustClock() // App ownership before journal ownership
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, err
	}
	store := &metadata.Store{Dir: dir}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	readIdentity := func(snapshot *metadata.ProfileSnapshot) (pinWakeIdentity, error) {
		policy, state := snapshot.TrustPolicy(), snapshot.AcceptedProfile()
		if err := metadata.CheckAppliedPinPolicy(policy.Delivery, state.PinPolicy); err != nil {
			return pinWakeIdentity{}, err
		}
		encodedPolicy, err := metadata.EncodeTrustPolicy(policy)
		if err != nil {
			return pinWakeIdentity{}, err
		}
		encodedState, err := json.Marshal(state)
		if err != nil {
			return pinWakeIdentity{}, err
		}
		return pinWakeIdentity{trustSHA: publicSHA(encodedPolicy), acceptedSHA: publicSHA(encodedState), version: state.ProfileVersion(), envelopeSHA: state.EnvelopeSHA256,
			floor: metadata.TrustClockFloor(policy, state), checkpointed: state.HasCheckpoint()}, nil
	}
	snapshot, err := store.InspectProfileSnapshot()
	if err != nil {
		return nil, err
	}
	identity, err := readIdentity(snapshot)
	if err != nil {
		return nil, err
	}
	now, err := clock.Now(identity.floor)
	if err != nil {
		return nil, err
	}
	view, err := snapshot.InspectRuntimeProfile(now, metadata.ProfileVersion{})
	if err != nil {
		return nil, err
	}
	if view == nil || !view.Checkpointed || !identity.checkpointed || view.Version != identity.version || view.EnvelopeSHA256 != identity.envelopeSHA {
		return nil, errPinWakeSource
	}
	channels, err := render.DescriptorsFromRuntimeProfile(view)
	if err != nil {
		return nil, err
	}
	current, err := store.InspectProfileSnapshot()
	if err != nil {
		return nil, err
	}
	if !snapshot.SameSource(current) {
		return nil, errPinWakeSource
	}
	return &pinWakeSource{identity: identity, view: view, now: now, selectable: len(render.Selectable(channels))}, nil
}

func (a *App) preparePinActivationWake() (*pinActivationWake, error) {
	source, err := a.inspectPinWakeSource()
	if err != nil {
		return nil, err
	}
	view := source.view
	if view.NextPinActivation == nil || !view.NextPinActivation.At.After(source.now) {
		return nil, nil
	}
	if source.selectable != 0 {
		return nil, nil // the ordinary recovery path owns currently eligible channels
	}
	return &pinActivationWake{trustSHA: source.identity.trustSHA, acceptedSHA: source.identity.acceptedSHA, version: view.Version,
		envelopeSHA: view.EnvelopeSHA256, signerSHA: view.SignerFingerprint, policySHA: view.PinPolicySHA256, activation: *view.NextPinActivation,
		delay: view.NextPinActivation.At.Sub(source.now), plannedAt: time.Now()}, nil
}

// Requires a.mu. The pointer itself is the one-use generation token, so a
// stopped callback cannot adopt a later schedule with the same due time.
func (a *App) schedulePinActivationWakeLocked(wake *pinActivationWake, intent uint64) {
	a.cancelPinActivationWakeLocked()
	wake.intent = intent
	a.pinWake = wake
	delay := wake.delay - time.Since(wake.plannedAt)
	if delay < 0 {
		delay = 0
	}
	wake.timer = time.AfterFunc(delay, func() { a.firePinActivationWake(wake) })
	a.state.Error += fmt.Sprintf(" Автопопытка с новым сертификатом: %s; до неё защита сохраняется.", wake.activation.At.UTC().Format(time.RFC3339))
}

func (a *App) cancelPinActivationWakeLocked() {
	if a.pinWake != nil && a.pinWake.timer != nil {
		a.pinWake.timer.Stop()
	}
	a.pinWake = nil
}

func (a *App) ownsPinActivationWakeLocked(wake *pinActivationWake) bool {
	return a.pinWake == wake && a.operationID == wake.intent && a.protectionDesired && !a.profileTransition && !a.proxyConflict && !a.installedImageChanged && !a.closing.Load() && a.restartCommit == nil && a.releaseCommit == nil && a.trustBlocked && a.state.State == "error" && a.state.BlockedReason == "policy_channels_unavailable"
}

func (a *App) firePinActivationWake(wake *pinActivationWake) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	owned := a.ownsPinActivationWakeLocked(wake)
	a.mu.Unlock()
	if !owned {
		return
	}
	source, err := a.inspectPinWakeSource()
	if err == nil && (source.identity.trustSHA != wake.trustSHA || source.identity.acceptedSHA != wake.acceptedSHA || source.view.Version != wake.version || source.view.EnvelopeSHA256 != wake.envelopeSHA || source.view.SignerFingerprint != wake.signerSHA || source.view.PinPolicySHA256 != wake.policySHA) {
		err = errPinWakeSource
	}
	if err == nil && !source.now.Before(wake.activation.At) {
		usable := false
		for _, channel := range source.view.Channels {
			pins, _ := channel.SignedLeafPins()
			if channel.ID == wake.activation.ChannelID && channel.Enabled && pins != nil && pins.Next != nil && pins.Next.SHA256 == wake.activation.CertificateSHA256 {
				_, pinErr := pins.Certificates(channel.Hostname, source.now)
				usable = pinErr == nil
			}
		}
		if !usable {
			err = errPinWakeSource
		}
	}
	a.mu.Lock()
	if !a.ownsPinActivationWakeLocked(wake) {
		a.mu.Unlock()
		return
	}
	if err != nil {
		a.cancelPinActivationWakeLocked()
		a.state.Error = "Автозапуск с новым сертификатом не подтверждён. Получите актуальный профиль или повторите подключение; защита сохраняется."
		if code := trustFailureCode(err); code != "" {
			a.state.BlockedReason, a.state.Error = code, trustFailureDetail(err)
		}
		a.pushStateLocked()
		a.mu.Unlock()
		return
	}
	if source.now.Before(wake.activation.At) {
		// No launch on an early callback. Re-arm only the same observed deadline;
		// the clock observer rejects discontinuity instead of extending trust.
		wake.timer = time.AfterFunc(wake.activation.At.Sub(source.now), func() { a.firePinActivationWake(wake) })
		a.mu.Unlock()
		return
	}
	a.cancelFailClosedRetryLocked()
	a.trustBlocked = false
	launch := a.beginRetryLaunchLocked()
	a.mu.Unlock()
	if launch == nil {
		return
	}
	// The ordinary generation-owned start rereads current trust, renders a new
	// worker and must pass protected probe. No old worker/session is resumed.
	launch()
}
