package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/update"
)

// DispatchEngineWorker must run before Wails, update housekeeping, timers and
// legacy helper dispatch. This mode never manages proxy/WFP or emits UI state.
// It uses the caller's token; it is neither a service nor an elevation channel.
func EngineWorkerInvocation(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-engine-worker") {
			return true
		}
	}
	return false
}

func DispatchEngineWorker(args []string) (handled bool, exitCode int) {
	if !EngineWorkerInvocation(args) {
		return false, 0
	}
	exe, err := os.Executable()
	if err != nil || update.CheckPairPending(filepath.Dir(exe)) != nil {
		return true, 2 // pending installation cannot start worker/network code
	}
	if len(args) != 1 || args[0] != EngineWorkerArgument || !workerPipesReady() {
		return true, 2 // malformed internal modes must never become GUI launches
	}
	return true, serveEngineWorker(os.Stdin, os.Stdout)
}

func serveEngineWorker(input, output *os.File) (exitCode int) {
	// Never forward panic/config/error text across the boundary. The parent
	// owns process retirement, including a panic or noncooperative Close.
	defer func() {
		if recover() != nil {
			exitCode = 3
		}
	}()
	if writeWorkerFrame(output, workerReply{Version: workerProtocolVersion, Code: "ready"}, maxWorkerReplyBytes) != nil {
		return 2
	}
	var engine *Engine
	var sequence uint64
	for {
		var request workerRequest
		if readWorkerFrame(input, &request, maxWorkerRequestBytes) != nil || !validWorkerRequest(request, sequence) {
			return 2 // process exit reclaims engine; no potentially blocking Close here
		}
		sequence = request.ID
		reply := workerReply{Version: workerProtocolVersion, ID: sequence, Code: "ok"}
		stop := false
		switch request.Action {
		case "start":
			if engine != nil {
				reply.Code = "state_error"
				break
			}
			engine = NewEngine(request.Raw)                // only worker/local harness embeds sing-box
			engine.workerCustody = &workerIngressCustody{} // never exposed to App/caller JSON
			ctx, cancel := context.WithTimeout(context.Background(), workerStartTimeout)
			err := engine.Start(ctx)
			cancel()
			if err != nil {
				reply.Code, stop = "start_failed", true
			}
		case "switch", "current":
			if engine == nil {
				reply.Code = "state_error"
				break
			}
			var err error
			if request.Action == "switch" {
				reply.Previous, err = engine.SwitchDefault(request.Selector, request.Tag)
			} else {
				reply.Previous, err = engine.CurrentDefault(request.Selector)
			}
			if err != nil {
				reply.Previous = ""
				if errors.Is(err, ErrSelectorSwitchUnavailable) {
					reply.Code = "selector_unavailable"
				} else {
					reply.Code = "state_error"
				}
			}
		case "probe":
			if engine == nil {
				reply.Code = "state_error"
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(request.Probe.TimeoutMillis)*time.Millisecond)
			var err error
			reply.Report, reply.AdmissionEpoch, reply.AdmissionToken, err = engine.runLocalWorkerProbe(ctx, request.Subject, request.Tag, request.Probe)
			if err == nil && request.Probe.TUN && request.Probe.NativeNonce != "" && probePassed(reply.Report) {
				reply.NativeAdmissionToken, err = engine.emitPreparedNative(ctx, request.Subject, request.Tag, reply.AdmissionEpoch, request.Probe.NativeNonce)
			}
			cancel()
			localFault := reportLifecycleError(reply.Report)
			if err != nil || localFault != nil {
				reply.Code, stop = "probe_failed", true
				failure := workerProbeFailure(err, localFault)
				if reply.AdmissionEpoch != 0 && reportLifecycleError(failure) != nil {
					// Exact request + captured original epoch identify negative work;
					// no current-state query may erase or refresh this finite fact.
					reply.Report, reply.Subject, reply.Previous = failure, request.Subject, request.Tag
				} else {
					reply.Report, reply.AdmissionEpoch = nil, 0
				}
				reply.AdmissionToken, reply.NativeAdmissionToken = "", ""
			}
		case "admit", "admit_native":
			if engine == nil {
				reply.Code = "state_error"
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
			var err error
			if request.Action == "admit_native" {
				err = engine.consumeLocalNativeForwarding(ctx, request.Subject, request.Tag, request.AdmissionEpoch, request.ScopeNonce, request.NativeAdmissionToken)
			} else {
				err = engine.consumeLocalForwarding(ctx, request.Subject, request.Tag, request.AdmissionEpoch, request.AdmissionToken)
			}
			cancel()
			if err != nil {
				reply.Code, stop = "admission_failed", true
			}
		case "tun_scope_open", "tun_scope_close", "tun_native_confirm":
			if engine == nil {
				reply.Code, stop = "state_error", true
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
			var err error
			if request.Action == "tun_scope_open" {
				err = engine.openLocalTUNScope(ctx, request.Subject, request.Tag, request.AdmissionEpoch, request.TUNScope)
				reply.ScopeNonce = request.TUNScope.Nonce
			} else if request.Action == "tun_scope_close" {
				err = engine.closeLocalTUNScope(ctx, request.Subject, request.Tag, request.AdmissionEpoch, request.ScopeNonce)
				reply.ScopeNonce = request.ScopeNonce
			} else {
				err = engine.confirmLocalNative(ctx, request.Subject, request.Tag, request.AdmissionEpoch, request.ScopeNonce, request.NativeEgress)
				reply.ScopeNonce = request.ScopeNonce
			}
			cancel()
			if err != nil {
				reply.Code, reply.ScopeNonce, stop = "tun_scope_failed", "", true
			}
		case "dns_pause", "dns_resume":
			if engine == nil {
				reply.Code, stop = "state_error", true
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), workerControlTimeout)
			err := engine.localServiceDNS(ctx, request)
			cancel()
			if err != nil {
				reply.Code, stop = "dns_handoff_failed", true
				if report := workerDNSFaultReport(request, err); report != nil {
					reply.Report, reply.Subject, reply.Previous, reply.AdmissionEpoch = report, request.Subject, request.Tag, request.AdmissionEpoch
				}
			}
		case "stop":
			stop = true
			if engine != nil && engine.Stop() != nil {
				reply.Code = "close_failed"
			}
		}
		if request.Action != "stop" && (reply.Code == "ok" || reply.Code == "selector_unavailable") {
			subject, selected, epoch, admitted, err := engine.localForwardingState()
			scoped := request.Action == "tun_scope_open" || request.Action == "tun_scope_close" || request.Action == "tun_native_confirm"
			dns := request.Action == "dns_pause" || request.Action == "dns_resume"
			if err != nil || !workerSubject(subject) || !validWorkerTag(selected) || epoch == 0 || reply.AdmissionEpoch != 0 && reply.AdmissionEpoch != epoch ||
				(request.Action == "probe" || request.Action == "admit" || request.Action == "admit_native" || scoped || dns) && (subject != request.Subject || selected != request.Tag) ||
				dns && (epoch != request.AdmissionEpoch || !admitted) ||
				request.Action == "admit_native" && (epoch != request.AdmissionEpoch || !admitted) || scoped && (epoch != request.AdmissionEpoch || admitted) {
				reply = workerReply{Version: workerProtocolVersion, ID: sequence, Code: "state_error"}
				stop = true
			} else {
				reply.Subject, reply.AdmissionEpoch, reply.Admitted = subject, epoch, admitted
				if request.Action == "probe" || request.Action == "admit" || request.Action == "admit_native" || scoped || dns {
					reply.Previous = selected
				}
			}
		}
		var beforeWrite func() error
		if request.Action == "probe" && reply.Code == "ok" && (reply.AdmissionToken != "" || reply.NativeAdmissionToken != "") {
			beforeWrite = func() error {
				if !engine.workerReplyPermitCurrent(&request, reply) {
					return errWorkerProtocol
				}
				return nil
			}
		}
		if writeWorkerFrameGuarded(output, reply, maxWorkerReplyBytes, beforeWrite) != nil {
			return 2
		}
		if stop {
			return 0 // main uses os.Exit, no deferred Wails/app cleanup
		}
	}
}
