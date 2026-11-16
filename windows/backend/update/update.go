// Package update — подписанные обновления приложения (V2-050/F15).
//
// Контракт: production caller выбирает только явно назначенные update-ключи
// из scoped trusted_keys.json; profile/recovery ключи EXE не разрешают.
// Проверка — по canonical-JSON без поля signature (тот же приём, что
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
const PairSchema = 2
const MaxPayloadSize int64 = 1 << 30

// Manifest — подписываемый документ. Signature — Ed25519 над Canonical().
type Manifest struct {
	Schema     int       `json:"schema"`
	Version    string    `json:"version"`
	ReleasedAt time.Time `json:"released_at"`
	Notes      string    `json:"notes,omitempty"`
	File       string    `json:"file"`   // имя payload-файла рядом с манифестом
	SHA256     string    `json:"sha256"` // hex
	Size       int64     `json:"size"`
	// Omitted in schema1 to retain its exact historical canonical byte order.
	Target             string        `json:"target,omitempty"`
	ActivationProtocol int           `json:"activation_protocol,omitempty"`
	NativeHelper       *NativeHelper `json:"native_helper,omitempty"`

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

// Verify checks the signature against keys already scoped to the update role
// by the caller. This primitive alone does not decide role/enrollment policy.
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
	if m.Schema != Schema && m.Schema != PairSchema {
		return fmt.Errorf("update: unsupported schema %d", m.Schema)
	}
	if _, err := ParseVersion(m.Version); err != nil {
		return fmt.Errorf("update: version: %w", err)
	}
	if len(m.File) > 128 || m.File == "" || filepath.Base(m.File) != m.File || strings.ContainsAny(m.File, `/\:<>"|?*`) || !strings.HasSuffix(strings.ToLower(m.File), ".exe") {
		return fmt.Errorf("update: payload file %q не .exe", m.File)
	}
	for _, ch := range m.File {
		if ch < 0x20 || ch == 0x7f {
			return errors.New("update: control character in payload basename")
		}
	}
	if len(m.SHA256) != 64 {
		return errors.New("update: sha256 должен быть hex длиной 64")
	}
	if _, err := hex.DecodeString(m.SHA256); err != nil {
		return fmt.Errorf("update: sha256: %w", err)
	}
	if m.Size <= 0 || m.Size > MaxPayloadSize {
		return errors.New("update: size must be positive and at most 1GiB")
	}
	if m.ReleasedAt.IsZero() {
		return errors.New("update: released_at zero")
	}
	return m.validatePair()
}

// VerifyPayload — SHA-256 payload-файла совпадает с манифестом.
func (m *Manifest) VerifyPayload(path string) error {
	sum, err := payloadDigest(path, m.Size)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, m.SHA256) {
		return errors.New("update: sha256 mismatch")
	}
	return nil
}

func payloadDigest(path string, size int64) (string, error) {
	return payloadDigestTo(path, size, io.Discard)
}

func payloadDigestTo(path string, size int64, dst io.Writer) (digest string, finalErr error) {
	if dst == nil {
		return "", errors.New("update: artifact writer required")
	}
	if size <= 0 || size > MaxPayloadSize {
		return "", errors.New("update: payload size outside bound")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return "", errors.New("update: payload must be a regular file of declared size")
	}
	lease, err := openPairMetadata(path, false)
	if err != nil {
		return "", fmt.Errorf("update: open payload: %w", err)
	}
	defer func() {
		finalErr = errors.Join(finalErr, lease.Close())
		if finalErr != nil {
			digest = ""
		}
	}()
	f := lease.owner.file
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != size {
		return "", errors.New("update: payload changed while opening")
	}
	h := sha256.New()
	n, err := io.CopyBuffer(io.MultiWriter(h, dst), io.LimitReader(f, size+1), make([]byte, 32*1024))
	if err != nil {
		return "", fmt.Errorf("update: hash: %w", err)
	}
	if n != size {
		return "", fmt.Errorf("update: размер файла %d ≠ манифесту %d", n, size)
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !samePairFile(opened, after) || after.Size() != size || !opened.ModTime().Equal(after.ModTime()) {
		return "", errors.New("update: payload changed while reading")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

// CurrentFloor is a compatibility/display wrapper; it discards storage errors.
// Deprecated: all acceptance and startup decisions must use ReadFloor.
func CurrentFloor() Version {
	v, _ := ReadFloor() // compatibility only; security callers must handle ReadFloor errors
	return v
}

// RaiseFloor — поднять этаж (только вверх). Возвращает фактический этаж.
func RaiseFloor(v Version) (Version, error) {
	return raiseFloor(v)
}

// String — "2.1.0" (без .build, если нулевая; "0.0.0" для нулевой версии).
func (v Version) String() string {
	if v.Build == 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
}
