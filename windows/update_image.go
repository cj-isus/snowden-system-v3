package main

import (
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/update"
)

var (
	errInstalledImageChanged      = errors.New("файлы приложения могли измениться; закройте и заново запустите приложение, при ошибке запуска восстановите установку")
	errUpdatePairRequired         = errors.New("update: требуется полный подписанный комплект приложения и вспомогательного модуля")
	errUpdatePairInstallerPending = errors.New("update: установка этого комплекта приложения и вспомогательного модуля в текущей версии не поддерживается")
)

// App, root/transaction-directory custody and streaming transaction are source
// implementations. A pinned current-code acceptance worker owns its process;
// GUI bootstrap/original Job lifetime transfer and DOM-ready acceptance are now
// source implementations. Native execution, durability/crash and release gates
// remain unverified: public installation stays unavailable until acceptance.
// Protocol5 companion publication and actual Job/SDK readiness are source-wired
// before pair mutation. Native sharing/loader/crash/durability/OS acceptance is
// still absent; a source receipt cannot enable public installation.
// Neither source integration nor activation_protocol is an installer PASS.
func checkUpdateInstallSupport(m *update.Manifest) error {
	if m.Schema == update.PairSchema || m.NativeHelper != nil {
		return errUpdatePairInstallerPending
	}
	if nativeProbeHelperSHA256 != "" {
		return errUpdatePairRequired
	}
	return nil // legacy unpaired build only; all ordinary authority gates still apply
}

// Caller holds App.mu. The flag is independent of profile/intent/Disconnect;
// another valid import or retry cannot make this old mapped image current.
func (a *App) blockChangedImageLocked() {
	a.state.State = "error"
	a.state.CoreReady, a.state.ProbeRunning = false, false
	a.state.BlockedReason, a.state.Error = "installed_image_changed", errInstalledImageChanged.Error()
	a.nativeObservation = nil
	a.cancelFailClosedRetryLocked()
	a.sm.markBlocked(time.Now())
}

// Register with defer AFTER handoff.release's defer. It must run before that
// token opens the lifecycle gate, including panic/rename/restore/launch failure.
func (h *restartHandoff) requireFreshImage() {
	a := h.app
	a.mu.Lock()
	a.installedImageChanged = true
	if !h.launched && h.request.sameLocked(a) {
		a.blockChangedImageLocked()
	}
	a.mu.Unlock()
	a.pushState()
}
