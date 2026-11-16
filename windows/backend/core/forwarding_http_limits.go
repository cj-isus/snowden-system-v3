package core

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	H "github.com/sagernet/sing/protocol/http"
)

// Initial wire-header budgets, not body-size, heap/RSS or operation-time limits.
const maxMixedHTTPHeaderBytes = 128 << 10

var errMixedHTTPHeaderLimit = errors.New("HTTP request headers exceed limit")

// One source for the existing handshake Reader. Its HTTP budget is switched on
// only around SDK ReadRequest; the same Reader and buffered prefix are retained.
// Raw socket/Read lifetime remains with the exact original mixed input.
type mixedHandshakeReader struct {
	conn                    *mixedIngressConnection
	mu                      sync.Mutex // values only, no socket IO while held
	active, reading, failed bool
	remaining               int
}

func newMixedHandshakeReader(ctx context.Context, raw net.Conn) (*mixedHandshakeReader, error) {
	conn, ok := raw.(*mixedIngressConnection)
	if !ok || conn == nil || conn.owner == nil || conn.owner.gate == nil || ctx == nil || ctx.Value(mixedInputKey{}) != conn.owner {
		return nil, ErrForwardingNotAdmitted
	}
	return &mixedHandshakeReader{conn: conn}, nil
}

func (source *mixedHandshakeReader) beginHTTP(buffered int) error {
	source.mu.Lock()
	invalid := source.failed || source.active || source.reading || buffered < 0 || buffered > mixedHandshakeReadSize || buffered > maxMixedHTTPHeaderBytes
	if invalid {
		source.failed = true
	} else {
		source.active, source.remaining = true, maxMixedHTTPHeaderBytes-buffered
	}
	source.mu.Unlock()
	if invalid {
		source.conn.owner.gate.noteRouteFault()
		return ErrEngineCleanupFailed
	}
	return nil
}

func (source *mixedHandshakeReader) endHTTP() error {
	source.mu.Lock()
	invalid := source.failed || !source.active || source.reading
	source.failed = source.failed || invalid
	source.active, source.remaining = false, 0
	source.mu.Unlock()
	if invalid {
		source.conn.owner.gate.noteRouteFault()
		return ErrEngineCleanupFailed
	}
	return nil
}

func (source *mixedHandshakeReader) Read(buffer []byte) (n int, finalErr error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	source.mu.Lock()
	if source.failed || source.reading {
		source.failed = true
		source.mu.Unlock()
		source.conn.owner.gate.noteRouteFault()
		return 0, ErrEngineCleanupFailed
	}
	active, limit := source.active, len(buffer)
	if active && limit > source.remaining {
		limit = source.remaining
	}
	if limit == 0 {
		source.mu.Unlock()
		return 0, errMixedHTTPHeaderLimit // no raw read beyond the budget
	}
	source.reading = true // original input owns this call, before raw IO
	source.mu.Unlock()
	returned := false
	defer func() {
		recovered := recover()
		fault := recovered != nil || !returned || n < 0 || n > limit
		source.mu.Lock()
		fault = fault || source.failed || source.active != active
		source.reading = false
		source.failed = source.failed || fault
		if active && !fault {
			source.remaining -= n
		}
		source.mu.Unlock()
		if fault {
			finalErr = ErrEngineCleanupFailed
			source.conn.owner.gate.noteRouteFault()
		}
		if recovered != nil || n < 0 || n > limit {
			panic(ErrEngineCleanupFailed)
		}
	}()
	n, finalErr = source.conn.Read(buffer[:limit])
	returned = true
	return
}

func readMixedHTTPRequest(ctx context.Context, reader *bufio.Reader, source *mixedHandshakeReader, until time.Time) (request *http.Request, finalErr error) {
	if reader == nil || source == nil {
		return nil, ErrForwardingNotAdmitted
	}
	if err := source.beginHTTP(reader.Buffered()); err != nil {
		return nil, err
	}
	defer func() {
		if err := source.endHTTP(); err != nil {
			request, finalErr = nil, errors.Join(finalErr, err)
		}
	}()
	finalErr = withMixedHandshakeBudget(ctx, source.conn, until, func() error {
		var err error
		request, err = H.ReadRequest(reader) // same SDK codec; no serialize/replay or nested reader
		return err
	})
	if finalErr != nil {
		request = nil
	}
	return
}

var _ io.Reader = (*mixedHandshakeReader)(nil)
