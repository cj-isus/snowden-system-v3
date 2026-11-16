//go:build !windows

package core

import (
	"context"

	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
)

type uacControlPipe struct{ native controlpipe.Pipe }

func (*uacControlPipe) validateParent(context.Context, *updateGUINativeLease, nativeImageLease) error {
	return ErrElevatedGUIUnavailable
}
func (pipe *uacControlPipe) close() error {
	if pipe == nil {
		return nil
	}
	return uacControlPipeError(pipe.native.Close())
}
func (pipe *uacControlPipe) closeAfterServer(ctx context.Context) error {
	if pipe == nil {
		return ErrElevatedGUIUnavailable
	}
	return uacControlPipeError(pipe.native.CloseAfterServer(ctx, contextError))
}
