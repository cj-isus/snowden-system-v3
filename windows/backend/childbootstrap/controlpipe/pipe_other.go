//go:build !windows

package controlpipe

import "context"

type Pipe struct{}

func (*Pipe) Retire() {}

func (*Pipe) Listen(context.Context, string, func(context.Context) error) error      { return ErrPipe }
func (*Pipe) Dial(context.Context, string, func(context.Context) error) error        { return ErrPipe }
func (*Pipe) Accept(context.Context, uint32, func(context.Context) error) error      { return ErrPipe }
func (*Pipe) CheckServer(context.Context, uint32, func(context.Context) error) error { return ErrPipe }
func (*Pipe) Exchange(context.Context, []byte, []byte, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) ExchangeObserved(context.Context, []byte, []byte, *ExchangeReceipt, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) listen(context.Context, string, pipeWire, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) dial(context.Context, string, pipeWire, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) exchange(context.Context, []byte, []byte, pipeWire, bool, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) exchangeObserved(context.Context, []byte, []byte, pipeWire, bool, *ExchangeReceipt, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) readyClient(context.Context, pipeWire, func(context.Context) error) error {
	return ErrPipe
}
func (*Pipe) CloseAfterServer(context.Context, func(context.Context) error) error { return ErrPipe }
func (*Pipe) Close() error                                                        { return nil }
