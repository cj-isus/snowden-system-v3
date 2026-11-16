package main

import (
	"errors"
	"os"
)

var errLogFileRetirement = errors.New("original log file operation or close unconfirmed")

// Owned by one original logging task. Other goroutines inspect its result only
// after that task's done fence. Never retry an interrupted Open/I/O/Close.
type originalLogFile struct {
	file *os.File

	openAttempted, opening, openReturned bool
	ioPending                            bool
	closeAttempted, closeReturned        bool
}

func (owner *originalLogFile) openFile(path string, flags int, mode os.FileMode) error {
	if owner.openAttempted || owner.file != nil || owner.closeAttempted {
		return errLogFileRetirement
	}
	owner.openAttempted, owner.opening = true, true
	var err error
	owner.file, err = os.OpenFile(path, flags, mode)
	owner.opening, owner.openReturned = false, true
	if err == nil && owner.file == nil {
		return errLogFileRetirement
	}
	return err
}

func (owner *originalLogFile) retire() (finalErr error) {
	returned := false
	defer func() {
		if recovered := recover(); recovered != nil || !returned {
			finalErr = errors.Join(finalErr, errLogFileRetirement)
		}
	}()
	if owner.opening || owner.openAttempted && !owner.openReturned || owner.ioPending || owner.closeAttempted && owner.file != nil {
		returned = true
		return errLogFileRetirement
	}
	if owner.file == nil {
		returned = true
		return nil // no acquisition, or this original Close already succeeded
	}
	owner.closeAttempted = true
	err := owner.file.Close()
	owner.closeReturned = true
	returned = true
	if err != nil {
		return errors.Join(errLogFileRetirement, err)
	}
	owner.file = nil
	return nil
}
