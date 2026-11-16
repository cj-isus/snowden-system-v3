package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
)

type PublicProfileSourceRequest struct {
	RequestID string                     `json:"request_id"`
	Purpose   string                     `json:"purpose,omitempty"` // import (default), export to fresh destination
	Source    onboarding.SourceSelection `json:"source"`
	Import    PublicProfileRequest       `json:"import"`
}

type PublicProfileSourcePreview struct {
	Token                      string             `json:"token"`
	RequestID                  string             `json:"request_id"`
	Route                      string             `json:"route"`
	ReleaseID                  string             `json:"release_id"`
	PackageSHA256              string             `json:"package_sha256"`
	ExpiresAt                  string             `json:"expires_at"`
	Profile                    *OnboardingPreview `json:"profile"`
	Purpose                    string             `json:"purpose"`
	DestinationRootSHA256      string             `json:"destination_root_sha256,omitempty"`
	DestinationPinPolicySHA256 string             `json:"destination_pin_policy_sha256,omitempty"`
}

type pendingPublicProfile struct {
	token            string
	request          PublicProfileRequest // exact downloaded JSON + explicit intent
	preview          *OnboardingPreview
	expires          time.Time
	purpose          string
	exportRoot       string
	exportCheckpoint string
	pinPreview       *PinPolicyPreview // scalar approval for the mutually exclusive policy-only draft
	registration     intentRegistration
	parent           context.Context // original App lifetime, not the short canceled preview child
}

type publicProfileSourceState struct {
	mu        sync.Mutex
	active    *publicSourceFlight
	requestID string
	pending   *pendingPublicProfile // one bounded public-only draft; no disk persistence
}

func validSourceRequestID(id string) bool {
	if len(id) == 0 || len(id) > 96 {
		return false
	}
	for _, ch := range id {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}

func publicSourceToken() (string, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce[:]), nil
}

func (a *App) PreviewPublicProfileSource(request PublicProfileSourceRequest) (*PublicProfileSourcePreview, error) {
	if !validSourceRequestID(request.RequestID) || request.Import.JSON != "" || a.closing.Load() {
		return nil, errors.New("onboarding: некорректный запрос публичного профиля")
	}
	if request.Purpose == "" {
		request.Purpose = "import"
	}
	if request.Purpose == "import" {
		if _, err := publicProfileIntent(request.Import, false); err != nil {
			return nil, err
		}
	} else if request.Purpose != "export" || request.Import.Mode != "profile" || request.Import.ApprovedRootSHA256 != "" || request.Import.ApprovedPinPolicySHA256 != "" || request.Import.ApprovedTargetSHA256 != "" {
		return nil, errors.New("onboarding: выберите отдельное действие импорта или переноса установленного профиля")
	}
	if _, err := request.Source.Validate(); err != nil {
		return nil, err
	}
	flight, err := a.beginPublicSource(request.RequestID)
	if err != nil {
		return nil, err
	}
	var result *PublicProfileSourcePreview
	err = a.runPublicSourceFlight(flight, func() error {
		var inner error
		result, inner = a.previewPublicProfileSource(flight, request)
		return inner
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) previewPublicProfileSource(flight *publicSourceFlight, request PublicProfileSourceRequest) (*PublicProfileSourcePreview, error) {
	ctx := flight.ctx
	s := &a.publicProfiles
	address, routeFence, err := a.publicProfileSourceRoute(request.Source.Route)
	if err != nil {
		return nil, err
	}
	if err = routeFence(); err != nil {
		return nil, err
	}
	downloaded, err := onboarding.FetchPublicProfile(ctx, request.Source, address)
	if err != nil {
		return nil, err
	}
	if err = routeFence(); err != nil {
		return nil, err
	}
	request.Import.JSON = string(downloaded.JSON)
	var preview *OnboardingPreview
	var exportRoot, exportPolicy, exportCheckpoint string
	if request.Purpose == "export" {
		payload, decodeErr := onboarding.DecodePublicProfile(downloaded.JSON)
		if decodeErr != nil {
			return nil, decodeErr
		}
		prepared, _, _, prepareErr := a.prepareEnrolledExport(ctx, payload, nil, "")
		if prepareErr != nil {
			return nil, prepareErr
		}
		preview, exportRoot, exportCheckpoint = prepared.preview, prepared.portableRoot, prepared.source.checkpoint
		exportPolicy = prepared.preview.PinPolicySHA256
	} else {
		preview, err = a.importPublicProfileContext(ctx, request.Import, false, nil, time.Time{}) // common SDK/application/credential gate
		if err != nil {
			return nil, err
		}
	}
	if err = routeFence(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	now, err := a.trustClock().Now(time.Time{})
	if err != nil {
		return nil, err
	}
	expires := now.Add(2 * time.Minute)
	deadline, err := time.Parse(time.RFC3339Nano, preview.ValidUntil)
	if err != nil {
		return nil, errors.New("onboarding: общий срок действия превью недоступен")
	}
	if deadline.Before(expires) {
		expires = deadline
	}
	if !expires.After(now) {
		return nil, errors.New("onboarding: срок превью истёк")
	}
	token, err := publicSourceToken()
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(downloaded.JSON)
	s.mu.Lock()
	if s.active != flight || s.requestID != request.RequestID || !a.publicSourceCurrent(flight) {
		s.mu.Unlock()
		return nil, errors.New("onboarding: превью отменено или заменено")
	}
	approval := *preview // returned UI object must not alias retained scalar approval fields
	s.pending = &pendingPublicProfile{token: token, request: request.Import, preview: &approval, expires: expires, purpose: request.Purpose, exportRoot: exportRoot, exportCheckpoint: exportCheckpoint, registration: flight.registration, parent: flight.parent}
	s.mu.Unlock()
	return &PublicProfileSourcePreview{Token: token, RequestID: request.RequestID, Route: request.Source.Route, ReleaseID: downloaded.ReleaseID, PackageSHA256: hex.EncodeToString(sum[:]), ExpiresAt: expires.Format(time.RFC3339), Profile: preview, Purpose: request.Purpose, DestinationRootSHA256: exportRoot, DestinationPinPolicySHA256: exportPolicy}, nil
}

func (a *App) ApplyPublicProfileSource(token, approvedTarget string, confirm bool) (*OnboardingPreview, error) {
	if !confirm || len(token) != 64 || a.closing.Load() {
		return nil, errors.New("onboarding: подтвердите показанное превью")
	}
	pending, flight, err := a.beginPublicSourceApply(token, "import")
	if err != nil {
		return nil, err
	}
	var result *OnboardingPreview
	err = a.runPublicSourceFlight(flight, func() error {
		var inner error
		result, inner = a.applyPublicProfileSource(flight, pending, approvedTarget)
		return inner
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) applyPublicProfileSource(flight *publicSourceFlight, pending *pendingPublicProfile, approvedTarget string) (*OnboardingPreview, error) {
	now, err := a.trustClock().Now(time.Time{})
	if err != nil || !now.Before(pending.expires) {
		return nil, errors.New("onboarding: превью истекло или часы требуют проверки")
	}
	request := pending.request
	switch request.Mode {
	case "recover", "epoch_recover", "epoch_enroll":
		if approvedTarget == "" || approvedTarget != pending.preview.TargetSHA256 || request.ApprovedTargetSHA256 != "" && request.ApprovedTargetSHA256 != approvedTarget {
			return nil, errors.New("onboarding: подтвердите точный SHA256 показанного профиля")
		}
		request.ApprovedTargetSHA256 = approvedTarget
	default:
		if approvedTarget != "" {
			return nil, errors.New("onboarding: обычный импорт не получает recovery approval")
		}
	}
	return a.importPublicProfileContext(flight.ctx, request, true, pending.preview, pending.expires) // no network/refetch between preview/apply
}

// Request IDs fence UI ownership, not authority. Cancel cannot undo an apply
// already explicitly approved; Stop/Disconnect retain their existing lifecycle.
func (a *App) CancelPublicProfileSource(requestID string) bool {
	s := &a.publicProfiles
	s.mu.Lock()
	flight := s.active
	if requestID == "" || s.requestID != requestID || flight != nil && flight.applying {
		s.mu.Unlock()
		return false
	}
	s.requestID, s.pending = "", nil
	s.mu.Unlock()
	if flight != nil {
		flight.cancel()
	} // exact original callback, outside App/state locks
	return true
}

func (a *App) publicProfileSourceRoute(route string) (string, func() error, error) {
	if route != "protected" {
		return "", func() error { return nil }, nil // selection.Validate requires explicit local/bootstrap intent
	}
	runtime_.mu.Lock()
	manager, session := runtime_.manager, runtime_.cc
	runtime_.mu.Unlock()
	if manager == nil || session == nil {
		return "", nil, errors.New("onboarding: защищённое подключение не запущено")
	}
	executable, err := os.Executable()
	if err != nil {
		return "", nil, errors.New("onboarding: не удалось определить процесс для проверки правил маршрутизации")
	}
	for _, excluded := range session.SplitDirect {
		if strings.EqualFold(excluded, filepath.Base(executable)) {
			return "", nil, errors.New("onboarding: приложение исключено из VPN правилами разделения; выберите маршрут загрузки явно")
		}
	}
	owner, generation := manager.Owner()
	fence := func() error {
		if !a.sessionCurrent(session) || manager.State() != core.StateRunning || a.closing.Load() {
			return errors.New("onboarding: защищённое подключение изменилось")
		}
		currentOwner, currentGeneration := manager.Owner()
		if currentOwner != owner || currentGeneration != generation {
			return errors.New("onboarding: поколение защищённого подключения изменилось")
		}
		return nil
	}
	return net.JoinHostPort(render.MixedListenAddress, strconv.Itoa(render.MixedListenPort)), fence, nil
}
