# Архитектура защиты, доверие и обновления

[Оглавление](../README.md) · [Термины](../GLOSSARY.md) · [Непрерывность доверия](TRUST-CONTINUITY-AND-AGREEMENT.md) · [Границы поставки](RELEASE-EVIDENCE-BOUNDARIES.md)

## Краткий вывод

Подлинность, свежесть, разрешение действия, сохранение истории и работа
сетевого барьера — разные контракты. Подпись не назначает полномочия сама себе,
ошибка не означает отсутствие допустимого прогресса, а успешный ответ API
не доказывает долговечность записи или защиту существующих потоков.

**Когда применять.** При импорте профилей, обновлении доверия, восстановлении
после сбоя, проектировании защиты и выборе границ платформенного SDK.

**Граница знания.** Здесь объединены принципы и датированные срезы 21–28 октября 2026 года.
Они не утверждают, что целевой ProtectionController, служба, portable SDK или
все native-гарантии уже реализованы. Детали хранилищ и применения находятся
у владельцев. Научное принятие ограниченного вывода не является приёмкой продукта.

Основные вопросы: [защитные принципы](#source-3), [доверие и автономность](#source-7-5),
[существующие потоки Windows](#source-13-9), [выбор реализации TUF](#source-16-3),
[первое независимое подтверждение](#source-16-6), [ошибка и история TUF](#tuf-progress-and-reset-2026-10-27).

<!-- source-unit: 3 -->
<a id="source-3"></a>
## 3. Архитектурные принципы защищённого клиента (выводы из практики)

1. **Защита сохраняется при автоматическом отказе.** Отсутствие разрешённого
   подтверждённого пути не открывает защищаемому трафику прямой выход.
   Явное отключение и ошибка имеют разные переходы по политике платформы.
2. **Готовность имеет область.** Исторический набор DNS+HTTPS+egress+negative
   leak checks был конкретной схемой приёмки, а не универсальным доказательством
   защиты всей ОС. [Точный смысл сетевой проверки](../observability/PROBE-PROOF-BOUNDARIES.md)
   задаёт наблюдаемую область. HTTP101, открытый порт и handshake подтверждают
   свои этапы, но не завершение полезной операции.
3. **Один владелец жизненного цикла.** Контекст времени операции Start/Reload
   не подменяет контекст жизни движка. Отмена наблюдателя и возврат метода
   не доказывают освобождение всей порождённой работы. Старые инциденты Manager
   объясняют причину принципа, а не текущее состояние кода.
4. **Секреты имеют явные роли.** В публикуемых конфигурациях — ссылки и
   placeholders; значения не попадают в Git, чат, журнал и diff. Ссылка на
   секрет не даёт права произвольно раскрыть его или применить к другому каналу.
5. **Проверяется нужный участник.** Для TLS выбирается действительное доверие
   PKI либо обоснованный pin; self-signed не исправляется через `insecure`.
   REALITY и составные транспорты имеют свои роли аутентификации:
   [основной разбор композиции](../transports/COMPOSITION-AND-TLS-CONTRACTS.md).
6. **Разнообразие не равно независимости.** TCP/CDN и UDP на одном VPS сохраняют
   общую машину, адрес и управление. [Граф отказов](../foundations/THREAT-MODEL.md#source-7-3)
   важнее числа строк в селекторе.
7. **Профиль проверяется до исполнения.** Нужны строгая схема, роли ссылок,
   совместимость, отсутствие placeholders и запрет неразрешённого fallback.
   Возможность поля upstream не означает его поддержку нашим рендером и UI.
8. **Транспорт — договор обоих концов.** Mux, заполнение и новые поля требуют
   совместимости клиента и сервера. Успех CDN/TLS/auth не исключает отказ данных.
   Исторический инцидент 12 сентября с h2mux+padding иллюстрировал это различие;
   глобальный default без согласования разнородных серверов из него не следует.
9. **Провал пробы не равен утечке.** Liveness timeout допускает ограниченное
   уточнение с сохранённой защитой. Совпадение IP при общем NAT и различие IP
   не доказывают соответственно утечку и отсутствие всех утечек.
10. **Начальное разрешение имеет отдельную политику.** До запуска туннеля нужны
    допустимые способы получения адреса, границы доверия/свежести и явные
    исключения. Plain DNS, DoH и кеш не назначаются универсально обязательными.
    [Основной DNS-разбор](../observability/DNS-AND-DIAGNOSTICS.md) разделяет
    bootstrap, пользовательский DNS и измеритель.
11. **Бездействие и восстановление учитываются отдельно.** NAT mapping, keepalive,
    прикладной heartbeat и срок канала различаются. Числа 60/20 с/TFO старой
    конфигурации не являются измеренным оптимумом. Parser/check не доказывает
    правильную аутентификацию и фактическую передачу.
12. **Оптимизация имеет цену.** Mux сокращает часть установлений соединения,
    но объединяет очереди и область отказа. Он не скрывает объём автоматически;
    профиль выбирается под нагрузку и проверку обоих концов.

<!-- /source-unit: 3 -->

<!-- source-unit: 7.4 -->
<a id="source-7-4"></a>
### 7.4. Что в коде сейчас, а что ещё целевой контракт

Сверка 2026-10-21, чтение исходников без запуска приложения:

| Место | Фактическая граница | Следствие для разработки |
|---|---|---|
| `windows/backend/render/fdmodel.go` | additive penalties, threshold 0.35; unknown без штрафа; любой ws по умолчанию carrier=cloudflare | Это эвристический rank, не вероятность/доказательство. Явный carrier + coverage/общий origin/control-plane — TH-03 |
| `windows/failclass.go` | классификация в том числе по строкам ошибок/порту; confidence — константы после ограниченного lab tuning | `reset_injected`, `whitelist_mode`, `tls_policing_443` — исторические ярлыки; источник RST/политика allowlist не установлены. Evidence/uncertainty/abstain — TH-02 |
| `windows/go.mod`, pinned `option/v2ray_transport.go` | sing-box **1.13.19**, transport cases http/ws/quic/grpc/httpupgrade; splithttp/xhttp отсутствуют | XHTTP нельзя «включить полем»; отдельное решение по совместимости после TH-04 |
| pinned `option/tls.go` | SPKI pins/fragment есть, TLS spoof из новой docs в этом типе нет | Upstream latest ≠ поставка. Поддержка ядром ≠ signed schema/render/UI; нужна проверка всего пути |
| PLAN OFF-01…09 | есть локальные инкременты, OS/native/live/comparative gates открыты | Здесь не объявляется реализованной production-служба/ProtectionController или универсальный TUN |
| V2-108 | Во время V2-109 появились новые code/evidence: root build failure исправлен, capability отмечена locally-tested | [Отдельное evidence](../../evidence/2026-10-21-binding-migration.md); V2-109 читает сохранённые результаты, не приписывает себе их запуск. Реальные owner profiles/pins не мигрировались |

Это расхождения среза 21 октября, не список доказанных нынешних дефектов.
Актуальное сопоставление выполняется по двум планам и текущим исходникам
владельца. Сам принцип сохраняется: ограниченный срез через существующие
App/Manager/Engine вместо второго владельца.


<!-- /source-unit: 7.4 -->

<!-- source-unit: 7.5 -->
<a id="source-7-5"></a>
### 7.5. Control plane, ключи и автономность

Нужны отдельно доступность профилей, подлинность, свежесть, совместимость и
восстановление доверия. Два mirror с одним key/аккаунтом не дают две независимые
границы доверия. Подписанный вредоносный профиль после кражи publisher key остаётся
подписанным; простая Ed25519-проверка не решает компрометацию издателя.

[TUF 1.0.37, 2026-10-08](https://theupdateframework.github.io/specification/v1.0.37/) — зрелый референс разделения ролей/thresholds,
rollback/freeze/mix-and-match защиты и ротации root. Это не утверждение, что
наш envelope уже реализует TUF. Адаптировать требования и regression scenarios,
исторический вопрос OFF-06 позднее получил решение по ADR-001 и совместимому
go-tuf2.4.2. Это смена этапа решения, не доказательство готовности всех адаптеров.

До выдачи профиля: ChannelID→SecretID→pin/capabilities, min/max client/core
version, auth и signed validity. Rotation: current+next, overlap, отзыв, crash
между stage/commit, clock jump и восстановление из backup. Пин должен приходить
по доверенному пути; TOFU в атакуемой bootstrap-сети не равен установленному доверию.

Автономность не означает бесконечно принимать expired/revoked профиль. Нужны
явная политика last-known-good **в пределах доверия/сроков**, отдельное поведение
ошибочных часов и восстановление из заранее доверенного offline package.
Expiry при недоступном publisher может снизить availability — trade-off
документировать, не скрывать автоматическим отключением проверки.


<!-- /source-unit: 7.5 -->

<!-- source-unit: 13.9 -->
<a id="source-13-9"></a>
### 13.9. Windows: барьер, существующие потоки и время

Microsoft best practices советует dynamic WFP sessions для автоматического cleanup,
но именно удаление filters при завершении процесса может противоречить crash-time
strict protection. Persistence, BFE activation и реальный enforcement — разные
свойства. Boot-time и persistent lifetimes документируются отдельно; из них нельзя
вывести непрерывность при runtime BFE failure. Нужны выбранный service/barrier owner,
ограниченные transport exceptions и external observation. [D21]

При отзыве/изменении правил проверять **уже существующие** TCP и односторонние UDP
flows, не только новые соединения. ALE stateful authorization и policy-change
reauthorization имеют свою семантику; недостающие fields при reauthorization не
заменять доверенными values из прошлого. TUN route removal, filter transaction commit
или enum result сами по себе не закрывают эту приёмку. Kernel driver/callout не
проектируется здесь автоматически: сначала supported user-mode возможности. [D22]

Monotonic не означает одинаковую suspend-семантику на любой платформе. Go time docs
предупреждают, что на некоторых ОС monotonic clock останавливается во сне;
Android elapsedRealtime включает deep sleep (§12.6). TTL grant, trust expiry,
measurement freshness и deadline активной работы требуют своих clocks/boot scopes.
После resume нельзя продлить grant, объявить старый proof свежим или сразу породить
все пропущенные periodic jobs. Отдельно проверить запрограммированные таймеры и
фактическое время callback под CPU pressure. [D23]


<!-- /source-unit: 13.9 -->

<!-- source-unit: 15.3 -->
<a id="source-15-3"></a>
### 15.3. Trust, эксплуатация и повседневный UX: окончательные решения

| Область | Выбранный путь / почему он достаточен для разработки | Конечная приёмка |
|---|---|---|
| TUF/delivery | **Reuse go-tuf2.4.2, ADR-001**; в срезе 24 октября интеграция по исходникам
не имела полного подтверждения. Новая криптобиблиотека не нужна. Delivery authenticity/roles/freshness и application grant/epoch/owner authority различны; TUF не installer EXE/APK | TH-03/08, AND-04/14: golden bytes/negative authority/clock/rotation/revoke/fixed-target atomic commit и реальная установка отдельно |
| Cold bootstrap/authority custody | **Adapt existing issuer/repository/cache.** Initial root/key fingerprint приходит из независимого доверенного канала; download с заблокированного origin не становится новой authority. No safe current trust/cache → BLOCKED/recovery intent | TH-03/07/COM-13: cold latest-known floor, root/threshold loss/rotation, offline backup/restore с внешним approved state; bounded transport bootstrap не снимает expiry |
| Deadline/retry/failure control | **Reuse owner/epoch/actual-return fences**, единые end-to-end deadline и admission budgets; timeout/cancel не завершение underlying work. Safety прежде optional quiet/refresh/probe | TH-01/02/04/RQ/TR/AND-02…07/11: blocked IO/native/late returns, stale result, queue saturation, no overlap/no direct gap; фактическая свежесть и useful completion |
| Один VPS/несколько узлов | **Один anchor достаточен для первого клиента, но остаётся SPOF.** Несколько transports на нём не независимые серверы. Второй узел выбирается после граф общих причин отказа, а не количества стран | TH-03/07/COM-15: фактическая инфраструктура, оплата, стоимость, квоты и маршруты, independent origin/ASN/publisher/resolver по цели; бюджет 12000 р/год не источник гарантии |
| Cloudflare build+sshx recipe | **Reject как постоянный VPS.** Удалённый terminal build job не даёт владельцу зарезервированный always-on gateway/порты/диск/availability contract. Платформенный worker/tunnel требует отдельного supported carrier contract | Нет обязанности разворачивать этот обход CI lifetime; новый официальный продукт/use case потребует отдельного ADR и условий сервиса |
| Мониторинг/диагностика | **Reuse scoped observations + optional blackbox ideas.** Дата оплаты с явным происхождением; provider API read-only только по конкретной пользе. Не ставить тяжёлый observability stack ради пары метрик | COM-07/11/12/15, TH-07: endpoint-dead≠tunnel-dead, stale/unknown distinct, diagnostic bytes/work caps, redacted export preview, без admin key в APK |
| Навигация/каждая страница | **Adapt existing UI/component API.** Windows left menu=раздел, top tabs=его подпункты, без дубликатов. Android отдельный adaptive scaffold. Status/service readiness/scope/freshness объясняются одним snapshot | TH-08/COM-01/02/07/11 и AND-08/09/12: onboarding, home, channels, checks, journal, settings/transfer; loading/empty/error/success/stale, narrow/wide/fonts/keyboard/TalkBack и основные пользовательские задачи |
| Доступность сервиса для работы | **Adapt task profiles/readiness, explicit quiet/portal/grants.** Нет mic/screen surveillance или авто-логина. Обход корпоративной policy не feature; incompatible full-tunnel честно unsupported/managed | COM-04…11/AND-06/09/10: effective route+DNS+identity/explain, TTL/reboot/network revoke, conflicts/portal возврат к protected proof; UX success/time/error/comprehension заранее |
| Release/security | **Reuse signed staged pipeline + platform signer.** Cargo-cult latest dependency, прямой launch downloaded code и unsafe recovery не выбраны | TH-08/OFF-07, AND-14/17: dependency/security/license inventory, reproducible inputs, signer custody, canary/revoke/known-good recovery без trust downgrade; support matrix |
| Optional R&D | **Defer** bonding/multi-hop/PQ/DAITA/Tor/XHTTP/port-forward/enterprise/TV/router, если нет выбранного use case и доказанной предельной пользы | COM-16…19/AND-18: named scenario→both-end compatibility/license→safety→matched benefit/resource comparison; отсутствие сценария — осознанный scope decision |

Эти решения не требуют нового control plane, написания своего transport/crypto
или импорта всей архитектуры upstream. Сохранены все конкретные COM требования
§6 COMPETITOR-FEATURES/PLAN§5.4 и страницы Android§5/§8.3; таблица выше не сокращает их.
Уникальность рынка не установлена. Гипотезы сочетания функций требуют usability и
FR-012 сравнения, даже если отдельные компоненты уже хорошо реализованы upstream.


<!-- /source-unit: 15.3 -->

<!-- source-unit: 16.3 -->
<a id="source-16-3"></a>
### 16.3. OP-03 — portable TUF: конфигурацией diskless режим не получить

Проверены exact go-tuf2.4.2/f5edbde исходники updater/config/trustedmetadata/go.mod,
4 сверенных Git-объекта. Утверждения Android ADR-017 подтверждены:

- DisableLocalCache пропускает запись/создание путей, но loadTimestamp/Snapshot/
  Targets всё равно используют loadLocalMetadata→os.ReadFile. Memory Fetcher
  перехватывает download, а не эти чтения; пустой LocalMetadataDir не изоляция.
- GetTrustedMetadataSet возвращает struct value с pointer/map fields, не deep
  owned snapshot и не все original signed bytes. UnsafeSetRefTime назван
  вспомогательный API upstream для тестирования. Это не отсутствие expiry checks в SDK.
- Lower-level TrustedMetadata имеет **exported instance RefTime**; после New
  его можно установить до update workflow без process-wide clock и Unsafe helper.
  Но этот слой не выполняет сам полный updater workflow/persistence/lookup.
- UpdateTimestamp/UpdateSnapshot могут изменить **внутреннее** промежуточное состояние
  **до возврата expiry/version error**. Это не разрешение сохранить новый rejected
  объект как trusted cache; error также не означает отсутствия допустимого
  predecessor progress. [Уточнение R05-001-A](#tuf-progress-and-reset-2026-10-27).
  Ротация root и rollback-state loading имеют порядок; New(root) self-signature
  check не устанавливает внешнюю authority и не доказывает свежесть конечного root.

В срезе этого анализа PC adapter использовал полный Updater и приватный
рабочий каталог, извлекая retainedState также при Refresh error. Это пример
разделения прогресса доверия и результата приложения, а не принятие всех
состояний при любой ошибке. Временный файловый контракт не становится
автоматически API без файлового ввода-вывода для Android.

| Вариант | Решение и цена |
|---|---|
| Full Updater + memory Fetcher + DisableLocalCache | **Reject как diskless решение:** скрытые local reads остаются |
| Full Updater + явный private workspace | **Reuse PC pattern**, если платформенный owner отдельно принимает FS-контракт; не удовлетворяет текущему запрету ambient/файловый ввод-вывод автоматически. Нужны CE/no-backup, fixed inventory, ownership, bounded reads/writes/cleanup и exact bytes |
| Pinned TrustedMetadata + узкий portable workflow | **Feasible source seam, conditional adapt.** Криптография остаётся upstream; наш workflow обязан покрыть ВСЕ root/rotation/floors/metadata/target hash/length/delegation-policy/expiry/progress шаги и exact-byte ownership. Это не подпись-only verifier и не выбранный готовый SDK |
| Поддерживаемые upstream Storage/Clock hooks | **Предпочтительный full-updater контракт** для жёсткого memory-only требования; нужен принятый API/patch/version, не предположение о наличии. До него либо отдельно одобренный lower-layer workflow, либо blocker сохраняется |

**Конечный выход исследования:** границы имеющихся API установлены для исследованной версии. В exact
full Updater нет config-only diskless режима; lower-layer даёт instance clock,
но требует отдельной оркестрации. Это конечный architecture decision для AND-04-S4,
а не повод искать случайный compatible SFA HEAD или молча ослабить memory-only.
Рекомендация — согласовать один из двух явных seams до написания adapter:
поддерживаемый full-updater Storage/Clock API либо отдельно reviewed lower-layer
полный workflow. Второй AAR, global chdir/clock/logger override, source fork TUF и
UnsafeLocalMode shortcut не выбраны.

**Gate TH-03/AND-04-S4/AND-14:** golden original bytes; новый rejected expired
timestamp/snapshot отдельно от ранее принятого cache и valid predecessor;
retained progress только по разрешённым переходам; same-version equivocation;
root/key rotation, missing roles, target length/hash, фиксированное время начала и fresh
application commit-time отдельно; no ambient FS/network; one artifact/JNI
provenance. Signature-verified intermediate не разрешает профиль или Connect.
[Pinned SDK](https://github.com/theupdateframework/go-tuf/tree/f5edbde31e5507f46db2069402dc38903fe6d9d4),
[TUF1.0.36 §5](https://theupdateframework.github.io/specification/v1.0.36/#detailed-client-workflow).
Этот анализ не заявляет conformance новой спецификации или готовность SDK.


<!-- /source-unit: 16.3 -->

<!-- source-unit: 16.6 -->
<a id="source-16-6"></a>
### 16.6. OP-06 — first policy SHA и custody нельзя получить из того же пакета

PC V2-144 требует независимо подтверждённые root SHA и cumulative-policy SHA;
это закрывает один cold-enrollment gap. Хеш пакета, полученный тем же путём,
обеспечивает сравнение bytes, но не независимую свежесть. Даже подписанный
baseline не доказывает отсутствие более нового отзыва в мире. Глобально latest
и latest-known-to-custodian имеют разный scope.

**Adapt существующий issuer/transfer UX**, не новый trust protocol: два digest
можно представить одним reviewable public record/QR с явными issuer/purpose/
recipient/scope/as-of/version. Это лишь представление уже обязательных approvals;
не вычислять approval автоматически из inbound bundle. Authority record должен
быть получен по доверенному независимому каналу; QR сам себя не аутентифицирует.
Никакого нового production wire field этим исследованием не вводится.

Custodian receipt хранит точную идентичность одобренного baseline/checkpoint, где/когда
проверен и для какой enrollment/recovery операции; publisher backup checksum не
подменяет receipt. Потеря checkpoint/key или uncertain clock → существующий
recovery/blocked, не fresh-install reset старого устройства. Копирование root
без policy и перенос старого private floor должны иметь разные явные outcomes.

**Gate TH-03/07/COM-13, AND-04/13/14:** валидный old package с omitted revocation,
задержанный policy record, same-channel hash substitution, wrong recipient,
ошибка копирования 128hex/QR, потеря key/checkpoint и backup restore. UX task:
пользователь понимает, что он подтвердил и откуда взял reference. Тут нет нового
SLO или результата user testing; TUF fixed workflow time не trusted wall clock
и не разрешение позднего application commit.


<!-- /source-unit: 16.6 -->

<a id="tuf-progress-and-reset-2026-10-27"></a>
## TUF: ошибка, сохранённая история и разрешённая ротация

**2026-10-27.** R05-001-A и ограниченная часть R05-001-B приняты после
[независимой сверки](../research/reviews/2026-10-27-R05-001-PROGRESS.md)
TUF 1.0.37/1.0.36 и публичный go-tuf v2.4.2
(`f5edbde31e5507f46db2069402dc38903fe6d9d4`). Это уточнение OP-03.
Поведение текущего адаптера Snowden и durable storage не установлено.

### Четыре состояния, которые нельзя объединять словом verified

| Состояние | Допустимый смысл | Следствие для retained history |
|---|---|---|
| Получен новый timestamp/snapshot, подпись корректна, но final expiry/version check не пройден | Новый объект отвергнут workflow; lower-level pointer мог уже измениться | Не превращать этот объект в новый accepted floor только по pointer/error |
| Timestamp/snapshot ранее принят, теперь его срок истёк | Не даёт свежего target, но хранит ранее известные версии | Сохранить rollback history до отдельного разрешённого reset; expiry сам по себе не очищает floor |
| Timestamp принят на своём шаге, следующий snapshot/target/application шаг завершился ошибкой | Допустимый предшествующий progress | Ошибка поздней стадии не отменяет уже принятое знание |
| Проверена последовательная root chain, затем конечный root оказался expired | Lineage prefix разрешён, финальное обновление запрещено | Root сохраняется по своему порядку; это не повод принять expired timestamp или профиль |

**Нормативное основание:** [TUF §5.4](https://theupdateframework.github.io/specification/v1.0.37/#update-timestamp)
и [§5.5](https://theupdateframework.github.io/specification/v1.0.37/#update-snapshot)
проверяют новый timestamp/snapshot до принятия/записи. Для root
[§5.3.4–10](https://theupdateframework.github.io/specification/v1.0.37/#update-root)
задаёт dual-threshold, строго N+1, сохранение intermediate и лишь затем проверку
срока конечного root. Общее правило «сохранять всё при error» ошибочно так же,
как «при error никогда нет progress».

**Основание по исходникам:** `TrustedMetadata.UpdateTimestamp/UpdateSnapshot` меняют
указатели до final checks, чтобы поддержать загрузку старого cache для rollback
protection. Но `Updater.loadTimestamp/loadSnapshot` сохраняют **новые downloaded**
bytes после успешного `Update*`. API snapshot не сообщает сам по себе, каким
путём объект туда попал. Нужны original bytes, роль, происхождение, исходный
checkpoint и завершённый шаг. Повторный workflow не должен наследовать
непринятые мутации только потому, что используется тот же объект SDK.

### Монотонность не равна общему max всех чисел

TUF 1.0.37 §5.3.11 после проверки свежести конечного root предписывает удалить
trusted **timestamp и snapshot**, если timestamp и/или snapshot keys были
ротированы. Обновление только root version не означает такую ротацию.
Удалить лишь snapshot при смене snapshot keys недостаточно: его version floor
может остаться в timestamp.

Условный пример t5→snapshot100 и разрешённой смены signer, а также
отличие полной замены от перекрытия ключей разобраны в основном разделе
[непрерывности доверия](TRUST-CONTINUITY-AND-AGREEMENT.md).
[Первичная рецензия](../research/reviews/2026-10-27-R05-001-PROGRESS.md)
сохраняет границы источников. Увеличение root version само по себе не разрешает
reset; «оба ключа заменены» не тождественно всем поддерживаемым ротациям.

**Проектная граница:** TUF reset не сбрасывает application profile/update floors,
epochs, cumulative bans, enrollment/custody и protection intent. Новый root,
обязательство reset и cache basis должны оставаться согласованными после
ошибки/рестарта. Способ хранения этого различия — предмет отдельного
[R05-002](../research/05-TRUST-SAFETY/handoffs/R05-002.md), а не обязательное новое
поле по итогам этой рецензии. Если root prefix сохранён, но final freshness
не пройдена, следующий шаг не может считать reset уже разрешённым и завершённым.

### Три разные проверки времени и сохранения

[TUF §5.1](https://theupdateframework.github.io/specification/v1.0.37/#fix-time)
фиксирует время **начала workflow**, чтобы metadata не истекла посередине
одного обновления. Это не доказательство правильных UTC-часов и не продление
прикладного профиля. Перед применением/запуском нужны собственные действующие
application deadlines и owner/source gates. Отказ обновления metadata сам по
себе не делает текущий ещё-допустимый data plane неисправным.

Принятие verifier, фактическое сохранение и применение приложением — разные
события. Успешный возврат или readback файла не доказывают сохранность после потери питания;
timeout не доказывает отсутствие записи. Pure API с единственным возвратом не
может подтвердить сохранение volatile root progress при hard kill до возврата.
Per-step durability требует реальной границы передачи store; per-return promise
имеет более узкую crash область. Выбор этого контракта пока открыт.

### Конечная приёмка существующего trust boundary

Координаты прежних требований — TH-03/FR-008/011 и отдельный Android-реестр.
Сначала согласуются точные adapter/publisher и поддерживаемые ротации;
исполнение требует своей приёмки. Все случаи ниже **NOT_RUN**.

| Случай | Ожидаемое различие |
|---|---|
| Новый expired t1000 после принятого t10; затем обычный t11 | Не создавать floor1000 из rejected объекта |
| Ранее принятый t10 теперь expired; получен fresh t9 | Expiry не разрешает обычный rollback |
| Valid timestamp, затем bad snapshot hash/expired snapshot/ошибка application | Сохраняется только допустимый predecessor; artifact/профиль не применяется |
| Intermediate root expired, следующий fresh; отдельно final root expired | Lineage и финальная свежесть имеют разные outcomes |
| Fresh R8 меняет только snapshot keys, T прежний | Проверить согласованный reset обоих TUF cache и S-only contract |
| Paired T/S rotation; отдельно overlap/старые подписи под новым quorum | Не считать два разных publisher recipes одним case |
| Root version вырос без ротации ключей timestamp/snapshot; либо root signatures неверны | Нет blanket reset по большему номеру или недоверенному флагу |
| Crash между root persistence, final check и reset | Не воскресить старый cache и не объявить незавершённый reset выполненным |
| Срок application grant истёк во время записи либо исход записи unknown | Retained history не откатывается ради запуска; Ready не публикуется |

Reuse выбранной криптографии и существующего store остаётся исходной позицией.
Полная conformance, ресурсы, native durability и recovery конкретного продукта
потребуют своих inputs и evidence; они не следуют из научного принятия A/B.

[D21]: https://learn.microsoft.com/en-us/windows/win32/fwp/object-management
[D22]: https://learn.microsoft.com/en-us/windows/win32/fwp/ale-re-authorization
[D23]: https://pkg.go.dev/time#hdr-Monotonic_Clocks

## Применение в двух планах

Для [ПК](../../planning/knowledge-application/PC-PLAN.md) основные этапы —
PC-01 (барьер), PC-02 (доверие) и PC-08 (поставка/восстановление).
Для [Android](../../planning/knowledge-application/ANDROID-PLAN.md) —
ANDR-01/04/09 с отдельными полномочиями ОС и артефактом JNI/AAR.
Универсальная криптографическая часть не получает владельца WFP, VpnService,
файловой долговечности или применения профиля.

Переиспользуются выбранная библиотека проверки и существующие issuer/store/
импорт. Выбор между полноценным Updater с явным файловым контрактом и
проверенным lower-level workflow зависит от требований платформы. Цена второго
варианта — полная оркестрация ролей, делегирования, сроков и сохранения истории;
она не исчезает из-за использования готовой криптографии. Лицензия, точные
версии и итоговый состав фиксируются до переноса кода.

Текущие статус реализации и разрешённая приёмка находятся в планах владельцев.
Эта редактура не выполняла OS/device/live или проверку потери питания.
Сохранены как открытые проверка долговечности, реальных полномочий,
поддержанных ротаций и защищённого применения.
