//go:build windows

package core

import (
	"errors"
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

// Only the original key owner can read. Every native input/output and each of
// the fixed two observations is retained before its callback. These private
// values may contain PAC credentials; never log, serialize or persist them.
type proxyRegistryRead struct {
	owner    *proxyRegistryKeyOwner
	key      windows.Handle // borrowed original input, never independent Close authority
	queries  [2][3]proxyRegistryQuery
	states   [2]proxyRegistryState
	started  bool
	returned bool
	result   error
}

type proxyRegistryQuery struct {
	name              []uint16
	buffer            []byte
	size, kind        uint32
	started, returned bool
	err               error
}

func (owner *proxyRegistryKeyOwner) readState() (proxyRegistryState, error) {
	if !owner.ready() {
		return proxyRegistryState{}, ErrProxyStateUncertain
	}
	read := &proxyRegistryRead{owner: owner, key: owner.handle}
	owner.read = read // before name/buffer preparation or any native query
	return read.run()
}

func (read *proxyRegistryRead) run() (state proxyRegistryState, result error) {
	if read == nil || read.started || read.owner == nil || read.owner.read != read {
		return proxyRegistryState{}, ErrProxyStateUncertain // never revive an unknown original frame
	}
	read.started = true
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			state, result = proxyRegistryState{}, ErrProxyStateUncertain
		}
		read.returned, read.result = returned, result
	}()
	for pass := range read.queries {
		for index := range read.queries[pass] {
			value, err := read.query(&read.queries[pass][index], index)
			if err != nil {
				returned = true // ordinary query/shape failure, not unknown callback completion
				return proxyRegistryState{}, err
			}
			read.states[pass][index] = value
		}
	}
	if !read.states[0].equal(read.states[1]) {
		returned = true
		return proxyRegistryState{}, ErrProxyOwnershipConflict
	}
	returned = true
	return read.states[1], nil // bounded double observation, not atomic registry CAS
}

func (read *proxyRegistryRead) query(query *proxyRegistryQuery, index int) (proxyRegistryValue, error) {
	if read.owner == nil || read.owner.read != read || read.owner.handle != read.key || read.owner.closeStarted || read.owner.closed ||
		!read.started || read.returned || index < 0 || index >= len(proxyValueNames) || query == nil || query.started {
		return proxyRegistryValue{}, ErrProxyStateUncertain
	}
	var err error
	query.name, err = windows.UTF16FromString(proxyValueNames[index])
	if err != nil {
		return proxyRegistryValue{}, ErrProxyStateUncertain
	}
	query.buffer = make([]byte, maxProxyValueBytes)
	query.size = uint32(len(query.buffer))
	query.started = true
	query.err = windows.RegQueryValueEx(read.key, &query.name[0], nil, &query.kind, &query.buffer[0], &query.size)
	runtime.KeepAlive(read)
	query.returned = true
	if errors.Is(query.err, windows.ERROR_FILE_NOT_FOUND) {
		return proxyRegistryValue{}, nil // actual missing value, not defaulted data
	}
	if query.err != nil {
		return proxyRegistryValue{}, fmt.Errorf("read system proxy %s: %w", proxyValueNames[index], query.err)
	}
	if query.size > uint32(len(query.buffer)) {
		return proxyRegistryValue{}, ErrProxyStateUncertain
	}
	return decodeProxyRegistryValue(index, query.kind, query.buffer[:query.size:query.size])
}
