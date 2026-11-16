//go:build !windows

package core

import "context"

type nativeTUNTransfer struct{}

func duplicateTUNActor(context.Context, processHost, processHost, *nativeTUNTransfer) error {
	return ErrNativeScopeUnavailable
}
func (*nativeTUNTransfer) apply(*workerTUNScopeRequest) {}

type unavailableTUNActor struct{}

func newTUNActorResource() tunActorResource                   { return unavailableTUNActor{} }
func (unavailableTUNActor) adopt(workerTUNScopeRequest) error { return ErrNativeScopeUnavailable }
func (unavailableTUNActor) alive() bool                       { return false }
func (unavailableTUNActor) Close() error                      { return ErrNativeScopeUnavailable }
