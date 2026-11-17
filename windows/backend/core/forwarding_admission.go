package core

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/protocol/group"
	tun "github.com/sagernet/sing-tun"
	N "github.com/sagernet/sing/common/network"
)

var ErrForwardingNotAdmitted = errors.New("user traffic has no current protected forwarding admission")

// The box starts closed. Library Start, configured/live labels and a successful
// UI report cannot open it. Activation must come from an original-worker,
// subject-bound diagnostic receipt through the original private worker pipe.
// There is deliberately no public boolean setter or data-to-permit constructor.
type forwardingAdmission struct {
	open                        atomic.Bool
	mu                          sync.Mutex
	epoch                       uint64
	closed                      bool
	subject, selector, pending  string
	pendingUntil                time.Time
	pendingIssue                *forwardingIssue // original ordinary issuance, before token publication
	tunScope                    *tunDiagnosticScope
	nativeReceipt               *workerNativeTUNReceipt  // one completed actual flow, not forwarding authority
	nativePending               *workerTUNPreparedPermit // prepared only; ordinary consume cannot open it
	nativeConsuming             *workerTUNConsumption    // exact attempt after original prepared slot was burned
	routes                      map[*forwardingRoute]struct{}
	routeStop                   chan struct{}
	routeFault                  bool // permanent local custody fault; never a channel-health result
	rejections                  map[*forwardingRejection]struct{}
	rejectionsSealed            bool
	workerCustody               *workerIngressCustody // private original worker-only negative exit capability
	mixedInputs                 map[*mixedIngress]struct{}
	ownedDNSPort53              bool // immutable exact first-rule qualification from original normalized config
	dnsQueries                  map[*forwardingDNSQuery]struct{}
	dnsWireBytes, dnsExtraBytes int
	dnsChanged                  chan struct{}
	mixedRates                  mixedIngressRates // immutable two-bucket configuration; no traffic authority
	closeScope                  *forwardingClose  // original sealed cohort, published before cancellation callbacks
	drainOwner                  *forwardingDrain  // one original post-SDK drain; retained on failure/pending
}

func newForwardingAdmission() *forwardingAdmission {
	return &forwardingAdmission{epoch: 1, mixedRates: newMixedIngressRates()}
}

func (admission *forwardingAdmission) state() (uint64, bool) {
	admission.mu.Lock()
	defer admission.mu.Unlock()
	return admission.epoch, admission.open.Load() && !admission.closed
}

func (admission *forwardingAdmission) consume(subject, selector string, epoch uint64, token string) error {
	admission.mu.Lock()
	defer admission.mu.Unlock()
	pending, until := admission.pending, admission.pendingUntil
	admission.clearPendingLocked() // burn both lanes before every outcome, including wrong subject/epoch
	if admission.closed || admission.epoch != epoch || admission.subject != subject || admission.selector != selector {
		return ErrForwardingNotAdmitted
	}
	if admission.open.Load() {
		if token == "" {
			return nil
		} // idempotent confirmation of existing authority
		return ErrForwardingNotAdmitted
	}
	if !workerSubject(token) || pending != token || !time.Now().Before(until) {
		return ErrForwardingNotAdmitted
	}
	admission.open.Store(true)
	return nil
}

func (admission *forwardingAdmission) close() error {
	closing := admission.beginClose()
	cleanup, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	return admission.finishClose(cleanup, closing)
}

// Wrap the existing registry, preserving its option decoders/constructors and
// box lifecycle. Every supported user inbound receives the guarded router;
// unknown inbound families cannot acquire an unguarded constructor fallback.
type admissionInboundRegistry struct {
	adapter.InboundRegistry
	admission *forwardingAdmission
}

type selectorControl interface {
	Now() string
	SelectOutbound(string) bool
}

// Also guard the actual selector exposed to native control APIs. Merely
// rejecting Engine.SwitchDefault would leave another SDK selector caller able
// to change the admitted subject. Stock selector initialization remains intact.
type admissionOutboundRegistry struct {
	adapter.OutboundRegistry
	admission *forwardingAdmission
	bootstrap *bootstrapWorker
}

func (registry *admissionOutboundRegistry) CreateOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag, outboundType string, options any) (adapter.Outbound, error) {
	if registry.bootstrap != nil {
		if spec := registry.bootstrap.services[tag]; spec != nil {
			return registry.bootstrap.newServiceDialOutbound(ctx, tag, outboundType, options, spec)
		}
		if serviceDialReserved(tag) {
			return nil, ErrPreflightScope // never delegate an unknown private tag to stock direct
		}
		var err error
		ctx, err = registry.bootstrap.outboundContext(ctx, tag, outboundType, options)
		if err != nil {
			return nil, err
		}
	}
	outbound, err := registry.OutboundRegistry.CreateOutbound(ctx, router, logger, tag, outboundType, options)
	if err != nil || outboundType != "selector" {
		return outbound, err
	}
	selector, ok := outbound.(*group.Selector)
	if !ok || registry.admission == nil {
		return nil, ErrForwardingNotAdmitted
	}
	return &admissionSelector{Selector: selector, admission: registry.admission, ctx: ctx}, nil
}

type admissionSelector struct {
	*group.Selector
	admission *forwardingAdmission
	ctx       context.Context // actual SDK constructor services, owned by this box
}

func (selector *admissionSelector) SelectOutbound(tag string) bool {
	admission := selector.admission
	admission.mu.Lock()
	defer admission.mu.Unlock()
	if admission.closed {
		return false
	}
	if selector.Selector.Now() == tag {
		return true
	}
	// Keep already delegated flows bound to the admitted old subject. A full
	// owned reload/retirement is required until safe seamless custody exists.
	if admission.open.Load() || admission.tunScope != nil || admission.epoch == ^uint64(0) {
		return false
	}
	admission.epoch++
	admission.subject, admission.selector = "", ""
	admission.clearPendingLocked()
	admission.nativeReceipt = nil
	return selector.Selector.SelectOutbound(tag)
}

func (registry *admissionInboundRegistry) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag, inboundType string, options any) (adapter.Inbound, error) {
	if registry.InboundRegistry == nil || registry.admission == nil || router == nil || tag == "" {
		return nil, ErrForwardingNotAdmitted
	}
	if inboundType != "mixed" && inboundType != "tun" {
		return nil, errors.New("inbound family has no protected forwarding adapter")
	}
	guarded := admissionRouter{Router: router, admission: registry.admission, tag: tag, inboundType: inboundType}
	if inboundType == "mixed" {
		return newOwnedMixedInbound(ctx, guarded, logger, tag, options)
	}
	return registry.InboundRegistry.Create(ctx, guarded, logger, tag, inboundType, options)
}

type admissionRouter struct {
	adapter.Router
	admission        *forwardingAdmission
	tag, inboundType string
}

var (
	_ boxInstance              = (*admissionBox)(nil)
	_ adapter.InboundRegistry  = (*admissionInboundRegistry)(nil)
	_ adapter.OutboundRegistry = (*admissionOutboundRegistry)(nil)
	_ adapter.Outbound         = (*admissionSelector)(nil)
	_ adapter.Router           = admissionRouter{}
)

func (router admissionRouter) allowed(metadata adapter.InboundContext) error {
	if router.admission == nil || metadata.Inbound != router.tag || metadata.InboundType != router.inboundType {
		return ErrForwardingNotAdmitted
	}
	if router.admission.open.Load() {
		return nil
	}
	router.admission.mu.Lock()
	defer router.admission.mu.Unlock()
	if !router.admission.closed && router.inboundType == "tun" && router.admission.tunScope.matches(metadata, router.admission.epoch) {
		return nil
	}
	return ErrForwardingNotAdmitted
}

func (router admissionRouter) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	flow, err := router.scopedTCP(ctx, conn, metadata, nil)
	if err != nil {
		return router.rejectIngress(conn, nil, err)
	}
	if flow != nil {
		return flow.wait(ctx)
	}
	route, err := router.registerRoute(ctx, conn, metadata, nil)
	if err != nil {
		return router.rejectIngress(conn, nil, err)
	}
	guarded := &forwardingTCPConnection{forwardingHandshake: forwardingHandshake{route}, conn: conn}
	route.delegate(func(lifetime context.Context, callback N.CloseHandlerFunc) {
		router.dispatchTCP(route, lifetime, guarded, metadata, callback)
	})
	return route.wait(ctx)
}

func (router admissionRouter) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	if conn == nil {
		return router.rejectIngress(nil, nil, ErrForwardingNotAdmitted)
	}
	if err := errors.Join(contextError(ctx), router.allowedPacket(metadata)); err != nil {
		return router.rejectIngress(conn, nil, err)
	}
	route, err := router.registerRoute(ctx, conn, metadata, nil)
	if err != nil {
		return router.rejectIngress(conn, nil, err)
	}
	guarded := &forwardingPacketConnection{forwardingHandshake: forwardingHandshake{route}, conn: conn}
	route.delegate(func(lifetime context.Context, callback N.CloseHandlerFunc) {
		router.dispatchPacket(route, lifetime, guarded, metadata, callback)
	})
	return route.wait(ctx)
}

func (router admissionRouter) RouteConnectionEx(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	flow, err := router.scopedTCP(ctx, conn, metadata, onClose)
	if err != nil {
		_ = router.rejectIngress(conn, onClose, err)
		return
	}
	if flow != nil {
		return // the retained owner completes the original callback after both I/O tasks
	}
	route, err := router.registerRoute(ctx, conn, metadata, onClose)
	if err != nil {
		_ = router.rejectIngress(conn, onClose, err)
		return
	}
	guarded := &forwardingTCPConnection{forwardingHandshake: forwardingHandshake{route}, conn: conn}
	route.delegate(func(lifetime context.Context, callback N.CloseHandlerFunc) {
		router.dispatchTCP(route, lifetime, guarded, metadata, callback)
	})
}

func (router admissionRouter) RoutePacketConnectionEx(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	if conn == nil {
		_ = router.rejectIngress(nil, onClose, ErrForwardingNotAdmitted)
		return
	}
	if err := errors.Join(contextError(ctx), router.allowedPacket(metadata)); err != nil {
		_ = router.rejectIngress(conn, onClose, err)
		return
	}
	route, err := router.registerRoute(ctx, conn, metadata, onClose)
	if err != nil {
		_ = router.rejectIngress(conn, onClose, err)
		return
	}
	guarded := &forwardingPacketConnection{forwardingHandshake: forwardingHandshake{route}, conn: conn}
	route.delegate(func(lifetime context.Context, callback N.CloseHandlerFunc) {
		router.dispatchPacket(route, lifetime, guarded, metadata, callback)
	})
}

func (router admissionRouter) allowedPacket(metadata adapter.InboundContext) error {
	if router.admission == nil || !router.admission.open.Load() || metadata.Network != "" && metadata.Network != N.NetworkUDP {
		return ErrForwardingNotAdmitted
	}
	return router.allowed(metadata)
}

func (router admissionRouter) PreMatch(metadata adapter.InboundContext, _ tun.DirectRouteContext, _ time.Duration, _ bool) (tun.DirectRouteDestination, error) {
	var throughStack tun.DirectRouteDestination
	// Only the typed TCP/UDP handlers below have a protected route adapter.
	// ICMP/unknown families need their own original resource/revoke contract;
	// a general forwarding flag cannot authorize an unguarded native route.
	if metadata.Network != N.NetworkTCP && metadata.Network != N.NetworkUDP {
		return throughStack, ErrForwardingNotAdmitted
	}
	if err := router.allowed(metadata); err != nil {
		return throughStack, err
	}
	// Pinned SDK PreMatch can create DirectRouteDestination for an explicit
	// route/bypass outbound even with supportBypass=false, or default ICMP.
	// Do not invoke it here. The actual TCP/packet Router still evaluates its
	// existing policy after this through-stack preparation; narrow diagnostic
	// TCP keeps its captured outbound. No native destination is created here.
	return throughStack, nil
}
