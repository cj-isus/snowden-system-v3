# Сетевой путь: применение теории блокировок, транспортов и диагностики

Дата среза: 28 октября 2026. Область — существующие Windows и заявленные Android
пути; ниже предложения, а не изменения продукта. Наличие ветви исходника,
собранный артефакт, работа на устройстве и защищённый выход в реальной сети —
разные уровни доказательства. Все будущие исполняемые критерии ниже — **NOT_RUN**.

<a id="n00"></a>
## N00. Основание и предел применения

Приоритет: сначала допустимость защищённого пути и достоверность его наблюдений,
затем управляемый bootstrap и переключения, затем скорость и один новый кандидат.
Количество транспортов, стран или маскирующих флагов само по себе не улучшает
доступность. Цель «лучше существующих» требует заранее выбранных сравнительных
сценариев; исходники не дают численного превосходства.

Основания: [модель противника](../../../theory/foundations/THREAT-MODEL.md),
[механизмы блокировок](../../../theory/transports/CENSORSHIP-MECHANISMS.md),
[композиция и TLS](../../../theory/transports/COMPOSITION-AND-TLS-CONTRACTS.md),
[DNS и диагностика](../../../theory/observability/DNS-AND-DIAGNOSTICS.md),
[границы доказательства проб](../../../theory/observability/PROBE-PROOF-BOUNDARIES.md),
[proxy/NAT64](../../../theory/observability/PROXY-DNS-NAT64.md),
[ECH/bootstrap](../../../theory/transports/ECH-DNS-BOOTSTRAP.md).
Архивные универсальные формулы цензора и старые предложения сверяются с этим
контекстом и текущими callers; они не являются действующими требованиями.

Текущая Windows опора: `render`, `metadata`, `core.PreflightResolver`,
`core.ProtectedProbe`, App ownership/lifecycle, типизированные observations и
`dependency_graph`. Android: `NativeGraphSecurity`, `PhysicalPathContext`,
существующий VPN owner и единственный whole-libbox bridge. Pure policy остаётся
в `shared/adaptive`; она не получает права измерять, изменять OS или разрешать
ослабление защиты. См. TH-01…08, OFF-01…09, AND-04…07/11/18, ADP-002…008.

<a id="n01"></a>
## N01. Наблюдаемая ошибка и её причина

**Уже реализовано в исходнике:** `windows/failclass.go:217` вызывает
`classifyObservedFailure`, а `failure_observation.go` различает timeout,
TLS verification/handshake, incomplete response, endpoint failure и unavailable.
Порты и старые regex не являются текущим доказательством DPI. Повторять старое
предложение «заменить port/regex классификатор» целиком не нужно.

**Применить:** сохранить типизированные стадии, время, полноту ответа и unknown;
исправлять семантику производителей фактов, прежде всего N07. Timeout не
устанавливает UDP block, throttling, active probing или намерение оператора.
Нельзя превращать ручной confidence в калиброванную вероятность причины.
Отдельный отказ необязательного endpoint не должен отравлять весь канал.

Источники: R02-001 §2–7; CENSORSHIP-MECHANISMS §2; TH-02/OFF-05,
AND-07/A-FR-06, ADP-004. Принятые scope/epoch/freshness правила сохраняются.

<a id="n02"></a>
## N02. Сначала конфиденциальность композиции

**Изменение P0, Windows owner.** `windows/backend/render/render.go:692–731`
создаёт ShadowTLS3 к donor и отдельный VLESS outbound без TLS, направленный через
него к remote loopback. `metadata/channel.go:181–203` проверяет UUID/password
роли и порты, но это не обеспечивает шифрование пользовательских данных.
`windows/backend/config/config.go:409–440` прямо освобождает detoured VLESS от
собственного verified TLS, приписывая шифрование ShadowTLS; это второй входной
gate, который должен быть исправлен вместе с metadata/render/selector.
После смены ShadowTLS на relay его framing/HMAC не заменяет payload encryption.
Комментарий «decrypted tunnel» не является криптографическим контрактом.

Самый дешёвый достаточный первый шаг — **отказать этой композиции в допуске** к
профилю, обещающему VPN-конфиденциальность, с конкретной причиной. Не включать
новый шифр или другой протокол автоматически. Разработку внутреннего TLS либо
поддерживаемого шифрующего inner-протокола выделить в согласованную миграцию:
клиент, профиль, реальные server inbound/detour и identity должны совпадать.
Аутентификация donor не аутентифицирует собственный proxy endpoint X.

Контраргумент: внутренний HTTPS приложения сохраняет собственное шифрование,
однако VPN должен защищать также не-HTTPS трафик и proxy metadata. Это scoped
source finding, не доказанная перехватом утечка установленного клиента.
Android `NativeGraphSecurity` ShadowTLS сейчас не принимает: переносить дефект
на него или ослаблять его allowlist ради паритета не следует.

Источники: COMPOSITION-AND-TLS-CONTRACTS, R02-10; TH-01/03, OFF-04,
AND-06/A-FR-05. Shared-PSK/salt сами по себе не доказывают PFS; `none` не кандидат.

У SS2022 client2 и server используют разные библиотеки/границы key parsing;
при выборе replacement нужен exact-length method/key contract. Replay cache одного
Service не становится durable/cluster-wide: выбрать ключи/область replay явно,
не заводить общую БД по умолчанию. Свежесть TLS-сессии не гарантирует единственный
прикладной эффект после retry. VPN не должен разбирать payload ради обещания
application exactly-once. R02-005/009 — условия альтернатив, а не выбранная замена.

<a id="n03"></a>
## N03. TLS capability зависит от полного пути

| Режим | Что уже известно из сохранённых source pins | Решение |
|---|---|---|
| Windows WS/TLS | Chrome preset + override ALPN `http/1.1`, WS early-data header; HTTP headers видит TLS endpoint/CDN | Описать фактический wire contract; не рекламировать h2 при HTTP/1.1 Upgrade |
| REALITY/no-flow | Ядро содержит Vision; Windows policy его запрещает, renderer не включает | Исправить основание запрета «ядро не поддерживает»; сохранить запрет до совместимости обоих концов |
| TLS minimum | uTLS preset может переписать SupportedVersions; один config.MinVersion не всегда gate | Проверять итоговый backend/preset/wrapper, не название настройки |
| Verify callbacks | STD disable-SNI и ShadowTLS shim имеют особые callback ветви | Разобрать только достижимые комбинации; не объявлять все проверки TLS выключенными |
| SPKI | Key pin не тождественен PKI hostname/time policy | Сохранить строгий Android reject неподдерживаемых режимов |
| WS first write | Lazy Upgrade может включать VLESS header и пользовательский prefix | Допуск до первой записи должен охватывать весь data unit; это не TLS 0-RTT |
| ECH | Renderer Windows реальный ECH не включает | Условная совместимость N04, а не текущий runtime дефект |

Основной pin: sing-box `v1.13.19/b5ebaa1fc0f2b94256180b95468e73ef53caa27d`;
uTLS `v1.8.4/cf49b0864331e156689feec6363ef9bf518a5ac7`;
sing-vmess `3aed155119a174c9290a4099841049f0cb275e02`.
Это source pins, а не аттестация установленного EXE/APK.

Для текущих traced TCP и HY2 constructors отсутствие session cache — сильный
отрицательный guard выводу «DialEarly/WS early data означает включённый TLS
0-RTT». Новая cache/resumption capability требует отдельной identity/epoch/replay
политики; допустимо сохранить запрет early data вместо нового replay-store.
Не включать session cache всем режимам ради предполагаемого одного RTT.

Exporter существует у native TLS/uTLS, но теряется при создании стандартного
ConnectionState literal в core wrapper; действующего product consumer не найдено.
Поэтому не добавлять binding без consumer. Если нужен готовый auth profile —
typed supported/unsupported adapter с реальным session lifecycle, без обращения
к неподдержанному exporter и без подмены secret публичным SNI/certificate hash.
Donor relay не владеет secret TLS endpoint и не получает exporter настройкой.

Не писать свой TLS/Vision. Сначала фиксировать backend, build tags, preset,
overrides, verifier roles и caller; потом один compatibility spike в TH-05.
Оппонент: библиотечный код может присутствовать, но отсутствовать в конкретной
сборке или ломаться на wrapper/server. Поле в Android graph не доказывает native
поддержку. Источники: R02-001 §5–7, R02-013/014 и C02 contracts §M01–M15.

<a id="n04"></a>
## N04. Один управляемый bootstrap вместо ещё одного resolver

**Изменение P1 после P0, Windows owner.** `corebridge.go:106–208` использует
`PreflightResolver` для включённого VLESS/WS; IP override сохраняет SNI/WS Host.
Это scoped bootstrap server-name, а не разрешение direct для пользовательских
запросов. Общий preflight deadline уже есть; его следует сохранить.

Конкретный долг `windows/backend/core/preflight.go`: host-only cache, retention
7 дней, unbounded `os.ReadFile`, upper-age-only проверка `At`, первый DNS ответ
попадает в cache до TLS, смешаны plain UDP и JSON DoH, HTTP client наследует
неявную proxy/redirect policy. Caller теряет cache provenance. Кэш не является
«последним аутентифицированным хорошим адресом».

Расширить существующий resolver, не создавать третий DNS сервис:

1. Ограничить чтение/число entries/размер ответа до разбора; валидировать типы,
   address role и будущую/недостоверную дату. Не превращать отсутствие времени
   в свежесть или permission на бесконечное использование.
2. Namespace: профиль/ревизия, purpose, физический network context и resolver
   policy. Смена сети не наследует Pref64, адресные hints или успех другой сети.
3. Вернуть небольшой immutable результат: address set + source/freshness/cache
   status + context. DNS provenance не расширяет доверие профилю и reference name.
4. Явно определить HTTPS proxy/redirect/privacy policy; bounded existing owner и
   бюджет. Любой first-success ответ не равен DNSSEC/authenticated origin binding.
5. Выбирать codec для реальных RR из уже используемых DNS зависимостей. Фиксированный
   `example.com A` codec native probe не превращать в общий SVCB resolver.

Нужны TH-03/RQ-07/08/09/12/15, AND-04…07; Android доверительный driver пока
отсутствует, Windows код туда не переносится целиком.

### ECH — отдельный совместимый режим

Сохранённые RFC 9848/9849/9460 различают real ECH, GREASE, SVCB optional/reliant,
ошибку DNS и аутентифицированный retry. Для SVCB-режима нужны связанный tuple
origin/scheme/port → RR provenance → target/ALPN/address/ECH config. Явно заданный
fixed endpoint + ECH config — другой допустимый режим, не полный SVCB selector.

Перед включением нужны одновременно: проверка origin O и public name P по своим
ветвям, поддерживаемый backend, authenticated retry на новом соединении,
проверяемая политика раскрытия имени и актуальный контекст. Для профиля с
обязательной ECH privacy безопасное серверное отключение ECH всё равно может
быть запрещено application policy. RFC fallback не отменяет VPN barrier.

В pin найдены условные границы: ECH wrapper cache/retry lifetime; различие
`utls.ECHRejectionError` и `crypto/tls.ECHRejectionError`; проверяемое имя в
rejected outer; потеря ECHAccepted в ConnectionState projection. Исправление
только error type недостаточно. O=P, multi-SAN, корректный custom verifier и
невызываемый retry handler — важные оппоненты общему обвинению. Проекция bool
не доказывает отказ; достоверный backend contract может подтвердить acceptance
без придуманного обязательного второго флага.

**Отложить real ECH integration**, пока нужен именно этот режим и закрыты
совместимость/доверие/lifecycle. Никаких собственных HPKE или auto plaintext fallback.

<a id="n05"></a>
## N05. Anti-probing: допуск, framing и ресурсный предел

Проверять отрицательные случаи по знанию и стадии: неверный внешний REALITY
admission отличается от правильного REALITY + неверного UUID; неизвестный HY2
obfs PSK — от правильного PSK + неверного HTTP auth. HTTP masquerade внутри
Salamander не превращает внешний UDP в обычный H3 cover. 400/404/timeout не
доказывают и не опровергают VPN без контролей.

У ShadowTLS нужны framing/HRR/replay/resource границы обеих сторон. Источник
с отличающейся ветвью ещё не доказывает practically distinguishable oracle.
Клиентский deadline не ограничивает автоматически donor/backend server sockets;
лимит одного record не ограничивает число partial readers, суммарные bytes и
время. Не добавлять unbounded reader/goroutine ради маскировки.

Конкретные future gates для того же pin:

- R02-006: scalar/buffer/vectorised writers имеют разные HMAC recurrence и record
  size/error semantics. Исправлять единый codec после caller review; наличие
  SS vector API не доказывает вызов плохой внешней vector ветви. Первый локальный
  IO error не должен теряться за последним успешным chunk.
- R02-007/принятый R02-15: HRR Random фиксирован; повторение первого MAC-предиката
  при прежнем PSK/transcript принято только как conditional source/model. Это не
  прохождение relay/inner/target. Готовый HRR-reject flag не установлен; выбор
  донора не гарантирует отсутствия HRR. Reject affected capability либо both-end
  state migration требуют отдельного проектирования, не одной настройки minimum.
- R02-008: parser получает один TLS record, поэтому корректная fragmentation
  ClientHello между records отличается от TCP segmentation и может оборвать
  donor fallback раньше auth. Нужны bounded reassembly и сохранение prefix через
  зрелый parser, не самописный TLS. Ordinary donor control обязателен.

Первый этап N02 позволяет исключить небезопасную композицию до дорогой работы
над этими ветвями. Нельзя объявлять три библиотечных вопроса текущими native
Android дефектами: его graph ShadowTLS не допускает.

Применение — расширить существующий E-04 совместимыми finite cases после P0,
не создавать scanner. Только свои endpoints, labels без credentials/raw first
write; controls включают malformed, loss, неверный config и обычные benign
сессии. Источники: R02-001/009/010/013/014, C02 contracts; TH-05/OFF-04, AND-18.

<a id="n06"></a>
## N06. Независимость резервов — provenance, а не бонус за страну

**Сохранить уже реализованное:** `windows/backend/render/dependency_graph.go` и
`fdmodel.go` не подставляют Cloudflare для любого WS, не угадывают ASN; различают
Shared/Different/Unknown/Coverage. Unknown не приносит независимость, RankHint не
является вероятностью совместного отказа. Старую additive penalty модель
повторно не внедрять.

**Следующий delta:** провести действительные зависимости от accepted профиля и
bootstrap/physical snapshot в этот граф. Обязательность и фаза различаются:
donor нового handshake, CDN edge, origin, exit, resolver, profile publisher,
provider account. Разные порты/transport на одном VPS не покрывают его неоплату;
другой carrier может покрывать блок direct entry, сохраняя общий origin outage.
Другая страна не доказывает другой ASN, account или publisher.

Покупка инфраструктуры не предшествует исправлению N02/N07. При лимите одного
VPS сохраняется ценность TCP/UDP альтернатив, но не серверная отказоустойчивость.
TH-03/07, OFF-09, ADP-002/004; методика R02-001 §4 и THREAT-MODEL.

У Windows `netwatch.go:211` App polling пропускает TUN mode. Это не доказательство
отсутствия всех native core callbacks, но нельзя считать этот App polling готовым
physical handover producer для TUN. Сначала определить actual native→App epoch
bridge и дополнить существующего owner; Android snapshot не копируется в Windows.

<a id="n07"></a>
## N07. Проба: удалённый ответ не равен отсутствию локальной утечки

**Изменение P0/P1 семантики, Windows owner.** `core/probe.go:332–364`
`directLeakResult` при успешном IP echo сравнивает public IP, выдаёт `leak/pass`,
а `Run:284–290` переносит `leak` в `LeakConfirmed`. `probe_observation.go:263`
передаёт один `ExpectedEgress` и TUN mode, а не независимое полное множество
direct egress с локальной route/flow provenance.

Одинаковый IP может происходить из общего NAT или другого relay, разный — из
multi-egress/разных потоков. Успех одного HTTPS потока не доказывает native DNS,
IPv6, UDP, per-app capture или отсутствие direct остальных потоков. Контраргумент:
при действительно заданной топологии и одном ожидаемом выходе сравнение полезно
для проверки пути; уже имеются HTTPS PKI, unavailable/stale/cancel guards.
Нужна точная семантика, а не удаление консервативной защиты.

Минимальное изменение:

1. Отдельный результат **egress comparison: match/mismatch/unknown** с scope.
2. `confirmed leak` только при достаточном проверяемом свидетельстве нарушения
   локальной границы/явно выполненных предпосылках, не из equality alone.
3. Неподтверждённый защищённый путь по-прежнему блокирует promotion; unknown не
   превращается в healthy. Подтверждённая утечка сохраняет немедленный hard stop.
4. Classifier, view и UI используют эти различия без обещания «утечек нет» по
   одному echo. Producer/consumer менять вместе, FR-003a/TH-02/OFF-05.
   `normalizeProbeReport` тоже превращает любой step Status=`leak` в
   `LeakConfirmed`: исправление одного агрегатного bool оставит прежнюю ошибку.

Не дублировать существующие guards: App capture/validate scope before+after,
operation owner, epoch/generation/freshness, protected/optional budget lanes,
лимит активных проб, reservation before work без очереди. Manual native DNS
`RunNativeProbe` уже имеет fixed bounded action, четыре UDP/TCP v4/v6 slots и
частичные outcomes. Это source progress, не аттестация WFP/TUN. Не включать
необязательные native измерения в mandatory health/failover автоматически.

Источники: PROBE-PROOF-BOUNDARIES, DNS-AND-DIAGNOSTICS; AND-07/A-FR-06,
ADP-003/004. Cache hit/DoH HTTP 200 не подтверждает свежую authoritative resolution;
проверка arbitrary websites не становится endpoint quorum.

<a id="n08"></a>
## N08. Proxy, IPv6-only и NAT64 привязаны к настоящему socket hop

Сначала обозначить граф N→P, P→E, inner TLS I и DNS R. Named CONNECT/SOCKS может
оставлять resolution E на proxy; явный DomainResolver над detour меняет это на
numeric endpoint и другую ACL/privacy зависимость. `protect/bind` применяется к
реальному leaf socket, NAT64 — к tuple этого socket, а не к SNI/WS Host/CONNECT
authority. SOCKS UDP ASSOCIATE создаёт дополнительный relay socket; обычный
CONNECT не доказывает CONNECT-UDP/HY2.

Android `PhysicalPathContext` уже учитывает DNS order, addresses/routes,
private DNS, proxy/PAC и Pref64 с API границами. Переиспользовать его контекст,
не вводить второй монитор. Android native protect/bind owner и оригинальный FD
остаются отдельным acceptance gate. Metadata `VALIDATED` — hint, не доказательство
доступности VPN. Pref64 нельзя переносить между сетями; `ipv4only.arpa` не
является произвольным reachability endpoint.

Library HTTP connector Basic/custom headers не означает готовый corporate UX:
NTLM/Negotiate, PAC и TLS interception имеют другие доверительные требования.
Нынешние Windows descriptors принимают VLESS/HY2, а не произвольный proxy graph.
Сначала capability matrix с typed unsupported; advanced corporate auth отложить.
TH-03/OFF-04/05, AND-05/06/07, RQ-12/15. Никакой глобальной DNS64 подмены.

<a id="n09"></a>
## N09. Скорость и ресурсы: сначала достижимый путь

Salamander обычного WriteTo добавляет 8 bytes и меняет wire bytes, но не делает
timing/length shaping. Его wrappers могут менять fast path. Наличие подозрительной
vectorized функции не доказывает её вызов: HY2 outbound в прочитанном пути
использует обычный WriteTo; inbound имеет другой dispatch. Обновление зависимости
по недостижимому фрагменту без caller анализа не обосновано.

PMTU считать по outer IP/UDP/obfs/QUIC headers; такой расчёт не разрешает снижать
inner IPv6 MTU ниже требований IPv6. Kernel TCP BBR не управляет userspace QUIC;
Brutal применяется при конкретном согласовании, не по названию протокола.
Queue slot cap не ограничивает суммарные bytes, blocked writers или время
завершения: transport PTO, application retry и owner retirement разные бюджеты.

После TH-01/02 — один фактор и workload: p50/p95 полезной latency, goodput,
resource/energy и recovery, контроль фактического return/ownership. Не вводить
глобальный mux/TFO/keepalive/padding/Brutal по теоретической «оптимальности».
Смежные lifetime/reassembly claims передавать владельцу скорости как scoped
source constraints, не как доказанные native crashes. TH-06/E-03/05/08,
AND-11/A-FR-10, RQ-05/06/16. Архивные численные настройки — hypotheses.

**Заимствование shaping пока defer:** sing-mux padding ограничен первыми 16 records
carrier wrapper, Naive Variant1 — первыми 8 frames отдельного stream; это не IP
packets и не одинаковая область reset. Cover/preamble может отказать при живом
защищённом CONNECT. У Maybenot fractions=0 не означают disabled, allowance и
replacement не hard caps. RegulaTor watchdog не oldest-packet deadline; FRONT
sampled target не hard resource quota; Tamaraw stop/events/units требуют adapter
trace. Подписанный machine/version не доказывает численную пригодность.

Готовый scheduler можно адаптировать лишь с peer-recognized framing, finite
actions/timers/queues, actual-send byte accounting, queue age/deadline и quota,
не обнуляемой каждым reconnect. Конечная квота несовместима с постоянным
положительным idle cover произвольной длительности. Maybenot MIT/Apache-2.0 —
кандидат primitives; DAITA/GotaTun — reference другого engine, не замена libbox.
Сравнивать joint bootstrap/handshake/steady trace, а не перемножать marginal
оценки или число seeds. Всё это ограничения модели, не найденный RF classifier.

<a id="n10"></a>
## N10. Готовые решения и стоимость совместимости

| Решение | Что переиспользовать | Условие и цена | Решение сейчас |
|---|---|---|---|
| Existing sing-box/sing/uTLS/Vision | Текущие зависимости, contracts и validators | GPL/licence notices, build tags, wrapper/server matrix, Android whole AAR | Reuse; отдельный Vision spike только после P0 |
| Existing DNS libraries/codecs | Bounded wire DNS и production RR parsing | Fixed probe codec не общий resolver; schema/freshness/role checks | Adapt N04 |
| Tailscale netmon dependency | Существующий pin и native integration seams | Наличие зависимости не доказывает подключённый OS callback | Reuse seam, не второй monitor |
| go-tuf | Уже выбранная модель доверия/ADR, portable driver у владельца trust | Не переписывать canonicalization в Kotlin и не импортировать Windows module | Согласовать контракт, не второй trust stack |
| Naive | Один compatibility spike в TH-05 | Двухсторонний wire, artifact/advisory/licence/SBOM, packaging и rollback | Already planned; бинарь не выбран/не принят этим документом |
| XHTTP | Возможный отдельный engine/sidecar/fork | В pin 1.13.19 нет upstream XHTTP; новый lifecycle/supply chain | Defer до доказанной нужды |
| Glorytun, FaaS/Workers, QUIC migration | Ограниченные архитектурные идеи из сохранённых источников | POSIX/third-party carrier/cost или иной protected-egress контракт | Не добавлять сейчас; build+sshx не постоянный VPS |

<a id="n11"></a>
## N11. Минимальные контракты и последовательность

Не нужен новый универсальный SDK или хранилище. Дополнения укладываются в
существующие immutable snapshot/descriptor/observation структуры:

- **Capability/admission:** profile revision + feature/backend/version + identity
  roles + confidentiality/privacy floor + typed supported/unsupported/reason.
- **Bootstrap:** purpose, trusted profile reference, physical epoch, resolver
  policy, bounded addresses, provenance/freshness; IP hint не новая authority.
- **Observation:** owner/generation/epoch + attempted/completed + stage/outcome,
  budget/deadline и scope; отсутствие данных не ноль и не pass.
- **User-data permit:** существующий platform barrier проверяется перед реальной
  первой записью и после ожиданий; candidate/probe success не заменяет этот допуск.

Не делать обязательными все поля для всех способов транспорта. ECH tuple нужен
только режиму с этой capability. Никаких raw UUID/password/headers/пользовательских
destinations в долговременном экспорте. Android SessionJournal остаётся bounded
typed event surface, а не forensic log или новый telemetry SDK.

| Этап | Конкретный результат и owner | Зависимости / независимость | Будущая приёмка |
|---|---|---|---|
| A1, P0 | Windows N02: reject неподдерживаемой конфиденциальной композиции | Accepted profile/schema; может делаться отдельно от ECH/новых транспортов | Invalid composition не доходит до data dial; старый клиент отвергает критическую неизвестную capability |
| A2, P0/P1 | Windows N07: split IP comparison/confirmed leak + consumers | Существующие TH-01 guards сохраняются; независимо от A1 | Common NAT/multi-egress/stale/partial cases не получают ложный no-leak; unconfirmed путь не promoted |
| A3, P0 | Platform owners закрывают текущие custody/trust/lifecycle gates TH-01/AND-04/11 | Ни один новый transport не обходит эти gates | Реальный owner return, barrier/epoch и fail-closed по платформенному плану |
| B, P1 | Windows N04: bounded context-bound bootstrap | Стабильные profile/physical-context interfaces; не требует нового VPS | Future/oversize/corrupt/cross-network cache и proxy/redirect cases; reference identity неизменна |
| C, P1 | Platform/adaptive: настоящие producers → existing dependency/typed policy | ADP-006 и platform ownership; не policy-owned sockets | Unknown остаётся unknown; scope/freshness rechecked после wait; отсутствует ложная независимость |
| D, P2 | Native protocol/OS acceptance по E-01…08 и Android matrix | Отдельное разрешение исполнения; собственные controlled endpoints | Tags/artifact/server tuple, first-write/admission/replay/DNS/IPv6/UDP/resource controls |
| E, P2 | Один speed фактор либо один TH-05 кандидат | Только после пригодного baseline; server и client вместе | Парные контроли, useful outcomes, цена ресурсов; нет default-on по одному успеху |
| F, field | TH-07/AND-16: несколько реальных сетей и canary | Оплаченный доступный VPS, разрешённый live и независимые receivers | Сопоставимые operator/time/workload/provenance, held-out случаи; rollback и unknown сохранены |

Оценки A1/A2 — небольшой source slice с согласованными consumers; B/C — средняя
интеграция; D/F — дорогие native/device/field gates. Это относительная сложность,
не обещание сроков, стоимости сервера, скорости или экономии токенов.

<a id="n12"></a>
## N12. Приёмка, которая действительно меняет вывод

1. **Source:** достижимый caller, pins, server mapping, owner/permit и контракт
   ошибок; source review не заменяется grep отсутствия имени.
2. **Local protocol:** positive и отрицательные credential/stage cases, forced
   partial I/O/HRR, cache clock/size/context, payload boundary; конечные бюджеты.
3. **Native/device:** реальный EXE/whole adapted AAR, OS capture/protect/bind,
   suspend/resume/network change, IPv6/DNS/UDP, resource return. Сборка не VPN PASS.
4. **Field:** совпадающие artifact/config/operator/date/workload и controls;
   OONI measurement/schema и ошибки относятся к конкретной версии/сетям.
   Исторические наблюдения РФ/GFW не становятся нынешней картой всех операторов.
5. **Rollout:** заранее заданные hard stops, сравнимый baseline, bounded canary,
   graceful recovery без direct fallback защищённого трафика.

Для внешних measurements переиспользовать понятия схемы, не встраивать OONI
engine/harvester. Сохранённые R02-014/R28-Z02 показывают:

- `blocking=null|false|string`, missing, `accessible` и API `confirmed/failure`
  разных версий/слоёв не сворачиваются в один bool. Backend revision отсутствует
  — parity unknown, одинаковый test_version недостаточен.
- Local/control TCP можно сопоставить по IP:port; control HTTP без remote IP/
  transaction/time не доказывает тот же endpoint. `dns_consistency` может означать
  общий ASN при разных адресных наборах. TLS probe и HTTPGet — разные операции.
- Code200 + пустой saved body + timeout не означает получение 0 bytes: saver
  может отбросить partial body. Успешный snapshot и ошибка дочитывания хвоста —
  разные outcomes; scalar accessible не доказывает full-body/useful completion.
- `Date/Age` HTTP cache не дата измерения; publication/upload не measurement
  path. Wi-Fi не доказывает fixed upstream, Android не доказывает mobile.
- Один scoped positive доказывает лишь такой positive; universal negative требует
  покрытия заданного множества. При известном равновзвешенном конечном D доля
  ограничена `[s/N,(s+u)/N]`, это identification interval, не confidence interval.
  Неизвестный denominator не заменяется количеством API rows или известных AS.

Published OONI data в сохранённом лицензионном срезе — CC BY-NC-SA 4.0; concepts
спецификации — более узкое заимствование, чем массовая коммерческая адаптация
dataset. Перед product ingestion нужен отдельный lawful-basis/licence review.
Не нужен telemetry SDK или новая база для трёх исследовательских records.

95/99,5%, «211 новых блокировок», «HY2 переживает подсетевой барьер» и «весь риск
platform camouflage только юридический» не используются как установленные
сетевые свойства. Source R02-014-A ограничивает числа; R02-012 — исторический
baseline с NEEDS_REVISION, не готовый выбор хостера/схемы обхода.

Не подтверждены установленный Windows/VPS tuple, работающий Android artifact,
native route/no-leak, параметры текущих блокировок РФ и превосходство над рынком.
Открытые gates остаются в существующих TH/OFF/AND/ADP пунктах; данный документ
их не закрывает. Недостающую семантику источника следует сохранить как needs
review, а не превратить в очередной транспорт или обязательную настройку.

### Адресные источники и существующие планы

Таблица ведёт к подробным аргументам и реестрам сохранённых первоисточников.
Повторное использование такого реестра не означает новую загрузку или полный
независимый аудит перечисленных там библиотек.

| Вопрос | Точное основание и область | Пункт разработки |
|---|---|---|
| Observer, Vision, WS/Salamander | [R02-001 §2–7](../../../theory/research/02-CENSORSHIP/REPORT.md), [пины/источники](../../../theory/research/02-CENSORSHIP/SOURCES.md); [принятая public-source поправка Vision/keygen](../../../theory/research/reviews/2026-10-27-R02-01-VISION.md) | TH-02/05, AND-01/18 |
| Joint trace и finite shaping | [R02-002 §4–7](../../../theory/research/02-CENSORSHIP/REPORT-002.md), [exact reuse pins](../../../theory/research/02-CENSORSHIP/SOURCES-002.md), [R02-003 constructors](../../../theory/research/02-CENSORSHIP/REPORT-003.md), [его источники](../../../theory/research/02-CENSORSHIP/SOURCES-003.md) | TH-05/06, E-04, AND-11/18 |
| Конфиденциальность композиции | [R02-004 §1–6](../../../theory/research/02-CENSORSHIP/REPORT-004.md), [scoped решение R02-10](../../../theory/research/reviews/2H-HB01-R02-10.md), [R02-005 условия замены](../../../theory/research/02-CENSORSHIP/REPORT-005.md), [R02-009 разные predicates](../../../theory/research/02-CENSORSHIP/REPORT-009.md) | TH-01/03, OFF-04 |
| Framing, HRR, first record | [R02-006](../../../theory/research/02-CENSORSHIP/REPORT-006.md), [R02-007](../../../theory/research/02-CENSORSHIP/REPORT-007.md), [R02-008](../../../theory/research/02-CENSORSHIP/REPORT-008.md), [принят только HRR predicate](../../../theory/research/reviews/4H-HB01-R02-15.md) | TH-05, OFF-04, E-04 |
| Exporter и отсутствие 0-RTT в traced constructors | [R02-010 §1–4](../../../theory/research/02-CENSORSHIP/REPORT-010.md), [R02-011 caller/guards](../../../theory/research/02-CENSORSHIP/REPORT-011.md) | TH-05/06, AND-01/18 |
| ECH, origin/public-name и first write | [R02-013 §2–5/12](../../../theory/research/02-CENSORSHIP/REPORT-013.md), [его primary/source locators](../../../theory/research/02-CENSORSHIP/SOURCES-013.md), [C02 M01–M15/B/C](../../../theory/research/cycles/2026-10-28-contracts/C02/REPORT.md), [C02 источники](../../../theory/research/cycles/2026-10-28-contracts/C02/SOURCES.md), [Z01 §3–8](../../../theory/research/cycles/2026-10-28-contracts/Z01/REPORT.md), [Z01 источники](../../../theory/research/cycles/2026-10-28-contracts/Z01/SOURCES.md) | TH-02/03/05, AND-04…07 |
| Числа/РФ/сырые измерения | [R02-014 §2–8](../../../theory/research/02-CENSORSHIP/REPORT-014.md), [его источники](../../../theory/research/02-CENSORSHIP/SOURCES-014.md), [принята только числовая область A](../../../theory/research/reviews/NINE-2026-10-27-R02-014-A.md), [Z02 joins/unknown](../../../theory/research/cycles/2026-10-28-contracts/Z02/REPORT.md), [Z02 версии/licences](../../../theory/research/cycles/2026-10-28-contracts/Z02/SOURCES.md) | TH-02/03/06/07, AND-07/16 |
| Дополнительные принятые source ограничения | [DR27-01/02/03/13/15](../../../theory/research/reviews/2026-10-28-DEEP-REVIEW.md): composition, effective TLS floor, projected exporter, proxy/hop и предел egress proof; остальные claims отдельно | TH-01…06, OFF-04/05 |

Нормативный маршрут реализации: [PLAN TH/OFF](../../../PLAN.md),
[Android AND/A-FR](../../../ANDROID-PLAN.md),
[adaptive ADP](../../../adaptive/PLAN.md). Последние сопоставленные обновления
V2-183 и AND-02-S5 не закрывают native/runtime gates и не меняют названные
сетевые выводы. Дальнейшие библиотечные и полевые вопросы не требуют нового
каталога исследований до применения этих ограниченных срезов.
