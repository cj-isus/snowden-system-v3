# 02-DETECTS — Дефекты, находки и решения (снимок 2026-09-20)

## Дефекты, найденные аудитом и закрытые

| ID | Дефект | Решение | Запись |
|---|---|---|---|
| D1 | Классификатор получал порт probe-цели (443) вместо порта канала → port_blackholed недостижим | ClientConfig.TunnelPort + ClassifyFailure(r, channelPort, probePort) | V2-068 |
| D2 | Провалившийся кандидат не получал FAIL → те же 2 попытки в одного → BLOCKED при живом соседе | recordCandidateFailure(target) в tryFailover | V2-075 |
| D4a | direct-контроль diagprobe не независим в TUN | tunMode параметр, not_independent статус | V2-071 |
| D4b | partial-data + timeout = timeout (не freeze) | freeze при got>0 + любой err; silence/abrupt формы | V2-071 |
| D5 | /range только на CDN-канале; 404 = отказ | live /range на VPS + verify-server | V2-078 |
| D6 | failing ≡ no_data (побитово равный score) | healthBucket первичный ключ + Evidence fix | V2-075 |
| D7 | ASN мёртв, все пары > порога при одном origin | fd.asn=AS57043 + clamp 0.25 | V2-073 |
| D10 | transportpref не переоценивался при смене сети | tryPreferredChannelOnNetworkChange | V2-077 |
| D11 | шаблон сервера ≠ прод | configs/README.md + verify-server.sh | V2-076 |
| D12 | route.final проверялся руками | verify-server.sh (FAIL → exit 1) | V2-076 |
| D14 | EOF → silent_freeze без учёта mux-контракта | hint: обе причины | V2-077 |
| D15 | self-hosted primary не enforced | probe_primary_host + валидатор | V2-072 |
| D16 | GetChannelIndependence не доехал до UI | contract.ts + facade + карточка | V2-074 |
| C1 | build-теги в 3 версиях (AGENTS) | единый = package.ps1:47 | V2-067 |
| C2 | AGENTS карта → несуществующие файлы | фактическое дерево | V2-067 |
| C3 | коллизия букв фаз | Adaptive A..D (v3) | V2-067 |
| C5 | §4.1 без GetIncidents | добавлен | V2-067 |

## Дефекты, найденные в live-эксплуатации

| Дефект | Решение | Запись |
|---|---|---|
| TFO ронял каждое соединение REALITY на Windows | TCPFastOpen удалён из render | V2-080 |
| Mutex-race: TUN EnableTUN «вылетало» | awaitSingleInstanceReleased(8s) | V2-080 |
| Listener regex (verify-server) ловил CRLF | trim + grep по границе порта | V2-078 |
| Тесты пишут в общий app.log | НЕ исправлено (дефект гигиены) | — |
| select-channel probe FAIL → rollback | правильное поведение (CDN мёртв) | — |
| CDN-path деградация волнами | failover-on-start работает (live) | V2-040 |

## Дефекты, открытые (не решены)

| Дефект | Причина | Приоритет |
|---|---|---|
| Калибровка весов C2/B2 | Phase E2 не начата (нужна история) | owner/Phase E |
| Эмпирическая корреляция отказов | D3 только кодовая | после B1 |
| B3 anti-probing для канала A | не закрыт (REALITY закрывает только B) | после B1 |
| L0-тайминги (RST-фазы, TTFB-детекция) | частично закрыто V2-079 | P2 |
| go test пишет в общий app.log | дефект гигиены | P3 |

## Анти-знание (не повторять)

«TLS-фрагментация спасёт от ТСПУ» — sing-box прямо отрицает
«uTLS = невидимость» — upstream Not Recommended
«ECH решает проблему SNI» — на CF сигнатурно заблокирован с 2024-11-05
«TCP Brutal как дефолтный CC» — без правила = 1 Mbps
«TFO работает на Windows» — tfo-go не поддерживает (V2-080)
«mux на REALITY» — V2-045 не рендерит mux для reality (консервативный дефолт)
«Второй домен = независимый failure domain» — один VPS = один домен, документировано
«Смена fingerprint после заморозки лечит» — может продлевать штраф-окно
«SSH-туннель как резерв» — под 16КБ-механикой SSH тоже исключение из заморозки