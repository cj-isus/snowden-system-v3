package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/metadata"
)

const (
	maxBootstrapCallers    = 16
	maxBootstrapAddresses  = 16
	bootstrapLookupTimeout = 6 * time.Second
)

// One original per-box domain, not a process-global worker cache. The App's
// preflight domain remains independent. No timer refreshes established flows.
type bootstrapWorker struct {
	ctx              context.Context
	cancel           context.CancelFunc
	admission        *forwardingAdmission
	source           string
	hints            map[string]config.BootstrapHint // immutable, already owned by strict Parse
	clock            metadata.ObservedClock
	work             preflightWorkDomain
	mu               sync.Mutex
	flight           *bootstrapFlight
	path             *bootstrapPath
	paused           bool
	callers          int // includes followers of already-completed flights until their return
	drained          chan struct{}
	drainClosed      bool
	closed           bool
	fault            error
	localEpoch       uint64
	localFault       error
	dnsServer        *config.DNSServer               // immutable normalized service-bootstrap role only
	dnsDetour        string                          // SDK view, not the normalized subject
	transport        *bootstrapDNSTransport          // original per-box owner, guarded by mu
	services         map[string]*serviceDialSpec     // immutable normalized source projections, no credentials
	serviceDials     map[*serviceDialTask]struct{}   // original pending calls, guarded by mu
	serviceOutbounds map[string]*serviceDialOutbound // original constructor owners, guarded by mu
}

type bootstrapFlight struct {
	path           *bootstrapPath
	host, endpoint string
	source         string
	done           chan struct{}
	addresses      []netip.Addr
	address        PreflightAddress // only the existing resolver has a DNS TTL receipt
	until          time.Time        // steady processing deadline, not a claimed fallback DNS TTL
	err            error
}

func newBootstrapWorker(ctx context.Context, admission *forwardingAdmission, source string, cfg *config.Config) *bootstrapWorker {
	lifetime, cancel := context.WithCancel(ctx)
	worker := &bootstrapWorker{ctx: lifetime, cancel: cancel, admission: admission, source: source, hints: cfg.BootstrapHints, drained: make(chan struct{}), path: newBootstrapPath(lifetime)}
	_, worker.services, worker.fault = lowerServiceDialers(cfg)
	worker.serviceDials = make(map[*serviceDialTask]struct{})
	worker.serviceOutbounds = make(map[string]*serviceDialOutbound)
	if worker.fault != nil {
		cancel()
	}
	for _, server := range cfg.DNS.Servers {
		if server.Tag == config.BootstrapDNSTag {
			copy := server
			worker.dnsServer, worker.dnsDetour = &copy, server.Detour
			if plainBootstrapDirect(cfg) {
				worker.dnsDetour = ""
			}
		}
	}
	return worker
}

func bootstrapEndpointDigest(tag, kind, host string) string {
	raw, _ := json.Marshal([3]string{tag, kind, host}) // fixed string array has no failing marshaler
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (worker *bootstrapWorker) now() (time.Time, error) { return worker.clock.Now(time.Time{}) }

func (worker *bootstrapWorker) current(ctx context.Context) bool {
	return worker.currentPath(ctx, worker.pathSource())
}

func (worker *bootstrapWorker) failure() error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.fault
}

// Selector-default changes alter the full runtime subject. Update the local
// address namespace only at the existing closed/quiescent selector boundary;
// never relabel an in-flight result or transfer an old forwarding permission.
func (worker *bootstrapWorker) bindSource(source string) error {
	worker.admission.mu.Lock()
	defer worker.admission.mu.Unlock()
	if worker.admission.closed || worker.admission.open.Load() || worker.admission.tunScope != nil {
		return ErrForwardingNotAdmitted
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.closed || worker.paused || worker.fault != nil || worker.flight != nil || worker.callers != 0 {
		return errors.Join(ErrPreflightBusy, worker.fault)
	}
	worker.source = source
	return nil
}

func (worker *bootstrapWorker) faultStamp() uint64 {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	return worker.localEpoch
}

func (worker *bootstrapWorker) localSince(stamp uint64) error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if stamp != worker.localEpoch {
		return errors.Join(worker.fault, worker.localFault)
	}
	return worker.fault
}

// Record only the most recent bounded local fact. A diagnostic snapshots the
// counter before work so HTTP's string errors cannot turn local budget/clock/
// cancellation failures into channel health evidence. No retained error chain.
func (worker *bootstrapWorker) noteLocal(path *bootstrapPath, err error) {
	if err == nil || !(errors.Is(err, ErrPreflightScope) || errors.Is(err, ErrPreflightBusy) ||
		errors.Is(err, ErrPreflightCleanup) || errors.Is(err, ErrPreflightAddressExpired) ||
		errors.Is(err, metadata.ErrClockUncertain) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return
	}
	worker.mu.Lock()
	if path == nil || worker.path != path {
		worker.mu.Unlock()
		return
	}
	worker.localFault = err
	if worker.localEpoch == ^uint64(0) {
		worker.mu.Unlock()
		worker.poison(ErrPreflightScope)
		return
	}
	worker.localEpoch++
	worker.mu.Unlock()
}

// Negative-only transition. Do not wait for native I/O under admission.mu or
// call the public Close recursively from the resolver that is being retired.
func (worker *bootstrapWorker) poison(err error) {
	worker.mu.Lock()
	if worker.fault == nil {
		worker.fault = errors.Join(ErrPreflightCleanup, err)
	}
	worker.mu.Unlock()
	worker.cancel()
	admission := worker.admission
	admission.open.Store(false)
	admission.mu.Lock()
	admission.closed = true
	admission.open.Store(false)
	admission.clearPendingLocked()
	admission.nativeReceipt = nil
	if admission.tunScope != nil {
		admission.tunScope.deactivateLocked()
	}
	admission.mu.Unlock()
}

func (worker *bootstrapWorker) lookup(ctx context.Context, host, endpoint string, fallback func(context.Context) ([]netip.Addr, error)) (addresses []netip.Addr, finalErr error) {
	path := worker.pathSource()
	borrowed := false
	defer func() {
		if borrowed && finalErr == nil {
			worker.mu.Lock()
			if worker.closed || worker.paused || worker.fault != nil || worker.path != path || !path.live {
				addresses, finalErr = nil, ErrPreflightScope
			}
			worker.mu.Unlock()
		}
		worker.noteLocal(path, finalErr) // before borrower retirement; old paths cannot poison a new diagnostic
		if borrowed {
			worker.mu.Lock()
			worker.callers--
			path.callers--
			path.signalDrainedLocked()
			worker.signalDrainedLocked()
			worker.mu.Unlock()
		}
	}()
	if !worker.currentPath(ctx, path) {
		return nil, errors.Join(ErrPreflightScope, worker.failure(), contextError(ctx))
	}
	worker.mu.Lock()
	if worker.closed || worker.paused || worker.fault != nil || worker.path != path || path == nil || !path.live {
		err := errors.Join(ErrPreflightScope, worker.fault)
		worker.mu.Unlock()
		return nil, err
	}
	if worker.callers >= maxBootstrapCallers {
		worker.mu.Unlock()
		return nil, ErrPreflightBusy
	}
	flight := worker.flight
	leader := flight == nil
	if leader {
		flight = &bootstrapFlight{path: worker.path, host: host, endpoint: endpoint, source: worker.source, done: make(chan struct{}), until: time.Now().Add(bootstrapLookupTimeout)}
		worker.flight = flight
	} else if flight.path != path || flight.host != host || flight.endpoint != endpoint {
		worker.mu.Unlock()
		return nil, ErrPreflightBusy // bounded local budget, never an endpoint health penalty
	}
	worker.callers++
	flight.path.callers++
	borrowed = true
	worker.mu.Unlock()
	if leader {
		worker.resolve(ctx, flight, fallback) // synchronous leader; no detached resolver owner
	} else {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-flight.path.ctx.Done():
			return nil, errors.Join(ErrPreflightScope, worker.failure())
		case <-flight.done:
		}
	}
	if flight.err != nil {
		return nil, flight.err
	}
	if !worker.currentPath(ctx, flight.path) || !time.Now().Before(flight.until) {
		return nil, ErrPreflightScope
	}
	now, err := worker.now()
	if err != nil {
		return nil, err
	}
	if flight.address.Address != "" && !flight.address.ValidAt(now) {
		return nil, ErrPreflightAddressExpired
	}
	if !worker.currentPath(ctx, path) {
		return nil, ErrPreflightScope
	}
	return append([]netip.Addr(nil), flight.addresses...), nil
}

func (worker *bootstrapWorker) resolve(parent context.Context, flight *bootstrapFlight, fallback func(context.Context) ([]netip.Addr, error)) {
	ctx, cancel := context.WithDeadline(parent, flight.until)
	linked := make(chan struct{})
	stop := context.AfterFunc(flight.path.ctx, func() { cancel(); close(linked) })
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			flight.err = ErrPreflightCleanup // a panic cannot assert that the original work retired
		}
		cancel()
		if !stop() {
			<-linked
		} // cancellation callback retirement, not a fire-and-forget link
		if errors.Is(flight.err, ErrPreflightCleanup) {
			worker.poison(flight.err)
		}
		worker.mu.Lock()
		mismatched := worker.flight != flight
		if mismatched {
			flight.err = errors.Join(flight.err, ErrPreflightCleanup)
			if worker.fault == nil {
				worker.fault = flight.err
			}
		} else {
			worker.flight = nil
		}
		close(flight.done) // after ALL resolver workers and the linked cancellation callback returned
		worker.mu.Unlock()
		if mismatched {
			worker.poison(flight.err)
		}
	}()
	resolver := PreflightResolver{work: &worker.work, Now: worker.now, Current: func() bool { return worker.currentPath(ctx, flight.path) },
		Scope: PreflightScope{SourceSHA256: flight.source, EndpointSHA256: flight.endpoint, Role: BootstrapWorkerService}}
	address, err := resolver.ResolveAddress(ctx, flight.host)
	if err == nil {
		ip, parseErr := netip.ParseAddr(address.Address)
		flight.address, flight.addresses, flight.err = address, []netip.Addr{ip}, parseErr
	} else if errors.Is(err, ErrPreflightScope) || errors.Is(err, ErrPreflightCleanup) || errors.Is(err, ErrPreflightBusy) ||
		errors.Is(err, metadata.ErrClockUncertain) || contextError(ctx) != nil || !worker.currentPath(ctx, flight.path) {
		flight.err = errors.Join(err, contextError(ctx))
	} else {
		// Only an ordinary inability to obtain a fresh IPv4 hint may use the
		// original no-cache resolver (including its IPv6/family behavior). This
		// interface has no TTL receipt: share only this in-flight result, never cache it.
		addresses, fallbackErr := fallback(ctx)
		if fallbackErr == nil && len(addresses) > 0 {
			flight.addresses = append([]netip.Addr(nil), addresses[:min(len(addresses), maxBootstrapAddresses)]...)
		} else {
			flight.err = errors.Join(err, fallbackErr)
		}
	}
	if !worker.currentPath(ctx, flight.path) || contextError(ctx) != nil {
		flight.err = errors.Join(flight.err, ErrPreflightScope, contextError(ctx))
	}
	if _, err := worker.now(); err != nil {
		flight.err = errors.Join(flight.err, err)
	}
	completed = true
}

func (worker *bootstrapWorker) beginClose() {
	worker.mu.Lock()
	worker.closed = true
	worker.paused = true
	if worker.path != nil {
		worker.path.live = false
		worker.path.signalDrainedLocked()
	}
	worker.signalDrainedLocked()
	worker.mu.Unlock()
	worker.cancel()
}

func (worker *bootstrapWorker) signalDrainedLocked() {
	if worker.closed && worker.callers == 0 && !worker.drainClosed {
		worker.drainClosed = true
		close(worker.drained)
	}
}

func (worker *bootstrapWorker) close() error {
	worker.beginClose()
	ctx, cancel := context.WithTimeout(context.Background(), workerCloseTimeout)
	defer cancel()
	if err := worker.retireServiceDials(ctx, nil); err != nil {
		worker.poison(err)
		return worker.failure()
	}
	if err := worker.retireDNSQueries(ctx, true); err != nil {
		worker.poison(err)
		return worker.failure()
	}
	worker.mu.Lock()
	flight := worker.flight
	worker.mu.Unlock()
	if flight != nil {
		select {
		case <-flight.done:
		case <-ctx.Done():
			worker.poison(ErrPreflightCleanup)
			return worker.failure() // original flight/domain retained until Box/Job retirement
		}
	}
	select {
	case <-worker.drained:
	case <-ctx.Done():
		worker.poison(ErrPreflightCleanup)
		return worker.failure()
	}
	if !worker.work.mu.TryLock() {
		worker.poison(ErrPreflightCleanup)
		return worker.failure()
	}
	retained := worker.work.owner != nil
	worker.work.mu.Unlock()
	if retained {
		worker.poison(ErrPreflightCleanup)
	}
	return worker.failure()
}
