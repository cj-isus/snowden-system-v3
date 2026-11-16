package childbootstrap

import (
	"bytes"
	"context"
	"encoding/binary"
)

// Operations borrow callbacks, never their original native resources. The caller
// retains every partial owner on failure/panic/Goexit and decides terminal cleanup.
// Scope must include the caller's original operation/source checks. Transport must
// enforce its own byte limits, peer identity and physical-I/O cancellation fences.
type Operations struct {
	Scope, Prepare, Validate, Verify func(context.Context) error
	Adopt                            func([2]uint64) error
	AdoptAncestor                    func([3]uint64) error
	ValidateJob                      func() error
	AwaitParent                      func(context.Context) error
	Exchange                         func(context.Context, []byte, []byte) error
	CloseTransport                   func() error
}

func (operations Operations) scope(ctx context.Context) error {
	return CheckScope(ctx, operations.Scope)
}

func (operations Operations) step(ctx context.Context, action func() error) error {
	if err := operations.scope(ctx); err != nil {
		return err
	}
	if err := action(); err != nil {
		return err
	}
	return operations.scope(ctx)
}

// Run preserves the fixed seed/ARM/GO/OWN/USE/ACK order. Only after USE may the
// child ACK, close transport, await its original parent and verify normal startup.
// Callback success is a local fact, not GUI readiness or signature authority.
// The sequencer starts no goroutine/timer and owns no handles. Native operations
// stay in caller callbacks, including their cancellation and cleanup obligations.
func Run(ctx context.Context, protocol Protocol, operations Operations) error {
	if !validPurpose(protocol.purpose) || protocol.NeedsAncestor() && operations.AdoptAncestor == nil ||
		!protocol.NeedsAncestor() && operations.Adopt == nil {
		return ErrProtocol
	}
	return runHandoff(ctx, protocol.Stages(), protocol.binding, protocol.SeedSize(), operations, protocol.ValidExchange,
		func(raw []byte) error {
			if protocol.NeedsAncestor() {
				var handles [3]uint64
				for index := range handles {
					handles[index] = binary.LittleEndian.Uint64(raw[FrameSize+index*8:])
				}
				if !ValidAncestorHandles(handles) {
					return ErrProtocol
				}
				return operations.AdoptAncestor(handles)
			}
			var handles [2]uint64
			for index := range handles {
				handles[index] = binary.LittleEndian.Uint64(raw[FrameSize+index*8:])
			}
			return operations.Adopt(handles)
		})
}

// Only compiled child wires call this implementation. Seed length and
// exchange bounds are role-specific; legacy public Operations/ValidExchange and
// native adoption remain unchanged. The broker wire must not widen U1's grammar.
func runHandoff(ctx context.Context, stages Stages, binding [32]byte, seedSize int, operations Operations,
	validExchange func(int, int) bool, adopt func([]byte) error) error {
	if ctx == nil || (seedSize != SeedSize && seedSize != AncestorSeedSize && seedSize != BrokerSuccessorSeedSize) || operations.Scope == nil ||
		operations.Prepare == nil || operations.Validate == nil || operations.Verify == nil ||
		adopt == nil || validExchange == nil || operations.ValidateJob == nil || operations.AwaitParent == nil ||
		operations.Exchange == nil || operations.CloseTransport == nil {
		return ErrProtocol
	}
	exchange := func(send, receive []byte) error {
		if !validExchange(len(send), len(receive)) {
			return ErrProtocol
		}
		return operations.step(ctx, func() error { return operations.Exchange(ctx, send, receive) })
	}
	var seed [BrokerSuccessorSeedSize]byte
	if err := exchange(nil, seed[:seedSize]); err != nil {
		return err
	}
	wantSeed := Frame(stages.Seed, binding)
	if !bytes.Equal(seed[:FrameSize], wantSeed[:]) {
		return ErrProtocol
	}
	if err := operations.step(ctx, func() error { return adopt(seed[:seedSize]) }); err != nil {
		return err
	}
	if err := operations.step(ctx, func() error { return operations.Prepare(ctx) }); err != nil {
		return err
	}
	armed := Frame(stages.Armed, binding)
	var command [FrameSize]byte
	if err := exchange(armed[:], command[:]); err != nil {
		return err
	}
	goFrame := Frame(stages.Go, binding)
	if !bytes.Equal(command[:], goFrame[:]) {
		return ErrProtocol
	}
	if err := operations.step(ctx, func() error { return operations.Validate(ctx) }); err != nil {
		return err
	}
	owned := Frame(stages.Owned, binding)
	if err := exchange(owned[:], command[:]); err != nil {
		return err
	}
	use := Frame(stages.Use, binding)
	if !bytes.Equal(command[:], use[:]) {
		return ErrProtocol
	}
	if err := operations.step(ctx, func() error { return operations.Validate(ctx) }); err != nil {
		return err
	}
	if err := operations.step(ctx, operations.ValidateJob); err != nil {
		return err
	}
	ack := Frame(stages.Ack, binding)
	if err := exchange(ack[:], nil); err != nil {
		return err
	}
	if err := operations.step(ctx, operations.CloseTransport); err != nil {
		return err
	}
	if err := operations.step(ctx, func() error { return operations.AwaitParent(ctx) }); err != nil {
		return err
	}
	if err := operations.step(ctx, func() error { return operations.Verify(ctx) }); err != nil {
		return err
	}
	if err := operations.step(ctx, func() error { return operations.Validate(ctx) }); err != nil {
		return err
	}
	return operations.step(ctx, operations.ValidateJob)
}
