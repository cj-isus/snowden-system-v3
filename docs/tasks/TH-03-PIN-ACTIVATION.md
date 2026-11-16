# TH-03 — попытка подключения при активации HY2 Next

Статус: V2-143, 2026-10-24, **исходники, unverified**, PC/Windows only.
Тесты не добавлялись/не менялись/не запускались; compile/build/typecheck/check/
module/gen/SDK/crypto/product CLI/browser/native/live не выполнялись. `gofmt -w`
шести затронутых Go-файлов завершился exit0; поведения это не подтверждает.

## Разрешённый случай

Runtime projection формирует только scheduling hint для подписанного включённого
канала `hysteria2/quic` со статусом configured/live-verified. Канал должен быть
отключён именно из-за отсутствия активного сертификата. Подписанные/local/global
отзывы канала и сертификатов уже применены к owned projection; будущий Next
повторяет static hostname/server-auth/interval/fingerprint gate. Его NotBefore
должен быть впереди наблюдаемого времени и строго раньше expiry профиля.
Исходные signed Enabled, статусы и сертификаты не изменяются ради расписания.

App назначает таймер только после `policy_channels_unavailable`, успешной полной
остановки старого ядра и отсутствия уже доступных каналов. Это не новый общий
retry и не реакция на pending/authority/profile/clock/storage fault. Обычная
certificate-expired/backoff ветка сохраняет прежний контракт.

## Владение и повторная проверка

Планирование читает общий read-only runtime loader под существующим import
journal, без vault/PEM-файлов/сети и без checkpoint writes. Требуются реальный
сохранённый checkpoint, его exact tuple/envelope identity, активный signer,
применённая policy и непрерывность `ObservedClock`. Whole encoded trust/checkpoint
сверяются до/после чтения, затем их hashes, tuple, envelope/signer/policy SHA,
channel ID и Next fingerprint сохраняются только в private timer state.

Один pointer token принадлежит одному operationID. Существующая отмена retry
отменяет и этот timer при manual Start/Stop, close, profile retirement и success.
Устаревший callback не очищает successor. Callback сериализован operationMu,
сначала проверяет владельца, затем повторяет полный read-only source/clock gate
и exact saved source identity, после чего проверяет владельца повторно.

Задержка выводится из наблюдаемого локального времени и steady elapsed;
raw wall не определяет момент разрешения. Ранний callback может переназначить
только тот же срок, без запуска. После срока требуется действующий, включённый
проектированный канал с тем же signed Next. Ошибка clock/source/expiry оставляет
blocked и удаляет timer; бесконечный повтор по этой ошибке не назначается.

Успешный callback создаёт новую operation generation через существующую точку
обычного запуска. Start заново читает текущее доверие, создаёт новый worker и
обязан пройти protected probe. Прежний worker не возобновляется, trust policy
не изменяется, direct fallback отсутствует. UI получает срок в тексте ошибки;
таймер не выдаётся за сетевой backoff, live health или успешное подключение.

## Незакрытый объём

Нужны разрешённые владельцем compiled/clock/suspend/early-late timer/source race/
Stop-close-cutover/retirement/native/protected-probe проверки. Ни один такой gate
не пройден. При reboot расписание не сохраняется: новый явный Start проходит
обычные gates. Нет автоматической ротации ключей, обновления профиля или обхода
pending-политики. Cold minimum-policy/custody/private history, P0 и все девять
TH-00…08, SDK/module/OS/crash/DR/resources/client/bindings/rollout/release/comparative
обязательства остаются открытыми.

Исходники: `windows/backend/metadata/{pins,runtime_profile}.go`,
`windows/{pin_activation,app,autoretry,trust_lifetime}.go`.
