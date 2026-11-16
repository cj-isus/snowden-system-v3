package quic

import (
	"slices"
	"time"

	"github.com/sagernet/quic-go/internal/monotime"
)

const (
	maxServerPathProbes               = 3
	maxServerPathDelay  time.Duration = 1<<63 - 1
)

// This clock belongs to the path, independently of packet ACK/loss history.
// The existing connection loop and timer alone read or mutate it.
type serverPathValidation struct {
	started, expires, lastSent, nextProbe monotime.Time
	pto                                   time.Duration
	sent                                  int
}

func scaledServerPathDelay(pto time.Duration, factor int) time.Duration {
	if factor <= 0 || pto <= 0 || pto > maxServerPathDelay/time.Duration(factor) {
		return maxServerPathDelay
	}
	return pto * time.Duration(factor)
}

func serverPathAfter(now monotime.Time, delay time.Duration) monotime.Time {
	const latest monotime.Time = 1<<63 - 1
	if now > 0 && monotime.Time(delay) > latest-now {
		return latest
	}
	return now.Add(delay)
}

func (v *serverPathValidation) start(now monotime.Time, pto time.Duration) {
	*v = serverPathValidation{started: now, pto: pto,
		expires: serverPathAfter(now, scaledServerPathDelay(pto, 7))}
}

func (v *serverPathValidation) expired(now monotime.Time) bool {
	return !v.expires.IsZero() && !now.Before(v.expires)
}

// RTT changes can lengthen this phase, never repeatedly restart it from now.
func (v *serverPathValidation) extendPTO(pto time.Duration) {
	if v.expires.IsZero() || pto <= v.pto {
		return
	}
	v.pto = pto
	v.expires = max(v.expires, serverPathAfter(v.started, scaledServerPathDelay(pto, 7)))
	if v.sent == 0 {
		return
	}
	wait := scaledServerPathDelay(pto, 1<<(v.sent-1))
	v.expires = max(v.expires, serverPathAfter(v.lastSent, wait))
	if !v.nextProbe.IsZero() {
		v.nextProbe = serverPathAfter(v.lastSent, wait)
	}
}

func (v *serverPathValidation) recordSent(now monotime.Time) {
	if v.sent == 0 {
		// Admission can wait for credit or a CID. The first actual send gets a
		// complete response window; admission itself still has a finite expiry.
		v.started = now
		v.expires = serverPathAfter(now, scaledServerPathDelay(v.pto, 7))
	}
	v.sent++
	v.lastSent = now
	wait := scaledServerPathDelay(v.pto, 1<<(v.sent-1))
	v.expires = max(v.expires, serverPathAfter(now, wait))
	v.nextProbe = 0
	if v.sent < maxServerPathProbes {
		v.nextProbe = serverPathAfter(now, wait)
	}
}

func (p *path) startValidation(now monotime.Time, pto time.Duration) {
	p.validation.start(now, pto)
	p.renewChallenge() // retain sent history for delayed responses to this path
}

func (c *Conn) serverPathPTO() time.Duration {
	pto := c.rttStats.PTO(true)
	ackDelay := max(c.rttStats.MaxAckDelay(), 0)
	if pto <= 0 || ackDelay > maxServerPathDelay-time.Second {
		return maxServerPathDelay
	}
	// An old path's short RTT doesn't establish the RTT of a new path. RFC9002
	// 6.2.2 recommends an initial PTO near one second when no estimate exists.
	return max(pto, time.Second+ackDelay)
}

func (c *Conn) nextServerPathValidationDeadline() monotime.Time {
	if c.pathManager == nil {
		return 0
	}
	var deadline monotime.Time
	for _, p := range c.pathManager.paths {
		candidate := p.validation.expires
		if !p.challengePending && (p.validated || p.credit > 0) {
			if retry := p.validation.nextProbe; !retry.IsZero() && (candidate.IsZero() || retry.Before(candidate)) {
				candidate = retry
			}
		}
		if !candidate.IsZero() && (deadline.IsZero() || candidate.Before(deadline)) {
			deadline = candidate
		}
	}
	return deadline
}

func (c *Conn) advanceServerPathValidation(now monotime.Time) error {
	pm := c.pathManager
	if pm == nil {
		return nil
	}
	if pm.retiring != nil {
		return errPacketOutputUncertain // never retry an unknown original CID retirement
	}
	pto := c.serverPathPTO()
	for i := 0; i < len(pm.paths); {
		p := pm.paths[i]
		p.validation.extendPTO(pto)
		if p.validation.expired(now) {
			// Keep the exact row attached through CID/reset/control-frame callbacks.
			// An abnormal return leaves it in both original owners and run fails.
			pm.retiring = p
			if p.cidAssigned {
				pm.retireConnID(p.id)
			}
			pm.paths = slices.Delete(pm.paths, i, i+1)
			pm.retiring = nil
			continue // the active peer and its connection are unchanged
		}
		if retry := p.validation.nextProbe; !retry.IsZero() && !now.Before(retry) {
			p.validation.nextProbe = 0 // pending unsent work cannot leave a past alarm
			p.renewChallenge()
		}
		i++
	}
	return nil
}
