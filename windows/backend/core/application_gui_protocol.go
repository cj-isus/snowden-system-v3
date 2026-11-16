package core

import (
	"context"
	"encoding/hex"
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

const restartGUIArgument = childbootstrap.ApplicationRestartArgument

// Fixed purposes share custody, never signing authority. Update binds an
// authenticated manifest SHA; restart uses a private random correlation nonce.
type applicationGUIProtocol struct {
	binding                               [32]byte
	seed, armed, goStage, owned, use, ack string
	wire                                  childbootstrap.Protocol
}

func fixedApplicationGUIProtocol(argument, binding string) (applicationGUIProtocol, error) {
	digest, err := nativeImagePin(binding)
	if err != nil {
		return applicationGUIProtocol{}, err
	}
	wire, err := childbootstrap.NewApplicationProtocol(argument, digest)
	if err != nil {
		return applicationGUIProtocol{}, ErrUpdateGUIBootstrap
	}
	stages := wire.Stages()
	return applicationGUIProtocol{binding: digest, wire: wire,
		seed: stages.Seed, armed: stages.Armed, goStage: stages.Go,
		owned: stages.Owned, use: stages.Use, ack: stages.Ack}, nil
}

// owner.mu held. The private restart constructor selects R1/R2; the SDK owner
// selects GUI6/GUI7 with the same authenticated manifest binding. Only an actual
// captured ancestor changes the version; a failed ancestor never selects legacy.
func (owner *UpdateCodeAcceptance) applicationGUIProtocol() (applicationGUIProtocol, error) {
	if owner.restart != nil {
		if !owner.restart.ready || owner.argument != "" || owner.manifestSHA != "" {
			return applicationGUIProtocol{}, ErrUpdateGUIBootstrap
		}
		argument := restartGUIArgument
		if owner.ancestor != nil {
			argument = childbootstrap.ApplicationRestartAncestorArgument
		}
		return fixedApplicationGUIProtocol(argument, hex.EncodeToString(owner.restart.nonce[:]))
	}
	if owner.argument != updateCodeAcceptanceArgument {
		return applicationGUIProtocol{}, ErrUpdateGUIBootstrap
	}
	argument := updateGUIArgument
	if owner.ancestor != nil {
		argument = childbootstrap.ApplicationUpdateAncestorArgument
	}
	return fixedApplicationGUIProtocol(argument, owner.manifestSHA)
}

// Original pipe/Job/parent-handle bootstrap. Callbacks retain their own image
// before acquisition and perform purpose-specific verification; no reply is
// installation authority. Distinct stages prevent crossover between purposes.
func bootstrapApplicationGUI(ctx context.Context, protocol applicationGUIProtocol, native *updateGUINativeLease,
	prepare, validate, verify func(context.Context) error) error {
	return bootstrapApplicationGUITransport(ctx, protocol, native, prepare, validate, verify,
		func(ctx context.Context, send, receive []byte) error {
			return native.exchangeStdio(ctx, send, receive)
		}, closeUpdateGUIStreams)
}

// Child bootstrap is synchronous before services. Retain each original I/O
// owner in the already-published native session; no slot reset or extra stage.
func (native *updateGUINativeLease) exchangeStdio(ctx context.Context, send, receive []byte) error {
	if native == nil || native.stdioFailed || int(native.stdioNext) >= len(native.stdio) {
		return ErrUpdateGUIBootstrap
	}
	if native.stdioNext != 0 {
		previous := native.stdio[native.stdioNext-1]
		if previous == nil || previous.ownedPipeIO.retirementReady() != nil {
			native.stdioFailed = true
			return errors.Join(ErrUpdateGUIBootstrap, ErrHelperCleanup)
		}
	}
	index := native.stdioNext
	native.stdioNext++ // burn before publishing/launching any goroutine
	completed := false
	defer func() {
		if !completed {
			native.stdioFailed = true
		}
	}() // terminal intent only, no native return fact
	adopted := native.mode != guiLeaseUnused
	if !adopted && len(send) != 0 {
		return ErrUpdateGUIBootstrap // the initial pre-adoption step may only receive its seed
	}
	if adopted {
		if err := native.validateJob(ctx); err != nil {
			return errors.Join(ErrUpdateGUIBootstrap, err)
		}
	}
	err := exchangeUpdateGUIObserved(ctx, os.Stdin, os.Stdout, send, receive, nil, &native.stdio[index])
	if err == nil && adopted && len(receive) != 0 {
		err = native.validateJob(ctx)
	}
	// Send-only ACK lets the parent exit. The sequencer now closes transport,
	// awaits that original parent and validates again; no parent-alive query here.
	completed = err == nil
	return err
}

func bootstrapApplicationGUITransport(ctx context.Context, protocol applicationGUIProtocol, native *updateGUINativeLease,
	prepare, validate, verify func(context.Context) error, exchange func(context.Context, []byte, []byte) error, closeTransport func() error) error {
	if native == nil || closeTransport == nil {
		return ErrUpdateGUIBootstrap
	}
	err := childbootstrap.Run(ctx, protocol.wire, childbootstrap.Operations{
		Scope: contextError, Adopt: native.adopt,
		AdoptAncestor: func(handles [3]uint64) error { return native.adoptAncestor(ctx, handles) },
		ValidateJob:   func() error { return native.validateJob(ctx) },
		Prepare:       prepare, Validate: validate, Verify: verify, AwaitParent: native.awaitParent,
		Exchange: exchange, CloseTransport: func() error {
			if closeTransport() != nil {
				return ErrUpdateGUIBootstrap
			}
			return nil
		},
	})
	if err == childbootstrap.ErrProtocol {
		return ErrUpdateGUIBootstrap // preserve the existing core error boundary
	}
	return err
}
