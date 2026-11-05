<a id="r28-c02--передача-руководителю"></a>
# R28-C02 — допустимость защищённого сетевого пути

28.10.2026.

**Авторский статус: READY_FOR_REVIEW для source/spec/manual-model области.** Это рекомендация для отдельного review, не ACCEPTED, разработка или product/native/live PASS.

Основной результат: [REPORT](REPORT.md), точные источники и reading scopes: [SOURCES](SOURCES.md).

## 1. Что нового

Переносимый **protection-preserving path-decision contract**: inputs, T/F/U predicates, отдельные допускаемые действия, матрица M01–M15, abstain, source-bound cache/handover и seam platform owner. Изменение endpoint/transport сохраняет исходные floors независимо от того, известна ли причина отказа.

Разделены четыре разрешения: **попробовать соединение → отправить tunnel credentials → выполнить owned validation → выпустить пользовательский трафик**. Недостающая reachability не запрещает попытку, но недостающий обязательный safety predicate не разрешает более сильное действие. Timeout не выдаёт прав на downgrade и не становится DPI diagnosis.

Не требуем отдельного ECH bool, если equivalent evidence следует из доказанного actual backend/caller/config contract; reconstructed false сам по себе такой evidence не даёт. Связанный deeper trace показал, что первый VLESS Write/WS early data может уже нести user payload. Поэтому setup permit нельзя расширять на весь первый buffer.

## 2. Общий паспорт области

| Поле | Значение |
|---|---|
| Evidence | Первичные RFC norms; exact source facts; ручные counterexamples и conditional model. Никакого execution/field evidence |
| Дата / pins | Windows manifest повторно прочитан 28.10: sing-box1.13.19 `b5ebaa1…`, uTLS1.8.4 `cf49b086…`, sing0.8.13, miekg/dns1.1.72; дополнительный first-write trace sing-vmess pseudo `3aed155119a1` / full `3aed155119a174c9290a4099841049f0cb275e02` |
| Actual caller | `corebridge.renderAndPrepareContext → PreflightResolver → render VLESS/WS → sing-box TLS/WS/dialer`; текущий renderer **не включает real ECH**. ECH transitions условны для future consumer, не текущая deployed неисправность |
| Принципиальные предпосылки | Действительная platform policy/owner/epoch; правильные reference identities и разрешённые disclosures; native barrier/validation отдельно. Их истинность runtime здесь не доказана |
| Рекомендация | Принять A как ограниченный proposed contract, B/C как source-scoped prerequisites/counterexamples после отдельного review; не переносить scientific verdict в product PASS |
| Ready reuse / лицензии | Existing sing-box/sing-vmess GPL-3.0-or-later; Go/uTLS/miekg DNS BSD-3-Clause. Native crypto/parser/exchanges использовать; complete recovery адаптировать, не писать HPKE/ECH заново. Whole-product compliance и artifact parity остаются открыты |
| Цена | Immutable per-attempt plan/source/freshness/owner state, bounded cache и один общий budget; HTTPS/A/AAAA/alias work, отдельный handshake retry; typed backend evidence, handover и поздние результаты. Численные RAM/latency/energy/token/cost не измерены |

## 3. Паспорта выводов

### R28-C02-A — разрешение перехода без causal certainty

| Поле | Содержание |
|---|---|
| Вопрос / короткий вывод | Какие действия допустимы при unknown failure? Конечная попытка другого разрешённого пути возможна без диагноза DPI; confidentiality/identity/bootstrap/user-routing floors при этом не снижаются |
| Статус / вид | READY_FOR_REVIEW; proposed decision contract + RFC/source/manual model |
| Источники | REPORT §§2–6; RFC9848 §§4–5, RFC9849 §§6.1.6–6.2.1, RFC9460 §§3.1/5.1/8, RFC9525 §6.1.1; SOURCES S1–S5 |
| Самый сильный opposing case | Fixed endpoint с общим ECH key и корректной role-policy может быть безопасен без generic Service Binding selector. Nil от доказанного real-ECH-only handshake может заменить отдельный acceptance bool. Эти случаи разрешены контрактом; отсутствие transport evidence не blanket deny |
| Связанное углубление | First VLESS Write включает payload; WS header после TLS может содержать его префикс. Origin TLS/egress success не доказывает секретность от разрешённого CDN terminator. Unknown operation delivery не даёт exactly-once replay |
| Reuse / owner consequence | Использовать текущие render/dial/Manager/probe/session seams и dependency Shared/Different/Unknown; не новый controller/engine. Platform владеет фактами/enforcement, portable policy лишь рекомендует действие/abstain |
| Что проверить / NOT_RUN | ECH-required vs opportunistic; O/T/P split, first-write content/probe ownership, handover между стадиями, late success, native protection и validation scope; REPORT casesA–K |

### R28-C02-B — freshness и reset не удостоверяют endpoint binding

| Поле | Содержание |
|---|---|
| Вопрос / короткий вывод | Сохраняют ли готовые cache/retry state правильный plan? Нет такого общего доказательства: singleton IP/TTL не содержит service/source binding; конкретные source transitions требуют отдельного admission |
| Статус / вид | READY_FOR_REVIEW; ограниченные exact-source facts и условные traces |
| Exact callers | sing-box `ech.go:133–189`, `client.go:117–134`, `resolve.go:94–111`; Windows `preflight.go:66–71,231–245`, `corebridge.go:148`; NetworkManager.ResetNetwork → VLESS.InterfaceUpdated → WS.Close |
| Новые факты | Expired config + NOERROR без нового ech оставляет старый nonempty list; Clone не переносит lastTTL; zero-TTL/expired fetch может overwrite принятый retry; domain retry не сохраняет прежний address set. DNS/CloseAll/mux reset не означает reset ECH wrapper |
| Сильный guard / контрпример | Transport/Rcode DNS error прекращает handshake; fresh TTL/совместимый fixed IP/shared key могут исключить конкретный mismatch. CloseAll/reset действительно присутствуют, полный no-op не заявлен. Current Windows renderer без real ECH и literal override ограничивает applicability |
| Reuse / цена | Existing DNS/TLS primitives + immutable source-bound attempt view; не общий hostname-only retry cache и не blind new lock. Source assignment/lifetime/source identity и budgets требуют адаптации |
| Что проверить / NOT_RUN | REPORT casesB/C/F/I/J; active SDK build/native reset reachability, actual server/policy unknown. Никакой гонки/runtime failure этим срезом не воспроизведено |

### R28-C02-C — backend error type не доказывает правильный public-name verifier

| Поле | Содержание |
|---|---|
| Вопрос / короткий вывод | Достаточно ли нормализовать ECHRejectionError? Нет: сначала требуется доказать проверку P, а не O, на точном backend |
| Статус / вид | READY_FOR_REVIEW; targeted source-совместимость + manual opposing certificate cases. Не security scan/exploit/CVE |
| Exact trace | Core uTLS Config.ServerName=O → real-ECH SNI=P; `SNIExtension.writeToUConn` сохраняет config O; UConn сохраняет outer P отдельно. uTLS `handshake_client.go:1161–1195` default rejection verifier берёт **config.ServerName**/explicit verify name, а не `c.serverName` |
| Контрпример | O≠P, cert только P: name predicate O не пройдёт, хотя это правильный P-only recovery peer. Cert только O удовлетворяет этому name predicate без доказательства P; chain/time/Finished и остальные guards всё ещё нужны |
| Ограничивающие guards | Rejection возвращается error до successful handshake completion; core retry ждёт другой named stdlib type. O=P, multi-SAN cert либо доказанный dedicated verifier — отдельные положительные случаи. Current renderer real ECH не включает |
| Ready reuse / owner consequence | Go/std native primitives либо uTLS после доказанной совместимости. Одно исправление errors.As не закрывает auth role. Не добавлять самодельный verifier в adaptive policy; platform TLS adapter обязан сохранить typed scope |
| Что проверить / NOT_RUN | REPORT caseD: STD/uTLS, P-only/O-only/both/wrong cert, actual SDK/error path, wrong public name; artifact/server/native неизвестны |
| Primary evidence | [uTLS verification](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/handshake_client.go), [SNI guard](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_tls_extensions.go), [UConn caller](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_handshake_client.go), REPORT §6.2 / SOURCES S3–S4 |

## 4. Что нельзя перенести в продукт как установленное

- Текущая работоспособность EXE/VPS, ECH deployment, native fail-closed, confidential egress, конкретные блокировки РФ, скорость/ресурсы — UNKNOWN / NOT_RUN.
- Ни positive expected-egress probe, ни label `http-failure` не доказывают origin role/ECH privacy/общую недоступность ASN или bootstrap dependency.
- Состояние защиты нельзя считать восстановленным по одному transport success. Неизвестные hard predicates требуют abstain для соответствующего действия; это не универсальный ban всех попыток.
- Нет разрешения менять продукт, dependencies, DNS/WFP/TUN, запустить tests/lab/live или переносить выводы в чужие планы. Руководитель решает scientific integration отдельно.

## 5. Work stages / конечность

1. **Пройдено:** exact manifest/pins/current caller/source availability, границы public source и artifact.
2. **Пройдено:** полный contract inputs/predicates/action permits, M01–M15, abstain.
3. **Пройдено:** endpoint mismatch/shared-key positive control, authenticated rejection/backend guard, GREASE/observation projection, shared DNS/IP/ASN evidence scope; дополнительно cache/handover/name split и first-write payload.
4. **Пройдено:** готовые APIs/licences/reuse/adapt/defer и state/query/migration cost; finite future cases NOT_RUN.

Нерешённое требует отдельной platform/artifact/native/field приёмки. Paired input и финальная статическая сверка фиксируются в SOURCES.
