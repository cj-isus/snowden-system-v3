# План разработки ПК-клиента

Редакция 4 ноября 2026 года. Цель — удобный Windows-клиент с проверяемой защитой,
быстрым восстановлением и измеримой производительностью. Сначала законченный
защищённый путь; затем рабочие сценарии и адаптация, пользу которой можно проверить.

Это последовательность дальнейшей разработки, а не отчёт о выполнении.
[Реестр требований и статусов](../../PLAN.md) и [снимок ПК](../../STATE.md)
остаются у владельца Windows. [План Android](ANDROID-PLAN.md) развивается
параллельно; общие договорённости согласуются один раз для конкретного потребителя.

## 1. От чего продолжаем

- При принятии этого плана снимок ПК — V2-198, `source/unverified`; V2-199
  и V2-199R продолжают PC-00/01/03/04, V2-200…206 — PC-01, V2-207…209 — PC-03 в исходниках. Существующие guards процессов,
  Stop, транзакций и обновлений сохраняем. Старые успешные проверки нельзя
  переносить на изменившийся код.
- TUN пока намеренно не получает общий допуск. V2-200 подключает native boundary
  `windows/tunnel_boundary.go` к original helper/WFP capture, но возвращает только
  подготовленное наблюдение с закрытым ingress. Same-image TUN control и worker
  activation по-прежнему отказывают; private native→worker/final Manager commit
  ещё не завершён. Ограничение нужно закончить протоколом, сохраняя strict barrier.
- Отказ от небезопасной композиции ShadowTLS и Windows-local TUF/storage/progress
  уже описаны в текущих исходниках. Не назначаем их повторно как отсутствующие
  функции; проверяем оставшиеся границы поведения и миграции.
- Общий trust SDK ещё не опубликован. `shared/adaptive` содержит отдельные
  исходники чистой policy, но не принятую интеграцию в VPN.
- Интерфейс уже имеет шесть разделов, десять страниц и подходящие компоненты.
  Развиваем их, сохраняя существующий Vue/Wails API и идентификаторы навигации.

### Публичная версия исходных условий

Сведения реального сервера, рабочие конфигурации и операционные квитанции исключены из публичного снимка. Исходники не подтверждают доступность сервера, VPN-путь или готовность релиза. Полный исходный план сохранён локально у владельца.

## 2. Порядок работ и владельцы

| Этап | Законченный результат | Основной владелец | Условие начала |
|---|---|---|---|
| PC-00 | Актуальная карта совместимости и доказательств | ПК + серверный оператор в своей области | Текущие исходники и обезличенный inventory |
| PC-01 | Защита и завершённый допуск пользовательского трафика | ПК | Существующий W4/process/protection source |
| PC-02 | Проверка профилей и согласованный переход на trust SDK | SDK + ПК | Конкретный C1/API и право записи SDK |
| PC-03 | Один реально пригодный канал и корректная диагностика | ПК + серверный оператор | PC-00, PC-01, принятый профиль |
| PC-04 | Понятный интерфейс всех существующих страниц | ПК | Проверенный presentation/action API для затронутого экрана |
| PC-05 | Смена сети и объяснимые рабочие правила | ПК | PC-01/03, effective capture contract |
| PC-06 | Ограниченные ресурсы и совместимый transport stack | ПК | Точный source/caller и критерий допуска артефакта |
| PC-07 | Один обоснованный adaptive consumer | Adaptive + ПК | Реальные producers, PC-01 и принятый C4/C5 |
| PC-08 | Безопасная установка и восстановление пары App/helper | ПК | C1/C2, существующие транзакции и packaging contract |
| PC-09 | Приёмка и ограниченный выпуск | ПК | Применимые P0 и отдельные разрешения на проверки |

PC-00, подготовка PC-02, интерфейсная оболочка PC-04 и offline-подготовка сервера
могут идти независимо. Доработка защиты не ждёт Android, нового сервера или ML.
Ни один этап не означает разрешения запускать тесты, менять сеть или делать deploy.

## 3. Этапы

### PC-00. Убрать неоднозначность исходных условий

**Работа.** Сопоставить актуальные source/STATE и старые FR/TH/OFF/COM с конкретными
оставшимися изменениями. Для первого серверного канала получить запись:
endpoint, порт, transport, TLS/SNI/authority, auth role, WS path, mux/early-data,
UDP/obfs, client/server versions и scope последнего результата.

Исправление безусловного DNS-hijack и сверку nginx upstream/listener выполнить
через серверного владельца. Перед применением — проверка в staging, сохранённая
известная версия и понятный rollback. Не копировать боевые секреты в документацию.

**Выход.** Несовместимость или неизвестное поле дают `unsupported/unknown`, а не
успешный Start. Серверная запись различает `rendered`, `checked`, `applied`,
`running`, `path checked`, `rollback`. Один VPS остаётся общей точкой отказа
всех своих каналов, включая варианты через CDN.

Host identity и безопасный операторский доступ принимаются отдельно. Ошибочно
распознанный ранее SSH fingerprint не становится trusted pin. До изменения
способа входа нужны подтверждённый ключевой доступ и сохранённый recovery path.
Сами ключи и пароли в план или сообщения не включаются.

**Опора:** [клиент–сервер](crosschecks/CLIENT-SERVER.md),
[один VPS](crosschecks/ONE-VPS.md), [FR-002/005/006](../../PLAN.md).

### PC-01. Завершить protection и W4c

**Работа.** Соединить original-helper protocol, узкое WFP-разрешение именно
диагностического потока, ограниченный raw response и конечную проверку текущего
owner/session/intent перед общим forwarding. Не заменять это обычным SOCKS probe
или широким разрешением интерфейса.

Свести Start/Stop/retry/Switch/Reload/Close к существующему владельцу операции.
Учитывать фактический возврат ресурсов: process/Job/helper/socket/interface,
а не только timeout, cancel или завершение интереса UI. Неизвестное завершение
сохраняет владельца и блокирует опасного преемника.

Сохранить различие ручного Disconnect и автоматической ошибки. Добавить
конфликт-безопасное сохранение и восстановление только собственных proxy writes:
исходное значение, тип/наличие, readback, generation и результат восстановления.
Чужие настройки, PAC/MDM и другой VPN не восстанавливаются по эвристике порта.

**Выход.** До принятого gate нет пользовательского forwarding. Отказ, поздний
ответ, повтор команды, Stop и неудачная очистка не снимают protection intent.
Переходы и outcome описаны source-контрактом; право включить TUN дополнительно
зависит от будущей Windows/WFP/процессной приёмки.

**Опора:** Stop ownership — `docs/tasks/TH-01-STOP-OWNERSHIP.md`,
TUN lifecycle — `docs/tasks/TH-01-TUN-LIFECYCLE.md`, FR-009/010, OFF-01/02/03/05.

### PC-02. Профили, доверие и общий SDK

**Работа.** Переиспользовать Windows-local полный TUF workflow, codecs и value
policy. До переноса согласовать callable C1: source/API version, исходные bytes,
purpose/approval, accepted predecessor, eligible progress, candidate, ошибки,
лимиты и точный clock contract. Регистрация trust нужна до записи общего пакета;
она не блокирует независимые исправления Windows.

Особые вопросы перед freeze:

- Единое фиксированное reference time должно использоваться самой проверкой,
  а не только возвращаться в echo. Clock floor, elapsed time и доверенное UTC
  не взаимозаменяемы.
- Root rotation/reset, intermediate roots и продолжение после ошибки требуют
  явной supported-таблицы. Новый pointer или input не становится accepted history.
- Для первого потребителя предлагается per-return preparation. Временная запись
  SDK в память не является durable barrier платформы; более сильное per-step
  обещание требует отдельного согласованного протокола.
- Проверить полезный полный bundle в пересечении лимитов Windows/Android.
  Не повышать Android cap молча, не выбрасывать историю и не заменять full TUF
  проверкой одной подписи.

Подключить один Windows consumer через существующий worker/storage/commit seam.
Сохранить fixed4/no-op/conflict/uncertain/cleanup distinctions. Runtime projection
не продлевает signed authority. Старый дубль удаляется после приёмки замены,
а не до неё. Не навязывать работающим законным путям новую семантику без
явной совместимости и процедуры перехода.

**Выход.** Описаны принятая версия и первый migration slice. Import/preview/apply/
recovery не обходят approval и transaction. Parity, persistence и native gates
остаются самостоятельной приёмкой.

**Опора:** [trust](sections/TRUST.md), [SDK migration](crosschecks/SDK-MIGRATION.md),
Android trust requests — `docs/android/CHANGE-REQUEST-TRUST-001.md`.

### PC-03. Рабочий сетевой путь и честные проверки

**Работа.** Проверить одну фактическую client/server пару до расширения списка
протоколов. WS/TLS, HY2, httpupgrade и REALITY — кандидаты с отдельными условиями,
а не заранее работающий набор. Конфиденциальность и авторизация endpoint должны
предшествовать передаче credentials и пользовательских данных.

Разделить transport handshake, probe-only path, пользовательский forwarding,
DNS и полезную прикладную операцию. Исправить доказательную семантику
`directLeakResult`: совпадение/расхождение удалённых IP само по себе не доказывает
локальную утечку или её отсутствие. Отдельно хранить topology/path scope,
`match/mismatch/unknown` и основание подтверждённого нарушения. Unknown не даёт
права ослабить защиту.

Bootstrap resolver/cache привязать к profile revision, роли endpoint, network
epoch, clock/freshness и лимитам. Protected user DNS не смешивать с bootstrap.
По каждой поддерживаемой IPv4/IPv6/UDP/TCP клетке нужен собственный результат.

**Выход.** Пользователь может применить один принятый профиль и получить
проверяемый scope работающего пути. Проверка через другой VPN явно так помечена;
порт, HTTP `/ip`, DNS answer, handshake и полный VPN не склеены в один PASS.

**Опора:** [сеть](sections/NETWORK.md),
[границы probe](../../theory/observability/PROBE-PROOF-BOUNDARIES.md), FR-003/003a/006/007.

### PC-04. Завершить интерфейс по страницам

Сохранить `NavRail`, `ContextTabs`, существующие composables и единственный
`api/backend.ts`. Wails bindings генерировать штатно после разрешения; руками
их не исправлять. Оболочка не требует нового router или отдельного state owner.

| Страница | Что довести |
|---|---|
| Главная | Один основной статус, Connect/Stop, причина blocked/unknown, область и возраст последней проверки |
| Контекст сети | Наблюдаемые факты, источник и свежесть; без диагноза DPI из timeout |
| Адаптация | Причина выбора, cooldown/budget и отсутствующие измерения; без выдуманных процентов успеха |
| Статистика | Known zero отдельно от unknown; единицы, период и источник |
| Локальное восстановление | Что принадлежит приложению, что можно восстановить, результат и конфликт |
| Каналы | Declared/eligible/selected/applied/reachable отдельно; активный канал не обозначать как избранный |
| Проверки | Названная проверка, scope, срок и следующий безопасный шаг |
| Журнал | Причины и фильтры, ограниченный redacted support preview до экспорта |
| Настройки | Профиль, mode restart, update/recovery с настоящим outcome |
| Локальные секреты | Явное безопасное действие без показа секретов в ошибках/телеметрии |

Левое меню задаёт раздел, табы — только его детей. Для одного ребёнка полоса табов
не нужна. Уменьшить вложенность карточек, сохранить тёмную тему с оранжевым акцентом.
Для каждой страницы: loading/empty/error/blocked/unknown/success, keyboard/focus,
малое окно, DPI и доступность. Save/dispatch/running не выдавать за applied/protected.
Stop остаётся доступным при ожидающем Start/Switch.

**Выход.** Восемь сквозных сценариев — первый запуск, import, Connect, switch,
сбой, manual Disconnect, resume и update/support — имеют понятный следующий шаг.
Source review интерфейса не заменяет browser/WebView2 и usability приёмку.

**Опора:** [product](crosschecks/PRODUCT.md), [COM и UX](sections/PRODUCT-ANDROID.md),
[навигация](../../../windows/frontend/src/composables/navigation.ts).

### PC-05. Смена сети и правила для работы

**Работа.** Использовать существующие network callbacks/epoch и владельца
восстановления. При смене сети или resume отзывать старый proof и admission;
не запускать второй monitor. Switch default не обещает перенос уже живых потоков.

Для apps/sites/LAN показывать effective capture и action в текущем SOCKS/TUN/WFP
режиме, точность identity и причину. Неизвестная identity не получает direct.
Проверить shared resolver/split DNS, рабочий LAN и конфликт с внешними policies.

Сценарии «Дом/Работа/Поездка», favourites и quiet mode добавлять на этот контракт.
Сохранённый сценарий не равен применённому. Portal/временный grant требуют
явных границ сети/срока/отзыва и не означают выключение всей защиты.

**Выход.** Есть предсказуемые applied/retained/blocked outcomes и объяснение правил.
Неизвестная continuity ведёт к контролируемому восстановлению, не к обещанию
«бесшовно». Новые временные grants отложены до принятого protection contract.

**Опора:** FR-001/010, COM-03…08/11/12/15,
[DNS и диагностика](../../theory/observability/DNS-AND-DIAGNOSTICS.md).

### PC-06. Ограниченные ресурсы и transport compatibility

**Работа.** Для каждого carrier/queue/cache/fragment/borrowed buffer зафиксировать
count+bytes до clone/spawn, срок и actual last-holder return. Отделить session
cancel от whole-carrier retirement. Устранить неограниченный per-DATAGRAM fan-out
и cache/reassembly lifetime там, где это подтверждено точным текущим caller.

QUIC/sing-quic/Brutal рассматривать совместимой парой с одним frozen source.
Не править module cache и не обновлять зависимости массово. Auto CC — baseline,
а не обещание лучшей скорости. Изменения требуют API/tags/licence/SBOM/rollback
сверки и будущих CPU/RAM/FD/latency измерений.

**Выход.** Есть структурные bounds, понятный отказ при pressure и критерии
приёмки. Численные выигрыши, hard RSS и wall-time не объявляются из одного
source review или Job limit.

**Опора:** [performance](sections/PERFORMANCE.md), TH-06, RQ-03/05/11/12/16/19.

### PC-07. Измерения и один adaptive consumer

**Работа.** Сначала согласовать существующий API1 с actual producers:
owner/session/intent/network/config/image/path, clock domain/unit/boot,
freshness/deadline, known/unknown, outcome и actual custody. Не подставлять
константы за отсутствующие измерения и не смешивать UTC с elapsed.

Latest health, полный учёт зарегистрированных попыток и recommendation — разные
вещи. Не строить процент пользовательского успеха из последних probe samples.
Сохранить errata и ограничения статистических моделей из базы.

Первый кандидат — suppression только optional automatic background work при
реальном coherent pressure snapshot. До подключения: baseline/parity/negative
приёмка. Затем один consumer, atomic final platform recheck и отключаемый cutover.
Он не подавляет Stop/revoke/recovery и не освобождает ресурс по timeout.
При отсутствии нужного producer оптимизация остаётся выключенной.

Ranking/failover мигрировать следующим отдельным срезом, если есть обоснование;
не держать две production policy и не ждать ML для детерминированного safety.

**Выход.** Recommendation объяснима, bounded и не даёт полномочий сама.
`RequiresPlatformGate=true` остаётся обязательной границей всех эффектов.
Сравнительная польза проверяется позже на сопоставимых path/workload/context.

**Опора:** API — `docs/adaptive/API.md`, адаптеры — `docs/adaptive/ADAPTER-MATRIX.md`,
первый consumer — `docs/adaptive/ADP-005-DESIGN.md`, FR-012 и ADP-002…008.

### PC-08. Установка, обновление и recovery

**Работа.** Довести существующую pair transaction App/helper, original Job/image
owner, журнал, fixed lock, version floor и restart handoff до принятого поведения
при каждом промежуточном состоянии. Криптографическую authority предоставляет
PC-02; packaging/ABI и process ownership не выводятся из успешной подписи.

Разделить staged/applied/accepted/cleanup-uncertain/rollback-failed. Сохранять
known-good material и recovery kit вне VPS. Pending/неизвестный commit не должен
запускать новую сеть или маскироваться как no-effect. После обновления ordinary
fresh admission нужен заново; запуск GUI не означает VPN Ready.

**Выход.** Либо принятая durable install/recovery state machine, либо явный
unsupported для schema2. Не включать public install по наличию исходников.
Старые клиенты, reparse/ACL/volume aliases, power loss и UAC — отдельные gates.

**Опора:** pair update — `docs/tasks/TH-01-UPDATE-PAIR.md`,
[release trust](crosschecks/TRUST-RELEASE.md), FR-011, OFF-04/06/07, TH-08.

### PC-09. Доказать готовность и выпускать постепенно

| Gate | Что принять | Чего он не доказывает |
|---|---|---|
| Source/local | Точный cut, применимые unit/negative/type/build проверки после разрешения | Windows/WFP и сетевой путь |
| Native/OS | Helper/Job/WFP/Stop/cleanup, crash/restart/install, DPI/a11y/tray | Доступность VPS из нужной сети |
| Protected path | Конкретные канал, ОС, network/egress и endpoint; DNS/IPv4/IPv6/UDP scope | Все операторы и все пользовательские задачи |
| Рабочие сценарии | Приложение/звонок/видео/рабочий доступ, переходы и восстановление | Абсолютное превосходство над другими VPN |
| Release | Identity App/helper, reproducibility, лицензии/SBOM, подпись, rollback/support | Автоматическое право на deploy или публикацию |

Отдельно принять сценарии power loss, истёкшей authority, позднего ответа,
потери DNS/UDP, отказа origin/CDN и несовместимого обновления. TLS errors нельзя
обходить отключением проверки сертификата. Сначала ограниченный RC с поддержкой
и безопасным откатом, затем расширение только подтверждённой support matrix.

Цели задержки/восстановления/ресурсов объявлять до сравнительного опыта на
зафиксированном workload. Пользу измерять вместе с ценой переключений,
ложных отказов и пропущенных наблюдений; сейчас численный выигрыш не подтверждён.

## 4. Общие зависимости без лишних потоков

| Contract | Кто готовит | Кто принимает |
|---|---|---|
| C1 trust | SDK после регистрации; platform capture/storage остаются у ПК/Android | PC-02 и Android trust этап |
| C2 artifact/API | Владелец конкретной сборки; exact source/tags/ABI/licences | Каждый платформенный consumer отдельно |
| C3/C7 channel/server | Серверный ZCode даёт факты apply; профиль задаёт client authority | PC-00/03 и Android network этап |
| C4 observation/policy | Платформы дают факты, adaptive — чистое решение | PC-07 и Android adapter с final recheck |
| C5 custody | Владелец ресурса платформы | Stop/recovery/consumer, без передачи права timeout-ом |
| C6 UI/action | Существующий backend presentation | PC-04; Android имеет собственный UI |

Общие документы [KA-1](PLAN.md) и [сверка решений](RECONCILIATION.md) — основание
контрактов, а не неизменный source inventory. Новые БД, Android2/3, второй
controller и отдельный координатор core не нужны без доказанного разрыва.
Серверный оператор не получает права подписывать клиентскую trust history.

## 5. Сохранение прежнего объёма и заимствование

| Старый объём | Где продолжается |
|---|---|
| FR-001…007 | PC-00/01/03/05/06/09 |
| FR-008…011, OFF-01…07 | PC-01/02/08/09 |
| FR-012, TH-02/04/06 | PC-06/07/09 |
| TH-00/01/03/05/07/08, OFF-08/09 | PC-00…05/08/09; серверный backlog в своей области |
| COM-00…15 | PC-04/05/07/08/09; специфические grants только после своего gate |
| RQ-01…20, TR-01…06 | Соответствующие network/resource/measurement этапы; спорное сохраняется как review/defer |

Таблица — маршрут, не закрытие каждого требования. Подробные записи остаются
в реестре; не охваченный узким срезом пункт не исчезает и не получает PASS.
Продвинутые transports, bonding/multihop и новые telemetry/cohort stores
отложены до отдельной пользы и принятой совместимости.

Переиспользовать существующие Vue/Wails компоненты, Windows-local TUF workflow
и bounded policy API. Для внешнего кода нужны exact pin, licence/NOTICE,
transitive dependencies, поддерживаемый caller, стоимость сопровождения и откат.
В частности QUIC/Brutal не переносится по одному подходящему методу.

## 6. Ближайший конкретный срез

ПК-владелец принял **весь PC-00…09**, сохранив прежние FR/TH/OFF/COM/RQ/TR.
Маршрут исполнения и открытые границы — [реестр §5.11](../../PLAN.md#pc-plan-execution).
V2-199/V2-199R уточняют сравнение egress, ограничения противоречивых отчётов и presentation в
PC-03 — `docs/tasks/PC-03-EGRESS-SEMANTICS.md`; W4c и его native gate этим не закрыты.
V2-200 соединяет original capture с platform lifecycle scope, exact closed WFP и
typed custody failure; prepared observation не является общим TUN admission.
V2-201 привязывает fixed native flow к actual selected adapter с original
upstream/I/O retirement и IPC4 receipt; final private worker/Manager join ещё OPEN.
V2-202 передаёт opaque original native result в исходный worker через one-use
confirmation/probe binding IPC5. OS-control separation и final platform/Manager
admission не закрыты; TUN остаётся недоступен, исполнение проверок не разрешено.
V2-203 отделяет informational OS comparison от required native/actual-phase,
добавляет actual admitted periodic без нового helper/native refresh и без нового
token/parent permit; IPC6 и existing presenter сохраняют эти различия. Unknown
роль остаётся required unavailable; standalone TUN без private binding не успешен.
Native-expiry-bound issuance/final compiled platform+Manager admission ещё OPEN;
initial TUN явно unavailable, source не доказывает compile/runtime/OS/UX.
V2-204 реализует отдельную original-worker prepared native+outbound lane:
одноразовая issuance/emission, deadline не позже original native/5s, IPC7 и
opaque original parent source. Ordinary permit не получает эту authority;
публичный required activation остаётся unavailable. Final native consumption и
compiled platform/Manager/WFP/ingress commit ещё OPEN, без исполнения проверок.
V2-205 реализует guarded final consume/compiled WFP phase/opaque Manager receipt
в исходниках. Acceptance const остаётся false: TUN не включён и normal helper
Establish не запускается. Следующий P0 source-аудит — commit→finish/Ready при
поздней отмене/Stop; native/OS/ресурсные/release критерии этим не закрыты.
V2-206 в исходниках разделяет terminal decision/own cancellation, сериализует
Stop и Running decision и проверяет original App ctx до Ready. Late fault
отзывает positive rollback outcome и сохраняет владельца через cleanup. Race/
runtime не проверены; acceptance gate закрыт. V2-207 различает unavailable и
actual required mismatch, сохраняет это при повторной нормализации и направляет
safety report до debounce. Local protection/cleanup/trust и неподдержанные
классы не пишут channel FAIL; bounded recovery work не становится network
evidence. V2-208 добавляет actual profile/endpoint/role/network/process-scoped
TTL hint/cache, existing wire DNS codec и original network/file retirement,
per-channel renderer hints с сохранёнными TLS SNI/Host. Source8 Go/4 docs, без
исполнения проверок. Следующее — PC-03 engine DNS/capture/family/path/refresh;
cold-restart cache identity/hard I/O time bounds и runtime приёмка открыты.
V2-209 добавляет service-only per-outbound bootstrap, protected TLS final/
independent cache и port53 hijack перед engine split/LAN; blind TUN exclusion
удалён, strict Config gate обновлён без изменения подписанного profile schema.
Source4 Go/4 docs, pinned source/primary docs/gofmt/manual diff only. Далее —
active-route address refresh/reconnect/family/path; DNS/native Windows/PKI/fixtures/
ресурсная приёмка остаётся отдельным незакрытым объёмом.
V2-210 [service refresh](../../tasks/PC-03-ENDPOINT-REFRESH.md) сохраняет original
service hostname, expiry hints в полном runtime subject/rollback и private one-use
seed до открытия ingress. При новых physical dials existing resolver работает в
original per-box domain: один flight/до16 borrowers, без persistent worker cache
или сброса здорового соединения; ordinary failure сохраняет stock family fallback.
Local clock/cancel/busy/cleanup передаются как protection unavailable, uncertain
retirement закрывает admission и сохраняет original owner. Source16 Go/4 docs,
gofmt/manual diff/bytes-SHA only; реальный reconnect/family/path/OS/resource/API
fixture acceptance не проводился. Далее source family/path scope и stale-operation
handoff; полный PC00…09 и прежний backlog остаются OPEN, TUN gate=false.
V2-211 [network handoff](../../tasks/PC-03-NETWORK-HANDOFF.md) отзывает original
path source на delivered observer down/change до cancellation callbacks. Down
сохраняет health history/номер, но закрывает pending operation/bootstrap/render/
probe positive publication; committed engine lifetime не заменяется. Source6 Go/
4 docs/gofmt/manual diff only, без тестирования. Physical path/family matrix,
worker DNS-flight handoff, debounce/race/native/OS/API fixtures остаются OPEN;
далее явный scope IPv4/IPv6/UDP/TCP по фактическому измерению, без общего PASS.
V2-212 [measurement scope](../../tasks/PC-03-MEASUREMENT-SCOPE.md) добавляет pure
bounded projection typed HTTP/DoH facts и optional App/UI view: приложение
отличает полученный ответ от непроверенных native IPv4/IPv6 × TCP/UDP и local
safety. Existing ProbeReportView переиспользован для semantic таблицы, без новых
карточек/измерителя. Stale/legacy/partial не promoted; required/informational и
skipped учитываются отдельно. Source3 Go/2 TS/1 Vue/4 docs, gofmt/manual diff only;
bindings/typecheck/browser/a11y/DPI/API/native/family-path acceptance OPEN.
Следующее source — worker DNS-flight handoff на смене сети; весь PC-план активен.
V2-213 [worker DNS handoff](../../tasks/PC-03-WORKER-DNS-HANDOFF.md) связывает
delivered observer event с original DNS-generation retirement/resume через
private IPC9 и один coalesced task/one pending value. Actor/config/tag/epoch/
admitted/lifetime guards, seed/late-fault fencing, shutdown cancellation перед
Stop; успешный handoff сохраняет established SDK transports. Source9 Go/4 docs,
gofmt/manual diff only; no tests/IPC/binary/worker/network/native execution.
Legacy8 fixtures/generation, races/Close/shutdown/control latency/stock pool/
already-issued SDK dial/family/OS/resource/terminal-owner reconciliation OPEN.
Далее source control-fault/reconnect/observer scope; весь PC00…09 сохраняется.
V2-214 [observer availability](../../tasks/PC-03-OBSERVER-AVAILABILITY.md) отделяет
unknown от explicit all-link-down: ошибки/ambiguous/TUN не pause DNS и не health.
Down только по полному непустому non-loopback snapshot без up links, включая
virtual/IPv6/TUN. Local IPv4-preferred lookup допускает IPv6 при ошибке IPv4,
без family/path PASS. Baseline delivered; unknown/TUN gap очищает pending и
перезапускает debounce, observed A→down/B→A сохраняет discontinuity/change.
Same sample не продлевает debounce, initial confirmed-down доставляется,
captured event class вне watcher lock. Source1 Go/4 docs/gofmt/manual diff/SHA
only; tests/fixtures/collector/socket/native/build не выполнялись. Actual flags/
virtual/PPP/IPv6/Close/races/debounce/OS/resources/API gates OPEN. Далее source
terminal DNS-owner/control-fault/manual reconnect reconciliation, full PC active.
V2-215 [DNS observer recovery](../../tasks/PC-05-DNS-OBSERVER-RECOVERY.md) добавляет
sticky terminal lease revocation, exact runtime DNS owner и guarded non-safety/
health/retained Running publication. Local fault снимает Ready/native cache и
подавляет channel retry; independent safety сохраняется. Start joins terminal
task до operationMu; actual Manager.Stop/helpers и original done до одного
successor на App lifetime, без второго poller/замены uncertain owner. Failed
entry wait сохраняет actual successor, latest retired predecessor bounded1.
App operations наследуют App.ctx. Source6 Go/4 docs/gofmt/manual diff/SHA only;
tests/fixtures/build/task/native/OS/resources/API acceptance OPEN. Далее source
original poller completion/stop ownership; full PC00…09 и прежний backlog активны.
V2-216 [network observer owner](../../tasks/PC-05-NETWORK-OBSERVER-OWNER.md) даёт
poller original App ctx/entry/done/fault и связывает его с DNS lease. Producer
fault отменяет только actual linked task/runtime; local recovery без health/
epoch synthesis. Start joins terminal tasks до operationMu, actual Stop/helpers
и original done до одной новой пары; healthy poller не дублируется. Baseline
unknown, ordinary health только после actual delivered event; unsafe stop path
сохранён. Shutdown cancel обоих до общего wait/re-read original done, uncertain
retained, predecessors bounded1 each. Source7 Go/4 docs/gofmt/manual diff only;
tests/fixtures/build/task/native/OS/resources/API acceptance OPEN. Далее collector
cancellation/socket Close custody и first-event/Start/automatic resume integration.
V2-217 [socketless route facts](../../tasks/PC-05-SOCKETLESS-NETWORK-FACTS.md)
убирает UDP observer socket/Close и Addrs scan: local Windows route/source/LUID/
class query через pinned types/native APIs с повторными tuple/index checks.
Original caller ctx, cooperative whole-sample700ms/checkpoints, no detached
timeout/Background; unknown/all-link-down semantics сохранены. Class captured,
fire/preference без второго MIB read; non-Windows unknown/no socket fallback.
Source7 Go/4 docs/gofmt/manual diff/SHA/public primary docs only, без tests/
fixtures/build/API/native/network execution. ABI/API/source/DAD/family/virtual/
cancel/Stop/race/resources acceptance OPEN. Далее first-real-event/Start/
ограниченное automatic recovery/resume; полный ПК-план активен.
V2-218 [Start network recovery](../../tasks/PC-05-START-NETWORK-RECOVERY.md)
сохраняет original Start cause/intent при сетевой отмене. Actual Stop/helpers
success даёт private retirement receipt; только новый current path ускоряет
existing retry, полный новый Start/trust/probe обязателен. Cleanup uncertainty
блокирует retry, также в certificate-expired trust path. Timer never-reset/
saturating serial + captured intent отсекают старый callback; Stop не join.
Общие max2 issued network fast requests за fail-closed episode/max1 на original
path, carry через automatic IDs/backoff, full cancel reset; revoked fast path
возвращается к backoff, dead App.ctx не retry. Source5 Go/4 docs/gofmt/manual
diff/SHA only, без tests/fixtures/build/parser/product/native/network execution.
Timer/race/cleanup/Stop/intent/API/resources/OS acceptance OPEN. Далее source
cause/intent вне Start и desktop suspend/resume; full PC00…09/old9/SDK/W3b/
interprocess/durability/UX/release/comparative OPEN. IPC9/Job8 прежние,
native gate=false/TUN unavailable/schema2 unsupported.
V2-219 [switch/observed retirement](../../tasks/PC-05-START-NETWORK-RECOVERY.md#v2-219--переключение-reload-и-остановка-по-наблюдению)
сохраняет joined switch/Reload error через cancel finalizer и original intent
до operationMu/во всех failure и observed Stop/trust paths. Любая cleanup error
typed local/BLOCKED/no retry; joined rollback не обходит custody failure.
Atomic attempt token до completion, original path/epoch stale success/failure
fence incl epoch0-down. Local failures defer own attempt, сохраняют budget/
cooldown, не candidate hint/health grant. Automatic Applied не manual Started
reset; exhaustion не Stop retained Running. Source3 Go/4 existing docs/gofmt/
manual diff/SHA only, без tests/fixtures/build/parser/product/native/network.
Queue/Stop/cancel/intent/reset/cleanup/API/race/resources/OS acceptance OPEN;
далее async negative/trust callbacks и desktop suspend/resume. Full PC/old9/
SDK/W3b/interprocess/durability/UX/release/comparative OPEN, native gate=false/
IPC9/Job8/TUN unavailable/schema2 unsupported.
V2-220 [async fault scope](../../tasks/PC-05-START-NETWORK-RECOVERY.md#v2-220--исходный-scope-фоновых-ошибок)
связывает first DNS negative fence с immutable actual Manager/Engine/config/id;
повторный fault не adopt successor, cancellation вне locks, повторный tuple/id
перед retirement. DNS control/trust lifetime/manual probe errors scoped; clock
checkpoint id до wait + actual applied manager/report identity до I/O. Late
fault не новый user intent, probe cleanup typed local/no retry. Current non-test
generic failure calls отсутствуют. Source4 Go/4 existing docs/gofmt/manual diff/
SHA only, без tests/fixtures/build/parser/product/native/network. Queue/scope/
cancel/Stop/cleanup/locks/I-O/API/runtime/race/resources/OS acceptance OPEN.
Далее desktop suspend/resume и generic decision-to-mutation/legacy publication
atomicity; full PC/old9/SDK/W3b/interprocess/durability/UX/release/comparative
OPEN, native gate=false/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.
V2-221 [power continuity](../../tasks/PC-05-POWER-CONTINUITY.md) связывает Win32
sleep/automatic-resume registration с original poller: one cookie/thunk/slot,
opaque context/pinned params, actual unregister + entered callbacks, uncertain
native custody retained. Callback only local negative lease/path + wake1,
existing poller/DNS retirement; ordinary retry actual Stop/helpers/native close
до fresh pair/source/full Start/probe. No synthetic up/down/epoch/new monitor;
Ready projection masked, independent local cleanup preserved across trust route.
Source13 Go/4docs/gofmt/manual diff/SHA/primary docs only, без tests/fixtures/build/
native/product/network execution. Kernel revoke/ABI/power modes/cleanup/queued
actions/UI/clock/API/resources/OS OPEN, native gate=false/IPC9/Job8 unchanged.
Disconnected observer recovery may await manual Start; no measured wake-speed
claim. Далее generic decision-to-mutation/legacy publication atomicity и
disconnected producer recovery; full PC/old9/SDK/W3b/interprocess/durability/
UX/release/comparative OPEN, TUN unavailable/schema2 unsupported.
V2-222 [disconnected recovery](../../tasks/PC-05-POWER-CONTINUITY.md#v2-222--восстановление-наблюдателей-при-отключённом-vpn)
использует existing watchdog/App.ctx до auto-connect return. Power gap не ordinary
health; только inactive current intent/no engine/both original done/known clean
power cause + actual helper/native retirement допускают observer pair setup.
Нет engine Start/Stop/proxy/policy Arm-Release/probe/health/intent mutation; busy defer и one
restart attempt per original producer/id, queued intent сохраняется. Source5 Go/
4docs/gofmt/manual diff/SHA only, без tests/fixtures/build/native/product/network.
Watchdog/queue/helper busy-mixed fault/API/registration/UI/runtime/resources/OS
OPEN; далее generic decision-to-mutation/legacy publication atomicity. Full PC/
old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN, native=false/
IPC9/Job8/TUN unavailable/schema2 unsupported прежние; wake speed не измерена.
V2-223 [probe intent](../../tasks/PC-03-PROBE-INTENT.md): original App intent
в operation ctx; lifecycle связан с конкретным Manager, retained rollback
выполняет fresh Run под исходным caller ctx того же intent/Manager. Periodic
binding до Run сохраняет applied pointer/Manager/engine/generation/path/epoch
и intent из исторической identity сессии; старые PASS/time не переиздаются.
Guard до/после factory и при completion, BeforeChange/native commit и original
native path fence не дают запоздавшему probe принять новый App id. Lifetime
и independent cleanup сохранены. Source7Go/4docs/gofmt/manual diff/SHA only;
без tests/fixtures/build/native/product/network. Queue/rollback/rejection/API/
power/runtime/OS/resources OPEN; далее writer/terminal/legacy publication
atomicity. Native=false/IPC9/Job8/TUN unavailable/schema2 unsupported, full PC/
old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.
V2-224 [operation/cleanup](../../tasks/PC-03-PROBE-INTENT.md#pc-v224-lifecycle-source)
исправляет actual cleanup→positive BeforeChange coupling, пропущенный V223.
Separate BeforeEngineStop сохраняет restriction retained boundary после cancel/
нового intent, actual engine/proxy cleanup продолжается при error/panic;
Manager.cleanup errors typed local custody. Positive source gates перед
selector/Start/Reload/rollback/proxy и по сторонам forwarding RPC; terminal
Running под original Manager→App→epoch value lease, release до watch/done/cancel.
Failure не закрывает done до actual cleanup. No native I/O under source locks;
нет полного native writer/power-callback atomicity claim. Source8Go/4docs/gofmt/
manual diff/SHA only, без tests/fixtures/build/product/native/network. API/
partial lease/queue/cleanup/rollback/runtime/OS/resources OPEN; далее private
worker/native writer/legacy App publication. Native=false/IPC9/Job8/TUN
unavailable/schema2 unsupported, full PC/old9/SDK/W3b/UX/release OPEN.
V2-225 [queued writer](../../tasks/PC-03-PROBE-INTENT.md#pc-v225-operation-sources):
negative-only intent/operation/path/closing atomic sources проходят через ctx
до Engine.mu и pipe writer; без root callbacks/new grants/wire fields. Все4
регистрации App intent revoke старый source до ID; normal end отзывает только
операцию. Report private original intent/Clone поддерживают periodic после
успешного Start, без adoption нового source. Foreign rebind only denies;
core deadline/source повторены перед writer и после I/O, unsent revoked request
не резервирует sequence, uncertain reserved owner не переиспользуется.
Passive intent/operation revoke channels будят existing exchange без нового
watcher; actual process/Job/I-O retirement остаётся отдельным gate.
Independent Stop/native revoke/reap/lifetime сохранены. Source11Go/4docs/gofmt/
manual diff/SHA only, без tests/fixtures/build/product/native/network. In-flight
write/worker/kernel/legacy App publication/API/JSON/queue/OS/resources OPEN;
full PC/old9/SDK/W3b/UX/release/comparative OPEN, native=false/IPC9/Job8/TUN
unavailable/schema2 unsupported прежние.
V2-226 [report scope](../../tasks/PC-03-PROBE-INTENT.md#pc-v226-report-publication):
private original path/actual-work marker у report/Clone. Positive/retained/
health/selection требуют exact current intent source/path/epoch; public flags
не scope. Только actual unsafe fact может остановить того же inactive original
Manager-engine-owner после network/time loss, через negative-only guard, без
health/time refresh. Pre-work gap не safety fault, Ready снимается до cleanup;
observed cleanup error сохраняется независимо от later Stop nil. Bound policy
Observe и typed owned fault/panic сохраняют actual cause; pure cancellation
не выдаётся за policy failure. Source9Go/4docs/gofmt/manual diff/SHA only, без
tests/fixtures/build/product/native/network. API/Clone/JSON/negative scope/
queue/cleanup/power/runtime/OS/resources OPEN; далее service-DNS/remaining
worker-native writes. Full PC/old9/SDK/W3b/UX/release/comparative OPEN,
native=false/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.
V2-227 [service-DNS scope](../../tasks/PC-03-WORKER-DNS-HANDOFF.md#pc-v227-dns-event-scope):
original request/source/applied actor до event callbacks; exact event source из
epoch commit, без позднего adoption после operationMu. Existing periodic identity
reader переиспользован без health promotion. Down control source не physical path;
task/power negative fences доходят до Engine/pipe. Original observer revocation
до cancel/native cleanup ожидания, original publication lease и done независимы.
Scoped reaction/retry не принимают новый Start/actor; сохранены episode/path caps.
Pure canceled healthy worker не policy failure; independent original custody/
failed engine/panic сохраняются после event loss. First task fault bound к
registered request source и original tuple. Core pre-reservation canceled source
не уничтожает healthy predecessor; reserved/unknown result требует прежнего reap.
Source10Go/4docs/gofmt/manual diff/SHA
only; tests/fixtures/build/product/native/network не выполнялись. Scope/API/legacy
fixtures/queue/flaps/actor/retry/power/fault/cleanup/runtime/race/OS/resources OPEN.
Далее remaining queued worker/native writers; full PC/old9/SDK/W3b/interprocess/
durability/UX/release/comparative OPEN, native=false/IPC9/Job8/TUN unavailable/
schema2 unsupported прежние.
V2-228 [native TUN I/O](../../tasks/TH-01-TUN-LIFECYCLE.md#pc-v228-native-reader):
single original done-published helper exchange, fresh goroutine/write/decoder
source gates и passive intent/operation/dependency wake. No replacement unfinished
I/O; completed original cause присоединяется после existing retirement, без новой
health attribution. Actor хранит original bounded ctx; descriptor/confirm/consuming
probe сохраняют bound sources, deadline не продлевается. Rechecks перед native
duplication и scope-open после ожиданий; original partial target-handle custody и
independent cleanup прежние. helperScope/panic preserve earlier cause + late cancel.
Source7Go/4docs/gofmt/manual diff/SHA only, без tests/fixtures/build/product/native/
network. API/legacy fixtures/generic ctx/async publication/late cause/transfer/
cleanup/runtime/race/resources/OS OPEN; no atomic kernel/I/O preemption claim.
Далее common helper reader/native DNS и remaining native writers; full PC/old9/
SDK/W3b/interprocess/durability/UX/release/comparative OPEN. Native=false/IPC9/Job8/
TUN unavailable/schema2 unsupported прежние.
V2-229 [helper reader](../../tasks/TH-01-NATIVE-HELPER.md#pc-v229-helper-reader):
reuse original done-published header между common helper/TUN; one process/ioDone
owner. Fixed maxOutput+1 allocation before child, stdlib ReadFull с pure source
checks around each Read; EOF/overflow/no-progress semantics, passive source wake
для read+exit. Late original cause переживает caller cancellation и independent
retirement attempt; bytes до actual gates не выдаются, unknown не receipt.
Typed earlier guard/image/panic/cancel сохраняются. Native DNS scope до decode,
parse/after-scope joined без health promotion. Source5Go/4docs/gofmt/manual diff/
SHA only; tests/fixtures/build/product/native/network не выполнялись. API/fixtures/
EOF/empty/exact/overflow/source/late cause/keepImage/cleanup/runtime/race/resources/
OS OPEN, no total RSS/allocator/syscall/kernel/latency proof. Далее updater GUI
exchange и remaining native writers; full PC/old9/SDK/W3b/interprocess/durability/
UX/release/comparative OPEN, native=false/IPC9/Job8/TUN unavailable/schema2 unsupported.
V2-230 [GUI exchange](../../tasks/TH-01-UPDATE-PAIR.md#pc-v230-gui-source):
fixed owned frames и shared done/source-gated I/O; late goroutine не пишет в
caller receive, no unfinished replacement. Shared reader/writer reused helper/
TUN/worker/GUI, worker panic finite. Precommit original child/reaper/late cause;
ctx перед каждым native handle duplication, source/exit rechecks после waits и
до custody commit. Session/child gates сохраняют actual negative image-close
fact и проверяют ctx после native work; committed lifetime/OWN≠Ready прежние.
Unbound bootstrap не получает fake App/path identity; root token/SDK independent.
Source7Go/4docs/gofmt/manual diff/SHA only, без tests/fixtures/build/product/native/
network. API/legacy shapes/alias/source/partial I/O/panic/transfer/commit/child/
runtime/race/resources/OS OPEN, no atomic syscall/readiness/performance proof.
Далее root restart/update intent-source registration и worker late-fault paths;
full PC/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN,
native=false/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.
V2-231 [intent registration](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v231-intent-source):
private id+original source в Stop/restart/update/activation/recovery. Initial0
maintenance source не forwarding grant; positive current и negative same identity
разделены, scalar internal teardown не разрешает manual Release. Bound update
ctx uses existing slot/actual path/epoch, no Background revival/fake proof;
Apply preflight/journal/source guards, fresh post-Stop pair/legacy context и
cleanup до token unlock. Actual negative prefix/image/custody facts сохранены.
Source11Go/4docs/gofmt/manual diff/SHA only, без tests/fixtures/build/product/
native/network. API/fixtures/0-wrap/Stop/ctx/retention/recovery/native Release/
launch/async failure/runtime/race/resources/OS OPEN, no external-action atomicity
or readiness/performance proof. Далее worker late-fault/native authorization gaps;
full PC/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN,
native=false/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.
V2-232 [worker result](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v232-worker-result):
normal worker result/done объединены existing ownedPipeIO, незавершённый pipe
owner не заменяется; finite protocol/codec/panic/version/ID/prepared-token cause
публикуется до original done и сохраняется через cancellation + bounded reap.
Process удерживает только completion header. Все operation-error retirement
callers сохраняют original completed cause; actual Stop cleanup/TUN retired receipt
отделены, no stale positive reply или исторический protocol fault→cleanup failure.
Source7Go/4docs/gofmt7+2+1 exit0/manual diff/byte-SHA only, без tests/fixtures/
build/product/native/network. Compile/race/codec/cancel/reap/OS/resources и late
probe flags/action-specific validation остаются OPEN. Далее correlated late facts
и native authorization gaps; full PC/old9/SDK/W3b/interprocess/durability/UX/
release/comparative OPEN, native=false/IPC9/Job8/TUN unavailable/schema2 unsupported.
V2-233 [late probe facts](../../tasks/PC-03-PROBE-INTENT.md#pc-v233-late-probe):
original captured probe epoch survives subsequent ctx/actor/native errors for
negative facts only; child emits bounded finite cleanup/trust probe_failed with
no tokens/health/current-state refresh. Parent validates original wire correlation,
subject/tag/epoch/shape/contradictions before I/O done; typed cause flows through
V232 reap and sanitized workerProbeFailure, trust reason allowlist reused.
Ordinary failure invents no cleanup/trust fault, undelivered facts stay unknown.
Source5Go/4docs/gofmt5 exit0/manual caller-type-lock-diff/byte-SHA only, без tests/
fixtures/build/product/native/network. Compile/IPC9/cancel/epoch/late frames/
classification/race/resources/OS OPEN. Other-action semantics/native Release/
launch atomicity/scalar async provenance и весь PC/old9/SDK/W3b/interprocess/
durability/UX/release/comparative OPEN; native=false/IPC9/Job8/TUN unavailable/
schema2 unsupported. Далее original-scope handling оставшихся command writes.
V2-234 [command replies](../../tasks/PC-03-WORKER-DNS-HANDOFF.md#pc-v234-command-replies):
shared pure original admit/TUN/DNS reply validation before I/O done, reused by
parent positive predicates; correct subject/tag/epoch/nonce/admitted phase and
empty token/report constraints. Expected empty failure carries no invented local
fault. Actual validated bootstrap DNS work alone creates private ID/action/actor/
epoch fault; child exact match emits bounded existing negative envelope, parent
preserves finite cleanup/trust through done/reap. Probe/control empty-failure
check reused. Source9Go/4docs/gofmt9 exit0/manual caller-type-lock-diff/byte-SHA
only, no tests/fixtures/build/product/native/network. Compile/IPC9/phase/negative/
foreign replies/cancel/late-frame/reap/race/resources/OS OPEN. Start/current/
switch/stop late semantics/native Release-launch atomicity/scalar async provenance
и весь PC/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN;
native=false/IPC9/Job8/TUN unavailable/schema2 unsupported. Далее remaining worker
startup/selector/stop reply ownership и native lifecycle authorization.
V2-235 [state replies](../../tasks/TH-01-PROCESS.md#pc-v235-state-replies):
hello/start/current/switch/stop now join pre-done known-command reply dispatch.
Parent-only immutable original process/next-ID/action/old subject/selector and
bounded prepared target snapshot, unbound/stale request rejected before pipes;
private field omitted from wire/allowlist. Exact old Previous/target digest/phase,
nonfatal unavailable/noop distinct; parent publishes prepared bytes, repeats
source/actor, unknown mutation reaped. Pure owned-JSON selector rewrite removes
certificate-file validation under Engine lock, W1/local generic fallback retained.
Stop remains actual reap-only. Source5Go/4docs/gofmt5+1+3 exit0/manual caller-type-
lock-normalize-diff/byte-SHA only; no tests/fixtures/build/product/native/network.
Compile/API/fixtures/IPC9/private JSON omission/sequence/alias/noop/unavailable/
rollback/cancel/publication/reap/race/resources/OS OPEN. Далее native Release/launch
authorization и scalar async provenance; full PC/old9/SDK/W3b/interprocess/
durability/UX/release/comparative OPEN, native=false/IPC9/Job8/TUN unavailable/
schema2 unsupported. Протокол/source не объявляются runtime-ready.
V2-236 [native intent](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v236-native-intent):
private manual Disconnect release phase only after original cleanup; exact live
nonzero/current registration, immutable original policy, once outside App.mu under
existing operationMu through terminal Stop. Central4 registration producers reject
release/restart commit before mutation; Close deferred/transferred, finite native
error/panic/unfinished custody, denied positive Release retains protection. Full
initial startup registration prevents late DOM/normal auto-connect from adopting
Disconnect; DOM source/renderer/lifetime guards, App observers separate from resume.
Source9Go/4docs/gofmt7+2+2+2 exit0/manual caller-type-lock-defer-diff/byte-SHA only,
no tests/fixtures/build/product/native/network. Compile/API/fixtures/0-wrap/commit/
Close/DOM/retry/panic/Goexit/race/resources/OS OPEN. App ordering is not atomic
native/WFP/interprocess proof. Next owned launch/ctx-path revocation/scalar async
provenance; full PC/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN,
native=false/IPC9/Job8/TUN unavailable/schema2 unsupported. FR009 controller/service
не объявляются реализованными этим token.
V2-237 [owned GUI](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v237-owned-gui):
original process preparation/opaque App authorization/final wire receipt separate.
GUI6 USE6/ACK6 gates child parent-exit/Wails path after source-approved grant;
local-only OWN5 decision rejected during review because parent crash bypassed it.
Matching one-use receipt finalizes actual custody even alongside late error;
before-transfer failure reaps original Job/I/O, after-transfer retained to parent
exit. All3 pair/recovery/headless callers and Close/GUITransferred integrated.
Existing signed SDK/image/handles/DOM gates preserved, extra bounded36-byte
exchange, no GUI Ready/forwarding authority. Source8Go/4docs/gofmt7+1+7+2+4exit0/
manual type-caller-lock-frame-receipt-diff/byte-SHA only; no tests/fixtures/build/
product/native/network. Compile/API/GUI5→6/EOF/crash/partial/source/replay/receipt/
parent wait/SDK/DOM/reap/race/resources/OS OPEN. Next legacy exec/UAC/single-image
owned launch, native OS/source authorization; full PC/old9/SDK/W3b/interprocess/
durability/UX/release/comparative OPEN; native=false/IPC9/Job8/GUI6/TUN unavailable/
schema2 unsupported. Signed pair checks не заменяются self-hash legacy launch.
V2-238 [mode startup](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v238-mode-startup):
TUN unavailable проверяется до Stop/UAC; mode restart сохраняет original committed
registration/path/lifetime через existing maintenance operation slot и повторные
pre-OS value guards. Exact private args, стандартный Windows EscapeArg вместо
ручного escaping. Await-previous timeout/native/ACL/Close uncertainty теперь
terminal exit3 до App/storage/Wails, original uncertain probe handle retained;
fresh mutex probe не приравнивается к authenticated parent exit/child readiness.
Source4Go/4docs/gofmt4+1exit0/manual4 original comparisons/types/callers/locks/
startup order/byte-SHA only, local Go codec source read without execution.
Тесты/fixtures/build/product/native/network не выполнялись. Compile/API/quoting
fixture migration/timeout/Close/race/OS/elevation/de-elevation остаются OPEN;
30s context не прерывает UAC/syscall. Полная миграция legacy exec/UAC/single-EXE
к original image/process/Job/child protocol, scalar async provenance и весь
PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative остаются в плане.
Native=false/IPC9/Job8/GUI6/TUN unavailable/schema2 unsupported; no runtime PASS.
V2-239 [ordinary restart](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v239-restart-custody):
R1 fixed private purpose/nonce/stages переиспользует GUI6 custody без подписной
authority: original suspended process/Job/I/O/image, App grant и matching receipt,
child authorization до parent wait, image close перед normal startup gates.
App сохраняет original owner до confirmed Close или completed parent exit;
Start/Stop/power barriers его учитывают, cleanup_unconfirmed отличим от обычного
restart_failed. Bare exec mode fallback удалён; UAC request и single-EXE update
не объявлены этим owned/accepted. Shared child bootstrap сохраняет GUI6 stages/
actual SDK/DOM и frozen buffers52/36, late local error после transfer требует
parent exit, не повторного admission. Source14Go/4docs/gofmt12+4+1+4exit0/manual
source/types/callers/locks/frames/defer/diff/byte-SHA only, без tests/fixtures/
build/product/native/network. Compile/API/R1↔GUI6/each-stage crash/EOF/replay/
parent wait/Close/SDK/DOM/race/resources/OS/packaging OPEN. UAC original handles/
bridge, signed single-EXE fresh child proof, de-elevation/scalar async и полный
PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative в плане.
Native=false/IPC9/Job8/GUI6/R1 source/TUN unavailable/schema2 unsupported.
V2-240 [UAC owner](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v240-uac-owner):
original ShellExecuteExW hProcess/Job/image/token вместо ShellExecuteW success
без handle; same-user elevated token, original path, no PID reopen/self/unknown
enclosing Job, fixed args и target query Job lifetime duplicate. Pure App owner
до native acquisition, COM/thread/S_FALSE/pinning/layout по локальному SDK/xsys.
Process-request receipt отделён от child ACK и CAS-сериализован с Close;
ошибки сохраняют original/uncertainty, helper barriers его учитывают. Running
pre-attachment gap остаётся witness после root exit/empty assigned Job.
Private child bridge отсутствует: отдельный compiled UACbridge=false запрещает
запуск до Stop и в core даже после будущего W4c acceptance, native=false прежний.
Source8Go/4docs/gofmt8+1+1+1+2+1exit0/manual new+original/native-header/type/lock/
defer/CAS/byte-SHA only; no tests/fixtures/build/product/native/network.
Compile/ABI/COM/rights/Job/SID/remote duplicate/Close/pre-attach/crash/UAC child
bridge/OS/packaging и signed single-EXE/de-elevation/scalar async/full PC00…09/
old9/SDK/W3b/interprocess/durability/UX/release/comparative остаются OPEN.
Нет whole-tree/GUI readiness/OS atomicity/performance PASS; schema2 unsupported.
V2-241 [U1 private bridge](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v241-uac-bridge):
local nonce/DACL/first-instance pipe и bidirectional original-handle peer checks,
fixed52/36-byte U1 frames/shared bootstrap, App grant перед final USE/ACK и
matching completed receipt. Child до USE не переходит к parent wait/Wails;
actual original parent wait/normal startup остаются отдельными gates. Checked
NOWAIT transport, original Close retention, ACK/server EOF race рассмотрен в
source; go-winio Close suppresses native errors, dependency не добавлялась.
Original operation/deadline в Commit нельзя заменить свежим ctx; late wire fact
не превращается в разрешение повторного запуска. Source12Go/4docs/gofmt12+4+1+1
exit0/manual new+original/type-caller-lock-CAS-frame-EOF/byte-SHA only. Тесты/
fixtures/build/product/native/network не выполнялись. Pre-main/init/pre-Job
witness и native pipe/ACL/rights/EOF/crash/cleanup/OS/packaging acceptance OPEN;
bridgeAccepted=false/native=false, source implementation не включает UAC.
Next minimal bootstrap/init contract, signed single-EXE/fresh child authority,
de-elevation/scalar async/full PC00…09/old9/SDK/W3b/interprocess/durability/UX/
release/comparative retained. GUI6/R1/IPC9/Job8/schema2 unsupported, no runtime PASS.
V2-242 [pre-main/bootstrap](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v242-uac-bootstrap):
отдельный compiled bootstrapAccepted=false включён в existing shared admission,
bridgeAccepted=false/native=false сохранены. Приёмка U1 сама по себе не включает
UAC до закрытия original pre-Job custody. Pinned Wails production/windows chain
выполняет native init до main; dispatcher не аттестует loader/все dependencies.
Существующий pinned helper — reuse-кандидат с отдельными import/loader/role/
parent-image/remote-handle/pair compatibility gates; меньший helper не доказывает
отсутствие потомков до Job. Source2Go/4docs/gofmt2exit0/manual calls/diffs/bytes,
без tests/build/product/native/network;6 originals/task71 rev496 сохранены.
Следующее — минимальный executable bootstrap seam с исходным owner; bootstrap
implementation/rights/descendants/Close/OS/packaging и полный PC00…09/old9 OPEN.
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported; runtime PASS отсутствует.
V2-243 [minimal childbootstrap](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v243-child-bootstrap):
вынесены existing fixed stages/36-52-byte frames/exchange shapes и child sequence
в stdlib-only package, без core/Wails/sing-box/native dependencies. GUI6/R1/U1
используют thin core adapter и прежние original owners/native/image/Job/parent/
transport/SDK callbacks; original contextError scope проверяется вокруг шагов.
Wire/args/version и USE→ACK→Close→original-parent wait→Verify порядок сохранены.
Нет нового helper mode/resource owner/подписной authority; bootstrap/bridge/
native=false и pre-Job witness сохранены. Source5Go/4docs/gofmt5+2+1exit0/manual2
new+3 original diffs/callers/order/bytes only,9 originals/task71 rev500 archived.
Без tests/fixtures/build/product/native/network. Далее minimal native parent/Job
lease seam и concrete pinned-helper bootstrap; API/ABI/versions/partial/EOF/
crash/Close/OS/packaging и full PC00…09/old9 OPEN. TUN unavailable/schema2
unsupported; source reuse не является runtime safety/performance PASS.
V2-244 [native child lease](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v244-native-lease):
existing query Job/original-parent custody вынесена в separate nativelease
subpackage с pinned xsys, без core/Wails/sing-box. Базовый sequencer stdlib-only.
Original session/common adapter сохраняет callbacks/contextError/error causes;
one-attempt Adopt/Wait, TryLock, retained claimed/original handles, sticky native/
callback/Goexit/Close failure. U1 peer/image/SID/token checks получают parent
через locked callback, без raw getter/конкурирующего Close. Прежние Job flags/
membership/noninherit/nonself и15s/100ms wait сохранены; access-mask proof OPEN.
Source8Go/4docs/gofmt8+1exit0/manual4 new+4 original diffs/callers/locks/state/
bytes only,12 originals/task71 rev504 archived; tests/build/native не запускались.
Далее image/pipe seam и concrete pinned-helper bootstrap; API/ABI/rights/partial/
concurrency/Close/crash/resources/OS/packaging/full PC00…09/old9 OPEN. Все три
gates=false, ranBeforeJob retained, TUN unavailable/schema2 unsupported.
V2-245 [isolated controlpipe](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v245-control-pipe):
existing U1 transport вынесен в minimal subpackage: same local nonce/DACL/SQOS/
NOWAIT/peer/fixed frames, original pipe+SID-text/SDDL allocations и checked
Close/Free. Pinned xsys helpers скрывают LocalFree result; direct output slots
сохраняют partial owner. TryLock/atomic Retire запрещают positive reuse при
busy/cleanup uncertainty; ACK/EOF race сохранён без новых metadata/peer queries.
Common value CheckScope reused; core parent-image/SID/App authority и tokenMu/
one-shot token Close остаются. Source10Go/4docs/gofmt10+3+2exit0/manual5 new+
5 original diffs/types/callers/locks/atomics/bytes;14 originals/task71 rev508
archived. Без tests/build/native/network. ABI/DACL/SID/SQOS/alloc/Free/Close/
Retire/cancel/Goexit/OS/resources/packaging OPEN. Далее image seam/concrete
pinned-helper role; all gates=false/ranBeforeJob/full PC00…09/old9 сохранены.
V2-246 [image file slots](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v246-image-slots):
CreateFile result теперь захватывается original slot до обработки ошибки,
begin/finish witness покрывает5 image/installer creation sites. Общий one-shot
Close не повторяет unknown handle; validation после uncertainty запрещена.
Existing directory rebind сохраняет known-close0/private FileID и root lock.
Fixed PE projection и borrowed32KiB ReadAt выделены в minimal imagecheck;
core mutex/error/Scope/32MiB-helper/1GiB-app limits сохранены. Source10Go/4docs/
gofmt10exit0/manual3 new+7 original diffs/callers/bounds/rebind/bytes;14 originals/
task71 rev512 archived, без tests/build/native/network. Полный image/path/ACL/
WFP owner ещё core-bound, hidden LocalFree в ACL helper OPEN. Далее этот seam
и concrete helper; read-Close serialization/API/ABI/partial/rebind/Goexit/OS/
resources/packaging/full PC00…09/old9 OPEN, все gates=false/ranBeforeJob retained.
V2-247 [owned ACL descriptors](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v247-owned-acl):
raw GetSecurityInfo/checked LocalFree в per-file Slot для image/read/write/
journal/workspace/floor; original pointer до native call, partial/unknown/free
witness и retired/busy guard. ACL cleanup до original file/root release, fresh
query только после confirmed Free; rebind снимает old slot только known-clean.
Legacy API сохраняется через fixed1 slot без maps/queues; core trust использует
minimal leaf. Existing SID/allow-ACE/selected-input-output policy сохранена,
добавлены bounded relative SD/ACL/ACE/SID spans и typed cleanup causes.
Source12Go/4docs/gofmt12+3exit0/manual3 new+9 original diffs/callers/Free/bytes,
16 originals/task71 rev516 archived. Tests/build/native/network не запускались.
ABI/ACL corpus/Free/retire/Goexit/OS/resources/packaging и полный image/path/
WFP/identity-SID/concrete helper/full PC00…09/old9 OPEN; gates=false/ranBeforeJob retained.
V2-248 [shared image/path inspection](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v248-image-inspection):
fixed-local component/ancestor plan, original file path/FileID/ACL/NTFS и process
path query используют existing minimal imagecheck. Core сохраняет original
image/installer record, Close/rebind/private-ID/root-lock и WFP AppID. Metadata
пишется в original snapshot, typed ACL cleanup не теряется; compatibility trust
API сохранён. Process source fence до/после query; pseudo-self допустим, PID не
переоткрывается. Source5Go/4docs/gofmt5+3exit0/manual2 new+3 original diffs,
9 originals/task71 rev520 archived; no tests/build/native/network. Full reusable
file/identity owner, concrete helper, ABI/path corpus/scope/races/OS/resources/
packaging и full PC00…09/old9 OPEN. Gates=false/ranBeforeJob retained.
V2-249 [original file+ACL owner](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v249-file-owner):
filelease.Lease содержит private native handle/ACL/IO_STATUS_BLOCK и acquisition/
Close witnesses; original core record хранит pointer до acquire. Все5 Open/NtCreate
sites и image/verifier/recovery/pair/installer consumers подключены. Checked ACL
cleanup/one-shot Close/unknown retention и known-close0 directory rebind сохранены;
borrowed HandleValue не передаёт владение и требует прежней caller serialization.
Path/private FileID/root-lock/WFP/SID trust остаются у core. Source12Go/4docs/
gofmt12+1exit0/manual2 new+10 original diffs,16 originals/task71 rev524 archived.
No tests/build/native/network; ABI/private-type/partial/races/OS/resources/packaging,
SID factories/concrete bootstrap/full PC00…09/old9 OPEN. Gates=false/ranBeforeJob retained.
V2-250 [bounded local identity/trust](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v250-file-identity):
shared fileacl CurrentUser/Trust заменяют5 duplicated SID factories; fixed aligned
Go128-byte TOKEN_USER query/typed header/bounded Go68-byte SID copy, fixed installer
encoding без StringToSid/native allocation/hidden Free. Existing primary-token
selector/principal set/selected-I-O bypass/domain errors сохранены; zero Trust
rejects. Controlpipe использует общий user SID, прежние native owners сохранены.
Source9Go/4docs/gofmt9+1exit0/manual2 new+7 original diffs,13 originals/task71 rev528
archived. No tests/build/native/network; ABI/SID/private-type/GC-lifetime/OS/resources/
packaging/concrete image-helper bootstrap/full PC00…09/old9 OPEN. Gates=false/ranBeforeJob retained.
V2-251 [concrete minimal image pair](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v251-image-pair):
actual helper/installed-pair image custody переведена на imagelease.Pair с fixed66
slots, scoped32KiB hash/PE reads, shared ACL/NTFS/FileID trust, TryLock/retire и
checked reverse Close. WFP/assigned-host/source/role/caps/Job lifetime — core;
WithPaths удерживает original files через WFP/process callbacks. Mutable installer/
verifier/recovery records сохранены, SameSnapshot policy общая. Source8Go/4docs/
gofmt8+4+4+2exit0/manual4 new+4 original diffs,12 originals/task71 rev532 archived.
No tests/build/native/network. ABI/private-type/retire/Goexit/caller-Job/OS/resources/
packaging/fixed-role executable bootstrap/full PC00…09/old9 OPEN. Gates=false/ranBeforeJob retained.
V2-252 [published image custody/UAC migration](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v252-published-custody):
2 stale trust.user consumers исправлены exact-user matcher/bounded borrowed-token
SID query. Pure image allocation отделена от Prepare; helper/TUN/delivery retain
до acquire. RetainInstalledPair/RetainRecoveryImage заполняют original caller slot,
nested image owner и facade/startup/acceptance/recovery consumers migrated; GUI
session удерживается до adoption. Source21Go/4docs/gofmt21+1exit0/manual21 diffs,
25 originals/task71 rev536 archived. No tests/build/native/network;56 pending
checks exact, ничего не waived. Outer factory/Goexit/API/SID/ABI/OS/resources/
packaging/executable bootstrap/full PC00…09/old9 OPEN. Gates=false/ranBeforeJob retained.
V2-253 [original process and outer owners](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v253-process-publication):
pure process allocation и caller publication предшествуют one-attempt native
spawn во всех найденных production consumers. Engine не заменяет existing owner,
резервирует instance до spawn и использует operation ctx (behavior unverified).
RetainUpdateCodeAcceptance/RetainInstallationRoot сохраняют App и nested native
root до подготовки; legacy factories compatibility only. Source15Go/4docs/
gofmt15exit0/manual15 diffs,19 originals/task71 rev540 archived;56 pending checks
exact/no waiver. No tests/build/native/network. Whole-tree/Goexit/context/Close/
ABI/OS/resources/packaging/executable bootstrap/full PC00…09/old9 OPEN.
All gates=false/ranBeforeJob retained; GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.
V2-254 [original spawner native retirement](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v254-native-retirement):
one-attempt CloseHandle receipts для original thread/process/Job и3 inherited
handles; unknown result удерживает numeric slot и оставшиеся native resources.
LocalFree/Unpin attempts sticky; незавершённый one-use attribute initialize
блокирует cleanup. Actual implementation перенесён в minimal spawnprep,
core использует one-use WithStartup, interrupted borrow запрещает release.
Source7Go/4docs/gofmt3+7exit0/manual3new+4original diffs,11 snapshots (8 originals/3 absent-new)/task71
rev544 archived;56 pending checks exact/no run/no waiver. No tests/build/native/
network. Это необходимая правка перед reusable spawner/fixed-role helper,
не реализация executable bootstrap или native acceptance. Full PC00…09/old9/
ABI/Goexit/Close/OS/packaging OPEN; gates=false/ranBeforeJob retained.

V2-255 [minimal original process custody](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v255-native-process):
actual core native process/Job/accounting uses spawnprep.Process; handles private,
trusted synchronous create/borrow callbacks и reserved single physical Wait.
Перед native Close повторяются root exit/Job empty; unknown operation/borrow/close
удерживает owner. Actual image/GUI/TUN/resource consumers migrated; UAC running
request остаётся отдельным, policy/ranBeforeJob сохранены.2 stale kernel32 refs
исправлены. Source12Go/4docs/gofmt12+1exit0/manual2new+10original diffs;
16 snapshots/task71 rev549 archived,56 checks exact/no run/no waiver.
No tests/build/native/network. Helper executable/I-O/policy seam/ABI/Wait/Close/
OS/packaging/full PC00…09/old9 OPEN; all gates=false/ranBeforeJob retained.

V2-256 [original Pipes/File and Job limits](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v256-original-pipes):
actual spawner использует shared original pipe/stdio/attribute preparation и
parent endpoints, опубликованные до native work. One-use acquire/retire/Close
witnesses удерживают unknown owner; late acquire не разрешает новый I/O.
Native Job limits/projection/verifier predicates общие, fixed roles/caps прежние.
Source11Go/4docs/gofmt11+1exit0/manual4new+7original diffs,15 snapshots/task71
rev553 archived;56 pending checks exact/no run/no waiver. No tests/build/native/
network. Actual helper dispatch/roles-pair/init-loader/creation-time/ABI/I-O/
Close/OS/packaging/full PC00…09/old9 OPEN; all gates=false/ranBeforeJob retained.

V2-257 [closed argv/env consumers](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v257-fixed-invocation):
production GUI/UAC и current DNS/TUN helper используют общие fixed argv contracts;
spawner — bounded system/public-store/GUI environment builder. Decode DNS остаётся
после Job/pipe precondition; purpose/binding не authority. U1 same-App parent
predicate не ослаблен. Новая App-helper-GUI broker role требует отдельного
purpose/ancestor-Job ownership protocol, ещё не реализована.
Source11Go/4docs/gofmt11+1+1exit0/manual3new+8original diffs,15 snapshots/task71
rev557 archived;56 checks exact/no run/no waiver. No tests/build/native/network.
Full PC00…09/old9/argv/env/ABI/OS/packaging/creation-time OPEN; gates=false/ranBeforeJob retained.

V2-258 [creation-time containment source](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v258-creation-job):
production CreateProcess получает original Job в JOB_LIST; one-attempt
IsProcessInJob заменяет late Assign. Resume принимает только count1, unknown
membership cleanup требует root exit AND original Job empty. GUI remote handles
передаются после Resume перед seed с повторной image/source проверкой.
Source6Go/4docs/gofmt6+1exit0/manual6diffs/callers/reaper/local SDK source;
10 exact snapshots/task71 rev561 archived,56 checks exact/no run/no waiver.
OS/ABI/nested Job/breakaway/creation guarantees/EOF/loader/partial OPEN;
UAC Shell pre-Job interval и новая broker chain не решены. Full PC00…09/old9
сохранён; gates=false/ranBeforeJob, GUI6/R1/U1/IPC9/Job8/TUN/schema2 прежние.

V2-259 [broker contract and reserved flags](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v259-broker-wire):
distinct control/GUI protocols, canonical argv, seeds3/4 original target handles;
separate helper resource Job and ancestor lifetime Job. Retained one-attempt
coordinator records actual adopt/prepare/commit/ACK/close facts. GUI reuses child
sequence with both-origin await callback. Legacy U1 grammar/predicate unchanged.
Actual entrypoints reject both broker namespaces until native handler exists;
wire/sequencers НЕ подключены к native execution. Source6Go/4docs/gofmt6+2exit0,
manual3new+3original diffs/callers,10 snapshots/task71 rev565 archived;56checks
exact/no run/no waiver. Full PC00…09/old9/ABI/partial/OS/packaging OPEN;
gates=false/ranBeforeJob, GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

V2-260 [typed broker transport](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v260-broker-transport):
distinct broker control/GUI pipe names and60/68 grammar reuse existing original
Win32 Pipe. Typed immutable protocol/nonce binding; one-attempt client sequencer
adapters/under-mutex I-O mode latch, no manual Exchange mixing or reconnect.
Legacy U1 name/52 grammar and peer/ACL/Close/EOF rules retained; internal buffer68.
Source5Go/4docs/gofmt5+3+3exit0/manual1new+4original diffs/callers/locks/EOF;
9 snapshots/task71 rev569 archived;56 checks exact/no run/no waiver.
Native chain/Jobs/fixed pair/SID/spawn/dispatch not integrated; new roles remain
rejected. Full PC00…09/old9/OS/resource/packaging OPEN; all gates=false/ranBeforeJob.

V2-261 [native broker GUI custody](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v261-broker-custody):
closed3-role Job policy, retained native3/4-handle lease, same primary SID/
elevation/peer/file-pair path checks, both-original WaitAll and checked Close.
Actual App dispatcher/native GUI session connected behind old gates plus
chainAccepted=false; parent/helper launch not implemented. Initial helper128MiB/
20CPU sec limits unmeasured. Source9Go/4docs/gofmt9+2+1+2exit0/manual6new+
3original diffs/callers/locks/Wait,13 snapshots/task71 rev573 archived;56checks
exact/no run/no waiver. Before chain acceptance: versioned R1/GUI6 ancestor
reference propagation, then actual parent/helper controllers/spawn/ACK receipts.
Full PC00…09/old9/native/OS/ABI/packaging/UX/release OPEN; all gates=false/ranBeforeJob.

V2-262 [versioned ancestor relaunch](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v262-ancestor-relaunch):
GUI7/R2 transfers own GUI Job/parent/same ancestor in60 bytes. Original source
registered before adoption and captured privately; effective child ancestry+
departure from old role Job checked before Resume, query duplicate after Resume.
Actual parent/child factories/stdio/native lease/dispatch connected; one parent
Wait/checked Close and fresh paths/SID/elevation checks, no growing root layers.
Legacy52-byte GUI6/R1/U1 preserved; new acceptance flag=false. Returned canceled
borrow no longer destroys a confirmed permanent source, uncertain native/Close
still restrictive. Source18Go/4docs/gofmt18+6exit0/manual4new+14original diffs,
22 snapshots/task71 rev577 archived;56 checks exact/no run/no waiver.
Next actual parent/helper broker controllers/spawn/ACK/exit. Full PC00…09/old9/
native rights/breakaway/install-rename/ABI/de-elevation/release remain OPEN.

V2-263 [actual App-parent broker owner](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v263-broker-parent):
App captures compiled helper pin before image/COM/native acquisition and reuses
existing private phase/grant/receipt. Original Shell hProcess plus separate
ancestor/helper Jobs; retained assignment/duplicate/Wait/termination/Close facts,
query-only target handles, exact primary SID/image/peer/resource checks.
Distinct pinned control-v2 argv5/stages/namespace preserves old wires. After ACK/
server EOF require actual helper exit zero/empty helper Job/waiting GUI in ancestor;
failure retires both original Jobs and known helper without erasing ranBeforeJob.
Shared Shell timing and one-attempt Close/token acquisition corrections included.
All gates remain false; helper native controller still absent/terminally rejected.
Source10Go/4docs/gofmt10+5+3exit0/manual3new+7original diffs,14 snapshots/task71
rev581 archive;56 checks NOT_RUN exact/no waiver. Next compose actual minimal
helper using existing Process/Pipes/BrokerLease/coordinator, not more extraction.
Full PC00…09/old9/OS/rights/nested hierarchy/SDK/de-elevation/release remain OPEN.

V2-264 [helper controller and actual suspended GUI](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v264-helper-controller):
helper dispatcher now routes only pinned control-v2 to an original native
session/coordinator behind brokerHelperAccepted=false. Held App/SID/Job/peer
authentication precedes pin use/Pair preparation. Fixed GUI reuses original
Process/Pipes/creation-time Job list; same ancestor/outside helper resource Job
checked before Resume; retained4 query/sync target slots, exact GUI ARM/OWN/ACK,
GUI EOF/helper ACK/App EOF/terminal helper exit. Detached NUL stdio adds no I-O
pumps. Bounded native known-folder owner/clean GUI environment and shared fixed
App filename alias preserve package boundaries. Local child retirement does not
grant ancestor-tree cleanup; original App retains root control/ranBeforeJob.
Source9Go/4docs/gofmt9+1+1exit0/manual4new+5original diffs,13 snapshots/task71
rev585 archive;100 OPEN/56 NOT_RUN exact/no waiver. All acceptance flags=false.
Next source integration across parent/helper/GUI/relaunch and PC-01 traffic
admission/teardown; root query-reference lifetime and physical native/OS/rights/
profile continuity/packaging/SDK/full PC00…09/old9/release gates stay OPEN.

V2-265 [retained final pipe effects](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v265-final-io-facts):
original bounded raw I/O receipt prevents loss of a physically read ACK when
later peer/source validation fails. Partial counts and native pending/return
facts retained; original native Read/Write/EOF nonreturn blocks Close. Parent
failed final USE is cleanup-classified uncertainty; helper stores actual GUI ACK
without converting a failed exchange into accepted transfer/readiness. U1/Broker
adapters reuse existing Pipe; APIs/wires/positive grant/native checks preserved.
Source8Go/4docs/gofmt8+3exit0/manual1new7diff/12 snapshots/task71 rev589 archive;
100 OPEN/56 NOT_RUN exact/no waiver/all gatesfalse. Existing Manager final scalar
source/Stop/cleanup reused. Stdio final-effect retention/root query-reference
lifetime/explicit root control/whole-chain retirement/PC-01 cutover and full
PC00…09/old9/native/SDK/ABI/packaging/UX/release acceptance remain OPEN.

V2-266 [stdio GUI effect retention](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v266-stdio-effects):
original raw stdio counts/pending-return data and final App decision preserve
actual ACK across late cancellation; only atomic entered-write is read before
done. Original process retirement checks raw I/O return after goroutine ends;
managed pipe cancellation/physical wait/Job cleanup order retained. Child native
session owns four original exchange slots, no reset/retry/extra stage. Existing
reader/writer/header/transport reused; positive grant/source/image/phase unchanged.
Source6Go/4docs/gofmt4+2+1exit0/manual6diff/10 originals/task71 rev593 archive;
100 OPEN/56 NOT_RUN exact/no waiver/all gatesfalse. Failed possible USE cue is
distinct from unconfirmed cleanup. Next same owner-return facts for worker/helper/
TUN, then root lifetime/whole-chain admission/teardown. Full PC00…09/old9/native/
SDK/ABI/packaging/UX/release/comparative remain OPEN.

V2-267 [worker/helper/TUN/delivery original I/O](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v267-process-io-owners):
all five found production core ownedPipeIO families now attach original raw-call
facts before publication. Common exact header/channel/actual-return guard covers
exchange replacement and process retirement; helper/TUN/delivery completed results
propagate original cleanup uncertainty. Delivery adapter's separate ioDone-only
producer migrated with its original WriteCloser EOF/Close facts, same SDK/wire/
post-retirement policy; duplicate result channel removed, no new worker/transport.
Source6Go/4docs/gofmt5+2+1exit0/manual6diff/10 originals/task71 rev597 archive;
100 OPEN/56 NOT_RUN exact/no waiver/all gatesfalse. Next fresh original GUI/root
custody integration before positive lifecycle/admission, not control authority
from QUERY handles. Root lifetime/whole-chain cleanup/full PC00…09/old9/native/
SDK/ABI/packaging/UX/release/comparative remain OPEN.

V2-268 [GUI/root в process lifecycle](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v268-root-custody):
исходная bootstrap-ссылка сохранена в Windows process owner; fresh QUERY до
создания/перед и после Resume, membership обычного потомка и successor различаются.
Положительные worker-команды, включая оба admit, проверяют исходную custody до
обмена и после original done. Stop/dns_pause/tun_scope_close и reaper не закрыты
этим guard; faulted ancestor не заменяется отсутствующим. Source4Go/4docs/
gofmt4+2 exit0/manual4diff/8 originals/task71rev601;100 OPEN/56 NOT_RUN exact.
Далее helper/TUN positive actions/descriptor/result publication. Root control/
lifetime/whole-chain cleanup/native/full PC00…09/old9/SDK/UX/release OPEN.

V2-269 [helper/TUN/delivery custody](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v269-helper-custody):
original ancestor перед helper/delivery I/O, actor Descriptor и вокруг positive
TUN exchange; exact release/EOF и physical cleanup не требуют positive query.
После original retirement выдача результата требует root-source-only query;
delivery SDK late-cancel restriction-prefix semantics сохранены независимым
bounded custody budget, без новой authority. Source3Go4docs/gofmt3exit0/manual3diff/
7 originals/task71rev605;100 OPEN/56 NOT_RUN exact. Далее typed local source fault
через existing retry/failover barrier, отдельно от pure cancellation. All gatesfalse.

V2-270 [typed GUI/root fault](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v270-root-fault):
structural ancestor failure — existing local retry/failover barrier даже после
successful worker cleanup; worker/TUN/native/terminal reports сохраняют restriction.
Pure cancellation отдельно от mixed native fault; private error walk32 slots/64
visits, unknown/cycle/overflow fail local. Source6Go4docs/gofmt4+2/manual6diff/
10 originals/task71rev609/initial8+extension archived;100 OPEN/56 NOT_RUN exact.
Далее retained root в GUI stdio/final USE/ACK/private transfer без потери original
effect facts/irreversible acceptance. All gatesfalse; full PC/old9/native OPEN.

V2-271 [GUI final custody](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v271-gui-custody):
original root в parent stdio pre/post original done, final private accept/transfer;
child adopted exchange queries с сохранением ACK→Close→AwaitParent semantics.
Original effect/ACK/uncertain facts и accepted transfer не стираются ошибкой.
Source3Go4docs/gofmt3/manual3diff/7 originals/task71rev614;100 OPEN/56 NOT_RUN exact.
Далее root source при successful cleanup barrier перед Release/new work: повторный
Stop не должен забывать faulted retained root. Negative cleanup/root control/
whole-chain retirement/admission/native/full PC/old9 остаются отдельными.

V2-272 [retained root cleanup barrier](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v272-root-barrier):
successful core helper cleanup повторно проверяет original root после всех3
negative lane reaps/unlocks; failed root остаётся local fault перед Release/new work.
Shared private query wrapper, прежний independent budget/typed cancellation policy,
no-root без timer/query; никакого fake owner/root-empty/cleanup receipt. Source2Go/
4docs/gofmt2/manual2diff/6 originals/task71rev618;100 OPEN/56 NOT_RUN exact.
Далее original root creator/control/QUERY lifetime/terminal exit; whole-chain
physical retirement/admission/native/full PC/old9/release OPEN. All gatesfalse.

V2-273 [root control / App GUI cleanup](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v273-root-control):
оба untransferred GUI owners получают negative cleanup даже после первой ошибки;
original errors/pointers сохранены до successful Close, accepted owners не abort.
Source matrix разделяет initial App root control и helper/GUI QUERY-to-OS-exit.
Singleton root predicate отвергнут: GUI/Wails/WebView2 descendants ещё не учтены;
две пробные query правки byte-exact restored, gate/API не добавлены. Source1Go/
4docs/manual1finaldiff/gofmt initial3+final1exit0/7 originals/task71rev622;
100 OPEN/56 NOT_RUN exact. Далее actual GUI/renderer descendant ownership и
terminal teardown; root physical retirement/native/full PC/old9/release OPEN.

V2-274 [Close/terminal Quit и WebView2](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v274-renderer):
repeated window Close ждёт original cleanup; отдельный mu-guarded quitRequested
разрешает terminal callback только из requestQuit после прежней задержки.
Pending/accepted handoff и Stop-error terminal exit сохранены; один Quit/fallback.
Local pinned Wails/WebView2 trace не даёт original browser Process/Job/exit proof;
singleton/control/egress gates не добавлены. Source1Go4docs/gofmt1/manual1diff/
5 originals/task71rev627 +10 dependency source SHA;100 OPEN/56 NOT_RUN exact.
Далее async startup/closing reset/resource owner against terminal close, затем
browser supervision/terminal tree/egress/native/full PC/old9/release OPEN.

V2-275 [startup/Close и original log writer](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v275-startup):
synchronous startup и observer publication используют existing operationMu;
closing не reset, initial baseline не заменяет queued Start/Stop, auto-connect
после unlock. One-attempt original update done; Start без live ctx не регистрируется.
Logger original ch/done capture, mu-serialized send/close/detach, actual done wait
outside lock/no replacement/no post-close second disk writer. Source1Go4docs/
gofmt1file2batches/manual1finaldiff/5 originals/task71rev631;100 OPEN/56 NOT_RUN exact.
Далее actual tailer/bgprobe/tray task return и tray Exit routing; original flush/
durability/browser/root whole-chain/native/full PC/old9/release OPEN. All gatesfalse.

V2-276 [producer joins / tray Exit](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v276-producers):
retained tailer/bg original done channels, App-bound BG cancellation/admission/dial,
reuse existing task join with2s wait/final original re-read; pending owner не erased.
Ordinary Close/mode restart record join errors and still honor terminal/accepted
exit. Tray Exit uses App close, not systray-only exit. Source4Go4docs/gofmt4+2/
manual4diff/8 originals/task71rev635 +2 dependency SHA;100 OPEN/56 NOT_RUN exact.
Далее original systray native/ready/actions/relays lifetime и remaining update/
partial-accepted Quit cleanup. No physical socket/file/root/native proof; full PC/
old9/release OPEN, all gatesfalse.

V2-277 [retained original tray](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v277-tray):
one App-bound process-global session до scheduling; dedicated native OS thread,
separate Run/ready/workers/aggregate facts. Cancel-aware UI tasks; ready-return
seals Add, workers return before guarded Quit; no pre-ready once burn/replacement.
All8 terminal Quit callers observe same stop/join before terminal permission;
pending warning never revokes accepted exit. Source2Go4docs/gofmt2/manual2diff/
6 originals/task71rev639 +3 source SHA;100 OPEN/56 NOT_RUN exact. Go task aggregate
не native HWND/PostMessage/Destroy proof. Далее all-terminal tailer/bg/logger
integration/flush bounds, actual native/browser/root/full PC/old9/release OPEN.

V2-278 [all-terminal producers/logger](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v278-terminal):
one quit reservation and independent delay+2s+3s fallback before cleanup, no
Flush/Close/Wails-return dependency or hard OS bound. All8 consumers cancel exact
net/DNS/tailer/bg/tray before shared2s joins; logger gets remaining budget.
Pending retains original owners and accepted exit. Closing plus tray publication
recheck reject late start. Extracted original logger: early memory replay/no disk
under App.mu, queue drops, checked rotation/Open/Write/Flush/final Sync/Close,
sticky failed append operation/no replacement. Source6Go4docs/gofmt6/manual6diff/
10 snapshots (9 existing+1 absent)/task71rev643;100 OPEN/56 NOT_RUN exact. No test/
compile/native/durability proof. Next tailer file ownership and pre-request Close/
Stop; root/browser/admission/full PC/old9/UX/release/comparative OPEN.

V2-279 [tailer file owner/budgets](../../tasks/TH-01-RESTART-HANDOFF.md#pc-v279-tailer):
existing tailer APIs extracted; original App state before task, I/O result only
after exact done. App writer/tailer share originalLogFile Open/in-flight I/O/
one-attempt Close, retain partial failures/no replacement. ReadSlice+LimitReader
bounds reads256KiB/pass,128 completed rows,64KiB buffer; counted oversized discard,
ordinary partial-row offset and rotation hint preserved. Early Close cancels
tailer/bg before net join/Stop; no pre-Stop forced exit/Release or hard wait proof.
Source5Go4docs/gofmt5/manual5diff/9 snapshots7existing2absent/task71rev647;100 OPEN/
56 NOT_RUN exact. Next main PC01 original-helper/WFP admission, root/browser/control;
ordinary Stop/native/OS/full PC/old9/SDK/UX/release/comparative remain OPEN.

V2-280 [PreMatch escape/family gate](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v280-prematch):
pinned SDK supportBypass=false still creates explicit native route destination
and default ICMP. Guarded adapter now returns zero destination for allowed TCP/
UDP without calling SDK PreMatch; actual SDK route still applies policy. ICMP/
unknown/empty PreMatch families unavailable pending owned protected implementation.
Typed TCP/packet family gates and nil packet rejection preserve guarded boundary.
Source2Go4docs/gofmt2/manual2diff/6 originals/task71rev650 +4 local source SHA;
100 OPEN/56 NOT_RUN exact. Next ordinary delegated-flow original lifetime/I/O/
callback/Close/revoke and root/browser/control. Native families/WFP/ordinary Stop/
full PC/old9/SDK/OS/UX/release/comparative remain OPEN, all gatesfalse.

V2-281 [ordinary TCP/UDP receipts](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v281-ingress):
All4 ordinary entry points claim original per-box route before SDK Ex delegation;
real SDK policy retained.4096 pending/failed+live receipts, success-only removal.
Tracked ingress raw I/O/Close, route/SDK/external callback and AfterFunc fences;
guards preserve handshake/early/deadline and UDP ownership without raw unwrap.
First local custody fault seals gate; no channel-health reclassification. Two-
phase close cancels before original Box.Close, drains afterwards under shared5s,
avoiding callback dependency cycle. Existing worker probe retains original-epoch
finite cleanup_unconfirmed across late failure/cancel/issuance; no IPC change.
No hard OS/whole-upstream proof.
Source4Go4docs/gofmt3+1+2+2/manual4diff/8 snapshots6existing2absent/task71rev652 +9
local source/version SHA;100 OPEN/56 NOT_RUN exact. Rejected-ingress/root alias/
opaque SDK retirement, compatibility/CPU/limits/races and full PC/old9 remain OPEN.

V2-282 [rejection custody](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v282-rejection):
all4 Router negative paths публикуют bounded original socket/callback receipt до
Close/callback. Known private wrapper aliases join original Close outcome, без
native retry; post-Box.Close rejection seal и shared5s drain не забывают late work.
Late/full rejection вызывает private original worker-only exit4, parent сохраняет
обязательный Process/Job/IO/handle reap; локальный harness не получает GUI Exit.
5Go4docs/gofmt5+5/manual5diff/9 snapshots8existing1absent/task71rev655 +10 local SHA.
Source/unverified; pre-router sockets/opaque aliases/native/limits/performance и
весь PC00…09/старые9 OPEN.100 OPEN/56 NOT_RUN/all gatesfalse; TUN unavailable.

V2-283 [pre-Router TCP custody](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v283-accept):
owned Listen/Accept task регистрирует original TCP socket до metadata/Go/SDK
handshake; reused unchanged mixed processor без duplicate bind. Separate4096
input receipts, raw IO/Close/handler/callback fences, gate cancel до SDK Close и
post-Box shared seal/drain. Input pressure использует bounded rejection/backpressure,
temporary Accept retry5ms…1s не заменяет listener. Source5Go4docs/gofmt5+3+1/manual5/
9 snapshots7existing2absent/task71rev657 +16 local SHA;100 OPEN/56 NOT_RUN.
Native UDP association/first packet/opaque tasks/OS/limits/performance/full PC/old9
OPEN; это не native/W4c acceptance и не право включить TUN.

V2-284 [UDP before Router](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v284-udp):
local mixed glue reuses SDK SOCKS/HTTP/auth/UoT parsers with owned PacketListener.
Exact TCP owner retains one original UDP bind attempt before native creation,
raw IO/one Close/result and cancellation before first packet; input cleanup also
requires UDP receipt. Partial known raw closed once even on failed bind frame;
unknown retained/no replacement.4Go4docs/gofmt4+1/manual4/8 snapshots6existing2absent/
task71rev659 +20 pinned SHA;100 OPEN/56 NOT_RUN. SDK watcher return/first buffer/
opaque tasks/native/limits/performance and full PC/old9 OPEN; no TUN admission.

V2-285 [UDP task/buffer custody](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v285-association):
SOCKS5 glue reuses pinned SDK codecs/auth, owns original association frame/watcher
and first buffer before scheduling/IO. UDP task waits exact frame/watcher/buffer
outcomes; native Close wrapper avoids self-join during delegation. Cached first
packet requires full destination capacity, one checked Release/no opaque cache.
Checked deadlines use existing TimeoutPacketConn, no timer fallback task. Strict
offered-method negotiation/finite auth errors.4Go4docs/gofmt4+1+1/manual4/8 snapshots
6existing2absent/task71rev661 +28 SHA;100 OPEN/56 NOT_RUN. HTTP/UoT/mux/opaque tasks/
TCP prefetch/native/OS/performance/full PC/old9 OPEN; no native/W4c/TUN acceptance.

V2-286 [SOCKS CONNECT prefetch](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v286-prefetch):
original TCP input удерживает bounded4096 already-buffered parser remainder до
capture/read/clear completion. SOCKS4 same SDK LazyConn augmented before delegation,
SOCKS5 wraps original Conn before NewLazyConn; empty path без prefix allocation.
Native Close fence отделён от prefix/handler drain, no self-join/raw unwrap/pooled
cache transfer.4Go4docs/gofmt4+1+1+2/manual4/8 snapshots7existing1absent/task71rev663
+30 local SHA;100 OPEN/56 NOT_RUN. HTTP/UoT/mux/opaque upstream/native/limits/perf/
full PC/old9 OPEN; no native/W4c/TUN acceptance.

V2-287 [UoT buffer retirement](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v287-uot):
original packet/frame writer fixes inspected missing input Release in SDK non-
vector path; v2/legacy request/address codecs reused. Bounded uint16 frame, checked
copy/short write/read capacity, no continuation after partial frame error. Original
Write outcome and independent buffer/frame releases retained; Close before writer
join, no native retry. Shared bounded negative-buffer custody also covers packet
guard rejection before real delegation.5Go4docs/gofmt3+2+1+1+3/manual5/9 snapshots
7existing2absent/task71rev665 +36 SHA;100 OPEN/56 NOT_RUN. HTTP pipes/cache/upgrade/
transport/mux/opaque SDK/native/limits/perf/full PC/old9 OPEN; no W4c/TUN acceptance.

V2-288 [all HTTP CONNECT prefixes](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v288-http-connect):
outer loop uses SDK codecs and existing original-input prefix custody for each
CONNECT, including a later request after HTTP keep-alive/auth retry. No pooled
cached connection/request replay; short response Write/cancellation checked.
Ordinary HTTP/Upgrade algorithms retained as local one-request helpers, not
declared original pipe/Transport/body/task receipts.2Go4docs/gofmt2+1/manual2/
6 snapshots5existing1absent/task71rev668 +38 SHA;100 OPEN/56 NOT_RUN. HTTP pipe/
transport/upgrade/header bounds/mux/native/perf/full PC/old9 OPEN; W4c stays closed.

V2-289 [HTTP pipe-pair custody](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v289-http-pipes):
both ordinary HTTP/Upgrade register exact bounded original pairs before acquisition/
Go/Router. Each end IO/Close and original handler/callback outcome retained, no
raw/ReadWaiter escape. Input seal/cancel before native/handler waits, then exact
pair drain; Close waits original end-close only, not self-dependent tasks. Known
ends closed even on failed metadata frame; unknown retained/no retry or permit.
4Go4docs/gofmt4+3+1/manual4/8 snapshots6existing2absent/task71rev671 +42 SHA;
100 OPEN/56 NOT_RUN. Transport/body/dial task/timer/HTTP quota/mux/native/perf/full
PC/old9 OPEN; no hard deadline/resource/speed or W4c/TUN acceptance.

V2-290 [original used HTTP bodies](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v290-http-body):
request/response body receipts and explicit same-original Close on every helper
exit cover Go header-write failures before writeBody. Reads/EOF/remaining length,
one Close and failed/unknown retention; incomplete Close cancels original input
before raw drain. Root closes TCP before body cleanup; no native retry, prior-Do
unknown state never fabricated.3Go4docs/gofmt3+2/manual3/7 snapshots6existing1absent/
task71rev674 +46 SHA;100 OPEN/56 NOT_RUN. Transport internal tasks/WithoutCancel/
idle/timers/HTTP quotas/mux/native/perf/full PC/old9 OPEN; W4c/TUN not accepted.

V2-291 [HTTP request dial scope](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v291-http-dial):
published request scope captures original cancellable user/source context before
Do, covering Transport WithoutCancel. Each admitted original dial frame precedes
pipe acquisition; late/failed results retained, stale delivery rejected. Close
seals/cancels before original call/pair joins, unknown sticky; root cancels scopes
before native/handler waits.3Go4docs/gofmt3+1/manual3/7 snapshots6existing1absent/
task71rev677 +46 SHA;100 OPEN/56 NOT_RUN. Internal Transport read/write/idle/TLS/
timers/native/perf/full PC/old9 OPEN; no whole-goroutine retirement or W4c acceptance.

V2-292 [HTTP request/response headers](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v292-http-headers):
single existing Reader uses128KiB request-line/header source budget incl Buffered
credit around SDK parse only, per request/auth retry; body/tunnel unaffected by
that byte cap. Builtin response header cap128KiB, phase/read faults retained.
No reader layering/reset/replay or feature removal.3Go4docs/gofmt3/manual3/
7 snapshots6existing1absent/task71rev680 +48 SHA;100 OPEN/56 NOT_RUN. Private
Transport loop/idle/TLS/timer/public-join gap, auth/time/heap budgets/native/perf/
full PC/old9 OPEN; no whole SDK/worker/native cleanup or W4c/TUN acceptance.

V2-293 [phase budgets and HTTP auth](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v293-handshake-time):
retained timer before first-byte/header/control IO,30s initial policies and4 failed
HTTP auth attempts/shared window; success resets. Release/one Stop/original callback
fences before handoff, no guessed native deadline restore. Base auth ctx per
request and cloned principal avoid retained context/credential backing chain.
5Go4docs/gofmt5+2/manual5/9 snapshots8existing1absent/task71rev683 +51 SHA;
100 OPEN/56 NOT_RUN. Remaining SOCKS phase/global rate/SDK/native/clock/interop/
perf/full PC/old9 OPEN; phase timer not whole worker/native retirement proof.

V2-294 [SOCKS phases](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v294-socks-phases):
complete SOCKS4/5 negotiation uses retained30s/earlier-context budget, with no
per-field extension. UDP bind/reply share remaining window; timer retirement
precedes stream/packet lifetime. Pinned bounded codecs retained; explicit replies
reject short writes, SOCKS4 refusal has required8 bytes.4Go4docs/manual4/
gofmt4+1/8 snapshots7existing1absent/task71rev686 +55 SHA;100 OPEN/56 NOT_RUN.
Deferred LazyConn reply/global auth rate/SDK/native/interop/perf/full PC/old9 OPEN.

V2-295 [Windows/VPS reconciliation](../../tasks/PC-VPS-REQUIREMENTS-2026-11-07.md):
new VPS31.207.5.154 has attributed stock directWS TLS/HappON evidence; absence of
VPS is no longer the sole assumption for PC-00/03. Signed profile/delivery/vault,
fixed WS /ws/Host/SNI/early-data compatibility and native/bootstrap acceptance
still separate. HY2 new certificate requires DER-pin refresh; HU unsupported,
REALITY absent, CDN/external UDP unknown. PC-SRV-01…04 and exact missing execution
scope documented; no server/policy/network change or new test.4docs +19 SHA/
4 snapshots3existing1absent/task71rev688;100 OPEN/56 NOT_RUN/full PC/old9 active.

V2-296 [retained deferred reply](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v296-socks-reply):
private original input reply owner replaces opaque LazyConn state, preserving
pinned codecs/lazy DNS-UoT semantics and native/prefix ownership. One attempt,
strict write/sticky result,30s phase before payload; native close precedes timer/
reply frame joins. No retry/unwrap/unconditional success; full8-byte SOCKS4 reply.
5Go4docs/manual5/gofmt5exit0/9snap8existing1absent/task71rev690 +61 SHA;
100 OPEN/56 NOT_RUN. SDK DNS task/buffer custody/global rate/native/interop/
perf/full PC/old9 OPEN. New attributed VPS facts need PC-03 signed path/TLS update.

V2-297 [PC-SRV-01 facts/route](../../tasks/PC-VPS-REQUIREMENTS-2026-11-07.md#pc-v297-ws-contract):
public receipt proves secret WS path mismatch with /ws; early2048/header/Host/
upgrade/muxfalse compatible. Cloudflare Origin CA + stock explicit cert does not
prove standard renderer trust. Choose Windows versioned signed WS path/endpoint
leaf-trust extension preserving legacy canonical omission and existing revocation/
import/bootstrap authority. Design route only, not implemented/live; server alias
not requested or created.4docs/manual7 SHA/4existing snapshots/task71rev692;
100 OPEN/56 NOT_RUN/full PC00…09/old9 and all false native/bootstrap gates remain.

V2-298 [optional signed WS path](../../tasks/PC-VPS-REQUIREMENTS-2026-11-07.md#pc-v298-ws-path):
Channel.WS version1/path value with omitzero preserves legacy canonical omission
and /ws. Strict finite-error object/path checks,2048-byte exact escaped path bound;
no query/normalization. Existing publisher/sign/import/projection and renderer
consume same field before credentials.3Go4docs/manual3/gofmt3exit0/7snap6existing
1absent/task71rev694 +19 SHA;100 OPEN/56 NOT_RUN. Endpoint TLS leaf/pin policy
integration, legacy byte/SDK/wire/runtime/native/full PC/old9 acceptance still OPEN.

V2-299 [signed WS TLSv2](../../tasks/PC-VPS-REQUIREMENTS-2026-11-07.md#pc-v299-ws-tls):
explicit purpose/ID/hostname-bound leaf trust, deep clone and retained continuity
purpose/defaultlegacyHY2; no sameID trust removal/transport-purpose change.
Existing local/global bans, availability/filtering/future wake/selected session
expiry and inline renderer connected; no arbitrary CA/insecure/SPKI/file patch.
9Go4docs/manual9/gofmt9exit0/13snap12existing1absent/task71rev696 +15 SHA;
100 OPEN/56 NOT_RUN. Legacy/new wire/native TLS/clock/bundle/SDK/provision/
OS/full PC/old9 acceptance unexecuted, gatesfalse; next original DNS task custody.

V2-300 [original TCP DNS requests](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v300-dns-tcp):
exact first port53 rule and original mixed root only, same DNSRouter/protected
policy. Original scope/query/read/Exchange/pack/write/timer owners retained;
stream4/global128/owned wire8MiB/expansion16MiB/response1MiB initial bounds,
deadline-bounded capacity waits. Raw Close before joins, unknown sticky.
5Go4docs/manual5/gofmt5exit0/9snap7existing2absent/task71rev698 +18 SHA;
100 OPEN/56 NOT_RUN. Not whole DNS/SDK or heap/RSS proof; UDP/other policies/TUN/
ordinary HTTP pipe/native/interop/perf/full PC/old9 acceptance OPEN, gatesfalse.

V2-301 [original UDP DNS and peer custody](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v301-dns-udp):
shared TCP-UDP scope/query quotas, same resolver/protected policy and exact first
port53 decision; owned packet buffers, SDK EDNS/truncation, original idle timer
and query deadline after read. Native SOCKS peer fixed before parallel responses
through existing immutable bind adapter; foreign sender/count/short-write controls.
7Go4docs/manual7/gofmt6+7+1exit0/11snap9existing2absent/task71rev700 +26 SHA;
100 OPEN/56 NOT_RUN. Other policies/TUN/SDK/heap/global rates/fairness/interop/
native/OS/perf/full PC/old9 OPEN; this is source, not complete DNS/VPN acceptance.

V2-302 [актуализация server baseline](../../tasks/PC-VPS-REQUIREMENTS-2026-11-07.md#pc-v302-server-baseline):
два public operator RESULT7–8Nov разделены по слоям: CF DNS repair/strict,
server CDN payload, PC upgrade101 при relay timeout, HY2 local/external и первый
SFA/manual плюс operator device HTTPS current-route. Эти результаты не закрывают
Windows signed/native/protected path; blanket WS-upgrade block/DPI выводы не
подтверждены. Alias не нужен, V299 contract/V301 code unchanged.
4docs/4 snapshots/task71rev703 +6 SHA;100 OPEN/56 NOT_RUN/all gatesfalse.
Полный PC00…09/old9 продолжается с PC01 source; тестирование/исполнение запрещены.

V2-303 [начальные intake/auth retry limits](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v303-mixed-rates):
переиспользован pinned x/time/rate0.11.0, без установки/обновления версии.
Два per-box бюджета: native pre-Accept и SDK injection делят intake, неудачные
HTTP keepalive auth retries — отдельный bucket; действующие потоки не throttled.
Прежние raw owners/rejection/close и закрытый forwarding gate сохранены.
5Go1manifest4docs/manual5+manifest/gofmt5+1/10snap9existing1absent/task71rev705
+12 SHA;100 OPEN/56 NOT_RUN. Fairness/probe priority/queue/backlog/tuning/
interop/native/OS/perf/full PC/old9 acceptance OPEN; не whole-system rate proof.

V2-304 [applied proxy lifecycle checks](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v304-proxy-validation):
повторный exact tuple readback через original writer handle до forwarding и
финального Running; private applied phase не подменяет own-write/restore receipt.
Общий finalizer сохраняет Start/select/Reload/rollback и stale/error cleanup,
IO вне Manager/Engine locks. Existing WFP source chain не включён без acceptance.
4Go4docs/manual4/gofmt4+2exit0/8snap7existing1absent/task71rev707 +11 SHA;
100 OPEN/56 NOT_RUN. CAS/ABA/crash/interprocess/periodic/native/perf/full PC/old9
OPEN; следующий независимый source объём — PC04 страницы/правдивые статусы.

V2-305 [PC-04 source presentation](../../tasks/PC-04-UI-PRESENTATION.md#pc-v305-ui-status):
Главная/Проверки/Локальное восстановление через existing Vue components/composables/
API. Stop при pending Start/Switch, single pending list refresh с actual outcome,
scope/time/historical/unknown expiry, renderer receipt отдельно от OS measurement,
порты/задачи/старые события не создают recovery/protection authority.
8UI4docs/manual source/12snap11existing1absent/task71rev709 +15 SHA;
100 OPEN/56 NOT_RUN. Browser/WebView2/DPI/keyboard/compile/runtime/полный PC/old9
не приняты; дальше PC04 остальные страницы. PC01/PC03 native gates не изменены.

V2-306 [PC-04 source: Network/Adaptation/Statistics](../../tasks/PC-04-UI-PRESENTATION.md#pc-v306-network-telemetry):
источники/время/null vs zero, same-poller invalidation после manual sites, единый
InfoFacts owner, actual policy/epoch/health/dependency/incident snapshots без
выдуманных budgets/причины/DPI/WARP applied. Bounded counter intervals с gap/reset,
известный zero и unknown rate раздельны; никакой session/adapter identity из имени.
8UI4docs/manual source/12existing snap/task71rev711 +17 SHA;100 OPEN/56 NOT_RUN.
Browser/WebView2/DPI/keyboard/compile/native/perf и весь PC00…09/old9 OPEN;
дальше PC04 Channels/Logs/Settings/Secrets и8flows. Native gates не изменены.

V2-307 [PC-04 source: Channels/Logs](../../tasks/PC-04-UI-PRESENTATION.md#pc-v307-channels-support):
declared/current-selector/lifecycle/health вместо favorite/applied/reachable inference;
root-owned single pending channel refresh с generation/revision fence. Logs filters/
bounded local buffer и explicit frozen support summary preview before clipboard copy;
нет raw unknown event JSON dump и утверждения «весь журнал без секретов».
6UI4docs/manual source/10snap8existing2absent/task71rev713 +17SHA;
100 OPEN/56 NOT_RUN. Не fullPC08 support bundle, не browser/compile/native acceptance.
Далее PC04 Settings/Secrets/8flows; полный PC00…09/old9 и native gates OPEN/false.

V2-308 [PC-04 source: Settings/Secrets](../../tasks/PC-04-UI-PRESENTATION.md#pc-v308-settings-secrets):
owner-fenced forms/vault/reveal/transfer/update outcomes, nullable reported metadata,
finite UI/facade errors, exact checked autostart/readback/foreign conflict instead
of optimistic bool. Draft cleanup не JS/Wails zeroization, dispatch не new-image
readiness.8UI2Go4docs/manual/gofmt2+2exit0/14existing snap/task71rev715 +18SHA;
100 OPEN/56 NOT_RUN. Native/browser/compile/DPI/clipboard/CAS/whole goal не приняты.
P0 source finding: TestSecret App.mu across network препятствует Stop registration;
следующий safety/lifecycle срез до оставшейся PC04 8flow приёмки. FullPC/old9 OPEN.

V2-309 [PC-01/03/04 source: TestSecret](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v309-secret-control):
устранено прямое App.mu-through-network в production GUI diagnostic; один retained
intent/lifetime task, source cancel/socket deadline/terminal original done join;
vault encrypted snapshot pre/post fence, immutable target, no fallback IP/external
SSH credential, pinned trust retained. Bounded SSH/HTTP, finite reports, nil Result
safe, no detached command timeout.4Go4docs/manual/gofmt4+4+2+3+2exit0/7existing+1new
snap/task71rev717 +17SHA;100OPEN56NOT_RUN. Это не приёмка compile/runtime/Stop hard
bound/native/SDK/trust/egress/perf. Legacy purpose binding/UDP/expectations/budget,
PC04 all8flows/full PC00…09/old9 OPEN; все gates остаютсяfalse.

V2-310 [PC-01/03/04 source: diagnostics](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v310-diagnostic-source):
общий original manual task/optional admission/terminal custody, known IDs и
intent/path/cache fences; bounded nonce-bound QUIC VN/unknown вместо random
Initial/filtered claim, no old UDP IP fallback. Actual profile credential validator,
mutation fence; original freeze subject/App ctx/full plan/HTTPS and exact4 tunnel
UI aggregation отдельно от видимых OS-route observations.6Go1TS4docs/manual/
gofmt6+4+1+2+2exit0/11snap9existing2new/task71rev719 +21SHA/1cached source.100OPEN56NOT_RUN;
нет compile/runtime/interop/SDK/native/egress/perf acceptance, gatesfalse.
Legacy SSH purpose/pins/expectation I/O, library/native joins и весь PC/old9 OPEN.

V2-311 [PC-01/03/04 source: SSH expectation/Wails](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v311-ssh-expectation):
bounded lifetime reader4KiB/max2 captured paths, original owner on uncertain Close,
missing отдельно от read/format/conflict failure, strict one-key16hex/mismatch
refusal; no new host trust. Cached Wails dispatcher drops3 outputs; TestSecret
исправлен на существующий JS report,error контракт, metadata reload отдельно.
3Go4docs/manual/gofmt2+1exit0/7snap6existing1new/task71rev722 +18SHA/3cached source;
100OPEN56NOT_RUN, без compile/native/ABI/browser/network приёмки. FullPC/old9,
SSH signed-purpose/root22/pin migration, native lifecycle/UX/release OPEN/gatesfalse.

V2-312 [PC-01/04 source: диагностический context/owner](../../tasks/PC-04-UI-PRESENTATION.md#pc-v312-diagnostic-ui):
original lossless presentation tuple/revision и explicit native contextCurrent;
cache не retagged, stale finalizer отказ, atomic path mirror без нового epoch lock.
UI один state read/poller/pending action; future batch dispatch прекращается на
unmount/context/ручной Stop, прежний RPC owner сохраняется. Central manual aggregate
для mapper/card, finite feedback, metadata polling без измерений.6Go7UI4docs,
manual/gofmt6+6+1exit0/17snap16existing1new/task71rev725/18SHA;100OPEN56NOT_RUN.
Browser/ABI/typecheck/native/physical path/TTL/fullPC/old9 OPEN; все gates=false.

V2-313 [PC-03 source: pinned SDK bootstrap DNS](../../tasks/PC-03-ENGINE-DNS.md#pc-v313-sdk-dns):
cached1.13.19 typed UDP Start отклоняет detour к empty direct; options parse
этого не доказывает. Только SDK-view bootstrap-dns теряет detour при exact plain
direct match; copied DNS/servers сохраняют normalized subject/IPC/rollback.
Protected final TLS/proxy, resolver roles/capture/worker/admission неизменны;
custom direct не переопределяется.1Go4docs/manual/gofmt1+1exit0/5existing snapshots/
task71rev728/13SHA+7cached;100OPEN56NOT_RUN, SDK/native/network/fullPC/old9 OPEN.

V2-314 [PC-03 source: original bootstrap DNS sockets](../../tasks/PC-03-WORKER-DNS-HANDOFF.md#pc-v314-dns-transport):
per-query service-only transport вместо persistent SDK UDP pool; existing
registry/client/router/remote dialer/codec/sticky connection owner reused.
Private original worker/path/host, cap2 A/AAAA, correlated UDP/TCP same-server/
deadline/4096-byte frame; pause/resume/Reset/Close join original calls and retain
uncertain ownership. Protected final/normalized subject/IPC/authority unchanged.
8Go4docs/manual/gofmt8+3+1exit0/12snap10existing2new/task71rev731/11SHA+14cached;
100OPEN56NOT_RUN, compile/SDK/native/perf/fullPC/old9 OPEN/all gates=false.

V2-315 [PC-03 source: before-DNS service dial owner](../../tasks/PC-03-WORKER-DNS-HANDOFF.md#pc-v315-service-dial):
SDK-only private socket detours под original protocol clients; normalized
subject/IPC/trust/selector сохранены. Exact source endpoint/declared hop ports,
reserved namespace/options/original constructor; cap16 pending calls до DNS,
callback join/checked Close либо same-object handoff; pause/resume/Close retirement.
10Go4docs/manual/gofmt10+4+1+2exit0/14snap11existing3new/task71rev734/11SHA+14cached;
100OPEN56NOT_RUN. SDK late racers/loser Close и shared TLS/QUIC publication —
следующие OPEN gates; no native/SDK/build/browser/network or current-EXE proof.

V2-316 [PC-03 source: SDK inner attempt owner](../../tasks/PC-03-WORKER-DNS-HANDOFF.md#pc-v316-native-attempts):
exported NewResolveDialer + owned inner N.Dialer, прежний Happy Eyeballs;
original attempt/late loser/Close/budget retained beyond early winner, passive
metadata progress and sealed native admission. Exact task DNS-address membership,
same native/SDK packet handoff и finite local fault scope.6Go4docs/manual/
gofmt6+5exit0/10snap8existing2new/task71rev737/12SHA+9cached;100OPEN56NOT_RUN.
Base-internal interface/custom-detour fanout, shared TLS/QUIC session and full
native/SDK/OS/perf/fullPC/old9 acceptance OPEN/all gates=false.

V2-317 [PC-01/03 source: strict route graph](../../tasks/PC-03-WORKER-DNS-HANDOFF.md#pc-v317-runtime-graph):
final=proxy и весь executable graph ограничен validated candidates/canonical
split roles; rule target/action allowlist, conditional direct и validated match
entries. Общая exact plain-direct форма DNS/SDK; foreign PlatformInterface
rejected до box constructor. Pinned Windows production path не включает
conditional SDK interface fanout; ShadowTLS detours уже не admitted.
5Go4docs/manual/gofmt5+1exit0/9snap8existing1new/task71rev740/15SHA+8cached;
100OPEN56NOT_RUN/fullPC00…09/old9 active. Shared HY2/mux/TLS/QUIC publication,
original-session retirement и native/SDK/OS/perf acceptance OPEN/gates=false.

V2-318 [PC-01/06 source: operation lifetime owner](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v318-session-retirement):
original per-operation sampler stop/done, join before terminal source/trust/proxy
and active release. Sampling stop не отменяет success; canceled blocked join
пытается engine/protection cleanup, сохраняет original active до callback return.
Abnormal sampler exit negative, missing receipt rejected; no timeout retirement.
4Go4docs/manual/gofmt4+1exit0/8snap7existing1new/task71rev743/15SHA+7cached;
100OPEN56NOT_RUN/fullPC00…09/old9 active. Running-watch return/rearm remains next;
shared protocol publication/retirement API gap — local WIN-TRANSPORT-001,
не trust C1. Все native/SDK/OS/perf acceptance OPEN/gates=false.

V2-319 [PC-01/06 source: running watch return/rearm](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v319-running-lifetime):
original canceled watch retained до done; exact predecessor join before new
operation sampler/positive work, Stop cleanup before join, separate self-expiry
op/full watch receipt. Fresh watch reserve before spawn after active release;
late trust faults/abnormal watch exit negative. Blocking App callback заменён
cap1 wake existing watchdog; current local failure reconciliation only.
7Go4docs/manual/gofmt7exit0/11snap10existing1new/task71rev746/7SHA;
100OPEN56NOT_RUN/fullPC00…09/old9 active. Synchronous lifecycle abnormal exits,
whole-App/global task bounds, protocol/native/SDK/OS/perf acceptance OPEN/gates=false.

V2-320 [PC-01 source: lifecycle return/cleanup](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v320-lifecycle-return):
private original bodyReturned + direct common finalizer for Start/Switch/Reload/
owned Stop; negative cleanup/watch joins/StateError on missing return/panic/
Goexit, positive terminal witness required. Deferred ordered cleanup with original
sequence result; no false native Close proof or replay after drained sequence.
Terminal Manager unlock on abnormal path first rejects provisional Running.
5Go4docs/manual/gofmt5+2exit0/9snap8existing1new/task71rev749/11SHA;
100OPEN56NOT_RUN/fullPC00…09/old9 active. Internal Box/App/opaque callback/native/
SDK/OS/perf acceptance OPEN/gates=false; no tests or product execution.

V2-321 [PC-01 source: original Box/drain](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v321-box-retirement):
one original close/drain owner, immutable completed sequence result; pending
or reentrant Close restrictive, no native/SDK retry. Original bootstrap/gate/
SDK/post-SDK order retained through registered deferred steps; cohort published
before cancellation, rejected/mixed intake sealed after SDK Close attempt.
Failed scope remains owned; Engine abnormal close error sticky; late routeFault
not hidden by old successful result. Sequence done is not native/SDK task proof.
5Go4docs/manual/final gofmt5exit0/9snap7existing2new/task71rev752/9+1SHA;
100OPEN56NOT_RUN/fullPC00…09/old9/gates=false. App pre-acquisition/RPC and SDK
internal cleanup/shared-session retirement/native acceptance remain OPEN.

V2-322 [PC-01 source: serialized App Stop](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v322-app-stop-outcome):
ordinary-return witness/direct negative outcome for exact original registration;
ordered deferred publication/protection/Manager/helper cleanup. Manual Release
remains a separate positive phase after drained/error-free original cleanup.
Deferred capture/restart/elevation groups and protected/optional/delivery lanes;
claimed mutex releases on abnormal unwind, original failed owners retained.
6Go4docs/manual/gofmt6+2exit0/10snap8existing2new/task71rev755/12SHA;
100OPEN56NOT_RUN/fullPC00…09/old9. Pre-serializer registration/Start/Switch/RPC/
native/SDK/full-task acceptance OPEN, all gates=false; no tests or product runs.

V2-323 [PC-01 source: App Start/Switch](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v323-app-lifecycle):
original registration captured before waits; body ordinary-return/direct negative
boundary through existing operationMu. Abnormal original source revoked; stale
rollback invalidated; negative Stop then same-registration Error. Retained original
Stop cohort avoids replay while slots are original/cleared. beginOperation checks
exact source atomically; Stop registration has no Wails callback before cancel.
6Go4docs/manual/gofmt6+4+5exit0/10snap8existing2new/task71rev758/12SHA;
100OPEN56NOT_RUN/fullPC00…09/old9. Startup/Close/other action/full-task/native/SDK/
RPC/release acceptance OPEN/all gates=false; no tests or product execution.

V2-324 [PC-01 source: startup/Close/Quit](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v324-startup-close):
startup/producer preparation ordinary-return boundary; readiness after outcome,
sticky abnormal startup gate also denies recovery/protected operations. Single
original Close task, ordered cancellation/Stop, terminal producer joins/log drain
through registered steps. Quit finalizer preserves earlier errors and attempts
terminal UI request despite abnormal producer/log callbacks; existing backstop
unchanged, blocked pre-Quit/native custody not proven.7Go4docs/manual/gofmt6+1+3exit0/
11snap7existing4new/task71rev761/12SHA;100OPEN56NOT_RUN/fullPC00…09/old9/gates=false.
Watchdog/other-action/full-task/native/SDK/OS/UX/release acceptance OPEN; no tests.

V2-325 [PC-01 source: watchdog ownership](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v325-watchdog-owner):
one original entry/error/done task, no replacement after cancel/deadline; Start
requires entry before native setup. One-way negative fence precedes mutex waits;
same-goroutine original cancel/Stop/Error, no self-join or remote channel blame.
Terminal cancellation/join uses actual original done and shared producer budget.
Enabled/DiagnosticContext/positive publication use task/fence, not watchOn history.
10Go4docs/manual/gofmt9+3+2exit0/14snap12existing2new/task71rev764/13SHA;
100OPEN56NOT_RUN/fullPC00…09/old9. Other-action/full-App/native/SDK/OS/UX/release
acceptance OPEN/gates=false; no tests/fixtures/build/product execution.

V2-326 [PC-01 source: RunProbe/native actions](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v326-probe-actions):
original RunProbe registration/body outcome reuses existing lifecycle serializer;
busy cleanup precedes UI event. Protected failure preserves original attempted
subject only for restrictive custody, never health. Optional native action has
separate sanitized outcome/own-draft rejection, no automatic mandatory VPN failure;
source-bound native cache and failed value-lease unwind.4Go4docs/manual/final gofmt4exit0/
8snap7existing1new/task71rev767/12SHA;100OPEN56NOT_RUN/fullPC00…09/old9/gates=false.
Manual diagnostic socket/watcher/full-App/native/SDK/OS/release acceptance OPEN.

V2-327 [PC-01 source: manual diagnostic retirement](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v327-manual-retirement):
Original bounded Dial registry перед callback, late-connection custody и one-attempt
native Close используются HTTP/SSH/UDP. Body/watch/log ordinary-return и exact
Dial/Close done отделены от deadline/cancel. Known resource retirement позволяет
release при stale result; unknown cleanup оставляет original slot/admission pending.
32 total Dial callbacks — локальная граница, не per-request/wire/RSS guarantee;
исчерпание даёт unavailable без обвинения сети. Fixed sanitized finalization,
original budget release один раз, App pointer/done в конце.8Go4docs/manual/
gofmt8+8+2+2exit0/12snap/12reviewed+2cachedSHA. Source/unverified; pre-admission,
whole-App/SDK/native/OS/release gates OPEN/fullPC00…09/old9/100OPEN56NOT_RUN.


V2-328 [PC-01 source: diagnostic admission](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v328-diagnostic-admission):
Shared budget context/release construction moved before active reservation;
unused cancel/deferred pool unlock and post-lock ctx check. Manual preparation
has original ordinary-return outcome, presentation before admission, separate
unused cancel/release guards and final source/path/closing/ctx handoff check.
Rolling costs/lanes unchanged; no refund, replacement owner or cleanup-as-success.
2Go4docs/manual/gofmt2+1exit0/6snap/12reviewed+2cachedSHA, source/unverified.
RunAdaptiveCheck Background/parallel HTTP original ownership remains next;
context internals/full-App/SDK/native/OS/release OPEN/100OPEN56NOT_RUN.


V2-329 [PC-01 source: optional site batch](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v329-adaptive-owner):
Original manual diagnostic owner/context/budget, copied max8 targets and exact
applied subject replace Background/WaitGroup-only completion. Every original
classifier ordinary-return/done is joined; HTTP Dial/Close uses same task owner.
History stage before release reuses retained filetxn slots/32KiB/max8 decoder;
history is not current/last-command proof. Safe unavailable reply and Vue feedback
without Wails/schema/binding changes.4Go1Vue4docs/manual/gofmt4+1exit0/9snap/
20reviewed+2cachedSHA/source-unverified. Background TCP Close/body ownership,
whole-App/SDK/native/OS/UX/release OPEN/fullPC00…09/old9/100OPEN56NOT_RUN.


V2-330 [PC-01 source: background TCP ownership](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v330-background-owner):
Original background task normal-return/fault/attempt and result-aware terminal
join replace done-only cleanup acceptance. Serial TCP attempt before admission,
one native Close, original cancel observer joined before release; uncertain local
work retained without channel blame/replacement. Round original registration/
active/path/control/epoch captured before target load. Fresh blocked-intent rounds
are weak data only; actual Dial timestamp and older/equal background health refusal.
5Go4docs/manual/gofmt4+2+2+2+2+1exit0/9snap/18reviewedSHA/source-unverified.
Remaining tray/network/full-App/SDK/native/OS/release OPEN/100OPEN56NOT_RUN.


V2-331 [PC-01 source: producer retirement](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v331-producer-retirement):
Original tray callback/worker/Run/retire normal-return receipts, sealed worker
admission, exact result-aware terminal joins for tray/poller/DNS. Registered
network cleanup distinguishes body failure, functional power continuity and
unknown cleanup; explicit rearm requires original cleanup plus prior native/
Manager/helper guards. Native power entry retained before Pin/Register; one
unregister attempt and original callback outcome.9Go4docs/manual/gofmt9+3+1exit0/
13snap11existing2new/task71rev782/13reviewed+2cachedSHA/source-unverified.
Next registry handle/Close/readback/unknown autostart audit. Native systray/API/
OS/full-App/UX/release/fullPC00…09/old9 OPEN/100OPEN56NOT_RUN/gatesfalse/no tests.


V2-332 [PC-01/04 source: autostart ownership](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v332-autostart-owner):
Original process registry operation before Open, retained output, one checked
Close, terminal seal/result join; shared tray/Wails serializer. Bounded64KiB
exact REG_SZ query без realloc/truncated foreign value adoption. Tray unknown/
external-change first click refresh-only, explicit subsequent toggle/readback.
5Go4docs/manual/gofmt5+1exit0/9snap6existing3new/task71rev785/15reviewed+4cachedSHA.
Source/unverified; no external-writer CAS/kernel/login/SDK/UI proof. Next Settings
concurrent refresh/write/readback stale publication and stale success/error audit.
FullPC00…09/old9/100OPEN56NOT_RUN/all gatesfalse/no tests/build/native/network.


V2-333 [PC-04 source: Settings autostart](../../tasks/PC-04-UI-PRESENTATION.md#pc-v333-settings-autostart):
Existing row now one AutostartSetting UI owner: serial read/write/readback,
one explicit queued refresh/revision/unmount fence; stale result/error refusal,
fresh success clears old error. Independent refresh from profile/runtime summary,
nullable state, accepted-response timestamp, focusable guarded actions/stable status.
2Vue4docs/manual/6snap5existing1new/task71rev788/14reviewedSHA/source-unverified.
Native/API/store/bindings unchanged; compile/race/browser/a11y/scaling/UX OPEN.
Next Settings profile/runtime coupled allSettled publication/loading/timestamp
audit. FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/no tests/build/native/network.


V2-334 [PC-04 source: independent facts](../../tasks/PC-04-UI-PRESENTATION.md#pc-v334-independent-facts):
Settings profile/runtime и shared infoFacts network/failover больше не связывают
publication/error/receipt через allSettled. Original per-source pending/flags,
specific refresh и refcount; Network/AutoProtect используют свой источник.
createPoller manual-null mode/no timer и pre-load generation admission guard;
numeric timer formula сохранена, queued stale entry теперь пропускается для всех.
3Vue2TS4docs/manual/9snap9existing/task71rev791/14reviewedSHA/source-unverified.
Next usePolling raw-error-to-UI contract audit. FullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/compile/browser/a11y/native/SDK/OS/UX/release OPEN/no tests/build/network.


V2-335 [PC-04 source: safe UI failures](../../tasks/PC-04-UI-PRESENTATION.md#pc-v335-safe-ui-errors):
Local readonly action catalog replaces raw Error.message/String in polling,
lifecycle toasts and public-profile/pin forms. Apply failure remains uncertain;
preview error clears receipt/expiry, export/save clears stale success; owned
correlation checks preserved. No API/native policy or generic logger redesign.
2Vue5TS4docs/manual/11snap10existing1new/task71rev795/17reviewedSHA/source-unverified.
Next native log.text producer/retention/support-export trace; no actual leak claim.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/no tests/build/browser/native/network.


V2-336 [PC-04 source: log record projection](../../tasks/PC-04-UI-PRESENTATION.md#pc-v336-log-projection):
App log projection before memory/queue/Wails/disk, Go4096 bytes and UI4096 UTF16
units including marker, whole-piece truncation/visible controls/invalid encoding;
known severity fallback. Actual engine prefix recognition fixed; existing closed-
vocabulary support snapshot retained.2Go2TS4docs/manual/gofmt2+1exit0/8snap7existing1new/
task71rev798/20reviewed+1stdlibSHA/source-unverified. Not secret redaction/global
memory/quota/native proof. Next raw SDK log/producers/privacy/retention source
audit. FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/no tests/build/native/network.


V2-337 [PC-04 source: log producer privacy](../../tasks/PC-04-UI-PRESENTATION.md#pc-v337-log-privacy):
startup storage notices fixed/no raw error/path; success only profileErr=nil.
Bootstrap logs fixed cache/DNS/DoH hint observations after addressHint, not
service/path proof, no host/IP/resolver address interpolation. SDK writer default
warn+ suppresses per-connection info output; observable has separate semantics
and still may emit lower levels. SDK file append/retention open; no blanket
redactor/no-log claim.4Go4docs/manual/gofmt4+1+1exit0/8existing snapshots/task71rev801/
16reviewed+6cachedSDK SHA. Next central failCore state/log projection and remaining
SDK raw-log retention. FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/source-unverified;
no tests/build/parser/SDK/browser/native/network/secrets/otherrole work.


V2-338 [PC-04 source: lifecycle failure text](../../tasks/PC-04-UI-PRESENTATION.md#pc-v338-lifecycle-failure):
failCore/Stop/restart finite stage/typed-detail projection, no raw error/unknown
stage/channel ID copy; trust-cleanup suffix fixed. Corebridge scoped producer
notices no raw Error/egress/watchdog Summary; finite diagnostic code/hint and
original control/returns/late guards preserved.4Go4docs/manual/gofmt4+1exit0/
8snap7existing1new/task71rev804/17reviewed SHA/source-unverified. SDK log lifetime/
retention and remaining raw returns/producers OPEN; fullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/no tests/build/SDK/browser/native/network.


V2-339 [PC-01/04/06 source: SDK log policy](../../tasks/PC-04-UI-PRESENTATION.md#pc-v339-engine-log):
renderer/private SDK projection explicitly disable raw logger; no production
engine.log tailer or candidate log path/dir effect. Existing App finite events
and SDK lifecycle retained; nonempty legacy path unsupported before secrets.
V337 warn-only policy superseded.5Go4docs/manual/gofmt5exit0/9existing snapshots/
task71rev807/17reviewed+8cachedSDK SHA/source-unverified. No old-file cleanup,
generic redactor, global no-log or metrics/native parity proof. Next bounded
current-session metrics HTTP consumer; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/
no tests/build/parser/SDK/browser/native/network/real data/other-role work.


V2-340 [PC-04/06 source: bounded metrics intake](../../tasks/PC-04-UI-PRESENTATION.md#pc-v340-metrics):
complete2MiB body/2048 connection cap with bounded strict token shape before
typed cloning; required nonnegative counters, no duplicate/case alias/trailing
prefix acceptance. Loopback/no redirects/compression, per-session poll TryLock,
App ctx/final metrics-pointer check, overflow-safe stable ranking and unknown
on counter regression.2Go4docs/manual/gofmt2+2+1+2exit0/6snap5existing1new/
task71rev810/13reviewedSHA/source-unverified. Original HTTP/custody/full-scope
publication next; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/no tests/build/
product/native/network. Prior0% implementation report withdrawn, not a code metric.


V2-341 [PC-01/04/06: original metrics ownership](../../tasks/PC-04-UI-PRESENTATION.md#pc-v341-metrics-owner):
one process-wide original request/Dial/socket/body/cancellation observer before
I/O; fixed loopback HTTP/1 without Transport pool/retry, framed16KiB headers/
4MiB accepted wire reads plus V3402MiB decoded limit. One native socket Close,
original callback outcome and joins; unknown owner retained. Stop joins before
Manager.Stop; terminal seals admission/cancels/joins original attempt. No stale
baseline/view commit: retained existing runtime→Manager→App→epoch identity lease,
sort/allocations outside it. No new health/forwarding grant.7Go4docs/manual/
gofmt7+1exit0/11snap9existing2new/task71rev813/source-unverified. Native/SDK/race/
whole lifecycle/Wails/UI/release OPEN;100OPEN56NOT_RUN/gates=false/fullPC00…09/
old9 active. Owner estimate≈55% is rough, not measured implementation/acceptance.


V2-342 [PC-04/06: correlated metrics UI](../../tasks/PC-04-UI-PRESENTATION.md#pc-v342-metrics-ui):
process-local collector + same-value-commit DiagnosticContext, strict optional
native-record receipt mapping; unsupported older receipt unavailable. Existing
poller original Symbol/request.current, synchronous optional scope reset, hidden
pause/before-unmount cleanup, receipt-before-data. coreMetrics reuses root state/
poller/history and exact context key; rates only across same collector/context,
no cross-session stitching.3Go5TS1Vue4docs/manual/gofmt3+3exit0/13snap12existing1new/
task71rev816/source-unverified. Compile/types/Wails generation/native/SDK/browser/
a11y/release OPEN; fullPC00…09/old9/100OPEN56NOT_RUN/gates=false. No tests/build/
product/native/network or generated/fixtures edits;55% rough/unmeasured remains.


V2-343 [PC-01/06: original proxy key](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v343-proxy-key):
original native output/owner before RegOpenKeyEx and one RegCloseKey attempt for
writer/query; invalid/pseudo/unknown output retained, no Close replay/replacement.
Borrowed key never grants custody; Restore refuses I/O after Close attempt.
Existing own-write/full-tuple/type/presence/conflict authority preserved; failed
read-only snapshot clears display values.3Go4docs/manual/gofmt3+3+1exit0/
7snap6existing1new/task71rev819/14reviewedSHA+2cached/source-only. W4c acceptance
false unchanged; durable W3b/interop/registry/WinINET/native/resources/OS/release
OPEN. FullPC00…09/old9/100OPEN56NOT_RUN/gates=false/no tests/build/product/native/
network or real data;55% rough estimate, not measured progress.


V2-344 [PC-01/06: proxy write/notify outcomes](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v344-proxy-phases):
original adapter before/target/readback/outcome and per-writer WinINET Find/39/37
attempts before callbacks. Unknown phase refuses further same-writer I/O/Close/
replacement; returned normal errors remain distinct. Missing delete no longer
earns own-write receipt; real no-op stays before callback. Reuse bounded typed
setters/full tuple/phase checks; private data not logged/persisted.4Go4docs/manual/
gofmt4+4+4exit0/8snap6existing2new/task71rev822/12reviewed+1cached/source-only.
Durable W3b/NETGUARD-001 external scope/protocol/migration and full native/registry/
WinINET/ABA/OS/resources/release OPEN; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/
TUN unavailable/schema2 unsupported. No tests/build/native/network;55% rough only.


V2-345 [PC-01/06 original proxy reads](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v345-proxy-read):
one retained HKEY-bound read/2×3 native input-output records before query;
shared typed decoder/double observation, unknown query cannot be replaced or
closed. All6 production read sites migrated.4Go4docs/manual/gofmt4+4+1exit0/
8snap7existing1new/task71rev825/source-only; native/ABI/registry/resources/durable
W3b/NETGUARD migration/OS/release OPEN.100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/
TUN unavailable/schema2 unsupported/no tests/build/native/network;55% rough only.


<a id="pc-v346-proxy-conflict"></a>

### V2-346 — PC-01/04/06, known proxy conflict / source-only

- Sticky original-snapshot conflict cannot be erased by later matching bytes.
  Known read/write/notify phases permit non-mutating original HKEY retirement;
  an unknown callback/readback/Close retains its original owner and prior barrier.
- Public Restore still reports conflict. Manager accepts only a private one-use
  exact proxy/native/confirmed-Close receipt as component retirement; no public
  boolean/error/display tuple or generic adapter can promote unknown cleanup.
- Manager blocks reuse/positive commit and preserves the conflict in lifecycle
  errors. Root blocks candidate churn and automatic timer/network/pin/startup/
  profile resume; accepted explicit Start clears only its local retry latch.
  Previous.Stop conflict is checked before constructing the replacement Manager.
- Existing UI reason/label explains proxy_conflict; finite warning preserves
  confidentiality. Other cleanup errors, explicit-only WFP release and all
  ingress/native/source/trust gates retain their prior meaning.

15Go2TS4docs/21snap20existing1new, manual16diff+1new, gofmt13+2exit0 only.
No tests/build/parser/types/SDK/browser/native/registry/WinINET/network execution.
W3b durable recovery/UserMode protocol/NETGUARD installed-old migration, native
Close/ABI/OS/resource/compile/UI acceptance and full PC-00…09/old9 remain OPEN.
All100OPEN56NOT_RUN/gates=false/TUN unavailable/schema2 unsupported;55% rough.


<a id="pc-v347-lifecycle-ui"></a>

### V2-347 — PC-04, request availability и объяснимый recovery / source-only

- Local Start/Stop predicates выделены из actual handlers; projection использует
  те же условия. Reloading/unknown state не принимает новый Start. Snapshot
  не форматирует error и не вызывает native API. Allowed относится только к
  регистрации команды, не worker/egress/cleanup/protection authority.
- Optional lifecycleActions schema1 binds exact decimal intent/lifecycle to
  existing DiagnosticContext. Missing/unknown/inconsistent facts = unavailable;
  stopped CoreReady=false не равно Start denial. Go всё повторяет при вызове.
- Existing facade/store/Dashboard use finite blocked reason/next-step catalog,
  preserve unrecognized reasons and proxy explicit latch after Stop, explain
  Disconnect effect, keep Stop during pending Start/Switch, allow state refresh.
  Existing status/live region/native buttons/layout retained, no new cards.

4Go4TS1Vue4docs/manual8diff+1new/14reviewed/gofmt4exit0 only. No tests/fixtures/
build/types/bindings/browser/native/network. Whole action completion/receipt
freshness/old-client and generated-model compatibility/keyboard/DPI/WebView2/
all pages/native/OS/release remain OPEN; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse.


<a id="pc-v348-lifecycle-requests"></a>

### V2-348 — PC-01/04, original UI requests и late-positive fence / source-only

- UI session + canonical uint64 decimal sequence/high-water и original handler
  registration находятся в одном App.mu domain. Older queued Start/select cannot
  register after newer Stop. Fixed original positive/Stop callbacks reserve before
  handlers; unknown frame retained, session opening does not reset their slots.
- Finite structured receipts distinguish registered/returned/rejected/failed/
  superseded/unconfirmed. Start registered is not running; ordinary return is
  not native cleanup/ingress/protected egress. Old positive public calls disabled
  after protocol activation; explicit legacy Stop remains a negative escape.
- Facade resolves optional generated exports, retains one Open, validates exact
  session/sequence/action/outcome/intent, no positive legacy/window.go bypass.
  Frontend opaque root/action/read owners retain original calls across dispose.
  Post-command state demand coalesces one original read without waiter growth;
  native context floor/minimum intent reject stale observations. No applied toast.
- Existing Dashboard/Channels/HintBox show finite command feedback, compatible
  feature absence and explicit negative Stop, including unavailable state.

5Go4TS2Vue4docs/manual10diff+1new/17reviewed/gofmt5exit0 only. Tests/build/types/
generation/browser/native/network not run. Generated binding delivery/transport/
panic-Goexit/races/native/whole UX/OS/SDK/W3b/NETGUARD/release acceptance OPEN.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.


<a id="pc-v349-mode-actions"></a>

### V2-349 — PC-01/04, process mode/request/next process отдельно / source-only

- RuntimeInfo schema1 mode facts contain actual PID/immutable flag and existing
  compiled negative gates. Facade validates PID binding/mode/bools/consistency;
  missing or old data is unknown, legacy boolean reader rejects non-bools.
- Mode targets share ordered admission and captured original restart source;
  original preflight/handoff/Stop/UAC/native guards remain, legacy positive mode
  calls cannot bypass an activated protocol. Body-entry marker is not OS proof.
- One serial mode reader and opaque original request owner; target never flips
  observed mode. Unknown response/entered failure/awaiting new process retains
  negative hold across root disposal, no success toast or automatic retry.
- Existing Autostart layout/controls inform shared ProcessModeSetting used on
  Dashboard/Settings. Header removes running→TUN proof, source/refresh/unknown/
  request/outcome shown separately; aria-busy reflects actual pending work.

5Go3TS4Vue4docs/manual10diff+2new/18reviewed/gofmt5exit0 only. No tests/build/types/
generation/browser/native/network. Generated delivery/native child/handoff/OS/
privilege/recovery/keyboard/DPI/whole pages/SDK/W3b/NETGUARD/release OPEN.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.


<a id="pc-v350-update-owner"></a>

### V2-350 — PC-04/08, конкретный пакет и original update workflow / source-only

- Превью получило одноразовое подтверждение точных manifest/sidecar bytes,
  original intent/path, renderer session и local monotonic consent deadline.
  Источник захватывается до ожидания serializer; version policy проверяется
  уже в preview. Approval выбирает цель, не заменяет доверие или native допуск.
- Ordered update consumes approval/captures restart registration under App.mu.
  Повторное чтение сверяет approved raw bytes до SDK/history retention; прежние
  подпись/role/source/floor/payload/install/handoff/native guards сохранены.
  Protocol-active legacy ApplyUpdate не обходит выбор и порядок команд.
- Original picker/check/apply остаётся занят после unmount/root disposal до
  фактического возврата. Lost viewer/session не публикует late preview и не
  отправляет late apply. One positive slot shared with Start/Switch/mode;
  Stop remains negative escape, no ordinary return -> installed/new process.
- Original install/staging-body entry bit — только phase fact. Ordinary known
  before-install refusal permits fresh check, без обещания неизменной history.
  Entered/unknown install retains negative hold, не снимаемый Stop/display reply.
- Existing component/HintBox/tokens, finite reason catalogue, strict optional
  approval mapping/null size, semantic described controls/wrapping/no new cards.

5Go4TS2Vue4docs/manual9diff+2new/17pinned/gofmt5+1exit0. No tests/fixtures/build/
types/parser/SDK/generation/browser/native/network. Generated delivery/installer/
child/handoff/recovery/wholeUX/OS/SDK/W3b/NETGUARD/release OPEN.100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported/fullPC00…09/old9 active.


<a id="pc-v351-profile-owner"></a>

### V2-351 — PC-04, original profile workflow / source-only

- Один original frontend slot с bounded metadata для профиля, списка отзывов
  и переноса. Original pending переживает disclosure/root disposal; новая view
  не запускает второй вызов до actual return. Это не backend/native admission.
- Общая selected revision инвалидирует замещённое preview; original root/view/
  input interest отделён от pending. Late picker/check/save не продолжает
  сценарий и не публикует прежний результат на новой странице. Negative cancel
  не освобождает original slot и не объявляется завершённой native очисткой.
- Parent demand свежих profile/runtime facts переиспользует Settings serial
  readers после actual completion, включая ответ прежней закрытой панели.
  Ordinary profile return не выдаётся за readiness/защищённое соединение.
- Opaque input revision заменяет password-containing JSON snapshot. View cleanup
  очищает пароли/transport/QR/export/preview refs, без обещания zeroization Wails.
  Existing components/expiry/controls/tokens/wrapping; убрана outer card.

1TS4Vue4docs/manual4diff+1new/15pinned/source-only; no tests/fixtures/build/types/
parser/generation/browser/native/network. Backend/public-source lifetime/ordered
mutation/SDK/native retirement/wholeUX/OS/installer/W3b/NETGUARD/release OPEN.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.


<a id="pc-v352-public-lifetime"></a>

### V2-352 — PC-01/04, public-source original lifetime / source-only

- Public profile/pin/export operations reserve one exact original backend flight.
  Actual App lifetime/restart registration required, no absent-context Background.
  Normal original callback return retires only its pointer; panic/Goexit frame
  remains restrictive, timeout/Cancel does not free admission.
- Preview captures original intent/path and one passive cancellation watcher.
  Stop requests cancellation; watcher joined before ordinary release. Epoch/source
  changes deny preview publication; physical I/O interruption is not proved.
- Draft retains original App parent/intent apart from short preview child.
  Matching purpose/token consumed once, old intent rejected at apply/export
  admission. Actual bounded parent supplied to existing private handlers;
  their own profile retirement does not cancel the operation by construction.
- Cancel captures only original callback and invokes outside locks, never frees
  active or undoes an approved apply. Full fresh private trust/source/expiry/
  credential/retention/native checks remain; no native authority from callback.

4Go4docs/manual3diff+1new/15pinned/gofmt4+2+1exit0 only. No tests/fixtures/build/
SDK/browser/native/network. Captured mutation intent after admission/operationMu
and explicit source transfer across own profile retirement remain OPEN, as do
other legacy fallback paths, native SDK retirement, wholeUX/OS/installer/release.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.


<a id="pc-v353-profile-registration"></a>

### V2-353 — PC-01/04, original profile request и own-cutover handoff / source-only

- Capture original intent/source before public decode/encrypted KDF/serializer,
  private bound scope не recaptures stale source. Common profile/pin import
  rejects absent App lifetime, exact source rechecked after operationMu wait.
- Actual own profile retirement transfers negative source under the same App.mu
  as advanceOperationLocked; exact successor rechecked after original native Stop.
  Manual Stop later invalidates it; value-only journal/commit publication fences
  do not acquire App locks and do not constitute native retirement authority.
- Public mutation flight carries admission registration; public export checks
  vault/result against the same scope across its waits. Original private SDK/
  role/hash/expiry/recovery/credential/store gates retained. Authenticated observed
  prefix still must be retained even if positive request became stale.

8Go4docs/manual7diff+1new/14pinned/gofmt8+2+2exit0. No tests/fixtures/build/SDK/types/
parser/generation/browser/native/network. Native write linearization/durability/
retirement/cancel/resource bounds, remaining legacy export/file contexts, source
UI outcomes/support clipboard/wholeUX/OS/installer/SDK/adaptive/release OPEN.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.


<a id="pc-v354-transfer-lifetime"></a>

### V2-354 — PC-01/04, original export→dialog→file workflow / source-only

- Existing public-source flight/slot reused for complete legacy export/QR/save/
  load macros. One captured App/intent/90s child spans waits/crypto/dialog/I/O;
  no nested admission, no fresh lifetime at a later stage. Local transfers work
  independently of physical network path, public/policy previews retain their path gate.
- Legacy export checks original source/vault after operationMu waits and around
  credential projection/sealing/final result; no stale QR/ciphertext publication.
  Read results and selected file inputs have negative source gates as well.
- Existing selected writer gains optional value-only source guard at existing
  acquisition/chunk/pre-rename/final checks in both platform implementations.
  Old guard cannot be replaced; Context.Err/Done semantics/trusted-store mode/
  original ownership/Close/publication/uncertain receipts remain intact.
- Save returns an error alongside possible/already completed name publication;
  cancellation does not promise rollback or absent file. No fresh writer/temp
  owner created as a retry fallback, no destination validity renewed by save.

8Go4docs/manual8diff/13pinned/gofmt8+2+1exit0 only. No tests/fixtures/build/SDK/parser/
types/generation/browser/native/network. Native commit linearization/cancel/
durability/resources/context callbacks/chooser/wholeUX/OS/installer/release OPEN.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.


<a id="pc-v355-support-clipboard"></a>

### V2-355 — PC-04, shared clipboard workflow / source-only

Support and secret copy now reserve one metadata-only original workflow before
vault reveal and retain it across view/root disposal until actual callback and
original provider-promise settlement. A missing completion handle after dispatch
retains an unconfirmed barrier. One captured existing Wails/browser provider;
no automatic fallback, retry, clipboard read or clear. Provider acknowledgement
is not current clipboard content, successful paste, native custody or zeroization.
Frozen support snapshot/current filter fence/manual preview and stale secret
response cleanup implemented in source; existing UI components/tokens reused.
2TS4Vue4docs/manual6diff/12pinned only; tests/build/types/browser/native not run.
Full PC-00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.
Next PC-04: finite interrupted-source/publication outcomes across profile/update
screens; wholeUX/native/OS/installer/release acceptance remains OPEN.


<a id="pc-v356-action-outcomes"></a>

### V2-356 — PC-04, refusal/dialog/late/publication outcomes / source-only

Existing API now rejects missing/malformed path/transfer/QR replies instead of
coercing them to a cancelled dialog. Only a private local target-specific refusal
brand means not sent; RPC errors and decoded-result failures remain unconfirmed.
Ten profile workflows retain finite metadata-only settlement/negative reminders
across page/root disposal, with display-only dismissal. Optional selected profile
identity/SHA checked on apply reply; policy Cancel during apply promises no rollback.
Existing update original owners/holds preserved; typed not-sent/before-install/
next-process-unconfirmed/unconfirmed outcome and finite late-check display added.
No new backend RPC/DTO/generated sources/deps.5TS4Vue4docs/manual9diff/13pinned only.
No tests/fixtures/build/types/parser/SDK/browser/native/network. FullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported. Native receipt/
IPC/commit/reconciliation/wholeUX/OS/installer/release remain OPEN. Next PC-01 W4c.


<a id="pc-v357-native-admission"></a>

### V2-357 — PC-01 W4c, original owner before acquisition / source-only

Production WFP lease now follows existing image owner allocation/in-place
preparation contract. Exact capture owns it before native calls and owns ingress
scope before duplicate/RPC. Legacy signatures retained, unreturned missing owner
cannot establish clean absence. Both revocations registered as existing lifecycle
cleanup steps; scoped fallback retirement requires normal callback return.
Unknown Open/Add/Close retains original resources; pure/busy owner does not touch
another lease. Existing diagnostic filter and compiled forwarding acceptance=false
unchanged.4Go4docs/manual4diff/12+4pinned/gofmt4+3exit0 only; no tests/build/native.
FullPC00…09/old9/100OPEN56NOT_RUN/TUN unavailable/schema2 unsupported. Windows/WFP/
IPC/OS/resources/full acceptance remain OPEN; next shared cleanupCall witnesses.


<a id="pc-v358-cleanup-witness"></a>

### V2-358 — PC-01, callback completion and failed flow terminal / source-only

Shared cleanupCall now requires ordinary callback return without relying on a
nonnil panic payload. Existing source/completion guards reuse it; ordinary error,
context and lease-release contracts preserved. Native TCP registers original
cancel/stream closes/SDK callback before callbacks; failed done is a local fault,
never a retire/proven receipt. Lifetime query interruption no longer keeps positive
legacy default; typed trust_unavailable and normal source errors preserved.
4Go4docs/8snap7initial+1expanded/15+2pinned/manual4diff/gofmt3+1+1exit0 only.
No tests/fixtures/build/types/parser/SDK/browser/native/network. FullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported. Underlying SDK/
native I-O/locks/OS/wholeUX/installer/release remain OPEN. Next PC-01 source audit
against requirements, without replacing native acceptance with callback return.


<a id="pc-v359-pc01-audit"></a>

V2-359 (2026-11-10), source/unverified: [PC-01 requirements matrix](../../tasks/PC-01-SOURCE-AUDIT.md)
сопоставляет W4c/current owner/cleanup/manual Disconnect/proxy с исходниками.
Native gate остаётся false; durable authenticated W3b/interprocess/external
NETGUARD migration действительно не реализованы и требуют согласованного scope.
Это не нулевая реализация PC-01 и не закрытый этап.

PC-06/04: удалён whole-file log read; original logging owner с одним read lane,
64KiB tail+256B prefix/256 complete rows before decode/4096B record, unknown
retirement retained. Optional strict log DTO/существующий экран показывают
missing/busy/unavailable и пределы фрагмента; log failure сохраняет task/proxy.
2Go2TS1Vue4docs/9snap7existing2new/manual4diff+1new/gofmt2exit0, без тестов/build/types.
COM init result/thread affinity — следующий независимый source-дефект.
100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/TUN unavailable/schema2 unsupported;
OS/native/SDK/physical I-O/resources/wholeUX/installer/W3b/release OPEN.

<a id="pc-v360-com-observer"></a>

V2-360 (2026-11-10), source/unverified, PC-06:
[COM observer](../../tasks/PC-06-NETGUARD-OBSERVATION.md) заменяет ignored init и
thread migration одним original collector/OS thread. S_OK/S_FALSE balanced,
failed init без Uninitialize; captured VARIANTs/interfaces, проверенный single
Clear и no borrowed-dispatch double Release. Shared service/folder для3bounded
tasks/4typed properties. Unknown native phase/Clear/retirement удерживает owner;
failed cleanup не публикует partial batch. No arbitrary Value/current-time
fallback/fabricated HRESULT0.2Go4docs/6snap4existing2new/manual1diff+1new,
12reviewed/8cached/gofmt2+1exit0, без tests/build/types/native. V359 log/UI intact.
Loopback dial/Close owner — следующий independent source gap; OS/COM/native/
SDK/resources/wholeUX/W3b/installer/release OPEN.100OPEN56NOT_RUN/fullPC00…09/old9,
gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v361-loopback-owner"></a>

V2-361 (2026-11-10), source/unverified, PC-06/04:
[original loopback observer](../../tasks/PC-06-NETGUARD-OBSERVATION.md#pc-v361-loopback-owner)
переиспользует preflightNetwork/sticky Close/lifecycleCleanupStep; one retained
lane/2unique literal IPv4 ports/one requested300ms budget. Unknown original
dial/deadline/Close/cancel не даёт replacement или positive result; Windows
refusal typed WSAECONNREFUSED. Actual App parent/dedup batch/final negative fence;
legacy standalone bool не является App fallback. Strict optional DTO/shared
labels formatter исправляют оба actual consumers NETGUARD+NetworkView; old
backend unknown.3Go3TS2Vue4docs/10initial+2expanded snapshots/manual7diff+1new,
17named+1review-only cached/gofmt3+1+1exit0, без tests/build/types/native/network.
Native/SDK/OS/I-O/resources/wholeUX/W3b/installer/release OPEN;100OPEN56NOT_RUN,
fullPC00…09/old9/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v362-page-interest"></a>

V2-362 (2026-11-10), source/unverified, PC-04/06:
[visible/source interest](../../tasks/PC-04-UI-PRESENTATION.md#pc-v362-page-interest)
подключает existing pauseWhenHidden у9 passive readers/5views. Shared infoFacts
использует тот же visibility adapter и отдельные counts/masks для network и
failover, no-arg compatibility retained. Original pending RPC переживает
pause/unmount без replacement; values/receipts очищаются, resume ждёт свежий
read/loading. Network manual completion guard не выдаёт paused read за failure.
2TS5Vue4docs/11existing snapshots/manual7diff/17named, без product execution.
Root/Settings/createPoller/Go engine/protection неизменны; visibility не native
authority. Backend facts counters/external work/cache unwind/retirement — next.
Native/SDK/OS/wholeUX/resources/W3b/release OPEN;100OPEN56NOT_RUN/fullPC00…09/old9,
gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v363-adapter-facts"></a>

V2-363 (2026-11-10), source/unverified, PC-03/06:
[native adapter facts](../../tasks/PC-06-NETGUARD-OBSERVATION.md#pc-v363-adapter-facts)
отделяет GetNetStats от external HTTP/PowerShell/cache, reuse existing native
route row. Original App query owner/requested700ms; virtual stats purpose не
меняет old netwatch exclusions. Strict source/identity/route metadata/history
reset, legacy compatibility. SSID — existing optional helper/fixed netsh/
16KiB/2s/alias match. Cache short lock/unwind/App-path-mode-epoch/collection-start
TTL and epoch-filtered ISP label; cache is not HTTP resource retirement.
8Go3TS2Vue4docs/17snap16existing1new/manual12diff+1new/13named1cached/gofmt8+3+1+1
exit0; без tests/build/types/native/network. External HTTP Background/body/
transport/workers — actual next gap.100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/
TUN unavailable/schema2 unsupported; native/SDK/OS/wholeUX/W3b/release OPEN.

<a id="pc-v364-egress-owner"></a>

V2-364 (2026-11-10), source/unverified, PC-03/06:
[original egress HTTPS](../../tasks/PC-06-NETGUARD-OBSERVATION.md#pc-v364-egress-owner)
заменяет unowned Background/parallel Transport на synchronous fixed HTTP1/TLS,
existing optional budget/original socket-body owner и pinned SOCKS codec.
Original App/path/intent/control/applied-session cohort, joined watcher и Stop
join до Manager/helper/protection handoff. Status/header/plaintext/body+1 bounds,
no redirect/retry/compression; validated endpoint IP, no invented source URL.
Ordinary decoder Close errors reject replies; unknown native cleanup retains
owner/budget. OS control не fallback, SOCKS HTTPS не DNS/TUN admission.
4Go4docs/8snap6existing2new/manual2diff+2new/17named3cached/gofmt3+2+3+2+1+1
exit0; tests/build/native/network not run. OS-resolution custody/outcome-age
и actual W4c/W3b gaps остаются;100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/
TUN unavailable/schema2 unsupported/native/SDK/OS/wholeUX/release OPEN.

<a id="pc-v365-os-dns"></a>

V2-365 (2026-11-10), source/unverified, PC-03/06:
[original OS address query](../../tasks/PC-06-NETGUARD-OBSERVATION.md#pc-v365-os-dns)
убирает implicit Go resolver из fixed OS-control TCP. Synchronous x/sys
GetAddrInfoW в original egress frame; original list/one Winsock ref and checked
FreeAddrInfoW/WSACleanup, pending/failed retirement blocks replacement/Release.
Native list32/unique public IPv4 addresses8; canonical numeric dial-only gate,
up to8 known-retired address failures plus1SOCKS underexisting10/request deadline.
No alternate resolver/helper/Go fallback; no forced native interruption claim.
3Go4docs/7snap5existing2new/manual1diff+2new/14named3cached/gofmt3+2exit0;
без tests/build/native/network. Outcome/age/W4c/W3b implementation remains next;
100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/TUN unavailable/schema2 unsupported/
native/SDK/OS/wholeUX/release OPEN.

<a id="pc-v366-egress-status"></a>

V2-366 (2026-11-10), source/unverified, PC-03/04/06:
[per-route outcome/age](../../tasks/PC-06-NETGUARD-OBSERVATION.md#pc-v366-egress-status)
добавляет optional schema1 с outcome/reason/attempt/stage/HTTP/start/end и
monotonic elapsed/age. Cached reply не обновляет measurement. No-attempt
budget/busy/skipped и cleanup uncertainty не становятся measured failure;
ok назначает IP parser. Strict facade не downgrade malformed-present schema
в legacy IP/bool. Shared labels в existing Network/Dashboard rows; duplicate
SOCKS row убрана, без cards/navigation/timers/live regions/CSS изменений.
4Go3TS2Vue4docs/13snap12existing1new/manual8diff+1new/16named0cached/gofmt4+1
exit0; no tests/build/types/native/browser/network. Historical metadata не
current health/admission. Далее actual PC-01 W4c/W3b source gaps;100OPEN56NOT_RUN/
fullPC00…09/old9/gatesfalse/TUN unavailable/schema2 unsupported/native/SDK/OS/
wholeUX/release OPEN.

<a id="pc-v367-proxy-writer"></a>

V2-367 (2026-11-10), source/unverified, PC-01 W3b component:
[proxy writer lease](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v367-proxy-writer) reuse existing
filetxn original native lock/ACL/identity across ProxyManager lifecycle. Fixed
primary-token LocalAppData namespace; lease before HKCU, HKEY Close before lease
Close, same lease retirement in opaque foreign-preservation receipt. Unknown
ownership retained; known busy/state uncertainty stop candidate churn.7Go5docs/
12snap10existing2new/manual5diff+2new/16named1cached/gofmt7+1+3exit0; no tests/
build/native/registry/network. Same-domain coordination не authenticated receipt,
whole-HKCU/legacy interoperability или crash recovery. Private receipt/DPAPI/
prewrite journal/process-death/NETGUARD migration — actual remaining W3b code,
HealStale unavailable.100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/TUN unavailable/
schema2 unsupported/native/SDK/OS/W4c/W3b/release OPEN.

<a id="pc-v368-proxy-journal"></a>

V2-368 (2026-11-10), PC-01 **partial source/unverified**:
[encrypted proxy journal](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v368-proxy-journal).
Bounded exact binary tuples + purpose/SID/process creation/nonce/generation;
current-user DPAPI with retained original native allocation/checked LocalFree;
trusted bounded encrypted file under original lease. Prepared before setter,
fresh post-I/O tuple fence, readback confirmation before local state advance,
closed/foreign-preserved before original HKEY/lease retirement. Corrupt/unfinished
old record blocks new apply, no heuristic reset/adoption or plaintext persistence.
8Go5docs/13snap10existing3new/manual5diff+3new/15named3cached/gofmt6+2+2+4+1exit0;
no tests/build/native/registry/network. Full W3b missing process-death/freshness/
rollback/legacy/recovery; HealStale unavailable.100OPEN56NOT_RUN/fullPC00…09/old9/
gatesfalse/TUN unavailable/schema2 unsupported/native/SDK/OS/W4c/release OPEN.

<a id="pc-v369-proxy-process"></a>

V2-369 (2026-11-10), PC-01 **partial source/unverified**:
[prior-process observation](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v369-proxy-process) wired
to unresolved encrypted journal intake. Original limited-rights process handle,
PID/creation/primary SID/zero-time wait, original token/process checked Close;
finite live/reused/exited-recovery/unavailable refusals. Missing/error/reused PID
не доказывают original death; unknown native retirement retains original graph
and lease. No new snapshot/file reset/registry recovery authority.4Go5docs/9snap/
8existing1new/manual3diff+1new/16named3cached/gofmt4+2exit0; no tests/build/native/
registry/network. Actual recovery admission/freshness/antirollback/legacy protocol
still missing; HealStale unavailable.100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/
TUN unavailable/schema2 unsupported/native/SDK/OS/W3b/W4c/release OPEN.

<a id="pc-v370-proxy-subject"></a>

V2-370 (2026-11-10), PC-01 **partial source/unverified**:
[immutable proxy subject](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v370-proxy-subject). Existing
bounded reader now exposes same-read FileInfo after original checked Close,
preserving Read API. DPAPI unresolved record binds original store/lease/slots/
observer/path/FileID/bytes/canonical subject/nonce/generation. Post-observation
recheck failure sticky retains logical owner/lease; no stale reason/new capture/
empty-owner reset. Actual lease validation before reads/encrypted writes.6Go5docs/
11snap10existing1new/manual5diff+1new/15named0cached/gofmt6+2+2exit0; no tests/build/
native/registry/network. This is local consistency, not durable freshness/
antirollback/OS CAS or recovery authority. Full recovery/legacy still absent;
HealStale unavailable.100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/TUN unavailable/
schema2 unsupported/native/SDK/OS/W3b/W4c/release OPEN.

<a id="pc-v371-proxy-anchor"></a>

V2-371 (2026-11-10), PC-01 **partial source/unverified**:
[independent registry anchor](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v371-proxy-anchor).
Purpose-separated bounded DPAPI SID/path/counter/committed+pending file subject;
actual prepare+RegFlushKey → encrypted file+known capture → commit+flush before
state advance. Original HKEY/native query/Close retained on unknown; prior file
checked twice, subject binds anchor, final anchor/system-HKEY/lease Close order.
Only both absent enroll; old unanchored/pending/mismatched records refused, no
automatic migration/reset.9Go5docs/14snap11existing3new/manual6diff+3new/14named5cached/
gofmt9+2+4exit0; no tests/build/native/registry/network. File-only consistency не
whole-hive/profile rollback/global freshness/restore authority. Pending repair/
legacy/native durability/latency/full W3b remain OPEN; HealStale unavailable.
100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/TUN unavailable/schema2 unsupported/
native/SDK/OS/W4c/release OPEN.

<a id="pc-v372-proxy-reconcile"></a>

V2-372, PC-01 **partial source/unverified**:
[pending target metadata completion](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v372-proxy-reconcile).
Exact authenticated target + original file identity/anchor/counter/lease pre/post
checks may finish existing anchor commit. Old/missing/neither remain refused;
no file rewrite, system-proxy restore or process-death inference.5Go5docs/10snap/
manual4diff+1new/16named0cached/gofmt5+1+1exit0; no tests/build/native/network.
Old/missing repair/empty-key/legacy/per-value recovery/full W3b still OPEN;
HealStale unavailable/fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/native/OS/release unverified.

<a id="pc-v373-proxy-stage"></a>

V2-373, PC-01 **partial source/unverified**:
[parent-bound staging/replay](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v373-proxy-stage).
Persist bounded encrypted SID/path/parent-counter/identity/target intent before
anchor prepare; same original slots/lease and guarded journal-only replay for
pending/orphan/committed backup/initial both-absent. WriteNew on missing, exact
source/target checked, no unrelated overwrite or Internet Settings mutation.
9Go5docs/14snap/manual6diff+3new/16named0cached/gofmt9+1+1+1+1+2exit0; no tests/build/
native/network. Empty-key/legacy/mixed writer/no-write/per-value recovery remain;
HealStale unavailable/fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/native/OS/release OPEN.

<a id="pc-v374-proxy-no-write"></a>

V2-374, PC-01 **partial source/unverified**:
[no-write journal retirement](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v374-proxy-no-write).
Only authenticated anchored active/written0/exact original=current=target closes
through retained lease/file/stage/anchor subject and same-session generation+1.
Private shared publisher keeps original writer admission and explicit retirement
caller; target commit/readback checked before fresh snapshot. No prepared-intent
discard, Internet Settings write, process-death claim or restore authority.
4Go5docs/9snap/manual2diff+2new/16named0cached/gofmt4+3+1exit0; no tests/build/native/
network. Per-value crash/startup/manual recovery, empty-key/legacy/mixed writers,
native/OS/release remain. HealStale unavailable/fullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v375-proxy-recovery"></a>

V2-375, PC-01 **partial source/unverified**:
[original-exit per-value recovery](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v375-proxy-recovery).
Actual original-exit + source/lease/file/stage/anchor gated transaction resolves
prepared source/target and restores only recorded own fields through existing
original key/setter/notification owners. Intent before setter, whole tuple
readback, foreign preservation and checked Close before fresh snapshot. Current
recovery writer PID persisted with old session nonce; fresh identity untouched.
9Go5docs/14snap/manual6diff+3new/18named0cached/gofmt9+3+1exit0; no tests/build/
native/network. Unopenable-process/manual/resume admission, empty-key/legacy/mixed
writers and native/OS/release remain. HealStale unavailable/fullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v376-proxy-consent"></a>

V2-376, PC-01 **partial core source/unverified**:
[manual source-bound takeover](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v376-proxy-consent).
Prepare/Confirm/Cancel API, exact original source/lease/file/stage/anchor binding,
one-shot32B token/fixed scope/monotonic2min start TTL; no PID-absence death claim.
Known-retired unknown/reused process can be explicitly transferred; observed
running/unknown callbacks stay refused. Existing recovery/publisher reused with
pending/context/source fences. Cancel/Stop retires only unused original resources.
7Go5docs/12snap/manual5diff+2new/18named0cached/gofmt7+4+3exit0; no tests/build/
native/network. App/Manager/UI wiring and live failed-owner resume still missing;
empty-key/legacy/mixed writers/native/OS/release remain. HealStale unavailable/
fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v377-proxy-ui"></a>

V2-377, PC-01 **partial source/unverified**:
[Manager/App/NetGuard integration](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v377-proxy-ui).
Actual lifecycle/intent/request/Manager ownership, profileless runtime actor with
no Engine, stopped-proxy Stop/composition guards, actual callback return fencing
and early cancellation. Existing NetGuard/DialogFrame/optional generated namespace
UI: prepare/ack/confirm/expiry/close/navigation, retained original pending task,
finite result and no VPN health/cleanup promotion. Historical/new conflict split.
6Go5UI5docs/16snap/manual7diff+4new/19named0cached/gofmt5+2+1+3+1exit0;
no tests/build/types/native/browser/network/generation. Current generated bindings
and executable were not updated; feature remains unavailable there. Live failed
recovery resume/empty-key/legacy/mixed writers/native/OS remain. HealStale
unavailable/fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2
unsupported/release OPEN.

<a id="pc-v378-proxy-resume"></a>

V2-378, PC-01 **partial source/unverified**:
[продолжение удержанной транзакции](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v378-proxy-resume).
Свежий grant на тот же owner/source/HKEY/lease после известных ошибок; ограниченное
удержание прошлых callbacks, монотонные counters и бюджет8 переходов. Неизвестный
исход не допускает retry; отмена grant не подтверждает освобождение транзакции.
8Go5docs/manual6diff+2new/15named0cached/gofmt8exit0 reconfirmed; no execution.
Manager/App/UI resume integration, migration/native/OS и полный PC-00…09 остаются.
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v379-proxy-continuation"></a>

V2-379, PC-01 **partial source/unverified**:
[Manager/App/NetGuard continuation](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v379-proxy-continuation).
Same original Manager/native transaction, fresh source-bound proposal after actual
prior frame return, private known-pause gate. Typed cancellation separates unused
grant from native retirement; bounded two-ID reconciliation and duplicate-cancel
guard preserve original request. Strict resume/reason/ack UI and Close focus.
3Go4UI5docs/manual7diff/12named0cached/gofmt3exit0; no execution/generation.
Migration/native/OS/W4c/wholePC00…09 and current generated/EXE delivery remain open.
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v380-proxy-enrollment"></a>

V2-380, PC-01 **partial source/unverified**:
[initial empty-value enrollment](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v380-proxy-enrollment).
Metadata-only replay after interrupted Create requires exact authenticated initial
no-write stage and original existing HKEY/source custody. Empty value alone grants
nothing. Missing-key replay also requires generation1 active0; encode/DPAPI precedes
Create and final source query precedes Set. No system-settings recovery authority.
4Go5docs/manual3diff+1new/12named0cached/gofmt4+2exit0; no execution/generation.
Legacy/mixed writers/NETGUARD-001/W4c/native/OS/wholePC00…09 remain OPEN;
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v381-proxy-writers"></a>

V2-381, PC-01 **partial source/unverified**:
[running proxy observation](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v381-proxy-writers).
One original native child validates the applied tuple with exact Manager/session
binding. Pure trust sampling stays independent; cleanup precedes original child
join. Late negative findings, foreign preservation and no channel churn remain.
Finite App proxy report is distinct from trust expiry and resource retirement.
6Go6docs/manual5diff+1new/12named0cached/gofmt6+2exit0; no execution/generation.
Root NETGUARD-001/installed-task/DNS migration remains review-required; legacy/W4c/
native/OS/wholePC00…09 OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v382-proxy-expiry"></a>

V2-382, PC-01 **partial source/unverified**:
[early lifetime fault](../../tasks/TH-08-PROXY-OWNERSHIP.md#pc-v382-proxy-expiry).
Follow-up to V381: active Reload/Switch receives immutable negative fault before
whole child return and can attempt existing cleanup. Cancelled join keeps old
trust checks alive, with abnormal join still gated by actual native child done.
2Go5docs/manual2diff/8named0cached/gofmt2+1exit0; no execution/generation.
Early fault is no resource-retirement/forwarding authority. Root NETGUARD-001/
legacy/migration/W4c/native/OS/fullPC00…09 remain;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v383-wfp-call-return"></a>

V2-383, PC-01 **partial source/unverified**:
[original WFP call returns](../../tasks/TH-01-FORWARDING-ADMISSION.md#pc-v383-wfp-call-return).
Existing App/core W4c commit remains; original transaction/query/write/cleanup
return records deny native successors after unknown return. Deferred original
cleanup and pre-cleanup/deferred failed-grant retention prevent lost custody.
5Go5docs/10existing/manual5diff/10named0cached/gofmt5+1exit0; no execution.
No ABI/filter/admission change; native gate false. Root NETGUARD-001/legacy/
migration/native/W4c/fullPC00…09/old9 remain;100OPEN56NOT_RUN/TUN unavailable/schema2 unsupported.

<a id="pc-v384-process-cleanup"></a>

V2-384, PC-01 **partial source/unverified**:
[original process cleanup](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v384-process-cleanup).
Register original child preparation and both parent pipe cleanup steps before
callbacks through existing lifecycleCleanupStep. Interrupted cleanup keeps a
negative result and still attempts independent endpoints; no new native owner,
permanent latch or protocol.1Go5docs/manual1diff/12named0cached/gofmt1exit0.
No execution/native retirement proof; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v385-navigation"></a>

V2-385, PC-04 **partial source/unverified**:
[navigation/layout](../../tasks/PC-04-UI-PRESENTATION.md#pc-v385-navigation).
Reuse registry/NavRail/ContextTabs. Manual tab focus avoids unintended page loads;
hidden panel shells retain ARIA IDs with active-only view lifecycle. Rail labels
collapse at900, shared shell width/status wrapping/inset focus and actual overlay
expanded state. Page transition resets scroll and conditionally restores departed
control focus.5Vue4docs/manual5diff/12named0cached; no product execution/formatter.
Whole browser/WebView2/keyboard/DPI UX remains OPEN; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v386-dialog-search"></a>

V2-386, PC-04 **partial source/unverified**:
[modal/search](../../tasks/PC-04-UI-PRESENTATION.md#pc-v386-dialog-search).
Existing DialogFrame native modal/focus ownership with owner-controlled dismissal,
connected teardown and no nonmodal fallback on open failure. App modal replacement
and CtrlK scope retained; section-aware combobox has one Tab entry, selected-row
reveal, bounded scroll, IME-safe activation and chosen-page focus.
3Vue4docs/manual3diff/14named0cached; no compiler/browser execution/formatter.
WholeUX/native/fullPC00…09/old9 OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v387-action-focus"></a>

V2-387, PC-04 **partial source/unverified**:
[action focus/target](../../tasks/PC-04-UI-PRESENTATION.md#pc-v387-action-focus).
Existing Stop/guards retained. Current-interest-only request target labels one
row without selected/applied authority; aria-disabled guarded actions preserve
focus, Enter repeats blocked, Dashboard Disconnect and Channels row-removal focus
handoff scoped to original controls/body/no modal. Live command feedback.
3source(1TS2Vue)4docs/manual3diff/13named0cached; no product execution/formatter.
WholeUX/native/fullPC00…09/old9 OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v388-source-controls"></a>

V2-388, PC-04 **partial source/unverified**:
[source refresh/action control](../../tasks/PC-04-UI-PRESENTATION.md#pc-v388-source-controls).
Shared native ActionButton reuses tokens/classes and guards disabled click/Enter
without losing mounted focus. Independent per-source Network refresh retains
original pollers/owners; Settings/production diagnostic UI share the same control.
busy does not disable batch stop-next or imply current-call retirement.
5Vue4docs/manual4diff+1new/14named0cached; no formatter/product execution.
WholeUX/native/fullPC00…09/old9 OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v389-action-reuse"></a>

V2-389, PC-04 **partial source/unverified**:
[action/focus reuse](../../tasks/PC-04-UI-PRESENTATION.md#pc-v389-action-reuse).
Dashboard/Channels reuse ActionButton; one root-scoped removed-focus helper serves
3 pages with connected same-root/body/no-modal guards. Explicit Stop heading
handoff covers conditional slot removal; global btn styles honor aria-disabled.
Original command/target/poller ownership retained.
5source(1TS3Vue1CSS)4docs/manual4diff+1new/14named0cached; no product execution.
WholeUX/native/fullPC00…09/old9 OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v390-process-retirement"></a>

V2-390, PC-01 **partial source/unverified**:
[early process failure cleanup](../../tasks/TH-01-STOP-OWNERSHIP.md#pc-v390-process-retirement).
Independent original endpoints attempted on early failure; abnormal Wait is a
negative result, never an exit/release receipt.1Go4docs/manual1diff/gofmt1exit0.
FullPC00…09/old9/native/OS/release OPEN; no tests/build/native/network.

<a id="pc-v391-hy2-receive"></a>

V2-391, PC-06 D0/D3 **partial source/unverified**:
[HY2 receive owner and bounded reassembly](../../tasks/PC-06-HY2-RECEIVE.md).
Windows-only frozen dependency source, serial RX, bounded count/bytes/groups/
assembly and close drain.6Go1mod1origin4docs+73verbatim mirrors; gofmt6+1exit0.
Lower QUIC/TX/carrier/consumer/resource acceptance remains OPEN; no tests/build/native/network.

<a id="pc-v392-hy2-transmit"></a>

V2-392, PC-06 D4 **partial source/unverified**:
[bounded HY2 TX/fragment plan](../../tasks/PC-06-HY2-RECEIVE.md#pc-v392-hy2-transmit).
One staging permit, bounded typed destination, checked initial/retry sizes and
fragment count, exact input/view ownership and a retry ID distinct from its own
old ID.5Go1origin4docs; gofmt5+2+1exit0, no tests/build/native/network.
Lower QUIC/session cancel/carrier retirement/whole-resource acceptance remains OPEN.

<a id="pc-v393-quic-datagrams"></a>

V2-393, PC-06 **partial source/unverified**:
[lower QUIC DATAGRAM ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v393-quic-datagrams).
Paired Windows source forks, pre-copy count/bytes, packer/history credit transfer,
bounded RX copy and HY2 context-aware pending enqueue.9Go1mod2origin4docs plus
176verbatim mirrors; gofmt8+2exit0. No tests/build/native/network.
Atomic session seal/write deadlines, whole-carrier and global/native acceptance OPEN.

<a id="pc-v394-session-deadline"></a>

V2-394, PC-06 **partial source/unverified**:
[session seal and write deadline](../../tasks/PC-06-HY2-RECEIVE.md#pc-v394-session-deadline).
Original scope shares seal/enqueue fence; checked deadline/reusable timer and
retiring Send-call quota integrate actual HY2 constructors/writes/Close.
7Go2origin4docs/gofmt7+1exit0; no tests/build/native/network.
Whole-carrier abort, global resource inventory and native/OS/fullPC acceptance OPEN.

<a id="pc-v395-hop-retirement"></a>

V2-395 PC-06 source/unverified: [hopping lifetime](../../tasks/PC-06-HY2-RECEIVE.md#pc-v395-hop-retirement).
Original socket closes no longer wait for Write/control's lifetime mutex; one
close task/result per socket, repeated Close joins the original hop/receive/public
calls before queue drain. Calls16/receive generations3 before admission/spawn;
control->access order and retained pending/retiring sockets.2Go1origin4docs,
gofmt2+2exit0/manual2diff+1new; no tests/build/native/network. Legacy dial, whole
carrier/offer retention, read deadline reset, Salamander TX, global/native/OS/UX/
release remain open. FullPC00…09 and all previous obligations retained.

<a id="pc-v396-carrier-retirement"></a>

V2-396 PC-06 source/unverified: [original carrier](../../tasks/PC-06-HY2-RECEIVE.md#pc-v396-carrier-retirement).
Explicit one QUIC dial/HTTP3 ClientConn, original raw/QUIC/std Transport independent
close tasks and retained offer/reset outcome. QUIC Closing precedes sendQueue
wait without changing Context; initializer timeout has an atomic phase fence.
Carrier-owned later-hop/UDP context and sealed RX/TX ledgers retain actual credits.
6Go2origin4docs/gofmt5+2+4+2exit0/manual7diff+1new; no tests/build/native/network.
Earlier wrapper socket custody, custom/HTTP3/TCP/lower QUIC/global holders and
fullPC/native/OS/UX/release acceptance remain open; original full scope preserved.

<a id="pc-v397-socket-salamander"></a>

V2-397 PC-06 source/unverified: [socket/Salamander](../../tasks/PC-06-HY2-RECEIVE.md#pc-v397-socket-salamander).
Original UDP reservation/Close before wrapper callbacks, retained partial/abnormal
result and original init/Hop handoff. Salamander immutable ciphertext, continuous
XOR, exact consuming-input cleanup, unmanaged downstream view and bounded staging;
negative abnormal owner prevents reuse.5Go1origin4docs/gofmt5+1+1exit0/manual4diff+
2new; no tests/build/native/network. Hopping read-timeout reset, global/custom/
HTTP3/TCP/lower QUIC/native/OS/fullPC/UX/release acceptance remain open.

<a id="pc-v398-hop-read-deadline"></a>

V2-398 PC-06 source/unverified: [Hop read deadline](../../tasks/PC-06-HY2-RECEIVE.md#pc-v398-hop-read-deadline).
Wrapper-owned monotonic deadline and dequeue fence preserve native receive loops
through timeout/reset. Readiness1/change wake/reusable timer, stale native fault
selection vs hop publication atomic; returned I/O error separate from cleanup.
Controls2/IO16 and inline read metadata prevent reader quota from exhausting
control admission.3Go1origin4docs/gofmt3+2exit0/manual3diff+1new, no tests/build/
native/network. Metadata/Hysteria1/global/HTTP3/TCP/custom/native/OS/fullPC/UX/
release acceptance remains open; original full scope is preserved.

<a id="pc-v399-packet-metadata"></a>

V2-399 PC-06 source/unverified: [метаданные исходного сокета](../../tasks/PC-06-HY2-RECEIVE.md#pc-v399-packet-metadata).
Hop/raw/QUIC sender хранят собственный local address snapshot, допускают ошибку
неизвестных метаданных без заглушки. Metadata2 отдельно от IO16/control2;
RawConn callbacks привязаны к исходному поколению и прежнему Close join.
Все3 sender caller обрабатывают ошибку, новая path queue захвачена явно.
9Go2origin4docs/gofmt9exit0/manual9diff+2new, no tests/build/native/network.
H1 отвергается текущим Windows channel validator, оставаясь compiled/registered;
legacy library lifecycle/reachability остаётся отдельным OPEN gate без активации
нового транспорта и без исключения старого объёма. FullPC00…09/old9, native,
global/custom/HTTP3/TCP/lowerQUIC/OS/wholeUX/release acceptance остаются открытыми.

<a id="pc-v400-http3-ownership"></a>

V2-400 PC-06 source/unverified: [HTTP3 ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v400-http3-ownership).
HTTP3 client task64 admission до spawn/callback registration и общий seal,
actual task/qlogger completion; известный overload отдельно от abnormal cleanup.
HY2 получает original HTTP3 join и ранний QUIC Closing watch до auth. Header
limit64KiB/no auto compression. HTTP3 DATAGRAM reader включается только при
local opt-in И peer SETTINGS; наличие live-дефекта не утверждается.
8Go2origin4docs/manual9diff+1new/gofmt8exit0; tests/build/native/network не было.
Custom TLS transport/default HTTP pool/TCP/lowerQUIC/path/global/native/OS/UX/
release и весь PC-00…09/old9 сохраняются открытыми без сокращения объёма.

<a id="pc-v401-tcp-ownership"></a>

V2-401 PC-06 source/unverified: [HY2 TCP ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v401-tcp-ownership).
Stream owners256 до clone/OpenStream, coreIO512/per-stream8, отдельные controls8/2
и staging8×64KiB. Serial framing сохраняет partial header/payload count и response
parser state через deadlines; первый prefix≤16KiB, large tail borrowed.
Original stream Close после core-call drain, raw abort независимо до joins;
uncertain owners retained.6Go1origin4docs/manual4diff+3new/gofmt6exit0;
tests/build/native/network не было. Advanced promoted controls/early-handshake/
custom provider/lowerQUIC/global/native/OS/wholeUX/release и весь PC-00…09/old9
остаются открытыми без сокращения объёма.

<a id="pc-v402-tcp-halfclose"></a>

V2-402 PC-06 source/unverified: [HY2 half-close](../../tasks/PC-06-HY2-RECEIVE.md#pc-v402-tcp-halfclose).
Actual cached ConnectionManager/CopyConn capability gap: tracked CloseWrite
сохраняет ответ после upload EOF, CloseRead сохраняет write. Modern atomic early
predicates и legacy kickWriteHandshake/Write(nil) path; original FIN outcome.
Empty/known coalescing-pressure path использует bounded header+borrowed payload
на том же QUIC. TLS preflight до socket без замены custom provider на STD.
3Go1origin4docs/manual3diff+1new/gofmt3exit0; tests/build/native/network не было.
Advanced controls/custom hidden/vectorised merge/lowerQUIC/global/native/OS/wholeUX/
release и весь PC-00…09/old9 сохраняются открытыми без сокращения объёма.

<a id="pc-v403-tcp-vectorised"></a>

V2-403 PC-06 source/unverified: [HY2 consuming writes](../../tasks/PC-06-HY2-RECEIVE.md#pc-v403-tcp-vectorised).
Pinned N.ExtendedWriter/VectorisedWriter имеют один original input owner,
общий scalar framing/write mutex, без whole-vector merge.256 slots/4096 pointers/
16MiB exposed backing, включая queued writers; count/bytes до pointer-copy/ref/
ожидания mutex, inline single. Release только после actual return; abnormal
originals/credits retained и poison до unlock.3Go1origin4docs/manual3diff+1new/
gofmt3; no tests/build/native/network. LowerQUIC/advanced/custom/global/OS/wholeUX/
release и весь PC-00…09/old9 остаются открытыми; нет measured performance claim.

<a id="pc-v404-stream-borrow"></a>

V2-404 PC-06 source/unverified: [lowerQUIC Write](../../tasks/PC-06-HY2-RECEIVE.md#pc-v404-stream-borrow).
Ordinary input view снимается под packetizer mutex до Write return, terminal/
deadline проверяются до short-tail copy. Partial/accepted counts и upper HY2
consuming custody сохранены; abnormal body retains originals/sticky local fence
до writer reuse, без recovery-as-success/new tasks/copies/quotas. qrev6/singrev14,
1Go2origin4docs/manual3diff/gofmt1exit0; no tests/build/native/network.
RX/sorter/ACK/reliable/advanced/global/OS/wholeUX/release и весь PC-00…09/old9 OPEN.

<a id="pc-v405-receive-retirement"></a>

V2-405 PC-06 source/unverified: [QUIC receive](../../tasks/PC-06-HY2-RECEIVE.md#pc-v405-receive-retirement).
Original current/queued callback custody at CancelRead/shutdown/consumed EOF/
effective reset; sealed Discard/ref retention/no retry and native call lock fence.
Read/Peek honor reliable prefix, partial Peek retained, smaller duplicate wakes,
abort не выдаётся за EOF. qrev7/singrev15/3Go2origin4docs/manual4diff+1new/gofmt3exit0;
no tests/build/native/network. Parser/map/Push-error intake, RX admission/ACK/
advanced/global/native/wholeUX/release и весь PC-00…09/old9 остаются OPEN.

<a id="pc-v406-stream-intake"></a>

V2-406 PC-06 source/unverified: [STREAM intake](../../tasks/PC-06-HY2-RECEIVE.md#pc-v406-stream-intake).
Original Conn/maps/receive slots, known skip/refusal handoff и consuming sorter
Push; range/1452 до parser copy. Receive-only8192 holders/8MiB до holder/trim
allocation, Pop/current quota до actual return; trim overlap charged, crypto
nil-callback contract сохранён.8Go2origin4docs/manual8diff+2new/gofmt8exit0/qrev8/
singrev16; no tests/build/native/network. First parser/Conn/global budgets,
native aggregation/ACK/advanced/wholeUX/release и весь PC-00…09/old9 OPEN.

<a id="pc-v407-connection-receive"></a>

V2-407 PC-06 source/unverified: [Conn receive budget](../../tasks/PC-06-HY2-RECEIVE.md#pc-v407-connection-receive).
32768 owners/32MiB до первой parser allocation/copy; immutable original release,
общий Conn+stream trim credit и sealed data/parser completion. HY2 original
QUIC close joins WaitForStreamReceiveDataShutdown; negative удерживает original,
known overload не маскируется peer framing fault.13Go2origin4docs/manual13diff+
2new/gofmt13exit0/qrev9/singrev17; no tests/build/native/network. Full control/
native/global/raw/crypto/ACK/advanced/wholeUX/release и весь PC-00…09/old9 OPEN.

<a id="pc-v408-receive-calls"></a>

V2-408 PC-06 source/unverified: [receive calls](../../tasks/PC-06-HY2-RECEIVE.md#pc-v408-receive-calls).
512 Read/Peek+64 independent controls/per-stream8/2/native1 до allocation/waits;
public close wakes readers, original Closing fence и separate loop teardown.
Actual returns включают mutex/gate/sender callbacks; abnormal original retained.
HY2 joins WaitForStreamReceiveShutdown(call+data/parser), data-only API сохранён.
5Go2origin4docs/manual6diff+1new/gofmt5+5+1exit0/qrev10/singrev18; no tests/build/
native/network. Send/global/raw/crypto/ACK/advanced/OS/wholeUX/release и весь
PC-00…09/old9 остаются OPEN.

<a id="pc-v409-send-calls"></a>

V2-409 PC-06 source/unverified: [send-call ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v409-send-calls).
Один shared RX/TX registry с отдельными512 data+64 controls/per-direction8/2;
original native lane1 общий. Send token/gate/mutex admission, close wakeup,
abnormal poison-before-reuse и separate receive/all-call completion в HY2.
7Go2origin4docs/manual8diff+1new/gofmt7+2exit0/qrev11/singrev19; no tests/build/
native/network. ACK/loss/frame original identity/quota/last holders, global/raw/
crypto/OS/wholeUX/release и весь PC-00…09/old9 остаются OPEN.

<a id="pc-v410-stream-send-data"></a>

V2-410 PC-06 source/unverified: [STREAM data ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v410-stream-send-data).
32768 owners/32MiB до первой pool/copy, immutable loan/attempt и history-cell
handoff before pool/retransmission, split overlap/current/outstanding credits.
Post-FIN/0RTT discard cleanup; residual data join требует positive original
loop+all-call quiescence.9Go2origin4docs/manual7diff+4new/gofmt9+2exit0/qrev12/
singrev20; no tests/build/native/network. Raw/crypto/ACK-control metadata/global/
OS/wholeUX/release и весь PC-00…09/old9 остаются OPEN.

<a id="pc-v411-sent-history"></a>

V2-411 PC-06 source/unverified: [sent history/output custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v411-sent-history).
Existing sequential history/SendMode soft guards сохранены; hard pre-getPacket
admission и physical slice cap50000 per number space, pending original до CC,
sticky skip/PTO/registration errors before send. Completed output/GSO batch owner
возвращает known-unsent DATAGRAM credits/raw buffers; alternate path WriteTo
buffer возвращается после actual provider return.6Go2origin4docs/manual6diff+
2new/gofmt6+1exit0/qrev13/singrev21; no tests/build/native/network. Partial packer/
raw receive/crypto/CC/global/OS/wholeUX/release и весь PC-00…09/old9 OPEN.

<a id="pc-v412-packer-buffers"></a>

V2-412 PC-06 source/unverified: [partial packer custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v412-packer-buffers).
Original raw buffer/DATAGRAM entries до callbacks/Pop/pool; ordinary failure/
empty output освобождает owned resources, GSO buffer остаётся borrowed. Unknown
packer result сохраняет originals и запрещает Conn pool return/positive receipt.
SendProbe consumes raw buffer после actual ordinary WriteTo.5Go2origin4docs/
manual6diff+1new/gofmt5exit0/qrev14/singrev22; no tests/build/native/network.
Whole queue/provider/crypto/raw/global/OS/UX/release/fullPC00…09/old9 OPEN.

<a id="pc-v413-send-queue"></a>

V2-413 PC-06 source/unverified: [sendQueue custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v413-send-queue).
Fixed8 pending/worker1/probe1, accepted/error transfer, original raw/DATAGRAM
ownership через actual return и known-unsent drain. Idempotent Close даёт
retired/error отдельно; Conn проверяет early exit/path/recreate/terminal gates.
Short/partial/invalid raw counts не дают silent success/GSO prefix replay.
5Go2origin4docs/manual6diff+1new/gofmt5exit0/qrev15/singrev23; no tests/build/
native/network. Terminal cache/raw receive/crypto/CC/global/OS/UX/release и весь
PC-00…09/old9 остаются OPEN.

<a id="pc-v414-terminal-output"></a>

V2-414 PC-06 source/unverified: [terminal output](../../tasks/PC-06-HY2-RECEIVE.md#pc-v414-terminal-output).
Original close buffer удерживается до actual Write и всех cache-copy returns;
per-Transport immutable retry backing≤1452 отделён от pool. Closed input refs
возвращаются, original views очищаются; expiry использует exact nonzero identity.
4Go2origin4docs/manual5diff+1new/gofmt4exit0/qrev16/singrev24; no tests/build/
native/network. Aggregate cache/timer/ID admission, retry/control-worker joins,
crypto/raw receive/CC/global/OS/wholeUX/release и весь PC-00…09/old9 OPEN.

<a id="pc-v415-closed-cache"></a>

V2-415 PC-06 source/unverified: [closed-cache ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v415-closed-cache).
256 records/384KiB/64 IDs до allocation; original Conn/wrappedConn registration,
actual timer/retry-return fence для credits. Existing control worker сохраняет
retry/stateless views и rawConn read-stop; Close joins worker/cache, unknown
retains originals.5Go2origin4docs/manual5diff+2new/gofmt5exit0/qrev17/singrev25;
no tests/build/native/network. RootClose/init/listener/directWriteTo/raw receive/
CID/crypto/CC/global/OS/wholeUX/release и весь PC-00…09/old9 остаются OPEN.

<a id="pc-v416-transport-lifecycle"></a>

V2-416 PC-06 source/unverified: [Transport lifecycle](../../tasks/PC-06-HY2-RECEIVE.md#pc-v416-transport-lifecycle).
Исходные init/abort/root/Close/recorder outcomes и server receive/send joins;
повторный Close не повторяет cleanup. Direct WriteTo≤64/4MiB exposed spans/
65535 до allocation/init/provider; init waiters≤64. Original refs/credits при
unknown сохраняются; borrowed deadline clear только после positive joins.
6Go2origin4docs/manual6diff2new/gofmt6exit0/qrev18/singrev26. No tests/build/
native/network. Raw receive/queue/CID/constructor/control/crypto/CC/global/OS/
wholeUX/release/fullPC00…09/old9 и все100OPEN56NOT_RUN остаются OPEN.

<a id="pc-v417-raw-receive"></a>

V2-417 PC-06 source/unverified: [raw read/nonQUIC](../../tasks/PC-06-HY2-RECEIVE.md#pc-v417-raw-receive).
Оригинальный basic raw read до pool/provider и Transport input перед callbacks;
known local drops освобождают1 ref. Fixed32 nonQUIC queue, read calls64/4MiB до
allocation/init/context; seal/wake, actual-return credits, original drain/Close
join. Stateless reset без per-packet destroy spawn/token log. qrev19/singrev27;
6Go2origin4docs/manual6diff2new/gofmt6exit0; no tests/build/native/network.
Conn/coalesced/undecryptable/server/0RTT/response queue/output/global/nativeOS/
wholeUX/release/fullPC00…09/old9 и100OPEN56NOT_RUN остаются OPEN.

<a id="pc-v418-conn-input"></a>

V2-418 PC-06 source/unverified: [Conn packet input](../../tasks/PC-06-HY2-RECEIVE.md#pc-v418-conn-input).
64 fixed intake cells/64*20KiB, queue256/close fence; original datagram anchor,
actual working-ref handoff, pending undecryptable32 и terminal/replay drain.
WaitForPacketInputShutdown интегрирован в original Transport/HY2 close; recorder
join/error учитываются. qrev20/singrev28;4Go2origin4docs/manual5diff1new/gofmt4exit0;
no tests/build/native/network. Server/0RTT/response/output/CID/constructor/control/
crypto/CC/global/nativeOS/wholeUX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v419-server-input"></a>

V2-419 PC-06 source/unverified: [server custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v419-server-input).
Fixed64 intake/backing spans, queue1024 seal, original receive/0RTT/handoff/drain;
response input/output/immutable pooled backing/OOB/provider до actual Write/count.
Reply1452 before output allocation/composition; known terminal queue drain,
unknown retained/negative. Failure/Accept/Close не подменяют producer completion;
failed server root сохраняет rows без unstarted-loop wait. qrev21/singrev29;
5Go2origin4docs/manual5diff2new/gofmt5exit0; no tests/build/native/network.
Constructor/context/trace/CID/control/crypto/CC/global/OS/wholeUX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v420-cid-authority"></a>

V2-420 PC-06 source/unverified: [CID/reset authority](../../tasks/PC-06-HY2-RECEIVE.md#pc-v420-cid-authority).
Expected original Conn deletion, same-owner idempotence и no foreign overwrite;
dual-CID server и initial Dial admission. Generated CID публикуется после всех
registrations; pending prefix/sticky failure, handshake/0RTT errors propagated.
Known additional-path CID conflict откатывает original-owned entries и возвращается
через Probe без PATH_CHALLENGE; peer CID retired/original Probe timer stopped.
qrev22/sing30;4Go2origin4docs/manual6diff/gofmt4exit0; no tests/build/native/network.
SameConn epochs/reset migration/constructor/path/control/global/nativeOS/wholeUX/
release/fullPC00…09/old9/100OPEN56NOT_RUN остаются OPEN.

<a id="pc-v421-connection-opening"></a>

V2-421 PC-06 source/unverified: [opening/handshake outcomes](../../tasks/PC-06-HY2-RECEIVE.md#pc-v421-connection-opening).
Fixed server prefix/context/cancel/trace/partial Conn до callbacks, known handoff/
retirement либо retained failure. CID validation до context/trace. Early handshake
errors/observed cancellation идут через общий terminal cleanup; timer Stop и
private close normalization. Public completion/failure остаётся; atomic success
gate проверяют Dial/enqueue/NextConnection, Accept пропускает closing peers.
qrev23/sing31;4Go2origin4docs/manual6diff/gofmt4+4exit0; no tests/build/native/network.
Client constructor/lock/callback/full recreation/path/reset/global/OS/wholeUX/release/
fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v422-client-opening"></a>

V2-422 PC-06 source/unverified: [client opening custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v422-client-opening).
Fixed64 construction cells, original ctx/TLS/config/raw/send/trace/partial Conn;
client callbacks вне metadata locks. CID reservation/map promotion/generic Add
связаны с seal. Known rejected original loop/retirement до cell return; unknown
retained/negative. Promoted-client list не зависит от CID rows; existing retirement
до unlink/recreate, negative worker сохраняет original. Root joins openings до
map/client/trace/deadline cleanup. qrev24/sing32;5Go2origin4docs/manual6diff1new/
gofmt5exit0; no tests/build/native/network. SelfClose/listener/init/provider/full
recreation/path/reset/global/OS/wholeUX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v423-recreation-trace"></a>

V2-423 PC-06 source/unverified: [recreation/recorder custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v423-recreation-trace).
Per-attempt recorder closes при version recreation; close failure блокирует existing
original retirement. Standard Writer Close cached, joins accepted sends; original
Run/current event/drain до final producer close, encoding/native errors preserved.
Stopped-task wakeup даёт blocked senders дойти до negative Close; unknown event/
provider retained, без повторной raw cleanup. qrev25/sing33;2Go2origin4docs/manual4diff/
gofmt2+2exit0; no tests/build/native/network. Event/sink/provider/selfClose/VN/
listener/init/path/reset/global/OS/wholeUX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v424-version-binding"></a>

V2-424 PC-06 source/unverified: [VN request binding](../../tasks/PC-06-HY2-RECEIVE.md#pc-v424-version-binding).
Immutable initial source CID; phase gates и two-CID echo check до versions allocation.
Correct decoder/qlog destination-source order, existing decoder/preference/error/
packet ownership. No new IP/port gate. qrev26/sing34;1Go2origin4docs/manual3diff/
gofmt1exit0. Read-only RFC9000 lookup; no tests/build/native/VPN/live/network changes.
Request binding не является peer/TLS/path/downgrade authentication. FullVNRetry/
hopping/listener/init/provider/path/reset/global/OS/wholeUX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v425-retry-binding"></a>

V2-425 PC-06 source/unverified: [Retry request binding](../../tasks/PC-06-HY2-RECEIVE.md#pc-v425-retry-binding).
Initial CID/version/phase guards, original destination для tag, length proof и
closing recheck; actual received destination qlog. Parser empty-token rejection
и Peek packet numbers уже были корректны и сохранены. qrev27/sing35;
1Go2origin4docs/manual3diff/gofmt1exit0; no tests/build/native/VPN/live/network changes.
Current RFC fetch failed; prior RFC9000/local source used, не live evidence.
FullRetry/crypto/TLS/route/hopping/listener/init/provider/path/global/OS/wholeUX/
release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v426-server-opening"></a>

V2-426 PC-06 source/unverified: [original listener opening](../../tasks/PC-06-HY2-RECEIVE.md#pc-v426-server-opening).
One fixed reservation before init/provider work, retained partial server before
WithPrefix; callbacks вне metadata locks. Constructor не спавнит; publication и
оба existing worker starts разделяют Close seal fence. Root joins actual opening
before snapshot/trace/deadline cleanup; unknown retained/negative. Diagnostic
address capture reused. qrev28/sing36;5Go2origin4docs/manual6diff1new/gofmt5exit0.
No tests/build/native/VPN/live/network changes. Full init/wrap/factory/provider/
selfClose/path/global/OS/UX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v427-cid-init"></a>

V2-427 PC-06 source/unverified: [early CID length admission](../../tasks/PC-06-HY2-RECEIVE.md#pc-v427-cid-init).
Generator retained до single callback; existing0…20/error gate до raw wrap/socket
tuning/queues/workers. Zero/default/custom preserved. GenerateConnectionID checks
negative/>20 before slice/random input; valid crypto/rand unchanged. Seal check
не доказывает atomic native cancellation. qrev29/sing37;
2Go2origin4docs/manual4diff/gofmt2exit0; no tests/build/native/VPN/live/network changes.
Full init/wrap/provider/factory/selfClose/path/global/OS/UX/release/fullPC00…09/old9/
100OPEN56NOT_RUN OPEN.

<a id="pc-v428-setup-cleanup"></a>

V2-428 PC-06 source/unverified: [convenience setup cleanup](../../tasks/PC-06-HY2-RECEIVE.md#pc-v428-setup-cleanup).
Owned TLS/resolve before bind; original Transport before socket acquisition.
Known Dial/Listen errors use exact cached Close; original cause and failed
cleanup retained with exact hidden Transport in private error. Borrowed ownership
preserved, no native retry/global inventory. qrev30/sing38;
3Go2origin4docs/manual4diff1new/gofmt3exit0; no tests/build/native/VPN/live/network changes.
Full init/wrap/abnormal factory/provider/selfClose/global/OS/UX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v429-df-admission"></a>

V2-429 PC-06 source/unverified: [observed DF admission](../../tasks/PC-06-HY2-RECEIVE.md#pc-v429-df-admission).
Wrapper no longer assumes DF without setup; Windows returns OR of actual option
success observations, not unconditional true after both failures. Existing
Control error/one-family behavior/basic/sendConn/handshake PMTU gate retained.
qrev31/sing39;2Go2origin4docs/manual4diff/gofmt2exit0;
no tests/build/native/VPN/live/network changes. Family/readback/hopping/raw-provider/
init/global/fullPMTU/OS/UX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v430-init-custody"></a>

V2-430 PC-06 source/unverified: [original init prefix](../../tasks/PC-06-HY2-RECEIVE.md#pc-v430-init-custody).
One embedded initOnce prefix retains exact socket/address/control/partial wrapper
до callbacks; actual init and abort share captured socket. Known return clears,
abnormal callback retains with existing negative outcome. Both buffer helper
source/write mirror changed together without generation. qrev32/sing40;
5Go2origin4docs/manual7diff/gofmt5exit0; no tests/build/native/VPN/live/network changes.
Full provider/async/nonWindows/global/init/PMTU/OS/UX/release/fullPC00…09/old9/
100OPEN56NOT_RUN OPEN.

<a id="pc-v431-pmtu-reset"></a>

V2-431 PC-06 source/unverified: [PMTU reset/probe identity](../../tasks/PC-06-HY2-RECEIVE.md#pc-v431-pmtu-reset).
Reset/start/due-send share handshake/config/target sendConn DF gate; refused reset
stays disabled. Existing handler identity replaces wrapping uint8 generation;
reset/known ACK/loss invalidate current probe, stale/duplicate callbacks ignored.
Existing search/RTT/loss/payload algorithm preserved. qrev33/sing41;
2Go2origin4docs/manual4diff/gofmt2exit0; no tests/build/native/VPN/live/network changes.
Full family/hopping/path-switch/provider/global/fullPMTU/OS/UX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v432-path-switch"></a>

V2-432 PC-06 source/unverified: [original path switch](../../tasks/PC-06-HY2-RECEIVE.md#pc-v432-path-switch).
One loop-owned candidate retains original/next objects through old Close;
negative/abnormal outcome retained. Target seal/closeErr fence covers final
conn/queue+existing worker publication. Initial/migrated workers notify original
Conn close on Run error/abnormal exit; original queue custody/panic preserved.
qrev34/sing42;2Go2origin4docs/manual3diff1new/gofmt2exit0;
no tests/build/native/VPN/live/network changes. Constructor/target CID/multiowner/
global/family/OS/UX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v433-send-metadata"></a>

V2-433 PC-06 source/unverified: [sender metadata publication](../../tasks/PC-06-HY2-RECEIVE.md#pc-v433-send-metadata).
All3 sender constructors сохраняют original metadata/partial sender в parent
opening до callbacks. Public Conn address/GSO methods используют atomic sender
view; migration node независим от prefix reuse. GSO failure atomic, first-retry
поведение сохранено после caller audit. qrev35/sing43;
6Go2origin4docs/manual8diff/gofmt6exit0; no tests/build/native/VPN/live/network changes.
Full metadata/aliases/provider/target CID/multiowner/global/OS/UX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v434-path-ownership"></a>

V2-434 PC-06 source/unverified: [additional-path ownership](../../tasks/PC-06-HY2-RECEIVE.md#pc-v434-path-ownership).
Original Conn на дополнительном Transport сохраняется независимо от CID/reset
rows. Fixed64 registration cells precede callbacks; target promotion shares seal
fence, known refusal rolls back only new runner before root snapshot. Root joins
registrations, dedups path owners и releases rows after actual Conn retirement.
AddPath gates handshake/closing/nil target before init и повторяет closing gate.
qrev36/sing44;5Go2origin4docs/manual6diff1new/gofmt5exit0;
no tests/build/native/VPN/live/network changes. Path/runner/global/reset migration/
aliases/provider/OS/UX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v435-path-reset"></a>

V2-435 PC-06 source/unverified: [reset tokens across paths](../../tasks/PC-06-HY2-RECEIVE.md#pc-v435-path-reset).
Оба CID manager callbacks используют existing Conn runner interface для reset
Add/Remove на всех transports. Новый target получает active/probing tokens
до owner promotion; known отказ удаляет только его tokens до CID rollback.
Unknown callback/rollback сохраняет original registration prefix. No new worker/
lock/channel/callback fields; qrev37/sing45;4Go2origin4docs/manual6diff/gofmt4+1exit0.
No tests/build/native/VPN/live/network changes. Следующий defect: public Path.Close
caller-side CID retirement; global/aliases/provider/OS/UX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v436-path-close"></a>

V2-436 PC-06 source/unverified: [Path.Close original-loop retirement](../../tasks/PC-06-HY2-RECEIVE.md#pc-v436-path-close).
Caller только помечает existing path row и закрывает abandon под pm mutex;
original loop освобождает CID/reset/frame в existing switch fence до pressure.
cidAssigned не теряется после validation, pending flag пропускает пустой scan.
Original Closing/Context.Done gates и terminal Probe wakeup; active Close refusal
сохранён, Close означает abandonment, не synchronous native retirement.
qrev38/sing46;2Go2origin4docs/manual4diff/gofmt2+2exit0;
no tests/build/native/VPN/live/network changes. Concurrent Probe/validation
channels/probe bounds/global/aliases/provider/OS/UX/release/fullPC00…09/old9/
100OPEN56NOT_RUN OPEN.

<a id="pc-v437-path-probes"></a>

V2-437 PC-06 source/unverified: [bounded concurrent Probe](../../tasks/PC-06-HY2-RECEIVE.md#pc-v437-path-probes).
Fixed64 original calls precede callbacks/attempt/timer allocations; concurrent
waiters share stable channels, cancellation local. Internal rows64/unique queue64/
challenge ring32/shared30s capped backoff; default200ms preserved. Bounded CID
rotation skips a starving queue head. Actual packet-input shutdown joins Probe
calls outside packet locks; unknown prefix negative, wrapped IDs refused.
qrev39/sing47;3Go2origin4docs/manual4diff1new/gofmt3exit0;
no tests/build/native/VPN/live/network changes. Cross-transport response binding/
external Path-context graphs/lifetime runners/global/provider/OS/UX/release/
fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v438-path-reuse"></a>

V2-438 PC-06 source/unverified: [responder existing-path reuse](../../tasks/PC-06-HY2-RECEIVE.md#pc-v438-path-reuse).
Existing row не проходит new-path max3/age/eviction; repeated challenge при full
capacity сохраняет прежнюю row/CID. Прежний proposed incoming-Transport filter
отозван после primary RFC review: это не требуемый дефект acceptance rule.
Actual same-path responder/origin/CID/packetInfo/padding/CC остаётся OPEN.
qrev40/sing48;1Go2origin4docs/manual3diff/gofmt1exit0;
no tests/build/product/native/VPN/live/system network changes; RFC read only.
Global/provider/OS/UX/release/fullPC00…09/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v439-path-replies"></a>

V2-439 PC-06 source/unverified: [routed PATH_RESPONSE output](../../tasks/PC-06-HY2-RECEIVE.md#pc-v439-path-replies).
Client/server-active replies сохраняют input Transport/remote/packetInfo в
existing256 queue; original prefix precedes CID/pack/trace/native write. Padded
reply uses SendModeAny/pacing/normal shared-Conn accounting. Private OOB write
reuses original64/4MiB admission, public WriteTo/default unchanged. Stale/no-CID
route dropped without another-path fallback; unknown prefix remains negative.
qrev41/sing49;7Go2origin4docs/manual8diff1new/gofmt7exit0;
no tests/build/product/native/VPN/live/network changes. Server non-active/antiamp/
data CID/per-path RTT-CC/kernel source/global/provider/OS/UX/release/fullPC00…09/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v440-path-credit"></a>

V2-440 PC-06 source/unverified: [server path credit/MTU](../../tasks/PC-06-HY2-RECEIVE.md#pc-v440-path-credit).
Original bounded3 records/input anchor дают credit3x один раз на datagram.
Original output prefix списывает лимит до immediate/pending CID/pack/trace/write;
minimum limited packet и separate address/MTU follow-up сохраняют default1200.
Fixed32 sent nonce/size history и loss renewal удерживают delayed responses;
switch follows highest non-probing app-data PN including0RTT. q42/s50;
6Go2origin4docs/manual7diff1new/gofmt6exit0. No tests/build/native/live/network
changes. ACK-without-response deadline/backoff, initial/bypass/global amp,
all-challenge queue/CID/privacy/per-path CC/kernel/global/OS/UX/release/fullPC/
old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v441-path-timeout"></a>

V2-441 PC-06 source/unverified: [independent server response clock](../../tasks/PC-06-HY2-RECEIVE.md#pc-v441-path-timeout).
ACK/loss не завершают validation; existing Conn timer/loop ведёт original3 rows,
max3 own probes/address-or-MTU phase,1/2/4 PTO windows/7PTO budget. Conservative
floor/current PTO, saturating deadline, no-CID/credit expiry без past retry alarm.
Small/full phase и original expiry CID custody; active connection retained.
q43/s51;4Go2origin4docs/manual5diff1new/gofmt4exit0;
no tests/build/native/live/network changes. Initial/bypass/global amp/every-
challenge replies/dataCID/privacy/CC/kernel/global/OS/UX/release/fullPC/old9/
100OPEN56NOT_RUN OPEN.

<a id="pc-v442-server-replies"></a>

V2-442 PC-06 source/unverified: [queued normal server replies](../../tasks/PC-06-HY2-RECEIVE.md#pc-v442-server-replies).
Each admitted challenge сохраняет route/scalar pathID в existing256 queue;
original input credit once до frames, no immediate/last-frame output. Shared3x
output, normal reply flight accounting, due own challenge piggyback и known
no-fit response-only repack. Bounded deferred scan без stale-ID rebind/busy
reschedule; original promoted reply CID retained. q44/s52;5Go2origin4docs/manual7
diff0new/gofmt5exit0. No tests/build/native/live/network changes. Actual3 local
service constructors disable path manager; bypass/initial/global amp/queue
terminal/dataCID/privacy/CC/kernel/global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v443-path-retirement"></a>

V2-443 PC-06 source/unverified: [terminal path metadata](../../tasks/PC-06-HY2-RECEIVE.md#pc-v443-path-retirement).
Original final guard очищает response queue/server rows только после actual
loop/drain/sendQueue/input/output/packer receipts; late response admission sealed.
Nested negative publication/original marker усиливают input-shutdown receipt.
Canonical closed-packet release очищает Transport stamp после pool callbacks.
q45/s53;5Go2origin4docs/manual7diff0new/gofmt5exit0;
no tests/build/native/live/network changes. Whole framer/stream/ACK/outgoing/
runner/native/global/bypass/initial amp/dataCID/privacy/CC/kernel/OS/UX/release/
fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v444-framer-retirement"></a>

V2-444 PC-06 source/unverified: [registered stream framer retirement](../../tasks/PC-06-HY2-RECEIVE.md#pc-v444-framer-retirement).
Existing owner/original finisher очищает framer после loop+all6 call classes,
без нового worker/wait/self-join. Sealed late admission, maps/control/ring/flow
release и separate framerFailure входят в whole-call receipt; receive-only scope
сохранён. Uniform lock order/atomic overflow diagnostic; active path без лишнего
registry-lock dispatch. q46/s54;4Go2origin4docs/manual5diff1new/gofmt4exit0;
no tests/build/native/live/network changes. Open-Accept/stream-map/ACK/crypto/
outgoing/raw/runner/global/bypass/dataCID/CC/kernel/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v445-stream-map-calls"></a>

V2-445 PC-06 source/unverified: [Open/Accept lifetime](../../tasks/PC-06-HY2-RECEIVE.md#pc-v445-stream-map-calls).
Original parent registry:512 calls до locks/context/waiter allocation, actual
ctx/child/candidate retained across0RTT, atomic first-close entry/result gate.
Unknown original sticky; running includes outside-lock failure notification.
Separate map-call waiter consumed by HY2 close, six-class scope unchanged.
Outgoing queue bounded/old channel cleared, Err/Done outside child locks with
unwind guards; nil and broken canceled-context errors, available-first Accept.
q47/s55;6Go2origin4docs/manual7diff1new/gofmt6exit0; no tests/build/native/network
changes. Factory/native callbacks/retained-skipped rows/0RTT generation/ACK/
crypto/raw/global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v446-incoming-map-lifetime"></a>

V2-446 PC-06 source/unverified: [incoming factory/close](../../tasks/PC-06-HY2-RECEIVE.md#pc-v446-incoming-map-lifetime).
Whole skipped-ID batch admitted before factory/insertion:4096 rows/16MiB logical
credit per original incoming map, not RSS. Original operation gate, callbacks
outside metadata mutex, sticky batch/candidate/credits and native retiring owner.
Close fences/wakes once before gate wait, no snapshot/retry or naive row clear;
known repeat idempotent. Known suffix/deletion refunds, requested result retained
across Accept/Delete. Wire limits unchanged, local pressure InternalError.
q48/s56;2Go2origin4docs/manual3diff1new/gofmt2exit0; no tests/build/native/network
changes. Outgoing/partial constructors/MAX_STREAMS/0RTT/retained rows/ACK/crypto/
raw/global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v447-outgoing-map-lifetime"></a>

V2-447 PC-06 source/unverified: [outgoing factory/close/FIFO](../../tasks/PC-06-HY2-RECEIVE.md#pc-v447-outgoing-map-lifetime).
Per-map4096 rows/16MiB logical credits до factory; original prefix/retiring stream
retained, callbacks вне metadata mutex, seal/wake before gate wait/no row clear.
Direct pressure declines one call; Sync existing FIFO waits local+peer capacity,
exact head/Delete wake/explicit pressure outcome. Window/reset gate avoids missed
candidate; exact map uncertainty escalates original public registry/root close.
q49/s57;3Go2origin4docs/manual4diff1new/gofmt3exit0; no tests/build/native/network
changes. Control/update callbacks/partial constructors/0RTT/retained rows/ACK/
crypto/raw/global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v448-outgoing-updates"></a>

V2-448 PC-06 source/unverified: [bounded outgoing updates](../../tasks/PC-06-HY2-RECEIVE.md#pc-v448-outgoing-updates).
Window/reset callbacks вне metadata mutex, existing original gate сохраняет
factory/update/close ordering. Snapshot4096 elements/64KiB actual element backing
reserved до allocation; exact original/current/index retained при unknown, no
retry/target substitution. Known close skips uncalled tail/refs cleared before
gate release; native/GC/RSS acceptance отдельная. q50/s58;3Go2origin4docs/manual4diff1new/
gofmt3exit0; no tests/build/native/network changes. Scalar control/partial/params/
0RTT/retained rows/global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v449-stream-map-control"></a>

V2-449 PC-06 source/unverified: [scalar control lifetime](../../tasks/PC-06-HY2-RECEIVE.md#pc-v449-stream-map-control).
STREAMS_BLOCKED/MAX_STREAMS callbacks вне metadata/factory locks: one original
actor/current frame+callback/one monotone pending scalar, admission before
allocation/nonwaiting atomic handoff. Unknown retained/no retry. Close joins
actual control before factory and old-map return/0RTT framer reset. MAX_STREAMS
count bounded before ID arithmetic. q51/s59;5Go2origin4docs/manual6diff1new/gofmt5exit0;
no tests/build/native/network changes. Partial/params/stale0RTT IDs/retained rows/
global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v450-stream-generations"></a>

V2-450 PC-06 source/unverified: [original stream generation](../../tasks/PC-06-HY2-RECEIVE.md#pc-v450-stream-generations).
All4 actual factories bind exact original child sender/Delete and nonzero token.
Data/control/remove compare token under existing framer locks; initial binding
precedes exposure,0RTT clears old data/control getters and swaps generation under
both locks, terminal retirement clears it. Atomic hint not flow barrier.
q52/s60;5Go2origin4docs/manual6diff1new/gofmt5exit0; no tests/build/native/network
changes. Parent params/reset flag/old flow/partial constructors/retained rows/
global/OS/UX/release/fullPC/old9/100OPEN56NOT_RUN OPEN.

<a id="pc-v451-stream-parameters"></a>

V2-451 PC-06 source/unverified: [original stream parameters](../../tasks/PC-06-HY2-RECEIVE.md#pc-v451-stream-parameters).
Immutable scalar snapshots и разные incoming-received/outgoing-applied atomic
heads сохраняют native0RTT/1RTT phases. All4 factories capture one snapshot;
private flow factory no longer reads mutable peerParams. Reset enable условный;
оба wire0RTT checks reject remembered reset-support downgrade. q53/s61;
5Go2origin4docs/manual6diff1new/gofmt5exit0. No tests/build/native/network changes.
Old flow/global counters/other peerParams/partial constructors/rows/OS/fullPC OPEN.

<a id="pc-v452-stream-flow-epochs"></a>

V2-452 PC-06 source/unverified: [original connection-flow epoch](../../tasks/PC-06-HY2-RECEIVE.md#pc-v452-stream-flow-epochs).
Value scopes bind exact owner/nonzero token. All5 stream-to-connection methods
check epoch/access counters under original successful-Reset mutex; stale scope
grants no current credit/window/mutation. Atomic capture is nonwaiting for
window-increase constructor reentrancy. Native send methods share mutex; receive
helpers reused. q54/s62;4Go2origin4docs/manual5diff1new/gofmt4exit0.
No tests/build/native/network changes. Partial/context/rows/other peerParams/
hot-path cost/global/OS/fullPC OPEN. Local source commit3cd1417, no publication.

<a id="pc-v453-stream-construction"></a>

V2-453 PC-06 source/unverified: [partial construction custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v453-stream-construction).
Fixed original map prefix retains known flow/stream/halves/senders/context result
before later calls. All4 factories bind original child; unknown creation/insert
retains prefix/credits/sealed owner, no partial close/retry. Sorter shell precedes
native init; known row transfers graph, incoming prefix clears per item.
q55/s63;9Go2origin4docs/manual10diff1new/gofmt9exit0. No tests/build/native/network
changes. Hidden context/provider internals/actual cleanup/rows/other peerParams/
global/OS/fullPC OPEN.

<a id="pc-v454-stream-shutdown"></a>

V2-454 PC-06 source/unverified: [native shutdown results](../../tasks/PC-06-HY2-RECEIVE.md#pc-v454-stream-shutdown).
Original per-half receipts и explicit private close errors проходят через все4
child maps в Conn. Send context отменяется с причиной вне send mutex после wake;
receive failures не скрываются. Failed0RTT cleanup сохраняет старые maps и не
сбрасывает framer/flow. Terminal error/negative loop receipt сохраняются, без
нового panic на ordinary error. q56/s64;10Go2origin4docs/manual11diff1new/
gofmt10exit0; no tests/build/native/network changes. Actual holder/row retirement,
flow-reset transition/other peerParams/global/OS/fullPC OPEN.

<a id="pc-v455-stream-reset"></a>

V2-455 PC-06 source/unverified: [original0RTT reset](../../tasks/PC-06-HY2-RECEIVE.md#pc-v455-stream-reset).
Old/candidate sets удержаны в original prefix. Child close/native flow Reset/
build проходят вне parent metadata lock; flow refusal до candidate publication.
Strict framer old/next identity, parent commit после phases/close recheck и
negative gate для pending/failed UseResetMaps/NextConnection. No retry/invented
rollback. q57/s65;4Go2origin4docs/manual5diff1new/gofmt4exit0; no tests/build/native/
network changes. Actual terminal holder/row retirement/global/OS/fullPC OPEN.

<a id="pc-v456-map-retirement"></a>

V2-456 PC-06 source/unverified: [terminal map metadata retirement](../../tasks/PC-06-HY2-RECEIVE.md#pc-v456-map-retirement).
External aggregate joins original input/map/receive/send(all6/framer), then one
retained finalizer/fixed3 sets preflights and clears only metadata refs/credits.
Concurrent callers join same result; negative retains originals/no retry.
Empty unpublished candidate has separate zero-row original close. Actual HY2/
native Transport integrated. q58/s66;7Go2origin4docs/manual7diff2new/gofmt7exit0;
no tests/build/native/network changes. Raw/ACK/crypto/external Stream/Context/
GC/RSS/OS/global/fullPC OPEN.

<a id="pc-v457-peer-parameters"></a>

V2-457 PC-06 source/unverified: [peer snapshots/DATAGRAM admission](../../tasks/PC-06-HY2-RECEIVE.md#pc-v457-peer-parameters).
Observed и outbound scalar heads заменяют public wire reads; restore/receive/apply/
rejected0RTT timing сохранён. Actual queue publication/reserve/enqueue/Peek/history
transfer сверяют exact whole-frame limit под одним mutex; broadcast будит всех
waiting producers. Unsent queue retained through rejection, fresh apply revalidates.
q59/s67 unchanged consumer;3Go2origin4docs/manual4diff1new/gofmt3exit0;
no tests/build/native/network changes. Initial Conn/raw/ACK/crypto/MTU/global/OS/
UX/release/fullPC OPEN.

<a id="pc-v458-connection-construction"></a>

V2-458 PC-06 source/unverified: [initial Conn construction custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v458-connection-construction).
Original Conn/prefix удерживает partial crypto/map/stream/ACK/congestion shells
до callbacks/native init; старые wrappers используют общий initializer. Complete
marker после known graph transfer и actual client/server identity/completion
gate до native use. Unknown opening retained/no partial Close/retry/worker.
q60/s68 unchanged consumer;11Go2origin4docs/manual12diff1new/gofmt11exit0;
no tests/build/native/network changes. Opaque/pre-return/Retry/raw/ACK/crypto/
global/OS/UX/release/fullPC OPEN.

<a id="pc-v459-packet-transitions"></a>

V2-459 PC-06 source/unverified: [native packet/congestion transitions](../../tasks/PC-06-HY2-RECEIVE.md#pc-v459-packet-transitions).
Retry и migration держат original old/candidate/current prefixes; probe drain
исправляет skipping compacted rows. Bounded shared congestion change публикует
candidate под mutex после foreign callback вне lock, с actual Closing fence.
Public void setter closes on refusal/abnormal setup; native typed errors и
negative terminal status integrated. q61/s69 unchanged consumer;
6Go2origin4docs/manual6diff2new/gofmt6exit0; no tests/build/native/network changes.
Custom-call joins/aliasing/raw/ACK/crypto/global/OS/fullPC OPEN.

<a id="pc-v460-congestion-calls"></a>

V2-460 PC-06 source/unverified: [original congestion setter receipt](../../tasks/PC-06-HY2-RECEIVE.md#pc-v460-congestion-calls).
One Conn cell сохраняет actual setter до last defer; close atomic/Closing/seal
fence, unknown negative/no retry. Additive external receipt соединяет original
packet/native-loop/public-call completion и ACK status, отдельно от producer-loop
receipt. Known returned unpublished Closing refusal не превращается в unknown.
Actual Transport/HY2 join до map/path retirement. q62/s70 changed consumer;
6Go2origin4docs/manual7diff1new/gofmt6exit0; no tests/build/native/network changes.
All-custom methods/aliasing/raw/ACK/crypto/global/OS/fullPC OPEN.

<a id="pc-v461-datagram-retirement"></a>

V2-461 PC-06 source/unverified: [DATAGRAM data retirement](../../tasks/PC-06-HY2-RECEIVE.md#pc-v461-datagram-retirement).
Original live-entry ledger and separate producer/queue/history refs preserve
owned payloads through original work return. One external receipt joins native
packet loop and admitted producers before complete bounded history-data
preflight/release; unknown retained/negative/no retry. Actual Transport/HY2 join
after congestion before map/path retirement. q63/s71 changed consumer;
5Go2origin4docs/manual6diff1new/gofmt5exit0; no tests/build/native/network/Git.
Scope/public-call/receive/ACK/control/history/crypto/global/OS/fullPC OPEN.

<a id="pc-v462-packet-history"></a>

V2-462 PC-06 source/unverified: [sent-history metadata](../../tasks/PC-06-HY2-RECEIVE.md#pc-v462-packet-history).
External aggregate joins actual native/public setter/STREAM/DATAGRAM/map receipts;
one fixed-prefix finalizer preflights all spaces before clearing owned pointer
cells. No recycled packet/frame access or ACK/loss/pool callbacks; unknown
retained/negative/no retry. Actual Transport/HY2 consume aggregate. q64/s72;
6Go2origin4docs/manual6diff2new/gofmt6exit0. No tests/build/native/network/Git;
crypto/control/retransmission/received-history/provider/OS/fullPC OPEN.

<a id="pc-v463-crypto-queues"></a>

V2-463 PC-06 source/unverified: [TLS completion и crypto queues](../../tasks/PC-06-HY2-RECEIVE.md#pc-v463-crypto-queues).
Original native Close error and completion receipt separated; V462/TLS/framer
joins and fixed3-stream identity/sorter preflight precede local head retirement.
Actual Transport/HY2 use aggregate; unknown retained/no retry, no byte/key wipe
or pool/ACK/loss callbacks. q65/s73;8Go2origin4docs/manual8diff2new/gofmt8exit0;
18reviewed+1SDK/261cache203verbatim. No tests/build/native/network/Git;
TLS internal/received-history/byte budgets/provider/OS/fullPC OPEN.

<a id="pc-v464-received-history"></a>

V2-464 PC-06 source/unverified: [received ACK history](../../tasks/PC-06-HY2-RECEIVE.md#pc-v464-received-history).
Original pointer-bound receiver; existing history aggregate now joins received
metadata after native/public-call/data/map/sent receipts. Fixed3 tracker/range/
floor preflight before internal heads retire; no cached ACK object mutation or
fake delivery. Unknown retained/no retry; unchanged Transport/HY2 crypto consumer.
q66/s74;6Go2origin4docs/manual7diff1new/gofmt6exit0;16reviewed0SDK/
261cache202verbatim. No tests/build/native/network/Git; TLS internal/byte budgets/
provider/OS/fullPC OPEN.

<a id="pc-v465-crypto-events"></a>

V2-465 PC-06 source/unverified: [owned TLS event data](../../tasks/PC-06-HY2-RECEIVE.md#pc-v465-crypto-events).
Write/ticket copies before SDK borrow expiry;128 pending slots/8MiB queued clone
budget and explicit pressure, Go1.26 error-event propagation. Fork minimum now
matches Windows1.26; no dependency version/resolver change. Native event metadata
retires after actual loop/data/history/TLS/local sorter receipts; unknown retained,
no retry/byte/key wipe. q67/s75 unchanged consumer;6Go1manifest2origin4docs/
manual8diff1new/gofmt6exit0. No tests/build/native/network/Git; TLS keys/stream-
crypto/retransmission lifetime budgets/provider/OS/fullPC OPEN.

<a id="pc-v466-key-references"></a>

V2-466 PC-06 source/unverified: [local key references](../../tasks/PC-06-HY2-RECEIVE.md#pc-v466-key-references).
Existing native/public/data/history/TLS/event/local-sorter aggregate дополнен
одним original-shell/exact-TLS retirement pass. Fixed typed prefix, no callbacks/
waits/byte wipe; unknown retained/no retry. Local key/config/parameter/RTT/trace
aliases отпускаются, TLS Conn/ConnectionState и historical Used0RTT сохраняются.
Serial key APIs/late event dispatch закрыты; native derivation уже owns outputs.
q68/s76 unchanged consumer;6Go2origin4docs/manual7diff1new/gofmt6exit0.
No tests/build/native/network/Git на этом срезе; crypto-stream/retransmission
lifetime budgets/provider/TLS internals/OS/fullPC remain OPEN.

<a id="pc-v467-crypto-write-storage"></a>

V2-467 PC-06 source/unverified: [crypto transmit storage](../../tasks/PC-06-HY2-RECEIVE.md#pc-v467-crypto-write-storage).
Per original stream:8MiB accepted lifetime/4096 writes/16MiB backing/fixed64 buffers,
admission before make/copy and clamped growth. No Pop/Finish/ACK/timeout refund;
Initial scrambling preserved, server ticket Write error propagated. All3 original
stores preflight before event/key retirement; local references clear only after
aggregate receipts, unknown prefix retained/no retry/no byte wipe.
q69/s77 unchanged consumer;4Go2origin4docs/manual5diff1new/gofmt4exit0.
No tests/build/native/network/Git; retx tail copies and queue accounting/global/
TLS/provider/native/OS/fullPC remain OPEN.

<a id="pc-v468-retransmission-storage"></a>

V2-468 PC-06 source/unverified: [retransmission storage](../../tasks/PC-06-HY2-RECEIVE.md#pc-v468-retransmission-storage).
Original queue admission4096 entries/16MiB views/1MiB frame, clamped metadata;
4096 lifetime split copies/16MiB backing before allocation. Stored pending credit
refunds are separate from retained split backing; emitted views clamped. Void-
OnLost errors reach native lifecycle/packer gates, unknown prefix retained/no retry.
Initial/Handshake key drop retires pending heads; metadata-only cleanup after
native/history/TLS/event/key, no frame callbacks/pool/byte wipe. Wire API unchanged.
q70/s78 unchanged consumer;7Go2origin4docs/manual7diff2new/gofmt7exit0.
No tests/build/native/network/Git; receive/framer/provider/TLS/global/OS/fullPC OPEN.

<a id="pc-v469-crypto-receive-storage"></a>

V2-469 PC-06 source/unverified: [CRYPTO receive storage](../../tasks/PC-06-HY2-RECEIVE.md#pc-v469-crypto-receive-storage).
Native borrowed parser view сохраняет old owning API. Existing16KiB range/phase
проверяется до lazy16KiB data+2KiB bitmap allocation; generic crypto sorter/trim
clones исключены. Accepted bytes не переписываются, TLS views immutable/capped;
Finish сохраняет backing, unknown blocks readiness/native receipt/no retry.
Bitmap-only preflight и reference release после native/history/TLS/event/key.
q71/s79 unchanged consumer;7Go2origin4docs/manual8diff1new/gofmt7exit0.
No tests/build/native/network/Git; segmentation/overlap/protocol/fixtures и
framer/control/provider/TLS/global/OS/fullPC remain OPEN.

<a id="pc-v470-framer-control-storage"></a>

V2-470 PC-06 source/unverified: [control-frame storage](../../tasks/PC-06-HY2-RECEIVE.md#pc-v470-framer-control-storage).
Generic framer: existing16384 count plus16MiB pending payload views/1MiB frame,
shared immutable entry/layout with retx, оба blocked-frame append через admission,
очистка снятых references. Original wake signal и atomic failure соединены с
native loop/readiness/send и regular packer gates. Unknown operation сохраняется;
Append unlocks, bounded 0RTT/filter и metadata-only retirement не маскируют ошибку.
Explicit concrete-framer assertion исправляет frameSource seam; aggregate
проверяет exact original identity. q72/s80 unchanged HY2;9Go2origin4docs/
manual10diff1new/gofmt9exit0. No tests/build/native/network/Git.
Token/path/stream-control/packet/qlog/provider/global/OS и fullPC остаются OPEN.

<a id="pc-v471-client-token-storage"></a>

V2-471 PC-06 source/unverified: [client token storage](../../tasks/PC-06-HY2-RECEIVE.md#pc-v471-client-token-storage).
Native borrowed NEW_TOKEN parser и scalar debug/qlog metadata; owning parser и
old raw qlog alias сохранены. Original optional store/key/Pop/clone/Put cell
ограничивает lifetime copies до64/1MiB/16KiB token перед allocation. Default LRU
имеет64 origins/16 slots/1MiB views+owned keys, safe zero sizes и immutable credit.
Capacity не отвергает peer framing и не включает cache. Unknown state удерживается,
native/readiness/aggregate gated; local aliases снимаются после original receipt.
q73/s81 unchanged HY2/no TokenStore;9Go2origin4docs/manual10diff1new/gofmt9exit0.
No tests/build/native/network/Git. Issuer/Retry/header/other qlog/provider/global/
native/OS/fullPC остаются OPEN.

<a id="pc-v472-token-issuer"></a>

V2-472 PC-06 source/unverified: [token issuer](../../tasks/PC-06-HY2-RECEIVE.md#pc-v472-token-issuer).
Собственный issuer ограничен1024 encrypted/plain976 до crypto и получает один
reserved output; address copy/marshal inputs ограничены после provider return.
Decoded address/CID/RTT validation предшествует CID parsing/duration arithmetic;
server future age отвергается после actual address validation. Native Initial/
Retry header diagnostics не передают token bytes; opaque client policy, raw
standalone qlog и Retry integrity/phase остаются. q74/s82 unchanged HY2;
5Go2origin4docs/manual7diff0new/gofmt5exit0. No tests/build/native/network/Git.
Retry parser/Initial fit/provider/String/global/native/OS/fullPC OPEN.

<a id="pc-v473-packet-token"></a>

V2-473 PC-06 source/unverified: [packet token admission](../../tasks/PC-06-HY2-RECEIVE.md#pc-v473-packet-token).
Native Conn borrowed Initial/Retry parser сохраняет owning/server paths. Existing
header/CRYPTO length API проверяет configured Initial-fit с PN4/CID20/AEAD16/offset
headroom перед clone/mutation. Original packer/stream/retx identity и separate
single-Retry owned slot<=1452 удерживаются через reset/key/CID callbacks; nonfit
не меняет state, unknown retained/no retry/no native-ready. Metadata checked
before aggregate reference release. q75/s83 unchanged HY2;4Go2origin4docs/
manual5diff1new/gofmt4exit0. No tests/build/native/network/Git.
Server pre-parser/whole-flight/MTU/provider/global/native/OS/fullPC OPEN.

<a id="pc-v474-server-header"></a>

V2-474 PC-06 source/unverified: [server header custody](../../tasks/PC-06-HY2-RECEIVE.md#pc-v474-server-header).
Vendored listener использует borrowed pre-parser и original header slot;
rejected messages содержат inline Header/presence/raw ref. Fixed4/4/8 queues
имеют общий retained handoff, без token clone/shared mutable Header pointer.
Views снимаются перед pool return; metadata/kind/range и empty prefixes gated
до callbacks/positive receipts. Unknown retained/no retry, raw panic dump removed.
Actual HY2 unchanged/no listener activation; q76/s84;3Go2origin4docs/
manual5diff0new/gofmt3exit0. No tests/build/native/network/Git.
Queue/private fixtures/wire/concurrency/provider/global/OS/fullPC OPEN.

<a id="pc-v475-http3-fields"></a>

V2-475 PC-06 source/unverified: [HTTP3 field accounting и inventory](../../tasks/PC-06-HY2-RECEIVE.md#pc-v475-http3-fields).
Без qlog не создаётся field collector; decoded name+value+32 проверяется до
map/collector retention, включая wire trailers с исходным configured limit.
Actual HY264KiB headers, unread-body Close/no trailer parsing; SETTINGS8KiB/
1xx/task64 уже ограничены. QPACK field allocation предшествует этому учёту:
нет pre-decoder/global/RSS/native гарантии. q77/s85;3Go2origin4docs/gofmt3.
No tests/build/native/network/Git; wire/private/fullPC/all old gates OPEN.

<a id="pc-v476-http3-datagram"></a>

V2-476 PC-06 source/unverified: [HTTP3 DATAGRAM storage](../../tasks/PC-06-HY2-RECEIVE.md#pc-v476-http3-datagram).
Lazy32 existing ringbuffer/zeroing pop и immutable1MiB queued capacity charge,
native original prefix учтён до удержания; queued drain сохранён. Lazy close
broadcast и data handoff для ожидающих, nil/cancel до pop; без clone/new worker.
Actual HY2 auth HTTP3 datagrams disabled. Map/external/native/global/RSS/time
не ограничены этим срезом. q78/s86;2Go2origin4docs/gofmt2. No tests/build/
native/network/Git; wire/private/fullPC/all old gates OPEN.

<a id="pc-v477-http3-registry"></a>

V2-477 PC-06 source/unverified: [HTTP3 original registry](../../tasks/PC-06-HY2-RECEIVE.md#pc-v477-http3-registry).
Independent map64/duplicate/closing gate до wrapper/callback; map/WG owner до
watcher, exact-owner removal. Общий Add/seal/Wait mutex; server rawConn pointer
сохраняет original Context callback owner вместо value copy. No purge on Closing;
data/native/external last-holder отдельно. q79/s87 HY2 unchanged/auth uses registry;
4Go2origin4docs/gofmt4. No tests/build/native/network/Git; wire/private/fullPC OPEN.

<a id="pc-v478-http3-join"></a>

V2-478 PC-06 source/unverified: [HTTP3 registry join](../../tasks/PC-06-HY2-RECEIVE.md#pc-v478-http3-join).
WaitForShutdown ждёт original registry WG даже без qlog; task drain/Closing
предшествуют seal/wait, original qlogger join сохранён. Map/clear/failure predicate
под original mutex; unknown clear удерживает исходный slot и sticky error.
Known request errors cancel обе стороны до reqDone; original error/join сохранены.
HY2 negative owner retention unchanged. Raw borrowers могут удерживать join;
нет purge/timeout/native cleanup. q80/s88;4Go2origin4docs/gofmt4.
No tests/build/native/network/Git; data/native/global/private/fullPC OPEN.

<a id="pc-v479-http3-native"></a>

V2-479 PC-06 source/unverified: [native stream receipt → HTTP3 metadata](../../tasks/PC-06-HY2-RECEIVE.md#pc-v479-http3-native).
Один finalizer join-ит existing original native map/call/data aggregate. Только
positive/cause/snapshot64 допускают local completion/exact remove/broadcast;
queued drain сохранён, native Close/Cancel/empty callback не вызываются.
Repeated Wait использует original result; unknown retains prefix, no retry.
q81/s89 HY2 unchanged;4Go2origin4docs/gofmt4. No tests/build/native/network/Git;
provider blocking/external/global/native/OS/private/fullPC gates OPEN.

<a id="pc-v480-hy2-udp-close"></a>

V2-480 PC-06 source/unverified: [HY2 UDP original close](../../tasks/PC-06-HY2-RECEIVE.md#pc-v480-hy2-udp-close).
Retained parent UDP map/current + child drain/group references; shared shutdown/
destroy error joined, no unknown retry or credit reset. Close-phase budget failure
notifies original client outside RX locks; Scope.Close stays seal only. Existing
resource ceilings preserved, q82 metadata-only/s90;5Go2origin4docs/gofmt5.
No tests/build/native/network/Git; decoder/read/feed/assembly/provider/global/OS/
private/fullPC gates OPEN.

<a id="pc-v481-hy2-rx-prefix"></a>

V2-481 PC-06 source/unverified: [original RX/input/assembly](../../tasks/PC-06-HY2-RECEIVE.md#pc-v481-hy2-rx-prefix).
Carrier task-owned raw/charge/message, serialized child input/output, defrag
assembly charge/message до original return; unknown retained/no retry. Parent
idle check после task join, failure notification после RX unlock. q83 metadata-
only/s91;7Go2origin4docs/gofmt7. No tests/build/native/network/Git; reader/Wait/
output/accounting/native/provider/global/OS/private/fullPC gates OPEN.

<a id="pc-v482-hy2-readers"></a>

V2-482 PC-06 source/unverified: [original UDP readers/output](../../tasks/PC-06-HY2-RECEIVE.md#pc-v482-hy2-readers).
Fixed8/session512/carrier, budget session roots after public destroy and original
read settlement; public Close seal only. Message/output/charge retained on unknown,
options snapshot/bounded allocation, Wait full-fit/n and nil error output.
q84 metadata-only/s92;5Go2origin4docs/gofmt5. No tests/build/native/network/Git;
TX staging/pack/fragment/release custody/native/provider/global/OS/fullPC OPEN.

<a id="pc-v483-hy2-send-owners"></a>

V2-483 PC-06 source/unverified: [admitted TX owners](../../tasks/PC-06-HY2-RECEIVE.md#pc-v483-hy2-send-owners).
Eight fixed send slots, original input/message/current fragment/pack/release
roots; checked retained cap+start within128KiB. Unknown no cleanup/replay/refund,
client callback after unlock. Sequential fragments and original bounded retry.
q85 metadata-only/s93;3Go2origin4docs/gofmt3. No tests/build/native/network/Git;
rejected-buffer/preflight/constructor/control/native/global/OS/fullPC OPEN.

<a id="pc-v484-hy2-constructor"></a>

V2-484 PC-06 source/unverified: [original UDP constructor](../../tasks/PC-06-HY2-RECEIVE.md#pc-v484-hy2-constructor).
One pending candidate before native init; publication fenced with closing under
udpAccess->lifeAccess. Caller/parent share one outside-lock unpublished abort,
unknown retained/no replay, positive-only root drop. q86 metadata-only/s94;
5Go2origin4docs/gofmt5. No tests/build/native/network/Git; control/post-Close
deadline timer/LocalAddr/rejected-buffer/preflight/native/global/OS/fullPC OPEN.

<a id="pc-v485-hy2-controls"></a>

V2-485 PC-06 source/unverified: [UDP controls/timer/snapshot](../../tasks/PC-06-HY2-RECEIVE.md#pc-v485-hy2-controls).
Independent control slots8/carrier2/session, post-lock seal recheck, final timer
clear under session mutex; unknown retained/no replay/callback after unlock.
LocalAddr constructor snapshot, no native getter. q87 metadata-only/s95;
5Go2origin4docs/gofmt5. No tests/build/native/network/Git. Next PC01 W4c/helper/
WFP source, TUN unavailable; PC06 and fullPC/native/OS/rejected/preflight OPEN.

Абсолютный запрет владельца на тестирование/сборки/исполнение сети сохраняется.

1. ПК-владельцу: закончить W4c original-helper/WFP admission, сохранив закрытый
   TUN gate до доказанной реализации; рядом уточнить egress/leak contract.
2. Trust-владельцу после регистрации: заморозить первый callable C1 с clock,
   progress/reset и полезным пересечением лимитов; не переносить весь Windows-код.
3. Серверному оператору: согласовать исправленный шаблон и applied config,
   точные TLS/channel facts и отдельную проверку пути без скрытого upstream VPN.
4. UI можно дорабатывать независимо в пределах уже существующих подтверждённых
   полей; отсутствующее показывать как unavailable/unknown.

План подготовлен статическим анализом. Новых тестов, сборок, сетевых проверок,
изменений продукта, VPS или Cloudflare при составлении не выполнялось.
