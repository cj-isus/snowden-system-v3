# STATE — public Windows source snapshot

Current implementation: V491, source only. All PC-00..09 and prior requirements remain open. No tests, builds, native, VPN or release acceptance were performed. Private operational records and the full local state are excluded.

## V2-491 — owned legacy firewall cleanup

Продолжение **PC-01**, source/unverified. Raw exec.Command/CombinedOutput удалён
из actual legacy osRunner. Новый `legacy_firewall_windows.go` принимает только
семь исторических deletes, исполняет собственные canonical args и absolute
SystemDirectory/netsh.exe через original protected helper lane.

- Existing Job1/128MiB,5s command ctx,4096+1 output. Новый typed completed-command
  result даёт exit code и bytes после original Job/pipe/handle/source retirement;
  default helper/verifier zero-exit contract сохранён.
- Command-only PrepareCombined объединяет stdout/stderr в один bounded reader.
  Это два distinct inherited duplicates одного original output pipe; default/
  Detached stdio прежние, engine/verifier/GUI сочетания отклонены.
- Только completed capture/exit1/exact whole-message даёт absence sentinel.
  Реальные errors typed/redacted и не пропускаются legacy string heuristic;
  stdout не пишется в UI/logs. Mixed/unknown/overflow/cleanup failure — ошибка.
- Manual Release commit и marker-removal order прежние: ошибка сохраняет marker.

8 source diff, gofmt5+4 exit0 (8 unique files);8Go4docs/10existing2new;
18 reviewed inputs/1 cached x/text input.3 stdio targets добавлены после обнаружения
stderr sink, их оригиналы совпали с predecessor hashes и сохранены до правок;
initial/accepted receipts разделены. q87/s95/261/190/59 frozen sources неизменны.
Native/locale/Job/stdio/compile не проверены;5s/128MiB не measured SLO/RSS.
Marker bounded read/persistence/removal identity/durability — следующий source-пункт.
Тестов/сборки/CLI нет, полный план OPEN, TUN unavailable/schema2 unsupported.
[Контракт](tasks/TH-01-TUN-LIFECYCLE.md#pc-v491-owned-legacy-firewall).


## Next

Continue PC-01 bounded protection marker persistence and original file identity/custody review. TUN remains unavailable; schema2 activation remains unsupported. Native, locale, provider, OS and comparative acceptance remain open. See `docs/planning/knowledge-application/PC-PLAN.md`.
