package hysteria2

import (
	"errors"
	"io"
	"net"

	"github.com/sagernet/sing/common/buf"
	N "github.com/sagernet/sing/common/network"
)

const (
	maxTCPVectorWrites     = maxTCPConnections
	maxTCPVectorInputs     = 256
	maxTCPVectorHeldInputs = 4096
	maxTCPVectorBytes      = 16 << 20
)

var (
	_ N.VectorisedWriter = (*clientConn)(nil)
	_ N.ExtendedWriter   = (*clientConn)(nil)
)

var (
	errTCPVectorSize     = errors.New("invalid or oversized HY2 TCP vector input")
	errTCPVectorCapacity = errors.New("HY2 TCP vector input capacity exceeded")
)

// Fixed carrier slots own the original consuming inputs, not their caller's
// slice backing or a full merged copy. Exposed backing is charged separately
// from the existing bounded request-prefix staging. Hidden allocator/alias
// storage and upstream generation before this API remain outside this budget.
type tcpVectorWrite struct {
	held   bool
	bytes  int
	count  int
	pinned int
	inputs []*buf.Buffer
	single [1]*buf.Buffer
}

func tcpVectorCharge(inputs []*buf.Buffer) (charge int, hasPayload bool, err error) {
	if len(inputs) > maxTCPVectorInputs {
		return 0, false, errTCPVectorSize
	}
	for _, input := range inputs {
		if input == nil {
			return 0, false, errTCPVectorSize
		}
		start, length, capacity, backing := input.Start(), input.Len(), input.Cap(), input.RawCap()
		if start < 0 || start > capacity || capacity > backing || backing > maxTCPVectorBytes-charge ||
			length < 0 || length > capacity-start {
			return 0, false, errTCPVectorSize
		}
		// buf.As can expose a slice with capacity greater than its length.
		// Include that reachable suffix before retaining the original object.
		view := input.Bytes()
		if cap(view) > maxTCPVectorBytes-charge-start {
			return 0, false, errTCPVectorSize
		}
		charge += max(backing, start+cap(view))
		hasPayload = hasPayload || length != 0
	}
	return charge, hasPayload, nil
}

func (c *clientConn) reserveVectorWrite(count, charge int) (*tcpVectorWrite, error) {
	c.owner.lifeAccess.Lock()
	defer c.owner.lifeAccess.Unlock()
	if c.poisoned || c.owner.tcpFailure != nil || c.owner.failure != nil {
		return nil, errClientUncertain
	}
	if c.owner.closing || c.closing {
		return nil, net.ErrClosed
	}
	if charge > maxTCPVectorBytes-c.owner.tcpVectorBytes || count > maxTCPVectorHeldInputs-c.owner.tcpVectorInputs {
		return nil, errTCPVectorCapacity
	}
	for index := range c.owner.tcpVectors {
		original := &c.owner.tcpVectors[index]
		if !original.held {
			original.held, original.count, original.bytes = true, count, charge
			c.owner.tcpVectorBytes += charge
			c.owner.tcpVectorInputs += count
			return original, nil
		}
	}
	return nil, errTCPVectorCapacity
}

// No native write still borrows any of these inputs. Pool release precedes
// returning the slot/byte credit; a cleanup panic leaves the original slot held.
func (c *clientConn) releaseVectorWrite(original *tcpVectorWrite) {
	for index := 0; index < original.pinned; index++ {
		input := original.inputs[index]
		input.DecRef()
		input.Release()
		original.inputs[index] = nil
	}
	c.owner.lifeAccess.Lock()
	c.owner.tcpVectorBytes -= original.bytes
	c.owner.tcpVectorInputs -= original.count
	*original = tcpVectorWrite{}
	c.owner.lifeAccess.Unlock()
}

// The default scalar copy path uses the same consuming custody as the batch
// path, instead of an upstream adapter's unconditional deferred pool release.
// This fixed one-pointer call frame has no input-sized metadata allocation.
func (c *clientConn) WriteBuffer(input *buf.Buffer) error {
	inputs := [1]*buf.Buffer{input}
	return c.WriteVectorised(inputs[:])
}

// WriteVectorised follows sing's consuming API, including ordinary error and
// admission refusal. Abnormal native return retains the original ref-pinned
// inputs and poisons the carrier before another admitted writer can take over.
// It creates no worker, timer, payload merge, or upstream/raw-stream escape.
func (c *clientConn) WriteVectorised(inputs []*buf.Buffer) error {
	registered, locked, returned := false, false, false
	var original *tcpVectorWrite
	defer func() {
		cleaned := false
		defer func() {
			if registered {
				if !returned || !cleaned {
					c.poisonBeforeUnlock()
				}
				if locked {
					c.write.Unlock()
				}
				c.endCall(false, returned && cleaned)
			}
		}()
		if original != nil {
			if !returned {
				return // no release, refund or replacement of uncertain originals
			}
			c.releaseVectorWrite(original)
		} else {
			buf.ReleaseMulti(inputs)
		}
		cleaned = true
	}()
	charge, hasPayload, err := tcpVectorCharge(inputs)
	if err != nil {
		returned = true
		return err
	}
	if err = c.beginCall(false); err != nil {
		returned = true
		return err
	}
	registered = true
	if len(inputs) != 0 {
		original, err = c.reserveVectorWrite(len(inputs), charge)
		if err != nil {
			returned = true
			return err
		}
		// Both pointer count and exposed backing were reserved before make.
		// Copy only the admitted pointers, never retain a larger caller table.
		if len(inputs) == 1 {
			original.inputs = original.single[:]
		} else {
			original.inputs = make([]*buf.Buffer, len(inputs))
		}
		for index, input := range inputs {
			original.inputs[index] = input
			input.IncRef()
			original.pinned++
		}
	}
	// Queued writers retain and charge their inputs too. Waiting for the
	// direction mutex must not hide consuming buffers outside the carrier cap.
	c.write.Lock()
	locked = true
	if c.isClosed() || c.writeClosed.Load() {
		returned = true
		return net.ErrClosed
	}
	if !hasPayload {
		_, err = c.writePayload(nil) // preserve Write(nil)'s early-header flush
	} else {
		for index := 0; index < original.count; index++ {
			payload := original.inputs[index].Bytes()
			if len(payload) == 0 {
				continue
			}
			var n int
			n, err = c.writePayload(payload)
			if err == nil && n != len(payload) {
				err = io.ErrShortWrite
			}
			if err != nil {
				break // partial bytes may be sent; this API cannot return their count
			}
		}
	}
	returned = true
	return err
}
