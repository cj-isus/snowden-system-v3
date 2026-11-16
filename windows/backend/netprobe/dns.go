package netprobe

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	DNSProbeHost       = "example.com"
	MaxDNSMessage      = 4096
	MaxDNSQuery        = 512
	maxDNSProbeRecords = 32
	DNSMediaType       = "application/dns-message"
)

var ErrDNSResponse = errors.New("invalid bounded correlated DNS response")
var ErrDNSAnswerMissing = errors.New("expected recursive DNS A answer unavailable")
var ErrDNSTruncated = errors.New("DNS response truncated")

func dnsProbeQuestion() (dnsmessage.Question, error) {
	return dnsHostQuestion(DNSProbeHost)
}

// CanonicalDNSHost accepts only a bounded ASCII hostname. IP literals belong
// to the caller's separate address path, not to a recursive DNS question.
func CanonicalDNSHost(host string) (string, error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if len(host) == 0 || len(host) > 253 || net.ParseIP(host) != nil {
		return "", ErrDNSResponse
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrDNSResponse
		}
		for _, ch := range label {
			if ch != '-' && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
				return "", ErrDNSResponse
			}
		}
	}
	return host, nil
}

func dnsHostQuestion(host string) (dnsmessage.Question, error) {
	host, err := CanonicalDNSHost(host)
	if err != nil {
		return dnsmessage.Question{}, err
	}
	name, err := dnsmessage.NewName(host + ".")
	return dnsmessage.Question{Name: name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}, err
}

func EncodeDNSQuery(id uint16) ([]byte, error) {
	return EncodeDNSHostQuery(DNSProbeHost, id)
}

func EncodeDNSHostQuery(host string, id uint16) ([]byte, error) {
	question, err := dnsHostQuestion(host)
	if err != nil {
		return nil, err
	}
	// The caller owns correlation: DoH uses0; native packet probes choose an ID.
	message := dnsmessage.Message{Header: dnsmessage.Header{ID: id, RecursionDesired: true}, Questions: []dnsmessage.Question{question}}
	raw, err := message.Pack()
	if err != nil || len(raw) > MaxDNSQuery {
		return nil, ErrDNSResponse
	}
	return raw, nil
}

// Preflight only frames bounded sections. Pinned dnsmessage then validates
// compressed names and resource bodies. Its Unpack does not reject trailing
// bytes, so exact framing is checked here before allocating parsed records.
// ValidateDNSServiceFrame applies the existing bounded one-question wire frame
// gate before a service resolver's codec can allocate sections. It does not
// authenticate a server, correlate a question, or establish a usable address.
func ValidateDNSServiceFrame(raw []byte) error { return frameDNSProbe(raw) }

func frameDNSProbe(raw []byte) error {
	if len(raw) < 12 || len(raw) > MaxDNSMessage || binary.BigEndian.Uint16(raw[4:6]) != 1 || binary.BigEndian.Uint16(raw[2:4])&0x0040 != 0 {
		return ErrDNSResponse
	}
	answers, authorities, additional := int(binary.BigEndian.Uint16(raw[6:8])), int(binary.BigEndian.Uint16(raw[8:10])), int(binary.BigEndian.Uint16(raw[10:12]))
	if answers > maxDNSProbeRecords || authorities > maxDNSProbeRecords || additional > maxDNSProbeRecords {
		return ErrDNSResponse
	}
	skipName := func(offset int) (int, error) {
		for labels := 0; labels < 128 && offset < len(raw); labels++ {
			size := int(raw[offset])
			if size&0xc0 == 0xc0 {
				if offset+2 > len(raw) {
					return 0, ErrDNSResponse
				}
				return offset + 2, nil // target/loop/name validation belongs to dnsmessage
			}
			if size&0xc0 != 0 || offset+1+size > len(raw) {
				return 0, ErrDNSResponse
			}
			offset += size + 1
			if size == 0 {
				return offset, nil
			}
		}
		return 0, ErrDNSResponse
	}
	offset, err := skipName(12)
	if err != nil || offset+4 > len(raw) {
		return ErrDNSResponse
	}
	offset += 4
	for records := answers + authorities + additional; records > 0; records-- {
		offset, err = skipName(offset)
		if err != nil || offset+10 > len(raw) {
			return ErrDNSResponse
		}
		kind := dnsmessage.Type(binary.BigEndian.Uint16(raw[offset : offset+2]))
		size := int(binary.BigEndian.Uint16(raw[offset+8 : offset+10]))
		start, end := offset+10, offset+10+size
		if end > len(raw) {
			return ErrDNSResponse
		}
		// dnsmessage's fixed/name-only codecs do not compare their consumed
		// bytes with RDLENGTH. Enforce that boundary before it can read a later RR.
		position, names, fixed := start, 0, 0
		switch kind {
		case dnsmessage.TypeA:
			fixed = 4
		case dnsmessage.TypeAAAA:
			fixed = 16
		case dnsmessage.TypeCNAME, dnsmessage.TypeNS, dnsmessage.TypePTR:
			names = 1
		case dnsmessage.TypeSOA:
			names, fixed = 2, 20
		case dnsmessage.TypeMX:
			position, names = start+2, 1
		case dnsmessage.TypeSRV:
			position, names = start+6, 1
		default:
			offset = end // length-aware/opaque resource codecs retain their own gate
			continue
		}
		if position > end {
			return ErrDNSResponse
		}
		for ; names > 0; names-- {
			position, err = skipName(position)
			if err != nil || position > end {
				return ErrDNSResponse
			}
		}
		if position+fixed != end {
			return ErrDNSResponse
		}
		offset = end
	}
	if offset != len(raw) {
		return ErrDNSResponse
	}
	return nil
}

type DNSHostAnswer struct {
	Addresses []string
	TTL       uint32 // minimum across the followed CNAME chain and returned A set; zero is not cacheable
}

func DecodeDNSResponse(raw []byte, id uint16) ([]string, error) {
	answer, err := DecodeDNSHostResponse(raw, id, DNSProbeHost)
	return answer.Addresses, err
}

// The same bounded framing, exact question and public-IPv4 gate serve bootstrap
// and the fixed protected measurement. Neither path establishes DNSSEC or a
// trusted service identity; the actual TLS/transport admission remains separate.
func DecodeDNSHostResponse(raw []byte, id uint16, host string) (DNSHostAnswer, error) {
	if err := frameDNSProbe(raw); err != nil {
		return DNSHostAnswer{}, err
	}
	var message dnsmessage.Message
	if err := message.Unpack(raw); err != nil {
		return DNSHostAnswer{}, ErrDNSResponse
	}
	question, err := dnsHostQuestion(host)
	if err != nil || !message.Response || message.ID != id || message.OpCode != 0 || len(message.Questions) != 1 {
		return DNSHostAnswer{}, ErrDNSResponse
	}
	normalize := func(name dnsmessage.Name) string { return strings.TrimSuffix(strings.ToLower(name.String()), ".") }
	actual := message.Questions[0]
	if actual.Type != question.Type || actual.Class != question.Class || normalize(actual.Name) != normalize(question.Name) {
		return DNSHostAnswer{}, ErrDNSResponse
	}
	if message.Truncated {
		return DNSHostAnswer{}, ErrDNSTruncated
	}
	if !message.RecursionAvailable || message.RCode != dnsmessage.RCodeSuccess {
		return DNSHostAnswer{}, ErrDNSAnswerMissing
	}
	for _, record := range message.Additionals {
		if record.Header.Type == dnsmessage.TypeOPT && record.Header.TTL>>16 != 0 {
			return DNSHostAnswer{}, ErrDNSResponse // extended RCODE/unsupported EDNS version
		}
	}
	name, seen := normalize(question.Name), map[string]bool{}
	ttl := ^uint32(0)
	for hops := 0; hops <= 8; hops++ {
		if seen[name] {
			return DNSHostAnswer{}, ErrDNSResponse
		}
		seen[name] = true
		alias, addresses := "", []string{}
		unique := map[string]bool{}
		for _, record := range message.Answers {
			if record.Header.Class != dnsmessage.ClassINET || normalize(record.Header.Name) != name {
				continue
			}
			switch value := record.Body.(type) {
			case *dnsmessage.AResource:
				ip := net.IP(value.A[:])
				if !PublicIPv4(ip) {
					return DNSHostAnswer{}, ErrDNSResponse // bootstrap/probe cannot point at local/private space
				}
				ttl = min(ttl, record.Header.TTL)
				if text := ip.String(); !unique[text] {
					addresses, unique[text] = append(addresses, text), true
				}
			case *dnsmessage.CNAMEResource:
				target := normalize(value.CNAME)
				if target == "" || alias != "" && alias != target {
					return DNSHostAnswer{}, ErrDNSResponse
				}
				ttl = min(ttl, record.Header.TTL)
				alias = target
			}
		}
		if len(addresses) != 0 {
			if alias != "" {
				return DNSHostAnswer{}, ErrDNSResponse
			}
			return DNSHostAnswer{Addresses: addresses, TTL: ttl}, nil
		}
		if alias == "" {
			return DNSHostAnswer{}, ErrDNSAnswerMissing
		}
		name = alias
	}
	return DNSHostAnswer{}, ErrDNSResponse
}

// PublicIPv4 is a pure shared gate, not a full IANA special-purpose table.
func PublicIPv4(ip net.IP) bool {
	return ip != nil && ip.To4() != nil && ip.IsGlobalUnicast() && !ip.IsPrivate()
}
