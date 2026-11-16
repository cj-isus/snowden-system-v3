package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strconv"
	"time"
)

var errUpdateApproval = errors.New("update: prepare a fresh preview of this exact package")

// Original body phase only: false on an ordinary returned preparation failure
// permits a fresh check, not a claim that retained delivery history is unchanged.
type updateInstallProgress struct{ entered bool }

// Process-local consent to a target, never installation or trust authority.
// The original source/session and monotonic expiry cannot be supplied by JS.
type updatePreviewApproval struct {
	token, directory, session string
	manifest, sidecar         [sha256.Size]byte
	registration              intentRegistration
	source                    context.Context // original intent/path, not preview's short operation context
	expires                   time.Time
}

func (approval *updatePreviewApproval) matchesManifest(raw []byte) bool {
	return approval.current() && sha256.Sum256(raw) == approval.manifest
}

func (approval *updatePreviewApproval) matchesSidecar(raw []byte) bool {
	return approval.current() && sha256.Sum256(raw) == approval.sidecar
}

func (approval *updatePreviewApproval) current() bool {
	return approval != nil && time.Now().Before(approval.expires) && lifecyclePublicationCurrent(approval.source)
}

func (a *App) beginUpdatePreview() (intentRegistration, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	registration, err := a.restartIntentLocked()
	if err != nil || a.updatePreviewBusy || a.uiLifecycle != nil && (a.uiLifecycle.positive != nil || a.uiLifecycle.stop != nil) {
		return intentRegistration{}, "", context.Canceled
	}
	a.updatePreviewBusy, a.updateApproval = true, nil
	session := ""
	if a.uiLifecycle != nil {
		session = a.uiLifecycle.session
	}
	return registration, session, nil
}

func (a *App) finishUpdatePreview() {
	a.mu.Lock()
	a.updatePreviewBusy = false // only the original preview frame releases its slot
	a.mu.Unlock()
}

func (a *App) approveUpdatePreview(directory, session string, registration intentRegistration, source context.Context, verified *verifiedUpdate) (*updatePreviewApproval, error) {
	if verified == nil || verified.manifest == nil {
		return nil, errUpdateApproval
	}
	token, err := publicSourceToken() // existing bounded cryptographic token generator
	if err != nil {
		return nil, errUpdateApproval
	}
	approval := &updatePreviewApproval{token: token, directory: filepath.Clean(directory), session: session,
		registration: registration, source: source, manifest: sha256.Sum256(verified.raw), sidecar: sha256.Sum256(verified.sidecar),
		expires: time.Now().Add(5 * time.Minute)}
	a.mu.Lock()
	defer a.mu.Unlock()
	current, err := a.restartIntentLocked()
	if err != nil || current != registration || !approval.current() || !a.updatePreviewBusy ||
		a.uiLifecycle != nil && (a.uiLifecycle.session != session || a.uiLifecycle.positive != nil || a.uiLifecycle.stop != nil) {
		return nil, errUpdateApproval
	}
	a.updateApproval = approval
	return approval, nil
}

// App.mu held, before waiting for updateMu/operationMu. A matching approval is
// consumed even on expiry/source failure. An unrelated token cannot consume it.
func (a *App) consumeUpdateApprovalLocked(token, session string) (*updatePreviewApproval, error) {
	approval := a.updateApproval
	if approval == nil || approval.token != token {
		return nil, errUpdateApproval
	}
	a.updateApproval = nil
	current, err := a.restartIntentLocked()
	if err != nil || current != approval.registration || session != approval.session || a.updatePreviewBusy || !approval.current() {
		return nil, errUpdateApproval
	}
	return approval, nil
}

func updateManifestIdentity(approval *updatePreviewApproval) string {
	return hex.EncodeToString(approval.manifest[:])
}

func formatUpdateIntent(registration intentRegistration) string {
	return strconv.FormatUint(registration.intent, 10)
}
