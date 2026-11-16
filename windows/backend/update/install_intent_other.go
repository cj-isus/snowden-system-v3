//go:build !windows

package update

// No non-Windows write fallback for the Windows install barrier. Structural
// publishers can still read an initial floor without invoking a native store.
func readPairStateNative() ([]byte, error)   { return nil, nil }
func lockPairStartup() (func() error, error) { return nil, nil }

type unsupportedPairStateWrite struct{}

func newPairStateWrite(_, _ []byte) pairStateWrite { return unsupportedPairStateWrite{} }
func (unsupportedPairStateWrite) Set() error       { return ErrPairStateUnavailable }
func (unsupportedPairStateWrite) Finish() error    { return nil }
