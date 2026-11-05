# R28-Z04 — источники и глубина проверки

Чтение исходного среза — **28 октября 2026 года**, только исходники и документы.
`FULL_TEXT_READ` означает прочитанный целиком небольшой файл/документ;
`SECTIONS_READ` — только названные symbols/sections. Сохранение файла само
по себе не означает его прочтение. Сборка, native и проверки сети — **NOT_RUN**.

## Краткий вывод

Реестр позволяет проверить происхождение локальных границ очередей,
вызывающих путей и кандидата парной миграции. Хеши подтверждают идентичность
выбранных исходников; они не доказывают совместимость сборки, полноту всего
модуля, предел памяти или поведение выпущенного VPN.

## 1. Идентичность исходников на дату среза

| ID | Версия / источник | Глубина и используемая область |
|---|---|---|
| C1 | Snowden `windows/go.mod`, `go.sum`, `tools/package.ps1` | SECTIONS_READ: sing-box1.13.19, quic-go0.59.0-mod.4, sing-quic pseudo d83826c, sing0.8.13; windows/amd64 production tags |
| C2 | quic-go `ccd46abedd39c6f8384b1df8daf42008724958f5` | Пять выбранных публичных raw-файлов совпали с локальным кешем; локальные файлы сохранены. Tag/version взяты из манифеста, commit сопоставлен по опубликованному основанию и raw bytes. Это не идентичность EXE |
| C3 | sing-quic `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` | Шесть выбранных raw-файлов совпали с кешем; сохранены выбранные исходники. Pseudo-version содержит commit/time |
| C4 | Локальный Go1.26.7 reference | Унаследован из R04-010; использован для скалярных Windows socket/cancellation границ. Версию toolchain артефакта не подтверждает |
| C5 | Windows worker | SECTIONS_READ: `corebridge.go`, worker_process/host/spawn/protocol, engine. Владелец, reap и отсутствие заданного Job memory limit в этом срезе |

`fresh-source-equality.json` фиксирует 11 файлов с `remote_equals_local=true`.
`local-inputs.json` содержит 66 выбранных файлов; хеши до/после каждой копии
совпали. Копии последовательны: атомарный снимок дерева и аудит всего модуля
не установлены. Исходная загрузка публичных файлов использовала обычную TLS-проверку.

## 2. Кандидат согласованной пары

### 2.1. Контроллеры и вызывающий код

SagerNet sing-quic:
[`4ab2eceaac81e073f53b22ec72ff56aaea89a3d1`](https://github.com/SagerNet/sing-quic/commit/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1).
Метаданные GitHub commit API прочитаны 28 октября около 09:47Z:
точный SHA, сообщённый API `verified=true`, author/commit date
2026-08-28T02:24:25Z и сообщение
“Fix congestion control senders and remove unused implementations”.
Это запись API, не независимо выполненная криптографическая проверка.

Raw-копии в `candidate-controller/`; глубина чтения:

- `go.mod`, FULL_TEXT_READ: Go1.25, quic-go0.61 mod6, sing0.9 beta4,
  x/crypto0.54, x/sys0.47, косвенная utls1.8.7.
- `LICENSE`, FULL_TEXT_READ: GPL-3.0-or-later.
- `hysteria/congestion/brutal.go`, FULL_TEXT_READ: полный Ex assertion,
  monotime event, no-op neuter/lost/app-limited, actual packet-size constructor.
- `hysteria/congestion/pacer.go`, FULL_TEXT_READ: seed/burst/time arithmetic.
  Проверка отрицательного overflow не доказывает корректность всей арифметики.
- `congestion_meta2/bbr_sender.go`, SECTIONS_READ: constructor/profile,
  ожидание prior-flight в OnPacketSent, batch event, neuter/lost/app-limited.
- `bandwidth_sampler.go`, SECTIONS_READ: создание map256, запись
  `bytesInFlight+bytes`, ACK/loss lookup, neuter, RemoveUpTo.
- `packet_number_indexed_queue.go`, `ringbuffer.go`, FULL_TEXT_READ:
  присутствующие записи и слоты диапазона PN; геометрический рост;
  Pop не уменьшает ёмкость; комментарий об опасности двух далёких вставок.
- `hysteria2/client.go/service.go`, SECTIONS_READ: согласованные конструкторы
  и поведение по connection/session.

Запрос tag-ref `v0.7.0` завершился TLS timeout: **FETCH_FAILED**.
Доступны точные commit bytes, метаданные commit и go.mod кандидата.
Поэтому основание — commit4ab2ece, а обозначение v0.7.0 унаследовано из
предыдущего материала; независимое успешное чтение tag-ref здесь не заявлено.

### 2.2. Отправитель

SagerNet quic-go:
[`v0.61.0-sing-box-mod.6`](https://github.com/SagerNet/quic-go/tree/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80).
GitHub API ref указывает на
`f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`.
В исходном срезе прочитаны метаданные commit: `verified=true`,
author/commit date2026-08-28T02:23:55Z и сообщение
“Fix reporting to injected congestion controllers” с prior-flight,
connection-wide numbering, install, neuter, least-unacked, app-limited и migration.
Само сообщение не заменяет прочитанный вызывающий путь.

- `LICENSE`, FULL_TEXT_READ: MIT.
- `go.mod`, FULL_TEXT_READ: Go1.25, x/crypto0.54/net0.56/sys0.47,
  metacubex/utls1.8.7 и tooling deps.
- `congestion/interface.go`, FULL_TEXT_READ: публичный prior-flight contract
  и terminal methods Ex.
- `sent_packet_handler.go`, SECTIONS_READ: congestionPacketNumber, prior/legacy
  post dispatch, `reports`, DropPackets/neuter, ACK/loss batch+least-unacked,
  остаток QueueProbePacket, migration и SetCongestionControl.
- `cc_adapter_ex.go`, `packet.go`, FULL_TEXT_READ.
- `connection.go`, SECTIONS_READ: sendProbePacket→QueueProbePacket.

Манифест кандидата сохраняет пути, хеши и статус получения. Публичная копия
не подтверждает компиляцию, исполнение или совместимость всего ядра.

## 3. Источники учёта владельцев

### quic-go в исходном срезе

- `datagram_queue.go`, FULL_TEXT_READ: send32, receive128, копирование до
  receive-cap, блокирующий Add, перенос при Pop, сигнал Close.
- `packet_packer.go`, SECTIONS_READ: Pop DATAGRAM в payload frames,
  последующее хранение кадра в метаданных отправленного пакета.
- `sent_packet_handler.go`, SECTIONS_READ: callbacks, history, terminal,
  limits и PTO.
- `sent_packet_history.go`, `packet.go`, `ringbuffer.go`, FULL_TEXT_READ:
  владельцы указателей/frames, разрывы и сохраняемая ёмкость.
- `send_queue.go`, FULL_TEXT_READ: queue8, обслуживаемая запись и drain Close.
- `buffer_pool.go`, `protocol/params.go/protocol.go`, `connection.go`,
  названные разделы: размеры, tracked guards, idle/close, владельцы очередей.
- `server.go`, названные разделы: raw1024, accept32, callbacks/retry,
  handshake goroutine и post-handshake enqueue. Предел очереди не равен
  пределу всех активных соединений.

### sing-quic/sing в исходном срезе

- `hysteria2/packet.go`, SECTIONS_READ: caller buffers, frag aliases,
  channel64, defragger age10 без size cap, перекрытие сборки и перенос владения.
- `client.go/client_packet.go/service.go/service_packet.go`, названные разделы:
  singleflight, active/retiring, клиентская goroutine на сообщение,
  серверные connection/session goroutines, auth после транспортного Accept.
- `hysteria/hop.go`, FULL_TEXT_READ: отдельная optional queue1024.
- `sing/common/buf` и LruCache, названные разделы: logical capacity, Release,
  ленивое удаление по возрасту, отсутствие WithSize у defragger.

## 4. Спецификации и предыдущее основание

- RFC9221, официальный TXT, март2022, Pauly/Kinnear/Schinazi, §§5.2–5.4:
  SECTIONS_READ в прежнем R04-010. ACK DATAGRAM относится к транспорту;
  нет вклада в flow control, приёмник может отбросить данные, congestion control действует.
- RFC9000, официальный TXT, май2021, Iyengar/Thomson, §§10.1–10.2.2:
  SECTIONS_READ в прежних копиях; idle floor3PTO и область closing/draining.
- Получение RFC9002 в этом срезе — FETCH_FAILED из-за TLS timeout.
  Новые выводы не опираются на непрочитанный текст; трасса callbacks —
  отдельное доказательство по реализации.
- `QUIC-ACCOUNTING-AND-LIMITS.md`, SECTIONS_READ: использованные утверждения
  сверены с исходниками, сам тематический текст не принят целиком автоматически.
- R04-007 handoff и нужные части R04-010: историческое основание сравнения,
  не подтверждение актуальных полномочий или выпуска.
- C04 HANDOFF не входил в первоначальный source-review; внешняя парная
  валидация этим пакетом не заявлена.

## 5. Пределы получения и вывода

`spec-identity-fetch.json` фиксирует TLS timeouts RFC9000/9002/9221
и одного sing-quic tag-ref. Использованные пункты RFC9000/9221 уже были
доступны официальными TXT в прежнем ограниченном срезе и прочитаны тогда.
Успех копирования исходников не является приёмкой VPN или сетевого пути.

Внешняя статья о CCA не использовалась. Выводы о цензуре, реальной скорости,
fairness или действующей атаке из этого пакета не следуют.

<a id="r28-z04--sources"></a>
<a id="1-current-source-identity"></a>
<a id="2-candidate-coupled-pair"></a>
<a id="21-controllercaller-side"></a>
<a id="22-sender-side"></a>
<a id="3-holderaccounting-current-sources"></a>
<a id="quic-go-current"></a>
<a id="sing-quicsing-current"></a>
<a id="4-specifications-and-previous-baseline"></a>
<a id="5-failedlimited-acquisition-log"></a>
