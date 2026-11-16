package quic

import (
	"errors"
	"io"
	"net"
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
)

type sender interface {
	// accepted distinguishes transfer from refusal, including a failed probe write.
	Send(packetSendBatch, uint16, protocol.ECN) (accepted bool, err error)
	SendProbe(packetSendBatch, net.Addr) (accepted bool, err error)
	Run() error
	WouldBlock() bool
	Available() <-chan struct{}
	// retired proves custody completion separately from the original write error.
	Close() (retired bool, err error)
}

type queueEntry struct {
	packetSendBatch
	gsoSize         uint16
	ecn             protocol.ECN
	used, releasing bool
}

// The mutex protects metadata only. Provider/pool calls run outside it.
type sendQueue struct {
	access                                                    sync.Mutex
	entries                                                   [sendQueueCapacity + 1]queueEntry
	probe                                                     queueEntry
	queue                                                     [sendQueueCapacity]*queueEntry
	head, pending                                             int
	active                                                    *queueEntry
	probeRunning                                              bool
	started, workerFinished, sealed, failed, settled, retired bool
	cause, result                                             error
	wake, available, done                                     chan struct{}
	conn                                                      sendConn
}

var _ sender = &sendQueue{}

const sendQueueCapacity = 8

var errSendQueueCapacity = errors.New("quic: send queue capacity exceeded")

func newSendQueue(conn sendConn) sender {
	return &sendQueue{conn: conn, wake: make(chan struct{}, 1), available: make(chan struct{}, 1), done: make(chan struct{})}
}

func (h *sendQueue) Send(batch packetSendBatch, gso uint16, ecn protocol.ECN) (bool, error) {
	_, err := h.admit(batch, gso, ecn, false)
	return err == nil, err
}

func (h *sendQueue) SendProbe(batch packetSendBatch, addr net.Addr) (bool, error) {
	e, err := h.admit(batch, 0, protocol.ECNUnsupported, true)
	if err != nil {
		return false, err
	}
	returned := false
	defer h.finishProbe(&returned)
	err = h.conn.WriteTo(e.original.Data, addr)
	if errors.Is(err, errPacketOutputUncertain) {
		h.fail()
		returned = true
		return true, err
	}
	fatal := fatalPacketWrite(err)
	if fatal {
		h.stop(err)
	}
	if releaseErr := h.release(e, err != nil); releaseErr != nil {
		returned = true
		return true, releaseErr
	}
	returned = true
	if fatal {
		return true, err
	}
	return true, nil // EMSGSIZE/DatagramTooLarge retain the existing loss/MTU policy
}

func (h *sendQueue) WouldBlock() bool {
	h.access.Lock()
	defer h.access.Unlock()
	return h.sealed || h.failed || h.pending == sendQueueCapacity
}

func (h *sendQueue) Available() <-chan struct{} { return h.available }

func fatalPacketWrite(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrShortWrite) {
		return true
	}
	var tooLarge *DatagramTooLargeError
	return !isSendMsgSizeErr(err) && !errors.As(err, &tooLarge)
}

func (h *sendQueue) Run() error {
	if err := h.start(); err != nil {
		return err
	}
	returned := false
	defer h.finishWorker(&returned)
	for {
		e, discard, stop, err := h.take()
		if stop {
			returned = true
			return err
		}
		if e == nil {
			<-h.wake
			continue
		}
		if !discard {
			err = h.conn.Write(e.original.Data, e.gsoSize, e.ecn)
			if errors.Is(err, errPacketOutputUncertain) {
				h.fail()
				returned = true
				return err
			}
			if fatalPacketWrite(err) {
				h.stop(err) // seal before draining; no further provider calls
			}
		}
		if releaseErr := h.release(e, discard || err != nil); releaseErr != nil {
			returned = true
			return releaseErr
		}
	}
}

func (h *sendQueue) Close() (bool, error) {
	h.stop(nil)
	<-h.done // actual original worker and admitted synchronous probe completion
	h.access.Lock()
	defer h.access.Unlock()
	return h.retired, h.result
}
