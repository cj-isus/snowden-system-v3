package core

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

type restartGUIBinding struct {
	nonce            [32]byte
	attempted, ready bool // original owner.mu held, never a caller-supplied report
}

// Native allocation is pure; the owner is published BEFORE prepare acquires
// original image/ancestor handles. Partial panic/Goexit stays reachable.
type verifierApplicationLease interface {
	nativeImageLease
	prepare(context.Context) error
}

// Ordinary same-application restart, without elevation or update authority.
// Reuses the original process/Job/I/O/image and one-use custody machinery.
// It cannot invoke the SDK check worker or mint a signed installation verdict.
type RestartGUIOwner struct {
	owner  UpdateCodeAcceptance
	closed atomic.Bool
}

func NewRestartGUIOwner() *RestartGUIOwner {
	return &RestartGUIOwner{owner: UpdateCodeAcceptance{restart: &restartGUIBinding{}}}
}

func (restart *RestartGUIOwner) Prepare(ctx context.Context, guard func(context.Context) error) (finalErr error) {
	if restart == nil || guard == nil || restart.closed.Load() || !restart.owner.mu.TryLock() {
		return ErrUpdateGUIBootstrap
	}
	defer restart.owner.mu.Unlock()
	owner := &restart.owner
	if !owner.lane.TryLock() {
		return ErrHelperBusy
	}
	defer owner.lane.Unlock()
	guard = restart.guard(guard)
	if restart.closed.Load() || owner.restart == nil || owner.restart.attempted || owner.lane.image != nil || owner.lane.owner != nil {
		return ErrUpdateGUIBootstrap
	}
	owner.restart.attempted = true
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrUpdateGUIBootstrap)
		}
	}()
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	image := newVerifierApplication()
	owner.lane.image = image // before any native acquisition, including partial preparation
	if image == nil {
		return ErrNativeImageUnavailable
	}
	if err := image.prepare(ctx); err != nil {
		return err
	}
	if _, err := rand.Read(owner.restart.nonce[:]); err != nil {
		return err
	}
	if err := helperScope(ctx, guard); err != nil {
		return err
	}
	owner.restart.ready, owner.verified = true, true // on-disk custody only; not an SDK signature result
	completed = true
	return nil
}

func (restart *RestartGUIOwner) StartGUI(ctx context.Context, tun, resume bool, guard func(context.Context) error) error {
	if restart == nil || restart.closed.Load() || tun && !NativeTUNForwardingAvailable() {
		return ErrUpdateGUIBootstrap
	}
	return restart.owner.StartGUI(ctx, tun, resume, restart.guard(guard))
}

func (restart *RestartGUIOwner) CommitGUI(ctx context.Context, guard func(context.Context) error, accept func(UpdateGUICommit) error) (UpdateGUIReceipt, error) {
	if restart == nil || restart.closed.Load() {
		return UpdateGUIReceipt{}, ErrUpdateGUIBootstrap
	}
	return restart.owner.CommitGUI(ctx, restart.guard(guard), accept)
}

func (restart *RestartGUIOwner) guard(guard func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		if restart.closed.Load() || guard == nil {
			return ErrUpdateGUIBootstrap
		}
		if err := guard(ctx); err != nil {
			return err
		}
		if restart.closed.Load() {
			return ErrUpdateGUIBootstrap
		}
		return nil
	}
}

func (restart *RestartGUIOwner) GUITransferred() bool {
	return restart != nil && restart.owner.GUITransferred()
}

func (restart *RestartGUIOwner) Close() error {
	if restart == nil {
		return nil
	}
	restart.closed.Store(true) // all new positive work is burned, cleanup may retry the same owner
	return restart.owner.Close()
}

// Retained by main until process exit, including every partial bootstrap.
// Job is the same non-inheritable lifetime reference as GUI6. On-disk image
// custody does not attest loaded sections or authenticate an update manifest.
type RestartGUISession struct {
	native      updateGUINativeLease
	image       verifierApplicationLease
	tun, resume bool
}

var restartGUIBootstrapOwner *RestartGUISession // before first image/native acquisition, including nonreturn

func (session *RestartGUISession) Mode() (tun, resume bool) { return session.tun, session.resume }

func (session *RestartGUISession) validate(ctx context.Context) error {
	if session.image == nil {
		return ErrNativeImageUnavailable
	}
	if err := session.image.Validate(ctx); err != nil {
		return err
	}
	return session.native.authenticate(ctx, session.image.Executable())
}

// Reserved malformed/replayed/legacy restart invocations are terminal. No
// generic argv fallback, PID reopening, SDK admission or unauthenticated resume.
func DispatchRestartGUI(args []string) (session *RestartGUISession, handled bool, finalErr error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-restart-gui") {
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
	if restartGUIBootstrapOwner != nil || !validUpdateGUIArguments(args) ||
		(args[0] != restartGUIArgument && args[0] != childbootstrap.ApplicationRestartAncestorArgument) || args[2] == "tun" && !NativeTUNForwardingAvailable() {
		return nil, true, ErrUpdateGUIBootstrap
	}
	protocol, err := fixedApplicationGUIProtocol(args[0], args[1])
	if err != nil {
		return nil, true, err
	}
	if protocol.wire.NeedsAncestor() && !ancestorRelaunchAccepted || !workerPipesReady() {
		return nil, true, ErrUpdateGUIBootstrap
	}
	session = &RestartGUISession{tun: args[2] == "tun", resume: args[3] == "resume"}
	restartGUIBootstrapOwner = session
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err = bootstrapApplicationGUI(ctx, protocol, &session.native, func(ctx context.Context) error {
		session.image = newVerifierApplication() // original allocation before native preparation
		if session.image == nil {
			return ErrNativeImageUnavailable
		}
		if err := session.image.prepare(ctx); err != nil {
			return err
		}
		return session.native.authenticate(ctx, session.image.Executable())
	}, session.validate, session.validate)
	if err != nil {
		return session, true, err
	}
	// Normal startup still checks actual install/floor/profile authority. Close
	// this extra image before its housekeeping, while retaining the lifetime Job.
	if err := session.image.Close(); err != nil {
		return session, true, err
	}
	session.image = nil
	if err := contextError(ctx); err != nil {
		return session, true, err
	}
	completed = true
	return session, true, nil
}
