# R28-Z04 — QUIC/HY2: владельцы ресурсов и матрица совместимости

**Срез 28 октября 2026 года. Статус автора — READY_FOR_REVIEW.**
[Источники, версии и глубина чтения](SOURCES.md) · [Краткая сводка](HANDOFF.md).
Сохранённые материалы: `.artifacts/research/r28-contracts/Z04/`.

## Краткий вывод и предел применения

Число элементов в одной очереди ограничивает только эту очередь. После копирования,
извлечения, сериализации и передачи в историю одни данные могут удерживаться
разными владельцами. Общая граница памяти требует учёта этих владельцев,
активных и завершающихся поколений, размеров выделенных буферов и условий допуска.

Найдена согласованная пара исходников отправителя и контроллеров, исправляющая
названные расхождения обратных вызовов. Она остаётся **кандидатом миграции**:
ветвь PTO, PMTU, ёмкость sampler, общий допуск и совместимость всего ядра
не закрываются одним обновлением. RSS, CPU, время остановки и скорость не измерены.

**Применимость:** оценка ресурсов и подготовка ограниченной миграции ядра.
**Предел:** выбранные исходники, ручные трассы и условная арифметика.
Слова ACCEPT/REJECT в матрице ниже — предложения автора в названной области,
не новые научные решения руководителя и не подтверждение работы продукта.

## 1. Ограниченные утверждения

### R28-Z04-A — граница очереди относится к своему владельцу и единице

`sendQueue=8` считает сериализованные `packetBuffer`; `datagramQueue=32/128` —
кадры и копии QUIC DATAGRAM; HY2 `data=64` — прикладные `udpMessage`.
Defragger хранит группы фрагментов, sent history — метаданные и ссылки на кадры,
а BBR sampler — слоты между первым и последним congestion packet number.
`Pop`, возврат socket-write и ACK/loss — разные события освобождения.

Поэтому count, окно истории или единственная незавершённая подготовка сами
по себе не задают общий предел RAM или времени. Нужны предпосылки о числе
вызывающих, сессий, фрагментов, обработчиков, поколений и ёмкости выделений.

### R28-Z04-B — исправление отправителя и контроллеров рассматривается парой

- Отправитель: SagerNet quic-go
  `f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`,
  ref `v0.61.0-sing-box-mod.6`, MIT.
- Контроллеры и вызывающий код: SagerNet sing-quic
  `4ab2eceaac81e073f53b22ec72ff56aaea89a3d1`,
  go.mod требует quic-go mod.6, GPL-3.0-or-later.

В названных ветвях внедрённый `OnPacketSent` получает flight **до отправки**;
идентичность пакета становится общей для соединения и пространств номеров;
пакеты до установки контроллера отфильтровываются; drop/migration могут вызывать
`OnPacketNeutered`; после ACK/loss передаётся точный `leastUnacked`.
Brutal реализует полный Ex и monotime; начальная настройка pacer использует
фактический начальный размер пакета.

**Предложение автора:** `ADAPT_AS_COUPLED_PAIR`, а не прямая замена без адаптации
или полное исправление ресурсов. Brutal Ex восстанавливает потерянный batch
dispatch; BBR получает согласованные prior flight и названные terminal callbacks.
Но PTO `QueueProbePacket` всё ещё удаляет пакет из engine history и очищает frames
без `OnPacketNeutered`/`notifyLeastUnacked` на этой ветви. Запись sampler может
пережить именно это удаление до следующего соответствующего события или
освобождения соединения. Полный предел sampler этим не доказан.

## 2. Рассмотренный путь и паспорт владельцев

Манифест на дату среза: sing-box1.13.19; quic-go0.59.0-mod.4
`ccd46abedd39c6f8384b1df8daf42008724958f5`; sing-quic pseudo-version
`d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`; sing0.8.13.
При первоначальной сверке 11 выбранных публичных файлов совпали с локальным
module cache. Упаковка Windows задавала windows/amd64 и теги
with_quic/utls/gvisor/clash_api. Это не идентичность выпущенного EXE.

Адресно подтверждённый в R04-010 Windows/HY2-путь без hopping:
DefaultDialer connected UDP → trackedConn при наличии manager → UnbindPacketConn →
скалярный Salamander → quic-go Windows basicConn → Go UDP WSARecv/WSASend.
Unbind не экспортирует SyscallConn/ReadMsgUDP/WriteMsgUDP; Windows `newConn`
возвращает basicConn. Сохраняются границы одной UDP-датаграммы;
Linux UDP_SEGMENT нельзя приписать этому пути по слову GSO.

### 2.1. Паспорт владельцев

U — исходные байты прикладного payload; D — упакованный HY2 DATAGRAM payload;
L≤1452 — выбранный обычный encoded QUIC packet этого Windows-пути.
N_* — максимальное одновременное число соответствующих владельцев.
В произведениях ND величина D должна ограничивать все входящие в этот член
буферы, а не быть размером одного удобного примера. Это условная модель,
не измерение RSS и не универсальный предел для других MTU/путей.

| Стадия / переменная | Единица | Владелец | Локальная граница | Освобождение или перенос |
|---|---|---|---|---|
| Исходный `U_caller` | Байты и исходное выделение | Прикладной caller | MaxUDPSize4096 только на HY2 API | Возврат Write/WritePacket не доказывает немедленный GC буфера caller |
| Фрагменты HY2 `F_frag` | `*udpMessage`, срезы-алиасы | Один Write caller | U≤4096; wire `fragmentTotal` — uint8; проверки C=B-H и F≤255 не найдены | `releaseMessages`; алиас не владеет исходным выделением, время жизни caller существенно |
| Упаковка HY2 `B_pack` | `buf.Buffer` на фрагмент | `writePacket` | Размер пакета и класс allocator | deferred Release после возврата `SendDatagram` |
| Копии QUIC до Add `N_wait·D` | `wire.DatagramFrame.Data` | Каждый одновременный caller SendDatagram | Копирование до Add; Add ждёт при полной queue32 | Ошибка Add или принятие очередью; ожидающие копии не входят в 32 |
| Отправляемые QUIC DATAGRAM | Указатели на кадры и Data | `datagramQueue.sendQueue` | 32 элемента | Pop переносит кадр в packer/history, не освобождает Data |
| Payload и история пакетов | Ссылки на кадры/streams | `sentPacketHistory` | В срезе guards 40k/50k, разные PN spaces | ACK/loss/remove/neuter; history удерживает `Frame.Data` после Pop/write |
| Сериализованная очередь QUIC | Encoded buffer bytes | `sendQueue` | 8 элементов; выбранный Windows non-GSO: Σencoded≤8·1452=11616 | Возврат worker Write освобождает encoded buffer, не DATAGRAM Data |
| Текущая запись, builder, Salamander | Encoded/staging backing | Send worker, packer, Salamander | Одна обслуживаемая скалярная запись и текущий builder в названной трассе; allocator округляет | Возврат/write/defer; kernel completion не подтверждает peer или приложение |
| UDP-буферы ОС | Kernel bytes | Windows Winsock | Запрашиваемые размеры не равны установленным; лимит памяти engine Job не задан | Закрытие socket/kernel completion, не событие Go queue |
| Приёмная очередь QUIC DATAGRAM | Копии `[]byte` | datagramQueue | 128 элементов; копия создаётся до проверки ёмкости | Receive извлекает; новая отброшенная копия лишь становится кандидатом GC |
| Клиентский обработчик после извлечения | Goroutine, `[]byte`, message | Потомок `loopMessages` | Goroutine на сообщение, локальный численный предел не найден | Возврат handler или закрытие; upstream128 уже не ограничивает обработчиков |
| Сервер после извлечения | Message/session goroutine | serverSession | Последовательный цикл до новой сессии, затем goroutine на новую UDP-сессию | udpTimeout/canceler/close; число одновременных сессий требует отдельной предпосылки |
| Готовая очередь HY2 | `udpMessage` и data | Каждый `udpPacketConn` | Channel64 | Read/WaitRead; без копирования headroom WaitRead может передать владение `p.data` |
| Группы defrag `G_defrag` | Запись map, до 255 slots и буферы | LruCache packetID | Age10s; **нет WithSize** | Eviction вызывается лениво при операциях кеша; время само не вызывает освобождение |
| Сборка принятого сообщения | Фрагменты и новый output | `udpDefragger.feed` | Повторная проверка общей длины≤4096 не найдена | Фрагменты освобождаются после копирования; полная ready queue отбрасывает уже выделенный output |
| BBR sampler | RingBuffer слотов connectionState | Внедрённый контроллер | Начально 256, геометрический рост, потолок ёмкости не найден | ACK sample не удаляет немедленно; event-end RemoveUpTo/neuter. Clear/Pop обнуляют slots, но сохраняют выделенную ёмкость |
| Незавершённая подготовка | Один `clientOffer` | HY2 Client | Один pending под mutex | done закрывается до очистки отброшенного соединения; это не предел завершающихся поколений |
| Активное/завершающееся поколение | QUIC, raw socket, sessions | clientQUICConnection / Windows worker | connDone раньше полного закрытия; платформа запрещает замену после неудачного Stop | Graceful/raw close; Job Terminate/Wait/Empty — более сильное процессное событие, но commitBytes=0 |

### 2.2. Условная композиционная арифметика

Для выбранного carrier, без kernel/TLS/stream/pool overhead, логические
удерживаемые payload/backing bytes имеют только параметрическую форму:

`B_live ≤ B_caller + N_wait·D + 32D + B_history + 8L + B_writer + B_builder
        + B_salamander + 128D_rx + N_handler·D_rx
        + Σ_sessions(64·U_ready + B_defrag + B_assembly) + B_sampler + B_retiring`

Условия чтения формулы:

- `B_defrag ≤ Σ_groups(Σparts U_part + slots·metadata)` только при заданных
  числе групп, parts/group и ёмкостях; age10s не задаёт количество групп.
- `B_sampler` зависит от диапазона PN и сохранённой максимальной ёмкости,
  а не только от числа присутствующих записей.
- `B_retiring` требует предела поколений и подтверждения их завершения.
- `B_caller`, N_wait, N_handler, sessions, groups и retiring generations
  не ограничены рассмотренными счётчиками очередей.

Без этих условий формула не даёт конечного общего числа. Контрпример:
queue32 полна, N callers уже скопировали D и ждут Add; одновременно Pop
переносит 32 Data в history, а socket queue8 удерживает другие encoded buffers.
Оба локальных count истинны, но данные принадлежат разным владельцам.

## 3. Матрица отправителя и контроллера

Каждая оценка ниже — ограниченное предложение автора, не новое ACCEPTED.

| Граница | Mod.4 + d838 в исходном срезе | Кандидат f7b7e9c + 4ab2ece | Что именно предлагается |
|---|---|---|---|
| Brutal batch dispatch | Выбран base interface; Ex-метод не получает обычный batch, ackRate не обновляется этим путём | CongestionControlEx и monotime batch | Принять только устранение названного dispatch-расхождения |
| `OnPacketSent` flight | Передаётся post-flight; sampler добавляет L и получает F+2L | Передаётся prior F; sampler хранит F+L | Согласованная пара; не вычитать L у всех legacy callers |
| Идентичность пакета | PN раздельны для Initial/Handshake/App; map контроллера предполагает один порядок | Отдельный congestionPacketNumber между пространствами | Устранение названного расхождения идентичности |
| Установка контроллера | Старые пакеты могут пересечь SetCongestionControl без явного reports-предиката | firstPacketNumber отфильтровывает pre-install | Только эта граница; не весь lifecycle соединения |
| Очистка ACK/loss | Ex event и приблизительный/current lowest app PN; BBR сам оценивает leastUnacked | Отправитель вычисляет точный least по reported spaces после события | Согласование названного удаления sampler |
| Drop/migration | History может исчезнуть без terminal-события контроллера | `OnPacketNeutered`, контроллер сохраняется при migration | Только перечисленные ветви |
| PTO probe retirement | QueueProbePacket DeclareLost и очистка frames без loss batch/neuter/least на этой ветви | Явный terminal callback sampler здесь также не виден | UNKNOWN/NEEDS_ADAPTATION |
| PMTU loss | Individual callback пропущен; Ex list может включать loss | Ex lost list добавляется под reports, вне individual PMTU guard | UNKNOWN; общего исправления PMTU нет |
| App-limited | Пустой packer может чрезмерно отмечать промежутки между записями | Кандидат подавляет отметку в пределах RTT после network-limited | Факт исходника; эффект в поле неизвестен |
| Начальный pacer | Brutal использует 1252; у старого BBR отдельные clock/size seams | Actual `InitialPacketSize`, monotime, profile constructor | Согласование API; rates/overflow отдельно |
| Набор методов | Старый BBR Ex; Brutal не полный Ex | Оба реализуют новый Ex, включая neuter/lost/app-limited | Только парой; новый контроллер не получит новые callbacks от старого отправителя |
| Лицензии и зависимости | MIT + GPL-3+; sing0.8.13 в исходном составе | Корневые лицензии те же; sing0.9 beta4, quic0.61 mod6, x/*/utls меняются | Адаптация с проверкой всего состава, не прямая замена |

### 3.1. Частичные исправления

- **Только Brutal full Ex/monotime поверх старого отправителя:** восстанавливает
  dispatch при точном старом Ex method set, но не prior/post flight, идентичность,
  install, drop/migration или app-limited. DEFER как переходный вариант,
  не принятое полное исправление.
- **Вычесть длину пакета в старом sampler:** может исправить одну post→before
  трассу, но испортить prior/legacy callers; identity/retirement остаются.
  REJECT как универсальное исправление.
- **Только новый отправитель со старым BBR:** нет нового публичного
  `OnPacketNeutered`; собственная estimated cleanup отличается. Нужна
  адаптация типа и смысла; no-op neuter теряет очистку. REJECT как полное решение.
- **Новая пара без PTO terminal:** кандидат только для dispatch/prior-flight,
  не для полной ресурсной гарантии или всего lifecycle.

## 4. Возражения и границы допуска

### 4.1. Существующие защитные ограничения

В срезе найдены raw receive queue1024, per-connection unprocessed256,
accept queue32, retry/reject queues, anti-amplification3×, tracked packets
40k/50k, DATAGRAM32/128 и HY2 ready64. Они ограничивают свои стадии.
quic-go API предоставляет ранние `GetConfigForClient`/ConnContext;
handshake idle timeout завершает неактивные попытки при работе scheduler/timer.

Windows имеет одного изолированного владельца worker: неудачный Stop
блокирует замену. Job count8 и Terminate/Wait/Empty поддерживают завершение.
Лимит памяти в рассмотренном engine Job **не задан** (`commitBytes=0`).

### 4.2. Почему это не полный допуск

Рассмотренный HY2 Service задаёт `MaxIncomingStreams=1<<60`, не задаёт
`GetConfigForClient`, а qtls Listen не передаёт ConnContext. После Accept
запускается `handleConnection` на соединение. Прикладная HTTP/3 auth идёт
после транспортного handshake; неверный пароль получает masquerade response,
а не отказ до выделения всех handshake-ресурсов. Новая sessionID после auth
создаёт udpPacketConn, запись map и handler; локальный предел map не найден.

Accept queue32 ограничивает завершённые, ещё не принятые соединения,
а не все активные и уже исполняющиеся handshake. Anti-amplification ограничивает
отправленные байты до проверки адреса, не CPU, maps или TLS state.
Timeout ограничивает длительность условно на доставку таймера, не параллелизм.
Для общей гарантии нужен замкнутый учёт допуска всех стадий и памяти;
одна возможная реализация — согласованные бюджеты handshake/connection/session.
Новая БД или отдельный механизм на каждую стадию из этого не следуют.

Адресная проверка, auth map, timeouts, сбросы очередей и Job reap существенно
снижают воздействие. Здесь не установлены реальный удалённый DoS,
достижимый злонамеренный профиль, частота отказов или эффект RSS.

## 5. Применение, переиспользование и цена

Рассматривать f7b7e9c+4ab2ece как одну единицу совместимости после проверки
API, сборки и лицензий. Первая цель — смысл событий prior-flight,
идентичность между PN spaces, install, exact least-unacked, drop/migration
и Brutal Ex dispatch. PTO/PMTU остаются открытыми. Обещание исправить всю
память или ускорить VPN этим результатом не обосновано.

Цена переноса:

- go.mod меняет quic-go mod.4→mod.6, sing0.8.13→0.9.0-beta.4,
  utls1.8.4→1.8.7 и x/*, конструкторы, обёртки и API потребителей;
- GPL-3.0-or-later и MIT двух корней не заменяют проверку полного состава
  бинарного файла и обязательств по соответствующим исходникам;
- исправление callback не ограничивает сохранённую ёмкость BBR sampler
  и число владельцев; наблюдения должны сохранять единицу, владельца и
  фактическое событие освобождения;
- скалярный Windows/Salamander-путь остаётся условием совместимости.
  Исправление sender/controller само не включает GSO и не меняет wire framing.

Маршрут: PC-06/07 [плана ПК](../../../../../planning/knowledge-application/PC-PLAN.md)
и ANDR-02/07 [плана Android](../../../../../planning/knowledge-application/ANDROID-PLAN.md).
Основное изложение — [учёт QUIC и пределы](../../../../performance/QUIC-ACCOUNTING-AND-LIMITS.md).
Платформенные владельцы сохраняют жизненный цикл и собственную приёмку;
второй runtime или массовое обновление зависимостей не требуется по одному отчёту.

## 6. Будущая приёмка — NOT_RUN

1. **Интерфейс и сборка:** точная пара с production tags; нет скрытого no-op
   для neuter/lost/app-limited; полный состав зависимостей и лицензий.
2. **Семантика:** F=0/F>0, STREAM/DATAGRAM, ACK-only, ACK+PING, PMTU/PATH,
   пересечения PN Initial/Handshake/App и установка при уже существующих пакетах.
3. **Завершение:** ACK, threshold/time loss, key drop, Retry, PTO, migration,
   замена контроллера и close; сравнить history, frame backing, sampler
   present и сохранённую ёмкость slots.
4. **Учёт владельцев:** полная queue с N ожидающими SendDatagram;
   Add→Pop→Write→ACK/loss; RX128→handlers→session64; группы defrag255/age10,
   перекрытие сборки, активные и завершающиеся поколения.
5. **Допуск:** одновременные handshake, принятые connection, session,
   per-user attempts и CPU/bytes; неверная auth, медленный клиент и timeout.
6. **Платформа:** заблокированный graceful close, ошибка write, terminate/
   Wait/Empty и память; без прямого fallback после ошибки.
7. **Сравнение:** полезное завершение и goodput, хвосты задержек, классы потерь,
   RSS/CPU и fairness при заданной зарегистрированной нагрузке. Это отдельное
   разрешённое выполнение, а не результат чтения исходников.

Тесты, сборка, симуляции, benchmarks, native/сеть/endpoint и изменения кода
в этом срезе не выполнялись. Эмпирическая и продуктовая приёмка остаётся неизвестной.

<a id="r28-z04--quichy2-все-holders-и-semantic-matrix-готового-исправления"></a>
<a id="1-ограниченный-ответ"></a>
<a id="r28-z04-a--queuecallback-bounds-теряют-силу-при-смене-owner-и-unit"></a>
<a id="r28-z04-b--готовое-исправление-действительно-парное-но-scoped"></a>
<a id="2-actual-path-и-holder-passport"></a>
<a id="21-паспорт-holders"></a>
<a id="3-sendercontroller-semantic-matrix"></a>
<a id="31-partial-fix-decisions"></a>
<a id="4-opposing-cases-and-admission-guards"></a>
<a id="41-strong-existing-guards"></a>
<a id="42-why-these-do-not-close-admission"></a>
<a id="5-recommendation--reuse--cost"></a>
<a id="6-future-finite-acceptance--not_run"></a>
