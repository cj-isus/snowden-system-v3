// updatetool — офлайн-CLI подписи обновлений приложения (V2-050/F15).
//
// Значения не печатает: наружу — только метаданные (версия, sha256, key_id).
//
// Использование:
//
//	updatetool keygen <trusted_keys.json> <private.key>
//	  — отдельная пара Ed25519 с явной ролью update в scoped trust policy; comment
//	    не назначает полномочий. Профильные ключи сохраняются отдельно, приватный
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
//	updatetool sign-pair[-copy] <app.exe> <helper.exe> <version> <private.key> <new-outdir> [notes]
//	  — schema2 Windows/amd64, signed fixed app/helper/protocol; manifest last.
//	updatetool verify-pair <update.json> <payload-dir> <trusted_keys.json>
//	  — update-role signature and both bounded hashes; not an installer/OS check.
//
//	updatetool verify <update.json> <payload> <trusted_keys.json>
//	  — офлайн-проверка: структура + подпись + sha256 (без политики версий —
//	    это делает приложение при применении).
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/update"
	"github.com/snowden-system/windows/tools/internal/deliverypack"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
	"github.com/snowden-system/windows/tools/internal/publisherkeys"
	"github.com/snowden-system/windows/tools/internal/repository"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "updatetool: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: updatetool keygen|sign|sign-copy|sign-pair|sign-pair-copy|verify|verify-pair|delivery-pack|repository ...")
	}
	switch os.Args[1] {
	case "repository":
		report, err := repository.Run(os.Args[2:])
		if err != nil {
			fatal("repository: %v", err)
		}
		fmt.Println(string(report))
	case "delivery-pack":
		report, err := deliverypack.Run(context.Background(), delivery.UpdateManifest, os.Args[2:])
		if err != nil {
			fatal("delivery-pack: %v", err)
		}
		fmt.Println(string(report))
	case "keygen":
		cmdKeygen()
	case "sign":
		cmdSign(false)
	case "sign-copy":
		cmdSign(true)
	case "sign-pair":
		cmdSignPair(false)
	case "sign-pair-copy":
		cmdSignPair(true)
	case "verify-pair":
		cmdVerifyPair()
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
	keyID, err := publisherkeys.Generate(keysPath, privPath, metadata.RoleUpdate)
	if err != nil {
		fatal("provision: %v", err)
	}
	fmt.Printf("ok: key %s role=update; private -> %s (offline); restart after local policy provisioning\n", keyID, privPath)
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
	if err := m.Validate(); err != nil {
		fatal("manifest: %v", err)
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
	mb, err := publisherfiles.Read(os.Args[2], update.MaxManifestSize)
	if err != nil {
		fatal("manifest: %v", err)
	}
	m, err := update.DecodeStrict(mb)
	if err != nil {
		fatal("manifest: %v", err)
	}
	if m.Schema == update.PairSchema {
		fatal("schema2 requires verify-pair; one EXE is not the complete artifact set")
	}
	policy, present, err := metadata.ReadTrustPolicyFile(os.Args[4])
	if err != nil || !present {
		fatal("trusted keys: %v", err)
	}
	keys := policy.VerificationKeys(metadata.RoleUpdate)
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
