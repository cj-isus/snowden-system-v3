# PC-03 — границы HTTP/DoH отчёта и семейства системного пути

2026-11-05, V2-212, Windows **source/unverified**. Часть полного
[PC-00…09](../planning/knowledge-application/PC-PLAN.md), после
[network handoff](PC-03-NETWORK-HANDOFF.md). Новых измерений или разрешений
на тестирование нет; существующий core/worker/network pipeline не запускается.

## Проблема и готовые компоненты

Existing ProbeObservation содержит typed HTTP phase/body/timing/requested route,
а DNS step — bounded correlated DoH wire observation с попытками. App `toView`
отбрасывал эти структурные facts и отдавал только name/status/detail/comparison.
Общий ProbeReportView уже используется Dashboard/Statistics/TestCard; его
расширение сохраняет единый UX. HTTP success не доказывает native TCP/UDP или
IP family: VLESS/QUIC могут иметь другую внешнюю транспортную реализацию,
а egress IP/DNS A answer — не локальный маршрут. Строки Detail не источник
машинной классификации, RTT/скорости/отсутствия утечек.

## Реализация

- Новый cohesive `core/probe_scope.go` — pure projection `DescribeProbeScope`,
  не измеритель/контроллер/verifier. До16 existing steps, только value поля,
  без сети, JSON/Detail parser, адресов, тел ответов, credentials или новых tasks.
  Основной report, required steps и engine private permit не изменяются.
- HTTPS group связывает actual named step с `egress_http`, engine/SOCKS requested
  route и тем же endpoint. DoH group связывает свой actual step с
  `resolver_doh_wire` и exact last existing attempt/count в фиксированном массиве.
  Typed schema, complete bounded positive body, HTTP200, body phase, no error,
  supported role/status и timestamp window обязательны для observed.
- Все named HTTPS steps должны завершиться; mixed missing/partial не становится
  общим observed. DoH success описывает ответ выбранного fallback endpoint,
  не независимость источников или native UDP. Legacy без typed facts остаётся
  unmeasured; stale/invalid window/partial/inconsistent — incomplete либо
  unmeasured. Observed означает полученный прикладной ответ в этом отчёте,
  не текущий live route, скорость или успешный общий VPN.
- Четыре native клетки IPv4/TCP, IPv4/UDP, IPv6/TCP, IPv6/UDP и local-safety
  остаются **unmeasured**. Это отсутствие доказательства в поддержанном формате,
  не mock/измеренный отказ/нулевой трафик. Public Native TUN flow/admission label,
  порт, профиль, remote IP, DNS A или HTTPS response не заполняют эти клетки.
  Принятый native источник потребует отдельного контракта и приёмки.
- Value-only scope добавлен к App ProbeReportView; aggregate OK вычисляется
  прежним normalized report. Existing scope копируется без shared slices/maps.
  Core ProbeReport/worker IPC8/generated namespace не меняются; это optional
  описательное расширение UI DTO, не новый command или forwarding boolean.
- Handwritten UI contract/strict mapper поддерживают только schema1/finite
  application states и unmeasured native cells. Historical+observed противоречие
  либо неподдержанная схема не получают positive отображения. Отсутствующее
  scope не подставляется из имён шагов, preview, default capability или флагов.
- Wails JS GetState возвращает raw bridge result в просмотренном source.
  Mapper использует существующий bounded-shape/nativeRecord approach; прямого
  window.go обхода нет. Старый generated model constructor может отбросить новое
  поле: в этом случае UI честно показывает отсутствие области измерения.
  Generated files не патчились; обычная generation и compatibility ещё OPEN.
- Existing state mapper больше не требует PASS от supported informational OS
  control. Есть хотя бы один required step, каждый required status=pass и
  backend OK=true; skipped, unavailable, pending, invalid role или только
  informational steps не создают overall OK. Это UI aggregation, не native grant.
- Existing ProbeReportView показывает одну семантическую таблицу с caption,
  column/row headers и текстовыми статусами. Семь строк, два переносимых столбца,
  existing color/type tokens; новых карточек/router/store/framework нет.
  Loading/empty/missing scope/historical предусмотрены в source, старые props
  остаются совместимыми. TestCard с report только из steps не получает fake scope.

## Skills и границы проверки

Применены frontend-postgres и modern-web-guidance. Из локального
`responsive-table` взята semantic foundation (table/caption/thead/tbody/th scope).
Двум столбцам не нужен отдельный scrolling region/sticky header или преобразование
строк в карточки; такие варианты не добавлены. Guide не доказывает rendered UX.

Source3 Go/2 TS/1 Vue/4 docs; gofmt3 exit0, ручной source/caller/diff и byte/SHA
inventory: `.artifacts/memory/family-path-source-evidence.json`. No tests/fixtures/
build/compile/vet/typecheck/parser/check/doctor/install/generation/SDK/helper/
product/native/browser/admin/live/DNS/server/owner-values/other-role/chat/subagents.
Installed local guide search/retrieve — чтение справочника, не запуск продукта.

Открыты: Go/TS/Vue compilation/API fixtures, optional DTO/binding generation и
older constructor compatibility; actual current/stale/mixed/partial/last-attempt/
informational/skipped поведения; screen reader/keyboard/940×600/scaling125–200%/
layout/contrast/browser; реальные native IPv4/IPv6/UDP/TCP/capture/WFP/route/PKI/
provider/path, worker DNS-flight handoff между сетями, resources/RSS/latency.
Ни одна native клетка и ни одна platform acceptance этим source-срезом не закрыта.

TUN gate=false, TUN unavailable, schema2 install unsupported. Все PC00…09 и
прежние FR/TH/OFF/COM/RQ/TR/TH00…08, W3b/SDK/interprocess/durability/UX/release/
comparative остаются OPEN. Следующее source — original worker DNS-flight
invalidation/retirement при смене сети с сохранением здорового established flow.
