package core

import "github.com/snowden-system/windows/backend/netprobe"

const (
	maxDNSProbeQuery  = netprobe.MaxDNSQuery
	dnsProbeMediaType = netprobe.DNSMediaType
)

var (
	errDNSProbeResponse      = netprobe.ErrDNSResponse
	errDNSProbeAnswerMissing = netprobe.ErrDNSAnswerMissing
	errDNSProbeTruncated     = netprobe.ErrDNSTruncated
)

func dnsProbeQuery() ([]byte, error)               { return netprobe.EncodeDNSQuery(0) }
func dnsProbeAnswers(raw []byte) ([]string, error) { return netprobe.DecodeDNSResponse(raw, 0) }
