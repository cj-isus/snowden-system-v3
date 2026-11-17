package main

import "errors"

// Original frame/cleanup receipts only, never readiness or engine retirement.
// A functional fault stays separate from these actual return/cleanup facts.
type appProducerReceipt struct {
	bodyReturned        bool
	preparationOnly     bool // original setup failed before the actual run was scheduled
	preparationReturned bool
	cleanupReturned     bool
	cleanupResult       error
}

func (receipt appProducerReceipt) cleanupKnown() bool {
	return receipt.cleanupReturned && receipt.cleanupResult == nil && (!receipt.preparationOnly || receipt.preparationReturned)
}

func (receipt appProducerReceipt) terminalResult(unavailable error) error {
	if !receipt.bodyReturned && !(receipt.preparationOnly && receipt.preparationReturned) || !receipt.cleanupKnown() {
		return errors.Join(unavailable, receipt.cleanupResult)
	}
	return nil
}
