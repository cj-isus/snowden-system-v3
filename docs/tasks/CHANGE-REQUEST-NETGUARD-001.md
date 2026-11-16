# NETGUARD-001 — внешний UserMode/SYSTEM writer

2026-10-30, Windows owner request, KA-01/W3. **Не выполнено, source proposal**.

## Причина отдельного запроса

В `.harness/ownership.json` Windows owns `windows/` и платформенные документы;
root `tools/netguard/` не назначен роли и имеет `unknown: review-required`.
Чужой чат не читался/не получал сообщение, binding/правила не менялись. Внешние
scripts/installed tasks не правились и не запускались. Этот документ — конкретный
scope для согласования владельцем; наличие запроса не разрешает его исполнение.

## Точный разрыв

- `tools/netguard/netguard.ps1` UserMode отключает ProxyEnable по совпадению
  loopback1080/dead listener, без original write receipt, PAC/full tuple и owner.
- Его SYSTEM branch определяет «VPN active» по listener и может изменять
  DNS/DoH physical adapters по собственным probes. Это не protection authority.
- `tools/netguard/install.ps1` устанавливает эти tasks, глобальную DoH policy и
  скрипт в ProgramData. Изменить source недостаточно для миграции старых copies.
- W3a внутри приложения устраняет свой heuristic writer, но внешний старый script
  может независимо менять те же настройки. Full W3 поэтому остаётся открытым.

## Предлагаемый следующий scope

1. Согласовать Windows ownership этих трёх root scripts или явную отдельную роль
   этого конкретного migration. Не расширять права автоматически.
2. Сначала убрать автоматические proxy/DNS writes без подтверждённого владельца.
   No-port/listener/probe/log heuristic recovery. Старые tasks пока не объявлять
   совместимыми/безопасными; SYSTEM readout не доказательство защиты.
3. Выбрать общий persistent original-write receipt и native interprocess owner
   для startup/manual/UserMode. Сохранить exact prior presence/type, ограниченные
   входы, write intent/readback/conflict, actual process death и частичные outcomes.
   Receipt служит recovery, не разрешением traffic/trust/direct fallback.
4. Сделать явный staged migration/rollback установленной копии и трёх task names,
   без тихой перезаписи чужих DNS/PAC/MDM/другого VPN; не менять global DoH как
   побочный эффект installer. Требуются original process/task/source/identity и
   честная диагностика mixed/legacy state, не версия/marker как acceptance.
5. Native/system/admin acceptance выполнять только после отдельного разрешения
   тестирования/OS действий. Source plan не разрешает установку или сеть.

## Приёмка (всё OPEN)

Нет foreign overwrite, no-reset при unknown/missing/typed error; bounded coherent
snapshot, exact own writes и readback, crash/reboot/partial write/Close/cancel,
cross-process/thread owner retirement, installed-old/mixed/rollback cases,
PAC/MDM/foreign VPN preservation. Обновление source и hash не считается native
PASS. До этого W3b и документ [W3a](TH-08-PROXY-OWNERSHIP.md) остаются unverified.

## V2-381 — платформенная реакция на внешний writer

2026-11-10, **source/unverified; запрос изменения root scripts не выполнен**.
Повторно прочитан source: UserMode пишет ProxyEnable по port/listener, SYSTEM
пишет DNS/DoH. Root scope по текущему ownership остаётся review-required.
Scripts, установленная копия, задачи, DNS и registry runtime не изменялись.

В Windows-коде добавлен один original native observer работающего ProxyManager;
сверка фактического tuple вызывает существующий lifecycle cleanup при конфликте
или неизвестном чтении. Проверки доверия независимы от этого I/O; исходный child
должен реально вернуться до освобождения общего owner. App различает локальную
ошибку прокси и настоящий trust fault, блокирует автоматический перебор каналов.
[Источник и ограничения](TH-08-PROXY-OWNERSHIP.md#pc-v381-proxy-writers).

Это наблюдение не нейтрализует external writer, не защищает SYSTEM DNS и не
разрешает миграцию/установку root tools. Все исходные критерии этого запроса
остаются OPEN; без runtime/admin разрешения задачи не проверять и не менять.
