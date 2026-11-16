package core

import (
	"context"
	"errors"
	"time"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

// This chain requires its own native acceptance. Acceptance of the legacy U1
// bridge must not silently authorize a different parent/helper/GUI protocol.
// Existing GUI6/R1 successors cannot carry the extra ancestor Job reference:
// that lifecycle must be implemented before this gate can ever be accepted.
const brokerGUIChainAccepted = false

var brokerGUIBootstrap *ElevatedGUISession

// DispatchBrokerGUI is terminal for either reserved namespace, including mixed
// or malformed flags. Only the successor belongs in the App; control belongs in
// the minimal helper. Existing native/bridge gates remain independently closed.
func DispatchBrokerGUI(args []string, helperPin string) (session *ElevatedGUISession, handled bool, finalErr error) {
	if !childbootstrap.BrokerInvocationRequested(args) {
		return nil, false, nil
	}
	handled = true
	completed := false
	defer func() {
		if recovered := recover(); recovered != nil || !completed {
			finalErr = errors.Join(finalErr, ErrElevatedGUIUnavailable)
		}
	}()
	protocol, resume, err := childbootstrap.ParseBrokerArguments(args)
	if err != nil || protocol.Purpose() != childbootstrap.BrokerSuccessor ||
		brokerGUIBootstrap != nil || elevatedGUIBootstrap != nil || !ElevatedGUIAvailable() || !brokerGUIChainAccepted {
		return nil, true, ErrElevatedGUIUnavailable
	}
	if _, err := nativeImagePin(helperPin); err != nil {
		return nil, true, err
	}
	session = &ElevatedGUISession{resume: resume, broker: new(brokerGUINative)}
	brokerGUIBootstrap = session // before pipe/image/token/native adoption, including Goexit
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := session.broker.run(ctx, protocol, helperPin); err != nil {
		return session, true, err
	}
	completed = true
	return session, true, nil
}
