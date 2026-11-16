// Package metadata — FR-008 (B2): подписанный envelope доставки профилей
// каналов поверх встроенных дескрипторов render.
//
// Контракт (docs/PLAN.md §3 FR-008, решение V2-033):
//   - Ed25519-подпись над canonical-JSON envelope (signature-поле пустое);
//   - strict decode: unknown fields и trailing data — reject (как config.Parse);
//   - fail-closed валидация: схема, issued_at (5min skew), строгий expiry,
//     антидаунгрейд (version >= minimumVersion), unknown key_id,
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
	"sort"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
)

// SchemaVersion is the legacy epoch0 envelope schema; schema2 carries an epoch.
const SchemaVersion = 1

// Bounds descriptor/graph/pairwise work before profile acceptance. This is a
// resource ceiling, not a measured maximum supported deployment size.
const MaxProfileChannels = 64
const MaxEnvelopeSize = 1 << 20 // legacy/public file reader; TUF target has its own tighter budget

// MaxClockSkew — допуск issued_at и local clock continuity. Expiry строгий.
const MaxClockSkew = 5 * time.Minute

// Envelope — подписанный набор дескрипторов каналов.
type Envelope struct {
	SchemaVersion   int       `json:"schema_version"`
	MetadataVersion uint64    `json:"metadata_version"`
	ProfileEpoch    uint64    `json:"profile_epoch,omitempty"`
	IssuedAt        time.Time `json:"issued_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	KeyID           string    `json:"key_id"`
	Signature       string    `json:"signature,omitempty"`
	Channels        []Channel `json:"channels"`
	Revocations     []string  `json:"revocations,omitempty"`
	// V2-048/F10: процессы, идущие мимо туннеля (нижний регистр, без пути).
	// Полный список заменяет предыдущий (не merge): envelope — единый источник
	// правды метаданных. Доставed owner'ом через тот же подписанный конверт.
	SplitDirect  []string             `json:"split_direct,omitempty"`
	Dependencies DeliveryDependencies `json:"dependencies,omitempty,omitzero"`
}

// Validate — fail-closed проверка envelope БЕЗ подписи (структура и семантика).
// minimumVersion — последний подтверждённо принятый version (антидаунгрейд).
func (e *Envelope) Validate(now time.Time, minimumVersion uint64) error {
	if e == nil || e.ProfileEpoch != 0 {
		return ErrProfileEpoch // legacy scalar API cannot validate an epoch profile
	}
	return e.ValidateVersion(now, ProfileVersion{Counter: minimumVersion})
}

func (e *Envelope) ValidateVersion(now time.Time, minimum ProfileVersion) error {
	if err := e.validateVersionShape(minimum); err != nil {
		return err
	}
	if err := e.Dependencies.Validate(); err != nil {
		return err
	}
	if e.IssuedAt.IsZero() || e.ExpiresAt.IsZero() || !e.ExpiresAt.After(e.IssuedAt) {
		return errors.New("metadata: invalid validity interval")
	}
	if e.IssuedAt.After(now.Add(MaxClockSkew)) {
		return ErrTrustNotYetValid
	}
	if !e.ExpiresAt.After(now) {
		return ErrProfileExpired
	}
	if len(e.Channels) == 0 || len(e.Channels) > MaxProfileChannels {
		return errors.New("metadata: channel count outside 1..64 bound")
	}
	// V2-048/F10: имена процессов — нижний регистр, без пути/аргументов.
	// Пустой список валиден (нет сплита); «.exe» голым не бывает — имя файла.
	for i, p := range e.SplitDirect {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || strings.ContainsAny(p, `/\`) || strings.Contains(p, " ") || filepath.Base(p) != p {
			return fmt.Errorf("metadata: split_direct[%d]: invalid process name %q", i, e.SplitDirect[i])
		}
	}
	seen := map[string]bool{}
	revoked := map[string]bool{}
	for _, id := range e.Revocations {
		if !ValidChannelID(id) {
			return errors.New("metadata: invalid revoked channel id")
		}
		revoked[id] = true
	}
	for i, c := range e.Channels {
		if !ValidChannelID(c.ID) {
			return fmt.Errorf("metadata: channels[%d]: invalid portable id", i)
		}
		if seen[c.ID] {
			return fmt.Errorf("metadata: channels[%d]: duplicate id %s", i, c.ID)
		}
		seen[c.ID] = true
		// Ревокация здесь НЕ фатальна (в отличие от старых contracts): ревокация
		// — штатный механизм отзыва канала; envelope может одновременно нести
		// и revoked-канал, и его замену. Право интерпретации — у render
		// (applyEnvelopeOverride удаляет revoked из набора).
		// Revoked records stay inert; retain the historical reference-shape gate.
		validate := ValidateChannel
		if revoked[c.ID] {
			validate = validateCredentialRefs
		}
		if err := validate(c); err != nil {
			return fmt.Errorf("metadata: channel %s: %w", c.ID, err)
		}
		// Certificate structure remains authenticated/strict. Time availability
		// is a per-channel eligibility gate; one expired identity must not poison
		// healthy alternatives in the same still-valid signed profile.
	}
	return nil
}

// Verify — проверка подписи по таблице доверенных ключей (key_id → ключ).
func (e *Envelope) Verify(keys map[string]ed25519.PublicKey) error {
	if len(keys) == 0 {
		return fmt.Errorf("%w: metadata: no trusted profile keys", ErrTrustRole)
	}
	key, ok := keys[e.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: metadata: unknown profile key id %q", ErrTrustRole, e.KeyID)
	}
	sig, err := base64.RawStdEncoding.DecodeString(e.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("%w: metadata: invalid signature encoding", ErrTrustRole)
	}
	if !ed25519.Verify(key, e.Canonical(), sig) {
		return fmt.Errorf("%w: metadata: signature mismatch", ErrTrustRole)
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
	if len(data) == 0 || len(data) > MaxEnvelopeSize {
		return nil, errors.New("metadata: envelope outside size bound")
	}
	if err := checkCheckpointJSON(data); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return nil, errors.New("metadata: envelope object required")
	}
	for name, raw := range fields {
		switch name {
		case "schema_version", "metadata_version", "profile_epoch", "issued_at", "expires_at", "key_id", "signature", "channels", "revocations", "split_direct", "dependencies":
		default:
			return nil, errors.New("metadata: unknown/aliased envelope property")
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, errors.New("metadata: envelope property cannot be null")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var e Envelope
	if err := dec.Decode(&e); err != nil {
		return nil, errors.New("metadata: malformed envelope property")
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
func (s *Store) KeysPath() string      { return filepath.Join(s.Dir, keysFileName) }
func (s *Store) ImportJournal() string { return filepath.Join(s.Dir, "import.pending") }
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
	return s.LoadRoleKeys(RoleProfile)
}

// SaveKeys publishes encoded legacy public keys through the bounded original
// writer. The caller owns journal serialization; it cannot flatten schema2.
// Windows permissions follow the checked inherited ACL, not POSIX mode bits.
func (s *Store) SaveKeys(keys map[string]ed25519.PublicKey) error {
	return filetxn.WithWriter(s.ImportJournal(), func() error { return s.saveKeys(keys) })
}

func (s *Store) saveKeys(keys map[string]ed25519.PublicKey) error {
	if err := filetxn.Check(s.ImportJournal()); err != nil {
		return err
	}
	if policy, present, err := s.LoadTrustPolicy(); err != nil {
		return err
	} else if present && policy.Schema == 2 {
		return errors.New("metadata: legacy SaveKeys cannot flatten a scoped trust policy")
	}
	data, err := EncodeKeys(keys)
	if err != nil {
		return err
	}
	return storedPolicyFile.write(s.KeysPath(), data)
}

// EncodeKeys prepares the same representation as SaveKeys without disk writes.
func EncodeKeys(keys map[string]ed25519.PublicKey) ([]byte, error) {
	f := trustedKeysFile{Schema: 1, Keys: []trustedKey{}}
	for id, pub := range keys {
		if len(pub) != ed25519.PublicKeySize || id != KeyIDFor(pub) {
			return nil, fmt.Errorf("metadata: key %s: bad size", id)
		}
		f.Keys = append(f.Keys, trustedKey{KeyID: id, Public: hex.EncodeToString(pub)})
	}
	sort.Slice(f.Keys, func(i, j int) bool { return f.Keys[i].KeyID < f.Keys[j].KeyID })
	return json.MarshalIndent(f, "", "  ")
}

// ReadEnvelopeBytes returns the exact bounded public file after checked Close.
// It shares LoadEnvelope's paired reader/writer uncertainty gate. The caller
// supplies a coherent native scope when combining it with other source files;
// this method does not acquire that scope or authenticate these bytes.
func (s *Store) ReadEnvelopeBytes() ([]byte, bool, error) {
	if err := filetxn.Check(s.ImportJournal()); err != nil {
		return nil, false, errors.Join(ErrTrustRole, err)
	}
	data, present, err := storedEnvelopeFile.read(s.EnvelopePath())
	if err != nil {
		return nil, present, fmt.Errorf("%w: read envelope: %w", ErrTrustRole, err)
	}
	return data, present, nil
}

// LoadEnvelope reads the public signed envelope. Confirmed absence returns
// present=false; the runtime/acceptance caller decides whether retained state
// permits bootstrap. Journal, storage and decode failures remain trust faults.
func (s *Store) LoadEnvelope() (result *Envelope, present bool, finalErr error) {
	defer func() {
		if finalErr != nil && !errors.Is(finalErr, ErrTrustRole) {
			// Local journal/storage/decode failure is not a channel observation.
			finalErr = errors.Join(ErrTrustRole, finalErr)
		}
	}()
	data, present, err := s.ReadEnvelopeBytes()
	if err != nil {
		return nil, present, err
	}
	if !present {
		return nil, false, nil
	}
	e, err := DecodeStrict(data)
	if err != nil {
		return nil, true, err
	}
	return e, true, nil
}

// SaveEnvelope publishes with the original bounded writer. Import retains its
// existing four-file journal; this helper grants no signature/import authority.
// The caller owns local journal/source serialization. This public entry acquires
// the native writer scope and cannot be nested under another WithWriter call.
func (s *Store) SaveEnvelope(e *Envelope) error {
	return filetxn.WithWriter(s.ImportJournal(), func() error { return s.saveEnvelope(e) })
}

func (s *Store) saveEnvelope(e *Envelope) error {
	if err := filetxn.Check(s.ImportJournal()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("metadata: marshal: %w", err)
	}
	return storedEnvelopeFile.write(s.EnvelopePath(), data)
}

// Apply — полный путь использования: load → verify → validate.
// minimumVersion передаёт вызывающий (хранит последний принятый version).
func (s *Store) Apply(now time.Time, minimumVersion uint64) (*Envelope, error) {
	return s.ApplyVersion(now, ProfileVersion{Counter: minimumVersion})
}

func (s *Store) ApplyVersion(now time.Time, minimum ProfileVersion) (*Envelope, error) {
	if _, _, err := s.LoadTrustPolicy(); err != nil {
		return nil, err // including an omitted enrolled root/checkpoint
	}
	e, present, err := s.LoadEnvelope()
	if err != nil {
		return nil, err
	}
	if !present {
		state, err := LoadAcceptedProfile(s.Dir)
		if err != nil {
			return nil, err
		}
		if state.ProfileVersion() != (ProfileVersion{}) || minimum != (ProfileVersion{}) {
			return nil, fmt.Errorf("%w: metadata: accepted envelope missing; embedded fallback prohibited", ErrTrustRole)
		}
		return nil, nil
	}
	keys, keysPresent, err := s.LoadKeys()
	if err != nil {
		return nil, fmt.Errorf("metadata: envelope отвергнут (таблица ключей повреждена): %w", err)
	}
	if !keysPresent {
		return nil, fmt.Errorf("%w: metadata: envelope есть, доверенных ключей нет (fail-closed)", ErrTrustRole)
	}
	if err := e.Verify(keys); err != nil {
		return nil, err
	}
	if err := e.ValidateVersion(now, minimum); err != nil {
		return nil, err
	}
	if _, err := s.PrepareAcceptance(e, keys, now); err != nil {
		return nil, err
	}
	return e, nil
}

// ---------- helpers ----------

// secretShaped — защита от подсовывания значений вместо ссылок (контракт
// старых contracts); ссылки вида «vless-uuid-b» этих маркеров не содержат.
func secretShaped(value string) bool {
	// Established credential roles, never credential values.
	switch value {
	case "hy2-password", "hy2-obfs-password", "shadowtls-password-d":
		return false
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"secret", "token", "password", "passwd", "private_key", "pem"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
