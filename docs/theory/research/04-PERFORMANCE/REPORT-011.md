# R04-011 — Android Doze/App Standby, VPN FGS и энергия QUIC

27 октября 2026. **READY_FOR_REVIEW**, не ACCEPTED и не product/device PASS.
Область: source/spec/model; device/energy измерения не выполнялись.

## 1. Пробел и точная область

R04-003 правильно развёл Android `elapsedRealtime` и transport monotime, local
TCP keepalive и outer HY2 heartbeat, а также предупредил, что FGS не означает
«вечную работу». Но для текущего Android36 source не были установлены вместе:

1. фактический FGS type/caller и Android16 network/power guards;
2. разница Doze, App Standby, power allowlist и Low Power Standby (LPS);
3. точная формула pinned HY2/quic-go 10s keepalive и источник часов;
4. реальный OSS Android pause caller и его предел на HY2;
5. что можно измерить на устройстве, не превращая radio model в mAh-обещание.

Версии/файлы/URL/licences/status чтения — [SOURCES-011](SOURCES-011.md).

## 2. Фактическая Android-интеграция Snowden

Current source (не build proof): `compileSdk36`, `targetSdk36`, `minSdk29`;
`SnowdenVpnService` — private `:vpn` service с `BIND_VPN_SERVICE`, permissions
`FOREGROUND_SERVICE` и `FOREGROUND_SERVICE_SYSTEM_EXEMPTED`, manifest type
`systemExempted`, `directBootAware=false`, `stopWithTask=false`, always-on
support=false. На API34+ `VpnNotification.start` передаёт
`FOREGROUND_SERVICE_TYPE_SYSTEM_EXEMPTED`; FGS promotion выполняется в
`onCreate` **до** `WorkerProcess.attach`/profile/JNI. Ошибка promotion ведёт к
retirement/stop; `onStartCommand=START_NOT_STICKY`.

Это подтверждает корректный source ordering, но не runtime eligibility:
Android16 AOSP policy требует type permission и допустимый eligibility signal,
для VPN — allowed `OP_ACTIVATE_VPN`; public Android docs описывают «VPN app,
configured in Settings». Реальный AppOp после consent/configuration — device
fact. Сам label `systemExempted` не является power-allowlist или LPS exemption.

`NativeRuntime`/`NativeWorkerSession` текущего проекта не экспортируют Pause/Wake,
а CommandServer control server сознательно не запускается. Project search не
нашёл WakeLock, power-allowlist request, Doze receiver или LPS API. Поэтому
текущий source не доказывает ни удержание CPU, ни специальную pause-policy.
Native source lock также оставляет `aarSha256=null`, `go=null` и объявляет
strict-owner adaptation not applied/built/verified.

## 3. R04-011-A — матрица power-режимов, не единый «фон»

**Короткий вывод:** для current Android16 source реально foreground UID снимает
сетевые blocked reasons обычных Doze/App Standby/Battery Saver, но FGS type сам
по себе не создаёт wake lock и не преодолевает LPS. App Standby bucket, job quota,
network firewall, CPU scheduling/deep suspend и power allowlists — разные механизмы.

| Условие | Спецификация / AOSP source | Что означает для текущего VPN | Чего не доказывает |
|---|---|---|---|
| Долгий active FGS, normal interactive/awake | Public App Standby: long-running FGS — active criterion; AOSP foreground threshold = `BOUND_FOREGROUND_SERVICE` | UID должен получать foreground reason; ordinary Doze/App-standby firewall reasons снимаются source-моделью | Фактический process state/OEM; радиорасход; protected path |
| App Standby без active FGS | Buckets ограничивают jobs/alarms и в Rare/Restricted сеть; OEM criteria vary | Не использовать WorkManager как VPN keepalive; current active FGS source должен предотвращать эту idle-классификацию, если реально жив | Что FGS никогда не убит/ограничен пользователем или OEM |
| Android16 FGS + JobScheduler/WorkManager | P03: job runtime quota теперь применяется и при FGS, bucket-dependent | Optional profile refresh/diagnostics остаются scheduler jobs и квотируются отдельно | Квота job не является квотой QUIC goroutine/UDP loop |
| Ordinary Doze, foreground UID | NetworkPolicy foreground reason clears DOZE; PowerManagerService не отключает partial wakelock до `BOUND_FOREGROUND_SERVICE` | При фактическом FGS UID ordinary Doze network firewall не должен сам блокировать UDP; уже приобретённый partial lock не попадает в этот idle guard | FGS **не приобретает** partial wake lock; CPU callback/PING не гарантирован |
| Power allowlist / `isIgnoringBatteryOptimizations` | Частичное исключение: network + partial wakelocks; jobs/sync/alarms всё ещё имеют границы; Play допускает только core-function cases | Отдельный будущий policy choice; current app не запрашивает и не проверяет | Не синоним FGS/systemExempted; не bypass LPS/OEM/extreme guardrails |
| Low Power Standby, noninteractive outside maintenance | PowerManager API33: network disabled/wakelocks ignored **including FGS**; AOSP LPS clears only TOP or LPS allowlist | `systemExempted` VPN FGS не входит автоматически в перечисленные LPS reasons; carrier может стать недоступным | Что каждый OEM/device включает LPS; actual observed block/kill |
| Deep suspend CPU-off | Android `uptimeMillis`/`System.nanoTime` and Linux CLOCK_MONOTONIC exclude sleep | Go runtime timers cannot execute while CPU is suspended; reference timer elapsed excludes suspended duration | Doze всегда означает deep suspend; фактическая residency без device trace |
| Always-on bootstrap | AOSP `Vpn.startAlwaysOnVpn`: temporary 60s device-idle allowlist | Relevant only if always-on enabled later | Persistent carrier/network exemption; current manifest explicitly disables always-on |
| User manually restricts app battery / OEM policy | P03 network depends on bucket/process; OEM may add policy | Must be device matrix cell and typed failure, not silently labelled censorship | Универсальное поведение across OEMs |

### Контрпримеры

- **FGS ≠ вечная сеть:** включённый LPS прямо ограничивает network/wakelocks FGS.
- **Doze ≠ гарантированный blackout VPN:** AOSP foreground network reason removes
  ordinary Doze/App Standby block for a genuinely foreground UID.
- **No WakeLock ≠ обязательно drain:** CPU may be awake for unrelated work; radio
  may already be active due to other apps. Conversely FGS alone does not keep it.
- **App Standby exemption ≠ no power policy:** job quota and LPS are separate.

## 4. R04-011-B — exact source модель 10s HY2 keepalive

Pinned caller graph:

```text
sing-box Hysteria2OutboundOptions
  (no keepalive-period field)
→ protocol/hysteria2.NewOutbound
→ sing-quic hysteria2.NewClient
  MaxIdleTimeout = 30s
  KeepAlivePeriod = 10s
→ quic-go Config
→ qtls/http3 → quic-go Conn run loop
→ PingFrame queue → sendQueue(capacity 8) → sendConn/raw PacketConn
→ UDP socket (with optional Salamander wrapper before quic-go transport)
```

После transport parameters:

```text
I = min(10s, negotiatedIdle / 2)
D = lastPacketReceived + max(I, 3*PTO/2)
```

если `KeepAlivePeriod!=0`, `keepAlivePingSent=false`, connection not blocked in
the disallowing mode. At D, run loop **queues** a PING and sets the flag. Any
received QUIC packet updates `lastPacketReceived` and clears both PING flag and
first-ack-eliciting-idle marker; это не ACK конкретного PING. If hard-blocked,
timer path arms an idle deadline; send queue and socket write are later stages.
Therefore «10s» is neither a fixed callback schedule nor a wire/NAT receipt:
PTO can make it longer, peer idle can cap it, traffic reanchors it, queue/socket/
network add delay, and blocked state changes the path.

Модель часов по исходникам, не идентичность поставленного артефакта:
quic-go monotime использует `time.Since`. Справочная Go1.26.0-цепочка
runtimeNano→nanotime→Linux arm64 `CLOCK_MONOTONIC` исключает suspend;
Go Timer `when(d)` использует тот же runtimeNano. Android-полномочия опираются
на `SystemClock.elapsedRealtime`, включающий сон. При применимости этого
reference длительность transport не стареет во сне так же, как Kotlin authority.
После resume полномочие может истечь, пока keepalive/idle не учли время сна.
Lifecycle/callback guards перепроверяют полномочия, но не прерывают уже вошедший
native call. Точный принятый AAR/Go в этом срезе не установлен.

### Strong alternatives / falsifiers

- Другие принятые Go runtime/ABI и семантика часов могут нарушить перенос
  Linux-arm64 reference; нужна идентичность конкретного артефакта.
- Если иная нагрузка не даёт CPU глубоко уснуть, таймер может продолжить работу,
  но энергия относится к состоянию всего устройства.
- При PTO > 6.67s величина `3/2*PTO > 10s`; номинальные 10s уже не период.
- Трафик peer с интервалом<D подавляет отдельный keepalive, хотя внешний
  трафик остаётся. Один счётчик PING не учитывает всю активность.
- Принятие PING в очередь без успешного UDP write не обновляет NAT.

## 5. R04-011-C — upstream pause caller is not HY2 timer proof

SFA `bc73504…` — полезный ориентир вызывающего кода, не готовая замена.
Он регистрирует `ACTION_DEVICE_IDLE_MODE_CHANGED` и вызывает libbox
`CommandServer.pause()/wake()`. В libbox1.13.19 это
`PauseManager.DevicePause/DeviceWake`. Менеджер меняет pause channels и вызывает
зарегистрированные callbacks; `pause.RegisterTicker` останавливает/reset URL-test.
В названной HY2/quic-go области подписка PauseManager не найдена.

Therefore:

- Найден реальный Android lifecycle→pause caller.
- Не доказана остановка quic-go connection Timer/PTO/HY2 PING этим caller.
- В прочитанной Snowden facade такого caller нет; это датированная граница.
- Перенос всего SFA-кода не согласован с владельцем и защитой Snowden.
  В прочитанной лицензии — GPL-3.0-or-later и условие о названии/ассоциации.

Выбор переноса — только ориентир/адаптация. Будущий pause-интерфейс требует
найти конкретных потребителей и определить смысл: закрыть/restart соединение,
остановить необязательные URL tests или определённые таймеры.
Общая отметка pause не доказывает экономию энергии.

## 6. Энергетический вывод без выдуманных чисел

Let observations over matched interval Δ include:

- `B_out`, `P_out`: фактические внешние UDP bytes/packets после socket write.
- `K_due`, `K_queued`, `K_written`: отдельные due/queue/write стадии keepalive.
- `T_cpu`: время CPU приложения/native, callbacks и таймеры.
- `E_rail`: энергия radio/WLAN rail там, где доступен ODPM.
- `R(t)`: RAT/signal/thermal/charging/metered/screen/interactive/LPS/Doze.
- Полезное завершение, reconnect/handshake latency и отказы.

`E_rail` относится к устройству и включает другие приложения. Сравнение
условий требует согласованной заранее нагрузки и учёта контекста; без модели
изоляции нельзя приписать каждый джоуль K. Числа3G tail из P05 лишь
иллюстрируют механизм. Из изменения10s, pause callback или числа пакетов
не следуют проценты/mAh экономии.

Обмен двусторонний: меньше idle-пакетов может уменьшить radio/CPU работу,
но увеличить истечение соединения, reconnect, TLS/QUIC handshake, задержку
потерянной задачи и байты. Исход должен учитывать и простой, и восстановление.

## 7. Reuse / adapt / reject

| Candidate | Version/license | Decision and price |
|---|---|---|
| Existing `systemExempted` FGS ordering | Project GPL source; Android API34+ | **Reuse.** Preserve early promotion and typed failure. Do not reinterpret as power exemption |
| AOSP network blocked callbacks + current PhysicalNetworkMonitor | Android16 Apache-2.0 + project source | **Reuse** as one observation; add named power state only if owner accepts. Callback receipt is not causal proof or energy measure |
| Existing quic-go timer machinery | quic-go ccd46… MIT; sing-quic d838… GPL-3+ | **Reuse.** No blanket clock/keepalive patch before artifact+device attribution |
| SFA Doze receiver→PauseManager | bc73504…, GPL-3+ plus name clause | **Reference/adapt only.** First prove exact affected consumers; no code copied |
| Power allowlist request | Android API23+ | **Defer policy decision.** Core VPN may be eligible, but Play/user/UX/security cost and actual need require device evidence |
| Permanent app WakeLock | Android API | **Reject as default.** FGS does not require it; raises drain and does not solve LPS/network/NAT |
| WorkManager/JobScheduler as HY2 keepalive | Android scheduling APIs | **Reject.** Android16 jobs remain quota/bucket work and cannot provide 10s transport timing |
| Android SocketKeepalive | API29 IPsec NAT-T scope | **Reject as QUIC drop-in.** Different packet/crypto/state |

## 8. Конечная device-программа — все случаи NOT_RUN

Нужен **точный принятый APK/AAR receipt** (APK/AAB hash, AAR hash, Go/NDK/ABI,
build tags, Android/OEM/patch/modem/carrier). Thresholds/stop rules задаются до
сравнения; никаких процентов заранее.

| Case | Факторы / наблюдения | Положительный результат и falsifier |
|---|---|---|
| D1 Awake idle baseline | Same device/network/profile, screen on/off-but-awake; outer socket writes, PING stage counters, CPU, ODPM rails, useful task | Establish attribution floor; if timers/writes absent, source-to-artifact mapping wrong |
| D2 Ordinary Doze + active FGS | Force-idle only in approved device test; process state, blocked reasons/callback, CPU residency, QUIC packets | Network not blocked when foreground as AOSP predicts; actual deep suspend analysed separately. Block reason/OEM divergence falsifies transfer |
| D3 App Standby buckets | Active/Working/Rare/Restricted with active FGS and without it; jobs vs carrier separated | FGS carrier and scheduled jobs show their distinct constraints; no synthetic success from notification alone |
| D4 Low Power Standby | Supported API33+ device, enabled/noninteractive; `isExemptFromLowPowerStandby`, blocked status, wake/packets | Distinguish LPS from Doze; no claim on unsupported/OEM-disabled devices |
| D5 Power allowlist | User-chosen baseline vs exempt, same contexts; privacy/Play UX; packets/CPU/energy/recovery | Adopt only if core protected completion improves with bounded cost; lack of effect rejects request |
| D6 Suspend/resume clock trace | QUIC reference-compatible artifact; `elapsedRealtime`, Go monotime exported via bounded debug build, suspend residency | Authority expires on elapsed scope; no stale apply; transport resumes/reconnects without direct fallback. Clock mismatch revises model |
| D7 Keepalive candidates | Existing fixed source vs supported reviewed alternative/close-on-idle; idle+NAT-expiry+recovery matched | Compare total energy and useful recovery, not PING count; higher churn/failures reject candidate |
| D8 Wi-Fi/LTE/5G contexts | Signal bins, RAT, thermal, charging, other-traffic controls; ODPM WLAN/cellular where available | No cross-RAT transfer of P05 3G numbers; missing rails remain unknown |
| D9 OEM/patch matrix | At least one reference AOSP-like and owner-selected OEMs/patches, same script | Record policy divergence; no universal Android claim from one device |
| D10 Safety during power transitions | Revoke/Stop/authority expiry/network block while sleep/resume/JNI entered | No renewed grant, parallel worker or direct fallback; protection wins over energy optimization |

Приватность: локальные сокращённые counters и только ручной экспорт.
Без domains, credentials, app inventory и содержимого трафика. Шум измерения
ODPM всего устройства, цензурированные и пропавшие исходы сохраняются;
неизвестное не заполняется нулями.

<a id="9-выводы-для-руководителя"></a>
## 9. Выводы для разработки

### R04-011-A — OS power-mode contract

**READY_FOR_REVIEW, спецификация и закреплённый AOSP:** настоящий foreground
VPN UID отличается от idle background app в обычных Doze/App Standby network
правилах. FGS/systemExempted не дают универсального исключения: LPS, сон CPU,
jobs/alarms, power allowlist и OEM/user ограничения рассматриваются отдельно.
В исходном срезе Snowden не запрашивал WakeLock или power exemption.

### R04-011-B — QUIC timer contract

**READY_FOR_REVIEW, модель закреплённых исходников:**10s HY2 — вход
receive-anchored/PTO-bounded таймера, не фиксированный внешний трафик
или расход энергии. Формула и стадии — §4. Go1.26/Linux-arm64 reference
исключает suspend; он не подтверждает будущий Android AAR.

### R04-011-C — reuse boundary

**READY_FOR_REVIEW, закреплённые OSS-исходники:** SFA показывает Doze→libbox
pause/wake. PauseManager сам не доказывает подавление HY2/quic-go timer;
регистрация HY2 не найдена в названной области. Это ориентир для адаптации,
не совместимая прямая замена.

<a id="10-предлагаемая-вставка-в-общую-теорию-руководитель-решает"></a>
## 10. Предлагаемый вывод для общей теории

> Foreground service VPN, App Standby active, разрешение сети Doze, power
> allowlist и Low Power Standby — разные контракты. В Android16 source
> foreground state снимает обычные Doze/App Standby network reasons; LPS может
> запрещать сеть и игнорировать WakeLock для FGS. Сам FGS не приобретает WakeLock.
> Номинальные 10s HY2 соответствуют
> `last receive + max(min(period,idle/2),1.5*PTO)`, затем queue→socket→network.
> Это не фиксированное пробуждение радио и не NAT receipt. Энергетический вывод
> требует точного артефакта, сопоставимых device/RAT/power-state измерений
> и цены reconnect; pause API учитывается после проверки реальных подписчиков.

Подробное продолжение от 28 октября — [таймер HY2 и pause](../cycles/2026-10-28-contracts/Z05/REPORT.md).

## 11. Остаток знания

Не установлены точный принятый AAR и его Go clock, process state на разных OEM,
включение/исключения LPS для VPN, цена radio/CPU/батареи, сроки NAT, покрытие
SFA pause за пределами указанного поиска и сравнительная польза нового
интерфейса. Неизвестное не равно нулю и само по себе не является дефектом продукта.
