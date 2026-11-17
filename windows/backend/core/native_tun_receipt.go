package core

import (
	"context"
	"net/netip"
	"sync"
	"time"
)

// Local process clocks bound this diagnostic fact. Confirmation/consumption
// never extend it. This is not the short forwarding-token lifetime or a grant.
const nativeTUNReceiptLifetime = 30 * time.Second

type workerNativeTUNReceipt struct {
	subject, selector, nonce string
	epoch                    uint64
	until                    time.Time
	confirmed                bool
	prepared                 bool // one-way issuance burn; guarded by original admission.mu
	egress                   netip.Addr
}

func nativeReportedAddress(value string) (netip.Addr, bool) {
	if len(value) == 0 || len(value) > 64 {
		return netip.Addr{}, false
	}
	address, err := netip.ParseAddr(value)
	return address.Unmap(), err == nil && address.Zone() == "" && address.Unmap().Is4()
}

func (receipt *workerNativeTUNReceipt) current(subject, selector string, epoch uint64, nonce string) bool {
	return receipt != nil && workerSubject(nonce) && receipt.nonce == nonce && receipt.subject == subject && receipt.selector == selector && receipt.epoch == epoch && time.Now().Before(receipt.until)
}

// This opaque source is created only after original helper retirement and HTTP
// reparse. Copies share one burn receipt. Runtime/Egress getters cannot create
// one; no public report, raw nonce or caller boolean supplies this source.
type nativeTUNObservationSource struct {
	mu        sync.Mutex
	used      bool
	engine    *Engine
	scope     *engineTUNScope
	operation *nativeTUNOperation // immutable captured metadata; never a raw callback context under source.mu
	runtime   RuntimeIdentity
	egress    string
	completed time.Time
}

func (source *nativeTUNObservationSource) burn() {
	if source != nil {
		source.mu.Lock()
		source.used = true
		source.mu.Unlock()
	}
}

// Caller holds this Engine.mu, then source.mu. A wrong engine, canceled/changed
// operation, old scope/process/epoch or rejected comparison burns the attempt.
func (source *nativeTUNObservationSource) claimLocked(engine *Engine, operation *nativeTUNOperation, subject, selector, expected string) (string, string, uint64, error) {
	if source == nil {
		return "", "", 0, ErrNativeScopeUnavailable
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.used {
		return "", "", 0, ErrNativeScopeUnavailable
	}
	source.used = true
	actual, actualOK := nativeReportedAddress(source.egress)
	wanted, wantedOK := nativeReportedAddress(expected)
	if source.engine != engine || engine == nil || operation == nil || source.operation == nil || contextError(operation) != nil || contextError(source.operation) != nil || operation.Done() == nil || operation.Done() != source.operation.Done() ||
		!engine.isolated || engine.cleanupErr != nil || engine.process == nil || engine.process.exited() || engine.tunScope != nil || source.scope == nil ||
		!source.scope.closed || source.scope.retired || source.scope.process != engine.process || source.scope.instance != engine.process.instance || source.runtime.Instance != engine.process.instance || source.runtime.ForwardingAdmitted ||
		source.scope.request.Subject != subject || source.scope.request.Selector != selector || source.scope.request.Epoch != source.runtime.AdmissionEpoch ||
		!actualOK || !wantedOK || actual != wanted || source.completed.IsZero() || time.Now().Before(source.completed) || !time.Now().Before(source.completed.Add(nativeTUNReceiptLifetime)) {
		return "", "", 0, ErrNativeScopeUnavailable
	}
	return source.scope.request.Nonce, actual.String(), source.runtime.AdmissionEpoch, nil
}

// Caller retains Engine.mu across confirmation AND its consuming worker probe.
func (engine *Engine) confirmNativeObservationLocked(ctx *workerRPCContext, operation *nativeTUNOperation, source *nativeTUNObservationSource, subject, selector, expected string) (string, error) {
	engine.clearForwardingPermitsLocked()
	if ctx == nil || ctx.Context == nil || contextError(ctx) != nil {
		source.burn()
		return "", ErrNativeScopeUnavailable
	}
	current, err := configurationSubject(engine.rawJSON)
	if err != nil || current != subject {
		source.burn()
		return "", ErrNativeScopeUnavailable
	}
	nonce, egress, epoch, err := source.claimLocked(engine, operation, subject, selector, expected)
	if err != nil {
		return "", err
	}
	process := engine.process
	command := workerRequest{Action: "tun_native_confirm", Selector: "proxy", Subject: subject, Tag: selector, AdmissionEpoch: epoch, ScopeNonce: nonce, NativeEgress: egress}
	reply, err := process.request(ctx, command, workerControlTimeout)
	if err != nil || contextError(ctx) != nil || contextError(operation) != nil || engine.process != process || process.exited() || !validWorkerControlReply(&command, reply) {
		return "", engine.retireIsolatedFailureLocked(ErrNativeScopeUnavailable, err)
	}
	return nonce, nil
}

// Only the original parent can confirm HTTP after scope-close, filter removal
// and full helper retirement. The worker itself must already hold its real
// selected-flow/I-O receipt. This command does not issue a forwarding token.
func (engine *Engine) confirmLocalNative(ctx context.Context, subject, selector string, epoch uint64, nonce, egress string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if contextError(ctx) != nil {
		return ErrNativeScopeUnavailable
	}
	actual, selected, guarded, tunMode, err := engine.forwardingSubjectLocked()
	if err != nil || !tunMode || actual != subject || selected != selector {
		return ErrNativeScopeUnavailable
	}
	gate := guarded.admission
	gate.mu.Lock()
	defer gate.mu.Unlock()
	address, ok := nativeReportedAddress(egress)
	receipt := gate.nativeReceipt
	if gate.closed || gate.open.Load() || gate.tunScope != nil || gate.epoch != epoch || !receipt.current(subject, selector, epoch, nonce) || receipt.confirmed || !ok {
		gate.nativeReceipt = nil
		return ErrNativeScopeUnavailable
	}
	receipt.confirmed, receipt.egress = true, address // no deadline extension
	if contextError(ctx) != nil {
		gate.nativeReceipt = nil
		return ErrNativeScopeUnavailable
	}
	return nil
}

// Caller holds admission.mu. Every claiming native probe consumes/discards the
// previous fact before outcome. Plain probes cannot steal a pending handoff.
func (gate *forwardingAdmission) takeNativeLocked(request *workerProbeRequest, subject, selector string, epoch uint64) *workerNativeTUNReceipt {
	receipt := gate.nativeReceipt
	gate.nativeReceipt = nil
	wanted, ok := nativeReportedAddress(request.ExpectedEgress)
	if !request.TUN || !receipt.current(subject, selector, epoch, request.NativeNonce) || !receipt.confirmed || !ok || wanted != receipt.egress || gate.closed || gate.open.Load() || gate.tunScope != nil || gate.epoch != epoch {
		return nil
	}
	return receipt
}

func nativeTUNReceiptResult(receipt *workerNativeTUNReceipt) ProbeResult {
	result := ProbeResult{Name: "Native TUN flow", Status: "unavailable", Detail: "Результат отдельной проверки TUN отсутствует, устарел или уже использован. Общий допуск закрыт."}
	if receipt != nil && receipt.confirmed && time.Now().Before(receipt.until) {
		result.Passed, result.Status = true, "pass"
		result.Detail = "Отдельный IPv4-запрос подтверждён для выбранного выхода. Проверочное соединение завершено; общий допуск ещё закрыт."
	}
	return result
}
