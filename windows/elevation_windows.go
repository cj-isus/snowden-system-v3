package main

import (
	"context"
	"os"
	"syscall"
)

// This is a source/lifetime precondition, not atomic OS launch/child custody.
// Once the OS has accepted a request, a later cancellation cannot report it as
// never launched. The original UAC request still needs a child-auth bridge.
func restartRequestCurrent(ctx context.Context, guard func(context.Context) error) error {
	if ctx == nil || guard == nil || !lifecyclePublicationCurrent(ctx) {
		return context.Canceled
	}
	if err := guard(ctx); err != nil {
		return err
	}
	if !lifecyclePublicationCurrent(ctx) {
		return context.Canceled
	}
	return nil
}

// Use Go's existing Windows argument codec, including empty strings, embedded
// quotes and trailing backslashes. ComposeCommandLine's first argument has
// executable-specific rules and must not be used on this argument alone.
func quoteArg(a string) string {
	return syscall.EscapeArg(a)
}

// tunRequested — запрошен ли TUN-режим флагом командной строки (--tun).
// Флаг задаётся при elevated-перезапуске из UI (EnableTUN/DisableTUN).
func tunRequested() bool {
	if activeElevatedGUISession != nil {
		tun, _ := activeElevatedGUISession.Mode()
		return tun
	}
	if activeRestartGUISession != nil {
		tun, _ := activeRestartGUISession.Mode()
		return tun
	}
	if activeUpdateGUISession != nil {
		tun, _ := activeUpdateGUISession.Mode()
		return tun
	}
	for _, arg := range os.Args[1:] {
		if arg == "--tun" {
			return true
		}
	}
	return false
}
