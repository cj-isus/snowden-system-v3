package filetxn

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrStagingCleanup = errors.New("filetxn: retained staging cleanup requires attention")

// A bounded random destination, not an acquired directory or overwrite grant.
// Windows enforces no-replace in the subsequent original-handle rename.
func recoveryDestination(parent string) (string, error) {
	for attempt := 0; attempt < 16; attempt++ {
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", err
		}
		path := filepath.Join(parent, ".import-recovered-"+hex.EncodeToString(nonce[:]))
		_, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", ErrStagingCleanup
}

type journalCleanupFile struct {
	name, sha string
	size      int
}

// At most one directory receipt per journal, with 2*8+1 fixed file names.
// Windows also retains the original directory and its ancestors until cleanup.
type journalCleanup struct {
	directory            journalDirectory
	path, nextPath       string
	identity             os.FileInfo
	files                []journalCleanupFile
	acquiring, uncertain bool
}

func (cleanup *journalCleanup) record(name string, raw []byte) {
	for _, file := range cleanup.files {
		if file.name == name {
			return
		}
	}
	if len(cleanup.files) >= 17 {
		cleanup.uncertain = true
		return
	}
	cleanup.files = append(cleanup.files, journalCleanupFile{name: name, sha: digest(raw), size: len(raw)})
}

func (cleanup *journalCleanup) contains(name string) bool {
	for _, file := range cleanup.files {
		if file.name == name {
			return true
		}
	}
	return false
}

func (domain *journalDomain) newStage(parent string) (string, error) {
	if domain.pending != nil {
		return "", ErrStagingCleanup
	}
	cleanup := &journalCleanup{acquiring: true, files: make([]journalCleanupFile, 0, 17)}
	domain.pending = cleanup // before path-creating operations, including partial failure
	cleanup.directory = newJournalDirectory()
	if cleanup.directory != nil {
		path, info, err := cleanup.directory.Create(parent)
		cleanup.path, cleanup.identity, cleanup.acquiring = path, info, false
		if err != nil {
			return "", domain.fail(err)
		}
		return path, nil
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", domain.fail(err)
	}
	path, err := os.MkdirTemp(parent, ".import-stage-")
	cleanup.path, cleanup.acquiring = path, false
	if err != nil {
		return "", domain.fail(err)
	}
	cleanup.identity, err = domain.snapshotDirectory(path)
	if err != nil {
		return "", domain.fail(err)
	}
	return path, nil
}

func (domain *journalDomain) bindRecovery(path string) error {
	if cleanup := domain.pending; cleanup != nil && cleanup.directory != nil {
		if cleanup.acquiring || cleanup.nextPath != "" || journalKey(cleanup.path) != journalKey(path) {
			return domain.fail(ErrStagingCleanup)
		}
		if err := domain.sameDirectory(cleanup); err != nil {
			return domain.fail(err)
		}
		return nil
	}
	if domain.pending == nil {
		if directory := newJournalDirectory(); directory != nil {
			cleanup := &journalCleanup{path: path, directory: directory, acquiring: true, files: make([]journalCleanupFile, 0, 17)}
			domain.pending = cleanup // before opening the cold journal or any ancestor
			info, err := directory.Open(path)
			cleanup.identity, cleanup.acquiring = info, false
			if err != nil {
				return domain.fail(err)
			}
			return nil
		}
	}
	info, err := domain.snapshotDirectory(path)
	if err != nil {
		return err
	}
	if domain.pending == nil {
		domain.pending = &journalCleanup{path: path, identity: info, files: make([]journalCleanupFile, 0, 17)}
		return nil
	}
	if domain.pending.acquiring || domain.pending.nextPath != "" ||
		journalKey(domain.pending.path) != journalKey(path) || !sameBoundedReadFile(domain.pending.identity, info) {
		return domain.fail(ErrStagingCleanup)
	}
	return nil
}

func (domain *journalDomain) sameDirectory(cleanup *journalCleanup) error {
	var current os.FileInfo
	var err error
	if cleanup.directory != nil {
		current, err = cleanup.directory.Inspect(cleanup.path)
	} else {
		current, err = domain.snapshotDirectory(cleanup.path)
	}
	if err != nil || cleanup.identity == nil || !sameBoundedReadFile(cleanup.identity, current) {
		return errors.Join(ErrStagingCleanup, err)
	}
	return nil
}

// cleanupPending never traverses or recursively deletes a directory. Every
// eligible file name and exact byte digest came from this fixed journal set.
// Windows checks/deletes through original handles; the portable branch keeps
// the older path-based behavior and does not claim Windows custody guarantees.
func (domain *journalDomain) cleanupPending() (finalErr error) {
	cleanup := domain.pending
	if cleanup == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			cleanup.uncertain = true
			finalErr = domain.fail(ErrStagingCleanup)
		}
		if finalErr != nil {
			domain.cleanupStatus.Store(&journalFailure{err: errors.Join(ErrStagingCleanup, finalErr)})
		}
	}()
	if err := domain.fault(); err != nil {
		return err
	}
	if cleanup.acquiring || cleanup.uncertain || cleanup.nextPath != "" || journalKey(cleanup.path) == domain.dir {
		return ErrStagingCleanup
	}
	if err := domain.sameDirectory(cleanup); err != nil {
		return err
	}
	for _, file := range cleanup.files {
		if cleanup.directory != nil {
			if err := cleanup.directory.RemoveFile(file.name, file.size, file.sha); err != nil {
				if cleanup.directory.Uncertain() {
					return domain.fail(err)
				}
				return err
			}
			continue
		}
		path := filepath.Join(cleanup.path, file.name)
		raw, present, err := domain.read(path, maxFileSize)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		if len(raw) != file.size || digest(raw) != file.sha {
			return ErrStagingCleanup
		}
		if err := domain.sameDirectory(cleanup); err != nil {
			return err
		}
		cleanup.uncertain = true // error need not prove the original name survived
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("filetxn: cleanup known file: %w", err)
		}
		cleanup.uncertain = false
	}
	if err := domain.sameDirectory(cleanup); err != nil {
		return err
	}
	if cleanup.directory != nil {
		if err := cleanup.directory.Remove(); err != nil {
			if cleanup.directory.Uncertain() {
				return domain.fail(err)
			}
			return err
		}
		domain.pending = nil
		domain.cleanupStatus.Store(nil)
		return nil
	}
	cleanup.uncertain = true
	if err := os.Remove(cleanup.path); err != nil {
		return err
	} // unknown entries remain; no RemoveAll
	domain.pending = nil
	domain.cleanupStatus.Store(nil)
	return nil
}
