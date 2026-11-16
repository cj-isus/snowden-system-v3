package quic

import (
	"errors"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

const maxServerPathCredit protocol.ByteCount = 1<<62 - 1

var errPathProbeCredit = errors.New("quic: insufficient path probe credit")

type serverPathChallenge struct {
	data [8]byte
	size protocol.ByteCount
}

func (p *path) recordSentChallenge(data [8]byte, size protocol.ByteCount, now monotime.Time) {
	p.challengeHistory[p.challengeNext] = serverPathChallenge{data: data, size: size}
	p.challengeNext = (p.challengeNext + 1) % len(p.challengeHistory)
	if p.challengeCount < len(p.challengeHistory) {
		p.challengeCount++
	}
	p.challengeSize, p.challengePending = size, false
	p.validation.recordSent(now)
}

func (p *path) sentChallengeSize(data [8]byte) (protocol.ByteCount, bool) {
	for _, sent := range p.challengeHistory[:p.challengeCount] {
		if sent.data == data && sent.size > 0 {
			return sent.size, true
		}
	}
	return 0, false
}

func (p *path) addCredit(amount protocol.ByteCount) {
	if amount <= 0 {
		return
	}
	if amount > maxServerPathCredit-p.credit {
		p.credit = maxServerPathCredit
	} else {
		p.credit += amount
	}
}

func (p *path) receiveCredit(bytes protocol.ByteCount) {
	if bytes <= 0 {
		return
	}
	if bytes > maxServerPathCredit/3 {
		p.addCredit(maxServerPathCredit)
	} else {
		p.addCredit(3 * bytes)
	}
}

// One original loop owns the reservation, route and output through callbacks.
// A debit is never moved to a newly discovered path or refunded after a write
// attempt. Known unused packing credit returns only to its original record.
type serverPathProbe struct {
	active, tried  bool
	path           *path
	route          queuedPathResponse
	frames         []ackhandler.Frame
	response       *wire.PathResponseFrame // nil for our own validation challenge
	buffer         *packetBuffer
	limit, charged protocol.ByteCount
	err            error
}

func (c *Conn) prepareServerPathInput(input receivedPacket) error {
	if c.perspective != protocol.PerspectiveServer || c.config.DisablePathManager || addrsEqual(input.remoteAddr, c.RemoteAddr()) {
		return nil
	}
	owner := c.packetInput.processing
	if owner == nil || input.transport == nil || input.remoteAddr == nil {
		return errPacketInputUncertain
	}
	if c.pathManager == nil {
		c.pathManager = newPathManager(c.connIDManager.GetConnIDForPath, c.connIDManager.RetireConnIDForPath, c.logger)
	}
	owner.serverPath = c.pathManager.HandlePacket(input, c.serverPathInputCredit(), c.serverPathPTO())
	return nil
}

func (c *Conn) serverPathInputCredit() protocol.ByteCount {
	owner := c.packetInput.processing
	if owner == nil || owner.pathCreditUsed {
		return 0
	}
	owner.pathCreditUsed = true
	return owner.packet.Size() // original datagram, not each coalesced segment
}

func (c *Conn) finishServerPathProbe(returned *bool) {
	call := &c.serverProbe
	if !*returned {
		c.packetOutput.failed = true
		if c.receiveFrames != nil {
			c.receiveFrames.markUncertain()
		}
		c.destroyImpl(errPacketOutputUncertain)
		return // retain original debit, route, frames, buffer and outcome
	}
	if !call.tried && call.charged > 0 {
		call.path.addCredit(call.charged)
	}
	*call = serverPathProbe{}
}

func (c *Conn) sendServerPathPacket(p *path, response *wire.PathResponseFrame, now monotime.Time) (bool, error) {
	if c.serverProbe.active || p == nil {
		return false, errPacketOutputUncertain
	}
	call := &c.serverProbe
	*call = serverPathProbe{active: true, path: p, response: response,
		route: queuedPathResponse{transport: p.transport, remote: p.addr, info: p.info}}
	returned := false
	defer c.finishServerPathProbe(&returned)
	if err := c.readinessError(); err != nil {
		call.err, returned = err, true
		return false, err
	}
	now = monotime.Now()
	p.validation.extendPTO(c.serverPathPTO())
	if p.validation.expired(now) {
		returned = true
		return false, nil // original loop alone retires this candidate, without output
	}
	if call.response == nil && p.validation.sent >= maxServerPathProbes {
		returned = true
		return false, nil // peer replies never borrow our challenge attempt budget
	}
	if p.transport == nil || p.addr == nil {
		returned = true
		return false, nil
	}
	call.limit = protocol.MinInitialPacketSize
	if !p.validated {
		call.limit = min(call.limit, p.credit)
		if call.limit <= 0 {
			returned = true
			return false, nil
		}
		call.charged = call.limit
		p.credit -= call.charged // before CID, packing, trace or output callbacks
	}
	connID, ok := c.pathManager.getConnID(p.id)
	if !ok {
		returned = true
		return false, nil // known unsent; keep the row and pending challenge
	}
	p.cidAssigned = true
	if c.closeErr.Load() != nil {
		call.err = c.readinessError()
		returned = true
		return false, call.err
	}
	if call.response == nil {
		call.frames = []ackhandler.Frame{{
			Frame:   &wire.PathChallengeFrame{Data: p.pathChallenge},
			Handler: (*pathManagerAckHandler)(c.pathManager),
		}}
	} else {
		call.frames = []ackhandler.Frame{{Frame: call.response}}
		if p.challengePending && p.validation.sent < maxServerPathProbes {
			// One queued response can carry the due validation challenge too.
			// Response pressure must not indefinitely starve its original path.
			call.frames = append(call.frames, ackhandler.Frame{
				Frame:   &wire.PathChallengeFrame{Data: p.pathChallenge},
				Handler: (*pathManagerAckHandler)(c.pathManager),
			})
		}
	}
	packet, buf, err := c.packer.PackPathProbePacketWithLimit(connID, call.frames, c.version, call.limit)
	call.buffer, call.err = buf, err
	if errors.Is(err, errPathProbeCredit) && call.response != nil && len(call.frames) == 2 {
		// Known no-output packing result. Prefer the owed response if adding
		// our challenge cannot fit; leave validation pending on its own clock.
		call.frames[1] = ackhandler.Frame{}
		call.frames = call.frames[:1]
		packet, buf, err = c.packer.PackPathProbePacketWithLimit(connID, call.frames, c.version, call.limit)
		call.buffer, call.err = buf, err
	}
	if err != nil {
		if errors.Is(err, errPathProbeCredit) {
			returned = true
			return false, nil // known unsent; preserve pending validation
		}
		returned = !errors.Is(err, errPacketOutputUncertain)
		return false, err
	}
	if buf == nil || buf.Len() <= 0 || protocol.ByteCount(buf.Len()) > call.limit {
		call.err = errPacketOutputUncertain
		return false, call.err
	}
	size := protocol.ByteCount(buf.Len())
	if call.charged > size {
		p.addCredit(call.charged - size)
		call.charged = size // known unused padding, before any provider attempt
	}
	if call.response != nil {
		packet.IsPathProbePacket = false // replies use ordinary shared-Conn flight accounting
	}
	call.tried = true
	call.err = c.sendOwnedPathPacket(packet, buf, p.transport, now, &call.route)
	call.buffer = nil // the original output owner now owns any uncertain buffer
	returned = !errors.Is(call.err, errPacketOutputUncertain) && !c.packetOutput.failed
	if !returned && call.err == nil {
		call.err = errPacketOutputUncertain
	}
	if returned && call.err == nil {
		for _, frame := range call.frames {
			if challenge, ok := frame.Frame.(*wire.PathChallengeFrame); ok && challenge.Data == p.pathChallenge {
				p.recordSentChallenge(challenge.Data, size, monotime.Now())
			}
		}
	}
	return returned && call.err == nil, call.err
}

func (c *Conn) sendPendingServerPathProbe(now monotime.Time) (bool, error) {
	if c.pathManager == nil {
		return false, nil
	}
	for _, p := range c.pathManager.paths {
		if p.challengePending && p.validation.sent < maxServerPathProbes && (p.validated || p.credit > 0) {
			if sent, err := c.sendServerPathPacket(p, nil, now); sent || err != nil {
				return sent, err
			}
		}
	}
	return false, nil
}
