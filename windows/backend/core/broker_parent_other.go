//go:build !windows

package core

import (
	"context"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
)

type brokerParentNative struct{}

func (*brokerParentNative) execute(context.Context, nativeImageLease, bool, childbootstrap.BrokerProtocol, func(context.Context) error) error {
	return ErrElevatedGUIUnavailable
}
func (*brokerParentNative) validate(context.Context, nativeImageLease) error {
	return ErrElevatedGUIUnavailable
}
func (*brokerParentNative) alive(context.Context) error { return ErrElevatedGUIUnavailable }
func (*brokerParentNative) acceptPipe(context.Context, *controlpipe.BrokerPipe, func(context.Context) error) error {
	return ErrElevatedGUIUnavailable
}
func (*brokerParentNative) awaitExit(context.Context, nativeImageLease, func(context.Context) error) error {
	return ErrElevatedGUIUnavailable
}
func (*brokerParentNative) remoteHandles() [4]uint64 { return [4]uint64{} }
func (*brokerParentNative) retire() error            { return nil }
