package metadata

import (
	"fmt"
	"maps"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Channel — дескриптор канала во встроенном формате render (schema 1/2:
// tcp-reality несёт uuid_ref/reality_*_ref). Только ссылки на секреты.
type Channel struct {
	ID                   string   `json:"id"`
	Protocol             string   `json:"protocol"`
	Transport            string   `json:"transport"`
	Hostname             string   `json:"hostname"`
	Port                 uint16   `json:"port"`
	OriginServer         string   `json:"origin_server"`
	ExpectedEgress       string   `json:"expected_egress"`
	CredentialRefs       []string `json:"credential_refs"`
	ValidationStatus     string   `json:"validation_status"`
	ValidatedAt          *string  `json:"validated_at,omitempty"`
	Enabled              bool     `json:"enabled"`
	UUIDRef              string   `json:"uuid_ref,omitempty"`
	RealityPublicKeyRef  string   `json:"reality_public_key_ref,omitempty"`
	RealityShortIDRef    string   `json:"reality_short_id_ref,omitempty"`
	ShadowTLSPasswordRef string   `json:"shadowtls_password_ref,omitempty"`
	// SecretIDs binds declared roles to exact vault instances. A nonempty
	// map must be complete; omitted means legacy unique-kind resolution.
	SecretIDs map[string]string `json:"secret_ids,omitempty"`
	// Optional runtime tuning is authenticated with the rest of the channel.
	// ProbePrimaryHost binds the first HTTPS target; independence needs separate proof.
	ProbeTargets     []string `json:"probe_targets,omitempty"`
	ProbePrimaryHost string   `json:"probe_primary_host,omitempty"`
	// HY2 port ranges/interval and declared bandwidth (zero preserves BBR).
	ServerPorts []string `json:"server_ports,omitempty"`
	HopInterval string   `json:"hop_interval,omitempty"`
	UpMbps      int      `json:"up_mbps,omitempty"`
	DownMbps    int      `json:"down_mbps,omitempty"`
	// nil preserves transport defaults; false is an explicit signed override.
	// WS/REALITY render h2mux only on true; ShadowTLS has no mux renderer.
	Multiplex     *bool  `json:"multiplex,omitempty"`
	DiagnosticURL string `json:"diagnostic_url,omitempty"`
	// omitzero (Go 1.26 toolchain) preserves canonical bytes of legacy profiles.
	FD FailureDomain `json:"fd,omitempty,omitzero"`
}

// Clone isolates runtime edits from the signed object, including every mutable field.
func (ch Channel) Clone() Channel {
	ch.CredentialRefs = slices.Clone(ch.CredentialRefs)
	ch.SecretIDs = maps.Clone(ch.SecretIDs)
	ch.ProbeTargets = slices.Clone(ch.ProbeTargets)
	ch.ServerPorts = slices.Clone(ch.ServerPorts)
	if ch.ValidatedAt != nil {
		value := *ch.ValidatedAt
		ch.ValidatedAt = &value
	}
	if ch.Multiplex != nil {
		value := *ch.Multiplex
		ch.Multiplex = &value
	}
	return ch
}

func validateCredentialRefs(ch Channel) error {
	if len(ch.CredentialRefs) == 0 {
		return fmt.Errorf("channel[%s]: no credential refs", ch.ID)
	}
	for _, ref := range ch.CredentialRefs {
		if strings.ContainsAny(ref, "\r\n") || secretShaped(ref) {
			return fmt.Errorf("channel[%s]: invalid credential ref", ch.ID)
		}
	}
	return ValidateSecretIDs(ch.CredentialRefs, ch.SecretIDs)
}

// FailureDomain — наблюдаемые домены отказа канала (данные, не гипотезы).
type FailureDomain struct {
	// TransportFamily — протокол+транспорт ("vless+ws", "vless+tcp-reality",
	// "vless+shadowtls", "hysteria2+quic"). Деривируется из Protocol/Transport;
	// явное переопределение fd.transport_family допустимо.
	TransportFamily string `json:"transport_family,omitempty"`
	// Carrier — инфраструктурный носитель front-пути: "cloudflare", "" (origin
	// direct) или точное имя CDN. Для tcp-reality carrier = "" (TLS к донору,
	// не к CDN), донор фиксируется в fd.donor_sni.
	Carrier string `json:"carrier,omitempty"`
	// Origin — IP/хост origin-сервера (по умолчанию деривируется из
	// OriginServer; задать вручную имеет смысл только для маппинга за LB).
	Origin string `json:"origin,omitempty"`
	// ASN — номер автономной системы origin-хостинга (уточняет владелец;
	// "AS00000"-форма). Пусто = неизвестен и в независимости не участвует.
	ASN string `json:"asn,omitempty"`
	// PortFamily — класс порта front-пути: "443" | "other". HY2 с hopping
	// через 443 = "443" (server_ports "443:443").
	PortFamily string `json:"port_family,omitempty"`
	// Layer — класс сетевого слоя front-пути: "udp" (QUIC/HY2) | "tcp" (все
	// vless-транспорты, включая ShadowTLS v3).
	Layer string `json:"layer,omitempty"`
	// DonorSNI — SNI-донор для tcp-reality (свой домен отказа: снос донора
	// ломает канал независимо от VPS).
	DonorSNI string `json:"donor_sni,omitempty"`
}

// ValidateFailureDomain — форма явных FD-полей (schema 2, все optional; «current не мешает
// schema 3» — при переезде структура уже с понятиями plan_gpt §5). Точный
// перечень допустимых значений зашит здесь же: словарь домена — часть контракта,
// опечатки ловим при парсинге, а не при первом failover.
func ValidateFailureDomain(fd FailureDomain) error {
	// ASN — либо пусто, либо "AS" + 1..10 цифр.
	if fd.ASN != "" {
		ok := len(fd.ASN) >= 3 && len(fd.ASN) <= 12 && fd.ASN[0] == 'A' && fd.ASN[1] == 'S'
		if ok {
			for _, c := range fd.ASN[2:] {
				if c < '0' || c > '9' {
					ok = false
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("fd.asn: ожидалась форма AS<цифры> (например AS12345), получено %q", fd.ASN)
		}
	}
	switch fd.PortFamily {
	case "", "443", "other":
	default:
		return fmt.Errorf("fd.port_family: допустимы \"443\"|\"other\"|пусто, получено %q", fd.PortFamily)
	}
	switch fd.Layer {
	case "", "tcp", "udp":
	default:
		return fmt.Errorf("fd.layer: допустимы \"tcp\"|\"udp\"|пусто, получено %q", fd.Layer)
	}
	return nil
}

// ValidateChannel — правила одного дескриптора (общие для встроенных и
// envelope-каналов). Ошибка предваряется префиксом descriptors[%s].
func ValidateChannel(ch Channel) error {
	if ch.ID == "" {
		return fmt.Errorf("channel: empty id")
	}
	if err := validateCredentialRefs(ch); err != nil {
		return fmt.Errorf("descriptors[%s]: %w", ch.ID, err)
	}
	if ch.Protocol != "vless" && ch.Protocol != "hysteria2" {
		return fmt.Errorf("descriptors[%s]: unsupported protocol %q", ch.ID, ch.Protocol)
	}
	switch {
	case ch.Protocol == "vless" && ch.Transport == "ws":
	case ch.Protocol == "hysteria2" && ch.Transport == "quic":
	case ch.Protocol == "vless" && ch.Transport == "shadowtls":
		// V2-051: VLESS over ShadowTLS v3. Ровно две различные роли:
		// uuid_ref (внутренний vless-инбаунд) + shadowtls_password_ref.
		declared := map[string]bool{}
		for _, r := range ch.CredentialRefs {
			declared[r] = true
		}
		if ch.UUIDRef == "" || ch.ShadowTLSPasswordRef == "" ||
			!declared[ch.UUIDRef] || !declared[ch.ShadowTLSPasswordRef] {
			return fmt.Errorf("descriptors[%s]: shadowtls requires uuid_ref and shadowtls_password_ref, both listed in credential_refs", ch.ID)
		}
		if len(ch.CredentialRefs) != 2 || ch.UUIDRef == ch.ShadowTLSPasswordRef {
			return fmt.Errorf("descriptors[%s]: credential_refs must be exactly the two distinct role refs", ch.ID)
		}
		if ch.Port != 443 && (ch.Port < 1024) {
			return fmt.Errorf("descriptors[%s]: shadowtls port %d выглядит некорректно", ch.ID, ch.Port)
		}
	case ch.Protocol == "vless" && ch.Transport == "tcp-reality":
		// Роли ссылок заданы явно и все три присутствуют в списке,
		// ровно по одной (без дублей и лишних).
		declared := map[string]bool{}
		for _, r := range ch.CredentialRefs {
			declared[r] = true
		}
		roleRefs := []string{ch.UUIDRef, ch.RealityPublicKeyRef, ch.RealityShortIDRef}
		for _, r := range roleRefs {
			if r == "" || !declared[r] {
				return fmt.Errorf("descriptors[%s]: tcp-reality requires uuid_ref, reality_public_key_ref and reality_short_id_ref, all listed in credential_refs", ch.ID)
			}
		}
		if len(ch.CredentialRefs) != 3 || ch.UUIDRef == ch.RealityPublicKeyRef ||
			ch.UUIDRef == ch.RealityShortIDRef || ch.RealityPublicKeyRef == ch.RealityShortIDRef {
			return fmt.Errorf("descriptors[%s]: credential_refs must be exactly the three distinct role refs", ch.ID)
		}
		if ch.ShadowTLSPasswordRef != "" {
			return fmt.Errorf("descriptors[%s]: shadowtls_password_ref is not valid for tcp-reality", ch.ID)
		}
	default:
		return fmt.Errorf("descriptors[%s]: transport %q does not match protocol %q", ch.ID, ch.Transport, ch.Protocol)
	}
	if net.ParseIP(strings.TrimSpace(ch.ExpectedEgress)) == nil {
		return fmt.Errorf("descriptors[%s]: expected_egress is not an IP", ch.ID)
	}
	if ch.Hostname == "" || ch.Port == 0 || ch.OriginServer == "" {
		return fmt.Errorf("descriptors[%s]: incomplete endpoint", ch.ID)
	}
	if err := ValidateFailureDomain(ch.FD); err != nil {
		return fmt.Errorf("descriptors[%s]: %w", ch.ID, err)
	}
	if len(ch.CredentialRefs) == 0 {
		return fmt.Errorf("descriptors[%s]: no credential refs", ch.ID)
	}
	switch ch.ValidationStatus {
	case "planned", "configured", "locally-tested", "live-verified", "degraded", "blocked", "retired":
	default:
		return fmt.Errorf("descriptors[%s]: unknown validation_status %q", ch.ID, ch.ValidationStatus)
	}
	return validateChannelExtras(ch)
}

// validateChannelExtras — правила V2-037 (probe-цели/hopping/brutal/mux).
func validateChannelExtras(ch Channel) error {
	if len(ch.ProbeTargets) > 0 {
		if len(ch.ProbeTargets) < 2 {
			return fmt.Errorf("descriptors[%s]: probe_targets requires at least 2 entries (consistency check)", ch.ID)
		}
		if len(ch.ProbeTargets) > 3 {
			return fmt.Errorf("descriptors[%s]: probe_targets allows at most 3 entries", ch.ID)
		}
		for _, target := range ch.ProbeTargets {
			u, err := url.Parse(target)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				return fmt.Errorf("descriptors[%s]: probe target %q must be a valid https URL", ch.ID, target)
			}
		}
		primary, err := url.Parse(ch.ProbeTargets[0])
		if err != nil {
			return fmt.Errorf("descriptors[%s]: primary probe target is invalid: %w", ch.ID, err)
		}
		primaryHost := strings.ToLower(strings.TrimSuffix(primary.Hostname(), "."))
		expectedHost := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(ch.ProbePrimaryHost), "."))
		if expectedHost == "" {
			expectedHost = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(ch.Hostname), "."))
		}
		if expectedHost == "" || primaryHost == "" || primaryHost != expectedHost {
			return fmt.Errorf("descriptors[%s]: first probe target must be self-hosted host %q (got %q); set probe_primary_host explicitly for donor-SNI channels", ch.ID, expectedHost, primaryHost)
		}
	}
	if len(ch.ServerPorts) > 0 {
		if ch.Protocol != "hysteria2" {
			return fmt.Errorf("descriptors[%s]: server_ports is hysteria2-only (port hopping)", ch.ID)
		}
		for _, sp := range ch.ServerPorts {
			parts := strings.SplitN(sp, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("descriptors[%s]: server_ports %q must be lo:hi", ch.ID, sp)
			}
			lo, err1 := strconv.Atoi(parts[0])
			hi, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || lo < 1 || hi < 1 || lo > hi || hi > 65535 {
				return fmt.Errorf("descriptors[%s]: server_ports %q is not a valid range", ch.ID, sp)
			}
		}
	}
	if ch.HopInterval != "" {
		if ch.Protocol != "hysteria2" {
			return fmt.Errorf("descriptors[%s]: hop_interval is hysteria2-only", ch.ID)
		}
		d, err := time.ParseDuration(ch.HopInterval)
		if err != nil || d < 5*time.Second || d > 10*time.Minute {
			return fmt.Errorf("descriptors[%s]: hop_interval %q must be 5s..10m", ch.ID, ch.HopInterval)
		}
	}
	if (ch.UpMbps != 0 || ch.DownMbps != 0) && ch.Protocol != "hysteria2" {
		return fmt.Errorf("descriptors[%s]: bandwidth is hysteria2-only", ch.ID)
	}
	if ch.Multiplex != nil && (ch.Protocol != "vless" || ch.Transport == "shadowtls") {
		return fmt.Errorf("descriptors[%s]: multiplex is ws/reality-only", ch.ID)
	}
	if ch.UpMbps < 0 || ch.UpMbps > 10000 || ch.DownMbps < 0 || ch.DownMbps > 10000 {
		return fmt.Errorf("descriptors[%s]: up_mbps/down_mbps must be within 0..10000", ch.ID)
	}
	if (ch.UpMbps > 0) != (ch.DownMbps > 0) {
		return fmt.Errorf("descriptors[%s]: up_mbps and down_mbps must be set together", ch.ID)
	}
	// V2-057 (A2): diagnostic_url — только https и только наш origin
	// (диагностика обязана идти по защищённому пути выбранного канала;
	// сторонний URL дал бы измерение чужого пути).
	if ch.DiagnosticURL != "" {
		u, err := url.Parse(ch.DiagnosticURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("descriptors[%s]: diagnostic_url %q must be a valid https URL", ch.ID, ch.DiagnosticURL)
		}
	}
	return nil
}
