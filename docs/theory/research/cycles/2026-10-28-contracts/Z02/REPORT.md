# R28-Z02 — минимальный сопоставимый блок reachability и границы unknown

**Дата: 28.10.2026. Пакет: R28-Z02.**
Авторский статус **READY_FOR_REVIEW** для source/spec/manual-model области.
Источники/область чтения: [SOURCES](SOURCES.md); краткие выводы: [HANDOFF](HANDOFF.md).

## 1. Короткий ответ и новый результат

Нет универсального «минимум N измерений». Минимум зависит от **утверждения**:

- Для «в данной опубликованной записи connect к этому IP:port состоялся»
  достаточно одной идентифицированной операции с корректной семантикой поля.
  Это описательный факт записи, условный на достоверности producer; не
  историческая истина всего AS/всех абонентов и не вероятность будущего успеха.
- Для «контрольная попытка к тому же endpoint состоялась» нужен явный join
  по stage/IP/port/контексту. В выбранном WebConnectivity такой join есть для
  TCP, но отсутствует в сохранённом **HTTP control result**.
- Для «URL/приложение недоступно из-за ISP в часовом/региональном блоке» нужны
  отдельные повторения/контроли, адресная и временная согласованность и
  утверждённый критерий причинного вывода. Три выбранные записи этого не дают.
- Для покрытия всей совокупности (или её probability) нужны определённые
  denominator/sampling/unknown strata; «нет найденных success» не заменяет их.

**Новая дельта относительно R02-014-D:** source trace обнаруживает разрыв
адресной сопоставимости HTTP-контроля, семантику DNSConsistency по AS,
отсутствие snapshot-age gate у выбранного static cache и выбрасывание partial
body на error. Новая primary литература уточняет временные DNS joins и
submission-path quality; она используется для пределов, не для рейтинга РФ.

## 2. Фиксированный вопрос и evidence passport

### R28-Z02-A — join по endpoint/stage, не по label или домену

**Вопрос Q:** что позволяет утверждать один фиксированный опубликованный
блок о TCP reachability и о полном HTTP-получении `https://explorer.ooni.org/`?

Основной exact trace — прежняя запись S18:
`20260131203410.740128_RU_webconnectivity_d10431598d631e23`.
Компаратор S19 выбран только для проверки силы положительного свидетельства;
S16 — historical DNS/body/classifier counterexample. Новые records не добавлялись.

| Поле паспорта | S18, основной блок |
|---|---|
| Старт операции/измерения | measurement_start_time31.01.2026 20:33:10 UTC; относительные `t` имеют собственный begin |
| Начало набора URL | test_start_time20:10:36 — **не** начало данной операции |
| Время полного измерения | test_runtime60.131185676s включает TH-связь; завершение не upload-time |
| Группа provenance | self-reported RU, probe/resolver AS8402 Vimpelcom; city/access UNKNOWN; macos не доказывает fixed |
| Instrument | engine3.26.0; test_version0.4.3; recorded revision e9b4dc98c04d347f0ee5468c2585e32d94812b73; binary attestation отсутствует |
| Local DNS | system A:76.76.21.93 и66.33.60.194, t≈0.081s; ttl=null, не zero |
| TCP observations | оба IP:443 status.success=true/failure=null, t≈15.461/15.467s; control TCP entries тех же IP:443 successful |
| TLS observations | отдельные tcptls_experiment probes TLS1.3/h2 success; не автоматически та же TCP-сессия HTTPGet |
| HTTP observation | один GET, response code200, request failure=generic_timeout_error, t≈60.132s; сохранённый body пуст |
| TH DNS |66.33.60.35/76.76.21.123: **не local IP set**, несмотря на dns_consistency=consistent |
| TH HTTP |200/body_length65947, failure=null; IP выбранной HTTP-сессии/её start/end timestamps в result **отсутствуют** |
| API layer | anomaly=true/confirmed=false/failure=false в старом summary; backend implementation/revision UNKNOWN |
| Upload provenance | по существованию публикации известно, что запись дошла до storage; actual upload timestamp/path/collector geoIP не представлены |

**Estimand для одного endpoint:**
`Y(r, stage=tcp_connect, dst=76.76.21.93:443)` — сообщённый исход данной
операции. `Y=success` подтверждается field+source path. Допустимая формулировка:
«В записи r измеритель сообщает успешное установление TCP к endpoint; TH также
сообщает success к тому же endpoint внутри своего control response».
Нельзя заменять её «в20:33 весь AS8402 работает» или «control совпал с local
HTTP endpoint в ту же микросекунду».

**Estimand HTTP:** завершение одного GET в указанном request context.
В r оно сообщено как timeout, а не failure TCP establishment. Причинный
«ISP запретил этот домен», «получено0байт», «16КБ cap» и «это HY2/QUIC»
из r не идентифицируются. Результат отрицательного GET — scoped operational
failure; cause/общая availability **UNKNOWN**.

### Контекст двух остальных records, без расширения операторской матрицы

- S19 UID `20260131202120.671695_RU_webconnectivity_35145dae4a7e440f`:
 31.01.2026 20:21:18, AS8615 Central Telegraph, Android app5.3.0/engine3.28.0,
 revision пуст, network_type=wifi. Local IP66.33.60.34/76.76.21.164,
 TH DNS другие IP. GET200/failure=null, nonempty saved body, engine accessible=true.
 Это **один reported successful GET**; useful-task/subresources/protected-egress
 или отсутствие всех ограничений не доказаны. Wi-Fi upstream UNKNOWN.
- S16 UID `20240916045716.646385_RU_webconnectivity_1cf1f2dd6e536360`:
 16.09.2024 04:57:16, probe AS12668, resolver AS42728, answer AS31483;
 local answer188.186.154.88, control другие IP; TCP success/TLS EOF,
 engine blocking=dns. Это source-consistent priority label, не текущий
 ASN verdict и не raw API confirmed=true.

### Уровни допустимых выводов

| Утверждение | Достаточный объект в этом блоке | Решение |
|---|---|---|
| Поле данной операции содержит success | UID + raw bytes + путь/семантика field | Source/document факт допустим |
| Reported TCP endpoint был достигнут | local IP:port/step/time + trust producer assumption | Scoped positive observation допустим, не binary/wire attestation |
| Local/control TCP outcomes адресно сопоставлены | тот же endpoint в TH map, provenance TH, ограничение времени | Descriptive join допустим; exact control time неизвестно |
| HTTP timeout относится к проверенному TCP probe flow | общий transaction/session ID или actual HTTP remote address | В выбранном старом record не доказано |
| Конкретный URL не завершился в данном HTTPGet | request.URL/Failure/тестовый deadline, без общего causal label | Operational negative допустим |
| Все пути к URL заблокированы / причина ISP | полный designated endpoint set + temporal coverage + appropriate controls/alternative causes | UNKNOWN |
| Текущий public VPS/транспорт РФ достижим | свежий relevant owned observation | UNKNOWN, никакой тест владельца не проведён |

## 3. Exact source/pin/caller: где возникает и теряется сопоставимость

Все source statements этого раздела относятся к OONI probe-cli commit
**e9b4dc98c04d347f0ee5468c2585e32d94812b73**, предоставленному S18.
Source заново получен по immutable URLs и прочитан; remote hash caller/
summary/httpget согласуется с сохранённым baseline. Это **не current main**,
не exact Android3.28.0 и не unknown backend replay.

### 3.1 TCP/control versus HTTP/control — разные joins

`webconnectivity.go:149–168` получает локальный DNS, формирует endpoints и
посылает THRequest с **HTTPRequest=URL** отдельно от **TCPConnect=IP pairs**.
`:189–213` делает отдельные local TCP/TLS observations;
`ComputeTCPBlocking:235–245` сопоставляет локальный IP:port с TH map. Source
помечает blocked как `TH failure==nil && local failure!=nil` — это уже
analysis, не только wire observation.

`model/th.go:9–19,39–46,88–96`: control HTTPResponse хранит code/body_length/
headers/title/failure, но **не remote address/transaction identity/start/end**.
Из TCP controls всех известных IP не следует, что TH HTTP GET использовал
именно local GET endpoint. Доступность одного CDN front не гарантирует
одинаковую конфигурацию другого front за тем же domain/AS.

### 3.2 Что означает consistent DNS

`dnsanalysis.go:63–90` разрешает consistent, если измеритель и TH имеют
**общий ненулевой AS**. Только затем проверяется IP-overlap `:92–105`.
`control.go:40–49` заполняет TH AS через локальную GeoIP database.
S18 local/control DNS sets disjoint, но endpoint AS общий; consistent не
тождественно IP equality, validity-range или freshness. Поэтому его нельзя
использовать как единственный ключ сравнительного блока.

### 3.3 Выбор IP связан с локальным DNS snapshot, но не доказывает его freshness

`HTTPGet:48–69` получает `Addresses=dnsResult.Addresses()` и задаёт DNSCache.
`Configurer:51–70` помещает snapshot в map host→addresses.
`netx/http.go:16–36` строит Dialer/TLSDialer;
`netx/dialer.go:14–23` выбирает NewResolver;
`netx/resolver.go:21–24` оборачивает его static-cache.
`resolvercache.go:34–43,64–83` возвращает cache-hit напрямую, без TTL/age
проверки внутри этого scope. Это сохранение attempted snapshot, а не
«resolver всегда заново разрешит IP перед dial».

Для исходного hostname hit исключает новый lookup на этом пути. Другой
hostname после redirect может пройти underlying resolver. В S18 один
request, но exporter не заполнил request.address: конкретный HTTP selected-IP
из raw не устанавливаем только по candidate DNS set.

`queries[].ttl=null` не 0 и не infinity; observed snapshot не равен
авторитетному TTL-controlled mapping всего window. RFC1035 §3.2.1 определяет
TTL как permission на caching, **не** immutable DNS identity до конца TTL.
Не превращаем отсутствие age guard в exploit или новый product defect:
это свойство snapshot-style measurement, пригодное для конечного trace.

<a id="34-почему-code200--empty-body--timeout-не-равно0байт"></a>
### 3.4 Почему code200 + empty body + timeout не равно 0 байт

Новый source путь: `Runner.httpGet:63–103` вызывает транспорт, затем читает
body до EOF/ошибки. `netx/http.go:42–43` действительно подключает
`HTTPTransportSaver` с default snapshot. В `tracex/http.go:85–115`:

1. status/headers сохраняются до body read;
2. snapshot читается через LimitReader/ReadAllContext;
3. если read вернул error, сохранённому Event присваивается error, **не body**;
4. body/truncation флаг присваиваются только после successful snapshot read.

`archival.go:104–123` затем формирует raw request из Event.
В S18 поэтому совместимы code200, error и пустой saved body. **Нельзя**
вычислять фактически полученные payload bytes как len(saved body).
В нормальном пути snapshot cap также отделён от всего body: Runner дочитывает
остаток; отсутствие сохранённого полного body не автоматически failure.

`ArchivalHTTPRequestResult` может иметь address/t0/transaction_id, но этот
legacy exporter их не заполняет в прочитанном constructor. Наличие полей
в современной схеме не достраивает отсутствующие поля исторической записи.

### 3.5 Clock domains и время контролей

`tracex/archival.go:44–65,94–123` пишет относительное `event.Time.Sub(begin)`.
`measurement.go:92–98,150–159` отделяет start конкретного input от start набора.
В S18 test_start20:10:36 не заменяет measurement_start20:33:10.
Control HTTP `Date=10:53:49 GMT`, `Age=34776` и X-Vercel-Cache=HIT:
RFC9111 §§4.2.3/5.1 различает возраст ответа и момент test-request.
Date может относиться к cached representation, **не** к выполнению TH query;
не датируем этим заголовком «старый контроль утром вместо вечернего».
TH-local order известен по caller, точная wall-clock synchronization/TH-operation
interval отсутствует. Для узкого descriptive TCP join это ограничение;
для time-aligned causal/hour threshold — missing premise.

## 4. Versioned field crosswalk, без выдуманной compatibility

| Объект | Presence/type в fixed records | Инструментальное значение | Что не следует |
|---|---|---|---|
| queries/tcp_connect/tls_handshakes/requests[].failure | string или null, конкретная операция | отчёт локального эксперимента | boolean failure всей записи/диагноз цензуры |
| test_keys.blocking | S16"dns",S18"http-failure",S19false | engine union null/false/string | API confirmed или actual nationwide block |
| test_keys.accessible | false/false/true | engine heuristic outcome | useful-task/protected egress |
| dns_consistency | inconsistent/consistent/consistent | в historical source общий AS допустим | равный IP, current DNS и одинаковый endpoint |
| x_status |8480/8704/1 | experimental flags historical engine | version-independent bitfield/API score |
| API anomaly/confirmed/failure | summary S18/S19 присутствуют; raw root/test_keys отсутствуют | сохранённый backend response неизвестной revision | field absence=false; failure=false отменяет HTTP timeout |
| requests.address/t0/transaction_id | отсутствуют во всех 3 old records | unknown endpoint/transaction linkage | заполнять из современного optional schema |
| annotations.network_type | S19wifi; другие absent | self-reported local connection | fixed/cellular upstream или current user profile |
| backend_version/bucket_date | отсутствуют в raw | unknown collector version/process-date | modern API retrieval date=measurement time |

Freshly read schema pin **8a68ed35b1dfeaeec737c0f1e8985865828d75d7**
(22.10.2026) нужен для **документирования нормализации**, не трактуется как
историческая версия engine. В df-005-tcpconnect поле blocked в тексте названо
string, в примере boolean; actual recorded source bool pointer. Для каждого
field priority — raw presence + historical caller, не тип по названию schema.

## 5. Сильные opposing cases и существующие guards

### O1. Тот же timeout, другой control — другой engine label

Source `summary.go:118–129,199–206,240–268`: HTTP failure с good control
может дать http-failure; ControlFailure!=nil оставит Accessible/Blocking=nil,
раньше timeout switch; inconsistent DNS изменит cause label при одной request.
Это ручной opposing trace, **не fixture/run**. Server outage, CDN front difference,
local congestion/measurement deadline остаются альтернативами. Backend
classification неизвестна, потому не обещаем API labels для этих вариантов.

### O2. Положительный HTTPS label не доказывает полезный ответ

`summary.go:113–123` HTTPS-success shortcut не проверяет application content
или конкретный HTTP status. Authors' issue#2661 (30.01.2024, bassosimone)
описывает server-side block/CAPTCHA: HTTPS page response может быть marked
accessible, хотя intended content отсутствует. Это первичный developer
counterexample, не операторский incident. Для S19 code200/nonempty body
поддерживает reported GET success, но useful-task требует отдельной semantics.
Для `accessible=true` существует и consistent-NXDOMAIN branch `:131–147`;
один bool не означает даже успешного fetch во всех разрешённых входах.

### O2a. Успех snapshot и ошибка дочитывания хвоста — не одинаковый outcome

Связанное углубление после initial trace, **ручной conditional source-case**.
В рассмотренном caller transport saver читает не более snapshot cap
(default1<<17=131072) и возвращает HTTPResponse с восстановленным body
(MultiReader сохранённого префикса и остатка). Если префикс прочитан без
ошибки, event request.Failure=nil, code сохранён и BodyIsTruncated=true.
Далее `Runner.httpGet:94–95` обязан дочитать весь body и может получить error
в хвосте. `Getter.Get:53–68` сохранит этот error как общий result.Failure,
а Requests из saver останется успешным префиксом; `Measurer.Run:223–228`
публикует оба поля отдельно.

В этом условном состоянии `requests[0].failure=null`, но
`http_experiment_failure!=null`. HTTPS shortcut `summary.go:118–123`
возвращает accessible=true, **не обращаясь к** HTTPExperimentFailure.
Это точный пример, почему поле accessible и full-body completion имеют
разные contracts даже без CAPTCHA/DNS mismatch. Существующий Runner
**действительно читает хвост и сообщает ошибку** — отсутствие detection не
заявляется; потеря различия возникает лишь при чтении одного summary bool.

В fixed S19 HTTPExperimentFailure=null и request.Failure=null, поэтому
эта particular tail-error альтернатива **не наблюдается** в выбранной записи.
S18 request.Failure ненулевой, следовательно условный prefix-success путь
тоже не объясняет её как факт. Контрпример ограничивает перенос semantics,
не переоценивает существующий corpus и не доказывает bug frequency/цензуру.

### O3. Правильный recent source не аттестует producer

VCS в raw — self-report. OONI faulty measurements (06.07.2026) анализирует
check-in19–21.03.2025 и clocks02.03–01.04.2025; GeoIP может устареть,
upload/check-in идти через proxy. Credentials ограничивают sender-age/count,
не доказывают test-path outcome. Мы не переоцениваем raw truth по score и
не объявляем запись malicious из-за пустого revision Android.

### O4. «Partial block» не positive и stale join не verified lower bound

OONI LALIGA Methods/Limitations (30.06.2026) группирует IP×probe-AS×hour,
TH-контроль и own thresholds; DNS4times/day с04.05.2026, для раннего периода
используется **первый позднейший snapshot**. Авторы прямо допускают untested IP
в partial-block. Мы не импортируем >5/<1%/2/>90% критерии как classifier РФ.
Утверждение bound на домены условно истинно только при валидной исторической
DNS map/detection: неточно датированный join может и добавлять false
attributions, поэтому «все ошибки лишь занижают» не следует автоматически.

### O5. Витрина/collector/missing records не outage всей сети

Primary OONI Explorer report отделяет dashboard и main website/upload;
2025 unblock notice не observed all-network unblocking2026. Пустая API страница,
нет новых publications или недоступность dashboard не определяют Y по paths.
Стабильная submission не доказывает, что upload и measurement шли одним путём.
Настоящие user logs/collector accounts не читаются; оси неизвестности сохранены.

## 6. Продолжение внутри пакета: positive/negative strength и unknown strata

### R28-Z02-B — переносимость свидетельства ограничена квантором и целевой совокупностью

Это следующий существенный вопрос TASK, а не новый обзор операторов.
Пусть конечное заранее определённое множество **D** содержит admissible
endpoint/stage/context/time-cells. `Y_d=1` — stage-specific success в cell d,
`Y_d=0` — stage-specific failure по установленному критерию, `Y_d=U` — unknown.
Ограничение: клетки/свидетельства считаются достоверными и принадлежат D;
какое-то количество повторных публикаций не создаёт новые клетки.

- Для `exists d in D: Y_d=1` достаточно одного verified/scoped positive в D.
  Он не доказывает, что безопасный client выберет его, что весь dependency graph
  доступен или что Y=1 сохраняется после времени записи.
- Для `forall d in D: Y_d=0` требуется покрыть **все** designated клетки
  достаточным negative evidence. Если хотя бы одна U, universal negative
  **UNKNOWN**, не false и не partial success. Один failed attempt сам не
  доказывает true path failure — raw outcome и criterion условие отдельны.
- Если desired statement — existence for каждый пользователь/весь период,
  требуется иной квантор: positive одного пользователя не свидетель этого claim.

Если D известно и равновзвешенно, `N=|D|>0`, наблюдены s истинных 1,f истинных 0,
u неизвестных, N=s+f+u, то finite population share идентифицировано лишь:

` s/N <= theta_D <= (s+u)/N `.

**Вывод:** неизвестные могут все оказаться 0 или все 1; оба заполнения дают
края, независимость/MAR не нужна. Это deterministic missing-data identification,
**не** confidence interval и не новая ошибка budget theorem. Если N/sampling
и целевые strata неизвестны, формула не вычисляется; нельзя подставлять
число API rows или известных AS за N всех попыток РФ. Для weighted population
нужны заданные массы cells; не заменяем их assumed equal.

Самый сильный контрпример: два мира с одинаковым опубликованным блоком
(s=1,f=1) и arbitrarily many необследованными клетками: в одном остальные
success, в другом все failures. Block равен, national availability radically
разная. Неизвестный denominator + self-selection upload не позволяет
перенести observed fraction1/2. Это ручная indistinguishability модель,
**не оценка** данных MTS/МегаФон и не simulated результат.

### Temporal coverage passport, а не universal freshness seconds

Для переносимого утверждения нужно specify target window, clock uncertainty,
адресный mapping validity, action context и sampling. «Данные этого дня» не
автоматически same-period для rotating CDN. При неизвестном clock offset
обозначаем recorded timestamp, не true UTC certainty; `Date` response не
замена. Historical-to-future transportability требует независимого основания
стабильности; RFC2330 §9.2 делает temporal composition явной conjecture.

Сам positive S19 остаётся полезен **без** решения этих transportability gaps:
он свидетель reported successful GET того момента. Для deployment decision
now гарантированная applicability не доказана. Unknown не запрещает платформе
новую bounded admissible attempt; он запрещает повысить claim о результате
и заменить missing mandatory safety predicate опубликованным old success.

## 7. Готовое reuse/licence/cost, без создания новой платформы

### R28-Z02-C — reuse схемы допустим, indiscriminate dataset ingestion не бесплатен

| Ready artefact | Exact pin/API/scope | Решение и licence/cost |
|---|---|---|
| OONI data-format spec | ooni/spec8a68… df-000-base, df-005-tcpconnect, df-001-httpt | **Reuse schema concepts** presence/null/types/time/transaction scopes; root licence BSD-3 текст прочитан. Preserve notices при копировании. Не выдаём latest schema за historical producer code |
| Historical OONI source caller | probe-cli e9b4dc98… Runner/Configurer/static DNS/Saver/model | **Reference-only**, не встраивать engine; GPL3 LICENSE header подтверждён, complete dependency/legal compliance NOT_REVIEWED. Недостаточно одного HTTPGet helper для proof whole VPN |
| Published OONI records |3 fixed raw records; metadata+UID+prior SHA256; summary frozen отдельно | **Desk research reuse with provenance**, не live ingestion. OONI data licence pinned ce329e1… **CC BY-NC-SA4.0**: attribution/NC/SA/modified indication; copyrighted/database rights scope отдельно. Commercial data reuse не разрешён этим review |
| OONI LALIGA/faulty methodological examples |30.06/06.07.2026 primary publications | **Adapt reasoning**, не пороги/проценты/новый anonymous-credential subsystem; regional methods не product estimator РФ |

CC noncommercial не означает автоматический запрет всех factual conclusions
или законных quotations: licence §2(a)(2)/§8 сохраняет exceptions. Но licence
не автоматически разрешает коммерческую массовую адаптацию dataset.
OONI attribution не означает endorsement Snowden. При желании commercial
redistribution требуется lawful basis/дополнительное согласование — NOT_REVIEWED.

**Минимальный proposed normalized block (не реализация/API):**
`block_id; requested_claim/quantifier; declared_population/window;
raw_ref/hash/source_reading_scope; probe/resolver/endpoint roles;
operation_stage; local observed outcome; control_ref+stage+endpoint join;
query_name/RR/observed_time/TTL_presence; chosen_endpoint_presence;
instrument_revision; classifier_layer/version_presence;
clock/path/upload provenance; address/time coverage+unknown reasons;
licence/attribution; decision_use_limit`.

Нормализация не удаляет raw distinctions. При missing transaction_id нельзя
склеивать flow из похожих timestamps; missing/null/false различаются.
Адресные наборы сравниваются как sets для chosen claim, не по строке бренда.
Две copies одного measurement_uid не два независимых incidents; отсутствие
сырого UID допускает artifact identity, **не доказанный** device identity.

**Цена по scope, без выдуманных чисел:**
- source/version adapters и enum compatibility вместо одного universal bool;
- immutable own snapshot/index и явные missing fields; не полный OONI harvest;
- privacy: не собирать raw user logs/device IDs/секреты, export ручной;
- clock/DNS mapping/history требуют budget; unknown лучше ложного cache refresh;
- code reuse/licensing имеет integration/compliance cost, schema conceptual
  reference значительно уже. RAM/CPU/time/token/cost gains не измерены.

## 8. Paired C02 и последствие для платформ

На review-stage уже доступный C02 HANDOFF адресно прочитан: §1/claimA
отделяет разрешение попытки, credentials, validation и выпуск user traffic.
Не ждали/не опрашивали соседей; его ECH source/B/C independently не проверяли.

Наш strongest limitation **согласуется по scope**: unknown cause/reachability
не универсальный отказ от безопасной конечной попытки. Но внешний old record
не разрешает user traffic/понижение identity/confidentiality/floors и не заменяет
owned current validation. Согласие двух текстов не independent experimental
corroboration и не ACCEPTED. Матрицы C02 не менялись.

Адресаты Windows/Android измерителей и portable advisor: выдать различие
observed_success / observed_failure / insufficient_for_claim; нет нового
product status и no code. Остальные trust/action gates остаются платформе.

## 9. Scoped recommendation и конечность

### Закрытый список missing premises для выбранного claim

| ID | Недостающее основание | Какое усиление вывода блокирует |
|---|---|---|
| U1 | Actual remote IP/session/transaction ID HTTPGet и TH HTTP | Адресно совпавшее причинное сравнение HTTP local/control |
| U2 | TH operation timestamps и clock uncertainty | Same-window causal claim, а не order внутри одного measurement |
| U3 | Collector/check-in/test-path provenance и GeoIP актуальность | Атрибуция declared AS/access истинному пути, а не self-report |
| U4 | Exact Android source revision и backend classifier revision | Полная cross-version/API-label parity; main не подставляется |
| U5 | Valid historical domain→IP set для всего target window | Whole-domain недоступность при DNS rotation и untested endpoints |
| U6 | Defined finite target population/weights/sampling/completeness | National/region probability и перенос observed rate на unknown strata |
| U7 | Outcome semantics intended content/subresources и protected egress | Useful/protected completion вместо scalar accessible/GET snapshot |

Эти семь оснований — список для **усиленных** claims, а не blanket запрет
использовать известные stage-specific facts или делать новую admissible попытку.
Момент приобретения публикации не заполняет U2/U3/U5. Новое количество
перепечаток не заполняет U6. Для описательного reported TCP success U1/U7
не обязательны, если вывод действительно остаётся на этом уровне.


Предлагаемые решения:
- **A принять только как source-bound comparability criterion** и typed passport,
  с разрывами endpoint/time, partial-body и неизвестным backend.
- **B принять как условный конечный логический вывод** при named D/criteria;
  reject перенос national probability и universal-negative по partial sample.
- **C принять только reuse/adapt/defer decision**, не commercial licence approval,
  full parser/engine integration и не измеренный cost benefit.

Авторский максимум всех claims **READY_FOR_REVIEW**. Ни measured censorship,
ни current RF reachability/blocked ISP, reliability/advisor effect не доказаны.

Work stages1–4 пройдены; связанное continuation (positive/negative/unknown)
пройдено после первого source finding. Большего списка операторов не нужно.
Оставшееся: historical backend classifier pin; Android exact revision;
HTTP chosen endpoint/TH precise timings; true clock/GeoIP/path quality;
покрытие designated population, full application/protected outcome.
Они не заполнены догадками и требуют отдельного input/authorisation,
не текущего endless source search.

Конечные future acceptance **NOT_RUN** (не executable cases/fixtures):
1. согласованный archived full block со known endpoint/control identities;
2. reader review presence/null/false и version compatibility;
3. known DNS rotation до/после query при совпавшем AS;
4. unknown адрес при negative только части set;
5. HTTP partial read/code200 против saved empty body;
6. clock/upload/dashboard failure при сохранённом local outcome;
7. licence/legal check перед product-data integration.

Проверки трафика, тесты, fixtures, сборка, симуляции, crypto/native/live,
действия с ОС и VPS не выполнялись. Эти границы относятся к срезу
28 октября; результаты R02-014 не принимаются автоматически.
