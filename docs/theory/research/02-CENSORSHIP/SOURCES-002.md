# R02-002 — источники совместного наблюдения и shaping

2026-10-25 · `READY_FOR_REVIEW`. Предыдущие [REPORT](REPORT.md),
[SOURCES](SOURCES.md) и [handoff R02-001](handoffs/R02-001.md) сохранены.

## Происхождение и границы чтения

Новый source-index — `.artifacts/research/theory-02/2026-10-25-r02-002/source-index.json`
содержит exact pins, URL, размеры и SHA-256 **47 новых source/context файлов**.
Это число сохранённых файлов, не число полностью аудированных компонентов.
Проверяемые выводы ограничены указанными ниже branches/callers.

local-manifest — `.artifacts/research/theory-02/2026-10-25-r02-002/local-manifest.json`
содержит 13 снимков локальных входов, включая прежний пакет, текущую теорию,
DEPTH-CYCLE-002, manifest, renderer/validator, Windows preflight и Android guards.
Снимки не атомарны для всего репозитория. Финальная актуальность —
self-review — `.artifacts/research/theory-02/2026-10-25-r02-002/OPPOSITION.md`.

Прежние U01–U04 остаются привязаны к
manifest R02-001 — `.artifacts/research/theory-02/2026-10-25-r02-01/manifest.json`.
Сохранён hash этого manifest; повторная загрузка всех старых зависимостей не
выдаётся за новую проверку. Существующие captures не перезаписывались.

Некоторые raw/API обращения завершались timeout/404; успешные альтернативные
captures и metadata отражены отдельно. Несуществующий tag `v2.2.2` не использован
как источник Maybenot: pin получен из published crate VCS metadata и проверен
по Cargo.toml. Никаких установок и исполнения captured source не было.

## Новые закреплённые реализации

### D01 — существующий sing-mux

**v0.3.5**, commit `6fb501d02534177fed5567ee8f63afbc825e2861`.
Версия — windows/go.mod, tag lookup сохранён в `sing-mux-pin.json`.

- [client.go](https://github.com/SagerNet/sing-mux/blob/6fb501d02534177fed5567ee8f63afbc825e2861/client.go):
  `Client.offerNew` → protocol wrapper → `newPaddingConn` → `newClientSession`.
- [padding.go](https://github.com/SagerNet/sing-mux/blob/6fb501d02534177fed5567ee8f63afbc825e2861/padding.go):
  `kFirstPaddings`, `write`, `WriteBuffer`, `WriteVectorised`; общий счётчик wrapper.
- session.go — контекст создания mux session, не complete mux concurrency audit.
- Корневой LICENSE: **GPL-3.0-or-later**. Файлы прочитаны из module cache;
  отдельная pinned remote копия padding.go совпала по SHA-256 с cache.

**Вывод:** padding имеет конечную область записей одного carrier, не каждой
пользовательской операции; не утверждать точное число внешних TLS/IP пакетов.

### D02 — NaiveProxy

**v154.0.8037.49-5**, commit `d04efe1ea4477c2db24a7d0e2f068b3bee931e45`.
Tag ref повторно подтверждён в этом цикле (`naive-tag-ref.json`);
release опубликован **2026-10-22**, metadata — `naive-release.json`.

- [naive_proxy.cc](https://github.com/klzgrad/naiveproxy/blob/d04efe1ea4477c2db24a7d0e2f068b3bee931e45/src/net/tools/naive/naive_proxy.cc):
  `DoAcceptComplete` → cold/expired `DoPreamble` либо warm `StartOne` → Connect;
  `DoPreambleComplete` продолжает Connect и при ошибке preamble.
- [preamble_getter.cc](https://github.com/klzgrad/naiveproxy/blob/d04efe1ea4477c2db24a7d0e2f068b3bee931e45/src/net/tools/naive/preamble_getter.cc):
  `Start` → обычный GET через Chromium API; root parsing → ссылки того же
  host/port → `StartOne`. Это фактические дополнительные requests, не просто UA.
- naive_proxy_delegate.cc — negotiation padding capability, HTTP headers.
- naive_connection.cc `DoConnectServerComplete` → отдельный `NaivePaddingSocket`
  на stream connection; [naive_padding_socket.cc](https://github.com/klzgrad/naiveproxy/blob/d04efe1ea4477c2db24a7d0e2f068b3bee931e45/src/net/tools/naive/naive_padding_socket.cc)
  → первые8 frames; naive_padding_framer.{cc,h} → header3, max padding255.
  Небольшие записи одной ветки direction имеют другой random range, чем
  упрощённое uniform описание README; формулы REPORT ограничены явно.
- Сохранены дополнительные Chromium context файлы; их наличие не означает,
  что полный HTTP/TLS stack или server forwardproxy прошёл security review.
- Корневой LICENSE и указанные собственные файлы — **BSD-style / 3-clause**;
  полный Chromium/third-party SBOM не заменяется корневым LICENSE.

**Вывод:** пригодный full-stack reuse candidate при отдельном spike, не модуль
для копирования одной функции в Go renderer. Server-side padding/fronting
capability требуется отдельно; серверная реализация здесь не аудирована.

### D03 — Maybenot 2.2.2

Commit `51ce20e806f8e8bdffa0c39b464774d79d617234`,
[published crate VCS record](https://docs.rs/crate/maybenot/2.2.2/source/.cargo_vcs_info.json).
crates/maybenot/Cargo.toml у этого commit содержит version2.2.2.

- [framework.rs](https://github.com/maybenot-io/maybenot/blob/51ce20e806f8e8bdffa0c39b464774d79d617234/crates/maybenot/src/framework.rs):
  `new`, `trigger_events`, `process_event`, `below_limit_padding/blocking`.
  Проверены normal/padding counters, нулевая fraction, startup allowance,
  replacement blocking; встроенные tests не запускались.
- event.rs — очередь/intention события отличаются от `TunnelSent` near-wire.
- action.rs — `SendPadding`, `BlockOutgoing`, `replace/bypass`, timer semantics.
- machine.rs — поля и validate; rate_limited_framework.rs — rate of actions,
  а не byte quota. Не проводился исчерпывающий анализ rate-limiter.
- crates/maybenot-ffi/src/lib.rs — C representations/обёртка; само наличие FFI
  не проверяет ABI, Go/JNI integration или ресурсные границы.
- LICENSE-MIT / LICENSE-APACHE и workspace manifest: **MIT OR Apache-2.0**;
  выбранный Rust MSRV1.85, не требование менять наш toolchain сейчас.

**Вывод:** scheduler/state machine reuse candidate. Размеры пакетов,
распознавание dummy на peer, таймеры/IO/lifecycle/OS protection реализует adapter.

### D04 — GotaTun/DAITA как конкретный интегратор

Commit `2a867b3f322d95dc08043498d34eb91fb4266590`, snapshot main в этом цикле;
Cargo package0.10.2. Это **не аттестация установленного Mullvad-клиента**.

- Cargo.toml + Cargo.lock: фактически записан **maybenot2.2.2** с registry checksum;
  package manifest сам по себе был бы только semver requirement.
- [device/mod.rs](https://github.com/mullvad/gotatun/blob/2a867b3f322d95dc08043498d34eb91fb4266590/gotatun/src/device/mod.rs):
  outgoing `on_normal_sent` → tunnel encapsulation → `on_tunnel_sent` → UDP;
  incoming WriteToTunnel → `on_data_recv` → TUN. peer_state.rs связывает
  approved config с `DaitaHooks::new`.
- [daita/hooks.rs](https://github.com/mullvad/gotatun/blob/2a867b3f322d95dc08043498d34eb91fb4266590/gotatun/src/device/daita/hooks.rs):
  constant-size data padding, skip handshake/keepalive в delay hook, byte meters,
  bounded delay queue и отдельные event/action channels.
- daita/events.rs: events → Framework → timer scheduling → action channel.
- daita/actions.rs: action → `PaddingSent` accounting → replace/queue/drop/send
  ветки → encrypted UDP; types.rs содержит queue/timer support.
- UAPI.md — настройка peer; это не совместимый sing-box API.
- Корневой и указанные file headers: **MPL-2.0**; root Rust MSRV1.95.
  Другие компоненты/лицензии и native targets целиком не проверены.

**Вывод:** reference design для packet/queue/metering seams, не готовая DAITA
кнопка для WS/REALITY/HY2. Копирование всего WireGuard engine не предлагается.

## Литература и границы переноса

| ID | Первичный источник | Использовано / ограничение |
|---|---|---|
| Q01 | [Pulls/Witwer, Maybenot v2](https://arxiv.org/html/2304.09510v2), revised 2024-09-27, §§3–4, 6.1, 6.5 | Event/action integration; decoy и length padding раздельны; авторы оговаривают sim2real и причинность trace replay. Наши API-выводы привязаны к D03, не автоматически к paper version |
| Q02 | [Pulls, evaluation первых восьми DAITA серверов](https://pulls.name/blog/2024-06-05-eval-first-daita-servers/), 2024-06-05; collection 2024-05-29…06-02 | Прямая оценка WF-защиты и congestion tradeoff в конкретном Windows/WireGuard/corpus setting. Её цифры не прогнозируются для наших ОС/транспортов/РФ |
| Q03 | [Xue et al., USENIX Security 2024](https://www.usenix.org/system/files/usenixsecurity24-xue-fingerprinting.pdf), §§6–8 | Burst/encapsulated-handshake detector и ограничения простого padding/mux; протокольный риск, не current RF measurement. Те же идеи уже есть в THEORY, новизна здесь — D01 scope и joint model |
| Q04 | [Wu et al., GFW2023](https://gfw.report/publications/usenixsecurity23/en/), §4.4 | TCP-specific field result не превращён в доказательство UDP/Salamander или российского detector |
| Q05 | [Xue et al., IMC2022 TSPU](https://censoredplanet.org/papers/tspu-imc22.pdf) | Историческая stateful/in-path/partial visibility модель РФ; не аттестация deployment2026 |
| Q06 | [Cloudflare, 2025-06-26](https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/) | Whole-carrier ограничения из собственной CDN-телеметрии как контрпример «правильный cover всегда доступен». Не текущий рейтинг путей |

Q01/Q02 и crate metadata имеют отдельные capture outcomes в
`.artifacts/research/theory-02/2026-10-25-r02-002/references.json`.
Q03 прочитан через web, Q04–Q06 — повторно используемый baseline первого пакета;
новые raw-копии не заявляются. Современная полевая доступность не измерялась.
