package core

import (
	"context"
	"time"
)

// Current parent IPC consumes fixed source values and stdlib cancellation
// keys. Keep the latter on a real owned stdlib child: a metadata-only context
// would hide cancelCtx identity and introduce a propagation watcher instead.
// Unknown Value keys still follow the real child; this is not a general SDK
// service/trace context or a guarantee about arbitrary future Value callbacks.
type workerRPCContext struct {
	context.Context // only a child constructed below, never the raw caller
	deadline        time.Time
	sources         any
	parentScope     *nativeTUNOperation // exact RPC parent metadata, checked after cancel before diagnostic release
}

func (ctx *workerRPCContext) Deadline() (time.Time, bool) { return ctx.deadline, true }

func (ctx *workerRPCContext) Value(key any) any {
	if _, ok := key.(operationSourcesContextKey); ok {
		return ctx.sources // exact original binding, including future revocation
	}
	return ctx.Context.Value(key) // owned child handles stdlib cancellation keys itself
}

// Caller prepares AND cancels this scope outside Engine.mu. The absolute
// budget starts here and includes later lock waiting; an exchange cannot reset
// it. operation must snapshot this exact RPC parent; the lifecycle handoff
// identity is retained separately by Engine. No metadata view is used as a
// cancellation parent.
func prepareWorkerRPCContext(ctx context.Context, operation *nativeTUNOperation, limit time.Duration) (*workerRPCContext, context.CancelFunc, error) {
	if ctx == nil || operation == nil || limit <= 0 {
		return nil, nil, ErrForwardingNotAdmitted
	}
	if err := contextError(operation); err != nil {
		return nil, nil, err
	}
	parentDeadline, parentBounded := ctx.Deadline()
	if ctx.Done() != operation.Done() || parentBounded != operation.hasDeadline || parentBounded && !parentDeadline.Equal(operation.deadline) {
		return nil, nil, ErrForwardingNotAdmitted // a different metadata scope cannot supply the RPC budget
	}
	deadline := time.Now().Add(limit)
	var bounded context.Context
	var cancel context.CancelFunc
	if operation.hasDeadline && !deadline.Before(operation.deadline) {
		deadline = operation.deadline
		bounded, cancel = context.WithCancel(ctx) // reuse parent's earlier/equal deadline; no duplicate timer
	} else {
		bounded, cancel = context.WithDeadline(ctx, deadline)
	}
	return &workerRPCContext{Context: bounded, deadline: deadline, sources: operation.sources, parentScope: operation}, cancel, nil
}

// The shorter diagnostic reservation has its own deadline and may carry
// additional negative sources. Snapshot it, not the lifecycle operation;
// original lifecycle/native handoff checks keep their separate original view.
func prepareWorkerProbeRPCContext(ctx context.Context, request *workerProbeRequest) (*workerRPCContext, context.CancelFunc, error) {
	settings, err := request.settings()
	if err != nil {
		return nil, nil, err
	}
	diagnostic, err := snapshotNativeTUNOperation(ctx)
	if err != nil {
		return nil, nil, err
	}
	return prepareWorkerRPCContext(ctx, diagnostic, settings.Timeout+workerControlTimeout)
}

// The fixed negative Stop has no caller context/source to consult. Its only
// parent is stdlib Background, so construction/cancel under Engine.mu cannot
// invoke user getters or parent unlink callbacks. Actual Job retirement is
// still required after this request, regardless of its reply or timeout.
func (p *engineProcess) requestStop() (workerReply, error) {
	deadline := time.Now().Add(workerCloseTimeout)
	child, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	ctx := &workerRPCContext{Context: child, deadline: deadline}
	return p.request(ctx, workerRequest{Action: "stop"}, workerCloseTimeout)
}
