package handshake

import (
	"crypto/tls"
	"errors"

	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/qlogwriter"
)

var ErrCryptoKeyReferences = errors.New("quic: original crypto key reference retirement is uncertain")

// One fixed prefix in the original Close cell, not another key owner or copy.
// An abnormal result retains the original references even after a partial pass.
type cryptoKeyReferences struct {
	aead    *updatableAEAD
	config  *tls.Config
	params  [3]*wire.TransportParameters
	openers [3]LongHeaderOpener
	sealers [3]LongHeaderSealer
	rtt     *utils.RTTStats
	trace   qlogwriter.Recorder
	logger  utils.Logger
}

// RetireKeyReferences is external-only after original Conn/native/public/data/
// history/TLS/event receipts and local sorter preflight. No TLS/provider calls,
// waits, key-byte writes or AEAD mutations occur while holding the metadata lock.
// The original TLS Conn remains for ConnectionState and can retain its own keys.
func (h *cryptoSetup) RetireKeyReferences() (result error) {
	r := &h.closeState
	r.access.Lock()
	defer r.access.Unlock()
	if r.keysStarted {
		return r.keysResult
	}
	r.keysStarted = true
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(ErrCryptoKeyReferences, result)
		}
		r.keysResult = result
	}()
	if r.owner != h || r.done == nil || !r.started || !r.returned ||
		r.original != nil || r.closed == nil || r.closed != h.conn ||
		!r.eventsStarted || r.eventsResult != nil || !h.eventsSealed ||
		h.events != nil || h.eventBytes != 0 || h.keysRetired ||
		h.aead == nil || h.tlsConf == nil {
		returned = true
		return ErrCryptoKeyReferences
	}
	r.keys = cryptoKeyReferences{
		aead: h.aead, config: h.tlsConf,
		params:  [3]*wire.TransportParameters{h.ourParams, h.peerParams, h.zeroRTTParameters},
		openers: [3]LongHeaderOpener{h.initialOpener, h.handshakeOpener, h.zeroRTTOpener},
		sealers: [3]LongHeaderSealer{h.initialSealer, h.handshakeSealer, h.zeroRTTSealer},
		rtt:     h.rttStats, trace: h.qlogger, logger: h.logger,
	}
	// Only original, quiescent, serialized users can reach this boundary. Late
	// serial key APIs are sealed before any reference can become nil.
	h.keysRetired = true
	h.aead, h.tlsConf, h.rttStats = nil, nil, nil
	h.ourParams, h.peerParams, h.zeroRTTParameters = nil, nil, nil
	h.initialOpener, h.handshakeOpener, h.zeroRTTOpener = nil, nil, nil
	h.initialSealer, h.handshakeSealer, h.zeroRTTSealer = nil, nil, nil
	h.qlogger, h.logger = nil, nil
	// Do not reset historical flags/Used0RTT or mutate objects behind aliases.
	r.keys = cryptoKeyReferences{}
	returned = true
	return nil
}
