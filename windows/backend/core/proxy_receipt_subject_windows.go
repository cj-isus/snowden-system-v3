//go:build windows

package core

import (
	"crypto/sha256"
	"errors"
	"os"

	"github.com/snowden-system/windows/backend/filetxn"
)

// Private consistency subject, NOT a restoration capability. This binds an
// already authenticated unresolved record to the original same-read file facts,
// store, storage lease and process observer. Nothing here asserts global latest,
// persistence across rollback/power loss, current registry ownership or OS CAS.
type proxyReceiptSubject struct {
	store                     *proxyReceiptStore
	lease                     *filetxn.WindowsWriterLease
	reader                    *filetxn.BoundedReadSlot
	writer                    *filetxn.BoundedWriteSlot
	anchor                    *proxyReceiptAnchor
	anchorSequence            uint64
	anchorSHA256              [sha256.Size]byte
	path                      string
	info                      os.FileInfo
	cipherSHA256, plainSHA256 [sha256.Size]byte
	previous                  *proxyDurableReceipt
	observer                  *proxyReceiptProcess
	nonce                     [16]byte
	generation                uint64
	failure                   error
}

func sameProxyReceiptFile(original, observed os.FileInfo) bool {
	return filetxn.SameWindowsFile(original, observed) && original.Size() == observed.Size() && original.ModTime().Equal(observed.ModTime())
}

// Value/pure-codec checks only. The native metadata is the existing reader's
// immutable Go snapshot; SameWindowsFile neither opens a path nor acquires I/O.
func (subject *proxyReceiptSubject) matchesMemory() bool {
	if subject == nil || subject.store == nil || subject.lease == nil || subject.reader == nil || subject.writer == nil || subject.anchor == nil || subject.previous == nil || subject.observer == nil ||
		subject.store.lease != subject.lease || subject.store.path != subject.path || subject.store.previous != subject.previous ||
		subject.store.reader != subject.reader || subject.store.writer != subject.writer ||
		subject.store.anchor != subject.anchor || subject.store.anchorConflict || subject.anchor.state.pending || subject.anchor.state.sequence != subject.anchorSequence || sha256.Sum256(subject.anchor.current) != subject.anchorSHA256 ||
		subject.store.priorProcess != subject.observer || subject.store.subject != subject || subject.store.subjectConflict || subject.store.active || subject.store.opened || subject.store.closed ||
		subject.previous.nonce != subject.nonce || subject.previous.generation != subject.generation ||
		!filetxn.SameWindowsFile(subject.info, subject.info) || !subject.info.Mode().IsRegular() ||
		subject.info.Size() <= int64(len(proxyCipherPurpose)) || subject.info.Size() > maxProxyReceiptCipher+int64(len(proxyCipherPurpose)) {
		return false
	}
	plain, err := encodeProxyReceipt(*subject.previous)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(plain)
	clear(plain) // temporary Go bytes only; no native consumer owns this buffer
	return digest == subject.plainSHA256
}

// Caller holds the same original proxy locks/lease throughout intake. Repeatable
// only after known reader retirement; any failure is sticky for this subject.
// No registry access, crypto fallback, candidate replacement or file repair.
func (subject *proxyReceiptSubject) revalidate() (result error) {
	if subject == nil || subject.failure != nil {
		return ErrProxyReceiptSubjectChanged
	}
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(ErrProxyReceiptSubjectChanged, ErrProxyStateUncertain)
		}
		if result != nil {
			subject.failure = result
			if subject.store != nil {
				// Retain this original logical owner; no new capture.
				subject.store.subjectConflict = true
			}
		}
	}()
	if !subject.matchesMemory() || !subject.observer.retired() || !subject.store.reader.Retired() {
		returned = true
		return ErrProxyReceiptSubjectChanged
	}
	if err := subject.lease.Validate(); err != nil {
		returned = true
		return errors.Join(ErrProxyReceiptSubjectChanged, ErrProxyStateUncertain)
	}
	if err := subject.anchor.check(); err != nil {
		returned = true
		return errors.Join(ErrProxyReceiptSubjectChanged, err)
	}
	raw, info, err := subject.store.reader.ReadSnapshot(subject.path, maxProxyReceiptCipher+int64(len(proxyCipherPurpose)))
	if err != nil {
		returned = true
		return errors.Join(ErrProxyReceiptSubjectChanged, err)
	}
	if !sameProxyReceiptFile(subject.info, info) || sha256.Sum256(raw) != subject.cipherSHA256 || !subject.matchesMemory() {
		returned = true
		return ErrProxyReceiptSubjectChanged
	}
	if err := subject.lease.Validate(); err != nil {
		returned = true
		return errors.Join(ErrProxyReceiptSubjectChanged, ErrProxyStateUncertain)
	}
	if err := subject.anchor.check(); err != nil {
		returned = true
		return errors.Join(ErrProxyReceiptSubjectChanged, err)
	}
	returned = true
	return nil
}
