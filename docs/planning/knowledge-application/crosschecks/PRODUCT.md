# Product crosscheck: полезность, recovery UX и честная готовность

> **Область и статус: PARTIAL.** Сопоставление использует прочитанные выдержки
> current Android/Windows UI и product plans, а также ранее разобранный R06/R28
> baseline. Frozen inventory этой application-фазы проиндексирован, но тела его
> units адресно не прочитаны и не сверены с прежним baseline. Поэтому 61 ссылка
> на units учитывается как `indexed/prior-baseline`, а не `reviewed` coverage.
> Полное применение comfort/UX/Android/recovery корпуса этим документом не закрыто.
>
> **Applicability cut:** последняя содержательная партия была запрошена до
> deadline `2026-10-28T12:53:51Z`; возвращённый source-hash capture имеет время
> `12:57:23Z`. Это поздний capture, не подтверждение соблюдения cutoff всей
> партией. Последующая hash-only сверка обнаружила изменения `SnowdenApp.kt`,
> `SystemScreen.kt`, `PLAN.md`, `ANDROID-PLAN.md`, `docs/STATE.md`, `android/STATE.md`.
> Новые bodies не читались. Ниже — предварительные предложения для review,
> а не принятие актуального Android/UI/plan состояния.

## 1. Фактическое состояние продукта

Snowden имеет существующие интерфейсные границы для раздельного отображения
состояний. В прочитанных выдержках UI/API task-level receipt не выявлен. Это
ограниченный результат, не доказательство отсутствия такого producer во всём
репозитории: полного end-to-end caller review эта application-фаза не завершила.

- **Windows:** `DashboardView.vue` различает stopped/starting/running/error,
  `blockedReason`, retry и статусы protected-probe; `contract.ts` сохраняет
  `unavailable/unknown`, а не превращает отсутствие данных в ноль. Это готовая
  presentation-граница для protection/path facts, но не доказательство полезной
  задачи (`windows/frontend/src/views/DashboardView.vue:50-84,134-151,227-289,372-400`;
  `windows/frontend/src/api/contract.ts:19-21,38-54,89-101`).
- **Android:** `ConnectionState` хранит lifecycle/network/issue; `SnowdenApp`
  передаёт Start/Stop/Refresh и OS-settings actions; `SystemScreen` уже отделяет
  diagnostics/history/settings; `MainActivity` явно помечает settings/share как
  dispatch, не завершённое действие (`android/.../SnowdenApp.kt:63-74,135-178,209-214`;
  `SystemScreen.kt:32-49,64-119`; `MainActivity.kt:55-63,105-121,124-141`).
- **Android diagnostics:** process-local `SessionJournal` и frozen report/share
  дают bounded typed evidence, но не полный аудит, task outcome или доказательство
  VPN. Новая БД/telemetry stack только ради красивой метрики не обоснованы.
- **Планы:** в прочитанном срезе `COM-00` принят как документация, а
  `COM-01…19` обозначены planned; Android AAR/APK/device gates оставались
  открытыми. PLAN/STATE изменились после этого среза: текущие статусы требований
  не подтверждены настоящим crosscheck. UI source не является доказательством
  готового пользовательского сервиса.

В просмотренных UI/API выдержках нет достаточного основания объявлять банковскую
операцию, рабочий звонок или печать завершёнными и защищёнными на всём интервале.
Это граница данного crosscheck. Полный поиск и проверка task-level producer в
backend/native/application chain не завершены; отсутствие producer во всём
продукте не доказано.

## 2. Применение накопленных знаний

| Family | Корпус / источник | Existing code и plan | Реальная разница | Решение |
|---|---|---|---|---|
| **P1. Честная готовность** | R06-01; R06-013 review; R28-Z09/C06 | Windows Dashboard/contract; Android `ConnectionState`; TH-08, A-FR-02 | `running`, point probe и UI declaration не равны protected task | **Change required:** единый presentation contract из отдельных protection/path/task/clock facts; без нового transport/controller |
| **P2. Повседневный путь** | Current UI/source и планы; frozen S1/competitor/Android UX bodies только индексированы | COM-01; A-FR-07/AND-08; существующий onboarding/navigation | Установка→профиль→Start есть как компоненты, но нет принятого end-to-end useful-task PASS | **Reuse + future acceptance:** один основной путь, без ручного протокола/порта/MTU в everyday UI |
| **P3. Смена сети и сна** | Current lifecycle/source и планы; frozen S2/R06-003 bodies только индексированы | COM-03; A-FR-04/AND-05; lifecycle/network facts | Connectivity callbacks/state не доказывают незаметный рабочий звонок | **Already planned:** показывать recovery/action status; task continuity остаётся unknown до task receipt |
| **P4. Portal/LAN/рабочая сеть** | R06-010; current OS-settings/UI callers; R06-006 body не рецензирован в этом cut | A-FR-05/AND-06; OS settings handoff; COM comfort routes | Строгая защита конфликтует с portal, LAN, split DNS, proxy/MDM/чужим VPN; universal direct fallback небезопасен | **Defer to policy owners:** scoped action/unknown; не обещать автоисправление и не советовать blanket protection-off |
| **P5. Сервер/control-plane отказ** | Current failover/retry source и планы; frozen S5/R06-002 bodies только индексированы | COM-13/14/15, TH-07/08; recovery/import seams | Резерв без замкнутых prerequisites не восстанавливает задачу | **Apply existing roadmap:** bounded retry, recovery packet/runbook, честное “причина неизвестна”; без нового owner/UI wizard до contract gates |
| **P6. Диагностика/privacy** | R06-004; R06-013 privacy sections; current journal/report/share callers | Android journal/report/store/share; Windows diagnostics/log UI; COM-05/07/12, AND-12/A-FR-17 | Current typed export полезен, но не causal diagnosis/task proof | **Reuse current components:** aggregate/unknown/coverage; explicit manual frozen export; no automatic upload/raw task inventory |
| **P7. Task/cohort metrics** | Ранее разобранные R06-011/013, NEEDS_REVISION, R28-Z09/C06; frozen bodies в этой фазе не прочитаны | В просмотренных UI/API task producer и complete roster не установлены; глобальное отсутствие не доказано | Early success/pending, неизвестные clocks и point protection receipt недостаточны для task rate | **Defer:** полный caller/producer review нужен до предложения production metric; не назначать новую БД по отсутствию поля в UI |
| **P8. Comfort/competitor reuse** | Current code/plans/project rules; frozen competitor/Android reuse bodies только индексированы | Existing NavRail/tabs/tray/tiles/widget/settings/report; COM-01…19 | Feature parity легко добавляет ручные controls без пользы и authority | **Selective reuse:** reuse navigation/actions/explain; reject new framework, duplicate tray/store, protocol knobs and superiority claims |

## 3. Необходимые бытовые и рабочие сценарии

Минимальный набор будущей пользовательской приёмки — не полный каталог функций:

1. **Первый защищённый путь:** установка/импорт допустимого профиля → Start →
   свежий protection/path receipt → одна заранее объявленная task endpoint.
2. **Обычное утро:** cold start/offline-valid state/обновление без fake Ready и
   без обязательной повторной ручной конфигурации.
3. **Wi-Fi ↔ LTE / sleep-resume:** lifecycle остаётся управляемым; old generation
   retired/unknown показывается отдельно; рабочий звонок не объявляется сохранённым
   по одному HTTP probe.
4. **Captive portal:** защита не снимается автоматически; UI показывает внешний
   OS/network prerequisite и одно допустимое действие.
5. **Дом/офис:** scoped LAN/internal-DNS/policy outcome; отсутствие permit —
   blocked/unknown, не blanket direct. Корпоративный VPN/MDM остаётся отдельным
   владельцем.
6. **Рабочая сеть с proxy/PAC:** browser success не доказывает VPN support;
   credentials/settings не собираются “для удобства” без admitted contract.
7. **Сервер/control plane недоступен:** bounded retry → action required/offline
   recovery; неоплата/ACL/ISP/overload не диагностируются из timeout.
8. **Понятная диагностика:** пользователь видит “что известно / что неизвестно /
   что можно сделать” и вручную экспортирует ровно previewed bytes.

Этого достаточно для полезного первого продукта. Multi-hop, bonding, новый
transport survey, десять протокольных knobs и “всё автоматически починили” не
являются prerequisite этих сценариев.

## 4. Минимальная последовательность изменений

### Stage 0 — заморозить честный presentation contract (P0)

**Owners:** Windows backend+frontend; Android VPN repository+app UI.

Состояние для UI составляется из независимых осей:

```text
ProtectionFact = applied(scope, generation, observed_at, clock_scope)
               | off_manual | not_confirmed | unknown
PathFact       = protected_pass(scope, age) | fail | running | unavailable | unknown
TaskFact       = complete | partial | noncomplete | unknown
ActionFact     = requested | admitted | applied | rejected | uncertain
ClockFact      = comparable(age) | timing_unknown
```

- Windows расширяет существующие labels/derived presentation; не создаёт второй
  state owner и не меняет generated Wails bindings без backend contract.
- Android переиспользует `ConnectionState`, presentation mapping и System
  diagnostics; main screen не показывает внутренние phase/reason codes.
- Main action остаётся Start/Stop; channel/protocol/MTU/ports — advanced или
  отсутствуют, если нет подтверждённой бытовой задачи.

### Stage 1 — сценарий → одно допустимое действие (P0/P1)

Переиспользовать existing retry, OS settings, notification permission, history,
report/share и onboarding actions. Каждый action сообщает `requested/rejected/
uncertain`, пока platform receipt не подтвердил effect. Не добавлять generic
“Repair all”, не изменять чужие MDM/PAC/NRPT/WFP и не отключать strict protection
по сетевой ошибке.

### Stage 2 — scenario acceptance без product telemetry (P1)

Для каждого из восьми сценариев владелец заранее фиксирует:

- task endpoint и horizon;
- protection scope, path receipt и clock scope;
- manual-unprotected/partial/missing outcomes;
- expected action и residual resource state.

Это кандидаты будущей device/manual acceptance, а не фоновый сбор. Task status
должен оставаться `unknown`, если достаточный источник исхода не определён;
наличие или отсутствие такого источника требует отдельной проверки caller chain.

### Stage 3 — cohort metric только после contract gates (defer)

R28-Z09 fixed roster/maturity можно применять лишь после появления registration
owner, durable completeness, clock bridge и task/protection coverage producer.
До этого:

- не показывать success percentage;
- не считать ранние successes в меньшем denominator;
- не объединять endpoint-protected и whole-task-protected;
- не считать point protection sample покрытием бизнес-операции.

Новая БД, analytics SDK и OONI code не нужны Stage 0–2. OONI остаётся reference
разделения local save/network submit, не компонент продукта.

## 5. Контракты и совместимость

### 5.1 Protection/path/task boundary

- `endpoint-protected`: task endpoint receipt + producer-declared protection
  coverage этого endpoint event/interval.
- `whole-task-protected`: coverage полного task active interval. Достаточный
  caller/producer в просмотренной области не установлен; неизвестность не
  разрешает наследовать значение от endpoint или утверждать глобальное отсутствие.
- Tunnel-up, port-open, browser mock, Start dispatch и protected probe имеют
  собственные scopes; ни один автоматически не повышается до task success.

### 5.2 Clock/freshness

- Age вычисляется только в сравнимом monotonic/boot scope.
- После reboot/unknown bridge — `timing_unknown`.
- Wall time используется для отображения, не authority/freshness.
- Late success не переписывает своевременный outcome.

### 5.3 Backward compatibility

- Не удалять текущие `state/blockedReason/probe status`; новые UI labels должны
  иметь conservative mapping: неизвестное старое поле → `unknown/unavailable`.
- Android старый provider/AAR, который игнорирует новое поле, не получает Ready;
  compatibility является gate, не fallback.
- Существующий journal/report text-v2 остаётся support export, а не мигрирует
  молча в task/cohort store.

### 5.4 Privacy

Closed enums и случайный episode ID; без domains/URL/IP/SNI/SSID/package/account/
workplace inventory, task content, credentials, stable device ID и raw errors.
Local aggregate default; explicit immutable manual export. Clear не обещает recall
уже shared bytes.

## 6. Будущая приёмка — всё NOT_RUN

| ID | Сценарий | Проверяемый результат |
|---|---|---|
| PA-01 | Start accepted, protected probe pending | UI: action admitted/path running/task unknown; не Ready |
| PA-02 | Probe pass с named scope, task не проверена | “защищённый путь проверен”; task unknown; whole-task unknown |
| PA-03 | Task endpoint complete + coverage receipt | endpoint-protected PASS только в named scope/time |
| PA-04 | Point protection sample, endpoint interval не покрыт | task complete/protection unknown; не endpoint-protected |
| PA-05 | Protection-off → task complete → protection-on | manual-unprotected либо unknown overlap; не whole-task protected |
| PA-06 | Wi-Fi→LTE / resume / stale generation | stale result не перезаписывает fresh; task continuity unknown до receipt |
| PA-07 | Captive portal под strict protection | нет auto-disable/direct; OS prerequisite и действие показаны |
| PA-08 | LAN/split DNS без admitted permit | blocked/unknown; нет blanket direct fallback |
| PA-09 | Corporate VPN/MDM owns policy | Snowden не обещает изменить чужую policy; actor/action наблюдаемы |
| PA-10 | Все каналы failed / VPS unavailable | bounded retry→action required; timeout не “DPI/VPS unpaid” |
| PA-11 | Android process loss/journal eviction | history incomplete явно; current state не выводится из пустого journal |
| PA-12 | Share report | exact displayed bytes; dispatch ≠ delivery; no auto upload |
| PA-13 | Fixed cohort: ранний success + pending | fixed N; pending входит unknown mass; rate не 100% resolved-only |
| PA-14 | reboot/clock bridge absent | timing unknown остаётся denominator, не success/failure |
| PA-15 | old backend/AAR ignores field | conservative unavailable/blocked; не fake Ready |
| PA-16 | 940×600 и Android accessibility | controls named/focusable; blocked/error action доступен без цвета |

Приёмка разделяет source/unit/native/device/live. Наличие компонентов или успешно
отрисованный UI не закрывает native/device сценарий.

## 7. Что не следует делать

- Не добавлять competitor feature matrix в UI и не обещать “лучший VPN”.
- Не выставлять ручной выбор транспорта/портов/MTU основным пользовательским путём.
- Не создавать второй tray, router/store, diagnostics database или shared UX layer.
- Не запускать automatic portal/direct fallback и не снимать protection по timeout.
- Не собирать task content/список приложений/рабочую среду ради readiness.
- Не выводить protected task completion rate до registry/clock/task/protection gates.
- Не называть endpoint-protected whole-task-protected.
- Не использовать source-only Android UI или Windows mock как APK/EXE/device PASS.

## 8. Остаток и владельцы

- **Windows owner:** backend fact scope/freshness, frontend conservative mapping,
  COM-01/03/05/07/12/13–15/TH-08 sequencing.
- **Android owner:** A-FR-02/04/05/07 и AND-05/06/08/12 platform mapping; AAR/APK/
  device gates остаются открыты.
- **Scientific/adaptive owners:** только review fixed-cohort/statistical contract;
  не новый runtime owner.
- **Unknown:** admitted task producer, operation-coverage receipt, numerical task
  horizons, real user burden, device accessibility, OEM behavior and legal/privacy
  approval.

Самая дешёвая достаточная стратегия — сначала честно использовать уже имеющиеся
state/action/diagnostic компоненты, затем закрыть восемь сценариев future device
acceptance. Cohort metric и UX-пилот остаются отложенными до явного отдельного
разрешения и producer contracts.
