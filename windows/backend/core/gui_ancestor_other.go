//go:build !windows

package core

import "context"

type updateGUIAncestorLease struct{}

func (*updateGUIAncestorLease) adopt(context.Context, [3]uint64) error { return ErrUpdateGUIBootstrap }
func (*updateGUIAncestorLease) validate(context.Context) error         { return ErrUpdateGUIBootstrap }
func (*updateGUIAncestorLease) await(context.Context) error            { return ErrUpdateGUIBootstrap }
func (*updateGUIAncestorLease) authenticate(context.Context, string) error {
	return ErrUpdateGUIBootstrap
}
