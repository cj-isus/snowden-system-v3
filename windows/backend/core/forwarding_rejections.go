package core

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
)

// Separate bounded custody for rejected inputs; it cannot mint admission or
// consume the ordinary forwarding slots. No per-rejection goroutine is added.
const maxForwardingRejections = maxForwardingRoutes

// This private negative-only capability is created inside the validated worker
// entry, never from JSON/caller context or an App setter. The parent retains the
// original Process/Job and still requires actual exit/I/O/empty Job/handle reap.
type workerIngressCustody struct{}
type workerIngressCustodyKey struct{}

func (*workerIngressCustody) abort() { os.Exit(4) }

type forwardingRejection struct {
	gate                              *forwardingAdmission
	raw                               io.Closer
	onClose                           N.CloseHandlerFunc
	done                              chan struct{}
	alias                             *forwardingRoute
	input                             *mixedIngress
	aliasJoined                       bool
	closeStarted                      bool
	closeReturned                     bool
	callbackStarted, callbackReturned bool
	failed                            bool
	result                            error // fields inspected only after this original done fence
}

func (router admissionRouter) rejectIngress(raw io.Closer, onClose N.CloseHandlerFunc, cause error) error {
	if cause == nil {
		cause = ErrForwardingNotAdmitted
	}
	if raw == nil && onClose == nil {
		return cause // no resource or callback acquisition
	}
	gate := router.admission
	if gate == nil {
		panic(ErrEngineCleanupFailed) // invalid private adapter; no unowned Close fallback
	}
	gate.mu.Lock()
	if gate.rejectionsSealed || len(gate.rejections) >= maxForwardingRejections {
		gate.routeFault = true
		gate.revokeRegistrationLocked()
		worker := gate.workerCustody
		gate.mu.Unlock()
		// Do not create an unbounded overflow owner or silently return after
		// losing the incoming socket. Production embeds the SDK only here.
		stopUntrackedIngress(worker)
	}
	if gate.rejections == nil {
		gate.rejections = make(map[*forwardingRejection]struct{})
	}
	owner := &forwardingRejection{gate: gate, raw: raw, onClose: onClose, done: make(chan struct{})}
	gate.rejections[owner] = struct{}{} // BEFORE original Close/callback/native work
	gate.mu.Unlock()
	return owner.run(cause)
}

func stopUntrackedIngress(worker *workerIngressCustody) {
	if worker != nil {
		worker.abort()
	}
	panic(ErrEngineCleanupFailed) // local harness: no authority to Exit a GUI
}

// PacketWriter consumes its buffer even when no downstream writer was called.
// Record the original memory release before executing it, using the same
// bounded failure/pending custody as rejected sockets; no extra goroutine.
func (gate *forwardingAdmission) rejectPacketBuffer(buffer *buf.Buffer, cause error) error {
	if cause == nil {
		cause = ErrForwardingNotAdmitted
	}
	if buffer == nil {
		return cause // no buffer acquisition
	}
	return (admissionRouter{admission: gate}).rejectIngress(&forwardingBufferRelease{buffer}, nil, cause)
}

type forwardingBufferRelease struct{ buffer *buf.Buffer }

func (owner *forwardingBufferRelease) Close() error {
	owner.buffer.Release() // actual memory release, not a native socket Close proof
	return nil
}

func (owner *forwardingRejection) run(cause error) (finalErr error) {
	complete := false
	defer func() {
		if recovered := recover(); recovered != nil || !complete {
			owner.failed = true
			finalErr = errors.Join(cause, ErrEngineCleanupFailed)
		}
		owner.result = finalErr
		if owner.failed {
			owner.gate.noteRouteFault()
		}
		close(owner.done) // returned stack, not an invented successful native Close
		if !owner.failed {
			owner.gate.mu.Lock()
			delete(owner.gate.rejections, owner)
			owner.gate.mu.Unlock()
		} // failed original raw/callback receipt remains owned by this box
	}()
	var closeErr error
	if owner.raw != nil {
		owner.alias = forwardingIngressOwner(owner.raw)
		if connection, ok := owner.raw.(*mixedIngressConnection); ok {
			owner.input = connection.owner
		}
		if owner.alias != nil || owner.input != nil {
			closeErr = closeForwardingIngress(owner.gate, owner.raw, nil)
			owner.aliasJoined = closeErr == nil
		} else {
			owner.closeStarted = true
			closeErr = owner.raw.Close()
			owner.closeReturned = true
		}
	}
	if closeErr != nil {
		owner.failed, complete = true, true
		return errors.Join(cause, ErrEngineCleanupFailed, closeErr)
	}
	if owner.onClose != nil {
		owner.callbackStarted = true
		owner.onClose(cause) // only after known no-resource or confirmed original Close
		owner.callbackReturned = true
	}
	complete = true
	return cause // negative routing outcome; never forwarding/Ready authority
}

func forwardingIngressOwner(raw io.Closer) *forwardingRoute {
	switch connection := raw.(type) {
	case *forwardingTCPConnection:
		return connection.route
	case *forwardingPacketConnection:
		return connection.route
	default:
		return nil
	}
}

// A wrapper alias already has an original one-attempt close owner. Observe its
// exact outcome rather than interpreting a concurrent pending/ErrClosed return
// as a new native attempt. Foreign/cyclic chains never grant cleanup authority.
func closeForwardingIngress(gate *forwardingAdmission, raw io.Closer, caller *forwardingRoute) error {
	if connection, ok := raw.(*mixedIngressConnection); ok {
		if connection.owner.gate != gate {
			return ErrEngineCleanupFailed
		}
		return connection.owner.joinClose()
	}
	alias := forwardingIngressOwner(raw)
	if alias == nil {
		return raw.Close()
	}
	for current, depth := alias, 0; current != nil; current, depth = forwardingIngressOwner(current.raw), depth+1 {
		if current == caller || current.gate != gate || depth >= maxForwardingRoutes {
			return ErrEngineCleanupFailed
		}
	}
	alias.mu.Lock()
	started := alias.closeStarted
	alias.mu.Unlock()
	if !started {
		_ = alias.closeOriginal() // the original owner arbitrates any concurrent first call
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	select {
	case <-alias.closeWait:
	case <-ctx.Done():
	}
	select {
	case <-alias.closeWait:
		alias.mu.Lock()
		returned, err := alias.closeReturned, alias.closeErr
		alias.mu.Unlock()
		if returned && err == nil {
			return nil
		}
		return errors.Join(ErrEngineCleanupFailed, err)
	default:
		return ErrEngineCleanupFailed // original stays pending; never retry its native Close
	}
}

// Called only in post-Box.Close drain. A later SDK invocation cannot append
// forgotten work to a supposedly retired input cohort; it stops its worker.
func (gate *forwardingAdmission) sealRejections() []*forwardingRejection {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.rejectionsSealed = true
	owners := make([]*forwardingRejection, 0, len(gate.rejections))
	for owner := range gate.rejections {
		owners = append(owners, owner)
	}
	return owners
}

func (gate *forwardingAdmission) drainRejections(ctx context.Context, owners []*forwardingRejection) error {
	unconfirmed := false
	for _, owner := range owners {
		select {
		case <-owner.done:
		case <-ctx.Done():
		}
		select {
		case <-owner.done:
			unconfirmed = unconfirmed || owner.failed
		default:
			unconfirmed = true
		}
	}
	if unconfirmed {
		return ErrEngineCleanupFailed
	}
	return nil
}
