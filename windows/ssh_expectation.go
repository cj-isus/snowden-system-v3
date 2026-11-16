package main

// Legacy operator hint about the vault private-value fingerprint, NOT an SSH
// host-key trust anchor. This bounded reader never grants trust or rotates pins.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/snowden-system/windows/backend/filetxn"
)

const maxSSHExpectationBytes = 4 << 10

type keyExpectation struct {
	PrivateKeyFingerprint string `json:"private_key_fingerprint"`
}

var (
	errSSHExpectationRead     = errors.New("Локальное ожидание недоступно: проверьте доступ к .tmp/expectations.v1.json рядом с EXE и в рабочем каталоге. SSH-запрос не отправлен.")
	errSSHExpectationFormat   = errors.New("Файл локального ожидания должен содержать один private_key_fingerprint: 16 строчных hex-символов, без других или повторяющихся полей. SSH-запрос не отправлен.")
	errSSHExpectationConflict = errors.New("Два допустимых файла локального ожидания содержат разные fingerprint. Устраните неоднозначность перед SSH-проверкой.")
)

// One process-lifetime domain/reader, not one slot per request/path. TryLock
// rejects contention without a waiting goroutine. Uncertain I/O retains both
// its reader and the original task; a successor must not claim that resource.
var sshExpectations = struct {
	mu     sync.Mutex
	reader *filetxn.BoundedReadSlot
	owner  *manualDiagnosticTask
}{reader: filetxn.NewBoundedReadSlot(maxSSHExpectationBytes)}

func expectationsPaths() ([]string, error) {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return nil, errSSHExpectationRead
	}
	besideExe, err := filepath.Abs(filepath.Join(filepath.Dir(exe), ".tmp", "expectations.v1.json"))
	if err != nil {
		return nil, errSSHExpectationRead
	}
	working, err := filepath.Abs(filepath.Join(".tmp", "expectations.v1.json"))
	if err != nil {
		return nil, errSSHExpectationRead
	}
	if besideExe == working {
		return []string{besideExe}, nil
	}
	return []string{besideExe, working}, nil // exact original candidates, captured once
}

// nil,nil means ONLY that every candidate was positively observed absent with
// its original reader retired. Denied/busy/oversized/malformed/conflicting
// input is not absence and must not silently bypass this existing local check.
func loadKeyExpectation(task *manualDiagnosticTask) (*keyExpectation, error) {
	if task == nil || task.currentError() != nil || !sshExpectations.mu.TryLock() {
		return nil, errSSHExpectationRead
	}
	defer sshExpectations.mu.Unlock()
	if sshExpectations.owner != nil || !sshExpectations.reader.Retired() {
		return nil, errSSHExpectationRead // keep the earlier original owner, not this caller
	}
	sshExpectations.owner = task
	defer func() {
		if sshExpectations.reader.Retired() {
			sshExpectations.owner = nil
		} else {
			task.cleanupFailed.Store(true) // shared finisher retains task/done/budget
		}
	}()
	paths, err := expectationsPaths()
	if err != nil {
		return nil, errSSHExpectationRead
	}
	var found *keyExpectation
	for _, path := range paths {
		if task.currentError() != nil {
			return nil, errSSHExpectationRead
		}
		raw, err := sshExpectations.reader.Read(path, maxSSHExpectationBytes)
		if !sshExpectations.reader.Retired() {
			return nil, errSSHExpectationRead // deferred original-owner fence, never fallback
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || task.currentError() != nil {
			return nil, errSSHExpectationRead
		}
		expectation, err := decodeSSHExpectation(raw)
		if err != nil {
			return nil, errSSHExpectationFormat
		}
		if found != nil && found.PrivateKeyFingerprint != expectation.PrivateKeyFingerprint {
			return nil, errSSHExpectationConflict
		}
		found = expectation
	}
	if task.currentError() != nil {
		return nil, errSSHExpectationRead
	}
	return found, nil
}

// Closed one-key object. Reuse the existing fingerprint validator; reject JSON
// aliases/duplicates/null/unknown members and trailing values before using it.
// No generic trust decoder is exported for this legacy shape in metadata.
func decodeSSHExpectation(raw []byte) (*keyExpectation, error) {
	if len(raw) == 0 || len(raw) > maxSSHExpectationBytes {
		return nil, errSSHExpectationFormat
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, errSSHExpectationFormat
	}
	var result keyExpectation
	seen := false
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil || name != "private_key_fingerprint" || seen {
			return nil, errSSHExpectationFormat
		}
		seen = true
		if decoder.Decode(&result.PrivateKeyFingerprint) != nil || !secretHashValid(result.PrivateKeyFingerprint) {
			return nil, errSSHExpectationFormat
		}
	}
	ending, err := decoder.Token()
	if err != nil || ending != json.Delim('}') || !seen || decoder.Decode(new(any)) != io.EOF {
		return nil, errSSHExpectationFormat
	}
	return &result, nil
}
