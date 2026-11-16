//go:build !windows

package core

import "context"

// Windows display facts have no implicit Go-resolver fallback on another OS.
type egressAddressQuery struct{}

func (*egressAddressQuery) lookup(context.Context, string) ([]string, error) {
	return nil, ErrEgressObservation
}

func (*egressAddressQuery) retire() error { return nil }
