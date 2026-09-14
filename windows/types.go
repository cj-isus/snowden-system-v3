package main

// Типы контракта UI↔backend (PLAN.md §4.1). Зеркало
// windows/frontend/src/api/contract.ts. Если меняешь здесь — меняй и там.

// AppState — фактическое состояние lifecycle.
type AppState struct {
	State         string           `json:"state"`         // stopped|starting|running|stopping|error
	BlockedReason string           `json:"blockedReason"` // no_validated_channel|probe_failed|config_invalid|null
	ActiveID      string           `json:"activeId"`      // id активного канала или ""
	Active        *ChannelView     `json:"active"`        // nil = нет данных
	ProxyMode     string           `json:"proxyMode"`     // socks|tun|""
	Probe         *ProbeReportView `json:"probe"`         // nil = probe не запускался
	ProbeRunning  bool             `json:"probeRunning"`
	ProbeLastAt   string           `json:"probeLastAt"` // RFC3339 или ""
	Error         string           `json:"error"`
	CoreReady     bool             `json:"coreReady"`    // false = ядро ещё не подключено
	CoreBlockMsg  string           `json:"coreBlockMsg"` // пояснение для UI (build phase ядра)
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
	Name   string `json:"name"`
	Status string `json:"status"` // pending|running|pass|fail|skipped
	Detail string `json:"detail"`
}

// ProbeReportView — отчёт probe (все обязательные шаги).
type ProbeReportView struct {
	OK    bool            `json:"ok"`
	Steps []ProbeStepView `json:"steps"`
}

// TestResultView — карточка теста (PLAN §4.1: факты, без fake).
// OK == nil означает «не запускался / результат недоступен».
type TestResultView struct {
	ID        string          `json:"id"`
	OK        *bool           `json:"ok"`
	Detail    string          `json:"detail"`
	CheckedAt string          `json:"checkedAt"`
	Steps     []ProbeStepView `json:"steps"`
}
