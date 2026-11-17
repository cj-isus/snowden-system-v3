package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"

	"github.com/snowden-system/windows/backend/metadata"
)

var errUILifecycleRequest = errors.New("UI lifecycle protocol unavailable or request invalid")

type UILifecycleSessionView struct {
	Schema  int    `json:"schema"`
	Session string `json:"session"`
}

type UILifecycleRequest struct {
	Schema      int    `json:"schema"`
	Session     string `json:"session"`
	Sequence    string `json:"sequence"`
	Action      string `json:"action"`
	ChannelID   string `json:"channelId"`
	UpdateToken string `json:"updateToken"`
}

// Registration/ordinary-return witness only, never a native cleanup, admission
// or applied/protected-path receipt. No error payload or endpoint is serialized.
type UILifecycleReceipt struct {
	Schema               int    `json:"schema"`
	Session              string `json:"session"`
	Sequence             string `json:"sequence"`
	Action               string `json:"action"`
	Intent               string `json:"intent"`
	Outcome              string `json:"outcome"`
	HandlerEntered       bool   `json:"handlerEntered"`
	UpdateInstallEntered bool   `json:"updateInstallEntered"`
}

type uiLifecycleState struct {
	session        string
	sequence       uint64
	positive, stop *uiLifecycleAttempt // original callbacks, including an unreturned frame
}

type uiLifecycleAttempt struct {
	request        UILifecycleRequest
	owner          *uiLifecycleState
	registration   intentRegistration
	stopRequest    stopRequest
	accepted       bool
	outcome        string
	handlerEntered bool // original target body only; not a native side-effect receipt
	updateApproval *updatePreviewApproval
	updateProgress updateInstallProgress
}

// Opening a renderer scope changes ordering metadata only. It never cancels a
// live Engine, discards original calls, resets their slots or creates authority.
func (a *App) OpenUILifecycle() (*UILifecycleSessionView, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, errUILifecycleRequest
	}
	session := hex.EncodeToString(nonce[:])
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.appLifetimeCurrentLocked() || a.closing.Load() {
		return nil, errUILifecycleRequest
	}
	if a.uiLifecycle == nil {
		a.uiLifecycle = &uiLifecycleState{}
	}
	a.uiLifecycle.session, a.uiLifecycle.sequence = session, 0
	a.updateApproval = nil // a new renderer cannot adopt an old visible approval
	return &UILifecycleSessionView{Schema: 1, Session: session}, nil
}

func validUILifecycleRequest(request UILifecycleRequest) (uint64, bool) {
	if request.Schema != 1 || len(request.Session) != 64 || len(request.Sequence) == 0 || len(request.Sequence) > 20 {
		return 0, false
	}
	bytes, err := hex.DecodeString(request.Session)
	if err != nil || hex.EncodeToString(bytes) != request.Session {
		return 0, false
	}
	sequence, err := strconv.ParseUint(request.Sequence, 10, 64)
	if err != nil || sequence == 0 || strconv.FormatUint(sequence, 10) != request.Sequence {
		return 0, false
	}
	switch request.Action {
	case "start", "stop", "mode_proxy", "mode_tun":
		return sequence, request.ChannelID == "" && request.UpdateToken == ""
	case "switch":
		return sequence, request.UpdateToken == "" && len(request.ChannelID) <= 128 && metadata.ValidChannelID(request.ChannelID)
	case "update":
		if request.ChannelID != "" || len(request.UpdateToken) != 64 {
			return 0, false
		}
		token, err := hex.DecodeString(request.UpdateToken)
		return sequence, err == nil && hex.EncodeToString(token) == request.UpdateToken
	default:
		return 0, false
	}
}

// All ordering and original App registration happen under the same value lock.
// A newer Stop therefore rejects a late older Start before it can register work.
func (a *App) admitUILifecycle(attempt *uiLifecycleAttempt, sequence uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	owner, request := a.uiLifecycle, attempt.request
	if owner == nil || owner.session != request.Session || sequence <= owner.sequence {
		attempt.outcome = "superseded"
		return
	}
	owner.sequence = sequence // a rejected newer request still fences older queued work
	if owner.stop != nil || request.Action != "stop" && owner.positive != nil {
		attempt.outcome = "rejected"
		return
	}
	if request.Action == "mode_proxy" || request.Action == "mode_tun" {
		enable := request.Action == "mode_tun"
		registration, err := a.restartIntentLocked()
		if modeRequestRejection(enable) != nil || err != nil || enable == tunRequested() {
			attempt.outcome = "rejected" // pure preflight; no Stop/UAC/process callback entered
			return
		}
		attempt.registration = registration
	}
	if request.Action == "update" {
		approval, err := a.consumeUpdateApprovalLocked(request.UpdateToken, request.Session)
		if err != nil {
			attempt.outcome = "rejected"
			return
		}
		attempt.updateApproval, attempt.registration = approval, approval.registration
	}
	attempt.owner, attempt.accepted = owner, true
	if request.Action == "stop" {
		owner.stop = attempt
		registered, err := a.registerStopLocked(stopDisconnect)
		attempt.stopRequest, attempt.registration = registered, registered.intentRegistration
		if err != nil {
			attempt.outcome = "rejected"
		}
		return
	}
	owner.positive = attempt // before any handler callback or queued lifecycle wait
	if request.Action == "start" {
		attempt.handlerEntered = true
		if err := a.startLockedForUser(true); err != nil {
			attempt.outcome = "rejected"
		} else {
			attempt.outcome = "registered" // Start's worker/probe has not completed
		}
	}
	if request.Action == "start" || request.Action == "switch" {
		attempt.registration = a.intentRegistrationLocked()
	}
}

func (a *App) finishUILifecycle(attempt *uiLifecycleAttempt, returned bool) *UILifecycleReceipt {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !returned {
		attempt.outcome = "unconfirmed" // retain the original callback slot, including Goexit
	} else if attempt.accepted {
		owner := attempt.owner
		if attempt.request.Action == "stop" && owner.stop == attempt {
			owner.stop = nil
		} else if attempt.request.Action != "stop" && owner.positive == attempt {
			owner.positive = nil
		} else {
			attempt.outcome = "unconfirmed"
		}
		if (attempt.outcome == "registered" || attempt.outcome == "returned") &&
			(a.uiLifecycle != owner || owner.session != attempt.request.Session || owner.sequenceString() != attempt.request.Sequence) {
			attempt.outcome = "superseded"
		}
	}
	return &UILifecycleReceipt{Schema: 1, Session: attempt.request.Session, Sequence: attempt.request.Sequence,
		Action: attempt.request.Action, Intent: strconv.FormatUint(attempt.registration.intent, 10), Outcome: attempt.outcome,
		HandlerEntered: attempt.handlerEntered, UpdateInstallEntered: attempt.updateProgress.entered}
}

func (owner *uiLifecycleState) sequenceString() string { return strconv.FormatUint(owner.sequence, 10) }

// The original goroutine owns this whole call. No rescue goroutine, timeout
// retirement, generic error-to-success conversion or native fallback is added.
func (a *App) RequestUILifecycle(request UILifecycleRequest) (receipt *UILifecycleReceipt, result error) {
	sequence, valid := validUILifecycleRequest(request)
	if !valid {
		return nil, errUILifecycleRequest
	}
	attempt := &uiLifecycleAttempt{request: request, outcome: "pending"}
	returned := false
	defer func() {
		_ = recover() // no panic payload leaves this finite presentation boundary
		receipt = a.finishUILifecycle(attempt, returned)
	}()
	a.admitUILifecycle(attempt, sequence)
	if !attempt.accepted || attempt.outcome != "pending" {
		returned = true
		return nil, nil
	}
	var err error
	attempt.handlerEntered = true
	if request.Action == "stop" {
		err = a.acquireStopRequest(attempt.stopRequest)
		if err == nil {
			defer a.operationMu.Unlock()
			err = a.stopCoreRegisteredLocked(stopDisconnect, attempt.stopRequest.intentRegistration)
		}
	} else if request.Action == "mode_proxy" || request.Action == "mode_tun" {
		err = a.switchModeRegistered(request.Action == "mode_tun", attempt.registration)
	} else if request.Action == "update" {
		_, err = a.applyUpdateRegistered(attempt.updateApproval.directory, attempt.registration, attempt.updateApproval, &attempt.updateProgress)
	} else {
		err = a.switchChannelCoreRegistered(attempt.registration, request.ChannelID)
	}
	attempt.outcome = "returned"
	if err != nil {
		attempt.outcome = "failed"
		if localRetirementFailure(err) {
			attempt.outcome = "unconfirmed"
		}
	}
	returned = true
	return nil, nil
}

// Explicit legacy Disconnect must also fence an already queued protocol Start.
// Keep the enabled marker and every original callback slot; no positive legacy
// fallback is re-enabled by closing this session.
func (a *App) retireUILifecycleLocked() {
	if a.uiLifecycle != nil {
		a.uiLifecycle.session = ""
	}
	a.updateApproval = nil
}
