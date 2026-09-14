//go:build live

// Минимальный SOCKS5-клиент (RFC 1928: no-auth, CONNECT, домены + IPv4)
// без внешних зависимостей. IPv6 не поддержан намеренно: матричный трафик
// идёт через туннель с prefer_ipv4.
package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// socksDial — коннектор через локальный SOCKS5.
func socksDial(proxyAddr string) func(network, addr string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		if network != "tcp" {
			return nil, fmt.Errorf("socks: unsupported network %q", network)
		}
		var d net.Dialer
		conn, err := d.Dial("tcp", proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("socks: dial proxy: %w", err)
		}
		if err := socksHandshake(conn, addr); err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
}

func socksHandshake(conn net.Conn, targetAddr string) error {
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer conn.SetDeadline(time.Time{})

	// 1. Greeting: VER=5, 1 method, NO-AUTH(0).
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return fmt.Errorf("socks: greeting write: %w", err)
	}
	resp := make([]byte, 2)
	if _, err := readFull(conn, resp); err != nil {
		return fmt.Errorf("socks: greeting read: %w", err)
	}
	if resp[0] != 5 || resp[1] != 0 {
		return fmt.Errorf("socks: proxy rejected method ver=%d method=%d", resp[0], resp[1])
	}

	// 2. CONNECT: ATYP=domain (большинство целей — домены; IP тоже поддержим).
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return fmt.Errorf("socks: target addr: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("socks: bad port %q", portStr)
	}
	req := []byte{5, 1, 0}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append(req, 1)
			req = append(req, v4...)
		} else {
			req = append(req, 4)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return errors.New("socks: hostname too long")
		}
		req = append(req, 3, byte(len(host)))
		req = append(req, host...)
	}
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], uint16(port))
	req = append(req, portBytes[:]...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks: connect write: %w", err)
	}

	// 3. Ответ: VER REP RSV ATYP BND.ADDR BND.PORT.
	head := make([]byte, 4)
	if _, err := readFull(conn, head); err != nil {
		return fmt.Errorf("socks: reply read: %w", err)
	}
	if head[0] != 5 {
		return fmt.Errorf("socks: bad reply ver %d", head[0])
	}
	if head[1] != 0 {
		return fmt.Errorf("socks: CONNECT failed code=%d (%s)", head[1], socksErrText(head[1]))
	}
	var addrLen int
	switch head[3] {
	case 1:
		addrLen = 4
	case 4:
		addrLen = 16
	case 3:
		l := make([]byte, 1)
		if _, err := readFull(conn, l); err != nil {
			return fmt.Errorf("socks: reply domain len: %w", err)
		}
		addrLen = int(l[0])
	default:
		return fmt.Errorf("socks: bad ATYP %d", head[3])
	}
	rest := make([]byte, addrLen+2)
	if _, err := readFull(conn, rest); err != nil {
		return fmt.Errorf("socks: reply addr: %w", err)
	}
	return nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func socksErrText(code byte) string {
	switch code {
	case 1:
		return "general failure"
	case 2:
		return "connection not allowed"
	case 3:
		return "network unreachable"
	case 4:
		return "host unreachable"
	case 5:
		return "connection refused"
	case 6:
		return "TTL expired"
	case 7:
		return "command not supported"
	case 8:
		return "address type not supported"
	}
	return "unknown"
}

// socksContextDialer — обёртка для http.Transport DialContext.
func socksContextDialer(proxyAddr string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	inner := socksDial(proxyAddr)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		type result struct {
			conn net.Conn
			err  error
		}
		ch := make(chan result, 1)
		go func() {
			c, e := inner(network, addr)
			ch <- result{c, e}
		}()
		select {
		case <-ctx.Done():
			go func() {
				if r := <-ch; r.conn != nil {
					r.conn.Close()
				}
			}()
			return nil, ctx.Err()
		case r := <-ch:
			return r.conn, r.err
		}
	}
}
