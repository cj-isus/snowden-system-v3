//go:build !windows

package main

import "errors"

func openNativeTUNSocket() (nativeTUNSocket, error) {
	return nil, errors.New("native TUN socket requires Windows")
}
