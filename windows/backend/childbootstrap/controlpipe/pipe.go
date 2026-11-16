// Package controlpipe owns fixed local byte/NOWAIT connections for U1 and the
// distinct broker hops. It does not authorize images, elevate processes or
// establish pre-Job tree containment.
package controlpipe

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

var ErrPipe = errors.New("child bootstrap control pipe unavailable")
var ErrCleanup = errors.New("child bootstrap control pipe retirement unconfirmed")

type pipeWire uint8

const (
	wireUAC pipeWire = iota + 1
	wireBrokerControl
	wireBrokerSuccessor
	wireBrokerPinnedControl
)

func brokerWire(protocol childbootstrap.BrokerProtocol) pipeWire {
	if protocol.PinnedControl() {
		return wireBrokerPinnedControl
	}
	switch protocol.Purpose() {
	case childbootstrap.BrokerControl:
		return wireBrokerControl
	case childbootstrap.BrokerSuccessor:
		return wireBrokerSuccessor
	default:
		return 0
	}
}

func (wire pipeWire) validExchange(send, receive int) bool {
	if wire == wireUAC {
		return childbootstrap.ValidExchange(send, receive)
	}
	var purpose childbootstrap.BrokerPurpose
	switch wire {
	case wireBrokerControl, wireBrokerPinnedControl:
		purpose = childbootstrap.BrokerControl
	case wireBrokerSuccessor:
		purpose = childbootstrap.BrokerSuccessor
	default:
		return false
	}
	protocol, err := childbootstrap.NewBrokerProtocol(purpose, [32]byte{})
	return err == nil && protocol.ValidExchange(send, receive)
}

func Name(nonce string) (string, error) {
	return wireName(wireUAC, nonce)
}

func wireName(wire pipeWire, nonce string) (string, error) {
	var prefix string
	switch wire {
	case wireUAC:
		prefix = `\\.\pipe\snowden-uac-v1-`
	case wireBrokerControl:
		prefix = `\\.\pipe\snowden-broker-v1-`
	case wireBrokerPinnedControl:
		prefix = `\\.\pipe\snowden-broker-v2-`
	case wireBrokerSuccessor:
		prefix = `\\.\pipe\snowden-broker-gui-v1-`
	default:
		return "", ErrPipe
	}
	if len(nonce) != 64 {
		return "", ErrPipe
	}
	raw, err := hex.DecodeString(nonce)
	if err != nil || hex.EncodeToString(raw) != nonce {
		return "", ErrPipe
	}
	return prefix + nonce, nil
}

func checkScope(ctx context.Context, check func(context.Context) error) error {
	if err := childbootstrap.CheckScope(ctx, check); err != nil {
		if err == childbootstrap.ErrScope {
			return ErrPipe
		}
		return err
	}
	return nil
}

func pause(ctx context.Context, check func(context.Context) error) error {
	if err := checkScope(ctx, check); err != nil {
		return err
	}
	delay := 10 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		delay = min(delay, time.Until(deadline))
	}
	if delay <= 0 {
		return context.DeadlineExceeded
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return checkScope(ctx, check)
	}
}
