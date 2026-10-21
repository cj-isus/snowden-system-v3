package filetxn

import (
	"path/filepath"
	"strings"
	"sync"
)

var profileLocks sync.Map

// Serialize coordinates cooperating profile import/recovery and render floor
// publication in this process. It is deliberately not an interprocess lock.
// Acquire it before the vault mutex; never acquire it recursively.
func Serialize(journal string) func() {
	key, err := filepath.Abs(journal)
	if err != nil {
		key = filepath.Clean(journal)
	}
	value, _ := profileLocks.LoadOrStore(strings.ToLower(key), &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
