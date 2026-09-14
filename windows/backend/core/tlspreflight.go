// TLS preflight — предстартовая диагностика защищённых каналов (урок
// P0-2026-09-08 «HY2 self-signed cert»): превращает непрозрачную x509-ошибку
// из sing-box в конкретную причину ДО старта туннеля. Диагностика, не
// послабление: проверка не отключает верификацию и не добавляет direct.
package core

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrTLSUntrusted         = errors.New("server certificate is not trusted by the system trust store")
	ErrTLSHostMismatch      = errors.New("server certificate does not match the requested host")
	ErrTLSInvalid           = errors.New("server certificate is invalid")
	ErrPreflightUnsupported = errors.New("preflight is not supported for this transport")
)

// TLSTarget — публичные метаданные канала для проверки (без секретов).
type TLSTarget struct {
	Tag             string
	Server          string
	Port            uint16
	ServerName      string
	CertificatePath string
	Transport       string // "tcp" | "quic"
}

func (t TLSTarget) Addr() string {
	host := t.Server
	if host == "" {
		host = t.ServerName
	}
	return net.JoinHostPort(host, strconv.Itoa(int(t.Port)))
}

// SupportsTCPPreflight: QUIC-каналы по TCP не проверяются — ложный негатив
// хуже отсутствия проверки.
func (t TLSTarget) SupportsTCPPreflight() bool {
	return !strings.EqualFold(t.Transport, "quic")
}

// TLSReport — redacted-результат, пригодный для лога и UI.
type TLSReport struct {
	Tag        string
	Address    string
	ServerName string
	Verified   bool
	Subject    string
	Issuer     string
	NotAfter   time.Time
	Reason     string
}

func (r TLSReport) String() string {
	if r.Verified {
		return fmt.Sprintf("tls preflight %s: OK, issuer=%q, not_after=%s",
			r.Tag, r.Issuer, r.NotAfter.Format(time.RFC3339))
	}
	return fmt.Sprintf("tls preflight %s: FAIL, reason=%q", r.Tag, r.Reason)
}

// CheckProtectedTLS проверяет цепочку сертификата. RootCAs: pinned-файл
// канала, если задан; иначе системное хранилище доверия (то же, что в sing-box).
func CheckProtectedTLS(ctx context.Context, target TLSTarget, timeout time.Duration) (TLSReport, error) {
	report := TLSReport{
		Tag:        target.Tag,
		Address:    target.Addr(),
		ServerName: target.ServerName,
	}
	if !target.SupportsTCPPreflight() {
		report.Reason = "skipped: transport " + target.Transport + " is not verifiable over TCP"
		return report, fmt.Errorf("%w: %s", ErrPreflightUnsupported, target.Transport)
	}
	if err := ctx.Err(); err != nil {
		report.Reason = err.Error()
		return report, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < timeout {
		timeout = time.Until(deadline)
	}

	rootCAs, err := certificateRoots(target.CertificatePath)
	if err != nil {
		report.Reason = "pinned certificate is unavailable: " + err.Error()
		return report, errors.New(report.Reason)
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: timeout},
		Config: &tls.Config{
			ServerName: target.ServerName,
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
		},
	}
	conn, err := dialer.DialContext(ctx, "tcp", report.Address)
	if err != nil {
		classified := classifyTLSError(err)
		report.Reason = redactTLSReason(classified.Error(), target)
		return report, safeTLSError(classified, report.Reason)
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		report.Reason = "TLS dial returned a non-TLS connection"
		return report, fmt.Errorf("%w: non-TLS connection", ErrTLSInvalid)
	}
	chain := tlsConn.ConnectionState().PeerCertificates
	if len(chain) == 0 {
		report.Reason = "server presented no certificate"
		return report, fmt.Errorf("%w: empty certificate chain", ErrTLSInvalid)
	}
	leaf := chain[0]
	report.Verified = true
	report.Subject = certName(leaf.Subject.CommonName, leaf.Subject.String())
	report.Issuer = certName(leaf.Issuer.CommonName, leaf.Issuer.String())
	report.NotAfter = leaf.NotAfter
	return report, nil
}

// RunTLSPreflight — проверка всех защищённых каналов конфига. Возвращает
// ошибку, если хотя бы один TCP-канал не прошёл проверку (QUIC — skip).
func RunTLSPreflight(ctx context.Context, endpoints []TLSTarget, timeout time.Duration) ([]TLSReport, error) {
	var reports []TLSReport
	failed := 0
	for _, ep := range endpoints {
		report, err := CheckProtectedTLS(ctx, ep, timeout)
		reports = append(reports, report)
		if err != nil && !errors.Is(err, ErrPreflightUnsupported) {
			failed++
		}
	}
	if failed > 0 {
		return reports, fmt.Errorf("%d of %d protected channels are not verifiable", failed, len(endpoints))
	}
	return reports, nil
}

func certificateRoots(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pinned certificate: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("pinned certificate is not valid PEM")
	}
	return roots, nil
}

func redactTLSReason(reason string, target TLSTarget) string {
	for _, value := range []string{target.Addr(), target.Server, target.ServerName} {
		if value != "" {
			reason = strings.ReplaceAll(reason, value, "<endpoint>")
		}
	}
	return reason
}

func safeTLSError(classified error, reason string) error {
	for _, category := range []error{
		context.Canceled,
		context.DeadlineExceeded,
		ErrTLSUntrusted,
		ErrTLSHostMismatch,
		ErrTLSInvalid,
	} {
		if errors.Is(classified, category) {
			return fmt.Errorf("%w: %s", category, reason)
		}
	}
	return errors.New(reason)
}

func certName(commonName, fallback string) string {
	if strings.TrimSpace(commonName) != "" {
		return commonName
	}
	return fallback
}

func classifyTLSError(err error) error {
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidErr x509.CertificateInvalidError

	switch {
	case errors.As(err, &unknownAuthority):
		return fmt.Errorf("%w: %s (likely a self-signed or private CA certificate on the server; fix the server certificate, do not set tls.insecure)",
			ErrTLSUntrusted, err)
	case errors.As(err, &hostnameErr):
		return fmt.Errorf("%w: %s", ErrTLSHostMismatch, err)
	case errors.As(err, &invalidErr):
		return fmt.Errorf("%w: %s", ErrTLSInvalid, err)
	default:
		return err
	}
}
