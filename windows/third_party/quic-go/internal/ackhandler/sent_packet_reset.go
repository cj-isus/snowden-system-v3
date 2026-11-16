package ackhandler

import (
	"errors"

	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
)

type packetResetKind uint8

const (
	packetResetIdle packetResetKind = iota
	packetResetRetry
	packetResetPath
)

type packetResetStage uint8

const (
	packetResetCaptured packetResetStage = iota + 1
	packetResetReturningFrames
	packetResetMeasuringRTT
	packetResetBuilding
	packetResetPublishing
	packetResetChangingCongestion
	packetResetTimer
)

// Original native Conn loop only. Keep both old spaces, candidate spaces and
// the current packet before history removal/callbacks. It is not a rollback
// journal: unknown or negative work is retained without a second attempt.
type sentPacketReset struct {
	kind                               packetResetKind
	stage                              packetResetStage
	ended, uncertain                   bool
	failure                            error
	initial, app, nextInitial, nextApp *packetNumberSpace
	current                            *packet
	firstPacketSendTime                monotime.Time
	previousBytesInFlight              protocol.ByteCount
	previousPTOCount                   uint32
	previousAlarm                      alarmTimer
}

func packetSpaceResetReady(space *packetNumberSpace) bool {
	return space != nil && space.pns != nil && space.history.failure == nil &&
		!space.history.admitting && !space.history.pendingProbe && space.history.pendingPacket == nil &&
		space.history.numOutstanding >= 0 && space.history.numOutstanding <= len(space.history.packets)
}

func (h *sentPacketHandler) beginPacketReset(kind packetResetKind) (*sentPacketReset, error) {
	if h.historyFailure != nil {
		return nil, h.historyFailure
	}
	if h.resettingPackets.kind != packetResetIdle || !packetSpaceResetReady(h.appDataPackets) ||
		(kind != packetResetRetry && kind != packetResetPath) {
		return nil, h.failHistory(ErrSentHistoryUncertain)
	}
	if kind == packetResetRetry && (!packetSpaceResetReady(h.initialPackets) || len(h.appDataPackets.history.pathProbePackets) != 0) {
		return nil, h.failHistory(ErrSentHistoryUncertain)
	}
	call := &h.resettingPackets
	*call = sentPacketReset{
		kind: kind, stage: packetResetCaptured, initial: h.initialPackets, app: h.appDataPackets,
		previousBytesInFlight: h.bytesInFlight, previousPTOCount: h.ptoCount, previousAlarm: h.alarm,
	}
	return call, nil
}

func (h *sentPacketHandler) finishPacketReset(call *sentPacketReset, returned bool, result *error) {
	if call != &h.resettingPackets || !returned || *result != nil {
		*result = errors.Join(ErrSentHistoryUncertain, *result)
		h.failHistory(*result)
		if call == &h.resettingPackets {
			call.ended, call.uncertain, call.failure = true, !returned, *result
		}
		return
	}
	*call = sentPacketReset{} // known transfer only; no pool/native/resource close
}
