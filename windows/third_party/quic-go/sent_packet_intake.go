package quic

import (
	"errors"
	"io"
	"net"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
)

// This packer emits at most one DATAGRAM per packet. GSO appends another packet
// only after a full-sized segment (at least MinInitialPacketSize); allow one tail.
const maxPacketOutputDatagrams = protocol.MaxLargePacketBufferSize/protocol.MinInitialPacketSize + 1

var (
	errPacketOutputUncertain = errors.New("quic: original packet output ownership is uncertain")
	errPacketOutputCapacity  = errors.New("quic: packet output datagram capacity exceeded")
)

// One original Conn loop owns this pre-handoff slot. Queued/raw-provider output
// keeps its existing owner; a failed transfer is never released as known-unsent.
type packetSendBatch struct {
	original  *packetBuffer
	datagrams [maxPacketOutputDatagrams]*datagramSendEntry
	count     int
}

type packetSendIntake struct {
	packetSendBatch
	refused                                    *packetBuffer
	failed, releasing, transferring, borrowing bool
}

func (c *Conn) beginPacketOutput(buf *packetBuffer) error {
	s := &c.packetOutput
	if buf == nil || s.failed || s.releasing || s.transferring || s.borrowing || s.original != nil {
		if s.refused == nil && buf != s.original {
			s.refused = buf
		}
		s.failed = true
		c.receiveFrames.markUncertain()
		return errPacketOutputUncertain
	}
	s.original = buf
	return nil
}

func (c *Conn) recordPacketDatagrams(frames []ackhandler.Frame) error {
	s := &c.packetOutput
	if s.original == nil || s.failed || s.releasing || s.transferring || s.borrowing {
		c.receiveFrames.markUncertain()
		return errPacketOutputUncertain
	}
	for _, frame := range frames {
		entry, ok := frame.Handler.(*datagramSendEntry)
		if !ok {
			continue
		}
		duplicate := false
		for _, previous := range s.datagrams[:s.count] {
			if previous == entry {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		if s.count == len(s.datagrams) {
			// Fresh unrecorded inputs have not entered a raw provider. The earlier
			// batch remains owned and is retired by the ordinary error caller.
			retireUnsentDatagrams(frames)
			return errPacketOutputCapacity
		}
		s.datagrams[s.count] = entry
		s.count++
	}
	return nil
}

func (c *Conn) releasePacketOutput() {
	s := &c.packetOutput
	if s.original == nil || s.failed || s.releasing || s.transferring || s.borrowing {
		c.receiveFrames.markUncertain()
		panic(errPacketOutputUncertain)
	}
	s.releasing = true
	for i := 0; i < s.count; i++ {
		entry := s.datagrams[i]
		entry.retireHistory(&entry.frame)
		s.datagrams[i] = nil
	}
	s.count = 0
	s.original.Release()
	s.original = nil
	s.releasing = false
}

func (c *Conn) rejectPacketOutput(err error) error {
	if errors.Is(err, errPacketOutputUncertain) {
		// A partial packer can still borrow this exact GSO buffer. Its negative
		// result is not permission to return either original to the pool.
		c.packetOutput.failed = true
		c.receiveFrames.markUncertain()
		return err
	}
	if errors.Is(err, ackhandler.ErrSentHistoryUncertain) {
		c.receiveFrames.markUncertain()
	}
	c.releasePacketOutput()
	return err
}

func (c *Conn) clearTransferredPacketOutput() {
	s := &c.packetOutput
	s.original = nil // no raw buffer read after Send may have returned it to pool
	clear(s.datagrams[:s.count])
	s.count = 0
	s.transferring = false
}

func (c *Conn) beginPacketTransfer() (packetSendBatch, error) {
	s := &c.packetOutput
	if s.original == nil || s.failed || s.releasing || s.transferring || s.borrowing {
		c.receiveFrames.markUncertain()
		s.failed = true
		return packetSendBatch{}, errPacketOutputUncertain
	}
	s.transferring = true
	return s.packetSendBatch, nil
}

func (c *Conn) finishPacketTransfer(accepted bool, err error) error {
	if accepted {
		c.clearTransferredPacketOutput()
		if errors.Is(err, errPacketOutputUncertain) {
			c.receiveFrames.markUncertain()
		}
		return err
	}
	c.packetOutput.transferring = false
	if err == nil {
		err = errPacketOutputUncertain
	}
	return c.rejectPacketOutput(err)
}

func (c *Conn) sendPacketOutput(gso uint16, ecn protocol.ECN) error {
	batch, err := c.beginPacketTransfer()
	if err != nil {
		return err
	}
	accepted, err := c.sendQueue.Send(batch, gso, ecn)
	return c.finishPacketTransfer(accepted, err)
}

func (c *Conn) sendProbePacketOutput(addr net.Addr) error {
	batch, err := c.beginPacketTransfer()
	if err != nil {
		return err
	}
	accepted, err := c.sendQueue.SendProbe(batch, addr)
	return c.finishPacketTransfer(accepted, err)
}

// Alternate Transport.WriteTo borrows Data; it does not adopt packetBuffer.
// Keep the original before tracing, registration and the actual provider call.
func (c *Conn) sendOwnedPathProbe(probe shortHeaderPacket, buf *packetBuffer, tr *Transport, now monotime.Time) error {
	return c.sendOwnedPathPacket(probe, buf, tr, now, nil)
}

func (c *Conn) sendOwnedPathPacket(probe shortHeaderPacket, buf *packetBuffer, tr *Transport, now monotime.Time, response *queuedPathResponse) error {
	if err := c.beginPacketOutput(buf); err != nil {
		return err
	}
	if err := c.recordPacketDatagrams(probe.Frames); err != nil {
		return c.rejectPacketOutput(err)
	}
	c.logger.Debugf("sending path validation packet")
	c.logShortHeaderPacket(probe, protocol.ECNNon, buf.Len())
	if err := c.registerPackedShortHeaderPacket(probe, protocol.ECNNon, now); err != nil {
		return c.rejectPacketOutput(err)
	}
	wanted := len(buf.Data)
	c.packetOutput.borrowing = true
	var n int
	var err error
	if response == nil {
		n, err = tr.WriteTo(buf.Data, c.conn.RemoteAddr())
	} else {
		n, err = tr.writePacketTo(buf.Data, response.remote, response.info)
	}
	c.packetOutput.borrowing = false
	if n < 0 || n > wanted {
		c.packetOutput.failed = true
		c.receiveFrames.markUncertain()
		return errPacketOutputUncertain
	}
	if err == nil && n != wanted {
		err = io.ErrShortWrite
	}
	c.releasePacketOutput()
	return err
}
