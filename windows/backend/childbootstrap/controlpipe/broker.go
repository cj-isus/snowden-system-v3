package controlpipe

import (
	"context"
	"encoding/hex"
	"sync/atomic"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

// BrokerPipe fixes one purpose and its nonce before acquisition. Publish this
// original pointer before Listen/Dial; never copy it or expose the private Pipe.
// The underlying owner serializes native calls and retains uncertain release.
// Transport checks do not authenticate the original App/helper/image/Job/SID.
type BrokerPipe struct {
	native     Pipe
	protocol   childbootstrap.BrokerProtocol
	wire       pipeWire
	nonce      string
	runStarted atomic.Bool // one original sequencer attempt; never reset after failure
}

func NewBrokerPipe(protocol childbootstrap.BrokerProtocol) (*BrokerPipe, error) {
	wire := brokerWire(protocol)
	if wire == 0 {
		return nil, ErrPipe
	}
	binding := protocol.Binding()
	return &BrokerPipe{protocol: protocol, wire: wire, nonce: hex.EncodeToString(binding[:])}, nil
}

func (pipe *BrokerPipe) valid() bool {
	return pipe != nil && pipe.wire != 0 && pipe.wire == brokerWire(pipe.protocol)
}

// Name is an immutable endpoint name, never a peer-authentication receipt.
func (pipe *BrokerPipe) Name() (string, error) {
	if !pipe.valid() {
		return "", ErrPipe
	}
	return wireName(pipe.wire, pipe.nonce)
}

func (pipe *BrokerPipe) Listen(ctx context.Context, check func(context.Context) error) error {
	if !pipe.valid() {
		return ErrPipe
	}
	return pipe.native.listen(ctx, pipe.nonce, pipe.wire, check)
}

func (pipe *BrokerPipe) Dial(ctx context.Context, check func(context.Context) error) error {
	if !pipe.valid() {
		return ErrPipe
	}
	return pipe.native.dial(ctx, pipe.nonce, pipe.wire, check)
}

// expected is derived from an original held child process, never reopened by PID.
func (pipe *BrokerPipe) Accept(ctx context.Context, expected uint32, check func(context.Context) error) error {
	if !pipe.valid() {
		return ErrPipe
	}
	return pipe.native.Accept(ctx, expected, check)
}

// expected is derived from an original held parent handle in the adopted seed.
func (pipe *BrokerPipe) CheckServer(ctx context.Context, expected uint32, check func(context.Context) error) error {
	if !pipe.valid() {
		return ErrPipe
	}
	return pipe.native.CheckServer(ctx, expected, check)
}

func (pipe *BrokerPipe) Exchange(ctx context.Context, send, receive []byte, check func(context.Context) error) error {
	if !pipe.valid() || pipe.runStarted.Load() {
		return ErrPipe
	}
	return pipe.native.exchange(ctx, send, receive, pipe.wire, false, check)
}

// The caller retains this original one-attempt effect receipt on every outcome.
// It cannot replace the sequencer's transport or authorize a different wire.
func (pipe *BrokerPipe) ExchangeObserved(ctx context.Context, send, receive []byte, receipt *ExchangeReceipt, check func(context.Context) error) error {
	if !pipe.valid() || pipe.runStarted.Load() || receipt == nil {
		return ErrPipe
	}
	return pipe.native.exchangeObserved(ctx, send, receive, pipe.wire, false, receipt, check)
}

func (pipe *BrokerPipe) CloseAfterServer(ctx context.Context, check func(context.Context) error) error {
	if !pipe.valid() {
		return ErrPipe
	}
	return pipe.native.CloseAfterServer(ctx, check)
}

func (pipe *BrokerPipe) Retire() {
	if pipe != nil {
		pipe.native.Retire()
	}
}

func (pipe *BrokerPipe) Close() error {
	if pipe == nil {
		return nil
	}
	return pipe.native.Close()
}

// These adapters bind only the original transport. The caller must already have
// dialed it and retains every original native chain/spawn owner on all outcomes.
// No caller-provided exchange/close callback may replace this binding. Errors
// after child commit/ACK do not erase the coordinator's irreversible step facts.
func (pipe *BrokerPipe) RunCoordinator(ctx context.Context, owner *childbootstrap.BrokerCoordinator, operations childbootstrap.BrokerOperations) error {
	if !pipe.valid() || pipe.protocol.Purpose() != childbootstrap.BrokerControl || owner == nil ||
		operations.Exchange != nil || operations.CloseTransport != nil || !pipe.runStarted.CompareAndSwap(false, true) {
		return ErrPipe
	}
	scope := operations.Scope
	if err := pipe.native.readyClient(ctx, pipe.wire, scope); err != nil {
		return err
	}
	operations.Exchange = func(ctx context.Context, send, receive []byte) error {
		return pipe.native.exchange(ctx, send, receive, pipe.wire, true, scope)
	}
	operations.CloseTransport = func() error { return pipe.native.CloseAfterServer(ctx, scope) }
	return owner.Run(ctx, pipe.protocol, operations)
}

func (pipe *BrokerPipe) RunGUI(ctx context.Context, operations childbootstrap.BrokerGUIOperations) error {
	if !pipe.valid() || pipe.protocol.Purpose() != childbootstrap.BrokerSuccessor ||
		operations.Exchange != nil || operations.CloseTransport != nil || !pipe.runStarted.CompareAndSwap(false, true) {
		return ErrPipe
	}
	scope := operations.Scope
	if err := pipe.native.readyClient(ctx, pipe.wire, scope); err != nil {
		return err
	}
	operations.Exchange = func(ctx context.Context, send, receive []byte) error {
		return pipe.native.exchange(ctx, send, receive, pipe.wire, true, scope)
	}
	operations.CloseTransport = func() error { return pipe.native.CloseAfterServer(ctx, scope) }
	return childbootstrap.RunBrokerGUI(ctx, pipe.protocol, operations)
}
