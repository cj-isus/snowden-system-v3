package hysteria2

import (
	"crypto/rand"
	"errors"
	"io"
	"net"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/crypto/blake2b"
)

const (
	maxSalamanderWriters = 8
	maxSalamanderPacket  = 65535
	maxSalamanderVectors = 256
)

var (
	errSalamanderSize      = errors.New("invalid Salamander packet size or vector count")
	errSalamanderCapacity  = errors.New("Salamander write capacity exceeded")
	errSalamanderUncertain = errors.New("Salamander original write cleanup is uncertain")
)

// One fixed staging slot precedes allocation. Each contains at most65535 bytes
// (one64KiB class in the default allocator). Input/password/hash scratch, pool
// high-water, caller goroutines and kernel buffers are outside this ledger.
func (s *SalamanderPacketConn) beginWrite() (int, error) {
	s.txAccess.Lock()
	defer s.txAccess.Unlock()
	if s.txClosed {
		return 0, net.ErrClosed
	}
	if s.txPoisoned {
		return 0, errSalamanderUncertain
	}
	for slot := range s.txHeld {
		if !s.txHeld[slot] {
			s.txHeld[slot] = true
			s.txActive++
			return slot, nil
		}
	}
	return 0, errSalamanderCapacity
}

func (s *SalamanderPacketConn) settleWritesLocked() {
	if s.txClosed && !s.txReported && (s.txPoisoned || s.txActive == 0) {
		s.txReported = true
		close(s.txDone)
	}
}

func (s *SalamanderPacketConn) finishWrite(slot int, returned bool) {
	s.txAccess.Lock()
	packet := s.txBuffers[slot]
	if !returned {
		// A foreign writer may still hold the output view. Keep its original
		// pooled backing, poison admission, and report a negative Close result.
		s.txPoisoned = true
		s.settleWritesLocked()
		s.txAccess.Unlock()
		s.rawClose.start()
		return
	}
	s.txAccess.Unlock()
	if packet != nil {
		packet.Release() // actual writer returned before pool/slot reuse
	}
	s.txAccess.Lock()
	s.txBuffers[slot] = nil
	s.txHeld[slot] = false
	s.txActive--
	s.settleWritesLocked()
	s.txAccess.Unlock()
}

func (s *SalamanderPacketConn) newPacket(slot, payload int) (*buf.Buffer, [blake2b.Size256]byte, error) {
	packet := buf.NewSize(salamanderSaltLen + payload)
	s.txAccess.Lock()
	s.txBuffers[slot] = packet
	s.txAccess.Unlock()
	salt := packet.Extend(salamanderSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, [blake2b.Size256]byte{}, err
	}
	return packet, blake2b.Sum256(append(s.password, salt...)), nil
}

func appendSalamander(packet *buf.Buffer, input []byte, key *[blake2b.Size256]byte, offset int) int {
	output := packet.Extend(len(input))
	for index, value := range input {
		output[index] = value ^ key[(offset+index)%blake2b.Size256]
	}
	return (offset + len(input)) % blake2b.Size256
}

// WriteTo borrows its input unchanged. Both wrapper variants use one owned
// ciphertext buffer and the original PacketConn.WriteTo API, preserving connected
// and custom destination behavior without invoking an arbitrary Addr.String.
func (s *SalamanderPacketConn) WriteTo(input []byte, destination net.Addr) (int, error) {
	if len(input) > maxSalamanderPacket-salamanderSaltLen {
		return 0, errSalamanderSize
	}
	slot, err := s.beginWrite()
	if err != nil {
		return 0, err
	}
	returned := false
	defer func() { s.finishWrite(slot, returned) }()
	packet, key, err := s.newPacket(slot, len(input))
	if err != nil {
		returned = true
		return 0, err
	}
	appendSalamander(packet, input, &key, 0)
	n, err := s.PacketConn.WriteTo(packet.Bytes(), destination)
	returned = true
	if err != nil {
		return 0, err
	}
	if n != packet.Len() {
		return 0, io.ErrShortWrite
	}
	return len(input), nil
}

// This consuming API releases its original inputs itself. The downstream writer
// only sees an unmanaged view of our output, so its ReleaseMulti / early-error
// behavior cannot return our managed staging to the pool or release inputs twice.
func (s *VectorisedSalamanderPacketConn) WriteVectorisedPacket(inputs []*buf.Buffer, destination M.Socksaddr) error {
	slot, reserved, returned := 0, false, false
	defer func() {
		cleaned := false
		defer func() {
			if reserved {
				s.finishWrite(slot, returned && cleaned)
			}
		}()
		buf.ReleaseMulti(inputs)
		cleaned = true
	}()
	if len(inputs) > maxSalamanderVectors {
		return errSalamanderSize
	}
	var length int
	for _, input := range inputs {
		if input == nil || input.Len() < 0 || input.Len() > maxSalamanderPacket-salamanderSaltLen-length {
			return errSalamanderSize
		}
		length += input.Len()
	}
	var err error
	slot, err = s.beginWrite()
	if err != nil {
		return err
	}
	reserved = true
	packet, key, err := s.newPacket(slot, length)
	if err != nil {
		returned = true
		return err
	}
	var offset int
	for _, input := range inputs {
		offset = appendSalamander(packet, input.Bytes(), &key, offset)
	}
	err = s.writer.WriteVectorisedPacket([]*buf.Buffer{buf.As(packet.Bytes())}, destination)
	returned = true
	return err
}

func (s *SalamanderPacketConn) Close() error {
	s.txAccess.Lock()
	s.txClosed = true
	s.settleWritesLocked()
	s.txAccess.Unlock()
	// Independent original socket abort precedes the original writer join.
	// Repeated Close waits for this same result, never a fresh underlying Close.
	s.rawClose.start()
	<-s.rawClose.done
	<-s.txDone
	s.txAccess.Lock()
	uncertain := s.txPoisoned || s.txActive != 0
	s.txAccess.Unlock()
	if uncertain {
		return errors.Join(s.rawClose.err, errSalamanderUncertain)
	}
	return s.rawClose.err
}
