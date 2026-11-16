package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/update"
)

const updateRecoveryArgument = "--internal-update-recovery-v5"
const updateRecoveryCheckArgument = "--internal-update-recovery-check-v5"

// Must run before every Wails/worker/other updater dispatch. The companion is
// never a GUI/engine fallback. No supplied root/source path, enrollment or old
// journal code grant. A cold invocation repairs only through the real callback.
func DispatchUpdateRecovery(args []string, check func(context.Context, string) error, repair func(context.Context) error) (handled bool, exitCode int) {
	exe, exeErr := os.Executable()
	companion := exeErr == nil && strings.EqualFold(filepath.Base(exe), update.RecoveryApplicationFile)
	handled = companion
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-update-recovery") {
			handled = true
		}
	}
	if !handled {
		return false, 0
	}
	exitCode = 3
	defer func() {
		if recover() != nil {
			exitCode = 3
		}
	}()
	if !companion || len(args) > 2 {
		return true, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel() // soft work budget, not a native I/O deadline
	if len(args) == 2 && args[0] == updateRecoveryCheckArgument {
		if check == nil || !workerPipesReady() || !updateCodeCheckResourcesReady() {
			return true, 2
		}
		digest, err := nativeImagePin(args[1])
		if err != nil {
			return true, 2
		}
		checkCtx, checkCancel := context.WithTimeout(ctx, helperCaptureTimeout)
		defer checkCancel()
		if err := check(checkCtx, args[1]); err != nil || checkCtx.Err() != nil {
			return true, 3
		}
		if !updateCodeCheckResourcesReady() || checkCtx.Err() != nil {
			return true, 3
		}
		var reply [updateCodeAcceptanceReplySize]byte
		reply[0] = byte(update.PairActivationProtocol)
		copy(reply[1:], digest[:])
		if n, err := os.Stdout.Write(reply[:]); err != nil || n != len(reply) {
			return true, 3
		}
		return true, 0
	}
	if len(args) != 0 && (len(args) != 1 || args[0] != updateRecoveryArgument) || repair == nil {
		return true, 2
	}
	if err := repair(ctx); err != nil || ctx.Err() != nil {
		return true, 3
	}
	return true, 0
}

// The same process/Job/image owner as current-code verification, with one fixed
// read-only companion mode. StartGUI rejects this owner; it proves no placement
// or acceptance. The App must hold the actual source/clock/lifecycle guard.
func OpenUpdateRecoveryCheck(ctx context.Context, manifest *update.Manifest, manifestSHA string, guard func(context.Context) error) (owner *UpdateCodeAcceptance, finalErr error) {
	owner = &UpdateCodeAcceptance{manifestSHA: manifestSHA, argument: updateRecoveryCheckArgument}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = ErrUpdateCodeAcceptance
		}
	}()
	digest, err := nativeImagePin(manifestSHA)
	if err != nil || guard == nil || manifest == nil || manifest.Validate() != nil || manifest.Schema != update.PairSchema {
		return owner, ErrUpdateCodeAcceptance
	}
	owner.want[0] = byte(update.PairActivationProtocol)
	copy(owner.want[1:], digest[:])
	if err := helperScope(ctx, guard); err != nil {
		return owner, err
	}
	image := &RecoveryImageLease{}
	owner.lane.image = image // original wrapper before its first native acquisition
	err = image.open(ctx, manifest.SHA256, manifest.Size)
	if err != nil {
		return owner, err
	}
	finalErr = helperScope(ctx, guard)
	completed = true
	return owner, finalErr
}

// Only after Verify returns: not callable recursively from its source guard.
// The private verified flag is set solely after the original Job is empty and
// the exact current-code receipt matches; retained image custody must still hold.
func (owner *UpdateCodeAcceptance) ValidateRecoveryReady(ctx context.Context) error {
	if owner == nil || !owner.mu.TryLock() {
		return ErrUpdateCodeAcceptance
	}
	defer owner.mu.Unlock()
	if !owner.lane.TryLock() {
		return ErrHelperBusy
	}
	defer owner.lane.Unlock()
	if owner.argument != updateRecoveryCheckArgument || !owner.verified || owner.gui != nil || owner.lane.image == nil {
		return ErrUpdateCodeAcceptance
	}
	return owner.lane.image.Validate(ctx)
}
