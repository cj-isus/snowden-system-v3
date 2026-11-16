package quic

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/sagernet/quic-go/internal/handshake"
	"github.com/sagernet/quic-go/internal/wire"
)

type connectionConstructionStage uint8

const (
	connectionConstructingShell connectionConstructionStage = iota + 1
	connectionConstructingTrace
	connectionConstructingIDs
	connectionConstructingContext
	connectionConstructingSetup
	connectionConstructingACK
	connectionConstructingCryptoStreams
	connectionConstructingParameters
	connectionConstructingCrypto
	connectionConstructingPacker
	connectionConstructingToken
	connectionConstructed
)

// The original opening already retains Conn. Its original constructor alone
// owns this fixed prefix until known completion; no partial native cleanup or
// concurrent observer treats it as an initialized connection. Returned child
// shells live in typed Conn fields or crypto below before later callbacks.
type connectionConstruction struct {
	stage                 connectionConstructionStage
	complete              bool
	parent                context.Context
	tlsConf               *tls.Config
	runner                connRunner
	generator             ConnectionIDGenerator
	localAddr, remoteAddr net.Addr
	parameters            *wire.TransportParameters
	crypto                handshake.CryptoSetup
	token                 *ClientToken
}

func (c *Conn) finishConstruction() {
	p := &c.construction
	if p.complete || (p.stage != connectionConstructingPacker && p.stage != connectionConstructingToken) ||
		c.ctx == nil || c.ctxCancel == nil || c.initialStream == nil ||
		c.handshakeStream == nil || c.oneRTTStream == nil || c.streamsMap == nil ||
		!c.streamsMap.currentMapSet().valid() || c.streamsMap.initialMaps != (streamMapSet{}) ||
		c.sentPacketHandler == nil || c.receivedPacketHandler == nil || c.cryptoStreamHandler == nil || p.crypto == nil ||
		c.packer == nil || c.unpacker == nil || c.cryptoStreamManager == nil {
		panic(errPacketOutputUncertain)
	}
	// Every temporary result now has its original complete Conn owner. Do not
	// retain duplicate constructor inputs/aliases for the lifetime of the Conn.
	*p = connectionConstruction{stage: connectionConstructed, complete: true}
}
