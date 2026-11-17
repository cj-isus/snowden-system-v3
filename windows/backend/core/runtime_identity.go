package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/snowden-system/windows/backend/config"
)

// RuntimeIdentity is private lifecycle evidence, never a Wails/UI/log payload.
// The digest covers already-rendered in-memory config; it must not be logged.
// A worker acknowledgement and observed adapter name do not prove kernel handle
// ownership or protect against a malicious local administrator.
type RuntimeIdentity struct {
	Instance           uint64
	ConfigDigest       [sha256.Size]byte
	TUNName            string
	DefaultOutbound    string
	AdmissionEpoch     uint64
	ForwardingAdmitted bool
}

func runtimeConfiguration(raw []byte) (*config.Config, [sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if len(raw) == 0 || len(raw) > maxWorkerConfigBytes {
		return nil, digest, errors.New("runtime identity config exceeds bound")
	}
	// Identity reads only owned JSON values. config.Parse also validates legacy
	// certificate files; repeating that under e.mu would add unrelated file I/O.
	cfg := new(config.Config)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(cfg); err != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, digest, errors.New("runtime identity config is invalid")
	}
	cfg.Normalize() // reviewed value-only aliases, matching Manager normalization
	normalized, err := json.Marshal(cfg)
	if err != nil {
		return nil, digest, errors.New("runtime identity normalization failed")
	}
	return cfg, sha256.Sum256(normalized), nil
}

func RuntimeConfigurationDigest(raw []byte) ([sha256.Size]byte, error) {
	_, digest, err := runtimeConfiguration(raw)
	return digest, err
}

// CurrentRuntime confirms that the same owned worker responds with its current
// selector. Called after Engine.Start/Reload has returned, never under e.mu.
func (e *Engine) CurrentRuntime(ctx context.Context) (RuntimeIdentity, error) {
	operation, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return RuntimeIdentity{}, err
	}
	rpc, cancel, err := prepareWorkerRPCContext(ctx, operation, workerControlTimeout)
	if err != nil {
		return RuntimeIdentity{}, err
	}
	cancelAttempted := false
	cancelRPC := func() {
		if !cancelAttempted {
			cancelAttempted = true // no repeated parent unlink after the final observation fence
			cancel()
		}
	}
	defer cancelRPC()
	identity, scope, err := e.currentRuntimePrepared(rpc)
	if err != nil {
		return RuntimeIdentity{}, err
	}
	cancelRPC() // ordinary cleanup return before publication, outside Engine.mu
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(operation); err != nil {
		return RuntimeIdentity{}, err
	}
	if !time.Now().Before(rpc.deadline) {
		return RuntimeIdentity{}, context.DeadlineExceeded // cleanup did not make an old observation fresh
	}
	if scope == nil || !e.isolated || e.cleanupErr != nil || e.process == nil || e.process != scope.process || e.process.instance != identity.Instance ||
		!e.process.ready || e.process.unusable || e.process.exited() || e.process.sequence != scope.id ||
		len(scope.raw) == 0 || len(e.rawJSON) != len(scope.raw) || &e.rawJSON[0] != &scope.raw[0] {
		return RuntimeIdentity{}, ErrEngineWorkerUnavailable // discard old facts, never retire a successor
	}
	return identity, nil
}

// The exact command scope retains the original owned graph until final
// post-cancel validation. Raw config is immutable; header identity avoids a
// second parse/hash after cancellation and cannot alias a freed old buffer.
// RPC/native work remains serialized; the caller prepares/cancels outside.
func (e *Engine) currentRuntimePrepared(ctx *workerRPCContext) (RuntimeIdentity, *workerStateReplyScope, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var identity RuntimeIdentity
	if ctx == nil {
		return identity, nil, ErrEngineWorkerUnavailable
	}
	if err := contextError(ctx); err != nil {
		return identity, nil, err
	}
	if !e.isolated || e.process == nil || e.cleanupErr != nil || e.process.instance == 0 {
		return identity, nil, ErrEngineWorkerUnavailable
	}
	instance := e.process.instance
	command := workerRequest{Action: "current", Selector: "proxy"}
	scope, err := prepareWorkerStateReply(e.process, e.rawJSON, command)
	if err != nil {
		return identity, nil, e.retireIsolatedFailureLocked(err)
	}
	command.replyState = scope
	reply, err := e.isolatedCommandLocked(ctx, command)
	if err != nil {
		return identity, nil, err
	}
	cfg, digest, err := runtimeConfiguration(e.rawJSON)
	if err != nil {
		return identity, nil, err
	}
	for _, inbound := range cfg.Inbounds {
		if inbound.Type == "tun" {
			if identity.TUNName != "" || inbound.InterfaceName == "" {
				return RuntimeIdentity{}, nil, errors.New("runtime identity has ambiguous TUN configuration")
			}
			identity.TUNName = inbound.InterfaceName
		}
	}
	if err := contextError(ctx); err != nil {
		return RuntimeIdentity{}, nil, err
	}
	if e.process != scope.process || e.process.instance != instance || e.process.exited() {
		return RuntimeIdentity{}, nil, ErrEngineWorkerUnavailable
	}
	identity.Instance, identity.ConfigDigest, identity.DefaultOutbound = instance, digest, reply.Previous
	identity.AdmissionEpoch, identity.ForwardingAdmitted = reply.AdmissionEpoch, reply.Admitted
	return identity, scope, nil
}
