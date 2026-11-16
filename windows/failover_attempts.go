package main

import "time"

const candidateAttemptTTL = 2 * reloadCooldown
const maxCandidateAttemptHints = 64

// Attempt hints only avoid repeating expensive failed work. They are not
// evidence of transport/DNS/egress failure and cannot remove a candidate from
// the eligible set, quarantine it, or bypass its protected probe.
func (p *failoverPolicy) expireAttemptsLocked(epoch int, now time.Time) {
	if p.recentEpoch != epoch {
		p.recentEpoch = epoch
		p.recentAttempts = nil
	}
	for id, at := range p.recentAttempts {
		if now.Before(at) || now.Sub(at) >= candidateAttemptTTL {
			delete(p.recentAttempts, id)
		}
	}
}

func (p *failoverPolicy) noteAttemptFailure(id string, epoch int, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expireAttemptsLocked(epoch, now)
	if p.recentAttempts == nil {
		p.recentAttempts = map[string]time.Time{}
	}
	if _, known := p.recentAttempts[id]; !known && len(p.recentAttempts) >= maxCandidateAttemptHints {
		oldestID := ""
		var oldest time.Time
		for key, at := range p.recentAttempts {
			if oldestID == "" || at.Before(oldest) {
				oldestID, oldest = key, at
			}
		}
		delete(p.recentAttempts, oldestID)
	}
	p.recentAttempts[id] = now
}

func (p *failoverPolicy) orderByRecentAttempts(ids []string, epoch int, now time.Time) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expireAttemptsLocked(epoch, now)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, recent := p.recentAttempts[id]; !recent {
			out = append(out, id)
		}
	}
	for _, id := range ids {
		if _, recent := p.recentAttempts[id]; recent {
			out = append(out, id)
		}
	}
	return out
}

func (p *failoverPolicy) clearAttemptHints() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.recentAttempts = nil
	p.mu.Unlock()
}
