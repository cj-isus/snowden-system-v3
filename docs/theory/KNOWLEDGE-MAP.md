# Карта вопросов и знаний

[Оглавление](README.md) · [Термины](GLOSSARY.md) · [Применение к разработке](../planning/knowledge-application/README.md)

У каждого вопроса ниже указан основной разбор. Дополнения раскрывают другой
участок задачи или содержат подробное обоснование; они не создают второй
независимый ответ на тот же вопрос. В спорной части сначала читается
[применимая рецензия или поправка](research/REVIEW-REGISTER.md).

| Практический вопрос | Основной разбор | Дополнения |
|---|---|---|
| Что реально можно обещать при блокировках? | [Модель угроз](foundations/THREAT-MODEL.md) | [Механизмы фильтрации](transports/CENSORSHIP-MECHANISMS.md) |
| Как сохранять шифрование и проверку нужного участника в цепочке транспортов? | [Композиция TLS](transports/COMPOSITION-AND-TLS-CONTRACTS.md) | [REALITY и совместимость](transports/REALITY-RESEARCH.md) |
| Как связаны ECH, DNS и выбранный сервер? | [ECH и начальное соединение](transports/ECH-DNS-BOOTSTRAP.md) | [DNS-наблюдения](observability/DNS-AND-DIAGNOSTICS.md) |
| Когда заполнение и маскировка трафика дают полезный результат? | [Padding и окно наблюдения](transports/PADDING-AND-OBSERVATION-WINDOW.md) | [Модель противника](foundations/THREAT-MODEL.md) |
| Что подтверждают подпись ответа, внешний IP и отсутствие обнаруженных утечек? | [Границы сетевой проверки](observability/PROBE-PROOF-BOUNDARIES.md) | [Метод измерения](methodology/RESEARCH-METHOD.md) |
| Как разделить пользовательский DNS, начальное разрешение шлюза и проверочный запрос? | [DNS и диагностика](observability/DNS-AND-DIAGNOSTICS.md) | [ECH](transports/ECH-DNS-BOOTSTRAP.md) |
| Где выполняются DNS, авторизация и NAT64 при работе через прокси? | [Прокси и фактический путь](observability/PROXY-DNS-NAT64.md) | [UID и привязка сокетов](android/UID-ROUTING-AND-OWNERSHIP.md) |
| Как выбирать действие по неполным и меняющимся данным? | [Адаптивная политика](adaptation/POLICY-AND-EVIDENCE.md) | [Интервалы и сравнение политик](adaptation/INTERVALS-AND-POLICY-EVALUATION.md) |
| Как учитывать пропуски и сравнивать эффект целой политики? | [Неполные наблюдения и оценка политик](adaptation/INTERVALS-AND-POLICY-EVALUATION.md) | [Достоверность набора измерений](methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md) |
| Как сохранять защиту при обновлении, ошибке и восстановлении? | [Защита и доверие](architecture/SAFETY-AND-TRUST.md) | [Непрерывность истории](architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) |
| Какие сведения о доверии должны переживать смену ключей и репозитория? | [Непрерывность доверия](architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) | [Поставка и первое исполнение](architecture/RELEASE-EVIDENCE-BOUNDARIES.md) |
| Что доказывают проверенный файл, подпись установщика и ответ системного API? | [Границы проверки поставки](architecture/RELEASE-EVIDENCE-BOUNDARIES.md) | [Научные решения](research/REVIEW-REGISTER.md) |
| Что ограничивает скорость и время полезного восстановления? | [Транспорты и ресурсы](performance/TRANSPORT-AND-RESOURCES.md) | [QUIC/HY2: счётчики, память и время](performance/QUIC-ACCOUNTING-AND-LIMITS.md) |
| Почему ограничение очереди или размер окна не равны пределу памяти? | [Учёт ресурсов QUIC/HY2](performance/QUIC-ACCOUNTING-AND-LIMITS.md) | [Нагрузка и восстановление](performance/TRANSPORT-AND-RESOURCES.md) |
| Кого перехватывает Android VPN и что разрешает lockdown? | [UID, маршрутизация и владелец VPN](android/UID-ROUTING-AND-OWNERSHIP.md) | [Android и мобильные сценарии](android/PLATFORM-AND-UX.md) |
| Как организовать мобильность, фон и понятный интерфейс Android? | [Платформа и UX](android/PLATFORM-AND-UX.md) | [Повседневные функции](ux/COMPETITOR-FEATURES.md) |
| Какие функции полезны дома, в дороге и на работе? | [Сценарии и готовые практики](ux/COMPETITOR-FEATURES.md) | [Android UX](android/PLATFORM-AND-UX.md) |
| Как проверить силу утверждения и достаточность источника? | [Метод исследования](methodology/RESEARCH-METHOD.md) | [Качество измерений](methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md), [ограничения](research/LIMITATIONS.md) |
| Что реализовано и какой следующий этап разработки? | [План ПК](../planning/knowledge-application/PC-PLAN.md) и [план Android](../planning/knowledge-application/ANDROID-PLAN.md) | [Обоснования применения](../planning/knowledge-application/README.md) и указанные в планах реестры владельцев |

Основной разбор сохраняет источники, предпосылки и ограничения. Его место
в этой карте не означает принятие всех утверждений. Поздняя поправка имеет
приоритет только в своей области. Вывод модели, факт об исходниках и работа
артефакта на устройстве подтверждаются разными данными.
