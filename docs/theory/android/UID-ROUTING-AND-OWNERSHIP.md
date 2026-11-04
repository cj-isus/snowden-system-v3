<a id="android-uid-ownership-2026-10-28"></a>
# Android VPN: UID capture, lockdown и текущий владелец

[Оглавление](../README.md) · [Основная Android-теория](PLATFORM-AND-UX.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-14.** Нормативный API и source snapshot Android16/API36,
`android-16.0.0_r1`. AOSP frameworks/base:
`99b01a65cc4c104933788b3143285ab6bae65827`; Connectivity:
`627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`; netd:
`68859d33e9bfe9ddb1afdc282905c63339c1928d`. Это не доказательство поведения
каждого OEM, актуальных security patches или Android-приложения Snowden.
AND-статусы и код его владельца не меняются.

## 1. Кого перехватывает VPN и кому запрещён non-VPN — разные множества

В [Vpn.java](https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/services/core/java/com/android/server/connectivity/Vpn.java?format=HTML)
`agentConnect→createUserAndRestrictedProfilesRanges` строит capture UID-набор,
а `setVpnForcedLocked` строит lockdown-набор, исключая package VPN-владельца
и административную lockdown allowlist. Включение собственного package в
TUN capture поэтому не делает его репрезентативным для проверки lockdown
обычного, не освобождённого UID.

Package-привязка разрешается в UID конкретного user. Shared UID не разделяется
одним package-правилом; snapshot также учитывает SDK sandbox UID.
Наследование ranges **restricted profiles** не равно managed/work-profile
наследованию: проверяются другой тип и parent relation. Для своего VPN
restricted user имеет отдельный запрет.

Административная `DevicePolicyManager.setAlwaysOnVpnPackage(...,lockdownAllowlist)`
не является Builder allowlist. Ей нужны device/profile-owner полномочия;
она допускает исключения при недоступном VPN, а подключённый VPN ещё должен
включить эти apps в capture. Builder exclusion само по себе не снимает lockdown.
`allowBypass` в AOSP учитывается вместе с `!mLockdown`.
[DPM API](https://developer.android.google.cn/reference/android/app/admin/DevicePolicyManager?hl=en).

**Контрпример:** TUN отсутствует, lockdown сохранён. VPN-owner UID имеет
разрешённый bootstrap/direct доступ, обычный UID блокируется. Успех probe
внутри VPN-приложения не подтверждает барьер другого UID; его direct-доступ
сам по себе также не доказывает утечку обычного app. Нужен предметный scope.

## 2. Разрешить family не значит направить её через TUN

`makeLinkProperties` начинает с `allowIPv4/allowIPv6`, затем разрешает family
при соответствующем address, unicast route **или DNS-server address**.
Для неразрешённой family добавляется default `RTN_UNREACHABLE`.

Отсюда важное различие:

- v4 TUN без каких-либо v6-входов может блокировать v6;
- добавление **одного IPv6 DNS-server** разрешает v6 family, но не создаёт `::/0`
  через TUN;
- при разрешённой family без capture route возможен fall-through, если другая
  действующая policy его не запрещает;
- при lockdown для обычного UID соответствующий non-VPN путь должен быть
  заблокирован; это не доказательство работающего v6 через VPN;
- `excludeRoute` (API33) использует `RTN_THROW`, а не reject.

[Builder API](https://developer.android.google.cn/reference/android/net/VpnService.Builder?hl=en)
и `makeLinkProperties` подтверждают различные механизмы. Proof IPv4 не
переносится на IPv6, другой user/profile или иную exemption class.

## 3. Underlying declaration, bind и protect выполняют разные действия

По public API `setUnderlyingNetworks`:

| Значение | Сообщаемая информация |
|---|---|
| null | Используется системная default network без собственного списка |
| Пустой массив | VPN не имеет underlying connection |
| Список N1,N2 | Используются перечисленные underlying networks в порядке предпочтения |

Setter проверяет установленного owner, копирует массив и передаёт его
NetworkAgent. Дальнейшая доставка асинхронна. `true` не подтверждает actual
socket migration, egress или закрытие старых потоков. `[]` не является
подтверждённым firewall/retire barrier.
[VpnService API](https://developer.android.google.cn/reference/android/net/VpnService?hl=en),
[NetworkAgent](https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/framework/src/android/net/NetworkAgent.java?format=HTML).

netd `PROTECT_FROM_VPN` проверяет право UID и меняет mark конкретного socket.
Он сам не задаёт gateway allowlist и TTL исключения. `denyProtect` запрещает
следующие такие действия, но его функция не закрывает все уже существующие
transport sockets. Ограничение «только gateway/bootstrap до deadline» остаётся
обязательством приложения и его owners.
[FwmarkServer](https://android.googlesource.com/platform/system/netd/+/68859d33e9bfe9ddb1afdc282905c63339c1928d/server/FwmarkServer.cpp?format=HTML),
[NetworkController](https://android.googlesource.com/platform/system/netd/+/68859d33e9bfe9ddb1afdc282905c63339c1928d/server/NetworkController.cpp?format=HTML).

Контрпример: socket явно bound к Wi-Fi, приложение сообщило `[cellular]`.
Декларация не перепривязывает socket и не выдаёт receipt его retirement.
DNS lookup realm дополнительно рассматривается в
[proxy/NAT64 note](../observability/PROXY-DNS-NAT64.md).

## 4. prepare() не read-only permission getter

В AOSP `Vpn.prepare` ранее одобренное приложение может вызвать
`prepareInternal` и сменить текущего обычного VPN-owner без нового диалога.
Другой always-on VPN — отдельная запрещающая ветвь. Поэтому фоновая
«проверка разрешения» этим вызовом способна иметь lifecycle side effect.

Затем `establish()` **заново** проверяет owner UID и consent AppOp. Утрата
полномочия даёт null; неправильное объявление service/BIND_VPN_SERVICE —
отдельная SecurityException. Ранее полученный RESULT_OK не резервирует право
до будущего establish.

`prepareInternal` сначала отключает interface/agent, затем отправляет revoke
через one-way Binder. `onRevoke` поэтому не является началом блокировки и не
подтверждает завершение workers/закрытие принадлежащих приложению fd.
При смене TUN API допускает ещё валидный старый fd, но новый outbound traffic
уже направляется в новый interface. `updateLinkPropertiesInPlaceIfPossible`
может сохранить NetworkAgent; стабильный netId не заменяет generation TUN.
Первичка — `prepare`, `prepareInternal`, `establish` в закреплённом Vpn.java.

## 5. Предмет proof и будущая приёмка

Минимальная модель scope:

`user/profile · UID/capture/exemption class · inner/outer family · route scope · TUN generation · socket protect/bind · network epoch · время`.

Это модель интерпретации, не требование логировать реальные UID/app inventory.
Для diagnostics нужны разрешённые typed факты и честные unknown.
Reuse platform ownership/epoch machinery и проверки результата protect;
не вводить второй VPN owner. Цена — раздельная приёмка представительских UID,
families, transitions и административных исключений.

Cases **NOT_RUN**: Cancel/Allow consent; revoke между prepare/establish;
pre-consented A против обычного/always-on B; owner/обычный/DPM-exempt UID при
lockdown без TUN; v4-only против v4+v6-DNS безv6-route; personal/work/restricted
profiles; shared UID/sandbox; новыйTUN при старомfd/netId; bindingWi-Fi при
declarationcellular/empty; actual cleanup после revoke. Эти ожидания относятся
к source/API, а device/OEM evidence получают отдельно.
