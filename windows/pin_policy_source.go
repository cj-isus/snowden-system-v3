package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/onboarding"
)

type PinPolicySourceRequest struct {
	RequestID string                     `json:"request_id"`
	Source    onboarding.SourceSelection `json:"source"`
}

type PinPolicySourcePreview struct {
	Token         string            `json:"token"`
	RequestID     string            `json:"request_id"`
	Route         string            `json:"route"` // protected/bootstrap/local/file
	ReleaseID     string            `json:"release_id,omitempty"`
	PackageSHA256 string            `json:"package_sha256"`
	ExpiresAt     string            `json:"expires_at"`
	Policy        *PinPolicyPreview `json:"policy"`
}

type pinPolicyDraftInput struct {
	json      []byte
	route     string
	releaseID string
	fence     func() error
}

func (a *App) PreviewPinPolicySource(request PinPolicySourceRequest) (*PinPolicySourcePreview, error) {
	if request.Source.TargetPath != delivery.PinPolicyPath {
		return nil, errors.New("pin policy: требуется отдельный источник списка отзывов")
	}
	return a.previewPinPolicyDraft(request.RequestID, func(ctx context.Context) (*pinPolicyDraftInput, error) {
		address, routeFence, err := a.publicProfileSourceRoute(request.Source.Route)
		if err != nil {
			return nil, err
		}
		if err = routeFence(); err != nil {
			return nil, err
		}
		downloaded, err := onboarding.FetchPinPolicy(ctx, request.Source, address)
		if err != nil {
			return nil, err
		}
		if err = routeFence(); err != nil {
			return nil, err
		}
		return &pinPolicyDraftInput{json: downloaded.JSON, route: request.Source.Route, releaseID: downloaded.ReleaseID, fence: routeFence}, nil
	})
}

// The file chooser and decoder run only in the same bounded source admission.
// Canceling the chooser returns nil; it never silently selects another source.
func (a *App) PreviewPinPolicyFile(requestID string) (*PinPolicySourcePreview, error) {
	return a.previewPinPolicyDraft(requestID, func(ctx context.Context) (*pinPolicyDraftInput, error) {
		raw, err := a.selectPinPolicyFile(ctx)
		if err != nil || raw == nil {
			return nil, err
		}
		return &pinPolicyDraftInput{json: raw, route: "file"}, nil
	})
}

func (a *App) previewPinPolicyDraft(requestID string, load func(context.Context) (*pinPolicyDraftInput, error)) (*PinPolicySourcePreview, error) {
	flight, err := a.beginPublicSource(requestID)
	if err != nil {
		return nil, err
	}
	var result *PinPolicySourcePreview
	err = a.runPublicSourceFlight(flight, func() error {
		var inner error
		result, inner = a.preparePinPolicyDraft(flight, requestID, load)
		return inner
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) preparePinPolicyDraft(flight *publicSourceFlight, requestID string, load func(context.Context) (*pinPolicyDraftInput, error)) (*PinPolicySourcePreview, error) {
	ctx := flight.ctx
	input, err := load(ctx)
	if err != nil || input == nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if len(input.json) == 0 || len(input.json) > onboarding.MaxPinPolicyCapsuleSize {
		return nil, onboarding.ErrTooLarge
	}
	preview, err := a.importPinPolicyContext(ctx, string(input.json), PinPolicyApproval{}, false, time.Time{}) // read-only enrolled SDK/monotonic gate
	if err != nil {
		return nil, err
	}
	if input.fence != nil {
		if err = input.fence(); err != nil {
			return nil, err
		}
	}
	deadline, err := time.Parse(time.RFC3339, preview.ValidUntil)
	if err != nil {
		return nil, errors.New("pin policy: срок действия списка недоступен")
	}
	now, err := a.trustClock().Now(time.Time{})
	if err != nil {
		return nil, err
	}
	expires := now.Add(2 * time.Minute)
	if deadline.Before(expires) {
		expires = deadline
	}
	if !now.Before(expires) {
		return nil, errors.New("pin policy: срок превью истёк")
	}
	token, err := publicSourceToken()
	if err != nil {
		return nil, err
	}
	s := &a.publicProfiles
	s.mu.Lock()
	if s.active != flight || s.requestID != requestID || !a.publicSourceCurrent(flight) {
		s.mu.Unlock()
		return nil, errors.New("pin policy: запрос отменён или заменён")
	}
	approval := *preview // returned UI object cannot mutate the retained scalar approval
	s.pending = &pendingPublicProfile{token: token, request: PublicProfileRequest{JSON: string(input.json)}, expires: expires, purpose: "pin_policy", pinPreview: &approval, registration: flight.registration, parent: flight.parent}
	s.mu.Unlock()
	return &PinPolicySourcePreview{Token: token, RequestID: requestID, Route: input.route, ReleaseID: input.releaseID, PackageSHA256: preview.Approval.CapsuleSHA256, ExpiresAt: expires.UTC().Format(time.RFC3339), Policy: preview}, nil
}

func (a *App) ApplyPinPolicySource(token string, confirm bool) (*PinPolicyPreview, error) {
	if !confirm || len(token) != 64 || a.closing.Load() {
		return nil, errors.New("pin policy: подтвердите показанное превью списка отзывов")
	}
	pending, flight, err := a.beginPublicSourceApply(token, "pin_policy")
	if err != nil {
		return nil, err
	}
	var result *PinPolicyPreview
	err = a.runPublicSourceFlight(flight, func() error {
		if pending.pinPreview == nil {
			return errPublicSourceUnavailable
		}
		var inner error
		// Fresh private checks remain; the short preview child is not reused.
		result, inner = a.importPinPolicyContext(flight.ctx, pending.request.JSON, pending.pinPreview.Approval, true, pending.expires)
		return inner
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
