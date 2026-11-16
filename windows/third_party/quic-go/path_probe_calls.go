package quic

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	maxPathProbeCalls = 64
	maxPathProbeRows  = 64
	maxPathChallenges = 32
	minPathProbeDelay = time.Millisecond
	maxPathProbeDelay = 30 * time.Second
)

var (
	errPathProbeCapacity  = errors.New("quic: path probe capacity reached")
	errPathProbeUncertain = errors.New("quic: original path probe did not retire")
	errPathProbeContext   = errors.New("quic: nil path probe context")
	errPathIDExhausted    = errors.New("quic: path ID space exhausted")
)

// mx protects registration and attempt metadata. Each admitted caller alone owns
// its timer and context callbacks, outside mx. Unknown returns keep its cell.
type pathProbeCall struct {
	active   bool
	ctx      context.Context
	ctxDone  <-chan struct{}
	path     *Path
	outgoing *pathOutgoing
	attempt  *pathProbeAttempt
	timer    *time.Timer
}

type pathProbeCalls struct {
	slots                   [maxPathProbeCalls]pathProbeCall
	count                   int
	sealed, failed, settled bool
	done                    chan struct{}
}

// Channels are immutable for all admitted waiters. A later attempt is installed
// only when the previous attempt has no registered callers. mx owns the fields.
type pathProbeAttempt struct {
	done                          chan struct{}
	sent                          chan struct{}
	completed, queued             bool
	err                           error
	waiters                       int
	initialDelay, nextDelay       time.Duration
	nextDue                       time.Time
	challenges                    [maxPathChallenges][8]byte
	challengeCount, challengeNext int
}

func pathProbeDelay(initial time.Duration) time.Duration {
	return min(max(initial, minPathProbeDelay), maxPathProbeDelay)
}

func (pm *pathManagerOutgoing) probeAdmissionErrorLocked() error {
	if pm.probeCalls.failed {
		return errPathProbeUncertain
	}
	if pm.probeCalls.sealed || pm.isClosing() {
		return net.ErrClosed
	}
	return nil
}

func (pm *pathManagerOutgoing) reserveProbe(ctx context.Context, p *Path) (*pathProbeCall, error) {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if err := pm.probeAdmissionErrorLocked(); err != nil {
		return nil, err
	}
	select {
	case <-p.abandon:
		return nil, ErrPathClosed
	default:
	}
	s := &pm.probeCalls
	if s.count >= len(s.slots) {
		return nil, errPathProbeCapacity
	}
	path := pm.paths[p.id]
	if path == nil && len(pm.paths) >= maxPathProbeRows {
		return nil, errPathProbeCapacity
	}
	if path != nil && path.enableErr != nil {
		return nil, path.enableErr
	}
	for i := range s.slots {
		call := &s.slots[i]
		if call.active {
			continue
		}
		*call = pathProbeCall{active: true, ctx: ctx, path: p}
		s.count++ // before context callbacks, row, attempt, channels or timer
		if path == nil {
			call.outgoing = &pathOutgoing{tr: p.tr, enablePath: p.enablePath}
			path = call.outgoing
			pm.paths[p.id] = path
		} else {
			call.outgoing = path
		}
		if path.attempt == nil || path.attempt.waiters == 0 {
			delay := pathProbeDelay(p.initialRTT)
			call.attempt = &pathProbeAttempt{initialDelay: delay, nextDelay: delay}
			path.attempt = call.attempt
			call.attempt.done = make(chan struct{})
			call.attempt.sent = make(chan struct{}, 1)
		} else {
			call.attempt = path.attempt
		}
		call.attempt.waiters++
		return call, nil
	}
	return nil, errPathProbeUncertain
}

func (pm *pathManagerOutgoing) settleProbeCallsLocked() {
	s := &pm.probeCalls
	if pm.isClosing() {
		s.sealed = true
	}
	if (s.sealed || s.failed) && !s.settled && (s.failed || s.count == 0) {
		s.settled = true
		close(s.done) // failure is negative, not proof that context callbacks stopped
	}
}

func (pm *pathManagerOutgoing) finishProbe(call *pathProbeCall, returned *bool) {
	if call.timer != nil {
		call.timer.Stop() // caller-owned channel timer, no callback; outside mx
	}
	pm.mx.Lock()
	defer pm.mx.Unlock()
	s := &pm.probeCalls
	if !*returned || !call.active {
		s.failed = true // retain original ctx/path/attempt/timer and registration
	} else {
		call.attempt.waiters--
		if call.attempt.waiters == 0 {
			pm.removeQueuedProbeLocked(call.path.id)
			call.attempt.queued = false
		}
		s.count--
		*call = pathProbeCall{}
	}
	pm.settleProbeCallsLocked()
}

// This only schedules work for the original connection loop. All waiters share
// its cadence and at most one queue row, even when many caller timers fire.
func (pm *pathManagerOutgoing) prepareProbe(call *pathProbeCall) (delay time.Duration, complete, wake bool, err error) {
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if err = pm.probeAdmissionErrorLocked(); err != nil {
		return
	}
	select {
	case <-call.path.abandon:
		err = ErrPathClosed
		return
	default:
	}
	a := call.attempt
	if a.completed {
		complete, err = true, a.err
		if err == nil {
			call.path.validated.Store(true)
		}
		return
	}
	now := time.Now()
	if !a.queued && !now.Before(a.nextDue) {
		if pm.queuedProbes == len(pm.pathsToProbe) {
			err = errPathProbeCapacity
			return
		}
		pm.pathsToProbe[pm.queuedProbes] = call.path.id
		pm.queuedProbes++
		a.queued, wake = true, true
	}
	delay = a.initialDelay
	if !a.queued && now.Before(a.nextDue) {
		delay = a.nextDue.Sub(now)
	}
	return
}

func (pm *pathManagerOutgoing) probePath(ctx context.Context, p *Path) error {
	if ctx == nil {
		return errPathProbeContext
	}
	call, err := pm.reserveProbe(ctx, p)
	if err != nil {
		return err
	}
	returned := false
	defer pm.finishProbe(call, &returned)
	err = pm.runProbe(call)
	returned = true
	return err
}

func (pm *pathManagerOutgoing) runProbe(call *pathProbeCall) error {
	call.ctxDone = call.ctx.Done() // original ctx retained before this callback
	for {
		select {
		case <-call.ctxDone:
			if cause := context.Cause(call.ctx); cause != nil {
				return cause
			}
			return context.Canceled
		default:
		}
		delay, complete, wake, err := pm.prepareProbe(call)
		if complete || err != nil {
			return err
		}
		if wake {
			pm.scheduleSending() // actual callback outside metadata lock
		}
		if call.timer == nil {
			call.timer = time.NewTimer(delay)
		} else {
			if !call.timer.Stop() {
				select {
				case <-call.timer.C:
				default:
				}
			}
			call.timer.Reset(delay)
		}
		select {
		case <-call.ctxDone:
			continue
		case <-pm.closing:
			return net.ErrClosed
		case <-pm.done:
			return net.ErrClosed
		case <-call.path.abandon:
			return ErrPathClosed
		case <-call.attempt.done:
			continue // recheck closing/abandon before publishing validation
		case <-call.attempt.sent:
		case <-call.timer.C:
		}
	}
}

func (pm *pathManagerOutgoing) removeQueuedProbeLocked(id pathID) {
	for i := 0; i < pm.queuedProbes; i++ {
		if pm.pathsToProbe[i] != id {
			continue
		}
		copy(pm.pathsToProbe[i:], pm.pathsToProbe[i+1:pm.queuedProbes])
		pm.queuedProbes--
		pm.pathsToProbe[pm.queuedProbes] = 0
		break
	}
	if path := pm.paths[id]; path != nil && path.attempt != nil {
		path.attempt.queued = false
	}
}

func (pm *pathManagerOutgoing) completeProbeLocked(id pathID, a *pathProbeAttempt, err error) {
	if a.completed {
		return
	}
	pm.removeQueuedProbeLocked(id)
	a.err, a.completed = err, true
	a.challenges = [maxPathChallenges][8]byte{}
	a.challengeCount, a.challengeNext = 0, 0
	close(a.done)
}

// The caller has already observed actual original Conn loop/input retirement.
// A manager published later cannot admit calls against already closed signals.
func (pm *pathManagerOutgoing) waitProbeCalls() error {
	pm.mx.Lock()
	pm.probeCalls.sealed = true
	pm.settleProbeCallsLocked()
	done := pm.probeCalls.done
	pm.mx.Unlock()
	<-done
	pm.mx.Lock()
	defer pm.mx.Unlock()
	if pm.probeCalls.failed || pm.probeCalls.count != 0 {
		return errPathProbeUncertain
	}
	return nil
}
