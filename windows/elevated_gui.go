package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/snowden-system/windows/backend/core"
)

var activeElevatedGUISession *core.ElevatedGUISession // fixed before Wails, including partial original owners

// operationMu held. Broker final wire custody is separate from GUI/VPN readiness;
// compiled parent/chain/bridge/TUN acceptance gates remain closed independently.
func (a *App) launchElevatedModeGUI(ctx context.Context, h *restartHandoff, resume bool, guard func(context.Context) error) (accepted bool, finalErr error) {
	if h == nil || h.app != a || a.elevatedGUI != nil {
		return false, core.ErrHelperCleanup
	}
	if err := restartRequestCurrent(ctx, guard); err != nil {
		return false, err
	}
	a.elevatedGUI = core.NewBrokerGUIRequest(nativeProbeHelperSHA256) // original owner before any image/COM/native acquisition
	original := a.elevatedGUI
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, core.ErrElevatedGUIUnavailable)
		}
		if original.Transferred() {
			if !completed {
				a.closing.Store(true)
				accepted = true
				finalErr = errors.Join(finalErr, core.ErrElevatedGUITransferred)
				a.requestQuit(300 * time.Millisecond)
			}
			return // original process/Job/image/token retained until terminal parent exit
		}
		if a.elevatedGUI != original {
			finalErr = errors.Join(finalErr, core.ErrHelperCleanup)
			return
		}
		if err := original.Close(); err != nil {
			finalErr = errors.Join(finalErr, core.ErrHelperCleanup, err)
		} else {
			a.elevatedGUI = nil
		}
	}()
	err := original.Execute(ctx, resume, guard)
	if err != nil {
		if errors.Is(err, core.ErrElevatedGUIDeclined) {
			return false, fmt.Errorf("запрос прав администратора отклонён: %w", err)
		}
		return false, err
	}
	receipt, err := original.Commit(ctx, guard, func(commit core.ElevatedGUICommit) error { return h.authorizeElevatedRequest(ctx, commit) })
	if finishErr := h.finishElevatedRequest(receipt); finishErr != nil {
		return false, errors.Join(err, finishErr)
	}
	accepted, completed = true, true
	return true, err
}
