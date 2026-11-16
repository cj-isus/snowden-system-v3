package main

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"sync/atomic"

	"github.com/snowden-system/windows/backend/core"
)

type ProxyRecoveryPrepareRequest struct {
	RequestID         string `json:"requestId"`
	Intent            string `json:"intent"`
	PreviousRequestID string `json:"previousRequestId"` // fresh proposal on the exact retained Manager
}

type ProxyRecoveryConfirmRequest struct {
	RequestID string `json:"requestId"`
	Intent    string `json:"intent"`
	Token     string `json:"token"`
	Scope     string `json:"scope"`
}

type ProxyRecoveryPreviewView struct {
	Schema    int                        `json:"schema"`
	RequestID string                     `json:"requestId"`
	Intent    string                     `json:"intent"`
	Preview   *core.ProxyRecoveryPreview `json:"preview"`
}

type ProxyRecoveryResultView struct {
	Schema    int                       `json:"schema"`
	RequestID string                    `json:"requestId"`
	Intent    string                    `json:"intent"`
	Result    *core.ProxyRecoveryResult `json:"result"`
}

// Component scope only. A retired unused grant may leave the original native
// transaction held; neither outcome describes Engine/WFP or whole-app cleanup.
type ProxyRecoveryCancellationView struct {
	Schema         int    `json:"schema"`
	RequestID      string `json:"requestId"`
	OwnerRequestID string `json:"ownerRequestId"`
	Outcome        string `json:"outcome"` // released | transaction_held | blocked
}

type ProxyRecoveryCancelRequest struct {
	RequestID         string `json:"requestId"`
	PreviousRequestID string `json:"previousRequestId"`
}

type appProxyRecovery struct {
	requestID                        string
	registration                     intentRegistration
	manager                          *core.Manager
	engine                           *core.Engine
	preview                          *core.ProxyRecoveryPreview
	current                          atomic.Bool // local maintenance interest, never physical-path/forwarding proof
	prepareDone, confirmDone         chan struct{}
	prepareEntered, confirmEntered   bool
	prepareReturned, confirmReturned bool
	confirmStarted, cancelStarted    bool
	cancelReturned, cancelRequested  bool
	cancelInFlight                   bool
	finished                         bool
}

func validProxyRecoveryID(text string) bool {
	if len(text) != 32 {
		return false
	}
	raw, err := hex.DecodeString(text)
	return err == nil && hex.EncodeToString(raw) == text
}

func proxyRecoveryUIError(err error) error {
	switch {
	case errors.Is(err, core.ErrProxyRecoveryConsentExpired):
		return errors.New("Подтверждение истекло или уже использовано. Подготовьте восстановление заново.")
	case errors.Is(err, core.ErrProxyRecoveryConsentInvalid), errors.Is(err, context.Canceled):
		return errors.New("Запрос восстановления устарел или отменён. Настройки не сбрасываются автоматически.")
	case errors.Is(err, core.ErrProxyPriorOwnerRunning):
		return errors.New("Прежний экземпляр Snowden ещё работает. Закройте его перед восстановлением.")
	case errors.Is(err, core.ErrProxyStateUncertain):
		return errors.New("Завершение восстановления или освобождение его ресурсов не подтверждено. Повторная операция заблокирована; используйте остановку VPN.")
	default:
		return errors.New("Восстановление сейчас недоступно: нужны сохранённые изменения Snowden и свободное управление прокси. При необходимости сначала остановите VPN.")
	}
}

// App.mu held; this grants only local maintenance interest, including intent0.
func (owner *appProxyRecovery) currentLocked(app *App) bool {
	return app.proxyRecovery == owner && owner.registration.currentLocked(app) && owner.current.Load() && !owner.cancelRequested &&
		app.ctx != nil && app.ctx.Err() == nil && !app.closing.Load() && app.restartCommit == nil && app.releaseCommit == nil &&
		!app.installedImageChanged && app.startupErr == nil && !app.profileTransition &&
		(app.state.State == "stopped" || app.state.State == "error")
}

// operationMu held. The existing source contract is used with this actual local
// request's negative lifetime; no network-health or Engine admission is created.
func (a *App) proxyRecoveryContext(owner *appProxyRecovery) (context.Context, context.CancelFunc, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !owner.currentLocked(a) {
		return nil, nil, context.Canceled
	}
	ctx, cancel := context.WithCancel(a.ctx)
	operation := core.NewOperationSource()
	ctx = core.WithOperationSources(ctx, core.OperationSources{Intent: owner.registration.source, Operation: operation, Path: &owner.current, Closing: &a.closing})
	retire := func() { operation.Revoke(); cancel() }
	a.operationCancel = retire
	return ctx, retire, nil
}

// operationMu held. A profileless actor is installed in the actual runtime so
// ordinary Stop/quit/replacement retains and retires it; no second Engine exists.
func (a *App) proxyRecoveryRuntime(owner *appProxyRecovery, prepare bool) error {
	runtime_.mu.Lock()
	manager, engine := runtime_.manager, runtime_.engine
	if prepare && manager == nil && engine == nil {
		manager = core.NewManager()
		runtime_.manager = manager
	}
	runtime_.mu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	if !owner.currentLocked(a) || manager == nil || !prepare && (owner.manager != manager || owner.engine != engine) {
		return context.Canceled
	}
	if prepare {
		owner.manager, owner.engine = manager, engine
	}
	return nil
}

func (a *App) PrepareProxyRecovery(request ProxyRecoveryPrepareRequest) (view *ProxyRecoveryPreviewView, result error) {
	resume := request.PreviousRequestID != ""
	if !validProxyRecoveryID(request.RequestID) || resume && (!validProxyRecoveryID(request.PreviousRequestID) || request.PreviousRequestID == request.RequestID) {
		return nil, proxyRecoveryUIError(core.ErrProxyRecoveryConsentInvalid)
	}
	a.mu.Lock()
	previous := a.proxyRecovery
	if request.RequestID == a.proxyRecoveryCanceledID || request.Intent != strconv.FormatUint(a.operationID, 10) ||
		previous != nil && previous.requestID == request.RequestID ||
		previous != nil && (!previous.prepareReturned || previous.confirmStarted && !previous.confirmReturned || previous.cancelStarted && !previous.cancelReturned) ||
		!resume && previous != nil && !previous.finished ||
		resume && (previous == nil || previous.requestID != request.PreviousRequestID || previous.finished || previous.manager == nil) {
		a.mu.Unlock()
		return nil, proxyRecoveryUIError(core.ErrProxyRecoveryConsentInvalid)
	}
	owner := &appProxyRecovery{requestID: request.RequestID, registration: a.intentRegistrationLocked(), prepareDone: make(chan struct{})}
	if resume {
		owner.manager, owner.engine = previous.manager, previous.engine
		previous.current.Store(false) // actual old callbacks returned; native custody stays in the same Manager
	}
	owner.current.Store(true)
	a.proxyRecovery = owner         // before queue waits or any native callback
	a.cancelFailClosedRetryLocked() // maintenance never triggers an automatic reconnect
	a.mu.Unlock()
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			view, result = nil, proxyRecoveryUIError(core.ErrProxyStateUncertain)
		}
		a.mu.Lock()
		owner.prepareReturned = returned
		close(owner.prepareDone)
		a.mu.Unlock()
	}()
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	owner.prepareEntered = true
	a.mu.Unlock()
	ctx, cancel, err := a.proxyRecoveryContext(owner)
	if err == nil {
		defer cancel()
		err = a.proxyRecoveryRuntime(owner, !resume)
	}
	var preview *core.ProxyRecoveryPreview
	if err == nil {
		if resume {
			preview, err = owner.manager.ResumeProxyRecovery(ctx)
		} else {
			preview, err = owner.manager.PrepareProxyRecovery(ctx)
		}
	}
	a.mu.Lock()
	owner.preview = preview // retain late/stale token for its original negative cleanup
	current := owner.currentLocked(a)
	a.mu.Unlock()
	if preview != nil && (!current || err != nil) {
		if cancelErr := owner.manager.CancelProxyRecovery(preview.Token); cancelErr != nil {
			err = core.ErrProxyStateUncertain
		} else if err == nil {
			err = context.Canceled
		}
	}
	pending := owner.manager != nil && owner.manager.ProxyRecoveryPending()
	a.mu.Lock()
	if !pending {
		owner.finished = true
		owner.current.Store(false)
	}
	a.mu.Unlock()
	returned = true
	if err != nil || !current || preview == nil {
		return nil, proxyRecoveryUIError(err)
	}
	return &ProxyRecoveryPreviewView{Schema: 1, RequestID: owner.requestID, Intent: request.Intent, Preview: preview}, nil
}

func (a *App) ConfirmProxyRecovery(request ProxyRecoveryConfirmRequest) (view *ProxyRecoveryResultView, result error) {
	a.mu.Lock()
	owner := a.proxyRecovery
	if owner == nil || owner.requestID != request.RequestID || owner.preview == nil || !owner.prepareReturned || owner.confirmStarted ||
		request.Intent != strconv.FormatUint(owner.registration.intent, 10) || request.Token != owner.preview.Token || request.Scope != core.ProxyRecoveryRecordedValues {
		a.mu.Unlock()
		return nil, proxyRecoveryUIError(core.ErrProxyRecoveryConsentInvalid)
	}
	owner.confirmStarted = true // one attempt before waiting for the lifecycle slot
	owner.confirmDone = make(chan struct{})
	a.mu.Unlock()
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			view, result = nil, proxyRecoveryUIError(core.ErrProxyStateUncertain)
		}
		a.mu.Lock()
		owner.confirmReturned = returned
		close(owner.confirmDone)
		a.mu.Unlock()
	}()
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	owner.confirmEntered = true
	a.mu.Unlock()
	ctx, cancel, err := a.proxyRecoveryContext(owner)
	if err == nil {
		defer cancel()
		err = a.proxyRecoveryRuntime(owner, false)
	}
	var value *core.ProxyRecoveryResult
	if err == nil {
		value, err = owner.manager.ConfirmProxyRecovery(ctx, core.ProxyRecoveryConfirmation{Token: request.Token, Scope: request.Scope})
	} else if owner.manager != nil {
		if cleanupErr := owner.manager.CancelProxyRecovery(request.Token); cleanupErr != nil {
			err = core.ErrProxyStateUncertain
		}
	}
	pending := owner.manager != nil && owner.manager.ProxyRecoveryPending()
	a.mu.Lock()
	current := owner.currentLocked(a)
	if !pending {
		owner.finished = true
		owner.current.Store(false)
	}
	if value != nil && value.Outcome == "foreign_preserved" {
		a.proxyConflict = true // negative only; leave Engine/WFP/protection/error state intact
		a.cancelFailClosedRetryLocked()
	}
	a.mu.Unlock()
	returned = true
	if err != nil || value == nil {
		return nil, proxyRecoveryUIError(err)
	}
	if !current {
		return nil, proxyRecoveryUIError(context.Canceled)
	}
	return &ProxyRecoveryResultView{Schema: 1, RequestID: owner.requestID, Intent: request.Intent, Result: value}, nil
}

// Revoke interest before waiting for a callback's actual return. Retire only the
// originally captured Manager; a successor runtime never substitutes its owner.
func (a *App) CancelProxyRecovery(request ProxyRecoveryCancelRequest) (view *ProxyRecoveryCancellationView, result error) {
	requestID, ownerID := request.RequestID, request.RequestID
	cancellation := func(outcome string) *ProxyRecoveryCancellationView {
		return &ProxyRecoveryCancellationView{Schema: 1, RequestID: requestID, OwnerRequestID: ownerID, Outcome: outcome}
	}
	if !validProxyRecoveryID(requestID) || request.PreviousRequestID != "" && (!validProxyRecoveryID(request.PreviousRequestID) || request.PreviousRequestID == requestID) {
		return nil, proxyRecoveryUIError(core.ErrProxyRecoveryConsentInvalid)
	}
	a.mu.Lock()
	owner := a.proxyRecovery
	if owner != nil && owner.requestID != requestID && owner.requestID == request.PreviousRequestID {
		// The new proposal may have been refused before registration. Tombstone
		// its ID before addressing only its explicitly named original request.
		a.proxyRecoveryCanceledID = requestID
		ownerID = owner.requestID
	}
	if owner == nil || owner.requestID != ownerID {
		if owner != nil && !owner.finished {
			a.mu.Unlock()
			return nil, proxyRecoveryUIError(core.ErrProxyRecoveryConsentInvalid)
		}
		a.proxyRecoveryCanceledID = requestID // cancellation can precede its queued Prepare RPC
		a.mu.Unlock()
		return cancellation("released"), nil // tombstone only; no native component was dispatched for this ID
	}
	if owner.cancelInFlight {
		a.mu.Unlock()
		return nil, proxyRecoveryUIError(core.ErrProxyRecoveryConsentUnavailable)
	}
	owner.cancelRequested, owner.cancelStarted, owner.cancelInFlight, owner.cancelReturned = true, true, true, false
	owner.current.Store(false)
	if owner.finished && owner.prepareReturned && (!owner.confirmStarted || owner.confirmReturned) {
		owner.cancelReturned = true
		owner.cancelInFlight = false
		a.mu.Unlock()
		return cancellation("released"), nil // no original manual callback/resource remains to retire
	}
	a.mu.Unlock()
	returned := false
	defer func() {
		if payload := recover(); payload != nil || !returned {
			view, result = nil, proxyRecoveryUIError(core.ErrProxyStateUncertain)
		}
		a.mu.Lock()
		owner.cancelReturned = returned
		owner.cancelInFlight = !returned // an abnormal original cancellation cannot be replaced
		a.mu.Unlock()
	}()
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	a.mu.Lock()
	var prepareDone, confirmDone chan struct{}
	if owner.prepareEntered && !owner.prepareReturned {
		prepareDone = owner.prepareDone
	}
	if owner.confirmEntered && !owner.confirmReturned {
		confirmDone = owner.confirmDone
	}
	a.mu.Unlock()
	// Owning operationMu proves entered predecessors have released it. Their
	// remaining defers need only App.mu; queued successors are never waited here.
	if prepareDone != nil {
		<-prepareDone
	}
	if confirmDone != nil {
		<-confirmDone
	}
	a.mu.Lock()
	ordinary := (!owner.prepareEntered || owner.prepareReturned) && (!owner.confirmEntered || owner.confirmReturned)
	finished := owner.finished
	a.mu.Unlock()
	if !ordinary {
		returned = true
		return nil, proxyRecoveryUIError(core.ErrProxyStateUncertain)
	}
	if finished {
		returned = true
		return cancellation("released"), nil
	}
	var err error
	if owner.manager != nil && owner.manager.ProxyRecoveryPending() {
		token := ""
		if owner.preview != nil {
			token = owner.preview.Token
		}
		err = owner.manager.CancelProxyRecovery(token)
	}
	pending := owner.manager != nil && owner.manager.ProxyRecoveryPending()
	a.mu.Lock()
	owner.finished = err == nil && !pending
	a.mu.Unlock()
	returned = true
	if err != nil {
		return cancellation("blocked"), nil // exact request identity only; no resource-retirement assertion
	}
	if pending {
		return cancellation("transaction_held"), nil // unused grant retired; original native transaction still owned
	}
	return cancellation("released"), nil
}
