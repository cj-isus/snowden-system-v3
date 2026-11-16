package main

// Типы контракта UI↔backend (PLAN.md §4.1). Зеркало
// windows/frontend/src/api/contract.ts. Если меняешь здесь — меняй и там.

import "github.com/snowden-system/windows/backend/core"

// DiagnosticContextView is a process-local presentation fence only. IDs are
// lossless strings, never authority, physical-network identity or health proof.
type DiagnosticContextView struct {
	Schema        int    `json:"schema"`
	Revision      string `json:"revision"` // presentation changes, not a durable/native receipt
	Intent        string `json:"intent"`
	Path          string `json:"path"`
	IntentCurrent bool   `json:"intentCurrent"`
	PathCurrent   bool   `json:"pathCurrent"`
	Closing       bool   `json:"closing"`
	Channel       string `json:"channel"` // reported App selector, not a native route receipt
	Mode          string `json:"mode"`
	Lifecycle     string `json:"lifecycle"`
	Ready         bool   `json:"ready"`
}

// Request registration only. Allowed does not assert worker/native readiness,
// completed cleanup, a committed operation or protected traffic admission.
type LifecycleActionsView struct {
	Schema                int    `json:"schema"`
	Intent                string `json:"intent"`
	Lifecycle             string `json:"lifecycle"`
	StartAllowed          bool   `json:"startAllowed"`
	StartBlockedBy        string `json:"startBlockedBy"`
	StopAllowed           bool   `json:"stopAllowed"`
	StopBlockedBy         string `json:"stopBlockedBy"`
	ExplicitStartRequired bool   `json:"explicitStartRequired"`
}

// AppState — фактическое состояние lifecycle.
type AppState struct {
	DiagnosticContext *DiagnosticContextView `json:"diagnosticContext,omitempty"`
	LifecycleActions  *LifecycleActionsView  `json:"lifecycleActions,omitempty"`
	State             string                 `json:"state"`         // stopped|starting|running|stopping|error
	BlockedReason     string                 `json:"blockedReason"` // no_validated_channel|probe_failed|config_invalid|null
	ActiveID          string                 `json:"activeId"`      // id активного канала или ""
	Active            *ChannelView           `json:"active"`        // nil = нет данных
	ProxyMode         string                 `json:"proxyMode"`     // socks|tun|""
	Probe             *ProbeReportView       `json:"probe"`         // nil = probe не запускался
	ProbeRunning      bool                   `json:"probeRunning"`
	ProbeLastAt       string                 `json:"probeLastAt"` // RFC3339 или ""
	Error             string                 `json:"error"`
	CoreReady         bool                   `json:"coreReady"`    // running readiness, never Start request availability
	CoreBlockMsg      string                 `json:"coreBlockMsg"` // пояснение для UI (build phase ядра)
	// V2-053: авто-повтор после fail-closed. NextRetryAt — RFC3339 (UTC)
	// времени следующей автоматической попытки запуска; "" = не запланировано.
	// RetryAttempt — номер этой попытки (1..); 0 = не запланировано.
	NextRetryAt  string `json:"nextRetryAt"`
	RetryAttempt int    `json:"retryAttempt"`
}

// ChannelView — карточка канала (FR-002, только метаданные).
type ChannelView struct {
	ID         string `json:"id"`
	Transport  string `json:"transport"`
	Server     string `json:"server"`
	Port       int    `json:"port"`
	Validation string `json:"validation"`
	Enabled    bool   `json:"enabled"`
}

// ProbeStepView — один шаг probe-отчёта.
type ProbeStepView struct {
	Name        string               `json:"name"`
	Status      string               `json:"status"` // pending|running|pass|fail|skipped|unavailable|leak
	Detail      string               `json:"detail"`
	Comparison  core.ProbeComparison `json:"comparison,omitzero"`
	Requirement string               `json:"requirement,omitempty"`
}

// ProbeReportView — отчёт probe (все обязательные шаги).
type ProbeReportView struct {
	OK    bool            `json:"ok"`
	Steps []ProbeStepView `json:"steps"`
	Scope core.ProbeScope `json:"scope,omitzero"`
}

// TestResultView — карточка теста (PLAN §4.1: факты, без fake).
// OK == nil означает «не запускался / результат недоступен».
type TestResultView struct {
	Context        *DiagnosticContextView `json:"context,omitempty"`
	ContextCurrent *bool                  `json:"contextCurrent,omitempty"` // as-of reply/read, never route authority or TTL
	ID             string                 `json:"id"`
	OK             *bool                  `json:"ok"`
	Detail         string                 `json:"detail"`
	CheckedAt      string                 `json:"checkedAt"`
	Steps          []ProbeStepView        `json:"steps"`
}
