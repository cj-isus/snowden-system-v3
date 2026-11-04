# Как совместить предложения применения

Решения для будущего общего плана по сопоставлению накопленной теории с кодом.
Срез — 28 октября 2026 года. Это условия проектирования, а не подтверждение работы
VPN, сборки, устройства или сервера. Текущие платформенные планы сохраняют силу.

## 1. Независимые исправления раньше общей интеграции

| Вопрос | Решение | Основание |
|---|---|---|
| Нужно ли ждать VPS для исправлений ПК? | Нет. Проверка допустимости транспорта, владение proxy, сохранение trust-history и source-контракт допуска пользовательского трафика доступны без сервера. Работа реального пути проверяется отдельно. | [Сеть](sections/NETWORK.md), [Windows](crosschecks/WINDOWS-INTEGRATION.md) |
| Обязательны ли сразу все транспорты? | Первый узел может поддерживать согласованные WS через nginx и прямой Hysteria2. REALITY — отдельное расширение после полного шаблона. ECH, hopping и ShadowTLS не являются условием первого полезного узла. | [Клиент–сервер](crosschecks/CLIENT-SERVER.md), [один VPS](crosschecks/ONE-VPS.md) |
| Можно ли считать нынешний ShadowTLS защищённым? | Нет: текущая композиция без требуемого внутреннего конфиденциального слоя не допускается к protected selection и отправке credentials/user payload. Доступность пункта интерфейса не означает допустимость канала. | [NETWORK N01](sections/NETWORK.md), [композиция](crosschecks/CLIENT-SERVER.md) |
| Должны ли ПК и Android ждать друг друга? | Нет. Они используют согласованные значения и версии контрактов; их платформенное исполнение, жизненный цикл и приёмка независимы. | [Контракты](crosschecks/CONTRACTS.md), [адаптация](sections/ADAPTATION.md) |
| Нужны ли новые Android2/3, БД и core-чат? | Основания нет. Один Android-владелец интегрирует native/JNI/provider/service. Изменения core оформляются ограниченной воспроизводимой серией с существующими владельцами и явным потребителем. | [Android native](crosschecks/ANDROID-NATIVE.md), [архитектура](sections/ARCHITECTURE-OPERATIONS.md) |

## 2. Допуск пользовательского трафика и измерения

В исходном срезе 28 октября [Manager.Start](../../../windows/backend/core/manager.go) движок
запускается, затем публикуется system proxy, затем выполняется lifecycle probe.
Контекст операции, требование probe и guards очистки уже существуют: их следует
сохранить. Перестановка `SetProxy` сама по себе не закрывает TUN, который может
установить маршруты раньше проверки. Нужен согласованный режим допуска только
probe до готовности пользовательского пути.

Авторизация endpoint и credentials для handshake предшествует отправке. Успех
последующего probe не может задним числом разрешить первую отправку. Protected
probe, открытый порт, работающий процесс и полезная пользовательская операция
имеют разные области доказательства.

`/ip` и необязательный `/range` выбираются из принятого профиля/measurement plan.
Не требуется новый универсальный probe-протокол или signer только ради этих
сверок. Nonce полезен лишь вместе с проверяемой привязкой ответа к попытке и
endpoint; он не превращает ответ в доказательство целого VPN-сеанса.

Источник: [сеть](sections/NETWORK.md), [контракты](crosschecks/CONTRACTS.md),
[клиент–сервер](crosschecks/CLIENT-SERVER.md).

## 3. Trust: полный workflow и действительные результаты записи

- Изменившийся указатель TUF store не означает, что новый metadata predecessor
  принят. Новый просроченный timestamp не становится допустимым progress по одному
  факту присваивания; старую принятую историю, включая просроченную, сохраняют.
- Критерий timestamp `G ∧ (H ∨ F)` остаётся предложением с конкретными
  предпосылками, единым временем verifier и отдельными snapshot checks. Он не
  является уже реализованным универсальным API.
- До переноса полного workflow нужно выбрать: одна подготовка результата после
  возврата updater либо явные платформенные barriers для сохранения каждого шага.
  Название метода не обеспечивает межшаговую durability.
- `DisableLocalCache` не делает полный TUF updater diskless. Нужны поддержанные
  storage/clock hooks или отдельно рассмотренный полный workflow с сохранением
  всех обязательных проверок. Подмена signature-only проверкой неприемлема.
- После исходных сверок V2-187 добавил Windows-local кандидат: полный pinned
  updater с явным `Storage` и bounded metadata-only memory adapter. Его следует
  рассматривать как существующий source seam, а не поручать заново. Это ещё не
  опубликованный shared API, не проверка всего SDK и не доказанная behavioral
  parity/durability. Текущий срез — `docs/tasks/TH-01-SDK-STORAGE.md`.
- V2-188 затем добавил начальную Job policy существующему update-check worker.
  Это не изоляция всех profile/pin/export SDK calls и не измеренные RSS/CPU/latency
  гарантии. Область source-изменения — `docs/tasks/TH-01-SDK-RESOURCES.md`.
- V2-183…186 уже содержат source-изменения coherent capture, selected-file I/O,
  раздельных quarantine/cleanup facts и сохранения факта commit при поздней ошибке.
  Их не следует выдавать как отсутствующие исправления. SDK internal I/O,
  directory durability, restart/native behavior остаются отдельными gates.
- При bootstrap root и первая pin policy требуют независимого одобрения,
  не полученного из того же входящего bundle. Это может быть одна понятная
  процедура одобрения двух привязанных значений, а не два случайных диалога.

Источник: [trust](sections/TRUST.md), [trust/release](crosschecks/TRUST-RELEASE.md),
[перенос SDK](crosschecks/SDK-MIGRATION.md), текущий
QuarantineError — `windows/backend/filetxn/journal_io.go` и
публикация storage facts — `windows/delivery_profile.go`.

## 4. Один Android-артефакт, последовательные версии

«Один AAR» означает один согласованный Go/native runtime в конкретной сборке.
Допустимы последовательные новые immutable версии с новым одобрением digest,
source closure, API/JNI inventory, ABI, toolchain и лицензий. Второй Go AAR для
trust создаёт дополнительную границу владения и не принят.

Подготовка build recipe, source parity и JNI inventory может идти до готовности
SDK. Промежуточный технический артефакт не включает Connect и не становится
одобренным пользовательским runtime. Для этого нужны production trust driver,
accepted-only source provider и принятая версия T01…14 в том же whole libbox.
Предлагаемый `VerifierV1` из сверки переноса пока не является существующим API.

Source-исправления resource/cancellation могут развиваться одной контролируемой
серией. Первый технический артефакт не обязан ждать всех будущих оптимизаций;
публичный выпуск обязан пройти применимые P0 resource/lifecycle gates. Timeout
не заменяет actual return/death и не освобождает retained owner.

Android AND-08-S4/S5/S6 уже выделяет header/primary navigation и проверяет дочерний
выбор. Новый общий план сохраняет это, а не создаёт второй router или повторную
переделку меню. Render, Back/restore, доступность и устройство ещё не проверены.
AND-06-S7/S8 затем добавил source guards для исходного identity batch и отдельных
package-manager/native UID шагов; их также сохраняют. Это не atomic OS cutoff,
Stop latency или доказанная полнота UID coverage.

Источник: [Android native](crosschecks/ANDROID-NATIVE.md),
[SDK migration](crosschecks/SDK-MIGRATION.md),
ADR-068 — `android/decisions/068-validated-child-navigation.md`.

## 5. Производительность и адаптация без подмены safety

Узкое исправление выбора Brutal может быть полезным отдельным результатом.
Оно не закрывает весь congestion-control, отмену, очереди и время жизни ресурсов.
Совместимая пара новых CC-компонентов остаётся ориентиром для миграции, а не
разрешением автоматически обновить зависимости.

Сначала сохраняются original owner/custody и границы count+bytes до clone/spawn,
после отмены учитывается фактическое завершение последнего владельца. Нет
«возврата бюджета» по таймауту или потере UI-интереса. Session cancellation и
whole-carrier retirement различаются.

Следующий adaptive consumer выбирается один. Для optional background suppression
нужны согласованный pressure snapshot и финальная проверка платформы. Перенос
ranking — независимый возможный срез, не обязательная вторая policy одновременно.
Статистический cohort/model bridge не блокирует Stop/revoke/recovery. Последний
health sample не заменяет полный учёт всех зарегистрированных попыток.

Источник: [performance](sections/PERFORMANCE.md),
[adaptation](sections/ADAPTATION.md), [contracts](crosschecks/CONTRACTS.md).

## 6. Один VPS и честное состояние продукта

Серверный rollout сохраняет ручную активацию: private staging → проверка → backup
известной версии → apply → отдельные restart/path checks → rollback при ошибке.
Неизвестный или неудачный rollback блокирует следующий apply. Receipt различает
declared/rendered/checked/installed/running; проверенный конфиг не доказывает
фактически работающий путь. Один узел остаётся SPOF независимо от количества
протоколов на нём. Recovery kit хранится вне узла; publisher trust и client
accepted history не сбрасываются при восстановлении сервера.

Интерфейс переиспользует текущие state/action/diagnostic компоненты. Protection,
path, task, action и clock — смысловые оси, а не требование ещё одного state owner
или общего DTO. Без admitted task producer и покрытия интервала task остаётся
unknown. Production-процент успешных задач, cohort store и telemetry SDK отложены.
Восемь повседневных сценариев служат будущей ручной/device-приёмке. Portal, LAN,
PAC/MDM и чужой VPN не разрешают автоматическое снятие защиты или blanket reset.

Источник: [один VPS](crosschecks/ONE-VPS.md), [product](crosschecks/PRODUCT.md),
[Windows](crosschecks/WINDOWS-INTEGRATION.md),
[повседневное использование](sections/PRODUCT-ANDROID.md).

## Граница этой сводки

Исходные предложения и ссылки сохранены. Принятые здесь уточнения ограничивают
их применение; спорная теория не становится реализованным требованием.
Дальнейшая последовательность собрана в [плане ПК](PC-PLAN.md) и
[плане Android](ANDROID-PLAN.md). Они учитывают более новый платформенный source
и границы новых серверных сообщений. Частичные предложения не превращаются
в полную приёмку. Проверки выполнения, native/device/live, текущая доступность
VPS и численные выигрыши этой сводкой не подтверждаются.
