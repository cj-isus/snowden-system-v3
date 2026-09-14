package core

import (
	"encoding/json"
	"net"
	"os"
)

// Тонкие обёртки над stdlib для читаемости тестов.

func netListen(network, addr string) (net.Listener, error) {
	return net.Listen(network, addr)
}

type netTCPAddr = net.TCPAddr

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
