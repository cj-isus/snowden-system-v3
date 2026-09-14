// Package secretvault — локальное хранилище секретов Windows.
//
// Правила проекта (PLAN.md §7, AGENTS.md §2.4):
//   - значения шифруются DPAPI (пользовательский scope) и живут только в
//     gitignored-файле windows/secrets/vault.v1.json;
//   - наружу (UI/логи/диагностика) отдаются только метаданные: SHA256-префикс
//     (fingerprint), статус проверки, счётчик раскрытий;
//   - сверка client↔server — по хешам, не по значениям;
//   - запись атомарна (temp + rename), файл не читаем другими пользователями.
package secretvault

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const schemaVersion = 1

// Kind — тип секрета. Определяет правила локальной валидации формата.
type Kind string

const (
	KindVlessUUID      Kind = "vless-uuid"
	KindHy2Password    Kind = "hy2-password"
	KindHy2ObfsPasword Kind = "hy2-obfs-password"
	KindVpsSSHKey      Kind = "vps-ssh-key"
	KindCfApiToken     Kind = "cf-api-token"
	KindCustom         Kind = "custom"

	// Слоты канала C (VLESS+TCP+REALITY на том же VPS, V2-044). Имена
	// generic по роли (не по ID канала): будущие tcp-reality каналы
	// переиспользуют те же Kind'ы, а ссылку на конкретный канал задаёт
	// дескриптор (uuid_ref/reality_public_key_ref/reality_short_id_ref).
	KindVlessUUIDC      Kind = "vless-uuid-c"
	KindRealityPubKeyC  Kind = "reality-public-key-c"
	KindRealityShortIDC Kind = "reality-short-id-c"
)

// AllKinds — список допустимых типов.
func AllKinds() []Kind {
	return []Kind{KindVlessUUID, KindHy2Password, KindHy2ObfsPasword, KindVpsSSHKey, KindCfApiToken, KindCustom,
		KindVlessUUIDC, KindRealityPubKeyC, KindRealityShortIDC}
}

var (
	ErrNotFound = errors.New("secretvault: secret not found")
	ErrNotSet   = errors.New("значение ещё не задано")
)

var base64Std = base64.StdEncoding

// storedEntry — запись в файле. CipherB64 — DPAPI-шифротекст (base64);
// пусто = значение не задано. Открытый текст в файл не пишется никогда.
type storedEntry struct {
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Hint      string    `json:"hint"`
	CipherB64 string    `json:"cipher_b64,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// audit — метаданные аудита (не секреты).
type audit struct {
	Reveals        int        `json:"reveals"`
	CreatedAt      time.Time  `json:"created_at"`
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
	VerifyStatus   string     `json:"verify_status"`
	VerifyError    string     `json:"verify_error,omitempty"`
}

type fileFormat struct {
	Schema int                    `json:"schema"`
	Items  map[string]storedEntry `json:"items"`
	Order  []string               `json:"order"`
	Audit  map[string]audit       `json:"audit"`
}

// Meta — карточка секрета для UI. Значения здесь нет по построению.
type Meta struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Hint           string `json:"hint"`
	Backend        string `json:"backend"`
	StoredValue    string `json:"storedValue"` // "dpapi" | ""
	Fingerprint    string `json:"fingerprint"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	LastVerifiedAt string `json:"lastVerifiedAt"`
	VerifyStatus   string `json:"verifyStatus"`
	VerifyError    string `json:"verifyError"`
	Reveals        int    `json:"reveals"`
}

// Manager — владелец файла хранилища и всех операций.
type Manager struct {
	path string
	mu   sync.Mutex // сериализация load→mutate→save; файл читают из нескольких горутин (UI + render)
}

func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// Path — расположение файла хранилища (для подсказок в UI).
func (m *Manager) Path() string { return m.path }

// ---------- загрузка/сохранение ----------

func (m *Manager) load() (*fileFormat, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &fileFormat{
				Schema: schemaVersion,
				Items:  map[string]storedEntry{},
				Order:  []string{},
				Audit:  map[string]audit{},
			}, nil
		}
		return nil, fmt.Errorf("secretvault: read: %w", err)
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("secretvault: parse %s: %w", m.path, err)
	}
	if f.Schema != schemaVersion {
		return nil, fmt.Errorf("secretvault: unsupported schema %d in %s", f.Schema, m.path)
	}
	if f.Items == nil {
		f.Items = map[string]storedEntry{}
	}
	if f.Order == nil {
		f.Order = []string{}
	}
	if f.Audit == nil {
		f.Audit = map[string]audit{}
	}
	return &f, nil
}

// save атомарна: temp-файл рядом + rename. Права — только у владельца.
func (m *Manager) save(f *fileFormat) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("secretvault: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("secretvault: marshal: %w", err)
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("secretvault: write temp: %w", err)
	}
	if err := os.Rename(tmp, m.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("secretvault: rename: %w", err)
	}
	return nil
}

// ---------- операции ----------

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Крайне маловероятно; падаем честно, а не генерим дубликат id.
		panic("secretvault: entropy unavailable: " + err.Error())
	}
	// RFC4122 v4 биты
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Fingerprint — SHA256-префикс значения (16 hex-символов). Это публичная
// сверка client↔server: по хешу, не по значению.
func Fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

// Seed — предзаполнение типовых слотов без значений. Существующие слоты
// (по kind) не перезаписываются — правило не трогать данные пользователя.
func (m *Manager) Seed() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return err
	}
	seeds := []struct {
		kind  Kind
		title string
		hint  string
	}{
		{KindVlessUUID, "UUID VLESS — канал A", "Идентификатор пользователя VLESS на сервере. Значение знает только сервер и клиент; в чат и скриншоты не вставлять."},
		{KindHy2Password, "Пароль HY2", "Пароль аутентификации Hysteria2 (UDP-канал). Сверка с сервером — по SHA256-хешу, не по значению."},
		{KindHy2ObfsPasword, "Пароль obfs (HY2)", "Пароль обфускации salamander для HY2. Должен совпадать с серверным; иначе UDP-пакеты не распознаются."},
		{KindVpsSSHKey, "SSH-ключ VPS", "Приватный SSH-ключ доступа к VPS. Значение никогда не попадает в логи и diagnostics."},
		{KindCfApiToken, "API-токен Cloudflare", "API-токен Cloudflare. Утечка в git-историю = компрометация: токен нужно отозвать и перевыпустить."},
		{KindVlessUUIDC, "UUID VLESS — канал C (REALITY)", "Идентификатор пользователя VLESS на REALITY-инбаунде. Совпадает с серверным; в чат и скриншоты не вставлять."},
		{KindRealityPubKeyC, "Публичный ключ REALITY (канал C)", "X25519 public key REALITY (43 симв. base64url). Публичный по природе, но хранится тут для целостности набора."},
		{KindRealityShortIDC, "Short ID REALITY (канал C)", "hex до 16 символов; должен совпадать с серверным списком short_id."},
	}
	changed := false
	for _, s := range seeds {
		if findKind(f, s.kind) != "" {
			continue
		}
		id := newID()
		f.Items[id] = storedEntry{Kind: string(s.kind), Title: s.title, Hint: s.hint, UpdatedAt: time.Now()}
		f.Order = append(f.Order, id)
		f.Audit[id] = audit{CreatedAt: time.Now(), VerifyStatus: "unverified"}
		changed = true
	}
	if !changed {
		return nil
	}
	return m.save(f)
}

func findKind(f *fileFormat, k Kind) string {
	for id, it := range f.Items {
		if it.Kind == string(k) {
			return id
		}
	}
	return ""
}

// orderedIDs — порядок карточек: f.Order + элементы вне порядка (сортировка по id).
func orderedIDs(f *fileFormat) []string {
	seen := map[string]bool{}
	ordered := make([]string, 0, len(f.Items))
	for _, id := range f.Order {
		if _, ok := f.Items[id]; ok && !seen[id] {
			seen[id] = true
			ordered = append(ordered, id)
		}
	}
	extra := make([]string, 0)
	for id := range f.Items {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	return append(ordered, extra...)
}

// List — метаданные всех карточек (без значений).
func (m *Manager) List() ([]Meta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return nil, err
	}
	out := make([]Meta, 0, len(f.Items))
	for _, id := range orderedIDs(f) {
		out = append(out, m.metaOf(id, f.Items[id], f.Audit[id]))
	}
	return out, nil
}

// Save записывает значение слота с локальной валидацией формата по типу.
// Невалидное значение не сохраняется: «сначала корректный формат, потом шифрование».
func (m *Manager) Save(id, value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return errors.New("значение пустое: вставьте секрет в поле ввода")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return err
	}
	it, ok := f.Items[id]
	if !ok {
		return ErrNotFound
	}
	if err := validateKind(Kind(it.Kind), v); err != nil {
		return err
	}
	cipher, err := dpapiProtect([]byte(v))
	if err != nil {
		return err
	}
	it.CipherB64 = base64Std.EncodeToString(cipher)
	it.UpdatedAt = time.Now()
	f.Items[id] = it // storedEntry — value-тип: запись в map обязательна

	a := f.Audit[id]
	a.VerifyStatus = "unverified"
	a.VerifyError = ""
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	f.Audit[id] = a
	return m.save(f)
}

// AddSave — сохранить значение в слот по kind (используется сеялкой UI).
// Если слота нет — создаёт его (для «своих» секретов).
func (m *Manager) AddSave(kind, title, value, hint string) (Meta, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return Meta{}, errors.New("значение пустое: вставьте секрет в поле ввода")
	}
	if title == "" {
		return Meta{}, errors.New("укажите название секрета")
	}
	k := Kind(kind)
	valid := false
	for _, x := range AllKinds() {
		if x == k {
			valid = true
			break
		}
	}
	if !valid {
		return Meta{}, fmt.Errorf("неизвестный тип секрета: %s", kind)
	}
	if err := validateKind(k, v); err != nil {
		return Meta{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return Meta{}, err
	}
	// Для типовых kinds — повторно используем существующий слот.
	id := findKind(f, k)
	if id == "" {
		id = newID()
		f.Items[id] = storedEntry{Kind: kind}
		f.Order = append(f.Order, id)
		f.Audit[id] = audit{CreatedAt: time.Now(), VerifyStatus: "unverified"}
	}
	it := f.Items[id]
	it.Title = title
	it.Hint = hint
	it.UpdatedAt = time.Now()
	cipher, err := dpapiProtect([]byte(v))
	if err != nil {
		return Meta{}, err
	}
	it.CipherB64 = base64Std.EncodeToString(cipher)
	f.Items[id] = it

	a := f.Audit[id]
	a.VerifyStatus = "unverified"
	a.VerifyError = ""
	f.Audit[id] = a

	if err := m.save(f); err != nil {
		return Meta{}, err
	}
	return m.metaOf(id, it, a), nil
}

// Remove удаляет слот целиком.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return err
	}
	if _, ok := f.Items[id]; !ok {
		return ErrNotFound
	}
	delete(f.Items, id)
	delete(f.Audit, id)
	for i, x := range f.Order {
		if x == id {
			f.Order = append(f.Order[:i], f.Order[i+1:]...)
			break
		}
	}
	return m.save(f)
}

// Verify — локальная проверка формата по типу (без сети). «ok» — факт
// корректного формата, не доказательство, что сервер примет значение.
func (m *Manager) Verify(id string) (Meta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return Meta{}, err
	}
	it, ok := f.Items[id]
	if !ok {
		return Meta{}, ErrNotFound
	}
	a := f.Audit[id]
	now := time.Now()

	setStatus := func(status, errMsg string) error {
		a.VerifyStatus = status
		a.VerifyError = errMsg
		a.LastVerifiedAt = &now
		f.Audit[id] = a
		return m.save(f)
	}

	if it.CipherB64 == "" {
		if err := setStatus("failed", ErrNotSet.Error()); err != nil {
			return Meta{}, err
		}
		return m.metaOf(id, it, a), ErrNotSet
	}
	plain, err := m.decryptEntry(it)
	if err != nil {
		_ = setStatus("failed", err.Error())
		return Meta{}, err
	}
	if err := validateKind(Kind(it.Kind), string(plain)); err != nil {
		_ = setStatus("failed", err.Error())
		return m.metaOf(id, it, a), err
	}
	if err := setStatus("ok", ""); err != nil {
		return Meta{}, err
	}
	return m.metaOf(id, it, a), nil
}

// Reveal отдаёт значение по явному запросу (копирование в буфер в UI) и
// увеличивает счётчик раскрытий. В логи не пишется никогда.
func (m *Manager) Reveal(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return "", err
	}
	it, ok := f.Items[id]
	if !ok {
		return "", ErrNotFound
	}
	if it.CipherB64 == "" {
		return "", ErrNotSet
	}
	plain, err := m.decryptEntry(it)
	if err != nil {
		return "", err
	}
	a := f.Audit[id]
	a.Reveals++
	f.Audit[id] = a
	if err := m.save(f); err != nil {
		return "", err
	}
	return string(plain), nil
}

// Value — внутренний доступ к значению БЕЗ счётчика раскрытий: для живых
// тестов владельца (сверка с сервером по хешу, вход по ключу). Не для UI:
// значение не должно попадать в интерфейс и логи.
func (m *Manager) Value(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.load()
	if err != nil {
		return "", err
	}
	it, ok := f.Items[id]
	if !ok {
		return "", ErrNotFound
	}
	if it.CipherB64 == "" {
		return "", ErrNotSet
	}
	plain, err := m.decryptEntry(it)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (m *Manager) decryptEntry(it storedEntry) ([]byte, error) {
	cipher, err := base64Std.DecodeString(it.CipherB64)
	if err != nil {
		return nil, fmt.Errorf("secretvault: decode: %w", err)
	}
	return dpapiUnprotect(cipher)
}

func (m *Manager) metaOf(id string, it storedEntry, a audit) Meta {
	mt := Meta{
		ID:          id,
		Kind:        it.Kind,
		Title:       it.Title,
		Hint:        it.Hint,
		Backend:     "dpapi",
		UpdatedAt:   it.UpdatedAt.Format(time.RFC3339),
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		Reveals:     a.Reveals,
		StoredValue: "",
	}
	if it.CipherB64 != "" {
		mt.StoredValue = "dpapi"
		if plain, err := m.decryptEntry(it); err == nil {
			mt.Fingerprint = Fingerprint(string(plain))
		}
	}
	if a.LastVerifiedAt != nil {
		mt.LastVerifiedAt = a.LastVerifiedAt.Format(time.RFC3339)
	}
	mt.VerifyStatus = a.VerifyStatus
	mt.VerifyError = a.VerifyError
	return mt
}

// validateKind — правила локальной проверки формата (по типу).
func validateKind(k Kind, v string) error {
	switch k {
	case KindVlessUUID, KindVlessUUIDC:
		return validateUUID(v)
	case KindRealityPubKeyC:
		// X25519 в base64.RawURLEncoding: ровно 43 символа без пэйдинга
		// (тот же гвард, что в рендере — ошибка ловится до старта движка).
		if len(v) != 43 || strings.ContainsAny(v, "+/=") {
			return fmt.Errorf("ожидался 43-символьный base64url без пэйдинга (X25519), получено %d симв.", len(v))
		}
	case KindRealityShortIDC:
		if len(v) > 16 {
			return fmt.Errorf("short_id длиннее 16 символов")
		}
		for _, r := range v {
			if !strings.ContainsRune("0123456789abcdef", r) {
				return fmt.Errorf("short_id должен быть hex в нижнем регистре")
			}
		}
	case KindHy2Password, KindHy2ObfsPasword:
		if len(v) < 8 {
			return fmt.Errorf("слишком короткий пароль (%d символов; минимум 8)", len(v))
		}
	case KindVpsSSHKey:
		if !strings.Contains(v, "PRIVATE KEY-----") {
			return errors.New("ожидается PEM приватного ключа (-----BEGIN ... PRIVATE KEY-----)")
		}
	case KindCfApiToken:
		if len(v) < 30 {
			return fmt.Errorf("слишком короткий токен (%d символов; у CF обычно 40)", len(v))
		}
	case KindCustom:
		if len(v) < 4 {
			return errors.New("слишком короткое значение (минимум 4 символа)")
		}
	default:
		return fmt.Errorf("неизвестный тип секрета: %s", k)
	}
	return nil
}

// ValidateKind — публичный доступ к правилам проверки формата (используется
// живыми тестами секретов: сначала формат, потом сеть).
func ValidateKind(k Kind, v string) error { return validateKind(k, v) }

// validateUUID — строго 8-4-4-4-12 hex.
func validateUUID(v string) error {
	const hexDigits = "0123456789abcdefABCDEF"
	if len(v) != 36 {
		return fmt.Errorf("UUID должен быть 36 символов (8-4-4-4-12), получено %d", len(v))
	}
	for i, r := range v {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return fmt.Errorf("ожидается '-' на позиции %d", i+1)
			}
		default:
			if !strings.ContainsRune(hexDigits, r) {
				return fmt.Errorf("недопустимый символ на позиции %d (ожидается hex)", i+1)
			}
		}
	}
	return nil
}
