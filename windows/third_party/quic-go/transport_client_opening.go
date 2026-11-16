package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"net"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/qlogwriter"
)

var errClientTLSConfig = errors.New("quic: nil TLS configuration")

// Lifecycle.access guards active/reserved/ID and the opening count. The original
// call alone owns the remaining prefix until promotion or actual retirement.
// Transport.mutex precedes access whenever the CID table is involved. Neither
// metadata lock is held across context, trace, provider or constructor callbacks.
type clientConnectionOpening struct {
	active, reserved bool
	srcConnID        protocol.ConnectionID
	context          context.Context
	addr             net.Addr
	tlsConf          *tls.Config
	config           *Config
	raw              rawConn
	conn             sendConn
	sendOpening      sendConnOpening
	trace            qlogwriter.Trace
	connection       *Conn
}

// The initial transport retains each promoted client independently of CID maps:
// an original loop can remove its IDs before its registered calls have retired.
// lifecycle.access guards this intrusive list; no failure-time holder is allocated.
type clientConnectionLifetime struct {
	next       *Conn
	registered bool
}

func (o *transportLifecycle) retainClientLocked(conn *Conn) {
	if conn.clientLifetime.registered {
		return
	}
	conn.clientLifetime.next, conn.clientLifetime.registered = o.clients, true
	o.clients = conn
}

func (t *Transport) releaseClientConnection(conn *Conn) bool {
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	for link := &o.clients; *link != nil; link = &(*link).clientLifetime.next {
		if *link == conn {
			*link = conn.clientLifetime.next
			conn.clientLifetime = clientConnectionLifetime{}
			return true
		}
	}
	return false
}

func (t *Transport) failClientConnection(conn *Conn) {
	o := t.lifecycle()
	o.access.Lock()
	o.retainClientLocked(conn)
	o.failed, o.sealed = true, true
	o.settleCallsLocked()
	o.access.Unlock()
}

func (o *transportLifecycle) settleClientOpeningsLocked() {
	if o.sealed && !o.openingsSettled && (o.failed || o.openingCount == 0) {
		o.openingsSettled = true
		close(o.openingsDone) // failure is negative, not proof that callbacks stopped
	}
}

func (t *Transport) reserveClientOpening(ctx context.Context, addr net.Addr, tlsConf *tls.Config, config *Config) (*clientConnectionOpening, error) {
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed {
		return nil, errPacketOutputUncertain
	}
	if o.sealed {
		return nil, ErrTransportClosed
	}
	if o.openingCount >= len(o.openings) {
		return nil, errTransportCallCapacity
	}
	for i := range o.openings {
		call := &o.openings[i]
		if call.active {
			continue
		}
		o.openingCount++ // before init waits, clones and provider callbacks
		*call = clientConnectionOpening{active: true, context: ctx, addr: addr, tlsConf: tlsConf, config: config}
		return call, nil
	}
	return nil, errPacketOutputUncertain
}

func (t *Transport) finishClientOpening(call *clientConnectionOpening, returned *bool) {
	o := t.lifecycle()
	t.mutex.Lock()
	o.access.Lock()
	failed := !*returned || !call.active
	if failed {
		o.failed, o.sealed = true, true // keep the original cell, CID and partial Conn
	} else {
		o.openingCount--
		*call = clientConnectionOpening{}
	}
	o.settleCallsLocked()
	o.access.Unlock()
	t.mutex.Unlock()
	if failed {
		t.abortTransport() // original call; never synchronously joins its own root
	}
}

// The caller holds lifecycle.access. Table mutation additionally holds mutex.
func (o *transportLifecycle) clientCIDReservedLocked(id protocol.ConnectionID) bool {
	for i := range o.openings {
		call := &o.openings[i]
		if call.active && call.reserved && call.srcConnID == id {
			return true
		}
	}
	return false
}

func (t *Transport) reserveClientCID(call *clientConnectionOpening, id protocol.ConnectionID) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.sealed || o.failed || t.closeErr != nil {
		return ErrTransportClosed
	}
	if !call.active || call.reserved || id.Len() != t.connIDLen || o.clientCIDReservedLocked(id) {
		return errCIDRegistration
	}
	if _, exists := t.handlers[id]; exists {
		return errCIDRegistration
	}
	call.srcConnID, call.reserved = id, true
	return nil
}

func (t *Transport) publishClientOpening(call *clientConnectionOpening, conn *wrappedConn) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.sealed || o.failed || t.closeErr != nil {
		return ErrTransportClosed
	}
	if !call.active || !call.reserved || conn == nil || conn.Conn == nil || call.connection != conn.Conn || !conn.construction.complete {
		return errPacketOutputUncertain
	}
	if _, exists := t.handlers[call.srcConnID]; exists {
		return errCIDRegistration
	}
	if conn.clientLifetime.registered {
		return errPacketOutputUncertain
	}
	o.retainClientLocked(conn.Conn)
	t.handlers[call.srcConnID] = conn
	call.reserved = false // same commit fence transfers CID custody to the map
	return nil
}

func (t *Transport) waitClientOpenings() error {
	o := t.lifecycle()
	<-o.openingsDone
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || o.openingCount != 0 {
		return errPacketOutputUncertain
	}
	return nil
}

func (t *Transport) prepareClientDial(ctx context.Context, addr net.Addr, tlsConf *tls.Config, config *Config) (*clientConnectionOpening, error) {
	call, err := t.reserveClientOpening(ctx, addr, tlsConf, config)
	if err != nil {
		return nil, err
	}
	returned, transferred := false, false
	defer func() {
		if !transferred {
			t.finishClientOpening(call, &returned)
		}
	}()
	if cause := context.Cause(ctx); cause != nil {
		returned = true
		return nil, cause
	}
	if tlsConf == nil {
		returned = true
		return nil, errClientTLSConfig
	}
	if err := t.init(t.isSingleUse); err != nil {
		returned = true
		return nil, err
	}
	if err := validateConfig(config); err != nil {
		returned = true
		return nil, err
	}
	call.config = populateConfig(config)
	call.tlsConf = tlsConf.Clone()
	call.raw = t.conn // before newSendConn calls the original LocalAddr provider
	call.conn, err = newSendConn(&call.sendOpening, call.raw, addr, packetInfo{}, utils.DefaultLogger)
	if err != nil {
		returned = true
		return nil, err
	}
	transferred = true // doDial/startClientConnection inherits this exact original
	return call, nil
}

func (t *Transport) startClientConnection(call *clientConnectionOpening, initialPacketNumber protocol.PacketNumber, hasNegotiatedVersion, use0RTT bool, version protocol.Version) (*wrappedConn, <-chan error, <-chan errCloseForRecreating, error) {
	returned := false
	defer t.finishClientOpening(call, &returned)
	if cause := context.Cause(call.context); cause != nil {
		returned = true
		return nil, nil, nil, cause
	}
	srcConnID, err := t.connIDGenerator.GenerateConnectionID()
	if err != nil {
		returned = true
		return nil, nil, nil, err
	}
	destConnID, err := generateConnectionIDForInitial()
	if err != nil {
		returned = true
		return nil, nil, nil, err
	}
	if err := t.reserveClientCID(call, srcConnID); err != nil {
		returned = true
		return nil, nil, nil, err
	}
	if cause := context.Cause(call.context); cause != nil {
		returned = true // no trace producer or partial connection exists yet
		return nil, nil, nil, cause
	}
	if call.config.Tracer != nil {
		call.trace = call.config.Tracer(call.context, true, destConnID)
	}
	logger := utils.DefaultLogger.WithPrefix("client")
	logger.Infof("Starting new connection to %s (%s -> %s), source connection ID %s, destination connection ID %s, version %s", call.tlsConf.ServerName, call.conn.LocalAddr(), call.conn.RemoteAddr(), srcConnID, destConnID, version)
	conn := newClientConnection(
		context.WithoutCancel(call.context), call.conn, (*packetHandlerMap)(t),
		destConnID, srcConnID, t.connIDGenerator, t.statelessResetter,
		call.config, call.tlsConf, initialPacketNumber, use0RTT, hasNegotiatedVersion,
		call.trace, logger, version, call,
	)
	if conn == nil || conn.Conn == nil || call.connection != conn.Conn || !conn.construction.complete {
		return nil, nil, nil, errPacketOutputUncertain
	}
	err = context.Cause(call.context)
	if err == nil {
		err = t.publishClientOpening(call, conn)
	}
	if err != nil {
		// A complete but unregistered candidate owns an actual terminal loop.
		// No new dial may reuse its reserved CID before original cleanup returns.
		conn.destroyImpl(err)
		runErr := conn.run()
		retireErr := retireTransportConn(conn.Conn)
		returned = retireErr == nil
		return nil, nil, nil, errors.Join(err, runErr, retireErr)
	}
	errChan := make(chan error, 1)
	recreateChan := make(chan errCloseForRecreating, 1)
	go t.runClientConnection(conn, errChan, recreateChan)
	returned = true // original map + scheduled loop now own the complete Conn
	return conn, errChan, recreateChan, nil
}

func (t *Transport) runClientConnection(conn *wrappedConn, errChan chan<- error, recreateChan chan<- errCloseForRecreating) {
	published := false
	defer func() {
		if !published {
			t.failClientConnection(conn.Conn)
			errChan <- errPacketOutputUncertain
			t.abortTransport()
		}
	}()
	err := conn.run()
	retireErr := retireTransportConn(conn.Conn)
	if err == nil || retireErr != nil {
		t.failClientConnection(conn.Conn)
		errChan <- errors.Join(errPacketOutputUncertain, err, retireErr)
		published = true
		t.abortTransport()
		return
	}
	var recreateErr *errCloseForRecreating
	if errors.As(err, &recreateErr) {
		if !t.releaseClientConnection(conn.Conn) {
			return // original worker guard retains and publishes a negative outcome
		}
		recreateChan <- *recreateErr
		published = true
		return
	}
	if !t.releaseClientConnection(conn.Conn) {
		return
	}
	if t.isSingleUse {
		if closeErr := t.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	errChan <- err
	published = true
}
