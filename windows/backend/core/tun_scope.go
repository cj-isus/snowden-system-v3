package core

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
)

// Private original-parent pipe only. A numeric handle, PID, decoded message or
// nonce is not a public permit. Parent custody validates the pinned helper and
// duplicates its ORIGINAL process object into the ORIGINAL engine process.
type workerTUNScopeRequest struct {
	ActorHandle   uint64 `json:"actor_handle"`
	ActorPID      uint32 `json:"actor_pid"`
	ActorCreated  uint64 `json:"actor_created"`
	Nonce         string `json:"nonce"`
	SourcePort    uint16 `json:"source_port"`
	TimeoutMillis int    `json:"timeout_millis"`
}

func (request *workerTUNScopeRequest) valid() bool {
	return request != nil && request.ActorHandle != 0 && request.ActorPID != 0 && request.ActorCreated != 0 && workerSubject(request.Nonce) && request.SourcePort != 0 && request.TimeoutMillis >= 500 && request.TimeoutMillis <= 8000
}

type tunActorResource interface {
	adopt(workerTUNScopeRequest) error
	alive() bool
	Close() error
}

type tunDiagnosticScope struct {
	subject, selector, nonce string
	inbound                  string
	epoch                    uint64
	port                     uint16
	until                    time.Time
	active                   bool
	actor                    tunActorResource
	selectorOwner            *admissionSelector
	outbound                 adapter.Outbound
	flow                     *nativeTUNFlow
	cleanupMu                sync.Mutex
	timer                    *time.Timer
	expired                  bool
}

// Caller holds admission.mu. No waiting for flow/callback retirement here.
func (scope *tunDiagnosticScope) deactivateLocked() {
	scope.active = false
	if scope.timer != nil {
		scope.timer.Stop()
	}
	if scope.flow != nil {
		scope.flow.cancel()
	}
}

// Call only after deactivation, OUTSIDE admission.mu. The flow/callback may
// still be returning into SDK code. Keep the actor until original I/O is done.
func (scope *tunDiagnosticScope) close() error {
	scope.cleanupMu.Lock()
	defer scope.cleanupMu.Unlock()
	if scope.flow != nil {
		if err := scope.flow.retire(); err != nil {
			return err
		}
	}
	if scope.actor == nil {
		return ErrNativeScopeUnavailable
	}
	return scope.actor.Close()
}

// Typed TCP handlers may receive empty Network from the pinned TUN inbound;
// PreMatch carries it explicitly. Do not infer TCP from a packet/UDP handler.
func (router admissionRouter) scopedTCP(ctx context.Context, connection net.Conn, metadata adapter.InboundContext, onClose func(error)) (*nativeTUNFlow, error) {
	if contextError(ctx) != nil || connection == nil || router.admission == nil || metadata.Inbound != router.tag || metadata.InboundType != router.inboundType {
		return nil, ErrForwardingNotAdmitted
	}
	if metadata.Network != "" && metadata.Network != "tcp" {
		return nil, ErrForwardingNotAdmitted
	}
	if router.admission.open.Load() {
		return nil, nil // ordinary admitted traffic keeps the stock Router
	}
	if metadata.Network == "" {
		metadata.Network = "tcp"
	}
	gate := router.admission
	gate.mu.Lock()
	defer gate.mu.Unlock()
	scope := gate.tunScope
	if gate.closed || router.inboundType != "tun" || !scope.matches(metadata, gate.epoch) || scope.flow != nil || scope.selectorOwner == nil || scope.outbound == nil || contextError(ctx) != nil {
		return nil, ErrForwardingNotAdmitted
	}
	flowCtx, cancel := context.WithDeadline(ctx, scope.until)
	flow := &nativeTUNFlow{ctx: flowCtx, cancel: cancel, done: make(chan struct{}), ingress: &tunScopedConnection{Conn: connection},
		selector: scope.selectorOwner, selected: scope.selector, outbound: scope.outbound, onClose: onClose,
		metadata: adapter.InboundContext{Inbound: scope.inbound, InboundType: "tun", Network: "tcp", IPVersion: 4, Source: metadata.Source, Destination: metadata.Destination, Outbound: scope.selector}}
	scope.flow = flow // retained BEFORE launching the only original flow owner
	go flow.run()     // retirement waits outside admission.mu, including callback return
	return flow, nil
}

var tunDiagnosticDestination = netip.MustParseAddr("1.1.1.1")

// Caller holds the per-box admission mutex. This is a narrow diagnostic TCP4
// flow, never an ordinary user, UDP, mixed inbound or direct-route bypass grant.
// A user-mode liveness check is not atomic kernel/process attestation.
func (scope *tunDiagnosticScope) matches(metadata adapter.InboundContext, epoch uint64) bool {
	return scope != nil && scope.active && scope.epoch == epoch && time.Now().Before(scope.until) &&
		metadata.Inbound == scope.inbound && metadata.InboundType == "tun" && metadata.Network == "tcp" && metadata.Source.Addr.Is4() && metadata.Source.Port == scope.port &&
		metadata.Destination.Addr.Unmap() == tunDiagnosticDestination && metadata.Destination.Port == 443 && metadata.Destination.Fqdn == "" &&
		scope.actor != nil && scope.actor.alive()
}

// Acquire Engine.mu before admission.mu. Retain the native owner before
// adopting/querying the remote resource; every failure remains restrictive.
func (e *Engine) openLocalTUNScope(ctx context.Context, subject, selected string, epoch uint64, request *workerTUNScopeRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if contextError(ctx) != nil || !request.valid() {
		return ErrNativeScopeUnavailable
	}
	actual, tag, guarded, tunMode, err := e.forwardingSubjectLocked()
	if err != nil || !tunMode || actual != subject || tag != selected {
		return ErrNativeScopeUnavailable
	}
	cfg, _, err := runtimeConfiguration(e.rawJSON)
	if err != nil {
		return ErrNativeScopeUnavailable
	}
	tunTag := ""
	for _, inbound := range cfg.Inbounds {
		if inbound.Type != "tun" {
			continue
		}
		if tunTag != "" || inbound.Tag == "" {
			return ErrNativeScopeUnavailable
		}
		tunTag = inbound.Tag
	}
	if tunTag == "" {
		return ErrNativeScopeUnavailable
	}
	gate := guarded.admission
	selectedType := ""
	for _, outbound := range cfg.Outbounds {
		if outbound.Tag == tag {
			selectedType = outbound.Type // graph was validated by protectedRuntimeConfig at Start
		}
	}
	proxy, exists := e.box.Outbound().Outbound("proxy")
	selectorOwner, wrapped := proxy.(*admissionSelector)
	selectedOutbound, selectedExists := e.box.Outbound().Outbound(tag)
	if !exists || !wrapped || selectorOwner == nil || selectorOwner.admission != gate || selectorOwner.ctx == nil || selectorOwner.Now() != tag || !selectedExists || selectedOutbound == nil || selectedOutbound.Tag() != tag || tag == "proxy" || selectedType == "" || selectedOutbound.Type() != selectedType || !slices.Contains(selectedOutbound.Network(), "tcp") {
		return ErrNativeScopeUnavailable
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if gate.closed || gate.open.Load() || gate.epoch != epoch || gate.tunScope != nil {
		return ErrNativeScopeUnavailable
	}
	gate.nativeReceipt = nil
	scope := &tunDiagnosticScope{subject: actual, selector: tag, nonce: request.Nonce, inbound: tunTag, epoch: epoch, port: request.SourcePort, until: time.Now().Add(time.Duration(request.TimeoutMillis) * time.Millisecond), actor: newTUNActorResource(), selectorOwner: selectorOwner, outbound: selectedOutbound}
	gate.tunScope = scope // native adoption or Close failure cannot lose custody
	gate.clearPendingLocked()
	if scope.actor == nil || scope.actor.adopt(*request) != nil || contextError(ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	scope.active = true
	scope.timer = time.AfterFunc(time.Until(scope.until), func() {
		gate.mu.Lock()
		if gate.tunScope != scope {
			gate.mu.Unlock()
			return
		}
		scope.expired = true
		scope.deactivateLocked()
		gate.mu.Unlock()
		_ = scope.close() // original close receipts remain retained on failure
	})
	return nil
}

func (e *Engine) closeLocalTUNScope(ctx context.Context, subject, selected string, epoch uint64, nonce string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if contextError(ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	actual, tag, guarded, _, err := e.forwardingSubjectLocked()
	if err != nil || actual != subject || tag != selected {
		return ErrNativeScopeUnavailable
	}
	gate := guarded.admission
	gate.mu.Lock()
	scope := gate.tunScope
	if scope == nil || scope.subject != actual || scope.selector != tag || scope.epoch != epoch || gate.epoch != epoch || scope.nonce != nonce {
		gate.mu.Unlock()
		return ErrNativeScopeUnavailable
	}
	gate.clearPendingLocked()
	actorLive := scope.active && scope.actor != nil && scope.actor.alive()
	scope.deactivateLocked()
	gate.mu.Unlock()
	err = scope.close()
	gate.mu.Lock()
	defer gate.mu.Unlock()
	if err != nil || contextError(ctx) != nil || gate.tunScope != scope || gate.closed || gate.epoch != epoch || !actorLive || scope.expired || !time.Now().Before(scope.until) || scope.flow == nil || !scope.flow.proven() {
		return errors.Join(ErrNativeScopeUnavailable, err)
	}
	gate.nativeReceipt = &workerNativeTUNReceipt{subject: actual, selector: tag, nonce: scope.nonce, epoch: epoch, until: time.Now().Add(nativeTUNReceiptLifetime)}
	gate.tunScope = nil
	return nil
}
