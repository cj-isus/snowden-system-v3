# V2-104 — точные ссылки на секреты

Дата: 2026-10-20. Область: OFF-04 / FR-011, credential identity.
Статус: locally-tested. Общий OFF-04 остаётся partial.

## Воспроизведение до правок

`go test . -run '^TestSecretID' -count=1 -timeout=60s -json`
завершился exit1, оба leaf cases FAIL:

- Два synthetic vault slots одного kind: `vaultSecrets.Get` выбирал первый
  вместо отказа при неоднозначности.
- Facade onboarding import: source IDs не находились после успешного Apply;
  Save → AddSave генерировал новый ID и объединял одинаковые kinds.

JSON: `.artifacts/tests/secret-id-before.jsonl`. Использовались только временные
synthetic ciphertext/keys и APPDATA; без owner vault или внешней сети.

## Исправление и совместимость

Переиспользованы Manager DPAPI/atomic save, metadata signatures, render validators,
onboarding scrypt/AES-GCM. Новых библиотек/транспортов не добавлено.

- `secret_ids` — подписанная привязка каждой declared credential role к ID.
  Непустая map обязана покрыть все refs; invalid ID/extra/missing binding — отказ.
  Формат ID: 1…128 ASCII букв/цифр, дефис или подчёркивание; существующие UUID
  vault IDs допустимы. Профили без map используют legacy unique-kind путь.
- `SecretIDSource.GetByID` → `Manager.ValueFor`: точное разрешение и type check,
  без fallback при missing ID/несовпадении kind/неподдержанном source. UUID kinds
  прежних C/D слотов совместимы с общей UUID ролью. Вызов не увеличивает Reveals.
- Legacy lookup и AddSave отказывают при нескольких слотах kind независимо от
  порядка. Сам по себе пустой второй слот тоже не даёт права выбрать первый.
- `ImportSlots` сохраняет source IDs, проверяет весь batch (включая collision
  ID с другим kind), шифрует и вызывает один save под mutex одного Manager.
  Обновление точного ID сохраняет unrelated slots. Validation/save failure
  оставляет committed vault bytes прежними; обычные вызовы не пишут plaintext.
- Metadata override сохраняет UUID/REALITY/ShadowTLS role refs и копию bindings.
  Точные established HY2/ShadowTLS password-role имена разрешены, сами значения
  в envelope не добавляются. Старые canonical bytes сохраняют подписи.
- Facade onboarding использует batch вместо Save→AddSave; его test helper
  теперь явно изолирует APPDATA для metadata writes.

Пример synthetic descriptor fragment (значений секретов здесь нет):

```json
{
  "credential_refs": ["hy2-password", "hy2-obfs-password"],
  "secret_ids": {
    "hy2-password": "synthetic-hy2-one",
    "hy2-obfs-password": "synthetic-obfs-one"
  }
}
```

Второй HY2 может иметь те же roles/kinds и другие IDs. Наличие map не является
доказательством правильных серверных credentials или защищённого egress.

## Проверки

| Проверка | Результат |
|---|---|
| Focused `go test . ./backend/render ./backend/metadata ./backend/secretvault -run '^TestSecretID' -count=5 -timeout=120s -json` | 39 leaf ×5 =195 PASS,0 fail/skip;12.93 s |
| Exact native `.tools/task/task.exe --taskfile Taskfile.yml backend:check` | ordinary → production tags → vet;exit0;78.53 s |
| gofmt -l 12 изменённых/новых Go файлов | пустой вывод |

Матрица: signed roles для WS/2 HY2/REALITY/ShadowTLS → encrypted bundle → verify
→ exact vault import → LoadDescriptors → RenderFrom → strict sing-box parser;
проверяются passwords/obfs/UUID/REALITY key+short-ID/ShadowTLS password. Negative
cases: missing ID, kind mismatch, source без ID API, partial/extra/bad bindings,
signature tamper и legacy canonical bytes. Vault: duplicate/colliding/invalid
batch, write fault, exact update, ambiguous AddSave, concurrent imports одним
Manager, отсутствие plaintext и изменений счётчика Reveals.

Focused и native records: `.artifacts/evidence/2026-10-20-secret-id-*.json`.
Outputs: `.artifacts/tests/secret-id-focused.jsonl`, `secret-id-backend.txt`.
Source hashes/delta/UI identity и harness record:
`.artifacts/evidence/2026-10-20-secret-identity.json`.
Full runner использовал отдельный APPDATA и `SNOWDEN_LIVE_USE_PROFILE=0`.

## Границы и следующий шаг

- **Общая import transaction не реализована:** keys всё ещё могут измениться
  до signature reject, envelope/floor — до vault failure. Strict preview и
  validate→stage→commit всех файлов — следующий срез OFF-04.
- Полная signed schema для FD/probe targets/diagnostic/hopping/mux и KDF
  aggregate memory/work admission остаются открыты. Проверка credential roles
  всех четырёх транспортов не доказывает roundtrip всех их настроек.
- Batch proof относится к validation/save faults и одному Manager; crash/power
  loss, межпроцессные writers, durable multi-file commit здесь не проверены.
- В новых tests не запускаются engine/внешний DNS/HTTPS; только crypto/DPAPI,
  synthetic временные files и parser. Новый test pin не используется в системе.
- UI исходники не менялись, frontend checks не повторялись; EXE/NSIS не строились,
  VPS/SSH/CF/admin/системный proxy/DNS/WFP/TUN не затрагивались; race при CGO=0
  не запускался. Глобальное превосходство над другими VPN не измерено.
