# 01-ARCHITECTURE — Архитектура и инварианты (снимок 2026-09-20)

> Файл для восстановления контекста. Читай после сжатия контекста.
> Источник правды — docs/PLAN.md; этот файл — снимок.

## Что за проект

snowden.system — fail-closed VPN-клиент Windows для обхода ТСПУ-блокировок.
Стек: Wails v2 (Vue3+TypeScript) + embedded sing-box 1.13.19 (pinned).
Клиент Windows-only. Android не начат (Phase C).

GitHub: cj-isus/snowden-system-v3 (public, main @ 753500e, 6 commits sanitized).
Локальная копия: D:\SNOWDEN-SYSTEM-V3 (origin добавлен, коммитов нет — всё untracked).
Легаси: old_snowden/snowden-system_v2/ (справочник, не копировать).
Снапшот: .tmp/clean-export/ (sanitized copy для публикации).

## Инварианты (необъемлемо)

1. Fail-closed: protected traffic только через validated каналы; direct запрещён
   в selector/fallback/recovery. Нет каналов → BLOCKED. Никогда direct.
2. Evidence over claims: статусы planned/configured/locally-tested/live-verified.
   running/handshake/HTTP 200/открытый порт ≠ live-verified.
3. Один источник правды: docs/PLAN.md. Журнал append-only §6 (V2-XXX).
4. Секреты по ссылке: значения только в gitignored файлах; в git/chat/логи — никогда.
5. Контекст движка ≠ контекст операции: движок живёт на engineCtx (владелец Manager);
   операции Start/Reload/Stop ограничены operationCtx.
6. Controller → только через Manager: любое переключение = switchChannelCore
   (render → двойная валидация → Manager.SwitchChannel/Reload → обязательный probe → rollback при FAIL).
7. Категория ошибки = наблюдаемый факт (не гипотеза «это DPI»).
8. NetworkFacts обязателен для интерпретации отказов (V2-054).
9. urltest не второй владелец выбора. AdaptiveController решает, применяет через Manager.
10. mutex name Wails: "wails-app-" + uniqueId + "sim" (uniqueId = "snowden-system-v3")

## Каналы

channel-a-vless: vless+ws через CF CDN 443 live-verified, mux=true, carrier=cloudflare
channel-a-hy2: hysteria2+quic 8444 live-verified, salamander obfs, pin-cert
channel-a-reality: vless+tcp-reality 8443 live-verified, донор samsung.com
channel-a-shadowtls: vless+shadowtls 8445 configured (не автопереключается)
Все: origin=203.0.113.10, fd.asn=AS57043 (V2-073).
CDN-путь деградирует волнами (V2-026/039/040); HY2 жив в те же минуты.
chatgpt.com: IP датацентра → 403 challenge; решено WARP-egress (V2-041).
## Fail-closed граф

protected → selector proxy → validated channels → engine
нет каналов → BLOCKED (stopCore, fail-closed)
мерцание → fastReprobe 2×5с → если ок → продолжить
подтверждённая серия → failoverPolicy → switchChannelCore
кандидаты исчерпаны → BLOCKED + авторетрай (backoff 30s→10m)
прямой fallback → ЗАПРЕЩЁН (FR-001)

## AdaptiveController (V2-060)

HEALTHY → SUSPECTED → fastReprobe → PASS=HEALTHY / FAIL
→ CLASSIFYING (failclass.go) → DEGRADED → SWITCHING (tryFailover)
→ PASS=HEALTHY / FAIL=DEGRADED / исчерпан=BLOCKED → RECOVERING → HEALTHY

## Файлы ядра (windows/)

main.go: wails.Run, SingleInstanceLock, awaitSingleInstanceReleased (V2-080)
instancelock_windows.go: V2-080 — ожидание mutex при переключении режима
app.go: startup, lifecycle, лог, pushState, switchMode, EnableTUN/DisableTUN
corebridge.go: startCore, stopCore, failCore, switchChannelCore, watchCore, failover-on-start
failover.go: failoverPolicy, tryFailover, recordCandidateFailure (V2-075)
scoring.go: C2 — rankCandidates, healthBucket (V2-075)
controller.go: C1 — controllerSM
failclass.go: ClassifyFailure(r, channelPort, probePort), 11 кодов
diagprobe.go: runFreezeDiagnostic, freeze/silence/abrupt, L0 (V2-079)
chanhealth.go: ChannelHealth, GetChannelHealth, GetChannelIndependence
quarantine.go: D2 — deriveChannelRecoveryState
incidents.go: D3 — ClusterIncidents
bgprobe.go: D1 — фоновые TCP-хендшейки
netepoch.go: NetworkEpoch
netwatch.go: netChangeReprobe, tryPreferredChannelOnNetworkChange (V2-077)
transportpref.go: pickDefaultChannel, preferredChannelForNetworkChange (V2-077)
autoretry.go: авто-повтор
elevation_windows.go: ShellExecuteW runas, restartWithArgs, tunRequested()
killswitch_windows.go: TUN kill switch
secrettests.go: TestSecret

## Файлы backend/

backend/config/config.go: строгий парсер
backend/core/engine.go: Engine
backend/core/manager.go: Manager, SwitchChannel
backend/core/probe.go: ProtectedProbe
backend/core/proxy.go: ProxyManager
backend/core/preflight.go: DNS-резолв
backend/render/render.go: RenderClientConfig, tunnelPortFor, validateChannelExtras
backend/render/fdmodel.go: FailureDomain, Independence
backend/metadata/metadata.go: Ed25519 envelope
backend/secretvault/vault.go: DPAPI
backend/update/update.go: обновления

## Frontend

src/api/contract.ts — ЕДИНСТВЕННЫЙ источник типов
src/api/backend.ts — фасад Wails
src/api/mock.ts — dev-мок
wailsjs/go/ — СГЕНЕРИРОВАНЫ (вручную не править)

Правило §4.1.5: расширение = contract.ts → §4.1 → app.go → wailsjs → UI
Правило конкурентного слоя: appendLog под Lock = дедлок
