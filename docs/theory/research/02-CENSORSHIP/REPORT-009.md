# R02-009 — граница повтора: framing, inner authority и target

26 октября2026 · **READY_FOR_REVIEW**.
[Sources](SOURCES-009.md) · [паспорт передачи](handoffs/R02-009.md).
Новый вопрос: какие сильные альтернативы R02-15 действительно блокируют вывод,
и какое событие следует считать результатом? Сам HRR finding не считается новым.
R02-10 отдельно принят руководителем только в [source/model scope](../reviews/2H-HB01-R02-10.md).

## 1. События, которые нельзя называть одним «replay succeeded»

| Событие | Точный смысл | Чего ещё не доказывает |
|---|---|---|
| F | Outer Service распознал data HMAC и передал cached bytes своему handler | Что routing доставит их нужному inner decoder |
| I | Inner protocol распарсил запрос и допустил его по своим credential/session условиям | Что target доступен и запрос дошёл до приложения |
| N | Создано/принято сетевое соединение к target | Что повторено прикладное действие |
| A | Target приложение приняло конкретную операцию | Что её бизнес-эффект уникален/повторён; это определяется приложением |

Цепочка F→I→N→A требует отдельных условий на каждом переходе. Она не является
логической импликацией из одного HMAC match. Возможность N имеет ресурсную цену
сама по себе; она не заменяет доказательства A. В этом срезе нет runtime events.

## 2. Strongest alternatives к прежнему R02-15

Baseline: наблюдённая legitimate HRR-сессия, остающийся PSK, повторимый первый
ServerHello-HRR и доступность прежнего signed ClientHello/first data record;
нет дополнительного внешнего replay gate. Эти условия из007 не доказываются заново.
Текущий source sing-shadowtls0.2.1/af3279d7… перечитан адресно.

| Альтернатива | Что проверено | Итог и граница |
|---|---|---|
| Entry требует настоящий donor TLS Finished перед switch | copyByFrameUntilHMACMatches распознаёт первый data tag; service выставляет handshakeFinished и закрывает donor | В traced branch переменная обозначает HMAC switch. Отдельной Finished verification entry здесь нет; это не позволяет переносить свойство donor TLS transcript на outer data state |
| Donor close обязательно делает весь Group ошибкой | service нормализует closed/cancel/timeout при handshakeFinished; exceptions включает EOF/net.ErrClosed; Group возвращает aggregate errors и не отменяет parent ctx как proof freshness | Допустима success-ветка к handler. Parent cancellation, другой error/порядок выполнения способны её прервать; schedule/source witness не исполнен |
| CachedConn требует новый обмен и не отдаёт прежний payload | matched record снимает header/tag; NewCachedConn.Read сначала отдаёт cache | Дополнительной freshness проверки в этом seam нет. Полнота inner header и будущих bytes остаётся условием I |
| Авторизованный user context свежий по определению | user получается из проверки статического CH tag/PSK и кладётся в context | Membership/auth identity не доказывает freshness отдельного запроса |
| Любой inner encrypted protocol автоматически остановит повтор | SS и TLS имеют разные gates ниже | Это сильный реальный контрпример переходу F→I, но не универсальное свойство слова «encrypted» |
| Server actual deployment отличается от cache/pin | Реальная конфигурация, binary и внешние gates неизвестны | Это существенный unknown. Из source-model нельзя объявить deployed exploit |

**Уточнённый результат:** выбранные lifecycle/CachedConn альтернативы не дают
универсального source-доказательства отказа F. Но сохраняются packet completeness,
donor/PSK/gate/routing/schedule предпосылки. Никакой exploitation/достижимости A
этой проверкой не установлено. Факт R02-15 остаётся кандидатом для независимой рецензии.

## 3. R02-18 — inner gate определяет, где заканчивается outer repeat

### Plain no-flow VLESS

Pinned sing-vmess3aed1551… Service.NewConnection: ReadRequest → UUID lookup →
flow equality → TCP/UDP/mux handler. В рассмотренной TCP/no-flow ветке нет
request timestamp/nonce/replay-cache. Полный ранее допустимый VLESS header с ещё
разрешённым UUID не получает самостоятельную session freshness от этой ветки.

Это условный переход к I при корректном decoder/routing и прежних credentials,
а не новый claim о confidentiality и не знание актуального server config.
User revocation/flow change/TLS перед VLESS/другая admission policy могут исключить I.

### SS2022: содержимое и anti-replay — разные проверки

Actual core server использует sing-shadowsocks0.2.8/e0612494… Single-user Service
создаёт собственный salt filter60s. Request проверяет salt, AEAD, direction/type,
timestamp±30s и variable header до handler. Поэтому если прежний request уже был
виден **этому же Service/cache**, повтор может закончиться до I даже при F.

Три различных случая:

1. **Same Service, seen salt:** salt gate отвергает повтор; не нужно выводить успех
   I из совпавшего outer HMAC.
2. **Новая cache/restart/другой Service, тот же inner key, допустимый timestamp:**
   local seen-salt evidence отсутствует; прежний корректный ciphertext снова
   удовлетворяет криптографическим проверкам, если вход/формат полны. Это условный
   source witness к I, не доказанная production атака.
3. **Timestamp вне окна/key изменён/первый request не дошёл:** результаты отличаются.
   В последнем случае первое принятие не является duplicate application effect.

В005 outer credential access был отдельной предпосылкой delivery к SS decoder.
Условная HRR-модель007 — один возможный способ изменить эту предпосылку при
наблюдённом transcript без знания PSK; это не автоматическая возможность любого
пассивного наблюдателя. Local filter не становится cluster-wide или durable от
наличия профиля с подписью.

### Настоящий inner TLS перед VLESS

Core VLESS inbound при tlsConfig сначала выполняет TLS server handshake и лишь
затем вызывает VLESS Service. Outbound tlsDialer завершает TLS handshake до
DialEarlyConn. В этом path **Early означает lazy VLESS header, не TLS0-RTT**.

Для fresh authenticated inner TLS1.3 1-RTT с современным key exchange и корректной
identity запись прежнего потока не получает прежний traffic key в новой сессии.
Возможное принятие старого ClientHello начинает TLS handshake, но не доказывает
I для VLESS и тем более A. Неподтверждённые resumption/0-RTT/слабая identity
меняют условия; их нельзя молча добавить к traced caller. В следующем срезе
нужна отдельная проверка actual cache/exporter/resumption seams.
[RFC8446 §2.2–2.3/§8](https://www.rfc-editor.org/rfc/rfc8446.html#section-2.2).

## 4. Fresh session не означает единственную прикладную операцию

Даже когда TLS исключает принятие старого ciphertext как нового protected data,
легитимный клиент может после потери ответа заново отправить ту же операцию в
другой свежей сессии. Сетевой VPN не знает, приняло ли приложение первую операцию.
Без application receipt/idempotency/dedup такой факт нельзя восстановить из
transport handshake/probe. Это ручной distributed-delivery контрпример обещанию
«fresh session ⇒ exactly-once effect», не исследование поведения приложения владельца.

Следствие для R03/R06: transport retry/reconnect и retry прикладного действия
нельзя учитывать как одно и то же. Для privacy/fail-closed не требуется читать
весь payload ради попытки вывести бизнес-семантику. Target operation control
остаётся у приложения, не у adaptive VPN advisor.

## 5. Что брать готовым и какова цена

| Механизм | Выбор и exact reuse | Цена/граница |
|---|---|---|
| Existing verified inner TLS | **Adapt** core1.13.19 TLS/VLESS caller, real TLS identity и actual record session | Both-end certificate/profile/admission migration; handshake/CPU; nested ClientHello visibility из005. Не обещать exporter или resumption по одному имени TLS |
| Existing SS2022 salt filter/codec | **Reuse с узким scope**, server0.2.8/NewService и client2 0.2.1 | Cache/clock/key scope; новый Service не наследует old seen-salt map. Нет PFS и глобального at-most-once |
| VLESS static UUID как replay barrier | **Reject это свойство** для traced no-flow branch | Membership check полезен, freshness им не заменяется |
| Включить consumer direct/raw fallback при старом/битом session proof | **Reject** | Ошибка проверки не даёт новой authority |
| Application exactly-once через VPN handshake | **Reject claim** | Нужен application protocol; VPN не должен притворяться его владельцем |

Go core/ShadowTLS/sing-vmess/SS implementation scopes GPL-3.0-or-later; notices
уже сохранены в предыдущих пакетах, новые файлы source с provenance здесь.
Подпись профиля, установленный binary и фактический network path — разные gates.
Никакой product patch/новая dependency/crypto implementation не создавались.

**Windows:** исследуется referenced composition, не выбранный активный профиль.
**Android:** current graph ShadowTLS отвергает; наличие того же core pin не доказывает
AAR/native и не превращает F в реальное Android событие. Acceptance owners не менялись.

## 6. Конечные future cases — NOT_RUN

1. Отдельные receipts для F/I/N/A, без raw secrets/payload в diagnostic export.
2. Parent cancel/closed donor/другая error classification и CachedConn boundary.
3. VLESS valid/invalid UUID/flow, revoked user, truncated header и routing deny.
4. SS same seen-salt cache, restart/second Service, timestamp boundary/key change,
   bytes not delivered originally; отдельные false replay/application cases.
5. True inner TLS1.3 handshake и replayed old stream; negotiated identity/key exchange;
   любые future0-RTT cases отдельно от текущего lazy-header caller.

Raw public module zip timed out; exact GitHub web acquisition failed. Повторной
remote byte equality service.go здесь нет. Используются dated actual module-cache
sources, manifests и прежние pins; эта граница не выдаётся за новый upstream audit.
No tests/fixtures/build/crypto/TLS execution/native/live. R02-18 — source/model
разделение predicates, не exploitability/severity judgment.
