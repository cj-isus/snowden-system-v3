# R28-Z08 — SOURCES и provenance

Чтение: 28 октября 2026, только относящихся sections. `SECTIONS_READ` != full downloaded source audit. `FULL_TEXT_READ` используется только для небольшого source/helper или основного тела короткой документации. Источник/API/source-model/runtime/поле не смешиваются.

## 1. Public API sources (fresh captures этого цикла)

Все страницы — Google/Android Developers, дата чтения28октября2026,
09:26–09:48Z. Дата первоначальной публикации большинства API reference unknown;
проверенная версия API-added и returned update date не считаются одной датой.

| ID / URL | Read section и доказанный контракт | Read-depth / capture |
|---|---|---|
| P01 <https://developer.android.com/reference/android/net/ConnectivityManager> | Signature `getConnectionOwnerUid(int, java.net.InetSocketAddress, java.net.InetSocketAddress)`; API29, TCP/UDP, active VpnService scope, INVALID_UID/IllegalArgumentException/SecurityException. Не arbitrary business-app origin | SECTIONS_READ, `connectivity-reference.html(.txt)` section getConnectionOwnerUid:9300+ |
| P02 <https://developer.android.com/reference/android/os/Process> | isApplicationUidAPI24; current-process isIsolatedAPI28, isIsolatedUid(int)API34; isSdkSandboxUid/getAppUidForSdkSandboxUid/getSdkSandboxUidForAppUidAPI35; private-compute mapping37.2 вне selected scope | SECTIONS_READ, `process-reference.html(.txt)` sections8710,8779,8881–8933. Даты added сверены непосредственно в fetched body |
| P03 <https://developer.android.com/reference/android/os/UserHandle> | getUserHandleForUidAPI24, user context≠kernel app UID; no cross-user permission grant | SECTIONS_READ, `userhandle-reference.html(.txt):8369+` |
| P04 <https://developer.android.com/reference/android/content/pm/PackageManager> | getPackageInfo(String,int), getPackagesForUid, getNameForUid; GET_SIGNING_CERTIFICATES. Latest snapshot update2026-08-28 | SECTIONS_READ, `pm-reference.html(.txt):13474,13578,13711` и flag/signing sections |
| P05 <https://developer.android.com/reference/android/content/pm/SigningInfo> | API28 current apk signers против rotation history; history null при multiple signer; generic documentation предпочитает history, exact-current contract отдельный purpose | SECTIONS_READ, `signing-reference.html(.txt):8400–8463` |
| P06 <https://developer.android.com/training/package-visibility?hl=ru> | **Фактически русский текст**, target30+ фильтрует сведения, список apps private/sensitive; queries/QUERY_ALL_PACKAGES price. Returned update2026-10-02 | FULL_TEXT_READ основного короткого тела, не весь navigation, `package-visibility-ru.html(.txt):579–596` |
| P07 <https://developer.android.com/training/package-visibility/automatic?hl=ru> | Фактически русский перевод, automatic visibility не весь device inventory, system visible set varies; guidance adb НЕ исполнялась. Returned update2026-10-24 | FULL_TEXT_READ основного тела, `package-visibility-automatic-ru.html(.txt):579–606` |
| P08 <https://developer.android.com/guide/topics/manifest/manifest-element?hl=ru> | sharedUserId/sharedUserMaxSdkVersion, существующая установка не исчезает с deprecation; untrusted partial fetch не primary evidence | FETCH_FAILED curl56 missing close_notify после initial connection. Файл не используется как новый полноценный source; текст baseline явно оставлен baseline, exact PackageInfo/guard прочитаны отдельно |
| P09 <https://developer.android.com/reference/android/os/IBinder> | linkToDeath remote binder, process death/notification; isBinderAlive false и true race; unlink не native cleanup. Returned update2026-09-16 | SECTIONS_READ `binder-reference.html(.txt):8495–8536,8592+` |

P01 canonical получен по `.../ConnectivityManager` без anchor.
Exact public signature — `(int, java.net.InetSocketAddress, java.net.InetSocketAddress)`.
Ни один API не вызывался на реальном Android/профиле.

## 2. AOSP exact source и version comparators

### F01 — Frameworks/base Android16 selected sources

Организация/автор AOSP; release `android-16.0.0_r1`; commit
`99b01a65cc4c104933788b3143285ab6bae65827`, commit date2025-05-07 (не дата
публичного релиза). Fresh fetched28октября09:28–09:48Z. Headers Apache-2.0.

Общий immutable URL prefix:
`https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/`.
Captures получены HTTPS raw.githubusercontent.com/aosp-mirror с тем же commit.

| Файл / read scope | Source-факт |
|---|---|
| `core/java/android/os/Process.java:883–1022` | public ordinary classifier, hidden legacy overload distinction, public mapper35/source arithmetic внутри платформы — не лицензия копировать offset на29–34 |
| `core/java/android/os/UserHandle.java:232–248` | isApp underlying bounded regular UID range; false != system. Скрытый isApp не предлагается public consumer, Process public wrapper используется |
| `core/java/android/content/pm/PackageInfo.java:119–130` | sharedUserId material доступен в selected package; actual guard читает его |
| `core/java/android/content/pm/SigningInfo.java:140–206` | exact current signers return contents, history current/latest ordering; no runtime signature/crypto execution |
| `core/java/android/app/ApplicationPackageManager.java:1167–1201` | getPackagesForUid через system invalidated cache; disable/invalidate cache helpers @hide |
| `services/core/java/com/android/server/pm/ComputerEngine.java:1987–2033,5877–5878` | caller-visible package group; sandbox UID→base SDK package; no evidence one returned package=all installed group |
| `services/core/java/com/android/server/connectivity/Vpn.java:1838–1942,2064–2108,4336–4376` | capture SDK UID closure, lockdown set отдельный; IKEv2 refresh branch explicitly conditional |
| `services/core/java/com/android/server/VpnManagerService.java:855–911` | non-replacing add/remove refresh, selected always-on replaced→restart. Not universal VpnService Builder refresh |

Все — SECTIONS_READ, не полный аудит ComputerEngine/Vpn; small PackageInfo
material/read methods only. Файлы доступны в own artifacts как одноимённые
`.java`; SHA256 в final source index. Просмотрено наличие LICENSE headers,
не complete AOSP NOTICE/deployment audit.

### F02 — ранние версии как отрицательный public-support comparator

- <https://github.com/aosp-mirror/platform_frameworks_base/blob/android-10.0.0_r1/core/java/android/os/Process.java>
  capture `Process.api29.java`, прочитаны isApplicationUid/isIsolated annotations.
- <https://github.com/aosp-mirror/platform_frameworks_base/blob/android-14.0.0_r1/core/java/android/os/Process.java>
  capture `Process.api34.java:886–963` **SECTIONS_READ**; sandbox classifier/mapper
  уже есть в source, но `@hide` + `@SystemApi(MODULE_LIBRARIES)` + TestApi.

Это tag-reference negative support, commits tag↔immutable здесь не разрешались
и whole platform/API29–34 variants не audited. Public unavailable вывод не
основан только на grep: он согласован с fetched API-added table P02. Не делать
«всех технологий sandbox на API29 нет» из отрицательного grep.

### F03 — Connectivity module: собственный previous immutable capture

- Commit `627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`, release android16r1,
  org AOSP, commit date2025-04-24.
- <https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/service/src/com/android/server/ConnectivityService.java>
  source12933–12959 getConnectionOwnerUid; raw UID→same VPN owner scope;
  dependencies1535–1537 delegates InetDiag.
- <https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/staticlibs/device/com/android/net/module/util/netlink/InetDiagMessage.java>
  source202–294 socket idiag_uid, UDP wildcard fallback/errors.

Fresh capture request этой партии не удался (DNS resolution, curl6).
Переиспользованы только **свои** immutable copies с проверенным SHA из own
previous equality manifest; перечитаны named sections. Metadata
`reused-immutable-sources.json` прямо указывает origin/capture-limit.
Это SECTIONS_READ reused source, **не** current remote equality или kernel/OEM
runtime proof. Предыдущий корпус не перечитывался целиком.

## 3. Repository/whole-core caller inputs

Captured initially09:32:18Z и extended адресно до09:50Z:37inputs
(27repository/metadata+10module refs; exact listing `inputs-start.json`).
Это sequence, не atomic tree и не source version всех платформенных изменений.
Читать extent указан ниже; source hashes сами не доказывают compilation.

| Input | Прочитанные sections / значение |
|---|---|
| android/app/corebridge build.gradle.kts | min29,target/compile36; не executed resolution |
| android/vpn Manifest | SUPPORTS_ALWAYS_ON=false, own service/worker scope; реальный merged manifest не установлен |
| AndroidAppIdentity.kt:44–106 | current-user targeted observer, sharedUserId+cardinality deny, signing exact-current, two reads/freshness bound |
| AppIdentityBinding.kt:34–62 | Original material pin+owner namespace/epoch/fingerprint; not public OS transaction lease |
| LibboxPlatform.kt:94–126 | Raw UID unchanged; expected chosen rawUID или package checks; mismatch generic fault/error |
| NativeApplicationGuard.kt:12–33 | FULL_TEXT_READ маленького helper; sticky original lease invalidation |
| NativeCallbackGate.kt:19–35,49–84,92–111 | pre/post local check, no monitor over JNI, phase/idle accounting; zero callbacks != death |
| NativeWorkerSession.kt:69–85,128–167,187–198 | Marker/tag/native-only overrides, closeNativeIfIdle graceful only, empty app binding scope; not installed AAR |
| NativeRuntime.kt:15–35 | Runtime owner forwards retirement, outcome не release |
| AppIdentityMonitor.kt:79–93,118–164,194–303,338–360 | Relevant broadcast epoch-before-read, registration, freshness vs isCurrent original lease, no rebind |
| WorkerProcess.kt:183–217,263–329,365–388 | Fresh prep gates, original app lease, beginRetirement/grace/exit separation, watchdog |
| WorkerProcessLink.kt:25–59,88–111,131–150 | Exact remote Binder anchor+death recipient; observer unlink != command completion |
| VpnSessionRepository.kt:426–470,660–674,791–856 | Two RPC actual returns, source custody and old death; force refresh before replacement after app failure |
| AndroidTunFactory.kt:31–82 | Full capture, package/partial/direct-exclusion capabilities rejected |
| NativePolicyLowering.kt:23–45,68–74 | exactUID+package predicates, current OWNER_CAPABILITY/PACKAGE_LIFECYCLE/parity blockers |
| NativePolicyPredicate.kt:87–98 | Equality and any group member matcher; returned source model only |
| windows/go.mod | current sing-box1.13.19 Go core reference; **не** Android AAR pinned artifact proof |

### N01 — актуальная native strict-owner adaptation

Пути `android/native/strict-owner/`:

- `source-lock.json`, `wiring.patch` — SECTIONS_READ contents+specific hunks,
  hashes compared, **patch not applied/checked**.
- `overlay/route/snowden_owner.go` — FULL_TEXT_READ75lines; exact taggedTUN fresh
  platformSearcher, errors before metadata usable, bounded returned group.
- `overlay/daemon/snowden_owner.go` — FULL_TEXT_READ59lines; override guards
  Android/oneTUN/strictAuto/noRedirect and non-JSON route property.
- `overlay/snowden_cache.go` — FULL_TEXT_READ42lines; persistent CacheFile
  admission control, не owner-cache invalidation.
- `overlay/experimental/libbox/snowden_owner.go` — FULL_TEXT_READ capability
  markers, self-revision не provenance.
- README и ADR057 — SECTIONS_READ relevant cache/acceptance blocks.

Base exact sing-box commit b5ebaa1…; GPL-3.0-or-later source adaptation.
Kotlin calls native markers требуют whole corresponding AAR со всеми wiring/
overlays; approvedDigest null/source-only статус сохранены. Runtime inventory
не получен, build/gomobile/JNI/patch execution NOT_RUN.

### N02 — upstream module cache/callers

`github.com/sagernet/sing-box@v1.13.19`, base commit
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d` из current source-lock.

- `route/route.go`:33–57 TCP error closure;60–98 scope/matchRule;
 183–225 UDP error/match;292–295 PreMatch error;403–414 guard insertion position.
- `route/router.go`:119–152 Android finder/cache initialization.
- `route/process_cache.go`:24–53 key/result/error/metadata; FULL_TEXT_READ
  маленького95line source file.
- `route/platform_searcher.go`:21–44 callback tuple/supported network.
- `experimental/libbox/service.go`:181–222 public-interface wrapper/no mapper.
- `route/rule/rule_item_user_id.go` и `rule_item_package_name.go` —
  FULL_TEXT_READ маленьких helpers: equality/raw list matcher.
- `experimental/libbox/command_server.go`, `option/route.go` — только relevant
  forwarding/base option hunks source-lock comparison, не все exports audited.
- `LICENSE` — FULL_TEXT_READ маленького copyright/license statement,
  GPL-3.0-or-later.

Remote immutable route.go/process_cache.go этой партии получены и bytes совпали
с captured cache.15initial base/overlay/wiring hash comparisons совпали.
Это не fresh remote equality каждого dependency или AAR.

## 4. Concurrent drift и отрицательные получения

09:50:06Z source-lock/wiring изменились concurrent platform work. Initial captures
не переписаны; diff/текущие captures в `drift-review/`. Добавлены sourceRevision3
Box.Start named error и observer-owner Close fixes. Direct owner-finder/mapping
hunks не изменены. На09:56–09:58Z обновилась metadata source-lock/README
(честное слово newRetryPath вместо retryAllowed, patch byte/hash reconciliation).
Final current patch9450bytes SHA0eaab91e… совпал source-lock, все6overlays
совпали с ним; это **статическая bytes/reference** consistency, не patch apply.
На следующем cut10:03:02Z снова изменились те же3inputs: Start cleanup добавляет
constant E.Cause вместо raw console panic/stack; current patch9812bytes/lock
SHAeebc335e… сохранены в `final-cut/`. Actual strict-owner routing/mapper hunks
не изменены. Current checkout непрерывно меняется: не заявляется whole-file
freshness после dated cut и не выполняется бессмысленный повтор всего review.
Анализ inner subscriber implementation/нового Start fix — вне mapper scope;
не переносить на all-native-lifecycle correctness.

FETCH_FAILED/ограничения:

- First multi-curl call interrupted before tool execution: файлы отсутствовали;
  осмотр состояния предшествовал отдельной новой acquisition.
- VpnManagerService initial timeout0bytes; одна новая bounded acquisition через
  ту же public raw URL позднее succeeded, не бесконечный retry.
- Connectivity googlesource fresh request DNS failed; reused own SHA-verified
  immutable capture F03, failure не считается source freshness evidence.
- Manifest ru curl closed_without_close_notify: не выключали TLS/не делали
  insecure re-fetch; failed file не используется как новый full source.

Никаких tests/builds/network probes/packages/ADB/MDM/scans/crypto execution.
Публичное HTTPS source fetch не VPN/live experiment; extract HTML text/SHA —
только обработка публичной документации.

## 5. Финальные документы и предел

REPORT/HANDOFF/STATUS этого Z08 — единственные durable research writes.
Private captures только `.artifacts/research/r28-contracts/Z08/`.
C06 TASK прочитан адресно, HANDOFF не существовал на paired-review-stage;
no independent result claimed, no neighbor wait/poll/messages.

Final source/reference preservation/drift и docs links: own `final-review.json`. Это статическая сверка, не runtime proof.
