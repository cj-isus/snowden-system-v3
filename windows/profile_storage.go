package main

import (
	"errors"

	"github.com/snowden-system/windows/backend/filetxn"
)

var errProfileStorageQuarantine = errors.New(profileStorageQuarantineMessage)

const profileStorageQuarantineMessage = "Хранилище требует восстановления. Подключение заблокировано. Изменения уже могли быть сохранены; не повторяйте импорт для устранения этой ошибки. Повторная попытка в этом процессе заблокирована; сохраните диагностику для восстановления."
const profileStorageCleanupMessage = "Очистка служебных временных данных отложена. Завершённая запись не отменяется; не повторяйте импорт только ради очистки. Это предупреждение не подтверждает готовность подключения или устойчивость записи при сбое питания."

// Memory-only process facts, not a disk scan, recovery or readiness verdict.
// Clean absence of an observed fault is deliberately not called healthy/ready.
func profileStorageFacts(journal string) (state, warning string) {
	store := &filetxn.Store{Dir: journal}
	if store.QuarantineError() != nil {
		return "recovery_required", profileStorageQuarantineMessage
	}
	if store.CleanupError() != nil {
		return "cleanup_pending", profileStorageCleanupMessage
	}
	return "not_observed", ""
}

// Inspect after the vault transaction callback has returned. Never turn benign
// staging debris into a failed logical commit; original-resource quarantine is
// a separate barrier that must suppress activation of a fresh session.
func profileStorageQuarantineError(tx *filetxn.Store) error {
	if err := tx.QuarantineError(); err != nil {
		return errors.Join(errProfileStorageQuarantine, err)
	}
	return nil
}

// Caller owns operationMu and has released journal/vault/native profile scopes.
// Separate installation-domain custody can remain held by the updater.
// Even a cache-only write can leave quarantined resources while the old session
// was not otherwise scheduled for retirement. Reuse its actual lifecycle owner.
func (a *App) retireProfileStorageFailure(expected, transition uint64, cause error) (uint64, error) {
	if !errors.Is(cause, errProfileStorageQuarantine) || transition != 0 {
		return transition, cause
	}
	id, err := a.retireProfileSession(expected)
	return id, errors.Join(cause, err)
}
