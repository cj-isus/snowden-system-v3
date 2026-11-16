package spawnprep

import (
	"os"
	"sync"
)

// File is an original one-acquisition os.File slot. Its owner publishes it before
// pipe/open calls; callers receive only Read/Write/Close. Acquire/Read/Write run
// outside the state mutex; Close retains checked-close serialization. Do not copy
// after use. Native completion and cleanup latency remain caller obligations.
type File struct {
	mu                                            sync.Mutex
	file                                          *os.File
	acquireAttempted, acquiring, retiring, closed bool
	closeErr                                      error
}

func (f *File) beginAcquire() bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.acquireAttempted || f.retiring || f.closed || f.closeErr != nil {
		return false
	}
	f.acquireAttempted, f.acquiring = true, true
	return true
}

func (f *File) finishAcquire() {
	f.mu.Lock()
	f.acquiring = false // actual return publishes the original file field, including partial errors
	f.mu.Unlock()
}

func openPipe(read, write *File) error {
	if read == nil || write == nil || read == write || !read.beginAcquire() {
		return ErrResources
	}
	if !write.beginAcquire() {
		read.finishAcquire() // no native call happened; this original empty slot stays one-use
		return ErrResources
	}
	var err error
	read.file, write.file, err = os.Pipe()
	read.finishAcquire()
	write.finishAcquire()
	return err
}

func (f *File) openErrorSink() error {
	return f.openNull(false)
}

func (f *File) openNull(input bool) error {
	if !f.beginAcquire() {
		return ErrResources
	}
	var err error
	mode := os.O_WRONLY
	if input {
		mode = os.O_RDONLY
	}
	f.file, err = os.OpenFile(os.DevNull, mode, 0)
	f.finishAcquire()
	return err
}

func (f *File) snapshot() (*os.File, error) {
	if f == nil {
		return nil, os.ErrClosed
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// Check acquiring first: only acquisition writes file outside this lock.
	if f.acquiring || f.retiring || f.closed || f.closeErr != nil || f.file == nil {
		return nil, os.ErrClosed
	}
	return f.file, nil
}

func (f *File) Read(data []byte) (int, error) {
	file, err := f.snapshot()
	if err != nil {
		return 0, err
	}
	return file.Read(data)
}

func (f *File) Write(data []byte) (int, error) {
	file, err := f.snapshot()
	if err != nil {
		return 0, err
	}
	return file.Write(data)
}

func (f *File) descriptor() (uintptr, error) {
	file, err := f.snapshot()
	if err != nil {
		return 0, err
	}
	return file.Fd(), nil // borrowed only by the serialized original Pipes preparation
}

func (f *File) Close() error {
	if f == nil {
		return ErrFileClose
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retiring = true
	if f.acquiring {
		// Never read or close an unpublished acquisition result.
		return ErrFileClose
	}
	if f.closeErr != nil {
		return f.closeErr
	}
	if f.closed || f.file == nil {
		f.closed = true // retirement seals even an empty slot against later acquisition
		return nil
	}
	f.closeErr = ErrFileClose // one physical Close attempt, sticky on error/panic/Goexit
	if err := f.file.Close(); err != nil {
		return f.closeErr
	}
	f.file, f.closed, f.closeErr = nil, true, nil
	return nil
}
