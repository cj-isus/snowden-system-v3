//go:build windows

package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

const (
	proxyStagePurpose   = "snowden/proxy/staged-receipt/v1\x00"
	maxProxyStagePlain  = maxProxyReceiptCipher + (16 << 10)
	maxProxyStageCipher = maxProxyStagePlain + (16 << 10)
)

type proxyStageFile struct {
	present           bool
	volume, high, low uint32
	size, written     uint64
}

func (file proxyStageFile) valid() bool {
	if !file.present {
		return file == (proxyStageFile{})
	}
	return (file.high != 0 || file.low != 0) && file.size > uint64(len(proxyCipherPurpose)) && file.size <= maxProxyReceiptCipher+uint64(len(proxyCipherPurpose))
}

// Protected intent, not an arbitrary backup. Parent counter/subject/identity
// binds the target to one recorded journal transition, even before preparation.
type proxyStageIntent struct {
	parentSequence uint64
	path           [sha256.Size]byte
	sid            []byte
	parent         proxyAnchorSubject
	file           proxyStageFile
	target         proxyAnchorSubject
	raw            []byte // exact already-encrypted canonical receipt, never plaintext PAC
}

func (intent proxyStageIntent) valid() bool {
	if intent.parentSequence%2 != 0 || intent.parentSequence > ^uint64(0)-2 || intent.path == [sha256.Size]byte{} ||
		!validProxyReceiptSID(intent.sid) || !intent.file.valid() || !validProxyAnchorTransition(intent.parent, intent.target) ||
		len(intent.raw) <= len(proxyCipherPurpose) || len(intent.raw) > maxProxyReceiptCipher+len(proxyCipherPurpose) ||
		!bytes.HasPrefix(intent.raw, []byte(proxyCipherPurpose)) || sha256.Sum256(intent.raw) != intent.target.cipher {
		return false
	}
	if intent.parentSequence == 0 {
		return intent.parent.empty() && !intent.file.present
	}
	return intent.parent.valid() && intent.file.present
}

func appendProxyStageSubject(raw []byte, subject proxyAnchorSubject) []byte {
	raw = append(raw, subject.nonce[:]...)
	raw = binary.LittleEndian.AppendUint64(raw, subject.generation)
	raw = append(raw, subject.cipher[:]...)
	return append(raw, subject.phase)
}

func encodeProxyStage(intent proxyStageIntent) ([]byte, error) {
	if !intent.valid() {
		return nil, ErrProxyStageUnavailable
	}
	raw := append([]byte{}, proxyStagePurpose...)
	raw = binary.LittleEndian.AppendUint64(raw, intent.parentSequence)
	raw = append(raw, intent.path[:]...)
	raw = append(raw, byte(len(intent.sid)))
	raw = append(raw, intent.sid...)
	raw = appendProxyStageSubject(raw, intent.parent)
	present := byte(0)
	if intent.file.present {
		present = 1
	}
	raw = append(raw, present)
	raw = binary.LittleEndian.AppendUint32(raw, intent.file.volume)
	raw = binary.LittleEndian.AppendUint32(raw, intent.file.high)
	raw = binary.LittleEndian.AppendUint32(raw, intent.file.low)
	raw = binary.LittleEndian.AppendUint64(raw, intent.file.size)
	raw = binary.LittleEndian.AppendUint64(raw, intent.file.written)
	raw = appendProxyStageSubject(raw, intent.target)
	raw = binary.LittleEndian.AppendUint32(raw, uint32(len(intent.raw)))
	raw = append(raw, intent.raw...)
	if len(raw) > maxProxyStagePlain {
		return nil, ErrProxyStageUnavailable
	}
	return raw, nil
}

func decodeProxyStage(raw []byte) (intent proxyStageIntent, err error) {
	if len(raw) > maxProxyStagePlain || !bytes.HasPrefix(raw, []byte(proxyStagePurpose)) {
		return intent, ErrProxyStageUnavailable
	}
	raw = raw[len(proxyStagePurpose):]
	take := func(n int) []byte {
		if n < 0 || n > len(raw) {
			err = ErrProxyStageUnavailable
			return nil
		}
		part := raw[:n]
		raw = raw[n:]
		return part
	}
	header := take(41)
	if err != nil {
		return intent, err
	}
	intent.parentSequence = binary.LittleEndian.Uint64(header[:8])
	copy(intent.path[:], header[8:40])
	size := int(header[40])
	if size < 8 || size > 68 {
		return intent, ErrProxyStageUnavailable
	}
	intent.sid = bytes.Clone(take(size))
	subject := func() proxyAnchorSubject {
		part := take(57)
		if err != nil {
			return proxyAnchorSubject{}
		}
		var value proxyAnchorSubject
		copy(value.nonce[:], part[:16])
		value.generation = binary.LittleEndian.Uint64(part[16:24])
		copy(value.cipher[:], part[24:56])
		value.phase = part[56]
		return value
	}
	intent.parent = subject()
	file := take(29)
	if err != nil {
		return intent, err
	}
	if file[0] > 1 {
		return intent, ErrProxyStageUnavailable
	}
	intent.file = proxyStageFile{present: file[0] == 1, volume: binary.LittleEndian.Uint32(file[1:5]), high: binary.LittleEndian.Uint32(file[5:9]),
		low: binary.LittleEndian.Uint32(file[9:13]), size: binary.LittleEndian.Uint64(file[13:21]), written: binary.LittleEndian.Uint64(file[21:29])}
	intent.target = subject()
	length := take(4)
	if err != nil {
		return intent, err
	}
	n := binary.LittleEndian.Uint32(length)
	if n > maxProxyReceiptCipher+uint32(len(proxyCipherPurpose)) {
		return intent, ErrProxyStageUnavailable
	}
	intent.raw = bytes.Clone(take(int(n)))
	if err != nil || len(raw) != 0 || !intent.valid() {
		return proxyStageIntent{}, ErrProxyStageUnavailable
	}
	return intent, nil
}
