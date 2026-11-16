package core

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
)

// Shared completion header for one original owned pipe goroutine. The writer
// publishes err before closing done; consumers read it only after that close.
// A notification/source/timeout is not a substitute for actual I/O return.
type ownedPipeIO struct {
	done    chan struct{}
	err     error
	effects *ownedPipeEffects // original raw-I/O facts; read plain fields only after done (legacy nil allowed)
}

// One original exchange's writer is its I/O goroutine. Only writeEntered
// is read while that goroutine may run; all other fields require its done fence.
// A defer closing done cannot stand in for an underlying Read/Write returning.
type ownedPipeEffects struct {
	writeEntered                                                    atomic.Bool
	written, read                                                   int
	writePending, readPending, writeReturned, readReturned, invalid bool
	closePending, closeReturned                                     bool // original delivery input EOF close, same I/O goroutine
}

func (effects *ownedPipeEffects) retirementError() error {
	if effects != nil && (effects.writePending || effects.readPending || effects.closePending) {
		return ErrHelperCleanup
	}
	return nil
}

// Only the original done fence publishes plain effect fields. Finite protocol
// errors can be retired; unfinished I/O and an unreturned raw call cannot.
func (owner *ownedPipeIO) retirementReady() error {
	if owner == nil {
		return nil
	}
	select {
	case <-owner.done:
		return owner.effects.retirementError()
	default:
		return ErrHelperCleanup
	}
}

func (process *engineProcess) captureFailure() error {
	if process == nil {
		return nil
	}
	return process.captureIO.completedFailure()
}

func (owner *ownedPipeIO) completedFailure() error {
	if owner == nil {
		return nil
	}
	select {
	case <-owner.done:
		return errors.Join(owner.err, owner.effects.retirementError())
	default:
		return nil // unknown unfinished cause, never an I/O-retirement receipt
	}
}

var errHelperOutputIncomplete = errors.New("native helper output incomplete")

type helperOutputRead struct {
	ownedPipeIO
	buffer []byte
	data   []byte // alias of the fixed buffer, published only by done
}

// Caller validated 1 <= maximum <= 64KiB. Allocate the complete output plus one
// overflow byte BEFORE launching a process or pipe goroutine. No ReadAll growth.
func newHelperOutputRead(maximum int64) *helperOutputRead {
	return &helperOutputRead{ownedPipeIO: ownedPipeIO{done: make(chan struct{}), effects: new(ownedPipeEffects)}, buffer: make([]byte, int(maximum)+1)}
}

// Only the existing original reader invokes this method. A frame/output error
// retains its finite cause alongside cancellation; raw pipe paths are omitted.
func (read *helperOutputRead) run(ctx context.Context, output io.Reader) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			read.data = nil
			read.err = errors.Join(read.err, errors.New("native helper read interrupted"))
		}
		close(read.done)
	}()
	n, err := io.ReadFull(ownedPipeReader{ctx: ctx, reader: output, failure: errHelperOutputIncomplete, effects: read.effects}, read.buffer)
	switch {
	case n == len(read.buffer):
		read.err = ErrHelperLimit // max+1 bytes observed; never return the prefix as success
	case err == io.EOF || err == io.ErrUnexpectedEOF:
		read.data = read.buffer[:n:n] // actual EOF, including empty or exactly max output
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		read.err = err // scope reader retains any independent finite read cause
	default:
		read.err = errHelperOutputIncomplete
	}
	read.err = errors.Join(read.err, contextError(ctx))
	if read.err != nil {
		read.data = nil
	}
	completed = true
}

// The source is checked around EACH pipe read, not merely when the goroutine
// is queued. No App callback or lock is acquired in this I/O goroutine. Already
// entered kernel reads still require the independent original-process reaper.
type ownedPipeReader struct {
	ctx     context.Context
	reader  io.Reader
	failure error // finite protocol-specific cause; no raw pipe path on mixed scope failure
	effects *ownedPipeEffects
}

func (reader ownedPipeReader) Read(buffer []byte) (int, error) {
	if err := contextError(reader.ctx); err != nil {
		return 0, err
	}
	failure := reader.failure
	if failure == nil {
		failure = errHelperOutputIncomplete
	}
	if reader.effects != nil {
		reader.effects.readPending = true
	}
	n, err := reader.reader.Read(buffer)
	if reader.effects != nil {
		reader.effects.readPending, reader.effects.readReturned = false, true
		if n < 0 || n > len(buffer) {
			reader.effects.invalid = true
		} else {
			reader.effects.read += n
		}
	}
	if n < 0 || n > len(buffer) {
		return 0, errors.Join(failure, contextError(reader.ctx))
	}
	if currentErr := contextError(reader.ctx); currentErr != nil {
		if err != nil && err != io.EOF {
			return n, errors.Join(failure, currentErr)
		}
		return n, currentErr
	}
	if n == 0 && err == nil && len(buffer) != 0 {
		return 0, failure // owned blocking pipe made no progress; never spin
	}
	return n, err
}

// Same negative gates around each actual write, including protocol header/body
// writes. A partial/unknown command still poisons its original protocol owner;
// these checks neither undo bytes nor interrupt an entered kernel write.
type ownedPipeWriter struct {
	ctx     context.Context
	writer  io.Writer
	failure error
	effects *ownedPipeEffects
}

func (writer ownedPipeWriter) Write(buffer []byte) (int, error) {
	if err := contextError(writer.ctx); err != nil {
		return 0, err
	}
	failure := writer.failure
	if failure == nil {
		failure = errHelperOutputIncomplete
	}
	if writer.effects != nil {
		writer.effects.writeEntered.Store(true)
		writer.effects.writePending = true
	}
	n, err := writer.writer.Write(buffer)
	if writer.effects != nil {
		writer.effects.writePending, writer.effects.writeReturned = false, true
		if n < 0 || n > len(buffer) {
			writer.effects.invalid = true
		} else {
			writer.effects.written += n
		}
	}
	if n < 0 || n > len(buffer) {
		return 0, errors.Join(failure, contextError(writer.ctx))
	}
	if err == nil && n != len(buffer) {
		err = io.ErrShortWrite
	}
	if currentErr := contextError(writer.ctx); currentErr != nil {
		if err != nil {
			if errors.Is(err, io.ErrShortWrite) {
				failure = errors.Join(failure, io.ErrShortWrite)
			}
			return n, errors.Join(failure, currentErr)
		}
		return n, currentErr
	}
	return n, err
}
