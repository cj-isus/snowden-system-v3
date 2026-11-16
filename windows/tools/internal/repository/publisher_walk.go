package repository

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type publisherUsage struct {
	bytes int64
	files int
	dirs  int
}

// Reused for admission and explicit maintenance. Bounded batched enumeration
// never follows reparse entries or feeds discovered paths into trust decisions.
func walkPublisherTree(ctx context.Context, base string, limit publisherUsage, visit func(string, os.FileInfo) error) (usage publisherUsage, finalErr error) {
	if ctx == nil || limit.bytes < 1 || limit.files < 1 || limit.dirs < 1 {
		return usage, errors.New("repository: bounded inventory context required")
	}
	var scan func(string, int) error
	scan = func(dir string, depth int) (finalErr error) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 32 || usage.dirs >= limit.dirs {
			return errors.New("repository: directory inspection budget exhausted")
		}
		if err := publisherPath(dir); err != nil {
			return err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return errors.Join(errors.New("repository: inventory directory required"), err)
		}
		f, err := os.Open(dir)
		if err != nil {
			return err
		}
		defer func() { finalErr = errors.Join(finalErr, f.Close()) }()
		opened, err := f.Stat()
		if err != nil || !os.SameFile(info, opened) || !opened.IsDir() {
			return errors.New("repository: inventory directory changed while opening")
		}
		usage.dirs++
		if visit != nil {
			if err = visit(dir, opened); err != nil {
				return err
			}
		}
		for {
			entries, readErr := f.ReadDir(128)
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return readErr
			}
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}
				name := filepath.Join(dir, entry.Name())
				if err := publisherPath(name); err != nil {
					return err
				}
				info, err := os.Lstat(name)
				if err != nil {
					return err
				}
				if info.IsDir() {
					if err = scan(name, depth+1); err != nil {
						return err
					}
					continue
				}
				if !info.Mode().IsRegular() || info.Size() < 0 {
					return errors.New("repository: nonregular inventory object")
				}
				usage.files++
				if usage.files > limit.files || info.Size() > limit.bytes-usage.bytes {
					return errors.New("repository: file inspection budget exhausted")
				}
				usage.bytes += info.Size()
				if visit != nil {
					if err = visit(name, info); err != nil {
						return err
					}
				}
			}
			if errors.Is(readErr, io.EOF) {
				return ctx.Err()
			}
		}
	}
	err := scan(base, 0)
	return usage, err
}
