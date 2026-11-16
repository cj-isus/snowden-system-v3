package quic

import (
	"context"

	"github.com/sagernet/quic-go/internal/flowcontrol"
	"github.com/sagernet/quic-go/internal/protocol"
)

type streamConstructionStage uint8

const (
	constructionIdle streamConstructionStage = iota
	constructionFlow
	constructionFlowReady
	constructionBidi
	constructionSend
	constructionContext
	constructionContextReady
	constructionReceive
	constructionSorter
	constructionReturned
)

// One fixed prefix in the original map creation record, protected by its
// factory gate. Unknown creation retains this prefix and the original row
// credits; it never tries to close a partially initialized public stream.
type streamConstruction struct {
	stage             streamConstructionStage
	id                protocol.StreamID
	parent            context.Context
	sender            streamSender
	parameters        *streamParameters
	initialSendWindow protocol.ByteCount
	flowFactory       func(protocol.StreamID, protocol.ByteCount) flowcontrol.StreamFlowController
	flow              flowcontrol.StreamFlowController
	stream            *Stream
	send              *SendStream // also retains a returned context/cancel pair
	receive           *ReceiveStream
	sendSender        *uniStreamSender
	receiveSender     *uniStreamSender
}

func (c *streamConstruction) begin(
	parent context.Context,
	id protocol.StreamID,
	sender streamSender,
	parameters *streamParameters,
	initialSendWindow protocol.ByteCount,
	factory func(protocol.StreamID, protocol.ByteCount) flowcontrol.StreamFlowController,
) flowcontrol.StreamFlowController {
	if c == nil || c.stage != constructionIdle || parent == nil || sender == nil || parameters == nil || factory == nil {
		panic(errStreamMapUncertain)
	}
	*c = streamConstruction{
		stage: constructionFlow, id: id, parent: parent, sender: sender,
		parameters: parameters, initialSendWindow: initialSendWindow, flowFactory: factory,
	}
	// Only a returned result is known. Hidden resources in an arbitrary opaque
	// provider that never returns are not claimed recovered by this record.
	c.flow = c.flowFactory(c.id, c.initialSendWindow)
	if c.flow == nil {
		panic(errStreamMapUncertain)
	}
	c.stage = constructionFlowReady
	return c.flow
}

func (c *streamConstruction) require(id protocol.StreamID) {
	if c == nil || c.stage == constructionIdle || c.stage == constructionReturned || c.id != id || c.flow == nil {
		panic(errStreamMapUncertain)
	}
}
