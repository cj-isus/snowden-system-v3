package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/render"
)

var errTrustCheckpoint = errors.New("cannot retain local trust clock checkpoint")

func (a *App) trustClock() *metadata.ObservedClock {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.observedClock == nil {
		a.observedClock = &metadata.ObservedClock{}
	}
	return a.observedClock
}

func (a *App) sessionLifetime(cc *render.ClientConfig) (time.Duration, error) {
	if cc == nil {
		return 0, errors.New("missing session validity")
	}
	now, err := a.trustClock().Now(cc.Validity.ClockFloor)
	if err != nil {
		return 0, err
	}
	return cc.Validity.Remaining(now)
}

// Candidate-only update, before engine mutation and outside App/runtime/epoch/
// Manager locks. Descriptor polling never records a local wall observation.
func (a *App) retainSessionClock(cc *render.ClientConfig) error {
	now, err := a.trustClock().Now(cc.Validity.ClockFloor)
	if err != nil {
		return err
	}
	if _, err := cc.Validity.Remaining(now); err != nil {
		return err
	}
	if cc.Profile.Source == "verified_metadata_signature" {
		dir, err := metadata.DefaultStoreDir()
		if err != nil {
			return fmt.Errorf("%w: store location unavailable", errTrustCheckpoint)
		}
		if err := (&metadata.Store{Dir: dir}).RecordRuntimeClockObservation(metadata.ProfileVersion{Epoch: cc.Profile.ProfileEpoch, Counter: cc.Profile.Version}, cc.Profile.EnvelopeSHA256, cc.Profile.PinPolicySHA256, now); err != nil {
			return fmt.Errorf("%w: %w", errTrustCheckpoint, err)
		}
	}
	cc.Validity.ClockFloor, cc.Profile.ObservedAt = now, now
	return nil
}

// The existing App watchdog supplies the low-frequency durable checkpoint.
// The independent Manager observer still retires expiry if disk I/O stalls.
// Applied cc is immutable: persist from a copy, never mutate live snapshots.
func (a *App) checkpointSessionClock(cc *render.ClientConfig) bool {
	intent, desired := a.probeIntent()
	if !desired || !a.appLifetimeCurrent() {
		return false
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if !a.operationCurrent(intent) || !a.sessionCurrent(cc) {
		return false
	}
	runtime_.mu.Lock()
	manager := runtime_.manager
	running := runtime_.cc == cc && manager != nil && manager.State() == core.StateRunning
	var report *core.ProbeReport
	if running {
		owner, generation := manager.Owner()
		running = owner == cc.OwnerID && generation == cc.OwnerGeneration
		report = manager.LastProbeReport() // identity only, never reused path/health evidence
	}
	runtime_.mu.Unlock()
	if !running || report == nil || report.SessionID != cc.SessionID || report.IntentGeneration != intent {
		return false
	}
	if err := a.retainSessionClock(cc.Clone()); err != nil {
		a.failTrustCoreIntentLocked(intent, err)
		return false
	}
	return a.operationCurrent(intent) && a.sessionCurrent(cc)
}

func trustFailureCode(err error) string {
	// Global trust faults outrank a candidate's certificate fault in joined
	// rollback errors; neither error order nor a partial recovery may hide them.
	switch {
	case errors.Is(err, metadata.ErrClockUncertain), errors.Is(err, &core.TrustFaultError{Reason: "clock_uncertain"}):
		return "clock_uncertain"
	case errors.Is(err, metadata.ErrProfileExpired), errors.Is(err, &core.TrustFaultError{Reason: "profile_expired"}):
		return "profile_expired"
	case errors.Is(err, metadata.ErrTrustNotYetValid), errors.Is(err, &core.TrustFaultError{Reason: "trust_not_yet_valid"}):
		return "trust_not_yet_valid"
	case errors.Is(err, metadata.ErrPinPolicyPending):
		return "pin_policy_pending"
	case errors.Is(err, metadata.ErrPinPolicyRestricted):
		return "policy_channels_unavailable"
	case errors.Is(err, errTrustCheckpoint):
		return "trust_checkpoint_unavailable"
	case errors.Is(err, metadata.ErrTrustRole):
		return "trust_authority_unavailable"
	case errors.Is(err, metadata.ErrIdentityExpired), errors.Is(err, metadata.ErrNoActivePin), errors.Is(err, &core.TrustFaultError{Reason: "certificate_expired"}):
		return "certificate_expired"
	case errors.Is(err, &core.TrustFaultError{Reason: "trust_unavailable"}):
		return "trust_unavailable"
	}
	return ""
}

func trustFailureDetail(err error) string {
	switch trustFailureCode(err) {
	case "clock_uncertain":
		return "Непрерывность локальных часов потеряна. Проверьте дату и время; сохранённые версии и историю доверия сбрасывать нельзя."
	case "profile_expired":
		return "Срок подписанного профиля истёк. Требуется новый действующий подписанный профиль."
	case "certificate_expired":
		return "Срок разрешённого сертификата истёк. Нужна новая сессия с действующим пином; прежняя сессия не продлевается сроком Next."
	case "trust_not_yet_valid":
		return "Профиль ещё не действует по локальным часам. Проверьте время и срок профиля."
	case "trust_checkpoint_unavailable":
		return "Не удалось сохранить состояние доверия. Проверьте доступность хранилища перед новым подключением."
	case "pin_policy_pending":
		return "Получен новый доверенный список отзывов, но его применение не завершено. Требуется завершить обновление политики; прежняя сессия не возобновляется."
	case "policy_channels_unavailable":
		return "В подписанном профиле сейчас нет разрешённого доступного канала. Нужен действующий безопасный канал или обновлённый профиль."
	case "trust_authority_unavailable":
		return "Профиль или полномочия его ключа не подтверждены. Требуется исправление доверенного профиля или явное офлайн-восстановление; сеть эту ошибку не исправит."
	}
	return "Срок доверия работающей сессии не подтверждён."
}

func trustFailureReport(err error) *core.ProbeReport {
	return &core.ProbeReport{TrustUnavailable: true, TrustReason: trustFailureCode(err), Results: []core.ProbeResult{{Name: "срок доверия", Status: "unavailable", Detail: trustFailureDetail(err)}}}
}

// The shared running owner also observes its actual proxy. Only a genuine trust
// fault may set TrustUnavailable; local registry failure is not certificate loss.
func sessionLifetimeFailureReport(cause error) *core.ProbeReport {
	if !errors.Is(cause, core.ErrProxyObservationUnavailable) && !errors.Is(cause, core.ErrProxyOwnershipConflict) {
		return trustFailureReport(cause)
	}
	report := &core.ProbeReport{}
	if trustFailureCode(cause) != "" {
		report = trustFailureReport(cause)
	}
	report.Results = append(report.Results, core.ProbeResult{Name: "системный прокси", Status: "unavailable", Detail: lifecycleFailureText("проверка", cause, false)})
	return report
}

// Caller holds operationMu. No channel-health/quarantine observation is written.
func (a *App) failTrustCoreLocked(cause error) {
	a.mu.Lock()
	intent := a.operationID
	a.mu.Unlock()
	a.failTrustCoreIntentLocked(intent, cause)
}

func (a *App) failTrustCoreIntentLocked(intent uint64, cause error) {
	proxyConflict := errors.Is(cause, core.ErrProxyOwnershipConflict)
	proxyFailure := proxyConflict || errors.Is(cause, core.ErrProxyObservationUnavailable)
	failureReport := sessionLifetimeFailureReport(cause)
	proxyMessage := lifecycleFailureText("остановка", core.ErrProxyOwnershipConflict, false)
	a.mu.Lock()
	if a.operationID != intent || !a.protectionDesired || a.profileTransition || a.closing.Load() {
		a.mu.Unlock()
		return
	}
	if proxyConflict {
		a.proxyConflict = true
	}
	a.cancelFailClosedRetryLocked()
	a.mu.Unlock()
	cleanupErr := a.stopCoreGenerationLocked(stopFailure, intent)
	var pinWake *pinActivationWake
	if cleanupErr == nil && !channelRetryBlocked(cause) && trustFailureCode(cause) == "policy_channels_unavailable" {
		// No App lock during public metadata inspection. A clock/pending/source
		// failure only suppresses the hint; it cannot authorize another retry.
		pinWake, _ = a.preparePinActivationWake()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.operationID != intent || !a.protectionDesired || a.profileTransition || a.closing.Load() {
		return
	}
	a.state.State = "error"
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.state.Error = trustFailureDetail(cause)
	a.state.BlockedReason = trustFailureCode(cause)
	if proxyFailure {
		a.state.Error = lifecycleFailureText("проверка", cause, false)
		a.state.BlockedReason = "protection_retained" // local observation unavailable, not failed cleanup or expired trust
	}
	if a.state.BlockedReason == "" {
		a.state.BlockedReason = "trust_unavailable"
	}
	if cleanupErr != nil || localRetirementFailure(cause) {
		if cleanupErr != nil {
			a.state.Error += "; остановка не подтверждена, результат очистки требует проверки"
		} else {
			a.state.Error += "; возврат ресурсов исходной операции не подтверждён"
		}
		a.state.BlockedReason = "cleanup_unconfirmed"
		a.state.Probe = toView(failureReport)
		a.trustBlocked = true
		a.sm.markBlocked(time.Now())
		a.pushStateLocked()
		return // even certificate_expired cannot retry over uncertain cleanup
	}
	if a.proxyConflict {
		a.state.Error = proxyMessage
		a.state.BlockedReason = "proxy_conflict"
		a.trustBlocked = true
		a.state.Probe = toView(failureReport)
		a.sm.markBlocked(time.Now())
		a.pushStateLocked()
		return
	}
	a.state.Probe = toView(failureReport)
	// Profile/clock/storage faults require correction, not network churn.
	a.trustBlocked = proxyFailure || trustFailureCode(cause) != "certificate_expired"
	if pinWake != nil {
		a.schedulePinActivationWakeLocked(pinWake, intent)
	} else if !a.trustBlocked {
		a.scheduleFailClosedRetryLocked(time.Now())
		a.sm.markRecovering(time.Now())
	}
	a.pushStateLocked()
}
