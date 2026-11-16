package render

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
)

// ProfileProvenance is populated by the authenticated local loader, never by
// channel JSON. Signature verification proves signing-key identity only, not
// live infrastructure, publisher availability or physical path independence.
type ProfileProvenance struct {
	Source            string
	Version           uint64
	ProfileEpoch      uint64
	EnvelopeSHA256    string
	SignerFingerprint string
	ExpiresAt         time.Time
	IssuedAt          time.Time
	ClockFloor        time.Time
	ObservedAt        time.Time
	Dependencies      metadata.DeliveryDependencies
	PinPolicySHA256   string
	PinPolicyVersion  string
}

// DescriptorSnapshot owns one coherent channels/split/provenance set. Readers
// receive fresh data; rendering does not consult a later UI/global split read.
type DescriptorSnapshot struct {
	Channels     []ChannelDescriptor
	SplitDirect  []string
	Profile      ProfileProvenance
	Restrictions []metadata.ChannelRestriction
}

func LoadDescriptorSnapshot() (*DescriptorSnapshot, error) {
	channels, err := parseDescriptors(descriptorsRaw)
	if err != nil {
		return nil, err
	}
	var embedded descriptorSet
	if err := json.Unmarshal(descriptorsRaw, &embedded); err != nil {
		return nil, err
	}
	snapshot := &DescriptorSnapshot{Channels: channels, SplitDirect: validateSplitDirect(embedded.SplitDirect), Profile: ProfileProvenance{Source: "embedded_configuration"}}
	if err := applyProfileSnapshot(snapshot); err != nil {
		return nil, err
	}
	var local []metadata.ChannelRestriction
	snapshot.Channels, local = projectProtectedPayloadEligibility(snapshot.Channels)
	snapshot.Restrictions = append(snapshot.Restrictions, local...)
	rememberConfiguredSplit(snapshot.SplitDirect)
	return snapshot, nil
}

func applyProfileSnapshot(snapshot *DescriptorSnapshot) error {
	store := envelopeStore
	if store == nil {
		return nil
	}
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	view, err := store.LoadRuntimeProfile(time.Now(), metadata.ProfileVersion{})
	if err != nil {
		return fmt.Errorf("metadata override: %w", err)
	}
	if view == nil {
		return nil
	}
	channels, err := DescriptorsFromRuntimeProfile(view)
	if err != nil {
		return err
	}
	snapshot.Channels = channels
	snapshot.Restrictions = view.Restrictions
	snapshot.SplitDirect = validateSplitDirect(view.SplitDirect)
	snapshot.Profile = ProfileProvenance{Source: "verified_metadata_signature", Version: view.Version.Counter, ProfileEpoch: view.Version.Epoch, EnvelopeSHA256: view.EnvelopeSHA256, SignerFingerprint: view.SignerFingerprint, ExpiresAt: view.ExpiresAt, IssuedAt: view.IssuedAt, ClockFloor: view.ClockFloor, ObservedAt: view.ObservedAt, Dependencies: view.Dependencies, PinPolicySHA256: view.PinPolicySHA256, PinPolicyVersion: view.PinPolicyVersion}
	return nil
}

// DescriptorsFromRuntimeProfile is the shared pure structural gate for an
// authenticated effective profile. It never loads/saves metadata or splitSet.
func DescriptorsFromRuntimeProfile(view *metadata.RuntimeProfile) ([]ChannelDescriptor, error) {
	if view == nil {
		return nil, fmt.Errorf("metadata: missing runtime profile")
	}
	channels := make([]ChannelDescriptor, len(view.Channels))
	for i, channel := range view.Channels {
		channels[i] = ChannelDescriptor(channel.Clone())
	}
	if _, err := parseSet(channels); err != nil {
		return nil, err
	}
	channels, _ = projectProtectedPayloadEligibility(channels)
	return channels, nil
}

// Copy descriptor elements before changing the local Enabled bit. Nested signed
// fields are not modified. Authenticated runtime views already carry their own
// restriction, so an already-disabled channel does not get counted twice.
func projectProtectedPayloadEligibility(channels []ChannelDescriptor) ([]ChannelDescriptor, []metadata.ChannelRestriction) {
	projected := append([]ChannelDescriptor(nil), channels...)
	var restrictions []metadata.ChannelRestriction
	for index, channel := range projected {
		if channel.Enabled && metadata.Channel(channel).CheckProtectedPayload() != nil {
			projected[index].Enabled = false
			restrictions = append(restrictions, metadata.ChannelRestriction{ChannelID: channel.ID, Reason: metadata.RestrictionPayloadConfidentiality, Disabled: true})
		}
	}
	return projected, restrictions
}
