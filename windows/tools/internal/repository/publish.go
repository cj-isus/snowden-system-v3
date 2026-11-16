package repository

import (
	"time"

	"github.com/snowden-system/windows/backend/metadata"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

func publishDocument(d, parent *document, raw []byte, threshold bool, output string, started time.Time) ([]byte, error) {
	reportBytes, err := report(d, raw, threshold)
	if err != nil {
		return nil, err
	}
	check := func() error {
		now := time.Now().UTC()
		if err := metadata.CheckClockFloor(now, started); err != nil {
			return err
		}
		if now.Before(started) {
			now = started
		}
		if err := d.fresh(now); err != nil {
			return err
		}
		if parent != nil && d.root == nil {
			return parent.fresh(now)
		}
		// An expired old root can authorize the next root through its retained
		// threshold. The new root must be fresh; no new bootstrap is implied.
		return nil
	}
	if err := publisherfiles.PublishNewChecked(output, raw, check); err != nil {
		return nil, err
	}
	return reportBytes, nil
}
