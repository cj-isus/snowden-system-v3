package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const engineFileLogName = "engine.log"

func engineFileLogPath() string { return filepath.Join(logsDir(), engineFileLogName) }

type engineLogMsg struct {
	level string
	text  string
}

func classifyEngineLine(line string) string {
	switch {
	case strings.HasPrefix(line, "ERROR"), strings.HasPrefix(line, "FATAL"), strings.HasPrefix(line, "PANIC"):
		return "error"
	case strings.HasPrefix(line, "WARN"):
		return "warn"
	case strings.HasPrefix(line, "DEBUG"), strings.HasPrefix(line, "TRACE"):
		return ""
	default:
		return "info"
	}
}

// Original aliases stay source-compatible; task return is not a file-close
// receipt. Only this exact original done fence permits reading the I/O result.
func (a *App) stopEngineLogTailer() error {
	ctx, cancel := context.WithTimeout(context.Background(), terminalProducerTimeout)
	defer cancel()
	return a.joinEngineLogTailerContext(ctx, a.cancelEngineLogTailer())
}

func (a *App) cancelEngineLogTailer() <-chan struct{} {
	a.mu.Lock()
	stop, done := a.engineLogDone, a.engineLogRetired
	if stop != nil {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}
	a.mu.Unlock()
	return done
}

func (a *App) joinEngineLogTailerContext(ctx context.Context, done <-chan struct{}) error {
	a.mu.Lock()
	state, original := a.engineLogState, a.engineLogRetired
	a.mu.Unlock()
	if done == nil && state == nil && original == nil {
		return nil // no task was published
	}
	if state == nil || done == nil || done != original {
		return errAppProducerPending
	}
	if err := joinAppProducerContext(ctx, done, errAppProducerPending); err != nil {
		return err
	}
	err := state.failure
	if state.skipped != 0 {
		err = errors.Join(err, fmt.Errorf("engine log omitted %d oversized records", state.skipped))
	}
	return err
}

var engineLogPathAt = engineFileLogPath
var errEngineLogRead = errors.New("engine log read failed")

const (
	engineLogLineBytes = 64 << 10
	engineLogPassBytes = 256 << 10
	engineLogPassRows  = 128
)

// One original reader state, published before its task. Only that task mutates
// it; failure/partial file is retained instead of reopening a replacement.
type tailerState struct {
	offset     int64
	last       string
	identity   os.FileInfo // rotation hint, not native identity/ACL custody
	discarding bool
	skipped    uint64
	stop       <-chan struct{}
	operation  *engineLogRead
	failure    error
}

type engineLogRead struct{ originalLogFile }

func (state *tailerState) stopping() bool {
	select {
	case <-state.stop:
		return true
	default:
		return false
	}
}

// Regular partial rows keep their initial offset. Oversized rows are discarded
// in bounded chunks with one explicit omission count; do not accumulate them.
// The byte budget limits reads in this pass, not OS call latency or total log.
func readNewEngineLines(path string, state *tailerState) (out []engineLogMsg) {
	if state == nil || state.stopping() || state.failure != nil || state.operation != nil {
		return nil
	}
	op := new(engineLogRead)
	state.operation = op // before Open; retain exact partial acquisition/I/O
	var err error
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			err = errors.Join(err, errEngineLogRead)
		}
		err = errors.Join(err, op.retire())
		if err != nil {
			state.failure = errors.Join(errEngineLogRead, err)
			out = nil
			return
		}
		state.operation = nil // only after actual I/O return and confirmed Close
	}()
	if err = op.openFile(path, os.O_RDONLY, 0); err != nil {
		if os.IsNotExist(err) {
			err = nil // engine has not created the file; no acquisition to attest
		}
		returned = true
		return nil
	}
	out, err = op.read(state)
	returned = true
	return out
}

func (op *engineLogRead) read(state *tailerState) ([]engineLogMsg, error) {
	op.ioPending = true
	info, err := op.file.Stat()
	op.ioPending = false
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errEngineLogRead
	}
	if state.identity != nil && !os.SameFile(state.identity, info) || info.Size() < state.offset {
		state.offset, state.last, state.discarding = 0, "", false
	}
	state.identity = info
	op.ioPending = true
	_, err = op.file.Seek(state.offset, io.SeekStart)
	op.ioPending = false
	if err != nil {
		return nil, err
	}
	r := bufio.NewReaderSize(io.LimitReader(op.file, engineLogPassBytes), engineLogLineBytes)
	var out []engineLogMsg
	for rows := 0; rows < engineLogPassRows && !state.stopping(); {
		op.ioPending = true
		fragment, readErr := r.ReadSlice('\n')
		op.ioPending = false
		switch readErr {
		case nil:
			state.offset += int64(len(fragment))
			rows++
			if state.discarding {
				state.discarding = false
				continue
			}
			line := strings.TrimRight(string(fragment), "\r\n")
			if line == "" || line == state.last {
				continue
			}
			if level := classifyEngineLine(line); level != "" {
				out = append(out, engineLogMsg{level: level, text: "движок " + line})
			}
			state.last = line
		case bufio.ErrBufferFull:
			state.offset += int64(len(fragment))
			if !state.discarding {
				state.skipped++
				out = append(out, engineLogMsg{level: "warn", text: "движок: слишком длинная строка журнала пропущена"})
			}
			state.discarding, state.last = true, ""
		case io.EOF:
			if state.discarding {
				state.offset += int64(len(fragment))
			}
			return out, nil // a normal partial row is reread from its start
		default:
			return nil, readErr
		}
	}
	return out, nil
}

func (a *App) startEngineLogTailer() {
	a.mu.Lock()
	stop := a.engineLogDone
	if stop == nil || a.engineLogRetired != nil || a.engineLogState != nil || a.closing.Load() || a.quitRequested {
		a.mu.Unlock()
		return
	}
	select {
	case <-stop:
		a.mu.Unlock()
		return
	default:
	}
	done := make(chan struct{})
	state := &tailerState{stop: stop}
	a.engineLogState, a.engineLogRetired = state, done
	a.mu.Unlock()
	go func() {
		returned := false
		defer func() {
			if recovered := recover(); recovered != nil || !returned {
				state.failure = errors.Join(state.failure, errEngineLogRead)
			}
			close(done)
		}()
		a.runEngineLogTailer(state)
		returned = true
	}()
}

func (a *App) runEngineLogTailer(state *tailerState) {
	for !state.stopping() {
		messages := readNewEngineLines(engineLogPathAt(), state)
		if state.failure != nil {
			a.appendLog("warn", "чтение журнала движка остановлено: "+state.failure.Error())
			return // diagnostic failure, not a channel-health or protection verdict
		}
		for _, msg := range messages {
			if state.stopping() {
				return
			}
			a.appendLog(msg.level, msg.text)
		}
		if !a.sleepTill(500 * time.Millisecond) {
			return
		}
	}
}

func (a *App) sleepTill(d time.Duration) bool {
	select {
	case <-a.engineLogDone:
		return false
	case <-time.After(d):
		return true
	}
}
