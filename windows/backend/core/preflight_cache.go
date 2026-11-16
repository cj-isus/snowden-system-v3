package core

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/netprobe"
)

const (
	BootstrapTLSWebSocket  = "tunnel_vless_ws_tls"
	BootstrapWorkerService = "worker_service"
	preflightCacheLimit    = 64 << 10
	preflightCacheEntries  = 64
	preflightCacheTTL      = 5 * time.Minute // ceiling; DNS/CNAME TTL and profile validity can only shorten it
)

// This is the identity of an address hint, never a TLS identity, successful
// channel observation or forwarding authority. Epoch zero cannot reuse cache.
type PreflightScope struct {
	ProfileSHA256  string    `json:"profile_sha256"`
	SourceSHA256   string    `json:"source_sha256,omitempty"`
	EndpointSHA256 string    `json:"endpoint_sha256"`
	Role           string    `json:"role"`
	NetworkEpoch   int       `json:"network_epoch"`
	NetworkStarted time.Time `json:"network_started"`
	Until          time.Time `json:"until"`
}

// A scoped, expiring address hint for one candidate construction. This value
// carries no permission to send user traffic or to refresh an active route.
type PreflightAddress struct {
	Address    string
	Host       string
	Scope      PreflightScope
	ObservedAt time.Time
	Until      time.Time
	Cached     bool
}

func (address PreflightAddress) ValidAt(now time.Time) bool {
	host, err := netprobe.CanonicalDNSHost(address.Host)
	validHost := err == nil && host == address.Host || net.ParseIP(address.Host) != nil && address.Host == address.Address
	return validHost && address.Scope.valid() && publicProbeIPv4(net.ParseIP(address.Address)) &&
		!address.ObservedAt.IsZero() && !now.Before(address.ObservedAt) && now.Before(address.Until) &&
		(address.Scope.Until.IsZero() || !address.Until.After(address.Scope.Until)) &&
		(!address.Cached || address.Scope.NetworkEpoch > 0)
}

func (scope PreflightScope) valid() bool {
	canonicalHash := func(value string) bool {
		raw, err := hex.DecodeString(value)
		return err == nil && len(raw) == sha256.Size && hex.EncodeToString(raw) == value
	}
	identity := scope.Role == BootstrapTLSWebSocket && canonicalHash(scope.ProfileSHA256) && scope.SourceSHA256 == "" ||
		scope.Role == BootstrapWorkerService && canonicalHash(scope.SourceSHA256) && scope.ProfileSHA256 == "" && scope.NetworkEpoch == 0 && scope.NetworkStarted.IsZero()
	return identity && canonicalHash(scope.EndpointSHA256) && scope.NetworkEpoch >= 0 &&
		(scope.NetworkEpoch == 0 || !scope.NetworkStarted.IsZero() && scope.NetworkStarted.Year() >= 1 && scope.NetworkStarted.Year() <= 9999) &&
		(scope.Until.IsZero() || scope.Until.Year() >= 1 && scope.Until.Year() <= 9999)
}

type preflightCacheEntry struct {
	Scope PreflightScope `json:"scope"`
	Host  string         `json:"host"`
	IP    string         `json:"ip"`
	At    time.Time      `json:"at"`
	Until time.Time      `json:"until"`
}

type preflightCacheDocument struct {
	Schema  int                            `json:"schema"`
	Process string                         `json:"process"`
	Entries map[string]preflightCacheEntry `json:"entries"`
}

// Reuse existing original-reader/writer ownership, including uncertain Close.
// These slots are not rebuilt per resolver and are independent of trust files.
var preflightCache = struct {
	mu     sync.Mutex
	once   sync.Once
	id     string
	err    error
	reader *filetxn.BoundedReadSlot
	writer *filetxn.BoundedWriteSlot
}{reader: filetxn.NewBoundedReadSlot(preflightCacheLimit), writer: filetxn.NewBoundedWriteSlot(preflightCacheLimit)}

func preflightProcessID() (string, error) {
	preflightCache.once.Do(func() {
		preflightCache.err = ErrPreflightScope
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			preflightCache.err = err
			return
		}
		preflightCache.id = hex.EncodeToString(raw[:])
		preflightCache.err = nil
	})
	return preflightCache.id, preflightCache.err
}

func (scope PreflightScope) cacheKey(host string) string {
	// Scope times are encoded as wall instants; no serialized monotonic value
	// is mistaken for a process clock. Process identity is separate and exact.
	scope.NetworkStarted = scope.NetworkStarted.UTC()
	scope.Until = scope.Until.UTC()
	raw, err := json.Marshal(struct {
		Scope PreflightScope
		Host  string
	}{scope, host})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (entry preflightCacheEntry) valid(now time.Time) bool {
	host, err := netprobe.CanonicalDNSHost(entry.Host)
	return err == nil && host == entry.Host && entry.Scope.valid() && entry.Scope.cacheKey(host) != "" && entry.Scope.NetworkEpoch > 0 &&
		publicProbeIPv4(net.ParseIP(entry.IP)) && !entry.At.IsZero() && !now.Before(entry.At) &&
		entry.Until.After(entry.At) && now.Before(entry.Until) && entry.Until.Sub(entry.At) <= preflightCacheTTL &&
		(entry.Scope.Until.IsZero() || !entry.Until.After(entry.Scope.Until))
}

// The caller holds the cache I/O gate. No App/clock/network callback is invoked
// inside it. A malformed/legacy file is unavailable, not a trusted old seed.
func loadPreflightCache(path, process string) (preflightCacheDocument, error) {
	empty := preflightCacheDocument{Schema: 1, Process: process, Entries: map[string]preflightCacheEntry{}}
	if err := preflightCache.writer.Check(); err != nil {
		return empty, err
	}
	raw, err := preflightCache.reader.Read(path, preflightCacheLimit)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	var encoded struct {
		Schema  int             `json:"schema"`
		Process string          `json:"process"`
		Entries json.RawMessage `json:"entries"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&encoded); err != nil {
		return empty, nil // confirmed-retired legacy/corrupt hint may be replaced only by fresh DNS data
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF || encoded.Schema != 1 {
		return empty, nil
	}
	if encoded.Process != process {
		return empty, nil // no cross-process/reboot network-identity assertion
	}
	entries := json.NewDecoder(bytes.NewReader(encoded.Entries))
	entries.DisallowUnknownFields()
	if opening, err := entries.Token(); err != nil || opening != json.Delim('{') {
		return empty, nil
	}
	document := preflightCacheDocument{Schema: 1, Process: process, Entries: map[string]preflightCacheEntry{}}
	for entries.More() {
		if len(document.Entries) >= preflightCacheEntries {
			return empty, nil
		} // before allocating another record
		token, err := entries.Token()
		key, ok := token.(string)
		if err != nil || !ok || len(key) != 64 {
			return empty, nil
		}
		if _, duplicate := document.Entries[key]; duplicate {
			return empty, nil
		}
		var entry preflightCacheEntry
		if err := entries.Decode(&entry); err != nil {
			return empty, nil
		}
		document.Entries[key] = entry
	}
	if closing, err := entries.Token(); err != nil || closing != json.Delim('}') {
		return empty, nil
	}
	if err := entries.Decode(&trailing); err != io.EOF {
		return empty, nil
	}
	return document, nil
}

func (r *PreflightResolver) cacheGet(host string) (string, bool) {
	entry, ok := r.cacheAddress(host)
	return entry.IP, ok
}

func (r *PreflightResolver) cacheAddress(host string) (preflightCacheEntry, bool) {
	if r.CachePath == "" || !r.Scope.valid() || r.Scope.NetworkEpoch == 0 || !r.current() {
		return preflightCacheEntry{}, false
	}
	key := r.Scope.cacheKey(host)
	if key == "" {
		return preflightCacheEntry{}, false
	}
	now, err := r.now()
	if err != nil {
		return preflightCacheEntry{}, false
	}
	process, err := preflightProcessID()
	if err != nil || !preflightCache.mu.TryLock() {
		return preflightCacheEntry{}, false
	}
	var document preflightCacheDocument
	func() {
		defer preflightCache.mu.Unlock()
		document, err = loadPreflightCache(r.CachePath, process)
	}()
	entry, found := document.Entries[key]
	if err == nil {
		now, err = r.now() // synchronous I/O cannot extend a hint's freshness
	}
	if err != nil || !found || entry.Scope.cacheKey(entry.Host) != key || !entry.valid(now) || !r.current() {
		return preflightCacheEntry{}, false
	}
	return entry, true
}

func (r *PreflightResolver) cachePutAnswer(host string, answer netprobe.DNSHostAnswer, observed time.Time) {
	if r.CachePath == "" || !r.Scope.valid() || r.Scope.NetworkEpoch == 0 || answer.TTL == 0 || len(answer.Addresses) == 0 || !r.current() {
		return
	}
	duration := min(time.Duration(answer.TTL)*time.Second, preflightCacheTTL)
	until := observed.Add(duration)
	if !r.Scope.Until.IsZero() && r.Scope.Until.Before(until) {
		until = r.Scope.Until
	}
	entry := preflightCacheEntry{Scope: r.Scope, Host: host, IP: answer.Addresses[0], At: observed, Until: until}
	now, err := r.now()
	if err != nil || !entry.valid(now) {
		return
	}
	process, err := preflightProcessID()
	if err != nil || !preflightCache.mu.TryLock() {
		return
	}
	defer preflightCache.mu.Unlock()
	document, err := loadPreflightCache(r.CachePath, process)
	if err != nil || !preflightCache.reader.Retired() {
		return // original I/O uncertainty is retained; no replacement slots
	}
	for key, item := range document.Entries {
		if !item.valid(now) || item.Scope.cacheKey(item.Host) != key {
			delete(document.Entries, key)
		}
	}
	key := r.Scope.cacheKey(host)
	if key == "" {
		return
	}
	if _, known := document.Entries[key]; !known && len(document.Entries) >= preflightCacheEntries {
		oldest := ""
		for candidate, item := range document.Entries {
			if oldest == "" || item.At.Before(document.Entries[oldest].At) || item.At.Equal(document.Entries[oldest].At) && candidate < oldest {
				oldest = candidate
			}
		}
		delete(document.Entries, oldest)
	}
	document.Entries[key] = entry
	raw, err := json.Marshal(document)
	if err == nil && len(raw) <= preflightCacheLimit {
		_ = preflightCache.writer.Write(r.CachePath, raw) // optional hint, never a required durable authority commit
	}
}
