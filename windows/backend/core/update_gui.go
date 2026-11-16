package core

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

const updateGUIArgument = childbootstrap.ApplicationUpdateArgument
const updateGUIFrameSize = childbootstrap.FrameSize // fixed stage + manifest SHA or nonce
const updateGUISeedSize = childbootstrap.SeedSize

var ErrUpdateGUIBootstrap = errors.New("update GUI bootstrap unavailable")
var ErrUpdateGUICommitted = errors.New("update GUI custody transferred; original owner retained until parent exit")

type updateGUITransfer struct {
	handles   [3]uint64 // target-process values only; optional retained ancestor, never parent CloseHandle
	process   *engineProcess
	committed bool // OWN6/7 or OR01/02 preparation only; final ACK/App receipt remain separate
	decision  *updateGUICommitState
}

func validUpdateGUIArguments(args []string) bool {
	if len(args) != 4 {
		return false
	}
	switch args[0] {
	case updateGUIArgument, restartGUIArgument, childbootstrap.ApplicationUpdateAncestorArgument, childbootstrap.ApplicationRestartAncestorArgument:
	default:
		return false
	}
	_, _, _, err := childbootstrap.ParseApplicationArguments(args)
	return err == nil
}

func updateGUIFrame(stage string, digest [32]byte) [updateGUIFrameSize]byte {
	return childbootstrap.Frame(stage, digest)
}

// StartGUI prepares fixed OWN6/OR01 custody, without readiness or App acceptance.
// Caller must CommitGUI or Close this original preparation. Accepted custody
// retains process/Job/image until OS exit, without closing a pending wait handle.
func (owner *UpdateCodeAcceptance) StartGUI(ctx context.Context, tun, resume bool, guard func(context.Context) error) (finalErr error) {
	if owner == nil || guard == nil || !owner.mu.TryLock() {
		return ErrUpdateGUIBootstrap
	}
	defer owner.mu.Unlock()
	if !owner.lane.TryLock() {
		return ErrHelperBusy
	}
	defer owner.lane.Unlock()
	completed := false
	var process *engineProcess // exact original child, including partial spawn/late I/O failure
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrUpdateGUIBootstrap)
		}
		if finalErr != nil && !owner.gui.accepted() {
			if process != nil {
				process.unusable = true
			}
			if retirement := retireHelperProcess(&owner.lane); retirement != nil {
				finalErr = errors.Join(finalErr, ErrHelperCleanup, retirement)
			}
		}
		finalErr = errors.Join(finalErr, process.captureFailure())
	}()
	if !owner.verified || owner.gui != nil || owner.lane.owner != nil || owner.lane.image == nil {
		return ErrUpdateGUIBootstrap
	}
	owner.ancestor = processGUIAncestor // capture the private original reference; never retry without it
	protocol, err := owner.applicationGUIProtocol()
	if err != nil {
		return err
	}
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	if owner.ancestor != nil {
		if !ancestorRelaunchAccepted {
			return ErrUpdateGUIBootstrap
		}
		if err := owner.ancestor.validate(ctx); err != nil {
			return err
		}
	}
	if err := owner.lane.image.Validate(ctx); err != nil {
		return err
	}
	args, err := protocol.wire.ApplicationArguments(tun, resume)
	if err != nil {
		return ErrUpdateGUIBootstrap
	}
	owner.gui = &updateGUITransfer{}
	policy := ownedProcessLimits{processes: 1, cleanSystemEnvironment: true, applicationEnvironment: true, successorGUI: true}
	policy.beforeResume = func(ctx context.Context, host processHost) error {
		if err := owner.lane.image.ValidateProcess(ctx, host); err != nil {
			return err
		}
		if owner.ancestor != nil {
			if err := owner.ancestor.checkChild(ctx, host); err != nil {
				return err // in ancestor AND outside old role Job, still before Resume
			}
		}
		return helperScope(ctx, guard)
	}
	process = newOwnedProcess()
	owner.lane.owner = process
	owner.gui.process = process
	if process == nil {
		return ErrEngineWorkerUnavailable
	}
	err = process.spawn(ctx, owner.lane.image.Executable(), args, policy)
	if err != nil || owner.lane.owner == nil {
		return errors.Join(ErrUpdateGUIBootstrap, err)
	}
	// Only a resumed child receives a Job reference: it can now observe seed
	// EOF/deadline rather than retaining the Job while permanently suspended.
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	if err := owner.lane.image.ValidateProcess(ctx, process.host); err != nil {
		return err
	}
	legacyHandles, err := prepareUpdateGUIHandles(ctx, process.host)
	copy(owner.gui.handles[:2], legacyHandles[:])
	if err != nil {
		return err
	}
	if owner.ancestor != nil {
		owner.gui.handles[2], err = owner.ancestor.duplicate(ctx, process.host)
		if err != nil {
			return err // target owns all partial duplicates; original child/Job reaper remains
		}
	}
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	seed, err := protocol.wire.EncodeSeed(owner.gui.handles)
	if err != nil {
		return err
	}
	var reply [updateGUIFrameSize]byte
	if err := exchangeUpdateGUI(ctx, process.output, process.input, seed, reply[:], process); err != nil {
		return err
	}
	armed := updateGUIFrame(protocol.armed, protocol.binding)
	if !bytes.Equal(reply[:], armed[:]) || process.exited() {
		return ErrUpdateGUIBootstrap
	}
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	commit := updateGUIFrame(protocol.goStage, protocol.binding)
	if err := exchangeUpdateGUI(ctx, process.output, process.input, commit[:], reply[:], process); err != nil {
		return err
	}
	owned := updateGUIFrame(protocol.owned, protocol.binding)
	if !bytes.Equal(reply[:], owned[:]) || process.exited() {
		return ErrUpdateGUIBootstrap
	}
	if err := owner.lane.image.Validate(ctx); err != nil {
		return err
	}
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	if process.exited() {
		return ErrUpdateGUIBootstrap // preceding validation may have outlived this original child
	}
	owner.gui.committed = true
	completed = true
	return nil
}

func (owner *UpdateCodeAcceptance) GUITransferred() bool {
	if owner == nil {
		return false
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.gui.accepted()
}

// This same process is first the bootstrap coordinator and then the Wails GUI.
// Job is a non-inheritable lifetime reference; only OS exit releases it. The
// prospective image is retained until actual DOM-ready installation acceptance.
type UpdateGUISession struct {
	mu       sync.Mutex
	native   updateGUINativeLease
	image    *InstalledPairLease
	manifest string
	tun      bool
	resume   bool
	ready    bool
	prepared bool
}

func (session *UpdateGUISession) Mode() (tun, resume bool) { return session.tun, session.resume }

// The App already holds its own fresh installed-pair lease. Retire the extra
// bootstrap image before floor/archive mutation, not after discarding pending.
func (session *UpdateGUISession) PrepareAcceptance(ctx context.Context) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return errors.Join(ErrUpdateGUIBootstrap, err)
	}
	if session.ready {
		return ErrUpdateGUIBootstrap
	}
	if session.prepared {
		return errors.Join(session.native.validateJob(ctx), contextError(ctx))
	}
	if session.image == nil {
		return ErrUpdateGUIBootstrap
	}
	if err := session.native.validateJob(ctx); err != nil {
		return err
	}
	if err := session.image.Validate(ctx); err != nil {
		return err
	}
	if err := session.image.Close(); err != nil {
		return errors.Join(err, contextError(ctx)) // keep partial image and restriction on uncertainty
	}
	session.image, session.prepared = nil, true
	return contextError(ctx) // actual image-close receipt stays retained even on later cancellation
}

// Called inside actual activation after SDK/floor/finalization/native cleanup,
// before App restriction clears. This is not independently installation authority.
func (session *UpdateGUISession) AcceptStartup(ctx context.Context) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := contextError(ctx); err != nil {
		return errors.Join(ErrUpdateGUIBootstrap, err)
	}
	if !session.prepared || session.ready {
		return ErrUpdateGUIBootstrap
	}
	if err := session.native.validateJob(ctx); err != nil {
		return err
	}
	if err := contextError(ctx); err != nil {
		return errors.Join(ErrUpdateGUIBootstrap, err)
	}
	session.ready = true
	return nil
}

func (session *UpdateGUISession) ManifestSHA() string { return session.manifest }

var updateGUIBootstrapOwner *UpdateGUISession // terminal original native session, before adoption

// Every malformed reserved invocation is terminal, never a normal GUI fallback.
// A failed/partial session is returned for reachability until main calls os.Exit.
func DispatchUpdateGUI(args []string, prepare func(context.Context, string) (*InstalledPairLease, error), verify func(context.Context, string) error) (session *UpdateGUISession, handled bool, finalErr error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-update-gui") {
			handled = true
			break
		}
	}
	if !handled {
		return nil, false, nil
	}
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrUpdateGUIBootstrap)
		}
	}()
	if updateGUIBootstrapOwner != nil || !validUpdateGUIArguments(args) ||
		(args[0] != updateGUIArgument && args[0] != childbootstrap.ApplicationUpdateAncestorArgument) || prepare == nil || verify == nil {
		return nil, true, ErrUpdateGUIBootstrap
	}
	protocol, err := fixedApplicationGUIProtocol(args[0], args[1])
	if err != nil {
		return nil, true, err
	}
	if protocol.wire.NeedsAncestor() && !ancestorRelaunchAccepted || !workerPipesReady() {
		return nil, true, ErrUpdateGUIBootstrap
	}
	session = &UpdateGUISession{manifest: args[1], tun: args[2] == "tun", resume: args[3] == "resume"}
	updateGUIBootstrapOwner = session
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err = bootstrapApplicationGUI(ctx, protocol, &session.native, func(ctx context.Context) error {
		var err error
		session.image, err = prepare(ctx, session.manifest)
		if err != nil || session.image == nil {
			return errors.Join(ErrUpdateGUIBootstrap, err)
		}
		return session.native.authenticate(ctx, installedApplicationExecutable(session.image.image))
	}, session.imageValidate, func(ctx context.Context) error { return verify(ctx, session.manifest) })
	if err != nil {
		return session, true, err
	}
	completed = true
	return session, true, nil
}

func (session *UpdateGUISession) imageValidate(ctx context.Context) error {
	if session.image == nil {
		return ErrUpdateGUIBootstrap
	}
	if err := session.image.Validate(ctx); err != nil {
		return err
	}
	return session.native.authenticate(ctx, installedApplicationExecutable(session.image.image))
}
