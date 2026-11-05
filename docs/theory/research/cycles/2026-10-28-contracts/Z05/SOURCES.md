# R28-Z05 — источники, версии и границы чтения

Дата исходного чтения и сохранения — **28 октября 2026 года**.
Материалы и выбранные хеши: `.artifacts/research/r28-contracts/Z05/`.
Исполнение, сборка, устройство и сетевые пробы не выполнялись.

## Краткий вывод

Реестр поддерживает ограниченный вывод о вызывающем пути PauseManager,
его подписчиках и отдельном таймере HY2. Он не удостоверяет поставленный AAR,
своевременность callback, энергопотребление, NAT или поведение OEM.
`FULL_TEXT_READ`, `SECTIONS_READ`, `SOURCE_READ`, `LINK_ONLY`, `FETCH_FAILED`
относятся к указанной глубине исходного чтения, не к готовности продукта.

## 1. Проект и предыдущее основание

| ID | Вход | Глубина / роль |
|---|---|---|
| L01 | `android/app/build.gradle.kts` | SOURCE_READ: compile/target36, min29 и dev version; без сборки |
| L02 | `android/vpn/src/main/AndroidManifest.xml`, `SnowdenVpnService.kt`, `WorkerProcess.kt` | SOURCE_READ: systemExempted/private process/always-on false; watchdog, полномочия и завершение |
| L03 | `android/profile/ProfileClock.kt` | SOURCE_READ: elapsedRealtime projection, deadline в домене процесса/загрузки, запрет восстановления старого deadline |
| L04 | corebridge `NativeCallbackGate`, `NativeWorkerSession`, `LibboxPlatform`, `PhysicalNetworkBinding`, `PhysicalPathContext`, `ProtectedSocketControl`, `NativeEvents` | SOURCE_READ: допуск JNI/platform, выбранный путь/blocked, mailbox callback, защита socket и граница завершения |
| L05 | `android/native/source-lock.json` | FULL_TEXT_READ: sing-box1.13.19/b5ebaa…, sourceRevision3; AAR hash и Go в этом срезе null, адаптация не собрана. Начальная копия revision2; поздняя сверка зафиксировала узкий переход2→3 без изменения нужных pin/artifact полей |
| B01 | `research/04-PERFORMANCE/REPORT-011.md`, `SOURCES-011.md` | SECTIONS_READ: pause, матрица питания, таймер HY2 и неизвестные |
| B02 | `performance/QUIC-ACCOUNTING-AND-LIMITS.md` | FULL_TEXT_READ: peer timeout, различие часов/пробуждения и ресурсов/владения |
| PAIRED | `cycles/.../C04/HANDOFF.md` | FULL_TEXT_READ на стадии исходного review; использована только граница ресурсов/отмены |

`selected-inputs.json` содержит хеши 30 выбранных входов.
Это последовательный срез, не атомарный снимок всего репозитория.
Поля артефакта выше датированы; их нельзя выдавать за сегодняшнее состояние Android.

## 2. Точные upstream-версии и вызывающий код

### U01 — вызывающий путь Android

- Репозиторий: `SagerNet/sing-box-for-android`.
- Commit: `bc73504fcb858400a939140ad785ed92f4c9f429`.
- Файл: `app/src/main/java/io/nekohasekai/sfa/bg/BoxService.kt`.
- Прочитаны `idleModeUpdates`, receiver `ACTION_DEVICE_IDLE_MODE_CHANGED`,
  `serviceUpdateIdleMode`, регистрация и уничтожение.
- Сохранённый URL:
  `https://raw.githubusercontent.com/SagerNet/sing-box-for-android/bc73504fcb858400a939140ad785ed92f4c9f429/app/src/main/java/io/nekohasekai/sfa/bg/BoxService.kt`.
- SOURCE_READ, файл сохранён и хеширован.
- LICENSE того же commit прочитан целиком: GPL v3-or-later и условие о названии
  производного приложения либо указании ассоциации без согласия. Решение —
  ориентир/адаптация, без копирования кода продукта.

### U02 — создание общего менеджера и вызов libbox

- `SagerNet/sing-box` v1.13.19,
  `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`.
- `box.go`: `Box.New`, область `pause.WithDefaultManager(ctx)` до
  network/router/outbound.
- `daemon/instance.go`: менеджер context сохраняется как `Instance.pauseManager`.
- `experimental/libbox/command_server.go`: `Pause`, `Wake`, `ResetNetwork`.
- Публичные копии `box.go` и `command_server.go` совпали с локальным модулем;
  пары SHA-256 сохранены.
- GPL-3.0-or-later; SOURCE_READ и FULL_TEXT_READ лицензии.

### U03 — менеджер и конкретные подписчики

- `SagerNet/sing` v0.8.13;
  commit из tag `7c349dacf402256d3a7029746073b05d2ead584a`.
- `service/pause/default.go`: `WithDefaultManager`, `DevicePause/Wake`,
  `WaitActive`, список и вызов callbacks.
- `service/pause/timer.go`: `RegisterTicker`, остановка при pause и reset при wake.
- `common/ntp/service.go`: конкретный NTP-подписчик `RegisterTicker`.
- Прочитанные потребители sing-box:
  `protocol/group/urltest.go` — ticker;
  `transport/wireguard/endpoint.go` — Down/Up;
  `transport/wireguard/client_bind.go` — WaitActive после ошибки;
  `route/rule/rule_set_remote.go` — поле manager есть, использования
  `s.pauseManager` в выбранном файле не найдено.
- GPL-3.0-or-later; SOURCE_READ. Два публичных pause-файла сохранены и хешированы.

### U04 — HY2/QUIC и отдельный путь восстановления

- sing-box v1.13.19 `protocol/hysteria2/outbound.go`:
  `NewOutbound`, `InterfaceUpdated`, `Close`.
- sing-quic v0.6.4-…d83826c306d7 `hysteria2/client.go`:
  `offer`, `completeOffer`, `offerNew`, `CloseWithError`, `active`.
- quic-go v0.59.0-sing-box-mod.4,
  `ccd46abedd39c6f8384b1df8daf42008724958f5`:
  `connection.go` run loop, `nextKeepAliveTime`, `maybeResetTimer`;
  `send_queue.go` queue/write/close.
- Отрицательный поиск: sing-box `protocol/hysteria2`, весь выбранный
  sing-quic и quic-go; нетестовые `*.go` и токены `PauseManager`, `pause.Manager`,
  `DevicePause`, `DeviceWake`, `WaitActive`, `RegisterTicker`, `RegisterCallback`.
  Совпадений не найдено.
- Вывод ограничен отсутствием явного PauseManager-ребра в этих корнях и версиях,
  не невозможностью любого механизма паузы в будущем.
- sing-box/sing-quic GPL-3.0-or-later, quic-go MIT.

## 3. Официальные Android-материалы

Ниже — исходное получение 28 октября с обычной проверкой HTTPS/TLS,
не новый запрос и не измерение поведения устройства.

1. Google, **Optimize for Doze and App Standby**: ограничения Doze,
   foreground-service критерий App Standby и частичное power exemption.
   SECTIONS_READ:
   https://developer.android.com/training/monitoring-device-state/doze-standby?hl=en
2. Google, **Foreground service types**: допуск VPN к `systemExempted`.
   SECTIONS_READ:
   https://developer.android.com/develop/background-work/services/fgs/service-types?hl=en
3. Google, **PowerManager API**: `isLowPowerStandbyEnabled`,
   `isExemptFromLowPowerStandby`, `isIgnoringBatteryOptimizations`.
   SECTIONS_READ:
   https://developer.android.com/reference/android/os/PowerManager?hl=en
4. Google, **SystemClock API**: `uptimeMillis/System.nanoTime` и
   `elapsedRealtime`, семантика глубокого сна. SECTIONS_READ:
   https://developer.android.com/reference/android/os/SystemClock?hl=en

AOSP Android16 reference, frameworks/base, tag-resolved commit из проверенного
основания: `99b01a65cc4c104933788b3143285ab6bae65827`.
Выбранные файлы сохранены в исходном срезе:

- `core/java/android/os/SystemClock.java` — декодированы и прочитаны нужные часы;
- `services/core/java/com/android/server/net/NetworkPolicyManagerService.java` —
  выбранные foreground/Doze/AppStandby/LPS причины blocked/allowed.

SOURCE_READ; файлы в исходном срезе содержат Apache-2.0 headers.
Соответствие конкретного устройства/OEM этим исходникам не установлено.

## 4. Сильные ограничения аргумента

- Доставка SFA receiver до сна CPU не ограничена установленным верхним временем.
- `DevicePause` синхронно вызывает подписчиков под mutex; цена медленного
  или повторно входящего callback не измерена.
- HY2 `active()` проверяет local context/connDone, не жизнь peer/NAT.
- Interface update закрывает HY2; следующая операция лениво получает общий
  pending offer. Это восстановление, не доказательство экономии энергии.
- Kotlin-полномочия используют boot-local `elapsedRealtime`; справочные native
  monotime могут исключать suspend. Порядок watchdog и уже начатой Go/socket
  write после resume не доказан.
- Идентичность AAR и runtime в этом датированном срезе не установлены.
  Транспортные выводы остаются областью исходников и модели.

## 5. Сохранённые материалы, неудачный запрос и исполнение

Реестры `selected-inputs.json`, `input-drift.json`, `artifact-sha256.txt`
сохраняют исходную идентичность. Финальная сверка обнаружила только
sourceRevision2→3 в `android/native/source-lock.json`; нужные core/SFA pins,
requiredApi, неизвестные AAR/Go и статус unverified не изменились.

Сохранены SFA caller/LICENSE, sing pause manager/timer, Box/command server,
quic-go connection/LICENSE, Android-документы и выбранные AOSP-файлы.

Один запрос метаданных SFA commit через GitHub API завершился timeout:
FETCH_FAILED. Immutable commit path успешно получен и использован;
никакой вывод не зависит от непрочитанных метаданных. Обход TLS-проверки
или ограничения частоты не применялся.

**NOT_RUN:** тесты, fixtures, сборка, compiler/SDK/Gradle/Go execution,
device/adb, принудительные power modes, батарея/радио/NAT,
VPN/network endpoints, benchmarks/simulation и запрос power exemption.
Это ограничения доказательства; существование отчёта не разрешает исполнение.

<a id="r28-z05--sources-and-provenance"></a>
<a id="1-project-and-baseline-inputs"></a>
<a id="2-exact-upstream-source-pins-and-callers"></a>
<a id="u01--android-caller-reference"></a>
<a id="u02--shared-manager-construction-and-libbox-call"></a>
<a id="u03--pause-manager-and-concrete-consumers"></a>
<a id="u04--hy2quic-boundary-and-recovery-edge"></a>
<a id="3-official-android-specificationssource"></a>
<a id="4-strongest-sourcemodel-boundaries"></a>
<a id="5-public-captures--failures--execution"></a>
