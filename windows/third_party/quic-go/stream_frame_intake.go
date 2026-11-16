package quic

import (
	"errors"
)

var errStreamFrameIntake = errors.New("quic: original STREAM frame intake is uncertain")

// One inline owner follows the original serialized frame dispatcher at each
// handoff. ReceiveStream uses its receive mutex. An abnormal callback retains
// its original object and seals reuse; transfer clears only the former owner.
type streamFrameIntake struct {
	original  *receivedStreamData
	budget    *streamReceiveBudget
	releasing bool
	failed    bool
}

func (s *streamFrameIntake) available() bool {
	return !s.failed && !s.releasing && s.original == nil
}

func (s *streamFrameIntake) begin(frame *receivedStreamData) error {
	if !s.available() || s.budget == nil || !s.budget.ownsFrame(frame) {
		if s.budget != nil {
			s.budget.markUncertain()
		}
		return errStreamFrameIntake
	}
	s.original = frame
	return nil
}

func (s *streamFrameIntake) transfer() *receivedStreamData {
	if s.failed || s.releasing || s.original == nil {
		panic(errStreamFrameIntake)
	}
	original := s.original
	s.original = nil
	return original
}

func (s *streamFrameIntake) release() {
	if s.original == nil {
		return
	}
	if s.failed || s.releasing {
		panic(errStreamFrameIntake)
	}
	s.releasing = true
	s.original.release()
	s.original = nil
	s.releasing = false
}

// Invoke as the original call's defer. A normal untransferred input is consumed
// before success/error returns. Body or cleanup panic/Goexit preserves the slot.
func (s *streamFrameIntake) finish(returned *bool) {
	cleaned := false
	defer func() {
		if !*returned || !cleaned {
			s.failed = true
			if s.budget != nil {
				s.budget.markUncertain()
			}
		}
	}()
	if *returned {
		s.release()
	}
	cleaned = true
}
