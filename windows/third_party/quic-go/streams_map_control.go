package quic

import (
	"sync"

	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

// Parent metadata mutex protects every field. One original actor holds its
// separate gate; only a monotone pending scalar can accumulate behind it.
type streamMapControl struct {
	running      bool
	active       bool
	failed       bool
	pending      bool
	pendingLimit protocol.StreamNum
	currentLimit protocol.StreamNum
	frame        wire.Frame
	callback     any // exact original typed callback, retained on abnormal return
}

func (c *streamMapControl) requestLocked(limit protocol.StreamNum) {
	if c.failed {
		return
	}
	if !c.pending || limit > c.pendingLimit {
		c.pendingLimit = limit
	}
	c.pending = true
}

func (c *streamMapControl) beginLocked(gate *sync.Mutex, uncertain error) bool {
	if c.running || !c.pending {
		return false
	}
	// Nonwaiting reservation under metadata. A prior actor releases its gate
	// and running flag under this same mutex, so pending handoff has no gap.
	if c.failed || !gate.TryLock() {
		c.failed = true
		panic(uncertain)
	}
	c.running = true
	return true
}

func (c *streamMapControl) takeLocked() protocol.StreamNum {
	c.currentLimit, c.pendingLimit = c.pendingLimit, 0
	c.pending, c.active = false, true // before wire frame or callback allocation
	return c.currentLimit
}

func (c *streamMapControl) acknowledgeLocked() {
	c.active, c.currentLimit = false, 0
	c.frame, c.callback = nil, nil
}

func (m *outgoingStreamsMap[T]) beginBlockedControl() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.stateErrorLocked() != nil {
		return false
	}
	return m.control.beginLocked(&m.controlMutex, errOutgoingMapUncertain)
}

func (m *outgoingStreamsMap[T]) drainBlockedControl() {
	if !m.beginBlockedControl() {
		return
	}
	returned := false
	defer func() {
		if !returned {
			m.mutex.Lock()
			m.control.failed, m.uncertain = true, true
			m.sealLocked(errOutgoingMapUncertain)
			m.control.running = false
			m.controlMutex.Unlock()
			m.mutex.Unlock()
		}
	}()
	for {
		m.mutex.Lock()
		if m.closeStarted || !m.control.pending {
			if m.closeStarted {
				m.control.pending, m.control.pendingLimit = false, 0
			}
			m.control.running, returned = false, true
			m.controlMutex.Unlock()
			m.mutex.Unlock()
			return
		}
		limit := m.control.takeLocked()
		callback := m.queueStreamIDBlocked
		m.control.callback = callback
		m.mutex.Unlock()
		frame := &wire.StreamsBlockedFrame{Type: m.streamType, StreamLimit: limit}
		m.mutex.Lock()
		m.control.frame = frame
		m.mutex.Unlock()
		callback(frame) // neither metadata nor factory/update gate is held
		m.mutex.Lock()
		m.control.acknowledgeLocked()
		m.mutex.Unlock()
	}
}

func (m *incomingStreamsMap[T]) beginMaxStreamsControl() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.stateErrorLocked() != nil {
		return false
	}
	return m.control.beginLocked(&m.controlMutex, errIncomingMapUncertain)
}

func (m *incomingStreamsMap[T]) drainMaxStreamsControl() {
	if !m.beginMaxStreamsControl() {
		return
	}
	returned := false
	defer func() {
		if !returned {
			m.mutex.Lock()
			m.control.failed, m.uncertain = true, true
			m.sealLocked(errIncomingMapUncertain)
			m.control.running = false
			m.controlMutex.Unlock()
			m.mutex.Unlock()
		}
	}()
	for {
		m.mutex.Lock()
		if m.closeStarted || !m.control.pending {
			if m.closeStarted {
				m.control.pending, m.control.pendingLimit = false, 0
			}
			m.control.running, returned = false, true
			m.controlMutex.Unlock()
			m.mutex.Unlock()
			return
		}
		limit := m.control.takeLocked()
		callback := m.queueMaxStreamID
		m.control.callback = callback
		m.mutex.Unlock()
		frame := &wire.MaxStreamsFrame{Type: m.streamType, MaxStreamNum: limit}
		m.mutex.Lock()
		m.control.frame = frame
		m.mutex.Unlock()
		callback(frame)
		m.mutex.Lock()
		m.control.acknowledgeLocked()
		m.mutex.Unlock()
	}
}

// Native close has sealed parent admission before either join. Join the actual
// original actor with no metadata/factory lock held, never from its own callback.
func (m *outgoingStreamsMap[T]) joinControl() bool {
	m.controlMutex.Lock()
	defer m.controlMutex.Unlock()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.control.failed || m.control.running || m.control.active {
		return false
	}
	m.control.pending, m.control.pendingLimit = false, 0
	return true
}

func (m *incomingStreamsMap[T]) joinControl() bool {
	m.controlMutex.Lock()
	defer m.controlMutex.Unlock()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.control.failed || m.control.running || m.control.active {
		return false
	}
	m.control.pending, m.control.pendingLimit = false, 0
	return true
}
