package repository

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

// PublisherHandler has no filesystem listing, redirect, network fallback or
// arbitrary path open. Construction does not create files or listen on a port.
// The two nonqueued admissions cover the whole response lifetime, including
// slow writes: owned decoded views cannot grow with an unbounded request queue.
type PublisherHandler struct {
	directory string
	admission chan struct{}
	cacheMu   sync.Mutex // owned view only; never held during disk/HTTP I/O
	cache     *servedRelease
	clock     metadata.ObservedClock
}

type servedRelease struct {
	head         publisherHead
	current      []byte
	files        map[string][]byte // private owned bytes, no methods expose mutable slices
	pinPolicySHA string
}

var errPublisherRootNotRetained = errors.New("repository: root not retained")
var errPublisherReleaseNotRetained = errors.New("repository: release not retained")

func NewPublisherHandler(directory string) (*PublisherHandler, error) {
	layout, err := publisherLayout(directory)
	if err != nil {
		return nil, err
	}
	return &PublisherHandler{directory: layout.root, admission: make(chan struct{}, 2)}, nil
}

func (h *PublisherHandler) load(ctx context.Context) (view *servedRelease, floor time.Time, finalErr error) {
	return h.loadSelection(ctx, "", 0)
}

func (h *PublisherHandler) loadSelection(ctx context.Context, requested string, rootVersion int64) (view *servedRelease, floor time.Time, finalErr error) {
	if ctx == nil {
		return nil, time.Time{}, errors.New("repository: read context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	if _, err := h.clock.Now(time.Time{}); err != nil {
		return nil, time.Time{}, err
	}
	store, release, err := openExistingPublisher(h.directory)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer func() { finalErr = errors.Join(finalErr, release()) }()
	source, err := store.read()
	if err != nil {
		return nil, time.Time{}, err
	}
	if source.state == nil || source.state.Published == nil {
		return nil, time.Time{}, errNoPublishedRelease
	}
	floor = publisherClockFloor(source.state)
	if _, err = h.clock.Now(floor); err != nil {
		return nil, time.Time{}, err
	}
	selected := source.state.Published
	current := requested == "" || requested == selected.ReleaseID
	if !current {
		selected = nil
		for i := range source.state.History {
			if source.state.History[i].ReleaseID == requested {
				selected = &source.state.History[i]
				break
			}
		}
		if selected == nil {
			return nil, time.Time{}, errPublisherReleaseNotRetained
		}
	}
	if rootVersion != 0 {
		raw, err := store.readIndexedRoot(ctx, source.state, rootVersion)
		if err != nil {
			return nil, time.Time{}, err
		}
		view = &servedRelease{head: *selected, files: map[string][]byte{"root": raw}}
	} else if current {
		h.cacheMu.Lock()
		view = h.cache
		h.cacheMu.Unlock()
	}
	if rootVersion == 0 && (view == nil || !bytes.Equal(view.current, source.raw[1]) || !current) {
		accepted, err := readAcceptedHeadView(ctx, store, *selected)
		if err != nil {
			return nil, time.Time{}, err
		}
		// Keep only the verified public file bytes; do not retain the potentially
		// 12MiB outer candidate frame in an HTTP cache or expose it as a route.
		view = &servedRelease{head: accepted.head, current: accepted.current, files: accepted.stage.files, pinPolicySHA: accepted.pinPolicySHA}
	}
	if err = ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	if err = store.fence(source); err != nil {
		return nil, time.Time{}, err
	}
	if err = ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	floor = publisherClockFloor(source.state)
	now, err := h.clock.Now(floor)
	if err != nil {
		return nil, time.Time{}, err
	}
	gate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		now, err := h.clock.Now(floor)
		if err != nil {
			return err
		}
		// Historical root expiration is evaluated by the client's sequential
		// TUF update. Never renew an expired profile/target/archive deadline.
		if rootVersion == 0 {
			if err = checkPublicationPinPolicy(source.state, view.pinPolicySHA); err != nil {
				return err
			}
			return checkPublicationTime(view.head.ValidUntil, floor, now)
		}
		return nil
	}
	if err = gate(); err != nil {
		return nil, time.Time{}, err
	}
	source, err = store.observeServingTime(ctx, source, now, gate)
	if err != nil {
		return nil, time.Time{}, err
	}
	floor = publisherClockFloor(source.state)
	if err = gate(); err != nil {
		return nil, time.Time{}, err
	}
	if rootVersion == 0 && current {
		h.cacheMu.Lock()
		h.cache = view // at most one current cache; archives are request-owned
		h.cacheMu.Unlock()
	}
	return view, floor, nil // lease released before any response/network write
}

func publisherHTTPError(w http.ResponseWriter, method string, status int, code string) {
	data := []byte("{\"error\":\"" + code + "\"}\n") // fixed local code, never OS/path errors
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	if method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (h *PublisherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		publisherHTTPError(w, r.Method, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	// Restrict before decoding/loading: no path cleaning, encoded separators,
	// percent aliases, query-driven downloads or redirects to a newer release.
	if r.URL == nil || r.URL.RawQuery != "" || strings.Contains(r.URL.EscapedPath(), "%") || len(r.URL.Path) > 400 {
		publisherHTTPError(w, r.Method, http.StatusBadRequest, "invalid_path")
		return
	}
	path := r.URL.Path
	var requestedRelease, relative string
	var rootVersion int64
	if strings.HasPrefix(path, "/metadata/") {
		name := strings.TrimPrefix(path, "/metadata/")
		number := strings.TrimSuffix(name, ".root.json")
		parsed, err := strconv.ParseInt(number, 10, 64)
		if err != nil || parsed < 1 || strconv.FormatInt(parsed, 10) != number || name != number+".root.json" {
			publisherHTTPError(w, r.Method, http.StatusNotFound, "not_found")
			return
		}
		rootVersion = parsed
	} else if path != "/current.json" {
		parts := strings.SplitN(strings.TrimPrefix(path, "/releases/"), "/", 2)
		if !strings.HasPrefix(path, "/releases/") || len(parts) != 2 || !canonicalHash(parts[0]) || !publisherfiles.RelativeJSON(parts[1]) || !(strings.HasPrefix(parts[1], "metadata/") || strings.HasPrefix(parts[1], "targets/") || strings.HasPrefix(parts[1], "clients/")) {
			publisherHTTPError(w, r.Method, http.StatusNotFound, "not_found")
			return
		}
		requestedRelease, relative = parts[0], parts[1]
	}
	if h == nil || h.admission == nil {
		publisherHTTPError(w, r.Method, http.StatusServiceUnavailable, "publisher_unavailable")
		return
	}
	select {
	case h.admission <- struct{}{}:
		defer func() { <-h.admission }()
	default:
		w.Header().Set("Retry-After", "1")
		publisherHTTPError(w, r.Method, http.StatusServiceUnavailable, "publisher_busy")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	view, floor, err := h.loadSelection(ctx, requestedRelease, rootVersion)
	if err != nil {
		if errors.Is(err, errPublisherReleaseNotRetained) || errors.Is(err, errPublicationPolicySuperseded) || requestedRelease != "" && (errors.Is(err, errPublicationExpired) || errors.Is(err, metadata.ErrPinPolicyPending)) {
			publisherHTTPError(w, r.Method, http.StatusGone, "release_retired")
			return
		}
		if errors.Is(err, errPublisherRootNotRetained) {
			publisherHTTPError(w, r.Method, http.StatusNotFound, "not_found")
			return
		}
		w.Header().Set("Retry-After", "1")
		publisherHTTPError(w, r.Method, http.StatusServiceUnavailable, "publisher_unavailable")
		return
	}
	w.Header().Set("X-Snowden-Release", view.head.ReleaseID)
	data := view.current
	if rootVersion != 0 {
		data = view.files["root"]
	} else if requestedRelease != "" {
		if requestedRelease != view.head.ReleaseID {
			// Explicit reselection/retry preserves a client's retained SDK prefix.
			// Never serve different bytes under the old generation's URL.
			publisherHTTPError(w, r.Method, http.StatusGone, "release_retired")
			return
		}
		var exists bool
		data, exists = view.files[relative]
		if !exists {
			publisherHTTPError(w, r.Method, http.StatusNotFound, "not_found")
			return
		}
	}
	if err = ctx.Err(); err != nil {
		publisherHTTPError(w, r.Method, http.StatusServiceUnavailable, "request_expired")
		return
	}
	now, clockErr := h.clock.Now(floor)
	if clockErr != nil || rootVersion == 0 && checkPublicationTime(view.head.ValidUntil, floor, now) != nil {
		publisherHTTPError(w, r.Method, http.StatusServiceUnavailable, "publication_unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("ETag", "\""+digest(data)+"\"")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data) // failure never accepts a release; clock may already be checkpointed
	}
}
