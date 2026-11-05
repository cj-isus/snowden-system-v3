# Клиент–сервер: реальная совместимость и безопасный путь

## Краткий вывод

Клиентский renderer Windows уже умеет собирать четыре канала — VLESS/WS,
Hysteria2, VLESS/REALITY и VLESS/ShadowTLS — на общем pin `sing-box v1.13.19`.
Но доступный в репозитории серверный `server-combined.json` воспроизводит только
минимальные WS+HY2 inbounds и прямо помечен как неполный baseline. Он расходится
с текущим client contract по WS loopback-порту и mux, не содержит REALITY,
ShadowTLS, port-hopping/DNAT и probe endpoints, а deploy-скрипт не фиксирует
версию server binary и записывает файл без целевого OFF-09 staging/backup/atomic
apply/rollback. Поэтому source-capability клиента шире воспроизводимой
клиент–серверной пары.

Безопасный порядок применения знаний:

1. **Сначала полный versioned server render/receipt и транзакционный deploy
   (OFF-09/FR-006), без новых транспортов.**
2. **Затем pre-start compatibility compile** по существующим `metadata.Channel`,
   `RenderFrom`, strict config parser и signed profile seams.
3. **Затем probe-only lifecycle:** endpoint-authorized credentials допустимы
   только после structural compatibility; пользовательский трафик — только после
   protected probe/Ready. В SOCKS-режиме system proxy следует публиковать после
   probe; для TUN нужен отдельный staged admission под FR-009/010.
4. Активировать по одной доказуемой паре: WS baseline → HY2 direct → REALITY.
   ShadowTLS текущей TLS-less inner-VLESS композицией не допускать к protected
   credentials/user traffic; ECH не включать, пока нет endpoint-bound bootstrap.

`sing-box check`, удачный TLS, открытый listener и protected probe подтверждают
разные свойства. Ни один из них в одиночку не доказывает совместимость сторон,
сохранение transport confidentiality или отсутствие утечек.

## 1. Актуальный source cut и версии

Матрица исходно составлена 28 октября. Статусы Android уточнены по
сборке/UI 30 октября — `docs/android/LOCAL-VALIDATION-2026-10-30.md` и
частичному JNI-срезу 4 ноября — `docs/android/NATIVE-CONFORMANCE-2026-11-04.md`.
Новые исходники не наследуют прежнюю приёмку. Покупка/настройка VPS сообщены
владельцем/оператором; независимый protected-path gate остаётся открытым.

| Слой | Фактический источник | Статус знания |
|---|---|---|
| Windows core | `windows/go.mod`: Go 1.26, `sing-box v1.13.19`, `uTLS v1.8.4`, `sing-quic v0.6.4-…d83826c`, `quic-go v0.59.0-sing-box-mod.4`, `sing-shadowtls v0.2.1`, `sing-vmess v0.2.8-…3aed1551` | Точный source pin; не текущий EXE/native PASS |
| Windows renderer | `windows/backend/render/render.go`, `metadata.Channel`, built-in `descriptors.json` | Четыре render branches; строгая client-side форма |
| Android core | `android/native/source-lock.json`: core commit `b5ebaa1…`; adaptation revision4; exact debug AAR SHA закреплён | Whole core/debug APK собраны и одобрены в узкой области 30 октября; UI проверен на одном телефоне. JNI частично проверен отдельно; protected traffic/release не приняты |
| Server binary | `docs/INFRASTRUCTURE.md` декларирует sing-box 1.13.19 | Историческое описание; deploy/verify не enforce exact version/commit/artifact hash |
| Public server template | `configs/singbox/server-combined.json` | Минимальный WS+HY2 baseline, не полная production копия |
| Deployed состояние | сентябрьские FR-006/INFRASTRUCTURE и поздние операторские отчёты о купленном VPS | Исторические и reported результаты разных узлов/путей; current availability и protected path требуют отдельного receipt |

Source cut анализа: 30 публичных project/upstream-файлов, зафиксированных в
`.artifacts/planning/knowledge-application/2026-10-28/captures/Z01/source-cut.json`.
Frozen theory coverage: 75 substantive units из corpus snapshot; архивные дубли
не использовались как независимые доказательства.

## 2. Versioned feature matrix

Статусы:

- **PAIR-TEMPLATE** — обе стороны представлены в доступных source/templates,
  но ещё требуют future acceptance.
- **CLIENT-SOURCE** — клиентский caller/renderer существует, versioned server
  template отсутствует или не воспроизводит пару.
- **SOURCE-CANDIDATE** — capability есть ниже renderer/product admission.
- **PLANNED** — обозначено существующим PLAN/Android PLAN, implementation/acceptance открыты.
- **UNSUPPORTED/DEFER** — текущий контракт не должен включать свойство.

| Функция / контракт | Windows client | Android client | Доступный server template | Итоговый статус | Минимальный совместимый change / gate |
|---|---|---|---|---|---|
| Common core identity | `sing-box 1.13.19`, production tags закреплены | Тот же commit, adaptation revision4 применена и exact debug AAR одобрен; protected-path gate открыт | Версия только в документации, не в deploy receipt | **PARTIAL** | OFF-09: full server manifest с core version/commit/artifact digest; Android AND-01 exact approved AAR отдельно |
| VLESS/WS over CDN | Renderer: server IP/host, TLS SNI, WS `/ws`, early data2048, header | Graph поддерживает VLESS+WS source-only | Template имеет VLESS/WS loopback:8443; nginx example проксирует 9443; FR-006 ожидает9443 | **PAIR-TEMPLATE DRIFT** | Один полный server render задаёт 9443 и nginx relation; client/server path/header проверяются одним compatibility receipt |
| WS h2mux+padding | Descriptor `multiplex=true`; renderer h2mux/padding | Graph source позволяет transport, mux whole-runtime не принят | `server-combined.json` multiplex не задаёт; README утверждает historical deployed mux | **CLIENT-SOURCE** | Добавить exact server inbound mux в full template; несовпадение fail до Start, не EOF в probe |
| WS first-write / early data | Первый VLESS Write содержит UUID/destination и может включать payload; до2048 bytes уходят в `Sec-WebSocket-Protocol` | Source candidate, AAR/runtime unknown | Template и WS server source умеют тот же header/max; nginx его форвардит | **PAIR-SOURCE, DISCLOSURE GATE OPEN** | Зафиксировать carrier/terminator disclosure policy до credentials; probe не ретроактивное разрешение. Early-data on/off — отдельная pair capability, не confidentiality fix |
| HY2 direct 8444 | Origin IP, TLS identity/pins, Salamander, password; auto BBR при zero rates | Hysteria2/QUIC graph source candidate; AAR absent | Template HY2:8444, Salamander, TLS cert/key | **PAIR-TEMPLATE (BASE)** | Согласовать exact cert/pin/current-next-revoke, password role, obfs, ALPN/core pair; будущий static pair check + protected probe |
| HY2 TLS/0-RTT auth | Pinned caller uses `DialEarly`, но current constructors не дают session cache; auth POST после handshake | Unknown runtime | Server auth HTTP3 после QUIC/TLS | **SUPPORTED SOURCE, 0-RTT AUTH NOT ENABLED** | Не добавлять cache без replay/multi-instance contract. Сохранить пароль только post-handshake; negative/positive wire cases NOT_RUN |
| HY2 port hopping | Descriptor: 443,8444,40000–41000; hop30s | Graph supports server_ports/hop source-only | Template слушает только8444; DNAT/firewall IaC отсутствуют | **CLIENT-SOURCE / INFRA PLANNED** | OFF-09 должен описать DNAT/firewall/ranges и rollback; до receipt renderer не включает hopping ranges |
| HY2 congestion callbacks | Current pair содержит known accounting/method-set gaps; BBR/Brutal paths source-known | Unknown artifact | Same server version undocumented by machine receipt | **NEEDS REVIEW / TH-06** | Не частичный backport. Либо оставить current auto BBR с известной границей, либо обновлять совместимую `sing-quic+quic-go` пару после resource/semantic acceptance |
| VLESS/REALITY base | Renderer/strict guards/public key/short ID/uTLS; no TFO Windows | Native graph supports VLESS+REALITY source-only | Public template отсутствует; FR-006/INFRASTRUCTURE описывают historical inbound8443 | **CLIENT-SOURCE, HISTORICAL SERVER** | Full versioned Reality inbound template; exact private/public key/short-id/user flow/donor name; keep `flow=""` first |
| XTLS Vision | Реализация есть в `sing-vmess` dependency, но renderer не задаёт flow, validator отвергает nonempty flow | Graph допускает `xtls-rprx-vision`, runtime unverified | Server user-flow не представлен в template | **SOURCE-CANDIDATE / DEFER** | Исправить documentation reason «policy disabled», но не включать. Отдельная two-sided compatibility profile + uTLS/UDP/XUDP/mux gates |
| VLESS/ShadowTLS v3 current composition | Renderer создаёт ST3 leg и TLS-less VLESS на server loopback9444 | Graph не имеет ShadowTLS outbound; AAR absent | Template отсутствует; historical FR-006 описывает8445→9444 | **CLIENT-SOURCE, CONFIDENTIALITY UNSAFE FOR PROTECTED TRAFFIC** | Не повышать выше configured/selectable. Минимальный кандидат — настоящий inner TLS до VLESS X и matching loopback TLS inbound; иначе defer/retire channel |
| ShadowTLS TLS adapter | uTLS `WithSessionIDGenerator` path достигается; fallback field-loss существует, но current path его не использует | Unsupported current graph | Нет versioned server pair | **SOURCE-CANDIDATE** | Зафиксировать uTLS-only profile и reject fallback без identity parity; проверять correct X identity, не donor D |
| Real ECH | Core supports explicit/dynamic config; renderer ECH не задаёт; uTLS Chrome лишь GREASE | Graph допускает только explicit inline config source candidate | Нет ECH server keys/HTTPS/SVCB contract; CDN edge отдельно | **SOURCE-CANDIDATE / DEFER** | Только endpoint-bound attempt plan либо fixed-endpoint signed ECHConfig. Нельзя включить одним `ech.enabled`; STD/uTLS P-verifier/error parity gate |
| Bootstrap address | Preflight multi-resolver first IPv4 + 7-day cache → IP override; TLS host remains domain | Physical-network/profile epoch planned | Server template irrelevant; sing-box bootstrap DNS direct1.1.1.1 отдельно | **IMPLEMENTED SOURCE, TRUST/PROVENANCE PARTIAL** | TH-02/03/OFF-05: bind source/network/profile epoch; do not mix preflight address with separate future HTTPS/ECH RR |
| Protected probe | Manager starts engine, publishes system proxy, then runs egress/DNS/leak probe; Running only after pass | A-FR-06/AND-07 implementation open | `/ip` and `/range` required by verifier but not in full reproducible server template | **CLIENT SOURCE + SERVER ENDPOINT DRIFT** | Full server IaC owns signed/versioned probe endpoint contract. Probe result remains egress evidence, not transport confidentiality or credential permit |
| Server deploy/rollback | Client relies on exact listeners/features | Android future consumers same server | Deploy directly scp-overwrites config; no backup/stage/atomic apply/rollback; restart manual | **PLANNED OFF-09, BLOCKER** | Implement OFF-09 before any server feature activation; `sing-box check` and listeners insufficient pair proof |

## 3. First-write и допуск данных

### 3.1 Что реально уходит до probe pass

Порядок `Manager.Start` сейчас: engine Start → system proxy SetProxy → protected
probe → StateRunning. Локальный mixed inbound и proxy опубликованы до probe.
В VLESS source первый `Write` вызывает `WriteRequest` с UUID, destination и
переданным первым payload. Для WS early-data до2048 bytes этого буфера
base64-кодируются в `Sec-WebSocket-Protocol`; сервер декодирует их до передачи
VLESS inbound. TLS handshake/WS upgrade защищают от произвольного on-path
наблюдателя, но CDN terminator и разрешённый entry остаются в trust scope.

Следствия:

- probe вправе использовать **endpoint-authorized test credentials** после
  успешного structural compatibility gate;
- probe не разрешает ретроактивно уже отправленный UUID/destination/payload;
- успешный expected-egress доказывает этот echo path, но не confidentiality
  транспортной композиции и не отсутствие параллельного bypass;
- текущий WFP/fail-closed барьер защищает от direct fallback, но не заменяет
  compatibility/identity/disclosure policy выбранного proxy endpoint.

### 3.2 Минимальный admission change

**SOCKS mode:** переставить `proxy.SetProxy` после успешного lifecycle probe.
Сам probe использует явно заданный локальный SOCKS endpoint и не требует
публикации system proxy. Это минимальная правка в существующем `Manager.Start`.

**TUN mode:** простая перестановка недостаточна: auto-route может принять
пользовательский поток сразу после Engine.Start. Нужен staged permit в существующих
FR-009/010 lifecycle/protection seams: до probe допускаются только owned probe
flow/endpoint facts; user/system traffic остаётся blocked. Это platform change,
не transport implementation и не задача renderer.

## 4. Component-level изменения по этапам

### Stage 0 — P0: сделать сервер воспроизводимым (infra owner, OFF-09 / FR-006)

1. Заменить минимальный `server-combined.json` **полным** declarative render:
   nginx WS8443? — выбрать один фактический relation и закрепить FR-006 вариант
   edge443→nginx→loopback9443; VLESS/WS+mux; HY2; REALITY; ShadowTLS/inner VLESS;
   `/ip` и `/range`; firewall/DNAT hopping; `route.final=direct`.
2. Выпускать public compatibility manifest без секретов: server core version,
   template revision/hash, listeners, protocol/transport options, mux/early-data,
   TLS identity/pin generation, hopping ranges, probe endpoint semantics.
3. Deploy: stage → exact binary version check → `sing-box check` → backup → atomic
   apply → restart → post-verify → rollback. Не scp-overwrite production config.
4. До current deployment receipt — только template/source result; факт покупки
   VPS и historical listener evidence не становятся current readiness.

**Opposing guard:** `sing-box check` полезен и остаётся, но валидирует один JSON,
не совместимость с client profile, nginx, firewall/DNAT и probe responses.

### Stage 1 — P0/P1: pre-start client↔server compatibility compile

В существующих seams `metadata.Channel` → `ValidateChannel` → `RenderFrom` →
`ClientConfig` добавить/вывести immutable pair requirements, не новый transport:

- версии протокола ядра клиента/сервера и признаки поддерживаемых возможностей;
- связь listener/front, транспорт, WS path/early-data/mux;
- роль TLS: имя origin, имя донора, pin-policy, ключ/short-id REALITY;
- obfs/auth HY2, свидетельство hopping/DNAT и режим CC;
- класс конфиденциальности/раскрытия и допустимые поля первой записи;
- версия контракта пробы, её host и ожидаемый egress.

Signed profile/source и server manifest должны дать один compatibility verdict.
`configured` channel остаётся explicit-only; `live-verified` не принимается без
current tuple. Pair mismatch блокирует Engine.Start **до credentials**.
Это продолжает OFF-04/FR-005/TH-03, не вводит второй profile registry.

### Stage 2 — P0/P1: probe-only lifecycle admission (client core, FR-009/010, OFF-01/03/05)

1. Arm strict protection до render/preflight — сохранить.
2. Validate pair receipt до worker/transport credentials.
3. Start engine в probe-only admission; SOCKS system proxy ещё не опубликован.
4. Run bounded protected probe; отдельно retain security/transport/egress/endpoint
   facts и unknown.
5. Только pass + current owner/generation/network epoch → publish system proxy/TUN
   user permit и Ready.
6. Failure → teardown/rollback с прежним fail-closed contract; probe outage не
   становится causal DPI diagnosis.

### Stage 3 — включение существующих пар по одной

1. **WS baseline:** исправить 9443/mux/template drift; решить carrier disclosure и
   early-data profile. Не считать early-data off полной confidentiality.
2. **HY2 direct8444:** exact TLS pin/current-next-revoke + password/obfs; hopping
   остаётся off до DNAT/IaC receipt. Не включать session cache/0-RTT auth.
3. **REALITY no-flow:** full server template, key/short-id/donor contract, no TFO;
   затем отдельный current probe. Vision не включать автоматически.
4. **ShadowTLS:** текущий TLS-less inner VLESS не допускать для protected data.
   Единственный минимальный кандидат в текущих primitives — inner TLS on C→X с
   matching server loopback TLS identity; если pair/identity не замыкается — defer.
5. **ECH:** последним, после endpoint-bound SVCB/fixed-endpoint contract и
   STD/uTLS rejection/public_name/observation parity. GREASE не acceptance.
6. **QUIC upstream fix:** отдельный TH-06 dependency-pair upgrade, не частичный
   backport и не blocker базового HY2 до измеренного effect.

### Stage 4 — Android parity, не перенос Windows статуса

Android использует pinned `v1.13.19` с применённой adaptation revision4 и
одобренным exact debug AAR; историческая сборка/UI и частичный JNI-срез имеют
раздельные датированные evidence. NativeGraphSecurity допускает VLESS/REALITY/HY2
и explicit ECH config, но не ShadowTLS. Поэтому:

- transport compatibility и protected-path rows остаются **unverified**;
  build/UI/JNI результаты признаются только в своей точной области;
- compatibility manifest/attempt-plan contract можно переиспользовать как schema,
  но execution owner — Android service/corebridge;
- необходимые existing gates: AND-01 exact artifact continuity, AND-04 accepted profile,
  AND-05 physical epoch/handover, AND-06 whole graph/strict owner, AND-07 protected
  probes, AND-11 resources, AND-15 native/device acceptance;
- нельзя объявлять server compatibility по одному Kotlin parser success.

## 5. Минимальные контракты

### 5.1 ServerCompatibilityReceipt

Минимальный immutable output server render/deploy, без credentials:

```text
server_core_version/commit/artifact_digest
template_revision/hash
channel_id → listener/front, protocol, transport, flow, mux,
             early_data mode, TLS identity role, obfs/hopping/CC,
             probe_contract_revision
applied_generation + valid_from/valid_until
```

Receipt не доказывает live path; он доказывает, что client profile и server
render относятся к одному feature contract. Подпись/authority остаётся в
существующей profile/delivery системе.

### 5.2 ChannelAttemptPlan

Renderer/Manager получает не только raw JSON, а frozen requirements:

```text
profile/source/version + channel id + selected server receipt
credential role IDs + allowed first-write/disclosure class
bootstrap source/network epoch + TLS identity/pin/public_name
probe inputs + expected egress + lifecycle owner/generation/validity
```

Late network/profile/receipt change отзывает plan; следующий Start рендерит заново.
Не хранить реальные secret values в receipt/diagnostics.

### 5.3 ProbeOutcome

Сохранять раздельно:

- совместимость пары до запуска: статический анализ и отдельно разрешённая проверка;
- установление TLS/транспорта;
- наблюдение endpoint/egress;
- область свидетельства о прямой утечке;
- наблюдение резолвера;
- допуск приложения/пользовательского трафика.

`AllPassed` не становится доказательством hidden transport confidentiality,
server-template version или отсутствия всех leaks.

## 6. Future acceptance — всё NOT_RUN

| Gate | Положительный случай | Сильный отрицательный случай |
|---|---|---|
| Version pair | client/server exact core+feature receipt matches | same JSON accepted by different/unknown server core |
| WS | 443→nginx→9443, matching path/mux/early mode | 8443/9443 drift; mux one side; nginx drops early header |
| First write | only probe identity before Ready; user traffic blocked | local/system client writes during Starting; UUID/destination in early header |
| HY2 | exact pin/current-next, auth, obfs, direct8444 | wrong cert/pin/password; session cache unexpectedly enables early auth |
| Hopping | every range has DNAT/firewall/rollback receipt | client hops to unopened40000–41000 or 443 conflict |
| REALITY | no-flow pair, key/short-id/donor, wrong key/name fails | server template missing; TFO rendered on Windows; unsupported flow |
| ShadowTLS | inner TLS authenticates X and protects UUID/destination/payload | donor TLS only; D identity substituted for X; fallback loses verifier |
| ECH | endpoint-bound E, real acceptance, O/P verification, bounded retry | GREASE-only; stale E/new address; wrong public_name verifier |
| Probe | expected egress + DNS + mode-correct leak scope, current epoch | identical remote receipt from leak world; endpoint cache response |
| Lifecycle | no user admission before probe, late results rejected | system proxy/TUN traffic available during Starting |
| Server apply | backup/stage/check/atomic apply/health/rollback | scp overwrite then failed restart, no known previous config |
| Android | approved source-adapted AAR + same receipt/profile + device proof | parser-only success or stock AAR substituted |

## 7. Что не стоит делать

- Не включать ECH одним полем и не считать GREASE privacy success.
- Не включать Vision только потому, что код найден в dependency closure.
- Не считать ShadowTLS donor TLS шифрованием post-switch VLESS.
- Не обновлять один `sing-quic`/`quic-go` callback без совместимой пары.
- Не использовать public baseline template для production replacement.
- Не повышать `configured` до `live-verified` по старому сентябрьскому probe.
- Не добавлять новый transport/provider/DB/shared layer ради матрицы.
- Не трактовать successful TCP/TLS/HTTP/probe как разрешение credentials/user traffic.

## 8. Остаток и владельцы

| Остаток | Principal owner / existing plan |
|---|---|
| Полный server render/deploy/receipt | infra + owner, OFF-09 / FR-006 |
| Pair compatibility compiler/profile schema | Windows profile+render, OFF-04 / FR-005 / TH-03 |
| Probe-only admission и traffic permit | client core/network safety, FR-009/010, OFF-01/03/05 |
| WS/HY2/REALITY pair rollout | transport + infra, TH-05/06 после P0 gates |
| ShadowTLS inner confidentiality | transport/security, TH-05; defer current pair |
| ECH endpoint-bound plan | transport/DNS/TLS, TH-02/03/05; source candidate only |
| Android parity | AND-01/04/05/06/07/11/15; no Windows status inheritance |
| Native/live/current VPS evidence | OFF-09 / AND-15, отдельное разрешение |

Ни одна будущая приёмка в этом planning-срезе не запускалась. Документ применяет
существующие знания к source/config/plan; он не меняет scientific acceptance,
product status, server state или release readiness.
