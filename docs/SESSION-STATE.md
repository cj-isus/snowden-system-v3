# SESSION-STATE — исторический снимок (2026-09-20, после V2-080)

> Устаревший снимок. С 2026-10-18 продолжение: docs/STATE.md → docs/PLAN.md.
> Не читать эту историю автоматически и не считать прежние live статусы текущими.

> Страховка от потери контекста. После сжатия: читать этот файл, затем PLAN.md §5-§6.
> Не источник правды (PLAN.md — источник), а снимок состояния сессии.

## 0. Проект: fail-closed VPN-клиент Windows (Wails v2 + sing-box 1.13.19)

Архитектура: `UI(Vue3+Wails) → App → Manager → Engine(sing-box) → ProtectedProbe`.
Принципы: fail-closed, evidence over claims, секреты по ссылке, честные статусы.
Репо: `cj-isus/snowden-system-v3` (public). Локально: `D:\SNOWDEN-SYSTEM-V3` (origin добавлен, коммитов нет).

## 1. VPS сервер

IP=203.0.113.10, AdminVPS AS57043 NL. sing-box 1.13.19.
Inbounds: vless-in WS localhost:9443(mux), hy2-in 0.0.0.0:8444, reality-in 0.0.0.0:8443(донор samsung), shadowtls-in 0.0.0.0:8445, vless-st-in localhost:9444.
nginx 443(TLS+WS→9443), /ip=200, /range=206(65536B). route.final=direct.
Домены: vpn.example.com + legacy-zone.dpdns.org (CF proxied).

## 2. Каналы (descriptors.json schema 2)

channel-a-vless: vless+ws CF 443 live-verified
channel-a-hy2: hysteria2+quic 8444 live-verified
channel-a-reality: vless+tcp-reality донор samsung 8443 live-verified
channel-a-shadowtls: vless+shadowtls 8445 configured
Все: origin=203.0.113.10, fd.asn=AS57043 (V2-073).

## 3. Фазы (PLAN §5 Phase D)

A-Evidence: V2-056/057/058/068/071/079 — закрыто
B-Infra: V2-059/073/074 — закрыто
C-Controller: V2-060/061/062/075/077 — закрыто
D-Recovery: V2-063/064/065 — закрыто
E-Lab: V2-069/070 — закрыто (21 scenarios, accuracy 20/21)
F-J: не начинаются (запрет §116)

## 4. Журнал V2-067..080

V2-067: Гигиена плана (build-теги, карта AGENTS, §4.1 GetIncidents, фазы)
V2-068: P0 порт КАНАЛА в классификаторе (был порт probe-цели)
V2-069: Controlled Lab 21 сценарий (параллельная сессия)
V2-070: Калибровка confidence (параллельная сессия)
V2-071: P1 partial+timeout=freeze; TUN direct не независимый
V2-072: self-hosted primary probe + probe_primary_host
V2-073: FD ASN AS57043 + clamp 0.25 same_origin+same_asn
V2-074: UI контракт ChannelIndependenceInfo + карточка
V2-075: P0 recordCandidateFailure + healthBucket ранжирование
V2-076: verify-server.sh + configs/README.md
V2-077: preference probe-gate при смене сети + EOF hint
V2-078: LIVE owner: /range развёрнут, verify-server все PASS, UI build
V2-079: L0-тайминги + Form(silence/abrupt) в diagnostic probe
V2-080: TCPFastOpen снят с REALITY + awaitSingleInstanceReleased

## 5. Ключевые решения

1. Fail-closed: direct запрещён; нет каналов → BLOCKED
2. Controller → Manager (switchChannelCore = probe + rollback)
3. Категория = наблюдаемый факт
4. Секреты по ссылке
5. PLAN.md = источник правды; archive/ = НЕ источники
6. healthBucket: healthy(0) > no_data(1) > failing(2) — не зависит от калибровки
7. tcp_fast_open НЕ работает на Windows (tfo-go) — убран из render
8. SingleInstance mutex race: awaitSingleInstanceReleased(8s) в main.go до wails.Run
9. Mutex name: "wails-app-snowden-system-v3sim"

## 6. Команды

Go: cd windows && go test ./... -count=1 && go vet ./... && go build -tags "with_utls,with_gvisor,with_quic,with_clash_api" ./...
FE: cd windows/frontend && npm run build
Pkg: powershell -ExecutionPolicy Bypass -File tools/package.ps1 -NoInstaller
Wails gen: cd windows && wails generate module -tags "with_utls,with_gvisor,with_quic,with_clash_api"
Server: bash configs/verify-server.sh
Export: bash scripts/make-clean-export.sh --check

## 7. Осталось (owner-блокеры)

B1-VPS-1: второй VPS (owner) — канал B, family-vs-instance
CF-TOKEN-1: отзыв токена вручную (DELETE → 403, owner сказал сам сделаю)
Live-волны: TSPU degradation для live-verify A4/D2/D3/freeze
Калибровка весов C2/B2: Phase E2 (история + ground truth)
Zen restart: подхватить прокси 1080 (user)
TUN crash: тест с новой сборкой после V2-080 fix

## 8. Решённые дефекты

D1: порт канала → V2-068; D2: FAIL кандидату → V2-075
D4a: TUN direct → V2-071; D4b: partial+timeout=freeze → V2-071
D5: /range → V2-078; D6: failing≡no_data → V2-075
D7: ASN+clamp → V2-073; D10: preference при смене → V2-077
D11: шаблон сервера → V2-076; D12: route.final verify → V2-076
D14: EOF hint → V2-077; D15: self-hosted → V2-072
D16: UI independence → V2-074; C5: §4.1 GetIncidents → V2-067
C1: build-теги → V2-067; C2: AGENTS карта → V2-067; C3: фазы → V2-067
NEW: TFO REALITY Windows → V2-080; NEW: mutex race → V2-080

## 9. Готчи (важно!)

1. Select-String -Recurse НЕ работает → Get-ChildItem | Select-String
2. -Include без -Recurse тихо не находит файлы (ложный dead code)
3. PowerShell ?? не поддерживается → if/else
4. Git Bash -lc кавычки/проценты ломаются → временный .sh файл
5. go test в sanitized-копии пишет в общий app.log (не исправлено)
6. Windows time.Since может вернуть 0 ns (sub-tick = 0 мкс)
7. Параллельная сессия редактирует файлы — проверять mtime
8. Установленный exe может быть sanitized — проверять бинарник
9. V2-номера могут конфликтовать — проверять PLAN.md

## 10. Среда сейчас

Приложение: PID 28252 --auto-connect, channel-a-vless (CDN ожил)
SOCKS: 127.0.0.1:1080; прокси socks=127.0.0.1:1080
Сторонний: v2rayN 10808/10809 (PID 22396)
Git: origin добавлен, коммитов нет, 214 файлов untracked
GitHub: cj-isus/snowden-system-v3, public, main @ 753500e (6 commits sanitized)
build/bin: только snowden-system.exe (36MB, 18.09 09:19) + wintun.dll
Installed: %LOCALAPPDATA%/Programs/snowden-system/ (та же сборка)

## 11. Изменённые файлы

docs: PLAN.md, AGENTS.md, CENSORSHIP-THEORY.md, SESSION-STATE.md
configs: verify-server.sh(NEW), README.md(NEW), deploy-vps.sh
windows: main.go, instancelock_windows.go(NEW), app.go, failclass.go(+test),
failover.go(+test), scoring.go(+test), netwatch.go, transportpref.go(+test),
chanhealth.go, corebridge.go, diagprobe.go, planv3_test.go, hardening_test.go,
lab_test.go, backend/render/{render.go,render_test.go,descriptors.json,fdmodel.go,fdmodel_test.go}
frontend: api/{contract.ts,backend.ts,mock.ts}, views/AutoProtectView.vue,
wailsjs/go/{models.ts,main/App.d.ts,main/App.js}
build: bin/snowden-system.exe (production 18.09 09:19), wintun.dll

## 12. Секреты (расположения)

vps.env: VPS_HOST,VPS_PORT,VPS_USER,VPS_SSH_KEY_PATH,CF_API_TOKEN,CF_ACCOUNT_ID
secrets/channels/: vless_uuid, hy2_password, hy2_obfs_password, shadowtls_password, vless_uuid_d, hy2_pin.pem
Vault: %AppData%/snowden-system/secrets/vault.v1.json
REALITY: .tmp/reality.env (если существует)
CF token: ACTIVE, DELETE→403, owner отзовёт вручную
