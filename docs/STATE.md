# STATE — public Windows source snapshot

Current implementation: V485, source only. All PC-00..09 and prior requirements remain open. No tests, builds, native, VPN or release acceptance were performed. Private operational records and the full local state are excluded.

## V2-485 — UDP control lane, final timer clear и LocalAddr snapshot

PC-06 source/unverified: [изменения и границы](tasks/PC-06-HY2-RECEIVE.md#pc-v485-hy2-controls).
Independent8/carrier2/session fixed controls хранят owner/deadline/stage вне
read lane. Setters и privileged close clear сериализованы; seal recheck после
mutex исключает late reset. Unknown poison до unlock, callback после; original
control slot/root остаётся, positive RX budget требует empty controls/slots.
Close использует original pipe.Deadline.Set(zero) вне budget/RX mutex; pinned
stop/close-action receipt не объявлен generic timer-thread/OS retirement.
LocalAddr возвращает constructor-time packetaddr snapshot; raw source удержан
до Capture return, последующих native callbacks нет. q87 pair-only/s95;
5Go2origin4docs/manual7diff1new/gofmt5exit0/16reviewed1cachedSDK.
No tests/build/native/network/Git. Далее приоритет PC-01 W4c/helper/WFP source;
PC06 rejected/preflight/provider/native/global/OS/private/fullPC/old9 OPEN.
100OPEN56 product NOT_RUN+2 Git-only passed/73tasks/TUN unavailable/schema2
unsupported/rootNETGUARD001 OPEN.


## Next

Continue PC-01 original-helper/WFP/W4c source review. TUN remains unavailable; schema2 activation remains unsupported. PC-06 resource, provider, OS and comparative acceptance remain open. See `docs/planning/knowledge-application/PC-PLAN.md`.
