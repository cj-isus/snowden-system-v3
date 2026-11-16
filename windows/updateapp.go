package main

// updateapp.go — применение обновлений приложения (V2-050/F15).
//
// Сценарий: владелец (или скрипт сборки) кладёт рядом с установленным exe
// update.json + payload (или указывает каталог с ними). Приложение:
//
//	1) читает манифест, проверяет подпись только явно назначенным update-ключом
//	   из scoped trusted_keys.json (profile import не разрешает замену EXE);
//	2) проверяет SHA-256 и размер payload;
//	3) требует версию строго больше текущей И выше этажа принятых версий
//	   (антидаунгрейд переживает переустановку старой версии поверх);
//	4) атомарная подмена: payload → temp в каталоге exe → rename поверх
//	   текущего exe (Windows: цель не должна быть запущена → swap выполняется
//	   специальным путём: текущий exe переименовывается в .old, новый
//	   переименовывается на его место, старт нового процесса, выход старого;
//	   .old удаляется новым процессом при старте);
//	5) новый процесс при старте поднимает этаж версий (update.RaiseFloor)
//	   и подчищает .old.
//
// Delivery/signature/hash/version gates precede rename. This is not a complete
// Windows installer transaction: restart/crash/power-loss acceptance stays open.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/update"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// runningVersion — версия текущего бинарника. Задаётся ldflags при сборке;
// для dev-сборок — честная метка "dev" (обновления на dev не применяются:
// нет ни осмысленного сравнения, ни подписанной истории).
var runningVersion = "dev"

// UpdatePreview — что покажет UI до применения.
type UpdatePreview struct {
	Schema              int    `json:"schema"`
	ApprovalToken       string `json:"approval_token,omitempty"`
	ApprovalIntent      string `json:"approval_intent,omitempty"`
	ApprovalExpiresAt   string `json:"approval_expires_at,omitempty"`
	ManifestSHA256      string `json:"manifest_sha256,omitempty"`
	ReasonCode          string `json:"reason_code,omitempty"`
	CurrentVersion      string `json:"current_version"`
	Version             string `json:"version"`
	Notes               string `json:"notes,omitempty"`
	ReleasedAt          string `json:"released_at,omitempty"`
	Size                int64  `json:"size"`
	SHA256              string `json:"sha256"`
	KeyID               string `json:"key_id"`
	Floor               string `json:"floor"`
	OK                  bool   `json:"ok"`
	Reason              string `json:"reason,omitempty"`
	DeliveryVerified    bool   `json:"delivery_verified"`
	DeliveryRootVersion int64  `json:"delivery_root_version,omitempty"`
	DeliveryValidUntil  string `json:"delivery_valid_until,omitempty"`
}

// checkVersionPolicy — антидаунгрейд и строгое возрастание.
func checkVersionPolicy(m *update.Manifest) error {
	cur, err := update.ParseVersion(runningVersion)
	if err != nil {
		return fmt.Errorf("update: текущая версия %q не разбирается: %w", runningVersion, err)
	}
	nv, err := update.ParseVersion(m.Version)
	if err != nil {
		return err
	}
	if nv.Compare(cur) <= 0 {
		return fmt.Errorf("update: версия %s не выше текущей %s — обновление отвергнуто", m.Version, runningVersion)
	}
	floor, err := update.ReadFloor()
	if err != nil {
		return err
	}
	if nv.Compare(floor) <= 0 && floor != (update.Version{}) {
		return fmt.Errorf("update: версия %s не выше этажа принятых %s (антидаунгрейд) — отвергнуто", m.Version, floor.String())
	}
	return nil
}

// CheckUpdate — проверка каталога обновления (без применения). Честный
// превью-результат даже при отказе: OK=false + finite ReasonCode. Approval names
// exact checked bytes; it does not waive fresh installation/source/native gates.
func (a *App) CheckUpdate(updateDir string) (*UpdatePreview, error) {
	prev := &UpdatePreview{Schema: 1, CurrentVersion: runningVersion}
	deny := func(code string) (*UpdatePreview, error) {
		prev.OK, prev.ReasonCode = false, code
		return prev, nil // finite presentation reason; no raw paths or underlying errors
	}
	intent, session, err := a.beginUpdatePreview()
	if err != nil {
		return deny("source_changed")
	}
	defer a.finishUpdatePreview()
	if updateDir == "" || len(updateDir) > 32767 {
		return deny("invalid_directory")
	}
	updateDir, err = filepath.Abs(updateDir)
	if err != nil {
		return deny("invalid_directory")
	}
	source, err := a.contextForIntent(intent)
	if err != nil {
		return deny("source_changed")
	}
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	ctx, cancel, err := a.beginUpdateOperation(intent, false)
	if err != nil {
		return deny("source_changed")
	}
	defer a.endOperation(cancel)
	floor, err := update.ReadFloor()
	if err != nil {
		return deny("floor_unavailable")
	}
	prev.Floor = floor.String()
	verified, err := a.prepareVerifiedUpdateContext(ctx, updateDir, false)
	if err != nil {
		return deny("manifest_or_authority")
	}
	m := verified.manifest
	if verified.proof != nil {
		prev.DeliveryVerified = true
		prev.DeliveryRootVersion = verified.proof.RootVersion()
		prev.DeliveryValidUntil = verified.proof.ValidUntil().Format(time.RFC3339)
	}
	prev.Version = m.Version
	prev.Notes = m.Notes
	prev.ReleasedAt = m.ReleasedAt.Format(time.RFC3339)
	prev.Size = m.Size
	prev.SHA256 = m.SHA256
	prev.KeyID = m.KeyID
	if err := checkVersionPolicy(m); err != nil {
		return deny("version_policy")
	}
	if err := checkUpdateInstallSupport(m); err != nil {
		return deny("installer_unavailable")
	}

	if err := m.VerifyPayloadSet(updateDir); err != nil {
		return deny("payload_mismatch")
	}
	// Payload hashing may outlive the prepared authority. Reuse the same
	// source/role/floor/time guard as install before publishing an OK preview.
	if err := a.withVerifiedUpdateSourceLocked(verified, func() error { return nil }); err != nil {
		prev.DeliveryVerified = false
		prev.DeliveryRootVersion, prev.DeliveryValidUntil = 0, ""
		return deny("source_changed")
	}
	approval, err := a.approveUpdatePreview(updateDir, session, intent, source, verified)
	if err != nil || !lifecyclePublicationCurrent(ctx) {
		return deny("approval_unavailable")
	}
	prev.ApprovalToken, prev.ManifestSHA256 = approval.token, updateManifestIdentity(approval)
	prev.ApprovalIntent, prev.ApprovalExpiresAt = formatUpdateIntent(intent), approval.expires.UTC().Format(time.RFC3339)
	prev.OK = true
	a.appendLog("info", fmt.Sprintf("update: манифест %s проверен (назначенная authority, sha256, версия) — готов к применению", m.Version))
	return prev, nil
}

// PickUpdateDir — диалог выбора каталога с update.json (нативный Wails).
func (a *App) PickUpdateDir() (string, error) {
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Каталог с update.json и payload",
	})
	if err != nil {
		return "", err
	}
	return dir, nil // отмена — пустая строка, не ошибка
}

// ApplyUpdate preserves the pre-protocol legacy entrypoint. Once ordered UI
// admission is activated, positive calls require its one-use target approval.
// An ordinary return never proves replacement-process readiness or activation.
func (a *App) ApplyUpdate(updateDir string) (string, error) {
	a.mu.Lock()
	if a.uiLifecycle != nil {
		a.mu.Unlock()
		return "", errUILifecycleRequest // no positive legacy bypass of ordered target approval
	}
	intent, err := a.restartIntentLocked()
	a.mu.Unlock()
	if err != nil {
		return "", err
	}
	return a.applyUpdateRegistered(updateDir, intent, nil, nil)
}

func (a *App) applyUpdateRegistered(updateDir string, intent intentRegistration, approval *updatePreviewApproval, progress *updateInstallProgress) (message string, finalErr error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if !a.restartIntentCurrent(intent) {
		return "", context.Canceled
	}
	if approval != nil && !approval.current() {
		return "", errUpdateApproval
	}
	if updateDir == "" {
		return "", errors.New("update: укажите каталог с update.json и payload")
	}
	// Повторная полная проверка (TOCTOU-гейт): между Check и Apply файлы могли
	// измениться — доверять можно только свежей проверке.
	verified, err := func() (*verifiedUpdate, error) {
		a.operationMu.Lock()
		defer a.operationMu.Unlock()
		if !a.restartIntentCurrent(intent) {
			return nil, context.Canceled
		}
		ctx, cancel, err := a.beginUpdateOperation(intent, false)
		if err != nil {
			return nil, err
		}
		defer a.endOperation(cancel)
		verified, err := a.prepareVerifiedUpdateContextApproved(ctx, updateDir, true, approval)
		if err != nil {
			return nil, err
		}
		if err := checkVersionPolicy(verified.manifest); err != nil {
			return nil, err
		}
		if err := checkUpdateInstallSupport(verified.manifest); err != nil {
			return nil, err
		}
		if err := verified.manifest.VerifyPayloadSet(updateDir); err != nil {
			return nil, err
		}
		if !a.restartIntentCurrent(intent) {
			return nil, context.Canceled
		}
		return verified, nil
	}()
	if err != nil {
		return "", err
	}
	if approval != nil && !approval.current() {
		return "", errUpdateApproval
	}
	if progress != nil {
		progress.entered = true // before any install/staging owner; never a mutation/success receipt
	}
	m := verified.manifest
	if m.Schema == update.PairSchema {
		return a.applyVerifiedPair(updateDir, verified, intent)
	}
	payload := filepath.Join(updateDir, m.File)
	a.appendLog("info", fmt.Sprintf("update: подготовлено %s — staging и повторный authority gate", m.Version))

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("update: путь exe: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	root := filepath.Dir(exe)
	rootContext, err := a.contextForIntent(intent)
	if err != nil {
		return "", err
	}
	rootContext, cancelRoot := context.WithTimeout(rootContext, 30*time.Second)
	defer cancelRoot()
	// Legacy single-EXE updates must participate in the same native writer
	// scope as pairs. Historical clients that lack it remain a migration gate.
	if err := a.discardUnpublishedPair(); err != nil {
		return "", err
	}
	rootCleanupInHandoff := false
	defer func() {
		if !rootCleanupInHandoff {
			finalErr = errors.Join(finalErr, a.closePairRoot())
		}
	}()
	if !a.restartIntentCurrent(intent) {
		return "", context.Canceled
	}
	if err := a.acquirePairRoot(rootContext, root); err != nil {
		return "", err
	}

	// 1. Копируем payload в temp РЯДОМ с exe (rename работает только в рамках
	// одного тома).
	tmp, err := stageUpdatePayload(payload, filepath.Dir(exe), m.Size)
	if err != nil {
		return "", fmt.Errorf("update: staging: %w", err)
	}
	removeStaged := true
	defer func() {
		if !rootCleanupInHandoff && removeStaged {
			_ = os.Remove(tmp) // still under the original writer scope
		}
	}()
	if err := m.VerifyPayload(tmp); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update: staged payload: %w", err)
	}

	// 2. Останавливаем ядро (TUN/прокси/kill switch) ДО подмены — так новый
	// процесс стартует в чистом окружении, а системный прокси не останется
	// указывать на мёртвый порт.
	handoff, err := a.beginRestartHandoff(intent)
	if err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update: остановка ядра перед подменой: %w", err)
	}
	defer handoff.release()
	rootCleanupInHandoff = true
	defer func() {
		if removeStaged {
			_ = os.Remove(tmp) // original staged name, before closing its writer and unlocking lifecycle
		}
		finalErr = errors.Join(finalErr, a.closePairRoot())
	}()
	mutationAttempted := false
	defer func() {
		if mutationAttempted {
			handoff.requireFreshImage()
		}
	}()
	if err := handoff.commit(); err != nil {
		return "", err
	}
	// Stop registered a new original request. Never carry the revoked staging
	// source across apply/launch, or borrow a later global operation instead.
	cancelRoot()
	committedContext, cancelCommitted, err := a.beginCommittedUpdate(handoff)
	if err != nil {
		return "", err
	}
	defer a.endOperation(cancelCommitted)
	rootContext, timeoutCommitted := context.WithTimeout(committedContext, 30*time.Second)
	defer timeoutCommitted()
	if err := m.VerifyPayload(tmp); err != nil {
		handoff.failed(err)
		return "", err
	}
	if err := a.validatePairRoot(rootContext, root); err != nil {
		handoff.failed(err)
		return "", err
	}

	// 3. Атомарная подмена: текущий exe → .old, staged → на место exe.
	old := exe + ".old"
	if err := a.withVerifiedUpdateSourceLocked(verified, func() error {
		if !lifecyclePublicationCurrent(rootContext) {
			return context.Canceled
		}
		mutationAttempted = true // an OS error is not proof that no installed name changed
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return fmt.Errorf("update: rename текущего exe: %w (exe запущен?)", err)
		}
		if err := os.Rename(tmp, exe); err != nil {
			return errors.Join(fmt.Errorf("update: подмена: %w", err), os.Rename(old, exe))
		}
		removeStaged = false // moved name must not be unlinked after releasing writer scope
		return nil
	}); err != nil {
		_ = os.Remove(tmp)
		handoff.failed(err)
		return "", err
	}

	if err := a.validatePairRoot(rootContext, root); err != nil {
		handoff.failed(err)
		return "", err
	}
	handoff.requireFreshImage()
	if err := a.closePairRoot(); err != nil {
		handoff.failed(err)
		return "", err
	}
	return a.launchUpdatedApplication(rootContext, handoff, exe)
}

func (a *App) launchUpdatedApplication(ctx context.Context, handoff *restartHandoff, exe string) (string, error) {
	if !lifecyclePublicationCurrent(ctx) || handoff == nil || !handoff.codeAcceptanceCurrent() {
		return "", context.Canceled
	}
	// Reuse the existing bounded --await-previous single-instance handoff.
	// No generated shell string or unbounded PID polling; Start is not readiness.
	args := modeSwitchArgs(tunRequested(), handoff.request.resumeRequested)
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		// Новый не стартовал: exe на месте корректный (проверен sha256),
		// владелец запустит вручную. Честный отказ.
		launchErr := fmt.Errorf("update: запрос запуска новой версии: %w (запустите приложение вручную)", err)
		handoff.failed(launchErr)
		return "", launchErr
	}
	handoff.markLaunched()
	_ = cmd.Process.Release()

	// 5. Выход старого. requestQuit делает cleanup-пути, но ядро уже остановлено;
	// короткая задержка — дать журналу записаться.
	a.appendLog("info", "update: запуск новой версии запрошен; готовность ещё не подтверждена — завершаю текущий")
	a.requestQuit(700 * time.Millisecond)
	return "обновление применено — приложение перезапускается", nil
}

// updateStartupHousekeeping — вызывается при старте нового процесса:
// поднять этаж версий (антидаунгрейд от отката) и удалить .old.
func updateStartupHousekeeping() error {
	if runningVersion == "dev" {
		return nil // dev-сборки не участвуют в цепочке обновлений
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := update.CheckPairPending(filepath.Dir(exe)); err != nil {
		return err // pending recovery retains both the floor and old bytes
	}
	if err := checkRunningVersionFloor(); err != nil {
		return err
	}
	v, err := update.ParseVersion(runningVersion)
	if err != nil {
		return err
	}
	if actual, err := update.RaiseFloor(v); err != nil || actual.Compare(v) > 0 {
		return errors.Join(update.ErrFloorUnavailable, err)
	}
	_ = os.Remove(exe + ".old")
	return nil
}

// Exclusive per-operation staging, bounded before copy; the caller separately
// authenticates the staged bytes. Same-user reparse/ACL/native gates stay open.
func stageUpdatePayload(src, dir string, size int64) (path string, finalErr error) {
	if size <= 0 || size > update.MaxPayloadSize {
		return "", errors.New("update: staging size outside bound")
	}
	info, err := os.Lstat(src)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return "", errors.New("update: payload must be a regular file of declared size")
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	opened, err := in.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) || opened.Size() != size {
		return "", errors.New("update: source changed while opening")
	}
	out, err := os.CreateTemp(dir, ".snowden-update-*.exe")
	if err != nil {
		return "", err
	}
	created := out.Name()
	defer func() {
		_ = out.Close()
		if finalErr != nil {
			_ = os.Remove(created)
		}
	}()
	n, err := io.Copy(out, io.LimitReader(in, size+1))
	if err != nil || n != size {
		return "", errors.New("update: staging copy/length failed")
	}
	if err := out.Sync(); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return created, nil
}
