package quic

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go/congestion"
	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/flowcontrol"
	"github.com/sagernet/quic-go/internal/handshake"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/internal/utils/ringbuffer"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/qlog"
	"github.com/sagernet/quic-go/qlogwriter"
)

type unpacker interface {
	UnpackLongHeader(hdr *wire.Header, data []byte) (*unpackedPacket, error)
	UnpackShortHeader(rcvTime monotime.Time, data []byte) (protocol.PacketNumber, protocol.PacketNumberLen, protocol.KeyPhaseBit, []byte, error)
}

type cryptoStreamHandler interface {
	StartHandshake(context.Context) error
	ChangeConnectionID(protocol.ConnectionID)
	SetLargest1RTTAcked(protocol.PacketNumber) error
	SetHandshakeConfirmed()
	GetSessionTicket() ([]byte, error)
	NextEvent() handshake.Event
	DiscardInitialKeys()
	HandleMessage([]byte, protocol.EncryptionLevel) error
	io.Closer
	WaitForShutdown() error
	RetireEventQueue() error
	ConnectionState() handshake.ConnectionState
}

type receivedPacket struct {
	buffer    *packetBuffer
	transport *Transport // assigned by the original Transport reader

	remoteAddr net.Addr
	rcvTime    monotime.Time
	data       []byte

	ecn protocol.ECN

	info packetInfo // only valid if the contained IP address is valid
}

type receivedPacketWithDatagramID struct {
	receivedPacket
	datagramID qlog.DatagramID
}

func (p *receivedPacket) Size() protocol.ByteCount { return protocol.ByteCount(len(p.data)) }

func (p *receivedPacket) Clone() *receivedPacket {
	return &receivedPacket{
		remoteAddr: p.remoteAddr,
		rcvTime:    p.rcvTime,
		data:       p.data,
		buffer:     p.buffer,
		ecn:        p.ecn,
		info:       p.info,
		transport:  p.transport,
	}
}

type connRunner interface {
	Add(protocol.ConnectionID, packetHandler) bool
	Remove(packetHandler, protocol.ConnectionID)
	ReplaceWithClosed(packetHandler, []protocol.ConnectionID, []byte, time.Duration)
	AddResetToken(protocol.StatelessResetToken, packetHandler) bool
	RemoveResetToken(packetHandler, protocol.StatelessResetToken)
}

type closeError struct {
	err       error
	immediate bool
}

type errCloseForRecreating struct {
	nextPacketNumber protocol.PacketNumber
	nextVersion      protocol.Version
}

func (e *errCloseForRecreating) Error() string {
	return "closing connection in order to recreate it"
}

var deadlineSendImmediately = monotime.Time(42 * time.Millisecond) // any value > time.Time{} and before time.Now() is fine

type blockMode uint8

const (
	// blockModeNone means that the connection is not blocked.
	blockModeNone blockMode = iota
	// blockModeCongestionLimited means that the connection is congestion limited.
	// In that case, we can still send acknowledgments and PTO probe packets.
	blockModeCongestionLimited
	// blockModeHardBlocked means that no packet can be sent, under no circumstances. This can happen when:
	// * the send queue is full
	// * the SentPacketHandler returns SendNone, e.g. when we are tracking the maximum number of packets
	// In that case, the timer will be set to the idle timeout.
	blockModeHardBlocked
)

// A Conn is a QUIC connection between two peers.
// Calls to the connection (and to streams) can return the following types of errors:
//   - [ApplicationError]: for errors triggered by the application running on top of QUIC
//   - [TransportError]: for errors triggered by the QUIC transport (in many cases a misbehaving peer)
//   - [IdleTimeoutError]: when the peer goes away unexpectedly (this is a [net.Error] timeout error)
//   - [HandshakeTimeoutError]: when the cryptographic handshake takes too long (this is a [net.Error] timeout error)
//   - [StatelessResetError]: when we receive a stateless reset
//   - [VersionNegotiationError]: returned by the client, when there's no version overlap between the peers
type Conn struct {
	construction    connectionConstruction // original constructor only, before exposure
	congestionCalls connectionCongestionCalls
	// Destination connection ID used during the handshake.
	// Used to check source connection ID on incoming packets.
	handshakeDestConnID protocol.ConnectionID
	// Set for the client. Destination connection ID used on the first Initial sent.
	origDestConnID protocol.ConnectionID
	// Set for the client. Immutable source CID of this attempt's first Initial.
	initialSrcConnID protocol.ConnectionID
	retrySrcConnID   *protocol.ConnectionID // only set for the client (and if a Retry was performed)

	srcConnIDLen int

	perspective protocol.Perspective
	version     protocol.Version
	config      *Config

	conn               sendConn
	initialSendState   connectionSendState
	sendState          atomic.Pointer[connectionSendState]
	sendQueue          sender
	pathSwitch         connectionPathSwitch     // only the original connection loop writes
	clientLifetime     clientConnectionLifetime // initial Transport's original client list
	initialTransport   *Transport               // immutable original runner identity
	pathResponse       pathResponseWrite        // only the original connection loop writes
	serverProbe        serverPathProbe          // original non-active server path output
	activeServerPathID pathID                   // known promoted path, independent of pending path rows

	pathMetadataRetiring bool
	pathMetadataRetired  bool // original loop publishes before packet-input completion

	// lazily initialzed: most connections never migrate
	pathManager           *pathManager
	largestRcvdAppData    protocol.PacketNumber
	largestRcvdNonProbing protocol.PacketNumber // application-data packet number space
	pathManagerOutgoing   atomic.Pointer[pathManagerOutgoing]

	streamsMap      *streamsMap
	connIDManager   *connIDManager
	connIDGenerator *connIDGenerator

	rttStats  *utils.RTTStats
	connStats utils.ConnectionStats

	cryptoStreamManager   *cryptoStreamManager
	sentPacketHandler     ackhandler.SentPacketHandler
	receivedPacketHandler *ackhandler.ReceivedPacketHandler
	retransmissionQueue   *retransmissionQueue
	framer                *framer
	connFlowController    flowcontrol.ConnectionFlowController
	tokenStoreKey         string                    // only set for the client
	tokenGenerator        *handshake.TokenGenerator // only set for the server

	unpacker          unpacker
	frameParser       wire.FrameParser
	streamFrameIntake streamFrameIntake // original serialized parser / trace / map handoff
	receiveFrames     *streamReceiveBudget
	packetOutput      packetSendIntake
	packer            packer
	mtuDiscoverer     mtuDiscoverer // initialized when the transport parameters are received

	currentMTUEstimate atomic.Uint32

	initialStream       *initialCryptoStream
	handshakeStream     *cryptoStream
	oneRTTStream        *cryptoStream // original 1-RTT stream on both client and server
	cryptoStreamHandler cryptoStreamHandler
	cryptoQueues        cryptoQueueRetirement // original external crypto-queue finalizer

	notifyReceivedPacket chan struct{}
	sendingScheduled     chan struct{}
	receivedPacketMx     sync.Mutex
	receivedPackets      ringbuffer.RingBuffer[receivedPacket]
	packetInput          connPacketInputState

	// closeChan is used to notify the run loop that it should terminate
	closeChan chan struct{}
	closeErr  atomic.Pointer[closeError]
	closing   chan struct{} // first close decision, before any send-queue drain

	ctx                   context.Context
	ctxCancel             context.CancelCauseFunc
	handshakeCompleteChan chan struct{}
	handshakeSignal       sync.Once
	handshakeSucceeded    atomic.Bool

	undecryptablePackets          []receivedPacketWithDatagramID // undecryptable packets, waiting for a change in encryption level
	undecryptablePacketsToProcess []receivedPacketWithDatagramID

	earlyConnReadyChan chan struct{}
	sentFirstPacket    bool
	droppedInitialKeys bool
	handshakeComplete  bool
	handshakeConfirmed bool

	receivedRetry       bool
	versionNegotiated   bool
	receivedFirstPacket bool

	blocked blockMode

	// the minimum of the max_idle_timeout values advertised by both endpoints
	idleTimeout  time.Duration
	creationTime monotime.Time
	// The idle timeout is set based on the max of the time we received the last packet...
	lastPacketReceivedTime monotime.Time
	// ... and the time we sent a new ack-eliciting packet after receiving a packet.
	firstAckElicitingPacketAfterIdleSentTime monotime.Time
	// pacingDeadline is the time when the next packet should be sent
	pacingDeadline monotime.Time

	// Native Conn.run only; public readers use immutable scalar heads.
	peerParams         *wire.TransportParameters
	peerObservedParams atomic.Pointer[peerParameterSnapshot]
	peerSendParams     atomic.Pointer[peerParameterSnapshot]

	timer *time.Timer
	// keepAlivePingSent stores whether a keep alive PING is in flight.
	// It is reset as soon as we receive a packet from the peer.
	keepAlivePingSent bool
	keepAliveInterval time.Duration

	datagramQueue *datagramQueue

	connStateMutex sync.Mutex
	connState      ConnectionState

	logID     string
	qlogTrace qlogwriter.Trace
	qlogger   qlogwriter.Recorder
	logger    utils.Logger
}

var _ streamSender = &Conn{}

type connTestHooks struct {
	run                     func() error
	earlyConnReady          func() <-chan struct{}
	context                 func() context.Context
	handshakeComplete       func() <-chan struct{}
	closeWithTransportError func(TransportErrorCode)
	destroy                 func(error)
	handlePacket            func(receivedPacket)
}

type wrappedConn struct {
	testHooks *connTestHooks
	*Conn
}

var newConnection = func(
	ctx context.Context,
	ctxCancel context.CancelCauseFunc,
	conn sendConn,
	runner connRunner,
	origDestConnID protocol.ConnectionID,
	retrySrcConnID *protocol.ConnectionID,
	clientDestConnID protocol.ConnectionID,
	destConnID protocol.ConnectionID,
	srcConnID protocol.ConnectionID,
	connIDGenerator ConnectionIDGenerator,
	statelessResetter *statelessResetter,
	conf *Config,
	tlsConf *tls.Config,
	tokenGenerator *handshake.TokenGenerator,
	clientAddressValidated bool,
	rtt time.Duration,
	qlogTrace qlogwriter.Trace,
	logger utils.Logger,
	v protocol.Version,
	opening *serverConnectionOpening,
) *wrappedConn {
	s := &Conn{
		ctx:                 ctx,
		ctxCancel:           ctxCancel,
		conn:                conn,
		config:              conf,
		handshakeDestConnID: destConnID,
		srcConnIDLen:        srcConnID.Len(),
		tokenGenerator:      tokenGenerator,
		perspective:         protocol.PerspectiveServer,
		qlogTrace:           qlogTrace,
		logger:              logger,
		version:             v,
	}
	if opening != nil {
		opening.connection = s // before the first trace / provider callback
	}
	s.construction = connectionConstruction{
		stage: connectionConstructingCryptoStreams, parent: ctx, tlsConf: tlsConf,
		runner: runner, generator: connIDGenerator,
	}
	s.oneRTTStream = &cryptoStream{}
	s.oneRTTStream.init()
	if original, ok := runner.(*packetHandlerMap); ok {
		s.initialTransport = (*Transport)(original)
	}
	s.initialSendState.conn = conn
	s.sendState.Store(&s.initialSendState)
	s.construction.stage = connectionConstructingTrace
	if qlogTrace != nil {
		s.qlogger = qlogTrace.AddProducer()
	}
	if origDestConnID.Len() > 0 {
		s.logID = origDestConnID.String()
	} else {
		s.logID = destConnID.String()
	}
	s.construction.stage = connectionConstructingIDs
	s.connIDManager = newConnIDManager(
		destConnID,
		func(token protocol.StatelessResetToken) {
			if !s.connIDGenerator.connRunners.AddResetToken(token, s) {
				s.destroyImpl(errCIDRegistration)
			}
		},
		func(token protocol.StatelessResetToken) { s.connIDGenerator.connRunners.RemoveResetToken(s, token) },
		s.queueControlFrame,
	)
	s.connIDGenerator = newConnIDGenerator(
		runner,
		srcConnID,
		&clientDestConnID,
		statelessResetter,
		connRunnerCallbacks{
			AddConnectionID:    func(connID protocol.ConnectionID) bool { return runner.Add(connID, s) },
			RemoveConnectionID: func(connID protocol.ConnectionID) { runner.Remove(s, connID) },
			ReplaceWithClosed: func(ids []protocol.ConnectionID, data []byte, expiry time.Duration) {
				runner.ReplaceWithClosed(s, ids, data, expiry)
			},
		},
		s.queueControlFrame,
		connIDGenerator,
	)
	s.construction.stage = connectionConstructingSetup
	s.preSetup()
	s.rttStats.SetInitialRTT(rtt)
	s.construction.stage = connectionConstructingACK
	s.sentPacketHandler = ackhandler.NewSentPacketHandlerRetained(
		&s.sentPacketHandler,
		0,
		protocol.ByteCount(s.config.InitialPacketSize),
		s.rttStats,
		&s.connStats,
		clientAddressValidated,
		s.conn.capabilities().ECN,
		s.receivedPacketHandler.IgnorePacketsBelow,
		s.perspective,
		s.qlogger,
		s.logger,
		s.closing,
	)
	s.currentMTUEstimate.Store(uint32(estimateMaxPayloadSize(protocol.ByteCount(s.config.InitialPacketSize))))
	s.construction.stage = connectionConstructingParameters
	statelessResetToken := statelessResetter.GetStatelessResetToken(srcConnID)
	params := &wire.TransportParameters{
		InitialMaxStreamDataBidiLocal:   protocol.ByteCount(s.config.InitialStreamReceiveWindow),
		InitialMaxStreamDataBidiRemote:  protocol.ByteCount(s.config.InitialStreamReceiveWindow),
		InitialMaxStreamDataUni:         protocol.ByteCount(s.config.InitialStreamReceiveWindow),
		InitialMaxData:                  protocol.ByteCount(s.config.InitialConnectionReceiveWindow),
		MaxIdleTimeout:                  s.config.MaxIdleTimeout,
		MaxBidiStreamNum:                protocol.StreamNum(s.config.MaxIncomingStreams),
		MaxUniStreamNum:                 protocol.StreamNum(s.config.MaxIncomingUniStreams),
		MaxAckDelay:                     protocol.MaxAckDelayInclGranularity,
		AckDelayExponent:                protocol.AckDelayExponent,
		MaxUDPPayloadSize:               protocol.MaxPacketBufferSize,
		StatelessResetToken:             &statelessResetToken,
		OriginalDestinationConnectionID: origDestConnID,
		// For interoperability with quic-go versions before May 2023, this value must be set to a value
		// different from protocol.DefaultActiveConnectionIDLimit.
		// If set to the default value, it will be omitted from the transport parameters, which will make
		// old quic-go versions interpret it as 0, instead of the default value of 2.
		// See https://github.com/quic-go/quic-go/pull/3806.
		ActiveConnectionIDLimit:   protocol.MaxActiveConnectionIDs,
		InitialSourceConnectionID: srcConnID,
		RetrySourceConnectionID:   retrySrcConnID,
		EnableResetStreamAt:       conf.EnableStreamResetPartialDelivery,
	}
	s.construction.parameters = params
	if s.config.EnableDatagrams {
		params.MaxDatagramFrameSize = wire.MaxDatagramSize
		if s.config.MaxDatagramFrameSize != 0 {
			params.MaxDatagramFrameSize = protocol.ByteCount(s.config.MaxDatagramFrameSize)
		}
	} else {
		params.MaxDatagramFrameSize = protocol.InvalidByteCount
	}
	if s.qlogger != nil {
		s.qlogTransportParameters(params, protocol.PerspectiveServer, false)
	}
	s.construction.stage = connectionConstructingCrypto
	s.construction.localAddr = conn.LocalAddr()
	s.construction.remoteAddr = conn.RemoteAddr()
	cs := handshake.NewCryptoSetupServerRetained(
		&s.construction.crypto,
		clientDestConnID,
		s.construction.localAddr,
		s.construction.remoteAddr,
		params,
		tlsConf,
		conf.Allow0RTT,
		s.rttStats,
		s.qlogger,
		logger,
		s.version,
	)
	s.cryptoStreamHandler = cs
	s.construction.stage = connectionConstructingPacker
	s.packer = newPacketPacker(srcConnID, s.connIDManager.Get, s.initialStream, s.handshakeStream, s.sentPacketHandler, s.retransmissionQueue, cs, s.framer, s.receivedPacketHandler, s.datagramQueue, s.perspective)
	s.unpacker = newPacketUnpacker(cs, s.srcConnIDLen)
	s.cryptoStreamManager = newCryptoStreamManager(s.initialStream, s.handshakeStream, s.oneRTTStream)
	s.finishConstruction()
	return &wrappedConn{Conn: s}
}

// declare this as a variable, such that we can it mock it in the tests
var newClientConnection = func(
	ctx context.Context,
	conn sendConn,
	runner connRunner,
	destConnID protocol.ConnectionID,
	srcConnID protocol.ConnectionID,
	connIDGenerator ConnectionIDGenerator,
	statelessResetter *statelessResetter,
	conf *Config,
	tlsConf *tls.Config,
	initialPacketNumber protocol.PacketNumber,
	enable0RTT bool,
	hasNegotiatedVersion bool,
	qlogTrace qlogwriter.Trace,
	logger utils.Logger,
	v protocol.Version,
	opening *clientConnectionOpening,
) *wrappedConn {
	s := &Conn{
		conn:                conn,
		config:              conf,
		origDestConnID:      destConnID,
		initialSrcConnID:    srcConnID,
		handshakeDestConnID: destConnID,
		srcConnIDLen:        srcConnID.Len(),
		perspective:         protocol.PerspectiveClient,
		logID:               destConnID.String(),
		logger:              logger,
		qlogTrace:           qlogTrace,
		versionNegotiated:   hasNegotiatedVersion,
		version:             v,
	}
	if opening != nil {
		opening.connection = s // before trace, address and token-store callbacks
	}
	s.construction = connectionConstruction{
		stage: connectionConstructingShell, parent: ctx, tlsConf: tlsConf,
		runner: runner, generator: connIDGenerator,
	}
	if original, ok := runner.(*packetHandlerMap); ok {
		s.initialTransport = (*Transport)(original)
	}
	s.initialSendState.conn = conn
	s.sendState.Store(&s.initialSendState)
	s.construction.stage = connectionConstructingTrace
	if qlogTrace != nil {
		s.qlogger = qlogTrace.AddProducer()
	}
	if s.qlogger != nil {
		var srcAddr, destAddr *net.UDPAddr
		s.construction.localAddr = conn.LocalAddr()
		if addr, ok := s.construction.localAddr.(*net.UDPAddr); ok {
			srcAddr = addr
		}
		s.construction.remoteAddr = conn.RemoteAddr()
		if addr, ok := s.construction.remoteAddr.(*net.UDPAddr); ok {
			destAddr = addr
		}
		s.qlogger.RecordEvent(startedConnectionEvent(srcAddr, destAddr))
	}
	s.construction.stage = connectionConstructingIDs
	s.connIDManager = newConnIDManager(
		destConnID,
		func(token protocol.StatelessResetToken) {
			if !s.connIDGenerator.connRunners.AddResetToken(token, s) {
				s.destroyImpl(errCIDRegistration)
			}
		},
		func(token protocol.StatelessResetToken) { s.connIDGenerator.connRunners.RemoveResetToken(s, token) },
		s.queueControlFrame,
	)
	s.connIDGenerator = newConnIDGenerator(
		runner,
		srcConnID,
		nil,
		statelessResetter,
		connRunnerCallbacks{
			AddConnectionID:    func(connID protocol.ConnectionID) bool { return runner.Add(connID, s) },
			RemoveConnectionID: func(connID protocol.ConnectionID) { runner.Remove(s, connID) },
			ReplaceWithClosed: func(ids []protocol.ConnectionID, data []byte, expiry time.Duration) {
				runner.ReplaceWithClosed(s, ids, data, expiry)
			},
		},
		s.queueControlFrame,
		connIDGenerator,
	)
	s.construction.stage = connectionConstructingContext
	s.ctx, s.ctxCancel = context.WithCancelCause(ctx)
	s.construction.stage = connectionConstructingSetup
	s.preSetup()
	s.construction.stage = connectionConstructingACK
	s.sentPacketHandler = ackhandler.NewSentPacketHandlerRetained(
		&s.sentPacketHandler,
		initialPacketNumber,
		protocol.ByteCount(s.config.InitialPacketSize),
		s.rttStats,
		&s.connStats,
		false, // has no effect
		s.conn.capabilities().ECN,
		s.receivedPacketHandler.IgnorePacketsBelow,
		s.perspective,
		s.qlogger,
		s.logger,
		s.closing,
	)
	s.currentMTUEstimate.Store(uint32(estimateMaxPayloadSize(protocol.ByteCount(s.config.InitialPacketSize))))
	s.construction.stage = connectionConstructingCryptoStreams
	s.oneRTTStream = &cryptoStream{}
	s.oneRTTStream.init()
	s.construction.stage = connectionConstructingParameters
	params := &wire.TransportParameters{
		InitialMaxStreamDataBidiRemote: protocol.ByteCount(s.config.InitialStreamReceiveWindow),
		InitialMaxStreamDataBidiLocal:  protocol.ByteCount(s.config.InitialStreamReceiveWindow),
		InitialMaxStreamDataUni:        protocol.ByteCount(s.config.InitialStreamReceiveWindow),
		InitialMaxData:                 protocol.ByteCount(s.config.InitialConnectionReceiveWindow),
		MaxIdleTimeout:                 s.config.MaxIdleTimeout,
		MaxBidiStreamNum:               protocol.StreamNum(s.config.MaxIncomingStreams),
		MaxUniStreamNum:                protocol.StreamNum(s.config.MaxIncomingUniStreams),
		MaxAckDelay:                    protocol.MaxAckDelayInclGranularity,
		MaxUDPPayloadSize:              protocol.MaxPacketBufferSize,
		AckDelayExponent:               protocol.AckDelayExponent,
		// For interoperability with quic-go versions before May 2023, this value must be set to a value
		// different from protocol.DefaultActiveConnectionIDLimit.
		// If set to the default value, it will be omitted from the transport parameters, which will make
		// old quic-go versions interpret it as 0, instead of the default value of 2.
		// See https://github.com/quic-go/quic-go/pull/3806.
		ActiveConnectionIDLimit:   protocol.MaxActiveConnectionIDs,
		InitialSourceConnectionID: srcConnID,
		EnableResetStreamAt:       conf.EnableStreamResetPartialDelivery,
	}
	s.construction.parameters = params
	if s.config.EnableDatagrams {
		params.MaxDatagramFrameSize = wire.MaxDatagramSize
		if s.config.MaxDatagramFrameSize != 0 {
			params.MaxDatagramFrameSize = protocol.ByteCount(s.config.MaxDatagramFrameSize)
		}
	} else {
		params.MaxDatagramFrameSize = protocol.InvalidByteCount
	}
	if s.qlogger != nil {
		s.qlogTransportParameters(params, protocol.PerspectiveClient, false)
	}
	s.construction.stage = connectionConstructingCrypto
	cs := handshake.NewCryptoSetupClientRetained(
		&s.construction.crypto,
		destConnID,
		params,
		tlsConf,
		enable0RTT,
		s.rttStats,
		s.qlogger,
		logger,
		s.version,
	)
	s.cryptoStreamHandler = cs
	s.construction.stage = connectionConstructingPacker
	s.cryptoStreamManager = newCryptoStreamManager(s.initialStream, s.handshakeStream, s.oneRTTStream)
	s.unpacker = newPacketUnpacker(cs, s.srcConnIDLen)
	s.packer = newPacketPacker(srcConnID, s.connIDManager.Get, s.initialStream, s.handshakeStream, s.sentPacketHandler, s.retransmissionQueue, cs, s.framer, s.receivedPacketHandler, s.datagramQueue, s.perspective)
	s.construction.stage = connectionConstructingToken
	if len(tlsConf.ServerName) > 0 {
		s.tokenStoreKey = tlsConf.ServerName
	} else {
		s.construction.remoteAddr = conn.RemoteAddr()
		s.tokenStoreKey = s.construction.remoteAddr.String()
	}
	if s.config.TokenStore != nil {
		s.construction.token = s.config.TokenStore.Pop(s.tokenStoreKey)
		if s.construction.token != nil {
			s.packer.SetToken(s.construction.token.data)
			s.rttStats.SetInitialRTT(s.construction.token.rtt)
		}
	}
	s.finishConstruction()
	return &wrappedConn{Conn: s}
}

func (c *Conn) preSetup() {
	c.packetInput.init()
	c.cryptoQueues.done = make(chan struct{})
	c.closeChan = make(chan struct{}, 1)
	c.closing = make(chan struct{})
	c.congestionCalls.init(&c.closeErr, c.closing)
	c.receiveFrames = newStreamReceiveBudget(c.closeLocal, c.closing)
	c.streamFrameIntake.budget = c.receiveFrames
	c.largestRcvdAppData = protocol.InvalidPacketNumber
	c.largestRcvdNonProbing = protocol.InvalidPacketNumber
	c.activeServerPathID = invalidPathID
	c.initialStream = &initialCryptoStream{}
	c.initialStream.init(c.perspective == protocol.PerspectiveClient)
	c.handshakeStream = &cryptoStream{}
	c.handshakeStream.init()
	c.sendQueue = newSendQueue(c.conn)
	c.retransmissionQueue = newRetransmissionQueue()
	c.frameParser = *wire.NewFrameParser(
		c.config.EnableDatagrams,
		c.config.EnableStreamResetPartialDelivery,
		false, // ACK_FREQUENCY is not supported yet
	)
	c.rttStats = utils.NewRTTStats()
	c.connFlowController = flowcontrol.NewConnectionFlowController(
		protocol.ByteCount(c.config.InitialConnectionReceiveWindow),
		protocol.ByteCount(c.config.MaxConnectionReceiveWindow),
		func(size protocol.ByteCount) bool {
			if c.config.AllowConnectionWindowIncrease == nil {
				return true
			}
			return c.config.AllowConnectionWindowIncrease(c, uint64(size))
		},
		c.rttStats,
		c.logger,
	)
	c.earlyConnReadyChan = make(chan struct{})
	c.framer = newFramer(c.connFlowController)
	c.streamsMap = &streamsMap{}
	c.streamsMap.init(
		c.ctx,
		c,
		c.queueControlFrame,
		c.newFlowController,
		uint64(c.config.MaxIncomingStreams),
		uint64(c.config.MaxIncomingUniStreams),
		c.perspective,
		c.receiveFrames,
		c.framer,
		c.closeLocal,
	)
	c.streamsMap.calls.closeState = &c.closeErr
	c.streamsMap.calls.onFailure = func() { c.destroyImpl(errStreamMapUncertain) }
	c.framer.streamGeneration.Store(c.streamsMap.generation) // before public exposure
	c.receiveFrames.calls.framer = c.framer                  // immutable original binding before public exposure
	c.receivedPackets.Init(8)
	c.notifyReceivedPacket = make(chan struct{}, 1)
	c.sendingScheduled = make(chan struct{}, 1)
	c.handshakeCompleteChan = make(chan struct{})

	now := monotime.Now()
	c.lastPacketReceivedTime = now
	c.creationTime = now

	c.receivedPacketHandler = ackhandler.NewReceivedPacketHandler(c.logger, c.closing)

	c.datagramQueue = newDatagramQueue(c.scheduleSending, c.logger)
	c.datagramQueue.peerParams = &c.peerSendParams
	c.connState.Version = c.version
}

// run the connection main loop
func (c *Conn) run() (err error) {
	defer c.signalHandshakeComplete() // terminal failures also wake public waiters
	defer func() { c.ctxCancel(err) }()
	receiveLoopReturned := false
	receiveDrainReturned := false
	sendQueueCloseAttempted := false
	sendQueueCloseReturned := false
	defer func() {
		returned := receiveLoopReturned && receiveDrainReturned && sendQueueCloseReturned && !c.pathSwitch.active && !c.pathResponse.active && !c.serverProbe.active && (c.pathManager == nil || c.pathManager.retiring == nil) && c.packetPackerQuiescent() && c.packetOutput.original == nil && !c.packetOutput.failed && c.sentPacketHandler.PacketStateError() == nil
		defer func() {
			// Publish a negative result even if metadata retirement unwinds
			// abnormally. No positive done signal precedes the actual cleanup.
			c.packetInput.finishLoop(&returned)
			if !returned {
				c.packetOutput.failed = true
				err = errors.Join(err, errPacketInputUncertain)
			}
			c.streamsMap.calls.finishLoop(&returned)
			c.receiveFrames.calls.finishLoop(&returned)
		}()
		if returned {
			returned = false // a panic/Goexit in cleanup cannot leave this positive
			returned = c.retirePathMetadata()
		}
	}()

	defer c.drainPacketInputs(&receiveDrainReturned)
	defer func() {
		if !sendQueueCloseAttempted {
			sendQueueCloseAttempted = true
			retired, queueErr := c.sendQueue.Close()
			sendQueueCloseReturned = retired
			if !retired {
				c.receiveFrames.markUncertain()
				if queueErr == nil {
					queueErr = errPacketOutputUncertain
				}
			}
			err = queueCloseError(err, queueErr)
		}
	}()
	defer c.packetInput.seal() // before a terminal send-queue wait, including early error
	go c.runSendQueue(c.sendQueue)

	c.timer = time.NewTimer(monotime.Until(c.idleTimeoutStartTime().Add(c.config.HandshakeIdleTimeout)))
	defer c.timer.Stop()

	if c.closeErr.Load() == nil {
		if cause := context.Cause(c.ctx); cause != nil {
			c.destroyImpl(cause)
		} else if err := c.cryptoStreamHandler.StartHandshake(c.ctx); err != nil {
			c.destroyImpl(err)
		} else if err := c.handleHandshakeEvents(monotime.Now()); err != nil {
			c.destroyImpl(err)
		}
	}
	if c.perspective == protocol.PerspectiveClient && c.closeErr.Load() == nil {
		c.scheduleSending() // so the ClientHello actually gets sent
	}

	var sendQueueAvailable <-chan struct{}

runLoop:
	for {
		if c.framer.QueuedTooManyControlFrames() {
			c.setCloseError(&closeError{err: &qerr.TransportError{ErrorCode: InternalError}})
			break runLoop
		}
		// Close immediately if requested
		select {
		case <-c.closeChan:
			break runLoop
		case <-c.ctx.Done():
			c.destroyImpl(context.Cause(c.ctx))
			break runLoop
		default:
		}

		// no need to set a timer if we can send packets immediately
		if c.pacingDeadline != deadlineSendImmediately {
			c.maybeResetTimer()
		}

		// 1st: handle undecryptable packets, if any.
		// This can only occur before completion of the handshake.
		if len(c.undecryptablePacketsToProcess) > 0 {
			var processedUndecryptablePacket bool
			c.packetInput.replay = c.undecryptablePacketsToProcess
			c.packetInput.replayPos = 0
			c.undecryptablePacketsToProcess = nil
			for i := range c.packetInput.replay {
				p := c.packetInput.replay[i]
				c.packetInput.delivery = p.receivedPacket
				c.packetInput.replay[i] = receivedPacketWithDatagramID{}
				c.packetInput.replayPos = i + 1
				processed, err := c.handleOnePacket(p.receivedPacket, p.datagramID)
				if err != nil {
					c.setCloseError(&closeError{err: err})
					break runLoop
				}
				if processed {
					processedUndecryptablePacket = true
				}
			}
			c.packetInput.replay, c.packetInput.replayPos = nil, 0
			if processedUndecryptablePacket {
				// if we processed any undecryptable packets, jump to the resetting of the timers directly
				continue
			}
		}

		// 2nd: receive packets.
		processed, err := c.handlePackets() // don't check receivedPackets.Len() in the run loop to avoid locking the mutex
		if err != nil {
			c.setCloseError(&closeError{err: err})
			break runLoop
		}

		// We don't need to wait for new events if:
		// * we processed packets: we probably need to send an ACK, and potentially more data
		// * the pacer allows us to send more packets immediately
		shouldProceedImmediately := sendQueueAvailable == nil && (processed || c.pacingDeadline.Equal(deadlineSendImmediately))
		if !shouldProceedImmediately {
			// 3rd: wait for something to happen:
			// * closing of the connection
			// * timer firing
			// * sending scheduled
			// * send queue available
			// * received packets
			select {
			case <-c.closeChan:
				break runLoop
			case <-c.ctx.Done():
				c.destroyImpl(context.Cause(c.ctx))
				break runLoop
			case <-c.timer.C:
			case <-c.sendingScheduled:
			case <-sendQueueAvailable:
			case <-c.notifyReceivedPacket:
				wasProcessed, err := c.handlePackets()
				if err != nil {
					c.setCloseError(&closeError{err: err})
					break runLoop
				}
				// if we processed any undecryptable packets, jump to the resetting of the timers directly
				if !wasProcessed {
					continue
				}
			}
		}

		// Check for loss detection timeout.
		// This could cause packets to be declared lost, and retransmissions to be enqueued.
		now := monotime.Now()
		if timeout := c.sentPacketHandler.GetLossDetectionTimeout(); !timeout.IsZero() && !timeout.After(now) {
			if err := c.sentPacketHandler.OnLossDetectionTimeout(now); err != nil {
				c.setCloseError(&closeError{err: err})
				break runLoop
			}
		}
		if err := c.advanceServerPathValidation(now); err != nil {
			c.setCloseError(&closeError{err: err})
			break runLoop
		}

		if keepAliveTime := c.nextKeepAliveTime(); !keepAliveTime.IsZero() && !now.Before(keepAliveTime) {
			// send a PING frame since there is no activity in the connection
			c.logger.Debugf("Sending a keep-alive PING to keep the connection alive.")
			c.framer.QueueControlFrame(&wire.PingFrame{})
			c.keepAlivePingSent = true
		} else if !c.handshakeComplete && now.Sub(c.creationTime) >= c.config.handshakeTimeout() {
			c.destroyImpl(qerr.ErrHandshakeTimeout)
			break runLoop
		} else {
			idleTimeoutStartTime := c.idleTimeoutStartTime()
			if (!c.handshakeComplete && now.Sub(idleTimeoutStartTime) >= c.config.HandshakeIdleTimeout) ||
				(c.handshakeComplete && !now.Before(c.nextIdleTimeoutTime())) {
				c.destroyImpl(qerr.ErrIdleTimeout)
				break runLoop
			}
		}

		c.connIDGenerator.RemoveRetiredConnIDs(now)

		if c.perspective == protocol.PerspectiveClient {
			pm := c.pathManagerOutgoing.Load()
			if pm != nil {
				tr, ok := pm.ShouldSwitchPath()
				if ok {
					c.switchToNewPath(tr, now)
				}
			}
		}

		if c.sendQueue.WouldBlock() {
			// The send queue is still busy sending out packets. Wait until there's space to enqueue new packets.
			sendQueueAvailable = c.sendQueue.Available()
			// Cancel the pacing timer, as we can't send any more packets until the send queue is available again.
			c.pacingDeadline = 0
			c.blocked = blockModeHardBlocked
			continue
		}

		if c.closeErr.Load() != nil {
			break runLoop
		}

		c.blocked = blockModeNone // sending might set it back to true if we're congestion limited
		if err := c.triggerSending(now); err != nil {
			c.setCloseError(&closeError{err: err})
			break runLoop
		}
		if c.sendQueue.WouldBlock() {
			// The send queue is still busy sending out packets. Wait until there's space to enqueue new packets.
			sendQueueAvailable = c.sendQueue.Available()
			// Cancel the pacing timer, as we can't send any more packets until the send queue is available again.
			c.pacingDeadline = 0
			c.blocked = blockModeHardBlocked
		} else {
			sendQueueAvailable = nil
		}
	}

	// The published close record is immutable. Terminal error normalization is
	// private to this original loop; Dial/path readiness can observe the original.
	localClose := *c.closeErr.Load()
	closeErr := &localClose
	if cryptoErr := c.cryptoStreamHandler.Close(); cryptoErr != nil {
		closeErr.err = errors.Join(closeErr.err, cryptoErr)
		closeErr.immediate = true
	}
	sendQueueCloseAttempted = true
	retired, queueErr := c.sendQueue.Close()
	sendQueueCloseReturned = retired
	if !retired {
		c.receiveFrames.markUncertain()
		if queueErr == nil {
			queueErr = errPacketOutputUncertain
		}
	}
	if queueErr != nil {
		localClose := *closeErr
		localClose.err = queueCloseError(localClose.err, queueErr)
		localClose.immediate = true // no new terminal write after failed queue custody
		closeErr = &localClose
	}
	shutdownErr := c.handleCloseError(closeErr)
	inputErr := c.waitPacketIntakeCalls()
	if inputErr != nil {
		c.packetOutput.failed = true
		localClose := *closeErr
		localClose.err = errors.Join(localClose.err, inputErr)
		closeErr = &localClose
	}
	if c.qlogger != nil && inputErr == nil {
		// Each attempt creates its own producer, including version recreation.
		// No successor owns this recorder or can retire it on our behalf.
		if traceErr := c.qlogger.Close(); traceErr != nil {
			c.packetOutput.failed = true
			localClose := *closeErr
			localClose.err = errors.Join(localClose.err, traceErr)
			closeErr = &localClose
		}
	}
	c.logger.Infof("Connection %s closed.", c.logID)
	receiveLoopReturned = shutdownErr == nil
	return closeErr.err
}

// blocks until the early connection can be used
func (c *Conn) earlyConnReady() <-chan struct{} {
	return c.earlyConnReadyChan
}

// Context returns a context that is cancelled when the connection is closed.
// The cancellation cause is set to the error that caused the connection to close.
func (c *Conn) Context() context.Context {
	return c.ctx
}

// A readiness notification is only an admission point. A close request that
// already won must not be published as a usable connection by Dial or Accept.
func (c *Conn) readinessError() error {
	if closeErr := c.closeErr.Load(); closeErr != nil {
		if closeErr.err != nil {
			return closeErr.err
		}
		return net.ErrClosed
	}
	return context.Cause(c.ctx)
}

// Closing is closed when the first connection close reason is recorded, before
// shutdown waits for the send queue. An owner may use it to start an independent
// raw-socket abort. It is not a task, packet-holder or native retirement receipt;
// Context and the original Close return keep their existing separate contracts.
func (c *Conn) Closing() <-chan struct{} { return c.closing }

// ConnectionState returns basic details about the QUIC connection.
func (c *Conn) ConnectionState() ConnectionState {
	c.connStateMutex.Lock()
	defer c.connStateMutex.Unlock()

	cs := c.cryptoStreamHandler.ConnectionState()
	c.connState.TLS = cs.ConnectionState
	c.connState.Used0RTT = cs.Used0RTT
	params := c.peerObservedParams.Load()
	c.connState.SupportsDatagrams.Remote = params != nil && params.maxDatagramFrameSize > 0
	c.connState.SupportsStreamResetPartialDelivery.Remote = params != nil && params.enableResetStreamAt
	c.connState.SupportsDatagrams.Local = c.config.EnableDatagrams
	c.connState.SupportsStreamResetPartialDelivery.Local = c.config.EnableStreamResetPartialDelivery
	c.connState.GSO = c.currentSendConn().capabilities().GSO
	return c.connState
}

// ConnectionStats contains statistics about the QUIC connection
type ConnectionStats struct {
	// MinRTT is the estimate of the minimum RTT observed on the active network
	// path.
	MinRTT time.Duration
	// LatestRTT is the last RTT sample observed on the active network path.
	LatestRTT time.Duration
	// SmoothedRTT is an exponentially weighted moving average of an endpoint's
	// RTT samples. See https://www.rfc-editor.org/rfc/rfc9002#section-5.3
	SmoothedRTT time.Duration
	// MeanDeviation estimates the variation in the RTT samples using a mean
	// variation. See https://www.rfc-editor.org/rfc/rfc9002#section-5.3
	MeanDeviation time.Duration

	// BytesSent is the number of bytes sent on the underlying connection,
	// including retransmissions. Does not include UDP or any other outer
	// framing.
	BytesSent uint64
	// PacketsSent is the number of packets sent on the underlying connection,
	// including those that are determined to have been lost.
	PacketsSent uint64
	// BytesReceived is the number of total bytes received on the underlying
	// connection, including duplicate data for streams. Does not include UDP or
	// any other outer framing.
	BytesReceived uint64
	// PacketsReceived is the number of total packets received on the underlying
	// connection, including packets that were not processable.
	PacketsReceived uint64
	// BytesLost is the number of bytes lost on the underlying connection (does
	// not monotonically increase, because packets that are declared lost can
	// subsequently be received). Does not include UDP or any other outer
	// framing.
	BytesLost uint64
	// PacketsLost is the number of packets lost on the underlying connection
	// (does not monotonically increase, because packets that are declared lost
	// can subsequently be received).
	PacketsLost uint64
}

func (c *Conn) ConnectionStats() ConnectionStats {
	return ConnectionStats{
		MinRTT:        c.rttStats.MinRTT(),
		LatestRTT:     c.rttStats.LatestRTT(),
		SmoothedRTT:   c.rttStats.SmoothedRTT(),
		MeanDeviation: c.rttStats.MeanDeviation(),

		BytesSent:       c.connStats.BytesSent.Load(),
		PacketsSent:     c.connStats.PacketsSent.Load(),
		BytesReceived:   c.connStats.BytesReceived.Load(),
		PacketsReceived: c.connStats.PacketsReceived.Load(),
		BytesLost:       c.connStats.BytesLost.Load(),
		PacketsLost:     c.connStats.PacketsLost.Load(),
	}
}

// Time when the connection should time out
func (c *Conn) nextIdleTimeoutTime() monotime.Time {
	idleTimeout := max(c.idleTimeout, c.rttStats.PTO(true)*3)
	return c.idleTimeoutStartTime().Add(idleTimeout)
}

// Time when the next keep-alive packet should be sent.
// It returns a zero time if no keep-alive should be sent.
func (c *Conn) nextKeepAliveTime() monotime.Time {
	if c.config.KeepAlivePeriod == 0 || c.keepAlivePingSent {
		return 0
	}
	keepAliveInterval := max(c.keepAliveInterval, c.rttStats.PTO(true)*3/2)
	return c.lastPacketReceivedTime.Add(keepAliveInterval)
}

func (c *Conn) maybeResetTimer() {
	var deadline monotime.Time
	if !c.handshakeComplete {
		deadline = c.creationTime.Add(c.config.handshakeTimeout())
		if t := c.idleTimeoutStartTime().Add(c.config.HandshakeIdleTimeout); t.Before(deadline) {
			deadline = t
		}
	} else {
		// A keep-alive packet is ack-eliciting, so it can only be sent if the connection is
		// neither congestion limited nor hard-blocked.
		if c.blocked != blockModeNone {
			deadline = c.nextIdleTimeoutTime()
		} else {
			if keepAliveTime := c.nextKeepAliveTime(); !keepAliveTime.IsZero() {
				deadline = keepAliveTime
			} else {
				deadline = c.nextIdleTimeoutTime()
			}
		}
	}
	// Path expiry is metadata work and must still run under output pressure.
	// A due retry only marks one pending attempt; it cannot leave a busy alarm.
	if t := c.nextServerPathValidationDeadline(); !t.IsZero() && t.Before(deadline) {
		deadline = t
	}
	// If the connection is hard-blocked, we can't even send acknowledgments,
	// nor can we send PTO probe packets.
	if c.blocked == blockModeHardBlocked {
		c.timer.Reset(monotime.Until(deadline))
		return
	}

	if t := c.receivedPacketHandler.GetAlarmTimeout(); !t.IsZero() && t.Before(deadline) {
		deadline = t
	}
	if t := c.sentPacketHandler.GetLossDetectionTimeout(); !t.IsZero() && t.Before(deadline) {
		deadline = t
	}
	if c.blocked == blockModeCongestionLimited {
		c.timer.Reset(monotime.Until(deadline))
		return
	}

	if !c.pacingDeadline.IsZero() && c.pacingDeadline.Before(deadline) {
		deadline = c.pacingDeadline
	}
	c.timer.Reset(monotime.Until(deadline))
}

func (c *Conn) idleTimeoutStartTime() monotime.Time {
	startTime := c.lastPacketReceivedTime
	if t := c.firstAckElicitingPacketAfterIdleSentTime; !t.IsZero() && t.After(startTime) {
		startTime = t
	}
	return startTime
}

func (c *Conn) handleHandshakeComplete(now monotime.Time) error {
	defer c.signalHandshakeComplete()
	// Once the handshake completes, we have derived 1-RTT keys.
	// There's no point in queueing undecryptable packets for later decryption anymore.
	discardQueuedPacketInputs(&c.undecryptablePackets)

	c.connIDManager.SetHandshakeComplete()
	c.connIDGenerator.SetHandshakeComplete(now.Add(3 * c.rttStats.PTO(false)))

	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.ALPNInformation{
			ChosenALPN: c.cryptoStreamHandler.ConnectionState().NegotiatedProtocol,
		})
	}

	// The server applies transport parameters right away, but the client side has to wait for handshake completion.
	// During a 0-RTT connection, the client is only allowed to use the new transport parameters for 1-RTT packets.
	if c.perspective == protocol.PerspectiveClient {
		if err := c.applyTransportParameters(); err != nil {
			return err
		}
		return c.publishHandshakeComplete()
	}

	// All these only apply to the server side.
	if err := c.handleHandshakeConfirmed(now); err != nil {
		return err
	}

	ticket, err := c.cryptoStreamHandler.GetSessionTicket()
	if err != nil {
		return err
	}
	if ticket != nil { // may be nil if session tickets are disabled via tls.Config.SessionTicketsDisabled
		c.oneRTTStream.Write(ticket)
		for c.oneRTTStream.HasData() {
			if cf := c.oneRTTStream.PopCryptoFrame(protocol.MaxPostHandshakeCryptoFrameSize); cf != nil {
				c.queueControlFrame(cf)
			}
		}
	}
	token, err := c.tokenGenerator.NewToken(c.conn.RemoteAddr(), c.rttStats.SmoothedRTT())
	if err != nil {
		return err
	}
	c.queueControlFrame(&wire.NewTokenFrame{Token: token})
	c.queueControlFrame(&wire.HandshakeDoneFrame{})
	return c.publishHandshakeComplete()
}

func (c *Conn) publishHandshakeComplete() error {
	if err := c.readinessError(); err != nil {
		return err
	}
	c.handshakeSucceeded.Store(true)
	return nil
}

func (c *Conn) signalHandshakeComplete() {
	c.handshakeSignal.Do(func() { close(c.handshakeCompleteChan) })
}

// Admission follows the actual send connection, including a prospective path.
// The socket's capability observation retains its existing family/path limits.
func (c *Conn) pathMTUDiscoveryEnabled(conn sendConn) bool {
	return c.handshakeConfirmed && !c.config.DisablePathMTUDiscovery && conn.capabilities().DF
}

func (c *Conn) handleHandshakeConfirmed(now monotime.Time) error {
	// Drop initial keys.
	// On the client side, this should have happened when sending the first Handshake packet,
	// but this is not guaranteed if the server misbehaves.
	// See CVE-2025-59530 for more details.
	if err := c.dropEncryptionLevel(protocol.EncryptionInitial, now); err != nil {
		return err
	}
	if err := c.dropEncryptionLevel(protocol.EncryptionHandshake, now); err != nil {
		return err
	}

	c.handshakeConfirmed = true
	c.cryptoStreamHandler.SetHandshakeConfirmed()

	if c.pathMTUDiscoveryEnabled(c.conn) {
		c.mtuDiscoverer.Start(now)
	}
	return nil
}

const maxPacketsToProcess = 32

func (c *Conn) handlePackets() (wasProcessed bool, _ error) {
	// Process packets from the receivedPackets queue.
	// Limit the number of packets to process to maxPacketsToProcess,
	// so we eventually get a chance to send out an ACK when receiving a lot of packets.
	c.receivedPacketMx.Lock()

	if c.receivedPackets.Empty() {
		c.receivedPacketMx.Unlock()
		return false, nil
	}

	var hasMorePackets bool
	for range maxPacketsToProcess {
		p := c.receivedPackets.PopFront()
		c.packetInput.delivery = p // before datagram-ID/trace/parser callbacks
		c.receivedPacketMx.Unlock()

		var datagramID qlog.DatagramID
		if c.qlogger != nil && len(p.data) > 0 && wire.IsLongHeaderPacket(p.data[0]) {
			datagramID = qlog.CalculateDatagramID(p.data)
		}
		processed, err := c.handleOnePacket(p, datagramID)
		if err != nil {
			return false, err
		}
		if processed {
			wasProcessed = true
		}
		c.receivedPacketMx.Lock()
		hasMorePackets = !c.receivedPackets.Empty()
		if !hasMorePackets {
			break
		}
		// Prioritize sending of new CRYPTO data.
		// This is especially relevant when processing 0-RTT packets.
		if !c.handshakeComplete && (c.initialStream.HasData() || c.handshakeStream.HasData()) {
			break
		}
	}
	c.receivedPacketMx.Unlock()

	if hasMorePackets {
		select {
		case c.notifyReceivedPacket <- struct{}{}:
		default:
		}
	}
	return wasProcessed, nil
}

func (c *Conn) handleOnePacketImpl(rp receivedPacket, datagramID qlog.DatagramID) (wasProcessed bool, _ error) {
	c.sentPacketHandler.ReceivedBytes(rp.Size(), rp.rcvTime)

	if wire.IsVersionNegotiationPacket(rp.data) {
		return false, c.handleVersionNegotiationPacket(rp)
	}

	var counter int
	var lastConnID protocol.ConnectionID
	data := rp.data
	p := rp
	for len(data) > 0 {
		if counter > 0 {
			p = rp // the old Clone was a shallow copy of the same original views
			p.data = data

			destConnID, err := wire.ParseConnectionID(p.data, c.srcConnIDLen)
			if err != nil {
				if c.qlogger != nil {
					c.qlogger.RecordEvent(qlog.PacketDropped{
						Raw:        qlog.RawInfo{Length: len(data)},
						DatagramID: datagramID,
						Trigger:    qlog.PacketDropHeaderParseError,
					})
				}
				c.logger.Debugf("error parsing packet, couldn't parse connection ID: %s", err)
				break
			}
			if destConnID != lastConnID {
				if c.qlogger != nil {
					c.qlogger.RecordEvent(qlog.PacketDropped{
						Header:     qlog.PacketHeader{DestConnectionID: destConnID},
						Raw:        qlog.RawInfo{Length: len(data)},
						DatagramID: datagramID,
						Trigger:    qlog.PacketDropUnknownConnectionID,
					})
				}
				c.logger.Debugf("coalesced packet has different destination connection ID: %s, expected %s", destConnID, lastConnID)
				break
			}
		}

		if wire.IsLongHeaderPacket(p.data[0]) {
			hdr, packetData, rest, err := wire.ParsePacket(p.data)
			if err != nil {
				if c.qlogger != nil {
					if err == wire.ErrUnsupportedVersion {
						c.qlogger.RecordEvent(qlog.PacketDropped{
							Header:     qlog.PacketHeader{Version: hdr.Version},
							Raw:        qlog.RawInfo{Length: len(data)},
							DatagramID: datagramID,
							Trigger:    qlog.PacketDropUnsupportedVersion,
						})
					} else {
						c.qlogger.RecordEvent(qlog.PacketDropped{
							Raw:        qlog.RawInfo{Length: len(data)},
							DatagramID: datagramID,
							Trigger:    qlog.PacketDropHeaderParseError,
						})
					}
				}
				c.logger.Debugf("error parsing packet: %s", err)
				break
			}
			lastConnID = hdr.DestConnectionID

			if hdr.Version != c.version {
				if c.qlogger != nil {
					c.qlogger.RecordEvent(qlog.PacketDropped{
						Raw:        qlog.RawInfo{Length: len(data)},
						DatagramID: datagramID,
						Trigger:    qlog.PacketDropUnexpectedVersion,
					})
				}
				c.logger.Debugf("Dropping packet with version %x. Expected %x.", hdr.Version, c.version)
				break
			}

			if counter > 0 {
				p.buffer.Split()
				c.packetInput.processing.working = true
			}
			counter++

			// only log if this actually a coalesced packet
			if c.logger.Debug() && (counter > 1 || len(rest) > 0) {
				c.logger.Debugf("Parsed a coalesced packet. Part %d: %d bytes. Remaining: %d bytes.", counter, len(packetData), len(rest))
			}

			p.data = packetData

			c.packetInput.processing.working = false // original working ref enters callee
			processed, err := c.handleLongHeaderPacket(p, hdr, datagramID)
			if err != nil {
				return false, err
			}
			if processed {
				wasProcessed = true
			}
			data = rest
		} else {
			if counter > 0 {
				p.buffer.Split()
				c.packetInput.processing.working = true
			}
			c.packetInput.processing.working = false
			processed, err := c.handleShortHeaderPacket(p, counter > 0, datagramID)
			if err != nil {
				return false, err
			}
			if processed {
				wasProcessed = true
			}
			break
		}
	}

	c.blocked = blockModeNone
	return wasProcessed, nil
}

func (c *Conn) handleShortHeaderPacket(
	p receivedPacket,
	isCoalesced bool,
	datagramID qlog.DatagramID, // only for logging
) (wasProcessed bool, _ error) {
	var wasQueued bool

	defer func() {
		// Put back the packet buffer if the packet wasn't queued for later decryption.
		if !wasQueued {
			p.buffer.Decrement()
		}
	}()

	destConnID, err := wire.ParseConnectionID(p.data, c.srcConnIDLen)
	if err != nil {
		if c.qlogger != nil {
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:   qlog.PacketType1RTT,
					PacketNumber: protocol.InvalidPacketNumber,
				},
				Raw:        qlog.RawInfo{Length: len(p.data)},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropHeaderParseError,
			})
		}
		return false, nil
	}
	pn, pnLen, keyPhase, data, err := c.unpacker.UnpackShortHeader(p.rcvTime, p.data)
	if err != nil {
		// Stateless reset packets (see RFC 9000, section 10.3):
		// * fill the entire UDP datagram (i.e. they cannot be part of a coalesced packet)
		// * are short header packets (first bit is 0)
		// * have the QUIC bit set (second bit is 1)
		// * are at least 21 bytes long
		if !isCoalesced && len(p.data) >= protocol.MinReceivedStatelessResetSize && p.data[0]&0b11000000 == 0b01000000 {
			token := protocol.StatelessResetToken(p.data[len(p.data)-16:])
			if c.connIDManager.IsActiveStatelessResetToken(token) {
				return false, &StatelessResetError{}
			}
		}
		wasQueued, err = c.handleUnpackError(err, p, qlog.PacketType1RTT, datagramID)
		return false, err
	}
	c.largestRcvdAppData = max(c.largestRcvdAppData, pn)

	if c.logger.Debug() {
		c.logger.Debugf("<- Reading packet %d (%d bytes) for connection %s, 1-RTT", pn, p.Size(), destConnID)
		wire.LogShortHeader(c.logger, destConnID, pn, pnLen, keyPhase)
	}

	if c.receivedPacketHandler.IsPotentiallyDuplicate(pn, protocol.Encryption1RTT) {
		c.logger.Debugf("Dropping (potentially) duplicate packet.")
		if c.qlogger != nil {
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:   qlog.PacketType1RTT,
					PacketNumber: pn,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropDuplicate,
			})
		}
		return false, nil
	}

	if err := c.prepareServerPathInput(p); err != nil {
		return false, err
	}

	var log func([]qlog.Frame)
	if c.qlogger != nil {
		log = func(frames []qlog.Frame) {
			c.qlogger.RecordEvent(qlog.PacketReceived{
				Header: qlog.PacketHeader{
					PacketType:       qlog.PacketType1RTT,
					DestConnectionID: destConnID,
					PacketNumber:     pn,
					KeyPhaseBit:      keyPhase,
				},
				Raw: qlog.RawInfo{
					Length:        int(p.Size()),
					PayloadLength: int(p.Size() - wire.ShortHeaderLen(destConnID, pnLen)),
				},
				DatagramID: datagramID,
				Frames:     frames,
				ECN:        toQlogECN(p.ecn),
			})
		}
	}
	isNonProbing, _, err := c.handleUnpackedShortHeaderPacket(destConnID, pn, data, p.ecn, p.rcvTime, log)
	if err != nil {
		return false, err
	}

	// In RFC 9000, only the client can migrate between paths.
	if c.perspective == protocol.PerspectiveClient {
		return true, nil
	}
	if isNonProbing {
		c.largestRcvdNonProbing = max(c.largestRcvdNonProbing, pn)
	}
	if addrsEqual(p.remoteAddr, c.RemoteAddr()) {
		return true, nil
	}
	if c.config.DisablePathManager {
		// for hysteria2 port hopping, direct change remote address without connection migration logic
		c.conn.ChangeRemoteAddr(p.remoteAddr, p.info)
		return true, nil
	}

	owner := c.packetInput.processing
	if owner == nil || owner.serverPath == nil {
		return true, nil // bounded path admission refused; never invent a reply route
	}
	path := owner.serverPath
	// We only switch paths in response to the highest-numbered non-probing packet,
	// see section 9.3 of RFC 9000.
	if !path.observeMigrationPacket(isNonProbing, pn) || path.largestNonProbing != c.largestRcvdNonProbing {
		return true, nil
	}
	c.pathManager.SwitchToPath(path)
	if err := c.sentPacketHandler.MigratedPath(p.rcvTime, protocol.ByteCount(c.config.InitialPacketSize)); err != nil {
		return true, err
	}
	maxPacketSize := protocol.ByteCount(protocol.MaxPacketBufferSize)
	if c.peerParams.MaxUDPPayloadSize > 0 && c.peerParams.MaxUDPPayloadSize < maxPacketSize {
		maxPacketSize = c.peerParams.MaxUDPPayloadSize
	}
	c.mtuDiscoverer.Reset(
		p.rcvTime,
		protocol.ByteCount(c.config.InitialPacketSize),
		maxPacketSize,
		c.pathMTUDiscoveryEnabled(c.conn),
	)
	c.conn.ChangeRemoteAddr(p.remoteAddr, p.info)
	c.activeServerPathID = path.id // only after the original promotion returned
	return true, nil
}

func (c *Conn) handleLongHeaderPacket(p receivedPacket, hdr *wire.Header, datagramID qlog.DatagramID) (wasProcessed bool, _ error) {
	var wasQueued bool

	defer func() {
		// Put back the packet buffer if the packet wasn't queued for later decryption.
		if !wasQueued {
			p.buffer.Decrement()
		}
	}()

	if hdr.Type == protocol.PacketTypeRetry {
		return c.handleRetryPacket(hdr, p.data, p.rcvTime), nil
	}

	// The server can change the source connection ID with the first Handshake packet.
	// After this, all packets with a different source connection have to be ignored.
	if c.receivedFirstPacket && hdr.Type == protocol.PacketTypeInitial && hdr.SrcConnectionID != c.handshakeDestConnID {
		if c.qlogger != nil {
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:   qlog.PacketTypeInitial,
					PacketNumber: protocol.InvalidPacketNumber,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropUnknownConnectionID,
			})
		}
		c.logger.Debugf("Dropping Initial packet (%d bytes) with unexpected source connection ID: %s (expected %s)", p.Size(), hdr.SrcConnectionID, c.handshakeDestConnID)
		return false, nil
	}
	// drop 0-RTT packets, if we are a client
	if c.perspective == protocol.PerspectiveClient && hdr.Type == protocol.PacketType0RTT {
		if c.qlogger != nil {
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:   qlog.PacketType0RTT,
					PacketNumber: protocol.InvalidPacketNumber,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropUnexpectedPacket,
			})
		}
		return false, nil
	}

	packet, err := c.unpacker.UnpackLongHeader(hdr, p.data)
	if err != nil {
		wasQueued, err = c.handleUnpackError(err, p, toQlogPacketType(hdr.Type), datagramID)
		return false, err
	}

	if c.logger.Debug() {
		c.logger.Debugf("<- Reading packet %d (%d bytes) for connection %s, %s", packet.hdr.PacketNumber, p.Size(), hdr.DestConnectionID, packet.encryptionLevel)
		packet.hdr.Log(c.logger)
	}

	if pn := packet.hdr.PacketNumber; c.receivedPacketHandler.IsPotentiallyDuplicate(pn, packet.encryptionLevel) {
		c.logger.Debugf("Dropping (potentially) duplicate packet.")
		if c.qlogger != nil {
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:       toQlogPacketType(packet.hdr.Type),
					DestConnectionID: hdr.DestConnectionID,
					SrcConnectionID:  hdr.SrcConnectionID,
					PacketNumber:     pn,
					Version:          packet.hdr.Version,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size()), PayloadLength: int(packet.hdr.Length)},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropDuplicate,
			})
		}
		return false, nil
	}

	if err := c.handleUnpackedLongHeaderPacket(packet, p.ecn, p.rcvTime, datagramID, p.Size()); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Conn) handleUnpackError(err error, p receivedPacket, pt qlog.PacketType, datagramID qlog.DatagramID) (wasQueued bool, _ error) {
	switch err {
	case handshake.ErrKeysDropped:
		if c.qlogger != nil {
			connID, _ := wire.ParseConnectionID(p.data, c.srcConnIDLen)
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:       pt,
					DestConnectionID: connID,
					PacketNumber:     protocol.InvalidPacketNumber,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropKeyUnavailable,
			})
		}
		c.logger.Debugf("Dropping %s packet (%d bytes) because we already dropped the keys.", pt, p.Size())
		return false, nil
	case handshake.ErrKeysNotYetAvailable:
		// Sealer for this encryption level not yet available.
		// Try again later.
		return c.tryQueueingUndecryptablePacket(p, pt, datagramID), nil
	case wire.ErrInvalidReservedBits:
		return false, &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: err.Error(),
		}
	case handshake.ErrDecryptionFailed:
		// This might be a packet injected by an attacker. Drop it.
		if c.qlogger != nil {
			connID, _ := wire.ParseConnectionID(p.data, c.srcConnIDLen)
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:       pt,
					DestConnectionID: connID,
					PacketNumber:     protocol.InvalidPacketNumber,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropPayloadDecryptError,
			})
		}
		c.logger.Debugf("Dropping %s packet (%d bytes) that could not be unpacked. Error: %s", pt, p.Size(), err)
		return false, nil
	default:
		var headerErr *headerParseError
		if errors.As(err, &headerErr) {
			// This might be a packet injected by an attacker. Drop it.
			if c.qlogger != nil {
				connID, _ := wire.ParseConnectionID(p.data, c.srcConnIDLen)
				c.qlogger.RecordEvent(qlog.PacketDropped{
					Header: qlog.PacketHeader{
						PacketType:       pt,
						DestConnectionID: connID,
						PacketNumber:     protocol.InvalidPacketNumber,
					},
					Raw:        qlog.RawInfo{Length: int(p.Size())},
					DatagramID: datagramID,
					Trigger:    qlog.PacketDropHeaderParseError,
				})
			}
			c.logger.Debugf("Dropping %s packet (%d bytes) for which we couldn't unpack the header. Error: %s", pt, p.Size(), err)
			return false, nil
		}
		// This is an error returned by the AEAD (other than ErrDecryptionFailed).
		// For example, a PROTOCOL_VIOLATION due to key updates.
		return false, err
	}
}

func (c *Conn) handleRetryPacket(hdr *wire.Header, data []byte, rcvTime monotime.Time) bool /* was this a valid Retry */ {
	if c.perspective == protocol.PerspectiveServer || !c.sentFirstPacket ||
		c.receivedFirstPacket || c.receivedRetry || c.closeErr.Load() != nil {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropUnexpectedPacket)
		c.logger.Debugf("Ignoring Retry outside the original Initial phase.")
		return false
	}
	if hdr.Version != c.version {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropUnexpectedVersion)
		return false
	}
	if hdr.DestConnectionID != c.initialSrcConnID {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropUnknownConnectionID)
		return false
	}
	if hdr.SrcConnectionID == c.origDestConnID {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropUnexpectedPacket)
		c.logger.Debugf("Ignoring Retry, since the server didn't change the Source Connection ID.")
		return false
	}
	// The production parser already rejects an empty token. Keep the callee's
	// length proof explicit before slicing the tag, using the parsed owned token.
	headerLen := 7 + hdr.DestConnectionID.Len() + hdr.SrcConnectionID.Len()
	if len(hdr.Token) == 0 || len(data) < headerLen+16 || len(hdr.Token) != len(data)-headerLen-16 {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropHeaderParseError)
		return false
	}

	// Retry binds to the immutable destination of our original Initial, not a
	// potentially updated CID-manager view. This is not TLS peer authentication.
	tag := handshake.GetRetryIntegrityTag(data[:len(data)-16], c.origDestConnID, hdr.Version)
	if !bytes.Equal(data[len(data)-16:], tag[:]) {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropPayloadDecryptError)
		c.logger.Debugf("Ignoring Retry with invalid integrity tag.")
		return false
	}
	if c.closeErr.Load() != nil {
		c.logDroppedRetryPacket(hdr, len(data), qlog.PacketDropUnexpectedPacket)
		return false
	}

	newDestConnID := hdr.SrcConnectionID
	c.receivedRetry = true
	if err := c.sentPacketHandler.ResetForRetry(rcvTime); err != nil {
		c.destroyImpl(err)
		return false
	}
	c.handshakeDestConnID = newDestConnID
	c.retrySrcConnID = &newDestConnID
	c.cryptoStreamHandler.ChangeConnectionID(newDestConnID)
	c.packer.SetToken(hdr.Token)
	c.connIDManager.ChangeInitialConnID(newDestConnID)

	if c.logger.Debug() {
		c.logger.Debugf("<- Received Retry:")
		(&wire.ExtendedHeader{Header: *hdr}).Log(c.logger)
		c.logger.Debugf("Switching destination connection ID to: %s", hdr.SrcConnectionID)
	}
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.PacketReceived{
			Header: qlog.PacketHeader{
				PacketType:       qlog.PacketTypeRetry,
				DestConnectionID: hdr.DestConnectionID,
				SrcConnectionID:  newDestConnID,
				Version:          hdr.Version,
				Token:            &qlog.Token{Raw: hdr.Token},
			},
			Raw: qlog.RawInfo{Length: len(data)},
		})
	}

	c.scheduleSending()
	return true
}

func (c *Conn) logDroppedRetryPacket(hdr *wire.Header, size int, reason qlog.PacketDropReason) {
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.PacketDropped{
			Header: qlog.PacketHeader{
				PacketType:       qlog.PacketTypeRetry,
				SrcConnectionID:  hdr.SrcConnectionID,
				DestConnectionID: hdr.DestConnectionID,
				Version:          hdr.Version,
			},
			Raw:     qlog.RawInfo{Length: size},
			Trigger: reason,
		})
	}
}

func (c *Conn) handleVersionNegotiationPacket(p receivedPacket) error {
	if c.perspective == protocol.PerspectiveServer || // servers never receive version negotiation packets
		!c.sentFirstPacket || c.receivedRetry || // a processed Retry already commits this attempt's version
		c.receivedFirstPacket || c.versionNegotiated || c.closeErr.Load() != nil {
		c.logDroppedVersionNegotiation(p, qlog.PacketDropUnexpectedPacket)
		return nil
	}

	// Bind the response before allocating its version list. The existing prefix
	// parser returns owned CID copies in destination/source order. It does not
	// authenticate the peer; TLS and path/route authorization remain separate.
	_, dest, src, err := wire.ParseArbitraryLenConnectionIDs(p.data)
	if err != nil {
		c.logDroppedVersionNegotiation(p, qlog.PacketDropHeaderParseError)
		c.logger.Debugf("Error parsing Version Negotiation packet: %s", err)
		return nil
	}
	if !bytes.Equal(dest.Bytes(), c.initialSrcConnID.Bytes()) || !bytes.Equal(src.Bytes(), c.origDestConnID.Bytes()) {
		c.logDroppedVersionNegotiation(p, qlog.PacketDropUnknownConnectionID)
		return nil
	}
	dest, src, supportedVersions, err := wire.ParseVersionNegotiationPacket(p.data)
	if err != nil {
		c.logDroppedVersionNegotiation(p, qlog.PacketDropHeaderParseError)
		c.logger.Debugf("Error parsing Version Negotiation packet: %s", err)
		return nil
	}

	if slices.Contains(supportedVersions, c.version) {
		c.logDroppedVersionNegotiation(p, qlog.PacketDropUnexpectedVersion)
		// The Version Negotiation packet contains the version that we offered.
		// This might be a packet sent by an attacker, or it was corrupted.
		return nil
	}

	c.logger.Infof("Received a Version Negotiation packet. Supported Versions: %s", supportedVersions)
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.VersionNegotiationReceived{
			Header: qlog.PacketHeaderVersionNegotiation{
				DestConnectionID: dest,
				SrcConnectionID:  src,
			},
			SupportedVersions: supportedVersions,
		})
	}
	newVersion, ok := protocol.ChooseSupportedVersion(c.config.Versions, supportedVersions)
	if !ok {
		c.destroyImpl(&VersionNegotiationError{
			Ours:   c.config.Versions,
			Theirs: supportedVersions,
		})
		c.logger.Infof("No compatible QUIC version found.")
		return nil
	}
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.VersionInformation{
			ChosenVersion:  newVersion,
			ClientVersions: c.config.Versions,
			ServerVersions: supportedVersions,
		})
	}

	c.logger.Infof("Switching to QUIC version %s.", newVersion)
	nextPN, _ := c.sentPacketHandler.PeekPacketNumber(protocol.EncryptionInitial)
	return &errCloseForRecreating{
		nextPacketNumber: nextPN,
		nextVersion:      newVersion,
	}
}

func (c *Conn) logDroppedVersionNegotiation(p receivedPacket, reason qlog.PacketDropReason) {
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.PacketDropped{
			Header:  qlog.PacketHeader{PacketType: qlog.PacketTypeVersionNegotiation},
			Raw:     qlog.RawInfo{Length: int(p.Size())},
			Trigger: reason,
		})
	}
}

func (c *Conn) handleUnpackedLongHeaderPacket(
	packet *unpackedPacket,
	ecn protocol.ECN,
	rcvTime monotime.Time,
	datagramID qlog.DatagramID, // only for logging
	packetSize protocol.ByteCount, // only for logging
) error {
	if !c.receivedFirstPacket {
		c.receivedFirstPacket = true
		if !c.versionNegotiated && c.qlogger != nil {
			var clientVersions, serverVersions []Version
			switch c.perspective {
			case protocol.PerspectiveClient:
				clientVersions = c.config.Versions
			case protocol.PerspectiveServer:
				serverVersions = c.config.Versions
			}
			c.qlogger.RecordEvent(qlog.VersionInformation{
				ChosenVersion:  c.version,
				ClientVersions: clientVersions,
				ServerVersions: serverVersions,
			})
		}
		// The server can change the source connection ID with the first Handshake packet.
		if c.perspective == protocol.PerspectiveClient && packet.hdr.SrcConnectionID != c.handshakeDestConnID {
			cid := packet.hdr.SrcConnectionID
			c.logger.Debugf("Received first packet. Switching destination connection ID to: %s", cid)
			c.handshakeDestConnID = cid
			c.connIDManager.ChangeInitialConnID(cid)
		}
		// We create the connection as soon as we receive the first packet from the client.
		// We do that before authenticating the packet.
		// That means that if the source connection ID was corrupted,
		// we might have created a connection with an incorrect source connection ID.
		// Once we authenticate the first packet, we need to update it.
		if c.perspective == protocol.PerspectiveServer {
			if packet.hdr.SrcConnectionID != c.handshakeDestConnID {
				c.handshakeDestConnID = packet.hdr.SrcConnectionID
				c.connIDManager.ChangeInitialConnID(packet.hdr.SrcConnectionID)
			}
			if c.qlogger != nil {
				var srcAddr, destAddr *net.UDPAddr
				if addr, ok := c.conn.LocalAddr().(*net.UDPAddr); ok {
					srcAddr = addr
				}
				if addr, ok := c.conn.RemoteAddr().(*net.UDPAddr); ok {
					destAddr = addr
				}
				c.qlogger.RecordEvent(startedConnectionEvent(srcAddr, destAddr))
			}
		}
	}

	if c.perspective == protocol.PerspectiveServer && packet.encryptionLevel == protocol.EncryptionHandshake &&
		!c.droppedInitialKeys {
		// On the server side, Initial keys are dropped as soon as the first Handshake packet is received.
		// See Section 4.9.1 of RFC 9001.
		if err := c.dropEncryptionLevel(protocol.EncryptionInitial, rcvTime); err != nil {
			return err
		}
	}

	c.lastPacketReceivedTime = rcvTime
	c.firstAckElicitingPacketAfterIdleSentTime = 0
	c.keepAlivePingSent = false

	if packet.hdr.Type == protocol.PacketType0RTT {
		c.largestRcvdAppData = max(c.largestRcvdAppData, packet.hdr.PacketNumber)
	}

	var log func([]qlog.Frame)
	if c.qlogger != nil {
		log = func(frames []qlog.Frame) {
			var token *qlog.Token
			if len(packet.hdr.Token) > 0 {
				token = &qlog.Token{Raw: packet.hdr.Token}
			}
			c.qlogger.RecordEvent(qlog.PacketReceived{
				Header: qlog.PacketHeader{
					PacketType:       toQlogPacketType(packet.hdr.Type),
					DestConnectionID: packet.hdr.DestConnectionID,
					SrcConnectionID:  packet.hdr.SrcConnectionID,
					PacketNumber:     packet.hdr.PacketNumber,
					Version:          packet.hdr.Version,
					Token:            token,
				},
				Raw: qlog.RawInfo{
					Length:        int(packetSize),
					PayloadLength: int(packet.hdr.Length),
				},
				DatagramID: datagramID,
				Frames:     frames,
				ECN:        toQlogECN(ecn),
			})
		}
	}
	isAckEliciting, isNonProbing, _, err := c.handleFrames(packet.data, packet.hdr.DestConnectionID, packet.encryptionLevel, log, rcvTime)
	if err != nil {
		return err
	}
	if c.perspective == protocol.PerspectiveServer && packet.encryptionLevel == protocol.Encryption0RTT && isNonProbing {
		c.largestRcvdNonProbing = max(c.largestRcvdNonProbing, packet.hdr.PacketNumber)
	}
	c.sentPacketHandler.ReceivedPacket(packet.encryptionLevel, rcvTime)
	return c.receivedPacketHandler.ReceivedPacket(packet.hdr.PacketNumber, ecn, packet.encryptionLevel, rcvTime, isAckEliciting)
}

func (c *Conn) handleUnpackedShortHeaderPacket(
	destConnID protocol.ConnectionID,
	pn protocol.PacketNumber,
	data []byte,
	ecn protocol.ECN,
	rcvTime monotime.Time,
	log func([]qlog.Frame),
) (isNonProbing bool, pathChallenge *wire.PathChallengeFrame, _ error) {
	c.lastPacketReceivedTime = rcvTime
	c.firstAckElicitingPacketAfterIdleSentTime = 0
	c.keepAlivePingSent = false

	isAckEliciting, isNonProbing, pathChallenge, err := c.handleFrames(data, destConnID, protocol.Encryption1RTT, log, rcvTime)
	if err != nil {
		return false, nil, err
	}
	c.sentPacketHandler.ReceivedPacket(protocol.Encryption1RTT, rcvTime)
	if err := c.receivedPacketHandler.ReceivedPacket(pn, ecn, protocol.Encryption1RTT, rcvTime, isAckEliciting); err != nil {
		return false, nil, err
	}
	return isNonProbing, pathChallenge, nil
}

// handleFrames parses the frames, one after the other, and handles them.
// It returns the last PATH_CHALLENGE frame contained in the packet, if any.
func (c *Conn) handleFrames(
	data []byte,
	destConnID protocol.ConnectionID,
	encLevel protocol.EncryptionLevel,
	log func([]qlog.Frame),
	rcvTime monotime.Time,
) (isAckEliciting, isNonProbing bool, pathChallenge *wire.PathChallengeFrame, _ error) {
	// Only used for tracing.
	// If we're not tracing, this slice will always remain empty.
	var frames []qlog.Frame
	if log != nil {
		frames = make([]qlog.Frame, 0, 4)
	}
	handshakeWasComplete := c.handshakeComplete
	var handleErr error
	var skipHandling bool

	for len(data) > 0 {
		frameType, l, err := c.frameParser.ParseType(data, encLevel)
		if err != nil {
			// The frame parser skips over PADDING frames, and returns an io.EOF if the PADDING
			// frames were the last frames in this packet.
			if err == io.EOF {
				break
			}
			return false, false, nil, err
		}
		data = data[l:]

		if ackhandler.IsFrameTypeAckEliciting(frameType) {
			isAckEliciting = true
		}
		if !wire.IsProbingFrameType(frameType) {
			isNonProbing = true
		}

		// We're inlining common cases, to avoid using interfaces
		// Fast path: STREAM, DATAGRAM and ACK
		if frameType.IsStreamFrameType() {
			if !c.streamFrameIntake.available() {
				return false, false, nil, errors.Join(handleErr, errStreamFrameIntake)
			}
			l, parsed, streamErr := c.consumeStreamFrame(frameType, data, func(original *receivedStreamData) error {
				if err := c.streamFrameIntake.begin(original); err != nil {
					return err
				}
				returned := false
				defer c.streamFrameIntake.finish(&returned)
				streamFrame := original.frameView()
				if log != nil {
					frames = append(frames, toQlogFrame(streamFrame))
				}
				if skipHandling {
					returned = true // deferred original release, preserving the earlier error
					return nil
				}
				wire.LogFrame(c.logger, streamFrame, false)
				err := c.streamsMap.HandleStreamFrame(c.streamFrameIntake.transfer(), rcvTime)
				returned = true
				return err
			})
			if !parsed {
				return false, false, nil, streamErr
			}
			data = data[l:]
			if skipHandling {
				if streamErr != nil {
					return false, false, nil, errors.Join(handleErr, streamErr)
				}
				continue
			}
			handleErr = streamErr
		} else if frameType.IsAckFrameType() {
			ackFrame, l, err := c.frameParser.ParseAckFrame(frameType, data, encLevel, c.version)
			if err != nil {
				return false, false, nil, err
			}
			data = data[l:]
			if log != nil {
				frames = append(frames, toQlogFrame(ackFrame))
			}
			// an error occurred handling a previous frame, don't handle the current frame
			if skipHandling {
				continue
			}
			wire.LogFrame(c.logger, ackFrame, false)
			handleErr = c.handleAckFrame(ackFrame, encLevel, rcvTime)
		} else if frameType.IsDatagramFrameType() {
			datagramFrame, l, err := c.frameParser.ParseDatagramFrameView(frameType, data)
			if err != nil {
				return false, false, nil, err
			}
			data = data[l:]

			if log != nil {
				frames = append(frames, toQlogFrame(datagramFrame))
			}
			// an error occurred handling a previous frame, don't handle the current frame
			if skipHandling {
				continue
			}
			wire.LogFrame(c.logger, datagramFrame, false)
			handleErr = c.handleDatagramFrame(datagramFrame)
		} else {
			frame, l, err := c.frameParser.ParseLessCommonFrame(frameType, data, c.version)
			if err != nil {
				return false, false, nil, err
			}
			data = data[l:]

			if log != nil {
				frames = append(frames, toQlogFrame(frame))
			}
			// an error occurred handling a previous frame, don't handle the current frame
			if skipHandling {
				continue
			}
			pc, err := c.handleFrame(frame, encLevel, destConnID, rcvTime)
			if pc != nil {
				pathChallenge = pc
			}
			handleErr = err
		}

		if handleErr != nil {
			// if we're logging, we need to keep parsing (but not handling) all frames
			skipHandling = true
			if log == nil {
				return false, false, nil, handleErr
			}
		}
	}

	if log != nil {
		log(frames)
		if handleErr != nil {
			return false, false, nil, handleErr
		}
	}

	// Handle completion of the handshake after processing all the frames.
	// This ensures that we correctly handle the following case on the server side:
	// We receive a Handshake packet that contains the CRYPTO frame that allows us to complete the handshake,
	// and an ACK serialized after that CRYPTO frame. In this case, we still want to process the ACK frame.
	if !handshakeWasComplete && c.handshakeComplete {
		if err := c.handleHandshakeComplete(rcvTime); err != nil {
			return false, false, nil, err
		}
	}
	return
}

func (c *Conn) handleFrame(
	f wire.Frame,
	encLevel protocol.EncryptionLevel,
	destConnID protocol.ConnectionID,
	rcvTime monotime.Time,
) (pathChallenge *wire.PathChallengeFrame, _ error) {
	var err error
	wire.LogFrame(c.logger, f, false)
	switch frame := f.(type) {
	case *wire.CryptoFrame:
		err = c.handleCryptoFrame(frame, encLevel, rcvTime)
	case *wire.ConnectionCloseFrame:
		err = c.handleConnectionCloseFrame(frame)
	case *wire.ResetStreamFrame:
		err = c.streamsMap.HandleResetStreamFrame(frame, rcvTime)
	case *wire.MaxDataFrame:
		c.connFlowController.UpdateSendWindow(frame.MaximumData)
	case *wire.MaxStreamDataFrame:
		err = c.streamsMap.HandleMaxStreamDataFrame(frame)
	case *wire.MaxStreamsFrame:
		c.streamsMap.HandleMaxStreamsFrame(frame)
	case *wire.DataBlockedFrame:
	case *wire.StreamDataBlockedFrame:
		err = c.streamsMap.HandleStreamDataBlockedFrame(frame)
	case *wire.StreamsBlockedFrame:
	case *wire.StopSendingFrame:
		err = c.streamsMap.HandleStopSendingFrame(frame)
	case *wire.PingFrame:
	case *wire.PathChallengeFrame:
		err = c.handlePathChallengeFrame(frame)
		pathChallenge = frame
	case *wire.PathResponseFrame:
		err = c.handlePathResponseFrame(frame)
	case *wire.NewTokenFrame:
		err = c.handleNewTokenFrame(frame)
	case *wire.NewConnectionIDFrame:
		err = c.connIDManager.Add(frame)
	case *wire.RetireConnectionIDFrame:
		err = c.connIDGenerator.Retire(frame.SequenceNumber, destConnID, rcvTime.Add(3*c.rttStats.PTO(false)))
	case *wire.HandshakeDoneFrame:
		err = c.handleHandshakeDoneFrame(rcvTime)
	default:
		err = fmt.Errorf("unexpected frame type: %s", reflect.ValueOf(&frame).Elem().Type().Name())
	}
	return pathChallenge, err
}

func (c *Conn) handleConnectionCloseFrame(frame *wire.ConnectionCloseFrame) error {
	if frame.IsApplicationError {
		return &qerr.ApplicationError{
			Remote:       true,
			ErrorCode:    qerr.ApplicationErrorCode(frame.ErrorCode),
			ErrorMessage: frame.ReasonPhrase,
		}
	}
	return &qerr.TransportError{
		Remote:       true,
		ErrorCode:    qerr.TransportErrorCode(frame.ErrorCode),
		FrameType:    frame.FrameType,
		ErrorMessage: frame.ReasonPhrase,
	}
}

func (c *Conn) handleCryptoFrame(frame *wire.CryptoFrame, encLevel protocol.EncryptionLevel, rcvTime monotime.Time) error {
	if err := c.cryptoStreamManager.HandleCryptoFrame(frame, encLevel); err != nil {
		return err
	}
	for {
		data := c.cryptoStreamManager.GetCryptoData(encLevel)
		if data == nil {
			break
		}
		if err := c.cryptoStreamHandler.HandleMessage(data, encLevel); err != nil {
			return err
		}
	}
	return c.handleHandshakeEvents(rcvTime)
}

func (c *Conn) handleHandshakeEvents(now monotime.Time) error {
	for {
		ev := c.cryptoStreamHandler.NextEvent()
		var err error
		switch ev.Kind {
		case handshake.EventNoEvent:
			return nil
		case handshake.EventError:
			if ev.Err == nil {
				return handshake.ErrCryptoEventState
			}
			return ev.Err
		case handshake.EventHandshakeComplete:
			// Don't call handleHandshakeComplete yet.
			// It's advantageous to process ACK frames that might be serialized after the CRYPTO frame first.
			c.handshakeComplete = true
		case handshake.EventReceivedTransportParameters:
			err = c.handleTransportParameters(ev.TransportParameters)
		case handshake.EventRestoredTransportParameters:
			err = c.restoreTransportParameters(ev.TransportParameters)
			if err == nil {
				close(c.earlyConnReadyChan)
			}
		case handshake.EventReceivedReadKeys:
			// queue all previously undecryptable packets
			c.undecryptablePacketsToProcess = append(c.undecryptablePacketsToProcess, c.undecryptablePackets...)
			clear(c.undecryptablePackets) // moved references must not remain in old cells
			c.undecryptablePackets = nil
		case handshake.EventDiscard0RTTKeys:
			err = c.dropEncryptionLevel(protocol.Encryption0RTT, now)
		case handshake.EventWriteInitialData:
			_, err = c.initialStream.Write(ev.Data)
		case handshake.EventWriteHandshakeData:
			_, err = c.handshakeStream.Write(ev.Data)
		}
		if err != nil {
			return err
		}
	}
}

func (c *Conn) handlePathChallengeFrame(f *wire.PathChallengeFrame) error {
	owner := c.packetInput.processing
	if owner == nil || owner.packet.transport == nil || owner.packet.remoteAddr == nil {
		return errPacketInputUncertain
	}
	if c.perspective == protocol.PerspectiveServer && !addrsEqual(owner.packet.remoteAddr, c.RemoteAddr()) {
		if c.config.DisablePathManager || owner.serverPath == nil {
			return nil // bypass amplification / bounded path refusal remain separate
		}
	}
	if c.framer.QueuePathResponse(f.Data, owner.packet, owner.serverPath) {
		c.scheduleSending()
	}
	return nil
}

func (c *Conn) handlePathResponseFrame(f *wire.PathResponseFrame) error {
	switch c.perspective {
	case protocol.PerspectiveClient:
		return c.handlePathResponseFrameClient(f)
	case protocol.PerspectiveServer:
		return c.handlePathResponseFrameServer(f)
	default:
		panic("unreachable")
	}
}

func (c *Conn) handlePathResponseFrameClient(f *wire.PathResponseFrame) error {
	pm := c.pathManagerOutgoing.Load()
	if pm == nil {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "unexpected PATH_RESPONSE frame",
		}
	}
	pm.HandlePathResponseFrame(f)
	return nil
}

func (c *Conn) handlePathResponseFrameServer(f *wire.PathResponseFrame) error {
	if c.pathManager == nil {
		// since we didn't send PATH_CHALLENGEs yet, we don't expect PATH_RESPONSEs
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "unexpected PATH_RESPONSE frame",
		}
	}
	c.pathManager.HandlePathResponseFrame(f, monotime.Now(), c.serverPathPTO())
	c.scheduleSending() // a short validation may require a full-sized follow-up
	return nil
}

func (c *Conn) handleNewTokenFrame(frame *wire.NewTokenFrame) error {
	if c.perspective == protocol.PerspectiveServer {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "received NEW_TOKEN frame from the client",
		}
	}
	if c.config.TokenStore != nil {
		c.config.TokenStore.Put(c.tokenStoreKey, &ClientToken{data: frame.Token, rtt: c.rttStats.SmoothedRTT()})
	}
	return nil
}

func (c *Conn) handleHandshakeDoneFrame(rcvTime monotime.Time) error {
	if c.perspective == protocol.PerspectiveServer {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "received a HANDSHAKE_DONE frame",
		}
	}
	if !c.handshakeConfirmed {
		return c.handleHandshakeConfirmed(rcvTime)
	}
	return nil
}

func (c *Conn) handleAckFrame(frame *wire.AckFrame, encLevel protocol.EncryptionLevel, rcvTime monotime.Time) error {
	acked1RTTPacket, err := c.sentPacketHandler.ReceivedAck(frame, encLevel, c.lastPacketReceivedTime)
	if err != nil {
		return err
	}
	if !acked1RTTPacket {
		return nil
	}
	// On the client side: If the packet acknowledged a 1-RTT packet, this confirms the handshake.
	// This is only possible if the ACK was sent in a 1-RTT packet.
	// This is an optimization over simply waiting for a HANDSHAKE_DONE frame, see section 4.1.2 of RFC 9001.
	if c.perspective == protocol.PerspectiveClient && !c.handshakeConfirmed {
		if err := c.handleHandshakeConfirmed(rcvTime); err != nil {
			return err
		}
	}
	// If one of the acknowledged packets was a Path MTU probe packet, this might have increased the Path MTU estimate.
	if c.mtuDiscoverer != nil {
		if mtu := c.mtuDiscoverer.CurrentSize(); mtu > protocol.ByteCount(c.currentMTUEstimate.Load()) {
			c.currentMTUEstimate.Store(uint32(mtu))
			c.sentPacketHandler.SetMaxDatagramSize(mtu)
		}
	}
	return c.cryptoStreamHandler.SetLargest1RTTAcked(frame.LargestAcked())
}

func (c *Conn) handleDatagramFrame(f *wire.DatagramFrame) error {
	if f.Length(c.version) > wire.MaxDatagramSize {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "DATAGRAM frame too large",
		}
	}
	c.datagramQueue.HandleDatagramFrame(f)
	return nil
}

func (c *Conn) setCloseError(e *closeError) {
	if c.closeErr.CompareAndSwap(nil, e) {
		c.congestionCalls.seal()
		c.packetInput.seal()
		close(c.closing)
		if c.streamsMap != nil {
			c.streamsMap.calls.seal(e.err)
		}
		if c.receiveFrames != nil {
			c.receiveFrames.calls.seal(e.err)
			c.receiveFrames.seal()
			c.receiveFrames.sent.seal()
		}
	}
	select {
	case c.closeChan <- struct{}{}:
	default:
	}
}

// closeLocal closes the connection and send a CONNECTION_CLOSE containing the error
func (c *Conn) closeLocal(e error) {
	c.setCloseError(&closeError{err: e, immediate: false})
}

// destroy closes the connection without sending the error on the wire
func (c *Conn) destroy(e error) {
	c.destroyImpl(e)
	<-c.ctx.Done()
}

func (c *Conn) destroyImpl(e error) {
	c.setCloseError(&closeError{err: e, immediate: true})
}

// CloseWithError closes the connection with an error.
// The error string will be sent to the peer.
func (c *Conn) CloseWithError(code ApplicationErrorCode, desc string) error {
	c.closeLocal(&qerr.ApplicationError{
		ErrorCode:    code,
		ErrorMessage: desc,
	})
	<-c.ctx.Done()
	return nil
}

func (c *Conn) closeWithTransportError(code TransportErrorCode) {
	c.closeLocal(&qerr.TransportError{ErrorCode: code})
	<-c.ctx.Done()
}

func (c *Conn) handleCloseError(closeErr *closeError) (shutdownErr error) {
	if closeErr.immediate {
		if nerr, ok := closeErr.err.(net.Error); ok && nerr.Timeout() {
			c.logger.Errorf("Destroying connection: %s", closeErr.err)
		} else {
			c.logger.Errorf("Destroying connection with error: %s", closeErr.err)
		}
	} else {
		if closeErr.err == nil {
			c.logger.Infof("Closing connection.")
		} else {
			c.logger.Errorf("Closing connection with error: %s", closeErr.err)
		}
	}

	e := closeErr.err
	if e == nil {
		e = &qerr.ApplicationError{}
	}
	defer func() { closeErr.err = e }()

	var (
		statelessResetErr     *StatelessResetError
		versionNegotiationErr *VersionNegotiationError
		recreateErr           *errCloseForRecreating
		applicationErr        *ApplicationError
		transportErr          *TransportError
	)
	var isRemoteClose bool
	var trigger qlog.ConnectionCloseTrigger
	var reason string
	var transportErrorCode *qlog.TransportErrorCode
	var applicationErrorCode *qlog.ApplicationErrorCode
	switch {
	case errors.Is(e, qerr.ErrIdleTimeout),
		errors.Is(e, qerr.ErrHandshakeTimeout):
		trigger = qlog.ConnectionCloseTriggerIdleTimeout
	case errors.As(e, &statelessResetErr):
		trigger = qlog.ConnectionCloseTriggerStatelessReset
	case errors.As(e, &versionNegotiationErr):
		trigger = qlog.ConnectionCloseTriggerVersionMismatch
	case errors.As(e, &recreateErr):
	case errors.As(e, &applicationErr):
		isRemoteClose = applicationErr.Remote
		reason = applicationErr.ErrorMessage
		applicationErrorCode = &applicationErr.ErrorCode
	case errors.As(e, &transportErr):
		isRemoteClose = transportErr.Remote
		reason = transportErr.ErrorMessage
		transportErrorCode = &transportErr.ErrorCode
	case closeErr.immediate:
		e = closeErr.err
	default:
		te := &qerr.TransportError{
			ErrorCode:    qerr.InternalError,
			ErrorMessage: e.Error(),
		}
		e = te
		reason = te.ErrorMessage
		code := te.ErrorCode
		transportErrorCode = &code
	}

	shutdownErr = c.streamsMap.CloseWithError(e)
	if shutdownErr != nil {
		e = errors.Join(e, shutdownErr)
		closeErr.immediate = true // uncertainty cannot authorize a terminal write
		c.receiveFrames.markUncertain()
	}
	if c.datagramQueue != nil {
		c.datagramQueue.CloseWithError(e)
	}

	// In rare instances, the connection ID manager might switch to a new connection ID
	// when sending the CONNECTION_CLOSE frame.
	// The connection ID manager removes the active stateless reset token from the packet
	// handler map when it is closed, so we need to make sure that this happens last.
	defer c.connIDManager.Close()

	if c.qlogger != nil && !errors.As(e, &recreateErr) {
		initiator := qlog.InitiatorLocal
		if isRemoteClose {
			initiator = qlog.InitiatorRemote
		}
		c.qlogger.RecordEvent(qlog.ConnectionClosed{
			Initiator:        initiator,
			ConnectionError:  transportErrorCode,
			ApplicationError: applicationErrorCode,
			Trigger:          trigger,
			Reason:           reason,
		})
	}

	// If this is a remote close we're done here
	if isRemoteClose {
		c.connIDGenerator.ReplaceWithClosed(nil, 3*c.rttStats.PTO(false))
		return
	}
	if closeErr.immediate {
		c.connIDGenerator.RemoveAll()
		return
	}
	// Don't send out any CONNECTION_CLOSE if this is an error that occurred
	// before we even sent out the first packet.
	if c.perspective == protocol.PerspectiveClient && !c.sentFirstPacket {
		c.connIDGenerator.RemoveAll()
		return
	}
	connClosePacket, err := c.sendConnectionClose(e)
	if err != nil {
		c.logger.Debugf("Error sending CONNECTION_CLOSE: %s", err)
	}
	c.connIDGenerator.ReplaceWithClosed(connClosePacket, 3*c.rttStats.PTO(false))
	if connClosePacket != nil {
		// Every transport copied the bytes before its ordinary callback return.
		// No cache or queued retry is allowed to keep a pooled-buffer view.
		c.releasePacketOutput()
	}
	return shutdownErr
}

func (c *Conn) dropEncryptionLevel(encLevel protocol.EncryptionLevel, now monotime.Time) error {
	if encLevel == protocol.Encryption0RTT {
		// Revoke new DATAGRAM admissions before dropping original history/resetting
		// streams. Fresh parameters can authorize outbound work only at actual apply.
		c.publishPeerSendParameters(&peerParameterSnapshot{phase: peerParametersRejected0RTT})
	}
	c.sentPacketHandler.DropPackets(encLevel, now)
	c.receivedPacketHandler.DropPackets(encLevel)
	//nolint:exhaustive // only Initial and 0-RTT need special treatment
	switch encLevel {
	case protocol.EncryptionInitial:
		c.droppedInitialKeys = true
		c.cryptoStreamHandler.DiscardInitialKeys()
	case protocol.Encryption0RTT:
		return c.streamsMap.ResetFor0RTT(c.connFlowController.Reset)
	}
	return c.cryptoStreamManager.Drop(encLevel)
}

// is called for the client, when restoring transport parameters saved for 0-RTT
func (c *Conn) restoreTransportParameters(params *wire.TransportParameters) error {
	if params == nil {
		return errPeerParameterState
	}
	if c.logger.Debug() {
		c.logger.Debugf("Restoring Transport Parameters: %s", params)
	}
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.ParametersSet{
			Restore:                         true,
			Initiator:                       qlog.InitiatorRemote,
			SentBy:                          c.perspective,
			OriginalDestinationConnectionID: params.OriginalDestinationConnectionID,
			InitialSourceConnectionID:       params.InitialSourceConnectionID,
			RetrySourceConnectionID:         params.RetrySourceConnectionID,
			StatelessResetToken:             params.StatelessResetToken,
			DisableActiveMigration:          params.DisableActiveMigration,
			MaxIdleTimeout:                  params.MaxIdleTimeout,
			MaxUDPPayloadSize:               params.MaxUDPPayloadSize,
			AckDelayExponent:                params.AckDelayExponent,
			MaxAckDelay:                     params.MaxAckDelay,
			ActiveConnectionIDLimit:         params.ActiveConnectionIDLimit,
			InitialMaxData:                  params.InitialMaxData,
			InitialMaxStreamDataBidiLocal:   params.InitialMaxStreamDataBidiLocal,
			InitialMaxStreamDataBidiRemote:  params.InitialMaxStreamDataBidiRemote,
			InitialMaxStreamDataUni:         params.InitialMaxStreamDataUni,
			InitialMaxStreamsBidi:           int64(params.MaxBidiStreamNum),
			InitialMaxStreamsUni:            int64(params.MaxUniStreamNum),
			MaxDatagramFrameSize:            params.MaxDatagramFrameSize,
			EnableResetStreamAt:             params.EnableResetStreamAt,
		})
	}

	c.peerParams = params
	if err := c.connIDGenerator.SetMaxActiveConnIDs(params.ActiveConnectionIDLimit); err != nil {
		return err
	}
	c.connFlowController.UpdateSendWindow(params.InitialMaxData)
	c.streamsMap.HandleTransportParameters(params)
	snapshot := snapshotPeerParameters(params, peerParametersRestored)
	c.peerObservedParams.Store(snapshot)
	c.publishPeerSendParameters(snapshot)
	return nil
}

func (c *Conn) handleTransportParameters(params *wire.TransportParameters) error {
	if params == nil {
		return errPeerParameterState
	}
	if c.qlogger != nil {
		c.qlogTransportParameters(params, c.perspective.Opposite(), false)
	}
	if err := c.checkTransportParameters(params); err != nil {
		return &qerr.TransportError{
			ErrorCode:    qerr.TransportParameterError,
			ErrorMessage: err.Error(),
		}
	}

	if c.perspective == protocol.PerspectiveClient && c.peerParams != nil && c.ConnectionState().Used0RTT && !params.ValidForUpdate(c.peerParams) {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: "server reduced limits or disabled partial reset after accepting 0-RTT data",
		}
	}

	c.streamsMap.receivedTransportParameters(params)
	c.peerParams = params
	c.peerObservedParams.Store(snapshotPeerParameters(params, peerParametersReceived))
	// On the client side we have to wait for handshake completion.
	// During a 0-RTT connection, we are only allowed to use the new transport parameters for 1-RTT packets.
	if c.perspective == protocol.PerspectiveServer {
		if err := c.applyTransportParameters(); err != nil {
			return err
		}
		// On the server side, the early connection is ready as soon as we processed
		// the client's transport parameters.
		close(c.earlyConnReadyChan)
	}
	return nil
}

func (c *Conn) checkTransportParameters(params *wire.TransportParameters) error {
	if c.logger.Debug() {
		c.logger.Debugf("Processed Transport Parameters: %s", params)
	}

	// check the initial_source_connection_id
	if params.InitialSourceConnectionID != c.handshakeDestConnID {
		return fmt.Errorf("expected initial_source_connection_id to equal %s, is %s", c.handshakeDestConnID, params.InitialSourceConnectionID)
	}

	if c.perspective == protocol.PerspectiveServer {
		return nil
	}
	// check the original_destination_connection_id
	if params.OriginalDestinationConnectionID != c.origDestConnID {
		return fmt.Errorf("expected original_destination_connection_id to equal %s, is %s", c.origDestConnID, params.OriginalDestinationConnectionID)
	}
	if c.retrySrcConnID != nil { // a Retry was performed
		if params.RetrySourceConnectionID == nil {
			return errors.New("missing retry_source_connection_id")
		}
		if *params.RetrySourceConnectionID != *c.retrySrcConnID {
			return fmt.Errorf("expected retry_source_connection_id to equal %s, is %s", c.retrySrcConnID, *params.RetrySourceConnectionID)
		}
	} else if params.RetrySourceConnectionID != nil {
		return errors.New("received retry_source_connection_id, although no Retry was performed")
	}
	return nil
}

func (c *Conn) applyTransportParameters() error {
	params := c.peerParams
	if params == nil {
		return errPeerParameterState
	}
	// Our local idle timeout will always be > 0.
	c.idleTimeout = c.config.MaxIdleTimeout
	// If the peer advertised an idle timeout, take the minimum of the values.
	if params.MaxIdleTimeout > 0 {
		c.idleTimeout = min(c.idleTimeout, params.MaxIdleTimeout)
	}
	c.keepAliveInterval = min(c.config.KeepAlivePeriod, c.idleTimeout/2)
	c.streamsMap.HandleTransportParameters(params)
	c.frameParser.SetAckDelayExponent(params.AckDelayExponent)
	c.connFlowController.UpdateSendWindow(params.InitialMaxData)
	c.rttStats.SetMaxAckDelay(params.MaxAckDelay)
	if err := c.connIDGenerator.SetMaxActiveConnIDs(params.ActiveConnectionIDLimit); err != nil {
		return err
	}
	if params.StatelessResetToken != nil {
		c.connIDManager.SetStatelessResetToken(*params.StatelessResetToken)
		if closeErr := c.closeErr.Load(); closeErr != nil {
			return closeErr.err
		}
	}
	// We don't support connection migration yet, so we don't have any use for the preferred_address.
	if params.PreferredAddress != nil {
		// Retire the connection ID.
		c.connIDManager.AddFromPreferredAddress(params.PreferredAddress.ConnectionID, params.PreferredAddress.StatelessResetToken)
	}
	maxPacketSize := protocol.ByteCount(protocol.MaxPacketBufferSize)
	if params.MaxUDPPayloadSize > 0 && params.MaxUDPPayloadSize < maxPacketSize {
		maxPacketSize = params.MaxUDPPayloadSize
	}
	c.mtuDiscoverer = newMTUDiscoverer(
		c.rttStats,
		protocol.ByteCount(c.config.InitialPacketSize),
		maxPacketSize,
		c.qlogger,
	)
	c.publishPeerSendParameters(snapshotPeerParameters(params, peerParametersApplied))
	return nil
}

func (c *Conn) triggerSending(now monotime.Time) error {
	c.pacingDeadline = 0

	sendMode := c.sentPacketHandler.SendMode(now)
	switch sendMode {
	case ackhandler.SendAny:
		return c.sendPackets(now)
	case ackhandler.SendNone:
		c.blocked = blockModeHardBlocked
		return nil
	case ackhandler.SendPacingLimited:
		deadline := c.sentPacketHandler.TimeUntilSend()
		if deadline.IsZero() {
			deadline = deadlineSendImmediately
		}
		c.pacingDeadline = deadline
		// Allow sending of an ACK if we're pacing limit.
		// This makes sure that a peer that is mostly receiving data (and thus has an inaccurate cwnd estimate)
		// sends enough ACKs to allow its peer to utilize the bandwidth.
		return c.maybeSendAckOnlyPacket(now)
	case ackhandler.SendAck:
		// We can at most send a single ACK only packet.
		// There will only be a new ACK after receiving new packets.
		// SendAck is only returned when we're congestion limited, so we don't need to set the pacing timer.
		c.blocked = blockModeCongestionLimited
		return c.maybeSendAckOnlyPacket(now)
	case ackhandler.SendPTOInitial, ackhandler.SendPTOHandshake, ackhandler.SendPTOAppData:
		if err := c.sendProbePacket(sendMode, now); err != nil {
			return err
		}
		if c.sendQueue.WouldBlock() {
			c.scheduleSending()
			return nil
		}
		return c.triggerSending(now)
	default:
		return fmt.Errorf("BUG: invalid send mode %d", sendMode)
	}
}

func (c *Conn) sendPackets(now monotime.Time) error {
	if sent, err := c.sendPendingPathResponse(now); err != nil {
		return err
	} else if sent {
		c.scheduleSending()
		return nil
	}
	if c.perspective == protocol.PerspectiveServer {
		if sent, err := c.sendPendingServerPathProbe(now); err != nil {
			return err
		} else if sent {
			c.scheduleSending()
			return nil
		}
	}
	if c.perspective == protocol.PerspectiveClient && c.handshakeConfirmed {
		if pm := c.pathManagerOutgoing.Load(); pm != nil {
			connID, frame, tr, ok := pm.NextPathToProbe()
			if ok {
				probe, buf, err := c.packer.PackPathProbePacket(connID, []ackhandler.Frame{frame}, c.version)
				if err != nil {
					return err
				}
				if err := c.sendOwnedPathProbe(probe, buf, tr, now); err != nil {
					return err
				}
				// There's (likely) more data to send. Loop around again.
				c.scheduleSending()
				return nil
			}
		}
	}

	// Path MTU Discovery
	// Can't use GSO, since we need to send a single packet that's larger than our current maximum size.
	// Performance-wise, this doesn't matter, since we only send a very small (<10) number of
	// MTU probe packets per connection.
	if c.handshakeConfirmed && c.mtuDiscoverer != nil && c.mtuDiscoverer.ShouldSendProbe(now) && c.pathMTUDiscoveryEnabled(c.conn) {
		ping, size := c.mtuDiscoverer.GetPing(now)
		p, buf, err := c.packer.PackMTUProbePacket(ping, size, c.version)
		if err != nil {
			return err
		}
		ecn := c.sentPacketHandler.ECNMode(true)
		if err := c.beginPacketOutput(buf); err != nil {
			return err
		}
		if err := c.recordPacketDatagrams(p.Frames); err != nil {
			return c.rejectPacketOutput(err)
		}
		c.logShortHeaderPacket(p, ecn, buf.Len())
		if err := c.registerPackedShortHeaderPacket(p, ecn, now); err != nil {
			return c.rejectPacketOutput(err)
		}
		if err := c.sendPacketOutput(0, ecn); err != nil {
			return err
		}
		// There's (likely) more data to send. Loop around again.
		c.scheduleSending()
		return nil
	}

	if offset := c.connFlowController.GetWindowUpdate(now); offset > 0 {
		c.framer.QueueControlFrame(&wire.MaxDataFrame{MaximumData: offset})
	}
	if cf := c.cryptoStreamManager.GetPostHandshakeData(protocol.MaxPostHandshakeCryptoFrameSize); cf != nil {
		c.queueControlFrame(cf)
	}

	if !c.handshakeConfirmed {
		packet, err := c.packer.PackCoalescedPacket(false, c.maxPacketSize(), now, c.version)
		if err != nil || packet == nil {
			return err
		}
		c.sentFirstPacket = true
		if err := c.sendPackedCoalescedPacket(packet, c.sentPacketHandler.ECNMode(packet.IsOnlyShortHeaderPacket()), now); err != nil {
			return err
		}
		//nolint:exhaustive // only need to handle pacing-related events here
		switch c.sentPacketHandler.SendMode(now) {
		case ackhandler.SendPacingLimited:
			c.resetPacingDeadline()
		case ackhandler.SendAny:
			c.pacingDeadline = deadlineSendImmediately
		}
		return nil
	}

	if c.conn.capabilities().GSO {
		return c.sendPacketsWithGSO(now)
	}
	return c.sendPacketsWithoutGSO(now)
}

func (c *Conn) sendPacketsWithoutGSO(now monotime.Time) error {
	for {
		buf := getPacketBuffer()
		if err := c.beginPacketOutput(buf); err != nil {
			return err
		}
		ecn := c.sentPacketHandler.ECNMode(true)
		if _, err := c.appendOneShortHeaderPacket(buf, c.maxPacketSize(), ecn, now); err != nil {
			if err == errNothingToPack {
				c.releasePacketOutput()
				// Notify congestion controller that app has no data to send
				c.sentPacketHandler.MaybeNotifyAppLimited()
				return nil
			}
			return c.rejectPacketOutput(err)
		}

		if err := c.sendPacketOutput(0, ecn); err != nil {
			return err
		}

		if c.sendQueue.WouldBlock() {
			return nil
		}
		sendMode := c.sentPacketHandler.SendMode(now)
		if sendMode == ackhandler.SendPacingLimited {
			c.resetPacingDeadline()
			return nil
		}
		if sendMode != ackhandler.SendAny {
			return nil
		}
		// Prioritize receiving of packets over sending out more packets.
		c.receivedPacketMx.Lock()
		hasPackets := !c.receivedPackets.Empty()
		c.receivedPacketMx.Unlock()
		if hasPackets {
			c.pacingDeadline = deadlineSendImmediately
			return nil
		}
	}
}

func (c *Conn) sendPacketsWithGSO(now monotime.Time) error {
	buf := getLargePacketBuffer()
	if err := c.beginPacketOutput(buf); err != nil {
		return err
	}
	maxSize := c.maxPacketSize()

	ecn := c.sentPacketHandler.ECNMode(true)
	for {
		var dontSendMore bool
		size, err := c.appendOneShortHeaderPacket(buf, maxSize, ecn, now)
		if err != nil {
			if err != errNothingToPack {
				return c.rejectPacketOutput(err)
			}
			if buf.Len() == 0 {
				c.releasePacketOutput()
				// Notify congestion controller that app has no data to send
				c.sentPacketHandler.MaybeNotifyAppLimited()
				return nil
			}
			dontSendMore = true
		}

		if !dontSendMore {
			sendMode := c.sentPacketHandler.SendMode(now)
			if sendMode == ackhandler.SendPacingLimited {
				c.resetPacingDeadline()
			}
			if sendMode != ackhandler.SendAny {
				dontSendMore = true
			}
		}

		// Don't send more packets in this batch if they require a different ECN marking than the previous ones.
		nextECN := c.sentPacketHandler.ECNMode(true)

		// Append another packet if
		// 1. The congestion controller and pacer allow sending more
		// 2. The last packet appended was a full-size packet
		// 3. The next packet will have the same ECN marking
		// 4. We still have enough space for another full-size packet in the buffer
		if !dontSendMore && size == maxSize && nextECN == ecn && buf.Len()+maxSize <= buf.Cap() {
			continue
		}

		if err := c.sendPacketOutput(uint16(maxSize), ecn); err != nil {
			return err
		}

		if dontSendMore {
			return nil
		}
		if c.sendQueue.WouldBlock() {
			return nil
		}

		// Prioritize receiving of packets over sending out more packets.
		c.receivedPacketMx.Lock()
		hasPackets := !c.receivedPackets.Empty()
		c.receivedPacketMx.Unlock()
		if hasPackets {
			c.pacingDeadline = deadlineSendImmediately
			return nil
		}

		ecn = nextECN
		buf = getLargePacketBuffer()
		if err := c.beginPacketOutput(buf); err != nil {
			return err
		}
	}
}

func (c *Conn) resetPacingDeadline() {
	deadline := c.sentPacketHandler.TimeUntilSend()
	if deadline.IsZero() {
		deadline = deadlineSendImmediately
	}
	c.pacingDeadline = deadline
}

func (c *Conn) maybeSendAckOnlyPacket(now monotime.Time) error {
	if !c.handshakeConfirmed {
		ecn := c.sentPacketHandler.ECNMode(false)
		packet, err := c.packer.PackCoalescedPacket(true, c.maxPacketSize(), now, c.version)
		if err != nil {
			return err
		}
		if packet == nil {
			return nil
		}
		return c.sendPackedCoalescedPacket(packet, ecn, now)
	}

	ecn := c.sentPacketHandler.ECNMode(true)
	p, buf, err := c.packer.PackAckOnlyPacket(c.maxPacketSize(), now, c.version)
	if err != nil {
		if err == errNothingToPack {
			return nil
		}
		return err
	}
	if err := c.beginPacketOutput(buf); err != nil {
		return err
	}
	if err := c.recordPacketDatagrams(p.Frames); err != nil {
		return c.rejectPacketOutput(err)
	}
	c.logShortHeaderPacket(p, ecn, buf.Len())
	if err := c.registerPackedShortHeaderPacket(p, ecn, now); err != nil {
		return c.rejectPacketOutput(err)
	}
	return c.sendPacketOutput(0, ecn)
}

func (c *Conn) sendProbePacket(sendMode ackhandler.SendMode, now monotime.Time) error {
	var encLevel protocol.EncryptionLevel
	//nolint:exhaustive // We only need to handle the PTO send modes here.
	switch sendMode {
	case ackhandler.SendPTOInitial:
		encLevel = protocol.EncryptionInitial
	case ackhandler.SendPTOHandshake:
		encLevel = protocol.EncryptionHandshake
	case ackhandler.SendPTOAppData:
		encLevel = protocol.Encryption1RTT
	default:
		return fmt.Errorf("connection BUG: unexpected send mode: %d", sendMode)
	}
	// Queue probe packets until we actually send out a packet,
	// or until there are no more packets to queue.
	var packet *coalescedPacket
	for packet == nil {
		if wasQueued := c.sentPacketHandler.QueueProbePacket(encLevel); !wasQueued {
			break
		}
		var err error
		packet, err = c.packer.PackPTOProbePacket(encLevel, c.maxPacketSize(), false, now, c.version)
		if err != nil {
			return err
		}
	}
	if packet == nil {
		var err error
		packet, err = c.packer.PackPTOProbePacket(encLevel, c.maxPacketSize(), true, now, c.version)
		if err != nil {
			return err
		}
	}
	if packet == nil || (len(packet.longHdrPackets) == 0 && packet.shortHdrPacket == nil) {
		return fmt.Errorf("connection BUG: couldn't pack %s probe packet: %v", encLevel, packet)
	}
	return c.sendPackedCoalescedPacket(packet, c.sentPacketHandler.ECNMode(packet.IsOnlyShortHeaderPacket()), now)
}

// appendOneShortHeaderPacket appends a new packet to the given packetBuffer.
// If there was nothing to pack, the returned size is 0.
func (c *Conn) appendOneShortHeaderPacket(buf *packetBuffer, maxSize protocol.ByteCount, ecn protocol.ECN, now monotime.Time) (protocol.ByteCount, error) {
	startLen := buf.Len()
	p, err := c.packer.AppendPacket(buf, maxSize, now, c.version)
	if err != nil {
		return 0, err
	}
	size := buf.Len() - startLen
	if err := c.recordPacketDatagrams(p.Frames); err != nil {
		return 0, err
	}
	c.logShortHeaderPacket(p, ecn, size)
	if err := c.registerPackedShortHeaderPacket(p, ecn, now); err != nil {
		return 0, err
	}
	return size, nil
}

func (c *Conn) registerPackedShortHeaderPacket(p shortHeaderPacket, ecn protocol.ECN, now monotime.Time) error {
	if p.IsPathProbePacket {
		return c.sentPacketHandler.SentPacket(
			now,
			p.PacketNumber,
			protocol.InvalidPacketNumber,
			p.StreamFrames,
			p.Frames,
			protocol.Encryption1RTT,
			ecn,
			p.Length,
			p.IsPathMTUProbePacket,
			true,
		)
	}
	if c.firstAckElicitingPacketAfterIdleSentTime.IsZero() && (len(p.StreamFrames) > 0 || ackhandler.HasAckElicitingFrames(p.Frames)) {
		c.firstAckElicitingPacketAfterIdleSentTime = now
	}

	largestAcked := protocol.InvalidPacketNumber
	if p.Ack != nil {
		largestAcked = p.Ack.LargestAcked()
	}
	if err := c.sentPacketHandler.SentPacket(
		now,
		p.PacketNumber,
		largestAcked,
		p.StreamFrames,
		p.Frames,
		protocol.Encryption1RTT,
		ecn,
		p.Length,
		p.IsPathMTUProbePacket,
		false,
	); err != nil {
		return err
	}
	c.connIDManager.SentPacket()
	return nil
}

func (c *Conn) sendPackedCoalescedPacket(packet *coalescedPacket, ecn protocol.ECN, now monotime.Time) error {
	if err := c.beginPacketOutput(packet.buffer); err != nil {
		return err
	}
	for _, p := range packet.longHdrPackets {
		if err := c.recordPacketDatagrams(p.frames); err != nil {
			return c.rejectPacketOutput(err)
		}
	}
	if p := packet.shortHdrPacket; p != nil {
		if err := c.recordPacketDatagrams(p.Frames); err != nil {
			return c.rejectPacketOutput(err)
		}
	}
	c.logCoalescedPacket(packet, ecn)
	for _, p := range packet.longHdrPackets {
		if c.firstAckElicitingPacketAfterIdleSentTime.IsZero() && p.IsAckEliciting() {
			c.firstAckElicitingPacketAfterIdleSentTime = now
		}
		largestAcked := protocol.InvalidPacketNumber
		if p.ack != nil {
			largestAcked = p.ack.LargestAcked()
		}
		if err := c.sentPacketHandler.SentPacket(
			now,
			p.header.PacketNumber,
			largestAcked,
			p.streamFrames,
			p.frames,
			p.EncryptionLevel(),
			ecn,
			p.length,
			false,
			false,
		); err != nil {
			return c.rejectPacketOutput(err)
		}
		if c.perspective == protocol.PerspectiveClient && p.EncryptionLevel() == protocol.EncryptionHandshake &&
			!c.droppedInitialKeys {
			// On the client side, Initial keys are dropped as soon as the first Handshake packet is sent.
			// See Section 4.9.1 of RFC 9001.
			if err := c.dropEncryptionLevel(protocol.EncryptionInitial, now); err != nil {
				return c.rejectPacketOutput(err)
			}
		}
	}
	if p := packet.shortHdrPacket; p != nil {
		if c.firstAckElicitingPacketAfterIdleSentTime.IsZero() && p.IsAckEliciting() {
			c.firstAckElicitingPacketAfterIdleSentTime = now
		}
		largestAcked := protocol.InvalidPacketNumber
		if p.Ack != nil {
			largestAcked = p.Ack.LargestAcked()
		}
		if err := c.sentPacketHandler.SentPacket(
			now,
			p.PacketNumber,
			largestAcked,
			p.StreamFrames,
			p.Frames,
			protocol.Encryption1RTT,
			ecn,
			p.Length,
			p.IsPathMTUProbePacket,
			false,
		); err != nil {
			return c.rejectPacketOutput(err)
		}
	}
	c.connIDManager.SentPacket()
	return c.sendPacketOutput(0, ecn)
}

func (c *Conn) sendConnectionClose(e error) ([]byte, error) {
	var packet *coalescedPacket
	var err error
	var transportErr *qerr.TransportError
	var applicationErr *qerr.ApplicationError
	if errors.As(e, &transportErr) {
		packet, err = c.packer.PackConnectionClose(transportErr, c.maxPacketSize(), c.version)
	} else if errors.As(e, &applicationErr) {
		packet, err = c.packer.PackApplicationClose(applicationErr, c.maxPacketSize(), c.version)
	} else {
		packet, err = c.packer.PackConnectionClose(&qerr.TransportError{
			ErrorCode:    qerr.InternalError,
			ErrorMessage: fmt.Sprintf("connection BUG: unspecified error type (msg: %s)", e.Error()),
		}, c.maxPacketSize(), c.version)
	}
	if err != nil {
		return nil, err
	}
	return c.writeConnectionClosePacket(packet)
}

func (c *Conn) maxPacketSize() protocol.ByteCount {
	if c.mtuDiscoverer == nil {
		// Use the configured packet size on the client side.
		// If the server sends a max_udp_payload_size that's smaller than this size, we can ignore this:
		// Apparently the server still processed the (fully padded) Initial packet anyway.
		if c.perspective == protocol.PerspectiveClient {
			return protocol.ByteCount(c.config.InitialPacketSize)
		}
		// On the server side, there's no downside to using 1200 bytes until we received the client's transport
		// parameters:
		// * If the first packet didn't contain the entire ClientHello, all we can do is ACK that packet. We don't
		//   need a lot of bytes for that.
		// * If it did, we will have processed the transport parameters and initialized the MTU discoverer.
		return protocol.MinInitialPacketSize
	}
	return c.mtuDiscoverer.CurrentSize()
}

// AcceptStream returns the next stream opened by the peer, blocking until one is available.
func (c *Conn) AcceptStream(ctx context.Context) (*Stream, error) {
	return c.streamsMap.AcceptStream(ctx)
}

// AcceptUniStream returns the next unidirectional stream opened by the peer, blocking until one is available.
func (c *Conn) AcceptUniStream(ctx context.Context) (*ReceiveStream, error) {
	return c.streamsMap.AcceptUniStream(ctx)
}

// OpenStream opens a new bidirectional QUIC stream.
// There is no signaling to the peer about new streams:
// The peer can only accept the stream after data has been sent on the stream,
// or the stream has been reset or closed.
// When reaching the peer's stream limit, it is not possible to open a new stream until the
// peer raises the stream limit. In that case, a [StreamLimitReachedError] is returned.
func (c *Conn) OpenStream() (*Stream, error) {
	return c.streamsMap.OpenStream()
}

// OpenStreamSync opens a new bidirectional QUIC stream.
// It blocks until a new stream can be opened.
// There is no signaling to the peer about new streams:
// The peer can only accept the stream after data has been sent on the stream,
// or the stream has been reset or closed.
func (c *Conn) OpenStreamSync(ctx context.Context) (*Stream, error) {
	return c.streamsMap.OpenStreamSync(ctx)
}

// OpenUniStream opens a new outgoing unidirectional QUIC stream.
// There is no signaling to the peer about new streams:
// The peer can only accept the stream after data has been sent on the stream,
// or the stream has been reset or closed.
// When reaching the peer's stream limit, it is not possible to open a new stream until the
// peer raises the stream limit. In that case, a [StreamLimitReachedError] is returned.
func (c *Conn) OpenUniStream() (*SendStream, error) {
	return c.streamsMap.OpenUniStream()
}

// OpenUniStreamSync opens a new outgoing unidirectional QUIC stream.
// It blocks until a new stream can be opened.
// There is no signaling to the peer about new streams:
// The peer can only accept the stream after data has been sent on the stream,
// or the stream has been reset or closed.
func (c *Conn) OpenUniStreamSync(ctx context.Context) (*SendStream, error) {
	return c.streamsMap.OpenUniStreamSync(ctx)
}

func (c *Conn) newFlowController(id protocol.StreamID, initialSendWindow protocol.ByteCount) flowcontrol.StreamFlowController {
	return flowcontrol.NewStreamFlowController(
		id,
		c.connFlowController,
		protocol.ByteCount(c.config.InitialStreamReceiveWindow),
		protocol.ByteCount(c.config.MaxStreamReceiveWindow),
		initialSendWindow,
		c.rttStats,
		c.logger,
	)
}

// scheduleSending signals that we have data for sending
func (c *Conn) scheduleSending() {
	select {
	case c.sendingScheduled <- struct{}{}:
	default:
	}
}

// tryQueueingUndecryptablePacket queues a packet for which we're missing the decryption keys.
// The qlogevents.PacketType is only used for logging purposes.
func (c *Conn) tryQueueingUndecryptablePacket(p receivedPacket, pt qlog.PacketType, datagramID qlog.DatagramID) bool {
	if c.handshakeComplete {
		panic("shouldn't queue undecryptable packets after handshake completion")
	}
	if c.pendingUndecryptablePackets() >= protocol.MaxUndecryptablePackets {
		if c.qlogger != nil {
			c.qlogger.RecordEvent(qlog.PacketDropped{
				Header: qlog.PacketHeader{
					PacketType:   pt,
					PacketNumber: protocol.InvalidPacketNumber,
				},
				Raw:        qlog.RawInfo{Length: int(p.Size())},
				DatagramID: datagramID,
				Trigger:    qlog.PacketDropDOSPrevention,
			})
		}
		c.logger.Infof("Dropping undecryptable packet (%d bytes). Undecryptable packet queue full.", p.Size())
		return false
	}
	c.logger.Infof("Queueing packet (%d bytes) for later decryption", p.Size())
	if c.qlogger != nil {
		c.qlogger.RecordEvent(qlog.PacketBuffered{
			Header: qlog.PacketHeader{
				PacketType:   pt,
				PacketNumber: protocol.InvalidPacketNumber,
			},
			Raw:        qlog.RawInfo{Length: int(p.Size())},
			DatagramID: datagramID,
		})
	}
	c.undecryptablePackets = append(c.undecryptablePackets, receivedPacketWithDatagramID{receivedPacket: p, datagramID: datagramID})
	return true
}

func (c *Conn) queueControlFrame(f wire.Frame) {
	c.framer.QueueControlFrame(f)
	c.scheduleSending()
}

func (c *Conn) onHasConnectionData() { c.scheduleSending() }

func (c *Conn) onHasStreamData(id protocol.StreamID, str *SendStream) {
	c.framer.AddActiveStream(id, str)
	c.scheduleSending()
}

func (c *Conn) onHasStreamControlFrame(id protocol.StreamID, str streamControlFrameGetter) {
	c.framer.AddStreamWithControlFrames(id, str)
	c.scheduleSending()
}

func (c *Conn) onStreamCompleted(id protocol.StreamID) {
	if err := c.streamsMap.DeleteStream(id); err != nil {
		c.closeLocal(err)
	}
	c.framer.RemoveActiveStream(id)
}

// SendDatagram sends a message using a QUIC datagram, as specified in RFC 9221,
// if the peer enabled datagram support.
// There is no delivery guarantee for DATAGRAM frames, they are not retransmitted if lost.
// The payload of the datagram needs to fit into a single QUIC packet.
// In addition, a datagram may be dropped before being sent out if the available packet size suddenly decreases.
// If the payload is too large to be sent at the current time, a DatagramTooLargeError is returned.
func (c *Conn) SendDatagram(p []byte) error {
	return c.SendDatagramContext(context.Background(), p)
}

// SendDatagramContext cancels pending queue admission without a helper task.
// Success transfers the frame to QUIC; later cancellation does not revoke it or
// prove delivery. Session sealing/deadlines use an original DatagramSendScope.
func (c *Conn) SendDatagramContext(ctx context.Context, p []byte) error {
	return c.sendDatagram(ctx, nil, p)
}

func (c *Conn) sendDatagram(ctx context.Context, scope *DatagramSendScope, p []byte) error {
	if ctx == nil {
		return errors.New("nil DATAGRAM context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	params := c.peerSendParams.Load()
	if err := params.datagramError(len(p)); err != nil {
		return err
	}

	// The payload size estimate is conservative.
	// Under many circumstances we could send a few more bytes.
	maxDataLen := min(
		params.maxDatagramPayloadSize,
		protocol.ByteCount(c.currentMTUEstimate.Load()),
	)
	if protocol.ByteCount(len(p)) > maxDataLen {
		return &DatagramTooLargeError{MaxDatagramPayloadSize: int64(maxDataLen)}
	}
	return c.datagramQueue.addCopy(ctx, scope, p)
}

// ReceiveDatagram gets a message received in a QUIC datagram, as specified in RFC 9221.
func (c *Conn) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	if !c.config.EnableDatagrams {
		return nil, errors.New("datagram support disabled")
	}
	return c.datagramQueue.Receive(ctx)
}

// LocalAddr returns the local address of the QUIC connection.
func (c *Conn) LocalAddr() net.Addr { return c.currentSendConn().LocalAddr() }

// RemoteAddr returns the remote address of the QUIC connection.
func (c *Conn) RemoteAddr() net.Addr { return c.currentSendConn().RemoteAddr() }

// getPathManager lazily initializes the Conn's pathManagerOutgoing.
// May create multiple pathManagerOutgoing objects if called concurrently.
func (c *Conn) getPathManager() *pathManagerOutgoing {
	old := c.pathManagerOutgoing.Load()
	if old != nil {
		// Path manager is already initialized
		return old
	}

	// Initialize the path manager
	new := newPathManagerOutgoing(
		c.connIDManager.GetConnIDForPath,
		c.connIDManager.RetireConnIDForPath,
		c.scheduleSending,
	)
	new.closing, new.done = c.Closing(), c.Context().Done()
	if c.pathManagerOutgoing.CompareAndSwap(old, new) {
		return new
	}

	// Swap failed. A concurrent writer wrote first, use their value.
	return c.pathManagerOutgoing.Load()
}

func (c *Conn) AddPath(t *Transport) (*Path, error) {
	if c.perspective == protocol.PerspectiveServer {
		return nil, errors.New("server cannot initiate connection migration")
	}
	if err := c.readinessError(); err != nil {
		return nil, err
	}
	params := c.peerSendParams.Load()
	if !c.handshakeSucceeded.Load() || params == nil || params.phase != peerParametersApplied {
		return nil, errPathHandshakePending
	}
	if params.disableActiveMigration {
		return nil, errors.New("server disabled connection migration")
	}
	if t == nil {
		return nil, errPathTransportMissing
	}
	if err := t.init(false); err != nil {
		return nil, err
	}
	if err := c.readinessError(); err != nil {
		return nil, err
	}
	return c.getPathManager().NewPath(
		t,
		200*time.Millisecond, // initial RTT estimate
		func() error {
			return t.registerPathConnection(c)
		},
	)
}

// HandshakeComplete blocks until the handshake completes (or fails).
// For the client, data sent before completion of the handshake is encrypted with 0-RTT keys.
// For the server, data sent before completion of the handshake is encrypted with 1-RTT keys,
// however the client's identity is only verified once the handshake completes.
func (c *Conn) HandshakeComplete() <-chan struct{} {
	return c.handshakeCompleteChan
}

// QlogTrace returns the qlog trace of the QUIC connection.
// It is nil if qlog is not enabled.
func (c *Conn) QlogTrace() qlogwriter.Trace {
	return c.qlogTrace
}

// NextConnection transitions a connection to be usable after a 0-RTT rejection.
// It waits for the handshake to complete and then enables the connection for normal use.
// This should be called when the server rejects 0-RTT and the application receives
// [Err0RTTRejected] errors.
//
// Note that 0-RTT rejection invalidates all data sent in 0-RTT packets. It is the
// application's responsibility to handle this (for example by resending the data).
func (c *Conn) NextConnection(ctx context.Context) (*Conn, error) {
	// The handshake might fail after the server rejected 0-RTT.
	// This could happen if the Finished message is malformed or never received.
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-c.Context().Done():
		return nil, context.Cause(c.ctx)
	case <-c.HandshakeComplete():
	}
	if cause := context.Cause(ctx); cause != nil {
		return nil, cause
	}
	if !c.handshakeSucceeded.Load() {
		// Completion can precede the original terminal cleanup. Join its cause;
		// a failed handshake never grants access to reset stream maps.
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case <-c.ctx.Done():
			return nil, context.Cause(c.ctx)
		}
	}
	if err := c.readinessError(); err != nil {
		return nil, err
	}
	if err := c.streamsMap.UseResetMaps(); err != nil {
		return nil, err
	}
	return c, nil
}

// estimateMaxPayloadSize estimates the maximum payload size for short header packets.
// It is not very sophisticated: it just subtracts the size of header (assuming the maximum
// connection ID length), and the size of the encryption tag.
func estimateMaxPayloadSize(mtu protocol.ByteCount) protocol.ByteCount {
	return mtu - 1 /* type byte */ - 20 /* maximum connection ID length */ - 16 /* tag size */
}

// SetCongestionControl replaces the current congestion control algorithm.
// A missing, overlapping, closed or abnormal setup requests connection close;
// this legacy void API must not silently imply that a refused change applied.
func (c *Conn) SetCongestionControl(cc congestion.CongestionControl) {
	call, err := c.congestionCalls.begin(cc, c.sentPacketHandler)
	if err != nil {
		c.destroyImpl(err)
		return
	}
	returned := false
	defer c.congestionCalls.finish(call, &returned) // last defer: includes close request
	defer func() {
		if !returned {
			c.destroyImpl(ackhandler.ErrCongestionChangeUncertain)
		}
	}()
	call.stage = congestionCallSetup
	call.result = call.handler.SetCongestionControl(call.controller)
	call.stage = congestionCallReturned
	if call.result != nil {
		call.stage = congestionCallClosing
		c.destroyImpl(call.result)
	}
	returned = true
}

// SetRemoteAddr Replace the current remote addr with a new one
func (c *Conn) SetRemoteAddr(addr net.Addr) {
	c.currentSendConn().SetRemoteAddr(addr)
}

// Config Return current config
func (c *Conn) Config() *Config {
	return c.config
}
