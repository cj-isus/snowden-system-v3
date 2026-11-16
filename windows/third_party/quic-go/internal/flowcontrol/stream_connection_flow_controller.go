package flowcontrol

import (
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
)

// Nonzero object identity. A live old scope retains its token, preventing ABA
// when stream IDs are reused after rejected 0-RTT. This is not an integer epoch.
type connectionFlowEpoch struct{ _ byte }

// Stored by value inside the existing stream flow controller. The scope never
// changes owner or epoch, and grants no connection reset / parameter authority.
type streamConnectionFlowController struct {
	owner *connectionFlowController
	epoch *connectionFlowEpoch
}

func (c *connectionFlowController) newStreamScope() streamConnectionFlowController {
	// Capture grants no counter access. In particular it must not relock the
	// connection mutex if AllowConnectionWindowIncrease opens a stream.
	epoch := c.streamEpoch.Load()
	if epoch == nil {
		panic("quic: connection flow epoch unavailable")
	}
	return streamConnectionFlowController{owner: c, epoch: epoch}
}

func (s streamConnectionFlowController) AddBytesSent(n protocol.ByteCount) {
	c := s.owner
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.streamEpoch.Load() != s.epoch {
		return
	}
	c.baseFlowController.AddBytesSent(n)
}

func (s streamConnectionFlowController) SendWindowSize() protocol.ByteCount {
	c := s.owner
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.streamEpoch.Load() != s.epoch {
		return 0
	}
	return c.baseFlowController.SendWindowSize()
}

func (s streamConnectionFlowController) AddBytesRead(n protocol.ByteCount) bool {
	c := s.owner
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.streamEpoch.Load() != s.epoch {
		return false
	}
	return c.addBytesReadLocked(n)
}

func (s streamConnectionFlowController) IncrementHighestReceived(n protocol.ByteCount, now monotime.Time) error {
	c := s.owner
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.streamEpoch.Load() != s.epoch {
		return nil // a benign old callback cannot fail the current connection
	}
	return c.incrementHighestReceivedLocked(n, now)
}

func (s streamConnectionFlowController) EnsureMinimumWindowSize(n protocol.ByteCount, now monotime.Time) {
	c := s.owner
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.streamEpoch.Load() != s.epoch {
		return
	}
	c.ensureMinimumWindowSizeLocked(n, now)
}

// The connection loop previously inherited these unlocked base methods. Its
// accesses now share the same mutex as stream scopes and successful Reset.
func (c *connectionFlowController) AddBytesSent(n protocol.ByteCount) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.baseFlowController.AddBytesSent(n)
}

func (c *connectionFlowController) SendWindowSize() protocol.ByteCount {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.baseFlowController.SendWindowSize()
}

func (c *connectionFlowController) UpdateSendWindow(n protocol.ByteCount) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.baseFlowController.UpdateSendWindow(n)
}

func (c *connectionFlowController) IsNewlyBlocked() (bool, protocol.ByteCount) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// The base method calls the base SendWindowSize directly, without relocking.
	return c.baseFlowController.IsNewlyBlocked()
}
