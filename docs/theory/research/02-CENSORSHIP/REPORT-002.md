# R02-002 — совместный наблюдатель и границы behavioral defence

**2026-10-25 · READY_FOR_REVIEW · Теория 2.**
Основание: [DEPTH-CYCLE-002](../LIMITATIONS.md), индивидуальное задание R02.
Предыдущий [R02-001](handoffs/R02-001.md) сохранён и ещё не ACCEPTED.
[Источники](SOURCES-002.md) · [новый handoff](handoffs/R02-002.md).

## 1. Точный дополнительный пробел

В R02-001 разведены наблюдатели, knowledge classes и отдельные стадии.
THEORY13–17 уже обсуждают packet path, padding/mux, причинность/стоимость
измерений и admissibility. **Здесь проверяется стык стадий:** остаётся ли
защитное утверждение верным, если один наблюдатель связывает bootstrap,
начальный handshake, поздние потоки того же carrier и результаты своих probes?

Результат — три дополнения, labels исследования, не product backlog:

- **R02-04:** joint evidence нельзя заменять независимыми оценками фаз;
  успешная маскировка первых records не аттестует поздний mux stream.
- **R02-05:** реальная область существующего padding и готового Naive cover
  различается по carrier/stream/событию и по обработке отказа cover.
- **R02-06:** Maybenot пригоден как scheduler, но его limits не являются
  byte/deadline/OS-safety контрактом; проверен конкретный интегратор D04.

Новых российских полевых фактов, результатов классификации или native proof нет.

## 2. Единая цепочка наблюдения

Обозначим `O=(B,H,S,A,Z,K)`:

- `B`: bootstrap/distribution/resolver наблюдения до полезного подключения;
- `H`: destination, транспорт, открытые handshake поля и ранний pattern;
- `S`: steady/late payload statistics, направления, длительность, reuse/churn;
- `A`: активные действия и ответы с указанием, **почему/когда** probe был выбран;
- `Z`: доступный observer context — сеть/дата/маршрут/видимость/фон;
- `K`: знания — public metadata, endpoint/path, outer credential, service credential.

Наличие такого tuple в модели не означает, что каждый цензор реально видит всё.
Различать: (1) возможно наблюдать; (2) есть устойчивый classifier; (3) можно
подтвердить гипотезу probe; (4) выгодно/возможно блокировать нужную единицу.
Оператор может подавить целый IP/ASN/carrier без установления VPN для каждого flow.

### 2.1. Ручной контрпример: одинаковые фазы, отличимое сочетание

Синтетические binary признаки H/S, **не реальные пакеты**:

| H,S | P(benign) | P(tunnel) |
|---|---:|---:|
| 0,0 | 1/2 | 0 |
| 0,1 | 0 | 1/2 |
| 1,0 | 0 | 1/2 |
| 1,1 | 1/2 | 0 |

В обоих классах marginal H и marginal S равны Bernoulli(1/2), но отношение
H=S различает классы полностью. Это ручная вероятностная конструкция,
без simulation: она опровергает достаточность отдельных marginal checks,
не доказывая такой RF classifier. При общем support:

`L(H,S | Z,K) = L(H | Z,K) × P(S | H,tunnel,Z,K) / P(S | H,benign,Z,K)`.

Второй множитель нельзя заменить marginal likelihood без нужной независимости.
Probe, выбранный после H/S, тоже не является независимым повторением passive
evidence. Идеальный classifier может игнорировать дополнительный признак;
поэтому его минимальный 0–1 risk при полной joint view не выше marginal risk.
Реальный обученный classifier может проиграть из-за данных/overfit/стоимости —
теорема не является обещанием практического детектора.

### 2.2. Единица подавления шире flow

Если противник действует по endpoint, одного замеченного соединения может быть
достаточно для его классификации. При N событиях вероятность хотя бы одного
flag ограничена сверху суммой individual probabilities даже при зависимости.
Формула `1−(1−p)^N` требует дополнительной независимости и одинакового p.
Ни N, ни p для нашей сети не измерены. Даже per-flow FPR нельзя автоматически
превратить в endpoint/account FPR. Это уточнение единицы анализа для R03.

## 3. Joint-observer matrix

Baseline код B01 — pinned U01–U04 предыдущего SOURCES, текущие local snapshots.
Windows WS preflight имеет caller в corebridge.go: enabled WS → Resolve →
IP override, при ошибке остаётся hostname dial. HY2/REALITY dial origin по IP.
Это source chain, не доказанная OS-маршрутизация bootstrap; OP-02 остаётся в силе.

| Наблюдатель / knowledge | WS/CDN: bootstrap → H → поздний S → probe | TCP/REALITY no-flow: B → H → S → probe | HY2/Salamander: B → H → S → probe |
|---|---|---|---|
| Access passive, без credentials | Может связать gateway DNS/resolver timing с TCP к CDN, CH/ALPN и долгоживущими WS bursts; HTTP headers за TLS не читает | Origin IP/порт + SNI donor + CH/ранние и поздние bursts; hostname donor не скрывает destination | Origin/UDP/порт + obfs lengths/timing/hopping; стандартное извлечение QUIC Initial без obfs key не применимо, но shapes остаются |
| Частичная/односторонняя view | Не обязан видеть resolver и оба направления; совместная метка требует реальной возможности связать наблюдения | Direction loss разрушает часть burst features; долгий flow сам по себе не proof | Одно направление/потери меняют наблюдаемое распределение; отсутствие ответа не диагноз UDP block |
| Активный blind | Обычная TLS/website поверхность и неверный path могут быть cover; правильный путь не следует из шифрованного trace | Неверный внешний admission проходит другую ветку, чем валидный REALITY | Без правильного obfs PSK не тот же H3 handler, что при PSK-known probe |
| Активный outer-informed, без service credential | Path/outer TLS известны, но VLESS UUID отдельно; наблюдается только доступная ему стадия | Валидный внешний admission не равен валидному VLESS UUID; donor fallback уже не универсальное объяснение | Правильный obfs PSK позволяет исследовать QUIC/H3 и неверный service auth; это сильнее blind модели |
| Подписчик с valid service credentials | Может подтвердить известный ему endpoint, но не автоматически перечислить остальные | Маскировка не запрещает известному авторизованному клиенту узнавать собственный сервис | То же; новая obfs сама по себе не решает credential distribution/revocation |
| TLS terminator/CDN или origin operator | CDN видит внешний HTTP/WS и может связать с edge metadata; inner HTTPS plaintext автоматически не получает | Внешний TLS endpoint = origin; donor и proxy имеют разные роли | Proxy origin видит свою HY2 сессию; шифрование полезного end-to-end HTTPS остаётся отдельным |
| Противник с возможностью carrier/IP/UDP denial | Качественный HTTP cover не отменяет carrier policy; origin outage общий | IP/порт/ASN block не требует burst classifier; новый handshake требует donor availability | Общий UDP/origin block не исправляется padding; hopping не меняет origin/account |

Ключ для join: фактическая видимость + возможность связать endpoint/flow/time,
а не совпадение одного названия транспорта. Colluding CDN/access/global observer
не предполагается автоматически. Модель compromised endpoint вне этого среза.

## 4. R02-04 — поздний поток пересекает границу защитного утверждения

**Механизм.** Пользовательские TLS handshakes могут возникать не только в начале
carrier, но и при новом stream через уже открытый mux. По D01:

`Windows renderer opt-in mux → sing-mux Client.offerNew → newProtocolConn →
newPaddingConn → newClientSession → многие streams на одном wrapper`.

`paddingConn` хранит свои read/write counters; первые16 padding records получают
framing, после этого `write`/WriteBuffer/WriteVectorised больше его не добавляют.
Открытие нового logical stream на **том же** carrier не создаёт новый wrapper.
Счётчик не равен HTTP-запросам, TLS records или IP packets.

**Ручной trace:** C1 открыт; ранняя обычная нагрузка исчерпала16 records; C1 ещё
пригоден по mux policy; создан новый logical stream; его inner TLS handshake
идёт через уже исчерпанный wrapper. Это source-valid counterexample к фразе
«mux padding скрывает каждый inner handshake». Он не доказывает успешный remote
detector: внешний TLS/WS, scheduler, конкурирующая нагрузка и segmentation меняют view.
При открытии **нового** carrier C2 counters начнутся заново; этот случай не смешивать.

**Цена существующей защиты, ручной вывод.** На error-free write-path
каждого такого record header4 bytes и `padding=256+U{0,…,511}` bytes.
Для k≤16 записей одного направления:

- added bytes `O_k = Σ(4+padding_i)`;
- `260k ≤ O_k ≤ 771k`, `E[O_k]=515.5k` при указанном uniform sampler;
- для k=16: **4160…12336 bytes**, expectation **8248 bytes**.

Это wrapper byte accounting, не wire overhead или benchmark. Framing/protocol
header, WS/TLS/TCP/IP, retransmits, read/write batching учитываются отдельно.
Vectorised и WriteBuffer ветки используют тот же counter/range; вывод не
аудирует корректность всех I/O веток и ошибок. Пустые записи/short writes/errors
не включены в численный error-free trace.

**Novelty:** вместо общего «padding ограничен» установлены scope/reset/caller и
точные source constants. Ни один прежний AD/OP/TR вывод этого reset boundary не
аттестовал. **Применение:** TH-05/E-04, AND-18; R03 не должен считать carrier-age
доказательством protection от traffic analysis, R04 получает отдельную единицу costs.

**Сильнейший контраргумент:** aggregate multiplexing само по себе может скрыть
handshake без дополнительного padding. Принят: это отдельный workload-dependent
эффект, который может сохраниться после16 records. Source finding не говорит,
что весь канал «раскрыт», и не рекомендует ротировать все carriers после16.
Rotation увеличивает handshake overhead/churn и может добавить fingerprint.

## 5. R02-05 — готовый behavioral cover меняет механизм, но не все фазы

### 5.1. Наивное копирование padding не равно NaiveProxy

D02 использует реальный Chromium stack и HTTP CONNECT. Caller `DoAcceptComplete`
для cold/expired session ведёт в `DoPreamble`; warm path вызывает `StartOne`.
Getter делает настоящий root GET, извлекает same-host/port ресурсы, использует
browser headers и HTTP API, отличающий обычный GET от CONNECT payload.
Это изменяет наблюдаемую внешнюю application history; просто добавить UA в WS
такой эффект не воспроизводит. Admission/fronting и peer padding negotiation
относятся к серверному контракту, который здесь не проверен.

**Контрпример отказа cover:** `DoPreambleComplete` логирует ошибку и всё равно
переходит к Connect. Следовательно, возможность полезного CONNECT ещё не
доказывает, что ожидаемый preamble сработал. Это не direct fallback и не доказанная
утечка: транспортный protection outcome и camouflage outcome надо разделять.

В `NaiveConnection::DoConnectServerComplete` создаётся `NaivePaddingSocket` для
данного stream. В Variant1 первые8 frames padded, потом write path unpadded.
Это **другой reset scope**, чем один global mux wrapper D01. Он может покрыть
начало позднего stream, но не обещает hiding всего steady payload этого stream.
Направление/размер записи влияет на random range; README simplified description
не заменяет source. Header3 и padding≤255 дают **не более2064 added bytes на
восемь frames одного направления**, исключая preambles и прочие framing layers.
Uniform mean130.5 bytes/frame допустим только для uniform ветки; общий mean
без распределения sizes/directions не установлен.

`WritePaddingV1Drain` в ветке `direction_==kServer` и
`200<consumed_payload<1024` также дробит попытки transport.Write на100…200 bytes.
Это user-space write granularity, не гарантированный размер IP/TLS packets:
нижние слои могут объединять/сегментировать writes. Такой механизм требует
целого совместимого stack, а не настройки размера TCP-пакета в renderer.

**Сильнейший оппонент:** любой preamble может быть распознан как preamble или
коррелировать с CONNECT; fixed root/resource set может отличаться от реального
браузинга. Принят: D02 меняет конкретные observable features, но источник не
доказывает matching joint distribution всех фаз/дат/клиентов. Хороший benign
corpus, failed-cover cases и поздние streams обязательны. Ни claim README
«defeated», ни release tag не являются нашим comparative PASS.

### 5.2. Privacy shaping не равно protocol camouflage

D04 показывает другой механизм: IP data padding до encapsulation, decoys и
задержки с обратной обработкой peer. Реальный caller device/mod.rs передаёт
normal packet в hook, затем в tunnel, затем в on_tunnel_sent и UDP send.
Incoming hook распознаёт и убирает decoy перед TUN.

Но on_tunnel_sent пропускает non-data/keepalive мимо delay logic. Поэтому этот
конкретный DAITA hook не превращает WireGuard handshake в browser TLS. Даже
удачное снижение website fingerprinting не доказывает сокрытие самого VPN
или обход protocol block. Это source-scope ограничение GotaTun pin, не вывод
о каждом клиенте/версии Mullvad. Прямой Q02 eval также имеет другой workload,
transport и geographies; его показатели не являются нашими прогнозами.

**Практический вывод:** выбирать цель до заимствования: cover внешнего
handshake/HTTP behaviour, защита inner workload pattern или availability
endpoint — разные objectives. Их нельзя закрыть одним именем «DAITA/Naive/obfs».

## 6. R02-06 — scheduler нуждается в более сильном adapter contract

### 6.1. Что реально делает Maybenot 2.2.2

D03 получает events и возвращает actions; интегратор исполняет timers, decoys,
blocking и сообщает результат. Generic `SendPadding` не определяет wire format
или допустимую точку вставки в TLS/QUIC. Существующие HY2 public methods B01
не предоставляют готовую SendPadding/Delay scheduler integration.
Произвольные дополнительные bytes на WS/VLESS/QUIC socket не эквивалентны
валидному peer-recognized dummy; формат и authentication надо сохранять.

В source `below_limit_padding/blocking`:

- fraction проверяется только при `>0`; **0 не означает запрет actions**;
- `allowed_padding_packets` / `allowed_blocked_microsec` дают allowance до
  применения fraction checks; это не quota cap;
- blocking replacement имеет отдельную ветку, не подчинённую тем же average
  limits; state limits и machine design сохраняют собственную семантику;
- NormalSent/PaddingSent считаются на enqueue/intention stage; TunnelSent
  отдельно сигнализирует near-wire stage, но не заменяет этот accounting.

Это documented/source semantics, **не утверждение об уязвимости библиотеки**.
Например, default D04 с пустым списком machines не генерирует actions; из нулевых
settings нельзя вывести действие без machines/состояний. Сама validity machine
не доказывает, что её policy укладывается в наши hard resource/SLO boundaries.

### 6.2. Почему packet fractions недостаточно

Ручной dimensional counterexample: один normal packet100 bytes и один padding
packet1000 bytes имеют padding packet fraction1/2, но padding byte fraction
1000/1100≈0.909. Это искусственный пример единиц, не measurement. При одинаковых
размерах выбранной наблюдаемой единицы доли могут совпасть; retransmits/headers/
служебные packets и actual send accounting тогда всё равно требуют определения.

D04 подтверждает проблему различения events и wire: `handle_decoy` сообщает
PaddingSent **до** веток replacement/queue/full-drop/реальной отправки.
В replace ветке может уйти normal packet, либо ничего нового не уйти.
Библиотечный счётчик поэтому не объявляется счётчиком фактических cover bytes.
D04 ведёт отдельные byte meters — полезный reference seam. Его event/action
channels unbounded, хотя delay queue bounded: копирование интеграции не доказывает
bounded memory всей нашей будущей системы. Никакого OOM/runtime дефекта здесь
не воспроизводилось.

RateLimitedFramework ограничивает темп actions, не непосредственно bytes,
queue age, energy или total memory. `BlockOutgoing`/`bypass` в этом API относятся
к shaper scheduling, **не к WFP/TUN fail-closed и не к разрешению direct**.

### 6.3. Нижняя граница цены непрерывного cover

Предпосылки модели одного направления: одинаковый observable packet size s bytes,
один send slot каждые Δ seconds, полезная capacity q bytes/slot, dummy заменяем
normal при наличии очереди. Тогда service ceiling `μ=q/Δ` bytes/s. При отсутствии
drop и cumulative accepted payload A(T) bytes backlog не меньше
`max(0,A(T)−q·floor(T/Δ))` bytes с выбранным началом слотов. Поэтому `λ>μ`
несовместимо с bounded backlog на бесконечном интервале, если долгосрочная
arrival rate `λ=lim A(T)/T` существует, в bytes/s. `λ<μ` даёт запас,
но не гарантирует deadline или отсутствие burst overflow; равенство допускает
особые детерминированные schedules и не объявляется всегда неустойчивым.
Номинальный total rate `r=s/Δ` bytes/s. За n idle слотов, `T=nΔ`, cover стоит
ровно `ns=rT` bytes в этой модели; для произвольного интервала нужны поправки
на границы слотов. Это не измерение фактического packet rate.

Если hard extra-byte budget C конечен, continuous idle cover возможен лишь
при `T≤C/r` для r>0 и указанного slot-aligned T. Нельзя одновременно обещать положительный постоянный cover,
произвольно долгий idle и конечную квоту. После исчерпания бюджета надо объявить
scope/degradation, а не скрывать смену поведения. Packetization/PMTU/native loss
могут разрушать модель fixed observable size; здесь нет numerical rate preset.

Доля времени blocking также не задаёт maximum packet delay: в идеализированной
average модели разрешённая продолжительность растёт с длительностью истории.
Следовательно, нужен отдельный per-packet deadline/queue-age cap; это не
рассчитанный bound на D03, а причина не использовать average fraction как SLO.
Battery cost не выводится из bytes: radio state и wakeup scheduling — вопрос R04.

**Adapter proposal:** approval-bound machine/version; finite machine/action/timer
counts; monotonic context clock; per-epoch cancellation; separate byte/rate/queue/
deadline caps; actual-send meters; peer-recognized framing; явно описанное поведение
при cap/error. При отказе shaper прекращается optional cover, а защита user traffic
и platform admission не ослабляются. Обязательность cover для конкретного профиля
должна быть отдельной policy, не неявным значением подключения.

## 7. Сравнение готового reuse и независимости

| Подход | Какой механизм меняет | Что не меняет / цена | Решение |
|---|---|---|---|
| Existing sing-mux D01 | Finite prefix sizes + естественное смешение streams | Не полный steady shaper; wrapper reset scope; лишние bytes, HoL/churn tradeoff | Reuse baseline и уточнить доказательную область; не усиливать глобально без замера |
| NaiveProxy D02 whole stack | Chromium outer behaviour, GET preambles, negotiated stream-prefix padding | Полное lifecycle/build/server/SBOM; cover может отказать при живом CONNECT; IP/carrier block остаётся | При принятом сценарии TH-05 compatibility spike; не копировать одну функцию |
| Maybenot D03 | Programmable event-dependent decoys/delays | Требуются формат/таймеры/peer integration, новые CPU/queues/ABI расходы; default numbers не наш budget | Scheduler reuse candidate после transport seam; не новый pure policy authority |
| GotaTun/DAITA D04 | Пример size-padding + decoys + metering/queues в WireGuard | Другой engine/API, handshake не замаскирован этим hook; unbounded channels требуют review | Reference/adapt seams, не подмена текущего ядра |
| Salamander / uTLS / Vision B01 | Отдельные bytes/handshake/encapsulation свойства | Не substitute для phase-spanning cover; R02-001 support≠включено≠native verified | Сохранить прежние оговорки; не повторять как новую рекомендацию |

Эти методы не являются независимыми резервами от outage одного VPS/carrier и
могут конкурировать за один CPU/link/battery budget. Independence надо оценивать
от выбранного механизма: изменение bytes без изменения timing не покрывает
timing criterion; хороший steady cover не создаёт destination reachability.
Одинаковый fallback/account остаётся общим cut независимо от числа shapers.

## 8. Применимость и вопросы соседним направлениям

**Windows:** current renderer и validator остаются прежними source-only inputs.
Существующий mux scope можно документировать без новых зависимостей. Naive
sidecar потребует owned process/image, socket bypass/protection, shutdown/update
и server contracts; наличие Windows binary upstream не закрывает TH-01/05.
Maybenot FFI/новый scheduler не интегрированы. Никаких изменений WFP/proxy/DNS.

**Android:** не наследовать Windows preflight/UA/wrapper path. Native source lock
по-прежнему не доказывает совместимый AAR. External/native integration обязана
сохранять protected socket ownership, epochs/cancellation/foreground lifecycle;
cover увеличивает потенциальные radio wakeups/traffic. У GotaTun другой Rust
toolchain/API; установка или смена Android core не предлагается.

**R03:** phase/knowledge/context и actual outcome до causal attribution;
no double-count joint evidence; action requested / queued / replaced / emitted /
useful task complete раздельны. Возможная разница phase distributions — гипотеза,
не новый hard admissibility score.

**R04:** единицы prefix records, logical streams, wire packets, bytes/time,
queue age и radio wakeups не смешивать; измерить cold/warm/late/sparse/bulk при
сохранённом фоне. Матрицы byte caps/HoL не назначают numerical presets здесь.

**R05:** machine provenance/limits и transport peer negotiation отдельны от trust
authority; `bypass` shaper не bypass барьера; отказ optional cover не выпускает
user traffic direct. Admission/profile epochs должны пережить timer completion.

## 9. Конечные будущие проверки — все NOT RUN

| Case | Конечная проверка / falsification | Сильный отрицательный контроль |
|---|---|---|
| J1 | На одном зафиксированном corpus сравнить H-only, S-only и joint; train/held-out по дням/endpoint, определить loss и budgets заранее | Если joint advantage исчезает при правильном split, не принимать joint classifier gain; manual XOR не evidence dataset |
| J2 | Blind / path-informed / outer-credential / full-service роли на своих endpoints; фиксировать selection of probes | Разные knowledge и direction visibility не объединять в один FPR; timeout с неверной локальной конфигурацией |
| P16 | Cold carrier, затем исчерпанный wrapper, затем late logical stream на том же ID и отдельно новом carrier | Не считать first16 internal records first16 network packets; учесть carrier reassignment и background mux |
| NP8 | Пер-stream padding negotiation, small/large/partial writes, late streams; actual extra bytes | Отсутствующий padding peer, error/disabled preamble, empty/broken root и repeated resource set |
| M0 | finite valid machines со fractions0/positive, startup allowance, replacement и hard adapter caps | Не интерпретировать0 как disabled; no-machine control; не выдавать validity за safe resource policy |
| MA | TriggerAction→timer→queue→replace/drop/send→wire metrics с fake/no-send control в разрешённом стенде | Duplicate/stale/cancelled action, queue-full, sender error; counters не расходятся по units |
| L1 | Bounded idle cover, hard quota, max queue age, PMTU change, CPU/battery/task latency | Quota exhaustion/overload снижает заявленный cover scope; direct запрещён; cap/recovery не притворяется full defence |
| RF | После отдельного разрешения paired benign/proxy workload в конкретных РФ сетях, днях и обоих направлениях | Общий origin/carrier outage, ordinary web breakage, protocol config mismatch и old source snapshot |

Никакие команды/fixtures/benchmarks/simulation для этой таблицы не создавались
и не запускались. Статистический дизайн J1 должен быть принят R03; performance
инструментирование R04, OS/native/RF checks остаются отдельной авторизацией.

## 10. Предлагаемая вставка руководителю и последовательность

В §2.4/§9.3 и E-04, без удаления материала R02-001:

> Уточнение R02-002: защитное утверждение относится к выбранным observer,
> knowledge class, фазам и единице carrier/stream. Совпадение отдельных
> marginal profiles не доказывает сходства joint trace. В pinned sing-mux0.3.5
> padding ограничен первыми16 records одного wrapper; новый stream на том же
> carrier не сбрасывает counter. NaiveProxy имеет другую stream-prefix и
> preamble механику; его preamble error не блокирует Connect. Maybenot задаёт
> actions, но не wire-byte/deadline/OS-protection contract: fraction0 не означает
> disabled, allowances и replacement имеют отдельную семантику. Заимствование
> требует peer-compatible framing, hard adapter budgets и actual-send metering.

Порядок предложений: (1) принять/уточнить scope assertions без продуктовых
изменений; (2) R03/R04/R05 согласуют observer/outcome/budget контракт;
(3) после P0 и разрешения E-04 сравнить нынешний mux с одним выбранным whole-stack
кандидатом; (4) programmable shaping только при показанном дополнительном
выигрыше, сохранённом protected path и допустимых resource costs.

Это порядок исследований/предложений TH-05/06/E-04, AND-01/18, COM-18,
не разрешение реализации и не закрытие этих product gates.

## 11. Самостоятельное оппонирование и граница знания

[OPPOSITION](../../../../.artifacts/research/theory-02/2026-10-25-r02-002/OPPOSITION.md)
содержит явные возражения/коррекции и final relevant source drift.
Нет доказательства, что данный joint classifier развёрнут в РФ; нет победителя
по защите/скорости/энергии или практически рассчитанного detection probability.
Полный Naive server/certificate path и Maybenot/GotaTun native integrations не
аудированы. Статус **READY_FOR_REVIEW**, решение ACCEPTED остаётся руководителю.
