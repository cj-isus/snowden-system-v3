<a id="r28-c05--handoff-руководителю"></a>
# R28-C05 — статусное знание и полномочия операции

**Авторский статус:** READY_FOR_REVIEW. 28 октября 2026. Source/spec/model-only; scientific ACCEPTED не присваивается.

Основной аргумент: [REPORT](REPORT.md). Pins/read depth/failures:
[SOURCES](SOURCES.md).

## Ответ

Статусное знание, accepted history и operation authority нужно сохранять
раздельно. Это семантические обязательства; из них не следует новый verifier,
протокол подписи, отдельная БД или обязательный новый phase field.
В REPORT§3 единая evidence→decision→recovery таблица с явными scope/time/authority.

## Claims и рекомендуемый verdict

| Claim | Что проверено / что нового | Рекомендация, не scientific decision |
|---|---|---|
| **R28-C05-A** — R05-009-D review | Exact Gson2.11.0 → DTO/projection → checker/provider/caller; valid absent/SUSPENDED collision подтверждён без malformed runtime предпосылки. Существующие map duplicate/EOF и throw guards сохранены | Поддержать D как semantic contract; не как full runtime guarantee/минимум полей. Сохранить schema-before-projection, supported scope, consistent per-request sources и actual provider qualifier |
| **R28-C05-B** — три вопроса recovery | ResetAllowed, повторная admissibility конкретного old envelope и устранение compromised quorum — разные predicates. Если `|K∩S′|≥q′`, reset не отнимает у K новое полномочие | Поддержать условный model result. Universal ResetAllowed для overlap/threshold/reuse остаётся UNKNOWN; новая authority и честный successor требуют отдельных оснований |
| **R28-C05-C** — current retention seam | R05-010 source finding независимо прослежен в current files до transaction attempt; новый expired pointer может войти в Progress после Refresh error. Нового runtime finding не объявляется | Передать Windows owner узкую eligibility правку; actual successful disk commit/lockout/exploit не доказаны. Не удалять ранее accepted expired B и не запрещать весь prefix только по общему error |

## Сильнейшее возражение / существующий guard

Две истории с одинаковым `(R8,T5)` и отсутствующим snapshot различаются фазой:
pre-reset cache либо законно повторно принятый post-reset T5. При overlap b,c
или dual signatures exact envelope может удовлетворять обоим quorums.

**Однако** существующий coherent cut `(R7,T5)→(R8,∅T,∅S)` может исключить
первую историю. Current `retainedRoot` просматривает intermediate roots,
`retainedState` возвращает root+reset в одном next и continuation для прежних
байтов. Поэтому это не доказанный missing-field дефект consumer и не требование
нового журнала. Actual power-loss durability/all-writer invariant не проверены.

## Проверенный end-to-end source slice

`prepareVerifiedUpdate(retain=true)` → `delivery.PrepareArtifacts` → full SDK
Refresh → pointer assignment → final expiry rejection → retainedState →
Progress+error → source/checkpoint/vault-owner fences → transaction **attempt** →
deliveryErr. Артефакт в этом countertrace не получает proof и отвергается.

При успешном пути остаются отдельные hash/length/full-chain/finalPolicy gates;
перед apply повторяются source/proof/current keys/time и purpose-specific version
checks. Кодовый guard не заменён заявлением об успешной native операции.

<a id="что-разумно-передать-разработчикам-после-решения-руководителя"></a>
## Применение к разработке после рецензии

1. **Windows retention:** на имеющемся seam отличать exact prior-accepted B
   от нового C и оценивать C относительно того же SDK workflow/RefTime;
   сохранять root/reset/source/owner guards. Ограниченное достаточное правило
   `G ∧ (H ∨ F)` — REPORT§5.2; оно не переносится автоматически на snapshot.
2. **Recovery profile:** документировать принятый ResetAllowed scope, coherent
   commit/admission barrier и модель compromised keys. Ни root version bump,
   ни фраза «ключи изменились» не являются всем recovery proof.
3. **Status consumer, если выбран Google candidate:** bounded schema/status/
   age/error acquisition до lossy set; existing engine использовать для цепочки,
   отдельную status policy — для SUSPENDED/unknown/as-of. Не маскировать SUSPENDED
   как REVOKED и не оборачивать failed future в synthetic success.
4. **Custody:** сохранить существующие OS primitives и owner transaction;
   current key/purpose/auth failure не лечить молчаливым replacement key/reset.
   Новый CNG/biometric обязательный flow без отдельного требования не предлагается.

Это предложения, не изменения product code/PLAN и не разрешение исполнения.

## Reuse и стоимость

- go-tuf v2.4.2 `f5edbde31e5507f46db2069402dc38903fe6d9d4` — уже объявлен
  Windows module; reuse full workflow, adapt узкий result seam.
- Google candidate `34652d3df68b8d6887a6670b00eb0856090dae6d` + Gson2.11.0
  `828a97be0f8d58108b140b77df8dc76b657f4a87` — reuse parser/chain engine,
  adapt preprojection contract; JVM/Android binary compatibility UNKNOWN.
- Все три source projects Apache-2.0; redistribution notices/модификации и
  будущие transitive binary licenses требуют соблюдения. No install/fork/deps change.
- DPAPI уже используется; Android CryptoObject/invalidation и Windows NCrypt
  остаются готовыми OS options по отдельному requirement. Не обещать TPM/presence
  или native success по одному API/source name.
- Цена описана через adapter/ABI/storage/hosting obligations; token/runtime/
  CPU/RAM/деньги/часы внедрения не измерялись и не заполнены нулями.

## Состояние программы и остаток

Все четыре stages TASK выполнены в source/model scope. Дополнительные ветки
«rejected incoming vs expired accepted predecessor» и один current consumer path
до отказа/apply seam разобраны. Future V1–V8 конечны и все **NOT_RUN**.

Независимый paired review Z06/Z07 не входил в этот source-срез.
Согласие с их позднейшими результатами не установлено данным пакетом.

Не закрыты исполнением: exact runtime/provider/native identity, actual headers/
status publication time, authenticated UTC, disk durability/power loss, keys K,
all-writer proof и performance. Эти UNKNOWN не спрятаны в статусе completion.

