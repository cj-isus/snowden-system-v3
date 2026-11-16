package update

import (
	"errors"
	"strings"
	"sync"
)

var floorMu sync.Mutex
var ErrFloorUnavailable = errors.New("update: version floor unavailable")

// Missing is an initial floor. Malformed, oversized, nonregular or unreadable
// state is an error, never a security decision made with a fabricated zero.
func ReadFloor() (Version, error) {
	floor, err := readFloorFile()
	if err != nil {
		return Version{}, err
	}
	state, _, err := readPairState()
	if err != nil {
		return Version{}, errors.Join(ErrFloorUnavailable, err)
	}
	durable, err := ParseVersion(state.Floor)
	if err != nil {
		return Version{}, errors.Join(ErrFloorUnavailable, err)
	}
	if durable.Compare(floor) > 0 {
		return durable, nil
	}
	return floor, nil
}

func readFloorFile() (Version, error) {
	path, err := FloorPath()
	if err != nil {
		return Version{}, errors.Join(ErrFloorUnavailable, err)
	}
	if pairAbsent(path) {
		return Version{}, nil
	}
	raw, err := pairRead(path, 64)
	if err != nil {
		return Version{}, errors.Join(ErrFloorUnavailable, err)
	}
	version, err := ParseVersion(strings.TrimSpace(string(raw)))
	if err != nil {
		return Version{}, errors.Join(ErrFloorUnavailable, err)
	}
	return version, nil
}

func raiseFloor(version Version) (actual Version, finalErr error) {
	floorMu.Lock()
	defer floorMu.Unlock()
	if parsed, err := ParseVersion(version.String()); err != nil || parsed != version {
		return Version{}, ErrFloorUnavailable
	}
	path, err := FloorPath()
	if err != nil {
		return Version{}, err
	}
	release, err := lockFloor(path + ".lock")
	if release != nil {
		defer func() { finalErr = errors.Join(finalErr, release()) }()
	}
	if err != nil {
		return Version{}, err
	}
	current, err := ReadFloor() // repeat while holding the actual cross-process lock
	if err != nil {
		return Version{}, err
	}
	if current.Compare(version) >= 0 {
		return current, nil
	}
	if err := writeFloorFile(path, []byte(version.String())); err != nil {
		return Version{}, err
	}
	actual, err = ReadFloor() // checked close/readback; publication alone does not prove accepted state
	if err != nil || actual.Compare(version) < 0 {
		return Version{}, errors.Join(ErrFloorUnavailable, err)
	}
	return actual, nil
}
