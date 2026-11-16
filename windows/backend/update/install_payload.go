package update

import (
	"context"
	"errors"
)

const maxPairStagingWrites = MaxOwnedPairStagingFiles

var errPairStagingClose = errors.New("update: staging file close unconfirmed")

// PairTransaction serializes these resources. The native root retains each
// original writer independently; a later generic "closed" error cannot supply
// confirmation after an uncertain native close or panic.
type pairStagingWrite struct {
	file     StagingFile
	closeErr error
	complete bool // exact writer completed Sync + Close without cancellation
}

func (resource *pairStagingWrite) Close() error {
	if resource.closeErr != nil {
		return resource.closeErr
	}
	if resource.file == nil {
		return nil
	}
	resource.closeErr = errPairStagingClose // preserve uncertainty even if the native delegate panics
	if err := resource.file.Close(); err != nil {
		return resource.closeErr
	}
	resource.file, resource.closeErr = nil, nil
	return nil
}

func (t *PairTransaction) retireWrites() error {
	var result error
	for _, resource := range t.writes {
		result = errors.Join(result, resource.Close())
	}
	if t.creationCustody != nil {
		result = errors.Join(result, t.creationCustody.ReleaseStagingFiles())
	}
	return result
}

func (t *PairTransaction) pinPayload(ctx context.Context, path string, allowed ...Payload) (PayloadFile, error) {
	if t.payloadCustody == nil || len(allowed) < 1 || len(allowed) > 2 {
		return nil, ErrPairRecoveryRequired
	}
	for _, file := range t.payloads {
		if file.Path() == path {
			for _, payload := range allowed {
				if file.Payload() == payload {
					return file, file.Validate(ctx)
				}
			}
			return file, ErrPairRecoveryRequired
		}
	}
	if len(t.payloads) >= MaxOwnedPairPayloads {
		return nil, ErrPairRecoveryRequired
	}
	file, err := t.payloadCustody.PinPayload(ctx, path, allowed)
	if file != nil {
		t.payloads = append(t.payloads, file) // keep exact partial receipt even on error
	}
	if err != nil || file == nil {
		return file, errors.Join(ErrPairRecoveryRequired, err)
	}
	return file, file.Validate(ctx)
}
