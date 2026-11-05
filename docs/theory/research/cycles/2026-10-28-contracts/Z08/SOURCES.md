# R28-Z08 — источники UID и границы Android-сопоставления

**Чтение исходного среза — 28 октября 2026 года.**
SECTIONS_READ означает только указанные части, не весь скачанный исходник.
FULL_TEXT_READ относится к небольшому файлу/helper или основному телу
короткого документа. [Аргумент](REPORT.md) · [Сводка](HANDOFF.md).

## Краткий вывод

Публичный API поддерживает лишь определённые UID, версии и области вызова.
Исходник платформы со скрытым mapper не превращает его в доступный API.
Рабочий overlay и Kotlin guards дают условный путь нового допуска,
но не доказывают установленный AAR, атомарную package-identity или завершение
каждого старого пакета. Эти уровни доказательства не смешиваются.

## 1. Публичные API

Google/Android Developers, исходное чтение 09:26–09:48Z.
Первоначальная публикация большинства API reference UNKNOWN;
версия API-added, дата обновления страницы и чтение — разные сведения.

| ID / источник | Прочитанный контракт | Глубина / копия |
|---|---|---|
| P01 <https://developer.android.com/reference/android/net/ConnectivityManager> | `getConnectionOwnerUid(int, java.net.InetSocketAddress, java.net.InetSocketAddress)`, API29, TCP/UDP, область активного VpnService, INVALID_UID/IllegalArgumentException/SecurityException. Не произвольная идентификация бизнес-приложения | SECTIONS_READ, `connectivity-reference.html(.txt)`, getConnectionOwnerUid:9300+ |
| P02 <https://developer.android.com/reference/android/os/Process> | `isApplicationUid` — API24; `isIsolated` для текущего процесса — API28; `isIsolatedUid(int)` — API34; `isSdkSandboxUid`/`getAppUidForSdkSandboxUid`/`getSdkSandboxUidForAppUid` — API35. Private-compute mapping37.2 вне выбранной области | SECTIONS_READ, `process-reference.html(.txt)`:8710,8779,8881–8933; API-added проверен по полученному телу |
| P03 <https://developer.android.com/reference/android/os/UserHandle> | `getUserHandleForUid` — API24; user context не равен kernel app UID и не выдаёт cross-user permission | SECTIONS_READ, `userhandle-reference.html(.txt)`:8369+ |
| P04 <https://developer.android.com/reference/android/content/pm/PackageManager> | getPackageInfo(String,int), getPackagesForUid, getNameForUid, GET_SIGNING_CERTIFICATES. В полученном срезе update2026-08-28 | SECTIONS_READ, `pm-reference.html(.txt)`:13474,13578,13711 и flag/signing |
| P05 <https://developer.android.com/reference/android/content/pm/SigningInfo> | API28: текущие APK signers и история ротации; history null при multiple signer. Рекомендация history в общем руководстве не заменяет отдельную цель exact-current | SECTIONS_READ, `signing-reference.html(.txt)`:8400–8463 |
| P06 <https://developer.android.com/training/package-visibility?hl=ru> | Фактически русский текст; target30+ фильтрует сведения, список apps чувствителен; queries/QUERY_ALL_PACKAGES имеют цену. Update2026-10-02 | FULL_TEXT_READ основного короткого тела, не всей навигации; `package-visibility-ru.html(.txt)`:579–596 |
| P07 <https://developer.android.com/training/package-visibility/automatic?hl=ru> | Фактически русский текст; автоматическая видимость не равна всему inventory, системный видимый набор меняется. Указанный в руководстве adb не исполнялся. Update2026-10-24 | FULL_TEXT_READ основного тела; `package-visibility-automatic-ru.html(.txt)`:579–606 |
| P08 <https://developer.android.com/guide/topics/manifest/manifest-element?hl=ru> | sharedUserId/sharedUserMaxSdkVersion; deprecation не удаляет существующую установку в прежнем основании. Новый неполный ответ не используется как полноценное основание; прежний контекст помечен отдельно, PackageInfo и guard прочитаны напрямую | FETCH_FAILED: curl56, missing close_notify. Отсутствие полного ответа не доказывает отсутствие установки/API |
| P09 <https://developer.android.com/reference/android/os/IBinder> | linkToDeath remote Binder; process death/notification; isBinderAlive true подвержен гонке, unlink не завершает native | SECTIONS_READ, `binder-reference.html(.txt)`:8495–8536,8592+; update2026-09-16 |

P01 получен по canonical ConnectivityManager без fragment.
Точная сигнатура: `(int, java.net.InetSocketAddress, java.net.InetSocketAddress)`.
API на реальном Android/профиле не вызывались.

## 2. AOSP и сравнение версий

### F01 — выбранные исходники frameworks/base Android16

AOSP, release `android-16.0.0_r1`, commit
`99b01a65cc4c104933788b3143285ab6bae65827`.
Commit date2025-05-07 — не дата публичного release.
Исходное получение 28 октября 09:28–09:48Z; headers Apache-2.0.
Immutable URL prefix:
`https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/`.
Копии получены через HTTPS aosp-mirror на raw.githubusercontent.com с тем же commit.

| Файл / строки | Используемый факт |
|---|---|
| `core/java/android/os/Process.java:883–1022` | Публичный ordinary classifier, отличие скрытых старых overloads, mapper35. Внутренняя арифметика ОС не разрешает угадывать offset на 29–34 |
| `core/java/android/os/UserHandle.java:232–248` | isApp ограничен диапазоном regular UID; false не означает system. Для приложения используется публичный Process wrapper, не hidden isApp |
| `core/java/android/content/pm/PackageInfo.java:119–130` | sharedUserId доступен в выбранном package; существующий guard читает его |
| `core/java/android/content/pm/SigningInfo.java:140–206` | Текущие signers, содержимое/порядок history. Runtime signature/crypto не выполнялись |
| `core/java/android/app/ApplicationPackageManager.java:1167–1201` | getPackagesForUid через системно инвалидируемый cache; helpers отключения/сброса @hide |
| `services/core/java/com/android/server/pm/ComputerEngine.java:1987–2033,5877–5878` | Группа packages видима caller; sandbox UID→base SDK package. Один результат не доказывает полный установленный shared group |
| `services/core/java/com/android/server/connectivity/Vpn.java:1838–1942,2064–2108,4336–4376` | Capture SDK UID closure и отдельный lockdown set; IKEv2 refresh условен |
| `services/core/java/com/android/server/VpnManagerService.java:855–911` | Refresh при некоторых add/remove и restart выбранного always-on при замене. Это не универсальное обновление VpnService Builder |

SECTIONS_READ; полного аудита ComputerEngine/Vpn не было.
Одноимённые `.java` и хеши сохранены в source index.
Наличие LICENSE headers не подтверждает весь AOSP NOTICE/состав поставки.

### F02 — ранние версии и отсутствие публичной поддержки

- <https://github.com/aosp-mirror/platform_frameworks_base/blob/android-10.0.0_r1/core/java/android/os/Process.java>:
  `Process.api29.java`, прочитаны isApplicationUid/isIsolated annotations.
- <https://github.com/aosp-mirror/platform_frameworks_base/blob/android-14.0.0_r1/core/java/android/os/Process.java>:
  `Process.api34.java:886–963`, SECTIONS_READ; sandbox classifier/mapper
  уже есть, но помечены `@hide`, `@SystemApi(MODULE_LIBRARIES)`, TestApi.

Это сравнение tag-reference; tag↔immutable commit в этом пакете не разрешён,
все варианты платформ API29–34 не проверены. Вывод о публичной поддержке
согласован с API-added P02, а не основан только на отрицательном grep.
Он не означает отсутствия любой технологии sandbox на API29.

### F03 — Connectivity: прежняя неизменяемая копия

AOSP Connectivity, commit `627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`,
android16r1; commit date2025-04-24.

- <https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/service/src/com/android/server/ConnectivityService.java>:
  12933–12959 getConnectionOwnerUid, raw UID и область того же VPN owner;
  1535–1537 delegation в InetDiag.
- <https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/staticlibs/device/com/android/net/module/util/netlink/InetDiagMessage.java>:
  202–294 idiag_uid, UDP wildcard fallback и ошибки.

Новое получение в исходном цикле завершилось DNS-ошибкой curl6.
Переиспользованы собственные immutable copies с проверенным SHA;
названные части перечитаны. `reused-immutable-sources.json` фиксирует
происхождение и предел. Это SECTIONS_READ старой копии,
не новое remote equality и не kernel/OEM runtime proof.

## 3. Проект и whole-core caller

Начальные копии 09:32:18Z, адресные добавления до 09:50Z:
37 входов (27 repository/metadata и 10 module refs), `inputs-start.json`.
Это последовательные снимки, не атомарная версия всех платформенных изменений.
Хеши не доказывают компиляцию.

| Вход / строки | Прочитанный предмет |
|---|---|
| Android app/corebridge build.gradle.kts | min29,target/compile36, без исполнения resolution |
| Android vpn Manifest | SUPPORTS_ALWAYS_ON=false, service/worker; итоговый merged manifest не установлен |
| `AndroidAppIdentity.kt:44–106` | Targeted current-user observer, sharedUserId/cardinality deny, exact-current signer, два чтения и freshness |
| `AppIdentityBinding.kt:34–62` | Исходные material pin, owner namespace/epoch/fingerprint; не OS transaction lease |
| `LibboxPlatform.kt:94–126` | Raw UID сохранён; expected rawUID/package; mismatch даёт fault/error |
| `NativeApplicationGuard.kt:12–33` | FULL_TEXT_READ малого helper; sticky invalidation исходного lease |
| `NativeCallbackGate.kt:19–35,49–84,92–111` | Pre/post local checks, без удержания monitor на JNI, phase/idle; ноль callbacks не равен смерти процесса |
| `NativeWorkerSession.kt:69–85,128–167,187–198` | Marker/tag/native-only overrides, graceful closeNativeIfIdle, пустой app binding scope; не установленный AAR |
| `NativeRuntime.kt:15–35` | Передача retirement владельцу; outcome не освобождает ресурсы автоматически |
| `AppIdentityMonitor.kt:79–93,118–164,194–303,338–360` | Epoch до чтения при relevant broadcast, регистрация, freshness/isCurrent исходного lease, без rebind |
| `WorkerProcess.kt:183–217,263–329,365–388` | Подготовка, исходный lease, beginRetirement/grace/exit, watchdog |
| `WorkerProcessLink.kt:25–59,88–111,131–150` | Точный remote Binder и death recipient; unlink observer не завершает команду |
| `VpnSessionRepository.kt:426–470,660–674,791–856` | Фактические возвраты двух RPC, владение source, смерть прежнего owner; refresh перед заменой после app failure |
| `AndroidTunFactory.kt:31–82` | Full capture; package/partial/direct-exclusion capabilities отвергнуты |
| `NativePolicyLowering.kt:23–45,68–74` | exactUID/package predicates; OWNER_CAPABILITY/PACKAGE_LIFECYCLE/parity blockers |
| `NativePolicyPredicate.kt:87–98` | Equality и any group member matcher; только возвращаемая source-модель |
| `windows/go.mod` | Go core reference sing-box1.13.19; не идентичность Android AAR |

### N01 — strict-owner adaptation на дату среза

Под `android/native/strict-owner/` прочитаны:

- `source-lock.json`, `wiring.patch`: SECTIONS_READ нужных hunks/содержимого,
  сравнение hashes; patch не применялся и не проверялся исполнением.
- `overlay/route/snowden_owner.go`: FULL_TEXT_READ75 строк;
  taggedTUN вызывает свежий platformSearcher, ошибки до пригодных metadata,
  ограниченная возвращаемая группа.
- `overlay/daemon/snowden_owner.go`: FULL_TEXT_READ59 строк;
  guards Android/oneTUN/strictAuto/noRedirect, non-JSON route property.
- `overlay/snowden_cache.go`: FULL_TEXT_READ42 строки;
  допуск persistent CacheFile, не сброс owner-cache.
- `overlay/experimental/libbox/snowden_owner.go`: FULL_TEXT_READ capability markers;
  self-revision не доказывает происхождение артефакта.
- README/ADR057: SECTIONS_READ блоков cache/acceptance.

Base sing-box b5ebaa1…, GPL-3.0-or-later. Native markers требуют whole AAR
со всеми wiring/overlays. ApprovedDigest=null и source-only — поля этого среза,
не сегодняшнее состояние сборки. Runtime inventory не получен;
build/gomobile/JNI/patch execution — NOT_RUN.

### N02 — upstream module cache и вызывающий код

`github.com/sagernet/sing-box@v1.13.19`,
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d` из source-lock среза:

- `route/route.go`:33–57 TCP errors;60–98 matchRule;
  183–225 UDP errors/match;292–295 PreMatch;403–414 место guard.
- `route/router.go`:119–152 Android finder/cache initialization.
- `route/process_cache.go`:24–53 key/result/error/metadata;
  FULL_TEXT_READ небольшого 95-строчного файла.
- `route/platform_searcher.go`:21–44 callback tuple и допустимая сеть.
- `experimental/libbox/service.go`:181–222 public-interface wrapper без mapper.
- `route/rule/rule_item_user_id.go`, `rule_item_package_name.go`:
  FULL_TEXT_READ малых equality/raw list helpers.
- `experimental/libbox/command_server.go`, `option/route.go`:
  только относящиеся forwarding/base option hunks; не все exports.
- `LICENSE`: FULL_TEXT_READ малого copyright/license statement,
  GPL-3.0-or-later.

Immutable route.go/process_cache.go совпали с module cache;
15 начальных base/overlay/wiring hash-пар также совпали.
Это не проверка каждого dependency или AAR.

## 4. Изменения во время исходного review

В 09:50:06Z параллельно изменились source-lock/wiring. Начальные копии сохранены;
`drift-review/` содержит diff и следующие снимки. SourceRevision3 добавлял
Box.Start named error и observer-owner Close; direct owner-finder/mapping
hunks не изменились. В 09:56–09:58Z изменились source-lock/README metadata:
newRetryPath вместо retryAllowed, согласование patch bytes/hash.
Patch9450bytes SHA0eaab91e… совпал с lock, шесть overlays также совпали.
Это статическая сверка ссылок/байтов, не patch apply.

На следующем срезе 10:03:02Z те же три входа изменились снова:
Start cleanup использовал постоянный E.Cause вместо raw panic/stack;
patch9812bytes/lock SHAeebc335e… сохранены в `final-cut/`.
Strict-owner routing/mapper hunks не изменились. Полная свежесть файла после
этого момента и корректность всех native lifecycle этим пакетом не заявляются.
Новый inner subscriber/Start fix за пределами mapper-вопроса.

Ограничения получения:

- Часть начальных вызовов не исполнилась или не дала файлов; отсутствие
  файла проверено до последующего адресного получения.
- VpnManagerService сначала timeout0bytes, затем успешно получен по тому же
  публичному raw-пути. Неполученный ответ не считается прочитанным источником.
- Connectivity: DNS failure; использована собственная проверенная F03-копия,
  а не объявлено новое remote equality.
- Русская manifest-страница: closed_without_close_notify;
  частичный файл не используется как новое полное основание.

## 5. Предел результата

Тесты, сборка, VPN-пробы, package/ADB/MDM действия, scans и crypto execution
не выполнялись. Source fetch и обработка HTML/SHA не являются опытом на VPN.
В исходном paired-review готового C06 HANDOFF ещё не было;
независимое принятие от этого пакета не выводилось.
`final-review.json` фиксирует source/reference/drift и локальные ссылки,
не runtime-результат. Применение, лицензии и стоимость собраны в HANDOFF.

<a id="r28-z08--sources-и-provenance"></a>
<a id="1-public-api-sources-fresh-captures-этого-цикла"></a>
<a id="2-aosp-exact-source-и-version-comparators"></a>
<a id="f01--frameworksbase-android16-selected-sources"></a>
<a id="f02--ранние-версии-как-отрицательный-public-support-comparator"></a>
<a id="f03--connectivity-module-собственный-previous-immutable-capture"></a>
<a id="3-repositorywhole-core-caller-inputs"></a>
<a id="n01--актуальная-native-strict-owner-adaptation"></a>
<a id="n02--upstream-module-cachecallers"></a>
<a id="4-concurrent-drift-и-отрицательные-получения"></a>
<a id="5-финальные-документы-и-предел"></a>
