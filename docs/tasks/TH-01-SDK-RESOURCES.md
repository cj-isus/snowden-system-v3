# TH-01 / OFF-06 — ресурсы SDK worker проверки обновления

2026-10-28, V2-188. Owner: `windows`. **Source implemented, unverified**.
Продолжение [SDK storage](TH-01-SDK-STORAGE.md) и
[pair updater](TH-01-UPDATE-PAIR.md). Все девять TH-этапов остаются открытыми.

## Обнаруженный дефект и граница среза

Уже существующий `UpdateCodeAcceptance.Verify` запускает SDK-проверку в отдельном
процессе. Однако `captureOwnedHelper` явно задавал commitBytes=0 для application
environment; CPU Job limit отсутствовал. Кроме того, Verify допускает два режима
проверки, а spawner разрешал только acceptance и отклонял recovery-check ещё до
создания процесса. Это source findings, без воспроизведения на Windows.

Исправляется этот существующий consumer. Обычные profile/pin/export SDK calls
пока выполняются в процессе приложения. Для их изоляции нужен отдельный канал,
связывающий private proof с точными входами и исходным процессом/образом, включая
restriction-only prefix при позднем отказе. Новый JSON success flag, публичный
конструктор proof или второй TUF-verifier здесь не добавлены.

## Начальная политика ресурсов

| Ресурс | Source contract |
| --- | --- |
| Job | Новый unnamed/non-inheritable, KILL_ON_JOB_CLOSE, без breakaway |
| Процессы | Один; перед Resume/в worker total=1, active=1; после exit total=1, active=0 |
| Память | 512 МиБ Job committed virtual memory, не RSS |
| CPU | 20 секунд суммарного user-mode CPU; 200 000 000 ticks по 100 нс |
| Превышение CPU | Явный TERMINATE_AT_END_OF_JOB, проверяемый чтением обратно |
| Wall time | Прежний capture context 30 секунд, затем самостоятельный reap budget 4 секунды |
| Ответ | Прежний фиксированный 33-byte protocol+manifest digest; Max+1 read и zero exit |

Числа — **начальные неизмеренные ceilings**, не SLO и не подобранный оптимум.
CPU time не включает kernel time и не задаёт процент CPU. Job memory limit
ограничивает commit; отказ нового allocation сам по себе не означает немедленное
завершение процесса. Kernel проверяет CPU периодически, возможен overshoot.
Context/reap budgets не доказывают точный deadline для kernel I/O/handle close.

## Установка и подтверждение до запуска

`ownedProcessLimits` расширен userTime с точностью 100 нс. Ненулевой CPU budget
разрешён только фиксированному application check, с точными memory/CPU ceilings.
`configureOwnedJobLimits` принимает исходный пустой Job ровно один раз до process
creation/assignment. Повторная настройка не может выдать свежую CPU allowance.

После Set ExtendedLimits устанавливается terminate action. Query проверяет размер
ответа API, точные flags/process count/memory/user-time и terminate action. После
Assign всё ещё suspended child проходит исходный image/source guard, затем ту же
проверку limits и accounting (один процесс, нет limit-terminated members, расход
не выше бюджета). Context проверяется ещё раз перед Resume. Любая ошибка возвращает
прежнего partial owner для retirement; запуска с ослабленными limits нет.

## Проверка в дочернем процессе и перед выдачей результата

Оба read-only dispatcher требуют pipe stdin/stdout и ресурсные preconditions до
SDK callback и после него. Query с NULL Job handle читает immediate Job текущего
процесса, в том числе при nesting. Это проверка ресурсов, **не аутентификация
родителя**; исходные image/source/SDK/floor gates продолжают задавать доверие.
Вызов без такого Job не доходит до SDK. На других ОС readiness=false.

Recovery-check получает собственный 30-second child context внутри прежнего
60-second recovery dispatcher. Изменяющий cold repair сохраняет свой прежний
контракт и не получает новые права через check. Whitelist spawner допускает ровно
acceptance и recovery-check с единственным проверенным digest; repair/GUI flags
не проходят эту ветку. GUI использует свой существующий отдельный dispatcher.

После pipe EOF и успешного Wait/zero exit capture требует native resource owner.
На исходном Job повторно проверяются limits/action, total/active/terminated counts,
TotalUserTime (включает уже вышедших members) и доступные peak memory counters.
При overshoot или нечитабельном accounting ответ отбрасывается. Peak counters
используются как дополнительные rejection checks, не как измеренный RSS/полная
история отказавших allocations. Затем прежние image validation, полный Job/I/O/
handle retirement и повторные context/source gates предшествуют выдаче bytes.
Неопределённый retirement сохраняет original owner и запрещает replacement.

## Совместимость и доказательства

Engine сохраняет ceiling8/commit0/CPU0. Системные diagnostic helpers — ceiling1/
128MiB/CPU0. Persistent GUI — прежний multiprocess/breakaway lifetime Job без
новых CPU/memory ceilings. Создание/retirement выполняются прежними owners;
дополнительных процессов, runtime fallback, SDK implementation или dependencies нет.

Wire format/activation protocol не менялись; требование ограниченного Job — новая
precondition check worker. Совместимость со старыми callers и nested Jobs ещё не
проверена. Public schema2 install по-прежнему **unsupported**.

Evidence: `.artifacts/memory/sdk-worker-source-evidence.json`, exact8 Go source
hashes и сохранённый source diff; gofmt8 + refinement1 captured exit0. Manual
review dispatch/policy/Set→Query→Assign→guard→Resume/exit→accounting→retirement.
No tests/fixtures/build/compile/vet/typecheck/parser/check/SDK/native/admin/live,
реальные owner values не читались; Win32 API не выполнялись.

Открыты API/ABI/native Query semantics/limits admission/nesting/overshoot/OOM/
cancel/Stop/partial creation/Close/concurrency/old caller gates, подбор budgets
и GUI+worker total CPU/RSS. Полный private-proof SDK worker bridge, whole-SDK
I/O/parity, directory power-loss/restart/source coverage, OS/DR/service/release/
comparative и TH-00…08 остаются незавершёнными.

## Первичные источники

V2-189: [product SDK worker](TH-01-SDK-WORKER.md) подключает обычные profile/pin/
update consumers к прежнему owner; existing bounded checks не создают nested SDK
child. Pending first enrollment тоже доходит до root signature gate внутри SDK.
Private IPC/compiled trusted adapter/native retirement — source-only, не закрытая
native/authority-boundary/CPU-RSS/parity приёмка; schema2 unsupported.

- [Microsoft BasicLimitInformation](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information): user-mode ticks, периодическая проверка, memory/process/breakaway flags.
- [Microsoft EndOfJobTimeInformation](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_end_of_job_time_information): DWORD action и terminate semantics.
- [Microsoft QueryInformationJobObject](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-queryinformationjobobject): NULL/immediate Job, information classes, returned length.
- [Microsoft ExtendedLimitInformation](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_extended_limit_information): committed memory и peak counters.
- [Microsoft BasicAccountingInformation](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information): total CPU/processes/limit terminations.
- [Pinned x/sys v0.44.0 amd64 type](https://github.com/golang/sys/blob/v0.44.0/windows/types_windows_amd64.go): PerJobUserTimeLimit int64; declarations/source сверены, ABI не исполнялась.
