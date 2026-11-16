package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	ProbeRouteSOCKS          = "tunnel_socks"
	ProbeRouteEngineOutbound = "engine_outbound"
	ProbeRouteOS             = "os_control"
)

// ProbeObservation records an HTTP measurement, not a censorship diagnosis.
// Route is the requested route; egress verification remains a separate step.
// Endpoint excludes credentials, path/query/fragment. Completion describes this
// bounded response only: it does not prove native DNS/UDP, bulk flow or VPN speed.
// All fields are values so copying a ProbeResult also isolates its observation.
type ProbeObservation struct {
	Schema        int       `json:"schema"`
	Kind          string    `json:"kind"`
	Route         string    `json:"route"`
	Endpoint      string    `json:"endpoint"`
	Stage         string    `json:"stage"`
	ErrorKind     string    `json:"error_kind,omitempty"`
	HTTPStatus    int       `json:"http_status,omitempty"`
	PayloadBytes  int64     `json:"payload_bytes"`
	PayloadLimit  int64     `json:"payload_limit"`
	BodyComplete  bool      `json:"body_complete"`
	StartedAt     time.Time `json:"started_at"`
	ElapsedMicros int64     `json:"elapsed_micros"`
}

type probeHTTPTrace struct {
	mu     sync.Mutex
	phase  int
	closed bool
	fact   ProbeObservation
}

var errProbeBodyLimit = errors.New("probe response exceeds observation body limit")

func (t *probeHTTPTrace) stage(phase int, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed && phase >= t.phase {
		t.phase, t.fact.Stage = phase, name
	}
}

func safeProbeEndpoint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "invalid-endpoint"
	}
	return u.Host // URL.Host excludes User, path, query and fragment.
}

func probeHTTPError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	var certificate *tls.CertificateVerificationError
	var hostname x509.HostnameError
	var authority x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &certificate) || errors.As(err, &hostname) || errors.As(err, &authority) || errors.As(err, &invalid) {
		return "tls_verification"
	}
	var record tls.RecordHeaderError
	if errors.As(err, &record) {
		return "tls_protocol"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "dns_lookup"
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "short_response"
	}
	var operation *net.OpError
	if errors.As(err, &operation) {
		switch operation.Op {
		case "dial":
			return "connect_error"
		case "read":
			return "read_error"
		case "write":
			return "write_error"
		}
	}
	return "request_error"
}

func (o ProbeObservation) Summary() string {
	completion := "partial/unknown"
	if o.BodyComplete {
		completion = "complete"
	}
	return fmt.Sprintf("stage=%s route=%s HTTP=%d body=%d/%d %s error=%s", o.Stage, o.Route, o.HTTPStatus, o.PayloadBytes, o.PayloadLimit, completion, o.ErrorKind)
}

// All protected HTTP/DoH I/O uses the same observation path. Trace callbacks
// may be concurrent or late, so the final value is frozen under the trace lock.
// No arbitrary error/body text is returned in the public observation.
func fetchProbeHTTP(ctx context.Context, client *http.Client, target, route, kind, accept string, limit int64) (fact ProbeObservation, body []byte, finalErr error) {
	return fetchProbeHTTPRequest(ctx, client, target, route, kind, accept, limit, probeHTTPRequest{})
}

type probeHTTPRequest struct {
	method, contentType, expectedMediaType string
	body                                   []byte
}

func fetchProbeHTTPRequest(ctx context.Context, client *http.Client, target, route, kind, accept string, limit int64, request probeHTTPRequest) (fact ProbeObservation, body []byte, finalErr error) {
	started := time.Now()
	trace := &probeHTTPTrace{fact: ProbeObservation{Schema: 1, Kind: kind, Route: route, Endpoint: safeProbeEndpoint(target), Stage: "request", PayloadLimit: limit, StartedAt: started.UTC()}}
	defer func() {
		trace.mu.Lock()
		defer trace.mu.Unlock()
		trace.closed = true
		trace.fact.ElapsedMicros = time.Since(started).Microseconds()
		if trace.fact.ErrorKind == "" {
			trace.fact.ErrorKind = probeHTTPError(finalErr)
		}
		fact = trace.fact
	}()
	if request.method == "" {
		request.method = http.MethodGet
	}
	if ctx == nil || client == nil || limit < 1 || limit > maxProbeBody || request.method != http.MethodGet && request.method != http.MethodPost || len(request.body) > maxDNSProbeQuery || request.method == http.MethodGet && len(request.body) != 0 {
		trace.fact.ErrorKind = "invalid_request"
		return fact, nil, errors.New("invalid bounded probe request")
	}
	var input io.Reader
	if len(request.body) != 0 {
		input = bytes.NewReader(request.body)
	}
	req, err := http.NewRequestWithContext(ctx, request.method, target, input)
	if err != nil {
		trace.fact.ErrorKind = "invalid_request"
		return fact, nil, errors.New("invalid probe URL")
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if request.contentType != "" {
		req.Header.Set("Content-Type", request.contentType)
	}
	callbacks := &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { trace.stage(1, "dns") },
		ConnectStart:         func(string, string) { trace.stage(2, "connect") },
		TLSHandshakeStart:    func() { trace.stage(3, "tls") },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { trace.stage(3, "tls") },
		GotFirstResponseByte: func() { trace.stage(4, "headers") },
	}
	resp, err := client.Do(req.WithContext(httptrace.WithClientTrace(ctx, callbacks)))
	if err != nil {
		return fact, nil, err
	}
	defer resp.Body.Close()
	trace.stage(4, "headers")
	trace.mu.Lock()
	trace.fact.HTTPStatus = resp.StatusCode
	trace.mu.Unlock()
	if request.expectedMediaType != "" && resp.StatusCode == http.StatusOK {
		mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || mediaType != request.expectedMediaType {
			trace.mu.Lock()
			trace.fact.ErrorKind = "content_type"
			trace.mu.Unlock()
			return fact, nil, errors.New("unexpected probe response media type")
		}
		if encoding := strings.TrimSpace(resp.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
			trace.mu.Lock()
			trace.fact.ErrorKind = "content_encoding"
			trace.mu.Unlock()
			return fact, nil, errors.New("unsupported encoded probe response")
		}
	}
	trace.stage(5, "body")
	body, finalErr = io.ReadAll(io.LimitReader(resp.Body, limit+1))
	trace.mu.Lock()
	trace.fact.HTTPStatus = resp.StatusCode
	trace.fact.PayloadBytes = int64(len(body))
	trace.fact.BodyComplete = finalErr == nil && int64(len(body)) <= limit
	if int64(len(body)) > limit {
		trace.fact.ErrorKind = "body_limit"
		finalErr = errProbeBodyLimit
	}
	trace.mu.Unlock()
	return fact, body, finalErr
}
