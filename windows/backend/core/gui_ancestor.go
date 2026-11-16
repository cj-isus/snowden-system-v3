package core

import (
	"context"
	"errors"
)

// Native acceptance is separate from the existing broker/GUI bridge gates.
const ancestorRelaunchAccepted = false

// A source-custody fault survives successful child retirement. It is local to
// this application chain and must not be scored/retried as a channel failure.
var ErrGUIAncestorUnavailable = errors.New("original GUI job custody unavailable")

// Published once by a child's original bootstrap, before adoption. It remains
// present on failure: an uncertain ancestor must never select the legacy wire.
// Main initializes it before services/goroutines can request another handoff.
var processGUIAncestor guiAncestor

type guiAncestor interface {
	validate(context.Context) error
	checkChild(context.Context, processHost) error
	checkDescendant(context.Context, processHost) error
	duplicate(context.Context, processHost) (uint64, error)
}

// Borrow the original bootstrap reference; never replace a faulted source with
// the no-ancestor path. Native queries run under the process/lifecycle owner,
// outside App/Manager value locks. This query grants no root control or cleanup.
func (process *engineProcess) ancestorScope(ctx context.Context, child bool) error {
	if process == nil {
		return errors.Join(ErrEngineWorkerUnavailable, ErrGUIAncestorUnavailable)
	}
	return guiAncestorQuery(ctx, func() error {
		if process.ancestor == nil {
			return nil
		}
		switch {
		case !child:
			return process.ancestor.validate(ctx)
		case process.successorGUI:
			return process.ancestor.checkChild(ctx, process.host)
		default:
			return process.ancestor.checkDescendant(ctx, process.host)
		}
	})
}

// Shared private boundary for an original process borrow and the retained
// bootstrap source itself. Callers never fabricate a process for a root query.
func guiAncestorQuery(ctx context.Context, query func() error) (finalErr error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrEngineWorkerUnavailable, ErrGUIAncestorUnavailable)
		}
	}()
	if err := contextError(ctx); err != nil {
		completed = true
		return err
	}
	finalErr = query()
	if finalErr != nil {
		if !ancestorCancellationOnly(finalErr) {
			finalErr = errors.Join(ErrGUIAncestorUnavailable, finalErr)
		}
		finalErr = errors.Join(ErrEngineWorkerUnavailable, finalErr)
	}
	finalErr = errors.Join(finalErr, contextError(ctx))
	completed = true
	return finalErr
}

// Inspect this private native-source error only. errors.Is(cancellation) alone
// would hide an independent joined native fault. Fixed space/work also makes
// an opaque, empty, oversized or cyclic error tree fail as a local source fault.
func ancestorCancellationOnly(err error) bool {
	var pending [32]error
	pending[0] = err
	count, visited := 1, 0
	sawCancellation := false
	for count != 0 {
		if visited == 64 {
			return false
		}
		visited++
		count--
		current := pending[count]
		pending[count] = nil
		if current == nil {
			continue
		}
		if current == context.Canceled || current == context.DeadlineExceeded {
			sawCancellation = true
			continue
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			children := wrapped.Unwrap()
			if len(children) == 0 || len(children) > len(pending)-count {
				return false
			}
			for _, child := range children {
				pending[count] = child
				count++
			}
		case interface{ Unwrap() error }:
			child := wrapped.Unwrap()
			if child == nil {
				return false
			}
			pending[count] = child // replaces the popped slot; count cannot overflow
			count++
		default:
			return false
		}
	}
	return sawCancellation
}
