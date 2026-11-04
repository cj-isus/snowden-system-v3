# План разработки Android-клиента

Редакция 4 ноября 2026 года. Цель — полноценное приложение для повседневной и
рабочей связи: понятное подключение, надёжное восстановление, объяснимые правила,
бережное использование ресурсов и доказанная защита в заявленном режиме.

Это следующий порядок разработки. [Действующий реестр Android](../../ANDROID-PLAN.md)
и [STATE](../../../android/STATE.md) сохраняют статусы AND/A-FR и находятся у
Android-владельца. [План ПК](PC-PLAN.md) использует общие trust/adaptive/server
контракты, но его Windows-реализация и приёмка не блокируют независимые Android-срезы.

## 1. Что сохраняем и чего пока нет

### Уже полученный результат

По [матрице 30 октября](../../android/LOCAL-VALIDATION-2026-10-30.md) есть целый
libbox 1.13.19/sourceRevision4, одобренный debug AAR, собранный APK, 17 профильных
unit tests и три Compose navigation tests на одном API36/arm64 устройстве.
Проверены девять номинальных страниц и сохранение одного дочернего выбора после
recreate. Точные hashes и команды находятся в этой матрице, не заменяются
пересказом и не распространяются на новую сборку автоматически.

Это не доказательство JNI/VpnService/TUN/защищённого пути, полной доступности,
16KiB runtime, холодного восстановления или выпуска. UI-only изменение с тем же
одобренным AAR не требует выдуманного нового core build; изменённые AAR bytes
требуют новой identity и отдельного одобрения.

### Следующий настоящий разрыв

В [SnowdenApplication](../../../android/app/src/main/kotlin/org/snowden/android/app/SnowdenApplication.kt)
пока нет production `ProfileTrustDriver` и accepted runtime source. Это нужно
соединить с существующим whole core. Нельзя разблокировать Connect по наличию
файла, parse-success, marker/hash или успешному импорту в SFA.

Существующий single lifecycle owner, source/generation fences, original deadlines,
borrowed-input custody и bounded RPC lanes сохраняются. Второй controller,
второй Go AAR и временный fake verifier не являются способом ускорить интеграцию.

### Новые серверные сведения

Сообщено о sing-box/nginx на VPS `78.17.84.209` и CDN-домене
`snowdensys.dpdns.org`. Ранее формулировка «домена нет» описывала старый момент.
Сейчас наличие, доступ и применённые настройки рассматриваются как датированное
сообщение оператора, а не новая проверка руководителем.

Важные ограничения для Android-плана:

- SOCKS «3/3» был уточнён как опыт через Amnezia/WARP. Это не приёмка мобильного
  пути без внешнего VPN и не приёмка нашего приложения.
- По телефону сообщены TUN/activity и timeout туннельных потоков в SFA.
  Поднятый TUN показывает запуск части цепочки, но не правильность всех правил,
  DNS, endpoint/TLS и полезного выхода. Причина отказа не установлена одним логом.
- Успех HTTPS и неуспех WS не дают единственного доказанного механизма DPI.
  Ошибка 526 на запасном порту — отдельный неразрешённый TLS outcome.
- В репозиторном [server-combined.json](../../../configs/singbox/server-combined.json)
  остаётся безусловный `hijack-dns`, который оператор связывает с поломкой data path.
  Нужна сверка исправленного шаблона и actual apply, чтобы не вернуть дефект.

Рабочий старый профиль в стороннем клиенте сохраняется как независимый контекст.
Его credentials не копируются в документацию или приложение без принятого
import/approval workflow. Happ на ПК не выключается ради Android-приёмки.

## 2. Дорожная карта

| Этап | Пользовательский результат | Что нужно до него |
|---|---|---|
| ANDR-00 | Понятная совместимость APK/core/profile/server | Текущий source и точный inventory |
| ANDR-01 | Настоящий безопасный импорт профиля | Принятый callable trust C1/T01…14 |
| ANDR-02 | Проверенный ограниченный native/JNI bridge | Exact approved artifact/API identity |
| ANDR-03 | Первый ручной защищённый путь | ANDR-01/02 и пригодная server/channel пара |
| ANDR-04 | Надёжные Stop/revoke/handover/recovery | Владельцы ресурсов и scope результата |
| ANDR-05 | Объяснимые приложения, сайты, DNS и LAN | Принятый effective capture/network contract |
| ANDR-06 | Полноценные страницы и системные действия | Настоящие producers для каждой функции |
| ANDR-07 | Ограниченные память/FD/очереди/фон/энергия | Точный caller и будущий device baseline |
| ANDR-08 | Измеримая адаптация без ослабления защиты | C4/M01…06, real observations и final gate |
| ANDR-09 | Проверенный ограниченный выпуск | Применимые P0, device/field/release evidence |
| ANDR-10 | Выбранные дополнительные возможности | Принятый use case после ANDR-09 |

Подготовка trust/API, bounded JNI conformance без сети и UI на существующих facts
может идти параллельно. Включение Connect ждёт настоящего accepted source и
runtime capability. Подготовка сборки не ждёт весь SDK; пользовательская готовность
не выводится из build success. Проверки и устройства требуют своего разрешения.

## 3. Этапы разработки

### ANDR-00. Зафиксировать реальную пару, не повторять первый APK

**Работа.** Свести текущие STATE/AND/A-FR, whole-AAR source lock, overlays,
generated Java/JNI/AIDL inventory, ABI/toolchain и app packaging. Сохранить
результаты прежнего debug artifact строго в их scope. При изменении native source
получить новую whole-artifact identity, а не второй runtime.

От сервера нужен минимальный compatibility record: endpoint/port, transport,
TLS/SNI/authority, auth role, WS/mux/early-data, UDP/obfs, client/server versions,
scope applied/path результата и rollback. Серверная запись не выдаёт клиенту
trust authority. Неизвестное или несовместимое остаётся unsupported.

**Выход.** Есть один exact набор app/core/API и требования первого канала.
Серверный оператор отдельно сводит исправленный DNS-hijack шаблон с применённым
конфигом. Выбор WS/HY2/httpupgrade/REALITY следует из совместимости и проверки
нужного маршрута, а не из списка доступных protocol names.

**Опора:** [BUILDING](../../../android/BUILDING.md),
[native review](crosschecks/ANDROID-NATIVE.md), [client/server](crosschecks/CLIENT-SERVER.md).

### ANDR-01. Production trust driver и полный offline import

**Работа.** Согласовать T01…14 с реальными callable entrypoints одной реализации
в existing whole libbox. Общий слой проверяет исходные bytes, полный TUF и
application policy; Android оставляет у себя CE/Keystore/source capture,
approval, clock observations, CAS/commit и recovery.

Первый поддержанный контракт должен явно определить:

- независимое одобрение точных root и первой cumulative pin-policy;
- purpose/profile/enroll/recover и недопустимое повышение полномочий входящим bundle;
- accepted predecessor/history, floors, revocation, epochs и ошибки presence/I/O;
- fixed reference time, effectiveEpochMs и exclusive deadlines без фиктивного echo;
- root rotation/reset/continuation и различие restriction-only progress/candidate;
- raw/canonical namespaces, uint64 и фактически полезный bundle в Android256KiB
  intake с учётом encoding overhead. Молчаливое увеличение cap недопустимо;
- borrowed input до actual return и независимо owned outputs; cancel не release.

Для первого consumer предлагается per-return preparation: после реального возврата
обработать eligible progress и отдельно application outcome. Внутренний memory
cache не объявлять durable step barrier. Поддержку более сильного crash/power-loss
обещания нужно согласовать отдельно, сохраняя ограничения полного workflow.

Подключить file picker → bounded inspect → понятный review → exact approval →
повторная verify/apply → платформенный commit. Новое source/material/deadline
инвалидирует review. Progress без accepted application не разрешает Connect.
Неизвестный commit не является пустым enrollment и не повторяется как fresh import.

**Выход.** Настоящий accepted profile/source, привязанный к текущей версии и
истории; ошибки invalid/expired/revoked/unsupported/uncertain различимы.
Только после этого строится accepted-only runtime projection/renderer.

**Опора:** [trust requests](../../android/CHANGE-REQUEST-TRUST-001.md),
[SDK migration](crosschecks/SDK-MIGRATION.md), [trust](sections/TRUST.md).

### ANDR-02. Native/JNI conformance без преждевременного VPN

**Работа.** Проверить generated API, native method calls, bounded transport,
serialization, buffers/FD/IPC и lifetime между app/:vpn/libbox. Использовать
существующие RPC lanes и original actual-return owner, не второй executor.

Prepared source нельзя освобождать до завершения обеих RPC/stub marshalling,
source idle и применимого retirement/process-death receipt. Timeout не даёт
права повторно одалживать данные или запускать нового владельца.

**Выход.** Есть точный JNI/API результат конкретного approved artifact.
Это ещё не Android VPN consent, TUN, правильный routing или protected path.
Начальную conformance-приёмку можно подготовить независимо от готовности VPS.

**Опора:** [native](../../../android/native/README.md),
[corebridge](../../../android/corebridge/), C2/C5 и AND-01/02/11.

### ANDR-03. Первый ручной защищённый путь

**Работа.** Связать accepted source, renderer и capability с явным Connect:
consent → FGS → worker/native → TUN/checking → текущий protected observation.
Авторизовать endpoint/credentials до чувствительной отправки.

Первый режим — минимальный принятый capture с понятными DNS и IPv4/IPv6/UDP
границами. Не начинать со сложного split routing и автоматического fallback.
Данные пользователя допускаются только согласно заявленному protection contract.
`STARTED`, уведомление и VPN icon не дают Ready.

По каждому каналу проверять полный путь отдельно: app/core version, physical
network и upstream VPN, generation, endpoint/TLS, DNS и фактическую прикладную
операцию. HTTP health, WS101, handshake и полезный ответ — разные ступени.
TLS ошибки не обходить `skip verify` или импортом origin private key в клиент.

**Выход.** На одном разрешённом устройстве и названной сети работает хотя бы
один безопасно принятый путь; scope и ограничения показаны пользователю.
Если входные authority/capability/protected facts отсутствуют, Connect остаётся
недоступным или операция заканчивается честным blocked/unknown.

**Опора:** AND-02…07, A-FR-01…06,
[измерения](../../android/MEASUREMENT-NATIVE.md), [network](sections/NETWORK.md).

### ANDR-04. Stop, revoke, смена сети и холодное восстановление

**Работа.** Пройти consent deny/retry/revoke, Stop во время Start, потерю Binder,
service/native death, source expiry/revocation, Wi-Fi↔mobile/resume и поздние
callbacks. Старый generation/proof не переносится на новый network context.
Использовать одного владельца с контролируемым restart и bounded backoff.

Сохранить original deadlines, фактический возврат/смерть владельца и закрытый
допуск при неопределённой очистке. Quiet mode не подавляет safety.

Always-on, lockdown, reboot, direct boot и sticky restart — отдельный последующий
срез после принятого cold-recovery workflow. До него сохранять нынешние
`SUPPORTS_ALWAYS_ON=false`, `directBootAware=false`, `START_NOT_STICKY` там,
где они ещё являются действующим контрактом. Перед изменением перечитать source.

**Выход.** Stop/revoke/recovery outcomes проверены в заявленном режиме;
не обещана непрерывность звонка или старого TCP без отдельного доказательства.
Notification permission/display не подменяет состояние FGS/защиты.

**Опора:** [VpnService boundary](../../../android/vpn/README.md),
AND-02/03/05/11, A-FR-01/02/04/10.

### ANDR-05. Приложения, DNS, сайты и рабочие сети

**Работа.** Расширить existing rule compiler и network callbacks:

1. Package/UID: install/uninstall/replace/reinstall, shared UID, current package
   identity и cache invalidation. Неизвестная/неоднозначная identity не direct.
2. Per-app capture: точная supported matrix, work/private user isolation,
   управляемая policy и фактически доступная локальная inventory. Не обещать
   управление чужим профилем или обход MDM.
3. DNS: gateway bootstrap отдельно от protected user resolver; context/family/
   authority/cache bound, Private DNS/split DNS и корректный отказ при неизвестном.
4. Family/MTU: IPv6-only, NAT64/DNS64, UDP/TCP, PMTU и handover без наследования
   чужого path proof. Каждая supported клетка требует своей приёмки.
5. Sites/IP/LAN: effective action, precedence и ограниченные временные grants.
   Portal handoff не является общей кнопкой отключить protection.

**Выход.** Экран показывает, что реально захватывается/маршрутизируется,
причину и границы. Сохранённые правила не считаются применёнными до platform
receipt. Unsupported режим не заменяется незаметным direct fallback.

**Опора:** [UID](../../theory/android/UID-ROUTING-AND-OWNERSHIP.md),
[proxy/DNS/NAT64](../../theory/observability/PROXY-DNS-NAT64.md), AND-05/06/10.

### ANDR-06. Закончить продукт по страницам и сценариям

Сохранить один Compose shell, compact bottom navigation/rail и нынешние
стабильные destinations. Верхние табы — подпункты раздела, не повтор меню.
Широкий экран использует пространство осмысленно; телефон не копирует desktop.

| Раздел/страница | Конечная ответственность |
|---|---|
| Главная | Один primary action, Stop, named protection/path state, причина и следующий шаг |
| Подключения → Сценарии | Сохранить draft отдельно от Apply; результат применения и ограничения сценария |
| Подключения → Серверы | Declared/eligible/observed отдельно; никаких выдуманных стран или RTT |
| Правила → Приложения | Доступная inventory, identity confidence, effective capture/action |
| Правила → Сайты | Предсказуемая область правила, DNS dependency, conflict/unknown |
| Правила → LAN | Явные local/portal grants с контекстом, сроком и отзывом |
| Система → Диагностика | Названные bounded проверки с scope, freshness и безопасным следующим действием |
| Система → История | Причины и redacted timeline; preview ровно экспортируемых данных |
| Система → Настройки | Профиль/approval, notification/permission recovery, бюджеты и update/recovery outcomes |

Системные surfaces — notification/actions, tile, widget и shortcuts — используют
один presentation source и current-generation action. Lock screen, stale widget
и перезапуск Activity не повышают полномочия. Save не равно Active; blocked/error
и unknown имеют разные объяснения. Секреты и сырой конфиг не попадают в export.

Сквозные потоки: импорт → approval → первый Connect; deny/retry/revoke;
смена сети; рабочие приложения/LAN; source expiry; восстановление; export/help.
Для каждого — loading/empty/error/unknown, Back/restore, TalkBack/keyboard,
large text/contrast, rotation/compact/wide и понятное действие без лишних permissions.

**Выход.** Девять страниц наполнены реальными producers и проверены по
сценариям. До producer готовность не изображается placeholder-значениями.
Уже проверенные navigation cases сохраняются; full usability не объявляется
по скриншотам или по одной автоматической semantics-проверке.

**Опора:** [Android UX](../../theory/android/PLATFORM-AND-UX.md),
[product](crosschecks/PRODUCT.md), AND-08…12 и A-FR-07…11/17.

### ANDR-07. Ресурсы, фон и энергия

**Работа.** До clone/spawn/cache admission учитывать count+bytes, deadlines,
FD/IPC/borrowed buffer owners и actual return. Bounded histories/diagnostics
не конкурируют с обязательными safety/control операциями.

Разделить metered/roaming, optional probe budgets и обязательный recovery.
Keepalive, pause/wake, wakelock и battery exemption не вводить как универсальный
speed/reliability preset. Сначала working protected path, затем device baseline
с условиями экрана/радио/нагрузки и сравнением по одинаковому workload.

**Выход.** Приняты структурные ограничения и их проверочные сценарии;
пользователь видит настоящие known/unknown data/resource facts. Численная
экономия батареи, hard RSS и latency требуют измерения, а не константы в UI.

**Опора:** [performance](sections/PERFORMANCE.md), AND-11 и A-FR-10.

### ANDR-08. Android observations и ограниченная адаптация

**Работа.** Сопоставить M01…06/API1 с actual Android producers. Нужны current
source/owner/network/config/image/path references, exact clock domain/unit/boot,
freshness/deadline, method/family/workload, typed outcome, missing/censoring
и custody. Checked elapsed-ms conversion не превращает elapsed в UTC.

Rule compiler не является consumer C2/ranking. Actor recovery не заменяется
общей policy. Latest health не даёт полный cohort; пропущенные и suppressed
попытки не исчезают из смысла измерения. Не создавать telemetry SDK/БД для
процента успеха, пока нет настоящего task producer и цели.

Первым подключать один обоснованный эффект, например suppression optional
background work по coherent protected pressure snapshot. Неизвестный pressure
не даёт ни suppression, ни admission grant. Stop/revoke/recovery идут своим
детерминированным путём; каждый эффект получает final platform recheck.

**Выход.** Приняты версия producer/adapter и один отключаемый consumer с
baseline/parity/negative gates. `RequiresPlatformGate` не теряется при JNI/FFI.
Дальнейший ranking/cooldown выбирается отдельным срезом после сопоставимых
observations и измерения пользы/цены переключений.

**Опора:** [adaptive API](../../adaptive/API.md),
[adapter matrix](../../adaptive/ADAPTER-MATRIX.md),
[measurement requests](../../android/CHANGE-REQUEST-TRUST-002-MEASUREMENTS.md).

### ANDR-09. Проверки, выпуск и эксплуатация

| Gate | Конкретный следующий объём |
|---|---|
| Local/source | Текущий profile/app/core cut, реальные negative cases, build/lint по отдельному разрешению |
| JNI/native | Approved whole-AAR, ABI/API/borrowed lifetime/FD/IPC/fault/actual return |
| Device/OS | Consent/FGS/revoke/death/reboot/cold recovery, разные API/OEM, 16KiB runtime отдельно от ZIP alignment |
| Сеть | Названный канал и physical/upstream context; DNS/IPv4/IPv6/UDP; handover и отказ origin/CDN |
| Пользователь | Полные import/Connect/Stop/rules/recovery/support потоки, доступность и понимание ошибок |
| Release | Release signing/update continuity, R8/ABI/SBOM/NOTICE/privacy disclosures, staged rollout и support |

Первый ручной protected path не обязан ждать всех будущих оптимизаций.
Публичный релиз обязан пройти каждый применимый P0. Debug signature не release
signing; installed APK не VPN readiness; connected icon не защита.

Для РФ field/comparative опыта заранее зафиксировать сети/AS, период, workload,
artifact/profile/channel и способ проверки upstream VPN. Не устранять зависимость
от Happ/Amnezia на рабочем ПК без разрешения: использовать отдельно согласованную
тестовую среду. Положительный контроль и один timeout не дают country-wide
вердикта о блокировке или гарантии превосходства над всеми VPN.

Отдельный device credential, revoke, encrypted backup и recovery kit остаются
задачами выпуска/эксплуатации; восстановление сервера не сбрасывает client history.
Начать с ограниченного RC и поддерживаемой matrix, затем расширять на основании
реальных результатов и безопасного возврата к известной версии.

### ANDR-10. Дополнительные возможности — отдельное решение

После полноценного клиента выбрать доказанно полезный use case для новых
transport/privacy механизмов, multihop/bonding/port forwarding/TV/router.
Не переносить их в обязательный P0 автоматически. Для каждого нужны польза,
совместимость, ресурсная цена, threat assumptions, поддержка и отдельный go/no-go.

## 4. Общие контракты и минимальная координация

- **Trust C1/T:** SDK публикует точную callable семантику; Android принимает
  её через существующий whole core. Платформа владеет storage/clock/approval/commit.
- **Artifact C2/C5:** один runtime в одной сборке, exact API/ABI identity и actual
  custody. Новые immutable версии разрешены через новую приёмку, не вторым AAR.
- **Server C3/C7:** оператор даёт versioned capability/apply/rollback факты;
  клиент сам проверяет профиль, endpoint authorization и свой path scope.
- **Measurements C4/M:** платформа даёт реальные факты, pure policy рекомендует,
  платформа исполняет после current recheck. Никакого authority из health label.
- **UI C6:** один presentation producer, отдельно action outcome. Android и ПК
  согласуют смысл терминов, но не общий UI framework.

Один Android-владелец интегрирует app/profile/vpn/corebridge/native. Параллельные
дополнительные чаты Android2/3/БД не нужны. Trust/adaptive/server получают только
конкретные prerequisites; все-платформенный barrier для небольшого source-среза
не вводится. Существующая регистрация/архивирование чатов не меняются этим планом.

## 5. Сохранение требований и разумное заимствование

| Действующий объём | Новый маршрут |
|---|---|
| AND-00/01, A-FR-13 | ANDR-00/02/09 |
| AND-02/03, A-FR-01/02 | ANDR-03/04 |
| AND-04, A-FR-03 | ANDR-01; recovery также ANDR-04/09 |
| AND-05/06, A-FR-04/05 | ANDR-04/05 |
| AND-07, A-FR-06 | ANDR-03/08 |
| AND-08/09/10, A-FR-07/08/09/17 | ANDR-05/06 с отдельным scope grants |
| AND-11/12, A-FR-10/11 | ANDR-06/07 |
| AND-13/14, A-FR-12/13 | ANDR-01/09 |
| AND-15/16/17, A-FR-14/15/16 | ANDR-09 |
| AND-18, A-FR-18 | ANDR-10, отложено до выбранной пользы |

Это маршрутизация, не новая копия статусов. Все прежние acceptance и
незакрытые platform/device/field gates остаются в действующем реестре.

Переиспользовать собственные Compose navigation/components и Android lanes,
один full libbox и общий verifier. Из уже изученных SFA/Mullvad/Proton/Amnezia
брать только обоснованный pattern, не второй stack/controller, vendor backend
или лишние разрешения. Для копируемого кода проверить exact pin, лицензию,
NOTICE/corresponding-source/transitive состав, совместимость и цену сопровождения.
Для Android adaptive layout/optional QR использовать уже рассмотренные варианты,
не внедрять новую библиотеку по одному названию.

## 6. Ближайшие законченные срезы

1. Согласовать production C1/T и useful intake; подключить настоящий driver и
   accepted-only source в существующий whole core.
2. Независимо подготовить ограниченный JNI conformance без VPN/сети и доработать
   UX import/unknown/blocked на настоящих types.
3. С серверным оператором зафиксировать исправленную applied конфигурацию,
   TLS/capability facts и один пригодный путь в отдельно согласованной среде.
4. После входных gates пройти manual consent → protected path → Stop/revoke.
   Только затем расширять рабочие правила, recovery и adaptive эффекты.

При подготовке плана не запускались новые тесты/сборки/ADB/VPN/SSH/Cloudflare
операции. Ранее разрешённые результаты Android сохранены в исходной области.
