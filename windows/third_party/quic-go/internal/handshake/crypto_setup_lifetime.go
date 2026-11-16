package handshake

import (
	"crypto/tls"
	"errors"
	"sync"
)

var ErrCryptoCloseUncertain = errors.New("quic: original TLS close did not return normally")

// Created on the original crypto shell before constructor callbacks. Only the
// original Close invokes TLS; observers join its result outside access.
type cryptoCloseState struct {
	access            sync.Mutex
	owner             *cryptoSetup // original shell, set before constructor callbacks
	done              chan struct{}
	started, returned bool
	original          *tls.QUICConn
	closed            *tls.QUICConn // exact returned original; diagnostics retain h.conn
	result            error
	eventsStarted     bool // external event retirement, guarded by access
	eventsResult      error
	keysStarted       bool
	keysResult        error
	keys              cryptoKeyReferences
}

// Close preserves the original TLS result, including an ordinary handshake
// error. QUICConn.Close joins the handshake before returning that error.
// Panic/Goexit is a different, retained negative outcome and is never retried.
func (h *cryptoSetup) Close() (result error) {
	r := &h.closeState
	r.access.Lock()
	if r.done == nil {
		r.access.Unlock()
		return ErrCryptoCloseUncertain
	}
	if r.started {
		done := r.done
		r.access.Unlock()
		<-done
		r.access.Lock()
		result = r.result
		r.access.Unlock()
		return result
	}
	r.started, r.original = true, h.conn
	original := r.original
	r.access.Unlock()
	returned := false
	defer func() {
		if !returned {
			result = errors.Join(ErrCryptoCloseUncertain, result)
		}
		r.access.Lock()
		defer r.access.Unlock()
		r.returned, r.result = returned, result
		if returned {
			r.closed = original
			r.original = nil // h.conn remains available for ConnectionState
		}
		close(r.done)
	}()
	if original == nil {
		return ErrCryptoCloseUncertain // a partial setup is not a closed TLS owner
	}
	result = original.Close() // no metadata lock over TLS or its handshake join
	returned = true
	return result
}

// WaitForShutdown reports original TLS call completion, not connection success.
// The external Conn owner calls it after native-loop shutdown; it never starts
// or retries Close. Do not call it from a TLS callback that Close itself joins.
func (h *cryptoSetup) WaitForShutdown() error {
	r := &h.closeState
	r.access.Lock()
	if r.done == nil || !r.started {
		r.access.Unlock()
		return ErrCryptoCloseUncertain
	}
	done := r.done
	r.access.Unlock()
	<-done
	r.access.Lock()
	defer r.access.Unlock()
	if !r.returned {
		return errors.Join(ErrCryptoCloseUncertain, r.result)
	}
	return nil // an ordinary TLS error is retained by Close, not a false success
}
