package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/snowden-system/windows/backend/filetxn"
)

const (
	strictMarkerMaxBytes         = 1024
	strictMarkerRecoveryMaxBytes = 16 << 20 // metadata-only removal; no content allocation/read
)

// One storage domain, not WFP enforcement or another lifecycle controller.
// Never replace these original slots after an uncertain acquisition or cleanup.
var strictMarkers = strictMarkerStorage{
	reader:  filetxn.NewBoundedReadSlot(strictMarkerMaxBytes),
	writer:  filetxn.NewBoundedWriteSlot(strictMarkerMaxBytes),
	remover: filetxn.NewWindowsRemovalSlot(strictMarkerRecoveryMaxBytes),
}

type strictMarkerStorage struct {
	mu        sync.Mutex
	pending   atomic.Bool
	uncertain atomic.Bool
	reader    *filetxn.BoundedReadSlot
	writer    *filetxn.BoundedWriteSlot
	remover   *filetxn.WindowsRemovalSlot
}

// Callers hold no App lock across this domain. TryLock prevents a second local
// operation from queuing behind native cleanup; WithWriter retains the existing
// fail-immediate Windows byte-range lock and original parent/file owners across
// the complete callback. Older/uncooperative writers are not covered by that lock.
func (store *strictMarkerStorage) use(path string, releasing bool, work func() error) (finalErr error) {
	if !store.mu.TryLock() {
		return filetxn.ErrWriterBusy
	}
	defer store.mu.Unlock()
	store.pending.Store(true)
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			finalErr = errors.Join(finalErr, filetxn.ErrWriteRetirement)
		}
		if finalErr != nil {
			store.uncertain.Store(true)
		} else if releasing {
			store.uncertain.Store(false) // still serialized against a successor Arm
		}
		store.pending.Store(false)
	}()
	finalErr = filetxn.WithWriter(path, work)
	returned = true
	return finalErr
}

func (store *strictMarkerStorage) arm(ctx context.Context, path, exe string, apply func(string) error) error {
	if err := wfpOperationScope(ctx); err != nil {
		return err
	}
	if apply == nil {
		return errProtectionRecoveryRequired
	}
	return store.use(path, false, func() error {
		if err := wfpOperationScope(ctx); err != nil {
			return err
		}
		data, original, err := store.reader.ReadSnapshot(path, strictMarkerMaxBytes)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %w", errProtectionRecoveryRequired, err)
		}
		if errors.Is(err, os.ErrNotExist) {
			data, err = json.Marshal(killSwitchState{AppliedAt: time.Now(), Policy: "strict-wfp"})
			if err != nil {
				return err
			}
			if err := wfpOperationScope(ctx); err != nil {
				return err
			}
			// WriteNew flushes the original temporary and published file and
			// requires checked Close; occupied names are never overwritten.
			if err := store.writer.WriteNew(path, data); err != nil {
				return fmt.Errorf("WFP strict marker: %w", err)
			}
			var stored []byte
			stored, original, err = store.reader.ReadSnapshot(path, strictMarkerMaxBytes)
			if err != nil || !bytes.Equal(stored, data) {
				return errors.Join(errProtectionRecoveryRequired, err)
			}
		}
		if !validStrictMarker(data) {
			return errProtectionRecoveryRequired
		}
		// A late marker is retained intent, never permission to apply stale WFP.
		if err := wfpOperationScope(ctx); err != nil {
			return err
		}
		applyErr := apply(exe)
		if scopeErr := wfpOperationScope(ctx); scopeErr != nil {
			return errors.Join(applyErr, scopeErr)
		}
		current, info, readErr := store.reader.ReadSnapshot(path, strictMarkerMaxBytes)
		if readErr != nil || !filetxn.SameWindowsFile(original, info) || !bytes.Equal(data, current) {
			return errors.Join(applyErr, errProtectionRecoveryRequired, readErr)
		}
		return errors.Join(applyErr, wfpOperationScope(ctx))
	})
}

func (store *strictMarkerStorage) release(path string, removeWFP func() error, legacy KillSwitchRunner) error {
	if removeWFP == nil || legacy == nil {
		return errProtectionRecoveryRequired
	}
	return store.use(path, true, func() error {
		// The original marker handle and ancestors stay held through both
		// backends. Malformed/empty markers need no decoding for explicit
		// manual recovery. Size/ACL/reparse/hardlink uncertainty fails closed.
		return store.remover.RemoveAfter(path, func() error {
			wfpErr := removeWFP()
			legacyErr := removeKillSwitch(legacy)
			if err := errors.Join(wfpErr, legacyErr); err != nil {
				return fmt.Errorf("kill switch cleanup: %w", err)
			}
			return nil
		})
	})
}

// Same token-first exact-field policy as the existing worker protocol, scoped
// here to three scalar marker fields. No alias, duplicate, null, unknown field
// or trailing object may silently change retained intent. Time is format only,
// never freshness, trust, enforcement or automatic-expiry authority.
func validStrictMarker(raw []byte) bool {
	if len(raw) == 0 || len(raw) > strictMarkerMaxBytes || !utf8.Valid(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	var seen [3]bool
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return false
		}
		index := -1
		switch name {
		case "policy":
			index = 0
		case "applied_at":
			index = 1
		case "helper_pid":
			index = 2
		}
		if index < 0 || seen[index] {
			return false
		}
		seen[index] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
		switch index {
		case 0:
			var policy string
			if json.Unmarshal(value, &policy) != nil || policy != "strict-wfp" {
				return false
			}
		case 1:
			var applied time.Time
			if json.Unmarshal(value, &applied) != nil || applied.IsZero() {
				return false
			}
		case 2:
			var pid int
			if json.Unmarshal(value, &pid) != nil || pid != 0 {
				return false
			}
		}
	}
	end, err := decoder.Token()
	return err == nil && end == json.Delim('}') && seen[0] && seen[1] && decoder.Decode(new(any)) == io.EOF
}
