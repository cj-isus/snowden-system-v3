package quic

import (
	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/wire"
)

// Exactly one original allocation/credit. refs/history/producing use sendMx;
// the packer explicitly leases every read, including during queue Close.
type datagramSendEntry struct {
	queue     *datagramQueue
	frame     wire.DatagramFrame
	size      int
	refs      int
	history   bool
	producing bool
}

func (h *datagramQueue) releaseEntryLocked(entry *datagramSendEntry) {
	_, owned := h.sendLive[entry]
	if entry == nil || entry.queue != h || !owned || entry.refs <= 0 || entry.size < 0 ||
		h.sendOwned <= 0 || h.sendOwned != len(h.sendLive) || entry.size > h.sendBytes {
		h.poisonSendLocked()
		return
	}
	if entry.refs == 1 && (entry.history || entry.producing) {
		h.poisonSendLocked()
		return
	}
	entry.refs--
	if entry.refs != 0 {
		return
	}
	entry.frame.ReleaseOwnedData()
	delete(h.sendLive, entry)
	h.sendOwned--
	h.sendBytes -= entry.size
}

func (entry *datagramSendEntry) retireHistory(frame wire.Frame) {
	h := entry.queue
	h.sendMx.Lock()
	defer h.sendMx.Unlock()
	if frame != &entry.frame {
		h.poisonSendLocked()
		return
	}
	if !entry.history {
		return // ACK/loss/unsent converge on one exact history return
	}
	entry.history = false
	h.releaseEntryLocked(entry)
}

func (entry *datagramSendEntry) OnAcked(frame wire.Frame) { entry.retireHistory(frame) }
func (entry *datagramSendEntry) OnLost(frame wire.Frame)  { entry.retireHistory(frame) }

// A known local pack failure did not hand this original frame to sent history.
func retireUnsentDatagrams(frames []ackhandler.Frame) {
	for _, frame := range frames {
		if entry, ok := frame.Handler.(*datagramSendEntry); ok {
			entry.retireHistory(frame.Frame)
		}
	}
}
