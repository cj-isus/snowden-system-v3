package main

import (
	"context"
	"encoding/hex"
	"errors"
	"time"
)

var (
	errPublicSourceUnavailable = errors.New("public source: application lifetime or original intent unavailable")
	errPublicSourceBusy        = errors.New("public source: original operation has not returned")
	errPublicSourceInterrupted = errors.New("public source: original handler interrupted; application recovery required")
)

// One original callback, not native/SDK retirement or trust authority.
// Preview/transfer flights watch intent cancellation; profile apply/enrolled export may
// intentionally retire the old profile and keep the App parent through that work.
type publicSourceFlight struct {
	ctx                 context.Context
	parent              context.Context
	registration        intentRegistration
	requestID           string
	cancel              context.CancelFunc
	watchDone           chan struct{}
	applying, uncertain bool
}

func (a *App) publicSourceBase() (context.Context, intentRegistration, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	registration, err := a.restartIntentLocked()
	if err != nil {
		return nil, intentRegistration{}, errPublicSourceUnavailable
	}
	return a.ctx, registration, nil
}

func (a *App) publicSourceCurrent(flight *publicSourceFlight) bool {
	return flight != nil && flight.ctx != nil && flight.ctx.Err() == nil && !a.closing.Load() &&
		(flight.applying || profileRequestCurrent(flight.ctx))
}

func (a *App) beginPublicSource(requestID string) (*publicSourceFlight, error) {
	return a.beginPublicSourceForDuration(requestID, 45*time.Second, true)
}

// Local transfer/export/file workflows need an actual App/intent lifetime,
// independently of physical network availability. They reuse the same slot and
// original watcher; public/policy previews retain their existing path capture.
func (a *App) withTransferSource(work func(context.Context) error) error {
	id, err := publicSourceToken()
	if err != nil {
		return errPublicSourceUnavailable
	}
	flight, err := a.beginPublicSourceForDuration("transfer-"+id, 90*time.Second, false)
	if err != nil {
		return err
	}
	return a.runPublicSourceFlight(flight, func() error { return work(flight.ctx) })
}

func (a *App) beginPublicSourceForDuration(requestID string, duration time.Duration, capturePath bool) (*publicSourceFlight, error) {
	if !validSourceRequestID(requestID) {
		return nil, errPublicSourceUnavailable
	}
	parent, registration, err := a.publicSourceBase()
	if err != nil {
		return nil, err
	}
	source := parent
	if capturePath {
		source, err = a.contextForIntent(registration)
		if err != nil {
			return nil, errPublicSourceUnavailable
		}
	}
	ctx, cancel := context.WithTimeout(source, duration)
	ctx = a.bindProfileRequest(ctx, parent, registration)
	flight := &publicSourceFlight{ctx: ctx, parent: parent, registration: registration, requestID: requestID, cancel: cancel, watchDone: make(chan struct{})}
	s := &a.publicProfiles
	s.mu.Lock()
	if s.active != nil || !a.publicSourceCurrent(flight) {
		failure := errPublicSourceBusy
		if s.active != nil && s.active.uncertain {
			failure = errPublicSourceInterrupted
		}
		s.mu.Unlock()
		cancel()
		return nil, failure
	}
	s.active, s.requestID, s.pending = flight, requestID, nil
	s.mu.Unlock()
	// Reserved before spawning. One passive watcher per original fetch/transfer, joined
	// before ordinary admission release; timeout/Cancel alone never frees active.
	go func() {
		defer close(flight.watchDone)
		select {
		case <-registration.source.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	return flight, nil
}

func (a *App) beginPublicSourceApply(token, purpose string) (*pendingPublicProfile, *publicSourceFlight, error) {
	if len(token) != 64 {
		return nil, nil, errPublicSourceUnavailable
	}
	decoded, err := hex.DecodeString(token)
	if err != nil || hex.EncodeToString(decoded) != token {
		return nil, nil, errPublicSourceUnavailable
	}
	parent, registration, err := a.publicSourceBase()
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	requestContext := a.bindProfileRequest(ctx, parent, registration)
	flight := &publicSourceFlight{ctx: requestContext, parent: parent, registration: registration, cancel: cancel, applying: true}
	s := &a.publicProfiles
	s.mu.Lock()
	pending := s.pending
	if s.active != nil || pending == nil || pending.token != token || pending.purpose != purpose {
		failure := errPublicSourceBusy
		if s.active != nil && s.active.uncertain {
			failure = errPublicSourceInterrupted
		}
		s.mu.Unlock()
		cancel()
		return nil, nil, failure
	}
	s.pending = nil // matching draft consumed once, including a stale source
	if pending.registration != registration || pending.parent == nil || pending.parent.Err() != nil ||
		(purpose == "pin_policy" && pending.pinPreview == nil || purpose != "pin_policy" && pending.preview == nil) ||
		!registration.source.Current() || !a.publicSourceCurrent(flight) {
		s.mu.Unlock()
		cancel()
		return nil, nil, errPublicSourceUnavailable
	}
	flight.requestID = s.requestID
	s.active = flight
	s.mu.Unlock()
	return pending, flight, nil
}

// An interrupted frame retains its exact slot even after cancellation/watcher
// join. An ordinary callback return frees only that pointer; neither is proof
// that native/SDK work retired or that a failed mutation left storage unchanged.
func (a *App) runPublicSourceFlight(flight *publicSourceFlight, work func() error) (finalErr error) {
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			finalErr = errPublicSourceInterrupted
		}
		if returned && !flight.applying && !a.publicSourceCurrent(flight) {
			finalErr = errors.Join(finalErr, context.Canceled)
		}
		flight.cancel()
		if flight.watchDone != nil {
			<-flight.watchDone
		}
		s := &a.publicProfiles
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.active != flight {
			finalErr = errors.Join(finalErr, errPublicSourceInterrupted)
			return
		}
		if finalErr != nil {
			s.pending = nil
		}
		if returned {
			s.active = nil
		} else {
			flight.uncertain = true
		}
	}()
	if !a.publicSourceCurrent(flight) {
		returned = true
		return context.Canceled
	}
	finalErr = work()
	returned = true
	return finalErr
}
