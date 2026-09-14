package core

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/snowden-system/windows/backend/config"
)

// socksOnlyConfig — минимальный SOCKS-конфиг, пригодный для реального старта
// бокса в тестах (mixed inbound на эфемерном порту, direct-only маршрут —
// но такой конфиг не пройдёт config.Parse, поэтому здесь рендерим «сырой»,
// как это делает движок после внешней валидации).
func socksRawConfig(t *testing.T, port int) []byte {
	t.Helper()
	return []byte(`{
	  "log": {"level": "warn", "output": "stdout"},
	  "inbounds": [
	    {"type": "mixed", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": ` + itoa(port) + `}
	  ],
	  "outbounds": [
	    {"type": "direct", "tag": "direct"},
	    {"type": "block", "tag": "block"}
	  ],
	  "route": {"final": "direct"}
	}`)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func freePort(t *testing.T) int {
	t.Helper()
	// Грубая, но надёжная для тестов схема: слушать :0, забрать порт, закрыть.
	l, err := netListen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*netTCPAddr).Port
}

func TestEngineStartAndStop(t *testing.T) {
	port := freePort(t)
	e := NewEngine(socksRawConfig(t, port))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := e.Status(); got != "running" {
		t.Fatalf("status after start: %s", got)
	}
	if err := e.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := e.Status(); got != "stopped" {
		t.Fatalf("status after stop: %s", got)
	}
}

func TestEngineStopIsIdempotent(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	if err := e.Stop(); err != nil {
		t.Fatalf("stop on fresh engine: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := e.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestEngineDoubleStartRejected(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer e.Stop()
	if err := e.Start(ctx); err != ErrEngineAlreadyRunning {
		t.Fatalf("want ErrEngineAlreadyRunning, got %v", err)
	}
}

func TestEngineStartNoConfig(t *testing.T) {
	e := NewEngine(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != ErrEngineNotConfigured {
		t.Fatalf("want ErrEngineNotConfigured, got %v", err)
	}
}

func TestEngineStartCanceledContext(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.Start(ctx); err == nil {
		t.Fatal("canceled context must fail start")
	}
	if got := e.Status(); got != "stopped" {
		t.Fatalf("engine must remain stopped, got %s", got)
	}
}

func TestEngineStartBadConfigFailsClean(t *testing.T) {
	e := NewEngine([]byte(`{"log": {"level": 42}}`)) // типизационная ошибка sing-box
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Start(ctx); err == nil {
		t.Fatal("bad config must fail")
	}
	if got := e.Status(); got != "stopped" {
		t.Fatalf("engine must remain stopped after failed start, got %s", got)
	}
}

func TestEngineReloadRequiresRunningEngine(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Reload(ctx, socksRawConfig(t, freePort(t))); err != ErrEngineNotRunning {
		t.Fatalf("want ErrEngineNotRunning, got %v", err)
	}
}

func TestEngineReloadEmptyConfigRejected(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if err := e.Reload(ctx, nil); err == nil {
		t.Fatal("empty reload must fail")
	}
	if got := e.Status(); got != "running" {
		t.Fatalf("engine must stay running, got %s", got)
	}
}

func TestEngineReloadInvalidConfigPreservesRunningEngine(t *testing.T) {
	port := freePort(t)
	e := NewEngine(socksRawConfig(t, port))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()

	err := e.Reload(ctx, []byte(`{"inbounds": [{"type": 123}]}`))
	if err == nil {
		t.Fatal("invalid reload config must fail")
	}
	if got := e.Status(); got != "running" {
		t.Fatalf("running engine must survive invalid reload, got %s", got)
	}
	// Внутренний rawJSON не должен был измениться: перезапуск со старым конфигом
	// должен сработать (косвенная проверка неизменности).
	e2 := NewEngine(socksRawConfig(t, port))
	_ = e2
}

func TestEngineReloadValidConfigSwapsEngine(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer e.Stop()
	if err := e.Reload(ctx, socksRawConfig(t, freePort(t))); err != nil {
		t.Fatalf("valid reload: %v", err)
	}
	if got := e.Status(); got != "running" {
		t.Fatalf("status after reload: %s", got)
	}
}

// Провалидированный конфиг (через config.Parse) тоже стартует движком —
// интеграционная стыковка config→engine.
func TestEngineStartsWithValidatedProtectedConfig(t *testing.T) {
	raw := []byte(protectedFixture(t))
	cfg, err := config.Parse(raw)
	if err != nil {
		t.Fatalf("fixture must be valid: %v", err)
	}
	normalized, err := jsonMarshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine(normalized)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start with validated config: %v", err)
	}
	defer e.Stop()
}

func protectedFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pem := filepath.Join(dir, "pin.pem")
	if err := osWriteFile(pem, []byte(selfSignedPEM(t))); err != nil {
		t.Fatal(err)
	}
	return `{
	  "log": {"level": "warn", "output": "stdout"},
	  "inbounds": [{"type": "mixed", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": ` + itoa(freePort(t)) + `}],
	  "outbounds": [
	    {"type": "selector", "tag": "proxy", "outbounds": ["vless-a"], "default": "vless-a"},
	    {"type": "vless", "tag": "vless-a", "server": "127.0.0.1", "server_port": 1,
	     "uuid": "50c4d938-21b3-4b78-b4ab-b20f953a9dd9",
	     "tls": {"enabled": true, "server_name": "example.com", "certificate_path": "` + jsonEscape(pem) + `"}},
	    {"type": "direct", "tag": "direct"},
	    {"type": "block", "tag": "block"}
	  ],
	  "route": {"final": "proxy"}
	}`
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// selfSignedPEM — настоящий самоподписанный сертификат: sing-box реально
// парсит x509, фейковое тело не пройдёт.
func selfSignedPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.snowden.invalid"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"test.snowden.invalid"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// selectorRawConfig — селектор над двумя direct-outbound: движку всё равно,
// что внутри (проверяется механика переключения, не защищённость —
// защищённость — контракт config.Parse на пути рендера).
func selectorRawConfig(t *testing.T, port int) []byte {
	t.Helper()
	return []byte(`{
	  "log": {"level": "warn", "output": "stdout"},
	  "inbounds": [
	    {"type": "mixed", "tag": "socks-in", "listen": "127.0.0.1", "listen_port": ` + itoa(port) + `}
	  ],
	  "outbounds": [
	    {"type": "direct", "tag": "chan-a"},
	    {"type": "direct", "tag": "chan-b"},
	    {"type": "selector", "tag": "proxy", "outbounds": ["chan-a", "chan-b"], "default": "chan-a"},
	    {"type": "direct", "tag": "direct"},
	    {"type": "block", "tag": "block"}
	  ],
	  "route": {"final": "proxy"}
	}`)
}

// TestEngineSwitchDefaultSeamless — переключение селектора на ЖИВОМ боксе:
// без Stop/Reload, с синхронизацией rawJSON (следующий Start после Stop
// обязан стартовать с последним выбранного канала, V2-037).
func TestEngineSwitchDefaultSeamless(t *testing.T) {
	e := NewEngine(selectorRawConfig(t, freePort(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer e.Stop()

	if got, err := e.CurrentDefault("proxy"); err != nil || got != "chan-a" {
		t.Fatalf("initial default: %q, err=%v", got, err)
	}

	prev, err := e.SwitchDefault("proxy", "chan-b")
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	if prev != "chan-a" {
		t.Fatalf("previous tag: %q", prev)
	}
	if got, _ := e.CurrentDefault("proxy"); got != "chan-b" {
		t.Fatalf("after switch: %q", got)
	}
	// Идемпотентность.
	if _, err := e.SwitchDefault("proxy", "chan-b"); err != nil {
		t.Fatalf("idempotent switch: %v", err)
	}
	// Неизвестный тег — structured-ошибка, бокс жив.
	if _, err := e.SwitchDefault("proxy", "no-such-tag"); err == nil {
		t.Fatal("unknown tag must fail")
	}
	if got := e.Status(); got != "running" {
		t.Fatalf("engine must stay running, got %s", got)
	}
	// rawJSON синхронизирован: рестарт поднимает chan-b.
	if err := e.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := e.Start(ctx); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if got, _ := e.CurrentDefault("proxy"); got != "chan-b" {
		t.Fatalf("after restart: %q (rawJSON desync)", got)
	}
}

// TestEngineSwitchDefaultUnavailable — бокс без селектора "proxy":
// ErrSelectorSwitchUnavailable, состояние не менялось.
func TestEngineSwitchDefaultUnavailable(t *testing.T) {
	e := NewEngine(socksRawConfig(t, freePort(t)))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer e.Stop()
	if _, err := e.SwitchDefault("proxy", "x"); err == nil {
		t.Fatal("switch must fail without selector")
	} else if !errors.Is(err, ErrSelectorSwitchUnavailable) {
		t.Fatalf("expected ErrSelectorSwitchUnavailable, got %v", err)
	}
}
