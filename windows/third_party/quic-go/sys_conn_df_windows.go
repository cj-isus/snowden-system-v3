//go:build windows

package quic

import (
	"errors"
	"syscall"

	"github.com/sagernet/quic-go/internal/utils"
	"golang.org/x/sys/windows"
)

const (
	// https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Networking/WinSock/constant.IP_DONTFRAGMENT.html
	//nolint:stylecheck
	IP_DONTFRAGMENT = 14
	// https://microsoft.github.io/windows-docs-rs/doc/windows/Win32/Networking/WinSock/constant.IPV6_DONTFRAG.html
	//nolint:stylecheck
	IPV6_DONTFRAG = 14
)

func setDF(rawConn syscall.RawConn) (bool, error) {
	// Zero values also cover a provider that returns without calling Control's
	// callback: missing observations must not enable PMTU probing.
	var enabledIPv4, enabledIPv6 bool
	if err := rawConn.Control(func(fd uintptr) {
		enabledIPv4 = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, IP_DONTFRAGMENT, 1) == nil
		enabledIPv6 = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, IPV6_DONTFRAG, 1) == nil
	}); err != nil {
		return false, err
	}
	switch {
	case enabledIPv4 && enabledIPv6:
		utils.DefaultLogger.Debugf("Setting DF for IPv4 and IPv6.")
	case enabledIPv4:
		utils.DefaultLogger.Debugf("Setting DF for IPv4.")
	case enabledIPv6:
		utils.DefaultLogger.Debugf("Setting DF for IPv6.")
	default:
		utils.DefaultLogger.Debugf("Setting DF failed for both IPv4 and IPv6.")
	}
	return enabledIPv4 || enabledIPv6, nil
}

func isSendMsgSizeErr(err error) bool {
	// https://docs.microsoft.com/en-us/windows/win32/winsock/windows-sockets-error-codes-2
	return errors.Is(err, windows.WSAEMSGSIZE)
}

func isRecvMsgSizeErr(err error) bool {
	// https://docs.microsoft.com/en-us/windows/win32/winsock/windows-sockets-error-codes-2
	return errors.Is(err, windows.WSAEMSGSIZE)
}
