// Package filetxn commits a bounded set of local files with a rollback journal.
// The caller serializes writers/readers and supplies fixed target paths again
// during recovery: journal contents can never choose filesystem destinations.
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

var ErrRecoveryRequired = errors.New("filetxn: unfinished import requires recovery")

type File struct {
	Name, Path string
	Data       []byte
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
	// Private fault boundaries; production always uses the native operations.
	replace func(string, []byte) error
	rename  func(string, string) error
	remove  func(string) error
}

func Check(dir string) error {
	if dir == "" {
		return nil
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
	files, err = validate(s.Dir, files)
	if err != nil {
		return err
	}
	if err = Check(s.Dir); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.Dir), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(s.Dir), ".import-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage) // exact directory just created by this operation
	m := manifest{Schema: 1}
	for _, file := range files {
		old, exists, readErr := readTarget(file.Path)
		if readErr != nil {
			return readErr
		}
		m.Entries = append(m.Entries, entry{Name: file.Name, Existed: exists, SHA256: digest(old)})
		if err = writeSync(filepath.Join(stage, file.Name+".old"), old); err != nil {
			return err
		}
		if err = writeSync(filepath.Join(stage, file.Name+".new"), file.Data); err != nil {
			return err
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err = writeSync(filepath.Join(stage, "manifest.json"), data); err != nil {
		return err
	}
	if err = s.move(stage, s.Dir); err != nil {
		return err
	}
	published := true
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("filetxn: panic during commit (%T)", p)
		}
		if err != nil && published {
			err = errors.Join(err, s.Recover(files))
		}
	}()
	for _, file := range files {
		if err = s.write(file.Path, file.Data); err != nil {
			return fmt.Errorf("filetxn: commit %s: %w", file.Name, err)
		}
	}
	// Removing the recovery barrier is the commit point. If it fails, restore
	// all originals. Leftover committed stage cleanup cannot turn success into
	// a reported failure after the commit point.
	if err = s.move(s.Dir, stage); err != nil {
		return err
	}
	published = false
	return nil
}

func (s *Store) Recover(files []File) error {
	files, err := validate(s.Dir, files)
	if err != nil {
		return err
	}
	if err := Check(s.Dir); err == nil {
		return nil
	}
	stat, err := os.Lstat(s.Dir)
	if err != nil || !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
		return errors.Join(ErrRecoveryRequired, err)
	}
	data, exists, err := readTarget(filepath.Join(s.Dir, "manifest.json"))
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
	backups := make([][]byte, len(files))
	// Authenticate the complete local backup set before restoring any file.
	// Hashes detect accidental corruption, not an attacker who can rewrite both.
	for i, file := range files {
		e := m.Entries[i]
		if e.Name != file.Name {
			return ErrRecoveryRequired
		}
		old, exists, err := readTarget(filepath.Join(s.Dir, file.Name+".old"))
		if err != nil || !exists || digest(old) != e.SHA256 {
			return errors.Join(ErrRecoveryRequired, err)
		}
		backups[i] = old
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
	}
	if len(failures) > 0 {
		return errors.Join(append([]error{ErrRecoveryRequired}, failures...)...)
	}
	trash, err := os.MkdirTemp(filepath.Dir(s.Dir), ".import-recovered-")
	if err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if err = os.Remove(trash); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	if err = s.move(s.Dir, trash); err != nil {
		return errors.Join(ErrRecoveryRequired, err)
	}
	_ = os.RemoveAll(trash) // exact temporary recovery directory created above
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
		if err != nil || file.Path == "" || !valid || seenNames[file.Name] || seenPaths[key] || len(file.Data) > maxFileSize || strings.EqualFold(p, dir) || strings.HasPrefix(key, strings.ToLower(dir)+string(filepath.Separator)) {
			return nil, errors.New("filetxn: invalid or duplicate target")
		}
		seenNames[file.Name], seenPaths[key] = true, true
		out[i] = File{Name: file.Name, Path: p, Data: bytes.Clone(file.Data)}
	}
	return out, nil
}

func readTarget(path string) ([]byte, bool, error) {
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxFileSize {
		return nil, false, errors.New("filetxn: non-regular or oversized file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if len(data) > maxFileSize {
		return nil, false, errors.New("filetxn: oversized file")
	}
	return data, true, err
}

func writeSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	return errors.Join(writeErr, syncErr, f.Close())
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".import-write-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
	return invoke(func() error {
		if s.replace != nil {
			return s.replace(path, data)
		}
		return atomicWrite(path, data)
	})
}
func (s *Store) move(from, to string) error {
	return invoke(func() error {
		if s.rename != nil {
			return s.rename(from, to)
		}
		return os.Rename(from, to)
	})
}
func (s *Store) erase(path string) error {
	return invoke(func() error {
		if s.remove != nil {
			return s.remove(path)
		}
		return os.Remove(path)
	})
}
