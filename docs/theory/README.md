# Snowden System — база знаний

Теория, инженерные ограничения и источники для адаптивного VPN. Материалы
собраны по предметам. Знания сопоставлены с кодом и действующими требованиями в
[плане ПК](../planning/knowledge-application/PC-PLAN.md) и
[плане Android](../planning/knowledge-application/ANDROID-PLAN.md).
[Обоснования по темам](../planning/knowledge-application/README.md) сохраняют
источники и условия применения. План не доказывает реализацию и работу VPN.

**[Начать чтение](READING-GUIDE.md)** · [Найти ответ на вопрос](KNOWLEDGE-MAP.md) · [Планы разработки](../planning/knowledge-application/README.md)

## Знания по темам

| Тема | Основные материалы |
|---|---|
| **Модель угроз и границы обещаний** | [Исходная постановка](OVERVIEW.md) · [Противник и модель угроз](foundations/THREAT-MODEL.md) |
| **Блокировки и транспорты** | [Механизмы фильтрации](transports/CENSORSHIP-MECHANISMS.md) · [REALITY/XHTTP](transports/REALITY-RESEARCH.md) · [Композиция и TLS identity](transports/COMPOSITION-AND-TLS-CONTRACTS.md) · [ECH и DNS bootstrap](transports/ECH-DNS-BOOTSTRAP.md) · [Padding и окно наблюдения](transports/PADDING-AND-OBSERVATION-WINDOW.md) |
| **Измерения и диагностика** | [DNS и диагностика](observability/DNS-AND-DIAGNOSTICS.md) · [Что доказывает probe](observability/PROBE-PROOF-BOUNDARIES.md) · [Корпоративный proxy и NAT64](observability/PROXY-DNS-NAT64.md) · [Достоверность корпуса измерений](methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md) |
| **Адаптивный выбор** | [Policy и evidence](adaptation/POLICY-AND-EVIDENCE.md) · [Неполные наблюдения и сравнение политик](adaptation/INTERVALS-AND-POLICY-EVALUATION.md) |
| **Доверие и обновления** | [Защита и доверие](architecture/SAFETY-AND-TRUST.md) · [Непрерывность доверия](architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) · [Границы доказательств поставки](architecture/RELEASE-EVIDENCE-BOUNDARIES.md) |
| **Скорость, память и ресурсы** | [Транспорты и ресурсы](performance/TRANSPORT-AND-RESOURCES.md) · [QUIC/HY2: accounting и lifetime](performance/QUIC-ACCOUNTING-AND-LIMITS.md) |
| **Android и повседневное использование** | [Платформа и UX](android/PLATFORM-AND-UX.md) · [UID routing и ownership](android/UID-ROUTING-AND-OWNERSHIP.md) · [Полезные функции и готовые практики](ux/COMPETITOR-FEATURES.md) |
| **Ограничения знания и проверка** | [Метод и критерии доказательства](methodology/RESEARCH-METHOD.md) · [Выводы для разработки и открытые вопросы](roadmap/RESEARCH-HANDOFF.md) |

[Карта вопросов](KNOWLEDGE-MAP.md) помогает выбрать материал по практической задаче.

## Аргументы, источники и поправки

- [Подробные материалы и источники](research/README.md): полные аргументы,
  контрпримеры и exact pins, на которых основаны тематические главы.
- [Решения и ограничения выводов](research/REVIEW-REGISTER.md): принятая область,
  спорное и требующее пересмотра. Источник или формула не доказывают работу продукта.
- [Исходный авторский сборник](archive/CENSORSHIP-THEORY-before-split-2026-10-26.md)
  сохранён для сверки; его темы распределены в [карте содержания](CONTENTS.json).

## Применение

[Сводка применения знаний](../planning/knowledge-application/README.md) связывает
содержание с текущим кодом и поэтапным планом. Действующие требования:
[ПК/общие](../PLAN.md), [Android](../ANDROID-PLAN.md), [adaptive](../adaptive/PLAN.md).
Новая сводка ещё не заменяет их статусы.

Markdown-файлы в этой папке — единственный редактируемый экземпляр; Obsidian
открывает те же файлы. [Где хранится база и как открыть её](../KNOWLEDGE-BASE.md).
