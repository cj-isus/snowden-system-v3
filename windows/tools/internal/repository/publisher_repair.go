package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/filetxn"
)

// Repair restores aliases of the existing accepted publication. It neither
// imports a candidate supplied by the caller nor changes any trust/floor/head.
// Expired publications may be repaired; that does not make them servable.
func repairPublisherAliases(approvedStateSHA, directory string) (report []byte, finalErr error) {
	if !canonicalHash(approvedStateSHA) {
		return nil, errors.New("repository: independent retained-state SHA256 approval required")
	}
	store, release, err := openExistingPublisher(directory)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if err := release(); err != nil {
			if committed {
				err = errors.Join(errors.New("repository: alias repair committed; writer release failed"), err)
			}
			finalErr = errors.Join(finalErr, err)
		}
	}()
	source, err := store.capture()
	if err != nil {
		return nil, err // journal recovery remains a separate explicit operation
	}
	if !source.present[0] || source.state == nil || source.state.Published == nil || digest(source.raw[0]) != approvedStateSHA {
		return nil, errors.New("repository: approved existing publication checkpoint required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	view, err := readAcceptedView(ctx, store, source)
	if err != nil {
		return nil, err
	}
	gate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := store.capture()
		if err != nil {
			return err
		}
		if err = samePublisherSource(source, current); err != nil {
			return err
		}
		return view.stage.check(store, ctx.Err)
	}
	if err = gate(); err != nil {
		return nil, err
	}
	files := store.targets() // state stays Keep, including exact schema/history bytes
	files[1].Keep, files[1].Data = false, view.current
	files[2].Keep, files[2].Data = false, view.stage.files["metadata/root.json"]
	files[3].Keep, files[3].Data = false, view.stage.files["metadata/timestamp.json"]
	journal := filetxn.Store{Dir: store.journal, BeforeCommit: gate}
	if err = journal.Commit(files); err != nil {
		return nil, err
	}
	committed = true
	if _, err = store.read(); err != nil {
		return nil, errors.Join(errors.New("repository: alias repair committed; final consistency read failed"), err)
	}
	return json.Marshal(struct {
		Schema                 int    `json:"schema"`
		Operation              string `json:"operation"`
		ReleaseID              string `json:"release_id"`
		TrustChanged           bool   `json:"trust_changed"`
		RequiresFreshTimeCheck bool   `json:"requires_fresh_time_check"`
	}{1, "local_repository_alias_repair", view.head.ReleaseID, false, true})
}
