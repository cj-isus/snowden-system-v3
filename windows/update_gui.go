package main

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/core"
)

var activeUpdateGUISession *core.UpdateGUISession // fixed dispatcher result, before Wails
var updateGUIBootstrapOwners *App                 // partial preparation owner, retained until terminal os.Exit

// Same actual candidate/SDK/native gate as the fixed acceptance worker. Transfer
// the exact image owner to bootstrap only after confirmed writer-root closure;
// on error/panic all partial native owners remain reachable until os.Exit.
func prepareUpdateGUIImage(ctx context.Context, expectedSHA string) (image *core.InstalledPairLease, finalErr error) {
	if updateGUIBootstrapOwners != nil {
		return nil, core.ErrUpdateGUIBootstrap
	}
	app := NewApp()
	updateGUIBootstrapOwners = app // before the first native acquisition
	app.ctx, app.installedImageChanged = ctx, true
	app.updateMu.Lock()
	defer app.updateMu.Unlock()
	app.operationMu.Lock()
	defer app.operationMu.Unlock()
	transferred := false
	defer func() {
		if recovered := recover(); recovered != nil {
			finalErr = core.ErrUpdateGUIBootstrap
		}
		if !transferred {
			// Return even a partial image owner. Unknown root handles remain owned
			// by this app until the terminal dispatcher exits this process.
			image = app.pairImage
			finalErr = errors.Join(finalErr, core.ErrUpdateGUIBootstrap)
		}
	}()
	root, verified, err := app.prepareInstalledUpdateCandidate(ctx, false, expectedSHA)
	if err != nil {
		return app.pairImage, err
	}
	if err := app.validatePairRoot(ctx, root); err != nil {
		return app.pairImage, err
	}
	if err := app.pairImage.Validate(ctx); err != nil {
		return app.pairImage, err
	}
	if err := app.withVerifiedUpdatePurposeLocked(verified, true, func() error { return ctx.Err() }); err != nil {
		return app.pairImage, err
	}
	if err := app.closePairRoot(); err != nil {
		return app.pairImage, err
	}
	image, app.pairImage = app.pairImage, nil
	updateGUIBootstrapOwners = nil // writer closed and image transferred; no partial owner remains
	transferred = true
	return image, nil
}

// Wails invokes this only after its DOM-ready event, after original parent exit.
// Startup has already recovered storage but deliberately started no producers.
func (a *App) updateGUIDOMReady(ctx context.Context) {
	if a.updateGUIStartupDone == nil || ctx == nil || ctx.Err() != nil {
		return
	}
	select {
	case <-a.updateGUIStartupDone:
	case <-ctx.Done():
		return
	}
	a.mu.Lock()
	if !a.installedImageChanged || a.updateGUIRendererEpoch == ^uint64(0) {
		a.mu.Unlock()
		return
	}
	a.updateGUIRendererEpoch++
	rendererEpoch := a.updateGUIRendererEpoch
	if a.updateGUIDOMStarted {
		a.mu.Unlock()
		return
	}
	a.updateGUIDOMStarted = true
	storageReady := a.updateGUIStorageReady
	registration := a.startupIntent // a late DOM callback cannot adopt a preceding Disconnect
	a.mu.Unlock()
	if !storageReady {
		return // initial profile/import recovery error remains authoritative
	}
	if err := awaitUpdateGUIRenderer(ctx); err != nil {
		a.appendLog("warn", "update: готовность интерфейса не подтверждена; установка и подключение остаются заблокированы")
		return // no floor/archive/prefix mutation based on a navigation label alone
	}
	if err := a.activateInstalledUpdate(rendererEpoch); err != nil {
		if errors.Is(err, errUpdateRecoveryRestarting) {
			a.appendLog("info", "update: подписанная пара восстановлена; приёмку продолжит новый процесс после перезапуска")
			return
		}
		a.appendLog("warn", "update: новое окно открыто, но принятие установки не завершено; подключение заблокировано")
		a.pushState()
		return
	}
	retained := strictProtectionRetained()
	a.mu.Lock()
	if ctx.Err() != nil || a.ctx == nil || a.ctx.Err() != nil || a.closing.Load() || a.installedImageChanged {
		a.mu.Unlock()
		return
	}
	// Clear only this bootstrap's old restriction, not retained protection or a
	// concurrent operation's outcome. Installation acceptance is never VPN Ready.
	if registration.currentLocked(a) && a.updateGUIRendererEpoch == rendererEpoch && a.restartCommit == nil && a.releaseCommit == nil && a.state.BlockedReason == "update_recovery_required" {
		a.state.BlockedReason, a.state.Error = "", ""
		a.state.State = "stopped"
		if retained {
			a.protectionDesired = true
			a.state.State, a.state.BlockedReason = "error", "protection_retained"
			a.state.Error = "Сохранена системная защита; подключение требует свежей проверки защищённого пути."
		}
	}
	a.mu.Unlock()
	a.pushState()
	a.startNetworkServices(nil, registration) // observers use App lifetime; automatic Connect still requires the initial registration
}
