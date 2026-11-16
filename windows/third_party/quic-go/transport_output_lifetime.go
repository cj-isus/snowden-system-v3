package quic

import (
	"crypto/rand"
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

// One existing transport send worker owns these original views. Admission seal
// is guarded by Transport.mutex; other fields are read only after done closes.
type transportOutputState struct {
	wake                       chan struct{}
	stopPending, stopActive    bool
	stopConn                   rawConn
	sealed, failed             bool
	done                       chan struct{}
	activeClose, drainClose    *closePacket
	closeOOB                   []byte
	resetInput, drainReset     receivedPacket
	resetActive, drainingReset bool
	resetData, resetOOB        []byte
}

// Called with Transport.mutex held. Rejected cache registrations do no native
// I/O; an automatic single-use read stop belongs to the existing worker.
func (t *Transport) requestClosedReadStopLocked() {
	if t.output.sealed || t.output.wake == nil {
		return
	}
	t.output.stopPending = true
	select {
	case t.output.wake <- struct{}{}:
	default:
	}
}

func (t *Transport) stopClosedRead() {
	t.mutex.Lock()
	if !t.output.stopPending || t.output.sealed {
		t.output.stopPending = false
		t.mutex.Unlock()
		return
	}
	t.output.stopPending = false
	t.output.stopConn = t.closedStopTargetLocked()
	t.output.stopActive = t.output.stopConn != nil
	t.mutex.Unlock()
	if t.output.stopActive {
		t.sealTransport()
		t.abortTransport()
		t.output.stopConn, t.output.stopActive = nil, false
	}
}

func (t *Transport) writeClosedRetry(p closePacket) bool {
	t.output.activeClose = &p
	if p.owner == nil {
		t.output.failed = true
		return false
	}
	t.mutex.Lock()
	sealed := t.output.sealed || t.closeErr != nil
	t.mutex.Unlock()
	if sealed {
		p.owner.finishRetry(true)
		t.output.activeClose = nil
		return true
	}
	payload := p.owner.payload // the queued original ref prevents clearing it
	t.output.closeOOB = p.info.OOB()
	n, err := t.conn.WritePacket(payload, p.addr, t.output.closeOOB, 0, protocol.ECNUnsupported)
	if errors.Is(packetWriteResult(n, len(payload), err), errPacketOutputUncertain) {
		p.owner.finishRetry(false)
		t.output.failed = true
		return false
	}
	p.owner.finishRetry(true) // best-effort write failures still end the actual borrow
	t.output.activeClose, t.output.closeOOB = nil, nil
	return true
}

func (t *Transport) writeStatelessReset(p receivedPacket) bool {
	t.output.resetInput, t.output.resetActive = p, true
	input := &t.output.resetInput
	t.mutex.Lock()
	sealed := t.output.sealed || t.closeErr != nil
	t.mutex.Unlock()
	if sealed {
		releaseClosedPacket(input)
		t.output.resetActive = false
		return true
	}
	connID, err := wire.ParseConnectionID(input.data, t.connIDLen)
	if err == nil {
		token := t.statelessResetter.GetStatelessResetToken(connID)
		t.output.resetData = make([]byte, protocol.MinStatelessResetSize-16, protocol.MinStatelessResetSize)
		if _, err := rand.Read(t.output.resetData); err == nil {
			t.output.resetData[0] = (t.output.resetData[0] & 0x7f) | 0x40
			t.output.resetData = append(t.output.resetData, token[:]...)
			t.output.resetOOB = input.info.OOB()
			n, writeErr := t.conn.WritePacket(t.output.resetData, input.remoteAddr, t.output.resetOOB, 0, protocol.ECNUnsupported)
			if errors.Is(packetWriteResult(n, len(t.output.resetData), writeErr), errPacketOutputUncertain) {
				t.output.failed = true
				return false
			}
		}
	}
	releaseClosedPacket(input) // actual write returned; one shared input ref
	t.output.resetData, t.output.resetOOB = nil, nil
	t.output.resetActive = false
	return true
}

func (t *Transport) finishTransportOutput(returned *bool) {
	cleanupReturned := false
	defer func() {
		if !*returned || !cleanupReturned || t.output.activeClose != nil || t.output.resetActive || t.output.drainingReset || t.output.stopActive {
			t.output.failed = true
		}
		t.mutex.Lock()
		if t.output.failed {
			t.closedCache.failed = true
		}
		t.settleClosedCacheLocked()
		close(t.output.done)
		t.mutex.Unlock()
	}()
	t.mutex.Lock()
	t.output.sealed, t.closedCache.sealed = true, true
	t.mutex.Unlock()
	// Producers share the seal fence, so no queued original can arrive later.
	for {
		select {
		case p := <-t.closeQueue:
			t.output.drainClose = &p
			if p.owner == nil {
				t.output.failed = true
			} else {
				p.owner.finishRetry(true)
			}
			t.output.drainClose = nil
		default:
			goto resetQueue
		}
	}
resetQueue:
	for {
		select {
		case p := <-t.statelessResetQueue:
			t.output.drainReset, t.output.drainingReset = p, true
			releaseClosedPacket(&t.output.drainReset)
			t.output.drainingReset = false
		default:
			cleanupReturned = true
			return
		}
	}
}

func (t *Transport) waitTransportOutput() error {
	if t.output.done != nil {
		<-t.output.done
	}
	t.retireClosedCaches()
	if t.closedCache.done != nil {
		<-t.closedCache.done
	}
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.output.failed || t.closedCache.failed {
		return errPacketOutputUncertain
	}
	return nil
}
