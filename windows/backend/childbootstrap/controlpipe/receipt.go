package controlpipe

import (
	"bytes"

	"github.com/snowden-system/windows/backend/childbootstrap"
)

// ExchangeReceipt retains one original bounded native exchange. Publish it
// before ExchangeObserved and serialize all access; never reuse after attempt.
// These are effect observations, never source/peer/Job or transfer authority.
// A matched reply can survive a later context/peer failure without making that
// failed exchange successful. No native handle can be obtained from this data.
type ExchangeReceipt struct {
	send, reply                                 [childbootstrap.BrokerSuccessorSeedSize]byte
	sendSize, replySize, written, read          int
	attempted, fault                            bool
	writeAttempted, writePending, writeReturned bool
	readPending, readReturned                   bool
}

func (receipt *ExchangeReceipt) begin(send []byte, replySize int) error {
	if receipt == nil || receipt.attempted || len(send) > len(receipt.send) || replySize < 0 || replySize > len(receipt.reply) {
		return ErrPipe
	}
	receipt.attempted = true
	receipt.sendSize, receipt.replySize = len(send), replySize
	copy(receipt.send[:], send)
	return nil
}

// MayHaveSent is deliberately negative: a native write that did not return is
// not proof of zero effect. This may require retirement; it cannot mint an ACK.
func (receipt *ExchangeReceipt) MayHaveSent() bool {
	return receipt != nil && receipt.attempted && receipt.sendSize != 0 && receipt.writeAttempted
}

// ReplyMatches checks original bytes and actual returned counts only. Callers
// still need every ordinary native/peer/source/grant check for positive transfer.
func (receipt *ExchangeReceipt) ReplyMatches(send, reply []byte) bool {
	return receipt != nil && receipt.attempted && !receipt.fault && !receipt.writePending && !receipt.readPending &&
		(receipt.sendSize == 0 || receipt.writeReturned) && receipt.readReturned && receipt.written == receipt.sendSize && receipt.read == receipt.replySize &&
		len(send) == receipt.sendSize && len(reply) == receipt.replySize && len(reply) != 0 &&
		bytes.Equal(receipt.send[:receipt.sendSize], send) && bytes.Equal(receipt.reply[:receipt.replySize], reply)
}
