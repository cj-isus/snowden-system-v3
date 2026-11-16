package quic

import (
	"crypto/tls"
	"net"

	"github.com/sagernet/quic-go/packetaddr"
)

// One fixed cell admits a listener constructor before init or provider work.
// access protects active; the original caller owns the other fields until its
// known return. A failed caller retains its exact partial server and inputs.
type transportServerOpening struct {
	active   bool
	tlsConf  *tls.Config
	config   *Config
	raw      rawConn
	addr     net.Addr
	local    packetaddr.Snapshot
	previous *baseServer
	server   *baseServer
}

func (o *transportLifecycle) settleServerOpeningLocked() {
	if o.sealed && !o.serverOpeningSettled && (o.failed || !o.serverOpening.active) {
		o.serverOpeningSettled = true
		close(o.serverOpeningDone) // failure is negative, not producer retirement
	}
}

func (t *Transport) reserveServerOpening(tlsConf *tls.Config, config *Config) (*transportServerOpening, error) {
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
	if o.serverOpening.active || t.server != nil {
		return nil, errListenerAlreadySet
	}
	o.serverOpening = transportServerOpening{
		active: true, tlsConf: tlsConf, config: config, previous: o.server,
	}
	return &o.serverOpening, nil
}

func (t *Transport) finishServerOpening(call *transportServerOpening, returned *bool) {
	o := t.lifecycle()
	o.access.Lock()
	failed := !*returned || call != &o.serverOpening || !call.active
	if failed {
		o.failed, o.sealed = true, true
	} else {
		*call = transportServerOpening{}
	}
	o.settleCallsLocked()
	o.access.Unlock()
	if failed {
		t.abortTransport() // don't join the root that may be waiting on this caller
	}
}

// Both map publication and the two existing worker starts share Close's seal
// fence. No provider callback or original init wait executes under these locks.
func (t *Transport) publishServerOpening(call *transportServerOpening) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	o := t.lifecycle()
	o.access.Lock()
	defer o.access.Unlock()
	if o.sealed || o.failed || t.closeErr != nil {
		return ErrTransportClosed
	}
	if call != &o.serverOpening || !call.active || call.server == nil || o.server != call.previous {
		return errPacketOutputUncertain
	}
	if t.server != nil {
		return errListenerAlreadySet
	}
	s := call.server
	t.server, o.server = s, s
	go s.run()
	go s.runSendQueue()
	return nil
}

func (t *Transport) waitServerOpening() error {
	o := t.lifecycle()
	<-o.serverOpeningDone
	o.access.Lock()
	defer o.access.Unlock()
	if o.failed || o.serverOpening.active {
		return errPacketOutputUncertain
	}
	return nil
}
