package core

import (
	"bufio"
	"context"
	"net"
	"sync"
)

// Keep the existing Go default explicit. Only already-buffered handshake bytes
// are copied: taking a prefix must not perform another socket read or hand its
// ownership to an opaque pooled-cache/replaceable reader.
const mixedHandshakeReadSize = 4096

type mixedPrefetch struct {
	input *mixedIngress
	conn  *mixedIngressConnection
	mu    sync.Mutex // values and bounded owned byte-copy/clear only

	data                        []byte
	offset                      int
	captureReturned, captured   bool
	captureDone, ioDone         chan struct{}
	closing, failed             bool
	ioPending                   int
	ioDoneClosed                bool
	retireStarted               bool
	clearStarted, clearReturned bool
}

func retainMixedPrefetch(ctx context.Context, raw net.Conn, reader *bufio.Reader) (net.Conn, error) {
	conn, ok := raw.(*mixedIngressConnection)
	if !ok || conn == nil || conn.owner == nil || ctx == nil || reader == nil || ctx.Value(mixedInputKey{}) != conn.owner {
		return nil, ErrForwardingNotAdmitted
	}
	size := reader.Buffered()
	if size > mixedHandshakeReadSize {
		return nil, ErrForwardingNotAdmitted
	}
	input := conn.owner
	input.mu.Lock()
	if input.closing || input.prefetch != nil || input.ctx.Err() != nil || ctx.Err() != nil {
		input.mu.Unlock()
		return nil, ErrForwardingNotAdmitted
	}
	if size == 0 {
		input.mu.Unlock()
		return raw, nil // common path: no copy, allocation, wrapper or extra owner
	}
	prefix := &mixedPrefetch{input: input, conn: conn, captureDone: make(chan struct{}), ioDone: make(chan struct{})}
	input.prefetch = prefix // BEFORE Peek, allocation, copy or Discard
	input.mu.Unlock()
	if err := prefix.capture(reader, size); err != nil {
		return nil, err // exact prefix stays attached, including partial capture
	}
	return &mixedPrefetchedConnection{mixedIngressConnection: conn, prefix: prefix}, nil
}

func (prefix *mixedPrefetch) capture(reader *bufio.Reader, size int) (finalErr error) {
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		prefix.mu.Lock()
		prefix.captureReturned, prefix.captured = returned, returned && finalErr == nil
		prefix.failed = prefix.failed || fault || finalErr != nil
		close(prefix.captureDone)
		prefix.mu.Unlock()
		if fault || finalErr != nil {
			prefix.input.gate.noteRouteFault()
		}
	}()
	// The original parser frame is the sole Reader user at this synchronous
	// transition. Peek/Discard of exactly Buffered() bytes do not fill from raw.
	buffered, err := reader.Peek(size)
	if err != nil || len(buffered) != size {
		returned = true
		return ErrEngineCleanupFailed
	}
	data := make([]byte, size)
	prefix.mu.Lock()
	prefix.data = data
	prefix.mu.Unlock()
	if copied := copy(data, buffered); copied != size { // retire waits captureDone; no other reader owns this slice yet
		returned = true
		return ErrEngineCleanupFailed
	}
	discarded, err := reader.Discard(size)
	returned = true
	if err != nil || discarded != size {
		return ErrEngineCleanupFailed
	}
	return nil
}

func (prefix *mixedPrefetch) read(buffer []byte) (n int, finalErr error) {
	prefix.mu.Lock()
	if prefix.closing || prefix.failed || !prefix.captured || prefix.input.ctx.Err() != nil {
		prefix.mu.Unlock()
		return 0, ErrForwardingNotAdmitted
	}
	prefix.ioPending++
	prefix.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		prefix.mu.Lock()
		prefix.ioPending--
		prefix.failed = prefix.failed || fault
		if prefix.closing && prefix.ioPending == 0 && !prefix.ioDoneClosed {
			prefix.ioDoneClosed = true
			close(prefix.ioDone)
		}
		prefix.mu.Unlock()
		if fault {
			prefix.input.gate.noteRouteFault()
		}
		if recovered != nil {
			panic(ErrEngineCleanupFailed) // never invent a partial Read/copy outcome
		}
	}()
	var fromPrefix bool
	n, fromPrefix = prefix.copyBuffered(buffer)
	if !fromPrefix {
		n, finalErr = prefix.conn.Read(buffer) // original ingress still counts the actual raw call
	}
	returned = true
	return n, finalErr
}

func (prefix *mixedPrefetch) copyBuffered(buffer []byte) (int, bool) {
	prefix.mu.Lock()
	defer prefix.mu.Unlock()
	if prefix.offset >= len(prefix.data) {
		return 0, false
	}
	n := copy(buffer, prefix.data[prefix.offset:])
	prefix.offset += n
	return n, true
}

func (prefix *mixedPrefetch) seal() {
	prefix.mu.Lock()
	prefix.closing = true
	if prefix.ioPending == 0 && !prefix.ioDoneClosed {
		prefix.ioDoneClosed = true
		close(prefix.ioDone)
	}
	prefix.mu.Unlock()
}

func (prefix *mixedPrefetch) retire() (finalErr error) {
	prefix.mu.Lock()
	if prefix.retireStarted {
		prefix.mu.Unlock()
		return ErrEngineCleanupFailed
	}
	prefix.retireStarted = true
	prefix.mu.Unlock()
	prefix.seal()
	<-prefix.captureDone
	<-prefix.ioDone
	prefix.mu.Lock()
	returned := false
	defer func() {
		fault := recover() != nil || !returned
		if fault {
			finalErr = ErrEngineCleanupFailed
		}
		prefix.failed = prefix.failed || fault || finalErr != nil
		prefix.mu.Unlock()
	}()
	prefix.clearStarted = true
	clear(prefix.data)
	prefix.clearReturned = true
	prefix.data = nil // memory retirement, not a claim about GC/RSS or copies elsewhere
	returned = true
	if prefix.failed || !prefix.captureReturned || !prefix.captured || prefix.ioPending != 0 {
		return ErrEngineCleanupFailed
	}
	return nil
}

// Embedding the exact original private connection preserves its handshake,
// deadline, peer/address and no-unwrap interfaces. The SOCKS reply stays outside it,
// so the SOCKS response still precedes any prefetched application bytes.
type mixedPrefetchedConnection struct {
	*mixedIngressConnection
	prefix *mixedPrefetch
}

func (conn *mixedPrefetchedConnection) Read(buffer []byte) (int, error) {
	return conn.prefix.read(buffer)
}
func (conn *mixedPrefetchedConnection) Close() error {
	conn.prefix.seal()
	conn.owner.cancel()
	return conn.owner.joinClose() // never self-join handler/prefix read retirement here
}

var _ net.Conn = (*mixedPrefetchedConnection)(nil)
