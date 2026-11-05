<a id="конкурентный-анализ-snowdensystem--фичи-обязательные-для-лучшего-сервиса"></a>
<a id="1-сводная-матрица"></a>
<a id="2-классы-конкурентов--что-берём-что-нет"></a>
<a id="21-amneziavpn--эталон-self-hosted-ux"></a>
<a id="22-hiddify--эталон-мультипротокольного-клиента"></a>
<a id="23-outline--эталон-простоты-раздачи-доступа"></a>
<a id="24-psiphon--эталон-устойчивости-рассылки-доступа"></a>
<a id="25-v2raynnekorayclash-verge-rev--эталон-контроля"></a>
<a id="26-protonmullvad--эталон-trust--privacy"></a>
<a id="3-итог-минимальный-набор-для-лучшего-в-классе"></a>
<a id="4-отдельные-требования-рынка-к-сервису"></a>
<a id="5-обновление-рынка-v2-047-2026-09-14-что-изменилось-с-v2-035"></a>
<a id="51-что-нового-у-конкурентов-за-полгода"></a>
<a id="52-новый-класс-локальные-обходчики-без-vps-zapretbyedpi"></a>
<a id="53-что-изменилось-в-тспу-ландшафте-перекликается-с-theory-24a"></a>
<a id="54-обновлённый-вывод-где-мы-сильнее-где-слабее"></a>
<a id="55-решения-по-бэклогу-дельта-к-3"></a>
<a id="6-исследование-бытовых-и-рабочих-сценариев--2026-10-21"></a>
<a id="61-метод-и-границы"></a>
<a id="62-проверенные-ожидания-рынка"></a>
<a id="63-что-уже-есть-у-snowden--source-inspection"></a>
<a id="64-каталог-пригодных-реализаций"></a>
# Повседневный VPN: сценарии, интерфейс и готовые практики

[Оглавление](../README.md) · [Термины](../GLOSSARY.md) · [Android UX](../android/PLATFORM-AND-UX.md) · [План ПК](../../planning/knowledge-application/PC-PLAN.md) · [План Android](../../planning/knowledge-application/ANDROID-PLAN.md)

## Краткий вывод

Удобство дают предсказуемые состояния, понятная настройка исключений,
безопасное восстановление и минимум действий в частых сценариях.
Число протоколов и длинный список функций этого не обеспечивают.
В первую очередь полезно развивать уже существующие механизмы проекта;
чужие клиенты дают отдельные идеи и реализации, но редко подходят целиком.

**Когда применять.** При проектировании первого подключения, работы из дома
и офиса, поездок, исключений для приложений, диагностики и обновления.

**Граница знания.** Сравнение основано на сохранённом разборе от 21 октября 2026 года
и отдельных уточнениях 24 октября. Это целевая выборка документации и исходников,
не перепись рынка и не текущая проверка всех платформ. Ни уникальность Snowden,
ни превосходство над другими VPN не установлены. Новые рыночные сведения
для этой редакции не собирались.


## Почему старая сравнительная оценка отозвана

Сентябрьские разделы 1–5 называли отдельные решения «уникальными», приписывали
конкурентам только проверку сокета и выдавали наличие кода за подтверждённую
защиту. Эти утверждения были отозваны поправкой COM-00 от 21 октября.
Старая матрица также утверждала отсутствие уже появившихся трея, переноса
профиля и обновления. Она не используется для текущего списка возможностей.

Это **исправленные ошибки и устаревшее состояние**, а не спор двух равных
актуальных оценок. Историческая редакция доступна в
[зафиксированной версии репозитория](https://github.com/cj-isus/snowden-system-v3/blob/0484a18261ffa9fe92365246bf956adc8983270e/docs/theory/ux/COMPETITOR-FEATURES.md).
Необоснованные ранги протоколов и утверждения о 15 КБ, XHTTP/splithttp,
RU-релее либо превосходстве диагностики оттуда не становятся рекомендациями.
Механизмы и пороги рассматриваются в
[транспортной главе](../transports/CENSORSHIP-MECHANISMS.md) и
[методе сопоставления](../methodology/RESEARCH-METHOD.md#measurement-comparability-2026-10-27).

<a id="comfort-research"></a>
<a id="6-исследование-бытовых-и-рабочих-сценариев--2026-10-21"></a>
## 6. Сценарии и основания решений

<a id="61-метод-и-границы"></a>
### 6.1. Метод и границы

В исходном разборе зафиксированы 13 upstream HEAD и дополнительно уже выбранный
SagerNet/tailscale. Сохранены 49 файлов, в том числе 14 лицензий,
манифесты и выбранные реализации. Прочитаны относящиеся к решениям участки,
а не весь код репозиториев; отдельно сопоставлялся sing-box v1.13.19.
Чужой код не исполнялся. HEAD обозначает конкретный снимок, не автоматически
выбранный стабильный выпуск. Источники и смысл рекомендаций сохранены ниже;
подробный ход работы агента не нужен для применения знания.

Приоритет определяется сценарием и проверяемым эффектом:

| Сценарий | Полезное поведение | Что требуется сохранить | Этапы двух планов |
|---|---|---|---|
| Первое подключение и новое устройство | Понятный импорт/QR, предварительная сводка изменений и восстановление после ошибки | Доверие, размер/формат, отзыв устройства; общий секрет не становится индивидуальным от копирования | PC-02/04; ANDR-01/06 |
| Ежедневное управление | Одно понятное действие подключения; трей/уведомление отражают применённое состояние | Различать желание пользователя, процесс и подтверждённый путь; доступность клавиатурой | PC-04; ANDR-06 |
| Старт ОС, сон и новая сеть | Возобновление без ручной чистки настроек | Владелец операции, новая сетевая эпоха, отсутствие запрещённого прямого выхода | PC-01/03; ANDR-03/04 |
| Работа с LAN, принтером и корпоративной сетью | Явные согласованные исключения, объяснение фактически применённого правила | Маршрут, DNS, firewall и доверие согласованы; сосуществование с чужим VPN не гарантировано | PC-05; ANDR-05 |
| Исключение приложения или сервиса | Понятные режимы и приоритет правил, предварительный просмотр действия | Идентичность процесса/UID, CNAME/IPv6/TTL, shared IP; неизвестное покрытие не превращается в полный обход | PC-05; ANDR-05 |
| Гостиница или captive portal | Ограниченная диагностика и отдельное разрешение входа в сеть | Редирект не даёт полномочий; исключение не распространяется на защищаемые приложения | PC-05; ANDR-05/06 |
| Звонок, презентация, загрузка | Временный запрет необязательных вмешательств с явным сроком | Обязательные защита, отзыв и восстановление продолжаются; это не бесшовный multipath | PC-07; ANDR-04/08 |
| Сбой отдельного сайта | Объяснение этапа отказа и обратимое предложение | Одна проба не доказывает login/media/P2P либо цензуру; нет скрытого ослабления защиты | PC-03/04; ANDR-03/06 |
| Ограниченный тариф и батарея | Пределы фоновых проб, повторов и необязательной активности | Обязательные сроки и полномочия сохраняются; экономию измеряют с полезной работой | PC-06/07; ANDR-07/08 |
| Обновление и восстановление | Проверяемая доставка, безопасная замена и понятная ошибка | Подпись не устанавливает файл и не заменяет crash/rollback-контракт | PC-08; ANDR-09 |
| Обращение за помощью | Предварительный просмотр очищенной диагностики | Секреты, полные назначения и пользовательские данные не попадают в экспорт автоматически | PC-04; ANDR-06 |

COM-00…19 остаются координатами прежнего реестра, не новым параллельным
backlog. Конкретный статус и очерёдность находятся в двух планах и реестрах
владельцев. Режим без туннеля, реклама/трекеры, multihop, DAITA и bonding
не назначаются обязательными только из-за наличия у аналога.

<a id="62-проверенные-ожидания-рынка"></a>
<a id="62-документированные-примеры-на21октября2026года"></a>
### 6.2. Документированные примеры на 21 октября 2026 года

В таблице подтверждается содержание просмотренной документации. Она не
доказывает удобство, безошибочность реализации, текущий тариф или поддержку
каждого устройства. Связь с COM обозначает применение идеи, а не факт интеграции.

| Первичный источник | Что было указано в просмотренной документации | Связь с планом |
|---|---|---|
| [Proton Profiles](https://protonvpn.com/support/connection-profiles) | Сохранение location/protocol/settings; Connect and Go открывает приложение/сайт после подключения; в профилях также есть NAT/port-forwarding настройки. Наличие зависит от плана/платформы | COM-01/04/19; пользовательский запуск после нашего свежего проверки защищённого пути |
| [Windscribe Split Tunneling](https://windscribe.com/features/split-tunneling) | Inclusive/exclusive app/hostname/IP policies; отдельно заявлено, что DNS остаётся системным ROBERT/custom DNS даже для исключений | COM-04/06: нельзя объявить split DNS автоматически решённым app bypass |
| [Amnezia Split Tunneling](https://docs.amnezia.org/documentation/instructions/vpn-split-tunneling/) | Разные возможности ОС; Windows app-exclusion имеет ограничения Store/anti-cheat. Документация предупреждает о разовом IPv4 resolution доменов и сложных зависимостях сервисов | COM-04/16: dynamic service rules, явная platform matrix; не обещать «любой exe/домен» |
| [IVPN firewall](https://www.ivpn.net/knowledgebase/general/do-you-offer-a-kill-switch-or-vpn-firewall/) и [exceptions](https://www.ivpn.net/knowledgebase/general/firewall-exceptions/) | Always-on/boot firewall, LAN/subnet exceptions, сценарий одновременного корпоративного VPN; исключение firewall и маршрут — разные настройки | COM-03/05/06: service ownership + согласованная policy; сосуществование только после отдельной проверки |
| [Mullvad desktop](https://mullvad.net/en/help/using-mullvad-vpn-app) | Auto-connect, LAN, lockdown, split, DNS blockers; DAITA и multihop имеют цену в traffic/battery/latency | COM-02/03/05/18: редкие функции — отдельный opt-in, не являются измеренным ускорением |
| [Tailscale app connectors](https://tailscale.com/docs/reference/best-practices/app-connectors) | DNS-derived A/AAAA/CNAME routes, invisible split DNS, approval/ACL и зависимости private network | COM-04/06/16: алгоритмический референс; импортировать control-plane SaaS в клиент не нужно |
| [Speedify bonding vs local balancing](https://support.speedify.com/article/892-can-i-use-speedify-without-vpn-server), [Enhance Streaming](https://support.speedify.com/article/692-enhance-streaming-mode) | Bonding использует общий server endpoint; local balancing распределяет sockets и не сохраняет упавший поток. Приоритизация/дублирование media расходуют ресурсы | COM-09/12/17: не путать отсрочку вмешательств, failover и бесшовное продолжение потока |
| [TUF overview](https://theupdateframework.io/docs/overview/) | Защита обнаружения/скачивания targets от rollback/freeze и компрометации ключей через metadata roles | COM-14: TUF не устанавливает EXE, не исправляет Windows service/crash rollback |


Consumer VPN, mesh/private access и маршрутизаторное объединение каналов
решают разные задачи. «Редкое» означает отобранный пример без измеренной
частоты на рынке. Ценность сочетания функций остаётся гипотезой до проверки
сценариев, а не установленной новизной.

<a id="63-что-уже-есть-у-snowden--source-inspection"></a>
### 6.3. Датированные точки переиспользования в Snowden

Статический разбор 21 октября указал следующие места. Они позволяют начать
с существующего кода, но не описывают весь нынешний состав приложения.

| Механизм | Координаты исходного разбора | Граница применения |
|---|---|---|
| Трей и автозапуск | `windows/tray_windows.go`, systray1.2.2, HKCU Run/`--auto-connect` | Меню должно отражать применённую политику; native/boot UX проверяется отдельно |
| Перенос и QR | `windows/onboarding.go`, `backend/onboarding/`, `onboarding_import.go` | Seal/Open и staged import не доказывают отзыв отдельного устройства или пригодность большого QR |
| Process split | `backend/render/render.go`, подписанный `split_direct` | В том срезе RouteRule имел Protocol/Action/Outbound/IPCidr/DomainSuffix/ProcessName; весь upstream API этим не поддерживался |
| Смена сети | `windows/netwatch.go` | Исторический пропуск итерации при tunRequested требовал отдельного контракта физических интерфейсов; не объявляется нынешним дефектом без актуальной сверки |
| Диагностические наблюдения | `backend/core/probe_http_observation.go`, `diagnostic_budget.go`, `diagprobe.go` | Расширять существующие этапы и бюджеты; второй независимый HTTP-prober не обоснован |
| Обновление | `backend/update/update.go`, `windows/updateapp.go` | Наличие manifest/signature/hash/version floor не закрывает долговечность, сервис и восстановление |
| Защита и зависимости | `wfp_killswitch_windows.go`, `backend/render/dependency_graph.go` | Наличие файлов не является доказательством native-безопасности |

Отсутствие корневой лицензии в прочитанном тогда наборе не устанавливает
состояние лицензирования всего нынешнего репозитория. Условия встроенного
sing-box и заимствуемых файлов всё равно учитываются до распространения.

<a id="64-каталог-пригодных-реализаций"></a>
### 6.4. Каталог реализаций и цена заимствования

Классы решений: **reuse existing** — развить уже выбранное; **adapt** —
адаптировать ограниченную реализацию; **reference** — использовать подход;
**candidate** — сначала оценить совместимость; **defer** — отложить.
Лицензии ниже относятся к просмотренным снимкам, не являются заключением
о совместимости всего будущего продукта. Перед копированием нужны notices,
точный состав зависимостей и условия распространения.

| Проект / фиксированный снимок и источник | Лицензия просмотренного кода | Решение и конкретное заимствование |
|---|---|---|
| SagerNet/sing-box `dfb80349bf5779ec0a3f834c9ec2794345bff8f0`; наш [v1.13.19 option/rule.go](https://github.com/SagerNet/sing-box/blob/v1.13.19/option/rule.go), [option/dns.go](https://github.com/SagerNet/sing-box/blob/v1.13.19/option/dns.go) | [GPL-3.0-or-later](https://github.com/SagerNet/sing-box/blob/dfb80349bf5779ec0a3f834c9ec2794345bff8f0/LICENSE) | **Reuse existing**, COM-04/05/06/18: process/path/domain/IP/rule-set/DNS primitives поверх pinned ядра. Дописать нашу строгую schema, signed contract и компилятор правил; наличие опции не гарантирует платформенное поведение. Не обновлять ядро ради поля из HEAD |
| SagerNet/tailscale **v1.92.4-sing-box-1.13-mod.9**; [netmon Windows](https://github.com/SagerNet/tailscale/blob/v1.92.4-sing-box-1.13-mod.9/net/netmon/netmon_windows.go), [go.mod](https://github.com/SagerNet/tailscale/blob/v1.92.4-sing-box-1.13-mod.9/go.mod) | BSD-3-Clause; Go1.24.1 в manifest | **Первый adapt-кандидат**, COM-03. Уже indirect dependency, не новый SDK: Windows route/address callbacks, unregister, ограниченная доставка в нашу эпоху. Сначала проверить экспорт/API/зависимости; наличие в go.mod не доказывает готовую интеграцию. 2026-10-24 tag разрешён в неизменяемый commit867c1f0012d915fa5fd57481dbf2a1ad16669fe7;3 exact files/Git blobs проверены, THEORY§15.4. интеграция с ОС пока не принята |
| Tailscale `47743a8b9ad16cf3a49504dc443208c37748072e`; [netmon](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/netmon/netmon_windows.go), [captive detection](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/captivedetection/captivedetection.go), [DNS manager](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/dns/manager.go) | [BSD-3-Clause](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/LICENSE); снимок с требованием Go1.27.3 | **Adapt/reference**, COM-03/06/10. Callback monitor, короткая привязанная к интерфейсу HTTP-проба без редиректов с проверкой 204 и challenge, компилятор разделённых DNS-правил. В detection есть netmon/DERP-specific dependencies: не переносить DERP адреса/бесконтрольный direct. Не импортировать весь свежий tailscale.com в Go1.26 и не считать detect=false доказательством отсутствия portal |
| tailscale/wf `6fbb0a674ee67f6c9b99eb7ce8a4330bb0145ec9`; [firewall.go](https://github.com/tailscale/wf/blob/6fbb0a674ee67f6c9b99eb7ce8a4330bb0145ec9/firewall.go) | [BSD-3-Clause](https://github.com/tailscale/wf/blob/6fbb0a674ee67f6c9b99eb7ce8a4330bb0145ec9/LICENSE) | **Reuse existing**, COM-04/05/08: этот exact commit уже direct dependency. Расширять наш WFP owner/transaction; не заводить второй engine session с чужими provider GUID |
| WireGuard Windows `6ece77bc487c8aa697e3c092197621c4f3e5ccb8`; [blocker.go](https://github.com/WireGuard/wireguard-windows/blob/6ece77bc487c8aa697e3c092197621c4f3e5ccb8/tunnel/firewall/blocker.go), [rules.go](https://github.com/WireGuard/wireguard-windows/blob/6ece77bc487c8aa697e3c092197621c4f3e5ccb8/tunnel/firewall/rules.go); официальный upstream git.zx2c4.com, GitHub mirror | [MIT](https://github.com/WireGuard/wireguard-windows/blob/6ece77bc487c8aa697e3c092197621c4f3e5ccb8/COPYING) | **Adapt helpers/reference**, COM-03/05: permit tunnel LUID, DNS restriction, DHCPv4/v6/NDP, weighted block and transaction. **Не брать blocker целиком:** FWPM_SESSION_FLAG_DYNAMIC не соответствует нашему persistent crash barrier. wireguard/windows v0.5.3 уже indirect, новый HEAD не выбран |
| IVPN desktop `1fd063064f466322c3749dc758f7e72dfbae4b99`; [Windows firewall](https://github.com/ivpn/desktop-app/blob/1fd063064f466322c3749dc758f7e72dfbae4b99/daemon/service/firewall/firewall_windows.go), [autoconnect CLI](https://github.com/ivpn/desktop-app/blob/1fd063064f466322c3749dc758f7e72dfbae4b99/cli/commands/autoconnect.go) | [GPL-3.0](https://github.com/ivpn/desktop-app/blob/1fd063064f466322c3749dc758f7e72dfbae4b99/LICENSE.md); уведомления отдельных файлов перед копированием | **Reference/adapt после проверки лицензии**, COM-03/05/06/08: persistent firewall, LAN/multicast/exceptions и settings semantics. Платформенные firewall/driver/UI не переносить целиком в Wails; нельзя подменять наш lifetime/очистку с установленным владельцем |
| Mullvad `52d4ab90aff4eb7d9455bfad071d144220b3a054`; [error_state.rs](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/talpid-core/src/tunnel_state_machine/error_state.rs), [persistent.rs](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/talpid-core/src/firewall/windows/objects/persistent.rs) | [GPL-3.0](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/LICENSE.md) | **Reference**, COM-03/05/08/18: blocked/error state применяет firewall policy с ограниченным endpoint/LAN, lifecycle отдельно от UI. Rust/FFI/driver stack — дорогое несоответствие; не добавлять вторую state machine. DAITA не код для произвольного transport |
| Proton Windows `d2a4f8bc92a0fd296943a7cdd15f4f870c8a87f9`; [SplitTunnel.cs](https://github.com/ProtonVPN/win-app/blob/d2a4f8bc92a0fd296943a7cdd15f4f870c8a87f9/src/ProtonVPN.Service/SplitTunneling/SplitTunnel.cs) и Profiles docs выше | [GPL-3.0](https://github.com/ProtonVPN/win-app/blob/d2a4f8bc92a0fd296943a7cdd15f4f870c8a87f9/LICENSE) | **UX/reference**, COM-01/04/19: сценарные настройки/Connect and Go, service-side split lifecycle. C#/service/native stack не Go готовая замена; реализация Profiles в коде отдельно не разобрана — её подтверждает только product doc |
| Amnezia `02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc`; [Windows split](https://github.com/amnezia-vpn/amnezia-client/blob/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc/client/platforms/windows/daemon/windowssplittunnel.cpp), [controller](https://github.com/amnezia-vpn/amnezia-client/blob/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc/client/core/controllers/appSplitTunnelingController.cpp) | [GPL-3.0](https://github.com/amnezia-vpn/amnezia-client/blob/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc/LICENSE) | **Reference**, COM-01/04/13/16: список приложений, split settings и понятная платформа. Windows code устанавливает/запускает driver/service: нельзя копировать как простой process_name. Qt/C++ UI не переносить; шифрование onboarding уже своё |
| go-tuf/v2 `45ad19f5a821abbcdbd1ba10b994362d12d39965`; [Updater](https://github.com/theupdateframework/go-tuf/blob/45ad19f5a821abbcdbd1ba10b994362d12d39965/metadata/updater/updater.go) | [Apache-2.0](https://github.com/theupdateframework/go-tuf/blob/45ad19f5a821abbcdbd1ba10b994362d12d39965/LICENSE); снимок с требованием Go1.27 | **Reuse совместимый v2.4.2/f5edbde31e5507f46db2069402dc38903fe6d9d4 по ADR-001; интеграция по исходникам здесь не подтверждена**, COM-14. Эта строка HEAD сохранена как исторический объект сравнения: trusted root→timestamp→snapshot→targets, target hash/length verification. Не переписывать криптографию; не включать UnsafeLocalMode для обхода recovery/expiry. Установка EXE, antirollback/atomic local state и Windows service остаются нашими |
| blackbox_exporter `d18e9bed33270911c09826234a4e4f500f775087`; [HTTP](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/prober/http.go), [DNS](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/prober/dns.go), [TCP](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/prober/tcp.go) | [Apache-2.0](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/LICENSE); снимок с требованием Go1.26.0 | **Reference или готовый внешний optional binary**, COM-07/11/15: staged httptrace/DNS/TLS/certificate metrics. В клиенте расширить существующие observations/budget; exporter снаружи не доказывает protected path и не должен быть public SSRF endpoint |
| getlantern/systray `22c167e80905f87867fc05fbe553342578976d7b`; [Windows implementation](https://github.com/getlantern/systray/blob/22c167e80905f87867fc05fbe553342578976d7b/systray_windows.go) | [Apache-2.0](https://github.com/getlantern/systray/blob/22c167e80905f87867fc05fbe553342578976d7b/LICENSE) | **Reuse existing v1.2.2**, COM-02. HEAD commit 2023-11-10: это не доказательство активной поддержки в 2026; сначала native compatibility. Не менять зависимость на другой tray framework без обнаруженного дефекта |
| OpenMPTCProuter `238f810e9e200cd4fac7303a6a1e12adfe85f099`; [README](https://github.com/Ysurac/openmptcprouter/blob/238f810e9e200cd4fac7303a6a1e12adfe85f099/README.md) | [GPL-3.0](https://github.com/Ysurac/openmptcprouter/blob/238f810e9e200cd4fac7303a6a1e12adfe85f099/LICENSE); packages имеют свои лицензии | **Готовое отдельное решение/candidate**, COM-17/16: router(OpenWrt)+VPS anchor, MPTCP/Glorytun/MLVPN. Можно оценить как внешнюю инфраструктуру вместо создания bonding с нуля. В этом срезе прочитан README, не scheduler/installer; Windows портирование и bypass не доказаны |
| Glorytun `32267e86a6da05b285bb3bf2b136c105dc0af4bb`; [README](https://github.com/angt/glorytun/blob/32267e86a6da05b285bb3bf2b136c105dc0af4bb/README.md) | [BSD-2-Clause](https://github.com/angt/glorytun/blob/32267e86a6da05b285bb3bf2b136c105dc0af4bb/LICENSE) | **Defer**, COM-17: multipath UDP/backup/shaping/MTU — полезные направления; просмотренный HEAD от 2020-10-30 и README просит stable branch. 2026-10-24 stable branch/latest0.3.4 разрешены в c113724eb0370ecd80d038192deeeb82a13ebed3 от 2020-03-25; stable README POSIX/Linux/libsodium. Интеграция ядра отклонена по цене/отсутствию обязательного use case. Полный native/криптографический аудит не выполнялся; reopen только по COM-17 scenario; не безопасный Windows готовая замена |


Порядок применения: существующие sing-box/wf/systray/onboarding/probe →
совместимая адаптация закреплённого netmon → собственные policy/DNS/UX
поверх имеющихся владельцев → согласованная версия TUF по действующему ADR.
Ни полный чужой клиент, ни новый глобальный владелец сети не выбраны.
Семантика ошибки, boot/crash и per-app enforcement важнее совпадения имени
«KillSwitch».

<a id="65-проверяемые-алгоритмы-адаптации"></a>
### 6.5. Практические алгоритмы и предел заимствования

1. **Событие сети:** callback ОС → короткая неблокирующая очередь → ограниченное
   объединение событий → новая эпоха → одна операция владельца восстановления.
   Физический интерфейс отличают от собственного TUN; подписка освобождается
   при завершении. Создание goroutine на каждый callback не заменяет пределы.
2. **Правила приложения/сервиса:** нормализация селекторов, приоритеты, DNS и
   явное действие при отказе → один проверенный применённый снимок.
   A/AAAA/CNAME/TTL не превращаются в вечный глобальный direct-список CDN-IP.
   Собственный DoH/ECH приложения и общий процесс браузера ограничивают покрытие.
   Полный разбор DNS — [в основной главе](../observability/DNS-AND-DIAGNOSTICS.md).
3. **Portal:** конечная проба с известным ответом и привязкой к физическому
   интерфейсу даёт наблюдение о возможном portal, не право отключить защиту.
   Открытие страницы требует отдельного ограниченного разрешения; произвольный
   URL из редиректа не назначает полномочия. Отрицательный результат пробы
   также не доказывает отсутствие portal.
4. **Восстановление:** ограничения защиты и разрешённые bootstrap-исключения
   согласованы до операций движка; новое состояние публикуется по контракту
   владельца. Идея IVPN/Mullvad адаптируется к существующей архитектуре.
5. **Важная сессия:** явный срок пользовательского режима ограничивает
   необязательные вмешательства, сохраняя обязательные. Bonding и бесшовное
   продолжение упавшего потока требуют отдельного протокола и серверной опоры.
6. **Обновление:** доверенные роли и target проверяются до staging;
   долговечность и замена файла проверяются отдельно. Подписанный сценарий
   не получает право выполнять произвольные импортированные команды.
   Основной разбор — [доставка и первое исполнение](../architecture/RELEASE-EVIDENCE-BOUNDARIES.md).

Совпадение идеи с upstream не доказывает отсутствие ошибок у него либо у нас.
Для каждого переноса остаются точная версия, лицензия, изменённый контракт,
платформенная совместимость и измеримая цена. Списки функций служат выбору
сценария, а не обязательству реализовать весь каталог.

<a id="68-итог-исследования"></a>
### 6.8. Применение и незакрытые условия

[Сводка от 24 октября](../roadmap/RESEARCH-HANDOFF.md#source-15) и
прежний реестр COM — `docs/PLAN.md#research-completion` сохраняют решения
исходного документального разбора. Они не закрывают реализацию.

Для ПК основные потребители — PC-04/05/08, для Android — ANDR-05/06/09;
защита и жизненный цикл предшествуют удобным автоматическим действиям.
Численная скорость, энергия, удобство native-интерфейса, сосуществование
с корпоративным VPN и сравнительное превосходство здесь **NOT_RUN**.
Текущие состояния берутся из [плана ПК](../../planning/knowledge-application/PC-PLAN.md)
и [плана Android](../../planning/knowledge-application/ANDROID-PLAN.md),
а не из значков старой конкурентной матрицы.
