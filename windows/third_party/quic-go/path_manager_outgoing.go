package quic

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

var (
	// ErrPathClosed is returned when trying to switch to a path that has been closed.
	ErrPathClosed = errors.New("path closed")
	// ErrPathNotValidated is returned when trying to use a path before path probing has completed.
	ErrPathNotValidated = errors.New("path not yet validated")
)

var errPathDoesNotExist = errors.New("path does not exist")

// Path is a network path.
type Path struct {
	id          pathID
	pathManager *pathManagerOutgoing
	tr          *Transport
	initialRTT  time.Duration

	enablePath func() error
	validated  atomic.Bool
	abandon    chan struct{}
}

// Probe waits for a validated response. Concurrent callers share one attempt;
// cancelling one caller does not cancel the other admitted waiters.
func (p *Path) Probe(ctx context.Context) error {
	return p.pathManager.probePath(ctx, p)
}

// Switch switches the QUIC connection to this path.
// It immediately stops sending on the old path, and sends on this new path.
func (p *Path) Switch() error {
	if err := p.pathManager.switchToPath(p.id); err != nil {
		switch {
		case errors.Is(err, ErrPathNotValidated):
			return err
		case errors.Is(err, errPathDoesNotExist) && !p.validated.Load():
			select {
			case <-p.abandon:
				return ErrPathClosed
			default:
				return ErrPathNotValidated
			}
		default:
			return ErrPathClosed
		}
	}
	return nil
}

// Close abandons a path.
// It is not possible to close the path that’s currently active.
// After closing, it is not possible to probe this path again.
// The connection loop retires the path's connection ID asynchronously.
func (p *Path) Close() error {
	return p.pathManager.removePath(p)
}

type pathOutgoing struct {
	tr          *Transport
	isValidated bool
	attempt     *pathProbeAttempt
	enablePath  func() error
	enableErr   error // protected by path-manager mutex
	abandoned   bool  // caller never retires CID state
	cidAssigned bool  // independent of challenge retention / probe attempts
}

type pathManagerOutgoing struct {
	getConnID       func(pathID) (_ protocol.ConnectionID, ok bool)
	retireConnID    func(pathID)
	scheduleSending func()
	closing, done   <-chan struct{} // original Conn signals, assigned before publication

	mx                sync.Mutex
	activePath        pathID
	pathsToProbe      [maxPathProbeRows]pathID
	queuedProbes      int
	probeCalls        pathProbeCalls
	paths             map[pathID]*pathOutgoing
	nextPathID        pathID
	pathToSwitchTo    *pathOutgoing
	retirementPending bool // avoids scanning paths on every normal send iteration
}

// newPathManagerOutgoing creates a new pathManagerOutgoing object. This
// function must be side-effect free as it may be called multiple times for a
// single connection.
func newPathManagerOutgoing(
	getConnID func(pathID) (_ protocol.ConnectionID, ok bool),
	retireConnID func(pathID),
	scheduleSending func(),
) *pathManagerOutgoing {
	return &pathManagerOutgoing{
		activePath:      0, // at initialization time, we're guaranteed to be using the handshake path
		nextPathID:      1,
		getConnID:       getConnID,
		retireConnID:    retireConnID,
		scheduleSending: scheduleSending,
		paths:           make(map[pathID]*pathOutgoing, 4),
		probeCalls:      pathProbeCalls{done: make(chan struct{})},
	}
}

func (pm *pathManagerOutgoing) isClosing() bool {
	select {
	case <-pm.closing:
		return true
	case <-pm.done:
		return true
	default:
		return false
	}
}

// Called only by the original connection loop. A reply reuses a registered
// transport's path CID; an abandoned/unregistered route has no fallback CID.
func (pm *pathManagerOutgoing) responseConnID(tr *Transport) (protocol.ConnectionID, bool) {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if pm.isClosing() {
		return protocol.ConnectionID{}, false
	}
	for id, path := range pm.paths {
		if path.tr == tr && path.cidAssigned && !path.abandoned && path.enableErr == nil {
			return pm.getConnID(id)
		}
	}
	return protocol.ConnectionID{}, false
}

func (pm *pathManagerOutgoing) removePath(p *Path) error {
	if err := pm.removePathImpl(p); err != nil {
		return err
	}
	if !pm.isClosing() {
		pm.scheduleSending()
	}
	return nil
}

func (pm *pathManagerOutgoing) removePathImpl(p *Path) error {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	select {
	case <-p.abandon:
		return nil
	default:
	}
	if p.id == pm.activePath {
		return errors.New("cannot close active path")
	}
	if path, ok := pm.paths[p.id]; ok {
		path.abandoned = true
		pm.removeQueuedProbeLocked(p.id)
		if !path.cidAssigned || pm.isClosing() {
			// Terminal manager.Close owns all remaining tokens. This caller
			// only removes metadata and never invokes that manager after close.
			delete(pm.paths, p.id)
		} else {
			pm.retirementPending = true
		}
	}
	close(p.abandon) // same mutex as addPath and every concurrent Close
	return nil
}

// Called only by the original connection loop while holding mx.
// The existing path row is the pending owner; no retirement queue is appended.
func (pm *pathManagerOutgoing) retireAbandonedPathsLocked() {
	if !pm.retirementPending || pm.isClosing() {
		return // original terminal connIDManager.Close owns token removal
	}
	for id, path := range pm.paths {
		if !path.abandoned {
			continue
		}
		if path.cidAssigned {
			pm.retireConnID(id)
			path.cidAssigned = false // only after original callback returns
		}
		delete(pm.paths, id)
	}
	pm.retirementPending = false // abnormal callbacks retain the pending owner
}

func (pm *pathManagerOutgoing) switchToPath(id pathID) error {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if err := pm.probeAdmissionErrorLocked(); err != nil {
		return err
	}

	p, ok := pm.paths[id]
	if !ok || p.abandoned {
		return errPathDoesNotExist
	}
	if p.enableErr != nil {
		return p.enableErr
	}
	if !p.isValidated {
		return ErrPathNotValidated
	}
	pm.pathToSwitchTo = p
	pm.activePath = id
	return nil
}

func (pm *pathManagerOutgoing) NewPath(t *Transport, initialRTT time.Duration, enablePath func() error) (*Path, error) {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if err := pm.probeAdmissionErrorLocked(); err != nil {
		return nil, err
	}
	if pm.nextPathID <= 0 {
		return nil, errPathIDExhausted
	}

	id := pm.nextPathID
	pm.nextPathID++
	return &Path{
		pathManager: pm,
		id:          id,
		tr:          t,
		enablePath:  enablePath,
		initialRTT:  initialRTT,
		abandon:     make(chan struct{}),
	}, nil
}

func (pm *pathManagerOutgoing) NextPathToProbe() (_ protocol.ConnectionID, _ ackhandler.Frame, _ *Transport, hasPath bool) {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if pm.probeAdmissionErrorLocked() != nil {
		return protocol.ConnectionID{}, ackhandler.Frame{}, nil, false
	}

	var p *pathOutgoing
	var connID protocol.ConnectionID
	id := invalidPathID
	// A path waiting for a new CID must not block re-probes that already own one.
	// Visit each originally queued row at most once; the queue is fixed-size.
	for remaining := pm.queuedProbes; remaining > 0; remaining-- {
		pID := pm.pathsToProbe[0]
		var ok bool
		p, ok = pm.paths[pID]
		if !ok || p.abandoned || p.enableErr != nil || p.attempt == nil || p.attempt.completed || p.attempt.waiters == 0 {
			pm.removeQueuedProbeLocked(pID)
			continue
		}
		connID, ok = pm.getConnID(pID)
		if !ok {
			copy(pm.pathsToProbe[:], pm.pathsToProbe[1:pm.queuedProbes])
			pm.pathsToProbe[pm.queuedProbes-1] = pID
			continue
		}
		id = pID
		break
	}
	if id == invalidPathID {
		return protocol.ConnectionID{}, ackhandler.Frame{}, nil, false
	}

	pm.removeQueuedProbeLocked(id)
	a := p.attempt
	p.cidAssigned = true // retain CID ownership before registration callbacks
	if err := p.enablePath(); err != nil {
		p.enableErr = err
		p.isValidated = false
		if pm.pathToSwitchTo == p {
			pm.pathToSwitchTo = nil
		}
		pm.retireConnID(id)
		p.cidAssigned = false
		pm.completeProbeLocked(id, a, err)
		return protocol.ConnectionID{}, ackhandler.Frame{}, nil, false
	}

	var b [8]byte
	_, _ = rand.Read(b[:])
	a.challenges[a.challengeNext] = b
	a.challengeNext = (a.challengeNext + 1) % len(a.challenges)
	if a.challengeCount < len(a.challenges) {
		a.challengeCount++
	}
	a.nextDue = time.Now().Add(a.nextDelay)
	if a.nextDelay > maxPathProbeDelay/2 {
		a.nextDelay = maxPathProbeDelay
	} else {
		a.nextDelay *= 2
	}

	select {
	case a.sent <- struct{}{}:
	default:
	}
	frame := ackhandler.Frame{
		Frame:   &wire.PathChallengeFrame{Data: b},
		Handler: (*pathManagerOutgoingAckHandler)(pm),
	}
	return connID, frame, p.tr, true
}

func (pm *pathManagerOutgoing) HandlePathResponseFrame(f *wire.PathResponseFrame) {
	pm.mx.Lock()
	defer pm.mx.Unlock()

	for id, p := range pm.paths {
		a := p.attempt
		if p.abandoned || a == nil || a.completed {
			continue
		}
		if slices.Contains(a.challenges[:a.challengeCount], f.Data) {
			p.isValidated = true
			pm.completeProbeLocked(id, a, nil)
			break
		}
	}
}

// Called by the original connection loop before send-queue pressure checks.
func (pm *pathManagerOutgoing) ShouldSwitchPath() (*Transport, bool) {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	pm.retireAbandonedPathsLocked()

	if pm.probeAdmissionErrorLocked() != nil || pm.pathToSwitchTo == nil || pm.pathToSwitchTo.abandoned {
		return nil, false
	}
	p := pm.pathToSwitchTo
	pm.pathToSwitchTo = nil
	return p.tr, true
}

type pathManagerOutgoingAckHandler pathManagerOutgoing

var _ ackhandler.FrameHandler = &pathManagerOutgoingAckHandler{}

// OnAcked is called when the PATH_CHALLENGE is acked.
// This doesn't validate the path, only receiving the PATH_RESPONSE does.
func (pm *pathManagerOutgoingAckHandler) OnAcked(wire.Frame) {}

func (pm *pathManagerOutgoingAckHandler) OnLost(wire.Frame) {}
