package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var errAppLogWrite = errors.New("log write failed")
var errAppLogRetirement = errors.New("original log writer outcome or retirement unconfirmed")

type appLogWriter struct {
	input   chan LogLine
	done    chan struct{}
	result  error // read only after this original done fence
	dropped atomic.Uint64
}

// One process-wide append domain. Publication/replacement writers are not an
// append API. Retain a partial operation/file after error; never reset this slot.
var appLogDisk struct {
	sync.Mutex
	path      string
	current   *appLogBatch
	failure   error
	wroteData bool
}

type appLogBatch struct {
	originalLogFile
	buffer *bufio.Writer

	flushAttempted, flushReturned bool
	syncAttempted, syncReturned   bool
}

func (a *App) startLogWriter() {
	a.mu.Lock()
	if a.logWriter != nil || a.logFileCh != nil || a.logFileDone != nil || a.closing.Load() || a.quitRequested {
		a.mu.Unlock()
		return
	}
	owner := &appLogWriter{input: make(chan LogLine, 1024), done: make(chan struct{})}
	// Early logs are already bounded by the existing 500-line memory history.
	// Queue them before scheduling; do no disk I/O under the App value lock.
	for _, line := range a.logs {
		owner.input <- line
	}
	a.logWriter, a.logFileCh, a.logFileDone = owner, owner.input, owner.done
	a.mu.Unlock()
	go owner.run()
}

func (owner *appLogWriter) run() {
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			owner.result = errors.Join(owner.result, errAppLogWrite)
		}
		close(owner.done) // task return, not an invented successful flush
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	pending := make([]LogLine, 0, 64)
	flush := func(final bool) error {
		err := persistLogLines(pending, final)
		if err == nil {
			pending = pending[:0]
		}
		return err
	}
	for {
		select {
		case line, ok := <-owner.input:
			if !ok {
				owner.result = flush(true)
				returned = true
				return
			}
			pending = append(pending, line)
			if len(pending) == 64 {
				if err := flush(false); err != nil {
					owner.result, returned = err, true
					return // keep original failed storage slot; no replacement loop
				}
			}
		case <-ticker.C:
			if err := flush(false); err != nil {
				owner.result, returned = err, true
				return
			}
		}
	}
}

func (a *App) stopLogWriter() error {
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	return a.stopLogWriterContext(ctx)
}

func (a *App) stopLogWriterContext(ctx context.Context) error {
	a.mu.Lock()
	owner := a.logWriter
	if owner == nil {
		unknown := a.logFileCh != nil || a.logFileDone != nil
		a.mu.Unlock()
		if unknown {
			return errAppLogRetirement
		}
		return nil // no original disk writer was published
	}
	if a.logFileDone != owner.done || a.logFileCh != nil && a.logFileCh != owner.input {
		a.mu.Unlock()
		return errAppLogRetirement
	}
	if a.logFileCh != nil {
		a.logFileCh = nil
		close(owner.input) // serialized with appendLogLocked
	}
	a.mu.Unlock()
	if err := joinAppProducerContext(ctx, owner.done, errAppLogRetirement); err != nil {
		return err // original task/file remains owned; do not read mutable result
	}
	err := owner.result
	if count := owner.dropped.Load(); count != 0 {
		err = errors.Join(err, fmt.Errorf("log disk queue omitted %d records", count))
	}
	return err
}

// Rotation is checked before each batch. The batch can exceed this threshold;
// this is not a hard disk quota or interprocess writer/rename lock.
var maxAppLogSize int64 = 64 << 20

func rotateAppLog(dir string) error {
	path := filepath.Join(dir, "app.log")
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Size() < maxAppLogSize {
		return nil
	}
	return os.Rename(path, path+".1")
}

// Ordinary batches Flush/Close; only final drain also requests Sync. A final
// empty batch syncs prior data without creating an empty replacement file.
func persistLogLines(lines []LogLine, finalSync ...bool) (finalErr error) {
	if len(finalSync) > 1 {
		return errAppLogWrite
	}
	syncFile := len(finalSync) == 1 && finalSync[0]
	if len(lines) == 0 && !syncFile {
		return nil
	}
	appLogDisk.Lock()
	defer appLogDisk.Unlock()
	if appLogDisk.failure != nil || appLogDisk.current != nil {
		return errors.Join(errAppLogRetirement, appLogDisk.failure)
	}
	if len(lines) == 0 && !appLogDisk.wroteData {
		return nil // no file/data acquisition to attest
	}
	op := new(appLogBatch)
	appLogDisk.current = op // before path/directory/rotation/Open/write work
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			finalErr = errors.Join(finalErr, errAppLogWrite)
		}
		appLogDisk.failure = errAppLogRetirement // sticky during uncertain Close
		if err := op.retire(); err != nil {
			finalErr = errors.Join(finalErr, errAppLogRetirement, err)
		}
		if finalErr != nil {
			appLogDisk.failure = finalErr
			return
		}
		appLogDisk.current, appLogDisk.failure = nil, nil
		appLogDisk.wroteData = appLogDisk.wroteData || len(lines) != 0
	}()
	if appLogDisk.path == "" {
		appLogDisk.path = filepath.Join(logsDir(), "app.log")
	}
	finalErr = op.append(appLogDisk.path, lines, syncFile)
	returned = true
	return finalErr
}

func (op *appLogBatch) append(path string, lines []LogLine, syncFile bool) error {
	flags := os.O_WRONLY
	if len(lines) != 0 {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := rotateAppLog(filepath.Dir(path)); err != nil {
			return err // do not grow a file after failed rotation
		}
		flags |= os.O_CREATE | os.O_APPEND
	}
	if err := op.openFile(path, flags, 0o600); err != nil {
		return err
	}
	op.buffer = bufio.NewWriter(op.file)
	for _, line := range lines {
		op.ioPending = true
		_, err := fmt.Fprintf(op.buffer, "%s [%s] %s\n", line.T, line.Level, line.Text)
		op.ioPending = false
		if err != nil {
			return err
		}
	}
	op.flushAttempted, op.ioPending = true, true
	err := op.buffer.Flush()
	op.flushReturned, op.ioPending = true, false
	if err != nil {
		return err
	}
	if syncFile {
		op.syncAttempted, op.ioPending = true, true
		err = op.file.Sync()
		op.syncReturned, op.ioPending = true, false
	}
	return err
}
