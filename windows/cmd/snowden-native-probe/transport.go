package main

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/netprobe"
)

func executeNative(ctx context.Context, request netprobe.NativeRequest) netprobe.NativeReply {
	reply := netprobe.NativeReply{Schema: netprobe.NativeProtocol, Nonce: request.Nonce, Results: []netprobe.NativeResult{}}
	for index, network := range request.Networks {
		for source, server := range netprobe.NativeEndpoints(network) {
			if ctx.Err() != nil {
				return reply
			}
			deadline, ok := ctx.Deadline()
			if !ok {
				return reply
			}
			left := 2*(len(request.Networks)-index) - source
			budget := min(2*time.Second, time.Until(deadline)/time.Duration(left))
			if budget <= 0 {
				return reply
			}
			attempt, cancel := context.WithTimeout(ctx, budget)
			result := nativeDNS(attempt, network, server, request.QueryIDs[2*index+source])
			cancel()
			reply.Results = append(reply.Results, result)
			if result.ErrorKind == "" {
				break
			}
		}
	}
	reply.Complete = ctx.Err() == nil
	return reply
}

func nativeError(ctx context.Context, err error, stage string) string {
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	if stage == "write" {
		return "write_incomplete"
	}
	return stage + "_unavailable"
}

func nativeDNS(ctx context.Context, network, server string, id uint16) (result netprobe.NativeResult) {
	started := time.Now()
	result = netprobe.NativeResult{Network: network, ServerIP: server, QueryID: id, Stage: "dial", Packet: []byte{}, ErrorKind: "dial_unavailable"}
	defer func() { result.ElapsedMicros = time.Since(started).Microseconds() }()
	query, err := netprobe.EncodeDNSQuery(id)
	if err != nil {
		return result
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(server, "53"))
	if err != nil {
		result.ErrorKind = nativeError(ctx, err, result.Stage)
		return result
	}
	defer connection.Close()
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	deadline, ok := ctx.Deadline()
	if !ok || connection.SetDeadline(deadline) != nil {
		return result
	}
	result.Stage = "write"
	request := query
	if strings.HasPrefix(network, "tcp") {
		request = make([]byte, 2+len(query))
		binary.BigEndian.PutUint16(request[:2], uint16(len(query)))
		copy(request[2:], query)
	}
	if n, err := connection.Write(request); err != nil || n != len(request) {
		if err == nil {
			err = io.ErrShortWrite
		}
		result.ErrorKind = nativeError(ctx, err, result.Stage)
		return result
	}
	result.Stage = "read"
	var packet []byte
	if strings.HasPrefix(network, "tcp") {
		var prefix [2]byte
		if _, err := io.ReadFull(connection, prefix[:]); err != nil {
			result.ErrorKind = nativeError(ctx, err, result.Stage)
			return result
		}
		size := int(binary.BigEndian.Uint16(prefix[:]))
		if size < 12 || size > netprobe.MaxDNSMessage {
			result.ErrorKind = "message_limit"
			return result
		}
		packet = make([]byte, size)
		if _, err := io.ReadFull(connection, packet); err != nil {
			result.ErrorKind = nativeError(ctx, err, result.Stage)
			return result
		}
	} else {
		buffer := make([]byte, netprobe.MaxDNSMessage+1)
		n, err := connection.Read(buffer)
		if err != nil {
			result.ErrorKind = nativeError(ctx, err, result.Stage)
			return result
		}
		if n > netprobe.MaxDNSMessage {
			result.ErrorKind = "message_limit"
			return result
		}
		packet = buffer[:n]
	}
	result.Stage, result.Packet = "response", packet
	if ctx.Err() != nil {
		result.Stage, result.Packet, result.ErrorKind = "read", []byte{}, nativeError(ctx, ctx.Err(), "read")
		return result
	}
	result.ErrorKind = netprobe.NativeDNSResponseKind(packet, id)
	return result
}
