# R02-003 — готовый автомат: бюджет, дедлайн и граница остановки

2026-10-26 · **READY_FOR_REVIEW**. R02, сессия
`01a171e0-a1b5-7542-80f1-6da9694158c4`.
Окно заканчивается **2026-10-26T06:30:26Z**. На boundary check05:56:55Z открыто.
[Источники](SOURCES-003.md). Предыдущие пакеты001–002 не изменены и не ACCEPTED.

## 1. Новый вопрос

R02-002 установил необходимость hard adapter caps. Здесь проверено более узко:
**какие свойства конкретных готовых FRONT/RegulaTor/Tamaraw constructors нельзя
наследовать из названия алгоритма или параметра?** Это не ещё одна общая рекомендация
добавить padding. Найдены конкретные различия timer/budget/reset semantics.

Pin Maybenot: `51ce20e806f8e8bdffa0c39b464774d79d617234`, core2.2.2.
Caller: `maybenot-machines/src/lib.rs::get_machine(StaticMachine)` → соответствующий
constructor → Machine states → core Framework events/timers/actions → platform adapter.
Последняя стадия для нашего VPN не реализована и не проверялась.

## 2. R02-07 — RegulaTor C в этом порте не доказанный packet-age cap

`regulator_client(u,c)` создаёт три machines. В
`make_regulator_client_queue_machine(c)` после NormalSent запускается timer
на `c*1_000_000` microseconds. **TunnelSent возвращает timer state к себе и
переназначает timer.** TimerEnd временно снимает blocking, затем оно включается
снова. Это watchdog по отсутствию передачи, а не индивидуальный timestamp
поступления каждого ожидающего packet.

Ручной counterexample, C>0, FIFO допускается, никакого исполнения:

| Время | Событие | Возраст последнего packet A |
|---|---|---|
| 0 | Три NormalSent: a1, a2, A; A в конце очереди | 0 |
| 0.6C | TunnelSent(a1); timer переустановлен до1.6C | 0.6C |
| 1.2C | TunnelSent(a2); timer переустановлен до2.2C | 1.2C |
| 1.8C | TunnelSent(A), ни один timeout не успел истечь | **1.8C>C** |

Этот event trace опровергает получение per-packet guarantee C **из одного
этого watchdog**. Он не доказывает, что полный RegulaTor неизбежно даёт такую
задержку: другой scheduler/адаптер может обеспечить независимый cap.
Другие машины, rate/surge и нагрузка должны рассматриваться вместе.

**Reuse:** использовать готовую управляющую часть при совместимом peer/adapter,
но для утверждения packet deadline нужен timestamp/age enforcement в реальной
очереди, а не имя `c`. Цена — метаданные очереди, early release и возможное
нарушение запланированного shape. Это tradeoff R03/R04/R05, не permission обхода
WFP/TUN. Контроль: progress-gap watchdog и oldest-packet cap проверять раздельно.

## 3. R02-08 — sampled padding budget не является hard resource cap

В pinned FRONT constructor:

- `padding_budget = rng.random_range(1..padding_budget_max)`;
- бюджет распределяется между `num_states` состояниями;
- у каждого состояния uniform limit имеет low1 и high=`padding_count.max(1)`;
- создаётся Machine с `allowed_padding_packets=u64::MAX`, fraction0.

Ручной source witness: `padding_budget_max=2`, `num_states=3`, корректный
положительный непустой range window. Sampled budget всегда1; каждый из трёх
padding states имеет limit1 и переходит по LimitReached в следующий/END.
При корректной подаче PaddingSent и отсутствии внешнего cap эта цепь запрашивает
три padding actions. Проверено дополнительное возражение: core Dist допускает
Uniform low=high и возвращает это значение напрямую; limit1 здесь не исключён
лишь из-за вырожденного Uniform. Это rendered source review, не исполнение.
Следовательно, параметр с именем max **не может сам служить
hard cap2**, а sampled target1 не равен суммарной count3. Это не замер и не
обвинение библиотеки: конструктор аппроксимирует распределение; ресурсная policy
не должна выводиться из обозначения параметра. Capture FRONT — rendered pinned
source observation; raw-byte equality не установлена.

При обычных параметрах target намного больше states, но это не доказывает cap
для всех разрешённых inputs. Framework fraction/allowance semantics из002 остаётся
отдельной границей. Проверять также NaN/nonpositive/degenerate inputs до constructor;
валидность protobuf/JSON/подписи не означает пригодность численных параметров.

**Единицы важны:** core sample durations конвертируются `Duration::from_micros`.
RegulaTor `c` переводит seconds→microseconds; Tamaraw `p` переводит seconds/packet
через множитель10^6, а `stop_window` передаёт в duration напрямую и документирован
как microseconds в StaticMachine. FRONT ranges проходят без такой конверсии.
Нельзя передавать одинаковое число «секунд» во все floating-point аргументы.

**Reset amplification, ручная модель.** Если каждый новый instance получает
startup allowance F bytes и за интервал создаётся k instances, per-instance cap
ограничивает лишь `ΣF_i≤kF`. Он не устанавливает общий cap G, не зависящий от k.
При возможности повторных reconnect/retrigger противник либо workload может
увеличивать k. Это условная модель, **не найденный exploit нынешнего клиента**:
Maybenot в продукт не встроен. Нужна явно выбранная ресурсная область quota,
которая не обнуляется автоматически вместе с network/protection epoch.
Обновление security context и выдача нового traffic budget — разные действия.

## 4. R02-09 — stopping rule и случайность входят в fingerprint

Tamaraw constructor включает padding machine и soft-stop machine. В последней
NormalSent перевзводит timer; после TimerEnd начинается tail state и затем release.
Source использует счётчик100, но до tail считает TunnelSent, а в tail — PaddingSent.
Уже установленное в002 различие queued intent / replacement / emitted означает,
что комментарий «кратно100 packets» нельзя без adapter trace объявить wire invariant.
NormalSent — enqueue event, а не подтверждение, что пользовательская операция
завершилась или очередь пуста. Поэтому stop boundary может зависеть от workload,
backpressure и event delivery, а не только от логической длины сессии.

Ручной вывод: два workload, одинаковых до момента t, но один с NormalSent
до expiry, могут получить разные stop times. Само наличие этого различия
**не даёт remote classifier**: требуется подтвердить, что разница наблюдаема и
отличима от benign idle/потерь. Finite cover не гарантирует сокрытие долгого idle;
adaptive stop не становится неправильным только потому, что раскрывает часть timing.
Нужно заранее выбрать защищаемую цель и допустимую leakage.

Случайная смена defenses также не создаёт криптографической неразличимости.
Для доступного наблюдения x и hidden sampled mode θ распределение имеет вид
`p(x|class)=Σθ wθ·p(x|class,θ)`; при continuous θ — интеграл. Противник может
обучаться на смеси/переносить признаки. Число seeds/modes само по себе не
ограничивает его accuracy. Это математическая модель, не анализ обученного RF.

Свежий контраргумент: PETS2026 исследует ephemeral defenses, но одновременно
проверяет transfer между defense families и оговаривает трудности faithful ports
RegulaTor и влияние soft stop. Отсюда берётся программа сравнения, **не обещание
процентов защиты для РФ/UDP/наших платформ**. Старый FRONT «zero-delay» означает
отсутствие намеренного задерживания payload; дополнительная нагрузка способна
создавать очереди. WPES2022 исследует это симуляцией Tor, не нашим live опытом.

## 5. Сравнение reuse

| Кандидат | Полезная часть | Неустановленное / решение |
|---|---|---|
| FRONT constructor, MIT OR Apache-2.0 | Finite randomized front schedules | Approximation и численные inputs; target budget≠hard cap. Adapt только с внешним quota и wire framing |
| RegulaTor constructor, та же лицензия | Rate decay/surge и progress watchdog | C не per-packet age по одному source branch; server/client composition и queue cap отдельно |
| Tamaraw constructor, та же лицензия | Fixed schedule + practical soft stop | Unit mismatch риск; tail/event semantics и overhead; не полный классический алгоритм по одному имени |
| Ephemeral PETS2026 artifact pin31ff5846… | Воспроизводимые research materials, references к experiment settings | Прочитан README, не запущены scripts/data/model training. Корневая license не установлена в этом срезе: reference only |

Производственный runtime не должен требовать GPU/research training pipeline только
потому, что artifact его использовал. Наличие badge у artifact не доказывает наш
native path и не разрешает запуск в текущем plan-only. Генерация/оценка defenses
отделяется от bounded исполнения заранее approved machine на устройстве.

**Windows/Android:** можно переиспользовать state-machine definitions только после
transport adapter/peer framing и lifecycle контракта. Ни protected socket ownership,
ни per-packet timestamp, ни AAR/FFI compatibility этим исследованием не появились.
На Android startup quota и frequent retry имеют отдельную battery/traffic цену.
В pure adaptive policy следует передавать наблюдаемые budgets/reasons, не отдавать
ей управление crypto/OS barrier или таймерами transport IO.

## 6. Передача и конечная будущая приёмка — NOT RUN

TH-05/06/E-04, AND-01/18 и COM-18; новые product IDs не заведены.

1. **R04:** progress trace a1/a2/A и oldest age; определить actual send vs enqueue,
   queue byte/packet units, partial/error/replacement cases.
2. **R03/R05:** resource quota scope при k resets; stale callbacks/clock/network
   epoch не выдают новый grant и не переносят counters произвольно.
3. **FRONT:** boundary `max=2/states=3`, обычные параметры, bad/NaN/zero ranges;
   witness сейчас ручной, code generation/crypto/source не исполнялись.
4. **Tamaraw:** late NormalSent до/после expiry, pending queued data, tail events,
   actual bytes и rounds; отдельно policy semantics и wire observation.
5. **Сравнение:** cold/warm/late/idle и held-out traffic/defense modes; прочие методы
   сравнивать на одинаковых byte/latency caps и safety scope, не только на average overhead.
6. **RF:** отдельные будущие конкретные network/operator/date tests; transport
   failure/endpoint outage не превращать в fingerprinting detector attribution.

Самооппонирование: противопоставлены исходный алгоритм/его порт, effect/cap,
счётчики/байты, выбранный schedule/фактический send и leakage/реальный classifier.
Научный ACCEPTED и интеграция принадлежат руководителю. До deadline можно перейти
к независимому source вопросу; после него новые срезы не начинать.
