package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/backend/onboarding"
)

type profileImportIntent struct {
	recovery          bool
	epochRecovery     bool
	epochEnrollment   bool
	commit            bool
	approvedTarget    string
	approvedRoot      string
	approvedPinPolicy string             // exact independent first-enrollment policy baseline only
	expectedPreview   *OnboardingPreview // optional exact source/intent approval from downloaded preview
	approvedUntil     time.Time          // backend-owned source token expiry only
	requestContext    context.Context    // original private App/request lifetime and negative intent scope; never JSON authority
}

type bundleAuthorityInput struct {
	store      *metadata.Store
	payload    *onboarding.BundlePayload
	envelope   *metadata.Envelope
	retained   *metadata.AcceptedProfileState
	policy     *metadata.TrustPolicy
	now        time.Time
	intent     profileImportIntent
	delivered  *bundleDelivery
	rawProfile []byte
}

type bundleAuthority struct {
	policy            *metadata.TrustPolicy
	accepted          *metadata.AcceptedProfileState
	mode              string
	retiredPublishers []string
	recoveryIssuer    string
}

func bundleProfileKeys(keys []onboarding.TrustedKey) (map[string]ed25519.PublicKey, error) {
	if len(keys) > metadata.MaxTrustKeys {
		return nil, errors.New("onboarding: too many public keys")
	}
	out := make(map[string]ed25519.PublicKey, len(keys))
	for _, key := range keys {
		pub, err := hex.DecodeString(key.Public)
		if err != nil || len(pub) != ed25519.PublicKeySize || len(key.Comment) > 256 {
			return nil, errors.New("onboarding: invalid public key")
		}
		id := metadata.KeyIDFor(ed25519.PublicKey(pub))
		if id != key.KeyID || out[id] != nil {
			return nil, errors.New("onboarding: duplicate or mismatched key ID")
		}
		out[id] = ed25519.PublicKey(pub)
	}
	return out, nil
}

// Caller owns App lifecycle, local serialization and a coherent native profile
// read/writer scope. This phase decides authority using public data only; it
// never writes files, invokes SDK client I/O or touches the vault.
func (in bundleAuthorityInput) prepare() (*bundleAuthority, error) {
	if err := in.delivered.check(in.policy, in.rawProfile, in.now); err != nil {
		return nil, err
	}
	if err := checkBootstrapPinApproval(in.policy, in.payload, in.intent); err != nil {
		return nil, err
	}
	incoming, err := bundleProfileKeys(in.payload.TrustedKeys)
	if err != nil {
		return nil, err
	}
	if in.envelope.MetadataVersion == 0 {
		return nil, errors.New("onboarding: metadata version must be positive")
	}
	out := &bundleAuthority{policy: in.policy.Scoped(), mode: "installed_profile_authority"}
	if in.intent.epochEnrollment {
		if in.intent.recovery || in.intent.epochRecovery || in.intent.approvedRoot == "" || len(in.payload.Recovery) != 0 || len(in.payload.EpochGrant) == 0 || in.delivered == nil || !in.delivered.enroll || len(incoming) != 1 || incoming[in.envelope.KeyID] == nil {
			return nil, errors.New("onboarding: initial epoch enrollment requires an explicit root and exact delivered signer, not recovery/reset")
		}
		out.policy, out.accepted, err = in.store.PrepareEpochEnrollment(metadata.EpochEnrollmentRequest{
			Next: in.envelope, SourcePolicy: in.policy, Enrollment: in.delivered.source,
			ApprovedRootSHA256: in.intent.approvedRoot,
			ProfileProof:       in.delivered.progress.ProofFor(delivery.Profile), TargetPath: in.delivered.path,
			ProfileBytes: in.rawProfile, GrantProof: in.delivered.progress.ProofFor(delivery.ProfileEpoch),
			GrantBytes: in.payload.EpochGrant, Now: in.now,
		})
		if err != nil {
			return nil, err
		}
		if in.intent.commit && in.intent.approvedTarget != out.accepted.EnvelopeSHA256 {
			return nil, errors.New("onboarding: подтвердите target SHA256 из превью initial epoch enrollment")
		}
		out.mode = "explicit_initial_profile_epoch_enrollment"
		for _, key := range out.policy.Keys {
			if key.Role == metadata.RoleProfile && key.Retired {
				out.retiredPublishers = append(out.retiredPublishers, key.KeyID)
			}
		}
	} else if in.intent.epochRecovery {
		if in.intent.recovery || in.intent.approvedRoot != "" || len(in.payload.Recovery) != 0 || len(in.payload.EpochGrant) == 0 || in.delivered == nil || in.delivered.enroll {
			return nil, errors.New("onboarding: epoch recovery cannot enroll root or use a single recovery permit")
		}
		out.policy, out.accepted, err = in.store.PrepareEpochRecovery(in.envelope, in.policy, in.delivered.progress.ProofFor(delivery.Profile), in.delivered.path, in.rawProfile, in.delivered.progress.ProofFor(delivery.ProfileEpoch), in.payload.EpochGrant, in.now)
		if err != nil {
			return nil, err
		}
		if in.intent.commit && in.intent.approvedTarget != out.accepted.EnvelopeSHA256 {
			return nil, errors.New("onboarding: подтвердите точный target SHA256 из превью epoch recovery")
		}
		out.mode = "explicit_profile_epoch_recovery"
		for _, key := range out.policy.Keys {
			if key.Role == metadata.RoleProfile && key.Retired {
				out.retiredPublishers = append(out.retiredPublishers, key.KeyID)
			}
		}
	} else if in.intent.recovery {
		if len(in.payload.EpochGrant) != 0 {
			return nil, errors.New("onboarding: recovery permit cannot carry an epoch transition")
		}
		permit, err := metadata.DecodeRecoveryPermit(in.payload.Recovery)
		if err != nil {
			return nil, err
		}
		if in.intent.commit && in.intent.approvedTarget != permit.TargetSHA256 {
			return nil, errors.New("onboarding: подтвердите точный target SHA256 из превью восстановления")
		}
		if in.policy.Delivery != nil {
			out.policy, out.accepted, err = in.store.PrepareDeliveredRecovery(in.envelope, permit, in.policy, in.delivered.progress.Proof(), in.delivered.path, in.rawProfile, in.now)
		} else {
			out.policy, out.accepted, err = in.store.PrepareProfileRecovery(in.envelope, permit, in.policy, in.now)
		}
		if err != nil {
			return nil, err
		}
		out.mode, out.recoveryIssuer = "explicit_offline_recovery", permit.KeyID
		for _, key := range out.policy.Keys {
			if key.Role == metadata.RoleProfile && key.Retired {
				out.retiredPublishers = append(out.retiredPublishers, key.KeyID)
			}
		}
	} else {
		if len(in.payload.Recovery) != 0 {
			return nil, errors.New("onboarding: recovery permit требует отдельного явного восстановления")
		}
		if len(in.payload.EpochGrant) != 0 {
			// A portable issuer package also carries the current grant for a
			// fresh recipient. An already enrolled PC may use it as a reference
			// only; this path must never assign a signer or advance/reset epochs.
			if in.delivered == nil || in.delivered.enroll || in.policy.Delivery == nil || !in.retained.HasCheckpoint() || in.retained.ProfileEpoch == 0 || in.envelope.ProfileEpoch != in.retained.ProfileEpoch {
				return nil, errors.New("onboarding: a different or initial epoch requires explicit enrollment/recovery")
			}
			grant, err := metadata.CheckEpochProofs(in.policy, in.delivered.progress.ProofFor(delivery.Profile), in.delivered.path, in.rawProfile, in.delivered.progress.ProofFor(delivery.ProfileEpoch), in.payload.EpochGrant, in.now)
			if err != nil {
				return nil, err
			}
			active := in.policy.VerificationKeys(metadata.RoleProfile)[in.envelope.KeyID]
			if len(active) == 0 || grant.ToEpoch != in.retained.ProfileEpoch || grant.Namespace != in.retained.EpochNamespace || grant.ProfilePublic != hex.EncodeToString(active) {
				return nil, errors.New("onboarding: current-epoch reference differs from retained namespace/signer")
			}
		}
		_, present, err := in.store.LoadEnvelope()
		if err != nil {
			return nil, err
		}
		profileKeys := in.policy.VerificationKeys(metadata.RoleProfile)
		// An explicit, verified root bootstrap on a clean installation binds
		// only the signer of the exact delivered profile. Seed profile keys may
		// exist without accepted history; keep them retired, never promote a
		// foreign/retired role and never grant extra unsigned bundle keys.
		if in.delivered != nil && in.delivered.enroll && in.retained.Version == 0 && !present {
			if len(incoming) != 1 || incoming[in.envelope.KeyID] == nil {
				return nil, errors.New("onboarding: initial root enrollment permits only the exact delivered profile signer")
			}
			for i := range out.policy.Keys {
				key := &out.policy.Keys[i]
				if key.Role == metadata.RoleProfile && !key.Retired && key.KeyID != in.envelope.KeyID {
					key.Retired = true
					out.retiredPublishers = append(out.retiredPublishers, key.KeyID)
				}
			}
			if profileKeys[in.envelope.KeyID] == nil {
				out.policy, err = out.policy.WithKey(incoming[in.envelope.KeyID], metadata.RoleProfile)
				if err != nil {
					return nil, err // no retired revival or update/recovery promotion
				}
			}
			profileKeys = out.policy.VerificationKeys(metadata.RoleProfile)
			out.mode = "initial_root_profile_enrollment"
		} else if len(profileKeys) == 0 && in.retained.Version == 0 && !present && len(in.policy.Keys) == 0 {
			// Legacy enrollment requires an empty installation. Missing authority
			// beside retained history is never a fresh bootstrap/reset.
			for _, pub := range incoming {
				out.policy, err = out.policy.WithKey(pub, metadata.RoleProfile)
				if err != nil {
					return nil, err
				}
			}
			profileKeys = out.policy.VerificationKeys(metadata.RoleProfile)
			out.mode = "initial_profile_enrollment"
		} else {
			for id, pub := range incoming {
				if !bytes.Equal(profileKeys[id], pub) {
					return nil, errors.New("onboarding: новый/retired publisher требует pre-enrolled recovery authority; обычный импорт не меняет доверие")
				}
			}
		}
		if err := in.envelope.Verify(profileKeys); err != nil {
			return nil, err
		}
		if in.policy.Delivery != nil {
			out.accepted, err = in.store.PrepareDeliveredAcceptance(in.envelope, profileKeys, in.delivered.progress.Proof(), in.delivered.path, in.rawProfile, in.now)
		} else {
			out.accepted, err = in.store.PrepareAcceptance(in.envelope, profileKeys, in.now)
		}
		if err != nil {
			return nil, err
		}
	}
	profileKeys := out.policy.VerificationKeys(metadata.RoleProfile)
	for id, pub := range incoming {
		if !bytes.Equal(profileKeys[id], pub) {
			return nil, errors.New("onboarding: bundle key is outside the authorized target profile role")
		}
	}
	if in.delivered != nil {
		out.policy.Delivery = in.delivered.progress.NextState()
		out.accepted.DeliveryAnchorSHA256 = out.policy.Delivery.AnchorSHA256
		if err := metadata.CheckDeliveryBinding(out.policy, out.accepted); err != nil {
			return nil, err
		}
		if in.delivered.enroll && out.mode != "initial_root_profile_enrollment" && out.mode != "explicit_initial_profile_epoch_enrollment" {
			out.mode = "explicit_delivery_enrollment"
		}
		out.accepted, err = in.delivered.applyPinPolicy(out.policy, out.accepted, in.envelope, in.rawProfile, in.now)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
