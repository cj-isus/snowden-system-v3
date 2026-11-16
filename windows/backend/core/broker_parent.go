package core

import (
	"context"
	"encoding/binary"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
)

// Parent source is independently gated from the existing bootstrap/GUI bridge.
// No gate attests Shell's unavoidable execution-before-Job interval.
const brokerParentAccepted = false

func brokerParentAvailable() bool {
	return brokerParentAccepted && brokerGUIChainAccepted && ancestorRelaunchAccepted
}

type brokerGUIParent struct {
	helperPin string
	protocol  childbootstrap.BrokerProtocol
	pipe      *controlpipe.BrokerPipe
	native    brokerParentNative
}

// Pure allocation. The App publishes this exact owner before any native work.
// The helper digest comes only from the build's existing compiled pin.
func NewBrokerGUIRequest(helperPin string) *ElevatedGUIRequest {
	return &ElevatedGUIRequest{broker: &brokerGUIParent{helperPin: helperPin}}
}

func (owner *ElevatedGUIRequest) prepareImage(ctx context.Context) error {
	if owner.broker == nil {
		image := newVerifierApplication()
		owner.image = image
		if image == nil {
			return ErrNativeImageUnavailable
		}
		return image.prepare(ctx)
	}
	image := newNativeImage()
	owner.image = image
	if image == nil {
		return ErrNativeImageUnavailable
	}
	return image.prepareCurrent(ctx, owner.broker.helperPin)
}

func (owner *ElevatedGUIRequest) listen(ctx context.Context, binding string, check func(context.Context) error) error {
	if owner.broker == nil {
		return owner.pipe.listen(ctx, binding)
	}
	protocol, err := childbootstrap.NewPinnedBrokerProtocol(owner.nonce, owner.broker.helperPin)
	if err != nil {
		return err
	}
	owner.broker.protocol = protocol
	owner.broker.pipe, err = controlpipe.NewBrokerPipe(protocol)
	if err != nil {
		return err
	}
	return owner.broker.pipe.Listen(ctx, check)
}

func (owner *ElevatedGUIRequest) executeNative(ctx context.Context, resume bool, binding string, check func(context.Context) error) error {
	if owner.broker == nil {
		return owner.native.execute(ctx, owner.image, resume, binding, check)
	}
	return owner.broker.native.execute(ctx, owner.image, resume, owner.broker.protocol, check)
}

func (owner *ElevatedGUIRequest) validateNative(ctx context.Context) error {
	if owner.broker == nil {
		return owner.native.validate(ctx, owner.image)
	}
	return owner.broker.native.validate(ctx, owner.image)
}

func (owner *ElevatedGUIRequest) alive(ctx context.Context) error {
	if owner.broker == nil {
		return owner.native.alive(ctx)
	}
	return owner.broker.native.alive(ctx)
}

func (owner *ElevatedGUIRequest) acceptPipe(ctx context.Context, check func(context.Context) error) error {
	if owner.broker == nil {
		return owner.native.acceptPipe(ctx, &owner.pipe, check)
	}
	return owner.broker.native.acceptPipe(ctx, owner.broker.pipe, check)
}

func (owner *ElevatedGUIRequest) protocol(binding string) (applicationGUIProtocol, error) {
	if owner.broker == nil {
		return fixedApplicationGUIProtocol(elevatedGUIArgument, binding)
	}
	stages := owner.broker.protocol.Stages()
	if !owner.broker.protocol.PinnedControl() {
		return applicationGUIProtocol{}, ErrElevatedGUIUnavailable
	}
	return applicationGUIProtocol{binding: owner.broker.protocol.Binding(), seed: stages.Seed, armed: stages.Armed,
		goStage: stages.Go, owned: stages.Owned, use: stages.Use, ack: stages.Ack}, nil
}

func (owner *ElevatedGUIRequest) seed(protocol applicationGUIProtocol) ([]byte, error) {
	if owner.broker != nil {
		return owner.broker.protocol.EncodeSeed(owner.broker.native.remoteHandles())
	}
	seed := make([]byte, updateGUISeedSize)
	frame := updateGUIFrame(protocol.seed, protocol.binding)
	copy(seed, frame[:])
	for index, handle := range owner.native.remoteHandles() {
		binary.LittleEndian.PutUint64(seed[updateGUIFrameSize+index*8:], handle)
	}
	return seed, nil
}

func (owner *ElevatedGUIRequest) exchange(ctx context.Context, send, receive []byte, check func(context.Context) error) error {
	if owner.broker == nil {
		return owner.pipe.exchange(ctx, send, receive, check)
	}
	return owner.broker.pipe.Exchange(ctx, send, receive, check)
}

func (owner *ElevatedGUIRequest) exchangeFinal(ctx context.Context, send, receive []byte, check func(context.Context) error) error {
	if owner.broker == nil {
		return owner.pipe.exchangeObserved(ctx, send, receive, &owner.finalExchange, check)
	}
	return owner.broker.pipe.ExchangeObserved(ctx, send, receive, &owner.finalExchange, check)
}

func (owner *ElevatedGUIRequest) closePipe() error {
	if owner.broker == nil {
		return owner.pipe.close()
	}
	return owner.broker.pipe.Close()
}

func (owner *ElevatedGUIRequest) retireNative() error {
	if owner.broker == nil {
		return owner.native.retire()
	}
	return owner.broker.native.retire()
}

func (owner *ElevatedGUIRequest) afterACK(ctx context.Context, guard func(context.Context) error) error {
	if owner.broker == nil {
		if err := owner.scope(ctx, guard); err != nil {
			return err
		}
		return owner.native.alive(ctx)
	}
	// Helper exits only after GUI ACK and server EOF. Do not demand a live helper
	// after closing the server; require the actual original process to exit zero.
	return owner.broker.native.awaitExit(ctx, owner.image, func(ctx context.Context) error { return owner.scope(ctx, guard) })
}
