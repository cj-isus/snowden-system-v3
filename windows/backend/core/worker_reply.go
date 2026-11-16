package core

// Pure protocol checks after original wire version/ID correlation. A successful
// ACK is not current source/native/Manager authority or a Job cleanup receipt.
func workerCommandReplyFailure(request *workerRequest, reply workerReply) error {
	if request == nil {
		if validWorkerHelloReply(reply) {
			return nil
		}
		return errWorkerProtocol
	}
	if workerStateReplyAction(request.Action) {
		return workerStateReplyFailure(request, reply)
	}
	if request.Action == "stop" {
		if emptyWorkerFailureReply(reply) {
			if reply.Code == "ok" {
				return nil
			}
			if reply.Code == "close_failed" {
				return ErrEngineWorkerUnavailable // original actual reap still decides Stop cleanup
			}
		}
		return errWorkerProtocol
	}
	if request.Action == "probe" {
		return workerProbeReplyFailure(request, reply)
	}
	failureCode := workerControlFailureCode(request.Action)
	if failureCode == "" {
		return errWorkerProtocol
	}
	if validWorkerControlReply(request, reply) {
		return nil
	}
	if reply.Code != failureCode && reply.Code != "state_error" {
		return errWorkerProtocol
	}
	if emptyWorkerFailureReply(reply) {
		return ErrEngineWorkerUnavailable
	}
	if (request.Action == "dns_pause" || request.Action == "dns_resume") && reply.Code == failureCode &&
		workerControlSubjectMatches(request, reply) && !reply.Admitted && reply.AdmissionToken == "" && reply.NativeAdmissionToken == "" && reply.ScopeNonce == "" &&
		reply.Report != nil && !reply.Report.AllPassed && len(reply.Report.Results) > 0 && len(reply.Report.Results) <= 16 {
		if cause := reportLifecycleError(reply.Report); cause != nil {
			return cause // finite original DNS restriction only, never fresh path evidence
		}
	}
	return errWorkerProtocol
}

func validWorkerHelloReply(reply workerReply) bool {
	return reply.Code == "ready" && emptyWorkerFailureReply(reply)
}

func workerControlFailureCode(action string) string {
	switch action {
	case "admit", "admit_native":
		return "admission_failed"
	case "tun_scope_open", "tun_scope_close", "tun_native_confirm":
		return "tun_scope_failed"
	case "dns_pause", "dns_resume":
		return "dns_handoff_failed"
	default:
		return ""
	}
}

func emptyWorkerFailureReply(reply workerReply) bool {
	return reply.Subject == "" && reply.Previous == "" && reply.AdmissionEpoch == 0 && !reply.Admitted &&
		reply.AdmissionToken == "" && reply.NativeAdmissionToken == "" && reply.ScopeNonce == "" && reply.Report == nil
}

func workerControlSubjectMatches(request *workerRequest, reply workerReply) bool {
	return request != nil && request.Selector == "proxy" && workerSubject(request.Subject) && validWorkerTag(request.Tag) && request.AdmissionEpoch != 0 &&
		reply.Subject == request.Subject && reply.Previous == request.Tag && reply.AdmissionEpoch == request.AdmissionEpoch
}

// Shared by the pipe owner and parent post-wait checks. The original command
// determines the ACK phase; no caller-supplied report or "current" bool is used.
func validWorkerControlReply(request *workerRequest, reply workerReply) bool {
	if !workerControlSubjectMatches(request, reply) {
		return false
	}
	admitted, nonce := false, ""
	switch request.Action {
	case "admit":
		admitted = true
	case "admit_native":
		if !workerSubject(request.ScopeNonce) || !workerSubject(request.NativeAdmissionToken) {
			return false
		}
		admitted = true
	case "dns_pause", "dns_resume":
		admitted = true // only the already-admitted original DNS lane may send these
	case "tun_scope_open":
		if request.TUNScope == nil || !workerSubject(request.TUNScope.Nonce) {
			return false
		}
		nonce = request.TUNScope.Nonce
	case "tun_scope_close", "tun_native_confirm":
		if !workerSubject(request.ScopeNonce) {
			return false
		}
		nonce = request.ScopeNonce
	default:
		return false
	}
	return validWorkerScopedStateReply(reply, request.Subject, request.Tag, request.AdmissionEpoch, admitted, nonce)
}

// Value-only scope shape also serves the existing netprobe request adapter.
// The actual original request and all native ownership gates stay with callers.
func validWorkerScopedStateReply(reply workerReply, subject, selector string, epoch uint64, admitted bool, nonce string) bool {
	return reply.Code == "ok" && workerSubject(subject) && validWorkerTag(selector) && epoch != 0 &&
		reply.Subject == subject && reply.Previous == selector && reply.AdmissionEpoch == epoch && reply.Admitted == admitted &&
		reply.AdmissionToken == "" && reply.NativeAdmissionToken == "" && reply.Report == nil && reply.ScopeNonce == nonce
}
