package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

type promotionInput struct {
	path, approval string
	raw            []byte
}

func outsidePublisherPath(filename string, store *publisherStore) (string, error) {
	if filename == "" || store == nil {
		return "", errors.New("repository: explicit outside-store path required")
	}
	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.VolumeName(store.root), filepath.VolumeName(abs)) {
		rel, err := filepath.Rel(store.root, abs)
		if err != nil || rel == "." || filepath.IsLocal(rel) {
			return "", errors.New("repository: candidate/authority paths must be outside publisher store")
		}
	}
	return abs, publisherPath(abs)
}

func readPromotionInput(filename, approval string, limit int64, store *publisherStore) (*promotionInput, error) {
	abs, err := outsidePublisherPath(filename, store)
	if err != nil {
		return nil, err
	}
	raw, err := publisherfiles.Read(abs, limit)
	if err != nil {
		return nil, err
	}
	if !canonicalHash(approval) || !approved(raw, approval) {
		return nil, errors.New("repository: independent input approval mismatch")
	}
	return &promotionInput{path: abs, approval: approval, raw: raw}, nil
}

func (input *promotionInput) check() error {
	if err := publisherPath(input.path); err != nil {
		return err
	}
	raw, err := publisherfiles.Read(input.path, int64(len(input.raw)))
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, input.raw) || !approved(raw, input.approval) {
		return errors.New("repository: approved external input changed")
	}
	return nil
}

// retainPrefix is a separate transaction so subsequent publication rollback
// restores this verified state, never the older pre-verification trust floors.
// A canceled/expired artifact is not accepted by retaining its SDK prefix.
func (s *publisherStore) retainPrefix(source *publisherSnapshot, progress *promotionProgress, roots []publisherRoot, gate func() error) (*publisherSnapshot, bool, error) {
	if source.state == nil || progress == nil || progress.next == nil {
		return source, false, nil // no partial first enrollment
	}
	before, err := source.state.Delivery.Digest()
	if err != nil || before != progress.sourceDigest {
		return nil, false, errors.New("repository: prefix source does not match retained authority")
	}
	after, err := progress.next.Digest()
	if err != nil {
		return nil, false, err
	}
	if progress.next.AnchorSHA256 != source.state.Delivery.AnchorSHA256 || progress.next.ClockFloor.Before(source.state.Delivery.ClockFloor) {
		return nil, false, errors.New("repository: prefix changed enrollment anchor or rolled back clock floor")
	}
	oldRoots, _ := json.Marshal(source.state.Roots)
	newRoots, err := json.Marshal(roots)
	if err != nil {
		return nil, false, err
	}
	if before == after && bytes.Equal(oldRoots, newRoots) {
		return source, false, nil
	}
	next := *source.state
	next.Delivery = progress.next.Clone()
	next.Roots = roots
	if err = s.commit(source, &next, nil, nil, nil, gate); err != nil {
		return nil, false, err
	}
	nextSource, err := s.read()
	return nextSource, true, err
}

type stagedPublication struct {
	files map[string][]byte // owned candidate files; paths derived locally only
	raw   []byte
	dir   string
}

// writeImmutable is new-only, with byte-identical replay permitted. The store
// lease serializes cooperating processes. A preexisting different byte string
// at a version/hash path is an error, not permission to replace history.
func writeImmutable(filename string, raw []byte) error {
	if err := publisherDirectory(filepath.Dir(filename)); err != nil {
		return err
	}
	existing, exists, err := optionalPublisherFile(filename, int64(len(raw)))
	if err != nil {
		return err
	}
	if exists {
		if !bytes.Equal(existing, raw) {
			return errors.New("repository: immutable publication path conflict")
		}
		return nil
	}
	return publisherfiles.PublishNewChecked(filename, raw, func() error { return publisherPath(filename) })
}

// visit derives the full content-addressed release view and the shared TUF
// version/hash paths. Client transfers live only in the release directory;
// unlike versioned metadata their deterministic client name may be reused.
func (p *stagedPublication) visit(store *publisherStore, visit func(string, []byte) error) error {
	base := filepath.Join(store.root, "public")
	// ReleaseID binds the manifest, not whitespace/map ordering of its outer
	// candidate JSON. Address the original frame by its own approved byte hash
	// so equivalent frames cannot collide at one supposedly immutable path.
	if err := visit(filepath.Join(base, filepath.FromSlash(p.dir), "candidates", digest(p.raw)+".json"), p.raw); err != nil {
		return err
	}
	names := make([]string, 0, len(p.files))
	for name := range p.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		local := filepath.FromSlash(name)
		if !publisherfiles.RelativeJSON(name) || !filepath.IsLocal(local) {
			return errors.New("repository: portable local immutable path required")
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasSuffix(part, ".") {
				return errors.New("repository: Windows trailing-dot path alias forbidden")
			}
		}
		if err := visit(filepath.Join(base, filepath.FromSlash(p.dir), local), p.files[name]); err != nil {
			return err
		}
		if name == "metadata/root.json" || name == "metadata/timestamp.json" || strings.HasPrefix(name, "clients/") {
			continue
		}
		if !strings.HasPrefix(name, "metadata/") && !strings.HasPrefix(name, "targets/") {
			return errors.New("repository: immutable path outside public namespaces")
		}
		if err := visit(filepath.Join(base, local), p.files[name]); err != nil {
			return err
		}
	}
	return nil
}

func (p *stagedPublication) check(store *publisherStore, gate func() error) error {
	err := p.visit(store, func(filename string, raw []byte) error {
		if err := gate(); err != nil {
			return err
		}
		actual, exists, err := optionalPublisherFile(filename, int64(len(raw)))
		if err != nil {
			return err
		}
		if !exists || !bytes.Equal(actual, raw) {
			return errors.New("repository: staged immutable publication changed")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return gate() // cancellation/expiry may have arrived during the last file read
}

func promote(candidateFile, candidateSHA, policyFile, policySHA, initialRoot, directory string) (report []byte, finalErr error) {
	store, release, err := openPublisher(directory)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if err := release(); err != nil {
			if committed {
				err = errors.Join(errors.New("repository: local publication committed; writer release failed"), err)
			}
			finalErr = errors.Join(finalErr, err)
		}
	}()
	source, err := store.read()
	if err != nil {
		return nil, err
	}
	candidate, err := readPromotionInput(candidateFile, candidateSHA, delivery.MaxEncodedPackageBytes, store)
	if err != nil {
		return nil, err
	}
	authority, err := readPromotionInput(policyFile, policySHA, metadata.MaxTrustPolicySize, store)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	clock := &metadata.ObservedClock{}
	prepared, err := store.preparePublisherRelease(ctx, source, candidate, authority, initialRoot, clock)
	prefixRetained := prepared != nil && prepared.prefixRetained
	defer func() {
		if finalErr != nil && prefixRetained && !committed {
			finalErr = errors.Join(errors.New("repository: release not accepted; verified trust prefix retained separately"), finalErr)
		}
	}()
	if err != nil {
		return nil, err
	}
	source, progress, roots := prepared.source, prepared.progress, prepared.roots
	inputs := func() error { return errors.Join(candidate.check(), authority.check()) }
	verified := progress.accepted
	if verified.sourceSHA256 != candidate.approval {
		return nil, errors.New("repository: private verification result belongs to different candidate bytes")
	}
	liveTime := func() (time.Time, error) {
		if err := ctx.Err(); err != nil {
			return time.Time{}, err
		}
		now, err := clock.Now(progress.next.ClockFloor)
		if err != nil {
			return time.Time{}, err
		}
		if now.Before(progress.next.ClockFloor) {
			now = progress.next.ClockFloor
		}
		if !now.Before(verified.candidate.wire.Manifest.ValidUntil) {
			return time.Time{}, errors.New("repository: publication expired while staging")
		}
		return now, nil
	}
	gate := func() error {
		now, err := liveTime()
		if err != nil {
			return err
		}
		if err := inputs(); err != nil {
			return err
		}
		return verified.check(ctx, now)
	}
	if err = gate(); err != nil {
		return nil, err
	}
	manifest := verified.candidate.wire.Manifest
	head := &publisherHead{Schema: 1, ReleaseID: verified.candidate.wire.ReleaseID, CandidateSHA256: candidateSHA, Directory: "releases/" + verified.candidate.wire.ReleaseID, RootSHA256: manifest.PublishedRootSHA256, TimestampSHA256: digest(verified.candidate.wire.Files["metadata/timestamp.json"]), ValidUntil: manifest.ValidUntil}
	head.Purpose = manifest.Purpose
	stage := &stagedPublication{files: verified.candidate.wire.Files, raw: candidate.raw, dir: head.Directory}
	pending := map[string][]byte{}
	if err = stage.visit(store, func(filename string, raw []byte) error { pending[filename] = raw; return nil }); err != nil {
		return nil, err
	}
	if err = store.admitPublicStorage(ctx, pending); err != nil {
		return nil, err
	}
	if err = stage.visit(store, func(filename string, raw []byte) error {
		if _, err := liveTime(); err != nil {
			return err
		}
		return writeImmutable(filename, raw)
	}); err != nil {
		return nil, err // append-only leftovers have no current pointer
	}
	finalGate := func() error {
		if err := gate(); err != nil {
			return err
		}
		// Avoid rereading large external inputs for every immutable file. Repeat
		// them once more after the bounded inventory pass, before journal publish.
		if err := stage.check(store, func() error { _, err := liveTime(); return err }); err != nil {
			return err
		}
		return gate()
	}
	next := &publisherState{Schema: 1, Authority: policySHA, Delivery: progress.next.Clone(), Published: head}
	next.PinPolicy = verified.pinPolicy.Clone()
	next.Roots, next.History = roots, archivedPublisherHeads(source, *head)
	if source.state != nil {
		next.ServingFloor = source.state.ServingFloor
	}
	if err = store.commit(source, next, head, verified.candidate.wire.Files["metadata/root.json"], verified.candidate.wire.Files["metadata/timestamp.json"], finalGate); err != nil {
		return nil, err
	}
	committed = true
	return json.Marshal(struct {
		Schema                   int    `json:"schema"`
		Operation                string `json:"operation"`
		ReleaseID                string `json:"release_id"`
		Directory                string `json:"directory"`
		RequiresClientAcceptance bool   `json:"requires_client_acceptance"`
		ServerActivated          bool   `json:"server_activated"`
	}{1, "local_repository_promotion", head.ReleaseID, head.Directory, true, false})
}

func publisherApplicationPolicy(raw []byte) (*metadata.TrustPolicy, error) {
	policy, err := metadata.DecodeTrustPolicy(raw)
	if err != nil {
		return nil, err
	}
	if policy.Schema != 2 || policy.Delivery != nil || len(policy.Keys) == 0 {
		return nil, errors.New("repository: independent scoped application keys without delivery cache required")
	}
	return policy, nil
}

// Application-role policy changes are an explicit owner-approved operation,
// never inferred from an unsigned client hint or candidate. Preserve every TUF
// floor and published alias. This does not migrate any installed client keys.
func authorizePublisherPolicy(filename, approvedSHA, previousSHA, directory string) (report []byte, finalErr error) {
	store, release, err := openPublisher(directory)
	if err != nil {
		return nil, err
	}
	defer func() { finalErr = errors.Join(finalErr, release()) }()
	source, err := store.read()
	if err != nil {
		return nil, err
	}
	if source.state == nil || !canonicalHash(previousSHA) || previousSHA != source.state.Authority {
		return nil, errors.New("repository: exact existing publisher authority approval required")
	}
	input, err := readPromotionInput(filename, approvedSHA, metadata.MaxTrustPolicySize, store)
	if err != nil {
		return nil, err
	}
	if _, err = publisherApplicationPolicy(input.raw); err != nil {
		return nil, err
	}
	next := *source.state
	next.Delivery = source.state.Delivery.Clone()
	next.Authority = approvedSHA
	if err = store.commit(source, &next, nil, nil, nil, input.check); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Schema    int    `json:"schema"`
		Operation string `json:"operation"`
		Authority string `json:"authority_sha256"`
	}{1, "local_application_policy_authorization", approvedSHA})
}

// Recovery never reads paths from a candidate/journal. It restores the fixed
// local target set under the same writer lease, without enrolling new trust.
func recoverPublisher(directory string) (report []byte, finalErr error) {
	store, release, err := openPublisher(directory)
	if err != nil {
		return nil, err
	}
	defer func() { finalErr = errors.Join(finalErr, release()) }()
	if err = publisherPath(store.journal); err != nil {
		return nil, err
	}
	for _, path := range store.paths {
		if err = publisherPath(path); err != nil {
			return nil, err
		}
	}
	journal := filetxn.Store{Dir: store.journal}
	if err = journal.Recover(store.targets()); err != nil {
		return nil, err
	}
	if _, err = store.read(); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Schema    int    `json:"schema"`
		Operation string `json:"operation"`
	}{1, "local_repository_recovery"})
}
