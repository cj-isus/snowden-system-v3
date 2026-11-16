package ackhandler

import (
	"errors"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
)

var ErrSentHistoryRetirement = errors.New("quic: original sent-history metadata retirement is uncertain")

type sentHistoryRetirementStage uint8

const (
	sentHistoryPreflight sentHistoryRetirementStage = iota + 1
	sentHistoryClearing
	sentHistoryScratch
)

// One original external finalizer. access protects admission/result only; the
// finalizer owns its fixed space prefix/stage/index until publishing done.
type sentHistoryRetirement struct {
	access         sync.Mutex
	done           chan struct{}
	started, ended bool
	result         error
	spaces         [3]*packetNumberSpace
	stage          sentHistoryRetirementStage
	index          int
}

func sentHistoryMetadataReady(space *packetNumberSpace) bool {
	if !packetSpaceResetReady(space) {
		return false
	}
	history := &space.history
	return cap(history.packets) <= protocol.MaxTrackedSentPackets &&
		cap(history.pathProbePackets) <= protocol.MaxTrackedSentPackets &&
		len(history.pathProbePackets) <= len(history.packets) &&
		cap(history.skippedPackets) <= maxSkippedPackets
}

// RetireHistoryMetadata is external-owner only, AFTER positive native-loop,
// public-congestion-call, STREAM data/all-call/framer and DATAGRAM data receipts.
// Closing is a defensive check, not a substitute for those caller-side joins.
// It drops owned pointer cells without inspecting recycled packets/frames,
// calling ACK/loss handlers, or returning anything to a pool.
func (h *sentPacketHandler) RetireHistoryMetadata() (result error) {
	r := &h.retirement
	r.access.Lock()
	if r.done == nil {
		r.access.Unlock()
		return ErrSentHistoryRetirement
	}
	if r.started {
		done := r.done
		r.access.Unlock()
		<-done
		r.access.Lock()
		result = r.result
		if !r.ended {
			result = errors.Join(ErrSentHistoryRetirement, result)
		}
		r.access.Unlock()
		return result
	}
	r.started, r.stage = true, sentHistoryPreflight
	r.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(ErrSentHistoryRetirement, result)
		}
		r.access.Lock()
		defer r.access.Unlock()
		r.result, r.ended = result, true
		if returned && result == nil {
			r.spaces = [3]*packetNumberSpace{}
		}
		close(r.done)
	}()
	// Capture before any validation or mutation; failed original work is retained.
	r.spaces = [3]*packetNumberSpace{h.initialPackets, h.handshakePackets, h.appDataPackets}
	select {
	case <-h.congestionClosing:
	default:
		returned = true
		return ErrSentHistoryRetirement
	}
	if err := errors.Join(h.PacketStateError(), h.CongestionStateError()); err != nil {
		returned = true
		return errors.Join(ErrSentHistoryRetirement, err)
	}
	// A partial ACK batch can mix history/pool aliases. Never guess its progress.
	if r.spaces[2] == nil || len(h.ackedPackets) != 0 {
		returned = true
		return ErrSentHistoryRetirement
	}
	for i, space := range r.spaces {
		r.index = i
		if space == nil {
			continue // Initial and Handshake may already have been dropped normally.
		}
		for j := 0; j < i; j++ {
			if space == r.spaces[j] {
				returned = true
				return ErrSentHistoryRetirement
			}
		}
		if !sentHistoryMetadataReady(space) {
			returned = true
			return ErrSentHistoryRetirement
		}
	}
	// Every current space passed preflight before any metadata is cleared.
	r.stage = sentHistoryClearing
	for i, space := range r.spaces {
		r.index = i
		if space == nil {
			continue
		}
		history := &space.history
		clear(history.packets[:cap(history.packets)])
		clear(history.pathProbePackets[:cap(history.pathProbePackets)])
		history.packets, history.pathProbePackets, history.skippedPackets = nil, nil, nil
		history.numOutstanding = 0
	}
	r.stage = sentHistoryScratch
	h.ackedPackets = nil
	h.lostPackets.lostPackets = nil
	// Providers may retain scalar info aliases: drop our heads, don't zero theirs.
	h.ackedPacketsInfo, h.lostPacketsInfo = nil, nil
	// Preserve scalar statistics, RTT/controller/ECN and their separate lifetimes.
	returned = true
	return nil
}
