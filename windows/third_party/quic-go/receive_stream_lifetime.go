package quic

import "errors"

var errReceiveStreamUncertain = errors.New("quic: original receive operation or frame release did not return normally")

// Original native calls retain their mutex state across intentional read/peek
// waits and sender notifications. Seal abnormal state before unlocking so an
// admitted waiter cannot reuse a frame whose callback did not return normally.
func (s *ReceiveStream) finishReceiveCall(mutexHeld, returned *bool) {
	if !*returned {
		if !*mutexHeld {
			s.mutex.Lock()
			*mutexHeld = true
		}
		s.receiveUncertain = true
		if s.frameIntake.budget != nil {
			s.frameIntake.budget.markUncertain()
		}
		if s.frameIntake.original != nil || s.frameIntake.releasing {
			s.frameIntake.failed = true
		}
	}
	if *mutexHeld {
		s.mutex.Unlock()
	}
}

// Caller holds the original receive mutex. A negative result is sticky and
// preserves the original current/queued/rejected frame references.
func (s *ReceiveStream) retirementError() error {
	if s.receiveUncertain || s.currentFrameReleasing || s.frameIntake.failed || s.frameIntake.releasing || s.frameQueue.retirementUncertain {
		return errReceiveStreamUncertain
	}
	return nil
}

func (s *ReceiveStream) releaseCurrentFrame() error {
	if err := s.retirementError(); err != nil {
		return err
	}
	s.currentFrameReleasing = true
	if s.currentFrameDone != nil {
		s.currentFrameDone()
	}
	s.currentFrame, s.currentFrameDone = nil, nil
	s.readPosInFrame = 0
	s.currentFrameReleasing = false
	return nil
}

func (s *ReceiveStream) discardBufferedFrames() error {
	if err := s.releaseCurrentFrame(); err != nil {
		return err
	}
	if err := s.frameQueue.Discard(); err != nil {
		s.receiveUncertain = true
		return err
	}
	return nil
}
