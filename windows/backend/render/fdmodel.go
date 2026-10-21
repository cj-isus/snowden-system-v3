package render

import "github.com/snowden-system/windows/backend/metadata"

// fdmodel.go — Failure Domain Model (PLAN §4 Phase B2; plan_gpt §5/§6, шаг 4
// из §116 — ПЕРЕД каналом B: без модели доменов новый канал родится без
// семантики независимости, и C2 (candidate scoring) не на чем считать).
//
// FD(channel) = { transport_family, carrier, origin, asn, port_family, layer }
// (plan_gpt §5; protocol выделен в transport_family вместе с транспортом —
// vless+ws и vless+shadowtls это разные транспортные семейства с разной
// уязвимостью, THEORY §2.10).
//
// Правило честности (AGENTS §3.3): поля fill-in данными владельца; то, чего нет
// в данных, деривация помечает "unknown" и НЕ участвует в штрафах независимости
// (plan_gpt §6: одинаковые unknown ≠ общий домен — иначе две незаполненные
// записи выглядели бы «одинаковыми»).

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
	// Carrier: у ws канала A это Cloudflare (V2-039/041); у quic/reality/
	// shadowtls — origin-direct (мимо CDN, V2-040/044). Владелец может
	// переопределить fd.carrier (другой CDN-провайдер).
	if fd.Carrier == "" {
		switch ch.Transport {
		case "ws":
			fd.Carrier = "cloudflare"
		case "quic", "tcp-reality", "shadowtls":
			fd.Carrier = ""
		default:
			fd.Carrier = ""
		}
	}
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

// IndependenceThreshold — порог независимости plan_gpt §6: пары ниже —
// «слишком общая инфраструктура» для кандидата failover (C2).
const IndependenceThreshold = 0.35

// independenceShadows — веса §6 (внутренняя инженерная policy-модель, план:
// калибровка на данных после Phase E2).
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

// Independence — детерминированная оценка независимости двух каналов по
// пересечению failure domains: 1 − Σ штрафов, [0..1]. unknown-домены штрафа не
// дают. Это policy-модель для сортировки кандидатов (C2), не физика.
func Independence(a, b ChannelDescriptor) float64 {
	if a.ID == b.ID {
		return 0
	}
	fa, fb := a.effectiveFD(), b.effectiveFD()
	ind := 1.0
	for _, s := range independenceShadows {
		var same bool
		switch s.name {
		case "same_origin":
			same = sameFDValue(fa.Origin, fb.Origin)
		case "same_asn":
			same = sameFDValue(fa.ASN, fb.ASN)
		case "same_carrier":
			same = sameFDValue(fa.Carrier, fb.Carrier)
		case "same_transport":
			same = sameFDValue(fa.TransportFamily, fb.TransportFamily)
		case "same_port_family":
			same = sameFDValue(fa.PortFamily, fb.PortFamily)
		case "same_layer":
			same = sameFDValue(fa.Layer, fb.Layer)
		case "same_donor":
			same = sameFDValue(fa.DonorSNI, fb.DonorSNI)
		}
		if same && ind > 0 {
			ind -= s.penalty
		}
	}
	// Safety policy (V2-073): one known origin hosted in one known ASN is one
	// dominant infrastructure failure domain. The additive weights are useful
	// for explaining shared properties, but must not allow transport diversity
	// to make four profiles on one VPS look like viable independent survivors.
	// Clamp is deliberately conservative and deterministic; it is not a claim
	// about backbone correlation and does not replace the second VPS or D3.
	if sameFDValue(fa.Origin, fb.Origin) && sameFDValue(fa.ASN, fb.ASN) && ind > 0.25 {
		ind = 0.25
	}
	if ind < 0 {
		ind = 0
	}
	return ind
}

// IndependenceSummary — человекочитаемый список общих доменов (для UI/journal).
// Пустая строка = общих известных доменов нет.
func IndependenceSummary(a, b ChannelDescriptor) string {
	if a.ID == b.ID {
		return "тот же канал"
	}
	fa, fb := a.effectiveFD(), b.effectiveFD()
	var parts []string
	add := func(cond bool, what string) {
		if cond {
			parts = append(parts, what)
		}
	}
	add(sameFDValue(fa.Origin, fb.Origin), "origin")
	add(sameFDValue(fa.ASN, fb.ASN), "ASN")
	add(sameFDValue(fa.Carrier, fb.Carrier), "carrier")
	add(sameFDValue(fa.TransportFamily, fb.TransportFamily), "транспорт")
	add(sameFDValue(fa.PortFamily, fb.PortFamily), "класс порта")
	add(sameFDValue(fa.Layer, fb.Layer), "слой")
	add(sameFDValue(fa.DonorSNI, fb.DonorSNI), "донор REALITY")
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += " + " + p
	}
	return out
}
