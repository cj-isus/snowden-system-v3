package quic

import (
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/protocol"
)

// Nonzero immutable object identity, not a StreamID or a reusable counter. The
// original Conn/framer bindings precede exposure; old senders retain this token.
type streamGeneration struct {
	framer  *framer
	sender  streamSender
	onError func(error)
	// Original generation identity is immutable; these heads publish separate
	// incoming-received and outgoing-applied immutable scalar snapshots.
	incomingParameters atomic.Pointer[streamParameters]
	outgoingParameters atomic.Pointer[streamParameters]
}

// One sender per original child, shared by that child's streams. Completion
// invokes its exact Delete method, without looking up the current parent maps.
type streamMapSender struct {
	generation *streamGeneration
	completed  func(protocol.StreamID) error
}

var _ streamSender = (*streamMapSender)(nil)

func (s *streamMapSender) onHasConnectionData() {
	g := s.generation
	if g.framer.streamGeneration.Load() == g {
		// This carries no stream identity or borrowed data. A racing late nudge
		// can only schedule the existing loop; it is not flow-epoch authority.
		g.sender.onHasConnectionData()
	}
}

func (s *streamMapSender) onHasStreamData(id protocol.StreamID, str *SendStream) {
	g := s.generation
	if g.framer.addActiveStream(g, id, str) {
		g.sender.onHasConnectionData()
	}
}

func (s *streamMapSender) onHasStreamControlFrame(id protocol.StreamID, str streamControlFrameGetter) {
	g := s.generation
	if g.framer.addStreamWithControlFrames(g, id, str) {
		g.sender.onHasConnectionData()
	}
}

func (s *streamMapSender) onStreamCompleted(id protocol.StreamID) {
	g := s.generation
	if err := s.completed(id); err != nil {
		g.onError(err) // original close notification, outside child/framer locks
	}
	g.framer.removeActiveStream(g, id)
}
