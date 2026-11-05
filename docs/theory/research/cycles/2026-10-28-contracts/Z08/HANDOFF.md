# Android UID: частичное сопоставление, допуск и завершение владельца

**R28-Z08, срез 28 октября 2026 года. Статус автора — READY_FOR_REVIEW.**
[Полный аргумент](REPORT.md) · [Источники](SOURCES.md).
Копии: `.artifacts/research/r28-contracts/Z08/`.

## Краткий вывод

Обоснованный контракт — частичное сопоставление UID, проверка исходной эпохи
при новом допуске и отдельное завершение старого владельца. Это не
универсальное установление приложения для каждого пакета socket.

Для обычного приложения того же пользователя rawUID сохраняется.
Публичный SDK sandbox mapper доступен с API35; неизвестные, изолированные,
чужие и shared случаи не получают догаданное single-app разрешение.
Signer-bound наблюдение связано с исходным lease, но чтение не атомарно
с изменением установки package.

**Применимость:** app rules, обработка смены установки/UID и кеша владельца.
**Предел:** исходники, API и условная композиция. Точный AAR, native-паритет,
непрерывная принадлежность каждого пакета и мгновенное прекращение старого flow
этим срезом не доказаны.

## 1. Что установлено в источниках

| ID | Область | Сильное ограничение | Предложение автора |
|---|---|---|---|
| R28-Z08-A | API29 raw socket query; публичные средства regular/same-user; SDK mapper35. API34 mapper найден в source как hidden, не обычный поддержанный API | isApplicationUid=false не означает system. Один видимый package не доказывает отсутствие shared group; существующий sharedUserId guard сохранён. Mapper35 не создаёт signer/time lease | Рассмотреть API/source часть; универсальный mapper отложить |
| R28-Z08-B | TaggedTUN→direct platformSearcher до route/outbound и исходные Kotlin pre/post guards прослежены в overlay+wiring | Whole wiring/AAR не применены/не собраны; raw sandboxUID ещё не нормализован, чтение не атомарно; открытый flow не проверяется заново на каждом пакете | Рассмотреть условную source-композицию; runtime correctness не принимать |
| R28-Z08-C | Раздельные rawUID/attributedUID, происхождение, installation material и исходная эпоха; late usable metadata отвергается после наблюдённой invalidation | Mutation-before-event/check-to-use могут быть неразличимы. TTL/двойное чтение не доказывают socket incarnation или атомарность PM. Death не равен parent-return | Рассмотреть design/model контракт; атомарную защиту продукта не считать доказанной |

Здесь предложены области рассмотрения, а не присвоены статусы ACCEPTED.
Число моделей, файлов и совпавших хешей не является независимой
эмпирической приёмкой.

## 2. Дельта относительно R06-012-D

1. Старый cached-tuple контрпример не объявлен нынешним дефектом:
   в более позднем source-кандидате strict-owner уже обходит stock5-tuple cache
   при новом taggedTUN admission. Stock finder200ms и persistent
   SessionCacheRevision — разные механизмы.
2. API29–34 не объявлены целиком непригодными. Для ordinary app возможно
   условное сохранение raw identity; ограничение касается неподдержанного
   nonregular sandbox/isolated mapping. API35 не создаёт backward fallback.
3. После invalidation исходного lease до postcheck результат callback
   отвергается. Это не доказывает, что OS installation атомарно актуальна
   в каждый момент между чтением и использованием.
4. UID reuse и кешированный tuple сопоставлены с direct guard и expected-oldUID;
   shared singleton — с существующим sharedUserId deny. Ветка обновления
   платформенного IKEv2 не переносится на произвольный VpnService Builder.
5. История законной ротации signer не одобряет автоматически текущий signer
   для любой цели. Отказ pin не является доказательством компрометации.

## 3. Точные версии и вызывающие пути

Android min29/target36, whole-core base sing-box1.13.19
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`.
AOSP frameworks/base Android16
`99b01a65cc4c104933788b3143285ab6bae65827`.
Raw socket scope — Connectivity
`627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`: прежние immutable copies
переиспользованы после сверки SHA, новый remote fetch завершился DNS-ошибкой.
Это не новая проверка удалённого источника или OEM/kernel runtime.

Рассмотрены:

- `LibboxPlatform.findConnectionOwner`, `AndroidAppIdentityLookup`;
- `NativePolicyLowering.selector`, точные UID/package predicates;
- `AppIdentityMonitor.changed`, `NativeApplicationGuard`, `NativeCallbackGate`;
- `NativeWorkerSession` tag/marker/overrides;
- `WorkerProcess.completeRetirement`, remote Binder в `WorkerProcessLink`;
- `VpnSessionRepository`: оба RPC/source returns, смерть прежнего процесса,
  затем обновление материала;
- overlay `route/snowden_owner.go`, daemon tag guards, wiring.matchRule;
- stock route TCP/UDP/PreMatch с ошибкой до outbound, processCache.

37 выбранных входов и 15 начальных source-lock ссылок сопоставлены;
два immutable route/cache файла совпали с локальными байтами.
Три wiring/source-lock/README файла менялись параллельно; сохранены
начальный, review и финальный срезы. Strict-owner/mapping части не изменились;
sourceRevision3 startup/observer не проверены как полный native-успех.
Это датированный снимок, не гарантия движущегося рабочего дерева.

## 4. Допуск и завершение — разные события

Кеш владельца требуется обойти либо связать с эпохой нового допуска.
Найденный strict-owner кандидат уже обходит stock cache; поручать это заново
как отсутствующее исправление не следует.

Но открытый flow не вызывает mapper на каждом пакете. Invalidation,
поздний ответ, закрытие нового admission, graceful stage, точная Binder death
и возврат родительских producers/RPC — отдельные подтверждения.
Public API, TTL и повторное чтение не доказывают непрерывную signer attribution
либо мгновенное завершение всех прежних отправок.

## 5. Переиспользование, лицензии и цена

- Брать публичные OS owner/user/package/current signing primitives и
  существующие original lease guards, без root, QUERY_ALL_PACKAGES и cross-user data.
- Адаптировать SDK mapperAPI35+ с раздельным raw/attributed происхождением
  и проверкой selected UID binding. Legacy nonregular остаётся typed unsupported.
- Сохранить существующий strict-owner whole-libbox overlay и один runtime,
  не создавать второй VPN-core. GPL-3.0-or-later; выбранный AOSP reference —
  Apache-2.0. Точный AAR/generated API/ABI и состав лицензий проверяются отдельно.
- Не использовать догадку об offset, hidden MODULE_LIBRARIES, первый видимый
  package, общий срок process-cache как полномочия или fallback в stockAAR/direct.
- Цена: targeted PM/signing на новом flow, давление callbacks,
  reconnect и потеря старых flow при замене процесса. Latency, энергия
  и численные затраты не измерены.

## 6. Применение и открытые проверки

Основной разбор — [UID, маршрутизация и владелец VPN](../../../../android/UID-ROUTING-AND-OWNERSHIP.md).
Маршрут Android: ANDR-02/04/05 [плана](../../../../../planning/knowledge-application/ANDROID-PLAN.md)
— артефакт, завершение и правила приложений. Для PC-05
[плана ПК](../../../../../planning/knowledge-application/PC-PLAN.md)
переносится различие OS identity/кеша/допуска; Android UID не является Windows API.

До активации app-rule нужно согласовать partial capability и metadata/schema:
какой UID использует matcher, где хранится raw/socket principal, как
selected-mismatch учитывает mapped UID и как fenced replacement получает
актуальную installation. Overlay не является принятым runtime-артефактом.

В исходном review готового C06 HANDOFF ещё не было; это ограничение
того среза, не отсутствие позднейшего C06.
14 будущих случаев REPORT §8 — **NOT_RUN**: API29/34/35, shared/UID reuse,
stock/adapted cache, mutation/check-to-use, late PM, old flow,
worker death при ожидающем RPC, IKEv2 refresh, signer rotation и artifact markers.
Физический/OEM/socket-incarnation/old-flow/network результат остаётся неизвестным.

<a id="receipt-задача-сдача"></a>
<a id="r28-z08--handoff"></a>
<a id="область-и-результат"></a>
<a id="главный-результат"></a>
<a id="scoped-claims-и-предлагаемое-решение"></a>
<a id="что-нового-сверх-r06-012-d"></a>
<a id="exact-source-и-scope"></a>
<a id="ready-reuse-cost-и-что-отвергнуто"></a>
<a id="work-stages-и-future-gates"></a>
<a id="platform-consequence-и-неизвестное"></a>
