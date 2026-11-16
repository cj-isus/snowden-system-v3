package quic

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sagernet/quic-go/internal/flowcontrol"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

// StreamLimitReachedError is returned from Conn.OpenStream and Conn.OpenUniStream
// when it is not possible to open a new stream because the number of opens streams reached
// the peer's stream limit.
type StreamLimitReachedError struct{}

func (e StreamLimitReachedError) Error() string { return "too many open streams" }

type streamsMap struct {
	ctx         context.Context // not used for cancellations, but carries the values associated with the connection
	perspective protocol.Perspective

	maxIncomingBidiStreams uint64
	maxIncomingUniStreams  uint64

	sender            streamSender
	queueControlFrame func(wire.Frame)
	newFlowController func(protocol.StreamID, protocol.ByteCount) flowcontrol.StreamFlowController
	framer            *framer
	onError           func(error)
	generation        *streamGeneration
	initialMaps       streamMapSet // retained original construction until publication
	// Original Conn loop only; public factories read their generation's atomics.
	receivedParameters *streamParameters
	appliedParameters  *streamParameters

	mutex               sync.Mutex
	outgoingBidiStreams *outgoingStreamsMap[*Stream]
	outgoingUniStreams  *outgoingStreamsMap[*SendStream]
	incomingBidiStreams *incomingStreamsMap[*Stream]
	incomingUniStreams  *incomingStreamsMap[*ReceiveStream]
	reset               bool
	resetState          streamMapReset
	retirement          streamMapRetirement
	frameIntake         streamFrameIntake // original Conn frame dispatcher, not a second producer
	receiveBudget       *streamReceiveBudget
	calls               streamMapCalls
}

func newStreamsMap(
	ctx context.Context,
	sender streamSender,
	queueControlFrame func(wire.Frame),
	newFlowController func(protocol.StreamID, protocol.ByteCount) flowcontrol.StreamFlowController,
	maxIncomingBidiStreams uint64,
	maxIncomingUniStreams uint64,
	perspective protocol.Perspective,
	receiveBudget *streamReceiveBudget,
	originalFramer *framer,
	onError func(error),
) *streamsMap {
	m := &streamsMap{}
	m.init(ctx, sender, queueControlFrame, newFlowController, maxIncomingBidiStreams,
		maxIncomingUniStreams, perspective, receiveBudget, originalFramer, onError)
	return m
}

// Conn retains m before this initializer allocates children or their metadata.
func (m *streamsMap) init(
	ctx context.Context,
	sender streamSender,
	queueControlFrame func(wire.Frame),
	newFlowController func(protocol.StreamID, protocol.ByteCount) flowcontrol.StreamFlowController,
	maxIncomingBidiStreams, maxIncomingUniStreams uint64,
	perspective protocol.Perspective,
	receiveBudget *streamReceiveBudget,
	originalFramer *framer,
	onError func(error),
) {
	if m == nil || m.retirement.done != nil || originalFramer == nil || onError == nil {
		panic(errStreamMapUncertain)
	}
	*m = streamsMap{
		ctx:                    ctx,
		perspective:            perspective,
		queueControlFrame:      queueControlFrame,
		newFlowController:      newFlowController,
		maxIncomingBidiStreams: maxIncomingBidiStreams,
		maxIncomingUniStreams:  maxIncomingUniStreams,
		sender:                 sender,
		receiveBudget:          receiveBudget,
		frameIntake:            streamFrameIntake{budget: receiveBudget},
		framer:                 originalFramer,
		onError:                onError,
		retirement:             streamMapRetirement{done: make(chan struct{})},
	}
	m.calls.init()
	m.initMaps()
}

func (m *streamsMap) initMaps() {
	m.buildMaps(&m.initialMaps)
	if !m.initialMaps.valid() {
		panic(errStreamMapUncertain)
	}
	m.publishMapSet(&m.initialMaps)
	m.initialMaps = streamMapSet{} // complete current fields now own this graph
}

func (m *streamsMap) buildMaps(target *streamMapSet) {
	if target == nil || *target != (streamMapSet{}) {
		panic(errStreamMapUncertain)
	}
	generation := &streamGeneration{framer: m.framer, sender: m.sender, onError: m.onError}
	generation.incomingParameters.Store(m.receivedParameters)
	generation.outgoingParameters.Store(m.appliedParameters)
	target.generation = generation
	outgoingBidiSender := &streamMapSender{generation: generation}
	outgoingBidi := &outgoingStreamsMap[*Stream]{}
	target.outgoingBidi = outgoingBidi
	outgoingBidi.init(
		protocol.StreamTypeBidi,
		func(id protocol.StreamID) *Stream {
			p := generation.outgoingConfig()
			construction := &outgoingBidi.creation.construction
			flow := construction.begin(m.ctx, id, outgoingBidiSender, p, p.bidiRemote, m.newFlowController)
			str := newStream(m.ctx, id, outgoingBidiSender, flow, p.resetStreamAt, m.receiveBudget, construction)
			construction.stage = constructionReturned
			return str
		},
		m.queueControlFrame,
		m.perspective,
	)
	outgoingBidiSender.completed = outgoingBidi.DeleteStream
	incomingBidiSender := &streamMapSender{generation: generation}
	incomingBidi := &incomingStreamsMap[*Stream]{}
	target.incomingBidi = incomingBidi
	incomingBidi.init(
		protocol.StreamTypeBidi,
		func(id protocol.StreamID) *Stream {
			p := generation.incomingConfig()
			construction := &incomingBidi.creation.construction
			flow := construction.begin(m.ctx, id, incomingBidiSender, p, p.bidiLocal, m.newFlowController)
			str := newStream(m.ctx, id, incomingBidiSender, flow, p.resetStreamAt, m.receiveBudget, construction)
			construction.stage = constructionReturned
			return str
		},
		m.maxIncomingBidiStreams,
		m.queueControlFrame,
		m.perspective,
	)
	incomingBidiSender.completed = incomingBidi.DeleteStream
	outgoingUniSender := &streamMapSender{generation: generation}
	outgoingUni := &outgoingStreamsMap[*SendStream]{}
	target.outgoingUni = outgoingUni
	outgoingUni.init(
		protocol.StreamTypeUni,
		func(id protocol.StreamID) *SendStream {
			p := generation.outgoingConfig()
			construction := &outgoingUni.creation.construction
			flow := construction.begin(m.ctx, id, outgoingUniSender, p, p.uni, m.newFlowController)
			str := newSendStream(m.ctx, id, outgoingUniSender, flow, p.resetStreamAt, m.receiveBudget, construction)
			construction.stage = constructionReturned
			return str
		},
		m.queueControlFrame,
		m.perspective,
	)
	outgoingUniSender.completed = outgoingUni.DeleteStream
	incomingUniSender := &streamMapSender{generation: generation}
	incomingUni := &incomingStreamsMap[*ReceiveStream]{}
	target.incomingUni = incomingUni
	incomingUni.init(
		protocol.StreamTypeUni,
		func(id protocol.StreamID) *ReceiveStream {
			p := generation.incomingConfig()
			construction := &incomingUni.creation.construction
			flow := construction.begin(m.ctx, id, incomingUniSender, p, p.uni, m.newFlowController)
			str := newReceiveStream(id, incomingUniSender, flow, m.receiveBudget, construction)
			construction.stage = constructionReturned
			return str
		},
		m.maxIncomingUniStreams,
		m.queueControlFrame,
		m.perspective,
	)
	incomingUniSender.completed = incomingUni.DeleteStream
}

func (m *streamsMap) OpenStream() (*Stream, error) {
	call, err := m.calls.begin(openBidiMapCall, nil)
	if err != nil {
		return nil, err
	}
	returned := false
	defer call.finish(&returned)
	m.mutex.Lock()
	reset := m.reset
	mm := m.outgoingBidiStreams
	m.mutex.Unlock()
	call.target = mm
	if reset {
		returned = true
		return nil, Err0RTTRejected
	}
	str, err := mm.OpenStream()
	str, err = streamMapResult(call, str, err)
	returned = true
	return str, err
}

func (m *streamsMap) OpenStreamSync(ctx context.Context) (*Stream, error) {
	call, err := m.calls.begin(openBidiSyncMapCall, ctx)
	if err != nil {
		return nil, err
	}
	returned := false
	defer call.finish(&returned)
	m.mutex.Lock()
	reset := m.reset
	mm := m.outgoingBidiStreams
	m.mutex.Unlock()
	call.target = mm
	if reset {
		returned = true
		return nil, Err0RTTRejected
	}
	str, err := mm.OpenStreamSync(ctx)
	str, err = streamMapResult(call, str, err)
	returned = true
	return str, err
}

func (m *streamsMap) OpenUniStream() (*SendStream, error) {
	call, err := m.calls.begin(openUniMapCall, nil)
	if err != nil {
		return nil, err
	}
	returned := false
	defer call.finish(&returned)
	m.mutex.Lock()
	reset := m.reset
	mm := m.outgoingUniStreams
	m.mutex.Unlock()
	call.target = mm
	if reset {
		returned = true
		return nil, Err0RTTRejected
	}
	str, err := mm.OpenStream()
	str, err = streamMapResult(call, str, err)
	returned = true
	return str, err
}

func (m *streamsMap) OpenUniStreamSync(ctx context.Context) (*SendStream, error) {
	call, err := m.calls.begin(openUniSyncMapCall, ctx)
	if err != nil {
		return nil, err
	}
	returned := false
	defer call.finish(&returned)
	m.mutex.Lock()
	reset := m.reset
	mm := m.outgoingUniStreams
	m.mutex.Unlock()
	call.target = mm
	if reset {
		returned = true
		return nil, Err0RTTRejected
	}
	str, err := mm.OpenStreamSync(ctx)
	str, err = streamMapResult(call, str, err)
	returned = true
	return str, err
}

func (m *streamsMap) AcceptStream(ctx context.Context) (*Stream, error) {
	call, err := m.calls.begin(acceptBidiMapCall, ctx)
	if err != nil {
		return nil, err
	}
	returned := false
	defer call.finish(&returned)
	m.mutex.Lock()
	reset := m.reset
	mm := m.incomingBidiStreams
	m.mutex.Unlock()
	call.target = mm
	if reset {
		returned = true
		return nil, Err0RTTRejected
	}
	str, err := mm.AcceptStream(ctx)
	str, err = streamMapResult(call, str, err)
	returned = true
	return str, err
}

func (m *streamsMap) AcceptUniStream(ctx context.Context) (*ReceiveStream, error) {
	call, err := m.calls.begin(acceptUniMapCall, ctx)
	if err != nil {
		return nil, err
	}
	returned := false
	defer call.finish(&returned)
	m.mutex.Lock()
	reset := m.reset
	mm := m.incomingUniStreams
	m.mutex.Unlock()
	call.target = mm
	if reset {
		returned = true
		return nil, Err0RTTRejected
	}
	str, err := mm.AcceptStream(ctx)
	str, err = streamMapResult(call, str, err)
	returned = true
	return str, err
}

func (m *streamsMap) DeleteStream(id protocol.StreamID) error {
	switch id.Type() {
	case protocol.StreamTypeUni:
		if id.InitiatedBy() == m.perspective {
			return m.outgoingUniStreams.DeleteStream(id)
		}
		return m.incomingUniStreams.DeleteStream(id)
	case protocol.StreamTypeBidi:
		if id.InitiatedBy() == m.perspective {
			return m.outgoingBidiStreams.DeleteStream(id)
		}
		return m.incomingBidiStreams.DeleteStream(id)
	}
	panic("")
}

func (m *streamsMap) HandleMaxStreamsFrame(f *wire.MaxStreamsFrame) {
	switch f.Type {
	case protocol.StreamTypeUni:
		m.outgoingUniStreams.SetMaxStream(f.MaxStreamNum.StreamID(protocol.StreamTypeUni, m.perspective))
	case protocol.StreamTypeBidi:
		m.outgoingBidiStreams.SetMaxStream(f.MaxStreamNum.StreamID(protocol.StreamTypeBidi, m.perspective))
	}
}

type sendStreamFrameHandler interface {
	updateSendWindow(protocol.ByteCount)
	handleStopSendingFrame(*wire.StopSendingFrame)
}

func (m *streamsMap) getSendStream(id protocol.StreamID) (sendStreamFrameHandler, error) {
	switch id.Type() {
	case protocol.StreamTypeUni:
		if id.InitiatedBy() != m.perspective {
			// an outgoing unidirectional stream is a send stream, not a receive stream
			return nil, &qerr.TransportError{
				ErrorCode:    qerr.StreamStateError,
				ErrorMessage: fmt.Sprintf("invalid frame for send stream %d", id),
			}
		}
		str, err := m.outgoingUniStreams.GetStream(id)
		if str == nil || err != nil {
			return nil, err
		}
		return str, nil
	case protocol.StreamTypeBidi:
		if id.InitiatedBy() == m.perspective {
			str, err := m.outgoingBidiStreams.GetStream(id)
			if str == nil || err != nil {
				return nil, err
			}
			return str, nil
		}
		str, err := m.incomingBidiStreams.GetOrOpenStream(id)
		if str == nil || err != nil {
			return nil, err
		}
		return str, nil
	}
	panic("unreachable")
}

func (m *streamsMap) HandleMaxStreamDataFrame(f *wire.MaxStreamDataFrame) error {
	str, err := m.getSendStream(f.StreamID)
	if err != nil {
		return err
	}
	if str == nil { // stream already deleted
		return nil
	}
	str.updateSendWindow(f.MaximumStreamData)
	return nil
}

func (m *streamsMap) HandleStopSendingFrame(f *wire.StopSendingFrame) error {
	str, err := m.getSendStream(f.StreamID)
	if err != nil {
		return err
	}
	if str == nil { // stream already deleted
		return nil
	}
	str.handleStopSendingFrame(f)
	return nil
}

type receiveStreamFrameHandler interface {
	handleResetStreamFrame(*wire.ResetStreamFrame, monotime.Time) error
	handleStreamFrame(*receivedStreamData, monotime.Time) error
}

func (m *streamsMap) getReceiveStream(id protocol.StreamID) (receiveStreamFrameHandler, error) {
	switch id.Type() {
	case protocol.StreamTypeUni:
		// an outgoing unidirectional stream is a send stream, not a receive stream
		if id.InitiatedBy() == m.perspective {
			return nil, &qerr.TransportError{
				ErrorCode:    qerr.StreamStateError,
				ErrorMessage: fmt.Sprintf("invalid frame for receive stream %d", id),
			}
		}
		str, err := m.incomingUniStreams.GetOrOpenStream(id)
		if err != nil || str == nil {
			return nil, err
		}
		return str, nil
	case protocol.StreamTypeBidi:
		var str *Stream
		var err error
		if id.InitiatedBy() == m.perspective {
			str, err = m.outgoingBidiStreams.GetStream(id)
		} else {
			str, err = m.incomingBidiStreams.GetOrOpenStream(id)
		}
		if str == nil || err != nil {
			return nil, err
		}
		return str, nil
	}
	panic("unreachable")
}

func (m *streamsMap) HandleStreamDataBlockedFrame(f *wire.StreamDataBlockedFrame) error {
	if _, err := m.getReceiveStream(f.StreamID); err != nil {
		return err
	}
	// We don't need to do anything in response to a STREAM_DATA_BLOCKED frame,
	// but we need to make sure that the stream ID is valid.
	return nil // we don't need to do anything in response to a STREAM_DATA_BLOCKED frame
}

func (m *streamsMap) HandleResetStreamFrame(f *wire.ResetStreamFrame, rcvTime monotime.Time) error {
	str, err := m.getReceiveStream(f.StreamID)
	if err != nil {
		return err
	}
	if str == nil { // stream already deleted
		return nil
	}
	return str.handleResetStreamFrame(f, rcvTime)
}

func (m *streamsMap) HandleStreamFrame(f *receivedStreamData, rcvTime monotime.Time) error {
	if err := m.frameIntake.begin(f); err != nil {
		return err
	}
	returned := false
	defer m.frameIntake.finish(&returned)
	str, err := m.getReceiveStream(f.frameView().StreamID)
	if err != nil {
		returned = true
		return err
	}
	if str == nil { // stream already deleted
		returned = true
		return nil
	}
	err = str.handleStreamFrame(m.frameIntake.transfer(), rcvTime)
	returned = true
	return err
}

func (m *streamsMap) HandleTransportParameters(p *wire.TransportParameters) {
	snapshot := newStreamParameters(p)
	m.appliedParameters = snapshot
	m.generation.outgoingParameters.Store(snapshot)
	if m.receivedParameters == nil {
		// Actual cached restoration applies before early-ready. Fresh received
		// input is published separately and must not be overwritten by this path.
		m.receivedParameters = snapshot
		m.generation.incomingParameters.Store(snapshot)
	}
	if snapshot.resetStreamAt {
		m.outgoingBidiStreams.EnableResetStreamAt()
		m.outgoingUniStreams.EnableResetStreamAt()
	}
	m.outgoingBidiStreams.UpdateSendWindow(snapshot.bidiRemote)
	m.outgoingBidiStreams.SetMaxStream(snapshot.maxBidi.StreamID(protocol.StreamTypeBidi, m.perspective))
	m.outgoingUniStreams.UpdateSendWindow(snapshot.uni)
	m.outgoingUniStreams.SetMaxStream(snapshot.maxUni.StreamID(protocol.StreamTypeUni, m.perspective))
}

func (m *streamsMap) CloseWithError(err error) error {
	m.mutex.Lock()
	original := m.currentMapSet()
	var resetErr error
	if m.resetState.started && (!m.resetState.ended || m.resetState.uncertain) {
		resetErr = errors.Join(errStreamMapUncertain, m.resetState.failure)
	}
	m.mutex.Unlock()
	return errors.Join(original.closeWithError(err), resetErr)
}
