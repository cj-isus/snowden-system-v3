package core

import (
	"context"
	"errors"
	"net"
	"time"
)

func (owner *mixedHTTPPipe) perform(call func() error) (finalErr error) {
	owner.mu.Lock()
	if owner.closing || owner.failed || owner.ctx.Err() != nil || !owner.acquisitionReturned {
		owner.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	owner.ioPending++
	owner.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		owner.mu.Lock()
		owner.ioPending--
		owner.failed = owner.failed || fault
		if owner.closing && owner.ioPending == 0 && !owner.ioDoneClosed {
			owner.ioDoneClosed = true
			close(owner.ioDone)
		}
		owner.mu.Unlock()
		if fault {
			owner.cancel()
			owner.input.gate.noteRouteFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	finalErr = call()
	returned = true
	return finalErr
}

func (owner *mixedHTTPPipe) joinClose() error {
	owner.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
	defer cancel()
	_ = mixedOriginalDone(ctx, owner.closeWait)
	owner.mu.Lock()
	returned, err := owner.closeReturned, owner.closeErr // fresh original fact after the final channel observation
	unknown := !returned || err != nil
	owner.failed = owner.failed || unknown // preserve uncertainty even if a Transport ignores Close's error
	owner.mu.Unlock()
	if unknown {
		owner.input.gate.noteRouteFault()
		return errors.Join(ErrEngineCleanupFailed, err)
	}
	return nil // original end Close results only, not whole-pair/task retirement
}

// Deliberately no embedding/Upstream/ReadWaiter/vector interface: in the pinned
// pipe waiter an error after receiving bytes can skip the writer's ack. Each
// actual Read/Write stays in its original receipt instead of escaping via cast.
type mixedHTTPPipeEnd struct {
	owner *mixedHTTPPipe
	index int
}

func (end *mixedHTTPPipeEnd) Read(buffer []byte) (n int, err error) {
	err = end.owner.perform(func() error { n, err = end.owner.raw[end.index].Read(buffer); return err })
	return
}
func (end *mixedHTTPPipeEnd) Write(buffer []byte) (n int, err error) {
	err = end.owner.perform(func() error { n, err = end.owner.raw[end.index].Write(buffer); return err })
	return
}
func (end *mixedHTTPPipeEnd) Close() error         { return end.owner.joinClose() }
func (end *mixedHTTPPipeEnd) LocalAddr() net.Addr  { return end.owner.local[end.index] }
func (end *mixedHTTPPipeEnd) RemoteAddr() net.Addr { return end.owner.remote[end.index] }
func (end *mixedHTTPPipeEnd) SetDeadline(value time.Time) error {
	return end.owner.perform(func() error { return end.owner.raw[end.index].SetDeadline(value) })
}
func (end *mixedHTTPPipeEnd) SetReadDeadline(value time.Time) error {
	return end.owner.perform(func() error { return end.owner.raw[end.index].SetReadDeadline(value) })
}
func (end *mixedHTTPPipeEnd) SetWriteDeadline(value time.Time) error {
	return end.owner.perform(func() error { return end.owner.raw[end.index].SetWriteDeadline(value) })
}
func (*mixedHTTPPipeEnd) ReaderReplaceable() bool { return false }
func (*mixedHTTPPipeEnd) WriterReplaceable() bool { return false }

var _ net.Conn = (*mixedHTTPPipeEnd)(nil)
