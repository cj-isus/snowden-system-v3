//go:build windows

package main

import (
	"context"
	"errors"
	"net"
	"os"

	"golang.org/x/sys/windows"
)

// Deliberately process-owned, not finalized or reused by another measurement.
// The raw socket reserves the wildcard port while net.FileConn's duplicate
// serves TLS/HTTP. No DuplicateHandle socket transfer or PID lookup is used.
// The parent must retain its original process/Job until every resource dies.
type windowsTUNSocket struct {
	data           windows.WSAData
	raw            windows.Handle
	port           uint16
	connectStarted bool
	file           *os.File
	connection     net.Conn
}

var heldTUNSocket *windowsTUNSocket

var errTUNSocket = errors.New("exclusive native TUN socket unavailable")

const soExclusiveAddrUse = ^windows.SO_REUSEADDR // Winsock2.h SO_EXCLUSIVEADDRUSE

func openNativeTUNSocket() (nativeTUNSocket, error) {
	if heldTUNSocket != nil {
		return nil, errTUNSocket
	}
	owner := &windowsTUNSocket{raw: windows.InvalidHandle}
	heldTUNSocket = owner // retain before any native acquisition, including errors
	if windows.WSAStartup(0x202, &owner.data) != nil || owner.data.Version != 0x202 {
		return nil, errTUNSocket
	}
	var err error
	owner.raw, err = windows.WSASocket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT)
	if err != nil || owner.raw == windows.InvalidHandle {
		return nil, errTUNSocket
	}
	if windows.SetsockoptInt(owner.raw, windows.SOL_SOCKET, soExclusiveAddrUse, 1) != nil {
		return nil, errTUNSocket
	}
	if windows.Bind(owner.raw, &windows.SockaddrInet4{}) != nil {
		return nil, errTUNSocket
	}
	address, err := windows.Getsockname(owner.raw)
	bound, ok := address.(*windows.SockaddrInet4)
	exclusive, optionErr := windows.GetsockoptInt(owner.raw, windows.SOL_SOCKET, soExclusiveAddrUse)
	if err != nil || !ok || bound.Port < 1 || bound.Port > 65535 || bound.Addr != [4]byte{} || optionErr != nil || exclusive == 0 {
		return nil, errTUNSocket
	}
	owner.port = uint16(bound.Port)
	return owner, nil
}

func (owner *windowsTUNSocket) Port() uint16 { return owner.port }

func (owner *windowsTUNSocket) Connect(ctx context.Context) (net.Conn, error) {
	if owner != heldTUNSocket || owner.connectStarted || owner.port == 0 || ctx == nil || ctx.Err() != nil {
		return nil, errTUNSocket
	}
	owner.connectStarted = true // never retry another socket/port after failure
	if windows.Connect(owner.raw, &windows.SockaddrInet4{Port: 443, Addr: [4]byte{1, 1, 1, 1}}) != nil || ctx.Err() != nil {
		return nil, errTUNSocket
	}
	address, err := windows.Getsockname(owner.raw)
	bound, ok := address.(*windows.SockaddrInet4)
	if err != nil || !ok || bound.Port != int(owner.port) {
		return nil, errTUNSocket
	}
	peer, err := windows.Getpeername(owner.raw)
	remote, ok := peer.(*windows.SockaddrInet4)
	if err != nil || !ok || remote.Port != 443 || remote.Addr != [4]byte{1, 1, 1, 1} {
		return nil, errTUNSocket
	}
	owner.file = os.NewFile(uintptr(owner.raw), "snowden-tun-original-socket")
	if owner.file == nil {
		return nil, errTUNSocket
	}
	owner.connection, err = net.FileConn(owner.file)
	if err != nil || owner.connection == nil {
		return nil, errTUNSocket
	}
	return owner.connection, nil
}
