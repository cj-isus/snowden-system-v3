package quic

import (
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

const initialTokenAEADOverhead = 16 // native Initial AES-GCM

// Pure size admission, without peeking packet numbers or invoking providers.
// Reserve PN4, possible destination-CID growth and a byte of CRYPTO through the
// native write offset domain, so admission isn't limited to the first fragment.
func (p *packetPacker) initialTokenFits(token []byte, dest protocol.ConnectionID, size protocol.ByteCount, v protocol.Version) bool {
	if p == nil || p.perspective != protocol.PerspectiveClient || len(token) == 0 ||
		len(token) > protocol.MaxPacketBufferSize || size < protocol.MinInitialPacketSize ||
		size > protocol.MaxPacketBufferSize || dest.Len() > protocol.MaxConnIDLen ||
		p.srcConnID.Len() > protocol.MaxConnIDLen {
		return false
	}
	header := wire.ExtendedHeader{
		Header:          wire.Header{Type: protocol.PacketTypeInitial, SrcConnectionID: p.srcConnID, DestConnectionID: dest, Token: token},
		PacketNumberLen: protocol.PacketNumberLen4,
	}
	var oneByte [1]byte
	crypto := wire.CryptoFrame{Offset: maxCryptoWriteBytes, Data: oneByte[:]}
	length := header.GetLength(v) + protocol.ByteCount(protocol.MaxConnIDLen-dest.Len()) +
		initialTokenAEADOverhead + crypto.Length(v)
	return length <= size
}

// Caller already holds the original token operation prefix. Capture the
// concrete packer before examining it; no anonymous adapter certifies custody.
func (c *Conn) captureTokenPacker() (*packetPacker, error) {
	r := &c.clientTokens
	p, native := c.packer.(*packetPacker)
	r.operation.packer = p
	if !native || p == nil || p.framer != c.framer || p.srcConnID != c.initialSrcConnID ||
		p.initialStream != c.initialStream || p.handshakeStream != c.handshakeStream ||
		p.retransmissionQueue != c.retransmissionQueue ||
		p.perspective != protocol.PerspectiveClient || !c.packetPackerQuiescent() ||
		(r.packer != nil && r.packer != p) {
		r.failed.Store(true)
		return nil, errClientTokenState
	}
	return p, nil
}

// Called only after original Retry phase/CID/version/length/integrity checks.
// Ordinary nonfit returns before changing Retry/CIDs/keys/retransmission state.
func (c *Conn) applyRetryToken(token []byte, dest protocol.ConnectionID, now monotime.Time) (bool, error) {
	r := &c.clientTokens
	if !r.prepared || r.packer == nil || r.retryReserved || !c.beginClientTokenOperation() {
		r.failed.Store(true)
		return false, errClientTokenState
	}
	returned := false
	defer c.finishClientTokenOperation(&returned)
	p, err := c.captureTokenPacker()
	if err != nil {
		return false, err
	}
	if !p.initialTokenFits(token, dest, protocol.ByteCount(c.config.InitialPacketSize), c.version) || c.closeErr.Load() != nil {
		c.clearClientTokenOperation(&returned)
		return false, nil
	}
	r.retryReserved = true // separate single-copy credit, never optional-cache quota
	r.operation.data = make([]byte, len(token))
	copy(r.operation.data, token)
	r.retryData = r.operation.data[:len(token):len(token)]
	c.receivedRetry = true
	if err := c.retransmissionQueue.failureAfter(c.sentPacketHandler.ResetForRetry(now)); err != nil {
		c.clearClientTokenOperation(&returned)
		return false, err
	}
	c.handshakeDestConnID = dest
	c.retrySrcConnID = &dest
	c.cryptoStreamHandler.ChangeConnectionID(dest)
	p.SetToken(r.retryData)
	r.retryInstalled = true
	c.connIDManager.ChangeInitialConnID(dest)
	c.clearClientTokenOperation(&returned)
	return true, nil
}
