//go:build windows

package core

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const (
	LoopbackReachable         = "reachable"
	LoopbackRefused           = "refused"
	LoopbackTimeout           = "timeout"
	LoopbackCanceled          = "canceled"
	LoopbackBusy              = "busy"
	LoopbackUnavailable       = "unavailable"
	loopbackPortLimit         = 2
	loopbackObservationBudget = 300 * time.Millisecond
)

// Read-only IPv4/TCP connect observation. No proxy protocol/process identity,
// protection, recovery authority or complete native retirement is attested.
type LoopbackObservation struct {
	Schema  int    `json:"schema"`
	Port    int    `json:"port"`
	Outcome string `json:"outcome"`
}

type loopbackObserver struct {
	network                         preflightNetwork // reuse reserve-before-dial and original sticky Close
	ctx                             context.Context
	cancel                          context.CancelFunc
	deadline                        time.Time
	pending                         bool
	cancelAttempted, cancelReturned bool
	closeAttempted, closeReturned   bool
}

var loopbackObservations struct {
	sync.Mutex
	owner *loopbackObserver
}

// ObserveLoopback admits one synchronous original operation in this process.
// Input count/values are bounded before copying; duplicate ports share one fact.
// The requested deadline never refunds an owner whose call/Close did not return.
func ObserveLoopback(parent context.Context, ports ...int) (facts []LoopbackObservation) {
	if len(ports) == 0 || len(ports) > loopbackPortLimit {
		return nil
	}
	var unique [loopbackPortLimit]int
	count := 0
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return nil
		}
		if count != 0 && unique[0] == port {
			continue
		}
		unique[count], count = port, count+1
	}
	selected := unique[:count]
	facts = loopbackFacts(selected, LoopbackUnavailable)
	if parent == nil {
		return facts
	}
	if err := contextError(parent); err != nil {
		return loopbackFacts(selected, loopbackFailure(err))
	}
	if !loopbackObservations.TryLock() {
		return loopbackFacts(selected, LoopbackBusy)
	}
	defer loopbackObservations.Unlock()
	if loopbackObservations.owner != nil {
		return facts
	}
	owner := &loopbackObserver{}
	loopbackObservations.owner = owner // before timer/dial can acquire anything
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			facts = loopbackFacts(selected, LoopbackUnavailable)
		}
		if err := owner.retire(); err != nil {
			facts = loopbackFacts(selected, LoopbackUnavailable)
			return // original unknown graph remains retained, never replaced
		}
		loopbackObservations.owner = nil
		if !returned {
			return
		}
		if err := contextError(parent); err != nil {
			facts = loopbackFacts(selected, loopbackFailure(err))
		} else if !time.Now().Before(owner.deadline) {
			facts = loopbackFacts(selected, LoopbackTimeout)
		}
	}()
	owner.pending = true
	owner.ctx, owner.cancel = context.WithTimeout(parent, loopbackObservationBudget)
	owner.deadline, _ = owner.ctx.Deadline()
	owner.pending = false
	dialer := &net.Dialer{Timeout: loopbackObservationBudget, KeepAlive: -1}
	for index, port := range selected {
		if err := contextError(owner.ctx); err != nil {
			facts[index].Outcome = loopbackFailure(err)
			continue
		}
		owner.pending = true
		conn, err := owner.network.dialWith(owner.ctx, "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), dialer.DialContext)
		owner.pending = false // includes the original dialWith/deadline frame's return
		if err == nil && conn != nil {
			facts[index].Outcome = LoopbackReachable
		} else {
			facts[index].Outcome = loopbackFailure(err)
		}
	}
	returned = true
	return facts // no fact escapes before the original cleanup below
}

func loopbackFacts(ports []int, outcome string) []LoopbackObservation {
	facts := make([]LoopbackObservation, len(ports))
	for index, port := range ports {
		facts[index] = LoopbackObservation{Schema: 1, Port: port, Outcome: outcome}
	}
	return facts
}

func loopbackFailure(err error) string {
	switch {
	case errors.Is(err, ErrPreflightCleanup):
		return LoopbackUnavailable
	case errors.Is(err, context.Canceled):
		return LoopbackCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return LoopbackTimeout
	case errors.Is(err, windows.WSAECONNREFUSED):
		return LoopbackRefused
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return LoopbackTimeout
	}
	return LoopbackUnavailable
}

func (owner *loopbackObserver) retire() (finalErr error) {
	finalErr = ErrPreflightCleanup
	receipt := &lifecycleCleanup{}
	defer func() {
		if receipt.result != nil || owner.pending || !owner.closeReturned || owner.cancel != nil && !owner.cancelReturned {
			return
		}
		complete, err := owner.network.retirementState()
		if complete && err == nil {
			finalErr = nil
		}
	}()
	// Independent original stages: abnormal cancellation must not skip closing
	// known returned sockets. An unfinished dial/deadline frame is never closed
	// speculatively or treated as a completed failed acquisition.
	defer lifecycleCleanupStep("loopback original connections", receipt, func() error {
		if owner.pending || owner.closeAttempted {
			return ErrPreflightCleanup
		}
		owner.closeAttempted = true
		err := owner.network.close()
		owner.closeReturned = true
		return err
	})
	defer lifecycleCleanupStep("loopback original cancellation", receipt, func() error {
		if owner.cancel == nil {
			if owner.pending {
				return ErrPreflightCleanup
			}
			return nil
		}
		if owner.cancelAttempted {
			return ErrPreflightCleanup
		}
		owner.cancelAttempted = true
		owner.cancel()
		owner.cancelReturned = true
		return nil
	})
	return finalErr
}
