# PC-01 — аудит требований и оставшейся реализации

<a id="pc-v359-pc01-audit"></a>

## V2-359 — источник, приёмка и конкретный разрыв

2026-11-10, Windows owner, **source/unverified**. Полная цель —
[PC-00…09](../planning/knowledge-application/PC-PLAN.md), прежние FR/TH/OFF/COM/RQ/TR
и TH-00…08 сохраняются. V358 — progress; предыдущий короткий ответ о проценте —
no-progress/status only. В этой сдаче меняются source/docs/checkpoint, не режим
выполнения.100 acceptance OPEN/56 checks NOT_RUN, waiver нет.

### Матрица PC-01

| Требование | Текущий source | Что остаётся |
|---|---|---|
| W4c перед общим forwarding | [native_tun_commit.go](../../windows/backend/core/native_tun_commit.go), original helper/capture/WFP lease и scoped ingress из V357 | Compiled nativeTUNForwardingAccepted=false. Source callback/receipt не подтверждает native поток. Нужна отдельная Windows/WFP/process приёмка; TUN не включён. |
| Current owner/session/intent при допуске | Existing Manager/lifecycle/guard и собственный commit owner | Actual IPC/native identity, late/duplicate/native failure и физическое закрытие ресурсов не доказаны. |
| Start/Stop/retry/Switch/Reload/Close | Существующие App/Manager registrations; V348 ordered UI source, V352…354 original profile/transfer lifetime | Native/SDK cancellation и полное возвращение process/Job/helper/socket/interface остаются OPEN. Timeout не освобождает владельца. |
| Ошибка и неизвестная очистка | V357 original lease/ingress publication; V358 normal-return cleanup и faulted TCP terminal | Источник сохраняет отрицательный исход и custody; физическое Close/native retirement не проверено. |
| Manual Disconnect отдельно от error | [protection_release.go](../../windows/protection_release.go), existing explicit release commit | Native barrier/actual OS effects и целая последовательность Disconnect требуют приёмки. Автоматическая ошибка не даёт права снять strict intent. |
| Proxy только своих writes в живой сессии (W3a) | Existing ProxyManager original key/write/readback/conflict phases; [TH-08](TH-08-PROXY-OWNERSHIP.md) | Process-local source, не persistent crash recovery. Actual registry/WinINET/interprocess outcome не проверен. |
| Durable crash/startup/manual recovery (W3b) | [proxyheal_windows.go](../../windows/backend/core/proxyheal_windows.go) явно возвращает ErrProxyReceiptUnavailable | **Общего authenticated original-write receipt/native interprocess owner пока нет. Это отсутствующая реализация, не только проверочный gate.** |
| Совместимость внешнего UserMode/SYSTEM writer | [NETGUARD-001](CHANGE-REQUEST-NETGUARD-001.md) описывает scope и legacy/mixed migration | Root tools/netguard review-required, owner не расширен. Scripts/установленные copies/tasks не менялись. Receipt/migration нельзя объявлять готовыми по наличию запроса. |
| ProtectionController/служба | FR-009…011 и OFF-06 остаются planned | Не описывать process isolation/фасад как уже реализованную службу. Вопрос остаётся в полном плане. |

Для каждой строки source и runtime/OS доказательства различаются. Количество
открытых gate не измеряет объём разработки; прежние около55% — грубая старая
оценка, не свежий расчёт и не готовность выпуска. Этот аудит PC-01 не закрывает.

### Найденный независимый дефект PC-06/04

App.NetGuardStatus раньше выполнял os.ReadFile(netguard.log), затем декодировал и
разбивал весь растущий файл. Только после этого выбирались12 событий. Размер
журнала не ограничивал память/число промежуточных строк. Обычная ошибка чтения
также превращала независимо полученные task/proxy observations в общий RPC error.

Вместо второго filetxn/domain переиспользован существующий
[originalLogFile](../../windows/log_file_owner.go). Новый
[netguard_log.go](../../windows/netguard_log.go) нужен для отдельного bounded tail
с legacy UTF-8/UTF-16LE/cp1251 decoder; incremental engine tailer имеет другой
контракт. SelectedInputSlot для активного externally-written лога не использован.

1. Один process-local TryLock lane. Original operation публикуется до Open;
   normal return требует завершённый I/O и original Close. Неизвестный
   Open/I-O/Close сохраняет тот же owner без replacement/retry; новые observers
   получают busy/unavailable. Новых goroutine/timer/repair нет.
2. До аллокации tail проверяются original file Stat/regular/nonnegative size.
   Запросы ReadAt суммарно не более65792 байт:64KiB tail и256-byte encoding sample.
   Short read/усечение не превращаются в успешную пустую историю. Appends
   разрешены, но coherent whole-file snapshot не заявлен.
3. UTF-16 alignment использует существующую encoding heuristic. Отбрасываются
   начальная запись на краю tail и незавершённый suffix. До decoder/split
   сохраняется не более256 полных строк; decoded строки длиннее4096 UTF-8 bytes
   пропускаются целиком. Сохраняется не более12 significant events. Truncated и
   incomplete объясняют неполноту, а не исправность сети/журнала.
4. Schema1 read-only log observation отличает observed/missing/busy/unavailable.
   Counts применимы только к observed. Missing/error не отменяют отдельные
   scheduler/proxy observations. Empty/nonabsolute ProgramData не даёт fallback
   к относительному working-directory файлу.
5. Optional строгий UI mapper сохраняет старый backend как unknown; malformed
   scope не выдаётся за bounded evidence. Тип неизвестного события больше не
   становится выдуманным PROBLEM. Existing details/summary показывает пределы и
   отсутствие наблюдения без новой карточки и без нулевого счётчика истории при
   unknown. Vue text interpolation/существующие wrapping styles сохранены.

### Границы результата

Это source resource policy для одного log reader. Она не ограничивает OS I/O
latency, RSS/GC, весь COM/Task Scheduler или остальной NetGuardStatus. Нет
native ACL/reparse/alias/network-path/rotation/ABA/atomic snapshot/interprocess
доказательства. UTF-16/cp1251 — прежняя совместимая эвристика, не проверка формата.
Busy/unavailable не подтверждают отсутствие файла/событий/сбоев. Любой journal
marker, код задачи, порт или DTO не даёт права сбрасывать proxy/DNS/WFP/TUN.

Root scripts/tasks, generated wailsjs, Android/shared/configs/harness/другие
чаты, реальные ProgramData/log/env/vault/серверы не трогались и не читались.
Контракт optional не заменяет будущую генерацию/приёмку bindings.

### Что было выполнено

2Go2TS1Vue4docs,9 original snapshots(7existing/2new). Просмотр4 оригинальных
source diff и нового файла; touched-only gofmt двух Go-файлов завершился exit0.
16 named reviewed inputs сохранены по SHA256. Только own source-byte/document/
checkpoint metadata. Tests/fixtures/build/vet/types/parser/SDK calls/generation/
browser/native/admin/network/real log execution не выполнялись. Compile/runtime/
encoding/COM/native/physical I-O/UX correctness не подтверждены.

### Следующий проверяемый source-срез

В существующем comGetTask найден отдельный разрыв: CoInitializeEx result
игнорируется, OS thread не закреплён, CoUninitialize выполняется без известного
успешного init. Сначала прочитать pinned go-ole source/HRESULT/VARIANT contracts,
затем отдельным checkpoint спроектировать original apartment/collector ownership
и отрицательные исходы. Не добавлять parallel collectors/timeout replacement.
W3b остаётся обязательным owner-dependent остатком, без самовольного расширения
root ownership. Полный PC-00…09 и прежние девять этапов остаются active.

<a id="pc-v367-proxy-writer"></a>

## V2-367 — уточнение W3b после возврата к PC-01

2026-11-10, source/unverified;
[Windows writer component](TH-08-PROXY-OWNERSHIP.md#pc-v367-proxy-writer).
V359 матрица выше остаётся датированным аудитом. Теперь Windows ProxyManager
имеет source foundation native writer lease в одном private LocalAppData domain:
reuse existing filetxn LockFileEx/ACL/FileID, acquire до HKCU, release после
HKEY Close, original lease в opaque known-foreign preservation receipt.
Known local busy/state uncertainty прекращают candidate channel loop; обычный
bounded retry не заменён permanent busy latch.7Go5docs/manual5diff+2new/gofmt7+1+3,
без tests/build/native/registry/network. Это не Windows/native acceptance.

**Общий W3b всё ещё не реализован:** authenticated durable original-write
receipt/DPAPI, state-before-write/readback commits, process-death proof, startup/
manual recovery и external UserMode/SYSTEM interoperability/migration отсутствуют.
Same-storage-domain lock не координирует legacy/MDM/другой root автоматически.
HealStale unavailable; root tools scope не расширен. W4c gatefalse/TUN unavailable/
schema2 unsupported/fullPC00…09/old9/100OPEN56NOT_RUN/native/SDK/OS/release OPEN.
Далее — private bounded authenticated receipt под retained Windows lease.

<a id="pc-v368-proxy-journal"></a>

## V2-368 — W3b original-write journal реализован частично

2026-11-10, source/unverified;
[private journal](TH-08-PROXY-OWNERSHIP.md#pc-v368-proxy-journal).
V359/V367 выше — датированные источники. Теперь actual production source содержит
bounded exact original/current/target binary receipt, primary SID/PID/creation/
nonce/generation и current-user DPAPI с original native allocation/checked free.
Trusted encrypted file работает под V367 retained lease. Initial capture,
prewrite intent, fresh tuple fence после file I/O, exact readback confirmation
и final closed/foreign-preserved record подключены к реальным proxy methods.
Unknown native/file/crypto сохраняет owner; corrupt/unresolved old receipt
не сбрасывается и блокирует fresh apply. Finite lifecycle text без raw tuple.

**Full W3b/recovery ещё не реализованы:** original prior-process death observation,
freshness/antirollback admission, startup/manual repair и legacy UserMode/SYSTEM
interoperability/migration. DPAPI/finished phase/free file lock не заменяют эти
условия. HealStale остаётся unavailable; root NETGUARD-001 не расширен. Следующий
срез — original prior-process observation и scoped unresolved-receipt admission.
8Go5docs/13snap/manual5diff+3new/15named3cached/gofmt6+2+2+4+1exit0; только source.
Без tests/build/native/registry/network; all100OPEN56NOT_RUN/fullPC00…09/old9/
gatesfalse/TUN unavailable/schema2 unsupported/native/SDK/OS/W4c/release OPEN.

<a id="pc-v369-proxy-process"></a>

## V2-369 — W3b prior-process observation подключён к journal intake

2026-11-10, source/unverified;
[original observer](TH-08-PROXY-OWNERSHIP.md#pc-v369-proxy-process).
Теперь unresolved DPAPI journal сохраняет original limited-rights process/token
owner до acquisition. Exact PID/creation/primary SID и zero-time same-handle wait,
повторный creation/exit observation для signaled branch; known original Close
через reused CloseOnce. Missing/access error/reused PID не получают death proof.
Live/reused/exited-recovery/unavailable дают разные finite refusals; unknown
native/Close удерживает original process/store/lease. Old file не сбрасывается,
previous record не становится fresh active capture или restoration permission.

**Actual remaining W3b code:** authenticated unchanged journal-subject admission,
freshness/local rollback model, scoped per-value partial recovery/finalization,
startup/manual repair и legacy UserMode/SYSTEM migration. Completely gone или
unopenable process остаётся unresolved. Эта observation не является native
приёмкой и не доказывает cleanup других ресурсов. HealStale unavailable;
root NETGUARD-001/other roles untouched.4Go5docs/9snap/manual3diff+1new/
16named3cached/gofmt4+2exit0; без tests/build/native/registry/network. FullPC00…09/
old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/OS/release OPEN.

<a id="pc-v370-proxy-subject"></a>

## V2-370 — immutable local subject до scoped recovery admission

2026-11-10, source/unverified;
[subject/recheck](TH-08-PROXY-OWNERSHIP.md#pc-v370-proxy-subject).
Existing bounded reader расширен ReadSnapshot: same-read original FileInfo и
bytes только после original checked Close, old Read signature сохранена.
Actual unresolved authenticated journal связывает original store/lease/slots/
observer, fixed path, FileID, exact cipher/canonical plain digests/nonce-generation.
После known original process retirement выполняется source recheck; failure
sticky удерживает original logical owner/lease, не tombstone/reset/adoption.
Store использует actual lease validation перед read/encrypted writes. Новой
Win32 read/identity implementation нет; metadata — value, не native custody.

**Actual remaining W3b code:** explicit recovery admission/freshness/local
rollback policy, initial absent/finished/own-write full persistent protocol,
per-value partial recovery/finalization и legacy UserMode/SYSTEM migration.
Local FileID/hash consistency не durable antirollback/global latest/OS CAS или
registry restore authority. HealStale unavailable; root NETGUARD-001/other roles
untouched.6Go5docs/11snap/manual5diff+1new/15named0cached/gofmt6+2+2exit0;
без tests/build/native/registry/network. Generic reader consumers/native/OS/FS/
ACL/power-loss compatibility unverified. FullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported/W3b/W4c/release OPEN.

<a id="pc-v371-proxy-anchor"></a>

## V2-371 — независимая anchor для file-only rollback consistency

2026-11-10, source/unverified;
[контракт](TH-08-PROXY-OWNERSHIP.md#pc-v371-proxy-anchor).
Actual journal writer теперь связан с bounded purpose-separated DPAPI HKCU
SID/path/sequence/committed+pending anchor. Original fixed HKEY/query/Set/readback/
RegFlushKey/Close; prepare+flush → encrypted file+known bytes/identity capture →
commit+flush, затем local state advance. Prior file checked twice before overwrite.
Subject binds actual anchor pointer/sequence/cipher; original anchor closes
before system HKEY/lease. Transient completion отделён от held HKEY ownership.
Only both absent enroll; legacy unanchored/pending/missing/mismatch refused,
без automatic reset/migration. No plaintext PAC/config в anchor или presentation.

Это consistency against independently retained hive state, **не correlated hive/
profile rollback/deletion protection, global freshness/TPM/OS CAS/restore grant**.
Registry ACL/link/identity/effective HKCU, native flush/FS/power-loss/latency и
crypto/Close acceptance OPEN. Actual code gaps: pending-pair reconciliation,
interrupted fresh setup/legacy migration, per-value crash/startup/manual recovery
и UserMode/SYSTEM/installed tasks protocol. Existing V368…370 unanchored receipt
не поддержан до explicit migration. HealStale unavailable; root NETGUARD-001/
other roles untouched.9Go5docs/14snap/manual6diff+3new/14named5cached/
gofmt9+2+4exit0; no tests/build/native/registry/network. FullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/W3b/W4c/release OPEN.

<a id="pc-v372-proxy-reconcile"></a>

## V2-372 — exact target metadata reconciliation

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v372-proxy-reconcile).
Actual pending intake теперь завершает прежний anchor commit только для exact
authenticated already-present target: original file identity/hash/record, counter,
anchor/lease pre-post checks и existing readback/flush. Нет file rewrite/Internet
Settings доступа/restore authority. Original reconciliation return учитывается
Store lifetime; old committed file/initial missing/neither — отдельные refusals,
без cancellation/reset/re-enrollment или потери pending evidence.
Остаток W3b: old/missing pending/empty-key/legacy migration, per-value crash/
startup/manual recovery и UserMode/SYSTEM interoperability. HealStale unavailable.
5Go5docs/10snap/manual4diff+1new/16named0cached/gofmt5+1+1exit0; no tests/build/native/
registry/network. Correlated hive/profile rollback/native/OS/durability/latency
unverified/fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2
unsupported/release OPEN.

<a id="pc-v373-proxy-stage"></a>

## V2-373 — parent-bound encrypted stage и journal replay

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v373-proxy-stage).
Actual writer сохраняет distinct-purpose encrypted SID/path/parent-counter/
committed/file-identity/exact-target intent до anchor prepare. Existing original
slots/crypto/lease reused; borrowed ciphertext inputs retained after unknown write.
Guarded intake supports pending old/missing, pre-anchor orphan, committed backup
и both-absent initial journal replay. Missing uses WriteNew; exact target reused,
unknown/corrupt data not overwritten, no staging deletion/Internet Settings writes.
Остаток: no-write retirement/per-value system recovery, empty-key/legacy/mixed
writers/UserMode-SYSTEM migration и full native/OS/latency/durability acceptance.
Correlated hive/profile/stage rollback не решён; HealStale unavailable.
9Go5docs/14snap/manual6diff+3new/16named0cached/gofmt9+1+1+1+1+2exit0;
no tests/build/native/registry/network/fullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v374-proxy-no-write"></a>

## V2-374 — private no-write journal retirement

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v374-proxy-no-write).
Only authenticated anchored active/written0/index255/exact original=current=target
may close its journal with same old session/generation+1/counter+2. Original owner,
source/FileID/bytes, stage, anchor and exclusive lease checked before/after shared
publication; fresh nonce/PID stay independent. Exact retirement caller admission
does not weaken the ordinary writer. Unknown operation/source retains owner/input.
Prepared/written records remain unresolved. No Internet Settings access, prior-
process/death inference or restore receipt; historical attempted setters not denied.
Остаток: per-value crash/startup/manual repair, empty-key/legacy/mixed writers/
UserMode-SYSTEM migration, correlated rollback and full native/OS/durability/
latency/resource acceptance. HealStale unavailable.
4Go5docs/9snap/manual2diff+2new/16named0cached/gofmt4+3+1exit0;
no tests/build/native/registry/network/fullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v375-proxy-recovery"></a>

## V2-375 — original-exit admitted per-value recovery

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v375-proxy-recovery).
Actual original PID/creation/SID/signaled exit+checked Close plus authenticated
source/file/stage/anchor/lease admits retained recovery. Existing original key,
setter, readback, notification and journal publisher reused. Prepared source/
target resolution, own fields only, intent-before-setter/full tuple fences,
foreign preservation and final metadata/checked HKEY Close before successor.
Responsible recovery writer PID recorded with old nonce/original tuple; fresh
session independent. Foreign cleanup carries actual original Close witness.
Остаток: missing/unopenable prior-process manual takeover, retained live recovery
resume, ordinary failure handling, empty-key/legacy/mixed writers/UserMode-SYSTEM/
NETGUARD-001 migration, correlated rollback and native/OS/durability/latency/resource
acceptance. No PID absence/free lock/port-based death inference; HealStale unavailable.
9Go5docs/14snap/manual6diff+3new/18named0cached/gofmt9+3+1exit0;
no tests/build/native/registry/network/fullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v376-proxy-consent"></a>

## V2-376 — core source-bound manual takeover

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v376-proxy-consent).
Prepare/Confirm/Cancel core API: exact original source/file/stage/anchor/lease/
process and one-shot32B token/fixed scope/monotonic2min start TTL. Known-retired
unobserved/reused process may receive explicit takeover, no death inference;
observed running/unknown native operation stays refused. Existing per-value owner,
publisher and pending/context guards reused. Typed outcome/count only after known
component closure. Cancel/ordinary Stop retires unused own preparation; no new
registry reset, auto-connect or Engine/WFP/protection release permission.
Core-only: App/Manager/Wails/UI wiring and retained failed live-owner resume are
still missing. Empty-key/legacy/mixed writers/NETGUARD-001/native/OS/durability/
latency/resource/release remain; HealStale unavailable. Consent token is not
independent human/process identity, an OS CAS or a hard call-abort deadline.
7Go5docs/12snap/manual5diff+2new/18named0cached/gofmt7+4+3exit0;
no tests/build/native/registry/network/fullPC00…09/old9/100OPEN56NOT_RUN/
gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v377-proxy-ui"></a>

## V2-377 — Manager/App/NetGuard manual workflow

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v377-proxy-ui).
Actual lifecycle/proxy ownership before callbacks, stopped-proxy Stop/composition
guards, profileless actual runtime Manager/no Engine; App operationMu/intent/
request/manager/engine/local context and entered-frame actual-return fencing.
Early cancel and original scope retirement, strict generated-only facade,
NetGuard/DialogFrame/global UI pending owner/expiry/ack/stale/close/navigation.
Historical foreign outcome separated from conflict newly found by replacement
Stop. Proxy outcome clears no Engine/WFP/protection/health/whole-cleanup gate.
Current generated bindings/EXE were not regenerated: feature stays unavailable
there until authorized normal generation. No raw bridge/generated-file edits.
Retained failed-owner resume, empty-key/legacy/mixed writers/NETGUARD-001 and
native/OS/UI/durability/latency/resource/release acceptance remain; HealStale unavailable.
6Go5UI5docs/16snap/manual7diff+4new/19named0cached/gofmt5+2+1+3+1exit0;
no tests/build/types/native/registry/browser/network/generation/fullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v378-proxy-resume"></a>

## V2-378 — retained recovery continuation core

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v378-proxy-resume).
Fresh source/anchor/stage/original-HKEY grant continues the same retained owner
only after known unchanged setter/notification failure or cancellation before
native intent. Unknown outcomes stay held. Previous callbacks/grant are bounded;
cumulative steps and generation/anchor counters are not reset, eight transitions
per grant. Unused grant cancellation releases no original transaction resource.
8Go5docs/13snap/manual6diff+2new/15named0cached/gofmt8exit0 reconfirmed.
Manager/App/UI resume wiring and current generated/EXE delivery remain open.
No tests/build/types/native/registry/browser/network/generation; native/OS/migration/
NETGUARD-001/wholePC00…09/old9 remain;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/release OPEN.

<a id="pc-v379-proxy-continuation"></a>

## V2-379 — actual continuation workflow

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v379-proxy-continuation).
Same Manager/native session with private known-pause preparation and fresh grant.
App requires exact previous request/runtime and actual callback return; duplicate
Cancel cannot retire another frame. Finite released/transaction_held/blocked keeps
unused grant separate from original native retirement. Bounded proposal/previous-ID
negative reconciliation and tombstone preserve the original on refused/unknown RPC.
Strict generated-only facade, reason/ack/expiry UI and persistent Close focus.
3Go4UI5docs/12snap/manual7diff/12named0cached/gofmt3exit0; no tests/build/types/
native/registry/browser/network/generation. Matching Wails regeneration is required;
current generated/EXE unavailable. Migration/NETGUARD-001/native/OS/W4c/fullPC00…09/
old9 remain;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v380-proxy-enrollment"></a>

## V2-380 — initial empty-value metadata enrollment

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v380-proxy-enrollment).
Existing original HKEY twice-missing value is classification only. A private
original replayer requires exact encrypted initial parent-zero/gen1 active0/no-write
stage, canonical absent/exact target, same HKEY/lease/source and repeated absence.
No arbitrary reset, legacy/session adoption or system-settings restoration grant.
Missing-key bootstrap is strengthened too. Encoding/DPAPI precedes CreateKey;
final exact source query precedes SetValue. Existing checked metadata completion
and no-write retirement remain before a separate fresh Snapshot.
4Go5docs/9snap/manual3diff+1new/12named0cached/gofmt4+2exit0; no tests/build/types/
native/registry/browser/network/generation. Correlated rollback/OS CAS/key-path/ACL/
alias/durability/resource/latency are unproven. Legacy/mixed writers/NETGUARD-001/
W4c/native/OS/fullPC00…09/old9 remain;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/release OPEN.

<a id="pc-v381-proxy-writers"></a>

## V2-381 — original running proxy observation

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v381-proxy-writers).
One native child under original lifetime owner validates the original-HKEY tuple
with Manager/watch/gen/Engine/PM binding. Trust checks stay independent of native
stalls; cleanup is attempted before child join and parent done includes actual
child return. Late negative cause retained, foreign preserved, no channel churn.
App local proxy report no longer fabricates trust/certificate failure; original
cleanup/explicit action/protection precedence remains. Serial draft rejected;
amendment and intermediate/final diffs retained.6Go6docs/12snap/manual5diff+1new/
12named0cached/gofmt6+2exit0; no tests/build/native/COM/network/generation.
Root UserMode/SYSTEM and installed tasks/scripts/DNS remain NETGUARD-001 review-
required, unchanged. No OS CAS/ABA/leak/detection deadline/CPU/RSS/native proof.
Legacy/W4c/native/OS/fullPC00…09/old9 remain;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/release OPEN.

<a id="pc-v382-proxy-expiry"></a>

## V2-382 — active lifecycle early fault/old trust during join

2026-11-10, source/unverified; [контракт](TH-08-PROXY-OWNERSHIP.md#pc-v382-proxy-expiry).
Follow-up to V381: early immutable negative fault wakes active Reload/Switch
before whole child return, so existing protection/engine cleanup can be attempted.
Original parent keeps immutable deny-only trust checks during cancelled native
join. Join panic/Goexit still waits original child before parent done closes.
Early signal cannot free/refund/replace or admit forwarding.2Go5docs/7snap/
manual2diff/8named0cached/gofmt2+1exit0; no tests/build/native/network/generation.
Native deadline/latency/cleanup/CPU/RSS/OS behavior unverified. Root NETGUARD-001/
legacy/migration/W4c/fullPC00…09/old9 remain;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/release OPEN.

<a id="pc-v383-wfp-call-return"></a>

## V2-383 — original WFP return graph and failed-grant retention

2026-11-10, source/unverified; [контракт](TH-01-FORWARDING-ADMISSION.md#pc-v383-wfp-call-return).
Existing App/core joint commit is present. Original Begin/Commit/Abort/query/write/
Free/Destroy/Close return records now refuse native successors after an unknown
original call. Deferred reader/enum/transaction cleanup does not skip the negative
Abort graph check on Goexit or issue native Abort/Close through pending I/O.
Failed grant is retained before fallible cleanup and in its deferred finalizer;
unknown original owner cannot be replaced. Fixed-object query GUID is retained.
5Go5docs/10existing/manual5diff/10named0cached/gofmt5+1exit0; source inspection only.
No native-call interruption, cleanup, OS/SDK/ABI or protected-egress proof. Existing
filters/C ABI/admission unchanged; nativeTUNForwardingAccepted=false. Root NETGUARD001/
legacy/migration/native/W4c/fullPC00…09/old9 remain;100OPEN56NOT_RUN/TUN unavailable/
schema2 unsupported/release OPEN. No tests/build/native/network/generation.

<a id="pc-v384-process-cleanup"></a>

## V2-384 — independent original pipe cleanup during process retirement

2026-11-10, source/unverified; [контракт](TH-01-STOP-OWNERSHIP.md#pc-v384-process-cleanup).
Existing Manager post-native-commit refusal already requests protection then
Engine.Stop/own-proxy cleanup. Fixed sequential original-process cleanup gap:
ClosePreparation and parent pipe stages registered before callbacks; input/output
Close registered separately through lifecycleCleanupStep. Goexit keeps negative
outcome and still attempts independent original endpoint cleanup. pipesClosed
requires successful ordinary endpoint returns. Native sticky slots and actual
Wait/I-O/empty Job/image gates retained, with no new permanent failure latch.
1Go5docs/6existing/manual1diff/12named0cached/gofmt1exit0. No tests/build/native/
network/generation. No physical cleanup/deadline/OS proof; fullPC00…09/old9/root
NETGUARD001/native/OS/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.
