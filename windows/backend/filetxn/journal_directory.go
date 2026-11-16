package filetxn

import "os"

// The receipt is allocated and retained before Create/Open. Its raw original
// handles survive every partial acquisition, mutation and uncertain retirement.
// The portable adapter returns nil and keeps the existing path-based behavior.
type journalDirectory interface {
	Create(parent string) (string, os.FileInfo, error)
	Open(path string) (os.FileInfo, error)
	Inspect(path string) (os.FileInfo, error)
	Move(from, to string) error
	RemoveFile(name string, size int, sha string) error
	Remove() error
	Uncertain() bool
}
