package core

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

var (
	ErrNativeImageUnavailable = errors.New("pinned native helper image unavailable")
	ErrNativeScopeUnavailable = errors.New("native observation scope unavailable")
	ErrNativeProbeIncomplete  = errors.New("native observation batch incomplete")
)

const maxNativeImageBytes int64 = netprobe.MaxNativeImageBytes // initial ceiling, not measured RSS

// A helper/updater lane owns this resource, including partial opens. Image handles
// cannot be released while an uncertain process/Job may still use the image.
type nativeImageLease interface {
	Executable() string
	Validate(context.Context) error
	ValidateProcess(context.Context, processHost) error
	Close() error
}

// Pure allocation returns this owner before either in-place preparation can
// acquire native files. Production callers store it in their original slot first.
type nativeImagePreparation interface {
	nativeImageLease
	prepareCurrent(context.Context, string) error
	prepareInstalled(context.Context, string) error
}

type helperCaptureOptions struct {
	imagePin               string
	guard                  func(context.Context) error
	keepImage              bool               // dedicated updater owner only; process retires before bytes, image stays owned
	applicationEnvironment bool               // same-user public SDK store, no GUI/runtime environment inheritance
	commandExit            *helperCommandExit // ordinary system-command facts only, never verifier/probe acceptance
}

func nativeImagePin(value string) ([32]byte, error) {
	var pin [32]byte
	if len(value) != 64 {
		return pin, ErrNativeImageUnavailable
	}
	raw, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(raw) != value {
		return pin, ErrNativeImageUnavailable
	}
	copy(pin[:], raw)
	return pin, nil
}

func helperScope(ctx context.Context, guard func(context.Context) error) (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = errors.Join(err, ErrNativeScopeUnavailable)
		}
	}()
	err = contextError(ctx)
	if err == nil && guard != nil {
		err = guard(ctx)
	}
	if currentErr := contextError(ctx); currentErr != nil && !errors.Is(err, currentErr) {
		err = errors.Join(err, currentErr) // original local guard failure survives a later cancellation
	}
	completed = true
	return err
}

// CaptureNativeDNS is a bounded, fixed-operation primitive, not a Wails action.
// The caller must supply its actual before/after session lease guard; a nonnil
// callback alone is not evidence that intent/worker/LUID/epoch/clock were bound.
// The PC App supplies its nativeProbeScope guard for the explicit manual action.
func CaptureNativeDNS(parent context.Context, pin string, networks []string, limit time.Duration, guard func(context.Context) error) (netprobe.NativeReply, error) {
	if guard == nil || limit < 500*time.Millisecond || limit > 12*time.Second {
		return netprobe.NativeReply{}, ErrNativeScopeUnavailable
	}
	if _, err := nativeImagePin(pin); err != nil {
		return netprobe.NativeReply{}, err
	}
	if err := helperScope(parent, guard); err != nil {
		return netprobe.NativeReply{}, err
	}
	request, err := netprobe.NewNativeRequest(networks, int(limit/time.Millisecond))
	if err != nil {
		return netprobe.NativeReply{}, err
	}
	invocation, err := netprobe.NewDNSInvocation(request)
	if err != nil {
		return netprobe.NativeReply{}, err
	}
	arguments, err := invocation.Arguments()
	if err != nil {
		return netprobe.NativeReply{}, err
	}
	attempts := len(request.QueryIDs)
	// Reserve every possible payload, plus bounded IPC encoding/capture. This
	// excludes OS buffers/headers/retransmissions, not a whole-machine I/O cap.
	cost := int64(netprobe.MaxNativeReply+1+base64.RawURLEncoding.EncodedLen(netprobe.MaxNativeRequest)) +
		int64(attempts)*(netprobe.MaxDNSQuery+netprobe.MaxDNSMessage+5)
	ctx, release, err := AdmitDiagnostic(parent, DiagnosticEndpoint, attempts, cost, limit)
	if err != nil {
		return netprobe.NativeReply{}, err
	}
	defer release()
	output, err := captureOwnedHelper(&optionalHelper, ctx, "", arguments, netprobe.MaxNativeReply, helperCaptureOptions{
		imagePin: pin, guard: guard,
	})
	if err != nil {
		return netprobe.NativeReply{}, err
	}
	if err := helperScope(ctx, guard); err != nil {
		return netprobe.NativeReply{}, err // no decode after original operation/source revocation
	}
	reply, err := netprobe.DecodeNativeReply(output, request)
	scopeErr := helperScope(ctx, guard)
	if err != nil || scopeErr != nil {
		return netprobe.NativeReply{}, errors.Join(err, scopeErr)
	}
	if !reply.Complete {
		return netprobe.NativeReply{}, ErrNativeProbeIncomplete
	}
	return reply, nil
}
