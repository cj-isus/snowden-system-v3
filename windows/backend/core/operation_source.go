package core

import (
	"context"
	"sync/atomic"
)

// OperationSource is a process-local negative fence. Revocation is permanent;
// a new source cannot revive a context/report bound to its predecessor. Being
// current is never evidence of health, custody, trust or forwarding authority.
// Do not copy a source; contexts/reports share only its pointer.
type OperationSource struct {
	current atomic.Bool
	done    chan struct{} // passive notification; no watcher goroutine or callback
}

func NewOperationSource() *OperationSource {
	source := &OperationSource{done: make(chan struct{})}
	source.current.Store(true)
	return source
}

func (source *OperationSource) Revoke() {
	if source != nil && source.current.Swap(false) {
		close(source.done) // one original source, at most one close
	}
}

func (source *OperationSource) Current() bool {
	return source != nil && source.current.Load()
}

func (source *OperationSource) Done() <-chan struct{} {
	if source == nil {
		return nil
	}
	return source.done
}

// Pointer-only sources: checks inside Engine.mu/pipe I/O do not call back into
// App/Manager or acquire their locks. Operation may be nil for a fresh periodic
// measurement of an already-applied intent; Intent and Path are mandatory.
type OperationSources struct {
	Intent     *OperationSource
	Operation  *OperationSource
	Dependency *OperationSource // optional original observer lifetime, never task-retirement proof
	Path       *atomic.Bool
	Closing    *atomic.Bool
	Blocked    *atomic.Bool // optional independent negative continuity fence
}

type operationSourcesContextKey struct{}

func WithOperationSources(ctx context.Context, sources OperationSources) context.Context {
	if ctx == nil {
		return nil
	}
	if value := ctx.Value(operationSourcesContextKey{}); value != nil {
		original, ok := value.(OperationSources)
		if !ok || original.Intent != sources.Intent || original.Path != sources.Path || original.Closing != sources.Closing ||
			original.Blocked != sources.Blocked || sources.Operation != nil && original.Operation != sources.Operation ||
			sources.Dependency != nil && original.Dependency != sources.Dependency {
			// A foreign/retired context may only become more restrictive. Never
			// replace its binding with the caller's newer live sources.
			return context.WithValue(ctx, operationSourcesContextKey{}, OperationSources{})
		}
		return ctx // keep original optional operation/dependency fences, including revocation
	}
	return context.WithValue(ctx, operationSourcesContextKey{}, sources)
}

// Preserve the original process-local negative binding when an owned actor or
// observation receives a shorter caller context. Never substitute the caller's
// newer sources. Generic unbound originals keep their existing contract; their
// own cancellation/deadline must still be checked by the owner.
func withOriginalOperationSources(ctx, original context.Context) context.Context {
	if original == nil || contextError(original) != nil {
		return WithOperationSources(ctx, OperationSources{})
	}
	value := original.Value(operationSourcesContextKey{})
	if value == nil {
		return ctx
	}
	sources, ok := value.(OperationSources)
	if !ok {
		sources = OperationSources{}
	}
	return WithOperationSources(ctx, sources)
}

func OperationSourcesCurrent(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value := ctx.Value(operationSourcesContextKey{})
	if value == nil {
		return true // unbound generic/independent cleanup contexts keep their contract
	}
	sources, ok := value.(OperationSources)
	return ok && sources.Intent.Current() && (sources.Operation == nil || sources.Operation.Current()) &&
		(sources.Dependency == nil || sources.Dependency.Current()) && sources.Path != nil && sources.Path.Load() &&
		(sources.Closing == nil || !sources.Closing.Load()) && (sources.Blocked == nil || !sources.Blocked.Load())
}

// DNS control continuity is deliberately separate from physical path continuity:
// a down event may pause DNS while the original network path is revoked. This
// binding grants no measurement/native/forwarding authority. Rebinding it to a
// protected probe's physical path is rejected by WithOperationSources.
func WithServiceDNSControlSources(ctx context.Context, intent, event, dependency *OperationSource, closing, blocked *atomic.Bool) context.Context {
	var path *atomic.Bool
	if event != nil {
		path = &event.current
	}
	return WithOperationSources(ctx, OperationSources{Intent: intent, Operation: event, Dependency: dependency, Path: path, Closing: closing, Blocked: blocked})
}

func operationSourceDone(ctx context.Context) (<-chan struct{}, <-chan struct{}, <-chan struct{}) {
	if ctx == nil {
		return nil, nil, nil
	}
	sources, ok := ctx.Value(operationSourcesContextKey{}).(OperationSources)
	if !ok {
		return nil, nil, nil
	}
	return sources.Intent.Done(), sources.Operation.Done(), sources.Dependency.Done()
}

// The report retains only its original intent source, not the completed
// operation's context/fence. Clone deliberately shares this revocation source;
// JSON, worker protocol and UI fields do not carry it. It can only deny reuse.
func (report *ProbeReport) OperationSource() *OperationSource {
	if report == nil {
		return nil
	}
	return report.operationSource
}

func (report *ProbeReport) BindOperationSource(source *OperationSource) {
	if report == nil {
		return
	}
	if source == nil || report.operationSource != nil && report.operationSource != source {
		report.AllPassed, report.Stale = false, true
		return // no retagging an existing observation to a different intent source
	}
	report.operationSource = source
	if !source.Current() {
		report.AllPassed, report.Stale = false, true
	}
}

func (report *ProbeReport) ObservationPath() *atomic.Bool {
	if report == nil {
		return nil
	}
	return report.observationPath
}

// Bind only the original process-local path. Public epoch values/JSON cannot
// manufacture this identity or move old work to a replacement path.
func (report *ProbeReport) BindObservationPath(path *atomic.Bool, restrictive bool) {
	if report == nil {
		return
	}
	if path == nil || report.observationPath != nil && report.observationPath != path {
		report.AllPassed, report.Stale = false, true
		return
	}
	report.observationPath = path
	if restrictive {
		report.MarkRestrictionObserved()
	}
	if report.restrictionObserved {
		report.AllPassed = false
	}
}

func (report *ProbeReport) RestrictionObserved() bool {
	return report != nil && report.operationSource != nil && report.observationPath != nil && report.restrictionObserved
}

// Only an owned, already-bound observation can carry a restrictive follow-up.
// This marker neither identifies a leak nor authorizes a positive transition.
func (report *ProbeReport) MarkRestrictionObserved() {
	if report != nil && report.operationSource != nil && report.observationPath != nil {
		report.restrictionObserved = true
		report.AllPassed = false
	}
}
