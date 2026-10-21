# OFF-01 — намерение защиты и причины остановки

Статус/owner/зависимости — только PLAN §5.1, строка OFF-01. Контракт FR-009.
Это спецификация следующего сложного среза, не дополнительный журнал.

## Результат

Выразить выбранную пользователем защиту независимо от engine state: автоматическая
ошибка/retry не становится разрешением direct; явный Disconnect меняет намерение.
Исчезновение UI и смерть engine не должны иметь неописанную сетевую семантику.

## Прочитать точечно

- PLAN FR-009/010 и OFF-01/02; аудит AUD-001/002/005/014/015.
- windows/corebridge.go: startCore, stopCore, blockUnsafePath, watchCore.
- windows/app.go: Start/Stop/RunProbe/OnBeforeClose, переключение режима.
- windows/killswitch_windows.go и wfp_killswitch_windows.go: lease/disarm.
- windows/backend/core/manager.go: cleanup и switch outcomes.
- windows/autoretry.go, netwatch.go, failover.go: инициаторы автоматических событий.

Не перечитывать все исходники или весь сентябрьский журнал.

## Срезы реализации, последовательно

1. Чистая модель desired protection, StopReason и разрешённых переходов.
   Описать strict default, пользовательское отключение, update/quit/retry/crash.
2. Один владелец изменения protection intent и барьера; список call sites.
   Ввести fake policy interface для локальной проверки без системной сети.
3. Перевести automatic error на blocked protection, сохранив explicit Disconnect.
   Старое поколение операции не должно изменять intent нового поколения.
4. Согласовать поведение с helper OFF-02: auto-unblock по таймеру несовместим
   с strict crash-политикой; аварийное явное восстановление должно оставаться
   доступным и понятным. Не объявлять subprocess-приёмку выполненной по unit.

## Критерии

- Однозначная таблица событий/состояний и исключений LAN/bootstrap/split.
- Error/all-failed/пауза retry сохраняют блок; ручной Disconnect допускает cleanup.
- Потеря engine не публикуется как «защита выключена пользователем».
- Для запрошенной локальной проверки использовать injected policy и synthetic
  inputs. Отдельный production EXE/Windows VM gate остаётся OFF-02/05.
- UI/API получают только факты, не фальшивый running или success из panic.
- Завершение: изменённые call sites, результат проверок, непроверенный OS gate,
  обновлённый PLAN и заменённый STATE. OFF-01 не закрывает весь OFF-02.

## Граница

Не подключать VPS, не менять firewall/proxy рабочей машины, не переносить все
packages, не вводить Windows-службу, новый transport/UI framework и не менять
dependency versions в этом срезе. Если нужен иной объём — сначала уточнить
причину в каноническом PLAN, сохраняя предыдущие доказательства.
