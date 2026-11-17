# STATE — public Windows source snapshot

Current implementation: V527, source only. All PC-00..09 and prior requirements remain open. No tests, builds, native, VPN or release acceptance were performed. Private operational records and the full local state are excluded.

## V2-527 — context preparation вне value locks

[Контракт](tasks/TH-01-FORWARDING-ADMISSION.md#pc-v527-context-value-locks): contextForIntent строит real wrappers вне App.mu и проверяет исходный intent/path перед возвратом. Adapter registry удерживает original slot, но не mutex, через context/query/cancel. Только обычный возврат исходной работы разрешает очистку own slot.

Source/unverified: 2 Go + 4 docs; diff просмотрен, gofmt2+1 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.


## Next

Continue PC-01 original operation cancellation ownership and context factories outside App value locks. TUN remains unavailable; schema2 activation remains unsupported. Compiler, fixture, native, OS, provider and comparative acceptance remain open. See `docs/planning/knowledge-application/PC-PLAN.md`.
