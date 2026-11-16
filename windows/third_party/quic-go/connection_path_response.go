package quic

import (
	"errors"
	"net"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/wire"
)

// The existing framer queue owns the route until the original loop takes it.
// A nil transport is an explicit legacy QueueControlFrame entry, never a routed
// response's fallback. Real challenge handlers always supply their input owner.
type queuedPathResponse struct {
	frame         *wire.PathResponseFrame
	transport     *Transport
	remote        net.Addr
	info          packetInfo
	hasServerPath bool
	serverPathID  pathID // immutable identity, not a borrowed path-record pointer
}

// The original Conn loop retains this prefix before CID, packer, trace or write
// callbacks. Frames use an independent allocation: sent history may outlive it.
type pathResponseWrite struct {
	active   bool
	response queuedPathResponse
	frames   *[1]ackhandler.Frame
	err      error
}

func (f *framer) QueuePathResponse(data [8]byte, input receivedPacket, serverPath *path) bool {
	if input.transport == nil || input.remoteAddr == nil {
		return false // never create an implicit legacy entry from missing origin
	}
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	if f.pathResponsesSealed || len(f.pathResponses) >= maxPathResponses {
		return false // retain the existing 256-response pressure policy
	}
	f.pathResponses = append(f.pathResponses, queuedPathResponse{
		transport: input.transport, remote: input.remoteAddr, info: input.info,
	})
	entry := &f.pathResponses[len(f.pathResponses)-1]
	if serverPath != nil {
		entry.hasServerPath, entry.serverPathID = true, serverPath.id
	}
	entry.frame = &wire.PathResponseFrame{Data: data} // after queue admission
	return true
}

func (f *framer) takePathResponse(call *pathResponseWrite) bool {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	if len(f.pathResponses) == 0 || f.pathResponses[0].transport == nil {
		return false // legacy entries keep their explicit ordinary-packet path
	}
	call.active, call.response = true, f.pathResponses[0]
	f.pathResponses[0] = queuedPathResponse{}
	f.pathResponses = f.pathResponses[1:]
	return true
}

func (f *framer) pathResponseCount() int {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	return len(f.pathResponses)
}

func (f *framer) deferPathResponse(response queuedPathResponse) bool {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	if f.pathResponsesSealed || len(f.pathResponses) >= maxPathResponses {
		return false // the same existing pressure policy; no overflow side queue
	}
	f.pathResponses = append(f.pathResponses, response)
	return true
}

func (f *framer) retirePathResponses() {
	f.controlFrameMutex.Lock()
	defer f.controlFrameMutex.Unlock()
	f.pathResponsesSealed = true // covers routed, deferred and explicit legacy entries
	clear(f.pathResponses)
	f.pathResponses = nil
}

// Called only by the original loop's final guard, after its actual drain,
// send-queue/close callbacks and packet-output/packer return receipts. No CID,
// provider, trace or native callback is invoked while releasing this metadata.
func (c *Conn) retirePathMetadata() bool {
	if c.pathMetadataRetiring || c.pathMetadataRetired || c.framer == nil ||
		c.closeErr.Load() == nil || c.pathSwitch.active || c.pathResponse.active || c.serverProbe.active ||
		!c.packetPackerQuiescent() || c.packetOutput.original != nil || c.packetOutput.failed ||
		!c.packetInput.pathMetadataReady() || c.pathManager != nil && c.pathManager.retiring != nil {
		return false
	}
	c.pathMetadataRetiring = true // before waiting on metadata locks; never retry unknown cleanup
	c.framer.retirePathResponses()
	if pm := c.pathManager; pm != nil {
		// Sent frame handlers may retain the manager. Its ACK/loss callbacks are
		// no-ops; release its rows and no-longer-callable closures as well.
		clear(pm.paths)
		pm.paths = nil
		pm.getConnID, pm.retireConnID, pm.logger = nil, nil, nil
		c.pathManager = nil
	}
	c.activeServerPathID = invalidPathID
	c.pathMetadataRetired, c.pathMetadataRetiring = true, false
	return true
}

func (c *Conn) promotedServerResponse(response *queuedPathResponse) bool {
	return c.perspective == protocol.PerspectiveServer && response.hasServerPath &&
		response.serverPathID == c.activeServerPathID && response.transport == c.initialTransport &&
		addrsEqual(response.remote, c.conn.RemoteAddr())
}

func (c *Conn) serverResponsePath(response *queuedPathResponse) *path {
	if c.pathManager == nil {
		return nil
	}
	for _, p := range c.pathManager.paths {
		if p.id == response.serverPathID && p.transport == response.transport && p.info == response.info && addrsEqual(p.addr, response.remote) {
			return p
		}
	}
	return nil // never bind a stale reply to a replacement row at the same address
}

func (c *Conn) pathResponseConnID(response *queuedPathResponse) (protocol.ConnectionID, bool) {
	if response.hasServerPath {
		if !c.promotedServerResponse(response) {
			return protocol.ConnectionID{}, false
		}
		return c.connIDManager.GetConnIDForPath(response.serverPathID)
	}
	if !addrsEqual(response.remote, c.conn.RemoteAddr()) {
		return protocol.ConnectionID{}, false // no fallback for a stale peer route
	}
	if response.transport == c.initialTransport {
		return c.connIDManager.Get(), true
	}
	if c.perspective == protocol.PerspectiveClient {
		if pm := c.pathManagerOutgoing.Load(); pm != nil {
			return pm.responseConnID(response.transport)
		}
	}
	return protocol.ConnectionID{}, false
}

func (c *Conn) finishPathResponse(returned *bool) {
	if !*returned {
		c.packetOutput.failed = true
		if c.receiveFrames != nil {
			c.receiveFrames.markUncertain()
		}
		c.destroyImpl(errPacketOutputUncertain)
		return // retain the exact original response / partial output prefix
	}
	c.pathResponse = pathResponseWrite{}
}

// Scan at most the original queue length, through ordinary SendModeAny/pacing.
// Deferred candidates rotate in the same queue; no immediate retry is scheduled.
func (c *Conn) sendPendingPathResponse(now monotime.Time) (bool, error) {
	if c.pathResponse.active {
		return false, errPacketOutputUncertain
	}
	budget := c.framer.pathResponseCount()
	for i := 0; i < budget; i++ {
		sent, taken, err := c.sendOnePathResponse(now)
		if sent || err != nil {
			return sent, err
		}
		if !taken {
			break // explicit legacy entry keeps its ordinary-packet path
		}
	}
	return false, nil
}

func (c *Conn) sendOnePathResponse(now monotime.Time) (sent, taken bool, _ error) {
	call := &c.pathResponse
	if call.active {
		return false, false, errPacketOutputUncertain
	}
	if !c.framer.takePathResponse(call) {
		return false, false, nil
	}
	returned := false
	defer c.finishPathResponse(&returned)
	if err := c.readinessError(); err != nil {
		call.err = err
		returned = true
		return false, true, err
	}
	if call.response.hasServerPath && !c.promotedServerResponse(&call.response) {
		p := c.serverResponsePath(&call.response)
		if p == nil {
			returned = true // exact original path expired/retired; no other-path fallback
			return false, true, nil
		}
		sent, call.err = c.sendServerPathPacket(p, call.response.frame, now)
		returned = !errors.Is(call.err, errPacketOutputUncertain) && !c.packetOutput.failed
		if !returned && call.err == nil {
			call.err = errPacketOutputUncertain
		}
		if returned && !sent && call.err == nil {
			c.framer.deferPathResponse(call.response) // known no-send: CID, credit or fit
		}
		return sent, true, call.err
	}
	connID, ok := c.pathResponseConnID(&call.response)
	if !ok {
		if c.promotedServerResponse(&call.response) {
			c.framer.deferPathResponse(call.response) // same promoted path temporarily lacks a CID
		}
		returned = true // known unused route; no socket borrowed this response
		return false, true, nil
	}
	if c.closeErr.Load() != nil {
		call.err = net.ErrClosed
		returned = true // CID callback requested close before any pack/native work
		return false, true, call.err
	}
	call.frames = &[1]ackhandler.Frame{{Frame: call.response.frame}}
	packet, buf, err := c.packer.PackPathProbePacket(connID, call.frames[:], c.version)
	call.err = err
	if err != nil {
		returned = !errors.Is(err, errPacketOutputUncertain)
		return false, true, err
	}
	// Padding comes from the path packer. A response consumes the normal shared
	// Conn flight budget, rather than the special unaccounted challenge category.
	packet.IsPathProbePacket = false
	call.err = c.sendOwnedPathPacket(packet, buf, call.response.transport, now, &call.response)
	returned = !errors.Is(call.err, errPacketOutputUncertain) && !c.packetOutput.failed
	if !returned && call.err == nil {
		call.err = errPacketOutputUncertain
	}
	return returned && call.err == nil, true, call.err
}
