package quic

import (
	"crypto/rand"
	"net"
	"slices"
	"time"

	"github.com/sagernet/quic-go/internal/ackhandler"
	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/utils"
	"github.com/sagernet/quic-go/internal/wire"
)

type pathID int64

const invalidPathID pathID = -1

// Maximum number of paths to keep track of.
// If the peer probes another path (before the pathTimeout of an existing path expires),
// this probing attempt is ignored.
const maxPaths = 3

// If no packet is received for a path for pathTimeout,
// the path can be evicted when the peer probes another path.
// This prevents an attacker from churning through paths by duplicating packets and
// sending them with spoofed source addresses.
const pathTimeout = 5 * time.Second

type path struct {
	id             pathID
	addr           net.Addr
	transport      *Transport
	info           packetInfo
	lastPacketTime monotime.Time

	validated         bool
	mtuValidated      bool
	rcvdNonProbing    bool
	largestNonProbing protocol.PacketNumber
	credit            protocol.ByteCount
	cidAssigned       bool

	pathChallenge                 [8]byte
	challengePending              bool
	challengeSize                 protocol.ByteCount
	challengeHistory              [maxPathChallenges]serverPathChallenge
	challengeCount, challengeNext int
	validation                    serverPathValidation
}

func (p *path) renewChallenge() {
	p.challengePending, p.challengeSize = true, 0
	rand.Read(p.pathChallenge[:]) // original record is already attached to its manager
}

type pathManager struct {
	nextPathID pathID
	// ordered by lastPacketTime, with the most recently used path at the end
	paths    []*path
	retiring *path // attached before expiry CID callbacks, cleared only on known return

	getConnID    func(pathID) (_ protocol.ConnectionID, ok bool)
	retireConnID func(pathID)

	logger utils.Logger
}

func newPathManager(
	getConnID func(pathID) (_ protocol.ConnectionID, ok bool),
	retireConnID func(pathID),
	logger utils.Logger,
) *pathManager {
	return &pathManager{
		paths:        make([]*path, 0, maxPaths+1),
		getConnID:    getConnID,
		retireConnID: retireConnID,
		logger:       logger,
	}
}

// Reuse or admit one original authenticated path before frame callbacks.
// The input owner contributes a datagram once; output reserves its own credit.
func (pm *pathManager) HandlePacket(
	input receivedPacket,
	receivedBytes protocol.ByteCount,
	probePTO time.Duration,
) *path {
	remoteAddr, t := input.remoteAddr, input.rcvTime
	now := monotime.Now()
	if input.transport == nil || remoteAddr == nil {
		return nil
	}
	var p *path
	for i, path := range pm.paths {
		if path.transport == input.transport && path.info == input.info && addrsEqual(path.addr, remoteAddr) {
			p = path
			p.lastPacketTime = t
			if pm.logger.Debug() {
				pm.logger.Debugf("received packet for path %s that was already probed, validated: %t", remoteAddr, path.validated)
			}
			if i != len(pm.paths)-1 {
				// move the path to the end of the list
				pm.paths = slices.Delete(pm.paths, i, i+1)
				pm.paths = append(pm.paths, p)
			}
			break // reusing this row does not consume another path slot
		}
	}

	if p == nil && len(pm.paths) >= maxPaths {
		if pm.paths[0].lastPacketTime.Add(pathTimeout).After(t) || pm.paths[0].validation.expires.After(now) {
			if pm.logger.Debug() {
				pm.logger.Debugf("received packet for previously unseen path %s, but already have %d paths", remoteAddr, len(pm.paths))
			}
			return nil
		}
		// evict the oldest path, if the last packet was received more than pathTimeout ago
		if pm.paths[0].cidAssigned {
			pm.retireConnID(pm.paths[0].id)
		}
		pm.paths[0] = nil
		pm.paths = pm.paths[1:]
	}

	if p == nil {
		if pm.nextPathID < 0 {
			return nil
		}
		p = &path{
			id: pm.nextPathID, addr: remoteAddr, lastPacketTime: t,
			transport: input.transport, info: input.info,
			largestNonProbing: protocol.InvalidPacketNumber,
		}
		pm.nextPathID++
		pm.paths = append(pm.paths, p)
		p.startValidation(now, probePTO) // attach before random, CID or logger callbacks
	}
	p.receiveCredit(receivedBytes)
	return p
}

func (p *path) observeMigrationPacket(isNonProbing bool, pn protocol.PacketNumber) bool {
	if isNonProbing {
		p.rcvdNonProbing = true
		p.largestNonProbing = max(p.largestNonProbing, pn)
	}
	return p.validated && p.mtuValidated && p.rcvdNonProbing
}

func (pm *pathManager) HandlePathResponseFrame(f *wire.PathResponseFrame, now monotime.Time, pto time.Duration) {
	for _, p := range pm.paths {
		p.validation.extendPTO(pto)
		if p.validation.expired(now) {
			continue // a late response cannot revive an abandoned validation phase
		}
		if size, ok := p.sentChallengeSize(f.Data); ok {
			wasValidated := p.validated
			p.validated = true
			if size >= protocol.MinInitialPacketSize {
				p.mtuValidated, p.challengePending = true, false
				p.validation = serverPathValidation{}
			} else if !wasValidated && !p.mtuValidated {
				if !p.challengePending && p.challengeSize >= protocol.MinInitialPacketSize {
					// Preserve an already outstanding full-size challenge and its
					// retry cadence, while allowing the MTU phase a fresh budget.
					p.validation.started = now
					p.validation.expires = max(p.validation.expires, serverPathAfter(now, scaledServerPathDelay(p.validation.pto, 7)))
				} else {
					p.startValidation(now, pto)
				}
			}
			pm.logger.Debugf("path %s address validated, MTU validated: %t", p.addr, p.mtuValidated)
			break
		}
	}
}

// SwitchToPath is called when the connection switches to a new path
func (pm *pathManager) SwitchToPath(selected *path) {
	// retire all other paths
	for _, path := range pm.paths {
		if path == selected {
			pm.logger.Debugf("switching to path %d (%s)", path.id, path.addr)
			continue
		}
		if path.cidAssigned {
			pm.retireConnID(path.id)
		}
	}
	clear(pm.paths)
	pm.paths = pm.paths[:0]
}

type pathManagerAckHandler pathManager

var _ ackhandler.FrameHandler = &pathManagerAckHandler{}

// Packet ACK/loss history does not own validation or retry timing. The original
// path clock survives both callbacks, including ACK without PATH_RESPONSE.
func (pm *pathManagerAckHandler) OnAcked(f wire.Frame) {}

func (pm *pathManagerAckHandler) OnLost(f wire.Frame) {}

func addrsEqual(addr1, addr2 net.Addr) bool {
	if addr1 == nil || addr2 == nil {
		return false
	}
	a1, ok1 := addr1.(*net.UDPAddr)
	a2, ok2 := addr2.(*net.UDPAddr)
	if ok1 && ok2 {
		return a1.IP.Equal(a2.IP) && a1.Port == a2.Port
	}
	return addr1.String() == addr2.String()
}
