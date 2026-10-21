package main

// onboarding.go — экспорт/импорт состояния устройства (V2-049/F8).
//
// Экспорт: доверенные ключи + подписанный envelope + секреты (из DPAPI)
// шифруются scrypt+AES-GCM от парольной фразы в transport-строку; она же
// кодируется в QR (PNG data-URL) и сохраняется в файл по выбору владельца
// (диалоги — в onboarding_dialogs.go).
//
// Импорт (fail-closed): transport-строка расшифровывается, подпись envelope
// проверяется ключами ИЗ бандла до записи envelope, антидаунгрейд по локальной
// version.txt, отпечатки ключей показываются владельцу ДО применения.
// Применение — только по явному подтверждению (confirm=true), иначе
// возвращается превью с отпечатками (без секретов).
// Общий strict preview/prepare и journaled commit — onboarding_import.go.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/secretvault"
)

// OnboardingKeyPreview — отпечаток доверенного ключа из бандла (для превью).
type OnboardingKeyPreview struct {
	KeyID       string `json:"key_id"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment,omitempty"`
}

// OnboardingPreview — что покажет UI до подтверждения импорта.
type OnboardingPreview struct {
	CreatedAt      string                 `json:"created_at"`
	DeviceName     string                 `json:"device_name,omitempty"`
	Channels       int                    `json:"channels"`
	Secrets        int                    `json:"secrets"`
	SplitDirect    int                    `json:"split_direct"`
	Keys           []OnboardingKeyPreview `json:"keys"`
	BundleVer      uint64                 `json:"bundle_version"`
	CurrentVer     uint64                 `json:"current_version"`
	WouldDowngrade bool                   `json:"would_downgrade"`
}

// ExportOnboarding — собрать transport-строку бандла из текущего состояния.
func (a *App) ExportOnboarding(passphrase string) (string, error) {
	if len(passphrase) > onboarding.MaxPassphraseSize {
		return "", onboarding.ErrTooLarge
	}
	if len(passphrase) < 8 {
		return "", errors.New("парольная фраза слишком короткая (минимум 8 символов)")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	// 1. Доверенные ключи (публичные данные).
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return "", fmt.Errorf("onboarding: store dir: %w", err)
	}
	store := &metadata.Store{Dir: dir}
	keys, present, err := store.LoadKeys()
	if err != nil {
		return "", fmt.Errorf("onboarding: ключи: %w", err)
	}
	if !present || len(keys) == 0 {
		return "", errors.New("onboarding: доверенных ключей нет — переносить нечего")
	}
	var kt []onboarding.TrustedKey
	for id, pub := range keys {
		kt = append(kt, onboarding.TrustedKey{
			KeyID:  id,
			Public: fmt.Sprintf("%x", []byte(pub)),
		})
	}

	// 2. Подписанный envelope «как есть» (байты файла, не ре-маршалинг:
	// подпись покрывает точную сериализацию).
	envRaw, err := os.ReadFile(store.EnvelopePath())
	if err != nil {
		return "", fmt.Errorf("onboarding: envelope: %w", err)
	}

	// 3. Секреты из DPAPI-хранилища (значения).
	metas, err := a.vault.List()
	if err != nil {
		return "", fmt.Errorf("onboarding: vault: %w", err)
	}
	var slots []onboarding.SecretSlot
	for _, m := range metas {
		if m.StoredValue == "" {
			continue // незаполненный слот переносить нечего
		}
		v, err := a.vault.Value(m.ID)
		if err != nil {
			if errors.Is(err, secretvault.ErrNotSet) {
				continue
			}
			return "", fmt.Errorf("onboarding: секрет %s: %w", m.ID, err)
		}
		slots = append(slots, onboarding.SecretSlot{Kind: m.Kind, ID: m.ID, Value: v})
	}

	payload := &onboarding.BundlePayload{
		Schema:      1,
		CreatedAt:   time.Now().UTC(),
		TrustedKeys: kt,
		Envelope:    json.RawMessage(envRaw),
		Secrets:     slots,
	}
	transport, err := onboarding.Seal(payload, passphrase)
	if err != nil {
		a.appendLogLocked("error", "onboarding export: "+err.Error())
		return "", err
	}
	a.appendLogLocked("info", fmt.Sprintf("onboarding: бандл собран (каналов в envelope: %d, секретов: %d)", countEnvelopeChannels(envRaw), len(slots)))
	return transport, nil
}

// OnboardingQRResult — результат экспорта с QR (transport-строка + data-URL).
// Wails отображает только (T, error) — потому структура, а не два значения.
type OnboardingQRResult struct {
	Transport string `json:"transport"`
	DataURL   string `json:"dataUrl"`
}

// ExportOnboardingQR — то же, что ExportOnboarding, плюс PNG data-URL QR-кода.
func (a *App) ExportOnboardingQR(passphrase string) (*OnboardingQRResult, error) {
	transport, err := a.ExportOnboarding(passphrase)
	if err != nil {
		return nil, err
	}
	png, err := qrcode.Encode(transport, qrcode.Medium, 640)
	if err != nil {
		return nil, fmt.Errorf("onboarding: qr: %w", err)
	}
	return &OnboardingQRResult{
		Transport: transport,
		DataURL:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	}, nil
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
