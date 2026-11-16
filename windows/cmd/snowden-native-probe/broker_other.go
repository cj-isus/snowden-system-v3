//go:build !windows

package main

import (
	"context"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

type brokerHelperSession struct{}

func (*brokerHelperSession) run(context.Context, childbootstrap.BrokerProtocol, bool) error {
	return errBrokerHelper
}
func (*brokerHelperSession) retire() error { return nil }
