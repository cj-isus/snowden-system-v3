// Package render — сборка runtime-конфига клиента из дескрипторов каналов
// (FR-002: каналы — данные, не хардкод) и значений секретов.
//
// Ключевые правила (docs/old-core-review.md §3):
//   - в защищённый селектор попадают ТОЛЬКО каналы с validation_status
//     "live-verified" (UDP-gate для hysteria2 — то же условие; HY2 не может
//     оказаться в селекторе, пока не доказан live из UDP-сети);
//   - DNS: все запросы — через туннель; direct-бутстрап только для имени
//     самого сервера туннеля (иначе круговое разрешение), это не трафик
//     пользователя;
//   - HY2 требует pin-сертификат: рендер откажется собирать канал без него;
//   - результат обязан проходить config.Parse (проверено тестом).
package render

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/metadata"
)

//go:embed descriptors.json
var descriptorsRaw []byte

// ChannelDescriptor — декларация канала (без секретов; только ссылки).
type ChannelDescriptor struct {
	ID               string   `json:"id"`
	Protocol         string   `json:"protocol"`  // vless | hysteria2
	Transport        string   `json:"transport"` // ws | quic | tcp-reality (schema 2)
	Hostname         string   `json:"hostname"`  // edge/SNI имя (для tcp-reality — SNI донора)
	Port             uint16   `json:"port"`      // порт на edge/сервере
	OriginServer     string   `json:"origin_server"`
	ExpectedEgress   string   `json:"expected_egress"`
	CredentialRefs   []string `json:"credential_refs"`
	ValidationStatus string   `json:"validation_status"`
	ValidatedAt      *string  `json:"validated_at"`
	Enabled          bool     `json:"enabled"`
	// Schema 2 (B1/V2-031, VLESS+TCP+REALITY): роль каждой ссылки задана
	// явно, чтобы рендер не угадывал её по имени (урок refOf: нечёткое
	// сопоставление подставляет пароль вместо obfs-пароля).
	UUIDRef             string `json:"uuid_ref,omitempty"`
	RealityPublicKeyRef string `json:"reality_public_key_ref,omitempty"`
	RealityShortIDRef   string `json:"reality_short_id_ref,omitempty"`

	// ---------- V2-037: живучесть/скорость (все поля optional) ----------

	// ProbeTargets — HTTPS-эхо egress для probe этого канала (2..3).
	// Первой целью обязан быть self-hosted эндпоинт (наш VPS за CDN) —
	// сторонние ipify/ifconfig rate-limit'ят watchdog каждые 30с и дают
	// ложные деградации (V2-037); вторая — независимое подтверждение.
	// Отсутствует → дефолтная пара (сторонние эхо).
	ProbeTargets []string `json:"probe_targets,omitempty"`
	// ServerPorts/HopInterval — HY2 port hopping ("lo:hi", "30s"): клиент
	// мигрирует по диапазону портов; на сервере диапазон DNAT'ится на
	// реальный порт. Снимает точечную UDP-блокировку порта.
	ServerPorts []string `json:"server_ports,omitempty"`
	HopInterval string   `json:"hop_interval,omitempty"`
	// UpMbps/DownMbps — HY2 brutal CC: фиксированная скорость вместо BBR.
	// 0/отсутствует = BBR (честный дефолт для неизвестного линка); заданы —
	// brutal, стабильно работающий на lossy-линках при честных значениях.
	UpMbps   int `json:"up_mbps,omitempty"`
	DownMbps int `json:"down_mbps,omitempty"`
	// Multiplex — vless: h2mux+padding в одном соединении (нет TLS-хендшейка
	// на каждое соединение пользователя; основная задержка VLESS+WS+CDN).
	// nil = вкл по умолчанию для ws; false = явно выключен; true = вкл
	// (для tcp-reality, где по умолчанию выключен).
	Multiplex *bool `json:"multiplex,omitempty"`
}

// DefaultProbeTargets — пара по умолчанию, если дескриптор не задаёт своих.
var DefaultProbeTargets = []string{"https://api.ipify.org", "https://ifconfig.me/all.json"}

type descriptorSet struct {
	Schema   int                 `json:"schema"`
	Channels []ChannelDescriptor `json:"channels"`
	// V2-048/F10: сплит-туннелинг по процессам. Процессы из списка идут
	// НАПРЯМУЮ (мимо туннеля) — безопасный дефолт для банков/игр/локальных
	// сервисов, несовместимых с TUN. Валидация: имя процесса нижнего регистра,
	// без пути ("chrome.exe"); «direct-all» запрещён (это выключение VPN).
	SplitDirect []string `json:"split_direct,omitempty"`
}

// SecretsSource — доступ рендера к значениям (реализация — secretvault).
type SecretsSource interface {
	// Get возвращает значение по ссылке на слот (credential_ref).
	Get(ref string) (string, error)
	// PinCertPath возвращает путь к pin-PEM канала ("" = нет).
	PinCertPath(channelID string) string
}

// LoadDescriptors читает встроенный набор дескрипторов, валидирует его и
// применяет подписанный metadata-envelope (FR-008, если развёрнут в
// %AppData%\snowden-system\metadata).
//
// Семантика override (fail-closed):
//   - валидный envelope заменяет встроенный набор целиком (replace), после
//     чего результат повторно проходит parseDescriptors (тот же строгий гейт);
//   - повреждённый/неподписанный/просроченный/даунгрейд- envelope — ОШИБКА,
//     а не тихий откат к встроенным (атакующий не должен иметь возможность
//     «сломать» доставку до встроенного статичного набора);
//   - revocations запрещают ID: revoked-канал удаляется из результата (если
//     из-за этого не остаётся каналов — рендер откажется ниже, fail-closed);
//   - повышение validation_status до "live-verified" допускается только если
//     в envelope-канале заполнены validated_at (evidence tuple). Иначе статус
//     понижается к "configured" — фейковые live-verified через metadata
//     невозможны.
func LoadDescriptors() ([]ChannelDescriptor, error) {
	channels, err := parseDescriptors(descriptorsRaw)
	if err != nil {
		return nil, err
	}
	// V2-048/F10: сплит-список берётся из встроенного набора, envelope (если
	// доставлен) заменяет его в applyEnvelopeOverride (единый источник правды).
	splitSet = nil
	var set descriptorSet
	if err := json.Unmarshal(descriptorsRaw, &set); err == nil {
		splitSet = validateSplitDirect(set.SplitDirect)
	}
	channels, err = applyEnvelopeOverride(channels)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// splitSet — активный список процессов прямого обхода (нижний регистр).
// Заполняется LoadDescriptors из встроенного набора или envelope.
var splitSet []string

// SplitDirect — текущий список сплита (для UI-карточки, read-only).
func SplitDirect() []string {
	out := make([]string, len(splitSet))
	copy(out, splitSet)
	return out
}

// validateSplitDirect — нормализация/валидация списка процессов (общая для
// встроенного набора и envelope): нижний регистр, без пути, без пробелов,
// дубликаты схлопываются. Пустые/битые имена — ошибка (fail-closed).
func validateSplitDirect(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || strings.ContainsAny(p, `/\\`) || strings.Contains(p, " ") {
			continue // мягко: встроенный набор статичен, ошибка в нём = ошибка сборки тестами
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// routeRulesWithSplit — базовые правила + опциональный процесс-сплит.
// ВАЖНО (fail-open guard): правило process_name добавляется ТОЛЬКО при
// непустом списке — правило без условий в sing-box матчит ВСЁ, и пустой
// сплит превратил бы весь трафик в direct (выключенный VPN без признаков).
func routeRulesWithSplit(split []string) []config.RouteRule {
	rules := make([]config.RouteRule, 0, 4)
	rules = append(rules, config.RouteRule{Protocol: "dns", Action: "hijack-dns"})
	if len(split) > 0 {
		// V2-048/F10: процессы из split_direct — прямиком (до RFC1918,
		// чтобы их локальный трафик не зависел от порядка правил).
		rules = append(rules, config.RouteRule{ProcessName: split, Outbound: "direct"})
	}
	rules = append(rules,
		config.RouteRule{IPCidr: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}, Outbound: "direct"},
		config.RouteRule{DomainSuffix: []string{".local"}, Outbound: "direct"},
	)
	return rules
}

// envelopeStore — точка подмены в тестах (по умолчанию — AppData-хранилище).
var envelopeStore = func() *metadata.Store {
	dir, err := metadata.DefaultStoreDir()
	if err != nil {
		return nil
	}
	return &metadata.Store{Dir: dir}
}()

// parseSet — строгая валидация готового набора дескрипторов (общий гейт
// для встроенных и envelope-каналов).
func parseSet(channels []ChannelDescriptor) ([]ChannelDescriptor, error) {
	seen := map[string]bool{}
	for i, ch := range channels {
		if ch.ID == "" {
			return nil, fmt.Errorf("descriptors[%d]: empty id", i)
		}
		if seen[ch.ID] {
			return nil, fmt.Errorf("descriptors[%d]: duplicate id %s", i, ch.ID)
		}
		seen[ch.ID] = true
		if err := validateChannel(ch); err != nil {
			return nil, err
		}
	}
	return channels, nil
}

// envelopeVersionPath — файл с последним принятым metadata_version
// (антидаунгрейд; живёт рядом с envelope).
func envelopeVersionPath(dir string) string { return filepath.Join(dir, "version.txt") }

func readEnvelopeVersion(dir string) uint64 {
	data, err := os.ReadFile(envelopeVersionPath(dir))
	if err != nil {
		return 0
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func writeEnvelopeVersion(dir string, v uint64) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := envelopeVersionPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatUint(v, 10)), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, envelopeVersionPath(dir))
}

// applyEnvelopeOverride — валидация, ревокации и статус-правила FR-008.
// nil-стора (нет UserConfigDir) или отсутствие envelope — встроенный набор.
func applyEnvelopeOverride(channels []ChannelDescriptor) ([]ChannelDescriptor, error) {
	store := envelopeStore
	if store == nil {
		return channels, nil
	}
	env, err := store.Apply(time.Now(), readEnvelopeVersion(store.Dir))
	if err != nil {
		return nil, fmt.Errorf("metadata override: %w", err)
	}
	if env == nil {
		return channels, nil
	}

	revoked := map[string]bool{}
	for _, id := range env.Revocations {
		revoked[id] = true
	}
	out := make([]ChannelDescriptor, 0, len(env.Channels))
	for _, mc := range env.Channels {
		if revoked[mc.ID] {
			continue
		}
		ch := ChannelDescriptor{
			ID: mc.ID, Protocol: mc.Protocol, Transport: mc.Transport,
			Hostname: mc.Hostname, Port: mc.Port,
			OriginServer: mc.OriginServer, ExpectedEgress: mc.ExpectedEgress,
			CredentialRefs:   mc.CredentialRefs,
			ValidationStatus: mc.ValidationStatus,
			Enabled:          mc.Enabled,
		}
		if mc.ValidatedAt != nil {
			va := *mc.ValidatedAt
			ch.ValidatedAt = &va
		}
		// Evidence-gate статуса: live-verified без validated_at понижается.
		if ch.ValidationStatus == "live-verified" && ch.ValidatedAt == nil {
			ch.ValidationStatus = "configured"
		}
		out = append(out, ch)
	}
	// Строгий гейт применяется к итоговому набору (те же правила, что и для
	// встроенных дескрипторов; envelope-каналы обязаны им соответствовать).
	if _, err := parseSet(out); err != nil {
		return nil, err
	}
	// V2-048/F10: сплит из envelope — единый источник правды метаданных
	// (заменяет встроенный список целиком, если конверт доставлен).
	splitSet = env.SplitDirect
	// version.txt коммитится ПОСЛЕ того, как весь набор прошёл строгий гейт
	// (V2-034): иначе отказ валидации на envelope N+1 зафиксировал бы его
	// version как «принятый» и навсегда заблокировал бы переустановку N.
	if err := writeEnvelopeVersion(store.Dir, env.MetadataVersion); err != nil {
		return nil, fmt.Errorf("metadata override: сохранить version: %w", err)
	}
	return out, nil
}

// parseDescriptors — валидация встроенного набора (точка для тестов schema 2).
func parseDescriptors(data []byte) ([]ChannelDescriptor, error) {
	var set descriptorSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("descriptors: %w", err)
	}
	if set.Schema != 1 && set.Schema != 2 {
		return nil, fmt.Errorf("descriptors: unsupported schema %d", set.Schema)
	}
	out, err := parseSet(set.Channels)
	if err != nil {
		return nil, fmt.Errorf("descriptors: %w", err)
	}
	// tcp-reality допустим только в schema 2 (проверка на уровне набора,
	// т.к. schema — поле envelope, а не канала).
	if set.Schema < 2 {
		for _, ch := range out {
			if ch.Transport == "tcp-reality" {
				return nil, fmt.Errorf("descriptors[%s]: tcp-reality requires descriptor schema 2", ch.ID)
			}
		}
	}
	return out, nil
}

// validateChannel — правила одного дескриптора (общие для встроенных и
// envelope-каналов). Ошибка предваряется префиксом descriptors[%s].
func validateChannel(ch ChannelDescriptor) error {
	if ch.Protocol != "vless" && ch.Protocol != "hysteria2" {
		return fmt.Errorf("descriptors[%s]: unsupported protocol %q", ch.ID, ch.Protocol)
	}
	switch {
	case ch.Protocol == "vless" && ch.Transport == "ws":
	case ch.Protocol == "hysteria2" && ch.Transport == "quic":
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
	default:
		return fmt.Errorf("descriptors[%s]: transport %q does not match protocol %q", ch.ID, ch.Transport, ch.Protocol)
	}
	if net.ParseIP(strings.TrimSpace(ch.ExpectedEgress)) == nil {
		return fmt.Errorf("descriptors[%s]: expected_egress is not an IP", ch.ID)
	}
	if ch.Hostname == "" || ch.Port == 0 || ch.OriginServer == "" {
		return fmt.Errorf("descriptors[%s]: incomplete endpoint", ch.ID)
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
func validateChannelExtras(ch ChannelDescriptor) error {
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
	if ch.UpMbps < 0 || ch.UpMbps > 10000 || ch.DownMbps < 0 || ch.DownMbps > 10000 {
		return fmt.Errorf("descriptors[%s]: up_mbps/down_mbps must be within 0..10000", ch.ID)
	}
	if (ch.UpMbps > 0) != (ch.DownMbps > 0) {
		return fmt.Errorf("descriptors[%s]: up_mbps and down_mbps must be set together", ch.ID)
	}
	return nil
}

// Selectable — каналы, которым разрешено войти в защищённый селектор.
// live-verified — безусловно. configured — тоже (A1.3, обобщено в B1: без
// этого канал невозможно когда-либо проверить live — проверка возможна
// только через сам селектор), НО default'ом становится только live-verified
// (pickDefault): вход configured-канала — только явным выбором SelectChannel,
// который обязан пройти probe (fail-closed). Авто-failover configured не
// выбирает (failoverCandidates — только live-verified, FR-007).
func Selectable(channels []ChannelDescriptor) []ChannelDescriptor {
	out := make([]ChannelDescriptor, 0, len(channels))
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		if ch.ValidationStatus == "live-verified" || ch.ValidationStatus == "configured" {
			out = append(out, ch)
		}
	}
	return out
}

// ClientConfig — собранный клиентский конфиг + метаданные для probe/UI.
type ClientConfig struct {
	Raw []byte
	// ExpectedEgress — ожидаемый egress ВЫБРАННОГО канала (default или того,
	// под кого делался рендер). Каналы B1 живут на разных VPS: ожидание
	// обязано следовать за выбором, иначе probe после переключения канала
	// сравнит egress нового сервера с IP старого (дефект пойман на ревью B1,
	// до его live-проявления).
	ExpectedEgress string
	// ProbeTargets — HTTPS-эхо egress для probe выбранного канала
	// (дескриптор → self-hosted первичная цель; иначе DefaultProbeTargets).
	ProbeTargets    []string
	SelectorTags    []string            // состав селектора (для UI, A1.4)
	SelectorDefault string              // текущий выбранный канал (A1.4)
	ChannelIDs      map[string]string   // tag → channel id
	Descriptors     []ChannelDescriptor // исходный набор (карточки UI из данных, FR-002)
}

// RenderClientConfig — сборка из встроенного набора дескрипторов.
func RenderClientConfig(src SecretsSource) (*ClientConfig, error) {
	channels, err := LoadDescriptors()
	if err != nil {
		return nil, err
	}
	return RenderFrom(channels, src)
}

// RenderOption — параметр сборки.
type RenderOption func(*renderOpts)

type renderOpts struct {
	defaultChannel string
	withTUN        bool
	engineLogPath  string
	// dialOverrides — hostname → IP (preflight-резолв, V2-037): vless+ws
	// каналы диалят по IP с SNI/Host=домену, снимая зависимость от
	// bootstrap-DNS sing-box (DoT 853 блокируется в РФ-сетях чаще всего).
	dialOverrides map[string]string
	// clashAPIPort — порт read-only метрик (clash_api) для UI (V2-048/F12).
	// 0 = метрики выключены (не рендерить experimental.clash_api) — сохраняет
	// старое поведение, когда ядро собрано без with_clash_api.
	clashAPIPort uint16
	// clashAPISecret — per-session секрет контроллера (Authorization: Bearer).
	clashAPISecret string
}

// WithClashAPI — включить в рендере experimental.clash_api (read-only
// метрики для UI, V2-048). Порт обязан быть loopback-ориентированным (127.0.0.1
// захардкожен в рендере); секрет генерирует вызывающий (per-session).
func WithClashAPI(port uint16, secret string) RenderOption {
	return func(o *renderOpts) {
		if port != 0 {
			o.clashAPIPort = port
			o.clashAPISecret = secret
		}
	}
}

// WithDefaultChannel — какой канал сделать default в селекторе (A1.4:
// переключение без повторного Start). Пусто/неизвестно → первый live-verified.
func WithDefaultChannel(id string) RenderOption {
	return func(o *renderOpts) { o.defaultChannel = id }
}

// WithTUN — добавить tun-inbound (A1.2: перехват трафика на IP-уровне).
// mixed-inbound остаётся: через него работают protected probe и управление.
// auto_route отправляет весь трафик в туннель; strict_route режет утечки мимо
// туннеля; gvisor-стек — без внешних wintun-стека и стабильнее на Windows.
func WithTUN() RenderOption {
	return func(o *renderOpts) { o.withTUN = true }
}

// WithEngineLogPath — путь файла лога ядра (V2-046): sing-box пишет строки
// с тегами компонентов (inbound/outbound/…) туда, приложение хвостит файл
// в общий журнал. Пусто (по умолчанию) = stdout (точка для тестов).
func WithEngineLogPath(path string) RenderOption {
	return func(o *renderOpts) { o.engineLogPath = path }
}

// WithDialOverrides — подстановка IP для dial по hostname (preflight-резолв).
func WithDialOverrides(m map[string]string) RenderOption {
	return func(o *renderOpts) {
		if o.dialOverrides == nil {
			o.dialOverrides = map[string]string{}
		}
		for k, v := range m {
			o.dialOverrides[k] = v
		}
	}
}

// RenderFrom — сборка из явно заданного набора (точка для контрактных тестов
// A1.4: селектор собирается из данных, а не из статического шаблона).
func RenderFrom(channels []ChannelDescriptor, src SecretsSource, opts ...RenderOption) (*ClientConfig, error) {
	o := renderOpts{}
	for _, fn := range opts {
		fn(&o)
	}
	selected := Selectable(channels)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no live-verified channels: refusing to render (fail-closed)")
	}

	// Лог ядра — в файл (V2-046): GUI-процесс не имеет консоли, stdout
	// умирает «в никуда» и handshake-ошибки REALITY/HY2 невидимы. Путь задаёт
	// приложение (EngineLogPath, nil = stdout — точка для тестов); файл
	// усекается на каждом Start (per-session, как журнал приложения).
	logOutput := "stdout"
	if o.engineLogPath != "" {
		logOutput = o.engineLogPath
	}
	cfg := &config.Config{
		// Timestamp: false при файловом выводе — форматтер sing-box даёт
		// «LEVEL сообщение» (уровень в начале строки: классификация tailer'а);
		// время в общий журнал добавляет appendLog от момента получения.
		Log: &config.LogConfig{Level: "info", Output: logOutput, Timestamp: logOutput == "stdout"},
		DNS: &config.DNSConfig{
			Servers: []config.DNSServer{
				{Tag: "tunnel-dns", Address: "tls://8.8.8.8", Detour: "proxy"},
				// Bootstrap для имени туннеля — plain UDP 1.1.1.1, НЕ DoT-853:
				// порт 853 к зарубежным резолверам блокируется в РФ-сетях чаще
				// всего (THEORY §2.1) и молча убивал канал A на старте. UDP-53
				// доступнее; подмена ответа не опасна — TLS проверит сертификат
				// домена, итоговый контроль за probe. Основной путь — вообще без
				// этого правила: preflight-резолв + IP-диал (WithDialOverrides).
				{Tag: "bootstrap-dns", Address: "1.1.1.1", Detour: "direct"},
			},
			Rules: []config.DNSRule{
				// Круговое разрешение: имя сервера туннеля — direct (bootstrap).
				{Domain: tunnelHostnames(selected), Server: "bootstrap-dns"},
				// Всё остальное — через туннель (никаких «any → direct»).
				{Server: "tunnel-dns"},
			},
			Strategy: "prefer_ipv4",
		},
		Inbounds: []config.Inbound{
			{Type: "mixed", Tag: "socks-in", Listen: "127.0.0.1", ListenPort: 1080},
		},
		Route: &config.RouteConfig{
			Rules: routeRulesWithSplit(splitSet),
			Final: "proxy",
		},
	}
	// V2-048/F10: find_process только при непустом сплите (не бесплатен).
	if len(splitSet) > 0 {
		cfg.Route.FindProcess = true
	}
	// Метрики для UI (V2-048/F12): clash_api на строгом loopback с per-session
	// секретом. Рендер НЕ решает, включать ли их — решение за вызывающим
	// (ядро без with_clash_api падает на этом блоке при Start).
	if o.clashAPIPort != 0 {
		cfg.Experimental = &config.ExperimentalConfig{
			ClashAPI: &config.ClashAPIConfig{
				ExternalController: fmt.Sprintf("127.0.0.1:%d", o.clashAPIPort),
				Secret:             o.clashAPISecret,
			},
		}
	}
	if o.withTUN {
		cfg.Inbounds = append(cfg.Inbounds, config.Inbound{
			Type:                "tun",
			Tag:                 "tun-in",
			InterfaceName:       "snowden0",
			AutoRoute:           true,
			StrictRoute:         true,
			RouteExcludeAddress: []string{"172.16.0.0/12"}, // локальная сеть остаётся прямой
			Stack:               "gvisor",
		})
	}

	var candidates []string
	channelIDs := map[string]string{}
	for _, ch := range selected {
		tag := ch.ID
		switch {
		case ch.Protocol == "vless" && ch.Transport == "tcp-reality":
			// B1 (V2-031/032): VLESS+TCP+REALITY. Vision в ядре нет (V2-031):
			// flow не задаётся; transport-блока нет (raw TCP); SNI = донор.
			uuid, err := getOrErr(src, ch, ch.UUIDRef)
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			pub, err := getOrErr(src, ch, ch.RealityPublicKeyRef)
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			sid, err := getOrErr(src, ch, ch.RealityShortIDRef)
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			// Формат-гварды (RESEARCH §5): X25519 private/public ключи REALITY —
			// base64.RawURLEncoding (43 символа, без пэйдинга); short_id — hex,
			// 0–16 символов (0–8 байт). Ловим неверное значение ДО старта движка.
			if !isRealityX25519(pub) {
				return nil, fmt.Errorf("channel %s: reality public key must be 43-char unpadded base64url (X25519), got %d chars", ch.ID, len(pub))
			}
			if len(sid) > 16 || !isHex(sid) {
				return nil, fmt.Errorf("channel %s: reality short_id must be hex, 0..16 chars", ch.ID)
			}
			cfg.Outbounds = append(cfg.Outbounds, config.Outbound{
				Type: "vless", Tag: tag,
				Server: ch.OriginServer, ServerPort: ch.Port, // REALITY идёт на origin напрямую
				UUID: uuid,
				TLS: map[string]any{
					"enabled":     true,
					"server_name": ch.Hostname, // SNI донора
					"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
					"reality":     map[string]any{"enabled": true, "public_key": pub, "short_id": sid},
				},
				// Dial-fields (V2-045, форма строкой с V2-046): TFO + короткий
				// keep-alive. REALITY-сессии простаивают дольше WS (нет mux-потока)
				// — системные TCP-таймеры (2ч+) рвут их на NAT/стейт-фильтрах;
				// 60s/20s держат запись живой. Длительности — duration-строки:
				// sing-box парсит их как badoption.Duration (int-форма = мёртвый рендер).
				TCPFastOpen:    true,
				TCPKeepAlive:   "60s",
				TCPKeepAliveIt: "20s",
			})
			// REALITY + mux — по явному opt-in (канал B1 не live-тестировался
			// с мультиплексором; консервативный дефолт, V2-037).
			if ch.Multiplex != nil && *ch.Multiplex {
				cfg.Outbounds[len(cfg.Outbounds)-1].Multiplex = &config.MultiplexConfig{
					Enabled: true, Protocol: "h2mux", Padding: true,
					MaxConnections: 4, MinStreams: 4,
				}
			}
		case ch.Protocol == "vless":
			uuid, err := getOrErr(src, ch, "vless-uuid")
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			// IP-диал за CDN (V2-037): server=IP из preflight-резолва,
			// SNI/Host остаются доменными — CDN маршрутизирует по ним,
			// а клиент не зависит от bootstrap-DNS sing-box. Без override —
			// прежнее поведение (hostname; bootstrap-правило DNS).
			server := ch.Hostname
			var headers map[string]string
			if ip := o.dialOverrides[ch.Hostname]; net.ParseIP(ip) != nil {
				server = ip
				headers = map[string]string{"Host": ch.Hostname}
			}
			ob := config.Outbound{
				Type: "vless", Tag: tag,
				Server: server, ServerPort: ch.Port,
				UUID: uuid,
				TLS: map[string]any{
					"enabled":     true,
					"server_name": ch.Hostname,
					"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
				},
				Transport: &config.Transport{
					Type: "ws", Path: "/ws",
					MaxEarlyData:        2048,
					EarlyDataHeaderName: "Sec-WebSocket-Protocol",
					Headers:             headers,
				},
			}
			// Multiplex — ТОЛЬКО явным разрешением дескриптора (V2-038, урок
			// live-инцидента V2-037): клиентский mux работает лишь если серверный
			// inbound умеет его терминировать (multiplex enabled+padding) —
			// гетерогенная ферма (envelope-доставленные каналы) молча ломалась
			// бы глобальным дефолтом (EOF на всех соединениях при живом CDN).
			if ch.Multiplex != nil && *ch.Multiplex {
				ob.Multiplex = &config.MultiplexConfig{
					Enabled: true, Protocol: "h2mux", Padding: true,
					MaxConnections: 4, MinStreams: 4,
				}
			}
			cfg.Outbounds = append(cfg.Outbounds, ob)
		case ch.Protocol == "hysteria2":
			pin := src.PinCertPath(ch.ID)
			if pin == "" {
				return nil, fmt.Errorf("channel %s: hysteria2 requires a pinned certificate (refusing to render)", ch.ID)
			}
			password, err := getOrErr(src, ch, "hy2-password")
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			obfs, err := getOrErr(src, ch, "hy2-obfs-password")
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			hy2 := config.Outbound{
				Type: "hysteria2", Tag: tag,
				Server: ch.OriginServer, ServerPort: ch.Port, // QUIC идёт на origin напрямую
				Password: password,
				Obfs:     map[string]any{"type": "salamander", "password": obfs},
				TLS: map[string]any{
					"enabled":          true,
					"server_name":      ch.Hostname,
					"certificate_path": pin,
				},
			}
			// Port hopping + brutal CC (V2-037): только когда заданы в
			// дескрипторе (сервер обязан иметь DNAT диапазона и не игнорить
			// client bandwidth).
			hy2.ServerPorts = ch.ServerPorts
			hy2.HopInterval = ch.HopInterval
			hy2.UpMbps = ch.UpMbps
			hy2.DownMbps = ch.DownMbps
			cfg.Outbounds = append(cfg.Outbounds, hy2)
		}
		candidates = append(candidates, tag)
		channelIDs[tag] = ch.ID
	}

	defaultID, err := pickDefault(selected, o.defaultChannel)
	if err != nil {
		return nil, err
	}
	cfg.Outbounds = append(cfg.Outbounds,
		config.Outbound{Type: "selector", Tag: "proxy", Outbounds: candidates, Default: defaultID},
		config.Outbound{Type: "direct", Tag: "direct"},
		config.Outbound{Type: "block", Tag: "block"},
	)

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal rendered config: %w", err)
	}
	// Двойная проверка контракта: строгий парсер обязан принять рендер.
	if _, err := config.Parse(raw); err != nil {
		return nil, fmt.Errorf("rendered config failed strict validation: %w", err)
	}
	expected, err := expectedEgressFor(selected, defaultID)
	if err != nil {
		return nil, err
	}
	return &ClientConfig{
		Raw: raw, ExpectedEgress: expected,
		ProbeTargets: probeTargetsFor(selected, defaultID),
		SelectorTags: candidates, SelectorDefault: defaultID,
		ChannelIDs: channelIDs, Descriptors: selected,
	}, nil
}

// probeTargets — цели probe выбранного канала: дескриптор обязателен к
// применению, дефолт — только при отсутствии (V2-037).
func probeTargetsFor(selected []ChannelDescriptor, defaultID string) []string {
	for _, ch := range selected {
		if ch.ID == defaultID && len(ch.ProbeTargets) >= 2 {
			return append([]string(nil), ch.ProbeTargets...)
		}
	}
	return append([]string(nil), DefaultProbeTargets...)
}

// pickDefault — default селектора: явно запрошенный канал (если он среди
// выбранных — осознанный SelectChannel, его погонит probe), иначе ПЕРВЫЙ
// live-verified, иначе первый выбранный. Default по умолчанию — всегда
// доказанный канал, даже если configured стоит раньше в файле (V2-032).
func pickDefault(selected []ChannelDescriptor, want string) (string, error) {
	if len(selected) == 0 {
		return "", fmt.Errorf("no selectable channels")
	}
	if want != "" {
		for _, ch := range selected {
			if ch.ID == want {
				return want, nil
			}
		}
	}
	for _, ch := range selected {
		if ch.ValidationStatus == "live-verified" {
			return ch.ID, nil
		}
	}
	return selected[0].ID, nil
}

// expectedEgressFor — ожидаемый egress выбранного канала: default селектора,
// если он среди выбранных; иначе первый выбранный. Ошибка невозможна при
// корректном наборе (default всегда из candidates), но fail-closed вместо
// тихого неверного ожидания (V2-032).
func expectedEgressFor(selected []ChannelDescriptor, defaultID string) (string, error) {
	for _, ch := range selected {
		if ch.ID == defaultID {
			return ch.ExpectedEgress, nil
		}
	}
	if len(selected) > 0 {
		return selected[0].ExpectedEgress, nil
	}
	return "", fmt.Errorf("no selectable channels for expected egress")
}

// isRealityX25519 — base64.RawURLEncoding X25519 ключ (43 символа без пэйдинга).
func isRealityX25519(s string) bool {
	if len(s) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(s)
	return err == nil
}

// isHex — короткий hex-идентификатор REALITY (0–16 символов).
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
} // refOf — точное сопоставление: дескриптор обязан декларировать именно ту
// ссылку, которую запрашивает рендер. Нечёткий поиск здесь опасен: пароль и
// obfs-пароль HY2 оба содержат «hy2», и нечёткое совпадение подставило бы
// пароль вместо obfs-пароля (тихая неверная конфигурация).
func refOf(ch ChannelDescriptor, want string) (string, error) {
	for _, ref := range ch.CredentialRefs {
		if ref == want {
			return want, nil
		}
	}
	return "", fmt.Errorf("descriptor %s does not declare credential ref %q", ch.ID, want)
}

// getOrErr — чтение секрета по точной ссылке с понятной ошибкой.
func getOrErr(src SecretsSource, ch ChannelDescriptor, want string) (string, error) {
	if _, err := refOf(ch, want); err != nil {
		return "", err
	}
	v, err := src.Get(want)
	if err != nil {
		return "", fmt.Errorf("secret %q unavailable: %w", want, err)
	}
	return v, nil
}

func tunnelHostnames(channels []ChannelDescriptor) []string {
	set := map[string]bool{}
	var out []string
	for _, ch := range channels {
		if !set[ch.Hostname] {
			set[ch.Hostname] = true
			out = append(out, ch.Hostname)
		}
	}
	return out
}
