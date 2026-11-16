package repository

import (
	"context"
	"errors"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
)

type preparedPublisherRelease struct {
	source         *publisherSnapshot
	progress       *promotionProgress
	roots          []publisherRoot
	prefixRetained bool
}

// Shared by promotion and explicit candidate supplementation. A later local
// artifact failure cannot roll back an independently authenticated SDK prefix.
// The caller owns the existing stable writer lease throughout this operation.
func (s *publisherStore) preparePublisherRelease(ctx context.Context, source *publisherSnapshot, candidate, authority *promotionInput, initialRoot string, clock *metadata.ObservedClock) (*preparedPublisherRelease, error) {
	if ctx == nil || source == nil || candidate == nil || authority == nil || clock == nil {
		return nil, delivery.ErrTrust
	}
	policy, err := publisherApplicationPolicy(authority.raw)
	if err != nil {
		return nil, err
	}
	var baseline *delivery.State
	var previousPin *metadata.RetainedPinPolicy
	if source.state != nil {
		if source.state.Authority != authority.approval || initialRoot != "retained" {
			return nil, errors.New("repository: retained authority changed or re-enrollment requested")
		}
		baseline, initialRoot = source.state.Delivery.Clone(), ""
		previousPin = source.state.PinPolicy.Clone()
	} else if !canonicalHash(initialRoot) {
		return nil, errors.New("repository: first enrollment needs independent root SHA256")
	}
	now, err := clock.Now(publisherClockFloor(source.state))
	if err != nil {
		return nil, err
	}
	progress, verifyErr := verifyRelease(ctx, candidate.raw, candidate.approval, baseline, initialRoot, policy, previousPin, now)
	out := &preparedPublisherRelease{source: source, progress: progress}
	if progress != nil {
		out.roots, err = s.stageRoots(ctx, source, progress)
		if err != nil {
			return out, errors.Join(verifyErr, err)
		}
	}
	inputs := func() error { return errors.Join(candidate.check(), authority.check()) }
	out.source, out.prefixRetained, err = s.retainPrefix(source, progress, out.roots, inputs)
	if err != nil {
		if out.prefixRetained {
			err = errors.Join(errors.New("repository: verified prefix committed; publisher reread failed"), err)
		}
		return out, errors.Join(verifyErr, err)
	}
	if verifyErr != nil {
		return out, verifyErr
	}
	if progress == nil || progress.accepted == nil || progress.next == nil {
		return out, delivery.ErrTrust
	}
	return out, nil
}
