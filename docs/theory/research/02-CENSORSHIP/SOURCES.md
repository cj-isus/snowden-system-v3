# R02 — источники первого среза

Дата сверки: **2026-10-25**. Пакет: `R02-001`, статус `READY_FOR_REVIEW`.
Это desk research и ручное чтение исходников; транспортные эксперименты не проводились.

## Воспроизводимость

[Manifest снимков](../../../../.artifacts/research/theory-02/2026-10-25-r02-01/manifest.json)
содержит 10 локальных файлов и 22 файла upstream: пути, точные commits, URL,
SHA-256 и совпадение с прочитанным Go module cache. Все 22 downloaded файла
совпали с cache побайтно по SHA-256. Это проверка происхождения прочитанного
текста, **не сборка или исполнение кода**, не аудит всей цепочки зависимостей.
Снимки отдельных файлов не являются атомарным snapshot всего репозитория.

Литература: [references.json](../../../../.artifacts/research/theory-02/2026-10-25-r02-01/references.json).
Успешный web read и успешное сохранение raw-копии различаются; ошибки capture
не превращаются в подтверждение содержания. Окончательное состояние — в
[audit.md](../../../../.artifacts/research/theory-02/2026-10-25-r02-01/audit.md).

## Закреплённые исходники

| ID | Версия / immutable pin | Прочитанные места и допустимый вывод |
|---|---|---|
| U01 | sing-box **v1.13.19**, `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | [option/vless.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/option/vless.go), [outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go), inbound: flow есть с обеих сторон и передаётся реализации. common/tls/reality_*: внешняя TLS/REALITY стадия. transport/v2raywebsocket/{client,server,conn}.go: Upgrade/ALPN/headers/early data |
| U02 | sing-vmess **v0.2.8-0.20250909125414-3aed155119a1**, `3aed155119a174c9290a4099841049f0cb275e02` | [client.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/client.go), service.go, vision.go, [vision_utls.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/vision_utls.go): поддержка Vision; TLS registry под with_utls; проверка UUID/flow. Наличие unsafe/reflection повышает цену native compatibility gate |
| U03 | MetaCubeX/utls **v1.8.4**, `cf49b0864331e156689feec6363ef9bf518a5ac7` | [u_common.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_common.go): HelloChrome_Auto = HelloChrome_133. [reality.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/reality.go): dial донора, внешний допуск и fallback. Это наш fork/pin, не обещание поведения текущего Xray |
| U04 | sing-quic **v0.6.4-0.20260803041914-d83826c306d7**, `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` | [salamander.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/salamander.go), client.go и service.go: обёртка до QUIC, salt8, auth и HTTP handler. Вывод о длине относится к обычному WriteTo; старый спор о vectorised branch здесь не переоткрывается |

Лицензии прочитаны в тех же pins и сохранены вместе с файлами:
sing-box / sing-vmess / sing-quic — **GPL-3.0-or-later** по тексту корневых LICENSE;
MetaCubeX/utls — корневой LICENSE с тремя BSD-условиями. Это scoped provenance,
не полный SBOM и не заключение о лицензиях всех файлов/зависимостей.
Новый upstream код в продукт не импортировался.

## Локальный baseline

| ID | Источник | Для чего прочитан |
|---|---|---|
| L01 | docs/CENSORSHIP-THEORY.md, §§2.4–2.5, 2.9–2.11, 3, 5, 7–11, 12.4, 13–17; тематический поиск | Отделить новое от уже известных TLS-in-TLS, ограничений Salamander, графа зависимостей, критериев E-04 и текущих OP/AD/TR |
| L02 | docs/B1-REALITY-RESEARCH.md | Прежнее основание «Vision нет»: таблица §2 и отсутствие flow в решении |
| L03 | windows/go.mod | Версии sing-box, sing-vmess, sing-quic, utls. Версия в документации сверена с manifest |
| L04 | windows/backend/render/{render.go,descriptors.json} | Три текущих семейства, Chrome preset, WS early data, отсутствие REALITY flow, общий origin/exit. validation_status — историческое поле, не актуальный live PASS |
| L05 | windows/backend/config/config.go | ValidateProtected отвергает любой непустой flow с ошибочным объяснением отсутствием поддержки ядра |
| L06 | windows/backend/render/{fdmodel.go,dependency_graph.go} | Современный граф/provenance/unknowns; не повторять уже исправленную старую scalar-модель как current defect |
| L07 | android/native/source-lock.json | Тот же core pin как source candidate; AAR hash null, stock AAR несовместим с обязательной source adaptation; native PASS не установлен |
| L08 | android/corebridge/.../configuration/NativeGraphSecurity.kt | Allowlist принимает пустой flow и xtls-rprx-vision. Это принятие поля, не доказательство native Vision или полного wire profile |

Полные локальные пути и хеши находятся в manifest. Общие документы не изменялись.

## Первичные исследования и спецификации

| ID | Источник и дата | Что взято / чего источник не доказывает |
|---|---|---|
| P01 | [Xue et al., USENIX Security 2024](https://www.usenix.org/conference/usenixsecurity24/presentation/xue-fingerprinting), paper pp.2689–2706 | Ограничения padding/mux против признаков вложенных TLS handshake. Экспериментальная работа, не доказательство внедрения такого классификатора в РФ и не сравнение нашего Vision |
| P02 | [Wu et al., USENIX Security 2023, §4.4](https://gfw.report/publications/usenixsecurity23/en/#how-the-gfw-disrupts-connections), авторская HTML-версия, modified 2024-01-12 | В изученном механизме fully-encrypted blocking затрагивал TCP, **не UDP**. Перенос результата прямо на HY2/Salamander или на ТСПУ некорректен |
| P03 | [Cloudflare, 2025-06-26](https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/) | Собственные наблюдения CDN о деградации доступа из РФ с 2025-06-09, включая разные HTTP-транспорты. Не свежая карта всех операторов и не доказательство общей AND-модели |
| P04 | [QUICstep, PETS 2026, §§3.1, 5](https://gfw.report/publications/pets26a/en/), release 2026-07-20 | Скрытый handshake и последующая миграция к доступному destination; оговорены stateless censor и отсутствие общего QUIC/IP block. Концепция не даёт разрешения protected traffic на direct |
| P05 | [CensorLess, PETS 2026, §§4.6, 6](https://gfw.report/publications/pets26b/en/), release 2026-07-20, modified 2026-09-12 | Реальное исследование FaaS-мостов. Private mode сохраняет VPS и меняет trust/performance/cost. Ни заявленная экономия, ни domain fronting автоматически не переносятся на наш масштаб/провайдеров |
| P06 | [Hysteria protocol, разделы HTTP/3 и Salamander](https://v2.hysteria.network/docs/developers/Protocol/), mutable, read 2026-10-25 | Проектные требования upstream отдельно от нашей реализации U04. Gecko/Mimic/Realms в новых docs не объявляются возможностями нашего pin |
| P07 | [Xue et al., IMC 2022, TSPU](https://censoredplanet.org/papers/tspu-imc22.pdf), DOI 10.1145/3517745.3561461 | Прямое исследование РФ: stateful in-path поведение и неоднородная видимость устройств. Историческая выборка; не полный современный детектор |
| P08 | [net4people/bbs #546](https://github.com/net4people/bbs/issues/546), opened 2025-11-14 | Первичное полевое сообщение о части домашних сетей: перечислены МТС/МГТС Москва, JustLan, LanInterCom; степень затронутости оговорена автором. Не репрезентативный эксперимент и не доказательство вреда Vision вообще |
| P09 | [net4people/bbs #490](https://github.com/net4people/bbs/issues/490), opened 2025-06-27 | Первичная реконструкция ограничения объёма одного соединения. Сочетание факторов в сообщении — гипотеза наблюдателя; не раскрытый код ТСПУ |
| P10 | [RFC 9001 §§5.2, 7](https://www.rfc-editor.org/rfc/rfc9001.html#section-5.2), May 2021 | Публичное выведение QUIC Initial keys; не расшифровка последующего защищённого application traffic. У Salamander на внешнем wire другая обёртка |

У P08/P09 сохранены датированные GitHub issue body через read-only API;
комментарии целиком не захватывались и не считаются проверенными этим пакетом.
Текущие цены, доступность VPS и актуальные условия облаков не исследовались.

## Готовое и решения о заимствовании

| Кандидат | Решение этого исследования | Что ещё нужно |
|---|---|---|
| Уже транзитивный Vision U01/U02 | **Reuse candidate**, исправить представление capability; отдельный добровольный совместимый профиль вместо смены ядра | Оба конца, native TLS wrappers, XUDP/mux/UDP-семантика, rollback и E-02/03/04/08; режим не включён |
| Уже встроенные uTLS/WS/HY2 U01/U03/U04 | **Adapt assessment**: паспорт наблюдателей и стадий вместо обещания неотличимости | Контрольный benign corpus, реальные wire captures и endpoint-specific negative cases после отдельного разрешения |
| QUICstep P04 | **Reference/defer**, свежая идея для отдельного узкого threat model | Артефакт/API/license не аудированы; generic HY2 API не предоставляет готовый integration path; не мигрировать пользовательские сайты на direct |
| CensorLess P05 | **Reference/defer**, отдельное исследование carrier/cost, не готовый VPN SDK | Код артефакта/license/security и современные provider constraints не проверены; не переносить crypto/fronting из статьи без отдельного review |

Количество найденных названий не является числом подходящих зависимостей.
