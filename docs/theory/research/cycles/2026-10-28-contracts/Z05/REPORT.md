# R28-Z05 — Android: вызов pause, таймер HY2 и допустимое решение

**Срез 28 октября 2026 года. Статус автора — READY_FOR_REVIEW.**
[Версии, источники и глубина чтения](SOURCES.md) · [Краткая сводка](HANDOFF.md).
Проверка продукта/native/live не проводилась. Тесты, сборка, устройство/adb,
энергия, радиомодуль и NAT — **NOT_RUN**.

## 1. Вопрос, краткий ответ и предел применения

**Вопрос:** достигает ли готовый Android callback питания таймера HY2
и какое действие обосновано без наблюдения wake/radio/NAT и полезного исхода?

**Ответ R28-Z05-A:** в рассмотренном составе исходников SFA callback доходит
до общего PauseManager, но не найдена его связь с таймером HY2/quic-go:

```text
ACTION_DEVICE_IDLE_MODE_CHANGED
→ PowerManager.isDeviceIdleMode
→ CommandServer.pause / wake
→ one Box-scoped pause.defaultManager
→ registered PauseManager subscribers only
```

Подтверждены подписчики NTP ticker, URL-test ticker, WireGuard endpoint/callers.
В названных HY2 outbound, sing-quic HY2 client и quic-go timer не найдены
`pause.Manager`, соответствующие callbacks и `WaitActive`.
`CommandServer.Pause()` сам не останавливает Box, соединения, таймеры или
UDP socket: он вызывает `DevicePause()` менеджера; эффект создают подписчики.

**Применимость:** проверить реального потребителя callback и условия решения
после сна. **Предел:** выбранные версии и поиск вне тестовых Go-файлов,
а не невозможность паузы во всех реализациях HY2:

- sing-box v1.13.19 / `b5ebaa1f…`;
- sing v0.8.13 / `7c349dac…`;
- sing-quic v0.6.4-…`d83826c…`;
- quic-go v0.59.0-sing-box-mod.4 / `ccd46abe…`;
- SFA callback commit `bc73504f…`.

Эти версии — исходный срез, не текущая идентичность установленного AAR.

## 2. Вызывающий код и создание общего менеджера

### 2.1. Android caller

SFA `BoxService.kt`:

1. При start регистрирует receiver `ACTION_DEVICE_IDLE_MODE_CHANGED`.
2. `serviceUpdateIdleMode()` читает `isDeviceIdleMode`.
3. Неограниченный coroutine channel упорядочивает булевы сообщения.
4. Потребитель вызывает `commandServer.pause()` / `wake()`.

Получение broadcast, помещение в channel, извлечение и JNI-call — четыре
события. Исходник не задаёт верхний предел доставки. Глубокий сон CPU
не гарантирует немедленного выполнения coroutine потребителя.

### 2.2. Идентичность менеджера

`Box.New` выполняет `pause.WithDefaultManager(ctx)` до создания network,
router/outbounds. `daemon.Instance` сохраняет менеджер из того же context;
libbox `CommandServer.Pause` вызывает его `DevicePause`. `defaultManager`:

- меняет `devicePaused` и заменяет либо закрывает channel;
- вызывает зарегистрированные callbacks под своим mutex;
- `WaitActive` ждёт закрытия device/network channels;
- сам не перечисляет и не останавливает транспорты.

### 2.3. Подтверждённые потребители

| Потребитель | Эффект pause | Связь с HY2 |
|---|---|---|
| sing NTP service | `RegisterTicker` останавливает ticker; после wake возможны немедленное обновление и reset | Служба часов, не HY2 |
| sing-box URL-test | Остановка/reset периодических проб выбора | Необязательная проверочная работа |
| WireGuard endpoint | Callback `device.Down/Up` | Другой транспорт |
| WireGuard client bind | `WaitActive` после ошибки connect | Граница повтора другого транспорта |
| RemoteRuleSet | Поле manager есть, но в прочитанном update loop использования не найдено | Хранение поля не является подпиской |
| HY2/quic-go | В указанных корнях не найдены extraction/register/wait менеджера | Ребро к таймеру не установлено |

quic-go создаёт отдельный `time.NewTimer`, перенастраивает его по keepalive/PTO/idle
и ждёт `timer.C` в run loop. Эти callers не проверяют PauseManager.
Достижимость Android receiver → manager поэтому не переносится на HY2 timer
без явного потребителя.

## 3. Как HY2 реагирует на изменение пути

У HY2 есть готовый, но другой путь восстановления:

```text
NetworkManager interface update
→ Outbound.InterfaceUpdated()
→ Client.CloseWithError("network changed")
→ close QUIC + raw socket
→ следующий Dial/ListenPacket lazily calls offer()
→ one pending offer shared by concurrent callers
```

Это закрытие/создание при установленном изменении интерфейса или контекста,
а не приостановка Doze. Переиспользование зависит от платформенного владельца:
он устанавливает новый или утраченный физический контекст и допускает
завершение/замену. Каждый wake сам по себе этого не доказывает.
Дополнительные QUIC/TLS/auth, CPU, радиомодуль и задержка задачи могут стоить
дороже сохранения пригодного соединения.

В прочитанном Snowden контекст содержит network handle, transports,
suspended/blocked, interface/MTU/address/DNS/routes. Mismatch/loss callback
переводит session в fault и запускает завершение worker. Но доставка callback
до первой нативной отправки после сна не доказана. Наблюдение callback
не является атомарным firewall или транспортным барьером ОС.

## 4. Состояние, часы и действие

| Состояние | Факт о сети/CPU | Таймер HY2 в прочитанной модели | Условно допустимое решение | Что неизвестно без устройства |
|---|---|---|---|---|
| Awake, тот же контекст | FGS/source допускает работу; CPU исполняет | Native monotime стареет, таймер/PTO может сработать | Сохранить соединение; нет причины для дополнительной pause/reconnect | Энергия, радио, NAT |
| Обычный Doze, foreground UID, CPU работает в окне обслуживания или по иной причине | Android16 source может освобождать foreground UID от обычной Doze/AppStandby блокировки сети | При работающем CPU таймер может исполняться; при deep suspend callback не исполняется | Наблюдать blocked/context; не закрывать только по флагу Doze | OEM/proc state, сон, порядок callbacks |
| FGS активен, CPU спит | FGS не равен WakeLock | Go/Linux reference monotime исключает suspend; callback таймера не выполняется | Сам по себе разрыв часов не доказывает гибель соединения; после возврата проверить полномочия и контекст | Сохранились ли peer/NAT |
| UID допущен в сеть, таймер не исполняется | ОС может разрешать пакеты, но спящий CPU их не создаёт | Нет подтверждённых due/queue/write | Не выводить обновление соединения из исключения ОС | Источник пробуждения и срабатывание таймера |
| Low Power Standby / blocked callback | Отдельная LPS policy может запрещать сеть FGS; путь сохраняет доставленный blocked | Нативная работа может пересекаться с доставкой | Защищённо завершать при принадлежащем операции актуальном fault | Порядок с первой отправкой после block |
| Resume, путь тот же, полномочия действительны | Peer мог забыть состояние, пока локальные часы не старели | `active()` — только локальный факт | Первая полезная операция может использовать соединение; типизированный отказ запускает ограниченное восстановление | Сохранность peer/NAT и цена reconnect |
| Resume, путь изменён или утрачен | Физический контекст недействителен | Старое соединение больше не допускается | Существующий interface-update close/recreate и владелец замены worker | Задержка callback и пакет в kernel I/O |
| Resume после deadline полномочий | `elapsedRealtime` включает сон; справочные transport clock могут его не учитывать | Таймер может не считать себя просроченным | Проверка полномочий имеет приоритет: завершить session/process, keepalive не продлевает grant | Порядок watchdog и уже начатой native write |
| Reboot/новый процесс | `elapsedRealtime` принадлежит загрузке; старый deadline нельзя восстанавливать из кеша | Старого процесса/соединения нет | Новое принятое trust/profile admission | Конкретная политика холодного восстановления |

Семантика справочных Go/Linux часов не является измерением конкретного AAR,
ОС или OEM. Матрица различает эти предпосылки, а не объявляет их проверенными
для всех Android-устройств.

## 5. Возражения и сильные ограничения

### O1 — foreground service при спящем CPU

FGS может сохранять foreground-классификацию, пока CPU находится в глубоком сне.
Разрешённая сеть foreground UID не означает исполнение таймера и пакет.
Callback pause также может не успеть до сна; его наличие не доказывает
энергетическую экономию.

### O2 — список callbacks не означает паузу всех служб

Менеджер оповещает подписавшихся. В выбранном составе найдены URL-test/NTP/WG,
но не HY2. RemoteRuleSet хранит manager без использования: доступность объекта
в context недостаточна для вывода о достигаемом эффекте.

### O3 — закрытие при каждом resume не доказано лучшим решением

Оно отбрасывает возможное старое соединение, но может ухудшать доступность
и энергию ненужными рукопожатиями. Смерть NAT из wake не следует.
Недействительные полномочия и контекст уже имеют отдельное правило завершения;
без полезного исхода универсальное close-on-resume не обосновано.

### O4 — сохранение соединения не превращает active() в готовность

HY2 `active()` проверяет локальный QUIC context/connDone. Peer/NAT могли истечь.
Переиспользование условно: первая операция должна сохранять типизированные
отказ и восстановление, а не публиковать выдуманный Ready.

### O5 — lifecycle guards не доказывают отсутствие каждого позднего пакета

Java/JNI/platform callbacks проходят guards; watchdog с интервалом 1s инициирует
завершение по `elapsedRealtime`, включая сон. Но уже работающие Go QUIC timers
и socket writes не вызывают `NativeCallbackGate` на каждом пакете.
Интервал watchdog не является жёстким временем исполнения после resume.

Правильная граница: keepalive не обновляет полномочия; будущий допуск
закрывается и запускается завершение. Строгий нулевой порядок отправок
после deadline требует отдельного платформенного барьера и приёмки.

## 6. Ограниченное решение без данных об исходе

Предлагается наблюдение → классификация → действие по установленному основанию,
а не обещание «pause экономит энергию». Схема исходного предложения сохранена
дословно; это модель контракта, не уже существующий публичный API:

```text
PowerPathObservation {
  boot/process generation,
  observed elapsed time,
  deviceIdle?, lowPowerStandby?,
  network blocked?, physical context generation/fingerprint,
  authority valid?, native session owner,
  outcome = unknown | carrier-failed | useful-completion
}

Decision (platform-owned):
1. authority invalid or owned context blocked/lost/changed → retire/fail closed;
2. same context + authority valid + no actual carrier failure → keep carrier;
3. first useful operation reports typed carrier failure → existing bounded
   close/recreate/lazy offer path, with one owner and recovery accounting;
4. optional health/NTP/URL-test tickers may pause through existing PauseManager;
5. energy optimization remains abstain/unknown until wake/wire/peer/recovery cost.
```

Такой вариант не требует transport fork, автоматического исключения из
энергосбережения, постоянного WakeLock или новой периодической пробы.
Он сохраняет единственного владельца и использует готовый recovery-путь
при установленной причине. Это обоснование ограниченного изменения,
не доказательство единственного или глобально минимального проекта.
Атомарная граница OS block/истечения полномочий и уже начатой native write
остаётся задачей платформенной защиты.

## 7. Переиспользование, лицензии и цена

| Кандидат | Прочитанная версия/лицензия | Что брать | Ограничение и цена |
|---|---|---|---|
| SFA idle receiver и последовательный caller pause/wake | `bc73504f…`; в LICENSE GPL-3.0-or-later и условие о названии/ассоциации производного приложения | Форму вызывающего пути как ориентир | HY2 не приостанавливает; весь service не согласован с владельцем Snowden. Код не копировался |
| sing pause.defaultManager / RegisterTicker | sing0.8.13 `7c349dac…`, GPL-3.0-or-later | Для реально подключённых необязательных ticker consumers того же ядра | Синхронный callback под mutex; подписка HY2 меняет core lifecycle/concurrency |
| HY2 `InterfaceUpdated→CloseWithError→offer` | sing-box1.13.19/sing-quic d838, GPL-3.0-or-later | Условно при недействительности пути или типизированном отказе | Reconnect требует CPU/радио/времени; энергетическая польза не доказана исходником |
| quic-go timer | ccd46…, MIT | Существующий механизм без приписанного pause hook | Сон, пробуждение и NAT outcome не измерены |
| Контекст, blocked callback и authority guards Snowden | Прочитанный GPL source; AAR в срезе не подтверждён | Входы решения владельца | Callback не атомарен с отправкой; продуктового proof нет |
| Power allowlist, постоянный WakeLock, закрытие на каждом wake | Механизмы и политика Android | Не включать по умолчанию из этого вывода | Пользовательская, Play и энергетическая цена; лишний reconnect не даёт peer/NAT receipt |

Прямая подписка HY2 на PauseManager пока не обоснована. Нужно выбрать смысл:
остановить таймер, закрыть соединение, запретить новые offers или отложить send.
Варианты имеют разные PTO/idle, владение очередью и гонки возобновления.
Одного булева `DevicePause` недостаточно.

Практический маршрут — ANDR-04/07/08 [плана Android](../../../../../planning/knowledge-application/ANDROID-PLAN.md).
В PC-05/06/07 [плана ПК](../../../../../planning/knowledge-application/PC-PLAN.md)
переносится различие часов, факта наблюдения и эффекта; Android broadcast
не становится Windows API. Основной разбор ресурсов и таймеров —
[QUIC/HY2](../../../../performance/QUIC-ACCOUNTING-AND-LIMITS.md).

## 8. Будущие различающие наблюдения — NOT_RUN

1. Точный принятый APK/AAR, Go, ABI и build tags.
2. Времена broadcast received → channel enqueue → JNI pause → subscriber;
   проверка порядка относительно suspend, без raw пользовательского трафика.
3. QUIC due/queued/socket-return, OS blocked callback и именованные часы.
4. Resume по тому же контексту: ответ peer, первое полезное завершение,
   reconnect только по типизированной ошибке; сравнение с безусловным reconnect.
5. Изменённый контекст: один владелец retirement/offer, отсутствие старых
   отправок после принятого платформенного барьера, без direct fallback.
6. Deadline через сон и reboot: watchdog/guard/фактический выход процесса,
   отдельная kernel/native I/O граница; keepalive не продлевает grant.
7. Radio/NAT/peer на Wi-Fi и выбранных мобильных сетях; reconnect bytes/CPU/
   latency и пропавшие либо цензурированные исходы остаются в учёте.
8. Pause URL-test/NTP до/после нагрузки; эффект не приписывается HY2
   без конкретного добавленного и измеренного подписчика.

Проценты, mAh, время NAT и гарантированное пробуждение не установлены.

## 9. Что даёт сопоставление с C04

При исходном review адресно прочитан C04 HANDOFF. Он поддерживает различие
общей pause/wake, неизвестного AAR/Go и доказанной энергии/времени, добавляет
границы ресурсов и отмены. Он не создаёт связь HY2 → PauseManager и не меняет
R28-Z05-A. Согласие пакетов не является независимым runtime-доказательством.

## 10. Предлагаемые решения и что остаётся открытым

**R28-Z05-A: предложение `ACCEPT_SOURCE_SCOPED_NEGATIVE`.** Конкретный путь
SFA Doze → CommandServer → общий PauseManager в рассмотренном составе
не достигает таймера HY2/quic-go. Он оповещает установленных подписчиков.
Нельзя утверждать, что готовый callback уже подавляет HY2 keepalive.

**R28-Z05-B: предложение `ACCEPT_MODEL_SCOPED`.** Без wake/wire/peer/NAT/useful
outcome использовать наблюдение и классификацию; завершать при недействительных
полномочиях/контексте, условно сохранять соединение при их действительности,
восстанавливаться через существующего владельца при недействительности пути
или типизированном отказе. Экономия энергии, исключение из политики питания,
широкий WakeLock и закрытие на каждом wake из source не следуют.

Эти предложения не принимают весь пакет автоматически. Не установлены
корректность продукта, ноль поздних пакетов, батарея, NAT survival, своевременность
callback, поведение конкретного устройства/OEM и готовность пользовательского пути.

<a id="r28-z05--exact-pause-caller-hy2-timer-и-допустимый-decision-seam"></a>
<a id="1-вопрос-и-ограниченный-ответ"></a>
<a id="2-exact-caller-и-shared-manager-construction"></a>
<a id="22-manager-identity"></a>
<a id="23-реальные-consumers"></a>
<a id="3-как-hy2-действительно-реагирует-на-path-change"></a>
<a id="4-state--clock--action-matrix"></a>
<a id="5-opposing-cases-and-strongest-limits"></a>
<a id="o1--fgs-active-but-suspended"></a>
<a id="o2--manager-has-callback-list-therefore-all-services-pause"></a>
<a id="o3--close-on-every-resume-is-safest"></a>
<a id="o4--leave-carrier-and-trust-active"></a>
<a id="o5--authority-gates-eliminate-post-expiry-packets"></a>
<a id="6-minimal-decision-seam-without-outcome-data"></a>
<a id="7-reuse-licence-and-cost"></a>
<a id="8-future-distinguishing-observations--all-not_run"></a>
<a id="9-paired-c04-input"></a>
<a id="10-scoped-verdict-proposed-to-lead"></a>
