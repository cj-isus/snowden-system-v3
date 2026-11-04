# Что ограничивают имеющиеся доказательства

Эта заметка собирает содержательные оговорки из прежних статусов и итогов.
Это редакционная сводка существующих материалов; новых научных решений здесь нет.
Принятая область каждого вывода определяется [полной рецензией](REVIEW-REGISTER.md).

## Соединение и блокировки

- Framing, inner authentication, target dial и полезный ответ — разные события.
  Условное повторение outer MAC не доказывает прохождение inner authentication
  или эффект на приложение. [Разбор HRR](reviews/4H-HB01-R02-15.md),
  [полная композиция](02-CENSORSHIP/REPORT-009.md).
- Наличие ECH/exporter/early-data API не доказывает использование в actual caller.
  Branch, конфигурация, peer identity и client–server совместимость проверяются
  отдельно. [ECH](02-CENSORSHIP/REPORT-013.md),
  [exporter](02-CENSORSHIP/REPORT-010.md), [HY2 early data](02-CENSORSHIP/REPORT-011.md).
- Проценты и число блокировок без установленной методики/знаменателя/сопоставимости
  не дают оценку текущей сети РФ. Неизвестная ячейка ISP/access matrix остаётся
  неизвестной. [Решение по числам](reviews/NINE-2026-10-27-R02-014-A.md),
  [источники измерений](02-CENSORSHIP/SOURCES-014.md).

## Адаптивный выбор и статистика

- Health/latest-value projection и полная зарегистрированная когорта решают разные
  задачи. Наличие журнала не доказывает полноту producer; пропуск не равен нулю.
  [Контракт evidence](cycles/2026-10-28-contracts/C03/REPORT.md),
  [регистрация и custody](cycles/2026-10-28-contracts/Z03/REPORT.md).
- Условная alpha-композиция требует своих модельных предпосылок. Она не создаёт
  численную валидность библиотеки, causal identification или гарантию будущей сети.
  [Принятая математическая область](reviews/NINE-2026-10-27-R03-010-A.md).
- Старую product-formula без независимости и тезис о бесконечном ожидании нельзя
  переносить в требования. [Поправка R03-17](reviews/NINE-2026-10-27-R03-17-ERRATUM.md).
  Kernel/envelope/target bridge остаются отдельными условиями
  [ограниченного advisor](cycles/2026-10-28-contracts/C03/HANDOFF.md).

## Память, очереди и энергия

- Cap одной очереди не является whole-process RSS bound. Close, drain и reap
  ресурсов не тождественны; retiring generations и удерживаемые buffers учитываются
  отдельно. [Windows/HY2](04-PERFORMANCE/REPORT-010.md),
  [ресурсный контракт](cycles/2026-10-28-contracts/C04/REPORT.md).
- Method-set и callback semantics требуют exact version/caller. Source-ошибка не
  устанавливает частоту, severity или измеренный выигрыш исправления.
  [Brutal dispatch](reviews/2H-HB02-R04-10.md),
  [смешанные BBR-решения](reviews/2026-10-28-DEEP-REVIEW.md).
- Nominal keepalive interval не равен radio wake rate или расходу батареи.
  Foreground service, network exemption, wake lock и Low Power Standby имеют
  разные условия. Source reference не аттестует установленный AAR/OEM/device.
  [Android power и таймеры](04-PERFORMANCE/REPORT-011.md).

## Доверие и операции

- Returned Progress, принятые metadata, retained bytes и успешный disk commit —
  разные события. Snapshot authenticity не выдаёт current operation authority.
  [Timestamp/Progress](reviews/NINE-2026-10-27-R05-010-A.md),
  [история и reset](05-TRUST-SAFETY/REPORT-010.md).
- Пять правильных status labels без связанного immutable witness и gate на момент
  admission недостаточны для разрешения операции. Старый Success после нового
  accepted deny не возвращает прежние полномочия. [Связанный witness](cycles/2026-10-28-contracts/Z06/REPORT.md),
  [отдельно принятая operational область](cycles/2026-10-28-contracts/REVIEW-01.md).
- Z06 повторно проверял собственный прежний baseline. Его самопроверка не является
  независимой рецензией; отдельные решения руководителя учитываются по своей области.
  [Происхождение вывода](cycles/2026-10-28-contracts/Z06/HANDOFF.md).

## Android, рабочая сеть и полезный результат

- Raw UID capture, lockdown, app-rule ownership и whole-task protection имеют
  разные границы. API capability не доказывает merged artifact/native parity.
  [VPN/MDM](06-RELIABILITY-UX/REPORT-012.md),
  [UID attribution](cycles/2026-10-28-contracts/Z08/REPORT.md).
- Новый probe не доказывает перевод старых flows или завершение задачи.
  Denominator, maturity horizon, censored/missing outcomes и scope защиты нельзя
  исправить одной формулой. [Рецензия метрики](reviews/NINE-2026-10-27-R06-013-A.md),
  [endpoint/task различие](cycles/2026-10-28-contracts/Z09/REPORT.md).
- Cache и recovery kit не создают новый сервер, действующее доверие или независимый
  failure domain. [Prerequisite graph восстановления](06-RELIABILITY-UX/REPORT.md).

## Источники и проверка продукта

У источника сохраняются version/commit, caller, дата, доступность и границы чтения
в соответствующем SOURCES. Совпадение выбранных bytes не доказывает атомарность
всего дерева или source-to-artifact identity. Отчёты и scoped acceptance не
подтверждают native/device/live, скорость, батарею или превосходство VPN.

[Тематические знания](../README.md) · [Полные аргументы](README.md)
· [Ограничения дополнительных материалов](cycles/2026-10-28-contracts/APPLICABILITY-LIMITS.md)
· [Применение к разработке](../../planning/knowledge-application/README.md).
