package update

import (
	"errors"
	"io"
	"os"
	"sync"
)

var errPairMetadataClose = errors.New("update: metadata reader close unconfirmed")

type pairMetadataFile interface {
	io.Reader
	Stat() (os.FileInfo, error)
	Close() error
}

// One bounded process-level quarantine. Reads are serialized, fail-fast; an
// uncertain close poisons future update reads until exit. Keep the original
// reader reachable. No metadata/authority is cached and no close is retried.
type pairMetadataReadOwner struct {
	sync.Mutex
	file     pairMetadataFile
	closeErr error
}

var pairMetadataOwner pairMetadataReadOwner

type pairMetadataLease struct {
	owner    *pairMetadataReadOwner
	released bool
	result   error
}

func openPairMetadata(path string, directory bool) (result *pairMetadataLease, finalErr error) {
	owner := &pairMetadataOwner
	if !owner.TryLock() {
		return nil, ErrPairBusy
	}
	lease := &pairMetadataLease{owner: owner}
	if owner.file != nil || owner.closeErr != nil {
		owner.Unlock()
		return nil, errPairMetadataClose
	}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrPairRecoveryRequired, lease.Close())
			result = nil
		}
	}()
	if err := openPairMetadataNative(path, directory); err != nil {
		completed = true
		if closeErr := lease.Close(); closeErr != nil {
			return nil, errors.Join(ErrPairRecoveryRequired, closeErr) // uncertainty is not absence
		}
		return nil, err
	}
	completed = true
	return lease, nil
}

func (lease *pairMetadataLease) Close() (finalErr error) {
	if lease.released {
		return lease.result
	}
	lease.released, lease.result = true, errPairMetadataClose
	defer lease.owner.Unlock()
	defer func() {
		if recover() != nil {
			finalErr = errPairMetadataClose
		}
		lease.result = finalErr
	}()
	owner := lease.owner
	if owner.closeErr != nil {
		return lease.result
	}
	if owner.file != nil {
		owner.closeErr = errPairMetadataClose
		if err := owner.file.Close(); err != nil {
			return lease.result
		}
		owner.file, owner.closeErr = nil, nil
	}
	return nil
}

func pairMetadataSnapshot(path string, directory bool) (info os.FileInfo, finalErr error) {
	before, err := os.Lstat(path)
	if err != nil || directory && before.Mode()&os.ModeType != os.ModeDir || !directory && !before.Mode().IsRegular() {
		return nil, errors.Join(ErrPairRecoveryRequired, err)
	}
	lease, err := openPairMetadata(path, directory)
	if err != nil {
		return nil, err
	}
	defer func() { finalErr = errors.Join(finalErr, lease.Close()) }()
	info, err = lease.owner.file.Stat() // eager FileID; comparison never opens a lazy Lstat path
	if err != nil || directory && info.Mode()&os.ModeType != os.ModeDir || !directory && !info.Mode().IsRegular() {
		return nil, errors.Join(ErrPairRecoveryRequired, err)
	}
	return info, nil
}
