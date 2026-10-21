# V2-105 — strict import и восстановление данных

Дата: 2026-10-20. OFF-04 / FR-011. Статус: locally-tested, общий OFF-04 partial.

## До исправления

`go test . -run '^TestImport' -count=1 -timeout=60s -json`: 3 leaf FAIL:

- Preview принимал envelope с неверной подписью.
- Signature rejection менял реальный temporary trusted_keys.json.
- Secret format rejection менял temporary keys/envelope/version, оставляя vault.

Воспроизведение использовало валидные synthetic keys/schema/channel/secret refs,
затем изменялся конкретный проверяемый input. Snapshot сравнивает bytes четырёх
фактически используемых файлов, а не постороннего пустого каталога.
`.artifacts/tests/import-before.jsonl`; без owner vault/системной сети.

## Реализация

- `windows/onboarding_import.go`: общий strict prepare для Preview и Apply,
  а не два разошедшихся валидатора. Проверяются key ID/duplicates/hex, signature,
  strict envelope, positive version/actual local floor, descriptor gate и staged
  credentials. Preview не восстанавливает journal и ничего не пишет на диск.
- `secretvault.WithPreparedImport` предоставляет проверенный staged snapshot под
  mutex; значения остаются в памяти, bytes для commit содержат только DPAPI.
- `render.DescriptorsFromEnvelope` — чистый существующий descriptor gate;
  ValidateCredentials проверяет selectable channel roles без engine/pin/network.
  Неподготовленные planned/disabled/revoked каналы не требуют credentials.
- `backend/filetxn`: bounded files/paths, staged originals/new bytes, synced
  journal, replacement четырёх файлов, commit point — removal of active barrier
  через rename directory. Error/panic до commit point запускает rollback; ошибка
  rollback оставляет journal/ErrRecoveryRequired. После commit point cleanup
  не превращает подтверждённую запись в reported failure.
- Recovery принимает fixed paths caller'а, а не journal. Все backup hashes и
  manifest проверяются до первого restore; повреждённый journal не игнорируется.
  Hashes обнаруживают corruption, не аутентифицируют локального атакующего,
  способного переписать и backup, и manifest.
- Metadata/vault APIs проверяют active barrier. App import/recovery и render
  floor publication используют общий in-process serializer. App operationMu
  сериализует prepare/commit с основными engine operations; App.mu и vault mutex
  защищают от обычных writers этого экземпляра.
- Startup recovery расположен до legacy vault migration/Seed; при отказе
  остаётся profile_recovery_required, auto-connect не запускается.
- Envelope сохраняется из тех bytes, которые прошли проверку. Validate больше
  не меняет подписанный SplitDirect; runtime нормализует свою копию. Keys encoding
  детерминирован, trailing keys data отвергаются.

Новых зависимостей нет. Переиспользованы crypto/metadata/vault/render API.
Существующие публичные Wails method signatures/preview fields сохранены.

## Финальная проверка

| Команда / область | Результат |
|---|---|
| `go test . ./backend/filetxn ./backend/metadata ./backend/render ./backend/secretvault -run '^(TestImport\|TestFileTransaction)' -count=5 -timeout=120s -json` | 52 leaf ×5=260 PASS,0fail/skip;71.43 s |
| `.tools/task/task.exe --taskfile Taskfile.yml backend:check` | ordinary → production tags → vet;exit0;82.21 s |
| gofmt -l затронутых Go sources/tests | пустой вывод |

App matrix: signature/keys/schema/version/floor/protocol/roles/secret faults,
успешный checked-bytes import, repeat Verify после Validate, pending journal
blocks reads, fresh App recovery, concurrent imports и поздний downgrade.
Component matrix: error/panic каждого write, publish/commit-point rename,
failed rollback, recovery/retry для existing/missing files, corrupt backup и
manifest, invalid targets. Неполный rollback не объявляется восстановленным.

App commit rejection и component write faults проверены раздельно; настоящий
Wails startup/kill/reboot/отключение питания не запускались. В первых итерациях
были исправлены unused import при build и missing-target fault fixture: recovery
для отсутствовавшего файла использует Remove; его fault нужно инъецировать отдельно.
Финальные прогоны выше относятся к актуальным inputs и заменяют interim timings.

Commands/JSON: `.artifacts/evidence/2026-10-20-import-focused.json`,
`2026-10-20-import-task.json`; outputs `.artifacts/tests/import-focused.jsonl`,
`import-backend.txt`. Source/hash/harness manifest:
`.artifacts/evidence/2026-10-20-import-transaction.json`.
APPDATA изолирован, live profile opt-in выключен. UI source identity сравнивается
с V2-104; frontend повторно не проверялся.

## Остаток

- KDF aggregate memory/work admission; полная signed transport/FD/probe schema
  и миграция неоднозначных legacy refs. Legacy kind после merge обязан оставаться
  однозначным: импорт не выбирает слот и не удаляет чужие empty slots автоматически.
- Durable Windows power-loss/ACL/reparse-point/crash/reboot и interprocess writers
  требуют отдельного gate. File Sync и synthetic interruption не доказывают их.
- Общая observer/network epoch atomicity остаётся OFF-03/05. Global profile API
  reader model вне cooperating App/render entrypoints не принят этим срезом.
- Орфанные stage directories после аварий и cleanup/resource budget требуют
  отдельной обработки. Удаление barrier после successful commit отделено от cleanup.
- Новые тесты не запускают engine, внешние DNS/HTTPS, VPS/SSH/CF/admin, proxy/WFP/TUN.
  EXE/NSIS и race при CGO=0 не выполнялись. Превосходство не измерено.
