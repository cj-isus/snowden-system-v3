package hysteria2

import (
	"errors"
	"net"
	"time"
)

const (
	maxUDPControlCalls        = 8
	maxUDPControlSessionCalls = 2
)

var errUDPControlCapacity = errors.New("HY2 UDP control capacity exceeded")

type udpControlStage uint8

const (
	udpControlWaiting udpControlStage = iota + 1
	udpControlChecking
	udpControlWriteDeadline
	udpControlReadDeadline
	udpControlReturned
)

// A separate fixed lane cannot be exhausted by waiting packet readers. The
// original slot roots its session/deadline/stage through native or timer work.
type udpControlCall struct {
	active   bool
	owner    *udpPacketConn
	deadline time.Time
	read     bool
	write    bool
	stage    udpControlStage
}

func (o *udpControlCall) idle() bool {
	return !o.active && o.owner == nil && o.deadline == (time.Time{}) && !o.read && !o.write && o.stage == 0
}

func (b *udpReceiveBudget) controlsIdleLocked() bool {
	for index := range b.controls {
		if b.controls[index].active || !b.controls[index].idle() {
			return false
		}
	}
	return true
}

func (c *udpPacketConn) beginUDPControl(t time.Time, read, write bool) (*udpControlCall, error) {
	if c.defragger == nil || c.defragger.budget == nil {
		return nil, errClientUncertain
	}
	b := c.defragger.budget
	b.mu.Lock()
	uncertain, controlUnknown := false, false
	defer func() {
		if uncertain {
			if controlUnknown {
				c.controlFailed = true
			}
			b.poisoned = true
			b.settleLocked()
		}
		b.mu.Unlock()
		if uncertain {
			b.markUncertain()
		}
	}()
	if b.poisoned || c.controlFailed {
		uncertain = true
		return nil, errClientUncertain
	}
	if c.controlCalls < 0 || c.controlCalls > maxUDPControlSessionCalls || b.controlCalls < 0 || b.controlCalls > maxUDPControlCalls {
		uncertain, controlUnknown = true, true
		return nil, errClientUncertain
	}
	if b.sealed || c.readSealed {
		return nil, net.ErrClosed
	}
	if c.controlCalls >= maxUDPControlSessionCalls || b.controlCalls >= maxUDPControlCalls {
		return nil, errUDPControlCapacity
	}
	for index := range b.controls {
		original := &b.controls[index]
		if original.active {
			continue
		}
		if !original.idle() {
			uncertain, controlUnknown = true, true
			return nil, errClientUncertain
		}
		*original = udpControlCall{active: true, owner: c, deadline: t, read: read, write: write, stage: udpControlWaiting}
		c.controlCalls++
		b.controlCalls++
		return original, nil
	}
	uncertain, controlUnknown = true, true
	return nil, errClientUncertain
}

// Used before releasing control on abnormal operation return. A previously
// admitted waiter must observe failure before it can reset the same timer.
func (c *udpPacketConn) poisonUDPControl() {
	b := c.defragger.budget
	b.mu.Lock()
	c.controlFailed = true
	b.poisoned = true
	b.settleLocked()
	b.mu.Unlock()
}

func (c *udpPacketConn) endUDPControl(original *udpControlCall, returned bool) error {
	b := c.defragger.budget
	b.mu.Lock()
	owned := false
	for index := range b.controls {
		if original == &b.controls[index] {
			owned = true
			break
		}
	}
	unknown := !returned || !owned || !original.active || original.owner != c || original.stage != udpControlReturned || c.controlCalls <= 0 || b.controlCalls <= 0
	failed := unknown || b.poisoned
	if failed {
		if unknown {
			c.controlFailed = true
		}
		b.poisoned = true
	} else {
		*original = udpControlCall{}
		c.controlCalls--
		b.controlCalls--
	}
	b.settleLocked()
	b.mu.Unlock()
	if failed {
		b.markUncertain() // control/RX/budget locks have all been released
		return errClientUncertain
	}
	return nil
}

func (c *udpPacketConn) udpControlStateError() error {
	b := c.defragger.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.poisoned || c.controlFailed {
		return errClientUncertain
	}
	if b.sealed || c.readSealed {
		return net.ErrClosed
	}
	return nil
}

func (c *udpPacketConn) setUDPDeadline(t time.Time, read, write bool) (err error) {
	original, err := c.beginUDPControl(t, read, write)
	if err != nil {
		return err
	}
	returned, locked := false, false
	defer func() {
		if !returned {
			c.poisonUDPControl() // before a waiting setter can acquire control
		} else {
			original.stage = udpControlReturned
		}
		if locked {
			c.control.Unlock()
		}
		if controlErr := c.endUDPControl(original, returned); controlErr != nil {
			err = errors.Join(err, controlErr)
		}
	}()
	c.control.Lock()
	locked = true
	original.stage = udpControlChecking
	if err = c.udpControlStateError(); err != nil {
		returned = true
		return err
	}
	if write {
		if c.sendScope == nil {
			returned = true
			return net.ErrClosed
		}
		original.stage = udpControlWriteDeadline
		if err = c.sendScope.SetWriteDeadline(t); err != nil {
			returned = true
			return err
		}
	}
	if read {
		original.stage = udpControlReadDeadline
		c.readDeadline.Set(t)
	}
	returned = true
	return nil
}

func (c *udpPacketConn) SetDeadline(t time.Time) error { return c.setUDPDeadline(t, true, true) }

func (c *udpPacketConn) SetReadDeadline(t time.Time) error { return c.setUDPDeadline(t, true, false) }

func (c *udpPacketConn) SetWriteDeadline(t time.Time) error { return c.setUDPDeadline(t, false, true) }

// Privileged original shutdown bypasses public control capacity. Seal is already
// set; after this mutex no admitted waiter may perform a late read-timer reset.
// Pinned Deadline.Set(zero) stops its timer or observes the fired close action.
func (c *udpPacketConn) stopReadDeadline() (err error) {
	if c.defragger == nil || c.defragger.budget == nil {
		return errClientUncertain
	}
	c.control.Lock()
	returned := false
	defer func() {
		if !returned {
			c.poisonUDPControl()
		}
		c.control.Unlock()
		if !returned {
			c.markShutdownUncertain()
		}
	}()
	b := c.defragger.budget
	b.mu.Lock()
	failed := c.controlFailed
	b.mu.Unlock()
	if failed {
		returned = true
		return errClientUncertain // do not replay an unknown timer operation
	}
	c.readDeadline.Set(time.Time{})
	returned = true
	return nil
}
