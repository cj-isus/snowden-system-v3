//go:build windows

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
	"github.com/snowden-system/windows/backend/childbootstrap/imagecheck"
	"github.com/snowden-system/windows/backend/childbootstrap/imagelease"
	"github.com/snowden-system/windows/backend/childbootstrap/spawnprep"
	"github.com/snowden-system/windows/backend/netprobe"
	"golang.org/x/sys/windows"
)

// Every original partial owner is reachable before any native call. This helper
// owns one coordinator and one fixed successor; it is not a general spawn service.
type brokerHelperSession struct {
	lease                    spawnprep.BrokerLease
	pair                     imagelease.Pair
	pipe                     *controlpipe.BrokerPipe
	coordinator              childbootstrap.BrokerCoordinator
	child                    brokerSuccessor
	application, helper      string
	ready, attempted, appEOF bool
}

func (session *brokerHelperSession) run(ctx context.Context, protocol childbootstrap.BrokerProtocol, resume bool) error {
	if session.attempted || !protocol.PinnedControl() {
		return errBrokerHelper
	}
	session.attempted = true
	exe, err := os.Executable()
	if err != nil || !filepath.IsAbs(exe) || filepath.Clean(exe) != exe || len(exe) > 2048 ||
		!strings.EqualFold(filepath.Base(exe), netprobe.NativeHelperName) {
		return errBrokerHelper
	}
	session.helper, session.application = exe, filepath.Join(filepath.Dir(exe), childbootstrap.ApplicationFile)
	session.pipe, err = controlpipe.NewBrokerPipe(protocol)
	if err != nil {
		return err
	}
	if err := session.pipe.Dial(ctx, brokerScope); err != nil {
		return err
	}
	err = session.pipe.RunCoordinator(ctx, &session.coordinator, childbootstrap.BrokerOperations{
		Scope: brokerScope,
		Adopt: func(handles [4]uint64) error {
			if err := session.lease.Adopt(ctx, childbootstrap.BrokerControl, handles, brokerScope); err != nil {
				return err
			}
			// Authenticate held original App and its pipe peer BEFORE using the pin.
			if err := session.authenticateOrigin(ctx); err != nil {
				return err
			}
			raw, err := hex.DecodeString(protocol.HelperPin())
			if err != nil || len(raw) != 32 {
				return errBrokerHelper
			}
			var pin [32]byte
			copy(pin[:], raw)
			if err := session.pair.Prepare(ctx, session.application, session.helper, pin, brokerScope); err != nil {
				return err
			}
			session.ready = true
			return session.validateOrigin(ctx)
		},
		ValidateChain:    session.validateChain,
		PrepareSuccessor: func(ctx context.Context) error { return session.child.prepare(ctx, session, resume) },
		CommitSuccessor:  func(ctx context.Context) error { return session.child.commit(ctx, session) },
	})
	// Coordinator preserves physical Close return even if a later scope fails.
	session.appEOF = session.coordinator.Progress().Close.Succeeded
	if err != nil {
		return err
	}
	if !session.appEOF || !session.child.ackObserved || !session.child.serverClosed {
		return errBrokerHelper
	}
	return session.validateChain(ctx) // original App still alive; no peer query after expected EOF
}

func (session *brokerHelperSession) authenticateOrigin(ctx context.Context) error {
	return session.lease.WithChain(ctx, brokerScope, func(ancestor, role, parent, origin windows.Handle) error {
		if parent != origin {
			return errBrokerHelper
		}
		if err := imagecheck.ProcessPathMatches(ctx, windows.CurrentProcess(), session.helper, brokerScope); err != nil {
			return err
		}
		if err := imagecheck.ProcessPathMatches(ctx, parent, session.application, brokerScope); err != nil {
			return err
		}
		id, err := windows.GetProcessId(parent)
		if err != nil || id == 0 {
			return errors.Join(errBrokerHelper, err)
		}
		if session.appEOF {
			return brokerScope(ctx)
		}
		return session.pipe.CheckServer(ctx, id, brokerScope)
	})
}

func (session *brokerHelperSession) validateOrigin(ctx context.Context) error {
	if !session.ready {
		return errBrokerHelper
	}
	if err := session.pair.Validate(ctx, brokerScope); err != nil {
		return err
	}
	return session.authenticateOrigin(ctx)
}

func (session *brokerHelperSession) validateChain(ctx context.Context) error {
	if err := session.validateOrigin(ctx); err != nil {
		return err
	}
	if session.child.ready {
		return session.child.validate(ctx, session)
	}
	return brokerScope(ctx)
}

func (session *brokerHelperSession) retire() error {
	// Local child resource retirement is not ancestor-tree cleanup authority. The
	// App retains the original ancestor control handle and handles terminal error.
	childErr := session.child.retire()
	pipeErr := session.pipe.Close()
	if err := errors.Join(childErr, pipeErr); err != nil {
		return err
	}
	return session.pair.Close()
}
