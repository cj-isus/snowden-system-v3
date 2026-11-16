package ackhandler

import (
	"errors"

	"github.com/sagernet/quic-go/internal/protocol"
)

var (
	errSentHistoryCapacity  = errors.New("quic: sent packet history capacity exceeded")
	ErrSentHistoryUncertain = errors.New("quic: original sent packet intake is uncertain")
)

// Keep the existing scheduler thresholds. This hard fence precedes packet-pool
// allocation and clamps physical slice growth, including holes and path probes.
func (h *sentPacketHistory) beginTracking(pathProbe bool) error {
	if h.failure != nil {
		return h.failure
	}
	if h.admitting {
		h.failure = ErrSentHistoryUncertain
		return h.failure
	}
	if len(h.packets) >= protocol.MaxTrackedSentPackets || pathProbe && len(h.pathProbePackets) >= protocol.MaxTrackedSentPackets {
		h.failure = errSentHistoryCapacity
		return h.failure
	}
	h.admitting, h.pendingProbe = true, pathProbe
	h.packets = growSentHistory(h.packets)
	if pathProbe {
		h.pathProbePackets = growSentHistory(h.pathProbePackets)
	}
	return nil
}

// No append can grow beyond the hard ceiling through Go's implicit growth rule.
func growSentHistory[T any](values []T) []T {
	if len(values) < cap(values) {
		return values
	}
	capacity := max(len(values)+1, 2*cap(values))
	capacity = min(capacity, protocol.MaxTrackedSentPackets)
	output := make([]T, len(values), capacity)
	copy(output, values)
	return output
}

func (h *sentPacketHistory) commitPacket(pn protocol.PacketNumber, p *packet, pathProbe bool) error {
	if h.failure != nil {
		return h.failure
	}
	if !h.admitting {
		if err := h.beginTracking(pathProbe); err != nil {
			return err
		}
		h.pendingPacket = p // legacy direct history caller; production reserves earlier
	}
	if h.pendingPacket != p || h.pendingProbe != pathProbe || p == nil {
		h.failure = ErrSentHistoryUncertain
		return h.failure
	}
	h.checkSequentialPacketNumberUse(pn)
	if pathProbe {
		h.packets = append(h.packets, &packet{isPathProbePacket: true})
		h.pathProbePackets = append(h.pathProbePackets, packetWithPacketNumber{PacketNumber: pn, packet: p})
	} else {
		h.packets = append(h.packets, p)
		if p.Outstanding() {
			h.numOutstanding++
		}
	}
	h.pendingPacket = nil
	h.admitting, h.pendingProbe = false, false
	return nil
}

func (h *sentPacketHandler) failHistory(err error) error {
	if h.historyFailure == nil {
		h.historyFailure = err
	}
	return h.historyFailure
}
