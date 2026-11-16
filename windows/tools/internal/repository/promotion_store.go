package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

const maxPublisherState = delivery.MaxStateBytes + (128 << 10)

// publisherState is private local authority. A public current pointer never
// supplies the retained trust baseline. Prefix retention may advance Delivery
// without changing Published or any public alias.
type publisherState struct {
	Schema       int                         `json:"schema"`
	Authority    string                      `json:"authority_sha256"`
	Delivery     *delivery.State             `json:"delivery"`
	Published    *publisherHead              `json:"published,omitempty"`
	History      []publisherHead             `json:"history,omitempty"`
	Roots        []publisherRoot             `json:"roots,omitempty"`
	ServingFloor *time.Time                  `json:"serving_floor,omitempty"`
	PinPolicy    *metadata.RetainedPinPolicy `json:"pin_policy,omitempty"`
}

type publisherHead = delivery.PublicationHead

type publisherStore struct {
	root    string
	journal string
	paths   [4]string // state, current, root alias, timestamp alias: fixed order
}

type publisherSnapshot struct {
	raw     [4][]byte
	present [4]bool
	state   *publisherState
}

func openPublisher(root string) (*publisherStore, func() error, error) {
	s, err := publisherLayout(root)
	if err != nil {
		return nil, nil, err
	}
	for _, dir := range []string{s.root, filepath.Join(s.root, "private"), filepath.Join(s.root, "public", "metadata")} {
		if err = publisherDirectory(dir); err != nil {
			return nil, nil, err
		}
	}
	release, err := publisherWriter(filepath.Join(s.root, "private", "writer.lock"), true)
	if err != nil {
		return nil, nil, err
	}
	return s, release, nil
}

func publisherLayout(root string) (*publisherStore, error) {
	if root == "" {
		return nil, errors.New("repository: explicit publisher directory required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if filepath.Dir(abs) == abs {
		return nil, errors.New("repository: dedicated publisher folder required, not drive root")
	}
	s := &publisherStore{root: abs, journal: filepath.Join(abs, "private", "promotion-journal")}
	s.paths = [4]string{filepath.Join(abs, "private", "state.json"), filepath.Join(abs, "public", "current.json"), filepath.Join(abs, "public", "metadata", "root.json"), filepath.Join(abs, "public", "metadata", "timestamp.json")}
	return s, nil
}

// Readers/status/repair never create an empty store or silently recover it.
// They acquire the writer's existing stable lease only while copying inputs.
func openExistingPublisher(root string) (*publisherStore, func() error, error) {
	s, err := publisherLayout(root)
	if err != nil {
		return nil, nil, err
	}
	for _, dir := range []string{s.root, filepath.Join(s.root, "private"), filepath.Join(s.root, "public", "metadata")} {
		if err = publisherPath(dir); err != nil {
			return nil, nil, err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return nil, nil, errors.Join(errors.New("repository: existing publisher directory required"), err)
		}
	}
	release, err := publisherWriter(filepath.Join(s.root, "private", "writer.lock"), false)
	if err != nil {
		return nil, nil, err
	}
	return s, release, nil
}

// publisherDirectory rejects existing reparse/symlink components before and
// after creation. It does not establish an ACL or defeat a hostile directory
// writer racing path resolution: that remains a Windows deployment gate.
func publisherDirectory(dir string) error {
	if err := publisherPath(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := publisherPath(dir); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("repository: publisher directory required")
	}
	return nil
}

func optionalPublisherFile(path string, limit int64) ([]byte, bool, error) {
	if err := publisherPath(path); err != nil {
		return nil, false, err
	}
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	raw, err := publisherfiles.Read(path, limit)
	return raw, true, err
}

func decodePublisherState(raw []byte) (*publisherState, error) {
	fields, err := publisherfiles.Object(raw, 8)
	if err != nil || len(fields) < 3 || fields["schema"] == nil || fields["authority_sha256"] == nil || fields["delivery"] == nil {
		return nil, errors.New("repository: invalid retained publisher state")
	}
	for name := range fields {
		if name != "schema" && name != "authority_sha256" && name != "delivery" && name != "published" && name != "history" && name != "roots" && name != "serving_floor" && name != "pin_policy" {
			return nil, errors.New("repository: unknown publisher state field")
		}
	}
	if value, exists := fields["pin_policy"]; exists && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, errors.New("repository: retained pin policy cannot be null")
	}
	if err = validatePublisherHistoryFields(fields); err != nil {
		return nil, err
	}
	if value, ok := fields["published"]; ok {
		if _, err = delivery.DecodePublicationHead(value); err != nil {
			return nil, err
		}
	}
	var state publisherState
	if json.Unmarshal(raw, &state) != nil || state.Schema != 1 || !canonicalHash(state.Authority) || state.Delivery == nil {
		return nil, errors.New("repository: malformed publisher state")
	}
	if err = state.Delivery.Validate(); err != nil {
		return nil, err
	}
	if err = metadata.CheckPinPolicyContinuity(nil, state.PinPolicy); err != nil {
		return nil, err
	}
	if head := state.Published; head != nil {
		if head.Schema != 1 || !canonicalHash(head.ReleaseID) || !canonicalHash(head.CandidateSHA256) || !canonicalHash(head.RootSHA256) || !canonicalHash(head.TimestampSHA256) || head.Directory != "releases/"+head.ReleaseID || head.ValidUntil.IsZero() {
			return nil, errors.New("repository: malformed retained publication pointer")
		}
	}
	if err = state.checkHistory(); err != nil {
		return nil, err
	}
	return &state, nil
}

// capture requires a valid retained state when present, but deliberately does
// not accept public aliases. Only explicit repair may use a mismatching capture.
func (s *publisherStore) capture() (*publisherSnapshot, error) {
	if err := filetxn.Check(s.journal); err != nil {
		return nil, err // recovery is always an explicit separate operation
	}
	snap := &publisherSnapshot{}
	for i, limit := range []int64{maxPublisherState, 16 << 10, delivery.MaxMetadataBytes, 64 << 10} {
		raw, exists, err := optionalPublisherFile(s.paths[i], limit)
		if err != nil {
			return nil, err
		}
		snap.raw[i], snap.present[i] = raw, exists
	}
	if snap.present[0] {
		state, err := decodePublisherState(snap.raw[0])
		if err != nil {
			return nil, err
		}
		snap.state = state
	}
	return snap, nil
}

func (s *publisherStore) read() (*publisherSnapshot, error) {
	snap, err := s.capture()
	if err != nil {
		return nil, err
	}
	if err = snap.checkAliases(); err != nil {
		return nil, err
	}
	return snap, nil
}

func (snap *publisherSnapshot) checkAliases() error {
	if !snap.present[0] {
		if snap.present[1] || snap.present[2] || snap.present[3] {
			return errors.New("repository: publisher authority missing; refusing re-enrollment")
		}
		return nil
	}
	state := snap.state
	if state.Published == nil {
		if snap.present[1] || snap.present[2] || snap.present[3] {
			return errors.New("repository: unexpected public aliases without retained publication")
		}
		return nil
	}
	head, err := json.Marshal(state.Published)
	if err != nil || !snap.present[1] || !snap.present[2] || !snap.present[3] || !bytes.Equal(head, snap.raw[1]) || digest(snap.raw[2]) != state.Published.RootSHA256 || digest(snap.raw[3]) != state.Published.TimestampSHA256 {
		return errors.New("repository: public aliases differ from retained publication; explicit repair required")
	}
	return nil
}

func samePublisherSource(source, current *publisherSnapshot) error {
	if source == nil || current == nil {
		return errors.New("repository: publisher source snapshot required")
	}
	for i := range current.raw {
		if source.present[i] != current.present[i] || !bytes.Equal(source.raw[i], current.raw[i]) {
			return errors.New("repository: publisher source changed")
		}
	}
	return nil
}

func (s *publisherStore) fence(source *publisherSnapshot) error {
	current, err := s.read()
	if err != nil {
		return err
	}
	return samePublisherSource(source, current)
}

func (s *publisherStore) targets() []filetxn.File {
	return []filetxn.File{{Name: "state", Path: s.paths[0], Keep: true}, {Name: "current", Path: s.paths[1], Keep: true}, {Name: "root", Path: s.paths[2], Keep: true}, {Name: "timestamp", Path: s.paths[3], Keep: true}}
}

func (s *publisherStore) commit(source *publisherSnapshot, state *publisherState, head *publisherHead, root, timestamp []byte, gate func() error) error {
	if gate == nil || state == nil {
		return errors.New("repository: owned state and final source gate required")
	}
	if err := checkPublisherHistoryTransition(source, state, head); err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > maxPublisherState {
		return delivery.ErrBudget
	}
	if _, err = decodePublisherState(raw); err != nil {
		return err
	}
	if head != nil {
		if state.Published != head || len(root) == 0 || len(timestamp) == 0 || digest(root) != head.RootSHA256 || digest(timestamp) != head.TimestampSHA256 {
			return errors.New("repository: state and publication aliases differ")
		}
	} else if source != nil {
		var previous *publisherHead
		if source.state != nil {
			previous = source.state.Published
		}
		a, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		b, err := json.Marshal(state.Published)
		if err != nil || !bytes.Equal(a, b) {
			return errors.New("repository: prefix/policy operation cannot change published pointer")
		}
	}
	files := s.targets()
	files[0].Data, files[0].Keep = raw, false
	if head != nil {
		files[1].Data, err = json.Marshal(head)
		if err != nil {
			return err
		}
		files[1].Keep = false
		files[2].Data, files[2].Keep = root, false
		files[3].Data, files[3].Keep = timestamp, false // mutable activation hint last
	}
	check := func() error {
		if err := s.fence(source); err != nil {
			return err
		}
		return gate()
	}
	if err = check(); err != nil {
		return err
	}
	store := filetxn.Store{Dir: s.journal, BeforeCommit: check}
	return store.Commit(files)
}
