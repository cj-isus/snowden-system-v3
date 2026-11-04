# Карта вопросов и знаний

[Оглавление](README.md) · [Применение к разработке](../planning/knowledge-application/README.md)

| Практический вопрос | Что читать |
|---|---|
| Что реально можно обещать при блокировках? | [Модель угроз](foundations/THREAT-MODEL.md), [механизмы фильтрации](transports/CENSORSHIP-MECHANISMS.md) |
| Как сохранять identity и защиту при смене транспорта? | [Композиция TLS](transports/COMPOSITION-AND-TLS-CONTRACTS.md), [ECH/bootstrap](transports/ECH-DNS-BOOTSTRAP.md), [REALITY](transports/REALITY-RESEARCH.md) |
| Что означает успех проверки и как отличить его от работающей задачи? | [Probe](observability/PROBE-PROOF-BOUNDARIES.md), [корпус измерений](methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md), [DNS](observability/DNS-AND-DIAGNOSTICS.md) |
| Как выбирать канал по неполным данным? | [Policy/evidence](adaptation/POLICY-AND-EVIDENCE.md), [интервалы и полные политики](adaptation/INTERVALS-AND-POLICY-EVALUATION.md) |
| Как не потерять доверие при обновлении, откате и ошибке? | [Safety/trust](architecture/SAFETY-AND-TRUST.md), [непрерывность истории](architecture/TRUST-CONTINUITY-AND-AGREEMENT.md), [поставка](architecture/RELEASE-EVIDENCE-BOUNDARIES.md) |
| Что ограничивает скорость, память, таймеры и восстановление? | [Ресурсы](performance/TRANSPORT-AND-RESOURCES.md), [QUIC/HY2](performance/QUIC-ACCOUNTING-AND-LIMITS.md), [proxy/NAT64](observability/PROXY-DNS-NAT64.md) |
| Что особенного у Android, app routing и системных полномочий? | [Платформа](android/PLATFORM-AND-UX.md), [UID/ownership](android/UID-ROUTING-AND-OWNERSHIP.md) |
| Какие функции нужны для дома, работы и дороги? | [Функции и готовые практики](ux/COMPETITOR-FEATURES.md), [Android UX](android/PLATFORM-AND-UX.md) |
| Где границы источника и что не подтверждено? | [Ограничения выводов](research/REVIEW-REGISTER.md), [метод](methodology/RESEARCH-METHOD.md), [подробные аргументы](research/README.md) |
| Что уже реализовано и что делать дальше? | [План ПК](../planning/knowledge-application/PC-PLAN.md), [план Android](../planning/knowledge-application/ANDROID-PLAN.md), [обоснования](../planning/knowledge-application/README.md) |

Каждая содержательная заметка сохраняет источники, предпосылки и ограничения.
Поздняя поправка имеет приоритет только в своей области; она не превращает
целый старый отчёт в принятый. Доказательство в модели, проверка исходника и
работоспособность на устройстве остаются разными уровнями.
