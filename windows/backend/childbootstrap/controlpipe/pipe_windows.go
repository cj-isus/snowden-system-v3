//go:build windows

package controlpipe

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/snowden-system/windows/backend/childbootstrap"
	"github.com/snowden-system/windows/backend/childbootstrap/fileacl"
	"github.com/snowden-system/windows/backend/childbootstrap/nativelease"
	"golang.org/x/sys/windows"
)

var pipeConvertSD = windows.NewLazySystemDLL("advapi32.dll").NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")

// Publish this zero owner before acquisition; do not copy it after use. It starts
// no I/O goroutine/overlapped request and retains every uncertain original value.
type Pipe struct {
	mu                      sync.Mutex
	retiring                atomic.Bool
	handle                  windows.Handle
	wire                    pipeWire // fixed before native acquire; never changed/reused
	sequencing              bool     // latched under mu before broker client sequencing
	server, connected       bool
	peerID                  uint32 // comparison only, never PID-based acquisition authority
	attempted, acquiring    bool
	ioPending               bool // native Write/Read/EOF has not actually returned
	failed, closed          bool
	awaitingEOF             bool
	closeAttempted          bool
	descriptorFreeAttempted bool
	sidFreeAttempted        bool
	descriptor              *windows.SECURITY_DESCRIPTOR // original Windows allocation, no hidden LocalFree
	sidText                 *uint16                      // original ConvertSidToStringSid allocation
}

func (pipe *Pipe) finish(completed *bool) {
	if !*completed {
		pipe.failed = true // includes callback panic/Goexit; original owner remains
	}
	pipe.mu.Unlock()
}

// Retire is a value-only stop intent, including while an original call is busy.
// It does not claim interruption or cleanup; Close still needs original results.
func (pipe *Pipe) Retire() {
	if pipe != nil {
		pipe.retiring.Store(true)
	}
}

func (pipe *Pipe) scope(ctx context.Context, check func(context.Context) error) error {
	if pipe.retiring.Load() {
		return ErrPipe
	}
	if err := checkScope(ctx, check); err != nil {
		return err
	}
	if pipe.retiring.Load() {
		return ErrPipe
	}
	return nil
}

func (pipe *Pipe) makeDescriptor(ctx context.Context, check func(context.Context) error) error {
	user, err := fileacl.CurrentUser()
	if err != nil || user == nil || !user.IsValid() {
		return ErrPipe
	}
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	pipe.acquiring = true
	err = windows.ConvertSidToStringSid(user, &pipe.sidText)
	pipe.acquiring = false
	runtime.KeepAlive(user)
	if err != nil || pipe.sidText == nil {
		return errors.Join(ErrPipe, err)
	}
	// The source is a validated primary-token SID formatted by Windows. Keep the
	// existing bounded conversion rather than accepting caller-controlled SDDL.
	sid := windows.UTF16ToString(unsafe.Slice(pipe.sidText, 256))
	if len(sid) == 0 || len(sid) >= 256 {
		return ErrPipe
	}
	text, err := windows.UTF16PtrFromString("O:" + sid + "D:P(A;;GA;;;SY)(A;;GA;;;" + sid + ")")
	if err != nil {
		return err
	}
	if err := pipeConvertSD.Find(); err != nil {
		return err
	}
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	pipe.acquiring = true
	ok, _, callErr := pipeConvertSD.Call(uintptr(unsafe.Pointer(text)), 1, uintptr(unsafe.Pointer(&pipe.descriptor)), 0)
	pipe.acquiring = false
	runtime.KeepAlive(text)
	runtime.KeepAlive(pipe)
	if ok == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return ErrPipe
	}
	if pipe.descriptor == nil {
		return ErrPipe
	}
	return pipe.scope(ctx, check)
}

func (pipe *Pipe) Listen(ctx context.Context, nonce string, check func(context.Context) error) error {
	return pipe.listen(ctx, nonce, wireUAC, check)
}

func (pipe *Pipe) listen(ctx context.Context, nonce string, wire pipeWire, check func(context.Context) error) error {
	if pipe == nil || !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	name, err := wireName(wire, nonce)
	if err != nil || pipe.handle != 0 || pipe.attempted || pipe.closed || pipe.failed {
		return ErrPipe
	}
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	pipe.attempted, pipe.wire = true, wire
	if err := pipe.makeDescriptor(ctx, check); err != nil {
		return err
	}
	security := windows.SecurityAttributes{SecurityDescriptor: pipe.descriptor}
	security.Length = uint32(unsafe.Sizeof(security))
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	pipe.server, pipe.acquiring = true, true
	pipe.handle, err = windows.CreateNamedPipe(path, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_NOWAIT|windows.PIPE_REJECT_REMOTE_CLIENTS, 1, 256, 256, 0, &security)
	pipe.acquiring = false
	runtime.KeepAlive(pipe.descriptor)
	if pipe.handle == windows.InvalidHandle {
		pipe.handle = 0
	}
	if err != nil {
		return err
	}
	if err := errors.Join(pipe.state(), pipe.scope(ctx, check)); err != nil {
		return err
	}
	completed = true
	return nil
}

func (pipe *Pipe) Dial(ctx context.Context, nonce string, check func(context.Context) error) error {
	return pipe.dial(ctx, nonce, wireUAC, check)
}

func (pipe *Pipe) dial(ctx context.Context, nonce string, wire pipeWire, check func(context.Context) error) error {
	if pipe == nil || !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	name, err := wireName(wire, nonce)
	if err != nil || pipe.handle != 0 || pipe.attempted || pipe.closed || pipe.failed {
		return ErrPipe
	}
	pipe.attempted, pipe.wire = true, wire
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	for {
		if err := pipe.scope(ctx, check); err != nil {
			return err
		}
		pipe.acquiring = true
		pipe.handle, err = windows.CreateFile(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
			windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		pipe.acquiring = false
		if pipe.handle == windows.InvalidHandle {
			pipe.handle = 0
		}
		if err == nil {
			break
		}
		if pipe.handle != 0 || !errors.Is(err, windows.ERROR_PIPE_BUSY) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return err
		}
		if err := pause(ctx, check); err != nil {
			return err
		}
	}
	mode := uint32(windows.PIPE_READMODE_BYTE | windows.PIPE_NOWAIT)
	if err := windows.SetNamedPipeHandleState(pipe.handle, &mode, nil, nil); err != nil {
		return err
	}
	if err := pipe.state(); err != nil {
		return err
	}
	if err := windows.GetNamedPipeServerProcessId(pipe.handle, &pipe.peerID); err != nil || pipe.peerID == 0 {
		return ErrPipe
	}
	pipe.connected = true
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	completed = true
	return nil
}

func (pipe *Pipe) state() error {
	if !nativelease.ValidHandleValue(pipe.handle) {
		return ErrPipe
	}
	var flags, maxInstances, mode uint32
	if err := windows.GetNamedPipeInfo(pipe.handle, &flags, nil, nil, &maxInstances); err != nil {
		return err
	}
	if flags&windows.PIPE_TYPE_MESSAGE != 0 || (flags&windows.PIPE_SERVER_END != 0) != pipe.server || maxInstances != 1 {
		return ErrPipe
	}
	if err := windows.GetNamedPipeHandleState(pipe.handle, &mode, nil, nil, nil, nil, 0); err != nil {
		return err
	}
	if mode&windows.PIPE_NOWAIT == 0 || mode&windows.PIPE_READMODE_MESSAGE != 0 {
		return ErrPipe
	}
	return nil
}

func (pipe *Pipe) check(ctx context.Context, check func(context.Context) error) error {
	if !pipe.connected || pipe.peerID == 0 {
		return ErrPipe
	}
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	var actual uint32
	query := windows.GetNamedPipeServerProcessId
	if pipe.server {
		query = windows.GetNamedPipeClientProcessId
	}
	if err := query(pipe.handle, &actual); err != nil || actual != pipe.peerID {
		return ErrPipe
	}
	return pipe.scope(ctx, check)
}

func (pipe *Pipe) Accept(ctx context.Context, expected uint32, check func(context.Context) error) error {
	if pipe == nil || !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	if pipe.failed || pipe.closed || !pipe.server || pipe.connected || expected == 0 {
		return ErrPipe
	}
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	if err := pipe.state(); err != nil {
		return err
	}
	for {
		if err := pipe.scope(ctx, check); err != nil {
			return err
		}
		err := windows.ConnectNamedPipe(pipe.handle, nil)
		if err != nil && !errors.Is(err, windows.ERROR_PIPE_LISTENING) && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			return err
		}
		var actual uint32
		peerErr := windows.GetNamedPipeClientProcessId(pipe.handle, &actual)
		if peerErr == nil {
			if actual != expected {
				return ErrPipe
			}
			pipe.peerID, pipe.connected = expected, true
			if err := pipe.scope(ctx, check); err != nil {
				return err
			}
			completed = true
			return nil
		}
		if !errors.Is(peerErr, windows.ERROR_PIPE_NOT_CONNECTED) && !errors.Is(peerErr, windows.ERROR_PIPE_LISTENING) && !errors.Is(peerErr, windows.ERROR_NO_DATA) {
			return peerErr
		}
		if err := pause(ctx, check); err != nil {
			return err
		}
	}
}

// CheckServer compares only to the PID derived from the caller's held original
// parent handle. It neither acquires a process nor authorizes its image/SID.
func (pipe *Pipe) CheckServer(ctx context.Context, expected uint32, check func(context.Context) error) error {
	if pipe == nil || !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	if pipe.failed || pipe.closed || pipe.server || pipe.awaitingEOF || expected == 0 || expected != pipe.peerID {
		return ErrPipe
	}
	if err := pipe.check(ctx, check); err != nil {
		return err
	}
	completed = true
	return nil
}

// A broker sequencer runs only on the original connected client endpoint.
// This is a transport/peer liveness check, not image/SID/Job authentication.
func (pipe *Pipe) readyClient(ctx context.Context, wire pipeWire, check func(context.Context) error) error {
	if pipe == nil || !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	if pipe.failed || pipe.closed || pipe.server || pipe.awaitingEOF || pipe.sequencing || pipe.wire != wire ||
		wire != wireBrokerControl && wire != wireBrokerSuccessor {
		return ErrPipe
	}
	pipe.sequencing = true
	if err := pipe.check(ctx, check); err != nil {
		return err
	}
	completed = true
	return nil
}

func (pipe *Pipe) Exchange(ctx context.Context, send, receive []byte, check func(context.Context) error) error {
	return pipe.exchange(ctx, send, receive, wireUAC, false, check)
}

func (pipe *Pipe) ExchangeObserved(ctx context.Context, send, receive []byte, receipt *ExchangeReceipt, check func(context.Context) error) error {
	if receipt == nil {
		return ErrPipe
	}
	return pipe.exchangeObserved(ctx, send, receive, wireUAC, false, receipt, check)
}

func (pipe *Pipe) exchange(ctx context.Context, send, receive []byte, wire pipeWire, sequenced bool, check func(context.Context) error) error {
	return pipe.exchangeObserved(ctx, send, receive, wire, sequenced, nil, check)
}

func (pipe *Pipe) exchangeObserved(ctx context.Context, send, receive []byte, wire pipeWire, sequenced bool, receipt *ExchangeReceipt, check func(context.Context) error) error {
	if receipt != nil {
		if err := receipt.begin(send, len(receive)); err != nil {
			return err
		}
	}
	if pipe == nil || !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	if pipe.failed || pipe.closed || pipe.awaitingEOF || pipe.ioPending || pipe.wire != wire || pipe.sequencing != sequenced || !wire.validExchange(len(send), len(receive)) {
		return ErrPipe
	}
	if err := pipe.scope(ctx, check); err != nil {
		return err
	}
	if err := pipe.state(); err != nil {
		return err
	}
	var localOutput, localInput [childbootstrap.BrokerSuccessorSeedSize]byte
	output, input := &localOutput, &localInput
	if receipt != nil {
		output, input = &receipt.send, &receipt.reply
	}
	copy(output[:], send)
	for offset := 0; offset < len(send); {
		if err := pipe.check(ctx, check); err != nil {
			return err
		}
		var count uint32
		pipe.ioPending = true
		if receipt != nil {
			receipt.writeAttempted, receipt.writePending = true, true
		}
		err := windows.WriteFile(pipe.handle, output[offset:len(send)], &count, nil)
		pipe.ioPending = false
		if receipt != nil {
			receipt.writePending, receipt.writeReturned = false, true
			if count > uint32(len(send)-offset) {
				receipt.fault = true
			} else {
				receipt.written += int(count)
			}
		}
		if err != nil {
			return err
		}
		if count > uint32(len(send)-offset) {
			return ErrPipe
		}
		offset += int(count)
		if count == 0 {
			if err := pause(ctx, check); err != nil {
				return err
			}
		}
	}
	for offset := 0; offset < len(receive); {
		if err := pipe.check(ctx, check); err != nil {
			return err
		}
		var count uint32
		pipe.ioPending = true
		if receipt != nil {
			receipt.readPending = true
		}
		err := windows.ReadFile(pipe.handle, input[offset:len(receive)], &count, nil)
		pipe.ioPending = false
		if receipt != nil {
			receipt.readPending, receipt.readReturned = false, true
			if count > uint32(len(receive)-offset) {
				receipt.fault = true
			} else {
				receipt.read += int(count)
			}
		}
		if err != nil && !errors.Is(err, windows.ERROR_NO_DATA) {
			return err
		}
		if count > uint32(len(receive)-offset) || err == nil && count == 0 {
			return ErrPipe
		}
		offset += int(count)
		if count == 0 {
			if err := pause(ctx, check); err != nil {
				return err
			}
		}
	}
	if len(receive) == 0 && !pipe.server {
		// Parent can close after reading final ACK: no post-write peer query.
		if err := pipe.scope(ctx, check); err != nil {
			return err
		}
		pipe.awaitingEOF = true
	} else {
		if err := pipe.check(ctx, check); err != nil {
			return err
		}
	}
	copy(receive, input[:len(receive)])
	completed = true
	return nil
}

func (pipe *Pipe) close() error {
	pipe.Retire()
	if pipe.acquiring || pipe.ioPending {
		return ErrCleanup // an original acquisition or native I/O has not actually returned
	}
	if pipe.handle != 0 {
		if pipe.closeAttempted || !nativelease.ValidHandleValue(pipe.handle) {
			return ErrCleanup
		}
		pipe.closeAttempted = true
		if err := windows.CloseHandle(pipe.handle); err != nil {
			return errors.Join(ErrCleanup, err)
		}
		pipe.handle = 0
	}
	if pipe.descriptor != nil {
		if pipe.descriptorFreeAttempted {
			return ErrCleanup
		}
		pipe.descriptorFreeAttempted = true
		remaining, err := windows.LocalFree(windows.Handle(unsafe.Pointer(pipe.descriptor)))
		if err != nil || remaining != 0 {
			return errors.Join(ErrCleanup, err)
		}
		pipe.descriptor = nil
	}
	if pipe.sidText != nil {
		if pipe.sidFreeAttempted {
			return ErrCleanup
		}
		pipe.sidFreeAttempted = true
		remaining, err := windows.LocalFree(windows.Handle(unsafe.Pointer(pipe.sidText)))
		if err != nil || remaining != 0 {
			return errors.Join(ErrCleanup, err)
		}
		pipe.sidText = nil
	}
	pipe.closed = true
	return nil
}

func (pipe *Pipe) Close() error {
	if pipe == nil {
		return nil
	}
	pipe.Retire() // burn positive work even if its original operation is busy
	if !pipe.mu.TryLock() {
		return ErrCleanup
	}
	defer pipe.mu.Unlock()
	return pipe.close()
}

func (pipe *Pipe) CloseAfterServer(ctx context.Context, check func(context.Context) error) error {
	if pipe == nil {
		return ErrPipe
	}
	pipe.Retire()
	if !pipe.mu.TryLock() {
		return ErrPipe
	}
	completed := false
	defer pipe.finish(&completed)
	if pipe.failed || pipe.closed || pipe.server || !pipe.connected || !pipe.awaitingEOF || !nativelease.ValidHandleValue(pipe.handle) {
		return ErrPipe
	}
	// Unchanged NOWAIT was verified by final ACK. No metadata/peer query here:
	// expected server Close can race either query. EOF itself authorizes nothing.
	for {
		if err := checkScope(ctx, check); err != nil {
			return err
		}
		var data [1]byte
		var count uint32
		pipe.ioPending = true
		err := windows.ReadFile(pipe.handle, data[:], &count, nil)
		pipe.ioPending = false
		if count != 0 {
			return ErrPipe
		}
		if errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) || err == nil {
			if err := errors.Join(pipe.close(), checkScope(ctx, check)); err != nil {
				return err
			}
			completed = true
			return nil
		}
		if !errors.Is(err, windows.ERROR_NO_DATA) {
			return err
		}
		if err := pause(ctx, check); err != nil {
			return err
		}
	}
}
