package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	ErrHelperBusy             = errors.New("native helper already owned")
	ErrHelperLimit            = errors.New("native helper output limit exceeded")
	ErrHelperExit             = errors.New("native helper exit was not successful")
	ErrHelperCleanup          = errors.New("native helper retirement unconfirmed")
	ErrProtectedHelperBusy    = ErrHelperBusy
	ErrProtectedHelperLimit   = ErrHelperLimit
	ErrProtectedHelperExit    = ErrHelperExit
	ErrProtectedHelperCleanup = ErrHelperCleanup
)

type helperCaptureLane struct {
	sync.Mutex
	owner *engineProcess
	image nativeImageLease
	tun   *nativeTUNCapture // scoped ingress/filter retire BEFORE the original socket owner
}

var protectedHelper, optionalHelper helperCaptureLane

const helperCaptureTimeout = 30 * time.Second
const helperCaptureCommitBytes uintptr = 128 << 20

// RetireCaptureHelpers is a lifecycle cleanup barrier, not a capture/action.
// Stop the engine/Manager first so it cannot begin new lifecycle captures. Each
// lane is tried without waiting: an in-flight capture or uncertain retirement
// is a cleanup error, never permission to release protection or forget owners.
// Retry uses the exact retained Job/image resources and never starts a process.
// A successful barrier also revalidates the retained GUI/root source after all
// negative reaps. That source query is not a whole-chain retirement receipt.
func RetireCaptureHelpers() error {
	if err := retireCaptureLanes(); err != nil {
		return err
	}
	ancestor := processGUIAncestor // original immutable bootstrap source, not a replacement process
	if ancestor == nil {
		return nil // existing no-ancestor mode has no query or additional timer
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerReapTimeout)
	defer cancel()
	if err := guiAncestorQuery(ctx, func() error { return ancestor.validate(ctx) }); err != nil {
		// This independent barrier could not attest the source, including on
		// its own deadline. Known child reaps do not authorize positive Release.
		return errors.Join(ErrGUIAncestorUnavailable, err)
	}
	return nil
}

// Sequence receipt only. Keep the original protected/optional/delivery order;
// all lanes are registered before any native callback, and a claimed lane's
// mutex is released even if its original retirement ends the goroutine.
func retireCaptureLanes() (result error) {
	sequence := &lifecycleCleanup{}
	defer func() {
		if sequence.result != nil {
			sequence.result = errors.Join(ErrHelperCleanup, sequence.result)
		}
		sequence.drained, result = true, sequence.result
	}()
	lanes := []*helperCaptureLane{&protectedHelper, &optionalHelper, &deliveryVerifierLane}
	for index := len(lanes) - 1; index >= 0; index-- {
		lane := lanes[index]
		defer lifecycleCleanupStep("retire original capture lane", sequence, func() error {
			if !lane.TryLock() {
				return errors.Join(ErrHelperCleanup, ErrHelperBusy)
			}
			defer lane.Unlock()
			return retireHelperLane(lane)
		})
	}
	return nil
}

func retireOwnedHelper(process *engineProcess) (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = ErrHelperCleanup
		}
	}()
	err = process.retire()
	completed = true
	return err
}

func retireHelperLane(lane *helperCaptureLane) (err error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			err = ErrHelperCleanup
		}
	}()
	if lane.tun != nil {
		if err = lane.tun.revoke(); err != nil {
			completed = true
			return err // do not release the socket/image while a grant is uncertain
		}
		lane.tun = nil
	}
	if err = retireHelperProcess(lane); err != nil {
		completed = true
		return err
	}
	if lane.image != nil {
		if err = lane.image.Close(); err != nil {
			completed = true
			return err // retain exact remaining handles; never replace the image
		}
		lane.image = nil
	}
	completed = true
	return nil
}

func retireHelperProcess(lane *helperCaptureLane) error {
	if lane.tun != nil {
		return ErrHelperCleanup // no direct process-only cleanup may bypass grant custody
	}
	if lane.owner == nil {
		return nil
	}
	if err := retireOwnedHelper(lane.owner); err != nil {
		return err
	}
	lane.owner = nil
	return nil
}

func validateOwnedProcessCommand(executable string, args []string) error {
	if !filepath.IsAbs(executable) || len(executable) > 32760 || strings.ContainsRune(executable, 0) || len(args) > 64 {
		return errors.New("owned process requires bounded absolute executable and arguments")
	}
	total := len(executable)
	for _, arg := range args {
		if len(arg) > 8192 || strings.ContainsRune(arg, 0) || total > 32760-len(arg)-1 {
			return errors.New("owned process arguments exceed bound")
		}
		total += len(arg) + 1
	}
	return nil
}

// CaptureProtectedHelper is a single no-queue protected-work lane, not an
// arbitrary command endpoint. Callers select a trusted absolute system helper;
// it is never exposed through Wails. Optional diagnostics must not consume this
// reserved owner. No shell, environment/PATH lookup or PID-based retirement.
func CaptureProtectedHelper(parent context.Context, executable string, arguments []string, maxOutput int64) (output []byte, finalErr error) {
	return captureOwnedHelper(&protectedHelper, parent, executable, arguments, maxOutput, helperCaptureOptions{})
}

// Optional local observations reuse the existing independent bounded lane.
// Callers supply a fixed trusted system image, never Wails/user command text.
func CaptureOptionalHelper(parent context.Context, executable string, arguments []string, maxOutput int64) ([]byte, error) {
	return captureOwnedHelper(&optionalHelper, parent, executable, arguments, maxOutput, helperCaptureOptions{})
}

func captureOwnedHelper(lane *helperCaptureLane, parent context.Context, executable string, arguments []string, maxOutput int64, options helperCaptureOptions) (output []byte, finalErr error) {
	if err := contextError(parent); err != nil {
		return nil, errors.Join(ErrNativeScopeUnavailable, err)
	}
	if maxOutput < 1 || maxOutput > 64<<10 || len(arguments) > 64 {
		return nil, errors.New("native helper request unavailable")
	}
	if (options.imagePin != "" || options.keepImage) && options.guard == nil || options.keepImage && options.imagePin != "" || options.applicationEnvironment && !options.keepImage {
		return nil, ErrNativeScopeUnavailable
	}
	if options.commandExit != nil && (options.imagePin != "" || options.keepImage || options.applicationEnvironment || options.guard != nil) {
		return nil, ErrNativeScopeUnavailable // command exit data cannot weaken a signed verifier/probe contract
	}
	if options.imagePin != "" {
		if _, err := nativeImagePin(options.imagePin); err != nil {
			return nil, err
		}
	}
	args := slices.Clone(arguments)
	if options.imagePin == "" && !options.keepImage {
		if err := validateOwnedProcessCommand(executable, args); err != nil {
			return nil, err
		}
	}
	if !lane.TryLock() {
		return nil, ErrHelperBusy
	}
	defer lane.Unlock()
	ctx, cancel := context.WithTimeout(parent, helperCaptureTimeout)
	defer cancel()
	ownsNew := false
	confirmed := false
	var process *engineProcess // immutable original helper, never a later lane.owner lookup
	defer func() {
		if recovered := recover(); recovered != nil || !confirmed && finalErr == nil {
			output, finalErr = nil, errors.Join(finalErr, errors.New("native helper interrupted"))
		}
		if process != nil && finalErr != nil {
			process.unusable = true // no reuse of an output owner after cancellation/unknown outcome
		}
		if ownsNew {
			retire := retireHelperLane
			if options.keepImage {
				retire = retireHelperProcess
			}
			if err := retire(lane); err != nil {
				finalErr = errors.Join(finalErr, ErrHelperCleanup, err)
			}
		}
		finalErr = errors.Join(finalErr, process.captureFailure()) // reads a cause only if the original I/O actually returned
		if err := contextError(ctx); err != nil {
			finalErr = errors.Join(finalErr, err)
		}
		if finalErr == nil {
			finalErr = process.ancestorScope(ctx, false) // source only; the original child has retired
		}
		if finalErr == nil {
			finalErr = helperScope(ctx, options.guard)
		}
		if finalErr != nil {
			output = nil // exit alone never releases bytes before complete retirement
		}
	}()
	retire := retireHelperLane
	if options.keepImage {
		retire = retireHelperProcess
	}
	if err := retire(lane); err != nil {
		return nil, errors.Join(ErrHelperCleanup, err)
	}
	if err := helperScope(ctx, options.guard); err != nil {
		return nil, err
	}
	if options.imagePin != "" {
		image := newNativeImage()
		lane.image, ownsNew = image, image != nil
		if image == nil {
			return nil, ErrNativeImageUnavailable
		}
		if err := image.prepareCurrent(ctx, options.imagePin); err != nil {
			return nil, err
		}
		executable = image.Executable()
	}
	if options.keepImage {
		if lane.image == nil {
			return nil, ErrNativeImageUnavailable
		}
		if err := lane.image.Validate(ctx); err != nil {
			return nil, errors.Join(ErrNativeImageUnavailable, err)
		}
		executable = lane.image.Executable()
	}
	if err := validateOwnedProcessCommand(executable, args); err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	read := newHelperOutputRead(maxOutput) // fixed allocation before child/reader creation
	policy := ownedProcessLimits{processes: 1, commitBytes: helperCaptureCommitBytes, cleanSystemEnvironment: true}
	policy.combinedOutput = options.commandExit != nil
	if options.applicationEnvironment {
		policy.commitBytes = updateCodeCheckCommitBytes
		policy.userTime = updateCodeCheckUserTime
		policy.applicationEnvironment = true
	}
	if lane.image != nil {
		policy.beforeResume = func(ctx context.Context, host processHost) error {
			if err := lane.image.ValidateProcess(ctx, host); err != nil {
				return err
			}
			return helperScope(ctx, options.guard)
		}
	}
	var err error
	process = newOwnedProcess()
	lane.owner, ownsNew = process, ownsNew || process != nil
	if process == nil {
		return nil, ErrEngineWorkerUnavailable
	}
	err = process.spawn(ctx, executable, args, policy)
	if err != nil {
		return nil, err // even a partial/suspended process is retired by the owner
	}
	if process == nil {
		return nil, errors.New("native helper owner unavailable")
	}
	if err := process.input.Close(); err != nil {
		return nil, errors.New("native helper input close failed")
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := process.ioRetirementReady(); err != nil {
		process.unusable = true
		return nil, err
	}
	if err := process.ancestorScope(ctx, true); err != nil {
		process.unusable = true
		return nil, err
	}
	process.captureIO, process.ioDone = &read.ownedPipeIO, read.done
	var readDone <-chan struct{} = read.done
	go read.run(ctx, process.output)
	intentDone, operationDone, dependencyDone := operationSourceDone(ctx)
	exitDone := process.done
	for readDone != nil || exitDone != nil {
		select {
		case <-readDone:
			if err := read.completedFailure(); err != nil {
				return nil, err
			}
			output, readDone = read.data, nil
		case <-exitDone:
			if process.waitErr != nil {
				return nil, errors.New("native helper wait unconfirmed")
			}
			exitDone = nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-intentDone:
			return nil, errors.Join(context.Canceled, contextError(ctx))
		case <-operationDone:
			return nil, errors.Join(context.Canceled, contextError(ctx))
		case <-dependencyDone:
			return nil, errors.Join(context.Canceled, contextError(ctx))
		}
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	status, ok := process.host.(interface{ ExitCode() (uint32, error) })
	if !ok {
		return nil, ErrHelperExit
	}
	code, err := status.ExitCode()
	if err != nil {
		return nil, ErrHelperExit
	}
	if options.commandExit != nil {
		options.commandExit.code, options.commandExit.returned = code, true
	}
	if code != 0 && options.commandExit == nil {
		return nil, fmt.Errorf("%w: status %d", ErrHelperExit, code)
	}
	if options.applicationEnvironment {
		resources, ok := process.host.(processResourceHost)
		if !ok {
			return nil, errors.New("bounded verifier resource owner unavailable")
		}
		if err := resources.ValidateResourceUsage(); err != nil {
			return nil, err
		}
	}
	if lane.image != nil {
		if err := lane.image.Validate(ctx); err != nil {
			return nil, err
		}
	}
	confirmed = true
	return output, nil
}
