// Package filetxn commits a bounded set of local files with a rollback journal.
// The caller serializes readers and supplies fixed target paths again during
// recovery: journal contents can never choose filesystem destinations. Store
// and WithWriter share the cooperative Windows writer lock. WithRead uses the
// existing lock for explicit coherent snapshots; bare loaders do not acquire it.
package filetxn

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 16 << 20
const maxManifestSize = 4 << 10 // at most eight fixed names/digests, not a payload

var ErrRecoveryRequired = errors.New("filetxn: unfinished import requires recovery")

type File struct {
	Name, Path string
	Data       []byte
	// Keep retains this target's current contents or absence at commit. It
	// remains in the fixed backup/recovery set; no journal-chosen path or
	// persistent manifest extension is needed. Keep forbids replacement Data.
	Keep bool
}
type entry struct {
	Name    string `json:"name"`
	Existed bool   `json:"existed"`
	SHA256  string `json:"sha256"`
}
type manifest struct {
	Schema  int     `json:"schema"`
	Entries []entry `json:"entries"`
}

type Store struct {
	Dir string
	// BeforeCommit repeats the caller's source/validity fence after staging
	// and syncing the complete backup/new set, before publishing the journal.
	// Caller still owns serialization; this callback must not mutate targets.
	// Store holds the writer scope: do not call WithWriter/Commit/Recover here.
	BeforeCommit func() error
	// Private fault boundaries; production uses the concrete filetxn operations.
	replace func(string, []byte) error
	rename  func(string, string) error
	remove  func(string) error
}

func Check(dir string) error {
	if dir == "" {
		return nil
	}
	if err := journalFor(dir).fault(); err != nil {
		return err // absence of a pathname cannot retire an uncertain original I/O
	}
	_, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	return ErrRecoveryRequired
}

func (s *Store) Commit(files []File) (err error) {
	domain := journalFor(s.Dir)
	if err := domain.enter(); err != nil {
		return err
	}
	defer domain.operation.Unlock()
	completed := false
	var releaseWriter func() error
	defer func() { domain.finishWriter(releaseWriter, &err, completed) }()
	defer func() { domain.retireTargets(&err, completed) }()
	ownedStage, published := false, false
	defer func() {
		if p := recover(); p != nil {
			err = domain.fail(fmt.Errorf("filetxn: panic during commit (%T)", p))
		}
		if err != nil && published && domain.fault() == nil {
			recoveryErr := s.recover(files, domain)
			err = errors.Join(err, recoveryErr)
			if recoveryErr == nil {
				published = false
			}
		}
		if ownedStage && !published && domain.fault() == nil {
			cleanupErr := domain.cleanupPending()
			// A completed commit/rollback remains completed when bounded debris
			// cleanup fails. The retained receipt/CleanupError blocks new staging.
			if err != nil {
				err = errors.Join(err, cleanupErr)
			}
		}
	}()
	files, err = validate(s.Dir, files)
	if err != nil {
		return err
	}
	releaseWriter, err = domain.acquireWriter()
	if err != nil {
		return err
	}
	if err = Check(s.Dir); err != nil {
		return err
	}
	if err = domain.cleanupPending(); err != nil {
		return err
	}
	if err = domain.pinTargets(files); err != nil {
		return err
	}
	ownedStage = true
	stage, err := domain.newStage(filepath.Dir(s.Dir))
	if err != nil {
		return err
	}
	m := manifest{Schema: 1}
	for _, file := range files {
		old, exists, readErr := domain.readTarget(file.Path)
		if readErr != nil {
			return readErr
		}
		m.Entries = append(m.Entries, entry{Name: file.Name, Existed: exists, SHA256: digest(old)})
		if err = domain.write(filepath.Join(stage, file.Name+".old"), old, true); err != nil {
			return err
		}
		domain.pending.record(file.Name+".old", old)
		if err = domain.write(filepath.Join(stage, file.Name+".new"), file.Data, true); err != nil {
			return err
		}
		domain.pending.record(file.Name+".new", file.Data)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err = domain.write(filepath.Join(stage, "manifest.json"), data, true); err != nil {
		return err
	}
	domain.pending.record("manifest.json", data)
	if s.BeforeCommit != nil {
		if err = s.BeforeCommit(); err != nil {
			if errors.Is(err, ErrWriteRetirement) || errors.Is(err, ErrReadClose) {
				return domain.fail(err) // caller's original I/O also keeps the writer scope
			}
			return err
		}
	}
	if err = domain.checkTargets(); err != nil {
		return err
	}
	if err = s.move(stage, s.Dir); err != nil {
		return err
	}
	published = true
	for _, file := range files {
		if file.Keep {
			continue
		}
		if err = s.write(file.Path, file.Data); err != nil {
			return fmt.Errorf("filetxn: commit %s: %w", file.Name, err)
		}
	}
	// Removing the recovery barrier is the commit point. An uncertain move
	// keeps the process-local recovery fault, even if the name disappeared.
	// Unknown outcome never authorizes a second rollback/move. Later debris
	// cleanup cannot change the completed commit result.
	if err = domain.checkTargets(); err != nil {
		return err
	}
	if err = s.move(s.Dir, stage); err != nil {
		return err
	}
	published = false
	completed = true
	return nil
}

func (s *Store) Recover(files []File) (finalErr error) {
	domain := journalFor(s.Dir)
	if err := domain.enter(); err != nil {
		return err
	}
	defer domain.operation.Unlock()
	var releaseWriter func() error
	defer func() { domain.finishWriter(releaseWriter, &finalErr, finalErr == nil) }()
	defer func() { domain.retireTargets(&finalErr, finalErr == nil) }()
	defer func() {
		if p := recover(); p != nil {
			finalErr = domain.fail(fmt.Errorf("filetxn: panic during recovery (%T)", p))
		}
	}()
	files, err := validate(s.Dir, files)
	if err != nil {
		return err
	}
	if Check(s.Dir) == nil && domain.pending == nil {
		return nil // no cold journal/debris: do not create a lock or parent directory
	}
	releaseWriter, err = domain.acquireWriter()
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	return s.recover(files, domain)
}

// The caller already holds the one journal operation gate. Internal rollback
// must not recursively acquire the exported Recover gate or the caller lock.
func (s *Store) recover(files []File, domain *journalDomain) (finalErr error) {
	defer func() {
		if p := recover(); p != nil {
			finalErr = domain.fail(fmt.Errorf("filetxn: panic during rollback (%T)", p))
		}
	}()
	if err := domain.fault(); err != nil {
		return err
	}
	if err := Check(s.Dir); err == nil {
		return domain.cleanupPending()
	}
	if err := domain.bindRecovery(s.Dir); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	data, exists, err := domain.read(filepath.Join(s.Dir, "manifest.json"), maxManifestSize)
	if err != nil || !exists {
		return errors.Join(ErrRecoveryRequired, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m manifest
	if err := dec.Decode(&m); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF || m.Schema != 1 || len(m.Entries) != len(files) {
		return ErrRecoveryRequired
	}
	domain.pending.record("manifest.json", data)
	backups := make([][]byte, len(files))
	// Authenticate the complete local backup set before restoring any file.
	// Hashes detect accidental corruption, not an attacker who can rewrite both.
	for i, file := range files {
		e := m.Entries[i]
		if e.Name != file.Name {
			return ErrRecoveryRequired
		}
		old, exists, err := domain.read(filepath.Join(s.Dir, file.Name+".old"), maxFileSize)
		if err != nil || !exists || digest(old) != e.SHA256 || !e.Existed && len(old) != 0 {
			return errors.Join(ErrRecoveryRequired, err)
		}
		backups[i] = old
		domain.pending.record(file.Name+".old", old)
		// New bytes are irrelevant to rollback authority. On cold recovery
		// observe them only for bounded debris cleanup; never apply them.
		if !domain.pending.contains(file.Name + ".new") {
			raw, present, cleanupErr := domain.read(filepath.Join(s.Dir, file.Name+".new"), maxFileSize)
			if err := domain.fault(); err != nil {
				return err
			}
			if cleanupErr != nil {
				domain.pending.uncertain = true
			} else if present {
				domain.pending.record(file.Name+".new", raw)
			}
		}
	}
	// Cold recovery creates/pins target parents only after every old backup
	// authenticates. Internal rollback reuses the operation's original parents.
	if err := domain.pinTargets(files); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	var failures []error
	for i, file := range files {
		if m.Entries[i].Existed {
			err = s.write(file.Path, backups[i])
		} else {
			err = s.erase(file.Path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("filetxn: restore %s: %w", file.Name, err))
		}
		// Never churn targets after uncertain original retirement.
		if err := domain.fault(); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		return errors.Join(append([]error{ErrRecoveryRequired}, failures...)...)
	}
	if err := domain.checkTargets(); err != nil {
		return err
	}
	trash, err := recoveryDestination(filepath.Dir(s.Dir))
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if err = s.move(s.Dir, trash); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	_ = domain.cleanupPending() // separate retained debris outcome after completed rollback
	return nil
}

func validate(dir string, files []File) ([]File, error) {
	if dir == "" || len(files) == 0 || len(files) > 8 {
		return nil, errors.New("filetxn: invalid transaction shape")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	lockPath, err := journalWriterPath(dir)
	if err != nil {
		return nil, err
	}
	seenNames, seenPaths := map[string]bool{}, map[string]bool{}
	out := make([]File, len(files))
	for i, file := range files {
		valid := len(file.Name) > 0 && len(file.Name) <= 32
		for _, c := range file.Name {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				valid = false
			}
		}
		p, err := filepath.Abs(file.Path)
		key := strings.ToLower(p)
		if err != nil || file.Path == "" || !valid || seenNames[file.Name] || seenPaths[key] || len(file.Data) > maxFileSize || file.Keep && len(file.Data) != 0 || strings.EqualFold(p, dir) || strings.EqualFold(p, lockPath) || strings.HasPrefix(key, strings.ToLower(dir)+string(filepath.Separator)) {
			return nil, errors.New("filetxn: invalid or duplicate target")
		}
		seenNames[file.Name], seenPaths[key] = true, true
		out[i] = File{Name: file.Name, Path: p, Data: bytes.Clone(file.Data), Keep: file.Keep}
	}
	return out, nil
}

// Existing private fault-injection callers retain this helper. Actual Store
// writes use their own journal domain, including legitimate empty files.
var standaloneWriter = newJournalWriter()

func atomicWrite(path string, data []byte) error {
	return standaloneWriter.Write(path, data)
}

func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func invoke(fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("filetxn: operation panic (%T)", p)
		}
	}()
	return fn()
}
func (s *Store) write(path string, data []byte) error {
	if err := journalFor(s.Dir).checkTarget(path); err != nil {
		return err
	}
	return invoke(func() error {
		if s.replace != nil {
			return s.replace(path, data)
		}
		return journalFor(s.Dir).write(path, data, false)
	})
}
func (s *Store) move(from, to string) error {
	domain := journalFor(s.Dir)
	if err := domain.fault(); err != nil {
		return err
	}
	if domain.pending == nil || journalKey(domain.pending.path) != journalKey(from) {
		return domain.fail(ErrStagingCleanup)
	}
	if err := domain.sameDirectory(domain.pending); err != nil {
		return domain.fail(err)
	}
	if _, err := os.Lstat(to); !errors.Is(err, os.ErrNotExist) {
		return domain.fail(errors.Join(ErrStagingCleanup, err)) // never knowingly replace an occupied directory
	}
	domain.pending.nextPath = to
	err := invoke(func() error {
		if s.rename != nil {
			return s.rename(from, to)
		}
		if domain.pending.directory != nil {
			return domain.pending.directory.Move(from, to)
		}
		return os.Rename(from, to)
	})
	if err != nil {
		return domain.fail(err)
	}
	var info os.FileInfo
	if domain.pending.directory != nil {
		info, err = domain.pending.directory.Inspect(to)
	} else {
		info, err = domain.snapshotDirectory(to)
	}
	if err != nil || !sameBoundedReadFile(domain.pending.identity, info) {
		return domain.fail(errors.Join(ErrStagingCleanup, err))
	}
	domain.pending.path, domain.pending.nextPath = to, ""
	return nil
}
func (s *Store) erase(path string) error {
	domain := journalFor(s.Dir)
	if err := domain.checkTarget(path); err != nil {
		return err
	}
	err := invoke(func() error {
		if s.remove != nil {
			return s.remove(path)
		}
		if domain.targets != nil {
			return domain.targets.Erase(path)
		}
		return os.Remove(path)
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return domain.fail(err)
	}
	return err
}
