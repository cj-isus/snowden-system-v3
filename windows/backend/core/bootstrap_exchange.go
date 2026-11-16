package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	mDNS "github.com/miekg/dns"

	"github.com/snowden-system/windows/backend/netprobe"
)

// Private worker/path/host binding is attached only to the original service
// Lookup fallback. Ordinary user DNS and serialized receipts cannot create it.
type bootstrapDNSUseKey struct{}
type bootstrapDNSUse struct {
	worker *bootstrapWorker
	path   *bootstrapPath
	host   string
}

type bootstrapExchange struct {
	use        *bootstrapDNSUse
	parent     context.Context
	ctx        context.Context
	cancel     context.CancelFunc
	owner      *preflightNetwork
	done       chan struct{}
	cleanupErr error // immutable when done closes; uncertain original stays in exchanges
}

// worker.mu held. Negative source continuity only, not network authentication.
func (transport *bootstrapDNSTransport) currentLocked(use *bootstrapDNSUse) bool {
	worker := transport.worker
	return use != nil && use.worker == worker && use.path != nil && worker.transport == transport &&
		!worker.closed && !worker.paused && worker.fault == nil && worker.path == use.path && use.path.live &&
		contextError(worker.ctx) == nil && contextError(use.path.ctx) == nil &&
		transport.started && !transport.closed && transport.retirement == nil
}

func (transport *bootstrapDNSTransport) Exchange(parent context.Context, message *mDNS.Msg) (response *mDNS.Msg, resultErr error) {
	if parent == nil || contextError(parent) != nil {
		return nil, ErrPreflightScope
	}
	use, _ := parent.Value(bootstrapDNSUseKey{}).(*bootstrapDNSUse)
	if !bootstrapDNSQuestion(message, use) {
		return nil, ErrPreflightScope
	}
	worker := transport.worker
	worker.mu.Lock()
	if !transport.currentLocked(use) {
		worker.mu.Unlock()
		return nil, ErrPreflightScope
	}
	if len(transport.exchanges) >= maxBootstrapExchanges {
		worker.mu.Unlock()
		return nil, ErrPreflightBusy
	}
	ctx, cancel := context.WithTimeout(parent, bootstrapLookupTimeout)
	query := &bootstrapExchange{use: use, parent: parent, ctx: ctx, cancel: cancel, owner: &preflightNetwork{}, done: make(chan struct{})}
	transport.exchanges[query] = struct{}{} // BEFORE copy/entropy/packet buffer or native dial
	worker.mu.Unlock()
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(callbackDone); _ = query.owner.close() })
	completed := false
	defer func() {
		if !completed {
			_ = recover()
			query.cleanupErr = ErrPreflightCleanup // panic/Goexit never claims retirement
		}
		query.cancel()
		if !stop() {
			<-callbackDone
		} // the original cancellation callback must return too
		query.cleanupErr = errors.Join(query.cleanupErr, query.owner.close())
		worker.mu.Lock()
		deadline, bounded := ctx.Deadline()
		current := transport.currentLocked(query.use) && contextError(query.parent) == nil && bounded && time.Now().Before(deadline)
		if query.cleanupErr == nil {
			delete(transport.exchanges, query)
		} else {
			if worker.fault == nil {
				worker.fault = errors.Join(ErrPreflightCleanup, query.cleanupErr)
			}
			worker.admission.open.Store(false) // negative fence BEFORE releasing the value lock
		}
		close(query.done) // native call/defers/checked Close/cancellation callback have returned
		worker.mu.Unlock()
		if query.cleanupErr != nil {
			worker.poison(query.cleanupErr) // original query/connection error retained, no successor socket
			response, resultErr = nil, errors.Join(ErrPreflightCleanup, resultErr, query.cleanupErr)
		} else if !current {
			response, resultErr = nil, errors.Join(ErrPreflightScope, resultErr)
		}
	}()
	response, resultErr = query.exchange(transport, message)
	completed = true
	return response, resultErr
}

func bootstrapDNSQuestion(message *mDNS.Msg, use *bootstrapDNSUse) bool {
	if use == nil || message == nil || message.Response || message.Opcode != 0 || message.Truncated || message.Rcode != 0 ||
		len(message.Question) != 1 || len(message.Answer) != 0 || len(message.Ns) != 0 || len(message.Extra) > 1 {
		return false
	}
	question := message.Question[0]
	if len(question.Name) > 254 || question.Qclass != mDNS.ClassINET || question.Qtype != mDNS.TypeA && question.Qtype != mDNS.TypeAAAA {
		return false
	}
	host, err := netprobe.CanonicalDNSHost(strings.TrimSuffix(question.Name, "."))
	if err != nil || host != use.host {
		return false
	}
	if len(message.Extra) == 1 {
		opt, ok := message.Extra[0].(*mDNS.OPT)
		if !ok || opt == nil || opt.Hdr.Name != "." || opt.Hdr.Rrtype != mDNS.TypeOPT || opt.Hdr.Ttl != 0 ||
			opt.Hdr.Class < 512 || opt.Hdr.Class > netprobe.MaxDNSMessage || len(opt.Option) != 0 {
			return false
		}
	}
	return true
}

func (query *bootstrapExchange) exchange(transport *bootstrapDNSTransport, borrowed *mDNS.Msg) (*mDNS.Msg, error) {
	request := borrowed.Copy() // only one bounded question and optional empty OPT, after reservation
	originalID := request.Id
	var nonce [2]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	request.Id, request.Compress = binary.BigEndian.Uint16(nonce[:]), false
	wire, err := request.PackBuffer(make([]byte, netprobe.MaxDNSQuery+1))
	if err != nil || len(wire) > netprobe.MaxDNSQuery {
		return nil, netprobe.ErrDNSResponse
	}
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		return transport.dialer.DialContext(ctx, network, transport.server)
	}
	udp, err := query.owner.dialWith(query.ctx, "udp", transport.server.String(), dial)
	if err != nil {
		return nil, err
	}
	if n, err := udp.Write(wire); err != nil || n != len(wire) {
		return nil, errors.Join(err, io.ErrShortWrite)
	}
	buffer := make([]byte, netprobe.MaxDNSMessage+1)
	n, err := udp.Read(buffer)
	if err != nil || n > netprobe.MaxDNSMessage {
		return nil, errors.Join(err, netprobe.ErrDNSResponse)
	}
	response, truncated, err := decodeBootstrapDNS(buffer[:n], request)
	if err != nil {
		return nil, err
	}
	if truncated {
		if err := udp.Close(); err != nil {
			return nil, err
		}
		tcp, err := query.owner.dialWith(query.ctx, "tcp", transport.server.String(), dial)
		if err != nil {
			return nil, err
		}
		frame := make([]byte, 2+len(wire))
		binary.BigEndian.PutUint16(frame[:2], uint16(len(wire)))
		copy(frame[2:], wire)
		if _, err := io.Copy(tcp, bytes.NewReader(frame)); err != nil {
			return nil, err
		}
		var prefix [2]byte
		if _, err := io.ReadFull(tcp, prefix[:]); err != nil {
			return nil, err
		}
		size := int(binary.BigEndian.Uint16(prefix[:]))
		if size < 12 || size > netprobe.MaxDNSMessage {
			return nil, netprobe.ErrDNSResponse
		}
		if _, err := io.ReadFull(tcp, buffer[:size]); err != nil {
			return nil, err
		}
		response, truncated, err = decodeBootstrapDNS(buffer[:size], request)
		if err != nil || truncated {
			return nil, errors.Join(err, netprobe.ErrDNSResponse)
		}
	}
	response.Id = originalID
	return response, nil
}

func decodeBootstrapDNS(wire []byte, request *mDNS.Msg) (*mDNS.Msg, bool, error) {
	if len(wire) < 12 || len(wire) > netprobe.MaxDNSMessage || binary.BigEndian.Uint16(wire[:2]) != request.Id ||
		binary.BigEndian.Uint16(wire[2:4])&0xf800 != 0x8000 || binary.BigEndian.Uint16(wire[4:6]) != 1 {
		return nil, false, netprobe.ErrDNSResponse
	}
	name, offset, err := mDNS.UnpackDomainName(wire, 12)
	if err != nil || offset+4 > len(wire) || !strings.EqualFold(name, request.Question[0].Name) ||
		binary.BigEndian.Uint16(wire[offset:offset+2]) != request.Question[0].Qtype ||
		binary.BigEndian.Uint16(wire[offset+2:offset+4]) != request.Question[0].Qclass {
		return nil, false, netprobe.ErrDNSResponse
	}
	if binary.BigEndian.Uint16(wire[2:4])&0x0200 != 0 {
		return nil, true, nil // correlated question, possibly truncated RR body; same-server TCP only
	}
	if err := netprobe.ValidateDNSServiceFrame(wire); err != nil {
		return nil, false, err
	}
	response := new(mDNS.Msg)
	if response.Unpack(wire) != nil {
		return nil, false, netprobe.ErrDNSResponse
	}
	return response, false, nil
}
