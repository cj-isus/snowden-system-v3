package flowcontrol

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/utils"
)

type connectionFlowController struct {
	baseFlowController
	// Token publication is atomic so a reentrant constructor need not take the
	// receive-window callback's mutex. Mutation checks still hold that mutex.
	streamEpoch atomic.Pointer[connectionFlowEpoch]
}

var _ ConnectionFlowController = &connectionFlowController{}
var _ connectionFlowControllerI = &connectionFlowController{}

// NewConnectionFlowController gets a new flow controller for the connection
// It is created before we receive the peer's transport parameters, thus it starts with a sendWindow of 0.
func NewConnectionFlowController(
	receiveWindow protocol.ByteCount,
	maxReceiveWindow protocol.ByteCount,
	allowWindowIncrease func(size protocol.ByteCount) bool,
	rttStats *utils.RTTStats,
	logger utils.Logger,
) *connectionFlowController {
	c := &connectionFlowController{
		baseFlowController: baseFlowController{
			rttStats:             rttStats,
			receiveWindow:        receiveWindow,
			receiveWindowSize:    receiveWindow,
			maxReceiveWindowSize: maxReceiveWindow,
			allowWindowIncrease:  allowWindowIncrease,
			logger:               logger,
		},
	}
	c.streamEpoch.Store(&connectionFlowEpoch{})
	return c
}

// IncrementHighestReceived adds an increment to the highestReceived value
func (c *connectionFlowController) IncrementHighestReceived(increment protocol.ByteCount, now monotime.Time) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.incrementHighestReceivedLocked(increment, now)
}

func (c *connectionFlowController) incrementHighestReceivedLocked(increment protocol.ByteCount, now monotime.Time) error {
	// If this is the first frame received on this connection, start flow-control auto-tuning.
	if c.highestReceived == 0 {
		c.startNewAutoTuningEpoch(now)
	}
	c.highestReceived += increment

	if c.checkFlowControlViolation() {
		return &qerr.TransportError{
			ErrorCode:    qerr.FlowControlError,
			ErrorMessage: fmt.Sprintf("received %d bytes for the connection, allowed %d bytes", c.highestReceived, c.receiveWindow),
		}
	}
	return nil
}

func (c *connectionFlowController) AddBytesRead(n protocol.ByteCount) (hasWindowUpdate bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.addBytesReadLocked(n)
}

func (c *connectionFlowController) addBytesReadLocked(n protocol.ByteCount) bool {
	c.addBytesRead(n)
	return c.hasWindowUpdate()
}

func (c *connectionFlowController) GetWindowUpdate(now monotime.Time) protocol.ByteCount {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	oldWindowSize := c.receiveWindowSize
	offset := c.getWindowUpdate(now)
	if c.logger.Debug() && oldWindowSize < c.receiveWindowSize {
		c.logger.Debugf("Increasing receive flow control window for the connection to %d kB", c.receiveWindowSize/(1<<10))
	}
	return offset
}

// EnsureMinimumWindowSize sets a minimum window size
// it should make sure that the connection-level window is increased when a stream-level window grows
func (c *connectionFlowController) EnsureMinimumWindowSize(inc protocol.ByteCount, now monotime.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ensureMinimumWindowSizeLocked(inc, now)
}

func (c *connectionFlowController) ensureMinimumWindowSizeLocked(inc protocol.ByteCount, now monotime.Time) {
	if inc <= c.receiveWindowSize {
		return
	}
	newSize := min(inc, c.maxReceiveWindowSize)
	if delta := newSize - c.receiveWindowSize; delta > 0 && c.allowWindowIncrease(delta) {
		c.receiveWindowSize = newSize
		if c.logger.Debug() {
			c.logger.Debugf("Increasing receive flow control window for the connection to %d, in response to stream flow control window increase", newSize)
		}
	}
	c.startNewAutoTuningEpoch(now)
}

// Reset resets the flow controller. This happens when 0-RTT is rejected.
// All stream data is invalidated, it's as if we had never opened a stream and never sent any data.
// At that point, we only have sent stream data, but we didn't have the keys to open 1-RTT keys yet.
func (c *connectionFlowController) Reset() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.bytesRead > 0 || c.highestReceived > 0 || !c.epochStartTime.IsZero() {
		return errors.New("flow controller reset after reading data")
	}
	// Invalidate old stream scopes and reset counters in one critical section.
	// Allocate the nonzero token before changing any connection state.
	nextEpoch := &connectionFlowEpoch{}
	c.streamEpoch.Store(nextEpoch)
	c.bytesSent = 0
	c.lastBlockedAt = 0
	c.sendWindow = 0
	return nil
}
