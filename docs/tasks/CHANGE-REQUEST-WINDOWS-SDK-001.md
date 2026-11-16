# WIN-SDK-001 — точки интеграции trust/adaptive для ПК

2026-10-25. Автор/owner: `windows`, thread
`01a0c989-b620-7f90-a021-e7499d835468`. Адресаты: владельцы `trust` и `adaptive`,
когда роли активированы и опубликованы контракты. Основание:
[WINDOWS](FIVE-STREAMS/WINDOWS.md), [порядок интеграции](FIVE-STREAMS/README.md).
Это локальный запрос API, **не сообщение другому чату и не изменение его модуля**.

## Текущая версия и точки подключения

Текущий Windows source snapshot — V2-187, unverified; значительная часть дерева
untracked, Git HEAD не идентифицирует все текущие исходники. На повторном чтении
2026-10-25 `shared/trust/` отсутствует; появился source-only adaptive API1 в
`shared/adaptive/` и его consumer request ADP-002. Номер semver-релиза не указан.
Windows этот модуль ещё не импортирует; module/parity acceptance не выполнена.
`windows/go.mod` сохраняет существующие зависимости, включая go-tuf/v2 v2.4.2.

| Семантика сейчас | Фактическая точка Windows | Требуемая граница |
|---|---|---|
| Profile/pin/epoch/key roles/clock/validity | `windows/backend/metadata/` | Pure bounded проверка точных owned bytes и retained state; I/O/clock sampling/storage остаются платформенными |
| Complete TUF artifact batch, final policy и private proof/progress | `windows/backend/delivery/client.go`, `policy.go`, `pin_policy.go` | Общее принятие полного batch без публикации частичного полномочия; SDK не пишет platform store |
| Prefix revocations, approval/deadline и commit | `windows/onboarding_*`, `profile_cutover.go`, `trust_lifetime.go` | Windows retire/staging/fixed4/clock floors/intent остаются единственным применяющим владельцем |
| Scoped observations и atomic Ready | `probe_observation.go`, `observer_publication.go`, `tunnel_boundary.go` | Платформа подтверждает actual worker/config/selector/LUID/intent/epoch; переносимый verdict не даёт Ready |
| Health/failure candidates/actions | `chanhealth.go`, `failover.go`, `controller.go`, `netepoch.go` | Pure recommendation с определёнными причинами; применяет только Windows Manager под свежим gate |
| Optional native DNS факты | `native_probe_scope.go`, `native_probe_view.go`, `native_probe_action.go` | Bounded evidence с собственной свежестью; не замена обязательному protected probe |

Это карта потребителя, не полный extraction plan общего модуля. Существующая
Windows реализация остаётся основанием до опубликованного API и явной миграции;
временная вторая trust/policy реализация не создаётся.

## Запрос к trust: WIN-SDK-001-T

Опубликовать в своей зоне версию переносимого контракта и exact changed paths:

- Входы: owned bounded bytes, complete profile/epoch/global pin policy artifacts,
  independently approved enrollment anchor, retained authority/history/floors/
  cumulative revocations и явно переданное наблюдаемое время. Указаны суммарные
  allocation/work caps, отсутствие скрытого download/OS/storage и cancel seam.
- Выход: непротиворечивый immutable candidate/progress с приватной привязкой к
  точным bytes/roles/source/epoch/policy, validity/deadline и различимым отказом.
  Public digest/JSON label не может изготовить новый validated proof.
- Раздельно описать restriction-only accepted prefix и полноценное принятие
  профиля: revoke может сохраняться после позднего отказа; prefix не расширяет
  authority и не снимает clock/pending/retirement barriers.
- Сохранить threshold TUF, epoch/origin continuity, antirollback/equivocation,
  global/local bans, allowed future Next и initial/current/next/recovery различия.
  Windows не передаёт SDK право включать TUN, менять WFP или записывать vault.
- Дать source compatibility/parity mapping старых потребителей, migration boundary,
  примеры публичного формата без owner data и невыполненные acceptance gates.

При опубликованной версии Windows сам меняет `go.mod`/adapters/store/publisher
и UI в своём срезе. Старую логику не удаляет до разрешённой parity/OS приёмки.

## Запрос к adaptive: WIN-SDK-001-A

- Версионированный pure input/outcome DTO: owner/session/intent/epoch/freshness и
  измеренные stage/bytes/completion/attempt facts, unknown/abstain и measurement
  infrastructure failure отдельно от channel failure. Общий SDK не наблюдает ОС.
- Детерминированные stay/reprobe/diagnose/switch/recover/blocked рекомендации с
  причиной, expected subject, bounded candidate set и явной стоимостью/бюджетом.
  Рекомендация не получает direct fallback, trust/protection authority или право
  менять worker/сеть. Старый outcome отбрасывает Windows gate.
- Согласовать значение TTL/clock discontinuity, epoch0 и independent source
  assertions. Fixed Cloudflare/Google DNS endpoints не доказывают независимость
  пути/quorum или состояние обязательного egress/leak gate.
- Не создавать второй scheduler/Manager/health database/transport owner; mapping
  текущего порядка выбора и parity с baseline публикует owner adaptive. Будущие
  replay/shadow/canary/comparative проверки сейчас не запускаются.

## Критерии интеграции и текущий следующий шаг

Принятая версия фиксируется Windows вместе с artifact/source identity, границами
и списком реальных consumers. При несовместимости — новый адресный запрос,
без редактирования чужих shared каталогов. Полнота all9/старых P0 сохраняется.
Trust API пока не обнаружен; adaptive API1 опубликован как source/unverified
предложение, не принятая Windows dependency. ПК P0 продолжается независимо.

Первым следующим P0 Windows остаётся OFF-01/02: source review исходного owner
и fail-closed/intent при отмене/ошибке/helper cleanup во всех собственных entrypoints,
с устранением конкретных обнаруженных дефектов. OFF-06 pair updater/rollback
после введения sibling helper также открыт. UI consumer optional DNS — отдельный
последующий срез, не замена P0. Source-only review не закрывает ни одного OS gate.

Тесты/fixtures/build/vet/doctor/SDK/crypto/gen/native/live/install/deploy запрещены
действующим запросом. Ничего из этого в данном срезе не запускалось.

## V2-158 — receipt adaptive API1 и следующий adapter срез

Прочитаны только опубликованные API/consumer request/module metadata, без чужих
STATE/PLAN или чатов. Source receipt SHA256 (не runtime/parity evidence):

| Вход | SHA256 |
|---|---|
| `docs/adaptive/API.md` | `60ec8e80fff7dcf4b9ff0ecd28b86776efbf7bffe0dfcfe96dacecb4bd07f591` |
| `docs/adaptive/CHANGE-REQUEST.md` | `91b101ccd15461c6dedb4f191c10b6633e5e778f17c0adfd7b80f9dc4de9e319` |
| `shared/adaptive/go.mod` | `0a4b5b5cd86409069f5a3fc86b47e0169e4e3a4fb49bf432f6548c17d67106bd` |

Module `github.com/snowden-system/shared/adaptive`, Go1.26, API version1;
operations Eligible/Rank/Prefer, <=64 descriptors/candidates/facts/attempts,
IDs<=64 ASCII, correlation refs<=128 printable ASCII. RequiresPlatformGate=true
не authority: actual probe/reserveObservedSwitch/Manager/protection/clock остаются
Windows. API1 пока не покрывает будущие typed outcomes/budgets ADP-003/004.

Windows adapter должен один раз снять coherent applied descriptor/selector/
ChannelID + health/D2/recent/dependency snapshot и explicit Now/Scope; startup
получает Eligible.IDs, failover — ReachableIDs. Preserve server_ports projection,
D2 aging, failed descriptor missing/empty candidates и old epoch behavior. API1
строже отвергает duplicates/malformed/nonfinite inputs; соответствие текущим
fallbacks требует решения до consumer switch, не silent truncation/retagging.

После текущего P0 — адресный Windows-only adapter/migration plan; до отдельно
разрешённой parity приёмки не заменять production controller и не оставлять две
независимо выполняемые policy. go.mod/lockfiles/dependencies не менялись.
Открытый WIN-SDK-001-A request дополняется API1 receipt, не закрывается полностью.

## Узкое предложение для общего LESSONS

Optional diagnostic completion не обновляет обязательное protected-path
наблюдение: отдельные value facts, subject и expiry предотвращают перенос
DNS success/measurement failure в VPN Ready/health. V2-149 source design,
runtime доказательств нет. Предложение оставлено здесь по новым границам записи;
общий LESSONS этим чатом не изменяется.

## V2-167 — Windows metadata I/O seam / 2026-10-26

В собственном backend/filetxn появился original Windows read owner; pair update
и delivery cache используют eager ID snapshots/checked Close. Source/unverified;
этот слой не заменяет TUF verification и не владеет shared trust module.

По чтению pinned [updater.go v2.4.2](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/updater/updater.go)
и [config.go](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/config/config.go):
DisableLocalCache прекращает persistMetadata/EnsurePathsExist, но не
loadLocalMetadata. Empty LocalMetadataDir даёт относительное os.ReadFile;
это не memory-only operation. Публичный storage callback в этих API не найден.
GetTrustedMetadataSet выдаёт parsed state; он не передаёт original serialized
bytes, нужные retained cache. Флаг/UnsafeLocalMode/private fields не изменены,
custom TUF workflow не создаётся. Источник raw-byte ограничения:
[metadata types v2.4.2](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/types.go).

Дополнение WIN-SDK-001-T: нужен public bounded metadata storage adapter с
явными Read/Persist/Close/Cleanup outcomes и retained original resource receipts,
либо документированный pure bytes workflow с теми же upstream verification
semantics. Нужны exact raw verified root/timestamp/snapshot/delegation bytes,
сохранение restriction-only prefix после позднего отказа, отсутствие artifact
proof при incomplete batch, budget/cancel contract и ошибка cleanup отдельно
от missing. Re-marshaled metadata не заменяет исходные signed/hash-bound bytes.

V2-168: own initial workspace writes/cleanup переведены на retained native owner
и bounded known-name cleanup, source/unverified. Late cleanup failure сохраняет
verified prefix, но убирает artifact proof. Внутренние SDK операции остаются
отдельным запросом seam; следующий source review — retained-prefix/antirollback
при local-read/persist failure. Новый TUF workflow не реализуется.
Пустой path/отключённая запись не считаются решением resource custody. Это
локальная source-запись/API request; чужие чаты/модули не изменены. Public schema2,
all9/OS/DR/resources/parity acceptance остаются открыты, tests/build запрещены.

## V2-169 — accepted inputs вместо persisted output / 2026-10-26

Windows delivery теперь связывает public parsed SDK objects с unique original
protected cache/successfully delivered inputs; ambiguous raw serialization
запрещает proof. Дополнительные retained floors не позволяют пропущенному local
read сбросить историю; accepted online-key rotation учитывает промежуточные
roots и разрешает отдельный timestamp/snapshot reset/continuation. Полный batch
требует exact retained parent links через SDK; safe prefix переживает late errors.
Это source/unverified, только gofmt8; tests/build/SDK/native не запускались.

WIN-SDK-001-T дополнен требованием явных raw-byte receipts для **принятого**
prefix независимо от успеха persistence, отдельно от full artifact authority.
Нужны documented intermediate expired timestamp/nonfinal snapshot semantics,
reset при authenticated online key rotation, continuity после local-read error,
неоднозначность исходных bytes и cleanup outcome. Нельзя выводить raw receipt
из произвольного повторного JSON marshal. Текущее object matching привязано к
pinned v2.4.2; shared реализацию/публичный API оно не заменяет. SDK opaque I/O,
resource retirement, parity и все platform acceptance gates остаются открытыми.

V2-170: Windows public input I/O переиспользует `filetxn.BoundedReadSlot` с
отдельными owners для App sidecars/export и SDK seed. Common checked Close/
quarantine/identity checks source-only; platform I/O остаётся у Windows.
[Контракт и остаток](TH-01-PUBLIC-READERS.md). Это не public shared-trust release
и не решение opaque internal SDK I/O; WIN-SDK-001-T/parity/native gates открыты.

V2-171: Windows policy/checkpoint/envelope storage readers используют те же
platform original slots с отдельной I/O-only serialization, confirmed-missing
и typed local-trust error semantics. Decode/shared verifier находятся вне I/O
lock; source binding/history/crypto остаются прежними. Source/gofmt4+subset2,
без tests/build/SDK/native. Shared API не изменён; WIN-SDK-001-T и platform
persistence/durability/concurrency acceptance по-прежнему открыты.

V2-172: [Windows stored writes](TH-01-STORED-WRITES.md) удерживают bounded
single-file original owner, точный encoded checkpoint и checked publication/
Close. Неопределённый outcome блокирует paired reader/writer; trust/SDK policy
не дублируется. Four-file import/recovery journal пока имеет отдельный I/O,
его original receipts/interprocess/durability и SDK internal I/O остаются открыты.
Source/gofmt only; shared API/manifests/modules не изменены.

V2-173: [actual Windows import journal I/O](TH-01-IMPORT-JOURNAL.md) использует
retained per-journal read/stage-new/target write slots; uncertain outcome остаётся
process-local recovery barrier независимо от path presence. Fixed full-set
backup/BeforeCommit/rollback и caller trust policy сохранены. Native directory
creation/move/delete ownership, interprocess/durability и SDK internal I/O receipts
остаются отдельными незакрытыми gates. Source/gofmt11+3+2 only, без tests/build/
SDK/native; shared API/manifests/modules не изменены.

V2-174: тот же journal retains original Windows parent/directory handles до
native relative create/no-replace moves/known-child hash-bound deletion/checked
Close. Stage writer sharing exception private и ограничено его exact directory.
Профиль/SDK authority остаётся у callers; shared contracts не менялись. Далее
fixed target parents/delete, cross-process/directory durability, runtime monotonic
guards и прежний SDK internal I/O seam. Source/gofmt10+2 only, все gates unverified.

V2-175: original deduplicated parent tree удерживается через actual fixed-target
Commit/Recover; relative original erase/checked absence probe используется при
возврате к old absence. Cold restore приобретает цели после full-backup validation;
completed outcome отделён от sticky retirement fault. Shared SDK API/manifests
не менялись. Далее cross-process real-writer coverage/directory durability,
runtime monotonic guard и прежний SDK internal I/O request. Source/gofmt8+1+1 only.

V2-176: Store и metadata writers разделяют original native writer lock; runtime
read/derive/save удерживает один scope. Unknown I/O/Close сохраняет receipt и
barrier, completed Store outcome не отменяется retirement fault. Vault writers
и повторный prepared-vault fence внутри Store ещё требуют интеграции; complete
cross-process coverage не заявлена. Shared SDK не изменён, internal I/O seam,
coherent reads/directory durability/monotonic/native/OS gates остаются открытыми.
Source/gofmt11+comment1 only, no tests/build/SDK/native; schema2 unsupported.

V2-177: vault original bounded I/O + exact-source CAS подключены к тому же journal
writer scope; final prepared-vault comparison теперь внутри Store.BeforeCommit
для profile/prefix/pin imports. CLI/legacy migration участвуют, binding explicit;
startup storage errors блокируют readiness. Source/gofmt9 only, no tests/build/
SDK/native/DPAPI. SDK внутренние I/O/shared contracts не изменены; monotonic
checkpoint/coherent readers/durability/native/API compatibility/OS gates открыты.

V2-178: отдельный Windows runtime checkpoint guard запечатывает всю identity/
history кроме monotonic observed TimeFloor; legacy bootstrap rederives installed
signed envelope через существующий PrepareAcceptance. Shared SDK/API не менялись;
source/gofmt4 only. Coherent readonly snapshots/retained identity, SDK internal I/O,
durability/restart/API/native/OS/DR/resources/all9 acceptance остаются открытыми.

V2-179: Windows owned ProfileSnapshot и existing-only shared reader scope на
прежнем journal lock. Runtime/profile facts/pin wake используют один captured
policy/checkpoint/envelope; projection без I/O, anchored history guard общий с
V178. Missing initialized lock — явная ошибка; no implicit bootstrap/fallback.
Shared SDK/API не менялись. Initial import/export/update/recovery captures,
exact raw export и SDK internal I/O остаются следующими интеграциями. Source/
gofmt12 only; existing-lock/API/native/durability/OS/all9 unverified.

V2-180: same native reader scope подключён к initial legacy/enrolled export,
read-only acceptance и post-prefix source. Final vault callback явно использует
already-held writer, без recursive WithRead. Full decoded policy/cache и exact raw
envelope source fence, renderer-owned secret selection и rederived legacy retained-
ban portability; committed prefix reporting сохраняется при late reload failure.
Shared SDK/API не менялись. Final gofmt7/source only; initial import/update/
pin-policy/recovery captures, internal SDK I/O/native/durability/all9 gates открыты.

V2-180a ReadEnvelopeBytes — Windows metadata exact raw API на той же paired
stored-envelope I/O gate, что LoadEnvelope; native source scope задаёт caller.
Никакого SDK boundary change, compile или compatibility proof.

V2-181: Windows TrustSnapshot captures policy/checkpoint only under existing-only
native shared scope; updater can verify permitted code with absent/corrupt/pending
data-plane envelope. SDK runs after source locks release, prefix keeps existing
writer; final preparation and install/current-code/forward-recovery source gates
hold native scope through separate installation callback. No coordinated profile
writes/SDK/vault/App calls inside it. Schema/presence/full encoded policy/checkpoint,
fresh clock/proof/roles/version gates retained. No shared SDK/API change; source/
final gofmt5 only, native/outcome/durability/OS/all9 gates open. Export no-op is
no longer reported as a newly executed prefix commit. Schema2 unsupported.

V2-181a CheckUpdate repeats the native source/role/floor/time guard after payload
hashing before OK; clears stale proof facts on error. No generated binding change.

V2-182: profile import uses coherent trust-only expected source and native scoped
authority/history preparation. Ordinary import binds exact old envelope through
final writer fence; explicit recovery does not require that file. SDK/credential
work releases profile scopes; final preview uses new Windows-only vault
WithPreparedReadSnapshot with explicit same-journal binding, no initialization or
history save. Outer read fence is distinct from already-held Store.BeforeCommit.
No shared SDK/trust API changes, source/gofmt4 only. Urgent pin-policy captures,
native/API/recovery replay/concurrency/durability/OS/resources/all9 remain open.

V2-183 urgent pin receiver reuses coherent frozen import source, no old-envelope
or vault requirement for public preview. SDK outside profile scopes; native read
acceptance/preview and final existing-writer fence preserve cumulative denials.
Shared prefix helper compares source under reader even on no-op; one outcome
wrapper tracks cache-only successful commit callback and late errors for pin/export.
No shared SDK/trust API changes. Source/gofmt4+comment1 only; public file intake,
internal SDK I/O/native/API/durability/OS/resources/all9 gates remain open.

V2-184 selected capsule/transport files reuse bounded original reader with an
explicit untrusted-input mode: native data/attributes/sync access, read-only share,
same-handle path/ID/size/mtime and checked Close before bytes. Two fixed domain
slots retain uncertain owners, no replacement/fallback. Default store readers
and exported inspector retain trusted ACL+single-link policy. Public input adds
no trust/SDK authority; common authentication/caps unchanged, context gates and
dialog error propagation added. No shared SDK/trust API changes, source/gofmt6
+subset1/hashes only; tests/build/SDK/dialog/native not run. Next selected encrypted
output receipt, directory durability/restart, internal SDK I/O and diagnostics UX;
provider/API/compatibility/OS/resources/all9 open, schema2 unsupported.

V2-185 selected encrypted output reuses original bounded temp writer with explicit
context, existing-only parents, full writes/Sync/flush/original-handle publication/
checked Close and retained uncertain receipt. Published/MayHavePublished facts
survive late flush/Close/cancel errors; App reports them without new Wails binding.
Typed stored/input/output inspection keeps strict metadata/vault/journal policy;
selected output skips trusted-store ACL but enforces created temp single-link.
No shared trust/SDK API change. Source/gofmt5+3+1/exact hashes only, no tests/build/
SDK/dialog/native. Target-CAS/metadata preservation/native/provider/durability/
OS/resources/all9 gates unverified; SDK internal I/O receipt seam and diagnostics
UX remain next, public schema2 unsupported.

V2-186 Windows Store.QuarantineError exposes existing process-local original I/O
fault separately from CleanupError warning. Actual commit callback outcome survives
late vault/quarantine errors in profile/pin/all4 prefix consumers; after-vault gate
suppresses fresh activation, existing lifecycle retires old owner outside profile
scopes. Benign debris never reverses logical commit. Same GetDeliveryProfile adds
storageFactsSchema1/process-only states/warning with no disk/cleanup/Ready authority;
existing Settings/mapper/HintBox and apply events display it, old backend unknown.
Update preview consumed/unmount-generation fenced; policy error avoids automatic
retry advice. No shared SDK/trust API change, no new Wails method/generated change.
Source/gofmt9+2+1+9/exact14 hashes only; no tests/build/typecheck/SDK/browser/native.
SDK internal I/O seam/directory-powerloss/restart/API/ABI/Stop/OS/resources/all9
remain open, schema2 unsupported.

V2-187 local go-tuf v2.4.2 updater adaptation requires explicit Storage and changes
only local filesystem boundaries; full protocol bodies and official pinned
metadata/trustedmetadata/config/crypto remain. Product cache is bounded owned bytes
with virtual namespace/genuine miss/sticky error/disabled target cache, not durable
trust state. Same workspace owner/legacy quarantine barriers remain, no new active
native SDK workspace; gates after New/Refresh/GetTargetInfo/pre-proof and final
post-retirement context retain restriction-only prefix on late failure. Origin/
license/exact diff/ADR recorded, go.mod/go.sum unchanged. This is Windows-private,
no shared trust API rewrite, no second verifier or disk fallback. Gofmt5+3+1+1/
exact source hashes only; no tests/build/SDK/module install/native. Whole-SDK I/O/
parity/API/CPU/RSS/deadline/directory durability/restart/OS/resources/all9 remain
unverified, schema2 unsupported. See TH-01-SDK-STORAGE.

V2-188 existing SDK acceptance/recovery-check worker now requests initial512MiB
committed Job memory/20 user-CPU-seconds and exact one-process/terminate policy.
Before Resume Set-Query/accounting, child immediate-Job gates and parent post-exit
resource checks reuse original retained process/Job/image/reaper. Two fixed readonly
arguments admitted; mutating repair/GUI privileges and engine/GUI policies unchanged.
No shared SDK/trust API change or second verifier. Full profile/pin/export SDK
private-proof process bridge still open; caps unmeasured/not RSS-rate-hard deadline.
Gofmt8+1/exact8 hashes/source only; no tests/build/SDK/native. Native ABI/nesting/
admission/old caller/parity/durability/restart/OS/resources/all9 remain unverified,
schema2 unsupported. See [SDK resources](TH-01-SDK-RESOURCES.md).

V2-189 [product SDK worker](TH-01-SDK-WORKER.md): profile/pin/update callers route
through the fixed compiled core adapter and original bounded Job. Canonical
owned20MiB input/6MiB output/fresh nonce-HMAC/request SHA/source/full batch, exact
EOF/zero exit/resource-image/full retirement precede private proof reconstruction.
Logical SDK failure or late cancel preserves restriction-only prefix; uncertain
transport returns no proof and retains original custody. The adapter is trusted
proof-issuing code; HMAC is not native/code attestation or protection from its
replacement. No runtime adapter registration or data-to-Prepared parser added.
Pending EnrollmentSource moves initial root self-signature into mandatory existing
updater.New; CLI Enroll remains signature-checked. Same SDK and source/fixed4/
clock/history gates, no shared API/dependency/generated change. Gofmt14+4+3/source
only, no tests/build/SDK/crypto/native. Native/IPC/TCB/parity/per-role prefix/total
resources/durability/restart/OS/all9 remain open; public schema2 unsupported.
