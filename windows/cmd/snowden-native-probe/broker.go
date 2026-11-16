package main

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

var errBrokerHelper = errors.New("native bootstrap broker unavailable")

// Acceptance belongs to this minimal executable as well as the App's independent
// parent/chain/native gates. Source composition alone cannot enable either one.
const brokerHelperAccepted = false

var activeBrokerHelper *brokerHelperSession

func brokerScope(ctx context.Context) error {
	if ctx == nil {
		return errBrokerHelper
	}
	return ctx.Err()
}

func runBrokerInvocation(args []string) (code int) {
	code = 2
	protocol, resume, err := childbootstrap.ParseBrokerArguments(args)
	if err != nil || !protocol.PinnedControl() || activeBrokerHelper != nil || !brokerHelperAccepted {
		return code
	}
	original := new(brokerHelperSession)
	activeBrokerHelper = original // before Dial/adoption/image/child/native acquisition
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			code = 2
			_ = original.retire() // terminal OS exit follows; original uncertainty stays reachable
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := original.run(ctx, protocol, resume); err != nil {
		return code
	}
	completed = true
	return 0 // actual helper exit, never a GUI/VPN readiness receipt
}
