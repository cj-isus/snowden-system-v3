// Package nativelease owns the query Job and original-parent handles delivered
// to a child bootstrap. It does not create/elevate processes, authenticate peers
// or images, authorize network access, or prove pre-attachment tree containment.
// Publish the zero Lease inside the original session before Adopt. Do not copy a
// Lease after use. Failure retains its original values until terminal process exit.
package nativelease

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

var ErrLease = errors.New("child bootstrap native lease unavailable")

// Scope must be a value-only original-source check and must not reenter Lease.
// The derived parent-wait deadline retains the original context and its values.
func checkScope(ctx context.Context, scope func(context.Context) error) error {
	if ctx == nil || scope == nil {
		return ErrLease
	}
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return errors.Join(ErrLease, err)
	}
	return nil
}
