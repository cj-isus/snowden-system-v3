package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

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
	e.mu.Lock()
	defer e.mu.Unlock()
	var identity RuntimeIdentity
	if err := contextError(ctx); err != nil {
		return identity, err
	}
	if !e.isolated || e.process == nil || e.cleanupErr != nil || e.process.instance == 0 {
		return identity, ErrEngineWorkerUnavailable
	}
	instance := e.process.instance
	reply, err := e.isolatedCommandLocked(ctx, workerRequest{Action: "current", Selector: "proxy"})
	if err != nil {
		return identity, err
	}
	cfg, digest, err := runtimeConfiguration(e.rawJSON)
	if err != nil {
		return identity, err
	}
	for _, inbound := range cfg.Inbounds {
		if inbound.Type == "tun" {
			if identity.TUNName != "" || inbound.InterfaceName == "" {
				return RuntimeIdentity{}, errors.New("runtime identity has ambiguous TUN configuration")
			}
			identity.TUNName = inbound.InterfaceName
		}
	}
	if err := contextError(ctx); err != nil {
		return RuntimeIdentity{}, err
	}
	if e.process == nil || e.process.instance != instance || e.process.exited() {
		return RuntimeIdentity{}, ErrEngineWorkerUnavailable
	}
	identity.Instance, identity.ConfigDigest, identity.DefaultOutbound = instance, digest, reply.Previous
	identity.AdmissionEpoch, identity.ForwardingAdmitted = reply.AdmissionEpoch, reply.Admitted
	return identity, nil
}
