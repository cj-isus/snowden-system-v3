// metadatatool — офлайн-CLI для FR-008 (B2): генерация ключей, подпись и
// проверка envelope'ов каналов. Значения секретов инструмент не читает и не
// печатает: envelope содержит ТОЛЬКО дескрипторы со ссылками.
//
// Использование:
//
//	metadatatool keygen <keys.json> <private.pem-ish.key>
//	  — новая пара Ed25519: публичный ключ добавляется в trusted_keys.json,
//	    приватный (hex) — в отдельный файл 0600 (хранится ОФЛАЙН, не в AppData).
//
//	metadatatool sign <envelope.json> <keyid|@private.key> <out.json>
//	  — строгая валидация структуры, подпись canonical-JSON, запись out.
//	    version/issued/expires задаются в envelope.json (это данные издателя).
//
//	metadatatool verify <envelope.json> <trusted_keys.json>
//	  — проверка подписи + валидации против таблицы доверенных ключей.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "metadatatool: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: metadatatool keygen|sign|verify ...")
	}
	switch os.Args[1] {
	case "keygen":
		cmdKeygen()
	case "sign":
		cmdSign()
	case "verify":
		cmdVerify()
	default:
		fatal("unknown command %q", os.Args[1])
	}
}

func cmdKeygen() {
	if len(os.Args) != 4 {
		fatal("usage: metadatatool keygen <trusted_keys.json> <private.key>")
	}
	keysPath, privPath := os.Args[2], os.Args[3]
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal("generate: %v", err)
	}
	keys := parseKeysFileOrEmpty(keysPath)
	id := metadata.KeyIDFor(pub)
	keys[id] = pub
	if err := writeKeysFile(keysPath, keys); err != nil {
		fatal("save keys: %v", err)
	}
	if err := os.WriteFile(privPath, []byte(hex.EncodeToString(priv)), 0o600); err != nil {
		fatal("save private key: %v", err)
	}
	fmt.Printf("key_id: %s\npublic_hex: %s\nprivate key written to %s (0600) — хранить офлайн\n", id, hex.EncodeToString(pub), privPath)
}

func cmdSign() {
	if len(os.Args) != 5 {
		fatal("usage: metadatatool sign <envelope.json> <@private.key> <out.json>")
	}
	raw, err := os.ReadFile(os.Args[2])
	if err != nil {
		fatal("read envelope: %v", err)
	}
	e, err := metadata.DecodeStrict(raw)
	if err != nil {
		fatal("%v", err)
	}
	if err := e.Validate(time.Now(), 0); err != nil {
		fatal("envelope invalid: %v", err)
	}
	privHex, err := readPrivHex(os.Args[3])
	if err != nil {
		fatal("%v", err)
	}
	privBytes, err := hex.DecodeString(privHex)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		fatal("private key: invalid hex or size")
	}
	pub := ed25519.PrivateKey(privBytes).Public().(ed25519.PublicKey)
	e.KeyID = metadata.KeyIDFor(pub)
	if err := e.Sign(ed25519.PrivateKey(privBytes)); err != nil {
		fatal("sign: %v", err)
	}
	out, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		fatal("marshal: %v", err)
	}
	if err := os.WriteFile(os.Args[4], out, 0o600); err != nil {
		fatal("write: %v", err)
	}
	fmt.Printf("signed: version=%d key_id=%s expires=%s → %s\n", e.MetadataVersion, e.KeyID, e.ExpiresAt.Format(time.RFC3339), os.Args[4])
}

func cmdVerify() {
	if len(os.Args) != 4 {
		fatal("usage: metadatatool verify <envelope.json> <trusted_keys.json>")
	}
	raw, err := os.ReadFile(os.Args[2])
	if err != nil {
		fatal("read envelope: %v", err)
	}
	e, err := metadata.DecodeStrict(raw)
	if err != nil {
		fatal("%v", err)
	}
	// Таблица ключей лежит рядом со своим canonical именем в каталоге AppData;
	// для офлайн-проверки допускаем произвольный путь: копируем сторову логику.
	data, err := os.ReadFile(os.Args[3])
	if err != nil {
		fatal("read keys: %v", err)
	}
	keys, err := parseKeysBytes(data)
	if err != nil {
		fatal("%v", err)
	}
	if err := e.Verify(keys); err != nil {
		fatal("VERIFY FAIL: %v", err)
	}
	if err := e.Validate(time.Now(), 0); err != nil {
		fatal("signature OK, но envelope невалиден: %v", err)
	}
	fmt.Printf("VERIFY OK: version=%d key_id=%s channels=%d revocations=%d expires=%s\n",
		e.MetadataVersion, e.KeyID, len(e.Channels), len(e.Revocations), e.ExpiresAt.Format(time.RFC3339))
}

// readPrivHex — приватный ключ из файла "@path" (единственный формат CLI).
func readPrivHex(arg string) (string, error) {
	if !strings.HasPrefix(arg, "@") {
		return "", fmt.Errorf("ключ задаётся как @private.key (офлайн-файл)")
	}
	b, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// parseKeysBytes — разбор таблицы ключей произвольного пути (тот же формат,
// что metadata.Store.LoadKeys; дублирование осознанное: Store жёстко привязан
// к своему каталогу).
func parseKeysBytes(data []byte) (map[string]ed25519.PublicKey, error) {
	var f struct {
		Schema int `json:"schema"`
		Keys   []struct {
			KeyID  string `json:"key_id"`
			Public string `json:"public_hex"`
		} `json:"keys"`
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("parse keys: %w", err)
	}
	if f.Schema != 1 {
		return nil, fmt.Errorf("keys schema %d", f.Schema)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, k := range f.Keys {
		raw, err := hex.DecodeString(k.Public)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("key %s: bad public hex", k.KeyID)
		}
		if k.KeyID != "" && k.KeyID != metadata.KeyIDFor(ed25519.PublicKey(raw)) {
			return nil, fmt.Errorf("key %s: key_id mismatch", k.KeyID)
		}
		keys[metadata.KeyIDFor(ed25519.PublicKey(raw))] = ed25519.PublicKey(raw)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("keys table empty")
	}
	return keys, nil
}

// parseKeysFileOrEmpty — чтение таблицы (отсутствует → пустая).
func parseKeysFileOrEmpty(path string) map[string]ed25519.PublicKey {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]ed25519.PublicKey{}
	}
	keys, err := parseKeysBytes(data)
	if err != nil {
		fatal("existing keys damaged (%v) — refusing to append", err)
	}
	return keys
}

// writeKeysFile — атомарная запись таблицы ключей.
func writeKeysFile(path string, keys map[string]ed25519.PublicKey) error {
	type keyEntry struct {
		KeyID  string `json:"key_id"`
		Public string `json:"public_hex"`
	}
	f := struct {
		Schema int        `json:"schema"`
		Keys   []keyEntry `json:"keys"`
	}{Schema: 1}
	for id, pub := range keys {
		f.Keys = append(f.Keys, keyEntry{KeyID: id, Public: hex.EncodeToString(pub)})
	}
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
