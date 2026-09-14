# Phase B1 — исследование REALITY и XHTTP для канала B

> Дата: 2026-09-11. Статус: **research done (V2-031); клиентская сторона реализована
> и live-verified (V2-032)** — дескриптор schema 2, рендер, строгая валидация,
> probe-семантика готовы; канал B появится в селекторе сразу после заполнения
> дескриптора и секретов. Осталось: VPS-B (B1-VPS-1, owner) → сервер → секреты → live-verify.
> Серверная реализация — после procurement VPS-B (за owner). Всё, что помечено
> «доказано по коду», проверено по исходникам pinned-ядра
> `github.com/sagernet/sing-box@v1.13.19` (go.mod windows/), а не по документации.

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
| XTLS Vision (`flow: xtls-rprx-vision`) | ❌ нет | `grep -ri vision` по модулю — только vcs.revision в version-команде |
| XHTTP transport | ❌ нет | нет `type: xhttp` в transport-фабрике; upstream SagerNet/sing-box issue **#3076 «is xHTTP transport support in the core planned?» — открыт** (подтверждён поиском 2026-09) |
| `sing-box generate reality-keypair` | ❌ нет в 1.13.19 | `cmd/sing-box/cmd_generate.go` — только `rand`/`uuid` (ech/tls/vapid/wireguard — отдельные файлы). Ключи X25519, base64.RawURLEncoding — генерируются офлайн-сниппетом или утилитой Xray |

Ключевой факт про XHTTP: он существует в **форках**. Наш анализ `sing-box-lx`
(README, 2026-09): XHTTP там — *client transport only*, «live-validated against
real Xray servers»; серверная сторона — это Xray-core. То есть «XHTTP-канал» для
нас означал бы либо Xray на сервере + форк-ядро в клиенте (смена pinned-ядра =
новый supply-chain контракт, против PLAN §1.6 и V2-015), либо ждать upstream.
Это отдельное большое решение, не входящее в B1.

**Вывод: в рамках контракта «pinned = server» выбор безальтернативен —
VLESS+TCP+REALITY (без Vision).** Он же закрыл бы B3 (anti-probing, §5).

## 3. Модель угрозы 2026 (что изменилось в РФ-сетях)

1. **TSPU freeze-метод** [полевое наблюдение, net4people/bbs#490, июн 2025 —
   обновления до конца 2025]: TCP-соединение по TLS 1.3 к IP иностранного
   датацентра замораживается после ~16–20 КБ (≈25 пакетов) внутри **одного**
   TCP-соединения — без RST, клиент просто ждёт таймаут. TLS 1.2 тоже
   фильтруется. Обход: SNI-whitelist (белый SNI в ClientHello спасает),
   раздача по коротким соединениям (дорого). CIDR-whitelist по подсетям
   назначения (upd4) — самый тяжёлый случай: требуется промежуточный узел
   из белого списка.
   Значение для нас: канал A имеет CDN-dest/SNI — в зоне риска по этой атаке
   меньше; HY2 (QUIC к «сырому» IP датацентра) — первый кандидат на деградацию;
   канал B с raw VPS-IP и TLS — обязан иметь белый SNI (это и есть REALITY-модель).
2. **Статус REALITY**: сообщения о блокировках VLESS(+REALITY) в отдельных сетях
   с конца 2025 [полевое наблюдение; Reddit/r/VPN, OONI-обзоры 2026]; технические
   слабости REALITY известны: ML-статистика (VPN-туннель = долгоживущая
   двунаправленная сессия, не похожая на сёрфинг «донора»), репутация IP
   (много клиентов с одним SNI на VPS-IP), MITM при наличии government-CA в ОС
   клиента. Итого: REALITY — лидер по anti-probing, но не серебряная пуля;
   разнородность каналов (TCP+REALITY / QUIC+obfs / WS+CDN) остаётся главной
   защитой (THEORY §3.6).
3. **uTLS** [документировано, sing-box docs 1.13]: upstream официально помечает
   uTLS «Not Recommended» (повторявшиеся fingerprint-уязвимости, качество
   библиотеки). Для REALITY-клиента в sing-box uTLS обязателен (доказано по
   коду, §2). Наша позиция: тег уже в сборке; fingerprint — дефолтный chrome;
   ограничение фиксируем честно, миграцию «наивпрокси-подобных» решений не
   затеваем.
4. **VISION недоступен** — значит защита против «анализа lengths/timing внутри
   TLS-сессии» у нас не улучшается относительно обычного TLS-транспорта;
   это принято как граница канала B (см. §6).

## 4. Сравнение кандидатов

| Критерий | Канал A: VLESS+WS+TLS (через CF) | **Канал B: VLESS+TCP+REALITY** | XHTTP (за форками/Xray) |
|---|---|---|---|
| Ядро клиента (pinned 1.13.19) | ✅ сегодня live-verified | ✅ встроено (см. §2) | ❌ нет; нужен форк в клиенте |
| Ядро сервера | sing-box 1.13.19 | sing-box 1.13.19 | Xray-core (или форк sing-box-lx — client-only) |
| Новый ASN/country (failure domain) | — | ✅ да (суть B1) | ✅ да |
| Скрытие IP origin | за CF edge (TCP) | ❌ IP виден, но SNI белый; редирект зонда на донора | как у A (за CDN) или raw |
| Anti-active-probing | среднее: неавторизованный WS → 400 от sing-box [THEORY §5, палево] | **лучшее**: неавторизованный TLS → прозрачный прокси к реальному донору [документировано XTLS] | среднее+ (зависит от CDN) |
| Устойчивость к freeze-методу (§3.1) | высокая (CDN-dest/SNI) | средняя-высокая: белый SNI, но raw DC-IP в CDN-чужом ASN | высокая (CDN) |
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
- `flow` НЕ задаётся (Vision в ядре нет; пустой flow = обычный VLESS поверх REALITY);
- выбор донора (owner): популярный сайт, TLS 1.3 + ALPN h2, X25519 key exchange,
  не наш домен/зона, не CDN нашего VPS, желательно из «белого» SNI-набора цензора
  (проверка донора с VPS-B: `openssl s_client -connect <donor>:443 -tls1_3 -alpn h2`);
- секреты: `vless-uuid-b`, `reality-private-key-b` (только сервер),
  `reality-public-key-b`, `reality-short-id-b` → файлы в `secrets/channels/` +
  DPAPI-слоты; значения не в git/чат/логи; генерация ключей — X25519 + base64.RawURLEncoding
  (`sing-box generate reality-keypair` в 1.13.19 отсутствует — см. §2;
  формат-гвард рендера поймает неверную кодировку до старта движка);
- дескриптор: **schema 2** — `transport: "tcp-reality"` + явные роли ссылок
  (`uuid_ref`, `reality_public_key_ref`, `reality_short_id_ref`, все ∈
  `credential_refs`, ровно три разных); schema 1 продолжает приниматься;
  **реализовано (V2-032)** — см. windows/backend/render/render.go;
- validator/render: **реализовано (V2-032)** — validateRealityTLS (полный
  reality-блок обязателен: enabled/public_key/short_id), запрет любого
  xtls-flow на защищённом outbound (Vision отсутствует в pinned-ядре),
  формат-гварды рендера: public key = 43-симв. base64url (X25519,
  base64.RawURLEncoding), short_id = hex ≤16 символов; insecure по-прежнему
  запрещён (пиннинг не нужен: сертификат донора — публичный);
- probe: expected_egress = IP VPS-B; UDP-gate не нужен; TLS-preflight должен
  распознавать REALITY-хендшейк (не требовать наш сертификат);
- `route.final` на VPS-B = `direct` (урок V2-002), nft/ufw: 22/tcp, 443/tcp.

Anti-probing бонус: неавторизованный TLS к VPS-B уезжает на донора — закрывает
старый gap «400 от sing-box» (THEORY §5, B3) для канала B.

## 6. Риски и честные границы

- IP VPS-B остаётся видимым и блокируемым по CIDR (§3.1 upd4). REALITY не лечит
  L2-блокировку — лечит другой ASN и смена IP. Продукт по-прежнему не обещает
  работу при allowlist-режиме (THEORY L6/L7).
- Без Vision статистический детект (долгоживущие сессии) остаётся возможен;
  mitigation — разнообразие каналов, не «магический транспорт».
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
VPS-A (203.0.113.10) как третий канал `channel-a-reality`. Что это даёт и чего
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
short_id 0123456c; inbound vless-reality-in на 0.0.0.0:8443/TCP (порт выбран
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
probe через outbound/vless(reality) → egress=203.0.113.10; curl без ключей на
203.0.113.10:8443 возвращает контент донора.

## 8. Источники

- pinned-ядро `sagernet/sing-box@v1.13.19`: `option/tls.go`, `common/tls/reality_{server,client,stub}.go`,
  `cmd/sing-box/cmd_generate.go` (проверено grep по GOMODCACHE, 2026-09-11);
- sing-box docs — configuration/shared/tls (REALITY fields), utls «Not Recommended»;
- SagerNet/sing-box issue #3076 (XHTTP — открыт); Leadaxe/sing-box-lx README
  (XHTTP client-only, валидация против Xray-серверов);
- net4people/bbs#490 (TSPU freeze ~16КБ/25 пакетов, SNI- и CIDR-whitelist);
- полевые обзоры 2025–2026: OONI/zona.media/dev.to (детект REALITY: статистика,
  IP-репутация, gov-CA MITM); valebyte.com, habr (практика выбора профилей).
