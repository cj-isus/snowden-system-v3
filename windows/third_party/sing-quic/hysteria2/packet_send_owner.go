package hysteria2

import (
	"net"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common/buf"
)

type udpWriteStage uint8

const (
	udpWriteAdmitted udpWriteStage = iota + 1
	udpWriteMessage
	udpWriteFragment
	udpWritePack
	udpWriteSend
	udpWritePackedRelease
	udpWriteFragmentRelease
	udpWriteMessageRelease
	udpWriteCallerRelease
	udpWriteReturned
)

// Fixed in the carrier before publication. Only the admitted caller touches
// payload fields while active; the budget mutex owns admission and final reset.
// Unknown aliases may already be pooled/transferred: retain, never dereference.
type udpWriteCall struct {
	active        bool
	owner         *udpPacketConn
	charge        int
	inputBuffer   *buf.Buffer // consumed only after known operation return
	inputData     []byte      // borrowed by WriteTo, also roots consumed buffer backing
	destination   net.Addr
	address       string
	message       *udpMessage
	fragment      *udpMessage
	packed        *buf.Buffer
	fragmentCount int
	fragmentIndex int
	fragmentStart int
	stage         udpWriteStage
}

// Only inspect an inactive slot under budget.mu. An active slot's payload is
// exclusively its original caller's, including during a blocking pool callback.
func (o *udpWriteCall) idle() bool {
	return !o.active && o.owner == nil && o.charge == 0 && o.inputBuffer == nil && o.inputData == nil && o.destination == nil && o.address == "" &&
		o.message == nil && o.fragment == nil && o.packed == nil && o.fragmentCount == 0 && o.fragmentIndex == 0 && o.fragmentStart == 0 && o.stage == 0
}

type udpSendBudget struct {
	mu              sync.Mutex
	owners          int
	bytes           int
	poisoned        bool
	sealed          bool
	settled         chan struct{}
	reported        bool
	slots           [maxUDPSendOwners]udpWriteCall
	onFailure       func() // installed once before client publication
	failureNotified bool
}

func (b *udpSendBudget) markUncertain() {
	b.mu.Lock()
	b.poisoned = true
	b.settleLocked()
	var notify func()
	if !b.failureNotified {
		b.failureNotified = true
		notify = b.onFailure
	}
	b.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (b *udpSendBudget) reserveLocked(charge int) bool {
	if b.sealed || b.poisoned || charge < 0 || charge > maxUDPSendBytes-b.bytes || b.owners >= maxUDPSendOwners {
		return false
	}
	b.owners++
	b.bytes += charge
	return true
}

// Counter-only compatibility helpers. The actual public writer uses the fixed
// original slots below; a counter reservation alone is not payload custody.
func (b *udpSendBudget) reserve() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reserveLocked(udpSendCharge)
}

func (b *udpSendBudget) release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.owners <= 0 || b.bytes < udpSendCharge {
		b.poisoned = true
		b.settleLocked()
		return
	}
	b.owners--
	b.bytes -= udpSendCharge
	b.settleLocked()
}

func (b *udpSendBudget) slotsIdleLocked() bool {
	for index := range b.slots {
		if b.slots[index].active || !b.slots[index].idle() {
			return false
		}
	}
	return true
}

func (b *udpSendBudget) settleLocked() {
	if b.sealed && !b.reported && (b.poisoned || b.owners == 0 && b.bytes == 0 && b.slotsIdleLocked()) {
		b.reported = true
		close(b.settled)
	}
}

func (b *udpSendBudget) seal() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sealed {
		b.sealed = true
		b.settled = make(chan struct{})
	}
	b.settleLocked()
	return b.settled
}

func (b *udpSendBudget) retirementError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sealed || b.poisoned || b.owners != 0 || b.bytes != 0 || !b.slotsIdleLocked() {
		return errClientUncertain
	}
	return nil
}

func (c *udpPacketConn) beginOwnedWrite(buffer *buf.Buffer, data []byte, destination net.Addr, start int) (*udpWriteCall, error) {
	if len(data) > protocol.MaxUDPSize {
		return nil, &quic.DatagramTooLargeError{MaxDatagramPayloadSize: protocol.MaxUDPSize}
	}
	// Bytes exposes the remaining original backing cap. Add its start for a
	// consumed buffer without overflowing. Borrowed input is retention credit,
	// not a promise that this wrapper owns or can free the caller's allocation.
	if start < 0 || start > maxUDPSendBytes-udpSendCharge || cap(data) > maxUDPSendBytes-udpSendCharge-start {
		return nil, errUDPSendCapacity
	}
	charge := udpSendCharge + start + cap(data)
	if c.sendBudget == nil || c.sendScope == nil {
		return nil, net.ErrClosed
	}
	select {
	case <-c.ctx.Done():
		return nil, net.ErrClosed
	default:
	}
	if err := c.sendScope.Err(); err != nil {
		return nil, err
	}
	b := c.sendBudget
	b.mu.Lock()
	uncertain := false
	defer func() {
		if uncertain {
			b.poisoned = true
			b.settleLocked()
		}
		b.mu.Unlock()
		if uncertain {
			b.markUncertain()
		}
	}()
	if b.poisoned || b.owners < 0 || b.bytes < 0 {
		uncertain = true
		return nil, errClientUncertain
	}
	if b.sealed {
		return nil, net.ErrClosed
	}
	if b.owners >= maxUDPSendOwners || charge > maxUDPSendBytes-b.bytes {
		return nil, errUDPSendCapacity
	}
	for index := range b.slots {
		original := &b.slots[index]
		if original.active {
			continue
		}
		if !original.idle() {
			uncertain = true
			return nil, errClientUncertain
		}
		if !b.reserveLocked(charge) {
			return nil, errUDPSendCapacity
		}
		*original = udpWriteCall{active: true, owner: c, charge: charge, inputBuffer: buffer, inputData: data, destination: destination, stage: udpWriteAdmitted}
		return original, nil
	}
	uncertain = true
	return nil, errClientUncertain
}

func (c *udpPacketConn) endOwnedWrite(original *udpWriteCall, returned bool) (err error) {
	b := c.sendBudget
	cleaned := false
	defer func() {
		b.mu.Lock()
		owned := false
		for index := range b.slots {
			if original == &b.slots[index] {
				owned = true
				break
			}
		}
		failed := !returned || !cleaned || b.poisoned || !owned || !original.active || original.owner != c || original.charge <= 0 || original.charge > b.bytes || b.owners <= 0
		if failed {
			b.poisoned = true
			err = errClientUncertain
		} else {
			charge := original.charge
			*original = udpWriteCall{}
			b.owners--
			b.bytes -= charge
		}
		b.settleLocked()
		b.mu.Unlock()
		if failed {
			b.markUncertain() // never a life/RX callback under the send mutex
		}
	}()
	if !returned || original == nil || original.owner != c || original.packed != nil || original.fragment != nil {
		return errClientUncertain // no cleanup of an unknown Send/pack/fragment
	}
	original.stage = udpWriteMessageRelease
	if original.message != nil {
		original.message.releaseMessage()
		original.message = nil
	}
	original.stage = udpWriteCallerRelease
	if original.inputBuffer != nil {
		original.inputBuffer.Release()
		original.inputBuffer = nil
	}
	original.stage = udpWriteReturned
	cleaned = true
	return nil
}

// Preserve WritePacket consumption even when admission fails. This is the
// original caller's cleanup, without a new HY2 staging allocation or task. It
// notifies an abnormal return, but fixed admitted slots do not claim persistent
// custody/bounds for arbitrary concurrent rejected-buffer allocator callbacks.
func (c *udpPacketConn) releaseUnadmittedPacket(buffer *buf.Buffer) {
	returned := false
	defer func() {
		if !returned {
			if c.sendBudget != nil {
				c.sendBudget.markUncertain()
			} else {
				c.markShutdownUncertain()
			}
		}
	}()
	buffer.Release()
	returned = true
}
