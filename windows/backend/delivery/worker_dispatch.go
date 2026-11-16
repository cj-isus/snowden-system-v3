package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

// ServeWorker is called only by the fixed native pipe/Job dispatcher before
// Wails, store access or network producers. This is the same PrepareArtifacts
// implementation, not a separate verifier or a remote/public proof endpoint.
func ServeWorker(ctx context.Context, input io.Reader, output io.Writer, expectedDigest string) (finalErr error) {
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed && finalErr == nil {
			finalErr = ErrTrust
		}
	}()
	if contextError(ctx) != nil || !validHash(expectedDigest) {
		return ErrTrust
	}
	raw, key, err := workerReadFrame(input, workerRequestMagic, MaxWorkerRequestBytes)
	defer clear(key[:])
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != expectedDigest {
		return ErrTrust
	}
	var request workerInput
	if err := decodeWorkerJSON(raw, &request); err != nil || request.Schema != 1 || !validHash(request.Nonce) || request.State == nil || request.Package == nil ||
		len(request.Requests) < 1 || len(request.Requests) > MaxArtifactRequests || request.Now.IsZero() {
		return ErrTrust
	}
	source, err := request.State.Digest()
	if err != nil {
		return err
	}
	requests := make([]Request, len(request.Requests))
	for index, artifact := range request.Requests {
		requests[index] = Request{Kind: artifact.Kind, Path: artifact.Path, Bytes: artifact.Bytes}
	}
	progress, verifyErr := PrepareArtifacts(ctx, request.State, request.Package, requests, request.Now)
	result := workerResult{Schema: 1, Source: source}
	if progress != nil {
		if progress.source != source {
			return ErrTrust
		}
		result.Next = progress.next
	}
	if verifyErr != nil {
		for _, class := range []struct {
			err  error
			flag uint8
		}{{ErrTrust, workerErrTrust}, {ErrBudget, workerErrBudget}, {context.Canceled, workerErrCanceled}, {context.DeadlineExceeded, workerErrDeadline}} {
			if errors.Is(verifyErr, class.err) {
				result.Status |= class.flag
			}
		}
		if result.Status == 0 {
			result.Status = workerErrTrust // sanitized failure, never an unknown success
		}
	} else {
		if progress == nil || progress.next == nil || len(progress.proofs) != len(requests) {
			return ErrTrust
		}
		for _, artifact := range requests {
			proof := progress.proofs[artifact.Kind]
			if proof.Check(artifact.Kind, artifact.Path, artifact.Bytes, request.Now) != nil {
				return ErrTrust
			}
			sum := sha256.Sum256(artifact.Bytes)
			result.Proofs = append(result.Proofs, workerProof{proof.kind, proof.targetPath, hex.EncodeToString(sum[:]), proof.rootVersion, proof.expires, proof.pinPolicyRequired})
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > MaxWorkerReplyBytes {
		return ErrBudget
	}
	tag := workerResultMAC(key, digest, encoded)
	if err := workerWriteFrame(ctx, output, workerReplyMagic, encoded, tag, MaxWorkerReplyBytes); err != nil {
		return err
	}
	completed = true
	return nil
}
