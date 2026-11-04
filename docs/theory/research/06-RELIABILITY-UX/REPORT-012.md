# R06-012 — VPN/MDM: полномочия, исключения и наблюдаемость Windows/Android

**Дата исследования:** 2026-10-27. **Статус:** READY_FOR_REVIEW, не научный ACCEPTED и не product/native PASS.

## 1. Вопрос и метод

Что может выполнить обычное VPN-приложение, пользователь, device/profile owner
и Windows MDM; какие исключения допускает платформа; какие наблюдения достаточны
для объяснения ограничения, но не доказывают защищённый выход?

Baseline — R06-006, R06-010 (§25–30/39 общего REPORT), **не принятые аксиомы**.
Новые утверждения локальны пакету: R06-012-A/B/C. Старые документы не изменены.
Исследование ограничено чтением публичных API, выбранных source-фрагментов,
ручным разбором контрпримеров и документальной сверкой. Описанная конфигурация
устройства пользователя не читалась; вызовы ОС/сети VPN не выполнялись.

Источники и глубина чтения — [SOURCES-012](SOURCES-012.md). Основной Android pin:
AOSP `android-16.0.0_r1`, frameworks/base
`99b01a65cc4c104933788b3143285ab6bae65827`; Settings
`0fb38ed81e9b49d5da4be8f50d9d69865c1192e8`. Шесть выбранных файлов проверены
побайтно tag↔immutable commit. Это не аудит всего Android/OEM или APK.
Windows: публичные документы Microsoft, текущие исходники Snowden,
`tailscale/wf@v0.0.0-20240214030419-6fbb0a674ee6`, `sing-box@v1.13.19`.

## 2. Поправки к прежней матрице

| Baseline R06-010 | Проверенное уточнение |
|---|---|
| `setAlwaysOnVpnPackages` | Публичный метод **singular** `setAlwaysOnVpnPackage`. Три аргумента — API24; четыре, включая `Set<String>`, — API29. Прежняя ссылка на plural метод не является API evidence |
| «per-app наборы — вне приложения; пользователь меняет их в системных настройках/MDM» | Наборы задаёт **само VPN-приложение** методами Builder, API21. Пользователь может выбирать их в UI этого VPN, если функция реализована. Общего обязательного системного UI для произвольного per-app списка не установлено. DPC lockdown allowlist — другой список |
| «`prepare()` отбирает TUN у предыдущего» | Неподготовленный/несогласованный клиент обычно получает Intent и должен пройти consent. Ранее согласованный клиент способен подготовиться с побочным эффектом; выбранный always-on пакет защищён от замены обычным prepare. Поэтому prepare не безусловный захват и не безусловно read-only |
| «весь egress мимо TUN блокируется; приложение не может ослабить» | Lockdown — требование для конкретных UID/user ranges с исключениями VPN owner, DPC allowlist и отдельными системными случаями. VPN не вправе самостоятельно менять admin policy; пользовательская настройка и admin-enforced настройка имеют разных владельцев |
| «Always-on/lockdown на Snowden» | В текущем `android/vpn/.../AndroidManifest.xml:18` — `SUPPORTS_ALWAYS_ON=false`. Для VpnService-пути это opt-out; AOSP имеет отдельную раннюю ветку для установленного PlatformVpnProfile (ни provision caller, ни такой профиль этим пакетом не установлены). Наличие API29/readers и target36 opt-out не устраняет |
| «нет LocalSubnet permit ⇒ LAN-принтер обязательно заблокирован; только отключить защиту» | В renderer уже есть RFC1918/`.local` direct через engine; другой AppID создаёт исходный и повторно открытый engine socket. Наличие или отсутствие разрешения надо проверять **для каждого участка**; правило `172.16/12` исключает этот маршрут из TUN и образует отдельный случай |
| «второй VPN обязательно не установится, потому что AppID другой» | Верно только для нужного direct physical connect без иных matching permits. При его транспортном соединении через Snowden TUN LUID permit может соответствовать; совместимость вложенного транспорта, чужая policy и остальные WFP layers — отдельные предпосылки |
| «NRPT/PAC fetch всегда block» | NRPT — выбор resolver/namespace, PAC — выбор proxy; эффективный DNS/dial может пройти TUN/engine, physical или иной provider. Имя/timeout не устанавливают ни путь, ни причину |

## 3. R06-012-A — Android: три разных полномочия, не один список исключений

### 3.1 Публичные API и точный scope

| API | Added | Кто вызывает / семантика |
|---|---|---|
| `VpnService.prepare(Context)` | 14 | VPN-приложение. `null` при доступной подготовке; иначе Intent системного consent. Результат и подготовка могут устареть до establish |
| `VpnService.onRevoke()` | 14 | Callback ОС: интерфейс уже деактивирован; не обязательно main thread. Требуется retirement ресурсов, не обещание сохранённой защиты |
| `Builder.addAllowedApplication(String)` / `addDisallowedApplication(String)` | 21 | VPN-приложение строит **свой** capture set до establish. В одном Builder один тип списка; смешение вызывает UnsupportedOperationException, отсутствующий package — NameNotFoundException |
| `Builder.allowBypass()` | 21 | VPN-приложение просит разрешить eligible apps выбрать другую Network. AOSP connected VPN использует `allowBypass && !mLockdown`; это не обход admin lockdown |
| `setAlwaysOnVpnPackage(ComponentName,String,boolean)` | 24 | Device/profile owner выбирает пакет для своего user; очищает lockdown allowlist. Не обычный active DeviceAdminReceiver сам по себе |
| `setAlwaysOnVpnPackage(ComponentName,String,boolean,Set<String>)` | 29 | Тот же owner плюс **lockdown allowlist**: допускает указанные приложения без подключённого VPN; когда VPN есть, используют его, если включены в capture set |
| `getAlwaysOnVpnPackage(ComponentName)` | 24 | Owner query. `null` не доказывает отсутствие любого VPN: документация допускает system-provided вместо app |
| `isAlwaysOnVpnLockdownEnabled(ComponentName)` / `getAlwaysOnVpnLockdownWhitelist(ComponentName)` | 29 | Owner query; `Whitelist` — реальное legacy имя public getter, хотя setter использует allowlist. Getter allowlist возвращает null, если always-on/lockdown не активны |
| `VpnService.isAlwaysOn()` / `isLockdownEnabled()` | 29 | Обычный provider читает состояние **своего** сервиса через `isCallerCurrent...`; не глобальная инвентаризация других профилей/allowlist |

No-arg DPM `getAlwaysOnVpnPackage()`/`isAlwaysOnVpnLockdownEnabled()` встречаются
в системном Settings source, но помечены `@hide`/system-only. Они не public
replacement для приложения Snowden. В DPM service API есть специальный
`MAINLINE_NETWORK_STACK` путь для lockdown query — это signature-роль системного
NetworkStack, не «можно любой VPN». Серверный setter проверяет
`isDefaultDeviceOwner || isProfileOwner`, никакого общего delegate-пути в этом
проверенном методе нет.

### 3.2 Capture set и lockdown allowlist

Обозначим U — UID ranges, за которые отвечает данный user/profile VPN;
C — UID, включённые в VPN capture; E — исключённые из lockdown UID; L — lockdown.

AOSP строит C из package→UID, не только строк package. `getAppsUids` добавляет
соответствующий SDK sandbox UID. Shared-UID пакеты требуют дополнительной
проверки: выбор одного package не обещает изоляцию другого package с тем же UID.

При L=true блокируемый набор вычисляется по user/restricted-profile ranges,
**не** как копия C. Следовательно, приложение вне C, но не в E, не получает
обычный direct-доступ. Это конкретный counterexample «исключить из VPN =
разрешить обход». E — VPN package плюс DPC allowlist; source отдельно исключает
UID0 из user0 диапазона для kernel IPsec. Слово «system app» в DPM JavaDoc
не надо превращать в утверждение, что любой предустановленный APK/UID всегда
имеет direct: реализация перечисляет ranges, а не общий PackageManager-флаг
`FLAG_SYSTEM`. Ни этот пакет, ни стандартная документация не дают полного
OEM inventory системных исключений.

Это **scope-модель** read source, не математическое доказательство исполнения
на конкретном устройстве. VPN outer sockets должны идти вне собственного TUN:
это необходимый bootstrap, а не автоматическое разрешение пользовательского
трафика direct. Наличие `protect()` не доказывает шифрование outer transport.

### 3.3 Actor/action/observability matrix

| Actor / сценарий | Допустимое действие | Наблюдаемость и безопасная граница |
|---|---|---|
| Пользователь unmanaged профиля | Consent системного Intent; выбор/выключение Always-on и Block connections without VPN для supporting provider через Settings | Нельзя обещать наличие переключателя при provider opt-out или admin restrictions; действие меняет OS policy, требует явного выбора пользователя |
| Обычное VPN-приложение | Создать Builder capture set; запросить consent; protect своих tunnel sockets; читать свои API29 facts | Не назначает себя device/profile owner и не меняет чужой lockdown allowlist. Управляемая конфигурация может быть input приложения, но не добавляет owner-права |
| Profile owner | Назначить VPN/lockdown/allowlist в своём user, ограничить VPN-конфигурацию | Work и personal managed scopes не сливаются. Setter от parent-instance запрещён в прочитанном DPM; другие org-owned device ограничения вне этого setter не объявлены отсутствующими |
| Device owner | Настроить свою policy-owned VPN и user restrictions в допустимом scope | Это административное действие; никакого автономного обхода «если не работает» |
| Другой несогласованный personal VPN | Вызов prepare обычно возвращает Intent, не немедленный revoke | Реальный переход только после доступного consent/подготовки. При already-preconsented возможен prepareInternal; UI должен не вызывать prepare как passive health check |
| Другой клиент при выбранном always-on | В AOSP `Vpn.prepare` возвращает false для попытки сменить mPackage | Новая app не может просто «отобрать TUN». Управляющий user/admin должен явно изменить допустимую policy |
| Revoke после легального перехода | ОС отзывает prepared app/деактивирует интерфейс; provider закрывает ресурсы | Callback не определяет дальнейший защищённый выход: новый VPN, блокировка или обычная сеть должны быть установлены отдельно |
| Personal VPN + work VPN | По одному активному VPN в разных user/profile contexts | Не обещает бесшовный cross-profile доступ; restricted profiles наследуют parent UID range по специальной ветке, это не обычный managed work profile |

При `DISALLOW_CONFIG_VPN`: UserManager source различает owner scopes; с API24
admin-created always-on сохраняет возможность старта; с API31 restriction
очищает user-configured active VPN. DPM `vpnPackage=null` с API31 не означает
право стереть любой пользовательский VPN: setter сначала проверяет, что у
этого admin было своё назначение. Stock Settings отключает элементы по
`DISALLOW_CONFIG_VPN` и admin-owned always-on. Утверждение «обычный пользователь
всегда может снять lockdown» так же неверно, как «никогда не может».

### 3.4 Фактические callers Snowden

- Manifest scope: minSdk29, compile/target36; service BIND_VPN_SERVICE,
  systemExempted FGS, `directBootAware=false`, **always-on opt-out=true**.
  `Vpn.isAlwaysOnPackageSupported:796–839` сначала разрешает installed
  PlatformVpnProfile, иначе проверяет target24+/VpnService metadata и отвергает
  opt-out. В scoped Android Kotlin/XML search не найден VpnManager/Ikev2VpnProfile/
  provisionVpnProfile consumer; реальный установленный профиль не читался.
  Поэтому source-вывод ограничен текущим VpnService-путём без такого platform
  profile, не всем мыслимым VPN одного package. Merged APK/manifest/OEM NOT_RUN.
- `AndroidTunFactory.open` **отвергает** include/exclude packages, HTTP proxy,
  excluded/partial routes; затем full v4 и optional v6, DNS, выбранная physical
  Network. Не зовёт prepare; per-app — future compiler, не отсутствие Android API.
- `VpnSessionRepository.prepareExplicitOnce`: prepare доступен только явному
  Connect lineage; не passive reader. `SnowdenVpnService.onRevoke` передаёт
  retirement worker и вызывает super; original owner lifetime не закрывается
  одним stopSelf. `WorkerProcess.start:183,217` требует fresh alwaysOn/lockdown
  true для `requireLockdown`; при неисполненной предпосылке возвращает retirement
  `LOCKDOWN_REQUIRED`, не ослабляет требование. Manifest opt-out и этот gate
  вместе означают недоступность strict-start в указанном VpnService сценарии,
  **не** потребность в automatic bypass/переключении flag.
- `OsProtectionReader` читает isAlwaysOn/isLockdownEnabled, marks собственный
  unbound socket protect, сохраняет nullable facts и start timestamp.
  Эти факты не атомарны и не позволяют обычному app доказать пустоту E:
  текущий getter не раскрывает DPC allowlist. По двум true нельзя выводить
  «все пользователи/все приложения/весь physical трафик защищены».

## 4. R06-012-B — Windows: профиль MDM, арбитраж WFP и реальные сокеты

### 4.1 VPNv2 CSP — область встроенной платформы, не произвольного EXE

Проверены реальные русские страницы Microsoft, не просто URL с ru-ru.

| Node / свойство | Проверенное значение и ограничение |
|---|---|
| `./Device/Vendor/MSFT/VPNv2/{ProfileName}` / `./User/Vendor/MSFT/VPNv2/{ProfileName}` | Отдельные scopes профилей. Таблицы применимости для выбранных базовых nodes: Win10 1511+, Pro/Enterprise/Education/IoT варианты; Home из таблиц не следует |
| `.../AlwaysOn` | Автоматическая активация/подключение, не самостоятельный запрет иных выходов. Документ учитывает ручное отключение/сохранённое preference |
| `.../AppTriggerList/{appTriggerRowId}/App/Id` | Запуск приложения инициирует подключение профиля. Сам по себе не правило «только это приложение допускается» |
| `.../TrafficFilterList/{trafficFilterId}/App/Id` | Traffic-filter identity: PFN/FilePath, `SYSTEM` для некоторых kernel flows. Это другой объект, чем trigger |
| `.../NativeProfile/RoutingPolicyType` | `SplitTunnel`/`ForceTunnel`; ForceTunnel манипулирует default routes и metrics, более специфичный physical route может остаться. Не OS fail-closed доказательство |
| `.../DisableDisconnectButton` | Win11 21H2+ в прочитанной таблице: скрывает UI-кнопку. Не заменяет фильтрацию/LockDown |
| ProfileXML `<LockDown>` | Есть в XSD; отдельная Microsoft security-концепция: блокирует выход при unavailable VPN, нельзя изменить/удалить/отключить профиль; для built-in connection только IKEv2. Элемент в XSD не доказывает поддержку каждым provider/build |
| `.../ByPassForLocal` | Хотя присутствует в дереве/XSD, CSP описание: **«Не поддерживается»**. Нельзя предлагать его как готовый LAN escape hatch |

Ссылка vpnv2-profile-xsd при загрузке перенаправилась в VPNv2 CSP с embedded
XSD; это один источник, а не независимое подтверждение. Старый SyncML example
использует `Lockdown`, XSD — `LockDown`; точная интеграция должна следовать
поддерживаемому schema, не копировать пример как API. Не утверждается, что
документация доказывает profile на машине пользователя.

В выбранном Snowden caller — Wails/Go TUN plus own WFP и HKCU proxy lifecycle,
не NativeProfile/UWP VPN plugin. Scope публичного CSP не переносится на него
по аналогии; не найден/не установлен его registered CSP adapter. This is
source-scoped statement, не универсальное отсутствие всех management путей.

### 4.2 WFP arbitration: что значит weight

По официальному Filter Arbitration:

1. Приоритет sublayer и filter weight — разные уровни. Внутри sublayer
   первые matching terminating Permit/Block прекращают выбор нижних filters.
2. Между sublayers участвуют action type и write-right, а не лишь самый большой
   вес. Soft permit допускает block ниже; hard permit всё ещё допускает callout
   Veto. Нет cross-layer arbitration: permit одного layer не покрывает другой.
3. `FWPM_FILTER_FLAG_CLEAR_ACTION_RIGHT` нельзя отождествлять с универсальным
   bool «все actions soft/hard». В той же статье default **static filter block**
   указан hard, **callout block** — soft; WDK отдельно предписывает callout
   clearing ACTION_WRITE при block. Эти разные контексты надо явно именовать.

**Точный source-предел Snowden:** `wf.compose.go:129–130` только ставит
CLEAR_ACTION_RIGHT при `Rule.HardAction=true`; false просто не ставит этот flag.
Snowden регистрирует **static** `wf.ActionBlock`, не callout, а комментарий
`wfp_killswitch_windows.go:119–124` объявляет его soft на основании false.
Это обоснование **не доказано** и конфликтует с default-table Microsoft.
Нельзя на этом комментарии строить доказательство повторной маршрутизации
после уже заблокированной авторизации. Документация ALE описывает reauthorization
при policy change, arrival interface и pend/complete; она не обещает, что
любой отвергнутый SYN будет позже разрешён. Здесь **не заявлены** live bypass,
неработающая защита или успешная exploit-схема; требуется узкая native semantics
приёмка владельца. Код/флаги не изменены.

### 4.3 Source route/classification matrix вместо blanket LAN ban

Подтверждённые inputs:
- Base permits: AppID executable, loopback, DHCP; дополнительные permits —
  local-interface snowden0 LUID; catch-all block. Max sublayer weight 65535
  не гарантирует монопольный приоритет среди всех других providers/layers.
- AppID строится `FwpmGetAppIdFromFileName0`, **не PID**. Обычный worker spawn
  использует `os.Executable()`; parent/worker одного пути имеют одинаковое
  совпадение по этому condition. Это не grants для произвольных других binaries.
- Renderer `routeRulesWithSplit`: DNS hijack, optional ProcessName direct,
  RFC1918/`.local` direct; route.final proxy. TUN ещё содержит physical exclusion
  `172.16/12`. sing-box direct `DialContext` открывает сокет из engine process.

| Сценарий / этап | Local matching input | Допустимый вывод и неизвестное |
|---|---|---|
| Обычный app→192.168.x.x, маршрут попал в snowden0 | Исходный поток может соответствовать LUID permit; renderer выбирает RFC1918 direct; engine повторно открывает socket, соответствующий AppID permit | Возможна LAN-достижимость **без удаления WFP** при совместимых маршрутах/policy; actual classification, DNS/protocol/ответ принтера NOT_RUN |
| Обычный app→172.16.x.x по route exclusion | Physical app socket, не engine-AppID/loopback/DHCP/TUN | В собственном наборе достигает block при этих условиях. Нельзя переносить итог на весь RFC1918 или все LAN приложения |
| Kernel SMB/печать discovery/multicast | Другая identity/layer, возможно иной маршрут и неподдержанный протокол | `.local` unicast rule не доказывает mDNS/LLMNR/SMB/print успех; unknown без конкретного caller |
| Второй VPN direct outer socket | Другой AppID, physical interface | Вероятный собственный block при отсутствии других matching permits и с действующей policy |
| Второй VPN outer transport через Snowden TUN | Может соответствовать LUID; дальше engine outbound | Не универсально запрещено; нужны совместимость outer protocol, маршруты/loop prevention, корпоративное согласование и другие filters. Не выдаётся за готовую поддержку вложенного VPN |
| Corporate DNS/PAC | OS resolver/app может пойти physical/TUN/через loopback-прокси; DNS hijack меняет semantics | NRPT/PAC policy ≠ observed wire path. Core DNS может использовать tunnel resolver и не давать corporate resolution; timeout не идентифицирует владельца block |
| Engine-AppID permit + внешний firewall veto/другой layer block | Собственный permit совпадает, но не финальный action | Local permit/readback не доказывает egress; provider outcome нужен отдельно |

Source показывает наличие candidate route, не нормативную допустимость исключения.
Project safety policy остаётся у владельца платформы; любые изменения capture,
private direct, allowlist, credentials или чужой VPN требуют отдельного решения.
**Никакого** предложения «снять защиту по умолчанию» из матрицы не следует.

## 5. R06-012-C — объяснимость ограничивается полномочиями наблюдателя

Предлагаемый bounded contract (не новый API/реализация): наблюдение хранит
`platform/version, user/profile, actor-capability, operation, config-generation,
flow-stage/identity-if-known, observation-origin, timestamp, result-or-unknown`.

Отдельны:
- **Authority:** кто вправе изменить policy/capture;
- **Capability:** поддержаны ли method/provider/manifest и transport;
- **Configured:** назначение/allowlist/routes приняты;
- **Applied/capture:** TUN/WFP присутствуют в соответствующем scope;
- **Protected completion:** конкретный выход/задача под нужной защитой.

Boolean `lockdown=true`, own filter readback, DPC query и app consent не заменяют
последний уровень. Обычный provider не должен обещать global empty allowlist,
а timeout/другой VPN icon — давать уверенный badge «MDM запретил». Badge допустим
как **подтверждённый источник ограничения** только с scoped evidence; иначе
«источник не установлен / требуется проверка policy владельцем».

| Возможное безопасное действие | Предпосылка |
|---|---|
| Объяснить неподдержанный Always-on | Текущий provider opt-out/unsupported receipt; не утверждать, что ОС вообще не умеет |
| Объяснить admin-managed состояние и направить к IT | Подтверждённый policy ownership, не предположение по таймауту |
| Предложить пользователю просмотреть свои Settings | Unmanaged/разрешённый scope; не обещать доступность switch и не выполнять изменение |
| Сохранить действующий guard и unknown при неясном пути | Нет достаточного evidence для automatic bypass/restore чужой policy |
| Исследовать конкретный разрешённый LAN/alternate transport | Только уже допустимые policy+scope; результат не получается из одной route-table |

## 6. Reuse, цена и границы

- **Reuse:** existing Android explicit-consent lineage, nullable OS readers,
  manifest capability declaration и established-TUN observation; эти source
  пути лучше нового DPM service/привилегий. Per-app compiler и encrypted
  always-on recovery — отдельные requirements AND-06/AND-03/04, не закрытые.
- **Reference:** pinned AOSP Vpn/Settings contracts (Apache-2.0 file headers);
  их system-only calls нельзя импортировать в ordinary app. OEM/API29–35
  backports не audited: API-added table не гарантирует identical implementation.
- **Existing Windows wrapper:** wf BSD-3-Clause (LICENSE прочитан), compose.go
  remote exact6fbb… равен cache; behavior comment не native proof. Не добавлять
  callout driver или менять WFP hardness ради desk-research.
- **Цена:** required scope/caller evidence, cross-profile privacy, проверка
  combined policy, UX capability distinctions. Расход RAM/CPU/скорость,
  стоимость разработки и полевое качество не измерены.
- Не вводится новая telemetry/MDM платформа, не собираются workplace identities,
  package inventory, credentials, PAC URL или реальные user policies.

## 7. Конечная будущая приёмка — всё NOT_RUN

| Case | Изолированное условие / критерий |
|---|---|
| A01 | Неподготовленный B при активном A: prepare возвращает Intent, до доступного consent нет незаметного перехода |
| A02 | Ранее согласованный B без always-on: проверить side-effect prepareInternal и revoke A; не использовать как passive probe |
| A03 | Admin/user always-on A: попытка B не меняет выбранный пакет без легального изменения policy |
| A04 | Supporting test provider: Builder allow и disallow нельзя смешать; отсутствующий package не превращается в broad capture |
| A05 | App вне C при lockdown, без E: blocked; в policy E без VPN — permitted scope, при connected VPN capture определяется C |
| A06 | API31 DISALLOW_CONFIG_VPN: user-created и admin-created VPN имеют разные outcomes |
| A07 | Personal/work раздельны; restricted profile parent inheritance и shared/SDK-sandbox UID отдельно проверены |
| A08 | Snowden merged manifest сохраняет SUPPORTS_ALWAYS_ON=false; selector не должен считать provider supported. Изменить flag без recovery acceptance запрещено |
| A09 | Revoke/late Binder observation: no protected Ready из старых facts; retirement и новая policy outcome отдельно |
| W01 | Официальный reference action table: static block versus callout; HardAction false/true не проверяется одной строкой комментария |
| W02 | Собственный permit+чужой deny/veto/другой layer: итог различён; own readback не delivery receipt |
| W03 | 192.168 unicast через TUN→engine и 172.16 physical exclusion: разные flows/AppID; actual LAN protocol отдельно |
| W04 | Второй VPN direct versus через TUN: условия допустимости и loop avoidance названы, нет автоматического unprotected fallback |
| W05 | VPNv2 AlwaysOn/ForceTunnel/LockDown/DisableDisconnectButton различены; ByPassForLocal не объявлен рабочим API |
| W06 | DNS/PAC/NRPT path и effective resolver: известный receiver ≠ таймаут как диагноз; чужая policy не меняется |
| W07 | MDM/пользователь меняет proxy после snapshot: ownership conflict сохраняется, не silent restore чужого нового state |

Это планы будущего разрешённого isolated native/device исследования, не созданные
fixtures/tests. Текущее workstation/профиль пользователя не используется.

## 8. Предлагаемая вставка и остаток

Адресат — руководитель, тематическая Android-глава и UX/Windows safety crosswalk;
вставка не выполнена:

> Capture set, always-on owner, lockdown exception set и фактический egress —
> разные объекты. Полномочия обычного VPN-provider ограничены его user/profile
> и публичными API; admin policy не заменяется app consent. Per-app исключение
> из capture не является разрешением обходить lockdown. На Windows разрешение
> оценивается по stage/identity/layer, а не по отсутствию одного LAN-rule;
> повторный engine socket может иметь другой AppID outcome. Любое универсальное
> «заблокировано/защищено» требует evidence в названном scope, иначе unknown.

Открыто: deployed APK/merged manifest, API29–35/OEM enforcement различия,
native static-block/action-right semantics на Windows, эффективный corporate
resolver/PAC, shared UID и реальные сетевые paths. Их отсутствие не заполняется
абсолютами. Независимый review руководителя и product acceptance — отдельно.

## 9. R06-012-D — UID closure: захват VPN не равен принадлежности app-rule

**Дополнение 2026-10-28.** Исследуется остаток A/C: shared UID, SDK sandbox, package visibility и package churn. Результат **READY_FOR_REVIEW**, source/conditional model, не runtime defect.

### 9.1 Какие отображения требуется различать

Ранее capture C и lockdown exceptions E уже разделены. Теперь вводим точные
source-отображения в фиксированном user u и моменте t:

- `uid(p,u,t)` — UID текущей установки пакета p, **не вечная identity**.
- `s(uid)` — corresponding SDK sandbox UID, когда он применим. В AOSP16
  `Vpn.getAppsUids` добавляет uid и s(uid) для application UID.
- `V_c(uid,t)` — packages, возвращённые обычному caller c PackageManager;
  это результат видимости, не обещание полного списка владельцев.
- `O(f,t)` — UID владельца TCP/UDP socket по inet_diag, если найден и caller
  допущен к наблюдению этого VPN scope.
- `R_p(f)` — match конкретного app-rule p по native metadata.

Из `O(f) ∈ C` **не следует** `p ∈ V_c(O(f))` или `R_p(f)=true`. Первый факт
о захвате, остальные — о принадлежности метаданных и правиле. Равенство требует
явно доказанной связи между package identity, UID и runtime attribution.

Это не статистическая формула; логическое различие проверено по цепочкам ниже.
Значения t — разные read instants, не атомарный system snapshot.

### 9.2 Shared UID и package visibility: что уже защищено в Snowden

Первичная документация PackageManager говорит «all packages» для sharedUserId,
но реализация `ComputerEngine.getPackagesForUidInternalBody:2004–2033`
фильтрует каждый пакет по installed(user) и `shouldFilterApplication` для caller.
`ApplicationPackageManager:1167–1191` делегирует через invalidated cache, не
нормализует ответ в полную identity. В русской Android Developers странице
package visibility указано: targetAPI30+ получает фильтрованный состав,
`getPackageInfo`/поиск приложения не гарантируют наблюдение всех установленных.

Следовательно, длина `getPackagesForUid(uid)==1` сама по себе не доказывает,
что UID обслуживает ровно один установленный пакет. **Не делаем ошибочный
вывод о gap Snowden:** `AndroidAppIdentity.read:87–93` дополнительно отвергает
`PackageInfo.sharedUserId != null`, включая declared shared group с одним
видимым участником. Сохранён и отказ при нескольких package names. Это
существенный guard, не повод внедрить новое broad package inventory.

`sharedUserId` deprecated с API29, но старые установки не исчезают от этого:
официальный русский manifest reference различает existing install и fresh
install с `sharedUserMaxSdkVersion` API33. Не объявляем все современные Android
системы свободными от shared UID и не разрешаем неоднозначную app identity
только потому, что `targetSdk=36`.

### 9.3 SDK sandbox: точный source/caller контрпример

Для application UID a и sandbox UID s(a) source показывает следующие шаги:

1. `Vpn.java:1838–1851` строит capture/exemption UID ranges: package p → a и
   **s(a)**. Поэтому sandbox traffic может быть охвачен тем же VPN, что p.
2. `ConnectivityService.getConnectionOwnerUid:12939–12959` получает socket UID
   через InetDiagMessage, проверяет принадлежность VpnService-based VPN scope
   и owner-caller. Возвращает найденный UID, не переписывает s(a) в a.
   `InetDiagMessage:202–294` читает `idiag_uid` (UDP дополнительно имеет
   wildcard lookup); это socket owner, не signer-bound semantic app identity.
3. `ComputerEngine:1991–2001` для SDK sandbox UID использует
   `getBaseSdkSandboxUid()`; :5877–5878 возвращают UID **пакета SDK sandbox**.
   Это не `Process.getAppUidForSdkSandboxUid`, который сопоставляет клиенту a.
   Returned package зависит и от visibility; возможны sandbox package либо
   null/empty, а не гарантированно клиентское p.
4. Actual Snowden `LibboxPlatform.findConnectionOwner:105–126` сохраняет raw
   uid в `ConnectionOwner.userId`; `packagesForConnection` читает обычный
   PackageManager. В просмотренном пути нет s(a)→a normalization или отдельного
   поля «исходный socket UID / attribution UID».
5. Candidate app-rule строится `NativePolicyLowering.selector:68–74` как AND:
   inbound + **Uid(a)** + **Package(p)**. `NativePolicyPredicate.Uid` — exact
   equality; Package — any observed group member, не signer approval.
   libbox wrapper `service.go:213–221` переносит metadata как есть; pinned
   sing-box `UserIdItem.Match` проверяет exact map key, PackageNameItem — список.

**Ограниченный counterexample:** если поток реально имеет `O(f)=s(a) != a`,
owner query допустим/успешен и эта metadata достигла candidate app-rule,
Uid(a) уже false. Даже нахождение p в package list не спасает conjunction;
при returned sandbox package также ложен Package(p). Значит capture sandbox
не доказывает совпадение точной app-rule по клиентскому UID.

Это **не утверждение, что в current Snowden работает обход BLOCK**:
- `AndroidTunFactory` по-прежнему full capture, package split отвергается;
- native lowering возвращает SourceCandidate и **уже** выставляет
  STRICT_OWNER_CAPABILITY_NOT_VERIFIED, PACKAGE_LIFECYCLE_NOT_FENCED и parity
  blockers для app-rule;
- returned metadata может быть unavailable, callback может завершиться error,
  rule может не быть admitted, session — остановиться. Не измерены доставка
  sandbox traffic и deployed graph/default disposition;
- действующий default tunnel может сохранить защиту при невыбранной app-rule.

Counterexample также неприменим, если app вообще не использует sandbox sockets
или отдельный validated adapter связывает sandbox с owner до rule evaluation.
Такой adapter не найден в **названных** текущих caller'ах; не заявляется его
отсутствие во всех Android/VPN implementations.

### 9.4 Public reuse и API границы

Android Process reference: `isSdkSandboxUid(int)` и
`getAppUidForSdkSandboxUid(int)` — **public API35**; повторное отображение
`getSdkSandboxUidForAppUid(int)` также API35. В AOSP `toSdkSandboxUid`
используется платформой, но помечен `@hide`, MODULE_LIBRARIES — это **не**
готовый публичный вызов для minSdk29 Snowden.

Предложение для отдельного platform-review: не менять raw owner без следа;
при supported API/валидированном mapping сохранять отдельные socket UID,
attributed app UID, mapper/version/capability и current signer-bound material.
При неизвестном mapping вернуть unsupported/unknown, не угадывать фиксированным
арифметическим offset и не выдавать SDK sandbox package за клиентское app.
API29–34 fallback требует собственного обоснования; broad privileges/root/
QUERY_ALL_PACKAGES из этого исследования не следуют.

Это proposal, не реализованный интерфейс. Публичная availabilityAPI35 не
доказывает сохранение socket attribution при делегировании сети сервисам,
proxy или OEM модификациях; они вне выбранного counterexample.

### 9.5 Package churn и недостоверное обещание мгновенного обновления

`VpnManagerService:875–910` на non-replacing add/remove вызывает
`refreshPlatformVpnAppExclusionList`; replacement выбранного always-on пакета
может инициировать restart. Но `Vpn.updateAppExclusionList:4361–4376`
пересчитывает UID ranges **только** при platform IKEv2 runner.
Не переносим эту refresh branch на обычный VpnService Builder или объявляем
все прочие refresh paths отсутствующими. DPM документация прежнего A всё равно
требует admin-maintained актуальности своего lockdown allowlist; этот setter
и platform app-exclusion storage — не один набор.

В Snowden selected app monitor уже регистрирует package events, advance epoch,
invalidates old selection, rereads installation and signer material; named
unrelated event может быть проигнорирован. Worker watchdog и callback guards
помогают обнаружить подтверждённое изменение. Однако:
- broadcasts/coalescing 250ms — не атомарная блокировка PackageManager mutation;
- два подряд чтения одной installation — не непрерывный fence;
- core `route/process_cache.go:24–35` может вернуть owner cache result без
  повторного Kotlin callback; `router.go:150–152` задаёт 200ms lifetime.
  Key состоит из network/source/destination, не installation epoch;
- source guards сами **не** дают доказанного максимального окна неправильного
  app-rule/default. Delivered events, cache expiration, callback timing,
  tuple reuse и реальный retirement требуют отдельной совместной приёмки.

Отсутствие named package event не означает неизменность всей UID-группы.
Положительные safeguards названы, но нельзя подписать «app policy стала
атомарной» на основании reader+monitor+cache только по исходникам.

### 9.6 Короткий контракт проверки и безопасное следствие

Перед будущим app-specific enforcement установить:
1. Отдельны ли raw socket UID, attributed client UID и видимый пакет?
2. Доказана ли current installation identity, включая shared UID/видимость?
3. Поддерживает ли версия mapper sdk/isolated owner scope без скрытых APIs?
4. Покрывает ли admitted graph необходимые приложения и соответствующие UID?
5. Как invalidation reaches **cache, new admission, already-owned flows и
   retirement**, и что остаётся неизвестным?

Из этого следует адресное дополнение к existing STRICT_OWNER_CAPABILITY gate,
не новая VPN/MDM-система, не разрешение broad direct и не patch parser.
Данные хранить scoped/redacted: package inventory и workplace users не собирать.
Применимость shared-UID deny/default, sandbox-mapping и package-churn — разные
cases; их нельзя объединить одним «включён per-app» boolean.

### 9.7 Новые finite cases — NOT_RUN

| ID | Что нужно проверить в отдельно разрешённой изоляции |
|---|---|
| D01 | Shared UID с несколькими visible packages: rejected identity; не first-member match |
| D02 | Shared UID с одним visible package: sharedUserId guard сохраняет отказ; видимость не выдаётся за completeness |
| D03 | Fresh installAPI33+ versus прежняя sharedUserMaxSdkVersion установка: разные UID outcomes |
| D04 | SDK sandbox socketO(f)=s(a): captured versus candidate Uid(a) match, metadata/null/default отдельно |
| D05 | API35 mapping и API29–34 unsupported: raw UID не стирается, no hidden arithmetic fallback |
| D06 | Package add/remove refresh у platform IKEv2 versus ordinary VpnService: не предполагать общий механизм |
| D07 | Install replacement/UID reuse + cached 5-tuple metadata: epoch/retirement/cache behavior, не объявлять окно ≤200/250ms |
| D08 | Kernel/socket lookup missing или foreign VPN owner: INVALID_UID/unknown, не approve по package name |

Все исполнения NOT_RUN; ни одного native/package/MDM действия не выполнялось.
Source conclusions относятся к указанным pins/callers. Проверка Android/OEM,
полный eBPF/kernel путь и application rule admission/доставка не установлены.
