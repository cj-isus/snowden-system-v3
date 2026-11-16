package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
)

// Owned public input for both export formats. policySHA is the authority
// digest shown in previews; policySource seals all observed policy fields,
// including schema/comments/cache. Raw envelope bytes are never reconstructed.
type profileExportSource struct {
	store         *metadata.Store
	policy        *metadata.TrustPolicy
	state         *metadata.AcceptedProfileState
	raw           []byte
	policySHA     string
	checkpoint    string
	policySource  string
	policyPresent bool
}

// Caller owns a native read/writer scope and local profile serialization.
// The helper never acquires that scope itself, calls the vault, or runs SDK.
func readProfileExportSourceInScope(store *metadata.Store) (*profileExportSource, error) {
	if store == nil {
		return nil, metadata.ErrTrustRole
	}
	policy, present, err := store.LoadTrustPolicy()
	if err != nil {
		return nil, err
	}
	state, err := metadata.LoadAcceptedProfile(store.Dir)
	if err != nil {
		return nil, err
	}
	if err := metadata.CheckDeliveryBinding(policy, state); err != nil {
		return nil, err
	}
	raw, envelopePresent, err := store.ReadEnvelopeBytes()
	if err != nil {
		return nil, errors.Join(metadata.ErrTrustRole, err)
	}
	if !envelopePresent || len(raw) == 0 {
		return nil, errors.Join(metadata.ErrTrustRole, errors.New("onboarding: установленный envelope отсутствует или пуст"))
	}
	policySHA, err := policy.Digest()
	if err != nil {
		return nil, err
	}
	policyBytes, err := json.Marshal(policy) // retain observed schema; no Scoped promotion
	if err != nil {
		return nil, err
	}
	checkpoint, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	if err := filetxn.Check(store.ImportJournal()); err != nil {
		return nil, err
	}
	return &profileExportSource{store: store, policy: policy, state: state, raw: raw,
		policySHA: policySHA, checkpoint: publicSHA(checkpoint), policySource: publicSHA(policyBytes), policyPresent: present}, nil
}

// Captures under the existing initialized shared lock. Retirement failure
// withholds every captured value; a missing lock cannot become absent data.
func captureProfileExportSource(store *metadata.Store, inspect func(*profileExportSource) error) (*profileExportSource, error) {
	if store == nil {
		return nil, metadata.ErrTrustRole
	}
	var source *profileExportSource
	err := filetxn.WithRead(store.ImportJournal(), func() error {
		var err error
		source, err = readProfileExportSourceInScope(store)
		if err != nil {
			return err
		}
		if inspect != nil {
			return inspect(source) // read-only; no vault/SDK/nested native scope
		}
		return nil
	})
	if err != nil {
		return nil, errors.Join(metadata.ErrTrustRole, err)
	}
	return source, nil
}

// Caller already owns the native scope, including final vault snapshot checks.
// Do not replace this with check(): WithRead is deliberately not reentrant.
func (s *profileExportSource) checkInScope() error {
	if s == nil || s.store == nil {
		return metadata.ErrTrustRole
	}
	current, err := readProfileExportSourceInScope(s.store)
	if err != nil {
		return err
	}
	if current.policyPresent != s.policyPresent || current.policySource != s.policySource || current.checkpoint != s.checkpoint || !bytes.Equal(current.raw, s.raw) {
		return errors.New("onboarding: установленный профиль или доверие изменились; повторите подготовку переноса")
	}
	return nil
}

// The optional callback is read-only and remains under the same native scope
// as comparison. It must not commit, enter the vault, or acquire another scope.
func (s *profileExportSource) withCurrentSource(use func() error) error {
	if s == nil || s.store == nil {
		return metadata.ErrTrustRole
	}
	err := filetxn.WithRead(s.store.ImportJournal(), func() error {
		if err := s.checkInScope(); err != nil {
			return err
		}
		if use != nil {
			return use()
		}
		return nil
	})
	if err != nil {
		return errors.Join(metadata.ErrTrustRole, err)
	}
	return nil
}

func (s *profileExportSource) check() error { return s.withCurrentSource(nil) }
