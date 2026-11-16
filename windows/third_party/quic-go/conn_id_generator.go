package quic

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/sagernet/quic-go/internal/monotime"
	"github.com/sagernet/quic-go/internal/protocol"
	"github.com/sagernet/quic-go/internal/qerr"
	"github.com/sagernet/quic-go/internal/wire"
)

var errCIDRegistration = errors.New("connection ID or reset token registration refused")

type connRunnerCallbacks struct {
	AddConnectionID    func(protocol.ConnectionID) bool
	RemoveConnectionID func(protocol.ConnectionID)
	ReplaceWithClosed  func([]protocol.ConnectionID, []byte, time.Duration)
}

// The memory address of the Transport is used as the key.
type connRunners map[connRunner]connRunnerCallbacks

func (cr connRunners) AddConnectionID(id protocol.ConnectionID) bool {
	for _, c := range cr {
		if !c.AddConnectionID(id) {
			return false
		}
	}
	return true
}

func (cr connRunners) RemoveConnectionID(id protocol.ConnectionID) {
	for _, c := range cr {
		c.RemoveConnectionID(id)
	}
}

func (cr connRunners) AddResetToken(token protocol.StatelessResetToken, original packetHandler) bool {
	for runner := range cr {
		if !runner.AddResetToken(token, original) {
			return false
		}
	}
	return true
}

func (cr connRunners) RemoveResetToken(original packetHandler, token protocol.StatelessResetToken) {
	for runner := range cr {
		runner.RemoveResetToken(original, token)
	}
}

func (cr connRunners) ReplaceWithClosed(ids []protocol.ConnectionID, b []byte, expiry time.Duration) {
	for _, c := range cr {
		c.ReplaceWithClosed(ids, b, expiry)
	}
}

type connIDToRetire struct {
	t      monotime.Time
	connID protocol.ConnectionID
}

type connIDGenerator struct {
	generator   ConnectionIDGenerator
	highestSeq  uint64
	connRunners connRunners
	connIDLen   int

	registrationFailure error

	activeSrcConnIDs        map[uint64]protocol.ConnectionID
	connIDsToRetire         []connIDToRetire       // sorted by t
	initialClientDestConnID *protocol.ConnectionID // nil for the client

	statelessResetter *statelessResetter

	queueControlFrame func(wire.Frame)
}

func newConnIDGenerator(
	runner connRunner,
	initialConnectionID protocol.ConnectionID,
	initialClientDestConnID *protocol.ConnectionID, // nil for the client
	statelessResetter *statelessResetter,
	callbacks connRunnerCallbacks,
	queueControlFrame func(wire.Frame),
	generator ConnectionIDGenerator,
) *connIDGenerator {
	m := &connIDGenerator{
		generator:         generator,
		connIDLen:         initialConnectionID.Len(),
		activeSrcConnIDs:  make(map[uint64]protocol.ConnectionID),
		statelessResetter: statelessResetter,
		connRunners:       map[connRunner]connRunnerCallbacks{runner: callbacks},
		queueControlFrame: queueControlFrame,
	}
	m.activeSrcConnIDs[0] = initialConnectionID
	m.initialClientDestConnID = initialClientDestConnID
	return m
}

func (m *connIDGenerator) SetMaxActiveConnIDs(limit uint64) error {
	if m.registrationFailure != nil {
		return m.registrationFailure
	}
	if m.generator.ConnectionIDLen() == 0 {
		return nil
	}
	// The active_connection_id_limit transport parameter is the number of
	// connection IDs the peer will store. This limit includes the connection ID
	// used during the handshake, and the one sent in the preferred_address
	// transport parameter.
	// We currently don't send the preferred_address transport parameter,
	// so we can issue (limit - 1) connection IDs.
	for i := uint64(len(m.activeSrcConnIDs)); i < min(limit, protocol.MaxIssuedConnectionIDs); i++ {
		if err := m.issueNewConnID(); err != nil {
			return err
		}
	}
	return nil
}

func (m *connIDGenerator) Retire(seq uint64, sentWithDestConnID protocol.ConnectionID, expiry monotime.Time) error {
	if m.registrationFailure != nil {
		return m.registrationFailure
	}
	if seq > m.highestSeq {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: fmt.Sprintf("retired connection ID %d (highest issued: %d)", seq, m.highestSeq),
		}
	}
	connID, ok := m.activeSrcConnIDs[seq]
	// We might already have deleted this connection ID, if this is a duplicate frame.
	if !ok {
		return nil
	}
	if connID == sentWithDestConnID {
		return &qerr.TransportError{
			ErrorCode:    qerr.ProtocolViolation,
			ErrorMessage: fmt.Sprintf("retired connection ID %d (%s), which was used as the Destination Connection ID on this packet", seq, connID),
		}
	}
	m.queueConnIDForRetiring(connID, expiry)

	delete(m.activeSrcConnIDs, seq)
	// Don't issue a replacement for the initial connection ID.
	if seq == 0 {
		return nil
	}
	return m.issueNewConnID()
}

func (m *connIDGenerator) queueConnIDForRetiring(connID protocol.ConnectionID, expiry monotime.Time) {
	idx := slices.IndexFunc(m.connIDsToRetire, func(c connIDToRetire) bool {
		return c.t.After(expiry)
	})
	if idx == -1 {
		idx = len(m.connIDsToRetire)
	}
	m.connIDsToRetire = slices.Insert(m.connIDsToRetire, idx, connIDToRetire{t: expiry, connID: connID})
}

func (m *connIDGenerator) issueNewConnID() error {
	if m.registrationFailure != nil {
		return m.registrationFailure
	}
	connID, err := m.generator.GenerateConnectionID()
	if err != nil {
		return err
	}
	if connID.Len() == 0 || connID.Len() != m.connIDLen || m.containsConnID(connID) {
		m.registrationFailure = errCIDRegistration
		return m.registrationFailure
	}
	// Keep the original pending ID attached before callbacks. If a later runner
	// refuses it, connection teardown must still remove earlier registrations.
	m.activeSrcConnIDs[m.highestSeq+1] = connID
	if !m.connRunners.AddConnectionID(connID) {
		m.registrationFailure = errCIDRegistration
		return m.registrationFailure
	}
	m.queueControlFrame(&wire.NewConnectionIDFrame{
		SequenceNumber:      m.highestSeq + 1,
		ConnectionID:        connID,
		StatelessResetToken: m.statelessResetter.GetStatelessResetToken(connID),
	})
	m.highestSeq++
	return nil
}

func (m *connIDGenerator) containsConnID(id protocol.ConnectionID) bool {
	if m.initialClientDestConnID != nil && *m.initialClientDestConnID == id {
		return true
	}
	for _, active := range m.activeSrcConnIDs {
		if active == id {
			return true
		}
	}
	for _, retiring := range m.connIDsToRetire {
		if retiring.connID == id {
			return true
		}
	}
	return false
}

func (m *connIDGenerator) SetHandshakeComplete(connIDExpiry monotime.Time) {
	if m.initialClientDestConnID != nil {
		m.queueConnIDForRetiring(*m.initialClientDestConnID, connIDExpiry)
		m.initialClientDestConnID = nil
	}
}

func (m *connIDGenerator) RemoveRetiredConnIDs(now monotime.Time) {
	if len(m.connIDsToRetire) == 0 {
		return
	}
	for _, c := range m.connIDsToRetire {
		if c.t.After(now) {
			break
		}
		m.connRunners.RemoveConnectionID(c.connID)
		m.connIDsToRetire = m.connIDsToRetire[1:]
	}
}

func (m *connIDGenerator) RemoveAll() {
	if m.initialClientDestConnID != nil {
		m.connRunners.RemoveConnectionID(*m.initialClientDestConnID)
	}
	for _, connID := range m.activeSrcConnIDs {
		m.connRunners.RemoveConnectionID(connID)
	}
	for _, c := range m.connIDsToRetire {
		m.connRunners.RemoveConnectionID(c.connID)
	}
}

func (m *connIDGenerator) ReplaceWithClosed(connClose []byte, expiry time.Duration) {
	connIDs := make([]protocol.ConnectionID, 0, len(m.activeSrcConnIDs)+len(m.connIDsToRetire)+1)
	if m.initialClientDestConnID != nil {
		connIDs = append(connIDs, *m.initialClientDestConnID)
	}
	for _, connID := range m.activeSrcConnIDs {
		connIDs = append(connIDs, connID)
	}
	for _, c := range m.connIDsToRetire {
		connIDs = append(connIDs, c.connID)
	}
	m.connRunners.ReplaceWithClosed(connIDs, connClose, expiry)
}

func (m *connIDGenerator) AddConnRunner(runner connRunner, r connRunnerCallbacks) (added bool, err error) {
	if m.registrationFailure != nil {
		return false, m.registrationFailure
	}
	// The transport might have already been added earlier.
	// This happens if the application migrates back to and old path.
	if _, ok := m.connRunners[runner]; ok {
		return false, nil
	}
	// Attach the original prefix before any callback. Unknown callback/cleanup
	// completion must not detach registrations from their connection owner.
	m.connRunners[runner] = r
	if m.initialClientDestConnID != nil && !r.AddConnectionID(*m.initialClientDestConnID) {
		m.rollbackConnRunner(runner, r)
		return false, errCIDRegistration
	}
	for _, connID := range m.activeSrcConnIDs {
		if !r.AddConnectionID(connID) {
			m.rollbackConnRunner(runner, r)
			return false, errCIDRegistration
		}
	}
	return true, nil
}

func (m *connIDGenerator) rollbackConnRunner(runner connRunner, r connRunnerCallbacks) {
	if m.initialClientDestConnID != nil {
		r.RemoveConnectionID(*m.initialClientDestConnID)
	}
	for _, connID := range m.activeSrcConnIDs {
		r.RemoveConnectionID(connID)
	}
	delete(m.connRunners, runner)
}
