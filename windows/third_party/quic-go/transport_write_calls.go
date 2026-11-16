package quic

import (
	"errors"
	"net"

	"github.com/sagernet/quic-go/internal/protocol"
)

const (
	maxTransportCalls       = 64
	maxTransportWriteBytes  = 4 << 20
	maxTransportPacketBytes = 65535
)

var errTransportCallCapacity = errors.New("quic: transport operation capacity exceeded")

// Original caller input/address and exact provider remain attached across init,
// native writes and abnormal exit. These bounds cover exposed spans, not RSS.
type transportWriteCall struct {
	input []byte
	addr  net.Addr
	conn  rawConn
	info  packetInfo
	oob   []byte
}

func (o *transportLifecycle) settleWritesLocked() {
	if o.sealed && !o.writesSettled && (o.failed || o.writeCount == 0) {
		o.writesSettled = true
		close(o.writesDone) // failed is a negative result, not permission to reuse
	}
}

func (t *Transport) reserveTransportWrite(b []byte, addr net.Addr, info packetInfo) (*transportWriteCall, error) {
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed {
		return nil, errPacketOutputUncertain
	}
	if o.sealed {
		return nil, net.ErrClosed
	}
	if o.writeCount >= maxTransportCalls || len(b) > maxTransportPacketBytes || len(b) > maxTransportWriteBytes-o.writeBytes {
		return nil, errTransportCallCapacity
	}
	o.writeCount++
	o.writeBytes += len(b)
	if o.writes == nil {
		o.writes = make(map[*transportWriteCall]struct{})
	}
	call := &transportWriteCall{input: b, addr: addr, info: info}
	o.writes[call] = struct{}{}
	return call, nil
}

func (t *Transport) finishTransportWrite(call *transportWriteCall, returned *bool) {
	o := t.lifecycle()
	o.access.Lock()
	if *returned {
		o.writeBytes -= len(call.input)
		o.writeCount--
		call.input, call.addr, call.conn = nil, nil, nil
		call.info, call.oob = packetInfo{}, nil
		delete(o.writes, call)
	} else {
		o.failed, o.sealed = true, true
	}
	o.settleCallsLocked()
	o.access.Unlock()
}

// WriteTo sends one packet through the original underlying connection. Register
// before initializer waits and provider callbacks; closing cannot bypass it.
func (t *Transport) WriteTo(b []byte, addr net.Addr) (int, error) {
	return t.writePacketTo(b, addr, packetInfo{})
}

// Same original write admission, with retained local packet metadata for routed
// protocol replies. Public WriteTo keeps its zero-metadata default.
func (t *Transport) writePacketTo(b []byte, addr net.Addr, info packetInfo) (int, error) {
	call, err := t.reserveTransportWrite(b, addr, info)
	if err != nil {
		return 0, err
	}
	returned := false
	defer t.finishTransportWrite(call, &returned)
	if err := t.init(false); err != nil {
		returned = true // no provider borrowed this write's input
		return 0, err
	}
	o := t.lifecycle()
	o.access.Lock()
	if o.sealed || o.failed {
		o.access.Unlock()
		returned = true
		return 0, net.ErrClosed
	}
	call.conn = t.conn
	o.access.Unlock()
	call.oob = call.info.OOB() // outside lifecycle lock, before the native borrow
	n, err := call.conn.WritePacket(call.input, call.addr, call.oob, 0, protocol.ECNUnsupported)
	err = packetWriteResult(n, len(call.input), err)
	returned = !errors.Is(err, errPacketOutputUncertain)
	// Never synchronously close here: a connection loop can be this caller.
	return n, err
}

func (t *Transport) waitTransportWrites() error {
	o := t.lifecycle()
	<-o.writesDone
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || o.writeCount != 0 {
		return errPacketOutputUncertain
	}
	return nil
}
