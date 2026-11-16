package childbootstrap

import (
	"bytes"
	"context"
	"errors"
)

// These callbacks belong to the retained platform owner. Adopt authenticates
// the original App, separate helper/ancestor Jobs and peer before ARM;
// ValidateChain repeats image/SID/peer/source/tree checks. The helper's resource
// Job cannot become the GUI's lifetime resource ceiling. PrepareSuccessor creates
// only the fixed GUI, includes its Job at CreateProcess and retains partial owners.
// CommitSuccessor sends GUI USE and observes its exact ACK, without treating a
// deadline, callback success or root exit as a whole-tree retirement receipt.
type BrokerOperations struct {
	Scope, ValidateChain              func(context.Context) error
	Adopt                             func([4]uint64) error
	PrepareSuccessor, CommitSuccessor func(context.Context) error
	Exchange                          func(context.Context, []byte, []byte) error
	CloseTransport                    func() error
}

// Step facts are not transferable capabilities. Returned includes actual errors;
// Succeeded records only that local callback result, before a later scope check.
type BrokerStep struct{ Attempted, Returned, Succeeded bool }

type BrokerProgress struct {
	Started, Failed                                       bool
	Adoption, Preparation, Commit, Acknowledgement, Close BrokerStep
}

// Publish this original coordinator before Run; never copy it after use. Caller
// serializes access and retains native owners independently. No hidden cleanup,
// goroutine, timeout, replacement or positive recovery follows an interrupted step.
type BrokerCoordinator struct{ progress BrokerProgress }

func (owner *BrokerCoordinator) Progress() BrokerProgress {
	if owner == nil {
		return BrokerProgress{}
	}
	return owner.progress
}

func brokerStep(ctx context.Context, scope func(context.Context) error, step *BrokerStep, action func() error) error {
	if err := CheckScope(ctx, scope); err != nil {
		return err
	}
	step.Attempted = true
	err := action()
	step.Returned, step.Succeeded = true, err == nil
	return errors.Join(err, CheckScope(ctx, scope))
}

// Run coordinates the helper side only. App GO precedes GUI preparation; App USE
// precedes GUI commit; broker ACK follows the GUI ACK and a fresh chain check.
// Every failure is terminal for this coordinator, including loss of the broker
// ACK after the GUI has committed. The original platform owner must preserve that
// uncertainty and retain/retire its Job tree; it cannot retry as a new handoff.
func (owner *BrokerCoordinator) Run(ctx context.Context, protocol BrokerProtocol, operations BrokerOperations) error {
	if owner == nil || owner.progress.Started {
		return ErrProtocol
	}
	owner.progress.Started = true
	completed := false
	defer func() {
		if !completed {
			owner.progress.Failed = true // also panic/Goexit; actual step facts remain
		}
	}()
	if protocol.purpose != BrokerControl || ctx == nil || operations.Scope == nil ||
		operations.Adopt == nil || operations.ValidateChain == nil || operations.PrepareSuccessor == nil ||
		operations.CommitSuccessor == nil || operations.Exchange == nil || operations.CloseTransport == nil {
		return ErrProtocol
	}
	step := func(action func() error) error {
		var receipt BrokerStep
		return brokerStep(ctx, operations.Scope, &receipt, action)
	}
	validate := func() error {
		return step(func() error { return operations.ValidateChain(ctx) })
	}
	exchange := func(send, receive []byte) error {
		if !protocol.ValidExchange(len(send), len(receive)) {
			return ErrProtocol
		}
		return step(func() error { return operations.Exchange(ctx, send, receive) })
	}
	var seed [BrokerControlSeedSize]byte
	if err := exchange(nil, seed[:]); err != nil {
		return err
	}
	handles, err := protocol.DecodeSeed(seed[:])
	if err != nil {
		return err
	}
	if err := brokerStep(ctx, operations.Scope, &owner.progress.Adoption, func() error { return operations.Adopt(handles) }); err != nil {
		return err
	}
	if err := validate(); err != nil {
		return err
	}
	stages := protocol.Stages()
	armed := Frame(stages.Armed, protocol.binding)
	var command [FrameSize]byte
	if err := exchange(armed[:], command[:]); err != nil {
		return err
	}
	goFrame := Frame(stages.Go, protocol.binding)
	if !bytes.Equal(command[:], goFrame[:]) {
		return ErrProtocol
	}
	if err := validate(); err != nil {
		return err
	}
	if err := brokerStep(ctx, operations.Scope, &owner.progress.Preparation, func() error { return operations.PrepareSuccessor(ctx) }); err != nil {
		return err
	}
	if err := validate(); err != nil {
		return err
	}
	owned := Frame(stages.Owned, protocol.binding)
	if err := exchange(owned[:], command[:]); err != nil {
		return err
	}
	use := Frame(stages.Use, protocol.binding)
	if !bytes.Equal(command[:], use[:]) {
		return ErrProtocol
	}
	if err := validate(); err != nil {
		return err
	}
	if err := brokerStep(ctx, operations.Scope, &owner.progress.Commit, func() error { return operations.CommitSuccessor(ctx) }); err != nil {
		return err
	}
	if err := validate(); err != nil {
		return err
	}
	ack := Frame(stages.Ack, protocol.binding)
	if err := brokerStep(ctx, operations.Scope, &owner.progress.Acknowledgement, func() error {
		return operations.Exchange(ctx, ack[:], nil)
	}); err != nil {
		return err
	}
	if err := brokerStep(ctx, operations.Scope, &owner.progress.Close, operations.CloseTransport); err != nil {
		return err
	}
	completed = true
	return nil
}
