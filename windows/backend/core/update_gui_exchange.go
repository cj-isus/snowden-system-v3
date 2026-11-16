package core

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

// Finite protocol-owned buffers survive cancellation without keeping a caller's
// mutable frame as an active read/write target. The original I/O header remains
// on the same process/ioDone; child dispatch retains it until terminal OS exit.
type updateGUIExchange struct {
	ownedPipeIO
	raw                   ownedPipeEffects
	send, receive         [childbootstrap.AncestorSeedSize]byte
	sendSize, receiveSize int
}

func (exchange *updateGUIExchange) mayHaveSent() bool {
	return exchange != nil && exchange.sendSize != 0 && exchange.raw.writeEntered.Load()
}

// This snapshot is effect data only. Plain counters/buffers require the original
// goroutine's done fence, even when the caller already returned on cancellation.
func (exchange *updateGUIExchange) replyMatches(send, receive []byte) bool {
	if exchange == nil {
		return false
	}
	select {
	case <-exchange.done:
	default:
		return false
	}
	return !exchange.raw.invalid && exchange.raw.retirementError() == nil &&
		(exchange.sendSize == 0 || exchange.raw.writeReturned) && exchange.raw.readReturned &&
		exchange.raw.written == exchange.sendSize && exchange.raw.read == exchange.receiveSize &&
		len(send) == exchange.sendSize && len(receive) == exchange.receiveSize && len(receive) != 0 &&
		bytes.Equal(exchange.send[:exchange.sendSize], send) && bytes.Equal(exchange.receive[:exchange.receiveSize], receive)
}

func validUpdateGUIExchange(send, receive int) bool {
	if childbootstrap.ValidExchange(send, receive) {
		return true
	}
	protocol, err := childbootstrap.New(childbootstrap.PurposeUpdateAncestor, [32]byte{})
	return err == nil && protocol.ValidExchange(send, receive) // capacity only; each wire checks its own grammar
}

// Exactly one fixed exchange, with original-scope checks at each physical I/O.
// A successful return joins its original goroutine before copying receive bytes.
// On failure the parent retains/retires the original process; child main exits.
// Neither OWN nor this I/O completion proves GUI readiness/installation authority.
func exchangeUpdateGUI(ctx context.Context, input io.Reader, output io.Writer, send, receive []byte, process *engineProcess) (finalErr error) {
	return exchangeUpdateGUIObserved(ctx, input, output, send, receive, process, nil)
}

func exchangeUpdateGUIObserved(ctx context.Context, input io.Reader, output io.Writer, send, receive []byte, process *engineProcess, original **updateGUIExchange) (finalErr error) {
	if original != nil && *original != nil {
		return ErrUpdateGUIBootstrap
	}
	if err := contextError(ctx); err != nil {
		return errors.Join(ErrUpdateGUIBootstrap, err)
	}
	if !validUpdateGUIExchange(len(send), len(receive)) {
		return ErrUpdateGUIBootstrap
	}
	if process != nil {
		if process.unusable || process.exited() {
			return ErrUpdateGUIBootstrap
		}
		if err := process.ioRetirementReady(); err != nil {
			process.unusable = true
			return errors.Join(ErrUpdateGUIBootstrap, err)
		}
		if err := process.ancestorScope(ctx, true); err != nil {
			process.unusable = true
			return errors.Join(ErrUpdateGUIBootstrap, err)
		}
	}
	exchange := &updateGUIExchange{ownedPipeIO: ownedPipeIO{done: make(chan struct{})}, sendSize: len(send), receiveSize: len(receive)}
	exchange.ownedPipeIO.effects = &exchange.raw
	copy(exchange.send[:], send)
	if original != nil {
		*original = exchange
	} // original decision before launching its only I/O goroutine
	if process != nil {
		process.captureIO, process.ioDone = &exchange.ownedPipeIO, exchange.done
	}
	defer func() {
		if finalErr != nil {
			if process != nil {
				process.unusable = true
			}
			finalErr = errors.Join(ErrUpdateGUIBootstrap, finalErr, exchange.completedFailure())
		}
	}()
	go func() {
		completed := false
		defer func() {
			if recovered := recover(); recovered != nil || !completed {
				exchange.err = errors.Join(exchange.err, ErrUpdateGUIBootstrap)
			}
			close(exchange.done) // goroutine-end fence; raw pending/return facts stay independent
		}()
		exchange.err = contextError(ctx)
		if exchange.err == nil && exchange.sendSize != 0 {
			writer := ownedPipeWriter{ctx: ctx, writer: output, failure: ErrUpdateGUIBootstrap, effects: &exchange.raw}
			_, exchange.err = writer.Write(exchange.send[:exchange.sendSize])
		}
		if exchange.err == nil && exchange.receiveSize != 0 {
			reader := ownedPipeReader{ctx: ctx, reader: input, failure: ErrUpdateGUIBootstrap, effects: &exchange.raw}
			_, exchange.err = io.ReadFull(reader, exchange.receive[:exchange.receiveSize])
		}
		exchange.err = errors.Join(exchange.err, contextError(ctx))
		completed = true
	}()
	intentDone, operationDone, dependencyDone := operationSourceDone(ctx)
	select {
	case <-exchange.done:
		if err := errors.Join(exchange.completedFailure(), contextError(ctx)); err != nil {
			return err
		}
		if process != nil {
			if err := process.ancestorScope(ctx, true); err != nil {
				return err // original raw write/ACK facts remain on exchange even without a successful return
			}
		}
		copy(receive, exchange.receive[:exchange.receiveSize])
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-intentDone:
		return errors.Join(context.Canceled, contextError(ctx))
	case <-operationDone:
		return errors.Join(context.Canceled, contextError(ctx))
	case <-dependencyDone:
		return errors.Join(context.Canceled, contextError(ctx))
	}
}
