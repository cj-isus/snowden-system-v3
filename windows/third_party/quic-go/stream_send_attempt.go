package quic

import (
	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/wire"
)

type streamSendTerminal uint8

const (
	streamSendAcknowledged streamSendTerminal = iota + 1
	streamSendLost
	streamSendDiscarded
)

type streamSendAttempt struct {
	owner             *streamSendBudget
	loan              *sentStreamData
	terminal          streamSendTerminal // guarded by owner.access
	finishing, closed bool
}

var _ ackhandler.FrameHandler = (*streamSendAttempt)(nil)

func (original *sentStreamData) newAttempt() (*streamSendAttempt, error) {
	b := original.owner
	b.access.Lock()
	_, owned := b.live[original]
	valid := owned && !original.started && original.frame != nil && original.attempt == nil
	failed := b.failure != nil
	b.access.Unlock()
	if failed {
		return nil, errStreamSendUncertain
	}
	if !valid {
		b.markUncertain()
		return nil, errStreamSendUncertain
	}
	// The original stream mutex and native-call owner cover this opening.
	attempt := &streamSendAttempt{owner: b, loan: original}
	b.access.Lock()
	original.attempt = attempt
	b.access.Unlock()
	return attempt, nil
}

func (a *streamSendAttempt) mark(frame wire.Frame, terminal streamSendTerminal) {
	b := a.owner
	b.access.Lock()
	if a.closed {
		b.access.Unlock()
		return
	}
	valid := a.loan != nil && a.loan.attempt == a && a.loan.frame == frame &&
		!a.loan.started && !a.finishing && (a.terminal == 0 || a.terminal == terminal)
	if valid {
		a.terminal = terminal
	}
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		panic(errStreamSendUncertain)
	}
}

func (a *streamSendAttempt) OnAcked(frame wire.Frame) { a.mark(frame, streamSendAcknowledged) }
func (a *streamSendAttempt) OnLost(frame wire.Frame)  { a.mark(frame, streamSendLost) }
func (a *streamSendAttempt) OnStreamFrameDiscarded(frame wire.Frame) {
	a.mark(frame, streamSendDiscarded)
}

// Invoked only after the original history cell cleared its raw frame/handler.
// No history borrower remains when this original hands data to queue or pool.
func (a *streamSendAttempt) OnStreamFrameReleased() {
	b := a.owner
	b.access.Lock()
	if a.closed {
		b.access.Unlock()
		return
	}
	loan, terminal := a.loan, a.terminal
	valid := loan != nil && loan.attempt == a && !loan.started && !a.finishing && terminal != 0
	if valid {
		a.finishing = true
	}
	b.access.Unlock()
	if !valid {
		b.markUncertain()
		panic(errStreamSendUncertain)
	}
	s := loan.stream
	call, err := s.beginSendCall(sendNativeCall)
	if err != nil {
		b.markUncertain()
		return
	}
	returned, mutexHeld := false, false
	defer call.finish(&returned)
	defer s.finishSendCall(&mutexHeld, &returned)
	s.mutex.Lock()
	mutexHeld = true
	if s.writeUncertain {
		b.markUncertain()
		returned = true
		return
	}
	frame := loan.raw()
	resetWithoutReliable := s.resetErr != nil && s.reliableOffset() == 0
	if !resetWithoutReliable {
		s.numOutstandingFrames--
		if s.numOutstandingFrames < 0 {
			panic("numOutStandingFrames negative")
		}
	}
	retransmit := terminal == streamSendLost && s.shutdownErr == nil && !resetWithoutReliable
	if retransmit && s.resetErr != nil {
		reliable := s.reliableOffset()
		if frame.Offset >= reliable {
			retransmit = false
		} else if frame.Offset+frame.DataLen() > reliable {
			frame.Data = frame.Data[:reliable-frame.Offset]
		}
	}
	// Detach this immutable attempt before explicit queue/pool handoff. A late
	// repeated terminal callback sees closed, never a reused pooled frame.
	b.access.Lock()
	loan.attempt, a.loan, a.closed = nil, nil, true
	b.access.Unlock()
	if retransmit {
		frame.DataLenPresent = true
		s.retransmissionQueue = append(s.retransmissionQueue, loan)
		s.mutex.Unlock()
		mutexHeld = false
		s.sender.onHasStreamData(s.streamID, s)
		returned = true
		return
	}
	completed := terminal != streamSendDiscarded && s.shutdownErr == nil && !resetWithoutReliable && s.isNewlyCompleted()
	s.mutex.Unlock()
	mutexHeld = false
	if completed {
		s.sender.onStreamCompleted(s.streamID)
	}
	loan.release()
	returned = true
}
