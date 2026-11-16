//go:build !windows

package repository

import "errors"

func publisherPath(string) error { return errors.New("repository: publisher store requires Windows") }

func publisherWriter(string, bool) (func() error, error) {
	return nil, errors.New("repository: publisher store requires Windows writer ownership")
}
