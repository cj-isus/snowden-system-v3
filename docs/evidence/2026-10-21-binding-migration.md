# V2-108 — явная миграция legacy credential bindings

Дата: 2026-10-21. OFF-04 / FR-011. Статус: locally-tested, общий OFF-04 partial.

## Что добавлено

Старый профиль без secret_ids при нескольких slots одного kind правильно
отвергался как неоднозначный. Отсутствовал publisher-путь миграции. Baseline
`go test ./tools/metadatatool -run '^TestBindingCLIEntry$' -count=1 -timeout=30s
-json` дал 1 leaf FAIL: unknown command bind. Только generated keys/temporary files.

- `metadata/bindings.go`: публичный BindingMigration содержит только role/ID
  назначения, version и validity. MigrateLegacyBindings проверяет старую подпись
  и структуру, требует полные назначения всех non-revoked legacy channels.
  Unknown/duplicate channel/role, неполные или invalid IDs отвергаются.
- Нельзя менять existing exact bindings или назначать revoked channel.
  Выбора по kind нет. Возвращается глубокая unsigned copy с новой version и
  explicit validity; старый профиль/подпись/refs и plan не меняются.
- Authentic expired source можно обновить: source shape проверяется на issued_at,
  новый interval — против now с обычным clock-skew contract. Новый expiry должен
  быть в будущем, version строго выше source. Реальный client floor остаётся gate.
- Plan ограничен 256 KiB и 64 уровнями JSON. Unknown/trailing data, duplicate
  JSON properties, duplicate entries и aliases имён полей отвергаются.
- `metadatatool bind` читает bounded regular files, проверяет соответствие
  исходного signer и целостность seed/public-half Ed25519 private file,
  подписывает и повторно Verify проверяет output. Private key не выводится.
- Source/plan/output ≤256 KiB каждый, key file ≤1024 bytes. Итоговый onboarding
  дополнительно проверяет общий размер payload/секретов по V2-106.
- Полный output сначала записывается/sync/close в temporary file. Hard link
  публикует его только при отсутствии destination. Existing output или alias к
  source/plan/key не заменяется. Обычные отказы очищают stage file.

В review тесты выявили 2 дополнительных FAIL нового parser: SECRET_ID и
Unicode long-s alias перекрывали secret_id из-за case folding encoding/json.
Теперь имена properties должны точно соответствовать lowercase ASCII spelling.
Pre-alias successful runs сохранены как superseded; финальные проверки повторены.
Первоначальная ошибка arity LoadEnvelope в integration test исправлена до приёмки.

## Как использовать издателю

Из `windows` с установленными project dependencies:

```powershell
go run ./tools/metadatatool bind source.json bindings.json '@publisher.key' migrated.json
```

Нужен matching private key исходного издателя. Команда не добавляет доверенный
ключ, не ротирует его автоматически и не применяет профиль к клиенту. Output
должен быть новым файлом. Необходимо выбрать version выше source и фактического
client floor, актуальные даты и реальные IDs нужных slots.

Пример **структуры**, не готовый production plan:

```json
{
  "schema": 1,
  "metadata_version": 42,
  "issued_at": "2026-10-21T07:00:00Z",
  "expires_at": "2026-10-22T07:00:00Z",
  "channels": [
    {
      "channel_id": "example-ws",
      "secrets": [
        {"ref": "vless-uuid", "secret_id": "chosen-slot-id"}
      ]
    }
  ]
}
```

Каждый non-revoked legacy channel, включая disabled/planned, требует полного
набора refs. Уже связанные и revoked channels в plan не включаются. IDs могут
указывать на slots из onboarding bundle или уже существующие на клиенте;
сам publisher не проверяет vault. При доставке Preview/Apply проверяют наличие
и kind по фактическому staged vault и продолжают fail-closed при несовпадении.

## Финальная проверка

| Команда / область | Результат |
|---|---|
| `go test . ./backend/metadata ./tools/metadatatool -run '^TestBinding' -count=5 -timeout=120s -json` | 67 leaf ×5=335 PASS,0fail/skip;26.26 s |
| `.tools/task/task.exe --taskfile Taskfile.yml backend:check` | ordinary → production tags → vet;exit0,105.95 s |
| `gofmt -l` шести затронутых Go files | пустой вывод |
| `node scripts/harness/project.mjs check --scope harness --run` | exact record сохранён в manifest |

Теги: with_utls,with_gvisor,with_quic,with_clash_api. APPDATA изолирован;
SNOWDEN_LIVE_USE_PROFILE=0. Все private keys/slots/profiles/vaults synthetic.

- Pure matrix: verified expired source, current validity, version/overflow,
  complete assignments, duplicate/unknown/revoked/already-bound records,
  WS/HY2/REALITY/ShadowTLS role sets, unchanged optional fields и mutable isolation.
- JSON: size boundaries, nesting, unknown/trailing/malformed/null,
  duplicate properties и case/Unicode aliases.
- CLI: настоящий main entrypoint в subprocess test binary; matching key,
  invalid/inconsistent key, caps/regular-file/unknown/tamper, no-overwrite input
  aliases и existing output, missing parent, oversized output; unchanged inputs.
- App: два разных UUID одного kind в temporary DPAPI vault. До миграции Preview
  и Apply дают ErrAmbiguousKind без записи; после — exact values в двух rendered
  outbounds, checked envelope bytes и все старые slots сохранены. Missing ID/
  wrong kind не меняют keys/envelope/floor/vault. Config.Parse — проектный parser;
  native sing-box gate V2-107 остаётся отдельным proof в полном наборе.

Source/evidence manifest: `.artifacts/evidence/2026-10-21-binding-migration.json`.
231 source/doc/input hashes; 1 изменённый Go file и 5 новых. Native input hashes
сняты до финального полного run и сверены после. Exact commands/UTC times/exit
codes и оба negative proofs сохранены в `.artifacts/tests/binding-*`.
Новых runtime dependencies, frontend/Wails API изменений нет.

## Что остаётся открыто

Реальные ключи, vault и профили владельца не читались и не мигрировались. Это
проверка capability на synthetic данных. Signing command не управляет глобальным
version ledger и не доказывает правильность человеческого выбора ID.

Hard-link publication требует поддержки filesystem; неподдержанная операция
возвращает ошибку. Fsync файла не доказывает directory/power-loss durability.
ACL/reparse/interprocess adversary, crash-orphan cleanup и неподконтрольные
filesystem faults остаются отдельными gates; secure wipe копий ключа не доказан.

HY2 pins исследованы только по коду: corebridge ищет pins/<channel-id>.pem,
затем legacy paths только для channel-a-hy2. Bundle их не доставляет. Следующий
срез: signed pin identity/content, доставка/ротация/отзыв и ID/path safety до
записи новых файлов. Текущее наличие/содержимое owner pins не проверялось.

Новые tests не запускают engine/network/WFP/TUN. Полный backend сохраняет
прежние loopback/fake-policy gates. UI/EXE/NSIS/race/live/admin не запускались,
CGO=0; VPS/SSH/CF и системная сеть не затронуты. Общая готовность VPN и
превосходство/экономия токенов не заявляются.
