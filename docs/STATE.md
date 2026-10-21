# STATE — продолжение Snowden System

Снимок: 2026-10-21, V2-109. Нормативные статусы — PLAN §3/§5.1/§5.3.

## Текущий срез

По запросу владельца: глубокая редакция CENSORSHIP-THEORY и поэтапный план.
Документационный срез; код/сеть/профили в нём не менялись. V2-108 отдельно
локально принят на synthetic данных; owner rollout не выполнялся.

## Что изменено

- Сохранены авторские §0–6 и номера подразделов, уточнены AND/allowlist/ECH,
  entry≠exit, mux/volume, chunked/Range, QUIC CID, active probing и причинность
  исторических инцидентов. Исходная копия — .artifacts/theory/2026-10-21-original.md.
- Добавлены threat/dependency/observation model, uncertainty и bounded adaptation,
  trust/update/offline recovery, speed/overhead model и экспериментальный протокол.
- 27 источников с периодом/версией/границами, shortlist reuse; новые docs ядра
  отделены от pinned sing-box1.13.19. XHTTP/splithttp в pin отсутствуют.
- PLAN §5.3: TH-00…08, owners/dependencies/gates/stop criteria; THEORY §10:
  E-01…08. Тесты/реализация/серверный доступ из плана автоматически не запускаются.
- V2-108 остаётся отдельным evidence: explicit offline publisher IDs→signed
  bindings, bounded/canonical JSON, no-overwrite output, temporary App import.

## Проверки и evidence

- evidence/2026-10-21-theory-review.md; .artifacts/theory/verification.json.
  Проверены номера прежних разделов, reference definitions, local links/anchors,
  fenced blocks, фазы/опыты и границы STATE; harness record отдельно.
  TH-00 принят только как документация; статическая обвязка exit0,
  не проверка runtime, сервера или VPN-пути.
- При чтении обнаружена параллельная доработка V2-108:4 Go inputs изменились
  относительно начального snapshot, совпадают с новым migration manifest.
  151 Go/mod input этого manifest сверены; unexplained drift нет. Это не
  заявление «всё дерево неизменно» или новый запуск Go-проверок.
- Прочитан final binding-focused JSON:335 leaf PASS,0fail; backend record exit0
  и 105.95s — из отдельного V2-108 evidence, в V2-109 не перезапускались.
- Product/UI/EXE/NSIS/race/live/admin в V2-109 не запускались; VPS/SSH/CF,
  системные proxy/DNS/WFP/TUN и реальные vault/keys не затронуты.

## Остаток и следующий шаг

1. OFF-04: signed HY2 pin identity/content, delivery/rotation/revocation и ID/path
   safety перед записью. Current owner pins не читались. Actual owner binding
   rollout требует его keys/IDs; старые strict клиенты отвергают optional fields.
2. OFF-04/06/07: power-loss/crash/reboot/ACL/reparse/interprocess/orphan cleanup.
   Hard-link publication не гарантирует crash durability и требует FS support.
   KDF gate — estimated active buffers, не RSS/GC/hard latency/API rate/несколько
   процессов; forced cancellation не реализована.
3. OFF-03/05: facade preflight и network-epoch/observer decision atomicity.
   OFF-02/06: hard bound/process isolation non-cooperative Start/Close.
4. OFF-01/02/05: running old helper/EXE migration, production subprocess;
   isolated elevated corruption/IPv4/IPv6/UDP/DNS. Production TUN не принят.
5. OFF-05/06: endpoint independence, actual DNS/UDP/IPv6, budgets, staged update.
6. OFF-07/08: native WebView2 DPI/error/blocked, race/reproducible Wails/NSIS;
   исторический runner exit201 без stdout имеет неизвестную причину.
7. OFF-09: server IaC/live после оплаты и запроса; FR-012 same-input comparative
   experiment/raw evidence. Превосходство и экономия токенов не измерены.

Продолжение: STATE → PLAN §5.3 TH-01 / §5.1 OFF-04 signed pins/path safety → код.

## Дополнительный срез GH-001

По запросу владельца готовится sanitized GitHub publication с сохранением history;
после проверки — удаление трёх явно указанных старых remote repositories.
Подробности и результат: evidence/2026-10-21-github-publication.md. TH-01/OFF-04
и остальные открытые обязательства выше сохраняются.

