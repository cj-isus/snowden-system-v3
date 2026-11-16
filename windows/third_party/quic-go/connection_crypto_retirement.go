package quic

import (
	"errors"
	"sync"
)

var errCryptoQueueRetirement = errors.New("quic: original crypto queue retirement is uncertain")

type cryptoQueueRetirementStage uint8

const (
	cryptoQueueJoining cryptoQueueRetirementStage = iota + 1
	cryptoQueuePreflight
	cryptoQueueEvents
	cryptoQueueKeys
	cryptoQueueClearing
)

// access guards admission/result only. One finalizer owns this fixed prefix;
// negative/abnormal outcomes retain it rather than start another attempt.
type cryptoQueueRetirement struct {
	access         sync.Mutex
	done           chan struct{}
	started, ended bool
	result         error
	stage          cryptoQueueRetirementStage
	index          int
	handler        cryptoStreamHandler
	manager        *cryptoStreamManager
	streams        [3]*baseCryptoStream
	retransmission *retransmissionQueue
	framer         *framer
	packer         *packetPacker
}

func cryptoControlMetadataReady(f *framer) bool {
	if f == nil {
		return false
	}
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.sealed && f.metadataRetired && f.pathResponsesSealed &&
		f.pathResponses == nil && f.controlFrames == nil && f.controlMetadataReadyLocked()
}

func (c *Conn) captureCryptoQueueRetirement(r *cryptoQueueRetirement) bool {
	if c.initialStream == nil || c.handshakeStream == nil || c.oneRTTStream == nil ||
		r.manager == nil || r.retransmission == nil || !r.retransmission.metadataReady() ||
		r.manager.initialStream != c.initialStream || r.manager.handshakeStream != c.handshakeStream ||
		r.manager.oneRTTStream != c.oneRTTStream {
		return false
	}
	r.streams = [3]*baseCryptoStream{&c.initialStream.baseCryptoStream,
		&c.handshakeStream.baseCryptoStream, &c.oneRTTStream.baseCryptoStream}
	p, ok := c.packer.(*packetPacker)
	r.packer = p
	if !ok || p == nil || !c.packetPackerQuiescent() || p.initialStream != c.initialStream ||
		p.handshakeStream != c.handshakeStream || p.retransmissionQueue != r.retransmission ||
		p.framer != r.framer || !cryptoControlMetadataReady(r.framer) || !c.clientTokenMetadataReady() ||
		r.retransmission.initial != nil && r.retransmission.initial == r.retransmission.handshake {
		return false
	}
	for i, stream := range r.streams {
		r.index = i
		for j := 0; j < i; j++ {
			if stream == r.streams[j] {
				return false
			}
		}
		if !stream.cryptoReceiveMetadataReady() || !stream.cryptoWriteMetadataReady() {
			return false
		}
	}
	return true
}

// WaitForCryptoQueueRetirement joins the native/public-call/STREAM/DATAGRAM/
// sent/received-history aggregate and original TLS Close receipt before dropping
// LOCAL crypto event/key/stream/retransmission references. Never self-join a
// callback. Original TLS diagnostics, provider aliases and physical native/OS/
// GC/RSS retirement remain separate. Bytes and key backing aren't wiped.
func (c *Conn) WaitForCryptoQueueRetirement() (result error) {
	if c == nil || c.ctx == nil {
		return errCryptoQueueRetirement
	}
	r := &c.cryptoQueues
	r.access.Lock()
	if r.done == nil {
		r.access.Unlock()
		return errCryptoQueueRetirement
	}
	if r.started {
		done := r.done
		r.access.Unlock()
		<-done
		r.access.Lock()
		result = r.result
		if !r.ended {
			result = errors.Join(errCryptoQueueRetirement, result)
		}
		r.access.Unlock()
		return result
	}
	r.started, r.stage = true, cryptoQueueJoining
	r.handler, r.manager = c.cryptoStreamHandler, c.cryptoStreamManager
	r.retransmission, r.framer = c.retransmissionQueue, c.framer
	r.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(errCryptoQueueRetirement, result)
		}
		r.access.Lock()
		defer r.access.Unlock()
		r.result, r.ended = result, true
		if returned && result == nil {
			r.handler, r.manager, r.retransmission, r.framer, r.packer = nil, nil, nil, nil, nil
			r.streams = [3]*baseCryptoStream{}
		}
		close(r.done)
	}()
	if err := c.WaitForPacketHistoryRetirement(); err != nil {
		returned = true
		return errors.Join(errCryptoQueueRetirement, err)
	}
	if r.handler == nil {
		returned = true
		return errCryptoQueueRetirement
	}
	if err := r.handler.WaitForShutdown(); err != nil {
		returned = true
		return errors.Join(errCryptoQueueRetirement, err)
	}
	r.stage = cryptoQueuePreflight
	if !c.captureCryptoQueueRetirement(r) {
		returned = true
		return errCryptoQueueRetirement
	}
	r.stage = cryptoQueueEvents
	if err := r.handler.RetireEventQueue(); err != nil {
		returned = true
		return errors.Join(errCryptoQueueRetirement, err)
	}
	r.stage = cryptoQueueKeys
	if err := r.handler.RetireKeyReferences(); err != nil {
		returned = true
		return errors.Join(errCryptoQueueRetirement, err)
	}
	// All original identities/sorters passed before any local queue head changes.
	r.stage = cryptoQueueClearing
	r.retransmission.retireStorageReferences()
	for i, stream := range r.streams {
		r.index = i
		stream.retireCryptoReceiveStorage()
		stream.retireCryptoWriteStorage()
		stream.finished = true
	}
	c.retireClientTokenReferences(r.packer)
	returned = true
	return nil
}
