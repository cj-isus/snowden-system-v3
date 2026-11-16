package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/onboarding"
)

// PublicProfileRequest selects a user intent explicitly. JSON contains public
// signed descriptors/proof only; credentials remain in the local vault or the
// separate encrypted transfer. Generated Wails/UI bindings are a later step.
type PublicProfileRequest struct {
	JSON                    string `json:"json"`
	Mode                    string `json:"mode"` // profile, enroll, recover, epoch_recover, epoch_enroll
	ApprovedRootSHA256      string `json:"approved_root_sha256,omitempty"`
	ApprovedPinPolicySHA256 string `json:"approved_pin_policy_sha256,omitempty"`
	ApprovedTargetSHA256    string `json:"approved_target_sha256,omitempty"`
}

func (a *App) PreviewPublicProfile(request PublicProfileRequest) (*OnboardingPreview, error) {
	return a.importPublicProfile(request, false)
}

func (a *App) ApplyPublicProfile(request PublicProfileRequest, confirm bool) (*OnboardingPreview, error) {
	if !confirm {
		return nil, errors.New("onboarding: применение публичного профиля требует подтверждения после превью")
	}
	return a.importPublicProfile(request, true)
}

func (a *App) importPublicProfile(request PublicProfileRequest, commit bool) (*OnboardingPreview, error) {
	return a.importPublicProfileApproved(request, commit, nil)
}

func (a *App) importPublicProfileApproved(request PublicProfileRequest, commit bool, expected *OnboardingPreview) (*OnboardingPreview, error) {
	return a.importPublicProfileContext(nil, request, commit, expected, time.Time{})
}

func (a *App) importPublicProfileContext(ctx context.Context, request PublicProfileRequest, commit bool, expected *OnboardingPreview, approvedUntil time.Time) (*OnboardingPreview, error) {
	ctx, _, err := a.captureProfileRequest(ctx)
	if err != nil {
		return nil, err
	}
	intent, err := publicProfileIntent(request, commit)
	if err != nil {
		return nil, err
	}
	intent.expectedPreview = expected
	intent.approvedUntil, intent.requestContext = approvedUntil, ctx
	if len(request.JSON) == 0 || len(request.JSON) > onboarding.MaxPublicProfileSize {
		return nil, onboarding.ErrTooLarge
	}
	payload, err := onboarding.DecodePublicProfile([]byte(request.JSON))
	if err != nil {
		return nil, err
	}
	return a.importPreparedProfilePayload(payload, intent)
}

func publicProfileIntent(request PublicProfileRequest, commit bool) (profileImportIntent, error) {
	intent := profileImportIntent{commit: commit, approvedPinPolicy: request.ApprovedPinPolicySHA256}
	switch request.Mode {
	case "profile":
		if request.ApprovedRootSHA256 != "" || request.ApprovedTargetSHA256 != "" {
			return intent, errors.New("onboarding: обычный импорт не меняет root/recovery authority")
		}
	case "enroll":
		if request.ApprovedRootSHA256 == "" || request.ApprovedTargetSHA256 != "" {
			return intent, errors.New("onboarding: enrollment требует отдельный out-of-band root SHA256")
		}
		intent.approvedRoot = request.ApprovedRootSHA256
	case "recover":
		if request.ApprovedRootSHA256 != "" || commit && request.ApprovedTargetSHA256 == "" {
			return intent, errors.New("onboarding: recovery требует target SHA256 и не назначает новый root")
		}
		intent.recovery, intent.approvedTarget = true, request.ApprovedTargetSHA256
	case "epoch_recover":
		if request.ApprovedRootSHA256 != "" || commit && request.ApprovedTargetSHA256 == "" {
			return intent, errors.New("onboarding: epoch recovery требует target SHA256 и прежнюю enrolled lineage")
		}
		intent.epochRecovery, intent.approvedTarget = true, request.ApprovedTargetSHA256
	case "epoch_enroll":
		if request.ApprovedRootSHA256 == "" || commit && request.ApprovedTargetSHA256 == "" {
			return intent, errors.New("onboarding: initial epoch enrollment требует отдельные root и target SHA256")
		}
		intent.epochEnrollment = true
		intent.approvedRoot, intent.approvedTarget = request.ApprovedRootSHA256, request.ApprovedTargetSHA256
	default:
		return intent, errors.New("onboarding: выберите явный режим profile, enroll, recover, epoch_recover или epoch_enroll")
	}
	return intent, checkBootstrapIntent(intent)
}

func samePublicProfileApproval(a, b *OnboardingPreview) bool {
	return a != nil && b != nil && a.TargetSHA256 == b.TargetSHA256 && a.SignerKeyID == b.SignerKeyID && a.SignerFingerprint == b.SignerFingerprint && a.AuthorityMode == b.AuthorityMode && a.BundleIdentity == b.BundleIdentity && a.CurrentIdentity == b.CurrentIdentity && a.DeliveryAnchorSHA256 == b.DeliveryAnchorSHA256 && a.EpochGrantSHA256 == b.EpochGrantSHA256 && a.SourceTrustSHA256 == b.SourceTrustSHA256 && a.PinPolicySHA256 == b.PinPolicySHA256 && a.PinPolicyVersion == b.PinPolicyVersion && a.CurrentPinPolicySHA256 == b.CurrentPinPolicySHA256 && a.PinPolicyRequired == b.PinPolicyRequired && a.ValidUntil == b.ValidUntil && a.BootstrapPolicySHA256 == b.BootstrapPolicySHA256
}
