package main

// onboarding.go — экспорт/импорт состояния устройства (V2-049/F8).
//
// Экспорт: доверенные ключи + подписанный envelope + секреты (из DPAPI)
// шифруются scrypt+AES-GCM от парольной фразы в transport-строку; она же
// кодируется в QR (PNG data-URL) и сохраняется в файл по выбору владельца
// (диалоги — в onboarding_dialogs.go).
//
// Импорт (fail-closed): transport-строка расшифровывается; подпись, роли,
// сохранённая история, pin policy и явный режим проходят общий private gate.
// Бандл сам не назначает authority: enrollment требует независимых approvals.
// Применение — только по явному подтверждению (confirm=true), иначе
// возвращается превью с отпечатками (без секретов).
// Общий strict preview/prepare и journaled commit — onboarding_import.go.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/skip2/go-qrcode"
	"github.com/snowden-system/windows/backend/onboarding"
)

// OnboardingKeyPreview — отпечаток доверенного ключа из бандла (для превью).
type OnboardingKeyPreview struct {
	KeyID       string `json:"key_id"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment,omitempty"`
}

// OnboardingPreview — что покажет UI до подтверждения импорта.
type OnboardingPreview struct {
	BootstrapPolicySHA256  string                 `json:"bootstrap_policy_sha256,omitempty"` // enforced independent baseline; not copied from profile JSON
	CreatedAt              string                 `json:"created_at"`
	DeviceName             string                 `json:"device_name,omitempty"`
	Channels               int                    `json:"channels"`
	Secrets                int                    `json:"secrets"`
	SplitDirect            int                    `json:"split_direct"`
	Keys                   []OnboardingKeyPreview `json:"keys"`
	BundleVer              uint64                 `json:"bundle_version"`
	CurrentVer             uint64                 `json:"current_version"`
	WouldDowngrade         bool                   `json:"would_downgrade"`
	AuthorityMode          string                 `json:"authority_mode"`
	SignerKeyID            string                 `json:"signer_key_id"`
	SignerFingerprint      string                 `json:"signer_fingerprint"`
	RecoveryIssuerKeyID    string                 `json:"recovery_issuer_key_id,omitempty"`
	TargetSHA256           string                 `json:"target_sha256"`
	RetiredPublishers      []string               `json:"retired_publishers,omitempty"`
	DeliveryAnchorSHA256   string                 `json:"delivery_anchor_sha256,omitempty"`
	DeliveryRootVersion    int64                  `json:"delivery_root_version,omitempty"`
	DeliveryValidUntil     string                 `json:"delivery_valid_until,omitempty"`
	ValidUntil             string                 `json:"valid_until,omitempty"` // actual minimum of signed profile/proofs/permits
	BundleEpoch            uint64                 `json:"bundle_epoch"`
	CurrentEpoch           uint64                 `json:"current_epoch"`
	EpochGrantSHA256       string                 `json:"epoch_grant_sha256,omitempty"`
	BundleIdentity         string                 `json:"bundle_identity"`
	CurrentIdentity        string                 `json:"current_identity"`
	SourceTrustSHA256      string                 `json:"source_trust_sha256,omitempty"`
	PinPolicyVersion       string                 `json:"pin_policy_version,omitempty"`
	PinPolicySHA256        string                 `json:"pin_policy_sha256,omitempty"`
	CurrentPinPolicySHA256 string                 `json:"current_pin_policy_sha256,omitempty"`
	PinPolicyValidUntil    string                 `json:"pin_policy_valid_until,omitempty"`
	PinPolicyRequired      bool                   `json:"pin_policy_required"`
}

// ExportOnboarding — собрать transport-строку бандла из текущего состояния.
func (a *App) ExportOnboarding(passphrase string) (string, error) {
	if err := onboardingPassphraseError(passphrase); err != nil {
		return "", err
	}
	var transport string
	err := a.withTransferSource(func(ctx context.Context) error {
		var inner error
		transport, inner = a.exportLegacyProfile(ctx, passphrase)
		return inner
	})
	if err != nil {
		return "", err
	}
	return transport, nil
}

func onboardingPassphraseError(passphrase string) error {
	if len(passphrase) > onboarding.MaxPassphraseSize {
		return onboarding.ErrTooLarge
	}
	if len(passphrase) < 8 {
		return errors.New("парольная фраза слишком короткая (минимум 8 символов)")
	}
	return nil
}

// OnboardingQRResult — результат экспорта с QR (transport-строка + data-URL).
// Wails отображает только (T, error) — потому структура, а не два значения.
type OnboardingQRResult struct {
	Transport string `json:"transport"`
	DataURL   string `json:"dataUrl"`
}

// ExportOnboardingQR — то же, что ExportOnboarding, плюс PNG data-URL QR-кода.
func (a *App) ExportOnboardingQR(passphrase string) (*OnboardingQRResult, error) {
	if err := onboardingPassphraseError(passphrase); err != nil {
		return nil, err
	}
	var result *OnboardingQRResult
	err := a.withTransferSource(func(ctx context.Context) error {
		transport, err := a.exportLegacyProfile(ctx, passphrase)
		if err != nil {
			return err
		}
		if !profileRequestCurrent(ctx) {
			return context.Canceled
		}
		png, err := qrcode.Encode(transport, qrcode.Medium, 640)
		if err != nil {
			return fmt.Errorf("onboarding: qr: %w", err)
		}
		if !profileRequestCurrent(ctx) {
			return context.Canceled
		}
		result = &OnboardingQRResult{Transport: transport, DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// countEnvelopeChannels — количество каналов в подписанном envelope-файле.
func countEnvelopeChannels(envRaw []byte) int {
	var probe struct {
		Channels []json.RawMessage `json:"channels"`
	}
	if json.Unmarshal(envRaw, &probe) != nil {
		return 0
	}
	return len(probe.Channels)
}
