# Аудит алгоритма обхода и адаптации — 2026-09-23

## 1. Область аудита

Проверены нормативная теория из `docs/CENSORSHIP-THEORY.md` и текущая реализация Windows-клиента:

- protected probe и direct leak control;
- классификатор причин отказа;
- controller state machine;
- быстрые репробы, failover, quarantine и scoring;
- реакция на смену сети;
- fail-closed и TUN kill switch;
- volume/freeze diagnostic;
- связь сетевого контекста с evidence.

Аудит опирается на код и локальные unit/integration tests. Живое воздействие ТСПУ, смена оператора и авария kill switch в этой сессии не воспроизводились, поэтому такие свойства отмечены как требующие live-проверки.

## 2. Нормативный контракт теории

Критические правила, по которым оценивалась реализация:

1. Protected proof состоит из DNS через туннель, двух HTTPS-целей, ожидаемого egress и negative direct-leak test (`CENSORSHIP-THEORY.md:561`).
2. Провал probe, утечка и недоступность канала — разные состояния с разными реакциями (`CENSORSHIP-THEORY.md:568`):
   - мерцание → быстрые репробы;
   - реальная утечка → немедленный fail-closed;
   - подтверждённая деградация → class-aware failover.
3. Allowlist/shutdown не имеют программного обхода: нужен честный `BLOCKED`, физически другой путь и автоматическая перепроверка после восстановления (`CENSORSHIP-THEORY.md:156`, `:356`).
4. Классификация обязана сохранять оператора и тип сети на момент события (`CENSORSHIP-THEORY.md:456`).
5. Fail-closed обязателен, прямой fallback запрещён.

## 3. Итог

Каркас алгоритма сильный, но сейчас это **адаптивный монитор с универсальным failover**, а не полностью реализованный class-aware обход из теории.

Что уже соответствует теории:

- запуск и переключение канала проходят через обязательный protected probe;
- expected egress, две HTTPS-цели и DNS через туннель реально проверяются;
- краткое мерцание поглощается серией быстрых репроб;
- кандидаты проходят обязательный probe, при неудаче выполняется rollback либо остановка;
- прямого fallback в селекторе нет;
- классификатор детерминирован, умеет честный `unknown` и различает основные наблюдаемые формы;
- health evidence привязано к эпохе сети, stale evidence не считается свежим;
- scoring учитывает health, независимость, свежесть и объём evidence;
- volume diagnostic измеряет 8/16/32/64 KiB, TTFB, полное время и форму `silence/abrupt`;
- конфиг запрещает insecure TLS и проверяется до запуска.

Главный разрыв: `FailureClass` влияет на журнал и UI, но почти не влияет на действие. DNS failure, egress divergence, silent freeze, TLS policing, port blackhole, allowlist и общий channel unavailable после подтверждения идут в одну и ту же ветку generic failover.

## 4. Критические находки

### P0-1. Обнаруженная утечка не вызывает немедленный fail-closed

**Код:**

- `windows/backend/core/probe.go:307-330` формирует отрицательный результат `Direct leak check`.
- `windows/corebridge.go:548-594` любой `!AllPassed`, включая утечку, сначала проводит две репробы с паузой 5 секунд, затем классифицирует и запускает обычный failover.
- `windows/failclass.go:229-349` не имеет отдельного класса direct leak, хотя `splitSteps` видит этот шаг.

**Почему это нарушает теорию:** правило §3.9 требует для реальной утечки отдельной немедленной реакции. Сейчас утечка смешана с деградацией канала и может сохраняться на время репроб и переключения.

**Исправление:** результат probe должен иметь отдельный структурный outcome `LEAK_CONFIRMED`. Watchdog обязан без debounce остановить manager, применить/сохранить блокировку, записать incident и перейти в `BLOCKED/RECOVERING`. Failover допускается только после остановки опасного пути и только как новая probe-gated попытка.

### P0-2. TUN продолжает работу при недоступном kill switch

**Код:**

- `windows/corebridge.go:346-359`: ошибка получения exe или применения firewall помечается `fail-open`, но успешный TUN-запуск не отменяется.
- `windows/corebridge.go:648-666`: ошибка продления lease только логируется.
- `windows/killswitch_windows.go:44-48`: lease истекает через 90 секунд.

**Риск:** при отказе применения/продления firewall TUN-сессия считается запущенной без гарантии fail-closed. После аварии процесса dead-man намеренно снимает блокировку, то есть постоянной fail-closed гарантии тоже нет.

**Исправление:** в TUN `apply/renew kill switch` должен быть частью стартового и runtime safety gate. Ошибка означает немедленную остановку TUN и `BLOCKED`, а не продолжение. Dead-man policy нужно явно выбрать на уровне продукта: безопасность после crash либо автоматическое восстановление обычной сети.

## 5. Высокий приоритет

### P1-1. Отсутствие negative leak evidence считается успешным шагом

`windows/backend/core/probe.go:314-324` меняет `Passed=false` на `Passed=true`, если direct-запрос недоступен. Это поведение закреплено тестом `windows/backend/core/probe_test.go:105-110`.

В результате `AllPassed=true` возможно без negative direct-leak evidence, хотя теория включает этот тест в состав protected proof. Нужен трёхзначный статус шага: `pass | fail | unavailable`. `unavailable` не должен изображать доказательство; итог должен быть `protected_path_ok_but_leak_unproven` или блокировать режим согласно policy.

Дополнительно `windows/backend/core/probe.go:136` создаёт direct-клиент с default transport. Он может учитывать `HTTP_PROXY/HTTPS_PROXY`, поэтому контроль не гарантированно прямой. Нужен собственный `http.Transport{Proxy:nil}`.

### P1-2. Класс отказа не выбирает контрмеру

`windows/failclass.go:21` прямо фиксирует, что classifier не меняет поведение. `windows/corebridge.go:578-594` после любого класса вызывает одну ветку `tryFailover`, а `windows/scoring.go` не принимает `FailureClass`.

Это не реализует матрицу §2.10:

| Наблюдаемый класс | Требуемая реакция | Сейчас |
|---|---|---|
| `dns_path_failure` | сменить bootstrap/DNS strategy, повторить доказательство | generic failover |
| `egress_divergence` / leak | немедленный fail-closed | generic failover после репроб |
| `silent_freeze` | short-lived/split/UDP-класс | любой следующий scored candidate |
| `tls_policing_443` | high port/mux/другой transport class | любой следующий scored candidate |
| `port_blackholed` | другой port family/transport | любой следующий scored candidate |
| `whitelist_mode` | `BLOCKED`, подсказка о другом операторе/физическом пути | попытка переключения каналов того же origin |
| `channel_unavailable` | survivor по failure domains | generic scoring — частично соответствует |

Нужен явный `Decision` между classifier и executor: допустимые контрмеры, запрещённые действия, требуемая уверенность, safety gate и причина выбора.

### P1-3. Ветка смены сети нарушает state machine и теряет evidence

`windows/netwatch.go:360-384` после подтверждённого отказа вызывает `markDegraded` напрямую. Но `windows/controller.go:125-133` разрешает этот переход только из `CLASSIFYING`; состояние остаётся `SUSPECTED`, после чего `tryFailover` всё равно переводит его в `SWITCHING`.

В этой ветке также нет:

- `ClassifyFailure` и `markClassified`;
- записи FAIL активному каналу в `chanHealth`;
- записи последнего отчёта и успешного evidence при восстановлении после fast reprobe;
- продления kill switch и переоценки preference после такого восстановления.

Нужен единый pipeline обработки probe outcome для watchdog и netwatch, чтобы порядок `observe → confirm → classify → decide → execute → verify` не расходился.

### P1-4. После неудачного switch возможен fail-closed без назначенного авторетрая

`windows/failover.go:212-221` оставляет остановленный manager при отказе обоих каналов. `tryFailover` затем возвращает `blocked=false`, пока лимит попыток ещё не исчерпан (`windows/failover.go:82-98`, `:283-288`).

В watchdog `windows/corebridge.go:614-631` состояние переводится в `RECOVERING`, но `scheduleFailClosedRetryLocked` не вызывается. В netwatch `windows/netwatch.go:384-386` функция вообще возвращается на `!blocked`. Получается остановленный защищённый путь без обещанного автоматического восстановления.

**Исправление:** решение о retry должно зависеть от фактической живости manager, а не только от счётчика policy. Любой остановленный manager в `RECOVERING` обязан иметь назначенный retry/backoff.

### P1-5. Диагностика может выдать ложное резюме direct-контроля

В SOCKS-режиме `windows/diagprobe.go:83-132` выставляет `rep.OK=false` только по tunnel-ступеням. `summarizeDiag` на `windows/diagprobe.go:283-289` при `rep.OK && DirectControl` пишет, что все ступени прошли и через туннель, и напрямую, даже если direct-ступени упали.

Это текстовая недостоверность evidence. Следует отдельно агрегировать `TunnelOK` и `DirectOK`; direct failure не обязан ронять tunnel verdict, но summary должен отражать его честно.

### P1-6. Равные timestamps дают противоречивый health verdict

`windows/chanhealth.go:65` считает failure последним только при строгом `LastFailAt.After(LastOKAt)`. На Windows два последовательных события могут получить одинаковое wall-clock значение. Тогда health становится `healthy`, а `windows/quarantine.go:91` не считает успех сильным и может оставить `QUARANTINED`.

Нужен монотонный sequence/event ID либо единая запись `lastOutcome + observedAt`; сравнение двух независимо обновляемых timestamp недостаточно.

### P1-7. Классификация не хранит operator/network context события

Теория требует оператор и тип сети, но `ClassifyFailure` получает только `ProbeReport` и порты (`windows/failclass.go:229`), а log line содержит только код и confidence (`:354-357`). Текущие NetworkFacts существуют отдельно и могут измениться после события.

В TUN это ещё слабее: netwatch намеренно не видит физическую сеть (`windows/netwatch.go:207-210`), а UI collector выбирает текущий default route, которым может быть `snowden0` (`windows/infobindings_windows.go:25-45`).

Нужен immutable `FailureEvidence` с `epoch`, operator/ISP, network class, adapter identity, channel, port family, transport, probe steps и timestamps.

## 6. Средний приоритет и архитектурные ограничения

### P2-1. «Direct egress» в TUN не является независимым direct

`windows/infobindings.go:280-303` отключает HTTP proxy, но TUN маршрутизирует сам IP-пакет через `snowden0`. Поэтому поле нельзя называть независимым direct в TUN. Diagnostic code уже правильно признаёт отсутствие independent direct control; NetworkFacts должен применять ту же семантику.

### P2-2. «Все каналы исчерпаны» ограничено константой двух попыток

`windows/failover.go:36-37` задаёт `failoverMaxSwitches=2`. Для текущего набора из трёх каналов это совпадает с числом альтернатив, но при расширении signed envelope третья и последующие альтернативы не будут проверены. Лимит следует вычислять из incident candidate set либо называть его именно attempt budget, не «все каналы».

### P2-3. Порог независимости информационный, а не gate

`windows/backend/render/fdmodel.go:155-157` объявляет пары ниже `0.35` слишком общей инфраструктурой, но scoring использует непрерывное значение и всё равно допускает такого кандидата. Это может быть разумным временным fallback при одном VPS, однако policy должна явно определить: низкая независимость запрещает автоматический switch или только понижает ранг.

### P2-4. Один слабый background failure сразу показывает QUARANTINED

`windows/quarantine.go:111-115` переводит канал в `QUARANTINED` при `bg.Consec >= 1`, хотя подтверждённый background failure требует серии. Это консервативно для tie-break, но расходится с принципом «одиночный сетевой чих не создаёт факт отказа». До подтверждения уместнее `SUSPECTED/PROBATION`.

## 7. Целевая схема алгоритма

```text
Observation
   ↓
Structured evidence
   ├─ protected path proof
   ├─ direct leak proof / unavailable
   ├─ network epoch + operator + type
   └─ channel failure-domain facts
   ↓
Outcome gate
   ├─ LEAK_CONFIRMED ─────────→ stop immediately → BLOCKED → safe retry
   ├─ EVIDENCE_INCOMPLETE ─────→ limited state / policy gate
   ├─ TRANSIENT_PROBE_FAILURE ─→ fast reprobe
   └─ CONFIRMED_DEGRADATION ───→ classifier
                                      ↓
                                  decision table
                                      ↓
                         candidate constrained by countermeasure
                                      ↓
                         execute → protected probe → commit/rollback
```

Минимальный контракт `Decision`:

```go
type Decision struct {
    Action           string   // stop, retry, switch, change_bootstrap, blocked
    AllowedClasses   []string // transport/port/failure-domain constraints
    RequirePhysical  bool
    RetryAfter       time.Duration
    Reason           string
    EvidenceID       string
}
```

## 8. Рекомендуемый порядок исправлений

1. Разделить `leak`, `probe failure` и `evidence unavailable`; сделать leak немедленным fail-closed.
2. Сделать TUN kill switch обязательным runtime gate.
3. Исправить потерянный retry после остановки manager.
4. Объединить watchdog/netwatch в один outcome pipeline.
5. Ввести class-aware decision table и ограничивать кандидатов по transport/port/failure domains.
6. Сохранять immutable network context вместе с failure evidence.
7. Исправить direct transport, diagnostic summary и порядок событий health.
8. После unit/contract tests провести live matrix: SOCKS/TUN, Wi-Fi/LTE, kill switch apply/renew failure, direct leak simulation, network change, каждый classifier class, все кандидаты мертвы и восстановление сети.

## 9. Проверка в этой сессии

- `go test ./... -count=1` из `windows/` — PASS для всех Go-пакетов.
- Прохождение текущих тестов не опровергает находки: часть ошибочной семантики закреплена тестами (direct unavailable = pass), а class-aware reaction, leak fast-stop, dead-manager retry и равные timestamps отдельными contract tests не покрыты.

## 10. Статус реализации V2-087

Закрыты P0-1 и P1-1…P1-7: утечка стала отдельным outcome с немедленным stop;
unavailable больше не PASS; watchdog/netwatch используют полный порядок
классификации; dead manager получает retry; выбор кандидата учитывает класс;
diagnostic summary, порядок health-событий и event-time network context исправлены.
Один слабый background fail больше не карантинит канал.

P0-2 закрыт безопасным ограничением: TUN production отключён. Дополнительная
проверка показала, что explicit Windows Firewall block имеет приоритет над
обычным allow, поэтому прежняя catch-all netsh policy не могла служить
доказательством работоспособного kill switch. Возврат TUN требует отдельной
WFP-реализации и live crash/apply/renew проверки. До этого рабочим режимом
остаётся SOCKS.

Основание для ограничения TUN: Microsoft документирует, что explicit block
имеет приоритет над конфликтующим allow и что Windows Firewall не поддерживает
назначаемый администратором порядок правил:
https://learn.microsoft.com/windows/security/operating-system-security/network-security/windows-firewall/rules

## 11. Финальная live-проверка и владение proxy (V2-088)

Production EXE запущен с `--auto-connect`. В текущей сети protected probe не
подтвердил канал: после двух неуспешных проверок локальный listener 1080 был
закрыт, а сохранённый до запуска системный proxy восстановлен. Это подтверждает
fail-closed cleanup, но не является доказательством доступности канала.

Во время проверки обнаружена отдельная ошибка владения ресурсом: self-heal и
scheduled NetGuard распознавали любой `127.0.0.1:<port>` как собственный proxy.
На машине одновременно работает другой локальный proxy на ином порту, поэтому
широкий предикат мог повредить чужое рабочее состояние после его остановки.
Предикат сужен до `127.0.0.1:1080` — единственного адреса, который записывает
Snowden System. Тесты обновлены; внешний proxy сохранён при smoke-запуске.

Финальные локальные гейты: `go test ./... -count=1`, `go vet ./...`, Wails
production build и smoke — PASS. Артефакт: 36 060 672 bytes, SHA256
`DE77779ED00D88B7829E8DD4FFDAA4DC5EEEE885D2C3C43E6CF1AFB5D918267B`.

Установленная системная копия NetGuard затем обновлена с elevation и проверена
живым запуском задачи: result=0, широкий loopback-предикат отсутствует,
сторонний активный proxy не изменён, порт Snowden System 1080 закрыт.
