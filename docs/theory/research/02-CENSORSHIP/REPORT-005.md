# R02-005 — достаточные условия шифрующей композиции

26 октября 2026 · **READY_FOR_REVIEW**, R02.
[источники](SOURCES-005.md).
R02-10 из 004 остаётся авторским READY. Здесь исследуется следующий вопрос:
**что именно должно быть доказано у готовой замены, чтобы не повторить ошибку
переноса свойства между разными стадиями/узлами?**

## 1. R02-11 — свойство принадлежит участку и стадии

Обозначения: C — клиент, E — ShadowTLS entry, D — TLS donor, X — inner proxy,
T — конечный прикладной адрес. E и X могут находиться на одном VPS. Название
`127.0.0.1` в client-side inner outbound не превращает C→E в локальный участок:
ShadowTLS outbound игнорирует destination при выборе собственного TCP server.
Фактическое перенаправление E→X задаётся серверной композицией.

| Стадия/участок | SS2022 через ShadowTLS | Verified TLS→VLESS через ShadowTLS |
|---|---|---|
| C→D handshake через E | Donor TLS проверяет D; ShadowTLS PSK/HMAC относится к authorized relay | То же; это ещё не inner TLS |
| C→X data через E | SS AEAD шифрует destination/payload до держателя SS key на X | Настоящий TLS шифрует VLESS UUID/destination/payload до X |
| Authentication X | Подтверждение владения SS key; response связан с request salt | Сертификат/цепочка и reference identity X; UUID затем авторизует клиента на X |
| Peer binding | Shared PSK обозначает множество держателей, не уникальный IP/домен | Inner certificate name/key проверяет X независимо от donor name D |
| Replay | SS request salt/timestamp и server cache; область cache требует отдельного решения | Fresh TLS handshake/record state; 0-RTT требует отдельного replay-safe профиля |
| PFS | SS2022 сам не даёт; donor ECDHE не охватывает SS payload после switch | Условно: реальный inner TLS с соответствующим key exchange, а не только TLS boolean |
| C→E camouflage | Donor handshake + TLS-looking framing, внутри SS ciphertext | После switch inner ClientHello также наблюдаем внутри framing |
| X→T | Proxy уже расшифровал поток; дальше plaintext либо прикладной HTTPS | То же; VPN transport не становится end-to-end encryption до T |

**Ручная модель покрытия.** Для каждого attacker-visible edge и каждого этапа
передачи sensitive bytes должно существовать непрерывное шифрующее покрытие
до разрешённой точки termination. Наличие confidentiality у другого edge или
раннего handshake не покрывает данный edge. Суммировать booleans узлов графа
недостаточно. Authentication, PFS и camouflage тоже нельзя получать объединением
несвязанных свойств. Это модель приёмки композиции, не выполненная формальная
верификация и не новый graph SDK.

### Четыре разных имени

1. Адрес TCP entry E, используемый для dial.
2. Donor SNI/verification name D и предложенный ему ALPN.
3. Inner TLS reference identity X и отдельный inner ALPN, если выбран TLS вариант.
4. Destination T внутри encrypted proxy request.

Их равенство не требуется, а различие само по себе не ошибка. Нельзя копировать
donor name в inner certificate validation и считать проверенным свой proxy.
Если X использует donor name, X всё равно обязан предъявить подходящее доверенное
удостоверение; relay к D не даёт X его private key. Нельзя назначать `h2` только
ради внешнего вида, не определив, кто реально обслуживает выбранный protocol.
ALPN handshake и следующие encrypted bytes относятся к разным наблюдателям;
packet shape отдельно, автоматический DPI успех не заявляется.

## 2. Actual pinned caller и codec: SS2022

Core **sing-box1.13.19/b5ebaa1f…** использует разные реализации:

- Outbound → `sing-shadowsocks2 v0.2.1/1f9f2077…` → `CreateMethod` registry →
  `shadowaead_2022.NewMethod` → `DialEarlyConn` → `writeRequest`/`shadowio.Writer`.
- Inbound → **`sing-shadowsocks v0.2.8/e0612494…`**, не client library2 →
  `shadowaead_2022.NewServiceWithPassword` → `newConnection`/`serverConn.writeResponse`.

По одному current client package нельзя принимать server behavior. Ключевые
client method.go и server service.go побайтно сверены с exact tag raw captures.
Оба SessionKey используют один BLAKE3 context и PSK+salt; headers/type/timestamps
и response request-salt связаны на traced однопользовательском AES-GCM пути.
Это evidence протокольной совместимости выбранных branches, **не interoperability
PASS всех режимов/multi-user/UoT/native**.

Конкретные дополнительные границы:

- Registry принимает **`method=none`**: один `type=shadowsocks` не гарантирует cipher.
  В protected composition нужен явный allowlist метода и численных/ключевых inputs.
  Это не текущий обход Windows admission: app сейчас SS вообще не допускает.
- Client требует decoded key ровно 16/32 bytes по методу. Legacy server NewService
  принимает более длинный ключ через собственный Key derivation, а client2 его
  отвергает. Достаточное согласованное условие — canonical exact-length PSK обеим
  сторонам; не копировать все inputs, допустимые только серверу.
- SS key должен иметь собственную роль и достаточную entropy; строка нужной длины
  или повторное использование ShadowTLS password этого не доказывают. Secrets не
  читались и не генерировались.
- Client response проходит AEAD, server type, timestamp и equality request salt.
  Это привязка ответа к текущему запросу; не сертификат конкретной машины и не PFS.
- SS destination/payload шифруются до отправки в ShadowTLS. Существующий AEAD
  codec следует переиспользовать, а не заменять HMAC/XOR/self-made encryption.

### Область TCP/UDP

ShadowTLS core adapter принимает только TCP и отвергает native UDP. SS outbound
по умолчанию имеет отдельный UDP dial path; он не становится совместимым с таким
detour от наличия двух известных transport names. Core содержит UoT/mux ветви,
а server inbound — соответствующие routers: это варианты **адаптации с обоих концов**,
с явными version/framing и scope. Они добавляют TCP head-of-line blocking и общие
очереди; numerical выигрыш/батарея не измерены. Ошибка UDP здесь не доказывает
direct leak: высшая failover policy в этом срезе не исследована.

## 3. R02-12 — replay/PFS не переходят между слоями и экземплярами

Server Service создаёт `replay.NewSimple(60*time.Second)`. Map и mutex принадлежат
этому экземпляру; новый Service создаёт новую map. Single-user branch проверяет
salt, затем AEAD/type/timestamp/variable header перед handler. Timestamp допускает
окно 30s; client response тоже проверяет timestamp и request salt.

**Ручной контрпример переносу гарантии:** два независимых Service A/B с одинаковым
SS key, пустыми caches и валидным request ciphertext R в допустимом времени.
Если противник способен доставить R обоим, оба могут принять его впервые локально.
После restart тот же вопрос возникает у новой cache. Локальная защита не является
глобальным exactly-once для cluster. Это не описание найденной production атаки:
при ShadowTLS потребуется возможность пройти/обойти внешний authorized framing;
пассивный наблюдатель без outer credentials этого автоматически не получает.
Также не доказан повтор прикладного действия: оно зависит от payload, server-first
protocol, destination и состояния T.

Следовательно, при multi-entry rollout следует выбрать область replay/keys:
раздельные endpoint/epoch keys, либо shared replay state/affinity с явными гарантиями
и отказами, либо отказаться от глобального replay обещания. Нельзя из подписи общего
профиля вывести целостность runtime replay state. Приёмка и key lifecycle — R05;
source здесь только задаёт точный вопрос. Pre-auth salt insertion и память invalid
requests требуют отдельного resource/probing анализа, не объявлены exploit.

**PFS witness, без исполнения:** SS session key вычисляется из PSK и видимого salt.
При последующем раскрытии PSK эти inputs снова доступны для derivation. Donor TLS
handshake использует другой секрет и не участвует в указанном KDF SS. Поэтому
замена 004 на SS2022 закрывает отсутствие payload cipher при корректной композиции,
но не даёт заявлять PFS за счёт соседнего TLS1.3 donor handshake.
Спецификация прямо ограничивает это свойство. [SIP022](https://shadowsocks.org/doc/sip022.html).

Внутренний TLS — другой вариант: требуется negotiated modern key exchange,
проверенная identity X и корректный data record path. Для TLS1.3 0-RTT свойства
слабее: нет обычной forward secrecy/replay гарантии. В traced VLESS path DialEarlyConn
означает ленивый proxy header **после завершения inner TLS handshake**, а не TLS0-RTT.
0-RTT не объявлен включённым. [RFC8446 §2.3/§8](https://www.rfc-editor.org/rfc/rfc8446.html#section-2.3).

## 4. Camouflage и граница identity

Verified inner TLS может закрыть конфиденциальность VLESS, но его ClientHello
находится в post-switch ShadowTLS P без cipher этого внешнего слоя. При отсутствии
inner ECH наблюдатель может прочитать второй SNI и предложенный ALPN; повторные
plaintext TLS record headers образуют отдельный fingerprint candidate. «Добавить TLS»
не означает «сохранить прежнюю неотличимость». Это source/model inference, не
проверенный classifier в РФ. ECH совместимость/покрытие не предполагались.

STD/uTLS adapters берут ServerName отдельно, проверяют сертификаты при нормальном
режиме, разрешают explicit ALPN/version options. Точная wire форма/negotiated version
должна подтверждаться отдельно; параметр min_version сам не является wire proof.
Inner identity выбирается из доверенного reference контекста до peer response;
peer-provided name не становится authority. [RFC9525 §6](https://www.rfc-editor.org/rfc/rfc9525.html#section-6).

## 5. Reuse / миграционная цена

| Решение | Выбор | Цена и условие |
|---|---|---|
| Existing SS2022 AES-GCM codec + ShadowTLS | **Adapt** existing pinned core; отдельный inner key, explicit method/network scope | Оба конца, profile/schema/admission и replay/key scope. PFS отсутствует; никакого «всё защищено» по имени SS |
| Existing verified inner TLS + VLESS | **Adapt/reference**, если identity/metadata tradeoff приемлемы | TLS server X/certificate delivery/ALPN/проверка имени, ещё один handshake; inner ClientHello visible |
| Plain VLESS + ShadowTLS | **Reject как confidential candidate**, по 004 | Здесь не исправлялось и не принято руководителем |
| Custom cipher / HMAC как encryption / trust по donor cert | **Reject** | Создаёт неподтверждённую криптографию или неверную identity |
| Глобальное replay/PFS по одному TLS boolean / local cache | **Reject claim** | Нужен явный scope и модель противника |

Go components: GPL-3.0-or-later; исходные notices сохранены. Не импортировались
новые библиотеки. Это выбор reuse design, не blanket license approval всей поставки.

**Windows:** текущее app Outbound не имеет SS `method`; protected allowlist принимает
VLESS/HY2. Core presence не создаёт product support. Для inner TLS renderer должен
задавать настоящий TLS, отдельную identity, совместимый X, version/capability и
безопасный отказ от старого профиля; downgrade к plain VLESS не является recovery.

**Android:** graph subset допускает VLESS/Trojan/HY2 с TLS, SS/ShadowTLS отвергает.
Native artifact/capability, graph/admission и lifecycle — отдельные условия. Нельзя
механически переносить Windows detour exception или считать composition уже supported.

## 6. Будущая конечная проверка — NOT_RUN

1. Canonical method/key/version/network и server/client branches: negative `none`,
   wrong-length/base64, mismatched endpoint/epoch key, expired/revoked profile.
2. SS request/response: tamper/reorder/truncation/wrong request-salt, timestamp,
   per-Service restart и два Service с общим key; отдельно outer-credential model.
3. Inner TLS: wrong name/trust chain, actual TLS version, second SNI/ALPN visibility,
   no insecure/downgrade, lazy VLESS header не подменяет 0-RTT policy.
4. TCP-only/UoT/mux both-end matrix, segmentation/coalescing/partial writes/errors,
   correct queue ownership и protection barrier. Именно codec composition требует
   следующего независимого исследования.
5. Затем bounded canary/wire/native/field checks при отдельной авторизации.

Никаких tests/fixtures/build/crypto execution/simulation/native/live. Свойства RFC,
ручные модели, static branches и product/runtime evidence различаются. Никакого
универсального RF обхода, throughput gain или ACCEPTED этим отчётом не заявлено.
