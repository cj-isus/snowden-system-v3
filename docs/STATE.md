# STATE — public Windows source snapshot

Completed source documentation: V533. This snapshot also includes reviewed in-progress V534 watchdog/context changes; V534 documentation and acceptance are not finalized. All PC-00..09 and prior requirements remain open. Tests, builds, native, VPN and release acceptance were not performed. Private operational records and full local state are excluded.

## V2-533 — finite startup rejection без private cause callbacks

[Контракт](tasks/TH-01-FORWARDING-ADMISSION.md#pc-v533-startup-rejection): Start больше не fmt-wraps startupErr под App.mu. Public error содержит static text и только static cleanup category; original cause остаётся private/sticky. State/error используют общий текст, profile/closing errors фиксированы, startup UI hint соответствует требованию перезапуска после проверки остановки. Далее — actual watchdog ctx alias/WithCancel под runtime+App locks до owner reservation.

Source/unverified: 2 Go + 1 static TS label + 4 docs; вручную просмотрены 3 diff, gofmt2 exit0. Тесты/сборки/typecheck/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.


## In progress: V534

Reserve the original watchdog owner before constructing its actual context outside value locks. Preserve partial cancellation and original ordinary-return witnesses. Apply a cancellation requested before the partial cancel was published, and reject canceled preparation before later parent getters. Source review only; no runtime or compiler acceptance.

## Next

Finalize V534 source documentation and ownership evidence, then inspect original network/DNS producer context construction and reservation. TUN remains unavailable; schema2 activation remains unsupported. Compiler, fixtures, native, OS, provider and comparative acceptance remain open. See docs/planning/knowledge-application/PC-PLAN.md.
