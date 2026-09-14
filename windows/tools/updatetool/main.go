// updatetool — офлайн-CLI подписи обновлений приложения (V2-050/F15).
//
// Значения не печатает: наружу — только метаданные (версия, sha256, key_id).
//
// Использование:
//
//	updatetool keygen <trusted_keys.json> <private.key>
//	  — пара Ed25519 ДЛЯ ПОДПИСИ ОБНОВЛЕНИЙ: публичный ключ добавляется в ту же
//	    таблицу доверия, что и envelope (comment "update-signing"), приватный
//	    (hex) — в отдельный файл 0600 (хранится офлайн).
//
//	updatetool sign <payload.exe> <version> <private.key> <outdir> [notes]
//	  — sha256 payload, манифест update.json, подпись canonical-JSON,
//	    запись outdir/update.json. Payload в outdir не копируется (по умолчанию
//	    владелец раскладывает пару файлов сам; см. copy).
//
//	updatetool sign-copy <payload.exe> <version> <private.key> <outdir> [notes]
//	  — то же + копия payload в outdir (готовый каталог обновления).
//
//	updatetool verify <update.json> <payload> <trusted_keys.json>
//	  — офлайн-проверка: структура + подпись + sha256 (без политики версий —
//	    это делает приложение при применении).
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/update"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "updatetool: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: updatetool keygen|sign|sign-copy|verify ...")
	}
	switch os.Args[1] {
	case "keygen":
		cmdKeygen()
	case "sign":
		cmdSign(false)
	case "sign-copy":
		cmdSign(true)
	case "verify":
		cmdVerify()
	default:
		fatal("unknown command %q", os.Args[1])
	}
}

func cmdKeygen() {
	if len(os.Args) != 4 {
		fatal("usage: updatetool keygen <trusted_keys.json> <private.key>")
	}
	keysPath, privPath := os.Args[2], os.Args[3]
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal("generate: %v", err)
	}
	// Таблица доверия: та же схема, что у AppData/metadata/trusted_keys.json.
	var f struct {
		Schema int `json:"schema"`
		Keys   []struct {
			KeyID   string `json:"key_id"`
			Public  string `json:"public_hex"`
			Comment string `json:"comment,omitempty"`
		} `json:"keys"`
	}
	data, err := os.ReadFile(keysPath)
	if err == nil {
		if err := json.Unmarshal(data, &f); err != nil {
			fatal("parse %s: %v", keysPath, err)
		}
		if f.Schema != 1 {
			fatal("%s: schema %d", keysPath, f.Schema)
		}
	} else if !os.IsNotExist(err) {
		fatal("read %s: %v", keysPath, err)
	} else {
		f.Schema = 1
	}
	keyID := metadata.KeyIDFor(pub)
	for _, k := range f.Keys {
		if k.KeyID == keyID {
			fatal("key %s уже в таблице", keyID)
		}
	}
	f.Keys = append(f.Keys, struct {
		KeyID   string `json:"key_id"`
		Public  string `json:"public_hex"`
		Comment string `json:"comment,omitempty"`
	}{KeyID: keyID, Public: hex.EncodeToString(pub), Comment: "update-signing"})
	out, _ := json.MarshalIndent(&f, "", "  ")
	if err := atomicWrite(keysPath, out); err != nil {
		fatal("write keys: %v", err)
	}
	if err := os.WriteFile(privPath, []byte(hex.EncodeToString(priv)), 0o600); err != nil {
		fatal("write private: %v", err)
	}
	fmt.Printf("ok: key %s добавлен в %s\n    приватный (hex) -> %s (хранить офлайн)\n", keyID, keysPath, privPath)
}

func loadPriv(path string) ed25519.PrivateKey {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("read private key: %v", err)
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		fatal("private key: ожидался hex %d байт", ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(raw)
}

func cmdSign(copyPayload bool) {
	if len(os.Args) < 6 {
		fatal("usage: updatetool sign|sign-copy <payload.exe> <version> <private.key> <outdir> [notes]")
	}
	payload, version, privPath, outDir := os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	notes := ""
	if len(os.Args) > 6 {
		notes = os.Args[6]
	}
	if _, err := update.ParseVersion(version); err != nil {
		fatal("version: %v", err)
	}
	st, err := os.Stat(payload)
	if err != nil {
		fatal("payload: %v", err)
	}
	sum, err := fileSHA256(payload)
	if err != nil {
		fatal("sha256: %v", err)
	}
	m := update.Manifest{
		Schema:     update.Schema,
		Version:    version,
		ReleasedAt: time.Now().UTC(),
		Notes:      notes,
		File:       filepath.Base(payload),
		SHA256:     hex.EncodeToString(sum),
		Size:       st.Size(),
	}
	if err := m.Sign(loadPriv(privPath)); err != nil {
		fatal("sign: %v", err)
	}
	mb, _ := json.MarshalIndent(&m, "", "  ")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal("outdir: %v", err)
	}
	out := filepath.Join(outDir, "update.json")
	if err := atomicWrite(out, mb); err != nil {
		fatal("write manifest: %v", err)
	}
	if copyPayload {
		dst := filepath.Join(outDir, m.File)
		if err := copyFile(payload, dst); err != nil {
			fatal("copy payload: %v", err)
		}
	}
	fmt.Printf("ok: %s (v%s, %s %d байт, sha256 %.16s…)\n", out, version, m.File, m.Size, m.SHA256)
}

func cmdVerify() {
	if len(os.Args) != 5 {
		fatal("usage: updatetool verify <update.json> <payload> <trusted_keys.json>")
	}
	mb, err := os.ReadFile(os.Args[2])
	if err != nil {
		fatal("manifest: %v", err)
	}
	var m update.Manifest
	dec := json.NewDecoder(strings.NewReader(string(mb)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		fatal("manifest: %v", err)
	}
	if err := m.Validate(); err != nil {
		fatal("validate: %v", err)
	}
	data, err := os.ReadFile(os.Args[4])
	if err != nil {
		fatal("trusted keys: %v", err)
	}
	var f struct {
		Keys []struct {
			KeyID  string `json:"key_id"`
			Public string `json:"public_hex"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		fatal("trusted keys: %v", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, k := range f.Keys {
		raw, err := hex.DecodeString(k.Public)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			fatal("trusted key %s: bad hex", k.KeyID)
		}
		keys[k.KeyID] = ed25519.PublicKey(raw)
	}
	if err := m.Verify(keys); err != nil {
		fatal("verify: %v", err)
	}
	if err := m.VerifyPayload(os.Args[3]); err != nil {
		fatal("payload: %v", err)
	}
	fmt.Printf("ok: подпись и sha256 подтверждены (v%s, key %s)\n", m.Version, m.KeyID)
}

// ---------- helpers ----------

func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
