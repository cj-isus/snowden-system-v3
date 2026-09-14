//go:build windows

package core

import (
	"net"
	"testing"
)

func TestLoopbackProxyPort(t *testing.T) {
	cases := []struct {
		name   string
		server string
		port   int
		ok     bool
	}{
		{"наш формат socks", "socks=127.0.0.1:1080", 1080, true},
		{"без префикса socks", "127.0.0.1:1080", 1080, true},
		{"другой порт", "socks=127.0.0.1:9999", 9999, true},
		{"PAC не трогаем", "http://server/proxy.pac", 0, false},
		{"внешний proxy не трогаем", "192.168.1.1:3128", 0, false},
		{"комбинированный формат не трогаем", "http=127.0.0.1:8080;https=127.0.0.1:8080", 0, false},
		{"мусор не трогаем", "garbage", 0, false},
		{"порт за границей", "socks=127.0.0.1:70000", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port, ok := LoopbackProxyPort(tc.server)
			if ok != tc.ok || port != tc.port {
				t.Fatalf("LoopbackProxyPort(%q) = %d, %v; want %d, %v", tc.server, port, ok, tc.port, tc.ok)
			}
		})
	}
}

func TestLoopbackAlive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	if !LoopbackAlive(port) {
		t.Fatalf("порт с листенером должен считаться живым")
	}
	// Свободный порт: берём ещё один listener, закрываем — порт почти наверняка свободен.
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen2: %v", err)
	}
	freePort := ln2.Addr().(*net.TCPAddr).Port
	ln2.Close()
	if LoopbackAlive(freePort) {
		t.Fatalf("порт без листенера должен считаться мёртвым")
	}
}
