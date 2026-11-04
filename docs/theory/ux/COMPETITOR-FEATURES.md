# Конкурентный анализ snowden.system — фичи, обязательные для «лучшего сервиса»

> **Актуальная исследовательская редакция: 2026-10-21, COM-00.** Разделы 1–5
> ниже сохранены как история сентября; их значки, сравнительные превосходства,
> сведения о наличии функций/серверов и транспортных API **не являются текущими
> выводами**. Утверждения «у всех только сокет», «уникально», «нет аналогов»,
> «сильнее рынка подтверждено» отозваны как недоказанные. Текущий source-based
> вывод — [§6](#comfort-research); требования/статусы — только [PLAN §5.4](../../PLAN.md#comfort-roadmap).

> Дата: 2026-09-12. Статус: research done (V2-035). Владелец задачи: план/документация.
> Назначение: множество фич конкурентов, которое обязан закрыть продукт, чтобы
> считаться лучшим в классе «антицензорный персональный VPN-клиент». Всё
> сверено с текущим состоянием PLAN.md/AGENTS.md; статусы покрытости — по
> журналам V2-001…V2-034, не по обещаниям.
>
> Метод: классы аналогов (AGENTS §2.3 «до кода изучаются зрелые аналоги»):
> 1. **антицензорные self-hosted клиенты** — AmneziaVPN, Hiddify, Outline;
> 2. **антицензорные сервис-клиенты** — Psiphon, Lantern (как класс);
> 3. **мультипротокольные ядра-обёртки** — v2rayN/Nekoray, Clash Verge Rev, NekoBox/Streisand (как класс);
> 4. **мейнстрим privacy-VPN** — Proton VPN, Mullvad, NordVPN, Surfshark;
> 5. **системные инструменты обхода** — NetGuard-класс (у нас уже есть свой).
>
> Правила честности из PLAN §4.1/UI-FEATURES сохраняются: фича не «закрыта»,
> пока нет evidence-статуса; отсутствие данных ≠ ноль.

---

## 1. Сводная матрица

Легенда: ✅ есть и подтверждено (live-verified/implemented с evidence) · 🔶 частично
(ядро есть, нет UI/дистрибуции/live-verify) · ❌ нет ни кода, ни плана ·
🚫 сознательно отклонено с записанной причиной (не долг).

| # | Фича (ожидание рынка) | Лучшие в классе | У нас | Комментарий |
|---|---|---|---|---|
| F1 | VLESS (+WS/CDN, REALITY), Hysteria2 — «свежие» анти-DPI протоколы | Hiddify, Amnezia, v2rayN | ✅ | VLESS+WS+TLS (канал A) и HY2 live-verified (V2-004, V2-017); REALITY-клиент готов (V2-032), ждёт VPS-B |
| F2 | ≥2 протокола с разными классами отказа (TCP + UDP) | Amnezia, Hiddify | 🔶 | Оба канала на одном VPS = один failure domain; второй ASN — B1-VPS-1 |
| F3 | Auto-failover по состоянию пути, а не «тупой ping» | Hiddify (url-test), Proton | 🔶 | A4 реализовано (V2-027) по protected probe — глубже рынка; live-verify после B1 |
| F4 | Kill switch (fail-closed) с честной семантикой | Proton, Mullvad, Amnezia | ✅ | Наша модель сильнее: probe-gate + fail-closed + BLOCKED вместо «прямого» трафика (V2-026 — подтверждено live как работающее) |
| F5 | Anti-probing: неавторизованный запрос выглядит как сайт | REALITY-инбаунды, Amnezia | 🔶 | Канал B закрывает дизайном REALITY; для канала A (WS) — B3, открыто (THEORY §5) |
| F6 | Подписанная доставка конфигов/обновлений профилей | Outline (ключи), Proton (API) | ✅ | B2/FR-008: Ed25519-envelope, антидаунгрейд, ревокации, evidence-gate (V2-033/034, live data-path) |
| F7 | Общий self-hosted lifecycle (XRay/Sing-box на сервере) | Amnezia, HiddifyManager, Outline | 🔶 | Сервер жив и проверен, но установка — руками по SSH, нет серверного инсталлятора |
| F8 | Подключение новых устройств по ссылке/QR | Amnezia, Hiddify, Outline | ❌ | Нет экспорта клиентской конфигурации в подписку/QR |
| F9 | Импорт подписок/конфигов по URL | v2rayN, Hiddify, Clash | ❌ | FR-008 — только signed envelope; публичные подписки осознанно не поддерживаются (совместимо с fail-closed моделью) |
| F10 | Сплит-туннелинг по приложениям/доменам | Proton, Amnezia, Psiphon | ❌ | RFC1918/.local в клиентском конфиге; пользовательских правил нет |
| F11 | Kill switch на уровне адаптера (TUN без утечек при падении движка) | Mullvad, Proton | 🔶 | Persistent WFP policy + bounded dead-man lease реализованы V2-091 и покрыты unit-гейтами; elevated crash/cleanup + полный TUN e2e остаются обязательным live evidence |
| F12 | Трафик/скорость в реальном времени | Clash Verge, v2rayN, Hiddify | ❌ | Запрет AGENTS §3.4/UI-FEATURES: нет достоверного источника метрик; нужен контракт (Clash API/счётчики движка) до реализации |
| F13 | Трей-иконка + быстрое управление | почти все | ❌ | Wails SingleInstanceLock есть; системный трей не реализован |
| F14 | Автозапуск с системой, автоподключение к доверенной сети | Proton, Amnezia | 🔶 | --auto-connect как флаг режима/перезапуска есть; автозапуска нет |
| F15 | Обновления приложения с проверкой подписи | Proton, Mullvad, Hiddify | ❌ | Инсталлятор есть (V2-029), автообновления/подпись кода — нет |
| F16 | Мультиплатформенность (Android/iOS/macOS/Linux) | все | ❌ | Только Windows; Android — Phase C (не начат) |
| F17 | DNS-защита: туннелированный DoH/DoT, анти-утечки | Proton (NetShield), Amnezia (AmneziaDNS) | ✅ | DNS через туннель — обязательный шаг probe; NetGuard чинит DoH вне туннеля |
| F18 | Блок рекламы/трекеров | Proton NetShield, Surfshark CleanWeb | ❌ | Осознанно вне ядра продукта; решается правилами route при появлении контракта |
| F19 | Мультихоп/Secure Core | Proton, Mullvad | ❌ | Двухсерверная топология не спланирована; не конкурентное преимущество в антицензор-нише |
| F20 | quantum-resistant / DAITA-класс статистической защиты | Mullvad | ❌ | Mullvad-эксклюзив, антицензор-аналоги не имеют; референс на будущее |
| F21 | Аудит безопасности / no-logs политика | Proton, Mullvad, Amnezia | 🔶 | Кодовая гигиена сильная (fail-closed, секреты-DPAPI, reveal-аудит), но внешний аудит невозможен до релиза |
| F22 | Диагностика окружения (UDP, admin, reachability, secret verify) | Psiphon/Amnezia слабее; Hiddify (ping) | ✅ | Tests page + TestSecret по видам секретов (V2-014) — глубже большинства |
| F23 | Самопочинка системы (прокси/DNS после падений) | практически нет у конкурентов | ✅ | NetGuard (V2-020/022/023) — уникальное преимущество |
| F24 | Активный сторож защищённого пути с probe каждые 30с | нет прямых аналогов | ✅ | watchCore + failoverPolicy (V2-025/027) |
| F25 | Сервисный режим (учётка/служба для TUN без UAC-перезапусков) | Proton, Amnezia | ❌ | Сейчас UAC relaunch (§4.1); сервис — отдельное архитектурное решение |
| F26 | Мультиязычный UI | Hiddify, Proton | 🔶 | UI на русском; структура i18n не заложена |
| F27 | Тёмная/светлая тема, удобный UX | почти все | 🔶 | Тёмная тема Wails; светлой нет |
| F28 | Экспорт/импорт локальной конфигурации | Outline (ключи), Amnezia | ❌ | Связано с F8; сейчас только локальный vault DPAPI |
| F29 | Свободная лицензия + открытый код клиента | Amnezia, Hiddify, Psiphon | 🔶 | Репозиторий приватный; лицензия не выбрана |
| F30 | Статус/телеграм-канал рассылки конфигураций при блоках | Amnezia Free, Psiphon | ❌ | Функционально заменяется B2 envelope; канал доставки — не приложение, а процесс |

## 2. Классы конкурентов — что берём, что нет

### 2.1. AmneziaVPN — эталон self-hosted UX
Берём в бэклог: F8 (QR/ссылка на конфиг), F7 (серверный инсталлятор),
F14 (автозапуск), F10 (сплит-туннелинг). Не берём: AmneziaWG как протокол
(наш pinned sing-box 1.13.19 не содержит AWG; смена ядра отклонена в V2-031),
Amnezia Premium/Free (не наш класс — мы не сервис).

### 2.2. Hiddify — эталон мультипротокольного клиента
Берём: F3 (уже сделано лучше — по protected probe, не по ping), F5 (anti-probing
через REALITY), F12 (после контракта на источник трафика), F16 (Android Phase C).
Не берём: WARP-интеграция (внешняя зависимость, ломает fail-closed гарантии),
Psiphon/TUIC/Naive/Mieru протоколы (не в pinned ядре; §2.4 AGENTS о зависимости
от внешних ядер), мультиподписки.

### 2.3. Outline — эталон простоты раздачи доступа
Берём: F8 (access-key/QR семантика), F28 (экспорт конфигурации). Не берём:
Shadowsocks (уступает современным протоколам; V2-031 — сознательный выбор
REALITY), Outline Manager (панель не нужна — у нас метаданные через подписанный
envelope, F6).

### 2.4. Psiphon — эталон устойчивости рассылки доступа
Берём идею (F30) только как процесс: при блоке канала владелец рассылает новый
подписанный envelope. Не берём: L2TP/IPSec (устарел), рекламную модель,
централизованные серверы (не self-hosted).

### 2.5. v2rayN/Nekoray/Clash Verge Rev — эталон контроля
Берём: F12 (трафик/соединения — через строгий контракт источника), F13 (трей),
F14 (автозапуск). Не берём: редактор сырых конфигов в UI (противоречит
fail-closed валидации), url-test как владелец выбора канала (AGENTS §2.3/§3.3 —
политику выбирает только наш failover-контроллер).

### 2.6. Proton/Mullvad — эталон trust & privacy
Берём: F15 (подписанные обновления), F21 (внешний аудит после релиза),
F17 (уже есть), F11 (TUN kill switch firewall). Не берём: Secure Core/мультихоп
(F19) и DAITA (F20) как обязательные — в антицензор-нише их нет у Amnezia/
Hiddify/Psiphon; фиксируем как референсы, не как долг.

## 3. Итог: минимальный набор для «лучшего в классе»

Без закрытия этих пунктов претензия «лучший сервис» некорректна (внутри
антицензор-ниши, а не против Proton по широте фич):

1. **F2 + F3 live-verified** — второй физический канал (B1-VPS-1) и живая
   проверка auto-failover (A4 live) — это уже в существующем плане;
2. **F5 anti-probing для канала A** — B3;
3. **F8/F28** — QR/ссылка/экспорт конфигурации нового устройства (нет в плане);
4. **F13 + F14** — трей и автозапуск (нет в плане);
5. **F15** — подписанные автообновления приложения (нет в плане);
6. **F10** — сплит-туннелинг по приложениям/доменам (нет в плане);
7. **F12** — честный трафик/соединения через новый контракт источника (нет в плане);
8. **F16** — Android-клиент (Phase C существует, не начат);
9. **F25** — сервисный режим TUN без UAC-перезапусков (нет в плане);
10. **F29** — открытая лицензия/публичный код (нет в плане; решение владельца).

## 4. Отдельные требования рынка к «сервису»

Конкуренты уровня Proton/Mullvad задают и не-функциональные ожидания, которые
в антицензор-нише пока не закрывает никто, но о них стоит помнить: RAM-only
инфраструктура, публичные no-logs аудиты, канареечные релизы, bug bounty,
прозрачные отчёты. Это уровень не MVP, а зрелого сервиса — фиксируется как
долгосрочный вектор, не как задача плана v2.

---

## 5. Обновление рынка (V2-047, 2026-09-14): что изменилось с V2-035

Метод тот же: классы аналогов, но с прогоном по свежим источникам (блоги
Amnezia/Proton/Mullvad, обзоры клиентов 2026, полевые гайды по обходу). Ниже —
только дельта и выводы, матрица §1 сохранена как история.

### 5.1. Что нового у конкурентов за полгода

| Конкурент | Дельта 2026 | Значение для нас |
|---|---|---|
| **Amnezia** | AmneziaWG **2.0 → 3.1**: маскировка VPN-трафика под «обычные» протоколы (QUIC, DNS и др.), self-hosted — с Sep 2026 | Новый класс протокольной маскировки, которого нет в sing-box. Не берём (смена ядра отклонена в V2-031), но концепция «транспорт под видом QUIC/DNS» — в THEORY как референс |
| **Hiddify** | v4: **двухъядерность** (Sing-box + Xray), XHTTP, встроенные Psiphon/Naive/Mieru | Двухъядерность = два набора протоколов. Наше ядро pinned сознательно; XHTTP берём на заметку (см. 5.3) |
| **Throne / Karing / FlClash** | Новая волна sing-box/Xray-клиентов, Throne (наследник NekoRay) — быстрый рост на Windows | Конкуренция в классе «обёрток» усилилась; их слабость прежняя — нет политики fail-closed и честного evidence |
| **v2rayN** | Cross-platform (Avalonia), в Microsoft Store, активные routing-rulesets для RU | Эталон контроля (F12/F13) остаётся актуальным |
| **Proton** | Авг 2026: полная перестройка протокольного стека, Stealth приходит на Linux, курс на post-quantum | F15/F21-план подтверждён; Stealth как референс для канала B |
| **Mullvad** | DAITA + quantum-resistant по умолчанию на десктопе; OpenVPN удалён (Jan 2026) | Ставка на статистическую защиту — референс (F20), не долг |
| **Класс zapret/ByeDPI/youtubeUnblock** | Массовый рост локальных no-VPS обходчиков (фрагментация TLS, fake-packets) | Класс **не покрыт** нашей матрицей V2-035 — см. 5.2 |

### 5.2. Новый класс: локальные обходчики без VPS (zapret/ByeDPI)

Не конкуренты в строгом смысле (не приватность, не шифрование — только доступ),
но **функционально пересекаются** с нами в главной точке боли: «открыть заблокированное».
Что важно:

- **Дополняют, а не заменяют**: работают только против SNI/DPI-блоков, ничего не
  дают против IP-блоков (наша TH-диагностика §2). Не дают egress — AI-сервисы
  с challenge останутся недоступны.
- **Zero-infra**: не требуют VPS — а мы требуем по определению продукта.
- **Идея для нас**: режим «обход DPI без туннеля» как **вспомогательный слой**
  (например, для сетей, где туннель не поднимается). Требует отдельного контракта
  (WinDivert/WinTUN-совместимость, fail-closed семантика «это НЕ защита»).
  Не берём в план без owner-решения.

### 5.3. Что изменилось в ТСПУ-ландшафте (перекликается с THEORY §2.4a)

Полевые гайды 2026 подтверждают и уточняют наши выводы:

- **Эвристика 15 КБ**: цензор режет «зарубежный IP + >15 КБ в одном TCP-соединении»
  без разбора протокола — именно поэтому у нас vless-CDN деградирует волнами, а
  HY2/REALITY живут. Подтверждает стратегию «транспорт важнее протокола».
- **VLESS+XHTTP+REALITY — новый мейнстрим серверной части** (Xray-экосистема);
  XHTTP распределяет нагрузку соединения и лучше живёт под поведенческим детектом,
  чем чистый WS. В sing-box ближайший аналог — splithttp-транспорт;
  кандидат на канал-будущего (требует серверного эксперимента).
- **Каскад client → RU-VPS → EU-VPS** остаётся рабочим обходом белых списков
  мобильных БС. Для нас неактуален (нужен второй VPS), но подтверждает ценность
  B1-топологии.
- **Белые списки мобильных сетей сужаются** — окно обхода через RU-релей закрывается.
  Усиливает приоритет независимых зарубежных путей (наш план).

### 5.4. Обновлённый вывод: где мы сильнее, где слабее

**Сильнее рынка (подтверждено):**
1. **Честность статусов** (evidence-gate, fail-closed) — у всех конкурентов
   «подключено» = «сокет открылся», без probe-гейта и без честного BLOCKED.
2. **Сторож + protected probe каждые 30с** — нет прямых аналогов в классе.
3. **NetGuard-самопочинка** системного прокси/DNS — уникально.
4. **Секреты в DPAPI + reveal-аудит** — у self-hosted конкурентов креды в файлах/QR.
5. **Живая диагностика окружения** (UDP/elevation/secret-test по видам) — глубже,
   чем ping-тесты Hiddify/v2rayN.

**Слабее рынка (честно):**
1. **UX первого запуска**: Amnezia/Hiddify ставятся и работают за 2 минуты;
   наш путь «дескриптор + SSH-деплой + сервер руками» — для владельца, не для
   пользователя. F7/F8 закрывают ровно это.
2. **Трей + автозапуск** (F13/F14) — банально, но этого ждут все.
3. **Сплит-туннелинг** (F10) — есть у Proton/Amnezia/Psiphon, у нас нет.
4. **Трафик/соединения в реальном времени** (F12) — есть у всех «обёрток».
5. **Мультиплатформенность** (F16) — только Windows.
6. **Готовый серверный инсталлятор** (F7) — Amnezia ставит всё в один клик.

### 5.5. Решения по бэклогу (дельта к §3)

- **Берём в план** (новое): F10-легковесный сплит по доменам через route-правила
  рендера (без UI-редактора — через подписанный envelope); F12 через clash-api
  контракт (соблюдая fail-closed: только read-only метрики); F13-трей.
- **Оставляем отклонённым**: AWG/Xray-ядро, WARP-интеграция в клиент (серверный
  WARP уже есть и это правильный слой), локальные no-VPS обходчики как фича.
- **Референсы без обязательств**: AmneziaWG-стиль QUIC/DNS-мимикрии (для
  канала-будущего), splithttp-транспорт как альтернатива WS на CDN, Stealth-модель
  Proton для канала B.

<a id="comfort-research"></a>
## 6. Исследование бытовых и рабочих сценариев — 2026-10-21

### 6.1. Метод и границы

Проверены публичные product docs, upstream деревья/выбранные реализации и
LICENSE: 13 upstream HEAD зафиксированы полным commit SHA; дополнительно
исследован уже закреплённый fork SagerNet/tailscale. Получены 49 файлов (14
лицензий, manifests/README/релевантные исходники); прочитаны относящиеся к
решениям участки, не весь код репозиториев. Текущему sing-box отдельно сопоставлен
tag v1.13.19. Это статическое исследование, без исполнения чужого кода, сборок,
тестов, установки зависимостей или подключения к пользовательскому VPN/VPS.

HEAD здесь означает конкретный прочитанный снимок, **не выбранный стабильный
release**. При заимствовании проверить release/advisories, transitive лицензии,
SBOM, platform APIs и нашу приёмку. В таблице «взять» означает рекомендацию,
а не выполненную интеграцию. Реестр задач и все их статусы — PLAN COM-00…19.
Исходные снимки/HTTP результаты/SHA256 находятся в локальной ignored
`.artifacts/research/2026-10-21-comfort/`; переносимые решения и ссылки — здесь.

### 6.2. Проверенные ожидания рынка

| Первичный источник | Подтверждённая возможность / существенная граница | Связь с планом |
|---|---|---|
| [Proton Profiles](https://protonvpn.com/support/connection-profiles) | Сохранение location/protocol/settings; Connect and Go открывает приложение/сайт после подключения; в профилях также есть NAT/port-forwarding настройки. Наличие зависит от плана/платформы | COM-01/04/19; пользовательский запуск после нашего свежего protected gate |
| [Windscribe Split Tunneling](https://windscribe.com/features/split-tunneling) | Inclusive/exclusive app/hostname/IP policies; отдельно заявлено, что DNS остаётся системным ROBERT/custom DNS даже для исключений | COM-04/06: нельзя объявить split DNS автоматически решённым app bypass |
| [Amnezia Split Tunneling](https://docs.amnezia.org/documentation/instructions/vpn-split-tunneling/) | Разные возможности ОС; Windows app-exclusion имеет ограничения Store/anti-cheat. Документация предупреждает о разовом IPv4 resolution доменов и сложных зависимостях сервисов | COM-04/16: dynamic service rules, явная platform matrix; не обещать «любой exe/домен» |
| [IVPN firewall](https://www.ivpn.net/knowledgebase/general/do-you-offer-a-kill-switch-or-vpn-firewall/) и [exceptions](https://www.ivpn.net/knowledgebase/general/firewall-exceptions/) | Always-on/boot firewall, LAN/subnet exceptions, сценарий одновременного корпоративного VPN; исключение firewall и маршрут — разные настройки | COM-03/05/06: service ownership + согласованная policy; coexistence только проверенная |
| [Mullvad desktop](https://mullvad.net/en/help/using-mullvad-vpn-app) | Auto-connect, LAN, lockdown, split, DNS blockers; DAITA и multihop имеют цену в traffic/battery/latency | COM-02/03/05/18: редкие функции — отдельный opt-in, не бесплатное ускорение |
| [Tailscale app connectors](https://tailscale.com/docs/reference/best-practices/app-connectors) | DNS-derived A/AAAA/CNAME routes, invisible split DNS, approval/ACL и зависимости private network | COM-04/06/16: алгоритмический референс; импортировать control-plane SaaS в клиент не нужно |
| [Speedify bonding vs local balancing](https://support.speedify.com/article/892-can-i-use-speedify-without-vpn-server), [Enhance Streaming](https://support.speedify.com/article/692-enhance-streaming-mode) | Bonding использует общий server endpoint; local balancing распределяет sockets и не сохраняет упавший поток. Приоритизация/дублирование media расходуют ресурсы | COM-09/12/17: не путать отсрочку вмешательств, failover и seamless handover |
| [TUF overview](https://theupdateframework.io/docs/overview/) | Защита обнаружения/скачивания targets от rollback/freeze и компрометации ключей через metadata roles | COM-14: TUF не устанавливает EXE, не исправляет Windows service/crash rollback |

Это целевая выборка, не перепись всех VPN. «Редкое» — отобранные примеры, без
статистической частоты. Новизна COM-07…11 — гипотеза ценности сочетания UX и
policy, а не установленная уникальность. Consumer VPN, mesh/private access и
router bonding решают разные задачи; сравнивать по одному слову VPN некорректно.

### 6.3. Что уже есть у Snowden — source inspection

| Опора | Фактические файлы | Что ещё требуется |
|---|---|---|
| Трей/автозапуск | `windows/tray_windows.go`: systray, каналы, HKCU Run с --auto-connect; dependency уже v1.2.2 | COM-02/03: coherent applied меню, ошибки registry, native UX/boot gates. Заново устанавливать systray не нужно |
| Перенос/QR | `windows/onboarding.go`, `backend/onboarding/`, `onboarding_import.go`: encrypted Seal/Open, ExportOnboardingQR, staged import | COM-01/13: устройство/отзыв/provisioning, oversized QR, UX. Текущий общий секрет не становится per-device только от копирования |
| Process split | `backend/render/render.go`: signed split_direct→process_name; непустое условие перед direct; coherent snapshot V2-113 | COM-04: цели/modes/identity/DNS/priority/expiry. Текущая схема RouteRule имеет лишь Protocol/Action/Outbound/IPCidr/DomainSuffix/ProcessName; не принимает весь upstream API |
| Network recovery | `windows/netwatch.go`: polling/coalescing/reprobe; при tunRequested наблюдатель пропускает итерацию | COM-03: callbacks physical path и работа под TUN — конкретный пробел; изменения только через Manager/epoch |
| Наблюдения/лимиты | `backend/core/probe_http_observation.go`, `diagnostic_budget.go`, `diagprobe.go` | COM-07/11/12: развить, не импортировать второй HTTP-prober целиком |
| Обновление | `backend/update/update.go`, `windows/updateapp.go`: manifest/signature/hash/version floor, local staging/подмена | COM-14: crash/service ownership, full delivery roles и platform acceptance; «нет обновлений» в старой матрице неверно |
| Защита/зависимости | `wfp_killswitch_windows.go`, `backend/render/dependency_graph.go`, TH-01/04 | P0/OS gates открыты, исходники после V2-110R unverified; source presence не текущая безопасность |

Корневой файл лицензии продукта в исследованном наборе не найден; выбор
распространения/NOTICE остаётся отдельным gate. Существующий embedded sing-box
уже имеет GPL-условия: новая рекомендация не означает, что копирование любого
GPL-кода автоматически согласовано. Текущий remote visibility/auth не проверялись.

### 6.4. Каталог пригодных реализаций

Классы: **reuse existing** — уже есть в проекте; **adapt** — взять ограниченную
реализацию с изменением контракта; **reference** — переиспользовать подход/UX;
**candidate** — отдельный compatibility spike; **defer** — не интегрировать сейчас.

| Проект / фиксированный снимок и источник | Лицензия просмотренного кода | Решение и конкретное заимствование |
|---|---|---|
| SagerNet/sing-box `dfb80349bf5779ec0a3f834c9ec2794345bff8f0`; наш [v1.13.19 option/rule.go](https://github.com/SagerNet/sing-box/blob/v1.13.19/option/rule.go), [option/dns.go](https://github.com/SagerNet/sing-box/blob/v1.13.19/option/dns.go) | [GPL-3.0-or-later](https://github.com/SagerNet/sing-box/blob/dfb80349bf5779ec0a3f834c9ec2794345bff8f0/LICENSE) | **Reuse existing**, COM-04/05/06/18: process/path/domain/IP/rule-set/DNS primitives поверх pinned ядра. Дописать нашу строгую schema, signed contract и компилятор правил; availability option не platform guarantee. Не обновлять ядро ради поля из HEAD |
| SagerNet/tailscale **v1.92.4-sing-box-1.13-mod.9**; [netmon Windows](https://github.com/SagerNet/tailscale/blob/v1.92.4-sing-box-1.13-mod.9/net/netmon/netmon_windows.go), [go.mod](https://github.com/SagerNet/tailscale/blob/v1.92.4-sing-box-1.13-mod.9/go.mod) | BSD-3-Clause; Go1.24.1 в manifest | **Первый adapt-кандидат**, COM-03. Уже indirect dependency, не новый SDK: Windows route/address callbacks, unregister, bounded delivery в наш epoch. Сначала проверить экспорт/API/зависимости; наличие в go.mod не доказывает готовую интеграцию. 2026-10-24 tag разрешён в immutable867c1f0012d915fa5fd57481dbf2a1ad16669fe7;3 exact files/Git blobs проверены, THEORY§15.4. OS/native integration пока не принята |
| Tailscale `47743a8b9ad16cf3a49504dc443208c37748072e`; [netmon](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/netmon/netmon_windows.go), [captive detection](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/captivedetection/captivedetection.go), [DNS manager](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/dns/manager.go) | [BSD-3-Clause](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/LICENSE); HEAD Go1.27.3 | **Adapt/reference**, COM-03/06/10. Callback monitor, interface-bound short HTTP/no redirects/204+challenge checks, split-DNS compiler. В detection есть netmon/DERP-specific dependencies: не переносить DERP адреса/бесконтрольный direct. Не импортировать весь свежий tailscale.com в Go1.26 и не считать detect=false доказательством отсутствия portal |
| tailscale/wf `6fbb0a674ee67f6c9b99eb7ce8a4330bb0145ec9`; [firewall.go](https://github.com/tailscale/wf/blob/6fbb0a674ee67f6c9b99eb7ce8a4330bb0145ec9/firewall.go) | [BSD-3-Clause](https://github.com/tailscale/wf/blob/6fbb0a674ee67f6c9b99eb7ce8a4330bb0145ec9/LICENSE) | **Reuse existing**, COM-04/05/08: этот exact commit уже direct dependency. Расширять наш WFP owner/transaction; не заводить второй engine session с чужими provider GUID |
| WireGuard Windows `6ece77bc487c8aa697e3c092197621c4f3e5ccb8`; [blocker.go](https://github.com/WireGuard/wireguard-windows/blob/6ece77bc487c8aa697e3c092197621c4f3e5ccb8/tunnel/firewall/blocker.go), [rules.go](https://github.com/WireGuard/wireguard-windows/blob/6ece77bc487c8aa697e3c092197621c4f3e5ccb8/tunnel/firewall/rules.go); официальный upstream git.zx2c4.com, GitHub mirror | [MIT](https://github.com/WireGuard/wireguard-windows/blob/6ece77bc487c8aa697e3c092197621c4f3e5ccb8/COPYING) | **Adapt helpers/reference**, COM-03/05: permit tunnel LUID, DNS restriction, DHCPv4/v6/NDP, weighted block and transaction. **Не брать blocker целиком:** FWPM_SESSION_FLAG_DYNAMIC не соответствует нашему persistent crash barrier. wireguard/windows v0.5.3 уже indirect, новый HEAD не выбран |
| IVPN desktop `1fd063064f466322c3749dc758f7e72dfbae4b99`; [Windows firewall](https://github.com/ivpn/desktop-app/blob/1fd063064f466322c3749dc758f7e72dfbae4b99/daemon/service/firewall/firewall_windows.go), [autoconnect CLI](https://github.com/ivpn/desktop-app/blob/1fd063064f466322c3749dc758f7e72dfbae4b99/cli/commands/autoconnect.go) | [GPL-3.0](https://github.com/ivpn/desktop-app/blob/1fd063064f466322c3749dc758f7e72dfbae4b99/LICENSE.md); file notices перед копированием | **Reference/adapt после license gate**, COM-03/05/06/08: persistent firewall, LAN/multicast/exceptions и settings semantics. Native firewall/driver/UI не переносить целиком в Wails; нельзя подменять наш lifetime/owned cleanup |
| Mullvad `52d4ab90aff4eb7d9455bfad071d144220b3a054`; [error_state.rs](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/talpid-core/src/tunnel_state_machine/error_state.rs), [persistent.rs](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/talpid-core/src/firewall/windows/objects/persistent.rs) | [GPL-3.0](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/LICENSE.md) | **Reference**, COM-03/05/08/18: blocked/error state применяет firewall policy с ограниченным endpoint/LAN, lifecycle отдельно от UI. Rust/FFI/driver stack — дорогое несоответствие; не добавлять вторую state machine. DAITA не код для произвольного transport |
| Proton Windows `d2a4f8bc92a0fd296943a7cdd15f4f870c8a87f9`; [SplitTunnel.cs](https://github.com/ProtonVPN/win-app/blob/d2a4f8bc92a0fd296943a7cdd15f4f870c8a87f9/src/ProtonVPN.Service/SplitTunneling/SplitTunnel.cs) и Profiles docs выше | [GPL-3.0](https://github.com/ProtonVPN/win-app/blob/d2a4f8bc92a0fd296943a7cdd15f4f870c8a87f9/LICENSE) | **UX/reference**, COM-01/04/19: сценарные настройки/Connect and Go, service-side split lifecycle. C#/service/native stack не Go drop-in; реализация Profiles в коде отдельно не разобрана — её подтверждает только product doc |
| Amnezia `02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc`; [Windows split](https://github.com/amnezia-vpn/amnezia-client/blob/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc/client/platforms/windows/daemon/windowssplittunnel.cpp), [controller](https://github.com/amnezia-vpn/amnezia-client/blob/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc/client/core/controllers/appSplitTunnelingController.cpp) | [GPL-3.0](https://github.com/amnezia-vpn/amnezia-client/blob/02838a94d5600bdc4ba9f898d8735cbe7d2f2cdc/LICENSE) | **Reference**, COM-01/04/13/16: список приложений, split settings и понятная платформа. Windows code устанавливает/запускает driver/service: нельзя копировать как простой process_name. Qt/C++ UI не переносить; шифрование onboarding уже своё |
| go-tuf/v2 `45ad19f5a821abbcdbd1ba10b994362d12d39965`; [Updater](https://github.com/theupdateframework/go-tuf/blob/45ad19f5a821abbcdbd1ba10b994362d12d39965/metadata/updater/updater.go) | [Apache-2.0](https://github.com/theupdateframework/go-tuf/blob/45ad19f5a821abbcdbd1ba10b994362d12d39965/LICENSE); HEAD Go1.27 | **Reuse совместимый v2.4.2/f5edbde31e5507f46db2069402dc38903fe6d9d4 по ADR-001; source integration unverified**, COM-14. Эта строка HEAD сохранена как historical comparator: trusted root→timestamp→snapshot→targets, target hash/length verification. Не переписывать криптографию; не включать UnsafeLocalMode для обхода recovery/expiry. Установка EXE, antirollback/atomic local state и Windows service остаются нашими |
| blackbox_exporter `d18e9bed33270911c09826234a4e4f500f775087`; [HTTP](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/prober/http.go), [DNS](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/prober/dns.go), [TCP](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/prober/tcp.go) | [Apache-2.0](https://github.com/prometheus/blackbox_exporter/blob/d18e9bed33270911c09826234a4e4f500f775087/LICENSE); HEAD Go1.26.0 | **Reference или готовый внешний optional binary**, COM-07/11/15: staged httptrace/DNS/TLS/certificate metrics. В клиенте расширить существующие observations/budget; exporter снаружи не доказывает protected path и не должен быть public SSRF endpoint |
| getlantern/systray `22c167e80905f87867fc05fbe553342578976d7b`; [Windows implementation](https://github.com/getlantern/systray/blob/22c167e80905f87867fc05fbe553342578976d7b/systray_windows.go) | [Apache-2.0](https://github.com/getlantern/systray/blob/22c167e80905f87867fc05fbe553342578976d7b/LICENSE) | **Reuse existing v1.2.2**, COM-02. HEAD commit 2023-11-10: это не доказательство активной поддержки в 2026; сначала native compatibility. Не менять зависимость на другой tray framework без обнаруженного дефекта |
| OpenMPTCProuter `238f810e9e200cd4fac7303a6a1e12adfe85f099`; [README](https://github.com/Ysurac/openmptcprouter/blob/238f810e9e200cd4fac7303a6a1e12adfe85f099/README.md) | [GPL-3.0](https://github.com/Ysurac/openmptcprouter/blob/238f810e9e200cd4fac7303a6a1e12adfe85f099/LICENSE); packages имеют свои лицензии | **Готовое отдельное решение/candidate**, COM-17/16: router(OpenWrt)+VPS anchor, MPTCP/Glorytun/MLVPN. Можно оценить как внешнюю инфраструктуру вместо создания bonding с нуля. В этом срезе прочитан README, не scheduler/installer; Windows портирование и bypass не доказаны |
| Glorytun `32267e86a6da05b285bb3bf2b136c105dc0af4bb`; [README](https://github.com/angt/glorytun/blob/32267e86a6da05b285bb3bf2b136c105dc0af4bb/README.md) | [BSD-2-Clause](https://github.com/angt/glorytun/blob/32267e86a6da05b285bb3bf2b136c105dc0af4bb/LICENSE) | **Defer**, COM-17: multipath UDP/backup/shaping/MTU — полезные направления; просмотренный HEAD от 2020-10-30 и README просит stable branch. 2026-10-24 stable branch/latest0.3.4 разрешены в c113724eb0370ecd80d038192deeeb82a13ebed3 от2020-03-25; stable README POSIX/Linux/libsodium. Core integration отклонена по цене/отсутствию обязательного use case. Полный native/crypto audit не выполнялся; reopen только по COM-17 scenario; не безопасный Windows drop-in |

**Приоритет reuse:** существующие sing-box/wf/systray/onboarding/probe → адаптация
уже pinned netmon → policy/DNS/UX на нашей архитектуре → совместимый go-tuf spike.
Полный чужой VPN-клиент не выбран. Защита после падения/boot и per-app enforcement
имеют разные lifetime/identity semantics — совпадение названия KillSwitch не gate.

### 6.5. Проверяемые алгоритмы адаптации

1. **События сети:** OS route/address callback → короткая неблокирующая enqueue →
   bounded coalescing → новый network epoch → одна recovery operation Manager.
   Отделить physical path от own TUN, снять callbacks при shutdown. Не копировать
   произвольное создание goroutine на каждое событие без наших hard bounds.
2. **App/service policy:** normalized selectors + precedence + DNS policy +
   explicit failure action → единый validated applied snapshot. DNS A/AAAA/CNAME/
   TTL обновления не превращать в глобальный direct IP allowlist shared CDN.
   In-app DoH/ECH/общий browser process означают unknown coverage; не обещать
   абсолютную точность per-site без совместимого browser/network механизма.
3. **Portal:** ограниченный physical-interface HTTP probe без redirect, ожидаемый
   ответ/nonce и конкурирующие причины → possible portal, не proof blocked VPN.
   Открытие страницы требует отдельного scoped grant COM-10; direct-detection
   не расширяется на protected apps. Не доверять URL редиректа как разрешению.
4. **Сбой/восстановление:** blocked policy и ограниченные bootstrap endpoints
   предшествуют engine operations; новое applied состояние публикуется после
   проверки. Перенести идеи IVPN/Mullvad в существующего owner, сохраняя FR-009.
5. **Диагностика сервиса:** stage observation с route/owner/epoch/bytes/deadline →
   объяснение конкретного правила/сбоя → адресное обратимое предложение; generic
   network probe не доказывает login/media/P2P или причину цензуры.
6. **Важная сессия:** lease от пользователя → запрет optional вмешательств до
   deadline → обязательные safety/revoke/recovery остаются разрешены. Сначала
   реализовать это; настоящий multipath требует своего server endpoint/протокола.
7. **Update delivery:** проверенные trusted metadata роли и target до staging;
   durable checkpoint/version floor и Windows replacement отдельно. Никакой
   регистрации/запуска команд из импортированного сценария, даже подписанного.

Для этих адаптаций нужны собственные acceptance из PLAN, а не предположение,
что upstream код без ошибок. Заимствование лицензированного кода требует
атрибуции; алгоритмическое изучение не разрешает удалить notices из копии.

### 6.8. Итог исследования

2026-10-24: [THEORY§15](../roadmap/RESEARCH-HANDOFF.md#source-15)/[PLAN§5.8](../../PLAN.md#research-completion) сводят весь desk scope. Все20 COM имеют disposition/canonical route/future gate в [92-ID coverage](../../../.artifacts/research/2026-10-24-research-completion/audit/coverage.json). Историческое сравнение §§1–5 не становится current market claim. Research завершён; implementation/native/UX/comparative и выбор optional scenario остаются по плану.
