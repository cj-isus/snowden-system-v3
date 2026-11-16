package ackhandler

import "github.com/sagernet/quic-go/internal/wire"

// An owned STREAM attempt receives this after its terminal callback returned
// and the original history cell dropped its raw frame/handler aliases.
type streamFrameReleaseHandler interface{ OnStreamFrameReleased() }
type streamFrameDiscardHandler interface{ OnStreamFrameDiscarded(wire.Frame) }

func (f *StreamFrame) Acked() { f.finish(false) }
func (f *StreamFrame) Lost()  { f.finish(true) }

// Key-space invalidation is not an ACK or a retransmission request. Existing
// legacy handlers keep their old discard behavior; owned data has an explicit hook.
func (f *StreamFrame) Discarded() {
	original := *f
	if handler, ok := original.Handler.(streamFrameDiscardHandler); ok {
		handler.OnStreamFrameDiscarded(original.Frame)
	}
	f.Frame, f.Handler = nil, nil
	if handler, ok := original.Handler.(streamFrameReleaseHandler); ok {
		handler.OnStreamFrameReleased()
	}
}

func (f *StreamFrame) finish(lost bool) {
	original := *f
	if original.Handler != nil {
		if lost {
			original.Handler.OnLost(original.Frame)
		} else {
			original.Handler.OnAcked(original.Frame)
		}
	}
	// Preserve the containing slice's length for packet ack-eliciting accounting.
	f.Frame, f.Handler = nil, nil
	if handler, ok := original.Handler.(streamFrameReleaseHandler); ok {
		handler.OnStreamFrameReleased()
	}
}
