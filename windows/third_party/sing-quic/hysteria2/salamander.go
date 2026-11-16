package hysteria2

import (
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/crypto/blake2b"
)

const salamanderSaltLen = 8

const ObfsTypeSalamander = "salamander"

type SalamanderPacketConn struct {
	net.PacketConn
	password   []byte
	rawClose   *clientCloseCall
	txAccess   sync.Mutex
	txClosed   bool
	txPoisoned bool
	txActive   int
	txHeld     [maxSalamanderWriters]bool
	txBuffers  [maxSalamanderWriters]*buf.Buffer
	txDone     chan struct{}
	txReported bool
}

func NewSalamanderConn(conn net.PacketConn, password []byte) net.PacketConn {
	writer, isVectorised := bufio.CreateVectorisedPacketWriter(conn)
	if isVectorised {
		wrapped := &VectorisedSalamanderPacketConn{writer: writer}
		wrapped.initialize(conn, password)
		return wrapped
	}
	wrapped := &SalamanderPacketConn{}
	wrapped.initialize(conn, password)
	return wrapped
}

func (s *SalamanderPacketConn) initialize(conn net.PacketConn, password []byte) {
	s.PacketConn = conn
	s.password = append([]byte(nil), password...)
	s.password = s.password[:len(s.password):len(s.password)]
	s.rawClose = newClientCloseCall(conn.Close)
	s.txDone = make(chan struct{})
}

func (s *SalamanderPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if len(p) < salamanderSaltLen {
		return 0, nil, io.ErrShortBuffer
	}
	for {
		n, addr, err = s.PacketConn.ReadFrom(p)
		if err != nil {
			return
		}
		if n < 0 || n > len(p) {
			return 0, nil, errSalamanderSize
		}
		if n < salamanderSaltLen {
			continue // an incomplete salt is not an application datagram
		}
		key := blake2b.Sum256(append(s.password, p[:salamanderSaltLen]...))
		for index, c := range p[salamanderSaltLen:n] {
			p[index] = c ^ key[index%blake2b.Size256]
		}
		return n - salamanderSaltLen, addr, nil
	}
}

func (s *SalamanderPacketConn) Upstream() any {
	return s.PacketConn
}

type VectorisedSalamanderPacketConn struct {
	SalamanderPacketConn
	writer N.VectorisedPacketWriter
}
