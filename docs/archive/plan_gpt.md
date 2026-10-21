> [АРХИВ 2026-09-17, V2-066] — НЕ источник правды (первоисточник-консультация).
> Перенесён из docs/plan_gpt.md: консолидация планов в один (PLAN.md §1, §5).
> Дистиллят этого документа — docs/archive/PLAN-V3.md; реализованные пункты
> закрыты записями журнала PLAN.md §6 (V2-056…V2-065).

Да. Здесь уже имеет смысл перейти от «дорожной карты» к **полноценному техническому master plan v3**, где каждый этап отвечает на четыре вопроса:

1. Что строим.
2. Почему это нужно именно с учётом твоей теории.
3. Как это устроено внутри кода.
4. По каким объективным критериям этап считается завершённым.

У текущего `PLAN.md` уже очень сильная основа: `Manager → Engine → ProtectedProbe`, strict config validation, fail-closed, TUN/SOCKS, per-channel descriptors, metadata envelope, failover-on-start и уже подтверждённое переключение VLESS ↔ HY2.   Поэтому следующий план должен не разрушать это ядро, а надстроить над ним **систему наблюдения, диагностики и адаптации**.

Исследования Xue et al. и Wu et al. подтверждают именно необходимость такого подхода: encrypted/obfuscated traffic может классифицироваться по структурным и статистическим признакам, но эти работы **не доказывают конкретный механизм TSPU**. Xue показывает protocol-agnostic detection через encapsulated TLS handshakes и ограничения простого padding/multiplexing; Wu исследует passive classification fully encrypted traffic на GFW через набор эвристик. ([USENIX][1])

Ниже — версия, которую я бы уже использовал как основу для нового `PLAN.md`.

---

# SNOWDEN.SYSTEM — ADAPTIVE TRANSPORT MASTER PLAN v3

## 0. Главная концепция

Проект больше нельзя мыслить как:

```text
VPN client
    ├── VLESS
    ├── HY2
    └── REALITY
```

Правильная модель:

```text
                    ┌───────────────────────┐
                    │      NetworkFacts     │
                    │ ISP / ASN / type      │
                    │ region / IPv4 / IPv6   │
                    │ DNS environment        │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │     Observation       │
                    │ TCP / TLS / DNS       │
                    │ RTT / resets / bytes   │
                    │ TTFB / completion      │
                    │ egress / leak          │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │   Failure Classifier  │
                    │                       │
                    │ transport             │
                    │ path                  │
                    │ server                │
                    │ DNS                   │
                    │ freeze                │
                    │ reset                 │
                    │ blackhole              │
                    │ throttling             │
                    │ correlated             │
                    │ unknown                │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │    Channel Health     │
                    │                       │
                    │ A = health/confidence  │
                    │ B = health/confidence  │
                    │ C = health/confidence  │
                    │ failure domains       │
                    └───────────┬───────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │  AdaptiveController   │
                    │                       │
                    │ stay                  │
                    │ reprobe               │
                    │ switch                │
                    │ recover               │
                    │ blocked               │
                    └───────────┬───────────┘
                                │
                                ▼
                         ┌────────────┐
                         │  Manager   │
                         └──────┬─────┘
                                │
                 ┌──────────────┼──────────────┐
                 ▼              ▼              ▼
             Channel A      Channel B      Channel C
```

Главное изменение:

> **Transport — это уже не решение. Это один из инструментов, которым управляет Controller.**

---

# 1. Что уже считается фундаментом

Это не надо переписывать.

## 1.1. Lifecycle

Оставляем:

```text
Start
→ render
→ validate
→ engine.Start
→ protected probe
→ running
```

и:

```text
Stop
→ stop engine
→ restore proxy
→ stopped
```

Текущая архитектура уже правильно разделяет `engineCtx` и короткоживущие operation contexts; это было найдено как реальный P0-дефект и исправлено. 

---

# 2. Что именно сейчас не хватает

Текущий план уже умеет:

```text
"Канал B доступен?"
```

Но новый план должен уметь отвечать:

```text
Почему B не доступен?

Это проблема:
- сервера?
- сети?
- транспорта?
- egress?
- DNS?
- return path?
- конкретной последовательности?
- временного колебания?
- коррелированной деградации?

Что сейчас вероятнее всего произойдёт с A, B и C?

Какой канал имеет наименьшее пересечение failure domains?

Когда стоит переключаться?

Когда стоит НЕ переключаться?

Когда попробовать старый канал снова?
```

Это и есть **P0 AdaptiveController**.

---

# 3. Новая иерархия компонентов

Предлагаю сделать архитектуру:

```text
core/
├── engine/
│   ├── engine.go
│   └── selector.go
│
├── manager/
│   └── manager.go
│
├── probe/
│   ├── protected.go
│   ├── diagnostic.go
│   ├── signatures.go
│   └── evidence.go
│
├── network/
│   ├── facts.go
│   ├── resolver.go
│   └── path.go
│
├── health/
│   ├── channel.go
│   ├── history.go
│   └── scoring.go
│
├── classify/
│   ├── deterministic.go
│   ├── features.go
│   └── confidence.go
│
├── adaptive/
│   ├── controller.go
│   ├── state.go
│   ├── policy.go
│   ├── incident.go
│   ├── recovery.go
│   └── candidate.go
│
├── metadata/
│   ├── envelope.go
│   └── policy.go
│
└── telemetry/
    ├── event.go
    ├── store.go
    └── privacy.go
```

Зависимости:

```text
UI
 ↓
Facade
 ↓
Manager
 ↓
AdaptiveController
 ↓
Probe / Health / Classifier
 ↓
Engine
```

Но Controller **не должен управлять Engine напрямую**.

Только:

```text
Controller → Manager.SwitchChannel(...)
```

Это сохраняет уже существующее правило, что controller применяет решение через `Manager`. 

---

# 4. Channel Descriptor v2

Текущий descriptor содержит транспорт, server reference, port, SNI, credential reference, validation status и evidence tuple. 

Новый descriptor должен описывать ещё и **ожидаемую модель отказа**.

## 4.1. Пример

```yaml
id: channel-a-vless

transport:
  protocol: vless
  carrier: websocket
  security: tls
  network: tcp

endpoint:
  server_ref: server-a
  port: 443
  sni_ref: sni-a

failure_domain:
  transport_family: tcp
  carrier_family: cdn
  origin_id: origin-a
  asn_class: cdn
  provider_class: cloud-edge
  network_layer: l3-l4

behavior:
  long_lived: false
  multiplex: true
  short_connection_preferred: true
  udp: false

probe_profile:
  cheap_probe: true
  diagnostic_probe: true
  max_payload: ...
  timeout_ms: ...

expected_failure_classes:
  - SilentFreeze
  - TlsPolicing443
  - ReturnPathFailure
```

Теперь Controller знает:

```text
A — не просто VLESS.
A — TCP/WS/CDN channel с определённым набором failure domains.
```

---

# 5. Failure Domain Model

Это один из самых важных новых компонентов.

Пусть канал описывается:

```text
FD(channel) =
{
    transport,
    protocol,
    carrier,
    origin,
    IP,
    ASN,
    provider,
    port-family,
    network-layer
}
```

Например:

### A

```text
TCP
VLESS
WS
TLS
Cloudflare
origin-1
ASN-CDN
443
```

### B

```text
TCP
VLESS
REALITY
raw TCP
origin-2
ASN-2
8443
```

### C

```text
UDP
QUIC
HY2
origin-1
ASN-1
8444+
```

Поэтому A и C:

```text
transport ≠
protocol ≠
network layer ≠
```

но:

```text
origin = same
ASN = same
VPS = same
```

То есть они **частично независимы**, но не полностью.

Это гораздо точнее, чем просто:

```text
A ≠ B ≠ C
```

---

# 6. Формальная независимость каналов

Ввести функцию:

$$
Independence(A,B)
$$

которая зависит от пересечения failure domains.

Например:

```text
same VPS        = -0.30
same ASN        = -0.20
same transport  = -0.20
same carrier    = -0.20
same port class = -0.05
same protocol   = -0.05
```

Это не научная универсальная формула — это **внутренняя инженерная policy model**, которую позже можно калибровать на данных.

Например:

```text
A ↔ B = 0.85
A ↔ C = 0.42
B ↔ C = 0.78
```

И Controller будет знать:

> Если A умер, C не обязательно лучший следующий канал только потому, что он другой protocol. Он может иметь слишком много общего инфраструктурного failure domain.

---

# 7. NetworkContext

Существующая теория абсолютно правильно требует учитывать оператор/тип сети, потому что распределённая архитектура TSPU означает, что поведение может различаться по сети. 

Новая структура:

```go
type NetworkContext struct {
    ISPClass        string
    ASN             uint32

    ConnectionType  ConnectionType

    RegionClass     string

    IPv4Available   bool
    IPv6Available   bool

    DNSMode         DNSMode

    CapturedAt      time.Time
}
```

Но не надо хранить избыточные идентификаторы.

---

# 8. Network epoch

Ввести понятие:

```text
NetworkEpoch
```

Например:

```text
2026-09-16T10
```

или:

```text
epoch-123
```

Когда меняется:

```text
ISP
Wi-Fi ↔ LTE
IPv4 ↔ IPv6
DNS environment
```

создаётся новый epoch.

Почему это нужно?

Потому что:

```text
A failed 10:00
```

и:

```text
A failed 14:00
```

не обязательно две одинаковые события.

---

# 9. ProtectedProbe v2

Существующий protected probe оставляем как **L1 evidence gate**:

```text
DNS
+
HTTPS #1
+
HTTPS #2
+
egress
+
direct leak
```

Такой подход уже был live-проверен и исправлялся после обнаружения ложных/дублирующих шагов. 

Но добавить уровни.

## L0 — Connectivity

```text
DNS
TCP connect
TLS handshake
```

## L1 — Protection

```text
HTTPS
egress
direct leak
```

## L2 — Diagnostics

```text
small payload
medium payload
multiple connections
response completion
freeze detection
```

---

# 10. Почему это важно

Сейчас:

```text
probe failed
```

может означать всё.

Новая система:

```text
L0 fail
→ transport/path problem

L0 pass, L1 fail
→ data-plane problem

L1 pass, L2 fail
→ behavioral degradation
```

---

# 11. Probe должен возвращать не bool

Не:

```go
Probe() bool
```

а:

```go
type ProbeResult struct {
    DNS DNSEvidence
    TCP TCPEvidence
    TLS TLSEvidence
    HTTP HTTPEvidence
    Egress EgressEvidence
    Leak LeakEvidence

    Diagnostic *DiagnosticEvidence

    Duration time.Duration
}
```

---

# 12. TCP Evidence

```go
type TCPEvidence struct {
    DialStarted bool
    Connected   bool

    ConnectDuration time.Duration

    RSTSeen bool
    FINSeen bool

    Timeout bool
}
```

---

# 13. TLS Evidence

```go
type TLSEvidence struct {
    Started bool
    Completed bool

    Duration time.Duration

    CertificateValid bool

    TLSVersion string

    Failure string
}
```

---

# 14. HTTP Evidence

```go
type HTTPEvidence struct {
    Started bool
    HeadersReceived bool

    StatusCode int

    TTFB time.Duration

    BytesReceived uint64
    ExpectedBytes uint64

    Completed bool

    Timeout bool
}
```

---

# 15. Freeze Signature

Вот это особенно важно для твоей теории.

У тебя уже есть наблюдения, связанные с freeze после определённого объёма данных, включая ~16 KB / ~25 packets. 

Поэтому:

```go
type FreezeSignature struct {
    Detected bool

    BytesBeforeFailure   uint64
    PacketsBeforeFailure uint32

    DurationBeforeFailure time.Duration

    Direction Direction

    RST bool
    FIN bool

    LastProgress time.Time
}
```

---

# 16. Почему `16 KB` нельзя превращать в hardcoded rule

Очень важно.

Нельзя:

```go
if bytes == 16384 {
    return SilentFreeze
}
```

Потому что теория сама показывает, что наблюдаемый диапазон и условия могут меняться.

Правильнее:

```text
freeze_offset ≈ expected threshold
```

с confidence.

Например:

```text
8–20 KB → weak indication
14–20 KB repeated → strong indication
```

Это будет статистическая сигнатура.

---

# 17. Failure Taxonomy v2

Сделать двухмерную систему.

## Причина уровня инфраструктуры

```text
TransportFailure
PathFailure
ServerFailure
DNSFailure
EgressFailure
ReturnPathFailure
```

## Наблюдаемое поведение

```text
ResetInjected
SilentFreeze
PortBlackholed
TlsPolicing
DnsSubverted
Throttling
WhitelistMode
CorrelatedOutage
Unknown
```

Например:

```text
RootCause:
    PathFailure

ObservedBehavior:
    SilentFreeze

Confidence:
    0.84
```

---

# 18. Никогда не хранить в classifier

```text
TSPU = true
```

Вместо этого:

```text
InterferenceSuspected = true
```

плюс:

```text
Evidence:
    repeated freeze
    direct path healthy
    multiple channels affected
    deterministic boundary
```

Потому что доказан класс отказа, а не внутренний алгоритм конкретного censor.

Это особенно важно с учётом Xue/Wu: исследования демонстрируют возможности определённых систем классификации, но не дают доказательства конкретного алгоритма российской системы. ([USENIX][1])

---

# 19. Deterministic Failure Classifier

Первоначально никакого ML.

```text
if !SYN:
    PortBlackholed

if SYN && TLS handshake && RST:
    ResetInjected

if TLS success &&
   bytes_progress &&
   timeout &&
   no_RST:
    SilentFreeze

if DNS mismatch:
    DnsSubverted

if multiple independent channels fail simultaneously:
    CorrelatedOutage

otherwise:
    Unknown
```

Но все условия должны иметь confidence.

---

# 20. Confidence model

Каждый сигнал:

```text
signal weight
```

Например:

```text
RST after TLS      = 0.50
repeat x2          = +0.20
direct path OK     = +0.15
other channel OK   = +0.15
```

И:

```text
confidence = min(sum, 1.0)
```

Позже заменить на calibrated classifier.

---

# 21. Почему confidence важнее самой категории

Пример:

```text
SilentFreeze
confidence = 0.91
```

Controller может реагировать.

А:

```text
SilentFreeze
confidence = 0.47
```

не должен менять канал.

То есть:

```text
classification
+
confidence
=
decision input
```

а не просто:

```text
classification
=
decision
```

---

# 22. AdaptiveController state machine

Теперь сам Controller.

```text
                  ┌───────────┐
                  │  HEALTHY  │
                  └─────┬─────┘
                        │
                   probe fail
                        │
                        ▼
                ┌──────────────┐
                │   SUSPECTED  │
                └──────┬───────┘
                       │
                  fast reprobe
                  /           \
               PASS           FAIL
                │               │
                ▼               ▼
             HEALTHY       CLASSIFYING
                                │
             ┌──────────────────┼─────────────────┐
             ▼                  ▼                 ▼
          PATH FAIL          FREEZE            RESET
             │                  │                 │
             └──────────────────┼─────────────────┘
                                ▼
                           DEGRADED
                                │
                                ▼
                           CANDIDATE
                                │
                        ┌───────┴────────┐
                        ▼                ▼
                      SWITCH          RECOVER
                        │
                ┌───────┴───────┐
                ▼               ▼
              PASS             FAIL
                │               │
                ▼               ▼
             HEALTHY       NEXT CANDIDATE
                                   │
                                   ▼
                                 BLOCKED
```

---

# 23. SUSPECTED state

Очень важный промежуточный state.

Одного failure недостаточно.

```text
HEALTHY
   ↓
one failure
   ↓
SUSPECTED
```

Это решает уже наблюдавшуюся проблему transient network failure.

В текущей реализации watchdog уже был изменён так, чтобы первый провал не убивал туннель, а запускал fast re-probe. 

Теперь эту логику нужно формализовать как полноценное состояние.

---

# 24. DEGRADED

Переход:

```text
SUSPECTED
→ 2–3 consistent failures
```

либо:

```text
high-confidence failure
```

Например:

```text
confidence >= 0.85
```

Тогда Controller имеет право выбирать другой канал.

---

# 25. SWITCHING

Сейчас у тебя уже появился очень хороший механизм бесшовного switch через selector, с Reload fallback. 

Новый Controller должен использовать:

```text
Manager.SwitchChannel(...)
```

и ничего больше.

То есть:

```text
Controller
   ↓
candidate selection
   ↓
Manager.SwitchChannel
   ↓
Engine selector
   ↓
Probe
```

---

# 26. Switch must have rollback

Сценарий:

```text
A = degraded
B = candidate
```

Controller пытается B.

B не проходит.

Нельзя просто оставить:

```text
VPN = dead
```

Если A ещё usable.

Алгоритм:

```text
A healthy
↓
try B
↓
B fail
↓
probe A
↓
A pass
↓
restore A
```

Если:

```text
A fail
B fail
C fail
```

то:

```text
BLOCKED
```

Это логическое расширение уже существующей rollback semantics.

---

# 27. Candidate scoring

Теперь самое интересное.

Каждый кандидат получает score.

Например:

$$
S(c)=
w_hH(c)+
w_rR(c)+
w_iI(c)+
w_eE(c)-
w_fF(c)-
w_aA(c)
$$

где:

* `H` — current health;
* `R` — recency;
* `I` — independence;
* `E` — evidence confidence;
* `F` — failure streak;
* `A` — age of evidence.

---

# 28. Independence должна иметь большой вес

Это фундаментальное изменение.

Пусть:

```text
A:
health=0.92
independence=0.2

B:
health=0.78
independence=0.9
```

При отказе A:

```text
B может быть предпочтительнее A-like profile
```

потому что нам нужна не самая высокая историческая стабильность, а **минимально коррелированный survivor**.

---

# 29. Evidence age

`live-verified` должен стареть.

Например:

```text
0–10 min   → strong
10–60 min  → medium
1–6 h      → weak
>6 h       → stale
```

Не обязательно именно такие интервалы; это policy.

Но принцип обязательный.

---

# 30. Health score

Можно использовать:

$$
Health =
0.40 \cdot RecentSuccess
+
0.25 \cdot ConsecutiveSuccess
+
0.15 \cdot LowLatency
+
0.20 \cdot EvidenceRecency
$$

Но первоначально я бы даже **не вычислял один score**.

Лучше хранить:

```text
health vector
```

```text
Availability
Latency
Stability
Evidence confidence
Freshness
Independence
```

А score считать только непосредственно перед выбором.

---

# 31. Incident Model

Каждая серия деградации должна получать `incident_id`.

Например:

```text
incident-20260916-001
```

Структура:

```go
type Incident struct {
    ID string

    StartedAt time.Time
    EndedAt *time.Time

    NetworkEpoch string

    Channels []ChannelIncident

    Correlated bool

    PrimaryClass FailureClass

    Confidence float64

    Attempts int

    Resolution Resolution
}
```

---

# 32. Incident lifecycle

```text
OPEN
 ↓
OBSERVING
 ↓
CLASSIFIED
 ↓
MITIGATING
 ↓
RECOVERED
```

или:

```text
OPEN
 ↓
CLASSIFIED
 ↓
MITIGATING
 ↓
BLOCKED
```

---

# 33. Incident clustering

Три почти одновременных отказа:

```text
A 10:01:12
B 10:01:14
C 10:01:18
```

не должны стать тремя incident'ами.

Если:

```text
same network epoch
+
time window <= N sec
+
multiple channels
```

то:

```text
correlated incident
```

---

# 34. Correlated outage

Состояние:

```text
CORRELATED_OUTAGE
```

не должно автоматически означать censorship.

Это означает:

> несколько каналов отказали при похожем времени/контексте.

Варианты:

```text
shared origin
shared provider
shared path
local connectivity
operator event
unknown
```

---

# 35. Shared-failure graph

Можно представить систему как граф:

```text
           Network
              │
       ┌──────┼───────┐
       │      │       │
      ISP    DNS     IPv4
       │
 ┌─────┼─────────────┐
 │     │             │
 A     B             C
 │     │             │
 CF   VPS-B         VPS-A
 │     │             │
TCP   TCP           UDP
```

Если одновременно падают:

```text
A + C
```

и они имеют:

```text
VPS-A common
```

то это сильный намёк на common failure domain.

Если:

```text
A + B
```

но VPS/ASN разные, а network context один:

```text
network-level correlation
```

становится вероятнее.

---

# 36. Recovery subsystem

После переключения:

```text
A failed
B active
```

A нельзя удалять из памяти.

Он переходит:

```text
DEGRADED
```

Потом:

```text
RECOVERY_PENDING
```

И периодически:

```text
small probe
```

Например:

```text
T+5m
T+15m
T+30m
T+60m
```

с exponential backoff.

---

# 37. Recovery score

Если:

```text
A success ×3
```

его health постепенно возвращается.

Но не мгновенно:

```text
BLOCKED
↓
RECOVERING
↓
PROBATION
↓
HEALTHY
```

Это лучше, чем:

```text
one success
↓
healthy
```

---

# 38. Canary channels

Новый важный механизм.

Когда A активен:

```text
B
C
```

могут получать **cheap diagnostic probes**.

Это не значит использовать их для реального трафика.

Только:

```text
"жив ли канал?"
```

Таким образом Controller постоянно знает:

```text
A = healthy
B = healthy
C = unhealthy
```

до того, как понадобится переключение.

---

# 39. Почему это лучше текущего failover

Текущий механизм:

```text
A fails
→ try B
→ try C
```

Новый:

```text
A active
B health = known
C health = known

A fails
→ no need to blindly try everything
→ choose best current survivor
```

Это уменьшает время failover.

---

# 40. Cheap / normal / diagnostic probes

### Cheap

```text
TCP
TLS
```

### Normal

```text
HTTPS
egress
DNS
```

### Diagnostic

```text
controlled payload
freeze profile
multiple connections
```

Диагностический уровень запускается только при необходимости.

---

# 41. Probe budget

Controller должен иметь бюджет.

Например:

```text
max normal probes / min
max diagnostic probes / incident
max recovery probes / hour
```

Это нужно не только ради производительности — частое probing само изменяет traffic behavior.

У Xue как раз важен behavioural/traffic-shape аспект: даже padding и multiplexing не убирают некоторые признаки, связанные с bursts и RTT. ([USENIX][1])

---

# 42. Self-hosted probe targets

В текущем плане уже было правильное решение убрать постоянную зависимость от сторонних `ipify/ifconfig` и сделать свои probe endpoints. 

Дальше:

каждый канал получает собственные:

```text
probe targets
```

Например:

```text
A:
  /probe/a
  /health/a

B:
  /probe/b
  /health/b

C:
  /probe/c
```

Это позволяет отделить:

```text
channel failure
```

от:

```text
third-party endpoint failure
```

---

# 43. Но один origin для всех probe — тоже проблема

Если:

```text
A → origin A
B → origin B
C → origin A
```

тесты A и C коррелированы.

Поэтому probe target тоже должен быть частью failure domain.

---

# 44. Egress Layer

Это должен быть отдельный subsystem:

```text
TransportHealth
PathHealth
EgressHealth
```

Потому что ты уже реально обнаружил:

```text
tunnel alive
+
egress IP degraded
```

и исправил это через WARP на серверной стороне. 

Следовательно:

```text
Transport != Egress
```

---

# 45. EgressHealth

```go
type EgressHealth struct {
    IPClass string

    Reachability ReachabilityState

    HTTPReachability map[string]TargetStatus

    ReputationClass string

    LastChecked time.Time
}
```

Не надо пытаться превращать reputation в абсолютное число.

Например:

```text
CLEAN
NEUTRAL
CHALLENGED
UNUSABLE_FOR_TARGET
UNKNOWN
```

---

# 46. TransportHealth

```text
HEALTHY
DEGRADED
FAILED
UNKNOWN
```

---

# 47. PathHealth

```text
STABLE
LOSSY
HIGH_LATENCY
ASYMMETRIC
BLACKHOLED
UNKNOWN
```

И тогда:

```text
ChannelStatus {
    TransportHealth
    PathHealth
    EgressHealth
}
```

---

# 48. Почему это важно архитектурно

Если:

```text
transport = healthy
egress = challenged
```

твой Controller должен **не менять транспорт**.

Он должен менять:

```text
egress route
```

например:

```text
direct egress
→ WARP
```

Это совершенно другой action.

То есть появляется:

```text
AdaptationAction
```

---

# 49. AdaptationAction

```text
KeepCurrent
Reprobe
SwitchChannel
SwitchEgress
ChangeProbeProfile
EnterRecovery
EnterBlocked
```

Важно:

```text
failure class
```

определяет:

```text
action family
```

---

# 50. Action Matrix

Например:

| Failure           | Primary action                            |
| ----------------- | ----------------------------------------- |
| ResetInjected     | alternate transport                       |
| SilentFreeze      | short-lived profile / alternate transport |
| PortBlackholed    | different port/path                       |
| DnsSubverted      | bootstrap fallback                        |
| ReturnPathFailure | alternate channel/network                 |
| EgressDegraded    | alternate egress                          |
| ServerFailure     | different origin                          |
| Correlated outage | wait/reprobe                              |
| Unknown           | diagnostic probe                          |
| WhitelistMode     | BLOCKED                                   |

Это и есть **policy engine**.

---

# 51. Controller должен иметь Policy, а не hardcode

Создать:

```yaml
controller_policy:
  reprobe:
    failures_before_reprobe: 1

  switch:
    failures_before_switch: 2
    high_confidence_threshold: 0.85

  cooldown:
    initial: 30s
    multiplier: 2
    max: 10m

  recovery:
    initial: 5m
    multiplier: 2
    max: 6h
```

Подписанный metadata envelope уже умеет безопасно доставлять policy-данные с anti-downgrade/expiry semantics. 

---

# 52. Однако controller policy нельзя полностью доверять metadata

Правильно разделить:

```text
hard safety invariants
```

и:

```text
remote policy
```

Hard invariants:

```text
direct запрещён
insecure запрещён
invalid channel запрещён
unknown transport запрещён
```

Они **никогда не могут быть переопределены envelope**.

---

# 53. Metadata v2

Теперь envelope может содержать:

```text
channels
controller policy
probe profiles
feature flags
minimum client version
channel retirement
```

Но:

```text
controller cannot override hard invariants
```

---

# 54. Versioned policy

Каждое решение должно знать:

```text
engine_version
channel_revision
controller_policy_version
probe_policy_version
metadata_version
```

Тогда incident воспроизводим.

---

# 55. Evidence tuple v2

Сделать примерно:

```json
{
  "channel": "A",
  "network_epoch": "E42",
  "timestamp": "...",

  "l0": {
    "tcp": "pass",
    "tls": "pass"
  },

  "l1": {
    "dns": "pass",
    "https": "fail",
    "egress": "pass",
    "leak": "pass"
  },

  "l2": {
    "bytes_before_failure": 16384,
    "rst": false,
    "timeout": true
  },

  "classification": {
    "class": "SilentFreeze",
    "confidence": 0.89
  },

  "health": {
    "transport": "degraded",
    "path": "suspect",
    "egress": "healthy"
  }
}
```

---

# 56. Telemetry model

Нужна **event log**, а не просто текстовый log.

Типы событий:

```text
ProbeStarted
ProbeCompleted
FailureObserved
FailureClassified
HealthChanged
ControllerDecision
SwitchStarted
SwitchSucceeded
SwitchFailed
RecoveryStarted
RecoverySucceeded
MetadataAccepted
MetadataRejected
```

---

# 57. Пример event sequence

```text
10:32:00 ProbeStarted A

10:32:01 ProbeCompleted
A
TLS=pass
HTTP=fail

10:32:01 FailureObserved
bytes=16384
RST=false

10:32:01 FailureClassified
SilentFreeze
confidence=0.88

10:32:02 ControllerDecision
action=SwitchChannel
candidate=B

10:32:02 SwitchStarted
A→B

10:32:03 ProbeCompleted
B=pass

10:32:03 SwitchSucceeded

10:32:03 IncidentResolved
```

Это уже материал для будущего ML.

---

# 58. Что НЕ собирать

Не нужны:

```text
URL history
payload
cookies
query parameters
credentials
full packet captures
personal IP history
```

Для classifier достаточно:

```text
timing
sizes
result
network class
transport class
failure class
channel id
revision
```

---

# 59. Dataset

Хранить не:

```text
user session
```

а:

```text
ProbeEvent
```

Например:

```json
{
  "network_class": "lte-operator-a",
  "transport_class": "tcp-ws",
  "connect_ms": 84,
  "tls_ms": 110,
  "ttfb_ms": 290,
  "bytes": 16384,
  "rst": false,
  "timeout": true,
  "label": "SilentFreeze"
}
```

---

# 60. Controlled lab

Здесь начинается научная часть.

Сделать Linux environment:

```text
client
  ↓
bridge
  ↓
impairment layer
  ↓
server
```

Сценарии:

```text
Normal
Loss
Delay
Jitter
RST
Blackhole
DNS manipulation
Connection cap
Freeze after N bytes
Return path failure
Packet reorder
```

---

# 61. Почему lab нужен именно до ML

В lab label известен.

Например:

```text
Profile = SilentFreeze16K
```

→ мы точно знаем ground truth.

На реальном наблюдении:

```text
Observed = freeze at 15.8KB
```

→ это только evidence.

Поэтому:

```text
lab = ground truth
field = noisy observation
```

---

# 62. Synthetic profiles

Минимум:

```text
LAB-001 normal
LAB-002 tcp-rst
LAB-003 tls-rst
LAB-004 blackhole
LAB-005 freeze-8k
LAB-006 freeze-16k
LAB-007 freeze-25pkt
LAB-008 dns-subversion
LAB-009 return-path-loss
LAB-010 loss+latency
LAB-011 high-load
LAB-012 intermittent
```

---

# 63. Classifier v1

Решающее дерево руками:

```text
SYN?
 ├─ NO → PortBlackholed
 └─ YES

TLS?
 ├─ NO
 │   ├─ RST → ResetInjected
 │   └─ timeout → TLSFailure
 │
 └─ YES

DATA?
 ├─ NO → ApplicationFailure
 └─ YES

RST?
 ├─ YES → ResetInjected
 └─ NO

progress?
 ├─ NO → SilentFreeze
 └─ YES
```

---

# 64. Classifier v2

Когда накопятся данные:

```text
Gradient Boosting
Random Forest
Logistic Regression
```

Но baseline:

```text
Rule classifier
```

обязательно оставить.

Модель сравнивается с ним.

---

# 65. ML target

ML должен предсказывать:

```text
failure class
```

а не:

```text
"какой VPN сейчас использовать?"
```

Это резко упрощает задачу.

---

# 66. Dataset split

Нельзя random split по отдельным probes.

Лучше:

```text
train = earlier epochs
validation = later epochs
holdout = completely later epoch / different network context
```

Потому что иначе соседние измерения одного и того же события окажутся и в train, и в test.

---

# 67. Метрики classifier

Минимум:

```text
accuracy
precision
recall
F1
confusion matrix
calibration
unknown rejection
```

Но важнее:

```text
false switch rate
```

и:

```text
false recovery rate
```

Потому что classifier работает не сам по себе — его ошибка вызывает action.

---

# 68. Controller metrics

Основные:

```text
MTTD
```

Mean Time To Detect.

```text
MTTR
```

Mean Time To Recovery.

```text
switch_success_rate
```

```text
unnecessary_switch_rate
```

```text
blocked_duration
```

```text
channel_recovery_time
```

---

# 69. Главная метрика системы

Не throughput.

И не:

```text
"процент блокировок"
```

А:

> **Time to restore a validated protected path after degradation.**

То есть:

```text
A failed
↓
detect
↓
classify
↓
select
↓
switch
↓
probe
↓
RUNNING
```

Измеряем весь цикл.

---

# 70. Second metric

```text
unnecessary failover rate
```

Например:

```text
100 transient network hiccups
→ 80 unnecessary switches
```

Это плохой controller.

---

# 71. Third metric

```text
correlated failure detection accuracy
```

Если три канала одновременно теряют data path:

система должна понять:

```text
"возможно, это общий failure domain"
```

а не три независимых transport failure.

---

# 72. Canary policy

Активному каналу:

```text
full probe
```

Неактивным:

```text
cheap probe
```

Диагностический full probe:

```text
только при высокой необходимости
```

---

# 73. Exploration / exploitation

Пока без RL.

Но принцип можно реализовать детерминированно.

```text
95%
→ exploit best-known channel

5%
→ explore alternative canary
```

Однако exploration должен быть:

```text
controlled
bounded
background
```

а не live switching ради эксперимента.

---

# 74. Pseudocode Controller

```go
func (c *Controller) Tick(ctx context.Context) {
    active := c.health.Active()

    result := c.probe.Normal(active)

    c.health.Update(active, result)

    if result.Pass {
        c.incident.RecordSuccess(active)
        c.recovery.Update(active)
        return
    }

    state := c.classifier.Classify(result)

    if state.Confidence < c.policy.MinConfidence {
        c.probe.FastReprobe(active)
        return
    }

    if c.incident.IsTransient(state) {
        return
    }

    candidates := c.selector.Candidates(
        active,
        c.health,
        c.failureDomains,
    )

    best := c.selector.Select(candidates, state)

    if best == nil {
        c.enterBlocked()
        return
    }

    c.manager.SwitchChannel(best.ID)
}
```

---

# 75. Но Controller не должен быть таким простым

Перед `SwitchChannel` ещё должны проверяться:

```text
candidate.enabled
candidate.config_valid
candidate.evidence_valid
candidate.not_stale
candidate.not_in_cooldown
candidate.failure_domain_independent
candidate.not_same_incident_bad_actor
```

---

# 76. Candidate rejection reasons

Controller должен уметь объяснить:

```text
B rejected:
    stale evidence

C rejected:
    same failure domain

D rejected:
    not live verified

E rejected:
    previous failure confidence 0.94
```

А не просто:

```text
"No channel available"
```

---

# 77. Channel quarantine

Если канал несколько раз падает одинаковым образом:

```text
A:
SilentFreeze × 8
```

не надо пробовать A каждые 30 секунд.

Перевести:

```text
QUARANTINED
```

например:

```text
5 min
→ 15 min
→ 30 min
→ 1 h
```

---

# 78. Но quarantine тоже context-dependent

Потому что тот же канал может работать через:

```text
Wi-Fi
```

но не работать через:

```text
LTE
```

Поэтому quarantine:

```text
channel + NetworkEpoch
```

а не глобально.

---

# 79. Пример

```text
A bad on LTE
A good on Wi-Fi
```

Нельзя:

```text
A = globally retired
```

Нужно:

```text
A:
 LTE → degraded
 Wi-Fi → healthy
```

---

# 80. Политика retirement

Канал переходит в `retired` только при:

```text
confirmed configuration invalid
```

или:

```text
server intentionally removed
```

или:

```text
metadata revocation
```

Плохая статистика не должна автоматически уничтожать descriptor.

---

# 81. Channel B acceptance criteria

Очень подробно.

### Infrastructure

```text
different VPS
different ASN
different provider class
```

### Protocol

```text
REALITY
TCP
no Vision
```

### Config

```text
strict validation PASS
no insecure
valid Reality key
valid short_id
valid SNI
```

### Runtime

```text
engine starts
TLS handshake
authenticated connection
protected probe
egress verification
```

### Evidence

```text
3 independent successful probes
at different times
```

### Failure

```text
server-origin failure ≠ client failure
```

### Status

```text
live-verified
validated_at
evidence tuple
```

---

# 82. B1 — важный architectural checkpoint

После появления B обязательно проверить:

```text
A expected egress = VPS-A
B expected egress = VPS-B
```

Это уже нужно, потому что текущий план специально исправлял потенциальную ошибку, когда Reload использовал старый expected egress после переключения на другой VPS. 

Это хороший пример того, почему новый descriptor/evidence architecture действительно нужен.

---

# 83. Channel C

HY2 оставляем как:

```text
UDP/QUIC failure domain
```

Существующий port hopping:

```text
443
8444
40000–41000
```

можно оставить per-channel. Текущая теория правильно предупреждает, что нестандартный port **не является гарантированным обходом**: наблюдался случай, когда не-443 диапазон целиком blackholed. 

Поэтому:

```text
port hopping
```

это **одна из вариаций одного transport family**, а не настоящий новый failure domain.

---

# 84. Это важная поправка

Нельзя считать:

```text
HY2:8444
HY2:40000
HY2:45000
```

тремя каналами.

Это:

```text
один QUIC/HY2 failure domain
```

с различными endpoint policies.

---

# 85. Аналогично с VLESS-профилями

Нельзя считать:

```text
VLESS+WS
VLESS+WS другой порт
VLESS+WS другой SNI
```

тремя полностью независимыми каналами.

Это:

```text
один protocol family
```

с различными parameters.

---

# 86. Channel family vs channel instance

Ввести:

```text
ChannelFamily
```

и:

```text
ChannelInstance
```

Например:

```text
Family:
    TCP/VLESS/WS

Instances:
    A1 CF 443
    A2 CF 8443
    A3 CDN-B
```

Controller должен сначала выбирать family diversity, потом конкретный instance.

---

# 87. Transport diversity matrix

| Family | TCP | UDP | CDN | Direct | REALITY |     WS |
| ------ | --: | --: | --: | -----: | ------: | -----: |
| A      |   ✓ |     |   ✓ |        |         |      ✓ |
| B      |   ✓ |     |     |      ✓ |       ✓ |        |
| C      |     |   ✓ |     |      ✓ |         |        |
| D      |   ✓ |     |   ✓ |        |  future | future |

Так будет видно, где действительно есть разнообразие.

---

# 88. Transport research backlog

## P0

```text
VLESS+WS+TLS
VLESS+REALITY
HY2/QUIC
```

## P1

```text
additional origin
additional CDN path
additional port profile
splithttp/XHTTP when supported by chosen engine
```

## P2

```text
WebTunnel
Snowflake
meek-like transports
traffic morphing
steganographic research
```

Почему не делать всё сразу:

Xue показывает, что просто добавление encapsulation/padding/multiplexing не гарантирует исчезновения classification signal. ([USENIX][1])

---

# 89. XHTTP decision

Твой текущий выбор:

```text
XHTTP not in pinned sing-box
→ do not change core
```

я бы сохранил.

Но сделать отдельный research record:

```text
XHTTP:
status = research
engine support = unavailable
replacement = splithttp candidate
decision = deferred
```

Не смешивать с MVP.

---

# 90. REALITY

REALITY должен быть описан так:

```text
anti-probing advantage
+
TLS mimicry
+
different failure domain
```

но не:

```text
undetectable
```

Потому что твоя теория сама фиксирует, что uTLS/REALITY не дают гарантированной невидимости, а Xue показывает более общий класс traffic fingerprinting.  ([USENIX][1])

---

# 91. uTLS

uTLS остаётся:

```text
fingerprint mitigation
```

а не:

```text
anti-detection guarantee
```

Это соответствует и твоей теории, где upstream limitation уже зафиксирован. 

---

# 92. Multiplex

Сделать policy matrix:

| Failure hypothesis             | Mux            |
| ------------------------------ | -------------- |
| SilentFreeze                   | prefer off     |
| TLS-policing / many handshakes | may help       |
| Unknown                        | conservative   |
| low-latency stable path        | optional       |
| REALITY behavioral-sensitive   | off by default |

Теория как раз правильно фиксирует конфликт: mux может помогать одному классу проблем и одновременно усиливать другой. 

Поэтому:

```text
mux = channel policy
```

а не global setting.

---

# 93. Padding

То же:

```text
padding
```

не является самостоятельным “stealth mode”.

В descriptor:

```text
padding_mode:
    off
    conservative
    aggressive
```

Но default:

```text
conservative / off
```

в зависимости от transport family.

---

# 94. Short-lived connections

Это уже хорошо обоснованная инженерная линия в твоей теории.

Но делать её не просто глобальным правилом:

```text
short connections = always good
```

а:

```text
behavior profile
```

Например:

```yaml
connection_policy:
  max_lifetime: ...
  idle_timeout: ...
  max_bytes_per_connection: ...
```

Controller может выбирать profile.

---

# 95. Evidence-based adaptation

Например:

```text
SilentFreeze confidence 0.92
```

→

```text
connection_profile = short
mux = off
candidate = different family
```

А:

```text
TlsPolicing confidence 0.89
```

→

```text
mux = candidate
or alternate port
```

Но только если конкретный channel descriptor разрешает такую policy.

---

# 96. Почему adaptation должен изменять профиль, а не только канал

Это следующий уровень после обычного failover.

Не:

```text
A → B
```

а:

```text
A/profile-1
       ↓
A/profile-2
       ↓
B/profile-1
       ↓
C/profile-1
```

Но profile должен иметь собственный ID:

```text
channel-a/profile-short
channel-a/profile-mux
channel-b/profile-default
```

---

# 97. Profile = immutable configuration bundle

Нельзя динамически менять 20 параметров независимо.

Сделать:

```text
TransportProfile
```

Например:

```yaml
id: vless-ws-short

multiplex: false
keepalive: 60
padding: false
connection_lifetime: 30s
```

Controller выбирает profile, а не отдельные knobs.

Это сильно снижает сложность.

---

# 98. Profile compatibility matrix

Каждый profile:

```text
client capability
server capability
transport family
expected behavior
```

И перед switch:

```text
profile compatible?
```

Твой текущий live-инцидент с mux показал, насколько опасен mismatch client/server transport features. 

---

# 99. Contract test

Для каждого transport profile:

```text
render
→ validate
→ server capability
→ engine start
→ probe
```

Нельзя объявлять profile supported только потому, что `sing-box check` его принял.

Теория уже содержит реальный пример, когда синтактически валидный REALITY config был семантически мёртв из-за неправильного server name. 

---

# 100. Semantic validation

Вводим:

```text
SyntaxValid
SemanticValid
LiveValidated
```

### SyntaxValid

JSON корректен.

### SemanticValid

конфигурация логически непротиворечива.

### LiveValidated

реальный path прошёл evidence gate.

---

# 101. Новый lifecycle статусов

```text
UNKNOWN
 ↓
CONFIGURED
 ↓
SEMANTICALLY_VALID
 ↓
VALIDATED
 ↓
HEALTHY
 ↓
DEGRADED
 ↓
QUARANTINED
 ↓
RECOVERING
 ↓
HEALTHY
```

Отдельно:

```text
REVOKED
RETIRED
```

---

# 102. Почему это лучше текущего `live-verified`

Потому что теперь:

```text
validated
```

и:

```text
healthy
```

— разные понятия.

Это решает один из главных концептуальных недостатков текущего плана.

---

# 103. Acceptance gates

Каждый этап должен иметь:

```text
Code
Tests
Evidence
Failure conditions
Rollback
```

Например:

## Channel B

```text
code ✓
unit tests ✓
semantic tests ✓
live probe ✓
3 repeated probes ✓
cross-network evidence ?
```

Статус:

```text
VALIDATED
```

а не:

```text
HEALTHY EVERYWHERE
```

---

# 104. Research evidence levels

Твоё правило из THEORY надо сохранить:

```text
[доказано нами]
[документировано]
[исследование]
[полевое наблюдение]
[гипотеза]
```



Но в PLAN я бы добавил:

```text
[engineering assumption]
```

для внутренних policy decisions.

---

# 105. Source hierarchy

Для архитектурных решений:

### Level A

```text
RFC
official protocol documentation
official upstream code
```

### Level B

```text
peer-reviewed / USENIX / IMC papers
```

### Level C

```text
OONI / Cloudflare / measurement reports
```

### Level D

```text
community measurements
net4people
```

### Level E

```text
single user observation
```

### Level F

```text
LLM / forum / unverified blog
```

И нельзя повышать уровень достоверности автоматически.

---

# 106. OONI как методологический источник

OONI — полезен здесь не потому, что он доказывает механизм конкретного VPN, а потому что предоставляет модель массовых измерений censorship/performance и большой исторический dataset. OONI прямо описывает свою систему как измерение censorship и network performance, с миллионами измерений из множества стран. ([OONI][2])

В плане это означает:

```text
community observation
≠
ground truth
```

но:

```text
repeated measurements
+
consistent methodology
```

намного сильнее единичного наблюдения.

---

# 107. Test methodology для проекта

Не отдельный “полевой testing section”, а встроенный **Evidence Pipeline**.

```text
Hypothesis
 ↓
Controlled test
 ↓
Observation
 ↓
Repeated runs
 ↓
Classification
 ↓
Evidence record
 ↓
Theory update
 ↓
Architecture decision
```

---

# 108. Example

Гипотеза:

```text
REALITY channel fails specifically because of port 8443
```

Не делать сразу:

```text
switch to 443
```

Сначала:

```text
same protocol
same server
same client
different port
```

Если:

```text
8443 fail
443 pass
```

→ evidence for port-specific failure.

Если:

```text
both fail
```

→ port hypothesis weakened.

---

# 109. A/B testing in controlled environment

Для каждого transport change:

```text
baseline
vs
modified
```

Меняем:

```text
one variable at a time
```

Иначе невозможно понять причину результата.

---

# 110. Experiment record

```yaml
experiment_id: EXP-042

hypothesis:
  "mux reduces failure under TLS connection policing"

baseline:
  mux=false

variant:
  mux=true

variables:
  same_server: true
  same_network: true
  same_probe: true

results:
  baseline: ...
  variant: ...

conclusion:
  supported / rejected / inconclusive
```

---

# 111. Почему это особенно важно для твоего проекта

В журнале уже были ситуации:

```text
симптом → неверная гипотеза → live-проверка → реальная причина
```

Например:

* считали UDP недоступным;
* считали проблему nginx;
* подозревали mux;
* выясняли return-path;
* находили server-side config mismatch.

Это уже показывает, что **diagnostic methodology — самостоятельная часть продукта**, а не просто отладка. 

---

# 112. Теперь дорожная карта

Ниже уже не просто фазы, а порядок реализации.

---

# PHASE A — Evidence Core

## A0 — Health model

### P0

Implement:

```text
ChannelHealth
NetworkContext
EvidenceAge
HealthEvidence
```

Acceptance:

```text
unit tests
serialization tests
state transition tests
```

---

## A1 — Failure taxonomy

### P0

Implement:

```text
FailureClass
ObservedSignature
Confidence
```

Tests:

```text
RST
freeze
blackhole
DNS
return path
unknown
```

---

## A2 — Diagnostic Probe

### P0

Добавить:

```text
bytes before failure
packets before failure
RST/FIN
TTFB
completion
freeze duration
```

Acceptance:

```text
normal connection
controlled reset
controlled timeout
controlled freeze
```

---

# PHASE B — Independent Infrastructure

## B1 — Channel B

### P0

```text
VPS-B
ASN-B
REALITY
independent origin
```

---

## B2 — Failure Domain Model

### P0

Добавить:

```text
transport family
provider
ASN
origin
carrier
network layer
port family
```

---

## B3 — Channel Family

### P0/P1

Разделить:

```text
Family
Instance
Profile
```

Например:

```text
TCP/VLESS/WS
  ├── A1
  ├── A2
```

---

# PHASE C — AdaptiveController

## C1 — State machine

### P0

```text
HEALTHY
SUSPECTED
CLASSIFYING
DEGRADED
SWITCHING
BLOCKED
RECOVERING
```

---

## C2 — Candidate selection

### P0

Учитывать:

```text
health
confidence
independence
freshness
failure history
```

---

## C3 — Rollback

### P0

```text
new channel fail
→ old channel probe
→ restore if healthy
```

---

## C4 — Quarantine

### P1

```text
repeated class-specific failures
→ quarantine
→ exponential recovery
```

---

# PHASE D — Recovery System

## D1 — Background probes

### P1

Все неактивные validated channels:

```text
cheap probe
```

---

## D2 — Recovery state

```text
QUARANTINED
→ RECOVERING
→ PROBATION
→ HEALTHY
```

---

## D3 — Incident clustering

### P1

```text
A + B + C failure
→ one incident
```

---

# PHASE E — Controlled Lab

## E1 — Network emulator

### P1

Сценарии:

```text
RST
timeout
blackhole
freeze
latency
loss
DNS
return path
```

---

## E2 — Ground truth dataset

### P1

```text
scenario
→ transport
→ features
→ known label
```

---

## E3 — Rule classifier benchmark

### P1

До ML.

---

# PHASE F — Statistical Classifier

## F1

Feature extraction:

```text
connect_ms
tls_ms
ttfb
bytes
packets
rst
timeout
rtt
loss
transport
network
```

---

## F2

Models:

```text
baseline rules
logistic regression
random forest
gradient boosting
```

---

## F3

Validation:

```text
temporal holdout
network holdout
transport holdout
```

---

# PHASE G — Policy Optimization

## G1

Сравнивать:

```text
static priority
vs
health-based
vs
health+independence
```

---

## G2

Metric:

```text
time-to-recover
unnecessary switches
blocked duration
```

---

## G3

Только потом:

```text
contextual ranking
```

---

# PHASE H — Transport Expansion

### P2

После C–G:

```text
additional CDN
additional origin
XHTTP/splithttp
additional QUIC profile
WebTunnel
Snowflake
other research transports
```

---

# PHASE I — Android

### P2

Только после стабилизации controller model.

Архитектура:

```text
core/adaptive
core/probe
core/classifier
core/model
```

shared.

Windows:

```text
windows adapter
```

Android:

```text
android adapter
```

---

# PHASE J — ML/RL research

### P2/P3

ML:

```text
classification
ranking
```

RL:

```text
offline simulation only
```

На реальном клиенте initially:

```text
NO autonomous RL
```

---

# 113. Что должно попасть в новый PLAN буквально

Я бы сделал в нём следующие разделы:

```text
0. Mission
1. Non-negotiable invariants
2. Current verified state
3. Evidence model
4. Channel model
5. Failure-domain model
6. Network context
7. Protected Probe
8. Diagnostic Probe
9. Failure taxonomy
10. Failure classifier
11. Channel Health
12. Incident model
13. AdaptiveController
14. Candidate Selection
15. Recovery
16. Egress adaptation
17. Transport Profiles
18. Metadata / Control Plane
19. Telemetry
20. Controlled Lab
21. Dataset
22. ML
23. Acceptance Gates
24. Windows roadmap
25. Android roadmap
26. Research backlog
27. Evidence journal
28. Source map
```

То есть план будет не 300–400 строк, а, скорее всего, **800–1200 строк**, но уже без бесконечного журнала в теле основных требований.

---

# 114. Очень важное разделение документов

Сейчас `PLAN.md` перегружен одновременно:

```text
architecture
requirements
implementation status
journal
research
```

Я бы разделил:

```text
docs/
├── PLAN.md
├── ARCHITECTURE.md
├── FAILURE-MODEL.md
├── ADAPTIVE-CONTROLLER.md
├── PROBE-SPEC.md
├── TRANSPORTS.md
├── EVIDENCE-SCHEMA.md
├── LAB-MODELS.md
├── ML-DESIGN.md
├── SOURCE-MAP.md
└── JOURNAL.md
```

А `PLAN.md` оставить как **master roadmap + ссылки**.

Это особенно полезно, потому что сейчас журнал уже стал огромным — и доказательная история смешивается с актуальным design contract. В самом плане видно, что журнал V2 уже содержит десятки подробных live-изменений. 

---

# 115. `PLAN.md` тогда должен выглядеть так

```text
# snowden.system

## 0. Mission

## 1. Invariants

## 2. Current state

## 3. Architecture

## 4. Evidence model

## 5. AdaptiveController

## 6. Transport roadmap

## 7. Windows

## 8. Android

## 9. Controlled laboratory

## 10. ML

## 11. Acceptance gates

## 12. Roadmap

## 13. Open decisions

## 14. Links
```

А:

```text
JOURNAL.md
```

будет содержать весь V2-XXX.

---

# 116. Что сделать первым — прямо сейчас

Не начинать Android.

Не начинать Snowflake.

Не начинать steganography.

Не писать RL.

Не менять sing-box.

Не делать ещё десять транспортов.

Сделать:

### Шаг 1

```text
ChannelHealth
```

### Шаг 2

```text
FailureClass
```

### Шаг 3

```text
DiagnosticProbe
```

### Шаг 4

```text
FailureDomain
```

### Шаг 5

```text
NetworkContext
```

### Шаг 6

```text
Channel B / real independent infrastructure
```

### Шаг 7

```text
AdaptiveController
```

### Шаг 8

```text
Recovery + quarantine
```

### Шаг 9

```text
Controlled lab
```

### Шаг 10

```text
classifier
```

---

# 117. Первый milestone новой системы

Он должен выглядеть так:

```text
A = active

B = validated
C = validated

A fails
 ↓
diagnostic probe
 ↓
SilentFreeze confidence=0.91
 ↓
failure domain:
    TCP/CDN/ASN-A
 ↓
B:
    TCP/REALITY/ASN-B
 ↓
independence = high
 ↓
switch B
 ↓
probe B PASS
 ↓
A → QUARANTINED
 ↓
incident RESOLVED
```

Это уже **Adaptive VPN**.

Не просто VPN с двумя кнопками.

---

# 118. Второй milestone

```text
B active

A:
cheap recovery probe → fail

C:
cheap probe → pass

B:
degrades

Controller:
   C selected

B:
QUARANTINED

after 15m:
B recovery probe pass

after 3 pass:
B PROBATION

after additional pass:
B HEALTHY
```

Это уже система, умеющая жить в изменяющейся среде.

---

# 119. Третий milestone

Controlled lab:

```text
scenario:
16KB freeze

A:
fails

classifier:
SilentFreeze 0.96

controller:
chooses C

C:
passes

incident:
resolved

classifier:
prediction compared with ground truth
```

После сотен таких controlled cases можно говорить о статистическом классификаторе.

---

# 120. Четвёртый milestone

На исторических данных:

```text
network context
+
transport family
+
failure signatures
+
health history
```

получаем:

```text
P(success | channel, context)
```

Но не просто:

```text
A = 78%
```

а:

```text
A | network_epoch=E42 | LTE | ASN=X | current conditions
```

---

# 121. И только после этого — действительно “умный” Controller

Финальная модель:

```text
                 OBSERVATIONS
                       │
                       ▼
               FEATURE EXTRACTION
                       │
                       ▼
            ┌────────────────────┐
            │  FAILURE CLASSIFIER│
            └─────────┬──────────┘
                      │
                      ▼
               CHANNEL HEALTH
                      │
                      ▼
            FAILURE-DOMAIN GRAPH
                      │
                      ▼
             CANDIDATE SCORING
                      │
                      ▼
             ADAPTIVE CONTROLLER
                      │
          ┌───────────┼────────────┐
          ▼           ▼            ▼
        STAY       SWITCH       RECOVER
          │           │            │
          └───────────┴────────────┘
                      │
                      ▼
                   PROBE
                      │
                      ▼
                  EVIDENCE
                      │
                      └──────────► HISTORY
```

Вот это уже архитектура, которую я бы считал **главной целью проекта**.

---

# 122. Главное изменение относительно текущего PLAN

Текущий план уже очень хорошо решает:

> **Как безопасно запустить конкретный transport и доказать, что он работает?**

Следующая версия должна решать:

> **Как системе понять, почему transport перестал работать, определить независимый путь, выбрать его, проверить результат, дождаться восстановления старого пути и накопить evidence для будущих решений?**

И это логично вытекает именно из твоей теории: многослойность L1–L5, stateful/in-path модель TSPU, разные failure signatures, различия ISP/network context, 16-КБ freeze, несовершенство port switching, независимость TCP/UDP и ограниченность uTLS/ECH/padding как универсальных решений.  

А исследования Xue/Wu дают научное основание для более осторожной модели: **не считать протокол “невидимым” только потому, что он зашифрован или похож на HTTPS; наблюдаемые статистические/структурные признаки могут оставаться классифицируемыми.** При этом перенос результатов китайского GFW или лабораторного ISP из Xue непосредственно на TSPU нельзя делать без отдельного evidence. ([USENIX][1])

**Поэтому я бы зафиксировал главный принцип v3 одной строкой:**

```text
Do not build a larger list of transports.
Build a better system for choosing among imperfect transports.
```

Именно после этого уже имеет смысл добавлять новые transports: они становятся **плагинами для AdaptiveController**, а не очередным набором ручных костылей.

[1]: https://www.usenix.org/conference/usenixsecurity24/presentation/xue-fingerprinting?utm_source=chatgpt.com "Fingerprinting Obfuscated Proxy Traffic with Encapsulated TLS Handshakes | USENIX"
[2]: https://ooni.org/?utm_source=chatgpt.com "OONI: Open Observatory of Network Interference | OONI"
