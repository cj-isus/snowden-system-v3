package main

// Тесты авто-выбора транспорта по классу сети (V2-054). Чистые функции от
// (descriptors, class) — сеть и рендер не нужны: дескрипторы — фикстуры
// той же формы, что render.LoadDescriptors.

import (
	"testing"

	"github.com/snowden-system/windows/backend/render"
)

func prefDescriptors() []render.ChannelDescriptor {
	return []render.ChannelDescriptor{
		{ID: "channel-a-vless", Protocol: "vless", Transport: "ws", Enabled: true, ValidationStatus: "live-verified"},
		{ID: "channel-a-hy2", Protocol: "hysteria2", Transport: "quic", Enabled: true, ValidationStatus: "live-verified"},
		{ID: "channel-a-reality", Protocol: "vless", Transport: "tcp-reality", Enabled: true, ValidationStatus: "live-verified"},
		{ID: "channel-a-shadowtls", Protocol: "vless", Transport: "shadowtls", Enabled: true, ValidationStatus: "configured"},
	}
}

func TestPickDefaultChannelMobilePrefersHY2(t *testing.T) {
	got := pickDefaultChannel(prefDescriptors(), "mobile")
	if got != "channel-a-hy2" {
		t.Fatalf("mobile: default = %q, want channel-a-hy2 (QUIC живее CGNAT-валов)", got)
	}
}

func TestPickDefaultChannelWifiPrefersCDN(t *testing.T) {
	for _, cls := range []string{"wifi", "ethernet"} {
		got := pickDefaultChannel(prefDescriptors(), cls)
		if got != "channel-a-vless" {
			t.Fatalf("%s: default = %q, want channel-a-vless", cls, got)
		}
	}
}

func TestPickDefaultChannelUnknownClassNoPreference(t *testing.T) {
	if got := pickDefaultChannel(prefDescriptors(), ""); got != "" {
		t.Fatalf("пустой класс: default = %q, want \"\" (прежний pickDefault)", got)
	}
	if got := pickDefaultChannel(prefDescriptors(), "satellite"); got != "" {
		t.Fatalf("неизвестный класс: default = %q, want \"\"", got)
	}
}

// Не-valid каналы не участвуют в предпочтении: тот же гейт, что у
// селектора — фейковый live-verified через политику невозможен.
// Если лучшая предпочтение класса не validated — берётся СЛЕДУЮЩЕЕ
// в порядке класса (rank-fallback); если не осталось ни одного — ""
// (прежний pickDefault).
func TestPickDefaultChannelRespectsValidationGate(t *testing.T) {
	descs := prefDescriptors()
	descs[1].ValidationStatus = "configured" // HY2 больше не validated
	// mobile: HY2 выпал → следующий в порядке = reality (rank-fallback).
	if got := pickDefaultChannel(descs, "mobile"); got != "channel-a-reality" {
		t.Fatalf("mobile без HY2: default = %q, want channel-a-reality (rank-fallback)", got)
	}
	descs[2].ValidationStatus = "blocked" // и reality выпал
	if got := pickDefaultChannel(descs, "mobile"); got != "" {
		t.Fatalf("mobile без HY2 и reality: default = %q, want \"\"", got)
	}
	descs[1].ValidationStatus = "live-verified" // HY2 вернуть: пригодится wifi-ветке
	descs[0].ValidationStatus = "blocked" // wifi: vless+ws выпал
	if got := pickDefaultChannel(descs, "wifi"); got != "channel-a-hy2" {
		t.Fatalf("wifi без vless: default = %q, want channel-a-hy2 (rank-fallback)", got)
	}
}

// Rank-порядок: на mobile HY2 лучше reality (первый в порядке предпочтений).
func TestPickDefaultChannelRankOrder(t *testing.T) {
	descs := prefDescriptors()
	descs[1].ValidationStatus = "configured" // убрать HY2 из кандидатов
	// mobile: hysteria2+quic недоступен → следующий в порядке = vless+tcp-reality.
	if got := pickDefaultChannel(descs, "mobile"); got != "channel-a-reality" {
		t.Fatalf("mobile без HY2: default = %q, want channel-a-reality", got)
	}
}

// Тай-брейк стабильности: два канала одного транспорта — берётся первый
// по порядку дескрипторов.
func TestPickDefaultChannelStableTieBreak(t *testing.T) {
	descs := prefDescriptors()
	extra := render.ChannelDescriptor{ID: "channel-b-hy2", Protocol: "hysteria2", Transport: "quic", Enabled: true, ValidationStatus: "live-verified"}
	descs = append(descs, extra)
	if got := pickDefaultChannel(descs, "mobile"); got != "channel-a-hy2" {
		t.Fatalf("тай-брейк: default = %q, want channel-a-hy2 (первый в дескрипторах)", got)
	}
}

// Транспорт-ключ — тот же формат, что ChannelView.Transport в UI.
func TestTransportKey(t *testing.T) {
	if got := transportKey(prefDescriptors()[0]); got != "vless+ws" {
		t.Fatalf("transportKey = %q, want vless+ws", got)
	}
}
