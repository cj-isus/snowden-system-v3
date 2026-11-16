package main

// Production optional endpoint observation. QUIC invariants/version negotiation
// only: no TLS/HY2/obfs authentication, traffic admission or DPI classification.

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"net"
	"strconv"
	"time"
)

const (
	udpObservationAttempts     = 2
	udpObservationRequestBytes = 1200
	udpObservationMaxReply     = 4096
	// A reserved version elicits version negotiation where supported. This is
	// deliberately not a QUIC v1 Initial or an authenticated handshake.
	udpObservationVersion uint32 = 0x0a0a0a0a
)

func diagnosticUDPAddress(host string, port uint16) string {
	return net.JoinHostPort(host, strconv.Itoa(int(port)))
}

// Validate the complete bounded version-list reply and BOTH reflected random
// connection IDs. Connected UDP additionally restricts the source tuple.
// Version negotiation remains unauthenticated and is not server identity proof.
func matchingVersionReply(reply, request []byte) bool {
	if len(request) != udpObservationRequestBytes || len(reply) < 27 || len(reply) > udpObservationMaxReply ||
		reply[0]&0x80 == 0 || binary.BigEndian.Uint32(reply[1:5]) != 0 || reply[5] != 8 ||
		!bytes.Equal(reply[6:14], request[15:23]) || reply[14] != 8 || !bytes.Equal(reply[15:23], request[6:14]) || (len(reply)-23)%4 != 0 {
		return false
	}
	for i := 23; i < len(reply); i += 4 {
		version := binary.BigEndian.Uint32(reply[i : i+4])
		if version == 0 || version == udpObservationVersion {
			return false
		}
	}
	return true
}

func observeQUICVersion(task *manualDiagnosticTask, address string) (*bool, string, []ProbeStepView) {
	conn, err := task.dialConnection("udp", address)
	if err != nil {
		return diagnosticUnavailable("UDP-запрос не выполнен; причина доступности пути не установлена.")
	}
	defer conn.Close()
	for attempt := 0; attempt < udpObservationAttempts; attempt++ {
		if task.currentError() != nil {
			return diagnosticUnavailable("Исходный запрос или путь отозван; результат не установлен.")
		}
		request := make([]byte, udpObservationRequestBytes)
		if _, err := rand.Read(request); err != nil {
			return diagnosticUnavailable("Не получены случайные идентификаторы; запрос не отправлен.")
		}
		request[0] = 0xc0
		binary.BigEndian.PutUint32(request[1:5], udpObservationVersion)
		request[5], request[14] = 8, 8
		deadline := time.Now().Add(3 * time.Second)
		if limit, ok := task.ctx.Deadline(); ok && limit.Before(deadline) {
			deadline = limit
		}
		if conn.SetDeadline(deadline) != nil {
			return diagnosticUnavailable("Не установлен deadline исходного UDP-сокета.")
		}
		if n, err := conn.Write(request); err != nil || n != len(request) {
			continue
		}
		reply := make([]byte, udpObservationMaxReply+1)
		n, err := conn.Read(reply)
		if err == nil && task.currentError() == nil && matchingVersionReply(reply[:n], request) {
			ok := true
			detail := "Получен связанный по двум случайным CID ответ QUIC Version Negotiation от " + address + ". Маршрут ОС мог проходить через другой VPN; TLS/HY2/obfs и protected forwarding не проверены."
			return &ok, detail, []ProbeStepView{{Name: "UDP / QUIC Version Negotiation", Status: "pass", Detail: detail}}
		}
	}
	return diagnosticUnavailable("Подходящий ответ не получен в пределах двух запросов. Это не устанавливает фильтрацию UDP, отказ HY2 или причину тишины.")
}
