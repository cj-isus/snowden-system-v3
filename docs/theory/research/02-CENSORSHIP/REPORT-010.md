# R02-010 — готовый binding должен пережить adapter boundary

26 октября 2026 · **READY_FOR_REVIEW**.
[Sources](SOURCES-010.md) · [паспорт](handoffs/R02-010.md).
Вопрос после 009: какие готовые session-binding свойства реально доступны через
pinned TLS caller, а какие только существуют в библиотеке/спецификации?

## 1. R02-19 — ConnectionState metadata не равно exporter capability

Common sing0.8.13 TLS Conn возвращает стандартный Go ConnectionState. У обычного
core STD client это состояние настоящего TLS conn. Но core1.13.19/b5ebaa1f…
**utlsConnWrapper.ConnectionState** создаёт новый стандартный struct literal,
копируя Version/HandshakeComplete/DidResume/certificate/ALPN и несколько других
полей из MetaCubeX/uTLS1.8.4/cf49b086… state.

У обоих native state types exporter использует закрытый callback `ekm`.
Он не переносится между Go packages через такой literal. Получается состояние,
в котором HandshakeComplete может быть true, но штатного exporter capability нет.
В доступном reference source Go1.26.7 метод ExportKeyingMaterial непосредственно
вызывает `cs.ekm`; у созданного literal это nil. Попытка будущего consumer вызвать
такой метод не является корректной supported операцией; в указанной source
реализации это nil-function call, **не исполненный panic**.

Это конкретный API compatibility gap для **предлагаемого** binding. Поиск
ExportKeyingMaterial в windows/backend и Android corebridge не нашёл действующего
consumer. Поэтому не заявляется текущий пользовательский crash или exploit.
Другие TLS adapters и production binary этим срезом не аттестованы.

### Сильнейшее альтернативное объяснение

Native uTLS сам умеет exporter: его conn.go формирует собственный state с ekm,
а TLS1.3 handshake устанавливает exporter callback. Это **не вывод об отсутствии
exporter в uTLS library**. Теряется доступность в конкретном преобразовании core.
Получение native capability через правильный typed adapter может решить API seam,
но его нет в рассмотренном common Conn contract. Upstream() даёт путь к native
conn; автоматически снимать защитные IO wrappers ради этого нельзя.

Для будущего consumer нужен явный supported/unsupported capability и error path,
соответствующий завершённому, проверенному TLS-сеансу. SNI/сертификат hash/
HandshakeComplete/DidResume не заменяют exporter: они могут совпадать у разных
соединений. Unsupported binding не должен тихо превращаться в public metadata
или пустую строку. Platform owner задаёт lifecycle/epoch; здесь API не менялся.

## 2. Где готовый tls-exporter применим

[RFC9266](https://www.ietf.org/rfc/rfc9266.txt), July2022, задаёт channel-binding
profile с label `EXPORTER-Channel-Binding`, zero-length context, length32. Важны
две границы: это **не secret key для произвольного шифрования**, и binding одного
TLS connection не даёт отдельной уникальности нескольким auth instances поверх
него. RFC ограничивает такое повторное использование. Конкретный потребитель
должен следовать своему auth profile, не просто пересылать/сравнивать bare bytes.

| Участок | Возможность готового binding |
|---|---|
| Настоящий inner TLS C↔X, завершаемый обоими peers | Native TLS exporter — **reference/adapt** после решения adapter/capability seam и auth profile |
| Donor TLS C↔D через ShadowTLS entry E | E не владеет секретом TLS C↔D; нельзя заставить E получить тот же exporter простой настройкой |
| Несколько mux streams/auth exchanges поверх одного carrier TLS | Один binding идентифицирует carrier, а не каждый верхний auth instance. Нужен уже определённый совместимый профиль или auth once с отдельно заданными stream authorization rules |
| Early exporter | Иной secret/interface и weaker replay scope; не подмена обычного exporter |

Для ShadowTLS текущий TLSHandshakeFunc возвращает только error; после handshake
client выдаёт verifiedConn над исходным соединением. Даже наличие exporter у C
не создаёт общего TLS endpoint на E. Это ограничение композиции, не недостаток
HKDF/Go/uTLS. Перейти на настоящий TLS между C и X — другая архитектура termination,
с server identity/profile/migration/camouflage ценой из 005/009.

Свежесть data record уже даёт правильно выбранный terminating TLS protocol.
Добавлять отдельный exporter необязательно для каждого VPN потока. Его применение
разумно, когда требуется определённая channel-bound authentication, и только при
полном совместимом consumer. Новый собственный auth/cipher здесь не проектируется.

## 3. Resumption/0-RTT: actual caller вместо имени опции

Проверены точные constructors и fallback:

- STD/uTLS core NewClient создают Config без ClientSessionCache; в просмотренном
  common/tls package нет её установки. uTLS UClient не добавляет cache сам;
  uLoadSession прекращает загрузку при nil. Go reference loadSession делает то же.
- ShadowTLS default helper создаёт **новый** uTLS Config из subset STD fields;
  ClientSessionCache не копируется. Native support resumption не делает его
  действующим в этой цепочке.
- VLESS TCP tlsDialer завершает HandshakeContext до DialEarlyConn, как установлено 009.
  Ленивый VLESS header и WS `max_early_data` — **не TLS0-RTT**. Поиск нашёл WS fields,
  но не их превращение в TLS early-data API.
- Внешний mutator/другой adapter/другой artifact могли бы изменить Config; их
  отсутствие во всех runtime paths не доказано. Scope — traced constructors/callers.

Поэтому generic предупреждение «у текущего TCP VLESS включён replayable0-RTT»
**отвергнуто в рассмотренном path**. А обещание ускорения resumption сейчас
не обеспечено просто наличием session ticket extension в Chrome preset.
Нужна явная cache/capability/admission работа до измерения выигрыша.

Если resumption будет добавлена, нужны peer/profile/trust epoch partition и expiry,
какие key exchange modes разрешены, разрешён ли early data и где его replay state.
Fresh 1-RTT resumption и replayable early data — разные cases. Regular exporter
и early exporter также разделены в [RFC8446 §7.5](https://www.rfc-editor.org/rfc/rfc8446.html#section-7.5).
Exported bytes не добавляют application exactly-once из 009.

## 4. Выбор reuse и цена

| Механизм | Решение | Цена/предел |
|---|---|---|
| Existing inner TLS data session | **Reuse/adapt** в задаче freshness при корректном identity/key exchange | Server/profile/admission и actual negotiated branch; не free fallback к plain VLESS |
| Native Go/uTLS exporter | **Reference/adapt** через явный capability adapter, если действительно нужен auth profile | Не переписывать crypto; common metadata literal недостаточен; native lifecycle/retirement остаются у платформ |
| RFC9266 bytes как собственный cipher key | **Reject** | Противоречит назначению binding; нет готового безопасного wire protocol |
| Public certificate/SNI/DidResume вместо binding | **Reject** | Не unique per TLS session и не proof possession свежего secret |
| Добавить global session cache всем endpoint/profile режимам | **Defer/reject blanket default** | Отдельные identity/epoch/replay/privacy/clock/compatibility условия; numerical win неизвестен |

Go std/uTLS — BSD-3-Clause; core/sing/ShadowTLS — GPL-3.0-or-later, scope notices
сохранены. Новый dependency, API patch или consumer не установлен. Само получение
32bytes не определяет network/CPU cost: его задаёт выбранный authentication protocol.
Ни latency gain, ни battery gain не измерены.

**Windows:** наличие pinned core не создаёт exporter consumer или работающий cache.
**Android:** source bridge/graph/AAR/native gates раздельны; static source support
не даёт capability на устройстве. R05 нужен explicit lifecycle/identity scope,
R03 — unknown при отсутствии capability; их SDK/планы не менялись.

## 5. Конечная будущая приёмка — NOT_RUN

1. STD versus uTLS wrapper: successful handshake state и доступность native exporter;
   unsupported path возвращает контролируемую ошибку, без nil-function invocation.
2. Native/адаптированный exporters совпадают у правильных peers и различаются у
   разных завершённых connections; wrong peer/profile/epoch/retired callback.
3. Auth profile cardinality: один carrier/multiple streams и запрет недопустимого
   reuse; bare binding не становится новым shared encryption key.
4. Traced default cache=nil; future cache partition и revoked trust; full1RTT,
   resumed1RTT и explicit0RTT разделены, wrong ALPN/name/key modes проверяются отдельно.
5. Artifact/runtime/native/wire доказательство при явном разрешении, не по parser.

Никаких crypto/TLS execution/tests/fixtures/build/native/live. Go1.26.7 прочитан
из установки как reference source, не версия аттестованного EXE/AAR. Remote byte
equality новых cache files не заявлена. R02-19 — authored source/capability finding,
не current production vulnerability/severity.
