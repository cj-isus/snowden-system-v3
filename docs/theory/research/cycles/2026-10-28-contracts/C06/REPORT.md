# R28-C06 — честное восстановление дома и на работе

Статус: **READY_FOR_REVIEW**. Автор C06 / теория (6). Source-only research, не реализация.
Pins и глубина чтения — в [SOURCES](SOURCES.md); окончательный объём и ограничения — в [HANDOFF](HANDOFF.md).

## 1. Ограниченный ответ

Удобное восстановление требует четырёх отдельных ответов: **что наблюдаем;
какое действие разрешено; изменилось ли нужное состояние; завершилась ли задача**.
Единственный зелёный статус «всё исправлено» эти ответы не заменяет.

Новая связанная дельта к baseline:

1. **R28-C06-A:** recovery contract с границами акторов, наблюдений и действий.
   Android уже содержит полезные guards; Windows NetGuard местами повышает факт
   о планировщике/порту до обещания защиты или исправления.
2. **R28-C06-B:** R06-013-A нуждается в исправлении maturity/denominator. Ранний
   terminal outcome нельзя использовать как условие допуска в знаменатель
   fixed-horizon recovery rate. Потерянные записи не равны отсутствующим попыткам.
3. **R28-C06-C:** R06-012-D сохраняет силу как **условный** UID/app-rule gap.
   Публичный mapper появился в API35; существующие native admission blockers
   не позволяют объявлять source-контрпример уже работающим обходом BLOCK.

Рекомендуемые verdict — предложения руководителю. Научного ACCEPTED, product
PASS, device/native/live validation этот пакет не присваивает.

Проблемы maturity/early Q-F/snapshot scope уже отмечены в
[рецензии 27 октября](../../../reviews/NINE-2026-10-27-R06-013-A.md).
Здесь они независимо перепроверены на baseline и доведены до event/census/time
contract; обнаружение прежнего замечания не объявляется новой научной теоремой.
Дополнение — конкретные command-admission/retention seams, собственная policy
при чужом VPN и Windows caller, повышающий слабые наблюдения до обещаний.

## 2. R28-C06-A — контракт наблюдения и действия

### 2.1 Четыре области, которые нельзя склеивать

| Область | Пример наблюдения | Допустимый вывод | Недостающий вывод |
|---|---|---|---|
| Authority/capability | локальный consent request, revoked, admission blocker | наш клиент сейчас может/не может продолжить конкретную операцию либо этого не знаем | кто управляет всей корпоративной policy; защищён ли пользовательский трафик |
| Lifecycle/resources | Stop принят, worker retiring, release unknown | команда принята; наблюдение worker имеет возраст; старый ресурс ещё не подтверждён освобождённым | процесс точно умер; все дескрипторы закрыты; successor уже безопасен |
| Собственная policy/path | датированный scope/gen receipt, если платформой реализован и принят | ровно заявленное свойство указанного scope во времени | вся сеть/все приложения/весь интервал задачи без утечек |
| Полезная задача | локальная отметка о завершении выбранной задачи | пользователь сообщил исход этой задачи | банковская операция окончательно проведена; печать физически завершилась; причина успеха — VPN |

Android `ConnectionState` не содержит принятого protected Ready. Комментарий
`RuntimeSourceProvider` прямо сохраняет необходимость принятого renderer и
связанного native/capture/policy contract. `OsProtection` наблюдает возможности
этого приложения; `protect()` для собственного незапущенного socket не доказывает
путь обычного приложения. `prepared*ContextMatches` — соответствие подготовленного
контекста, не applied policy. `scratchCleanup` — локальная очистка файлов, не
подтверждение native release. Нельзя синтезировать недостающий receipt из этих полей.

### 2.2 Малая матрица состояний и действий

Строки — контракт будущей реализации, а не новые DTO или доказательство полевого UX.
При нескольких причинах показываются установленный blocker и безопасное действие;
неопределённая причина остаётся неопределённой.

| Ситуация | Что действительно известно / что unknown | Полезное действие | Граница результата |
|---|---|---|---|
| Дом → рабочая сеть, наблюдаем смену underlay | прежний network context устарел; proxy/DNS/captive cause ещё не установлена | дождаться нового наблюдения и существующего ограниченного recovery; собственный Stop доступен по admission | reconnect/проверка не означает завершение рабочей задачи |
| Есть явный сигнал входа в сеть | ОС сообщила prerequisite для конкретной сети; один timeout этого не доказывает | открыть доступные системные настройки/системный вход по явному действию; при невозможности — инструкция и выбор другой сети | открытие страницы и даже исчезновение captive signal не доказывают наш protected egress |
| Proxy/DNS prerequisite не установлен | известен отказ конкретного шага, причина и фактический resolver/proxy path неизвестны | локальная диагностика с typed reason; настройка только своей явно выбранной конфигурации | не обещать «снимем корпоративный DNS/PAC/NRPT» и не менять их автоматически |
| Собственная VPN-authority revoked / нужен consent | этот клиент потерял/не имеет нужного права; foreign policy полностью не видна | foreground явный Connect либо открыть VPN settings; при внешнем управлении — объяснить ограничение | settings dispatch / RESULT_OK не резервируют authority и не подтверждают tunnel/task |
| Foreign VPN/MDM возможен, но owner/policy не установлен | own capability отсутствует/неизвестна; причина может быть иной | «Не удалось подтвердить право на VPN. Проверьте системные настройки»; только явный выбор пользователя | не писать «корпоративный VPN мешает», не отключать чужой owner и не обещать одновременную работу |
| Истекла authority источника/подготовки либо установлен trust blocker | конкретный typed admission blocker; он не автоматически означает истечение TLS-сертификата | собственный путь повторной проверки/подготовки либо обновления профиля по причине | сетевой retry/повторный consent не исправляют иной trust blocker |
| Attribution выбранного приложения неизвестна/изменилась | app-specific candidate не принят; общий capture и app-rule — разные свойства | оставить эту policy непринятой; объяснить необходимость повторной проверки. Другой режим — только явно, после его собственной приёмки | нельзя объявлять весь VPN сломанным или молча превращать BLOCK в default/direct |
| Старый worker/resource pending | Stop/replacement запрошен; release не подтверждён | «Остановка запрошена. Ждём подтверждения»; диагностика/системный экран при недоступном контроллере | повторная кнопка не создаёт право на successor и не заменяет release receipt |
| Уведомление/плитка устарели или публикация не подтверждена | поверхность могла показывать старый текст; backend неизвестен до нового чтения | открыть приложение; действие повторно проверяется текущим владельцем | `notify()`/`updateTile()` — запросы публикации, не display/delivery/protection receipt |
| Явный Stop принят | сохранено намерение прекратить работу нашего клиента | ожидать соответствующего lifecycle observation; не возобновлять таймером вопреки Stop | остановка нашего клиента не доказывает ни отсутствие другой защиты, ни завершение задачи |

**Порядок действий:** навигация → новое локальное наблюдение → отдельное решение
платформенного владельца → отдельная отметка задачи. Навигация не запускает скрытый
replay Connect/consent/Stop после возврата. Разрешённость кнопки в UI не является
capability token: исполнитель обязан повторно проверить актуальное намерение,
контекст и admission на своей границе.

| Actor/caller | Собственная операция / observation scope | За пределами его доказательства |
|---|---|---|
| Android UI / Tile | projection текущего `ConnectionState`, foreground settings/navigation, fresh explicit command | VPN authority, native readiness, факт прочтения текста человеком |
| Android parent repository | command admission, desired intent/version, связь с валидированным worker observation | действие ОС до receipt, глобальная foreign-owner policy, бизнес-результат |
| Android worker / OS reader | typed lifecycle, собственные socket/always-on/lockdown samples | атомарный снимок всех OS фактов, обычный app egress; отмена sample не равна OS denial |
| Android системный экран / policy owner | решение о доступном consent/settings; действующая authority может измениться | наш UI не управляет MDM и не резервирует последующий establish |
| Windows App / protection owner | own operationID/intent/StopReason и локальная policy/lifecycle | параметры всех корпоративных providers и пользовательская задача |
| Windows NetGuard caller | Scheduler COM, HKCU proxy-поля, один loopback TCP connect, локальный журнал | подлинность listener, владелец proxy, WFP protection и фактический успех ремонта |
| Пользователь / task producer | согласованная локальная отметка goal/time basis | по умолчанию нет внешнего commit/физического подтверждения |

На Android `VpnService.prepare()` меняет подготовку VPN и не годится как фоновый
read-only health check; разрешение может быть отозвано другим переходом. Системный
always-on/MDM не равен доступному нам обычному consent. Это согласуется с
[официальным VpnService API](https://developer.android.com/reference/android/net/VpnService).
Для captive portal не предлагается произвольный raw URL или скрытый API: текущий
`MainActivity.openSettings` реализует VPN/notification/application pages; отдельный
допустимый portal/Wi-Fi caller в этом срезе не установлен.

### 2.3 Сильные существующие guards Android — сохранить

- `ConnectionPresentation.observationTime`: использует старшую по возрасту из
  producer/receipt дат; UI refresh не омолаживает факты. Порог 5000ms — политика
  отображения, **не** доказанный предел freshness authority/path.
- `TilePresentation` — OPEN_APP/STOP, всегда не protected Ready.
  `SnowdenTileService.onClick` сверяет последнее запрошенное и текущее действие;
  после unlock не воспроизводит ранее захваченный Stop. Навигационная continuation
  ограничена, одноразова и отбрасывается при потере lifecycle/актуальности.
- `MainActivity.openSettings`: foreground/focus/lifecycle guard и обработка
  недоступного Activity; true означает запрос навигации. Документация
  [Settings.ACTION_VPN_SETTINGS](https://developer.android.com/reference/android/provider/Settings#ACTION_VPN_SETTINGS)
  также требует учитывать отсутствие подходящего Activity.
- `VpnNotification`: private/public redacted варианты, immutable PendingIntent,
  стабильный channel ID; отказ POST_NOTIFICATIONS не превращён в VPN failure.
  Успешный вызов публикации не превращён в доказательство показа пользователю.
  [Официальное описание notification permission](https://developer.android.com/develop/ui/compose/notifications/notification-permission)
  различает denied drawer notifications и notices foreground service в Task Manager;
  отсутствие уведомления в шторке само по себе не доказывает остановку service.
- `VpnSessionRepository.disconnect`: Boolean — admission/already-retained Stop;
  повторный Stop не заводит новую epoch. `connect` пишет journal только **после**
  ряда ранних отказов — полезный command log не является census всех попыток.

Эти guards опровергают предположение «устаревшая плитка обязательно повторит
опасный Stop» в проверенном caller. Они не доказывают поведение OEM UI, фактический
показ уведомлений, a11y или устройство. Новую конкурирующую UI-owner систему
создавать не требуется.

### 2.4 Windows: конкретные места повышения фактов до обещаний

**Источник:** `NetGuardView.vue` hero, `netguard.go`,
`backend/core/proxyheal_windows.go`; hashes — input manifest.

1. `TASK_STATE_READY/RUNNING` становится «Защита активна» и «DNS-сбои устраняются
   автоматически». READY означает готовность плановой задачи без запущенного
   экземпляра; RUNNING означает наличие экземпляра. Это не outcome ремонта.
   [Microsoft TASK_STATE](https://learn.microsoft.com/en-us/windows/win32/api/taskschd/ne-taskschd-task_state)
   не предоставляет более сильного свойства.
2. В ветке `no-access` текст утверждает «SYSTEM-задача продолжает работать».
   Отказ доступа подтверждает ограниченность чтения, а не текущее выполнение.
3. При stale proxy герой обещает устранение в течение двух минут/мгновенно при
   старте, хотя текущие права, расписание, успешный repair и результат задачи
   этим наблюдением не установлены.
4. `LoopbackAlive` — один TCP connect с timeout300ms. Успех не аутентифицирует
   Snowden/proxy/tunnel; ошибка не отличает отсутствие listener от иного отказа.
   UI-строка «локальный порт доступен» уже точнее, чем внутреннее `vpnActive`.
5. `HealStaleSystemProxy` проверяет формат и порт1080, но это не происхождение
   настройки. Другой процесс может оставить такой же адрес. Между read и
   `ProxyEnable=0` нет fence от внешней записи; нет и принятого owner receipt.
   Сохранение PAC-значения не доказывает неизменность эффективного proxy path.

**Ручной source-контрпример, не воспроизведение:** другой клиент выставил такой
же loopback1080 → connect не удался → текущие predicates пропускают очистку.
Альтернатива: после чтения ProxyServer внешняя сторона меняет конфигурацию →
очистка всё ещё пишет ProxyEnable=0. По одному формату нельзя доказать «наш» owner.

**Сильный opposing guard:** функция отклоняет иной порт, внешний/комбинированный
формат и доступный listener. Это существенно сужает действия; нельзя писать, что
она безусловно выключает произвольный корпоративный PAC/VPN. В этом исследовании
не установлены реальные настройки, гонка на устройстве или фактический ущерб.

**Рекомендация:** переиспользовать source facts и poller generation guard;
переписать будущие projections как «Плановые задачи доступны/готовы к запуску»,
«Состояние задачи недоступно», «Локальный порт отвечает», «Ремонт запрошен».
Автоматический heal при unknown ownership **не** переносить в новый recovery
contract без отдельного ownership/concurrency разбора платформой. Даже успешное
отключение собственного stale proxy считается authority/configuration repair,
а не protected/task recovery. Общий frontend `AppState` тоже не даёт package/work
profile/foreign-policy receipt: `coreReady` и probe не заполняют этот пробел.

Сохранить и Windows guard: `StopReason.releasesProtection()` разрешает release
собственной защиты только для explicit Disconnect; failure/quit/restart — другие
причины. `registerStopLocked` меняет intent/operation, отменяет retry и публикует
stopping; это не повод показывать уже завершённый release. Plan-only source review
не подтверждает успешный WFP teardown на текущем компьютере.

### 2.5 Восстановление сети не повторяет бизнес-операцию

Отдельный связанный сценарий: запрос операции мог достичь сервера, но ответ
потерялся при handover. Состояние «timeout» не доказывает, что операция не
выполнена. VPN не знает идемпотентность банковской/рабочей операции и не должен
выдавать сетевой reconnect за разрешение автоматически повторить её.
В HTTP это различение непосредственно задано
[RFC9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2):
автоматический retry non-idempotent запроса требует дополнительных оснований;
для proxy установлен более строгий запрет. Это source/spec перенос, **не**
утверждение, что текущий Snowden уже повторяет HTTP POST или расшифровывает TLS.

Пользовательский текст при неизвестном результате: «Связь восстановлена/проверяется.
Результат операции неизвестен. Проверьте её состояние в приложении» — только если
первая часть подтверждается собственным receipt. Нельзя говорить «платёж не прошёл,
повторите». Отсутствие бизнес-ответа остаётся unknown в task metric, даже если
последующая собственная VPN-проверка успешна. Application retry/idempotency key
остаются у соответствующего приложения, не у VPN recovery controller.

### 2.6 Краткость объяснения и доступность

Поверхности приложения, плитки и уведомления используют одну typed причину,
но разный объём текста: **состояние → граница известного → одно доступное действие**.
При неизвестном controller не показывать неработоспособную кнопку Stop как
принятую возможность; открыть foreground диагностику/системный экран. Декоративный
щит, цвет и бесконечный spinner не заменяют текст «неизвестно» или «ждём release».
Истечение provisional UI freshness меняет сообщение, а не authority/политику.

Из [W3C Understanding SC4.1.3](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html)
берётся принцип доступного объявления status changes без принудительного focus.
Для Windows markup нужен соответствующий semantic status; для Android —
платформенная семантика, не копирование ARIA. Не объявлять каждый polling tick
как новую ошибку; важный переход и старение факта должны различаться. Конкретные
тайминги, чтение screen reader и понятность copy требуют будущей пользовательской/
device проверки: **NOT_RUN**, compliance этому пакету не присваивается.

## 3. R28-C06-C — проверка R06-012-D

### 3.1 Что подтверждено текущим caller

Manifest: minSdk29, compileSdk36; native target libbox1.13.19 с отдельно одобряемым
SHA gate. Наличие compileSdk не добавляет APIs старому устройству и не подтверждает
содержимое/допустимость отсутствующего либо непроверенного AAR.

`LibboxPlatform.findConnectionOwner` получает raw socket UID через ConnectivityManager,
возвращает его как `ConnectionOwner.userId`, packages — через видимый caller набор.
В прочитанном пути нет sandbox→client normalization.
`NativePolicyLowering.selector` строит **AND inbound + Uid(a) + Package(p)**.
Следовательно, capture sandbox UID не устанавливает совпадение app selector.

Public arbitrary-UID mapper (`isSdkSandboxUid`, `getAppUidForSdkSandboxUid`,
`getSdkSandboxUidForAppUid`) добавлен в **API35** — проверено по
[официальному API34→35 diff](https://developer.android.com/sdk/api_diff/35/changes/android.os.Process).
`isSdkSandbox()` API33 сообщает о текущем процессе; это иной вопрос.
На API29–34 нельзя предлагать кнопку «проверить через mapper» или скрытый offset
как реализованный публичный путь.

| Capability slice | Разрешённый вывод |
|---|---|
| Обычный app UID, совпадающий с signer-bound identity собственного user | Mapper sandbox не нужен для самого равенства; это всё равно source candidate с native/lifecycle blockers |
| API29–32 | SDK sandbox не часть обычного публичного API этого диапазона; не объявлять все обычные app rules несовместимыми лишь из-за отсутствия mapper |
| API33–34, UID семантически не подтверждён как обычный app | current-process `isSdkSandbox()` не решает arbitrary socket attribution; неподдержанная связь остаётся unknown |
| API35+ | Есть публичный API floor для sandbox mapping; он не доказывает installed/signer/profile/native selector parity и не отменяет expiry/churn |
| Чужой managed/work user/profile | текущий own-user identity caller не доказывает его app-rule/capture scope; restricted-profile closure ОС нельзя назвать управлением work profile |

Scenario home/work — пользовательский контекст исследования. Из SSID, package
inventory или timeout он автоматически не выводится и в metrics raw не сохраняется.

### 3.2 Почему это ещё не deployed BLOCK bypass

Текущий lowerer всегда сохраняет blockers SIGNED_CORE_GRAPH_NOT_VERIFIED и
CAPTURE_AND_NATIVE_PARITY_NOT_VERIFIED, а для app rules добавляет
PACKAGE_LIFECYCLE_NOT_FENCED и STRICT_OWNER_CAPABILITY_NOT_VERIFIED.
`AndroidAppIdentity` отвергает legacy sharedUserId и неоднозначную наблюдаемую
группу; signer material проверяется, UID reuse учтён. Нельзя из видимости одного
package автоматически выводить отсутствие этих guards.

`AppIdentityMonitor` инвалидирует epoch при относящихся package events и перечитывает
материал. Его coalescing250ms и core cache200ms из baseline не складываются в
доказанную максимальную stale window: доставка/планирование/весь lifecycle не
ограничены этими двумя числами. Это source design, не измеренный timing/device proof.

**Практическое следствие для UX:** «Не удалось подтвердить правило для выбранного
приложения» точнее, чем «VPN не защищает» или «это приложение не работает».
Полный tunnel mode может быть самостоятельным явно выбранным вариантом, только
если его capture/native/trust contract принят отдельно. Он не доказывает сохранение
исходного app-specific BLOCK и не должен включаться как молчаливое ослабление policy.

## 4. R28-C06-B — исправление cohort/maturity/outcome contract

### 4.1 Контрпример к знаменателю R06-013-A

Baseline §2.1: episode зрелый при terminal outcome **либо** t0+D; pending вне N.
Ручной пример: два episode зарегистрированы t0=0, D=10. A завершился успешно на1,
B ещё pending на5. Такой maturity даёт N=1 и100%. На10 у B подтверждён failure:
N=2 и50%, хотя ни работа алгоритма, ни уже известный A не изменились.

Это outcome-dependent попадание в знаменатель. «Terminal-or-horizon» может быть
полезно для UI завершённых обращений; его доля отвечает вопросу об уже resolved
subset, **не** доле всех зарегистрированных попыток с fixed follow-up D.

Вторая несовместимость: M включает неизвестную зрелость из-за clock, но N объявлен
зрелой cohort. Unknown maturity нельзя одновременно считать доказанной зрелостью.
Третья: journal принятых команд теряет ранние rejected requests, Clear/eviction и
process lifetime. Он не устанавливает количество всех зарегистрированных episode.

### 4.2 Цель измерения и census

Предлагаемый предмет измерения — **локально сообщённое завершение заранее заданной
задачи к D**, отдельно с подтверждённым собственным policy claim в заявленном scope.
Это не объективный учёт банковских/рабочих транзакций и не causal эффект алгоритма.

До исходов фиксируются: единица episode, task class/goal, политика учёта повторов,
cohort membership, D по классу, rule для manual opt-out, границы claim scope,
follow-up R и report cut. Retry/переход в настройки не меняют t0 и не создают
новый episode. Если пользователь явно начал новую независимую задачу, связь и
правило нового enrollment задаются до знания её результата.

Для выбранного goal заранее задаётся, какой completion оценивается: например,
первое сообщённое достижение этого goal, а не последний удобный повтор после
включения VPN. Event time и порядок должны следовать доступному evidence;
первый пришедший callback не обязательно первое completion событие. Когда task
успешен к D, но порядок/own policy нужного completion неизвестны, это X; когда
неизвестна сама timely принадлежность — Z. Нельзя выбирать самый зелёный receipt.
При point-at-completion estimand off→on→completion может дать P и сохранённый
opt-out flag; это всё ещё не continuous-protection estimand. Политика обращения
с таким вмешательством выбирается до исходов.

`EpisodeEnrolled` происходит на согласованной границе регистрации восстановления
**до command admission/outcome**. Если запись не создана, это recording failure,
не успешно невидимая попытка. Coverage UI/enrollment сообщается только при наличии
источника; число невидимых попыток и остальных пользователей неизвестно.

Для закрытой зарегистрированной cohort C количество N и membership не зависят
от terminal events. Если N известно, потеря outcome сохраняет unknown mass.
Если потерян сам census либо неизвестна полнота регистрации: **DENOMINATOR_UNAVAILABLE**,
только наблюдаемые counts и причина; никаких recovery процентов. Отзыв согласия/
privacy Clear соблюдается: тайно сохранять идентификаторы ради знаменателя нельзя.

Enrollment receipt означает регистрацию, не все физические нажатия человека.
Crash между нажатием и durable записью не устраняется одним названием события.
Платформа должна явно различить requested/recorded/recording-unknown. Пока
полнота неизвестна, известен только наблюдаемый subset; его нельзя переименовать
в полный поток recovery intent. Record-before-admission — требование к будущей
границе, а не утверждение об атомарной транзакции UI, journal и VPN worker.

### 4.3 Время, maturity и delayed evidence

Оси независимы: enrollment → maturity → task outcome → policy claim → resource
release. Для cut T каждая запись имеет maturity `known_mature`, `known_pending`
либо `unknown_time_linkage`. `known_mature` определяется только доказанным
отношением t0+D≤T, без условия успеха. Early terminal сохраняется, но не ускоряет
включение в matured fixed-D subset. Счётчики этих групп показываются рядом.

Android elapsedRealtime включает сон и начинается с boot; wall clock может
скакать. Это описано в [SystemClock](https://developer.android.com/reference/android/os/SystemClock).
Сравнение требует общего подтверждённого clock domain/epoch. Process restart в
одном подтверждённом boot не обязательно ломает elapsed clock; reboot без
проверенной связи ломает её. Не выдумывать bridge из похожих wall timestamps.
Windows monotonic domain/cold recovery в этом срезе отдельно не установлен.

Разделять original event/observation time и arrival/recording time. Поздняя
доставка может уточнить прежний episode в новой revision, если original time
и linkage доказывают событие к D. Поздняя самооценка без достоверного времени —
unknown timeliness, а не автоматически своевременный успех и не автоматически
failure. Интервал возможного события, пересекающий D, остаётся неоднозначным.
Grace для ascertainment не сдвигает D. Отсутствие callback на t0+D не означает,
что horizon не наступил: maturity вычисляется на report cut, когда часы сравнимы.

### 4.4 Outcome accounting без ложных нулей

| Наблюдение | Учёт | Ошибка, которую исключаем |
|---|---|---|
| Timely completion + валидный связанный own policy receipt | P для ровно принятого claim scope | P нельзя получать из consent, settings open, порта, coreReady, Tile или собственной socket-protect возможности |
| Timely completion при установленном отсутствии **нашей** требуемой policy | A: completion without required own policy | Manual Stop/opt-out сам по себе не доказывает фактический маршрут и не означает globally unprotected: возможен другой VPN |
| Timely completion, policy claim unknown | X | Не повышать в P и не понижать в A |
| Отмечено partial/not completed **до** D | промежуточное наблюдение | Не превращать в absorbing F/Q: к D задача ещё может завершиться |
| Надёжное подтверждение «goal не достигнут к D» | negative task outcome; partial отдельно | Текущий timeout/неответивший UI не является таким подтверждением |
| Stop/withdrawal и последующий исход неизвестен | C/unknown follow-up | Не считать failure; если timely completion уже известен, Stop не стирает его |
| Follow-up или linkage отсутствует/потерян | M/unknown с причиной; maturity отдельна | Не исключать запись из известного N, не подставлять ноль |
| Completion time неизвестен/интервал пересекает D | timely outcome unknown | Arrival time не заменяет event time |
| Старый resource release поздно подтверждён | только отдельная R-ось | Не исправлять timely task outcome и не приписывать release настройкам/Stop ack |

Manual opt-out — событие вмешательства, потеря follow-up — missing observation;
это разные поля. Методический перенос соответствует различению intercurrent
events/missing data в [ICH E9(R1), A.3.1–A.3.3](https://database.ich.org/sites/default/files/E9-R1_Step4_Guideline_2019_1203.pdf).
Это словарь проектирования измерения, не перенос клинического estimator или
доказательство случайной выборки/независимого censoring.

Point receipt у completion подтверждает только свою временную область. Если
метрика называется «задача полностью выполнялась под защитой», требуется покрытие
всего соответствующего интервала и scope, включая разрешённые исключения. Текущая
UI freshness константа или одна точка такой coverage не создаёт. При неизвестной
полноте/валидности receipt допустим только более узкий label observed evidence.

Более сильный opposing trace: событие completion локализовано в интервале [10,12],
а own-policy наблюдалась только в точке11. Пересечение интервалов и «возраст меньше
5s» не доказывают, что policy действовала в неизвестный фактический момент10.
Для такого положительного claim нужна проверенная coverage **всего** допустимого
completion interval нужным receipt, либо более узкий явно названный observational
endpoint. Freshness не превращает sample в гарантию постоянства свойства.

### 4.5 Минимальный event contract для будущего проектирования

Это **спецификация research record, не production API**:

| Event / record | Минимальный смысл |
|---|---|
| Enrollment | random local episode ID, cohort/config revision, closed task/trigger class, t0 с clock domain, заранее заданный horizon class |
| Action requested / action observed | closed action enum, own/system actor scope, локальная request generation, admission/effect/unknown раздельно; retries остаются в episode |
| Task evidence | complete/partial/not complete/withdrawal, что именно засвидетельствовано, event interval/time basis и arrival; local self-report не называется внешней транзакцией |
| Policy evidence | accepted claim kind, scope class, generation/linkage, original observation window и age/currentness; отсутствующий producer остаётся unavailable |
| Resource observation | retirement/release/retained/unknown и отдельный R horizon; не переносится в task success |
| Report snapshot | fixed cohort/revision, cut с time basis, N/coverage completeness, maturity partition, counts/unknown reasons, event evidence revision |

Достаточность transient producer identity при join и privacy stored record —
разные задачи: отсутствие package/IP в сохранённой статистике не разрешает weak
join «по близкому времени». Нужна проверенная локальная связь с нужной generation/
scope до редукции в closed enum. Без неё X/M, даже если оба события выглядят свежими.

Запрет stored payload: URL/domain/IP/SNI/SSID/ASN, package/workplace inventory,
аккаунты/содержимое задачи/секреты/raw error/native body, profile UUID и стабильный
device ID. Hash домена/аккаунта не делает его безопасным enum. Новый network uploader
не требуется. Local opt-in, bounded retention и явное удаление — самостоятельные
требования, не следствие слова «локально».

### 4.6 Bound contract и способы отчёта

Для **известной полной зарегистрированной** cohort N>0 каждому episode задаётся
интервал [l_e,u_e] для выбранного заранее binary outcome: l=1 только при достаточном
положительном evidence; u=0 только при достаточном evidence невозможности этого
исхода к D. Во всех остальных случаях [0,1]. Тогда

`L = Σl_e / N`, `U = Σu_e / N`.

Это детерминированный диапазон по неизвестным записям, условный на корректность
положительных/отрицательных labels и заданный предмет измерения. Ошибочно связанный
receipt или недостоверная отрицательная отметка нарушают предпосылку, а не просто
расширяют sample confidence interval. Pending/time-unknown нельзя подставлять u=0.
Независимость episodes для этой арифметики не нужна; sharpness диапазона при
межэпизодных ограничениях не заявляется. Это не sample-based статистическая гарантия.

**Конкретная непересекающаяся разметка закрытого N:**

1. Если своевременный completion установлен, classify policy: P — достаточный
   положительный own claim; A — установленное отсутствие требуемой own policy;
   X — policy unknown/неразрешённое противоречие. Stop intent не является A.
2. Иначе F — достаточное подтверждение отсутствия goal **к D**; подтверждённый
   partial-without-goal может быть его подтипом. Early partial/not-yet не F.
3. Иначе Z — неизвестный timely task outcome. Pending, missing, censoring,
   time/linkage unavailable — reasons внутри Z, а не новые знаменатели.
4. Противоречивые task receipts до разрешения переводят запись в Z; не выбирать
   самый удобный outcome. Результат, уже достаточно установленный до последующего
   Stop, не стирается одним событием Stop. Исправление неверного evidence меняет
   outcome в новой report revision, сохраняя N и причину коррекции.

Тогда **N=P+A+X+F+Z** и при указанных assumptions:

- own-claim timely completion: `[P/N, (P+X+Z)/N]`;
- timely task completion любым способом: `[(P+A+X)/N, (P+A+X+Z)/N]`.

Maturity counts и несколько missing-reason flags — отдельные breakdown; их не
прибавляют к N повторно. Если отрицательный label A/F не имеет достаточного
producer evidence, он переходит в X/Z соответственно, расширяя верхнюю границу.
Эта форма покрывает и provisional full-cohort cut, где известный ранний completion
может сосуществовать с time-pending maturity того же episode.

Два разрешённых представления:

1. **Fixed enrolled cohort:** весь N неизменен; запись не исключается по maturity.
   Outcome остаётся неизвестным только при недостаточном evidence; известный
   early timely completion может сузить bound ещё до maturity. Label:
   «предварительный диапазон по зарегистрированной группе», рядом counts/cut.
2. **Known-mature subset:** выбор только t0+D≤T и подтверждённый time linkage;
   показывать N_subset, все excluded pending/clock-unknown и coverage. Это
   условный результат subset; он не исправляет informative reboot/lost follow-up
   и не обобщается на весь enrolled N. Для N_subset=0 rate не показывается.

Исходные P/U/X/Q/F/C/M bounds R06-013-A можно сохранить **после** исправления N,
временных определений и U→absence-of-own-policy semantics. Только при корректной
взаимоисключающей разметке mature cohort они сводятся к тем же sums.
Нельзя обещать «истинный protected rate» сверх soundness receipt/самооценки.
Нет MAR/Kaplan–Meier/imputation/causal сравнения версий по умолчанию; repeated
episodes одного устройства не независимая выборка пользователей.

Для текущего Android отсутствующий принятый task-level protection producer не
заменяется статистикой. Нулевой count P может означать отсутствие достаточных
receipts, а не нулевую фактическую надёжность. Нужно показывать availability/
unknown и узкий предмет измерения. Успешные episodes без доказанного claim
остаются X, даже если их визуально хочется сделать зелёными.

### 4.7 Сводка ручных противоположных случаев

Числа иллюстративны; это reasoning traces в документе, не запущенные cases,
не выбранные проектные horizons и не синтетические результаты системы.

| Trace | Исправленный вывод |
|---|---|
| t0=0,D=10; A timely success на1, B pending на5 | fixed N=2: при достаточном claim A bound [1/2,1]; mature subset на5 пуст. По maturity оба ещё pending, но исход A уже известен — эти оси не суммируются как взаимоисключающие outcomes |
| На10 B достоверно не завершился к D | тот же N=2, bound [1/2,1/2]; обновился evidence, не выбор удобных записей |
| На2 «пока partial», на8 completion при D=10 | early partial не absorbing failure; учитывается timely completion с отдельно известной/unknown policy |
| На11 пришло доказуемое original completion на4 при D=10 | новая report revision того же episode/N; arrival не переносится в event time. Без надёжного original time timely outcome unknown |
| Clock после reboot нельзя связать с t0 | maturity unknown; никаких t0+D≤T по wall clock. Уже доказанный timely факт сохраняется, если его собственная временная связь валидна |
| В ring осталась одна успешная запись, другая вытеснена | при известном census N=2 вытесненный исход unknown; если census утрачен — denominator unavailable, не100% |
| Explicit Stop принят, foreign VPN мог продолжить защиту | admission не доказывает own release; собственная policy и глобальная защита различаются. Task completion после него не автоматически U/P |
| Settings opened / consent granted / порт отвечает | только соответствующий action/fact receipt; task и protected completion ещё не наблюдались |
| Resource release подтверждён после D | отдельная R-ось; task failure/missing не исправляются release событием |

### 4.8 Retention и resource clock — два дополнительных seam

**Dedup после потери записи:** сохранённый aggregate N не даёт право применить
любой поздний receipt к нему. После eviction может быть утрачена проверяемая
membership/linkage и возможность отличить повтор доставки. Тогда поздний исход
не применяется, unknown остаётся. Будущая платформа должна выбрать bounded
membership/dedup contract; нельзя обещать бесконечную точность ring128.
После privacy Clear новый cohort/generation не принимает старые delayed events;
удаление не должно скрыто превращаться в tombstone tracker.

**R origin:** follow-up ресурсов относительно terminal task из baseline не
измеряет задержку конкретного retirement. Для lifecycle-показателя нужен свой
`r0 = retirement requested` для соответствующей generation и отдельно заранее
заданный R. Task может ещё продолжаться, когда старый worker уже должен уходить.
Такой показатель называется request-to-confirmed-release, а не чистым временем
native teardown: фактическое начало teardown/единый clock отдельно не установлены.
Пересечение R и D не делает их общей шкалой успеха; R deadline не является release.
Несколько retirements одного recovery episode — несколько resource observations,
а не несколько независимых task successes. Численное R и platform release proof
здесь не выбраны и не реализованы.

## 5. Reuse/adapt/defer и цена

| Решение | Что именно берём | Pin/API/licence scope | Цена и незакрытая граница |
|---|---|---|---|
| REUSE существующего Android projection | `ConnectionPresentation`, `workerLifecyclePresentation`, Tile recheck и foreground `openSettings` | Точные текущие local hashes в inputs; source SPDX GPL-3.0-or-later, `android/LICENSE` прочитан. Нового third-party кода нет | Согласовать labels и consumer semantics; не заводить второго владельца lifecycle. Показ/понятность/a11y на устройстве ещё неизвестны |
| REUSE Windows poller / ADAPT projections | generation fence уже отбрасывает позднюю публикацию; NetGuard typed task states сохраняют no-access/not-found/unknown | local hashes; новый OSS import/redistribution не выполняются, отдельное заключение о лицензии Windows tree здесь не делается | Небольшой scope правки presentation, но нужны producer dating/availability вместо сильных promises. Трудозатраты и regressions не измерены |
| ADAPT event delivery, не census | `SessionJournal` typed command/state/worker observation как diagnostic reference | Android local source; capacity128/process-local/dedup/Clear проверены | Новый опциональный episode record нужен отдельно; ordering, crash ambiguity, membership, retention, clock/join и privacy ещё проектируются. Не расширять журнал raw payload |
| CONDITIONAL API ADAPT | public Process sandbox mapper; OS owner/visibility contract | Public floor API35; AOSP exact commits/Apache-2.0 headers; compileSdk36/minSdk29 у проекта | Сохранить raw UID и attributed identity/provenance раздельно. Замена одного `userId` не доказывает native app-rule semantics; signer/user/profile/cache/epoch/admission нужно согласовать. API29–34 unsupported/unknown, не arithmetic fallback |
| REFERENCE, без кода | SRE user-oriented good/total event framing; ICH endpoint/intercurrent/missing vocabulary; RFC9110 retry semantics; W3C status-message guidance | Официальные первичные документы, точные sections в SOURCES; новых library dependencies нет | Методический перенос требует нашего producer/endpoint contract. Эти документы не валидируют VPN, estimator, доступность или восстановление |
| DEFER внешнего measurement framework | OONI local-save/submit separation уже разобрана в baseline SOURCES-013 | baseline-only: probe-cli v3.31.0 / `1605e00481e2b98488d62cfc2f7ce34ff345eb70`, GPL-3.0 по старому capture; новый source-аудит OONI не выполнялся | Не брать payload/IP scrub/uploader/experiments как готовый recovery metric. Здесь нет gap, требующего новый SDK; current licence/source fitness для копирования не установлены |
| DEFER автоматического ремонта чужого окружения | NetGuard port heuristic не становится ownership contract; скрытый captive API/прямой fallback не добавляются | Source-only counterexample, без OS-setting/device/VPS действий | Подлинность владельца и конкурирующие внешние записи требуют отдельной платформенной постановки; user permission не создаёт отсутствующего API/receipt |

Вместо переноса framework предложено расширение уже существующих boundaries.
Для episode store ожидается ограниченный объём локальных записей/агрегатов,
зависящий от числа enrolled episodes и выбранной retention. Конкретные bytes,
RAM/CPU/battery, I/O latency, сроки и token/cost **не измерялись**. Нет оснований
обещать нулевую стоимость или вводить периодические network probes ради счётчиков.
Все будущие events — опциональные local records; автоматическая отправка не нужна.

### 5.1 Paired review

Один адресный read Z08/Z09 TASK и проверка HANDOFF выполнены на review stage;
clock receipt после batch: **09:21:32Z**. Оба HANDOFF **NOT_AVAILABLE_AT_REVIEW**.
Сообщений, ожидания или повторного polling нет. Назначение Z08 совпадает с
raw→attributed identity/cache seam; Z09 — с исправлением maturity/outcome.
Ни один неподготовленный результат не заимствован. Независимый peer input остаётся
неполученным; этот законченный авторский пакет готов для проверки руководителем.

## 6. Последовательные предложения платформенным владельцам

Это handoff порядка работ; product PLAN и чужие STATE не изменены.

1. **Сначала semantics без новых механизмов:** Windows убрать повышение
   Scheduler/no-access/порта до protection/repair guarantee; Android сохранить
   текущие Stop/Tile/settings guards и явные unknown. В обоих клиентах request,
   observation, successful effect и user task — разные сообщения.
2. **Затем producer contract:** установить доступные observation scope/time/
   generation и недостающие поля. Не собирать work/package inventory ради красивой
   причины. Для неизвестного foreign owner безопасная навигация/инструкция,
   не автоматическое отключение. Отдельно разобрать Windows proxy heal ownership.
3. **До app-rule admission:** Android raw UID↔semantic identity, public API matrix,
   signer/user/profile/epoch и native cache/lifecycle должны согласоваться с
   подписанным renderer/capture parity. Не снимать blockers ради нового UX.
4. **До recovery процентов:** определить маленький opt-in local episode store,
   enrollment frontier и privacy deletion, clocks/dedup/linkage, fixed goal/D/R,
   формулы §4.6 и report revision. Если census/receipt producer недоступны,
   показывать ограниченный diagnostic/count результат.
5. **После отдельного разрешения на проверку:** источник истины для protected
   path/task/resource, платформенные проверки lifecycle/handover и понятность
   действий/доступность. В текущем plan-only режиме этот шаг **NOT_RUN**.

## 7. Границы вывода и остаток

| Что установлено | Что ещё не установлено |
|---|---|
| Текущие source callers/guards, конкретные weak-to-strong UI переходы Windows | Реальные ОС-политики, доступность системных экранов, фактический foreign owner и работа служб на устройстве |
| Публичный mapper floor API35 и exact-commit Android capture/owner/visibility semantics | Наличие нужной capability на конкретном устройстве; native AAR/ABI/graph/capture parity и old/new-flow retirement |
| Логическая ошибка outcome-dependent N и исправленный conditional accounting | Полный durable episode census, clock bridge, sound task/protection producer, реализованный pilot/реальные результаты |
| Компактный action contract и source-based opposing traces | Понятность текста человеку, OEM notifications/Tile display, a11y/focus и реальный UX |
| Reuse в существующей архитектуре, без нового uploader/framework | Измеренные стоимость/скорость/CPU/RAM/батарея и преимущество над другим VPN |

Runtime/tests/fixtures/build/vet/doctor/simulation/benchmarks/security scan/
native/device/live/admin/VPS/SSH/deploy/OS network — **NOT_RUN**. Никаких новых
исполняемых acceptance cases. Product code/plans/dependencies/roles/bindings,
общая theory и прежние отчёты не менялись этим пакетом. Заявления «лучший VPN»,
«безотказность», «теоретически исчерпан весь максимум» из этого исследования не следуют.
