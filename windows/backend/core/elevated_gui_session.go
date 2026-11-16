package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/controlpipe"
)

const elevatedGUIArgument = childbootstrap.ApplicationElevationArgument

func elevatedPipeName(nonce string) (string, error) {
	name, err := controlpipe.Name(nonce)
	if err != nil {
		return "", ErrNativeImageUnavailable
	}
	return name, nil
}

type ElevatedGUISession struct {
	native updateGUINativeLease
	pipe   uacControlPipe
	image  verifierApplicationLease
	resume bool
	broker *brokerGUINative // distinct four-handle chain; never an old U1 native lease
}

var elevatedGUIBootstrap *ElevatedGUISession // pure allocation before first native acquisition, including Goexit

func (session *ElevatedGUISession) Mode() (tun, resume bool) { return true, session.resume }
func (session *ElevatedGUISession) validate(ctx context.Context) error {
	if session.image == nil {
		return ErrElevatedGUIUnavailable
	}
	return session.image.Validate(ctx)
}

func DispatchElevatedGUI(args []string) (session *ElevatedGUISession, handled bool, finalErr error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--internal-elevated-gui") {
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
			finalErr = errors.Join(finalErr, ErrElevatedGUIUnavailable)
		}
	}()
	_, tun, resume, parseErr := childbootstrap.ParseApplicationArguments(args)
	if elevatedGUIBootstrap != nil || brokerGUIBootstrap != nil || !ElevatedGUIAvailable() || parseErr != nil || args[0] != elevatedGUIArgument || !tun {
		return nil, true, ErrElevatedGUIUnavailable
	}
	protocol, err := fixedApplicationGUIProtocol(elevatedGUIArgument, args[1])
	if err != nil {
		return nil, true, err
	}
	session = &ElevatedGUISession{resume: resume}
	elevatedGUIBootstrap = session
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := session.pipe.dial(ctx, args[1]); err != nil {
		return session, true, err
	}
	err = bootstrapApplicationGUITransport(ctx, protocol, &session.native, func(ctx context.Context) error {
		session.image = newVerifierApplication()
		if session.image == nil {
			return ErrNativeImageUnavailable
		}
		if err := session.image.prepare(ctx); err != nil {
			return err
		}
		return session.pipe.validateParent(ctx, &session.native, session.image)
	}, session.validate, session.validate, func(ctx context.Context, send, receive []byte) error {
		return session.pipe.exchange(ctx, send, receive, nil)
	}, func() error { return session.pipe.closeAfterServer(ctx) })
	if err != nil {
		return session, true, err
	}
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
