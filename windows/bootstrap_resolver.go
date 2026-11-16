package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/netprobe"
	"github.com/snowden-system/windows/backend/render"
)

// Resolve only bootstrap service names from this coherent descriptor snapshot.
// These are address hints for the same TLS/Host identity, not protected user DNS.
func (a *App) prepareBootstrap(ctx context.Context, snapshot *render.DescriptorSnapshot) (map[string]core.PreflightAddress, error) {
	if snapshot == nil || !lifecyclePublicationCurrent(ctx) {
		return nil, core.ErrPreflightScope
	}
	raw, err := json.Marshal(struct {
		Source, Envelope, PinPolicy string
		Version, Epoch              uint64
		Channels                    []render.ChannelDescriptor
	}{snapshot.Profile.Source, snapshot.Profile.EnvelopeSHA256, snapshot.Profile.PinPolicySHA256,
		snapshot.Profile.Version, snapshot.Profile.ProfileEpoch, snapshot.Channels})
	if err != nil {
		return nil, err
	}
	profile := sha256.Sum256(raw)
	path := netEpoch.pathSource()
	epoch, started, _ := netEpoch.current()
	clock := a.trustClock()
	a.mu.Lock()
	intent := a.operationID
	a.mu.Unlock()
	current := func() bool {
		if !path.current.Load() || !lifecyclePublicationCurrent(ctx) {
			return false
		}
		a.mu.Lock()
		valid := a.operationID == intent && a.observedClock == clock && !a.profileTransition &&
			!a.closing.Load() && !a.installedImageChanged && a.restartCommit == nil
		a.mu.Unlock()
		number, began, _ := netEpoch.current()
		return valid && number == epoch && began.Equal(started)
	}
	now := func() (time.Time, error) {
		value, err := clock.Now(snapshot.Profile.ClockFloor)
		if err == nil && !snapshot.Profile.ExpiresAt.IsZero() && !value.Before(snapshot.Profile.ExpiresAt) {
			err = metadata.ErrProfileExpired
		}
		return value, err
	}
	if !current() {
		return nil, core.ErrPreflightScope
	}
	if _, err := now(); err != nil {
		return nil, err
	}
	resolver := core.PreflightResolver{
		CachePath: filepath.Join(appDataDir(), "resolve-cache.json"), Now: now, Current: current,
		Scope: core.PreflightScope{ProfileSHA256: hex.EncodeToString(profile[:]), Role: core.BootstrapTLSWebSocket,
			NetworkEpoch: epoch, NetworkStarted: started, Until: snapshot.Profile.ExpiresAt},
		// Core emits fixed address-hint notices, never host/IP or error text.
		Logf: func(message string, _ ...any) { a.appendLog("info", message) },
	}
	batch, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	hints := map[string]core.PreflightAddress{}
	attempts := 0
	for _, channel := range render.Selectable(snapshot.Channels) {
		if !current() {
			return nil, core.ErrPreflightScope
		}
		if channel.Protocol != "vless" || channel.Transport != "ws" || net.ParseIP(channel.Hostname) != nil {
			continue
		}
		if !lifecyclePublicationCurrent(batch) || attempts >= 8 {
			break
		} // unresolved names retain the existing engine bootstrap route
		endpointRaw, err := json.Marshal(channel)
		if err != nil {
			return nil, err
		}
		endpoint := sha256.Sum256(endpointRaw)
		resolver.Scope.EndpointSHA256 = hex.EncodeToString(endpoint[:])
		attempts++
		address, err := resolver.ResolveAddress(batch, channel.Hostname)
		if err != nil {
			if !current() || errors.Is(err, core.ErrPreflightCleanup) || errors.Is(err, core.ErrPreflightBusy) ||
				errors.Is(err, metadata.ErrClockUncertain) ||
				errors.Is(err, metadata.ErrProfileExpired) || errors.Is(err, netprobe.ErrDNSResponse) {
				return nil, err // no cache/hostname fallback after a local or stale-owner failure
			}
			if errors.Is(err, core.ErrPreflightScope) {
				if errors.Is(err, context.DeadlineExceeded) && errors.Is(batch.Err(), context.DeadlineExceeded) && lifecyclePublicationCurrent(ctx) {
					break // only the local batch budget ended after confirmed network retirement
				}
				return nil, err
			}
			a.appendLog("warn", "preflight: "+channel.ID+" — адрес не подтверждён; остаётся настроенное разрешение имени, путь ещё не проверен")
			continue
		}
		hints[channel.ID] = address
	}
	if !current() {
		return nil, core.ErrPreflightScope
	}
	observed, err := now()
	if err != nil {
		return nil, err
	}
	for id, hint := range hints {
		if !hint.ValidAt(observed) {
			delete(hints, id)
		}
	}
	return hints, nil
}
