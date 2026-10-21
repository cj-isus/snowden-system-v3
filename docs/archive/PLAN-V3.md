# [АРХИВ 2026-09-17, V2-066] — НЕ источник правды

> Перенесён из docs/PLAN-V3.md: консолидация планов в один (см. PLAN.md
> §1 «Один источник правды» и §5 «Phase D — Adaptive Core»). Статусы фаз
> теперь живут в PLAN.md §5 (Phase D), дизайн-детали и обоснования — здесь,
> исторически. Живые ссылки на «PLAN-V3» в других файлах означают этот файл.

# snowden.system — Adaptive Master Plan v3 (дистиллят docs/plan_gpt.md)

> Версия: 2026-09-16 (v1 дистилляции). Источник: docs/plan_gpt.md (4256 строк,
> консультация GPT, получена владельцем 2026-09-16) + обновление
> docs/CENSORSHIP-THEORY.md от 2026-09-16 (§2.8–2.11).
>
> Статус документа: **проектный (design), не контракт**. Этот файл НЕ заменяет
> docs/PLAN.md (один источник правды — PLAN.md, §1; решение владельца V2-035 —
> поверх действующего плана «план v2 поверх него НЕ пишется»). Здесь —
> систематизация предложения по AdaptiveController-слою, сверенная с уже
> реализованным (V2-027/037/040/053/054), с честным маппингом «что есть / чего
> нет». Реализация каждого пункта входит в PLAN.md только после явного решения
> владельца и записывается журналом V2-XXX в PLAN.md.

---

## 0. Mission (одна строка)

```text
Do not build a larger list of transports.
Build a better system for choosing among imperfect transports.   (plan_gpt §122)
```

Транспорт — уже не решение, а инструмент в руках контроллера: наблюдение сети →
классификация отказа → здоровье каналов → выбор независимого кандидата →
probe-гейт → evidence. Fail-closed (FR-001) не ослабляется ни на одном уровне.

## 1. Invariants (необъемлемо, наследуется из PLAN §1 и AGENTS §3.3)

1. **Fail-closed**: защищённый трафик только через validated каналы; `direct`
   запрещён в protected selector/fallback/recovery. Нет каналов → BLOCKED.
2. **Controller → только через Manager**: контроллер не управляет Engine
   напрямую; любое переключение = switchChannelCore (= путь SelectChannel:
   render → двойная валидация → Manager.Reload/SwitchChannel → обязательный
   probe). Уже реализовано в V2-027/037 — наследуется.
3. **Категория ошибки = наблюдаемый факт**, не гипотеза «это DPI» (AGENTS §3.3).
   Классификатор выдаёт код + confidence; код ≠ диагноз без evidence.
4. **NetworkFacts — обязателен** для любой интерпретации отказов: без
   оператора/типа сети любая классификация причин — шум (THEORY §2.10 п.16,
   доказано V2-054).
5. **Секреты по ссылке**, значение не попадает ни в классификатор, ни в
   telemetry, ни в журнал (PLAN §1.4).
6. **Честные статусы** (evidence over claims, PLAN §1.2): `implemented`,
   `locally-tested`, `live-verified` — не путать.

## 2. Модель (диаграмма из plan_gpt §0)

```text
NetworkFacts (V2-054 netwatch: класс сети, fingerprint, переходы)
        ↓
Observation (protected probe 6/6 — L1; diagnostic probe — TODO Phase A2)
        ↓
Failure Classifier (детерминированный v1: failclass.go; ML — Phase F, не раньше Controlled Lab)
        ↓
Channel Health / Failure Domain (дескриптор: transport family, origin, ASN, порт)
        ↓
AdaptiveController (сейчас: failoverPolicy V2-027 + авто-ретрай V2-053 + netwatch V2-054)
        ↓
Manager (switchChannelCore / startCore — единственная точка применения решений)
```

## 3. Что уже реализовано (маппинг plan v3 → V2-XXX)

| Компонент plan v3 | Статус | Чем закрыто |
|---|---|---|
| Protected Probe v2, уровень L1 (DNS + 2×HTTPS + egress + leak) | live-verified | PLAN §3 FR-003; DNS-шаг сделан независимым (V2-037) |
| NetworkContext / NetworkFacts | live-verified (частично: класс сети + переходы) | V2-054 netwatch (fingerprint без PowerShell, GetIfEntry2Ex, дебаунс); ASN/IPv6/DNS-режим — нет |
| NetworkEpoch | implemented + locally-tested (V2-057) | netepoch.go: up/change бампят, down нет; здоровье каналов скоупится эпохой (fail-closed к no_data при смене) |
| Авто-выбор транспорта по классу сети | implemented + locally-tested | V2-054 transportpref (mobile→HY2, wifi→vless+ws) |
| Авто-повтор после fail-closed (Recovery, начало) | live-verified | V2-053 autoretry (backoff 30с→10м, wake по смене сети) |
| Failover-перебор validated каналов по probe | implemented (live-verify — после B1) | V2-027 failoverPolicy + V2-040 failover-on-start; бесшовное переключение V2-037 (Manager.SwitchChannel, rollback-семантика) |
| Failure Classifier v1 (детерминированный) | implemented + locally-tested (V2-056) | windows/failclass.go — коды THEORY §2.10 п.16 поверх probe-отчёта; интегрирован в журнал сторожа и машину состояний C1 (V2-060) |
| AdaptiveController state machine | **v1 (V2-060)** | controller.go: HEALTHY→SUSPECTED→CLASSIFYING→DEGRADED→SWITCHING→BLOCKED/RECOVERING поверх failoverPolicy (поверх, не вместо); fail-closed не изменён |
| Candidate scoring (C2) | **v1 (V2-061)** | scoring.go: 0.30·H + 0.35·I + 0.20·F + 0.15·E (§28 независимость — главный вес); tryFailover пересортировывает кандидатов; калибровка весов — E2 |
| Egress как отдельное измерение | live-verified | V2-041 (WARP endpoint + адаптивный детектор challenge), THEORY §5 |
| Diagnostic Probe (freeze-детекция контролируемыми объёмами; TTFB/RST-фазы — нет) | **частично (A2: V2-057)** | diagprobe.go: ступени 8–64КБ туннель+direct, freeze/timeout/reset классификация; freeze-check на «Проверках» (нужен серверный /range — за владельцем); L0-тайминги — дальше |
| ChannelHealth (history/score/quarantine) | **частично (A0: V2-057)** | chanhealth.go: OK/FAIL/класс отказа по каналам внутри сетевой эпохи + EvidenceAge (свежесть 2м); score/quarantine — Phase C4 |
| Channel B / независимая инфраструктура | blocked (owner) | B1-VPS-1 (PLAN §2.4); клиентская сторона готова (V2-032/044: schema 2, tcp-reality) |
| Failure Domain Model (family/instance, транспортная диверсификация) | **v1 (V2-059)** | fdmodel.go: fd-поля дескриптора + деривация + Independence/порог 0.35 (plan_gpt §5/§6); weights-калибровка — E2, family-vs-instance — B3 |
| Incident model / clustering | **нет** | plan_gpt §31–35 |
| Controlled Lab / Dataset / ML | **нет, сознательно отложено** | plan_gpt §60–71, Phase E–J: только после лабораторных ground-truth сценариев |
| Metadata / подписанные профили | implemented (live-ввод — после B1) | V2-033/034, FR-008 |

## 4. Дорожная карта (Phase A–J из plan_gpt §112, перенумеровано под наш статус)

### Phase A — Evidence Core (текущая)

- A0 Health model — **ядро сделано (V2-057)**: NetworkEpoch + ChannelHealth
  (OK/FAIL/класс, EvidenceAge 2м, verdict no_data/healthy/failing) в памяти
  процесса; персистентность и score — после B2.
- A1 Failure taxonomy — **classifier v1 сделан** (V2-056, failclass.go); v2
  (features: RST-фазы, freeze-длительность) — после A2-live.
- A2 Diagnostic Probe — **freeze-детекция сделана (V2-057, diagprobe.go)**:
  ступени 8–64КБ, туннель+direct, честный skip без diagnostic_url; осталось:
  серверный /range-эндпоинт (владелец), live-прогон, L0-тайминги.
- A3 ChannelHealth UI — **карточка «Здоровье каналов» сделана (V2-058,
  AutoProtectView, вкладка «Состояние»)**: эпоха + netClass, verdict-пилюли
  (здоров/сбоит/нет данных), ok/fail-счётчики, человекочитаемый класс
  последнего отказа, свежесть факта; contract.ts → backend.ts → wailsjs →
  UI по правилу PLAN §4.1.5; honest-семантика бэкенда не тронута (включая
  транспорт из дескриптора FR-002). Осталось: live-проверка на реальной волне
  деградации (совместно с A0/A2).

### Phase B — Independent Infrastructure

- B1 канал B (VPS-B, другой ASN/страна, REALITY) — **blocker B1-VPS-1, owner**.
  Приёмка — docs/B1-REALITY-RESEARCH.md §7.
- B2 Failure Domain Model v1 — **закрыто раньше B1 (V2-059)**: plan_gpt §116
  ставит модель доменов (шаг 4) ДО канала B (шаг 6), иначе канал B рождается
  без семантики независимости. fdmodel.go: FailureDomain в дескрипторе
  (carrier/asn/origin/port_class/tls_donor/layer, optional) + деривация из уже
  валидированных полей + Independence(a,b) = 1−Σштрафов с порогом 0.35
  (план: калибровка весов на ground truth — Phase E2); биндинг
  GetChannelIndependence готов к C2-сортировке кандидатов.
- B3 Channel Family vs Instance (один сервер, два профиля = одна семья) — P1.

### Phase C — AdaptiveController (эволюция существующего)

- C1 State machine (HEALTHY/SUSPECTED/CLASSIFYING/DEGRADED/SWITCHING/BLOCKED/
  RECOVERING) — **закрыто (V2-060)**: controller.go формализует существующую
  механику (fast-репробы V2-037 = SUSPECTED, серия = CLASSIFYING, авто-ретрай
  V2-053 = RECOVERING) поверх failoverPolicy; fail-closed семантика не изменена
  (failover_test.go зелёные без правок — критерий «поверх, не вместо»);
  GetFailoverStatus несёт smState/smSince/smClass; UI-карточка «Адаптивный
  цикл» на вкладке «Состояние» — закрыто (V2-062) по правилу §4.1.5.
- C2 Candidate scoring — **закрыто (V2-061)**: scoring.go S(c) = 0.30·health +
  0.35·independence + 0.20·freshness + 0.15·evidence (§28: независимость —
  наибольший вес, «минимально коррелированный survivor»); tryFailover
  пересортировывает только достижимых кандидатов (инвариант V2-037 цел), при
  равном score — прежний порядок V2-027; веса до калибровки — Phase E2.
- C3 Rollback — **уже реализован** (V2-037 Manager.SwitchChannel: новый канал
  не прошёл probe → прежний жив; оба мертвы → fail-closed teardown).
- C4 Quarantine/recovery-состояния каналов — P1.

### Phase D — Recovery System

- D1 Фоновые дешёвые probe неактивных validated каналов — **реализовано**
  (V2-063: bgprobe.go, бюджет 10м/канал, TCP-хендшейк вне туннеля, только
  подтверждённые отказы пишут FAIL src=background; решение по-прежнему только
  probe на переключении).
- D2 QUARANTINED→RECOVERING→PROBATION→HEALTHY — **реализовано** (V2-064:
  quarantine.go — состояние выводится из фактов (probe = сильные, фон D1 =
  слабые), не хранится и не решает; влияние — тай-брейк равных score в C2
  и карточка здоровья; новая эпоха хоронит карантин).
- D3 Incident clustering — **реализовано** (V2-065: incidents.go — чистая
  кластеризация подтверждённых отказов эпохи по двойной шкале времени
  90с/60с; волна в журнал при неудавшемся переключении; вкладка «Инциденты»
  показывает волны + сырой журнал).

### Phase E — Controlled Lab

- E1 Эмулятор сценариев (RST/timeout/blackhole/freeze/loss/DNS/return-path) —
  P1; без него классификатор нечем валидировать (§60 plan_gpt).
- E2 Ground-truth датасет (сценарий → транспорт → фичи → метка) — P1.
- E3 Benchmark rule-classifier до всякого ML — P1.

### Phase F–J (не начинаем)

- F Statistical Classifier, G Policy Optimization, H Transport Expansion
  (XHTTP/splithttp, WebTunnel, Snowflake…), I Android, J ML/RL — только после
  E. Порядок и запреты — plan_gpt §116 (не начинать Android, не менять ядро,
  не делать «ещё десять транспортов»).

## 5. Failure Classifier v1 (реализовано в этой записи — V2-056)

Коды — словарь THEORY §2.10 п.16; вход — структурный ProbeReport (секретов в
нём нет), выход — {Code, Confidence, Hint}:

| Код | Наблюдаемое основание (v1, детерминированное) | Реакция (hint) |
|---|---|---|
| `reset_injected` | все HTTPS-цели через туннель: connection reset; DNS жив | другой SNI/транспорт/канал |
| `port_blackholed` | все цели: dial timeout на origin-порту ≠ 443; direct жив | 443-вариант канала |
| `silent_freeze` | разрыв середины потока (headers получены, body оборван) | короткоживущий транспорт/UDP-канал; нужен diagnostic probe |
| `tls_policing_443` | TLS-хендшейк рвётся на всех целях через 443; direct жив | mux-профиль/порт/другой канал |
| `whitelist_mode` | TLS к origin-порту рвётся при живом TCP (не 443) | честный BLOCKED (L6), программного обхода нет |
| `protected_channel_unavailable` | DNS + все цели падают транспортом одновременно | перебор каналов (существующий failover) |
| `dns_path_failure` | только DNS-шаг падает при живых HTTPS | bootstrap-стратегии (V2-037 уже делает) |
| `egress_divergence` | egress ≠ expected (цели отвечают чужим IP) | отдельное измерение от транспорта (V2-041); канал не «падал» — путь подменён |
| `degraded` | часть целей жива, часть мертва | серия репроб (существующий fastReprobe) |
| `unknown` | нет согласованного паттерна (в т.ч. timeout на 443 без других улик) | честное «причина неизвестна» |

Честные границы v1 (зафиксированы в коде):
- малый probe (256 байт) НЕ видит 16КБ-заморозку и volume-policing — это
  Phase A2 (diagnostic probe с controlled payload);
- confidence — экспертные константы, не статистика; станет статистикой после
  Phase E2 (ground truth);
- v1 не меняет поведение: диагноз попадает только в журнал сторожа; решения
  принимают существующие failoverPolicy/авто-ретрай (C1-C2 позже, через
  Manager-only правило).

## 6. NetworkEpoch (принято для A0/D3)

Эпоха = период между сменами NetworkFacts (класс сети, интерфейс, DNS-среда).
Любое health/history-суждение имеет смысл только внутри эпохи: «канал A жив в
LTE-эпохе E42» ≠ «канал A жив». Реализация — поверх netwatch-переходов V2-054
(счётчик событий + временная метка), без новых системных вызовов.

## 7. Milestones (plan_gpt §117–121, переформулированы под наш контракт)

1. **Adaptive VPN v0**: активный канал падает → classifier v1 в журнале →
   failover перебирает validated → rollback/блок честные. *Достижим после
   live-verify V2-056 (нужна реальная волна деградации или Controlled Lab).*
2. Health/history per channel + quarantine + фоновые probe (Phase A0/C4/D).
3. Classifier с features из Diagnostic Probe и валидацией на Lab-сценариях
   (Phase A2/E) — «prediction compared with ground truth».
4. P(success | channel, network_epoch) — статистика поверх накопленного
   evidence; только потом «умный» контроллер.

## 8. Что сознательно НЕ делаем (план §116 + анти-знание THEORY §2.10)

Android до стабилизации контроллера; Snowflake/steganography; RL на живом
клиенте; смена pinned-ядра sing-box; TLS-фрагментация против ТСПУ (анти-знание);
«uTLS = невидимость» (upstream Not Recommended); ECH через CF (сигнатурно
заблокирован, THEORY §2.9); Brutal как дефолтный конгест-контроль.

## 9. Связи

- PLAN.md — единственный источник правды и журнал V2-XXX; этот файл — проектная
  надстройка и ссылается на него по статусам.
- THEORY §2.10 п.16–17 — словарь классов и методика Cheburcheck-тестов.
- B1-REALITY-RESEARCH §5/§7 — канал B (Phase B1) и приёмка.
- COMPETITOR-FEATURES §5 — дельта рынка (F10/F12/F13 закрыты в V2-048).
- plan_gpt.md — первоисточник 4256 строк; разделы §0–§122; в этот файл взяты
  §0–§9 (модель), §31–35 (инциденты), §40–42 (probe-уровни), §60–61 (lab до ML),
  §112–122 (фазы, milestones, mission).
