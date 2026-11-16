package core

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
)

const deliveryWorkerArgument = "--internal-delivery-verifier-v1"

var deliveryVerifierLane helperCaptureLane

// Preserve WorkerCall's original WriteCloser/EOF contract. Read/Write share the
// existing source-fenced wrappers; Close remains a checked original-file call,
// never a deferred/synthetic EOF. This adapter belongs to one I/O goroutine.
type deliveryWorkerInput struct {
	ownedPipeWriter
	original       io.WriteCloser
	closeAttempted bool
}

func (input *deliveryWorkerInput) Close() error {
	if input == nil || input.original == nil || input.closeAttempted {
		return ErrHelperCleanup
	}
	input.closeAttempted = true
	if input.effects != nil {
		input.effects.closePending = true
	}
	err := input.original.Close()
	if input.effects != nil {
		input.effects.closePending, input.effects.closeReturned = false, true
	}
	return err
}

// PrepareDeliveryArtifacts is the product artifact-verification SDK entry.
// An existing bounded update-check process has the same CPU/memory/one-process Job policy:
// use its SDK directly, without a forbidden nested child or a fresh CPU budget.
// The GUI must use the compiled native transport. Query/spawn failure does not
// grant an in-process fallback. Offline publisher tools keep their own API.
func PrepareDeliveryArtifacts(ctx context.Context, state *delivery.State, pack *delivery.Package, requests []delivery.Request, now time.Time) (*delivery.Progress, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if updateCodeCheckResourcesReady() {
		return delivery.PrepareArtifacts(ctx, state, pack, requests, now)
	}
	return delivery.PrepareWithTrustedWorker(ctx, state, pack, requests, now, exchangeDeliveryWorker)
}

func exchangeDeliveryWorker(parent context.Context, call *delivery.WorkerCall) (finalErr error) {
	if call == nil {
		return delivery.ErrTrust
	}
	if err := contextError(parent); err != nil {
		return err
	}
	if !deliveryVerifierLane.TryLock() {
		return ErrHelperBusy
	}
	defer deliveryVerifierLane.Unlock()
	lane := &deliveryVerifierLane
	ctx, cancel := context.WithTimeout(parent, helperCaptureTimeout)
	defer cancel()
	ownsNew, completed := false, false
	var process *engineProcess // exact original even after full retirement clears lane.owner
	defer func() {
		if recovered := recover(); recovered != nil || !completed && finalErr == nil {
			finalErr = errors.Join(finalErr, delivery.ErrTrust)
		}
		if process != nil && finalErr != nil {
			process.unusable = true
		}
		if ownsNew {
			if err := retireHelperLane(lane); err != nil {
				finalErr = errors.Join(finalErr, ErrHelperCleanup, err)
			}
		}
		finalErr = errors.Join(finalErr, process.captureFailure())
		if finalErr == nil {
			// This source attestation follows physical retirement. Use a fresh
			// bounded custody budget so the SDK can still retain its verified
			// restriction prefix after late cancellation, never artifact authority.
			custodyCtx, custodyCancel := context.WithTimeout(context.Background(), workerReapTimeout)
			defer custodyCancel()
			finalErr = process.ancestorScope(custodyCtx, false)
		}
		// After confirmed retirement, delivery itself handles late cancellation:
		// only a verified restriction prefix survives it, never artifact authority.
	}()
	if err := retireHelperLane(lane); err != nil {
		return errors.Join(ErrHelperCleanup, err)
	}
	image := newVerifierApplication()
	lane.image, ownsNew = image, image != nil
	if image == nil {
		return ErrNativeImageUnavailable
	}
	if err := image.prepare(ctx); err != nil {
		return errors.Join(ErrNativeImageUnavailable, err)
	}
	policy := ownedProcessLimits{processes: 1, commitBytes: updateCodeCheckCommitBytes, userTime: updateCodeCheckUserTime,
		cleanSystemEnvironment: true, deliveryVerifier: true}
	policy.beforeResume = func(ctx context.Context, host processHost) error { return lane.image.ValidateProcess(ctx, host) }
	process = newOwnedProcess()
	lane.owner, ownsNew = process, ownsNew || process != nil
	if process == nil {
		return ErrEngineWorkerUnavailable
	}
	if err := process.spawn(ctx, lane.image.Executable(), []string{deliveryWorkerArgument, call.Digest()}, policy); err != nil {
		return err
	}
	if process == nil {
		return delivery.ErrTrust
	}
	if err := process.ioRetirementReady(); err != nil {
		process.unusable = true
		return err
	}
	if err := process.ancestorScope(ctx, true); err != nil {
		process.unusable = true
		return err
	}
	exchange := &ownedPipeIO{done: make(chan struct{}), effects: new(ownedPipeEffects)}
	process.captureIO, process.ioDone = exchange, exchange.done
	go func() {
		finished := false
		defer func() {
			if recovered := recover(); recovered != nil || !finished {
				exchange.err = errors.Join(exchange.err, delivery.ErrTrust)
			}
			close(exchange.done) // goroutine end only; raw Read/Write/EOF-Close facts remain independent
		}()
		input := &deliveryWorkerInput{
			ownedPipeWriter: ownedPipeWriter{ctx: ctx, writer: process.input, failure: delivery.ErrTrust, effects: exchange.effects},
			original:        process.input,
		}
		output := ownedPipeReader{ctx: ctx, reader: process.output, failure: delivery.ErrTrust, effects: exchange.effects}
		exchange.err = call.Exchange(ctx, input, output)
		finished = true
	}()
	var readDone <-chan struct{} = exchange.done
	exitDone := process.done
	for readDone != nil || exitDone != nil {
		select {
		case <-readDone:
			if err := exchange.completedFailure(); err != nil {
				return err
			}
			readDone = nil
		case <-exitDone:
			if process.waitErr != nil {
				return ErrHelperExit
			}
			exitDone = nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	status, ok := process.host.(interface{ ExitCode() (uint32, error) })
	if !ok {
		return ErrHelperExit
	}
	if code, err := status.ExitCode(); err != nil || code != 0 {
		return ErrHelperExit
	}
	resources, ok := process.host.(processResourceHost)
	if !ok {
		return delivery.ErrTrust
	}
	if err := resources.ValidateResourceUsage(); err != nil {
		return err
	}
	// Original deny-write/delete image custody lasts through this independent
	// post-exit check and the common retirement, even if the request just expired.
	retirementCtx, retirementCancel := context.WithTimeout(context.Background(), workerReapTimeout)
	defer retirementCancel()
	if err := lane.image.Validate(retirementCtx); err != nil {
		return err
	}
	completed = true
	return nil // deferred full Job/I-O/image retirement must still succeed
}

// The pure public-byte verifier is terminal before every application dispatcher,
// including recovery companions. It cannot repair files, enroll trust, start
// Wails/network producers or acquire a profile/vault store. Invalid internal
// flags and an absent exact bounded Job never fall through to another mode.
func DispatchDeliveryWorker(args []string) (handled bool, exitCode int) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-delivery-verifier") {
			handled = true
		}
	}
	if !handled {
		return false, 0
	}
	exitCode = 3
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed && exitCode == 0 {
			exitCode = 3
		}
	}()
	if len(args) != 2 || args[0] != deliveryWorkerArgument || !workerPipesReady() || !updateCodeCheckResourcesReady() {
		return true, 2
	}
	if _, err := nativeImagePin(args[1]); err != nil {
		return true, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), helperCaptureTimeout)
	defer cancel()
	if err := delivery.ServeWorker(ctx, os.Stdin, os.Stdout, args[1]); err != nil || ctx.Err() != nil || !updateCodeCheckResourcesReady() {
		return true, 3
	}
	completed = true
	return true, 0
}
