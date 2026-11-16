# TH-03 — сроки доверия и локальные часы

2026-10-21, V2-117: исходники изменены, **unverified**, включая compilation.
Все девять этапов TH-00…08 сохраняются. По прямому указанию владельца tests
не добавлялись/менялись/запускались; build/vet/harness/experiments/native/live
не выполнялись. Gofmt — форматирование, не проверка поведения.

Продолжение local roles/offline profile recovery — [V2-118](TH-03-TRUST.md),
также source-only/unverified. Полный TUF/root/freshness этим не объявлен готовым.

## Исправляемый разрыв

Проверка срока envelope при загрузке и сертификата при handshake не ограничивала
уже работающие соединения. Raw wall time мог отступить; срок Next мог ошибочно
восприниматься как продление существующего соединения с Current. Legacy PEM
передавался путём, позволяя worker читать уже другие байты. Переключение selector
может сохранять старые streams: срок только нового default недостаточен.

## Что добавлено в исходники

1. `metadata/clock.go`: общий App ObservedClock сопоставляет UTC wall с process
   steady elapsed. Наблюдение берётся под его mutex, чтобы параллельные вызовы
   не переставляли samples. Регресс steady, wall ниже retained floor−5min либо
   wall/steady discontinuity >5min возвращают ErrClockUncertain без продвижения
   observer/floor. Внутри допустимого skew effective time назад не движется.
2. В прежний public `version.txt` добавлены optional `time_floor` и
   `issued_at_floor`; null для них отклоняется. Старый numeric checkpoint
   мигрирует через аутентифицированный установленный envelope. Version/hash,
   retained/retired IDs и глобальные revocations сохраняются. Нового пятого
   import target нет. Истёкший исторический подписанный envelope остаётся
   якорем истории при приёме нового действующего профиля, не runtime-разрешением.
3. Envelope expiry строгий: now≥expires_at отклоняется без прежнего grace.
   Envelope сохраняет структурную проверку всех pins; отсутствие активного
   HY2 leaf исключает конкретный канал из runtime eligibility и не объявляет
   повреждённой подпись всего профиля. Ошибки DER/hash/hostname не скрываются.
   Явно выбранный исключённый канал не заменяется молча другим.
4. `render.ClientConfig.Validity` — owned scalar snapshot profile interval,
   local floor и известного certificate deadline. Для signed HY2 это earliest
   NotAfter среди сейчас допустимых active leaves, без revoked/future-only
   продления старой сессии. Новый рендер после Current expiry исключает его
   и может использовать действующий Next. TLS hostname/time verification остаётся.
5. Legacy public PEM: regular/SameFile check, чтение ≤1MiB, ≤64 секций,
   строгие public CERTIFICATE blocks без skipped/private/trailing data.
   Worker получает inline snapshot тех же сертификатов, не повторно путь.
   Runtime validator принимает ≤64 public entries, ≤16KiB каждый и ≤1MiB
   всего; signed Current/Next отдельно по-прежнему ≤2. Expired legacy entries
   исключаются, future-only bundle недоступен, пустой список не становится
   system-root fallback. Legacy trust от этого не становится подписанным.
6. `core/lifetime.go`: Manager owns один observer работающего поколения.
   При local trust failure тот же Manager резервирует lifecycle operation,
   снимает owned TUN permits через существующий cleanup, останавливает worker
   и завершает StateError; старый watcher не останавливает successor. Callback
   App вызывается после finish вне Manager locks и несёт owner/generation/
   session/intent scope старого отчёта только для идентичности, не новый PASS.
7. Во время Start/select/Reload отдельный короткоживущий observer только
   отменяет operation context с trust cause; cleanup исполняет lifecycle owner.
   На входе и после probe проверяется lifetime; expired rollback не принимается.
   Seamless success и verified selector rollback сохраняют минимальные deadlines
   streams предыдущего и кандидата. Только полная замена worker отбрасывает
   прежние ограничения; принудительный обрыв всех streams на каждом switch
   не включён. Это консервативно и иногда требует лишнего recycle.
   Основание: локальный pinned sing-box1.13.19 `protocol/group/selector.go`
   вызывает `common/interrupt.Group.Interrupt(false)` при выключенном option;
   `common/interrupt/group.go` сохраняет external connections. Исходники
   зависимости прочитаны, поведение в runtime не проверялось.
8. App probe проверяет trust до/после работы; публикация не выдаёт expired
   result за Ready/health. TrustUnavailable отделён от ProtectionUnavailable
   и LeakConfirmed; положительный safety fault не теряется из-за expiry.
   Watchdog/failed-publication path сохраняют фактическую Manager lifetime cause,
   а не превращают её в «VPS/канал сломан». Clock/profile/storage faults блокируют
   network retry/wakeup до исправления и явного Start/действующего импорта.
   Certificate expiry допускает прежний bounded backoff с новым render/probe.
   Value-only commit удерживает short Manager owner read guard и повторно
   проверяет clock/expiry перед записью health/UI. Lifetime retirement не может
   сменить Manager generation внутри этого commit. Порядок runtime→Manager→App
   →epoch→clock/health; guard освобождается до EventsEmit/I/O/lifecycle calls.
   Проверка порядка по исходникам не заменяет concurrency/runtime приёмку.
9. Сохранение clock observation происходит до engine mutation и на существующем
   App watchdog tick, из копии applied config, вне App/runtime/epoch/Manager locks.
   Запись floor — при шаге ≥1min; descriptor/UI poll его не продвигает.
   Writer проверяет exact version/hash под прежней filetxn serialization.
   Preview импортирует только in-memory observation, без записи floor.

## Границы, которые остаются открыты

- Это **не authenticated UTC**. Signed issued_at — утверждение издателя, не
  time attestation. После reboot неизвестно реальное offline elapsed; локальный
  floor допускает ограниченный skew/интервал последней записи и зависит от
  filesystem/ACL. Нельзя обещать устранение rollback/replay при local admin.
- Сон/hibernate, clock corrections, process restart, потеря/повреждение floor,
  disk-full/ACL/reparse/interprocess/power-loss, callback/cancellation races,
  blocked filesystem и Windows Job/WFP cleanup пока не проверены.
- Tick ≤1s в исходниках не hard wire-cut SLO. Scheduling/suspend/OS driver и
  lifecycle cleanup ограничиваются отдельно. Истечение во время операции
  отменяет cooperative context; OS/worker teardown gates всё ещё открыты.
- CA/root bundle не раскрывает actual peer leaf NotAfter. Для обычного PKI
  VLESS/WS peer leaf deadline неизвестен; native handshake verifier остаётся,
  непрерывный контроль lifetime всех произвольных PKI peers не реализован.
- Нет доказанной бесшовности pin rotation. Старая сессия не заимствует срок
  Next; recycle/backoff может прервать трафик. Валидность не равна доступности.
- Новые optional local checkpoint fields требуют согласованного rollout;
  старый strict reader может их отвергать. EXE/NSIS/update не собраны.
- Полные root/profile/update/recovery roles, authenticated freshness и явное
  offline recovery без сброса antirollback/retired/revocation history — следующий
  срез TH-03. Нет автоматического reset version/floor/pin history.
- TH-02 endpoint diversity/quorum/native families, TH-04 full bounded policy,
  TH-05…08, все OS/release/comparative gates остаются открытыми. Прежние PASS
  не являются evidence этого кода. Реальные owner certificates/keys не читались.

## Следующий шаг

Изучить текущие trusted_keys/profile/update import APIs и уже выбранные upstream
trust patterns; реализовать explicit role-bound recovery/renewal с сохранением
retained history. Не начинать tests/build/native до изменения указания владельца.
