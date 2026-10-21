# V2-109 — редакция теории и план развития

Дата: 2026-10-21. Scope: документация; TH-00 / FR-012. Источник запроса —
авторский CENSORSHIP-THEORY, требование углубить его и дать поэтапный план.
Этот результат не означает реализацию плана или текущую доступность VPS.

## Сохранение и смысловые исправления

Прочитаны все исходные §0–6, включая §2.8–2.11 и таблицы/исторические выводы.
[Точная исходная редакция](../archive/CENSORSHIP-THEORY-pre-V2-109-2026-10-21.md)
сохранена отдельно, SHA256:
`7e8a3ef564cdcf0b0d3f434adde55a186464f9b8a626646c1401599b41a90332`.
Все 19 исходных номеров разделов/подразделов сохранены. Изменены неточные
названия §2.4a/§5; новые модели добавлены как §7–11. Это сохранение структуры
и доступности исходника, не утверждение о неизменности всех предложений.

Исправлены непосредственно в соответствующих разделах:

- AND/ML-схема ТСПУ и атрибуция active probes — гипотезы, не установленный алгоритм.
- Allowlist, доменные исключения из throttling и отсутствие внешней связи различены.
- CDN entry и VPS/server egress; обычный reverse proxy и domain fronting различены.
- HTTP chunks/Range, TCP keep-alive, carrier rotation и transport reassembly различены.
- Mux даёт разнонаправленные эффекты; blanket on/off и магический 16КБ-порог сняты.
- QUIC Initial visibility не означает расшифровку application data; CID migration
  не устраняет IP-block и не переносит произвольную сессию на другой VPN exit.
- Ответ 400, reset/timeout/x509 и порт не устанавливают причину блокировки;
  чистые серверные логи не доказывают единственную неисправность return path.
- Direct DoH не назван автоматически plaintext leak; bootstrap exception
  отделён от protected DNS policy. Совпадение IP не полное доказательство утечки.
- Latest TLS docs отделены от pinned Options 1.13.19; ошибочное утверждение
  о splithttp в pin исправлено. Native Hysteria YAML не JSON нашего sing-box.
- Юридические суммы/выводы об UI отозваны как непроверенная правовая справка;
  старая хроника оставлена с явной границей повторной проверки.

## Углубление и разработка

THEORY §7–10: роли наблюдателей/доверие, detection/enumeration/suppression/
recovery, base-rate пример, dependency graph и условные отказы, coverage/provenance,
update/pin/clock/offline trust, partial observability, safe bounded adaptation,
speed/BDP/goodput/resource model, методика paired experiments и статистические границы.

27 source definitions в §11 с периодом/географией/версией/ограничениями.
Просмотрены первичные описания IMC/USENIX 2022–2025, RFC 9000/9001/9114/9849,
полевые сообщения net4people, OONI, Cloudflare, Tor support, upstream docs и
конкретный NaiveProxy prerelease, TUF 1.0.37. Глубина чтения ограничена
соответствующими выводам доступными разделами/abstracts/docs; репликация
исследований и полный внешний source audit не выполнялись.

Shortlist reuse: NaiveProxy, XHTTP, Hysteria2, WebTunnel/Snowflake, UPGen,
TUF, Toxiproxy/iperf3, с compatibility/license/runtime limits. Ничего не установлено.
[PLAN §5.3](../PLAN.md#theory-roadmap): 9 этапов TH-00…08 с владельцами,
зависимостями, gate/stop criteria; THEORY E-01…08 задаёт будущие опыты.
Отдельный плановый parking lot содержит MASQUE (RFC 9298/9484, просмотрены)
и исследовательские направления, не выбранную поставку.

## Проверки и provenance

| Команда / область | Результат |
|---|---|
| `node .artifacts/theory/verify.mjs` | номера прежних разделов, 27 refs, local links/anchors/fences, 9 фаз/8 опытов, STATE limit, code drift и сохранённый V2-108 record; финальный JSON рядом |
| `node scripts/harness/project.mjs check --scope harness --run` | static Task/Node/doctor, exit0; exact command inputs/UTC result в .artifacts/theory/harness.json |
| Source review | web primary-source reading и pinned module cache `option/tls.go`, `option/v2ray_transport.go`; render/fdmodel.go, failclass.go, go.mod; без app/engine start |

Продуктовые Go/UI tests в V2-109 **не запускались**. Во время работы появилась
параллельная V2-108 доработка:4 Go files изменились относительно начального
snapshot и совпадают с её новым manifest. 151 Go/mod input manifest сверены;
остальной начальный список не имеет необъяснённого drift. Не приписываем
авторство этих изменений документационному срезу и не заявляем неизменность дерева.

Сохранённый binding-focused JSON прочитан:335 leaf PASS,0fail. Полный backend
exit0/105.95s — отдельный [результат V2-108](2026-10-21-binding-migration.md),
не новая проверка V2-109. Начальный arity build FAIL уже superseded, в новый
backlog как текущий дефект не перенесён. Manifest V2-108 фиксирует прежние doc
hashes; редакция V2-109 закономерно меняет THEORY/PLAN/STATE/LESSONS.

Артефакты: `.artifacts/theory/verification.json`, `verify.mjs`, `code-before.json`,
`2026-10-21-original.md`, `plan-before-journal.md`, `harness.json`;
`.artifacts/evidence/2026-10-21-theory-review.json` — итоговый manifest.

## Остаток

TH-01…08 и OFF-01…09 остаются открыты в своих областях. Ближайшая реализация —
signed HY2 pins/ID-path safety. Native OS durability/interprocess/orphan,
facade/observer/process bounds, реальные DNS/UDP/IPv6, build/race/NSIS,
оплаченная инфраструктура/live и сравнительный выигрыш не подтверждены этим обзором.
Нет заявления об абсолютной безотказности, максимальной скорости или превосходстве.
