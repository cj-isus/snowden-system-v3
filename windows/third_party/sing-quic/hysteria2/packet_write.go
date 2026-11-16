package hysteria2

import (
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/quicvarint"
	"github.com/sagernet/sing-quic/hysteria2/internal/protocol"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
)

const (
	maxUDPSendOwners       = 8
	maxUDPSendBytes        = 128 << 10
	initialUDPPacketBudget = 1200 - 3
	// Existing default allocator rounds a <=6160-byte packed frame to <=8192.
	// Include bounded address materialization and a 255-pointer fragment table.
	udpSendCharge = 8192 + 2*(protocol.MaxAddressLength+64) + 255*8
)

var (
	errUDPSendCapacity = errors.New("HY2 UDP send capacity exceeded")
	errUDPFragmentSize = errors.New("invalid HY2 UDP fragment budget")
	errUDPDestination  = net.InvalidAddrError("unsupported or oversized HY2 UDP destination")
)

// One non-waiting admission per carrier. This ledger covers HY2 staging until
// synchronous write/cleanup return, not QUIC copies/history, foreign backing,
// caller goroutines or pool/RSS high-water. No waiter or background task is made.
type udpSendBudget struct {
	mu       sync.Mutex
	owners   int
	bytes    int
	poisoned bool
	sealed   bool
	settled  chan struct{}
	reported bool
}

func (b *udpSendBudget) reserve() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sealed || b.poisoned || b.owners >= maxUDPSendOwners || udpSendCharge > maxUDPSendBytes-b.bytes {
		return false
	}
	b.owners++
	b.bytes += udpSendCharge
	return true
}

func (b *udpSendBudget) release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.owners <= 0 || b.bytes < udpSendCharge {
		b.poisoned = true
		b.settleLocked()
		return
	}
	b.owners--
	b.bytes -= udpSendCharge
	b.settleLocked()
}

func (b *udpSendBudget) settleLocked() {
	// Poison settles negatively, never as a drained success or fresh capacity.
	if b.sealed && !b.reported && (b.poisoned || b.owners == 0 && b.bytes == 0) {
		b.reported = true
		close(b.settled)
	}
}

func (b *udpSendBudget) seal() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sealed {
		b.sealed = true
		b.settled = make(chan struct{})
	}
	b.settleLocked()
	return b.settled
}

func (b *udpSendBudget) retirementError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.sealed || b.poisoned || b.owners != 0 || b.bytes != 0 {
		return errClientUncertain
	}
	return nil
}

// Only known value representations may materialize an address. Never call an
// arbitrary net.Addr.String before (or after) checking the resulting length.
// Actual Windows callers use Socksaddr/UDPAddr; unknown adapters fail explicitly.
func udpDestinationString(destination net.Addr) (string, error) {
	var result string
	switch address := destination.(type) {
	case M.Socksaddr:
		if !address.IsValid() || len(address.Fqdn) > protocol.MaxAddressLength || len(address.Addr.Zone()) > protocol.MaxAddressLength || address.IsIP() && address.Fqdn != "" {
			return "", errUDPDestination
		}
		address = address.Unwrap()
		host := address.Fqdn
		if address.IsIP() {
			host = address.Addr.String()
		}
		result = net.JoinHostPort(host, strconv.Itoa(int(address.Port)))
	case *M.Socksaddr:
		if address == nil {
			return "", errUDPDestination
		}
		return udpDestinationString(*address)
	case *net.UDPAddr:
		if address == nil || len(address.IP) != net.IPv4len && len(address.IP) != net.IPv6len || len(address.Zone) > protocol.MaxAddressLength || address.Port < 0 || address.Port > 65535 {
			return "", errUDPDestination
		}
		result = address.String()
	case *net.TCPAddr:
		if address == nil || len(address.IP) != net.IPv4len && len(address.IP) != net.IPv6len || len(address.Zone) > protocol.MaxAddressLength || address.Port < 0 || address.Port > 65535 {
			return "", errUDPDestination
		}
		result = address.String()
	default:
		return "", errUDPDestination
	}
	if len(result) > protocol.MaxAddressLength {
		return "", errUDPDestination
	}
	return result, nil
}

func (m *udpMessage) headerSize() int {
	return 8 + int(quicvarint.Len(uint64(len(m.destination)))) + len(m.destination)
}

func (m *udpMessage) pack() *buf.Buffer {
	buffer := buf.NewSize(m.headerSize() + m.data.Len())
	common.Must(
		binary.Write(buffer, binary.BigEndian, m.sessionID),
		binary.Write(buffer, binary.BigEndian, m.packetID),
		binary.Write(buffer, binary.BigEndian, m.fragmentID),
		binary.Write(buffer, binary.BigEndian, m.fragmentTotal),
		protocol.WriteVString(buffer, m.destination),
		common.Error(buffer.Write(m.data.Bytes())),
	)
	return buffer
}

func udpFragmentPlan(length, destinationLength, packetBudget int) (capacity, count int, err error) {
	if length < 0 || length > protocol.MaxUDPSize || destinationLength < 0 || destinationLength > protocol.MaxAddressLength || packetBudget <= 0 {
		return 0, 0, errUDPFragmentSize
	}
	header := 8 + int(quicvarint.Len(uint64(destinationLength))) + destinationLength
	capacity = packetBudget - header
	if capacity < 0 || capacity == 0 && length != 0 {
		return 0, 0, errUDPFragmentSize
	}
	count = 1
	if length > 0 {
		count = 1 + (length-1)/capacity // checked ceil, without length+capacity overflow
	}
	if count > 255 {
		return 0, 0, errUDPFragmentSize
	}
	return capacity, count, nil
}

func fragUDPMessage(message *udpMessage, packetBudget int) ([]*udpMessage, error) {
	if message == nil || message.data == nil {
		return nil, errUDPFragmentSize
	}
	capacity, count, err := udpFragmentPlan(message.data.Len(), len(message.destination), packetBudget)
	if err != nil {
		return nil, err
	}
	if capacity <= 0 {
		return nil, errUDPFragmentSize
	}
	fragments := make([]*udpMessage, count)
	data := message.data.Bytes()
	for index := range fragments {
		end := len(data)
		if end > capacity {
			end = capacity
		}
		fragment := allocMessage()
		fragment.sessionID, fragment.packetID = message.sessionID, message.packetID
		fragment.fragmentID, fragment.fragmentTotal = uint8(index), uint8(count)
		fragment.destination = message.destination
		fragment.data = buf.As(data[:end]) // borrowed view; never copy RX credits/ownership
		fragments[index] = fragment
		data = data[end:]
	}
	return fragments, nil
}

func (c *udpPacketConn) beginWrite(length int) error {
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	if c.sendScope == nil {
		return net.ErrClosed
	}
	if err := c.sendScope.Err(); err != nil {
		return err
	}
	if length > protocol.MaxUDPSize {
		return &quic.DatagramTooLargeError{MaxDatagramPayloadSize: protocol.MaxUDPSize}
	}
	if c.sendBudget == nil || !c.sendBudget.reserve() {
		return errUDPSendCapacity
	}
	return nil
}

func (c *udpPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	reserved := false
	defer func() {
		buffer.Release() // consumes the original caller buffer exactly once
		if reserved {
			c.sendBudget.release()
		}
	}()
	if buffer == nil {
		return errUDPFragmentSize
	}
	if err := c.beginWrite(buffer.Len()); err != nil {
		return err
	}
	reserved = true
	address, err := udpDestinationString(destination)
	if err != nil {
		return err
	}
	return c.writeDatagram(buffer.Bytes(), address)
}

func (c *udpPacketConn) WriteTo(data []byte, destination net.Addr) (int, error) {
	if err := c.beginWrite(len(data)); err != nil {
		return 0, err
	}
	defer c.sendBudget.release()
	address, err := udpDestinationString(destination)
	if err != nil {
		return 0, err
	}
	if err := c.writeDatagram(data, address); err != nil {
		return 0, err
	}
	return len(data), nil // enqueue/write-call outcome, not peer delivery
}

func (c *udpPacketConn) writeDatagram(data []byte, destination string) error {
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	budget := c.udpMTU // no unbound PMTU cache or unsupported larger initial budget
	if budget <= 0 || budget > initialUDPPacketBudget {
		return errUDPFragmentSize
	}
	if _, _, err := udpFragmentPlan(len(data), len(destination), budget); err != nil {
		return err // plan before message/view allocation or packet-ID reservation
	}
	message := allocMessage()
	message.sessionID = c.sessionID
	message.packetID = c.nextPacketID(0, false)
	message.fragmentTotal = 1
	message.destination = destination
	message.data = buf.As(data)
	defer message.releaseMessage()
	err := c.writeMessage(message, budget)
	if err == nil {
		return nil
	}
	if c.ctx.Err() != nil {
		return errors.Join(err, net.ErrClosed)
	}
	var tooLarge *quic.DatagramTooLargeError
	if !errors.As(err, &tooLarge) || tooLarge.MaxDatagramPayloadSize <= 3 || tooLarge.MaxDatagramPayloadSize >= int64(budget)+3 {
		return err
	}
	// Check the signed value before int conversion/subtraction. Preserve the
	// original conservative -3 and one retry; require actual size progress.
	nextBudget := int(tooLarge.MaxDatagramPayloadSize - 3)
	if _, _, planErr := udpFragmentPlan(len(data), len(destination), nextBudget); planErr != nil {
		return errors.Join(err, planErr)
	}
	message.packetID = c.nextPacketID(message.packetID, true)
	return c.writeMessage(message, nextBudget)
}

// All local ID reservations use this short carrier metadata fence. A retry's
// own prior ID is skipped even across uint16 wrap/concurrent other writers.
// This does not guarantee uniqueness against every old packet on the wire.
func (c *udpPacketConn) nextPacketID(previous uint16, retry bool) uint16 {
	c.sendBudget.mu.Lock()
	defer c.sendBudget.mu.Unlock()
	id := uint16(c.packetId.Add(1))
	if retry && id == previous {
		id = uint16(c.packetId.Add(1))
	}
	return id
}

func (c *udpPacketConn) writeMessage(message *udpMessage, budget int) error {
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	if message.data.Len()+message.headerSize() <= budget {
		return c.writePacket(message)
	}
	fragments, err := fragUDPMessage(message, budget)
	if err != nil {
		return err
	}
	defer releaseMessages(fragments)
	for _, fragment := range fragments {
		if err := c.writePacket(fragment); err != nil {
			return err
		}
	}
	return nil
}

func (c *udpPacketConn) writePacket(message *udpMessage) error {
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	buffer := message.pack()
	defer buffer.Release()
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	err := c.sendScope.Send(c.ctx, buffer.Bytes())
	if err != nil && c.ctx.Err() != nil {
		return errors.Join(err, net.ErrClosed)
	}
	return err
}
