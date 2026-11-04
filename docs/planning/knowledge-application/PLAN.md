# Применение знаний: единый план разработки

28 октября 2026. Редакция KA-1. Это сводная последовательность задач для
существующих владельцев; она не подтверждает реализацию или работу VPN.
Статусы требований остаются в [ПК-плане](../../PLAN.md),
[Android-плане](../../ANDROID-PLAN.md) и [adaptive-плане](../../adaptive/PLAN.md).
Владельцы добавляют туда свои конкретные срезы; этот документ связывает их,
не копирует журналы и не закрывает требования за другую платформу.

Дальнейший порядок работ от 4 ноября собран в [плане ПК](PC-PLAN.md) и
[плане Android](ANDROID-PLAN.md). KA-1 сохраняет общие контракты и исходные
решения; сведения о коде и сервере ниже относятся к указанному срезу 28 октября,
а не к текущей аттестации узла или приложения.

## 1. Основание и ограничения

- Получены все14 application-сдач и собственный раздел руководителя. Основные
  назначения содержат disposition для2714 initial+late units. Это полнота
  сопоставления знаний с кодом/планами, не независимая научная проверка каждого
  утверждения. Z02 и Z09 завершили независимые сверки как PARTIAL; индексирование
  Z09 не засчитано за прочитанные bodies. Основной Android/UX разбор — C06.
- Уточнения и устранённые противоречия: [RECONCILIATION](RECONCILIATION.md).
  Непринятые выводы, условия модели и отложенные идеи не становятся обязательными
  runtime requirements. Анализировать весь корпус не означает внедрять всё.
- Source baseline: Windows V2-188 и Android AND-06-S8 с сохранением прежних
  storage/lifecycle/consent/UID/UI guards. Это выборочно прочитанные текущие seams,
  не whole-tree, compile, race, OS, device или live proof. Перед своим изменением
  владелец сверяет actual source и свой STATE; более новые срезы не откатываются.
- Владелец сообщает о покупке нового VPS. Доступ/host identity/OS/сервисы ещё
  не подтверждены. Старый inventory не наследуется. Имя заказа не является
  купленным доменом. Один сервер остаётся общим отказом для всех его каналов.
- Happ на рабочем ПК остаётся включённым: текущая связь от него зависит.
  Локальные TUN/proxy/DNS/routes/firewall и работающие процессы не меняются.
  Измерение через Happ и внешний RF-замер имеют разные path scope.
- Сохраняется plan-only: сейчас никаких tests/fixtures/build/vet/doctor,
  simulations/benchmarks/crypto/security scans/native/device/live/admin/install,
  SSH/VPS/Cloudflare/deploy или изменений сети. Ни один будущий gate ниже не
  разрешает его выполнение. Секреты/реальные профили/пользовательские логи не читают.

## 2. Владельцы и минимальные связи

| Владелец | Ответственность | Что передаёт |
|---|---|---|
| ПК | Windows source/backend/frontend/helper, platform storage/protection/release | Версионированные факты/ограничения и конкретный contract request |
| Android | Весь android/: app/profile/vpn/native/JNI/packaging | T/M request, принятая artifact/API identity, platform outcomes |
| SDK доверия | После регистрации: переносимая полная trust semantics/value API | Точная версия API/source, supported limits, proof и error semantics |
| Adaptive | Только shared/adaptive и docs/adaptive: pure bounded policy | Версия входов/решений/причин; без measurement/OS execution |
| Серверный ZCode | Выбранный владельцем исполнитель нового VPS | Capability/config/apply/recovery facts; не client/publisher authority |
| Обвязка | Только необходимые ownership/registry prerequisites | Зарегистрированный scope, без исполнения и ослабления policy |

`trust`/`infra` ещё отсутствуют в фактически прочитанном реестре. SDK не начинает
создание общего пакета до регистрации. Обвязке нужен отдельный bounded prerequisite
для trust по существующему proposal, с сохранением всех roles/bindings и plan-only.
Серверный исполнитель теперь в ZCode: старому Codex infra-чату автоматически
владение не передаётся. Для записи configs нужен согласованный серверный scope.
До его определения изменения готовятся как предложения.

Новых БД, Android2/3, core-чата, параллельного controller или дополнительных
постоянных координаторов не требуется. Интеграцию делает владелец компонента.
Платформенные чаты не переписывают общий SDK/обвязку и чужие STATE/PLAN.

## 3. Минимальные договорённости

Это смысловые обязательства существующих seams, не требование создать новые DTO,
таблицы, подписи или сервисы. Сначала таблица полей и совместимость текущих API.

| Контракт | Вход → выход | Обязательная граница |
|---|---|---|
| C1 Trust/admission | Исходные bytes, independently approved root+first policy, accepted history, reference time → подготовленный результат и per-role prefix/error | Новый input/pointer не accepted predecessor; commit и OS — платформа. Per-return/per-step persistence выбирают явно |
| C2 Artifact/API | Source/pins/patches/toolchain/tags/ABI/licences → whole-AAR identity и generated API inventory | Один Go runtime в сборке; новый immutable revision требует нового одобрения. Marker/hash не runtime proof |
| C3 Channel capability | Принятый профиль+endpoint/port/TLS/auth/bootstrap и server tuple → допустимая попытка | Confidentiality/endpoint authorization до sensitive payload; unsupported/unknown не direct fallback |
| C4 Observation/policy | Context/owner/generation, method/workload, clock domain/unit/boot/age, known/unknown → policy recommendation+reason+budget | Known0 отдельно от unknown; health projection отдельно от cohort; final execution recheck у платформы |
| C5 Custody/retirement | Original owner, original deadline, borrowed bytes/resources → actual return/death/last-holder facts | Timeout/cancel/UI Stop не возврат ресурса. Session cancellation отдельно от whole-carrier retirement |
| C6 UI/action | Scoped applied/path/action facts и freshness → существующая presentation | Dispatch/installed/running/probe/whole-task не смешиваются; unknown не zero/Ready; ручное отключение отдельно от failure |
| C7 Server apply | Exact rendered/config/binary identity → staged/checked/applied/restarted/path/cleanup/rollback outcome | Все outcomes отдельно. Unknown/failed rollback блокирует следующий apply; receipt не доказывает сверх своего scope |

В Android T01…14 и M01…06 остаются существующими requests. Нужный measurement plan
может уже находиться в принятом T-output: второй метод не обязателен. API1 adaptive
остаётся draft до consumer mapping. Новые ABI/enums/Wails bindings не вводятся
случайно. Root S→S′→S и actual retry-schedule token не заменяются простым индексом.

## 4. Этапы и выходные критерии

### KA-00. Зафиксировать совместимость и открыть только нужный ownership

**Владельцы:** руководитель, обвязка, SDK/adaptive и два платформенных владельца.
**Вход:** этот план, current STATE/request и существующие registry/proposal.

Согласовать C1…C7 только для ближайшего потребителя. Зарегистрировать trust через
владельца обвязки; SDK затем делает собственный bind. Не менять execution policy,
allowedScopes, dependencies, существующие bindings или platform code. Infra actor
уточняется отдельно для ZCode. Не ждать всех TH/AND/native доказательств для
маленькой таблицы semantic compatibility.

**Выход:** поддержанные версии/лимиты/ошибки/clock semantics и один owner каждого
seam. Отсутствие регистрации блокирует только соответствующие repo writes.

### KA-01. Независимые P0 платформ

**ПК, последовательно в одном чате:**

1. **W1 / TH-01, FR-006:** исключить нынешнюю VLESS-over-ShadowTLS композицию без
   внутренней конфиденциальности из protected eligibility до чтения/отправки
   sensitive credentials/user payload. Сохранить signed-profile history и другие
   допустимые каналы. Не чинить это добавлением непроверенного TLS слоя наугад.
2. **W2 / TH-01/03, OFF-06:** eligible per-role progress вместо принятия pointer
   assignment за predecessor; сохранить старую accepted expired history, lawful
   prefix и fixed4/no-op/CAS/source fences. Timestamp criterion review отдельно
   от durability и complete application acceptance. SDK snapshot checks остаются.
3. **W3 / TH-08, FR-009…011, COM-08:** own-write proxy receipt/readback/conflict;
   presence/type/error до восстановления. Согласовать startup/manual/UserMode
   writer. Не применять reset/heal по port/listener heuristic. NetGuard/SYSTEM
   task/DNS healing не выдаются за protection; foreign policy не перетирается.
4. **W4 / TH-01/08:** отделить probe-only admission от user forwarding. Сохранить
   current operation/process/probe/cleanup guards; перестановка SetProxy сама по
   себе не закрывает TUN routes. Credentials endpoint авторизуется до handshake.

**Android, параллельно source-подготовка:** сохранять AND-02/03/05/06/11 guards,
последние package-manager step checkpoints и единый actual-return owner; закрывать
следующий доступный P0 по [source-readiness](../../android/SOURCE-READINESS-2026-10-28.md).
Не писать fake trust driver/accepted source и не включать Connect по marker.

**Выход:** source deltas с конкретным blocked/unknown/retained outcome, code review
и будущей конечной приёмкой. Tests/native/OS gates всё ещё NOT_RUN. Ни один из
этих source-срезов не ждёт оплаченного VPS, Android release или нового estimator.

### KA-02. Полный trust workflow и переносимый SDK

**Владельцы:** SDK после KA-00; ПК сохраняет current wrappers/storage; Android —
свои clock/Keystore/JNI/coordinator/provider.

- Переиспользовать value policy/codecs и полный pinned go-tuf workflow. V2-187
  уже содержит Windows-local `internal/tufupdater.Storage` и bounded memory cache;
  это существующий кандидат для изучения/извлечения, не готовый shared API.
- До переноса сравнить upstream delta/notice, root/role/delegation/expiry/version/
  hash behavior, sticky faults, genuine miss, readback и cancellation/proof fences.
  DisableLocalCache/signature-only replacement не принимается.
- Явно выбрать per-return preparation либо per-step platform persistence barrier.
  Сохранить original raw/canonical fields, full uint64, private proof/source binding,
  authority deadlines, epochs/bans и restriction-only progress.
- Разрешить first-pin approval отдельно от incoming bundle. Согласовать supported
  Windows/Android size intersection; Android256KiB не увеличивать молча. Unsupported
  Windows executable/split-direct semantics Android отклоняет явно.
- V2-183…186 capture/no-op/selected I/O/quarantine/commit facts сохраняются.
  Cleanup warning не rollback, quarantine не здоровое состояние. SDK internal
  I/O/full behavior/directory-powerloss/restart остаются проверочными gates.
- V2-188 уже ограничивает существующий update acceptance/recovery-check worker
  начальной Job policy. Она ещё не измерена и не равна RSS/hard wall deadline.
  Profile/pin/export SDK calls пока не получают такой process/private-proof bridge;
  это отдельная сохраняемая граница, не повод создать второй verifier.

**Выход:** принятая callable версия T API + source identity/лицензии/лимиты/error
contract и первый consumer migration map. Windows internal package не импортируется
Android/shared напрямую. Общий verifier не становится владельцем OS/commit.

### KA-03. Один VPS: безопасный доступ и воспроизводимая поставка

**Владелец:** серверный ZCode. Offline design может идти параллельно KA-01/02.
Remote этап требует переданного доступа, independently verified host identity и
отдельного разрешения в серверном чате. Private SSH key/password не передают чатам.
Сохраняют второй проверенный key-login и provider console до закрытия старого входа.

Первый node capability — exact-version WS/nginx и direct HY2, если реально
согласованы client/TLS/domain/auth/UDP prerequisites. Настоящий domain/certificate
identity решается отдельно от hostname заказа. REALITY позже; ShadowTLS protected
и hopping выключены до их собственных gates. Серверный direct egress не client
direct fallback. `/range` необязателен; probe contract берётся из accepted plan.

Исправить `scp -P` отдельно от `ssh -p`, JSON rendering вместо secret sed,
private staging/check до overwrite, bounded known-good backup, atomic same-filesystem
apply, ручную активацию и отдельные restart/path/cleanup/rollback outcomes.
Recovery kit вне узла, без publisher/root private signing keys и сброса client trust.

**Выход:** compatibility tuple и rollback/runbook; затем разрешённый staging/restore
drill и лишь потом live activation. ICMP/TCP22 одной RF-ноды, RDAP/страна/ASN —
разные сведения, не country-wide/VPN/UDP proof. Один origin остаётся SPOF.

### KA-04. Первый пригодный Android path и настоящая platform acceptance

**Вход:** Android принял T API/accepted source; exact native source closure.
Один Android-владелец соединяет revision3 overlays/wiring, весь libbox/JNI/driver/
provider/service. Техническая source/build recipe готовится раньше; первая
промежуточная сборка не включает Connect и не становится approved runtime.

После отдельного разрешения: whole-AAR build → generated Java/JNI/AIDL inventory
→ independent approved digest → ABI/16KiB/toolchain/licence/SBOM → app packaging.
Сохранить borrowed input lifetime и both-RPC/source-idle/actual-process-death
retirement. Проверить original authority deadline, independent bootstrap approval,
accepted-only source, OS consent/FGS/revoke/cold recovery/lockdown. Always-on/direct
boot/START_STICKY не включать до принятой cold recovery.

**Выход:** на разрешённом устройстве доказан конкретный protected path и Stop/
revoke/recovery scope. APK install/VPN icon/port/probe не whole-task proof.
ПК проходит собственные OS/helper/protection gates независимо от Android.

### KA-05. Ограниченные ресурсы и один adaptive consumer

**Владельцы:** существующие платформенные owners + pure adaptive policy.

Сначала count+bytes admission до clone/spawn, cache borrower lifetime, blocked
writes/reassembly/history и actual last-holder return; отдельные cancellation
session/carrier. Narrow Brutal dispatch fix — ограниченный результат, не весь
CC/resource repair. Совместимую quic-go/sing-quic пару принимать отдельной миграцией,
без редактирования module cache и автоматического обновления dependencies.

Использовать реальные protected observations/pressure от той же admitted source.
Первый consumer выбирается один: optional background suppression с coherent
pressure/final platform recheck либо отдельно обоснованный ranking cutover.
Policy рекомендует; платформа исполняет. Deterministic Stop/revoke/recovery не
ждёт statistical model. Latest health и complete cohort accounting раздельны.

**Выход:** один наблюдаемый bounded change с причиной и fallback к допустимому
поведению, original budget/custody. Полезность/ресурсные выигрыши измеряются позже.
HY2 pause/wakelock/close-on-every-resume не добавляются без reached caller/device
evidence. Battery/RSS/CPU/FD/wakeup/SLO пока неизвестны, не равны нулю.

### KA-06. Повседневный UX и совместимость рабочих сетей

Переиспользовать существующие NavRail/child tabs/tray/Compose/diagnostics/report;
меню — разделы, табы — их подпункты. AND-08-S4/S5/S6 не переделывать повторно.
Пройти каждую страницу: Home/Connect, профиль/onboarding, каналы/серверы,
rules/apps/sites/LAN, diagnostics/history/settings. Состояния loading/empty/
blocked/error/unknown, readable explanations, keyboard/focus, constrained viewport,
font scale и accessibility имеют отдельную будущую приёмку.

Восемь бытовых flow: первый профиль/Start, cold/offline-valid запуск, handover/
sleep, portal, LAN/internal DNS, corporate proxy/PAC/MDM/другой VPN, отказ VPS и
ручной support export. Одно доступное действие на проблему, без protocol/MTU
knobs по умолчанию и Repair-all. Scoped LAN/quiet/temporary grants требуют
authority/expiry/owner; safety/revoke выше quiet. Нет automatic protection-off.
UID classification/SDK sandbox/shared UID, DNS/IPv6/NAT64 относятся к actual hop
и full effective policy; один visible package не доказывает полный capture.

**Выход:** page/flow acceptance отдельно от native/network proof. Task и whole-task
protection остаются unknown без admitted producer и interval coverage. Публичный
процент успешных задач, fixed-cohort store и analytics SDK пока не внедряются.

### KA-07. Автономность, устройства и поставка

Развивать существующие signed provisioning/revoke/backup/recovery/installer
seams. Per-device enrollment/revoke вместо общего секрета; transfer с preview,
independent approval и сохранением history/floors. Серверный runtime restore,
publisher restore и client accepted-state recovery — разные процедуры.
Release bytes → installed → executing → adopted → readiness проверяются отдельно.
Windows schema2 и Android package/signing lineage/update channel остаются gates.
Privacy до записи: bounded typed local diagnostics, redaction, exact manual frozen
export, no automatic raw upload и без новой diagnostics DB ради метрики.

**Выход:** воспроизводимые update/recovery/support сценарии без reset trust,
доступный offline recovery kit и ограниченная canary/support стратегия.

### KA-08. Разрешённые доказательства и выпуск

Все строки ниже сейчас **NOT_RUN**. Последовательность разрешений/доказательств:

| Уровень | Что доказывает | Что не закрывает |
|---|---|---|
| Source/contracts | Exact inputs, decision paths, ownership, bounds, upstream delta | Компиляцию, races, OS, durability, protected path |
| Contract/local checks | Конкретные normal/failure/late/partial cases на неизменных inputs | Real kernel/device/remote path |
| Native/OS | Реальные file/ACL/reparse/interprocess/powerloss/helper/WFP/proxy/FD/IPC outcomes | Все ISP/OEM/приложения |
| Device | Exact API/OEM/patch/Doze/reboot/force-stop/lockdown/Back/a11y scenarios | Всю РФ или превосходство |
| Live/field | Named endpoint/protocol/path/time/ASN и workload | Причину timeout и универсальную доступность |
| Comparative/canary | Заранее заданные workloads, сопоставимые бюджеты/версии, distributions и rollback | «Лучший» без ограниченной области сравнения |

До замеров объявить endpoints/horizons/coverage, success/failure/partial/missing/
pending/manual-unprotected, original clocks/age и resource остатки. Pending не
исключать из denominator, endpoint не whole-task, поздний успех не timely outcome.
Нет обещанных процентов скорости/отказов/энергии или RTO до измерений.

## 5. Что именно брать готовым

| Основа | Применение и стоимость |
|---|---|
| Платформенные DPAPI/Keystore/filetxn и текущие guards | Сохранить конкретную custody/commit/cleanup модель; не выдавать source за OS durability |
| go-tuf/v2 v2.4.2 + V2-187 adaptation | Apache-2.0 с origin/change notice/license; review полного workflow delta, ownership/limits/parity. Не signature-only fork |
| libbox/sing-box1.13.19 candidate + revision3 overlays | Один whole artifact, exact pins/JNI/toolchain; GPL obligations/corresponding source/licences/SBOM и upkeep patches. Не stock AAR по одному marker |
| quic-go/sing-quic compatible pair | Отдельная compatibility/resource/callback migration и измерения; не blind upgrade или частичное объявление полного fix |
| Existing netmon/lifecycle/network observers | Встроить в current TUN context/owner seams; не новый monitor/HEAD poller |
| Existing Vue/Wails/Compose/navigation/report/tray | Доработать страницы/доступность/факты; не новый UI framework/store/router |
| Existing nginx/sing-box templates и scripts | Исправить parity и transactional rollout; exact version/licence/secret custody, не копировать исторический runtime |
| Competitor/OONI/SFA references | Переиспользовать обоснованные patterns; whole app, telemetry submit и внешний control-plane не переносить |

Точные аргументы, source pins и исключения остаются в тематических разделах;
нет отдельной непроверенной лицензии для ещё не созданного shared пакета.

## 6. Покрытие знаний и отложенные решения

| Раздел применения | Где применяется |
|---|---|
| [NETWORK N00…N12](sections/NETWORK.md) | KA-00/01/03/05/06/08; confidentiality, bootstrap, hop/probe scope и independence |
| [ADAPTATION](sections/ADAPTATION.md) | C4/C5, KA-05/08; bounded policy, missingness/cohort, reference clocks и eligibility |
| [PERFORMANCE F00…F15](sections/PERFORMANCE.md) | KA-01/02/03/04/05/08; exact resource owners и compatible upstream migration |
| [TRUST K1…K11](sections/TRUST.md) | KA-00/01/02/04/07/08; raw history/progress/custody/time/release |
| [PRODUCT-ANDROID F00…F14](sections/PRODUCT-ANDROID.md) | KA-01/04/06/07/08; platform/UX/workplace/recovery/privacy |
| [ARCHITECTURE-OPERATIONS](sections/ARCHITECTURE-OPERATIONS.md) | Owners/C1…C7 и KA-00…08; one-node economy и существующие seams |

Unit-level решения сохранены в private coverage с source hashes; три root mixed
unit corrections и шесть lead overlays исправляют конкретные mislinks/omissions.
Дубли с разным контекстом не схлопнуты. Unique архивы/источники сохранены.
Два reader summaries имеют lineage к уже назначенным units, не новые научные claims.

Отложены: ECH/Vision/Naive/new core/shaping/bonding/ML/DAITA, probabilistic optimizer
без модели/producer bridge, новые attestation/transparency/multirepository функции,
общая БД/кластер и task-completion telemetry. Возврат — только при конкретном
use case, актуальной совместимости/лицензии, privacy/resource budget и отдельной
проверке. Статусы NEEDS_REVISION и assumptions не заменены обязательством внедрить.

## 7. Как продолжать без «сломанного телефона»

Первый handoff — только существующему владельцу и один ограниченный срез:
ПК W1, Android current source-readiness/T-contract seam, SDK review/extraction
после регистрации, adaptive один semantic consumer contract, обвязке trust
prerequisite. Активный срез сохраняется; модель/runtime/checkout не меняются.
Серверному ZCode передаётся отдельный подготовленный access/operations prompt,
его доставка/remote authority не выдумываются. Старый Codex infra не запускается.

Дальше владелец читает свой STATE, этот этап и нужный request; фиксирует actual
code delta и следующий finite gate. Общение через опубликованные версии/request,
а не цепочку свободных пересказов. Адресная отправка задания не proof реализации.
Редакционная очистка идёт отдельными private patch proposals; она не блокирует
этот план, пока не выявлен содержательный preservation/conflict issue.
