// DNS stream orchestration adapted from sing-box v1.13.19/route/dns.go and
// protocol/dns/handle.go. Copyright (C) 2022 by nekohasekai
// <contact-sagernet@sekai.icu>. SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"

	"github.com/snowden-system/windows/backend/config"
)

const (
	maxForwardingDNSQueries      = 128
	maxForwardingDNSScopeQueries = 4
	maxForwardingDNSWireBytes    = 8 << 20
	maxForwardingDNSExtraBytes   = 16 << 20
	maxForwardingDNSMessage      = 65535
	forwardingDNSResponseBase    = maxForwardingDNSMessage + 1 // PackBuffer requires uncompressedLen+1
	maxForwardingDNSExpanded     = 1 << 20
)

var errForwardingDNSProtocol = errors.New("DNS stream message or resource limit rejected")

// Qualify only the exact first unconditional port53 action in the normalized
// runtime. Other policies remain SDK-owned; this is not a second rule engine.
func firstDNSPort53Rule(cfg *config.Config) bool {
	if cfg == nil || cfg.Route == nil || len(cfg.Route.Rules) == 0 || cfg.DNS == nil || len(cfg.DNS.Rules) != 0 {
		return false
	}
	rule := cfg.Route.Rules[0]
	return rule.Action == C.RuleActionTypeHijackDNS && rule.Outbound == "" && rule.Protocol == "" && len(rule.Port) == 1 && rule.Port[0] == 53 && len(rule.IPCidr) == 0 && len(rule.DomainSuffix) == 0 && len(rule.ProcessName) == 0
}

func dnsOriginalMixedConnection(raw io.Closer) *mixedIngressConnection {
	switch conn := raw.(type) {
	case *mixedIngressConnection:
		return conn
	case *mixedPrefetchedConnection:
		if conn != nil {
			return conn.mixedIngressConnection
		}
	case *mixedSOCKSConnection:
		if conn != nil {
			return conn.original
		}
	}
	return nil
}

func (router admissionRouter) dispatchTCP(route *forwardingRoute, ctx context.Context, conn net.Conn, metadata adapter.InboundContext, callback N.CloseHandlerFunc) {
	original := dnsOriginalMixedConnection(route.raw)
	if !route.gate.ownedDNSPort53 || metadata.Destination.Port != 53 || metadata.InboundDetour != "" || original == nil {
		router.Router.RouteConnectionEx(ctx, conn, metadata, callback)
		return
	}
	matched, err := router.firstDNSMetadata(metadata, N.NetworkTCP)
	if err != nil {
		_ = N.CloseOnHandshakeFailure(conn, callback, err)
		return
	}
	dns := service.FromContext[adapter.DNSRouter](ctx)
	scope, err := newForwardingDNSScope(route, original, conn, nil, dns, matched)
	if err == nil {
		err = scope.serve()
	}
	_ = N.CloseOnHandshakeFailure(conn, callback, err)
}

func (router admissionRouter) firstDNSMetadata(metadata adapter.InboundContext, network string) (adapter.InboundContext, error) {
	// Same first SDK rule/action for the exact immutable config qualification;
	// no second policy search or broadening to sniffing, later rules or TUN.
	metadata.Network = network
	metadata.ResetRuleCache()
	rules := router.Router.Rules()
	if len(rules) == 0 || rules[0].Action() == nil || rules[0].Action().Type() != C.RuleActionTypeHijackDNS || !rules[0].Match(&metadata) {
		return metadata, ErrForwardingNotAdmitted
	}
	return metadata, nil
}

type forwardingDNSScope struct {
	route                          *forwardingRoute
	original                       *mixedIngressConnection
	conn                           net.Conn     // guarded route IO/handshakes; original native Close is elsewhere
	packet                         N.PacketConn // mutually exclusive with conn; same original route owner
	dns                            adapter.DNSRouter
	metadata                       adapter.InboundContext
	mu                             sync.Mutex // values only; no IO/SDK/timer/cancel/join while held
	writeMu                        sync.Mutex // serialize complete TCP/UDP response frames; Close never acquires it
	closing, frameReturned, failed bool
	frameDone                      chan struct{}
	queries                        map[*forwardingDNSQuery]struct{}
	idle                           *forwardingDNSIdle // UDP activity watcher; retained before timer construction
}

func newForwardingDNSScope(route *forwardingRoute, original *mixedIngressConnection, conn net.Conn, packet N.PacketConn, dns adapter.DNSRouter, metadata adapter.InboundContext) (*forwardingDNSScope, error) {
	if original == nil || original.owner == nil || original.owner.gate != route.gate || route.ctx.Value(mixedInputKey{}) != original.owner || dns == nil || (conn == nil) == (packet == nil) {
		return nil, ErrForwardingNotAdmitted
	}
	gate := route.gate
	gate.mu.Lock()
	route.mu.Lock()
	_, owned := gate.routes[route]
	if !owned || !gate.open.Load() || gate.closed || gate.routeFault || route.closing || route.dnsSealed || route.dns != nil || route.ctx.Err() != nil || gate.epoch != route.epoch || gate.subject != route.subject || gate.selector != route.selector {
		route.mu.Unlock()
		gate.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	metadata.Destination = M.Socksaddr{} // same DNS-router query context as the pinned hijack handler
	scope := &forwardingDNSScope{route: route, original: original, conn: conn, packet: packet, dns: dns, metadata: metadata, frameDone: make(chan struct{}), queries: make(map[*forwardingDNSQuery]struct{})}
	route.dns = scope // BEFORE reading a header, allocating a message or creating a query task
	route.mu.Unlock()
	gate.mu.Unlock()
	return scope, nil
}

func (scope *forwardingDNSScope) serve() (finalErr error) {
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		scope.mu.Lock()
		scope.frameReturned, scope.failed = returned, scope.failed || fault
		close(scope.frameDone)
		scope.mu.Unlock()
		if fault {
			scope.route.markFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	if scope.packet != nil {
		finalErr = scope.servePackets()
		returned = true
		if E.IsClosedOrCanceled(finalErr) {
			return nil
		}
		return finalErr
	}
	for {
		var header [2]byte
		var query *forwardingDNSQuery
		until := time.Now().Add(C.DNSTimeout)
		err := withMixedHandshakeBudget(scope.route.ctx, scope.original, until, func() error {
			if _, err := io.ReadFull(scope.conn, header[:]); err != nil {
				return err
			}
			length := int(binary.BigEndian.Uint16(header[:]))
			if length == 0 {
				return errForwardingDNSProtocol
			}
			var err error
			query, err = scope.reserve(length)
			return err // capacity wait is inside the original header phase deadline
		})
		if query != nil {
			err = query.consume(until, err)
		}
		if err != nil {
			returned = true
			if E.IsClosedOrCanceled(err) {
				return nil
			}
			return err
		}
	}
}

// Caller holds gate.mu and scope.mu. Capacity pressure is not a channel failure.
func (scope *forwardingDNSScope) currentLocked() bool {
	gate, route := scope.route.gate, scope.route
	_, owned := gate.routes[route]
	return owned && !scope.closing && !gate.closed && !gate.routeFault && gate.open.Load() && gate.epoch == route.epoch && gate.subject == route.subject && gate.selector == route.selector && route.ctx.Err() == nil
}

func (scope *forwardingDNSScope) reserve(length int) (*forwardingDNSQuery, error) {
	gate := scope.route.gate
	charge := length + forwardingDNSResponseBase + scope.responseOverhead()
	for {
		gate.mu.Lock()
		scope.mu.Lock()
		if !scope.currentLocked() {
			scope.mu.Unlock()
			gate.mu.Unlock()
			return nil, ErrForwardingNotAdmitted
		}
		if gate.dnsChanged == nil {
			gate.dnsChanged = make(chan struct{})
		}
		if len(scope.queries) < maxForwardingDNSScopeQueries && len(gate.dnsQueries) < maxForwardingDNSQueries && charge <= maxForwardingDNSWireBytes-gate.dnsWireBytes {
			query := &forwardingDNSQuery{scope: scope, charge: charge, length: length, done: make(chan struct{})}
			if gate.dnsQueries == nil {
				gate.dnsQueries = make(map[*forwardingDNSQuery]struct{})
			}
			scope.queries[query], gate.dnsQueries[query] = struct{}{}, struct{}{} // BEFORE body make/Unpack/go
			gate.dnsWireBytes += charge
			scope.mu.Unlock()
			gate.mu.Unlock()
			return query, nil
		}
		changed := gate.dnsChanged
		scope.mu.Unlock()
		gate.mu.Unlock()
		select {
		case <-changed:
		case <-scope.route.ctx.Done():
			return nil, scope.route.ctx.Err()
		}
	}
}

func (scope *forwardingDNSScope) reserveExtra(query *forwardingDNSQuery, bytes int) error {
	if bytes <= 0 {
		return nil
	}
	gate := scope.route.gate
	for {
		gate.mu.Lock()
		scope.mu.Lock()
		if !scope.currentLocked() {
			scope.mu.Unlock()
			gate.mu.Unlock()
			return ErrForwardingNotAdmitted
		}
		if bytes <= maxForwardingDNSExtraBytes-gate.dnsExtraBytes {
			gate.dnsExtraBytes += bytes
			query.extra = bytes // original task only; retirement waits for its return
			scope.mu.Unlock()
			gate.mu.Unlock()
			return nil
		}
		changed := gate.dnsChanged
		scope.mu.Unlock()
		gate.mu.Unlock()
		select {
		case <-changed:
		case <-scope.route.ctx.Done():
			return scope.route.ctx.Err()
		}
	}
}

func (route *forwardingRoute) sealDNS() *forwardingDNSScope {
	route.mu.Lock()
	route.dnsSealed = true
	scope := route.dns
	route.mu.Unlock()
	if scope != nil {
		scope.mu.Lock()
		scope.closing = true
		scope.mu.Unlock()
	}
	return scope
}

func (scope *forwardingDNSScope) retire() error {
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	if !mixedOriginalDone(ctx, scope.frameDone) {
		scope.mu.Lock()
		scope.failed = true
		scope.mu.Unlock()
		scope.route.markFault()
		return ErrEngineCleanupFailed
	}
	scope.mu.Lock()
	failed := scope.failed || !scope.frameReturned
	idle := scope.idle
	queries := make([]*forwardingDNSQuery, 0, len(scope.queries))
	for query := range scope.queries {
		queries = append(queries, query)
	}
	scope.mu.Unlock()
	if idle != nil {
		if err := idle.retire(ctx); err != nil {
			scope.route.markFault()
			return err
		}
	}
	for _, query := range queries {
		if !mixedOriginalDone(ctx, query.done) {
			query.mu.Lock()
			query.failed = true
			query.mu.Unlock()
			failed = true
			break
		}
		query.mu.Lock()
		failed = failed || query.failed || !query.retireReturned
		query.mu.Unlock()
	}
	if failed {
		scope.route.markFault()
		return ErrEngineCleanupFailed
	}
	return nil
}
