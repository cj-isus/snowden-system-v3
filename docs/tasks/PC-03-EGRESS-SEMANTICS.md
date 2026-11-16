# PC-03 — сравнение egress и граница локальной безопасности

2026-11-04, Windows, V2-199/V2-199R/V2-207 **source/unverified**. Часть полного
[PC-00…09](../planning/knowledge-application/PC-PLAN.md), не замена остальных этапов.

## Источник дефекта и принятый контракт

`directLeakResult` раньше считал совпадение IP в SOCKS и расхождение в TUN
подтверждённой локальной утечкой; обратные варианты называл отсутствием утечек.
Одинаковый NAT/другой VPN/перевод адресов и WFP AppID exception делают этот вывод
некорректным. `Expected egress` mismatch также не устанавливает причину маршрута.
Основание — [границы доказательства probe](../theory/observability/PROBE-PROOF-BOUNDARIES.md)
и PC-03 сводного плана; новых сетевых экспериментов не проводилось.

## Изменение источников

- Существующий `ProtectedProbe` и native worker остаются владельцами проверки.
  `probe_comparison.go` выделяет value-only модель/сопоставление без нового
  controller, fetcher, TLS policy или store. Семантическое равенство IP использует
  `net.IP.Equal`; формат IPv6 не становится ложным расхождением.
- На expected/consistency/control шагах передаётся schema1 comparison:
  `reference`, `outcome=match|mismatch|unknown`, remote HTTPS observation point,
  topology unknown, path unattested, local safety unknown, violation basis none.
  Это описание ответа, не native proof и не forwarding permission.
- Expected mismatch/расхождение полных ответов остаются отказами диагностики.
  Недостаточно пригодных данных — явный unknown/unavailable шаг; некорректный
  diagnostic plan по-прежнему отвергается на входе как configuration failure.
  OS control считается IP-сравнением только после полного HTTP200 observation.
  Ни один из этих производителей не выставляет `LeakConfirmed`/`status=leak`.
- В SOCKS режиме пройденное сравнение означает только факт сравнения. Прежний
  required control сохраняет отказ при недостающем наблюдении; это не обход
  gate по missing data. TUN same-image control остаётся unavailable независимо
  от relation, с ProtectionUnavailable и без worker token. Original distinct
  helper/WFP/private proof/final Manager path по W4c ещё не подключён.
- Worker bound/clone и Go projection сохраняют comparison как value, reject
  malformed schema. V199R добавляет одну value-only `NormalizeComparison` для
  соответствия machine step/reference/schema/outcome: unknown не PASS,
  expected/consistency mismatch — отказ, IP-only leak label — unavailable.
  Metadata другого шага не принимается. Тот же ограничивающий вызов выполняется
  в private `probePassed` до token/Manager admission, не только в UI conversion;
  функция не превращает false в true и не меняет оригинал отчёта.
  Новая UI metadata optional; namespace/generated Wails не
  правятся и не генерируются. Статические типы и будущая генерация/совместимость
  старой сборки требуют отдельной приёмки.
- В существующем API mapper один decoder для state/manual probe/test reports.
  Отсутствующие/неподдержанные completed comparisons — unavailable; старый PASS
  не выдаёт privacy verdict. Pending/running/skipped остаются своими состояниями.
  Отрицательный исход диагностики не превращается в успешный из-за нового mapper.
  В V199R decoder явно обрабатывает не-массив и некорректный элемент, reference
  связывается с именем шага; unknown/mismatch и IP-only leak не сохраняют PASS.
  Общий `probeReportOutcome` ограничивает aggregate по failed/incomplete steps
  и сохраняет уже отрицательный итог. Используется mapper и TestCard, включая
  presentation потребителя без native mapper. All-skipped остаётся «пропущено».
- `ProbeReportView` переиспользован в Dashboard, TestCard и Statistics. Detail и
  scope видимы текстом, не только title/color. Шумные live-region на polling не
  добавлялись (modern-web-guidance accessibility §8). Новых страниц, cards,
  router/store, dependencies, mock или test fixtures нет. Дубли list styles удалены.
- Главная использует конкретный Expected egress, а не первый consistency шаг;
  утверждения «IP скрыт/утечек нет» заменены названными результатами и unknown.
  Тексты страницы диагностики/адаптации не обещают native DNS, UDP, чужой VPN
  или отсутствие всех утечек по HTTP. Статус running и native enablement не
  создаются этими presentation данными.

## Границы и следующий шаг

Source/gofmt/manual diff — не compile, runtime, browser/WebView2 или path proof.
V199R: изменены 6 Go/3 TS/6 Vue/4 docs; gofmt4 exit0 при продолжении, оригиналы,
source/docs diff и identity snapshot — `.artifacts/memory/egress-semantics-*`.
Тесты/fixtures/build/typecheck/parser/SDK/helper/native/admin/live запрещены и
не выполнялись. UI компоненты диагностики редактировались как продуктовый код;
сами диагностические действия не запускались. All PC-00…09/old TH-00…08 OPEN.

W4c требует актуального App owner/intent/network/trust scope, compiled helper
pin, bounded original capture и expected egress, private native receipt после
полного retirement, затем actual outbound verdict/короткий one-use permit и
fresh Manager phase commit. Comparison нельзя использовать вместо этого.
Отдельны bootstrap/cache/IPv4/IPv6/UDP/прикладной путь, native local-safety producer,
policy continuity/ABA, server inventory, все страницы UX и последующая приёмка.
Публичный schema2 install остаётся unsupported; TUN admission остаётся закрытым.

## V2-207 — failure classification и обработка safety report

Source-аудит обнаружил отдельные дефекты потребления уже введённой модели:

1. `classifyObservedFailure` выдавал divergence для любого непройденного
   Expected/Consistency шага, включая unknown/unavailable/неподдержанную схему.
   Теперь для divergence нужны normalized required `status=fail`, valid
   comparison нужного шага и `outcome=mismatch`. Missing measurements остаются
   observation unavailable. Реальный mismatch не теряется из-за другого
   недоступного шага; local trust/protection/cleanup не становятся network fact.
2. `NormalizeDiagnostic` сохранял comparison при недопустимой роли; следующий
   `NormalizeComparison` снова делал из unavailable mismatch. При role rejection
   comparison теперь сбрасывается. IP-only `status=leak` также инвалидирует своё
   comparison, чтобы следующий consumer не восстанавливал rejected verdict.
   Независимый leak после comparison validation сохраняется как required/failed
   даже при неправильной роли. Никаких положительных фактов это не создаёт.
3. Watch/network сначала вызывали suspected publication, которая намеренно
   отвергает unsafe report; потому настоящий initial mismatch мог не попасть
   в последующий stop. Общий `blockUnsafeObservation` теперь вызывается до этого
   gate и после fast-reprobe. Сама серия возвращает первый safety report без
   следующего ожидания. Manual probe также останавливает по mismatch/cleanup,
   сохраняя собственный operation lock и прежние scope/Manager guards.
4. Publication и observed candidate ranking берут класс из actual normalized
   report. Local protection/cleanup/trust не пишут health FAIL. Пустые,
   неподдержанные и legacy causal codes воздерживаются от такой записи;
   действующие typed отрицательные channel facts — leak и egress mismatch.
   Это не переносит событие измерителя в диагноз отказа транспорта или DPI.

Ограниченные попытки восстановления после серии непройденных проверок сохранены.
Их счётчик и TTL-порядок кандидатов означают стоимость неуспешной работы, не
новый channel-health факт и не quarantine. Нет кандидата/исчерпан budget —
отсутствие принятого пути восстановления, а не доказанная недоступность всех
сетевых путей. Тексты затронутых ошибок различают эти причины. Unknown не
становится PASS, не обновляет successful probe time/health и не даёт допуск.
Legacy API `BlockedReason=all_channels_failed` сохранён как compatibility token
исчерпанного восстановления; он не добавляет измерение всех каналов.

Изменены 8 Go/4 docs; gofmt8 файлов, manual source/diff/bytes-SHA only.
Исходные 12 файлов и текущие source identities:
`.artifacts/memory/measurement-policy-source-evidence.json`.
Tests/fixtures/build/compile/vet/typecheck/parser/check/SDK/helper/native/browser/
admin/live не добавлялись и не запускались. Изменения не скомпилированы;
race/конкуренция/OS/health/quarantine/recovery поведение остаётся непроверенным.
IPC8 и compiled native acceptance const=false не менялись; TUN unavailable,
schema2 unsupported. Весь PC-00…09/old9 и предыдущие критерии остаются OPEN.
Следующий source-срез — bootstrap/cache identity, роль endpoint, profile revision,
network epoch, freshness и отдельные IPv4/IPv6/UDP/TCP/path результаты.
