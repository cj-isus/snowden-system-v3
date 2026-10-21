# V2-107 — общая схема подписанного и встроенного канала

Дата: 2026-10-21. OFF-04 / FR-011. Статус: locally-tested; OFF-04 partial.

## Дефект до изменения

Runtime ChannelDescriptor поддерживал 9 групп, отсутствовавших в metadata.Channel:
probe_targets, probe_primary_host, server_ports, hop_interval, up_mbps, down_mbps,
multiplex, diagnostic_url, fd. Strict signed decode отвергал их как unknown;
ручное преобразование имело отдельный неполный перечень полей.

До правок `go test ./backend/render -run '^TestProfileSchemaWireFields$'
-count=1 -timeout=30s -json` дал **9 leaf FAIL**,0pass/skip. Сохранены исходные
metadata/render/fdmodel, тест, hashes и JSONL: `.artifacts/tests/schema-before-*`.
Первый runner завершился -1073741819 с пустым выводом, причина неизвестна;
он не засчитан. Прямой повтор на тех же pre-fix sources дал ожидаемые 9 FAIL.
Промежуточная ошибка cwd скрипта не меняла production files; исправлена до реализации.

## Изменения и владение

- `metadata/channel.go`: существующий metadata.Channel стал единственной
  формой полей. Новые optional поля добавлены после старых, порядок старого
  canonical JSON сохранён. FailureDomain и per-channel validation находятся
  рядом с этой моделью; signing CLI и runtime вызывают один validator.
- `render.ChannelDescriptor` — named underlying type metadata.Channel:
  сохраняет composite literals, render methods и публичное имя. FailureDomain
  — alias общего типа; derivation/scoring остаются в render/fdmodel.go.
- DescriptorsFromEnvelope преобразует `mc.Clone()` целиком. Clone отделяет
  refs/SecretIDs/probe targets/ports и указатели validated_at/multiplex.
  Runtime evidence downgrade не меняет подписанный объект. Test с reflection
  требует заполнить каждый mutable field, чтобы новое поле не обошло clone guard.
- Встроенный parser стал strict для unknown fields/trailing JSON, включая FD.
  Неподдерживаемые настройки отвергаются: bandwidth только для HY2, multiplex
  только для WS/REALITY. ShadowTLS renderer mux не поддерживает; настройки не
  принимаются молча. Revoked записи остаются inert с прежним reference gate.

| Поля | Что проверено после доставки |
|---|---|
| probe_targets / probe_primary_host | HTTPS/primary-host gate, ClientConfig.ProbeTargets |
| server_ports / hop_interval / up_mbps / down_mbps | HY2-only, ranges/duration/pair bounds, реальные outbound значения |
| multiplex | nil/false/true не смешиваются, runtime WS/REALITY следует значению |
| diagnostic_url | HTTPS gate, ClientConfig.DiagnosticURL |
| FD: все 7 полей | wire roundtrip, common validation, подпись каждого поля, сохранение в runtime descriptor |

Go toolchain проекта поддерживает `omitzero`: нулевой FD не добавляет `fd:{}`
в старые canonical bytes. Явный `multiplex:false` подписывается и не исчезает.
Старые canonical bytes/подписи проверены; SchemaVersion остаётся 1. Это
совместимость нового клиента со старым профилем: старые strict клиенты отвергнут
новые optional поля. Новых transport protocols, зависимостей и crypto нет.

## Проверки

| Команда / область | Результат |
|---|---|
| `go test . ./backend/metadata ./backend/render -run '^TestProfileSchema\|^TestSecretIDLegacy' -count=5 -timeout=120s -json` | 77 leaf ×5=385 PASS,0fail/skip;27.04 s |
| `.tools/task/task.exe --taskfile Taskfile.yml backend:check` | ordinary → production tags → vet;exit0,93.98 s |
| `gofmt -l` семи затронутых Go files | пустой вывод |
| `node scripts/harness/project.mjs check --scope harness --run` | exact result сохранён в manifest |

Production tags: with_utls,with_gvisor,with_quic,with_clash_api.
APPDATA изолирован, SNOWDEN_LIVE_USE_PROFILE=0. 75 новых leaf и 2 legacy controls:

- 9 wire groups; 24 invalid shared-validation cases; tamper 8 extension fields
  и 7 FD fields; unknown/nested/type/trailing JSON в обоих decode paths.
  Все 24 отрицательные cases сверяют причину отказа и валидность baseline fixture.
  При review исправлен ShadowTLS ref: прежний invalid role отвергался до mux
  gate. Повторный финальный набор подтверждает именно целевые проверки; ранние
  логи сохранены как before-review и не подменяют итоговый proof.
- Старый canonical JSON подписан независимо Ed25519 и принят текущим Verify;
  nil/false/true mux и deep clone/runtime alias isolation проверены отдельно.
- Общий helper доставляет WS, два HY2 с разными IDs/values, REALITY и ShadowTLS
  через sign → encrypted Seal/Open → verify/Validate → temporary metadata/vault
  → LoadDescriptors → RenderFrom. Сравниваются **все** channel fields и outbound
  значения. HY2 pin — отдельный synthetic PEM в temporary directory.
- Каждый из 5 tuned configs проходит **настоящий pinned sing-box**
  `option.Options.UnmarshalJSONContext(include.Context(...))`. Box.New/Start
  не вызываются. Старый TestRenderAllChannelsPassSingboxParse использовал
  только проектный config.Parse; его имя не доказывало native parse.
- App Preview не пишет; Apply сохраняет исходные checked bytes и все поля,
  повторный Verify проходит. 6 invalid-extension cases проверяют реальные bytes
  keys/envelope/floor/vault после отказа и Preview, и Apply.

Manifest `.artifacts/evidence/2026-10-21-profile-schema.json` хранит точные
команды/UTC times/exit codes, 225 source/document/input hashes, artifact hashes
и результаты runner/harness. Входы Go и четыре command/dependency inputs
зафиксированы до финального полного прогона и повторно сверены после него.
Delta от V2-106: 4 изменённых Go files (включая усиленный прежний test helper),
3 новых Go files; frontend, dependencies и публичные Wails signatures не менялись.

## Оставшиеся границы

Это приёмка существующих descriptor fields. Доставка/ротация/отзыв HY2 pin
files не проверены: fixture предоставляет их отдельно. Неоднозначные legacy
kind refs всё ещё требуют явных signed bindings; они не выбираются автоматически.
Следующий срез — офлайн-миграция таких bindings и аудит границы pin provisioning.

Парсер не доказывает доступность сервера, независимость endpoints/ASN, скорость,
устойчивость к блокировкам, OS WFP/TUN или power-loss/interprocess durability.
Все ключи/секреты/vault/certificates в новых tests synthetic/temporary. Полный
backend сохраняет существующие loopback/fake-policy scenarios; новых engine/
public-network/admin действий нет. UI/EXE/NSIS/race не запускались; CGO=0.
Observer epoch/process isolation/orphan cleanup/native/live/comparative gates
сохранены в PLAN/STATE. Готовность всего VPN и превосходство не заявляются.
