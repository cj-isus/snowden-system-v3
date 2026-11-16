# STATE — public Windows source snapshot

Current implementation: V465, source only. Full PC-00..09 and prior requirements remain open; no tests, build, native, VPN or release acceptance was performed for this publication. The private local state and historical operations records are preserved outside this update.

## V2-465 — владение TLS events и лимит ожидающих копий

PC-06 source/unverified: [контракт](tasks/PC-06-HY2-RECEIVE.md#pc-v465-crypto-events).
SDK гарантирует QUICEvent.Data лишь до следующего NextEvent; исходные writeRecord
и GetSessionTicket удерживали borrowed slices дольше. Теперь write/ticket bytes
копируются до продвижения SDK; queue резервирует128 slots/8MiB queued clone bytes
до allocation. Погашенные event cells очищаются перед reslice, пустая head отпускается.
Additive EventError/Err с прежней NextEvent signature достигает actual Conn error
path. Go1.26 QUICErrorEvent/Err обрабатывается явно; fork minimum1.26 соответствует
существующему Windows parent, dependency versions не менялись.
RetireEventQueue использует original Close cell/result после native/data/history/
TLS/local sorter receipts. Ordinary capacity может retire-иться после quiescence;
inconsistent credit/unknown original retained/negative без retry. Нет payload/key
wipe или provider callbacks. Existing Transport/HY2 source unchanged.
q67/s75;6Go1manifest2origin4docs/manual8diff1new/gofmt6exit0;13reviewed3SDK/
261cache201verbatim. No tests/build/native/network/Git. TLS keys/STREAM-crypto/
retransmission lifetime budgets/provider/native/OS/UX/release/fullPC OPEN;
100OPEN56NOT_RUN/TUN unavailable/schema2 unsupported.


## Next

Inspect TLS traffic-secret/AEAD ownership and diagnostic consumers before any key-reference retirement; crypto-stream/retransmission lifetime budgets remain open. TUN is unavailable and schema2 activation is unsupported. Follow the role-owned PC plan in `docs/planning/knowledge-application/PC-PLAN.md`.
