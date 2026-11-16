package core

import "context"

const ProxyRecoveryRecordedValues = "restore_recorded_proxy_values"

// Presentation only; never PAC/registry bytes, paths, SID/PID or file hashes.
type ProxyRecoveryPreview struct {
	Token          string `json:"token"`
	Scope          string `json:"scope"`
	ExpiresAt      string `json:"expiresAt"`
	RecordedValues int    `json:"recordedValues"`
	PriorProcess   string `json:"priorProcess"`
	Resume         bool   `json:"resume"`
	ResumeReason   string `json:"resumeReason"`
}

type ProxyRecoveryConfirmation struct {
	Token string `json:"token"`
	Scope string `json:"scope"`
}

type ProxyRecoveryResult struct {
	Outcome        string `json:"outcome"`
	RestoredValues int    `json:"restoredValues"` // exact successful setters in this component operation
}

// The private retirement method excludes arbitrary public/boolean adapters.
type proxyRecoveryAPI interface {
	systemProxy
	PrepareProxyRecovery(context.Context) (*ProxyRecoveryPreview, error)
	ConfirmProxyRecovery(context.Context, ProxyRecoveryConfirmation) (*ProxyRecoveryResult, error)
	CancelProxyRecovery(string) error
	proxyPreparationRetired() bool
	proxyRecoveryResumable() bool
}

type managerProxyRecovery struct {
	proxy      systemProxy
	api        proxyRecoveryAPI
	generation uint64
	token      string
	finished   bool
}

type proxyRecoveryAction uint8

const (
	proxyRecoveryPrepare proxyRecoveryAction = iota
	proxyRecoveryResume
	proxyRecoveryConfirm
	proxyRecoveryCancel
)

// Only tracks this component's original pending interest, never Engine cleanup.
func (m *Manager) ProxyRecoveryPending() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxyRecovery != nil && !m.proxyRecovery.finished && m.proxyActive
}

// One actual callback uses the existing lifecycle slot. The UI's preview wait
// does not hold an active operation, so Stop can retire its original component.
func (m *Manager) runProxyRecovery(ctx context.Context, action proxyRecoveryAction, token string, body func(context.Context, *managerProxyRecovery) error) (result error) {
	if err := contextError(ctx); err != nil {
		return err
	}
	operationCtx, cancel := context.WithCancel(ctx)
	op := &lifecycleOperation{cancel: cancel, done: make(chan struct{})}
	m.mu.Lock()
	if action == proxyRecoveryCancel && m.proxyRecovery != nil && m.proxyRecovery.token == token && m.proxyRecovery.finished {
		m.mu.Unlock()
		cancel()
		return nil // the exact old component is retired, independent of a successor Engine
	}
	if m.active != nil || m.lifetimeWatch != nil || m.state != StateStopped && m.state != StateError ||
		m.engine != nil && m.engine.Status() != "stopped" || m.generation == ^uint64(0) {
		m.mu.Unlock()
		cancel()
		return ErrProxyRecoveryConsentUnavailable
	}
	session := m.proxyRecovery
	if action == proxyRecoveryPrepare {
		if session != nil && !session.finished && m.proxyActive {
			m.mu.Unlock()
			cancel()
			return ErrProxyRecoveryConsentUnavailable
		}
		api, ok := m.proxy.(proxyRecoveryAPI)
		if !ok {
			m.mu.Unlock()
			cancel()
			return ErrProxyRecoveryConsentUnavailable
		}
		session = &managerProxyRecovery{proxy: m.proxy, api: api}
		m.proxyRecovery = session // before the original native component can acquire anything
	} else if session == nil || session.proxy != m.proxy ||
		action == proxyRecoveryResume && (session.finished || !m.proxyActive) ||
		action != proxyRecoveryResume && session.token != token && !(action == proxyRecoveryCancel && token == "") ||
		action == proxyRecoveryConfirm && (session.finished || session.generation != m.generation) {
		m.mu.Unlock()
		cancel()
		return ErrProxyRecoveryConsentInvalid
	} else if action == proxyRecoveryCancel && (session.finished || !m.proxyActive) {
		session.finished = true
		m.mu.Unlock()
		cancel()
		return nil
	}
	previousState := m.state
	m.active, m.proxyActive = op, true // cleanup is registered before partial preparation
	m.generation++
	session.generation = m.generation
	m.mu.Unlock()
	defer func() {
		_ = recover() // never expose a callback/native panic payload
		if !op.bodyReturned {
			m.mu.Lock()
			m.state = StateError // keep the original active owner/done unreturned
			m.mu.Unlock()
			result = ErrProxyStateUncertain
			cancel()
			return
		}
		finished := false
		defer func() {
			if !finished {
				_ = recover()
				result = ErrProxyStateUncertain
			}
		}()
		retired := session.api.proxyPreparationRetired() // value-only, outside Manager.mu
		func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.active != op || m.proxyRecovery != session || m.proxy != session.proxy {
				result = ErrProxyStateUncertain
				return
			}
			if retired {
				m.proxyActive, session.finished = false, true
			}
			state := previousState
			if result != nil {
				state = StateError
			}
			if !m.finishLocked(op, state) {
				result = ErrProxyStateUncertain
			}
		}()
		cancel()
		finished = true
	}()
	return op.runBody(func() error { return body(operationCtx, session) })
}

func (m *Manager) PrepareProxyRecovery(ctx context.Context) (preview *ProxyRecoveryPreview, result error) {
	return m.prepareProxyRecovery(ctx, false)
}

// A fresh proposal on the exact retained component. This never reuses the old
// confirmation or releases its pending transaction to construct a new owner.
func (m *Manager) ResumeProxyRecovery(ctx context.Context) (*ProxyRecoveryPreview, error) {
	return m.prepareProxyRecovery(ctx, true)
}

func (m *Manager) prepareProxyRecovery(ctx context.Context, resume bool) (preview *ProxyRecoveryPreview, result error) {
	action := proxyRecoveryPrepare
	if resume {
		action = proxyRecoveryResume
	}
	result = m.runProxyRecovery(ctx, action, "", func(current context.Context, session *managerProxyRecovery) error {
		if resume && !session.api.proxyRecoveryResumable() {
			return ErrProxyRecoveryConsentUnavailable
		}
		value, err := session.api.PrepareProxyRecovery(current)
		if err != nil {
			return err // no implicit Restore/settings mutation on a refused preview
		}
		if value == nil || resume && !value.Resume {
			return ErrProxyStateUncertain
		}
		m.mu.Lock()
		session.token = value.Token
		m.mu.Unlock()
		if err := contextError(current); err != nil {
			if cleanupErr := session.api.CancelProxyRecovery(value.Token); cleanupErr != nil {
				return ErrProxyStateUncertain
			}
			return err
		}
		copy := *value
		preview = &copy
		return nil
	})
	if result != nil {
		preview = nil
	}
	return preview, result
}

func (m *Manager) ConfirmProxyRecovery(ctx context.Context, request ProxyRecoveryConfirmation) (value *ProxyRecoveryResult, result error) {
	result = m.runProxyRecovery(ctx, proxyRecoveryConfirm, request.Token, func(current context.Context, session *managerProxyRecovery) error {
		var err error
		value, err = session.api.ConfirmProxyRecovery(current, request)
		if err == nil && value != nil && value.Outcome == proxyForeignPreserved {
			m.mu.Lock()
			m.proxyConflict = true // negative only; never auto-connect after preserved foreign settings
			m.proxyRestoreOutcome = proxyForeignPreserved
			m.mu.Unlock()
		}
		return err
	})
	if result != nil {
		value = nil
	}
	return value, result
}

// Negative cleanup remains usable after the positive intent was revoked. An
// empty token addresses only this Manager's retained grant after a failed preview;
// it never authorizes a Restore/settings write or a successor runtime.
func (m *Manager) CancelProxyRecovery(token string) error {
	return m.runProxyRecovery(context.Background(), proxyRecoveryCancel, token, func(_ context.Context, session *managerProxyRecovery) error {
		if session.token == "" {
			if session.api.proxyPreparationRetired() {
				return nil
			}
			return ErrProxyStateUncertain
		}
		return session.api.CancelProxyRecovery(session.token)
	})
}
