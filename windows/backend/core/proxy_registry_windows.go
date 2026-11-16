//go:build windows

package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	proxyServerIndex = iota
	proxyPACIndex
	proxyEnableIndex
	maxProxyValueBytes = 16 << 10
)

var proxyValueNames = [3]string{proxyServerValue, autoConfigURLValue, proxyEnableValue}

type proxyRegistryValue struct {
	present bool
	kind    uint32
	raw     []byte
}
type proxyRegistryState [3]proxyRegistryValue

func (state proxyRegistryState) equal(other proxyRegistryState) bool {
	for index, value := range state {
		if value.present != other[index].present || value.kind != other[index].kind || !bytes.Equal(value.raw, other[index].raw) {
			return false
		}
	}
	return true
}

func proxyStringValue(text string, kind uint32) (proxyRegistryValue, error) {
	encoded, err := syscall.UTF16FromString(text)
	if err != nil || len(encoded) > maxProxyValueBytes/2 {
		return proxyRegistryValue{}, errors.New("invalid or oversized system proxy string")
	}
	raw := make([]byte, len(encoded)*2)
	for index, value := range encoded {
		binary.LittleEndian.PutUint16(raw[index*2:], value)
	}
	return proxyRegistryValue{present: true, kind: kind, raw: raw}, nil
}

func (value proxyRegistryValue) text() string {
	words := make([]uint16, len(value.raw)/2)
	for index := range words {
		words[index] = binary.LittleEndian.Uint16(value.raw[index*2:])
	}
	return syscall.UTF16ToString(words)
}

func proxyDWORDValue(value uint32) proxyRegistryValue {
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, value)
	return proxyRegistryValue{present: true, kind: registry.DWORD, raw: raw}
}

// Pure validation after a normally returned original query. Presence/missing
// is handled by that query, never inferred from a zero length or wrong type.
func decodeProxyRegistryValue(index int, kind uint32, raw []byte) (proxyRegistryValue, error) {
	if index < 0 || index >= len(proxyValueNames) || len(raw) > maxProxyValueBytes {
		return proxyRegistryValue{}, ErrProxyStateUncertain
	}
	n := len(raw)
	value := proxyRegistryValue{present: true, kind: kind, raw: raw}
	if index == proxyEnableIndex {
		if kind != registry.DWORD || n != 4 {
			return proxyRegistryValue{}, registry.ErrUnexpectedType
		}
	} else {
		if kind != registry.SZ && kind != registry.EXPAND_SZ || n%2 != 0 {
			return proxyRegistryValue{}, registry.ErrUnexpectedType
		}
		// Typed setters must reproduce exact bytes. Reject unterminated,
		// malformed UTF-16/embedded-NUL inputs rather than normalize history.
		canonical, err := proxyStringValue(value.text(), kind)
		if err != nil || !bytes.Equal(canonical.raw, value.raw) {
			return proxyRegistryValue{}, errors.New("noncanonical system proxy string")
		}
	}
	return value, nil
}

func writeProxyRegistryValue(key registry.Key, index int, value proxyRegistryValue) error {
	if index < 0 || index >= len(proxyValueNames) || len(value.raw) > maxProxyValueBytes {
		return ErrProxyStateUncertain
	}
	if !value.present && (value.kind != 0 || len(value.raw) != 0) {
		return ErrProxyStateUncertain
	}
	if value.present {
		if index == proxyEnableIndex {
			if value.kind != registry.DWORD || len(value.raw) != 4 {
				return ErrProxyStateUncertain
			}
		} else {
			if value.kind != registry.SZ && value.kind != registry.EXPAND_SZ || len(value.raw)%2 != 0 {
				return ErrProxyStateUncertain
			}
			canonical, err := proxyStringValue(value.text(), value.kind)
			if err != nil || !bytes.Equal(canonical.raw, value.raw) {
				return ErrProxyStateUncertain
			}
		}
	}
	name := proxyValueNames[index]
	if !value.present {
		// writeLocked already refused a no-op before entering this callback.
		// Missing now can mean a foreign delete; it cannot earn our write receipt.
		return key.DeleteValue(name)
	}
	if index == proxyEnableIndex {
		return key.SetDWordValue(name, binary.LittleEndian.Uint32(value.raw))
	}
	if value.kind == registry.EXPAND_SZ {
		return key.SetExpandStringValue(name, value.text())
	}
	return key.SetStringValue(name, value.text())
}

var proxyObservation *proxyRegistryKeyOwner

// ReadSystemProxySnapshot never writes settings. Unknown type/read/Close is an
// error, not disabled/empty. A failed original query-handle Close is retained;
// neither later observations nor a new writer silently replace that owner.
func ReadSystemProxySnapshot() (snapshot ProxySnapshot, result error) {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	if proxyObservation != nil {
		return snapshot, ErrProxyStateUncertain
	}
	owner := &proxyRegistryKeyOwner{}
	proxyObservation = owner // reserve before Open, even on failed/abnormal acquisition
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			result = errors.Join(result, ErrProxyStateUncertain)
		}
		if err := owner.close(); err != nil {
			result = errors.Join(result, ErrProxyStateUncertain, err)
		} else if owner.closed && proxyObservation == owner {
			proxyObservation = nil
		} else {
			result = errors.Join(result, ErrProxyStateUncertain)
		}
		if result != nil {
			snapshot = ProxySnapshot{} // failed/uncertain observation never leaves a populated display tuple
		}
	}()
	if err := owner.open(registry.QUERY_VALUE); err != nil {
		returned = true
		return snapshot, err
	}
	state, err := owner.readState()
	if err != nil {
		returned = true
		return snapshot, err
	}
	if state[proxyEnableIndex].present {
		snapshot.ProxyEnable = binary.LittleEndian.Uint32(state[proxyEnableIndex].raw)
	}
	if state[proxyServerIndex].present {
		snapshot.ProxyServer = state[proxyServerIndex].text()
	}
	if state[proxyPACIndex].present {
		snapshot.AutoConfigURL = state[proxyPACIndex].text()
	}
	returned = true
	return snapshot, nil
}
