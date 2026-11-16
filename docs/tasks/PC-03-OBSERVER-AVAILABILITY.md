# PC-03 — неизвестность наблюдателя и локальный link-down

2026-11-05, V2-214, Windows **source/unverified**. Продолжение
[worker DNS handoff](PC-03-WORKER-DNS-HANDOFF.md) в полном
[PC00…09](../planning/knowledge-application/PC-PLAN.md).

## Дефект и изменение

Ранее zero fingerprint означал одновременно ошибку route/interface collection,
TUN/unmatched route и отсутствие сети. Present→zero доставлял `down`; после
V2-213 это могло приостановить DNS работающего original worker. Отложенное
событие также могло сработать уже после потери наблюдаемости.

`windows/netwatch.go` сохраняет существующий poller/collector/consumer API:

- Zero теперь **unknown**. Никакого `down/up/change`, path revocation, DNS
  pause/resume, Ready или channel-health факта из него не получается.
- Explicit `allLinksDown` требует успешного непустого `net.Interfaces` snapshot:
  хотя бы один non-loopback adapter, ни одного поднятого link. Учитываются
  IPv6-only, сторонние virtual и собственный TUN. Активный virtual adapter
  может сделать наблюдение unknown; его нельзя отбросить для доказательства down.
  В установленном Go Windows source `net/interface_windows.go` FlagUp/Running
  выставляются по `OperStatusUp`, перечисление использует `AF_UNSPEC`.
- При активном link существующий UDP route lookup сначала использует IPv4
  literal, затем IPv6 literal только при ошибке IPv4 lookup. Приложение не
  отправляет payload/HTTP/DNS. Ошибка одной семьи не становится общим down;
  успешный lookup не утверждает Internet/UDP/TCP/protected/family PASS.
- Ошибка enumeration/address/Close, пустой snapshot, отсутствующее или
  неоднозначное сопоставление адреса, loopback/link-local/point-to-point/наш TUN
  дают unknown. После forbidden/malformed/uncertain result другой family lane
  не пробуется. Новый socket owner, DNS resolver или сетевой producer не создан.
- Baseline меняется **при доставке**, не при каждом sample. Unknown/TUN gap
  сбрасывает pending/debounce/cap, сохраняя baseline. Следующее observed изменение
  должно заново пройти debounce; старое событие через gap не доставляется.
- Подтверждённый departure от delivered route сохраняет continuityLost.
  A→down/B→A до доставки объединяется в один `change` после стабилизации,
  включая unknown gap после observed departure. Сам unknown этот флаг не ставит.
  После доставленного down возврат того же адреса является `up`.
- Повторный одинаковый sample не продлевает debounce. При непрерывно observed
  flapping сохраняется прежний cap35s; unknown/TUN gap начинает ожидание заново.
  Initial confirmed-down доставляется даже без прежнего route baseline.
- Класс берётся из captured event interface, MIB read вне watcher mutex.
  Проверка stop после collection и повторный TUN guard перед обработкой;
  журнал описывает local links/route, не объявляет Internet восстановленным.

## Границы

Snapshot/route/address/MIB calls не атомарны и не аттестуют physical path.
IPv4 preferred: изменение только IPv6 при доступном прежнем IPv4 не наблюдается.
PPP/point-to-point и route ownership за virtual adapters консервативно unknown;
Ethernet-shaped TAP нельзя признать physical по одному имени/Flags. Исчезновение
всех адаптеров даёт unknown. Active virtual adapter может скрыть physical outage.
Сокетный timeout700ms на попытку не доказывает hard syscall/Close bound; две
семьи могут увеличить стоимость опроса. Uncertain socket Close даёт unknown,
полная original-handle custody этого legacy collector отдельно не реализована.
Unknown не отменяет independent watchdog/protection checks и не означает health.

## Evidence и следующий шаг

Только source read/edit, gofmt одного Go-файла, manual diff и byte/SHA inventory:
`.artifacts/memory/observer-scope-source-evidence.json`. Tests/fixtures не
добавлены/изменены/запущены. Compile/build/vet/typecheck/parser/check/doctor/
generation/SDK/worker/product/socket/native/browser/admin/live/server execution
не выполнялись. ОС и пользовательские proxy/DNS/WFP/TUN settings не менялись.

Открыты Windows AF_UNSPEC/flags/IPv6 index/route/address/Close races, snapshot
ambiguity/virtual/PPP semantics, debounce/cap/unknown/flaps/initial-down/Stop
fixtures, actual worker handoff/reconnect/established flow continuity, resource
и latency acceptance. Далее source — terminal DNS-owner/control-fault/manual
reconnect reconciliation; actual family/path/native acceptance остаётся OPEN.
IPC9/Job ceiling8 не менялись; native acceptance const=false, TUN unavailable,
schema2 install unsupported. Полный PC00…09 и прежний backlog остаются активными.
