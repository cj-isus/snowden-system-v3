package core

import (
	"crypto/sha256"
	"encoding/hex"
)

// Immutable parent-only expectation for one original process/next command.
// It is never serialized. Digests/selected tags are comparison values, not a
// forwarding permit; targetRaw is the bounded prepared selector publication.
type workerStateReplyScope struct {
	process                  *engineProcess
	id                       uint64
	action, selector, target string
	before, after, selected  string
	raw, targetRaw           []byte
}

func workerStateReplyAction(action string) bool {
	return action == "start" || action == "current" || action == "switch"
}

// Protocol reservation only; no source, health or forwarding permission.
func (process *engineProcess) canReserveStateCommand() bool {
	return process != nil && process.ready && !process.unusable && !process.exited() && process.sequence != ^uint64(0)
}

// Engine.mu owns raw and the original process across preparation and request.
// Only owned JSON values are read; no certificate files or native operations.
func prepareWorkerStateReply(process *engineProcess, raw []byte, request workerRequest) (*workerStateReplyScope, error) {
	if process == nil {
		return nil, ErrEngineNotRunning
	}
	if !process.canReserveStateCommand() {
		return nil, ErrEngineWorkerUnavailable
	}
	if !workerStateReplyAction(request.Action) || request.Action != "start" && request.Selector != "proxy" {
		return nil, errWorkerProtocol
	}
	cfg, digest, err := runtimeConfiguration(raw)
	if err != nil {
		return nil, err
	}
	scope := &workerStateReplyScope{process: process, id: process.sequence + 1, action: request.Action, selector: request.Selector, target: request.Tag,
		before: hex.EncodeToString(digest[:])}
	scope.after = scope.before
	index := -1
	for i, outbound := range cfg.Outbounds {
		if outbound.Tag == "proxy" {
			if index != -1 || outbound.Type != "selector" || !validWorkerTag(outbound.Default) {
				return nil, ErrSelectorSwitchUnavailable
			}
			index, scope.selected = i, outbound.Default
		}
	}
	if index < 0 {
		return nil, ErrSelectorSwitchUnavailable
	}
	selectedMember, targetMember := false, false
	for _, candidate := range cfg.Outbounds[index].Outbounds {
		selectedMember = selectedMember || candidate == scope.selected
		targetMember = targetMember || candidate == request.Tag
	}
	if !selectedMember {
		return nil, ErrSelectorSwitchUnavailable
	}
	switch request.Action {
	case "start":
		scope.raw = raw // original owned slice retained until this exchange actually returns
	case "switch":
		if !validWorkerTag(request.Tag) {
			return nil, ErrSelectorSwitchUnavailable
		}
		if !targetMember {
			scope.after = "" // only a nonfatal selector_unavailable reply can match
			break
		}
		scope.targetRaw, err = marshalSelectorConfiguration(cfg, "proxy", request.Tag)
		if err != nil {
			return nil, err
		}
		// runtimeConfiguration already normalized this typed graph; changing
		// Default does not change Normalize's urltest alias transformation.
		after := sha256.Sum256(scope.targetRaw)
		scope.after = hex.EncodeToString(after[:])
	}
	return scope, nil
}

func (scope *workerStateReplyScope) matches(process *engineProcess, request *workerRequest) bool {
	if scope == nil || request == nil || scope.process != process || scope.id != request.ID || scope.action != request.Action || scope.selector != request.Selector || scope.target != request.Tag {
		return false
	}
	return request.Action != "start" || len(scope.raw) > 0 && len(scope.raw) == len(request.Raw) && &scope.raw[0] == &request.Raw[0]
}

func workerStateReplyFailure(request *workerRequest, reply workerReply) error {
	scope := request.replyState
	if scope == nil || scope.action != request.Action || scope.selector != request.Selector || scope.target != request.Tag {
		return errWorkerProtocol
	}
	if reply.Code == "state_error" || request.Action == "start" && reply.Code == "start_failed" {
		if emptyWorkerFailureReply(reply) {
			return ErrEngineWorkerUnavailable
		}
		return errWorkerProtocol
	}
	if !validWorkerState(reply) {
		return errWorkerProtocol
	}
	if request.Action != "start" && reply.Code == "selector_unavailable" {
		if reply.Previous == "" && reply.Subject == scope.before {
			return nil // valid nonfatal result; never a channel-health or admission claim
		}
		return errWorkerProtocol
	}
	if reply.Code != "ok" || scope.after == "" || reply.Subject != scope.after {
		return errWorkerProtocol
	}
	if request.Action == "start" {
		if reply.Previous != "" || reply.Admitted {
			return errWorkerProtocol
		}
	} else if reply.Previous != scope.selected || request.Action == "switch" && scope.target != scope.selected && reply.Admitted {
		return errWorkerProtocol
	}
	return nil
}
