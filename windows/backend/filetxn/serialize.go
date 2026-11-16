package filetxn

import (
	"sync"
)

var profileLocks sync.Map

// Serialize coordinates cooperating profile import/recovery and render floor
// publication in this process. It is deliberately not an interprocess lock.
// Acquire it before the vault mutex; never acquire it recursively.
func Serialize(journal string) func() {
	domain := journalFor(journal)
	domain.serial.Lock()
	return domain.serial.Unlock
}
