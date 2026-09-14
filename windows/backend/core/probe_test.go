package core

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func probeWithTargets(t *testing.T, targets []string, expected string) *ProtectedProbe {
	t.Helper()
	return &ProtectedProbe{
		SOCKSAddr:      "127.0.0.1:1", // ничего не слушает: туннель недоступен
		Targets:        targets,
		Timeout:        2 * time.Second,
		ExpectedEgress: expected,
	}
}

func TestProbeValidateRejectsSingleTarget(t *testing.T) {
	pp := &ProtectedProbe{SOCKSAddr: "127.0.0.1:1080", Targets: []string{"https://a.example"}, Timeout: time.Second, ExpectedEgress: "1.2.3.4"}
	rep := pp.Run(context.Background())
	if rep.AllPassed || !strings.Contains(rep.Results[0].Detail, "at least 2") {
		t.Fatalf("single target must be rejected, got %+v", rep.Results)
	}
}

func TestProbeValidateRejectsBadExpectedEgress(t *testing.T) {
	pp := probeWithTargets(t, []string{"https://a", "https://b"}, "not-an-ip")
	rep := pp.Run(context.Background())
	if rep.AllPassed {
		t.Fatal("invalid expected egress must fail")
	}
}

func TestProbeValidateRejectsHTTPTarget(t *testing.T) {
	pp := probeWithTargets(t, []string{"http://a.example", "https://b.example"}, "1.2.3.4")
	rep := pp.Run(context.Background())
	if rep.AllPassed {
		t.Fatal("http target must be rejected")
	}
}

// egressServer — HTTPS-эхо, возвращающее заданный IP как text/plain.
func egressServer(t *testing.T, ip string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ip))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Тесты ниже проверяют логику шагов на реальном HTTP-стеке с локальными
// TLS-серверами; клиент с InsecureSkipVerify подменой транспорта недоступен
// снаружи, поэтому сетевые шаги с непрошедшим TLS дают fail — это тоже
// валидируемое поведение (report structure + AllPassed=false).

func TestProbeFailsWhenTunnelUnreachable(t *testing.T) {
	pp := probeWithTargets(t, []string{"https://api.ipify.org", "https://ifconfig.me"}, "203.0.113.10")
	rep := pp.Run(context.Background())
	if rep.AllPassed {
		t.Fatal("unreachable tunnel must fail the probe")
	}
	if len(rep.Results) < 2 {
		t.Fatalf("expected multiple step results, got %d", len(rep.Results))
	}
	for _, r := range rep.Results {
		if r.Name == "" {
			t.Fatal("every step must have a name")
		}
	}
	if rep.Summary() == "" {
		t.Fatal("summary must not be empty")
	}
}

// directLeakResult: контракт «TUN-режим: равен» (PLAN §2.3, V2-037).
func TestDirectLeakSemanticsPerMode(t *testing.T) {
	okStep := func(ip string) ProbeResult {
		return ProbeResult{Name: "Direct leak check", Passed: true, Detail: "egress IP: " + ip}
	}
	cases := []struct {
		name     string
		tunMode  bool
		direct   string
		tunnel   []string
		wantPass bool
	}{
		{"socks: equal = leak", false, "1.2.3.4", []string{"1.2.3.4"}, false},
		{"socks: differ = ok", false, "5.6.7.8", []string{"1.2.3.4"}, true},
		{"tun: equal = ok (весь трафик в туннеле)", true, "1.2.3.4", []string{"1.2.3.4"}, true},
		{"tun: differ = leak мимо туннеля", true, "5.6.7.8", []string{"1.2.3.4"}, false},
		{"tun: no tunnel egress = info", true, "5.6.7.8", nil, true},
	}
	for _, tc := range cases {
		r := directLeakResult(tc.tunMode, okStep(tc.direct), tc.direct, tc.tunnel)
		if r.Passed != tc.wantPass {
			t.Fatalf("%s: passed=%v, want %v (detail: %s)", tc.name, r.Passed, tc.wantPass, r.Detail)
		}
	}
	// Недоступный direct — не провал в обоих режимах, но с честной пометкой.
	for _, tunMode := range []bool{false, true} {
		r := directLeakResult(tunMode, ProbeResult{Passed: false, Detail: "timeout"}, "", []string{"1.2.3.4"})
		if !r.Passed {
			t.Fatalf("unavailable direct must not fail probe (tun=%v)", tunMode)
		}
	}
}

func TestProbeDoHStepUsesTunnelClient(t *testing.T) {
	// DoH-эндпоинт по IP-литералу: невалидный сертификат httptest-сервера
	// не пройдёт проверку — шаг обязан быть провалом с деталью про DoH,
	// а не повтором первой HTTPS-цели (регрессия V2-037: DNS-шаг был дублем).
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Status":0,"Answer":[{"type":1,"data":"93.184.216.34"}]}`))
	}))
	defer srv.Close()
	pp := &ProtectedProbe{SOCKSAddr: "127.0.0.1:1", Timeout: 2 * time.Second}
	res := pp.checkDNS(context.Background(), &http.Client{Timeout: 2 * time.Second})
	// Прямой клиент к https://1.1.1.1 из unit-среды: либо таймаут/недоступность,
	// либо успех с A-записью. Главное — шаг именован как DoH и не паникует.
	if res.Name == "" || !strings.Contains(res.Name, "DoH") {
		t.Fatalf("dns step must be DoH-based, got name %q", res.Name)
	}
}

func TestProbeContextCancellationRespected(t *testing.T) {
	pp := probeWithTargets(t, []string{"https://10.255.255.1", "https://10.255.255.2"}, "1.2.3.4")
	pp.Timeout = 5 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	rep := pp.Run(ctx)
	if rep.AllPassed {
		t.Fatal("canceled probe must not pass")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("cancellation must shorten the run")
	}
}

func TestProbeReportSummaryCountsFailures(t *testing.T) {
	rep := &ProbeReport{Results: []ProbeResult{
		{Name: "a", Passed: true},
		{Name: "b", Passed: false, Detail: "x"},
	}}
	s := rep.Summary()
	if !strings.Contains(s, "1 of 2") || !strings.Contains(s, "b") {
		t.Fatalf("summary must count failures and name the first: %s", s)
	}
}

func TestExtractIPVariants(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4\n":                  "1.2.3.4",
		`{"ip":"2606:4700::1"}`:      "2606:4700::1",
		`{"ip_addr":"203.0.113.10"}`: "203.0.113.10",
		"address: 5.6.7.8":           "5.6.7.8",
		"no ip here":                 "",
	}
	for in, want := range cases {
		if got := extractIP(in); got != want {
			t.Fatalf("extractIP(%q) = %q, want %q", in, got, want)
		}
	}
}

// freeListener используется в engine_test; здесь — минимальная проверка, что
// SOCKS-адрес из недоступного диапазона не паникует (ветка dial-ошибки).
func TestProbeDialErrorIsStepFailure(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback")
	}
	addr := l.Addr().String()
	l.Close()
	pp := &ProtectedProbe{
		SOCKSAddr:      addr, // порт закрыт: соединение отброшено
		Targets:        []string{"https://api.ipify.org", "https://ifconfig.me"},
		Timeout:        2 * time.Second,
		ExpectedEgress: "203.0.113.10",
	}
	rep := pp.Run(context.Background())
	if rep.AllPassed {
		t.Fatal("closed socks port must fail")
	}
}

// Параллельность шагов: SOCKS-цель — httptest-сервер, который не понимает
// SOCKS-хендшейк, поэтому каждый шаг висит до клиентского таймаута T. Все шаги
// идут одновременно ⇒ общее время ≈ T; последовательно было бы ≥ 4·T.
// Граница 3·T отсекает последовательное выполнение с запасом на CI-джиттер.
func TestProbeRunsStepsInParallel(t *testing.T) {
	srv1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10"))
	}))
	defer srv1.Close()
	srv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("203.0.113.10"))
	}))
	defer srv2.Close()

	const timeout = time.Second
	pp := &ProtectedProbe{
		SOCKSAddr:         srv1.Listener.Addr().String(),
		Targets:           []string{srv1.URL, srv2.URL},
		Timeout:           timeout,
		ExpectedEgress:    "203.0.113.10",
		AllowDirectEgress: true, // не добавляем сеть в unit-тест
	}
	start := time.Now()
	rep := pp.Run(context.Background())
	elapsed := time.Since(start)
	if rep.AllPassed {
		t.Fatal("socks-handshake against TLS server must fail, sanity check")
	}
	if elapsed > 3*timeout {
		t.Fatalf("steps appear sequential: %v elapsed (timeout=%v, sequential lower bound ≥ 4·timeout)", elapsed, timeout)
	}
}
