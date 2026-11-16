package childbootstrap

import (
	"context"
	"errors"
	"time"
)

var ErrScope = errors.New("child bootstrap source unavailable")

// CheckScope keeps the caller's original value-only source check. It creates no
// context/timer and checks the real deadline even when timer delivery is delayed.
// Native-owner callers must retain partial resources if the callback fails/exits.
func CheckScope(ctx context.Context, check func(context.Context) error) error {
	if ctx == nil || check == nil {
		return ErrScope
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, bounded := ctx.Deadline(); bounded && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	if err := check(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, bounded := ctx.Deadline(); bounded && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}
