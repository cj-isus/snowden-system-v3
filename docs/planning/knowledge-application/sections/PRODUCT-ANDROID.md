# Android и повседневная надёжность: применение знаний

## Цель и фактическая отправная точка

Первый полезный результат — клиент, который принимает действительный профиль,
подключается по разрешённому пути, честно объясняет свою область защиты и безопасно
восстанавливается после отказа. Дом, работа и дорога требуют разных правил, но
одного владельца подключения. Обещание «лучше всех» требует последующих сравнений
на объявленной матрице; исходники и число транспортов такого результата не дают.

Текущий Android имеет Compose-навигацию, controller/service/IPC, policy compiler,
ограниченный импорт, локальную историю и быстрые поверхности. Это **source unverified**:
нет принятого adapted AAR, production `ProfileTrustDriver`, accepted-profile producer,
рабочего APK и подтверждённого VPN. HomeScreen — `android/app/src/main/kotlin/org/snowden/android/app/ui/HomeScreen.kt`
и WorkspaceScreens — `android/app/src/main/kotlin/org/snowden/android/app/ui/WorkspaceScreens.kt`
правильно оставляют Connect и импорт недоступными. Последний просмотренный
Android STATE — `android/STATE.md` содержит AND-05-S4: same-binding TUN
admission; AND-02-S5 callback value-step guards и AND-01-S4 cleanup staging
артефактов сохранены. Эти guards не нужно планировать заново.

В Windows уже есть операция с generation/Stop priority, failover и scoped UI polling.
Их следует развивать. Реальные отличия от требований остались в proxy ownership,
NetGuard copy и доступности некоторых сетевых сценариев. Срез ниже относится к
прочитанным компонентам; сборка, OS/device, внешний capture и live-путь не проверялись.
Точные байты и глубина чтения — source cut — `.artifacts/planning/knowledge-application/2026-10-28/captures/C06/current-source-manifest.json`,
учтённые изменения планов — source deltas — `.artifacts/planning/knowledge-application/2026-10-28/captures/C06/late-diff.json`.

<a id="evidence"></a>
## 1. Основания и правила переноса — F00

Основные основания:

- [Android platform/UX §12.1–12.10 и 35 решений §15.5](../../../theory/android/PLATFORM-AND-UX.md)
  — исходная мобильная архитектура и reuse; архивная копия §15.5 не второй backlog.
- [UID, routing и ownership](../../../theory/android/UID-ROUTING-AND-OWNERSHIP.md),
  [R06-012 §2–9](../../../theory/research/06-RELIABILITY-UX/REPORT-012.md)
  — исправленные platform/MDM/WFP/app-identity границы.
- [R06 REPORT, S1–S5, R06-002…008](../../../theory/research/06-RELIABILITY-UX/REPORT.md)
  — recovery prerequisites, continuity, приватная диагностика, corporate proxy и NAT64.
- [R28-C06 §2–6](../../../theory/research/cycles/2026-10-28-contracts/C06/REPORT.md)
  — truthful actions и уточнение task accounting;
  [Z08 §3–8](../../../theory/research/cycles/2026-10-28-contracts/Z08/REPORT.md)
  — current UID/cache/invalidation;
  [Z09 §2–9](../../../theory/research/cycles/2026-10-28-contracts/Z09/REPORT.md)
  — fixed cohort и endpoint completion.
- [Рецензия R06-013-A](../../../theory/research/reviews/NINE-2026-10-27-R06-013-A.md)
  ограничивает [первоначальный R06-013](../../../theory/research/06-RELIABILITY-UX/REPORT-013.md).
  Готовность пакета к review не означает принятия формулы или продукта.

У каждого переноса сохраняются версия, actual caller, scope и более сильный
контрпример. API reference не доказывает текущий consumer; upstream HEAD не
подтверждает capability pinned AAR; локальная отметка не доказывает egress. Старые
START/STATE/STATUS/инструкции в корпусе учитываются как происхождение и ограничения
выводов, а не как команды разработчику. Неудачно полученный источник не становится
подтверждением, в частности 152-ФЗ здесь не основание заявления о legal compliance.

## 2. Применение по существующим компонентам

Идентификаторы F01–F14 ниже обозначают семейства знаний этого документа, а не новые
проектные требования. Работы относятся к существующим AND/A-FR/COM/OFF/TH.

<a id="trust"></a>
### F01. Принятый профиль, artifact и холодный запуск

**Опора:** `SelectedProfileReader → ProfileImportCoordinator → ProfileTrustDriver →
EncryptedProfileStore`; `SnowdenApplication` прямо сообщает об отсутствии production
driver/source. Reader уже ограничивает URI/байты, сохраняет exact input, удерживает
неопределённый close; coordinator повторно проверяет source revision, approval и
expiry, сохраняет verified delivery progress отдельно от rejected application.
Эти guards не являются криптографической проверкой.

**Решение:** завершать AND-01/04/14, A-FR-03/13 через существующие T01…14 requests
и один whole libbox. Получить принятую portable trust API, связать исходные signed
bytes и историю trust с renderer/prepared source, затем подключить UI preview/apply.
Не вводить Kotlin canonicalizer, signature-only driver, второй Go runtime или fake
accepted profile. `validUntil` signed authority и короткая preparation freshness
остаются разными дедлайнами; reconnect не продлевает authority.

**Сильное ограничение:** текущий manifest отключает always-on/direct boot, service
возвращает START_NOT_STICKY; `WorkerProcess` отклоняет `requireLockdown=true`, когда
свежие OS facts не подтверждают оба флага. Не устранять несовместимость снятием gate
или переключением manifest. Сначала согласовать поддерживаемые режимы и recovery
источник после process death/boot/unlock. CE до первого unlock и обычная блокировка
экрана после unlock — разные состояния. Общий пароль профиля не даёт device revoke.

**Delta:** настоящие driver/accepted producer/renderer + их compatibility acceptance;
повторная реализация bounded reader, CAS или AND-01-S4 cleanup не нужна.

<a id="lifecycle"></a>
### F02. Один controller и доказуемое завершение владельца

**Опора:** `VpnSessionRepository`, `WorkerProcessLink`, `WorkerProcess`,
`NativeWorkerSession`, `NativeCallbackGate`, `NativeScratchCleanup`. В repository
уже есть две retained RPC lanes, sticky retirement epoch, приоритет Stop,
restart budget и запрет нового binding до actual returns. Child death не завершает
parent source/Stub; `sealAndIsIdle()` проверяется отдельно. Native owner публикуется
до JNI; graceful Close требует последующего parent-observed death.

**Решение:** сохранить эти владельцы и завершать AND-02/03/11, A-FR-01/02/10.
Интеграция accepted source обязана пройти тот же путь и не создавать второй
watchdog/retry daemon. Cancellation request, RPC timeout, unbind, process death,
native callback quiescence и storage close — отдельные результаты. Не отпускать
original TUN PFD по истечению grace и не закрывать borrowed transport fd.

**Ограничение:** caps очередей, FileLock и source fences не доказывают ограничение
whole-process RSS, момент kernel release или завершение зависшего Binder/JNI.
Новый план продолжает текущий Android срез, не объявляет его заново отсутствующим.

<a id="network"></a>
### F03. Физический путь, bootstrap и совместимость сети

**Опора:** `PhysicalNetworkMonitor` уже получает ordered callbacks INTERNET/NOT_VPN,
различает availability generations и не активирует radio; `PhysicalPathContext`
включает DNS/routes/proxy/NAT64/metered/roaming в bounded fingerprint.
`ProtectedSocketControl` делает protect и bind owned duplicate с повторными fences.
Это transport socket, не protected probe. `localDNSTransport` сейчас явно unsupported.
В учтённом AND-05-S4 factory уже получает тот же `PhysicalNetworkBinding`, повторно
проверяет его вокруг capability read и перед establish, сохраняя поздний PFD/fact
до postchecks. Не добавлять второй lease/read/poller ради этого же gap; callback
delay и выполняющийся OS syscall всё ещё не атомарны с local admission.
Поздний source delta также передаёт существующий callback gate в socket adapter:
dup/protect/bind имеют отдельные admission steps, owned duplicate закрывается
в retained finally после failed step. Это усиливает исходный guard без доказательства
kernel cutoff или разрешения повторно bind уже connected socket.

**Delta:** AND-05/07 и COM-03/06 должны получить typed failure/capability в контексте
конкретного hop. Разделять logical peer, physical dial target, TLS/SNI identity,
resolver/network context и применимость адреса. При proxy P→E локальный NAT64 может
помогать N→P и не помогать remote P→E. Не подставлять универсальный `64:ff9b::/96`,
не принуждать публичный DNS и не выводить поддержку outer IPv6 из IPv4 TUN.

Windows actual caller: `corebridge.renderAndPrepareContext → PreflightResolver.Resolve
→ render.WithDialOverrides → VLESS server`, только для enabled VLESS+WS; без override
остаётся hostname bootstrap. Resolver предпочитает public IPv4/A, cache по hostname
до семи дней; generic IP literal проходит отдельно. Это конкретный compatibility
delta для IPv6-only/NAT64, **не** отсутствие IPv6 во всём приложении. Уточнять cache
scope/expiry и family selection в существующем пути с сохранением TLS peer identity.

**Ограничение:** declaration underlying network не migration; повторный bind старого
connected socket не безопасный handover. Пока native migration seam не принят,
использовать controlled retirement/restart с честным сообщением о перерыве.
Timeout/403/один неответивший probe не устанавливают DPI, portal или отказ VPS.

<a id="truth"></a>
### F04. Правдивые состояния и допустимые действия

**Опора Android:** `ConnectionState → ConnectionPresentation → Home/Tile/Widget`,
dated `ProtectionState`, отдельный notification observer и consent owner. Нет Ready;
OS facts nullable и aging не продлевает observation. Existing stale-known-worker Stop,
unlocked gesture и settings-on-controller-failure полезны и сохраняются.

**Конкретный Windows delta:** `NetGuardStatus → usePolling → NetGuardView` повышает
ready/running scheduler в «Защита активна», no-access в «SYSTEM-задача продолжает
работать», обещает исправление за две минуты/мгновенно. `DoHDisabled=true` означает
ошибку чтения лога, а не измеренное состояние DoH. `SocksAlive` — TCP listener.
Исправлять backend contract и copy по COM-07/OFF-05/08; это уже противоречит
[UI-DESIGN](../../../UI-DESIGN.md), повторный redesign shell не нужен.

Минимальная проекция — четыре независимых факта: **полномочия/OS policy; lifecycle;
applied route/proof в scope; пользовательская задача**. Для каждого показывать
source availability, original age и permitted next action. Actor `own app/system/
foreign-managed/unknown` назначается только по доступному evidence. Открытие Settings,
consent callback, intent admission и listener не подтверждают эффект действия.
Результат «нет доступа» не устанавливает ни здоровье, ни отказ чужого владельца.

В основном экране достаточно короткого статуса, причины и одного доступного
действия; source/age/scope раскрываются в деталях. Четыре оси не требуют четырёх
новых панелей или перегруженного главного экрана.

`prepare()` допустим в явном consent/Connect пути: это не read-only getter и не
гарантированная кража слота при каждом вызове. Current monitoring его не вызывает.
Ни Stop ack, ни отсутствие нашей policy не доказывают отсутствие другого VPN.

<a id="policy"></a>
### F05. Capture, UID attribution и effective policy

**Опора:** `AndroidTunFactory` отвергает package/excluded-route/proxy capture и
не вызывает allowBypass/allowFamily. `AndroidAppIdentityLookup` уже проверяет own
user, declared sharedUserId, полный observed UID group и exact current signer set;
`LibboxPlatform` удерживает selected UID после исчезновения package. Lowering
конъюнктирует UID+package, выпускает DNS rules и пока отклоняет DIRECT.

**Существующая remediation:** adapted native tagged TUN вызывает
`snowdenRequireOwner` до route/sniff/resolve/outbound через platform searcher,
обходя stock owner cache. Тег задаётся при непустых application bindings. Это
исходники, stock AAR несовместим; не планировать повторный cache bypass. Persistent
session-cache admission — другой seam, также уже получает отдельный override.

**Реальный delta:** raw kernel UID не всегда client app UID. Перед selected-UID
negative checks нужен version-gated partial resolver с raw UID, attributed UID,
mapping provenance/unsupported reason и текущим user scope. Public arbitrary
SDK-sandbox mapping доступен с API35; API33 `isSdkSandbox()` относится к текущему
процессу, не является заменой. API34 isolated classification не даёт client mapping.
На API29–34 неизвестные nonregular UID не угадывать по арифметике; недоказанная
identity не получает direct. Selected shared UID остаётся неподдерживаемым.

Original application lease pre/post callback уже закрывает наблюдённую invalidation,
но не делает PackageManager mutation атомарной с route admission и не проверяет
каждый packet старого flow. Exact signer mismatch требует review/reapproval;
legitimate rotation не означает compromise, signer history не даёт автоматического
нового разрешения. AND-06/A-FR-05/AND-15: отдельно capture set, lockdown exempt set,
UID attribution, DNS-owner semantics, applied native rule и existing-flow lifetime.

LAN/банковские исключения вводить только после effective route/DNS/scope preview.
Android excludeRoute — выход из VPN route, не firewall deny. Builder per-app lists
задаёт VPN-приложение; DPM управляет своим profile/always-on allowlist. Windows
engine AppID/TUN LUID permits исключают blanket-вывод «LAN и другой VPN всегда
запрещены». Actual coexistence остаётся OS gate; `HardAction=false` не основание
самовольно менять WFP flags или обещать soft-block reauthorization.

<a id="resources"></a>
### F06. Ресурсы и батарея как часть надёжности

AND-11/COM-12/TH-06 расширяют существующие lanes/caps и API1 budget contracts.
Нужны отдельные budgets запросов, bytes, active work, retained owners, reconnect и
UI history; safety retirement/Stop не конкурируют с optional bulk probes/update.
Metered/roaming/power-saving влияют на optional work, но не обновляют trust или proof.

Сохранить fixed ring128, single pending poller, callback inventory8, native session
cache/closed-history exclusion и текущие scratch lifetime guards. Число15s-poll
opportunities за сутки не число wakeups; QUIC keepalive не равно OS-offloaded
SocketKeepalive. Настройка Go memory limit/очереди не RSS guarantee. Численные SLO
выбирать до workload измерений; idle/call/bulk/resume и cellular расход измеряются
раздельно. Дорогое фоновое «проверить всё» не нужно для первого рабочего клиента.

<a id="ux"></a>
### F07. Страницы, доступность и быстрые поверхности

Навигация **уже реализована по требуемой иерархии**: Android четыре destinations,
их собственные tabs, rail с600dp; Windows `navigation.ts` — один registry разделов
и страниц. Не создавать альтернативный navigator и не копировать desktop globe.
Сохранять existing Page/Section/FactRow/ConnectionPresentation; выносить новый
компонент только при отдельной ответственности/повторном использовании.

| Экран | Ближайшая полезная доработка | Зависимость |
|---|---|---|
| Главная | Один статус и действие; scope/age/причина; accepted-only Connect/Stop, unknown без выдуманных RTT | F01/F02/F04; AND-08 |
| Подключения → Сценарии/Серверы | Рабочий bounded file import→preview→approval→commit; offline profile list; configured/observed exit раздельно | AND-04; затем AND-09/13 |
| Правила → Приложения/Сайты/LAN | Effective policy, precedence, supported/unsupported, controlled Apply и reconnect impact | AND-06; не включать DIRECT раньше admission |
| Система → Диагностика/История/Настройки | Нужное действие по typed issue, доступный redacted report, permissions/update/backup с реальными capabilities | AND-07/12/13/14 |

Home/notification/Tile/widget/shortcut должны использовать один state projection,
но не переносить command authority в старый snapshot. Widget остаётся historical
view; locked tap открывает UI, не воспроизводит Stop после unlock. Notification
permission denial — потеря поверхности, не автоматически отказ VPN. Share chooser,
widget pin и add-tile dispatch не доказательство доставки/размещения. Сохранить
existing ActivityResult UUID/generation и current displayed-report checks.

Следующий visual pass — размеры/типографика/длинные причины/keyboard/TalkBack/font
scale/insets/fold/saved scroll, light/system theme и reduced motion. Новая россыпь
карточек не требуется. Фиксированный600dp в source не доказывает adaptive acceptance.
Инструкции frontend skill применяются при реализации, а не вместо review данных.

<a id="workflows"></a>
### F08. Работа, поездки, quiet mode и временные разрешения

AND-09/10 и COM-01/06…11 уже планируют эти возможности. Первый deliverable —
сохранённый сценарий с выбранным профилем, supported policy и понятным explanation;
«готов к звонку» появляется только при объявленных dependencies/coverage/freshness,
не после одного HTTP endpoint и не через scraping рабочего аккаунта.

Quiet mode: явный deadline/cancel, задерживает только optional switch/probes/update;
mandatory safety/expiry/revoke/Stop сильнее. Default-route switch может оборвать
звонок; серверный anchor/bonding — отдельная R&D задача.

Portal: признаки → bounded диагностика → явный OS Settings/handoff либо принятый
узкий grant → revoke → новый proof. `startCaptivePortalApp` из сохранённого AOSP —
privileged/hidden, не public solution обычного VPN. Открытый Wi-Fi Settings не
подтверждает login. Нельзя добавлять общий pause/direct fallback или копировать
upstream portal detector как безопасную сеть целиком. Grant требует actor/scope,
network/profile generation, TTL и правила reboot/uncertain-clock/early revoke;
если такой scope недоказуем, честный handoff лучше неработающего «автоисправления».

<a id="diagnostics"></a>
### F09. Приватная диагностика и поддержка

`SessionJournal → immutable SessionHistorySnapshot → ControllerReportStore →
MainActivity.ACTION_SEND` уже даёт ring128, dedup, retained obs/recv/record times,
редактируемую frozen preview и exact displayed String. text-v2 ограничен32768 ASCII,
access TTL10min — срок доступа, не физическое стирание памяти и не recipient recall.
Clear не сбрасывает VPN/trust; новый observation context не называется worker restart.

Развивать COM-07/OFF-05/AND-08/12: отдельно observer unavailable, projection,
retention/eviction, delivery, export filter и authority scope. StateFlow conflation
не доказывает потерю событий ring. По неполной истории показывать несколько
совместимых причин и следующее безопасное действие; correlation ID не доказывает
причину/подлинность. Нет нужды в OTel SDK, новой БД или постоянной телеметрии для
этого шага. Не обещать полноту native history или delivered support ticket.

<a id="task-metrics"></a>
### F10. Полезное восстановление: отложенный отдельный контракт

Текущий journal регистрирует **принятые** команды и ограничен process/ring/Clear.
Он не census recovery requests и не наблюдатель бизнес-задачи. Ни Android, ни
просмотренный Windows state не дают task-level protection producer. Не выводить
«процент успешных задач» из CHECKING/coreReady/probe/Stop или сохранённых успехов.

Для будущих AND-09/15/17, COM-11 и FR-012 нужны:

1. До исходов фиксируются task goal, cohort membership, horizon D, повтор/новый
   episode, opt-out/cancel/resume semantics и цель измерения. Enrollment до command
   admission; requested/recorded/recording-unknown раздельно. Незаписанные намерения
   не становятся известным полным потоком.
2. Fixed enrolled N не зависит от success/terminal maturity. Mature определяется
   только доказанным t0+D≤cut; pending и unknown-time отдельны. Достоверный early
   completion может быть известен при ещё pending maturity. Нельзя объявлять
   проценты по исходам, сохранившимся после eviction; потерян census →
   DENOMINATOR_UNAVAILABLE.
3. Endpoint-protected требует receipt, покрывающего **весь допустимый интервал
   именованного completion event**. Sample в11 не покрывает event[10,12].
   Whole-task protected требует отдельного active interval и coverage по всему
   интервалу; сейчас SOURCE_NOT_AVAILABLE. Unknown не failure.
4. Различать original event time, arrival, report-as-of и revision. Late arrival
   достоверного события доD может уточнить новую revision того же N; исходный
   опубликованный snapshot не перезаписывается. Самоотчёт без time linkage не
   переименовывается в timely success. Позднее событие послеD не success-by-D.
5. Retention/dedup/membership bounded; потерянная связь не восстанавливается близким
   временем. Privacy Clear соблюдается без скрытого tombstone tracker. Stored schema
   — closed enums/counts, без package/URL/IP/SSID/аккаунта/стабильного device ID;
   hash домена не безопасный способ сохранить его.

**Разрешение разногласий C06/Z09:** предлагается использовать факторизованные axes
и только затем заранее выбранный endpoint. В C06 completion без **собственной**
policy (`A`) не равен глобально unprotected (`U`), возможен foreign VPN. Stop ack
не устанавливает ни A, ни U. Z09 system-ITT, где abandon считается0, — другой
показатель, чем latent task completion; применять его только с явным определением
«без abandon», не после известного completion. Q «full completion не установлен»
без отрицательного horizon evidence остаётся unknown, не known zero. Known timely
факт старого boot не стирается только из-за неизвестной связи текущего cut.

При sound exclusive labels и `N>0` возможна форма C06: `N=P+A+X+F+Z`, own-claim bounds
`[P/N,(P+X+Z)/N]`, any-route bounds `[(P+A+X)/N,(P+A+X+Z)/N]`. Здесь F требует
подтверждения goal-not-completed-by-D, Z включает неизвестный исход/срок; A —
доказанное отсутствие требуемой собственной policy, X — timely completion с
неизвестной policy. Это worst/best bounds registered cohort, не confidence interval,
не causal эффект и не population guarantee. Формулы Z09 не смешивать с этой
разметкой; его Q/C/U требуют отдельного endpoint/negative-evidence review.
При `N=0` публикуются только счётчики и отсутствие данных для доли.

**Решение:** минимальный recording protocol и выбор цели — proposal/needs review;
storage/pilot/публичные KPI **defer** до producers, privacy и пользы. Они не блокируют
первый безопасный клиент. R06-013 terminal-or-horizon maturity, early partial→F и
R06-011 перенос union bound как готовой гарантии отвергнуть. Новая статистическая
библиотека не чинит отсутствие наблюдателя.

<a id="recovery"></a>
### F11. Восстановление, устройства и небольшая инфраструктура

AND-13/COM-13/15/TH-07: offline kit — original signed profile + применимая trust
history/roles/device credentials + app/native compatibility + действующий способ
доставки/approval. Applicability привязана к exact version/epoch/envelope/trust
digests, не только слову «backup». Validity окна и prerequisite AND/OR описывать
в playbook; не нужен graph solver ради одного резервного пути. Clock floor не
даёт доверенной верхней UTC границы, backup не обновляет expired authority.

Начать с отдельного device credential, ручной даты paid-until с provenance,
read-only health/expiry и проверяемого restore-to-staging. В APK нет provider admin
token. Локальное удаление не server revoke. Два транспорта одного VPS разделяют
IP/ASN/оплату/диск/сертификаты; резервные байты не дополнительная серверная capacity.
Warm reserve должен иметь заранее доступные полномочия, а не зависеть от упавшего
issuer. Бюджет до12000₽/год — ограничение выбора, не свежая цена или текущая оплата.

Restic0.19.1/BSD-2-Clause годится как operator-side backup CLI после политики
repository credentials/locks; обычный restore не обязательно read-only. Он не
валидирует наш TUF/application trust. Cloudflare build shell/sshx не выделенный VPS.
Не размещать Prometheus/Grafana/control plane ради двух показателей малого gateway.

<a id="continuity"></a>
### F12. Переключение default и непрерывность задачи

Windows `switchChannelCore → Manager.SwitchChannel` и fallback `Reload` различны.
`cc.Validity.Constrain(oldCC.Validity)` **уже** удерживает deadline старых streams;
не заявлять missing validity guard. Новый default может сосуществовать со старыми
flows на старом канале; rollback default не откатывает внешнюю транзакцию. Native
interrupt_existing_connections не доказывает отсутствие dial, зарегистрированного
позже interruption. Android protect/bind/declaration аналогично не migration.

COM-09/AND-09/10: отдельно отображать default route, старые flows, task outcome и
proof freshness. Откладывать необязательный switch в явном quiet window; safety
выше quiet. Не повторять банковский POST/публикацию/отправку документа из VPN:
HTTP idempotency/application confirmation принадлежит приложению. Generic Retry
кнопка означает только разрешённую сетевую операцию, не гарантированно безопасный
повтор бизнес-действия. Bonding/server anchor оставить AND-18/COM-17.

<a id="proxy-ownership"></a>
### F13. Windows proxy ownership и корпоративный upstream

Actual callers: `App.startup → HealStaleSystemProxy`; `RunProxyHeal →` тот же helper;
`Manager.restoreProxy → ProxyManager.Restore`. Narrow regex fixed1080 и проверка
listener уже исключают другие порты/внешние/живые proxy; их сохранить. Они **не**
доказывают владельца совпадающего формата. TCP timeout не доказательство смерти
процесса. Между read/check и clear возможна внешняя смена настроек.

Конкретный delta по OFF-01/02/05, COM-06/07:

- Snapshot хранит presence/type/value/ошибку чтения, а не сводит missing/denied к0.
- Set сохраняет owned written state/generation, проверяет PAC removal и readback;
  partial write — отдельный результат с retained recovery context.
- Restore изменяет только поля, всё ещё совпадающие с собственным written state;
  external S2 не заменяется старым S0. Readback/conflict/unknown не объявляются healed.
- Automatic heal без достаточного own-write receipt/known-dead lifecycle не
  меняет глобальную настройку: typed conflict/unowned/unknown + явное scoped действие.
  Manual action также повторно проверяет scope/current values перед mutation.
- Один owner сериализует наши writes. Registry read-compare-write не атомарен с
  произвольным внешним writer; не обещать CAS/absence-of-race. При неопределённости
  прекратить automatic mutation, не эскалировать в принудительный сброс сети.

Эту правку можно делать независимо от Android SDK. Она не требует нового service
и не должна снимать WFP или менять текущий DNS. OS action receipt нужен отдельно.

Downstream HKCU SOCKS — не upstream authenticated proxy. Для корпоративного
P→E нужен capability contract: protocol/CONNECT target/auth/TLS/DNS/dial hop и
разрешённый OS actor. Existing core HTTP Basic/TCP не NTLM/Negotiate/PAC/UDP adapter;
`Builder.setHttpProxy` — рекомендация приложениям, не настройка libbox transport.
Cheapest useful release: typed unsupported/auth-required и approved IT handoff.
WinHTTP/другой connector — отдельный compatibility slice по подтверждённому кейсу,
без автоматической отправки OS credentials обнаруженному proxy и бесконечных407.

<a id="reuse"></a>
### F14. Готовые реализации, совместимость и цена

Фиксированные snapshots и лицензии — [Android reuse §12.7](../../../theory/android/PLATFORM-AND-UX.md),
[R06 sources](../../../theory/research/06-RELIABILITY-UX/SOURCES.md) и
[Z08 sources](../../../theory/research/cycles/2026-10-28-contracts/Z08/SOURCES.md).
Это выбранные файлы/callers, не полный security/transitive licence audit.

| Материал | Решение и цена интеграции | Gate/несовместимость |
|---|---|---|
| sing-box1.13.19, b5ebaa1; GPL-3.0-or-later | **Reuse whole core**, текущая adapted sourceRevision3; высокая native/packaging цена, экономит повторное packet engine | Exact AAR/API/hash/NDK/gomobile/ABI/JNI и corresponding source/notices; stock AAR не имеет наших6 marker/override APIs |
| SFA bc73504; GPL-3.0-or-later | **Adapt/reference** Android bridge ordering; текущие fences сильнее просмотренных upstream patterns | HEAD DNS/notification API отличается; не переносить ignored protect, allowBypass, unconditional unmetered и continue-on-missing-package |
| Android APIs/AOSP; Apache-2.0 source references | **Reuse public primitives**, Settings/ActivityResult/Tile/notification/Network callbacks; низкая цена относительно собственного OS слоя | API/user/profile/permissions/OEM scope; hidden APIs не приложение capability |
| Now in Android2982e63; Apache-2.0 | **Adapt patterns** adaptive navigation/lifecycle, существующие Compose компоненты прежде копирования | Не тащить весь news graph/analytics/HEAD Navigation3; notices и pinned AndroidX совместимость |
| Mullvad3d6146a, Proton316a3d4, Amnezia02838a9; GPL family | **Adapt UX** lockdown explanation/task profiles/app selection/self-host onboarding | Не копировать accounts/billing/vendor API/Qt/Rust engines; AWG не вытекает из libbox pin |
| ZXing cc70a9d; Apache-2.0 | **Reuse decoder** за bounded adapter после file-import MVP; средняя camera/image/permission цена | Decode не trust; oversized/malformed/denied camera→file, no plaintext shortcut |
| Existing go-tuf workflow + profile SDK request | **Reuse**, не второй verifier | Portable side-effect-free exact-byte API/clock/progress accepted ещё не поставлен; Android integration в одном whole AAR |
| Tailscale47743…, BSD-3-Clause | **Adapt ideas** net events/portal constraints | Прочитанный upstream Go1.27.3 не drop-in в проект1.26; detector endpoints/direct dialer не наш protected probe |
| Restic0.19.1/BSD-2-Clause | **Operator tool**, optional restore-to-staging | Lock/write/delete permissions; не runtime dependency APK и не trust verifier |
| OONI Probe CLI3.31.0/GPL-3.0 | **Reference** explicit save/submit privacy boundary; код не нужен | CLI upload stub/измерения не доказывают task census/protected completion; не запускать из приложения автоматически |
| Hiddify f26aadb Extended GPLv3; Outline unpinned; v2rayNG118cf5b | Hiddify/Outline **reference only**; v2rayNG routing reference; новое ядро **defer** | Hiddify дополнительные условия, Outline approved source pin отсутствует, Xray AAR не libbox |

Нет причин вводить новую БД, telemetry backend, второй controller/Go AAR, general
graph planner или статистическую runtime dependency ради этого плана. Library
licence compatibility и стоимость adoption оцениваются на фактически копируемых
компонентах; таблица не разрешает blanket copy репозитория.

## 3. Последовательность разработки и владельцы компонентов

Оценки стоимости относительные: низкая/средняя/высокая, не сроки и не экономия
токенов. Независимые deliverables можно вести параллельно без общей переделки API.

| Этап | Конкретный deliverable и owner кода | Зависимости / existing IDs / стоимость | Выход и будущая приёмка |
|---|---|---|---|
| A — безопасная основа | Android: approved adapted whole AAR + принятая trust API, driver, accepted producer/renderer, оригинальные authority deadlines | AND-01…04/14, A-FR-01…03/13; высокая. Trust owner принимает SDK; Android adapter не подменяет его | Import file→точный preview→approval→commit→valid source; rejection/cancel/expiry/replay и cleanup. **NOT_RUN** |
| B — поддерживаемый connect | Android: согласованный OS-mode/cold-recovery contract, тот же service/controller/worker, fresh scoped protected producer | A + AND-05/07/15, A-FR-02/04/06; высокая. M01…06 requested, не accepted proof | Consent/revoke/Stop/worker death/network change; no fake Ready/direct fallback. Full-capture MVP, unsupported policy явно. **NOT_RUN** |
| W1 — Windows безопасное восстановление | Windows backend: own proxy receipt/conflict/partial write/readback; UI: NetGuard neutral facts и доступное действие | F13/F04; OFF-01/02/05/08, COM-06/07; средняя; независимо от A | S0→ownS1→foreignS2→Restore сохраняетS2; unowned1080/no-access/timeout/partialPAC не healed. OS **NOT_RUN** |
| C — честный ежедневный интерфейс | Android app/Windows views: четыре axes/status/action, existing navigation/components, file onboarding, diagnostics/report; fast surfaces shared projection | A/B для enabling; тексты/a11y можно отдельно; AND-08/12, COM-01/02/07; средняя | First run/offline/denied/unavailable/stale/controller failure, keyboard/TalkBack/font/DPI/size, exact report share. **NOT_RUN** |
| D — реальная policy и сети | Android policy/corebridge + Windows render/network: UID partial resolver, supported route/DNS/LAN scope, scoped bootstrap cache/family/proxy capability | B; AND-05/06/15, COM-04/05/06, A-FR-05; высокая | API29–36/SDK sandbox/shared UID/churn/new+existing TCP/UDP/DNS/IPv6/NAT64; fail closed only in declared scope. **NOT_RUN** |
| E — домашние и рабочие сценарии | UI/controller: presets, explicit quiet window, service explain/readiness; затем временные grants/portal handoff | D + measurement/budgets; AND-09/10/11, COM-08…12; средняя/высокая | Неполный dependency→unknown; safety выше quiet; expiry/network/reboot revoke; task не бизнес-retry. **NOT_RUN** |
| F — перенос и поддержка | Android profile/infra + Windows onboarding: per-device credential, encrypted recovery packet, staged restore, explicit backup/update and server facts | A + accepted server semantics; AND-13/14/17, COM-13…15; высокая | Key loss/expired authority/bad signer/low disk/issuer outage; локальное delete≠revoke. **NOT_RUN** |
| G — доказательства качества | Platform/QA: fault matrix и scoped release сначала; optional task recording protocol/pilot после privacy/producer review; comparative Россия после оплаченной инфраструктуры | AND-15…17/FR-012, F10; высокая; не препятствует C до публичных claims | Versioned source/native/device/live/comparative evidence отдельно; no all-user rate/universal bypass. **NOT_RUN** |

Кратчайший путь к usable Android: **A→B→C**, full capture с объявленными пределами,
затем **D→E**. W1 независим. F зависит от настоящего provisioning/trust; наличие
backup файла не делает F завершённым. G начинается с критериев ещё до реализации,
но выполнение gates требует отдельного разрешения и среды. AND-18/COM-17…19 не
стоят перед этими этапами.

## 4. Минимальные договорённости между компонентами

| Граница | Достаточный вход/выход | Что не переносится через неё |
|---|---|---|
| Existing T01…14 trust → Android adapter | Exact bytes/current trust history/approval+preview fence/clock → rejected либо owned accepted/progress с authority bound | SDK не запускает VPN/Settings/store; decrypt/CAS/constructor не proof; cancelled call удерживается до actual return |
| Accepted renderer/source → worker | Configuration/policy digest, original network/application lease, preparation и signed authority deadlines, advertised mode/capabilities | UI selection не acceptance; new worker не renewal; unavailable OS mode не скрытый downgrade |
| Existing M01…06 measurement → presentation/controller | Named protected observation scope/generation/network epoch/original time+coverage/typed uncertainty/budget result | Bootstrap transport socket/HTTP200/core started не protected receipt; API1 recommendation не authority/apply |
| UID callback → native predicates | Raw UID + partial attributed UID/mapping basis/API/user + observed package/signers/lease validity | SDK sandbox mapping не произвольная isolated UID inference; metadata cache не capability; stored identity не вечное разрешение |
| Repository → UI/fast surfaces | Existing immutable state с четырьмя независимыми axes, availability/freshness и permitted actions | Locked/stale tile/widget не command token; dispatch/admission не OS effect; worker facts не task result |
| Windows proxy owner → heal/UI | Read observation с presence/type/error, own-written state/generation, lifecycle evidence → applied/conflict/unknown + readback | Формат1080/отрицательный connect не владение; локальный mutex не external atomic CAS; PAC/чужая policy не наша настройка |
| Journal → report | Frozen immutable source + options → exact displayed redacted revision | Report не complete native log/census/signed evidence; share chooser не delivered receipt |
| Optional episode producer → aggregate | Fixed enrollment/goal/D/clock linkage, task evidence interval, claim coverage, report revision; отдельный r0/R для ресурсов | Никакого join по времени, скрытой записи после Clear, переноса listener/Ready в task success |

Эти договорённости уточняют существующие компоненты и requests. Из них не следует
создание одноимённых DTO/shared packages; сначала принять нужный seam, затем менять
его владельца и callers совместно. API1 пока draft/no consumers; Android FFI или
новый общий слой ради формального переиспользования преждевременны.

## 5. Конечные критерии и открытые gates

Все следующие пункты — **NOT_RUN**. Это будущая приёмка существующих требований,
не добавленные/запущенные тесты и не новые результаты сети.

| Сценарий | Проверяемый результат | Уровень |
|---|---|---|
| S1 cold/offline/update | Original-byte valid profile/authority; first unlock/CE/clock uncertainty различаются; bad signer не требует uninstall/снятия защиты | Source/local → artifact/native → device |
| S2 звонок/Wi-Fi↔LTE/sleep | Одна transition lineage; old streams/default/task раздельны; delayed old result не Ready; Stop/revoke выше quiet | Native/device, внешний capture; live для gateway |
| S3 portal | Timeout не диагноз; public Settings handoff/узкий принятый grant; выход из сценария отзывает grant и требует новый proof | Device/OS; controlled network |
| S4 банк/принтер/корпоративная сеть | Effective capture/DNS/routes объяснимы; unknown UID не direct; sandbox API matrix; чужая MDM/VPN/proxy policy не перезаписывается | Native/device/Windows OS |
| S5 VPS/issuer/payment failure | Paid/capacity/path отдельно; kit applicability и резервные authority/delivery; expired objects не продлеваются restore/reconnect | Local/OS → explicit paid live |
| Неполная диагностика | Eviction/Clear/late reply/no-access не причинный диагноз и не100%; export exact preview, no-secret closed fields | Source/local → platform UI/share |
| Ресурсы | Actual-return/death/source-idle custody сохраняется под timeout; late release не task success; измерены active/retained/RSS/FD/bytes/wakeups | Native/device/измеренный workload |
| Optional task metric | FixedN; earlypartial≠failure; known completion/unknown policy→X; point11≠coverage[10,12]; Stop/late/as-of/dedup/Clear не меняют знаменатель задним числом | Contract review → разрешённый local pilot; не population inference |

Открытые gates: portable trust/accepted source, approved adapted AAR/toolchain,
OS mode/cold recovery, protection-measurement producer, native policy parity и
existing-flow enforcement, controlled OS/device/a11y, paid live и comparative.
UI skeleton/source patch может быть закончен раньше gate и всё равно не даёт права
включать недоступный Start/strict badge или заявлять работоспособность VPN.

## 6. Что исключить из ближайшего объёма

- Общий pause/direct fallback, снятие lockdown/WFP ради автоисправления, скрытый
  takeover чужого VPN и массовый сброс proxy/DNS.
- Blanket утверждения R06-010 о запрете любого LAN/корпоративного VPN и per-app
  настройках только через MDM; исправления R06-012 имеют приоритет.
- Повторный strict-owner cache bypass, bounded reader, journal или отдельный
  reconnect controller при уже существующих владельцах.
- Public «процент защищённых задач», task census из ring и статистическая гарантия
  из неподтверждённых producers/union bounds; survey/аналитика не первый релиз.
- Новое ядро, AWG/XHTTP/ECH из HEAD docs, bonding/DAITA/PQ/port forwarding без
  конкретного use case, обоих концов и compatibility/resource gates.
- Новый UI framework, dashboard из KPI/карточек и второй navigator; текущая
  иерархия меню→дочерние tabs уже подходит запросу владельца.

Критерий полезности следующего изменения: оно закрывает конкретный existing AND/
COM/OFF пункт, сохраняет текущий guard и даёт наблюдаемое действие/результат на своей
платформе. Непроверенный остаток остаётся явно открытым.
