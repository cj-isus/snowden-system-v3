<a id="deep-review-2026-10-28"></a>
# Решения глубокой сверки DR27 — 2026-10-28

[Карта подробных разборов](../DEEP-REVIEW-2026-10-28.md) · [Очередь review](../REVIEW-QUEUE.md) · [Библиотека](../../README.md)

Сохранены 15 полных разборов и частичный DR27-16 с ограниченным дополнением.
Два дополнительных оппонирующих прохода не дали результата; это не считается
положительной рецензией или дополнительной независимой проверкой.

## 1. Метод и уровень решения

- Авторские R02…R06 использованы как вопросы и provenance, а не автоматически
  принятые доказательства. Субагенты самостоятельно читали публичные source/spec.
- Руководитель прочитал результаты и отдельно сверил критические функции:
  core STD/uTLS/ShadowTLS adapters, uTLS preset/config/version checks, HY2
  fragmenter/reassembly, QUIC handler/sampler/transport parameters, TUF
  multirepo/equality, proxy/resolver и AOSP owner/family/lockdown ветки.
- Математические counterexamples и простые bounds проверены ручным выводом.
  Выполнение программы не используется как доказательство математики.
- Scientific ACCEPTED ниже означает **только сформулированную область**.
  Current product wiring, local byte equality, compiler/SDK/native/device/field
  behaviour не доказаны. Все experiments и cases — NOT_RUN.

## 2. Адресные решения по результатам

| Исследование | Решение | Принятое содержание и граница |
|---|---|---|
| **DR27-01 / R02-11,12** | **ACCEPTED_SCOPED_SOURCE_MODEL** | Настоящий inner TLS и SS2022 имеют разные confidentiality/identity/PFS/replay границы. Приняты raw-TCP paths и условная property matrix; pin identity уточнена, native UDP не принят по TCP. [Текст](../../transports/COMPOSITION-AND-TLS-CONTRACTS.md) |
| **DR27-02 / R02-16** | **ACCEPTED_SCOPED_PUBLIC_SOURCE** | Requested TLS1.3 floor перезаписывается конкретным Chrome133/non-real-ECH preset; поздний version check использует новый Config. Все presets/REALITY/product не охвачены. [Текст](../../transports/COMPOSITION-AND-TLS-CONTRACTS.md) |
| **DR27-03 / R02-19** | **ACCEPTED_SCOPED_PUBLIC_SOURCE** | Common ConnectionState reconstruction теряет exporter capability; native Upstream сохраняет её. Руководитель также отметил неперенесённый ECHAccepted. Отдельно подтверждён пропуск STD VerifyConnection в ST fallback при указанных предпосылках. Готовое freshness-исправление не принято. [Текст](../../transports/COMPOSITION-AND-TLS-CONTRACTS.md) |
| **DR27-04** | **ACCEPTED_SCOPED_METHOD/REFERENCE** | Stop/restart и observation horizon являются частью оценки; page_label first-N результат не proxy/benign full-session доказательство. WFDefProxy author v1 и artifact source не заменены заявлением о полном PDF review. [Текст](../../transports/PADDING-AND-OBSERVATION-WINDOW.md) |
| **DR27-05 / R04-13** | **ACCEPTED_SCOPED_PUBLIC_SOURCE** | При normal included send old sender→meta2 snapshot получает F+2L. Wire bytes не удвоены, performance effect неизвестен. **R04-14 NEEDS_REVISION**: требуется compatibility/semantic matrix, не blind upgrade. [Текст](../../performance/QUIC-ACCOUNTING-AND-LIMITS.md) |
| **DR27-06** | **ACCEPTED_SCOPED_SOURCE_MODEL** | Fragment arithmetic/uint8 narrowing, receiver allocation и DATAGRAM sent-history ownership требуют разных bounds. Не установлен deployed remote DoS; ordinary peer defaults и allocator/native границы сохранены. [Текст](../../performance/QUIC-ACCOUNTING-AND-LIMITS.md) |
| **DR27-07** | **ACCEPTED_SCOPED_SOURCE/API** | Peer idle raw→parsed→effective различается; явный0 и absent0 в pin не эквивалентны. Clock aging, wake, callback и NAT maintenance не peer liveness. Energy gain не измерен. [Текст](../../performance/QUIC-ACCOUNTING-AND-LIMITS.md) |
| **DR27-08 / R03-15,16** | **ACCEPTED_CONDITIONAL_MODEL** | Finite/historical interval валиден при полном cohort, joint envelope и указанной filtration; β и bridge η отдельно. Не получен future/marginal forecast по одному sampling radius. [Текст](../../adaptation/INTERVALS-AND-POLICY-EVALUATION.md) |
| **DR27-09** | **ACCEPTED_CONDITIONAL_MODEL** | Whole-policy ITT при isolated randomization; отсутствие joint exposure может скрыть даже знак общего rollout-effect. Sequential IPS не запрещён реактивной средой, но требует своих assumptions. [Текст](../../adaptation/INTERVALS-AND-POLICY-EVALUATION.md) |
| **DR27-10 / R05-002** | **ACCEPTED_SCOPED + NEEDS_REVISION** | Принята необходимость различимости при заявленных positive recovery branches; обязательное новое поле/DB не следует. Crash witness уточнён относительно continuation/barrier. Полная минимальность store и ResetAllowed overlap/reuse остаются открыты. [Текст](../../architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) |
| **DR27-11 / R05-007-A,B** | **ACCEPTED_SCOPED_SOURCE_MODEL** | Mapping counts изолированы, conflict guard есть. Representative hash relation нетранзитивна; успешный download не гарантирует все constraints каждого approver. Product bypass вопреки честному byte approval не доказан. [Текст](../../architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) |
| **DR27-12** | **ACCEPTED_SCOPED_SPEC/API** | Revocation observation/knowledge/history, MEM_IMAGE/initialization и installer checksum provenance различаются. R05-004/005 product claims остаются pending. [Текст](../../architecture/RELEASE-EVIDENCE-BOUNDARIES.md) |
| **DR27-13** | **ACCEPTED_SCOPED_PUBLIC_SOURCE/SPEC** | Resolver placement меняет CONNECT authority; bind/translation относятся к actual hop; SOCKS UDP имеет второй relay path. HTTP200-only против RFC2xx — scoped interop mismatch. [Текст](../../observability/PROXY-DNS-NAT64.md) |
| **DR27-14** | **ACCEPTED_SCOPED_AOSP/API** | Capture/lockdown UID sets, family allowance и owner transitions проверены на указанном AOSP snapshot. OEM/device behaviour и все protected subjects не приняты одним proof. [Текст](../../android/UID-ROUTING-AND-OWNERSHIP.md) |
| **DR27-15** | **ACCEPTED_CONDITIONAL_PROOF/SPEC** | Signed nonce/egress не различают некоторые локальные safe/leak worlds; отсутствие утечек требует полноты наблюдения declared scope. [Текст](../../observability/PROBE-PROOF-BOUNDARIES.md) |
| **DR27-16** | **PARTIAL_SEARCH / REFERENCE_ADOPTED** | Руководитель добавил3 primary OONI sources и ограничения quality/time/coverage. Headline numeric inconsistencies не скопированы как точная статистика; systematic current-RF search не завершён. [Текст](../../methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md) |

Эта таблица не принимает остальные claims REPORT целиком и не редактирует
научные статусы автора задним числом. Результаты R02-15/R04-06/R04-10 и
R05-001-A/B из прежних рецензий сохраняются в своей исходной области.

## 3. Важные ограничения и опровержённые расширения

1. **Нет универсального «TLS работает безопасно», выведенного из имени типа.**
   Поле, wrapper, full handshake, authentication scope и consumer проверяются
   по отдельности. Наличие pin не всегда равно PKI name/chain/time policy.
2. **Нет доказательства двукратного bandwidth/RSS или постоянного DoS.**
   Snapshot F+2L, число fragments и byte lifetime имеют свои units/guards.
3. **Нет новой production probability.** Историческая conditional mean,
   marginal mean, future failure risk и whole-system intervention различны.
4. **Не найдено смешивания counts разных mappings.** Слабое свойство agreement
   связано с equality/representative/full policy, а не с выдуманным accumulator.
5. **Не требуется новый cache-basis field во всех designs.** Existing atomic
   cut/journal или ограниченный publisher recipe могут обеспечивать различимость.
6. **Нет вывода об утечке каждого Android UID.** VPN owner/DPM exemptions,
   обычный UID, user/profile и family — разные subjects.
7. **Нет актуальной карты РФ.** Статьи, особенно с поздними обновлениями и
   другой страной, дают dated reference и метод; сейчас внешние VPN пути не измерялись.

## 4. Как дальше получать новое знание

Приоритетный следующий **desk**-срез — сопоставить найденные contracts с
actual current artifact/source manifest, когда владелец предоставит основной
code tree: compiler tags, selected TLS path, sender/controller pair, resolver
placement, product policy и current call sites. Документационный снимок этого
доказательства не содержит; отсутствующее wiring нельзя ни обвинить, ни оправдать.

Чисто исследовательские открытые вопросы: точный reset contract overlap/reuse;
численные bounds producer corruption/completeness; late-handshake
distinguishability в полнооконном future-day корпусе; temporal alignment
DNS/IP observations; ограничения platform image/ownership attestations.
Для каждого — один конечный вопрос, competing explanation и stop criterion.

Future lab/native/field планы в заметках не являются запусками. Не написаны
новые исполняемые tests, не менялись зависимости, провайдерская квота, VPS,
профили, credentials, platform PLAN/STATE или системные настройки.
