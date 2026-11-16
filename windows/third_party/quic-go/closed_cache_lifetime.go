package quic

import (
	"net"
	"time"

	"github.com/sagernet/quic-go/internal/protocol"
)

const (
	maxClosedCacheOwners = 256
	maxClosedCacheBytes  = 384 << 10
	maxClosedCacheIDs    = 64
)

// Transport.mutex guards metadata. Payload/IDs are immutable after ready and
// payload reads require an original queued/current retry reference.
type closedCacheBudget struct {
	owners                  map[*closedCacheOwner]struct{}
	owned, bytes            int
	sealed, failed, settled bool
	done                    chan struct{}
}

type closedCacheOwner struct {
	transport                                                      *Transport
	payload                                                        []byte
	ids                                                            []protocol.ConnectionID
	handler                                                        packetHandler
	ready                                                          chan struct{}
	timer                                                          *time.Timer
	refs, size                                                     int
	opening, cached, timerPending, retiring, stopAttempted, failed bool
}

func (t *Transport) settleClosedCacheLocked() {
	b := &t.closedCache
	if b.settled || !b.sealed || b.done == nil {
		return
	}
	if b.failed || b.owned == 0 {
		b.settled = true
		close(b.done) // failure is a negative result, never authority to reuse roots
	}
}

func (t *Transport) refundClosedCacheLocked(o *closedCacheOwner) {
	b := &t.closedCache
	if _, ok := b.owners[o]; !ok {
		return
	}
	if o.failed || o.opening || o.cached || o.timerPending || o.retiring || o.refs != 0 {
		return
	}
	o.payload, o.ids, o.handler, o.timer = nil, nil, nil, nil
	delete(b.owners, o)
	b.owned--
	b.bytes -= o.size
	t.settleClosedCacheLocked()
}

func closedSourceMatches(current, original packetHandler) bool {
	if original == nil {
		return false
	}
	if current == original {
		return true
	}
	conn, ok := original.(*Conn)
	if !ok || conn == nil {
		return false
	}
	wrapped, ok := current.(*wrappedConn)
	return ok && wrapped != nil && wrapped.Conn == conn
}

func (t *Transport) removeClosedSourceLocked(original packetHandler, ids []protocol.ConnectionID) {
	if original == nil {
		return
	}
	for _, id := range ids {
		if closedSourceMatches(t.handlers[id], original) {
			delete(t.handlers, id)
		}
	}
}

func (t *Transport) closedStopTargetLocked() rawConn {
	if len(t.handlers) == 0 && t.isSingleUse && t.closeErr != nil {
		return t.conn
	}
	return nil
}

func (t *Transport) replaceClosedCache(original packetHandler, ids []protocol.ConnectionID, data []byte, expiry time.Duration) {
	t.mutex.Lock()
	b := &t.closedCache
	matched := false
	for _, id := range ids {
		matched = matched || closedSourceMatches(t.handlers[id], original)
	}
	size := len(data)
	if size > protocol.MaxPacketBufferSize {
		size = 0 // absorb packets without retaining an invalid retry body
	}
	if !matched || t.closeErr != nil || t.transportSealed() || b.sealed || b.failed || expiry <= 0 || len(ids) > maxClosedCacheIDs || b.owned >= maxClosedCacheOwners || size > maxClosedCacheBytes-b.bytes {
		t.removeClosedSourceLocked(original, ids)
		stop := t.closedStopTargetLocked()
		if stop != nil {
			t.requestClosedReadStopLocked()
		}
		t.mutex.Unlock()
		return
	}
	// Admission precedes the holder, map entry, copies, ready channel and timer.
	b.owned++
	b.bytes += size
	o := &closedCacheOwner{transport: t, ready: make(chan struct{}), size: size, opening: true}
	b.owners[o] = struct{}{}
	t.mutex.Unlock()
	returned := false
	defer o.finishRegistration(&returned)
	o.ids = make([]protocol.ConnectionID, len(ids))
	copy(o.ids, ids)
	if size != 0 {
		o.payload = make([]byte, size)
		copy(o.payload, data)
		local := newClosedLocalConn(func(addr net.Addr, info packetInfo) { t.enqueueClosedRetry(o, addr, info) }, t.logger).(*closedLocalConn)
		local.owner = o
		o.handler = local
	} else {
		remote := newClosedRemoteConn().(*closedRemoteConn)
		remote.owner = o
		o.handler = remote
	}
	t.mutex.Lock()
	o.timerPending = true
	t.mutex.Unlock()
	timer := time.AfterFunc(expiry, o.expire)
	t.mutex.Lock()
	o.timer = timer
	if t.closeErr == nil && !t.transportSealed() && !b.sealed && !b.failed {
		for _, id := range o.ids {
			if closedSourceMatches(t.handlers[id], original) {
				t.handlers[id] = o.handler
				o.cached = true
			}
		}
	}
	if !o.cached {
		t.removeClosedSourceLocked(original, ids)
	}
	t.mutex.Unlock()
	returned = true
}

func (o *closedCacheOwner) finishRegistration(returned *bool) {
	t := o.transport
	t.mutex.Lock()
	o.opening = false
	if !*returned {
		o.failed, t.closedCache.failed = true, true
		if o.timer == nil {
			o.timerPending = false
		}
	}
	close(o.ready)
	retire := !o.cached || o.failed
	t.refundClosedCacheLocked(o)
	t.settleClosedCacheLocked()
	t.mutex.Unlock()
	if retire {
		o.retire()
	}
}

func (o *closedCacheOwner) retire() {
	<-o.ready
	t := o.transport
	t.mutex.Lock()
	if o.retiring {
		t.mutex.Unlock()
		return
	}
	o.retiring = true
	removed := o.cached
	o.cached = false
	for _, id := range o.ids {
		if t.handlers[id] == o.handler {
			delete(t.handlers, id)
		}
	}
	if removed && t.closedStopTargetLocked() != nil {
		t.requestClosedReadStopLocked()
	}
	timer := o.timer
	stopTimer := !o.stopAttempted && o.timerPending && timer != nil
	if stopTimer {
		o.stopAttempted = true
	}
	t.mutex.Unlock()
	returned, stopped := false, false
	defer func() {
		t.mutex.Lock()
		if !returned {
			o.failed, t.closedCache.failed = true, true
			o.timerPending = false // uncertain root retained; negative completion only
		} else if stopped {
			o.timerPending = false // actual Stop=true: original callback will not run
		}
		o.retiring = false
		t.refundClosedCacheLocked(o)
		t.settleClosedCacheLocked()
		t.mutex.Unlock()
	}()
	if stopTimer {
		stopped = timer.Stop()
	}
	returned = true
}

func (o *closedCacheOwner) expire() {
	<-o.ready
	returned := false
	defer func() {
		t := o.transport
		t.mutex.Lock()
		if !returned {
			o.failed, t.closedCache.failed = true, true
		}
		o.timerPending = false
		t.refundClosedCacheLocked(o)
		t.settleClosedCacheLocked()
		t.mutex.Unlock()
	}()
	o.retire()
	returned = true
}

func (t *Transport) enqueueClosedRetry(o *closedCacheOwner, addr net.Addr, info packetInfo) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.closeErr != nil || t.output.sealed || t.closedCache.sealed || t.closedCache.failed || o.failed || o.opening || !o.cached {
		return
	}
	o.refs++
	select {
	case t.closeQueue <- closePacket{owner: o, addr: addr, info: info}:
	default:
		o.refs-- // no transfer: producer retains no payload view
	}
}

func (o *closedCacheOwner) finishRetry(known bool) {
	t := o.transport
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if !known || o.refs <= 0 {
		o.failed, t.closedCache.failed = true, true
	} else {
		o.refs--
	}
	t.refundClosedCacheLocked(o)
	t.settleClosedCacheLocked()
}

func (t *Transport) retireClosedCaches() {
	t.mutex.Lock()
	t.closedCache.sealed = true
	owners := make([]*closedCacheOwner, 0, len(t.closedCache.owners))
	for o := range t.closedCache.owners {
		owners = append(owners, o)
	}
	t.settleClosedCacheLocked()
	t.mutex.Unlock()
	for _, o := range owners {
		o.retire()
	}
}
