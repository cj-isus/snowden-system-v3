package filetxn

import "errors"

// One operation's caller-fixed target namespace. This is neither a manifest
// authority nor a second journal. Retain before Acquire, including its failures.
type journalTargets interface {
	Acquire([]File) error
	Matches([]File) bool
	Validate() error
	Check(path string) error
	Erase(path string) error
	Close() error
}

func (domain *journalDomain) pinTargets(files []File) error {
	if err := domain.fault(); err != nil {
		return err
	}
	if domain.targets != nil {
		if !domain.targets.Matches(files) {
			return domain.fail(ErrRecoveryRequired)
		}
		return domain.checkTargets() // internal rollback keeps the original scope
	}
	domain.targets = newJournalTargets()
	if domain.targets == nil {
		return nil // portable compatibility, no native parent custody claim
	}
	return domain.targets.Acquire(files)
}

func (domain *journalDomain) checkTargets() error {
	if err := domain.checkWriter(); err != nil {
		return err
	}
	if domain.targets != nil {
		if err := domain.targets.Validate(); err != nil {
			return domain.fail(err)
		}
	}
	return nil
}

func (domain *journalDomain) checkTarget(path string) error {
	if err := domain.fault(); err != nil {
		return err
	}
	if domain.targets != nil {
		if err := domain.targets.Check(path); err != nil {
			return domain.fail(err)
		}
	}
	return nil
}

func (domain *journalDomain) readTarget(path string) ([]byte, bool, error) {
	if err := domain.checkTarget(path); err != nil {
		return nil, true, err
	}
	return domain.read(path, maxFileSize)
}

// Run after journal rollback/cleanup and before releasing the operation gate.
// An already completed transaction stays completed; a subsequent failed Close
// is visible through Check and CleanupError and never permits a new operation.
func (domain *journalDomain) retireTargets(finalErr *error, completed bool) {
	if domain.targets == nil || domain.fault() != nil {
		return // uncertain original resources stay owned by this journal domain
	}
	if err := invoke(domain.targets.Close); err != nil {
		fault := domain.fail(err)
		domain.cleanupStatus.Store(&journalFailure{err: errors.Join(ErrStagingCleanup, fault)})
		if !completed {
			*finalErr = errors.Join(*finalErr, fault)
		}
		return
	}
	domain.targets = nil
}
