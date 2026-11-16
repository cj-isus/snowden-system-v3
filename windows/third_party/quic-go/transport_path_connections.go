package quic

import (
	"errors"
	"net"
	"time"

	"github.com/sagernet/quic-go/internal/protocol"
)

var (
	errPathHandshakePending = errors.New("quic: connection handshake is not complete")
	errPathTransportMissing = errors.New("quic: nil path transport")
)

// Fixed registration cells precede callbacks. The original connection loop owns
// conn/callbacks/added until its outcome; lifecycle.access protects active/count.
type pathConnectionOpening struct {
	active    bool
	conn      *Conn
	callbacks connRunnerCallbacks
	added     bool
}

func (o *transportLifecycle) settlePathOpeningsLocked() {
	if o.sealed && !o.pathOpeningsSettled && (o.failed || o.pathOpeningCount == 0) {
		o.pathOpeningsSettled = true
		close(o.pathOpeningsDone) // failure does not prove callback retirement
	}
}

func (t *Transport) reservePathConnection(conn *Conn) (*pathConnectionOpening, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed {
		return nil, errPacketOutputUncertain
	}
	if o.sealed || t.closeErr != nil {
		return nil, ErrTransportClosed
	}
	if conn.closeErr.Load() != nil {
		return nil, net.ErrClosed
	}
	if o.pathOpeningCount >= len(o.pathOpenings) {
		return nil, errTransportCallCapacity
	}
	for i := range o.pathOpenings {
		call := &o.pathOpenings[i]
		if !call.active {
			o.pathOpeningCount++
			*call = pathConnectionOpening{active: true, conn: conn}
			return call, nil
		}
	}
	return nil, errPacketOutputUncertain
}

func (t *Transport) finishPathConnection(call *pathConnectionOpening, returned *bool) {
	o := t.lifecycle()
	o.access.Lock()
	failed := !*returned || !call.active
	if failed {
		o.failed, o.sealed = true, true // retain original cell and callback prefix
	} else {
		o.pathOpeningCount--
		*call = pathConnectionOpening{}
	}
	o.settleCallsLocked()
	o.access.Unlock()
	if failed {
		t.abortTransport() // original abort only; never join this own registration
	}
}

func (t *Transport) promotePathConnection(call *pathConnectionOpening) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || !call.active {
		return errPacketOutputUncertain
	}
	if o.sealed || t.closeErr != nil {
		return ErrTransportClosed
	}
	if call.conn.closeErr.Load() != nil {
		return net.ErrClosed
	}
	if o.pathConns == nil {
		o.pathConns = make(map[*Conn]struct{})
	}
	o.pathConns[call.conn] = struct{}{} // survives removal of every CID/reset row
	return nil
}

func (t *Transport) registerPathConnection(conn *Conn) error {
	if err := conn.readinessError(); err != nil {
		return err
	}
	call, err := t.reservePathConnection(conn)
	if err != nil {
		return err
	}
	returned := false
	defer t.finishPathConnection(call, &returned)
	runner := (*packetHandlerMap)(t)
	call.callbacks = connRunnerCallbacks{
		AddConnectionID:    func(id protocol.ConnectionID) bool { return runner.Add(id, conn) },
		RemoveConnectionID: func(id protocol.ConnectionID) { runner.Remove(conn, id) },
		ReplaceWithClosed: func(ids []protocol.ConnectionID, data []byte, expiry time.Duration) {
			runner.ReplaceWithClosed(conn, ids, data, expiry)
		},
	}
	call.added, err = conn.connIDGenerator.AddConnRunner(runner, call.callbacks)
	if err != nil {
		returned = true // AddConnRunner's known refusal already joined rollback
		return err
	}
	if call.added && !conn.connIDManager.forEachActiveResetToken(func(token protocol.StatelessResetToken) bool {
		return runner.AddResetToken(token, conn)
	}) {
		t.rollbackPathConnection(call)
		returned = true
		return errCIDRegistration
	}
	if err := t.promotePathConnection(call); err != nil {
		if call.added {
			// Only this newly registered runner can be rolled back. A previous
			// valid binding still owns its original connection on this transport.
			t.rollbackPathConnection(call)
		}
		returned = err != errPacketOutputUncertain
		return err
	}
	returned = true
	return nil
}

func (t *Transport) rollbackPathConnection(call *pathConnectionOpening) {
	conn := call.conn
	runner := (*packetHandlerMap)(t)
	// Remove token rows while the original new runner binding is still held.
	// An abnormal removal must retain that binding and the admitted cell.
	conn.connIDManager.forEachActiveResetToken(func(token protocol.StatelessResetToken) bool {
		runner.RemoveResetToken(conn, token)
		return true
	})
	conn.connIDGenerator.rollbackConnRunner(runner, call.callbacks)
}

func (t *Transport) waitPathOpenings() error {
	o := t.lifecycle()
	<-o.pathOpeningsDone
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || o.pathOpeningCount != 0 {
		return errPacketOutputUncertain
	}
	return nil
}

// Called only after actual connection-loop, input and stream/call retirement.
// The ended loop's runner map is then immutable. Concurrent retirement observers
// can repeat these locked metadata deletions without repeating native cleanup.
func (c *Conn) retirePathConnections() {
	if c.connIDGenerator == nil {
		return
	}
	for runner := range c.connIDGenerator.connRunners {
		if target, ok := runner.(*packetHandlerMap); ok && target != nil {
			o := (*Transport)(target).lifecycle()
			o.access.Lock()
			delete(o.pathConns, c)
			o.access.Unlock()
		}
	}
}
