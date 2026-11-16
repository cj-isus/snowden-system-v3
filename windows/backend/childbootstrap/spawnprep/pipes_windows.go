//go:build windows

package spawnprep

import (
	"errors"
	"io"

	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

// Pipes owns the original five Go file slots, three inherited duplicates and
// attribute allocation. Publish zero Pipes before obtaining its parent endpoints
// or preparing anything. Serialize whole-Pipes operations; parent File I/O/Close
// may run concurrently. No handles or os.File pointers are returned to callers.
type Pipes struct {
	inputChild, outputChild, stderr              File
	inputParent, outputParent                    File
	inherited                                    [3]windows.Handle
	closeAttempted                               [3]bool
	attributes                                   Attributes
	prepareAttempted, preparing, ready, retiring bool
}

func (p *Pipes) Input() io.WriteCloser {
	if p == nil {
		return nil
	}
	return &p.inputParent
}

func (p *Pipes) Output() io.ReadCloser {
	if p == nil {
		return nil
	}
	return &p.outputParent
}

// Owns checks original endpoint identity, not readiness or invocation authority.
func (p *Pipes) Owns(input io.WriteCloser, output io.ReadCloser) bool {
	writer, writerOK := input.(*File)
	reader, readerOK := output.(*File)
	return p != nil && writerOK && readerOK && writer == &p.inputParent && reader == &p.outputParent
}

func (p *Pipes) Prepare(job windows.Handle) error {
	return p.prepareOnce(job, false)
}

// PrepareDetached binds the existing original stdio/allowlist/Job owner to NUL.
// A GUI with a separate named control pipe needs no anonymous I/O pump. The
// standard input is read-only, output/error write-only. No parent endpoints open.
func (p *Pipes) PrepareDetached(job windows.Handle) error {
	return p.prepareOnce(job, true)
}

func (p *Pipes) prepareOnce(job windows.Handle, detached bool) error {
	if p == nil || !nativelease.ValidHandleValue(job) || p.retiring || p.prepareAttempted {
		return ErrResources
	}
	p.prepareAttempted, p.preparing = true, true
	err := p.prepare(job, detached)
	p.ready = err == nil
	p.preparing = false // never deferred: a nonreturning preparation cannot be released
	return err
}

func (p *Pipes) prepare(job windows.Handle, detached bool) error {
	if detached {
		for index, file := range []*File{&p.inputChild, &p.outputChild, &p.stderr} {
			if err := file.openNull(index == 0); err != nil {
				return errors.Join(ErrResources, err)
			}
		}
	} else {
		if err := openPipe(&p.inputChild, &p.inputParent); err != nil {
			return errors.Join(ErrResources, err)
		}
		if err := openPipe(&p.outputParent, &p.outputChild); err != nil {
			return errors.Join(ErrResources, err)
		}
		if err := p.stderr.openErrorSink(); err != nil {
			return errors.Join(ErrResources, err)
		}
	}
	for index, file := range []*File{&p.inputChild, &p.outputChild, &p.stderr} {
		fd, err := file.descriptor()
		if err != nil {
			return errors.Join(ErrResources, err)
		}
		if err := windows.DuplicateHandle(windows.CurrentProcess(), windows.Handle(fd), windows.CurrentProcess(), &p.inherited[index], 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return errors.Join(ErrResources, err)
		}
	}
	return p.attributes.Initialize(&p.inherited, job)
}

// One synchronous trusted create callback borrows the fixed original stdio and
// allowlist and creation-time Job list. It may set window flags; it must not
// retain/mutate stdio/list or reenter Pipes. The original Job stays open in its
// caller owner. Attributes keeps an unreturned borrow restrictive.
func (p *Pipes) WithStartup(use func(*windows.StartupInfoEx) error) error {
	if p == nil || use == nil || !p.ready || p.retiring || p.preparing {
		return ErrResources
	}
	return p.attributes.WithStartup(func(startup *windows.StartupInfoEx) error {
		startup.Flags = windows.STARTF_USESTDHANDLES
		startup.StdInput, startup.StdOutput, startup.StdErr = p.inherited[0], p.inherited[1], p.inherited[2]
		return use(startup)
	})
}

// ClosePreparation releases only the child's preparation, never the parent I/O
// endpoints needed after Resume. Caller closes those endpoints and awaits actual
// I/O separately before releasing its original process/Job/image owners.
func (p *Pipes) ClosePreparation() (finalErr error) {
	if p == nil {
		return ErrResources
	}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrResources)
		}
	}()
	p.retiring, p.ready = true, false
	if p.preparing {
		completed = true
		return ErrResources
	}
	if err := p.attributes.Close(); err != nil {
		completed = true
		return err
	}
	for index := range p.inherited {
		if err := CloseOnce(&p.inherited[index], &p.closeAttempted[index]); err != nil {
			completed = true
			return err
		}
	}
	for _, file := range []*File{&p.inputChild, &p.outputChild, &p.stderr} {
		finalErr = errors.Join(finalErr, file.Close())
	}
	completed = true
	return finalErr
}
