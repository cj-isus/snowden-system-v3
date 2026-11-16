package quic

import (
	"errors"
	"strings"
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/protocol"
)

const (
	maxClientTokenSize      = 16 << 10
	maxTokenStoreKeyBytes   = 1024
	maxClientTokenCopies    = 64
	maxClientTokenCopyBytes = 1 << 20
)

var errClientTokenState = errors.New("quic: original client token operation is uncertain")

// The original constructor, then native loop, serially own the operation.
// External readiness reads only failed; a callback is never moved to a worker.
type clientTokenStorage struct {
	owner             *Conn
	store             TokenStore
	key               string
	packer            *packetPacker
	retryData         []byte
	retryReserved     bool
	retryInstalled    bool
	prepared, retired bool
	copies, bytes     int // lifetime admitted copies, not current provider storage
	failed            atomic.Bool
	operation         clientTokenOperation
}

type clientTokenOperation struct {
	active bool
	input  *ClientToken
	data   []byte
	packer *packetPacker
}

func (c *Conn) clientTokenStateError() error {
	if c == nil || c.clientTokens.owner != c || c.clientTokens.failed.Load() {
		return errClientTokenState
	}
	return nil
}

func (c *Conn) beginClientTokenOperation() bool {
	r := &c.clientTokens
	if c.clientTokenStateError() != nil || r.retired || r.operation.active ||
		r.operation.input != nil || r.operation.data != nil || r.operation.packer != nil || r.copies < 0 ||
		r.copies > maxClientTokenCopies || r.bytes < r.copies ||
		r.bytes > maxClientTokenCopyBytes || r.bytes > r.copies*maxClientTokenSize {
		r.failed.Store(true)
		return false
	}
	r.operation.active = true
	return true
}

func (c *Conn) finishClientTokenOperation(returned *bool) {
	if !*returned {
		c.clientTokens.failed.Store(true) // retain original input/data/store, no retry
	}
}

func (c *Conn) clearClientTokenOperation(returned *bool) {
	c.clientTokens.operation = clientTokenOperation{}
	*returned = true
}

func (c *Conn) copyClientToken(data []byte) bool {
	r := &c.clientTokens
	n := len(data)
	if n == 0 || n > maxClientTokenSize || r.copies == maxClientTokenCopies ||
		n > maxClientTokenCopyBytes-r.bytes {
		return false // optional address-validation optimization, no protocol failure
	}
	r.copies++
	r.bytes += n // lifetime credit before make/copy, never refunded by Pop/Close
	r.operation.data = make([]byte, n)
	copy(r.operation.data, data)
	r.operation.data = r.operation.data[:n:n]
	return true
}

// Called once inside the original retained client constructor. Configured
// providers return before their result is inspected or copied for the packer.
func (c *Conn) prepareClientTokens(serverName string) {
	r := &c.clientTokens
	if r.prepared {
		r.failed.Store(true)
		panic(errClientTokenState)
	}
	if !c.beginClientTokenOperation() {
		panic(errClientTokenState)
	}
	returned := false
	defer c.finishClientTokenOperation(&returned)
	p, err := c.captureTokenPacker()
	if err != nil {
		panic(err)
	}
	r.packer = p
	r.store = c.config.TokenStore
	if r.store != nil {
		key := serverName
		if key == "" {
			c.construction.remoteAddr = c.conn.RemoteAddr()
			key = c.construction.remoteAddr.String()
		}
		if len(key) == 0 || len(key) > maxTokenStoreKeyBytes {
			r.store = nil
		} else {
			r.key = strings.Clone(key) // bounded owned key, not a substring alias
		}
	}
	c.tokenStoreKey = r.key
	r.prepared = true
	if r.store != nil {
		r.operation.input = r.store.Pop(r.key)
		c.construction.token = r.operation.input
		if token := r.operation.input; token != nil &&
			p.initialTokenFits(token.data, c.origDestConnID, protocol.ByteCount(c.config.InitialPacketSize), c.version) && c.copyClientToken(token.data) {
			c.construction.token = &ClientToken{data: r.operation.data, rtt: token.rtt}
			p.SetToken(c.construction.token.data)
			if token.rtt > 0 {
				c.rttStats.SetInitialRTT(token.rtt)
			}
		} else {
			c.construction.token = nil
		}
	}
	c.clearClientTokenOperation(&returned)
}

// data borrows the current pinned packet. Only an admitted independent copy
// reaches a TokenStore, whose aliases and lifetime remain provider-owned.
func (c *Conn) saveClientToken(data []byte) error {
	r := &c.clientTokens
	if c.clientTokenStateError() != nil || !r.prepared || r.retired {
		r.failed.Store(true)
		return errClientTokenState
	}
	if r.store == nil {
		return nil
	}
	if r.key != c.tokenStoreKey || !c.beginClientTokenOperation() {
		r.failed.Store(true)
		return errClientTokenState
	}
	returned := false
	defer c.finishClientTokenOperation(&returned)
	if c.copyClientToken(data) {
		r.operation.input = &ClientToken{data: r.operation.data, rtt: c.rttStats.SmoothedRTT()}
		r.store.Put(r.key, r.operation.input)
	}
	c.clearClientTokenOperation(&returned)
	return nil
}

func (c *Conn) clientTokenMetadataReady() bool {
	r := &c.clientTokens
	if c.clientTokenStateError() != nil || r.retired || r.operation.active ||
		r.operation.input != nil || r.operation.data != nil || r.operation.packer != nil || r.copies < 0 ||
		r.copies > maxClientTokenCopies || r.bytes < r.copies ||
		r.bytes > maxClientTokenCopyBytes || r.bytes > r.copies*maxClientTokenSize ||
		len(r.key) > maxTokenStoreKeyBytes || r.key != c.tokenStoreKey {
		return false
	}
	if c.perspective == protocol.PerspectiveClient {
		if !r.prepared || r.packer == nil || c.packer != r.packer ||
			!((r.store == nil && r.key == "") || (r.store != nil && r.key != "")) {
			return false
		}
		if !r.retryReserved {
			return r.retryData == nil && !r.retryInstalled && !c.receivedRetry
		}
		n := len(r.retryData)
		if n == 0 || n > protocol.MaxPacketBufferSize || cap(r.retryData) != n || !c.receivedRetry {
			return false
		}
		if !r.retryInstalled {
			return c.closeErr.Load() != nil // known reset error, no token was installed
		}
		return len(r.packer.token) == n && cap(r.packer.token) == n && &r.packer.token[0] == &r.retryData[0]
	}
	return !r.prepared && r.store == nil && r.key == "" && r.copies == 0 && r.bytes == 0 &&
		r.packer == nil && r.retryData == nil && !r.retryReserved && !r.retryInstalled
}

// Only after original native/history/TLS/event/key receipts and metadata preflight.
// Drop LOCAL aliases, never clear a provider's cache or overwrite token bytes.
func (c *Conn) retireClientTokenReferences(p *packetPacker) {
	r := &c.clientTokens
	r.store, r.key, c.tokenStoreKey = nil, "", ""
	r.packer, r.retryData = nil, nil
	p.token = nil // also the original packer's Retry-token alias; native is joined
	r.retired = true
}
