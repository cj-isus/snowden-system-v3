package main

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

// One synchronous TCP attempt retained before admission/Dial. No extra worker,
// cancellation-as-completion, repeated native Close or endpoint-based owner.
type backgroundProbeAttempt struct {
	scope           *backgroundProbeScope
	target          bgTarget
	done            chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	release         func()
	watchDone       chan struct{}
	watchReturned   bool // immutable after original watchDone
	admitted        bool
	dialStarted     bool
	dialReturned    bool
	dialTimely      bool      // captured before our own finalizer cancellation
	observedAt      time.Time // original Dial callback return, not later cleanup/publication time
	conn            net.Conn
	dialErr         error
	closeStarted    bool
	closeReturned   bool
	closeErr        error
	releaseStarted  bool
	releaseReturned bool
}

func (scope *backgroundProbeScope) probe(target bgTarget) {
	if !scope.current() {
		return
	}
	owner := scope.owner
	owner.mu.Lock()
	if owner.attempt != nil || owner.fault != nil || owner.stopping || owner.ctx.Err() != nil {
		owner.mu.Unlock()
		return
	}
	attempt := &backgroundProbeAttempt{scope: scope, target: target, done: make(chan struct{})}
	owner.attempt = attempt // before context admission or the original Dial callback
	owner.mu.Unlock()
	attempt.run()
}

func (attempt *backgroundProbeAttempt) run() {
	returned := false
	defer attempt.finish(&returned)
	attempt.body()
	returned = true
}

func (attempt *backgroundProbeAttempt) body() {
	if attempt.target.QUIC || !attempt.scope.current() {
		return // no UDP garbage/probe; QUIC cooldown is separate scheduling data
	}
	base, cancel := context.WithCancel(attempt.scope.ctx)
	attempt.cancel = cancel
	ctx, release, err := core.AdmitDiagnostic(base, core.DiagnosticBackground, 1, 0, bgDialTimeout)
	if err != nil {
		return // refused local admission is not a channel fact or an attempted dial
	}
	attempt.ctx, attempt.release, attempt.admitted = ctx, release, true
	attempt.watchDone = make(chan struct{})
	go attempt.watch() // one original cancellation observer per serial admitted attempt
	if !attempt.scope.current() || ctx.Err() != nil {
		return
	}
	attempt.dialStarted = true
	attempt.conn, attempt.dialErr = (&net.Dialer{Timeout: bgDialTimeout}).DialContext(ctx, "tcp", net.JoinHostPort(attempt.target.Host, attempt.target.Port))
	attempt.dialReturned = true // raw socket retained before any later validation/cleanup
	attempt.observedAt = time.Now()
	attempt.dialTimely = ctx.Err() == nil
}

func (attempt *backgroundProbeAttempt) finish(returned *bool) {
	payload := recover()
	if !*returned || payload != nil {
		attempt.scope.owner.fail()
	}
	sequence := &appCleanupSequence{}
	defer attempt.complete(sequence, *returned && payload == nil)
	defer appCleanupStep("join original background cancellation observer", sequence, func() error {
		if attempt.watchDone != nil {
			<-attempt.watchDone
			if !attempt.watchReturned {
				return errBackgroundProbeUnavailable
			}
		}
		return nil
	})
	defer appCleanupStep("close original background TCP socket", sequence, attempt.closeOriginal)
	defer appCleanupStep("cancel original background TCP context", sequence, func() error {
		if attempt.cancel != nil {
			attempt.cancel()
		}
		return nil
	})
}

func (attempt *backgroundProbeAttempt) watch() {
	defer close(attempt.watchDone)
	defer func() {
		if payload := recover(); payload != nil || !attempt.watchReturned {
			attempt.scope.owner.fail()
		}
	}()
	// A fresh optional round may inspect inactive channels after a blocked
	// intent. Its already-closed source grants nothing and cannot notify twice;
	// original identity checks still reject a later replacement intent.
	var intentDone <-chan struct{}
	if attempt.scope.sourceCurrent {
		intentDone = attempt.scope.registration.source.Done()
	}
	select {
	case <-attempt.ctx.Done():
	case <-intentDone:
	case <-attempt.scope.control.Done():
	case <-attempt.scope.owner.stop:
	}
	attempt.cancel() // never release admission, close native socket or borrow a successor
	attempt.watchReturned = true
}

func (attempt *backgroundProbeAttempt) closeOriginal() (result error) {
	if attempt.conn == nil {
		return nil // complete separately requires original Dial to have returned
	}
	if attempt.closeStarted {
		return attempt.closeErr // no replay even when the original result is unknown
	}
	attempt.closeStarted, attempt.closeErr = true, errBackgroundProbeUnavailable
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errBackgroundProbeUnavailable
		}
		attempt.closeReturned, attempt.closeErr = returned, result
	}()
	err := attempt.conn.Close()
	returned = true
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return errBackgroundProbeUnavailable
	}
	return nil
}

func (attempt *backgroundProbeAttempt) complete(sequence *appCleanupSequence, bodyReturned bool) {
	owner := attempt.scope.owner
	finished := false
	defer close(attempt.done) // frame outcome only, never a native retirement grant
	defer func() {
		if payload := recover(); payload != nil || !finished {
			owner.fail()
		}
	}()
	confirmed := bodyReturned && sequence.result == nil && (!attempt.dialStarted || attempt.dialReturned) &&
		(attempt.conn == nil || attempt.closeStarted && attempt.closeReturned && attempt.closeErr == nil) &&
		(!attempt.dialStarted || attempt.dialErr != nil || attempt.conn != nil)
	if !confirmed {
		owner.fail()
		finished = true
		return // retain original attempt, socket and admission; no replacement/retry
	}
	attempt.publish()
	if attempt.release != nil {
		if attempt.releaseStarted {
			owner.fail()
			finished = true
			return
		}
		attempt.releaseStarted = true
		attempt.release()
	}
	attempt.releaseReturned = true
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.attempt != attempt {
		owner.fault = errBackgroundProbeUnavailable
		finished = true
		return
	}
	owner.attempt = nil // actual body/Close/publication/log/release frames returned
	finished = true
}

func (attempt *backgroundProbeAttempt) publish() {
	scope, target := attempt.scope, attempt.target
	if !scope.current() {
		return // retired resources can release without publishing a stale observation
	}
	if target.QUIC {
		bgMarkDone(scope.epoch, target.ChannelID, time.Now())
		return
	}
	if !attempt.admitted || !attempt.dialStarted {
		return
	}
	bgMarkDone(scope.epoch, target.ChannelID, time.Now())
	if attempt.dialErr == nil {
		deadline, bounded := attempt.ctx.Deadline()
		if attempt.dialTimely && (!bounded || time.Now().Before(deadline)) {
			bgReg.resetConsec(scope.epoch)
			bgReg.markOK(scope.epoch, target.ChannelID, attempt.observedAt)
		}
		return // TCP connect is never a protected-path health PASS
	}
	cls := classifyBgError(attempt.dialErr)
	if bgReg.confirmFail(scope.epoch, target.ChannelID, attempt.observedAt) {
		chanHealth.observeBg(scope.epoch, target.ChannelID, cls, attempt.observedAt)
		scope.app.appendLog("warn", "фон: "+target.ChannelID+": "+cls+" — наблюдение для порядка кандидатов; защищённый путь этим не проверялся")
	}
}
