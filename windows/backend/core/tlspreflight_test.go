package core

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPreflightQUICUnsupported(t *testing.T) {
	target := TLSTarget{Tag: "hy2", Server: "127.0.0.1", Port: 8444, ServerName: "x.invalid", Transport: "quic"}
	rep, err := CheckProtectedTLS(context.Background(), target, time.Second)
	if !errors.Is(err, ErrPreflightUnsupported) {
		t.Fatalf("want ErrPreflightUnsupported, got %v", err)
	}
	if rep.Verified || !strings.Contains(rep.Reason, "skipped") {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestPreflightUntrustedSelfSignedClassified(t *testing.T) {
	// Локальный TLS-сервер с самоподписанным сертификатом — та же ситуация,
	// что с HY2-сервером до пиннинга (P0-2026-09-08).
	srv := egressServer(t, "203.0.113.99") // httptest: самоподписанный сертификат
	addr := strings.TrimPrefix(srv.URL, "https://")
	target := TLSTarget{Tag: "a", Server: "127.0.0.1", ServerName: "127.0.0.1", Transport: "tcp"}
	target.Port = uint16(portOf(addr))
	rep, err := CheckProtectedTLS(context.Background(), target, 3*time.Second)
	if !errors.Is(err, ErrTLSUntrusted) {
		t.Fatalf("want ErrTLSUntrusted, got %v", err)
	}
	// Контракт redaction: endpoint не появляется в сообщении вообще
	// (x509-ошибка без адреса — заменять нечего, но адрес не должен утекать).
	if strings.Contains(rep.Reason, "127.0.0.1") {
		t.Fatalf("endpoint must be redacted: %s", rep.Reason)
	}
	if !strings.Contains(rep.Reason, "self-signed") {
		t.Fatalf("classification hint missing: %s", rep.Reason)
	}
}

func TestPreflightPinnedCertVerifies(t *testing.T) {
	// Сервер httptest + его сертификат как pin: проверка должна пройти.
	srv := egressServer(t, "203.0.113.100")
	pemPath := writeServerCertPEM(t, srv)
	addr := strings.TrimPrefix(srv.URL, "https://")
	host, _, _ := strings.Cut(addr, ":")
	target := TLSTarget{
		Tag: "pinned", Server: host, ServerName: host,
		Port: uint16(portOf(addr)), Transport: "tcp", CertificatePath: pemPath,
	}
	rep, err := CheckProtectedTLS(context.Background(), target, 3*time.Second)
	if err != nil {
		t.Fatalf("pinned verify must pass: %v (report %+v)", err, rep)
	}
	if !rep.Verified || rep.Issuer == "" {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestPreflightUnreachableIsPlainError(t *testing.T) {
	target := TLSTarget{Tag: "down", Server: "127.0.0.1", Port: 1, ServerName: "x.invalid", Transport: "tcp"}
	_, err := CheckProtectedTLS(context.Background(), target, time.Second)
	if err == nil || errors.Is(err, ErrTLSUntrusted) {
		t.Fatalf("connection refused must not be classified as untrusted: %v", err)
	}
}

func TestRunTLSPreflightAggregatesFailures(t *testing.T) {
	eps := []TLSTarget{
		{Tag: "q", Server: "127.0.0.1", Port: 1, ServerName: "q", Transport: "quic"}, // skip
		{Tag: "t", Server: "127.0.0.1", Port: 1, ServerName: "t", Transport: "tcp"},  // refused
	}
	reports, err := RunTLSPreflight(context.Background(), eps, time.Second)
	if err == nil {
		t.Fatal("refused channel must fail the aggregate")
	}
	if len(reports) != 2 {
		t.Fatalf("want 2 reports, got %d", len(reports))
	}
}

func TestTLSReportString(t *testing.T) {
	ok := TLSReport{Tag: "a", Verified: true, Issuer: "R3", NotAfter: time.Now()}
	if s := ok.String(); !strings.Contains(s, "OK") || !strings.Contains(s, "R3") {
		t.Fatalf("ok string: %s", s)
	}
	bad := TLSReport{Tag: "b", Reason: "no trust"}
	if s := bad.String(); !strings.Contains(s, "FAIL") {
		t.Fatalf("fail string: %s", s)
	}
}

func TestPreflightCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := TLSTarget{Tag: "x", Server: "127.0.0.1", Port: 443, ServerName: "x", Transport: "tcp"}
	if _, err := CheckProtectedTLS(ctx, target, time.Second); err == nil {
		t.Fatal("canceled context must fail")
	}
}

// ---------- helpers ----------

func portOf(hostport string) int {
	_, p, _ := strings.Cut(hostport, ":")
	n := 0
	for _, r := range p {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// writeServerCertPEM выписывает сертификат httptest-сервера во временный PEM.
func writeServerCertPEM(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	cert := srv.Certificate()
	if cert == nil || len(cert.Raw) == 0 {
		t.Fatal("server has no certificate")
	}
	return writeTempPEMFromDER(t, cert.Raw)
}
