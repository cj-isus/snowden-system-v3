# 04-JOURNAL-SUMMARY — Журнал V2-067..V2-080 (снимок 2026-09-20)

## Полная таблица записей этой сессии

| ID | Дата | Статус | Краткое содержание |
|---|---|---|---|
| V2-067 | 2026-09-17 | done | Гигиена плана: build-теги единые, карта AGENTS → фактическому дереву, §4.1 + GetIncidents, фазы Adaptive A..D, THEORY-ссылки |
| V2-068 | 2026-09-17 | implemented + unit green | Порт КАНАЛА в классификаторе: ClientConfig.TunnelPort + ClassifyFailure(r, channelPort, probePort); port_blackholed теперь достижим |
| V2-069 | 2026-09-17 | implemented + unit green | Controlled Lab: эмулятор + 21 сценарий + benchmark (accuracy 20/21, miss=LAB-015) — параллельная сессия |
| V2-070 | 2026-09-17 | implemented + unit green | Калибровка confidence-констант E2 (3 значения выровнены) — параллельная сессия |
| V2-071 | 2026-09-17 | implemented + unit green | partial-data + timeout = freeze; TUN direct-контроль не выдаётся за независимый; 3 квадранта covered |
| V2-072 | 2026-09-17 | implemented + unit green | probe_primary_host в дескрипторе; валидатор отклоняет third-party primary; reality/shadowtls → self-hosted |
| V2-073 | 2026-09-17 | implemented + unit green | fd.asn=AS57043 + clamp 0.25 (same_origin+same_asn); все пары < порога |
| V2-074 | 2026-09-17 | implemented + UI build green | UI: ChannelIndependenceInfo тип + getChannelIndependence facade + карточка в AutoProtectView; wailsjs сгенерированы |
| V2-075 | 2026-09-17 | implemented + unit green | recordCandidateFailure (провалившийся кандидат получает FAIL); healthBucket первичный ключ ранжирования; Evidence только для живого факта |
| V2-076 | 2026-09-17 | implemented + smoke-tested | verify-server.sh (инварианты живого сервера); configs/README.md; deploy-vps.sh ссылка |
| V2-077 | 2026-09-17 | implemented + unit green | tryPreferredChannelOnNetworkChange (probe-gate PASS → preference через switchChannelCore); EOF-hint обе причины |
| V2-078 | 2026-09-17 | live-verified | /range развёрнут на VPS (nginx location + range.bin 65536B); verify-server все PASS; UI build ok; CF token 403 (не отозван) |
| V2-079 | 2026-09-20 | implemented + unit green | L0-тайминги (TTFBus, ElapsedUs) + Form (silence/abrupt/no_data) в diagOutcome; freezeForm чистая функция |
| V2-080 | 2026-09-20 | implemented + unit green | (1) TCPFastOpen снят с REALITY (tfo-go не поддерживает Windows); (2) awaitSingleInstanceReleased(8s) — mutex race fix для EnableTUN |

## Фазы (PLAN §5 Phase D после консолидации)

| Фаза | Статус |
|---|---|
| Adaptive A (v3) — Evidence Core | кодовое ядро закрыто; live-волна ТСПУ — owner |
| Adaptive B (v3) — Independent Infra | v1 закрыта; эмпирическая корреляция после B1 |
| Adaptive C (v3) — AdaptiveController | кодовое ядро закрыто; калибровка C2 — Phase E2 |
| Adaptive D (v3) — Recovery | закрыта; live-verify — за волнами владельца |
| E — Controlled Lab | закрыта (лабораторный контур); калибровка C2/B2 и live-волны — за владельцем |
| F–J | не начинаются (запрет §116) |

## Параллельная сессия

V2-069/070 были сделаны параллельной сессией (lab_test.go, failclass.go confidence).
V2-065/066 тоже параллельные. Не пересекались по коду, кроме:
- lab_test.go: 2 вызова ClassifyFailure приведены к новой сигнатуре (V2-068)
- failclass.go: конфликта нет (разные строки)
- failover.go: моя правка (recordCandidateFailure) + их правка (logWaveIfAny) — обе в текущем файле

## Ключевые уроки за сессию

1. Классификатор обязан получать порт КАНАЛА, не порт probe-цели (все встроенные дескрипторы — https/443)
2. Провалившийся кандидат обязан получать FAIL-факт — иначе детерминированный scoring выбирает его снова
3. Failing и no_data давали ПОБИТОВО равный score при весах 0.30/0.15 — нужен первичный ключ healthBucket
4. Wails SingleInstanceLock: второй процесс при живом первом → os.Exit(0) молча
5. ShellExecuteW runas возвращается сразу — старый процесс ещё держит mutex
6. tcp_fast_open НЕ работает на Windows (tfo-go) — снимать для Windows-only клиента
7. TSPU freeze = тишина ПОСЛЕ частичных данных (не RST) — fetchRange должен это различать
8. Системный прокси меняется приложением, но браузер (Zen) читает его только при старте
9. go test в sanitized-копии пишет в общий %AppData% app.log
10. Параллельная сессия может редактировать файлы — проверять mtime и PLAN.md