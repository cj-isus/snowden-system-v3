//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

const (
	proxyAnchorPurpose   = "snowden/proxy/registry-anchor/v1\x00"
	maxProxyAnchorPlain  = 512
	maxProxyAnchorCipher = 4096
)

type proxyAnchorSubject struct {
	nonce      [16]byte
	generation uint64
	cipher     [sha256.Size]byte
	phase      byte
}

func (subject proxyAnchorSubject) empty() bool { return subject == (proxyAnchorSubject{}) }
func (subject proxyAnchorSubject) valid() bool {
	return subject.nonce != [16]byte{} && subject.generation != 0 && subject.cipher != [sha256.Size]byte{} &&
		subject.phase >= proxyReceiptActive && subject.phase <= proxyReceiptForeign
}

// DPAPI-protected independent registry state. A pending pair is an explicit
// crash-recovery obligation; neither member is silently adopted on restart.
type proxyAnchorState struct {
	sequence          uint64
	path              [sha256.Size]byte
	sid               []byte
	committed, target proxyAnchorSubject
	pending           bool
}

func validProxyAnchorTransition(before, after proxyAnchorSubject) bool {
	if !after.valid() {
		return false
	}
	if before.empty() {
		return after.generation == 1 && after.phase == proxyReceiptActive
	}
	if !before.valid() {
		return false
	}
	if before.nonce != after.nonce {
		return (before.phase == proxyReceiptClosed || before.phase == proxyReceiptForeign) && after.generation == 1 && after.phase == proxyReceiptActive
	}
	if before.generation == ^uint64(0) || after.generation != before.generation+1 {
		return false
	}
	return before.phase == proxyReceiptActive && (after.phase == proxyReceiptPrepared || after.phase == proxyReceiptClosed || after.phase == proxyReceiptForeign) ||
		before.phase == proxyReceiptPrepared && after.phase == proxyReceiptActive
}

func (state proxyAnchorState) valid() bool {
	if state.sequence == 0 || state.path == [sha256.Size]byte{} || !validProxyReceiptSID(state.sid) {
		return false
	}
	if state.pending != (state.sequence%2 == 1) {
		return false
	}
	if state.pending {
		return validProxyAnchorTransition(state.committed, state.target) && (!state.committed.empty() || state.sequence == 1)
	}
	return state.committed.valid() && state.target.empty()
}

// Even a private caller cannot skip preparation, reset the counter or publish
// a different SID/path/committed member through the ordinary writer.
func validProxyAnchorStep(previous, next proxyAnchorState) bool {
	if !next.valid() || previous.sequence == ^uint64(0) || next.sequence != previous.sequence+1 {
		return false
	}
	if previous.sequence == 0 {
		return next.pending && next.committed.empty()
	}
	if !previous.valid() || previous.path != next.path || !bytes.Equal(previous.sid, next.sid) {
		return false
	}
	if previous.pending {
		return !next.pending && next.committed == previous.target && next.target.empty()
	}
	return next.pending && next.committed == previous.committed
}

func encodeProxyAnchor(state proxyAnchorState) ([]byte, error) {
	if !state.valid() {
		return nil, ErrProxyAnchorUnavailable
	}
	raw := append([]byte{}, proxyAnchorPurpose...)
	raw = binary.LittleEndian.AppendUint64(raw, state.sequence)
	raw = append(raw, state.path[:]...)
	raw = append(raw, byte(len(state.sid)))
	raw = append(raw, state.sid...)
	pending := byte(0)
	if state.pending {
		pending = 1
	}
	raw = append(raw, pending)
	for _, subject := range [2]proxyAnchorSubject{state.committed, state.target} {
		raw = append(raw, subject.nonce[:]...)
		raw = binary.LittleEndian.AppendUint64(raw, subject.generation)
		raw = append(raw, subject.cipher[:]...)
		raw = append(raw, subject.phase)
	}
	if len(raw) > maxProxyAnchorPlain {
		return nil, ErrProxyAnchorUnavailable
	}
	return raw, nil
}

func decodeProxyAnchor(raw []byte) (state proxyAnchorState, err error) {
	if len(raw) > maxProxyAnchorPlain || !bytes.HasPrefix(raw, []byte(proxyAnchorPurpose)) {
		return state, ErrProxyAnchorUnavailable
	}
	raw = raw[len(proxyAnchorPurpose):]
	if len(raw) < 42 {
		return state, ErrProxyAnchorUnavailable
	}
	state.sequence = binary.LittleEndian.Uint64(raw[:8])
	copy(state.path[:], raw[8:40])
	size := int(raw[40])
	if size < 8 || size > 68 || len(raw) != 42+size+2*57 || raw[41+size] > 1 {
		return state, ErrProxyAnchorUnavailable
	}
	state.sid = bytes.Clone(raw[41 : 41+size])
	state.pending = raw[41+size] == 1
	raw = raw[42+size:]
	for _, subject := range [2]*proxyAnchorSubject{&state.committed, &state.target} {
		copy(subject.nonce[:], raw[:16])
		subject.generation = binary.LittleEndian.Uint64(raw[16:24])
		copy(subject.cipher[:], raw[24:56])
		subject.phase = raw[56]
		raw = raw[57:]
	}
	if !state.valid() {
		return proxyAnchorState{}, ErrProxyAnchorUnavailable
	}
	return state, nil
}

func proxyAnchorFor(record proxyDurableReceipt, cipher []byte) proxyAnchorSubject {
	return proxyAnchorSubject{nonce: record.nonce, generation: record.generation, cipher: sha256.Sum256(cipher), phase: record.phase}
}
