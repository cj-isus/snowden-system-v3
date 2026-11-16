//go:build !windows

package imagecheck

import "context"

func ReadAt(context.Context, uintptr, int64, int64, []byte, int64, func(context.Context) error) (int, error) {
	return 0, ErrImage
}
