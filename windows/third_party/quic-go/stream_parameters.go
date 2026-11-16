package quic

import (
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

// Immutable scalar copy of an actual restored or validated received/applicable
// parameter set. No borrowed wire object, address, reset token or key is stored.
type streamParameters struct {
	bidiLocal     protocol.ByteCount
	bidiRemote    protocol.ByteCount
	uni           protocol.ByteCount
	maxBidi       protocol.StreamNum
	maxUni        protocol.StreamNum
	resetStreamAt bool
}

func newStreamParameters(p *wire.TransportParameters) *streamParameters {
	if p == nil {
		panic(errStreamMapUncertain)
	}
	return &streamParameters{
		bidiLocal: p.InitialMaxStreamDataBidiLocal, bidiRemote: p.InitialMaxStreamDataBidiRemote,
		uni: p.InitialMaxStreamDataUni, maxBidi: p.MaxBidiStreamNum, maxUni: p.MaxUniStreamNum,
		resetStreamAt: p.EnableResetStreamAt,
	}
}

func (g *streamGeneration) outgoingConfig() *streamParameters {
	p := g.outgoingParameters.Load()
	if p == nil {
		panic(errStreamMapUncertain) // no synthetic limits before actual publication
	}
	return p
}

func (g *streamGeneration) incomingConfig() *streamParameters {
	p := g.incomingParameters.Load()
	if p == nil {
		panic(errStreamMapUncertain)
	}
	return p
}

// Original Conn loop only, after validating received transport parameters.
// Client inbound 1-RTT streams can precede outbound parameter application.
func (m *streamsMap) receivedTransportParameters(p *wire.TransportParameters) {
	snapshot := newStreamParameters(p)
	m.receivedParameters = snapshot
	m.generation.incomingParameters.Store(snapshot)
}
