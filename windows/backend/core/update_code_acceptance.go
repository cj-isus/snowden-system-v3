package core

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/update"
)

const updateCodeAcceptanceArgument = "--internal-update-code-acceptance-v5"
const updateCodeAcceptanceReplySize = 33

// Initial, unmeasured ceilings for the existing read-only SDK check worker.
// Job committed memory is not RSS; user-mode CPU time is not wall time/rate.
const updateCodeCheckCommitBytes uintptr = 512 << 20
const updateCodeCheckUserTime = 20 * time.Second

func updateCodeCheckArgument(argument string) bool {
	return argument == updateCodeAcceptanceArgument || argument == updateRecoveryCheckArgument
}

var ErrUpdateCodeAcceptance = errors.New("new application code acceptance unavailable")

// Dedicated installer owner, independent of both network diagnostic lanes.
// App/updateMu serializes use. The process/Job retires before response bytes
// become usable; the prospective app/helper image stays owned until Close.
// A successful Verify is not GUI readiness, floor promotion or installation.
type UpdateCodeAcceptance struct {
	mu          sync.Mutex // serializes verification/transfer/Close before lane ownership
	lane        helperCaptureLane
	manifestSHA string
	argument    string // one of the two fixed internal modes, never public command text
	want        [updateCodeAcceptanceReplySize]byte
	verified    bool
	gui         *updateGUITransfer
	restart     *restartGUIBinding // ordinary restart purpose, never an SDK proof
	ancestor    guiAncestor        // exact process bootstrap reference, captured once for GUI handoff
}

type acceptanceApplicationImage struct{ pair *InstalledPairLease }

func (image *acceptanceApplicationImage) Executable() string {
	if image == nil || image.pair == nil {
		return ""
	}
	return installedApplicationExecutable(image.pair.image)
}

func (image *acceptanceApplicationImage) Validate(ctx context.Context) error {
	if image == nil {
		return ErrNativeImageUnavailable
	}
	return image.pair.Validate(ctx)
}

func (image *acceptanceApplicationImage) ValidateProcess(ctx context.Context, host processHost) error {
	if err := image.Validate(ctx); err != nil {
		return err
	}
	return validateInstalledApplicationProcess(ctx, image.pair.image, host)
}

func (image *acceptanceApplicationImage) Close() error { return image.pair.Close() }

func OpenUpdateCodeAcceptance(ctx context.Context, manifest *update.Manifest, manifestSHA string, guard func(context.Context) error) (owner *UpdateCodeAcceptance, finalErr error) {
	finalErr = RetainUpdateCodeAcceptance(ctx, &owner, manifest, manifestSHA, guard)
	return owner, finalErr
}

// Store the original outer owner before a guard or native image preparation can
// exit without returning. The caller serializes this retained slot and its Close.
func RetainUpdateCodeAcceptance(ctx context.Context, slot **UpdateCodeAcceptance, manifest *update.Manifest, manifestSHA string, guard func(context.Context) error) (finalErr error) {
	if slot == nil || *slot != nil {
		return ErrHelperCleanup
	}
	owner := &UpdateCodeAcceptance{manifestSHA: manifestSHA, argument: updateCodeAcceptanceArgument}
	*slot = owner
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrUpdateCodeAcceptance)
		}
	}()
	finalErr = owner.prepare(ctx, manifest, guard)
	completed = true
	return finalErr
}

func (owner *UpdateCodeAcceptance) prepare(ctx context.Context, manifest *update.Manifest, guard func(context.Context) error) error {
	if owner == nil || owner.lane.image != nil || owner.gui != nil {
		return ErrHelperCleanup
	}
	digest, err := nativeImagePin(owner.manifestSHA)
	if err != nil || guard == nil || manifest == nil || manifest.Validate() != nil || manifest.Schema != update.PairSchema {
		return ErrUpdateCodeAcceptance
	}
	owner.want[0] = byte(update.PairActivationProtocol)
	copy(owner.want[1:], digest[:])
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	image := &acceptanceApplicationImage{}
	owner.lane.image = image // partial original image owner is retained on every error
	err = RetainInstalledPair(ctx, &image.pair, manifest.SHA256, manifest.Size, manifest.NativeHelper.SHA256, manifest.NativeHelper.Size)
	if err != nil {
		return err
	}
	return helperScope(ctx, guard) // repeat actual authority after potentially long hashing
}

func (owner *UpdateCodeAcceptance) Verify(ctx context.Context, guard func(context.Context) error) error {
	if owner == nil || guard == nil {
		return ErrUpdateCodeAcceptance
	}
	if !owner.mu.TryLock() {
		return ErrHelperBusy
	}
	defer owner.mu.Unlock()
	if owner.gui != nil || owner.restart != nil {
		return ErrUpdateGUIBootstrap // never feed a persistent GUI into helper retirement
	}
	owner.verified = false
	if !updateCodeCheckArgument(owner.argument) {
		return ErrUpdateCodeAcceptance
	}
	output, err := captureOwnedHelper(&owner.lane, ctx, "", []string{owner.argument, owner.manifestSHA}, updateCodeAcceptanceReplySize,
		helperCaptureOptions{keepImage: true, applicationEnvironment: true, guard: guard})
	if err != nil {
		return errors.Join(ErrUpdateCodeAcceptance, err)
	}
	if !bytes.Equal(output, owner.want[:]) {
		return ErrUpdateCodeAcceptance
	}
	owner.verified = true
	return nil
}

func (owner *UpdateCodeAcceptance) Close() error {
	if owner == nil {
		return nil
	}
	if !owner.mu.TryLock() {
		return errors.Join(ErrHelperCleanup, ErrHelperBusy)
	}
	defer owner.mu.Unlock()
	if !owner.lane.TryLock() {
		return errors.Join(ErrHelperCleanup, ErrHelperBusy)
	}
	defer owner.lane.Unlock()
	if owner.gui.accepted() {
		return ErrUpdateGUICommitted // retained until old process exit, never terminate the transferred GUI
	}
	if err := retireHelperLane(&owner.lane); err != nil {
		return err
	}
	owner.gui = nil // unaccepted preparation retired, including original process/I/O/image
	return nil
}

// Called before Wails/single-instance wait/housekeeping. This fixed internal
// mode accepts one exact candidate without committing retained trust/install
// state, then exits. The SDK cache uses the existing bounded memory store.
// Invalid flags/pipes/Job resources, errors or panic never become a GUI launch.
func DispatchUpdateCodeAcceptance(args []string, accept func(context.Context, string) error) (handled bool, exitCode int) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-update-code-acceptance") {
			handled = true
			break
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
	if len(args) != 2 || args[0] != updateCodeAcceptanceArgument || accept == nil || !workerPipesReady() || !updateCodeCheckResourcesReady() {
		return true, 2
	}
	digest, err := nativeImagePin(args[1])
	if err != nil {
		return true, 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), helperCaptureTimeout)
	defer cancel()
	if err := accept(ctx, args[1]); err != nil || ctx.Err() != nil {
		return true, 3
	}
	if !updateCodeCheckResourcesReady() || ctx.Err() != nil {
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
