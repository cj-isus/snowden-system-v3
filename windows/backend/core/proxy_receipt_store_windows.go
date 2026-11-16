//go:build windows

package core

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"os"
	"runtime"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/filetxn"
	"golang.org/x/sys/windows"
)

// Serialized by proxyOwnerMu -> ProxyManager.mu and the same original Windows
// writer lease. No recursive WithWriter or system-proxy restore authority.
type proxyReceiptStore struct {
	path                            string
	fileInfo                        os.FileInfo
	lease                           *filetxn.WindowsWriterLease
	reader                          *filetxn.BoundedReadSlot
	writer                          *filetxn.BoundedWriteSlot
	crypto                          proxyReceiptCrypto
	primaryUser                     *windows.SID
	previous                        *proxyDurableReceipt // private unresolved history; never adopted as a new session
	priorProcess                    *proxyReceiptProcess
	subject                         *proxyReceiptSubject
	subjectConflict                 bool // sticky logical hold, distinct from unknown native completion
	anchor                          *proxyReceiptAnchor
	reconciliation                  *proxyAnchorReconciliation
	stage                           *proxyReceiptStage
	stageReplay                     *proxyStageReplay
	noWriteRetirement               *proxyNoWriteRetirement
	crashRecovery                   *proxyCrashRecovery
	writeRaw                        []byte // exact canonical writer input, including unknown publication
	anchorConflict, freshAnchor     bool
	record                          proxyDurableReceipt
	pending, opened, active, closed bool
	failure                         error
}

func newProxyReceiptStore(path string, lease *filetxn.WindowsWriterLease) *proxyReceiptStore {
	return &proxyReceiptStore{path: path, lease: lease, anchor: newProxyReceiptAnchor(), stage: newProxyReceiptStage(path), reader: filetxn.NewBoundedReadSlot(maxProxyReceiptCipher + int64(len(proxyCipherPurpose))), writer: filetxn.NewBoundedWriteSlot(maxProxyReceiptCipher + int64(len(proxyCipherPurpose)))}
}

// The caller retains this store before entry, including unknown identity/file/
// crypto completion. A free storage lease never proves the old process died.
func (store *proxyReceiptStore) open() (result error) {
	if store == nil || store.opened || store.pending || store.closed {
		return ErrProxyReceiptUnavailable
	}
	store.pending = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
		if returned && store.crashReturned() && store.noWriteReturned() && store.replayReturned() && store.stage.idle() && store.reconciliationReturned() && store.priorProcessRetired() && store.crypto.retired() && store.reader.Retired() && store.anchor.idle() && store.writer.Check() == nil {
			store.pending = false
		}
		if store.pending {
			result = errors.Join(result, ErrProxyStateUncertain)
		}
		store.failure = result
	}()
	sid, err := fileacl.CurrentUser()
	if err != nil || sid == nil {
		returned = true
		return ErrProxyReceiptUnavailable
	}
	store.primaryUser = sid
	header := unsafe.Slice((*byte)(unsafe.Pointer(sid)), 8)
	if header[0] != 1 || header[1] > 15 {
		returned = true
		return ErrProxyReceiptUnavailable
	}
	store.record.sid = bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(sid)), 8+4*int(header[1])))
	runtime.KeepAlive(sid)
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		returned = true
		return ErrProxyReceiptUnavailable
	}
	store.record.pid = windows.GetCurrentProcessId()
	store.record.creation = uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
	if _, err := rand.Read(store.record.nonce[:]); err != nil || store.record.pid == 0 || store.record.creation == 0 || store.record.nonce == [16]byte{} {
		returned = true
		return ErrProxyReceiptUnavailable
	}
	if err := store.validateLease(); err != nil {
		returned = true
		return err
	}
	if err := store.anchor.open(); err != nil {
		returned = true
		return errors.Join(ErrProxyAnchorUnavailable, err)
	}
	input, err := store.loadJournal()
	if err != nil {
		returned = true
		return err
	}
	if input.missing {
		store.opened, returned = true, true
		return nil
	}
	store.fileInfo = input.info
	if proxyReceiptHasNoWrites(input.record) {
		input, err = store.retireNoWrite(input)
		if err != nil {
			returned = true
			return err
		}
	}
	raw, info, previous, plainDigest := input.raw, input.info, input.record, input.digest
	if previous.phase != proxyReceiptClosed && previous.phase != proxyReceiptForeign {
		store.previous = &previous // retain the exact decoded subject before process acquisition
		store.priorProcess = &proxyReceiptProcess{expectedPID: previous.pid, expectedCreation: previous.creation, currentUser: store.primaryUser}
		store.subject = &proxyReceiptSubject{store: store, lease: store.lease, reader: store.reader, writer: store.writer, anchor: store.anchor,
			anchorSequence: store.anchor.state.sequence, anchorSHA256: sha256.Sum256(store.anchor.current), path: store.path, info: info,
			cipherSHA256: sha256.Sum256(raw), plainSHA256: plainDigest, previous: store.previous,
			observer: store.priorProcess, nonce: previous.nonce, generation: previous.generation}
		err := store.priorProcess.observe()
		if store.priorProcess.retired() {
			if sourceErr := store.subject.revalidate(); sourceErr != nil {
				returned = true
				return sourceErr // no stale finite process classification can replace the original subject
			}
			if errors.Is(err, ErrProxyRecoveryRequired) {
				store.crashRecovery = newProxyCrashRecovery(store, input)
				if store.crashRecovery != nil {
					returned = true
					return nil // private pending admission only; acquireWriterLocked must run it before Snapshot
				}
			}
		}
		returned = true
		return errors.Join(ErrProxyReceiptUnavailable, err) // no liveness outcome is recovery authority
	}
	// A finished record permits a fresh snapshot, never recovery. DPAPI does not
	// establish anti-rollback/freshness or protect against a malicious same user.
	store.opened, returned = true, true
	return nil
}

func (store *proxyReceiptStore) priorProcessRetired() bool {
	return store != nil && (store.priorProcess == nil || store.priorProcess.retired())
}

func (store *proxyReceiptStore) validateLease() error {
	if store == nil || store.lease == nil {
		return ErrProxyStateUncertain
	}
	if err := store.lease.Validate(); err != nil {
		return errors.Join(ErrProxyStateUncertain, err)
	}
	return nil
}

// Normal session admission for the shared private journal publisher. The next
// record becomes current only after original publication, Close and anchor commit.
func (store *proxyReceiptStore) write(next proxyDurableReceipt) (result error) {
	if store == nil || !store.opened || store.pending || store.closed || store.failure != nil || store.previous != nil || store.subjectConflict || store.anchorConflict || !store.crashReturned() || !store.noWriteReturned() || !store.replayReturned() || !store.stage.idle() || !store.priorProcessRetired() || !store.reader.Retired() || !store.crypto.retired() || !store.anchor.idle() {
		return ErrProxyReceiptUnavailable
	}
	store.pending = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = ErrProxyStateUncertain
		}
		if returned && store.crashReturned() && store.noWriteReturned() && store.replayReturned() && store.stage.idle() && store.crypto.retired() && store.reader.Retired() && store.anchor.idle() && store.writer.Check() == nil {
			store.pending = false
		}
		if store.pending {
			result = errors.Join(result, ErrProxyStateUncertain)
		}
		if result != nil {
			store.failure = result
		}
	}()
	published, err := store.publishJournal(next, nil, nil)
	if err != nil {
		returned = true
		return err
	}
	store.fileInfo = published.info
	store.record = next
	store.writeRaw = nil // original file publication/readback and anchor commit completed
	returned = true
	return nil
}

func (store *proxyReceiptStore) begin(original proxyRegistryState) error {
	if store == nil || store.active || !store.opened {
		return ErrProxyReceiptUnavailable
	}
	next := store.record
	next.generation, next.phase, next.index = 1, proxyReceiptActive, proxyReceiptNoIndex
	next.original, next.current, next.target = original, original, original
	if err := store.write(next); err != nil {
		return err
	}
	store.active = true
	return nil
}

func (store *proxyReceiptStore) ready(current proxyRegistryState) bool {
	return store != nil && store.active && store.opened && !store.closed && !store.pending && store.failure == nil &&
		store.previous == nil && !store.subjectConflict && !store.anchorConflict && store.crashReturned() && store.noWriteReturned() && store.replayReturned() && store.stage.idle() && store.anchor != nil && store.anchor.loaded && !store.anchor.state.pending && store.anchor.idle() && store.priorProcessRetired() && store.record.phase == proxyReceiptActive &&
		store.record.current.equal(current) && store.crypto.retired() && store.reader.Retired()
}

func (store *proxyReceiptStore) prepare(index int, before, target proxyRegistryState) error {
	if !store.ready(before) || index < 0 || index >= 3 || store.record.generation == ^uint64(0) {
		return ErrProxyReceiptUnavailable
	}
	next := store.record
	next.generation++
	next.phase, next.index, next.target = proxyReceiptPrepared, byte(index), target
	return store.write(next)
}

func (store *proxyReceiptStore) confirm(observed proxyRegistryState, changed bool) error {
	if store == nil || !store.active || store.record.phase != proxyReceiptPrepared || store.record.generation == ^uint64(0) {
		return ErrProxyReceiptUnavailable
	}
	next := store.record
	expected := next.current
	if changed {
		expected = next.target
	}
	if !observed.equal(expected) {
		return ErrProxyReceiptUnavailable
	}
	if changed {
		next.written |= 1 << next.index
	}
	next.generation++
	next.phase, next.index = proxyReceiptActive, proxyReceiptNoIndex
	next.current, next.target = observed, observed
	return store.write(next)
}

func (store *proxyReceiptStore) finish(outcome string, current proxyRegistryState) error {
	if store == nil {
		return ErrProxyReceiptUnavailable
	}
	if !store.active { // a failed load must never tombstone somebody else's unfinished record
		if store.anchorConflict {
			return ErrProxyAnchorUnavailable
		}
		if store.subjectConflict {
			// Keep the original lease/subject for explicit recovery.
			return ErrProxyReceiptSubjectChanged
		}
		if !store.operationsReturned() || store.writer.Check() != nil {
			return ErrProxyStateUncertain
		}
		if err := store.closeAnchor(); err != nil {
			return err
		}
		store.closed = true
		return nil
	}
	if !store.ready(current) || store.record.generation == ^uint64(0) {
		return ErrProxyStateUncertain
	}
	next := store.record
	next.generation++
	next.index, next.phase = proxyReceiptNoIndex, proxyReceiptClosed
	switch outcome {
	case proxyForeignPreserved:
		next.phase = proxyReceiptForeign // records preservation, does not claim the foreign registry tuple
	case proxyRestored:
		next.current = next.original // caller observed original or confirmed each restore write
	case proxyNotWritten:
		if next.written != 0 || !next.current.equal(next.original) {
			return ErrProxyStateUncertain
		}
	default:
		return ErrProxyStateUncertain
	}
	next.target = next.current
	if err := store.write(next); err != nil {
		return err
	}
	if err := store.closeAnchor(); err != nil {
		return err
	}
	store.active, store.closed = false, true
	return nil
}
