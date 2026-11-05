# R06-012 — источники, версии и границы чтения

**Дата чтения и captures:** 27 октября 2026 UTC.

`FULL_TEXT_READ` ниже означает прочитанную целиком короткую страницу;
`RELEVANT_SECTIONS_READ` — выбранные конкретные sections/symbols большого
справочника/файла. Скачивание большого файла **не** означает его полный аудит.
Ни один источник не считается runtime/native/field evidence.

## 1. Первичные Android источники

### A01 — Android Developers, «VPN» (реальный русский перевод)

- URL: <https://developer.android.com/develop/connectivity/vpn?hl=ru>.
- Организация: Google / Android Developers.
- Дата страницы: «Последнее обновление: 2026-10-24 UTC» в возвращённом тексте;
  дата первого издания не установлена.
- Чтение: 2026-10-27, партия 13:40–13:43Z; `RELEVANT_SECTIONS_READ`:
  «VPN-сервис», «Подготовка сервиса», «Жизненный цикл», «Постоянная VPN»,
  «заблокированные подключения», «Контроль трафика VPN для приложений»,
  «Разрешенные приложения», «Список приложений без доступа к VPN».
- Capture: `android-vpn.ru.html` + извлечённый `.html.txt:475–641`.
- Подтверждено: UI consent; один provider на user/profile; Builder списки задаёт
  VPN-приложение до establish; no-list default; lockdown влияет на excluded apps.
- Перевод местами называет application lists «списками сайтов» и формулирует
  блокировку всех flows без подробных системных исключений. Для точной семантики
  сверено с A02–A08, не перенесено буквально как theorem.

### A02 — Android API, DevicePolicyManager

- URL: <https://developer.android.com/reference/android/app/admin/DevicePolicyManager>.
- Организация: Google / Android Developers; публикация/история добавления
  **конкретных API**: API24/29/31 (различия ниже). Дата первого текста неизвестна.
- Чтение: 2026-10-27, 13:41–13:43Z; `RELEVANT_SECTIONS_READ`:
  `setAlwaysOnVpnPackage` (обе перегрузки), `getAlwaysOnVpnPackage`,
  `isAlwaysOnVpnLockdownEnabled`, `getAlwaysOnVpnLockdownWhitelist`.
- Capture: `dpm.reference.html`, `.html.txt:13653–13686,16232–16247,17530–17612`.
- Added-in table: setter 3 args и get package — API24; setter 4 args и
  admin lockdown/whitelist getters — API29. `String` nullable, четвертый arg
  `Set<String>`, **не List**; method name singular.
- Reference source newer than Android16 pin возможен: только перечисленные
  signatures/added-in и связанные semantics сопоставлены с A03/A05.

### A03 — AOSP frameworks/base, DevicePolicyManager.java

- Source release: `android-16.0.0_r1`; commit
  `99b01a65cc4c104933788b3143285ab6bae65827`.
- URL: <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/app/admin/DevicePolicyManager.java>.
- Организация/авторство: Android Open Source Project, file header Apache-2.0.
  Commit committer date 2025-05-07; это **не** дата публичного релиза Android16.
- Чтение: 2026-10-27, 13:37–13:39Z, дополнительные проверки 13:55Z;
  `RELEVANT_SECTIONS_READ`: строки 8528–8718.
- Capture: `DevicePolicyManager.android16.java`, `.immutable.java`.
- Важно: setter owner scope; 3-arg очищает allowlist; no-arg queries `@hide`;
  API31 clearing owner vs user config; native JavaDoc system-app caveat.

### A04 — AOSP VpnService.java и API-reference

- Immutable URL: <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/net/VpnService.java>.
- References: <https://developer.android.com/reference/android/net/VpnService>,
  <https://developer.android.com/reference/android/net/VpnService.Builder>.
- Организация: AOSP / Android Developers; Android16 source commit date как A03;
  добавление VpnService/prepare/onRevoke — API14; списки/allowBypass — API21;
  `isAlwaysOn/isLockdownEnabled` — API29.
- Чтение: 2026-10-27 13:37–13:49Z; `RELEVANT_SECTIONS_READ`:
  source 183–215,382–455,791–872,927–979; Builder-reference txt 8349–8395,
  8510–8522; VpnService reference API-added sections.
- Captures: `VpnService.android16.java`, `.immutable.java`,
  `builder.reference.html(.txt)`, `vpnservice.reference.html`, `.txt`.
- Контрпример: establish nullable и seamless handover старый fd ещё валиден,
  но новый interface получает выходные пакеты; это не обещание task continuity.
- Нет claim, что любой вызов prepare side-effect-free или всегда revoke.

### A05 — AOSP DevicePolicyManagerService.java

- URL: <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/devicepolicy/java/com/android/server/devicepolicy/DevicePolicyManagerService.java>.
- Организация: AOSP; source release/commit как A03, file header Apache-2.0.
- Чтение: 2026-10-27 13:47–13:55Z; `RELEVANT_SECTIONS_READ` 7563–7722.
- Captures: `DevicePolicyManagerService.android16.java`, `.immutable.java`.
- Actual setter: owner gate; null при отсутствии admin-owned history no-op;
  package/user checks; calls `VpnManager.setAlwaysOnVpnPackageForUser`.
  Lockdown query содержит отдельный signature `MAINLINE_NETWORK_STACK` путь.
  Никакая owner роль не получена обычным Snowden app в этом исследовании.

### A06 — AOSP Vpn.java (platform enforcement, не API facade)

- URL: <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/core/java/com/android/server/connectivity/Vpn.java>.
- Организация: AOSP, Apache-2.0 file header; release/commit как A03.
- Чтение: 2026-10-27 13:42–13:50Z; `RELEVANT_SECTIONS_READ`:
  783–839 always-on support (installed PlatformVpnProfile branch before metadata);
  1194–1265 prepare; 1597–1616 `allowBypass && !mLockdown`;
  1838–1942 package→UID/SDK sandbox/restricted-profile ranges;
  2055–2140 setVpnForcedLocked/setRequireVpnForUids; always-on setup fragments.
- Captures: `Vpn.android16.java`, `.immutable.java`.
- Подтверждено: always-on dethroning guards; previously consented prepare can
  call prepareInternal; blocked UID ranges distinct from capture C; VPN package
  и admin allowlist exempt, UID0 special case. Полные netd/eBPF/OEM paths не audited.

### A07 — AOSP UserManager.java, DISALLOW_CONFIG_VPN

- URL: <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/os/UserManager.java>.
- Организация: AOSP; source/commit как A03.
- Чтение: 2026-10-27 13:42–13:43Z; `RELEVANT_SECTIONS_READ` 858–878.
- Captures: `UserManager.android16.java`, `.immutable.java`.
- История behavior из JavaDoc: profile-owner restriction эффект API23+,
  admin-created always-on API24+, user-created VPN clearing API31+.
  Signature permission для управляемых restrictions не заменяет обычному app
  owner-authority setter из A05.

### A08 — AOSP Settings, AppManagementFragment.java

- Release `android-16.0.0_r1`; Settings commit
  `0fb38ed81e9b49d5da4be8f50d9d69865c1192e8`, committer 2025-04-25.
- URL: <https://android.googlesource.com/platform/packages/apps/Settings/+/0fb38ed81e9b49d5da4be8f50d9d69865c1192e8/src/com/android/settings/vpn2/AppManagementFragment.java>.
- Организация: AOSP, Apache-2.0 file header.
- Чтение: 2026-10-27 13:46–13:50Z; `RELEVANT_SECTIONS_READ` 228–298.
- Captures: `AppManagementFragment.android16.java`, `.immutable.java`.
- Stock UI: checks restriction/admin owner/support before enabling controls;
  system no-arg DPM calls не public API для Snowden. OEM UI не проверен.

## 2. Первичные Microsoft источники

Для коротких W01/W03/W04/W05/W06 pages тело прочитано целиком; для большого
VPNv2 CSP — только названные nodes/XSD и таблицы применимости. Русский язык
возвращённого текста проверен; MT-метаданные не скрываются.

### W01 — Microsoft, «Фильтрация арбитража» / Filter Arbitration

- <https://learn.microsoft.com/ru-ru/windows/win32/fwp/filter-arbitration>;
  <https://learn.microsoft.com/en-us/windows/win32/fwp/filter-arbitration>.
- Документ `ms.date` 2018-05-31; content commit
  `d3d620dcc2a9270060b83bd23779e6decdeceaff`; ru translation MT, snapshot
  updated 2026-07-05 / en snapshot 2025-03-11 (разные locale update dates).
- Чтение: 2026-10-27 13:39–13:49Z; `FULL_TEXT_READ` короткого тела обеих версий.
- Captures: `wfp-arbitration.ru.md`, `.en.md` (ранний `.en.html` фактически markdown).
- Sections: Filtering Model, Configurable Override Policy, default action table,
  Veto, no cross-layer arbitration.
- **Неоднозначность документа:** общий paragraph про CLEAR_ACTION_RIGHT и hard/
  soft может читаться не так, как default action table; поэтому source bool
  не объявлен native guarantee. Отдельный static block default/table — основание
  сомневаться в локальном комментарии, не доказательство runtime fault.

### W02 — Microsoft, FWPM_FILTER0

- <https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_filter0>.
- `ms.date` 2018-12-05, snapshot updated 2024-02-22;
  commit `9267262487657894a8af112d7165006fed5035a7`.
- Чтение: 2026-10-27 13:43–13:51Z; `RELEVANT_SECTIONS_READ` Flags, Weight,
  Conditions, Action, Requirements (Vista+, desktop).
- Capture `fwpm-filter0.en.md`.
- `CLEAR_ACTION_RIGHT` = clear action right; не отдельный универсальный API
  «soften arbitrary static block». Conditions AND и weight scope требуют контекста.

### W03 — Microsoft WDK, FWPS_CALLOUT_CLASSIFY_FN0

- <https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/fwpsk/nc-fwpsk-fwps_callout_classify_fn0>.
- `ms.date` 2019-10-28, snapshot updated 2024-02-22;
  commit `5ea3c2ebb6627443093da84cf357fead33d1b950`.
- Чтение 2026-10-27 13:51–13:53Z; `FULL_TEXT_READ` короткого тела;
  Remarks: clear ACTION_WRITE при block, при permit+filter clear flag.
- Capture `wdk-classifyfn.en.md`.
- Это **callout** contract, не доказательство, что own static block есть callout.

### W04 — Microsoft, «Повторная авторизация ALE»

- <https://learn.microsoft.com/ru-ru/windows/win32/fwp/ale-re-authorization>.
- `ms.date` 2018-05-31, content commit `d3d620dcc2a9270060b83bd23779e6decdeceaff`;
  ru snapshot updated 2026-07-05, официальный MT.
- Чтение 2026-10-27 13:48–13:49Z; `FULL_TEXT_READ` короткого тела.
- Capture `ale-reauthorization.ru.md`.
- Events policy/interface/pend-complete; no guarantee «любой blocked SYN
  оживёт после маршрутизации», unknown fields могут быть FWP_EMPTY.

### W05 — Microsoft, VPN routing decisions (прочитан и русский вариант)

- <https://learn.microsoft.com/ru-ru/windows/security/operating-system-security/network-security/vpn/vpn-routing>;
  en canonical для проверки термина:
  <https://learn.microsoft.com/en-us/windows/security/operating-system-security/network-security/vpn/vpn-routing>.
- `ms.date` 2025-01-27; content commit
  `1875064b378dbc925f2dbd1b5b17841f2875a807`, snapshot 2026-10-08.
- Чтение 2026-10-27 13:44–13:49Z; `FULL_TEXT_READ` короткого тела.
- Capture `vpn-routing.ru.md`.
- ForceTunnel manipulates default routes/metric, более специфичный physical
  route может победить; route presence не kill switch.

### W06 — Microsoft, «Автоматически инициируемые параметры профиля VPN»

- <https://learn.microsoft.com/ru-ru/windows/security/operating-system-security/network-security/vpn/vpn-auto-trigger-profile>.
- `ms.date` 2025-01-27; commit как W05, ru snapshot updated 2026-10-17.
- Чтение 2026-10-27 13:54–13:55Z; `FULL_TEXT_READ` короткого тела.
- Capture `vpn-auto-trigger.ru.md`.
- Always On triggers; user preference; app identity triggers versus filtering.
  Актуальный реестр RasMan на устройстве **не** читался.

### W07 — Microsoft, «Функции безопасности VPN»

- <https://learn.microsoft.com/ru-ru/windows/security/operating-system-security/network-security/vpn/vpn-security-features>.
- `ms.date` 2025-01-27; commit как W05, ru snapshot updated 2026-10-17.
- Чтение 2026-10-27 около 13:57Z; `FULL_TEXT_READ` короткого тела.
- Capture `vpn-security-features.ru.md`.
- «Блокировка по VPN»: запрет отключения/изменения/удаления профиля, блокировка
  при unavailable VPN; built-in LockDown только IKEv2. Это не удостоверяет
  provider registration текущего Snowden или runtime каждого Windows build.

### W08 — Microsoft, VPNv2 CSP и embedded ProfileXML XSD

- <https://learn.microsoft.com/ru-ru/windows/client-management/mdm/vpnv2-csp>.
- `ms.date` 2025-03-12; commit
  `aeb8f2bbd5d93e85c6f2a46e2705928aa1df9c9e`, snapshot 2026-10-07.
- Чтение 2026-10-27 13:40–13:51Z; `RELEVANT_SECTIONS_READ`:
  Device/User profile roots, AlwaysOn, AppTriggerList/App/Id,
  TrafficFilterList/App/Id/RoutingPolicyType, NativeProfile/RoutingPolicyType,
  ByPassForLocal, DisableDisconnectButton, ProfileXML, embedded XSD/SyncML example.
- Captures `vpnv2.ru.html(.txt)`, `vpnv2-xsd.ru.md`.
- URL <https://learn.microsoft.com/ru-ru/windows/client-management/mdm/vpnv2-profile-xsd>
  **перенаправлен в ту же CSP-страницу**: не два независимых sources.
- Nodes applicability сохранена рядом; `ByPassForLocal` явно unsupported,
  `LockDown` XSD versus old example `Lockdown` различены; наличие поля не
  заменяет accepted native/provider contract.

## 3. Текущий repository source и actual callers

23 public inputs захвачены последовательно в `local-inputs.json` с before/copy/
after для 17 repository paths; шесть module files — отдельные scoped captures.
Это **не** атомарное дерево, не свежесть binary. Код прочитан выборочно:

| Input / pin | Read scope / смысл |
|---|---|
| `android/app/build.gradle.kts:10–15` | compile36, target36, min29; resolution/build NOT_RUN |
| `android/vpn/src/main/AndroidManifest.xml:6–18` | BIND_VPN_SERVICE, systemExempted, directBootAware=false, SUPPORTS_ALWAYS_ON=false |
| `android/.../AndroidTunFactory.kt:31–82` | include/exclude/HTTP proxy/routes rejected; full v4/optional v6; no prepare/allowBypass; establish nullable |
| `android/.../OsProtection.kt:24–46,65–79` | Own API29 facts/protect observation, nullable, age start not atomic/global |
| `android/.../VpnSessionRepository.kt:905–917` | Explicit Connect lineage prepare; passive probing не делает prepare |
| `android/.../SnowdenVpnService.kt:39–54` | START_NOT_STICKY, revoke→worker/super; not whole retirement proof |
| `android/.../WorkerProcess.kt:183,217,499–513` | strict start/check gates требуют fresh alwaysOn/lockdown; подтверждённое снятие факта ведёт к retirement, не bypass |
| `windows/go.mod:3,8,12`; scoped go.sum rows | Go1.26, sing-box1.13.19, tailscale/wf exact pseudo-version |
| `windows/wfp_killswitch_windows.go:65–126,138–180,288–350` | Static permits/AppID/LUID/catch-all; HardAction flag; own shape checks не полный foreign policy inventory |
| `windows/backend/render/render.go:147–159,570–595,613–627` | RFC1918/.local direct, DNS hijack; 172.16/12 excluded; actual worker egress не исходный application AppID |
| `windows/backend/core/worker_host_windows.go:142–149` | Default worker spawn os.Executable, AppID path same as parent; не PID isolation |
| `windows/backend/core/proxy_windows.go:36–119` | HKCU snapshot/set/restore; no expected-current compare at restore; R06-006 caveat подтверждён source, not incident |
| `tailscale/wf@6fbb0a674ee6/compose.go:129–130` | HardAction true → CLEAR_ACTION_RIGHT; false does not set flag |
| `tailscale/wf/.../firewall.go:147–158,585–619`; parse.go:290 | AppID from filename; wrapper comment vs engine semantics; not generic soft-block contract |
| `sing-box@1.13.19/protocol/direct/outbound.go:122–147` | Actual engine dialer opens direct socket, не passthrough исходного userspace socket |

Лицензии: scoped Android/AOSP headers Apache-2.0, Snowden Android GPL-3.0-or-later;
wf LICENSE — BSD-3-Clause, remote exact6fbb… файл прочитан; sing-box LICENSE
GPL-3.0-or-later из module cache. License чтение не означает function/native audit.

## 4. Provenance и неудачи получения

Own artifacts:
`.artifacts/research/theory-06/2026-10-27-mdm-corporate-res8/`.
`local-inputs.json`, `remote-equality.json`, `capture-index.json` — хэши, размеры,
даты; final preservation/drift snapshot оформлен отдельно при сдаче.

Проверена equality tag↔immutable для VpnService, Vpn, DevicePolicyManager,
DevicePolicyManagerService, UserManager, Settings AppManagementFragment;
wf compose.go remote6fbb…↔cache совпал. Не заявляется полная upstream closure,
воспроизводимая сборка, matched deployed artifact или OEM enforcement.

Отрицательные retrieval результаты (`FETCH_FAILED`, не evidence):
- WebFetch DPM и Android VPN ru timed out; заменены successful curl HTTPS
  captures, TLS verification оставлена включённой.
- candidate framework `api/api-versions.xml` — 404; added-in взят из API reference.
- Connectivity module candidate path Vpn.java — 404; правильный source найден
  в frameworks/base/services/core.
- GitHub tag API — 403; tag→commit взят из official googlesource JSON.
- AOSP root LICENSE — 404; лицензия проверена scoped file headers.
- candidate Microsoft FWP_ACTION_TYPE URL — 404; использованы W01–W03.
- candidate `vpn-always-on` page — 404; actual auto-trigger/security pages W06/W07.

Никакие TLS-ошибки не обходились отключением сертификатов; никаких ключей,
пользовательских policy/профилей, package lists, PAC/NRPT/WFP inventories не читалось.
Административные API, native/device/живые сетевые probes — **NOT_RUN**.

## 5. Новый отдельный этап 28 октября — R06-012-D, UID closure

Дата дополнения: **2026-10-28**. Область — один gap UID identity/capture/visibility/churn. Captures: `continuation-2026-10-28-uid/`; исходный evidence-срез 27 октября сохранён отдельно.

### U01 — Google/Android Developers: реальный русский package visibility

- <https://developer.android.com/training/package-visibility?hl=ru>.
- Название «Фильтрация видимости пакетов на Android»; returned page update
  **2026-10-02 UTC**; дата первой публикации unknown.
- Прочитано 28 октября 06:44Z: `RELEVANT_SECTIONS_READ` основного текста,
  capture `package-visibility.ru.html(.txt):579–596`.
- targetAPI30+ filtering, privacy списка установленных apps, automatic visibility,
  `<queries>` и restricted QUERY_ALL_PACKAGES. Не само доказательство поведения
  getPackagesForUid: оно отдельно проверено по U03.
- WebFetch timed out, curlHTTPS succeeded без выключения TLS проверки.

### U02 — Google/Android Developers: русский manifest/sharedUserId reference

- <https://developer.android.com/guide/topics/manifest/manifest-element?hl=ru>.
- Название manifest element; returned update **2026-08-19 UTC**.
- Прочитано 28 октября 06:45–06:48Z: `RELEVANT_SECTIONS_READ`
  sharedUserId/sharedUserMaxSdkVersion, capture `manifest-element.ru.txt:626–641`.
- DeprecatedAPI29 не означает исчезновение старых установок; new install
  sharedUserMaxSdkVersion API33 отличается от уже установленного shared group.
- Документ не даёт current device/package inventory; примеры чужих пакетов
  в нём не воспринимаются как реальные user данные.

### U03 — AOSP frameworks/base: PackageManager facade/cache/server

- Организация AOSP, source release `android-16.0.0_r1`, immutable commit
  `99b01a65cc4c104933788b3143285ab6bae65827` (предыдущий tag pin переиспользован;
  **новые** файлы получены по commit).
- URLs:
  - <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/content/pm/PackageManager.java>
  - <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/app/ApplicationPackageManager.java>
  - <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/core/java/com/android/server/pm/ComputerEngine.java>
- Дата source commit: 2025-05-07 (не дата public release или чтения).
- Прочитано 28 октября 06:40–06:49Z: `RELEVANT_SECTIONS_READ`, не полный аудит
  скачанных больших файлов:
  - PackageManager:7328–7341 «all packages» JavaDoc;
  - ApplicationPackageManager:1167–1191 delegates cached query to server;
  - ComputerEngine:1972–2033 cached/partial structures caveats, caller
    installed+visibility filter, SDK sandbox mapping;
  - ComputerEngine:5877–5878 getBaseSdkSandboxUid→sandbox package UID.
- Последний mapping **не** client app UID; caller фильтр и facade cache не
  устраняются фразой JavaDoc «all packages».
- Captures одноимённые `.java`, SHA256 в `external-index.json` при сдаче;
  Apache-2.0 по headers. No local toolchain/device verification.

### U04 — AOSP Process и public API sandbox mapping

- Source URL:
  <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/os/Process.java>.
- Reference <https://developer.android.com/reference/android/os/Process>.
- Google/AOSP; source commit date как U03. Public APIs **added35** по
  возвращённому API-reference; reference page publication date unknown.
- Прочитано 28 октября 06:40–06:46Z: source:955–1014, reference
  `process.reference.html.txt:8710–8724,8779–8793,8923–8933`.
- `isSdkSandboxUid/getAppUidForSdkSandboxUid` publicAPI35; opposite
  `getSdkSandboxUidForAppUid` publicAPI35. Platform `toSdkSandboxUid` помечен
  @hide MODULE_LIBRARIES — не assumed public min29 fallback.
- Ref и source одного символа не обещают runtime behaviour всехAPI29–34/OEM.

### U05 — AOSP ConnectivityService + InetDiagMessage (exact owner caller)

- Организация AOSP, module Connectivity release `android-16.0.0_r1`, commit
  **`627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`**, committer2025-04-24.
- URLs:
  - <https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/service/src/com/android/server/ConnectivityService.java>
  - <https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/staticlibs/device/com/android/net/module/util/netlink/InetDiagMessage.java>
- Прочитано 28 октября 06:42–06:48Z: `RELEVANT_SECTIONS_READ`:
  ConnectivityService:1535–1537 deps delegation,12933–12959 scope gate;
  InetDiagMessage:202–294 idiag_uid/UDP wildcard/query errors.
- Captures decoded from official googlesource TEXT base64; tag↔commit equality
  обеих файлов установлена в `connectivity-equality.json`.
- Kernel socket UID не переписывается здесь в client semantic app UID;
  visibility gate допускает current VpnService owner только для применимого
  UID. Не audited kernel/netd/eBPF/OEM/mainline update closure.

### U06 — AOSP VpnManagerService и Vpn package refresh

- URL:
  <https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/core/java/com/android/server/VpnManagerService.java>.
- AOSP source date/pin как U03; Apache-2.0 header.
- Прочитано 28 октября 06:40–06:46Z: `RELEVANT_SECTIONS_READ`
  event registration:709–711/755–764, handlers:855–911.
- `Vpn.immutable.java` **из собственного прежнего capture**, SHA256 seal
  сохранён: вновь прочитаны 1838–1851(getAppsUids),4336–4403(platform app
  exclusions), связка refresh→isIkev2VpnRunner. Это новая интерпретация
  прежних unchanged bytes, не новое device измерение.
- non-replacing add/remove refresh только platform app exclusion branch;
  не объявлено отсутствие всех остальных VpnService refresh mechanisms.

### U07 — actual Snowden и pinned sing-box metadata consumer

28 октября заново прочитаны 20selected source inputs (12repository+8module files)
и captured в `inputs.json`; перед/copy/после для названных repository paths
согласованы. На 06:47:47Z исторические 23inputs27 октября не изменились; это
проверка named paths, не всего дерева/артефакта.

| Source | Read symbols / роль |
|---|---|
| AndroidAppIdentity.kt:44–106 | Два targeted reads, sharedUserId explicit reject и names-count reject; NOT_FOUND_OR_NOT_VISIBLE сохраняется; no QUERY_ALL_PACKAGES/cross-user |
| AppIdentityMonitor.kt:79–93,194–279,355–365 | Relevant package events invalidate selected, advance epoch/re-read; 250ms coalescing не SLO/атомарность |
| AppIdentityBinding.kt:34–62 | Current UID/version/update/signers material; hash = binding identity, не protection proof |
| LibboxPlatform.kt:94–126 | getConnectionOwnerUid→packagesForConnection; сохраняет raw UID; no selected arbitrary shared member; child mismatch fault/event |
| NativePolicyLowering.kt:23–45,68–74 | candidate selectors AND Uid+Package; already blockers OWNER_CAPABILITY/PACKAGE_LIFECYCLE/parity |
| NativePolicyPredicate.kt:87–98 | Exact UID match и package-any group match; source prediction, не executed classifier |
| AndroidPolicyCompiler.kt:22–83 | ALL_APPS capture и protected DNS; direct rejected; current signer/freshness predicates |
| AndroidTunFactory.kt:31–82 | Current full capture, include/exclude package requests rejected |
| WorkerProcess.kt:365–388 | Selected lease watchdog/fault retirement; actual scheduling/native NOT_RUN |
| sing-box1.13.19 experimental/libbox/service.go:181–222 | Platform owner finder true; procfs branch disabled caller; copied raw UserId/package list |
| route/router.go:119–152 | Android platform turns needFindProcess true; process cache capacity256/lifetime200ms configured |
| route/process_cache.go:24–53 | Cache may bypass new callback; key network/source/destination, no epoch; lookup error does not supply owner |
| route/platform_searcher.go:21–44 | TCP/UDP request to platform finder |
| route/rule/rule_item_user_id.go:28–32 | Exact numeric UID equality, not parent sandbox match |
| route/rule/rule_item_package_name.go:27–36 | Match any observed package member |

Module source version сверена windows/go.mod sing-box1.13.19, **не** весь
Android AAR/ABI/compiled dependency proof. Source fetched filename+symbol,
никаких Go/Java/Kotlin builds и вызовов PackageManager в пользовательской ОС.

## 6. Границы нового этапа

Новый claim D — conditional source/caller model. Не объявляет текущий app-rule
admitted, sandbox leak, полноту UID inventory или atomic enforcement.
Нынешние safeguards и blockers названы; future-case D01–D08 NOT_RUN.
Capture UTF8/base64 extraction — только обработка публичных документов.
Большие source files/reference downloaded ≠ FULL_TEXT_READ; только названные
фрагменты analyzed. Русские primary страницы действительно получены и прочитаны,
не выведены из подстановки ru-RU. Runtime/device/OS enforcement не проверены.
