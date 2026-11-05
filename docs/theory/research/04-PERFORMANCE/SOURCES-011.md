# R04-011 — источники: Android power modes, VPN FGS и HY2 keepalive

Срез источников: 27 октября 2026. Последняя внешняя запись `15:02:13Z`;
captures и manifests:
`.artifacts/research/theory-04/2026-10-27-android-energy-res5/`.

Статусы ниже обязательны: `FULL_TEXT_READ` — тело прочитано; `SOURCE_READ` —
прочитаны названные symbols/границы исходника; `REFERENCE_SOURCE` — выбранный
source reference без доказательства shipped artifact; `FETCH_FAILED` — материал
не использован как источник.

## 1. Публичные Android primary

| ID | Организация / документ / дата | Прочитанная область и конкретный вывод | Статус / URL |
|---|---|---|---|
| P01 | Google, Android Developers, **Optimize for Doze and App Standby**, обновлено 2026-10-22 UTC; чтение 2026-10-27 | `Understand Doze`, restrictions, `Understand App Standby`, exemptions. Общий Doze откладывает CPU/network/jobs; foreground service не даёт App Standby считать приложение idle; power allowlist частичный: network + partial wakelocks, но не все ограничения. RU-страница прямо предупреждает об AI-переводе, поэтому ключевые формулировки сверены с EN | `FULL_TEXT_READ` RU+EN: [RU](https://developer.android.com/training/monitoring-device-state/doze-standby?hl=ru), [EN](https://developer.android.com/training/monitoring-device-state/doze-standby?hl=en) |
| P02 | Google, Android Developers, **Foreground service types**, обновлено 2026-10-24 UTC; чтение 2026-10-27 | `systemExempted`: Android14+ type/permission; VPN app, configured in Settings, is an eligible category; otherwise `ForegroundServiceTypeNotAllowedException`. This is FGS-type eligibility, not Doze/LPS exemption | `FULL_TEXT_READ` RU+EN: [RU](https://developer.android.com/develop/background-work/services/fgs/service-types?hl=ru), [EN](https://developer.android.com/develop/background-work/services/fgs/service-types?hl=en) |
| P03 | Google, Android Developers, **Power management resource limits**, обновлено 2025-07-29 UTC; чтение 2026-10-27 | Tables `device state`, `app state`, `standby bucket`, Android16 changes. FGS row: network unrestricted but jobs/alarms remain bucket-dependent; Android16 quotas JobScheduler/WorkManager even with FGS. Values are guidance, not execution guarantees | `FULL_TEXT_READ` RU+EN: [RU](https://developer.android.com/topic/performance/power/power-details?hl=ru), [EN](https://developer.android.com/topic/performance/power/power-details?hl=en) |
| P04 | Google, Android Developers, **App Standby Buckets**, обновлено 2026-10-24 UTC; чтение 2026-10-27 | Long-running FGS is active-bucket criterion; VPN apps are listed among exemptions from restricted-bucket inactivity trigger; OEM may choose bucket criteria. This does not establish Doze/LPS network or timer behavior | `FULL_TEXT_READ` RU+EN: [RU](https://developer.android.com/topic/performance/appstandby?hl=ru), [EN](https://developer.android.com/topic/performance/appstandby?hl=en) |
| P05 | Google, Android Developers, **Optimize network access**, обновлено 2026-10-24 UTC; чтение 2026-10-27 | Radio state/tail and batching model. Numeric tail/transition example is explicitly a typical AT&T **3G** model; technology/carrier determine LTE/5G behavior. It cannot yield Snowden mAh or radio-tail savings | `FULL_TEXT_READ` RU: [link](https://developer.android.com/develop/connectivity/network-ops/network-access-optimization?hl=ru); AI-translation warning recorded |
| P06 | Google, Android Developers, **Power Profiler**, обновлено 2026-03-06 UTC; чтение 2026-10-27 | ODPM rails: Android10+, Pixel6+; rails vary; ODPM is device-level, not app-only, therefore contains other-app/device noise. Non-ODPM charge/current gauges are not equivalent rails | `FULL_TEXT_READ` RU: [link](https://developer.android.com/studio/profile/power-profiler?hl=ru) |
| P07 | Google, Android Developers, **PowerManager API**, reading 2026-10-27 | `isLowPowerStandbyEnabled` (API33): outside maintenance, noninteractive LPS can disable network and ignore wake locks **including foreground-service apps**. `isExemptFromLowPowerStandby` and policy reasons API34. `isIgnoringBatteryOptimizations` is a different power allowlist and still has extreme guardrails | `FULL_TEXT_READ` relevant API entries, RU: [link](https://developer.android.com/reference/android/os/PowerManager?hl=ru) |
| P08 | Linux man-pages project, **clock_gettime(2)**, reading 2026-10-27 | `CLOCK_MONOTONIC` does not count suspend; `CLOCK_BOOTTIME` does. Used only to interpret exact Go reference below, not shipped Android artifact | `FULL_TEXT_READ` relevant sections: [link](https://man7.org/linux/man-pages/man2/clock_gettime.2.html) |

## 2. Pinned AOSP Android 16 source

Tag `android-16.0.0_r1` independently resolved on 2026-10-27 to:

- `platform/frameworks/base`, commit
  `99b01a65cc4c104933788b3143285ab6bae65827`.
- `platform/packages/modules/Connectivity`, commit
  `627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`: указан как граница источника;
  нижеследующие выводы не опираются на непрочитанный Connectivity-файл.

Выбранные frameworks/base-файлы прочитаны по immutable commit;
их headers указывают Apache-2.0.

| ID | Файл / symbols | Source-факт и граница |
|---|---|---|
| A01 | `core/java/android/net/NetworkPolicyManager.java`: `FOREGROUND_THRESHOLD_STATE`, `isProcStateAllowedWhileIdleOrPowerSaveMode` | Foreground threshold is `PROCESS_STATE_BOUND_FOREGROUND_SERVICE`; allowed in idle/power-save at or above that importance or with capability. Actual Snowden process state is device evidence, not inferred solely from manifest |
| A02 | `services/core/java/com/android/server/net/NetworkPolicyManagerService.java`: `updateRulesForPowerRestrictionsULInner`, `UidBlockedState.getEffectiveBlockedReasons` | Foreground reason clears Battery Saver, Doze and App Standby blocked reasons; TOP clears Low Power Standby. LPS allowlist is separate. Therefore an actual FGS UID and `systemExempted` FGS type are not a universal power exemption |
| A03 | `services/core/java/com/android/server/power/PowerManagerService.java`: `setWakeLockDisabledStateLocked` | Idle partial-wakelock guard exempts process state through `BOUND_FOREGROUND_SERVICE`; LPS uses a stricter TOP/allowlist check. This says when an existing partial wake lock is ignored; it does not create one for an FGS |
| A04 | `core/java/android/app/ForegroundServiceTypePolicy.java`: `FGS_TYPE_POLICY_SYSTEM_EXEMPTED`, `AppOpPermission` | Type requires `FOREGROUND_SERVICE_SYSTEM_EXEMPTED` plus one accepted eligibility signal, including allowed `OP_ACTIVATE_VPN`. The type name is unrelated to `isExemptFromLowPowerStandby` |
| A05 | `services/core/java/com/android/server/connectivity/Vpn.java`: `startAlwaysOnVpn` | Always-on startup gets a **temporary 60s** device-idle allowlist for bootstrap. This is not persistent VPN/QUIC exemption; Snowden declares `SUPPORTS_ALWAYS_ON=false` |
| A06 | `apex/jobscheduler/.../AppStandbyController.java`: `getAppMinBucket` | Power allowlists/admin/app-op can lower the minimum bucket; exact VPN exemption mapping was not traced end-to-end here. Public P04 is the basis for the VPN restricted-bucket statement |
| A07 | `core/java/android/os/SystemClock.java` | `uptimeMillis`/`System.nanoTime` stop in deep sleep; `elapsedRealtime` includes it. Existing Snowden authority clocks use the latter; native QUIC reference uses the former family |

AOSP-копии и tag records: `aosp-*.b64/json`; декодированные файлы чтения —
`aosp-*.read.txt`. Это разбор исходников, не подтверждение соответствия
конкретного Android-устройства.

## 3. Snowden Android current source inputs

Выбранные входы скопированы в `local/` с SHA-256. Перечень и совпадение
байтов после копирования — в `selected-source-manifest.json`.

| ID | Current source | Fact / boundary |
|---|---|---|
| L01 | `android/app/build.gradle.kts` | `compileSdk=36`, `minSdk=29`, `targetSdk=36`, version `0.1.0-dev` |
| L02 | `android/vpn/src/main/AndroidManifest.xml` | `FOREGROUND_SERVICE` + `FOREGROUND_SERVICE_SYSTEM_EXEMPTED`; VPN service in `:vpn`, BIND_VPN_SERVICE, `systemExempted`, directBoot=false, stopWithTask=false, always-on support=false |
| L03 | `SnowdenVpnService.kt`, `VpnNotification.kt` | `startForeground` occurs in `onCreate` before `WorkerProcess.attach`/JNI work; type passed on API34+; on failure process retires; `START_NOT_STICKY`; no claim that notification display or start request is protection receipt |
| L04 | `VpnSessionRepository.kt` (read-only input, not copied in selected manifest) | `startForegroundService` result is explicitly only accepted request, not host attachment/start completion |
| L05 | `NativeRuntime.kt`, `NativeWorkerSession.kt`, `NativeCallbackGate.kt` | Current facade has no public `Pause/Wake` path; constructs CommandServer but intentionally does not start its control server. Authority deadlines use `SystemClock.elapsedRealtime` and are rechecked around lifecycle/callback steps |
| L06 | `PhysicalNetworkMonitor.kt`, `PhysicalNetworkBinding.kt` | Observes `onBlockedStatusChanged`; blocked path is not eligible. Delivery/causality under Doze/LPS remains device acceptance, not source proof |
| L07 | project-wide Android search | No `WakeLock`/`WAKE_LOCK`, power-allowlist request, `isIgnoringBatteryOptimizations`, Doze receiver, or Low Power Standby call in current Android source. Negative search scope: current `android/` Kotlin/XML/Go source only |
| L08 | `android/native/source-lock.json`, strict-owner lock | Core source candidate sing-box v1.13.19 / b5ebaa…, but AAR SHA-256 and Android Go compiler are null; adaptation source-only, not applied/built/verified. Therefore Go/quic timing below is a reference model, not attested APK behavior |

## 4. QUIC/HY2 source model

Манифест sing-box v1.13.19 закрепляет quic-go
`v0.59.0-sing-box-mod.4`, sing-quic
`v0.6.4-0.20260803041914-d83826c306d7`, sing v0.8.13. Записаны даты module `.info`
и локальные исходники. В первоначальном срезе три критичных файла получены
по immutable commit и побайтово совпали с module cache:

- quic-go `connection.go` — `ccd46abedd39c6f8384b1df8daf42008724958f5`;
- sing-quic `hysteria2/client.go` — `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`;
- sing-box `experimental/libbox/command_server.go` —
  `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`.

| ID | Source / licence | Прочитанный caller и предел |
|---|---|---|
| U01 | sing-box v1.13.19, GPL-3.0-or-later; `protocol/hysteria2/outbound.go` | `NewOutbound` builds sing-quic `ClientOptions`; no keepalive option exposed by `Hysteria2OutboundOptions` |
| U02 | sing-quic d838…, GPL-3.0-or-later; `hysteria2/client.go`, `hysteria/protocol.go` | `NewClient`: `MaxIdleTimeout=30s`, `KeepAlivePeriod=10s`; UDP `DialContext` → unbound PacketConn → optional Salamander → qtls transport. `ClientOptions` does not expose period |
| U03 | quic-go ccd46…, MIT; `connection.go` | Negotiated `idleTimeout=min(local, peer if nonzero)`; `keepAliveInterval=min(config period, idle/2)`; due=`lastPacketReceived + max(interval, 3/2 PTO)`; no due while PING flag set; blocked path may arm idle deadline; due queues PING and any received packet resets state |
| U04 | quic-go ccd46…; `send_queue.go`, `send_conn.go`, `sys_conn.go` | PING intent reaches bounded queue (capacity8), then raw PacketConn/UDP write. Timer due/queue acceptance/socket write are separate events. Salamander adds its packet transform outside quic-go |
| U05 | quic-go `internal/monotime/time.go` | QUIC monotime is `time.Since(start)`. It is process-relative and cannot be subtracted from Android boot elapsed directly |
| U06 | sing-box libbox `CommandServer.Pause/Wake`; sing v0.8.13 pause manager | DevicePause is callback/ticker coordination, not proof whole engine stopped. `pause.RegisterTicker` is used by URL-test; direct HY2/pinned quic-go timer registration was not found in searched exact closure |

## 5. Explicit Go runtime reference

Go `go1.26.0`, commit `d90b98e65320778f3b1f99a6951ab20f04d218b3`
(BSD-3-Clause), was read only as an explicit reference:

- `time.Since` и timer `when(d)` используют `runtimeNano`.
- Runtime `time_runtimeNano` вызывает `nanotime`.
- Linux arm64 `nanotime1` запрашивает `CLOCK_MONOTONIC`.
- Прочитанная Linux man-page указывает, что эти часы исключают suspend.

Справочная модель согласуется с Android SystemClock-документацией:
`System.nanoTime`/uptime останавливаются в глубоком сне. Она не удостоверяет
будущий Snowden AAR: в датированном L08 Go и digest AAR не установлены.

## 6. OSS Android reference and licence

SagerNet `sing-box-for-android` commit
`bc73504fcb858400a939140ad785ed92f4c9f429`, commit date
2026-10-11T04:15:19Z (`1.15.0-alpha.12`), files `BoxService.kt`,
`VPNService.kt`, LICENSE:

- Receiver наблюдает `ACTION_DEVICE_IDLE_MODE_CHANGED` и вызывает libbox
  `pause()/wake()`.
- Это реальный Android caller; U06 показывает, почему он не доказывает
  подавление таймера HY2 QUIC.
- В прочитанной лицензии — GPL-3.0-or-later и условие о названии/ассоциации
  производного приложения без согласия. Решение — ориентир/адаптация идеи;
  код не копировался, совместимая прямая замена не заявлена.

## 7. Failed/limited sourcing and execution boundary

- `setup-power-rails?hl=ru` вернул HTTP404; прочитана исправленная страница P06.
- URL корневого AOSP LICENSE вернул HTTP404; headers выбранных файлов
  отдельно указывают Apache-2.0. Это не полный аудит NOTICE всей поставки.
- Android RU-страницы отмечают AI/Cloud Translation; ключевые Doze/FGS/job
  положения сопоставлены с EN. Числа и сведения об устройствах P05/P06
  не превращаются в измеренный эффект Snowden.
- Тесты, сборка/toolchains, device/adb, принудительный Doze, VPN runtime,
  симуляции, benchmarks и батарея не измерялись. Все device-случаи REPORT — NOT_RUN.
