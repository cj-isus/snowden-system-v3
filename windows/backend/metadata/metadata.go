// Package metadata — FR-008 (B2): подписанный envelope доставки профилей
// каналов поверх встроенных дескрипторов render.
//
// Контракт (docs/PLAN.md §3 FR-008, решение V2-033):
//   - Ed25519-подпись над canonical-JSON envelope (signature-поле пустое);
//   - strict decode: unknown fields и trailing data — reject (как config.Parse);
//   - fail-closed валидация: схема, временной интервал (±5min clock skew),
//     expiry, антидаунгрейд (version >= minimumVersion), unknown key_id,
//     ревокации, secret-shaped ссылки;
//   - доверие: локальный файл trusted_keys.json (%AppData%), key_id = первые
//     16 hex SHA256 публичного ключа; нет файла/ключей — override не
//     применяется, работают встроенные дескрипторы (не ошибка);
//   - секреты в envelope — только ссылки; значения секретов сюда не попадают
//     никогда (AGENTS §2.4).
package metadata

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SchemaVersion — единственная поддерживаемая схема envelope.
const SchemaVersion = 1

// MaxClockSkew — допуск на расхождение часов при проверке issued_at/expires_at.
const MaxClockSkew = 5 * time.Minute

// Envelope — подписанный набор дескрипторов каналов.
type Envelope struct {
	SchemaVersion   int       `json:"schema_version"`
	MetadataVersion uint64    `json:"metadata_version"`
	IssuedAt        time.Time `json:"issued_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	KeyID           string    `json:"key_id"`
	Signature       string    `json:"signature,omitempty"`
	Channels        []Channel `json:"channels"`
	Revocations     []string  `json:"revocations,omitempty"`
	// V2-048/F10: процессы, идущие мимо туннеля (нижний регистр, без пути).
	// Полный список заменяет предыдущий (не merge): envelope — единый источник
	// правды метаданных. Доставed owner'ом через тот же подписанный конверт.
	SplitDirect []string `json:"split_direct,omitempty"`
}

// Channel — дескриптор канала во встроенном формате render (schema 1/2:
// tcp-reality несёт uuid_ref/reality_*_ref). Только ссылки на секреты.
type Channel struct {
	ID                  string   `json:"id"`
	Protocol            string   `json:"protocol"`
	Transport           string   `json:"transport"`
	Hostname            string   `json:"hostname"`
	Port                uint16   `json:"port"`
	OriginServer        string   `json:"origin_server"`
	ExpectedEgress      string   `json:"expected_egress"`
	CredentialRefs      []string `json:"credential_refs"`
	ValidationStatus    string   `json:"validation_status"`
	ValidatedAt         *string  `json:"validated_at,omitempty"`
	Enabled             bool     `json:"enabled"`
	UUIDRef             string   `json:"uuid_ref,omitempty"`
	RealityPublicKeyRef string   `json:"reality_public_key_ref,omitempty"`
	RealityShortIDRef   string   `json:"reality_short_id_ref,omitempty"`
}

// Validate — fail-closed проверка envelope БЕЗ подписи (структура и семантика).
// minimumVersion — последний подтверждённо принятый version (антидаунгрейд).
func (e *Envelope) Validate(now time.Time, minimumVersion uint64) error {
	if e.SchemaVersion != SchemaVersion {
		return fmt.Errorf("metadata: unsupported schema version %d", e.SchemaVersion)
	}
	if e.MetadataVersion < minimumVersion {
		return fmt.Errorf("metadata: version downgrade (%d < %d)", e.MetadataVersion, minimumVersion)
	}
	if e.IssuedAt.IsZero() || e.ExpiresAt.IsZero() || !e.ExpiresAt.After(e.IssuedAt) {
		return errors.New("metadata: invalid validity interval")
	}
	if e.IssuedAt.After(now.Add(MaxClockSkew)) {
		return errors.New("metadata: issued_at too far in the future")
	}
	if !e.ExpiresAt.After(now.Add(-MaxClockSkew)) {
		return errors.New("metadata: expired")
	}
	if len(e.Channels) == 0 {
		return errors.New("metadata: empty channels")
	}
	// V2-048/F10: имена процессов — нижний регистр, без пути/аргументов.
	// Пустой список валиден (нет сплита); «.exe» голым не бывает — имя файла.
	for i, p := range e.SplitDirect {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || strings.ContainsAny(p, `/\`) || strings.Contains(p, " ") || filepath.Base(p) != p {
			return fmt.Errorf("metadata: split_direct[%d]: invalid process name %q", i, e.SplitDirect[i])
		}
		e.SplitDirect[i] = p
	}
	seen := map[string]bool{}
	revoked := map[string]bool{}
	for _, id := range e.Revocations {
		revoked[id] = true
	}
	for i, c := range e.Channels {
		if c.ID == "" {
			return fmt.Errorf("metadata: channels[%d]: empty id", i)
		}
		if seen[c.ID] {
			return fmt.Errorf("metadata: channels[%d]: duplicate id %s", i, c.ID)
		}
		seen[c.ID] = true
		// Ревокация здесь НЕ фатальна (в отличие от старых contracts): ревокация
		// — штатный механизм отзыва канала; envelope может одновременно нести
		// и revoked-канал, и его замену. Право интерпретации — у render
		// (applyEnvelopeOverride удаляет revoked из набора).
		if len(c.CredentialRefs) == 0 {
			return fmt.Errorf("metadata: channel %s: no credential refs", c.ID)
		}
		for _, ref := range c.CredentialRefs {
			if strings.ContainsAny(ref, "\r\n") || secretShaped(ref) {
				return fmt.Errorf("metadata: channel %s: invalid credential ref %q", c.ID, ref)
			}
		}
	}
	return nil
}

// Verify — проверка подписи по таблице доверенных ключей (key_id → ключ).
func (e *Envelope) Verify(keys map[string]ed25519.PublicKey) error {
	if len(keys) == 0 {
		return errors.New("metadata: no trusted keys")
	}
	key, ok := keys[e.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("metadata: unknown key id %q", e.KeyID)
	}
	sig, err := base64.RawStdEncoding.DecodeString(e.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("metadata: invalid signature encoding")
	}
	if !ed25519.Verify(key, e.Canonical(), sig) {
		return errors.New("metadata: signature mismatch")
	}
	return nil
}

// Canonical — байты, подписываемые/проверяемые: JSON без signature-поля
// (omitempty; encoding/json детерминирован для структур — подпись и проверка
// используют один и тот же код, tamper-тест фиксирует контракт).
func (e *Envelope) Canonical() []byte {
	cp := *e
	cp.Signature = ""
	b, _ := json.Marshal(&cp)
	return b
}

// Sign — заполнить Signature закрытым ключом (canonical-байты).
func (e *Envelope) Sign(key ed25519.PrivateKey) error {
	if len(key) != ed25519.PrivateKeySize {
		return errors.New("metadata: invalid private key size")
	}
	e.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(key, e.Canonical()))
	return nil
}

// DecodeStrict — строгий разбор файла: unknown fields/trailing data запрещены.
func DecodeStrict(data []byte) (*Envelope, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var e Envelope
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("metadata: decode: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("metadata: trailing data after envelope")
		}
		return nil, fmt.Errorf("metadata: decode: %w", err)
	}
	return &e, nil
}

// KeyIDFor — стабильный идентификатор ключа: первые 16 hex SHA256 ключа.
func KeyIDFor(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

// ---------- trusted keys store (%AppData%\snowden-system\metadata) ----------

// Store — каталог доверенных ключей и подписанных envelope'ов.
type Store struct {
	Dir string
}

// DefaultStoreDir — %AppData%\snowden-system\metadata.
func DefaultStoreDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return "", fmt.Errorf("metadata: user config dir: %w", err)
	}
	return filepath.Join(base, "snowden-system", "metadata"), nil
}

const (
	keysFileName     = "trusted_keys.json"
	envelopeFileName = "channels.json"
)

// KeysPath / EnvelopePath — канонические пути файлов.
func (s *Store) KeysPath() string { return filepath.Join(s.Dir, keysFileName) }
func (s *Store) EnvelopePath() string {
	return filepath.Join(s.Dir, envelopeFileName)
}

// trustedKeysFile — формат файла ключей. Приватных ключей здесь нет и быть
// не может: подпись выполняется офлайн (metadatatool).
type trustedKeysFile struct {
	Schema int          `json:"schema"`
	Keys   []trustedKey `json:"keys"`
}

type trustedKey struct {
	KeyID   string `json:"key_id"`
	Public  string `json:"public_hex"`
	Comment string `json:"comment,omitempty"`
}

// LoadKeys — чтение и разбор доверенных ключей. Отсутствие файла — не ошибка:
// возвращается пустая таблица и present=false (override просто не применяется).
func (s *Store) LoadKeys() (keys map[string]ed25519.PublicKey, present bool, err error) {
	data, err := os.ReadFile(s.KeysPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("metadata: read keys: %w", err)
	}
	var f trustedKeysFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, true, fmt.Errorf("metadata: parse keys: %w", err)
	}
	if f.Schema != 1 {
		return nil, true, fmt.Errorf("metadata: keys file schema %d", f.Schema)
	}
	keys = make(map[string]ed25519.PublicKey, len(f.Keys))
	for _, k := range f.Keys {
		raw, err := hex.DecodeString(k.Public)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, true, fmt.Errorf("metadata: key %s: invalid public key hex", k.KeyID)
		}
		id := KeyIDFor(ed25519.PublicKey(raw))
		if k.KeyID != "" && k.KeyID != id {
			return nil, true, fmt.Errorf("metadata: key %s: key_id mismatch (expected %s)", k.KeyID, id)
		}
		keys[id] = ed25519.PublicKey(raw)
	}
	return keys, true, nil
}

// SaveKeys — атомарная запись таблицы ключей (temp + rename), права 0600.
func (s *Store) SaveKeys(keys map[string]ed25519.PublicKey) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("metadata: mkdir: %w", err)
	}
	f := trustedKeysFile{Schema: 1}
	for id, pub := range keys {
		if len(pub) != ed25519.PublicKeySize {
			return fmt.Errorf("metadata: key %s: bad size", id)
		}
		f.Keys = append(f.Keys, trustedKey{KeyID: id, Public: hex.EncodeToString(pub)})
	}
	return atomicWrite(s.KeysPath(), f)
}

// LoadEnvelope — чтение подписанного envelope с диска. Отсутствие файла —
// не ошибка (present=false): встроенные дескрипторы продолжают действовать.
func (s *Store) LoadEnvelope() (*Envelope, bool, error) {
	data, err := os.ReadFile(s.EnvelopePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("metadata: read envelope: %w", err)
	}
	e, err := DecodeStrict(data)
	if err != nil {
		return nil, true, err
	}
	return e, true, nil
}

// SaveEnvelope — атомарная запись подписанного envelope (temp + rename).
func (s *Store) SaveEnvelope(e *Envelope) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("metadata: mkdir: %w", err)
	}
	return atomicWrite(s.EnvelopePath(), e)
}

// Apply — полный путь использования: load → verify → validate.
// minimumVersion передаёт вызывающий (хранит последний принятый version).
func (s *Store) Apply(now time.Time, minimumVersion uint64) (*Envelope, error) {
	e, present, err := s.LoadEnvelope()
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	keys, keysPresent, err := s.LoadKeys()
	if err != nil {
		return nil, fmt.Errorf("metadata: envelope отвергнут (таблица ключей повреждена): %w", err)
	}
	if !keysPresent {
		return nil, errors.New("metadata: envelope есть, доверенных ключей нет (fail-closed)")
	}
	if err := e.Verify(keys); err != nil {
		return nil, err
	}
	if err := e.Validate(now, minimumVersion); err != nil {
		return nil, err
	}
	return e, nil
}

// ---------- helpers ----------

// secretShaped — защита от подсовывания значений вместо ссылок (контракт
// старых contracts); ссылки вида «vless-uuid-b» этих маркеров не содержат.
func secretShaped(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"secret", "token", "password", "passwd", "private_key", "pem"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func atomicWrite(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("metadata: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("metadata: write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("metadata: rename: %w", err)
	}
	return nil
}
