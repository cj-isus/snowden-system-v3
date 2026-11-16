package quic

import "errors"

// Fixed pointer set. The original reset record retains both sets, including a
// partially built candidate; publication never overwrites its original owner.
type streamMapSet struct {
	generation   *streamGeneration
	outgoingBidi *outgoingStreamsMap[*Stream]
	outgoingUni  *outgoingStreamsMap[*SendStream]
	incomingBidi *incomingStreamsMap[*Stream]
	incomingUni  *incomingStreamsMap[*ReceiveStream]
}

func (s *streamMapSet) valid() bool {
	return s != nil && s.generation != nil && s.outgoingBidi != nil && s.outgoingUni != nil &&
		s.incomingBidi != nil && s.incomingUni != nil
}

func (s *streamMapSet) closeWithError(cause error) error {
	if !s.valid() {
		return errStreamMapUncertain
	}
	outgoingBidiErr := s.outgoingBidi.CloseWithError(cause)
	outgoingUniErr := s.outgoingUni.CloseWithError(cause)
	incomingBidiErr := s.incomingBidi.CloseWithError(cause)
	incomingUniErr := s.incomingUni.CloseWithError(cause)
	return errors.Join(outgoingBidiErr, outgoingUniErr, incomingBidiErr, incomingUniErr)
}

type streamMapResetStage uint8

const (
	resetClosing streamMapResetStage = iota + 1
	resetFlow
	resetBuilding
	resetFramer
	resetPublishing
	resetPublished
)

type streamMapReset struct {
	// Shared result fields use the parent mutex. Only the original Conn loop
	// touches stage/candidate or invokes the retained native flow reset.
	started, ended, completed, uncertain bool
	failure                              error
	stage                                streamMapResetStage
	original, candidate                  streamMapSet
	flowReset                            func() error
}

// Caller owns the parent mutex, or the parent has not been exposed yet.
func (m *streamsMap) currentMapSet() streamMapSet {
	return streamMapSet{
		generation: m.generation, outgoingBidi: m.outgoingBidiStreams,
		outgoingUni: m.outgoingUniStreams, incomingBidi: m.incomingBidiStreams,
		incomingUni: m.incomingUniStreams,
	}
}

func (m *streamsMap) publishMapSet(s *streamMapSet) {
	m.generation = s.generation
	m.outgoingBidiStreams, m.outgoingUniStreams = s.outgoingBidi, s.outgoingUni
	m.incomingBidiStreams, m.incomingUniStreams = s.incomingBidi, s.incomingUni
}

// The first-close pointer is immutable after native Conn setup. This does not
// take the call registry mutex or invoke a foreign error/context method.
func (m *streamsMap) resetCloseError() error {
	if m.calls.closeState == nil {
		return errStreamMapUncertain
	}
	if closed := m.calls.closeState.Load(); closed != nil {
		if closed.err != nil {
			return closed.err
		}
		return errStreamMapUncertain
	}
	return nil
}

// Only the original Conn loop executes this one attempt. Parent admission is
// fenced first; no parent metadata lock spans native close, flow reset, factory
// allocation or framer locks. A later failure never rolls back native counters.
func (m *streamsMap) ResetFor0RTT(flowReset func() error) (result error) {
	m.mutex.Lock()
	if err := m.resetCloseError(); err != nil {
		m.mutex.Unlock()
		return err
	}
	r := &m.resetState
	if r.started {
		ended, completed, uncertain, failure := r.ended, r.completed, r.uncertain, r.failure
		m.mutex.Unlock()
		if !ended || uncertain || !completed && failure == nil {
			return errors.Join(errStreamMapUncertain, failure)
		}
		return failure // known success or exact ordinary failure; never retry
	}
	m.reset = true
	*r = streamMapReset{
		started: true, stage: resetClosing, original: m.currentMapSet(), flowReset: flowReset,
	}
	m.mutex.Unlock()
	returned, uncertain := false, false
	defer m.finishReset(&returned, &uncertain, &result)
	if !r.original.valid() || r.flowReset == nil {
		returned, uncertain = true, true
		return errStreamMapUncertain
	}
	if err := r.original.closeWithError(Err0RTTRejected); err != nil {
		returned = true
		return err
	}
	r.stage = resetFlow
	if err := m.resetCloseError(); err != nil {
		returned = true
		return err
	}
	if err := r.flowReset(); err != nil {
		returned = true
		return err // no candidate or new framer token exists at this point
	}
	if err := m.resetCloseError(); err != nil {
		returned = true
		return err
	}
	r.stage = resetBuilding
	m.buildMaps(&r.candidate)
	if !r.candidate.valid() {
		returned, uncertain = true, true
		return errStreamMapUncertain
	}
	if err := m.resetCloseError(); err != nil {
		returned = true
		return err
	}
	r.stage = resetFramer
	if err := m.framer.Handle0RTTRejection(r.original.generation, r.candidate.generation); err != nil {
		returned, uncertain = true, true
		return err
	}
	r.stage = resetPublishing
	m.mutex.Lock()
	err := m.resetCloseError()
	if err == nil && (!m.reset || m.generation != r.original.generation) {
		err, uncertain = errStreamMapUncertain, true
	}
	if err == nil {
		m.publishMapSet(&r.candidate)
		r.stage = resetPublished
	}
	m.mutex.Unlock()
	returned = true
	return err
}

func (m *streamsMap) finishReset(returned, uncertain *bool, result *error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	r := &m.resetState
	r.ended = true
	r.uncertain = !*returned || *uncertain
	if r.uncertain {
		*result = errors.Join(errStreamMapUncertain, *result)
	}
	r.completed = *result == nil && !r.uncertain && r.stage == resetPublished &&
		m.generation == r.candidate.generation
	if !r.completed {
		m.reset = true
		if *result == nil {
			r.uncertain = true
			*result = errStreamMapUncertain
		}
	}
	r.failure = *result
}

func (m *streamsMap) UseResetMaps() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.resetCloseError(); err != nil {
		return err
	}
	if m.reset && (!m.resetState.completed || !m.resetState.ended || m.resetState.failure != nil) {
		return errors.Join(errStreamMapUncertain, m.resetState.failure)
	}
	m.reset = false
	return nil
}
