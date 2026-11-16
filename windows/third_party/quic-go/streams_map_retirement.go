package quic

import "errors"

var errStreamMapRetirement = errors.New("quic: original stream map metadata retirement is uncertain")

type streamMapRetirementStage uint8

const (
	mapRetirementJoining streamMapRetirementStage = iota + 1
	mapRetirementSnapshot
	mapRetirementUnusedCandidate
	mapRetirementPreflight
	mapRetirementClearing
	mapRetirementParent
)

type streamMapRetirement struct {
	// Shared result metadata uses the parent mutex. Only its one external
	// finalizer accesses the fixed pointer prefix, stage and index until done.
	started, ended bool
	done           chan struct{}
	result         error
	sets           [3]streamMapSet
	stage          streamMapRetirementStage
	index          int
}

func (r *streamMapRetirement) distinct(index int) bool {
	if r.sets[index] == (streamMapSet{}) {
		return false
	}
	for i := 0; i < index; i++ {
		if r.sets[index] == r.sets[i] {
			return false
		}
	}
	return true
}

func (s *streamMapSet) unusedEmpty() bool {
	return s.valid() && s.outgoingBidi.unusedEmpty() && s.outgoingUni.unusedEmpty() &&
		s.incomingBidi.unusedEmpty() && s.incomingUni.unusedEmpty()
}

func (s *streamMapSet) metadataReady() error {
	if !s.valid() {
		return errStreamMapRetirement
	}
	return errors.Join(s.outgoingBidi.metadataReady(), s.outgoingUni.metadataReady(),
		s.incomingBidi.metadataReady(), s.incomingUni.metadataReady())
}

func (s *streamMapSet) retireMetadata() error {
	if err := s.outgoingBidi.retireMetadata(); err != nil {
		return err
	}
	if err := s.outgoingUni.retireMetadata(); err != nil {
		return err
	}
	if err := s.incomingBidi.retireMetadata(); err != nil {
		return err
	}
	return s.incomingUni.retireMetadata()
}

// Caller holds the parent mutex, after all original producer/call receipts.
func (m *streamsMap) captureRetirementSets() error {
	if m.calls.closeState == nil || m.calls.closeState.Load() == nil {
		return errStreamMapRetirement
	}
	r := &m.retirement
	r.sets[0] = m.currentMapSet()
	if !r.sets[0].valid() {
		return errStreamMapRetirement
	}
	reset := &m.resetState
	if !reset.started {
		return nil
	}
	if !reset.ended || reset.uncertain || !reset.original.valid() ||
		reset.completed && r.sets[0] != reset.candidate ||
		!reset.completed && (reset.failure == nil || r.sets[0] != reset.original) ||
		reset.candidate != (streamMapSet{}) && !reset.candidate.valid() {
		return errStreamMapRetirement
	}
	r.sets[1], r.sets[2] = reset.original, reset.candidate
	return nil
}

// WaitForStreamMapRetirement is an external-owner aggregate. It joins the
// original packet input, Open/Accept, receive data/calls, and send data/all-call/
// framer receipts before dropping current/0-RTT map metadata references.
// Never call it from a registered/native operation's own callback. It does not
// retire externally held streams/contexts or all raw/ACK/crypto/OS/GC resources.
func (c *Conn) WaitForStreamMapRetirement() (result error) {
	<-c.ctx.Done()
	m := c.streamsMap
	if m == nil {
		return errStreamMapRetirement
	}
	m.mutex.Lock()
	r := &m.retirement
	if r.done == nil {
		m.mutex.Unlock()
		return errStreamMapRetirement
	}
	if r.started {
		done := r.done
		m.mutex.Unlock()
		<-done // join the original finalizer outside metadata locks
		m.mutex.Lock()
		result = r.result
		if !r.ended {
			result = errors.Join(errStreamMapRetirement, result)
		}
		m.mutex.Unlock()
		return result
	}
	r.started, r.stage = true, mapRetirementJoining
	m.mutex.Unlock()
	returned := false
	defer func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		if !returned {
			result = errors.Join(errStreamMapRetirement, result)
		}
		r.result, r.ended = result, true
		if returned && result == nil {
			r.sets = [3]streamMapSet{}
		}
		close(r.done)
	}()
	inputErr := c.WaitForPacketInputShutdown()
	mapErr := c.WaitForStreamMapShutdown()
	receiveErr := c.WaitForStreamReceiveShutdown()
	// Send data shutdown already joins all six stream-call classes and framer.
	sendErr := c.WaitForStreamSendDataShutdown()
	if err := errors.Join(inputErr, mapErr, receiveErr, sendErr); err != nil {
		returned = true
		return errors.Join(errStreamMapRetirement, err)
	}
	r.stage = mapRetirementSnapshot
	m.mutex.Lock()
	err := m.captureRetirementSets()
	m.mutex.Unlock()
	if err != nil {
		returned = true
		return err
	}
	// A fully built but unpublished candidate has no stream/native call to
	// perform after loop end. Prove every child empty before its original close.
	if r.distinct(2) && r.sets[2] != r.sets[0] {
		r.stage, r.index = mapRetirementUnusedCandidate, 2
		if !r.sets[2].unusedEmpty() {
			returned = true
			return errStreamMapRetirement
		}
		if err := r.sets[2].closeWithError(streamShutdownCause(nil)); err != nil {
			returned = true
			return errors.Join(errStreamMapRetirement, err)
		}
	}
	r.stage = mapRetirementPreflight
	for i := range r.sets {
		if !r.distinct(i) {
			continue
		}
		r.index = i
		if err := r.sets[i].metadataReady(); err != nil {
			returned = true
			return errors.Join(errStreamMapRetirement, err)
		}
	}
	r.stage = mapRetirementClearing
	for i := range r.sets {
		if !r.distinct(i) {
			continue
		}
		r.index = i
		if err := r.sets[i].retireMetadata(); err != nil {
			returned = true
			return errors.Join(errStreamMapRetirement, err)
		}
	}
	r.stage = mapRetirementParent
	m.mutex.Lock()
	if m.currentMapSet() != r.sets[0] {
		m.mutex.Unlock()
		returned = true
		return errStreamMapRetirement
	}
	m.generation = nil
	m.outgoingBidiStreams, m.outgoingUniStreams = nil, nil
	m.incomingBidiStreams, m.incomingUniStreams = nil, nil
	m.ctx, m.newFlowController, m.queueControlFrame = nil, nil, nil
	m.receivedParameters, m.appliedParameters = nil, nil
	m.resetState.original, m.resetState.candidate = streamMapSet{}, streamMapSet{}
	m.resetState.flowReset = nil
	m.mutex.Unlock()
	returned = true
	return nil
}
