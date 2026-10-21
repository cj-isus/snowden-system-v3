package filetxn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, missing bool) (*Store, []File) {
	t.Helper()
	dir := t.TempDir()
	s := &Store{Dir: filepath.Join(dir, "pending")}
	files := []File{}
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("file-%d", i)
		p := filepath.Join(dir, name)
		if !missing {
			if err := os.WriteFile(p, []byte("old-"+name), 0600); err != nil {
				t.Fatal(err)
			}
		}
		files = append(files, File{Name: name, Path: p, Data: []byte("new-" + name)})
	}
	return s, files
}

func assertOld(t *testing.T, files []File, missing bool) {
	t.Helper()
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if missing {
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatal("new target survived rollback")
			}
		} else if err != nil || string(data) != "old-"+file.Name {
			t.Fatalf("old target lost %s: %v", file.Name, err)
		}
	}
}

func TestFileTransactionCommit(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			s, files := fixture(t, missing)
			if err := s.Commit(files); err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				data, err := os.ReadFile(file.Path)
				if err != nil || !bytes.Equal(data, file.Data) {
					t.Fatalf("new bytes missing: %v", err)
				}
			}
			if err := Check(s.Dir); err != nil {
				t.Fatal(err)
			}
			if err := s.Recover(files); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFileTransactionWriteFault(t *testing.T) {
	for i := 0; i < 4; i++ {
		for _, panicFault := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d-panic-%v", i, panicFault), func(t *testing.T) {
				s, files := fixture(t, false)
				calls := 0
				s.replace = func(path string, data []byte) error {
					call := calls
					calls++
					if call == i {
						if panicFault {
							panic("synthetic-private-payload")
						}
						return errors.New("write fault")
					}
					return atomicWrite(path, data)
				}
				err := s.Commit(files)
				if err == nil || strings.Contains(err.Error(), "synthetic-private-payload") {
					t.Fatalf("fault/payload: %v", err)
				}
				assertOld(t, files, false)
				if err := Check(s.Dir); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestFileTransactionRenameFault(t *testing.T) {
	for _, phase := range []int{0, 1} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			s, files := fixture(t, false)
			calls := 0
			s.rename = func(from, to string) error {
				call := calls
				calls++
				if call == phase {
					return errors.New("rename fault")
				}
				return os.Rename(from, to)
			}
			if err := s.Commit(files); err == nil {
				t.Fatal("rename fault accepted")
			}
			assertOld(t, files, false)
			if err := Check(s.Dir); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func retained(t *testing.T, missing bool) (*Store, []File) {
	t.Helper()
	s, files := fixture(t, missing)
	calls := 0
	s.replace = func(path string, data []byte) error {
		calls++
		if calls > 1 {
			return errors.New("disk unavailable")
		}
		return atomicWrite(path, data)
	}
	if missing {
		s.remove = func(string) error { return errors.New("remove unavailable") }
	}
	if err := s.Commit(files); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("recovery barrier missing: %v", err)
	}
	if err := Check(s.Dir); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatal("unfinished transaction readable")
	}
	return s, files
}

func TestFileTransactionRecovery(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			s, files := retained(t, missing)
			// A fresh instance models recovery after interruption. This is not a
			// claim about Windows power-loss durability or a subprocess kill test.
			fresh := &Store{Dir: s.Dir}
			if err := fresh.Recover(files); err != nil {
				t.Fatal(err)
			}
			assertOld(t, files, missing)
			if err := fresh.Recover(files); err != nil {
				t.Fatal(err)
			}
			if err := fresh.Commit(files); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFileTransactionCorruptRecoveryDoesNotWrite(t *testing.T) {
	for _, name := range []string{"backup", "manifest", "schema", "name", "extra-field", "trailing", "missing-backup"} {
		t.Run(name, func(t *testing.T) {
			s, files := retained(t, false)
			before := make([][]byte, len(files))
			for i, f := range files {
				before[i], _ = os.ReadFile(f.Path)
			}
			manifestPath := filepath.Join(s.Dir, "manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var m manifest
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "backup":
				err = os.WriteFile(filepath.Join(s.Dir, files[0].Name+".old"), []byte("corrupt"), 0600)
			case "missing-backup":
				err = os.Remove(filepath.Join(s.Dir, files[0].Name+".old"))
			case "manifest":
				data = []byte("{")
			case "schema":
				m.Schema = 99
				data, _ = json.Marshal(m)
			case "name":
				m.Entries[0].Name = "../../outside"
				data, _ = json.Marshal(m)
			case "extra-field":
				data = append(data[:len(data)-1], []byte(`,"path":"outside"}`)...)
			case "trailing":
				data = append(data, []byte(" {}")...)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifestPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			fresh := &Store{Dir: s.Dir}
			if err := fresh.Recover(files); !errors.Is(err, ErrRecoveryRequired) {
				t.Fatalf("corruption accepted: %v", err)
			}
			for i, f := range files {
				after, err := os.ReadFile(f.Path)
				if err != nil || !bytes.Equal(before[i], after) {
					t.Fatal("recovery wrote before validating all backups")
				}
			}
		})
	}
}

func TestFileTransactionValidation(t *testing.T) {
	for _, name := range []string{"empty-name", "path-name", "empty-path", "duplicate-name", "duplicate-path", "inside-journal", "oversized", "directory-target"} {
		t.Run(name, func(t *testing.T) {
			s, files := fixture(t, false)
			original := append([]File(nil), files...)
			switch name {
			case "empty-name":
				files[0].Name = ""
			case "path-name":
				files[0].Name = "../x"
			case "empty-path":
				files[0].Path = ""
			case "duplicate-name":
				files[1].Name = files[0].Name
			case "duplicate-path":
				files[1].Path = files[0].Path
			case "inside-journal":
				files[0].Path = filepath.Join(s.Dir, "x")
			case "oversized":
				files[0].Data = make([]byte, maxFileSize+1)
			case "directory-target":
				files[0].Path = t.TempDir()
			}
			if err := s.Commit(files); err == nil {
				t.Fatal("invalid target accepted")
			}
			assertOld(t, original, false)
			if err := Check(s.Dir); err != nil {
				t.Fatal("validation left a recovery barrier")
			}
		})
	}
}
