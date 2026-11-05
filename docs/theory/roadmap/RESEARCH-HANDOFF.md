<a id="исследовательские-вопросы-и-передача-в-разработку"></a>
# Решения для разработки и границы применения

[Оглавление](../README.md) · [Карта вопросов](../KNOWLEDGE-MAP.md) · [Применение знаний](../../planning/knowledge-application/README.md)

## Краткий вывод

Накопленная теория даёт основания для ограниченных решений разработки,
а не завершённый продукт. Сначала проверяются применимость к точным версиям,
защита и владение ресурсами; затем измеряется цена одного изменения.
Прежние исследовательские очереди заменены маршрутом к предметным главам
и двум действующим планам.

**Когда применять.** Чтобы понять, какое решение уже обосновано, какой вопрос
открыт из-за отсутствующих входов и где находится его основной разбор.

**Граница знания.** Таблицы 24–28 октября 2026 года отражают свои срезы и
области рецензирования. «Исследовано» означает завершение названного разбора,
а не закрытие всех поздних вопросов, native-гарантий или сравнительной скорости.
Команды и адресаты прежних исследований не служат действующими поручениями.

<!-- source-unit: 4 -->
<a id="source-4"></a>
## 4. Реестр ложных блокеров (анти-знание)

Это уроки из прежних инцидентов. Часть основана на сохранённых сообщениях
о проверках конкретной установки; они не повторялись этой редактурой.
Обобщение «каждый пункт опровергнут live» слишком широко: ограничения
ниже относятся только к названным входам.

| Ложный блокер | Откуда | Правда |
|---|---|---|
| «nginx возвращает 400 на Sec-WebSocket-Protocol: VLESS» | SERVER-2026-09-01-1 | nginx отдаёт 101; 400 отдаёт сам sing-box на невалидный base64 subprotocol. Литерал «VLESS» не используется реальным клиентом [доказано] |
| «нужен скрипт vps-diagnose-and-fix-nginx-ws.sh» | Тот же инцидент | Причина конкретного 400 была не в предлагаемом изменении nginx. Это не доказательство отсутствия любых серверных ошибок |
| «ограничение CF-token блокирует любую диагностику» | Прежняя запись phase-0 | Для названных тогда проверок API-токен не требовался. Это не универсальный отказ от API: чтение конфигурации зоны и проверка сетевого пути имеют разные контракты |
| «второй домен сам даёт независимый резерв» | Старое B1-предположение | Исправлено: домен не устраняет общие аккаунт, origin, IP или маршрут. Независимость определяется графом; второй домен не является обязательным блокером разработки |
| «WS 400 — ожидаемо, реальный клиент пройдёт» | PHASE-8A journal | Дважды неверно: и 400 не от nginx, и «пройдёт» оказалось ложью, пока не починили серверный route.final [доказано] |
| «сертификат snowdensys не соответствует → Full (Strict) недоступен» | RELEASE-2026-08-28-1 | Починено и не имеет отношения к клиентскому пути |

| «клиентский mux можно включить глобальным дефолтом, сервер не трогая» | V2-037 live | EOF на всех соединениях при живом CDN+TLS+auth: sing-box-сервер без multiplex-блока рвёт mux-стримы; чинится сервером (multiplex enabled+padding) или per-канальным дескриптором [доказано] |
| «probe EOF однозначно означает дефект клиента/сервера (V2-039)» | сессия 2026-09-12 | Mux on/off дали одинаковое зависание, сервер видел dial/open stream. Это не локализует неисправность только в клиент↔CF и не исключает серверный egress/CDN/цель. Старое «сервер ни при чём» и контрфактическое «curl прошёл бы» отозваны. Нужны независимые контрольные запросы и captures обоих концов. |

Мета-вывод: история содержит реальные серверные ошибки, но доля «почти все»
не посчитана. Проверка обоих концов ускоряет локализацию; clean logs и живой
/ip не исключают остальные причины. Режим исполнения определяется отдельной политикой; историческая таблица
не разрешает SSH, изменение сервера или локальной сети.

---


<!-- /source-unit: 4 -->

<!-- source-unit: 5 -->
<a id="source-5"></a>
## 5. Открытые вопросы и направления (исследовательские заметки)

Исходные двенадцать вопросов получили датированные решения в
[§15.2](#source-15-2). Их повторный список с промежуточными статусами удалён
из основного чтения: прежние «ещё исследовать» не должны конкурировать
с более поздним ответом. Остаток знания отличается от задачи реализации.

Три полезных уточнения прежнего списка сохраняются:

- В сентябрьской модели `windows/backend/render/fdmodel.go` оценка независимости
  была `1−Σштрафов` с порогом 0.35; неизвестные поля не штрафовались.
  Это эвристический порядок, не вероятность. Неизвестность не подтверждает
  независимость; пример «0.55 < 0.35» был арифметической ошибкой.
- Исторический классификатор использовал имена ResetInjected/SilentFreeze/
  TlsPolicing443/PortBlackholed/DnsSubverted/WhitelistMode/DirectLeakConfirmed.
  Эти имена не доказывают причины; малой пробы недостаточно для объёмной
  остановки. [Основной разбор адаптации](../adaptation/POLICY-AND-EVIDENCE.md)
  отделяет наблюдение, допустимое действие и неопределённость.
- В V2-041 сообщалось об изменении trace IP через дополнительный выход при
  challenge отдельного сайта. Это не доказывает единственную причину ASN
  или успешное прохождение браузером. Cookies, TLS/HTTP-поведение, нагрузка
  и политика сервиса остаются альтернативами. CDN-вход не меняет выход
  сам по себе; [границы сетевой проверки](../observability/PROBE-PROOF-BOUNDARIES.md).

<!-- /source-unit: 5 -->

<!-- source-unit: 13.11 -->
<a id="source-13-11"></a>
### 13.11. Порядок применения и проверенные модели

Порядок: устранить подтверждённый reachable safety/correctness defect → ограничить
work/memory/retries → измерить текущий path → выбрать одну обратимую оптимизацию →
paired replay/lab → device/native → owner-authorized field/canary. Не сочетать
новое ядро, mux, MTU, CC, padding и сервер в одном сравнении. Обоснованный отрицательный
результат сохраняется; отсутствие приемлемого gains останавливает расширение.

| Модель | Что действительно проверено | Чего из неё не следует |
|---|---|---|
| M01 | Common-node availability ceiling и ошибочный independent подсчёт | Доступность наших операторов/VPS |
| M02 | До 27 lower attempts при 3×3×3 против 3 при одном owner | Фактические retries/latency текущих приложений |
| M03 | Fixed-window boundary burst и отличие token bucket от rolling cap | Необходимость заменить уже имеющийся rolling budget |
| M04 | BDP≈1MB; drain условной full16MiB queue≈13.42s | Универсальный socket buffer или observed bufferbloat |
| M05 | Outer IPv6 UDP budget1232 при PMTU1280; с Salamander salt8 остаётся 1224 для QUIC; inner overhead отдельно | Рекомендация снизить inner IPv6 MTU ниже 1280 или доказанный PMTU defect |
| M06 | Exact zero-failure iid upper bounds | Независимость полевых samples или абсолютный no-leak |
| M07 | Average of p99 не pooled p99 | Один подходящий percentile для всех workloads |
| M08 | Switch cost может превышать условный выигрыш | Новая production threshold/calibrated probability |
| M09 | Hostname-only cache смешивает разные realms | Воспроизведённый дефект текущего DNS |
| M10 | Индексный counterexample в точном pinned source | Достижимость/remote exploit/native crash в нашем VPN |
| M11 | 15s schedule —5760 opportunities/day | Измеренные wakeups/энергия |
| M12 |9MiB raw→12MiB base64, multiplicative live copies | Измеренный RSS или право менять trust bounds |

Модели описывают арифметику и условные контрпримеры, а не измерения
продукта. Таблица не является командой запуска или свидетельством текущей
приёмки. Формулы и предпосылки находятся в предметных главах:
[граф отказов](../foundations/THREAT-MODEL.md#source-7-3),
[ресурсы](../performance/TRANSPORT-AND-RESOURCES.md),
[адаптация](../adaptation/POLICY-AND-EVIDENCE.md),
[метод измерения](../methodology/RESEARCH-METHOD.md).
Сохранённые подробные результаты остаются основанием именно своих моделей.


<!-- /source-unit: 13.11 -->

<!-- source-unit: 15 -->
<a id="source-15"></a>
<a id="research-complete"></a>
## 15. Итог исследования и передача в разработку — 2026-10-24


<!-- /source-unit: 15 -->

<!-- source-unit: 15.2 -->
<a id="source-15-2"></a>
### 15.2. Адресное закрытие всех 12 вопросов старого §5

Таблица фиксирует решения 24 октября по исходному списку.
C01…C14 — координаты выводов, не новая очередь разработки.
Столбец приёмки указывает, что требовалось доказать, а не текущий статус
каждого владельца. Меняющиеся API/тарифы и конфигурации относятся к тому срезу;
поздние поправки имеют приоритет в своей области.

| Старый вопрос | Окончательный вывод документального разбора / решение | Будущий gate в существующем плане |
|---|---|---|
| 1. REALITY/XHTTP, канал B | **Reuse REALITY; defer XHTTP.** В exact sing-box1.13.19 parser нет xhttp/splithttp; неизвестный transport отвергается. Нельзя включить его флагом в текущем renderer. C01 | TH-05/AND-18: reopen только при доказанном недостающем сценарии, compatible core/ADR и E-02/03/04/08; замена ядра сейчас не выбрана |
| 2. ECH со своим public_name | **Defer deployment.** ECH скрывает часть ClientHello в пределах anonymity set; отдельные IP/public_name/traffic/config delivery могут сохранять отличимость. Свой домен не универсальный обход. C02 | TH-05/E-04: owned endpoint, exact client/server, authenticated config/cert, reject/retry/rotation, отсутствие insecure fallback; paired ISP/time/family |
| 3. XHTTP вместо WS | **Сохранить WS baseline.** Тот же source ответ C01, а не второй transport engine или поиск случайного preset | TH-05/E-08: сравнение кандидата только после compatible seam и safety gate |
| 4. WS/REALITY anti-probing | **Adapt adversary/cover matrix.** 400/404 и auth не доказывают неотличимость; passive TLS-in-TLS и active probing — разные механизмы, результаты GFW не доказательство нынешней РФ. C03 | TH-05/E-04: свои серверы, wrong/absent/replayed auth, malformed/short/timing/close/concurrency, benign corpus и held-out days |
| 5. Национальный DNS/удаление домена | **Adapt authenticated bootstrap.** Подмена локального ответа, отказ resolver и authoritative/delegation loss различаются. Reachable trusted resolver помогает не во всех случаях; DNSSEC подтверждает данные/отсутствие, не доступность. Подписанный numeric endpoint убирает lookup, сохраняя identity/hostname/pin/expiry; NAT64 отдельно. C04 | TH-03/RQ-07/08/09: controlled NXDOMAIN/NODATA/SERVFAIL/timeout/poisoned AAAA/resolver outage; no circular DoH bootstrap, negative-cache scope; protected user DNS без direct fallback |
| 6. Калибровка classifier | **Adapt observable facts + unknown.** Таймаут/RST/x509/порт не устанавливают причину; литература не калибрует probability текущего ISP. C05 | TH-02/E-06: known faults + held-out ISP/day/family confusion matrix, abstain/coverage/false-switch против fixed-policy baseline |
| 7. Failure domains/веса/unknown | **Adapt dependency graph + provenance.** Два транспорта могут разделять origin/ASN/CDN/publisher/resolver. Unknown не independence, score не probability; implicit WS=CF убрать на реализации. C06 | TH-03/E-07/RQ-14/20: shared-node outage и unknown edges; joint outage estimates только из подходящих paired blocks |
| 8. Первый hop в РФ | **Defer paid infrastructure experiment.** Меняется dependency первого участка, добавляются trust/cost/failure domains. RU ASN не подтверждает allowlist. C07 | TH-07/E-07: owner-approved two-leg setup, whitelist-active evidence той же сети/периода, оба legs/end-to-end TCP+UDP/overhead |
| 9. Альтернативный CDN/Front Door | **Defer deployment по стоимости/ценности.** Standard/Premium поддерживает WS при disabled cache, idle5min/max2h; исключения SNI/Host требуют своих domains в одной subscription/routes. Чужой fronting отвергнут, base+usage оплачиваются. C08 | TH-03/07: budget/domain/origin TLS/Upgrade/WAF/close-recovery и signed carrier facts; проверить реальную независимость и ISP path |
| 10. IPv6 TUN leaks | **Adapt full family policy.** Outer family, inner IPv6 и DNS — три оси. DNS64 не исправляет numeric IPv4; Pref64 network-scoped. Android getNat64Prefix API30+, для 29 reuse OS resolver/observed CLAT; без proof IPv4 literal на v6-only unsupported/unknown. C09 | TH-01/RQ-03/09/AND-05/06/15/E-01/05: v4/v6/NAT64/CLAT/MTU/PTB/handover; capture-or-block обеих защищённых families. ipv4only.arpa не connectivity probe |
| 11. dns.rules.outbound:any / [proxy] | **Точный source ответ:** any означает metadata.Outbound≠пусто; именованный список — exact tag, а не тип proxy. VLESS и direct выставляют tag до dial, поэтому any охватывает и внутренние contexts. Условие не выбирает resolver и не доказывает protected OS/app DNS. В pinned docs deprecated/removal1.14; adapt domain_resolver/default_domain_resolver при обычном schema gate. C10 | TH-03/RQ-07/08/AND-06: renderer tags/user-vs-bootstrap scopes/other match fields/actions/negative results/cache/no-loop/signed migration; без массового upgrade |
| 12. Egress/site policy | **Reuse independent health axes.** HTTP403/challenge/geo-block при здоровых controls — scoped site/egress observation. CDN ingress не меняет exit; смена IP не гарантирует доступ и не всегда нужна. C11 | TH-02/04/COM-07/11/E-06: independent protected controls + выбранный target, workload parity, свежий proof после exit change; без входа в аккаунты |

Первичка точного DNS/parser:
[matcher](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/route/rule/rule_item_outbound.go),
[migration](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/docs/migration.md#migrate-outbound-dns-rule-items-to-domain-resolver),
[transport options](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/option/v2ray_transport.go).
Остальная первичка: [ECH RFC9849](https://www.rfc-editor.org/rfc/rfc9849.html),
[DNSSEC RFC4033](https://www.rfc-editor.org/rfc/rfc4033.html),
[NAT64 RFC7050](https://www.rfc-editor.org/rfc/rfc7050.html),
[Front Door WS](https://learn.microsoft.com/en-us/azure/frontdoor/standard-premium/websocket),
[fronting FAQ](https://learn.microsoft.com/en-us/azure/frontdoor/front-door-faq#how-does-azure-front-door-block-domain-fronting),
[billing](https://learn.microsoft.com/en-us/azure/frontdoor/billing).

Дополнительные C12…C14 решения:

- **РФ:** нет подтверждённого вечного общероссийского protocol/IP ranking.
  Cloudflare report относится к их сети/периоду, issue650 — к трём MTS testers,
  issue516 — одному Beeline report. Разделять обычный DPI, active allowlist и
  shutdown; актуальное полевое свидетельство возможно только на конкретных ISP/time/endpoint.
- **Один новый carrier-кандидат:** NaiveProxy, defer integration до TH-01/03/04.
  На 2026-10-24 официальный stable v154.0.8037.49-5, commit
  `d04efe1ea4477c2db24a7d0e2f068b3bee931e45`. Upstream advisory о compromised
  eMudhra intermediates делает старый v147 исторической ссылкой, а не выбранным
  binary. Перед фактическим выбором заново проверить release/advisory, third-party
  licenses, carrier capabilities, isolation/protect/bind, padding/auth и SBOM.
  [Release](https://github.com/klzgrad/naiveproxy/releases/tag/v154.0.8037.49-5).
- **Параметры/ML:** сохранить baseline, менять один factor с bounded stop rule.
  Универсальные mux/MTU/CC/padding/cooldown литература не даёт. Нужен paired
  useful-work эксперимент с failures/latency/recovery/tails/bytes/resources/energy;
  отсутствие эффекта закрывает кандидата, не запускает бесконечный fingerprint search.


<!-- /source-unit: 15.2 -->

<!-- source-unit: 15.4 -->
<a id="source-15-4"></a>
### 15.4. Уточнение выбора готовых компонентов

1. **Pinned netmon adapt:** tag SagerNet/tailscale
   `v1.92.4-sing-box-1.13-mod.9` теперь разрешён в неизменяемый commit
   `867c1f0012d915fa5fd57481dbf2a1ad16669fe7`. Exact Windows callback/go.mod/LICENSE
   получены, Git blobs проверены; Go1.24.1/BSD-3-Clause. COM-03 развивает один
   существующий владелец/epoch monitor. Наличие callback/unregister API и indirect
   dependency не доказывает полную интеграцию и фактическую доставку событий ОС.
2. **TUF** не оставлять «подобрать compatible release»: выбран v2.4.2
   `f5edbde31e5507f46db2069402dc38903fe6d9d4`, Apache-2.0, ADR-001. Head Go1.27
   — исторический объект сравнения. Android переиспользует portable verifier seam,
   а не Windows module и не Kotlin serializer для canonical signed bytes.
3. **Glorytun defer:** stable branch и latest release v0.3.4 указывают на
   `c113724eb0370ecd80d038192deeeb82a13ebed3` от 2020-03-25. Stable README описывает
   POSIX/Linux + libsodium; это не Windows/Android drop-in. Даты/README недостаточно
   для оценки безопасности. Для core release кандидат отклонён по интеграционной
   цене и отсутствию обязательного bonding use case. Полный native/crypto audit
   не нужен для такого отказа; при новом сценарии COM-17 нужен отдельный аудит.
4. Остальной список 14 COM upstream и 10 Android pins/лицензий остаётся §6/§12;
   просмотр лицензии исходника не лицензирует assets/бренд/сервис и весь будущий
   transitive binary graph. Exact composed artifact получает отдельный SBOM.

Источники: [netmon commit](https://github.com/SagerNet/tailscale/blob/867c1f0012d915fa5fd57481dbf2a1ad16669fe7/net/netmon/netmon_windows.go),
[TUF release](https://github.com/theupdateframework/go-tuf/tree/f5edbde31e5507f46db2069402dc38903fe6d9d4),
[Glorytun stable](https://github.com/angt/glorytun/tree/c113724eb0370ecd80d038192deeeb82a13ebed3),
[release](https://github.com/angt/glorytun/releases/tag/v0.3.4).

Отказ от build+sshx как постоянного VPS опирается на
[Workers Builds limits](https://developers.cloudflare.com/workers/ci-cd/builds/limits-and-pricing/)
и назначение [sshx](https://github.com/ekzhang/sshx). Временный диск сборки не
означает зарезервированное постоянное хранилище шлюза; другие продукты Cloudflare
не оцениваются по ограничениям build job.


<!-- /source-unit: 15.4 -->

<!-- source-unit: 15.6 -->
<a id="source-15-6"></a>
### 15.6. Окончательная последовательность разработки и доказательств

Полная последовательность, владельцы, зависимости и критерии находятся
только в [плане ПК](../../planning/knowledge-application/PC-PLAN.md) и
[плане Android](../../planning/knowledge-application/ANDROID-PLAN.md).
Старая семишаговая копия из теории больше не является отдельным планом.

Устойчивый принцип порядка: подтверждённый дефект защиты и доверия →
ограничение ресурсов и наблюдаемость → одно обоснованное улучшение →
раздельные native/device/полевые доказательства. Совмещать новое ядро,
mux, MTU, CC и сервер в одном сравнении нельзя без возможности отделить
вклад изменений. Новая маскировка не закрывает P0-владение и безопасность.

Полезные будущие входы: точные устройства/ОС, сценарий и измеримые цели,
состав релиза, полномочия на ключи, инфраструктура и доступ к нужному пути.
Их отсутствие фиксируется как отсутствующий вход, а не заполняется
предположением. Наличие команды в документе не меняет разрешённый режим.

Историческое завершение разбора 24 октября касалось оговорённого тогда набора.
Позднее обнаруженные ошибки и ограничения сохраняют собственные решения.
Новых исследований или внедрения из этой главы автоматически не следует.

<!-- /source-unit: 15.6 -->

<!-- source-unit: 16.7 -->
<a id="source-16-7"></a>
### 16.7. Что делать в разработке после этого среза

После этого среза оставались контракты portable TUF, начального DNS,
измерения и поддержки пользователя. Их предметные основания находятся в
[архитектуре доверия](../architecture/SAFETY-AND-TRUST.md#source-16-3),
[DNS](../observability/DNS-AND-DIAGNOSTICS.md) и
[повседневных сценариях](../ux/COMPETITOR-FEATURES.md).
Статус adapter/JNI/AAR и очередной ограниченный срез определяет владелец
платформы. Прежний список адресатов не выдаётся за действующее поручение.

<!-- /source-unit: 16.7 -->

<a id="research-priorities-2026-10-27"></a>
<a id="продолжение-исследования-приоритеты-на-2026-10-27"></a>
## Открытые инженерные вопросы по срезу 27 октября 2026

Ниже сохранены инженерные вопросы, выделенные при сверке корпуса
27 октября. Они описывают пробелы знания, а не действующее поручение продолжать
исследования. Порядок применения знаний задан [планом ПК](../../planning/knowledge-application/PC-PLAN.md)
и [планом Android](../../planning/knowledge-application/ANDROID-PLAN.md).

### Почему именно эти направления

В библиотеке уже подробно рассмотрены модели противника, транспорты,
адаптация/статистика, ресурсы, доверие и UX. Основной пробел теперь не в
количестве названий протоколов, а в **связи конкретного утверждения с полным
путём, проверяемыми предпосылками и действием, которое оно меняет**.

В срезе 27 октября были выделены пробелы: противоречие Vision между
тематической главой и R02-01; недостаточно раскрытый ECH/DNS bootstrap;
смешение byte/packet observations; pending trust/callback/producer contracts.
Закрытие прежнего desk scope в §15 не означает, что исчерпаны все исследования.
Архив, старые snapshots и авторские REPORT не переписываются.

### Пять выбранных направлений

Ниже сохранена **историческая карта вопросов 27 октября**. Она объясняет,
зачем появился последующий разбор; будущие формулировки в её строках
не запускают продолжение и не определяют нынешний порядок реализации.
Итоги 28 октября и их принятые области указаны ниже и в предметных главах.

| Очередь | Вопрос и причина выбора | Следующий конечный результат | Основание / граница |
|---|---|---|---|
| **1 / P0: свойства защиты и полномочия** | Какие реальные bytes защищает каждая стадия transport, и какое trust progress разрешено сохранять после ошибки? Ошибка здесь делает последующий «успех» небезопасным | Review R05-001-A/B завершён в ограниченной области; далее выбор supported reset/durability contract R05-002. Отдельно одна матрица confidentiality/authentication/replay для выбранной замены TLS-less ShadowTLS-композиции | [Принятые A/B](../research/reviews/2026-10-27-R05-001-PROGRESS.md), [R05-002](../research/05-TRUST-SAFETY/handoffs/R05-002.md), [R02-005](../research/02-CENSORSHIP/handoffs/R02-005.md); адресаты TH-01/03/05. Pending claims не приняты автоматически; live уязвимость/исправление не заявлены |
| **2 / P1: DNS/ECH от discovery до результата** | Когда recovery сохраняет приватность, а когда публикуется ложный success или допускается downgrade? Смена сети/CDN/Alt-Svc делает один hostname недостаточным | Нормативная часть дополнена в этом срезе. Далее exact TLS/resolver/caller matrix с владельцами lookup, retry, cache и observable negotiation; затем десять cases из новой заметки | [ECH/DNS](../transports/ECH-DNS-BOOTSTRAP.md), [наблюдения](../observability/DNS-AND-DIAGNOSTICS.md#service-binding-observations-2026-10-27); TH-02/03/05, E-04/E-06. Это не предложение включить ECH сейчас |
| **3 / P1: различение причин сетевого отказа** | Какие данные отличат объёмный порог, packet/time/connection-count, дефект endpoint и измерителя? Неверная причина провоцирует бесполезные переключения | Предрегистрация одного E-06 контроля, затем одного E-02 пилота: единицы, эквивалентный объект, пределы и альтернативные объяснения. Поле только после разрешения и текущих OS/safety gates | [Методическая дельта и четыре пилота](../methodology/RESEARCH-METHOD.md#measurement-comparability-2026-10-27); TH-02/06/07. Ссылки на чужие reports не заменяют свежие ISP/day/family blocks |
| **4 / P1: достоверные inputs адаптации и цена решения** | Какие callbacks и outcomes действительно измеряют то, что использует алгоритм? Missing, delayed и held resources не могут стать нулевой стоимостью/ошибкой | Отдельный review R04-006/007: flightBefore/After и actual caller. Затем минимальный producer contract R03-006/007/008 с abstain и deterministic baseline, без нового controller | [R04-006](../research/04-PERFORMANCE/handoffs/R04-006.md), [R04-007](../research/04-PERFORMANCE/handoffs/R04-007.md), [R03-008](../research/03-ADAPTATION/handoffs/R03-008.md); TH-02/04/06, E-05/E-08. Нет измеренного gains и разрешения на dependency upgrade |
| **5 / P2: полезное восстановление в сложной сети** | Доступен ли весь путь bootstrap→proxy→tunnel→task, когда ломаются общий resolver, старые flows или recovery prerequisites? Новый default не равен завершению работы | Один end-to-end сценарий из R06: named corporate proxy + IPv6-only/NAT64, с failure/dependency cuts, policy/credentials и ясным UX unknown. Затем task-based acceptance, не общий «VPN работает» | [R06-006](../research/06-RELIABILITY-UX/handoffs/R06-006.md), [R06-007](../research/06-RELIABILITY-UX/handoffs/R06-007.md), [R06-008](../research/06-RELIABILITY-UX/handoffs/R06-008.md); TH-03/07/08 и COM. Реальные доступы, ОС/устройства и user study не выполнены |

### Что уже получено в этом срезе

- **Исправлен source-факт:** Vision присутствует в названной public
  upstream closure. [Независимая рецензия](../research/reviews/2026-10-27-R02-01-VISION.md)
  принимает только эту часть R02-01; текущие artifacts/schema/server не проверены.
  Отдельно проверена существующая upstream CLI-команда `reality-keypair`:
  прежний negative search пропустил регистрацию в соседнем файле.
- **Дополнена нормативная теория:** RFC 9848/9849/9460 связывают конфигурацию,
  identity, retry, cache и privacy. Предлагаемая модель наблюдений помечена как
  предложение; соответствие продукта стандартам не установлено.
- **Исправлена интерпретация полевых данных:** нет равенства «16 КБ = 25 пакетов»,
  общего рейтинга CDN/REALITY/HY2 и вывода обо всём не-443 диапазоне по трём портам.
- **Приняты ограниченные R05-001-A/B:** [TUF progress/reset](../architecture/SAFETY-AND-TRUST.md#tuf-progress-and-reset-2026-10-27)
  различает прежний cache, новый rejected объект и допустимый prefix. Правило
  reset обоих TUF cache после ротации ключей timestamp/snapshot не сбрасывает application
  history; full-key S-only trace не подменяет все rotations или product defect.

### Когда углубление действительно закончено

Для каждого вопроса нужен проверенный источник/версия, сильный контрпример,
короткое решение reuse/adapt/defer/reject и конечный gate. Если ответ упирается
в отсутствующие actual source/artifact/field inputs, результат фиксируется как
unknown или handoff. Ещё один обзор не заменяет такие inputs. Новые протоколы,
ML, padding presets и дорогое provisioning не вытесняют выбранные вопросы
без нового конкретного сценария или доказанного ограничения baseline.

Указанные в прежнем срезе R02-005 иR05-002 получили последующие уточнения:
[композиция TLS](../transports/COMPOSITION-AND-TLS-CONTRACTS.md),
[различимость истории доверия](../architecture/TRUST-CONTINUITY-AND-AGREEMENT.md).
Это не означает принятия каждого утверждения исходных пакетов.
Открытые вопросы сохраняются в своей области; повторное поручение review
из исторической очереди не требуется.

<a id="research-priorities-2026-10-28"></a>
<a id="после-глубокой-сверки-28-октября-где-исследование-даст-следующий-результат"></a>
## Результаты глубокой сверки и непроверенные вопросы

[Обзор](../research/DEEP-REVIEW-2026-10-28.md) связывает предметные результаты,
а [рецензия](../research/reviews/2026-10-28-DEEP-REVIEW.md) фиксирует
ограниченные решения по отдельным утверждениям. Не все материалы получили
независимую рецензию; полнота отчёта не заменяет её.

### Что уже меняет теоретическую основу

- Шифрование, identity, effective TLS minimum, exporter и replay принадлежат
  конкретной **композиции и caller**, а не названию протокола. Есть достаточные
  source-основания для адресной проверки сохранения TLS-policy.
- QUIC callback units/position, fragmentation admission, byte ownership и
  peer deadlines образуют разные контракты. Наличие более нового upstream
  fix не доказывает совместимость старого consumer.
- Conditional historical interval, marginal rate, future risk и whole-policy
  intervention являются разными estimands; нужны разные producer assumptions.
- Trust recovery требует различимости истории, но не обязательно нового поля.
  Multirepo agreement может быть слабее выполнения всех constraints approvers.
- Probe signer/egress, Android UID/family scope, installed checksum и executing
  identity доказывают разные утверждения. Один положительный факт не заполняет
  отсутствующие доказательства следующего слоя.

<a id="следующие-исследовательские-приоритеты"></a>

### Открытые инженерные вопросы

| Приоритет | Один конечный вопрос | Выход и условие завершения |
|---|---|---|
| **P0 — композиционная безопасность** | Какие найденные TLS/version/identity ветки достижимы в фактическом актуальном profile→renderer→artifact? | Exact manifest/caller matrix и один допустимый encrypted candidate. Без чтения точного актуального дерева вывод о дефекте продукта остаётся открытым; наличие исходников в библиотеке не заменяет анализ вызывающей цепочки. TH-01/03/05 |
| **P1 — семантика adapter и ресурсов** | Как согласовать точные sender/controller рассматриваемой версии, ordinary/PMTU/PATH events, fragment admission и actual last owner? | Уточнённый R04-14 oracle и finite ownership/accounting matrix; source compatibility раньше любого performance сравнения. TH-02/06 |
| **P1 — границы знаний adaptive** | Кто гарантирует полноту cohort, совместный Γ/β и конкретный bridge к нужной future задаче? | Producer-owned proof assumptions и deterministic abstain при отсутствии. Для эффекта policy — назначение полного изолированного домена либо доказанный sequential/exposure support. TH-02/04 |
| **P1 — recovery при смене полномочий** | Как трактуются overlap/threshold-only/S→S′→S и continuation через expired root? | Явный supported ResetAllowed и эквивалентная различимость existing store; full old/new authority predicates. Новое поле/DB только при доказанной необходимости. TH-03 |
| **P1 — поздняя отличимость camouflage** | Отличим ли late inner handshake после finite padding-prefix на тёплом carrier от benign late activity? | Определённый full-window observer, будущие day/endpoint/transport splits, контролируемый stop/restart cost. Сначала план/контрпримеры; E-04/E-08 требуют разрешённого корпуса |
| **P2 — временное и географическое покрытие** | Как отделить execution path от upload path и сохранить contemporaneous DNS/IP/AS evidence при смене сети? | Одна bounded модель temporal joins/quality labels; затем согласованный dated dataset. Новые статьи и сообщение об unblock не актуальное полевое свидетельство. TH-02/07 |

Платформенные тесты/эксперименты остаются отдельными gates. В текущем исследовании
не запускались native/code tests, SDK, simulator, VPN, VPS или реальные сетевые
пробы. Открытые вопросы нельзя закрыть числом субагентов или количеством страниц.
