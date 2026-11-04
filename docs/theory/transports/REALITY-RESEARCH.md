# Phase B1 — исследование REALITY и XHTTP для канала B

> Дата: 2026-09-11. Статус: **research done (V2-031); клиентская сторона реализована
> и live-verified (V2-032)** — дескриптор schema 2, рендер, строгая валидация,
> probe-семантика готовы; канал B появится в селекторе сразу после заполнения
> дескриптора и секретов. Осталось: VPS-B (B1-VPS-1, owner) → сервер → секреты → live-verify.
> Серверная реализация — после procurement VPS-B (за owner). Всё, что помечено
> «доказано по коду», проверено по исходникам pinned-ядра
> `github.com/sagernet/sing-box@v1.13.19` (go.mod windows/), а не по документации.

<a id="capability-correction-2026-10-27"></a>
**Уточнение 2026-10-27:** статус выше и §7a относятся к историческому срезу,
не к текущему состоянию сервера или приложения. Независимая
[проверка R02-01](../research/reviews/2026-10-27-R02-01-VISION.md) исправляет
утверждение об отсутствии Vision в публичной upstream closure; no-flow профиль
не меняется. Ниже также сняты неподтверждённые рейтинги устойчивости и
смешение порогов из разных полевых сообщений. Основания и единицы измерения:
[методическая поправка](../methodology/RESEARCH-METHOD.md#measurement-comparability-2026-10-27).
Исходников Windows и реальных runtime configs в этой проверке не было.
Дополнительная source-поправка того же дня: `reality-keypair` **есть** в upstream
CLI, её регистрация находится в `cmd_generate_wireguard.go`; одно имя файла
не определяет область возможностей. [Основание](../research/reviews/2026-10-27-R02-01-VISION.md#reality-keygen-source-2026-10-27).

---

## 1. Цель и границы

Канал B нужен, чтобы у проекта стало **два независимых failure domain** (AGENTS §3,
THEORY §3.6): сейчас канал A (VLESS+WS+TLS через CF) и канал C (HY2/UDP) живут на
одном VPS (AdminVPS, AS57043, NL) и одном CF-аккаунте — один инцидент хостера
роняет оба.

Вопрос исследования: какой транспорт для канала B — **VLESS+TCP+REALITY**,
**XHTTP** или ещё один WS-канал? Исследование покрывает: поддержку ядром,
устойчивость к актуальной цензуре (РФ-сети, TSPU), интеграцию в наш render/
validator/probe, секреты. Не покрывает: выбор конкретного хостера и донор-сайта
(решение owner), деплой.

## 2. Что умеет наше ядро (доказано по коду v1.13.19)

| Возможность | Статус в pinned ядре | Evidence |
|---|---|---|
| REALITY server | ✅ есть | `option/tls.go:193 InboundRealityOptions{Handshake,PrivateKey,ShortID,MaxTimeDifference}`; `common/tls/reality_server.go` |
| REALITY client | ✅ есть, **только под `with_utls`** | `option/tls.go:234 OutboundRealityOptions{Enabled,PublicKey,ShortID}`; `common/tls/reality_client.go:1 //go:build with_utls` — тег уже в нашей сборке |
| XTLS Vision (`flow: xtls-rprx-vision`) | Есть в public upstream closure; product admission и binary отдельно | sing-box manifest требует sing-vmess `3aed155119a174c9290a4099841049f0cb275e02`, VLESS callers вызывают `NewVisionConn`; [review 2026-10-27](../research/reviews/2026-10-27-R02-01-VISION.md) |
| XHTTP transport | ❌ нет | нет `type: xhttp` в transport-фабрике; upstream SagerNet/sing-box issue **#3076 «is xHTTP transport support in the core planned?» — открыт** (подтверждён поиском 2026-09) |
| `sing-box generate reality-keypair` | Есть в public upstream CLI | [cmd_generate_wireguard.go](https://raw.githubusercontent.com/SagerNet/sing-box/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/cmd/sing-box/cmd_generate_wireguard.go): `init` регистрирует `commandGenerateRealityKeyPair`; это не генерация ключей в текущей сессии и не CLI Snowden |

Ключевой факт про XHTTP: он существует в **форках**. Наш анализ `sing-box-lx`
(README, 2026-09): XHTTP там — *client transport only*, «live-validated against
real Xray servers»; серверная сторона — это Xray-core. То есть «XHTTP-канал» для
нас означал бы либо Xray на сервере + форк-ядро в клиенте (смена pinned-ядра =
новый supply-chain контракт, против PLAN §1.6 и V2-015), либо ждать upstream.
Это отдельное большое решение, не входящее в B1.

**Исторический baseline: VLESS+TCP+REALITY без Vision.** Наличие Vision в
dependency closure исправляет основание выбора, но не снимает продуктовый
запрет flow и не требует его включать. Anti-probing B3 не закрывается названием
транспорта: нужны проверки корректного и неавторизованного клиента по E-04.

## 3. Модель угрозы 2026 (что изменилось в РФ-сетях)

1. **Объёмная остановка, исторические сообщения.**
   [net4people #490](https://github.com/net4people/bbs/issues/490#issue-3183910059)
   открыт 2025-06-27: исходно описаны примерно 15–20 KB **server→client на одно
   TCP-соединение**, с различиями по провайдеру. В отдельном upd5 заявлены
   25 пакетов для TCP/UDP в любом направлении; дата вставки и точное правило
   счёта здесь не установлены. Это не равенство «16 КБ = 25 пакетов».
   [Cloudflare, 2025-06-26](https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/)
   сообщает об ограничениях собственных CDN-сервисов, в том числе QUIC.
   Поэтому CDN/SNI не дают оснований считать A более устойчивым, а HY2 первым
   кандидатом на отказ. Описанные SNI-исключения не гарантируют обход отдельного
   IP allowlist. REALITY не требует существования универсального «белого SNI».
2. **Статус REALITY и причинность.**
   [net4people #546, 2025-11-14](https://github.com/net4people/bbs/issues/546#issue-3627041588)
   описывает иной предполагаемый режим на части сетей; автор отделяет его от
   #490 и указывает ограниченность данных. Полезность смены порта/mux оказалась
   неоднородной. Эти сообщения не устанавливают ML-алгоритм ТСПУ, лидерство
   REALITY по anti-probing или MITM данного профиля через произвольный OS CA.
   Статистический анализ, IP-блокировка и компрометация доверия требуют разных
   моделей и проверок. Разные wire profiles являются кандидатами резерва,
   а не доказательством независимости или сравнительной устойчивости.
3. **uTLS** [документировано, sing-box docs 1.13]: upstream официально помечает
   uTLS «Not Recommended» (повторявшиеся fingerprint-уязвимости, качество
   библиотеки). Для REALITY-клиента в sing-box uTLS обязателен (доказано по
   коду, §2). Наша позиция: тег уже в сборке; fingerprint — дефолтный chrome;
   ограничение фиксируем честно, миграцию «наивпрокси-подобных» решений не
   затеваем.
4. **Vision: реализация не равна эффекту.** Она есть в указанной public closure,
   но описанный профиль остаётся no-flow. Ни наличие Vision, ни его отключение
   сами по себе не устанавливают изменение детектируемости. Нужны exact artifacts,
   compatibility и сопоставимый workload; сравнительные измерения не выполнены.

## 4. Сравнение кандидатов

| Критерий | Канал A: VLESS+WS+TLS (через CF) | **Канал B: VLESS+TCP+REALITY** | XHTTP (за форками/Xray) |
|---|---|---|---|
| Ядро клиента (pinned 1.13.19) | ✅ сегодня live-verified | ✅ встроено (см. §2) | ❌ нет; нужен форк в клиенте |
| Ядро сервера | sing-box 1.13.19 | sing-box 1.13.19 | Xray-core (или форк sing-box-lx — client-only) |
| Новый ASN/country (failure domain) | — | ✅ да (суть B1) | ✅ да |
| Видимость входного IP | Наблюдатель видит CF edge; другие каналы раскрытия origin остаются | Наблюдатель видит IP VPS; donor fallback не скрывает этот IP | Зависит от выбранной CDN/raw схемы |
| Anti-active-probing | HTTP400 сам по себе не доказательство VPN; нужен cover control | Donor fallback по дизайну; превосходство не измерено | Зависит от реализации; сопоставимого корпуса нет |
| Устойчивость к объёмной остановке (§3.1) | Рейтинг не установлен | Рейтинг не установлен | Рейтинг не установлен |
| UDP-зависимость | нет | нет | нет |
| Новая зависимость/риск | — | **нет** (то же ядро, новый inbound) | смена ядра (форк) = supply-chain решение, сознательно отложено |
| Интеграция (render/validator/probe) | готово | умеренная (дескриптор schema 2 + TLS-reality в validator) | крупная (новый транспорт везде) |
| Не даёт failover-разнообразия по… | — | даёт TCP-класс отказа (не QUIC, не CDN) | дублирует CDN-класс канала A |

## 5. Профиль канала B (v1, к реализации)

Сервер (VPS-B: **другой ASN, другая страна** — жёсткое требование B1; небольшой
региональный хостер предпочтительнее «известных» DC-сетей (Hetzner/DO/OVH чаще
под фильтром/в списках — §3.1 upd4); KVM, Ubuntu 24.04, 1 vCPU/1–2 ГБ достаточно;
желательна возможность смены IP, если IP попадёт под флаг):

```json
{
  "type": "vless", "tag": "vless-reality-in",
  "listen": "0.0.0.0", "listen_port": 443,
  "users": [{ "name": "user-1", "uuid": "{{VLESS_UUID_B}}" }],
  "tls": {
    "enabled": true,
    "reality": {
      "enabled": true,
      "handshake": { "server": "{{DONOR_HOST}}", "server_port": 443 },
      "private_key": "{{REALITY_PRIVATE_KEY}}",
      "short_id": ["{{REALITY_SHORT_ID}}"],
      "max_time_difference": "1m"
    }
  }
}
```

Клиент (outbound в рендере):

```json
{
  "type": "vless", "tag": "channel-b-reality",
  "server": "{{VPS_B_IP}}", "server_port": 443,
  "uuid": "<ref: vless-uuid-b>",
  "tls": {
    "enabled": true, "server_name": "{{DONOR_HOST}}",
    "utls": { "enabled": true, "fingerprint": "chrome" },
    "reality": { "enabled": true, "public_key": "<ref: reality-public-key-b>",
                 "short_id": "<ref: reality-short-id-b>" }
  }
}
```

Правила профиля (по образцу FR-007):
- `flow` НЕ задаётся в сохранённом baseline; это выбор профиля, а не отсутствие
  Vision в upstream. Возможный opt-in требует отдельной приёмки обоих концов;
- выбор донора (owner): популярный сайт, TLS 1.3 + ALPN h2, X25519 key exchange,
  не наш домен/зона, не CDN нашего VPS, желательно из «белого» SNI-набора цензора
  (проверка донора с VPS-B: `openssl s_client -connect <donor>:443 -tls1_3 -alpn h2`);
- секреты: `vless-uuid-b`, `reality-private-key-b` (только сервер),
  `reality-public-key-b`, `reality-short-id-b` → файлы в `secrets/channels/` +
  DPAPI-слоты; значения не в git/чат/логи; генерация ключей — X25519 + base64.RawURLEncoding
  (в upstream CLI есть `generate reality-keypair` — см. §2; наличие этой команды
  не доказывает её доступность в Wails-приложении, генерация здесь не выполнялась);
- дескриптор: **schema 2** — `transport: "tcp-reality"` + явные роли ссылок
  (`uuid_ref`, `reality_public_key_ref`, `reality_short_id_ref`, все ∈
  `credential_refs`, ровно три разных); schema 1 продолжает приниматься;
  **реализовано (V2-032)** — см. windows/backend/render/render.go;
- validator/render: **реализовано (V2-032)** — validateRealityTLS (полный
  reality-блок обязателен: enabled/public_key/short_id), запрет любого
  xtls-flow на защищённом outbound (историческая product policy, не отсутствие Vision),
  формат-гварды рендера: public key = 43-симв. base64url (X25519,
  base64.RawURLEncoding), short_id = hex ≤16 символов; insecure по-прежнему
  запрещён; публичность сертификата донора не заменяет проверку REALITY peer;
- probe: expected_egress = IP VPS-B; UDP-gate не нужен; TLS-preflight должен
  распознавать REALITY-хендшейк (не требовать наш сертификат);
- `route.final` на VPS-B = `direct` (урок V2-002), nft/ufw: 22/tcp, 443/tcp.

Anti-probing свойство дизайна: неавторизованный TLS может обрабатываться через
донора. Это один элемент E-04, не закрытие B3 и не доказанная неотличимость.

## 6. Риски и честные границы

- IP VPS-B остаётся видимым и блокируемым по CIDR (§3.1 upd4). Другой ASN/IP
  является кандидатом пути, но не гарантированным исправлением. Продукт не обещает
  работу при allowlist-режиме (THEORY L6/L7).
- Статистический детект не исключён ни у no-flow, ни у Vision по одному названию;
  эффективность конкретного профиля требует отдельного сравнения.
- uTLS «Not Recommended» upstream — известное ограничение клиента (§3.3).
- REALITY-хендшейк зависит от донора: если донор деградирует (перестанет отдавать
  TLS 1.3/h2 или уедет в чёрный список) — канал B деградирует; probe это поймает,
  но донора придётся менять руками.
- CF-TOKEN-1 (блокер PLAN §2.4) и procurement VPS-B — вне этого исследования;
  владельцу: при покупке VPS-B не использовать текущий CF-аккаунт для зоны канала B
  (независимый failure domain — смысл B1).

## 7. Критерии приёмки B1 (будущие)

1. `sing-box check` серверного конфига VPS-B + post-verify листенеров/journal (FR-006).
2. Клиент: render schema 2 → строгий парсер → Start → probe 6/6 через
   outbound/vless (reality) → egress = IP VPS-B; Stop чистый.
3. Неавторизованный HTTPS к VPS-B (без secret-marker) возвращает контент донора —
   анти-пробинг-проверка (curl без ключей из другой сети).
4. SelectChannel на channel-b-reality → Reload → probe — PASS (путь A1.4).
5. A4 получает второй validated канал: live-verify автоматического failover
   (снятие пометки V2-027 «переключению не с чего происходить»).
6. Все новые секреты — в DPAPI-vault, verify=ok; sha256-сверка с сервером; в git/
   логах значений нет (V2-013-процедура).

## 7a. Канал C: REALITY на том же VPS (V2-044) — что сделано, пока B1 в поиске

Пока владелец ищет VPS-B по ТЗ §4, транспорт REALITY развёрнут на СУЩЕСТВУЮЩЕМ
VPS-A (89.125.1.217) как третий канал `channel-a-reality`. Что это даёт и чего
не даёт:

Даёт:
- TCP-путь МИМО Cloudflare CDN (origin напрямую) — жив при деградации CDN-ветки
  (хроническая проблема этой сети, V2-026/039/040) и при проблемах UDP (когда
  QUIC/HY2 тонет, TCP+REALITY остаётся);
- anti-probing B3 для канала A: неавторизованный TLS уезжает на донора
  www.samsung.com (сертификат публичный, пиннинг не нужен);
- live-обкатку schema-2 конвейера (рендер/валидатор/формат-гварды/секреты) до
  появления VPS-B — ввод канала B сведётся к замене IP/донора в дескрипторе.

Не даёт (честно):
- второго failure domain НЕТ: тот же физический сервер, тот же ASN, тот же IP —
  при падении VPS-A умрут все три канала; при блокировке IP по CIDR — тоже;
- IP-репутацию не лечит (ChatGPT и т.п. чинит WARP-egress, V2-041).

Сервер: X25519 keypair (base64.RawURLEncoding, openssl DER-tail), UUID-C,
short_id df8098b2; inbound vless-reality-in на 0.0.0.0:8443/TCP (порт выбран
«TCP-близнецом» старого WS-порта для простоты firewall-политики); старый
vless-in (WS) переехал 8443→9443/localhost, nginx proxy_pass обновлён
(WS-путь канала A не тронут); ufw 8443/tcp; sing-box check + restart ok;
backup config.json.bak.20260912-reality. Донор выбран из «белого» SNI-набора,
проверен с VPS: TLS 1.3 + X25519 + ALPN h2 (§5-правила соблюдены).

Клиент: дескриптор channel-a-reality (schema 2, validation_status=configured —
гейт probe при переключении), 3 новых Kind в secretvault
(vless-uuid-c/reality-public-key-c/reality-short-id-c, формат-гварды 43-симв
base64url + hex≤16), значения заполнены в DPAPI-vault владельца через vaulttool.

Acceptance канала C (по §7, адаптировано): render+Parse ок (тесты), selector
[vless, hy2, reality] live в app.log; live-переключение владельцем из UI →
probe через outbound/vless(reality) → egress=89.125.1.217; curl без ключей на
89.125.1.217:8443 возвращает контент донора.

## 8. Источники

- pinned-ядро `sagernet/sing-box@v1.13.19`: `option/tls.go`, `common/tls/reality_{server,client,stub}.go`,
  `cmd/sing-box/cmd_generate.go` (проверено grep по GOMODCACHE, 2026-09-11);
- sing-box docs — configuration/shared/tls (REALITY fields), utls «Not Recommended»;
- SagerNet/sing-box issue #3076 (XHTTP — открыт); Leadaxe/sing-box-lx README
  (XHTTP client-only, валидация против Xray-серверов);
- [net4people #490](https://github.com/net4people/bbs/issues/490): отдельные
  версии byte/packet гипотез и сообщения об исключениях; [#546](https://github.com/net4people/bbs/issues/546)
  не является их независимой калибровкой;
- [первичная source-проверка Vision 2026-10-27](../research/reviews/2026-10-27-R02-01-VISION.md);
- общие ссылки прежней редакции на OONI/Reddit/dev.to/Habr без конкретного
  документа не подтверждают ML-детект, gov-CA MITM или рейтинг anti-probing.


---

> Историческая врезка репозитория: локальная source-сверка EVE. Отдельное public-source ACCEPTED выше не стирает её прежнюю область CONFIRMED. Упоминания полевых порогов в этом прежнем срезе не заменяют новые ограничения сопоставимости в начале главы и методическую поправку.

## Датированная capability-поправка — 2026-10-27

Строка таблицы §2 «XTLS Vision — ❌ нет» **исправлена независимой верификацией**
[EVE-2026-10-27-R02-01](../research/reviews/EVE-2026-10-27-R02-01.md):
функциональный код Vision присутствует в dependency closure пина (sing-vmess
v0.2.8-0.20250909…3aed155119a1: vless/client.go:33, vision.go:37, vision_utls.go
под with_utls) и достижим из sing-box VLESS outbound (outbound.go:91). Ошибка
§2 — scope grep'а («по модулю» sing-box вместо closure; единственный .go-хит
«vision» в sing-box — подстрока в «vcs.revision», cmd_version.go:44).

Вывод §2 «безальтернативен VLESS+REALITY без Vision» сохраняется **как решение
политики**, а не как отсутствие возможности: серверные users flow не задают,
two-sided flow contract/native-совместимость не подготовлены, P08-деградации
не опровергнуты. Ранги freeze-устойчивости и anti-probing §3 остаются
[полевое наблюдение]+заявки без matched-blocks корпуса; корроборация порядка
величины 16-20КБ — dept.one 2026-04-01 (15-20КБ для датацентровых диапазонов).
Исторический текст выше сохранён как есть; это поправка, не переписывание.
