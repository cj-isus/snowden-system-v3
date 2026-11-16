# Pinned go-tuf updater: explicit local storage

Source implementation only, **unverified**, Snowden V2-187 (2026-10-28),
success-return observation V2-191 (2026-10-30).

- Upstream: `github.com/theupdateframework/go-tuf/v2`, **v2.4.2**.
- Commit: `f5edbde31e5507f46db2069402dc38903fe6d9d4`.
- [Original updater.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go).
- Original UTF-8 SHA256: `61653cc910879edd28c6cdd7e28bcc7bacacd28c0a022e0f3ff62b9f16c10dc2`.
- Public Go proxy version-info Origin and GitHub tag ref agreed on that commit;
  tag and commit-specific updater bytes had the same SHA256. This does not verify
  the complete module zip against go.sum or replace later dependency acceptance.
- `LICENSE` and `NOTICE` are exact upstream commit files, Apache-2.0. Original
  copyright/header retained; the modified updater carries a prominent change note.

## Deliberate source delta

`updater.go` is the complete original file with a required `Storage` argument and
local I/O boundaries changed: path setup, metadata read/write/readback and
target-cache read/write. There is no default disk implementation or fallback.
`storage.go` contains that interface. Raw metadata is never re-encoded by Storage.

V2-191 also observes original per-role SDK calls through `progress.go`: successful
result/error returns update a distinct SuccessfulMetadataSet before later storage
writes. Intermediate timestamp/snapshot pointer assignments on a failing return
do not become success receipts. The wrappers return original values/errors and
keep original call arguments/order/branches, including isTrusted snapshot checks.
The adapter applies original-byte/continuity/effective-time/link eligibility to
this set for restriction progress only; complete artifact proof still uses the
original full SDK set and its final snapshot checks. No callback, independent
verifier, crypto fork, retry or reference-clock override was added. This is now
storage plus observation delta, not a claim of unchanged storage-only source.

Root/timestamp/snapshot/targets sequencing, delegated walk, threshold/signature/
version/expiry decisions and hash checks remain upstream source. Types, parsers,
trusted metadata and crypto still import the **original pinned module**; no second
trust implementation or simplified verifier is introduced. Module version, sums,
UnsafeLocalMode, DisableLocalCache and the clock contract are unchanged in the
product adapter. The complete upstream public API remains, including unsafe
reference-time setter; production does not call it. Target storage is deliberately
unsupported by the product's metadata-only implementation; artifact acceptance
keeps the existing bounded complete-batch length/hash/role checks.

Only `delivery/client.go` imports this internal package. Its owned in-memory
storage checks sticky faults after New, Refresh, target lookup and before complete
proof publication. This matters because the upstream workflow can swallow local
read errors as a cache miss. A genuine permitted-name cache miss stays distinct.

## Review and update boundary

The pinned source and exact local diff are recorded in
`.artifacts/memory/sdk-io-upstream/` at the repository root; current hashes and
source-only evidence are in `.artifacts/memory/sdk-io-source-evidence.json`.
V2-191 delta/identities are separately captured in
`.artifacts/memory/role-prefix-source-evidence.json`; the original V2-187 source
receipt remains historical, not a claim that the new updater hash is unchanged.
Review the diff when changing this local copy; updates must repeat provenance,
license, policy-delta review and permitted parity acceptance. Do not edit the TUF
workflow here to evade a shared trust contract, and do not add a disk fallback.

No SDK execution, compile/build, tests, module installation or native/runtime
acceptance was performed. Full behavioral parity, malformed-cache/missing/rollback/
intermediate-rotation/online-key-reset/delegation/batch/error/cancel compatibility
and process memory/CPU/deadline/OS/release acceptance remain open. Eliminating the
explicit local metadata-cache file calls is not whole-SDK I/O attestation,
an aggregate RSS bound, a performance result or installation/VPN readiness.
