# TH-02-OBS — структурные наблюдения

2026-10-21, V2-112: implementation, **unverified**. Полная цель — TH-00…08;
тестирование/сборки/эксперименты отложены владельцем.

## Реализация

- `core/probe_http_observation.go`: единый bounded HTTP/DoH fetch, value-only
  ProbeObservation: requested route, endpoint без userinfo/path/query, стадия
  DNS/connect/TLS/headers/body, typed error, HTTP status, body bytes/limit,
  completion и monotonic elapsed. Trace callbacks синхронизированы/замораживаются.
  Содержимое body и произвольные URL/error strings не выводятся в observation.
- Native DNS/UDP/IPv6/bulk performance не выводятся из DoH/small HTTPS ответа.
  HTTP error page не используется как egress echo; DNS требует явный NOERROR/A.
- Report получает config snapshot ID, Manager ID/lifecycle generation, network
  epoch и timestamps. Смена owner/epoch/clock в процессе делает report stale.
  Manager хранит clone последнего report; aggregate PASS без steps, с failed/
  unavailable/stale facts не проходит gate. Старые lightweight fixtures могут
  требовать обновления позже; сейчас tests не изменяются.
- `failure_observation.go`: только наблюдаемые timeout/connect/TLS/endpoint/
  incomplete/unavailable классы. Номер порта/regex не создаёт allowlist, DPI,
  injected reset или freeze. Confidence — не калиброванная вероятность.
- Health принимает report только своего snapshot/owner/epoch. Запись и snapshot
  используют порядок epoch.mu→health.mu. Старые items не получают новый epoch
  label; неизвестная физическая эпоха даёт no_data. Ошибки измерителей не
  создают protected quarantine. Это не новая проверка реальной сети.
- Failed candidate work — отдельный hint: ≤64 IDs, TTL60s, scoped network epoch,
  stable retry order. Eligible set не сокращается, protected probe обязателен.
  Profile commit очищает hints. Generic switch error не превращается в
  protected_channel_unavailable с выдуманным epoch/strong probe counter.
- UI labels обновлены для новых factual codes; legacy causal names отмечены
  как исторические гипотезы. Публичный Wails shape не расширялся вручную.

## Открыто

V2-145 [DoH wire + резервные источники](TH-02-RESOLVER-OBS.md) заменяет один
hardcoded JSON resolver step: signed optional selected-channel endpoints,
shared admission/overall deadline и bounded sequential fallback, correlated wire
question/answer и fixed value attempt observations. Source-only/unverified;
остальные обязательные egress/leak gates сохраняются. Это не native DNS/UDP/IPv6
или независимый quorum, их приёмка и реализация полного контракта остаются ниже.

Compilation/runtime/concurrency/native/live приёмка отсутствует по запрету
тестирования. Полная controller decision atomicity, process hard bounds,
actual DNS/UDP/IPv6, dependency/provenance graph, staged policy promotion и
весь TH-00…08 scope сохраняются в PLAN §5.3. Gate не закрыт.
