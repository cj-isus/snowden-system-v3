// Package packetaddr owns bounded address facts without retaining mutable
// net.Addr implementations or invoking their methods after capture.
package packetaddr

import (
	"errors"
	"net"
	"strings"
	"unicode"
)

const (
	maxNetwork = 64
	maxText    = 2048
	maxUnix    = 4096
	udpKind    = 1
	tcpKind    = 2
	ipKind     = 3
	unixKind   = 4
	opaqueKind = 5
)

var ErrInvalid = errors.New("invalid or oversized packet address metadata")

// Snapshot is an immutable value. Addr returns a fresh standard address or a
// frozen opaque value; a caller cannot mutate the stored IP or invoke a provider.
// It is an observed socket fact, not a path/protection/native retirement proof.
type Snapshot struct {
	kind    uint8
	ip      [16]byte
	ipSize  uint8
	port    int
	zone    string
	network string
	text    string
}

func clean(value string, maximum int) bool {
	return len(value) <= maximum && strings.IndexFunc(value, unicode.IsControl) < 0
}

func captureIP(kind uint8, ip net.IP, port int, zone string) (Snapshot, error) {
	if len(ip) != 0 && len(ip) != net.IPv4len && len(ip) != net.IPv6len || port < 0 || port > 65535 || !clean(zone, maxText) {
		return Snapshot{}, ErrInvalid
	}
	snapshot := Snapshot{kind: kind, ipSize: uint8(len(ip)), port: port, zone: strings.Clone(zone)}
	copy(snapshot.ip[:], ip)
	return snapshot, nil
}

// Capture may call an opaque provider's Network/String methods. Its caller must
// retain the original metadata operation through return/abnormal completion.
// Limits precede our clones, not arbitrary allocations inside those callbacks.
func Capture(address net.Addr) (Snapshot, error) {
	switch value := address.(type) {
	case *net.UDPAddr:
		if value != nil {
			return captureIP(udpKind, value.IP, value.Port, value.Zone)
		}
	case *net.TCPAddr:
		if value != nil {
			return captureIP(tcpKind, value.IP, value.Port, value.Zone)
		}
	case *net.IPAddr:
		if value != nil {
			return captureIP(ipKind, value.IP, 0, value.Zone)
		}
	case *net.UnixAddr:
		if value != nil && value.Net != "" && clean(value.Net, maxNetwork) && clean(value.Name, maxUnix) {
			return Snapshot{kind: unixKind, network: strings.Clone(value.Net), text: strings.Clone(value.Name)}, nil
		}
	default:
		if address != nil {
			network := address.Network()
			if network == "" || !clean(network, maxNetwork) {
				return Snapshot{}, ErrInvalid
			}
			text := address.String()
			if clean(text, maxText) {
				return Snapshot{kind: opaqueKind, network: strings.Clone(network), text: strings.Clone(text)}, nil
			}
		}
	}
	return Snapshot{}, ErrInvalid
}

func (s Snapshot) Valid() bool { return s.kind != 0 }

func (s Snapshot) Addr() net.Addr {
	var ip net.IP
	if s.ipSize != 0 {
		ip = make(net.IP, int(s.ipSize))
		copy(ip, s.ip[:s.ipSize])
	}
	switch s.kind {
	case udpKind:
		return &net.UDPAddr{IP: ip, Port: s.port, Zone: s.zone}
	case tcpKind:
		return &net.TCPAddr{IP: ip, Port: s.port, Zone: s.zone}
	case ipKind:
		return &net.IPAddr{IP: ip, Zone: s.zone}
	case unixKind:
		return &net.UnixAddr{Name: s.text, Net: s.network}
	case opaqueKind:
		return opaqueAddr{network: s.network, text: s.text}
	default:
		return nil
	}
}

type opaqueAddr struct{ network, text string }

func (a opaqueAddr) Network() string { return a.network }
func (a opaqueAddr) String() string  { return a.text }
