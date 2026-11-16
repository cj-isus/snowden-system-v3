package http3

import (
	"errors"

	"github.com/sagernet/quic-go"
)

const maxTrackedStreams = 64

var (
	errStreamTrackingStopped   = errors.New("http3: stream tracking admission closed")
	errStreamTrackingCapacity  = errors.New("http3: tracked stream capacity exceeded")
	errStreamTrackingDuplicate = errors.New("http3: stream is already tracked")
	errStreamTrackingUncertain = errors.New("http3: stream tracking did not settle normally")
)

// streamAdmissionAllowedLocked serializes all original qloggerWG additions
// with the seal preceding Wait. Closing stops intake; it never clears owners.
func (c *rawConn) streamAdmissionAllowedLocked() bool {
	if c.streamAdmissionClosed || c.streamClosing == nil {
		return false
	}
	select {
	case <-c.streamClosing:
		c.streamAdmissionClosed = true
		return false
	default:
		return true
	}
}

func (c *rawConn) beginQloggerTask() bool {
	c.streamMx.Lock()
	defer c.streamMx.Unlock()
	if !c.streamAdmissionAllowedLocked() {
		return false
	}
	c.qloggerWG.Add(1)
	return true
}

func (c *rawConn) sealStreamAdmission() {
	c.streamMx.Lock()
	c.streamAdmissionClosed = true
	c.streamMx.Unlock()
}

func (c *rawConn) failStreamRegistryLocked() {
	c.streamAdmissionClosed = true
	if c.streamRegistryFailure == nil {
		c.streamRegistryFailure = errStreamTrackingUncertain
	}
}

func (c *rawConn) failStreamRegistry() {
	c.streamMx.Lock()
	c.failStreamRegistryLocked()
	c.streamMx.Unlock()
}

// Called by the external shutdown join after the client task owner drains.
// Join the actual original native stream owner before terminating residual
// local metadata. Closing alone cannot remove entries or synthesize native work.
func (c *rawConn) waitForStreamRegistry() error {
	select {
	case <-c.streamClosing:
	default:
		return errStreamTrackingUncertain // do not seal an active / unconfigured owner
	}
	return c.retireStreamRegistry()
}

func (c *rawConn) TrackStream(str *quic.Stream) (*stateTrackingStream, error) {
	// This permit belongs to the one original send-context callback. The map
	// has an independent limit: callback return does not remove its wrapper.
	reserved := c.clientTasks != nil
	if reserved && !c.clientTasks.begin() {
		return nil, c.clientTasks.admissionError()
	}
	watcherEntered, returned := false, false
	defer func() {
		if !returned {
			c.failStreamRegistry() // retain any published original on uncertainty
		}
		if reserved && !watcherEntered {
			c.clientTasks.finish(true) // callback installation was never entered
		}
	}()
	hstr, err := c.registerStream(str)
	if err != nil {
		returned = true
		return nil, err
	}
	// Map and WG own the complete wrapper before AfterFunc can start. Unknown
	// installation keeps both that owner and its original callback permit.
	watcherEntered = true
	hstr.watchSendContext(c.clientTasks)
	returned = true
	return hstr, nil
}

func (c *rawConn) registerStream(str *quic.Stream) (*stateTrackingStream, error) {
	id := str.StreamID()
	c.streamMx.Lock()
	defer c.streamMx.Unlock()
	if !c.streamAdmissionAllowedLocked() || c.streams == nil {
		return nil, errStreamTrackingStopped
	}
	if _, exists := c.streams[id]; exists {
		return nil, errStreamTrackingDuplicate
	}
	if len(c.streams) >= maxTrackedStreams {
		return nil, errStreamTrackingCapacity
	}
	// Count admission precedes wrapper / channel allocation. This constructor
	// only initializes local fields; callback registration happens after unlock.
	hstr := newUnwatchedStateTrackingStream(str, c, func(b []byte) error {
		return c.sendDatagram(id, b)
	})
	c.streams[id] = hstr
	c.qloggerWG.Add(1)
	return hstr, nil
}
