package main

// Production implementation of the explicit GUI diagnostics; no Go test code.
// Results are observations, never forwarding/protection/transport admission.

import (
	"time"

	"github.com/snowden-system/windows/backend/render"
	"github.com/snowden-system/windows/backend/secretvault"
)

func testIDs() []struct{ id, desc string } {
	return []struct{ id, desc string }{
		{"udp-availability", "Ответ QUIC Version Negotiation от 8.8.8.8:443. Описывает только этот UDP endpoint через текущий маршрут ОС; отсутствие ответа не устанавливает фильтрацию."},
		{"hy2-reachability", "Ограниченный UDP-запрос к одному объявленному HY2 endpoint. Ответ не проверяет HY2 auth/obfs или VPN-путь; тишина оставляет результат неизвестным."},
		{"vless-config-present", "Привязки и формат credentials выбранных каналов на одном снимке vault. Это не проверка доступа к серверу."},
		{"elevated-process", "Наличие административного токена у процесса. Само по себе не разрешает TUN."},
		{"freeze-check", "Получение всех заданных объёмов через текущий туннель и дополнительный маршрут ОС. Причина сбоя и независимость маршрутов этим не устанавливаются."},
		{"core-wired", "Текущий reported CoreReady процесса. Это не самостоятельная проверка VPN-пути."},
	}
}

type diagnosticScope struct {
	task        *manualDiagnosticTask
	credentials bool
}

// App.mu held. Normal task completion cancels its ctx, so historical results
// use original done/source/path, not a replacement context or old health PASS.
func (scope diagnosticScope) currentLocked(a *App, current *DiagnosticContextView) bool {
	task := scope.task
	if task == nil || task.parent == nil || task.parent.Err() != nil || a.closing.Load() || task.resultRejected || task.cleanupFailed.Load() || scope.credentials && task.credentialsChanged.Load() ||
		!task.registration.currentLocked(a) || task.path != nil && !task.path.current.Load() || !sameDiagnosticContext(task.contextView, current) {
		return false
	}
	select {
	case <-task.done:
		return true
	default:
		return false
	}
}

func (a *App) ListTests() []TestResultView {
	a.mu.Lock()
	defer a.mu.Unlock()
	context := a.diagnosticContextLocked()
	out := make([]TestResultView, 0, len(testIDs()))
	for _, t := range testIDs() {
		view := TestResultView{ID: t.id, Detail: t.desc}
		if result, exists := a.tests[t.id]; exists {
			scope := a.tscope[t.id]
			if scope.task != nil {
				view.Context = scope.task.contextView
			}
			view.OK, view.Detail, view.Steps = result, a.tdetail[t.id], append([]ProbeStepView(nil), a.tsteps[t.id]...)
			if checked, ok := a.tlast[t.id]; ok {
				view.CheckedAt = checked.Format(time.RFC3339)
			}
			current := scope.currentLocked(a, context)
			view.ContextCurrent = &current
			if !current {
				view.OK = nil
				view.Detail = "Сохранённое наблюдение не подтверждено для текущего intent/пути или ещё не завершено. Запустите проверку заново."
				view.Steps = []ProbeStepView{{Name: "актуальность", Status: "unavailable", Detail: view.Detail}}
			}
		}
		out = append(out, view)
	}
	return out
}

func (a *App) RunTest(id string) (result TestResultView, resultErr error) {
	known := false
	for _, entry := range testIDs() {
		known = known || id == entry.id
	}
	if !known {
		return TestResultView{}, errSecretTestUnavailable // no unbounded arbitrary-ID cache/log
	}
	timeout, requests, cost, network := 10*time.Second, 0, int64(0), false
	switch id {
	case "udp-availability", "hy2-reachability":
		requests, cost, network = udpObservationAttempts, udpObservationAttempts*(udpObservationRequestBytes+udpObservationMaxReply+1), true
	case "freeze-check":
		timeout, network = 90*time.Second, true // Volume plan admits below, retaining release through the original owner
	}
	task, vault, err := a.beginManualDiagnostic(timeout, requests, cost, network)
	if err != nil {
		return TestResultView{}, errSecretTestUnavailable
	}
	outcome := &manualDiagnosticOutcome{app: a, task: task, credentials: id == "vless-config-present"}
	defer outcome.finishTest(&result, &resultErr)
	result, resultErr = a.runManualDiagnosticBody(task, vault, id)
	outcome.returned = true
	return
}

func (a *App) runManualDiagnosticBody(task *manualDiagnosticTask, vault *secretvault.Manager, id string) (TestResultView, error) {
	result := a.performManualDiagnostic(task, vault, id)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.diagnosticTask != task || a.vault != vault || task.currentError() != nil || !task.registration.currentLocked(a) ||
		id == "vless-config-present" && task.credentialsChanged.Load() {
		return TestResultView{}, errSecretTestUnavailable
	}
	now := time.Now()
	result.ID, result.CheckedAt = id, now.Format(time.RFC3339)
	result.Context = task.contextView // original, never retag a cached observation after a context change
	current := true                   // deferred finalizer rejects the whole reply if this context no longer matches
	result.ContextCurrent = &current
	if a.tscope == nil {
		a.tscope = make(map[string]diagnosticScope)
	}
	a.tests[id], a.tdetail[id] = result.OK, result.Detail
	a.tsteps[id], a.tlast[id] = append([]ProbeStepView(nil), result.Steps...), now
	a.tscope[id] = diagnosticScope{task: task, credentials: id == "vless-config-present"}
	return result, nil
}

func (a *App) performManualDiagnostic(task *manualDiagnosticTask, vault *secretvault.Manager, id string) TestResultView {
	var result TestResultView
	switch id {
	case "udp-availability":
		result.OK, result.Detail, result.Steps = observeQUICVersion(task, "8.8.8.8:443")
	case "hy2-reachability":
		result.OK, result.Detail, result.Steps = a.testHY2Reachability(task)
	case "vless-config-present":
		result.OK, result.Detail = secretsPresent(task, vault)
	case "elevated-process":
		ok := isAdmin()
		result.OK = &ok
		if ok {
			result.Detail = "У процесса есть административный токен. Допуск TUN остаётся отдельным закрытым gate."
		} else {
			result.Detail = "У процесса нет административного токена. SOCKS не требует elevation; работа сети этим не проверялась."
		}
	case "freeze-check":
		result.OK, result.Detail, result.Steps = a.testFreezeCheck(task)
	case "core-wired":
		a.mu.Lock()
		ok := a.stateSnapshotLocked().CoreReady
		a.mu.Unlock()
		result.OK, result.Detail = &ok, "Reported CoreReady текущего процесса; это не отдельная проверка защищённого пути."
	}
	return result
}

func (a *App) testHY2Reachability(task *manualDiagnosticTask) (*bool, string, []ProbeStepView) {
	a.mu.Lock()
	active, running := a.state.ActiveID, a.state.State == "running"
	a.mu.Unlock()
	snapshot, err := render.LoadDescriptorSnapshot()
	if err != nil {
		return diagnosticUnavailable("Дескрипторы недоступны; запасной адрес не используется.")
	}
	var selected *render.ChannelDescriptor
	for i := range snapshot.Channels {
		ch := &snapshot.Channels[i]
		if running && ch.ID == active && ch.Protocol != "hysteria2" {
			return diagnosticUnavailable("Reported активный канал не HY2. Эта UDP-проверка не описывает его путь.")
		}
		if !ch.Enabled || ch.Protocol != "hysteria2" || running && ch.ID != active {
			continue
		}
		if selected != nil {
			return diagnosticUnavailable("Назначение неоднозначно: нужен один подходящий HY2-дескриптор.")
		}
		selected = ch
	}
	if selected == nil || selected.OriginServer == "" || selected.Port == 0 {
		return diagnosticUnavailable("Единственный подходящий HY2 endpoint не получен; запасной адрес не используется.")
	}
	return observeQUICVersion(task, diagnosticUDPAddress(selected.OriginServer, selected.Port))
}

func diagnosticUnavailable(detail string) (*bool, string, []ProbeStepView) {
	return nil, detail, []ProbeStepView{{Name: "наблюдение", Status: "unavailable", Detail: detail}}
}

// App.mu held, before even an uncertain mutation. No callback/I/O or wait here.
func (a *App) invalidateCredentialDiagnosticLocked() {
	delete(a.tscope, "vless-config-present")
	if a.diagnosticTask != nil {
		a.diagnosticTask.credentialsChanged.Store(true)
	}
}

func secretsPresent(task *manualDiagnosticTask, vault *secretvault.Manager) (*bool, string) {
	profile, err := render.LoadDescriptorSnapshot()
	if err != nil {
		return nil, "Дескрипторы недоступны; привязки credentials не проверены."
	}
	channels := render.Selectable(profile.Channels)
	if len(channels) == 0 {
		return nil, "Нет выбранных допустимых каналов; пустой набор не означает наличие credentials."
	}
	snapshot, err := vault.PrepareImportSnapshot(nil)
	if err != nil {
		return nil, "Снимок vault недоступен; результат не установлен."
	}
	ok := render.ValidateCredentials(channels, snapshot) == nil
	if vault.WithPreparedSnapshot(snapshot, task.currentError) != nil {
		return nil, "Исходный снимок или запрос изменился; результат не установлен."
	}
	if !ok {
		return &ok, "Привязки или локальный формат credentials выбранных каналов не прошли штатный валидатор. Это не сетевой отказ."
	}
	return &ok, "Штатный валидатор подтвердил привязки и формат credentials выбранных каналов на одном снимке vault. Доступ к серверу не проверялся."
}
