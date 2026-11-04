# R28-C05 — Знание статуса, принятая история и право операции

Авторский статус **READY_FOR_REVIEW**.
Метод: независимый source/spec/model review; никаких исполнений verifier,
тестов, криптографии, native API или восстановления. Источники и read depth —
[SOURCES](SOURCES.md). Вердикты ниже — рекомендации руководителю.
Независимость здесь означает повторный разбор primary inputs и возражений;
отдельная модель/эксперт не привлекались.

## 1. Ограниченный ответ и новая дельта

Пять результатов R05-009-D §10 полезны как **семантические обязательства
потребителя**. Они сохраняют сведения, которые невозможно восстановить из
одного `Set<String>`. Число DTO, файлов или БД этим не определяется.
Ни полный status snapshot, ни успешная chain validation, ни retained prefix
по отдельности не разрешают текущую операцию.

Новое уточнение относительно baseline: нужно различать три самостоятельных
вопроса восстановления:

1. **Допустимо ли инвалидировать старые dependent floors?** Это вопрос
   принятой root lineage, конкретного rotation/reset scope и фазы перехода.
2. **Может ли старый exact envelope пройти новый quorum?** Это вопрос его
   конкретных подписей и новых полномочий; список изменившихся ключей ответа
   не даёт. Из этого следует условная необходимость различать две истории.
3. **Восстановлена ли безопасность после компрометации?** Даже законный reset
   не помогает против оставшегося достаточного скомпрометированного quorum.

В текущем Windows consumer уже есть guard для промежуточного root/reset:
`retainedRoot` просматривает **все** принятые intermediate roots, а
`retainedState` очищает timestamp/snapshot в том же возвращаемом State и
требует continuation при совпадении с прежними байтами. Поэтому теоретическую
неразличимость нельзя автоматически объявить дефектом этого consumer.
Отдельно подтверждается source path до **попытки** retention нового timestamp,
который SDK назначил до окончательной проверки expiry. Это не доказательство
успешной записи или принятия артефакта (§5).

## 2. Независимый review R05-009-D

### 2.1 Exact inputs и предел вывода

- Candidate: `android/keyattestation`, commit
  `34652d3df68b8d6887a6670b00eb0856090dae6d`; это captured source commit,
  не тождество опубликованному Maven release или установленному binary.
- Manifest этого commit: Gson **2.11.0**, Kotlin **2.2.0**, Java toolchain **21**.
  Gson source pin `828a97be0f8d58108b140b77df8dc76b657f4a87`.
- Consumer library source: `GoogleRevocationList.kt`, `Verifier.kt`,
  `provider/RevocationChecker.kt`, `KeyAttestationCertPath*`, provider registration.
  Интеграция этого candidate в Snowden данным исследованием не установлена.
- Публичные pinned files повторно получены; хэши и исходные URL сохранены.
  Некоторые вспомогательные ветви Gson прочитаны из имеющегося source archive;
  их связь со свежими центральными файлами сверяется в SOURCES.
- Не выполнялись JVM/ART, reflection, R8/desugaring, HTTP status endpoint или
  реальные сертификаты. Non-null Kotlin declaration не служит доказательством
  runtime schema validation. Ни один конкретный malformed runtime outcome
  не объявляется выполненным тестом.

### 2.2 Что выдержало проверку и что ограничивает claim

| Проверяемое утверждение | Самостоятельная проверка | Итог review |
|---|---|---|
| Невозможно восстановить SUSPENDED из revoked-only set | `filterValues { status == "REVOKED" }`; valid documents с absent s и SUSPENDED s дают один результат | Подтверждено source/model; не требует malformed JSON или догадки о reflection |
| Kotlin тип гарантирует обязательность поля JSON | Gson reflected field assignment и constructor selection не валидируют Kotlin nullability как схему | Такое обоснование отклонить. Невалидные поля могут дать ошибку либо иное reflected состояние; runtime не исполнялся |
| Любая ошибка fetch превращается в пустой set | Fetch helper бросает HTTP/IO/parse error; источники вызываются вне catch для path validation | В исходном helper такого catch-to-empty нет. Он возможен только в дополнительно выбранном caller и отдельно требует evidence |
| Любое нарушение схемы обязательно бросает parse error | STRING adapter допускает boolean/number coercion; DTO пропускает неизвестные поля; map умеет array-of-pairs | Не подтверждено. Синтаксически/десериализационно успешный результат ещё не schema-valid |
| Gson везде last-wins | Map adapter отвергает duplicate при non-null предыдущем value; reflected DTO поля читаются повторно без такого guard | Blanket claim отклонить; различать map keys, null edge и DTO member names |
| Strictness.STRICT решает всю валидацию | Доступна настройка синтаксиса; тип, enum, required/unknown members и duplicate policy — отдельные обязанности | Адаптировать готовый parser до lossy projection, не писать новый JSON parser |
| Проверяется весь переданный chain, включая anchor | CertPath исключает последний certificate; checker вызывается по certificates этого path | Обещание о trust anchor требует отдельной политики root acceptance/retirement |
| `VerificationResult.Success` означает operation permit | Параметры challenge и expected app identity имеют отдельные условия; результат не выполняет private-key operation | Отклонить такое расширение смысла |

Опубликованный Google schema profile подтверждает lowercase/unpadded serials,
два status enum, required поля и закрытые additional properties. Entry `expires`
описывает expiry сертификата и очистку списка, а не TTL snapshot. Это проверка
текущей документации, а не наблюдение реального status response. [Google schema](https://developer.android.com/privacy-and-security/security-key-attestation#certificate_revocation_status_list).

**Рекомендация R28-C05-A:** принять D как ограниченный source/model contract
после сохранения следующих границ. Это не научный ACCEPTED в общем реестре.

1. `UnknownValue` — диагностическое знание об unsupported input, не разрешение
   считать весь такой документ complete supported snapshot. `NotListed` требует
   доказанной применимости и полноты выбранной схемы; partial result нельзя
   незаметно повысить до whole-snapshot evidence.
2. Schema и duplicate policy применяются **до** map/DTO projection. Проверка
   уже построенного объекта не восстанавливает потерянные repeated members.
3. Malformed outcomes остаются условными source deductions при ordinary
   reflection; Java runtime/сборка и binary dependencies неизвестны.
4. Функции `trustAnchorsSource`/`revokedSerialsSource` и clock должны относиться
   к одному согласованному verification request. Возврат «нового set» сам по
   себе не удостоверяет источник, freshness или соответствие root scope.
5. `Verifier` регистрирует provider, но выбирает validator по имени алгоритма
   через JCA. Source review предполагает, что выбран именно изученный provider;
   фактический runtime provider не установлен. JCA допускает выбор по provider
   preference. [JDK21 API](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/security/cert/CertPathValidator.html#getInstance(java.lang.String)).

Пункты1–4 развивают уже сформулированные D ограничения; новый provider qualifier
не означает, что на устройстве обнаружен подменённый provider.

### 2.3 Отрицательное знание и давность

`NotListed(s, D)` означает отсутствие s в применимом принятом D. Из него не
следуют «никогда не отзывался», «не отозван во всём мире сейчас», доступность
private key или право операции. Старый accepted REVOKED/SUSPENDED не исчезает
из истории при acquisition failure; но это не вечное объединение всех deny.
Authorized supersession и scope должны быть названы явно. В частности,
исчезновение entry после разрешённой очистки expired certificates не является
самостоятельным историческим свидетельством unrevocation.

Давность HTTP response, возраст status knowledge и policy deadline различны.
Локальное чтение cache не обновляет age. Подходящий304 может обновить metadata
сохранённого representation; для этого нужны правила выбора response и
валидации, а не произвольный304 или совпадение hash. `Age`/`Date` и residence
time входят в HTTP age; они не дают границу мирового времени публикации отзыва.
Числовой допустимый offline interval и доверенный UTC здесь не установлены.
[RFC9111 §§4.2.3,4.3.4](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.2.3).

Текущий Google fetch helper принимает только видимый `HTTP_OK`; отдельно
полученный304 он сам не оформляет как revalidation success. HTTP cache может
обработать304 ниже этого уровня, но фактическая реализация/headers неизвестны.
Кроме того, helper возвращает set без response age/validator evidence. Нельзя
дописать достоверный provenance только обёрткой над уже потерявшим сведения set.

Primary code для review: [status projection](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/src/main/kotlin/GoogleRevocationList.kt),
[checker](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/src/main/kotlin/provider/RevocationChecker.kt),
[Gson reflected reader](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/internal/bind/ReflectiveTypeAdapterFactory.java).
Thrown error/failed future/cancellation не являются typed Success; точный
caller adapter обязан сохранять это различие, даже если SDK их не возвращает
внутри sealed VerificationResult.

## 3. Единый evidence → decision → recovery contract

Область subject должна включать применимые issuer/root identity и policy
profile: один serial вне контекста не универсальная идентичность сертификата.
Time inputs и operation authority связаны с конкретным request и purpose.
Ниже — семантика, не предлагаемая новая БД или production wire format.

| Evidence / scope / время | Допустимое решение | Допустимое восстановление и что сохраняется |
|---|---|---|
| Нет принятого snapshot; connect/read/HTTP failed | `SnapshotUnavailable`; причины stage сохраняются | Повторить bounded acquisition по разрешённому пути; не выпускать synthetic empty/good |
| Новый input schema-invalid/unsupported/resource-limited | Новый input не заменяет принятую status authority; bounded diagnostic | Оставить прежнее evidence с прежней давностью, обновить поддерживаемый parser/profile через отдельную authority |
| Refresh failed, но предыдущий snapshot ещё acceptable по явной policy | Возможно использовать **прежний** snapshot с as-of и записанной текущей ошибкой | Не freshen age, не выдавать его за успешный текущий fetch; новый permit требует остальных gates |
| Прежний snapshot stale/unknown age | Нельзя выдать свежий positive status result | Revalidation/acquisition либо bounded unavailable; accepted negative/history не стирать только из-за expiry |
| Accepted REVOKED или SUSPENDED в своём scope | Policy deny/hold с точной причиной; статусы не переименовывать | Release deny только по именованной supersession/recovery policy; ошибка сети его не отменяет |
| Complete supported snapshot, subject NotListed, acceptable as-of | Только `NotListedAsOf`, отдельно от chain result | Проверить весь обещанный scope; без authority/purpose/key/intent это не permit |
| Chain valid при выбранных roots/time/constraints | Только chain/attestation evidence в этих inputs | Отдельно проверить status policy, challenge, app identity и текущие полномочия |
| Chain/status acceptable; native key отсутствует, invalidated, требует auth или иной purpose | Операция не разрешена данным evidence | Соответствующий OS auth/re-enrollment при отдельной authority; не создавать replacement key молча |
| Current key/purpose/intent/authority/required presence и exact operation inputs подтверждены | Permit только для этого действия и актуальной source generation | При отмене/смене authority повторить gates; policy permit не тождественен успешному OS effect |
| Retained TUF predecessor ранее принят, теперь expired, reset его не инвалидировал | Сохранять historical rollback restriction, не использовать как fresh artifact authority | Новый законный successor обязан соблюдать floors; не стирать B из-за ошибки нового C |
| SDK pointer указывает на новый C, но final timestamp predicate не пройден | Новый C не становится accepted floor только от pointer/hash/log | Сохранить допустимый B; failed incoming отдельно от accepted history |
| Timestamp принят, более поздний snapshot/target отвергнут | Допустим проверенный retained prefix; artifact operation отвергается | Сохранить разрешённый prefix, повторить оставшуюся цепочку; общий Refresh error не отменяет все ранние steps |
| Законный online-role reset обязателен/начат | Не возвращать старый pre-reset cache в новый effective scope | Сохранить root continuity и восстанавливаемую фазу/coherent cut; application epoch/ban этим не стирать |
| Новый coherent post-reset successor принят | Продолжить с новым floor в новом scope | Не повторять reset при каждом чтении cache; доказательство phase может быть в существующем commit protocol |
| Reset/lineage/owner/cut неизвестны | Не угадывать release/reset | BLOCKED/continuation с точным unknown; retained inputs не являются replay authorization |

Таблица не предписывает разрывать уже принятый VPN при любой сетевой ошибке.
Политика продолжения существующего режима и полномочия новой операции имеют
разные scope. Доказательство защиты текущего трафика вне этого исследования.

## 4. Reset, две истории и сильное возражение

### 4.1 Нормативная граница

Прочитанная TUF **1.0.37**, §§5.1–5.5: fixed start time; последовательная
root continuity с old/new threshold; запись каждого принятого root; проверка
expiry конечного root; затем удаление trusted timestamp **и** snapshot после
ротации соответствующих online keys. Сброс не отменяет application history,
root lineage или отдельный запрет с иной authority.
[Versioned specification](https://theupdateframework.github.io/specification/v1.0.37/).

Фраза о rotated keys не задаёт исчерпывающего предиката для всех вариантов
overlap, threshold-only и повторного введения материала. Поэтому ниже
**явно разделены** нормативное правило, source поведение и наш модельный вывод.
Текущий product predicate не переименовывается в универсальную норму TUF.

| Переход | Что можно заключить о reset / old envelope | Current consumer / предел |
|---|---|---|
| Полная замена T/S keys через законную root chain, final root fresh | Прямой recovery scope §5.3.11; T/S cache инвалидируется | Другие floors/запреты не сбрасываются |
| `2-of-{a,b,c} → 2-of-{b,c,d}` | Old envelope с b,c удовлетворяет новому quorum; с a,b без дополнительных подписей — нет | Current material-set diff включает reset; нормативность всех partial rotations отдельно не доказана |
| Полностью разные keys, но old envelope заранее подписан достаточными old и new signers | Disjoint sets не исключают повторное принятие **тех же** байтов после законного reset | `VerifyDelegate` выбирает подписи допустимых keyIDs, не требует отсутствия чужих дополнительных подписей |
| Добавление/удаление key material с overlap | Нужно проверять конкретный новый quorum, а не слово «замена» | Current predicate замечает material-set change; одной этой строки недостаточно для claim о восстановлении после компрометации |
| Только keyID alias, материал тот же | KeyID и material identity различны; нельзя разрешить удаление истории лишь косметическим именем | `roleKeys` сравнивает identities Ed25519 public material; alias diff сам reset не включает. SDK signature matching по ID остаётся отдельным условием |
| Только threshold | Проверка подписей меняется; blanket разрешение floor reset из этого не выведено | Current predicate threshold не включает. Recovery recipe, рассчитывающий только на его повышение, должен отдельно обосновать floor treatment |
| `S → S′ → S` в последовательных roots | Endpoint equality скрывает промежуточное изменение; reuse не доказывает, что ключ никогда не компрометировался | Current `retainedRoot` проходит промежуточные roots и OR-ит reset: endpoint-only контрпример этот guard не проходит |
| Только новый root version, online material прежний | Сам номер root не разрешает reset | Current predicate не сбрасывает из-за номера |
| Законная chain, final root expired либо следующий root недоступен | Root progress и fresh online acceptance различны; положение expiry до normative reset надо сохранить в модели фаз | Current возвращаемый State может уже содержать root+очищенные caches. Это source поведение, не доказательство полного conformance/durability |

### 4.2 Две истории с одинаковым cache: условная необходимость информации

Возьмём root R7 с timestamp authority `2-of-{a,b,c}` и следующий законный R8
с `2-of-{b,c,d}`. Предположим, что выбранный recovery profile относит этот
переход к ResetAllowed. Timestamp T5 имеет допустимые подписи b,c, не истёк
в рассматриваемые fixed times и удовлетворяет прочим ограничениям.
В обоих состояниях snapshot отсутствует: T5 — ранее разрешённый timestamp-only
prefix до неуспеха более позднего шага. Остальные наблюдаемые cache/time/lineage
поля выбраны одинаковыми. Это сравнение полного выбранного cache observation,
а не разных состояний, из которых просто скрыли snapshot.

- **h_pre:** R7/T5 были приняты; R8 проверен и записан, но требуемый T/S reset
  ещё не завершён. Наблюдаемый cache: `(R8,T5)`. До продолжения нового admission
  нужно выполнить требуемый reset/barrier, не считать старое принятие post-reset.
- **h_post:** R8 принят; T/S reset завершён; тот же exact T5 снова получен и
  принят под b,c в новом scope. Наблюдаемый cache опять `(R8,T5)`. Это уже
  допустимый predecessor; повторно стирать его как незавершённый old cache нельзя.

Проверка подписи и чтение пары bytes не различают эти истории. Если обе
достижимы и нужна положительная recovery liveness, решение не может быть функцией
только от этой пары. Аналог возможен при disjoint sets с dual-signed envelope.
Если policy отвергает такой overlap, данный пример неприменим; это не повод
подставить условие молча.

**Сильное возражение:** coherent commit `(R7,T5) → (R8,∅T,∅S)` может исключить
h_pre из множества наблюдаемых принятых states. Тогда существующий atomic cut,
подтверждённая phase или replay obligation уже несут нужную информацию.
Нового поля/журнала/БД из теоремы не следует. Всегда оставаться BLOCKED тоже
не требует различения, но не обеспечивает заявленное положительное восстановление.
Один crash до окончания записи без последующего admission не доказывает нарушения.

**Проверка против текущего кода:** внутри `retainedState` новый root и очистка
T/S входят в один `next`; совпадение timestamp или snapshot с pre-reset cache
приводит к continuation. `retainDeliveryProgress` кодирует State вместе с policy
и сохраняет через существующую transaction с source/checkpoint fence.
Это конкретный guard против наивного переноса h_pre. Все writers, фактическая
durability этого cut и поведение после power loss данным source slice не доказаны.
`State.Validate` проверяет форму/границы, а не prior acceptance provenance.

### 4.3 ResetAllowed не равен восстановлению доверия — R28-C05-B

Пусть K — набор всё ещё доступных противнику приватных online keys,
S′ — новая role authority, q′ — её threshold уникальных ключей. Если

`|K ∩ S′| ≥ q′`,

то противник по-прежнему может удовлетворять подпись нового timestamp/snapshot.
Удаление cache устраняет старый version floor, но не это полномочие.
Например, для K={b,c} переход `2-of-{a,b,c}→2-of-{b,c,d}` сохраняет quorum.
Это условная криптографическая модель, не обнаружение утечки наших ключей.

Следовательно, recovery claim требует отдельно: offline root authority
сохранилась; новая authority не содержит достаточного compromised quorum;
нужные честные inputs доступны; phase/commit/time assumptions соблюдены.
Проверенная root chain авторизует изменение keys, но не доказывает уничтожение
старых приватных копий или честность каждого нового signer.

Сам removal ключа из role не равен отдельному факту его compromise. И наоборот,
явный accepted compromise deny не должен исчезать от простого reset dependent
T/S cache. Его release/reuse policy требует своей authority. Это уточнение
контракта; автоматический запрет всякого reuse здесь не предлагается.

## 5. Один current consumer path: retention попытка и отдельный отказ операции

Срез привязан к source hashes `local-preimage.json` от09:01:58Z, а не к живым
state/vault. Current manifest `windows/go.mod` содержит go-tuf **v2.4.2**;
исследованный upstream commit `f5edbde31e5507f46db2069402dc38903fe6d9d4`.

### 5.1 Условный countertrace, исходящий из accepted predecessor

Входные предпосылки: уже enrolled policy/checkpoint корректно связаны; B=T10
ранее принят, snapshot reference20; serialized public package и подписи допустимы;
online reset в этом проходе отсутствует. Incoming C=T1000 имеет reference20,
но `C.expires < SDK.RefTime` строго, поэтому equality boundary не используется.
Все source/owner/cancel/shape/resource guards выбранного trace проходят.
Это описанный вход, не созданная fixture и не выполненный сценарий.

| Шаг source path | Наблюдаемый эффект / существующий guard |
|---|---|
| `App.prepareVerifiedUpdate` → `delivery.Prepare` | Public bundle читается bounded reader; операция и journal сериализованы; current policy/checkpoint загружены |
| `PrepareArtifacts` → isolated upstream Updater → `Refresh` | Состояние клонируется, workspace отдельный, unsafe local mode выключен; package fetcher не обращается к сети |
| `TrustedMetadata.UpdateTimestamp` | После type/root/signature/version guards назначает `Timestamp=C`, затем final expiry check возвращает error |
| Updater `loadTimestamp` | При этом error не вызывает persistMetadata(timestamp,C); full Refresh заканчивается ошибкой |
| `PrepareArtifacts` после Refresh | Всё равно получает trusted pointers и вызывает `retainedState` **до** проверки refreshErr |
| `boundMetadata` и continuity | Exact original bytes C однозначно связываются с pointer; higher timestamp/snapshot floor проходит. Это binding, не receipt финального принятия |
| `retainedState` и State.Validate | C помещается в `next.Cache`; в прочитанном пути нет final-expiry guard для **нового** timestamp, форма не запрещает expired C |
| Возврат Prepare | Возвращается restriction-only Progress + error; artifact proof не создан |
| `prepareVerifiedUpdate(retain=true)` | Перед возвратом deliveryErr вызывает retention; source digest/anchor/time/binding/CAS/checkpoint/vault-owner gates ограничивают путь |
| `retainDeliveryProgress` | При прошедших gates доходит до существующего transaction/committer вызова; actual successful write НЕ установлена |
| Возврат ошибки update | До `verified.proof=progress.Proof()` и допуска артефакта этот trace не доходит |

**Рекомендация R28-C05-C:** сохранить узкий дефект retention eligibility из
R05-010 как source finding для platform owner. Не повышать его до доказанной
записи T1000, lockout на реальном диске, signature bypass или VPN exploit.

Проверенные seams: [SDK assignment/final check](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/trustedmetadata/trustedmetadata.go#L103),
PrepareArtifacts — `windows/backend/delivery/client.go:150`,
retention caller — `windows/update_delivery.go:87`,
transaction attempt — `windows/delivery_state.go:41`.

### 5.2 Противоположная крайность и ограниченное достаточное правило

Если pointer C остаётся exact ранее принятым B, B теперь expired, а новый input
отвергнут, автоматическое удаление B потеряет ранее принятый rollback floor.
Текущий код такое сохранение допускает. Исправление «фильтровать любой expired
pointer» было бы некорректно. И общий Refresh error не означает, что timestamp
отвергнут: новый допустимый timestamp мог пройти и записаться до ошибки snapshot.

Из exact assignment/final-check порядка независимо поддерживается ограниченное
достаточное правило R05-010D:

`G(C) ∧ (H(C) ∨ F(C))`.

H: exact prior-accepted predecessor, его provenance и context позволяют retention;
F: привязанный новый timestamp и root строго fresh относительно **того же** SDK
RefTime; G: exact bytes/source/shape/continuity/root/reset/ownership/cancellation
guards. Нужны isolated nonpanicking workflow, отсутствие внешней мутации SDK
pointers/RefTime и все прежние source assumptions. Для snapshot этот вывод
не переносится: после назначения там остаются другие финальные проверки.
Это restriction-only criterion, не upstream receipt, не минимум полей и не permit.

SDK RefTime берётся из `time.Now().UTC()`; current `now` дополнительно сверяется
с ClockFloor, finalPolicy и `Prepared.Check` применяют отдельную строгую expiry
границу. Ни один из этих clocks source сам не удостоверяет как истинный UTC.

### 5.3 Настоящий operation seam в том же пути

При успешном Prepare код отдельно требует полный target lookup, hashes/length,
finalPolicy, completeRetainedState. Затем `prepareVerifiedUpdate` проверяет
exact delivery proof, current RoleUpdate keys и manifest release time.
Непосредственно перед apply `withVerifiedUpdateModeLocked` снова сопоставляет
policy/checkpoint bytes, проверяет proof/keys/time и **purpose-specific version**
policy; только затем вызывает переданный filesystem callback `apply()`.

Это реальный source guard против «ChainSuccess → разрешено всё». Выполнение
callback, Windows installer/lifecycle, фактическая disk durability и runtime
identity остаются UNKNOWN. Полный аудит transaction engine/всех callers сюда
не входит; broad statements из комментариев не заменяют такого аудита.

## 6. Готовое reuse, consumer seams и цена

| Решение / exact identity | Reuse / adapt / defer | Совместимость, цена и недостающие предпосылки |
|---|---|---|
| go-tuf v2.4.2, `f5edbde31e5507f46db2069402dc38903fe6d9d4`, Apache-2.0; go≥1.25.0 | **REUSE** полный Updater; **ADAPT** только result/retention seam | Уже объявлен в Windows module. Не заменять на lower primitive ради удобного return value; это новая orchestration. Нужны source-bound eligibility и existing transaction/barrier, а не новый verifier |
| Google verifier source `34652d3df68b8d6887a6670b00eb0856090dae6d`, Apache-2.0 | **REUSE** как candidate chain/attestation engine; **ADAPT** status acquisition до projection и caller contract | JVM/Kotlin dependency graph, Java21 toolchain и actual Android/runtime compatibility не проверены. Не копировать JVM engine в Go. Off-device verification имеет отдельные hosting/availability/privacy издержки, здесь не развёрнуто |
| Gson2.11.0 source `828a97be0f8d58108b140b77df8dc76b657f4a87`, Apache-2.0 | **REUSE** lexer/stream reader и explicit Strictness; **ADAPT** bounded schema adapter выбранного profile | Проверять token types до STRING coercion, required/enum/unknown/duplicate members до DTO/map. Цена — один узкий parser adapter и request/result plumbing; количественно не измерена. Reflection DTO не нужен для доказательства схемы |
| Новый общий schema-validator dependency / fork всего verifier | **DEFER** | Пока нет доказанного gap, оправдывающего новый graph/ABI/лицензионную поверхность. Validator после collapse duplicates не восстанавливает имена; ни один новый пакет не установлен |
| Существующий `secretvault/crypto_windows.go`: CryptProtectData/UnprotectData, `CRYPTPROTECT_UI_FORBIDDEN`, NULL prompt | **REUSE** OS encryption и текущий owner/transaction seam | Source уже использует DPAPI. DPAPI ciphertext не является доказательством TPM custody, current signer purpose или доверенного времени. Доступность именно этого профиля Windows не проверена |
| Android `BiometricPrompt.CryptoObject` API28 и `KeyPermanentlyInvalidatedException` API23 | **REUSE по отдельному platform requirement**, не custom unlock | OS различает auth-per-use/time-based доступ и permanent invalidation. Адаптер должен сохранить отказ/отмену и scope действия. Совместимость устройства, authenticator/key configuration и фактическая операция UNKNOWN |
| Windows `NCryptSignHash`, `NCRYPT_KEY_HANDLE`, `NCRYPT_SILENT_FLAG` (documented Vista+) | **DEFER**, пока нет отдельного требования к private-key operation custody | Готовый KSP API предпочтительнее собственной криптографии, но не drop-in замена DPAPI или timestamp receipt. Silent failure не означает согласие пользователя; actual provider/key/operation не исследовались |

Лицензии трёх исходных проектов получены по exact pins; Apache-2.0 допускает
адаптацию с выполнением условий распространения, notices и обозначения изменений.
Это не аудит всех transitive dependencies/NOTICE всех будущих binary packages.
OS API использует platform SDK/runtime terms; Apache-лицензию библиотеки на него
переносить нельзя. Никаких оценок «0 рублей», часов внедрения или ускорения нет.

Источники OS semantics: [DPAPI](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata),
[Android CryptoObject](https://developer.android.com/reference/android/hardware/biometrics/BiometricPrompt.CryptoObject),
[key invalidation](https://developer.android.com/reference/android/security/keystore/KeyPermanentlyInvalidatedException),
[NCryptSignHash](https://learn.microsoft.com/en-us/windows/win32/api/ncrypt/nf-ncrypt-ncryptsignhash).

### 6.1 Минимальная точка адаптации без потери различий

Для candidate Google engine один согласованный immutable status snapshot
получается и проверяется **до** revoked-only projection. От него отдельно
выводятся status-policy result и REVOKED set для существующего checker.
SUSPENDED не маскируется именем REVOKED; отдельный policy gate может deny оба.
Это устраняет повторную acquisition в середине одного chain request и сохраняет
сведения. Реальная latency/память такого адаптера не измерены.

Далее caller связывает chain result и status disposition с issuer/root/profile,
challenge/request, current key, purpose, intent, generation и обязательной OS auth.
Attestation относится к утверждениям при создании/аттестации key; операция
позднее может встретить invalidated/missing/locked key. Даже свежая status list
этого не опровергает. Auth при критическом import/recovery и разрешение фонового
VPN reconnect должны иметь явно разные purpose policies; обязательная биометрия
на каждое соединение из данного исследования не следует.

Для Windows delivery нужная точка уже существует: `retainedState` решает,
что разрешено отдать как prefix, а `Prepared`/`CheckDeliveryProof` и повторный
purpose gate решают допуск конкретного update. Не объединять эти полномочия
в один bool и не добавлять второй storage engine.

## 7. Независимое оппонирование, неизвестное и будущая приёмка

### 7.1 Что способно опровергнуть вывод

- Если требуемая status policy намеренно равна только membership REVOKED,
  утеря SUSPENDED не опровергает **такую узкую** membership функцию. Она
  опровергает расширенное обещание full status knowledge или иной consumer policy.
- Если current consumer запрещает все состояния h_pre coherent atomic cut,
  conditional information necessity не требует нового сохранённого marker.
- Если новый C прошёл final predicate, а упал более поздний snapshot или только
  SDK workspace persistence, общий error сам по себе не запрещает retained C.
- Если exact B был принят раньше и остаётся в разрешённом scope, нынешняя expiry
  не превращает его историю в rejected incoming. Простая expiry-фильтрация
  была бы контрпримером предлагаемому исправлению.
- Если новая authority уже исключает достаточный compromised quorum, пример
  §4.3 неприменим. Неизвестный набор K не заполняется предположением об утечке.

Проверка paired availability выполнена один раз на review stage **09:10:09Z**:
Z06/Z07 TASK доступны, HANDOFF отсутствуют. **AWAITING_INDEPENDENT_INPUT**;
ожидания/polling/сообщений не было. Результат самостоятельный; совпадение мнений
не объявляется независимым runtime доказательством.

### 7.2 Явные UNKNOWN

1. Actual Google/JVM/ART binary/provider/dependency identity, ABI/desugaring,
   exact reflection exception outcomes, root-source lifecycle на устройстве.
2. Real endpoint headers/cache/revalidation, origin publication latency,
   допустимый offline interval и authenticated UTC.
3. Normative ResetAllowed для всех overlap/add/remove/threshold/reuse вариантов;
   принятие конкретного product profile остаётся решением руководителя/owner.
4. Фактические keys/compromise set K, доступность honest successor inputs.
5. Успешный disk commit, power-loss recovery, native identity/operation,
   полный inventory writers/callers transaction engine. Source fences не proof этого.
6. Quantitative CPU/RAM/latency/token/cost benefit; измерений нет.

### 7.3 Конечные будущие cases — все NOT_RUN

Это текстовые критерии для отдельного разрешённого verification package;
fixtures, executable cases и команды исполнения не создавались.

| Case | Требуемый результат будущей проверки | Сейчас |
|---|---|---|
| V1 — supported absent/SUSPENDED/REVOKED против malformed/unknown enum/partial read | Разные typed outcomes; ни один unavailable/partial не превращается в empty-good; существующие duplicate/EOF guards сохраняются | NOT_RUN |
| V2 — refresh unavailable при acceptable старом snapshot, затем stale | Ошибка сохранена; age не обнуляется; explicit old-evidence policy; accepted deny не исчезает | NOT_RUN |
| V3 — matching/mismatching304 и cache restore | Только допустимое representation обновляет HTTP metadata; certificate expires не используется как snapshot TTL | NOT_RUN |
| V4 — accepted B + новый expired C=T1000 | Новый C не становится accepted floor; permitted B сохраняется; artifact proof отсутствует | NOT_RUN |
| V5 — valid новый timestamp + bad snapshot; expired B + rejected incoming | Различить ранее принятый prefix и отвергнутый input; не удалять B и не отвергать все prefix только по Refresh error | NOT_RUN |
| V6 — overlap, dual signatures, `S→S′→S`, restart на root/reset boundary | Учитываются concrete quorum, phase/coherent cut и successor; приложение не сбрасывает unrelated history | NOT_RUN |
| V7 — source/checkpoint/purpose меняются между prepare и apply | Повторный exact source/proof/current purpose gate отказывает до callback; отдельно проверить native/disk effect | NOT_RUN |
| V8 — attestation/status приемлемы, actual key invalidated/locked/не того purpose | Нет silent replacement/reset или inherited operation permit; typed cancel/auth/re-enrollment требует нужной authority | NOT_RUN |

<a id="8-итог-по-этапам-и-рекомендация-руководителю"></a>
## 8. Итоговые выводы и ограничения

1. **Stage1 выполнен source/model:** independently пройден exact parser →
   projection → checker/provider/caller; выполнен guard crosswalk D.
2. **Stage2 выполнен:** единая таблица evidence/decision/recovery связывает
   as-of, цепочку, history и текущую operation authority, без synthetic good.
3. **Stage3 выполнен:** opposing histories, coherent-cut возражение и quorum
   compromise condition разобраны; обязательное новое поле/store не выведено.
4. **Stage4 выполнен source-only:** ready reuse/pins/licenses/qualitative cost,
   current end-to-end update path до refusal/apply seam; paired input отсутствует.

**Предлагаемый verdict:** R05-009-D — поддержать как ограниченный semantic
contract с §2 qualifiers. Для reset принять условную necessity при достижимых
неразличимых histories и отрицательное quorum-утверждение §4.3; универсальный
ResetAllowed и product durability **не закрывать**. Current new-expired-pointer
retention — узкий source finding; правка принадлежит Windows owner и должна
сохранять prior accepted history и уже существующие operation gates.

Ни scientific ACCEPTED, ни implemented/local/native/live PASS не выставляются.
Старые авторские reports, общие главы и product files не редактировались.
Статическая упаковка09:30:20–22Z: все8 локальных Markdown links существуют,
все26 записанных source capture hashes совпали;15 baseline/current inputs
сохранили исходные hashes. Receipts: `doc-review.json`, `preservation-review.json`
в private C05 captures. Эти результаты относятся к документации и source identity.
