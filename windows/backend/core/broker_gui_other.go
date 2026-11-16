//go:build !windows

package core

import (
	"context"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

type brokerGUINative struct{}

func (*brokerGUINative) run(context.Context, childbootstrap.BrokerProtocol, string) error {
	return ErrElevatedGUIUnavailable
}
