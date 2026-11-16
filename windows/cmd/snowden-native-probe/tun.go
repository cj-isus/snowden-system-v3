package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

type nativeTUNSocket interface {
	Port() uint16
	Connect(context.Context) (net.Conn, error)
}

func runNativeTUNInvocation() int {
	// Subject/nonce never belong in a command line or process audit event.
	// The original parent sends one bounded request over the inherited pipe.
	startup := time.AfterFunc(5*time.Second, func() { os.Exit(3) })
	request, err := netprobe.ReadTUNRequest(os.Stdin)
	if !startup.Stop() || err != nil {
		return 2
	}
	return runNativeTUN(request)
}

// This mode owns one bound socket for its entire one-shot process. A result or
// released message is never cleanup proof: the parent must revoke BOTH scoped
// ingress and WFP filters, request release, and confirm original Job/I/O exit.
// The production parent bridge is still pending; no forwarding is enabled here.
func runNativeTUN(request netprobe.TUNRequest) int {
	if request.Validate() != nil {
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(request.TimeoutMillis)*time.Millisecond)
	defer cancel()
	// Anonymous pipe writes and native Connect need not honor a Go deadline.
	// A one-shot helper may exit; the parent must still prove full retirement.
	// This is not a hard Windows kernel-call termination guarantee.
	watchdog := time.AfterFunc(time.Duration(request.TimeoutMillis)*time.Millisecond, func() { os.Exit(3) })
	defer watchdog.Stop()
	socket, err := openNativeTUNSocket()
	if err != nil || socket == nil || socket.Port() == 0 {
		return 2
	}
	port := socket.Port()
	if netprobe.WriteTUNMessage(os.Stdout, netprobe.NewTUNMessage(request, "ready", port), request) != nil {
		return 2
	}
	command, err := netprobe.ReadTUNMessage(os.Stdin, request)
	if err != nil || command.Stage != "go" || command.SourcePort != port || ctx.Err() != nil {
		return 2
	}
	result := executeTUNHTTP(ctx, socket, request)
	if netprobe.WriteTUNMessage(os.Stdout, result, request) != nil {
		return 2
	}
	// Keep the original exclusive bound socket even after a failed connect,
	// EOF or closed TLS duplicate. Closing it here would permit port reuse
	// while the parent's diagnostic filters might still exist.
	command, err = netprobe.ReadTUNMessage(os.Stdin, request)
	if err != nil || command.Stage != "release" || command.SourcePort != port || ctx.Err() != nil {
		return 2
	}
	if netprobe.WriteTUNMessage(os.Stdout, netprobe.NewTUNMessage(request, "released", port), request) != nil {
		return 2
	}
	return 0 // main immediately os.Exit; retained socket resources die with process
}

func executeTUNHTTP(ctx context.Context, socket nativeTUNSocket, request netprobe.TUNRequest) netprobe.TUNMessage {
	result := netprobe.NewTUNMessage(request, "result", socket.Port())
	fail := func(kind string) netprobe.TUNMessage {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = "timeout"
		} else if errors.Is(ctx.Err(), context.Canceled) {
			kind = "canceled"
		}
		result.ErrorKind = kind
		return result
	}
	connection, err := socket.Connect(ctx)
	if err != nil || connection == nil {
		return fail("connect_unavailable")
	}
	deadline, ok := ctx.Deadline()
	if !ok || connection.SetDeadline(deadline) != nil {
		return fail("connect_unavailable")
	}
	secured := tls.Client(connection, &tls.Config{ServerName: "1.1.1.1", MinVersion: tls.VersionTLS12})
	if secured.HandshakeContext(ctx) != nil {
		return fail("tls_unavailable")
	}
	var used atomic.Bool
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10,
		DialTLSContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" || address != netprobe.TUNProbeAddress || ctx.Err() != nil || used.Swap(true) {
				return nil, netprobe.ErrNativeProtocol
			}
			return secured, nil
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, netprobe.TUNProbeURL, nil)
	if err != nil {
		return fail("http_unavailable")
	}
	httpRequest.Header.Set("Cache-Control", "no-cache, no-store")
	httpRequest.Header.Set("Pragma", "no-cache")
	response, err := client.Do(httpRequest)
	if err != nil {
		return fail("http_unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, netprobe.MaxTUNBody+1))
	if len(body) > netprobe.MaxTUNBody {
		return fail("response_limit")
	}
	if err != nil || ctx.Err() != nil {
		return fail("http_unavailable")
	}
	result.HTTPStatus, result.Body = response.StatusCode, body
	return result
}
