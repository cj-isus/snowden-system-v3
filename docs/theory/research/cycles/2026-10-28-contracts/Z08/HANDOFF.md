# R28-Z08 — HANDOFF

<a id="receipt-задача-сдача"></a>
## Область и результат

28 октября 2026.

- Статус автора: **READY_FOR_REVIEW**, не ACCEPTED.
- Пакет: [REPORT](REPORT.md), [SOURCES](SOURCES.md).
- Captures: `.artifacts/research/r28-contracts/Z08/`.
- Source availability: **AVAILABLE** selected Android source/whole-core adaptation,
  manifests и module cache; artifact/native/live availability не доказана.

## Главный результат

**Минимальный публичный contract — partial mapping + original-epoch admission +
отдельный retirement**, а не универсальный mapper каждого socket в приложение.
Для ordinary same-user app rawUID сохраняется; SDK sandbox mapped publicAPI35+;
unsupported/isolated/foreign/shared cases — unknown/отказ single-app approval.
Current signer-bound observation согласуется с original app lease, но read
не atomic с package mutation. Owner cache надо bypass либо fence по epoch;
current source-кандидат strict-owner **уже bypasses** stock5-tuple cache на
новом taggedTUN admission. Это положительная дельта, не готовый AAR proof.

Уже routed flow не вызывает mapper на каждый packet: invalidation/late return,
закрытие новых admission, graceful stage, exact Binder death и parent producers/
RPC returns — отдельные receipts. Public API/read/TTL не доказывают непрерывную
current signer attribution или мгновенное прекращение всех old-flow packets.

## Scoped claims и предлагаемое решение

| Claim | Что можно принять в narrow scope | Сильнейший opposing case / ограничение | Предложение lead |
|---|---|---|---|
| **R28-Z08-A** — partial public mapper | API29 raw socket query, regular/same-user public primitives; SDK mapper35; API34 source mapper @hide, не ordinary supported reuse. Current signer-set/read purpose явно назван | isApplicationUidfalse не значит system; shared singleton filtered, explicit sharedUserId guard уже есть. Mapper35 не signer/timestamp lease; isolated classifier34 не owner mapping | **ACCEPTED только API/source scope как рекомендация**, broader universal mapper DEFERRED |
| **R28-Z08-B** — fresh current strict-owner seam | Captured overlay+wiring taggedTUN→direct platformSearcher до route/outbound, Kotlin original guard pre/post. Baseline stock-cache trace не переносится автоматически | Whole wiring/AAR not applied/built; still raw sandboxUID not normalized; callback read не atomic. Already-open flow не per-packet reevaluated | **ACCEPTED conditional source composition как рекомендация**, runtime correctness DEFERRED |
| **R28-Z08-C** — cache/epoch/admission/retirement contract | Раздельные rawUID/attributedUID/provenance/current material/original epoch; observed invalidation запрещает late usable metadata; exact owner retained до required receipts | Mutation-before-event и check-to-use worlds неразличимы; TTL/fresh double-read не socket-incarnation или atomic PM proof; death != parent-return/latest material | **ACCEPTED design/model scope как рекомендация**, product atomic enforcement DEFERRED |

Предложение scientific disposition — не назначение статусов своей работой.
Число моделей/файлов/совпадений хэшей не independent empirical acceptance.

## Что нового сверх R06-012-D

1. **Не наследован старый cached-tuple counterexample как current defect**:
   strict-owner candidate появился и прослежен на fresh sources. Stock finder
   cache200ms и persistent SessionCacheRevision разведены.
2. **API29–34 не объявлены целиком непригодными**: ordinary app identity-map
   supported условно; unsupported — именно nonregular sandbox/isolated mapping
   без public support. PublicAPI35 не blanket backward fallback.
3. **Observed invalidation proof сформулирован**: если original lease уже
   invalidated до postcheck, callback result rejected; требование stronger
   «OS installation current в каждую наносекунду» из этих APIs не выводится.
4. **Пройдены оба продолжения**: UID reuse+cached tuple countered by direct guard
   и expected-oldUID checks; shared visibility singleton countered existing
   sharedUserId deny. Platform IKEv2 refresh branch не перенесён на Builder.
5. **Сильная граница current signer purpose**: legitimate rotation history не
   automatically approve current signer, отказ pin≠доказанный compromise.

## Exact source и scope

Android min29/target36; whole-core base sing-box1.13.19
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`. AOSP frameworks/baseAndroid16
`99b01a65cc4c104933788b3143285ab6bae65827`; raw socket scope — named Connectivity
`627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a` reused own immutable captures after
hash validation (fresh remote DNS fetch failed, не freshness attestation).

Current source callers:
- `LibboxPlatform.findConnectionOwner`, `AndroidAppIdentityLookup`;
- `NativePolicyLowering.selector`, exact UID/package predicates;
- `AppIdentityMonitor.changed`, `NativeApplicationGuard`, `NativeCallbackGate`;
- `NativeWorkerSession` native tag/marker/overrides;
- `WorkerProcess.completeRetirement`, `WorkerProcessLink` remote anchor,
  `VpnSessionRepository` both-RPC/source-return/death→refresh;
- native overlay `route/snowden_owner.go`, daemon tag guards и wiring.matchRule;
- stock core route TCP/UDP/PreMatch error-before-outbound, processCache.

37selected inputs,15initial source-lock references matched; two core remote
immutable route/cache files matched local bytes. Concurrent3files wiring/
source-lock/README changed during review; retained initial/review/final cuts.
Strict-owner/mapping parts unchanged; sourceRevision3 startup/observer patches
не reviewed as whole-native success. Final dated snapshot, not moving tree guarantee.

## Ready reuse, cost и что отвергнуто

- **Reuse** public OS owner/user/package/current signing primitives и existing
  explicit original lease gates — без root/QUERY_ALL_PACKAGES/cross-user data.
- **Adapt candidate** SDK mapperAPI35+ с raw/attributed provenance и selected
  UID-binding recheck; legacy nonregular остаётся typed unsupported.
- **Reuse source seam** existing strict-owner whole-libbox overlay, не новый
  VPN-core/Go runtime. GPL-3.0-or-later; scoped AOSP referenceApache-2.0;
  matching AAR/generated API/ABI/license closure gates отдельно.
- **Reject** offset guess/hidden MODULE_LIBRARIES/first package, generic
  process-cache expiry как authority, fallback в stockAAR/непроверенный direct.
- **Цена** targeted PM/signing work на new flow, potential callback pressure,
  reconnect/старые flow losses при process replacement. Cost/latency/energy
  не измерены и не записаны0.

## Work stages и future gates

TASK1–4 пройдены: pins/callers D-trace; public mapping matrix; cache/epoch/new
admission/old-flow retirement; minimal seam+reuse. Оба пункта продолжения
(UID reuse/shared visibility и IKEv2-vs-Builder refresh) разобраны.
Paired C06 TASK прочитан адресно; готовый HANDOFF на review-stage не найден;
independent input unavailable, не ждали/не polling/не messaging.

14finite future cases **NOT_RUN** в REPORT §8: API29/34/35 mapping, shared/UID
reuse, stock/adapted tuple-cache, mutation/check-to-use, late PM result, old flow,
worker-death+RPC pending, IKEv2-refresh, signer rotation и artifact markers.
Нет executable fixtures/build/tests/native/MDM/ADB/серверов/live/probes/crypto.

## Platform consequence и неизвестное

Перед activation app-rule владельцуAndroid нужно согласовать partial capability
и metadata/schema parity: какой UID matcher использует, где retains raw/socket
principal, как отрицательная selected-mismatch включает mapped UID и как
old-owner fenced replacement получает актуальную installation. Не считать
текущий overlay уже accepted runtime artifact. Физические/OEM/socket
incarnation/old-flow/network proof не заменяются design note.

Этот handoff — конечная сдача. Старые файлы/соседние пакеты/платформенные plans,
roles/deps/bindings/settings, модели/limits, git history не менялись.
