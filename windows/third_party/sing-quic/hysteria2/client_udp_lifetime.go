package hysteria2

import (
	"context"
	"errors"
	"os"
)

var errUDPConstructing = errors.New("HY2 original UDP construction is still pending")

func udpOperationContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return context.Canceled
}

func (c *clientQUICConnection) listenPacket(ctx context.Context) (result *udpPacketConn, err error) {
	operationDone := ctx.Done() // caller Context methods are outside carrier locks
	c.udpAccess.Lock()
	locked, returned := true, false
	var original *udpPacketConstruction
	defer func() {
		if !returned && locked && original != nil {
			original.initializationFailed = true
		}
		if locked {
			c.udpAccess.Unlock()
		}
		if !returned {
			c.markUncertain() // no notification while holding udpAccess/lifeAccess
		}
	}()
	select {
	case <-c.connDone:
		returned = true
		return nil, errors.Join(c.connErr, os.ErrClosed)
	default:
	}
	select {
	case <-operationDone:
		c.udpAccess.Unlock()
		locked, returned = false, true
		return nil, udpOperationContextError(ctx)
	default:
	}
	if c.udpConstructing != nil {
		returned = true
		return nil, errUDPConstructing // bounded refusal, not a fresh constructor
	}
	if len(c.udpConnMap) >= maxUDPReceiveSessions {
		c.receiveBudget.drop()
		returned = true
		return nil, errUDPReceiveCapacity
	}
	// Reserve a free ID across uint32 wrap without replacing a live session.
	sessionID := c.udpSessionID
	for attempts := 0; attempts <= maxUDPReceiveSessions; attempts++ {
		if _, present := c.udpConnMap[sessionID]; !present {
			break
		}
		sessionID++
	}
	c.udpSessionID = sessionID + 1
	original = newUDPPacketConstruction(c.markUncertain)
	c.udpConstructing = original // original candidate rooted before native scope
	candidate := original.candidate
	candidate.sessionID = sessionID
	err = initializeUDPPacketConn(candidate, c.ctx, c.quicConn, &c.receiveBudget, &c.sendBudget, func() {
		c.udpAccess.Lock()
		if c.udpConnMap[sessionID] == candidate {
			delete(c.udpConnMap, sessionID)
		}
		c.udpAccess.Unlock()
	})
	operationCancelled, carrierClosing := false, false
	if err == nil {
		// One short metadata fence with requestClose. Order: udpAccess ->
		// lifeAccess; close never holds lifeAccess while acquiring udpAccess.
		// No Context/native/pool/cancel/error-formatting call under lifeAccess.
		func() {
			c.lifeAccess.Lock()
			defer c.lifeAccess.Unlock()
			if c.failure != nil {
				err = c.failure
				return
			}
			if c.closing {
				err, carrierClosing = c.connErr, true
				return
			}
			select {
			case <-operationDone:
				operationCancelled = true
				return
			default:
			}
			c.udpConnMap[sessionID] = candidate
			c.udpConstructing = nil // map takes over before original slot is removed
			result = candidate
		}()
		if result != nil {
			returned = true
			return result, nil
		}
	}
	// Unpublished prefix remains in the carrier while the one original abort
	// runs outside its locks. A retiring carrier may join the same Once/result.
	c.udpAccess.Unlock()
	locked = false
	abortErr := original.abort()
	c.udpAccess.Lock()
	locked = true
	if c.udpConstructing == original && abortErr == nil && !original.initializationFailed {
		c.udpConstructing = nil
	}
	c.udpAccess.Unlock()
	locked, returned = false, true
	if operationCancelled {
		err = udpOperationContextError(ctx)
	}
	if carrierClosing {
		err = errors.Join(err, os.ErrClosed)
	}
	if abortErr != nil {
		err = errors.Join(err, abortErr)
	}
	return nil, err
}
