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
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const (
	// Magic "SNOB" + версия формата бандла.
	magic = "SNOB"
	// Export defaults stay stable; imported costs are checked in kdf.go.
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
	return (bundleCodec{kdf: defaultKDF}).seal(payload, passphrase)
}

// Open validates bounded container inputs and derives through the shared gate.
func Open(transport, passphrase string) (*BundlePayload, error) {
	return (bundleCodec{kdf: defaultKDF}).open(transport, passphrase)
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
