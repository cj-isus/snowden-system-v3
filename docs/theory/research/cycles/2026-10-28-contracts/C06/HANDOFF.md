<a id="r28-c06--передача-руководителю"></a>
# R28-C06 — полезное восстановление и Android UX

Статус: **READY_FOR_REVIEW**, source/model область. 28 октября 2026.

## Результат

Полный аргумент — [REPORT](REPORT.md), pins/read depth — [SOURCES](SOURCES.md).

| Claim | Ограниченный вывод / recommendation |
|---|---|
| **R28-C06-A** | Recovery UX должен различать observation, action admission, effect и task. Android существующие Tile/settings/Stop guards сохранить. Windows NetGuard READY/RUNNING/no-access/порт не повышать до обещания защиты/работающего ремонта. Предложение **READY_FOR_REVIEW** |
| **R28-C06-B** | Исправленный concrete cohort contract: fixed enrolled N, maturity независимо от outcome, original/arrival times, clock-unknown, census/dedup loss, delayed evidence и отдельный resource clock. Формулы условны на sound producers; **READY_FOR_REVIEW**, baseline R06-013-A NEEDS_REVISION самовольно не закрывается |
| **R28-C06-C** | Exact AOSP/public API подтверждают conditional capture≠app-rule gap R06-012-D и mapper floor API35. Текущие native/signer/shared-UID/lifecycle blockers сохраняются; это **не** доказанный работающий BLOCK bypass. Proposal **READY_FOR_REVIEW** |

### Что нового относительно прежней рецензии

Прежние maturity/early-QF/snapshot замечания независимо перепроверены и доведены
до operational record. Добавлены: command rejection до journal; census completeness
и bounded retention/dedup; отсутствие нашей policy ≠ глобально unprotected при
другом VPN; временное пересечение sample ≠ coverage; retirement origin отдельно
от terminal task; source-конкретные Windows weak-to-strong projections.

Конкретный accounting: **N=P+A+X+F+Z**. P — достаточный own claim при timely goal;
A — timely goal при установленном отсутствии требуемой own policy; X — timely goal,
policy unknown; F — надёжный noncompletion к D; Z — неизвестный timely outcome.
Maturity/reasons отдельно. Bounds:

- own claim: `[P/N,(P+X+Z)/N]`;
- task любым способом: `[(P+A+X)/N,(P+A+X+Z)/N]`.

Без полного census — **DENOMINATOR_UNAVAILABLE**. Без принятого protection producer
P не создаётся из порта/consent/плитки/пробы; неизвестность — не нулевая надёжность.
Это local reported-goal accounting, не causal ranking всех VPN или confidence interval.

## Самая сильная оппозиция

- **К maturity:** из двух зарегистрированных A рано успешен, B pending; baseline
  resolved-only N даёт 100%, хотя full-cohort result ещё не установлен. Наш N
  остаётся 2; pending не исчезает, early partial не становится failure.
- **К защите:** receipt в точке 11 не покрывает completion interval[10,12]; свежий
  sample не доказывает whole-task protection. Own Stop также не доказывает foreign
  protection off. Unknown нельзя исправить только новым названием метрики.
- **К Windows repair:** один и тот же loopback1080 может выставить другая сторона;
  формат/неуспешный connect не доказывают owner. Прочитанному heal caller не хватает
  owner/concurrency receipt перед ProxyEnable=0. Другие порты/форматы и доступный
  listener текущий guard отклоняет — это не blanket отключение любых PAC/VPN.
- **К Android bypass claim:** raw UID/package seam существует в source, однако
  native candidate содержит существенные blockers; sharedUserId/signers/epoch и
  Tile fresh-action guards уже реализованы. Они не дают назвать контрпример live.

Дополнительно: сетевой timeout не доказывает неуспех бизнес-операции; RFC9110 retry
семантика перенесена как ограничение UX. Не советовать автоматически повторять
операцию с неизвестным commit и не обещать фактический показ уведомления.

## Переиспользование и следующий порядок

1. Windows source facts/poller сохранить, заменить неподтверждённые projections;
   Android projections/Tile/settings guards расширять в текущих boundaries.
2. Отдельно зафиксировать producer scope/time/currentness и Windows proxy ownership;
   не менять foreign MDM/PAC/NRPT/DNS/WFP ради recovery wizard.
3. Android raw/semantic UID seam и API capability, signer/user/epoch/native parity
   закрывать до снятия app-policy blockers. Public mapper — часть решения.
4. Opt-in episode store проектировать отдельно от ring128: enrollment frontier,
   immutable cohort, deletion, clock linkage, bounded dedup и report revision.
5. Проверки runtime/device/protected task/UX — только следующим разрешённым срезом.

Android local source SPDX GPL-3.0-or-later; AOSP captures Apache-2.0 headers.
Нового OSS кода/dependency/uploader не добавлено. OONI — baseline-only reference,
не current source/licence approval на копирование. Стоимость реализации/CPU/RAM/
батарея/сроки и экономия токенов не измерялись.

## Пройденный объём и остаток

Четыре work stages пройдены: source/pins/callers; actor/state/action matrix;
opposition + concrete outcome/cohort; reuse/API/licence/cost/paired availability.
Z08/Z09 HANDOFF **не были доступны** при чтении 28 октября в 09:21:32Z.
Поэтому независимое сопоставление этих результатов не входило в данный срез.

Авторский source/математический review и статическая сверка завершены;
36/36 исходных файлов совпали с сохранёнными хешами на 09:42:19Z.
Runtime/tests/fixtures/build/simulation/native/
device/live/admin/VPS/SSH/deploy/OS-network: **NOT_RUN** по текущему plan-only.
Task census/receipt validity/clock bridge/native parity и реальная понятность UX
остаются открытыми предпосылками будущей реализации.

Позднейшие [Z08](../Z08/HANDOFF.md) и [Z09](../Z09/HANDOFF.md) сохранены
отдельно; совпадение тем не означает независимого принятия друг друга.
Результат по исходникам не является продуктовым PASS.
