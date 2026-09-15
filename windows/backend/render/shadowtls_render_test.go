package render

// shadowtls_render_test.go — тесты канала D (V2-051): валидация дескриптора,
// двухзвенный рендер (vless с detour → shadowtls), строгий парсер ядра,
// отказ при неполных ролях.

import (
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/config"
)

// shadowTLSChannel — корректный дескриптор канала D для тестов.
func shadowTLSChannel() ChannelDescriptor {
	return ChannelDescriptor{
		ID: "channel-a-shadowtls", Protocol: "vless", Transport: "shadowtls",
		Hostname: "www.samsung.com", Port: 8445, OriginServer: "203.0.113.10",
		ExpectedEgress: "203.0.113.10",
		CredentialRefs: []string{"vless-uuid-d", "shadowtls-password-d"},
		UUIDRef:        "vless-uuid-d", ShadowTLSPasswordRef: "shadowtls-password-d",
		ValidationStatus: "configured", Enabled: true,
	}
}

// shadowTLSSecrets — секреты с валидными значениями канала D.
func shadowTLSSecrets() map[string]string {
	return map[string]string{
		"vless-uuid-d":         "00000000-0000-4000-8000-00000000000d",
		"shadowtls-password-d": "FsAQBjjhfc4q0ShK3w2xzg==", // base64(16 байт)
	}
}

func TestShadowTLSDescriptorValid(t *testing.T) {
	ch := shadowTLSChannel()
	if err := validateChannel(ch); err != nil {
		t.Fatalf("valid shadowtls descriptor rejected: %v", err)
	}
	// Неполные роли: только пароль без uuid_ref.
	bad := ch
	bad.UUIDRef = ""
	if err := validateChannel(bad); err == nil {
		t.Fatal("descriptor without uuid_ref accepted")
	}
	// Роль не в credential_refs.
	bad2 := ch
	bad2.CredentialRefs = []string{"vless-uuid-d"}
	if err := validateChannel(bad2); err == nil {
		t.Fatal("descriptor with missing password ref in credential_refs accepted")
	}
	// Лишняя третья ссылка.
	bad3 := ch
	bad3.CredentialRefs = []string{"vless-uuid-d", "shadowtls-password-d", "hy2-password"}
	if err := validateChannel(bad3); err == nil {
		t.Fatal("descriptor with extra credential ref accepted")
	}
	// Одинаковые роли.
	bad4 := ch
	bad4.ShadowTLSPasswordRef = "vless-uuid-d"
	if err := validateChannel(bad4); err == nil {
		t.Fatal("descriptor with duplicate role refs accepted")
	}
	// tcp-reality с shadowtls_password_ref — отказ (перепутанные поля).
	bad5 := shadowTLSChannel()
	bad5.Transport = "tcp-reality"
	if err := validateChannel(bad5); err == nil {
		t.Fatal("tcp-reality with shadowtls_password_ref accepted (roles confused)")
	}
}

func TestShadowTLSRenderChain(t *testing.T) {
	splitSet = nil
	defer func() { splitSet = nil }()
	ch := shadowTLSChannel()
	src := &fakeSecrets{values: shadowTLSSecrets()}
	cc, err := RenderFrom([]ChannelDescriptor{ch}, src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	raw := string(cc.Raw)
	// Звено 1: shadowtls v3 с паролем и маскировочным TLS к донору.
	if !strings.Contains(raw, `"type": "shadowtls"`) && !strings.Contains(raw, `"type":"shadowtls"`) {
		t.Fatal("shadowtls outbound not rendered")
	}
	if !strings.Contains(raw, `"version": 3`) && !strings.Contains(raw, `"version":3`) {
		t.Fatal("shadowtls version 3 not rendered")
	}
	if !strings.Contains(raw, "www.samsung.com") {
		t.Fatal("shadowtls handshake SNI (donor) missing")
	}
	// Звено 2: vless с detour на shadowtls-звено и loopback-сервером.
	if !strings.Contains(raw, `"detour": "channel-a-shadowtls-st"`) &&
		!strings.Contains(raw, `"detour":"channel-a-shadowtls-st"`) {
		t.Fatal("vless link does not detour through shadowtls link")
	}
	if !strings.Contains(raw, `"server": "127.0.0.1"`) && !strings.Contains(raw, `"server":"127.0.0.1"`) {
		t.Fatal("vless inner link must target server loopback")
	}
	// Селектор: vless-звено присутствует (shadowtls-звено не выбирается напрямую).
	if !strings.Contains(raw, `"channel-a-shadowtls"`) {
		t.Fatal("vless link tag missing")
	}
}

func TestShadowTLSRenderRejectsBadPassword(t *testing.T) {
	splitSet = nil
	defer func() { splitSet = nil }()
	ch := shadowTLSChannel()
	secrets := shadowTLSSecrets()
	secrets["shadowtls-password-d"] = "short" // не base64 24 симв.
	src := &fakeSecrets{values: secrets}
	if _, err := RenderFrom([]ChannelDescriptor{ch}, src); err == nil {
		t.Fatal("render accepted non-24-char shadowtls password")
	}
}

func TestShadowTLSConfigGateAcceptsChain(t *testing.T) {
	// Полный конфиг с цепочкой проходит строгий валидатор конфига
	// (тот же код, что защищает селектор — validateSelector и пр.).
	ch := shadowTLSChannel()
	src := &fakeSecrets{values: shadowTLSSecrets()}
	cc, err := RenderFrom([]ChannelDescriptor{ch}, src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatalf("strict config gate rejected shadowtls chain: %v", err)
	}
}
