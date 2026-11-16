package quic

import (
	"net"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

func (b *streamReceiveBudget) beginParser() error {
	b.access.Lock()
	if b.parserActive || b.pending != nil {
		b.access.Unlock()
		b.markUncertain()
		return errStreamReceiveUncertain
	}
	if b.sealed {
		err := b.failure
		b.access.Unlock()
		if err != nil {
			return err
		}
		return net.ErrClosed
	}
	b.parserActive = true
	b.access.Unlock()
	return nil
}

func (b *streamReceiveBudget) finishParser(returned *bool) {
	defer func() {
		b.access.Lock()
		b.parserActive = false
		b.settleLocked()
		b.access.Unlock()
	}()
	if !*returned {
		b.markUncertain()
		return
	}
	b.access.Lock()
	pending, uncertain := b.pending, b.failure != nil
	b.access.Unlock()
	if pending != nil && !uncertain {
		pending.release()
		b.access.Lock()
		b.pending = nil
		b.access.Unlock()
	}
}

func (b *streamReceiveBudget) allocateParserFrame(size int) (*wire.StreamFrame, error) {
	if size < 0 || size > protocol.MaxPacketBufferSize {
		return nil, &qerr.TransportError{ErrorCode: qerr.InternalError, ErrorMessage: "invalid STREAM allocation size"}
	}
	b.access.Lock()
	valid := b.parserActive && b.pending == nil
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		return nil, &qerr.TransportError{ErrorCode: qerr.InternalError, ErrorMessage: errStreamReceiveUncertain.Error()}
	}
	charge := size
	if size >= protocol.MinStreamFrameBufferSize {
		charge = protocol.MaxPacketBufferSize
	}
	original, err := b.acquire(charge)
	if err != nil {
		return nil, &qerr.TransportError{ErrorCode: qerr.InternalError, ErrorMessage: err.Error()}
	}
	b.access.Lock()
	b.pending = original
	b.access.Unlock()
	var frame *wire.StreamFrame
	if size >= protocol.MinStreamFrameBufferSize {
		frame = wire.GetStreamFrame()
	} else {
		frame = &wire.StreamFrame{}
	}
	b.access.Lock()
	original.frame = frame
	b.access.Unlock()
	if size >= protocol.MinStreamFrameBufferSize {
		frame.Data = frame.Data[:size]
	} else if size != 0 {
		frame.Data = make([]byte, size)
	}
	return frame, nil
}

func (b *streamReceiveBudget) takeParsed(frame *wire.StreamFrame) (*receivedStreamData, error) {
	b.access.Lock()
	original := b.pending
	valid := original != nil && original.frame == frame && !original.started
	if valid {
		b.pending = nil
	}
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		return nil, errStreamReceiveUncertain
	}
	return original, nil
}

// Admission spans header parse, first allocation/copy, tracing, all handoffs and
// the original handler return. A partial allocator result stays in pending.
func (c *Conn) consumeStreamFrame(typ wire.FrameType, data []byte, handle func(*receivedStreamData) error) (int, bool, error) {
	b := c.receiveFrames
	if b == nil {
		return 0, false, errStreamReceiveUncertain
	}
	if err := b.beginParser(); err != nil {
		return 0, false, err
	}
	returned := false
	defer b.finishParser(&returned)
	frame, n, err := c.frameParser.ParseStreamFrameWithAllocator(typ, data, c.version, b.allocateParserFrame)
	if err != nil {
		returned = true
		return n, false, err
	}
	original, err := b.takeParsed(frame)
	if err != nil {
		returned = true
		return n, false, err
	}
	err = handle(original)
	returned = true
	return n, true, err
}
