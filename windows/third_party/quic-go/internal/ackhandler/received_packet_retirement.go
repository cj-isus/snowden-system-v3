package ackhandler

import (
	"errors"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/quicvarint"
)

var ErrReceivedHistoryRetirement = errors.New("quic: original received-history metadata retirement is uncertain")

type receivedHistoryRetirement struct {
	// owner is immutable. Copies of this handler cannot retire the original.
	owner          *ReceivedPacketHandler
	access         sync.Mutex
	done           chan struct{}
	started, ended bool
	result         error
	trackers       [3]*receivedPacketTracker // one finalizer's fixed original prefix
	index          int
}

func receivedHistoryMetadataReady(history *receivedPacketHistory) bool {
	// LargestAcked+1 is a valid ignore-all floor, even at the maximum packet.
	if history.deletedBelow < protocol.InvalidPacketNumber ||
		history.deletedBelow > protocol.PacketNumber(quicvarint.Max)+1 ||
		len(history.ranges) > protocol.MaxNumAckRanges {
		return false
	}
	previous := protocol.InvalidPacketNumber
	for _, interval := range history.ranges {
		if interval.Start < 0 || interval.End < interval.Start ||
			interval.End > protocol.PacketNumber(quicvarint.Max) || interval.Start <= previous ||
			history.deletedBelow != protocol.InvalidPacketNumber && interval.Start < history.deletedBelow {
			return false
		}
		previous = interval.End
	}
	return true
}

// RetireHistoryMetadata is external-only AFTER positive native input/output,
// public setter/data/call/map and sent-history receipts. Closing is defensive,
// not authorization. It drops only owned heads; cached ACK objects and their
// backing are never inspected, reset, pooled or modified through stale aliases.
func (h *ReceivedPacketHandler) RetireHistoryMetadata() (result error) {
	if h == nil || h.retirement.owner != h {
		return ErrReceivedHistoryRetirement
	}
	r := &h.retirement
	r.access.Lock()
	if r.done == nil {
		r.access.Unlock()
		return ErrReceivedHistoryRetirement
	}
	if r.started {
		done := r.done
		r.access.Unlock()
		<-done
		r.access.Lock()
		result = r.result
		if !r.ended {
			result = errors.Join(ErrReceivedHistoryRetirement, result)
		}
		r.access.Unlock()
		return result
	}
	r.started = true
	r.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(ErrReceivedHistoryRetirement, result)
		}
		r.access.Lock()
		defer r.access.Unlock()
		r.result, r.ended = result, true
		if returned && result == nil {
			r.trackers = [3]*receivedPacketTracker{}
		}
		close(r.done)
	}()
	r.trackers = [3]*receivedPacketTracker{h.initialPackets, h.handshakePackets,
		&h.appDataPackets.receivedPacketTracker}
	select {
	case <-h.closing:
	default:
		returned = true
		return ErrReceivedHistoryRetirement
	}
	if h.retired {
		returned = true
		return ErrReceivedHistoryRetirement
	}
	for i, tracker := range r.trackers {
		r.index = i
		if tracker == nil {
			continue // Initial/Handshake may have been dropped normally.
		}
		for j := 0; j < i; j++ {
			if tracker == r.trackers[j] {
				returned = true
				return ErrReceivedHistoryRetirement
			}
		}
		if !receivedHistoryMetadataReady(&tracker.packetHistory) {
			returned = true
			return ErrReceivedHistoryRetirement
		}
	}
	// Every tracker passed preflight before any original reference is dropped.
	h.retired = true
	for i, tracker := range r.trackers {
		r.index = i
		if tracker != nil {
			tracker.packetHistory.ranges = nil
			tracker.lastAck = nil // never dereference or reset this borrowed ACK alias
			tracker.hasNewAck = false
		}
	}
	h.appDataPackets.ackQueued, h.appDataPackets.ackAlarm = false, 0
	h.appDataPackets.ackElicitingPacketsReceivedSinceLastAck = 0
	// Keep ECN/observed/floor/connection statistics; this fabricates no ACK delivery.
	returned = true
	return nil
}
