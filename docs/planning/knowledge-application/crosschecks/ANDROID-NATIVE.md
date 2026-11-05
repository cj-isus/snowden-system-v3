# Android: от исходников native-ядра к одному рабочему клиенту

Анализ Android-каркаса на 28 октября. Позднейшие exact AAR/APK/UI/JNI
результаты имеют собственные границы; см. [актуальный план Android](../ANDROID-PLAN.md).
Старое отсутствие артефакта не следует переносить на новый source-срез.

**Вывод:** первый согласованный native-клиент требует одной проверяемой цепочки
исходники → AAR → trust/provider → lifecycle → устройство. **Применение:**
[Android, этапы 00–04 и 09](../ANDROID-PLAN.md). **Ограничение:** это разбор
среза 28 октября; отсутствие артефакта в нём не описывает позднейшую сборку.

## Состояние исходников на 28 октября

На срезе 28 октября Android-дерево содержит каркас владения, но **не подтверждает
готовность рабочего нативного клиента**:

- `:app` владеет одним `VpnSessionRepository`; UI/tile/widget не создают ядро.
- `SnowdenVpnService` работает в private `:vpn`, продвигается в FGS до JNI и
  передаёт lifecycle одному `WorkerProcess`.
- `WorkerProcess` допускает один start attempt на worker, удерживает unknown owner,
  разделяет control/retirement RPC lanes и требует фактическую смерть процесса перед
  заменой.
- `NativeWorkerSession` удерживает TUN/physical/app/authority owners, проверяет JNI
  steps и требует три capability revision1 до `CommandServer`:
  strict-owner, session-cache disable и closed-history disable.
- `PhysicalNetworkBinding`, `ProtectedSocketControl`, `AndroidTunFactory` и
  application-owner guard уже задают deny/retire semantics без `allowBypass`.
- `corebridge` уже имеет правильный artifact gate: независимый SHA-256 адресует одну
  verified-копию whole AAR; из неё извлекается `classes.jar` для compile, а приложение
  пакует **тот же whole AAR** через producer-owned `approvedLibbox`.

Критический разрыв находится до runtime:

1. `android/native/strict-owner` — sourceRevision3, `wiring.patch` + 6 overlays,
   но patch не применён, не собран и не проверен;
2. `android/native/source-lock.json`: `aarSha256=null`, Android Go compiler `null`;
3. stock libbox AAR несовместим с Kotlin consumers;
4. generated Java/JNI inventory отсутствует;
5. `ProfileTrustDriver` не имеет production implementation, а T01…T14 имеют статус
   **requested, SDK/API не опубликованы и не приняты**;
6. `SnowdenApplication` намеренно не создаёт fake driver/provider;
7. `NativeConfigurationCandidate.signedAuthorityVerified=false`; структурный graph
   composer не является authority;
8. APK/native/device/protected-path gates не выполнялись.

Следовательно, очередная source-фича или power callback не является кратчайшим путём.
Кратчайший путь — зафиксировать один complete source closure, получить один whole AAR
с проверяемым JNI/API contract и только затем соединить принятый профильный источник
с уже существующим lifecycle owner.

## Применение накопленных знаний

| Знание / источник | Существующий code + plan | Разница | Решение |
|---|---|---|---|
| Один владелец VpnService/native/TUN; coroutine timeout/Close не death (`PLATFORM-AND-UX` §12.2, C04 lifetime units) | `VpnSessionRepository` → `WorkerProcess` → `NativeWorkerSession`; retained Binder/RPC/source/death fences. AND-02/03, A-FR-01/02 | Runtime/JNI/process-death behaviour не выполнено; неизвестный owner может сделать систему недоступной, но не разрешает второй core | **Reuse, не переписывать.** Один Android platform owner реализует последовательность; не дробить lifecycle и native core между независимыми исполнителями |
| Whole libbox1.13.19 + узкий bridge, не SFA HEAD (`PLATFORM-AND-UX` §12.7/§15.5) | `source-lock.json`, strict-owner revision3, `NativeWorkerSession` version/capability checks. AND-01 | Current patch/API exists only as source; stock AAR lacks markers/wiring | **Adapt current closure only.** Применить exact patch/overlays к b5ebaa1…, не обновлять core и не копировать SFA service wholesale |
| Hash/marker не доказывает artifact (`R04-011`, Z05, Z08 late source identity) | `verifyLibboxArtifact`, approved-SHA-qualified output, same AAR/JAR source. AND-01-S3/S4, ADR-023/061 | Нет approved digest, generated API inventory, ABI/16KiB/toolchain/SBOM receipt | **Complete existing gate**, не создавать второй pipeline |
| Signed profile authority отдельно от structural graph (`PLATFORM-AND-UX` §12.5, trust units) | `ProfileTrustDriver` mandatory; `ProfileImportCoordinator`; `NativeConfigurationComposer`; `RuntimeSourceProvider`. AND-04/06, A-FR-03/05 | Production driver и accepted-only provider отсутствуют; T01…14 только requested | **Dependency blocker.** Trust owner сначала публикует accepted API/source revision. Android интегрирует его в **тот же whole AAR**, затем реализует driver/provider; до этого Connect остаётся disabled |
| Authority clock включает suspend; transport clock/peer state — другой контракт (`R04-011`, Z05/C04, QUIC accounting) | `ProfileClock`, `authorityValidUntilElapsedMs`, `NativeCallbackGate`, worker watchdog. AND-02/04/11 | Java/JNI gates не дают per-packet atomic cutoff; accepted Go/AAR clock неизвестен | **Reuse current expiry/retirement contract.** Не вводить transport-derived renewal. Device acceptance отдельно проверяет in-flight boundary и resume/reboot |
| SFA Doze→PauseManager не достигает HY2 timer (R28-Z05) | Current Snowden вообще не экспортирует Pause/Wake; physical blocked/context fault ведёт к retirement. AND-05/11 | Нет wake/radio/NAT outcome и нет HY2 subscriber edge | **Reject direct pause feature.** Optional NTP/URL-test pause допустим только внутри exact core consumers; HY2 оставлять как есть. Path invalidation использует existing retire/recreate, same-path resume — conditional reuse |
| HY2 due/queue/write/peer, peer timeout и retention — разные receipts (`R04-011`, `QUIC-ACCOUNTING`, C04) | Pinned sing-quic/quic-go source, existing queue caps; Android private worker owner. AND-02/07/11 | Нет complete internal admission/resource ledger; session cancel может не прервать waiting `Add`; blocked write/reap bounds неизвестны | **Defer transport changes до принятого artifact.** Сначала preserve worker owner and retirement reserve; затем один scoped upstream/adapter task, без timeout-goroutine с ранним Release |
| Cache и closed-history retention не нужны Android app (`ADR-057/058`) | Native-only `SnowdenDisablePersistentCache` и `SnowdenDisableClosedConnectionHistory`; Kotlin всегда требует/задаёт оба. AND-11 | Marker/wiring не artifact; active trackers/events/other native IO остаются | **Reuse current revision3.** Приёмка проверяет propagation, wrong scope, no cache.db, active-event parity; не обещает RSS/secure wipe |
| Observer admission и startup error должны пережить cleanup (`ADR-059/060`) | sourceRevision3 закрывает Observer owners; `Box.Start` сохраняет start error при cleanup panic | Patch/JNI/runtime paths не проверены; Close return не worker death | **Include in same AAR receipt.** Нельзя одобрять older revision2 AAR только потому, что JNI markers прежние |
| APK install/signing/adoption различаются (`R05-004`) | AND-14 planned; current app versionCode1/dev, release minify false | Whole APK/splits/signer/lineage/update/adoption отсутствуют | **После native+trust closure.** PackageInstaller/signing/readiness — отдельный release gate, не часть source SHA approval |
| Native backup/Keystore не authority (`R05-008`) | CE/no-backup encrypted store, `ProfileClock`, trust floors. AND-04/13 | Restore/re-enrollment/server floors и production trust driver отсутствуют | **Keep recovery out of first native activation.** No directBoot/always-on until accepted offline recovery; no trust reset on reinstall |
| Battery/speed benefit нельзя вывести из keepalive/pause/source (`R04-011/Z05/C04`) | AND-11/15/16 gates; no wake lock/exemption; source only | Нет actual AAR, device/OEM/RAT/NAT/useful-task measurements | **No preset/claim.** Instrument only after protected client works; thresholds preregistered, missing outcomes retained |

## Минимальная последовательность одного согласованного native artifact

### Этап 0. Зафиксировать входной контракт

**Владелец:** Android platform owner + trust owner как поставщик принятого portable API.
**Пункты плана:** AND-01/04, A-FR-03/13.

Зафиксировать один source manifest:

- sing-box v1.13.19, commit `b5ebaa1…`;
- вариант strict-owner `owner-r1-session-cache-r1-closed-history-r1`, sourceRevision3;
- все хеши исходных файлов, `wiring.patch` и 6 overlays;
- зависимость sing v0.8.13 и закреплённые версии выбранных транспортов;
- **принятую** версию trust API и его исходников по T01…T14;
- Go/gomobile/NDK/AGP/Kotlin/JDK, флаги сборки и набор ABI;
- соответствующие исходники, уведомления о лицензиях и входной граф SBOM.

Stop condition: пока T01…14 не опубликованы/приняты, допустима подготовка native
build recipe, но нельзя фиксировать final artifact digest или включать Connect.
Trust module добавляется в существующий gomobile bind/package list и один Go runtime;
отдельный trust AAR/второй Go runtime запрещён.

### Этап 1. Применить revision3 и подтвердить соответствие исходников

**Owner:** один Android native/platform owner.
**Plan:** AND-01, AND-11; ADR-057…060.

В чистом checkout exact base:

1. сверить каждый base SHA до patch;
2. применить **весь** `wiring.patch` и все overlays;
3. проверить destination inventory, отсутствие лишних/пропущенных generated inputs;
4. зафиксировать post-apply source hashes и sourceRevision3;
5. убедиться, что revision3 включает cache/history/observer/start-error изменения как
   один closure, а не набор marker-only файлов;
6. включить принятый trust package и narrow JNI exports в тот же manifest.

**Будущая приёмка, NOT_RUN:** применимость патча, форматирование и компиляция;
сравнение с манифестом; отклонение при пропущенном или изменённом фрагменте,
при старой revision2 или исходниках без требуемых изменений.

### Этап 2. Собрать один AAR и сверить сгенерированный API

**Owner:** тот же Android native owner; security/release reviewer независимо выдаёт
approved SHA.
**Plan:** AND-01, AND-14; ADR-023/061.

Свидетельство сборки должно включать:

- точные версии Go/ветки gomobile/NDK/JDK, флаги, теги и граф модулей;
- SHA-256 всего AAR и хеш манифеста соответствующих исходников;
- перечень сгенерированных Java-классов и JNI-символов;
- ожидаемое значение `Libbox.version()==1.13.19`;
- три метода-маркера revision1 и три поля OverrideOptions;
- принятую версию trust API, методы и правила владения DTO;
- набор ABI (`arm64-v8a`, при необходимости x86_64 для лаборатории),
  выравнивание ELF/ZIP и совместимость со страницами 16 KiB;
- отсутствие второго Go runtime и native-библиотеки с конфликтующими символами;
- лицензии, SBOM, уведомления и материалы для предоставления исходников.

Существующее условие допуска в Gradle сохраняется: независимо одобренный хеш —
входной параметр. `corebridge` компилируется с `classes.jar`, извлечённым из
одобренного AAR, а `:app` пакует тот же полный AAR. Кандидат не одобряет собственный хеш.

**Существенное ограничение:** совпадение API и маркеров необходимо, но недостаточно.
Неполный или подменённый AAR тоже может вернуть ожидаемые номера версий.
Свидетельство сборки должно связывать полный состав исходников, сгенерированный API,
native-библиотеки и независимо одобренные байты всего артефакта.

### Этап 3. Соединить принятый профиль с выполнением

**Владелец:** Android profile/integration после принятия зависимости trust.
**Пункты плана:** AND-04/06, A-FR-03/05.

В существующей архитектуре нужны два Android-адаптера:

1. JNI-адаптер `ProfileTrustDriver` к принятому API T01…T14:
   - синхронный вызов с принадлежащими вызывающей стороне байтами;
   - полная проверка TUF, полномочий приложения, progress, нижних границ, отзыва и часов;
   - типизированные результаты без чувствительных данных;
   - отсутствие неявного доступа к ФС, сети и глобальному состоянию;
   - срок полномочий отдельно от TTL подготовки.
2. `RuntimeSourceProvider`, создающий runtime-источник только из принятого состояния:
   - читает принятый checkpoint и действующую политику;
   - составляет кандидат через `NativePolicyPlan` и `NativeConfigurationComposer`;
   - отклоняет любой `admissionBlocker` и обход через `signedAuthorityVerified=false`;
   - связывает точные хеши конфигурации/политики, требуемые native revisions, TUN tag,
     владельцев сети/приложения, epoch/fingerprint, worker/request и исходный срок полномочий;
   - возвращает один локальный для родителя `IPreparedRuntimeSource` для однократного потребления.

Прямое создание из preview/JSON/UI/Intent и фиктивный успешный результат исключены.
`SnowdenApplication` может открыть импорт/подключение после установки рабочих
driver/provider и проверки соответствия источника одобренному AAR.

### Этап 4. Сохранить управление жизненным циклом, ошибками и ресурсами

**Владелец:** Android service/native; отдельная команда второго контроллера не требуется.
**Пункты плана:** AND-02/03/05/06/11; A-FR-01/02/04/05/10.

Сохраняются следующие контракты:

- один репозиторий состояния приложения, один private worker и одна native-попытка на процесс;
- перевод в FGS до профиля/JNI, типизированная ошибка запуска foreground service;
- один владелец исходного TUN; заимствованный дубликат сокета не закрывает fd вызывающего;
- проверки физической сети, приложения и полномочий у каждого JNI/OS шага;
  терминальная ошибка закрывает допуск до отложенной очистки;
- отсутствие command listener/hot reload; Start/TUN сами по себе не публикуют Ready;
- Stop/Close/тайм-аут coroutine не означают смерть: замена ждёт Binder death,
  возврата обоих RPC, завершения тела источника/Stub и очистки монитора родителя;
- неопределённые владельцы FD/native/observer удерживаются до выхода процесса;
- исходные конфигурации, домены, пакеты и тексты ошибок не проходят в состояние/логи/поддержку;
- отключение постоянного кеша и closed-history требуется для каждого worker;
- отсутствие direct/`allowBypass`/plaintext fallback.

Ресурсные изменения следует ограничить необходимым: сначала проверить удержание
владельцев revision3 и существующие фиксированные линии вызовов. Отмена общего
carrier/session и учёт ресурсов из C04 — последующая задача владельца транспорта.
Первое подключение AAR не зависит от обновления quic-go, смены CCA или нового подписчика pause.

### Этап 5. Проверить платформенную интеграцию

**Owner:** Android platform owner.
**Plan:** AND-02/03/05/06/07/11.

Последовательность будущей приёмки; всё `NOT_RUN`:

1. Сгенерированные вызовы Kotlin/Java/JNI возвращают ожидаемые версии и типизированные ошибки.
2. Согласие, допустимость FGS, отзыв и destroy проверены на API29/API34+/target36.
3. TUN establish/protect/bind сохраняют исходного владельца PFD, включая ошибки.
4. Strict-owner действует для TCP/UDP/PreMatch/DNS/fakeIP/ICMP/фрагментов; неизвестный UID отклоняется.
5. Физический контекст проверяется до TUN/socket; blocked/lost/change завершают старое владение
   управляемо и не включают direct fallback старого пути.
6. Истечение/отзыв профиля учитываются при блокирующем JNI, позднем callback и позднем PFD.
7. Ошибка запуска сохраняет ненулевую типизированную причину при panic/hang очистки, без сырых диагностик.
8. Флаги cache/history отклоняют неверные scope/options; проверяются отсутствие cache DB
   и сохранение активных tracker/event.
9. Проверены поздний Subscribe/Close, обратное давление observer и фактическая смерть worker.
10. Проверено завершение родителя/потомка при заблокированном JNI/Binder/raw write и неопределённом владельце.
11. До `ready` требуется принятое наблюдение защищённого пути; STARTED/handshake/иконка ключа его не заменяют.

### Этап 6. Проверить артефакт, пакет, устройство и выпуск

**Owner:** Android release/QA; dependencies AND-01…14.
**Plan:** AND-14/15/16/17, A-FR-13…16.

- Подписанный полный APK/набор split из одобренного AAR: история подписанта,
  versionCode, SBOM, лицензии, соответствующие исходники и идентичность канала.
- Результат установки отдельно от принятия установленного клиента и готовности VPN.
- Матрица устройств: API29/API34+/API36, выбранные OEM/patch, 4/16 KiB, arm64 и лабораторный x86_64.
- Матрица отказов: отказ в согласии/уведомлениях, always-on/lockdown, другой VPN,
  смерть UI/native/процесса, reboot/force-stop/pre-unlock, сеть/Doze/LPS/политика OEM.
- Проверки защищённого пути и приватности предшествуют заявлениям о скорости.
- Полевое сравнение в России и замеры энергии — после AND-15 и явного разрешения live-проверок.
- RC→canary→release — после закрытия P0 и условий rollback/поддержки.

## Минимальные межкомпонентные контракты

### Идентичность артефакта

```text
NativeArtifactIdentity {
  baseCommit,
  sourceRevision = 3,
  overlay/wiring manifest hash,
  trustApiRevision,
  toolchain + tags + ABI,
  generatedApiInventoryHash,
  wholeAarSha256,
  correspondingSource/SBOM hash
}
```

Одного возвращаемого маркера или хеша всего AAR недостаточно для выполнения этого контракта.

### Принятый источник выполнения

```text
AcceptedRuntimeSource {
  acceptedCheckpoint identity + authority fence,
  config/policy digests,
  native required revisions,
  network owner/epoch/context,
  app owner/epoch/pins,
  worker/request identity,
  preparation deadline,
  independent authority deadline
}
```

Источник локален для родителя, потребляется однократно и не раскрывает чувствительные
данные. Он удерживается до возврата producer и завершения формирования Binder-ответа.
Переподключение или новый worker не продлевают полномочия.

### Владение native-ресурсами

```text
NativeOwnerReceipt {
  process/session incarnation,
  original TUN/PFD owner,
  gate phase + in-flight JNI/callback counts,
  control and retirement RPC completion,
  prepared source sealed+idle,
  parent monitor cleanup,
  actual Binder/process death
}
```

Штатный `Close` и отсутствие локальных callbacks — промежуточные факты;
они сами по себе не разрешают освободить все ресурсы владельца.

### Наблюдение пути, энергии и транспорта

```text
PathObservation {
  boot/process/network generation,
  physical fingerprint + blocked/suspended facts,
  authority validity,
  native session owner,
  outcome = unknown | carrier_failed | protected_useful_completion
}
```

В изученном pin SFA PauseManager не достигает таймера HY2. Поэтому общий обработчик
pause не входит в первый артефакт. Возобновление в том же контексте условно допускает
повторное использование carrier; недействительный контекст или типизированный отказ
carrier используют существующего владельца завершения/пересоздания.
Выигрыш для батареи, радиомодуля и NAT остаётся неизвестным до приёмки AND-15/16.

## Контрпримеры и зависимости

- **«Патч уже реализует рабочий клиент».** На срезе 28 октября sourceRevision3 и маркеры
  имеют `artifactProof=false`; AAR и перечня Go/compiler/JNI для его сборки нет.
- **«Независимого SHA достаточно».** Хеш связывает одобренные байты, но не доказывает
  соответствие исходникам, generated API, ABI/16 KiB и корректность trust.
- **«Достаточно stock AAR и проверок Kotlin».** В этом срезе stock release не содержит требуемых API/wiring.
- **«Соберём native, trust добавим позже».** Промежуточная сборка может проверить patch/JNI,
  но не становится финальным runtime-артефактом и не открывает Connect.
  Полный состав финального AAR включает принятый trust module/API.
- **«Отдельный trust AAR проще».** Он создаёт второй Go runtime и границы зависимости,
  ABI и lifecycle вопреки T14 и существующему единому владельцу.
- **«SFA pause/wake экономит батарею HY2».** Изученный вызов не имеет HY2-подписчика;
  результат на устройстве также не получен.
- **«На каждом resume надо закрывать carrier».** Без отказа/смены пути это не обосновано
  и может увеличить число handshake, задержку и расход энергии. Существующего пути
  отзыва контекста достаточно как исходного решения.
- **«По тайм-ауту можно освободить ресурсы».** Заблокированный JNI/Binder/UDP/raw write
  может удерживать владельцев: slot/process/source сохраняются до возврата или смерти.
- **«Каждому слою нужен независимый разработчик».** До фиксации контрактов native patch,
  generated JNI, Kotlin bridge, trust adapter, provider и service разделяют один артефакт
  и жизненный цикл. Независимый разбор неизменяемых документов/fixtures/выпуска допустим
  после фиксации source/API; интеграцией управляет один основной владелец Android.

## Будущая приёмка и неизвестное

Все перечисленные проверки остаются `NOT_RUN`:

- воспроизводимость патча, сборки, API/ABI, toolchain и артефакта;
- принятие рабочего trust API и соответствие семантике T01…14;
- владение данными, ошибки, отмена и приватность JNI;
- жизненный цикл, смерть процесса, гонки FD/IPC и пределы прекращения работы;
- фактические writers транспорта, очереди, отмена, ресурсные пределы и задержка завершения;
- поведение API/OEM/patch/Doze/LPS/reboot/force-stop/lockdown;
- защищённый выход и завершение полезной задачи;
- энергия, скорость, RSS, FD, пробуждения и NAT на устройстве;
- установка/обновление/принятие подписанного пакета, откат и поддержка выпуска.

Неизвестные входы остаются условиями будущей приёмки; их нельзя считать отказом
или заменять нулём. Из описанного source-среза не следуют выигрыш скорости/батареи,
готовность runtime или дата выпуска.

## Что не стоит делать

- Не писать второй VPN controller, второй TUN/native owner или второй Go AAR.
- Не переносить SFA HEAD/whole service или добавлять another transport during parity.
- Не включать pause/wakelock/power exemption/keepalive preset без device outcome.
- Не создавать Kotlin signature/TUF canonicalizer или принимать parsed JSON as trust.
- Не подменять accepted AAR marker/hash иконкой VPN, STARTED или handshake.
- Не добавлять broad package inventory, direct fallback, `allowBypass`, raw native logs.
- Не блокировать первый coherent artifact UI-параллелизмом, store submission или
  optional performance work; сначала source→artifact→provider→lifecycle→device gate.

## Источники и трассируемость

Основные материалы:

- владение Android и платформа: `PLATFORM-AND-UX` §§12.2/12.3/12.5–12.7/13.8/15.5;
- энергия/HY2: R04-011 REPORT/SOURCES/HANDOFF;
- владение и время QUIC: `QUIC-ACCOUNTING-AND-LIMITS` §§4–7;
- вызов pause: R28-Z05 REPORT/SOURCES/HANDOFF;
- ресурсы/отмена: R28-C04 REPORT/HANDOFF;
- артефакт/принятие/восстановление: R05-004 §§4/6–8, R05-008 §§2–4;
- поздняя сверка strict-owner: Z08 SOURCES, разделы N01/N02/изменений среза.

Исходники и пункты планов:

- `android/native/{README.md,source-lock.json,strict-owner/}`;
- `android/{corebridge,profile,vpn,app}`;
- `android/corebridge/build.gradle.kts`, `android/app/build.gradle.kts`;
- `docs/android/CHANGE-REQUEST-TRUST-001.md`;
- `android/STATE.md`, `docs/ANDROID-PLAN.md` (read-only current status).

Это сопоставление относится к выбранным знаниям и последовательным снимкам исходников
28 октября; оно не доказывает атомарность общего среза или поведение позднейших сборок.
Актуальные артефакты и их приёмка проверяются по плану Android.

<a id="android-native-от-source-кандидата-к-одному-рабочему-клиенту"></a>
<a id="краткое-состояние-кода"></a>
<a id="stage-0--заморозить-входной-contract-не-собирать-два-ядра"></a>
<a id="stage-1--применить-revision3-и-получить-source-parity-receipt"></a>
<a id="stage-2--собрать-один-whole-aar-и-доказать-generated-api-parity"></a>
<a id="stage-3--connect-accepted-profile-to-runtime-without-bypass"></a>
<a id="stage-4--retain-existing-lifecycleerrorprivacyresource-controls"></a>
<a id="stage-5--platform-integration-gates-before-working-vpn"></a>
<a id="stage-6--artifactpackagedevicerelease-evidence"></a>
<a id="frozen-cross-component-contracts"></a>
<a id="artifact-identity"></a>
<a id="accepted-runtime-source"></a>
<a id="native-ownership"></a>
<a id="pathpowertransport-observation"></a>
<a id="strongest-opposing-guards-and-dependencies"></a>
<a id="future-acceptance-and-remaining-unknowns"></a>
