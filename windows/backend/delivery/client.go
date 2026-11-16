package delivery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	updater "github.com/snowden-system/windows/backend/delivery/internal/tufupdater"
	tuf "github.com/theupdateframework/go-tuf/v2/metadata"
	tufconfig "github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata"
)

// This is an identifier for the in-memory fetcher, never a contacted endpoint.
const packageURL = "https://snowden.invalid/metadata/"

type packageFetcher struct {
	ctx       context.Context
	pack      *Package
	calls     int
	bytes     int64
	delivered map[string]bool
}

func (f *packageFetcher) DownloadFile(rawURL string, maxLength int64, _ time.Duration) ([]byte, error) {
	if err := contextError(f.ctx); err != nil {
		return nil, err
	}
	f.calls++
	if f.calls > MaxRecords || maxLength <= 0 {
		return nil, ErrBudget
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host != "snowden.invalid" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || !strings.HasPrefix(u.Path, "/metadata/") {
		return nil, ErrTrust
	}
	name := strings.TrimPrefix(u.Path, "/metadata/")
	if !metadataName.MatchString(name) {
		return nil, ErrTrust
	}
	data, present := f.pack.Metadata[name]
	if !present {
		// TUF stops sequential root rotation on a 404. Missing required role
		// metadata also fails through the normal library workflow.
		return nil, &tuf.ErrDownloadHTTP{StatusCode: 404, URL: rawURL}
	}
	if int64(len(data)) > maxLength || len(data) > MaxMetadataBytes {
		return nil, ErrBudget
	}
	f.bytes += int64(len(data))
	if f.bytes > MaxPackageBytes {
		return nil, ErrBudget
	}
	if f.delivered == nil {
		f.delivered = make(map[string]bool)
	}
	f.delivered[name] = true
	return bytes.Clone(data), nil
}

// Prepare uses an isolated upstream Updater per operation. No network, shared
// logger, UnsafeLocalMode or test-only clock override is used. now must be a
// current effective local-clock observation; it is not authenticated UTC.
func Prepare(ctx context.Context, state *State, pack *Package, request Request, now time.Time) (progress *Progress, err error) {
	return PrepareArtifacts(ctx, state, pack, []Request{request}, now)
}

// PrepareArtifacts uses one Updater for the bounded set. Every proof is bound
// to the same source and final retained state; a partial target set returns
// verified prefix only, never a subset of artifact proofs.
func PrepareArtifacts(ctx context.Context, state *State, pack *Package, requests []Request, now time.Time) (progress *Progress, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			progress = nil // do not persist a partially observed panic outcome
			err = fmt.Errorf("%w: verifier panic (%T)", ErrTrust, recovered)
		}
	}()
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if err := state.Validate(); err != nil {
		return nil, err
	}
	state = state.Clone() // validate resource bounds before copying caller buffers
	if now.IsZero() || now.Before(state.ClockFloor) {
		return nil, ErrTrust
	}
	if len(requests) == 0 || len(requests) > MaxArtifactRequests {
		return nil, ErrBudget
	}
	checked := make([]Request, len(requests))
	seen := map[Kind]bool{}
	for i, request := range requests {
		if seen[request.Kind] {
			return nil, ErrTrust
		}
		seen[request.Kind] = true
		if err := request.validate(); err != nil {
			return nil, err
		}
		checked[i] = Request{Kind: request.Kind, Path: request.Path, Bytes: bytes.Clone(request.Bytes)}
	}
	owned, err := pack.snapshot()
	if err != nil {
		return nil, err
	}
	if err := guardMetadata("root.json", state.Root); err != nil {
		return nil, err
	}
	source, err := state.Digest()
	if err != nil {
		return nil, err
	}
	work, err := openWorkspace(ctx, state.Cache)
	if work != nil {
		defer func() {
			retirementErr := errors.Join(work.Close(), contextError(ctx))
			if retirementErr != nil {
				if progress != nil {
					// Restriction-only progress survives late retirement/cancel
					// failure, but no artifact authority may escape that failure.
					progress = &Progress{source: progress.source, next: progress.next}
				}
				err = errors.Join(err, ErrTrust, retirementErr)
			}
		}()
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w: metadata workspace unavailable", ErrTrust), err)
	}
	metadataDir, targetsDir := work.Paths()
	cfg, err := tufconfig.New(packageURL, state.Root)
	if err != nil {
		return nil, fmt.Errorf("%w: verifier configuration", ErrTrust)
	}
	fetcher := &packageFetcher{ctx: ctx, pack: owned}
	cfg.Fetcher = fetcher
	cfg.LocalMetadataDir, cfg.LocalTargetsDir = metadataDir, targetsDir
	cfg.MaxRootRotations, cfg.MaxDelegations = MaxRootRotations, MaxDelegations
	cfg.RootMaxLength, cfg.TimestampMaxLength = MaxMetadataBytes, 64<<10
	cfg.SnapshotMaxLength, cfg.TargetsMaxLength = MaxMetadataBytes, MaxMetadataBytes
	cfg.UnsafeLocalMode, cfg.DisableLocalCache = false, false
	client, err := updater.New(cfg, work.storage)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%w: initial root rejected", ErrTrust), work.storage.Check())
	}
	if err := work.storage.Check(); err != nil {
		return nil, err
	}
	refreshErr := client.Refresh() // complete upstream root/timestamp/snapshot/targets workflow
	trusted := client.GetTrustedMetadataSet()
	next, retainErr := retainedState(state, fetcher, client.GetSuccessfulMetadataSet(), now)
	if next != nil {
		progress = &Progress{source: source, next: next}
	}
	if retainErr != nil {
		return progress, retainErr
	}
	if storageErr := work.storage.Check(); storageErr != nil {
		return progress, errors.Join(storageErr, refreshErr)
	}
	if err := contextError(ctx); err != nil {
		return progress, err
	}
	oldRoot, parseErr := tuf.Root().FromBytes(state.Root)
	if parseErr != nil {
		return nil, ErrTrust
	}
	if trusted.Root.Signed.Version-oldRoot.Signed.Version >= MaxRootRotations {
		// Upstream stops its loop at the configured ceiling. Keep the verified
		// prefix, but require another bounded pass before asserting a full tail.
		return progress, fmt.Errorf("%w: root rotation continuation required", ErrBudget)
	}
	if refreshErr != nil {
		return progress, fmt.Errorf("%w: %w", ErrTrust, refreshErr)
	}
	pinPolicyRequired := trusted.Targets[tuf.TARGETS] != nil && trusted.Targets[tuf.TARGETS].Signed.Targets[PinPolicyPath] != nil
	if seen[Profile] && pinPolicyRequired && !seen[PinPolicy] {
		return progress, fmt.Errorf("%w: authenticated repository requires its pin-policy target", ErrTrust)
	}
	prepared := make(map[Kind]*Prepared, len(checked))
	for _, request := range checked {
		info, lookupErr := client.GetTargetInfo(request.Path)
		trusted = client.GetTrustedMetadataSet()
		next, retainErr = retainedState(state, fetcher, client.GetSuccessfulMetadataSet(), now)
		if next != nil {
			progress.next = next
		}
		if retainErr != nil {
			return progress, retainErr
		}
		if storageErr := work.storage.Check(); storageErr != nil {
			return progress, errors.Join(storageErr, lookupErr)
		}
		if err := contextError(ctx); err != nil {
			return progress, err
		}
		if lookupErr != nil || info == nil {
			return progress, fmt.Errorf("%w: artifact metadata lookup failed", ErrTrust)
		}
		expires, policyErr := finalPolicy(trusted, request.Kind, now)
		if policyErr != nil {
			return progress, policyErr
		}
		if len(info.Hashes["sha256"]) != 32 || info.Length != int64(len(request.Bytes)) {
			return progress, ErrTrust
		}
		if err := info.VerifyLengthHashes(request.Bytes); err != nil {
			return progress, fmt.Errorf("%w: artifact hash/length mismatch", ErrTrust)
		}
		prepared[request.Kind] = &Prepared{source: source, kind: request.Kind, targetPath: request.Path, artifact: request.Bytes, rootVersion: trusted.Root.Signed.Version, expires: expires, pinPolicyRequired: pinPolicyRequired}
	}
	if err := contextError(ctx); err != nil {
		return progress, err
	}
	if err := completeRetainedState(progress.next, trusted); err != nil {
		return progress, err
	}
	for _, proof := range prepared {
		proof.next = progress.next.Clone()
	}
	if err := work.storage.Check(); err != nil {
		return progress, err
	}
	progress.proofs = prepared
	progress.proof = prepared[checked[0].Kind]
	return progress, nil
}

func finalPolicy(trusted trustedmetadata.TrustedMetadata, kind Kind, now time.Time) (time.Time, error) {
	if err := rootPolicy(trusted.Root); err != nil {
		return time.Time{}, err
	}
	if trusted.Timestamp == nil || trusted.Snapshot == nil {
		return time.Time{}, ErrTrust
	}
	expires := trusted.Root.Signed.Expires
	for _, at := range []time.Time{trusted.Timestamp.Signed.Expires, trusted.Snapshot.Signed.Expires} {
		if at.Before(expires) {
			expires = at
		}
	}
	role := "profiles"
	if kind == UpdateManifest {
		role = "updates"
	}
	roles := []string{tuf.TARGETS, role}
	if kind == ProfileEpoch || kind == PinPolicy {
		roles = []string{tuf.TARGETS} // reserved direct target, offline threshold >=2
	}
	for _, name := range roles {
		meta := trusted.Targets[name]
		if err := targetsPolicy(name, meta); err != nil {
			return time.Time{}, err
		}
		if meta.Signed.Expires.Before(expires) {
			expires = meta.Signed.Expires
		}
	}
	// A profile delegate may not also be one of the offline root/targets keys.
	delegations := trusted.Targets[tuf.TARGETS].Signed.Delegations
	for _, delegated := range delegations.Roles {
		if delegated.Name != "profiles" {
			continue
		}
		profileKeys, _ := roleKeys(delegations.Keys, delegated.KeyIDs, delegated.Threshold, 1)
		for _, name := range []string{tuf.ROOT, tuf.TARGETS} {
			rootRole := trusted.Root.Signed.Roles[name]
			offlineKeys, _ := roleKeys(trusted.Root.Signed.Keys, rootRole.KeyIDs, rootRole.Threshold, 2)
			if overlaps(profileKeys, offlineKeys) {
				return time.Time{}, ErrTrust
			}
		}
	}
	// Extra local-clock gate: no production call to UnsafeSetRefTime. This can
	// be stricter than upstream's wall-clock reference; it cannot extend expiry.
	if !expires.After(now) {
		return time.Time{}, fmt.Errorf("%w: delivery metadata expired", ErrTrust)
	}
	return expires, nil
}
