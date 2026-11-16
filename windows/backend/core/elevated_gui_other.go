//go:build !windows

package core

import "context"

type elevatedGUINativeRequest struct{}

func (*elevatedGUINativeRequest) execute(context.Context, nativeImageLease, bool, string, func(context.Context) error) error {
	return ErrElevatedGUIUnavailable
}
func (*elevatedGUINativeRequest) validate(context.Context, nativeImageLease) error {
	return ErrElevatedGUIUnavailable
}
func (*elevatedGUINativeRequest) retire() error               { return nil }
func (*elevatedGUINativeRequest) remoteHandles() [2]uint64    { return [2]uint64{} }
func (*elevatedGUINativeRequest) alive(context.Context) error { return ErrElevatedGUIUnavailable }
func (*elevatedGUINativeRequest) acceptPipe(context.Context, *uacControlPipe, func(context.Context) error) error {
	return ErrElevatedGUIUnavailable
}
