# STATE — public Windows source snapshot

Source documentation through V537, including finalized V534 and subsequent V535-V537 network/DNS, background and GUI diagnostic ownership changes. All PC-00..09 and prior requirements remain open. Tests, builds, native, VPN and release acceptance were not performed. Private operational records and full local state are excluded.

## V2-537 — pending GUI diagnostic task/Dial и original budget

[Контракт](tasks/TH-01-FORWARDING-ADMISSION.md#pc-v537-manual-context): task и каждый bounded Dial резервируются до real context/admission callbacks вне mutex. Partial context/cancel/budget остаются у original owner; unknown return не освобождает slot/lease. Final original App gate публикует immutable parent/child/deadline metadata; current/history больше не вызывают Context под value locks. Registered watch/socket stages, actual cancellation join и original source revocation сохранены. Далее — original release custody в core diagnostic pool.

Source/unverified: 8 Go + 4 docs, два новых helper; вручную просмотрены 6 diff и 2 новых файла, gofmt8+3 exit0. GUI-диагностика, тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.


## Next

Inspect core diagnostic budget release custody and every actual admission/release consumer. Preserve the protected lane, optional concurrency and rolling work limits, original context/lease ownership and ordinary return evidence; no refund on cancellation or error. TUN remains unavailable; schema2 activation remains unsupported. Compiler, fixtures, native, OS, provider and comparative acceptance remain open. See docs/planning/knowledge-application/PC-PLAN.md.
