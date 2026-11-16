//go:build windows

package core

import (
	"context"
	"errors"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"golang.org/x/sys/windows"
)

type windowsGUIAncestor struct{ lease *spawnprep.BrokerLease }

func retainGUIAncestor(lease *spawnprep.BrokerLease) error {
	if lease == nil || processGUIAncestor != nil {
		return ErrUpdateGUIBootstrap
	}
	processGUIAncestor = &windowsGUIAncestor{lease: lease}
	return nil
}

func (ancestor *windowsGUIAncestor) validate(ctx context.Context) error {
	if ancestor == nil || ancestor.lease == nil {
		return ErrUpdateGUIBootstrap
	}
	return ancestor.lease.WithJobs(ctx, contextError, func(root, current windows.Handle) error { return nil })
}

// Lock/borrow order: original child Process -> current GUI native lease. Both
// source Jobs stay query-only and immutable through this synchronous callback.
// Ordinary descendants stay in the current GUI role; a successor must leave it.
func (ancestor *windowsGUIAncestor) withChild(ctx context.Context, host processHost, sameRole bool, use func(root, child windows.Handle) error) error {
	owned, ok := host.(*windowsProcessHost)
	if !ok || owned == nil || ancestor == nil || ancestor.lease == nil {
		return ErrUpdateGUIBootstrap
	}
	return owned.native.WithAssigned(func(job, child windows.Handle) error {
		return ancestor.lease.WithJobs(ctx, contextError, func(root, current windows.Handle) error {
			if err := updateIsProcessInJob.Find(); err != nil {
				return err
			}
			for _, item := range []struct {
				job  windows.Handle
				want bool
			}{{root, true}, {current, sameRole}} {
				var member int32
				ok, _, _ := updateIsProcessInJob.Call(uintptr(child), uintptr(item.job), uintptr(unsafe.Pointer(&member)))
				if ok == 0 || (member != 0) != item.want {
					return ErrUpdateGUIBootstrap
				}
			}
			if use != nil {
				return use(root, child)
			}
			return contextError(ctx)
		})
	})
}

func (ancestor *windowsGUIAncestor) checkChild(ctx context.Context, host processHost) error {
	return ancestor.withChild(ctx, host, false, nil)
}

func (ancestor *windowsGUIAncestor) checkDescendant(ctx context.Context, host processHost) error {
	return ancestor.withChild(ctx, host, true, nil)
}

func (ancestor *windowsGUIAncestor) duplicate(ctx context.Context, host processHost) (remoteValue uint64, finalErr error) {
	finalErr = ancestor.withChild(ctx, host, false, func(root, child windows.Handle) error {
		var remote windows.Handle
		err := windows.DuplicateHandle(windows.CurrentProcess(), root, child, &remote, updateJobQueryAccess, false, 0)
		remoteValue = uint64(remote) // target-owned partial result; never close in this parent
		if err != nil || !createdProcessHandle(remote) {
			return errors.Join(ErrUpdateGUIBootstrap, err)
		}
		return contextError(ctx)
	})
	return
}

type updateGUIAncestorLease struct{ native spawnprep.BrokerLease }

func (lease *updateGUIAncestorLease) adopt(ctx context.Context, handles [3]uint64) error {
	if lease == nil {
		return ErrUpdateGUIBootstrap
	}
	if err := retainGUIAncestor(&lease.native); err != nil {
		return err
	}
	return lease.native.AdoptAncestor(ctx, handles, contextError)
}

func (lease *updateGUIAncestorLease) validate(ctx context.Context) error {
	return lease.native.Validate(ctx, contextError)
}

func (lease *updateGUIAncestorLease) await(ctx context.Context) error {
	return lease.native.AwaitOrigins(ctx, contextError)
}

func (lease *updateGUIAncestorLease) authenticate(ctx context.Context, expected string) error {
	if expected == "" {
		return ErrUpdateGUIBootstrap
	}
	if err := nativeProcessPathMatches(ctx, windows.CurrentProcess(), expected); err != nil {
		return err
	}
	if lease.native.OriginsRetired() {
		return lease.native.Validate(ctx, contextError)
	}
	return lease.native.WithChain(ctx, contextError, func(root, role, parent, origin windows.Handle) error {
		return nativeProcessPathMatches(ctx, parent, expected)
	})
}
