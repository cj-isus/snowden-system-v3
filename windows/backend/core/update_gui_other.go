//go:build !windows

package core

import "context"

func prepareUpdateGUIHandles(context.Context, processHost) ([2]uint64, error) {
	return [2]uint64{}, ErrUpdateGUIBootstrap
}

func closeUpdateGUIStreams() error { return ErrUpdateGUIBootstrap }
