package hysteria2

import (
	"errors"
	"io"
	"net"
	"os"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	maxUDPReadCalls        = 512
	maxUDPReadSessionCalls = 8
	maxUDPReadBufferSize   = 65535
)

var (
	errUDPReadCapacity = errors.New("HY2 UDP read capacity exceeded")
	errUDPReadOptions  = errors.New("invalid or oversized HY2 UDP read options")
)

// Eight fixed slots per session, admitted by the carrier's existing RX mutex.
// Only its admitted caller accesses payload fields. The budget's readers map
// roots this session until all known reads return, including after public Close
// removes the session from the active map. An unknown call keeps its slot/root.
type udpReadCall struct {
	active       bool
	message      *udpMessage
	callerBuffer *buf.Buffer // borrowed; never released by this owner
	callerBytes  []byte      // borrowed; its capacity is not HY2-owned storage
	output       *buf.Buffer // allocated wait output or zero-copy return value
	outputCharge int
	options      N.ReadWaitOptions
}

func (c *udpPacketConn) beginRead(buffer *buf.Buffer, data []byte) (*udpReadCall, error) {
	if c.defragger == nil || c.defragger.budget == nil {
		return nil, errClientUncertain
	}
	b := c.defragger.budget
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
	if b.poisoned {
		uncertain = true
		return nil, errClientUncertain
	}
	_, rooted := b.readers[c]
	if c.readCalls < 0 || c.readCalls > maxUDPReadSessionCalls || b.readCalls < 0 || b.readCalls > maxUDPReadCalls || rooted != (c.readCalls != 0) {
		uncertain = true
		return nil, errClientUncertain
	}
	if b.sealed || c.readSealed {
		return nil, io.ErrClosedPipe
	}
	if b.readCalls >= maxUDPReadCalls || c.readCalls >= maxUDPReadSessionCalls {
		return nil, errUDPReadCapacity
	}
	for index := range c.readSlots {
		original := &c.readSlots[index]
		if original.active {
			continue
		}
		if original.message != nil || original.callerBuffer != nil || original.callerBytes != nil || original.output != nil || original.outputCharge != 0 || original.options != (N.ReadWaitOptions{}) {
			uncertain = true
			return nil, errClientUncertain
		}
		if b.readers == nil {
			b.readers = make(map[*udpPacketConn]struct{})
		}
		b.readers[c] = struct{}{}
		*original = udpReadCall{active: true, callerBuffer: buffer, callerBytes: data, options: c.readWaitOptions}
		c.readCalls++
		b.readCalls++
		return original, nil
	}
	// Counts and fixed-slot occupancy disagree. Do not guess a free owner or
	// call a parent while holding the budget mutex.
	uncertain = true
	return nil, errClientUncertain
}

func (c *udpPacketConn) endRead(original *udpReadCall, returned bool) error {
	b := c.defragger.budget
	b.mu.Lock()
	_, rooted := b.readers[c]
	ownedSlot := false
	for index := range c.readSlots {
		if original == &c.readSlots[index] {
			ownedSlot = true
			break
		}
	}
	failed := !returned || b.poisoned || !ownedSlot || !original.active || original.message != nil || original.outputCharge != 0 || !rooted || c.readCalls <= 0 || b.readCalls <= 0
	if failed {
		b.poisoned = true
	} else {
		// No pool/buffer/caller operation follows this original call's defer.
		// Return values are already established; only remove our own aliases.
		*original = udpReadCall{}
		c.readCalls--
		b.readCalls--
		if c.readCalls == 0 {
			delete(b.readers, c)
		}
	}
	b.settleLocked()
	b.mu.Unlock()
	if failed {
		b.markUncertain() // starts parent abort outside budget/RX locks, no join
		return errClientUncertain
	}
	return nil
}

func (c *udpPacketConn) sealReadCalls() error {
	if c.defragger == nil || c.defragger.budget == nil {
		return errClientUncertain
	}
	b := c.defragger.budget
	b.mu.Lock()
	c.readSealed = true
	failed := b.poisoned
	b.mu.Unlock()
	if failed {
		return errClientUncertain
	}
	return nil // seal only; the whole-carrier budget joins original readers
}

// Match the pinned ReadWaitOptions.NewPacketBuffer allocation before calling it.
// Default pool backing rounds to 64..65536 bytes. Custom allocators, caller
// outputs and pool/RSS retention remain separate from this engineering charge.
func udpReadWaitPlan(options N.ReadWaitOptions) (size, charge int, err error) {
	if options.FrontHeadroom < 0 || options.RearHeadroom < 0 || options.MTU < 0 || options.ReadOverhead < 0 || options.BatchSize < 0 ||
		options.FrontHeadroom > maxUDPReadBufferSize || options.RearHeadroom > maxUDPReadBufferSize || options.MTU > maxUDPReadBufferSize || options.ReadOverhead > maxUDPReadBufferSize {
		return 0, 0, errUDPReadOptions
	}
	switch {
	case options.IncreaseBuffer:
		size = maxUDPReadBufferSize
	case options.MTU > 0:
		size = options.MTU
		for _, extra := range [...]int{options.ReadOverhead, options.FrontHeadroom, options.RearHeadroom} {
			if extra > maxUDPReadBufferSize-size {
				return 0, 0, errUDPReadOptions
			}
			size += extra
		}
	default:
		size = buf.UDPBufferSize
	}
	if size <= 0 || size > maxUDPReadBufferSize || options.FrontHeadroom > size || options.RearHeadroom > size-options.FrontHeadroom {
		return 0, 0, errUDPReadOptions
	}
	charge = 64
	for charge < size {
		charge <<= 1
	}
	return size, charge, nil
}

func (c *udpPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	original, err := c.beginRead(buffer, nil)
	if err != nil {
		return M.Socksaddr{}, err
	}
	returned := false
	defer func() {
		if readErr := c.endRead(original, returned); readErr != nil {
			err = errors.Join(err, readErr)
		}
	}()
	if buffer == nil {
		returned = true
		return M.Socksaddr{}, io.ErrShortBuffer
	}
	select {
	case original.message = <-c.data:
		p := original.message
		_, err = buffer.ReadOnceFrom(p.data)
		destination = M.ParseSocksaddr(p.destination).Unwrap()
		p.releaseMessage()
		original.message = nil
		returned = true
		return
	case <-c.ctx.Done():
		returned = true
		return M.Socksaddr{}, io.ErrClosedPipe
	case <-c.readDeadline.Wait():
		returned = true
		return M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}

func (c *udpPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	original, err := c.beginRead(nil, p)
	if err != nil {
		return 0, nil, err
	}
	returned := false
	defer func() {
		if readErr := c.endRead(original, returned); readErr != nil {
			err = errors.Join(err, readErr)
		}
	}()
	select {
	case original.message = <-c.data:
		pkt := original.message
		n = copy(p, pkt.data.Bytes())
		destination := M.ParseSocksaddr(pkt.destination).Unwrap()
		if destination.IsFqdn() {
			addr = destination
		} else {
			addr = destination.UDPAddr()
		}
		pkt.releaseMessage()
		original.message = nil
		returned = true
		return n, addr, nil
	case <-c.ctx.Done():
		returned = true
		return 0, nil, io.ErrClosedPipe
	case <-c.readDeadline.Wait():
		returned = true
		return 0, nil, os.ErrDeadlineExceeded
	}
}
