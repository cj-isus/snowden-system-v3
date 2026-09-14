// Package onboarding — перенос состояния устройства на новое устройство
// (V2-049/F8): один шифрованный бандл + QR/файл.
//
// Гарантии (fail-closed по духу V2-033/034):
//   - Бандл зашифрован scrypt(N=2^15,r=8,p=1)+AES-256-GCM от парольной фразы;
//     короткая фраза = слабый бандл, но враг без фразы получает только мусор.
//   - Внутри бандла — ПОДПИСАННЫЙ envelope в исходном виде; при импорте подпись
//     проверяется ключами ИЗ бандла до любой записи на диск. Подделанный
//     (переподписанный чужим ключом) бандл расшифруется, но будет отвергнут
//     решением человека (отпечаток ключа показывается до подтверждения).
//   - Версия envelope на целевой машине НЕ понижается (антидаунгрейд V2-034).
//   - Секреты переносятся как значения: на исходной машине читаются из DPAPI-
//     хранилища, на целевой пишутся в DPAPI того пользователя.
package onboarding

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
)

const (
	// Magic "SNOB" + версия формата бандла.
	magic = "SNOB"
	// scrypt параметры: на mid-range CPU ~1-2 c в Go (десктоп, не мобильный).
	scryptN  = 1 << 15
	scryptR  = 8
	scryptP  = 1
	keyLen   = 32 // AES-256
	saltLen  = 32
	nonceLen = 12
)

// MaxBundleSize — верхняя граница полезной нагрузки (секреты + envelope
// + ключи укладываются в единицы килобайт; граница отсекает мусор/DoS).
const MaxBundleSize = 256 * 1024

// Errors, по которым вызывающий код принимает решения.
var (
	ErrWrongPassphrase = errors.New("onboarding: неверная парольная фраза или повреждённый бандл")
	ErrTooLarge        = errors.New("onboarding: бандл превышает разумный размер")
	ErrBadFormat       = errors.New("onboarding: формат бандла не распознан")
)

// TrustedKey — открытый ключ из таблицы доверия (публичные данные).
type TrustedKey struct {
	KeyID   string `json:"key_id"`
	Public  string `json:"public_hex"`
	Comment string `json:"comment,omitempty"`
}

// SecretSlot — один секрет (значение уже расшифровано из DPAPI источника).
type SecretSlot struct {
	Kind  string `json:"kind"`  // secretvault.Kind
	ID    string `json:"id"`    // id записи в vault источника (для стабильности)
	Value string `json:"value"` // открытым текстом только ВНУТРИ зашифрованного бандла
}

// BundlePayload — содержимое бандла.
type BundlePayload struct {
	Schema      int             `json:"schema"` // всегда 1
	CreatedAt   time.Time       `json:"created_at"`
	DeviceName  string          `json:"device_name,omitempty"`
	TrustedKeys []TrustedKey    `json:"trusted_keys"`
	Envelope    json.RawMessage `json:"envelope"` // подписанный channels.json как есть
	Secrets     []SecretSlot    `json:"secrets"`
}

// ---------- шифрованный контейнер ----------

// sealedFormat — то, что реально сериализуется в base64-строку бандла:
// magic | version | scrypt params | salt | nonce | AES-GCM(gzip(payload)).
type sealedFormat struct {
	Magic    string `json:"magic"`
	Version  int    `json:"version"`
	N        int    `json:"n"`
	R        int    `json:"r"`
	P        int    `json:"p"`
	SaltB64  string `json:"salt"`
	NonceB64 string `json:"nonce"`
	DataB64  string `json:"data"` // gzip(payload) зашифрованный AES-GCM
}

// Seal — зашифровать payload парольной фразой в transport-строку
// (то, что кодируется в QR / пишется в файл).
func Seal(payload *BundlePayload, passphrase string) (string, error) {
	if payload == nil {
		return "", errors.New("onboarding: payload nil")
	}
	if payload.Schema == 0 {
		payload.Schema = 1
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("onboarding: marshal payload: %w", err)
	}
	var gz bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	if _, err := zw.Write(plain); err != nil {
		return "", fmt.Errorf("onboarding: gzip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("onboarding: gzip close: %w", err)
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("onboarding: salt: %w", err)
	}
	key, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return "", fmt.Errorf("onboarding: scrypt: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("onboarding: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("onboarding: gcm: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("onboarding: nonce: %w", err)
	}
	// Дополнительные данные: magic+version+params — подделка параметров
	// (например, подмена N на ослабляющий) рвёт расшифровку.
	aad := aadFor(1, scryptN, scryptR, scryptP, salt)
	ct := gcm.Seal(nil, nonce, gz.Bytes(), aad)

	sealed := sealedFormat{
		Magic: magic, Version: 1,
		N: scryptN, R: scryptR, P: scryptP,
		SaltB64:  base64.StdEncoding.EncodeToString(salt),
		NonceB64: base64.StdEncoding.EncodeToString(nonce),
		DataB64:  base64.StdEncoding.EncodeToString(ct),
	}
	raw, err := json.Marshal(sealed)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	out.WriteString(magic + "1.")
	out.WriteString(base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(raw))
	return out.String(), nil
}

// Open расшифровывает transport-строку. Неверная фраза/повреждение —
// ErrWrongPassphrase (GCM tag), мусор — ErrBadFormat.
func Open(transport string, passphrase string) (*BundlePayload, error) {
	transport = strings.TrimSpace(transport)
	if len(transport) > MaxBundleSize {
		return nil, ErrTooLarge
	}
	dot := strings.IndexByte(transport, '.')
	if dot <= 0 || transport[:dot] != magic+"1" {
		return nil, ErrBadFormat
	}
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(transport[dot+1:])
	if err != nil {
		return nil, ErrBadFormat
	}
	var sealed sealedFormat
	if err := json.Unmarshal(raw, &sealed); err != nil {
		return nil, ErrBadFormat
	}
	if sealed.Magic != magic || sealed.Version != 1 {
		return nil, ErrBadFormat
	}
	// Параметры KDF принимаются из бандла, но с границами снизу
	// (слабые параметры отвергаем — не даём врагу их ослабить).
	if sealed.N < 1<<14 || sealed.R < 8 || sealed.P < 1 || sealed.N > 1<<21 {
		return nil, ErrBadFormat
	}
	salt, err := base64.StdEncoding.DecodeString(sealed.SaltB64)
	if err != nil || len(salt) < 16 {
		return nil, ErrBadFormat
	}
	nonce, err := base64.StdEncoding.DecodeString(sealed.NonceB64)
	if err != nil || len(nonce) != nonceLen {
		return nil, ErrBadFormat
	}
	ct, err := base64.StdEncoding.DecodeString(sealed.DataB64)
	if err != nil || len(ct) == 0 {
		return nil, ErrBadFormat
	}
	key, err := scrypt.Key([]byte(passphrase), salt, sealed.N, sealed.R, sealed.P, keyLen)
	if err != nil {
		return nil, ErrBadFormat
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrBadFormat
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrBadFormat
	}
	aad := aadFor(sealed.Version, sealed.N, sealed.R, sealed.P, salt)
	plain, err := gcm.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, ErrWrongPassphrase
	}
	zr, err := gzip.NewReader(bytes.NewReader(plain))
	if err != nil {
		return nil, ErrBadFormat
	}
	payloadJSON, err := io.ReadAll(io.LimitReader(zr, MaxBundleSize))
	if err != nil {
		return nil, ErrBadFormat
	}
	var payload BundlePayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, ErrBadFormat
	}
	if payload.Schema != 1 {
		return nil, ErrBadFormat
	}
	return &payload, nil
}

// aadFor — AAD для GCM: связывает заголовок с шифротекстом.
func aadFor(version, n, r, p int, salt []byte) []byte {
	h := sha256.New()
	_ = binary.Write(h, binary.BigEndian, uint32(version))
	_ = binary.Write(h, binary.BigEndian, uint32(n))
	_ = binary.Write(h, binary.BigEndian, uint32(r))
	_ = binary.Write(h, binary.BigEndian, uint32(p))
	h.Write(salt)
	return h.Sum(nil)
}

// KeyFingerprint — отпечаток открытого ключа для показа человеку
// (первые 16 hex-символов SHA-256).
func KeyFingerprint(publicHex string) string {
	raw, err := hex.DecodeString(publicHex)
	if err != nil {
		return "invalid"
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}
