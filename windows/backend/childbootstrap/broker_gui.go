package childbootstrap

import "context"

// The native callback must adopt all four original target handles before any
// prepared GUI service. ValidateTree must check BOTH Jobs and the fixed original
// helper/App identities and primary SID. AwaitOrigins waits for BOTH original
// processes, with no PID reopen and no success from timeout or lost interest.
type BrokerGUIOperations struct {
	Scope, Prepare, Validate, Verify func(context.Context) error
	Adopt                            func([4]uint64) error
	ValidateTree                     func() error
	AwaitOrigins                     func(context.Context) error
	Exchange                         func(context.Context, []byte, []byte) error
	CloseTransport                   func() error
}

// RunBrokerGUI reuses the existing child ARM/GO/OWN/USE/ACK/startup sequence with
// distinct stages, four-handle seed and native chain callbacks. It cannot run a
// broker-control or legacy U1 wire. Platform dispatch and native owners must be
// integrated separately; successful callbacks do not prove creation-time safety.
func RunBrokerGUI(ctx context.Context, protocol BrokerProtocol, operations BrokerGUIOperations) error {
	if protocol.purpose != BrokerSuccessor || operations.Adopt == nil {
		return ErrProtocol
	}
	callbacks := Operations{Scope: operations.Scope, Prepare: operations.Prepare,
		Validate: operations.Validate, Verify: operations.Verify,
		ValidateJob: operations.ValidateTree, AwaitParent: operations.AwaitOrigins,
		Exchange: operations.Exchange, CloseTransport: operations.CloseTransport}
	return runHandoff(ctx, protocol.Stages(), protocol.binding, BrokerSuccessorSeedSize, callbacks,
		protocol.ValidExchange, func(raw []byte) error {
			handles, err := protocol.DecodeSeed(raw)
			if err != nil {
				return err
			}
			if err := operations.Adopt(handles); err != nil {
				return err
			}
			if err := CheckScope(ctx, operations.Scope); err != nil {
				return err
			}
			return operations.ValidateTree() // before Prepare/ARM, then repeated before ACK/startup
		})
}
