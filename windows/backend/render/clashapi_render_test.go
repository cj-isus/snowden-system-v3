package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/snowden-system/windows/backend/config"
)

// TestClashAPINotRenderedByDefault — без опции блока нет (обратная совместимость:
// старые сборки ядра без with_clash_api не должны получать experimental вообще).
func TestClashAPINotRenderedByDefault(t *testing.T) {
	cc, err := RenderClientConfig(testSecrets(true))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(cc.Raw), "clash_api") {
		t.Fatalf("clash_api must not be rendered without the opt-in option:\n%s", cc.Raw)
	}
}

// TestClashAPIRenderedWithOption — с WithClashAPI блок рендерится на строгом
// loopback с секретом, и полученный конфиг по-прежнему проходит строгий парсер
// ядра (гейт V2-046 — класс «наши типы ≠ типы ядра»).
func TestClashAPIRenderedWithOption(t *testing.T) {
	cc, err := RenderFrom(mustChannels(t), testSecrets(true), WithClashAPI(34567, "sess-secret"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var raw struct {
		Experimental *struct {
			ClashAPI *struct {
				ExternalController string `json:"external_controller"`
				Secret             string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(cc.Raw, &raw); err != nil {
		t.Fatalf("raw config is not JSON: %v", err)
	}
	if raw.Experimental == nil || raw.Experimental.ClashAPI == nil {
		t.Fatalf("clash_api block missing:\n%s", cc.Raw)
	}
	if got := raw.Experimental.ClashAPI.ExternalController; got != "127.0.0.1:34567" {
		t.Fatalf("external_controller = %q, want strict loopback", got)
	}
	if raw.Experimental.ClashAPI.Secret != "sess-secret" {
		t.Fatalf("secret not rendered")
	}
	if _, err := config.Parse(cc.Raw); err != nil {
		t.Fatalf("sing-box parser rejected config with clash_api: %v", err)
	}
}

// TestClashAPIPortZeroIgnored — WithClashAPI(0, …) = выключено (guard на 0,
// чтобы случайный порт по умолчанию не включил контроллер).
func TestClashAPIPortZeroIgnored(t *testing.T) {
	cc, err := RenderFrom(mustChannels(t), testSecrets(true), WithClashAPI(0, "x"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(cc.Raw), "clash_api") {
		t.Fatalf("port 0 must disable clash_api rendering")
	}
}

// mustChannels — дескрипторы для RenderFrom (RenderClientConfig не принимает
// опций, а WithClashAPI — опция).
func mustChannels(t *testing.T) []ChannelDescriptor {
	t.Helper()
	channels, err := LoadDescriptors()
	if err != nil {
		t.Fatalf("descriptors: %v", err)
	}
	return channels
}
