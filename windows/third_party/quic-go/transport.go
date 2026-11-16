package quic

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/internal/wire"
	"github.com/sagernet/quic-go/packetaddr"
	"github.com/sagernet/quic-go/qlog"
	"github.com/sagernet/quic-go/qlogwriter"
)

// ErrTransportClosed is returned by the [Transport]'s Listen or Dial method after it was closed.
var ErrTransportClosed = &errTransportClosed{}

type errTransportClosed struct {
	err error
}

func (e *errTransportClosed) Unwrap() []error { return []error{net.ErrClosed, e.err} }

func (e *errTransportClosed) Error() string {
	if e.err == nil {
		return "quic: transport closed"
	}
	return fmt.Sprintf("quic: transport closed: %s", e.err)
}

func (e *errTransportClosed) Is(target error) bool {
	_, ok := target.(*errTransportClosed)
	return ok
}

var errListenerAlreadySet = errors.New("listener already set")

type closePacket struct {
	owner *closedCacheOwner
	addr  net.Addr
	info  packetInfo
}

// The Transport is the central point to manage incoming and outgoing QUIC connections.
// QUIC demultiplexes connections based on their QUIC Connection IDs, not based on the 4-tuple.
// This means that a single UDP socket can be used for listening for incoming connections, as well as
// for dialing an arbitrary number of outgoing connections.
// A Transport handles a single net.PacketConn, and offers a range of configuration options
// compared to the simple helper functions like [Listen] and [Dial] that this package provides.
type Transport struct {
	// A single net.PacketConn can only be handled by one Transport.
	// Bad things will happen if passed to multiple Transports.
	//
	// A number of optimizations will be enabled if the connections implements the OOBCapablePacketConn interface,
	// as a *net.UDPConn does.
	// 1. It enables the Don't Fragment (DF) bit on the IP header.
	//    This is required to run DPLPMTUD (Path MTU Discovery, RFC 8899).
	// 2. It enables reading of the ECN bits from the IP header.
	//    This allows the remote node to speed up its loss detection and recovery.
	// 3. It uses batched syscalls (recvmmsg) to more efficiently receive packets from the socket.
	// 4. It uses Generic Segmentation Offload (GSO) to efficiently send batches of packets (on Linux).
	//
	// After passing the connection to the Transport, it's invalid to call ReadFrom or WriteTo on the connection.
	Conn net.PacketConn

	// The length of the connection ID in bytes.
	// It can be any value between 1 and 20.
	// Due to the increased risk of collisions, it is not recommended to use connection IDs shorter than 4 bytes.
	// If unset, a 4 byte connection ID will be used.
	ConnectionIDLength int

	// Use for generating new connection IDs.
	// This allows the application to control of the connection IDs used,
	// which allows routing / load balancing based on connection IDs.
	// All Connection IDs returned by the ConnectionIDGenerator MUST
	// have the same length.
	ConnectionIDGenerator ConnectionIDGenerator

	// The StatelessResetKey is used to generate stateless reset tokens.
	// If no key is configured, sending of stateless resets is disabled.
	// It is highly recommended to configure a stateless reset key, as stateless resets
	// allow the peer to quickly recover from crashes and reboots of this node.
	// See section 10.3 of RFC 9000 for details.
	StatelessResetKey *StatelessResetKey

	// The TokenGeneratorKey is used to encrypt session resumption tokens.
	// If no key is configured, a random key will be generated.
	// If multiple servers are authoritative for the same domain, they should use the same key,
	// see section 8.1.3 of RFC 9000 for details.
	TokenGeneratorKey *TokenGeneratorKey

	// MaxTokenAge is the maximum age of the resumption token presented during the handshake.
	// These tokens allow skipping address resumption when resuming a QUIC connection,
	// and are especially useful when using 0-RTT.
	// If not set, it defaults to 24 hours.
	// See section 8.1.3 of RFC 9000 for details.
	MaxTokenAge time.Duration

	// DisableVersionNegotiationPackets disables the sending of Version Negotiation packets.
	// This can be useful if version information is exchanged out-of-band.
	// It has no effect for clients.
	DisableVersionNegotiationPackets bool

	// VerifySourceAddress decides if a connection attempt originating from unvalidated source
	// addresses first needs to go through source address validation using QUIC's Retry mechanism,
	// as described in RFC 9000 section 8.1.2.
	// Note that the address passed to this callback is unvalidated, and might be spoofed in case
	// of an attack.
	// Validating the source address adds one additional network roundtrip to the handshake,
	// and should therefore only be used if a suspiciously high number of incoming connection is recorded.
	// For most use cases, wrapping the Allow function of a rate.Limiter will be a reasonable
	// implementation of this callback (negating its return value).
	VerifySourceAddress func(net.Addr) bool

	// ConnContext is called when the server accepts a new connection. To reject a connection return
	// a non-nil error.
	// The context is closed when the connection is closed, or when the handshake fails for any reason.
	// The context returned from the callback is used to derive every other context used during the
	// lifetime of the connection:
	// * the context passed to crypto/tls (and used on the tls.ClientHelloInfo)
	// * the context used in Config.QlogTrace
	// * the context returned from Conn.Context
	// * the context returned from SendStream.Context
	// It is not used for dialed connections.
	ConnContext func(context.Context, *ClientInfo) (context.Context, error)

	// A Tracer traces events that don't belong to a single QUIC connection.
	// Recorder.Close is called when the transport is closed.
	Tracer qlogwriter.Recorder

	mutex       sync.Mutex
	handlers    map[protocol.ConnectionID]packetHandler
	resetTokens map[protocol.StatelessResetToken]packetHandler

	initOnce sync.Once
	initErr  error
	life     transportLifecycle

	// If no ConnectionIDGenerator is set, this is the ConnectionIDLength.
	connIDLen int
	// Set in init.
	// If no ConnectionIDGenerator is set, this is set to a default.
	connIDGenerator   ConnectionIDGenerator
	statelessResetter *statelessResetter

	server *baseServer

	conn rawConn

	closeQueue          chan closePacket
	statelessResetQueue chan receivedPacket
	closedCache         closedCacheBudget
	output              transportOutputState

	listening   chan struct{} // is closed when listen returns
	closeErr    error
	createdConn bool
	isSingleUse bool // was created for a single server or client, i.e. by calling quic.Listen or quic.Dial

	readingNonQUICPackets atomic.Bool
	nonQUICPackets        chan receivedPacket

	logger utils.Logger
}

// Listen starts listening for incoming QUIC connections.
// There can only be a single listener on any net.PacketConn.
// Listen may only be called again after the current listener was closed.
func (t *Transport) Listen(tlsConf *tls.Config, conf *Config) (*Listener, error) {
	s, err := t.createServer(tlsConf, conf, false)
	if err != nil {
		return nil, err
	}
	return &Listener{baseServer: s}, nil
}

// ListenEarly starts listening for incoming QUIC connections.
// There can only be a single listener on any net.PacketConn.
// ListenEarly may only be called again after the current listener was closed.
func (t *Transport) ListenEarly(tlsConf *tls.Config, conf *Config) (*EarlyListener, error) {
	s, err := t.createServer(tlsConf, conf, true)
	if err != nil {
		return nil, err
	}
	return &EarlyListener{baseServer: s}, nil
}

func (t *Transport) createServer(tlsConf *tls.Config, conf *Config, allow0RTT bool) (*baseServer, error) {
	if tlsConf == nil {
		return nil, errors.New("quic: tls.Config not set")
	}
	opening, err := t.reserveServerOpening(tlsConf, conf)
	if err != nil {
		return nil, err
	}
	returned := false
	defer t.finishServerOpening(opening, &returned)
	if err := validateConfig(conf); err != nil {
		returned = true
		return nil, err
	}
	if opening.previous != nil && !opening.previous.closeRetired() {
		returned = true
		return nil, errListenerAlreadySet
	}
	opening.config = populateConfig(conf)
	if err := t.init(false); err != nil {
		returned = true
		return nil, err
	}
	if t.transportSealed() {
		returned = true
		return nil, ErrTransportClosed
	}
	opening.raw = t.conn // retained before constructor/logger/address callbacks
	maxTokenAge := t.MaxTokenAge
	if maxTokenAge == 0 {
		maxTokenAge = 24 * time.Hour
	}
	s := newServer(
		opening,
		opening.raw,
		(*packetHandlerMap)(t),
		t.connIDGenerator,
		t.statelessResetter,
		t.ConnContext,
		opening.tlsConf,
		opening.config,
		t.Tracer,
		t.closeServer,
		*t.TokenGeneratorKey,
		maxTokenAge,
		t.VerifySourceAddress,
		t.DisableVersionNegotiationPackets,
		allow0RTT,
	)
	opening.addr = opening.raw.LocalAddr()
	opening.local, err = packetaddr.Capture(opening.addr)
	if err != nil {
		returned = true // the candidate has no workers or packet borrowers yet
		return nil, err
	}
	local := opening.local.Addr()
	s.logger.Debugf("Listening for %s connections on %s", local.Network(), local.String())
	if err := t.publishServerOpening(opening); err != nil {
		returned = err != errPacketOutputUncertain
		return nil, err
	}
	returned = true
	return s, nil
}

// Dial dials a new connection to a remote host (not using 0-RTT).
func (t *Transport) Dial(ctx context.Context, addr net.Addr, tlsConf *tls.Config, conf *Config) (*Conn, error) {
	return t.dial(ctx, addr, "", tlsConf, conf, false)
}

// DialEarly dials a new connection, attempting to use 0-RTT if possible.
func (t *Transport) DialEarly(ctx context.Context, addr net.Addr, tlsConf *tls.Config, conf *Config) (*Conn, error) {
	return t.dial(ctx, addr, "", tlsConf, conf, true)
}

func (t *Transport) dial(ctx context.Context, addr net.Addr, host string, tlsConf *tls.Config, conf *Config, use0RTT bool) (*Conn, error) {
	opening, err := t.prepareClientDial(ctx, addr, tlsConf, conf)
	if err != nil {
		return nil, err
	}
	return t.doDial(opening,
		0,
		false,
		use0RTT,
		opening.config.Versions[0],
	)
}

func (t *Transport) doDial(
	opening *clientConnectionOpening,
	initialPacketNumber protocol.PacketNumber,
	hasNegotiatedVersion bool,
	use0RTT bool,
	version protocol.Version,
) (*Conn, error) {
	// The constructor releases its opening cell before waiting for the handshake.
	// Preserve these original inputs: that fixed cell can then serve a later call.
	ctx, addr, sendConn := opening.context, opening.addr, opening.conn
	tlsConf, config := opening.tlsConf, opening.config
	conn, errChan, recreateChan, err := t.startClientConnection(opening, initialPacketNumber, hasNegotiatedVersion, use0RTT, version)
	if err != nil {
		return nil, err
	}

	// Only set when we're using 0-RTT.
	// Otherwise, earlyConnChan will be nil. Receiving from a nil chan blocks forever.
	var earlyConnChan <-chan struct{}
	if use0RTT {
		earlyConnChan = conn.earlyConnReady()
	}
	handshakeChan := conn.HandshakeComplete()
	cancelDial := func(cause error) (*Conn, error) {
		conn.destroy(cause)
		// wait until the Go routine that called Conn.run() returns
		select {
		case <-errChan:
		case <-recreateChan:
		}
		return nil, cause
	}
	for {
		if cause := context.Cause(ctx); cause != nil {
			return cancelDial(cause)
		}
		select {
		case <-ctx.Done():
			return cancelDial(context.Cause(ctx))
		case params := <-recreateChan:
			next, err := t.reserveClientOpening(ctx, addr, tlsConf, config)
			if err != nil {
				return nil, err
			}
			next.raw, next.conn = t.conn, sendConn
			return t.doDial(next,
				params.nextPacketNumber,
				true,
				use0RTT,
				params.nextVersion,
			)
		case err := <-errChan:
			return nil, err
		case <-earlyConnChan:
			if context.Cause(ctx) != nil {
				continue
			}
			if conn.readinessError() != nil {
				earlyConnChan, handshakeChan = nil, nil
				continue // join the original loop's error / recreation result
			}
			return conn.Conn, nil
		case <-handshakeChan:
			if context.Cause(ctx) != nil {
				continue
			}
			if !conn.handshakeSucceeded.Load() || conn.readinessError() != nil {
				earlyConnChan, handshakeChan = nil, nil
				continue
			}
			return conn.Conn, nil
		}
	}
}

func (t *Transport) initializeTransport(allowZeroLengthConnIDs bool, opening *transportConnOpening) error {
	// Validate before wrapping can tune the original socket. Retain the exact
	// generator before its one length callback; invalid input starts no workers.
	t.connIDGenerator = t.ConnectionIDGenerator
	if t.connIDGenerator != nil {
		t.connIDLen = t.connIDGenerator.ConnectionIDLen()
	} else {
		t.connIDLen = t.ConnectionIDLength
		if t.connIDLen == 0 && !allowZeroLengthConnIDs {
			t.connIDLen = protocol.DefaultConnectionIDLength
		}
		if t.connIDLen >= 0 && t.connIDLen <= protocol.MaxConnIDLen {
			t.connIDGenerator = &protocol.DefaultConnectionIDGenerator{ConnLen: t.connIDLen}
		}
	}
	if t.connIDLen < 0 || t.connIDLen > protocol.MaxConnIDLen {
		return protocol.ErrInvalidConnectionIDLen
	}
	if t.transportSealed() {
		return net.ErrClosed
	}
	var conn rawConn
	if c, ok := opening.socket.(rawConn); ok {
		conn = c
		opening.wrapped = c
	} else {
		var err error
		conn, err = wrapConn(opening)
		if err != nil {
			return err
		}
	}

	t.logger = utils.DefaultLogger // TODO: make this configurable
	t.conn = conn
	t.handlers = make(map[protocol.ConnectionID]packetHandler)
	t.resetTokens = make(map[protocol.StatelessResetToken]packetHandler)
	t.listening = make(chan struct{})

	t.closeQueue = make(chan closePacket, 4)
	t.statelessResetQueue = make(chan receivedPacket, 4)
	t.nonQUICPackets = make(chan receivedPacket, maxQueuedNonQUICPackets)
	if t.TokenGeneratorKey == nil {
		var key TokenGeneratorKey
		if _, err := rand.Read(key[:]); err != nil {
			return err
		}
		t.TokenGeneratorKey = &key
	}

	t.statelessResetter = newStatelessResetter(t.StatelessResetKey)
	t.closedCache = closedCacheBudget{owners: make(map[*closedCacheOwner]struct{}), done: make(chan struct{})}
	t.output.done = make(chan struct{})
	t.output.wake = make(chan struct{}, 1)

	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.sealed || o.failed {
		return net.ErrClosed
	}
	o.loopsScheduled = true
	go t.runTransportListener(conn)
	go t.runSendQueue()
	return nil
}

func (t *Transport) runSendQueue() {
	returned := false
	defer t.finishTransportOutput(&returned)
	for {
		select {
		case <-t.listening:
			returned = true
			return
		case <-t.output.wake:
			t.stopClosedRead()
		case p := <-t.closeQueue:
			if !t.writeClosedRetry(p) {
				t.close(errPacketOutputUncertain)
				returned = true
				return
			}
		case p := <-t.statelessResetQueue:
			if !t.writeStatelessReset(p) {
				t.close(errPacketOutputUncertain)
				returned = true
				return
			}
		}
	}
}

func (t *Transport) closeServer() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.server = nil
	if t.isSingleUse {
		t.closeErr = ErrServerClosed
	}

	if len(t.handlers) == 0 {
		t.maybeStopListening()
	}
}

func (t *Transport) listen(conn rawConn) {
	for {
		p, err := t.readTransportPacket(conn)
		if errors.Is(err, errPacketInputUncertain) {
			t.failTransportInput()
			t.close(err)
			return
		}
		if err == nil && len(p.data) > 0 && p.buffer == nil {
			t.failTransportInput()
			t.close(errPacketInputUncertain)
			return
		}
		//nolint:staticcheck // SA1019 ignore this!
		// TODO: This code is used to ignore wsa errors on Windows.
		// Since net.Error.Temporary is deprecated as of Go 1.18, we should find a better solution.
		// See https://github.com/quic-go/quic-go/issues/1737 for details.
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			t.mutex.Lock()
			closed := t.closeErr != nil || t.transportSealed()
			t.mutex.Unlock()
			if closed {
				t.discardTransportPacket()
				return
			}
			t.logger.Debugf("Temporary error reading from conn: %w", err)
			t.discardTransportPacket()
			continue
		}
		if err != nil {
			// Windows returns an error when receiving a UDP datagram that doesn't fit into the provided buffer.
			if isRecvMsgSizeErr(err) {
				t.discardTransportPacket()
				continue
			}
			t.discardTransportPacket()
			t.close(err)
			return
		}
		t.handlePacket(p)
		t.clearTransportPacket() // original handler / drop returned normally
	}
}

func (t *Transport) maybeStopListening() {
	if t.isSingleUse && t.closeErr != nil {
		t.requestClosedReadStopLocked()
	}
}

func (t *Transport) handlePacket(p receivedPacket) {
	if len(p.data) == 0 {
		releaseClosedPacket(&p)
		return
	}
	if !wire.IsPotentialQUICPacket(p.data[0]) && !wire.IsLongHeaderPacket(p.data[0]) {
		t.handleNonQUICPacket(p)
		return
	}
	connID, err := wire.ParseConnectionID(p.data, t.connIDLen)
	if err != nil {
		t.logger.Debugf("error parsing connection ID on packet from %s: %s", p.remoteAddr, err)
		if t.Tracer != nil {
			t.Tracer.RecordEvent(qlog.PacketDropped{
				Raw:     qlog.RawInfo{Length: int(p.Size())},
				Trigger: qlog.PacketDropHeaderParseError,
			})
		}
		releaseClosedPacket(&p)
		return
	}

	// If there's a connection associated with the connection ID, pass the packet there.
	if handler, ok := (*packetHandlerMap)(t).Get(connID); ok {
		handler.handlePacket(p)
		return
	}
	// RFC 9000 section 10.3.1 requires that the stateless reset detection logic is run for both
	// packets that cannot be associated with any connections, and for packets that can't be decrypted.
	// We deviate from the RFC and ignore the latter: If a packet's connection ID is associated with an
	// existing connection, it is dropped there if if it can't be decrypted.
	// Stateless resets use random connection IDs, and at reasonable connection ID lengths collisions are
	// exceedingly rare. In the unlikely event that a stateless reset is misrouted to an existing connection,
	// it is to be expected that the next stateless reset will be correctly detected.
	if isStatelessReset := t.maybeHandleStatelessReset(p.data); isStatelessReset {
		releaseClosedPacket(&p)
		return
	}
	if !wire.IsLongHeaderPacket(p.data[0]) {
		if statelessResetQueued := t.maybeSendStatelessReset(p); !statelessResetQueued {
			if t.Tracer != nil {
				t.Tracer.RecordEvent(qlog.PacketDropped{
					Header:  qlog.PacketHeader{PacketType: qlog.PacketType1RTT},
					Raw:     qlog.RawInfo{Length: int(p.Size())},
					Trigger: qlog.PacketDropUnknownConnectionID,
				})
			}
			releaseClosedPacket(&p)
		}
		return
	}

	t.mutex.Lock()
	server := t.server
	t.mutex.Unlock()
	if server == nil { // no server set
		t.logger.Debugf("received a packet with an unexpected connection ID %s", connID)
		if t.Tracer != nil {
			t.Tracer.RecordEvent(qlog.PacketDropped{
				Raw:     qlog.RawInfo{Length: int(p.Size())},
				Trigger: qlog.PacketDropUnknownConnectionID,
			})
		}
		releaseClosedPacket(&p)
		return
	}
	server.handlePacket(p) // original server admission shares its own seal fence
}

func (t *Transport) maybeSendStatelessReset(p receivedPacket) (statelessResetQueued bool) {
	if t.StatelessResetKey == nil {
		return false
	}

	// Don't send a stateless reset in response to very small packets.
	// This includes packets that could be stateless resets.
	if len(p.data) <= protocol.MinStatelessResetSize {
		return false
	}
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.closeErr != nil || t.output.sealed {
		return false
	}

	select {
	case t.statelessResetQueue <- p:
		return true
	default:
		// it's fine to not send a stateless reset when we're busy
		return false
	}
}

func (t *Transport) maybeHandleStatelessReset(data []byte) bool {
	// stateless resets are always short header packets
	if wire.IsLongHeaderPacket(data[0]) {
		return false
	}
	if len(data) < 17 /* type byte + 16 bytes for the reset token */ {
		return false
	}

	token := protocol.StatelessResetToken(data[len(data)-16:])
	t.mutex.Lock()
	conn, ok := t.resetTokens[token]
	t.mutex.Unlock()

	if ok {
		t.logger.Debugf("Received a stateless reset. Closing connection.")
		requestStatelessReset(conn)
		return true
	}
	return false
}

func (t *Transport) handleNonQUICPacket(p receivedPacket) {
	o := t.lifecycle()
	o.access.Lock()
	queued := false
	traceDrop := !o.sealed && !o.inputs.sealed && t.readingNonQUICPackets.Load()
	if traceDrop {
		select {
		case t.nonQUICPackets <- p:
			queued = true
		default:
		}
	}
	o.access.Unlock()
	if !queued {
		if traceDrop && t.Tracer != nil {
			t.Tracer.RecordEvent(qlog.PacketDropped{
				Raw:     qlog.RawInfo{Length: int(p.Size())},
				Trigger: qlog.PacketDropDOSPrevention,
			})
		}
		releaseClosedPacket(&p)
	}
}

const maxQueuedNonQUICPackets = 32

func setTLSConfigServerName(tlsConf *tls.Config, addr net.Addr, host string) {
	// If no ServerName is set, infer the ServerName from the host we're connecting to.
	if tlsConf.ServerName != "" {
		return
	}
	if host == "" {
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			tlsConf.ServerName = udpAddr.IP.String()
			return
		}
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil { // This happens if the host doesn't contain a port number.
		tlsConf.ServerName = host
		return
	}
	tlsConf.ServerName = h
}

type packetHandlerMap Transport

var _ connRunner = &packetHandlerMap{}

func (h *packetHandlerMap) Add(id protocol.ConnectionID, handler packetHandler) bool /* was added */ {
	h.mutex.Lock()
	o := (*Transport)(h).lifecycle()
	o.access.Lock()
	if handler == nil || id.Len() != h.connIDLen || h.closeErr != nil || o.sealed || o.failed || o.clientCIDReservedLocked(id) {
		o.access.Unlock()
		h.mutex.Unlock()
		return false
	}

	if current, ok := h.handlers[id]; ok {
		sameOwner := closedSourceMatches(current, handler)
		o.access.Unlock()
		h.mutex.Unlock()
		return sameOwner
	}
	h.handlers[id] = handler
	o.access.Unlock()
	h.mutex.Unlock()
	h.logger.Debugf("Adding connection ID %s.", id)
	return true
}

func (h *packetHandlerMap) Get(connID protocol.ConnectionID) (packetHandler, bool) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	handler, ok := h.handlers[connID]
	return handler, ok
}

func (h *packetHandlerMap) AddResetToken(token protocol.StatelessResetToken, handler packetHandler) bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	o := (*Transport)(h).lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if handler == nil || h.closeErr != nil || o.sealed || o.failed {
		return false
	}
	if current, exists := h.resetTokens[token]; exists {
		return closedSourceMatches(current, handler)
	}
	h.resetTokens[token] = handler
	return true
}

func (h *packetHandlerMap) RemoveResetToken(original packetHandler, token protocol.StatelessResetToken) {
	h.mutex.Lock()
	if closedSourceMatches(h.resetTokens[token], original) {
		delete(h.resetTokens, token)
	}
	h.mutex.Unlock()
}

func (h *packetHandlerMap) AddWithConnID(clientDestConnID, newConnID protocol.ConnectionID, handler packetHandler) bool {
	h.mutex.Lock()
	o := (*Transport)(h).lifecycle()
	o.access.Lock()
	if handler == nil || clientDestConnID == newConnID || newConnID.Len() != h.connIDLen || h.closeErr != nil || o.sealed || o.failed || o.clientCIDReservedLocked(clientDestConnID) || o.clientCIDReservedLocked(newConnID) {
		o.access.Unlock()
		h.mutex.Unlock()
		return false
	}

	if _, ok := h.handlers[clientDestConnID]; ok {
		o.access.Unlock()
		h.mutex.Unlock()
		h.logger.Debugf("Not adding connection ID %s for a new connection, as it already exists.", clientDestConnID)
		return false
	}
	if _, exists := h.handlers[newConnID]; exists {
		o.access.Unlock()
		h.mutex.Unlock()
		return false
	}
	h.handlers[clientDestConnID] = handler
	h.handlers[newConnID] = handler
	o.access.Unlock()
	h.mutex.Unlock()
	h.logger.Debugf("Adding connection IDs %s and %s for a new connection.", clientDestConnID, newConnID)
	return true
}

func (h *packetHandlerMap) Remove(original packetHandler, id protocol.ConnectionID) {
	h.mutex.Lock()
	if !closedSourceMatches(h.handlers[id], original) {
		h.mutex.Unlock()
		return
	}
	delete(h.handlers, id)
	h.mutex.Unlock()
	h.logger.Debugf("Removing connection ID %s.", id)
}

// ReplaceWithClosed is called when a connection is closed.
// Depending on which side closed the connection, we need to:
// * remote close: absorb delayed packets
// * local close: retransmit the CONNECTION_CLOSE packet, in case it was lost
func (h *packetHandlerMap) ReplaceWithClosed(original packetHandler, ids []protocol.ConnectionID, connClosePacket []byte, expiry time.Duration) {
	(*Transport)(h).replaceClosedCache(original, ids, connClosePacket, expiry)
}
