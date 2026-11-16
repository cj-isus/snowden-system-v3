# PC-04 — presentation существующих страниц Windows

Канонические требования и статусы — PC-PLAN §PC-04 и PLAN §5.11. Этот документ
сохраняет конкретный source срез и границы доказательств, не заменяет реестр.

<a id="pc-v305-ui-status"></a>

## V2-305 — Главная, Проверки и Локальное восстановление

**Source/unverified, 2026-11-08.** Полный PC-00…09 и девять TH этапов активны;
100 OPEN/56 NOT_RUN. Абсолютный запрет tests/fixtures/build/vet/typecheck/browser/
product parser/SDK/native/registry/network сохранён. Приложение не запускалось.

### Найденное и переиспользованное

Vue3/TypeScript/Wails, существующие NavRail/ContextTabs/navigation, API backend.ts
и contract.ts, backendState/lifecycle/infoFacts/diagnostics/polling, ProbeReportView,
TestCard, PageHeading/HintBox, общие dark/orange tokens и focus-visible.
Новый router/store/UI framework, API fields и ручные Wails bindings не вводились.
Skills repo-recon/frontend-postgres/modern-web-guidance применены адресно;
локальные guides accessibility (semantic headings/live regions) и responsive-design
влияют на разметку/переносы/размер текста, не доказывают доступность или дизайн.

### Управление соединением

Existing backendState.primaryAction читает reactive stopping, busy/switching и
native lifecycle. При ожидающих Start/Switch или starting/reloading основное
действие — Stop, а не disabled Connect. Только уже ожидающий Stop/stopping
блокирует повторный Stop. Ранний UI Stop до первого backend starting event
допускается через busy, даже если snapshot ещё stopped. Existing actionGeneration
сохраняет отзыв права старого Start публиковать свой finally/toast после Stop.
CoreReady=false блокирует Connect, но не Stop активного/ожидающего соединения.
Это source tracing Promise/state владельца, не доказанная нативная отмена Wails.

Главная: один основной lifecycle heading, причина blocked/error/unknown, Connect/
Stop, следующий шаг и действующее расписание retry; отдельно режим процесса,
сообщённый/выбранный канал и статус реестра без обещания reachability/protection.
Декоративная карта и повторные карточки удалены. TUN activation gate не открыт;
остался доступный возврат запущенного TUN-процесса в режим прокси через старый API.

Проверка пути: собственный display clock страницы считает возраст probeLastAt;
invalid/future time остаётся неизвестным/ошибкой часов, число не заполняется нулём.
Часы не измеряют сеть, не назначают TTL и очищаются при unmount. Historical/
missing/partial/failed/required-pass различаются; informational шаги не становятся
обязательными. Положительные шаги не подтверждают native capture/DNS/IPv6/UDP/
отсутствие утечек. Срок применимости не передан backend и не придуман UI.
Подробные шаги/границы раскрывает existing ProbeReportView; failure раскрывается.

### Список проверок и результаты

Diagnostics.refreshTests возвращает фактический success boolean, объединяет
параллельные list requests одной pending Promise, хранит loading/loaded/loadError,
различает null и пустой массив. Кнопки run не начинают второй сценарий при
loading/batch/single. Обновление списка не запускает проверки и не сообщает
успех до backend response. Existing execute/runAll/list refresh и toast path
сохранены; Wails запрос физически не отменяется и hard time bound не обещан.

Production TestCard (не тестовый файл) получил читаемые имена существующих
шести Go diagnostic IDs, scope и безопасный next step; unknown ID не получает
выдуманную область. Raw сообщение явно подписано сообщением проверки. UDP
случайный пакет не назван доказательством QUIC/HY2/filtering; наличие секретов,
права администратора и core readiness не подменяют транспорт/защиту. Existing
skipped/legacy unknown/pass/fail remain; scoped steps раскрываются по запросу.
CheckedAt — старый момент результата, не current network/channel attribution.
Дедлайн и session identity этих записей отсутствуют; это явно показано. Структурные
backend diagnostic методы/сообщения и их измерительные дефекты этим не исправлены.

### Локальное восстановление

UsePolling переиспользует прежний createPoller generation/one-pending path;
совместимый optional onLoading callback меняет loading для каждого запроса.
ReceivedAt — только renderer receipt time успешного non-null ответа; error/null
не сохраняют ложное время. Callback публикации по-прежнему отзывается owner stop/
generation. Старое наблюдение во время refresh помечено; таймер/cleanup прежние.
Изменение затрагивает всех существующих usePolling consumers, runtime не проверен.

Экран не вызывает RunProxyHeal и не создаёт recovery authority. Нет original-owner
receipt в его контракте; порт/tuple projection/задача/HEALED event не разрешают
сбросить настройки. Обычный Stop может восстановить только подтверждённые own
writes через backend owner; conflict не объявлен исправленным. Статусы scheduler
ready/running и lastCode=0 нейтральны, не зелёное «защищено». История вынесена
в disclosure и подписана historical; пустота/нет прав/unknown не становятся PASS.

### Ручной просмотр и остаток

Просмотрены восемь изменённых production UI исходников и их реальные API/callers,
основные ветви pending Start/Stop/Switch, polling generation/loading, diagnostics
null/empty/error/result, timestamp unknown/future/history и отсутствие новых
bindings/dependencies/state owner. CSS: minmax(0,..)/min-width:0, перенос длинных
значений/кнопок, media collapse, native button/details/summary, сохранённый global
focus-visible; display clock вне live region, periodic loading не озвучивается.

8UI4docs;12 original snapshots11existing1absent/task71rev709 +15 reviewed SHA;
evidence bootstrap-v305-ui-status-source-evidence.json. Byte identity и manual
source review не подтверждают Vue/TS compile, browser visual/usability, keyboard,
940x600/1180x780/1366x768, WebView2/DPI125/150/200%, native Stop/interop/recovery,
реальный protected egress, скорость/ресурсы/превосходство. Все такие gates OPEN.

PC-04 продолжается: Контекст сети/Адаптация/Телеметрия, затем Каналы/Журнал/
Настройки/Секреты и восемь сквозных UX сценариев. Не выдавать три страницы за
закрытие PC-04 или полного PC-00…09. PC01 OS-CAS/ABA/interprocess/crash/W3b/W4c,
PC03 accepted signed provision/native и остальные прежние обязательства сохранены.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false, ranBeforeJob
sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported не изменены.


<a id="pc-v306-network-telemetry"></a>

## V2-306 — Контекст сети, Адаптация и Телеметрия

**Source/unverified, 2026-11-08.** Продолжение полного PC-00…09/TH-00…08, не
закрытие PC-04.100 OPEN/56 NOT_RUN. Ни приложение, ни tests/fixtures/formatter/
parser/build/typecheck/browser/SDK/native/network/system операции не запускались.

### Источники и переиспользование

Сохранены Vue views, NavRail/ContextTabs/navigation, api/backend.ts/contract.ts,
PageHeading/HintBox, backendState, singleton infoFacts, createPoller/usePolling и
createTrafficHistory. Не введены router/store/UI framework/зависимости или новые
native API fields. Существующие numeric UI fields расширены до number|null для
отсутствующих/неподдерживаемых наблюдений; Go и generated Wails не изменены.
Все production-потребители затронутых nullable полей просмотрены адресным поиском.
Локальные accessibility/responsive guides из modern-web-guidance применены для
семантики, disclosure, доступной таблицы графика, intrinsic sizing/переносов. Это
не browser/DPI/accessibility доказательство.

### Конкретные исправленные source-трактовки

1. Класс Wi-Fi/mobile/Ethernet показывается как свойство интерфейса, не
   оптимальный транспорт/CGNAT/DPI verdict. ISP/страна — сведения HTTPS-измерителя
   об IP, не автоматически физический провайдер пользователя. Control без
   system proxy не означает обход внешнего VPN/OS route. SOCKS5h HTTPS success
   не объявлен исправностью системного DNS/UDP/IPv6 или всех приложений.
2. Время сетевого снимка, renderer receipt и отсутствующие per-field time/epoch/
   validity различаются. Ожидание/ошибка/неполный ответ не заполняются фактами.
3. Статические WARP domains из приложения подписаны декларацией. Их применение
   текущим VPS не измерено. Последние ответы сайтов не объявлены результатом
   текущего channel/network; challenge marker/403/timeout не устанавливает причину.
4. Пустой список SplitDirect/GetChannelIndependence не назван отсутствием правил/
   зависимостей: Go API также возвращает пустоту при ошибке чтения/построения.
   Empty API response и отсутствие ответа UI различаются без ложного вывода.
5. Policy nextCheckIn=0 означает сейчас ИЛИ неприменимость по существующему Go
   контракту; UI перестал обещать «сейчас». Точная причина выбора, оставшийся
   budget, кандидатный набор и deadline не переданы и не рассчитываются из
   жёстко скопированных policy констант.
6. Инциденты группируются по времени, что не устанавливает общую причину или
   согласованную блокировку. Empty clusters/regex logs не доказывают отсутствие
   сбоев. Старые causal class labels обозначены как исторические/сообщённые коды.

### Async owner и ручная проверка сайтов

У createPoller появился совместимый invalidate(): новый generation снимает право
публикации старого read; уже ожидающий Promise завершается, после него текущий
владелец получает один новый запрос через прежний refresh/таймер. Параллельный
load не создаётся. Stop/более новый generation отзывает queued resume. Старый
finally timer очищается новым refresh. Wails Promise физически не прерывается;
зависание старого запроса может задержать новый, hard bound не заявлен.

Network.runSites больше не присваивает adaptive.value напрямую, конкурируя с
фоновым GetAdaptiveStatus. После завершения команды вызывается invalidate того
же poller и заново читается stored status. Component owner=false при unmount
не публикует action outcome и не запускает последующий refresh. Start/switch/busy
не получают второй site action. Ответ не превращается в current channel proof:
привязка отсутствует в самом backend контракте и отмечена на экране.

InfoFacts сохраняет прежний один poller и ref-counted lifecycle. Добавлены общий
loading и раздельные facts/failover error/receipt для fulfilled/rejected/null;
ожидание обоих requests не приписывается OS measurement timestamp. Native calls,
периодичность, external endpoints и policy ownership не менялись.

### Наблюдаемые числа и интервалы

В api/backend.ts observedCounter принимает только nonnegative safe integers,
observedRate — nonnegative finite numbers. Missing/invalid/precision-lost значения
получают null, известный 0 сохраняется. Затронуты adapter/metrics/connection counts,
policy counts, epoch/health, dependency score, incident count и site HTTP code.
DNSViaTunnel missing остаётся null, false не превращается в DNS-failure diagnosis.

CreateTrafficHistory использует validated copied counter snapshot. Новые reset и
from/to объясняют окно; первый ответ даёт baseline с unknown delta. Missing
observation, invalid counter, другое имя, decrease и at<=previousAt сбрасывают
старые points/window; восстановленный ответ не склеивает пропуск. Default24,
валидный capacity1…512, invalid limit не превращается в unbounded slice(-0).
График не наращивает пустые/выдуманные samples и не поднимает настоящий zero
искусственным минимальным столбиком. Таблица содержит bytes и фактические
renderer receipt интервалы тех же points; ширина bars не объявлена длительностью.
OS time отсутствует, повторный cached response не выдан за новый OS measurement.
Имя адаптера не LUID/identity; скрытая same-name replacement этим не решена.

Statistics показывает adapter counters отдельно от core. Сырые backend rate поля
не имеют собственного validity interval и могут быть initial0; экран считает
average только по двум available core snapshots с возрастающим At и валидными
session counters. Первый snapshot, error/unavailable, clock/counter reset,
lifecycle/channel change снимают базу и показывают unknown rate. Подтверждённая
нулевая дельта двух подходящих snapshots остаётся 0 Б/с. Написаны источник и
интервал, это не forecast/канальная скорость/проверка protected egress.

Core session ID/полный атомарный с lifecycle snapshot не переданы. Late old
snapshot или same-ID rebuild могут не распознаться по одному имени/числам;
среднее описывает полученные snapshots, а не доказанную native session continuity.
База OS history использует renderer time, core average — backend At. Эти часы
не смешиваются. Суммы ядра названы от базы его сборщика, не месячной историей.
Список connections — ограниченная возвращённая выборка; empty не нулевой трафик.

### Композиция и ручной просмотр

Network: основная таблица фактов, компактные local/split сведения, site responses;
Adaptation: основной policy snapshot/health list, dependencies/incidents/rules в
native details; убрана третья полоса локальных pseudo-tabs. Statistics: adapter/
core blocks, numeric intervals и connections в details; дубликат Home probe убран.
Top-level sections и ContextTabs неизменны. Таблицы имеют caption/th scope и
внутренний keyboard-scroll region, длинные значения/кнопки переносятся, global
focus-visible сохранён; новая анимация/постоянные numeric live announcements не
вводились. Это только source layout intent, не visual/usability acceptance.

Просмотрены восемь изменённых production UI исходников, реальные API/callers и
контракты; manual branches null/known0/partial/reset/pending/invalidation/owner
traced.8UI4docs/12existing original snapshot/task71rev711/17 immutable input SHA;
evidence bootstrap-v306-network-telemetry-source-evidence.json. Никаких tests,
fixtures, compile/formatter/typecheck/parser/browser/WebView2/DPI/keyboard/native
checks не выполнялось; соответствующие критерии OPEN, no PASS.

Далее PC04 Channels/Logs, затем Settings/Secrets и восемь сквозных UX flows.
PC01 W3b/W4c, PC03 signed provisioning/native, весь PC-00…09 и девять TH этапов
сохранены.100 OPEN/56 NOT_RUN; no waivers; all bootstrap/bridge/native/chain/
relaunch/parent/helper gates=false, ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8,
TUN unavailable/schema2 unsupported не изменены. Без других ролей/чатов/субагентов.


<a id="pc-v307-channels-support"></a>

## V2-307 — Каналы и Журнал

**Source/unverified, 2026-11-08.** Полная цель PC-00…09/TH-00…08 сохраняется;
100 OPEN/56 NOT_RUN. Tests/fixtures/build/typecheck/parser/browser/SDK/native/
network/system не запускались, generated bindings/dependencies не менялись.

### Каналы: факты вместо смешанных статусов

Прочитаны реальные ListChannels/channelViewOf/SelectChannel и render mapping.
ListChannels возвращает объявленные descriptors, но Enabled вычисляется через
текущий SelectorTags + descriptor enabled + supported protected payload. До Start
cc может отсутствовать и все Enabled=false; это не «все выключены в профиле».
UI показывает этот допуск отдельно от metadata validation и lifecycle ActiveID.
Вместо звезды «избранного» — текстовая метка lifecycle. Из health API отдельно
показан последний reported verdict с временем соответствующего positive/negative
наблюдения. Registry live status не объявлен current reachability. Per-channel
original applied receipt отсутствует и отмечен unknown, не выведен из running/IP.
Реестр/lifecycle/health не имеют общего атомарного снимка, UI это сообщает.

Поиск/фильтр включая planned/sort сохранены. Empty/null/loading/error/no match
различаются; selectorCount known0 не unavailable. Переключение идёт через прежний
backendState/SelectChannel с backend revalidation. На странице есть Stop при
pending Start/Switch; original actionGeneration и Stop owner не изменены.

### Свежесть списка у существующего root owner

Раньше список читался только bootstrap, часто до появления running selector, а
initBackendState кешировал initialization и не обновлял Enabled после Start.
Добавлены root-owned refreshChannels/loading/error/renderer receipt. Один pending
request; lifecycle/activeID/mode edge снимает старую проекцию и увеличивает revision.
Поздний old-generation/revision result не публикуется; после завершения старого
request current owner запускает один новый read. Dispose отзывает publication.
Manual/open page refresh использует тот же request, нового poller/store нет.
Устаревший read не объявлен ошибкой источника только за утрату generation.

Wails Promise физически не отменяется и может задержать новый read. Если profile
изменился без этих lifecycle edges, нужен manual refresh; source ID/atomic selector
receipt не добавлены. UI admission не заменяет окончательный backend gate.

### Сырой локальный буфер и подготовка сводки

Сохранён singleton uiLog и лимит1000 записей; payload сообщения ограничен4096
UTF16 units плюс marker. Это retention bound UI, не bound входного native allocation/
всего процесса и не удаление секретов из raw message. Timestamp normalizer принимает
только bounded RFC3339-like string и valid Date; unsupported time становится empty,
не текущим временем. Собственные UI events без t получают renderer receipt time.

Backend callback читает только t/level/text. Missing/unknown event больше не
попадает в JSON.stringify целого body. Неподдерживаемый уровень получает конечное
сообщение об ошибке формата; original unknown fields не сериализуются. Произвольный
валидный raw text всё ещё может содержать приватные данные; producers/backend/native
logs требуют отдельного аудита. Утверждение «журнал free of secrets» удалено.

Logs source/level/search расширены debug и probe. Prefix source — подсказка UI,
не attested provenance. Clear очищает только локальный буфер; фильтр не удаляет
backend/Windows logs. Автопрокрутка наблюдает саму выборку, а не только length:
новый event при полном кольце учитывается. Пользователь выше низа приостанавливает
follow; explicit return/end и unmount generation защищают от позднего nextTick.

### Новый компонент по обнаруженному пробелу

В каталоге не найден аналог support preview: существующие ProbeReportView и
profile/pin previews имеют другие данные/authority. Добавлены SupportLogPreview
(presentation/frozen selection/copy outcome) и pure supportLog (bounded summary,
shared prefix/time normalization), в существующих components/composables.
Vue/Wails navigation/API/state architecture и PageHeading/HintBox/tokens/focus
переиспользованы; новый router/store/библиотека/экспорт native API не введены.

Сводка включает только normalized timestamp, finite level, prefix source и один
упомянутый code из фиксированного списка. Raw text, captured regex fragments,
query, URLs/адреса/пути/имена каналов не подставляются. Unknown code не выводит
оригинальное сообщение. Совпадение — упоминание, не диагноз или факт исхода.
Доступны последние<=200 selected records, output<=32000 UTF16 characters с явными
counts omitted. Это ограниченная диагностическая сводка, не полный PC08 bundle,
не signed native report и не гарантия анонимности (timestamps/levels остаются).

Prepare фиксирует точный текст; новые события не меняют reviewed body. Компонент
показывает readonly labelled textarea до explicit Copy; копируется selected.text.
Изменение фильтра/clear отзывает preview, unmount/generation отзывают outcome/
focus. Повторный clipboard call запрещён до завершения исходного Promise. Его
физическую отмену после dispatch UI не обещает; старый reviewed body может успеть
попасть в clipboard, но не превращается в подтверждение нового preview. Ошибка
показывается без raw exception; текст можно вручную выделить. Файлы/сеть/uploads/
сообщения сторонним людям здесь не отправляются агентом и новым кодом автоматически.

### Ручной просмотр и непроверенное

Manual source/diff/caller review6 production UI files: readonly facts, list
freshness/stale owner, known0/null, switch/Stop, raw retention/finite export fields,
frozen copy publication, filter/clear/unmount/autoscroll. CSS/HTML: native controls,
caption/th scope, inner keyboard table/log region, labels/heading focus, min-width0/
wrapping/media; existing global focus-visible. Relevant local accessibility/
responsive guides использованы без browser execution.

6UI4docs;10 original snapshots8existing2absent/task71rev713/17 immutable input SHA;
evidence bootstrap-v307-channels-support-source-evidence.json. Source/byte review
не доказывает Vue/TS compile, clipboard/WebView2/keyboard/DPI/usability, native
selector/applied/reachability/recovery, secrecy всех logs, скорость или ресурсы.
Никаких tests/fixtures/build/typecheck/formatter/parser/browser/native проверок.

Далее PC04 Settings/Secrets и восемь сквозных UX flows. PC08 full support pipeline
и остальные PC00…09/девять TH этапов остаются OPEN.100 OPEN/56 NOT_RUN/no waiver;
all bootstrap/bridge/native/chain/relaunch/parent/helper gates=false, ranBeforeJob
sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported сохранены.


<a id="pc-v308-settings-secrets"></a>

## V2-308 — Настройки, Секреты и владение действиями

**Source/unverified, 2026-11-08.** Полный PC-00…09/TH-00…08 активен;
100 OPEN/56 NOT_RUN. Tests/fixtures/build/typecheck/browser/native/registry/network/
SDK не запускались. Из formatting только gofmt двух затронутых Go файлов.

### Метаданные и ошибки vault

API mapper берёт backend из Meta, не константу DPAPI; missing backend=null,
missing/unsupported storedValue=unknown отдельно от реально пустого слота.
Reveals nullable safe integer, 0 остаётся0; fingerprint принимается только в
реальном16hex формате текущего Go helper. Verify status unsupported не positive,
raw verifyError не показывается: статус и фиксированная инструкция формата.
Existing native DTO fields/JS data shape сохранены, generated bindings не менялись.

SecretsStore сохраняет metadata singleton, но добавляет ref-counted view owner,
generation/revision, single pending list, loading/error/renderer receipt и один
pending operation без очереди. На последнем release cached metadata/outcome
отзываются; старый read не публикуется в нового потребителя, после его окончания
текущий owner получает новый read. Native operation не физически отменяется;
новая страница может ждать старый запрос. Mutation outcome и повторный metadata
read различаются. Raw errors/title/value не отправляются store в toast/log.

Save/Delete/Verify/Test/Reveal facade logs в app.go заменены конечными фразами;
названия/ID слотов и err.Error не интерполируются. TestSecret менял только текст
логов, не транспортные проверки/fixtures/tests. Native error returns и остальные
producers не переписаны: это не доказанная глобальная очистка IPC/native logs.

### Формы и Reveal

SecretsView и SecretCard очищают draft/confirmation/show flags при cancel/unmount
и после отправки записи. Action snapshot получает исходный ввод до очистки refs;
вызов Wails уже мог начаться и может удерживать строки. Не заявлено гарантированное
стирание immutable JS strings, памяти IPC/OS или уже записанного clipboard/history.
Название — видимые metadata, не место для самого значения.

Card action lease отзывается при unmount и наблюдаемой смене id/updatedAt/fingerprint.
После Reveal результат проверяется до clipboard dispatch, stale owner не запускает
writeText; после dispatch физическая отмена невозможна, outcome тоже generation-
fenced. Возвращённый plaintext не кладётся в reactive store; не ждёт metadata
reload перед возвратом исходному caller. Пустой ответ не очищает clipboard.
Счётчик выдач не объявлен успешным копированием. Действия слота сериализованы;
удаление имеет отдельное явное подтверждение без прежнего4s race/timer, исходный
input очищается на закрытии. Parent focus переносится до удаления DOM слота.

Локальный формат и сетевые проверки разделены. SSH/API могут использовать ключ/
токен по назначению, прежнее обещание «значение никогда не покидает компьютер»
удалено. Сетевой вызов только по кнопке; агент его не вызывал. Отчёт со skipped/
непереданными шагами не получает автоматический green success; native route не
подтверждается. Текущие report details/producers и доверие endpoint требуют
отдельного PC03/PC08 анализа; полной гарантии отсутствия секретов там нет.

### Настройки и автозапуск

Profile/runtime/autostart читаются parallel через single pending owned summary;
import/update changed отзывает старый read, не создавая второй параллельный load.
Ошибка одного источника не блокирует отображение других. Autostart write имеет
отдельный revision; старый summary не затирает результат записи. После Set идёт
Get с повторным owner check, unknown/error не показывается как false/off.
Профиль/состав/разрешение/отзывы/storage state/time отдельно от VPN path. Хардкод
runtime vault/log paths убран. Mode/recovery используют прежние APIs и NavRail/
ContextTabs; общий TUN gate не открыт. Закрытые transfer/update panels unmount
children, а не просто скрывают живую форму с passphrase и pending callbacks.

В tray_windows.go readAutostart/readAutostartValue узнают exact текущую команду
REG_SZ. Missing key/value и empty command — off; access/type/foreign command —
error/unknown. Setter запрашивает QUERY|SET, отказывается от наблюдаемого чужого/
неподдерживаемого value, existing same desired state даёт no-op. После собственной
write/delete — readback на том же handle, затем UI reread. Disable absent value
идемпотентен. GetAutostart теперь bool,error (обычный Wails error channel), JS
Promise<boolean> data shape прежний; normal regeneration/compile не выполнялись.
Legacy tray checkbox всё ещё bool и скрывает unknown, но setter conflict guard
общий; это ограничение не объявлено исправленным во всех поверхностях.

Это последовательные чтения, не OS CAS/interprocess writer exclusion, launch
identity/signature/Job receipt или доказанное исполнение следующего входа.
GetStringValue/native mutex/registry close не получили hard budget/retirement
proof. Missing HKCU Run key при enable может по-прежнему дать error; никакой
скрытой registry repair, alias normalization или overwrite чужой команды нет.

### Перенос и обновление

ProfileTransfer содержит own mounted/generation checks после каждого await:
late file picker не запускает preview, stale export/apply не публикуется/не
emits changed. Exact bytes/passphrase/mode/root/policy/target и validUntil checks
сохранены. Apply captures request перед consume/clear полей; Backend повторяет
свои проверки. Import expiry очищает inputs под silent-reset guard: sync watch
не снимает expired flag и не делает expired preview допустимым. Password/QR/
transport refs очищаются на закрытии/unmount, export pass — после dispatch.
Apply reply не назван active native application: нужны reread accepted version и
новое соединение. Guard отсутствие C1/native acceptance не заменён обходом.

ApplicationUpdate отделяет verified package от install/relaunch outcome. Preview
consumed даже при partial failure, directory очищен до dispatch/unmount. Возврат
Apply не объявляет новый EXE/готовый child: awaitingRestart без вечного busy spinner,
без повторного Apply из того же preview. Closing не отменяет native operation;
loaded-image/update guards остаются у backend. Check failures тоже обновляют
summary, поскольку delivery state мог измениться; source/native acceptance OPEN.

### Приоритетный lifecycle finding

Прямой source trace: app.go App.TestSecret удерживает a.mu через runSecretTest;
secrettests.go dispatcher может идти в SSH/Cloudflare network operations.
protection.go stopCoreFor сначала захватывает тот же a.mu для registerStopLocked.
Поэтому frontend доступность Stop сама не доказывает быструю registration при
сетевой проверке секрета. Runtime не проверялся. В этом срезе долгий mutex не
устранён: нельзя просто отпустить lock без capture/lifetime/vault/source fences.
Следующая работа — P0 safety/lifecycle owner для этого production control path,
не запуск теста, не blind unlock и не выдача disabled feature за полный результат.
После него остаются PC04 все8 UX flows и весь PC00…09/девять TH этапов.

### Evidence и границы

Manual source/diff/caller/owner review8UI2Go;14 existing original snapshots/
task71rev715;18 immutable reviewed SHA (17 начальных после scope extension и
добавленный Stop caller). Дополнительный tray snapshot снят до его изменения,
исходный reviewed inventory сохранён. gofmt2+2, explicit exit0. Evidence:
bootstrap-v308-settings-secrets-source-evidence.json. Compile/Wails generation,
clipboard/WebView2/DPI/keyboard/usability, vault/native/registry/crash/relaunch,
protected egress/secrecy/performance не проверены. Никаких новых тестов/fixtures.

100 OPEN/56 NOT_RUN/no waiver. All bootstrap/bridge/native/chain/relaunch/parent/
helper gates=false, ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/
schema2 unsupported сохранены. Не затрагивались другие роли/чаты/секретные файлы.


<a id="pc-v312-diagnostic-ui"></a>

## V2-312 — контекст отчёта и владелец пакетной диагностики

**Source/unverified, 2026-11-08.** Продолжается весь PC-00…09/TH-00…08,
100 OPEN/56 NOT_RUN, без waiver.6 Go/7 UI/4 docs; никаких tests/fixtures,
build/typecheck/SDK/native/browser/network действий. Это дополнение к прежнему
описанию страницы: session/context теперь передаётся, но TTL и физический путь
по-прежнему не измерены. Все восемь страниц PC-04 целиком этим не принимаются.

### Общий контракт и cache

AppState.diagnosticContext и TestResultView.context содержат schema1, uint64 IDs
в canonical decimal string без JS rounding, reported selector/mode/lifecycle/
Ready и отрицательные current/closing flags. Новый файл diagnostic_context.go
обоснован общей публикацией GetState/pushState/manual task, а не новым store.
Только presentation revision на сравнимом tuple; при A→B→A это новый receipt.
На переполнении revision — неизвестно, path ID не оборачивается. UI не посылает
receipt обратно и не получает admission/trust/native authority.

GetState/pushState уже могут вызываться с Epoch.mu: helper читает существующий
atomic original-path mirror, immutable ID и atomic negative flags, не берёт
Epoch.mu и не создаёт новую сеть. App.mu охраняет последнее presentation value.
Private task context/parent/intent/path/closing/deadline и original done остаются
основанием допуска/завершения. Capture после admission связывает original task;
finalizer отвергает изменившийся tuple, даже если ресурсы уже retired.

ListTests возвращает context исходного task и explicit contextCurrent as-of read;
stale/unfinished/cleanup-uncertain result остаётся unavailable, без retagging.
RunTest current=true только у ответа, прошедшего deferred finalizer. Credential
mutation invalidation сохранена. Core-wired читает уже имеющуюся power-revoked
проекцию CoreReady. Correlation не доказывает физическую неизменность сети,
atomic source+I/O, protection, freshness TTL или общую linearizability.

API mapper принимает только поддерживаемую форму, canonical bounded IDs и
известные lifecycle/mode; legacy/missing/invalid context означает unknown.
Generated models/bindings и preview/test fixtures не редактировались/не запускались.
Native Wails JSON retention и совместимость всех сборок ещё требуют приёмки.

### Жизнь страницы и действия

Удалён неиспользуемый дублирующий tests cache/startup read из backendState.
Один root-owned GetState pending; новый native event выигрывает запоздалый
snapshot. Dispose не освобождает реально ожидающий вызов; новый generation
получает свежие данные после его возврата. Прежние lifecycle controls сохранены.

Diagnostics использует существующий createPoller. Refcount/effectScope управляют
единственным watcher/timer; metadata load имеет generation/context. List bound64,
IDs bound128/duplicate rejection; один invalidate continuation + latest-needed
bit не накапливают очередь от событий. Пять секунд — период metadata, не TTL,
не сетевой замер. ReceivedAt — время чтения списка, CheckedAt не подменяется.

Каждый action принадлежит инициирующему consumer/generation/context. Второй
RunTest не отправляется, пока original Promise не settled. Отзыв публикации
не равен отмене RPC. Unmount/смена scope/context/ручное «Не запускать следующие»
прекращают будущие dispatch пакета, включая microtask до первого api.runTest.
Уже отправленный вызов остаётся pending; возврат на страницу его не присваивает.
Поздний ответ/ошибка не публикует новый current результат или toast другого
consumer. Batch summary считает ответы приложения, не успешные измерения.

Одна кнопка пакета сохраняется в DOM; подпись меняется до ожидания ответа.
Keyboard/focus не перемещается программно. Для сообщений переиспользован общий
ToastHost polite region; metadata polling не создаёт per-row live announcements.
Finite action errors/UI log не сериализуют raw ошибки/IDs/details. Это ограничение
затронутого flow, не полная редакция всех backend логов/ошибок приложения.

### Один итог отчёта

TestCard раньше повторно вызывал generic aggregate по всем steps после mapper,
что возвращало необязательный direct comparison в итог freeze-check. Новый общий
manualDiagnosticOutcome используется обеими точками: ровно четыре tunnel объёма
8/16/32/64КБ, уникальные имена/terminal statuses; сравнение остаётся видимым.
Generic protected-probe role/allowlist не менялись. Нет contextCurrent+совпадения
tuple — нет current pass/fail. Старый timestamp не обновляется при чтении.

### Доказательство и остаток

Manual source/caller/defer/error/Promise review, touched gofmt6+6+1exit0,
17 snapshots(16 existing/1 new), original task71rev725,18 immutable reviewed SHA.
Evidence: .artifacts/memory/bootstrap-v312-diagnostic-ui-source-evidence.json.
Modern Web Guidance accessibility §8 использован для существующего polite region;
Temporal-specific guide не применялся, зависимости/новые библиотеки не добавлены.

OPEN: compile/typecheck/generated ABI/fixtures/browser/keyboard/DPI/actual Wails
late response, concurrency/epoch/power/cache/TTL/native task cancellation and
resource bounds; полная redaction/lifecycle/защита всех приложений/SDK/release/
performance/fullPC/old9. Описанные source guards не заменяют эти доказательства.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8; TUN unavailable/schema2 unsupported.

Следующий P0 — source compatibility normalized DNS→SDK lowering: внешний отчёт
stock1.13.19 указывает на detour=direct с empty direct outbound. Нужна собственная
проверка исходников, сохранение service-only bootstrap/protected final DNS/
subject/signature/worker custody. Не blanket removal, direct fallback или server
changes. Внешний payload PASS не становится собственным Windows evidence;
чужие чаты/планы/runtime не читались/не управлялись. Android/shared/configs/
harness/dependencies/Git не менялись. Полная цель active, measured superiority нет.


<a id="pc-v333-settings-autostart"></a>

## V2-333 — Настройки: исходная операция автозапуска и актуальный ответ

**Source/unverified, 2026-11-09.** PC-04; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.2 Vue/4 docs; V332 был progress.

### Найденные дефекты

SettingsView включал getAutostart в общий Promise.allSettled профиля/процесса:
готовое чтение не публиковалось, пока не завершится посторонний запрос. Refresh,
начатый во время toggle, захватывал тот же autostartRevision, который оставался
после write/readback; поздний ответ мог затереть результат mutation. Успешный
refresh не очищал прежнюю autostartOutcome error. При неизвестном состоянии
кнопка изменения была disabled, а независимого обновления строки не было.

### Изменения и переиспользование

- AutostartSetting.vue выделяет существующую row, nullable boolean, error mapping
  и existing api.getAutostart/setAutostart. Нет пригодного отдельного autostart
  component/composable; поиск найденных components/stories/consumers подтвердил
  единственный Settings consumer. ProfileTransfer/ApplicationUpdate остаются
  отдельными компонентами. Existing revision/unmount/secretsStore flow и global
  tokens/actions-row/focus styles использованы как проектные соглашения.
- Component владеет одним original read или write→fresh readback, без параллельных
  вызовов. Явное обновление во время read инвалидирует старую publication и
  ставит один follow-up; во время write только ставит follow-up. Original pending
  promise сохраняется до settlement. Двойной toggle/действие при unknown/preview/
  pending read отклоняется guard. Нет таймера, timeout-as-completion, polling,
  автоматического повторения записи или дополнительного SDK/network owner.
- Revision меняется до/после mutation; write readback — собственный новый Get,
  а не общий старый read promise. Late success/error не публикуются после
  invalidation/unmount. Unmount снимает UI interest и queued refresh, не
  объявляет Wails/native operation отменённой. Original key/Close/native task
  ownership остаётся в Go V332; backend source здесь не менялся.
- На новом чтении старые value/receipt/error очищаются; success принимает
  фактический boolean и время получения экраном, failure оставляет null.
  Write success показывается только после fresh Get; несовпадение с запросом
  отображает фактически прочитанное значение и отрицательный outcome. Raw
  ошибки/registry command/secret values не публикуются. Preview не вызывает
  native API и не получает mock false как факт.
- Parent Settings сохраняет profile/runtime summary и Stop/mode/profile/update
  actions; autostart исключён из его allSettled и state mutation. Общая кнопка
  refresh уведомляет child через refreshVersion, local refresh строки работает
  независимо от ожидания summary. Native Wails API/bindings не менялись.
- Разметка остаётся плоской строкой, без новой карточки. Unknown/loading/write/
  outcome разделены; accepted UI receipt time подписан отдельно. Persistent
  role=status/aria-atomic сообщает итог, interstitial loading не отправляется
  в live region. aria-busy описывает row; aria-disabled buttons остаются
  focusable, оба handlers реально блокируют недоступное действие, type=button
  не отправляет окружающую форму. Existing focus-visible и colors сохранены;
  flex-wrap/min-width/normal button text используют existing tokens/styles.

### Разбор и границы

Ручной original diff/new component/callers review: initial read, fulfilled/rejected/
unknown/preview, original pending read, refresh coalescing/invalidation, write
queue/readback/mismatch/error, повторный click, fresh error clearing и unmount.
2Vue4docs/6snap5existing1new/task71rev788 archived/14reviewedSHA. Vue вручную
отформатирован; formatter/gofmt/parser/options/typecheck/SDK/compile не запускались.
Skills frontend-postgres и modern-web-guidance применены; local guide accessibility
повлиял на semantic actions/focus/стабильный status, не доказывает a11y/visual UX.

No tests/fixtures/build/vet/typecheck/browser/native/registry/network/product
checks, actual logs/vault/profiles/secrets. Compile/types/Vue props/watch ordering/
screen reader/keyboard/scaling940×600/1180×780/1366×768/125…200%/current browser/
EXE/OS/login/native retirement/protected egress остаются непроверенными. Promise
settlement/UI revision не доказывает retirement всех SDK/FFI/kernel работ;
нет atomic external-writer CAS, hard RSS/wall-time bound или измеренного выигрыша.
Тестовые fixtures/reset/helpers не редактировались; future compatibility OPEN.

### Следующий срез

Remaining Settings profile/runtime allSettled всё ещё связывает publication/loading:
один готовый ответ задерживается другим, shared receipt timestamp выставляется
после пары. Исследовать original independent response ownership и stale generation
до следующего изменения, не создавать overlapping retries или новый global store.
FullPC/old9/100OPEN56NOT_RUN/all gates=false/ranBeforeJob sticky/GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
state/chats/subagents/Git/dependencies/system network не изменялись.


<a id="pc-v334-independent-facts"></a>

## V2-334 — независимые факты настроек, сети и адаптации

**Source/unverified, 2026-11-09.** PC-04; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.3 Vue/2 TS/4 docs; V333 был progress.

### Исправления по исходникам

SettingsView ожидал profile/runtime через allSettled, задерживал готовые факты
и подписывал их временем после завершения пары. Такой же общий barrier связывал
OS network query и failover status в infoFacts. Дополнительно createPoller мог
вызвать уже queued load после stop/invalidation: проверка была только перед
publication. Это source findings, не воспроизведённые runtime failures.

- Existing createPoller получил interval=null: один manual reader без timer.
  Его pending/generation/start/stop/invalidate переиспользованы в Settings вместо
  второго request helper/global store. Числовая формула timer после completion
  не менялась. Новый gate перед load влияет на всех callers: queued inactive/
  old-generation call пропускается до API. Typed wrapper отличает skipped entry
  от actual loaded null, без type cast или synthetic value. Begun call остаётся
  под original pending до settlement; late result/error не публикуются.
- Settings profile/runtime имеют свои original reader, loading/error/data и
  accepted-response UI receipt. Один готовый источник не ждёт другой; own refresh
  buttons guarded/focusable, general refresh coalesces pending без блокировки
  доступных controls. На новом manual read старые data/error/receipt очищаются;
  actual null/error остаются unavailable. Profile/update changed инвалидирует
  обе original generations, но каждый successor ждёт только своего predecessor.
  Initial embedded-profile suggestion и Stop/mode/autostart/actions сохранены.
- infoFacts имеет два shared singleton poller с прежним users refcount. На
  источник приходится один original pending и completion-based timer10s, не
  отдельный poller на страницу. First acquire запускает оба, last release
  останавливает оба; unfinished calls сохраняются. Restart не принимает старый
  ответ и не накладывает successor на original pending этого источника.
- Ошибка network больше не стирает failover и наоборот; receipt присваивается
  отдельно после принятого собственного ответа. Legacy loading Ref и aggregate
  refresh completion сохранены; новые factsLoading/failoverLoading и source
  refresh используют NetworkView/AutoProtectView. Dashboard facts API не менялся.
  Во время periodic read старый снимок сохраняет прежний receipt, не выдаётся за
  новое наблюдение. Это два независимых снимка, не atomic cross-source snapshot.

### Проверено вручную / не проверено

Five original diffs/callers/source reviewed: independent responses/errors,
manual normal/failed/null, queued skip, load already entered then invalidated,
late publication refusal, stop/start while pending, coalesced refresh, null mode
без periodic retry, numeric schedule, pair refcount/source flags/legacy API.
Checkpoint progress сохранял intermediate manual-mode source и план admission
guard до его edit; следующий checkpoint расширил scope до infoFacts/двух views
после before snapshots. Initial6 snapshots/initial17 reviewed archived; final
9snap9existing/task71rev791/14reviewedSHA. Original AutoProtect missing final
newline сохранён. Source вручную форматирован, formatter/parser не запускались.

Skills frontend-postgres/modern-web-guidance: existing factory/components/API/
tokens и ранее прочитанный local accessibility guide. Native button semantics,
aria-disabled guards/focus и existing wrap сохранены; browser/a11y/scaling proof
отсутствует. No tests/fixtures/types/build/vet/SDK/browser/native/registry/network
или product checks; actual logs/vault/profiles/secrets не читались.

Local generation/promise проверяют UI entry/publication, не retirement всех
Wails/SDK/FFI/kernel работ, реальную свежесть OS measurement или protection gate.
Количество load/timer bounded только внутри соответствующего owner; не заявлены
total RSS/wall-time bound, native latency, cross-view global RPC uniqueness,
callback reentrancy proof или измеренный выигрыш. Async delivery/typing/timers/
watch ordering/mount/unmount/device/EXE/protected egress остаются непроверенными.

### Следующий срез

Generic usePolling пока сохраняет reason.message/String(reason) в error ref,
а Network/AutoProtect/Statistics печатают его в UI. Trace error contract и
существующее sanitization/recovery перед правкой; это подтверждённый source path,
не доказательство фактической утечки секретов. API payload/structured error/
downstream compatibility и future tests требуют отдельной приёмки.
FullPC/old9/100OPEN56NOT_RUN/all gates=false/ranBeforeJob sticky/GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
states/chats/subagents/Git/dependencies/system network не изменялись.


<a id="pc-v335-safe-ui-errors"></a>

## V2-335 — локальные сообщения отказов вместо текста исключения

**Source/unverified, 2026-11-09.** PC-04; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.2 Vue/5 TS/4 docs; V334 был progress.

### Найдено и изменено

usePolling сохранял reason.message/String(reason) в UI ref; lifecycle/backendState
передавали его в toast, который также пишет error text в uiLog. errText назывался
«безопасным», но просто дописывал raw error к prefix. PublicProfileSource и
PinPolicySource также показывали exception message. API wrappers не гарантируют
перехват rejection Wails до этих handlers. Это source path, не доказательство
реально переданного секрета; пользовательские логи/профили/ошибки не читались.

- failureMessages.ts — новый маленький frozen catalog из15 локальных action
  сообщений; пригодного общего mapper не найдено. Существующие private errText/
  component snippets не были safe general boundary. Каталог не принимает
  исключение, не вызывает его message/toString/getters и не пытается regex-
  redaction неизвестного payload. Выбор ключа задаёт локальное действие.
- usePolling сохраняет fixed load failure, оставляя data/receipt=null. Его
  pending/generation/null-vs-unavailable/timer behavior не менялся. Страницы
  уже подписывают источник данных; локальная причина отказа не выдумывается.
- Start/Stop/Switch/mode и lifecycle probe/openLogs используют action-specific
  messages без сырого suffix. Action generation, busy/stopping и доступность
  Stop не менялись. Rejected command означает неподтверждённый результат, не
  доказанный rollback/полную остановку/неприменённую запись. Canonical blocked
  reason/status остаются в существующем UI state/labels, не извлекаются из Error.
- PublicProfile/PinPolicy preview failure снимает receipt и expiry перед
  original cancel; error после получения превью не оставляет applyable token
  на экране. Apply warnings сохраняют возможность уже выполненной записи и
  необходимость перечитать факты. Original one-use token/nonce/generation,
  trust/hash/version correlation и emit changed cleanup сохранены.
- Own export correlation mismatch отображается отдельно контролируемым текстом;
  pin result mismatch остаётся отрицательным. Новое export action очищает
  exported/savedPath, новое save — старый savedPath. Поздний или неудачный
  результат не выдаёт предыдущий success за результат новой попытки. Raw
  transport/password/path не добавляется к сообщениям; сохранённый path по
  успешному явному Save остаётся отдельным разрешённым результатом API.
- Toast/uiLog behavior не переделан; comments прямо отмечают caller obligation:
  toast не является автоматическим sanitizer. В затронутых handlers до него
  доходит только local catalog. Backend log events — отдельный не закрытый путь.

### Ручной разбор / ограничения

Six existing diffs/one new catalog/callers reviewed: fixed15 keys/imports,
polling error→refs→views, action error→toast→uiLog, uncertain apply, exact
correlation mismatch, preview-error receipt revocation и new-attempt clearing.
11snap10existing1new/task71rev795 archived/17reviewedSHA. No formatter/parser/
types/build/tests/fixtures/SDK/browser/native/registry/network/product checks.
Native wrappers/Go/Wails bindings/contracts/dependencies не менялись.

Skills frontend-postgres/modern-web-guidance: существующие компоненты, API,
generation/expiry/status regions и local accessibility guide. Layout не
перестраивался; keyboard/screen reader/scaling/browser UX не проверены. Source
каталог не доказывает отсутствие секретов во всех данных, native events/logs,
transport, backend error storage или support artifacts. Native actions/actual
apply/cleanup/SDK/FFI/kernel/egress/performance остаются unverified; ни один
полный критерий не закрыт и измеренного выигрыша не заявлено.

### Следующий срез

backendState по-прежнему принимает Go log event.text после проверки типа и
передаёт его в uiLog. Проследить actual Go producers/redaction → local display/
retention → support export projection; переиспользовать существующую обработку
до изменений. Не удалять полезные сведения слепым blanket logger/redactor и
не читать реальные логи или секреты. Actual leak не подтверждён.
FullPC/old9/100OPEN56NOT_RUN/all gates=false/ranBeforeJob sticky/GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
states/chats/subagents/Git/dependencies/system network не изменялись.


<a id="pc-v336-log-projection"></a>

## V2-336 — границы и формат записи журнала App/UI

**Source/unverified, 2026-11-09.** PC-04; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.2 Go/2 TS/4 docs; V335 был progress.

### Аудит пути и изменения

appendLogLocked создавал raw LogLine до App ring500, writer queue1024, TailLogs,
Wails event и app.log. Production constructors найдены здесь; disk writer
форматирует Text через %s с финальным newline. Число записей ограничено, длина
Text на этом пути — нет. Embedded controls могли менять физическое представление
строки. UI старый slice добавлял marker сверх4096 units и мог разрезать surrogate.
Это source findings; реальные записи/ошибки/профили не читались.

- app_log_projection.go — новый pure helper в существующем App package; generic
  пригодного аналога не найдено. Под appendLogLocked value mutex, до retention/
  queue/event, accepted severity ограничена debug/info/warn/error; unknown даёт
  fixed warning без исходного level/text. Нет I/O/callback/дополнительного lock.
- Go Text≤4096 UTF8 bytes с marker внутри cap. Bounded output buffer и поэтапный
  utf8.DecodeRune не копируют/не сканируют весь большой input; обычные Unicode
  слова сохраняются. Control/Cf/LS/PS становятся visible Go escapes через
  strconv.AppendQuoteRuneToASCII; invalid UTF8 byte — visible hex. Safe boundary
  сохраняет whole UTF8/escape piece при усечении. Backslash/обычный текст не
  декодируется заново. Timestamp по-прежнему создаёт App, file framing не менялся.
- UI uiLog insertion повторяет ограничения своего слоя: ≤4096 UTF16 units
  включая marker, whole codepoint/escape pieces; CR/LF/tab/C0/C1/browser line/
  bidi-format controls и lone surrogate показываются явно. Это targeted browser
  control set, не копия всей Go Unicode Cf table. Неподдерживаемые runtime
  level/text дают fixed notice, без stringify неизвестного объекта. Existing
  ring1000/API и timestamp normalization сохранены.
- Engine tailer добавляет «движок » перед LEVEL message; renderer source задаёт
  file Timestamp=false. Старый uiLogSource требовал literal '[' после префикса
  и терял эту классификацию. Теперь узнаются «движок » и «движок:» omission notice.
  Source по-прежнему inferred from prefix, не аутентифицированный producer.
- SupportLogSnapshot уже экспортирует только normalized time/reported level/
  inferred source/mentioned allowlist code, максимум200 rows/32000 chars; raw
  message/filter/path/address не копируются. Политика этой проекции и selected
  immutable clipboard preview сохранена. Отдельный native engine.log file не
  проходит через App insertion до собственной записи и этим срезом не исправлен.

### Ручной разбор / доказательства и остаток

Three existing diffs/one new helper/callers reviewed: App constructor→memory/
queue/event/disk formatter, allowed/unknown severity, bounded output/marker,
empty/normal text/Unicode/control/invalid encoding/truncation boundaries, UI
typed insertion/whole pieces, producer prefix→filter/support source. gofmt2+1
observed exit0.8snap7existing1new/task71rev798 archived/20reviewed source SHA и
1 installed stdlib quote.go identity. Это не проверка версии/совместимости
активного toolchain. Frontend вручную форматирован, no formatter/types/parser.

No tests/fixtures/build/vet/product checks/SDK/browser/native/registry/network,
actual logs/vault/profiles/runtime-config/secrets не читались. Compile/API/Unicode/
runtime/wire/file/clipboard/race/keyboard/scaling/UX и all native gates unverified.
Per-record logical limits не являются total RSS/wall-time/file quota bound:
input уже мог быть построен caller/SDK, другие retained objects/FFI/IPC/storage
живут отдельно. Защищается production append/insertion path, не произвольная
мутация exposed refs, внешняя запись файла или malicious trusted process.

Projection не скрывает credentials/адреса/пути, не является secret redactor,
не делает raw backend text export-safe и не доказывает отсутствие утечек.
Actual Go error producers/SDK engine.log privacy/retention остаются отдельными.
Next: trace их existing domain-specific coded summaries/redactors/log level/
output ownership до изменений; не удалять полезную диагностику blanket фильтром
и не читать реальные логи. Модели protection/native/traffic не менялись.
FullPC/old9/100OPEN56NOT_RUN/all gates=false/ranBeforeJob sticky/GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
states/chats/subagents/Git/dependencies/system network не изменялись.


<a id="pc-v337-log-privacy"></a>

## V2-337 — сообщения startup/bootstrap и штатный уровень журнала SDK

**Source/unverified, 2026-11-09.** PC-04/fullPC00…09/old9 active;
100 OPEN/56 NOT_RUN, без waiver.4 Go/4 docs; предыдущий V336 — progress.

### Подтверждённый по исходникам путь

startup копировал profileErr.Error() трёх этапов и vaultPath в общий журнал.
secretvault/filetxn могут возвращать wrapped filesystem errors; наличие
реальной утечки не устанавливалось, пользовательские данные не читались.
Bootstrap дважды форматировал host/IP и resolver identity через Logf→App fmt.
Сообщения публиковались до addressHint, который мог отвергнуть локальный scope/TTL.

Закреплённый sing-box v1.13.19 VLESS/HY2 пишет destination пользовательского
соединения на info. log.New разбирает level; observableLogger.Log пишет в writer
только level<=configured, но при needObservable отдельно Emit'ит messageSimple,
в том числе lower levels. box.New включает observable для Clash API. Это не
runtime доказательство конфигурации/маршрута; источник — локальный cached SDK.
defaultFactory.Start открывает engine.log с O_APPEND, не O_TRUNC. В текущем
Windows caller candidate prepare специально не усекает активный журнал.

### Изменения

- App startup: фиксированные сообщения recovery/migration/seed без raw error
  или пути. Исходный profileErr сохраняется для возврата/blocked/auto-connect
  gates. Сообщение «хранилище подготовлено» только при profileErr=nil; оно не
  доказывает весь startup/readiness и не содержит технического пути.
- PreflightResolver: существующий Logf API сохранён; private logAddressHint
  имеет конечные cache/DNS/DoH/unknown сообщения без format args. Worker strategy
  содержит только transport class, не адрес resolver. Host/IP остаются рабочими
  данными адресной подсказки, не копируются в log notice. Callback вызывается
  после успешного addressHint. Это наблюдение подсказки, не результат последующей
  retirement/service authentication/protected probe. Cache TTL/clock/current/
  оригинальные worker cleanup/ошибки/fallback/gates не заменены строкой журнала.
- App bootstrap adapter больше не fmt.Sprintf неизвестный args; fmt import
  убран. Production producer передаёт только перечисленные fixed literals.
- Renderer: штатный SDK writer level info→warn; предупреждения/ошибки сохранены,
  per-connection info destinations исключены из этого output. Output path и
  timestamp framing прежние. Исправлен ошибочный комментарий про per-start
  truncate: SDK append не задаёт квоту, ротацию или очистку истории.

### Ручной разбор / ограничения

Four original source diffs, startup nil/non-nil/profileErr return/autoconnect,
две hint insertion branches/finite default/zero callback args, callback callers,
renderer→cached SDK level/write/observable/append branches просмотрены вручную.
gofmt4+1+1 завершился exit0.8 existing snapshots/task71rev801 archived;
16 reviewed source SHA и6 cached SDK SHA сохранены. Cached source identity не
подтверждает активный SDK/toolchain, compile, его ошибки или native поведение.
No tests/fixtures/build/vet/product parser/types/SDK/browser/native/registry/
network/deploy/real logs/profile/runtime-config/vault/secret values read/run.

Это уменьшает данные в названных production messages/output, не generic secret
redactor и не no-logs/privacy acceptance. Existing raw SDK warn/error may carry
destinations/errors; observable lower-level stream отдельно, file append/рост/
ACL/retirement/квота открыты. Старые записи не удаляются. App record cap V336
не ограничивает native SDK file. Raw corebridge/failCore error-to-state/log и
другие domain producers ещё требуют адресного source audit. Пер-этапная fixed
строка не заменяет typed cause/error/control и не создаёт observation authority.

Следующий срез: central failCore/error-to-state/log и существующие typed finite
summaries до правок; SDK observable/retention остаётся отдельным обязательством.
Весь PC-00…09/old9, compile/native/browser/UX/release/comparative остаётся OPEN;
все gates=false, TUN unavailable/schema2 unsupported. Полный план не закрыт.
Evidence: .artifacts/memory/bootstrap-v337-log-privacy-source-evidence.json.


<a id="pc-v338-lifecycle-failure"></a>

## V2-338 — представление ошибок lifecycle без исходного текста ошибки

**Source/unverified, 2026-11-09.** PC-04/fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.4 Go/4 docs; V337 — progress.

### Источник и изменения

failCoreForIntent копировал stage+err.Error в AppState.Error и затем appendLog;
Stop/restart также выдавали raw cause в state. Trust branch имел fixed основную
причину, но дописывал cleanupErr.Error. Наличие реальной утечки не исследовалось;
source finding — возможность переноса произвольного filesystem/SDK error text
через эти producer boundaries. App record framing V336 не скрывает содержимое.

- Новый lifecycle_failure_text.go в App package: существующего общего safe
  lifecycle presentation helper не найдено. Этап выбирается из конечных меток;
  prefix switch распознаётся без копирования ID, unknown stage даёт fixed label.
  Detail использует original cleanup classification, existing trustFailureDetail
  и typed image/scope/cancel/deadline/TLS sentinels. Unknown/nil cause даёт fixed
  unknown. Нет Error()/fmt/рефлексии/string-based причин, I/O, новых владельцев,
  goroutines, схем или публичного API. Text не создаёт observation authority.
- failCore/Stop/restart вычисляют presentation и existing cleanup flag до App
  value mutex; final original intent/registration/closing guards остаются перед
  state publication. Cleanup/control используют исходный error и прежний
  localRetirementFailure; helper text не используется в retry/trust/Stop решениях.
  Не меняются Stop reason, protection release, retirement receipts и return err.
- Trust branch сохраняет существующий fixed trust detail и код; raw cleanup
  suffix заменён fixed notice. Cleanup-unconfirmed блокирует прежним способом.
- Corebridge previous-manager/WFP/render/cleanup log producers больше не
  копируют err.Error. Старт не копирует expected egress в свою status log строку.
  Watchdog не копирует ProbeReport.Summary с Names/Details; existing finite
  classifyFailureLogLine и Hint, typed reports/observations/health/control остались.

### Ручной разбор / остаток

Three original source diffs/one new helper, known/unknown/empty stages, dynamic
channel label, nil/unknown/joined causes, cleanup priority, trust reuse, outside-
value-lock preparation, late publication guards и original returned errors
просмотрены вручную. Corebridge log calls/finite diagnostic classifier также
прочитаны. gofmt4+1 observed exit0.8snap7existing1new/task71rev804 archived;
17 reviewed source SHA. Никаких tests/fixtures/build/vet/product parser/types/
SDK/browser/native/admin/network/real log/config/profile/vault/secret reads/run.

Это не generic redactor и не доказательство отсутствия приватных данных во всём
приложении. Raw Wails returned errors, остальные App/domain producers, reports,
SDK warn/error/observable, прежние log records и file retention остаются OPEN.
errors.Is/ошибочный custom error graph не имеют здесь hard latency/RSS гарантии.
Compile/API/message correctness/race/native/UI/UX ещё не проверены. Проценты
написанного кода не измерялись; 100 критериев приёмки остаются открытыми.

Next PC01/06: original SDK writer/observable lifetime, engine.log ownership и
retention до новой реализации; remaining raw returns/producers сохраняются в
объёме PC04. Полный PC00…09/old9 и release/comparative OPEN, gates=false,
TUN unavailable/schema2 unsupported. Evidence:
.artifacts/memory/bootstrap-v338-lifecycle-failure-source-evidence.json.


<a id="pc-v339-engine-log"></a>

## V2-339 — отключение неограниченного сырого SDK-журнала

**Source/unverified, 2026-11-09.** PC-01/04/06/fullPC00…09/old9 active;
100 OPEN/56 NOT_RUN, без waiver.5 Go/4 docs; V338 — progress.

### Аудит и решение

Cached sing-box v1.13.19 defaultFactory.Start открывает file с O_APPEND, 0644;
Box.Close закрывает logger последним после services/internalService. Ошибка
или unwind более раннего SDK этапа не доказывает close logger; current Windows
admissionBox/isolated process сохраняет negative cleanup, не повторяет Box.Close.
Замена на произвольный writer не является публичным box.Options seam:
PlatformLogWriter добавляет получателя, не заменяет default file/observable.
Warn V337 ограничивал writer, но observable отдельно Emit'ил lower levels.
Native file не имел принятой квоты/ротации/redacted record contract.

Поддержанный option.LogOptions.Disabled заставляет log.New вернуть NOPFactory
до построения defaultFactory/subscriber/file output. Cached NOP implements
ObservableFactory, Start/Close без file, NewLogger без emission, Subscribe даёт
os.ErrInvalid; Clash /logs handler отвечает NoContent. /connections использует
отдельный traffic manager; App вызывает только этот read-only endpoint.
NOP Level() возвращает Trace, поэтому SDK logDebug flag отличается; его текущий
api_meta branch и logger Fatal/Panic/NOP semantics прочитаны/ограничения отмечены.
Это source evidence, не доказательство runtime/метрик/API/security parity.

Вместо нового незащищённого sink штатная политика — отключённый raw SDK logger.
События lifecycle, typed probe/failure classes и bounded support preview App
сохраняются; произвольный SDK message не становится operational fact.

### Изменения

- config.LogConfig добавляет официальный disabled bool; renderer явно выдаёт
  log.disabled=true без raw output path/level. Nonempty WithEngineLogPath
  сохранён как callable signature, но даёт explicit unsupported после options
  и до secret-source/pin callbacks. Автоматического debug opt-in/fallback нет.
- sdkRuntimeConfiguration задаёт отдельный Log pointer с Disabled=true в private
  sdk copy. Original cfg/Log pointer/normalized bytes для subject/IPC/rollback
  не мутируются. Обе текущие SDK option-validation и box-construction ветки
  используют этот общий lowering; legacy raw config не включает raw sink.
- Corebridge не вызывает WithEngineLogPath, не создаёт logsDir при подготовке
  candidate. Удалён единственный os use/import этого файла. Runtime routing,
  DNS/service dial lowering, metrics options и forwarding/protection не менялись.
- App startup не запускает engine.log tailer, поэтому прежние записи не
  переносятся в новую App session. Старые файлы не открываются/не очищаются.
  Existing tailer API/original stop/join machinery остаются, без нового запуска.
  App log writer, finite lifecycle/probe messages и support projection сохранены.

### Ручной разбор / остаток

Five original diffs, rendered disabled flag→config parser→private independent
SDK Log copy→SDK disabled/NOP/observable contract, validation/construction callers,
production tailer/path callers и nil no-acquisition shutdown просмотрены вручную.
gofmt5 observed exit0.9 existing snapshots/task71rev807 archived;
17 reviewed source SHA и8 cached SDK SHA. Tests/fixtures/build/vet/product
parser/types/SDK/browser/native/admin/network/deploy/actual log/config/profile/
vault/secret values не читались/не запускались. Dependencies/module cache не
изменялись. Changed log semantics/legacy fixtures ещё не прошли compile/parity.

Это source removal конкретного raw SDK sink, не глобальный no-logs/privacy/RSS/
latency/retirement acceptance. SDK deprecation/global stderr/returned errors,
другие App producers/reports, connection metadata UI и прежние файлы остаются
отдельными. Debug/raw SDK logging недоступен до принятого bounded/redacted owner;
общий диагностический API остаётся, но полные SDK тексты не пересылаются.

Следующий PC06 finding: metrics.go напрямую Decode'ит неограниченный connections
response, shared client допускает redirects, Body.Close результат игнорируется;
GetMetrics не делает final runtime metrics-pointer publication check. Нужны
bounded intake/current-session/resource owner с сохранением useful diagnostics;
метрики не являются VPN/protection proof. Это source finding, не live результат.
FullPC00…09/old9/compile/native/browser/UX/release/comparative OPEN, gates=false,
TUN unavailable/schema2 unsupported. Evidence:
.artifacts/memory/bootstrap-v339-engine-log-source-evidence.json.


<a id="pc-v340-metrics"></a>

## V2-340 — ограниченный вход метрик и непрерывность счётчиков

**Source/unverified, 2026-11-09.** PC-04/06/fullPC00…09/old9 active;
100 OPEN/56 NOT_RUN, без waiver.2 Go/4 docs; V339 — progress.

**Поправка отчёта владельцу.** Прежние слова «0% по этапам» были некорректным
переводом незавершённой приёмки в процент реализации. Оценка0% отозвана.
Значительный объём исходников существует; доля реализации не измерена по
взвешенным подзадачам. Open acceptance/checks не являются таким измерителем.

### Источник и изменения

metrics.go делал прямой Decode неограниченного connections response и сортировал
через signed Upload+Download. Отсутствующие counters превращались в0, trailing
JSON не проверялся. Reset переустанавливал baseline, хотя комментарий обещал
сохранить накопленные totals. Частые samples стирали rate interval. Неизвестный
Start форматировался как00:00:00, пустой endpoint — как ':'. Возврат старого
metrics pointer после смены runtime не отвергался. Это findings по исходникам,
не наблюдение реального SDK response/профиля или результата VPN.

- metrics_snapshot.go адаптирует существующий private strict-JSON token walk
  delivery/model.go и metadata/accepted_profile.go; готового общего bounded
  telemetry intake не найдено. Trust packages не становятся зависимостью App UI.
  ReadAll ограничен2MiB+1; переполнение/ошибка/invalid UTF8/отмена отвергаются.
  До typed Unmarshal проверяются body shape, depth≤8/tokens≤262144,≤64 fields/
  generic-array members, ASCII key≤64 bytes, strings≤4096 bytes, numbers≤32 chars,
  connections≤2048. Duplicate keys включая ASCII typed-decoder case aliases и
  non-ASCII key aliases отклоняются. После одного root требуется EOF.
- Root uploadTotal/downloadTotal/connections обязательны; totals и per-connection
  upload/download — целые nonnegative int64, не null/implicit0. Connections:null
  поддерживает пустой SDK slice; каждый непустой элемент обязан быть object.
  Typed decode и повторная value validation происходят после shape admission.
  Over-budget/incomplete response не превращается в частичный успешный top20.
- pollContext принимает App lifetime, TryLock ограничивает один HTTP call на
  metrics session без очереди; старый poll signature оставлен internal. Endpoint
  только literal127.0.0.1/canonical nonzero uint16 port, secret непустой≤4096 bytes.
  Shared HTTP client не следует redirects, не включает automatic compression;
  response headers≤16KiB/MaxConnsPerHost1. Returned Body.Close error не допускает
  positive snapshot; closeAttempted ставится до вызова, без повторной Close при
  unwind. Native resource retirement этим не доказан и owner ещё не реализован.
- GetMetrics не начинает read без live App ctx, выдаёт fixed unavailable notes
  без raw error/endpoint. Перед выдачей повторно проверяет runtime.metrics exact
  pointer/App closing/context. Это as-of value check, не полная atomic gate для
  session/intent/path/epoch/image и не защищённый сетевой путь.
- apply повторно проверяет bounded/nonnegative values, ранжирует uint64 суммой
  двух int64 без signed overflow/stable ties. Отдельные seen counters обнаруживают
  regression даже при коротких rate intervals; last rate baseline обновляется
  после>0.2s, не стирая короткие дельты. При regression counters становятся unknown
  до нового metricsState; автоматическая новая база не подменяет прежний итог.
  Missing Since/Host не заменяются временем00:00 или фиктивным ':'. API/fields
  MetricsView/ConnView прежние; метрики не меняют protection/health/retry.

### Ручной разбор / остаток

One original diff/one new helper, complete/overflow/EOF, empty/null connections,
missing/negative/float/duplicate/alias counters, wide/deep/large values, top20/
max counter sums, short intervals/regression, fixed failures, App ctx/final pointer
и original caller signatures просмотрены вручную. gofmt2+2+1+2 observed exit0.
6snap5existing1new/task71rev810 archived/13reviewed source SHA. Один patch был
отвергнут без изменений из-за устаревшего whitespace контекста; исправлен по
актуальным строкам. Никаких tests/fixtures/build/vet/product parser/types/SDK/
browser/native/admin/network/real logs/profile/runtime-config/vault/secrets run/read.

Bounds относятся к intake/value retention, не total RSS/FD/latency/native calls.
Токеновый проход/Unmarshal могут аллоцировать ограниченные input-sized buffers;
внутренние Transport dial/read loops/idle connections и error graph независимы.
Per-session mutex не является process-wide quota. Требуются original HTTP task/
Dial/socket/body/Close receipt, retained unknown owner, Stop/Close join и full
intent/path/epoch/image gate до final baseline/view commit. Старый m может получить
свою baseline до final pointer refusal; новый m её не принимает. Это ещё OPEN.
Strict limits/SDK payload compatibility/negative cases/counter semantics/UI/
compile/runtime/native не проверены. FullPC00…09/old9/UX/release/comparative OPEN,
gates=false/TUN unavailable/schema2 unsupported. Evidence:
.artifacts/memory/bootstrap-v340-metrics-source-evidence.json.


<a id="pc-v341-metrics-owner"></a>

## V2-341 — владелец HTTP-метрик и единая публикация счётчиков

**Source/unverified, 2026-11-09.** PC-01/04/06, полный PC-00…09 и старые девять
этапов сохраняются. V340 — progress;100 OPEN/56 NOT_RUN, без waiver.
7 Go/4 docs;11 snapshots:9 existing/2 new, исходная task71/revision813 сохранена.

### Дефект и переиспользование

V340 защищал только один HTTP call внутри metrics session. Shared Transport мог
сохранять idle/read-loop/dial work, а смена metrics pointer разрешала другую
сессию одновременно с исходной. Счётчики изменялись до final pointer refusal.
Stop/terminal producers не учитывали этот запрос. Это вывод из исходников,
не зарегистрированная утечка или выполненный runtime failure.

- Переиспользованы appProbeSubject и порядок runtime→Manager→App→epoch из
  probe_subject/native_probe_scope/observer_publication. current сохраняет свои
  исходные predicates, новый lockCurrent отдаёт ту же lease; частично взятые
  locks отпускаются через defer при error/panic/Goexit. Готового telemetry
  owner не найдено: adapted original-attempt/one-Close/join pattern background
  и manual diagnostic, без их endpoint-health/admission/test semantics.
- metrics_scope.go захватывает applied metrics/config/Manager/engine, session,
  owner/generation, original intent source, private path/control/epoch и App ctx.
  Исходный probe report даёт identity, не переносит старый health PASS.
  Новая сессия, изменение пути/control, Stop/закрытие/quit, trust/profile/restart/
  image-denial/startup/watchdog flags отказывают публикации. Epoch0 не превращён
  в доказательство наблюдаемой физической сети. Native image revalidation нет.
- metrics_attempt.go резервирует один process-wide оригинальный запрос до
  watcher/Dial/Write/read. Старые и новые metrics sessions не обходят слот;
  refused contender не стоит в очереди и не запускает I/O. Внутренний legacy
  poll использует тот же слот; он не является App-bound production API.
- Прямой tcp4 Dial на canonical literal127.0.0.1 + фиксированный GET/connections
  с per-session Bearer/Connection:close. Secret bounded visible ASCII, без
  управляющих bytes в заголовке; адреса/секреты/ошибки не выводятся. Нет Transport,
  idle pool, redirects, proxy/DNS, auto compression/retry. До http.ReadResponse
  полный CRLF header framing≤16KiB, включая fragmented lines; buffered body не
  теряется. Общие accepted HTTP reads≤4MiB включают chunk/trailer framing;
  decoded JSON≤2MiB и V340 structural limits действуют отдельно. Это read-admission
  limits, не RSS/kernel buffering/wire hard bound. Content-Encoding отвергается.
- Retain raw socket даже при late/error Dial и response/body до результата
  cleanup. Один watcher реагирует на original ctx/intent/control и делит с
  finalizer одну native socket Close. closeStarted ставится до вызова;
  closeDone/returned/result принадлежат оригинальной попытке. Никакого повторного
  Close после неизвестного результата. Сначала cancel, socket Close и его join,
  затем body Close и watcher join; native calls вне value/registry locks.
- ReadResponse body использует только retained reader/socket; в прочитанном
  Go transfer.go Body.Close после drain записывает closed даже при returned
  read error. Поэтому после known socket Close normal body Close с drain error
  запрещает snapshot, но не объявляется неизвестным вторым native owner.
  Panic/Goexit/невернувшийся body/socket/watcher сохраняют исходный слот.
  Ordinary network/decode error после known retirement разрешает следующий опрос.
  Две секунды — request/deadline/join interest, не proof прекращения native call.
- Stop cleanup отменяет/joins original metrics до Manager.Stop; прежние deferred
  protection/Manager/helper stages и запрет Release при cleanup error сохранены.
  Terminal producer cancellation необратимо seals App metrics admission и
  передаёт точный original attempt в join, без отмены чужой/newer сессии.
  Seal и reservation синхронизированы App→registry: exit не пропускает late
  reservation и не меняет baseline одновременно с final seal. Cancel вне locks.
  done frame отдельно от retired outcome; timeout не очищает original slot.
- В production baseline/counter update теперь только после resource cleanup и
  повторной original scope lease. Sorting/top20/string allocation вне locks;
  applyPrepared внутри короткой value lease меняет только numerical state и view.
  Валидность/overflow/regression V340 сохранены. Для rate используется original
  completed-read timestamp. Данные остаются as-of telemetry; delayed Wails/UI
  delivery и actual physical egress этой lease не доказываются.

### Просмотр и оставшиеся gates

Просмотрены пять original diffs и два новых файла, cancellation/late Dial,
fragmented header, returned network/decode errors, body drain errors, abnormal
Close/watcher, Stop и terminal ordering, lock order/negative branch. После
просмотра уточнены fragmented-header accounting, abnormal lease release,
returned body drain vs unknown Close, nil/nil Dial refusal, secret header bytes и late reservation/terminal seal race.
Четыре gofmt7+7+7+1; последние три с explicit exit0, последний после seal correction.
Source/byte/manual evidence не устанавливает compile/types/race/native correctness.
Tests/fixtures/build/vet/parser/typecheck/SDK/CLI/browser/network/admin/native не
добавлялись/не запускались. Реальные профили/vault/env/logs/secrets не читались;
Android/shared/configs/harness/чужие чаты и исходные PLAN/task bytes сохранены.

OPEN: actual Go Dial cancellation internals/socket Close/FFI/deadline/resource
bounds, SDK /connections HTTP/chunks/trailers parity, Stop/terminal/race/Wails
ordering, UI late-response handling, full protected lifecycle/bridge/bootstrap/
OS/browser/accessibility/release/comparative gates. Все gates=false;TUN unavailable,
schema2 unsupported. Не заявлены VPN-приёмка, measured speed/reliability/superiority.

**Статус владельцу:**≈55% разработки ПК — грубая инженерная оценка по исходникам,
не взвешенный подсчёт по этапам и не процент проверенной работоспособности.
Прежние0% были ошибкой отчёта. Acceptance counters этого не измеряют.

**Следующий срез:** production metrics consumer в Vue/Wails: original request/
session ownership, отмена интереса при lifecycle/navigation и stale/empty/error
presentation без ложного health PASS; сохранять useful telemetry. Затем следующие
P0 PC-01/03/05 и Windows SDK/adaptive integration по полной матрице плана.


<a id="pc-v342-metrics-ui"></a>

## V2-342 — исходный запрос и сессия метрик в интерфейсе

**Source/unverified, 2026-11-09.** Полный PC-00…09/прежние девять этапов active;
V341 — progress.100 OPEN/56 NOT_RUN, без waiver.3 Go/5 TS/1 Vue/4 docs;
13 snapshots:12 existing/1 new, original task71/revision816 сохранена.

### Источник и изменения

StatisticsView использовал существующий serial poller с retirement при unmount,
но изменение lifecycle/channel не отзывалo ответ запроса на открытой странице.
Прежние суммы оставались видимыми, а rates могли сопоставить два сборщика одного
канала. В MetricsView не было collector/context receipt. Это findings по source,
не runtime trace, observed leak или выполненная приёмка.

- metrics.go: оба production collector constructors используют новый monotonic
  process-local uint64 ID в decimal string, без wrap; exhaustion отключает
  optional metrics. Не endpoint/secret/hash/health identity. metrics_scope требует
  reported running и ID. В original full value lease metrics_attempt получает
  existing immutable DiagnosticContext через diagnosticContextLocked до baseline
  и возвращает context+collector вместе со счётчиками. Отсутствующий/exhausted
  context отказывает positive commit. Native/physical identity не создаётся.
- contract.ts получает optional context/collector для совместимости типов старых
  bindings. backend.ts переиспользует nativeRecord и mapDiagnosticContext;
  прежняя decimal uint64 проверка вынесена в nativeIdentity с теми же пределами.
  Нет supported receipt — available=false, counters/rates=null, time/list empty.
  Никаких guessed IDs/default success. Wails JS wrapper передаёт raw object;
  новые поля читаются через existing unknown boundary. Generated bindings не
  исправлялись/не генерировались; реальная serialization/types compatibility OPEN.
  Старые binary/mock ответы без receipt не выдают metrics success; fixtures не
  изменялись, сравнение старого mock с working VPN не заявлено.
- createPoller сохраняет один pending original read/timer. Owner теперь opaque
  Symbol вместо числового счётчика; новый PollingRequest.current означает только
  интерес к continuation/publication, не abort или retirement Wails/native call.
  Zero-argument существующие loaders продолжают тот же API flow. Список прямых
  consumers и zero-argument getters просмотрен, compile/types ещё не проверены.
- usePolling optional scope synchronously stop→clear→restart, чтобы A→B→A не
  скрылась внутри batched render. Null scope pauses. Restart ждёт исходный
  pending call, не принимает его ответ и не делает параллельный read. Changed
  scope/hidden/unmount очищают numbers/receipt/interval вместо stale display.
  Current document visibility проверяется также before-load/before-publication;
  handler/watch/timer сняты onBeforeUnmount. Только statistics opts включают
  visibility pause; другие страницы сохраняют существующий default polling.
- Receipt time/error теперь пишутся до data. Это нужно sync data watchers:
  прежний порядок позволял использовать timestamp предыдущего ответа. Смена
  scope и очистка работают до render; это не atomic OS measurement или гарантия
  frame/event timing браузера. Явный invalidate очищает старую видимую запись.
- Новый composable coreMetrics оправдан отсутствием пригодного metrics consumer
  с original scope/receipt/rate ownership. Он композирует root backendState,
  usePolling, diagnosticContextKey и history, не создаёт второй state subscription,
  router/store или backend. Проверяет lifecycle/context/mode/channel/current intent,
  known path denial и UI action busy. Unknown path0 не выдаёт physical-path PASS.
  Positive receipt должен совпасть с original/current context. Mismatch не retag:
  только ещё current request может попросить existing root refreshState; поздний
  ответ после unmount/hidden/scope retirement не инициирует follow-up. Старая
  версия без scope честно недоступна. Error не становится нулевым трафиком.
- Existing trafficHistory получает createCoreTrafficHistory: interval лишь для
  двух safe nonnegative cumulative samples одного collector/context с возрастающим
  backend time. First/missing/duplicate/regressed/future time resets interval;
  actual equal counters за positive interval дают known0. Равное имя канала не
  склеивает новый collector. StatisticsView становится consumer: отдельные
  adapter/core readings, context/visibility reset, aria-busy, понятная first-sample
  подсказка без opaque ID на пользовательском экране. Стили/навигация не менялись.

### Просмотр и ограничения

Применены frontend-postgres и modern-web-guidance: existing components/composables/
contracts/tokens просмотрены; lookup/retrieve local guide detect-initial-visibility-state
без установки/внешней сети. Guide про initial analytics не доказывает начальную
видимость: здесь используется current visibility/event только для scheduling,
не performance timeline, first-background time или precise historical visibility.

Просмотрены восемь source diffs и новый coreMetrics: original request interest,
A→B→A, pending restart, скрытие/unmount, mismatch/unsupported receipt, decimal ID
bounds/exhaustion, first/duplicate/reset/missing intervals, getter signatures,
zero-vs-unknown, receipt/data order. Это ручной просмотр, без исполнения продукта.
Два touched-only gofmt3 с explicit exit0. Не запускались tests/fixtures/build/vet/
parser/typecheck/formatter TS/Vue/Wails generation/SDK/browser/OS/network/native.
Android/shared/configs/harness/зависимости/generated/чужие чаты не изменялись;
профили/vault/env/runtime-config/реальные logs/secrets не читались.

OPEN: actual compile/types/generation compatibility, all existing poller consumers,
Vue sync watcher/visibility/Wails event ordering, OS measurement identity of adapter,
widget focus/a11y/responsive/browser/WebView2, Go original owner/native deadlines/
Close/Dial/FFI/RSS/SDK wire, full P0/protected lifecycle/release/comparison. Receipt
is negative UI correlation, not positive egress/trust/forwarding authority. Source
work не закрывает эти gates;100OPEN56NOT_RUN/gates=false/TUN unavailable/schema2
unsupported.55% — прежняя грубая инженерная оценка, не новое измерение прогресса.

**Следующий срез:** вернуться к PC-01/03 W4c original-helper/WFP/egress contract:
сверить текущие P0 source gaps и закрыть следующий реальный seam из полного
PC-PLAN/PLAN §5.1. Сохранить незакрытые metrics compile/runtime/UI gates; далее
Windows trust/adaptive integration и остальные PC-00…09, без тестирования.


<a id="pc-v347-lifecycle-ui"></a>

## V2-347 — доступность команды отдельно от готовности соединения

2026-11-09. PC-04. **Partial source, unverified**.

**Дефект по исходникам:** startup AppState и successful Stop выставляют
CoreReady=false, но frontend primaryAction трактовал этот флаг как запрет Start.
Это смешивало ещё не запущенное ядро и невозможность принять команду. Кроме того,
frontend знал только часть BlockedReason и скрывал необходимость explicit Start
при proxy conflict после ручной остановки. Это source finding, не runtime trace.

**Изменения:**

- lifecycle_actions.go содержит actual local Start/Stop predicates. Public
  handlers и presentation используют те же условия. Start дополнительно
  отклоняет reloading/unknown lifecycle; unknown rejection не получает default
  success. StopQuit closing exception и original registration/release/source
  guards сохранены. Ошибки строятся только для actual command, не snapshot.
- Optional AppState.lifecycleActions schema1 содержит decimal intent, lifecycle,
  Start/Stop request availability и finite reason, explicit-start requirement.
  Это только local request registration, не завершённая операция или authority
  Engine/native/WFP/cleanup/traffic. Actual handler повторяет свои проверки;
  все последующие native/trust/protected gates остаются обязательными.
- Facade через existing nativeRecord проверяет optional schema, exact existing
  DiagnosticContext intent/lifecycle, bools и finite reasons/consistency. Missing,
  old/unsupported или malformed action facts дают null, без fallback к CoreReady.
  Existing generated JS GetState — passthrough; generated sources не менялись.
- Finite BlockedReason catalog покрывает текущие profile/trust/cleanup/update/
  restart/proxy codes и отдельный unrecognized. Unknown/malformed reason не
  превращается в отсутствие блокировки и не выводится как raw code/property key.
  Labels и next-step guidance переиспользуются в существующей главной странице.
- Existing backendState.primaryAction использует новые request facts. Missing
  positive data не убирает negative Stop escape; committed restart/release/close
  и pending Stop учитываются. startVpn повторяет display denial перед dispatch.
- Snapshot сохраняет proxy_conflict после manual Stop при отсутствии более
  приоритетной причины. UI объясняет explicit Start, не перекрывая cleanup
  failure заголовком proxy. Stop и снятие защиты описаны как запрос после
  cleanup, не успешный release. Refresh работает и при unknown/pending status.
- Existing single polite status region, semantic buttons, wrapping layout и
  components сохранены. aria-describedby связывает controls с next step/effect;
  нет новой карточки, UI framework, router, root store, timer или зависимости.

**Просмотр:** manual8 original source diffs+1 new, shared predicate callers,
Start/Stop/reloading/closing/profile/commit/unknown branches, optional old/missing/
invalid/context-mismatch presentation and negative escape, finite codes, existing
Wails JS passthrough.14 reviewed sources unchanged. gofmt4 explicit exit0.
Modern Web Guidance accessibility guide применён к existing status/live region/
semantic controls и описанию эффекта; это не accessibility acceptance.
4Go4TS1Vue4docs/13snap12existing1new. Tests/fixtures/build/vet/types/product parser/
SDK/generation/browser/native/admin/network и actual owner data не запускались.

**OPEN:** compile/types/current generated-model/Wails/WebView2/keyboard/focus/
small-window/DPI acceptance, actual old-client migration, all eight journeys and
all pages. Request metadata может устареть сразу: не подтверждает original
command completion/retirement/applied result. Existing numeric frontend action
generation и post-command fresh-state reconciliation требуют отдельного среза;
не объявлять успешное переключение только по resolved dispatch. Source/SHA/
gofmt не закрывают protected egress или original native gates. W3b/NETGUARD
migration/native/SDK/adaptive/OS/resources/release/comparative остаются OPEN.
FullPC00…09/old9/100OPEN56NOT_RUN/gates=false/TUN unavailable/schema2 unsupported;
55% — прежняя грубая оценка реализации, не измеренная готовность.

**Далее:** original frontend lifecycle request owner и post-command freshness,
переключение без premature applied toast; затем полная PC-04/PC-01/SDK/adaptive/
OS/release matrix. Запрет тестирования и исходные обязательства сохраняются.


<a id="pc-v348-lifecycle-requests"></a>

## V2-348 — исходные UI-запросы, порядок Stop и свежий ответ состояния

2026-11-09. PC-01/04. **Partial source, unverified**.

**Дефекты по исходникам:** frontend numeric action generation не отзывал late
feedback после root dispose; finally старой команды мог сбросить общий busy.
Resolved SelectChannel сразу выдавал «Канал переключён», без свежего состояния.
Обычный независимый Start RPC, доставленный после следующего Stop, мог получить
новую App registration. Одного frontend token для серверного порядка недостаточно.

### Backend и существующие исполнители

- ui_lifecycle.go добавляет OpenUILifecycle/RequestUILifecycle. Один process-local
  random session, canonical positive decimal uint64 sequence/high-water, finite
  action/channel shape. High-water и actual App Start/Stop registration atomic
  under App.mu; newer rejected request тоже запрещает older queued work. Same-
  session duplicate/out-of-order и old-session запрос не создают новую работу.
- Оригинальные positive/Stop callback slots сохраняются до ordinary return;
  unknown/panic/Goexit не выдаётся за их завершение. Open новой сессии не сбрасывает
  слоты и не отменяет существующий Engine. Stop может отменять pending positive.
- Start использует прежний private registration; Stop — прежний registered Stop;
  select получает точный captured intentRegistration до ожидания operationMu.
  Common switch outcome/body и все existing source/trust/cleanup/native/WFP gates
  сохраняются. After activation публичные legacy Start/select не обходят порядок;
  explicit legacy Disconnect отзывает session, сохраняя enabled marker и owners.
- Finite structured outcome registered/returned/rejected/failed/superseded/
  unconfirmed — только command registration/ordinary callback return. Здесь нет
  native resource/admission/whole-cleanup/applied/egress authority. Raw errors,
  endpoints, paths и channel names не сериализуются в receipt.

### Frontend

- Existing generated namespace через optional Reflect getters; generated files
  не менялись. Missing methods блокируют positive dispatch. No raw window.go или
  legacy positive fallback. Original Open Promise retained through interest
  revocation; exact decimal increment has ceiling/no Number conversion/wrap.
  Response schema/session/sequence/action/intent/outcome checked against request.
- Для explicit Stop допускается прежний negative API при missing session/method,
  counter exhaustion или known superseded result. Unknown outstanding RPC не
  заменяется автоматическим rescue call. Legacy response не создаёт fake receipt.
- Root/action/event/read/channels owners теперь opaque Symbols. Original positive
  и Stop promises удерживаются отдельно и освобождаются только original finally;
  root dispose отзывает публикацию, не «отменяет» native call и не очищает owners.
  Late old result не очищает successor slot и не публикует toast/feedback.
- Единственный latest state demand вместо списка ожиданий: post-command GetState
  ждёт original read без наложения, затем загружает текущий demand. Source decimal
  diagnostic revision floor и full same-revision key запрещают regress/equivocation;
  reported intent должен достигнуть receipt intent. Failed/missing refresh не
  объявляет применение; manual refresh/negative Stop доступны, automatic read
  retry loop/timer не введены. Floor относится к тому же backend process/module.
- Убрано premature applied switch toast. Existing status/HintBox показывают finite
  command feedback; command reply отдельно от fresh state/protected path. Explicit
  Stop доступен при unknown state, Start/Switch pending или backend stopping;
  original pending Stop и реальные commit barriers не обходятся. No new cards,
  UI framework/router/store/dependency; existing semantic/layout components reused.

**Просмотр:** manual10 original source diffs+1 new, original admission/lock/callback
ownership, queued Start-before/after Stop, exact switch source, stale/duplicate/
unknown outcomes, original Open/sequence/request root retirement, single state
demand/context floor/minimum intent, finite feedback and negative escape.
17 reviewed sources unchanged; gofmt5 explicit exit0. Tests/fixtures/build/vet/
types/product parser/SDK/generation/browser/native/admin/network/real data не
добавлялись/не запускались. Нет runtime proof порядка или transport cancellation.

**OPEN:** штатная generation и совместимые generated exports/types/Wails/Go build;
текущие generated exports ещё не содержат новые methods, positive feature остаётся
unavailable до разрешённой доставки. Native/FFI/ABI/Goexit/cancellation/resource/
bridge-response/renderer remount/HMR/old-client migration, same-process context
identity, whole eight journeys/all pages/keyboard/DPI/WebView2 acceptance.
Per-call/slot shape не доказывает whole API/RSS/rate/hard deadline bounds. Наличие
server session не является renderer authentication, traffic trust или разрешением
переписать внешний proxy. W3b/NETGUARD/native/SDK/adaptive/OS/release/comparative
остаются OPEN.100OPEN56NOT_RUN/gates=false/TUN unavailable/schema2 unsupported;
fullPC00…09/old9 active;55% — прежняя грубая оценка, не измеренная готовность.

**Далее:** полный PC-04 action/presentation matrix для profile/mode/update/support
и оставшихся страниц; PC-01/native/W3b/SDK/adaptive/OS/release obligations сохраняются.


<a id="pc-v349-mode-actions"></a>

## V2-349 — фактический режим, команда и новый процесс

2026-11-09. PC-01/04. **Partial source, unverified**.

**Дефекты по исходникам:** failed IsTUNMode load превращался в false/прокси;
header выводил «TUN · путь проверен» из running и boolean mode. Settings смешивал
applied proxyMode и флаг процесса, два экрана дублировали mode controls. Resolved
mode call показывал restart toast; rejected call освобождал busy даже при
неизвестном исходе передачи. Legacy mode RPC мог принять более поздний Stop за
новое намерение, если callback доставлялся с задержкой. Это source findings.

### Изменения

- mode_presentation.go/RuntimeInfo.ProcessMode: schema1, actual os.Getpid bound
  to outer RuntimeInfo PID, immutable process flag, existing native TUN/elevation
  admission predicates. Это факты/negative gates, не proxy/adapter/child/egress
  proof. Core flags и accepted/native constants не менялись.
- Shared modeRequestRejection сохраняет прежний early deny before Stop/UAC.
  restartIntentLocked выделяет те же original predicates. Ordered mode admission
  captures exact registration under App.mu, private switchModeRegistered повторяет
  source/preflight и передаёт original intent в прежний beginRestartHandoff.
  Legacy positive mode calls после protocol activation не обходят sequence fence.
- UILifecycle handlerEntered marker: false+known rejected/superseded mode означает
  отсутствие входа именно в mode body; missing older marker = null. Ни этот bit,
  ни registered/returned, PID или data refresh не удостоверяют native retirement,
  child readiness, новый режим или защищённый путь. Initial maintenance intent0
  допустим для mode receipt, не для Start/select forwarding authority.
- Facade строго проверяет optional mode/PID/schema/bools и gate consistency;
  старый IsTUNMode также не принимает non-bool. Mode calls используют existing
  optional ordered namespace, без legacy positive/raw window.go fallback.
- BackendState переиспользует serial createPoller без таймера: original pending
  read не заменяется при stop/start/unmount. Missing/error/old facts = null,
  tunMode теперь boolean|null. Original pid/mode anchor запрещает silent source
  change в том же module; receipt time отдельно от validity/native evidence.
- Mode request занимает тот же original positive slot, связывает root/action
  interest и client dispatch boundary. Выбранный target не меняет observed mode.
  Known rejection before body может завершить попытку; otherwise unknown/entered
  error/awaiting new process retains negative modeHold, в том числе после dispose.
  Old callback не публикует positive feedback в новый root. Stop remains negative
  escape; original callback/native source checks сохраняются. No success toast.
- ProcessModeSetting создан после поиска: обе прежние реализации одинакового
  mode row заменены одним компонентом. Existing AutostartSetting style/tokens/
  semantic controls reused: wrapping, min-width0, focusable aria-disabled with
  dispatch guards, unique caller help IDs, no new card/framework/store/dependency.
  aria-busy относится к reader/original request, не к idle unknown outcome.
  Duplicate hold text не выводится дважды. SectionHeader больше не повышает
  reported running до TUN proof; actual process mode не равен active proxy state.

**Просмотр:** manual10 original source diffs+2 new, preflight/legacy/ordered mode
paths, exact source before handoff wait, maintenance intent0, unknown callbacks/
missing marker/late reply, serial reader/root retirement, PID binding/anchor,
unknown vs false, two consumers/header and finite outcomes.18 reviewed sources
unchanged. Pure existing native/broker admission predicates read; no native calls.
gofmt5 explicit exit0. Modern Web Guidance accessibility/responsive guidance и
existing Autostart layout использованы; это не visual/a11y acceptance.
5Go3TS4Vue4docs/16snap14existing2new. Tests/fixtures/build/vet/types/product parser/
SDK/generation/browser/native/admin/network и actual owner data не запускались.

**OPEN:** compatible generated exports/models/types/compile and actual Wails
delivery; old/missing facts unavailable. Native child/elevation/privilege drop/
UAC/Job/custody/parent exit/Goexit/cancel/order/bridge/resource/RSS/OS acceptance.
PID/immutable flag не являются original native identity/custody. Same-module anchor
не доказывает HMR/backend migration. Entered/transport-unknown mode hold не снимается
по одному новому display snapshot или ordinary Stop reply: original handoff/
cleanup reconciliation и новый процесс остаются отдельными gates. Не заявляется
подтверждённый restart или автоматическое восстановление этой неоднозначности.
All pages/eight journeys/keyboard/focus/small-window/DPI/WebView2/native/SDK/adaptive/
W3b/NETGUARD/release/comparative OPEN.100OPEN56NOT_RUN/gates=false/TUN unavailable/
schema2 unsupported/fullPC00…09/old9 active;55% rough/unmeasured only.

**Далее:** PC-04 profile/update/support action outcomes и оставшаяся matrix;
PC-01 native/handoff/recovery, SDK/adaptive/OS/release obligations сохраняются.


<a id="pc-v350-update-owner"></a>

## V2-350 — пакет, подтверждение и исходный вызов обновления

2026-11-09. PC-04/08. **Partial source, unverified**.

### Что было видно в исходниках

1. CheckUpdate/ApplyUpdate связывались только строкой каталога. Apply повторял
   подпись/hash, но другой корректно подписанный пакет в том же каталоге мог
   заменить именно выбранную в preview версию. Это дефект согласования цели,
   не доказанный обход подписей или проведённая установка.
2. Preview не вызывал checkVersionPolicy явно, хотя заявлял проверку версии.
3. ApplicationUpdate хранил busy/directory/preview только внутри disclosure.
   Закрытие уничтожало owner и позволяло новому компоненту начать второй вызов.
4. UI принимал truthy OK/Number coercion и raw backend reason. Применение шло
   по legacy directory RPC отдельно от общего ordering/positive slot.

### Реализация

- update_approval.go: один original preview callback и один bounded process-local
  approval. Existing publicSourceToken выдаёт nonce; retained scalar hashes bind
  raw update.json и exact delivery.json (включая отсутствие sidecar), private
  directory, exact original intent pointer/ID и network path, UI session.
  Monotonic time.Time ограничивает согласие 5 минутами. Это выбор цели, не
  authenticated UTC/trust/native install/child/egress authority.
- CheckUpdate захватывает restart registration и original path до operationMu,
  использует existing beginUpdateOperation/endOperation, не включает protection
  или сетевые producers. Stop/epoch/source/session replacement отвергает approval.
  Полные прежние trust/source/payload checks + checkVersionPolicy до OK/token.
  Причины отказа передаются конечным ReasonCode, без raw error/path в UI.
- New renderer/legacy Disconnect инвалидирует approval. Matching token одноразово
  consumed под тем же App.mu, что ordered sequence/high-water/positive reservation.
  update command несёт только token, не произвольный directory. Исходный restart
  registration передаётся private apply до ожиданий updateMu/operationMu.
- prepareVerifiedUpdateContextApproved сверяет manifest digest сразу после read,
  sidecar digest до DecodeTargetPackage/SDK prepare/accepted-prefix retention.
  Замена на иной valid signed target не получает его prefix retention от старого
  approval. Затем выполняется полная прежняя private trust/version/hash/source
  проверка; approval не служит её кэшем. Existing observed-prefix retention на
  ошибках того же approved target остаётся, поэтому отказ не означает no changes.
- Legacy ApplyUpdate после активации ordered protocol отвергается; pre-protocol
  entrypoint сохранён для прежнего callable контракта и не используется новым UI.
  Raw window.go/legacy positive fallback не добавлены, generated stubs не правились.
- UpdateInstallEntered — optional original body phase marker. False + ordinary
  rejected/superseded/failed означает, что этот callback не входил в install/staging
  body; это не receipt очистки, native custody или неизменности trust history.
  Missing marker/unconfirmed/entered failure/ordinary return после install не
  трактуются как безопасное повторение: frontend сохраняет negative hold.
- backendState: тот же original positive slot, root/action interest и Stop ordering.
  Hold сохраняется при root disposal и не снимается display snapshot/Stop reply;
  receipt не включает installed version. Unknown outcome не запускает auto retry.
- applicationUpdate.ts выделен, потому что существующий компонент не сохранял
  original pending вне disclosure. Один native picker/check/apply живёт до actual
  return, при unmount не заменяется. Latest viewer/root/session interest отдельно:
  late picker не запускает Check, late session-open не отправляет Check/apply,
  late Check не создаёт visible preview. New viewer видит original busy; completion
  даёт только demand перечитать profile/runtime facts, не adopts старый preview.
- Facade строго проверяет optional schema/token/digests/intent/bools/size; missing
  approval -> unavailable, size unavailable -> null. Применение только через ordered
  RequestUILifecycle. Existing ApplicationUpdate/HintBox/style tokens развиты без
  новых cards: отдельные phase/error/preview/outcome, сроки/версия/hash строками,
  button names/descriptions и guard для aria-disabled, wrapping без fixed width.

### Evidence и пределы

Manual source review: nine original source diffs + two new source files; targeted
original source/operation/phase/retention/consumer/Stop/import-cycle review.
17 pinned reviewed inputs unchanged при финальном FS receipt. Touched-only gofmt:
5 файлов, затем 1 изменённый token-size guard, explicit exit0. Это форматирование,
не compile/behavior/native/SDK proof. Modern Web Guidance accessibility guide и
existing semantic controls/tokens использованы без visual/a11y acceptance.

5Go4TS2Vue4docs/15snap13existing2new. Tests/fixtures/build/vet/types/product parser/
SDK/generation/browser/native/admin/network/runtime data не запускались.
Exact-target approval не доказывает hostile namespace/TOCTOU custody, payload/
loaded-section identity, original child/Job retirement, crash/power-loss durability,
UAC/privilege/relaunch/GUI readiness/installer recovery. Schema2 всё ещё unsupported;
original native/child/bootstrap acceptance gates не менялись и остаются false.
Known pre-install failure не удостоверяет consistency/retirement SDK storage; новая
проверка всё равно проходит прежние guards. Unknown install hold требует actual
handoff/recovery/new process; его reconciliation ещё OPEN, не заменён UI booleans.
Generated exports/models/types/compile/actual Wails delivery и native window/DPI/
keyboard/focus/screenreader/all-pages/eight-journeys/wholeUX остаются OPEN.
FullPC00…09/old FR/TH/OFF/COM/RQ/TR/TH00…08/100OPEN56NOT_RUN сохранены; W3b/
NETGUARD/SDK/adaptive/OS/release/comparison OPEN.55% rough/unmeasured only.

**Далее:** PC-04 profile/support action ownership и оставшаяся matrix;
PC-01/native handoff/recovery и PC-08 real installer obligations сохраняются.


<a id="pc-v351-profile-owner"></a>

## V2-351 — original UI owner импорта, экспорта и списка отзывов

2026-11-09. PC-04. **Partial source, unverified**.

**Source findings:** ProfileTransfer/PublicProfileSource/PinPolicySource держали
pending в локальной view. После disclosure unmount новая view могла отправить
второй picker/check/apply/export/save. Public/pin previews делят один backend draft,
но старое UI preview оставалось пригодным на экране после замены другой панелью.
Некоторые save ответы проверяли только mounted. Encrypted preview использовал
JSON.stringify transport/password/режима/approval fields и сохранял extra строку
с паролем для сравнения ввода. Эти наблюдения не являются runtime воспроизведением.

### Изменения

- Новый profileActions.ts в существующем Vue composables слое: один bounded
  OriginalAction с owner/token Symbols и finite kind, shared selected revision,
  actual-completion demand. Никаких password/transport/endpoint/approval tokens/
  preview/receipt payload в общем owner. Отдельного backend state store/engine/
  trust/protection authority не создано; applicationUpdate owner не подходит
  для этих самостоятельных API/workflows и не дублируется новым framework.
- App root активирует/retire display lifetime. Panel/root unmount не заменяет
  pending, не отменяет Wails и не снимает busy. Только actual original finally
  освобождает соответствующий pointer; pending/error не превращаются в timeout
  success. Нет таймера, waiter list, replay или автоматической повторной отправки.
- Три существующих компонента резервируют общий slot до native вызовов; внешний
  busy блокирует conflicting controls. Selected revision инвалидирует previous
  preview/results других панелей, matching original scope не отменяет себя.
  Existing public/policy cancellation использует прежний request ID: запрос
  отмены не освобождает original slot, apply не отменяется как обычный preview.
- PublicProfileSource сохраняет root/view/input generation для preview/apply/
  export/save. Save captures selected transfer и current generation; late save
  не публикует filename как результат другого выбранного пакета. На unmount
  очищаются expiry/пароль/export/receipt/target-confirmation refs. Public profile
  ordinary return показан как ответ на применение и demand перечитать accepted
  version, не доказательство applied engine или защищённого подключения.
- PinPolicySource сохраняет прежнюю exact version/package/digest/committed check,
  expiry/focus/cancel behavior. Общий original slot и root/view guard добавлены
  вокруг preview/apply, включая late nextTick focus. Местный cancel не снимает
  loading, пока original callback не вернулся. Native cancellation не доказана.
- ProfileTransfer импорт сохраняет scope через file-picker→decrypt/check.
  Editing input инвалидирует late error/result; programmatic selected transport
  assignment не считается user replacement. Opaque inputRevision/previewInput
  Symbols заменяют JSON/password snapshot. Exact actual request и SDK проверки
  прежние; plaintext может оставаться в original Wails request/closure до возврата.
  Credential zeroization, RSS bound или отсутствие plaintext в памяти не заявлены.
- Export/apply inputs и displayed QR/transport/preview/result refs очищаются при
  закрытии/замене view; оригинальный отправленный вызов продолжает жить отдельно.
  Parent один раз наблюдает shared completion demand и вызывает существующий
  Settings refreshSummary/serial readers; старое preview/result не переносится
  в новую view. Child changed API сохранён, повторная parent forwarding убрана.
- Existing components/tokens/expiry/semantic controls сохранены. Actual shared
  pending label/aria-busy, explanatory next step, min-width/wrapping; внешняя
  card переноса убрана. Новый action заменяет прежнее preview, что указано в UI.

### Evidence и открытое

Manual four original Vue diffs + new TS source; original pointer/selection/root/
input/cancel/import-cycle/late picker/late save/consumer review.15 pinned reviewed
sources unchanged при final FS receipt. Go не менялся, gofmt неприменим.
1TS4Vue4docs/9snap8existing1new. No tests/fixtures/build/types/parser/generation/
browser/native/SDK/network/admin/real-data execution. Accessibility guidance и
existing controls/tokens использованы без visual/keyboard/screenreader acceptance.

Это current-module original frontend workflow guard, не interprocess/backend
hard bound или original native owner receipt. Direct API callers/old renderer/
HMR/new JS realm/uncertain callback/native SDK storage retirement остаются своими
P0 gates. Ordinary RPC failure может следовать за сохранённой delivery history;
новая попытка всё равно проходит прежние private backend checks. Закрытие view
не отменяет уже отправленное positive действие. Backend ordered mutation/source
registration/public-source missing-lifetime/cancel и полное reconciliation ещё
OPEN; исходники backend/SDK в этом срезе не изменены. Общий owner не получает
власть над Start/Stop/update и не заменяет их native guards/ordered protocol.

Generated/compile/actual Wails delivery, all pages/eight journeys/focus/keyboard/
small-window/DPI/WebView2/wholeUX, native profile/SDK/OS/installer/handoff/recovery/
W3b/NETGUARD/adaptive/release/comparison OPEN.100OPEN56NOT_RUN/all gatesfalse/
TUN unavailable/schema2 unsupported/fullPC00…09/old9 сохраняются.55% rough only.

**Далее:** public-source original lifetime/cancel/handler boundaries и support
clipboard original owner; остальные PC-00…09/TH-00…08 обязательства не сняты.


<a id="pc-v352-public-lifetime"></a>

## V2-352 — original backend flight и настоящий App lifetime

2026-11-09. PC-01/04. **Partial source, unverified**.

**Source findings:** beginPublicSource/ExportPublicProfileSource заменяли missing
App context на Background. Preview не был связан с исходным intent/path. Общий
source slot описывался busy/applying/cancel; deferred cleanup освобождал booleans
после любого unwinding, включая interrupted handler, без original pointer match.
Cancel вызывал context callback под state mutex. Это source findings, не runtime
или native воспроизведение; не доказанная эксплуатация/утечка трафика.

### Реализация

- public_source_lifetime.go: один App-owned publicSourceFlight, actual context,
  original intent registration, request ID, cancellation/watcher и apply phase.
  publicSourceBase переиспользует реальные restartIntentLocked predicates и
  отказывает при missing/canceled/closing/restart/release/changed-image source.
  Никакой replacement App lifetime или authority не создаётся.
- Preview captures actual contextForIntent negative path/intent sources до
  protected/local/bootstrap/file I/O. Один passive goroutine ждёт original source
  Done или child context Done, затем заканчивается. Slot reserved до spawn;
  watcher joined before normal retirement. Deadline45s и Cancel не снимают active,
  пока original handler не вернулся. Physical epoch revocation здесь value fence
  before publication; это не synchronous I/O/kernel cancellation guarantee.
- runPublicSourceFlight вызывает исходный private body на том же goroutine.
  Ordinary return освобождает только exact matching active pointer. Panic/Goexit/
  interrupted frame отменяет свой context, join watcher, очищает pending и
  сохраняет uncertain original slot. Нет recovery goroutine, timeout replacement,
  повторного вызова SDK или native cleanup. Error не становится успехом; panic
  payload не выводится. Goexit не превращён в returned RPC receipt.
- Profile/pin publication проверяет active pointer/request ID/current original
  context и сохраняет только bounded прежний public draft. App parent и original
  intent сохраняются отдельно от короткого preview child, который отменяется
  при завершении preview. SHA/version/role/source/private SDK approval прежние.
- ApplyPublicProfileSource/ApplyPinPolicySource/ExportPublicProfileSource имеют
  exact-flight wrappers и прежние private bodies. Canonical bounded token/purpose
  сопоставляется и consumes once; registration/parent/preview shape проверяются
  до body. Старое намерение после Stop до admission требует нового preview.
  Полная private проверка/expiry/independent root-target approvals/credential
  snapshot/retention/version/native guards выполняется снова как прежде.
- Mutation получает actual App parent с 90s child, не nil и не уже canceled
  preview child. Preview alone watches original Stop source. Apply/export сами
  могут намеренно retire profile при смене policy/prefix, поэтому к ним не
  подключён watcher, который сам отменял бы такую легальную остановку. Это
  сохраняет existing cutover semantics, но не закрывает captured registration
  through operationMu или explicit own-retirement handoff: следующий P0 остаётся.
- CancelPublicProfileSource захватывает actual original pointer/cancel под коротким
  mutex, очищает matching preview metadata, вызывает только этот cancel вне locks.
  Не освобождает active, не отменяет approved apply/export и не удостоверяет
  closure/retirement. Late cancellation callback не ищет successor по state заново.
  Correlation ID не является authority; reused IDs/other callers требуют своего
  session/ordering контракта. Request ID не заменяет pointer matching cleanup.
- Public API signatures/DTO остаются прежними, generated stubs не редактировались.
  Frontend V351 original admission сохранён; Go flight — его backend boundary,
  не второй engine/SDK/trust authority. Public-source paths больше не получают
  Background; общие legacy import/pin helpers с другими callers не переписаны.

### Evidence и пределы

Manual three original Go diffs + new scope helper; all production publicProfiles
consumers/begin calls, pointer lifecycle, watcher join, lock order, imports,
separate preview/mutation contexts и private cutover seams просмотрены.
15 pinned reviewed inputs unchanged при final FS receipt. Touched-only gofmt4,
then2, then1, explicit exit0.4Go4docs/8snap7existing1new. No tests/fixtures/build/
vet/SDK/parser/types/generation/browser/native/admin/network/runtime-data work.

Ordinary return/watcher join не доказывают SDK subprocess/native handle/storage
retirement, durability, no-side-effects-on-error, cancellation latency, RSS или
interprocess hard bound. Interrupted slot требует реального recovery/process
lifecycle; флаг UI не освобождает его. Current module/old callers/native input
chooser/real HTTP/Goexit/panic/Stop/timeout/resource acceptance не выполнялись.
Unpaid VPS не использовался. Никакого direct fallback/выключения защиты/сети.

**OPEN:** carry mutation registration captured before wait through operationMu
and actual profile retirement, then explicit private source handoff for own Stop;
не усыновлять более поздний manual Stop. Сейчас apply/export check это делает
только at admission; downstream common helpers имеют свои старые checks. Existing
prefix retention может уже изменить историю до отказа; source wrapper не rollback.
Legacy missing-context fallback paths вне public-source wrappers, finite UI
reporting of interrupted source state, support clipboard owner, SDK/OS/native
retirement/wholeUX/installer/handoff/recovery/W3b/NETGUARD/adaptive/release/comparison
остаются OPEN.100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/
fullPC00…09/old FR/TH/OFF/COM/RQ/TR/TH00…08 сохранены.55% rough/unmeasured only.

**Далее:** mutation registration through operationMu/own cutover handoff; затем
support clipboard original owner. Полная цель не заменена этим source срезом.


<a id="pc-v353-profile-registration"></a>

## V2-353 — исходный запрос через ожидание и собственную смену профиля

2026-11-09. PC-01/04. **Partial source, unverified**.

**Source defect:** common profile/pin import and enrolled export читали operationID
после ожидания operationMu; late explicit request мог усыновить более поздний
manual Stop. Public-source admission V352 ограничивал только начало. Передача
старого intent-cancel context напрямую в commit ломала бы легальный own retirement,
потому что сам import меняет intent. Common import/pin также имели Background при
отсутствующем App lifetime. Это source findings, не runtime воспроизведение.

### Изменения

- profile_request.go расширяет существующий intentRegistration отрицательным
  private request scope: actual App parent/request context и atomic pointer на
  immutable registration. Нет JSON/SDK/native/forwarding authority, credential
  payload, второго engine или replacement App state. Bound stale/foreign/nil/
  invalid scope не заменяется свежим. Journal callbacks читают context/source
  values, не вызывают App methods и не захватывают App.mu.
- Public-source mutation flight binds registration, захваченную V352 до ожиданий,
  в private context. Common public profile decode, protected/legacy encrypted
  bundle KDF и pin decode получают original scope заранее. Re-entering common
  preparation повторно использует тот же scope. После operationMu wait проверяется
  currentLocked against exact App ID/source до metadata/vault work; source change
  возвращает typed context.Canceled, не сообщение об отсутствующем vault.
- Common profile/pin import больше не заменяют отсутствующий App lifetime на
  Background. Actual request context/deadline сохраняются через SDK child scopes;
  SDK timeout не отменяет parent request. Missing lifetime требует нового реального
  App session, а не синтетической authority.
- retireProfileSessionRegistered переиспользует существующий cutover/Manager Stop.
  До advance проверяется original registration и request scope. После actual
  advanceOperationLocked только этот writer получает exact successor registration
  под тем же App.mu. До завершения native Stop это negative continuity, не receipt
  успешной очистки. После Stop проверяются exact successor/context/source; более
  поздний manual Stop делает старый запрос stale. Старый numeric helper сохранён
  для storage-quarantine callers, внутри сопоставляет original registration.
- Existing profile sourceFence/Store.BeforeCommit и pin checkFresh проверяют
  requestScope.current до и после private source/time/authority checks. Scope
  source меняется только собственным cutover, ручное отключение не adopted. Shared
  SDK/signatures/hash/version/root/target/expiry/vault/transaction/recovery/native
  checks не заменены наличием текущего scope.
- Public enrolled export переносит тот же scope через SDK/operationMu/vault wait,
  credential projection/sealing/final private snapshot gate. Source cancellation
  не маскируется как смена vault. Prefix retention захватывает original ID до
  ожидания; для positive export stale source возвращает Cancel, но подтверждённые
  SDK observations по-прежнему сохраняются прежним private retention протоколом.
  Own retirement при обнаружении unapplied pin policy сохраняет существующий deny
  export outcome; scope transfer не разрешает отправить старые credentials.
- Mandatory authenticated prefix retention отделена от positive target activation.
  Stop после SDK observation не означает, что принятая история не изменилась.
  Ошибки native retirement/storage/expiry всё ещё блокируют activation/resume;
  finishProfileTransition сохраняет свой generation fence для automatic start.

### Evidence и пределы

Manual seven original Go diffs + new private helper; original source capture/
reuse, admission-before-KDF, after-wait guard, actual cutover source transfer,
context parent/child cancellation, prefix retention and journal callback lock
order просмотрены. Go module declares1.26; dependencies/build tags не менялись.
14 pinned reviewed inputs unchanged при final FS receipt. Touched-only gofmt8,
then2, then2, explicit exit0.8Go4docs/12snap11existing1new. No tests/fixtures/build/
vet/types/parser/SDK/generation/browser/native/network/admin/runtime-data work.

Scope transfer/current flag не доказывают original native cleanup, SDK child/
storage retirement, process identity, kernel revocation, cancellation latency,
RSS/interprocess bound или crash/power-loss durability. Flag check и native store
commit не объявлены одним atomic OS действием: Stop может overlap уже принятый
commit, partial writes/history требуют настоящего recovery. KDF/SDK CPU не получают
новой forced cancellation. Runtime race/order/Goexit/native/resource acceptance
и актуальные test fixtures/compile остаются OPEN, не заменены source review.
Другие legacy export/file actions требуют отдельного source/lifetime аудита. Ordering между RPC до входа в Go method остаётся отдельным gate: scope начинается с handler/admission и не заменяет sender/receiver sequence protocol.

Public API/DTO/bindings unchanged, no generated edits. Native/W3b/NETGUARD/SDK/
adaptive/wholeUX/OS/installer/handoff/recovery/release/comparison OPEN.100OPEN56NOT_RUN/
all gatesfalse/TUN unavailable/schema2 unsupported/fullPC00…09/old FR/TH/OFF/COM/
RQ/TR/TH00…08 сохраняются.55% rough/unmeasured only.

**Далее:** remaining legacy export/file action lifetime/source ownership и PC-04
support clipboard original owner; actual native commit/cancellation acceptance
остаётся отдельным разрешённым в будущем объёмом, сейчас testing запрещён.


<a id="pc-v354-transfer-lifetime"></a>

## V2-354 — один запрос через экспорт, QR и файловый диалог

2026-11-09. PC-01/04. **Partial source, unverified**.

**Source findings:** legacy export не сохранял original intent через waits/KDF;
ExportOnboardingQR завершал export до encode, SaveOnboardingFile переходил через
два public вызова ExportOnboarding→SaveProtectedProfileTransfer. Save/load читали
App context отдельно и без original registration. Selected output проверял только
Context.Err, что не давало value-only intent fence до native rename при задержке
cancellation watcher. Нативная гонка не воспроизводилась; это source findings.

### Изменения

- Никакого нового owner/каталога/SDK: existing publicSourceFlight общий для legacy
  export, QR, SaveOnboardingFile, SaveProtectedProfileTransfer, LoadOnboardingFile.
  Один 90s child/actual App parent/original intent на whole macro, one passive
  watcher и pointer-matched original finally; interruption/Goexit/timeout/Cancel
  не заменяют original call. Previous public/pin draft invalidated при новом
  действии, что согласовано с existing UI selection contract V351.
- Public/policy previews по-прежнему получают negative path fence/45s child.
  Local crypto/file workflows используют App/intent без physical route dependency:
  offline file transfer не требует fictitious VPN/network availability. Этот
  helper не предназначен для profile mutations с own-cutover; их separate
  admission/handoff V352/353 сохранён.
- Export/QR/save pure password bounds reused before admission. Private legacy
  export получает тот же context/scope, проверяет exact original request/vault
  после operationMu waits и around credential projection/Seal/private snapshot.
  Прежние role/signature/antirollback/source/expiry/legacy-TUF rejection и native
  vault checks сохранены. Ciphertext/QR не выдаётся при stale source.
- ExportOnboardingQR spans private export→encode→base64 within original flight.
  SaveOnboardingFile spans private export→dialog→selected write, без nested public
  admission, source reset или adoption нового Stop. Independent saved transport
  остаётся encrypted bounded input, а не trusted profile/install authority.
- File dialog/read передаёт original context. readSelectedImportFile и final
  transport publication проверяют negative profile source before/after original
  bounded reader; private policy picker проверяет scope before dialog. Native
  synchronous read/chooser не объявляются forcibly canceled.
- filetxn.WithSelectedOutputGuard добавляет optional negative value-only callback
  в existing selected-output context. Current App caller проверяет только scope
  values, без App/journal mutex, I/O или cleanup. Old/invalid guard не заменяется
  fresh guard; nil/invalid/panicking guard rejects. Context.Err/Done стандартны,
  fake cancellation context не введён. Independent trusted writers с nil selected
  context сохраняют прежний contract, metadata journal не заменён этим guard.
- Shared slot и Windows/portable writer используют guard в existing acquisition,
  chunk/flush-preparation/pre-rename и final-after-Close checks. Actual native
  resource fields, original slot, publication flags, flush/Close/uncertain sticky
  errors и no replacement fallback не менялись. Guard не останавливает cleanup
  после уже состоявшейся publication и не выдаёт authority на rename.
- Published/MayHavePublished facts сохраняются. Save wrapper сохраняет path/error
  и явно говорит о возможном файле даже если source потерян после ordinary write
  return; cancellation/error не интерпретируются как rollback/отсутствие файла.
  Destination повторяет full acceptance; save не продлевает delivery/profile TTL.
- Public signatures/DTO/bindings unchanged. Уточнён outdated onboarding comment:
  bundled keys сами не назначают authority, common role/independent approval gate
  не заменён encrypted container. Standalone readOnboardingTransport(path) helper
  сохраняет независимый read-only contract; App entrypoints не используют его
  Background вместо actual App lifetime.

### Evidence и пределы

Manual eight original Go diffs; all production legacy export/WriteSelected callers,
shared admission/no nested calls, original source/wait/vault/KDF/dialog/result,
guard default/trusted-mode/Close/publication and context semantics просмотрены.
13 pinned reviewed sources unchanged при final FS receipt. Touched-only gofmt8,
then2, then1, explicit exit0.8Go4docs/12snap12existing/no new source files/dependencies.
No tests/fixtures/build/vet/types/parser/SDK/generation/browser/native/admin/network/
runtime data. Wails/QR/native input/output код не выполнялся.

Value guard не делает Stop/rename атомарным OS действием. Stop может overlap уже
принятый native publish; ciphertext file может существовать при error. Hard latency,
forced KDF/SDK/native I/O/chooser cancellation, crash/power-loss/directory durability,
ACL/alias/sharing/native retirement/RSS/interprocess bounds не доказаны. Generic
output callback требует value-only caller contract; arbitrary callbacks не
сертифицированы. Portable path races не объявлены Windows custody proof.
No zeroization claim for original strings/Wails/QR/closure memory. Shared source slot
bounds these five wrappers, not every raw RPC/global KDF operation or JS realm.
Pre-method RPC ordering остаётся своим gate; native tests/fixtures/compile/new EXE/
whole journeys/keyboard/DPI/wholeUX/OS/SDK/installer/handoff/recovery/W3b/NETGUARD/
adaptive/release/comparison OPEN.100OPEN56NOT_RUN/all gatesfalse/TUN unavailable/
schema2 unsupported/fullPC00…09/old FR/TH/OFF/COM/RQ/TR/TH00…08 сохраняются.
55% rough/unmeasured only.

**Далее:** PC-04 support clipboard original owner и finite interrupted-source/
publication outcomes; полная native/OS/wholeUX приёмка остаётся OPEN по запрету tests.


<a id="pc-v355-support-clipboard"></a>

## V2-355 — копирование сводки и секрета при смене страницы

2026-11-09. PC-04. **Partial source, unverified**.

**Наблюдение по исходникам:** SupportLogPreview и SecretCard независимо вызывали
browser clipboard. Local busy терялся при unmount; secretsStore освобождал свою
action admission после reveal, до окончания clipboard write. Stale secret response
возвращался до очистки plaintext field. Успешный Promise назывался фактическим
содержимым буфера, а rejection — доказанным отсутствием копирования.
Гонки не воспроизводились: tests/runtime запрещены владельцем.

### Изменено

- Найдены оба caller и существующий profileActions lifecycle pattern. Новый
  clipboard composable нужен для отдельного общего clipboard domain: профильный
  helper имеет иной purpose/selection contract. Новых UI cards/store/framework нет.
- Shared reactive state содержит только purpose/phase/owner/token. Текст,
  secret ID, callbacks и Promise в него не попадают. Одна original macro operation
  резервируется до reveal; page/root disposal отменяет только интерес к результату.
  Callback ожидается до возврата, original clipboard promise — также в owner finally,
  даже если caller не await-нул writeText. Timer/queue/automatic retry отсутствуют.
- Provider выбирается до выдачи секрета: existing generated Wails ClipboardSetText,
  browser writeText только при отсутствии native runtime. Identity проверяется
  непосредственно перед dispatch без await, повторная запись в одной lease запрещена.
  Failed native call не запускает browser fallback. Bindings не патчились.
- Success означает только соответствующий ответ provider: Wails true/browser void.
  Rejection/false не объявляются доказанным отсутствием записи. Sync throw или
  malformed completion handle после входа сохраняет unconfirmed admission; закрытие
  root не сбрасывает его. Pending и unconfirmed показаны отдельно; aria-busy не
  обозначает неопределённый результат как продолжающуюся операцию.
- Secret macro держит original slot через reveal и запись. Полученное plaintext
  поле ответа очищается до stale-interest return; локальная ссылка отпускается до
  await записи. Старый экран не публикует результат. Другие secret actions не
  блокируются чужой clipboard операцией, кроме существующей store admission.
- Support snapshot Readonly/Object.freeze; original snapshot identity, Symbol
  generation и текущий context fence проверяются перед dispatch/result. LogsView
  инвалидирует фильтр синхронно, callback читает parent revision до prop render.
  Preview из ещё не обновлённых props не создаётся. Новые события не меняют snapshot.
- Просмотр и его закрытие доступны во время копирования. Readonly textarea,
  explicit label, existing focus restoration, wrapping/actions/tokens сохранены.
  Постоянный outcome status показывает finite сообщения без raw exception/секрета.
  Unsupported provider не вызывает reveal; остаётся ручное выделение сводки.

### Доказательство и предел

Manual review шести original source diffs: 2TS/4Vue; четыре owned docs,
10 snapshots/9 existing/1 new,12 reviewed inputs. Source diff — read-only git
--no-index к собственным snapshots; exit1 означает различие, не product PASS.
Только FS/hash/checkpoint bookkeeping; tests/fixtures/build/typecheck/parser/
SDK/generation/browser/clipboard calls/native/network/реальные секреты не запускались.

Это сериализация двух caller в одном JS module, не межпроцессная/межоконная
блокировка буфера. Другие программы, renderer restart/HMR, native clipboard
service ordering, ОС и clipboard history не контролируются этим owner.
Provider promise settlement — контракт API, не native retirement/актуальное
содержимое/успешная вставка. JS/Wails/browser строки не могут быть объявлены
zeroized; очистка одного response field — освобождение ссылки, не стирание памяти.
Нет принудительного cancel и hard deadline; pending не освобождается по timeout.
Защита от malicious replacement runtime/Promise globals не заявляется.
Whole journeys/keyboard/DPI/permissions/native lifecycle/OS/crash/SDK/install/
W3b/NETGUARD/adaptive/release и все требования PC-00…09 остаются открытыми
в применимом объёме.100OPEN56NOT_RUN/all gatesfalse/TUN unavailable/schema2 unsupported.
55% — прежняя приблизительная оценка объёма, не новый замер или готовность выпуска.

**Далее:** finite interrupted-source/publication outcomes в profile/update UI;
полный PC-план и прежние FR/TH/OFF/COM/RQ/TR/TH-00…08 сохраняются.


<a id="pc-v356-action-outcomes"></a>

## V2-356 — отказ до отправки, отмена диалога и возможное сохранение

2026-11-09. PC-04. **Partial source, unverified**.

**Наблюдение:** legacy save/load превращали null/undefined в пустую строку;
build-phase возвращала fake empty success. Pin file preview смешивал null и
undefined; QR mapper допускал пустой результат. После ухода с панели терялось
напоминание о неопределённом результате. Profile apply UI не сопоставлял reply
с optional selected identity/SHA. Поздний policy Cancel мог сообщить, что изменения
не применялись. Update apply возвращал void и при local refusal, и после dispatch.
Это source findings; runtime не воспроизводился по запрету владельца.

### Изменения

- Existing failureMessages содержит finite action vocabulary и private WeakMap
  local refusal. Метку создают только pre-invocation guards API для конкретного
  target action. Строки исключений/Go error/context.Canceled и shape failures
  не анализируются и не получают not-sent. Ранее выполненный этап macro, например
  выбор/чтение файла до crypto preview, этой меткой не отменяется.
- Missing binding/build-phase больше не возвращают пустой успешный QR/save/load/
  update picker. Exact empty string означает только dialog cancellation; Pin file
  принимает exact null. Undefined/нестроковый/слишком длинный/NUL path — unconfirmed.
  Selected path сохраняется без trim/rebuild. Transport shape/budgets reused:
  SNOB1 256KiB/SNOB2 2MiB, PNG data-URL viewing budget 2MiB. Это UI bounds, не
  crypto/PNG decode/trust/native path или публикационная приёмка.
- Existing profileActions owner остаётся одним original slot до actual finally.
  Десять caller сообщают received/cancelled/local-refused/unread/unconfirmed.
  Два bounded metadata records хранят только kind/outcome: last settlement и
  negative reminder, без payload, пути, URL, password, token, DTO или raw error.
  Новое действие не очищает reminder, page/root disposal его также сохраняет.
  «Скрыть напоминание» меняет только display, не unlock/rollback/native gate.
- Existing ProfileTransfer/HintBox reused, новых cards/store/framework нет.
  Сохранение с error описывается как возможный существующий/изменённый файл.
  Wails error не раскрывает Published vs MayHavePublished; эта точность не выдумана.
  Empty dialog не считается отсутствием прежних изменений. Late/mismatched reply
  не принимается экраном; optional selected bundleIdentity/targetSHA сверяются.
- Policy Cancel во время applying сообщает pending apply, не вызывает invalidate
  как доказательство отмены. Close preview/Cancel request не обещают clean rollback;
  оригинальный pending slot сохраняется до actual response. NextTick focus path
  повторно проверяет original view interest.
- Existing update owner сохраняет finite outcome display через unmount/root.
  Late picker не запускает Check, selection/check errors раздельны. Platform
  applyUpdatePackage возвращает not_sent/before_install/next_process_unconfirmed/
  unconfirmed по existing dispatch witness и existing updateInstallEntered receipt.
  Consent consumption, original positive slot, Stop supersession, updateHold и
  modeHold admission не изменены. Before-install — handler phase contract,
  не native installer termination; returned не считается запуском нового процесса.

### Доказательство и предел

Manual review девяти original source diffs:5TS4Vue;4 owned docs,13 snapshots/
13 existing/13 pinned inputs. Read-only git --no-index к своим snapshots;
exit1 означает source difference, не product PASS. Только FS/hash/checkpoint
bookkeeping; tests/fixtures/build/typecheck/parser/SDK/generation/browser/native/
dialog/network/реальные данные не запускались. Generated sources и Go API сохранены.

Local refusal — frontend observation до target invocation, не подтверждённый отказ
сервера/нативного installer. Generic RPC rejection остаётся uncertain; полноценные
native finite profile/file receipts и whole IPC ordering остаются отдельной работой.
Shared reminders — bounded display history одного JS module; hide/renderer restart
не являются доказательством отмены, no-file или полного current-state reconciliation.
Свежий read demand не доказывает совпадение с original operation. Legacy optional
identity поля не превращены в обязательный authenticated commit receipt.
No zeroization/native retirement/durability/resource/wholeUX/OS/SDK/installer/release
claim.100OPEN56NOT_RUN/all gatesfalse/TUN unavailable/schema2 unsupported.
55% остаётся прежней приблизительной оценкой без нового замера.

**Далее:** PC-01 W4c original-helper/WFP admission по текущему PC-PLAN; native
finite receipts/refreshed-state reconciliation/wholeUX и весь PC-00…09/старый
FR/TH/OFF/COM/RQ/TR/TH-00…08 сохраняются открытыми в применимом объёме.


<a id="pc-v362-page-interest"></a>

## V2-362 — пассивные наблюдения видимой страницы

2026-11-10, **source/unverified**, PC-04/06. V361 — progress: original loopback
owner/два UI consumers сохранены. Полный PC-00…09/прежние девять этапов active;
100 acceptance OPEN/56 checks NOT_RUN, без waiver.

### Найденные потребители и выбранное переиспользование

Existing usePolling уже имеет pauseWhenHidden, scope и original pending promise;
createPoller использует opaque generation и не заменяет не вернувшийся request.
Эти механизмы сохранены. Только существующие visibility predicate/listener
извлечены в polling.ts для общего infoFacts. Нового cache/store/таймера поверх
текущих pollers нет. App.vue использует v-if/v-else-if; разделы размонтируются.

| View | Пассивные readers с pauseWhenHidden | Запрошенный shared source |
|---|---|---|
| Dashboard | NETGUARD | network facts |
| Channels | channel health | — |
| AutoProtect | health, independence, incidents | failover |
| Network | NETGUARD, split, adaptive read | network facts |
| NETGUARD | status | — |

Всего9 opt-in readers пяти страниц. Scoped core metrics и adapter stats уже
имели pauseWhenHidden, их source не менялся. Settings profile/runtime и root
mode reader — one-shot/manual owners; общий lifecycle/Stop/state-event owner
не приостанавливается. Go engine/protection/watchdog/network services не менялись.

### Shared infoFacts

Раньше любой потребитель запускал обе ветки, даже если использовал только одну.
Optional mask facts/failover захватывается один раз в setup; no-arg API совместим
и запрашивает оба источника. Каждый имеет свой count, один original poller и
отдельные loading/error/receipt. Добавление потребителя другого источника не
перезапускает уже используемый источник. Один shared visibility listener живёт
от первого до последнего nonempty consumer; removal idempotent.

На текущем hidden/последнем source release прекращаются timer/publication и
очищаются value/receipt, без synthetic0. Existing pending promise сохраняется;
после возобновления свежий read ждёт возврата старого. Loading обозначает и это
ожидание. Direct refresh callbacks проверяют свой captured source/released flag
и текущий visible interest; stale consumer не обновляет источник другой страницы.
Load/publication/error повторяют source-interest check. Receipt записывается до
нового value, чтобы synchronous observers не увидели новую запись со старым
renderer timestamp. Это не measurement time или validity deadline.

### Ручные команды и visibility

hasInterest из usePolling означает только UI scheduling/publication. В Network
запущенная вручную проверка сайтов остаётся у своего original command owner.
Если она завершилась, когда read paused или ждёт fresh continuation, null report
не объявляется failed refresh. При visible завершённом read прежнее сообщение
unavailable сохраняется. Visibility не повторяет manual command автоматически,
не снимает backend slot и не доказывает native cancellation/retirement.

Для frontend-postgres переиспользованы текущие composables/API; PostgreSQL нет.
Modern-web-guidance: offline detect-initial-visibility-state guide отличает
текущую visibility от истории загрузки. Здесь нужен только current interest;
initial background/foreground duration не измеряются. Performance entries,
новые browser APIs/polyfills/зависимости не добавлены; non-browser fallback
сохранён. Это не подтверждение поведения Wails при минимизации/сне/OS focus.

### Результат и ограничения

2 TS/5 Vue/4 docs,11 existing original snapshots. Перед уточнением source masks
сохранён refine checkpoint в том же наборе файлов. Просмотрены7 original source
diffs; incidental EOF у AutoProtect возвращён к исходному, CSS/layout не менялись.
17 named source inputs сохранены по SHA256. createPoller implementation, current
scoped metrics/stats/Settings/root backendState, bindings и Go source unchanged.

Tests/fixtures/build/vet/typecheck/parser/generation/browser/SDK/native/admin/
network не добавлялись/не запускались. Только source/docs/own metadata и чтение
локального guide. Browser/keyboard/visibility/async/runtime/native/resource/RSS
приёмка не выполнена; ускорение/экономия не измерены. Backend GetNetworkFacts
может продолжать уже начатую работу: UI interest не доказывает её завершение.
W4c gatefalse/TUN unavailable/schema2 unsupported/fullPC00…09 остаются.

### Следующий backend-срез

Read-only source audit windows/infobindings.go показал GetNetStats→snapshotFacts
→collectFacts, включая внешние egress HTTP. Счётчики адаптера не должны запускать
этот дополнительный запрос. Также нужно разобрать cache ownership: deferred
Unlock остаётся зарегистрированным вокруг участка collectFacts с отпущенным
mutex; abnormal unwind требует явного исхода. HTTP использует Background и
непроверенные body/transport retirement. Сначала original App/path/collect
owner и существующие helper/probe/budget API, затем source implementation;
без direct fallback, ложного cached zero и без запуска сети.

<a id="pc-v385-navigation"></a>

## V2-385 — section navigation, manual tabs and responsive shell

2026-11-10, **partial source/unverified**, PC-04.

### Source findings and reuse

- At max-width1180 the old rail hid labels even at the required940px window.
  The rail now keeps labels until900px and shares a shell width variable with
  TopBar. Bottom navigation uses intrinsic height and start-aligned overflow;
  selected items are revealed without moving keyboard focus.
- Arrow/Home/End used to activate the tab and unmount/mount views immediately.
  ContextTabs now maintains local roving focus only; native button Enter/Space
  and click select the page. Modifier/default-prevented events are preserved.
  Leaving the widget restores its selected tab as the entry point.
- Every tab previously controlled an inactive panel ID absent from the DOM.
  App derives shells from the existing section registry and keeps inactive shells
  hidden/empty. Only the selected view mounts; no KeepAlive, inactive poller or
  action is introduced. Single-page sections still have no tablist.
- Main is named by its real section heading. Page changes reset main scroll;
  a revision-bound nextTick restores focus only when the old page control was
  removed and focus fell to body. Rail/tab/dialog/newer focus is preserved.
- Inset focus outlines and forced-colors selection survive decorative clipping.
  Tabs size from their labels; secondary captions use existing muted/text tokens.
  Header can wrap its status and keeps verification-scope copy at narrow widths.
- Search names reflect its all-page results. Existing search/notification dialog
  state is exposed by optional TopBar props; backend/API/Wails contracts and
  dialog event ownership remain unchanged. Removed unused rail subitem/count/
  divider/footer styles while preserving section IDs and remembered pages.

Reused Vue3 Composition API, existing registry/NavRail/ContextTabs/SectionHeader/
TopBar, tokens, page teardown and reduced-motion CSS. No router/store/framework,
new visual cards, CSS parser, dependency or generated binding change.
Local modern-web-guidance: spatial-navigation (roving focus and nearest scroll),
css-layout (intrinsic flex sizing/min-width0/wrapping), accessibility (explicit
activation, names and visible focus). Only applicable existing browser features
were used; guide compatibility claims are not product-runtime evidence.

### Source evidence and remaining acceptance

5Vue4docs/9 existing snapshots; manual5 original diffs;12named0cached.
No source formatter, compiler, parser or product code invoked. Receipts:
.artifacts/memory/bootstrap-v385-navigation-.
After authorization, cover manual tab activation, active-only view teardown,
section memory/search/quick transitions, focus recovery and real narrow/DPI
rendering. Existing tests were not changed or run; old keyboard expectations do
not establish the new behavior. DialogFrame remains the existing custom modal;
its background inertness/focus lifetime is the next source review, not accepted.
No tests/fixtures/build/vet/types/parser/SDK/browser/WebView2/native/network/generation. Manual source/diff review does not prove Vue compilation, keyboard/screen-reader behavior, rendered layout, contrast, 940x600/1180x780/1366x768 or DPI125/150/200 usability. Backend/Wails bindings unchanged. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release remain OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v386-dialog-search"></a>

## V2-386 — native modal lifecycle and page-search combobox

2026-11-10, **partial source/unverified**, PC-04.

### Concrete source changes

1. DialogFrame retained its label/slot/close interface for search, notifications
   and proxy recovery. Replace the custom focus trap and delayed mounted/unmounted
   focus callbacks with connected native showModal/close. Native top-layer modal
   behavior owns background inertness and focus; no global aria-hidden/inert
   mutation, duplicate trap, external overlay stack or dependency was introduced.
2. Native cancel is prevented and forwarded to the existing owner as close.
   Backdrop dismissal requires matching primary pointer-down/up outside the frame
   and the following outside click; drag and keyboard-synthetic click do not
   request dismissal. No closedby auto-dismiss bypasses the owner. Unmount first
   marks the original component inactive, then closes its still-connected dialog;
   the queued native close event cannot request closure from a newer component.
3. Missing/failed showModal produces a finite existing error toast and requests
   owner close. A confirmation dialog is not silently shown as a nonmodal panel.
   No exception payload is logged. Proxy recovery still owns actual cancellation,
   pending native operations and retained transactions; the Frame grants no
   permission to cancel native work or claim restoration success.
4. App search/notification overlays share one v-if/else-if branch, so replacement
   unmounts the previous component before mounting the new one. CtrlK respects
   default prevention, composition, repeat/modifiers and unrelated modal scope.
   Search navigation uses its own revision/active-page/DOM/overlay checks before
   moving focus to the chosen page after native close, including same-page picks.
5. Search derives section labels from the original registry and matches section,
   page and caption. Combobox focus stays on its input; option buttons use -1
   tabindex, selected ID is valid or absent, empty status is outside the listbox.
   Selected row is revealed by nearest scroll in the bounded result pane. Enter
   ignores IME/repeat/modifiers; Up/Down preserve defaults when no result exists.
   Text wraps, forced-colors selection is outlined, and keyboard help is named.

Local modern-web-guidance accessibility/css-layout and platform-controls-dismiss-
dialog/light-dismiss-a-dialog informed this change. Limited closedby/invoker
features are not required. Runtime/native compatibility remains unverified.

### Evidence and remaining acceptance

3Vue4docs/7 existing snapshots; manual3 original/final source diffs;
14named0cached. No source formatter/compiler/parser/product code invoked.
Initial diff retained; final diff adds template indentation/forced-colors polish.
Receipts: .artifacts/memory/bootstrap-v386-dialogs-.
After authorization, cover real native modal focus/background, Escape/click/drag,
replacement and unmount during pending recovery, missing showModal, IME, search
empty/long results, same/new-page focus and small/DPI windows. Existing tests were
not changed or run; old custom-dialog test assumptions are not current evidence.
No tests/fixtures/build/vet/types/parser/SDK/browser/WebView2/native/network/generation. Source/diff review does not prove Vue compilation, native dialog/Tab/Escape/backdrop/IME/screen-reader behavior, focus restoration, rendered geometry or DPI usability. Existing proxy-recovery ownership/cancel logic and backend/Wails bindings unchanged. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release remain OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v387-action-focus"></a>

## V2-387 — Dashboard/Channels pending target and action focus

2026-11-10, **partial source/unverified**, PC-04.

Channels already contained Stop and commandMessage. Their existence is not a new
implementation here. Source gaps: every row displayed pending during one Switch;
native-disabled action controls could lose keyboard reachability; refreshed
registry could remove the focused row; Dashboard had a second h1 inside the
section h1 and its secondary Disconnect button disappears as Stop starts.

- PositiveRequest is a private discriminated union; only Switch owns target.
  The read-only pendingSwitchTarget checks original token/currentAction, active
  root/state and absence of pending Stop. Root retirement changes currentAction
  and stateLoaded; Stop changes stopRequest/currentAction. Hiding this label never
  clears the original callback/request hold or admits another positive command.
  It is local request metadata, not a backend selector/applied/protected receipt.
- Only the target row shows pending/aria-busy; the current lifecycle ID remains
  separately labelled. Stop/root revocation removes the row claim. Command text
  is a plain live status row, without another visual card or success inference.
- Primary/Disconnect/Stop/Refresh/Switch actions use aria-disabled with actual
  existing guards in their handlers and the shared state methods. No disabled
  attribute is simply removed without a dispatch guard. Non-target rows leave
  the sequential Tab order while busy. Enter-repeat prevention blocks held Enter
  from reactivating a button after an async result changes its action/state.
- Dashboard secondary Disconnect focuses its stable primary before starting
  the existing Stop method. Primary remains mounted and focusable during pending
  outcomes. The large connection heading is h2 under the section's single h1.
- Channels before/after-update hooks capture only its own focused descendant.
  A removed control can recover to the persistent page heading only when focus
  fell to body and no native modal is open. Retained/newer/outside focus is not
  moved; no asynchronous waiter, global listener or retained inactive page added.

Reused backendState/useLifecycle, PageHeading, existing guard/receipt logic and
modern-web-guidance accessibility rules for focusable unavailable controls and
status feedback. No backend DTO, Wails generation, permission, polling owner,
request completion, readiness or applied-channel semantics changed.

3source(1TS2Vue)4docs/7 existing snapshots/manual3 original diffs/13named0cached;
no formatter/compiler/parser/product execution. Own receipts:
.artifacts/memory/bootstrap-v387-action-focus-.
After authorization, cover late Switch after Stop/root retirement, target hidden
by filters, row refresh/removal, repeated Enter across phase changes, no dispatch
from aria-disabled controls, secondary Disconnect focus and real small/DPI UI.
Existing tests were neither changed nor run; native focus assumptions remain open.
No tests/fixtures/build/vet/types/parser/SDK/browser/WebView2/native/network/generation. Manual source/diff review does not prove TypeScript/Vue compilation, keyboard/Enter-repeat/focus/live-region behavior, real Start/Switch/Stop ordering, source retirement, rendered geometry or DPI usability. Existing request/receipt/reconciliation/canStop/backend/Wails guards unchanged. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release remain OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v388-source-controls"></a>

## V2-388 — independent source refresh and shared guarded action control

2026-11-10, **partial source/unverified**, PC-04.

### Reuse and source changes

No existing Button/Action presentation component or story was found. Existing
page-local guards remain; one small ActionButton belongs in components rather
than duplicating native event/focus behavior for each new source-refresh action.

- ActionButton uses a native button and existing btn/tokens/slots/attributes.
  Its declared click event is emitted only after checking disabled/default-
  prevented state; unavailable activation is prevented and stopped. aria-disabled
  preserves the mounted control's focus and Enter-repeat is suppressed. Expose
  only its single native focus operation; no API caller, timer or request owner.
- busy is presentation only. It does not automatically disable a negative
  action such as stop-next while a batch's original call is outstanding. Native
  button semantics, original parent guards and actual module guards all remain.
- Network has separate facts, NetGuard, split-list and site-report metadata
  refresh controls. They reuse the existing source-specific loading refs and
  refresh functions; owner/current-pending gates remain. A slow aggregate source
  no longer removes these independent renderer actions. No new poller, timer,
  retry queue, native cancellation or overlapping same-source read was added.
- Refreshing site-report metadata is labelled separately from the existing
  explicit site check. Old/unknown measurement freshness and scope copy remain.
- Settings profile/runtime refresh and Stop use the shared control with their
  existing loading/canStop guards. Production TestsView/TestCard share it for
  manual actions; batch stop-next remains usable before its request is made,
  while current-call completion is still awaited by the existing diagnostics
  owner. Explicit view guards preserve disabled semantics under custom events.
- Diagnostic presentation/results/context checks and backend contracts are
  unchanged. TestCard hover selector follows aria-disabled; obsolete native-
  disabled styling removed. Network section actions wrap within existing rows,
  without new cards or a new visual framework.

These are production UI files, not test files. No fixture, test runner, automated
test, diagnostic call, site check or metadata RPC was added/run by this work.
Local modern-web-guidance accessibility/css-layout informed focusable unavailable
controls, native semantics and wrapping; runtime proof is still absent.

### Evidence and remaining acceptance

5Vue4docs/9 snapshots (8existing/1new); manual4 original diffs + new component;
14named0cached. Initial/intermediate source diffs retained; latest source diff
also records the diagnostic comment and hover-selector correction. No formatter,
compiler, parser or product code invoked. Receipts:
.artifacts/memory/bootstrap-v388-source-controls-.
After authorization, cover disabled mouse/keyboard/custom-event dispatch,
Enter-repeat, busy-but-available stop-next, slot/attribute/focus behavior, one
source stalled while another refreshes, metadata vs measurement and narrow/DPI
layouts. Existing unit/browser assumptions require review; no current PASS.
No tests/fixtures/build/vet/types/parser/SDK/browser/WebView2/native/network/generation. Production diagnostic UI source edits do not execute diagnostics. Manual source/diff review does not prove Vue typing/event fallthrough, focus/Enter behavior, rendered layout/DPI, RPC latency/retirement, independent native collectors or runtime resource bounds. Existing diagnostic context/custody, poller and backend/Wails owners remain unchanged. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v389-action-reuse"></a>

## V2-389 — shared action component and page-owned removed focus

2026-11-10, **partial source/unverified**, PC-04.

- Reuse existing ActionButton on Dashboard/Channels instead of duplicating
  disabled dispatch, native focus and Enter-repeat behavior. Keep original
  primary/canStop/Switch/refresh guards and row tabindex/target metadata. A busy
  pending action is still distinct from the availability of negative Stop.
- Dashboard's primary ref uses the component's actual exposed focus method;
  secondary Disconnect still focuses that stable primary before invoking Stop.
  Other async actions also use the existing component; navigation is unchanged.
- Extract the existing Channels update-hook logic into removedControlFocus.
  It captures only a focused descendant of its provided root. Automatic recovery
  requires that original control be disconnected and current focus be body.
  The root and queried fallback must remain connected, and the target must be
  within the same root; an open native dialog prevents the move. Unmount clears
  the captured reference. No timer, MutationObserver, global focus listener,
  deferred callback or inactive-page retention was introduced.
- Reuse the helper on Dashboard, Channels and Settings. Conditional slot content
  can update without the containing page's update hooks. Guarded Stop in Channels/
  Settings therefore explicitly invokes the helper before dispatch, moving an
  owned page focus to its persistent heading. This does not rely on detecting a
  later slot removal. Existing Stop and native cleanup owners remain unchanged.
- Global btn normal/primary/success hover selectors exclude aria-disabled, and
  disabled/active styling recognizes it. This aligns the actual shared control
  state with base styles while keeping native disabled controls supported.

The new helper is a compatible extraction with three consumers; no existing
focus composable was found. Reuse actual root refs and stable local IDs instead
of introducing a router, cross-page focus manager or global observer. Local
modern-web-guidance accessibility/css-layout remains the applicable guidance.

5source(1TS3Vue1CSS)4docs/9 snapshots (8existing/1new); manual4 original diffs plus
new helper;14named0cached. No formatter/compiler/parser/product code invoked.
Receipts: .artifacts/memory/bootstrap-v389-action-reuse-.
After authorization, cover component-ref focus exposure, disabled/Enter behavior,
pending Stop and slot removal, removed-row/body/newer-focus/modal cases, unmount
and small/DPI rendering. No current runtime/visual acceptance is claimed.
No tests/fixtures/build/vet/types/parser/SDK/browser/WebView2/native/network/generation or diagnostic execution. Manual source/diff review does not prove Vue component-ref/slot update behavior, actual focus restoration/keyboard dispatch, CSS rendering/contrast/DPI, native cleanup or protected egress. Existing request, diagnostic, poller and backend/Wails owners remain unchanged. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.
