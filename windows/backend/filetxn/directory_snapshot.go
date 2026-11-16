package filetxn

import "os"

// snapshotDirectory returns an immutable identity after the original checked
// Close. A snapshot does not retain custody across path operations or grant
// ownership of entries discovered at that path.
func (slot *BoundedReadSlot) snapshotDirectory(path string) (info os.FileInfo, finalErr error) {
	if slot == nil || !slot.mu.TryLock() {
		return nil, ErrReadBusy
	}
	defer slot.mu.Unlock()
	if slot.file != nil || slot.closeErr != nil || slot.acquiring {
		return nil, ErrReadClose
	}
	defer func() {
		if recover() != nil {
			finalErr = ErrBoundedRead
		}
		if err := slot.close(); err != nil {
			finalErr = err
		}
		if finalErr != nil {
			info = nil
		}
	}()
	slot.acquiring = true
	err := slot.open(path, 0, true)
	slot.acquiring = false
	if err != nil {
		return nil, err
	}
	info, err = slot.file.Stat()
	if err != nil || info == nil || !info.IsDir() {
		return nil, ErrBoundedRead
	}
	return info, nil
}
