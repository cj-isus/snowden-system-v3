package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	metricsHeaderBytes = 16 << 10
	// Includes chunk framing/trailers, independently of the 2 MiB decoded JSON.
	// This bounds accepted reads, not kernel buffering, RSS or execution time.
	metricsFrameBytes = 4 << 20
)

// One original process-wide I/O owner across all old/new metrics sessions and
// Wails calls. Unknown cleanup keeps the original pointer and refuses new work.
var metricsPolls struct {
	mu      sync.Mutex
	attempt *metricsAttempt
}

type metricsAttempt struct {
	m        *metricsState
	scope    *metricsScope // nil only for the retained internal poll entry point
	parent   context.Context
	ctx      context.Context
	cancel   context.CancelFunc
	deadline time.Time
	done     chan struct{}
	retired  bool // immutable after done; done alone is not a retirement receipt
	failed   atomic.Bool

	watchDone     chan struct{}
	watchReturned bool // immutable after watchDone
	mu            sync.Mutex
	conn          net.Conn
	closeStarted  bool
	closeReturned bool // immutable after closeDone
	closeErr      error
	closeDone     chan struct{}

	dialStarted, dialReturned bool
	dialResultKnown           bool
	response                  *http.Response
	bodyCloseStarted          bool
	bodyCloseReturned         bool
	bodyCloseErr              error
	readTimely                bool
	observedAt                time.Time
	snapshot                  clashSnapshot
	view                      MetricsView
	result                    error
}

func (m *metricsState) pollOwned(parent context.Context, scope *metricsScope) (MetricsView, error) {
	if m == nil || parent == nil || parent.Err() != nil {
		return MetricsView{}, errMetricsSnapshot
	}
	if scope != nil && (scope.metrics != m || scope.parent != parent) {
		return MetricsView{}, errMetricsSnapshot
	}
	host, portText, endpointErr := net.SplitHostPort(m.port)
	port, portErr := strconv.ParseUint(portText, 10, 16)
	if endpointErr != nil || portErr != nil || host != "127.0.0.1" || port == 0 || strconv.FormatUint(port, 10) != portText || m.secret == "" || len(m.secret) > metricsSnapshotStringBytes {
		return MetricsView{}, errMetricsSnapshot
	}
	for index := range m.secret {
		if m.secret[index] < '!' || m.secret[index] > '~' {
			return MetricsView{}, errMetricsSnapshot // never serialize control bytes into Authorization
		}
	}
	ctx, cancel := context.WithTimeout(parent, metricsHTTPTimeout)
	defer cancel()
	attempt := &metricsAttempt{m: m, scope: scope, parent: parent, ctx: ctx, cancel: cancel,
		done: make(chan struct{}), closeDone: make(chan struct{}), result: errMetricsSnapshot}
	attempt.deadline, _ = ctx.Deadline()
	if !attempt.reserve() {
		return MetricsView{}, errMetricsSnapshot
	}
	attempt.run() // original synchronous frame; no worker replacement after timeout
	return attempt.view, attempt.result
}

func (attempt *metricsAttempt) reserve() bool {
	if attempt.scope != nil {
		release, ok := attempt.scope.lockCurrent()
		if !ok {
			return false
		}
		defer release()
	}
	metricsPolls.mu.Lock()
	defer metricsPolls.mu.Unlock()
	if metricsPolls.attempt != nil || attempt.ctx.Err() != nil || attempt.scope != nil && attempt.scope.app.metricsSealed.Load() {
		return false
	}
	metricsPolls.attempt = attempt // before watcher, Dial, Write, response or body
	return true
}

func (attempt *metricsAttempt) run() {
	returned := false
	defer attempt.finish(&returned)
	attempt.body()
	returned = true
}

func (attempt *metricsAttempt) body() {
	attempt.watchDone = make(chan struct{})
	go attempt.watch()
	if attempt.ctx.Err() != nil {
		return
	}
	if attempt.scope != nil {
		release, ok := attempt.scope.lockCurrent()
		if !ok {
			return
		}
		release()
	}
	attempt.dialStarted = true
	raw, dialErr := (&net.Dialer{Timeout: metricsHTTPTimeout}).DialContext(attempt.ctx, "tcp4", attempt.m.port)
	attempt.mu.Lock()
	attempt.conn = raw // keep even a late or failed Dial's nonnil original socket
	attempt.mu.Unlock()
	attempt.dialReturned = true
	attempt.dialResultKnown = dialErr != nil || raw != nil
	if dialErr != nil || raw == nil || attempt.ctx.Err() != nil || raw.SetDeadline(attempt.deadline) != nil {
		return
	}
	req, err := http.NewRequestWithContext(attempt.ctx, http.MethodGet, "http://"+attempt.m.port+"/connections", nil)
	if err != nil {
		return
	}
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+attempt.m.secret)
	if req.Write(raw) != nil || attempt.ctx.Err() != nil {
		return
	}
	// Fixed loopback HTTP/1: no Transport, idle pool, redirect, DNS, proxy,
	// compression negotiation or implicit retry owns an untracked socket.
	reader := bufio.NewReaderSize(io.LimitReader(raw, metricsFrameBytes), 4096)
	header, err := readMetricsHeader(reader)
	if err != nil {
		return
	}
	attempt.response, err = http.ReadResponse(bufio.NewReader(io.MultiReader(bytes.NewReader(header), reader)), req)
	resp := attempt.response
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode != http.StatusOK ||
		resp.ContentLength > metricsSnapshotBytes || resp.Header.Get("Content-Encoding") != "" ||
		resp.ProtoMajor != 1 || attempt.ctx.Err() != nil {
		return
	}
	attempt.snapshot, err = readMetricsSnapshot(attempt.ctx, resp.Body)
	attempt.observedAt = time.Now()
	attempt.readTimely = err == nil && attempt.ctx.Err() == nil && attempt.observedAt.Before(attempt.deadline)
}

// Admit complete CRLF-delimited headers before the stdlib MIME parser, without
// limiting only a prefix or losing bytes already buffered from the body.
func readMetricsHeader(reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, 0, 4096)
	lineBytes := 0
	for {
		line, err := reader.ReadSlice('\n')
		if len(line) > metricsHeaderBytes-len(header) {
			return nil, errMetricsSnapshot
		}
		header = append(header, line...)
		lineBytes += len(line)
		if err != nil {
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			return nil, errMetricsSnapshot
		}
		if lineBytes < 2 || header[len(header)-2] != '\r' {
			return nil, errMetricsSnapshot
		}
		if lineBytes == 2 {
			return header, nil
		}
		lineBytes = 0
	}
}

func (attempt *metricsAttempt) watch() {
	defer close(attempt.watchDone)
	defer func() {
		if payload := recover(); payload != nil || !attempt.watchReturned {
			attempt.failed.Store(true)
		}
	}()
	var intentDone, controlDone <-chan struct{}
	if attempt.scope != nil {
		intentDone = attempt.scope.subject.intentSource.Done()
		controlDone = attempt.scope.control.Done()
	}
	select {
	case <-attempt.ctx.Done():
	case <-intentDone:
	case <-controlDone:
	}
	attempt.cancel()
	attempt.closeSocket() // same original one-attempt Close as the finalizer
	attempt.watchReturned = true
}

// A concurrent caller observes the same closeDone; it neither waits inside the
// critical section nor retries an interrupted native Close.
func (attempt *metricsAttempt) closeSocket() {
	attempt.mu.Lock()
	conn := attempt.conn
	if conn == nil || attempt.closeStarted {
		attempt.mu.Unlock()
		return
	}
	attempt.closeStarted = true
	attempt.mu.Unlock()
	returned := false
	result := errMetricsSnapshot
	defer func() {
		if payload := recover(); payload != nil || !returned {
			attempt.failed.Store(true)
			result = errMetricsSnapshot
		}
		attempt.closeReturned, attempt.closeErr = returned, result
		close(attempt.closeDone)
	}()
	err := conn.Close()
	returned = true
	if err == nil || errors.Is(err, net.ErrClosed) {
		result = nil
	}
}

func (attempt *metricsAttempt) joinSocket() error {
	attempt.mu.Lock()
	hasSocket, started := attempt.conn != nil, attempt.closeStarted
	attempt.mu.Unlock()
	if !hasSocket {
		return nil
	}
	if !started {
		return errMetricsSnapshot
	}
	<-attempt.closeDone
	if !attempt.closeReturned || attempt.closeErr != nil {
		return errMetricsSnapshot
	}
	return nil
}

func (attempt *metricsAttempt) closeBody() (result error) {
	if attempt.response == nil || attempt.response.Body == nil {
		return nil
	}
	if attempt.bodyCloseStarted {
		return errMetricsSnapshot // no second call may supply a replacement receipt
	}
	attempt.bodyCloseStarted = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errMetricsSnapshot
			attempt.bodyCloseErr = errMetricsSnapshot
		}
		attempt.bodyCloseReturned = returned
	}()
	attempt.bodyCloseErr = attempt.response.Body.Close() // socket Close already joined, including on decode failure
	returned = true
	// ReadResponse's body owns only this reader, not a second socket/worker.
	// A returned drain error after our socket closed denies a snapshot; it does
	// not create an unknown native owner. Abnormal/unreturned Close does.
	return nil
}

func (attempt *metricsAttempt) finish(returned *bool) {
	payload := recover()
	sequence := &appCleanupSequence{}
	defer attempt.complete(sequence, *returned && payload == nil)
	defer appCleanupStep("join original metrics cancellation observer", sequence, func() error {
		if attempt.watchDone != nil {
			<-attempt.watchDone
			if !attempt.watchReturned {
				return errMetricsSnapshot
			}
		}
		return nil
	})
	defer appCleanupStep("close original metrics body", sequence, attempt.closeBody)
	defer appCleanupStep("join original metrics socket close", sequence, attempt.joinSocket)
	defer appCleanupStep("close original metrics socket", sequence, func() error { attempt.closeSocket(); return nil })
	defer appCleanupStep("cancel original metrics request", sequence, func() error { attempt.cancel(); return nil })
}

func (attempt *metricsAttempt) complete(sequence *appCleanupSequence, returned bool) {
	finished := false
	defer close(attempt.done)
	defer func() {
		if payload := recover(); payload != nil || !finished {
			attempt.failed.Store(true)
			attempt.retired = false
			attempt.view, attempt.result = MetricsView{}, errMetricsSnapshot
		}
	}()
	hasBody := attempt.response != nil && attempt.response.Body != nil
	known := returned && sequence.result == nil && !attempt.failed.Load() &&
		(!attempt.dialStarted || attempt.dialReturned && attempt.dialResultKnown) &&
		(!hasBody || attempt.bodyCloseStarted && attempt.bodyCloseReturned) &&
		(attempt.watchDone == nil || attempt.watchReturned)
	if !known {
		return // retain original response/socket/frame; no successor work
	}
	if attempt.readTimely && attempt.bodyCloseErr == nil && attempt.parent.Err() == nil && time.Now().Before(attempt.deadline) {
		// Sorting/strings/allocations happen before the value-only lifecycle lease.
		connections := metricsConnections(attempt.snapshot)
		if attempt.scope == nil {
			attempt.view = attempt.m.applyPrepared(attempt.snapshot, connections, attempt.observedAt)
			attempt.result = nil
		} else if release, ok := attempt.scope.lockCurrent(); ok {
			func() {
				defer release()
				contextView := attempt.scope.app.diagnosticContextLocked()
				if contextView != nil && time.Now().Before(attempt.deadline) {
					attempt.view = attempt.m.applyPrepared(attempt.snapshot, connections, attempt.observedAt)
					attempt.view.Collector, attempt.view.Context = attempt.m.id, contextView
					attempt.result = nil
				}
			}()
		}
	}
	metricsPolls.mu.Lock()
	defer metricsPolls.mu.Unlock()
	if metricsPolls.attempt != attempt {
		return
	}
	attempt.retired = true
	metricsPolls.attempt = nil // all original I/O/Close/watcher and publication frames returned
	finished = true
}

func (a *App) cancelMetrics(seal bool) *metricsAttempt {
	var attempt *metricsAttempt
	func() {
		// Match admission/publication's App -> registry order: terminal cannot
		// miss a late reservation or race a baseline commit after its seal.
		if seal {
			a.mu.Lock()
			defer a.mu.Unlock()
		}
		metricsPolls.mu.Lock()
		defer metricsPolls.mu.Unlock()
		if seal {
			a.metricsSealed.Store(true)
		}
		attempt = metricsPolls.attempt
		if attempt != nil && (attempt.scope == nil || attempt.scope.app != a) {
			attempt = nil
		}
	}()
	if attempt != nil {
		attempt.cancel() // no Close/Wait under App/registry/lifecycle value locks
	}
	return attempt
}

func joinMetrics(ctx context.Context, attempt *metricsAttempt) error {
	if attempt == nil {
		return nil
	}
	if ctx == nil {
		return errMetricsSnapshot
	}
	select {
	case <-attempt.done:
		if attempt.retired {
			return nil
		}
		return errMetricsSnapshot
	case <-ctx.Done():
		return errMetricsSnapshot // interest ended; original owner/slot remains
	}
}

func (a *App) retireMetrics() error {
	attempt := a.cancelMetrics(false)
	ctx, cancel := context.WithTimeout(context.Background(), metricsHTTPTimeout)
	defer cancel()
	return joinMetrics(ctx, attempt)
}
