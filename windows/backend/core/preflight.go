package core

// Bootstrap resolves only configured tunnel endpoints, before the engine or
// within its original per-box work domain when a new service dial needs DNS.
// Addresses are untrusted hints: TLS/service authentication and protected-path
// admission still run. Resolver redundancy does not prove path independence.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

var preflightUDPResolvers = [4]string{"1.1.1.1:53", "8.8.8.8:53", "77.88.8.8:53", "94.140.14.14:53"}
var preflightDoHEndpoints = [2]struct{ ip, path string }{{"1.1.1.1", "/dns-query"}, {"8.8.8.8", "/dns-query"}}

// A resolver owns one immutable operation scope. File/network resource owners
// are process-lived rather than lost when this lightweight value is discarded.
type PreflightResolver struct {
	CachePath string
	Timeout   time.Duration
	// Retained callback signature; production notices are fixed text without args.
	Logf     func(format string, args ...any)
	Scope    PreflightScope
	Now      func() (time.Time, error)
	Current  func() bool
	resolved *PreflightAddress    // per-call owned copy, never shared resolver state
	work     *preflightWorkDomain // nil: original App domain; worker: original per-box owner
}

func (r *PreflightResolver) ResolveAddress(ctx context.Context, host string) (PreflightAddress, error) {
	if r == nil {
		return PreflightAddress{}, ErrPreflightScope
	}
	local := *r
	var address PreflightAddress
	local.resolved = &address
	if _, _, err := local.Resolve(ctx, host); err != nil {
		return PreflightAddress{}, err
	}
	now, err := local.now()
	if err != nil {
		return PreflightAddress{}, err
	}
	if !preflightContextCurrent(ctx) || !local.current() {
		return PreflightAddress{}, errors.Join(ErrPreflightScope, contextError(ctx))
	}
	if !address.ValidAt(now) {
		return PreflightAddress{}, ErrPreflightAddressExpired
	}
	return address, nil
}

func (r *PreflightResolver) addressHint(ctx context.Context, host, ip string, cached bool, observed, until time.Time) (PreflightAddress, error) {
	now, err := r.now()
	if err != nil {
		return PreflightAddress{}, err
	}
	if deadline, bounded := ctx.Deadline(); bounded {
		limit := now.Add(time.Until(deadline)) // convert steady remaining budget to the existing logical-clock domain
		if limit.Before(until) {
			until = limit
		}
	}
	if !r.Scope.Until.IsZero() && r.Scope.Until.Before(until) {
		until = r.Scope.Until
	}
	address := PreflightAddress{Address: ip, Host: host, Scope: r.Scope, ObservedAt: observed, Until: until, Cached: cached}
	if !preflightContextCurrent(ctx) || !r.current() {
		return PreflightAddress{}, errors.Join(ErrPreflightScope, contextError(ctx))
	}
	if !address.ValidAt(now) {
		return PreflightAddress{}, ErrPreflightAddressExpired
	}
	if r.resolved != nil {
		*r.resolved = address
	}
	return address, nil
}

// These are local address-hint observations, never service authentication or
// protected-path proof. Keep host/IP/resolver identity and SDK errors out of Logf.
func (r *PreflightResolver) logAddressHint(source string) {
	if r.Logf == nil {
		return
	}
	switch source {
	case "cache":
		r.Logf("preflight: получена подсказка адреса из локального кеша; защищённый путь ещё не проверен")
	case "udp/tcp":
		r.Logf("preflight: получена подсказка адреса через DNS; защищённый путь ещё не проверен")
	case "doh":
		r.Logf("preflight: получена подсказка адреса через DoH; защищённый путь ещё не проверен")
	default:
		r.Logf("preflight: получена подсказка адреса; защищённый путь ещё не проверен")
	}
}
func (r *PreflightResolver) current() (ok bool) {
	defer func() { _ = recover() }()
	return r != nil && r.Scope.valid() && r.Current != nil && r.Current()
}
func (r *PreflightResolver) now() (now time.Time, err error) {
	err = ErrPreflightScope
	defer func() { _ = recover() }()
	if r == nil || r.Now == nil {
		return now, err
	}
	now, err = r.Now()
	if err != nil {
		return time.Time{}, err
	}
	if now.IsZero() || !r.Scope.Until.IsZero() && !now.Before(r.Scope.Until) {
		return time.Time{}, ErrPreflightScope
	}
	return now, err
}
func preflightContextCurrent(ctx context.Context) bool {
	if ctx == nil || ctx.Err() != nil {
		return false
	}
	deadline, bounded := ctx.Deadline()
	return !bounded || time.Now().Before(deadline)
}

type preflightResult struct {
	answer     netprobe.DNSHostAnswer
	observed   time.Time
	strategy   string
	err        error
	localFault bool
}

type preflightTiming struct {
	at  time.Time
	err error
}

func (r *PreflightResolver) timedDNS(raw []byte, id uint16, host string, timing *preflightTiming) (netprobe.DNSHostAnswer, error) {
	answer, err := netprobe.DecodeDNSHostResponse(raw, id, host)
	if err == nil {
		timing.at, timing.err = r.now()
		err = timing.err
	}
	return answer, err // sample before deferred Close; slow retirement cannot restart DNS TTL
}

func (r *PreflightResolver) resolveCached(ctx context.Context, host string) (string, bool, error) {
	address, ok := r.cacheAddress(host)
	if !ok {
		return "", false, nil
	}
	if _, err := r.addressHint(ctx, host, address.IP, true, address.At, address.Until); err != nil {
		return "", false, err
	}
	r.logAddressHint("cache")
	return address.IP, true, nil
}

// Resolve returns IPv4 only after every launched worker returned and original
// resources were retired. Caller cancellation never falls through to cache.
func (r *PreflightResolver) Resolve(parent context.Context, host string) (ip string, cached bool, err error) {
	if !preflightContextCurrent(parent) {
		return "", false, errors.Join(ErrPreflightScope, contextError(parent))
	}
	if literal := net.ParseIP(host); literal != nil {
		if !publicProbeIPv4(literal) {
			return "", false, netprobe.ErrDNSResponse
		}
		if r != nil && r.resolved != nil {
			now, err := r.now()
			if err != nil {
				return "", false, err
			}
			if _, err := r.addressHint(parent, host, literal.String(), false, now, now.Add(4*time.Second)); err != nil {
				return "", false, err
			}
		}
		return literal.String(), false, nil
	}
	host, err = netprobe.CanonicalDNSHost(host)
	if err != nil {
		return "", false, err
	}
	if !r.current() {
		return "", false, ErrPreflightScope
	}
	if _, err := r.now(); err != nil {
		return "", false, err
	}
	work := r.work
	if work == nil {
		work = &preflightWork
	}
	if !work.mu.TryLock() {
		return "", false, ErrPreflightBusy
	}
	if work.owner != nil {
		work.mu.Unlock()
		return "", false, ErrPreflightCleanup
	}
	owner := &preflightNetwork{}
	work.owner = owner
	timeout := r.Timeout
	if timeout <= 0 || timeout > 4*time.Second {
		timeout = 4 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	results := make(chan preflightResult, len(preflightUDPResolvers)+len(preflightDoHEndpoints))
	launched, received := 0, 0
	var localFault error
	note := func(result preflightResult) {
		if result.localFault {
			localFault = errors.Join(localFault, result.err)
		}
	}
	retired := false
	retire := func() error {
		cancel()
		_ = owner.close() // a dial can still be acquiring; final inspection follows its return
		for received < launched {
			note(<-results)
			received++
		}
		cleanup := owner.close()
		retired = cleanup == nil
		return cleanup
	}
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			ip, cached, err = "", false, errors.Join(err, ErrPreflightScope)
		}
		cleanup := retire()
		if cleanup != nil {
			ip, cached, err = "", false, errors.Join(err, ErrPreflightCleanup, cleanup)
		}
		if retired {
			work.owner = nil
		}
		work.mu.Unlock()
	}()
	ip, cached, err = func() (string, bool, error) {
		// A still-current hint needs no new DNS work. Reading it never refreshes
		// its observation time, TTL, health or service-authentication status.
		if address, found, err := r.resolveCached(parent, host); err != nil {
			return "", false, err
		} else if found {
			return address, true, nil
		}
		if !preflightContextCurrent(parent) || !r.current() {
			return "", false, errors.Join(ErrPreflightScope, contextError(parent))
		}
		if _, err := r.now(); err != nil {
			return "", false, err
		}
		launch := func(strategy string, fn func(*preflightTiming) (netprobe.DNSHostAnswer, error)) {
			launched++
			go func() {
				result := preflightResult{strategy: strategy}
				completed := false
				defer func() {
					if !completed {
						_ = recover()
						result.answer, result.err = netprobe.DNSHostAnswer{}, ErrPreflightScope
						result.localFault = true
					}
					results <- result // exactly one bounded slot per worker, including panic
				}()
				var timing preflightTiming
				result.answer, result.err = fn(&timing)
				result.observed, result.localFault = timing.at, timing.err != nil
				completed = true
			}()
		}
		for _, server := range preflightUDPResolvers {
			server := server
			launch("udp/tcp", func(timing *preflightTiming) (netprobe.DNSHostAnswer, error) {
				return r.udpAnswer(ctx, owner, server, host, timing)
			})
		}
		for _, endpoint := range preflightDoHEndpoints {
			endpoint := endpoint
			launch("doh", func(timing *preflightTiming) (netprobe.DNSHostAnswer, error) {
				return r.dohAnswer(ctx, owner, endpoint.ip, endpoint.path, host, timing)
			})
		}
		var winner preflightResult
	collect:
		for received < launched {
			select {
			case <-ctx.Done():
				break collect
			case result := <-results:
				received++
				note(result)
				if result.err == nil && len(result.answer.Addresses) > 0 && publicProbeIPv4(net.ParseIP(result.answer.Addresses[0])) {
					winner = result
					break collect
				}
			}
		}
		if cleanup := retire(); cleanup != nil {
			return "", false, errors.Join(ErrPreflightCleanup, cleanup)
		}
		if localFault != nil {
			return "", false, errors.Join(ErrPreflightScope, localFault)
		}
		if !preflightContextCurrent(parent) || !r.current() {
			return "", false, errors.Join(ErrPreflightScope, contextError(parent))
		}
		now, clockErr := r.now()
		if clockErr != nil {
			return "", false, clockErr
		}
		if len(winner.answer.Addresses) > 0 {
			if now.Before(winner.observed) {
				return "", false, ErrPreflightScope
			}
			if winner.answer.TTL > 0 && !now.Before(winner.observed.Add(time.Duration(winner.answer.TTL)*time.Second)) {
				return "", false, ErrPreflightAddressExpired
			}
			r.cachePutAnswer(host, winner.answer, winner.observed)
			now, clockErr = r.now()
			if clockErr != nil {
				return "", false, clockErr
			}
			if !preflightContextCurrent(parent) || !r.current() {
				return "", false, errors.Join(ErrPreflightScope, contextError(parent))
			}
			if winner.answer.TTL > 0 && !now.Before(winner.observed.Add(time.Duration(winner.answer.TTL)*time.Second)) {
				return "", false, ErrPreflightAddressExpired
			}
			duration := min(time.Duration(winner.answer.TTL)*time.Second, preflightCacheTTL)
			if winner.answer.TTL == 0 {
				duration = 4 * time.Second
			} // one fresh candidate only, never cache
			if _, err := r.addressHint(parent, host, winner.answer.Addresses[0], false, winner.observed, winner.observed.Add(duration)); err != nil {
				return "", false, err
			}
			r.logAddressHint(winner.strategy)
			return winner.answer.Addresses[0], false, nil
		}
		if address, found, err := r.resolveCached(parent, host); err != nil {
			return "", false, err
		} else if found {
			return address, true, nil
		}
		return "", false, errors.New("bootstrap DNS strategies did not return a usable public IPv4")
	}()
	completed = true
	return ip, cached, err
}

func (r *PreflightResolver) udpAnswer(ctx context.Context, owner *preflightNetwork, server, host string, timing *preflightTiming) (answer netprobe.DNSHostAnswer, err error) {
	var nonce [2]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return answer, err
	}
	id := binary.BigEndian.Uint16(nonce[:])
	query, err := netprobe.EncodeDNSHostQuery(host, id)
	if err != nil {
		return answer, err
	}
	conn, err := owner.dial(ctx, "udp", server)
	if err != nil {
		return answer, err
	}
	udp := conn
	defer func() { err = errors.Join(err, udp.Close()) }()
	if n, err := conn.Write(query); err != nil || n != len(query) {
		return answer, errors.Join(err, io.ErrShortWrite)
	}
	buffer := make([]byte, netprobe.MaxDNSMessage+1)
	n, err := conn.Read(buffer)
	if err != nil || n > netprobe.MaxDNSMessage {
		return answer, errors.Join(err, netprobe.ErrDNSResponse)
	}
	answer, err = r.timedDNS(buffer[:n], id, host, timing)
	truncated := errors.Is(err, netprobe.ErrDNSTruncated) || n >= 4 && binary.BigEndian.Uint16(buffer[:2]) == id && binary.BigEndian.Uint16(buffer[2:4])&0x0200 != 0
	if !truncated {
		return answer, err
	}
	if err := conn.Close(); err != nil {
		return netprobe.DNSHostAnswer{}, err
	}
	conn, err = owner.dial(ctx, "tcp", server) // same named resolver and exact question, no system DNS fallback
	if err != nil {
		return netprobe.DNSHostAnswer{}, err
	}
	tcp := conn
	defer func() { err = errors.Join(err, tcp.Close()) }()
	frame := make([]byte, len(query)+2)
	binary.BigEndian.PutUint16(frame[:2], uint16(len(query)))
	copy(frame[2:], query)
	if _, err := io.Copy(conn, bytes.NewReader(frame)); err != nil {
		return netprobe.DNSHostAnswer{}, err
	}
	var prefix [2]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		return netprobe.DNSHostAnswer{}, err
	}
	size := int(binary.BigEndian.Uint16(prefix[:]))
	if size < 12 || size > netprobe.MaxDNSMessage {
		return netprobe.DNSHostAnswer{}, netprobe.ErrDNSResponse
	}
	if _, err := io.ReadFull(conn, buffer[:size]); err != nil {
		return netprobe.DNSHostAnswer{}, err
	}
	return r.timedDNS(buffer[:size], id, host, timing)
}

func (r *PreflightResolver) dohAnswer(ctx context.Context, owner *preflightNetwork, address, path, host string, timing *preflightTiming) (answer netprobe.DNSHostAnswer, err error) {
	query, err := netprobe.EncodeDNSHostQuery(host, 0)
	if err != nil {
		return answer, err
	}
	var dialed atomic.Bool
	target := net.JoinHostPort(address, "443")
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxConnsPerHost: 1, MaxResponseHeaderBytes: 4096, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, actual string) (net.Conn, error) {
			if actual != target || !dialed.CompareAndSwap(false, true) {
				return nil, ErrPreflightScope
			}
			return owner.dial(ctx, "tcp", target)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+address+path, bytes.NewReader(query))
	if err != nil {
		return answer, err
	}
	request.GetBody = nil // no automatic replay of this bounded query
	request.Header.Set("Content-Type", netprobe.DNSMediaType)
	request.Header.Set("Accept", netprobe.DNSMediaType)
	response, err := client.Do(request)
	if response != nil && response.Body != nil {
		body, bodyErr := owner.body(response.Body)
		if body != nil {
			defer func() { err = errors.Join(err, body.Close()) }()
		}
		if err != nil || bodyErr != nil {
			return answer, errors.Join(err, bodyErr)
		}
		media, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if response.StatusCode != http.StatusOK || mediaErr != nil || media != netprobe.DNSMediaType || response.ContentLength > netprobe.MaxDNSMessage {
			return answer, netprobe.ErrDNSResponse
		}
		raw, readErr := io.ReadAll(io.LimitReader(body, netprobe.MaxDNSMessage+1))
		if readErr != nil || len(raw) > netprobe.MaxDNSMessage {
			return answer, errors.Join(readErr, netprobe.ErrDNSResponse)
		}
		return r.timedDNS(raw, 0, host, timing)
	}
	return answer, errors.Join(err, netprobe.ErrDNSResponse)
}

// Legacy JSON decoder remains only for source compatibility; the runtime
// bootstrap path uses the bounded correlated wire decoder above, including TTL.
func parseDoHAnswer(endpoint string, body []byte) (string, error) {
	if len(body) > netprobe.MaxDNSMessage {
		return "", netprobe.ErrDNSResponse
	}
	var payload struct {
		Status int
		Answer []struct {
			Type int
			Data string
		}
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Status != 0 {
		return "", netprobe.ErrDNSResponse
	}
	var ips []string
	for _, answer := range payload.Answer {
		if answer.Type == 1 {
			ips = append(ips, answer.Data)
		}
	}
	return firstPublicIPv4(ips)
}
func firstPublicIPv4(ips []string) (string, error) {
	for _, value := range ips {
		if ip := net.ParseIP(value); publicProbeIPv4(ip) {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("public IPv4 answer unavailable")
}
func publicProbeIPv4(ip net.IP) bool { return netprobe.PublicIPv4(ip) }
