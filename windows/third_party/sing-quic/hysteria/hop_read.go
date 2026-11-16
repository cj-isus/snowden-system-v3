package hysteria

import (
	"net"
	"os"
	"time"

	"github.com/sagernet/quic-go/monotime"
)

type hopReadTimeout struct{}

func (hopReadTimeout) Error() string   { return "hysteria: read deadline exceeded" }
func (hopReadTimeout) Timeout() bool   { return true }
func (hopReadTimeout) Temporary() bool { return true }
func (hopReadTimeout) Unwrap() error   { return os.ErrDeadlineExceeded }

// Like the existing scoped DATAGRAM bridge, anchor a standard deadline at one
// wall/monotonic reference. Saturation and a finite zero collision stay finite;
// only time.Time{} means unlimited. No trusted UTC or hard scheduler SLO implied.
func hopReadDeadline(t time.Time) monotime.Time {
	if t.IsZero() {
		return 0
	}
	reference := time.Now()
	base, delta := int64(monotime.FromTime(reference)), int64(t.Sub(reference))
	const maximum int64 = 1<<63 - 1
	const minimum int64 = -maximum - 1
	var value int64
	switch {
	case delta > 0 && base > maximum-delta:
		value = maximum
	case delta < 0 && base < minimum-delta:
		value = minimum
	default:
		value = base + delta
	}
	if value == 0 {
		value = 1
	}
	return monotime.Time(value)
}

func hopReadRemaining(deadline, now monotime.Time) time.Duration {
	if deadline <= now {
		return 0
	}
	// Unsigned subtraction after ordering preserves a positive signed-domain
	// distance without overflowing even when the instants have opposite signs.
	distance := uint64(deadline) - uint64(now)
	const maximum = uint64(1<<63 - 1)
	if distance > maximum {
		return time.Duration(maximum)
	}
	return time.Duration(distance)
}

func (c *HopPacketConn) SetReadDeadline(t time.Time) error {
	deadline := hopReadDeadline(t)
	c.access.Lock()
	defer c.access.Unlock()
	if c.done {
		return net.ErrClosed
	}
	c.readDeadline = deadline
	close(c.readChanged)
	c.readChanged = make(chan struct{})
	return nil
}

// Called only under access. Readiness is a hint; dequeue, deadline and original
// socket fault decisions are all repeated under the same owner fence.
func (c *HopPacketConn) signalReadLocked() {
	if len(c.packetChan) != 0 || c.readFailure != nil && c.readFailureSocket == c.currentConn {
		select {
		case c.packetReady <- struct{}{}:
		default:
		}
	} else {
		select {
		case <-c.packetReady:
		default:
		}
	}
}

func (c *HopPacketConn) recordReadFailure(original *hopSocket, err error) {
	c.access.Lock()
	defer c.access.Unlock()
	if c.done || original != c.currentConn {
		return // a previous/retiring generation cannot poison the new socket
	}
	c.readFailureSocket, c.readFailure = original, err
	c.signalReadLocked()
}

func stopHopReadTimer(timer *time.Timer) {
	if timer != nil && !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// Public read deadlines govern the bounded wrapper queue, not the native receive
// loops. A timeout leaves those original loops alive for extend/clear and hopping.
func (c *HopPacketConn) ReadFrom(output []byte) (int, net.Addr, error) {
	if _, err := c.beginCall(); err != nil {
		return 0, nil, err
	}
	returned := false
	defer func() { c.endCall(returned) }()
	var timer *time.Timer
	defer func() { stopHopReadTimer(timer) }()
	for {
		c.access.Lock()
		if c.done {
			c.access.Unlock()
			returned = true
			return 0, nil, net.ErrClosed
		}
		deadline, changed := c.readDeadline, c.readChanged
		if !deadline.IsZero() && hopReadRemaining(deadline, monotime.Now()) == 0 {
			c.access.Unlock()
			returned = true
			return 0, nil, hopReadTimeout{}
		}
		select {
		case packet := <-c.packetChan:
			c.signalReadLocked()
			c.access.Unlock()
			n := copy(output, packet.Bytes())
			packet.Release()
			returned = true
			return n, (*hopFakeAddr)(nil), nil
		default:
		}
		if c.readFailure != nil && c.readFailureSocket == c.currentConn {
			err := c.readFailure
			originals, started := c.sealCloseLocked(nil)
			c.access.Unlock()
			// This is a returned native I/O failure, not an uncertain Close.
			// Retire through the original owner; preserve any actual close error.
			if started {
				c.launchClose(originals)
			}
			returned = true
			return 0, nil, err
		}
		c.readFailure, c.readFailureSocket = nil, nil
		c.signalReadLocked()
		c.access.Unlock()

		stopHopReadTimer(timer)
		var timeout <-chan time.Time
		if !deadline.IsZero() {
			remaining := hopReadRemaining(deadline, monotime.Now())
			if remaining == 0 {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(remaining)
			} else {
				timer.Reset(remaining)
			}
			timeout = timer.C
		}
		select {
		case <-c.doneChan:
		case <-c.packetReady:
		case <-changed:
		case <-timeout:
			// A tick from an old deadline is only a wakeup, never a decision.
		}
	}
}
