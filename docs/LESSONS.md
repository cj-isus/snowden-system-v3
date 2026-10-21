# Уроки проекта

Ищи записи по теме перед работой. Сохраняй подтверждённую причину и рабочее решение после исправления ошибки или поправки пользователя; не сохраняй секреты и статус задач.

Формат: `- [ГГГГ-ММ-ДД] область: причина → решение; доказательство/файл`.

Повторно подтверждённое общее правило переноси в AGENTS.md, удаляя дубли. Специализированные уроки оставляй здесь или в тематическом документе.

- [2026-10-18] UX, поправка владельца: меню и табы не должны повторяться → табы обозначают подразделы выбранного меню; см. frontend/AGENTS и App/ContextTabs.
- [2026-10-18] UX, поправка владельца: чрезмерное число карточек мешает сценарию → прежде нового блока проверить строки/таблицу и существующие компоненты; OFF-08.
- [2026-10-18] Safety evidence: Engine.Stop с сохранённой WFP policy не исполняет production GUI/helper crash → эти сценарии принимаются раздельно; AUD-001/002/015.
- [2026-10-18] Mutex: существующий binding-тест не покрывал активный RunProbe → нужны ветки работающей сессии, не только early return; AUD-003, исправление и regression V2-097.
- [2026-10-18] Build: BOM оказался во внешнем D:/package.json, не frontend/package.json → проверять файл, названный loader; не перекодировать проект вслепую; AUD-018.
- [2026-10-18] Контекст: старый справочник содержал with_awg/with_wireguard и несуществующие ReloadVPN/Engine.Close → команды/входы сверять с manifest и кодом, прошлое ядро не является API текущего; V2-095.
- [2026-10-18] Стоимость: пользователь явно предпочёл проверяемую пользу на токен → сравнивать acceptance, переделки и суммарный usage; меньший AGENTS сам по себе не является доказанной экономией.
- [2026-10-20] Go panic: recover в функции с безымянным error-результатом возвращал nil → именованный результат и errors.Join с cleanup; Manager Switch/Reload panic regressions.
- [2026-10-20] Сессия: второй random metrics endpoint и listener из проверки порта расходились с применённым engine → один endpoint/secret из applied raw, Close временного listener; authenticated engine regression.
- [2026-10-20] Retry: чтение generation внутри отложенной горутины может присвоить ей новый пользовательский Start → резервировать и захватывать ID до освобождения mutex; проверять ID перед публикацией.
- [2026-10-20] Тестирование intent: старые retry-fixtures не задавали protectionDesired; Fatal под удержанным mutex затем блокировал cleanup → fixtures выражают намерение, значения читаются до Unlock, assertion выполняется после Unlock.
- [2026-10-20] UI async: поздний bootstrap затирал более новое событие, interval мог наслаивать запросы → owner/event generation и следующий poll после завершения; backendState/polling regressions, V2-098.
- [2026-10-20] Responsive CSS: незавершённая группа селекторов перед комментарием присоединяла панели к flex-строке → разделить правила, проверять scrollWidth каждой страницы на узких размерах; AutoProtect и browser matrix.
- [2026-10-20] Resolver: явного PostCSS config недостаточно для Vitest/jsdom, которые тоже доходили до внешнего D:/package.json с BOM → корневая локальная package boundary; посторонний файл не изменять.
- [2026-10-20] Данные: Go nil slices приходят как null, полностью skipped тест мог иметь ok=true → нормализовать списки на API границе, показывать skipped отдельно от PASS; api/testResult regressions.
- [2026-10-20] Close ownership: sing-box 1.13.19 закрывает done до дочерних сервисов, повторный Close не повторяет cleanup; Start скрывает early Close error → сохранять неопределённый owner/error и запрещать новый engine/Release, не считать os.ErrClosed доказательством очистки; V2-099.
- [2026-10-20] Cleanup panic: recover исходной операции не защищает от второй паники в её очистке → конвертировать panic каждого cleanup шага отдельно, продолжить остальные шаги и finalizer; lifecycle_faults_test.go.
- [2026-10-20] Rollback: восстановленный JSON и статус running не доказывают прежний защищённый путь → fresh old-probe, typed proof/restarted outcome и единая обработка facade; core/reload_test.go, V2-100.
- [2026-10-20] Observer lifetime: перезапуск прежнего конфига сохранял pointer cc и baseline metrics → новый snapshot identity и collector из restored bytes; старый watchdog отзывается, verified rollback не получает BLOCKED от лимита кандидатов; switch_outcome_test.go.
- [2026-10-20] Legacy recovery: сохранённый PID не доказывает владение процессом, а валидатор неполных helper-флагов отправлял их в GUI → удалить неиспользуемый lease/PID lifecycle, завершать retired CLI до побочных эффектов; startup сохраняет marker, неизвестное состояние требует явного Disconnect; killswitch migration matrix, V2-101.
- [2026-10-20] Prepare ownership: ReloadOption выполнялся под mutex до panic finalizer → готовить вне lock/ownership, возвращать rejected при panic/nil/cancel, сверять generation при commit; state=running после Stop/Start недостаточен из-за ABA; prepare_test.go, V2-102.
- [2026-10-20] Fault tests: `{}` не проходит строгую конфигурационную валидацию и не достигает ReloadOption → использовать существующий protectedFixture для callback faults, не принимать ранний validation reject за доказательство проверки panic; отдельная validation-before-options matrix.
- [2026-10-20] Probe boundary: factory вычислялся до Manager panic guard, а ручной probe не имел общего обработчика → выполнять factory/Run внутри guarded Probe, возвращать structured failure при panic/nil; callback payload не публиковать; probe_observation_test.go, V2-103.
- [2026-10-20] Probe facts/owner: AllPassed скрывал leak/failed step, busy сбрасывался до проверки operation ID → нормализовать копию report по фактам, finalizer проверяет session+operation; fast reprobe перепроверяет session после Run; V2-103.
- [2026-10-20] Secret identity: первый kind зависел от порядка, AddSave при импорте терял ID и объединял два HY2 → точные signed bindings, unique legacy lookup, batch import по ID с проверкой kind и всей пачки до save; secret_identity/identity tests, V2-104.
- [2026-10-20] Signed refs: override терял REALITY role refs, metadata запрещала сами HY2 password-role имена → копировать все credential-role поля, разрешать точные established роли; legacy canonical bytes и signature tamper проверять отдельно; render/metadata SecretID tests, V2-104.
- [2026-10-20] Import validation: preview принимал неверную подпись, Apply сохранял keys до verify и floor до vault failure → общий strict prepare с staged secrets до первой записи; journaled commit четырёх files, actual snapshot assertions; V2-105.
- [2026-10-20] Recovery: rollback тоже может отказать; отсутствующий old target восстанавливается Remove, а не Write → retain read barrier и journal, проверять все backups до restore, fault inject фактический шаг, recover до Seed/migration; filetxn tests, V2-105.
- [2026-10-20] Signed validation: нормализация SplitDirect меняла подписанный объект → Validate не мутирует input, runtime нормализует копию; повторный Verify после Store.Apply и checked stored bytes, V2-105.
- [2026-10-21] KDF admission: отдельный предел N не ограничивает произведения N*r/p → checked memory/work до scrypt и общий gate Seal/Open с release на error/panic; hostile inputs проверять injected compute без опасных аллокаций; V2-106.
- [2026-10-21] Codec bounds: LimitReader(Max) может принять обрезанный valid JSON с whitespace → читать Max+1 и отвергать превышение; проверять file size до чтения, encoded output до KDF и обе стороны старого формата реальным default crypto; V2-106.
- [2026-10-21] Schema drift: metadata.Channel не принимал 9 групп runtime fields, ручной converter терял расширения → один metadata-owned тип и validator, named underlying render type + deep Clone; zero FD через omitzero сохраняет старые canonical bytes, false mux остаётся явным; V2-107.
- [2026-10-21] Parser evidence: имя TestRenderAllChannelsPassSingboxParse скрывало вызов только config.Parse → отдельно вызывать pinned option.Options.UnmarshalJSONContext с include.Context, без Box.New/Start; проверять rendered значения и не переносить local parser PASS на native/network; V2-107.
- [2026-10-21] Negative fixture: ShadowTLS mux-case отклонялся раньше на неизвестном credential ref → брать established role и проверять причину отказа каждого negative case, а не один err!=nil; V2-107.
- [2026-10-21] Binding migration: клиент не может переписать signed refs без новой подписи → явный offline publisher plan по IDs, greater version/validity, verify→clone→sign и новый output; staged client остаётся gate наличия/типа slots; V2-108.
- [2026-10-21] JSON alias: duplicate-key map не ловил SECRET_ID и Unicode long-s, которые encoding/json сводит к secret_id → exact lowercase ASCII properties до typed decode;2 review FAIL и финальная negative matrix; V2-108.
- [2026-10-21] Theory drift: общий disclaimer не исправляет локальные «CDN=exit», «chunked сбрасывает TCP limit», «400=VPN» и «allowlist=shutdown» → править тезис на месте, указывать observer/период/конкурирующую причину, отделять pinned Options от latest docs; V2-109.
