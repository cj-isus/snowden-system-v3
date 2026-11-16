package hysteria2

import (
	"errors"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/canceler"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

func (s *serverSession[U]) loopMessages() {
	for {
		message, err := s.quicConn.ReceiveDatagram(s.ctx)
		if err != nil {
			s.closeWithError(E.Cause(err, "receive message"))
			return
		}
		hErr := s.handleMessage(message)
		if hErr != nil {
			s.closeWithError(E.Cause(hErr, "handle message"))
			return
		}
	}
}

func (s *serverSession[U]) handleMessage(data []byte) error {
	message, err := receiveUDPMessage(data, &s.receiveBudget)
	if err != nil {
		if errors.Is(err, errUDPReceiveCapacity) {
			return nil
		}
		return E.Cause(err, "decode UDP message")
	}
	s.handleUDPMessage(message)
	return nil
}

func (s *serverSession[U]) handleUDPMessage(message *udpMessage) {
	sessionID := message.sessionID
	s.udpAccess.Lock()
	select {
	case <-s.connDone:
		s.udpAccess.Unlock()
		message.releaseMessage()
		return
	default:
	}
	udpConn, loaded := s.udpConnMap[sessionID]
	if !loaded || common.Done(udpConn.ctx) {
		if !loaded && len(s.udpConnMap) >= maxUDPReceiveSessions {
			s.udpAccess.Unlock()
			s.receiveBudget.drop()
			message.releaseMessage()
			return
		}
		if loaded {
			udpConn.closeWithError(errUDPReceiveCapacity)
		}
		var created *udpPacketConn
		var createErr error
		created, createErr = newUDPPacketConn(auth.ContextWithUser(s.ctx, s.authUser), s.quicConn, &s.receiveBudget, &s.sendBudget, func() {
			s.udpAccess.Lock()
			if s.udpConnMap[sessionID] == created {
				delete(s.udpConnMap, sessionID)
			}
			s.udpAccess.Unlock()
		})
		if createErr != nil {
			s.udpAccess.Unlock()
			s.receiveBudget.drop()
			message.releaseMessage()
			return
		}
		udpConn = created
		udpConn.sessionID = sessionID
		s.udpConnMap[sessionID] = udpConn
		s.udpAccess.Unlock()
		newCtx, newConn := canceler.NewPacketConn(udpConn.ctx, udpConn, s.udpTimeout)
		go s.handler.NewPacketConnectionEx(newCtx, newConn, M.SocksaddrFromNet(s.quicConn.RemoteAddr()).Unwrap(), M.ParseSocksaddr(message.destination).Unwrap(), nil)
	} else {
		s.udpAccess.Unlock()
	}
	udpConn.inputPacket(message)
}
