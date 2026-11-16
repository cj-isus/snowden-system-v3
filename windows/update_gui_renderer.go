package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/snowden-system/windows/backend/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const updateRendererChallenge = "app:renderer-challenge-v1"
const updateRendererReceipt = "app:renderer-mounted-v1"

// Wails calls OnDomReady from navigation-completed, even without checking
// navigation success. Require the mounted Vue root to answer this fresh nonce.
// This is renderer liveness only, never source/trust/protection authority.
func awaitUpdateGUIRenderer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return core.ErrUpdateGUIBootstrap
	}
	token := hex.EncodeToString(nonce[:])
	received := make(chan struct{}, 1)
	off := runtime.EventsOn(ctx, updateRendererReceipt, func(data ...interface{}) {
		if len(data) != 1 {
			return
		}
		value, ok := data[0].(string)
		if !ok || len(value) != 64 || value != token {
			return
		}
		select {
		case received <- struct{}{}:
		default:
		}
	})
	defer off()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return core.ErrUpdateGUIBootstrap
		}
		// Bounded retransmission avoids an OnStartup/renderer registration race.
		runtime.EventsEmit(ctx, updateRendererChallenge, token)
		select {
		case <-received:
			if ctx.Err() != nil {
				return core.ErrUpdateGUIBootstrap
			}
			return nil
		case <-ctx.Done():
			return core.ErrUpdateGUIBootstrap
		case <-ticker.C:
		}
	}
}
