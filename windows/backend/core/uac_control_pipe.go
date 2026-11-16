package core

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
)

func uacControlPipeError(err error) error {
	if errors.Is(err, controlpipe.ErrCleanup) {
		return errors.Join(ErrHelperCleanup, err)
	}
	if err == controlpipe.ErrPipe {
		return ErrElevatedGUIUnavailable
	}
	if errors.Is(err, controlpipe.ErrPipe) {
		return errors.Join(ErrElevatedGUIUnavailable, err)
	}
	return err
}

func (pipe *uacControlPipe) listen(ctx context.Context, nonce string) error {
	if pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	return uacControlPipeError(pipe.native.Listen(ctx, nonce, contextError))
}
func (pipe *uacControlPipe) dial(ctx context.Context, nonce string) error {
	if pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	return uacControlPipeError(pipe.native.Dial(ctx, nonce, contextError))
}
func (pipe *uacControlPipe) accept(ctx context.Context, expected uint32, guard func(context.Context) error) error {
	if pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	return uacControlPipeError(pipe.native.Accept(ctx, expected, func(ctx context.Context) error { return helperScope(ctx, guard) }))
}
func (pipe *uacControlPipe) exchange(ctx context.Context, send, receive []byte, guard func(context.Context) error) error {
	if pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	return uacControlPipeError(pipe.native.Exchange(ctx, send, receive, func(ctx context.Context) error { return helperScope(ctx, guard) }))
}

func (pipe *uacControlPipe) exchangeObserved(ctx context.Context, send, receive []byte, receipt *controlpipe.ExchangeReceipt, guard func(context.Context) error) error {
	if pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	return uacControlPipeError(pipe.native.ExchangeObserved(ctx, send, receive, receipt, func(ctx context.Context) error { return helperScope(ctx, guard) }))
}
