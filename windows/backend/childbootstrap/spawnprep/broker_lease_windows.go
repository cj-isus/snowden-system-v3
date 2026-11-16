//go:build windows

package spawnprep

import (
	"context"
	"errors"
	"sync"
	"time"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"golang.org/x/sys/windows"
)

// BrokerLease owns only original query/sync values delivered to this child.
// Publish zero before Adopt; never copy it. A failed/partial bootstrap is terminal:
// no arbitrary Close, re-adoption or PID acquisition. Job queries stay open until
// OS process exit; only successfully awaited origins/tokens are explicitly closed.
type BrokerLease struct {
	mu           sync.Mutex
	claimed      [4]uint64
	handles      [4]windows.Handle
	purpose      childbootstrap.BrokerPurpose
	singleOrigin bool // GUI7/R2: same retained ancestor and one original App predecessor

	identity                                                  brokerIdentity
	closeAttempted                                            [4]bool
	adoptAttempted, adopted, failed                           bool
	awaitAttempted, awaitReturned, waitPending, originsExited bool
}

func (lease *BrokerLease) finish(completed *bool) {
	if !*completed {
		lease.failed = true // native/callback panic/Goexit cannot clear original facts
	}
	lease.mu.Unlock()
}

func (lease *BrokerLease) Adopt(ctx context.Context, purpose childbootstrap.BrokerPurpose, handles [4]uint64, scope func(context.Context) error) error {
	return lease.adopt(ctx, purpose, handles, false, scope)
}

func (lease *BrokerLease) AdoptAncestor(ctx context.Context, handles [3]uint64, scope func(context.Context) error) error {
	return lease.adopt(ctx, childbootstrap.BrokerSuccessor, [4]uint64{handles[0], handles[1], handles[2], 0}, true, scope)
}

func (lease *BrokerLease) adopt(ctx context.Context, purpose childbootstrap.BrokerPurpose, handles [4]uint64, singleOrigin bool, scope func(context.Context) error) error {
	if lease == nil || !lease.mu.TryLock() {
		return ErrProcess
	}
	completed := false
	defer lease.finish(&completed)
	if lease.adoptAttempted || lease.failed {
		return ErrProcess
	}
	lease.adoptAttempted, lease.purpose, lease.claimed = true, purpose, handles
	lease.singleOrigin = singleOrigin
	protocol, err := childbootstrap.NewBrokerProtocol(purpose, [32]byte{})
	valid := protocol.ValidHandles(handles)
	if singleOrigin {
		valid = purpose == childbootstrap.BrokerSuccessor && handles[3] == 0 &&
			childbootstrap.ValidAncestorHandles([3]uint64{handles[0], handles[1], handles[2]})
	}
	if err != nil || !valid {
		return ErrProcess
	}
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return err
	}
	count := 4
	if purpose == childbootstrap.BrokerControl || singleOrigin {
		count = 3
	}
	for index := 0; index < count; index++ {
		if err := brokerHandle(handles[index]); err != nil {
			return err
		}
		lease.handles[index] = windows.Handle(handles[index])
	}
	if err := lease.validateJobs(ctx, scope); err != nil {
		return err
	}
	if err := lease.identity.prepare(lease.processHandles(), purpose, singleOrigin); err != nil {
		return err
	}
	if err := lease.validateProcesses(true); err != nil {
		return err
	}
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return err
	}
	lease.adopted = true
	completed = true
	return nil
}

func (lease *BrokerLease) jobHandles() (ancestor, role windows.Handle, policy BrokerJobRole) {
	if lease.purpose == childbootstrap.BrokerControl {
		return lease.handles[0], lease.handles[1], BrokerJobHelper
	}
	return lease.handles[2], lease.handles[0], BrokerJobGUI
}

func (lease *BrokerLease) processHandles() [2]windows.Handle {
	if lease.singleOrigin {
		return [2]windows.Handle{lease.handles[1], 0}
	}
	if lease.purpose == childbootstrap.BrokerControl {
		return [2]windows.Handle{lease.handles[2], 0}
	}
	return [2]windows.Handle{lease.handles[1], lease.handles[3]}
}

func (lease *BrokerLease) validateJobs(ctx context.Context, scope func(context.Context) error) error {
	ancestor, role, policy := lease.jobHandles()
	if err := processIsInJob.Find(); err != nil {
		return errors.Join(ErrProcess, err)
	}
	for _, item := range []struct {
		handle windows.Handle
		policy BrokerJobRole
	}{{ancestor, BrokerJobAncestor}, {role, policy}} {
		if err := childbootstrap.CheckScope(ctx, scope); err != nil {
			return err
		}
		var member int32
		ok, _, _ := processIsInJob.Call(uintptr(windows.CurrentProcess()), uintptr(item.handle), uintptr(unsafe.Pointer(&member)))
		if ok == 0 || member == 0 {
			return ErrProcess
		}
		actual, err := QueryBrokerJobLimits(item.handle, item.policy)
		if err != nil {
			return err
		}
		if item.policy == BrokerJobHelper {
			accounting, err := QueryAccounting(item.handle)
			if err != nil || accounting.ActiveProcesses != 1 || accounting.TotalProcesses != 1 || accounting.TerminatedProcesses != 0 ||
				accounting.TotalUserTime < 0 || accounting.TotalUserTime > int64(BrokerHelperUserTime/UserTimeTick) ||
				actual.PeakJobMemoryUsed > BrokerHelperCommitBytes || actual.PeakProcessMemoryUsed > BrokerHelperCommitBytes {
				return errors.Join(ErrLimits, err)
			}
		}
	}
	return childbootstrap.CheckScope(ctx, scope)
}

func (lease *BrokerLease) validateProcesses(requireAlive bool) error {
	processes := lease.processHandles()
	if err := lease.identity.validate(processes, lease.purpose, requireAlive, lease.singleOrigin); err != nil {
		return err
	}
	if !requireAlive {
		return nil // exited origins are awaited, not reclassified as live Job members
	}
	ancestor, role, _ := lease.jobHandles()
	for _, process := range processes {
		if process == 0 {
			continue
		}
		if err := brokerMembership(process, role, false); err != nil {
			return err // no original predecessor may belong to this child's role Job
		}
	}
	if lease.purpose == childbootstrap.BrokerSuccessor {
		return brokerMembership(processes[0], ancestor, true) // original helper shares the GUI's ancestor
	}
	return nil
}

func brokerMembership(process, job windows.Handle, want bool) error {
	var member int32
	ok, _, _ := processIsInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&member)))
	if ok == 0 || (member != 0) != want {
		return ErrProcess
	}
	return nil
}

func (lease *BrokerLease) validate(ctx context.Context, scope func(context.Context) error) error {
	if !lease.adopted || lease.failed || lease.waitPending || lease.awaitAttempted && !lease.originsRetired() {
		return ErrProcess
	}
	if err := lease.validateJobs(ctx, scope); err != nil {
		return err
	}
	if !lease.originsRetired() {
		if err := lease.validateProcesses(true); err != nil {
			return err
		}
	} else if err := lease.identity.validateSelf(); err != nil {
		return err
	}
	return childbootstrap.CheckScope(ctx, scope)
}

func (lease *BrokerLease) Validate(ctx context.Context, scope func(context.Context) error) error {
	if lease == nil || !lease.mu.TryLock() {
		return ErrProcess
	}
	completed := false
	defer lease.finish(&completed)
	if err := lease.validate(ctx, scope); err != nil {
		return err
	}
	completed = true
	return nil
}

// Borrow synchronously under the original mutex. Do not retain/close any handle
// or reenter this lease. The caller checks original file paths/pipe peer here.
// For control, parent==origin App; successor has distinct helper/App processes.
func (lease *BrokerLease) WithChain(ctx context.Context, scope func(context.Context) error,
	use func(ancestor, roleJob, parent, origin windows.Handle) error) error {
	if lease == nil || use == nil || !lease.mu.TryLock() {
		return ErrProcess
	}
	completed := false
	defer lease.finish(&completed)
	if lease.awaitAttempted {
		return ErrProcess
	}
	if err := lease.validate(ctx, scope); err != nil {
		return err
	}
	ancestor, role, _ := lease.jobHandles()
	processes := lease.processHandles()
	origin := processes[1]
	if lease.purpose == childbootstrap.BrokerControl || lease.singleOrigin {
		origin = processes[0]
	}
	if err := use(ancestor, role, processes[0], origin); err != nil {
		return err
	}
	if err := lease.validate(ctx, scope); err != nil {
		return err
	}
	completed = true
	return nil
}

func (lease *BrokerLease) originsRetired() bool {
	return lease.purpose == childbootstrap.BrokerSuccessor && lease.awaitReturned && lease.originsExited &&
		!lease.waitPending && lease.handles[1] == 0 && lease.handles[3] == 0 && lease.identity.closed()
}

// WithAncestor borrows the same retained lifetime reference for a future explicit
// handoff, including after both origins have exited. It cannot grant Job control
// rights or authorize the successor. Do not retain/close/reenter from callback.
func (lease *BrokerLease) WithAncestor(ctx context.Context, scope func(context.Context) error, use func(windows.Handle) error) error {
	if use == nil {
		return ErrProcess
	}
	return lease.WithJobs(ctx, scope, func(ancestor, current windows.Handle) error { return use(ancestor) })
}

// WithJobs additionally lends this GUI/helper role Job, so a successor can prove
// it left the old role Job while remaining in the same ancestor. No raw getter.
// After confirmed origin retirement a returned operation error/cancellation may
// preserve these permanent source refs; every new borrow validates them afresh.
// Nonreturn or a structural source validation failure still burns positive use.
func (lease *BrokerLease) WithJobs(ctx context.Context, scope func(context.Context) error, use func(ancestor, current windows.Handle) error) error {
	if lease == nil || use == nil || !lease.mu.TryLock() {
		return ErrProcess
	}
	completed := false
	defer lease.finish(&completed)
	reusable := lease.originsRetired() // permanent GUI query refs; this borrow changes no source slot
	if err := lease.validate(ctx, scope); err != nil {
		completed = reusable && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
		return err
	}
	ancestor, current, _ := lease.jobHandles()
	callbackErr := use(ancestor, current)
	if err := lease.validate(ctx, scope); err != nil {
		completed = reusable && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
		return errors.Join(callbackErr, err)
	}
	completed = callbackErr == nil || reusable
	return callbackErr // a returned candidate error does not destroy a confirmed permanent source
}

// OriginsRetired describes these process/token slots only, never whole-tree
// cleanup or application/forwarding authority.
func (lease *BrokerLease) OriginsRetired() bool {
	if lease == nil || !lease.mu.TryLock() {
		return false
	}
	defer lease.mu.Unlock()
	return lease.adopted && !lease.failed && lease.originsRetired()
}

func (lease *BrokerLease) AwaitOrigins(ctx context.Context, scope func(context.Context) error) error {
	if lease == nil || !lease.mu.TryLock() {
		return ErrProcess
	}
	completed := false
	defer lease.finish(&completed)
	if !lease.adopted || lease.failed || lease.awaitAttempted || lease.purpose != childbootstrap.BrokerSuccessor {
		return ErrProcess
	}
	lease.awaitAttempted = true
	err := lease.await(ctx, scope)
	lease.awaitReturned = true // only an actual function return, including failure
	if err != nil {
		return err
	}
	completed = true
	return nil
}

func (lease *BrokerLease) await(ctx context.Context, scope func(context.Context) error) error {
	if err := childbootstrap.CheckScope(ctx, scope); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := lease.validateJobs(ctx, scope); err != nil {
		return err
	}
	// Helper may already have exited after its ACK/EOF. Token identity remains
	// checked, but liveness is now established only by the actual original Wait.
	if err := lease.validateProcesses(false); err != nil {
		return err
	}
	processes := lease.processHandles()
	count := 2
	if lease.singleOrigin {
		count = 1
	}
	for {
		if err := childbootstrap.CheckScope(ctx, scope); err != nil {
			return err
		}
		lease.waitPending = true
		state, err := windows.WaitForMultipleObjects(processes[:count], true, 100)
		lease.originsExited = err == nil && state == windows.WAIT_OBJECT_0
		lease.waitPending = false
		if err != nil || state != windows.WAIT_OBJECT_0 && state != uint32(windows.WAIT_TIMEOUT) {
			return errors.Join(ErrProcess, err)
		}
		if err := childbootstrap.CheckScope(ctx, scope); err != nil {
			return err
		}
		if lease.originsExited {
			break
		}
	}
	if err := lease.identity.close(); err != nil {
		return err
	}
	for _, index := range []int{1, 3} {
		if lease.singleOrigin && index == 3 {
			continue
		}
		if err := CloseOnce(&lease.handles[index], &lease.closeAttempted[index]); err != nil {
			return err
		}
	}
	return lease.validateJobs(ctx, scope)
}
