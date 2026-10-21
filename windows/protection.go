package main

import (
	"context"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/render"
)

type protectionPolicy interface {
	Arm() error
	BlockDirect() error
	Release() error
}

type strictWFPPolicy struct{}

func (strictWFPPolicy) Arm() error {
	exe, err := osExecutable()
	if err != nil {
		return err
	}
	return armWFPKillSwitch(exe)
}

func (strictWFPPolicy) BlockDirect() error { return revokeWFPTUN() }
func (strictWFPPolicy) Release() error     { return disarmKillSwitch() }

func (a *App) protectionPolicy() protectionPolicy {
	if a.policy != nil {
		return a.policy
	}
	return strictWFPPolicy{}
}

// StopReason determines policy cleanup, independently of the engine state.
// Only explicit Disconnect authorizes opening direct traffic (FR-009/012).
type StopReason uint8

const (
	stopFailure StopReason = iota
	stopDisconnect
	stopQuit
	stopRestart
)

func (r StopReason) releasesProtection() bool { return r == stopDisconnect }

// Caller holds operationMu. a.mu is never held while calling Manager/network.
func (a *App) beginOperation(id uint64) (context.Context, context.CancelFunc, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.protectionDesired || a.closing.Load() || a.operationID != id {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.operationCancel = cancel
	return ctx, cancel, true
}

func (a *App) operationCurrent(id uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.protectionDesired && !a.closing.Load() && a.operationID == id
}

func (a *App) endOperation(cancel context.CancelFunc) {
	cancel()
	// Serialized by operationMu: no successor can have registered a cancel.
	a.mu.Lock()
	a.operationCancel = nil
	a.mu.Unlock()
}

func (a *App) stopCoreFor(reason StopReason) error {
	a.mu.Lock()
	a.operationID++
	if reason.releasesProtection() {
		a.protectionDesired = false
	}
	a.cancelFailClosedRetryLocked()
	a.state.State = "stopping"
	a.pushStateLocked()
	if a.operationCancel != nil {
		a.operationCancel()
	}
	a.mu.Unlock()
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	return a.stopCoreLocked(reason)
}

func (a *App) sessionCurrent(cc *render.ClientConfig) bool {
	runtime_.mu.Lock()
	ours := cc != nil && runtime_.cc == cc && runtime_.manager != nil
	runtime_.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	return ours && a.protectionDesired && !a.closing.Load()
}

// A watchdog result belongs to its captured session. Recheck after acquiring
// the lifecycle lock: it must never tear down a new session or undo Disconnect.
func (a *App) failSession(cc *render.ClientConfig, report *core.ProbeReport, reason string, exhausted bool) bool {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	if !a.sessionCurrent(cc) {
		return false
	}
	err := a.stopCoreLocked(stopFailure)
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.protectionDesired || a.closing.Load() {
		return false
	}
	a.state.State = "error"
	a.state.Error = reason
	if err != nil {
		a.state.Error += "; cleanup: " + err.Error()
	} else if exhausted {
		a.state.State = "stopped"
		a.state.BlockedReason = "all_channels_failed"
	}
	a.state.Probe = toView(report)
	a.scheduleFailClosedRetryLocked(time.Now())
	a.sm.markRecovering(time.Now())
	a.pushStateLocked()
	return true
}
