package main

import (
	"encoding/hex"
	"errors"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
)

func canonicalApprovalSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

// These are separate human-supplied out-of-band approvals, never values
// inferred from a downloaded package or from an empty device history.
func checkBootstrapIntent(intent profileImportIntent) error {
	if intent.approvedRoot == "" {
		if intent.approvedPinPolicy != "" {
			return errors.New("onboarding: подтверждение первого списка отзывов допустимо только при явной первой установке вместе с подтверждением корня доверия")
		}
		return nil
	}
	if !canonicalApprovalSHA256(intent.approvedRoot) || !canonicalApprovalSHA256(intent.approvedPinPolicy) || intent.recovery || intent.epochRecovery {
		return errors.New("onboarding: для первой установки независимо подтвердите SHA256 root.json и актуального списка отзывов (по 64 строчных hex-символа)")
	}
	return nil
}

// This only binds an independently approved minimum baseline to exact bytes.
// Full private SDK proof, roles, times, grants and cumulative bans still run;
// possession of either hash is not signing authority or proof of global latest.
func checkBootstrapPinApproval(policy *metadata.TrustPolicy, payload *onboarding.BundlePayload, intent profileImportIntent) error {
	if err := checkBootstrapIntent(intent); err != nil {
		return err
	}
	if policy == nil || payload == nil {
		return errors.New("onboarding: источник доверия для первой установки недоступен")
	}
	if policy.Delivery == nil && payload.Delivery != nil {
		if intent.approvedRoot == "" || intent.approvedPinPolicy == "" || len(payload.PinPolicy) == 0 {
			return errors.New("onboarding: первая установка требует список отзывов и его SHA256 из независимого доверенного источника")
		}
		if publicSHA(payload.PinPolicy) != intent.approvedPinPolicy {
			return errors.New("onboarding: список отзывов отличается от независимо подтверждённого; получите актуальный комплект и сверку у владельца")
		}
		return nil
	}
	if intent.approvedRoot != "" || intent.approvedPinPolicy != "" {
		if policy.Delivery != nil {
			return errors.New("onboarding: доверие уже настроено; первая установка не заменяет его историю. Выберите обновление или соответствующее восстановление")
		}
		return errors.New("onboarding: пакет первой установки не содержит проверяемой цепочки доставки")
	}
	return nil
}
