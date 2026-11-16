package hysteria2

import (
	"errors"
	"os"
)

type udpShutdownStage uint8

const (
	udpShutdownReaders udpShutdownStage = iota + 1
	udpShutdownScope
	udpShutdownCancel
	udpShutdownDeadline
	udpShutdownFragments
	udpShutdownQueue
	udpShutdownReturned
)

func (c *udpPacketConn) markShutdownUncertain() {
	if c.defragger != nil && c.defragger.budget != nil {
		c.defragger.budget.markUncertain()
	}
}

// Public and carrier close share one original shutdown and one destroy result.
// An uncertain shutdown keeps its parent registration instead of destroying it.
func (c *udpPacketConn) Close() error {
	return c.closeAndDestroy(os.ErrClosed)
}

func (c *udpPacketConn) closeAndDestroy(cause error) error {
	if err := c.closeWithError(cause); err != nil {
		return err
	}
	c.closeOnce.Do(func() {
		returned := false
		defer func() {
			if !returned {
				c.destroyErr = errClientUncertain
				c.markShutdownUncertain()
			}
		}()
		if c.onDestroy != nil {
			c.onDestroy()
		}
		returned = true
	})
	return c.destroyErr
}

// No fresh scope close / drain is started after an unknown original result.
// sync.Once also joins concurrent callers and publishes the immutable error.
func (c *udpPacketConn) closeWithError(cause error) error {
	c.shutdownOnce.Do(func() {
		returned := false
		defer func() {
			if !returned {
				c.shutdownErr = errors.Join(c.shutdownErr, errClientUncertain)
			}
			if c.shutdownErr != nil {
				// Queue mutex has already unwound. Notify the parent without a
				// budget/RX lock and without refunding original retained storage.
				c.markShutdownUncertain()
			}
		}()
		c.shutdownStage = udpShutdownReaders
		c.shutdownErr = c.sealReadCalls()
		c.shutdownStage = udpShutdownScope
		if c.sendScope == nil {
			c.shutdownErr = errors.Join(c.shutdownErr, errClientUncertain)
		} else {
			c.shutdownErr = errors.Join(c.shutdownErr, c.sendScope.Close())
		}
		if c.shutdownErr != nil {
			c.markShutdownUncertain() // start parent abort before independent RX cleanup
		}
		if cause == nil {
			cause = os.ErrClosed
		}
		c.shutdownStage = udpShutdownCancel
		c.cancel(cause)
		c.shutdownStage = udpShutdownDeadline
		c.shutdownErr = errors.Join(c.shutdownErr, c.stopReadDeadline())
		c.receiveAccess.Lock()
		defer c.receiveAccess.Unlock()
		c.shutdownStage = udpShutdownFragments
		if c.defragger == nil || c.receiveFailed || c.receiveInput != nil || c.receiveOutput != nil {
			c.shutdownErr = errors.Join(c.shutdownErr, errClientUncertain)
			returned = true
			return
		}
		if err := c.defragger.close(); err != nil {
			c.shutdownErr = errors.Join(c.shutdownErr, err)
			returned = true
			return
		}
		c.shutdownStage = udpShutdownQueue
		for {
			select {
			case message := <-c.data:
				c.shutdownCurrent = message // original stays held through pool callbacks
				message.releaseMessage()
				c.shutdownCurrent = nil
			default:
				if c.defragger.budget == nil || c.defragger.budget.uncertain() {
					c.shutdownErr = errors.Join(c.shutdownErr, errClientUncertain)
				}
				c.shutdownStage = udpShutdownReturned
				returned = true
				return
			}
		}
	})
	return c.shutdownErr
}
