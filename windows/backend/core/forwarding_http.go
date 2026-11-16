// HTTP orchestration adapted from sing v0.8.13/protocol/http/handshake.go.
// Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>.
// SPDX-License-Identifier: GPL-3.0-or-later
package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	SBufio "github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	H "github.com/sagernet/sing/protocol/http"
)

// The SDK's CONNECT cached connection moves parser bytes into an opaque pooled
// buffer. Reuse our original input's prefix owner instead. Own the outer loop so
// CONNECT after an ordinary keep-alive request cannot bypass that owner. SDK
// request/auth/address codecs and ordinary HTTP/Upgrade algorithms are retained.
// Pair and used-body API receipts are input-owned; Transport and other opaque
// SDK work still require their own retirement evidence.
func (processor *ownedMixedProcessor) http(ctx context.Context, conn net.Conn, reader *bufio.Reader, input *mixedHandshakeReader, metadata adapter.InboundContext, callback N.CloseHandlerFunc) error {
	if ctx == nil || conn == nil || reader == nil || input == nil || input.conn == nil || input.conn.owner == nil || input.conn.owner.gate == nil || input.conn != conn || ctx.Value(mixedInputKey{}) != input.conn.owner {
		return ErrForwardingNotAdmitted
	}
	handler := adapter.NewUpstreamHandlerEx(metadata, processor.connection, processor.packet)
	source := metadata.Source
	baseCtx := ctx
	var authUntil time.Time
	authFailures := 0
	for {
		ctx = baseCtx // one per-request auth layer; do not retain a growing WithValue chain
		if err := contextError(ctx); err != nil {
			return err
		}
		requestUntil := time.Now().Add(mixedHandshakeTimeout)
		if processor.authenticator != nil {
			if authUntil.IsZero() {
				authUntil = requestUntil
			}
			if authUntil.Before(requestUntil) {
				requestUntil = authUntil
			}
		}
		request, err := readMixedHTTPRequest(ctx, reader, input, requestUntil)
		if err != nil {
			return err
		}
		if request == nil || request.URL == nil {
			return ErrForwardingNotAdmitted
		}
		if processor.authenticator != nil {
			username, password, ok := H.ParseBasicAuth(request.Header.Get("Proxy-Authorization"))
			if ok && processor.authenticator.Verify(username, password) {
				ctx = auth.ContextWithUser(ctx, strings.Clone(username)) // detach from decoded "user:password" storage
				authFailures, authUntil = 0, time.Time{}
			} else {
				authFailures++
				keepAlive := mixedHTTPKeepAlive(request) && request.ContentLength == 0 &&
					authFailures < maxMixedHTTPAuthFailures && input.conn.owner.allowMixedHTTPAuthRetry()
				response := mixedHTTPResponse(request, http.StatusProxyAuthRequired)
				response.Header.Set("Proxy-Authenticate", `Basic realm="sing-box" charset="UTF-8"`)
				if !keepAlive {
					response.Header.Set("Connection", "close")
				}
				if err := withMixedHandshakeBudget(ctx, conn, requestUntil, func() error { return response.Write(conn) }); err != nil {
					return err
				}
				if keepAlive {
					continue
				}
				return ErrForwardingNotAdmitted // never return supplied credentials in an error
			}
		}
		if !time.Now().Before(requestUntil) {
			return errMixedHandshakeTimeout
		}
		if forwarded := H.SourceAddress(request); forwarded.IsValid() {
			source = forwarded // existing SDK protocol metadata, not a native peer identity
		}
		if request.Method != http.MethodConnect {
			if strings.EqualFold(request.Header.Get("Connection"), "upgrade") {
				return mixedHTTPUpgrade(ctx, handler, conn, reader, request, source)
			}
			if err := mixedHTTPProxyRequest(ctx, handler, conn, request, source); err != nil {
				return err
			}
			continue
		}
		destination := mixedHTTPDestination(request)
		buffered, err := retainMixedPrefetch(ctx, conn, reader) // original owner before reply or delegation
		if err != nil {
			return err // original input retires any partial capture; no cache handoff
		}
		response := fmt.Sprintf("HTTP/%d.%d 200 Connection established\r\n\r\n", request.ProtoMajor, request.ProtoMinor)
		err = withMixedHandshakeBudget(ctx, conn, requestUntil, func() error {
			n, err := io.WriteString(conn, response)
			if err != nil {
				return err
			}
			if n != len(response) {
				return io.ErrShortWrite
			}
			return nil
		})
		if err != nil {
			return err
		}
		handler.NewConnectionEx(ctx, buffered, source, destination, callback)
		return nil
	}
}

func mixedHTTPDestination(request *http.Request) M.Socksaddr {
	destination := M.ParseSocksaddrHostPortStr(request.URL.Hostname(), request.URL.Port()).Unwrap()
	if destination.Port == 0 {
		switch request.URL.Scheme {
		case "https", "wss":
			destination.Port = 443
		default:
			destination.Port = 80
		}
	}
	return destination
}

func mixedHTTPKeepAlive(request *http.Request) bool {
	return !(request.ProtoMajor == 1 && request.ProtoMinor == 0) && strings.EqualFold(strings.TrimSpace(request.Header.Get("Proxy-Connection")), "keep-alive")
}

// These are the pinned SDK algorithms, adapted only to our one-request loop.
// Pipe acquisition/IO/Close/handler/callback are retained by the exact input.
// Transport/opaque SDK task retirement remains a separate gate.
func mixedHTTPUpgrade(ctx context.Context, handler N.TCPConnectionHandlerEx, conn net.Conn, reader *bufio.Reader, request *http.Request, source M.Socksaddr) (finalErr error) {
	body, err := retainMixedHTTPBody(ctx, request.Body, request.ContentLength, false)
	if err != nil {
		return err
	}
	request.Body = body
	defer func() {
		if body != nil {
			finalErr = errors.Join(finalErr, body.Close())
		}
	}()
	owner, err := openMixedHTTPPipe(ctx, handler, source, mixedHTTPDestination(request), nil)
	if err != nil {
		return err
	}
	defer owner.cancel()
	server := owner.ends[0]
	if err := request.Write(server); err != nil {
		return err
	}
	if reader.Buffered() > 0 {
		if _, err := io.CopyN(server, reader, int64(reader.Buffered())); err != nil {
			return err
		}
	}
	return SBufio.CopyConn(ctx, conn, server)
}

func mixedHTTPProxyRequest(ctx context.Context, handler N.TCPConnectionHandlerEx, conn net.Conn, request *http.Request, source M.Socksaddr) (finalErr error) {
	body, err := retainMixedHTTPBody(ctx, request.Body, request.ContentLength, false)
	if err != nil {
		return err
	}
	request.Body = body
	defer func() {
		if body != nil {
			finalErr = errors.Join(finalErr, body.Close())
		}
	}()
	keepAlive := mixedHTTPKeepAlive(request)
	request.RequestURI = ""
	removeMixedHTTPHopHeaders(request.Header)
	removeMixedHTTPHostPort(request)
	if host := request.Header.Get("Host"); host != "" && host != request.URL.Host {
		request.Host = host
	}
	if request.URL.Scheme == "" || request.URL.Host == "" {
		return mixedHTTPResponse(request, http.StatusBadRequest).Write(conn)
	}
	var innerErr common.TypedValue[error]
	dials, err := newMixedHTTPDialScope(ctx, handler, source, innerErr.Store)
	if err != nil {
		return err
	}
	defer func() { finalErr = errors.Join(finalErr, dials.Close()) }()
	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression:     true,
			DialContext:            dials.dial,
			MaxResponseHeaderBytes: maxMixedHTTPHeaderBytes,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	requestCtx, cancel := dials.ctx, dials.cancel
	defer cancel() // also covers panic/Goexit; cancellation is not a Transport/task receipt
	response, err := client.Do(request.WithContext(requestCtx))
	if response != nil {
		responseBody, bodyErr := retainMixedHTTPBody(ctx, response.Body, response.ContentLength, err != nil)
		if responseBody != nil {
			response.Body = responseBody
			defer func() { finalErr = errors.Join(finalErr, responseBody.Close()) }()
		}
		if bodyErr != nil {
			return errors.Join(err, bodyErr)
		}
	}
	if err != nil {
		cancel()
		return E.Errors(innerErr.Load(), err, mixedHTTPResponse(request, http.StatusBadGateway).Write(conn))
	}
	if response == nil {
		return ErrEngineCleanupFailed
	}
	removeMixedHTTPHopHeaders(response.Header)
	if keepAlive {
		response.Header.Set("Proxy-Connection", "keep-alive")
		response.Header.Set("Connection", "keep-alive")
		response.Header.Set("Keep-Alive", "timeout=4")
	}
	response.Close = !keepAlive
	if err := response.Write(conn); err != nil {
		cancel()
		return E.Errors(innerErr.Load(), err)
	}
	cancel()
	if !keepAlive {
		return conn.Close()
	}
	return nil
}

func removeMixedHTTPHopHeaders(header http.Header) {
	for _, name := range []string{"Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailers", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
	connections := header.Get("Connection")
	header.Del("Connection")
	for name := range strings.SplitSeq(connections, ",") {
		header.Del(strings.TrimSpace(name))
	}
}

func removeMixedHTTPHostPort(request *http.Request) {
	host := request.Host
	if host == "" {
		host = request.URL.Host
	}
	if parsed, port, err := net.SplitHostPort(host); err == nil && port == "80" {
		if M.ParseAddr(parsed).Is6() {
			parsed = "[" + parsed + "]"
		}
		host = parsed
	}
	request.Host, request.URL.Host = host, host
}

func mixedHTTPResponse(request *http.Request, status int) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Proto: request.Proto, ProtoMajor: request.ProtoMajor, ProtoMinor: request.ProtoMinor, Header: make(http.Header)}
}
