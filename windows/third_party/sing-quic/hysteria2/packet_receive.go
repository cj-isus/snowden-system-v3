package hysteria2

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common/buf"
)

// Windows bundle revision 1: engineering ceilings, not a measured RSS/SLO.
// Credits cover HY2-owned input backing, destination strings, fragment slots
// and assembly backing through release or explicit buffer handoff to a reader.
// QUIC queues, caller-owned output buffers and other carriers have other owners.
const (
	maxUDPReceiveSessions = 256
	maxUDPReceiveOwners   = 4096
	maxUDPReceiveBytes    = 8 << 20
	maxUDPFragmentGroups  = 64
	maxUDPFragmentBytes   = 256 << 10
	udpFragmentAge        = 10 * time.Second
)

var (
	errUDPReceiveCapacity = errors.New("HY2 UDP receive capacity exceeded")
	errUDPReceiveProtocol = errors.New("invalid HY2 UDP message")
)

// One budget per carrier. A dequeued message retains its credit while a reader
// copies or transfers its buffer; queue removal alone never returns a credit.
type udpReceiveBudget struct {
	mu              sync.Mutex
	owners          int
	bytes           int
	dropped         uint64 // saturating, no peer-controlled labels/history
	poisoned        bool
	sealed          bool
	settled         chan struct{}
	reported        bool
	onFailure       func() // installed once before client publication; called outside budget/RX locks
	failureNotified bool
	readCalls       int
	readers         map[*udpPacketConn]struct{} // bounded by maxUDPReadCalls; owns active/unknown original slots
	controlCalls    int
	controls        [maxUDPControlCalls]udpControlCall // fixed independent control lane; roots original sessions
}

func (b *udpReceiveBudget) poison() {
	b.mu.Lock()
	b.poisoned = true
	b.settleLocked()
	b.mu.Unlock()
}

func (b *udpReceiveBudget) uncertain() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.poisoned
}

func (b *udpReceiveBudget) markUncertain() {
	b.mu.Lock()
	b.poisoned = true
	b.settleLocked()
	var notify func()
	if !b.failureNotified {
		b.failureNotified = true
		notify = b.onFailure
	}
	b.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (b *udpReceiveBudget) dropLocked() {
	if b.dropped != ^uint64(0) {
		b.dropped++
	}
}

func (b *udpReceiveBudget) drop() {
	b.mu.Lock()
	b.dropLocked()
	b.mu.Unlock()
}

func (b *udpReceiveBudget) reserve(bytes int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sealed || b.poisoned || bytes < 0 || bytes > maxUDPReceiveBytes-b.bytes || b.owners >= maxUDPReceiveOwners {
		b.dropLocked()
		return false
	}
	b.owners++
	b.bytes += bytes
	return true
}

func (b *udpReceiveBudget) release(bytes int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if bytes < 0 || b.owners <= 0 || bytes > b.bytes {
		b.poisoned = true // uncertain accounting never manufactures new capacity
		b.settleLocked()
		return
	}
	b.owners--
	b.bytes -= bytes
	b.settleLocked()
}

func (b *udpReceiveBudget) settleLocked() {
	// Poison settles negatively; the carrier retains this owner and refuses a
	// successor. Seal never refunds storage still held by an original reader.
	if b.sealed && !b.reported && (b.poisoned || b.owners == 0 && b.bytes == 0 && b.readCalls == 0 && len(b.readers) == 0 && b.controlCalls == 0 && b.controlsIdleLocked()) {
		b.reported = true
		close(b.settled)
	}
}

func (b *udpReceiveBudget) seal() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sealed {
		b.sealed = true
		b.settled = make(chan struct{})
	}
	b.settleLocked()
	return b.settled
}

func (b *udpReceiveBudget) retirementError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sealed || b.poisoned || b.owners != 0 || b.bytes != 0 || b.readCalls != 0 || len(b.readers) != 0 || b.controlCalls != 0 || !b.controlsIdleLocked() {
		return errClientUncertain
	}
	return nil
}

// Validate a bounded wire view without a string/message/buffer allocation.
// QUIC varints may use any of their existing 1/2/4/8-byte encodings.
func receiveUDPMessage(data []byte, budget *udpReceiveBudget) (*udpMessage, error) {
	return receiveUDPMessageOwned(data, budget, nil)
}

// The actual client supplies its original task-owned slot before any allocation
// or dispatch. The unchanged service/private wrapper does not claim that owner.
func receiveUDPMessageOwned(data []byte, budget *udpReceiveBudget, original *clientUDPReceive) (*udpMessage, error) {
	if len(data) < 9 {
		return nil, errUDPReceiveProtocol
	}
	width := 1 << (data[8] >> 6)
	if len(data) < 8+width {
		return nil, errUDPReceiveProtocol
	}
	addressLength := uint64(data[8] & 0x3f)
	for index := 1; index < width; index++ {
		addressLength = addressLength<<8 | uint64(data[8+index])
	}
	if addressLength > protocol.MaxAddressLength || addressLength > uint64(len(data)-8-width) || data[7] == 0 || data[6] >= data[7] {
		return nil, errUDPReceiveProtocol
	}
	header := 8 + width + int(addressLength)
	if len(data)-header > protocol.MaxUDPSize || cap(data) > maxUDPReceiveBytes-int(addressLength) {
		budget.drop()
		return nil, errUDPReceiveCapacity
	}
	charge := cap(data) + int(addressLength)
	if !budget.reserve(charge) {
		return nil, errUDPReceiveCapacity
	}
	if original != nil {
		original.reserved, original.charge = true, charge
	}
	message := allocMessage()
	if original != nil {
		original.message = message
	}
	message.rxBudget, message.rxBytes = budget, charge
	message.sessionID = binary.BigEndian.Uint32(data[:4])
	message.packetID = binary.BigEndian.Uint16(data[4:6])
	message.fragmentID, message.fragmentTotal = data[6], data[7]
	message.destination = string(data[8+width : header])
	message.data = buf.As(data[header:])
	return message, nil
}

// feed/close run only under udpPacketConn.receiveAccess. No cache callback or
// pointer to a fragment group escapes this owner, including during expiration.
type udpDefragger struct {
	budget         *udpReceiveBudget
	groups         map[uint16]*packetItem
	retained       int
	failed         bool
	assembly       *udpMessage // receiveAccess; retained until known input handoff
	assemblyCharge int         // reserved before the original message/buffer allocation
}

type packetItem struct {
	messages []*udpMessage
	count    int
	length   int
	metadata int
	retained int
	updated  time.Time
}

func newUDPDefragger(budget *udpReceiveBudget) *udpDefragger {
	return &udpDefragger{budget: budget, groups: make(map[uint16]*packetItem)}
}

func (d *udpDefragger) remove(id uint16) {
	if d.failed {
		panic(errClientUncertain) // never retry an unknown fragment release
	}
	item := d.groups[id]
	if item == nil {
		return
	}
	returned := false
	defer func() {
		if !returned {
			d.failed = true
			d.budget.poison() // caller owns RX lock; no parent callback here
		}
	}()
	for index, message := range item.messages {
		if message != nil {
			message.releaseMessage()
		}
		item.messages[index] = nil // only after the original release returned
	}
	d.budget.release(item.metadata)
	d.retained -= item.retained
	delete(d.groups, id)
	returned = true
}

func (d *udpDefragger) close() error {
	if d.failed || d.assembly != nil || d.assemblyCharge != 0 {
		return errClientUncertain
	}
	for id := range d.groups {
		d.remove(id)
	}
	if d.budget.uncertain() {
		return errClientUncertain
	}
	return nil
}

func (d *udpDefragger) feed(message *udpMessage) *udpMessage {
	if d.failed || d.assembly != nil || d.assemblyCharge != 0 {
		panic(errClientUncertain)
	}
	returned := false
	defer func() {
		if !returned {
			d.failed = true
			d.budget.poison() // input owner notifies only after releasing RX lock
		}
	}()
	result := d.feedMessage(message)
	if d.budget.uncertain() {
		panic(errClientUncertain)
	}
	returned = true
	return result
}

func (d *udpDefragger) feedMessage(message *udpMessage) *udpMessage {
	if message.rxBudget != d.budget || message.fragmentTotal < 2 || message.fragmentID >= message.fragmentTotal {
		message.releaseMessage()
		return nil
	}
	now := time.Now()
	for id, item := range d.groups {
		if now.Sub(item.updated) >= udpFragmentAge {
			d.remove(id) // bounded lazy expiry, not an independent reclaim timer
		}
	}
	item := d.groups[message.packetID]
	if item != nil && len(item.messages) != int(message.fragmentTotal) {
		d.remove(message.packetID)
		item = nil
	}
	if item == nil {
		// Eight bytes per pointer covers the frozen windows/amd64 slot table.
		metadata := int(message.fragmentTotal) * 8
		if len(d.groups) >= maxUDPFragmentGroups || metadata+message.rxBytes > maxUDPFragmentBytes-d.retained {
			d.budget.drop()
			message.releaseMessage()
			return nil
		}
		if !d.budget.reserve(metadata) {
			message.releaseMessage()
			return nil
		}
		item = &packetItem{messages: make([]*udpMessage, int(message.fragmentTotal)), metadata: metadata, retained: metadata}
		d.groups[message.packetID] = item
		d.retained += metadata
	}
	if item.messages[message.fragmentID] != nil {
		message.releaseMessage()
		return nil
	}
	if message.data.Len() > protocol.MaxUDPSize-item.length || message.rxBytes > maxUDPFragmentBytes-d.retained {
		d.budget.drop()
		d.remove(message.packetID)
		message.releaseMessage()
		return nil
	}
	item.messages[message.fragmentID] = message
	item.count++
	item.length += message.data.Len()
	item.retained += message.rxBytes
	item.updated = now
	d.retained += message.rxBytes
	if item.count != len(item.messages) {
		return nil
	}
	// Parts and output coexist until the copy ends. Reserve that peak before
	// allocating the assembled buffer; resource pressure drops this packet only.
	destination := item.messages[0].destination
	charge := protocol.MaxUDPSize + len(destination)
	if !d.budget.reserve(charge) {
		d.remove(message.packetID)
		return nil
	}
	d.assemblyCharge = charge
	d.assembly = allocMessage()
	assembled := d.assembly
	assembled.sessionID, assembled.packetID = message.sessionID, message.packetID
	assembled.fragmentTotal = 1
	assembled.destination = destination
	assembled.rxBudget, assembled.rxBytes = d.budget, charge
	assembled.data = buf.NewSize(protocol.MaxUDPSize)
	for _, part := range item.messages {
		if _, err := assembled.data.Write(part.data.Bytes()); err != nil {
			assembled.releaseMessage()
			d.assembly = nil
			d.assemblyCharge = 0
			d.remove(message.packetID)
			return nil
		}
	}
	d.remove(message.packetID)
	return assembled
}
