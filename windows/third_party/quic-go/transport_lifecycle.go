package quic

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sagernet/quic-go/qlogwriter"
)

// access protects lifecycle metadata only. Transport.mutex may precede access;
// access never precedes Transport.mutex, native I/O, callbacks or error methods.
// The original calls are retained on failure, including an abnormal Goexit.
type transportLifecycle struct {
	once                                      sync.Once
	access                                    sync.Mutex
	sealed, failed                            bool
	captured                                  bool
	socket                                    net.PacketConn
	ownsSocket                                bool
	tracer                                    qlogwriter.Recorder
	server                                    *baseServer
	initStarted, initReturned, loopsScheduled bool
	initWaiters                               int
	initDone                                  chan struct{}
	initErr                                   error
	connOpening                               transportConnOpening
	listenerReturned                          bool
	abort, root, outer, trace                 transportCloseCall
	handlers                                  []*transportHandlerClose
	writes                                    map[*transportWriteCall]struct{}
	writeCount, writeBytes                    int
	writesDone                                chan struct{}
	writesSettled                             bool
	inputs                                    transportInputState
	openings                                  [maxTransportCalls]clientConnectionOpening
	openingCount                              int
	openingsDone                              chan struct{}
	openingsSettled                           bool
	serverOpening                             transportServerOpening
	serverOpeningDone                         chan struct{}
	serverOpeningSettled                      bool
	clients                                   *Conn
	pathOpenings                              [maxTransportCalls]pathConnectionOpening
	pathOpeningCount                          int
	pathOpeningsDone                          chan struct{}
	pathOpeningsSettled                       bool
	pathConns                                 map[*Conn]struct{}
}

type transportCloseCall struct {
	started, returned bool
	done              chan struct{}
	err               error
}

type transportHandlerClose struct {
	handler  packetHandler
	returned bool
	err      error
}

func retireTransportConn(conn *Conn) error {
	err := conn.WaitForCryptoQueueRetirement()
	if err == nil {
		conn.retirePathConnections()
	}
	return err
}

func (t *Transport) lifecycle() *transportLifecycle {
	o := &t.life
	o.once.Do(func() {
		o.initDone = make(chan struct{})
		o.writesDone = make(chan struct{})
		o.openingsDone = make(chan struct{})
		o.serverOpeningDone = make(chan struct{})
		o.pathOpeningsDone = make(chan struct{})
		o.inputs.closing = make(chan struct{})
		o.inputs.readsDone = make(chan struct{})
		for _, call := range []*transportCloseCall{&o.abort, &o.root, &o.outer, &o.trace, &o.inputs.drain} {
			call.done = make(chan struct{})
		}
	})
	return o
}

func (o *transportLifecycle) captureLocked(t *Transport) {
	if !o.captured {
		o.socket, o.ownsSocket, o.tracer = t.Conn, t.createdConn, t.Tracer
		o.captured = true
	}
}

func (t *Transport) transportSealed() bool {
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	return o.sealed || o.failed
}

func (t *Transport) sealTransport() {
	o := t.lifecycle()
	o.access.Lock()
	o.captureLocked(t)
	o.sealed = true
	o.settleCallsLocked()
	o.access.Unlock()
}

func (t *Transport) finishTransportCall(call *transportCloseCall, returned *bool, result *error) {
	// No errors.Is / Error / Unwrap while holding access.
	err := *result
	if !*returned {
		err = errPacketOutputUncertain
	}
	o := t.lifecycle()
	o.access.Lock()
	call.returned, call.err = *returned, err
	if !*returned {
		o.failed, o.sealed = true, true
		o.settleCallsLocked()
	}
	close(call.done)
	o.access.Unlock()
}

func (t *Transport) waitTransportCall(call *transportCloseCall) error {
	<-call.done
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	return call.err
}

func (t *Transport) init(allowZeroLengthConnIDs bool) error {
	o := t.lifecycle()
	o.access.Lock()
	if o.failed || o.sealed {
		o.access.Unlock()
		return net.ErrClosed
	}
	if o.initWaiters >= maxTransportCalls {
		o.access.Unlock()
		return errTransportCallCapacity
	}
	o.initWaiters++ // before the original sync.Once wait
	o.access.Unlock()
	returned := false
	defer func() {
		o.access.Lock()
		if returned {
			o.initWaiters--
		} else {
			o.failed, o.sealed = true, true
			o.settleCallsLocked()
		}
		o.access.Unlock()
	}()
	t.initOnce.Do(func() {
		o.access.Lock()
		if o.sealed || o.failed {
			o.access.Unlock()
			return // Close won before the first native initializer.
		}
		o.captureLocked(t)
		o.connOpening = transportConnOpening{socket: o.socket}
		o.initStarted = true
		o.access.Unlock()
		normal := false
		defer func() {
			o.access.Lock()
			o.initReturned, o.initErr = normal, t.initErr
			if normal {
				// All original initializer callbacks returned. Published raw state
				// is retained independently in t.conn and its existing workers.
				o.connOpening = transportConnOpening{}
			} else {
				o.failed, o.sealed, o.initErr = true, true, errPacketOutputUncertain
				o.settleCallsLocked()
			}
			if !o.loopsScheduled {
				// Only the original initializer can settle these unscheduled promises.
				if t.listening != nil {
					close(t.listening)
				}
				if t.output.done != nil {
					close(t.output.done)
				}
			}
			close(o.initDone)
			o.access.Unlock()
		}()
		t.initErr = t.initializeTransport(allowZeroLengthConnIDs, &o.connOpening)
		normal = true
	})
	o.access.Lock()
	err := o.initErr
	if o.failed {
		err = errPacketOutputUncertain
	} else if o.sealed {
		err = net.ErrClosed
	}
	o.access.Unlock()
	returned = true
	return err
}

func (t *Transport) waitTransportInit() error {
	o := t.lifecycle()
	o.access.Lock()
	started := o.initStarted
	o.access.Unlock()
	if !started {
		return nil
	}
	<-o.initDone
	o.access.Lock()
	defer o.access.Unlock()
	if !o.initReturned {
		return errPacketOutputUncertain
	}
	return nil // an ordinary initialization error still ended the original call
}

// Close the original owned socket, or interrupt the original borrowed reader.
// A second caller observes this exact outcome; it never retries native cleanup.
func (t *Transport) abortTransport() (result error) {
	o := t.lifecycle()
	o.access.Lock()
	if o.abort.started {
		o.access.Unlock()
		return t.waitTransportCall(&o.abort)
	}
	o.captureLocked(t)
	o.abort.started = true
	socket, owns, initialized := o.socket, o.ownsSocket, o.initStarted
	o.access.Unlock()
	returned := false
	defer t.finishTransportCall(&o.abort, &returned, &result)
	if socket != nil {
		if owns {
			result = socket.Close()
		} else if initialized {
			result = socket.SetReadDeadline(time.Now())
		}
	}
	if errors.Is(result, net.ErrClosed) {
		result = nil // a known already-closed socket cannot keep its reader live
	}
	returned = true
	return result
}

func (t *Transport) runTransportListener(conn rawConn) {
	returned := false
	defer func() {
		o := t.lifecycle()
		o.access.Lock()
		o.listenerReturned = returned
		if !returned {
			o.failed, o.sealed = true, true
			o.settleCallsLocked()
		}
		o.access.Unlock()
		close(t.listening)
		if returned {
			t.close(nil)
			if t.waitTransportCall(&o.root) == nil && t.retireTransportInputs() == nil {
				t.closeTransportTracer()
			}
		}
	}()
	t.listen(conn)
	returned = true
}

// Root cleanup must not wait on the listener/control worker that may call it.
// Public Close joins the original root and those workers independently.
func (t *Transport) close(e error) {
	t.sealTransport()
	o := t.lifecycle()
	o.access.Lock()
	if o.root.started {
		o.access.Unlock()
		return
	}
	o.root.started = true
	o.access.Unlock()
	returned := false
	var result error
	defer t.finishTransportCall(&o.root, &returned, &result)
	result = t.abortTransport()
	if err := t.waitTransportInit(); err != nil {
		result = errors.Join(result, err)
		returned = true
		return
	}
	if err := t.waitClientOpenings(); err != nil {
		result = errors.Join(result, err)
		returned = true
		return // retain original prefixes / unstarted candidates on negative outcome
	}
	if err := t.waitServerOpening(); err != nil {
		result = errors.Join(result, err)
		returned = true
		return // no fabricated loop join for a partially constructed server
	}
	if err := t.waitPathOpenings(); err != nil {
		result = errors.Join(result, err)
		returned = true
		return // original callback / rollback prefixes stay retained on failure
	}
	t.mutex.Lock()
	t.closedCache.sealed, t.output.sealed = true, true
	if t.closeErr == nil {
		t.closeErr = &errTransportClosed{err: e}
	}
	cause := t.closeErr
	o.access.Lock()
	server := o.server
	o.access.Unlock()
	t.server = nil
	seen := make(map[packetHandler]struct{})
	// The two ID maps can refer to the same Conn through multiple wrapped aliases.
	add := func(handler packetHandler) {
		if wrapped, ok := handler.(*wrappedConn); ok && wrapped != nil {
			handler = wrapped.Conn
		}
		if handler == nil {
			return
		}
		if _, ok := seen[handler]; !ok {
			seen[handler] = struct{}{}
			o.handlers = append(o.handlers, &transportHandlerClose{handler: handler})
		}
	}
	for _, handler := range t.handlers {
		add(handler)
	}
	for _, handler := range t.resetTokens {
		add(handler)
	}
	o.access.Lock()
	for conn := o.clients; conn != nil; conn = conn.clientLifetime.next {
		add(conn)
	}
	for conn := range o.pathConns {
		add(conn)
	}
	o.access.Unlock()
	t.mutex.Unlock()
	if server != nil {
		server.close(cause, true)
		if !server.closeRetired() {
			result = errors.Join(result, errPacketOutputUncertain)
			// A failed original server may still own an unstarted candidate.
			// Keep all original rows; do not wait on a fabricated connection loop.
			returned = true
			return
		} else {
			o.access.Lock()
			o.server = nil
			o.access.Unlock()
		}
	}
	var wg sync.WaitGroup
	for _, call := range o.handlers {
		wg.Add(1)
		go func(call *transportHandlerClose) {
			defer wg.Done() // also accounts for an original destroy ending by Goexit
			call.handler.destroy(cause)
			if conn, ok := call.handler.(*Conn); ok {
				call.err = retireTransportConn(conn)
			}
			if call.err == nil {
				call.handler = nil
			}
			call.returned = true
		}(call)
	}
	t.retireClosedCaches()
	wg.Wait()
	handlersReturned := true
	for _, call := range o.handlers {
		if !call.returned || call.err != nil {
			handlersReturned = false
			result = errors.Join(result, errPacketOutputUncertain, call.err)
		}
	}
	if handlersReturned {
		t.mutex.Lock()
		clear(t.handlers)
		clear(t.resetTokens)
		t.mutex.Unlock()
		o.handlers = nil
	}
	returned = true
}

func (t *Transport) closeTransportTracer() (result error) {
	o := t.lifecycle()
	o.access.Lock()
	if o.trace.started {
		o.access.Unlock()
		return t.waitTransportCall(&o.trace)
	}
	// The listener and both server workers are original Recorder producers.
	if o.failed || o.openingCount != 0 || o.serverOpening.active || o.pathOpeningCount != 0 || len(o.pathConns) != 0 || (o.loopsScheduled && !o.listenerReturned) || o.server != nil {
		o.access.Unlock()
		return errPacketOutputUncertain
	}
	o.trace.started = true
	tracer := o.tracer
	o.access.Unlock()
	returned := false
	defer t.finishTransportCall(&o.trace, &returned, &result)
	if tracer != nil {
		result = tracer.Close()
	}
	if result == nil {
		o.access.Lock()
		o.tracer = nil
		o.access.Unlock()
	}
	returned = true
	return result
}

// Close stops the original listener and abruptly terminates existing
// connections. Concurrent callers join the same original result. A negative
// result retains uncertain resources and does not authorize their replacement.
// Callbacks must not synchronously Close their own Transport (self-join).
func (t *Transport) Close() (result error) {
	t.sealTransport()
	o := t.lifecycle()
	o.access.Lock()
	if o.outer.started {
		o.access.Unlock()
		return t.waitTransportCall(&o.outer)
	}
	o.outer.started = true
	o.access.Unlock()
	returned := false
	defer t.finishTransportCall(&o.outer, &returned, &result)
	t.close(nil)
	result = t.waitTransportCall(&o.root)
	if result != nil {
		returned = true
		return result // original references stay retained; no speculative restore
	}
	o.access.Lock()
	loops := o.loopsScheduled
	o.access.Unlock()
	if loops {
		<-t.listening
	}
	result = errors.Join(t.waitTransportOutput(), t.waitTransportWrites())
	result = errors.Join(result, t.retireTransportInputs())
	o.access.Lock()
	failed := o.failed || (loops && !o.listenerReturned)
	socket, restore := o.socket, o.initStarted && !o.ownsSocket
	o.access.Unlock()
	if failed {
		result = errors.Join(result, errPacketOutputUncertain)
	}
	if result == nil {
		result = t.closeTransportTracer()
	}
	if result == nil && restore && socket != nil {
		result = socket.SetReadDeadline(time.Time{})
		if errors.Is(result, net.ErrClosed) {
			result = nil
		}
	}
	if result == nil {
		o.access.Lock()
		o.socket = nil
		o.access.Unlock()
	}
	returned = true
	return result
}
