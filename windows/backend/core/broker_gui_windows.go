//go:build windows

package core

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

// The original App session owns all partial slots before acquisition. No resource
// owner is replaced after failure; main exits terminally with this session kept.
type brokerGUINative struct {
	lease spawnprep.BrokerLease
	pipe  *controlpipe.BrokerPipe
	image *windowsNativePair
}

func (native *brokerGUINative) validate(ctx context.Context) error {
	if native == nil || native.image == nil || native.pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	if err := native.image.Validate(ctx); err != nil {
		return err
	}
	if err := native.lease.Validate(ctx, contextError); err != nil {
		return err
	}
	if native.lease.OriginsRetired() {
		// Expected EOF/origin exit has already closed process/token slots. Never
		// reopen either process or demand a live pipe peer after that boundary.
		return native.image.pair.WithPaths(ctx, contextError, func(application, helper string) error {
			return nativeProcessPathMatches(ctx, windows.CurrentProcess(), application)
		})
	}
	// Lock order: lease -> held image pair -> original pipe. No callback reenters
	// an earlier owner. Validate above releases each mutex before this borrow.
	return native.lease.WithChain(ctx, contextError, func(ancestor, roleJob, parent, origin windows.Handle) error {
		return native.image.pair.WithPaths(ctx, contextError, func(application, helper string) error {
			if err := nativeProcessPathMatches(ctx, windows.CurrentProcess(), application); err != nil {
				return err
			}
			if err := nativeProcessPathMatches(ctx, parent, helper); err != nil {
				return err
			}
			if err := nativeProcessPathMatches(ctx, origin, application); err != nil {
				return err
			}
			id, err := windows.GetProcessId(parent)
			if err != nil || id == 0 {
				return errors.Join(ErrElevatedGUIUnavailable, err)
			}
			return native.pipe.CheckServer(ctx, id, contextError)
		})
	})
}

func (native *brokerGUINative) run(ctx context.Context, protocol childbootstrap.BrokerProtocol, helperPin string) error {
	if native == nil || native.pipe != nil || native.image != nil || protocol.Purpose() != childbootstrap.BrokerSuccessor {
		return ErrElevatedGUIUnavailable
	}
	native.image = new(windowsNativePair)
	if err := retainGUIAncestor(&native.lease); err != nil {
		return err
	}
	var err error
	native.pipe, err = controlpipe.NewBrokerPipe(protocol)
	if err != nil {
		return err
	}
	if err := native.pipe.Dial(ctx, contextError); err != nil {
		return err
	}
	err = native.pipe.RunGUI(ctx, childbootstrap.BrokerGUIOperations{
		Scope: contextError,
		Adopt: func(handles [4]uint64) error {
			return native.lease.Adopt(ctx, protocol.Purpose(), handles, contextError)
		},
		ValidateTree: func() error { return native.lease.Validate(ctx, contextError) },
		Prepare: func(ctx context.Context) error {
			if err := native.image.prepareCurrent(ctx, helperPin); err != nil {
				return err
			}
			return native.validate(ctx)
		},
		Validate: native.validate, Verify: native.validate,
		AwaitOrigins: func(ctx context.Context) error { return native.lease.AwaitOrigins(ctx, contextError) },
	})
	if err != nil {
		return err // exact partial owners remain in the terminal App session
	}
	if err := native.image.Close(); err != nil {
		return err
	}
	native.image = nil
	return contextError(ctx)
}
