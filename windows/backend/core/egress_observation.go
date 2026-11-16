package core

// Optional display observations, never a protected probe or forwarding grant.
// Reuse original preflight socket/body custody and the diagnostic budget. HTTP/1
// is synchronous: no Transport pool, automatic retry, redirect or HTTP worker.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

const (
	egressObservationBudget = 6 * time.Second
	egressRequestBudget     = 3 * time.Second
	egressHeaderLimit       = 16 << 10
	egressPlaintextLimit    = 32 << 10 // decoded TLS stream, not TLS/wire/RSS accounting
	egressControlBodyLimit  = 4096
	egressSOCKSBodyLimit    = 64
	egressSOCKSAddress      = "127.0.0.1:1080"
	maxEgressAddresses      = 8
)

var ErrEgressObservation = errors.New("optional egress observation unavailable")

// Bounded raw replies. Their contents are untrusted endpoint assertions. A
// nonempty SOCKS reply is not native DNS, VPN/TUN or route-independence proof.
type EgressHTTPSObservation struct {
	OSControl, SOCKS                []byte
	OSObservation, SOCKSObservation EgressRouteObservation
}

// The standard synchronous response decoder owns no independent socket. An
// ordinary Close error rejects its response; an interrupted Close retains the
// owner. preflightBody still makes only one original Close attempt.
type egressDecodedBody struct {
	io.ReadCloser
	returned bool
	err      error
}

func (body *egressDecodedBody) Close() error {
	err := body.ReadCloser.Close()
	body.err, body.returned = err, true
	return err
}

type egressObservationOwner struct {
	network                                        preflightNetwork
	ctx                                            context.Context
	cancel                                         context.CancelFunc
	release                                        func()
	deadline                                       time.Time
	pending, returned                              bool
	watchDone                                      chan struct{}
	watchReturned                                  bool // immutable after watchDone closes
	cancelReturned, closeReturned, releaseReturned bool
	responseRejected                               bool
	stopRequested                                  chan struct{}
	stopOnce                                       sync.Once
	done                                           chan struct{}
	retired                                        bool // immutable after done closes
	addresses                                      *egressAddressQuery
}

var egressObservations struct {
	sync.Mutex
	owner *egressObservationOwner
}

// Only the two compiled HTTPS targets and the compiled mixed listener are
// accepted. The callback is a negative, value-only caller fence; no I/O or
// lifecycle lock is held when it runs. Timeout never permits owner replacement.
func ObserveEgressHTTPS(parent context.Context, osControl bool, socksAddress string, current func() bool) (facts EgressHTTPSObservation, result error) {
	facts = newEgressObservation(osControl, socksAddress != "")
	if contextError(parent) != nil || current == nil || !egressSourceCurrent(current) || !osControl && socksAddress == "" || socksAddress != "" && socksAddress != egressSOCKSAddress {
		rejectEgressObservation(&facts, osControl, socksAddress != "", egressContextOutcome(parent))
		return facts, ErrEgressObservation
	}
	if !egressObservations.TryLock() {
		rejectEgressObservation(&facts, osControl, socksAddress != "", EgressBusy)
		return facts, ErrEgressObservation
	}
	if egressObservations.owner != nil {
		outcome := EgressBusy
		select {
		case <-egressObservations.owner.done:
			outcome = EgressCleanupUnconfirmed
		default:
		}
		egressObservations.Unlock()
		rejectEgressObservation(&facts, osControl, socksAddress != "", outcome)
		return facts, ErrEgressObservation
	}
	owner := &egressObservationOwner{stopRequested: make(chan struct{}), done: make(chan struct{})}
	egressObservations.owner = owner // before context, budget, watcher or dial acquisition
	egressObservations.Unlock()
	defer func() {
		egressObservations.Lock()
		if egressObservations.owner != owner {
			owner.retired = false
		} else if owner.retired {
			egressObservations.owner = nil
		}
		close(owner.done) // same final metadata frame; no successor before this fence
		egressObservations.Unlock()
	}()
	defer func() {
		if !owner.returned {
			_ = recover()
		}
		valid := owner.returned && owner.ctx != nil && contextError(owner.ctx) == nil && egressSourceCurrent(current)
		denied := egressContextOutcome(owner.ctx)
		if err := owner.retire(); err != nil {
			rejectEgressObservation(&facts, osControl, socksAddress != "", EgressCleanupUnconfirmed)
			result = ErrEgressObservation
			return // original graph and optional budget remain owned
		}
		owner.retired = true
		if owner.ctx == nil {
			return
		} // preserve explicit admission/budget outcome; no attempt was created
		if !time.Now().Before(owner.deadline) {
			denied = EgressTimeout
		}
		if !valid || owner.stopped() || owner.responseRejected || contextError(parent) != nil || !time.Now().Before(owner.deadline) || !egressSourceCurrent(current) {
			if owner.stopped() || !egressSourceCurrent(current) {
				denied = EgressCanceled
			}
			if owner.responseRejected {
				denied = EgressInvalidResponse
			}
			rejectEgressObservation(&facts, osControl, socksAddress != "", denied)
			result = ErrEgressObservation
		}
	}()
	owner.pending = true
	preparation, cancel := context.WithCancel(parent)
	owner.cancel = cancel
	requests := 0
	if osControl {
		requests++
	}
	if socksAddress != "" {
		requests++
	}
	owner.ctx, owner.release, result = AdmitDiagnostic(preparation, DiagnosticBackground, requests, int64(requests*egressPlaintextLimit), egressObservationBudget)
	owner.pending = false
	if result != nil {
		outcome := egressContextOutcome(preparation)
		if errors.Is(result, ErrDiagnosticBudget) && contextError(preparation) == nil {
			outcome = EgressBudget
		}
		rejectEgressObservation(&facts, osControl, socksAddress != "", outcome)
		owner.returned = true
		return facts, ErrEgressObservation
	}
	owner.deadline, _ = owner.ctx.Deadline()
	owner.watchDone = make(chan struct{})
	go owner.watch(current)
	owner.pending = true
	if osControl && owner.current(current) {
		facts.OSControl, facts.OSObservation = owner.fetch("ipapi.co", "/json/", "", egressControlBodyLimit)
	}
	if socksAddress != "" && owner.current(current) {
		facts.SOCKS, facts.SOCKSObservation = owner.fetch("api.ipify.org", "/", socksAddress, egressSOCKSBodyLimit)
	}
	owner.pending, owner.returned = false, true
	return facts, nil
}

func egressSourceCurrent(current func() bool) bool {
	value := false
	err := cleanupCall("egress source fence", func() error { value = current(); return nil })
	return err == nil && value
}

func (owner *egressObservationOwner) current(current func() bool) bool {
	return !owner.stopped() && contextError(owner.ctx) == nil && egressSourceCurrent(current)
}

func (owner *egressObservationOwner) stopped() bool {
	select {
	case <-owner.stopRequested:
		return true
	default:
		return false
	}
}

// Stop joins exactly the captured original task. A timeout is a failure receipt,
// not permission to remove its owner, refund its budget or release protection.
func RetireEgressHTTPS(ctx context.Context) error {
	if contextError(ctx) != nil {
		return ErrEgressObservation
	}
	egressObservations.Lock()
	owner := egressObservations.owner
	egressObservations.Unlock()
	if owner == nil {
		return nil
	}
	owner.stopOnce.Do(func() { close(owner.stopRequested) })
	select {
	case <-owner.done:
		if owner.retired {
			return nil
		}
		return ErrEgressObservation
	case <-ctx.Done():
		select {
		case <-owner.done:
			if owner.retired {
				return nil
			}
		default:
		}
		return ErrEgressObservation
	}
}

// One joinable application watcher. Closing a socket requests interruption; a
// pending dial/Close remains registered and must return before final retirement.
func (owner *egressObservationOwner) watch(current func() bool) {
	defer close(owner.watchDone)
	returned, timerStopped := false, false
	defer func() {
		if !returned || !timerStopped {
			_ = recover()
		}
		owner.watchReturned = returned && timerStopped
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer func() { ticker.Stop(); timerStopped = true }()
	intent, operation, dependency := operationSourceDone(owner.ctx)
	for {
		select {
		case <-owner.ctx.Done():
		case <-intent:
		case <-operation:
		case <-dependency:
		case <-owner.stopRequested:
		case <-ticker.C:
			if owner.current(current) {
				continue
			}
		}
		owner.cancel()
		owner.network.mu.Lock()
		owner.network.closing = true
		connections := append([]*preflightConn(nil), owner.network.conns...)
		owner.network.mu.Unlock()
		// Bodies are not closed concurrently with Read. Final retirement joins
		// this watcher and the synchronous request before closing original bodies.
		for _, conn := range connections {
			_ = conn.Close()
		}
		returned = true
		return
	}
}

func (owner *egressObservationOwner) fetch(host, path, proxyAddress string, limit int64) (raw []byte, fact EgressRouteObservation) {
	fact = EgressRouteObservation{Route: ProbeRouteOS, Outcome: EgressUnavailable}
	if proxyAddress != "" {
		fact.Route = ProbeRouteSOCKS
	}
	if owner.stopped() || contextError(owner.ctx) != nil {
		fact.Outcome = egressContextOutcome(owner.ctx)
		return nil, fact
	}
	ctx, cancel := context.WithTimeout(owner.ctx, egressRequestBudget)
	defer cancel()
	started := time.Now()
	fact.Attempted, fact.StartedAt = true, started.UTC().Format(time.RFC3339Nano)
	defer func() {
		ended := time.Now()
		elapsed := ended.Sub(started).Microseconds()
		fact.completed, fact.CompletedAt, fact.ElapsedMicros = ended, ended.UTC().Format(time.RFC3339Nano), &elapsed
		if contextError(ctx) != nil {
			fact.Outcome = egressContextOutcome(ctx)
			raw = nil
		}
	}()
	dialer := &net.Dialer{Timeout: egressRequestBudget, KeepAlive: -1}
	var conn net.Conn
	var err error
	if proxyAddress != "" {
		fact.Stage = "connect"
		conn, err = owner.network.dialWith(ctx, "tcp4", proxyAddress, dialer.DialContext)
	} else {
		fact.Stage = "dns"
		if owner.addresses != nil {
			return nil, fact
		} // never overwrite an original resolver result/ref
		owner.addresses = &egressAddressQuery{} // before the original OS resolver acquires anything
		addresses, lookupErr := owner.addresses.lookup(ctx, host)
		if lookupErr != nil {
			return nil, fact
		}
		fact.Stage = "connect"
		conn, err = owner.dialControlAddress(ctx, addresses, dialer)
	}
	if err != nil || conn == nil {
		return nil, fact
	}
	// The pinned codec performs only a synchronous CONNECT handshake on this
	// already owned socket; it does not start x/net proxy cancellation workers.
	if proxyAddress != "" {
		fact.Stage = "proxy"
		if _, err = socks.ClientHandshake5(conn, socks5.CommandConnect, M.ParseSocksaddrHostPort(host, 443), "", ""); err != nil {
			return nil, fact
		}
	}
	secure := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	fact.Stage = "tls"
	// Handshake is synchronous. Original TCP deadlines and our joined watcher
	// provide interruption, without an unjoined TLS context.AfterFunc callback.
	if err = secure.Handshake(); err != nil || contextError(ctx) != nil {
		return nil, fact
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+path, nil)
	if err != nil {
		return nil, fact
	}
	req.Close = true
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept", "application/json, text/plain")
	fact.Stage = "request"
	if err = req.Write(secure); err != nil {
		return nil, fact
	}
	fact.Stage = "headers"
	reader := bufio.NewReaderSize(io.LimitReader(secure, egressPlaintextLimit), 4096)
	// Bound the complete header before invoking the standard HTTP decoder.
	header := make([]byte, 0, 1024)
	for len(header) < egressHeaderLimit {
		b, readErr := reader.ReadByte()
		if readErr != nil {
			return nil, fact
		}
		header = append(header, b)
		if bytes.HasSuffix(header, []byte("\r\n\r\n")) {
			break
		}
	}
	if !bytes.HasSuffix(header, []byte("\r\n\r\n")) {
		fact.Outcome = EgressInvalidResponse
		return nil, fact
	}
	response, err := http.ReadResponse(bufio.NewReaderSize(io.MultiReader(bytes.NewReader(header), reader), 4096), req)
	if response == nil || response.Body == nil {
		fact.Outcome = EgressInvalidResponse
		return nil, fact
	}
	body, bodyErr := owner.network.body(&egressDecodedBody{ReadCloser: response.Body}) // retained before inspecting any response error
	fact.HTTPStatus = response.StatusCode
	if err != nil || bodyErr != nil || response.ProtoMajor != 1 || response.ContentLength > limit {
		fact.Outcome = EgressInvalidResponse
		return nil, fact
	}
	if response.StatusCode != http.StatusOK {
		fact.Outcome = EgressHTTPStatus
		return nil, fact
	}
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		fact.Outcome = EgressInvalidResponse
		return nil, fact
	}
	fact.Stage = "body"
	raw, err = io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil || int64(len(raw)) > limit || contextError(ctx) != nil {
		fact.Outcome = EgressInvalidResponse
		return nil, fact
	}
	fact.Stage, fact.Outcome = "complete", EgressResponse
	return raw, fact // not published until original socket/body/watcher retirement below
}

// Preserve finite alternate-address availability without returning a hostname
// to net.Dialer. The original request deadline covers resolution and all dials;
// only normally returned/retired failures can try another captured address.
func (owner *egressObservationOwner) dialControlAddress(ctx context.Context, addresses []string, dialer *net.Dialer) (net.Conn, error) {
	if len(addresses) == 0 || len(addresses) > maxEgressAddresses || dialer == nil {
		return nil, ErrEgressObservation
	}
	for _, value := range addresses {
		ip, err := netip.ParseAddr(value)
		if err != nil || !ip.Is4() || value != ip.String() || !publicProbeIPv4(net.IP(ip.AsSlice())) {
			return nil, ErrEgressObservation
		}
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return nil, ErrEgressObservation
	}
	for index, address := range addresses {
		if contextError(ctx) != nil || owner.stopped() {
			return nil, ErrEgressObservation
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ErrEgressObservation
		}
		attempt, cancel := context.WithTimeout(ctx, remaining/time.Duration(len(addresses)-index))
		conn, err := owner.network.dialWith(attempt, "tcp4", net.JoinHostPort(address, "443"), dialer.DialContext)
		cancel()
		if err != nil {
			if errors.Is(err, ErrPreflightCleanup) || errors.Is(err, ErrPreflightBusy) || errors.Is(err, ErrPreflightScope) {
				return nil, ErrEgressObservation
			}
			continue
		}
		if conn == nil || contextError(ctx) != nil || conn.SetDeadline(deadline) != nil {
			return nil, ErrEgressObservation
		}
		return conn, nil
	}
	return nil, ErrEgressObservation
}

func (owner *egressObservationOwner) networkRetired() bool {
	owner.network.mu.Lock()
	defer owner.network.mu.Unlock()
	for _, conn := range owner.network.conns {
		if !conn.mu.TryLock() {
			return false
		}
		complete := !conn.acquiring && conn.closed && conn.closeErr == nil
		conn.mu.Unlock()
		if !complete {
			return false
		}
	}
	for _, body := range owner.network.bodies {
		if !body.mu.TryLock() {
			return false
		}
		decoder, known := body.ReadCloser.(*egressDecodedBody)
		complete := body.closed && known && decoder.returned && (body.closeErr == nil && decoder.err == nil || decoder.err != nil && errors.Is(body.closeErr, decoder.err))
		body.mu.Unlock()
		if !complete {
			return false
		}
	}
	return true
}

func (owner *egressObservationOwner) retire() (result error) {
	result = ErrEgressObservation
	receipt := &lifecycleCleanup{}
	defer func() {
		if receipt.result == nil && owner.returned && !owner.pending && owner.cancelReturned && owner.closeReturned && (owner.release == nil || owner.releaseReturned) {
			result = nil
		}
	}()
	defer lifecycleCleanupStep("egress optional budget", receipt, func() error {
		if receipt.result != nil || !owner.returned || owner.pending || !owner.cancelReturned || !owner.closeReturned || !owner.networkRetired() {
			return ErrEgressObservation
		}
		if owner.release != nil {
			owner.release()
			owner.releaseReturned = true
		}
		return nil
	})
	defer lifecycleCleanupStep("egress original OS address query", receipt, func() error {
		if owner.addresses == nil {
			return nil
		}
		return owner.addresses.retire()
	})
	defer lifecycleCleanupStep("egress original socket and response", receipt, func() error {
		if owner.pending || !owner.returned {
			return ErrEgressObservation
		}
		err := owner.network.close()
		owner.closeReturned = true
		if !owner.networkRetired() {
			return errors.Join(ErrEgressObservation, err)
		}
		owner.responseRejected = err != nil // decoder failure after original sockets confirmed closed
		return nil
	})
	defer lifecycleCleanupStep("egress original watcher", receipt, func() error {
		if owner.watchDone == nil {
			return nil
		}
		<-owner.watchDone
		if !owner.watchReturned {
			return ErrEgressObservation
		}
		return nil
	})
	defer lifecycleCleanupStep("egress original cancellation", receipt, func() error {
		if owner.cancel == nil {
			if owner.pending {
				return ErrEgressObservation
			}
			owner.cancelReturned = true
			return nil
		}
		owner.cancel()
		owner.cancelReturned = true
		return nil
	})
	return result
}
