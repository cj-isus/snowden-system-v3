package core

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/uot"
)

const (
	maxUotPayloadBytes = 1<<16 - 1
	maxUotAddressBytes = 1 + 1 + 255 + 2 // family + FQDN length + name + port
)

type uotWrite struct {
	owner                                           *ownedUotConnection
	input, frame                                    *buf.Buffer
	done                                            chan struct{}
	inputReleaseStarted, inputReleaseReturned       bool
	frameAllocationStarted, frameAllocationReturned bool
	frameReleaseStarted, frameReleaseReturned       bool
	writeStarted, writeReturned                     bool
	failed                                          bool
	result                                          error // immutable after original done; never read while native work is pending
}

func (conn *ownedUotConnection) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) (finalErr error) {
	if buffer == nil {
		return ErrForwardingNotAdmitted
	}
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()
	conn.mu.Lock()
	if conn.closing.Load() || conn.protocolFailed.Load() || conn.pendingWrite != nil {
		conn.mu.Unlock()
		return conn.gate.rejectPacketBuffer(buffer, ErrForwardingNotAdmitted)
	}
	write := &uotWrite{owner: conn, input: buffer, done: make(chan struct{})}
	conn.pendingWrite = write // BEFORE buffer metadata/copy/release/allocation or native IO
	conn.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		finalErr = write.finish(finalErr, recovered != nil || !returned)
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // no fabricated partial wire/buffer outcome
		}
	}()
	finalErr = write.send(destination)
	returned = true
	return finalErr
}

func (write *uotWrite) send(destination M.Socksaddr) error {
	length := write.input.Len()
	if length < 0 || length > maxUotPayloadBytes {
		return errors.New("UoT packet exceeds uint16 frame length")
	}
	addressLength := 0
	if !write.owner.request.IsConnect {
		if len(destination.Fqdn) > 255 {
			return errors.New("UoT destination exceeds address bound")
		}
		addressLength = uot.AddrParser.AddrPortLen(destination)
		if addressLength < 0 || addressLength > maxUotAddressBytes {
			return errors.New("UoT destination exceeds address bound")
		}
	}
	write.frameAllocationStarted = true
	write.frame = buf.NewSize(addressLength + 2 + length)
	write.frameAllocationReturned = true
	if !write.owner.request.IsConnect {
		if err := uot.AddrParser.WriteAddrPort(write.frame, destination); err != nil {
			return err
		}
	}
	if write.frame.Len() != addressLength {
		return errors.New("UoT address codec length mismatch")
	}
	if err := binary.Write(write.frame, binary.BigEndian, uint16(length)); err != nil {
		return err
	}
	if copied, err := write.frame.Write(write.input.Bytes()); err != nil || copied != length {
		if err != nil {
			return err
		}
		return io.ErrShortBuffer
	}
	if write.frame.Len() != addressLength+2+length {
		return errors.New("UoT frame codec length mismatch")
	}
	if err := write.releaseInput(); err != nil {
		return err // caller buffer is never lent to Write; frame remains independently owned
	}
	write.writeStarted = true
	n, err := write.owner.conn.Write(write.frame.Bytes())
	write.writeReturned = true
	if err == nil && n != write.frame.Len() {
		err = io.ErrShortWrite
	}
	if err != nil {
		write.owner.protocolFailed.Store(true) // no frame retry after a partial TCP write
	}
	return err
}

func (write *uotWrite) releaseInput() (finalErr error) {
	if write.inputReleaseStarted {
		if write.inputReleaseReturned {
			return nil
		}
		return ErrEngineCleanupFailed
	}
	write.inputReleaseStarted = true
	returned := false
	defer func() {
		if recover() != nil || !returned {
			finalErr = ErrEngineCleanupFailed
		}
		write.inputReleaseReturned = returned
	}()
	write.input.Release()
	returned = true
	write.input = nil
	return nil
}

func (write *uotWrite) releaseFrame() (finalErr error) {
	if !write.frameAllocationStarted {
		return nil
	}
	if !write.frameAllocationReturned || write.writeStarted && !write.writeReturned {
		return ErrEngineCleanupFailed // original Write/constructor unknown, retain frame
	}
	if write.frameReleaseStarted {
		if write.frameReleaseReturned {
			return nil
		}
		return ErrEngineCleanupFailed
	}
	write.frameReleaseStarted = true
	returned := false
	defer func() {
		if recover() != nil || !returned {
			finalErr = ErrEngineCleanupFailed
		}
		write.frameReleaseReturned = returned
	}()
	write.frame.Release()
	returned = true
	write.frame = nil
	return nil
}

func (write *uotWrite) finish(cause error, fault bool) (finalErr error) {
	complete := false
	defer func() {
		if recover() != nil || !complete {
			write.failed = true
			finalErr = ErrEngineCleanupFailed
		}
		write.result = finalErr
		if write.failed {
			write.owner.gate.noteRouteFault()
		}
		close(write.done)
		if !write.failed {
			write.owner.mu.Lock()
			if write.owner.pendingWrite == write {
				write.owner.pendingWrite = nil
			}
			write.owner.mu.Unlock()
		} // failed original input/frame stays attached; no retry/replacement
	}()
	write.failed = fault
	inputErr := write.releaseInput()
	frameErr := write.releaseFrame()
	write.failed = write.failed || inputErr != nil || frameErr != nil || !write.inputReleaseReturned
	finalErr = errors.Join(cause, inputErr, frameErr)
	if write.failed {
		finalErr = errors.Join(finalErr, ErrEngineCleanupFailed)
	}
	complete = true
	return finalErr
}
