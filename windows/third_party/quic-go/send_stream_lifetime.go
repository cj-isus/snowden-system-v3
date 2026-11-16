package quic

func (s *SendStream) beginSendCall(kind streamCallKind) (*streamCall, error) {
	if kind < sendWriteCall || kind > sendNativeCall {
		return nil, errSendStreamWriteUncertain
	}
	return s.callBudget.beginStreamCall(s, &s.callCounts, kind)
}

// Track intentional unlocks across waits/notifications. Poison the original
// stream before its mutex/write gate can be reused; no close wait under mutex.
func (s *SendStream) finishSendCall(mutexHeld, returned *bool) {
	if !*returned {
		if !*mutexHeld {
			s.mutex.Lock()
			*mutexHeld = true
		}
		s.writeUncertain = true
		if s.callBudget != nil {
			s.callBudget.markUncertain()
		}
	}
	if *mutexHeld {
		s.mutex.Unlock()
	}
}
