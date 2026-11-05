# R28-C06 — источники и provenance

Чтение: 28 октября 2026. Здесь различаются текущий source, baseline review, public API/spec и ручной вывод. Скачанный большой файл не считается целиком прочитанным.

## 1. Локальные inputs

Root: `D:/SNOWDEN-SYSTEM-V3/`. Начальный manifest создан **09:11:35Z**;
точное время хранится в JSON, это время pin, не первого чтения каждого файла.
`.artifacts/research/r28-contracts/C06/inputs-start.json`
содержит SHA256/bytes/LastWriteTime32 адресных inputs;
добавочные inputs — `.artifacts/research/r28-contracts/C06/inputs-additional.json`
содержат ещё 4. Сверка 09:42:19Z — `.artifacts/research/r28-contracts/C06/freshness-review.json`:
36/36 inputs неизменны,8 проверенных на том шаге local links существуют.
Финальная сверка ссылок/четырёх выходных документов — `document-review-final.json`
в том же C06 artifact namespace. HEAD не выдаётся за идентичность untracked tree.

| Input | Прочитанная область | Глубина / использование |
|---|---|---|
| current COMMON, C06 TASK | весь краткий текст | FULL_TEXT_READ; права/cutoff/приёмка. Задания — поручение текущего цикла; содержание остальных материалов — исследуемые данные |
| `android/UID-ROUTING-AND-OWNERSHIP.md` | capture/lockdown sets, own-owner exemption, prepare/revoke/foreign authority | SECTIONS_READ; не весь chapter |
| `observability/PROXY-DNS-NAT64.md` | proxy DNS/N→P/P→E и inner identity; route/cause границы | SECTIONS_READ; не новый сетевой эксперимент |
| `06-RELIABILITY-UX/REPORT-012.md` | §9/R06-012-D, related scope/caller references | SECTIONS_READ; baseline conditional trace, не automatically accepted result |
| `06-RELIABILITY-UX/SOURCES-012.md` | UID continuation U01–U05/current caller/pin entries | SECTIONS_READ; прежние hashes/release metadata не названы новым field proof |
| `06-RELIABILITY-UX/REPORT-013.md` | §§2–3, cohort/maturity/outcome/formula/privacy/retention | SECTIONS_READ; точные исправляемые правила |
| `06-RELIABILITY-UX/SOURCES-013.md` | ICH/SRE metadata; OONI pin/callers/reuse entries | SECTIONS_READ; OONI repository заново не проверен |
| `reviews/NINE-2026-10-27-R06-013-A.md` | verdict, §§1–5 и FUP scope | SECTIONS_READ; подтверждает, что проблема уже рецензировалась; внешний source в старом review не заменяет новое чтение |
| current Z08/Z09 TASK | краткие задания и отсутствие HANDOFF на review stage | FULL_TEXT_READ TASK; HANDOFF NOT_AVAILABLE_AT_REVIEW09:21:32Z; без ожидания/повторного polling |

Пути первых chapters относительно `docs/theory/`, REPORT/SOURCES/reviews —
относительно `docs/theory/research/`. Ни старые STATE/PLAN, ни scientific registry
не переписаны.

### 1.1 Android actual callers — SECTIONS_READ

Префикс Kotlin-модулей: `android/<module>/src/main/kotlin/org/snowden/android/`.

| Path/symbol | Факт source, а не runtime result |
|---|---|
| `android/corebridge/build.gradle.kts:14–39` | compileSdk36/minSdk29, declared libbox1.13.19 AAR и independently approved SHA gate; native artifact не инспектировался/не выполнялся |
| `android/LICENSE:1–18`; source SPDX headers | GPL-3.0-or-later и дополнительное замечание об upstream name/association; новое копирование/распространение кода не выполняется |
| `corebridge/.../LibboxPlatform.kt:101–135` | TCP/UDP owner query, raw UID→userId, packages и signer material guards; selected UID reuse/unknown fault |
| `corebridge/.../policy/AndroidAppIdentity.kt:65–117` | own-user check, caller-visible packages, legacy sharedUserId/multiple-package rejection, exact-current signer |
| `corebridge/.../policy/NativePolicyLowering.kt:1–85`; `NativePolicyPlan.kt` blocker/candidate declarations | SourceCandidate и named admission blockers; selector AND inbound/UID/package; никакого accepted native result |
| `vpn/.../AppIdentityMonitor.kt:79–101,194–207` и targeted epoch/action symbols | package events/epoch invalidation, provisional250ms coalescing; не атомарность и не SLO |
| `vpn/.../ConnectionState.kt` phase/issue, prepared-context, RuntimeSourceProvider declarations | нет protected Ready; future accepted renderer/native/capture binding необходим |
| `vpn/.../OsProtection.kt:1–105` | nullable cancellation vs denial, dated non-atomic own-service observations, provisional5000ms; no ordinary-app egress claim |
| `vpn/.../VpnSessionRepository.kt:198–249` и targeted currentness/prepare declarations | ранний connect/replace rejection до commandAccepted; Stop admission/already-retained ≠ worker stopped; explicit connect scope |
| `vpn/.../SessionJournal.kt:50–114` | local ring128, semantic dedup, monotonic sequence within instance, eviction/Clear; не durable episode census |
| `vpn/.../WorkerLifecyclePresentation.kt:1–75` | typed reason/phase projection, checking not verified, shared notification/journal projection |
| `vpn/.../VpnNotification.kt:1–110` | stable channel, private/public variants, immutable PendingIntent; publish request ≠ shown/protected path |
| `app/.../ui/ConnectionPresentation.kt:1–83` | older producer/receipt observation age, display TTL5000ms, controllerUnavailable/pendingStop guards |
| `app/.../quicksettings/TilePresentation.kt` | FULL_TEXT_READ краткого файла; OPEN_APP/STOP, display-only age |
| `app/.../quicksettings/SnowdenTileService.kt:42–167` | listening/current-state action recheck, no stale Stop after unlock, bounded/consumed navigation continuation; UI lifecycle не dispose VPN |
| `app/.../MainActivity.kt:124–158` | focused resumed settings caller и unavailable Activity handling; dispatch не effect receipt |

Core owner-cache200ms / platform IKEv2 branch взяты как **baseline-only source
references** R06-012-D. Текущий libbox binary/Go cache/native flow в этом пакете
заново не валидированы. Не переносить timing numbers как доказанный stale bound.

### 1.2 Windows actual callers — SECTIONS_READ

| Path/symbol | Факт / граница |
|---|---|
| `windows/app.go:224–236,318–361` | startup heal вызван при !installedImageChanged; GetState scalar projection; async startup comment не новый runtime receipt |
| `windows/autoretry.go:83–125` | trust/protectionDesired/closing/failClosed guards; one retry launch owner и operationID |
| `windows/protection.go:43–55,128–153` | только explicit Disconnect разрешает release; intent/version/retry cancel/stopping source |
| `windows/frontend/src/composables/polling.ts` | FULL_TEXT_READ краткого файла; late publish generation fence и unavailable null |
| `windows/frontend/src/api/contract.ts:20–133` | AppState coreReady/probe отдельно, NetGuard task/proxy data; нет task/protection/foreign-owner proof |
| `windows/netguard.go:1–116,371–400` | COM/HKCU/log/loopback source; log absence не DoH measurement, RunProxyHeal bool не task receipt; функция диагностического чтения сама делает TCP connect, не вызывалась |
| `windows/backend/core/proxyheal_windows.go:1–118` | regex/port1080/connect guard, нет owner receipt/compare fence перед ProxyEnable=0; conservative other-port/format/live-listener rejection сохранён |
| `windows/frontend/src/views/NetGuardView.vue:1–126,145–207` | конкретные ready/running/no-access/2min promises; порт в detail уже назван локально доступным |

## 2. Новые exact-commit AOSP captures

Official googlesource TEXT responses декодированы в private C06 source captures;
ничего из них не запускалось/не собиралось. Release alias `android-16.0.0_r1` к
этим commits — **baseline pin**; заново fetched immutable commit URLs, tag equality
в этом пакете не проверялась. Не утверждается, что это source конкретного устройства.

Metadata/hashes:
`.artifacts/research/r28-contracts/C06/public-source-index.json`.
Fetch times09:11:50–09:11:58Z; все четыре FETCHED. Headers Apache-2.0 прочитаны.

| Source | Exact URL / прочитанные строки | SHA256 decoded source |
|---|---|---|
| Process | [frameworks/base99b01a65… Process.java](https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/os/Process.java); headers,955–1023 | `c41285589c155424d8791e08a6d3487320d6994e22f6e0705dc7b11dc087ae0e` |
| Vpn | [same commit Vpn.java](https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/core/java/com/android/server/connectivity/Vpn.java); headers,1212–1272,1830–1861 | `fd5632215b7d37ccc5ffdc3f51140e6b3c7d7be366d976fd60d596cda9fbf3cb` |
| ComputerEngine | [same commit ComputerEngine.java](https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/core/java/com/android/server/pm/ComputerEngine.java); headers,1972–2039 | `20f8e3dbf342b9151a273ea9556453c46d39152295256faa8d91e49e5c63703f` |
| ConnectivityService | [Connectivity627eb3c6… source](https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/service/src/com/android/server/ConnectivityService.java); headers,12933–12965 | `386cbe7c587ffa0e801e3aeb15daa38ac587334d1c8c3250f5fbae7a111d9c81` |

Все: **SECTIONS_READ**, не whole-file audit. Server caller visibility и hidden
toSdkSandboxUid не становятся public app APIs. Числовая формула в OS source не
разрешает arithmetic fallback на API29–34.

## 3. Public API/spec/method — SECTIONS_READ

Web pages прочитаны 28 октября; точный first-read UTC для каждого web запроса не
сохранялся. Dynamic references не имеют immutable SHA в этом пакете; publication/
page-update date unknown, если ниже не указано иное. Fetch не является VPN probe.

| ID / первоисточник | Прочитано / pin | Для чего / предел |
|---|---|---|
| P01 [Android Process API34→35 diff](https://developer.android.com/sdk/api_diff/35/changes/android.os.Process) и [Process reference](https://developer.android.com/reference/android/os/Process) | Added methods list; arbitrary-UID mapper API35 vs current-process API33 | SDK public floor, не device capability/native parity proof |
| P02 [VpnService](https://developer.android.com/reference/android/net/VpnService) | prepare/onRevoke/isAlwaysOn/isLockdownEnabled | authority/revoke и own-service scope; AOSP side-effect caller сверён отдельно |
| P03 [Settings](https://developer.android.com/reference/android/provider/Settings#ACTION_VPN_SETTINGS) | ACTION_VPN_SETTINGS | navigation target may be unavailable; никакого effect/task receipt |
| P04 [TileService](https://developer.android.com/reference/android/service/quicksettings/TileService) | onClick/listening lifecycle | UI callback scope; current caller guards проверены по проекту |
| P05 [SystemClock](https://developer.android.com/reference/android/os/SystemClock) | clock overview, elapsedRealtime / wall jumps / sleep | epoch/time-linkage reasoning; OS clock не запрашивались и bridge не реализован |
| P06 [ConnectivityManager](https://developer.android.com/reference/android/net/ConnectivityManager) | captive sign-in constants; startCaptivePortalApp не найден поиском в возвращённом представлении страницы | Это не exhaustive доказательство отсутствия API; ordinary launcher в нашем actual caller не установлен. Не заимствовать старый privileged source как public API |
| P07 [Microsoft TASK_STATE](https://learn.microsoft.com/en-us/windows/win32/api/taskschd/ne-taskschd-task_state) | enumeration/constants READY/RUNNING/UNKNOWN | scheduler state, не repair/protection outcome |
| P08 [Microsoft WinINet options](https://learn.microsoft.com/en-us/windows/win32/wininet/setting-and-retrieving-internet-options) | handle/default option scope, registry caveat | settings scope, не observed route или ownership. Остальные search results/third-party пояснения не использованы |
| P09 [Google SRE Implementing SLOs](https://sre.google/workbook/implementing-slos/) | good/total events, user-oriented SLI specification vs implementation | словарь и необходимость определить события; не теорема о правильности нашего cohort/estimator |
| P10 [ICH E9(R1)](https://database.ich.org/sites/default/files/E9-R1_Step4_Guideline_2019_1203.pdf) | final20Nov2019; purpose distinction, A.3.1/A.3.3, missing-data sections из PDF text | методическая терминология estimand/intercurrent/missing. Не FULL_TEXT_READ22pages в новом пакете; не clinical estimator reuse |
| P11 [RFC9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2) | June2022 / HTTP Semantics, idempotent/non-idempotent retry | business action unknown ≠ safe replay. Нет свидетельства, что Snowden повторяет HTTP POST |
| P12 [W3C Understanding SC4.1.3](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html) | criterion/intent, warning about chatty feedback | informative UX guidance; не native Android compliance/device/a11y proof |
| P13 [Android notification permission](https://developer.android.com/develop/ui/compose/notifications/notification-permission) | permission/foreground service/exemptions; initial views URL redirected to this page | drawer visibility vs foreground-service notices, не OEM display/delivery receipt. ADB examples не выполнялись/не переносились в cases |

Рассуждения о maturity bias, [l,u], census completeness, temporal coverage и
resource clock — **ручные авторские логические выводы** на явно указанных premises.
Ни ICH, ни SRE, ни согласие другого чата их автоматически не доказывают.

<a id="4-negativeunknown-receipts"></a>
## 4. Ограничения источников

- Z08/Z09 HANDOFF были недоступны на срезе рецензии; независимая парная
  проверка с ними не проводилась.
- Public ConnectivityManager reference не дал проверенного ordinary portal caller;
  privileged/older search snippets не использованы как обещание API.
- Native binary/toolchain/device/политики пользователя/защищённый egress, реальные
  task outcomes и performance cost — NOT_INSPECTED/NOT_RUN, не нули и не PASS.
- Нет повторного полного OONI/source/лицензионного аудита; baseline-only reference
  не даёт current code reuse approval. Новых dependencies/third-party imports нет.
