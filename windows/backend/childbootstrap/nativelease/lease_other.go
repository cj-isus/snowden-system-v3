//go:build !windows

package nativelease

import "context"

type Lease struct{}

func (*Lease) Adopt([2]uint64) error { return ErrLease }
func (*Lease) ValidateJob() error    { return ErrLease }
func (*Lease) AwaitParent(context.Context, func(context.Context) error) error {
	return ErrLease
}
