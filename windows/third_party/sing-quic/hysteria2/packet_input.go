package hysteria2

// One original input operation under receiveAccess. Queue publication transfers
// ownership; a pool return may invalidate the pointer. Clear aliases only after
// known return, and never inspect or retry a retained unknown pool prefix.
func (c *udpPacketConn) inputPacket(message *udpMessage) {
	c.receiveAccess.Lock()
	returned := false
	defer func() {
		failed := !returned || c.receiveFailed || c.defragger == nil || c.defragger.budget == nil
		if !failed {
			failed = c.defragger.budget.uncertain()
		}
		if failed {
			c.receiveFailed = true
			if c.defragger != nil && c.defragger.budget != nil {
				c.defragger.budget.poison()
			}
		}
		c.receiveAccess.Unlock()
		if failed {
			c.markShutdownUncertain() // no parent callback under receiveAccess
		}
	}()
	if c.receiveFailed || c.receiveInput != nil || c.receiveOutput != nil || c.defragger == nil || c.defragger.budget == nil {
		panic(errClientUncertain)
	}
	c.receiveInput = message
	select {
	case <-c.ctx.Done():
		message.releaseMessage()
		c.receiveInput = nil
		returned = true
		return
	default:
	}
	if message.fragmentTotal <= 1 {
		select {
		case c.data <- message:
		default:
			message.releaseMessage()
		}
		c.receiveInput = nil
	} else {
		newMessage := c.defragger.feed(message)
		// feed returned normally: the original input was released or retained
		// in the defragger. Do not dereference that possibly pooled alias.
		c.receiveInput = nil
		if newMessage != nil {
			c.receiveOutput = newMessage
			if c.defragger.assembly != newMessage || c.defragger.assemblyCharge != newMessage.rxBytes {
				panic(errClientUncertain)
			}
			// Both references are local to this serialized operation. The output
			// slot takes over before removing the original assembly reference.
			c.defragger.assembly = nil
			c.defragger.assemblyCharge = 0
			select {
			case c.data <- newMessage:
			default:
				newMessage.releaseMessage()
			}
			c.receiveOutput = nil
		}
	}
	returned = true
}
