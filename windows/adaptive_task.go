package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/netprobe"
	"github.com/snowden-system/windows/backend/render"
)

const maxAdaptiveTargets = 8

// One original optional batch, never a policy/health/forwarding authority.
type adaptiveBatch struct {
	app     *App
	task    *manualDiagnosticTask
	targets []string
	config  *render.ClientConfig
	subject appProbeSubject
	workers []*adaptiveSiteTask
	results []SiteStatus
}

type adaptiveSiteTask struct {
	batch     *adaptiveBatch
	domain    string
	mixed     string
	done      chan struct{}
	attempted bool // classifier entered; not proof of packets or a remote response
	returned  bool // immutable after done; ordinary classifier/SDK defer return only
	result    SiteStatus
}

func captureAdaptiveTargets() ([]string, error) {
	if len(adaptiveCheckTargets) < 1 || len(adaptiveCheckTargets) > maxAdaptiveTargets {
		return nil, errSecretTestUnavailable
	}
	targets := append([]string(nil), adaptiveCheckTargets...)
	seen := make(map[string]bool, len(targets))
	for _, domain := range targets {
		canonical, err := netprobe.CanonicalDNSHost(domain)
		if err != nil || canonical != domain || seen[domain] {
			return nil, errSecretTestUnavailable
		}
		seen[domain] = true
	}
	return targets, nil
}

func adaptiveCheckUnavailable() AdaptiveStatus {
	out := adaptiveStatusBase()
	out.Note = "Проверка не завершена: соединение изменилось, запрос отменён или достигнут лимит диагностик. Причина отказа сайта или канала не установлена. " + out.Note
	return out
}

func (a *App) runAdaptiveCheck() (out AdaptiveStatus) {
	targets, err := captureAdaptiveTargets()
	if err != nil {
		return adaptiveCheckUnavailable()
	}
	task, _, err := a.beginManualDiagnostic(30*time.Second, len(targets), int64(len(targets))*(16<<10), true)
	if err != nil {
		return adaptiveCheckUnavailable()
	}
	batch := &adaptiveBatch{app: a, task: task, targets: targets}
	var result error
	outcome := &manualDiagnosticOutcome{app: a, task: task}
	outcome.beforeRelease = func() error { return batch.publishHistory(&out) }
	defer outcome.finishAdaptive(&out, &result)
	out, result = batch.run()
	outcome.returned = true
	return
}

func (outcome *manualDiagnosticOutcome) finishAdaptive(report *AdaptiveStatus, result *error) {
	payload := recover()
	outcome.finish(payload, result, func() { *report = adaptiveCheckUnavailable() }, "проверка сайтов")
}

func (batch *adaptiveBatch) run() (AdaptiveStatus, error) {
	func() {
		runtime_.mu.Lock()
		defer runtime_.mu.Unlock()
		batch.config = runtime_.cc
	}()
	batch.subject = batch.app.appliedProbeSubject(batch.config)
	if batch.task.currentError() != nil || !batch.subjectCurrent(batch.task.ctx) {
		return adaptiveCheckUnavailable(), errSecretTestUnavailable
	}
	mixed := net.JoinHostPort(render.MixedListenAddress, strconv.Itoa(render.MixedListenPort))
	defer batch.join() // registered before the first original worker launch
	for _, domain := range batch.targets {
		worker := &adaptiveSiteTask{batch: batch, domain: domain, mixed: mixed, done: make(chan struct{})}
		batch.workers = append(batch.workers, worker) // retain original receipt before go
		go worker.run()
	}
	batch.join()
	if batch.task.currentError() != nil || !batch.subjectCurrent(batch.task.ctx) {
		return adaptiveCheckUnavailable(), errSecretTestUnavailable
	}
	batch.results = make([]SiteStatus, len(batch.workers))
	for index, worker := range batch.workers {
		if !worker.attempted {
			return adaptiveCheckUnavailable(), errSecretTestUnavailable
		}
		batch.results[index] = worker.result // same original done includes classifier's defers
	}
	out := adaptiveStatusBase()
	out.Sites = append([]SiteStatus(nil), batch.results...)
	out.Note = fmt.Sprintf("Получены отдельные HTTP-наблюдения для %d целей. ", len(batch.results)) + out.Note
	return out, nil
}

func (worker *adaptiveSiteTask) run() {
	defer close(worker.done)
	defer func() {
		if payload := recover(); payload != nil || !worker.returned {
			worker.batch.task.cleanupFailed.Store(true)
			worker.result = SiteStatus{} // never retain a partially completed positive classifier
		}
	}()
	worker.result = worker.observe()
	worker.returned = true
}

func (worker *adaptiveSiteTask) observe() SiteStatus {
	batch := worker.batch
	if batch.task.currentError() != nil || !batch.subjectCurrent(batch.task.ctx) {
		return SiteStatus{Domain: worker.domain, CheckedAt: time.Now().Format(time.RFC3339), Detail: "Исходная сессия недоступна; запрос не начат."}
	}
	worker.attempted = true
	return classifySite(batch.task, worker.mixed, worker.domain)
}

func (batch *adaptiveBatch) join() {
	for _, worker := range batch.workers {
		<-worker.done
		if !worker.returned {
			batch.task.cleanupFailed.Store(true)
		}
	}
}

func (batch *adaptiveBatch) subjectCurrent(ctx context.Context) bool {
	return batch.subject.intent == batch.task.registration.intent && batch.subject.intentSource == batch.task.registration.source &&
		batch.subject.path == batch.task.path && batch.subject.current(batch.app, ctx, batch.config)
}

// Network/body/watcher/Dial/Close stages have returned. The validation-only
// view retains original values across our own cancellation; no I/O uses it.
func (batch *adaptiveBatch) publishHistory(report *AdaptiveStatus) error {
	task := batch.task
	deadline, bounded := task.contextDeadline, task.contextBounded
	validation := context.WithoutCancel(task.ctx)
	current := func() bool {
		return task.originalParentCurrent() && bounded && time.Now().Before(deadline) &&
			!task.cleanupFailed.Load() && batch.subjectCurrent(validation)
	}
	if !current() {
		return errSecretTestUnavailable
	}
	err := persistAdaptive(batch.results)
	if errors.Is(err, filetxn.ErrWriteRetirement) || errors.Is(err, filetxn.ErrReadClose) {
		task.cleanupFailed.Store(true)
		return errSecretTestUnavailable
	}
	if !current() {
		return errSecretTestUnavailable
	}
	rememberAdaptive(batch.results)
	if err != nil {
		report.Note += " Сохранение истории на диск не подтверждено; выборка доступна в памяти этого процесса."
	}
	return nil
}
