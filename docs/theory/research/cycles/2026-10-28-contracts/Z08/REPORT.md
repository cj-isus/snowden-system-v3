# R28-Z08 — минимальный UID mapping/invalidation contract

28 октября 2026.
**Авторский статус:** READY_FOR_REVIEW. Предлагаемые решения ниже относятся
к source/API/model scope; не ACCEPTED и не product/native/live proof.

## 1. Короткий ответ и новая дельта

Минимальное публично поддержанное решение — **частичное**, а не универсальное
отображение каждого raw UID в имя приложения:

1. Получить raw socket UID допустимым `ConnectivityManager.getConnectionOwnerUid`
   для исходного TCP/UDP tuple и своего активного VpnService scope.
2. Для regular application UID своего user сохранить identity mapping. Для SDK
   sandbox на API35+ использовать public `Process.isSdkSandboxUid` и
   `getAppUidForSdkSandboxUid`. Для остальных неподдержанных случаев вернуть
   unknown/unsupported — без hidden API, арифметического offset или first-package.
3. Сопоставить **attributed UID**, выбранный package, current установленный
   signer-set и observed installation material с original application lease;
   raw UID, тип mapping и provenance не терять. Shared UID и неизвестная видимость
   не превращаются в single-package approval.
4. В admission seam обойти либо invalidation-fence **owner cache** и проверить
   original session/epoch до и после potentially blocking lookup. Никакая
   result metadata не применяется после наблюдённого invalidation этой lease.
5. Уже допущенные потоки имеют иной lifetime: прекращение новых допусков,
   retirement владельца, actual worker death и завершение parent producers/RPC
   должны быть отдельными receipts. Flush/TTL/mapping не удостоверяют их сами.

**Новая дельта к R06-012-D:** current repository уже содержит source-кандидат
strict-owner adaptation, обходящий 5-tuple process cache при tagged TUN admission.
Поэтому прежний cached-tuple trace не доказывает stale admission в этом пути.
Это ещё **не** реализованный public sandbox mapper: Kotlin callback по-прежнему
передаёт raw UID и PackageManager result. Persistent `SessionCacheRevision` —
другой механизм; его отключение не есть invalidation socket-owner cache.

Доказательство универсальной, непрерывно актуальной signer-bound attribution
каждого пакета **невозможно получить из рассмотренного публичного API-контракта**:
он не выдаёт транзакционный snapshot socket+installation+signers, не блокирует
package mutation и не переаттестует уже открытые/делегированные flows. Это
ограничение выбранных средств и scope, не theorem о любых Android решениях.

## 2. Доступность, версия и provenance

Source availability **AVAILABLE**. Проверены текущие manifests и реальные callers:
Android min29/compile36/target36; service/worker/identity readers и native
`strict-owner/{wiring.patch,overlay,source-lock.json}`. Pinned upstream
sing-box1.13.19 `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`, module cache.
Source-lock помечен `source-only-unverified-not-applied-or-built`, independently
approved AAR SHA отсутствует. Kotlin revision gates — контракт на будущую
поставку, не факт пригодного AAR.

Для AOSP16 прочитаны выбранные symbols frameworks/base
`99b01a65cc4c104933788b3143285ab6bae65827` и ранее собственный immutable capture
Connectivity `627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`. Новая загрузка Connectivity
не удалась из-за DNS; сохранённый файл принят только после проверки его SHA
по прежнему own equality manifest, и прочитан заново. Это не fresh remote equality.
API29/API34 Process sources — release-tag references, не checked whole platform.

37selected local inputs captured последовательно, не atomic tree. Первоначальные
15source-lock comparisons совпали. При09:50:06Z обнаружены concurrent изменения
wiring/source-lock: sourceRevision3, добавлены Box.Start error-result и
StartedService observer-close fixes. Сохранены initial и review bytes/diff.
Strict-owner/mapping hunks не изменены; новая ошибка/closure semantics не
объявлены executed или build-compatible. На10:03:02Z те же3files обновились
для constant E.Cause вместо raw error/panic output; final dated captures9812byte
patch/source-lock/README сохранены, не являются promise неизменного checkout.
Точная финальная сверка — STATUS.

[Полный ledger](SOURCES.md) различает SECTIONS_READ/FULL_TEXT_READ/FETCH_FAILED;
скачивание большой страницы не выдаётся за её полный аудит.

## 3. R28-Z08-A — partial public mapping по версиям и классам UID

### 3.1 Version/capability matrix

| Класс/операция | Публичная опора | API29–33 | API34 | API35–36 | Предел |
|---|---|---|---|---|---|
| TCP/UDP raw owner | `ConnectivityManager.getConnectionOwnerUid` API29 | Есть | Есть | Есть | Active own VpnService/caller scope; INVALID_UID/исключение не app identity |
| User scope | `UserHandle.getUserHandleForUid` API24, `Process.myUserHandle` | Есть | Есть | Есть | User equality не доказывает capture membership/signer |
| Regular application UID | `Process.isApplicationUid` API24 | Есть | Есть | Есть | Classification, не installation proof; false не означает «system» |
| Ordinary app mapping | identity `attributed=raw` при regular/same-user и validated package material | Условно | Условно | Условно | PackageManager read filtered и не atomic с socket/worker |
| SDK sandbox raw→client UID | `Process.isSdkSandboxUid/getAppUidForSdkSandboxUid` API35 | Нет подтверждённого public mapper | Source есть, но `@hide` MODULE_LIBRARIES/TestApi | Есть public методы | Mapping связывает числовые principals, не current signer-authority |
| SDK sandbox client→sandbox | `getSdkSandboxUidForAppUid` API35 | Не предполагать | Platform hidden alternatives не reuse для app | Public reference; implementation/version capability отдельно | Обратная проверка полезна, но не заменяет socket receipt/installation |
| Isolated UID другого процесса | `Process.isIsolatedUid(int)` API34 | Public `isIsolated()` API28 относится только к вызывающему процессу | Public classifier | Public classifier | В выбранных API нет общего signer-bound isolated→client mapper |
| Shared UID | PackageInfo.sharedUserId + visible packages | Требуется отказ для single-package approval | То же | То же | Одного имени недостаточно; одинаковые signers не разделяют runtime principals |
| Foreign profile/user | UserHandle проверка | Не наблюдать/не подставлять parent | То же | То же | Current-user PackageManager и own VpnService не дают global user inventory |
| Kernel/system/delegated service | Raw UID, но не выбранная app identity | Unknown для app-rule | То же | То же | Caller app может передать работу сервису; socket owner не исходная бизнес-задача |

Из `!isApplicationUid(raw)` не выводить ни «SDK sandbox», ни «безопасный system»:
публичная матрица специально остаётся частичной. На API29–34 ordinary flows
могут быть representable, поэтому «вся per-app политика unsupported до35» тоже
слишком сильный вывод. Неподдержан именно общий mapper нестандартных principals.

Текущий reference включает более поздний `getAppUidForPrivateComputeCoreUid`
(версия37.2); он **не** решение для min29/target36 и не переносится в матрицу
как готовое API. Указание метода в текущем справочнике не доказывает его
доступность на исследуемой платформе.

### 3.2 Проверенная source-цепочка D

- AOSP `Vpn.getAppsUids:1838–1851` добавляет app UID и SDK sandbox UID при
  application UID; capture UID closure не равен набору app-rule predicates.
- `ConnectivityService.getConnectionOwnerUid:12939–12959` возвращает socket UID
  от InetDiagMessage и проверяет owner/capture scope. На fresh request это не
  API произвольного packet-origin app. В выбранном InetDiag source raw
  `idiag_uid` не нормализован; UDP wildcard path — отдельная lookup semantics.
- `ComputerEngine.getPackagesForUidInternal:1991–2033` SDK sandbox UID направляет
  в `getBaseSdkSandboxUid()` (:5877), т.е. package SDK sandbox, не обязательно
  клиент p. Returned group фильтруется installed/user/caller visibility.
- Current Kotlin `LibboxPlatform.findConnectionOwner:94–126` получает uid,
  packages, проверяет expected applications по rawUID или selected package и
  возвращает raw `ConnectionOwner.userId`. SDK normalization нет в этом caller.
- `NativePolicyLowering.selector:68–74` строит conjunction `Uid(appUID)` AND
  `Package(packageName)`; exact match upstream UserIdItem не сравнивает
  родительский UID. Meta wrapper libbox переносит значения, не добавляет mapper.

Conditional counterexample D сохраняется **для будущего admitted app-rule**:
успешный socket lookup sandboxUID≠appUID делает `Uid(appUID)` false, даже если
package удалось найти. Но scope default tunnel может оставаться protected;
callback может вернуть unknown, native graph не accepted, blocker уже есть.
Ни actual bypass BLOCK, ни deployed leakage не установлены.

### 3.3 Current signer-bound material и отрицательные identity results

Current `AndroidAppIdentityLookup.read:78–106` читает PackageInfo+
GET_SIGNING_CERTIFICATES, application UID/user/enabled/suspended,
sharedUserId и group cardinality, current apkContentsSigners. Два targeted
чтения сравниваются по package/UID/version/update/current-signers; пятисекундное
окно provisional, не атомарное lifecycle/installation свидетельство.

Прежний guard **сохранён и существенен**: singleton visible list мог бы скрыть
часть shared group, но `sharedUserId != null` уже отвергается отдельно. Broad
QUERY_ALL_PACKAGES не требуется для восстановления этого слабого доказательства.
Нельзя написать «единственный видимый пакет значит sole UID owner».

SigningInfo docs API28 рекомендуют history для общей проверки legitimate
rotation. Наш исходный контракт намеренно строже — exact **current** signer-set,
а не «любой исторический сертификат». `getApkContentsSigners` действительно не
содержит past signers; `getSigningCertificateHistory` для single signer включает
цепочку, current последний, multiple signer возвращает null. Допустимо выбрать
exact-current purpose, но нормальная ротация будет invalidation и потребует
новой authority/approval; не объявлять отказ доказанным compromise. Не разрешать
новый signer автоматически из одной accepted Android rotation history.

## 4. R28-Z08-B — cache/invalidation composition: fresh trace важнее TTL

### 4.1 Два кэша и существующая remediation

| Объект | Прочитанная область | Что реально меняется |
|---|---|---|
| Stock owner process cache | core `route/process_cache.go:24–35`, `router.go:150–152` | Key network/source/destination, result/error; lifetime200ms, no installation epoch |
| Current strict-owner source adaptation | native overlay `route/snowden_owner.go:23–55`, wiring route hunk | Tagged TUN вызывает platformSearcher напрямую **до** rules; не использует cached/prefilled ProcessInfo/local-source shortcut |
| Persistent CacheFile | native `snowden_cache.go`, Kotlin `snowdenDisablePersistentCache` | Контролирует DB admission, не socket UID cache и не mapper |
| Platform PackageManager cache | ApplicationPackageManager:1167–1201 | System invalidated query cache; hidden cache-disable API не доступное app решение |
| Уже routed flows | core routeConnection/routePacketConnection | Admission завершён, выбранный outbound/tracker/connection имеет собственный lifetime |

Current adaptation цепочка source: Kotlin OverrideOptions tag→libbox override→
daemon `snowdenApplyStrictOwner` проверяет Android platform, ровно один явный
strict/auto TUN, без autoRedirect/deprecated inbound options→route option
`json:"-"`→Router→matchRule guard→platformSearcher→Kotlin callback.
Успешный owner заменяет metadata; ошибки становятся fatalErr, TCP/UDP/PreMatch
callers возвращают до outbound selection. Stock arbitrary raw JSON не включает
этот guard; revision marker один не доказывает wiring/AAR provenance.

InboundDetour/source scope/tuple checks важны: guard не называется глобальным
контроллером всех inbounds. `daemon` ограничивает допустимый TUN graph;
whole signed graph/ключи/policy/core parity остаются prerequisites. Вне named
TUN обычный cache path сохраняется. Не считать этот source-кандидат public OS API.

### 4.2 Original lease и observation-before/admission-after

Current path разделяет callbacks и lifecycle:

1. `AppIdentityMonitor.changed:200–209` при relevant event advances epoch и
   сбрасывает selected **до** coalescing/reread. Невалидный/неизвестный список
   не создаёт «без изменений»; named unrelated package event может быть ignored.
2. `NativeApplicationGuard` держит **original child lease**, не mutable latest.
   Failed sticky, последующее refresh той же signing material не оживляет guard.
3. `NativeCallbackGate.callback:19–35` проверяет original current до и после
   операции, deadline/phase; блокирующий PM/read может вернуться поздно, но
   не получает usable result после уже наблюдённой local invalidation.
4. Kotlin callback проверяет выбранный package **или прежний выбранный UID**.
   UID reused новым package не теряется как unselected/default: expected raw UID
   остаётся в `boundApplicationUids`; mismatch ведёт sticky fault+event/error.
5. `WorkerProcess.completeRetirement:303–329` закрывает local app/native admission,
   вызывает graceful phase на existing lane; watchdog/exit отдельны.
6. Parent `WorkerProcessLink` держит exact remote Binder anchor; death recipient/
   explicit dead registration path, не PID kill/poll-as-death. Repository не
   освобождает slot, пока оба pending RPC не вернулись и source custody не settled;
   при observed dead и application reason force-refresh перед bounded replacement.

**Условно доказанный локальный факт:** если invalidation уже committed в
selected original lease до post-step check, этот callback не возвращает успешную
usable metadata. Это source-local guarantee, не atomic OS/Go packet guard:
изменение PackageManager могло произойти раньше event delivery или после
последней проверки. Между successful callback и native decision также есть seam.

### 4.3 Где минимальный public proof заканчивается

Рассмотренные APIs не позволяют одним вызовом резервировать `(socket lifetime,
package installation, signer set, user/profile, policy epoch)` до конца flow.
Несколько worlds могут давать одинаковую историю read receipts:

- W0: package/UID/installation не менялись между reads и admission;
- W1: mutation произошла сразу после последнего read; broadcast ещё не получен.

Provider наблюдения одинаковы до доставки event, поэтому по ним нельзя
доказать непрерывное «current material при каждом packet». Это epistemic
counterexample **для названных наблюдений**, не отрицание privileged OS fences
или будущих других механизмов.

Уже вошедший JNI/syscall callback нельзя прервать local mutex. У уже routed flow
последующие packet copies могут не проходить owner-finder. TTL200ms не привязывает
cache к socket incarnation, coalescing250ms не max-error deadline, timeout5s
проверяется после actual read-return, а gracefulClose не completion all owners.
Доказательства нулевого промежутка stale enforcement или finite upper bound из
этих значений не следуют.

**Terminal scope:** process death может закрыть конкретные worker-owned sockets/
resources и исключить native callbacks этого process; это не автоматическое
исправление source data у parent, не latest PackageManager и не «весь device
traffic теперь protected». Поэтому dead receipt и parent actual return нужны
вместе, а новый mapping остаётся новым проверяемым observation.

## 5. R28-Z08-C — минимальный seam без новых привилегий

Это предложенный контракт, не API/code patch и не новый trust verifier.

### 5.1 Минимальная структура результата

`AttributionObservation` в памяти, privacy-minimal:

- Raw: `socketUID`, input TCP/UDP tuple scope, current user, VPN/TUN generation,
  observation start/end, lookup method/version, success/missing/denied.
- Mapping: `identity | sdkSandbox(API35+) | unsupported`, `attributedUID?`,
  public mapper identity/version; rawUID сохраняется отдельно.
- Installation: selected package p, attributed/current UID, version/update и
  current signer-set matching **original accepted pin**, observed interval.
- Authority binding: original owner namespace/epoch/material fingerprint,
  rule identity/config generation; не подпись самого observation.
- Disposition: `eligible-current-observation | unknown | invalidated | rejected`,
  причины typed, без raw identifiers в diagnostic/export.

Не обязательно передавать raw tuple/package hashes через новый IPC: существующий
callback/worker already holds scoped inputs; сохранять их минимально и только
на duration нужной операции. Hash не даёт новых полномочий/полноты.

### 5.2 Минимальные predicates и их предел

Для допуска выбранного правила требуется conjunction:

`same user + permitted socket-owner observation + supported mapping +
current observed installation matches original pin + original epoch current +
accepted capture/native graph scope + authoritative profile valid`.

- На API29–34 raw regular app UID можно identity-map при validated same-user
  PackageInfo. Nonregular UID не считать client app; return unknown/unsupported.
- На API35–36 sandbox mapping выполняется **до** поиска selected UID bindings
  и signer re-read; otherwise sandbox может уйти в default как unselected.
- Не подменять ядру rawUID на attributed без контракта: можно использовать
  explicit mapping provenance/отдельные fields либо согласованное расширение
  selector UID-set. Первый путь меняет metadata schema/Go-JNI parity; второй
  требует защищённой current app attribution даже при raw UID alternative.
  Ни один не реализован этим исследованием; simple `Uid(a)` update недостаточно.
- System/delegated/isolated/shared group нельзя automatically map по первому
  package name, process name или подписи platform. Остаётся unsupported scope.
- `isApplicationUid` classifier — не доказательство неизменной installation.
  `getNameForUid` durable label из docs — не signer proof/новый grant.

### 5.3 Cache and invalidation ownership

Самое дешёвое already-existing candidate reuse для **new admission** —
strict-owner direct lookup без owner-cache. Если caching позже понадобится,
минимум необходимо связать entries с original session/identity epoch и не
возвращать entry после invalidation; lookup и cache publication тоже fenced.
Это закрывает **observed** epoch-crossing, но не unseen package change или
5-tuple socket incarnation reuse в пределах той же epoch. Один expiry не
заменяет доказательство instance; для этого public query не даёт socket cookie.
Поэтому не обещать blanket correct cached attribution; bypass cache в strict
scope — обоснованный conservative выбор, не measured performance optimum.

Отдельные event receipts:

| Receipt | Чего достаточно / чего не доказывает |
|---|---|
| Package event received/dirty | Local epoch инвалидирован; не exact time mutation и не OS traffic cut |
| Fresh material observed | Материал двух readings совпал; не atomic lifetime lease |
| Admission rejected | Конкретный новый callback/flow не разрешён этим seam; не все старые flows closed |
| Local retirement armed | Future local gates closed; in-flight операции ещё owned |
| Graceful phase returned | Cleanup stage закончился; no worker-death/allFD proof |
| Original remote worker dead | Этот lifecycle anchor terminated; parent work может ещё идти |
| Parent source + both RPC actual returns | Нет потерянного old result/producer custody; current package re-read нужен отдельно |
| New prepared generation admitted | Scope нового процесса/material; не protected task completion |

## 6. Обязательные продолжения и strongest opposition

### 6.1 UID reuse with cached tuple

Stock trace: p/UIDa и tuple k дают cached owner at t0; old socket исчез,
непосредственно позже k reused UIDb или UIDa назначен другой установке; cache
k→old material ещё usable. Stock 200ms не socket-incarnation key. В current
strict-owner path этого **cache hit** нет: direct finder + selected old UID
validation опровергают перенос trace «как есть». Установка/OS-read race и
old admitted flows всё ещё открыты — честно сменён предмет контрпримера.

Повтор numericUID у нового package не автоматически уязвимость: expected UID
из original binding попадёт в selected checks даже если package старого исчез,
и current material mismatch должен вернуть error. Если mapper вводится для
sandbox, то же отрицательное selected-match должно использовать и attributed
UID; raw-only guard в текущем коде этого ещё не делает.

### 6.2 One visible shared participant

Raw PM singleton не full-group certificate. Но current `sharedUserId` reject
и bounded group cardinality уже есть; classический trace singleton-hidden
member не проходит initial selected material acceptance. Значит нельзя
объявить отсутствие shared guard по одному view. Source sharedUserId declaration
не даёт universal OEM nonshared completeness; остаётся scope statement и
known rejection, не broad package inventory request.

### 6.3 IKEv2 refresh versus ordinary VpnService

Прочитаны actual `VpnManagerService` add/remove/replaced branches и
`Vpn.updateAppExclusionList:4361–4376`: refresh changes NetworkCapabilities
only if `isIkev2VpnRunner`. Setter/refresh storage belongs platform profile,
не ordinary Builder configured list и не DPC lockdown allowlist. Более того,
replacement branch может restart выбранного always-on, non-replacing add/remove
обрабатываются иначе. Эти события не являются public coherent receipt
refresh всех caller-owned signers/caches/flows.

Ordinary VpnService manual Builder configuration по docs меняется созданием
нового connection; current Snowden Compiler/factory intentionally ALL_APPS и
rejects package include/exclude. Поэтому не предлагать Platform IKEv2 API
как drop-in invalidation для текущего app. Не доказано отсутствие иных Android
internal refresh paths, только неприменимость этой конкретной ветки.

## 7. Ready reuse / license / цена / scoped verdict

| Решение | Disposition и evidence | Цена / предел |
|---|---|---|
| OS socket-owner+sameUser+PackageInfo/current signing APIs | REUSE reference/publicAPI; API29+ | Targeted Binder/signers cost, sampling races; no privileges/install inventory |
| Process SDK mapper35+ | ADAPT после schema/identity review | Сохранение raw/attributed provenance, app selectors и UID reuse checks; legacy unknown branch обязательна |
| API29–34 hidden sandbox mapper/offset | REJECT в этом ordinary scope | Наличие @hide в source не public support; нет честной cross-version гарантии |
| Generic isolated→client mapping | DEFER/unsupported для app-rule | Public classifier34 не mapper; private-compute mapper37.2 вне target scope |
| Existing strict-owner whole-libbox candidate | REUSE/ADAPT source seam для new admission | GPL-3.0-or-later, exact base/patch+overlay; source-lock hashes не AAR/JNI/packet proof; PM/read32-callback pressure не measured |
| Generic TTL owner cache как identity proof | REJECT для strict observed-epoch guarantee | Socket incarnation/epoch ambiguity; cached API result не current authorization |
| Original applicationGuard+callback/lifecycle gate+worker death fence | REUSE existing source ownership | Закрывает observed invalidation/late result locally; OS mutation atomicity и in-flight/old-flow completion не следует |
| Whole process replacement после uncertain attribution | CONDITIONAL SAFE RECOVERY proposal | Full new graph/material/OS checks; стоимость reconnect/потеря старых flows и grace resources, не seamless UX promise |

Android/AOSP files headers Apache-2.0, Android/whole sing-box adaptation и source
GPL-3.0-or-later. OS API use не требует copying AOSP framework. Никаких dependencies
не импортировано. Full corresponding-source/license/AAR production gates остаются
у platform owner. Цена не измерена в рублях/ms/CPU/энергии; отсутствие measurement
не записано нулём.

**Предлагаемый verdict:** A/B/C `ACCEPTED` только в точно
указанном source/API/model scope **как предложение**, не авторская приёмка;
product-wide atomic signer-bound app enforcement `DEFERRED`. Current strict-owner
source seam — positive counterevidence к прежнему stock-cache finding. Выводы
о live bypass, instant cut после package update, imported AAR correctness не приняты.

## 8. Конечная будущая приёмка — NOT_RUN

| Case | Вход и требуемое разделение |
|---|---|
| Z08-01 | API29/33 regular UID: sameUser+PackageInfo material, visible/missing/rejected отдельно |
| Z08-02 | API34 sandbox mapper exists hidden: ordinary implementation возвращает unsupported, не offset/reflection |
| Z08-03 | API35 sandbox query: raw/attributed values раздельны, selected signer match и app-rule semantics согласованы |
| Z08-04 | Isolated/nonregular/foreign user: no guessed client identity, unknown не direct fallback |
| Z08-05 | Shared group multiple и singleton visible+declared sharedUserId: existing deny guard не снят |
| Z08-06 | UID reused новый package при original binding: отрицательная selectedUID проверка, callback error/fault/retirement раздельны |
| Z08-07 | Cached tuple при stock против adapted tagged TUN: fresh owner guard, cached metadata не достаточна |
| Z08-08 | Package mutation после last PM read/before native decision: локальный epoch не выдаётся за atomic OS fence |
| Z08-09 | Invalidation внутри blocked PM read: result late не usable, underlying operation ownership остаётся |
| Z08-10 | Old admitted TCP/UDP flow без новых mapper callbacks: actual retirement/OS drop/death receipts, не TTL |
| Z08-11 | Worker dead при parent normal/retire RPC pending: neither result discarded, no premature replacement |
| Z08-12 | Refresh Platform IKEv2 против VpnService Builder/DPC allowlist: scope методов не смешан |
| Z08-13 | Exact signer rotation: current-set mismatch не automatic compromise/approval; selected authority отдельна |
| Z08-14 | Stock revision/AAR или неполный overlay: rejected capability, no runtime fallback |

Не созданы executable fixtures/tests; никакие package, device, adb, MDM, network,
crypto/signature operations или builds не запускались.

## 9. Пройденные work stages, paired scope и остановка

Рассмотрены D trace/fresh pins, public version matrix,
package→epoch→cache→admission→old-flow/retirement, minimal seam/reuse/licence/cost,
UID reuse/shared visibility и IKEv2 refresh. Source failures заменены доступной
частью или проверенным immutable capture и не выдаются за новые reads/execution.
Независимый paired review C06 не входил в этот source-срез.

Открыто: artifact/SDK/API-level/OEM validation, actual socket incarnation and
atomic identity enforcement, chosen mixed metadata/selector design, channel
support/task outcomes, cache/stalled Binder/in-flight retirement measurements.
Полный netd/kernel/current policy inventory не audited. Это конечный research
пакет со scoped unknown, не обещание завершить всю UID тему.
