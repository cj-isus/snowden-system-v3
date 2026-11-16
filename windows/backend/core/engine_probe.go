package core

import (
	"context"
	"encoding/hex"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
)

type engineForwardingPermit struct {
	instance                 uint64
	subject, selector, token string
	epoch                    uint64
}

type engineProtectedProbe struct {
	engine            *Engine
	subject, selector string
	request           *workerProbeRequest
	native            *nativeTUNObservationSource
	nativeRequired    bool
}

// Preserve request cancellation/deadline/HTTP trace values while providing the
// real box's constructor services. A background context loses SDK DNS/network
// dependencies; using the lifetime alone loses this bounded request's deadline.
type workerOutboundContext struct {
	context.Context
	services context.Context
}

func (ctx workerOutboundContext) Value(key any) any {
	if value := ctx.Context.Value(key); value != nil {
		return value
	}
	return ctx.services.Value(key)
}

func configurationSubject(raw []byte) (string, error) {
	digest, err := RuntimeConfigurationDigest(raw)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(digest[:]), nil
}

// NewEngineProbe captures only a diagnostic plan and the exact expected
// configuration. It grants no forwarding authority and does no network I/O.
// Actual proof can come only from this engine's original owned worker.
func NewEngineProbe(engine *Engine, raw []byte, selector string, settings ProtectedProbe) Probe {
	return prepareEngineProbe(engine, raw, selector, settings)
}

func prepareEngineProbe(engine *Engine, raw []byte, selector string, settings ProtectedProbe) engineProtectedProbe {
	subject, subjectErr := configurationSubject(raw)
	request, requestErr := newWorkerProbeRequest(settings)
	if subjectErr != nil || requestErr != nil || !validWorkerTag(selector) {
		return engineProtectedProbe{}
	}
	return engineProtectedProbe{engine: engine, subject: subject, selector: selector, request: request}
}

// An opaque original observation supplies a one-use diagnostic handoff only.
// Runtime/Egress values, public reports or booleans cannot replace its source.
// The compiled platform and final Manager forwarding gate remain separate.
func NewEngineProbeWithNativeObservation(engine *Engine, raw []byte, selector string, settings ProtectedProbe, observation NativeTUNObservation) Probe {
	probe := prepareEngineProbe(engine, raw, selector, settings)
	probe.native, probe.nativeRequired = observation.source, true
	return probe
}

func (probe engineProtectedProbe) Run(ctx context.Context) *ProbeReport {
	if probe.engine != nil {
		probe.engine.mu.Lock()
		if !probe.nativeRequired && probe.engine.nativePermit.busyLocked(probe.engine) {
			probe.engine.mu.Unlock()
			return workerProbeFailure() // a background probe cannot steal this native owner
		}
		probe.engine.clearForwardingPermitsLocked() // includes budget/config rejection before RPC
		probe.engine.mu.Unlock()
	}
	if probe.engine == nil || probe.request == nil || !workerSubject(probe.subject) {
		probe.native.burn()
		return workerProbeFailure()
	}
	if probe.nativeRequired && (!probe.request.TUN || probe.native == nil) {
		probe.native.burn()
		return workerProbeFailure()
	}
	operation, release, err := probe.request.reserve(ctx)
	if err != nil {
		probe.native.burn()
		return &ProbeReport{Results: []ProbeResult{{Name: "бюджет probe", Status: "unavailable", Detail: "protected diagnostic budget unavailable; path not measured"}}}
	}
	defer release()
	request := *probe.request // retained probe cannot mutate/reuse a previous nonce
	var report *ProbeReport
	if probe.nativeRequired {
		report, err = probe.engine.probeOwnedWorkerWithNative(operation, ctx, probe.native, probe.subject, probe.selector, &request)
	} else {
		report, err = probe.engine.probeOwnedWorker(operation, probe.subject, probe.selector, &request)
	}
	if err != nil {
		return workerProbeFailure(err)
	}
	return report
}

func (e *Engine) probeOwnedWorker(ctx context.Context, subject, selector string, request *workerProbeRequest) (*ProbeReport, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.probeOwnedWorkerLocked(ctx, subject, selector, request, nil)
}

func (e *Engine) probeOwnedWorkerWithNative(ctx, operation context.Context, source *nativeTUNObservationSource, subject, selector string, request *workerProbeRequest) (*ProbeReport, error) {
	ctx = withOriginalOperationSources(ctx, operation) // confirmation and consuming probe share original negative sources
	e.mu.Lock()
	defer e.mu.Unlock()
	nonce, err := e.confirmNativeObservationLocked(ctx, operation, source, subject, selector, request.ExpectedEgress)
	if err != nil {
		return nil, err
	}
	owned := *request
	owned.NativeNonce = nonce
	return e.probeOwnedWorkerLocked(ctx, subject, selector, &owned, source)
}

func (e *Engine) probeOwnedWorkerLocked(ctx context.Context, subject, selector string, request *workerProbeRequest, native *nativeTUNObservationSource) (*ProbeReport, error) {
	if native == nil && e.nativePermit.busyLocked(e) {
		return workerProbeFailure(), nil // also cover a native preparation after Run's first guard
	}
	e.clearForwardingPermitsLocked() // a failed/new claiming observation cannot reuse an old result
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	current, err := configurationSubject(e.rawJSON)
	if err != nil || current != subject || !e.isolated || e.process == nil || e.cleanupErr != nil {
		return nil, ErrForwardingNotAdmitted
	}
	instance := e.process.instance
	command := workerRequest{Action: "probe", Selector: "proxy", Tag: selector, Subject: subject, Probe: request}
	reply, err := e.process.request(ctx, command, time.Duration(request.TimeoutMillis)*time.Millisecond+workerControlTimeout)
	if err != nil {
		return nil, e.retireIsolatedFailureLocked(err)
	}
	if contextError(ctx) != nil || e.process == nil || e.process.instance != instance || e.process.exited() || reply.Code != "ok" || !validWorkerProbeReport(&command, reply) {
		return nil, e.retireIsolatedFailureLocked(errWorkerProtocol, contextError(ctx))
	}
	report := reply.Report.Clone()
	if request.TUN && request.NativeNonce != "" && reply.Admitted {
		return nil, e.retireIsolatedFailureLocked(errWorkerProtocol) // native claiming probe requires the original closed phase
	}
	if reply.NativeAdmissionToken != "" {
		if native == nil || native.engine != e || native.scope == nil || native.scope.process != e.process || native.scope.request.Nonce != request.NativeNonce ||
			native.runtime.Instance != instance || native.runtime.AdmissionEpoch != reply.AdmissionEpoch || contextError(native.operation) != nil || !probePassed(report) {
			return nil, e.retireIsolatedFailureLocked(errWorkerProtocol)
		}
		until := time.Now().Add(workerControlTimeout) // local busy bound only, never native freshness authority
		if originalUntil := native.completed.Add(nativeTUNReceiptLifetime); originalUntil.Before(until) {
			until = originalUntil
		}
		if !time.Now().Before(until) {
			return nil, e.retireIsolatedFailureLocked(ErrNativeScopeUnavailable)
		}
		e.nativePermit = &engineNativeTUNPermit{process: e.process, instance: instance, epoch: reply.AdmissionEpoch, subject: subject, selector: selector, token: reply.NativeAdmissionToken, source: native, busyUntil: until, diagnostic: report.Clone()}
		return pendingTUNActivation(report), nil // product/Manager still require compiled final platform commit
	}
	if request.TUN && !reply.Admitted {
		if reply.AdmissionToken != "" || probePassed(report) {
			return nil, e.retireIsolatedFailureLocked(errWorkerProtocol) // missing prepared-native receipt
		}
		return pendingTUNActivation(report), nil
	}
	if probePassed(report) {
		if reply.Admitted && reply.AdmissionToken != "" || !reply.Admitted && !workerSubject(reply.AdmissionToken) {
			return nil, e.retireIsolatedFailureLocked(errWorkerProtocol)
		}
		if request.TUN && reply.Admitted {
			return report, nil // existing phase observation never mints a parent permit
		}
		e.forwardingPermit = &engineForwardingPermit{instance: instance, subject: subject, selector: selector, epoch: reply.AdmissionEpoch, token: reply.AdmissionToken}
	} else if reply.AdmissionToken != "" {
		return nil, e.retireIsolatedFailureLocked(errWorkerProtocol)
	}
	return report, nil
}

// Only Manager can commit the private observation after its own fresh intent,
// trust lifetime, actual engine and complete lifecycle gates. No public report
// parameter or caller-supplied boolean is accepted. A failed attempt is consumed.
func (e *Engine) admitForwarding(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	permit := e.forwardingPermit
	e.clearForwardingPermitsLocked()
	if err := contextError(ctx); err != nil {
		return err
	}
	if permit == nil || !e.isolated || e.process == nil || e.cleanupErr != nil || e.process.instance != permit.instance {
		return ErrForwardingNotAdmitted
	}
	subject, err := configurationSubject(e.rawJSON)
	if err != nil || subject != permit.subject {
		return ErrForwardingNotAdmitted
	}
	command := workerRequest{Action: "admit", Selector: "proxy", Tag: permit.selector, Subject: permit.subject, AdmissionEpoch: permit.epoch, AdmissionToken: permit.token}
	reply, err := e.process.request(ctx, command, workerControlTimeout)
	if err != nil {
		return e.retireIsolatedFailureLocked(err)
	}
	if contextError(ctx) != nil || e.process == nil || e.process.instance != permit.instance || e.process.exited() || !validWorkerControlReply(&command, reply) {
		return e.retireIsolatedFailureLocked(ErrForwardingNotAdmitted, contextError(ctx))
	}
	return nil
}

// Caller holds the worker Engine mutex. This is the current real selector and
// normalized applied graph, not the request's assertions or a UI label.
func (e *Engine) forwardingSubjectLocked() (string, string, *admissionBox, bool, error) {
	guarded, ok := e.box.(*admissionBox)
	if !ok || guarded.admission == nil || e.cleanupErr != nil {
		return "", "", nil, false, ErrForwardingNotAdmitted
	}
	cfg, _, err := runtimeConfiguration(e.rawJSON)
	if err != nil {
		return "", "", nil, false, err
	}
	outbound, exists := e.box.Outbound().Outbound("proxy")
	selector, selectable := outbound.(selectorControl)
	if !exists || !selectable {
		return "", "", nil, false, ErrSelectorSwitchUnavailable
	}
	selected := selector.Now()
	for _, candidate := range cfg.Outbounds {
		if candidate.Tag == "proxy" && candidate.Default != selected {
			return "", "", nil, false, ErrForwardingNotAdmitted
		}
	}
	tunMode := false
	for _, inbound := range cfg.Inbounds {
		tunMode = tunMode || inbound.Type == "tun"
	}
	subject, err := configurationSubject(e.rawJSON)
	return subject, selected, guarded, tunMode, err
}

func (e *Engine) localForwardingState() (string, string, uint64, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	subject, selected, guarded, _, err := e.forwardingSubjectLocked()
	if err != nil {
		return "", "", 0, false, err
	}
	epoch, active := guarded.admission.state()
	return subject, selected, epoch, active, nil
}

func (e *Engine) runLocalWorkerProbe(ctx context.Context, expected, selected string, request *workerProbeRequest) (report *ProbeReport, resultEpoch uint64, resultToken string, resultErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var originalGate *forwardingAdmission
	var originalEpoch uint64
	defer func() {
		if originalGate == nil || originalEpoch == 0 || originalGate.localRouteFault() == nil {
			return
		}
		// Keep the original correlated negative fact even if HTTP failed,
		// cancellation raced it, or issuance later rejected the sealed gate.
		// Worker RPC errors otherwise discard the report; this existing finite
		// cleanup report carries no token/positive authority or channel blame.
		report = workerProbeFailure(ErrEngineCleanupFailed, reportLifecycleError(report), resultErr)
		resultEpoch, resultToken, resultErr = originalEpoch, "", nil
	}()
	if err := contextError(ctx); err != nil {
		return nil, 0, "", err
	}
	subject, current, guarded, tunMode, err := e.forwardingSubjectLocked()
	if err != nil || subject != expected || current != selected || request == nil || request.TUN != tunMode {
		return nil, 0, "", ErrForwardingNotAdmitted
	}
	settings, err := request.settings()
	if err != nil {
		return nil, 0, "", err
	}
	guarded.admission.mu.Lock()
	epochBefore := guarded.admission.epoch
	originalGate, originalEpoch = guarded.admission, epochBefore
	if guarded.admission.routeFault {
		guarded.admission.mu.Unlock()
		return workerProbeFailure(ErrEngineCleanupFailed), epochBefore, "", nil
	}
	if guarded.admission.tunScope != nil || tunMode && request.NativeNonce == "" && (guarded.admission.nativePendingCurrentLocked() || guarded.admission.nativeReceipt != nil && time.Now().Before(guarded.admission.nativeReceipt.until)) {
		guarded.admission.mu.Unlock()
		return workerProbeFailure(), epochBefore, "", nil // do not steal the live native owner/receipt
	}
	guarded.admission.clearPendingLocked() // latest claiming/ordinary probe burns old permits after no-steal guard
	admittedBefore := guarded.admission.open.Load() && !guarded.admission.closed
	var native *workerNativeTUNReceipt
	if request.NativeNonce != "" {
		native = guarded.admission.takeNativeLocked(request, subject, current, epochBefore)
	} else if tunMode && !admittedBefore {
		guarded.admission.mu.Unlock()
		return workerProbeFailure(), epochBefore, "", nil // periodic cannot establish native authority
	}
	if tunMode {
		if native == nil && (request.NativeNonce != "" || guarded.admission.subject != subject || guarded.admission.selector != current) {
			guarded.admission.mu.Unlock()
			return workerProbeFailure(), epochBefore, "", nil
		}
		settings.tunBinding = &workerTUNProbeBinding{admission: guarded.admission, subject: subject, selector: current, epoch: epochBefore, native: native, existing: request.NativeNonce == "" && admittedBefore}
	}
	guarded.admission.mu.Unlock()
	outbound, exists := e.box.Outbound().Outbound("proxy")
	protected, wrapped := outbound.(*admissionSelector)
	if !exists || !wrapped || protected.admission != guarded.admission || protected.ctx == nil {
		return nil, 0, "", ErrForwardingNotAdmitted
	}
	allowed := make(map[string]bool, len(settings.Targets)+len(settings.DNSProbeTargets))
	for _, endpoint := range append(append([]string(nil), settings.Targets...), settings.DNSProbeTargets...) {
		address, parseErr := url.Parse(endpoint)
		if parseErr != nil {
			return nil, 0, "", errWorkerProtocol
		}
		port := address.Port()
		if port == "" {
			port = "443"
		}
		allowed[net.JoinHostPort(strings.ToLower(address.Hostname()), port)] = true
	}
	settings.SOCKSAddr = ""
	var bootstrapStamp uint64
	if guarded.bootstrap != nil {
		bootstrapStamp = guarded.bootstrap.faultStamp()
	}
	settings.tunnelDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" {
			return nil, ErrForwardingNotAdmitted
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil || !allowed[net.JoinHostPort(strings.ToLower(host), port)] {
			return nil, ErrForwardingNotAdmitted
		}
		destination := M.ParseSocksaddr(address)
		dialCtx, metadata := adapter.ExtendContext(workerOutboundContext{Context: ctx, services: protected.ctx})
		metadata.Network, metadata.Destination = network, destination
		dialCtx = context.WithValue(dialCtx, initialBootstrapProbeKey{}, guarded.admission)
		return protected.DialContext(dialCtx, network, destination)
	}
	report = boundWorkerProbeReport(settings.Run(ctx))
	if guarded.bootstrap != nil {
		if err := guarded.bootstrap.localSince(bootstrapStamp); err != nil {
			// The original worker protocol discards reports on RPC error. Keep
			// this typed negative report, with no permit, so the parent retains
			// local/cleanup facts instead of treating it as channel failure.
			return workerProbeFailure(err), epochBefore, "", nil
		}
	}
	if err := contextError(ctx); err != nil {
		return report, epochBefore, "", err // original negative facts only; child discards all positive data
	}
	after, afterTag, afterBox, _, err := e.forwardingSubjectLocked()
	if err != nil || after != subject || afterTag != current || afterBox != guarded {
		return report, epochBefore, "", ErrForwardingNotAdmitted
	}
	epochAfter, admittedAfter := guarded.admission.state()
	if epochAfter != epochBefore || tunMode && admittedAfter != admittedBefore || contextError(ctx) != nil {
		return report, epochBefore, "", ErrForwardingNotAdmitted
	}
	if tunMode {
		if !admittedBefore && probePassed(report) {
			if err := settings.tunBinding.prepareForwarding(ctx); err != nil {
				return report, epochBefore, "", err
			}
		}
		return report, epochBefore, "", nil // separate emission only; periodic neither issues nor refreshes a native token
	}
	if !probePassed(report) {
		epoch, _ := guarded.admission.state()
		return report, epoch, "", nil
	}
	epoch, token, err := guarded.admission.issue(subject, current, epochBefore)
	return report, epoch, token, err
}

func (e *Engine) consumeLocalForwarding(ctx context.Context, expected, selected string, epoch uint64, token string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return err
	}
	subject, current, guarded, tunMode, err := e.forwardingSubjectLocked()
	// W4c must bind the original scoped OS actor to this same subject before
	// TUN can be admitted. The engine's outbound report alone is insufficient.
	if tunMode && guarded != nil {
		guarded.admission.mu.Lock()
		guarded.admission.clearPendingLocked() // ordinary admit cannot consume, retain or redeem native preparation
		guarded.admission.mu.Unlock()
	}
	if err != nil || tunMode || subject != expected || current != selected {
		return ErrForwardingNotAdmitted
	}
	return guarded.admission.consume(subject, current, epoch, token)
}

// The complete lifecycle report is evaluated by Manager before this call.
// Repeat freshness on both sides of the bounded control RPC; an ACK alone must
// not publish running after a canceled or expired intent. Failure is cleaned up
// by the existing Manager path, retaining the original process on uncertainty.
func commitEngineForwarding(ctx context.Context, engine *Engine, probe Probe) error {
	if err := validateLifecycleSource(ctx, probe); err != nil {
		return err
	}
	if engine == nil || engine.Status() != "running" {
		return ErrEngineWorkerUnavailable
	}
	if _, err := lifetimeRemaining(probe); err != nil {
		return err
	}
	if err := engine.admitForwarding(ctx); err != nil {
		return err
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if engine.Status() != "running" {
		return ErrEngineWorkerUnavailable
	}
	if _, err := lifetimeRemaining(probe); err != nil {
		return err
	}
	return validateLifecycleSource(ctx, probe)
}
