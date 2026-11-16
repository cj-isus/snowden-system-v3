// DNS query orchestration adapted from sing-box v1.13.19/protocol/dns/handle.go.
// Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>.
// SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	mDNS "github.com/miekg/dns"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

type forwardingDNSQuery struct {
	scope                                   *forwardingDNSScope
	mu                                      sync.Mutex // values/bounded owned buffer clear only, never APIs/join
	charge, extra, length                   int
	readReturned, taskStarted, taskReturned bool
	failed, retireStarted, retireReturned   bool
	done                                    chan struct{}
	budget                                  *mixedHandshakeBudget
	wire, responseWire, packed              []byte
	request, response                       *mDNS.Msg
	readPacket, responsePacket              *buf.Buffer // unmanaged views of our retained byte storage
	destination                             M.Socksaddr
}

func (query *forwardingDNSQuery) consume(until time.Time, early error) (finalErr error) {
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		query.mu.Lock()
		query.readReturned, query.failed = returned, query.failed || fault
		query.mu.Unlock()
		if fault {
			query.scope.route.markFault()
		}
		query.tryRetire()
		if recovered != nil {
			panic(ErrEngineCleanupFailed)
		}
	}()
	if early != nil {
		returned = true
		return early
	}
	if query.scope.packet == nil {
		var err error
		query.budget, err = newMixedHandshakeBudget(query.scope.route.ctx, query.scope.original, until)
		if err != nil {
			returned = true
			return err
		} // any partial original timer is still input-owned
	}
	query.wire = make([]byte, query.length) // original record/quota exists before allocation
	wire := query.wire
	if query.scope.packet != nil {
		var err error
		wire, err = query.receivePacket()
		if err != nil {
			returned = true
			return err
		}
	} else if _, err := io.ReadFull(query.scope.conn, wire); err != nil {
		returned = true
		return err
	}
	// Reject impossible declared counts before Unpack can allocate its sections.
	// A question needs at least 5 wire bytes, each RR at least 11 (root name,
	// fixed fields, empty RDATA); compression cannot make those minima smaller.
	if len(wire) < 12 {
		returned = true
		return errForwardingDNSProtocol
	}
	questions := int(binary.BigEndian.Uint16(wire[4:6]))
	records := int(binary.BigEndian.Uint16(wire[6:8])) + int(binary.BigEndian.Uint16(wire[8:10])) + int(binary.BigEndian.Uint16(wire[10:12]))
	if 12+5*questions+11*records > len(wire) {
		returned = true
		return errForwardingDNSProtocol
	}
	query.request = new(mDNS.Msg)
	if query.request.Unpack(wire) != nil {
		returned = true
		return errForwardingDNSProtocol
	}
	query.mu.Lock()
	query.taskStarted = true // original task retained BEFORE Go
	query.mu.Unlock()
	go query.run()
	returned = true
	return nil
}

func (query *forwardingDNSQuery) run() {
	returned := false
	var result error
	defer func() {
		fault := recover() != nil || !returned
		query.mu.Lock()
		query.taskReturned, query.failed = returned, query.failed || fault || errors.Is(result, ErrEngineCleanupFailed)
		failed := query.failed
		query.mu.Unlock()
		if failed {
			query.scope.route.markFault()
		}
		if result != nil || failed {
			query.scope.route.cancel()
		} // original Close/retirement task, no self-join
		query.tryRetire()
	}()
	result = query.exchange()
	returned = true
}

func (query *forwardingDNSQuery) exchange() error {
	scope := query.scope
	metadata := scope.metadata
	response, err := scope.dns.Exchange(adapter.WithContext(scope.route.ctx, &metadata), query.request, adapter.DNSQueryOptions{})
	query.response = response // retain even an unexpected response+error
	if err != nil {
		return err
	}
	if response == nil || scope.route.ctx.Err() != nil {
		return errForwardingDNSProtocol
	}
	if scope.packet != nil {
		if err := scope.idle.touch(); err != nil {
			return err
		} // pinned UDP DNS activity point: successful Exchange, before pack/write
	}
	// Len() may report compressed size, but PackBuffer first needs the full
	// uncompressed size + 1. Measure without mutating the borrowed SDK response.
	view := *response
	view.Compress = false
	length := view.Len()
	if length < 12 || length > maxForwardingDNSExpanded || len(response.Question) > maxForwardingDNSMessage || len(response.Answer) > maxForwardingDNSMessage || len(response.Ns) > maxForwardingDNSMessage || len(response.Extra) > maxForwardingDNSMessage {
		return errForwardingDNSProtocol
	}
	if err := scope.reserveExtra(query, length+1-forwardingDNSResponseBase); err != nil {
		return err
	}
	// PackBuffer mutates EDNS extended-RCODE fields. Copy only after count/byte
	// reservations; never mutate/clear an object the DNS service may cache.
	query.response = response.Copy()
	if scope.packet != nil && query.response.Len() > query.packetLimit() {
		query.response.Truncate(query.packetLimit()) // pinned codec; only our copy can change
	}
	headroom := scope.responseOverhead()
	start := headroom
	if scope.packet != nil {
		start = headroom / 2
	}
	query.responseWire = make([]byte, headroom+length+1)
	query.packed, err = query.response.PackBuffer(query.responseWire[start : start+length+1])
	if err != nil {
		return errForwardingDNSProtocol
	}
	if len(query.packed) < 12 || len(query.packed) > maxForwardingDNSMessage {
		return errForwardingDNSProtocol
	}
	if &query.packed[0] != &query.responseWire[start] {
		return ErrEngineCleanupFailed
	} // unexpected extra allocation stays retained
	if scope.packet != nil {
		if len(query.packed) > query.packetLimit() {
			return errForwardingDNSProtocol // TSIG/question-only responses may not be truncatable
		}
		query.responsePacket = buf.With(query.responseWire)
		query.responsePacket.Resize(start, len(query.packed))
		return scope.writePacket(query.responsePacket, query.destination)
	}
	binary.BigEndian.PutUint16(query.responseWire[:2], uint16(len(query.packed)))
	frame := query.responseWire[:2+len(query.packed)]
	return scope.writeFrame(frame)
}

func (scope *forwardingDNSScope) writeFrame(frame []byte) error {
	scope.writeMu.Lock()
	defer scope.writeMu.Unlock() // Close never waits for this mutex before raw native Close
	n, err := scope.conn.Write(frame)
	if n < 0 || n > len(frame) {
		return ErrEngineCleanupFailed
	}
	if err == nil && n != len(frame) {
		return io.ErrShortWrite
	}
	return err // one complete original write, no partial response replay
}

func (query *forwardingDNSQuery) tryRetire() {
	query.mu.Lock()
	if query.retireStarted || !query.readReturned || query.taskStarted && !query.taskReturned {
		query.mu.Unlock()
		return
	}
	query.retireStarted = true
	failed := query.failed
	query.mu.Unlock()
	returned := false
	defer func() {
		if recover() != nil || !returned {
			query.mu.Lock()
			query.failed = true
			query.mu.Unlock()
			query.scope.route.markFault()
		}
	}()
	if query.budget != nil {
		err := query.budget.finish() // after read/query/pack/write return, before quota/resource release
		if errors.Is(err, ErrEngineCleanupFailed) {
			failed = true
		}
		if err != nil {
			query.scope.route.cancel()
		}
	}
	query.mu.Lock()
	failed = failed || query.failed // a timed-out observer cannot be overwritten by a late return
	query.mu.Unlock()
	if failed {
		query.mu.Lock()
		query.failed, query.retireReturned = true, true
		close(query.done)
		query.mu.Unlock()
		query.scope.route.markFault()
		returned = true
		return // original buffers/task/partial allocation and charges stay attached
	}
	// Controlled frames have returned. Only our references/byte storage are
	// released; this is not a receipt for SDK caches, DNS transports or native IO.
	query.mu.Lock()
	clear(query.wire)
	clear(query.responseWire)
	query.wire, query.responseWire, query.packed = nil, nil, nil
	query.request, query.response = nil, nil
	query.readPacket, query.responsePacket = nil, nil // unmanaged views; never returned to an SDK pool
	query.mu.Unlock()
	scope, gate := query.scope, query.scope.route.gate
	gate.mu.Lock()
	scope.mu.Lock()
	query.mu.Lock()
	_, global := gate.dnsQueries[query]
	_, local := scope.queries[query]
	if !global || !local || query.failed || gate.dnsWireBytes < query.charge || gate.dnsExtraBytes < query.extra {
		query.failed, query.retireReturned = true, true
		close(query.done)
		query.mu.Unlock()
		scope.mu.Unlock()
		gate.mu.Unlock()
		scope.route.markFault()
		returned = true
		return
	}
	query.retireReturned = true
	close(query.done) // original retirement outcome, atomically before record removal
	delete(gate.dnsQueries, query)
	delete(scope.queries, query)
	gate.dnsWireBytes -= query.charge
	gate.dnsExtraBytes -= query.extra
	close(gate.dnsChanged)
	gate.dnsChanged = make(chan struct{})
	query.mu.Unlock()
	scope.mu.Unlock()
	gate.mu.Unlock()
	returned = true
}
