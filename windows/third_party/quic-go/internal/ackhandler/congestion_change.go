package ackhandler

import (
	"errors"
	"net"
	"reflect"

	congestionExt "github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/quic-go/internal/congestion"
)

var (
	ErrCongestionChangeUncertain = errors.New("quic: original congestion change is uncertain")
	ErrCongestionChangeBusy      = errors.New("quic: original congestion change is active")
	errCongestionControlMissing  = errors.New("quic: nil congestion control")
)

// One bounded original change, shared by native migration and public setter.
// congestionMutex protects admission/result/publication; only the admitted call
// mutates its source/candidate outside the lock. Never wait for a callback here.
type congestionChange struct {
	active, published   bool
	previous, candidate congestion.SendAlgorithmWithDebugInfos
	source              congestionExt.CongestionControl
}

func congestionControlMissing(cc congestionExt.CongestionControl) bool {
	if cc == nil {
		return true
	}
	switch value := reflect.ValueOf(cc); value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// Caller holds congestionMutex. Closing is bound before production exposure.
func (h *sentPacketHandler) congestionClosingLocked() bool {
	select {
	case <-h.congestionClosing:
		return true
	default:
		return false
	}
}

func (h *sentPacketHandler) beginCongestionChange(source congestionExt.CongestionControl) (*congestionChange, error) {
	h.congestionMutex.Lock()
	defer h.congestionMutex.Unlock()
	if h.congestionFailed || h.congestion == nil {
		return nil, ErrCongestionChangeUncertain
	}
	if h.congestionClosingLocked() {
		return nil, net.ErrClosed
	}
	if h.changingCongestion.active {
		return nil, ErrCongestionChangeBusy
	}
	call := &h.changingCongestion
	*call = congestionChange{active: true, previous: h.congestion, source: source}
	return call, nil
}

func (h *sentPacketHandler) publishCongestionChange(call *congestionChange) error {
	h.congestionMutex.Lock()
	defer h.congestionMutex.Unlock()
	if call != &h.changingCongestion || !call.active || call.published || call.candidate == nil || h.congestionFailed {
		return ErrCongestionChangeUncertain
	}
	if h.congestionClosingLocked() {
		return net.ErrClosed
	}
	h.congestion = call.candidate
	call.published = true
	return nil
}

func (h *sentPacketHandler) finishCongestionChange(call *congestionChange, returned bool, result error) {
	h.congestionMutex.Lock()
	defer h.congestionMutex.Unlock()
	if call != &h.changingCongestion || !call.active || !returned {
		h.congestionFailed = true
		return // retain previous/source/candidate; no cleanup/retry of unknown work
	}
	if result == net.ErrClosed && !call.published && !h.congestionFailed && h.congestionClosingLocked() {
		// Setup returned, publication refused by the original Closing signal.
		// Previous remains current; only temporary aliases are dropped. This is
		// a known API failure, not an unknown callback or an applied candidate.
		*call = congestionChange{}
		return
	}
	if result != nil || !call.published {
		h.congestionFailed = true
		return
	}
	*call = congestionChange{}
}

func (h *sentPacketHandler) SetCongestionControl(cc congestionExt.CongestionControl) (result error) {
	if congestionControlMissing(cc) {
		return errCongestionControlMissing
	}
	call, err := h.beginCongestionChange(cc)
	if err != nil {
		return err
	}
	returned := false
	defer func() { h.finishCongestionChange(call, returned, result) }()
	if ccEx, ok := cc.(congestionExt.CongestionControlEx); ok {
		call.candidate = &ccAdapterEx{ccEx}
	} else {
		call.candidate = &ccAdapter{cc}
	}
	cc.SetRTTStatsProvider(h.rttStats) // foreign callback, no metadata mutex held
	result = h.publishCongestionChange(call)
	returned = true
	return result
}

// Native producer-loop state only. Public setter completion has its separate
// external Conn receipt; an active external callback is not a native-loop join.
func (h *sentPacketHandler) PacketStateError() error {
	if h.historyFailure != nil {
		return h.historyFailure
	}
	if h.resettingPackets.kind != packetResetIdle {
		return ErrSentHistoryUncertain
	}
	return nil
}

// External owner after native loop and public setter receipt. It does not call
// a controller, clear its retained change prefix or imply physical retirement.
func (h *sentPacketHandler) CongestionStateError() error {
	h.congestionMutex.RLock()
	failed := h.congestionFailed || h.changingCongestion.active
	h.congestionMutex.RUnlock()
	if failed {
		return ErrCongestionChangeUncertain
	}
	return nil
}
