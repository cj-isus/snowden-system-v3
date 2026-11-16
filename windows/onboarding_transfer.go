package main

import "errors"

// The encrypted container transports credential values; root/epoch/recovery
// authority still comes only from the same explicit independent approvals as
// public import. Existing legacy Preview/ApplyOnboardingString remain intact.
type ProtectedProfileTransferRequest struct {
	Transport               string `json:"transport"`
	Passphrase              string `json:"passphrase"`
	Mode                    string `json:"mode"`
	ApprovedRootSHA256      string `json:"approved_root_sha256,omitempty"`
	ApprovedPinPolicySHA256 string `json:"approved_pin_policy_sha256,omitempty"`
	ApprovedTargetSHA256    string `json:"approved_target_sha256,omitempty"`
}

func (a *App) PreviewProtectedProfileTransfer(request ProtectedProfileTransferRequest) (*OnboardingPreview, error) {
	return a.importProtectedProfileTransfer(request, false)
}

func (a *App) ApplyProtectedProfileTransfer(request ProtectedProfileTransferRequest, confirm bool) (*OnboardingPreview, error) {
	if !confirm {
		return nil, errors.New("onboarding: подтвердите зашифрованный перенос после превью")
	}
	return a.importProtectedProfileTransfer(request, true)
}

func (a *App) importProtectedProfileTransfer(request ProtectedProfileTransferRequest, commit bool) (*OnboardingPreview, error) {
	intent, err := publicProfileIntent(PublicProfileRequest{Mode: request.Mode, ApprovedRootSHA256: request.ApprovedRootSHA256, ApprovedPinPolicySHA256: request.ApprovedPinPolicySHA256, ApprovedTargetSHA256: request.ApprovedTargetSHA256}, commit)
	if err != nil {
		return nil, err
	}
	return a.importProfileBundleIntent(request.Transport, request.Passphrase, intent)
}
