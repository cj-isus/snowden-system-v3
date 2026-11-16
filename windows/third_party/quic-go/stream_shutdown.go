package quic

import (
	"errors"
	"net"
)

var errStreamShutdownUncertain = errors.New("quic: original stream shutdown did not complete")

// Original synchronous cleanup only, protected by the stream mutex. A positive
// receipt does not join other public calls, borrowed writes or ACK/history data.
type streamShutdownReceipt struct {
	started  bool
	returned bool
	failure  error
}

func (r *streamShutdownReceipt) begin() (already bool, result error) {
	if !r.started {
		r.started = true
		return false, nil
	}
	if !r.returned {
		return true, errStreamShutdownUncertain // original attempt cannot be retried
	}
	return true, r.failure
}

func (r *streamShutdownReceipt) complete(result error, budget *streamReceiveBudget) error {
	r.returned = false
	r.failure = result // retain the known error before its original notification
	if result != nil && budget != nil {
		budget.markUncertain()
	}
	r.returned = true
	return result
}

func streamShutdownCause(cause error) error {
	if cause == nil {
		return net.ErrClosed // nil cannot leave blocked readers without an error
	}
	return cause
}
