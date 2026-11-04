# Android: платформа, мобильность и UX

[Оглавление теории](../README.md).

Углубление 2026-10-28: [UID capture, lockdown, family и текущий VPN-owner](UID-ROUTING-AND-OWNERSHIP.md).
Это tagged AOSP/API evidence; текущие AND-статусы и device/OEM acceptance
остаются у владельца Android.

Разделы перенесены из авторского сборника с сохранением исходной нумерации, дат и границ доказательства. Исторический статус не является текущим product PASS.

- [§12 — 12. Android: пределы защиты, мобильная адаптация и повседневный UX](#source-12)
- [§12.1 — 12.1. Модель защиты и ограничения платформы](#source-12-1)
- [§12.2 — 12.2. Владение VpnService, native-ядром и доказательством готовности](#source-12-2)
- [§12.3 — 12.3. Смена Wi-Fi/LTE, DNS и область правил](#source-12-3)
- [§12.4 — 12.4. Российские блокировки: наблюдения → альтернативы → ограничения](#source-12-4)
- [§12.5 — 12.5. Доверие, офлайн-перенос и обновления](#source-12-5)
- [§12.6 — 12.6. Батарея и скорость: мобильные бюджеты](#source-12-6)
- [§12.7 — 12.7. Что заимствовать: реальные реализации и лицензии](#source-12-7)
- [§12.8 — 12.8. UX: удобство из известных практик и проверяемые гипотезы](#source-12-8)
- [§12.9 — 12.9. Публикация и пределы текущих выводов](#source-12-9)
- [§12.10 — 12.10. Первичные Android-источники](#source-12-10)
- [§13.8 — 13.8. Android: батарея и смысл изменения сети](#source-13-8)
- [§15.5 — 15.5. Полный Android research handoff — 35 решений](#source-15-5)

---

<!-- source-unit: 12 -->
<a id="source-12"></a>
<a id="android-theory"></a>
## 12. Android: пределы защиты, мобильная адаптация и повседневный UX

Исследование 2026-10-21…22. Это дополнение к §§0–11, а не замена прежней
теории. Нормативная Android-декомпозиция и статусы — ANDROID-PLAN — `docs/ANDROID-PLAN.md`.
Метод: первичные Android API/docs, измерения исследователей, выборочное чтение
исходников и лицензий десяти проектов. Это desk research; APK не устанавливались,
пользовательские сессии не наблюдались, usability и сетевые эксперименты не проводились.
Успешный upstream build и пригодность его кода для наших контрактов не доказаны.


<!-- /source-unit: 12 -->

<!-- source-unit: 12.1 -->
<a id="source-12-1"></a>
<a id="android-platform"></a>
### 12.1. Модель защиты и ограничения платформы

`VpnService` получает IP-пакеты через TUN в пределах Android user/profile.
Один профиль допускает один активный VPN; запуск другого может отозвать наш.
Рабочий профиль, личный профиль, tethering и трафик системных компонентов — разные
области. Их нельзя объединять подписью «весь телефон защищён». Always-on запускает
выбранный VPN, но установление туннеля остаётся ответственностью приложения. [A01]

Разделять три факта: **намерение пользователя**, **состояние приложения** и
**системную блокировку без VPN**. Собственный reject внутри работающего TUN не
является барьером после смерти приложения. Для строгого режима требуется проверяемая
конфигурация Android «Постоянная VPN» + «Блокировать соединения без VPN»; где она
недоступна или не включена, честно показывать ограниченную защиту. Ручное отключение,
системный revoke, crash, force-stop и исчезновение сети — разные переходы. ОС может
не разрешить приложению самопроизвольно восстановиться после force-stop. [A01][A03]

Нельзя обещать, что исключённый банковский клиент одновременно работает напрямую
и сохраняет строгий lockdown: системная блокировка может перекрыть исключённые
приложения. Сначала объяснить конфликт и фактическую область политики. Не предлагать
`allowBypass()` как исправление; оно разрешает обход VPN. [A02][A31]

**Остаточный риск ОС.** Mullvad публиковал отчёты о connectivity-check трафике
(2022), DNS (2024) и определённом QUIC-трафике Android 16 (2026), выходящем вне VPN,
в том числе при системной блокировке. Это сообщения конкретных исследований,
не доказательство уязвимости каждого актуального телефона. Версия ОС, security patch,
OEM и внешнее наблюдение обязательны; состояние исправлений на нашем устройстве
не проверено. Приложение не может обещать устранение дефекта ядра/системы. [A34][A35][A36]


<!-- /source-unit: 12.1 -->

<!-- source-unit: 12.2 -->
<a id="source-12-2"></a>
### 12.2. Владение VpnService, native-ядром и доказательством готовности

Рекомендуемый раздел ответственности: native Kotlin service владеет жизненным
циклом, libbox — сетевым ядром, UI читает неизменяемые снимки. Activity/Compose,
плитка и уведомление отправляют команды одному владельцу; их пересоздание не
должно перезапускать VPN. На Android нет Windows Job/WFP/DPAPI/Wails lifecycle.
Приватный процесс `:vpn` может изолировать падение UI, но сам по себе не добавляет
системный fail-closed и не защищает от native-зависания.

Контракт операции: `(intentRevision, serviceInstance, networkEpoch, configDigest,
attemptId)` + монотонное время. Запоздалый JNI/callback/probe результат не меняет
новую сессию. Отмена coroutine не доказывает прерывание блокирующего native-вызова:
нужны реальный cancel/close, ограниченная очередь и проверяемое завершение ресурсов.
Если исход завершения неизвестен — `blocked/unknown`, без параллельного нового ядра.

Foreground service должен вовремя перейти в foreground и корректно обрабатывать
revoke/destroy/restart. Для VPN Android документирует категорию `systemExempted`
при выполнении условий; тип, permission и eligibility проверяются для выбранного SDK.
`specialUse` из чужого proxy-service нельзя автоматически переносить. Отказ в
`POST_NOTIFICATIONS` не равен запрету запуска FGS: статус разрешения и работа VPN
должны отображаться отдельно. [A05][A19]

Каждый fd имеет единственного владельца: определить `ParcelFileDescriptor`,
borrow/dup/detach и порядок закрытия TUN/transport sockets/monitor callbacks.
`protect(fd)` выводит **конкретный сокет** из VPN для связи с gateway; требуется
проверить boolean и отказать при неуспехе. Это не проверка шифрования. Не исключать
целиком наш package: внутренний protected probe тогда тоже может выйти напрямую.
Разрешение на bootstrap/protected socket ограничено назначением и сроком. [A03]

«Ядро запущено», иконка ключа, `NET_CAPABILITY_VALIDATED`, успешный handshake и
свежий protected egress — разные факты. Снимок готовности включает версию профиля,
реальную область перехвата, свежесть/возможности проверки и причину ограничения.
Сбой measurement-сервиса не превращается автоматически в отказ канала.


<!-- /source-unit: 12.2 -->

<!-- source-unit: 12.3 -->
<a id="source-12-3"></a>
### 12.3. Смена Wi-Fi/LTE, DNS и область правил

Отслеживать физические `Network`/capabilities/LinkProperties, исключая сам VPN;
отдельно учитывать default network, validated/captive/metered/roaming и смену DNS.
Смена default не означает уничтожение прежней сети. Callback не выполняет тяжёлую
работу; изменения сводятся в новую epoch с bounded debounce. Подписки освобождаются.
`setUnderlyingNetworks` информирует ОС о выбранной основе, но не переносит любой
TCP-сеанс. Глобальный `bindProcessToNetwork` опасен для внутренних проверок. [A03][A04]

QUIC migration (§9, [S23]) может сохранить подходящее соединение после смены пути
к тому же серверу; она не переносит произвольный TCP/UDP поток на другой exit.
Разные SIM/ASN/страны/протоколы могут иметь общие origin/CDN/DNS и не давать
независимости. Полезность второго gateway измеряется по failure domains и реальному
восстановлению; смартфон не создаёт разрешённого пути туда, куда оператор не пускает.

Два уровня split: системный `VpnService.Builder` выбирает приложения, libbox
маршрутизирует уже захваченный трафик. Allowed/disallowed package lists взаимоисключены;
изменение может потребовать нового TUN. Удалённый пакет, shared UID, isolated process,
переустановка и рабочий профиль требуют явной семантики. Owner query API29+ помогает
атрибуции TCP/UDP, но не даёт права выдумать owner для любого пакета. Unknown identity
не получает direct. Windows `process_name` нельзя считать Android package policy. [A02][A03]

DNS — часть политики: resolver внутри VPN, bootstrap resolver и DNS исключений
имеют разные scopes. Private DNS, встроенные DoH/DoQ приложений, ECH и общий CDN IP
ограничивают доменную атрибуцию; «домен в списке» не доказывает перехват всех потоков.
Проверять IPv4/IPv6/AAAA, IPv6-only + NAT64/464XLAT и фактическую MTU. Нельзя
лечить IPv6 отсутствием маршрута, которое незаметно открывает обход. `allowFamily`,
route exclusions и отсутствие явного DNS требуют отдельной проверки. [A02][S07]

LAN/печать/casting — ограниченное правило для выбранной сети, без автоматического
доверия всему RFC1918 и без обхода корпоративных overlapping routes. Для target37+
Android документирует `ACCESS_LOCAL_NETWORK`; target≤36 не следует запрашивать
его только из-за наличия в upstream manifest. Отказ и отзыв разрешения — отдельные
состояния. Границы фактического relayed LAN-трафика через VPN уточняются native spike,
а не выводятся из одного INTERNET permission. [A15]


<!-- /source-unit: 12.3 -->

<!-- source-unit: 12.4 -->
<a id="source-12-4"></a>
<a id="android-ru"></a>
### 12.4. Российские блокировки: наблюдения → альтернативы → ограничения

| Класс | Основание и предел вывода | Что меняет Android-адаптация |
|---|---|---|
| DNS/SNI/IP/ASN | Модель §§2/7; отдельный timeout не устанавливает механизм | Разделять resolver/transport/egress/service проверки и историю по physical network epoch |
| Обрыв после небольшого объёма | Cloudflare: наблюдения с 2025-06-09, публикация 2025-06-26, отдельные сети и инфраструктура, TCP и QUIC [S04] | Нужна bounded bulk-проверка наряду с small HTTPS; не зашивать «15/16 КБ» как универсальный классификатор |
| Блокировка конкретного сервиса | OONI, 2026-03-30: Telegram, период 02-28…03-30, покрытие 547 AS; рост аномалий с 03-20, виден не в каждой сети [A32] | «VPN работает» и «Telegram готов» — разные scoped факты. Сервисный сбой не требует немедленного переключения всего телефона |
| Мобильный destination allowlist | Beeline field report 2025-09-07 [A33]; SNI-подмена на собственном VPS не помогла в том опыте | Замена SNI не открывает запрещённый destination. Показывать ограниченную достижимость, безопасно ждать/переизмерять |
| Новый transport работает у части абонентов | Issue650, 2026-08-19: три MTS тестера, без данных других операторов [A37] | Это гипотеза кандидата, не nationwide PASS; различать обычный DPI и allowlist режим |
| UDP/QUIC деградируют | Модель §2 и протокольные свойства [S06][S23][S24] | Держать независимую TCP-альтернативу; не обещать HY2 при блокировке UDP и не выдавать TCP-only probe за UDP proof |
| Доставка/обновления недостижимы | Общий CDN/origin, DNS/bootstrap могут ломаться вместе с tunnel | Офлайн подписанный профиль и проверенный last-known-good; зеркало не получает право менять trust root |
| Не цензура | Неоплаченный VPS, истёкший credential, часы, captive portal, OOM/Doze, лимит трафика, service geo-block | Отдельная диагностика и действие: оплатить/обновить/войти в portal/исправить политику, не бессмысленный перебор маскировок |

OONI-аномалии могут иметь иные причины; coverage неоднороден [S05]. Ссылки на
чужие новости внутри отчёта OONI не превращаются в собственные измерения OONI.
Публичные OONI-измерения не запускаются приложением автоматически. Для РФ полевого
опыта фиксировать оператор/AS/регион/день/тип сети, transport и endpoint, а не IMSI,
номер телефона, точные координаты или содержимое пользовательского трафика.

Решение адаптации — `stay → scoped recheck → validated candidate → commit` либо
`blocked`; без direct fallback защищённого трафика. Выбор минимизирует ожидаемый
перерыв/энергию/байты при safety constraints (§8), а не максимизирует абстрактный score.
Ни ECH, ни domain fronting чужого CDN, ни Cloudflare build shell не дают собственного
надёжного gateway по определению. Новый transport допускается только после проверки
обоих концов, лицензии, fingerprint, бюджета и независимого полевого результата.


<!-- /source-unit: 12.4 -->

<!-- source-unit: 12.5 -->
<a id="source-12-5"></a>
### 12.5. Доверие, офлайн-перенос и обновления

Android должен принимать тот же публичный профильный контракт и **те же подписанные
байты**, а не Kotlin-пересериализацию JSON перед проверкой. В проекте уже выбран
go-tuf2.4.2 и есть непроверенная Windows source integration: ADR-001 — `docs/decisions/001-tuf-delivery.md`.
Profile epoch recovery пока только контракт ADR-002 — `docs/decisions/002-profile-epochs.md`.
Они не являются готовым Android SDK. Переносить portable schema/signature/antirollback
и штатный TUF workflow через узкий seam; UI, filesystem transaction и vault — платформенные.
Не импортировать целиком модуль `windows` ради нескольких полезных packages.

APK signing, TUF update role, profile authority и device enrollment — разные
полномочия. Подписанный профиль не разрешает произвольный APK, а подпись APK не
делает свежим старый профиль. Учесть freeze/expiry/clock uncertainty, revocation,
rollback floor, exact source digest, offline valid cache и управляемое восстановление.
Недоступность доставки не обнуляет сохранённую доверенную историю. [S20][A24]

Android Keystore хранит ключ обёртки vault; hardware backing проверяется, а не
предполагается. Требование биометрии при каждом decrypt несовместимо с unattended
always-on. Credential-encrypted storage до первого unlock недоступно: безопасный
вариант pre-unlock — честно blocked, если секреты не готовы; не складывать credentials
в device-encrypted storage ради красивого autoconnect. [A07][A08]

Auto Backup/перенос телефона не должны восстанавливать ciphertext без рабочего
Keystore или старый trust checkpoint как новую чистую установку. Отделять разрешённые
несекретные настройки от vault и trust history, давать повторное enrollment без
неявного сброса antirollback. Экспорт — явный encrypted bundle; raw credential export
имеет отдельно объяснённую потерю device revocation/policy гарантий. [A09]

Импорт начинается без учётной записи/GMS: файл через системный picker, QR с камеры
или выбранного изображения, ограниченный deep link. ZXing — готовый offline decoder,
не новый QR-алгоритм. Ограничить входные байты/размер картинки/схемы URI/редиректы;
preview и подпись перед применением, никаких auto-connect/авторизации из внешнего Intent.
Token URL, clipboard и QR не попадают в логи/recents/support export. [A20][A21][A25]


<!-- /source-unit: 12.5 -->

<!-- source-unit: 12.6 -->
<a id="source-12-6"></a>
### 12.6. Батарея и скорость: мобильные бюджеты

Событийная адаптация предпочтительнее бесконечного таймера/foreground polling.
Doze, sleep и OEM power policy меняют доступное время исполнения; foreground service
не гарантирует вечную работу. Не просить всем отключить энергосбережение и не держать
wakelock постоянно. Периодическая задача обновлений не должна быть VPN keepalive. [A06]

Отдельные бюджеты: dial/handshake/probe bytes, candidate fanout, retries/backoff,
CPU/RSS/native heap/Go goroutines, fd/buffers, wakeups и metered/roaming traffic.
Не копировать безусловный SFA `setMetered(false)`: ОС должна знать реальные ограничения.
Режим «Не мешать сеансу» уменьшает необязательные проверки/переключения; обрыв пути
и потеря доказанной защиты остаются причиной безопасного действия. Слушать микрофон
или читать экран для обнаружения звонка не требуется.

Временные разрешения/TTL используют elapsedRealtime, учитывающий sleep; после
reboot временный grant не продлевается из-за сброса счётчика/смены wall clock. [A23]
Метрики сравнивать на одном устройстве/радиоканале/нагрузке: idle, screen-off,
handover, video/call, mobile data, throughput/latency/loss и стоимость проверок.
Чужие Mbps и RAM без workload не становятся требованиями нашей реализации.


<!-- /source-unit: 12.6 -->

<!-- source-unit: 12.7 -->
<a id="source-12-7"></a>
<a id="android-reuse"></a>
### 12.7. Что заимствовать: реальные реализации и лицензии

Зафиксированы 10 commit snapshots, 57 файлов, 10 корневых LICENSE; manifests,
ключевые lifecycle/bridge/UI файлы прочитаны выборочно. SHA256 и точные paths —
evidence — `docs/evidence/2026-10-21-android-research.md`. Это не полный security audit
репозиториев и не проверка transitive лицензий. Полный компонент брать только
после совместимости с нашей лицензией/notice/SBOM и контрактами владения.

| Проект / фиксированный commit | Реальный материал и решение | Что не переносить автоматически |
|---|---|---|
| [SFA bc73504](https://github.com/SagerNet/sing-box-for-android/tree/bc73504fcb858400a939140ad785ed92f4c9f429) | GPL-3.0-or-later; `VPNService.kt`, `BoxService.kt`, `PlatformInterfaceWrapper.kt`, `DefaultNetworkMonitor.kt`, tile/import. **Основной Android bridge адаптировать**, без переписывания packet engine | В прочитанном коде protect-result игнорируется, есть `allowBypass`, unconditional unmetered, глобальный monitor и продолжение после missing package. Нужны наши safety fences; не копировать Xposed/remote shell/providers/location permissions |
| [sing-box dfb8034](https://github.com/SagerNet/sing-box/tree/dfb80349bf5779ec0a3f834c9ec2794345bff8f0) + [v1.13.19](https://github.com/SagerNet/sing-box/tree/v1.13.19/experimental/libbox) | GPL-3.0-or-later; `platform.go`, `tun.go`, `service.go`, `build_libbox`. **Использовать ядро/libbox целиком** с фиксированным pair и минимальным Go facade | HEAD interface уже отличается от pinned1.13.19. Версии AAR/Kotlin wrapper/build tags/gomobile/NDK/ABI должны совпасть; новые XHTTP/transport не выводить из HEAD docs |
| [Mullvad 3d6146a](https://github.com/mullvad/mullvadvpn-app/tree/3d6146a416a44cbd906f8b34acc29485cf7c36a9) | GPL-3.0 family; `MullvadVpnService.kt`, split ViewModel и auto-connect UI wrapper. **Адаптировать service ordering, объяснение lockdown и split UX** | Rust/Talpid/серверную инфраструктуру не подменять Go-ядром; прочитанный AutoConnect ViewModel не является аудитом системного enforcement |
| [Proton 316a3d4](https://github.com/ProtonVPN/android-app/tree/316a3d499e120bd2cab63c2cd8ac396e3ab3b24a) | GPL-3.0 family; `CreateEditProfileViewModel.kt`, `SplitTunneling.kt`, notification/manifest. **Адаптировать профили задач, preview и включения/исключения** | Billing/accounts/телеметрию и vendor backend не включать ради UI; split/kill-switch совместимость проверять отдельно |
| [Amnezia 02838a9](https://github.com/amnezia-vpn/amnezia-client/tree/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc) | GPL-3.0; `AmneziaVpnService.kt`, `AppListProvider.kt`. **Адаптировать app selection/self-host onboarding опыт**, AWG — отдельный transport spike | Qt оболочку и весь второй стек не добавлять одновременно с native Compose; не считать AWG поддержанным pinned libbox |
| [v2rayNG 118cf5b](https://github.com/2dust/v2rayNG/tree/118cf5bc7d0b290947b0827f5ad03e73018d8e5d) | GPL-3.0; `CoreVpnService.kt`, manifest/build. **Референс protect/routing и lifecycle второго ядра** | Xray AAR не drop-in libbox; исключение своего package не подходит нашему protected probe |
| [Hiddify f26aadb](https://github.com/hiddify/hiddify-app/tree/f26aadb351fb23c222e06fcf4f5350d0ba596bb9) | `LICENSE.md` — Extended GPLv3 с дополнительными условиями. **Пока только reference UX/platform** | Не помечать обычной GPL: есть noncommercial/consent, naming/UI/attribution, GitHub/Actions условия. Код не брать до явного разрешения совместимости |
| [Now in Android 2982e63](https://github.com/android/nowinandroid/tree/2982e63cb691ad49cd009aab43dc206fee6699c2) | Apache-2.0; `NiaApp.kt`, `NiaAppState.kt`, `Navigation.kt`. **Брать подходящие adaptive navigation/lifecycle patterns и компоненты с notices** | News graph, analytics и экспериментальные HEAD Navigation3/adaptive API не нужны автоматически |
| [architecture-samples ee66e15](https://github.com/android/architecture-samples/tree/ee66e1526b84c026615df032c705842b7d2a521f) | Apache-2.0; LICENSE/build/manifest сверены. **Референс структуры**, глубина ниже NIA: реализации ViewModel здесь отдельно не аудировались | Не заявлять изученными все архитектурные алгоритмы и не копировать десятки модулей без задачи |
| [ZXing cc70a9d](https://github.com/zxing/zxing/tree/cc70a9d0b7aa04fc8b387a5eb84884946d0591cc) | Apache-2.0; LICENSE/README/`QRCodeReader.java`. **Готовый decoder целиком** за bounded import adapter | Camera/storage UI, лимиты памяти и проверка содержимого остаются нашей ответственностью; offline decode не проверяет подпись |

Outline — дополнительный кандидат для onboarding: canonical upstream теперь
`OutlineFoundation/outline-apps`; фиксированный API snapshot получить не удалось.
В успешные counts/одобренное заимствование не включён. Не делать второе ядро и не
копировать код по одному README. Подробный reuse gate — AND-01/04 в Android-плане.


<!-- /source-unit: 12.7 -->

<!-- source-unit: 12.8 -->
<a id="source-12-8"></a>
<a id="android-ux"></a>
### 12.8. UX: удобство из известных практик и проверяемые гипотезы

| Потребность | Изученный ориентир | Наше предложение / граница |
|---|---|---|
| Первый запуск без сети/аккаунта | SFA import, self-host Amnezia, offline ZXing | Получить профиль → preview authority/expiry/scope → системное VPN consent → свежая проверка. Камера необязательна |
| Быстрое ежедневное действие | SFA tile/notification, Mullvad service; native Android patterns | Один главный Connect/Stop и честное состояние; tile не разрешает новый trust или временный direct на lockscreen |
| Рабочие задачи вместо ручного редактирования транспорта | Proton profiles + Connect-and-Go [A30] | «Дом», «Работа», «Звонок», «Поездка» — policy presets со scope/ценой переключения; auto-open сервиса только по выбору пользователя |
| Банки/принтер/корпоративная сеть | Proton/Amnezia app split, Android OS rules | Preview effective rules и conflicts; package/UID ≠ вкладка браузера. Lockdown конфликт не скрывать |
| Понять «VPN включён, сервис не работает» | Наша evidence модель + отдельные service probes | Разделить gateway, egress, DNS и конкретный сервис; дать следующее действие без ложного общего FAIL |
| Складной экран, большие шрифты, управление одной рукой | NIA adaptive navigation + Compose accessibility [A17][A18] | Compact bottom bar → wide rail, list/detail; 48dp targets, TalkBack, 200% font, стабильные состояния после rotate |

Информационная архитектура: **Главная / Подключения / Правила / Система**.
Внутренние tabs — только подразделы выбранного раздела: например,
«Правила → Приложения / Сайты / Локальная сеть». Никаких дублирующих четырёх tabs
вверху и меню внизу/слева. Диагностика/история/обновление находятся в «Системе»
и доступны контекстной ссылкой из ошибки. Списки, строки параметров и один detail
предпочтительнее стены вложенных карточек. Цветовой акцент/тёмная тема поддерживают
выбранный владельцем стиль; статус дополнительно передаётся текстом и семантикой.

**Гипотезы для проверки, а не заявления об уникальности:** (1) объяснение готовности
конкретной рабочей задачи с scope/freshness; (2) короткие безопасные исключения с
видимой ценой для lockdown; (3) «Не мешать сеансу» без скрытого direct и слежения
за экраном/звонком; (4) бюджет мобильного трафика и автономный recovery packet в одном
потоке. Их интеграция может уменьшить ручное вмешательство; отсутствие всех четырёх
у каждого конкурента не доказано. Польза измеряется в AND-15/16, не числом features.


<!-- /source-unit: 12.8 -->

<!-- source-unit: 12.9 -->
<a id="source-12-9"></a>
### 12.9. Публикация и пределы текущих выводов

Native AAR означает проверку ABI, ELF/ZIP alignment, JNI/R8 и устройств с 4/16KiB
pages. Текущая Android страница указывает enforcement обновлений Google Play с
2027-02-01 для 64-bit apps target35+; это датированный policy snapshot, не причина
отложить совместимость. При релизе перепроверить. [A12]

API37 соответствует Android17 [A13]; прочитанный SFA HEAD использует compile/target37,
но это не доказывает выбранную нами stable SDK/toolchain. Release notes в выборке
недостаточны для актуального GA утверждения; AND-01 фиксирует поддерживаемый release,
NDK/AGP/Go/gomobile и Play target requirement до создания проекта. [A14]

Google Play требует обоснования VpnService/видимости пакетов и работы с чувствительными
данными; список установленных приложений остаётся локальным. Отдельный APK-канал
тоже требует signing/update/registration решения. На 2026-10-22 developer verification
страница описывает запуск 2026-09-30 в четырёх регионах и расширение в 2027; из этого
не следует, что sideload в России сейчас запрещён. [A10][A11][A26]

Статические документы не доказывают мобильную безотказность. Нужны будущие device/OEM,
screen-off/boot, external-capture и операторские сравнительные опыты, отдельно от
unit/contract/UI checks; планирование их не является разрешением запуска.


<!-- /source-unit: 12.9 -->

<!-- source-unit: 12.10 -->
<a id="source-12-10"></a>
<a id="android-sources"></a>
### 12.10. Первичные Android-источники

Страницы сверены 2026-10-21…22; mutable docs/policy повторно проверять перед релизом.
Источники S01…S27 выше сохраняют свою область. Upstream refs из §12.7 закреплены SHA.

| ID | Источник | Область |
|---|---|---|
| [A01] | Android VPN guide | Пользователь/profile, consent, always-on, lockdown |
| [A02] | VpnService.Builder API | Capture lists/routes/DNS/bypass/metered |
| [A03] | VpnService API | protect/underlying networks/revoke/OS state |
| [A04] | Reading network state | Network callbacks/capabilities/lifetime |
| [A05] | Foreground service types | systemExempted eligibility; не arbitrary specialUse |
| [A06] | Doze/Standby | Deferred execution, ограничения фоновой работы |
| [A07] | Android Keystore | Key custody, hardware/auth bounds |
| [A08] | Direct Boot | CE/DE и доступность до unlock |
| [A09] | Auto Backup | Backup/restore rules; не готовый перенос trust |
| [A10] | Play QUERY_ALL_PACKAGES policy | Sensitive package inventory/декларация |
| [A11] | Play VpnService policy | Core purpose, disclosure, endpoint encryption |
| [A12] | 16KiB pages | Native build/package/device acceptance; policy snapshot |
| [A13] | uses-sdk | API/version mapping |
| [A14] | Android17 release notes | Неполная для GA выборка, не наше обещание поддержки |
| [A15] | Local network permission | Target37+ LAN runtime gate |
| [A16] | Architecture recommendations | UDF/ViewModel/repository/lifecycle patterns |
| [A17] | Adaptive navigation | Window-based navigation |
| [A18] | Compose accessibility defaults | Touch targets/semantics |
| [A19] | Notification runtime permission | API33+, permission ≠ FGS eligibility |
| [A20] | Storage Access Framework | User-selected document/content URI |
| [A21] | Photo picker | Selected image without full gallery access |
| [A22] | Macrobenchmark | Будущие startup/UI measurements |
| [A23] | SystemClock | elapsedRealtime/deep sleep/reboot scope |
| [A24] | App signing | APK identity/key and update continuity |
| [A25] | Unsafe deep links | External intent validation |
| [A26] | Developer verification | Датированная региональная rollout policy |
| [A30] | Proton connection profiles | UX tasks/Connect-and-Go, не наша реализация |
| [A31] | Proton split tunneling | Android limits/reconnect и kill-switch conflict |
| [A32] | OONI Telegram Russia, 2026-03-30 | Измерения отдельных AS/периода и coverage limits |
| [A33] | net4people516, 2025-09-07 | Отдельный mobile allowlist field report |
| [A34] | Mullvad, 2022-10-10 | Historical OS connectivity-check leak report |
| [A35] | Mullvad, 2024-05-03 | Historical OS DNS leak report |
| [A36] | Mullvad, 2026-05-12 | Certain Android16 QUIC OS traffic; patch status device-specific |
| [A37] | net4people650, 2026-08-19 | Три MTS тестера, не общероссийская выборка |

[A01]: https://developer.android.com/develop/connectivity/vpn
[A02]: https://developer.android.com/reference/android/net/VpnService.Builder
[A03]: https://developer.android.com/reference/android/net/VpnService
[A04]: https://developer.android.com/develop/connectivity/network-ops/reading-network-state
[A05]: https://developer.android.com/develop/background-work/services/fgs/service-types
[A06]: https://developer.android.com/training/monitoring-device-state/doze-standby
[A07]: https://developer.android.com/privacy-and-security/keystore
[A08]: https://developer.android.com/privacy-and-security/direct-boot
[A09]: https://developer.android.com/identity/data/autobackup
[A10]: https://support.google.com/googleplay/android-developer/answer/10158779
[A11]: https://support.google.com/googleplay/android-developer/answer/18258653
[A12]: https://developer.android.com/guide/practices/page-sizes
[A13]: https://developer.android.com/guide/topics/manifest/uses-sdk-element
[A14]: https://developer.android.com/about/versions/17/release-notes
[A15]: https://developer.android.com/privacy-and-security/local-network-permission
[A16]: https://developer.android.com/topic/architecture/recommendations
[A17]: https://developer.android.com/develop/ui/compose/layouts/adaptive/build-adaptive-navigation
[A18]: https://developer.android.com/develop/ui/compose/accessibility/api-defaults
[A19]: https://developer.android.com/develop/ui/compose/notifications/notification-permission
[A20]: https://developer.android.com/training/data-storage/shared/documents-files
[A21]: https://developer.android.com/training/data-storage/shared/photo-picker
[A22]: https://developer.android.com/topic/performance/benchmarking/macrobenchmark-overview
[A23]: https://developer.android.com/reference/android/os/SystemClock
[A24]: https://developer.android.com/studio/publish/app-signing
[A25]: https://developer.android.com/privacy-and-security/risks/unsafe-use-of-deeplinks
[A26]: https://developer.android.com/developer-verification
[A30]: https://protonvpn.com/support/connection-profiles
[A31]: https://protonvpn.com/support/protonvpn-split-tunneling
[A32]: https://explorer.ooni.org/findings/2026-russia-blocked-telegram
[A33]: https://github.com/net4people/bbs/issues/516
[A34]: https://mullvad.net/en/blog/android-leaks-connectivity-check-traffic
[A35]: https://mullvad.net/en/blog/dns-traffic-can-leak-outside-the-vpn-tunnel-on-android
[A36]: https://mullvad.net/en/blog/any-app-on-recent-android-versions-can-leak-certain-traffic
[A37]: https://github.com/net4people/bbs/issues/650

---


<!-- /source-unit: 12.10 -->

<!-- source-unit: 13.8 -->
<a id="source-13-8"></a>
### 13.8. Android: батарея и смысл изменения сети

Keepalive, retransmit, health probe, refresh профиля и UI polling — разные работы,
но все могут будить радио.15-second schedule даёт5760 opportunities/day (M11);
это не число фактических wakeups. Бандлировать optional работу, использовать
metered/charging constraints и реальные device измерения. WorkManager не заменяет
VpnService; FCM не требуется для ядра/QR/offline recovery. [D18]

Android SocketKeepalive API29 документирует IPsec NAT-T socket; его нельзя считать
готовым offload произвольного Hysteria/QUIC keepalive. Доступность, интервал и ошибка
hardware support проверяются по API/device. RFC4787 описывает NAT requirements,
не измеренный timeout данного CGNAT.10s library keepalive не универсальный optimum;
его уменьшение/увеличение требует supported API, idle/resume/reconnect/energy опыта. [D19][D20]

Разделить identities: security/owner generation, физический path, resolver context,
стоимость metered/roaming, presentation/quality hints. Более строгая cellular policy
сразу останавливает запрещённые optional bytes; это не обязательно требует убийства
всех рабочих sockets. Изменение маршрута/привязки действительно может требовать
retire. Ослаблять parent/child admission fingerprint без доказанного альтернативного
контракта нельзя. Текущий новый Android fingerprint — WIP/source; исследования
reconnect churn не исправляют его незаметно из этого чата.

При смене сети сначала классифицировать причину/владение/capture, затем применять
нативно поддержанный path update или controlled restart. Отсутствие VALIDATED —
не отсутствие туннеля, а сильный revoke/blocked access не cosmetic hint. Не включать
requestNetwork/вторую SIM/роуминг скрытно ради цифры скорости. Два одновременно
живых native workers и новый exit не становятся seamless handover автоматически.


<!-- /source-unit: 13.8 -->

<!-- source-unit: 15.5 -->
<a id="source-15-5"></a>
### 15.5. Полный Android research handoff — 35 решений

Ниже весь пакет вопросов архитектуры, РФ, безопасности, ресурсов, бытовых/рабочих
функций, UX и distribution. P=подтверждённое прежнее source/ADR решение;
N=новое первичное уточнение. I/B/D/U/S/O — implementation/build/device/usability/
store-or-account/owner-or-infrastructure. Это research labels, не новые stage IDs.
Source inventory датирован: AND-06-S4 в packet; текущий Android owner уже продвинул
AND-06-S5. Поэтому этот пакет не откатывает composer/owner решения и не описывает
старый источник как актуальную полноту приложения. Очередь всегда android/STATE.

| Вопрос | Доказанный вывод / решение исследования | Reuse / adapt / defer | Текущий AND / A-FR / COM | Следующий gate, не новый общий research |
|---|---|---|---|---|
| 01. Stable SDK, minSdk, target | **N:** AGP8.13.2 поддерживает Kotlin2.3, Gradle8.13/JDK17 и API≤36.1. Для первого app кандидат compile/target36; с31.08.2026 Play new/update требует ≥36. API29 даёт owner query TCP/UDP. [AR01][AR02][AR03] | **Reuse** текущий compile36/min29; **defer** API26–28 и миграцию37 до отдельного запроса/среза. | AND-01/14/15; A-FR-13/14; COM-16 | **B/D:** app manifest target36, resolved graph и release на29/36/37-runtime; спрос на <29 — **O**, не техническая неизвестность. |
| 02. Совместимый core/bridge pair | **P:** точный source pair — libbox1.13.19 `b5ebaa…` + собственный bridge AND-06-S4 + strict-owner-r1. SFA HEAD имеет другой API. Stock AAR не реализует требуемый overlay. AR33 — `android/native/source-lock.json` | **Reuse** всё ядро и собственный bridge; **adapt** один whole-libbox build. | AND-01/02/06; A-FR-01/05/13 | **B:** применимость patch к exact base, одна AAR с ожидаемыми methods, provenance/digest/Java-JNI inventory. |
| 03. Go/gomobile/build recipe | **P:** core требует Go≥1.24.7, fork gomobile0.1.12; build_libbox ожидает OpenJDK17 и явные tags. Multi-package bind допускает единый libbox+trust artifact. Наличие поддерживаемого Go release не доказывает native compatibility. [AR33][AR34] | **Reuse** pinned recipe; **adapt** explicit tags/ABI и один Go runtime, без второго AAR. | AND-01/04/14; A-FR-01/03/13 | **B:** в build receipt зафиксировать actual compiler/NDK/wrapper hashes, `GOTOOLCHAIN`/flags/module graph; проверить generated API/ABI. Не заполнять неизвестный accepted compiler произвольной версией из desktop. |
| 04. ABI и16KiB | **N:** AGP≥8.5.1/NDK≥r28 дают подходящие defaults, но prebuilt libraries также должны быть16KiB-compatible. Runtime page assumptions и ELF/ZIP alignment проверяются отдельно. [AR04] | **Reuse** NDK28/arm64 основной/x86_64 lab; **defer**32-bit без спроса. | AND-01/14/15; A-FR-13/14 | **B/D:** инвентарь всех `.so`, release R8/JNI,4/16KiB load/start/upgrade; не один classes.jar checksum. |
| 05. Лицензии/SBOM | **P:** Android scoped GPL-3.0-or-later уже выбран; NIA/ZXing Apache2.0, Hiddify extended license — reference. Root license не описывает весь transitive graph/assets. AR40 — `android/LICENSE` | **Reuse** допустимый upstream; **adapt** notices/corresponding source; **defer** Hiddify copying. | AND-01/14; A-FR-13 | **B/S:** полный actual dependency/license/assets inventory, source-build instructions и соответствующая binary поставка. Это аудит будущего состава, а не неизвестный выбор core. |
| 06. Private process и отмена | **P:** `:vpn` изолирует native lifetime от UI, остаётся тем же app UID. Coroutine cancel/timeout/Close return не процессная смерть. Есть retained Binder death и один session owner. AR05 — `android/native/source-lock.json` | **Reuse** repository/actor/fixed RPC/watchdog/death fence, без второго controller. | AND-02; A-FR-01; COM-02/03 | **I/D:** blocked JNI/Binder/disk, late result, Stop→Connect и UI death; successor только после death и actual producer return. |
| 07. FGS type | **N:** `systemExempted` документирован для VPN, configured в Settings; нужны FGS и type permission. Несоблюдение eligibility даёт exception, manifest label не право. [AR06] | **Reuse** текущую manifest category; **adapt** first consent→promotion и typed refusal. | AND-02/03/14; A-FR-01/02/13 | **I/D/S:** первый connect/adopt/system start/revoke/FGS stop на API34+ и Play declaration. `specialUse` не запасной способ обойти eligibility. |
| 08. Prepare, boot, force-stop | **P/N:** consent — явный пользовательский flow; always-on lifecycle ОС не устанавливает gateway за приложение. CE недоступно до unlock; force-stop — отдельное user state. [AR05][AR18][AR39] | **Reuse** explicit-only prepare/dated OS facts; **defer** enable always-on до accepted offline recovery. | AND-03/04; A-FR-02/03; COM-03 | **I/D:** cold intent/boot после accepted cache, directBoot=false до отдельного принятого контракта; revoke/manual stop/force-stop/reboot не смешивать. |
| 09. Notification/camera/GMS | **N:** POST_NOTIFICATIONS не prerequisite запуска FGS, но service notification обязателен. Photo picker имеет ACTION_OPEN_DOCUMENT fallback. Camera нужна только camera flow; GMS не требуется ядру/SAF/ZXing. [AR07][AR14]AR15 — `docs/evidence/2026-10-21-android-research.md` | **Reuse** системные pickers/ZXing; **adapt** запрос по действию и denied state. | AND-03/04/08/12; A-FR-07/11/17 | **I/U/D:** camera denied→file/image, notification denied без ложного VPN-failed, non-GMS29 flow; no broad storage/phone/location permission без use case. |
| 10. App inventory/owner API | **N:** API29 owner query только TCP/UDP и active VpnService текущего user; invalid UID/permission error допустимы. Package visibility фильтрует targeted reads; QUERY_ALL_PACKAGES — отдельный restricted Play approval. [AR03][AR08][AR09] | **Reuse** targeted reads и typed unknown; **defer** broad inventory permission. | AND-06/08/14; A-FR-05/17; COM-04 | **I/D/S:** unknown/hidden/shared/isolated/reused UID, package lifecycle, доступный app chooser; visibility не превращать в полное знание списка. |
| 11. LAN permission | **N:** при target≥37 прямой/broad LAN требует ACCESS_LOCAL_NETWORK; system Cast/NSD pickers допускают scoped selected-device access без broad permission. Target≤36 docs велят её не добавлять/не запрашивать. Picker exception не доказывает VPN-relayed LAN. [AR10] | **Reuse** target36 branch; **adapt** scoped subnet/overlap policy; **defer** broad LAN scan. | AND-06/10/14; A-FR-05/09; COM-05 | **I/D:** selected-network LAN/mDNS/casting/overlap, own и relayed sockets, denial/revoke при37 migration; capability matrix отдельно от INTERNET. |
| 12. Lockdown/capture scope | **P:** OS always-on+block-without-VPN — барьер после app death; app reject сам по себе им не является. Per-profile scope/OS exceptions сохраняются. Builder.allowBypass разрешает обход. [AR05][AR12] | **Reuse** strict full capture; **defer** bypass. | AND-03/06/15; A-FR-02/14; COM-03/04 | **D:** внешний capture для crash/revoke/existing TCP/one-way UDP/patch-specific system traffic; не абсолютный «весь телефон». |
| 13. Split semantics | **P:** Builder allowed/disallowed lists взаимоисключены и меняются через новую connection. Исключённые apps при lockdown теряют сеть; native route действует только после OS capture. [AR05] | **Reuse** policy IR/explain; **adapt** два уровня с controlled replace; direct остаётся deferred. | AND-06/09/10; A-FR-05/08/09; COM-04/06/08 | **I/D/U:** effective route+DNS+scope, shared UID/updates/work profile и existing flows; action показывает reconnect и реальный lockdown conflict. |
| 14. DNS/IPv6/Private DNS | **P/N:** protected DNS, bootstrap и exclusions — разные scopes. Private DNS нельзя молча заменить plaintext; app DoH/shared IP не даёт полной domain attribution. allowFamily может открыть fall-through. [AR12][AR13] | **Reuse** native resolver и общий route/DNS AST; **adapt** явный protected graph; **defer** system DNS/direct до приёмки. | AND-04/06/07; A-FR-03/05/06; COM-04/06/07 | **I/D:** paired DNS/data first-match, cache namespace, fakeIP/DoH/NAT64/IPv6-only/PTB; unknown metadata не later/default direct. |
| 15. Handover/QUIC | **P:** THEORY14 уже различает supported lower path API и недоступный public HY2 seam. bindSocket требует unconnected socket; текущий Network lease/restart не seamless migration. [AR11][AR42] | **Reuse** leases/native path manager при отдельном seam; **defer** migration preset. | AND-02/05/07/11/15; A-FR-04/06/10; COM-03/17 | **I/D:** protect+bind до connect, applied-path receipt, CC/PMTU reset, no-event/delayed callbacks, continuity/gap/leak/energy. |
| 16. Strict owner overlay | **P:** native scoped guard до route/DNS/outbound, fresh selected identity; marker/API не реализация. Shared gate32/PM after-return threshold не throughput/SLO и не per-packet reattestation. [AR33][AR42] | **Reuse** overlay/targeted lookup/death refresh; **defer** activation blockers removal. | AND-06/07/11; A-FR-01/05/10 | **B/I/D:** actual patched AAR graph parity, TCP/UDP/PreMatch/DNS/PTB, existing flows и callback saturation/Stop latency. |
| 17. Offline import/QR/deep link | **P/N:** SAF/user-selected image grants narrow access; URI/MIME/filename не authority. External deep link требует validation; нельзя auto-Apply/connect из Intent. [AR14][AR15][AR38] | **Reuse** bounded SelectedProfileReader/coordinator/ZXing; **adapt** one preview и exact bytes. | AND-04/08; A-FR-03/07/17; COM-01/13 | **I/U/D:** malformed/oversize pixels/unknown-length/provider denial/late close, password and redirects; no logs/recents token; Cancel leaves no partial acceptance. |
| 18. Portable verifier/trust | **P:** exact signed bytes/full TUF workflow уже выбраны. go-tuf2.4.2 updater и gomobile package-list seam пригодны как design; Windows module/новый Kotlin canonicalizer не нужен. [AR32][AR34][AR35] | **Reuse** portable schema/Go canonical/trust code через узкий seam, один artifact. | AND-04/13/14; A-FR-03/12/13; COM-13/14 | **I/B:** public seam + cross-platform golden bytes, authority/roles/epoch/revocations/history/clock fence, accepted-only producer; SDK implementation отсутствует. |
| 19. Clock/expiry/recovery | **P:** elapsedRealtime включает сон, но не доверенный UTC; reboot сбрасывает scope. TUF freeze/expiry и local floors нельзя снять отсутствием delivery. [AR18][AR32][AR42] | **Reuse** ProfileClock/CAS/projected deadline; **adapt** recovery resolver. | AND-04/10/13; A-FR-03/09/12; COM-08/13 | **I/D:** wall jumps/suspend/reboot/old blob/pending commit, no floor reset и no renewed temporary grant. |
| 20. Keystore/vault | **P/N:** key material non-exportable; hardware backing проверяется KeyInfo, не обещается. Auth-per-use мешает unattended recovery. AEAD integrity сама не доказывает antirollback старого valid blob — вывод из модели. [AR16] | **Reuse** CE/no-backup AES-GCM/app-domain AAD; **defer** secret storage в DE. | AND-04/13; A-FR-03/12; COM-13 | **I/D:** missing/invalidated key, restore same alias, crash/fsync/CAS; hardware security level recorded, no plaintext/reset fallback. |
| 21. Backup/transfer | **N:** noBackupFilesDir исключён из Auto Backup. allowBackup=false на части target31+ OEM не отключает D2D; cloud и device transfer управляются отдельными rules. [AR17] | **Reuse** no-backup store; **adapt** explicit encrypted transfer/re-enrollment и final-app extraction rules. | AND-04/13/14; A-FR-03/12/13; COM-13 | **I/D:** cloud/D2D/restore/uninstall/key-loss; ciphertext без key не Empty; old trust history не «чистая новая установка». |
| 22. Per-device credentials | **N:** HY2 users/password и VLESS users/uuid поддерживают отдельные server identities; соответствующие поля есть и в exact1.13.19 options. Это не automatic device management или немедленное закрытие уже authenticated flows. [AR36][AR37][AR43][AR44] | **Reuse** серверный users механизм и offline issuer; **adapt** enrollment/revoke mapping. | AND-13; A-FR-12; COM-13/15 | **I/O/D:** actual issuer/gateway config и выдача unique credential, revoke active/new flows, offline history; admin keys остаются вне APK. |
| 23. Energy/scheduling | **N:** Doze откладывает network/jobs, WorkManager не VPN keepalive. ODPM device-level, Pixel6+; unsupported device может давать gauge/coulomb counters, не тот же сигнал. [AR19][AR20] | **Reuse** event-driven monitor/optional scheduling; **adapt** separate protected/optional budgets. | AND-07/11; A-FR-06/10; COM-09/12 | **D:** matched idle/screen-off/handover/call/bulk, bytes/wakeups/CPU/JVM+Go RSS/fd, signal/thermal context; thresholds preregister before comparison. |
| 24. UI architecture/navigation | **P/N:** Compose UDF/state hoisting и stable adaptive scaffolds доступны. Четыре destinations — продуктовая гипотеза, не доказанная usability победа. [AR21][AR22]AR40 — `docs/evidence/2026-10-21-android-research.md` | **Reuse** NIA patterns/NavigationSuiteScaffold/list-detail; **adapt** status/scope/freshness views. | AND-08/09/12; A-FR-07/08/11; COM-01/11 | **I/U:** flow prototype→backend state, back/rotate/fold and stale data; no News graph или duplicate native owner. |
| 25. Accessibility/usability | **N:** Android рекомендует48dp touch targets; semantic state, scaling и adaptive layout поддержаны стандартными компонентами. UX metrics нельзя получить чтением docs. [AR23] | **Reuse** Material semantics; **adapt** Russian copy/light-dark/system/fonts. | AND-08/12/15; A-FR-07/11/14 | **U/D:** заранее task-success/time/error/help/comprehension threshold; consent/bank/blocked/recovery, TalkBack/font200%/keyboard/one hand. |
| 26. Tile/notification/widget | **N:** TileService имеет unlockAndRun; API34+ launch uses PendingIntent. Поверхности отправляют intent одному repository, не создают независимое ядро. [AR24] | **Reuse** state/controller; **adapt** tile/notification/Glance or platform widgets. | AND-12; A-FR-11; COM-02 | **I/D/U:** stale tile, killed UI, lockscreen; новые trust/grants/secrets требуют unlock, ранее accepted connect отдельно разрешён политикой. |
| 27. Grants/portal/quiet mode | **P:** lockdown conflict нельзя исправить общим pause/allowBypass. Quiet — явный пользовательский intent, не слежение за mic/screen. TTL/network/profile/reboot revoke определены. [AR05][AR12][AR42] | **Adapt** scoped grant/handoff/quiet; **defer** direct grant пока policy не admitted. | AND-09/10/11; A-FR-08/09/10; COM-08/09/10 | **I/D/U:** portal evidence≠timeout, expiry/revoke/late callbacks; safety выше quiet, restore grant не продлевает доступ. |
| 28. Diagnostics/privacy | **P:** одна HTTPS/VALIDATED/handshake не общий path proof. Нужны typed stale/no-data/endpoint-dead scopes; app inventory чувствителен. [AR08][AR27][AR42] | **Reuse** common observations/bounded lanes; **adapt** local redacted export preview. | AND-07/08/09/14; A-FR-06/08/17; COM-07/11/15 | **I/U/D:** endpoint outage no false quarantine, counts/failures, no URL-token/app-list/traffic upload, user-controlled share. |
| 29. Managed/work profile | **N:** DPM always-on настройка требует device/profile owner и применяется current user. Managed configurations — sanctioned admin channel, не второй VPN в том же profile. [AR30][AR31] | **Reuse** normal app/RestrictionsManager; **defer** enterprise offering до явного use case. | AND-03/06/09/18; A-FR-02/05/18; COM-06/16 | **I/O/D:** managed install/admin authority/current-user boundary/conflicts; personal-profile PASS не work-profile PASS. |
| 30. APK/update signing | **N:** app signing identity/lineage обеспечивает update continuity; upload key, APK signer и profile/TUF authority разные роли. Обычный installer обязан принимать user-action outcomes. [AR25][AR26] | **Reuse** Android signing/PackageInstaller и signed payload; **adapt** staged update/known-good recovery. | AND-14/17; A-FR-13/16; COM-14 | **B/D/O:** signer custody/lineage/versionCode, wrong-key/partial/low-disk/denied; no uninstall workaround, no trusted downgrade, readiness после update отдельно. |
| 31. Play acceptance | **N:** core VPN допустим, но требуются declaration, encrypted device→endpoint, listing/disclosure/consent и policy review; FGS declarations отдельные. [AR02][AR06][AR27] | **Adapt** store flavor/disclosures только при выбранном канале; **defer** submission. | AND-14/17; A-FR-13/16/17 | **S/O:** account/country/policy forms/privacy/Data Safety/demo actual VPN flow/approval; техническая документация не одобрение Play. |
| 32. Direct APK/F-Droid | **N:** signed direct APK — исходный channel candidate. F-Droid требует FLOSS/source build/допустимый toolchain и dependencies; собственный opaque prebuilt AAR не готовый main-repo input. [AR25][AR28] | **Reuse** no-GMS stack; **adapt** source-build recipe; **defer** F-Droid publication до выбора владельца. | AND-01/14/17; A-FR-13/16; COM-14 | **O/S/B:** канал/key custody/source transparency и review. Это осознанный deferred channel, не необследованный engine. |
| 33. Developer verification | **N:** текущая Android page указывает с30.09.2026 participating stores/четыре страны на certified devices и дальнейшее расширение2027. Это отдельная developer registration, не VPN content approval. [AR29] | **Adapt** release checklist под actual country/channel; **defer** регистрацию без выбранного выпуска. | AND-14/17; A-FR-13/16 | **S/O:** live console/account eligibility, app/package/signer registration и действующий региональный scope перед release. Не экстраполировать roadmap на все устройства/каналы сегодня. |
| 34. Device/field support/пороги | **P:** нет реально подтверждённого current handset inventory, native/device/field результатов или comparative thresholds. Desk research не может создать эти факты. | **Reuse** §6 matrix; **adapt** staged lab→device→external capture→paid field. | AND-15/16/17; A-FR-14/15/16 | **O/D/U:** владелец выбирает доступные OS/OEM/patch/network и preregistered SLO/workload; unknown cells сохраняются unknown, raw evidence связывается с exact inputs. |
| 35. Другие transports/platforms/privacy/bonding | **P:** использование одного whole libbox остаётся выбранным; MPTCP/bonding/tethering/TV/router/PQ/multi-hop/port-forward не появляются из Android FGS. | **Defer** AND-18 по отдельному use case, обоим концам/license/resource benefit. | AND-18; A-FR-18; COM-16…19 | **O/I/B/D:** named capability decision + server support + safety/resource comparison после AND-17. Не требуется случайный дополнительный engine для research closure. |


[AR01]: https://developer.android.com/build/releases/agp-8-13-0-release-notes
[AR02]: https://support.google.com/googleplay/android-developer/answer/11926878?hl=en
[AR03]: https://developer.android.com/reference/android/net/ConnectivityManager#getConnectionOwnerUid(int,%20java.net.InetSocketAddress,%20java.net.InetSocketAddress)
[AR04]: https://developer.android.com/guide/practices/page-sizes
[AR05]: https://developer.android.com/develop/connectivity/vpn
[AR06]: https://developer.android.com/develop/background-work/services/fgs/service-types#system-exempted
[AR07]: https://developer.android.com/develop/ui/compose/notifications/notification-permission
[AR08]: https://developer.android.com/training/package-visibility
[AR09]: https://support.google.com/googleplay/android-developer/answer/10158779?hl=en
[AR10]: https://developer.android.com/privacy-and-security/local-network-permission
[AR11]: https://developer.android.com/reference/android/net/Network#bindSocket(java.io.FileDescriptor)
[AR12]: https://developer.android.com/reference/android/net/VpnService.Builder
[AR13]: https://developer.android.com/reference/android/net/LinkProperties
[AR14]: https://developer.android.com/training/data-storage/shared/documents-files
[AR15]: https://developer.android.com/training/data-storage/shared/photo-picker
[AR16]: https://developer.android.com/privacy-and-security/keystore
[AR17]: https://developer.android.com/identity/data/autobackup
[AR18]: https://developer.android.com/privacy-and-security/direct-boot
[AR19]: https://developer.android.com/training/monitoring-device-state/doze-standby
[AR20]: https://developer.android.com/studio/profile/power-profiler
[AR21]: https://developer.android.com/develop/ui/compose/architecture
[AR22]: https://android-developers.googleblog.com/2024/09/jetpack-compose-apis-for-building-adaptive-layouts-material-guidance-now-stable.html
[AR23]: https://developer.android.com/guide/topics/ui/accessibility/apps
[AR24]: https://developer.android.com/reference/android/service/quicksettings/TileService
[AR25]: https://developer.android.com/studio/publish/app-signing
[AR26]: https://developer.android.com/reference/android/content/pm/PackageInstaller.SessionParams#setRequireUserAction(int)
[AR27]: https://support.google.com/googleplay/android-developer/answer/12564964?hl=en
[AR28]: https://f-droid.org/docs/Inclusion_Policy/
[AR29]: https://developer.android.com/developer-verification
[AR30]: https://developer.android.com/reference/android/app/admin/DevicePolicyManager#setAlwaysOnVpnPackage(android.content.ComponentName,%20java.lang.String,%20boolean)
[AR31]: https://developer.android.com/work/managed-configurations
[AR32]: https://theupdateframework.github.io/specification/latest/

[AR34]: https://github.com/SagerNet/gomobile/blob/v0.1.12/cmd/gomobile/bind.go
[AR35]: https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go
[AR36]: https://sing-box.sagernet.org/configuration/inbound/hysteria2/#users
[AR37]: https://sing-box.sagernet.org/configuration/inbound/vless/#usersuuid
[AR38]: https://developer.android.com/privacy-and-security/risks/unsafe-use-of-deeplinks
[AR39]: https://developer.android.com/about/versions/15/behavior-changes-all#stopped-state


[AR42]: ../performance/TRANSPORT-AND-RESOURCES.md#reliability-depth
[AR43]: https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/option/hysteria2.go
[AR44]: https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/option/vless.go


<!-- /source-unit: 15.5 -->
