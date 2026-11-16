package core

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/canceler"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type mixedAssociation struct {
	packet *mixedPacket
	tcp    net.Conn
	ctx    context.Context
	mu     sync.Mutex // values and bounded buffer memory operations only, never native IO/callbacks

	frameDone, watcherDone                chan struct{}
	frameReturned, watcherStarted         bool
	watcherReturned, watcherReadReturned  bool
	frameErr, watcherErr                  error
	failed                                bool
	first                                 *buf.Buffer
	allocationStarted, allocationReturned bool
	readStarted, readReturned             bool
	copyStarted, copyReturned             bool
	firstConsumed, firstClosed            bool
	releaseStarted, releaseReturned       bool
	retireStarted, retireReturned         bool
	destination                           M.Socksaddr
}

func (association *mixedAssociation) stop() {
	association.packet.input.cancel()
	association.packet.cancel() // cancel requests are not native cleanup receipts
}

func (association *mixedAssociation) frame(call func() error) (finalErr error) {
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		association.mu.Lock()
		association.frameReturned, association.frameErr = returned, finalErr
		association.failed = association.failed || fault
		association.mu.Unlock()
		if fault || finalErr != nil {
			association.stop()
		}
		if fault {
			association.packet.input.gate.noteRouteFault()
		}
		close(association.frameDone)
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	finalErr = call()
	returned = true
	return finalErr
}

func (association *mixedAssociation) startWatcher() error {
	association.mu.Lock()
	if association.watcherStarted || association.ctx.Err() != nil {
		association.mu.Unlock()
		return ErrForwardingNotAdmitted
	}
	association.watcherStarted = true // original task declared before Go
	association.mu.Unlock()
	go association.watch()
	return nil
}

func (association *mixedAssociation) watch() {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		association.mu.Lock()
		association.watcherReturned = returned
		association.failed = association.failed || fault
		association.mu.Unlock()
		association.stop() // original native owners perform Close; never wait on whole association here
		if fault {
			association.packet.input.gate.noteRouteFault()
		}
		close(association.watcherDone)
	}()
	var value [1]byte
	_, err := association.tcp.Read(value[:])
	association.mu.Lock()
	association.watcherReadReturned, association.watcherErr = true, err
	association.mu.Unlock()
	returned = true // EOF/transport error remains distinct from custody failure
}

func (association *mixedAssociation) readFirst(conn N.PacketConn) (M.Socksaddr, error) {
	association.mu.Lock()
	association.allocationStarted = true
	association.mu.Unlock()
	first := buf.NewPacket()
	association.mu.Lock()
	association.first, association.allocationReturned, association.readStarted = first, true, true
	association.mu.Unlock()
	destination, err := conn.ReadPacket(first)
	association.mu.Lock()
	association.readReturned, association.destination = true, destination
	association.mu.Unlock()
	return destination, err
}

// Caller holds association.mu. The buffer is never handed to an opaque cache
// or refcount owner. A failed release retains the original pointer, no retry.
func (association *mixedAssociation) releaseFirstLocked() error {
	if !association.allocationStarted {
		return nil
	}
	if !association.allocationReturned || !association.readReturned || association.copyStarted && !association.copyReturned {
		return ErrEngineCleanupFailed
	}
	if association.releaseStarted {
		if association.releaseReturned {
			return nil
		}
		return ErrEngineCleanupFailed
	}
	if association.first == nil {
		return ErrEngineCleanupFailed
	}
	association.releaseStarted = true
	association.first.Release()
	association.releaseReturned = true
	association.first = nil // never access a buffer returned to the SDK pool
	return nil
}

func (association *mixedAssociation) closeFirst() (finalErr error) {
	association.mu.Lock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		association.failed = association.failed || fault || finalErr != nil
		failed := association.failed
		association.mu.Unlock()
		if failed {
			association.packet.input.gate.noteRouteFault()
		}
	}()
	association.firstClosed = true
	finalErr = association.releaseFirstLocked()
	returned = true
	return finalErr
}

func (association *mixedAssociation) consumeFirst(buffer *buf.Buffer) (destination M.Socksaddr, handled bool, finalErr error) {
	association.mu.Lock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		association.failed = association.failed || fault
		failed := association.failed
		association.mu.Unlock()
		if failed {
			association.packet.input.gate.noteRouteFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // no invented packet/copy or buffer-ownership result
		}
	}()
	returned = true
	if association.firstClosed || association.ctx.Err() != nil {
		return destination, true, ErrForwardingNotAdmitted
	}
	if association.firstConsumed {
		return destination, false, nil
	}
	if buffer == nil {
		return destination, true, ErrForwardingNotAdmitted
	}
	if !association.readReturned || association.first == nil {
		association.failed = true
		return destination, true, ErrEngineCleanupFailed
	}
	if buffer.FreeLen() < association.first.Len() {
		return destination, true, io.ErrShortBuffer // retain whole original packet for a suitable caller/Close
	}
	association.copyStarted, association.copyReturned = true, false
	returned = false
	size := association.first.Len()
	n, err := buffer.Write(association.first.Bytes())
	association.copyReturned = true
	returned = true
	if err != nil || n != size {
		if err == nil {
			err = io.ErrShortBuffer
		}
		return destination, true, err
	}
	returned = false
	releaseErr := association.releaseFirstLocked()
	returned = true
	if releaseErr != nil {
		association.failed = true
		return destination, true, releaseErr
	}
	association.firstConsumed = true
	return association.destination, true, nil
}

func (association *mixedAssociation) retire() (finalErr error) {
	association.mu.Lock()
	if association.retireStarted {
		association.mu.Unlock()
		return ErrEngineCleanupFailed
	}
	association.retireStarted = true
	association.mu.Unlock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		association.mu.Lock()
		association.retireReturned = returned
		association.failed = association.failed || fault || finalErr != nil
		association.mu.Unlock()
	}()
	<-association.frameDone
	association.mu.Lock()
	watcher := association.watcherStarted
	association.mu.Unlock()
	if watcher {
		<-association.watcherDone
	}
	bufferErr := association.closeFirst() // attempt known buffer cleanup even after a frame fault
	association.mu.Lock()
	failed := association.failed || !association.frameReturned || watcher && (!association.watcherReturned || !association.watcherReadReturned)
	association.mu.Unlock()
	returned = true
	if failed {
		return errors.Join(ErrEngineCleanupFailed, bufferErr)
	}
	return bufferErr
}

type mixedAssociationConnection struct {
	association *mixedAssociation
	inner       N.PacketConn
}

func (conn *mixedAssociationConnection) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	destination, handled, err := conn.association.consumeFirst(buffer)
	if handled {
		return destination, err
	}
	return conn.inner.ReadPacket(buffer)
}
func (conn *mixedAssociationConnection) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return conn.inner.WritePacket(buffer, destination)
}
func (conn *mixedAssociationConnection) Close() error {
	conn.association.stop()
	bufferErr := conn.association.closeFirst()
	// This may run inside the still-active delegation frame. Wait only original
	// native Close fences, never frame/watcher/whole association completion here.
	return errors.Join(bufferErr, conn.association.packet.joinClose(), conn.association.packet.input.joinClose())
}
func (conn *mixedAssociationConnection) LocalAddr() net.Addr { return conn.inner.LocalAddr() }
func (conn *mixedAssociationConnection) SetDeadline(value time.Time) error {
	return conn.inner.SetDeadline(value)
}
func (conn *mixedAssociationConnection) SetReadDeadline(value time.Time) error {
	return conn.inner.SetReadDeadline(value)
}
func (conn *mixedAssociationConnection) SetWriteDeadline(value time.Time) error {
	return conn.inner.SetWriteDeadline(value)
}
func (conn *mixedAssociationConnection) Timeout() time.Duration {
	if timed, ok := conn.inner.(canceler.PacketConn); ok {
		return timed.Timeout()
	}
	return 0
}
func (conn *mixedAssociationConnection) SetTimeout(value time.Duration) bool {
	if timed, ok := conn.inner.(canceler.PacketConn); ok {
		return timed.SetTimeout(value)
	}
	return false
}
func (*mixedAssociationConnection) ReaderReplaceable() bool { return false }
func (*mixedAssociationConnection) WriterReplaceable() bool { return false }

var _ N.PacketConn = (*mixedAssociationConnection)(nil)
