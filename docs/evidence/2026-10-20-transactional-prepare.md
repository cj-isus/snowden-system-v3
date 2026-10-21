# V2-102 — подготовка Reload и адаптация upstream-подходов

Дата: 2026-10-20. Scope: OFF-03/FR-010; comparative contract — FR-012.

## Что исправлено

Reload вызывал ReloadOption под manager mutex до установки panic finalizer.
Nil/panic оставляли lock захваченным; reentry в State/Stop блокировался.
Подготовка также проверяла только state, не различая возвращение в running
после новой Stop/Start или другого успешно завершённого Reload.

Теперь prepare проверяет JSON, нормализует bytes и применяет options вне mutex
и lifecycle ownership. Nil/panic/cancel возвращают ErrReloadRejected с причиной
без cleanup прежней сессии. Context и generation проверяются перед commit.
Каждое новое владение Start/Stop/Switch/Reload увеличивает manager generation;
результат прежней подготовки отклоняется. Неприменённый отказ другой подготовки
не отзывает рабочий candidate. Порядок options и default probe сохранены.

Stop может завершиться, пока prepare callback ещё заблокирован. Сам callback
должен вернуть управление; hard bound произвольного Go callback не заявлен.
Другие lifecycle/rollback/protection инварианты V2-099…101 сохранены.

## Заимствование и лицензии

Проверены официальные repositories и license files; commits закреплены в
`.artifacts/research/2026-10-20-reload/sources.json`. Внешний код не исполняли.

| Источник | Проверенный подход | Адаптация в этом срезе |
|---|---|---|
| [Tailscale userspace.go](https://github.com/tailscale/tailscale/blob/db03d4e19da508fe29c96043e5fd49156e577f60/wgengine/userspace.go#L993-L1112), [BSD-3-Clause](https://github.com/tailscale/tailscale/blob/db03d4e19da508fe29c96043e5fd49156e577f60/LICENSE) | RequestStatus получает callback через краткую блокировку, вызывает после её освобождения; Reconfig сериализован и хранит clone config | Options вне lock, snapshot поколения и повторная проверка перед commit. Наша реализация сохраняет прежний verified rollback контракт |
| [Mullvad architecture](https://github.com/mullvad/mullvadvpn-app/blob/8fecb45fa6ff0c04549e4cccf1ea3788c293a6bb/docs/architecture.md#tunnel-state-machine), [GPL-3.0](https://github.com/mullvad/mullvadvpn-app/blob/8fecb45fa6ff0c04549e4cccf1ea3788c293a6bb/LICENSE.md) | Tunnel state machine разделяет намерение connect/disconnect и protected error; connected появляется после проверки | Сохранены переходы и strict intent проекта. Prepared candidate получает ownership только при commit; ошибочная подготовка не разрушает работающий путь |

Перенесены инженерные принципы собственной Go-реализацией. Копирования исходников,
новой зависимости или второго владельца engine нет. Правила будущих сравнений
добавлены в существующий FR-012: same-input baseline, raw evidence, safety,
connect/recovery p50/p95, throughput/TTFB, CPU/RAM/handles и probe budget.
Сравнительное превосходство ещё не измерялось.

## Воспроизведение и проверки

Первые черновые tests использовали `{}` и останавливались на строгой валидации;
они не доказывали выполнение callback. Исправлена fixture, проверки не ослаблены.
Затем overlay вернул ровно прежний prepare-префикс Reload с options под mutex;
остальной актуальный код сохранён. Production source при replay не заменялся.

Из `windows`:

```powershell
go test -overlay ../.artifacts/research/2026-10-20-reload/overlay.json ./backend/core -run '^TestReloadPrepareRejectsOptionsWithoutTouchingSession$' -count=1 -timeout=60s -json
go test ./backend/core -run '^TestReloadPrepare' -count=5 -timeout=120s -json
```

Overlay baseline: exit=1, три leaf FAIL с escaped panic и удержанным mutex.
Он воспроизводит конкретный дефект до исправления, не весь исторический checkout.
Вывод `.artifacts/tests/prepare-baseline.jsonl`; overlay source в research artifacts.

Исправленный focused: 24 leaf cases ×5 =120 PASS, 0 FAIL/skip, exit=0, 24.98 s.
JSON `.artifacts/tests/prepare-focused.jsonl`, exact command/time record в
`.artifacts/evidence/2026-10-20-prepare-focused.json`.

Полный Task backend:check: ordinary → production tags → vet, exit=0, 78.50 s. Результат,
время и stdout — `.artifacts/evidence/2026-10-20-prepare-task.json` и
`.artifacts/tests/prepare-backend.txt`. Итоговая source/check identity:
`.artifacts/evidence/2026-10-20-transactional-prepare.json`.

## Остаток

Полный facade render/preflight/observer fault matrix, process isolation и hard
bound Start/Close, upstream early cleanup, старый running helper/production EXE
subprocess, elevated Windows crash/reboot/IPv4/IPv6/UDP/DNS и native UI gates
остаются открыты. VPS не используется; EXE/NSIS не пересобирались, race при CGO=0
не запускался. Исторический V2-099 runner anomaly сохраняется в OFF-07.
