package hysteria2

import (
	"errors"
	"io"
	"os"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func (c *udpPacketConn) InitializeReadWaiter(options N.ReadWaitOptions) (needCopy bool) {
	if c.defragger != nil && c.defragger.budget != nil {
		b := c.defragger.budget
		b.mu.Lock()
		if !b.poisoned && !b.sealed && !c.readSealed {
			c.readWaitOptions = options
		}
		b.mu.Unlock()
	}
	return options.NeedHeadroom()
}

func (c *udpPacketConn) WaitReadPacket() (buffer *buf.Buffer, destination M.Socksaddr, err error) {
	original, err := c.beginRead(nil, nil)
	if err != nil {
		return nil, M.Socksaddr{}, err
	}
	returned := false
	defer func() {
		if readErr := c.endRead(original, returned); readErr != nil {
			buffer = nil // ambiguous output stays in the original retained slot
			err = errors.Join(err, readErr)
		}
	}()
	options := original.options
	size, charge, err := udpReadWaitPlan(options)
	if err != nil {
		returned = true
		return nil, M.Socksaddr{}, err
	}
	select {
	case original.message = <-c.data:
		p := original.message
		destination = M.ParseSocksaddr(p.destination).Unwrap()
		if options.NeedHeadroom() {
			length := p.data.Len()
			if length > size-options.FrontHeadroom-options.RearHeadroom {
				p.releaseMessage()
				original.message = nil
				returned = true
				return nil, destination, io.ErrShortBuffer
			}
			b := c.defragger.budget
			if !b.reserve(charge) {
				p.releaseMessage()
				original.message = nil
				returned = true
				return nil, destination, errUDPReceiveCapacity
			}
			original.outputCharge = charge // before allocator/resize/reserve
			original.output = options.NewPacketBuffer()
			n, copyErr := original.output.Write(p.data.Bytes())
			if copyErr == nil && n != length {
				copyErr = io.ErrShortBuffer
			}
			p.releaseMessage()
			original.message = nil
			if copyErr != nil {
				original.output.Release()
				original.output = nil
				b.release(original.outputCharge)
				original.outputCharge = 0
				returned = true
				return nil, destination, copyErr
			}
			options.PostReturn(original.output)
			// Establish the caller-owned return value before ending our credit.
			buffer = original.output
			b.release(original.outputCharge)
			original.outputCharge = 0
		} else {
			original.output = p.data // retained across message pool return
			p.release()
			original.message = nil
			buffer = original.output
		}
		returned = true
		return
	case <-c.ctx.Done():
		returned = true
		return nil, M.Socksaddr{}, io.ErrClosedPipe
	case <-c.readDeadline.Wait():
		returned = true
		return nil, M.Socksaddr{}, os.ErrDeadlineExceeded
	}
}
