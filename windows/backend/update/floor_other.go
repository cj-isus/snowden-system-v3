//go:build !windows

package update

func lockFloor(string) (func() error, error) { return nil, ErrFloorUnavailable }

func writeFloorFile(string, []byte) error { return ErrFloorUnavailable }
