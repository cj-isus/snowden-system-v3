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
  продолжает PC-00/01/03/04 в исходниках. Существующие guards процессов,
  Stop, транзакций и обновлений сохраняем. Старые успешные проверки нельзя
  переносить на изменившийся код.
- TUN пока намеренно не получает общий допуск: существующий
  native boundary — `windows/tunnel_boundary.go` возвращает
  `ErrForwardingNotAdmitted`. Это ограничение безопасности, которое нужно
  завершить полноценным протоколом, а не убрать проверку.
- Отказ от небезопасной композиции ShadowTLS и Windows-local TUF/storage/progress
  уже описаны в текущих исходниках. Не назначаем их повторно как отсутствующие
  функции; проверяем оставшиеся границы поведения и миграции.
- Общий trust SDK ещё не опубликован. `shared/adaptive` содержит отдельные
  исходники чистой policy, но не принятую интеграцию в VPN.
- Интерфейс уже имеет шесть разделов, десять страниц и подходящие компоненты.
  Развиваем их, сохраняя существующий Vue/Wails API и идентификаторы навигации.

### Что изменили сведения о сервере

Предоставлены сообщения о развёртывании sing-box/nginx и использовании
`snowdensys.dpdns.org` через Cloudflare на VPS `78.17.84.209`. Это датированные
сведения оператора, не текущая независимая аттестация сервера.

1. В репозиторном [серверном шаблоне](../../../configs/singbox/server-combined.json)
   действительно осталось правило `hijack-dns` без match-условий. Оператор
   связывает с ним поломку передачи и сообщает об удалении на сервере. Шаблон
   и фактически применённая конфигурация должны быть сведены, чтобы следующий
   deploy не вернул дефект. Runtime-эффект здесь заново не проверялся.
2. Тест SOCKS → внешний HTTP endpoint «3/3» позже уточнён как проведённый через
   Amnezia/WARP. Он подтверждает только сообщённую составную цепочку, не прямую
   доступность с мобильной сети и не готовность Snowden EXE.
3. Доступный HTTPS `/ip` и таймаут WS на том же домене сужают область поиска,
   но не устанавливают единственную причину. Гипотезу DPI нельзя превращать
   в факт или гарантировать исправление заменой IP/транспорта.
4. Для порта 8443 сообщалась ошибка 526. Такой вариант не считается готовым
   резервом. REALITY и HY2 также требуют своей проверки клиент–сервер.

Happ на рабочем ПК сохраняется включённым. Проверку другой сетевой среды нужно
проводить отдельно и по разрешению, не меняя текущий доступ руководителя.

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
Маршрут исполнения и открытые границы — реестр §5.11 — `docs/PLAN.md#pc-plan-execution`.
V2-199 уточняет сравнение egress и presentation в
PC-03 — `docs/tasks/PC-03-EGRESS-SEMANTICS.md`; W4c и его native gate этим не закрыты.
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
