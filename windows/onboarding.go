package main

// onboarding.go — экспорт/импорт состояния устройства (V2-049/F8).
//
// Экспорт: доверенные ключи + подписанный envelope + секреты (из DPAPI)
// шифруются scrypt+AES-GCM от парольной фразы в transport-строку; она же
// кодируется в QR (PNG data-URL) и сохраняется в файл по выбору владельца
// (диалоги — в onboarding_dialogs.go).
//
// Импорт (fail-closed): transport-строка расшифровывается, подпись envelope
// проверяется ключами ИЗ бандла до любой записи, антидаунгрейд по локальной
// version.txt, отпечатки ключей показываются владельцу ДО применения.
// Применение — только по явному подтверждению (confirm=true), иначе
// возвращается превью с отпечатками (без секретов).

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
	"github.com/snowden-system/windows/backend/render"
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

// PreviewOnboardingString — расшифровать и показать, что внутри бандла,
// НЕ меняя ничего на диске. Второй шаг подтверждения — ApplyOnboardingString.
func (a *App) PreviewOnboardingString(transport, passphrase string) (*OnboardingPreview, error) {
	payload, err := onboarding.Open(transport, passphrase)
	if err != nil {
		return nil, err
	}
	prev := &OnboardingPreview{
		CreatedAt:  payload.CreatedAt.Format(time.RFC3339),
		DeviceName: payload.DeviceName,
		Secrets:    len(payload.Secrets),
	}
	for _, k := range payload.TrustedKeys {
		prev.Keys = append(prev.Keys, OnboardingKeyPreview{
			KeyID:       k.KeyID,
			Fingerprint: onboarding.KeyFingerprint(k.Public),
			Comment:     k.Comment,
		})
	}
	// Структуру envelope валидируем (без записи): сколько каналов, версия.
	var env metadata.Envelope
	if err := json.Unmarshal(payload.Envelope, &env); err == nil {
		prev.Channels = len(env.Channels)
		prev.SplitDirect = len(env.SplitDirect)
		prev.BundleVer = env.MetadataVersion
	}
	prev.CurrentVer = render.CurrentEnvelopeVersion()
	prev.WouldDowngrade = prev.BundleVer > 0 && prev.BundleVer < prev.CurrentVer
	return prev, nil
}

// ApplyOnboardingString — применить бандл: ключи + envelope + секреты.
// confirm=false — только превью (безопасный вызов из UI).
func (a *App) ApplyOnboardingString(transport, passphrase string, confirm bool) (*OnboardingPreview, error) {
	if !confirm {
		return a.PreviewOnboardingString(transport, passphrase)
	}
	payload, err := onboarding.Open(transport, passphrase)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil, fmt.Errorf("onboarding: store dir: %w", err)
	}
	store := &metadata.Store{Dir: dir}

	// Антидаунгрейд (V2-034): бандл старше принятой версии — отказ.
	current := render.CurrentEnvelopeVersion()
	var env metadata.Envelope
	if err := json.Unmarshal(payload.Envelope, &env); err != nil {
		return nil, fmt.Errorf("onboarding: envelope не разбирается: %w", err)
	}
	if env.MetadataVersion > 0 && env.MetadataVersion < current {
		return nil, fmt.Errorf("onboarding: версия бандла %d ниже принятой %d (антидаунгрейд) — импорт отвергнут", env.MetadataVersion, current)
	}

	// 1. Ключи: merge с существующими (ничего не теряем), после проверки hex.
	newKeys := map[string]ed25519.PublicKey{}
	for _, k := range payload.TrustedKeys {
		raw, err := hex.DecodeString(k.Public)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("onboarding: ключ %s: некорректный открытый ключ", k.KeyID)
		}
		newKeys[k.KeyID] = ed25519.PublicKey(raw)
	}
	existing, present, err := store.LoadKeys()
	if err != nil {
		return nil, fmt.Errorf("onboarding: существующие ключи: %w", err)
	}
	if !present {
		existing = map[string]ed25519.PublicKey{}
	}
	merged := map[string]ed25519.PublicKey{}
	for id, pub := range existing {
		merged[id] = pub
	}
	for id, pub := range newKeys {
		merged[id] = pub
	}
	if err := store.SaveKeys(merged); err != nil {
		return nil, fmt.Errorf("onboarding: сохранить ключи: %w", err)
	}

	// 2. Envelope: подпись проверяется ключами (уже включая импортированные)
	// и семантика — до записи. Пишем ТОЛЬКО после успешной проверки.
	keys, _, err := store.LoadKeys()
	if err != nil {
		return nil, fmt.Errorf("onboarding: ключи после merge: %w", err)
	}
	if len(keys) == 0 {
		return nil, errors.New("onboarding: доверенных ключей нет (fail-closed)")
	}
	var signed metadata.Envelope
	dec := json.NewDecoder(strings.NewReader(string(payload.Envelope)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&signed); err != nil {
		return nil, fmt.Errorf("onboarding: envelope strict: %w", err)
	}
	if err := signed.Verify(keys); err != nil {
		return nil, fmt.Errorf("onboarding: подпись envelope НЕ подтвердилась: %w", err)
	}
	if err := signed.Validate(time.Now(), current); err != nil {
		return nil, fmt.Errorf("onboarding: envelope не валиден: %w", err)
	}
	if err := store.SaveEnvelope(&signed); err != nil {
		return nil, fmt.Errorf("onboarding: сохранить envelope: %w", err)
	}
	if err := writeLocalEnvelopeVersion(dir, signed.MetadataVersion); err != nil {
		return nil, fmt.Errorf("onboarding: сохранить version: %w", err)
	}

	// 3. Секреты: идемпотентно (существующий id — обновление значения).
	imported := 0
	for _, s := range payload.Secrets {
		if err := a.vault.Save(s.ID, s.Value); err != nil {
			// Слота может не быть на новом устройстве — создаём.
			if _, aerr := a.vault.AddSave(string(secretvault.Kind(s.Kind)), s.ID, s.Value, "импорт с другого устройства"); aerr != nil {
				return nil, fmt.Errorf("onboarding: секрет %s (%s): %w", s.ID, s.Kind, err)
			}
		}
		imported++
	}

	a.appendLogLocked("info", fmt.Sprintf("onboarding: импорт применён (каналов: %d, секретов: %d, версия envelope: %d)", len(signed.Channels), imported, signed.MetadataVersion))
	prev := &OnboardingPreview{
		CreatedAt:  payload.CreatedAt.Format(time.RFC3339),
		Channels:   len(signed.Channels),
		Secrets:    imported,
		BundleVer:  signed.MetadataVersion,
		CurrentVer: signed.MetadataVersion,
	}
	return prev, nil
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

// writeLocalEnvelopeVersion — обновить локальный антидаунгрейд-маркер
// (тот же version.txt, что пишет render после успешного Apply).
func writeLocalEnvelopeVersion(dir string, v uint64) error {
	if v == 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, "version.txt"), []byte(fmt.Sprintf("%d", v)), 0o600)
}
