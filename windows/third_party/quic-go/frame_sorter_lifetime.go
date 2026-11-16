package quic

import "errors"

const (
	maxReceiveFrameOwners = 8192
	maxReceiveFrameBytes  = 8 << 20
)

var errReceiveFrameCapacity = errors.New("quic: receive stream frame capacity exceeded")

// Only ReceiveStream enables this ledger. Crypto streams retain their original
// callback-free Pop contract. The receive mutex owns all hold operations.
func newReceiveFrameSorter(budget *streamReceiveBudget) *frameSorter {
	s := &frameSorter{}
	s.initReceiveFrameSorter(budget)
	return s
}

func (s *frameSorter) initReceiveFrameSorter(budget *streamReceiveBudget) {
	s.connectionBudget = budget
	s.init()
	s.owned = make(map[*frameSorterHold]struct{})
	if budget == nil {
		s.closed, s.retirementUncertain = true, true
	}
}

type frameSorterHold struct {
	owner    *frameSorter
	data     []byte
	done     func()
	charge   int
	started  bool
	returned bool
}

func (s *frameSorter) reserveHold(charge int) (*frameSorterHold, error) {
	if charge < 0 || s.ownedCount >= maxReceiveFrameOwners || charge > maxReceiveFrameBytes-s.ownedBytes {
		return nil, errReceiveFrameCapacity
	}
	// Reservation precedes holder metadata, callback wrapping and any new copy.
	s.ownedBytes += charge
	s.ownedCount++
	original := &frameSorterHold{owner: s, charge: charge}
	s.owned[original] = struct{}{}
	return original, nil
}

// Both ledgers reserve before clone allocation while the source remains held.
// The generic crypto sorter keeps its original callback-free allocation path.
func (s *frameSorter) copyTrim(data []byte) ([]byte, func(), error) {
	if s.owned == nil {
		output := make([]byte, len(data))
		copy(output, data)
		return output, nil, nil
	}
	local, err := s.reserveHold(len(data))
	if err != nil {
		return nil, nil, err
	}
	original, err := s.connectionBudget.allocateCopy(len(data))
	if err != nil {
		local.release()
		return nil, nil, err
	}
	output := original.bufferView()
	local.data, local.done = output, original.release
	copy(output, data)
	return output, local.release, nil
}

func (h *frameSorterHold) release() {
	if h.started {
		if !h.returned {
			panic(errFrameSorterUncertain)
		}
		return
	}
	h.started = true
	h.owner.retirementUncertain = true
	if h.done != nil {
		h.done()
	}
	h.returned = true
	h.data, h.done = nil, nil
	delete(h.owner.owned, h)
	h.owner.ownedBytes -= h.charge
	h.owner.ownedCount--
	h.owner.retirementUncertain = false
}

func (s *frameSorter) releaseEntry(entry frameSorterEntry) {
	s.retirementUncertain = true
	if entry.DoneCb != nil {
		entry.DoneCb()
	}
	s.retirementUncertain = false
}

// Refusal never overwrites an earlier abnormal incoming owner. This separate
// fixed slot consumes a fresh unadmitted input without refunding old credits.
func (s *frameSorter) refuse(data []byte, done func(), cause error) error {
	if s.refusedActive {
		return errFrameSorterUncertain
	}
	s.refused, s.refusedActive = frameSorterEntry{Data: data, DoneCb: done}, true
	priorFailure := s.retirementUncertain
	s.releaseEntry(s.refused)
	s.refused, s.refusedActive = frameSorterEntry{}, false
	s.retirementUncertain = priorFailure
	return cause
}
