// Package config — строгий парсер/валидатор runtime-конфига sing-box.
//
// Это контракт ядра (PLAN §3, FR-005): конфиг валиден ТОЛЬКО если он
// защищённый — селектор "proxy" из валидных vless/hysteria2-каналов,
// без direct, без insecure TLS, без плейсхолдеров и приватных ключей,
// без битых ссылок. Всё остальное — ошибка запуска (fail-closed).
//
// Спецификация взята из ревью старого ядра (docs/old-core-review.md §2.1)
// и пере-реализована с нуля; расширение против старого ядра: явная проверка
// имён верхнеуровневых полей (незнакомое поле = ошибка контракта, не тихий
// пропуск), запрет вложенных creds в DNS, проверка дубликатов кандидатов.
package config

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — типизированное подмножество конфига sing-box, необходимое ядру.
// Неизвестные поля запрещены на верхнем уровне (Parse проверяет имена полей).
type Config struct {
	Log            *LogConfig               `json:"log,omitempty"`
	DNS            *DNSConfig               `json:"dns,omitempty"`
	Inbounds       []Inbound                `json:"inbounds"`
	Outbounds      []Outbound               `json:"outbounds"`
	Route          *RouteConfig             `json:"route"`
	Experimental   *ExperimentalConfig      `json:"experimental,omitempty"`
	BootstrapHints map[string]BootstrapHint `json:"bootstrap_hints,omitempty"` // application metadata, stripped only at the SDK boundary
}

// ExperimentalConfig — экспериментальный блок sing-box. Используем только
// clash_api (read-only метрики трафика/соединений для UI, V2-048): контроллер
// слушает только loopback, доступ по per-session секрету. CacheFile сознательно
// не включаем (V2-046: «cache.db в CWD» — мусор и стейт между сессиями).
type ExperimentalConfig struct {
	ClashAPI *ClashAPIConfig `json:"clash_api,omitempty"`
}

type ClashAPIConfig struct {
	ExternalController string `json:"external_controller,omitempty"`
	Secret             string `json:"secret,omitempty"`
}

type LogConfig struct {
	Disabled  bool   `json:"disabled,omitempty"`
	Level     string `json:"level,omitempty"`
	Output    string `json:"output,omitempty"`
	Timestamp bool   `json:"timestamp,omitempty"`
}

type DNSConfig struct {
	Servers          []DNSServer `json:"servers,omitempty"`
	Rules            []DNSRule   `json:"rules,omitempty"`
	Strategy         string      `json:"strategy,omitempty"`
	Final            string      `json:"final,omitempty"`
	IndependentCache bool        `json:"independent_cache,omitempty"`
	CacheCapacity    uint32      `json:"cache_capacity,omitempty"`
}

type DNSServer struct {
	Tag        string        `json:"tag"`
	Address    string        `json:"address,omitempty"` // read legacy input only; strict DNS contract rejects it
	Detour     string        `json:"detour,omitempty"`
	Type       string        `json:"type,omitempty"`
	Server     string        `json:"server,omitempty"`
	ServerPort uint16        `json:"server_port,omitempty"`
	TLS        *DNSTLSConfig `json:"tls,omitempty"`
}

type DNSTLSConfig struct {
	Enabled    bool   `json:"enabled,omitempty"`
	ServerName string `json:"server_name,omitempty"`
}

// Resolver for this outbound's service address, never a rule matching a user's
// queried domain. The pinned engine accepts this object in DialerOptions.
type DomainResolveOptions struct {
	Server       string `json:"server"`
	Strategy     string `json:"strategy,omitempty"`
	DisableCache bool   `json:"disable_cache,omitempty"`
}

type DNSRule struct {
	Server   string   `json:"server,omitempty"`
	Outbound string   `json:"outbound,omitempty"`
	Action   string   `json:"action,omitempty"`
	Domain   []string `json:"domain,omitempty"`
}

type Inbound struct {
	Type                string     `json:"type"`
	Tag                 string     `json:"tag"`
	Listen              string     `json:"listen,omitempty"`
	ListenPort          uint16     `json:"listen_port,omitempty"`
	InterfaceName       string     `json:"interface_name,omitempty"`
	Address             []string   `json:"address,omitempty"`
	AutoRoute           bool       `json:"auto_route,omitempty"`
	StrictRoute         bool       `json:"strict_route,omitempty"`
	RouteExcludeAddress []string   `json:"route_exclude_address,omitempty"`
	Stack               string     `json:"stack,omitempty"`
	Sniff               bool       `json:"sniff,omitempty"`
	Users               []User     `json:"users,omitempty"`
	Transport           *Transport `json:"transport,omitempty"`
}

type User struct {
	Name     string `json:"name,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
}

type Transport struct {
	Type                string `json:"type"`
	Path                string `json:"path,omitempty"`
	MaxEarlyData        int    `json:"max_early_data,omitempty"`
	EarlyDataHeaderName string `json:"early_data_header_name,omitempty"`
	// Headers — HTTP-заголовки транспорта (ws: Host при IP-диале за CDN;
	// без него sing-box отправит в Host адрес из server, и CDN не смаршрутизирует).
	Headers map[string]string `json:"headers,omitempty"`
}

// MultiplexConfig — мультиплексирование потоков в одном соединении outbound
// (h2mux/smux/yamux). Отдельные TLS-хендшейки на каждое соединение пользователя
// — основной источник задержки VLESS+WS+CDN; mux переиспользует соединения.
// Padding маскирует длины кадров от пассивного DPI (внутри TLS не видно,
// но полезен против размерной статистики самого WS-кадра).
type MultiplexConfig struct {
	Enabled        bool   `json:"enabled"`
	Protocol       string `json:"protocol,omitempty"`
	MaxConnections int    `json:"max_connections,omitempty"`
	MinStreams     int    `json:"min_streams,omitempty"`
	MaxStreams     int    `json:"max_streams,omitempty"`
	Padding        bool   `json:"padding,omitempty"`
}

type Outbound struct {
	Type           string                `json:"type"`
	Tag            string                `json:"tag"`
	Server         string                `json:"server,omitempty"`
	ServerPort     uint16                `json:"server_port,omitempty"`
	DomainResolver *DomainResolveOptions `json:"domain_resolver,omitempty"`
	UUID           string                `json:"uuid,omitempty"`
	Password       string                `json:"password,omitempty"`
	// V2-051 (shadowtls-цепочка): соединение через другой outbound
	// (vless-звено → shadowtls-звено). DialerOptions.detour ядра.
	Detour string `json:"detour,omitempty"`
	// ShadowTLS: версия протокола (v3). Остальные протоколы — не задают.
	Version                   int              `json:"version,omitempty"`
	Flow                      string           `json:"flow,omitempty"`
	TLS                       map[string]any   `json:"tls,omitempty"`
	Obfs                      map[string]any   `json:"obfs,omitempty"`
	Transport                 *Transport       `json:"transport,omitempty"`
	Multiplex                 *MultiplexConfig `json:"multiplex,omitempty"`
	Outbounds                 []string         `json:"outbounds,omitempty"`
	Default                   string           `json:"default,omitempty"`
	InterruptExistConnections bool             `json:"interrupt_exist_connections,omitempty"`
	// Hysteria2: port hopping (диапазоны портов сервера, DNAT на сервере)
	// и brutal CC (фиксированная скорость вместо BBR — стабилен на lossy-линках).
	ServerPorts []string `json:"server_ports,omitempty"`
	HopInterval string   `json:"hop_interval,omitempty"`
	UpMbps      int      `json:"up_mbps,omitempty"`
	DownMbps    int      `json:"down_mbps,omitempty"`
	// Dial-fields (V2-045): TFO ускоряет ресоединения (SYN+data в одном сегменте),
	// keep-alive держит NAT/стейт-записи живыми для простаивающих REALITY-сессий
	// (системные дефолты 2ч+ рвут сессии на транзитных фильтрах).
	// V2-046: строковая duration-форма ("60s") — sing-box парсит эти поля как
	// badoption.Duration; int-форма отвергается живым парсером ядра (дефект
	// V2-045 поймал владелец: рендер с REALITY ломал ПОЛНЫЙ Start, т.к. каналы
	// REALITY входят в общий селектор).
	TCPFastOpen    bool   `json:"tcp_fast_open,omitempty"`
	TCPKeepAlive   string `json:"tcp_keep_alive,omitempty"`          // duration-строка ("60s")
	TCPKeepAliveIt string `json:"tcp_keep_alive_interval,omitempty"` // duration-строка ("20s")
}

type RouteConfig struct {
	Rules       []RouteRule `json:"rules,omitempty"`
	Final       string      `json:"final"`
	FindProcess bool        `json:"find_process,omitempty"` // V2-048/F10: нужен для process_name-правил
}

// AllowsServicePort uses the same range parser as normalized outbound validation.
// It describes only the declared server/hop port set, never route authority.
func (o Outbound) AllowsServicePort(port uint16) bool {
	if port == 0 {
		return false
	}
	if o.Type != "hysteria2" || len(o.ServerPorts) == 0 {
		return port == o.ServerPort
	}
	matched := false
	for _, value := range o.ServerPorts {
		low, high, err := parsePortRange(value)
		if err != nil || low == 0 || high == 0 || low > high {
			return false
		}
		matched = matched || int(port) >= low && int(port) <= high
	}
	return matched
}

type RouteRule struct {
	Protocol     string   `json:"protocol,omitempty"`
	Action       string   `json:"action,omitempty"`
	Outbound     string   `json:"outbound,omitempty"`
	IPCidr       []string `json:"ip_cidr,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	ProcessName  []string `json:"process_name,omitempty"` // V2-048/F10: сплит по процессам (Windows)
	Port         []uint16 `json:"port,omitempty"`
}

// Parse разбирает и валидирует конфиг. Возвращает ошибку с точной причиной.
func Parse(data []byte) (*Config, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("config is empty")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		// DisallowUnknownFields даёт "unknown field" в тексте — оставляем как есть,
		// причина уже точная (json синтаксис/неизвестное поле).
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := noTrailingData(dec); err != nil {
		return nil, err
	}
	if err := checkUnknownTopLevel(data); err != nil {
		return nil, err
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

// Load — Parse из файла.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(data)
}

func noTrailingData(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("config: trailing data after JSON object")
		}
		return fmt.Errorf("config: trailing data: %w", err)
	}
	return nil
}

// checkUnknownTopLevel — неизвестные поля верхнего уровня запрещены явно
// (тот же эффект, что DisallowUnknownFields, но до типизации: если поле
// добавили в sing-box, это осознанное изменение контракта, а не тихий пропуск).
func checkUnknownTopLevel(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("config: not a JSON object: %w", err)
	}
	allowed := map[string]bool{
		"log": true, "dns": true, "inbounds": true, "outbounds": true, "route": true,
		"experimental":    true, // V2-048: clash_api метрики (read-only, loopback)
		"bootstrap_hints": true,
	}
	for k := range raw {
		if !allowed[k] {
			return fmt.Errorf("config: unknown top-level field %q", k)
		}
	}
	return nil
}

// Normalize — канонизация легаси-полей.
func (c *Config) Normalize() {
	if c == nil {
		return
	}
	for i := range c.Outbounds {
		if c.Outbounds[i].Type == "urltest" {
			c.Outbounds[i].Type = "selector"
		}
	}
}

// Validate — полный контракт защищённого конфига.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if len(c.Inbounds) == 0 {
		return errors.New("no inbounds")
	}
	if len(c.Outbounds) == 0 {
		return errors.New("no outbounds")
	}
	if c.Route == nil {
		return errors.New("no route section")
	}
	if strings.TrimSpace(c.Route.Final) == "" {
		return errors.New("route.final is empty")
	}

	// Public inline certificate PEM is permitted only in the validated TLS
	// certificate field. Every other value retains the full placeholder/key gate.
	forScan, err := withoutInlineCertificates(c)
	if err != nil {
		return err
	}
	serialized, err := json.Marshal(forScan)
	if err != nil {
		return fmt.Errorf("serialize for validation: %w", err)
	}
	if ContainsPlaceholder(string(serialized)) {
		return errors.New("placeholder or private-key material in config")
	}

	// Inbounds: теги уникальны и непусты.
	inTags := make(map[string]struct{}, len(c.Inbounds))
	for _, in := range c.Inbounds {
		if in.Type != "mixed" && in.Type != "tun" {
			return errors.New("inbound family has no protected forwarding adapter")
		}
		if strings.TrimSpace(in.Tag) == "" {
			return errors.New("inbound tag is empty")
		}
		if _, dup := inTags[in.Tag]; dup {
			return fmt.Errorf("duplicate inbound tag: %s", in.Tag)
		}
		inTags[in.Tag] = struct{}{}
	}

	// Outbounds: теги уникальны и непусты.
	tags := make(map[string]Outbound, len(c.Outbounds))
	for _, ob := range c.Outbounds {
		if strings.TrimSpace(ob.Tag) == "" {
			return errors.New("outbound tag is empty")
		}
		if _, dup := tags[ob.Tag]; dup {
			return fmt.Errorf("duplicate outbound tag: %s", ob.Tag)
		}
		tags[ob.Tag] = ob
	}

	// Защищённый селектор "proxy" — проверяем ПЕРЕД route.final: ошибка
	// отсутствия селектора понятнее, чем «route.final ссылается на неизвестный
	// outbound», когда финал указывает как раз на селектор.
	if err := validateProtectedSelector(c, tags); err != nil {
		return err
	}

	// Route authority must use the same selector whose complete candidate set
	// passed the protected-protocol gates above. Tag existence alone admits
	// direct aliases, unvalidated orphan services and selector-bypassing rules.
	if err := c.validateRuntimeGraph(tags); err != nil {
		return err
	}

	if err := c.validateDNS(); err != nil {
		return err
	}
	return c.validateBootstrapHints()
}

func validateProtectedSelector(c *Config, tags map[string]Outbound) error {
	sel, ok := tags["proxy"]
	if !ok {
		return errors.New(`protected selector "proxy" is required`)
	}
	if sel.Type != "selector" {
		return errors.New(`"proxy" must be a selector outbound`)
	}
	if len(sel.Outbounds) == 0 {
		return errors.New(`protected selector "proxy" has no candidates`)
	}
	if sel.Default == "" {
		return errors.New(`protected selector default is required`)
	}
	if !containsString(sel.Outbounds, sel.Default) {
		return errors.New(`protected selector default is not a candidate`)
	}
	// Reject the unsupported node anywhere in the runtime graph, including
	// unselected aliases/detours. A TLS-looking donor handshake or an added TLS
	// flag is not an approved inner-encrypted composition/profile migration.
	for _, outbound := range tags {
		if err := validateShadowTLSLink(outbound); err != nil {
			return err
		}
	}
	seen := make(map[string]bool, len(sel.Outbounds))
	for _, cand := range sel.Outbounds {
		if seen[cand] {
			return fmt.Errorf("protected selector duplicate candidate: %s", cand)
		}
		seen[cand] = true
		if cand == "direct" {
			return errors.New("direct is forbidden in protected selector")
		}
		prot, exists := tags[cand]
		if !exists {
			return fmt.Errorf("protected selector references unknown outbound: %s", cand)
		}
		if !isProtectedOutboundType(prot.Type) {
			return fmt.Errorf("protected selector candidate is not a protected protocol: %s (type %s)", cand, prot.Type)
		}
		if prot.Server == "" || prot.ServerPort == 0 {
			return fmt.Errorf("protected outbound %q lacks server/port", cand)
		}
		if prot.UUID == "" && prot.Password == "" {
			return fmt.Errorf("protected outbound %q lacks credentials", cand)
		}
		// Legacy graph-reference validation grants no payload-confidentiality
		// exemption. Current ShadowTLS nodes are rejected above for the whole graph.
		if prot.Detour != "" {
			up, ok := tags[prot.Detour]
			if !ok {
				return fmt.Errorf("protected outbound %q detours unknown outbound %q", cand, prot.Detour)
			}
			if up.Type != "shadowtls" {
				return fmt.Errorf("protected outbound %q detours non-shadowtls outbound %q (%s)", cand, prot.Detour, up.Type)
			}
		}
		// XTLS Vision отсутствует в pinned-ядре (V2-031): такие конфиги
		// отвергаются до старта движка с понятной причиной.
		if prot.Flow != "" {
			return fmt.Errorf("protected outbound %q: xtls flow %q is not supported by the pinned core", cand, prot.Flow)
		}
		if err := validateProtectedTLS(prot); err != nil {
			return fmt.Errorf("protected outbound %q: %w", cand, err)
		}
		if err := validateRealityTLS(prot); err != nil {
			return fmt.Errorf("protected outbound %q: %w", cand, err)
		}
		if err := validateTransportExtras(prot); err != nil {
			return fmt.Errorf("protected outbound %q: %w", cand, err)
		}
		if err := validateOutboundExtras(prot); err != nil {
			return fmt.Errorf("protected outbound %q: %w", cand, err)
		}
		if err := validateDialFields(prot); err != nil {
			return fmt.Errorf("protected outbound %q: %w", cand, err)
		}
	}
	return nil
}

// validateTransportExtras — заголовки транспорта: непустые имена/значения,
// стандартный набор для ws (Host и т.п.); произвольный мусор не нужен.
func validateTransportExtras(o Outbound) error {
	if o.Transport == nil {
		return nil
	}
	for name, value := range o.Transport.Headers {
		if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\r\n") {
			return fmt.Errorf("transport header name is empty or contains control characters")
		}
		if value == "" || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("transport header %q value is empty or contains control characters", name)
		}
	}
	return nil
}

// validateDialFields — dial-fields: duration-строки для keep-alive обязаны
// парситься (форма «60» без единицы отвергается ядром), TFO — без ограничений.
// V2-046: тест «рендер → sing-box Parse» дополнительно гоняет полный парсинг
// ядра на отрендеренном JSON (без сети) — unit-тесты нашего парсера дефект
// V2-045 не ловили, т.к. он жил в расхождении наших типов и типов ядра.
func validateDialFields(o Outbound) error {
	for name, v := range map[string]string{
		"tcp_keep_alive":          o.TCPKeepAlive,
		"tcp_keep_alive_interval": o.TCPKeepAliveIt,
	} {
		if v == "" {
			continue
		}
		if _, err := time.ParseDuration(v); err != nil {
			return fmt.Errorf("%s %q не является duration-строкой (ожидается форма \"60s\")", name, v)
		}
	}
	if o.TCPKeepAliveIt != "" && o.TCPKeepAlive == "" {
		return errors.New("tcp_keep_alive_interval без tcp_keep_alive не имеет смысла")
	}
	return nil
}

// validateOutboundExtras — multiplex/hopping/brutal: формат проверяется до
// старта движка, чтобы опечатка в дескрипторе не падала непонятной ошибкой
// sing-box на живом пути.
func validateOutboundExtras(o Outbound) error {
	if o.Multiplex != nil {
		switch o.Multiplex.Protocol {
		case "", "h2mux", "smux", "yamux":
		default:
			return fmt.Errorf("multiplex.protocol %q is not supported (h2mux|smux|yamux)", o.Multiplex.Protocol)
		}
		if o.Multiplex.MaxConnections < 0 || o.Multiplex.MinStreams < 0 || o.Multiplex.MaxStreams < 0 {
			return errors.New("multiplex connection/stream limits must not be negative")
		}
	}
	for _, sp := range o.ServerPorts {
		lo, hi, err := parsePortRange(sp)
		if err != nil {
			return fmt.Errorf("server_ports %q: %w", sp, err)
		}
		if lo == 0 || hi == 0 || lo > hi {
			return fmt.Errorf("server_ports %q: invalid range", sp)
		}
	}
	if o.HopInterval != "" {
		d, err := time.ParseDuration(o.HopInterval)
		if err != nil || d < 5*time.Second || d > 10*time.Minute {
			return fmt.Errorf("hop_interval %q must be a duration 5s..10m", o.HopInterval)
		}
	}
	if o.UpMbps < 0 || o.UpMbps > 10000 || o.DownMbps < 0 || o.DownMbps > 10000 {
		return errors.New("up_mbps/down_mbps must be within 0..10000")
	}
	return nil
}

// parsePortRange — "lo:hi" (оба 1..65535).
func parsePortRange(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("expected format lo:hi")
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("bad low port: %w", err)
	}
	hi, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("bad high port: %w", err)
	}
	if lo < 1 || lo > 65535 || hi < 1 || hi > 65535 {
		return 0, 0, errors.New("ports must be within 1..65535")
	}
	return lo, hi, nil
}

// validateProtectedTLS: TLS включён, insecure запрещён, SNI задан,
// пин-сертификат (если указан) — валидный PEM по существующему пути.
// validateRealityTLS: если outbound использует REALITY (tls.reality), блок
// обязан быть полным и осмысленным: enabled, публичный ключ сервера, short_id.
// REALITY не требует пиннинга и не допускает insecure — сертификат донора
// проверяется как обычный публичный (B1, V2-031/032).
func validateRealityTLS(o Outbound) error {
	raw, ok := o.TLS["reality"]
	if !ok {
		return nil // не REALITY-канал — правил нет
	}
	reality, ok := raw.(map[string]any)
	if !ok {
		return errors.New("tls.reality must be an object")
	}
	if !tlsBool(reality, "enabled") {
		return errors.New("tls.reality.enabled must be true (half-configured reality is forbidden)")
	}
	if tlsString(reality, "public_key") == "" {
		return errors.New("tls.reality.public_key is required")
	}
	if tlsString(reality, "short_id") == "" {
		return errors.New("tls.reality.short_id is required")
	}
	return nil
}

// isProtectedOutboundType — типы outbound'ов, разрешённые в защищённом
// селекторе. ShadowTLS camouflage is not payload encryption; the current
// VLESS-over-ShadowTLS composition is not admitted as a protected runtime graph.
func isProtectedOutboundType(t string) bool {
	return t == "vless" || t == "hysteria2"
}

// ShadowTLS has no supported inner-encrypted composition in the current product
// graph. A future version requires explicit profile/server/cipher migration.
func validateShadowTLSLink(o Outbound) error {
	if o.Type != "shadowtls" {
		return nil
	}
	return fmt.Errorf("shadowtls %q: unsupported protected payload confidentiality; an approved inner-encrypted composition is required", o.Tag)
}

func validateProtectedTLS(o Outbound) error {
	if o.TLS == nil {
		return errors.New("verified TLS is required")
	}
	if !tlsBool(o.TLS, "enabled") {
		return errors.New("tls.enabled must be true")
	}
	if tlsBool(o.TLS, "insecure") {
		return errors.New("tls.insecure is forbidden: certificate verification cannot be disabled")
	}
	if _, present := o.TLS["certificate_public_key_sha256"]; present {
		return errors.New("SPKI-only verification bypasses standard hostname/validity checks and is not supported")
	}
	if tlsString(o.TLS, "server_name") == "" {
		return errors.New("tls.server_name is required")
	}
	if path := tlsString(o.TLS, "certificate_path"); path != "" {
		return validatePinnedCertificate(path)
	}
	return nil
}

func validatePinnedCertificate(path string) error {
	_, err := readPinnedCertificates(path)
	return err
}

// Bounded legacy public bundle read. This is intentionally not used by the
// in-memory runtime identity digest; the native TLS verifier still checks peers.
func readPinnedCertificates(path string) ([]*x509.Certificate, error) {
	if ContainsPlaceholder(path) {
		return nil, errors.New("certificate path still contains a placeholder")
	}
	const limit = 1 << 20
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("pinned certificate: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("pinned certificate file must be bounded and regular")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("pinned certificate file cannot be opened")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("pinned certificate file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("pinned certificate read exceeds bound")
	}
	beginCount := bytes.Count(data, []byte("-----BEGIN "))
	var certs []*x509.Certificate
	for data = bytes.TrimSpace(data); len(data) > 0; data = bytes.TrimSpace(data) {
		if len(certs) >= 64 || !bytes.HasPrefix(data, []byte("-----BEGIN CERTIFICATE-----")) {
			return nil, errors.New("pinned certificate bundle has unexpected data")
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return nil, errors.New("pinned certificate bundle is invalid")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.NotAfter.After(cert.NotBefore) {
			return nil, errors.New("pinned certificate has invalid DER or dates")
		}
		certs = append(certs, cert)
		data = rest
	}
	if len(certs) == 0 || beginCount != len(certs) {
		return nil, errors.New("pinned certificate bundle contains skipped blocks")
	}
	return certs, nil
}

var ErrNoCurrentPinnedCertificate = errors.New("pinned certificate bundle has no current validity")

// PinnedCertificateSnapshot owns the exact public bytes sent to the worker.
// Expired entries are excluded; a future rotation entry cannot make an otherwise
// inactive bundle usable. A CA bundle does not reveal the peer leaf's NotAfter.
func PinnedCertificateSnapshot(path string, now time.Time) ([]string, time.Time, error) {
	certs, err := readPinnedCertificates(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	var deadline time.Time
	var bundle []string
	for _, cert := range certs {
		if !now.Before(cert.NotAfter) {
			continue
		}
		bundle = append(bundle, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})))
		if !now.Before(cert.NotBefore) && now.Before(cert.NotAfter) && (deadline.IsZero() || cert.NotAfter.Before(deadline)) {
			deadline = cert.NotAfter
		}
	}
	if deadline.IsZero() {
		return nil, time.Time{}, ErrNoCurrentPinnedCertificate
	}
	return bundle, deadline, nil
}

func PinnedCertificateDeadline(path string, now time.Time) (time.Time, error) {
	_, deadline, err := PinnedCertificateSnapshot(path, now)
	return deadline, err
}

func pemPool(data []byte) bool {
	// Настоящий x509-парс (как в runtime): секции BEGIN/END без валидного
	// сертификата внутри не проходят — это ловится ДО старта движка.
	roots := x509.NewCertPool()
	return roots.AppendCertsFromPEM(data)
}

func tlsBool(tls map[string]any, key string) bool {
	v, ok := tls[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func tlsString(tls map[string]any, key string) string {
	v, ok := tls[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// ContainsPlaceholder — значение осталось шаблонной заглушкой или содержит
// приватный ключ. Такие значения в runtime-конфиге запрещены (fail-closed).
func ContainsPlaceholder(value string) bool {
	upper := strings.ToUpper(value)
	return strings.Contains(upper, "YOUR_") ||
		strings.Contains(upper, "CHANGE_ME") ||
		strings.Contains(upper, "REPLACE_WITH") ||
		strings.Contains(upper, "{{") ||
		strings.Contains(value, "-----BEGIN")
}

// ---------- доступ к защищённой части ----------

// Endpoint — публичные метаданные защищённого канала (без секретов).
type Endpoint struct {
	Tag             string
	Type            string
	Server          string
	ServerPort      uint16
	ServerName      string
	CertificatePath string
}

// ProtectedEndpoints — защищённые сервера из селектора "proxy" (в порядке
// кандидатов). Используется TLS preflight.
func (c *Config) ProtectedEndpoints() []Endpoint {
	if c == nil {
		return nil
	}
	tags := make(map[string]Outbound, len(c.Outbounds))
	for _, ob := range c.Outbounds {
		tags[ob.Tag] = ob
	}
	sel, ok := tags["proxy"]
	if !ok {
		return nil
	}
	out := make([]Endpoint, 0, len(sel.Outbounds))
	for _, cand := range sel.Outbounds {
		prot, ok := tags[cand]
		if !ok || (prot.Type != "vless" && prot.Type != "hysteria2") {
			continue
		}
		out = append(out, Endpoint{
			Tag:             prot.Tag,
			Type:            prot.Type,
			Server:          prot.Server,
			ServerPort:      prot.ServerPort,
			ServerName:      tlsString(prot.TLS, "server_name"),
			CertificatePath: tlsString(prot.TLS, "certificate_path"),
		})
	}
	return out
}

// ProtectedSelector — порядок кандидатов селектора (копия, не алиас).
func (c *Config) ProtectedSelector() []string {
	if c == nil {
		return nil
	}
	for _, ob := range c.Outbounds {
		if ob.Type == "selector" && ob.Tag == "proxy" {
			return append([]string(nil), ob.Outbounds...)
		}
	}
	return nil
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
