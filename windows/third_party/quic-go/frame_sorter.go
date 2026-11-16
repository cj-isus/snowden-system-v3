package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/internal/utils/tree"
)

// byteInterval is an interval from one ByteCount to the other
type byteInterval struct {
	Start protocol.ByteCount
	End   protocol.ByteCount
}

type frameSorterEntry struct {
	Data   []byte
	DoneCb func()
}

type frameSorter struct {
	queue               map[protocol.ByteCount]frameSorterEntry
	readPos             protocol.ByteCount
	gapTree             *tree.Btree[utils.ByteInterval]
	closed              bool
	retirementUncertain bool
	owned               map[*frameSorterHold]struct{}
	ownedBytes          int
	ownedCount          int
	incoming            frameSorterEntry
	incomingActive      bool
	refused             frameSorterEntry
	refusedActive       bool
	connectionBudget    *streamReceiveBudget
}

var errDuplicateStreamData = errors.New("duplicate stream data")
var errFrameSorterClosed = errors.New("stream frame sorter is closed")
var errFrameSorterUncertain = errors.New("stream frame sorter release did not return normally")

func newFrameSorter() *frameSorter {
	s := &frameSorter{}
	s.init()
	return s
}

// Caller retains the original shell before native tree/map initialization.
func (s *frameSorter) init() {
	s.gapTree = tree.New[utils.ByteInterval]()
	s.queue = make(map[protocol.ByteCount]frameSorterEntry)
	s.gapTree.Insert(utils.ByteInterval{Start: 0, End: protocol.MaxByteCount})
}

func (s *frameSorter) Push(data []byte, offset protocol.ByteCount, doneCb func()) error {
	if s.closed || s.retirementUncertain || s.incomingActive {
		if s.retirementUncertain {
			return s.refuse(data, doneCb, errFrameSorterUncertain)
		}
		return s.refuse(data, doneCb, errFrameSorterClosed)
	}
	s.incoming, s.incomingActive = frameSorterEntry{Data: data, DoneCb: doneCb}, true
	if s.owned != nil {
		original, err := s.reserveHold(cap(data))
		if err != nil {
			s.releaseEntry(s.incoming)
			s.incoming, s.incomingActive = frameSorterEntry{}, false
			return err
		}
		original.data, original.done = data, doneCb
		doneCb = original.release
		s.incoming.DoneCb = doneCb
	}
	err := s.push(data, offset, doneCb)
	if err != nil {
		if err != errDuplicateStreamData {
			s.closed = true // failed gap mutation cannot be used by another reader
		}
		s.releaseEntry(s.incoming)
	}
	s.incoming, s.incomingActive = frameSorterEntry{}, false
	if err == errDuplicateStreamData {
		err = nil
	}
	return err
}

func (s *frameSorter) push(data []byte, offset protocol.ByteCount, doneCb func()) error {
	if len(data) == 0 {
		return errDuplicateStreamData
	}

	start := offset
	end := offset + protocol.ByteCount(len(data))
	covInterval := utils.ByteInterval{Start: start, End: end}

	gaps := s.gapTree.Match(covInterval)

	if len(gaps) == 0 {
		// No overlap with any existing gap
		return errDuplicateStreamData
	}

	startGap := gaps[0]
	endGap := gaps[len(gaps)-1]
	startGapEqualsEndGap := len(gaps) == 1

	if startGapEqualsEndGap && end <= startGap.Start {
		return errDuplicateStreamData
	}

	startsInGap := covInterval.Start >= startGap.Start && covInterval.Start <= startGap.End
	endsInGap := covInterval.End >= endGap.Start && covInterval.End < endGap.End

	startGapEnd := startGap.End // save it, in case startGap is modified
	endGapStart := endGap.Start // save it, in case endGap is modified
	endGapEnd := endGap.End     // save it, in case endGap is modified

	var adjustedStartGapEnd bool
	var wasCut bool

	pos := start
	var hasReplacedAtLeastOne bool
	for {
		oldEntry, ok := s.queue[pos]
		if !ok {
			break
		}
		oldEntryLen := protocol.ByteCount(len(oldEntry.Data))
		if end-pos > oldEntryLen || (hasReplacedAtLeastOne && end-pos == oldEntryLen) {
			// The existing frame is shorter than the new frame. Replace it.
			s.releaseEntry(oldEntry)
			delete(s.queue, pos)
			pos += oldEntryLen
			hasReplacedAtLeastOne = true
		} else {
			if !hasReplacedAtLeastOne {
				return errDuplicateStreamData
			}
			// The existing frame is longer than the new frame.
			// Cut the new frame such that the end aligns with the start of the existing frame.
			data = data[:pos-start]
			end = pos
			wasCut = true
			break
		}
	}

	if !startsInGap && !hasReplacedAtLeastOne {
		// cut the frame, such that it starts at the start of the gap
		data = data[startGap.Start-start:]
		start = startGap.Start
		wasCut = true
	}
	if start <= startGap.Start {
		if end >= startGap.End {
			// The frame covers the whole startGap. Delete the gap.
			s.gapTree.Delete(startGap)
		} else {
			s.gapTree.Delete(startGap)
			startGap.Start = end
			// Re-insert the gap, but with the new start.
			s.gapTree.Insert(startGap)
		}
	} else if !hasReplacedAtLeastOne {
		s.gapTree.Delete(startGap)
		startGap.End = start
		// Re-insert the gap, but with the new end.
		s.gapTree.Insert(startGap)
		adjustedStartGapEnd = true
	}

	if !startGapEqualsEndGap {
		s.deleteConsecutive(startGapEnd)
		for _, g := range gaps[1:] {
			if g.End >= endGapStart {
				break
			}
			s.deleteConsecutive(g.End)
			s.gapTree.Delete(g)
		}
	}

	if !endsInGap && start != endGapEnd && end > endGapEnd {
		// cut the frame, such that it ends at the end of the gap
		data = data[:endGapEnd-start]
		end = endGapEnd
		wasCut = true
	}
	if end == endGapEnd {
		if !startGapEqualsEndGap {
			// The frame covers the whole endGap. Delete the gap.
			s.gapTree.Delete(endGap)
		}
	} else {
		if startGapEqualsEndGap && adjustedStartGapEnd {
			// The frame split the existing gap into two.
			s.gapTree.Insert(utils.ByteInterval{Start: end, End: startGapEnd})
		} else if !startGapEqualsEndGap {
			s.gapTree.Delete(endGap)
			endGap.Start = end
			// Re-insert the gap, but with the new start.
			s.gapTree.Insert(endGap)
		}
	}

	if wasCut && len(data) < protocol.MinStreamFrameBufferSize {
		newData, copiedDone, err := s.copyTrim(data)
		if err != nil {
			return err
		}
		if doneCb != nil {
			s.releaseEntry(s.incoming)
		}
		data, doneCb = newData, copiedDone
		s.incoming = frameSorterEntry{Data: data, DoneCb: doneCb}
	}

	if s.gapTree.Len() > protocol.MaxStreamFrameSorterGaps {
		return errors.New("too many gaps in received data")
	}

	s.queue[start] = frameSorterEntry{Data: data, DoneCb: doneCb}
	return nil
}

// deleteConsecutive deletes consecutive frames from the queue, starting at pos
func (s *frameSorter) deleteConsecutive(pos protocol.ByteCount) {
	for {
		oldEntry, ok := s.queue[pos]
		if !ok {
			break
		}
		oldEntryLen := protocol.ByteCount(len(oldEntry.Data))
		s.releaseEntry(oldEntry)
		delete(s.queue, pos)
		pos += oldEntryLen
	}
}

func (s *frameSorter) Pop() (protocol.ByteCount, []byte, func()) {
	if s.closed || s.retirementUncertain || s.incomingActive || s.refusedActive {
		return s.readPos, nil, nil
	}
	entry, ok := s.queue[s.readPos]
	if !ok {
		return s.readPos, nil, nil
	}
	delete(s.queue, s.readPos)
	offset := s.readPos
	s.readPos += protocol.ByteCount(len(entry.Data))
	return offset, entry.Data, entry.DoneCb
}

// HasMoreData says if there is any more data queued at *any* offset.
func (s *frameSorter) HasMoreData() bool {
	return len(s.queue) > 0
}

// The owning receive mutex excludes current readers and new frame transfers.
// Keep each original entry until its callback actually returns. A failed
// callback seals the sorter negatively; a later Discard must never retry it.
func (s *frameSorter) Discard() error {
	if s.retirementUncertain || s.incomingActive || s.refusedActive {
		s.retirementUncertain = true
		return errFrameSorterUncertain
	}
	s.closed = true
	for offset, entry := range s.queue {
		s.releaseEntry(entry)
		delete(s.queue, offset)
	}
	s.queue = nil
	s.gapTree = nil
	return nil
}

var errTooLittleData = errors.New("too little data")

// Peek copies len(p) consecutive bytes starting at offset into p, without removing them.
// It is only possible to peek from an offset where a frame starts.
//
// If there isn't enough consecutive data available, errTooLittleData is returned.
func (s *frameSorter) Peek(offset protocol.ByteCount, p []byte) error {
	if s.closed || s.retirementUncertain || s.incomingActive || s.refusedActive {
		if s.retirementUncertain || s.incomingActive || s.refusedActive {
			return errFrameSorterUncertain
		}
		return errFrameSorterClosed
	}
	if len(p) == 0 {
		return nil
	}

	// first, check if we have enough consecutive data available
	pos := offset
	remaining := len(p)
	for remaining > 0 {
		entry, ok := s.queue[pos]
		if !ok {
			return errTooLittleData
		}
		entryLen := len(entry.Data)
		if remaining <= entryLen {
			break // enough data available
		}
		remaining -= entryLen
		pos += protocol.ByteCount(entryLen)
	}

	pos = offset
	var copied int
	for copied < len(p) {
		entry := s.queue[pos] // the entry is guaranteed to exist from the check above
		copied += copy(p[copied:], entry.Data)
		pos += protocol.ByteCount(len(entry.Data))
	}
	return nil
}
