package quic

// Caller owns the corresponding child metadata mutex. Native close joined its
// original control actor; this check does not wait for or invoke that actor.
func (c *streamMapControl) retirementReady() bool {
	return !c.running && !c.active && !c.failed && !c.pending && c.frame == nil && c.callback == nil
}

func (m *outgoingStreamsMap[T]) unusedEmpty() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return !m.closeStarted && !m.closeReturned && m.closeFailure == nil && !m.uncertain &&
		!m.metadataRetiring && !m.metadataRetired && !m.retiringActive &&
		!m.creation.active && m.creation.construction.stage == constructionIdle &&
		!m.updating.active && len(m.updating.streams) == 0 && m.control.retirementReady() &&
		m.rowCount == 0 && m.rowBytes == 0 && len(m.streams) == 0 && len(m.openQueue) == 0
}

func (m *incomingStreamsMap[T]) unusedEmpty() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return !m.closeStarted && !m.closeReturned && m.closeFailure == nil && !m.uncertain &&
		!m.metadataRetiring && !m.metadataRetired && !m.retiringActive &&
		!m.creation.active && m.creation.construction.stage == constructionIdle &&
		m.control.retirementReady() && m.rowCount == 0 && m.rowBytes == 0 && len(m.streams) == 0
}

func (m *outgoingStreamsMap[T]) metadataReadyLocked() error {
	if m.metadataRetiring || m.uncertain {
		return errOutgoingMapUncertain
	}
	if m.metadataRetired {
		return nil
	}
	if !m.closeStarted || !m.closeReturned ||
		m.closeFailure != nil || m.retiringActive || m.creation.active ||
		m.creation.construction.stage != constructionIdle || m.updating.active ||
		len(m.updating.streams) != 0 || !m.control.retirementReady() || len(m.openQueue) != 0 ||
		m.rowCount < 0 || m.rowCount > maxOutgoingStreamRows || m.rowCount != len(m.streams) ||
		m.rowBytes != m.rowCount*outgoingStreamRowCharge {
		return errOutgoingMapUncertain
	}
	return nil
}

func (m *incomingStreamsMap[T]) metadataReadyLocked() error {
	if m.metadataRetiring || m.uncertain {
		return errIncomingMapUncertain
	}
	if m.metadataRetired {
		return nil
	}
	if !m.closeStarted || !m.closeReturned ||
		m.closeFailure != nil || m.retiringActive || m.creation.active ||
		m.creation.construction.stage != constructionIdle || !m.control.retirementReady() ||
		m.rowCount < 0 || m.rowCount > maxIncomingStreamRows || m.rowCount != len(m.streams) ||
		m.rowBytes != m.rowCount*incomingStreamRowCharge {
		return errIncomingMapUncertain
	}
	return nil
}

func (m *outgoingStreamsMap[T]) metadataReady() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.metadataReadyLocked()
}

func (m *incomingStreamsMap[T]) metadataReady() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.metadataReadyLocked()
}

// Only the external parent retirement owner calls these after the original
// loop/map/stream/data receipts. No native stream or pool callback is invoked.
func (m *outgoingStreamsMap[T]) retireMetadata() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.metadataReadyLocked(); err != nil {
		return err
	}
	if m.metadataRetired {
		return nil
	}
	m.metadataRetiring = true
	returned := false
	defer func() {
		if !returned {
			m.uncertain = true // preserve the original incomplete retirement; no retry
		}
	}()
	clear(m.streams)
	m.streams, m.openQueue = nil, nil
	m.rowCount, m.rowBytes = 0, 0 // logical metadata credits, not native/RSS release
	m.creation = outgoingStreamCreation[T]{}
	m.updating = outgoingStreamUpdate[T]{}
	m.control = streamMapControl{}
	m.retiring = *new(T)
	m.newStream, m.queueStreamIDBlocked = nil, nil
	m.metadataRetiring, m.metadataRetired = false, true
	returned = true
	return nil
}

func (m *incomingStreamsMap[T]) retireMetadata() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err := m.metadataReadyLocked(); err != nil {
		return err
	}
	if m.metadataRetired {
		return nil
	}
	m.metadataRetiring = true
	returned := false
	defer func() {
		if !returned {
			m.uncertain = true
		}
	}()
	clear(m.streams)
	m.streams, m.newStreamChan = nil, nil // native close already sealed this channel
	m.rowCount, m.rowBytes = 0, 0
	m.creation = incomingStreamCreation[T]{}
	m.control = streamMapControl{}
	m.retiring = *new(T)
	m.newStream, m.queueMaxStreamID = nil, nil
	m.metadataRetiring, m.metadataRetired = false, true
	returned = true
	return nil
}
