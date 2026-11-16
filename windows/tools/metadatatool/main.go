// metadatatool — офлайн-CLI для FR-008 (B2): генерация ключей, подпись и
// проверка envelope'ов каналов. Значения секретов инструмент не читает и не
// печатает: envelope содержит ТОЛЬКО дескрипторы со ссылками.
//
// Использование:
//
//	metadatatool keygen <keys.json> <new-private.key> [profile|recovery]
//	  — новая пара Ed25519: публичный ключ добавляется в trusted_keys.json,
//	    приватный (hex) — в отдельный файл 0600 (хранится ОФЛАЙН, не в AppData).
//
//	metadatatool sign <envelope.json> <keyid|@private.key> <out.json>
//	  — строгая валидация структуры, подпись canonical-JSON, запись out.
//	    version/issued/expires задаются в envelope.json (это данные издателя).
//
//	metadatatool verify <envelope.json> <trusted_keys.json>
//	  — проверка подписи + валидации против таблицы доверенных ключей.
//
//	metadatatool bind <source.json> <bindings.json> <@private.key> <new-output.json>
//	  — явная миграция legacy refs, новая версия/срок/подпись, новый output.
//
//	metadatatool pin <public-leaf.pem> <new-pin.json>
//	  — bounded public certificate → normalized PEM/DER fingerprint; new output.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/deliverypack"
	"github.com/snowden-system/windows/tools/internal/publisherkeys"
	"github.com/snowden-system/windows/tools/internal/repository"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "metadatatool: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: metadatatool keygen|sign|verify|bind|pin|recovery-sign|delivery-pack|pin-policy-pack|repository ...")
	}
	switch os.Args[1] {
	case "repository":
		report, err := repository.Run(os.Args[2:])
		if err != nil {
			fatal("repository: %v", err)
		}
		fmt.Println(string(report))
	case "pin-policy-pack":
		report, err := deliverypack.Run(context.Background(), delivery.PinPolicy, os.Args[2:])
		if err != nil {
			fatal("pin-policy-pack: %v", err)
		}
		fmt.Println(string(report))
	case "delivery-pack":
		report, err := deliverypack.Run(context.Background(), delivery.Profile, os.Args[2:])
		if err != nil {
			fatal("delivery-pack: %v", err)
		}
		fmt.Println(string(report))
	case "keygen":
		cmdKeygen()
	case "sign":
		cmdSign()
	case "verify":
		cmdVerify()
	case "bind":
		if err := runBind(os.Args[2:], time.Now()); err != nil {
			fatal("bind: %v", err)
		}
		fmt.Println("binding migration written")
	case "pin":
		if err := runPin(os.Args[2:]); err != nil {
			fatal("pin: %v", err)
		}
		fmt.Println("public certificate pin written")
	case "recovery-sign":
		if err := runRecoverySign(os.Args[2:], time.Now()); err != nil {
			fatal("recovery-sign: %v", err)
		}
		fmt.Println("profile recovery permit written; use explicit recovery preview/apply")
	default:
		fatal("unknown command %q", os.Args[1])
	}
}

func cmdKeygen() {
	if len(os.Args) != 4 && len(os.Args) != 5 {
		fatal("usage: metadatatool keygen <trusted_keys.json> <new-private.key> [profile|recovery]")
	}
	keysPath, privPath := os.Args[2], os.Args[3]
	role := metadata.RoleProfile
	if len(os.Args) == 5 {
		role = metadata.KeyRole(os.Args[4])
	}
	if role != metadata.RoleProfile && role != metadata.RoleRecovery {
		fatal("metadatatool provisions profile/recovery roles only")
	}
	id, err := publisherkeys.Generate(keysPath, privPath, role)
	if err != nil {
		fatal("provision: %v", err)
	}
	fmt.Printf("key_id: %s role: %s; private key -> %s (offline); restart after local policy provisioning\n", id, role, privPath)
}

func cmdSign() {
	if len(os.Args) != 5 {
		fatal("usage: metadatatool sign <envelope.json> <@private.key> <out.json>")
	}
	raw, err := readBindingFile(os.Args[2], metadata.MaxEnvelopeSize)
	if err != nil {
		fatal("read envelope: %v", err)
	}
	e, err := metadata.DecodeStrict(raw)
	if err != nil {
		fatal("%v", err)
	}
	if err := e.ValidateCandidate(time.Now()); err != nil {
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
	fmt.Printf("signed: epoch=%d version=%d key_id=%s expires=%s → %s\n", e.ProfileEpoch, e.MetadataVersion, e.KeyID, e.ExpiresAt.Format(time.RFC3339), os.Args[4])
}

func cmdVerify() {
	if len(os.Args) != 4 {
		fatal("usage: metadatatool verify <envelope.json> <trusted_keys.json>")
	}
	raw, err := readBindingFile(os.Args[2], metadata.MaxEnvelopeSize)
	if err != nil {
		fatal("read envelope: %v", err)
	}
	e, err := metadata.DecodeStrict(raw)
	if err != nil {
		fatal("%v", err)
	}
	// Таблица ключей лежит рядом со своим canonical именем в каталоге AppData;
	// для офлайн-проверки допускаем произвольный путь: копируем сторову логику.
	data, err := readBindingFile(os.Args[3], metadata.MaxTrustPolicySize)
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
	if err := e.ValidateCandidate(time.Now()); err != nil {
		fatal("signature OK, но envelope невалиден: %v", err)
	}
	fmt.Printf("VERIFY OK: epoch=%d version=%d key_id=%s channels=%d revocations=%d expires=%s\n",
		e.ProfileEpoch, e.MetadataVersion, e.KeyID, len(e.Channels), len(e.Revocations), e.ExpiresAt.Format(time.RFC3339))
}

// readPrivHex — приватный ключ из файла "@path" (единственный формат CLI).
func readPrivHex(arg string) (string, error) {
	if !strings.HasPrefix(arg, "@") {
		return "", fmt.Errorf("ключ задаётся как @private.key (офлайн-файл)")
	}
	b, err := readBindingFile(strings.TrimPrefix(arg, "@"), 1024)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// parseKeysBytes — разбор таблицы ключей произвольного пути (тот же формат,
// что metadata.Store.LoadKeys; дублирование осознанное: Store жёстко привязан
// к своему каталогу).
func parseKeysBytes(data []byte) (map[string]ed25519.PublicKey, error) {
	policy, err := metadata.DecodeTrustPolicy(data)
	if err != nil {
		return nil, err
	}
	keys := policy.VerificationKeys(metadata.RoleProfile)
	if len(keys) == 0 {
		return nil, fmt.Errorf("keys table empty")
	}
	return keys, nil
}

// Legacy helper; only absence is empty. Publisher CLI provisioning uses the
// shared role-aware implementation and never flattens another role's records.
func parseKeysFileOrEmpty(path string) map[string]ed25519.PublicKey {
	policy, _, err := metadata.ReadTrustPolicyFile(path)
	if err != nil {
		fatal("existing keys damaged (%v) — refusing to append", err)
	}
	return policy.VerificationKeys(metadata.RoleProfile)
}

// writeKeysFile — атомарная запись таблицы ключей.
func writeKeysFile(path string, keys map[string]ed25519.PublicKey) error {
	policy, present, err := metadata.ReadTrustPolicyFile(path)
	if err != nil {
		return err
	}
	if present && policy.Schema == 2 {
		return fmt.Errorf("legacy helper cannot flatten a scoped trust policy")
	}
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
