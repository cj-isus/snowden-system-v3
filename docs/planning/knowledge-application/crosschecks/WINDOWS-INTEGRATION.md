# Windows: применение теории защиты, восстановления и рабочей сети

**Вывод:** уточнять существующие владельцев ресурсов и отображение результатов;
новый контроллер не требуется. **Применение:** [ПК, этапы 01 и 03–07](../PC-PLAN.md).
**Ограничение:** описанные пробелы относятся к исходникам 28 октября; их наличие
после изменений проверяется по плану и реестру владельца Windows.

## Состояние кода на source cut 2026-10-28

Windows-клиент уже имеет существенную основу, которую не следует заменять новым
controller или вторым daemon:

- `App.operationMu` сериализует lifecycle; `operationID`, session/owner generation,
  `ObservationID` и network epoch отсекают устаревшие результаты.
- `Manager` разделяет prepare и lifecycle ownership, удерживает engine process,
  возвращает typed cleanup uncertainty и допускает rollback только после свежего
  old-path probe.
- `StopReason` снимает strict WFP только при явном Disconnect. Автоматическая
  ошибка, quit и restart сохраняют барьер; неочищенный engine/helper переводит
  UI в `cleanup_unconfirmed` и исключается из auto-retry.
- TUN boundary связывает worker/config/selector, LUID/WFP и внешний probe при
  Start; periodic observation повторяет worker/config/LUID, но не внешний HTTP.
- Probe report внутри backend уже содержит session, owner generation, intent,
  observation ID, network epoch и времена. Публичный `ProbeReportView` теряет
  эти поля и оставляет только boolean `OK` и шаги.
- Renderer уже имеет DNS hijack, RFC1918/`.local` direct и TUN exclusion
  `172.16/12`; это не signed корпоративная split-DNS/LAN policy и не proof её
  эффективного socket-hop.
- Frontend polling и backendState имеют single-flight/generation guards; основной
  пробел — сила отображаемых обещаний и недостаточная типизация producer facts,
  а не отсутствие очередного store/router.

Все эти пути после V2-110 помечены в STATE как source-only/unverified; сохранённые
старые live PASS относятся к другим входам. Ниже предлагается применение знаний,
не изменение статусов PLAN и не product implementation.

## Карта знаний → текущий код/план → минимальное решение

| Семейство знания и frozen sources | Current code и plan | Разница / opposing guard | Решение |
|---|---|---|---|
| Fail-closed: intent отдельно от engine; timeout не завершает owner; cleanup uncertainty удерживает барьер. `SAFETY-AND-TRUST` K-b45…, K-ec96…; recovery K-fddf…, R06 K-c667… | `protection.go`, `Manager.Stop`, `Engine.retireIsolatedLocked`, `RetireCaptureHelpers`, `cleanup_unconfirmed`; FR-009/010, OFF-01/02/03 | Сильные guards уже есть: Stop intent, Job/reaper, WFP retention и retry suppression. Не доказаны OS process/WFP/reboot и старые TCP/UDP flows. Нельзя предлагать rewrite lifecycle или считать Stop acceptance cleanup completion | **REUSE existing lifecycle.** Добавлять только frozen receipts для proxy/settings и projections; native/OS приёмка остаётся gate |
| Proxy settings принадлежат actor/epoch; snapshot rollback может затереть внешний S2. R06 K-b87…, K-cee…, K-273…; C06 K-ad9… | `ProxyManager.Snapshot→SetProxy→Restore`; startup/manual `HealStaleSystemProxy`; NetGuard UserMode повторяет эвристику; COM-06/07, OFF-03/05 | Existing guard узок: только 127.0.0.1:1080, внешний/комбинированный proxy/PAC и живой listener не трогаются. Но адрес/мертвый listener не доказывают owner; heal и Restore имеют read/write TOCTOU, игнорируется PAC delete/readback, отсутствует receipt effective values | **CHANGE REQUIRED P0:** общий owner-aware proxy transaction до любых writes; unknown/conflict не мутирует foreign settings. UI manual heal — request/outcome, не гарантированный repair |
| Proxy P и endpoint E — разные hops/resolvers; browser SSO/metadata не transport permission; CONNECT/auth scope отдельны. PROXY-DNS K-0ade…, K-e5cd…, K-746… | Нет upstream corporate proxy adapter в admitted profile/render/guard/probe. HKCU manager — downstream proxy для apps; HTTP connector upstream существует лишь как GPL building block. COM-06/07 | Сильный guard: текущий profile admission не разрешает raw HTTP node, поэтому unsupported path не становится bypass. Добавление одного outbound не закрывает PAC, NTLM/Negotiate, DNS realm или WFP bootstrap | **DEFER transport adapter** до отдельного profile/schema/security design. Сейчас реализовать compatibility preflight/typed unsupported only; не брать browser/default credentials и не менять PAC |
| Route/DNS/private/foreign VPN нельзя выводить из отсутствия одного WFP permit; effective hop важнее названия policy. R06 K-084…, K-5f62…, K-2aa…, K-ef7… | Renderer RFC1918/.local direct, DNS hijack, route.final proxy, 172.16/12 TUN exclusion; static WFP AppID/LUID permits/catch-all. COM-04/05/06, OFF-05 | Engine AppID может повторно открыть LAN socket, physical exclusion — иной case; external WFP layers/veto неизвестны. Неподтверждён комментарий «HardAction=false делает static block soft». Нет per-suffix resolver contract/receipt | **NEEDS REVIEW then ADAPT:** сначала native WFP arbitration gate. Затем signed route/DNS policy compiler с explicit scope; no blanket private ranges/direct. Не строить UI обещание из renderer config |
| Proof должен разделять protection, task/path и recovery. Probe DNS/HTTPS/egress не business task и не вся OS. R06 K-dc53…, C06 K-dcc…, Z09 K-848… | Core report богат owner/intent/epoch/time; `toView` сворачивает в OK+steps; Dashboard пишет «Соединение защищено», «DNS через туннель», «Доступ к ресурсам открыт», «утечек нет». COM-07/11, OFF-05/08 | Existing normalization правильно не даёт PASS при unavailable/leak; UI labels расширяют scope: DoH/SOCKS step≠native DNS, две HTTPS цели≠resources, leak step≠весь device interval | **ADAPT projection P0/P1:** передать scope/freshness/evidence kind, заменить absolute copy на scoped facts/unknown; task completion отдельно и только opt-in future producer |
| Recovery retry имеет один owner/budgets; network transition должен отзывать old proof. Historical K-fddf…, R06 K-4a99…, performance K-cfe… | `autoretry` exactly-once/backoff; diagnostic lanes separate protected/optional; netwatch epochs up/change, но TUN mode polling skipped; watchdog periodic. FR-010, OFF-03/05, COM-03/09/12 | Strong guards: no retry for trust/cleanup/restart custody; protected lane reserved. Gap: physical transition without event in TUN means old epoch may persist until watchdog failure; no business-operation retry is present and should not be added | **REUSE budgets/owners; CHANGE observation contract:** introduce dated physical-path observation/coverage state, not another retry loop. Unknown/no-event TUN remains unknown; mandatory revoke outranks quiet mode |
| NetGuard task/listener facts are not protection/repair receipts. C06 K-ad9…, K-406… | `NetGuardStatus`, task COM status, local port connect, untrusted log, Vue hero; OFF-08, COM-07/12 | ready/running/no-access and TCP connect have narrower semantics. Current hero says protection active, tasks work, repair within 2 min, SYSTEM continues. Opposing guard: status parser preserves no-access/not-found/unknown and poller generations | **CHANGE REQUIRED P0 presentation:** scheduler available/running, status unavailable, local port responds; no protection/automatic repair promise. Date producer facts and split task/proxy/DNS sections |
| Resource/custody: bounded visible queue not total memory; timeout not owner completion. TRANSPORT K-2c1…, K-791…, K-f829… | Engine worker Job/process owner; diagnostic requests/bytes/minute and active≤2; helper lanes retained. COM-12, OFF-02/05 | Existing limits useful; NetGuard external scheduled task and PowerShell DoH probes are outside Go diagnostic budget. DNS script can rewrite adapter servers and DoH HKLM globally, and has no owner snapshot/restore contract | **DEFER/ISOLATE legacy NetGuard DNS heal:** inventory as separate privileged legacy mechanism, do not treat as client recovery. Any future migration needs policy owner, exact adapter generation/snapshot/conflict rollback, budgets and admin consent |
| Important session: optional switching/update can wait, safety cannot; ordinary selector switch does not promise old-flow continuity. R06 K-c667…, K-b8d…, PLAN COM-09/17 | Selector/Reload branches, validity constraints, fresh probe; no complete quiet intent contract in Windows source. COM-09 P1, COM-17 P3 | Existing code is safer than blanket claim but comments/UI can imply seamless channel change. Deferred bonding/migration should not block basic truthful scope | **ADAPT later P1:** bounded quiet intent at existing controller; copy says new connections/default only. Keep COM-17 deferred |

## Приоритетная последовательность компонентных изменений

### W0. Зафиксировать контракты наблюдений до UI и новых сетевых функций

**Владелец:** Windows client core. **Пункты плана:** FR-009/010, OFF-01/02/03/05.

Переиспользовать существующие `operationID`, идентичность/поколение manager,
epoch сессии/сети, владельцев процессов и резервирования диагностики.
Компактные внутренние свидетельства должны отражать:

- `LifecycleReceipt`: намерение, владелец/поколение, применённые config/session,
  фаза, исход очистки (`confirmed | retained-uncertain | not-attempted`) и время наблюдения.
- `ProtectionReceipt`: режим, поколение WFP policy, наблюдённые LUID/config/selector,
  вид свидетельства (`lifecycle-external | periodic-local | unavailable`) и его возраст.
- `SettingsReceipt`: actor/owner token, точный исходный fingerprint, записанное значение,
  поколение изменения, результат записи/readback, conflict/unknown и исход очистки.

Сначала фиксируется смысл контрактов; итоговые имена DTO выбирает владелец реализации.
Второй владелец lifecycle, БД и общая шина событий для этого не требуются.

### W1. Владение настройками proxy и условия восстановления

**Owner:** `windows/backend/core`, then App facade. **Plan:** OFF-03/05,
COM-06/07; protection implications FR-009/010.

1. Вместо свободного `ProxySnapshot{values,originalSet}` нужна транзакция владельца:
   точные наличие/значение ProxyEnable/ProxyServer/AutoConfigURL, поколение операции/сессии
   и каноническая целевая конфигурация. Адрес `127.0.0.1:1080` не устанавливает владельца.
2. До SetProxy проверяются полномочия/совместимость: непредставимые managed/policy/
   per-connection состояния остаются `unsupported/unknown` и не изменяются.
3. Изменение выполняется под одним удерживаемым владельцем. Ошибка удаления PAC и
   запись каждого значения требуют readback. Частичный Set означает обязанность
   очистки, а не успешно настроенное состояние.
4. Restore разрешён, только если значения ещё равны ожидаемой записи этого владельца.
   Внешний S2 даёт `conflict_external_change`: S2 сохраняется, интерфейс сообщает действие.
   Сравнение/readback не атомарны относительно MDM; при неопределённости итог не «восстановлено».
5. Автоматический/ручной heal принимает долговечное свидетельство прежней сессии Snowden.
   Старая эвристика остаётся **диагностикой без записи** либо явно подтверждённым пользователем
   режимом совместимости; адрес и мёртвый listener не разрешают автоматический `ProxyEnable=0`.
6. Go startup/manual и PowerShell UserMode должны использовать один контракт;
   дублирующий writer следует убрать, если согласовать его невозможно.

**Минимально достаточное изменение:** owner token и точное сравнение ожидаемого
текущего состояния/readback в существующем `ProxyManager`; без token auto-heal
не меняет настройки. Это защищает чужие proxy/PAC без реализации корпоративной proxy-auth.

### W2. Отображение жизненного цикла и очистки

**Owner:** App/Manager/Engine. **Plan:** FR-009/010, OFF-01/02/03.

- Передавать `LifecycleReceipt` через результаты Start/Stop/Reload/Switch.
  Существующие manager/engine/helper сохраняют владение; неопределённость не разрешает
  очистить runtime или снять WFP.
- Различать `stop_requested`, `engine_retired`, `helpers_retired`,
  `settings_restored/conflict`, `protection_released/retained` и финальный исход.
- Приём Stop и тайм-аут очистки не означают завершённый успех. При неопределённости
  автоматический retry отключён; явное действие повторяет очистку с исходными владельцами.
  Поздний возврат родителя не разрешает публикацию преемника.
- Сохранять существующие source guards: старое поколение не публикует результат,
  намерение Stop фиксируется до ожидания, пробы связаны с owner/session/epoch,
  WFP снимается после успешного завершения Manager/helper.

Эта изолированная работа над lifecycle Windows использует существующие контракты
и не требует предварительно завершить Android SDK или принятый provider.

### W3. Предварительная проверка маршрутов, DNS и рабочей сети

**Владелец:** render/config и платформенный адаптер Windows; политику одобряет её владелец.
**Пункты плана:** COM-04/05/06/07, OFF-04/05/06, FR-001/008/011.

До изменения настроек нужна **предварительная проверка без записи**:

- Предполагаемая область задачи/профиля, владельцы маршрутов и DNS-суффиксов,
  требование carrier (TCP proxy или UDP QUIC), области разрешения proxy P и endpoint E,
  условия WFP/app/LUID, известный конфликт с корпоративным/чужим владельцем и неизвестные поля.
- Кандидат renderer отдельно от наблюдённого/применённого результата ОС. RFC1918,
  `.local` и `172.16/12` не составляют единого разрешения LAN. mDNS/LLMNR/SMB,
  IPv6/link-local требуют явной политики и свидетельства вызывающего пути.
- Для split DNS — подписанная версионированная политика namespace→resolver с конечным
  правилом по умолчанию и областью сети/профиля. Автоматически менять NRPT/PAC/DNS адаптера
  нельзя. Неизвестный или управляемый full-tunnel возвращает требование действия,
  а не запускает ещё один повтор транспорта.
- Будущий корпоративный HTTP proxy условно поддерживает TCP carrier. Исключены неявные
  default credentials и неограниченные повторы 407; успешный CONNECT не доказывает
  inner trust, защищённый выход или завершение задачи.

### W4. Точные состояния UI и формулировки NetGuard

**Owner:** Go projection + Vue existing views/components. **Plan:** OFF-08,
COM-02/07/11/12, TH-04/08.

1. Extend projection from boolean `probe.ok` to scoped evidence already present in
   backend: evidence kind, observation time/age, mode/scope, owner/session/epoch match,
   protection unavailable/leak/unknown, and cleanup/settings conflict. Raw endpoints,
   PAC URLs, credentials and organization identifiers stay out.
2. Copy ladder:
   - **Request:** “Проверка/остановка/ремонт запрошены”.
   - **Observation:** “Планировщик: ready”, “Локальный порт отвечает”,
     “WFP/LUID обнаружены”, with timestamp/scope.
   - **Applied:** only from component receipt.
   - **Protected:** only current scoped protection receipt + applicable path evidence.
   - **Task complete:** never inferred from VPN probe.
3. Replace NetGuard hero promises:
   - `ready` → “задача готова к запуску”; `running` → “экземпляр выполняется”;
   - `no-access` → “состояние задачи недоступно”, not “SYSTEM continues”;
   - stale heuristic → “совпала конфигурация 127.0.0.1:1080, listener не ответил;
     владелец не подтверждён”; disable mutation button until owner receipt or explicit
     legacy-confirmed flow;
   - no “исправится за 2 минуты” without scheduled/fresh successful receipt.
4. Dashboard scoped labels: “Проверенные цели прошли через заявленный путь”,
   “DoH-проверка через туннель”, “ожидаемый egress совпал”, “в этом probe прямое
   расхождение не обнаружено”. Remove “resources open/no leaks” absolutes.
5. Reuse existing backendState/poller generation and components; no router/store/UI
   framework. A11y/native WebView2 remains NOT_RUN.

### W5. Разделение ресурсов клиента и старого NetGuard

**Owner:** Windows/platform operations, not UI. **Plan:** COM-12, OFF-05/06.

- Сохранить `DiagnosticProtected` и необязательные линии. Отказ бюджета — отказ
  локального допуска, а не неисправность канала. Повторами владеет один контроллер;
  PTO/HTTP/UI не создают независимые циклы восстановления.
- Отдельно учитывать старый привилегированный NetGuard DNS heal: выбор адаптера,
  DoH-пробы, флаги HKLM и замена DNS не представлены lifecycle/epoch/бюджетом App.
  Без контракта владения, конфликта и восстановления он не подтверждает защиту Snowden.
- Предпочтительны сужение или прекращение использования скрипта. Если он сохраняется,
  нужны явная административная область политики, идентичность/поколение адаптера,
  прежнее состояние DNS, восстановление с учётом конфликта, ограниченные сетевые вызовы
  и отдельный статус в интерфейсе.

## Минимальные контракты и зависимости

| Contract | Minimal inputs | Outputs / consumers | Dependency and gate |
|---|---|---|---|
| Settings ownership | exact value presence/bytes, actor+intent/session, owner token, expected current, policy/managed availability | applied/conflict/unknown receipt; cleanup obligation | W1 before any automatic proxy mutation; real HKCU/MDM fault acceptance NOT_RUN |
| Lifecycle result | intent, owner/generation, engine/helper owners, proxy transaction, protection reason | phase-specific receipt, retained uncertainty; App/UI | Existing operationMu/Manager/Jobs/WFP; no new owner |
| Route/DNS decision | signed profile/policy generation, network/profile scope, app/service/destination, namespace/resolver, carrier/proxy capabilities | eligible/unsupported/unknown + explain; renderer/platform adapter | COM-04 first; OS route/DNS/WFP evidence before protected claim |
| Protection evidence | config/session, WFP generation+LUID, evidence kind, network epoch, start/completion time, path targets | scoped protection projection; recovery controller | Lifecycle external evidence stronger than periodic local observation; no all-device promise |
| UI action | typed reason, evidence scope/age, current intent, allowed actor action | request receipt and later effect receipt; views/toasts | UI never owns authority; generation fences retained |
| Recovery episode (future) | fixed goal/scope/time basis and task producer | pending/success/failure/unknown; local opt-in | Z09/C06 contract; not prerequisite for W0–W4 safety changes |

Изменения Windows W1/W2 и исправление отображения NetGuard не зависят от полного
завершения Android/SDK. Реализация split-DNS/proxy транспорта требует согласования
профиля, безопасности и платформенной совместимости; предварительная проверка без
записи и точное состояние «не поддерживается» возможны раньше.

## Контрпримеры и сохраняемые ограничения

- Другой VPN использует тот же loopback/порт: совпадение адреса не устанавливает
  владельца. Существующие ограничения для другого порта, внешнего proxy, PAC и живого
  listener сохраняются, но не дают достаточных полномочий на изменение.
- MDM/пользователь меняет proxy после Snowden: конфликт ожидаемого состояния сохраняет
  новую запись. Mutex процесса не синхронизирует внешнего участника.
- Listener принимает TCP, но не является рабочим SOCKS/Snowden: это только наблюдение порта.
- Stop принят, а worker/helper завис при закрытии: преемник и освобождение запрещены;
  действуют существующие Job owner и `cleanup_unconfirmed`.
- Сеть меняется во время пробы: epoch/owner/intent отклоняют устаревший результат.
  Отсутствие события в TUN не позволяет обновить время старого свидетельства.
- Частный маршрут через engine может работать при заблокированном прямом пути приложения,
  и наоборот при других слоях WFP. Конфигурация не доказывает фактический результат.
- Proxy отвечает 200 или браузер работает, но auth/carrier/inner trust connector не проходят:
  сообщается точный этап, без перебора паролей и автоматического ослабления транспорта.
- Состояния планировщика ready/no-access и старое событие лога не доказывают текущий эффект.
- Пользовательский Stop завершает намерение, но не доказывает завершение задачи или снятие чужой политики.

## Будущая приёмка — всё NOT_RUN

1. **Владение proxy:** неизменённая собственная запись; PAC; частичный Set; внешний S2
   до restore/heal; конкурентная запись при compare/readback; чужой listener на том же
   адресе; crash/reboot и отсутствие receipt. Чужие настройки не перезаписываются.
2. **Lifecycle:** Stop/Disconnect/failure/restart/quit на каждой фазе Start/Reload;
   неопределённое закрытие engine/helper и обеих helper lanes; поздний результат;
   WFP снимается/сохраняется по причине, преемник ждёт фактического завершения владельцев.
3. **WFP/TUN:** арбитраж статических фильтров, повторная авторизация, прежние TCP и
   односторонний UDP, внешний provider/veto, перезапуск BFE, IPv4/IPv6/DNS/reboot.
   Перечень исходников, транзакция и удаление маршрута не заменяют приёмку.
4. **Маршруты/DNS:** 192.168 и исключённый 172.16; IPv6/link-local/overlap; unicast
   `.local` и mDNS/LLMNR/SMB; потеря split suffix/resolver; корпоративный full-tunnel;
   proxy по имени и числовой endpoint; TCP и HY2 UDP carrier.
5. **Observer:** смена физической сети под TUN без события, sleep/resume, потеря callback,
   поздний отчёт, ABA epoch, локальное периодическое и внешнее lifecycle-наблюдение.
6. **Ресурсы:** защищённые и необязательные линии, блокирующие сетевые/IО вызовы,
   очереди writers/buffers/helpers, память процесса и PowerShell планировщика.
   Тайм-аут не возвращает бюджет ещё выполняющейся работы.
7. **UI:** фактические состояния, stale/unknown, конфликт очистки/настроек, статусы задач,
   масштабирование native WebView2, клавиатура, screen reader и понятность формулировок.
   Браузер/mock не доказывает поведение ОС.

## Связь с действующими пунктами плана

- **P0 — переиспользовать/уточнить:** FR-009, OFF-01/02 для барьера, процессов и очистки;
  owner proxy W1 необходим до автоматических записей auto-heal.
- **P1:** FR-010, OFF-03/05 для атомарности operation/evidence/observer;
  COM-04/05/06/07 для маршрутов/DNS/рабочей сети; COM-12 для ресурсов; OFF-08 для UI.
- **Позже:** COM-09 — quiet intent после safety receipts; COM-11 — готовность задачи
  после контракта producer; COM-17 — бесшовность/bonding остаются отложенными.
- **Не включать сейчас:** широкие LAN/direct fallback, автоматическую смену корпоративной
  политики, неявные default credentials, второй controller/daemon/store, заявления о
  защите из Scheduler/порта/readback и новую БД/платформу телеметрии.

## Источники и границы разбора

Снимки исходников и планов на 28 октября получены последовательно, не атомарно.
Этот разбор описывает их ограничения; позднейшее состояние проверяется по плану ПК.
Срок реализации, runtime-расходы и полевой успех остаются неизвестными и не заменяются нулём.

<a id="w0-зафиксировать-frozen-producer-contracts-до-ui-и-новых-network-features"></a>
<a id="w1-proxy-settings-owner-and-heal-precondition--first-code-slice"></a>
<a id="w2-lifecyclecleanup-projection-not-another-controller"></a>
<a id="w3-effective-routedns-and-working-network-compatibility-preflight"></a>
<a id="w4-ui-weak-to-strong-ladder-and-netguard-copy-correction"></a>
<a id="w5-resource-and-legacy-netguard-separation"></a>
<a id="frozen-contracts-and-dependencies"></a>
<a id="strong-opposing-cases-and-guards-to-preserve"></a>
<a id="future-acceptance--all-not_run"></a>
<a id="existing-plan-routing-and-disposition"></a>
<a id="sources-coverage-and-limits"></a>
