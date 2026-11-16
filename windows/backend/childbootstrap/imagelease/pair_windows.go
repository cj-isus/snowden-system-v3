//go:build windows

package imagelease

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/filelease"
	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"golang.org/x/sys/windows"
)

type imageFile struct {
	lease     *filelease.Lease
	path      string
	directory bool
	identity  windows.ByHandleFileInformation
}

// Pair is a pure original owner before Prepare. Retain its pointer on every
// failure/panic/Goexit; do not copy after use. Native handles never leave Pair.
// Caller still owns the surrounding process/Job lifetime and source authority.
type Pair struct {
	mu                      sync.Mutex
	retiring                atomic.Bool
	files                   [66]imageFile // at most64 ancestors + App + helper
	count                   int
	application, helper     string
	trust                   fileacl.Trust
	attempted, ready, fault bool
}

// MatchesUser compares against this original Pair's primary user. Trusted ACL
// principals (Administrators/SYSTEM) are not substitutes for that exact user.
func (pair *Pair) MatchesUser(user *windows.SID) bool {
	if pair == nil || !pair.mu.TryLock() {
		return false
	}
	defer pair.mu.Unlock()
	return pair.ready && !pair.fault && !pair.retiring.Load() && pair.trust.MatchesUser(user)
}

func (pair *Pair) finish(completed *bool, finalErr *error) {
	if !*completed {
		pair.fault, pair.ready = true, false
	}
	if pair.retiring.Load() {
		*finalErr = errors.Join(*finalErr, ErrPair)
	}
	pair.mu.Unlock()
}

func (pair *Pair) scope(ctx context.Context, check func(context.Context) error) error {
	if pair.retiring.Load() || pair.fault {
		return ErrPair
	}
	if err := childbootstrap.CheckScope(ctx, check); err != nil {
		return err
	}
	if pair.retiring.Load() {
		return ErrPair
	}
	return nil
}

func (pair *Pair) inspectPaths(ctx context.Context, check func(context.Context) error, use func(string, string) error) error {
	if err := pair.scope(ctx, check); err != nil {
		return err
	}
	if use == nil {
		return nil
	}
	err := use(pair.application, pair.helper)
	if sourceErr := pair.scope(ctx, check); sourceErr != nil {
		return errors.Join(err, sourceErr)
	}
	return err
}

// Prepare pins the caller's exact sibling paths. The trusted caller fixes their
// roles/name and independently authorizes the helper digest; no caller-selected
// executable is launched here. Helper hash/PE checks reuse original held bytes.
func (pair *Pair) Prepare(ctx context.Context, application, helper string, helperPin [32]byte, check func(context.Context) error) (finalErr error) {
	if pair == nil {
		return ErrPair
	}
	if !pair.mu.TryLock() {
		return errors.Join(ErrPair, ErrBusy)
	}
	completed := false
	defer pair.finish(&completed, &finalErr)
	if pair.attempted {
		completed = true
		return ErrPair
	}
	pair.attempted = true
	finalErr = pair.prepare(ctx, application, helper, helperPin, check)
	pair.ready = finalErr == nil
	completed = true
	return
}

func (pair *Pair) prepare(ctx context.Context, application, helper string, pin [32]byte, check func(context.Context) error) error {
	if err := pair.scope(ctx, check); err != nil {
		return err
	}
	if runtime.GOARCH != "amd64" || len(application) > 2048 || len(helper) > 2048 ||
		!filepath.IsAbs(application) || !filepath.IsAbs(helper) || filepath.Clean(application) != application || filepath.Clean(helper) != helper ||
		!imagecheck.Component(filepath.Base(application)) || !imagecheck.Component(filepath.Base(helper)) ||
		!strings.EqualFold(filepath.Dir(application), filepath.Dir(helper)) || strings.EqualFold(application, helper) {
		return ErrPair
	}
	pair.application, pair.helper = application, helper
	paths, err := imagecheck.DirectoryPaths(filepath.Dir(application))
	if err != nil {
		return err
	}
	if err := pair.scope(ctx, check); err != nil {
		return err
	}
	pair.trust, err = fileacl.CurrentTrust()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := pair.open(ctx, path, true, check); err != nil {
			return err
		}
	}
	if err := imagecheck.NTFS(pair.files[0].lease.HandleValue()); err != nil {
		return err
	}
	for _, path := range []string{application, helper} {
		if err := pair.open(ctx, path, false, check); err != nil {
			return err
		}
	}
	app, tool := &pair.files[pair.count-2], &pair.files[pair.count-1]
	if imagecheck.SameFileID(app.identity, tool.identity) {
		return ErrPair
	}
	digest, err := pair.hash(ctx, tool, MaxHelperBytes, check)
	if err != nil {
		return err
	}
	if !bytes.Equal(digest[:], pin[:]) {
		return ErrPair
	}
	reader := &imageReader{ctx: ctx, handle: tool.lease.HandleValue(), size: int64(tool.identity.FileSizeLow), ceiling: MaxHelperBytes,
		scope: func(ctx context.Context) error { return pair.scope(ctx, check) }}
	if err := imagecheck.PE(reader, reader.size, false); err != nil {
		return err
	}
	return pair.validate(ctx, check)
}

func (pair *Pair) open(ctx context.Context, path string, directory bool, check func(context.Context) error) error {
	if err := pair.scope(ctx, check); err != nil {
		return err
	}
	if pair.count == len(pair.files) {
		return ErrPair
	}
	for index := 0; index < pair.count; index++ {
		if !pair.files[index].lease.Ready() {
			return ErrPair
		}
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrPair
	}
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT)
	access, share := uint32(windows.GENERIC_READ|windows.READ_CONTROL), uint32(windows.FILE_SHARE_READ)
	if directory {
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
		access = windows.FILE_TRAVERSE | windows.FILE_READ_ATTRIBUTES | windows.READ_CONTROL | windows.SYNCHRONIZE
		share |= windows.FILE_SHARE_WRITE
	}
	file := &pair.files[pair.count]
	*file = imageFile{lease: new(filelease.Lease), path: path, directory: directory}
	pair.count++ // original slot/lease is published before native acquisition
	if err := file.lease.Open(name, access, share, flags); err != nil {
		return err
	}
	if err := file.lease.Inspect(path, directory, &file.identity, pair.trust.Trusted); err != nil {
		return err
	}
	return pair.scope(ctx, check)
}

func (pair *Pair) validate(ctx context.Context, check func(context.Context) error) error {
	if pair.count < 3 {
		return ErrPair
	}
	for index := 0; index < pair.count; index++ {
		if err := pair.scope(ctx, check); err != nil {
			return err
		}
		file := &pair.files[index]
		var current windows.ByHandleFileInformation
		if err := file.lease.Inspect(file.path, file.directory, &current, pair.trust.Trusted); err != nil {
			return err
		}
		if !imagecheck.SameSnapshot(file.identity, current, file.directory) {
			return ErrPair
		}
	}
	return pair.scope(ctx, check)
}

func (pair *Pair) Validate(ctx context.Context, check func(context.Context) error) (finalErr error) {
	return pair.WithPaths(ctx, check, func(string, string) error { return nil })
}

// WithPaths holds original file custody through a trusted synchronous inspector.
// The callback receives only immutable names, must not reenter Pair and cannot
// outlive this call. Source/retirement fences surround the callback's result.
func (pair *Pair) WithPaths(ctx context.Context, check func(context.Context) error, use func(string, string) error) (finalErr error) {
	if pair == nil {
		return ErrPair
	}
	if !pair.mu.TryLock() {
		return errors.Join(ErrPair, ErrBusy)
	}
	completed := false
	defer pair.finish(&completed, &finalErr)
	if !pair.ready || use == nil {
		completed = true
		return ErrPair
	}
	finalErr = pair.validate(ctx, check)
	if finalErr == nil {
		finalErr = pair.inspectPaths(ctx, check, use)
	}
	completed = true
	return
}

// Paths are immutable names, not authority or a borrowed native handle.
func (pair *Pair) Paths() (application, helper string) {
	if pair == nil || !pair.mu.TryLock() {
		return "", ""
	}
	defer pair.mu.Unlock()
	return pair.application, pair.helper
}

// Close retires positive use before attempting its mutex. The caller must first
// settle its original process/Job. Busy/unknown Close retains this exact Pair.
func (pair *Pair) Close() error {
	if pair == nil {
		return nil
	}
	pair.retiring.Store(true)
	if !pair.mu.TryLock() {
		return errors.Join(ErrCleanup, ErrBusy)
	}
	defer pair.mu.Unlock()
	pair.ready = false
	for index := pair.count - 1; index >= 0; index-- {
		if err := pair.files[index].lease.Close(); err != nil {
			return errors.Join(ErrCleanup, err) // never release ancestors past an uncertain original file
		}
	}
	return nil
}
