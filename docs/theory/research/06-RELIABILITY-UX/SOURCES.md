# R06 — источники, закреплённые снимки и область чтения

Проверено **2026-10-25**. Только первичные документы/публикации и public source.
Это реестр оснований [REPORT](REPORT.md), не утверждение о runtime-приёмке.
Номера E — локальные ссылки исследования, не требования продукта.

## 1. Внешние первоисточники

| ID | Источник / просмотренный предмет | Что обосновывает и предел |
|---|---|---|
| E01 | [Google SRE Workbook — Implementing SLOs](https://sre.google/workbook/implementing-slos/), разделы SLIs, critical user journeys, зависимости | Метрика по пользовательскому исходу; спецификация и измерительная реализация различаются. Не задаёт targets Snowden и не доказывает нашу availability. R06 модель S/F/U — отдельный собственный вывод |
| E02 | [AWS Builders' Library — Static stability using Availability Zones](https://aws.amazon.com/builders-library/static-stability-using-availability-zones/), Static stability / control-data plane | Полезно заранее иметь нужное для переживания отказа; немедленный provisioning создаёт recovery dependency. Принцип переносим, AWS/multi-AZ архитектура для личного VPS не выбрана |
| E03 | [Tailscale Changelog](https://tailscale.com/changelog), записи **2026-10-07 v1.104.1**, **2026-10-21 v1.104.2** | Первое объявляет netmap caching отдельно для Linux/Windows/macOS. Второе — исправления Windows peer/DNS regressions и Go1.27.3. Это ограниченный release snapshot, не доказательство Android cache и не разрешение обновлять наши зависимости |
| E04 | [Tailscale — We're making Tailscale faster](https://tailscale.com/blog/making-tailscale-faster), Faster startup with netmap caching / rollout limits | Cache для ранее подключённого устройства, persistent storage, ограничения I/O; mobile описан как более поздний rollout. Заявленные gains автора не переносятся на Snowden и не используются как наши измерения |
| E05 | [Tailscale — Control and data planes](https://tailscale.com/docs/concepts/control-data-planes), coordination outage, Last validated 2026-01-05 | Различие control/data dependencies и продолжение ранее установленных связей из cache. Это высокоуровневый документ; более поздний E03 уточняет cold-start возможность по платформам. Не переносить отсутствие новых connections из старого описания как универсальное правило после release |
| E06 | [Mullvad desktop — Using the Mullvad VPN app](https://mullvad.net/en/help/using-mullvad-vpn-app), updated **2026-09-18**, Kill switch / Lockdown / API access | Явные manual Disconnect/Quit semantics, desktop split exception и отдельные API access методы. API reachable не означает доступность полезной цели. Не копировать автоматический Direct API fallback в общий protected path |
| E07 | [Mullvad Android](https://mullvad.net/en/help/using-mullvad-vpn-on-android), updated **2026-08-12**, lockdown/LAN/split/OS settings | Документированные ограничения excluded apps и LAN при системной блокировке; отличие платформенных возможностей. Не all-OEM тест и не доказательство отсутствия любого OS-originated трафика |
| E08 | [Android Developers — VPN](https://developer.android.com/develop/connectivity/vpn), service/user-profile/always-on/blocked connections | Один активный VPN service на user/profile, системные consent/always-on/lockdown, исключённые apps при blocking могут терять связность. Жизненный цикл ОС не устанавливает VPN gateway connection за клиента |
| E09 | [Android — Read network state](https://developer.android.com/develop/connectivity/network-ops/reading-network-state), capabilities / captive portal / dynamic change | INTERNET — configuration; VALIDATED и CAPTIVE_PORTAL — последние OS observations. Они не равны свежей VPN/service проверке. [NetworkCapabilities reference](https://developer.android.com/reference/android/net/NetworkCapabilities) просмотрена как дополнительный API указатель |
| E10 | [AOSP ConnectivityManager.java, commit 627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a](https://android.googlesource.com/platform/packages/modules/Connectivity/+/627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a/framework/src/android/net/ConnectivityManager.java), `startCaptivePortalApp` около строк 5710–5754 | `@hide`, module SystemApi и privileged network permissions. Проверены обе overload и Apache-2.0 file notice. Относится к этому AOSP16 snapshot, не device/OEM тест. Public ConnectivityManager reference не использована для выдуманного публичного метода |
| E11 | [Android Settings.ACTION_WIFI_SETTINGS](https://developer.android.com/reference/android/provider/Settings#ACTION_WIFI_SETTINGS) | Публичный переход к Wi-Fi settings; нет результата login; matching Activity может отсутствовать. Это handoff, а не гарантированное выполнение captive authentication |
| E12 | [Mullvad error_state.rs, pin 52d4ab90aff4eb7d9455bfad071d144220b3a054](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/talpid-core/src/tunnel_state_machine/error_state.rs), `enter/set_firewall_policy` | Причина ошибки отдельно от `block_failure`; platform-specific применение policy. Root [LICENSE.md](https://github.com/mullvad/mullvadvpn-app/blob/52d4ab90aff4eb7d9455bfad071d144220b3a054/LICENSE.md) GPL-3.0 просмотрена. **Reference** семантики, без копирования Rust/FFI/driver stack и без полного аудита репозитория |
| E13 | [Tailscale captivedetection.go, pin 47743a8b9ad16cf3a49504dc443208c37748072e](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/net/captivedetection/captivedetection.go), `NewDetector/Detect/verifyCaptivePortalEndpoint/dialContext` | Короткие запросы, запрет redirects, interface binding; `false` также при errors. [LICENSE](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/LICENSE) BSD-3-Clause; [go.mod](https://github.com/tailscale/tailscale/blob/47743a8b9ad16cf3a49504dc443208c37748072e/go.mod) объявляет Go1.27.3. **Adapt** отдельных идей/typed outcomes, не импорт SDK/DERP endpoints/logger в Go1.26 проект |
| E14 | [W3C WCAG2.2 Understanding SC4.1.3 — Status Messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html) | Существенный status доступен assistive technology без focus theft; чрезмерно частые alerts мешают. Informative explanation к WCAG, не сертификация нашего UI. Android требует native semantics, HTML роли не его API |
| E15 | [Binkhorst et al., USENIX Security 2022 — VPN Mental Models in a Corporate Context](https://www.usenix.org/conference/usenixsecurity22/presentation/binkhorst), [paper](https://www.usenix.org/system/files/sec22-binkhorst.pdf), abstract/метод и границы выборки | Качественное explorative исследование 11 experts/7 non-experts в корпоративном контексте Нидерландов. Обосновывает осторожность с mental models; не репрезентативная статистика РФ и не validation предлагаемой UI-схемы |
| E16 | [Akgul et al., USENIX Security 2025 — As Advertised?](https://www.usenix.org/conference/usenixsecurity25/presentation/akgul), abstract/описание метода | 217 участников, exposure/mental-model associations. Не причинный эксперимент над нашим UX; no significant correlation по некоторым mental models нельзя превратить в доказательство бесполезности объяснений |
| E17 | [Android SystemClock](https://developer.android.com/reference/android/os/SystemClock), clock overview, updated **2026-08-03** | elapsedRealtime включает deep sleep, uptimeMillis исключает. Основание явно определить время эпизода и boot domain; не доверенный clock для signed expiry |
| E18 | [Microsoft QueryUnbiasedInterruptTime](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryunbiasedinterrupttime), description/remarks | Этот clock исключает sleep/hibernation. Не утверждение о конкретном API, которым сейчас пользуется весь Snowden; не замена анализа Windows clock semantics |
| E19 | [Microsoft NCSI overview](https://learn.microsoft.com/en-us/windows-server/networking/ncsi/ncsi-overview), functionality/proxies/captive; [troubleshooting guidance](https://learn.microsoft.com/en-us/troubleshoot/windows-server/networking/troubleshoot-ncsi-guidance) | OS connectivity indicator зависит от своих probes/окружения; не VPN egress/task attestation. Server-labelled docs — механизм/ограничения, не доказанная support matrix каждого Windows client build |

Только результаты указанных чтений использованы как основания. Неразобранные
поисковые находки, marketing comparison и GitHub issue reports не повышены до
доказанных свойств. NIST SP800-34 metadata просмотрены как фон contingency
planning; полный текст здесь не разбирался и новых требований из него не выводилось.

## 2. Public source captures и provenance

Снимки сохранены в ignored
[.artifacts/research/theory-06](../../../../.artifacts/research/theory-06).
Это чужой публичный текст для чтения, не добавленные зависимости/fixtures и не
код, выполненный при исследовании. SHA256 относится к сохранённым UTF-8 bytes.

| Файл capture | Pin / provenance | SHA256 |
|---|---|---|
| `aosp16-ConnectivityManager.java` | tag `android-16.0.0_r1` разрешён официальным Gitiles JSON в commit `627eb3c63e7c7cb3cb9f8ea319e4bb1676079c3a`; повторное чтение exact commit дало те же bytes | `9221bfc0177a0cf90ba8891f463f27daf0a30678fb9d425740bd2bed986375b6` |
| `mullvad-error_state.rs` | raw exact E12 commit/path | `1d559eb169f841ddd84548b7156b641f7393572d23ef05bb6fb7fd9dacd174a6` |
| `mullvad-LICENSE.md` | raw E12 commit/root license | `8951bda6e616df7418ff3f80d9699f96da704df64f3bc9f0d5bb75a103680456` |
| `tailscale-captivedetection.go` | raw exact E13 commit/path | `518a0264c805fc1b7bc741e6054b0d67749648b3cd0c3829b397b113f7141907` |
| `tailscale-LICENSE` | raw E13 commit/root license | `a7ca6186a7963a0a60740f6047760eecd7a0234e8c38bd7e1e0bbcb324bda45b` |
| `tailscale-go.mod` | raw E13 commit/manifest | `0a1b5bff8d2bdf55c15075fa1d35e706c3103d08475f6b19935a545181f17bdc` |

Gitiles ref metadata: `aosp16-ref.json`; immutable comparison metadata:
`aosp16-immutable-source.json`. Gitiles HTML оказался недоступен через web tool;
официальный text endpoint прочитан обычным HTTPS-запросом, base64 декодирован
как данные. Remote code не запускался. Остальные документы просмотрены web tool;
их URL/date не обещают immutable webpage bytes.

Лицензия рассматривается **в пределах просмотренных файлов**. Перед будущим
копированием/распространением нужны applicable file notices, provenance и
совместимость всего выбранного состава; reference решения не означает
разрешение переносить branding/assets/полный стек.

## 3. Локальные inputs и точная область source review

Начальные fingerprints: [inputs-start.json](../../../../.artifacts/research/theory-06/inputs-start.json).
Дополнительные dated fingerprints: [inputs-extra.json](../../../../.artifacts/research/theory-06/inputs-extra.json).
Это метаданные прочитанных/навигационных входов, не снимок всего дерева.
Наличие hash не означает полный аудит каждой строки.

| Вход | Что читалось / роль |
|---|---|
| [THEORY](../../README.md) | §7.3/5, §12.8, §13.10, §15.3/5–6, §16.4–6, §17.3–4 и точечный поиск overlap. Это baseline, не новый полный аудит всего сборника |
| [PLAN](../../../PLAN.md) | §5.4 COM/контракты и точечные требования recovery; продуктовые статусы не изменялись |
| [ANDROID-PLAN](../../../ANDROID-PLAN.md) | State model, A-FR/AND ownership, AND-08…12, §5 UX flows/usability, текущая шапка. На чтении: AND-12-S5/AND-03-S8 source unverified; не бинарная приёмка |
| [COMPETITOR-FEATURES](../../ux/COMPETITOR-FEATURES.md) | §6 baseline/reuse pins/comfort flows; старые superiority claims не использованы |
| [configs/README](../../../../configs/README.md), [INFRASTRUCTURE](../../../INFRASTRUCTURE.md) | Template/production parity disclaimer и post-verify contract; датированные service/domain/failure-domain metadata. Не secrets и не live read |
| [netwatch](../../../../windows/netwatch.go), [autoretry](../../../../windows/autoretry.go) | `run/tunRequested` guard, delivery branches/существующая retry scheduling и custody guards; не весь Windows lifecycle |
| [Dashboard](../../../../windows/frontend/src/views/DashboardView.vue), [contract](../../../../windows/frontend/src/api/contract.ts) | Hero lifecycle/protection wording, failClosed/retry, AppState/probe timestamps. Рендер/сборка/browser не проверялись |
| [ConnectionState](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/ConnectionState.kt), [ConnectionPresentation](../../../../android/app/src/main/kotlin/org/snowden/android/app/ui/ConnectionPresentation.kt), `NotificationPresentation.kt` (исторический путь R06-001; позднее отсутствует, см. §4.2) | Phase/issue/scope, no invented READY, observation dating и resource projection |
| [PhysicalNetworkState](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/PhysicalNetworkState.kt), [PhysicalNetworkMonitor](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/PhysicalNetworkMonitor.kt) | OS hints/epoch/context, portal wait reason, process-scoped selection; не path attestation |
| [VpnSessionRepository](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/VpnSessionRepository.kt), [SessionJournal](../../../../android/vpn/src/main/kotlin/org/snowden/android/vpn/SessionJournal.kt) | Точечные `emit/retry/recovery/clearHistory`, bounded process-local journal, не production profile/verifier driver |
| `android/STATE.md`, `windows/backend/core/manager.go`, Android `HomeScreen/SystemScreen` | Навигационные/status fingerprints и выбранные presentation declarations; не источник заявления о готовности всего backend/Android |

Исходное дерево untracked. Own write set: только REPORT/SOURCES/STATE/handoff
в каталоге R06 и artifacts своего исследования. Чужие общие документы/source
могут меняться параллельно; итоговое сравнение hashes и смысл drift фиксируется
в сохранённых input manifests. Не переносить старый source review на новые bytes.

## 4. R06-002 — новые основания глубокого S5 среза

**2026-10-25.** E01…19/пакет R06-001 сохранены, scientific ACCEPTED не присвоен.
Exact baseline до дополнения: ignored `cycle-002/baseline-R06-001-REPORT.md`
и `…-SOURCES.md`. Новая часть REPORT — §9…15, новый handoff — R06-002.

| ID | Первичный источник / область чтения | Основание и предел |
|---|---|---|
| E20 | [Dechter/Meiri/Pearl, Temporal constraint networks, AI49,1991, DOI10.1016/0004-3702(91)90006-6](https://www.sciencedirect.com/science/article/pii/0004370291900066), publisher abstract/metadata | Интервальные constraints/simple-general distinction. Полный35-page paper не audited: university PDF fetch timed out. Наши inequalities/units — ограниченный собственный derivation, не реализация алгоритма статьи |
| E21 | [Vidal, A unified dynamic approach for dealing with Temporal Uncertainty and Conditional Planning, AIPS2000](https://cdn.aaai.org/AIPS/2000/AIPS00-044.pdf), abstract/background/temporal-contingency definitions | Consistency не тождественна online реакции на уже наблюдённые contingent durations. Не переносить controllability proof на наш AND/OR/security state model; solver/production API не выбран |
| E22 | [TUF Specification1.0.37](https://theupdateframework.github.io/specification/v1.0.37/index.html), modified **2026-10-08**, §5.1/5.2, expiry workflow/§7.1 | Fixed update-start, out-of-band trusted root; expired root может участвовать в root-update chain с последующим workflow expiry gate. Не trusted UTC/application-expiry waiver и не реализованный bogus-clock mode из future-work heading |
| E23 | [Restic v0.19.1 immutable tree](https://github.com/restic/restic/tree/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf), exact files ниже | CLI restore→OpenRepository/password/key→lock writes→restore/optional verify. BSD2-Clause; go1.25.8/toolchain1.25.10. Candidate external CLI/operator reuse, не embedded SDK/APK; полный security/crypto/backend audit не выполнен |
| E24 | [Restic0.19.1 Restoring from backup](https://restic.readthedocs.io/en/v0.19.1/050_restore.html), in-place/overwrite/platform privileges | Interrupted restore может оставить partial files; Windows metadata privileges отдельны. Не доказывает атомарное применение Snowden/trust/server config; restore drill не проводился |
| E25 | [Restic Working with repositories](https://restic.readthedocs.io/en/stable/045_working_with_repos.html), page declares0.19.1; pinned `doc/045_working_with_repos.rst` | Structure check отличается от чтения pack data с дополнительным I/O/egress. Integrity не равна полноте необходимых файлов, свежести authorization и работоспособности приложения. Команды не запускались |
| E26 | [Microsoft CryptProtectData](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata), scope/remarks | Обычно те же logon/user и компьютер; есть roaming exceptions. Согласуется с user-scope source Snowden, но не доказывает переносимость реального vault. DPAPI operations не выполнялись |
| E27 | [Android Support Direct Boot](https://developer.android.com/privacy-and-security/direct-boot), CE/DE/first-unlock/manifest | CE до первого unlock после reboot недоступен; повторный lockscreen после unlock не то же состояние. DE не общий store credentials. Current manifest не direct-boot-aware; ничего не менялось |

Для сверки терминологии также прочитаны индексированные abstract/введение
первичной Morris/Muscettola/Vidal «Dynamic Control of Plans With Temporal
Uncertainty». Complexity/algorithm guarantees в модель не переносились;
основное цитируемое contingency основание — E21. E02/E03 первого цикла остаются
reference static stability/cache, не новым обещанием готовой интеграции.

### 4.1. Restic pin, actual callers и captures

Official GitHub ref `v0.19.1` → annotated tag
`6099d4b5c4376368d42b9982c7d3e93ba13d83b1` → commit
`6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf`.
Tagger date **2026-07-05** — tag metadata, не вновь установленная дата binary
release. GPG/signature validation не выполнялась.

Все пути — в [этом immutable commit](https://github.com/restic/restic/tree/6aa3a516ce654808a1f28f9fa21e9b7c8e6e90bf):

| Путь/symbol | Прочитанный механизм |
|---|---|
| `cmd/restic/cmd_restore.go`: `newRestoreCommand/runRestore` | RunE caller, target, openWithReadLock, snapshot selection, NewRestorer, RestoreTo, conditional VerifyFiles |
| `cmd/restic/lock.go`: `openWithReadLock/internalOpenWithLocked` | OpenRepository → repository.Lock; noLock branch, unlock; read lock не значит отсутствие metadata mutations |
| `internal/global/global.go`: `OpenRepository/decryptRepository` | Backend/config access, password acquisition, Repository.SearchKey, bounded tries/errors; не env/credentials пользователя |
| `internal/repository/repository.go`: `(*Repository).SearchKey`, около853 | Реальный receiver → package SearchKey → key installation/LoadConfig/error restoration |
| `internal/repository/key.go`: `SearchKey/OpenKey/LoadKey` | Bounded key attempts и backend dependency; только чтение, без KDF/decrypt computations |
| `internal/repository/lock.go`: `Lock` | restic.NewLock, retry/ownership context; не latency bound backend |
| `internal/restic/lock.go`: `NewLock/createLock/Unlock` | SaveJSONUnpacked(LockFile) и RemoveUnpacked — обоснование write/remove prerequisite |
| `go.mod`, `LICENSE` | go1.25.8/toolchain1.25.10, BSD2-Clause; полный binary/dependency graph не проверялся |
| `doc/045_working_with_repos.rst`, `cmd/restic/main.go` | Documentation comparison/CLI navigation, не дополнительный audited call graph |

**11 public text captures** в [cycle-002](../../../../.artifacts/research/theory-06/cycle-002).
SHA256/bytes: [public-captures.json](../../../../.artifacts/research/theory-06/cycle-002/public-captures.json);
GitHub ref/tag/tree metadata: `restic-ref.json`, `restic-tag.json`,
`restic-selected-tree.json`. Hashes — source metadata, не crypto release
verification или проверка crypto реализации. HTTPS text прочитан как данные;
CLI/Go/backup/restore/check не запускались. File/license scope не лицензирует
автоматически весь будущий binary состав/branding/assets.

### 4.2. Local inputs и узкий review source drift

В ignored cycle-002: `inputs-start.json` (18 путей), `inputs-extra.json`
(ещё4), `inputs-reread.json` (2 targeted rereads и2 новых caller/consumer),
`inputs-end.json` (сравнение первоначальных22); финальная актуальность — STATE.
24 уникальных selected inputs, не snapshot всей codebase/ОС.

Windows: ApplyProfileRecovery → import/authority → prepareProfileRecovery →
sourceFence/transaction; render snapshot → LoadRuntimeProfile; clock →
ObservedClock; secretvault → user-scope DPAPI declarations. Просмотрены exact
файлы из REPORT §9.2, relevant branches; не реальные vault/profile/runtime и
не product tests. Shell server source прочитан, env/secrets не открывались.

Android после drift перечитан узко: OsProtectionReader.read(service,isCurrent)
nullable при отмене local admission; caller WorkerProcess.observeOs проверяет
retirement/authority и публикует dated worker facts; consumer VpnSessionRepository
→ ProtectionState.from. Null cancellation не повышена до OS denial/successful
unknown read. Compile/native/device proof этим чтением не создаётся.

PLAN drift: адресно повторно просмотрены COM-13/14/15; их mapping сохранён.
Остальной новый PLAN не reviewed целиком, новые продуктовые статусы не приняты.
Configs deploy/WS/NGINX перечитаны как offline source, не выполнены. Последующие
изменения inputs требуют новой оценки актуальности.

R06-002 final review обнаружил исчезновение прежнего
`android/vpn/src/main/kotlin/org/snowden/android/vpn/NotificationPresentation.kt`.
Единственная maintenance-правка ранней части SOURCES: исторический путь оставлен
текстом, не нерабочей ссылкой на якобы текущий файл. Исходная версия SOURCES
целиком сохранена в baseline-R06-001 capture; научные выводы первого пакета не
переписаны. Новая notification реализация здесь не audited. Header Android
STATE/PLAN обновился до AND-11-S3 source unverified; он перечитан как status,
не доказательство новых source/native gates. Детальный review новых изменений
планов не заявляется.

## 5. R06-003 — непрерывность, cutoff и текущие callers

Просмотрено 2026-10-26. Срез REPORT §16…21; никакие product/network процессы
не запускались. Следующие ссылки — первоисточники, не runtime claims Snowden.

| ID | Источник / просмотрено | Что можно переносить и предел |
|---|---|---|
| E28 | [RFC9293](https://www.rfc-editor.org/rfc/rfc9293.html), August2022, TCP terminology/connection pair of sockets | Идентичность TCP/state. Не утверждает, что изменение underlay всегда меняет inner tuple или непременно рвёт TCP |
| E29 | [RFC9000](https://www.rfc-editor.org/rfc/rfc9000.html#section-9), May2021, §§8.2,9,9.6 | CID/path validation/handshake/disable_active_migration/client migration и preferred_address exception. Не перенос крипто/state на произвольный независимый сервер, не multipath SDK |
| E30 | [WireGuard conceptual overview — Built-in Roaming](https://www.wireguard.com/#built-in-roaming) | Endpoint known peer обновляется из authenticated traffic. Это reference поведения; новый peer/backend или сохранность NAT на умершем VPS не установлены |
| E31 | [sing-box v1.13.19 release](https://github.com/SagerNet/sing-box/releases/tag/v1.13.19) → [commit b5ebaa1fc0f2b94256180b95468e73ef53caa27d](https://github.com/SagerNet/sing-box/commit/b5ebaa1fc0f2b94256180b95468e73ef53caa27d); [selector docs](https://sing-box.sagernet.org/configuration/outbound/selector/) | Actual selector/interrupt source ниже; docs field semantics corroborates. Release existence/GitHub signature display не заменяет собственной release security проверки; deps не менялись |
| E32 | [VpnService.setUnderlyingNetworks](https://developer.android.com/reference/android/net/VpnService#setUnderlyingNetworks(android.net.Network[])), API22+, doc last updated2026-09-16 | Объявляет actual underlying networks системе; не является transfer открытого socket. В проекте protect/bind и pinned physical context — отдельные операции |
| E33 | [Mosh — How Mosh works](https://mosh.org/#techinfo) | Application state synchronization/UDP roaming как пример ограниченной terminal continuity. Не VPN drop-in; релиз/pin/код/новая dependency не выбирались. Вывод о пригодности крипто здесь не делается |
| E34 | [RFC9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2), June2022 | Idempotency и ограничения automatic non-idempotent retry; общий VPN не знает remote outcome encrypted application operation |

### 5.1. Immutable source capture и license

Файлы pinned E31:

- [protocol/group/selector.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/group/selector.go): NewSelector options → SelectOutbound → DialContext/ListenPacket, NewConnectionEx/NewPacketConnectionEx и direct-route delegation.
- [common/interrupt/group.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/interrupt/group.go): registration mutex, Interrupt iteration/external filter.
- [context.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/interrupt/context.go) и [conn.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/interrupt/conn.go): external marker и registered wrapper lifetime.
- [option/group.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/option/group.go): InterruptExistConnections bool.
- [LICENSE](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/LICENSE): GPL-3.0-or-later. Existing dependency reuse; лицензия не переопределяется, новых вставок чужого кода нет.

Ignored `.artifacts/research/theory-06/cycle-003/` содержит шесть `singbox-*`
copies из существующего module cache и шесть `upstream-*` raw captures exact
commit. `immutable-pin-comparison.json`: **6/6 SHA256 совпадают**; manifest
`singbox-captures.json` сохраняет значения. `.info` cache содержит version/time,
не Origin.Hash. Две GitHub API попытки timeout; ранний `singbox-pin.json` с null
sha оставлен как failed lookup, **не используется как successful pin**.
Commit установлен из release link, затем подтверждены фактические bytes;
криптографическая подпись релиза/модуля не проверялась. Go/toolchain не запускались.

### 5.2. Локальные inputs и актуальность

19 selected local paths: `inputs-start.json`15 + `inputs-extra.json`4.
`inputs-end.json` повторяет15 стартовых fingerprints; на срезе 06:11UTC drift
среди них не обнаружен. Extra4 прочитаны/закреплены позднее; дальнейшая
актуальность после capture не обещается. Baseline REPORT/SOURCES cycle-002
сохранены в cycle-003 до дополнений; прежние пакеты и handoffs не удалялись.

Ключевые actual chains (от корня проекта):

- `windows/corebridge.go` SelectChannel → `windows/failover.go` switchChannelCoreFor/selectorOnlyChange/Constrain(oldCC.Validity)/rollback → `windows/backend/core/manager.go` SwitchChannel → `windows/backend/core/engine.go` SwitchDefaultContext; `worker_child.go` switch command. Полный Reload закрывает old box до нового start/rollback.
- `windows/backend/render/render.go` selector literal → `windows/backend/config/config.go` zero bool/omitempty; `windows/go.mod` v1.13.19. `windows/frontend/src/api/contract.ts` AppState.activeChannel и `views/DashboardView.vue` scalar current channel — не per-flow census.
- `android/vpn/.../PhysicalNetworkMonitor.kt`, WorkerProcess.kt → `android/corebridge/.../NativeWorkerSession.kt`, PhysicalNetworkBinding.kt, ProtectedSocketControl.kt, AndroidTunFactory.kt: selected/pinned physical context, borrowed socket protect/bind и underlying reporting. Контекст/fault boundaries рассмотрены; вся native интеграция/SDK/OEM/device работа не audited.
- PLAN COM-09/17, OFF-03 и THEORY handover caveats адресно сопоставлены. Предлагаемые выводы не новые закрытые requirement IDs, общие планы не менялись.

T9/T10 REPORT — ручные допустимые source traces при явно указанных предпосылках,
не выполненная симуляция или подтверждённая runtime vulnerability. Более сильный
owner fence или иной specialized handler способен изменить applicability.

## 6. R06-004 — bounded history и допустимость диагноза

Дата среза: 2026-10-26. Запись и consistency review относятся к уже прочитанным inputs; актуальность более позднего source не проверялась.

| ID | Первичный источник / просмотренный предмет | Предел использования |
|---|---|---|
| E35 | [Kotlin StateFlow API](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/-state-flow/), web API1.11.0; [release1.10.2](https://github.com/Kotlin/kotlinx.coroutines/releases/tag/1.10.2) → [5f8900478a8e20c073145b1608fbc71fe3d7378b](https://github.com/Kotlin/kotlinx.coroutines/commit/5f8900478a8e20c073145b1608fbc71fe3d7378b) | Reference conflation/equality/current snapshot semantics. Проект android/vpn/build.gradle.kts использует1.10.2; version mismatch явно открыт. Raw StateFlow.kt/LICENSE.txt не скачались по timeout; никакого verified immutable file/license capture или internals1.10.2 claim нет |
| E36 | [Lamport, Time, Clocks, and the Ordering of Events in a Distributed System](https://lamport.azurewebsites.net/pubs/time-clocks.pdf), Communications of ACM21(7),July1978; определение partial happens-before/Clock Condition и отсутствие converse | Reference reasoning о порядке/causality, не внедрённый distributed-clock algorithm. Parent journal sequence не объявлен Lamport clock; физическая причина сбоя не выводится из total ordering |
| E37 | [OpenTelemetry Logs Data Model](https://opentelemetry.io/docs/specs/otel/logs/data-model/), Stable, Timestamp/ObservedTimestamp/optional trace context | Vocabulary reference. Местные elapsed ages не OTLP UNIX timestamps; trace context не подпись. SDK/collector/dependency/license integration не выбирались |

Собственный source review: `SessionJournal.kt` → `VpnSessionRepository.kt`
retainObservation/emit/command admission; `WorkerProcessLink.kt` identify/inspect
identity и retained actual-return observations; `WorkerBinding.kt` local opaque
journalContext; app `ControllerReport.kt` producer → `ControllerReportStore.kt`
frozen draft/edit/share revision checks → `ControllerReportScreen.kt` preview
и callback. System recipient/onShare downstream/native flow целиком не audited.

`.artifacts/research/theory-06/cycle-004/inputs-start.json` содержит8 paths:
6 selected source files и2плана. Изначально предполагаемый versions.toml не
существует, он не записан как input; фактический `android/vpn/build.gradle.kts`
и WorkerBinding.kt добавлены metadata2, всего10selected local paths.
`kotlinx-captures.json` сохраняет **оба timeout**, не successful source pin.
Повторно одинаковый raw запрос после deadline не делался.

Итоговый metadata review: изменились VpnSessionRepository и PLAN. Relevant
journal callers адресно перечитаны при финальном consistency review: validated
reply→workerObserved сохранён; stateChanged теперь получает фактически published
phase/issue, а не исходные аргументы. Это согласуется с ограниченным выводом о
projection, **не аудит всей новой controller реализации**. Current repository
copy сохранена `final-VpnSessionRepository.kt`; начальный hash и end hash не
смешиваются. Новый общий PLAN целиком не reviewed, старое mapping IDs не означает
принятие новых product statuses. Другие selected start6 не изменились на review.

Final window metadata R06-00319inputs обнаружил поздний drift PLAN; выводы
caller/source R06-003 относятся к ранее закреплённым bytes, ранний no-drift06:11UTC
не обещает freshness всех файлов до конца окна. Baselines R06-003 REPORT/SOURCES
сохранены до R06-004 дополнений. Данные исследования не секреты/реальные логи,
проект/тесты не запускались, исходники сторонних dependencies не исполнялись.

## 7. R06-006 — corporate proxy/authority/settings ownership

Исследовано26октября2026 в новом окне08:03:26Z–10:03:26Z; retrieval date,
не дата фактического runtime измерения. REPORT §25…30, handoffR06-006.

| ID | Primary URL/version/caller | Что обосновывает / предел |
|---|---|---|
| E38 | [Setting WinINet Proxy Configurations in WinHTTP](https://learn.microsoft.com/en-us/windows/win32/winhttp/setting-wininet-proxy-configurations-in-winhttp), Microsoft Learn; retrieved2026-10-26 | WinINet/user config не автоматически WinHTTP session/request config; WPAD задаёт per-request необходимость. API reference, Windows adapter у нас не реализован этим исследованием |
| E39 | [Authentication in WinHTTP](https://learn.microsoft.com/en-us/windows/win32/winhttp/authentication-in-winhttp), QueryAuthSchemes/SetCredentials/auto-logon, retrieved2026-10-26 | 401 vs407, scheme distinctions, connection-bound exchanges NTLM/Negotiate, default credential disclosure policy. Examples не authorize unbounded retries/LOW policy; credential values не читались |
| E40 | [WinHttpGetProxyForUrl](https://learn.microsoft.com/en-us/windows/win32/api/winhttp/nf-winhttp-winhttpgetproxyforurl) и [WinHTTP AutoProxy API](https://learn.microsoft.com/en-us/windows/win32/winhttp/winhttp-autoproxy-api), retrieved2026-10-26 | Lookup для конкретного URL, failure/known PAC URL, получение PAC может требовать auth. PAC discovery/evaluation здесь не запускались; arbitrary PAC не читался |
| E41 | [Network.openConnection](https://developer.android.com/reference/android/net/Network#openConnection(java.net.URL,%20java.net.Proxy)), API23; [ProxyInfo](https://developer.android.com/reference/android/net/ProxyInfo); [Builder.setHttpProxy](https://developer.android.com/reference/android/net/VpnService.Builder#setHttpProxy(android.net.ProxyInfo)), API29; retrieved2026-10-26 | Network-bound HTTP(S) URL API vs native transport, proxy metadata vs capability; downstream VPN proxy recommendation/PAC/split restrictions. Не всем приложениям/SDK/всем OEM универсальная настройка |
| E42 | sing-boxv1.13.19 [commit](https://github.com/SagerNet/sing-box/commit/b5ebaa1fc0f2b94256180b95468e73ef53caa27d), [protocol/http/outbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/http/outbound.go); singv0.8.13 [release](https://github.com/SagerNet/sing/releases/tag/v0.8.13) → [7c349dacf402256d3a7029746073b05d2ead584a](https://github.com/SagerNet/sing/commit/7c349dacf402256d3a7029746073b05d2ead584a) | Local actual HTTP client source и provenance ниже; sing-box raw comparison успешен, sing raw files не получены. Не independently verified full sing pin, не enterprise-auth/product support |
| E43 | [OpenVPN2.6 manual — http-proxy/auto-nct](https://openvpn.net/community-docs/community-articles/openvpn-2-6-manual/), [management interface](https://openvpn.net/community-docs/management-interface/), retrieved2026-10-26 | Mature explicit-proxy/auth/interactive-credential workflow reference; management socket/credential files/protocol implementation не заимствованы. Release code/лицензионная интеграция нового OpenVPN core не выбирались |
| E44 | [RFC9110 §9.3.6 CONNECT](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.6), June2022; E34 §9.2.2 остаётся отдельным retry основанием | Tunnel formation не VPN peer/trust/protected egress; запрет broad app transaction retry не отменяется proxy handshake. RFC code не переносился |

### 7.1. Cache/public source provenance и licenses

В `.artifacts/research/theory-06/cycle-006/` сохранены cache copies и manifest:

- `sing-box` HTTP outbound exact commit raw SHA256
  `138e08737f22f3eec74df2d8aae8b066bd1292f912eac1c47416f6f640071b9a`,
  matches local module cache. Pinned GPL3-or-later LICENSE ранее сохранён cycle-003
  того же immutable commit; новых dependencies/code imports нет.
- sing cache v0.8.13 `protocol/http/client.go` SHA256
  `d3768cc7ee794dcdc7bd69cc50ff2ab79dd0e179669e30d57e663395aed53a74`,
  LICENSE SHA256 `2f02b7486bcfa90d115c71a20437f3906b6fd5bef81c5dc0efd341399e89d0fd`.
  Прочитанный cache LICENSE — GPL-3.0-or-later. NewClient→DialContext TCP→
  CONNECT→optional Basic→200/407 error/context cancellation вручную прочитаны.
  ПоляHeaders не реализуют сами NTLM/Negotiate state machine.
- `upstream-captures.json` фиксирует short-ref sing timeouts и успешный box;
  `fullpin-captures.json` — повтор с разрешённым полным commit, оба sing raw timeout.
  `cache-provenance.json` сохраняет явно **not obtained external comparison**.
  Следующая web blob/license партия закончилась network error. Это negative
  retrieval boundary, не подтверждённые public file bytes/не повод менять deps.

Прочитан также sing HTTP handshake inbound как смежный public-source context;
его raw error construction **не предлагается** generic диагностике/экспорту
или как новый runtime путь. Факт достижимости такого кода в продукте не проверялся;
security finding/scan и новый scope из него не создавались.

### 7.2. Local scope / source drift

11selected initial inputs: windows/go.mod; core/proxy_windows.go, manager.go,
probe.go, cleanup.go; config/config.go и render/render.go; Android
PhysicalPathContext, ProtectedSocketControl, AndroidTunFactory, PhysicalNetworkBinding.
Hashes в inputs-start.json; никаких registry/settings/real profiles/credentials
не читалось. Windows registry path в REPORT — literal кода, не live observation.

Actual chain reviewed: Manager.Start/restoreProxy→ProxyManager.Snapshot/SetProxy/
Restore; config isProtectedOutboundType/detour admission; box HTTP NewOutbound→
sing client. Android physical context metadata→bound socket protect; весь service/
JNI/credential/OS gate code здесь не audited. Peer/guard authority proposals
не объявлены существующим API. Старые REPORT/SOURCES/STATE baseline сохранены
до добавления нового окна; общая теория/планы/старые указатели не изменялись.

Timeouts не превращены в факт отсутствия upstream capability; вывод про текущий
client помечен cache-source scoped. Нужны независимый file pin и platform evidence
перед implementation/integration claim. ВсеW01…W15 — NOT_RUN.

## 8. R06-007 — IPv6-only/NAT64 bootstrap и path identity

Просмотрено26октября2026 в новом2h окне. Источники ниже — published primary
references/actual code, не утверждение о текущей мобильной сети владельца.

| ID | Primary URL / dated version | Просмотренный предмет и предел |
|---|---|---|
| E45 | [RFC6147](https://www.rfc-editor.org/rfc/rfc6147.html), April2011, introduction/DNS64 mechanism | AAAA synthesis из A для обращения по FQDN; не автоматическая обработка каждого IPv4 literal и не доказанный working DNS64 в данной сети |
| E46 | [RFC6877](https://www.rfc-editor.org/rfc/rfc6877.html), April2013, architecture/CLAT/PLAT и literals | 464XLAT допускает IPv4 literal/socket поверх IPv6-only сети. Не universal Android/Windows support/device receipt |
| E47 | [RFC6052](https://www.rfc-editor.org/rfc/rfc6052.html), October2010, §2 prefixes/encoding | Network-specific lengths32/40/48/56/64/96, well-known /96 — один вариант. Собственная encoding implementation/crypto не выполнялись |
| E48 | [RFC8305](https://www.rfc-editor.org/rfc/rfc8305.html), December2017, §7.1/7.2/7.3 | Literal synthesis, broken AAAA и VPN split-DNS resolver/underlay-prefix distinction. Dated reference, не claim «самый новый стандарт», не наши measured timers/SLO или implementation |
| E49 | [LinkProperties API reference](https://developer.android.com/reference/android/net/LinkProperties) и [RFC7050](https://www.rfc-editor.org/rfc/rfc7050.html) | Retrieval attempts **timeout**, полный текст не inspected. Ссылка — open source gap, не основание проверенного API/discovery algorithm. SDK>=30/nat64Prefix statement ограничен actual local PhysicalPathContext source |
| E50 | sing-boxv1.13.19, [common/dialer/resolve.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/resolve.go) | Cache actual NewResolveDialer/DialContext/ListenPacket: domain Lookup→parallel/serial, literal lower-dialer fast path. New raw comparison timeout, не independently confirmed file bytes; whole lower dialer/OS translation не audited |

### 8.1. Captures / exact local callers

Ignored `cycle-007/rfc-captures.json` и `rfc-extra-captures.json`: RFC6147/6877/
6052/8305 public TXT с SHA256 и UTC retrieval. Полные ссылки primary TXT:
`https://www.rfc-editor.org/rfc/rfc{6147,6877,6052,8305}.txt` (каждая загружена
отдельно; фигурные скобки здесь сокращение записи, не literal network URL).
RFC7050 failure сохранён, contents не использовались. API page timeout указан
в linkproperties-retrieval.json. Ни DNS64/CLAT/discovery код, ни crypto не запускались.

9selected local paths: inputs-start7 + extra2. Actual review:

- Windows corebridge.renderAndPrepareContext → PreflightResolver.Resolve
  (UDP LookupHost→firstPublicIPv4; DoH type=A; host cache) → render.WithDialOverrides
  → config numeric server + preserved domain TLS/WS Host; default bootstrap DNS
  and prefer_ipv4. IP literal fast path и cacheGet generic ParseIP учтены.
- Android PhysicalPathContext NAT64/privateDNS/route fingerprint,
  PhysicalNetworkBinding context lease, ProtectedSocketControl protect/bind;
  LibboxPlatform unsupported localDNS until accepted policy; AndroidTunFactory
  IPv4 TUN/optionalIPv6 capture без allowFamily/allowBypass. Не читать physical
  IPv4 из требования **внутреннего** TUN. Entire SDK/native/provider не audited.
- Metadata/source copies: source-preflight.go, cache-box-resolve.go;
  dialer-provenance.json (version/commit/hash), dialer-public-comparison.json
  (timeout). Existing GPL3-or-later core license ранее прочитан; новая third-party
  dependency/перенос исходников в продукт не выбирались.

Начальное `rg --files` не нашло дополнительных probe/transport-named Kotlin
файлов; это **не доказательство отсутствия** всей probe платформы. Выводы среза
сужены до просмотренных actual callers и названы conditional. Реальные caches,
profiles/env, пользовательские DNS/proxy/OS settings не читались.

Model branch local synthesis — предложение к authority/transport owners, не
новый существующий API и не разрешение публичного DNS fallback. Cases N01…N12
NOT_RUN. Теория описывает возможный closure/cut; фактический CLAT/NAT64/IPv6
provider path и количественная польза остаются неизвестны.

## 9. R06-008 — composition из уже закреплённых inputs

Дата среза: 2026-10-26; использованы уже закреплённые inputs без новой source-acquisition партии.
Использованы E42 actual `cycle-006/cache-sing-protocol_http_client.go`
(DialContext serverAddr→CONNECT destination), E44 RFC9110 CONNECT и E45…48
DNS64/CLAT/network-specific prefix/Happy Eyeballs references. Source и license/
byte-verification пределы R06-006/007 **полностью наследуются**, новое сильное
upstream evidence не заявляется.

T15 — ручной контрпример будущего global synthesis rewrite при разных network
realms P/N, не уже реализованный дефект/исполненная симуляция. Caller placement
важнее общей отметки «proxy/IPv6». Никакого нового формата/SDK/transport role не
создавали. Baseline предыдущих REPORT/SOURCES — ignored cycle-008; J01…J06 NOT_RUN.

## 10. R06-010 — split DNS/NRPT/PAC/LAN и Android-сочетания sources

Дата среза: 2026-10-26.

| Источник | Что фиксирует |
|---|---|
| Snowden `windows/wfp_killswitch_windows.go:65-107` | Точный strict-набор permit (engine/loopback/DHCP/TUN-LUID) + catch-all block; отсутствие LocalSubnet-разрешения — проектный fail-closed |
| Snowden `android/.../AndroidTunFactory.kt:52-78` | full-tunnel routes, один DNS, без allowBypass/allowFamily/per-app списков |
| [MS-GPNRPT: Name Resolution Policy Table Data Extension](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpnrpt/0fb6a915-3dcc-439b-bace-674100c97a25) | NRPT-записи в GPO-ветке реестра `Software\Policies\Microsoft\Windows NT\DNSClient\`; механизм namespace→servers |
| [Add-DnsClientNrptRule (DnsClient)](https://learn.microsoft.com/en-us/powershell/module/dnsclient/add-dnsclientnrptrule) | Управление NRPT-правилами, включая GPO-target; ru-RU локаль доступна заменой en-us→ru-ru на Learn |
| [VpnService / Android VPN](https://developer.android.com/develop/connectivity/vpn) | Один VpnService на пользователя/профиль; Always-on и «Block connections without VPN» — системные переключатели; per-app наборы через Builder.addAllowed/DisallowedApplication |
| [DevicePolicyManager.setAlwaysOnVpnPackages](https://developer.android.com/reference/android/app/admin/DevicePolicyManager#setAlwaysOnVpnPackages\(android.content.ComponentName,%20java.util.List%3Cjava.lang.String%3E,%20boolean\)) | DPC-управление always-on + lockdown в рабочем профиле — вне досягаемости приложения в personal-профиле |
| Baseline R06-006…008 | Прокси-ownership/NAT64/dial-hop — не повторяются, наследуются как есть |

Negative search: NRPT/DnsClientNrpt в windows/ — не найдено (Snowden NRPT не
меняет). ОС-доки читались как canonical en-US/протоколы; никакие NRPT/lockdown
настройки не применялись. Все future cases NOT_RUN.

## 11. R06-011 — измерение recovery sources

| Источник | Что фиксирует |
|---|---|
| [tailscale/tailscale LICENSE](https://github.com/tailscale/tailscale/blob/main/LICENSE) | BSD-3-Clause — прочитано этим срезом напрямую (raw) |
| Chromium net errors (паттерн) | Общеизвестный UI-паттерн объяснимых сетевых отказов; кодовая база BSD-3 |
| AOSP baseline R06-001 | E01-E19, AOSP16 immutable commit (SOURCES §1-4) — переиспользован как указатель, recheck NOT_RUN |
| AmneziaVPN, Mullvad | Кандидаты-референсы; file-check лицензий NOT_RUN этим срезом — честная граница |
| R03-009 (ledger) | Правило предрегистрации классов задач для метрики |
| R06-004 §22-24 | K(y)-критерий и frozen export как producer-источник Windows |

Дата среза: 2026-10-26. Пользовательские эксперименты/замеры не проводились; support-экспорт не собирался. Все future cases NOT_RUN.
