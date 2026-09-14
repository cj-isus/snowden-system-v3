package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/config"
)

// fakeSecrets — хранилище значений + опциональный pin-путь.
type fakeSecrets struct {
	values map[string]string
	pins   map[string]string
}

func (f *fakeSecrets) Get(ref string) (string, error) {
	if v, ok := f.values[ref]; ok {
		return v, nil
	}
	return "", os.ErrNotExist
}
func (f *fakeSecrets) PinCertPath(channelID string) string { return f.pins[channelID] }

func testSecrets(withPin bool) *fakeSecrets {
	s := &fakeSecrets{
		values: map[string]string{
			"vless-uuid":        "50c4d938-21b3-4b78-b4ab-b20f953a9dd9",
			"hy2-password":      "a9383b3b87bfeae0a9383b3b87bfeae0",
			"hy2-obfs-password": "236124c16317a5b7236124c16317a5b7",
			// Канал C (REALITY, V2-044): тестовые значения того же формата,
			// что требует валидатор (X25519 base64url 43 симв., hex short_id).
			"vless-uuid-c":         "00000000-0000-4000-8000-00000000000c",
			"reality-public-key-c": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			"reality-short-id-c":   "0123456c",
		},
		pins: map[string]string{},
	}
	if withPin {
		s.pins["channel-a-hy2"] = writePinFile()
	}
	return s
}

var pinPEM string

func writePinFile() string {
	if pinPEM == "" {
		key, err := rsaGen()
		if err != nil {
			panic(err)
		}
		pinPEM = pemOf(key)
	}
	dir, err := os.MkdirTemp("", "pin")
	if err != nil {
		panic(err)
	}
	p := filepath.Join(dir, "pin.pem")
	if err := os.WriteFile(p, []byte(pinPEM), 0o600); err != nil {
		panic(err)
	}
	return p
}

// ---------- descriptors ----------

func TestLoadDescriptorsValid(t *testing.T) {
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatalf("descriptors must be valid: %v", err)
	}
	if len(channels) != 3 {
		t.Fatalf("want 3 channels (V2-044: +REALITY), got %d", len(channels))
	}
	byID := map[string]ChannelDescriptor{}
	for _, ch := range channels {
		byID[ch.ID] = ch
	}
	a, ok := byID["channel-a-vless"]
	if !ok || a.ValidationStatus != "live-verified" || !a.Enabled {
		t.Fatalf("channel A must be live-verified and enabled: %+v", a)
	}
	h, ok := byID["channel-a-hy2"]
	if !ok || h.ValidationStatus != "live-verified" || !h.Enabled {
		// A1.3: HY2 live-verified 2026-09-09 (probe через outbound/hysteria2,
		// egress = expected). До этого был configured с допуском-исключением.
		t.Fatalf("channel HY2 must be live-verified+enabled: %+v", h)
	}
}

// ---------- UDP gate / selector ----------

func TestSelectableExcludesUnverified(t *testing.T) {
	channels, _ := LoadDescriptors()
	sel := Selectable(channels)
	// VLESS — live-verified; HY2 — live-verified (A1.3); REALITY — configured+enabled
	// (допуск B1: селектор собирает и configured, probe при переключении — гейт).
	if len(sel) != 3 {
		t.Fatalf("vless + hy2 + reality must be selectable, got %+v", sel)
	}
	if sel[0].ID != "channel-a-vless" {
		t.Fatalf("first candidate must be vless (default), got %+v", sel[0].ID)
	}
}

// ---------- render ----------

func TestRenderVlessOnlyPassesStrictValidation(t *testing.T) {
	// Default-набор без pin → HY2 рендер отклонить не может (он selectable в
	// configured), поэтому тест гоняет набор ТОЛЬКО с VLESS-каналом.
	channels, _ := LoadDescriptors()
	vlessOnly := channels[:1]
	cc, err := RenderFrom(vlessOnly, testSecrets(false))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	cfg, err := config.Parse(cc.Raw)
	if err != nil {
		t.Fatalf("rendered config must pass strict parse: %v", err)
	}
	if got := cfg.ProtectedSelector(); len(got) != 1 || got[0] != "channel-a-vless" {
		t.Fatalf("selector must contain only verified channel: %v", got)
	}
	if cc.ExpectedEgress != "203.0.113.10" {
		t.Fatalf("expected egress from descriptor: %q", cc.ExpectedEgress)
	}
	if cc.ChannelIDs["channel-a-vless"] != "channel-a-vless" {
		t.Fatalf("tag mapping broken: %v", cc.ChannelIDs)
	}
}

func TestRenderObfsPasswordNotConfusedWithPassword(t *testing.T) {
	// Регрессия нечёткого матчинга: password и obfs оба содержат «hy2».
	channels, _ := LoadDescriptors()
	for i := range channels {
		if channels[i].ID == "channel-a-hy2" {
			channels[i].ValidationStatus = "live-verified"
			channels[i].Enabled = true
		}
	}
	cc, err := RenderFrom(channels, testSecrets(true))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(cc.Raw)
	if strings.Contains(raw, "a9383b3b87bfeae0a9383b3b87bfeae0") {
		// пароль встречается ровно один раз (users-слот hysteria2), а не дважды
		count := strings.Count(raw, "a9383b3b87bfeae0a9383b3b87bfeae0")
		if count != 1 {
			t.Fatalf("hy2 password must appear exactly once, got %d", count)
		}
	}
	if !strings.Contains(raw, "236124c16317a5b7236124c16317a5b7") {
		t.Fatal("obfs password must be wired into obfs section")
	}
}

func TestRenderMissingDeclaredRefFails(t *testing.T) {
	channels, _ := LoadDescriptors()
	for i := range channels {
		if channels[i].ID == "channel-a-vless" {
			channels[i].CredentialRefs = []string{"wrong-ref"}
		}
	}
	_, err := RenderFrom(channels, testSecrets(false))
	if err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("undeclared ref must fail with precise error, got %v", err)
	}
}

func TestRenderSecretsWiredIntoConfig(t *testing.T) {
	channels, _ := LoadDescriptors()
	cc, err := RenderFrom(channels[:1], testSecrets(false))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(cc.Raw)
	if !strings.Contains(raw, "50c4d938-21b3-4b78-b4ab-b20f953a9dd9") {
		t.Fatal("uuid must be wired from secrets source")
	}
	if strings.Contains(raw, "REPLACE_WITH") || strings.Contains(raw, "{{") {
		t.Fatal("no placeholders may survive rendering")
	}
}

func TestRenderDNSGoesThroughTunnel(t *testing.T) {
	channels, _ := LoadDescriptors()
	cc, err := RenderFrom(channels[:1], testSecrets(false))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(cc.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DNS == nil || len(cfg.DNS.Servers) == 0 {
		t.Fatal("dns section required")
	}
	for _, srv := range cfg.DNS.Servers {
		if srv.Tag == "tunnel-dns" && srv.Detour != "proxy" {
			t.Fatalf("main resolver must go through the tunnel: %+v", srv)
		}
	}
	// Правило по умолчанию (без domain) должно указывать на tunnel-dns.
	hasDefault := false
	for _, rule := range cfg.DNS.Rules {
		if rule.Server == "tunnel-dns" && len(rule.Domain) == 0 {
			hasDefault = true
		}
	}
	if !hasDefault {
		t.Fatal("default dns rule must use the tunnel resolver")
	}
}

func TestRenderHY2WithoutPinRefused(t *testing.T) {
	// HY2 enabled+live-verified, но pin не задан → рендер обязан отказаться.
	channels, _ := LoadDescriptors()
	for i := range channels {
		if channels[i].ID == "channel-a-hy2" {
			channels[i].ValidationStatus = "live-verified"
			channels[i].Enabled = true
		}
	}
	_, err := RenderFrom(channels, testSecrets(false))
	if err == nil || !strings.Contains(err.Error(), "pinned certificate") {
		t.Fatalf("HY2 without pin must be refused, got %v", err)
	}
}

func TestRenderHY2WithPinAndVerifiedEnters(t *testing.T) {
	channels, _ := LoadDescriptors()
	for i := range channels {
		if channels[i].ID == "channel-a-hy2" {
			channels[i].ValidationStatus = "live-verified"
			channels[i].Enabled = true
		}
	}
	cc, err := RenderFrom(channels, testSecrets(true))
	if err != nil {
		t.Fatalf("HY2 with pin must render: %v", err)
	}
	cfg, err := config.Parse(cc.Raw)
	if err != nil {
		t.Fatal(err)
	}
	sel := cfg.ProtectedSelector()
	if len(sel) != 3 || sel[0] != "channel-a-vless" || sel[1] != "channel-a-hy2" || sel[2] != "channel-a-reality" {
		t.Fatalf("selector must be [vless hy2 reality], got %v", sel)
	}
	// Валидированный конфиг должен пройти и через sing-box-уровень типов
	// (config.Parse уже проверил структуру; здесь важен порядок кандидатов).
}

func TestRenderNoVerifiedChannelsFailsClosed(t *testing.T) {
	channels, _ := LoadDescriptors()
	for i := range channels {
		channels[i].ValidationStatus = "configured"
		channels[i].Enabled = false // не HY2 (его исключение — в Selectable-допуске), а generic-набор
	}
	_, err := RenderFrom(channels, testSecrets(false))
	if err == nil || !strings.Contains(err.Error(), "fail-closed") {
		t.Fatalf("no verified channels must fail closed, got %v", err)
	}
	if sel := Selectable(nil); len(sel) != 0 {
		t.Fatal("empty descriptor set must yield empty selection")
	}
}

// ---------- A1.4: default канала в селекторе ----------

func TestRenderSelectorDefaultHonored(t *testing.T) {
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatalf("descriptors: %v", err)
	}
	// Делаем оба канала live-verified и включёнными (иначе UDP-gate честно
	// исключит HY2 с enabled=false — это отдельное контрактное поведение).
	for i := range channels {
		ch := channels[i]
		ch.ValidationStatus = "live-verified"
		ch.Enabled = true
		channels[i] = ch
	}
	cc, err := RenderFrom(channels, testSecrets(true), WithDefaultChannel("channel-a-hy2"))
	if err != nil {
		t.Fatalf("render with hy2 default: %v", err)
	}
	cfg, err := config.Parse(cc.Raw)
	if err != nil {
		t.Fatalf("strict parse: %v", err)
	}
	// Дефолт должен читаться из собранного конфига, а не из наших надежд.
	found := false
	for _, ob := range cfg.Outbounds {
		if ob.Tag == "proxy" && ob.Default == "channel-a-hy2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("selector default must be channel-a-hy2")
	}
	if cc.SelectorDefault != "channel-a-hy2" {
		t.Fatalf("ClientConfig.SelectorDefault = %q, want channel-a-hy2", cc.SelectorDefault)
	}
}

func TestRenderSelectorDefaultUnknownFallsBackToFirst(t *testing.T) {
	channels, _ := LoadDescriptors()
	cc, err := RenderFrom(channels, testSecrets(true), WithDefaultChannel("channel-a-hy2-invalid-name"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if cc.SelectorDefault != "channel-a-vless" {
		t.Fatalf("unknown default must fall back to first candidate, got %q", cc.SelectorDefault)
	}
}

// ---------- B1: schema 2 / VLESS+TCP+REALITY (V2-032) ----------

// realityTestSet — набор schema 2: канал A (vless+ws, live-verified) + канал B
// (vless+tcp-reality). statusB задаёт статус канала B ('lv' | 'cfg' | 'planned').
func realityTestSet(t *testing.T, statusB string) ([]ChannelDescriptor, *fakeSecrets) {
	t.Helper()
	chans := []ChannelDescriptor{
		{
			ID: "channel-a-vless", Protocol: "vless", Transport: "ws",
			Hostname: "snowden.example", Port: 443,
			OriginServer: "203.0.113.10", ExpectedEgress: "203.0.113.10",
			CredentialRefs:   []string{"vless-uuid"},
			ValidationStatus: "live-verified", Enabled: true,
		},
		{
			ID: "channel-b-reality", Protocol: "vless", Transport: "tcp-reality",
			Hostname: "www.donor.example", Port: 443,
			OriginServer: "198.51.100.77", ExpectedEgress: "198.51.100.77",
			CredentialRefs:      []string{"vless-uuid-b", "reality-public-key-b", "reality-short-id-b"},
			UUIDRef:             "vless-uuid-b",
			RealityPublicKeyRef: "reality-public-key-b",
			RealityShortIDRef:   "reality-short-id-b",
			ValidationStatus:    statusB, Enabled: true,
		},
	}
	secrets := &fakeSecrets{
		values: map[string]string{
			"vless-uuid":           "50c4d938-21b3-4b78-b4ab-b20f953a9dd9",
			"vless-uuid-b":         "6e1e8c62-16af-4ad9-9c92-4d1e67a1b1a2",
			"reality-public-key-b": "jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0",
			"reality-short-id-b":   "0123456789abcdef",
		},
		pins: map[string]string{},
	}
	return chans, secrets
}

func outboundTLSOf(t *testing.T, raw []byte, tag string) map[string]any {
	t.Helper()
	var parsed struct {
		Outbounds []struct {
			Tag string         `json:"tag"`
			TLS map[string]any `json:"tls"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal rendered config: %v", err)
	}
	for _, ob := range parsed.Outbounds {
		if ob.Tag == tag {
			return ob.TLS
		}
	}
	t.Fatalf("outbound %q not found in rendered config", tag)
	return nil
}

func TestParseDescriptorsSchema2RealityValid(t *testing.T) {
	chans, _ := realityTestSet(t, "live-verified")
	data, _ := json.Marshal(descriptorSet{Schema: 2, Channels: chans})
	got, err := parseDescriptors(data)
	if err != nil {
		t.Fatalf("schema 2 set must parse: %v", err)
	}
	if len(got) != 2 || got[1].UUIDRef != "vless-uuid-b" {
		t.Fatalf("parsed descriptors wrong: %+v", got)
	}
}

func TestParseDescriptorsSchema1RejectsReality(t *testing.T) {
	chans, _ := realityTestSet(t, "live-verified")
	data, _ := json.Marshal(descriptorSet{Schema: 1, Channels: chans})
	if _, err := parseDescriptors(data); err == nil || !strings.Contains(err.Error(), "schema 2") {
		t.Fatalf("tcp-reality in schema 1 must be rejected: %v", err)
	}
}

func TestParseDescriptorsRealityRefsMismatchFails(t *testing.T) {
	chans, _ := realityTestSet(t, "live-verified")
	chans[1].RealityPublicKeyRef = "reality-public-key-NOT-DECLARED"
	data, _ := json.Marshal(descriptorSet{Schema: 2, Channels: chans})
	if _, err := parseDescriptors(data); err == nil {
		t.Fatal("role ref not in credential_refs must fail parse")
	}
	// Дубль роли: два поля ссылаются на одну ссылку — подмена значения.
	chans, _ = realityTestSet(t, "live-verified")
	chans[1].RealityShortIDRef = chans[1].UUIDRef
	data, _ = json.Marshal(descriptorSet{Schema: 2, Channels: chans})
	if _, err := parseDescriptors(data); err == nil {
		t.Fatal("duplicate role refs must fail parse")
	}
}

func TestRenderRealityOutboundShape(t *testing.T) {
	chans, secrets := realityTestSet(t, "live-verified")
	cc, err := RenderFrom(chans, secrets, WithDefaultChannel("channel-b-reality"))
	if err != nil {
		t.Fatalf("render reality channel: %v", err)
	}
	// Обязан проходить строгий парсер (render делает это сам, но проверяем явно).
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatalf("strict parse: %v", err)
	}
	tls := outboundTLSOf(t, cc.Raw, "channel-b-reality")
	if tls["server_name"] != "www.donor.example" {
		t.Fatalf("reality SNI must be donor host, got %v", tls["server_name"])
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok {
		t.Fatalf("tls.reality missing: %v", tls)
	}
	if reality["enabled"] != true || reality["public_key"] == "" || reality["short_id"] == "" {
		t.Fatalf("reality block incomplete: %v", reality)
	}
	utls, ok := tls["utls"].(map[string]any)
	if !ok || utls["enabled"] != true {
		t.Fatalf("reality requires utls (pinned core): %v", tls)
	}
	// На REALITY-outbound НЕ должно быть transport-блока (raw TCP).
	var parsed struct {
		Outbounds []struct {
			Tag       string         `json:"tag"`
			Transport map[string]any `json:"transport"`
		} `json:"outbounds"`
	}
	_ = json.Unmarshal(cc.Raw, &parsed)
	for _, ob := range parsed.Outbounds {
		if ob.Tag == "channel-b-reality" && ob.Transport != nil {
			t.Fatalf("tcp-reality must render without transport block: %v", ob.Transport)
		}
	}
	// Vision не задаётся (нет в ядре).
	if strings.Contains(string(cc.Raw), "xtls-rprx-vision") {
		t.Fatal("vision flow must not appear in rendered config")
	}
	// Egress следует за выбранным каналом (другой VPS).
	if cc.ExpectedEgress != "198.51.100.77" {
		t.Fatalf("expected egress must follow selected channel, got %q", cc.ExpectedEgress)
	}
}

func TestRenderRealityConfiguredNotDefaultButSelectable(t *testing.T) {
	// Регрессия семантики A1.3: раньше channel-a-hy2 был захардкожен в
	// Selectable. Теперь любой configured канал selectable (иначе его нельзя
	// live-верифицировать), но default — только live-verified.
	chans, secrets := realityTestSet(t, "configured")
	cc, err := RenderFrom(chans, secrets)
	if err != nil {
		t.Fatalf("render with configured reality: %v", err)
	}
	if cc.SelectorDefault != "channel-a-vless" {
		t.Fatalf("default must be live-verified channel, got %q", cc.SelectorDefault)
	}
	if len(cc.SelectorTags) != 2 {
		t.Fatalf("configured channel must be selectable for explicit switch: %v", cc.SelectorTags)
	}
	if cc.ExpectedEgress != "203.0.113.10" {
		t.Fatalf("egress expectation must follow the live-verified default, got %q", cc.ExpectedEgress)
	}
}

func TestRenderRealityBadKeyFailsClosed(t *testing.T) {
	chans, secrets := realityTestSet(t, "live-verified")
	secrets.values["reality-public-key-b"] = "not-a-base64url-x25519-key!!"
	if _, err := RenderFrom(chans, secrets, WithDefaultChannel("channel-b-reality")); err == nil {
		t.Fatal("invalid reality public key must fail render")
	}
	chans, secrets = realityTestSet(t, "live-verified")
	secrets.values["reality-short-id-b"] = "zzzz" // не hex
	if _, err := RenderFrom(chans, secrets, WithDefaultChannel("channel-b-reality")); err == nil {
		t.Fatal("non-hex short_id must fail render")
	}
}

func TestRenderRealityMissingSecretFails(t *testing.T) {
	chans, secrets := realityTestSet(t, "live-verified")
	delete(secrets.values, "reality-public-key-b")
	if _, err := RenderFrom(chans, secrets); err == nil {
		t.Fatal("missing reality key must fail render (fail-closed)")
	}
}

// ---------- A1.2: TUN-inbound ----------

func TestRenderTUNInboundAdded(t *testing.T) {
	channels, _ := LoadDescriptors()
	// HY2 теперь selectable в configured — для рендера нужен pin (реальный путь).
	cc, err := RenderFrom(channels, testSecrets(true), WithTUN())
	if err != nil {
		t.Fatalf("render with TUN: %v", err)
	}
	cfg, err := config.Parse(cc.Raw)
	if err != nil {
		t.Fatalf("strict parse: %v", err)
	}
	foundTUN, foundSocks := false, false
	for _, in := range cfg.Inbounds {
		switch in.Type {
		case "tun":
			foundTUN = true
			if !in.AutoRoute || !in.StrictRoute {
				t.Fatal("tun inbound must have auto_route+strict_route (leak protection)")
			}
		case "mixed":
			foundSocks = true
		}
	}
	if !foundTUN || !foundSocks {
		t.Fatalf("expected both tun and mixed inbounds, got tun=%v mixed=%v", foundTUN, foundSocks)
	}
}

// ---------- V2-037: IP-диал, mux, hopping/brutal, probe-цели ----------

func outboundOf(t *testing.T, raw []byte, tag string) config.Outbound {
	t.Helper()
	cfg, err := config.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, ob := range cfg.Outbounds {
		if ob.Tag == tag {
			return ob
		}
	}
	t.Fatalf("outbound %q not found", tag)
	return config.Outbound{}
}

func TestRenderDialOverrideUsesIPWithDomainSNIHost(t *testing.T) {
	channels, _ := LoadDescriptors()
	cc, err := RenderFrom(channels[:1], testSecrets(false),
		WithDialOverrides(map[string]string{"vpn.example.com": "104.21.5.10"}))
	if err != nil {
		t.Fatal(err)
	}
	ob := outboundOf(t, cc.Raw, "channel-a-vless")
	if ob.Server != "104.21.5.10" {
		t.Fatalf("dial must use resolved IP, got %q", ob.Server)
	}
	if ob.TLS["server_name"] != "vpn.example.com" {
		t.Fatalf("SNI must stay domain: %v", ob.TLS["server_name"])
	}
	if ob.Transport == nil || ob.Transport.Headers["Host"] != "vpn.example.com" {
		t.Fatalf("ws Host header must stay domain: %+v", ob.Transport)
	}
}

func TestRenderWithoutOverrideKeepsHostname(t *testing.T) {
	channels, _ := LoadDescriptors()
	cc, err := RenderFrom(channels[:1], testSecrets(false))
	if err != nil {
		t.Fatal(err)
	}
	ob := outboundOf(t, cc.Raw, "channel-a-vless")
	if ob.Server != "vpn.example.com" {
		t.Fatalf("server must stay hostname without override: %q", ob.Server)
	}
	if ob.Transport != nil && ob.Transport.Headers != nil {
		t.Fatalf("no Host header needed when dialing by name: %+v", ob.Transport.Headers)
	}
}

// TestRenderVLESSMultiplexExplicitOnly — mux рендерится ТОЛЬКО по явному
// multiplex:true дескриптора (V2-038): серверный inbound обязан уметь
// терминировать mux (live-инцидент V2-037 — EOF на живом CDN при
// глобальном дефолте), envelope-канал не может молча унаследовать фичу,
// которую его сервер не обещал.
func TestRenderVLESSMultiplexExplicitOnly(t *testing.T) {
	channels, _ := LoadDescriptors()
	// Встроенный дескриптор канала A задаёт multiplex:true (сервер демультиплексирует).
	cc, err := RenderFrom(channels[:1], testSecrets(false))
	if err != nil {
		t.Fatal(err)
	}
	ob := outboundOf(t, cc.Raw, "channel-a-vless")
	if ob.Multiplex == nil || !ob.Multiplex.Enabled || ob.Multiplex.Protocol != "h2mux" || !ob.Multiplex.Padding {
		t.Fatalf("descriptor multiplex:true must render h2mux+padding: %+v", ob.Multiplex)
	}
	// Отключение по дескриптору.
	off := false
	channels[0].Multiplex = &off
	cc, err = RenderFrom(channels[:1], testSecrets(false))
	if err != nil {
		t.Fatal(err)
	}
	if ob := outboundOf(t, cc.Raw, "channel-a-vless"); ob.Multiplex != nil {
		t.Fatalf("multiplex must be disabled by descriptor: %+v", ob.Multiplex)
	}
	// Отсутствие поля = mux НЕТ (не «унаследовал дефолт»): envelope-канал
	// без явного согласия сервера остаётся не-mux.
	channels[0].Multiplex = nil
	cc, err = RenderFrom(channels[:1], testSecrets(false))
	if err != nil {
		t.Fatal(err)
	}
	if ob := outboundOf(t, cc.Raw, "channel-a-vless"); ob.Multiplex != nil {
		t.Fatalf("absent multiplex field must NOT enable mux (both-ends contract): %+v", ob.Multiplex)
	}
}

func TestRenderHY2HopAndBrutalWired(t *testing.T) {
	channels, _ := LoadDescriptors()
	hy2 := channels[1]
	hy2.ServerPorts = []string{"40000:41000"}
	hy2.HopInterval = "30s"
	hy2.UpMbps = 50
	hy2.DownMbps = 200
	cc, err := RenderFrom([]ChannelDescriptor{hy2}, testSecrets(true))
	if err != nil {
		t.Fatal(err)
	}
	ob := outboundOf(t, cc.Raw, "channel-a-hy2")
	if len(ob.ServerPorts) != 1 || ob.ServerPorts[0] != "40000:41000" || ob.HopInterval != "30s" {
		t.Fatalf("hop fields not wired: %+v", ob)
	}
	if ob.UpMbps != 50 || ob.DownMbps != 200 {
		t.Fatalf("brutal fields not wired: %+v", ob)
	}
}

func TestProbeTargetsFromDescriptorAndFallback(t *testing.T) {
	channels, _ := LoadDescriptors()
	channels[0].ProbeTargets = []string{"https://vpn.example.com/ip", "https://api.ipify.org"}
	cc, err := RenderFrom(channels, testSecrets(true)) // default = vless (live-verified первый)
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.ProbeTargets) != 2 || cc.ProbeTargets[0] != "https://vpn.example.com/ip" {
		t.Fatalf("probe targets must follow default channel descriptor: %v", cc.ProbeTargets)
	}
	// Без поля — дефолтная пара.
	channels[0].ProbeTargets = nil
	cc, err = RenderFrom(channels, testSecrets(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(cc.ProbeTargets) != 2 || cc.ProbeTargets[0] != DefaultProbeTargets[0] {
		t.Fatalf("fallback targets expected: %v", cc.ProbeTargets)
	}
}

func TestValidateChannelExtrasRejectsBadShapes(t *testing.T) {
	base := func() ChannelDescriptor {
		ch, _ := LoadDescriptors()
		return ch[0]
	}
	cases := []struct {
		name string
		mut  func(*ChannelDescriptor)
	}{
		{"single probe target", func(c *ChannelDescriptor) { c.ProbeTargets = []string{"https://a.example"} }},
		{"http probe target", func(c *ChannelDescriptor) { c.ProbeTargets = []string{"http://a.example", "https://b.example"} }},
		{"hop on vless", func(c *ChannelDescriptor) { c.ServerPorts = []string{"40000:41000"} }},
		{"bad hop range", func(c *ChannelDescriptor) {
			c.Protocol = "hysteria2"
			c.Transport = "quic"
			c.ServerPorts = []string{"50000:40000"}
		}},
		{"hop interval on vless", func(c *ChannelDescriptor) { c.HopInterval = "30s" }},
		{"brutal half-set", func(c *ChannelDescriptor) { c.Protocol = "hysteria2"; c.Transport = "quic"; c.UpMbps = 10 }},
		{"brutal out of range", func(c *ChannelDescriptor) {
			c.Protocol = "hysteria2"
			c.Transport = "quic"
			c.UpMbps = 10
			c.DownMbps = 99999
		}},
	}
	for _, tc := range cases {
		ch := base()
		tc.mut(&ch)
		if err := validateChannel(ch); err == nil {
			t.Fatalf("%s: must be rejected", tc.name)
		}
	}
}
