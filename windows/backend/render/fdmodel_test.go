package render

// Тесты Failure Domain Model (V2-059, PLAN B2; plan_gpt §5/§6).

import (
	"strings"
	"testing"
)

func fdChannel(id string, ch ChannelDescriptor) ChannelDescriptor {
	ch.ID = id
	return ch
}

// fdEq — сравнение с допуском: 1 − Σштрафов в float64 не обязан совпадать
// с десятичной дробью побитно (1−0.30−0.05 ≠ 0.65 точно).
func fdEq(a, b float64) bool { return absDiff(a, b) < 1e-9 }

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestFDDerivation — умолчания из уже валидированных полей; явные fd.* сильнее.
func TestFDDerivation(t *testing.T) {
	base := func() ChannelDescriptor {
		return ChannelDescriptor{
			Protocol:     "vless",
			Transport:    "ws",
			Hostname:     "vpn.example.com",
			Port:         443,
			OriginServer: "203.0.113.10",
		}
	}

	// ws + порт 443: carrier=cloudflare, layer=tcp, port_family=443.
	fd := base().effectiveFD()
	if fd.TransportFamily != "vless+ws" || fd.Carrier != "cloudflare" ||
		fd.Origin != "203.0.113.10" || fd.PortFamily != "443" || fd.Layer != "tcp" {
		t.Fatalf("ws: неожиданная деривация: %+v", fd)
	}

	// HY2: layer=udp, carrier пуст (origin direct), port_family=other (8444).
	hy2 := base()
	hy2.Protocol, hy2.Transport, hy2.Port = "hysteria2", "quic", 8444
	fd = hy2.effectiveFD()
	if fd.Layer != "udp" || fd.Carrier != "" || fd.PortFamily != "other" {
		t.Fatalf("hy2: неожиданная деривация: %+v", fd)
	}

	// HY2 с server_ports "443:443" первым: port_family=443 (hopping через 443).
	hy2.ServerPorts = []string{"443:443", "8444:8444"}
	if fd = hy2.effectiveFD(); fd.PortFamily != "443" {
		t.Fatalf("hy2 hopping: port_family=%q, ждали 443", fd.PortFamily)
	}

	// tcp-reality: донор деривируется из hostname, layer=tcp.
	reality := base()
	reality.Protocol, reality.Transport = "vless", "tcp-reality"
	reality.Hostname = "www.samsung.com"
	fd = reality.effectiveFD()
	if fd.DonorSNI != "www.samsung.com" || fd.Layer != "tcp" || fd.Carrier != "" {
		t.Fatalf("reality: неожиданная деривация: %+v", fd)
	}

	// Явные поля сильнее деривации; ASN не деривируется никогда.
	ws := base()
	ws.FD.Carrier = "fastly"
	ws.FD.ASN = "AS12345"
	fd = ws.effectiveFD()
	if fd.Carrier != "fastly" || fd.ASN != "AS12345" {
		t.Fatalf("override: явные fd.* проигнорированы: %+v", fd)
	}
}

// TestFDPairsExhaustive — настоящая ценность модели: точные числа по РЕАЛЬНОМУ
// встроенному набору (6 пар). Числа = контракты plan_gpt §6, зашитые здесь:
// изменение модели без пересмотра этих значений ломает тест.
func TestFDPairsExhaustive(t *testing.T) {
	chans, err := parseDescriptors(descriptorsRaw)
	if err != nil {
		t.Fatalf("встроенные дескрипторы не парсятся: %v", err)
	}
	if len(chans) != 4 {
		t.Fatalf("ждали 4 канала, получено %d", len(chans))
	}
	byID := map[string]ChannelDescriptor{}
	for _, c := range chans {
		byID[c.ID] = c
	}

	const (
		vless   = "channel-a-vless"
		hy2     = "channel-a-hy2"
		reality = "channel-a-reality"
		shadow  = "channel-a-shadowtls"
	)

	// Все 6 пар: независимость + список общих доменов (для честного UI).
	for i := 0; i < len(chans); i++ {
		for j := i + 1; j < len(chans); j++ {
			a, b := chans[i], chans[j]
			ind := Independence(a, b)
			sum := IndependenceSummary(a, b)
			t.Logf("%s <-> %s: %.2f [%s]", a.ID, b.ID, ind, sum)
		}
	}

	// WS↔HY2: один origin + один ASN (один VPS) → safety clamp 0.25.
	if got := Independence(byID[vless], byID[hy2]); !fdEq(got, 0.25) {
		t.Fatalf("vless<->hy2 = %.2f, ждали safety clamp 0.25", got)
	}

	// Пары на одном origin: 203.0.113.10 у всех четырёх → ни одна пара
	// не независима полностью (V2-044 «домен не удвоен» теперь формально).
	pairs := [][2]string{
		{vless, reality}, {vless, shadow}, {hy2, reality},
		{hy2, shadow}, {reality, shadow},
	}
	for _, p := range pairs {
		if got := Independence(byID[p[0]], byID[p[1]]); got >= 1.0 {
			t.Fatalf("%s<->%s = %.2f: общий origin не учтён", p[0], p[1], got)
		}
	}

	// vless↔reality: один origin + один ASN → safety clamp 0.25.
	if got := Independence(byID[vless], byID[reality]); !fdEq(got, 0.25) {
		t.Fatalf("vless<->reality = %.2f, ждали safety clamp 0.25", got)
	}

	// reality↔shadowtls: один origin + один ASN → safety clamp 0.25.
	if got := Independence(byID[reality], byID[shadow]); !fdEq(got, 0.25) {
		t.Fatalf("reality<->shadowtls = %.2f, ждали safety clamp 0.25", got)
	}

	// vless↔shadowtls: один origin + один ASN → safety clamp 0.25.
	if got := Independence(byID[vless], byID[shadow]); !fdEq(got, 0.25) {
		t.Fatalf("vless<->shadowtls = %.2f, ждали safety clamp 0.25", got)
	}

	// hy2↔reality: один origin + один ASN → safety clamp 0.25.
	if got := Independence(byID[hy2], byID[reality]); !fdEq(got, 0.25) {
		t.Fatalf("hy2<->reality = %.2f, ждали safety clamp 0.25", got)
	}

	// hy2↔shadowtls: один origin + один ASN → safety clamp 0.25.
	if got := Independence(byID[hy2], byID[shadow]); !fdEq(got, 0.25) {
		t.Fatalf("hy2<->shadowtls = %.2f, ждали safety clamp 0.25", got)
	}

	// Порог: reality↔shadowtls ниже порога; общий донор остаётся объяснимым
	// доменом для UI/journal.
	if s := IndependenceSummary(byID[reality], byID[shadow]); !strings.Contains(s, "донор") {
		t.Fatalf("summary без донора: %q", s)
	}
	if got := Independence(byID[reality], byID[shadow]); got >= IndependenceThreshold {
		t.Fatalf("same origin+ASN must be below threshold: %.2f", got)
	}
}

func TestFDKnownOriginAndASNClamp(t *testing.T) {
	a := ChannelDescriptor{ID: "a", Protocol: "vless", Transport: "ws", OriginServer: "203.0.113.10", Port: 443}
	b := ChannelDescriptor{ID: "b", Protocol: "hysteria2", Transport: "quic", OriginServer: "203.0.113.10", Port: 8444}
	a.FD.ASN, b.FD.ASN = "AS57043", "AS57043"
	if got := Independence(a, b); !fdEq(got, 0.25) {
		t.Fatalf("known origin+ASN clamp = %.2f, want 0.25", got)
	}
	if !strings.Contains(IndependenceSummary(a, b), "ASN") {
		t.Fatalf("summary must expose shared ASN: %q", IndependenceSummary(a, b))
	}
	// Unknown ASN must not be invented: same origin alone keeps additive policy.
	b.FD.ASN = ""
	if got := Independence(a, b); fdEq(got, 0.25) {
		t.Fatalf("unknown ASN must not trigger known-origin+ASN clamp")
	}
}

// TestFDUnknownHonesty — unknown/пустые домены не создают ложной «общности».
func TestFDUnknownHonesty(t *testing.T) {
	a := fdChannel("a", ChannelDescriptor{Protocol: "vless", Transport: "ws", OriginServer: "1.2.3.4", Port: 443})
	b := fdChannel("b", ChannelDescriptor{Protocol: "vless", Transport: "ws", OriginServer: "5.6.7.8"})
	// Общие transport (0.20) + carrier cloudflare (0.20) + слой tcp (0.05)
	// = 0.45 → 0.55; классы порта разные (443 vs 0/other), origin/ASN разные.
	if ind := Independence(a, b); !fdEq(ind, 0.55) {
		t.Fatalf("a<->b = %.2f, ждали 0.55 (transport+carrier+layer)", ind)
	}
	if s := IndependenceSummary(a, b); !strings.Contains(s, "транспорт") || strings.Contains(s, "origin") {
		t.Fatalf("summary: %q", s)
	}
	// Никаких общих известных доменов → пустая строка, не «всё одинаково».
	// (Порты заданы явно: 443 vs 8444 — разные классы, иначе Port=0 дал бы
	// обоим «other» и ложный штраф.)
	c := fdChannel("c", ChannelDescriptor{Protocol: "hysteria2", Transport: "quic", OriginServer: "9.9.9.9", Port: 8444})
	c.FD.Carrier = "fastly"
	if s := IndependenceSummary(a, c); s != "" {
		t.Fatalf("независимые каналы дали summary %q", s)
	}
	if ind := Independence(a, c); ind != 1.0 {
		t.Fatalf("независимые каналы = %.2f, ждали 1.0", ind)
	}
	// Сам себе — 0.
	if ind := Independence(a, a); ind != 0 {
		t.Fatalf("self = %.2f, ждали 0", ind)
	}
}

// TestValidateFD — форма ASN/словари ловятся при парсинге.
func TestValidateFD(t *testing.T) {
	good := []FailureDomain{
		{},
		{ASN: "AS12345", PortFamily: "443", Layer: "tcp", Carrier: "cloudflare"},
		{PortFamily: "other"},
		{Layer: "udp"},
	}
	for _, fd := range good {
		if err := validateFD(fd); err != nil {
			t.Fatalf("валидный %+v отклонён: %v", fd, err)
		}
	}
	bad := []FailureDomain{
		{ASN: "12345"}, {ASN: "AS"}, {ASN: "AS12a45"}, {ASN: "as12345"},
		{PortFamily: "8080"}, {Layer: "icmp"},
	}
	for _, fd := range bad {
		if err := validateFD(fd); err == nil {
			t.Fatalf("невалидный %+v прошёл", fd)
		}
	}
}
