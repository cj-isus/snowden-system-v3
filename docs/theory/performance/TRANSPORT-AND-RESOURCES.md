# Скорость, восстановление и ресурсы

[Оглавление](../README.md) · [Термины](../GLOSSARY.md) · [Учёт QUIC/HY2](QUIC-ACCOUNTING-AND-LIMITS.md)

## Краткий вывод

Скорость определяется узким местом всего пути и полезным завершением задачи.
Увеличение окон, mux, offload и частые повторы помогают лишь при подходящих
условиях, добавляя собственные очереди и ресурсы. Сначала прослеживают
реальный метод, сокет и владельца данных, затем меняют одно ограничение.

**Когда применять.** При выборе оптимизации, ограничении памяти и очередей,
подготовке миграции пути и оценке маленького VPS или батареи телефона.

**Граница знания.** Исследованные исходники и снимки относятся к 23–28 октября 2026 года.
Размеры, окна и примеры ниже — параметры тех версий либо условные расчёты,
не нынешняя настройка приложения и не измеренный выигрыш.
R04-13 принят в области несовпадения контрактов; R04-14 требует уточнения
критериев совместимости. [Точная область решений](../research/reviews/2026-10-28-DEEP-REVIEW.md).
Подробный разбор счётчиков, HY2-сборки и времени другой стороны находится
в [основном дополнении QUIC](QUIC-ACCOUNTING-AND-LIMITS.md).

В этой главе: [модель затрат](#source-9-1), [выбор мер](#source-9-2),
[готовые решения](#source-9-3), [MTU и очереди](#source-13-3),
[фактические вызовы](#source-14), [жизненный цикл заимствованных данных](#source-18),
[передача событий контроллеру](#reviewed-brutal-dispatch).

<!-- source-unit: 9 -->
<a id="source-9"></a>
## 9. Скорость: физические ограничения и полезные оптимизации


<!-- /source-unit: 9 -->

<!-- source-unit: 9.1 -->
<a id="source-9-1"></a>
### 9.1. Модель затрат

[Инженерные приближения, не предсказание конкретного результата.]

```text
goodput = полезные завершённые bytes / wall time
goodput ≤ min(access, entry path, relay CPU, relay↔exit, exit↔target, target)
BDP = bottleneck bytes/s × RTT seconds
carrier throughput ≲ in-flight window / RTT  (если именно окно ограничивает)
recovery = обнаружение + очередь/cooldown + prepare/apply + fresh gate
```

Не складывать latency всех стадий вслепую: соединения/резолв могут переиспользоваться,
часть работы параллельна. Отдельно cold start, warm reuse, time-to-first-byte,
полное завершение и latency под нагрузкой. Speedtest с одним длинным скачиванием
не описывает интерактивный browsing и не включает цену отказавших попыток.

Короткие carriers уменьшают накопление байтов, но добавляют RTT, crypto/CPU,
NAT/port state и риск rate limit. При условном лимите `B` байт/carrier и полном
времени цикла `T` последовательный поток ограничен `B/T`; параллелизм может
помочь лишь до общей ёмкости, лимитов и новой сигнатуры. Не предлагать
«новый TCP каждые 16 КБ» без transport reassembly и проверки end-to-end семантики.


<!-- /source-unit: 9.1 -->

<!-- source-unit: 9.2 -->
<a id="source-9-2"></a>
### 9.2. Где искать выигрыш

| Мера | Возможный эффект | Цена / обязательный контроль |
|---|---|---|
| Меньше лишних reload/probe/повторов | recovery и CPU без смены протокола | не пропускать fresh gate, cancellation/epoch atomicity |
| Mux / bounded pool | меньше handshakes | TCP-level head-of-line, общий failure blast radius, очереди и bytes/carrier |
| QUIC streams | потери одного stream не требуют упорядоченной доставки другого | общий congestion controller/path; UDP block/MTU/NAT; не обещает отсутствие задержек [S12] |
| Надёжный внешний byte stream для TUN | совместимость TCP-пути | outer loss задерживает внутренние потоки и может вызывать повторные retransmission; отличать packet-over-TCP от split proxy |
| MTU/PMTUD и размеры буферов | устранение blackhole/оконного bottleneck | family/encapsulation overhead, oversized buffers → latency/RAM; E-05 |
| Congestion control / bandwidth caps | лучшее использование loss-path | rate выше доступного вызывает очереди/потери, измерять соседний поток; Brutal не default [S13] |
| Padding / timing shaping | меньше выбранных статистических признаков | bytes/CPU/latency и новые fingerprints, абляция и benign corpus [S03] |
| FEC / дублирование | latency при части случайных потерь | больше wire bytes, не лечит полный drop; не дублировать неидемпотентные запросы |
| Keep-alive / resumption / TFO / 0-RTT | idle recovery/handshake в некоторых условиях | расход сети, replay/idempotency, несовместимость middlebox; не replay control-plane Apply [S06] |

QUIC connection migration [S23] может сохранять сессию при смене адреса/пути, если
это поддерживают оба конца; не означает миграцию в другой независимый VPN exit
и не преодолевает заблокированный destination IP. Для SOCKS отдельно выяснять
поддержку UDP у приложения и прокси; для TUN — ОС-маршрут/DNS/IPv6, не браузерный mock.


<!-- /source-unit: 9.2 -->

<!-- source-unit: 9.3 -->
<a id="source-9-3"></a>
### 9.3. Что разумно заимствовать и когда

Это список кандидатов для оценки, **ничего автоматически не устанавливается**.
Меняющаяся документация содержит возможности вне исследованного пина; фиксировать tag/commit,
лицензию зависимостей, wire compatibility и rollback до любой интеграции.

| Источник | Что брать | Ограничение / решение |
|---|---|---|
| NaiveProxy [S14] [S15] | Сетевой стек Chromium, согласование заполнения и срок жизни несущего соединения; release v147.0.7727.49-3 добавляет realistic preambles | Это предварительный выпуск для сравнения, не выбранная поставка. Принципы → отдельный compatibility spike: pin, packaging, child process/kill-switch, server capability. Корневая BSD-style лицензия не отменяет third-party notices |
| Xray XHTTP [S16] | Разделение up/down, ограничение reuse, разные пути и HTTP transport modes | Не включён в sing-box 1.13.19; не менять ядро/встраивать sidecar до TH-04 и ADR с измерением. Дизайн upstream не даёт универсальной устойчивости |
| Hysteria2 [S11] | Согласование rate, QUIC/TCP+UDP proxy, HTTP/3 cover либо obfs как разные профили | Salamander скрывает QUIC wire image ценой несовместимости с обычным H3; не обходит общий UDP-block. Gecko/Realm/Mimic из просмотренной меняющейся документации не приняты нашим pin |
| Tor WebTunnel/Snowflake [S17] | Разделение транспорта, distribution и проверки свежести мостов | Не рейтинг и не готовый высокоскоростной backend VPN; runtime/dependencies/trust/нагрузка оцениваются отдельно |
| UPGen, USENIX 2025 [S18] | Диверсификация семейств протоколов как research alternative к единой FEP-сигнатуре | Результат авторов на своём threat model; не доказательство работы в РФ и не production-приоритет |
| TUF [S20] | Роли доверия и threat-driven update regression matrix | Применять к OFF-06 после инвентаризации envelope; не заявлять TUF compliance по сходству |
| Toxiproxy [S21] / iperf3 [S22] | Готовые TCP fault injections / эталонные controlled throughput measurements | MIT / BSD-3-Clause. Toxiproxy не эмулирует UDP/DPI/WFP; iperf3 не доказывает обход, Windows официально не поддерживается |

Важное новое направление из практики NaiveProxy: обновлять не только ClientHello,
но и поведение соединения. Preambles/shaping также имеют расход и могут создавать
новую сигнатуру; польза проверяется, а не принимается из маркетингового README.
Большую генеративную traffic-shaping систему или собственную криптографию до
этих сравнений не разрабатывать.


<!-- /source-unit: 9.3 -->

<!-- source-unit: 13 -->
<a id="source-13"></a>
<a id="reliability-depth"></a>
## 13. Надёжность и полезная скорость: детали обеих платформ

Срез от 2026-10-23 углубляет §§7–12;
требования Windows/общего контура — PLAN §5.6 — `docs/PLAN.md#reliability-research`,
Android — ANDROID-PLAN §8.1 — `docs/ANDROID-PLAN.md#android-reliability-depth`.
Evidence и расчёты — `docs/evidence/2026-10-23-deep-reliability.md` отделяют прочитанный
исходник, математический контрпример и будущую native/сетевую приёмку.
Прочитанный исходник и математическая модель не подтверждают
работоспособность VPN в реальном окружении.


<!-- /source-unit: 13 -->

<!-- source-unit: 13.1 -->
<a id="source-13-1"></a>
### 13.1. Что означает «быстрее и стабильнее»

Оптимизируем защищённую **полезную** доставку: время первого успешного действия,
p50/p95/p99 задержки под нагрузкой, goodput, число оборванных рабочих сеансов,
время восстановления и количество ложных переключений. Mbps без idle/loaded latency,
CPU/энергии/байтов и отказов не описывают комфорт. Безопасность — ограничение,
не слагаемое score, которое можно компенсировать скоростью. Tail latency особенно
важна при последовательных зависимостях и высокой загрузке [D16].

Сначала определить bottleneck: радио/оператор, RTT/потери/PMTU, VPS CPU steal/лимит,
криптография, копирование/GC, flow-control window, очередь или удалённый сервис.
Переносить нагрузку на более мощный CPU полезно лишь при подтверждённом CPU bottleneck.
Для телефона добавить screen-off/roaming/OEM; для Windows — startup/resume/worker/
TUN/WFP и native WebView. У каждой метрики есть denominator и наблюдаемая область.


<!-- /source-unit: 13.1 -->

<!-- source-unit: 13.2 -->
<a id="source-13-2"></a>
### 13.2. Закреплённый код важнее названия оптимизации

Прочитаны project pins: sing-box1.13.19; SagerNet/quic-go0.59.0-sing-box-mod.4;
sing-quic `d83826c306d7`; sing-tun `7c73233bd0fb`; x/sync0.20.0.
Полные commit/file/blob/SHA256 и лицензии — evidence. На момент среза 23 октября libbox был закреплён по исходникам,
но состав поставляемого Android AAR ещё не был подтверждён этим исследованием.
Это датированная граница: она не означает отсутствие позднее собранного AAR.
Граф Windows go.mod автоматически не подтверждает состав Android-артефакта.

| Наблюдение в выборке 23 октября 2026 года | Практическое следствие |
|---|---|
| QUIC `sys_conn_windows.go` возвращает basicConn; тот объявляет DF, но не GSO/ECN | Windows USO/ECN в ОС не означают их использование этим форком. Сначала capability фактический путь сокета, затем сравнение; не включать фиктивный UI switch |
| QUIC `wrapConn` проверяет OOBCapablePacketConn; send_conn имеет fallback после GSO error | Opaque obfs/protect wrappers могут изменить fast path. Сохранять датаграммные границы, буферное владение, DF/MTU и fallback; не обходить обёртку и её шифрование ради SyscallConn |
| HY2 client задаёт окна 8MiB stream/20MiB connection и keepalive10s из hysteria constants | В upstream уже есть расходы и таймеры. Ещё один app-level keepalive/«ускоритель окон» может дублировать их; лимиты flow control не равны немедленному выделению такой памяти |
| HY2 включает Brutal только при `!authResponse.RxAuto && actualTx > 0`; иначе BBR-family sender | Положительной rate недостаточно: server RxAuto сохраняет adaptive branch. Linux TCP `tcp_congestion_control=bbr` не меняет userspace QUIC CC; произвольный завышенный Up/DownMbps меняет алгоритм и конкуренцию |
| sing-box уже имеет parallel dialer, resolver cache/coalescing и mux client | Развивать их через supported API. Не добавлять внешний второй Happy Eyeballs/cache/mux с противоречивыми таймерами/политиками |
| Windows renderer держит TCP keepalive60s/20s, mux opt-in/max4/min4, WS early-data2048 | Это параметры датированного среза, не измеренные оптимумы. TFO уже исключён после прежнего Windows incident; OS support не равен поддержке pinned библиотеки |
| Android path fingerprint отделяет VALIDATED hints, сортирует адреса/маршруты, но включает metered/roaming/DHCP/proxy | Хорошее ограждение контекста может быть слишком широким restart key. Проверить отдельно security/path/resolver/billing/presentation changes; не выкидывать поля из admission fence ради скорости |

**RQ-01: сначала выражение, затем достижимость.** В векторном методе
закреплённого Salamander условные входы двух буферов `[8,3]` и `[8,64]`
показывают выход за границы из-за вычисления индексов. Это ручной контрпример,
не исполненный crash приложения. Более поздний разбор проследил обычные
ветки и не нашёл вызова этого проблемного метода в них: [цепочка вызовов](#source-14-1).
Поэтому прежнее «достижимость ещё не прослежена» уточнено для названных веток;
оно не распространяется на любой сторонний caller или будущее ядро.
[Закреплённый Salamander](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/salamander.go).

<!-- /source-unit: 13.2 -->

<!-- source-unit: 13.3 -->
<a id="source-13-3"></a>
### 13.3. PMTU, IPv6, фрагментация и offload

Различать TUN MTU, outer path MTU, UDP payload, QUIC packet и HY2 application
message. QUIC требует доступности UDP payload≥1200; на outer IPv6 path MTU1280
после 40-byte IP и 8-byte UDP остаётся 1232. Протокольный overhead уменьшает payload
дальше. В модели с условными 64 bytes остаётся 1168, но это **не** основание назначить
inner IPv6 MTU1168. Нужны подходящая application fragmentation/reassembly,
stream representation или другой путь. Число 64 иллюстративное, не HY2 спецификация. [D01] [D02]

У закреплённого Salamander overhead известен точнее: `salamanderSaltLen=8`,
по 8 bytes на внешнюю датаграмму. При outer IPv6 PMTU1280 без extension headers
после IP/UDP/salt остаётся **1224 bytes для QUIC packet**, ещё до QUIC/HY2 headers
(M05). Проверять, что probe size/max packet size и wrapper overhead согласованы;
сырой UDP budget1232 нельзя автоматически использовать как QUIC packet budget.
Это расчёт по прочитанному исходнику, не воспроизведённый PMTU defect или настройка TUN.

DPLPMTUD использует подтверждённые пробы и recovery от blackhole вместо доверия
одному ICMP/PTB. Оно не исправляет UDP block/allowlist. Проверять уменьшение MTU
после handover, асимметрию, отсутствие/подделку PTB, size-dependent loss отдельно
от volume-dependent throttling. Кэш PMTU привязан к конкретному path/endpoint,
а не стране или всему профилю. В pinned HY2 есть application fragment fields;
проверить bounds/count/TTL/duplicate/out-of-order/reassembly memory, не писать новый
fragmenter рядом. У QUIC DATAGRAM нет обязательной повторной передачи payload. [D01] [D04]

GSO/GRO уменьшают syscall/packet overhead, но не разрешают больший MTU на проводе.
Batch должен иметь допустимые адрес/размер/segment boundaries; fallback не теряет
и не дублирует packet. Возможность Linux TUN offload из sing-tun не доказывает
доступность для Android fd, выданного VpnService. На Windows Wintun и QUIC UDP
обрабатываются разными слоями. Оценивать CPU/packet и tail latency при том же goodput,
не объявлять «zero-copy» по наличию pooling или vectored API. [D05]


<!-- /source-unit: 13.3 -->

<!-- source-unit: 13.4 -->
<a id="source-13-4"></a>
### 13.4. Окна, congestion control, очереди и мультиплексирование

Приближение bandwidth-delay product: `bytes = bits_per_second × RTT / 8`.
100Mbps×80ms≈1MB in-flight (M04). Receive windows меньше BDP могут ограничивать
поток; слишком большие caps при множестве соединений расширяют memory exposure.
Socket buffer, receive credit, congestion window и очередь приложения — четыре
разные величины. 16MiB заполненной FIFO при 10Mbps требует≈13.42s дренирования;
это условный расчёт, не прогноз любой UDP queue. [D03] [D05] [D06]

Brutal требует реалистичной доступной rate, которая на LTE меняется и отличается
от скорости порта VPS. Завышение может увеличить потери, расход и задержку соседних
потоков. BBR/CUBIC/NewReno — не маскировка от DPI; поддерживаемый sender и его
реализацию определяет точная версия ядра. Проверять video+call+bulk, оба направления,
RTT/loss/jitter, metered bytes, CPU и соседний обычный TCP; один speedtest недостаточен.
Upstream Hysteria performance page предлагает большие buffers/priority/chrt —
это материалы для опыта, не готовый системный preset. Real-time priority может
ухудшить работу watchdog/DNS/SSH и других приложений. [D06] [D07]

Mux уменьшает handshakes, но собирает failure domain, flow-control credit и CC
нескольких потоков в одном transport. QUIC устраняет stream-level delivery HOL
между независимыми reliable streams, но сохраняет общие congestion/CPU/path ресурсы;
UDP payload в DATAGRAM не становится reliable stream. Вложенный TCP поверх reliable
TCP может усиливать задержки при потере; это не утверждение, что любой TCP tunnel
всегда хуже UDP на сети с UDP block. Разделять bulk/interactive pools, caps потоков
и idle eviction только после server compatibility и scoped измерения. [D03] [D04]

FQ-CoDel — готовый алгоритм для очереди в контролируемом bottleneck, не обещание,
что qdisc внутри гостевого VPS устранит очередь у гипервизора/оператора. ECN требует
поддержки и валидации на фактическом transport path. Не переносить DSCP/ECN metadata
в туннель без определённой семантики; L4S/FEC/custom scheduler — отдельный R&D gate,
а не новый обязательный transport первого релиза. [D08]


<!-- /source-unit: 13.4 -->

<!-- source-unit: 13.10 -->
<a id="source-13-10"></a>
### 13.10. Память, маленький VPS и операционная устойчивость

`GOMEMLIMIT` — soft limit памяти, управляемой Go; он не ограничивает весь RSS,
JNI/C allocations, mmap и kernel socket buffers. Слишком низкое значение увеличивает
GC pressure; unlimited pools и число sockets не исправляются одним GOGC. В Android
считать Kotlin+native+Go peak вместе, в Windows — GUI+worker+helpers, на VPS —
core+DNS+служебные процессы.9MiB исходных данных превращаются в 12MiB base64; три такие копии
уже 36MiB без parsed objects (M12), но фактические allocations пока не измерены. [D26]

Лимиты вводить до allocation/fanout: connections/streams/fd/goroutines/threads,
fragment maps, parser depth, image pixels, log/history bytes, distinct cache keys.
Reuse buffer требует clear ownership до последнего consumer; early release ради
latency может испортить ciphertext/профиль. Повторный hash/parse/serialize одного
immutable section можно сокращать только сохранив original-byte/private-proof/CAS
связь. No-backup и bounded size не доказывают power-loss durability или rollback safety.

На Linux VPS различать CPU quota/steal, memory/IO contention, UDP drops и remote
packet loss. PSI показывает время resource stalls; отсутствие PSI/cgroup доступа —
unknown, не «нагрузки нет». CPUWeight/MemoryHigh/MemoryMax/TasksMax/systemd версии —
инструменты будущего controlled resource profile, а не бездумный hard cap: watchdog,
DNS и recovery должны получать ресурсы. Latest systemd docs не доказывают наличие
каждой директивы в Ubuntu24.04. [D27] [D28]

Один личный VPS не требует нового distributed control plane. Больше transports
не защищает от неоплаты, disk-full, bad rollout или общего origin. Полезнее сначала
проверенные offline restore/immutable releases/credential rotation, bounded logs,
deadline оплаты с provenance и independent distribution cache. Maintenance/rekey
не начинается на всех gateways одновременно; старый сертификат не оставляется
после явного отзыва ради плавности. После rollback заново действуют source/trust/
path gates; запуск процесса не подтверждение готовности новой версии.


<!-- /source-unit: 13.10 -->

<!-- source-unit: 14 -->
<a id="source-14"></a>
<a id="transport-recovery-depth"></a>
## 14. Реальный путь пакета и восстановление: уточнение RQ-01/02

Исследование 2026-10-24 продолжает §13 и уточняет его открытые source questions.
Входы на 24 октября: Windows V2-140→141, Android AND-06-S4. Этот разбор
использовал исходники и не выполнял native/device-приёмку; поздние результаты
владельцев к нему не приписываются. Evidence — `docs/evidence/2026-10-24-transport-recovery.md`
содержит exact pins, снимки исходников и пределы доказательства.
PLAN §5.7 — `docs/PLAN.md#transport-recovery-research` и
Android §8.2 — `docs/ANDROID-PLAN.md#android-transport-recovery` задают будущую приёмку.


<!-- /source-unit: 14 -->

<!-- source-unit: 14.1 -->
<a id="source-14-1"></a>
### 14.1. Уточнение Salamander: дефект выражения и вызываемый метод

Встроенный HY2 outbound sing-box1.13.19 передаёт результат dial в
`bufio.NewUnbindPacketConn`, затем — в `NewSalamanderConn`. У Unbind wrapper
нет `SyscallConn`/`WriteVectorisedPacket`; packet factory проверяет интерфейсы
напрямую и не разворачивает `Upstream`. Поэтому здесь выбирается **обычный**
`SalamanderPacketConn.WriteTo`. Port hopping сохраняет эту цепь для child sockets.
Наличие underlying UDP socket само по себе не включает scatter/gather/GSO/ECN.

Проверены все 20 files закреплённого `common/tls`, включая build-tag alternatives.
Встроенные STD/ECH/KTLS configs не реализуют QUIC-specific `qtls.Config`.
uTLS/Reality `STDConfig` явно отвергают такое использование; это не скрытая
альтернативная QUIC-ветка. Этот вывод не означает, что любые TCP-only TLS options
автоматически работают в обычном QUIC STDConfig path.

У inbound raw UDP factory может выбрать vectorised Salamander, но QUIC basicConn
вызывает его `WriteTo`, который передаёт `[salt,payload]` **нижнему** writer.
Собственный `WriteVectorisedPacket` обёртки, арифметика которого проверена M10,
в этой цепи не вызывается. Следовательно, M10 остаётся корректным условным
контрпримером, а воспроизведённого дефекта штатного VPN из него не следует.
Внешний custom caller/изменённое ядро/будущий pin требуют новой проверки.

Оптимизация меняет также ownership: ordinary WriteTo копирует payload, vectorised
WriteTo изменяет его XOR на месте, нижний writer освобождает переданные buffers.
Нужен gate по lifetime/aliasing/error cleanup, а не только исправление индексов.
Scatter/gather одного datagram не тождествен GSO нескольких пакетов.


<!-- /source-unit: 14.1 -->

<!-- source-unit: 14.2 -->
<a id="source-14-2"></a>
### 14.2. Готовая QUIC-миграция есть ниже публичного HY2 API

В точном quic-go fork `ccd46a…` есть `Conn.AddPath(Transport)` → `Path.Probe`
→ `Path.Switch`. Комментарий «migration not supported» в обработке preferred_address
не отменяет существующий API. При этом HY2 Client не предоставляет owned Conn/
Transport/path API, а sing-box `InterfaceUpdated` закрывает client с network changed.
Из наличия метода в dependency не следует миграция текущего VPN. [T04]

`Path.Switch` записывает pending path; connection loop применяет его позднее.
Возврат Switch — ещё не receipt применения нового socket и полезной доставки.
В `switchToNewPath` сбрасываются PMTU/RTT/loss state. `MigratedPath` заменяет
установленный CC новым `CubicSender` с `useReno=true`: ранее выбранный HY2 Brutal/
BBR adapter не сохраняется. Новый migration seam обязан явно определить CC policy
после перехода; нельзя обещать прежний алгоритм или скорость по одному Switch.

В HY2 server задан `DisablePathManager=true`: при смене peer address используется
отдельный direct ChangeRemoteAddr path. Нельзя переносить вывод о client migration
CC reset на обычный server port hopping. Это различие веток исходников, не оценка
его security/runtime корректности.

RFC9000 различает NAT rebinding и активную миграцию; до handshake confirmation
активный перенос запрещён. Поддержка peer, connection IDs, path validation и reset
path state — отдельные условия. Валидация адреса не доказывает egress/сервис; малая
probe не всегда доказывает minimum MTU при anti-amplification ограничении. [T01]
Новый exit на другом VPS не получает state старого QUIC соединения по совпадению
сертификата. Migration не обещает multipath/bonding или сохранение любых TCP sessions.

Android `Network.bindSocket(fd)` требует unconnected socket. Повторный bind живого
соединения не является готовым способом handover. Нужны supported new-socket/path
API, protect+exact Network bind **до connect**, сохранив fd ownership и policy. [T05]
До такой приёмки нынешний controlled restart остаётся честным fallback внутри
защищённого режима; direct fallback не добавляется.


<!-- /source-unit: 14.2 -->

<!-- source-unit: 14.3 -->
<a id="source-14-3"></a>
<a id="143-pmtu-declared-capability-не-равно-наблюдаемому-свойству-сокета"></a>
### 14.3. PMTU: объявленная возможность не равна наблюдаемому свойству сокета

В `wrapConn` значение supportsDF начинается с true. Без доступного SyscallConn
оно остаётся true без вызова setDF в этой функции. На handshake confirmation
MTU discovery запускается по `!DisablePathMTUDiscovery && capabilities.DF`.
Это **не** доказательство отсутствия DF: underlying socket мог быть настроен ранее,
а ОС имеет свои defaults. Но и объявленный DF не доказывает реальную настройку.

У exact packet packer `PackPathProbePacket` добавляет padding до MinInitialPacketSize.
У Salamander добавляется ещё 8 bytes; этот wrapper budget учитывается отдельно.
Проверять фактические флаги сокета, wire lengths, outer IP family, PMTU decrease и
последствия смены пути. Не удалять obfs/protect/bind wrapper ради ускорения и не
выдавать PMTU timeout за доказательство DPI. RFC path/MTU semantics остаются
условием, которое проверяется на конкретной сборке и пути. [T01]


<!-- /source-unit: 14.3 -->

<!-- source-unit: 14.4 -->
<a id="source-14-4"></a>
### 14.4. UDP очередь ограничена по count, но запись может ждать

Pinned QUIC datagramQueue хранит до 32 send frames и 128 receive frames. При полном
send queue `Add` блокируется до dequeue либо закрытия общей очереди; контекста
отдельной записи у него нет. `SendDatagram` заранее копирует payload.
HY2 `udpPacketConn.SetWriteDeadline` возвращает `os.ErrInvalid`; read deadline
не ограничивает эту запись. Даже ограниченная очередь не ограничивает число внешних
заблокированных writers и их уже выделенные buffers.

Не оборачивать каждую просроченную запись новой goroutine и повторным SendDatagram.
Сначала trace владельца и фактического завершения, затем ограниченный допуск/байты/
freshness и поддерживаемая отмена. Нельзя закрыть общую QUIC connection для timeout
одного optional datagram, не определив влияние на другие TCP/UDP users.
Устаревший media datagram лучше отбросить по принятому контракту, чем доставить
после длинной очереди; универсальный срок для произвольного UDP не назначается.
DATAGRAM ACK подтверждает transport processing, не обработку приложения; receive
overflow может сбрасывать payload. Нужны отдельные drop/age/useful-completion facts. [T03]


<!-- /source-unit: 14.4 -->

<!-- source-unit: 14.5 -->
<a id="source-14-5"></a>
### 14.5. Таймер ядра, admission и полезное восстановление

HY2 keepalive10s/idle30s из §13 — входные настройки, не жёсткий watchdog SLO.
В exact connection учитываются peer idle timeout и PTO: keepalive interval зависит
от `max(min(configKeepAlive,effectiveIdle/2),1.5×PTO)`, idle deadline — от max(idle,3×PTO).
Keepalive scheduler подавляет повтор по `keepAlivePingSent`; обработка входящего
пакета сбрасывает флаг. Это не ACK receipt прежнего PING и не запрет PTO/path probes.
Планировщик/сон/CPU давление
добавляют задержку. PTO — работа transport recovery, не самостоятельный приговор
«канал заблокирован»; persistent congestion имеет собственные условия. [T02]

Android 3/60 ограничивает replacement admissions, Windows autoretry запускает full
start episode; это не общий cap для DNS races/dials/PTO/probes/channel iteration.
Нужны разные счётчики и один owner внешнего recovery. Смерть дочернего процесса не завершает зависший родительский
источник/Binder/IO. Новая работа должна учитывать фактическое завершение
и владение оставшимися ресурсами. Возврат одного метода также не доказывает
завершение порождённых работ. Конечный watchdog дочернего процесса
поэтому не задаёт конечное время восстановления всей службы.

В Windows рассмотренный netwatch пропускает polling при TUN и полагается на
сторожевой probe. Epoch fences отвергают **наблюдённую** смену, но сами не создают
событие о physical transition. Нужна freshness/coverage приёмка, включая no-event
TUN case; это source gap в доказательстве, не воспроизведённый false-connected bug.

Android strict-owner overlay добавляет fresh PackageManager lookups для новых
flows. Они и socket control делят NativeCallbackGate32; порог 5s проверяется после
return и не отменяет зависший Binder. При saturation нельзя ускоряться signer-cache/
UID shortcut без сохранения security contract. Проверять setup tails, old/new
flows, Stop/revoke, callback starvation и ICMP/PTB в реальном patched AAR.
В срезе 24 октября overlay ещё не был применён и собран.
Это не утверждение о текущем состоянии Android. Обычный AAR не подтверждает
изменённый путь; более поздний артефакт требует своего точного происхождения
и соответствующих device-результатов.


<!-- /source-unit: 14.5 -->

<!-- source-unit: 14.6 -->
<a id="source-14-6"></a>
### 14.6. Что переносить и в каком порядке

Переиспользуются exact native QUIC path manager/PMTU/PTO, HY2 factories, текущие
Network leases, retry owners, bounded lanes и scoped observations. Новые transport,
resolver и собственная реализация QUIC не требуются для этих улучшений.

1. TR-01: закрепить method/capability/ownership matrix, включая обычный fallback.
2. TR-05/06: honest freshness и saturation/actual-return recovery contract.
3. TR-04: bounded writers/bytes/age и общий учёт outer-vs-native recovery.
4. TR-02/03: один отдельно выбранный migration/PMTU prototype только после
   API/owner/CC/server gates и разрешения исполнения; controlled restart сохраняется.

Это исторические приоритеты разбора и входы будущей реализации; новые исследования
из них автоматически не запускаются. Безотказность/превосходство,
прирост Mbps и непрерывный звонок не доказаны чтением исходников.


<!-- /source-unit: 14.6 -->

<!-- source-unit: 14.7 -->
<a id="source-14-7"></a>
### 14.7. Первичные документы

[T01]: https://www.rfc-editor.org/rfc/rfc9000.html
[T02]: https://www.rfc-editor.org/rfc/rfc9002.html
[T03]: https://www.rfc-editor.org/rfc/rfc9221.html
[T04]: https://quic-go.net/docs/quic/connection-migration/
[T05]: https://developer.android.com/reference/android/net/Network#bindSocket(java.io.FileDescriptor)

T01 §§8.2/9/9.4, T02 §§6.2/7.6, T03 §§5.2…5.4 — стандарты с ограниченной областью.
T04/T05 прочитаны 2026-10-24, mutable docs не заменяют exact fork/Android API gate.
Точные upstream files/commits/blob/SHA256 и local source lines — в evidence.



<!-- /source-unit: 14.7 -->

<!-- source-unit: 18 -->
<a id="source-18"></a>
<a id="reviewed-borrower-lifetime"></a>
## 18. Рецензированный вывод: cache eviction и время жизни заимствованного сообщения

2026-10-26 МСК. **R04-06 принят только как source-факт и условный контрпример**.
[Рецензия руководителя](../research/reviews/HB01-R04-06.md) содержит exact
inputs, возражения и будущую приёмку. Это не воспроизведённый runtime-сбой,
security severity, измеренная частота или доказанное исправление.


<!-- /source-unit: 18 -->

<!-- source-unit: 18.1 -->
<a id="source-18-1"></a>
### 18.1. Что проверено

В sing-quic `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` клиент обрабатывает
DATAGRAM отдельными handlers. Reassembly lookup заканчивается до item.access;
eviction callback этот mutex не берёт. В sing `7c349dacf402256d3a7029746073b05d2ead584a`
удаление expired oldest entry может выполнить callback под cache mutex.
[Client caller](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client_packet.go),
[reassembly/pool](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/packet.go),
[LRU](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/cache/lrucache.go).

Условный trace: borrower A ещё собирает p и индивидуально приостановлен дольше
age; другой handler создаёт q той же живой session и вызывает expiry p.
Callback сбрасывает incoming referenced messages p до окончания A. Их backing
buffer может быть unmanaged: no-op Buffer.Release не отменяет отдельный reset
logical message. Это не утверждение об освобождении памяти ОС или воспроизведённом
повреждении. [Buffer semantics](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/buf/buffer.go).

Нужна индивидуальная scheduling pause, не один глобальный stop-the-world.
Server dispatcher в прочитанной ветке serial, поэтому тот же trace на него
автоматически не переносится. Windows go.mod указывает эти source versions;
наличие соответствующего кода в работающем EXE/whole Android AAR — отдельный gate.


<!-- /source-unit: 18.1 -->

<!-- source-unit: 18.2 -->
<a id="source-18-2"></a>
### 18.2. Следствие для ограничений ресурсов

Возраст entry и отсутствие его в cache не доказывают окончание всех использований.
Reclaim/recycle допустим после fencing новых borrowers и actual completion старых,
либо при едином сериализованном owner. Pointer reachability для GC не заменяет
logical lease на переиспользуемый объект.

Лимит видимых entries не является общим memory bound: отдельно учитываются
active, visible, detached, ready и retiring backing storage, без двойного счёта
shared aliases. Admission должен происходить до новых allocations/workers;
таймаут ожидания не возвращает credit за ещё работающего владельца.

Один дополнительный lock в callback нельзя считать готовым исправлением:
cache.mu→item.access и будущий item.access→Delete/cache.mu создадут инверсию.
Варианты для адресного design review — один owner либо detach/borrower accounting
с bounded retired memory и явным lock order. Новый transport/ядро ради этого
не требуется; source adaptation сохраняет лицензионные обязательства upstream.


<!-- /source-unit: 18.2 -->

<!-- source-unit: 18.3 -->
<a id="source-18-3"></a>
### 18.3. Передача в разработку и предел знания

Предложение уточняет существующие RQ-03/TH-06/COM-12/AND-11, не закрывает их.
После соответствующего разрешения: finite cases pause+expiry, duplicate/close,
size/janitor/lock-order и раздельный client/server artifact; затем измерение
useful completion и ресурсов. Пока все такие executions **NOT_RUN**.
Вероятность проявления, severity, native поведение и выигрыш по скорости/RSS/
батарее остаются неизвестными. Авторский предшествующий сборник сохранён полностью.

<!-- /source-unit: 18.3 -->


<a id="reviewed-brutal-dispatch"></a>
## 20. Рецензированный вывод: полный callback contract важнее имени алгоритма

26 октября 2026. **R04-10 ACCEPTED только в source/method-set scope**.
[Рецензия](../research/reviews/2H-HB02-R04-10.md) содержит exact inputs,
guards и ограничения. R04-09/весь R04-004 и реальная производительность не приняты.

В reviewed declared closure sing-quic d83826c306d7… / quic-go ccd46abedd39…
[Brutal batch method](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/congestion/brutal.go)
имеет time.Time вместо требуемого [extended monotime interface](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/congestion/interface.go)
и не имеет двух остальных extended methods. Normal selection берёт base adapter,
который не участвует в batch feedback dispatch. Individual ACK/loss stubs не
обновляют ackRate; на этом пути остаётся начальное 1. Это manual source derivation,
не исполнение Go или измеренный wire-rate.

Применимость ограничена выбранной Brutal-веткой; HY2 auto/BBR и server/client
условия различаются. SRTT, QUIC recovery/retransmission и pacing этим не выключены.
Исправление требует согласовать весь contract и monotonic sample history;
одна замена типа не является готовым fix. Future compile assertion плюс реальное
engine→adapter→controller evidence должны предшествовать comparative выводам.

Общее правило: presence алгоритма и base-interface compatibility не доказывают
доставку событий, от которых зависит его обещанное поведение. Усиленная компенсация
увеличивает offered load; скорость, fairness, полезное completion и energy требуют
отдельной приёмки. Все execution/native/live
gates **NOT_RUN**. Новый transport/самописный controller не предлагается.

[D16]: https://research.google/pubs/the-tail-at-scale/
[D01]: https://www.rfc-editor.org/rfc/rfc8899.html
[D02]: https://www.rfc-editor.org/rfc/rfc9000.html
[D04]: https://www.rfc-editor.org/rfc/rfc9221.html
[D05]: https://quic-go.net/docs/quic/optimizations/
[D03]: https://www.rfc-editor.org/rfc/rfc9002.html
[D06]: https://v2.hysteria.network/docs/advanced/Performance/
[D07]: https://v2.hysteria.network/docs/developers/Protocol/
[D08]: https://datatracker.ietf.org/doc/html/rfc8290
[D26]: https://go.dev/doc/gc-guide
[D27]: https://docs.kernel.org/accounting/psi.html
[D28]: https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html

[S12]: https://www.rfc-editor.org/rfc/rfc9114.html

[S13]: https://github.com/HyNetworks/tcp-brutal

[S03]: https://www.usenix.org/conference/usenixsecurity24/presentation/xue-fingerprinting

[S06]: https://www.rfc-editor.org/rfc/rfc9001.html

[S23]: https://www.rfc-editor.org/rfc/rfc9000.html

[S14]: https://github.com/klzgrad/naiveproxy/blob/master/README.md

[S15]: https://github.com/klzgrad/naiveproxy/releases/tag/v147.0.7727.49-3

[S16]: https://github.com/XTLS/Xray-core/discussions/4113

[S11]: https://github.com/HyNetworks/hysteria/blob/master/PROTOCOL.md

[S17]: https://forum.torproject.org/t/tor-project-user-support-report-for-january-2026/21221

[S18]: https://www.usenix.org/conference/usenixsecurity25/presentation/wails

[S20]: https://theupdateframework.github.io/specification/latest/

[S21]: https://github.com/Shopify/toxiproxy

[S22]: https://software.es.net/iperf/

## Применение в двух планах

Для [ПК](../../planning/knowledge-application/PC-PLAN.md) главный потребитель —
PC-06, затем измерительная частьPC-07; для
[Android](../../planning/knowledge-application/ANDROID-PLAN.md) — ANDR-07/08.
Миграция пути дополнительно зависит от защищённого жизненного цикла
и совместимости обоих концов. Платформа владеет исполнением, общая policy
не получает native-ресурсы.

Сначала переиспользуются возможности выбранного ядра и существующие владельцы
буферов, очередей и таймеров. Изменение окна или offload оправдано найденным
узким местом и проверенным API; цена включает совместимость, лицензию точной
цепочки и эксплуатацию. Простой личный VPS не требует нового распределённого
управления. Его текущие оплата, нагрузка, версия и порты ведутся отдельно
владельцем сервера, а не в теории.

Все количественные улучшения, проверки native-сокета, ресурсы устройства
и полезный сетевой путь здесь **NOT_RUN**. Формулы и условные traces остаются
моделями при явно указанных предпосылках.
