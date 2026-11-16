package delivery

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"time"
)

// WorkerCall is an immutable private invocation, not a proof constructor. Its
// only transport operation writes the owned input into an original child pipe
// and reads a bounded authenticated reply. No secret or result setter is public.
type WorkerCall struct {
	input     *workerInput
	raw       []byte
	source    string
	digest    [32]byte
	key       [32]byte
	started   atomic.Bool
	completed atomic.Bool
	reply     []byte
	tag       [32]byte
	ioErr     error // published by completed
}

func (call *WorkerCall) Digest() string { return hex.EncodeToString(call.digest[:]) }

// Exchange is single-use. The compiled platform adapter must provide original
// private process pipes and confirm zero exit, image/Job limits and complete
// retirement before returning nil to PrepareWithTrustedWorker. This adapter is trusted
// application code, never a Wails argument, plugin, network response or hook.
// A completed pipe exchange alone is not process retirement/attestation.
func (call *WorkerCall) Exchange(ctx context.Context, input io.WriteCloser, output io.Reader) (err error) {
	if call == nil || input == nil || output == nil || !call.started.CompareAndSwap(false, true) {
		return ErrTrust
	}
	finished := false
	defer func() {
		if recovered := recover(); recovered != nil || !finished && err == nil {
			err = ErrTrust
		}
		call.ioErr = err
		call.completed.Store(true)
	}()
	if err = workerWriteFrame(ctx, input, workerRequestMagic, call.raw, call.key, MaxWorkerRequestBytes); err != nil {
		return err
	}
	if err = input.Close(); err != nil {
		return ErrTrust // original checked Close, no synthetic EOF receipt
	}
	call.reply, call.tag, err = workerReadFrame(output, workerReplyMagic, MaxWorkerReplyBytes)
	finished = true
	return err
}

// PrepareWithTrustedWorker is for the fixed compiled platform transport only. All
// Prepared values are reconstructed privately after the owned native transport
// retires, authenticates the exact invocation and validates the complete batch.
// It never falls back to the in-process SDK when transport/admission fails.
// The adapter is part of the proof-issuing trusted code base: it can access the
// pipe request and must not be replaced by an arbitrary callback. HMAC binds a
// call/reply; it does not attest a malicious adapter or prove native retirement.
func PrepareWithTrustedWorker(ctx context.Context, state *State, pack *Package, requests []Request, now time.Time, exchange func(context.Context, *WorkerCall) error) (progress *Progress, finalErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			progress, finalErr = nil, ErrTrust
		}
	}()
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if exchange == nil || !workspaceMu.TryLock() {
		return nil, ErrTrust
	}
	defer workspaceMu.Unlock()
	if retainedWorkspace != nil || !cacheReaderRetired() {
		return nil, ErrTrust // do not bypass an original legacy SDK I/O owner
	}
	call, err := newWorkerCall(state, pack, requests, now)
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := exchange(ctx, call); err != nil {
		return nil, errors.Join(ErrTrust, err)
	}
	if !call.completed.Load() || call.ioErr != nil {
		return nil, ErrTrust
	}
	progress, finalErr = call.progress()
	// Do not clear/mutate call buffers on a failed/unfinished native exchange:
	// its retained I/O goroutine still owns them. Here completed is established.
	clear(call.key[:])
	if err := contextError(ctx); err != nil {
		if progress != nil {
			progress = &Progress{source: progress.source, next: progress.next}
		}
		finalErr = errors.Join(finalErr, err)
	}
	return progress, finalErr
}

func newWorkerCall(state *State, pack *Package, requests []Request, now time.Time) (*WorkerCall, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	if now.IsZero() || now.Before(state.ClockFloor) {
		return nil, ErrTrust
	}
	if len(requests) < 1 || len(requests) > MaxArtifactRequests {
		return nil, ErrBudget
	}
	owned, err := pack.snapshot()
	if err != nil {
		return nil, err
	}
	input := &workerInput{Schema: 1, State: state.Clone(), Package: owned, Now: now.UTC()}
	seen := map[Kind]bool{}
	for _, request := range requests {
		if err := request.validate(); err != nil {
			return nil, err
		}
		if seen[request.Kind] {
			return nil, ErrTrust
		}
		seen[request.Kind] = true
		input.Requests = append(input.Requests, workerArtifact{request.Kind, request.Path, append([]byte(nil), request.Bytes...)})
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, ErrTrust
	}
	input.Nonce = hex.EncodeToString(nonce[:])
	call := &WorkerCall{input: input}
	if _, err := rand.Read(call.key[:]); err != nil {
		return nil, ErrTrust
	}
	call.source, err = input.State.Digest()
	if err != nil {
		return nil, err
	}
	call.raw, err = json.Marshal(input)
	if err != nil || len(call.raw) > MaxWorkerRequestBytes {
		return nil, ErrBudget
	}
	call.digest = sha256.Sum256(call.raw)
	return call, nil
}

func (call *WorkerCall) progress() (*Progress, error) {
	want := workerResultMAC(call.key, call.digest, call.reply)
	if !hmac.Equal(want[:], call.tag[:]) {
		return nil, ErrTrust
	}
	var result workerResult
	if err := decodeWorkerJSON(call.reply, &result); err != nil || result.Schema != 1 || result.Source != call.source {
		return nil, ErrTrust
	}
	verifyErr, validStatus := workerStatusError(result.Status)
	if !validStatus || result.Next == nil && (verifyErr == nil || len(result.Proofs) != 0) {
		return nil, ErrTrust
	}
	if result.Next == nil {
		return nil, verifyErr
	}
	if result.Next.Validate() != nil || result.Next.AnchorSHA256 != call.input.State.AnchorSHA256 || !result.Next.ClockFloor.Equal(call.input.Now) {
		return nil, ErrTrust
	}
	progress := &Progress{source: call.source, next: result.Next}
	if verifyErr != nil {
		if len(result.Proofs) != 0 {
			return nil, ErrTrust // failure cannot transport a partial artifact grant
		}
		return progress, verifyErr
	}
	if len(result.Proofs) != len(call.input.Requests) {
		return nil, ErrTrust
	}
	proofs := make(map[Kind]*Prepared, len(result.Proofs))
	for index, fact := range result.Proofs {
		request := call.input.Requests[index]
		sum := sha256.Sum256(request.Bytes)
		if fact.Kind != request.Kind || fact.Path != request.Path || fact.SHA256 != hex.EncodeToString(sum[:]) || fact.RootVersion < 1 || !fact.Expires.After(call.input.Now) ||
			index > 0 && (fact.RootVersion != result.Proofs[0].RootVersion || fact.PinPolicyRequired != result.Proofs[0].PinPolicyRequired) {
			return nil, ErrTrust
		}
		proofs[fact.Kind] = &Prepared{source: call.source, next: result.Next.Clone(), kind: fact.Kind, targetPath: fact.Path, artifact: request.Bytes,
			rootVersion: fact.RootVersion, expires: fact.Expires, pinPolicyRequired: fact.PinPolicyRequired}
	}
	progress.proofs, progress.proof = proofs, proofs[call.input.Requests[0].Kind]
	return progress, nil
}

func workerStatusError(status uint8) (error, bool) {
	if status & ^(workerErrTrust|workerErrBudget|workerErrCanceled|workerErrDeadline) != 0 {
		return ErrTrust, false
	}
	var causes []error
	for _, class := range []struct {
		err  error
		flag uint8
	}{{ErrTrust, workerErrTrust}, {ErrBudget, workerErrBudget}, {context.Canceled, workerErrCanceled}, {context.DeadlineExceeded, workerErrDeadline}} {
		if status&class.flag != 0 {
			causes = append(causes, class.err)
		}
	}
	return errors.Join(causes...), true
}
