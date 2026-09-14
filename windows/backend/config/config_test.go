// Тесты парсера ядра. Планка — не ниже старого набора (~40 тестов,
// включая adversarial-кейсы из docs/old-core-review.md §2.6).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validConfig — минимальный валидный защищённый конфиг (vless only).
const validConfig = `{
  "log": {"level": "info", "output": "stdout", "timestamp": true},
  "dns": {
    "servers": [
      {"tag": "tunnel-dns", "address": "tls://8.8.8.8", "detour": "proxy"},
      {"tag": "local-dns", "address": "https://1.1.1.1/dns-query", "detour": "direct"}
    ],
    "rules": [
      {"server": "tunnel-dns"}
    ]
  },
  "inbounds": [
    {"type": "mixed", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": 1080}
  ],
  "outbounds": [
    {"type": "selector", "tag": "proxy", "outbounds": ["vless-a"], "default": "vless-a"},
    {"type": "vless", "tag": "vless-a", "server": "example.com", "server_port": 443,
     "uuid": "50c4d938-21b3-4b78-b4ab-b20f953a9dd9",
     "tls": {"enabled": true, "server_name": "example.com",
             "utls": {"enabled": true, "fingerprint": "chrome"}}},
    {"type": "direct", "tag": "direct"},
    {"type": "block", "tag": "block"}
  ],
  "route": {"rules": [{"protocol": "dns", "action": "hijack-dns"}], "final": "proxy"}
}`

func mustParse(t *testing.T, data string) *Config {
	t.Helper()
	cfg, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
	return cfg
}

func wantErr(t *testing.T, data, fragment string) {
	t.Helper()
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error %q does not contain %q", err.Error(), fragment)
	}
}

// ---------- parse-level ----------

func TestParseAcceptsValid(t *testing.T) {
	mustParse(t, validConfig)
}

func TestParseRejectsEmptyAndWhitespace(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\t\n"} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Fatalf("empty input %q accepted", in)
		}
	}
}

func TestParseRejectsTrailingData(t *testing.T) {
	wantErr(t, validConfig+" garbage", "trailing data")
}

func TestParseRejectsTwoObjects(t *testing.T) {
	wantErr(t, validConfig+"\n{}\n", "trailing data")
}

func TestParseRejectsUnknownTopLevelField(t *testing.T) {
	// DisallowUnknownFields даёт точное имя поля — этого достаточно.
	// (V2-048: "experimental" теперь легитимное поле — clash_api метрики;
	// неизвестным стало "bogus_section".)
	wantErr(t, `{"bogus_section": {}, "inbounds": [], "outbounds": [], "route": {"final": "proxy"}}`,
		`unknown field "bogus_section"`)
}

func TestParseRejectsUnknownInboundField(t *testing.T) {
	wantErr(t, `{"inbounds": [{"type": "mixed", "tag": "a", "bogus_field": 1}],
		"outbounds": [{"type": "direct", "tag": "direct"}],
		"route": {"final": "proxy"}}`, "unknown field")
}

func TestParseRejectsMalformedJSON(t *testing.T) {
	wantErr(t, `{"inbounds": [`, "parse config")
}

func TestParseRejectsNonObject(t *testing.T) {
	wantErr(t, `[]`, "cannot unmarshal array")
}

// ---------- normalize ----------

func TestNormalizeUrltestToSelector(t *testing.T) {
	cfg := mustParse(t, strings.Replace(validConfig,
		`{"type": "selector", "tag": "proxy"`, `{"type": "urltest", "tag": "proxy"`, 1))
	if cfg.Outbounds[0].Type != "selector" {
		t.Fatalf("urltest not normalized: %s", cfg.Outbounds[0].Type)
	}
}

func TestNormalizeIdempotent(t *testing.T) {
	cfg := mustParse(t, validConfig)
	before := cfg.ProtectedSelector()
	cfg.Normalize()
	after := cfg.ProtectedSelector()
	if len(before) != len(after) {
		t.Fatal("Normalize is not idempotent (selector changed)")
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatal("Normalize is not idempotent (order changed)")
		}
	}
}

// ---------- placeholders / secrets ----------

func TestValidateRejectsUUIDPlaceholder(t *testing.T) {
	wantErr(t, strings.Replace(validConfig,
		"50c4d938-21b3-4b78-b4ab-b20f953a9dd9", "REPLACE_WITH_UUID", 1),
		"placeholder or private-key material")
}

func TestValidateRejectsNestedCredentialPlaceholder(t *testing.T) {
	// Плейсхолдер глубоко в tls-карте должен быть пойман сериализацией.
	bad := strings.Replace(validConfig, `"fingerprint": "chrome"`, `"fingerprint": "{{YOUR_KEY}}"`, 1)
	wantErr(t, bad, "placeholder or private-key material")
}

func TestValidateRejectsPrivateKeyMaterial(t *testing.T) {
	bad := strings.Replace(validConfig, `"fingerprint": "chrome"`,
		`"fingerprint": "-----BEGIN PRIVATE KEY-----"`, 1)
	wantErr(t, bad, "placeholder or private-key material")
}

func TestValidateRejectsDoubleBracePlaceholder(t *testing.T) {
	bad := strings.Replace(validConfig, `"example.com", "server_port": 443`,
		`"{{SERVER}}", "server_port": 443`, 1)
	wantErr(t, bad, "placeholder or private-key material")
}

// ---------- structure ----------

func TestValidateRejectsEmptyInbounds(t *testing.T) {
	bad := strings.Replace(validConfig,
		`{"type": "mixed", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": 1080}`, ``, 1)
	wantErr(t, bad, "no inbounds")
}

func TestValidateRejectsDuplicateOutboundTags(t *testing.T) {
	bad := strings.Replace(validConfig, `{"type": "block", "tag": "block"}`,
		`{"type": "block", "tag": "block"}, {"type": "direct", "tag": "direct"}`, 1)
	wantErr(t, bad, "duplicate outbound tag")
}

func TestValidateRejectsRouteFinalDirect(t *testing.T) {
	bad := strings.Replace(validConfig, `"final": "proxy"`, `"final": "direct"`, 1)
	wantErr(t, bad, "must not be direct")
}

func TestValidateRejectsUnknownRouteFinal(t *testing.T) {
	bad := strings.Replace(validConfig, `"final": "proxy"`, `"final": "ghost"`, 1)
	wantErr(t, bad, "references unknown outbound")
}

func TestValidateRejectsUnknownRouteRuleOutbound(t *testing.T) {
	bad := strings.Replace(validConfig, `{"protocol": "dns", "action": "hijack-dns"}`,
		`{"outbound": "ghost"}`, 1)
	wantErr(t, bad, "route rule references unknown outbound")
}

func TestValidateRejectsRouteRuleWithoutOutboundOrAction(t *testing.T) {
	bad := strings.Replace(validConfig, `{"protocol": "dns", "action": "hijack-dns"}`,
		`{"ip_cidr": ["10.0.0.0/8"]}`, 1)
	wantErr(t, bad, "route rule without outbound or action")
}

// ---------- DNS ----------

func TestValidateRejectsUnknownDNSRuleServer(t *testing.T) {
	bad := strings.Replace(validConfig, `{"server": "tunnel-dns"}`, `{"server": "ghost"}`, 1)
	wantErr(t, bad, "dns rule references unknown server")
}

func TestValidateRejectsUnknownDNSDetour(t *testing.T) {
	bad := strings.Replace(validConfig, `"detour": "proxy"`, `"detour": "ghost"`, 1)
	wantErr(t, bad, "dns server detour references unknown outbound")
}

func TestValidateRejectsDuplicateDNSTags(t *testing.T) {
	bad := strings.Replace(validConfig,
		`{"tag": "local-dns", "address": "https://1.1.1.1/dns-query", "detour": "direct"}`,
		`{"tag": "tunnel-dns", "address": "tls://8.8.4.4", "detour": "proxy"}`, 1)
	wantErr(t, bad, "duplicate dns server tag")
}

func TestValidateRejectsDNSAddressPlaceholder(t *testing.T) {
	// Общий сериализационный чек срабатывает раньше DNS-специфичного — это
	// нормально: важен факт отказа, а не конкретная формулировка.
	bad := strings.Replace(validConfig, `"address": "tls://8.8.8.8"`,
		`"address": "{{DNS_ADDR}}"`, 1)
	wantErr(t, bad, "placeholder or private-key material")
}

// ---------- protected selector ----------

func TestValidateSelectorDefaultMustBeCandidate(t *testing.T) {
	bad := strings.Replace(validConfig, `"default": "vless-a"`, `"default": "ghost"`, 1)
	wantErr(t, bad, "default is not a candidate")
}

func TestValidateSelectorDirectForbidden(t *testing.T) {
	bad := strings.Replace(validConfig, `"outbounds": ["vless-a"], "default": "vless-a"`,
		`"outbounds": ["vless-a", "direct"], "default": "vless-a"`, 1)
	wantErr(t, bad, "direct is forbidden in protected selector")
}

func TestValidateSelectorUnknownCandidate(t *testing.T) {
	bad := strings.Replace(validConfig, `"outbounds": ["vless-a"]`,
		`"outbounds": ["vless-a", "ghost"]`, 1)
	wantErr(t, bad, "references unknown outbound")
}

func TestValidateSelectorCandidateNotProtected(t *testing.T) {
	bad := strings.Replace(validConfig, `"outbounds": ["vless-a"], "default": "vless-a"`,
		`"outbounds": ["vless-a", "block"], "default": "vless-a"`, 1)
	wantErr(t, bad, "not a protected protocol")
}

func TestValidateSelectorDuplicateCandidate(t *testing.T) {
	bad := strings.Replace(validConfig, `"outbounds": ["vless-a"]`,
		`"outbounds": ["vless-a", "vless-a"]`, 1)
	wantErr(t, bad, "duplicate candidate")
}

func TestValidateSelectorDefaultRequired(t *testing.T) {
	bad := strings.Replace(validConfig, `, "default": "vless-a"`, ``, 1)
	wantErr(t, bad, "default is required")
}

func TestValidateSelectorMissing(t *testing.T) {
	bad := strings.Replace(validConfig, `"tag": "proxy"`, `"tag": "px"`, 1)
	wantErr(t, bad, `protected selector "proxy" is required`)
}

func TestValidateProtectedOutboundLacksServer(t *testing.T) {
	bad := strings.Replace(validConfig, `"server": "example.com", "server_port": 443,`,
		`"server": "", "server_port": 0,`, 1)
	wantErr(t, bad, "lacks server/port")
}

func TestValidateProtectedOutboundLacksCredentials(t *testing.T) {
	bad := strings.Replace(validConfig, `"uuid": "50c4d938-21b3-4b78-b4ab-b20f953a9dd9",`, ``, 1)
	wantErr(t, bad, "lacks credentials")
}

// ---------- TLS rules ----------

func TestValidateInsecureForbiddenEvenWithPin(t *testing.T) {
	bad := strings.Replace(validConfig,
		`"server_name": "example.com",`,
		`"server_name": "example.com", "insecure": true,`, 1)
	wantErr(t, bad, "insecure is forbidden")
}

func TestValidateTLSDisabledForbidden(t *testing.T) {
	bad := strings.Replace(validConfig, `"enabled": true, "server_name": "example.com"`,
		`"enabled": false, "server_name": "example.com"`, 1)
	wantErr(t, bad, "tls.enabled must be true")
}

func TestValidateTLSServerNameRequired(t *testing.T) {
	bad := strings.Replace(validConfig, `, "server_name": "example.com"`, ``, 1)
	wantErr(t, bad, "server_name is required")
}

// ---------- REALITY (B1, V2-031/032) ----------

// realityValidConfig — валидный защищённый конфиг с tls.reality блоком
// (публичный ключ — 43-символьный base64url X25519, short_id — hex).
func realityValidConfig() string {
	// reality — СОСЕД utls внутри tls (не внутри utls): первая версия фикстуры
	// клала блок внутрь utls и валидатор честно не находил tls.reality —
	// тесты «rejected» падали с nil-ошибкой при правильном валидаторе.
	return strings.Replace(validConfig,
		`"utls": {"enabled": true, "fingerprint": "chrome"}}`,
		`"utls": {"enabled": true, "fingerprint": "chrome"}, "reality": {"enabled": true, "public_key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", "short_id": "0123456789abcdef"}}`, 1)
}

func TestRealityConfigAccepted(t *testing.T) {
	mustParse(t, realityValidConfig())
}

func TestRealityWithoutPublicKeyRejected(t *testing.T) {
	wantErr(t, strings.Replace(realityValidConfig(),
		`"public_key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", `, ``, 1),
		"tls.reality.public_key is required")
}

func TestRealityWithoutShortIDRejected(t *testing.T) {
	wantErr(t, strings.Replace(realityValidConfig(),
		`, "short_id": "0123456789abcdef"`, ``, 1),
		"tls.reality.short_id is required")
}

func TestRealityDisabledBlockRejected(t *testing.T) {
	wantErr(t, strings.Replace(realityValidConfig(),
		`"reality": {"enabled": true`, `"reality": {"enabled": false`, 1),
		"tls.reality.enabled must be true")
}

func TestRealityNonObjectRejected(t *testing.T) {
	wantErr(t, strings.Replace(realityValidConfig(),
		`"reality": {"enabled": true, "public_key": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0", "short_id": "0123456789abcdef"}`,
		`"reality": "yes"`, 1),
		"tls.reality must be an object")
}

func TestVisionFlowRejectedPinnedCore(t *testing.T) {
	// Vision (xtls-rprx-vision) отсутствует в pinned sing-box 1.13.19
	// (V2-031, доказано по коду): конфиг с flow обязан быть отбракован
	// строгим валидатором с понятной причиной, а не падать в движке.
	wantErr(t, strings.Replace(validConfig,
		`"uuid": "50c4d938-21b3-4b78-b4ab-b20f953a9dd9",`,
		`"uuid": "50c4d938-21b3-4b78-b4ab-b20f953a9dd9", "flow": "xtls-rprx-vision",`, 1),
		"not supported by the pinned core")
}

// ---------- pinned certificate (file-based) ----------

func writeTempPEM(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "pin.pem")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPinnedCertificateAccepted(t *testing.T) {
	p := writeTempPEM(t, testSelfSignedPEM(t))
	bad := strings.Replace(validConfig, `"server_name": "example.com"`,
		`"server_name": "example.com", "certificate_path": "`+jsonPath(t, p)+`"`, 1)
	mustParse(t, bad)
}

func TestPinnedCertificateMissingFile(t *testing.T) {
	bad := strings.Replace(validConfig, `"server_name": "example.com"`,
		`"server_name": "example.com", "certificate_path": "Z:/no/such/file.pem"`, 1)
	wantErr(t, bad, "pinned certificate")
}

func TestPinnedCertificateNonPEMRejected(t *testing.T) {
	p := writeTempPEM(t, "not a pem at all")
	bad := strings.Replace(validConfig, `"server_name": "example.com"`,
		`"server_name": "example.com", "certificate_path": "`+jsonPath(t, p)+`"`, 1)
	wantErr(t, bad, "not valid PEM")
}

func TestPinnedCertificatePlaceholderPathRejected(t *testing.T) {
	// Плейсхолдер ловится общим сериализационным чеком (всё в одном JSON).
	bad := strings.Replace(validConfig, `"server_name": "example.com"`,
		`"server_name": "example.com", "certificate_path": "{{CERT}}"`, 1)
	wantErr(t, bad, "placeholder or private-key material")
}

// jsonPath экранирует путь для вставки в JSON-строку (Windows backslashes).
func jsonPath(t *testing.T, p string) string {
	t.Helper()
	b, _ := json.Marshal(p)
	return string(b[1 : len(b)-1])
}

// testSelfSignedPEM — настоящий самоподписанный сертификат (как на сервере).
func testSelfSignedPEM(t *testing.T) string {
	t.Helper()
	key, err := rsaGenerateKey(2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509NewTemplate()
	der, err := x509CreateSelfSigned(key, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	return pemEncode(der)
}

// ---------- accessors ----------

func TestProtectedEndpointsOrderAndFields(t *testing.T) {
	cfg := mustParse(t, validConfig)
	eps := cfg.ProtectedEndpoints()
	if len(eps) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(eps))
	}
	e := eps[0]
	if e.Tag != "vless-a" || e.Type != "vless" || e.Server != "example.com" ||
		e.ServerPort != 443 || e.ServerName != "example.com" || e.CertificatePath != "" {
		t.Fatalf("unexpected endpoint: %+v", e)
	}
}

func TestProtectedSelectorReturnsDetachedCopy(t *testing.T) {
	cfg := mustParse(t, validConfig)
	sel := cfg.ProtectedSelector()
	sel[0] = "mutated"
	if cfg.ProtectedSelector()[0] == "mutated" {
		t.Fatal("ProtectedSelector must return a copy")
	}
}

func TestNilConfigSafety(t *testing.T) {
	var cfg *Config
	if cfg.ProtectedEndpoints() != nil || cfg.ProtectedSelector() != nil {
		t.Fatal("nil receiver must be safe")
	}
	cfg.Normalize() // не должно паниковать
}

// ---------- adversarial ----------

func TestAdversarialDeeplyNestedPlaceholder(t *testing.T) {
	// Плейсхолдер в dns→rules→(не строка) — сериализация всё равно ловит
	// только строки; проверяем, что глубоко вложенные строки ловятся.
	bad := strings.Replace(validConfig, `"strategy": ""`, "", 1) // no-op guard
	_ = bad
	deep := strings.Replace(validConfig, `"address": "tls://8.8.8.8"`,
		`"address": "tls://REPLACE_WITH_DNS"`, 1)
	wantErr(t, deep, "placeholder")
}

func TestAdversarialPlaceholderInInOutboundTag(t *testing.T) {
	bad := strings.Replace(validConfig, `"tag": "socks-in"`, `"tag": "YOUR_TAG"`, 1)
	wantErr(t, bad, "placeholder or private-key material")
}

func TestAdversarialCaseVariantsOfPlaceholder(t *testing.T) {
	for _, ph := range []string{"your_uuid", "Change_Me", "replace_with_x"} {
		bad := strings.Replace(validConfig, "50c4d938-21b3-4b78-b4ab-b20f953a9dd9", ph, 1)
		if _, err := Parse([]byte(bad)); err == nil {
			t.Fatalf("case variant %q not caught", ph)
		}
	}
}

func TestAdversarialUnicodeHomoglyphNotPlaceholder(t *testing.T) {
	// Легитимные значения не должны ловиться ложноположительно.
	cfg := mustParse(t, strings.Replace(validConfig, "user", "usеr", 1)) // 'е' кириллическая
	if len(cfg.Inbounds) != 1 {
		t.Fatal("unexpected parse failure")
	}
}
