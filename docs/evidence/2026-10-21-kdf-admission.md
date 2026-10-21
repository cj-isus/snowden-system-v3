# V2-106 — KDF admission и границы onboarding

Работа: 2026-10-20…21. OFF-04 / FR-011 и ресурсная часть OFF-06.
Статус: locally-tested; полные OFF-04/06 остаются partial.

## Воспроизведённый дефект

Старый Open проверял N отдельно, но пропускал опасные сочетания N/r/p до
вызова scrypt. Три synthetic headers достигли intercepted KDF:

| N / r / p | Что превышалось |
|---|---|
| 2^21 / 8 / 1 | один V buffer около 2 GiB |
| 2^14 / 4096 / 1 | один V buffer около 8 GiB |
| 2^15 / 8 / 1024 | работа N*r*p = 2^28 |

Команда из `windows`:

```powershell
go test -overlay ..\.artifacts\tests\kdf-baseline-overlay.json ./backend/onboarding -run '^TestKDFBaseline' -count=1 -timeout=30s -json
```

Результат: exit1, 3 leaf FAIL,0pass/skip. Overlay содержит точный pre-fix
onboarding.go с заменой только scrypt.Key на счётчик/stub и добавлением test file.
Production source не подменялся. Реальная опасная аллокация/тяжёлый KDF не
выполнялись: размеры — расчёт по pinned source, не замер потребления памяти.
Артефакты: `.artifacts/tests/kdf-baseline-*`, `kdf-before.jsonl`.

## Реализация

- `windows/backend/onboarding/kdf.go`: checked uint64 multiply/add;
  структурная проверка N/r/p, затем оценка памяти и работы **до** scrypt.
  Общий process-local executor/gate используется обоими публичными Seal/Open.
  При нехватке бюджета — ErrKDFBusy без очереди; error/panic освобождает
  reservation, не выдаёт частичный ключ и не раскрывает callback payload.
- Оценка `128*r*(N+p+2)` соответствует трём основным buffers pinned
  `golang.org/x/crypto v0.51.0/scrypt`: V=128*N*r, XY=256*r, B=128*r*p.
  Работа оценивается единицами N*r*p. Существующий standard scrypt/AES-GCM
  сохранён; новых зависимостей или заимствованной реализации нет. Исходник и
  BSD-3-Clause LICENSE проверены в module cache; hashes есть в manifest.
- `codec.go`: отдельный bounded encoder/decoder поверх того же SNOB1 формата.
  Верхняя граница transport проверяется до trim/decode; header shape/cost,
  salt/nonce/ciphertext — до KDF. Decoder отвергает unknown/trailing JSON.
  Gzip читается с Max+1, превышение отвергается; корректный JSON-префикс с
  лишним whitespace не может стать допустимым после усечения.
- Seal не изменяет caller payload: schema default задаётся копии. Дешёвые
  входные size/count bounds предшествуют Marshal; encoded transport size
  проверяется до KDF, чтобы Seal не выдавал строку, которую Open отвергнет.
- `onboarding_dialogs.go`: regular-file/stat gate, bounded read Max+1 вместо
  неограниченного os.ReadFile. Oversized export password отвергается в facade
  до обращения к vault. Публичные Wails signatures сохранены.

| Граница | Значение |
|---|---|
| Один KDF | ≤96 MiB расчётных buffers, N*r*p ≤2^21 |
| Все активные KDF в процессе | ≤128 MiB, N*r*p ≤2^22, максимум 2 операции |
| Transport / распакованный JSON | каждый ≤256 KiB |
| Password | ≤4096 bytes |
| Salt / nonce | 16…64 / ровно 12 bytes; export salt остаётся 32 |

Export defaults прежние: N=32768,r=8,p=1,key=32 bytes; version1, AAD и
двойное base64 encoding не менялись. Старые стандартные SNOB1 exports
совместимы. Нестандартные дорогие параметры, огромные password/salt и unknown
fields, которые раньше могли пройти permissive decoder, теперь отвергаются.

## Проверки

| Команда / область | Наблюдаемый результат |
|---|---|
| `go test . ./backend/onboarding -run '^TestKDF' -count=5 -timeout=120s -json` | 79 leaf ×5=395 PASS,0fail/skip;15.32 s |
| `.tools/task/task.exe --taskfile Taskfile.yml backend:check` | ordinary → production tags → vet; exit0,163.66 s |
| `gofmt -l` восьми изменённых/добавленных Go files | пустой вывод |
| `node scripts/harness/project.mjs check --scope harness --run` | PASS; exact record в manifest |

Production tags: `with_utls,with_gvisor,with_quic,with_clash_api`.
APPDATA изолирован, SNOWDEN_LIVE_USE_PROFILE=0. Первый общий запуск оставил
лишь строку старта go:unit и потерял tool session, без итогового exit record;
он сохранён как `kdf-backend-interrupted.txt` и не засчитан. Повторный точный
Task завершился с exit0; времена/команды/вывод сохранены отдельно.

Матрица покрывает overflow/hostile costs, допустимые границы, два конкурирующих
Seal/Open и третий busy, aggregate memory/work/worker limits, idempotent release,
error/panic/nil/short/long key, oversized inputs/file/decompression, gzip checksum,
schema/unknown/trailing JSON, encoded output bound и отсутствие мутации input.
Каналы определяют порядок; 3 s timeout — guard теста, не production latency SLO.
Hostile costs используют private injected function, global executor не меняется.

Реальная криптографическая совместимость проверена в обе стороны:

- Fixture `testdata/legacy-v1.txt` получен точным **старым Seal** через отдельный
  overlay с default scrypt; новый Open его расшифровывает. Значения synthetic.
- Новый Seal открывается независимым reference decoder по старой процедуре с
  trusted defaults. Wrong password и изменённый ciphertext отвергаются.
- Отдельный constant-key test меняет N/r/p/salt в допустимых пределах и
  проверяет привязку GCM AAD. Он не выдаётся за реальный hostile scrypt run.

Итоговая структура доказательств:

- `.artifacts/evidence/2026-10-21-kdf-admission.json`: source/artifact hashes,
  expected source delta, JSONL counts, native runner и harness record.
- `2026-10-20-kdf-focused.json`, `2026-10-21-kdf-task.json`: точные UTC timestamps,
  exit codes, elapsed seconds и пути вывода. Focus выполнен 20-го, full replay — 21-го.
- Delta от V2-105: 3 изменённых Go files, 5 новых Go files и synthetic fixture;
  217 source/document hashes. Frontend из прежнего manifest не изменился.

## Границы доказательства и следующий шаг

Active-workspace admission не ограничивает весь RSS, GC-retained heap,
внутренние transient allocations, длительность в секундах, запросы в секунду
или весь параллелизм API. Это не межпроцессный лимит и не принудительная отмена
неуступчивого scrypt. Освобождение reservation после return не гарантирует
немедленного освобождения памяти ОС. Clear key — best effort, не secure wipe.

В новых tests нет настоящего vault, network/engine/system dialog. Полный набор
содержит существующие loopback/fake-policy tests; реальный WFP/TUN не проверен.
UI/EXE/NSIS/race/live/admin не запускались; CGO=0. VPS/SSH/Cloudflare и системная
сеть не затронуты. Скорость VPN, отсутствие блокировок и превосходство не измерены.

Следующий OFF-04: единая signed transport/FD/probe/hopping/mux/diagnostic schema
и миграция неоднозначных legacy bindings. Windows power-loss/interprocess/orphan
cleanup, observer epoch, native/live и comparative gates остаются открыты.
