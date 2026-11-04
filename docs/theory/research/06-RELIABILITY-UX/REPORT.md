# R06 — отказ, безопасное восстановление и понятное действие

Дата исследования: **2026-10-25**. Статус: **READY_FOR_REVIEW**; решение научного руководителя не получено.

## 1. Вопрос, метод и предел доказательства

Как человеку продолжить конкретную домашнюю или рабочую задачу после отказа,
сохранив заявленную область защиты и понимая следующий шаг?

Это desk research: тематическое чтение проекта, первичных документов,
двух закреплённых реализаций и AOSP. Приложения, VPS и устройства не запускались;
тестов, симуляций, usability-сессий и измерений скорости/доступности нет.
Чтение public source по HTTPS не является проверкой VPN-пути.

В отчёте различаются:

- **Source-факт** — конкретный просмотренный код, не доказанное runtime-поведение.
- **Внешнее основание** — утверждение первоисточника с областью и датой в [SOURCES](SOURCES.md).
- **Вывод R06** — наша аргументация/операционный контракт; не новый мировой научный результат.
- **UX-гипотеза** — ожидаемая польза интерфейса, ещё не проверенная на людях.

Срез ограничен пятью сценариями и тремя дополнениями. R02 определяет механизмы
блокировок; R03 — принятие решений; R04 — packet/resource tuning; R05 — authority
и security invariants. Здесь задаются пользовательская цель, наблюдаемость,
восстановимость и контракт передачи им. Реальные секреты и профили не читались.

## 2. Что уже известно и чего этот срез не дублирует

| Baseline проекта | Уже установленное | Дополнение этого среза |
|---|---|---|
| [THEORY §7.3](../../README.md), §13.10 | Общие failure domains, маленький VPS, offline restore и эксплуатационные сроки | Проверять также зависимости **самого восстановления**, его порядок и допустимость к моменту применения |
| THEORY §12.8, §15.3/5–6; [PLAN §5.4](../../../PLAN.md) COM-01…15 | Task profiles, readiness, quiet, portal, LAN, support и update уже запланированы | Единый эпизод задачи, явные условия окончания, три независимые оси состояния и проверяемые действия |
| THEORY §16.4–6 | Journal ограничен, приватность до записи, offline custody требует независимого основания | Не превращать ring-журнал в историю доступности; отдельно определить знаменатель recovery SLI и предел сохранённого окна |
| THEORY §17.3–4 | Слабый измеритель, таймауты, censoring и selection bias | Применить эти ограничения к **пользовательскому эпизоду**, а не объявлять ещё один алгоритм оценки каналов |
| [Android §5](../../../ANDROID-PLAN.md) | Страницы, consent, portal/grants, split, recovery, a11y и будущая usability-приёмка | Привязать обещание действия к доступным полномочиям ОС; проверить реальный статус portal API |
| [configs/README](../../../../configs/README.md), [INFRASTRUCTURE](../../../INFRASTRUCTURE.md) | Baseline сервера не полная production parity; исторически общий VPS/CF account | Не принимать историческое состояние за живую инфраструктуру; предъявлять restore prerequisites и provenance |

### 2.1. Существенные source-факты нашего клиента

1. [Windows Dashboard](../../../../windows/frontend/src/views/DashboardView.vue),
   `heroDescription/heroTone`: основной текст опирается на lifecycle `running`,
   `tunMode` и `blockedReason`. [AppState](../../../../windows/frontend/src/api/contract.ts)
   содержит `probeLastAt/coreReady`, но отдельного контракта свежего результата
   **пользовательской задачи** там нет. Это основание уточнить проекцию, а не
   утверждение о найденной утечке или о провале всех backend gates.
2. [netwatch.run](../../../../windows/netwatch.go) пропускает итерацию при
   `tunRequested()`. Комментарий отсылает к сторожу; фактическая верхняя граница
   восстановления под TUN этим чтением не установлена. [autoretry](../../../../windows/autoretry.go)
   уже имеет backoff, `NextRetryAt`, единственный begin и запреты повтора при
   unresolved custody. Второй retry daemon здесь не нужен.
3. [Android ConnectionState](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/ConnectionState.kt)
   разделяет intent/phase/issue/protection/network и намеренно не придумывает READY.
   [ConnectionPresentation](../../../../android/app/src/main/kotlin/org/snowden/android/app/ui/ConnectionPresentation.kt)
   учитывает возраст worker/parent observations; `UI_FACTS_RECENT_MS=5000` —
   presentation aging, не security deadline или измеренное SLO.
4. [PhysicalNetworkState](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/PhysicalNetworkState.kt)
   уже различает OS validation, captive/partial/metered/roaming и process-scoped
   epoch; его комментарий запрещает считать validation результатом VPN/service.
   [VpnSessionRepository](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/VpnSessionRepository.kt)
   имеет budgeted recovery/retirement и journal. Production `RuntimeSourceProvider`
   остаётся отдельным gate; никакого работающего Android VPN этот отчёт не подтверждает.

Точные входы и границы чтения — SOURCES §3 и ignored captures. Общий исходный tree
untracked; `git diff` не является полным перечнем файлов или доказательством исполнения.

## 3. Пять критических сценариев

### S1. Утро: холодный запуск, offline-профиль и возврат после обновления

**Цель:** открыть рабочие ресурсы, не разбираясь в транспортах и служебных ключах.
Предусловия различаются: первый запуск без доверия; ранее принятое состояние;
запуск после update/reboot. Одного «подключить» для них недостаточно.

- **Отказы:** delivery недоступен; принятый cache отсутствует/не читается;
  authority истекла или неизвестна; пользователь отказал в consent; installer
  запустил процесс, но готовность не подтверждена. Несовместимый signer —
  отдельная ошибка, не предложение удалить приложение.
- **Наблюдаемое состояние:** профиль/authority и причина отказа, consent,
  applied protection scope, поколение процесса, текущие path observations,
  возраст доказательства. Наличие файла, подписи или процесса не означает доступ.
- **Safe recovery:** существующий accepted cache используется только по правилам
  R05/TH-03; иначе явный offline import/re-enrollment. Сохранить историю доверия;
  отмена preview не применяет половину профиля. После обновления — новое
  подтверждение применимого пути, без автоматического сброса security floors.
- **Текст/действие:** «Нужен профиль» → импорт файла/QR; «Нужно разрешение VPN» →
  системный consent; «Срок полномочий профиля истёк» → получить допустимый новый
  пакет. Нельзя по таймауту delivery писать «профиль отозван».
- **Failure domains:** installer/signing/distribution, publisher/root-policy
  receipt, локальное хранилище/ключ, часы, OS consent. Backup на том же потерянном
  устройстве или publisher за неработающим VPN не замыкает recovery.
- **SLI/SLO:** отдельно cold start и returning start; окончание успехом только
  после актуальных gates и выбранного scope проверки. Время ожидания человека
  не стирается из user elapsed time, но отдельно показывается в decomposition.
- **Будущая приёмка:** cancel, stale/expired cache, потеря key, неверный signer,
  restart между стадиями; ни один не создаёт READY или reset доверия.

**Различие ОС:** Windows proxy scope не равен всему устройству и GUI autostart
не boot protection. Android consent/always-on/lockdown принадлежат ОС; reread
после Settings обязателен. Основание: Android VPN [E08](https://developer.android.com/develop/connectivity/vpn).
Адресаты: COM-01/13/14, FR-008…011, TH-03/08; AND-03/04/08/13/14/17.

### S2. Рабочий звонок: Wi-Fi → другая сеть, сон и пробуждение

**Цель:** минимальный полезный перерыв без смены выхода ради небольшого выигрыша.

- **Отказы:** исчезновение адаптера, смена DNS/маршрута, флаппинг, поздний callback
  старой сети, сон в середине retry; новая сеть metered/roaming. Установленный
  TCP/UDP поток может оборваться даже при корректном новом туннеле.
- **Наблюдаемое состояние:** intent пользователя, новый network epoch,
  актуальность applied path/protection, стадия восстановления и причина
  ожидания. Старый successful probe остаётся историей, не доказательством новой сети.
- **Safe recovery:** один lifecycle owner коалесцирует события, отзывает
  неприменимые grants, перепроверяет допустимый путь; optional bulk/update
  откладываются quiet mode. Safety/revoke не откладываются. Не запускать
  конкурирующий lifecycle ради сравнения каналов.
- **Текст/действие:** «Сеть изменилась. Проверяем защищённый путь»; следующий
  retry — только если реально назначен. При policy conflict — конкретная причина,
  а не бесконечное «переподключение». Quiet включается явно, имеет конец и отмену.
- **Failure domains:** доступ ISP/radio, local monitor и его очередь,
  gateway/exit, task endpoint. Один failed endpoint не доказывает отказ всех путей.
- **SLI/SLO:** время до нового подтверждённого полезного доступа; отдельно число
  вмешательств, ненужных switches и незавершённых эпизодов. Сохранение конкретного
  звонка — отдельный end-to-end outcome; общий HTTP probe его не доказывает.
- **Будущая приёмка:** late success старого epoch, sleep дольше freshness,
  rapid flap, отмена quiet, lost observer, metered policy; ни один не продлевает
  proof/grant или создаёт второй owner.

**Различие ОС:** Windows monitor в просмотренном TUN-пути пропускает polling;
Android уже имеет physical epoch contract, но source не device acceptance.
Продолжительность через сон должна использовать названный clock: Android
`elapsedRealtime` включает deep sleep, `uptimeMillis` — нет; Windows
`QueryUnbiasedInterruptTime` также исключает сон. Это разные метрики, их нельзя
сравнивать как одинаковое user downtime. [E17](https://developer.android.com/reference/android/os/SystemClock),
[E18](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryunbiasedinterrupttime).
Адресаты: COM-03/09/12, TH-01/04; AND-02/03/05/07/10/11/15.

### S3. Гостиница: captive portal при включённой строгой защите

**Цель:** войти в разрешённую сеть, не открыв весь пользовательский трафик наружу.

- **Отказы:** portal требует web/SMS/payment/редирект; detector не ответил;
  walled garden даёт ложный успех probe; текущая OS policy не позволяет узкий
  exception. Timeout — только отсутствие результата, не установленный portal/DPI.
- **Наблюдаемое состояние:** OS portal hint или собственное scoped observation,
  источник/возраст/сеть, applied protection, доступность разрешённого handoff.
  `INTERNET/VALIDATED` — OS hints, не VPN egress/task evidence.
- **Safe recovery:** использовать доступный системный путь входа или допустимый
  ограниченный grant лишь после platform/R05 admission. Если ограничить scope
  нельзя — остаются защищённое ожидание, Settings/handoff или другая сеть.
  Возврат из браузера/Settings не означает успешный вход; нужны новые observations.
- **Текст/действие:** «Сеть требует входа» при соответствующем evidence;
  «Проверка сети недоступна» при unknown. Основная кнопка — «Открыть настройки
  Wi-Fi»/«Повторить проверку»/«Выбрать другую сеть» по фактической возможности.
  Не предлагать отключить всю защиту как обязательный скрытый этап.
- **Failure domains:** portal, его external identity/payment endpoints, ОС,
  actual interface, DNS и grant enforcement. Новый redirect не получает
  автоматически права старой страницы.
- **SLI/SLO:** отдельно время до **корректного выполнимого действия** и время
  до подтверждённого защищённого доступа. Показ подсказки — не recovery success.
- **Будущая приёмка:** portal hint/timeout/нет Activity/cancel/возврат без логина,
  network switch во время входа, expired grant, crash; READY только после нового gate.

**Различие ОС:** Android source API ограничения проверены в R06-03. Windows WFP
grant имеет другой enforcement; один UX contract не делает механизмы одинаковыми.
Основания: [Android network state E09](https://developer.android.com/develop/connectivity/network-ops/reading-network-state),
[Settings E11](https://developer.android.com/reference/android/provider/Settings#ACTION_WIFI_SETTINGS).
Адресаты: COM-03/08/10; AND-03/05/08/10; R05, без присвоения ему алгоритма.

### S4. Дом/офис: банк, принтер, внутренний DNS и другой корпоративный VPN

**Цель:** выполнить разрешённую задачу и заранее понимать область исключения.

- **Отказы:** одинаковые private ranges дома и в офисе; DNS идёт не тем путём;
  direct app исключён, но OS lockdown блокирует его; shared UID/process;
  корпоративный full-tunnel/managed policy конфликтует с Snowden.
- **Наблюдаемое состояние:** effective route+DNS+identity и границы покрытия,
  применённая policy, OS lockdown/work-profile facts. Намерение «банк напрямую»
  не является фактом доступности банка.
- **Safe recovery:** сначала feasibility/explain, затем controlled apply;
  unknown identity не direct. Домашний LAN grant не наследуется публичным Wi-Fi.
  Если выбранные требования несовместимы — показать ограничение и допустимый
  выбор, не пытаться замаскировать его другим транспортом.
- **Текст/действие:** «Системная блокировка запрещает доступ исключённому приложению»;
  «Корпоративная политика управляет этим подключением». Доступная альтернатива
  зависит от policy: защищённый маршрут, другая сеть/профиль, согласование с IT.
- **Failure domains:** пакет/UID/process identity, policy compiler, DNS,
  LAN/router, OS/MDM, task service. Повторный probe gateway их не исправляет.
- **SLI/SLO:** отдельно успех policy application и успех задачи; считать
  preventable configuration errors и лишние ручные действия. Не оценивать
  несовместимую конфигурацию как «нашёл более медленный сервер».
- **Будущая приёмка:** conflict до Apply, deny/unknown package, network change,
  shared identity, rollback policy, другой VPN в том же Android user/profile.

**Различие ОС:** Android допускает один активный VPN service на user/profile;
work profile имеет собственную границу. Mullvad отдельно документирует, что
Android lockdown мешает excluded apps/LAN, тогда как desktop semantics иные.
Это предупреждение конкретного продукта, не доказательство поведения всех OEM
или всех видов LAN маршрутизации через TUN. [E08](https://developer.android.com/develop/connectivity/vpn),
[E06](https://mullvad.net/en/help/using-mullvad-vpn-app),
[E07](https://mullvad.net/en/help/using-mullvad-vpn-on-android).
Адресаты: COM-04/05/06/08; AND-03/06/08/09/10.

### S5. VPS недоступен: неоплата, плохой rollout, сертификат или потерянный control plane

**Цель:** восстановить доступ из подготовленного состояния либо получить
осмысленное действие владельцу, сохранив доверие и защиту.

- **Отказы:** все transports разделяют выключенный origin; publisher/backup/
  инструкция восстановления находятся на нём же; reserve существует только
  в панели, но не provisioned; оплаченный сервер имеет disk-full/bad config;
  локальный accepted cache годен, но новый доступ к issuer отсутствует.
- **Наблюдаемое состояние:** пути не проходят проверку; health/billing/cert
  источники отдельно с as-of и provenance. Клиентский таймаут не позволяет
  отличить неоплату от ACL, ISP, overload или отказа measurement endpoint.
- **Safe recovery:** сначала уже доступный, допустимый и подготовленный путь;
  иначе bounded retry + offline runbook/packet. Не создавать fresh trust из
  входящего backup и не возвращать revoked credential. Успешный process start
  после restore не подтверждает новый protected egress.
- **Текст/действие:** «Пути недоступны; причина не установлена». «По внесённым
  владельцем данным оплата до …» — только с источником/возрастом. Действие:
  проверить независимый канал обслуживания/оплату или импортировать допустимый
  recovery packet. Admin/provider keys не помещаются в клиент.
- **Failure domains:** VPS/ASN/prefix, origin/exit, DNS/PKI/CDN, publisher,
  payment account, credential custody, storage и доступ оператора к runbook.
- **SLI/SLO:** отдельно warm failover, cold restore и ручное восстановление;
  отсчёт не начинается заново от каждого retry. Неоплаченный единственный VPS
  означает отсутствие серверной мощности, которую алгоритм не создаст.
- **Будущая приёмка:** нет publisher при живом reserve, нет reserve при целом
  backup, stale billing, old credential revoked, restart устройства, partial
  update/low disk и post-restore path gate. Это будущая матрица, не проведённый drill.

Основание принципа предварительной подготовки — AWS static stability
[E02](https://aws.amazon.com/builders-library/static-stability-using-availability-zones/).
Текущие цены/провайдеры не выбирались; INFRASTRUCTURE датирован 2026-09-24,
по сообщению владельца VPS не оплачен. Адресаты: COM-13/14/15, TH-03/07/08;
AND-04/13/14/17; infra — предложения через руководителя.

## 4. Три дополнительных вывода

### R06-01. Приёмка восстановления требует эпизода задачи и раздельных осей состояния

**Тезис.** Одного `running/blocked` и последовательности probe results недостаточно,
чтобы измерить восстановление или выбрать безопасное действие пользователю.
Нужны связанный эпизод задачи и раздельная проекция: применённая защита,
доступность проверяемой цели, процесс восстановления.

**Что добавлено сверх baseline:** useful-task-within-D, end-to-end wait и
censoring уже есть в §17.4. Новое здесь — связь этого требования с applied
protection receipt, проекцией действия в UI и созревшей cohort эпизодов.
Это уточнение операционной спецификации, не новый алгоритм path selection.

**Основание.** Наш Dashboard публикует агрегированный lifecycle текст; Android
уже избегает недоказанного READY. Закреплённый Mullvad хранит отдельно причину
ErrorState и `block_failure`: даже намерение блокировать не тождественно
успешному применению firewall. Это source pattern, не наша runtime гарантия.
[E12](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/talpid-core/src/tunnel_state_machine/error_state.rs).
Google SRE рекомендует SLIs по значимым пользовательским исходам и явно
различает спецификацию SLI и её наблюдающую реализацию.
[E01](https://sre.google/workbook/implementing-slos/).

**Предложение, не новый production DTO:** существующий lifecycle/repository
владеет компактным эпизодом; UI только отображает его.

| Ось | Примеры значений | Чего значение не доказывает |
|---|---|---|
| Protection | applied в известном scope / проверяется / не подтверждена / выключена пользователем | Доступность task target; весь device при proxy-only; отсутствие любого OS-originated трафика |
| Task/path | подтверждён в указанном scope и возрасте / недоступен по наблюдению / partial / unknown | Доступность логина/медиа/всех сайтов; блокировка цензором как причина |
| Recovery | автоматически пробуем / ждём человека / ждём сеть / остановлен intent / завершён | Уже применённая policy или успешное завершение задачи |

Нужные связи: process/boot scope, episode ID, intent revision, network/policy/
profile generation, start trigger/time, применимая observation identity,
stage/reason, назначенный retry/deadline и разрешённое действие. Это требования
к связи существующих данных; окончательный wire/schema определяют владельцы.
Новый polling timer не может приписать старому proof свежую дату.

**Условия окончания:**

- `CONFIRMED_READY` — нужные applied protection/authority и текущий path/task
  outcome в заявленном scope; generic probe не становится проверкой аккаунта.
- `ACTION_REQUIRED/BUDGET_EXHAUSTED` — объяснённое завершение автоматической
  стадии, **не** успешное восстановление; эпизод задачи остаётся нерешённым.
- `USER_STOP` — завершение intent, не авария, не автоматический успех и не
  обязательно снятая OS-защита. Семантика показывается отдельно.
- Потеря процесса/истории — `observation_lost/unknown`; длительность не додумывается.

**SLI-модель, собственный вывод.** Для заранее выбранного типа сценария,
окна и deadline D зарегистрировать N наблюдаемых допустимых эпизодов.
В отчётный N входят все эпизоды выбранной cohort, для которых deadline уже
наступил к моменту среза, независимо от раннего успеха. Незрелые эпизоды —
отдельный pending count; включать только их быстрые успехи нельзя. Правило
допустимости задаётся до исхода, потеря сети сама по себе не исключает отказ.
К сроку D: S имеют подтверждённый своевременный успех; F — подтверждённое
отсутствие такого успеха; U — исход неизвестен; N=S+F+U. При полном учёте этой
cohort доля истинных своевременных успехов лежит в `[S/N, (S+U)/N]`.
Это детерминированные границы по недостающим исходам, **не confidence interval**.
`N=0` → нет оценки. Ненаблюдаемые до регистрации эпизоды не включены: покрытие
cohort нужно сообщать отдельно, нижняя граница не описывает всех пользователей.

Для продукта можно отдельно установить SLO **подтверждённого** восстановления
`S/N`, где отсутствие нужного proof не даёт зачёт независимо от неизвестной
реальной связности. Поздний результат не переписывает timely outcome; продолжающийся
retry не создаёт новый знаменатель. User-cancel регистрируется отдельно по заранее
заданному правилу, а не вычищается задним числом. Latency только успешных эпизодов
публикуется вместе с N/F/U; иначе длинные отказы исчезнут из p95.

Срок D и service/task scope фиксируются **до** будущего сравнения по типу сценария;
численные targets сейчас не обоснованы baseline. Раздельно считать feedback
latency, time-to-action, time-to-confirmed-access, ручные вмешательства и unknown
coverage. Это дополняет §17, не заменяет статистику/каузальность R03.

Protection/authority invariants оцениваются отдельно: нарушение не компенсируется
успешными задачами и не получает разрешённую долю из availability error budget.

**Контраргумент/цена:** журнал каждой стадии создаст privacy/RSS/notification cost.
Поэтому использовать существующие bounded observations и SessionJournal; ring
не превращать в durable полный аудит. Локальная агрегированная статистика
допустима только с явной схемой retained window/dropped counts. Не собирать
URL, аккаунты, SSID, список приложений или реальный рабочий трафик ради SLI.

**UX-гипотеза:** короткое «что известно → что делаем → что можете сделать»
уменьшит неверное ручное отключение защиты. Её не подтверждают наличие кнопки
или исследование чужих пользователей. Корпоративное качественное исследование
11 экспертов и 7 неэкспертов выявляло различия mental models; его нельзя
экстраполировать на всех пользователей в РФ. [E15](https://www.usenix.org/conference/usenixsecurity22/presentation/binkhorst).
Работа 2025 года с 217 участниками также не доказывает эффективность нашего
интерфейса. [E16](https://www.usenix.org/conference/usenixsecurity25/presentation/akgul).

**Оповещения:** изменение существенного состояния должно быть доступно
screen reader без принудительного focus; не озвучивать каждую секунду retry.
WCAG status messages применимы прямо к web UI; Android использует свои semantics,
а не HTML ARIA. [E14](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html).

**Reuse:** Mullvad — reference семантики, GPL-3.0 root license просмотрена;
Rust state machine не переносить в Go/Kotlin. Существующие App/repository,
ProbeObservation и journal — основа. Доказательство new scope у R05/платформ,
решение retry у R03; UI не источник authority.

### R06-02. Резерв считается восстанавливаемым только при замкнутых зависимостях recovery

**Тезис.** Граф independence транспорта нужно дополнить графом необходимых
действий восстановления. Независимый reserve бесполезен, если его адрес,
допустимый профиль, ключ/receipt, DNS, executable или инструкция доступны
только через отказавший контур.

**Основание.** AWS static stability требует подготовить необходимое до отказа,
чтобы recovery не зависел от немедленного provisioning/control-plane changes.
Это операционный принцип, не рекомендация покупать AWS или копировать multi-AZ.
[E02](https://aws.amazon.com/builders-library/static-stability-using-availability-zones/).
Свежий Tailscale changelog v1.104.1 от 2026-10-07 отдельно объявляет netmap caching
для Linux/Windows/macOS. Их статья ограничивает идею ранее подключённым устройством
и persistent cache; mobile release не следует из desktop записи.
[E03](https://tailscale.com/changelog), [E04](https://tailscale.com/blog/making-tailscale-faster).
Это зрелый пример принципа, а не готовый signed-profile cache для Snowden.

**Модель, собственный вывод.** Для каждого отказа F и требуемого действия a
перечислить `preconditions(a)` и зависимости их получения. Recovery замкнут
относительно F, если существует конечный разрешённый порядок действий от
**реально доступных после F** inputs до свежего принятого path result.
Цикл вида «для скачивания профиля нужен VPN, для VPN нужен профиль» не имеет
такого порядка без независимого начального input. Несколько документов на
разных доменах при общем origin могут иметь тот же cut set.

У графа AND/OR semantics: для пути нужны все обязательные prerequisites;
существование одной полной альтернативы достаточно только в пределах её
собственных gates. Unknown dependency не считать доступной. Не умножать
uptime VPS и CDN без основания независимости.

**Временная граница:** для конкретного действия учитывать только реально
применимые authority/grant/cert gates, определённые R05/существующей policy.
Если их допустимые deadlines `d_g` известны в совместимом clock domain,
`H_a(t)=min(d_g-t)` — верхняя граница окна для этого набора prerequisites.
При неизвестной применимости/clock/window результат unknown; H не вероятность
и не гарантия доступности. Время стадий recovery должно помещаться в их
собственные окна, а необходимые gates перепроверяются у позднего commit.
Одна конечная длительность backup file не доказывает это свойство.

**Не вводить ложное общее expiry:** истёкший timestamp delivery metadata не
обязательно запрещает Start ранее принятого профиля; accepted-cache semantics
определяет TH-03/R05. Нельзя ради удобства продлить authority или принять старую
revoked политику. Срок оплаты от пользователя — отдельный manual as-of факт,
не криптографический deadline и не доказательство health сервера.

**Минимальная операционная карточка существующего reserve/restore:**

| Что требуется | Что хранить/показывать | Что остаётся неизвестным без проверки |
|---|---|---|
| Путь/endpoint | Источник конфигурации, реальные shared origin/ASN/control-plane зависимости | Достижимость из конкретной российской сети сейчас |
| Профиль/доверие | Accepted identity/checkpoint reference и применимость, источник recovery approval | Глобально последний отзыв или возможность re-enrollment offline |
| Локальная копия | Наличие поддерживаемого пакета/runbook и граница ключа/хранилища | Возможность decrypt/restore на новом устройстве и итоговый path |
| Серверный restore | Последовательность config/secrets/key custody/version/deploy/post-verify у владельца infra | Restore duration, parity шаблона и утрата server-side data |
| Оплата/сертификат | Manual/provider provenance, as-of, срок применимого документа | Причина текущего таймаута; автоматическое продление |

Это не новый публичный пакет с admin ключами. Содержимое секретного recovery kit
не создавалось и не читалось. Нужна согласованная модель R05/infra, а не клонирование
root/key/credential на все устройства. Loss одного device key не лечится сбросом trust.

**Контрпример:** reserve заранее provisioned, но его credential отозван —
замкнутый технический граф не делает его допустимым. Обратный контрпример:
валидный kit не создаёт замену единственному выключенному VPS. Поэтому четыре
отдельные оценки: подготовленность, authority, доступность и доказанная приёмка.

**Цена и бюджет:** 12000 руб/год ограничивает суммарные VPS + domain/storage +
fees/FX + renewal/traffic расходы, а не только рекламную месячную цену.
Тарифы здесь не проверялись. Первый полезный шаг — ручной inventory/provenance,
независимо доступный runbook и будущий restore drill. Второй постоянно оплаченный
узел — решение по нужному сценарию и коррелированным отказам; количество стран
само по себе не доказательство. Тяжёлый monitoring/control-plane stack не требуется.

**Reuse:** сохранить существующий accepted cache/TUF workflow и file transaction
у R05/платформ; Tailscale netmap cache — reference идеи, не импорт их authority
или SDK. Просмотренный Tailscale pin объявляет Go 1.27.3; он не drop-in для Go1.26
проекта. Cold restore и warm failover имеют разные зависимости/время.

### R06-03. Быстрое действие в UX должно проходить проверку полномочий и совместимости

**Тезис.** «Открыть portal», «банк напрямую», «локальная сеть» и «остановить»
нельзя унифицировать одной кнопкой pause. Действие разрешается только при
известной возможности платформы и policy; иначе нужен явный handoff/ограничение.

**Новый проверенный source-факт:** в AOSP Android 16, immutable Connectivity
commit `627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`,
`ConnectivityManager.startCaptivePortalApp(Network)` помечен `@hide`,
`SystemApi.Client.MODULE_LIBRARIES` и требует privileged network permission;
overload с extras также закрыт NetworkStack permission. Совпадение source bytes
tag/commit подтверждено SHA256. Это не публичная callable возможность обычного
APK. Reflection/system permissions не предлагаются как обход.
[E10](https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/framework/src/android/net/ConnectivityManager.java).

Публичный `ACTION_WIFI_SETTINGS` открывает настройки, но matching Activity
может отсутствовать и контракт не возвращает результат portal login.
Следовательно доступность handoff и исход входа — разные факты.
[E11](https://developer.android.com/reference/android/provider/Settings#ACTION_WIFI_SETTINGS).
Проверен этот AOSP snapshot и текущая public reference, не все OEM/API устройства.

**Второй source-факт, препятствующий слепому reuse:** Tailscale captive Detector
возвращает `false` также при detection errors. Его boolean недостаточен для нашей
трёхзначной проекции `possible/not_observed/unknown`. В pin есть interface-bound
короткие HTTP requests/no redirects, но error → false теряет смысл для вызывающего.
[E13](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/captivedetection/captivedetection.go).
Даже положительный HTTP признак не доказывает legitimate login URL или разрешение
на arbitrary redirect/DNS bypass. Нужны собственные typed outcomes и budgets.

**Предлагаемый контракт действия** над существующим owner, не новый универсальный
исполнитель: `доступность → разрешённый scope → явное согласие → execution →
новое наблюдение → отзыв временного доступа → подтверждение защищённого пути`.
Потеря актуальности intent/сети/policy на любой стадии отменяет старое действие.
Открытый Settings/browser не означает успешное execution и не повышает trust.

| Случай | Допустимое обещание интерфейса | Недопустимое обещание |
|---|---|---|
| OS сообщает portal, обычный Android APK | Открыть доступные Wi-Fi settings/объяснить системный вход; перечитать сеть | Напрямую запустить hidden privileged API или гарантировать вход под lockdown |
| Strict policy не допускает узкий grant | Сохранить защиту, предложить другую сеть/допустимый маршрут/осознанный handoff | Временно открыть весь device, назвав это «безопасным входом» |
| Excluded app при OS lockdown | Explain фактическое ограничение до Apply; показать разрешённые варианты | Обещать работающий direct по одному app toggle |
| Другой VPN в том же Android profile | Показать конфликт/выбор пользователя в пределах OS policy | Незаметно заменить корпоративный VPN ради auto-recovery |
| Пользователь нажал Stop | Отобразить принятие intent, завершение worker и реальное состояние OS guard раздельно | Считать исчезновение процесса снятием lockdown или полезным recovery |

**Контраргумент:** слишком много объяснений замедляет ежедневную задачу.
Предлагается один ясный status и одно основное выполнимое действие, детали
раскрываются по запросу. Disabled action имеет причину и альтернативу. Это UX
гипотеза; пользовательские успех/понимание/ошибки ещё нужно проверить. Обход
корпоративных политик не является целью сценария.

**Reuse/license:** AOSP API declarations — reference, Apache-2.0 notice в файле;
наша реализация использует public SDK. Tailscale detector — adapt selected ideas,
BSD-3-Clause, без копирования DERP инфраструктуры, raw address logs и direct
dialer в protected probes. Mullvad desktop/Android docs — reference semantics;
их разные исключения не превращаются в общую гарантию Snowden.

На Windows NCSI также описывает результат собственных OS probes/окружения,
а не protected VPN/task attestation. Успешный системный индикатор не заменяет
наше доказательство scope. [E19](https://learn.microsoft.com/en-us/windows-server/networking/ncsi/ncsi-overview).

## 5. Конечная будущая приёмка — не запущенные тесты

Все случаи ниже — задания разработчикам/QA после отдельного разрешения и
наличия нужных реализаций. Численные D, workload, OS/OEM, service scope и stop
rules утверждаются до измерения. **Сейчас ни одна строка не PASS.**

| № | Случай | Ожидаемый различимый результат |
|---|---|---|
| A01 | Running с устаревшим proof после новой сети | Нет current task READY; исторический результат остаётся историческим |
| A02 | Ошибка транспорта при успешном/неподтверждённом применении barrier | Различимые protection outcomes; не выводятся из одной причины BLOCKED |
| A03 | Два retries одного отказа | Один episode denominator, отдельные attempts; терминальный результат не переписывается |
| A04 | Поздний success старого intent/epoch | Не завершает новый эпизод и не освежает proof |
| A05 | Process restart/evicted journal/missing observer | Unknown/limited coverage, без выдуманного непрерывного времени |
| A06 | Достигнут D, пришёл более поздний успех | Timely outcome сохранён; факт позднего восстановления отдельно |
| A07 | User Stop при enabled OS lockdown | Intent, retirement и OS guard показаны отдельно; нет auto-retry против intent |
| A08 | Offline returning start с допустимым accepted cache | Не требует лишней online dependency; authority/path всё ещё проверяются по контракту |
| A09 | Cold start без accepted cache/receipt/key | Явный недостающий prerequisite; нет fresh trust из самого входящего пакета |
| A10 | Reserve IP другой, issuer/backup на dead origin | Граф показывает незамкнутое recovery, а не «два независимых сервера» |
| A11 | Reserve provisioned, authority/credential недопустимы | Путь не eligible независимо от доступности порта |
| A12 | Валидный kit, все серверы выключены | Не обещает восстановление; действие владельцу/другая подготовленная мощность |
| A13 | Истёкший manual paid-until, path жив | Billing предупреждение отдельно от path health; provenance/as-of видимы |
| A14 | Portal hint против timeout detector | Различимые possible и unknown; нет автоматического общего grant |
| A15 | Settings отсутствуют/пользователь отменил вход | Восстановимый handoff outcome; не READY и не crash |
| A16 | Сеть изменилась во время portal/grant | Старый scope отозван; новый путь требует новой допустимости |
| A17 | Redirect требует неизвестный домен | Нет автоматического расширения destination/DNS permissions |
| A18 | LAN/excluded app плюс lockdown | Conflict/explain до Apply; никаких ложных обещаний обхода OS guard |
| A19 | Quiet и срочный revoke/leak | Safety сильнее quiet, optional update/bulk отложены; outcome объяснён |
| A20 | Update/restore запускает процесс, protected probe не подтверждён | Launch success отдельно, пользовательский эпизод не завершён успехом |
| A21 | Длинный sleep/смена wall clock | Названный clock/boot scope; нет продления authority/grant или несопоставимого downtime |
| A22 | Support export/Cancel/отсутствие сети | Review одних bytes, явная отправка, без raw секретов/identity; offline support не ломает recovery |

Отдельная будущая usability-приёмка: человек без подсказки различает
«путь недоступен», «защита не подтверждена» и «нужно действие»; выполняет S1…S5;
видит последствия Stop/LAN/portal; может вернуться из error. Измерять success,
time, ошибочные опасные действия и требуемую помощь. Включить keyboard/screen
reader/крупный шрифт; выборку/критерии определить заранее. Desk research не
устанавливает процент успеха, рыночную уникальность или превосходство.

## 6. Предложения разработчикам в порядке зависимостей

| Шаг | Существующие требования/владельцы | Конкретный результат | Gate для следующего шага |
|---|---|---|---|
| 1. Согласовать meaning/authority | R05 + Windows FR-009/010/TH-01/03; Android AND-03/04 | Различимые intent, applied protection, path observation и recovery stage; applicable validity semantics | Источник/поколение/unknown и late-result contract определены; UI не authority |
| 2. Связать эпизод | COM-03/07/11/12; AND-05/07/11/12; R03 как потребитель | Связь существующих observations/attempts с task episode, bounded retention и denominator | A01…07/21 и privacy budget сформулированы, D/scope до измерений |
| 3. Сделать выполнимые действия | COM-01/04…10; AND-06/08/09/10/12 | Feasibility/explain, Settings/handoff, единые main/tray/tile/notification статусы | A08/09/14…19; device/platform admission и a11y отдельно |
| 4. Подготовить recoverability | COM-13/14/15, TH-03/07/08; AND-13/14/17 + infra/R05 | Один inventory prerequisite graph и допустимый independently accessible runbook/packet, без admin keys в клиенте | A09…13/20/22; future restore drill и source→native→path evidence |
| 5. Измерить пять сценариев | FR-012; AND-15/16/17, разрешённый QA/field | Предварительно заданные cohort/workload/D, S/F/U/coverage, tails и user comprehension | Сначала local/native/device, затем оплаченная разрешённая полевая инфраструктура |

Это адресные предложения научному руководителю, **не изменение PLAN статусов**,
не поручение другим чатам и не разрешение на выполнение. Новые numbered product
requirements не создаются. Не начинать дизайн-декор, ML или новый transport
вместо незакрытых protection/lifecycle/authority gates.

## 7. Предлагаемая вставка в общий сборник

Только после решения руководителя: дополнение к §12.8 и §13.10 со ссылками на
§16.4–6/§17; не переписывать исходный авторский материал. Возможный текст:

> Эксплуатационная приёмка относится к эпизоду пользовательской задачи:
> intent → применимая защита → актуальное наблюдение пути/цели → полезный исход.
> Причина BLOCKED не доказывает успешное применение barrier; RUNNING не доказывает
> текущую готовность задачи. Автоматический recovery, ожидание человека и состояние
> защиты показываются раздельно. Retry сохраняет эпизод, а новый UI sample не
> освежает старое доказательство. Unknown и потеря retained history остаются явными.
>
> Для заранее выбранной cohort/deadline показывать подтверждённые успехи, отказы,
> неизвестные исходы и coverage. Только успешные latency samples скрывают длинные
> незавершённые отказы. User cancel и сон учитываются по заранее названной методике;
> часы разных boot/OS domains не смешиваются. Численные SLO требуют baseline.
>
> Независимость транспорта дополняется замкнутостью восстановления: необходимый
> профиль, доверенное основание, ключ, executable, адрес и runbook должны быть
> достижимы в разрешённом порядке при названном отказе. Их допустимость должна
> сохраняться к моменту использования; cache не продлевает authority и не создаёт
> резервную серверную мощность. Warm failover и cold restore — разные сценарии.
>
> Portal/LAN/split/Stop действия зависят от полномочий ОС и effective policy.
> В просмотренном AOSP16 прямой запуск captive portal закрыт privileged API;
> обычному Android APK нужен доступный public Settings/handoff с повторным чтением
> фактов. Конфликт lockdown не лечится скрытым общим pause. Возврат из Settings
> не означает успешный вход или новый protected proof.

Первоисточники вставки: SOURCES E01/02/03/06…13/17…19. Это **проектный синтез**,
его научная/продуктовая приёмка остаётся открытой до рассмотрения и будущих проверок.

## 8. Ограничения и следующий исследовательский вопрос

- Исследованы документы и указанные source slices, не весь security/lifecycle
  контур; текущие изменения платформ могут сделать часть снимка устаревшей.
- На итоговой сверке 21 уникального local input выявлен drift Android
  `SystemScreen.kt` (навигационный/presentation вход); новая версия не reviewed.
  Ключевые source-факты §2.1 привязаны к неизменным на этой сверке файлам.
  Metadata и предел актуальности — SOURCES §3/STATE, не snapshot всей ОС.
- Нет provider API, платёжного доступа, живого сервера, handset inventory,
  экспериментов на российских операторах или наблюдений реального рабочего дня.
- Не доказаны causal diagnosis, автоматическое сохранение звонков, пригодность
  всех OEM, актуальная стоимость multi-country резерва и эффективность UX.
- Источники зрелых VPN показывают применимые приёмы и ограничения, не отсутствие
  аналогов. Число функций и стран не является критерием превосходства.
- Следующий срез R06: после замечаний руководителя детализировать один сценарий
  S2 либо S5 до согласованной платформенной последовательности и измерительной
  спецификации. Самостоятельный переход к продуктовой реализации не разрешён.

---

## 9. R06-002 — составной отказ и временная достижимость восстановления

**2026-10-25; READY_FOR_REVIEW.** Основание — DEPTH-CYCLE-002, углубление S5.
R06-001 выше сохранён; он не получил научного ACCEPTED. Его exact baseline
сохранён в ignored `cycle-002/baseline-R06-001-REPORT.md` и `…-SOURCES.md`.

### 9.1. Конкретная новизна и модель отказа

Первый цикл ввёл prerequisite closure и горизонт допустимости. THEORY §7.3/5,
§13.10, §16.6 и §17 уже обсуждают общий origin, offline authority, часы и
полезное completion. Здесь добавлены **операционные ограничения графа**:

1. У prerequisite есть **identity и стадия использования**, а не только expiry.
   Неистёкший kit может не соответствовать текущему checkpoint; retirement
   изменяет состояние и может уничтожить прежнюю возможность продолжать работу.
2. Различаются существование удобного расписания, исполнимость по фактически
   полученным наблюдениям и доказательство восстановления. Unknown duration/
   permission нельзя заменить конечной константой ради «гарантированного RTO».
3. Проверены hidden capabilities готового restore: repository lock, local
   decryptability, staging/application authority и explicit user confirmation.
   Один backup reader credential не замыкает всю цепочку.

**Incident F:** основной gateway A недоступен; issuer/publisher/control API
недоступны; администратор `O_admin` недоступен до конца рассматриваемого окна.
Kit K может быть неполным/старым. Отдельно задаются `U_local` (может ли человек
разблокировать устройство/подтвердить действие), сеть доступа, питание, живой B,
локальные keys/checkpoint и допустимый срок D. «Нет администратора» не означает
«никто не может нажать подтверждение»; для `U_local=false` это отдельный режим.

Цель G — **свежо подтверждённый полезный доступ к заданной задаче в заявленном
scope защиты**, не общий uptime и не обещание сохранить старое TCP-соединение.
Все cuts ниже относительны этому F, D и перечисленным допустимым действиям.
Unknown возможность нельзя объявлять ни доказанно доступной, ни физически невозможной.

### 9.2. Что source действительно позволяет: caller → gate → consumer

| Ветка | Просмотренная цепочка | Операционное следствие / предел |
|---|---|---|
| Windows replacement recovery | `ApplyProfileRecovery(confirm,targetSHA)` → `importProfileBundle`/`importPreparedProfilePayload` → `bundleAuthorityInput.prepare` → `PrepareProfileRecovery` либо `PrepareDeliveredRecovery` → `prepareProfileRecovery` | Explicit confirm и target из preview; enrollment recovery key заранее; exact epoch/version/envelope/trust source, target и validity. Это замена profile key из одного checkpoint, не универсальное восстановление всех authority |
| Windows позднее применение | В том же import: `sourceFence` → свежие policy/checkpoint/clock/approval/envelope/delivery/permit checks; повтор после `retireProfileSession`, staging и в `filetxn.BeforeCommit` | Preview не резервирует validity. Отказ после retirement не означает, что старый runtime всё ещё работает или гарантированно восстановлен |
| Windows прежний accepted runtime | `corebridge` → `render.LoadDescriptorSnapshot/applyProfileSnapshot` → `Store.LoadRuntimeProfile` → signer/version/clock floor/applied pin-policy/exact installed identity → render validity sampling | Existing accepted data plane не обязан получать новые delivery metadata. Но accepted cache не отменяет собственные profile/pin/clock restrictions и не создаёт B |
| Windows local clock | `ObservedClock.Now/Observe` + `TrustClockFloor`; callers в import/sourceFence и render | Это local wall/steady continuity и floor, **не authenticated UTC**, не доказанный upper bound времени после reboot |
| Windows сохранённые credentials | `secretvault.dpapiProtect/dpapiUnprotect` → DPAPI user scope | Копия ciphertext не доказанная переносимость на новый компьютер/аккаунт. Microsoft описывает обычную user/machine привязку и исключения roaming profile; абсолютный запрет всех migrations не выводится |
| Android | manifest `directBootAware=false`, `SUPPORTS_ALWAYS_ON=false`; `RuntimeSourceProvider` comment «no production provider»; `OsProtectionReader` → dated observation → `ProtectionState.from` | Нет принятого unattended boot/recovery path в текущем source. OS facts не protected READY; playbook ниже — целевой контракт после Android gates, не работающая сегодня функция |
| Server restore | `deploy-vps.sh` → substitution → scp в active path → remote validation; restart отдельный | Это не готовый unattended transactional restore: shared SSH/scp `-p`, upload до validation, нет автоматического backup/atomic apply/rollback; WS template8443 против NGINX9443. Существование скрипта не заполняет node «server restored» |

Pointers: [onboarding_import](../../../../windows/onboarding_import.go),
[onboarding_authority](../../../../windows/onboarding_authority.go),
[profile_recovery](../../../../windows/backend/metadata/profile_recovery.go),
[runtime_profile](../../../../windows/backend/metadata/runtime_profile.go),
[clock](../../../../windows/backend/metadata/clock.go),
[render snapshot](../../../../windows/backend/render/snapshot.go),
[corebridge](../../../../windows/corebridge.go),
[DPAPI source](../../../../windows/backend/secretvault/crypto_windows.go),
[Android manifest](../../../../android/vpn/src/main/AndroidManifest.xml),
[ConnectionState](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/ConnectionState.kt),
[OsProtection](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/OsProtection.kt),
[deploy source](../../../../configs/deploy-vps.sh). Ничего из этого не исполнялось.
DPAPI внешнее основание: [E26](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata).
Android storage/boot: [E27](https://developer.android.com/privacy-and-security/direct-boot).

**Новое ограничение kit coverage.** Для обычного recovery permit обозначим
source binding `q=(profile_epoch,version,envelope_digest,trust_digest)`.
Только соответствующий q, target, enrolled recovery authority и прочие gates
делают этот permit применимым; возраст файла и expiry недостаточны. Source
прямо отвергает replaced source и требует новый permit. Изменение только
наблюдаемого clock floor не объявляется автоматически сменой q: это отдельная
clock/freshness проверка. In-flight sourceFence строже — сравнивает подготовленные
policy/checkpoint bytes. Epoch recovery — другая разрешённая ветка с собственными
proofs, не wildcard этого permit. Нельзя делать rollback checkpoint ради совпадения K.

### 9.3. Temporal AND/OR модель с единицами и пределами

**Предлагаемая аналитическая модель; production schema/solver не создавались.**
Состояние содержит факты с identity/scope/provenance, их доступность и разрешённые
переходы. Для факта x хранится validity predicate `V_x(state,time,operation)`;
это может быть интервал времени, exact source binding или разрешение ОС.
Expired, known false и unknown различаются. Факт из cache не становится свежим
из-за нового чтения; observation не делает endpoint достижимым навсегда.

Действие a задаётся:

```text
pre_start(a), pre_commit(a), pre_during(a)  — AND; альтернативные способы — OR
actor(a) / permissions(a) / scope(a)      — кто и что вправе исполнить
duration(a) ∈ [l_a,u_a] секунд            — только если такой bound обоснован
effects+(a), effects−(a)                  — выдаёт факты и отзывает/потребляет старые
receipt(a), failure(a), compensation(a)   — наблюдение результата и допустимое восстановление
```

Timepoints `s_a,f_a` в одном named monotonic/incident domain: `l_a≤f_a−s_a≤u_a`;
предшественник i требует `f_i≤s_a`; deadline задачи `f_G≤D`. Два lifecycle
действия одного owner не считаются параллельными. Для операций с signed wall
validity нужна **отдельная** обоснованная связь с UTC; monotonic after-boot не UTC.

В идеализированном случае истинное UTC в момент τ лежит в `[L(τ),U(τ)]`.
Для обязательного gate `[n_x,e_x)` robust admission в τ требует
`L(τ)≥n_x` и `U(τ)<e_x`; equality на expiry не проходит. Policy skew/роль/исключения
определяет R05, это модель для действительно обязательного gate, не изменение
Validate semantics. Если неизвестен конечный U после reboot, строгую проверку
upper expiry из одного retained floor обосновать нельзя. Existing ObservedClock
не выдаётся за реализацию такого UTC interval oracle.

Для **выбранной serial ветки** с обоснованными `u_i` и обязательным commit gate e,
при bounded UTC uncertainty δ секунд достаточная временная проверка имеет вид
`s_k + Σ(i=k..commit) u_i + δ < e`, одновременно с task deadline и всеми
other start/during gates. `latestStart_k < e−δ−Σu_i`; если earliestStart_k
уже не меньше правой границы, ветка не помещается. Это algebraic bound,
не измеренный RTO. Если ожидание пользователя/сети имеет `u=∞`/unknown,
конечной worst-case гарантии для этой ветки нет.

Здесь e — UTC deadline, перенесённый относительно **того же incident anchor**,
что s_k; δ ограничивает ошибку этой clock projection, включая drift до commit.
Нельзя напрямую складывать monotonic uptime и абсолютный Unix timestamp или
дважды вычитать уже учтённую uncertainty. Без такой связи численный bound unknown.

На AND join по-настоящему параллельных действий earliest completion задаётся
максимумом доступных predecessor finishes, не суммой. Но один owner/storage
и несовместимые effects добавляют serialization. OR выбирается только по уже
доступной информации; «выберем ветку, которая впоследствии оказалась успешной»
не implementable strategy. Разные expiry на start/commit/during нельзя свести
к одному TTL файла. Retirement — отрицательный effect, поэтому простого
монотонного reachability по накопленным фактам недостаточно.

STN/STP дают интервальную временную consistency; это не полная модель знания,
authority или неопределённых длительностей. [Dechter/Meiri/Pearl, E20](https://www.sciencedirect.com/science/article/pii/0004370291900066).
Contingent temporal planning отдельно рассматривает реакцию только на уже
полученные наблюдения. [Vidal, AIPS2000, E21](https://cdn.aaai.org/AIPS/2000/AIPS00-044.pdf).
Для данного playbook взят принцип явных ограничений и неупреждающих решений,
**не** готовая гарантия dynamic controllability произвольной hostile сети.
Solver/модель не выполнялись; ручные traces ниже — условные рассуждения.

**Два времени TUF:** spec1.0.37 фиксирует update-start time для workflow,
а trusted root предполагает out-of-band. Это не новое право принять наш profile
после его application deadline и не источник authenticated UTC. Старый expired
root может участвовать в root update chain; нельзя объявлять любую root expiry
универсальным cut без учёта конкретной ветки. [E22](https://theupdateframework.github.io/specification/v1.0.37/index.html).

### 9.4. Один граф prerequisites: варианты и отрицательные effects

```text
G = AppliedProtection(scope,generation) AND CurrentPath(B) AND TaskResult(scope)
CurrentPath(B) <- Execute(one owner, eligible B) -> fresh path receipt
eligible B = compatible runtime AND permission AND usable credential
             AND applicable trust/profile/pins AND valid locator/TLS identity
locator(B) = accepted locator/IP binding OR successful permitted DNS resolution
profile(B) = accepted matching state OR admitted K OR new authorised enrollment
admitted K = accessible K AND decrypt capability AND matching source/target
             AND authority/time checks AND explicit approval when required
new enrollment = independent authorised inputs AND actor AND platform admission
fresh new issuance = reachable issuer/custodian AND their authority AND actor
cold server restore = capacity AND tool/config parity AND server secret custody
                      AND admin/delegated action AND native/post-path acceptance
```

`accepted locator/IP binding` не означает произвольный raw IP в HTTPS:
serverName/certificate/pin и route identity сохраняются по signed descriptor
и поддерживаемому transport API. Если DNS нужен для TLS donor, issuer, time,
artifact или иного обязательного шага, IP одного gateway их не устраняет.
Bootstrap direct exception возможен только в уже допустимом scope — не общий bypass.

Наличие `paid_until(B)` — provenance о договорном сроке, не egress receipt.
Оплаченный B может быть неисправен; выключенный B не чинится файлом K.
`operator отсутствует` блокирует новую manual issuance/provisioning, но не
заранее provisioned/accepted B. `U_local=false` отдельно блокирует confirmation,
unlock или новый OS consent. Автоматическое списание денег — не доказанный
самовосстанавливающийся путь при блокировке платёжного аккаунта.

### 9.5. Cuts: где именно прекращается достижимость

| Cut | Явное условие в этом action set | Почему нет разрешённой последовательности / честный исход |
|---|---|---|
| C1 capacity | A выключен; нет живого B; admin/provisioning отсутствуют | Нет действия, производящего compute/gateway. «Все подготовленные пути недоступны»; kit не сервер |
| C2 source identity | K только для q0; current q1; нет другой applicable authority/enrollment | Import не может произвести accepted q2 из этого K; нужны новые допустимые inputs. Это семантическая устарелость до expiry |
| C3 custody | Нет usable device credential/decrypt key, а доступная копия только ciphertext | Нет разрешённого decrypt/use перехода. «Нужен доступ к ключу/устройству»; ciphertext integrity не decryptability |
| C4 temporal | Все применимые ветки истекают до достижимого start/commit; renewal недоступен | Условный temporal cut по заданным bounds. Если bounds лишь unknown, это отсутствие гарантии, а не доказанный физический FAIL |
| C5 permissions/actor | Требуется новое consent/confirm/unlock/admin action, соответствующего actor нет | Кнопка/notification не создаёт полномочие. Ожидание человека либо заранее admitted warm branch |
| C6 bootstrap closure | Все способы получить locator/profile/time/tool требуют тот же недоступный VPN | Замкнутый dependency cycle без доступного initial input. Нужен действительно независимый input, не ещё один URL на A |
| C7 knowledge | Требуется доказать globally latest revocation/time, но доступных независимых сведений нет | Unknown/требование нового основания; нельзя объявить свежесть из старого cache |
| C8 observation | Data path потенциально работает, но требуемый receipt/observer недоступен | Недостижима **подтверждённая G**, не доказан физический отказ трафика. UI «проверка недоступна» |
| C9 current Android | Нет production accepted-source provider/approved APK; boot branch выключена manifest | Этот source snapshot не реализует заявленный playbook; future AND gates обязательны |

**Условное доказательство cut:** если любой разрешённый producer G требует
хотя бы одного элемента C, все элементы C отсутствуют/недопустимы до D, а ни
одно разрешённое действие из доступных inputs их не производит, конечного
плана до G нет. Это структурный вывод по закрытой перечисленной модели.
Неизвестные внешние способы/новая сеть/администратор меняют предпосылки;
после такого изменения прежний cut пересматривается.

**Непреодолимая граница свежести, собственный вывод.** Мир W0 и W1 дают клиенту
одинаковые local bytes/observations; только в W1 после последнего контакта был
отзыв. Без нового независимого сигнала клиент не различит миры. Если требуется
немедленно соблюдать любой глобальный отзыв и одновременно всегда продолжать
offline доступ, одно решение не выполнит оба требования. Это не ссылка на CAP
и не доказательство небезопасности любого offline cache. Policy может разрешить
ограниченный offline window: тогда нельзя рекламировать instantaneous global
revocation. Freshness contract и допустимый compromise задаёт R05/владелец.

## 10. R06-002 — единый recovery playbook

Это последовательность будущего поведения, **не инструкция запустить команды**.
Стартовая развилка — применимый warm path; cold restore не скрывается под тем
же обещанием «автовосстановление». В каждом шаге сохраняется исходный episode.

| Шаг | Inputs / доступное действие | Проверяемый результат и failure outcome | Платформенная граница / сообщение |
|---|---|---|---|
| P0 Зафиксировать incident | Current intent, факты path failure/unknown, generation, D; удержать/проверить существующий guard через owner | Отдельно причина отказа и observed barrier; не приписывать таймауту billing/DPI | ПК: proxy/TUN scope. Android: dated OS facts, не egress proof. «Путь недоступен; состояние защиты …» |
| P1 Проверить локальную возможность | Runtime/version, accepted head, usable device key, OS permission/lock/unlock; никакого массового чтения секретов в diagnostics | Missing/unknown/denied перечисляются кодами; cached profile не fresh-installed trust | Android current provider отсутствует → C9; после reboot до unlock — C5/C3. «Нужно разблокировать/дать разрешение» только когда это действительно следующий шаг |
| P2 Выбрать уже admitted B | B заранее provisioned, credential server-side настроен, accepted profile содержит B, locator/TLS допустимы и доступна physical сеть | Один owner делает bounded candidate/path check; успех создаёт receipt текущего generation; timeout оставляет unknown cause | Windows ordinary retry не требует новой profile-key recovery. Android будущий accepted provider+consent+protection. «Проверяем подготовленный резерв» |
| P3 Если B не eligible, оценить K | K доступен и decryptable; сверить exact q/target, roles, validity и late-apply window; preview без mutation | Строго distinguish: absent bytes, stale q, expiry, clock uncertainty, несовместимый target. Нет попыток «перебирать transports» против source-binding cut | Windows explicit preview/confirm API; Android production trust driver ещё gate. «Этот пакет подготовлен для другой версии профиля» |
| P4 Получить нужное согласие | U_local доступен; подтверждены exact target/scope; ОС может показать своё consent | Отказ/нет человека → ACTION_REQUIRED, не successful recovery; отсутствие O_admin само по себе не запрещает уже pre-issued K | Если U_local=false, allowed только заранее admitted auto branch. Не нажимать системные диалоги вместо пользователя |
| P5 Apply с поздним fence | Owner сериализует переход, retirement при необходимости, stage/commit и повтор применимых source/time/approval gates | Failed commit после retirement: «пакет не применён; путь остановлен/защита … по фактам». Старый active path не предполагается | Только поддерживаемый платформенный transaction/retirement adapter; file restore не его замена. Compensation допускается лишь с текущей допустимостью старого состояния |
| P6 Cold server restore — отдельная ветка | Нужны live capacity, operator или заранее authorised restoration service, toolchain/config parity, separate server secret custody, действия уровня host | Без actor/capacity — C1/C5. При наличии — validation/staging/application/post-path gates, не один process start | Сегодня deploy script не закрывает эту ветку; отсутствующий администратор не появится от UI retry. «Для этого восстановления нужен администратор» |
| P7 Подтвердить задачу | Fresh applied guard/path identity, выбранный service/task scope, бюджеты | Path success отдельно от task unknown/account/media; G только в оговорённом scope, late completion не исправляет timely outcome | Нельзя писать «звонок восстановлен» по одному HTTP endpoint; «Защищённый путь проверен; сервис ещё не проверен» допустимо |
| P8 Завершить или ожидать изменения prerequisites | Подтверждённая G, typed cut либо unknown/budget exhausted; safe bounded timeline/export локально | Смена сети/новый допустимый пакет/возврат actor открывают новый анализ той же задачи; retry сам не создаёт authority | «Ждём …» с одним выполнимым действием. При отсутствии такого действия объяснить ограничение, не бесконечный spinner |

**Что не участвует в автоматическом восстановлении:** повышение trust из
скачанного K, установка произвольного runtime, отключение TLS проверки,
новое продление expiry, provider admin key в клиенте, автоматический платёж,
замена корпоративного VPN и общий direct fallback. Это не необходимые
предпосылки описанного допустимого warm path.

### 10.1. Минимальное описание kit applicability

Reuse existing profile/recovery/public context, не новый обязательный wire format.
Нужно уметь предъявить: какую операцию K разрешает, source q/target identity,
применимые intervals/roles, необходимые artifact/tool versions, носитель и
custody requirement, зависимые endpoints/DNS, актор/permissions и ограничения.
Публичный inventory содержит references/status, а не ключи/пароли/полный профиль.
Показывать отдельно **наличие bytes**, **применимость**, **доступность ключа**,
**подготовленность сервера** и **дату последней практической приёмки**.

Один matching permit не покрывает автоматически будущие q1/q2. Набор
pre-issued permits конечен и покрывает только перечисленные входы/targets;
нельзя построить unlimited autonomous recovery, просто положив больше старых
файлов в архив. Готовить refresh K следует по authority-relevant transitions
и оставшемуся окну, а не только раз в месяц. Правила issuance/custody/retention
принимает R05; private signing key не копируется в обычный клиент ради удобства.

## 11. R06-002 — ручные traces и отрицательные примеры

Числа T1/T2 ниже **иллюстративные секунды**, не измерения, не defaults и не
выполненная симуляция. Предполагаются действующие права/обоснованный time anchor,
bounded δ=2s и перечисленные upper durations; в реальном проекте они неизвестны.

| Trace | Премисы и ручной ход | Вывод / что способно его опровергнуть |
|---|---|---|
| T1 Warm без issuer/DNS A | Уже accepted B имеет достаточный locator/TLS/credential, physical сеть достижима, guard/permissions применимы; local admission5s → single-owner path25s → task5s. Всего≤35s; applicable expiry120s, D60s: `35+2<120`, `35≤60` | Условно возможна G без O_admin и без нового import. Любой обязательный donor/DNS/time dependency вне initial inputs либо неограниченный network duration снимает этот bound |
| T2 Matching K с человеком | q0 совпадает; permit до90s, target до110s, D80s; read/preview до15s, явный confirm до18s, retirement≤15s, commit≤5s → commit≤38s; path≤20s+task≤5s →≤63s. `38+2<90`, `63+2<110`, `63≤80` | Есть условный порядок P3→P5→P7; preview не его последний gate. Поменявшийся q/approval/clock/sourceFence или настоящий unbounded stage опровергает применимость |
| T3 Поздний commit | Preview в20s, applicable permit expiry50s; retirement/staging доводят commit до54s | `54+δ<50` ложно; fresh fence должен отклонить. Нельзя считать старый path живым после retirement и нельзя продлевать permit |
| T4 Семантически старый K | Permit ещё действителен час, но FromVersion/envelope/trust/epoch не соответствует q1; O_admin отсутствует, других authority inputs нет | C2 именно для recovery-permit ветки. Ordinary accepted B или законная другая enrollment ветка могут обходить этот cut, если их prerequisites действительно есть |
| T5 «Есть backup», но нет custody/permission | Репозиторий цел; единственный decrypt capability потерян либо DPAPI ciphertext перенесён в неподдержанный user/machine context; либо default restic lock запрещён backend permissions | Restore bytes/permission cut; не «сломанный transport». Новый корректный ключ/поддержанная migration/разрешённый immutable restore путь меняют премисы, сами ciphertext не помогают |
| T6 Оба недоступны | A выключен; B отсутствует/тоже выключен; backup цел, operator/provisioning нет | C1 до изменения среды. Никакой конечный перебор портов не производит работающий gateway |
| T7 Два мира revocation | W0/W1 одинаково наблюдаемы offline; в W1 есть не доставленный отзыв | Нельзя гарантировать одновременно global freshness и continued availability в обоих мирах. Явная offline policy меняет требование, не информацию |
| T8 Android reboot без человека | U_local=false; current manifest не direct-boot-aware, production provider отсутствует; kit лежит в CE/нужен новый consent | Current C9; даже будущий direct-boot adapter не получает CE/consent автоматически. Простая блокировка экрана **после** первого unlock не равна CE locked-before-first-unlock |

Отдельная техническая коллизия: восстановить q0 из старого snapshot ради
применения K означает менять trust history/floors, а не «чинить файл».
Допустимость legitimate device disaster recovery и сохранность antirollback
состояния — вопрос R05. Restic integrity не выбирает, какой checkpoint законен.

## 12. R06-002 — минимальный резерв и стоимость

### 12.1. Минимальность только относительно выбранного отказа

Для F = «A и его control plane потеряны, оператор отсутствует, клиент и одна
physical сеть работают» нужен как минимум **один уже работающий альтернативный
gateway B**, если G требует remote egress. Файл backup не заменяет эту единицу
мощности. Один B условно достаточен только вместе с accepted authority, usable
credentials, permitted locator/bootstrap, compatible runtime/permissions и
текущим path/task подтверждением. Это нижняя граница модели, не доказательство
независимости реального неизвестного провайдера/маршрута.

Минимальный preparation set:

1. A и B не разделяют **тот отказ, который хотим переживать**: host/origin,
   при provider-account F — разные административные failure domains; при ASN/
   prefix F — соответствующее разнообразие отдельно. Разные страны недостаточны.
2. На клиенте заранее admitted B descriptor/credential/TLS identity и совместимые
   profile/pins; возможность Start не зависит от немедленного обновления issuer.
3. Доступный локально runbook/public applicability inventory; отдельно usable
   device credential custody. Для device-loss F нужен другой подготовленный
   способ recover/enroll, одного локального cache недостаточно.
4. Известные provenance/остаточные windows и план обслуживания до их исчерпания.
   Shared payment instrument/аккаунт и simultaneous annual expiry — общие риски.
   Prepayment уменьшает зависимость от ближайшего renewal, не исключает suspend.

Для **cold restore после потери обоих gateways** этот минимум уже недостаточен:
нужны capacity и authorised executor/оператор, secret custody и server parity.
Для **отказа physical access ISP** два зарубежных узла могут быть одинаково
недоступны: нужна другая доступная сеть/механизм, а не третья география.
Universal minimal independent reserve без заданного F и фактического inventory
не выводится. Ни одного реально подготовленного B это исследование не подтверждает.

### 12.2. Параметрическая денежная модель, руб/год

Пусть `p_A,p_B` — **годовые полные цены** в валютах счетов, `x_A,x_B` — руб/ед.
валюты, с заранее выбранным FX сценарием; налоги включены в p либо считаются
отдельно. Все остальные C ниже в руб/год:

```text
C_total = x_A*p_A + x_B*p_B + C_storage + C_domain
          + C_transfer + C_fees + C_other
C_plan  = C_total + R_cash ≤ 12000
```

`R_cash≥0` — оставляемый денежный запас, **не** ещё один сервер и не уже
понесённый расход. C_transfer включает backup/restore egress/overage; C_other —
только явно нужные дополнительные услуги. Труд администратора/обновление kit
учитываются отдельно часами, не объявляются бесплатными из-за отсутствия счёта.
Cash due при покупке и annualized цена многолетнего контракта — разные величины:
ограничение расходов первого года проверяется отдельно, скидка не обещается на renewal.

Если A/B стоят одинаково в рублях, годовой предел на каждый:
`p_node≤(12000−C_fixed−R_cash)/2`, где C_fixed — все остальные C в той же валюте.
Чистая алгебра: при условном `C_fixed+R_cash=1200` остаётся≤5400 руб/год на узел,
то есть≤450 руб/месяц. Это **не найденный тариф**, не наличие IPv4/трафика/
независимости/приёма карты. Налог/FX/overage могут сделать конкретное предложение
непроходящим; актуальные цены здесь не запрашивались и поставщик не выбирался.

Один A + encrypted offsite backup обычно представляет **другой service contract**:
восстановление с actor/capacity и большим неизвестным RTO. Если два подготовленных
узла не помещаются в фактический бюджет, нельзя называть cold backup автоматическим
резервом без оператора; нужно сузить fault/SLO обещание либо изменить бюджет.

## 13. R06-002 — готовые подходы: что взять, что они не закрывают

| Кандидат | Version / API / licence | Решение и предел |
|---|---|---|
| Existing Snowden accepted runtime + recovery/TUF | Windows go-tuf/v2.4.2 уже в go.mod; существующие exact APIs §9.2, Android seam pending. Новая зависимость не предлагается | **Reuse** authority workflow, public context/preview, late fences и one owner. Не выдавать adapter source за native durability и не заменять original signed bytes сериализацией inventory |
| Restic CLI для server/operator backup | **v0.19.1**, commit `6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf`, BSD-2-Clause; go1.25.8/toolchain1.25.10. CLI не публичный embedded SDK, `internal/*` не API приложения | **Candidate reuse** как отдельный инструмент server/operator, не в APK/packet path. Pin/version/licence просмотрены; binary provenance, поддержка OS, storage backend и restore gate ещё не приняты |
| Existing encrypted onboarding/export | Нынешние scoped profile APIs, отдельный platform adapter; не generic directory restore | **Reuse** для предназначенного переноса профиля с trust/recipient/target gates. Не считать перенос DPAPI ciphertext кроссплатформенным export |
| Generic VM/provider snapshot | Provider/account/format/permission конкретного предложения неизвестны | **Reference/conditional**, не считать independent reserve: тот же отказ account/storage может удалить original и snapshot; restore требует control plane/compute/actor |
| STN/contingent temporal planning | E20/E21 — первичные публикации, не выбранный library/API/license для production | **Reference** interval constraints/non-anticipation, ручной небольшой playbook. Solver dependency/deployment не нужен; uncertainty/authority effects не исчезают от наличия scheduling algorithm |

### 13.1. Restic caller trace: не абстрактное «backup работает»

В exact pin E23 просмотрена цепочка:

```text
newRestoreCommand.RunE → runRestore
  → openWithReadLock → internalOpenWithLocked
    → global.OpenRepository → decryptRepository → Repository.SearchKey
    → repository.Lock → restic.NewLock → createLock → SaveJSONUnpacked(LockFile)
  → snapshot selection → restorer.NewRestorer → RestoreTo
  → VerifyFiles только при соответствующем opts.Verify
  → unlock/remove lock metadata
```

Источники: [cmd_restore](https://github.com/restic/restic/blob/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf/cmd/restic/cmd_restore.go),
[command lock](https://github.com/restic/restic/blob/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf/cmd/restic/lock.go),
[global.OpenRepository](https://github.com/restic/restic/blob/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf/internal/global/global.go),
[repository lock](https://github.com/restic/restic/blob/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf/internal/repository/lock.go),
[lock writes](https://github.com/restic/restic/blob/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf/internal/restic/lock.go).

**Операционное следствие:** обычный restore имеет repository-metadata write/
remove capability через locks. Обещание «достаточно read-only backup token»
не следует из слова restore. Альтернативный no-lock режим существует, но его
применимость требует отдельного безопасного storage/concurrent-writer contract;
он не рекомендован как универсальный shortcut. Immutable offline copy с
проверенной последовательностью восстановления — отдельная подготовленная ветка.

Это не новая security vulnerability Restic и не основание выдавать клиенту
широкий delete credential. Нужен правильный least-privilege backend contract,
а actual workflow проверяется на изолированном стенде лишь после разрешения.

Документы E24 предупреждают о partial in-place restore; следовательно байты
восстанавливаются в отдельную staging область, а применение приложения/доверия
остаётся у его transaction owner. E25 различает structure check и чтение pack
data; даже полная data integrity не подтверждает актуальность credential,
включение нужных файлов или пригодность server/client state.
[E24](https://restic.readthedocs.io/en/v0.19.1/050_restore.html),
[E25](https://restic.readthedocs.io/en/stable/045_working_with_repos.html).
Ни restore/check, ни decrypt/sign/crypto operation здесь не выполнялись.

## 14. R06-002 — самостоятельное оппонирование

| Сильное возражение | Что пересмотрено / граница вывода | Как опровергнуть предлагаемую применимость |
|---|---|---|
| Граф с придуманными duration bounds красиво доказывает несуществующий RTO | Конечные u/UTC bounds считаются предпосылками; сеть/человек/payment могут быть unbounded. T1/T2 иллюстрации, не результаты | Показать mandatory stage без принятого finite bound — robust RTO для этой ветки снят |
| Missing receipt может означать только поломку detector | Выделен C8: недостижимость confirmed G не физический отказ data plane | Независимо наблюдать работающую задачу и broken observer; диагноз меняется, safety gates остаются |
| Ты объявил один stale permit запретом всякого recovery | C2 сужен до exact permit branch; accepted B/epoch/new enrollment рассматриваются отдельно со своими inputs | Предъявить уже разрешённую альтернативную ветку, не требующую отсутствующего issuer |
| Любое изменение checkpoint якобы ломает K | q ограничен релевантными source fields; clock floor и in-flight full byte fence разведены | Clock-only update может оставить permit source match; нельзя объявлять его универсально stale |
| Кэш безопасно живёт до явно выбранного offline expiry | Принято: freshness impossibility относится к одновременному **instant global revoke** требованию. Bounded offline policy не опровергается | Показать согласованный policy contract с допустимой задержкой отзывов; это другая спецификация, не magical freshness |
| Два узла уже достаточно, а metadata-граф избыточен | Для одного host failure достаточно заранее usable B; inventory может быть коротким. Тяжёлый CMDB/control plane не нужен | Если все дальнейшие действия действительно лишены publisher/DNS/time/custody dependencies, сократить ветку; не добавлять их ради схемы |
| Restic умеет альтернативные режимы и read-only restore не невозможен | Речь о просмотренном default caller, не обо всех конфигурациях. No-lock/offline copy требует доказанного контракта | Показать конкретный pinned безопасный workflow/backend permission matrix; расширить OR ветку, не отключать locks вслепую |
| Android CE доступен после повторной блокировки экрана | Согласовано: cut относится к before-first-unlock после reboot и current manifest/provider, не любому lockscreen | Device evidence другого состояния/новый accepted boot adapter меняют предпосылки |
| Резерв в другом ASN всё равно делит censor/payment/доступ ISP | Независимость заявляется только по выбранному F и имеющемуся provenance; market availability не установлена | Inventory показывает общий обязательный отказ — резерв не покрывает соответствующий F |

**Сильнейшее ограничение всего пакета:** без платёжного/host inventory,
device evidence, usable authority/custody и обоснованных time/duration bounds
можно доказать условный порядок/cut, но нельзя честно установить конечный
RTO или вероятность успеха Snowden. Отсутствующий оператор — реальная внешняя
зависимость; новый алгоритм не устраняет необходимость заранее выданных полномочий.

## 15. R06-002 — конечные будущие проверки и передача

Все проверки — **не выполнены**. Сейчас разрешены только ручной source/doc
review и derivation; нет tests/fixtures/build/vet/doctor/crypto/simulation/
bench/native/device/user/field/live/admin/install/deploy или OS-network changes.

| ID | Falsification / future case | Конечный ожидаемый критерий |
|---|---|---|
| B01 | У warm B принудительно отсутствует mandatory DNS/donor/bootstrap input | Не объявлять offline-independent warm branch complete; показать конкретный prerequisite |
| B02 | Matching permit против заменённого version/envelope/trust/epoch | Корректный branch допускается только по своему контракту; source mismatch отклоняется до успешного Apply |
| B03 | Clock-floor-only change и полный in-flight byte change | Не путать q coverage и sourceFence; никакого произвольного reset |
| B04 | Permit валиден в preview, истёк после retirement/staging | Late fence отклоняет; UI не утверждает old runtime alive или guard applied без факта |
| B05 | Unknown UTC after reboot против known local floor | Не выдумывается finite UTC upper bound; expired authority не продлевается настройкой часов |
| B06 | Новая profile recovery требует confirm при U_local=false | ACTION_REQUIRED/C5 либо другая заранее admitted ветка; нет автоматического подтверждения |
| B07 | Ciphertext цел на неподдержанном DPAPI user/machine | Restore integrity не обозначается usable device credential; нужен поддержанный recovery путь |
| B08 | Restic default restore с backend read/list без lock write/delete | Перечень операций совпадает с фактическими receipts/permissions; не импортируется широкий admin token |
| B09 | Interrupted in-place restore / staged restore | Не используется частично восстановленная application/trust state; staged bytes проходят application-specific gates |
| B10 | Android reboot до unlock/после первого unlock, consent revoked | Разные OS outcomes, без ложного boot/always-on READY; current missing provider остаётся gate |
| B11 | Два gateway sharing issuer/account/payment и независимый B | Failure matrix показывает реальное покрытие, unknown cells не зачисляются как independent |
| B12 | Live B с expired authority и dead B с валидным K | Authority и capacity не подменяют друг друга; оба неподходящих пути не дают G |
| B13 | Одновременно terminal/superseded/late observations при recovery | Generation/episode сохраняют правильный исход; stale receipt не подтверждает новую задачу |
| B14 | Budget при FX/fees/traffic/renewal выше исходного сценария | Пересчитаны годовая сумма и cash due; превышение не скрыто annualization/«бесплатным» storage |

Следующая реальная приёмка потребует заранее выбрать F, D, actor availability,
devices/OS, allowed bootstrap, current inventory и workload. Измерять полный
episode, required interventions, S/F/U/coverage и actual stage/restore timings;
расходы — реальные счета/квоты отдельно. No failed sample deletion.

**Адресная последовательность предложений:** R05 — q/authority/clock/custody и
поздние application gates; Windows FR-009…011/TH-03/COM-13…15 и Android
AND-03/04/13/14/17 — соответствующие adapters; infra — prepared B/restore parity;
R03 — bounded decision на доступных prerequisites, R04 — реальные stage/resource
bounds. Это вопросы/предложения для руководителя, не сообщения другим чатам,
реализация или изменение статусов PLAN.

Предлагаемая дельта общей теории после review: recovery graph должен учитывать
exact source identity, actor/permission, необратимые effects и stage-specific
windows; source/temporal/knowledge cut не смешиваются с физическим отказом.
Минимальный no-operator резерв — заранее admitted живой путь с доступными
prerequisites относительно названного F. Archive integrity не равно authority,
decryptability, atomic application или подтверждённой G. Подробности и ready
reuse — этот R06-002; старый материал R06-001 не заменяется.

Итоговый drift review: PLAN и Android OsProtection изменились во время работы;
COM-13…15 и nullable OS observation admission/caller/consumer перечитаны узко.
Поздний drift Android STATE/ANDROID-PLAN просмотрен только как status header
(AND-11-S3 source unverified). Отсутствующий исторический NotificationPresentation
помечен в SOURCES, не выдан за current file. Baseline копии R06-001 сохранены;
новые функции платформ не объявлены принятыми. Exact metadata — ignored
cycle-002/inputs-*.json и review-ready.json; актуальность после этих снимков
не гарантируется.

## 16. R06-003 — рабочая сессия при смене пути: предмет и новизна

2026-10-26. **READY_FOR_REVIEW**, desk research/source review. Обычная смена
селектора не обещает сохранение каждого TCP/UDP потока — это **уже записано**
в COM-09/17 и теории. Новый результат здесь — exact caller semantics текущего
клиента, различимые области результата и контрпример сильному cutoff-обещанию.
Алгоритм выбора R03, packet tuning R04 и authority R05 не переопределяются.

Сценарий S2: пользователь ведёт звонок/SSH/загрузку/рабочую операцию; меняются
Wi-Fi/мобильная сеть, канал или gateway. Цели различаются: продолжить задачу;
сменить выход для новых соединений; немедленно перестать пользоваться прежним
выходом. Успех одной цели не является успехом остальных.

Модель результата перехода `C=(P,D,O,A,X)`:

| Ось | Что требуется установить | Чего не доказывает соседний факт |
|---|---|---|
| P — protection | Applied safety/authority для фактических путей и поколения | Живой engine или новый probe не подтверждает все OS safety gates |
| D — default | Выбранный маршрут для последующих обращений к селектору | Не весь ранее начатый dial/поток уже использует D |
| O — old flows | Судьба ранее допущенных соединений/ассоциаций | Не видна из одного activeChannelId; без наблюдения остаётся unknown |
| A — task | Продолжение звонка/терминала/операции с её собственными semantics | Успех синтетического probe не подтверждает task acknowledgement |
| X — observation | Какие из P/D/O/A свежо наблюдаемы данным owner/epoch | Отсутствие наблюдения не равно отсутствию трафика/отказу задачи |

Это прикладное уточнение R06-01, **не новый runtime protocol**. Не предлагается
собирать адреса всех пользовательских потоков ради заполнения O. Можно честно
дать ограниченную capability statement и оставить O/A неизвестными.

## 17. R06-003 — source chain и два контрпримера

### 17.1. Реальные entrypoints и существующие safety constraints

Windows `App.SelectChannel` → `switchChannelCoreFor` → render/prepare →
`selectorOnlyChange` → `Manager.SwitchChannel` → `Engine.SwitchDefaultContext`
→ worker switch/in-process `Selector.SelectOutbound`. Далее protected probe,
owner/generation publication либо подтверждённый rollback/cleanup. Если selector
недоступен — `Manager.Reload`; `Engine.Reload` сначала закрывает старый box,
затем запускает кандидат; возможный restart прежних raw bytes **не восстанавливает
старые сокеты или уже выполненную сервером операцию**.

В `windows/failover.go` уже явно учтены selector-lived streams:
`cc.Validity.Constrain(oldCC.Validity)` и такой же rollback constraint.
Только fresh worker снимает старое ограничение. Поэтому «старые deadlines
забыты при switch» здесь **не обнаруженный дефект**. Проверка всех фактических
expiry/OS путей остаётся платформенной приёмкой; source presence не PASS.

Renderer создаёт selector без `InterruptExistConnections`; у local bool
`omitempty`, то есть false. `windows/go.mod` закрепляет sing-box **v1.13.19**.
Release указывает commit **`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`**.
Шесть файлов существующего module cache побайтно через SHA256 сопоставлены с
public raw этого commit. Это provenance текста, не Go/module/build/runtime check.
[E31 source](https://github.com/SagerNet/sing-box/tree/b5ebaa1fc0f2b94256180b95468e73ef53caa27d).

Pinned `SelectOutbound` меняет `selected`, затем вызывает interrupt group.
`DialContext`/`ListenPacket` сначала получают selected outbound, затем добавляют
полученный conn в group. При false group закрывает internal entries, а external
оставляет; inbound handling маркирует external context, также делегируя выбранному
outbound handler. **Не утверждаем, что каждый transport/route проходит одинаковую
обёртку**: direct-route и специализированные handlers требуют собственной проверки.
[Selector docs](https://sing-box.sagernet.org/configuration/outbound/selector/)
согласуются с различием inbound/internal; фактический прямой Go caller важнее
ограниченного описания управления через Clash API в живой документации.

Комментарий `Manager.SwitchChannel` «соединения не рвутся» шире доказанного
свойства: отсутствие Stop/Reload не исключает internal interruption, сетевой
отказ и разрыв приложения. Это source/documentation discrepancy для владельца
Windows, **не установленная полевая регрессия**. UI contract `reloading` также
описывает Reload+probe, хотя caller может использовать selector-only ветку.

### 17.2. Ручные traces, не эксперименты

**T9 — смешанный выход после успешного switch/rollback.** External flow fA
уже открыт через A; selected становится B, новый probe B проходит, fA может
оставаться A. В другой ветке probe B не проходит, selected возвращён A и
rollback probe A проходит; flow fB, успевший возникнуть через B, не обязан
переехать в A. Следовательно `default=A/B` и `verified rollback` описывают
соответствующий default/probe, а не полный обратимый сетевой transaction.
Source validity constraints выше учитывают сохранение транспортов; смешанный
защищённый выход **сам по себе не direct leak**, но может нарушить ожидание
немедленной смены страны/адреса. Доступность fA/fB зависит от transport/сети.

**T10 — даже true не универсальный drain fence.** Допустимое чередование
просмотренного generic dial path: (1) dial загрузил A, ещё не вернул conn;
(2) selector swap на B; (3) Interrupt(true) обошёл уже зарегистрированные entries;
(4) dial A вернул conn, NewConn зарегистрировал его после обхода. Между выбором
outbound и registration нет общего cutoff lock этого selector/group. Значит
из одного `interrupt_exist_connections=true` **не следует** отсутствие всех
старых/in-flight соединений после возврата switch. Если конкретный outbound
отменяет dial другим способом, этот trace ему неприменим; live воспроизводимость
не установлена. Manager сериализует lifecycle actions, а не все application dials.

Вывод — не включать true как «готовое мгновенное удаление старого выхода».
Строгому cutoff нужен owner-defined retirement/admission fence всех relevant
путей, проверка завершения и safety; текущий close/reload кандидат требует своих
OS/cleanup доказательств. Идти в direct при этом нельзя. Рефакторинг concurrency
или новый drain API этим исследованием не реализуется.

## 18. R06-003 — границы готовых механизмов непрерывности

| Механизм / reuse | Реальная полезная область | Предел / цена / решение |
|---|---|---|
| Existing selector + Manager, pinned E31 | Сохранить живой engine; выбрать default и проверить новый/rollback path; часть старых external flows может продолжиться | **Reuse уже имеющегося**, без второго control server. GPL-3.0-or-later по pinned LICENSE; redistribution obligations сохраняются. Не все flows/не atomic cutoff |
| QUIC migration, RFC9000 §§8.2/9 | При сохранённом connection state и допускаемых endpoint условиях путь может смениться с validation | Новая независимая VPN-машина не получает крипто/connection/NAT state от наличия CID. Handshake/disable_active_migration/preferred_address и отсутствие usable path ограничивают перенос. Reference для transport owner, не новый SDK |
| WireGuard authenticated roaming, E30 | Обновить outer endpoint уже известного peer по authenticated traffic | Это не автоматическое копирование exit/NAT/session state на независимый VPS. Описательный reference; реализация/лицензия нового backend не выбиралась, код не переносился |
| Mosh SSP, E33 | Терминальная задача может переживать разрывы благодаря application state synchronization | Не прозрачная замена произвольного TCP или HTTP transaction; требуются клиент/сервер и подходящий UDP. Оставить пример уровня приложения; snapshot/release/code integration не выбраны |
| Постоянный anchor для нескольких физических путей, existing COM-17 | Может сохранять gateway/внешнюю tuple при замене underlay | Сам anchor/failure domain остаётся; его outage не лечится одним roaming. Больше latency/расходов/операций; нужна отдельная future R&D приёмка, не P0 dependency |

TCP connection идентифицируется парой sockets; IP/port и состояние remote TCP
существенны. Из этого **не следует**, что любое изменение физической сети
обязательно рвёт TCP: прозрачный туннель с сохранённым exit/state может скрывать
его. Не следует и обратное — обычный proxy switch пересоздаёт arbitrary old TCP.
[E28 RFC9293](https://www.rfc-editor.org/rfc/rfc9293.html#section-3.3),
[E29 RFC9000](https://www.rfc-editor.org/rfc/rfc9000.html#section-9).

При потере ответа бизнес-операции VPN не знает, применил ли сервер её до
разрыва. Не следует автоматически повторять перевод/заказ/отправку формы ради
«бесшовности». Retry/reconciliation принадлежит приложению с его idempotency
contract. [E34 RFC9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2)
ограничивает автоматический retry non-idempotent requests; общий VPN не имеет
семантического знания произвольного зашифрованного трафика.

## 19. R06-003 — исполнимый UX contract и различие платформ

Предлагаемый минимальный contract COM-09/OFF-03/COM-17:

1. **Сохранить рабочую сессию:** явное ограниченное пользователем намерение
   откладывает optional score switch/bulk probes/update. Не бессрочная pin lease;
   expiry, urgent revoke, leak, Stop и обязательное безопасное recovery сильнее.
   Не определять звонок прослушиванием микрофона/окон/экранов.
2. **Выбрать новый выход:** перед действием сообщить ожидаемую область — новые
   соединения; существующие могут продолжить прежний путь/прерваться. После
   fresh probe сообщать его outcome и default, без «все приложения перенесены».
3. **Прекратить прежний путь:** отдельное намерение с предупреждением о
   возможном разрыве; обещание success только по actual retirement fence/receipt
   платформы. Если такой verified capability нет, не предлагать фальшивую кнопку
   «мгновенно сменить IP везде». Не выключать защиту для выполнения этой цели.
4. **Восстановить доступ после отказа:** показать recovery/blocked/unknown и
   ожидаемое вмешательство. Звонок или remote transaction могут требовать
   переподключения/проверки в приложении; фоновое retry не равно их успеху.

Это intent/actions поверх существующего controller, не четыре новых больших
страницы/карточки. В обычном UI достаточно текущего default, факта восстановления
и краткого scope пояснения; подробности раскрываются по необходимости. Упрощение
текста и понимание последствий — **UX-гипотеза, пользовательского исследования нет**.

**Windows:** source подтверждает две разные switch/reload ветки, fresh probe и
generation guards. Scalar `activeChannel` в facade и Dashboard не является flow
census. SOCKS/TUN/UDP/specialized handlers нужны в future coverage; не переносить
результат одной mode на другую. Native safety/device evidence здесь отсутствует.

**Android:** `PhysicalNetworkMonitor` меняет selection/epoch по callback facts;
`NativeWorkerSession` удерживает конкретный `PhysicalNetworkBinding`,
`ProtectedSocketControl` делает protect и bind borrowed transport socket через
retained duplicate с rechecks; `AndroidTunFactory` указывает underlying network.
Это реальные source boundaries, **не доказанная in-place migration на другой
Network**. В просмотренном callback потеря/изменение pinned context даёт fault;
production accepted provider/device runtime остаётся отдельным gate.
[E32 VpnService](https://developer.android.com/reference/android/net/VpnService#setUnderlyingNetworks(android.net.Network[]))
объясняет underlying networks как actual network information для системы;
этот вызов сам не переносит открытые сокеты и не реализует bonding. Не следует
заменять binding на reporting API ради красивого индикатора.

## 20. R06-003 — оппонирование и конечная будущая приёмка

| Возражение | Уточнение / что опровергнет вывод |
|---|---|
| «Почти всё уже есть в COM-09/17» | Да, общий caveat старый. Новое — exact false rendering→pinned selector/group, mixed rollback и in-flight cutoff counterexample; не переизобретать requirement |
| «У нас long-lived transport multiplexing, один swap всё переносит» | Показать actual outbound/stream/NAT migration contract с remote state. Pool/mux может сохранять/рвать группы потоков, одного selected pointer недостаточно |
| «true гарантированно закрывает всё» | Требуется invalidate/join in-flight dials и охват специализированных handlers. Показанный T10 опровергается конкретным дополнительным fence, не именем flag |
| «Один probe прошёл — рабочий звонок восстановлен» | Нужен собственный task observable; иначе A unknown. Probe действительно подтверждает свою ограниченную цель |
| «Смешанные выходы — автоматически утечка» | Нет: оба могут быть protected/admitted. Это отдельный user intent/authority question; direct leak требует собственного evidence |
| «Always reload решает всё» | Он помогает retirement, но увеличивает disruption, не отменяет cleanup uncertainty/OS gate и не повторяет безопасно remote transaction |
| «Anchor даёт отсутствие отказов» | Перенос underlay и смерть anchor разные F. Резерв anchor/state/authority/cost нуждается в своей модели, R06-002 cuts остаются |

Следующие **12 случаев не выполнялись**; это предложения платформам через
руководителя, без новых тестовых fixtures/code:

| ID | Будущий случай | Конечный критерий |
|---|---|---|
| C01 | A long-lived external TCP + новые B connections | D и observed flow outcomes разделены; нет universal-IP claim |
| C02 | UDP association/QUIC/mux/specialized handler, SOCKS против TUN | Каждый mode/transport имеет собственный scope result; unknown не PASS |
| C03 | Успех B probe при зависшем рабочем приложении | Probe success не отмечает task recovered |
| C04 | B dial между swap и rollback A | Default rollback не выдаётся за миграцию всех fB; validity остаётся constrained |
| C05 | Старый dial возвращается после Interrupt(true) | Либо owner fence исключает T10, либо strict-cutoff success не публикуется |
| C06 | Reload кандидат отклонён/старые bytes restarted | Не утверждать сохранение сокетов/бизнес-операции; защита проверяется отдельно |
| C07 | Quiet intent: optional score gain и urgent revoke/expiry/leak | Optional действие отложено, hard gate не блокируется; bounded intent не продлён restart |
| C08 | Пользователь просит немедленно сменить выход | Согласованный scope/предупреждение; success после доказанного retirement, не selector ack |
| C09 | Wi-Fi→mobile, same peer/state против нового gateway | Отдельные inner/outer/task outcomes, caps/расход cellular и authority проверены |
| C10 | Android reporting underlying network без новой socket binding | Не заявляется migration; старый context invalidation не скрыт |
| C11 | Ответ non-idempotent операции потерян | VPN не повторяет payload/не объявляет remote outcome; действие принадлежит приложению |
| C12 | Повторные switches, Stop/late probe и кончился quiet deadline | Generation/intent fences, no direct fallback, stale success не восстанавливает сессию |

Нужно заранее выбрать task fixtures/OS/modes и outcomes, а затем при отдельном
разрешении сравнивать interruption/reconnect/task success и цену переключений.
Численные SLO/скорость/процент сохранения sessions этим чтением не установлены.

## 21. R06-003 — предлагаемая вставка и передача

Предложение для руководителя (общие THEORY/PLAN не изменены):

> Непрерывность задаётся отдельно для protection, selected default, ранее
> допущенных потоков и пользовательской задачи. Selector switch и подтверждённый
> rollback не являются атомарным переносом/возвратом всех потоков. In-flight dial
> способен завершиться после обхода interrupt group; единственный flag не доказывает
> cutoff. Quiet intent откладывает optional churn, но не urgent safety/recovery.
> Независимый gateway не наследует состояние transport/NAT/application от наличия
> QUIC/WireGuard roaming. Смена default, retirement и task recovery имеют свои receipts.

Приоритет: сначала узкий OFF-03/COM-09 contract/outcome/copy над текущим Manager;
затем согласовать строгий retirement intent с platform/R05 и finite C01…C12;
COM-17/anchor исследовать только при доказанной нужде. Новых transport deps,
daemon или flow-inspection базы для этого среза не требуется. Runtime/code/UI
изменений и тестов нет. Metadata/captures и freshness — SOURCES §5 и R06-003 handoff.

## 22. R06-004 — что позволяет установить приватная история сбоя

Дата среза: **2026-10-26**. **READY_FOR_REVIEW** с открытым upstream-source gate; не продуктовая/научная приёмка.

Проблема: пользователь отправляет историю «не подключалось, затем заработало».
Нужно отделить полезное подтверждение controller outcomes от недоказанного
«причину точно установили / за всё время утечек не было / worker перезапускался».
R06-001 уже запрещает считать ring журнал знаменателем uptime/MTTR. Новизна
здесь — **точная цепочка проекций текущего Android кода**, независимые причины
неполноты и условие допустимости конкретного диагноза. Новый logger не нужен.

### 22.1. Фактическая цепочка и уже реализованные хорошие ограничения

`WorkerProcessLink.identify/inspect` читает reply, фиксирует actual parent
received elapsed time и проверяет worker identity. Данные сохраняются даже при
последующем death guard; это не заверение liveness. Repository `retainObservation`
передаёт именно этот validated return в `journal.workerObserved`, без второго RPC.
`WorkerBinding.journalContext=Any()` — marker локального binding generation,
не worker ID; `newObservationContext` **не означает перезапуск worker**.

`SessionJournal` — process-local ring **128** typed events; immutable snapshots,
parent sequence не сбрасывается при Clear/не wrap-ится, capacity exhaustion
обозначается отдельно. Command admission и state publication сериализуются
repository inputLock; causally related state не обгоняет допущенную команду
при записи. Это полезный local order, **не полная distributed causal trace**.

Дедупликация state использует только phase/issue; worker — local context плюс
phase/reason. Повторный reply с теми же признаками не создаёт новый worker event
ради свежих времён. Clear удаляет entries/evicted counter, но сохраняет sequence
и signatures: идентичный periodic sample не заполняет очищенную историю заново.
Process death удаляет эту память; public history не содержит реальную worker
identity/полный snapshot/profile/probe payload. Это privacy/scope design, не
обнаруженный дефект потери обязательного audit log.

`controllerReport` строит **allowlisted text-v2**: исходные observation/receive/
record ages; invalid time — unknown; категории/окно/отдельные строки можно убрать;
wall clock default выключен. Counts retained/matching/included, evicted и
recording_limited остаются явными. Основа окна — parent recording, не свежесть
worker fact. Text cap32768 ASCII — bound одного результата, не heap/Binder/RSS SLO.

`ControllerReportStore` удерживает один frozen source/capture token, merges edits
только внутри него, принимает share только exact displayed immutable revision,
проверяет десятиминутный access expiry. Это **не timed memory wipe** и не recall
уже отправленного текста. `ControllerReportScreen` готовит снимок из history.value,
показывает preview/filter/строки и вызывает onShare(preview). Весь системный
recipient/native share flow этим срезом **не audited**; ничего не отправлялось.

### 22.2. Шесть разных границ, которые нельзя называть одной «потерей логов»

| Граница | Что реально ограничено | Что следует для интерпретации |
|---|---|---|
| Observer coverage | Worker/system/traffic события между доступными validated returns могут не попасть в журнал | Нельзя отвергнуть событие только потому, что его нет в истории |
| Semantic projection/dedup | Журнал содержит выбранные typed изменения, а не каждый sample/полный snapshot | Количество строк не равно количеству probes/retries; record age не refresh time |
| Retention | Ring displacement, Clear и process loss имеют разные последствия | Evicted counter относится к current retained history, не ко всей жизни установки; zero не значит отсутствие всех omissions |
| State publication | StateFlow может пропускать промежуточные snapshots медленному collector | Это **не автоматически потеря каждого journal event**: последний snapshot включает ещё retained entries; loss возникает при переполнении/clear/process boundary |
| Export selection | Time window/categories/omitted rows уменьшают видимый набор | Диагноз после redaction должен стать слабее/unknown, а не восстановить строки догадкой |
| Evidence authority | Validated Binder reply — dated reported fact, printable text — несекретный editable export | Ни timestamps, ни TraceId/sequence не подпись, не proof OS enforcement/egress/task outcome |

StateFlow current official API описывает conflation/equality. Однако web API
сейчас1.11.0, проект явно использует coroutines-android**1.10.2**. Release commit
1.10.2 найден, **две raw-download попытки StateFlow.kt/LICENSE.txt истекли по
timeout**. Пinned implementation comparison не выполнен. Поэтому это reference
API semantics, не исследование source internals именно1.10.2/не повод обновить
зависимость. [E35 StateFlow API](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/-state-flow/).

### 22.3. Условие обоснованного диагноза и ручные контрпримеры

Пусть `H` — возможная фактическая история, `π` — описанные observation/projection/
retention/export операции, `y=π(H)` — полученный текст. `K(y)` — истории,
совместимые с y, source contracts и **известными** coverage bounds. Категоричный
вывод φ допустим только если `∀h∈K(y):φ(h)`. Если есть `h1,h2∈K(y)` с разным φ,
такой вывод **неидентифицируем из данного пакета**. Это собственный простой
conditional argument; множество K не вычислялось solver/ML, вероятности нет.

Примеры, не эксперименты:

- **T11:** network interruption между двумя доступными одинаковыми worker phases
  и история без interruption способны дать один retained semantic projection;
  если требуемый event/traffic observable отсутствует, текст не различает причины.
  Если иной реально записанный mandatory event исключает одну историю, K сужается:
  это фальсификация контрпримера, а не запрет всякого диагноза.
- **T12:** медленный UI не увидел intermediate snapshot, но более поздний ring
  содержит те же events. Объявлять потерю события только из conflation ошибочно.
- **T13:** команда и следующая строка worker состояния расположены по parent
  sequence; без достаточного request/result/context linkage их соседство не
  доказывает, что именно команда вызвала native transition. Same-boot elapsed
  ordering помогает проверке времени, но причинность сильнее временного порядка.
- **T14:** после Clear повторный идентичный state не добавлен; пустой журнал
  не доказывает stopped/VPN failure. Current state и history имеют разные цели.

[E36 Lamport1978](https://lamport.azurewebsites.net/pubs/time-clocks.pdf)
обосновывает partial happened-before и односторонность clock condition; наш
parent sequence **не объявляется Lamport clock implementation**. Даже полный
временной порядок не устанавливает причину сетевой блокировки.
[E37 OpenTelemetry Logs Data Model](https://opentelemetry.io/docs/specs/otel/logs/data-model/)
различает event/source и observed times; это полезный vocabulary reference.
Наш elapsed time не переводится в UNIX nanoseconds под видом эквивалентного
OTLP timestamp, clock uncertainty не скрывается. SDK/collector не предлагаются.

## 23. R06-004 — минимальный диагностический contract, reuse и оппонирование

Для COM-05/07 и AND-12/A-FR-17 предлагается response shape для каждой значимой
диагностической фразы: **что наблюдалось → область/интервал → что неизвестно →
какие альтернативы остаются → одно допустимое следующее действие**. Не обязательно
показывать пользователю техническую структуру: «Сервис сообщил о завершении;
выход трафика не проверен. Переподключите после…» — только при соответствующих
fresh facts и реальных полномочиях. «Причина — DPI» не выводится из timeout.

**Reuse:** текущие SessionJournal/ControllerReport/Store и OS/probe/controller
facts. Сначала интерпретировать имеющиеся scope/counts/unknowns. Если конкретному
диагнозу недостаёт связи, сначала назвать минимальное недостающее наблюдение и
его цену; не собирать всю network/process inventory. В возможном будущем
локальные segments/opaque aliases должны быть привязаны к одному export scope,
без стабильного device/IP/account marker. Ни их реализация, ни необходимость
принятыми не считаются. Binding context и worker restart не смешивать.

**Privacy/cost:** bounded typed projection сохраняет пользу и малый retained
объём. Больше persistent logging не доказывает cause и увеличивает чувствительные
данные/ресурсы. Export не должен извлекать secrets/profile/адреса из raw errors,
просить непрерывный traffic capture или автоматически отправляться в support.
Удаление/expiry контролируют только собственные references, а не чужие копии.
Срок хранения, размер всего процесса и фактическая безопасность share требуют
отдельной платформенной проверки; никаких численных gains не установлено.

**Платформы:** Android exact chain выше даёт основу bounded parent history;
provider/native/device proof остаётся открытым. Windows OFF-05/COM-05 нуждается
в том же interpretation contract поверх **своих** typed observations; Android
128/text-v2/process boundaries не объявляются уже существующим Windows API.
Новый cross-platform logging SDK, database или native actor не нужен по результатам
этого среза. Готовые Lamport/OTel модели используются как теория/словарь, не
копируемый runtime algorithm. Новую third-party code/license интеграцию не выбирали.

| Сильное возражение | Пересмотр / опровержение |
|---|---|
| «Надо переписать ring на SharedFlow, раз есть conflation» | Нет: snapshots содержат retained events. Замена delivery primitive не восстанавливает unobserved/evicted/redacted facts |
| «Current report уже честно пишет not proof, нового нет» | Верно для broad warning. Новое — раздельные six-loss boundaries и condition конкретного диагноза; существующие protections признаны, не предложены как отсутствующие |
| «Никакой root cause вообще невозможен» | Слишком сильно: достаточный typed receipt и coverage могут сузить K. Неидентифицируемость относится к конкретной φ/пакету |
| «Добавим correlation ID и докажем причинность» | ID — ссылка при проверенном producer/propagation contract, не самостоятельная authenticity/causality. Stable user marker ухудшает privacy |
| «Sequence/elapsed полностью решают проблему часов» | Parent order и same-boot intervals полезны; process/boot scope и actual message edges остаются. Optional wall clock не authenticated UTC |
| «Есть evictedCount=0, значит полный журнал» | Не учитывает unobserved, dedup, Clear/process scope и экспортные фильтры; statement completeness требует большей coverage specification |
| «Нет pinned coroutines source — весь вывод неверен» | Internals1.10.2 не объявлены проверенными. Own journal projection/retention/export выводы следуют из текущего local source; StateFlow reference имеет явный version gate |

## 24. R06-004 — конечные будущие случаи и предлагаемая вставка

Следующие10 случаев **не выполнены**; no new fixtures/tests/simulation:

| ID | Будущий случай | Ожидаемое конечное различие |
|---|---|---|
| D01 | Медленный collector, все relevant events ещё внутри ring | Последний snapshot восстанавливает retained events; не фиктивный loss alarm |
| D02 | Больше128 записей / Clear / новый parent process | Разные retention scopes; никакой lifetime completeness/uptime из zero counter |
| D03 | Повторные same phase/reason replies, но новые observation times | History dedup не выдаёт old event за fresh sample; current facts читаются своим путём |
| D04 | Новый binding к тому же worker и новый worker | ctx_new не переводится в утверждение о restart без actual evidence |
| D05 | Worker reply→IPC return→parent запись с invalid/reordered times | Original times сохраняются; unknown/ordered interpretation не чинится свежими часами |
| D06 | Time/category/row redaction удаляет prerequisite диагноза | Классификация ослаблена/unknown; строки не восстановлены inference |
| D07 | Share старого preview после edits/new capture/access expiry | Только exact displayed current revision либо отказ, без нового background capture |
| D08 | Current VPN state жив, история очищена/потеряна | UI не объявляет stopped/error по отсутствию истории |
| D09 | Единственный timeout или отсутствие protected receipt | Нет cause DPI/no-leak/task-success assertion; admissible targeted next action |
| D10 | Export event ID повторился после нового процесса/копии чужого текста | Нет глобального join/signature claim; privacy scope и trust source обозначены |

Предложение руководителю для THEORY/COM-05 (общие документы не менялись):

> Диагностический пакет является проекцией наблюдений с явными coverage и
> потерями. Observer gaps, semantic dedup, retention, delivery conflation и
> user redaction — разные причины неполноты. State snapshot может сохранять
> ring events при пропущенных UI updates. Категоричный диагноз требует, чтобы
> все истории, совместимые с пакетом и доказанным coverage, давали этот вывод;
> иначе сохраняются альтернативы/unknown и задаётся одно допустимое действие.
> Correlation/time порядок не заменяет authority, causal edges или traffic proof.

Сильнейший предел: desk research не устанавливает, какие φ реально достаточно
часто разрешимы в пользовательских инцидентах, и не измеряет diagnostic utility.
Native/device/user validation, полный recipient share flow и pinned1.10.2 source
review остаются открытыми для отдельного разрешённого цикла **после** этого окна.

## 25. R06-006 — рабочая сеть: браузер работает, VPN требует прокси

**READY_FOR_REVIEW**; desk/source research от 26 октября 2026, без продукта/тестов. Вместо повторного captive-portal/export разбора выбран непокрытый S2/S4 срез:
переход в рабочую сеть с PAC/explicit authenticated proxy, strict protection,
обычными пользовательскими правами и уже важной рабочей задачей.

**Предпосылки F:** организация разрешает данный способ VPN; интернет может быть
доступен лишь через proxy P, CONNECT разрешён не к любому адресу/порту; админ
недоступен; у пользователя нет права менять managed policy. Наличие именно
такого ограничения **нельзя выводить только из работающего браузера/таймаута
VPN** — это сценарная предпосылка либо отдельное подтверждённое наблюдение.

Нужно одновременно различать семь независимых результатов:

| Объект | Условие успеха / граница |
|---|---|
| Strict protection | Реально applied guard в своём scope; proxy setting не OS enforcement |
| Network/portal access | Доступ к порталу/локальному P разрешён отдельной authority; это не разрешение любому приложению идти direct |
| Corporate proxy authentication | Поддерживаемый scheme, допустимый P и credential actor; не VPN consent/ключ профиля |
| VPN consent/managed policy | Подготовленный владелец и разрешения ОС; нельзя незаметно заменить managed VPN |
| Default egress | Новый выбранный путь проверен своим owner/epoch; не переписана судьба всех старых flows |
| Existing flows | Могут остаться на старом пути/оборваться; R06-003 ограничения сохраняются |
| App operation | Звонок/RDP/remote command имеют свой outcome; VPN не повторяет чужую транзакцию |

**Новая полезная комбинация** — подготовка разрешённого рабочего underlay с
понятным authority/action barrier **до** вмешательства в текущую рабочую сессию;
затем проверка VPN и scoped settings cleanup. Это требование к orchestration
существующего controller, не новый UI/transport engine или список декоративных фич.

## 26. R06-006 — prerequisite/authority graph и честные cuts

Пусть `N=(network owner,epoch,context)`, `P` — разрешённый proxy, `E` — VPN
gateway, `A` — действующий профиль/authority, `I` — намерение пользователя.
Граф ветвится по фактически доступному пути, а не пытается все методы подряд:

```text
Known current N + applied protection + admissible I/A
  AND allowed access policy / no managed-owner conflict
  AND (
       already admitted direct transport to E
       OR [proxy locator for E
           AND permission to reach P on N
           AND supported authentication + permitted credentials/actor
           AND CONNECT allowed to E:port
           AND transport-compatible stream]
      )
  -> authenticated gateway/inner TLS with valid authority/time
  -> fresh protected path evidence for same owner/N/config
  -> publish default outcome
  -> separately observe/reconnect user task when appropriate
```

PAC branch добавляет `allowed PAC locator/fetch/evaluation` **для конкретной
цели**, а не универсальный «прокси сети». PAC/ProxyInfo — сведения маршрутизации,
не authority. Ответ DIRECT от PAC не разрешает direct fallback защищённого
трафика. Получение PAC само может зависеть от DNS/authentication/доступа, которого
ещё нет; credentials для PAC и для forward proxy — разные scopes.

Для каждого stage j нужны действующие prerequisites на всём required interval:
`[t_j,t_j+d_j] ⊆ valid_j`, текущие N/A/I и полномочия actor. Unbounded ожидание
пароля/админа/радиосети не становится finite RTO. Proxy authentication success
не продлевает profile/trust expiry и не восстанавливает trusted UTC.

| Условный cut | Почему retry не устраняет prerequisite | Достижимое действие без админа |
|---|---|---|
| P доступен только через ещё не поднятый VPN, а VPN требует PAC/P | Неразомкнутый bootstrap cycle | Уже разрешённый offline locator/иной admitted path; если их нет — объяснить конкретную зависимость |
| Guard разрешает E, но не допускает требуемое transport-соединение с P | Разрешение E не тождественно разрешению P | Подготовленный policy-admitted bootstrap lane либо другой разрешённый network; не общий bypass |
| P требует NTLM/Negotiate, текущий connector не умеет этот обмен | Повторить тот же Basic/password не добавляет scheme | Сообщить неподдерживаемый способ; согласованный connector/IT или разрешённая другая сеть |
| CONNECT запрещён к E или transport требует неподдержанный outer UDP | Credential/proxy reachable не дают нужный tunnel | Другой заранее admitted compatible endpoint/transport; не произвольный новый сервер |
| Managed VPN/права ОС запрещают владельца или корпоративная policy запрещает VPN | Пользовательский consent не заменяет admin authority | Действие IT/разрешённая личная сеть; запрет не обходить настройкой клиента |
| Нет доверенного времени/credential/profile уже недопустим | Сеть и 200 CONNECT не решают trust | Законный ранее описанный recovery; не «отключить проверку сертификата/времени» |

Нужна отдельная разница **transport bootstrap permission** и **user direct grant**:
разрешить единственному VPN owner соединиться с P для E не значит открыть P/LAN/
интернет всему device. Портал — ещё один отдельный actor/destination/срок; при
невозможности узкого безопасного разрешения конфликт остаётся видимым.
Это proposal для platform/R05, **не уже реализованный guard API**.

## 27. R06-006 — actual callers и обнаруженные границы

### Windows: управление downstream proxy не равно upstream support

`Manager.Start`: lifetime gate → proxy.Snapshot → prepareEngineChange → engine.Start
→ proxy.SetProxy(local SOCKS) → protected probe. `restoreProxy` обёртывает Restore,
а cleanup вызывает этот путь. `ProxyManager` читает HKCU ProxyEnable/ProxyServer/
AutoConfigURL, пишет `socks=127.0.0.1:1080`, **пытается** удалить PAC и уведомляет
WinINet. Restore возвращает значения из snapshot. Реальный registry не читался.

Три source-level ограничения, не установленный live incident:

1. Это downstream настройка использующих её приложений. В просмотренном пути
   нет WinHttpGetProxyForUrl/QueryAuthSchemes/SetCredentials connector для VPN.
   Microsoft отдельно описывает перенос WinINet config в WinHTTP на session/
   per-request basis; user/service identity имеет значение [E38/E39]. Наличие
   browser SSO не создаёт автоматически authenticated socket у Go core.
2. **Ручной lost-update trace:** Snapshot S0 → Set S1 → внешний user/MDM меняет
   settings на S2 → Restore записывает S0. Restore не сравнивает expected-owned
   current values перед записью. Поэтому rollback способен затереть новое S2.
   Это узкий static conflict, не доказательство частоты/доступности такой гонки
   на конкретном устройстве. Нужен settings ownership/conflict contract; одна
   global mutex приложения не сериализует MDM.
3. Ошибка DeleteValue(AutoConfigURL) при SetProxy игнорируется, readback нет.
   Поэтому nil от setter не доказывает, что PAC снят. Snapshot чтение отдельных
   значений также не различает все read errors и отсутствие. Полный Windows
   proxy stack/policy/AutoDetect/per-connection settings этим срезом не audited.

Предлагаемый порядок: read-only compatibility/authority preflight перед optional
захватом системных настроек; stage и проверка допустимого transport; settings
mutation/cleanup только владельцем с наблюдаемым outcome. Если current settings
изменены другим actor, не стирать S2 ради слепого «восстановлено»: сохранить
защитный контракт, отметить conflict и дать доступное действие. Compare/readback
не объявляются атомарной registry transaction или уже принятой реализацией.

### Core: upstream HTTP capability ещё не admitted product path

В windows/go.mod sing-box**v1.13.19**, sing**v0.8.13**. Exact sing-box commit
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`: HTTP NewOutbound строит dialer,
optional TLS и sing HTTP client; advertised network TCP, ListenPacket возвращает
ошибку. Его public raw outbound.go совпал с local cache по SHA256.

Локальный versioned sing cache `protocol/http/client.go`: TCP CONNECT, optional
preemptive Basic, 200 success, 407→constant authentication error с закрытием
connection; нет challenge-response NTLM/Negotiate negotiation loop. Context
cancellation закрывает handshake connection. Release v0.8.13 указывает commit
`7c349dacf402256d3a7029746073b05d2ead584a`; **public raw client/LICENSE downloads
timeout**, external byte comparison sing не получен. Это явно ограниченный
versioned-cache source факт, не independently verified immutable file pin.

Клиент не получает автоматически NTLM от наличия Headers/username/password.
Переиспользовать как готовый enterprise auth connector без нового контракта
нельзя. Project config admission допускает protected types vless/hysteria2,
detour ограничен shadowtls. Добавление HTTP node в raw не является уже готовой
разрешённой profile→render→guard→probe цепочкой.

TCP-only outer CONNECT **не доказывает невозможность всякого inner UDP**:
compatible VPN protocol способен нести внутренние datagrams в TCP stream.
Однако outer HY2/QUIC UDP не становится поддержанным одним HTTP CONNECT.
MASQUE/CONNECT-UDP — отдельный protocol/capability, не hidden extension текущего
client. End-to-end compatibility проверяет владелец transport, не UI score.

### Android: знать proxy != применить его к VPN socket

`PhysicalPathContext.capture/checkLinkBounds` включает proxy host/port/exclusions/
PAC URL в bounded network fingerprint; NAT64/private DNS также отдельные inputs.
`PhysicalNetworkBinding` удерживает выбранный context. `ProtectedSocketControl`
protect/bind borrowed native socket к выбранной physical Network через retained
duplicate и rechecks. Это **не** PAC evaluation/proxy-authenticated CONNECT.

Public API: `Network.openConnection(URL,Proxy)` API23 привязывает HTTP(S) к
данной Network и указанному proxy; это не универсальный native VPN dialer.
`VpnService.Builder.setHttpProxy` API29 рекомендует HTTP proxy приложениям VPN
network; apps могут игнорировать, PAC over VPN не поддержан, split-route имеет
дополнительные ограничения [E41]. Его нельзя подставить вместо upstream transport
bootstrap. Current source/native/provider/device gates остаются отдельными.

## 28. R06-006 — mature reuse и действия пользователя

| Кандидат | Решение / применимость / цена |
|---|---|
| Existing Manager/guard/prepare/probe/generation | **Reuse**, сохраняя единственного owner; добавить в план scope/outcome contract. Без второго daemon/HTTP control listener |
| sing-box HTTP outbound + sing client | **Conditional adapt** TCP/Basic/TLS building block; GPL3-or-later по прочитанным licenses, у sing cache provenance limit выше. Не NTLM/Kerberos/PAC drop-in; custody/typed auth errors/admission/OS protection требуют разработки и приёмки |
| WinHTTP config/PAC/auth APIs | **Reference/candidate** для Windows adapter с explicit endpoint/credential policy. Это OS API, не новая copied library; user/service token, per-target credentials, cancellation и GUI/service boundary требуют native evidence |
| OpenVPN2.6 manual/management [E43] | **Reference/adapt interaction**: explicit HTTP proxy settings, auth selection/user prompt, auto-nct rejects weak clear-text auth. Не менять core/server; management transport/credential files/examples не копировать в Snowden |
| Автоматически разрешить все P/PAC/LAN или взять OS default credentials для любого discovered endpoint | **Reject**: discovery не authority, unbounded credential disclosure/traffic exception |
| Общий Authenticator/system proxy mutation как лечение всех native sockets | **Reject**: другой scope, managed conflict и влияние на посторонние apps |

WinHTTP поддерживает несколько auth schemes, но серверное предпочтение не равно
нашей допустимости. Его auto-logon policy существует для ограничения default
credential disclosure; нельзя ставить LOW ради «работает везде». NTLM/Negotiate
зависят от connection continuity [E39]. Не копировать неограниченное повторение
407 из примера: отличать finite handshake exchanges от новой попытки с теми же
отвергнутыми credentials, учитывать account lockout и cancellation. Сам общий
VPN не повторяет application transaction после reconnect [E34].

При no-admin UX достаточно четырёх смысловых стадий, не большого нового меню:

- **Проверяем доступный способ подключения.** Passive metadata и разрешённые
  targeted checks; отсутствие evidence не переименовывается в «нужен пароль».
- **Нужно действие.** Запрос proxy credentials только для доказанного допустимого
  P/scheme, в foreground с cancel; либо конкретное «этот способ авторизации
  пока не поддерживается»/IT/другая разрешённая сеть. Corporate password не пароль VPN.
- **Проверяем VPN после входа в сеть.** Proxy/portal success не endpoint trust,
  protected egress или восстановленный звонок; старые flows не переносим обещанием.
- **Подключено в указанном scope / конфликт настроек.** Показывать фактический
  outcome и доступное действие. Не убирать guard и не принуждать сменить managed VPN.

Quiet intent откладывает optional preflight/mutation/смену выхода при живой
рабочей сессии; обязательные revoke/expiry/Stop/error recovery сильнее.
Смена N/P/auth scope/profile/intent во время диалога отменяет stale continuation.
Разрешённая mobile сеть — альтернатива лишь с consent/metered policy; нельзя
самовольно тратить cellular и обещать сохранение RDP/call.

**Accessibility/privacy hypothesis:** одно ясно названное действие и объяснение
причины; status не передаётся одним цветом, focus не похищается фоновыми auth
повторами, cancel доступен с клавиатуры/assistive tech. Запретить пароль/token/
Proxy-Authorization/PAC URL query/realm identifiers в generic errors/export.
Не проверять доступ логином в рабочий аккаунт. Пользовательских/a11y/device
исследований нет; конкретные UI решения здесь не реализуются.

Цена: новый VPS для этого orchestration не обязателен. Главные расходы —
платформенный auth/authority/settings lifecycle, credentials custody и validation;
обходной новый сервер не добавляет отсутствующий CONNECT/auth permission.
Сравнительной скорости/доли успешных рабочих сетей/стоимости разработки нет.

## 29. R06-006 — самостоятельное оппонирование и future acceptance

| Возражение | Ограничение / falsification |
|---|---|
| «У нас уже есть HTTP outbound, задача решена» | Показать полный admitted profile→renderer→auth connector→guard→probe путь. Cache building block один этого не даёт |
| «Есть 407 — значит можно спросить тот же пароль ещё раз» | Scheme/endpoint/credential authority и тип rejection важны; unsupported NTLM не лечится Basic retry |
| «Прокси TCP, значит звонки UDP вообще невозможны» | Смешаны outer/inner layers. Compatible UDP-over-stream способен изменить applicability; ресурсы/latency отдельно |
| «Browser SSO доказывает сеть пригодна VPN» | Доказывает лишь browser path. PAC target/auth session/CONNECT policy могут различаться |
| «Restore S0 всегда правильно» | Внешний S2 образует lost-update counterexample. Если внешний actor физически исключён доказанным owner contract, этот trace неприменим |
| «Достаточно compare перед restore» | Не атомарно с MDM/другим процессом; нужен conflict/failure contract, не новый универсальный lock promise |
| «Любой proxy bootstrap уже direct leak» | Transport connection owner→P для encrypted tunnel и пользовательский direct grant разные scopes; legitimacy определяется заранее admitted policy |
| «Неподдерживаемая рабочая сеть означает провал всей системы» | Это конкретная capability/authority граница. Нужен доступный alternate actor/path; не ложное обещание безотказности |

Все случаи ниже **NOT_RUN**, не созданные fixtures/tests и не реальные инциденты:

| ID | Случай | Конечный критерий |
|---|---|---|
| W01 | Browser works, VPN timeout, proxy metadata present | Не выводить proxy/DPI/auth причину из одного факта; scope/unknown видны |
| W02 | 407 Basic vs NTLM/Negotiate с тем же полем password | Supported/unsupported/denied различены, bounded attempts, no downgrade/credential spray |
| W03 | PAC target-dependent result, PAC requires unavailable tunnel | Не считать один proxy всемирным; concrete bootstrap cut/allowed alternative |
| W04 | PAC says DIRECT, policy forbids direct | Policy stronger; не hidden fallback |
| W05 | Guard allows E, not required P | Нет несанкционированной aperture; action barrier с named prerequisite |
| W06 | Proxy 200, inner TLS/time/profile invalid | Не публикуется protected/CONNECTED по proxy success |
| W07 | CONNECT denied vs UDP outer vs inner UDP-over-TCP | Раздельная eligibility и причина; не ban всей прикладной категории |
| W08 | Network/PAC/proxy изменился при auth dialog | Stale credentials/receipt не применяются к новому scope |
| W09 | Managed VPN conflict/нет OS consent | Нет невидимой замены/подделки согласия; доступное IT/OS действие |
| W10 | Живой call/RDP+optional score switch и mandatory expiry | Quiet не отменяет safety; по default ack не обещают continuity |
| W11 | Snapshot S0→own S1→external S2→Stop/failed Start | Не затирать S2 без ownership/conflict decision; фактическая защита отдельно |
| W12 | PAC DeleteValue failed, setter otherwise successful | Nil setter не successful effective configuration receipt |
| W13 | Cancel/Stop/credential expiry во время handshake | Без late publish/retry/снятия guard и без повторения remote transaction |
| W14 | Keyboard/screen reader/background repeated auth | Понятный status/action/cancel, нет focus loop; actual usability неизвестна до проверки |
| W15 | Proxy challenges/credential/PAC query попали в error | Allowlist projection не экспортирует secrets/организационные identifiers |

## 30. R06-006 — предложение руководителю

Существующие COM-03/04/07/09/10/OFF-03/05 и Android platform/UX требования
нужно дополнить **рабочим upstream-proxy contract**, а не засчитать одной
настройкой системного proxy. ID/status общих планов этим пакетом не меняются.

> При входе в рабочую сеть раздельно проверяются допустимость underlay/bootstrap,
> proxy discovery/authentication, VPN authority/consent и защищённый путь.
> Browser SSO, proxy metadata и CONNECT success не подтверждают остальные стадии.
> Настройки ОС имеют внешних владельцев: rollback снимка не должен молча стирать
> новый managed state. Важная сессия откладывает optional изменения, но не safety;
> отсутствующий protocol/permission обозначается честным action barrier.

Сильнейшие пределы: нет live corporate setup, actual credentials/MDM inventory,
native OS/guard/acceptance/user evidence; sing cache не independently byte-verified
из-за public raw timeouts. Auth/ownership предложения требуют platform/R05
review. Это конечный срез; diagnostic export R06-004 не повторён.

## 31. R06-007 — IPv6-only/NAT64: ошибка адреса не равна отказу VPS

Дата среза: **2026-10-26**. **READY_FOR_REVIEW**, source/desk review. В теории уже есть общий IPv6/NAT64/464XLAT caveat. Новое —
actual IPv4-biased bootstrap chain, условные no-admin recovery ветки и разделение
логического peer/physical dial representation/inner capture/пользовательского исхода.

Сценарий: ноутбук/Android меняет Wi-Fi на IPv6 mobile network; принятый VPN
gateway E имеет IPv4 literal либо имя. Browser может работать, VPN не подключается;
оператор недоступен, важная call/RDP session не должна лишний раз прерываться.
**IPv6 адрес primary link сам не доказывает отсутствие CLAT/stacked IPv4 пути.**
Таймаут не доказывает блокировку, смерть VPS или наличие именно NAT64.

| Сочетание фактических возможностей | Что принципиально возможно / неизвестно |
|---|---|
| Native IPv6 route + admitted IPv6 gateway | Native IPv6 dial при valid authority/свежих receipts; наличие адреса не health proof |
| IPv6 underlay + рабочий CLAT/PLAT + IPv4 E | IPv4 literal/socket может работать через464XLAT; нельзя объявлять каждый literal неисправимым |
| DNS64/NAT64 без пригодного IPv4 socket path + FQDN E | AAAA synthesis может дать compatible IPv6 representation, если используется подходящий resolver/path |
| Та же сеть, но только IPv4 literal без admitted synthesis/CLAT | У этого представления endpoint нет доказанного пути; это ещё не global server failure |
| Сломанный authoritative AAAA и живой A | Обычный DNS64 может не синтезировать AAAA; RFC8305§7.2 описывает отдельный local-synthesis fallback, не автоматическое право клиента делать его |

RFC6147 DNS64 синтезирует AAAA из A для обращения **по имени**; RFC6877
описывает CLAT для IPv4 literals/sockets; это разные механизмы [E45/E46].
NAT64 prefix не обязательно64:ff9b::/96: RFC6052 поддерживает network-specific
prefixes нескольких длин и свой encoding [E47]. Простое приписывание IPv4 bytes
к hardcoded /96 не универсальное решение.

## 32. R06-007 — source chain и закрытость bootstrap

**Windows:** `corebridge.renderAndPrepareContext` preflight только для enabled
vless+ws domains. HY2/REALITY origin literals этот resolver обходят; любой IP
literal (включая IPv6) fast path Resolve возвращает без DNS. Это не универсальное
утверждение «программа совсем не умеет IPv6».

`core.PreflightResolver`: UDP LookupHost к четырём явно указанным IPv4 resolver
targets; результат фильтруется `firstPublicIPv4`. Сам LookupHost не объявляем
исключительно A query. DoH JSON к двум IPv4 literals явно запрашивает type=A и
принимает Type1. При failures/time budget — host→IP cache с wall-clock TTL7days.
Нормальные network-produced результаты этой ветки IPv4; `cacheGet` сам принимает
любой net.ParseIP, поэтому не приписываем всему возможному cache строгую IPv4
типизацию. Реальный пользовательский cache/окружение не читались.

Renderer получает WithDialOverrides(host→IP), делает numeric dial, оставляя TLS
server_name/WS Host доменными. Без override остаётся hostname и показанный
bootstrap DNS1.1.1.1/direct, общий strategy prefer_ipv4; protected DNS через
tunnel — отдельный этап. prefer_ipv4 не значит запрет IPv6. Но переключить одну
эту строку на prefer_ipv6 недостаточно для IPv4 literal resolver/origin/override.

**Условный cut:** если нет пригодного IPv4 socket/CLAT/proxy пути к явно заданным
bootstrap targets, а альтернативный resolver/IPv6/synthesis path не admitted,
перечисленные retries не замыкают bootstrap. Даже полученный каким-либо способом
A/старый IPv4 cache не доказывает текущую reachability E. Не утверждаем отсутствие
всех внешних translation/proxy adapters или одинаковый исход всех конфигураций.

**Android:** PhysicalPathContext включает public `properties.nat64Prefix` при
SDK>=30 в fingerprint; prefix/private DNS/proxy/route changes — часть context.
Однако fingerprint не NAT64 health receipt. Pinned PhysicalNetworkBinding и
borrowed socket protect/bind требуют текущего physical context. `LibboxPlatform`
localDNSTransport.lookup/exchange явно отклонены до accepted resolver policy.
Это существующая fail-closed граница, а не разрешение молча взять global DNS.

AndroidTunFactory требует IPv4 **внутренний TUN** для pinned DNS bridge, optional
IPv6 capture, не вызывает allowFamily/allowBypass. Это не требование IPv4
**физической сети**: inner capture и outer transport различаются. Не «чинить»
NAT64 отключением IPv6 capture или добавлением fall-through route.

**Existing reuse:** sing-box1.13.19 cache `common/dialer/resolve.go` для domain
вызывает router.Lookup и DialParallel/DialSerial, а numeric destination передаёт
lower dialer без DNS. Значит address-family race не создаёт автоматически
NAT64 representation для произвольного literal. Новый raw comparison этого
файла timeout; cache hash/version/commit recorded. Whole lower dialer/native
translation paths не audited; нельзя объявлять синтез вообще отсутствующим в ОС.

## 33. R06-007 — admissible representation graph и user actions

Условная доступность transport к E:

```text
current N + valid profile/peer E + actual guard/consent
 AND (
      admitted native IPv6(E)
      OR working IPv4 route/CLAT(E4)
      OR FQDN(E) + admitted network-compatible DNS64 resolution + usable translator
      OR explicitly admitted local synthesis(E4,prefixN,encoding)
     )
 -> dial bound to same N/context + original authenticated peer identity
 -> protected path receipt for actual owner/config/epoch
 -> default outcome; existing flow/task outcome checked separately
```

Последняя OR ветка — **предложение для platform/R05**, не текущая implemented
feature. Prefix discovery/наличие AAAA не выдают новую server authority. Нужно
связать исходную identity E, physical address D, resolver policy, prefix/context,
scope и сроки; TLS/name/pins/peer authentication не заменять синтезированным IP.

RFC8305§7 даёт готовый reference для NAT64 literals/broken AAAA/VPN split DNS:
корпоративный resolver может знать A, но не underlay prefix; DNS64 underlay
может знать prefix, но не внутреннее имя. Описан local synthesis после правильного
разрешения, **не** универсальная команда поменять corporate/Private DNS на public.
Применимость этой ветки зависит от policy/routes/authority [E48]. RFC7050 полный
текст получить не удалось; процедуру discovery здесь не выдаём за independently
reviewed implementation. Собственный NAT64 resolver/translator не создаётся.

Разные caches/receipts не смешивать:

- Logical host→A/AAAA candidate ещё не current reachable path.
- Derived NAT64 address привязан к physical context/prefix/resolver policy;
  старый N→новый N не переносится одним hostname cache. Prefix может совпадать,
  translator/capacity — уже нет; нужен свежий evidence.
- Inner protected DNS, outer bootstrap DNS и actor/доступ к локальному resolver
  имеют разные permission/failure scopes. Один успешный web request не закрывает их.

Временные gates действуют до/после resolution/dial/probe; N/profile/intent change
отменяет late continuation. Unknown expiry/UTC не исправляется выбором другой
семьи адресов. Численного RTO при unbounded radio/translator wait нет.

**Достижимое без админа:** сохранить ещё работающий admitted путь; использовать
уже имеющийся OS translation/native IPv6 path после обычных gates; выбрать
заранее accepted IPv6 endpoint/profile branch; либо другую разрешённую сеть с
metered/roaming consent. Если вариантов нет — issuer/IT должен подготовить
endpoint/policy, нельзя подделать descriptor/выключить guard/DNS trust. Новый VPS
не обязателен при рабочем CLAT, но наличие одного IPv4 сервера само не гарантирует
доступ из любой IPv6-only сети. Фактическая инфраструктура/тариф не проверялись.

**UX:** «Проверяем совместимый путь в этой сети» и конкретное недостающее
условие, если оно установлено; иначе unknown. IPv6 underlay→NAT64→IPv4 VPN exit
допустим: address family сети не public egress family и не признак утечки.
Не quarantinе весь gateway/все сети из одного family/bootstrap failure; это
граница observations для R03, не реализация его decision algorithm.
Quiet intent/old flows/remote transaction правила R06-003 сохраняются.

## 34. R06-007 — reuse, оппонирование и finite NOT_RUN cases

| Кандидат | Решение / цена и предел |
|---|---|
| Existing renderer/prepare/guard/epoch/probe + physical context | **Reuse/extend contracts**, сохраняя logical peer vs physical address distinction. Не новый sidecar или глобальный DNS owner |
| RFC6147/6877/6052/8305 | **Reference** готовых архитектур/encoding/address race. Не копировать новый userspace CLAT stack; standards reading не device compatibility proof |
| Existing OS/native resolver/translation | **Conditional reuse** при accepted policy и actual network binding. Android local DNS gate пока явно закрыт; universal Windows/Android CLAT support не установлена |
| sing-box resolve/dial candidate chain | **Conditional adapt** existing GPL3-or-later dependency; numeric fast path/candidate routing учитываются. Cache source просмотрен, independent new-file raw comparison не получен |
| Hardcode64:ff9b::/96, force public DNS, убрать capture family, скрытый direct | **Reject**: не учитывает prefix format/context/authority, может ломать split/private DNS и protection |

Цена — сложность bootstrap/candidate provenance и finite platform coverage;
address races расходуют sockets/radio/probes, требуют budgets/cancellation.
RFC timing recommendations не переносим как измеренные SLO Snowden. Реальные
latency/battery/CPU/resource gains не измерены; это совместимость/надёжность,
а не обещание универсального ускорения.

| Возражение | Граница / falsification |
|---|---|
| «Android с CLAT всё уже решает» | Возможно для данного пути; показать actual socket/selected network evidence. Тогда no-CLAT cut неприменим; не повторять translation в приложении |
| «DNS64 автоматически лечит каждый IPv4 literal» | DNS64 работает через AAAA query; literal bypass и CLAT/local synthesis — отдельные ветки |
| «Везде prefer_ipv6 и проблема закрыта» | Не меняет IPv4 resolver literal/forced numeric target; full bootstrap closure нужна отдельно |
| «Сертификат делает любой resolver safe» | Peer auth не равна DNS disclosure permission/reachability; original privacy policy остаётся |
| «TUN IPv4 значит сеть должна IPv4» | Это разные слои. Противоположное утверждение также не доказывает работоспособность outer path |
| «В peer вместоIPv4 можно подписать NAT64 IPv6 и забыть» | Network-specific representation устаревает при N/prefix change; stable authority и ephemeral path различаются |
| «Слишком сложная новая модель» | Полного NAT64 engine/solver нет; сначала named cut и минимальный contract поверх существующих компонентов |

12future cases, все **NOT_RUN**, без fixtures/tests/simulation:

| ID | Случай | Критерий |
|---|---|---|
| N01 | IPv6/DNS64 no-CLAT vs usable CLAT к одному E4 | Разные path outcomes, не одинаковый «VPS умер» |
| N02 | FQDN DNS64 vs forced IPv4 override | Original TLS/peer identity сохранена; incompatible override не объявлен usable |
| N03 | IPv4 bootstrap targets unreachable, допустимый network DNS64 доступен | Named bootstrap cut/allowed alternative, без silent DNS/direct fallback |
| N04 | Broken AAAA/valid A на NAT64 | Admitted fallback либо явная граница; не бесконечный повтор broken family |
| N05 | Prefix/context сменился во время pending dial/probe | Late candidate/receipt не публикуется на новом N |
| N06 | Network-specific prefixes допустимых длин против hardcoded/96 | Correct representation по принятому contract либо отказ; не string concatenation |
| N07 | Signed logical E4 и derived physical D6 | Authority/identity и translation provenance разделены |
| N08 | IPv4 inner TUN при IPv6 underlay и optionalIPv6 capture | Capture не выключается ради подключения, protection scope подтверждается отдельно |
| N09 | Android localDNS unsupported/managed Private DNS | Нет автоматического global/plaintext resolver; action barrier |
| N10 | Старый logical cache/новый Network/translator unavailable | Cached candidate не health receipt; no global gateway quarantine |
| N11 | Address race на слабом radio, loser/cancel/Stop | Bounded resource policy, no late success; реальные затраты требуют измерения |
| N12 | Call/RDP+new default family/remote transaction unknown | Нет continuity/all-flow/transaction-success обещания по одному probe |

## 35. R06-007 — передача

Предложение руководителю для существующих COM-04/07/09/TH-02/04 и Android
DNS/platform roadmap; requirement statuses/общая теория не изменялись:

> Совместимость family определяется замкнутым bootstrap и фактическим network
> context. DNS64, CLAT и local synthesis — разные ветки; numeric address может
> обходить DNS/address-race логику. Logical peer identity и network-specific
> dial representation имеют разные сроки/authority. Failure одного такого пути
> не является доказательством отказа VPS; capture/Private DNS нельзя ослаблять
> ради доступности. Публикуются отдельные protected/default/task outcomes.

Четыре RFC primary текста получены с hashes, actual local chain закреплён.
RFC7050/AndroidLinkProperties page/new box source raw — timeout; эти gaps не
повышены до verified source. Full lower dialer/OS CLAT/native/device/live/user
evidence отсутствует. Продуктовый NAT64 support этим пакетом не объявляется.

## 36. R06-008 — совместная рабочая сеть: NAT64 относится к конкретному hop

Дата среза: **2026-10-26**. Короткий конечный composition срез **уже прочитанных** E40…48/actual captures R06-006/007, без новой source-acquisition партии. **READY_FOR_REVIEW**. Вопрос не сводится к двум отдельным чекбоксам «proxy»/
«IPv6»: нужно установить, **какой peer dial-ится на какой сети**.

Просмотренный sing HTTP client сначала вызывает lower dialer с `serverAddr=P`,
затем записывает CONNECT request с `destination=E` в URL.Host/Opaque. Это два
разных адресных scopes. В normal FQDN CONNECT proxy разрешает указанную цель
на своей стороне; local DNS/translation нужна для его собственного lower hop,
а не обязательно для конечного E. Это cache-scoped source факт с прежним
external-byte gap, не реализованный corporate-proxy путь продукта.

| Участок | Чей network/identity/authority нужен | Чего нельзя переносить |
|---|---|---|
| Device N→proxy P | Local N/context, bootstrap route/DNS64/CLAT, proxy endpoint/auth permission | Native IPv6 E не доказывает достижимость IPv4-only P |
| Proxy P→gateway E | CONNECT target policy и network resolver/reachability самого P | Prefix локальной N не обязан иметь смысл в сети proxy |
| Inner VPN peer authentication | Original E identity/name/pins/profile и time | Proxy200 и auth P не заменяют auth E |
| App protected path/task | Actual engine/guard/probe и observable приложения | CONNECT success не flow migration/task acknowledgement |

Условный совместный граф:
`Allowed_N(P) ∧ Reachable_N(P) ∧ Auth_P ∧ Allowed_P(E) ∧ Reachable_P(E)
∧ Auth_E ∧ Guard ∧ FreshProtectedReceipt`. У каждого leg свой срок, actor,
network/config/intent binding. Это **specification refinement**, не новый DSL,
SDK, планировщик R03 или authority service R05.

### Новый контрпример T15: полезный механизм в неправильном месте

Предположим P доступен из IPv6-only N, а сам P имеет нормальную IPv4 reachability
к E4. Прямой CONNECT E4 может сработать. Если будущая универсальная NAT64-fix
подменит E4 на `synthesize(prefix_N,E4)` **до** формирования remote CONNECT,
P получит network-specific D6, к которому в его сети может не быть маршрута.
Так можно сломать путь, исправляя не тот hop. При proxy в той же translator
области D6 иногда достижим; контрпример не доказывает универсальный отказ.
В текущем Snowden такой local-synthesis feature **не заявлен реализованным**:
это предотвращаемый composition defect, не найденный live bug.

Обратный случай: E имеет nativeIPv6, но P задан IPv4 literal без пригодного
local IPv4/CLAT/admitted synthesis. NativeIPv6 E не исправляет N→P. Простое
«включить IPv6» без topology может давать противоположный ожидаемому результат.

## 37. R06-008 — наблюдения, резерв и минимальная полезная обвязка

Названия server/channel недостаточно для recovery plan: надо удержать логический
peer отдельно от каждого physical leg и места DNS/translation. Это естественное
расширение existing prepare/render/observation contracts; новая база endpoints
и просмотр пользовательского трафика не требуются по данному выводу.

**Failure attribution:** отсутствие CONNECT до E при отказе N→P или auth P не
доказывает смерть E. Пара каналов E1/E2 через один P, один credential actor и
один bootstrap имеет общий cut. Разные страны VPS сами не делают этот резерв
независимым по F «рабочий proxy недоступен». Это concrete refinement reserve
graph R06-002; порядок кандидатов/статистику R03 не реализуем.

**Реально полезный UX:** условно разделить «подготовка доступа через рабочую
сеть» и «проверка VPN» с одним следующим действием по известному barrier.
Слова «прокси недоступен»/«нужен пароль» допустимы только по соответствующему
receipt; при opaque timeout сохраняется unknown. Auth foreground/cancel,
managed policy/quiet mode/privacy rules предыдущих пакетов сохраняются.
Пользователю не предлагается вручную синтезировать адреса, менять DNS/registry
или подтверждать чужой transaction retry. User-tested outcome отсутствует.

**Reuse/adapt/reference/reject:** existing controller + per-leg observation
contract; HTTP CONNECT/E44 и DNS64/CLAT/E45…48 — reference; existing GPL3+ core
building blocks — conditional adapt с source/admission gaps R06-006/007.
Reject единый global hostname→«исправленныйIP» rewrite перед любым detour.
Новые dependencies/code copies не нужны. Цена — bounded topology/provenance
metadata и дополнительные finite cases; бюджет ресурсов/выгода не измерены.

Применимость Windows: ownership системного downstream proxy не подменяет
описанные upstream legs. Android: protect/bind и physical fingerprint относятся
к реальному local socket; Builder.setHttpProxy не создаёт CONNECT leg. Actual
JNI/service/guard интеграция этих предложений не выполнена и не принята.

## 38. R06-008 — оппонирование и конечная передача

| Возражение | Граница |
|---|---|
| «E уже numeric, DNS на P ни при чём» | Верно для конкретного E4 literal, но не для FQDN; native vs derived D6 и routability P всё равно различаются |
| «Proxy тоже внутри N, prefix сработает» | Возможно; именно поэтому T15 conditional, нужен actual translation realm/route evidence |
| «Достаточно global Happy Eyeballs» | Он не решает, к какому leg относится resolver/authority; numeric/detour placements зависят от actual caller |
| «Это требует нового сложного orchestrator» | Не установлено. Сначала минимально дополнить существующий prepare/receipt contract и scope текста; новый runtime не выбран |

Шесть **NOT_RUN** случаев, ручные conditional traces не исполнялись:

| ID | Случай | Критерий |
|---|---|---|
| J01 | P6/E4 и P4/E6 на IPv6-only N | Family compatibility проверена на фактическом local hop, не только E |
| J02 | FQDN E разрешает P; local resolver E недоступен | Нет ненужного local DNS cut/ослабления policy; remote resolution не выдаёт новую trust authority |
| J03 | prefix_N-derived D6 отправлен P вне translator realm | Не universal rewrite; original logical E/leg scope сохранены |
| J04 | E1/E2 разных стран sharing один P/auth actor | Резерв не объявляется независимым по этому common cut |
| J05 | N/P/profile change между auth P и auth E/probe | Late leg receipts не подтверждают новый составной path |
| J06 | CONNECT/auth успешны, app/task или guard unknown | UX не публикует all-device/no-leak/continuity/business-success обещание |

Предложение руководителю: в accepted recovery/observation contracts фиксировать
placement DNS/translation и per-leg identity/authority. Общие планы/теория не
изменены. Модель не доказывает performance/reliability конкретной сети и не
устраняет missing adapter/native evidence. Внешний source pin limit наследуется;
новых credentials/settings/profile/live данных не читалось.

## 39. R06-010 — рабочие сети: split DNS/NRPT/PAC, LAN-сервисы и чужая политика на exact callers

Дата среза: **2026-10-26**. Область — R06-010; baseline R06-006…008 не повторяется. Использованы Snowden source и официальные документы ОС.

### 39.1 Windows: фактическая строгая политика и её следствия

Source-факт: strict-набор WFP разрешает только engine-AppID, loopback, DHCP
(67/68,546/547) и интерфейс snowden0; далее «block direct IPv4/IPv6» catch-all
(`wfp_killswitch_windows.go:75-96`). **LocalSubnet-разрешения нет** — это проектное
fail-closed решение, не дефект.

| # | Бытовая/рабочая задача | Authority | Actual hop | Исход под защитой | Безопасное действие |
|---|---|---|---|---|---|
| W1 | LAN-принтер/NAS по IP | Snowden WFP sublayer | прямые ALE-connect → block(18/19) | Заблокировано | Только явное ручное отключение защиты (manual action, различать с автоматической ошибкой); scoped LAN-permit — будущая policy-заявка владельца, не дефолт (PLAN §3: safe traffic без direct fallback) |
| W2 | Корпоративные имена (NRPT-правила GPO/VPN-клиента) | ОС/GPO + корп. VPN | NRPT → корп. resolver по прямому UDP → block | Имена не разрешаются; авто-fallback на публичный DNS не создаётся | Объяснимое состояние «корп. имена недоступны под защитой»; альтернатива — корп. профиль как обычный destination через туннель, если политика разрешает |
| W3 | Параллельный рабочий VPN-клиент | Его собственный WFP/route | его egress-процесс ≠ engine-AppID → block | Клиент не установит туннель | Никакого скрытого разрешения; последовательное использование (снять защиту → поднять рабочую VPN) — осознанный выбор владельца задачи |
| W4 | PAC/WPAD корп. прокси | Прокси-владелец (R06-006) | PAC-fetch с LAN/corp host → block | Автодетект прокси не работает | Наследует ownership R06-006: explicit settings, не тихая подмена |

MS-GPNRPT/Add-DnsClientNrptRule фиксируют NRPT-механику (GPO `Software\Policies\
Microsoft\Windows NT\DNSClient`, namespace→servers); Snowden NRPT не меняет
(negative grep по windows/) — конфликт W2 — свойство композиции, а не ошибки.

### 39.2 Android: сочетания, которые ОС реально разрешает

Source-факт Snowden: Builder full-tunnel `0.0.0.0/0`+`::/0`, один DNS, без
allowBypass/allowFamily, без per-app списков (`AndroidTunFactory.kt:66-78`);
VpnService применяется к пользователю профиля целиком.

| # | Сценарий | ОС-семантика | Что пользователь/приложение НЕ может |
|---|---|---|---|
| A1 | Snowden (personal) + корп. VPN (work profile) | Разные users → два VpnService живут одновременно, каждый в своём профиле | Snowden не видит и не управляет work-profile VPN; DPC-политика недоступна |
| A2 | Always-on + lockdown «блокировать без VPN» на Snowden | ОС блокирует весь egress мимо TUN профиля; при смерти туннеля — полная блокировка до восстановления | Приложение не может ослабить; это и есть требуемый fail-closed — защита совпадает с ОС-инвариантом |
| A3 | per-app VPN (allowed/disallowed sets) | Списки — прерогатива Builder/DPC; текущий Snowden их не задаёт | Пользователь меняет наборы в системных настройках/MDM, не кнопкой в приложении; для введения per-app нужен новый policy-слой, не UI-фича |
| A4 | Второй personal-VPN клиент | prepare() отбирает TUN у предыдущего VpnService (один на пользователя) | Snowden не может запретить отбор; обязан наблюдать утрату (OsProtection) и отражать незащищённое состояние |

### 39.3 R06-19 — минимальный UX-контракт конфликтующих владельцев

Существенный UX-пробел из поручения: «объяснимое состояние ограничений». Предложение:
каждое наблюдение блокировки несёт **owner-badge** ∈ {Snowden-защита, Чужой-VPN,
ОС-lockdown/MDM, Сеть/ISP} + минимальное безопасное действие + честное unknown.
Правило запрета: badge никогда не предлагает direct fallback и не подменяет
владельца (R06-006 ownership). Метрика — useful-task recovery из R06-004 (критерий
K(y) §22-24): успешное восстановление задачи, а не выданный порт/разрешение;
для W1/W3 recovery-действие = осознанный manual switch, и метрика обязана
учитывать его как отдельный transition, не как провал защиты.

### 39.4 Границы

Нет полевых замеров восстановления задач; матрицы — source/exact-caller модели
ОС-поведения; AOSP/Android-версии развиваются (флаги always-on/lockdown сверять
по API level при реализации). Русскоязычные страницы MS Learn для NRPT-командлет
существуют через ru-ru локаль, пин дан по canonical en-US + протоколу MS-GPNRPT.

### 39.5 NOT_RUN

Реальные LAN-принт под защитой; установка NRPT-правил; включение lockdown;
эмпирический per-app набор; замер K(y) на W1…W4/A1…A4. Все future cases NOT_RUN.

## 40. R06-011 — измерение useful-task recovery: контракт событий и референсы

Дата среза: **2026-10-26**. Уточняется измерение useful-task recovery с exact caller/version/license; baseline §39 (owner-badge) и R06-004 K(y) не повторяются.

### 40.1 R06-20 — контракт измерения

Единица анализа — **попытка пользовательской задачи**, не соединение/порт.
Событие (append-only, producer — платформа):

```
task_attempt {
  id, class ∈ {перевод, поиск, почта, звонок, стрим, корпоративный доступ, …}
  start_ts
  terminal ∈ { useful-complete | partial | fail | abstain }   // abstain ≠ fail
  failure_badge? ∈ { Snowden-защита | Чужой-VPN | ОС/MDM | Сеть/ISP | Неизвестно }
  transitions[] each { kind ∈ {manual-channel-switch, config-change,
                                protection-off(explicit), wait, none}, ts, caused_by }
  t_recover?  // start→useful-complete
}
```

Правила валидности измерения:

1. **Исход — только полезное завершение задачи.** port-open/DNS-resolved/
   tunnel-up не исход (запрет поручения и §39.3). Partial — отдельный исход,
   не округляется до success.
2. **Recovery-успех** = useful-complete после отказа за ≤ T_recover **без**
   прямого egress; `protection-off` учитывается как отдельный transition
   (осознанное ручное действие, не провал метрики и не успех защиты).
3. **Регистрация классов задач заранее** (R03-009 ledger): без предрегистрации
   набора классов отчётная доля успехов неидентифицируема — отбор задач сам
   смещает метрику.
4. Метрики здоровья UX: per-class recovery rate с union-границей по числу
   классов; распределение failure_badge (смещение к «Неизвестно» = провал
   объяснимости); доля protection-off-переходов как цена UX-дизайна.
5. Producers: Windows — типизированный журнал → frozen export (§22-24);
   Android — useful-task/outcome producers (AND-ветка). Ownership платформенный;
   научная валидация — R03.

### 40.2 Референс-реализации

| Реализация | License (проверено) | Паттерн | Цена адаптации |
|---|---|---|---|
| [tailscale/tailscale](https://github.com/tailscale/tailscale) | **BSD-3-Clause — файл LICENSE прочитан этим срезом** | Enum-состояния здоровья соединения + объяснимые причины + action-hint; здоровье собственного mesh | Переносим только паттерн enum+hint; их домен — свой mesh, чужих владельцев (MDM/ISP) у них нет — модель owner-badge наша |
| Chromium/Chrome net-error pages | BSD-3 (проект), общеизвестный UI-паттерн | Плоский код ошибки + человеческое объяснение + предложения действий | Паттерн, не код; для badge-модели нужен источник причин, которого у браузера нет |
| AOSP Always-on/lockdown системные уведомления | AOSP (Apache-2.0), baseline-пины R06-001 (E01-E19, AOSP16 commit) | Платформенный владелец сам объясняет ограничение — эталон owner-badge | Формат: exact-file recheck по AOSP pin NOT_RUN этим срезом |
| AmneziaVPN / Mullvad | широко документируемые GPL-3.0; **файл-проверка NOT_RUN** этим срезом | Диагностика/экспорт для support | Указаны как кандидаты для следующего среза, не как принятые референсы |

### 40.3 Границы и NOT_RUN

Контракт — модель; любые полевые измерения, калибровка T_recover, реализация
событий в журналах платформ, проверка лицензий Amnezia/Mullvad файлами —
NOT_RUN. Эффект UX не измерен; референс-паттерн ≠ доказательство применимости.
