package render

import "github.com/snowden-system/windows/backend/metadata"

// Configuration-level dependency compatibility helpers. The current graph
// separates explicit assertions, configured targets and verified signing-key
// identity. Unknowns contribute no ranking benefit; scores are never proof
// that physical infrastructure or recovery paths are independent.
// FailureDomain shares the signed wire schema; derivation/scoring remain here.
type FailureDomain = metadata.FailureDomain

func validateFD(fd FailureDomain) error { return metadata.ValidateFailureDomain(fd) }

// sameFDValue — сравнение одного домена: пустые/unknown значения с обеих сторон
// означают «неизвестно», а не «одинаково» (false — штрафа нет).
func sameFDValue(a, b string) bool {
	if a == "" || b == "" || a == "unknown" || b == "unknown" {
		return false
	}
	return a == b
}

// effectiveFD — FD канала с деривацией умолчаний. Явные поля дескриптора
// (fd.*) имеют приоритет; отсутствующие деривируются из уже валидированных
// полей (Protocol/Transport/Port/OriginServer). Деривация детерминирована и
// не делает сетевых запросов.
func (ch ChannelDescriptor) effectiveFD() FailureDomain {
	fd := ch.FD
	if fd.TransportFamily == "" {
		fd.TransportFamily = ch.Protocol + "+" + ch.Transport
	}
	// Carrier is unknown unless explicitly declared; transport does not identify a CDN.
	if fd.Origin == "" {
		fd.Origin = ch.OriginServer
	}
	// ASN НЕ деривируется: не угадываем хостера (AGENTS §3.3).
	if fd.PortFamily == "" {
		if len(ch.ServerPorts) > 0 {
			lo := ch.ServerPorts[0]
			if i := indexByte(lo, ':'); i >= 0 {
				lo = lo[:i]
			}
			if lo == "443" {
				fd.PortFamily = "443"
			} else {
				fd.PortFamily = "other"
			}
		} else if ch.Port == 443 {
			fd.PortFamily = "443"
		} else {
			fd.PortFamily = "other"
		}
	}
	if fd.Layer == "" {
		if ch.Transport == "quic" {
			fd.Layer = "udp"
		} else {
			fd.Layer = "tcp"
		}
	}
	if fd.DonorSNI == "" && (ch.Transport == "tcp-reality" || ch.Transport == "shadowtls") {
		// Общий TLS-донор (handshake-цель) — домен отказа для обоих донорных
		// транспортов: снос/блок донора ломает и reality, и shadowtls канал.
		fd.DonorSNI = ch.Hostname
	}
	return fd
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// IndependenceThreshold is a legacy advisory ranking cutoff. It cannot prove
// that either channel is safe/unsafe, independent or currently available.
const IndependenceThreshold = 0.35

// Historical weights retained for compatibility with old source consumers.
// Current ranking uses dependencyRoles; it never starts at 1 for unknown data.
var independenceShadows = []struct {
	name    string
	penalty float64
}{
	{"same_origin", 0.30},  // plan_gpt: same VPS = -0.30
	{"same_asn", 0.20},     // same ASN = -0.20
	{"same_carrier", 0.20}, // same carrier (CDN) = -0.20
	{"same_transport", 0.20},
	{"same_port_family", 0.05},
	{"same_layer", 0.05},
	{"same_donor", 0.10}, // донор REALITY — дополнительный общий домен (не из §6)
}

// Independence retains its API name but returns configured dependency
// differences with unknowns contributing zero. It is not independence proof.
func Independence(a, b ChannelDescriptor, profiles ...ProfileProvenance) float64 {
	if a.ID == b.ID {
		return 0
	}
	return CompareDependencies(a, b, profiles...).RankHint
}

// IndependenceSummary includes coverage, shared dependencies and unknown roles.
func IndependenceSummary(a, b ChannelDescriptor, profiles ...ProfileProvenance) string {
	if a.ID == b.ID {
		return "тот же канал"
	}
	return CompareDependencies(a, b, profiles...).Summary()
}
