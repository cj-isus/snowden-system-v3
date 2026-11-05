# R28-C02 — допуск защищённого пути при неизвестной причине отказа

Дата: 28.10.2026. **READY_FOR_REVIEW; source/spec/manual model, plan-only.**

## 1. Ограниченный ответ и новая дельта

**R28-C02-A, предлагаемый контракт:** неизвестная причина отказа разрешает конечную попытку другого заранее допустимого пути, но не снижение требований к confidentiality, identity, bootstrap disclosure или защите пользовательского трафика. Разрешение попробовать путь, успешная аутентификация peer и право выпустить пользовательские bytes — три разных решения. Для каждого требуется свой набор фактов; causal label `DPI` ни одно из них не заменяет.

Дельта к baseline — общий допуск действий, сохраняющий свойства при stale/unknown, со связанным endpoint/ECH state и разными границами отправки. Это не новый selector, транспорт или product PLAN. R02-013 уже описал ECH bootstrap и неполный endpoint closure; здесь они становятся явными предикатами переходов. R02-014 ограничивает диагностическое использование внешних измерений; новые российские полевые данные не собирались.

**R28-C02-B, узкий source-результат:** TTL/cache и `ECHRejectionError` не заменяют привязку источника к endpoint. В pinned DNS-ECH wrapper видны сохранение старого config после успешного refresh без `ech`, отсутствие `lastTTL` в Clone и повторный dial по прежнему domain destination. Влияние этих веток на runtime не проверено.

**R28-C02-C, opposing source-результат:** у рассмотренного uTLS pin-а default ECH-rejection verifier выбирает DNSName из `config.ServerName` либо `InsecureServerNameToVerify`; это не автоматически outer public name. Для разных origin/public names нельзя присвоить `authenticated_public` только по имени возвращённого типа ошибки. Ниже разобраны caller и границы этого утверждения.

## 2. Текущий Windows caller и предел доказательства

### 2.1 Source identity

`windows/go.mod`: Go **1.26**, sing-box **v1.13.19**, MetaCubeX/uTLS **v1.8.4**, sing **v0.8.13**, miekg/dns **v1.1.72**. Публичный tag ref sing-box указывает на `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`, uTLS — на `cf49b0864331e156689feec6363ef9bf518a5ac7`. Прочитанные immutable source bytes сверены с local cache; область и hashes — [SOURCES](SOURCES.md), private capture indices. Манифест и build recipe не доказывают состав установленного EXE. Локальный Go **1.26.7** использован только как source reference; toolchain/артефакт не исполнялись и не аттестованы.

### 2.2 Один фактический путь: VLESS + WS/CDN

| Граница | Фактический source caller | Что установлено / чего не следует |
|---|---|---|
| Источник кандидата | `windows/corebridge.go:106–208`, `LoadDescriptorSnapshot`, далее enabled VLESS/WS | Прочитана программа; реальные descriptors, profile, vault и endpoint не читались. Наличие поля не доказывает принятие профиля |
| Адрес | `corebridge.go:135–159` → `PreflightResolver.Resolve`; `preflight.go:81–147` | Параллельные plain UDP / provider JSON DoH, затем hostname cache. Результат — один IP, не ServiceMode tuple. Source возвращает `cached`, caller его отбрасывает |
| Bootstrap fallback | `corebridge.go:151–154`, `render.go:570–586` | При ошибке preflight остаётся dial по имени; bootstrap DNS имеет scoped `direct` detour. Это не разрешение protected user traffic через direct |
| TLS / HTTP имя | `render.go:737–763` | IP override меняет server, SNI и WS Host остаются `ch.Hostname`; uTLS `chrome`. **`tls.ech` этот renderer не создаёт**, поэтому его нельзя объявлять real-ECH consumer |
| Библиотечный выбор | `protocol/vless/outbound.go:44–75` → `common/tls/client.go:50–69` | При UTLS.Enabled выбирается `NewUTLSClient`; real ECH требует отдельного non-nil Enabled option |
| Допуск TLS/WS | `transport/v2raywebsocket/client.go:36–42,76–100` → `tls.defaultDialer` | Сначала dial, TLS handshake, затем WS Upgrade. WS `max_early_data` не доказывает TLS0RTT или ECH |
| Нижний resolver | `common/dialer/resolve.go:94–111`; `adapter/dns.go:17–39` | Domain → Lookup → адресный dial. Literal минует Lookup. Exchange возвращает DNS Msg, не согласованный endpoint/ECH план |

Первичные pinned caller links: [VLESS](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go), [TLS dial](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/client.go), [WS](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/transport/v2raywebsocket/client.go), [address lookup](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/resolve.go).

Current renderer может обеспечивать обычный TLS identity contract; отсутствие real ECH не является нарушением обещания, которого профиль не давал. Capability ECH библиотеки, включённая конфигурация, попытка handshake, ECH acceptance, origin identity и protected egress имеют отдельные evidence. `HandshakeComplete`/открытый порт/запущенный engine не объединяют их в PASS.

## 3. Inputs и предикаты переносимого контракта

Все поля ниже — **предложение boundary contract**, не существующий API.

### 3.1 Что поступает от владельцев

- **Intent/policy:** неизменяемый идентификатор разрешённого профиля/канала и действующая policy; роли допустимых peer; confidentiality/identity floor; обещана ли обязательная ECH name privacy; разрешён ли plain bootstrap и для какого конкретного имени/цели. Policy не производится из ошибки сети.
- **Candidate plan:** режим `fixed endpoint + supplied config` либо `Service Binding`; origin/reference identity O; service scheme/port; источник ECH; выбранный ServiceMode/TargetName T, ALPN/port/address set и их provenance/freshness; public name P. Отсутствующее поле остаётся unknown. Не требуем ServiceMode от явно фиксированного deployment, но его совместимость должна задаваться доверенным контрактом.
- **Ownership:** intent/profile/policy revision, network epoch, resolver realm, attempt/connection identity; retained protection barrier и его действительный владелец. Счётчик epoch — платформенный факт, не имя Wi-Fi и не геолокация.
- **Capability observation:** actual TLS backend/version/adapter, real offer vs GREASE vs unknown, доступность negotiation/error evidence; TLS authentication **с ролью и reference identity**, не только наличием сертификата.
- **Outcome:** stage, attempted/unsupported/unavailable/completed/error, timestamps/deadline; actual endpoint/family если наблюдаемы; application/probe outcome; missing fields. Failure attribution хранится отдельно.
- **Budgets:** один абсолютный deadline, пределы DNS work/alias depth/bytes/candidates/retries и объёма retained cache. Числа задаёт владелец по будущим замерам; этот отчёт их не выдумывает.

### 3.2 Три имени и связанные значения

O — исходная разрешённая service identity, T — выбранный DNS endpoint, P — public name из принятого ECHConfig. O/T/P могут совпадать, но их роли не сливаются. DNS alias/TargetName сам по себе не заменяет O при проверке application certificate. Public-name authentication разрешает только ECH recovery control flow, не отправку приложения origin. Основание: [RFC9460 §2.3](https://www.rfc-editor.org/rfc/rfc9460.html#section-2.3), [RFC9525 §6.1.1](https://www.rfc-editor.org/rfc/rfc9525.html#section-6.1.1), [RFC9849 §6.1.7](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.7).

Для Service Binding admission требуется единое решение:

`O + scheme/port + RR source/owner + compatible ServiceMode + T + ALPN/port + addresses + ECHConfig/source/freshness`.

Совместимый RR учитывает AliasMode, mandatory keys, effective TargetName и protocol mapping; первый `ech` в неупорядоченном Answer не является service selector. Для заранее открытого conn нужна согласованность с выбранной записью, а не лишь живой TCP. [RFC9460 §§3,5.1,8](https://www.rfc-editor.org/rfc/rfc9460.html#section-3), [RFC9848 §§4–5](https://www.rfc-editor.org/rfc/rfc9848.html#section-4).

### 3.3 Допуски действий — отдельно от диагноза

Значения обязательных фактов: **T / F / U**. Для опасного действия нужен T у каждого относящегося к нему hard predicate; U не превращается в T и не означает F у независимого свойства.

| Решение | Необходимые факты | Что это разрешает |
|---|---|---|
| `attempt_candidate` | Кандидат разрешён текущей policy; сохранены криптографические floors; возможные bootstrap/CH disclosure заранее разрешены; endpoint/config plan согласован; actual capability поддерживает требуемые свойства; current owner/epoch и остаток budget | Только конечное установление этого кандидата. Предварительная доказанная reachability **не требуется**, иначе возникает круговое условие |
| `send_tunnel_credentials` | Вышеуказанное плюс завершённая защищающая их криптографическая граница и authentication **того peer/роли**, которому разрешены credentials/destination metadata | Только setup данного туннеля. Донор или public-name recovery peer не получает origin credentials по этому допуску |
| `run_owned_validation` | Создан разрешённый туннель; platform barrier допускает только предназначенный owned validation; facts привязаны к этому owner/attempt/network epoch | Ограниченные служебные probes по разрешённому пути; не открытие произвольного user payload |
| `promote_user_path` | Актуальны candidate/policy/identity/required-ECH facts; platform protection и protected-path acceptance подтверждены для того же owner/session/epoch; поздний результат не superseded | Выпуск пользовательского трафика с сохранением floors и явно разрешённых split exceptions |
| `abstain_for_action` | Нужный predicate F/U или исчерпан budget; актуального безопасного перехода нет | Сохранить старый путь только если его собственные protection/trust gates ещё действительны; иначе BLOCKED под retained barrier. Причина отказа может остаться unknown |

Credentials являются чувствительными bytes, даже если это «только заголовок WS». Запрет приложения в rejected outer connection срабатывает **раньше** последующего egress probe. Egress success не исправляет уже раскрытое имя или отправку секрета неправильному peer. Для графа с CDN TLS termination отдельно указать, какие данные разрешены этому терминатору; TLS до CDN не доказывает end-to-end confidentiality от CDN до origin.

`cause_unknown` сам по себе не запрещает `attempt_candidate`. Он запрещает считать снижение floor лечением доказанной DPI-проблемы. Нельзя получить новое право из score, внешнего отчёта, retry count или строки timeout.

Наблюдение допускается получать и через **доказанный контракт конкретного backend**, а не обязательно отдельный bool. Например, если известно, что именно этот immutable real-ECH config передан именно этой TLS connection и backend возвращает nil только после accepted ECH и нужной authentication, успешный возврат может быть свидетельством acceptance. Потеря `ECHAccepted` при projection сама по себе не доказывает, что свойство принципиально невозможно установить. Нужны сохранённые offer/config/caller/connection facts; nil от общего API без этих предпосылок недостаточен.

T относится к **моменту допуска действия**, а не только к старой выборке. Между validation, ожиданием очереди и публикацией платформе нужен её existing ownership/epoch/validity fence; после последнего staged read повторить актуальность перед commit. Простое `if epoch == expected` перед асинхронной работой не доказывает сохранение permit. При смене сети после commit удерживаемый OS protection должен сохранять floors, даже если health устарел; это отдельная native обязанность, не обещанная текстовой индукцией.

## 4. Конечная decision/transition matrix

Это proposed controller contract. Стандарты задают TLS/SVCB нормы; более строгие строки помечены как policy.

| № | Состояние и необходимые факты | Разрешённое действие | Запрет / unknown |
|---|---|---|---|
| M01 | Требуемая capability или observation отсутствует у actual adapter | `unsupported` либо `unobservable_required_property`; выбрать другой заранее разрешённый backend/кандидат | Не рисовать real-ECH success из config field, GREASE или reconstructed false |
| M02 | A/AAAA уже есть, Service Binding resolution ещё не завершён | При допустимом preconnect открыть TCP; после RR проверить consistency | Не отправлять TLS CH до завершения SVCB resolution в ECH-bootstrap процедуре: RFC9848 §5.3 |
| M03 | Fresh согласованный plan, real ECH config поддержан, disclosure policy выполнена | Real-ECH attempt; credentials только после правильного handshake/auth gate | Freshness не доказывает reachability/acceptance/egress |
| M04 | Config/source/адресный plan stale, сменился resolver realm или source не установлен | Бюджетный refresh **связанного plan**; stale данные только hint, если отдельная policy это явно допускает с теми же floors | Не переносить retry config на адреса нового DNS решения; не выключать ECH из-за stale/timeout |
| M05 | Успешное новое DNS решение без поддерживаемого `ech` | `ECH-required`: unsupported/blocked. Без такого обещания — normal-TLS/GREASE candidate только по разрешённой discovery/fallback policy | DNS NODATA, отсутствие совместимого config, protected transport error и отсутствие observation — разные состояния. RFC9849 §6.1 рекомендует GREASE при отсутствии compatible config; это не authenticated server-disable |
| M06 | Защищённый SVCB resolution: auth/SERVFAIL/transport error/timeout | RFC9460 §3.1 рекомендует прекратить достижение этого сервиса; предложенный strict admission требует явного разрешения альтернативы без снижения floor | Не выдавать error за аутентифицированное «ECH отключён». При незащищённом DNS и chain-limit у RFC другие правила; stricter privacy policy может отказать |
| M07 | TCP/TLS timeout, RST или недостаёт transport measurement | Сохранить stage/outcome+unknown; конечная попытка другого допустимого plan для установления/будущего соединения либо backoff | Не выводить DPI, national allowlist или необходимость direct fallback; при неизвестном объёме уже отправленного приложения не replay user operation |
| M08 | Real ECH rejected, public-name authentication/handshake F/U | Закрыть conn; ошибка. Возможен другой **независимо разрешённый** plan в остатке бюджета | Не adopt retry config и не отключать ECH по этому событию |
| M09 | Rejection; достоверно authenticated P, handshake complete, поддерживаемый retry, тот же bootstrap source | Закрыть первый conn до приложения; один bounded retry на **новом** transport, с прежним совместимым address set/plan | Не брать IP из нового DNS query для старого retry. Положительный public-name факт не origin identity |
| M10 | То же доверенное rejection, сервер безопасно отключил ECH в смысле RFC9849 §6.1.6 | RFC допускает новый non-ECH handshake. При `ECH-required` policy — abstain; иначе проверить все прежние floors и заново O | Не использовать rejected outer как application conn; не менять VPN routing на direct |
| M11 | Немедленно второй retry-config либо исчерпан attempt budget | Остановить цепочку, отметить inconsistent recovery/unknown cause | RFC SHOULD NOT для repeated retry не превращать в вечный ban: будущая отдельная операция имеет новый локальный retry budget |
| M12 | GREASE-only / real offer не установлен | Normal-TLS candidate допустим только без ECH-required promise; сохранить `grease`, не `accepted` | GREASE response retry configs не сохраняются; extension не доказывает скрытый SNI |
| M13 | Смена network epoch, адреса или текущего owner; поздний старый success | Старый результат не promote; заново привязать plan/recovery/validation к актуальному контексту | Смена сети не отзывает автоматически доверенный ключ и не доказывает поломку существующего соединения |
| M14 | Origin auth / ECH requirement выполнены, но validation/protection facts U | Разрешить только owned setup/validation; иначе abstain | HandshakeComplete или HTTP/egress success отдельно не открывают user path |
| M15 | Все admission/protection факты T для актуального owner; policy действительна | Promote; дальнейшие переходы снова проходят те же gates | Это не гарантия будущей доступности, не диагноз отсутствия DPI и не аттестация всех пользовательских flows |

Primary: [RFC9849 §§6.1.6–6.2.1](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.6), [RFC9848 §§5–8](https://www.rfc-editor.org/rfc/rfc9848.html#section-5), [RFC9460 §§3.1,5.1](https://www.rfc-editor.org/rfc/rfc9460.html#section-3.1). M01/M04/M07/M13–15 дополняют их proposed platform policy, а не выдаются за текст IETF.

## 5. Cache/handover: что сохранять и что перестаёт быть permit

### 5.1 Области состояния

| Объект | Минимальная связь | Handover / refresh |
|---|---|---|
| Доверенная policy/reference identities | Принятый профиль, authority и revision/validity | Не заменяются DNS/peer error; повторяется current acceptance. Новая сеть сама по себе не меняет trust |
| Service plan | O/scheme/port, RR source/owner/выбранная запись, resolver/security realm, адреса и параметры с собственными сроками | Не склеивать A/AAAA одного пула с HTTPS другого. Refresh создаёт новое согласованное решение |
| ECH retry state | Исходный ECHConfig source, предыдущий plan/address set, attempt, lifetime/scope | С новым DNS решением — свежая операция/bootstrap, не перенос старого retry только по hostname/config_id |
| Reachability / stage failures / egress | Connection/attempt/session + network epoch + endpoint/family + измеренный stage | Старое наблюдение не current permit после handover; можно сохранить исторический факт с его контекстом |
| Живое соединение / streams | Actual owner и retained protection, peer authentication, действительная trust policy | Не убивать только из-за TTL DNS, но не считать старый verification достаточным при изменившемся path/protection scope. Selector switch не обязательно переносит уже открытые streams |

Это не требование глобально очищать все ECH keys при каждой смене Wi-Fi. Source-bound config может остаться допустимым по собственной policy/freshness; reachability и admission на новую сеть перепроверяются отдельно. ECHConfig не получает срок жизни из одного `config_id` или срока сертификата: источник/доставка обязаны задать правила freshness. `config_id` — короткий protocol selector, не глобальная identity.

RFC9849 §6.1.8 требует сохранять связь persisted state с исходным ECHConfig source; §6.1.6 ограничивает совместимость адресов при retry. [Первоисточник](https://www.rfc-editor.org/rfc/rfc9849.html#section-6.1.8). Scope network/resolver/profile в таблице — предложенный способ не потерять эту связь в продукте, не обязательная структура всех кэшей стандарта.

### 5.2 Повторно проверенные source seams, R28-C02-B

1. **Preflight не знает Service Binding.** `preflight.go:66–71,231–245`: TTL семь дней, key только host, запись IP/At. `corebridge.go` отбрасывает `cached`. Эти bytes не доказывают текущую DNS TTL, resolver realm, HTTPS source или ECH compatibility. Успешная обычная TLS проверка может сохранять payload confidentiality/identity, но не отменяет plain DNS disclosure и не обещает ECH privacy.
2. **ECH TTL не равен complete invalidation.** `ech.go:133–179`: при expired config выполняется Exchange; при error/Rcode error функция отказывает до Client. При успешном Answer без нового `ech` **старый nonempty ECHConfigList не очищается**, проверка `len==0` не срабатывает, и Client получает старые bytes. Они не становятся fresh из-за NOERROR. Контрпример к blanket claim «любой refresh failure включает fallback»: transport/Rcode error здесь, наоборот, прекращает попытку; даже reuse старых bytes не отключает ECH и не обходит TLS verification.
3. **Clone не сохраняет TTL.** `ech.go:182–189` копирует lastUpdate/config, но не lastTTL; новая структура получает TTL=0 и следующим fetch пытается обновиться. Это source-факт о методе, а не доказательство вызова Clone в данном active Windows consumer.
4. **Retry и refresh могут конфликтовать.** `client.go:127–133` меняет ECHConfigList на RetryConfigList, но не TTL/source metadata; следующий redial снова проходит fetch. Для TTL=0/expired нового wrapper очередной DNS answer с `ech` может перезаписать только что принятый retry. Если answer без `ech`, остаётся уже установленный retry. Это конечные ручные traces, не воспроизведение. Fresh неизменный plan/TTL или совпадающий refreshed config являются сильными случаями, где конфликта нет.
5. **Domain retry не фиксирует address set.** `defaultDialer` повторяет прежний destination; если он domain, `resolveDialer` снова делает Lookup и вправе получить другой адрес. Если destination literal либо прежний DNS cache возвращает тот же совместимый endpoint, контрпример не возникает. Current rendered IP override и отсутствие real ECH дополнительно ограничивают product applicability.
6. **DNS cache reset не тождественен ECH state reset.** `dns.Router.ResetNetwork:455–460` чистит DNS client cache и resets transports. Проверен сильный противоположный guard: `route.NetworkManager.ResetNetwork:457–483` делает CloseAll и вызывает InterfaceUpdated у outbounds. VLESS `InterfaceUpdated:136–143` закрывает transport/reset mux; но выбранный WS `Client.Close:126–128` — no-op, ECH config этим вызовом не реконструируется. Сам ECHClientConfig хранит lastTTL/config независимо и не содержит network epoch. Это не «reset ничего не делает»: CloseAll/mux/DNS остаются отдельными действующими source guards. Достижение этой функции актуальным Windows native callback и полный lifecycle здесь не доказаны; пересоздание engine/outbound могло бы отдельно очистить объект.

Source: [ECH wrapper](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/ech.go), [retry](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/client.go), [DNS reset](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/dns/router.go).

Дополнительный guard: [NetworkManager reset](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/route/network.go), [VLESS update](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go), [WS Close](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/transport/v2raywebsocket/client.go).

## 6. Сильные контрпримеры и реальные guards

### 6.1 Endpoint mismatch: доступность не следует из правильной криптографии

Ручная модель: O разрешает два пула A/B; transport открыт к A; отдельный HTTPS answer приносит ECHConfig B и target B. A не имеет ключа B и не уполномочен P_B. Fresh TTL, правильный ALPN и обычный открытый TCP не делают этот plan согласованным. M03 отказывает до чувствительного CH/credentials при известном mismatch; unknown binding не превращается в совместимость. Без binding возможен отказ, но это не доказательство раскрытия plaintext credentials: TLS/ECH authentication может корректно остановить соединение.

Сильнейший opposing case: оба пула делят нужный ECH key или полномочия public name и гарантируют recovery; фиксированный deployment может быть согласован. RFC9848 §4 разрешает такую архитектуру. Поэтому общий Service Binding selector не является единственным возможным решением, а «первый ech всегда ломает связь» отвергнуто.

### 6.2 Authenticated rejection: ошибка какого scope?

Для STD source reference Go1.26.7 `verifyServerCertificate` в ECH-rejected branch при nil `EncryptedClientHelloRejectionVerify` использует `DNSName: c.serverName`; `VerifyPeerCertificate`/`VerifyConnection` не вызываются как обычные callbacks. Отдельный rejection callback, если задан, сам несёт verification contract. Его имя не гарантирует корректность policy.

**R28-C02-C, exact uTLS opposing trace** (real ECH — условие, в current renderer его нет):

1. Core `NewUTLSClient:145–169` задаёт `config.ServerName=O` при обычном SNI-enabled профиле. Rejection override не задаётся; `InsecureServerNameToVerify` пуст.
2. В Chrome preset при real ECH `u_parrots.go:2851–2857` SNI extension получает P. **Существующий guard** `u_tls_extensions.go:201–207` не копирует outer P обратно в `config.ServerName`, если real config non-nil. Это сохраняет O для inner authentication.
3. `u_handshake_client.go:386–391,469–491,556–560` использует ECH context и запоминает outer hello name в `c.serverName`. После acceptance `handshake_client_tls13.go:110–120` возвращает inner hello и имя O. То есть inner/outer state реально различены.
4. Но rejection branch `handshake_client.go:1161–1195` строит VerifyOptions с `DNSName=c.config.ServerName` либо explicit `InsecureServerNameToVerify`, **не `c.serverName`**. При O≠P и сертификате, подходящем только P, именно эта проверка O не пройдёт. Это контрпример к предположению, что готовый pin автоматически обеспечивает стандартный public-name recovery.
5. Обратный manual case: доверенный сертификат только O, но не P, удовлетворяет этому name predicate. Из одной последующей `*utls.ECHRejectionError` нельзя вывести проверку P. Для завершения handshake также нужны правильные цепочка/срок/Finished и остальные guards; их обход не утверждается.
6. **Ограничивающий guard core:** automatic retry ищет другой named type `*crypto/tls.ECHRejectionError`. В traced uTLS definition нет As/Unwrap conversion; текущий caller нельзя считать успешно принимающим такой retry. Исправлять только type mapping и считать весь recovery безопасным было бы недостаточно. O=P, сертификат сразу для O и P, явно корректный dedicated rejection verifier либо другой backend — отдельные случаи, на них verdict не переносится.

Primary: [core construction](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/utls_client.go), [SNI guard](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_tls_extensions.go), [uTLS caller](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_handshake_client.go), [verification](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/handshake_client.go), [post-Finished rejection](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/handshake_client_tls13.go).

Это targeted source-совместимость для контрактного reuse, **не security scan, runtime exploit, CVE или доказанная утечка**. Native подтверждение/артефакт отсутствуют. Нельзя преобразовать ошибку в `authenticated_public` без доказанного backend contract.

Не каждое TLS failure позволяет прочитать причину на уровне policy: error conversion может терять тип, alert либо role. Это должно давать unknown, а не эвристику по тексту `certificate`/`ECH`. При невозможности получить достоверный scope безопасный результат — ошибка/recovery другим заранее допустимым plan, а не самодельная certificate проверка в selector.

### 6.3 GREASE и наблюдаемость

Current renderer выбирает Chrome fingerprint с GREASE, а не real config. Кроме того, `utlsConnWrapper.ConnectionState:89–105` не переносит native ECHAccepted. False из reconstructed struct не доказывает rejection; отсутствие offer observation — unknown. Native state доступен через Upstream, но обращение к backend-specific state требует узкой типизированной границы; нельзя обходить её сырым сокетом. M01/M12 предотвращают фиктивный privacy PASS.

### 6.4 Общий DNS/IP/ASN domain и неполные данные

- Один resolver для всех кандидатов может быть общей недоступной зависимостью; смена TCP на QUIC не устраняет необходимость того же bootstrap query. Но SERVFAIL одного имени не доказывает отказ всего resolver: переиспользуемый failure должен совпадать по query/source/security realm/времени и действительно быть prerequisite данного кандидата.
- Два разных транспорта к одному IP не защищают от отказа этого IP. Два адреса одного ASN могут иметь разную достижимость; **один ASN не доказывает общий актуальный блок**.
- Отсутствующий ASN/resolver field — unknown, а не независимость. Внешний dated report ограничен своим measurement interval, AS/access, endpoint/stage/control/instrument; label без lineage не становится локальным permit/ban.
- Для планирования различать: (a) неизменный exact attempt без нового факта — нет немедленной петли; возможен будущий bounded backoff; (b) подтверждённая в нужном scope общая prerequisite failure — переиспользовать факт в той же операции; (c) лишь structural shared ASN — ранжирование/coverage, не universal deny. Истечение срока наблюдения или смена сети снимают актуальность общего failure permit/ban, а не доказывают выздоровление.
- Текущий `render.DependencyComparison` уже различает Shared/Different/Unknown/Coverage; unknown остаётся в знаменателе, RankHint не физическая вероятность. Переиспользовать этот seam, не конструировать новую численную «независимость» из числа протоколов.

Последний пункт проверен по `windows/backend/render/dependency_graph.go:29–53,243–280`; это reuse configuration metadata, не измерение инфраструктуры. Полевой baseline — R02-014 §§5,8; новая причинная атрибуция РФ здесь не заявляется.

### 6.5 Почему unknown не лечится downgrade

Два мира могут дать одинаковый наблюдаемый timeout: W1 — обычная потеря/недоступность сервера; W2 — посредник отбрасывает выбранный handshake. По одному timeout controller не различает W1/W2. Если это наблюдение автоматически снижает identity/encryption/privacy floor, посредник в W2 может вызвать такой переход. Следовательно, preserving policy должна быть независима от этого causal verdict. Это логический counterexample, не оценка частоты DPI.

Инвариант контракта: разрешённый переход либо остаётся внутри исходных floors, либо прекращает отправку; введение нового endpoint/transport не изменяет trust/reference identities без отдельного разрешённого policy update. Индукция по конечной цепочке M01–M15 сохраняет floors **при истинности platform predicates и корректности реализации**. Не доказаны реализация, полнота платформенных фактов и native enforcement. Availability не гарантируется: если подходящих доступных кандидатов нет, честный результат — BLOCKED.

### 6.6 Связанный deeper trace: setup может уже содержать user payload

Actual no-mux TCP ветка `vlessDialer.DialContext:151–170` возвращает `DialEarlyConn` поверх WS transport. sing-vmess **v0.2.8-0.20250909125414-3aed155119a1**, commit `3aed155119a174c9290a4099841049f0cb275e02`: `vless.Conn.Write:137–146` при первом вызове передаёт **весь b** в `WriteRequest`; `vless/protocol.go:124–168` собирает UUID, destination и payload в один downstream Write. Аналогично WriteBuffer добавляет header к исходному buffer.

`EarlyWebsocketConn.writeRequest:157–198` берёт префикс этого content до maxEarlyData; при configured `Sec-WebSocket-Protocol` renderer-а кладёт Base64URL в HTTP header, остаток пишет после Upgrade. `Client.dialContext:76–100` проходит TLS-dial **перед** отправкой Upgrade: этот guard сохраняет TLS ordering; ранняя запись не является plaintext отправкой до handshake и не является TLS0RTT. При `Write(nil)` либо отсутствии payload в префиксе пользовательских bytes там может не быть — обязательная отправка user payload в каждом handshake не утверждается.

**Counterexample к слишком слабому admission:** controller разрешил весь первый Write как «только tunnel setup», считая, что user data появится позднее. В source этот Write уже может нести user payload. Разрешение `send_tunnel_credentials` из §3 не должно давать такой записи более широкие права: любой содержащий user data буфер требует `promote_user_path` либо доказанной ownership служебного probe. До этого user bytes нельзя передать в этот writer лишь потому, что он называется Early/handshake.

Если TLS peer — разрешённый CDN terminator, эти HTTP bytes доступны этому peer после расшифрования TLS; Base64 не добавляет шифрующую границу. Для policy, обещающей скрытие UUID/payload от CDN, необходим отдельный соответствующий inner protection contract. При policy, доверяющей CDN эти данные, сама ранняя отправка не становится дефектом. Правильный сертификат O, WS101 и успешный expected-egress probe могут сосуществовать с видимостью данных CDN: они не доказывают более сильную end-to-end confidentiality. Какая policy задана настоящему deployment, **UNKNOWN**.

Также различать TLS-handshake retry до application bytes и восстановление после возможной отправки приложения. Если неизвестно, принял ли origin предыдущую операцию, новый connection не доказывает её exactly-once. Предложенный path contract не разрешает автоматически replay произвольного пользовательского буфера; это отдельная семантика приложения. Owned idempotent validation не превращает обычный workload в idempotent.

Primary: [VLESS outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go), [VLESS client](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/client.go), [request encoder](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/protocol.go), [WS early writer](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/transport/v2raywebsocket/conn.go). Source bytes сверены с remote pin; это refinement контракта, не установленный native обход platform barrier.

## 7. Reuse / adapt / defer и цена

| Готовое основание | Решение | Что брать и что остаётся |
|---|---|---|
| sing-box v1.13.19, TLS Config/NewSTDClient/NewUTLSClient/ECHClientConfig/NewDialer; GPL-3.0-or-later | **ADAPT**, не drop-in recovery | Сохранять один существующий TLS/dial owner и native crypto. Нужны source-bound plan/attempt state, typed outcomes, backend-specific доказательство auth scope и budget. Не копировать слепо текущие TTL/Clone/retry semantics |
| Go TLS source reference 1.26.7, ECHConfigList / ECHRejectionError / ECHAccepted; BSD-3-Clause | **REUSE primitive; DEFER artifact claim** | Готовые HPKE/ECH/PKI, не собственная криптография. Exact SDK/build и wrapper должны быть сверены платформой. Режим обязательной ECH privacy и разрешение authenticated disable — application policy |
| MetaCubeX/uTLS v1.8.4; BSD-3-Clause | **REUSE fingerprint selectively; ADAPT boundary; DEFER real-ECH recovery** | Native state есть; common wrapper теряет ECHAccepted. Разные error types и описанная public-name ветка требуют отдельного closure. Рекомендация «только добавить errors.As» отклонена |
| miekg/dns v1.1.72, `HTTPS/SVCB`, `SVCBMandatory`, `SVCBPort`, `SVCBECHConfig.ECH`; BSD-3-Clause | **REUSE codec; ADAPT service consumer** | Это уже dependency. Парсер даёт структурированные поля, не полный Service Binding selector/доверие/endpoint plan. Alias depth, mandatory compatibility, source/freshness и bounded response processing остаются caller обязанностью |
| sing-box DNSRouter Exchange/Lookup/ClearCache/ResetNetwork; GPL-3.0-or-later | **REUSE exchanges; ADAPT ownership above API** | Не создавать второй глобальный resolver. DNS Msg и адресный список — разные outputs; не считать ClearCache retirement всех TLS/ECH objects |
| Windows `renderAndPrepareContext`/`WithDialOverrides`, dependency graph, operation/session/probe ownership | **ADAPT existing seams** | Preflight должен передавать provenance/freshness без поднятия IP до authority. Существующие operation/currentProbeReport/Manager gates использовать, не дублировать controller/engine owner. Полная корректность gates не доказана этим срезом |
| Windows `netprobe.EncodeDNSQuery` / `DecodeDNSResponse`, fixed `example.com` A question | **DEFER как общий ECH resolver** | Это bounded protected-probe codec, не generic HTTPS/SVCB consumer. Разные consumers preflight JSON и probe wire нельзя слить по названию DNS |

Прочитанные LICENSE и primary links — [SOURCES](SOURCES.md). Existing dependency не освобождает от notices/source redistribution условий GPL/BSD; whole-product SBOM/legal assessment не проводился. Не предлагаем новую dependency, fork или массовый upgrade.

**Цена состояния:** на active candidate нужен immutable plan, source token, сроки составляющих, ownership/epoch, текущий phase и конечный retry counter; на cache — ограниченное число source-bound entries/bytes с явной eviction policy. Размеры и RAM/energy не измерены. Удаление записи сбрасывает hint, но не даёт права снизить floor.

**Цена запросов:** новая ServiceMode цепочка может потребовать HTTPS lookup, дополнительных Alias/CNAME и A/AAAA TargetName; отдельный retry — новый transport и handshake. Существующий current preflight запускает шесть resolver strategies (четыре UDP, две DoH), поэтому «добавим ещё ECH query бесплатно» неверно. Делить один дедлайн и резервировать остаток для validation; счётчик retry не даёт бесконечных fresh budgets. Public captures — исследовательские fetch, не latency/availability benchmark.

**Цена миграции:** передача structured source/endpoint state вместо одиночного IP, сохранение auth-role evidence через wrapper, distinction unsupported/unknown/failure, синхронизация handover/late results. Native lifecycle, routing barrier, race/cancellation, artifact parity и server compatibility — будущие отдельные gates, не выполненная разработка.

Для retry нужен отдельный immutable attempt view, не неявная перезапись общей конфигурации, которую могут читать другие запросы: рассмотренный ECH fetch защищён собственным mutex только на своём участке, а SetECHConfigList в dialer retry вызывается через embedded API. Это недостаточно для доказательства сквозной concurrency-correctness и source binding. Наличие реальной гонки, её достижимость и runtime effect здесь не установлены; добавлять новый lock вслепую не предлагается.

## 8. Platform owner seam и конечная будущая приёмка

Адресат — Windows owner/DNS/TLS/transport и adaptive consumer; Android получает переносимые inputs/predicates, но не новый AND-статус. Платформа производит и исполняет факты; переносимая policy лишь рекомендует допустимое действие/abstain. Руководитель решает, что интегрировать в общую теорию/план.

Все cases ниже — **NOT_RUN**, текстовые критерии, не исполняемые fixtures:

| Case | Две различаемые стороны и нужный результат |
|---|---|
| A | Real ECH + O certificate + current barrier против GREASE/unknown projection: допуск privacy только в первой доказанной ветке |
| B | Fresh bound RR/endpoint против A↔B pool mismatch, включая общий ключ как сильный положительный control |
| C | Expired ECH + NOERROR без ech против transport/Rcode error; zero-TTL retry и Clone: сохранить provenance/floors, не выдать старое за fresh |
| D | Rejection: валидный P-only cert при O≠P против O-only/wrong-P cert; STD/uTLS actual adapter и backend error type отдельно |
| E | Authenticated disable при обычной policy против ECH-required: новый non-ECH conn разрешается только первой policy; outer никогда не application |
| F | Handover между DNS answer, CH, rejection и retry/validation: старые completion не promote; старый retry не смешан с новым address plan |
| G | O/T/P разные, неправильный origin reference и CDN termination: egress success не исправляет передачу credentials не той роли |
| H | Общий resolver/IP prerequisite против одного лишь общего ASN/unknown metadata: отсутствие ложной независимости и ложного universal ban |
| I | Недостаёт TLS/ECH observation, deadline истёк, source недоступен: explicit unknown/unsupported/abstain и retained protection |
| J | Selector-only switch с уже открытыми streams против worker replacement: новый channel label не аттестует старые flows |
| K | Первый VLESS Write/WriteBuffer с payload против `Write(nil)`; WS header/lateData и различимый owned probe: setup permit не пропускает user bytes раньше platform gate |

Для native/lab случаев потребуется отдельное разрешение. Синтетические имена/peer и настоящие проверки сертификатов, конечный бюджет; реальные пользовательские profiles и сеть в этой работе не используются.

## 9. Открытое и границы завершения

- Actual EXE/SDK provenance, действующий серверный ECH deployment, профиль/ключи, фактический endpoint, traffic и native protection **UNKNOWN / NOT_RUN**. Не исследовались Android actual caller и полная OS lifecycle.
- Не установлен готовый complete service-binding selector в текущем caller; наличие miekg RR parser его не доказывает. Без дополнительных предпосылок fixed endpoint и произвольный multi-CDN Service Binding не взаимозаменяемы.
- Proposed admission предполагает корректные platform producers; на source-only основании нельзя присвоить им runtime T. Ошибочная identity policy, злонамеренный доверенный issuer/admin или неполный validation scope остаются ограничениями модели.
- Независимый paired review Z01/Z02 не входил в этот source-срез. Позднейшее появление материалов не означает их автоматического согласования.
- Никаких tests/build/fixtures/simulation/probes/deploy/security scan, смены policy/bindings/dependencies или product statuses не выполнялось. 

### Итог документационной сверки

28 remote/local source records совпали; 18 local/baseline inputs без hash drift на 09:14:21Z; матрица содержит 15 строк, missing local Markdown links —0. Это статическая provenance/link проверка, **не product test/PASS**. Основной контракт и связанные source/model вопросы конечны; оставшиеся проверки требуют отдельного artifact/native/field назначения.
