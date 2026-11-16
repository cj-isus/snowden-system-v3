package main

import "strconv"

func sameDiagnosticContext(first, second *DiagnosticContextView) bool {
	return first != nil && second != nil && first.Schema == 1 && first.Revision != "" && first.Revision != "0" && *first == *second
}

// App.mu held. State publication can already hold Epoch.mu, so use the
// existing atomic original-path mirror and immutable ID, never lock Epoch here.
// Values may become stale immediately; only private sources govern operations.
func (a *App) diagnosticContextLocked() *DiagnosticContextView {
	if a.diagnosticContextExhausted {
		return nil
	}
	path := netEpoch.pathValue.Load()
	blocked := netEpoch.powerBlocked.Load() || a.watchdogBlocked.Load()
	lifecycle := a.state.State
	if blocked && a.protectionDesired && (lifecycle == "running" || lifecycle == "starting" || lifecycle == "reloading") {
		lifecycle = "error"
	}
	view := &DiagnosticContextView{Schema: 1, Intent: strconv.FormatUint(a.operationID, 10), Path: "0",
		IntentCurrent: a.ctx != nil && a.ctx.Err() == nil && a.intentSource.Current(), Closing: a.closing.Load(),
		Channel: a.state.ActiveID, Mode: a.state.ProxyMode, Lifecycle: lifecycle, Ready: a.state.CoreReady && !blocked && a.startupErr == nil}
	if path != nil {
		view.Path = strconv.FormatUint(path.identity, 10)
		view.PathCurrent = path.identity != 0 && path.current.Load() && !netEpoch.powerBlocked.Load()
	}
	previous := DiagnosticContextView{}
	if a.diagnosticContextLast != nil {
		previous = *a.diagnosticContextLast
		previous.Revision = ""
	}
	if previous != *view {
		if a.diagnosticContextRevision == ^uint64(0) {
			a.diagnosticContextExhausted = true
			return nil
		}
		a.diagnosticContextRevision++
	}
	view.Revision = strconv.FormatUint(a.diagnosticContextRevision, 10)
	a.diagnosticContextLast = view // immutable; older task/cache receipts keep their own object
	return view
}
