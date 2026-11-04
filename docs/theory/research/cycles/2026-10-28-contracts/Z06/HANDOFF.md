# HANDOFF — R28-Z06: связанный revocation witness и operation gate

Дата28.10.2026. **READY_FOR_REVIEW**, не scientific ACCEPTED.

[REPORT](REPORT.md) · [SOURCES](SOURCES.md).

## Главный ответ

Five-result contract сохраняет нужные категории, **но их отдельной правильности
недостаточно** для operation gate. Нужны identity/custody отношения:
результаты относятся к одному owned immutable evidence bundle, связанному с
subject/root/scope/policy/intent; gate сравнивает current accepted history и
повторно оценивает давность непосредственно при admission.

Сильнейший counterexample — поздний **старый Success** после **нового принятого
deny**, затем error-to-old-cache-success. Истина старой цепочки сохранена,
но current status-dependent permit не установлен. Если baseline уже требует
такой gate-атомарной проверки под словом «current», counterexample запрещён:
нужна precision/operational definition, не отбрасывание всей классификации.

## Проверенные named subclaims и предложенные verdicts

| ID | Evidence / новый результат | Рекомендуемая disposition |
|---|---|---|
| **R28-Z06-A** | Exact Gson2.11.0→DTO→projection→checker, fresh10/10 bytes: SUSPENDED/unknown/time loss; null/coercion/DTO duplicates conditional, map duplicate и Reader EOF safeguards существуют; HTTP exception не штатный empty-set success | Поддержать scoped source/model lossiness; non-null/type declarations не runtime schema proof |
| **R28-Z06-B** | Taxonomy alone не обеспечивает related witness/current gate. T1 accepted E1 deny overtakes pending E0 verification; T2 age expires even when revisions unchanged; T3 shared mutable set loses bytes-to-proof identity | **NEEDS_REVISION sufficiency interpretation**; сохранить пять категорий, добавить join/custody/current admission requirements |
| **R28-Z06-C** | Gson strict streaming primitives vs Jackson2.18.3+NetworkNT1.5.9. Generic schema source skip `#` unknown properties; explicit loader and parser guards нужны. Licence Apache2 проверена; whole dep/security/runtime не audited | **REUSE_GSON_PRIMITIVES** for one fixed profile; **DEFER_GENERIC_SCHEMA_GUARANTEE** without separate ingress whitelist/acceptance |

Рекомендации руководителю, не присвоенные глобальные verdicts. Внутри этой
сессии baseline D ранее авторский: это **primary повторная самопроверка**, не
внешняя независимая рецензия. Separate C05/lead decision необходим.

## Strongest opposing trace и guard

1. Под request I0 принят полный допустимый E0: s NotListed, chain/policy
   constraints проходят; проверка ещё работает.
2. Принят E1: s REVOKED. H1 должен исключать старый status-dependent permit.
3. Следующий refresh даёт ошибку. E1 deny от этого не исчезает, E0 можно
   хранить как историю с его original as-of.
4. Поздний E0 callback C0 Success или fallback-lastSuccess публикуется после
   E1, хотя все five-result labels формально truthful в своём времени.
5. Без relation-to-head/current-gate caller допускает действие по E0.

**Опровергает:** достаточность labels/individually correct values без связанного
gate. **Не опровергает:** старую chain authenticity, full logical conjunction
с effective current authority, политику законного bounded continuation,
existing HTTP throw/signature/duplicate/EOF guards или работающий VPN.
Никакого actual bypass/device/status endpoint experiment.

## Минимальный witness contract (PROPOSED, без новой БД/криптосхемы)

- W связывает exact E и subject set, accepted history incarnation H, root/action
  policy revision Q, operation intent I, chain binding и clock/expiry premises.
- Все five results derived от W; latestAttempt error хранится отдельно от
  selected old cached E. Wrong join «fresh acquisition + old map» невозможен.
- Gate требует coherent source/subjects/immutable used data, current H/Q/I,
  freshness valid **at admission**, status policy + chain/action constraints.
- Если новый accepted deny предшествует admission — reject старый W.
  If admission came first — действие/отмена после него по named in-flight policy;
  instant global revocation/OS effect не обещаются.
- Mutating callback/CAS/retry не включает irreversible effect в retriable
  function. AtomicReference обеспечивает atomic reference update, не deep
  immutable contents, file durability или remote action.
- Read-only Set/val не custody. Операция читает owned immutable data или
  эквивалентно guarded lease; restore/reboot/ABA не повторно активирует старую
  incarnation/time proof. Epoch counter не лечит полностью откатанный state.

Не требуется новый mandatory network service/telemetry; existing platform
owner may hold the witness. Pure classification не становится permission owner.

## Остальные связанные cases TASK continuation

- **Error-to-oldSuccess**, даже без error-to-emptySet, теряет последнюю ошибку
  и возраст выбранного cache. Типизированный error нужен до UI/gate/fallback.
- **Time queue**: rev=head equality не enough; разрешённая давность может истечь
  при задержке, t_gate отдельный input.
- **304**: cache key URL/method/Vary+validators. ETag opaque, не monotonic floor.
  Header update legal only matching representation; failed refresh не reset age.
- **Semantic partial**: JSON EOF success не source completeness. Incomplete200
  допустим cache как incomplete по RFC, не как полный status snapshot.
- **Roots**: anchor excluded из path checker, retirement/root provider отдельно.
  Chosen JCA provider actual runtime unknown, algorithm-name registration не
  аттестация поставщика каждого вызова.
- **Schema reuse**: check type/required/enum before projection; generic
  AdditionalPropertiesValidator skip `#` means root/entry whitelist still
  нужен. PropertyNames entry guard не заменяет root extra-field guard.

## Exact ready reuse / licence / price

- **Android keyattestation**34652d3df68b8d6887a6670b00eb0856090dae6d:
  supplied Set/status callback/path verifier; manifest Gson2.11/Kotlin2.2/JDK21,
  не drop-in Android toolchain17. Actual integrated consumer not found.
- **Gson2.11.0**828a97be0f8d58108b140b77df8dc76b657f4a87:
  ready strict JsonReader/TypeAdapter primitives; small explicit supported-schema
  adapter needed, **not implemented here**; no default coercive DTO for trust.
- **Jackson-core2.18.3**cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd:
  explicit duplicate detection before tree; StreamReadConstraints + actual
  ingress-byte bounds. Defaults not global RAM bound.
- **NetworkNT1.5.9**270df62471e6a0c564dc13ae6cf2bf4116916f82:
  fixed local Draft7 schema, typeLoose/nullable disabled; map loader then
  DisallowSchemaLoader, errors not ignored. Java8 and second Jackson/validator
  stack. Generic guarantee deferred due scoped `#` keyword skip.
- Apache2 LICENSE files прочитаны; transitive licence/security/cost measurements
  absent. Memory/CPU overhead bounded only under named B/N/L/D/A/R premises;
  values not guessed, no RSS/latency estimate promoted to product guarantee.

## Пройденные stages и proof boundaries

C05 REPORT §§2–3 прочитаны адресно; готовый HANDOFF при review отсутствовал. Независимая сверка с ним не заявляется.

Future finite cases **F1–F12 все NOT_RUN** (REPORT §10): no parser fixtures,
Java/Gson/crypto/device/load/HTTP status probes; no real certs/keys/profiles/vault.

**Остаток:** независимое принятие named claim; actual product caller и выбранная
action/in-flight policy; runtime/provider/schema acceptance; durable/global
resource proof и current endpoint headers/status **UNKNOWN**.
