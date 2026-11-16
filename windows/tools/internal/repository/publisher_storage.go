package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const maxPublisherPublicBytes int64 = 256 << 20
const maxPublisherPublicFiles = 8192
const maxPublisherPublicDirs = 2048

// A cooperating-writer admission bound on public logical bytes/entries, not a
// filesystem quota, free-space reservation or RSS guarantee. Include abandoned
// stage/release objects. No public file is deleted or adopted as authority.
func (s *publisherStore) admitPublicStorage(ctx context.Context, pending map[string][]byte) error {
	lengths := make(map[string]int64, len(pending))
	for name, raw := range pending {
		lengths[name] = int64(len(raw))
	}
	return s.admitPublicRecords(ctx, lengths)
}

// Restore admits exact verified record lengths without loading the whole backup
// into a map of byte slices. Ordinary publication uses the same budget core.
func (s *publisherStore) admitPublicRecords(ctx context.Context, pending map[string]int64) error {
	if ctx == nil {
		return errors.New("repository: storage context required")
	}
	base := filepath.Join(s.root, "public")
	usage, err := walkPublisherTree(ctx, base, publisherUsage{maxPublisherPublicBytes, maxPublisherPublicFiles, maxPublisherPublicDirs}, nil)
	if err != nil {
		return err
	}
	total, files, dirs := usage.bytes, usage.files, usage.dirs
	missingDirs := map[string]bool{}
	for filename, length := range pending {
		if length < 0 || length > maxPublisherPublicBytes {
			return errors.New("repository: invalid pending storage length")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(base, filename)
		if err != nil || rel == "." || !filepath.IsLocal(rel) || strings.Contains(rel, ":") {
			return errors.New("repository: storage admission path outside public namespace")
		}
		if err = publisherPath(filename); err != nil {
			return err
		}
		info, err := os.Lstat(filename)
		if err == nil {
			if !info.Mode().IsRegular() {
				return errors.New("repository: existing immutable path is not regular")
			}
			continue // writeImmutable separately requires exact existing bytes
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		files++
		if files > maxPublisherPublicFiles || length > maxPublisherPublicBytes-total {
			return errors.New("repository: publication would exceed public storage budget")
		}
		total += length
		for dir := filepath.Dir(filename); dir != base; dir = filepath.Dir(dir) {
			if missingDirs[dir] {
				continue
			}
			info, err = os.Lstat(dir)
			if errors.Is(err, os.ErrNotExist) {
				missingDirs[dir] = true
				if dirs+len(missingDirs) > maxPublisherPublicDirs {
					return errors.New("repository: publication directory budget exhausted")
				}
			} else if err != nil || !info.IsDir() {
				return errors.Join(errors.New("repository: invalid publication ancestor"), err)
			}
		}
	}
	// Reserve mutable aliases and one maximal temporary immutable file. Private
	// journal/state copies and physical disk allocation are a separate OS gate.
	if total > maxPublisherPublicBytes-(16<<20) || files > maxPublisherPublicFiles-4 {
		return errors.New("repository: public storage staging reserve exhausted")
	}
	return ctx.Err()
}
