package hysteria

import (
	"net"
	"os"
	"syscall"

	"github.com/sagernet/quic-go/packetaddr"
	"github.com/sagernet/sing/common"
)

// Caller holds access. Metadata getters and native callbacks use their own
// bounded lane; waiting readers/writers/control cannot exhaust it.
func (c *HopPacketConn) registerMetadataLocked(original *hopSocket) error {
	if c.done || original == nil || original != c.currentConn {
		return net.ErrClosed
	}
	if c.metadata >= maxHopMetadata {
		return errHopCapacity
	}
	c.metadata++
	c.callWG.Add(1)
	return nil
}

func (c *HopPacketConn) beginMetadata(original *hopSocket) (*hopSocket, error) {
	c.access.Lock()
	defer c.access.Unlock()
	if original == nil {
		original = c.currentConn
	}
	if err := c.registerMetadataLocked(original); err != nil {
		return nil, err
	}
	return original, nil
}

func (c *HopPacketConn) endMetadata(returned bool) {
	if !returned {
		c.beginClose(errHopCallUncertain)
	}
	c.access.Lock()
	c.metadata--
	c.access.Unlock()
	c.callWG.Done()
}

// Cache belongs to the exact socket. It can be cloned after closure without a
// provider callback. An unobserved/invalid address remains unknown (nil), never
// a placeholder or the previous generation's address.
func (c *HopPacketConn) LocalAddr() net.Addr {
	c.access.Lock()
	original := c.currentConn
	if original != nil && original.local.Valid() {
		local := original.local
		c.access.Unlock()
		return local.Addr()
	}
	if err := c.registerMetadataLocked(original); err != nil {
		c.access.Unlock()
		return nil
	}
	c.access.Unlock()
	returned := false
	defer func() { c.endMetadata(returned) }()
	local, err := packetaddr.Capture(original.conn.LocalAddr())
	if err != nil {
		returned = true
		return nil
	}
	c.access.Lock()
	if !original.local.Valid() {
		original.local = local
	}
	local = original.local
	c.access.Unlock()
	returned = true
	return local.Addr()
}

func (c *HopPacketConn) SyscallConn() (syscall.RawConn, error) {
	original, err := c.beginMetadata(nil)
	if err != nil {
		return nil, err
	}
	returned := false
	defer func() { c.endMetadata(returned) }()
	conn, ok := common.Cast[syscall.Conn](original.conn)
	if !ok {
		returned = true
		return nil, os.ErrInvalid
	}
	raw, err := conn.SyscallConn()
	returned = true
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, os.ErrInvalid
	}
	return &hopRawConn{owner: c, original: original, raw: raw}, nil
}

// Retain the original Go RawConn, never a numeric descriptor. Each callback
// operation is registered/joined; stale/closed handles cannot start native work.
// The provider/caller must still honor syscall.RawConn's synchronous FD contract.
type hopRawConn struct {
	owner    *HopPacketConn
	original *hopSocket
	raw      syscall.RawConn
}

func (r *hopRawConn) allowed() bool {
	r.owner.access.Lock()
	defer r.owner.access.Unlock()
	return !r.owner.done && r.owner.currentConn == r.original
}

func (r *hopRawConn) result(err error, called, stale bool) error {
	if stale {
		return net.ErrClosed
	}
	if err == nil && !called {
		r.owner.beginClose(errHopCallUncertain)
		return errHopCallUncertain
	}
	return err
}

func (r *hopRawConn) Control(callback func(uintptr)) error {
	if callback == nil {
		return os.ErrInvalid
	}
	if _, err := r.owner.beginMetadata(r.original); err != nil {
		return err
	}
	returned := false
	defer func() { r.owner.endMetadata(returned) }()
	called, stale := false, false
	err := r.raw.Control(func(fd uintptr) {
		if !r.allowed() {
			stale = true
			return
		}
		called = true
		callback(fd)
	})
	returned = true
	return r.result(err, called, stale)
}

func (r *hopRawConn) poll(write bool, callback func(uintptr) bool) error {
	if callback == nil {
		return os.ErrInvalid
	}
	if _, err := r.owner.beginMetadata(r.original); err != nil {
		return err
	}
	returned := false
	defer func() { r.owner.endMetadata(returned) }()
	called, stale := false, false
	wrapped := func(fd uintptr) bool {
		if !r.allowed() {
			stale = true
			return true // stop a stale poll without invoking caller code
		}
		called = true
		return callback(fd)
	}
	var err error
	if write {
		err = r.raw.Write(wrapped)
	} else {
		err = r.raw.Read(wrapped)
	}
	returned = true
	return r.result(err, called, stale)
}

func (r *hopRawConn) Read(callback func(uintptr) bool) error  { return r.poll(false, callback) }
func (r *hopRawConn) Write(callback func(uintptr) bool) error { return r.poll(true, callback) }
