// Package update — подписанные обновления приложения (V2-050/F15).
//
// Контракт: манифест подписывается теми же Ed25519-ключами, что и envelope
// каналов (одна таблица доверия trusted_keys.json — «существующая цепочка»),
// проверка — по canonical-JSON без поля signature (тот же приём, что
// metadata.Envelope). Антидаунгрейд: этаж принятых версий в
// AppData/snowden-system/update/update_floor.txt — версия ниже этажа
// отвергается даже с валидной подписью.
//
// Формат манифеста (update.json рядом с snowden-system.exe):
//
//	{
//	  "schema": 1,
//	  "version": "2.1.0",
//	  "released_at": "2026-09-14T12:00:00Z",
//	  "notes": "...",
//	  "file": "snowden-system.exe",
//	  "sha256": "<hex>",
//	 , "size": 35645440,
//	  "key_id": "...", "signature": "..."
//	}
//
// Схема применения (App-слой, updateapp.go):
//
//	manifest+payload → verify sig → verify sha256 → version > running
//	  и version > floor → атомарная подмена → relaunch → floor поднимается
//	  уже НОВЫМ процессом при старте (не старым: если новая версия не смогла
//	  стартовать и её убили, этаж остаётся честным).
package update

import (
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

	"github.com/snowden-system/windows/backend/metadata"
)

const Schema = 1

// Manifest — подписываемый документ. Signature — Ed25519 над Canonical().
type Manifest struct {
	Schema     int       `json:"schema"`
	Version    string    `json:"version"`
	ReleasedAt time.Time `json:"released_at"`
	Notes      string    `json:"notes,omitempty"`
	File       string    `json:"file"`   // имя payload-файла рядом с манифестом
	SHA256     string    `json:"sha256"` // hex
	Size       int64     `json:"size"`

	// Издатель (заполняется инструментом подписи).
	KeyID     string `json:"key_id"`
	Signature string `json:"signature,omitempty"`
}

// Canonical — байты под подписью: JSON без поля signature (тот же контракт,
// что metadata.Envelope.Canonical: подпись и проверка используют один код).
func (m *Manifest) Canonical() []byte {
	cp := *m
	cp.Signature = ""
	b, err := json.Marshal(&cp)
	if err != nil {
		// Marshal структуры со скалярами не падает; паника честнее тихого
		// разъезда подписи и проверки.
		panic(fmt.Sprintf("update: canonical marshal: %v", err))
	}
	return b
}

// Sign — подписать манифест приватным ключом (используется CLI-инструментом).
func (m *Manifest) Sign(priv ed25519.PrivateKey) error {
	if len(priv) != ed25519.PrivateKeySize {
		return errors.New("update: private key size")
	}
	m.KeyID = metadata.KeyIDFor(priv.Public().(ed25519.PublicKey))
	sig := ed25519.Sign(priv, m.Canonical())
	m.Signature = base64.RawStdEncoding.EncodeToString(sig)
	return nil
}

// Verify — проверка подписи против таблицы доверенных ключей (та же таблица,
// что для envelope каналов).
func (m *Manifest) Verify(keys map[string]ed25519.PublicKey) error {
	if len(keys) == 0 {
		return errors.New("update: no trusted keys")
	}
	key, ok := keys[m.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("update: unknown key id %q", m.KeyID)
	}
	sig, err := base64.RawStdEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("update: invalid signature encoding")
	}
	if !ed25519.Verify(key, m.Canonical(), sig) {
		return errors.New("update: signature mismatch")
	}
	return nil
}

// Validate — структурные инварианты, не зависящие от подписи.
func (m *Manifest) Validate() error {
	if m.Schema != Schema {
		return fmt.Errorf("update: unsupported schema %d", m.Schema)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return fmt.Errorf("update: version: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(m.File), ".exe") {
		return fmt.Errorf("update: payload file %q не .exe", m.File)
	}
	if len(m.SHA256) != 64 {
		return errors.New("update: sha256 должен быть hex длиной 64")
	}
	if _, err := hex.DecodeString(m.SHA256); err != nil {
		return fmt.Errorf("update: sha256: %w", err)
	}
	if m.Size <= 0 {
		return errors.New("update: size must be > 0")
	}
	if m.ReleasedAt.IsZero() {
		return errors.New("update: released_at zero")
	}
	return nil
}

// VerifyPayload — SHA-256 payload-файла совпадает с манифестом.
func (m *Manifest) VerifyPayload(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("update: open payload: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("update: hash: %w", err)
	}
	if n != m.Size {
		return fmt.Errorf("update: размер файла %d ≠ манифесту %d", n, m.Size)
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), m.SHA256) {
		return errors.New("update: sha256 mismatch")
	}
	return nil
}

// ---------- версии (semver-lite: major.minor.patch[.build]) ----------

// Version — разобранная версия. Числа, не строки: сравнение машинное.
type Version struct{ Major, Minor, Patch, Build int }

// ParseVersion — "2.1.0" / "2.1.0.7". Суффиксы (-rc1, +meta) не поддержаны:
// издатель нумерует релизы строго числами (fail-closed на незнакомом формате).
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, errors.New("пустая строка")
	}
	if strings.ContainsAny(s, "-+") {
		return Version{}, fmt.Errorf("суффиксы не поддержаны: %q", s)
	}
	parts := strings.Split(s, ".")
	if len(parts) < 3 || len(parts) > 4 {
		return Version{}, fmt.Errorf("ожидается major.minor.patch[.build]: %q", s)
	}
	var v Version
	for i, p := range parts {
		n := 0
		if p == "" {
			return Version{}, fmt.Errorf("пустая компонента в %q", s)
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return Version{}, fmt.Errorf("нечисловая компонента в %q", s)
			}
			n = n*10 + int(c-'0')
			if n > 1_000_000 {
				return Version{}, fmt.Errorf("компонента слишком велика в %q", s)
			}
		}
		switch i {
		case 0:
			v.Major = n
		case 1:
			v.Minor = n
		case 2:
			v.Patch = n
		case 3:
			v.Build = n
		}
	}
	return v, nil
}

// Compare — -1/0/+1.
func (v Version) Compare(o Version) int {
	switch {
	case v.Major != o.Major:
		return cmpInt(v.Major, o.Major)
	case v.Minor != o.Minor:
		return cmpInt(v.Minor, o.Minor)
	case v.Patch != o.Patch:
		return cmpInt(v.Patch, o.Patch)
	default:
		return cmpInt(v.Build, o.Build)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// ---------- этаж принятых версий (антидаунгрейд) ----------

// FloorPath — файл этажа: AppData/snowden-system/update/update_floor.txt.
func FloorPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return "", fmt.Errorf("update: user config dir: %w", err)
	}
	return filepath.Join(base, "snowden-system", "update", "update_floor.txt"), nil
}

// CurrentFloor — этаж принятых версий (0 = обновления ещё не применялись).
func CurrentFloor() Version {
	p, err := FloorPath()
	if err != nil {
		return Version{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Version{}
	}
	v, err := ParseVersion(strings.TrimSpace(string(data)))
	if err != nil {
		return Version{}
	}
	return v
}

// RaiseFloor — поднять этаж (только вверх). Возвращает фактический этаж.
func RaiseFloor(v Version) (Version, error) {
	p, err := FloorPath()
	if err != nil {
		return Version{}, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return Version{}, fmt.Errorf("update: mkdir: %w", err)
	}
	cur := CurrentFloor()
	if cur.Compare(v) >= 0 {
		return cur, nil // уже не ниже — ничего не пишем
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(v.String()), 0o600); err != nil {
		return Version{}, fmt.Errorf("update: write floor: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return Version{}, fmt.Errorf("update: rename floor: %w", err)
	}
	return v, nil
}

// String — "2.1.0" (без .build, если нулевая; "0.0.0" для нулевой версии).
func (v Version) String() string {
	if v.Build == 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
}
