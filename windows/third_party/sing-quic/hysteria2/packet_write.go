package hysteria2

import (
	"encoding/binary"
	"errors"
	"net"
	"strconv"

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
	// Keep the conservative address/former255-pointer-table staging charge.
	// Actual owned writes add retained input backing and stream one fragment.
	udpSendCharge = 8192 + 2*(protocol.MaxAddressLength+64) + 255*8
)

var (
	errUDPSendCapacity = errors.New("HY2 UDP send capacity exceeded")
	errUDPFragmentSize = errors.New("invalid HY2 UDP fragment budget")
	errUDPDestination  = net.InvalidAddrError("unsupported or oversized HY2 UDP destination")
)

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
	return m.packOwned(nil)
}

func (m *udpMessage) packOwned(original *udpWriteCall) *buf.Buffer {
	buffer := buf.NewSize(m.headerSize() + m.data.Len())
	if original != nil {
		original.packed = buffer // before encoding or any Must/pool failure
	}
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

// Counter-only private compatibility helper. Public writes use beginOwnedWrite.
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

func (c *udpPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) (err error) {
	var original *udpWriteCall
	returned := false
	defer func() {
		if original == nil {
			c.releaseUnadmittedPacket(buffer)
		} else if writeErr := c.endOwnedWrite(original, returned); writeErr != nil {
			err = errors.Join(err, writeErr)
		}
	}()
	if buffer == nil {
		return errUDPFragmentSize
	}
	data := buffer.Bytes()
	original, err = c.beginOwnedWrite(buffer, data, destination, buffer.Start())
	if err != nil {
		return err
	}
	address, err := udpDestinationString(destination)
	if err != nil {
		returned = true
		return err
	}
	original.address = address
	err = c.writeDatagram(original)
	returned = true
	return
}

func (c *udpPacketConn) WriteTo(data []byte, destination net.Addr) (n int, err error) {
	original, err := c.beginOwnedWrite(nil, data, destination, 0)
	if err != nil {
		return 0, err
	}
	returned := false
	defer func() {
		if writeErr := c.endOwnedWrite(original, returned); writeErr != nil {
			n = 0
			err = errors.Join(err, writeErr)
		}
	}()
	address, err := udpDestinationString(destination)
	if err != nil {
		returned = true
		return 0, err
	}
	original.address = address
	err = c.writeDatagram(original)
	returned = true
	if err != nil {
		return 0, err
	}
	return len(data), nil // enqueue/write-call outcome, not peer delivery
}

func (c *udpPacketConn) writeDatagram(original *udpWriteCall) error {
	data, destination := original.inputData, original.address
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
	original.stage = udpWriteMessage
	original.message = allocMessage()
	message := original.message
	message.sessionID = c.sessionID
	message.packetID = c.nextPacketID(0, false)
	message.fragmentTotal = 1
	message.destination = destination
	message.data = buf.As(data)
	err := c.writeMessage(original, message, budget)
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
	return c.writeMessage(original, message, nextBudget)
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

func (c *udpPacketConn) writeMessage(original *udpWriteCall, message *udpMessage, budget int) error {
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	if message.data.Len()+message.headerSize() <= budget {
		return c.writePacket(original, message)
	}
	capacity, count, err := udpFragmentPlan(message.data.Len(), len(message.destination), budget)
	if err != nil {
		return err
	}
	if capacity <= 0 || original.fragment != nil || original.packed != nil || original.fragmentCount != 0 {
		return errUDPFragmentSize
	}
	original.fragmentCount = count
	data := message.data.Bytes()
	for index := 0; index < count; index++ {
		original.fragmentIndex = index
		original.fragmentStart = index * capacity // bounded by the validated payload plan
		end := len(data)
		if end > capacity {
			end = capacity
		}
		original.stage = udpWriteFragment
		original.fragment = allocMessage()
		fragment := original.fragment
		fragment.sessionID, fragment.packetID = message.sessionID, message.packetID
		fragment.fragmentID, fragment.fragmentTotal = uint8(index), uint8(count)
		fragment.destination = message.destination
		fragment.data = buf.As(data[:end:end])
		err = c.writePacket(original, fragment)
		original.stage = udpWriteFragmentRelease
		fragment.releaseMessage()
		original.fragment = nil
		if err != nil {
			original.fragmentCount, original.fragmentIndex, original.fragmentStart = 0, 0, 0
			return err
		}
		data = data[end:]
	}
	original.fragmentCount, original.fragmentIndex, original.fragmentStart = 0, 0, 0
	return nil
}

func (c *udpPacketConn) writePacket(original *udpWriteCall, message *udpMessage) error {
	select {
	case <-c.ctx.Done():
		return net.ErrClosed
	default:
	}
	if original.packed != nil {
		return errClientUncertain
	}
	original.stage = udpWritePack
	buffer := message.packOwned(original)
	var err error
	select {
	case <-c.ctx.Done():
		err = net.ErrClosed
	default:
		original.stage = udpWriteSend
		err = c.sendScope.Send(c.ctx, buffer.Bytes())
	}
	if err != nil && c.ctx.Err() != nil {
		err = errors.Join(err, net.ErrClosed)
	}
	original.stage = udpWritePackedRelease
	buffer.Release()
	original.packed = nil
	return err
}
