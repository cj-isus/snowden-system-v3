# Архитектура, интеграция и один VPS

Исходная аналитическая часть плана от 28 октября: границы и решения,
не требующие новых исследований. Итоговая очередь — в [плане ПК](../PC-PLAN.md)
и [плане Android](../ANDROID-PLAN.md); более новые изменения не откатываются
к этому source-срезу.

## Исходное состояние и существующие границы

| Область | Что видно в текущих источниках | Последствие для плана |
|---|---|---|
| ПК | Windows Go/Wails backend, Vue frontend; владельцы lifecycle/core/render/metadata/delivery/secretvault уже разделены по пакетам | Менять существующие seams; не строить второй engine/controller параллельно |
| Android | Собственный app/service/native слой, исходные overlays и отдельный AND/A-FR план; source changes не удостоверяют полученный AAR/APK | Отдельный Android поток, source→artifact→device gates; не переносить Windows PASS |
| Адаптивный слой | В shared существует adaptive; его роль ограничена чистой policy, evidence/recommendations/budget semantics | Замеры, текущая защита, trust и исполнение остаются у платформ; actual integration каждого caller проверяется отдельно |
| Общий trust SDK | Есть предложение роли/извлечения; в прочитанном registry trust пока не зарегистрирован | Сначала минимальный контракт/существующий код и ownership prerequisite; отсутствие роли не лечится записью в чужие пакеты |
| Сервер | configs содержит публичные baseline templates и scripts; README отдельно описывает отличия исторического сервера | Поднять воспроизводимый сервер по согласованной compatibility matrix, а не считать template копией старого VPS |
| Эксплуатация | Infra роль в прочитанном registry отсутствует; будущий VPS ещё не предоставлен | Перед исполнением установить владельца/границы; текущая работа только source/plan |
| Данные | Отдельная продуктовая PostgreSQL/control-plane система не установлена приведёнными источниками | Не добавлять БД из названия инфраструктурного чата; сначала доказать нужный state/use case |

Источники: [карта существующих пакетов](../../../MAP.md), [общий план](../../../PLAN.md),
Android план — `docs/ANDROID-PLAN.md`, adaptive API — `docs/adaptive/API.md`,
предложение trust — `docs/tasks/FIVE-STREAMS/TRUST.md`,
[серверный baseline](../../../../configs/README.md), `.harness/ownership.json`.
Это source-срез; действующие разработчики продолжают менять свой код.

## Решения, уменьшающие зависимость потоков

1. **Один владелец фактической операции.** Платформа допускает переход, выполняет
   его и подтверждает результат. Shared policy рекомендует, а не переключает
   сеть/ключи/защиту самостоятельно.
2. **Согласовать небольшой контракт раньше полной реализации.** Exact evidence,
   identity/intent/epoch, error/unknown и lifetime scope должны иметь однозначный
   смысл. Не требуется закончить весь SDK, прежде чем платформы смогут улучшать
   собственные lifecycle и truthful UI.
3. **Повторять проверки на границе использования.** Старое свидетельство не
   заменяет current admission. Physical resource receipts, statistical budget и
   operation authority — разные обязательства.
4. **Разделять блокирующие зависимости и полезные дополнения.** Отсутствие
   вероятностного producer не запрещает безопасный deterministic recovery;
   отсутствие native artifact не позволяет назвать Android feature реализованной.
5. **Один интеграционный план.** Платформенные PLAN сохраняют свои requirement IDs
   и фактические статусы; общая сводка задаёт изменения/стыки, без переписывания
   чужих статусов и цепочки пересказов между чатами.

Основания: [policy/evidence](../../../theory/adaptation/POLICY-AND-EVIDENCE.md),
[неполные наблюдения](../../../theory/adaptation/INTERVALS-AND-POLICY-EVALUATION.md),
[trust continuity](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md),
[проверенные ограничения R28](../../../theory/research/cycles/2026-10-28-contracts/REVIEW-01.md).

## Предварительная последовательность

| Этап | Содержание | Условие выхода |
|---|---|---|
| Сопоставление | Все знания → existing code/plan → disposition, включая поздние и архивные уникальные разделы | Нет скрытых пропусков; conflicts/unknown обозначены, proposals не выданы за implemented |
| Общие стыки | Frozen semantics для evidence/authority/intent/lifetime; минимальные SDK extraction и ownership prerequisites | Платформенные части могут независимо реализовать свои изменения; один ответственный за каждый seam |
| Safety/lifecycle/trust | Закрытие конкретных существующих source gaps, сохранение strong guards и совместимости старых клиентов | Reviewable component changes и явные непройденные native/OS gates |
| Артефакты и приёмка | Source/core/overlays/bindings → release artifact → platform/device proof | Только после отдельного разрешения соответствующих проверок; source-only completion недостаточно |
| Один VPS | Воспроизводимая конфигурация/совместимость, staged apply, backup/rollback и проверяемые service invariants | VPS куплен и доступ предоставлен; trust/operations admission и execution явно разрешены |
| Адаптация и удобство | Доказанные producers/граничные условия → bounded policy; нужные повседневные функции | Измерения и UI соответствуют своему scope; нет выдуманного future-risk confidence |
| Сравнение | Скорость/восстановление/защищённые задачи в заданных условиях | Разрешённые реальные измерения, известный denominator/нагрузка/среда, воспроизводимые ограничения |

Это порядок зависимостей, а не финальный список уже принятых fixes. Некоторые
platform changes могут идти параллельно, если не меняют общий контракт.

## Минимальная организация разработки

- **ПК** и **Android** — два основных независимых владельца платформ.
- **Shared trust/adaptive** — отдельные существующие владельцы только своих
  пакетов; изменения общих contracts согласуются одним интеграционным решением.
  Не заводить новые Android 2/3 только ради распределения строк.
- **Сервер/эксплуатация** — существующий инфраструктурный поток в момент готовности
  серверного этапа. Обвязка подключается к доказанным registry/release prerequisites.

Число исследовательских проверок не определяет число будущих одновременно
работающих разработчиков. При финальном распределении каждая задача получит
непересекающиеся файлы, конкретный вход/выход и ограниченные зависимости.

## Контур одного сервера

Один VPS — единственный узел отказа. Разные transports одного IP/ASN не становятся
независимыми резервными серверами. Цели первого контура: корректная защита,
совместимость, управляемые ресурсы, восстановление после сбоя и повторяемая поставка.
Абсолютную безотказность или превосходство без измерений такой контур не доказывает.

План эксплуатации должен содержать source/config identity, безопасную загрузку
секретов, staged validation, сохранение предыдущего рабочего состояния, clear
commit/rollback outcome, bounded diagnostics и recovery. Existing deploy/verify
scripts оцениваются статически; никаких SSH/deploy/check сейчас не выполнялось.

База данных, сервис аккаунтов, подписки, централизованная телеметрия и отдельный
control plane добавляются только при подтверждённом требовании. Публичные manifests,
подписанные профили и platform-owned local state сами по себе не требуют нового
сетевого backend. Конкретную достаточность выбранного distribution варианта
подтвердит сопоставление trust/delivery и one-VPS частей.

## Структура знаний

Тематическое оглавление теперь показывает содержание и источники. Устаревшие
организационные страницы и задания закрытого цикла вынесены в `docs/archive/`;
полезные REPORT/SOURCES/краткие аргументы и научные ограничения сохранены.
Исходные bytes также доступны в frozen snapshots для полного coverage.
Новые задания/receipts/status/машинный census находятся только в `.artifacts/`.
Это позволяет сокращать читательский слой без потери исходных утверждений.

## Проверка утверждений против текущего кода

Источник этого сопоставления — существующие главы и датированные рецензии,
а также выбранные объявления/ветви актуального дерева 28 октября. Последние
прочитанные снимки: Windows V2-185 (selected-file input/output owner и checked retirement),
Android AND-02-S7/AND-05-S4 (terminal callback admission и same physical binding около capability/establish; прежние
AND-02-S5 callback guards сохранены). Это source-unverified изменения компонентов,
не доказательство принятого AAR/APK или runtime. Windows selected-output
Sync/Close/partial outcome, Android socket OS-step admission и общие artifact/
durability/native/device gates остаются открытыми.
Параллельная работа владельцев требует повторной сверки конкретного seam перед
реализацией. Наличие выбранного файла не означает аудит всего пакета.

| Знание и источник | Что уже есть / расхождение | Решение для разработки |
|---|---|---|
| [Историческая матрица удобства](../../../theory/ux/COMPETITOR-FEATURES.md) | В `windows/tray_windows.go` есть systray и HKCU Run; `onboarding.go:88` экспортирует QR; `updateapp.go:149` содержит ApplyUpdate. Сентябрьские «нет трея/QR/обновлений» устарели | **Доработать существующее** по COM-01/02/03/13/14. Для трея нужны согласованные applied/error состояния и native UX; для переноса — device/revoke/размер/ownership; для update — текущие pair/durability gates. Не создавать повторные подсистемы |
| [Process split и рабочие сценарии](../../../theory/ux/COMPETITOR-FEATURES.md#comfort-research) | `backend/render/render.go:147` уже создаёт process-name правила только для непустого split. DNS hijack и private/LAN исключения имеют собственный порядок | **Сохранить guard и уточнить policy**, COM-04/05/06: selector identity, priority, expiry, DNS scope, поддерживаемая платформа. Process-name rule не доказывает whole-app/per-site coverage или совместимость с любым корпоративным VPN |
| [Physical network events](../../../theory/roadmap/RESEARCH-HANDOFF.md#source-15-4) | `netwatch.go:211` пропускает watcher при tunRequested. В go.mod уже есть pinned SagerNet/tailscale; он ещё не подключён как physical monitor этим caller | **Адаптировать один существующий monitor**, COM-03/TH-02: bounded callback→coalescing→epoch→Manager admission, unregister/late events. Сохранить fallback наблюдений и ownership; не импортировать новый tailscale HEAD/daemon |
| [Качество измерений](../../../theory/methodology/RESEARCH-METHOD.md#source-13-7) | `core.ProbeObservation` разделяет route/stage/error/payload limit/completion; `AdmitDiagnostic` резервирует work до выполнения, защищённая lane отделена, ошибка не возвращает расход | **Расширять существующие producers**, TH-02/04/06, COM-07/11: typed units и acquisition scope, failed/censored outcomes, зависимость endpoints. Не переименовывать payload в wire traffic и короткий probe в speed test; не заменять rolling budget token bucket без дефекта |
| [Padding и окно наблюдения](../../../theory/transports/PADDING-AND-OBSERVATION-WINDOW.md) | Сохранённые papers и artifact устанавливают stop/restart/first-N ограничения оценки, не текущую неотличимость Snowden | **Отложить новый padding preset**, TH-05/E-04/E-08. Сначала совместимость текущего carrier и явный observer/workload; отдельно учитывать tail-cover, reset quota и useful completion. Website-label результат не становится proxy classifier |
| [Анти-знание и старые transport proposals](../../../theory/roadmap/RESEARCH-HANDOFF.md#source-4) | Исторические WS400/EOF и mux истории не локализуют текущий отказ по одному признаку. Старое «без Vision» уточнено upstream closure review; наличие исходника не включает flow у обеих сторон | **Сохранять совместимый baseline**, TH-03/05/07; version/capability/config/caller/client-server matrix раньше переключения engine/mux. Не чинить историческую причину без актуального source или наблюдения |
| [TUF и operational evidence](../../../theory/research/cycles/2026-10-28-contracts/REVIEW-01.md) | В Windows уже много custody/current-source/final-commit guards; SDK ещё не зарегистрирован. Shared adaptive API1 не владеет trust/OS execution | **Извлекать минимальный contract**, TH-01/03 и AND-04: accepted history, exact immutable evidence, current operation admission, scoped clock/error. Не считать reference/CAS disk commit, не вводить новую БД из necessity argument |

### Изменения, связанные с одним сервером

`configs/README.md` прямо отделяет минимальный template от исторического полного
сервера. Это причина построить новую compatibility matrix, а не восстановить
старые порты/домены вслепую. Уже наблюдаемые source-разрывы:

- `deploy-vps.sh:80–82` передаёт один SSH_OPTS с `-p` и в ssh, и в scp.
  Параметры команд нужно разделить; нестандартный SCP port не задан как `-P`.
- `:82–87` копирует прямо в рабочий REMOTE_CONFIG_PATH до `sing-box check`.
  Нужен уникальный staging, проверка точным server binary, сохранение old state,
  commit/rollback с явным outcome. Текущий скрипт не реализует этот transaction.
- `verify-server.sh` ожидает исторический набор listeners, а README описывает
  baseline с меньшими capabilities. Ожидания должны выводиться из выбранного
  server profile/client descriptor, а service-active/listening не закрывают
  protected client→exit→target path.

Это source-only основания для TH-07/FR-005/006 и будущего infra владельца.
Никакого текущего server health или испробованного rollback из них не следует.
Перед исполнением нужны купленный VPS, доверенная server identity, полномочия,
secret custody и отдельное разрешение запуска. Подготовка конфигурации может
быть самостоятельным выходом; runtime-часть не маскируется проверкой текста.

## Что заимствовать и какую цену учитывать

Сохранённый [каталог готовых реализаций](../../../theory/ux/COMPETITOR-FEATURES.md)
содержит exact pins/лицензии; это входные данные выбора, не разрешение нового
dependency upgrade или полный transitive licence audit.

| Решение | Вход / цена и граница |
|---|---|
| Reuse existing sing-box 1.13.19, tailscale/wf, systray 1.2.2 | Уже в windows/go.mod. Расширять schema/render, единственного WFP owner и tray presentation. Source capability не гарантирует native behaviour; GPL/BSD/Apache notices и composed artifact SBOM сохраняются |
| Adapt pinned SagerNet/tailscale netmon | `v1.92.4-sing-box-1.13-mod.9`, сохранённый commit 867c1f0012d915fa5fd57481dbf2a1ad16669fe7, BSD-3-Clause. Нужны caller lifecycle, concurrency/coalescing, physical/TUN distinction; callback API не результат OS acceptance |
| Reuse go-tuf/v2 2.4.2 | Закреплён в windows/go.mod, Apache-2.0. Переносимый verifier требует ограничения fetch/persistence/clock/progress и platform-owned commit; HEAD Go1.27 не выбран, UnsafeLocalMode не средство recovery |
| Reference WireGuard/IVPN/Mullvad/Amnezia/Proton | Заимствовать конкретные rule/lifecycle/UX решения после file/licence review. Dynamic WFP session не подмена persistent crash barrier; Rust/C#/Qt/driver/service stack не переносится ради одной функции |
| Reference blackbox_exporter и histogram semantics | Развивать текущий bounded probe/aggregation, без обязательного второго HTTP prober, Prometheus/Grafana/SSRF endpoint на одном VPS. Метки без URL query/session IDs/app inventory |
| Defer bonding/Glorytun/новое ядро/DAITA/ML | Shared server anchor, POSIX/native stack, расход cover/батареи/памяти и модельные предпосылки требуют отдельного use case. One-VPS baseline и deterministic safe recovery достаточны для следующей стадии |

## Контракты проверки вместо магических настроек

Все случаи ниже **NOT_RUN**. Они уточняют существующие E/TH/COM/AND/FR-012;
не создают разрешение на стенд, device, live или публикацию.

1. **E-01 / safety:** source invariant → процесс → Windows OS / Android device.
   Current intent/owner/epoch и old-flow/cleanup не заменяются успешным новым
   HTTP probe. Capture-or-block требуется для заявленных subjects/families.
2. **E-02/03/06 / причины отказа:** фиксировать stage, direction, application /
   unique transport / wire units, equivalent object, cache/reuse, фактический
   endpoint и контролируемый отказ измерителя. 16KB/25 packets/не-443 не presets.
   Budget-limited inconclusive не считается отсутствием блокировки.
3. **E-04 / camouflage:** schedule×stop×restart×quota/reset×observer horizon.
   First-N и full-window сравниваются без внутренних oracle features; future-day
   и held-out transport отделены. Cover overhead и tail после задачи не скрывать.
4. **E-05/08 / полезность:** paired useful completion/recovery/loaded latency /
   goodput при известных independent blocks; failures, cancellations, missing
   clocks и удержанные ресурсы входят в полный denominator. Не усреднять p99,
   не применять iid zero-failure bound к зависимым попыткам.
5. **TH-07 / сервер:** offline profile compatibility → owned staging/rollback →
   разрешённые server invariants → защищённый client path → task. Один IP/ASN
   остаётся common failure domain; две transport строки не резерв двух серверов.
6. **COM / удобство:** конкретная задача onboarding/work/LAN/portal/quiet/update,
   observation→admission→effect→task с понятным unknown/blocked/recovery action.
   Quick surface использует тот же controller; quiet запрещает optional churn,
   сохраняя обязательные safety/revocation действия. Не обещать seamless bonding.

Радиуса confidence interval недостаточно для future-risk обещания. Cohort,
envelope/filtration/численный kernel, deterministic hard authority и prospective
bridge имеют разные владельческие обязательства. Если мост не установлен,
остаются deterministic keep/measure/abstain и действующий safety controller.

## Поправки, ограничивающие применение старых предложений

| Старое предложение / источник | Что изменяет план |
|---|---|
| [R03-17: независимая product-formula и бесконечный credit loop](../../../theory/research/reviews/NINE-2026-10-27-R03-17-ERRATUM.md) | Эти положения не реализовывать. Unknown dependence не даёт произведение независимых вероятностей; физический allowance и statistical error spending раздельны. Corrected conditional alpha rules не подтверждают producer/численный kernel |
| [R06-013-A: useful-task interval](../../../theory/research/reviews/NINE-2026-10-27-R06-013-A.md) | До формулы определить enrolled N, deadline/maturity, clock-unknown и protection scope. Early incomplete не final failure; «точная доля» outcome-selected поднабора не всей когорты |
| [R04-14 и широкие resource claims](../../../theory/research/reviews/2026-10-28-DEEP-REVIEW.md) | Sender/controller pair и before/after-flight semantics проверить до upgrade. F+2L snapshot не double bandwidth, queue cap не process RSS; absent overload enforcement не подразумевается исправленным одним лимитом |
| [R02-012/014: числа и российские сети](../../../theory/research/reviews/NINE-2026-10-27-R02-014-A.md) | Ни 95/99,5%, ни разность counters211 не priors/transport ranking. Сохранить dated source/ASN/access/period и неизвестное; один новый VPS не гарантирует доступ в каждой allowlist/shutdown сети |
| [R06-010/012: MDM/UID/corporate VPN](../../../theory/android/UID-ROUTING-AND-OWNERSHIP.md) | Матрица capture/lockdown/owner/user/profile/family и app-rule attribution заменяет общие «запрещено/совместимо». Не обещать native/OEM поведение из одной API-константы или raw UID capture |
| [R05-002: minimal durable state](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) | Проверять поддерживаемые recovery branches и различимость current history. Необходимость различимости не доказывает обязательное новое DTO-поле/БД или единственный store; existing cut/journal и publisher ограничения рассматриваются по цене/полноте |

Повтор этих утверждений в прежнем STATE/очереди/итоге не является новым
доказательством. Для применения действует точная область рецензии и актуальный
caller. Авторские pending ветки сохраняются как design/verification вопросы,
без автоматического превращения всех идей в требования.

## Уточнения переноса знаний

### TLS minimum — отдельный compatibility gate

[Requested MinVersion](../../../theory/transports/COMPOSITION-AND-TLS-CONTRACTS.md#3-minversion-проходит-через-preset-а-не-действует-вне-его)
и [конфиденциальность ShadowTLS-композиции](NETWORK.md#n02) — разные вопросы.
В просмотренных Windows constructors есть Chrome preset, но нет явного requested
minimum 1.3. Поэтому условный library trace с перезаписанным floor требует
матрицы backend/preset/requested bound и достижимого consumer до выбора изменения.
Android проверяет форму min/max, что ещё не аттестует поведение native artifact.
Сохранить certificate/Finished и внешние guards. Это уточнение
[NETWORK N03](NETWORK.md#n03), TH-01/TH-05/OFF-04 и AND-01/06; новый транспорт
или безусловный патч TLS из него не следуют.

### Свежий аутентифицированный ответ — ограниченное свидетельство

[Подпись nonce и покрытие полей](../../../theory/observability/PROBE-PROOF-BOUNDARIES.md)
дают лишь названное утверждение доверенного signer при проверенной связи с попыткой.
Доверие к подписи и к signer как измерителю различаются; relay не нарушает подпись.
Нужны точные covered fields, request/context и временное окно. Ни signed echo,
ни сравнение внешних IP не доказывают путь конкретного процесса или непрерывную
защиту. TWAMP authenticated-mode — сохранённый reference counterexample покрытия
полей; внедрять TWAMP, новый signer или второй probe stack не требуется.
Эта граница дополняет [N07](NETWORK.md#n07), TH-02/OFF-05 и AND-07.

### Защитные действия, учёт и оптимизация

[Адаптация](ADAPTATION.md#F04) не должна задерживать Stop/revoke/детерминированное
восстановление ради статистического решения. Current health projection сохраняет
unsafe precedence и owner/epoch filters; она не заменяет полный знаменатель
зарегистрированных попыток. Такой учёт не требует новой БД или durable store
в pure policy. Task completion, endpoint protection и whole-task protection
сохраняют разные producers и остаются отдельным отложенным
[контрактом](PRODUCT-ANDROID.md#task-metrics).

### Ранние исправления с самостоятельной границей

[Отмена UDP session](PERFORMANCE.md#F06) и abort/retirement целого carrier — два
результата. Admission до clone/spawn, count+bytes и actual last-holder return
важнее нового timeout wrapper; процессные Windows/Android barriers сохраняются.
Готовая пара congestion-кода — reference согласованных semantics, её обновление
не исправляет автоматически cancellation, PTO/PMTU и memory accounting.

[TUF progress](TRUST.md#k1) требует per-role eligibility, отдельно от application
acceptance и durable commit. Current SDK pointer не достаточен для нового
predecessor; ранее принятая expired история и допустимый prefix при поздней ошибке
сохраняются. Конкретный критерий и reset recipe проходят scoped contract review
до patch, без второго verifier и переноса timestamp-условия на snapshot.

[Восстановление системного прокси](PRODUCT-ANDROID.md#proxy-ownership) требует
own-write receipt и current-value/conflict semantics: формат localhost:1080
и TCP timeout не устанавливают владельца. Это самостоятельный Windows этап,
не зависящий от Android/SDK. Факты NetGuard/listener не показываются как
подтверждённая защита. Реальные OS изменения здесь не выполнялись.

### Сохранённые ограничения из смешанных исторических разделов

Служебное происхождение раздела не отменяет его полезных оговорок.
Receive window/PTO/singleflight сохраняют исправления в
[PERFORMANCE F15](PERFORMANCE.md#F15), а artifact/adoption, offline status,
multirepo и old-client ограничения — в [TRUST K4/K8/K10](TRUST.md#k4).
Различия глубины чтения и ошибочная R03-17 формула сохранены в
[методике](../../../theory/methodology/RESEARCH-METHOD.md) и
[ADAPTATION F17](ADAPTATION.md#F17). Датированные Android release/API/Play сведения
остаются reference и будущим release compatibility gate, без нового web-поиска
или переноса старой даты как текущего обязательства. Равные тела архивных записей
не сворачиваются: их heading/link/review context остаётся доступен.

## Приёмка этого плана

До финального PLAN требуется: полное содержательное coverage, все platform
сопоставления и crosschecks, обработка relevant source drift, решение конфликтов,
один владелец каждого change и явные непроверенные gates. Готовность Markdown,
совпадение ответов моделей и число заполненных строк сами по себе этого не доказывают.
