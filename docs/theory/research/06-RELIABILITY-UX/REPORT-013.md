# R06-013 — полезное восстановление задачи: estimand, privacy и будущая локальная программа

> **R06-013-A: NEEDS_REVISION.** Правила maturity, Q/F до горизонта D, missing-clock и protected outcome требуют уточнения. Не использовать предложенные ratios как гарантию timely recovery всей cohort. Рецензия не принимает и не отвергает по аналогии B или OSS reuse: [решение и контрпримеры](../reviews/NINE-2026-10-27-R06-013-A.md).

Дата исследования: **2026-10-27**. Статус: **READY_FOR_REVIEW**. Это проект будущего локального измерения, не
пользовательский эксперимент, не telemetry design и не product/native PASS.

## 1. Пробел и проверка baseline

R06-004 уже доказывает важную отрицательную границу: bounded typed journal и
frozen export могут подтверждать отдельные controller/protection facts, но не
прикладное завершение задачи (`REPORT.md:1248–1253,1291–1300`). R06-01 ранее
задаёт зрелую cohort и детерминированный missing-outcome интервал
`[S/N,(S+U)/N]` (`REPORT.md:288–306`).

R06-011 сделал полезный шаг от port/tunnel-up к task outcome, но его контракт
нельзя принять без исправлений:

1. `per-class recovery rate с union-границей` (`REPORT.md:2062–2064`) зависит от
   математической ошибки R03-009 и отозван
   [NINE R03-17 erratum](../reviews/NINE-2026-10-27-R03-17-ERRATUM.md).
2. Фраза «платформы типизируют задачи/переходы» — предположение, не source-факт:
   текущий журнал не наблюдает результат почты/звонка/корпоративной операции.
3. `useful-complete` без указания protection state смешивает защищённое и ручное
   незащищённое выполнение. Это меняет смысл показателя, а не просто добавляет
   поле.
4. `fail`, `abstain`, user stop и потерянный follow-up не образуют корректной
   таксономии missing/censoring. Удаление отменённых/неполных эпизодов смещает
   знаменатель.

**Ограниченный вопрос пакета:** какой минимальный локальный record и какой
заранее заданный estimand позволяют отдельно считать protected completion,
ручной unprotected outcome, partial и неизвестный timely outcome, не собирая
контент задачи и не отправляя автоматическую телеметрию?

## 2. R06-013-A — два estimand вместо одного двусмысленного recovery rate

Термин *estimand* используется как переносимый методический словарь ICH E9(R1),
не как клиническая аналогия и не как статистическая гарантия продукту. Официальный
документ требует заранее определить условие, популяцию, переменную, обращение с
intercurrent events и population-level summary (A.3.1–A.3.4, полный текст в
SOURCES). Для software pilot задаются следующие атрибуты.

### 2.1 Общая cohort и часы

- **Единица:** один локально зарегистрированный recovery episode заранее
  выбранного `task_class`, начавшийся после наблюдаемого отказа или явного
  действия «Начать восстановление». Повторный retry не создаёт новый episode.
- **Популяция:** все зарегистрированные episodes выбранных классов в заявленном
  календарном окне на одном устройстве/профиле. Не зарегистрированные попытки
  вне pilot в estimand не входят; coverage сообщается отдельно.
- **Origin `t0`:** parent elapsed/monotonic time факта регистрации episode,
  **до** отображения исхода. Wall clock допустим только для человеческого
  календарного окна и не заменяет interval clock.
- **Follow-up horizon `D`:** фиксируется до начала pilot отдельно для класса;
  численные D сейчас не выбираются. Episode зрелый, когда есть terminal outcome
  либо наступил `t0+D`. Незрелые — `pending`, вне знаменателя текущего среза.
- **Retained-resource horizon `R`:** отдельный срок после terminal/outcome для
  наблюдения подтверждённого release/retirement. Он не продлевает `D` и не
  исправляет timely outcome задним числом.

### 2.2 Взаимоисключающие outcomes зрелого episode

| Код | Outcome к `D` | Что требуется | Что он не доказывает |
|---|---|---|---|
| `P` | **Protected completion** | ручной локальный task receipt `completed`, связанный с fresh protection receipt в заявленном scope на момент completion | Причину успеха, успех всех приложений, отсутствие всех утечек |
| `U` | **Manual unprotected completion** | пользователь явно отключил/обошёл защиту, затем локально отметил task completed | Успех защищённого восстановления; такое событие нельзя прибавлять к P |
| `X` | **Completion, protection unknown** | задача подтверждённо завершена, но protection receipt отсутствует, устарел или не связан во времени | Ни protected, ни manual-unprotected completion; X нельзя молча присвоить P или U |
| `Q` | **Partial** | заранее определённая ступень задачи достигнута, terminal goal — нет/не подтверждён | Полный task success; partial не округляется вверх |
| `F` | **Observed non-completion** | пользователь подтвердил `not completed` к D либо допустимый terminal application receipt это устанавливает | Конкретную сетевую причину |
| `C` | **Censored / user stop** | episode прекращён до D (stop/закрытие pilot/уход), timely task outcome после stop неизвестен | Failure; user stop не автоматически F |
| `M` | **Missing task outcome** | task follow-up к D отсутствует/потерян либо episode clock не позволяет установить зрелость | Ни success, ни failure; неизвестный protection receipt при известном task completion — X, не M |

Manual protection-off — **intercurrent event**: он меняет интерпретацию task
completion. Потеря follow-up — **missing/censoring**: нужное значение не
наблюдалось. Их нельзя кодировать одним `abstain`. Эта логика согласуется с
ICH E9(R1) A.3.1: событие, меняющее смысл измерения, отражается в estimand;
withdrawal/administrative censoring создаёт missing-data problem, а не иной
фактический outcome.

### 2.3 Два population-level summary

Пусть для зрелой локально зарегистрированной cohort
`N=P+U+X+Q+F+C+M`, `N>0`.

1. **Strict protected timely recovery:**
   - наблюдаемая подтверждённая доля `L_P=P/N`;
   - без допущений о неизвестных protection/task outcomes истинная доля
     protected completions лежит в identification interval
     `I_P=[P/N, (P+X+C+M)/N]`;
   - `U,Q,F` не могут быть protected success по определениям; X известен как
     task completion, но protection component не идентифицирован.
2. **Task resolved by any route:**
   - подтверждённая доля `L_A=(P+U+X)/N`;
   - interval `I_A=[(P+U+X)/N,(P+U+X+C+M)/N]`;
   - публикуется **только рядом** с разложением `P/U/X`, чтобы unprotected и
     protection-unknown completions не скрывались в «успехе восстановления».

Это детерминированные bounds по неизвестным outcomes, **не** confidence interval,
не FWER/FDR и не коррекция selection bias. При `N=0` оценки нет. Сравнение
версий/policies и uncertainty от sampling требует отдельного R03 estimand-aligned
метода; старая `1-(1-α)^W` не используется.

### 2.4 Selection и sensitivity table

Даже полный учёт зарегистрированной cohort не позволяет говорить «все
пользователи»: пользователь сам решает открыть pilot, отметить исход и остаться
до D. Минимальный отчёт обязан показывать:

- `eligible_seen` (сколько раз UI мог предложить pilot, если этот счётчик вообще
  доступен без нового tracking), `registered`, `mature`, `pending`;
- counts `P/U/X/Q/F/C/M`, а не только ratio;
- интервалы `I_P/I_A`; отдельно `(X+C+M)/N` как protected-unknown mass и
  `(C+M)/N` как task-outcome unknown mass;
- чувствительность только как worst/best-case assignment C/M; модели MAR/MNAR,
  weighting и imputation без данных не вводятся;
- отдельные strata по заранее выбранным task classes; никаких post-hoc
  «хороших классов». Multiple-inference budget — R03, не этот пакет.

## 3. R06-013-B — минимальный privacy-preserving local record

### 3.1 Два источника, один локальный join

Task completion приложением не наблюдаем из VPN journal. Поэтому record содержит
**два независимых receipts**:

1. `task_receipt` — минимальный ручной ответ пользователя после task attempt:
   `{complete, partial, not_complete, stop}`; интерфейс не просит URL, приложение,
   аккаунт, текст сообщения или адрес назначения.
2. `protection_receipt` — существующий typed platform/controller факт с
   `scope`, `generation`, `observed_at_elapsed`, `age/unknown`, без raw payload.

`P` допустим только при обоих receipts и проверенном temporal linkage; если
protection receipt отсутствует/устарел, но task `complete` известен, outcome —
`X`, а не P, U или M. Для strict protected estimand X расширяет только верхнюю
границу; для any-route task resolution X является подтверждённым завершением.
Нельзя «восстановить» защиту по тому, что задача заработала.

### 3.2 Предлагаемая schema — научный контракт, не production DTO

```text
recovery_episode_v1 {
  schema_version
  random_episode_id           // новый локальный случайный ID; не stable device ID
  task_class                  // закрытый малый enum; exact service/app отсутствует
  trigger_class               // closed enum: observed_failure | manual_start | other
  t0_elapsed
  horizon_class               // versioned class; не произвольный пользовательский текст
  terminal_elapsed?
  task_receipt                // complete | partial | not_complete | stop | missing
  protection_state_at_task?   // applied(scope enum) | off_manual | unknown
  intervention_class[]        // switch | wait | config_change | off_manual; без значений
  censor_reason?              // user_stop | app_closed | process_lost | invalid_linkage
  resource_receipts {         // только существующие typed facts
      old_generation_retired?: yes | no | unknown
      retained_after_terminal?: yes | no | unknown
      observed_until_elapsed?: duration
  }
}
```

**Не записывать:** URL/domain/IP/SNI/SSID/ASN, имя приложения/пакета, account,
контент операции, UUID профиля, реальные transport credentials, stable device ID,
raw exception, headers/body/certificate, список установленных приложений. Не
хешировать hostname или account: low-entropy hash остаётся перебираемым stable
identifier. OWASP Logging Cheat Sheet (FULL_TEXT_READ, 27.10.2026) поддерживает
purpose limitation и исключение session IDs/tokens/sensitive identifiers;
здесь применяется более строгий typed allowlist **до записи**, а не regex после.

### 3.3 Retention, aggregation и export

- По умолчанию record только локальный, отдельный от support SessionJournal.
  Никакой automatic upload/analytics SDK/background export.
- Хранить bounded rolling cohort по числу/возрасту; точные cap/TTL выбираются
  платформой после resource/privacy review. Report всегда показывает
  `retained`, `evicted`, `recording_limited`, `invalid_linkage`.
- Агрегация строится локально из matured records: counts и intervals. Для
  корректности нужны source counts; «только проценты» запрещены.
- Ручной export: explicit action → immutable preview → тот же byte sequence share,
  существующий R06-004 pattern. Default export содержит агрегаты; episode rows —
  отдельная opt-in детализация с тем же allowlist. Экспорт не включает raw journal.
- Clear local data удаляет текущие references, но не обещает удаления уже
  отправленного файла/backup. Export должен маркировать schema version,
  window/horizons и dropped/evicted counts.

### 3.4 Retained-resource outcome не смешивается с task outcome

После task terminal запись может до `R` удерживать только три-state receipts:
`old_generation_retired`, `retained_after_terminal`, `unknown`. Они нужны для
вопроса «восстановление завершилось, но остались старые ресурсы?», но:

- resource release не превращает F/Q/M в task success;
- P не доказывает release старого поколения;
- отсутствие строки в bounded journal даёт unknown, не `no_retained_resource`;
- поздний release обновляет resource axis, но не переписывает timely P/U/X/Q/F/C/M.

## 4. Будущая локальная программа — дизайн, не запуск

### 4.1 Цель и минимальный порядок

1. До pilot зафиксировать task classes, cohort window, D по каждому классу,
   resource horizon R, schema version и правила P/U/X/Q/F/C/M.
2. Pilot запускается только явным локальным действием пользователя после
   наблюдаемого failure/manual start; никакого пассивного отслеживания приложений.
3. На t0 сохранить минимальный episode shell. Платформа присоединяет только
   существующие typed protection receipts.
4. До/на D показать одну локальную карточку outcome с вариантами complete,
   partial, not complete, stop; отсутствие ответа остаётся M/C по правилам,
   приложение не присваивает F молча.
5. После terminal до R читать только existing typed retirement/resource facts.
   Не удерживать task content и не запускать дополнительные network probes ради
   метрики.
6. Просмотр отчёта локальный: cohort flow, counts, I_P/I_A, P/U/X отдельно,
   protected-unknown и task-unknown mass, resource unknowns. Export только
   вручную после immutable preview.

### 4.2 Нулевой и следующий этапы

- **Этап 0 (desk/schema):** synthetic records без реальных сервисов/пользователей;
  проверить взаимоисключительность outcomes, monotonic time arithmetic,
  late receipt, invalid linkage, eviction accounting, byte-exact preview/share.
- **Этап 1 (только отдельное разрешение):** локальный self-pilot владельца с
  заранее выбранными неопасными задачами; никакой cohort других пользователей,
  telemetry или automatic upload. Сейчас **NOT_RUN**.
- Более широкое UX/user study требует этики, consent, data-protection review,
  sampling plan и отдельного поручения; пакет его не предлагает.

### 4.3 Конечная будущая матрица — NOT_RUN

| ID | Будущий synthetic/device case | Требуемое различие |
|---|---|---|
| RP01 | task complete + fresh applied receipt | P, при условии scope/time linkage |
| RP02 | task complete после explicit protection-off | U, никогда не P |
| RP03 | login page открыта, операция не завершена | Q, не complete |
| RP04 | not-complete ответ до D | F; причина unknown |
| RP05 | user stop до D | C; не F и не success |
| RP06 | нет follow-up к D | M; входит в N и widening interval |
| RP07 | complete, protection receipt stale/missing | X: task complete, protection unknown; strict P не повышается |
| RP08 | late complete после D | timely outcome не переписывается; late отдельно |
| RP09 | P, но old generation retirement unknown | task P + resource unknown одновременно |
| RP10 | record evicted до maturity | M/coverage loss + evicted count; не исключать из отчёта молча |
| RP11 | Clear после export | local current data cleared; shared copy не обещана удалённой |
| RP12 | raw URL/token в exception path | не попадает в episode/export; schema rejects field |

## 5. OSS reference: OONI Probe CLI v3.31.0 — adapt boundary, не код

Exact pin: tag `v3.31.0`, commit
`1605e00481e2b98488d62cfc2f7ce34ff345eb70`, release 22.10.2026,
**GPL-3.0** (LICENSE full text captured). Source archive и hashes — собственный
artifacts-каталог.

Проверенный pattern:

- `internal/model/measurement.go:71–101` — versioned local `Measurement`;
- `cmd/ooniprobe/internal/nettests/nettests.go:160–167` — сетевой submitter
  создаётся только при `Sharing.UploadResults`;
- `internal/oonirun/experiment.go:204–216` передаёт
  `Enabled: !NoCollector`;
- `internal/oonirun/submitter.go:38–52` возвращает no-op `stubSubmitter`, если
  upload выключен; реальные network effects отделены от local saver;
- `ScrubMeasurement` заменяет известный current IP в serialized fields, но это
  **не общий privacy proof**: неизвестные identifiers/inputs могут остаться.
- `cmd ... upload/upload.go:9–16` — команда `upload` **не реализована**;
  следовательно, OONI нельзя цитировать как готовый ручной export caller.

**Reuse decision:** только **reference/adapt** для явного upload enable gate и
разделения local record / submitter. Код не копировать: GPL-3.0, модель данных
сетевого измерителя шире и чувствительнее нашей typed schema; scrub-by-replacement
хуже producer allowlist. Не запускать OONI network experiments в recovery pilot.

## 6. Цена, оппонирование и предел знания

| Сильное возражение | Ответ / пересмотр |
|---|---|
| «Ручной outcome субъективен» | Да. Он измеряет заявленный task outcome, а не объективную транзакцию сервиса. Для P дополнительно нужен protection receipt; remote business transaction остаётся за приложением |
| «Stop — просто failure» | Нет: неизвестно, завершилась ли бы задача до D. Кодирование stop как F меняет estimand и может систематически штрафовать/поощрять UX, удерживающий пользователя |
| «Missing можно исключить» | Нет: complete-case rate зависит от follow-up selection. M/C входят в N и interval; coverage сообщается отдельно |
| «Protected completion можно вывести из tunnel-up» | Нет: tunnel-up — proxy. Нужны task receipt и protection receipt с scope/time linkage |
| «Local-only означает privacy-safe» | Нет: device compromise, backup, screen/share и sensitive fields остаются рисками. Поэтому typed minimization, bounds и explicit export обязательны |
| «Нужен точный CI» | Не сейчас. Identification interval отражает неизвестные outcomes; sampling uncertainty и multiple comparisons требуют отдельного R03 метода и данных |
| «OONI уже всё реализовал» | Нет: другая цель, GPL-3.0, более широкий payload; CLI manual upload — заглушка. Переносится только verified architectural boundary |

Основной неизвестный параметр — связь добровольно зарегистрированной cohort с
реальными инцидентами: без eligibility/participation данных нельзя оценить
selection bias. Даже при полном follow-up `P/N` относится только к локальной
cohort. Пакет не устанавливает численный SLO, causal effect UX, среднюю
надёжность Snowden System или соответствие закону о персональных данных.

<a id="7-предлагаемая-вставка-руководителю"></a>
## 7. Предлагаемый вывод для общей теории

> Полезное восстановление измеряется в зрелой локально зарегистрированной cohort
> на уровне прикладной задачи, а не tunnel/probe. Protected completion требует
> одновременно task receipt и свежий protection receipt. Подтверждённое task
> completion при неизвестной защите — отдельный X, не protected и не missing
> task outcome. Ручное выполнение после снятия защиты — U, не успех защищённого
> восстановления. Partial, observed non-completion, user-stop/censoring и missing
> follow-up различаются. Для unknown components публикуются counts и
> детерминированные best/worst bounds, не confidence interval. Данные минимизируются typed allowlist до записи,
> агрегируются локально и экспортируются только явным действием через immutable
> preview; retained-resource receipts образуют отдельную ось и не переписывают
> timely task outcome.

Адресаты после научной рецензии: TH-08/COM-05/07/12, Android AND-08/11/12/14,
Windows/Android владельцы журналов и privacy; R03 — только для будущей
статистической inference. Статусы требований этим пакетом не меняются.
