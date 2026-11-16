//go:build windows

package core

import (
	"bytes"
	"encoding/binary"
)

const (
	proxyReceiptPurpose        = "snowden/proxy/original-write/v1\x00"
	proxyCipherPurpose         = "snowden/proxy/dpapi/v1\x00"
	maxProxyReceiptPlain       = 192 << 10
	maxProxyReceiptCipher      = 256 << 10
	proxyReceiptActive    byte = 1
	proxyReceiptPrepared  byte = 2
	proxyReceiptClosed    byte = 3
	proxyReceiptForeign   byte = 4
	proxyReceiptNoIndex   byte = 255
)

// Private disk data only, never a display DTO or authority to restore settings.
// Every state preserves exact presence/type/bytes, including an absent value.
type proxyDurableReceipt struct {
	phase, index, written     byte
	generation                uint64
	pid                       uint32
	creation                  uint64
	nonce                     [16]byte
	sid                       []byte
	original, current, target proxyRegistryState
}

func validProxyReceiptSID(sid []byte) bool {
	return len(sid) >= 8 && len(sid) <= 68 && sid[0] == 1 && sid[1] <= 15 && len(sid) == 8+4*int(sid[1])
}

func (record proxyDurableReceipt) valid() bool {
	if record.generation == 0 || record.pid == 0 || record.creation == 0 || record.nonce == [16]byte{} ||
		!validProxyReceiptSID(record.sid) || record.written & ^byte(7) != 0 {
		return false
	}
	for _, state := range [3]proxyRegistryState{record.original, record.current, record.target} {
		for index, value := range state {
			if !value.present {
				if value.kind != 0 || len(value.raw) != 0 {
					return false
				}
			} else if _, err := decodeProxyRegistryValue(index, value.kind, value.raw); err != nil {
				return false
			}
		}
	}
	for index := range record.current {
		if record.written&(1<<index) == 0 {
			expected := record.original
			expected[index] = record.current[index]
			if !expected.equal(record.original) {
				return false // an unconfirmed write cannot become the confirmed current tuple
			}
		}
	}
	switch record.phase {
	case proxyReceiptPrepared:
		if record.index >= 3 {
			return false
		}
		expected := record.current
		expected[record.index] = record.target[record.index]
		return expected.equal(record.target) && !record.current.equal(record.target)
	case proxyReceiptActive, proxyReceiptClosed, proxyReceiptForeign:
		return record.index == proxyReceiptNoIndex && record.target.equal(record.current) &&
			(record.phase != proxyReceiptClosed || record.current.equal(record.original))
	default:
		return false
	}
}

func encodeProxyReceipt(record proxyDurableReceipt) ([]byte, error) {
	if !record.valid() {
		return nil, ErrProxyReceiptUnavailable
	}
	raw := make([]byte, 0, 1024)
	raw = append(raw, proxyReceiptPurpose...)
	raw = append(raw, record.phase, record.index, record.written)
	raw = binary.LittleEndian.AppendUint64(raw, record.generation)
	raw = binary.LittleEndian.AppendUint32(raw, record.pid)
	raw = binary.LittleEndian.AppendUint64(raw, record.creation)
	raw = append(raw, record.nonce[:]...)
	raw = append(raw, byte(len(record.sid)))
	raw = append(raw, record.sid...)
	for _, state := range [3]proxyRegistryState{record.original, record.current, record.target} {
		for _, value := range state {
			present := byte(0)
			if value.present {
				present = 1
			}
			raw = append(raw, present)
			raw = binary.LittleEndian.AppendUint32(raw, value.kind)
			raw = binary.LittleEndian.AppendUint16(raw, uint16(len(value.raw)))
			raw = append(raw, value.raw...)
		}
	}
	if len(raw) > maxProxyReceiptPlain {
		return nil, ErrProxyReceiptUnavailable
	}
	return raw, nil
}

// Checked cursor; no claimed length can allocate before its fixed bound and
// remaining bytes have been checked. Decoded values own copies of plaintext.
func decodeProxyReceipt(raw []byte) (record proxyDurableReceipt, err error) {
	if len(raw) > maxProxyReceiptPlain || !bytes.HasPrefix(raw, []byte(proxyReceiptPurpose)) {
		return record, ErrProxyReceiptUnavailable
	}
	raw = raw[len(proxyReceiptPurpose):]
	take := func(n int) []byte {
		if n < 0 || len(raw) < n {
			err = ErrProxyReceiptUnavailable
			return nil
		}
		part := raw[:n]
		raw = raw[n:]
		return part
	}
	header := take(40) // phase/index/mask, generation, PID, creation, nonce, SID size
	if err != nil {
		return record, err
	}
	record.phase, record.index, record.written = header[0], header[1], header[2]
	record.generation = binary.LittleEndian.Uint64(header[3:11])
	record.pid = binary.LittleEndian.Uint32(header[11:15])
	record.creation = binary.LittleEndian.Uint64(header[15:23])
	copy(record.nonce[:], header[23:39])
	sidSize := int(header[39])
	if sidSize < 8 || sidSize > 68 {
		return record, ErrProxyReceiptUnavailable
	}
	record.sid = bytes.Clone(take(sidSize))
	if err != nil {
		return record, err
	}
	states := [3]*proxyRegistryState{&record.original, &record.current, &record.target}
	for _, state := range states {
		for index := range state {
			header := take(7)
			if err != nil {
				return record, err
			}
			if header[0] > 1 {
				return record, ErrProxyReceiptUnavailable
			}
			size := int(binary.LittleEndian.Uint16(header[5:7]))
			if size > maxProxyValueBytes {
				return record, ErrProxyReceiptUnavailable
			}
			value := proxyRegistryValue{present: header[0] == 1, kind: binary.LittleEndian.Uint32(header[1:5]), raw: bytes.Clone(take(size))}
			if err != nil {
				return record, err
			}
			state[index] = value
		}
	}
	if len(raw) != 0 || !record.valid() {
		return record, ErrProxyReceiptUnavailable
	}
	return record, nil
}
