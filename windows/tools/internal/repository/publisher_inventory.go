package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/snowden-system/windows/backend/delivery"
	"github.com/snowden-system/windows/tools/internal/publisherfiles"
)

type publisherRecord struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
	SHA256 string `json:"sha256"`
}

type publisherInventory struct {
	files    map[string]publisherRecord // canonical case-folded relative lookup
	releases map[string]bool
	records  []publisherRecord // sorted exact paths for backup/protected digest
	sha256   string
}

// Rebuild solely from the private checkpoint: discovered public files can be
// storage candidates, never accepted roots/heads. Expired retained publications
// remain protected; copying them cannot renew their validity.
func (s *publisherStore) protectedInventory(ctx context.Context, source *publisherSnapshot) (*publisherInventory, error) {
	read := func(name string, limit int64) ([]byte, error) {
		filename := filepath.Join(s.root, filepath.FromSlash(name))
		if err := publisherPath(filename); err != nil {
			return nil, err
		}
		return publisherfiles.Read(filename, limit)
	}
	out, err := s.checkpointInventory(ctx, source, read)
	if err != nil {
		return nil, err
	}
	if err = s.fence(source); err != nil {
		return nil, err
	}
	return out, ctx.Err()
}

// Derives one exact accepted inventory through a bounded object reader. Control
// bytes are the caller's approved checkpoint snapshot; the caller separately
// fences the real source/target. This supports a virtual backup without extraction.
func (s *publisherStore) checkpointInventory(ctx context.Context, source *publisherSnapshot, read func(string, int64) ([]byte, error)) (*publisherInventory, error) {
	if ctx == nil || source == nil || source.state == nil || read == nil {
		return nil, delivery.ErrTrust
	}
	out := &publisherInventory{files: map[string]publisherRecord{}, releases: map[string]bool{}}
	mark := func(filename string, raw []byte, verify bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, filename)
		if err != nil || rel == "." || !filepath.IsLocal(rel) {
			return errors.New("repository: protected object outside publisher store")
		}
		rel = filepath.ToSlash(rel)
		record := publisherRecord{Path: rel, Length: int64(len(raw)), SHA256: digest(raw)}
		key := strings.ToLower(rel)
		if old, exists := out.files[key]; exists {
			if old != record {
				return errors.New("repository: conflicting protected object identity")
			}
			return nil
		}
		if verify {
			actual, err := read(rel, int64(len(raw)))
			if err != nil || !bytes.Equal(actual, raw) {
				return errors.Join(errors.New("repository: accepted checkpoint object changed"), err)
			}
		}
		if len(out.files) >= 2*maxPublisherPublicFiles {
			return delivery.ErrBudget
		}
		out.files[key] = record
		return nil
	}
	for i, present := range source.present {
		if present {
			if err := mark(s.paths[i], source.raw[i], false); err != nil {
				return nil, err
			}
		}
	}
	heads := append([]publisherHead(nil), source.state.History...)
	if source.state.Published != nil {
		heads = append(heads, *source.state.Published)
	}
	for _, head := range heads {
		out.releases[head.ReleaseID] = true
		name := "public/" + head.Directory + "/candidates/" + head.CandidateSHA256 + ".json"
		raw, err := read(name, delivery.MaxEncodedPackageBytes)
		if err != nil {
			return nil, err
		}
		view, err := decodeAcceptedHead(raw, head)
		if err != nil {
			return nil, err
		}
		if err = mark(filepath.Join(s.root, filepath.FromSlash(name)), raw, false); err != nil {
			return nil, err
		}
		if err = view.stage.visit(s, func(filename string, raw []byte) error { return mark(filename, raw, true) }); err != nil {
			return nil, err
		}
	}
	var previous *document
	for _, root := range source.state.Roots {
		name := fmt.Sprintf("public/metadata/%d.root.json", root.Version)
		raw, err := read(name, delivery.MaxMetadataBytes)
		if err != nil {
			return nil, err
		}
		doc, err := decodeDocument("root", raw)
		if err != nil || doc.version() != root.Version || digest(raw) != root.SHA256 {
			return nil, errors.New("repository: checkpoint root differs from retained index")
		}
		if err = doc.verify(doc); err != nil {
			return nil, err
		}
		if previous != nil {
			if err = previous.verify(doc); err != nil {
				return nil, err
			}
		}
		previous = doc
		if err = mark(filepath.Join(s.root, filepath.FromSlash(name)), raw, false); err != nil {
			return nil, err
		}
	}
	for _, record := range out.files {
		out.records = append(out.records, record)
	}
	sort.Slice(out.records, func(i, j int) bool { return out.records[i].Path < out.records[j].Path })
	raw, err := json.Marshal(out.records)
	if err != nil {
		return nil, err
	}
	out.sha256 = digest(raw)
	return out, ctx.Err()
}

// The only deletion namespace. All shared metadata/targets, private files and
// every object beneath a retained release ID are excluded, including orphans.
func retiredPublisherPath(name string, directory bool, protected *publisherInventory) bool {
	if protected == nil || len(name) > 400 || !strings.HasPrefix(name, "public/releases/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(name, "public/releases/"), "/")
	if !canonicalHash(parts[0]) || protected.releases[parts[0]] {
		return false
	}
	if len(parts) == 1 {
		return directory
	}
	rel := strings.Join(parts[1:], "/")
	if directory {
		rel += "/probe.json" // validate the same portable component grammar
	}
	if !strings.HasPrefix(rel, "metadata/") && !strings.HasPrefix(rel, "targets/") && !strings.HasPrefix(rel, "clients/") && !strings.HasPrefix(rel, "candidates/") {
		return false
	}
	portable := rel
	leaf := rel[strings.LastIndex(rel, "/")+1:]
	if !directory && strings.HasPrefix(leaf, ".publisher-stage-") {
		portable += ".json" // exact approved stale publisher temp in a retired view
	}
	if !publisherfiles.RelativeJSON(portable) || !filepath.IsLocal(filepath.FromSlash(rel)) || protected.files[strings.ToLower(name)].Path != "" {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if strings.HasSuffix(part, ".") {
			return false
		}
	}
	return true
}
