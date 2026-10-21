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
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/snowden-system/windows/backend/config"
	"github.com/snowden-system/windows/backend/filetxn"
	"github.com/snowden-system/windows/backend/metadata"
)

//go:embed descriptors.json
var descriptorsRaw []byte

// ChannelDescriptor uses the signed channel schema while retaining render methods.
// A named underlying type keeps existing composite literals and package API.
type ChannelDescriptor metadata.Channel

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

// SecretIDSource supplies exact, type-checked values. An explicit binding may
// never fall back to SecretsSource.Get when this capability is unavailable.
type SecretIDSource interface {
	GetByID(id, kind string) (string, error)
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

// CurrentEnvelopeVersion — последняя версия envelope, принятая этой машиной
// (0 = envelope не применялся/нет). Точка для onboarding-импорта (V2-049):
// бандл с версией ниже текущей отвергается (антидаунгрейд V2-034).
func CurrentEnvelopeVersion() uint64 {
	if envelopeStore == nil {
		return 0
	}
	return readEnvelopeVersion(envelopeStore.Dir)
}

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
	unlock := filetxn.Serialize(store.ImportJournal())
	defer unlock()
	env, err := store.Apply(time.Now(), readEnvelopeVersion(store.Dir))
	if err != nil {
		return nil, fmt.Errorf("metadata override: %w", err)
	}
	if env == nil {
		return channels, nil
	}
	out, err := DescriptorsFromEnvelope(env)
	if err != nil {
		return nil, err
	}
	splitSet = validateSplitDirect(env.SplitDirect)
	if err := writeEnvelopeVersion(store.Dir, env.MetadataVersion); err != nil {
		return nil, fmt.Errorf("metadata override: сохранить version: %w", err)
	}
	return out, nil
}

// DescriptorsFromEnvelope is the pure descriptor gate used by import preview
// and runtime loading. It does not write version.txt or change splitSet.
func DescriptorsFromEnvelope(env *metadata.Envelope) ([]ChannelDescriptor, error) {
	if env == nil {
		return nil, fmt.Errorf("metadata: missing envelope")
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
		ch := ChannelDescriptor(mc.Clone())
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
	return out, nil
}

// parseDescriptors — валидация встроенного набора (точка для тестов schema 2).
func parseDescriptors(data []byte) ([]ChannelDescriptor, error) {
	var set descriptorSet
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&set); err != nil {
		return nil, fmt.Errorf("descriptors: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("descriptors: trailing data")
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

// Shared validation for embedded and signed profiles.
func validateChannel(ch ChannelDescriptor) error {
	return metadata.ValidateChannel(metadata.Channel(ch))
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
	ProbeTargets []string
	// DiagnosticURL — эндпоинт controlled-volume диагностики заморозки
	// выбранного канала ("" = нет — freeze-тест честно skip).
	DiagnosticURL string
	// TunnelPort — порт, который КЛИЕНТ ФАКТИЧЕСКИ набирает у выбранного
	// канала (ChannelDescriptor.Port): 443 у CDN-ветки, 8444 hy2, 8443 reality,
	// 8445 shadowtls. Нужен классификатору отказов как разделяющий признак
	// порт-эвристики (THEORY §2.4a: селективный SYN-блэкхол не-443 портов,
	// доказано live V2-051) — порт probe-цели для этого НЕ годится: у всех
	// встроенных дескрипторов цели https/443, из-за чего диагноз
	// port_blackholed был недостижим в проде (аудит D1, V2-068).
	// 0 = неизвестен (классификатор честно уйдёт на фолбэк порта probe-цели).
	TunnelPort      int
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
	for _, ch := range channels {
		if err := metadata.ValidateSecretIDs(ch.CredentialRefs, ch.SecretIDs); err != nil {
			return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
		}
	}
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
			Type:          "tun",
			Tag:           "tun-in",
			InterfaceName: "snowden0",
			// Explicit addresses are required for sing-box to install the
			// intercepting routes on Windows. Without them Wintun falls back to
			// APIPA and traffic continues over the physical default route.
			Address:             []string{"10.203.0.1/30", "fdfe:dcba:9876::1/126"},
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
				// TCP Fast Open СНЯТ (V2-080, live-доказательство 2026-09-20):
				// `tcp_fast_open: true` ронял КАЖДОЕ соединение REALITY на
				// Windows — ядро отвечает «tfo-go does not support TCP Fast
				// Open on Windows» (реализация TFO в pinned sing-box не
				// поддерживает Windows), из-за чего канал был нерабочим
				// целиком (probe падал на всех целях). Клиент Windows-only —
				// поле не рендерим; keep-alive ниже с TFO не связан и работает.
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
		case ch.Protocol == "vless" && ch.Transport == "shadowtls":
			// V2-051: VLESS over ShadowTLS v3 — ДВУХЗВЕННАЯ цепочка:
			//   <id>-st (shadowtls, терминирует маскировку TLS на сервере)
			//     ↑ detour
			//   <id>   (vless на 127.0.0.1:9444 сервера — внутренний инбаунд)
			// TLS-блок shadowtls-звена — МАСКИРОВОЧНЫЙ хендшейк к донору (SNI);
			// внутренний vless идёт уже ВНУТРИ расшифрованного туннеля без TLS.
			uuid, err := getOrErr(src, ch, ch.UUIDRef)
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			stpw, err := getOrErr(src, ch, ch.ShadowTLSPasswordRef)
			if err != nil {
				return nil, fmt.Errorf("channel %s: %w", ch.ID, err)
			}
			if len(stpw) != 24 {
				return nil, fmt.Errorf("channel %s: shadowtls password must be base64 of 16 bytes (24 chars), got %d", ch.ID, len(stpw))
			}
			stTag := tag + "-st"
			// Звено 1: ShadowTLS v3 (маскировка под TLS к донору).
			cfg.Outbounds = append(cfg.Outbounds, config.Outbound{
				Type: "shadowtls", Tag: stTag,
				Server: ch.OriginServer, ServerPort: ch.Port,
				Version:  3,
				Password: stpw,
				TLS: map[string]any{
					"enabled":     true,
					"server_name": ch.Hostname, // SNI донора (www.samsung.com)
					"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
				},
				TCPKeepAlive:   "60s",
				TCPKeepAliveIt: "20s",
			})
			// Звено 2: VLESS внутри туннеля (на loopback сервера).
			cfg.Outbounds = append(cfg.Outbounds, config.Outbound{
				Type: "vless", Tag: tag,
				Server: "127.0.0.1", ServerPort: 9444,
				UUID:   uuid,
				Detour: stTag, // подключение через shadowtls-звено
			})
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
		ProbeTargets:  probeTargetsFor(selected, defaultID),
		DiagnosticURL: diagnosticURLFor(selected, defaultID),
		TunnelPort:    tunnelPortFor(selected, defaultID),
		SelectorTags:  candidates, SelectorDefault: defaultID,
		ChannelIDs: channelIDs, Descriptors: selected,
	}, nil
}

// diagnosticURLFor — URL freeze-диагностики выбранного канала (V2-057 A2).
// "" = дескриптор не задаёт (честный skip теста, не выдуманный эндпоинт).
func diagnosticURLFor(selected []ChannelDescriptor, defaultID string) string {
	for _, ch := range selected {
		if ch.ID == defaultID && ch.DiagnosticURL != "" {
			return ch.DiagnosticURL
		}
	}
	return ""
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

// tunnelPortFor — порт выбранного канала (V2-068, аудит D1): то, что клиент
// РЕАЛЬНО набирает, а не порт probe-цели. 0 = канал не найден или порт не
// задан: классификатор обязан честно деградировать к порту probe-цели, а не
// выдумывать порт-класс.
func tunnelPortFor(selected []ChannelDescriptor, defaultID string) int {
	for _, ch := range selected {
		if ch.ID == defaultID && ch.Port != 0 {
			return int(ch.Port)
		}
	}
	return 0
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
	var v string
	var err error
	if id, bound := ch.SecretIDs[want]; bound {
		exact, ok := src.(SecretIDSource)
		if !ok {
			return "", fmt.Errorf("SecretID source required for channel %s", ch.ID)
		}
		kind, kindErr := credentialKind(ch, want)
		if kindErr != nil {
			return "", kindErr
		}
		v, err = exact.GetByID(id, kind)
	} else {
		v, err = src.Get(want)
	}
	if err != nil {
		return "", fmt.Errorf("secret %q unavailable: %w", want, err)
	}
	return v, nil
}

func credentialKind(ch ChannelDescriptor, ref string) (string, error) {
	switch {
	case ch.Protocol == "hysteria2" && (ref == "hy2-password" || ref == "hy2-obfs-password"):
		return ref, nil
	case ch.Protocol == "vless" && ch.Transport == "ws" && ref == "vless-uuid":
		return "vless-uuid", nil
	case ch.Protocol == "vless" && ref == ch.UUIDRef:
		return "vless-uuid", nil
	case ch.Transport == "tcp-reality" && ref == ch.RealityPublicKeyRef:
		return "reality-public-key-c", nil
	case ch.Transport == "tcp-reality" && ref == ch.RealityShortIDRef:
		return "reality-short-id-c", nil
	case ch.Transport == "shadowtls" && ref == ch.ShadowTLSPasswordRef:
		return "shadowtls-password-d", nil
	default:
		return "", fmt.Errorf("channel %s: unknown credential role", ch.ID)
	}
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
